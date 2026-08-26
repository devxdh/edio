package gitengine

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runGitWithEnv executes a Git command with custom environment variables injected.
//
// It copies the current process environment (os.Environ) and appends the extra variables.
// This is required to pass GIT_INDEX_FILE so Git uses a temporary index file instead
// of the default .git/index.
func runGitWithEnv(env []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %v, stderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// BuildIsolatedTree creates a full Git directory tree snapshot of the current workspace
// without altering the user's primary staging area (.git/index).
//
// Steps:
//  1. Creates a uniquely named temporary index file in .git/ (e.g. edio_index_<timestamp>_<random>.tmp).
//  2. Sets GIT_INDEX_FILE to force all subsequent Git commands in this run to target that file.
//  3. Seeds the temporary index from the current HEAD commit using "git read-tree HEAD" (if HEAD exists).
//  4. Stages all current file modifications, additions, and deletions using "git add -A" (respecting .gitignore).
//  5. Writes the resulting directory state into a Git tree object via "git write-tree".
//  6. Deletes the temporary index file in a deferred cleanup step.
//
// Invariants guaranteed:
//   - If the developer had files staged in .git/index, those remain unchanged.
//   - Build dirs, secrets, and ignored dependencies are excluded.
//   - The random temporary index name prevents collisions across rapid runs.
//
// Returns the 40-character SHA of the generated tree object.
func BuildIsolatedTree() (string, error) {
	if err := EnsureGitRepo(); err != nil {
		return "", err
	}

	gitDir, err := RunGit("rev-parse", "--git-dir")
	if err != nil {
		return "", fmt.Errorf("failed to locate git directory: %w", err)
	}

	// Generate a collision-resistant filename using high-resolution timestamp and random hex bytes
	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)
	tempIndexName := fmt.Sprintf(
		"edio_index_%d_%s.tmp",
		time.Now().UnixNano(),
		hex.EncodeToString(randomBytes),
	)
	tempIndexPath := filepath.Join(gitDir, tempIndexName)

	// Ensure the temporary index scratchpad is deleted on return, even on execution failure
	defer func() {
		_ = os.Remove(tempIndexPath)
	}()

	env := []string{fmt.Sprintf("GIT_INDEX_FILE=%s", tempIndexPath)}

	// Check whether HEAD exists. In a brand-new repository with zero commits,
	// "git rev-parse --verify HEAD" will fail, and we must skip read-tree.
	hasHead := true
	if _, err := RunGit("rev-parse", "--verify", "HEAD"); err != nil {
		hasHead = false
	}

	if hasHead {
		// Populate the temporary index with the base commit's tree structure
		if _, err := runGitWithEnv(env, "read-tree", "HEAD"); err != nil {
			return "", fmt.Errorf("failed to read HEAD into isolated index: %w", err)
		}
	}

	// Stage all current workspace changes (untracked, modified, deleted) into the temporary index
	if _, err := runGitWithEnv(env, "add", "-A"); err != nil {
		return "", fmt.Errorf("failed to stage working tree into isolated index: %w", err)
	}

	// Convert the staged index into a persistent Git tree object
	treeSHA, err := runGitWithEnv(env, "write-tree")
	if err != nil {
		return "", fmt.Errorf("failed to write isolated tree: %w", err)
	}

	if len(treeSHA) != 40 {
		return "", fmt.Errorf("invalid tree SHA received: %s", treeSHA)
	}

	return treeSHA, nil
}
