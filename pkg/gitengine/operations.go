// Package gitengine handles low-level Git execution and internal object creation.
//
// This package creates Git database objects (blobs, trees,
// and commits) and updates internal references directly without modifying the user's
// active workspace. Instead of running user-facing commands like "git commit"
// (which change branches and the staging area)
package gitengine

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var (
	// ErrGitNotInstalled is returned when the "git" executable is missing from the system PATH.
	ErrGitNotInstalled = errors.New("git is not installed or not found in system PATH")

	// ErrNotAGitRepo is returned when a command runs outside a valid Git working directory.
	ErrNotAGitRepo = errors.New("current directory is not a git repository")

	// ErrEmptyRef is returned when an empty string is passed as a Git reference path.
	ErrEmptyRef = errors.New("ref name cannot be empty")
)

// RunGit executes a Git command and returns its standard output as a string.
//
// It strips leading and trailing whitespace from the final output.
// If the command fails, it captures stderr to provide the actual Git error message.
//
// Warning: Do not use this for commands that output binary data or null-delimited
// records (like -z flags), because whitespace stripping can corrupt the data.
// Use RunGitRaw for those cases.
func RunGit(args ...string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("no git arguments provided")
	}

	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return "", ErrGitNotInstalled
		}

		errStr := strings.TrimSpace(stderr.String())
		if strings.Contains(errStr, "not a git repository") {
			return "", ErrNotAGitRepo
		}

		if errStr != "" {
			return "", fmt.Errorf("git %s: %s", args[0], errStr)
		}

		return "", fmt.Errorf("git %s failed: %w", args[0], err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// RunGitRaw executes a Git command and returns the raw output bytes without any modification.
//
// This is required when running commands with the "-z" flag. In "-z" mode, Git separates
// records with null bytes (\x00) and preserves exact file paths, including spaces and newlines.
func RunGitRaw(args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, errors.New("no git arguments provided")
	}

	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return nil, ErrGitNotInstalled
		}

		errStr := strings.TrimSpace(stderr.String())
		if strings.Contains(errStr, "not a git repository") {
			return nil, ErrNotAGitRepo
		}

		if errStr != "" {
			return nil, fmt.Errorf("git %s: %s", args[0], errStr)
		}

		return nil, fmt.Errorf("git %s failed: %w", args[0], err)
	}

	return stdout.Bytes(), nil
}

// EnsureGitRepo checks whether the current working directory is inside a Git repository.
// It returns nil if the check passes, or ErrNotAGitRepo if outside a repository.
func EnsureGitRepo() error {
	_, err := RunGit("rev-parse", "--is-inside-work-tree")
	return err
}

// GetRepoRoot returns the absolute filesystem path of the repository root directory.
func GetRepoRoot() (string, error) {
	return RunGit("rev-parse", "--show-toplevel")
}

// HashBlob writes a single file into Git's object database (.git/objects/) as a blob.
//
// The "-w" flag forces Git to write the object immediately to disk.
// Returns the 40-character SHA-1 hash of the created blob.
func HashBlob(filePath string) (string, error) {
	if strings.TrimSpace(filePath) == "" {
		return "", errors.New("file path cannot be empty")
	}
	return RunGit("hash-object", "-w", filePath)
}

// CommitTree creates a commit object directly from a directory tree SHA.
//
// Why this is used instead of "git commit":
// "git commit" modifies the active branch (HEAD) and commits the active staging area (.git/index).
// "git commit-tree" creates an isolated commit in the object database without moving HEAD
// or changing any active branch.
//
// Parameters:
//   - treeSHA: The 40-character SHA of the directory tree snapshot to commit.
//   - parentSHA: The 40-character SHA of the previous commit. Pass "" to create a root commit.
//   - message: The commit message text.
//
// Returns the 40-character SHA of the newly created commit.
func CommitTree(treeSHA, parentSHA, message string) (string, error) {
	if strings.TrimSpace(treeSHA) == "" {
		return "", errors.New("tree SHA cannot be empty")
	}

	if strings.TrimSpace(message) == "" {
		message = "igit: turn snapshot"
	}

	args := []string{"commit-tree", treeSHA, "-m", message}
	if parentSHA != "" {
		args = append(args, "-p", parentSHA)
	}

	return RunGit(args...)
}

// UpdateRef updates or creates a Git reference on disk (e.g. "refs/igit/active/sess_1/1").
//
// Why this is used:
// This writes a pointer file in .git/refs/ directly. It creates references that
// standard Git commands (like "git branch" or "git status") will not expose to the user,
// keeping igit's snapshot history completely isolated from normal Git usage.
func UpdateRef(refName, commitSHA string) error {
	if strings.TrimSpace(refName) == "" {
		return ErrEmptyRef
	}

	if strings.TrimSpace(commitSHA) == "" {
		return errors.New("commit SHA cannot be empty")
	}

	_, err := RunGit("update-ref", refName, commitSHA)
	return err
}

// GetRef resolves a Git reference name to its 40-character commit SHA.
//
// It uses "git rev-parse --verify --quiet".
// If the ref does not exist on disk, Git returns an error, but GetRef suppresses it
// and returns an empty string ("") with a nil error so callers can easily check
// whether a reference exists without error handling clutter.
func GetRef(refName string) (string, error) {
	if strings.TrimSpace(refName) == "" {
		return "", ErrEmptyRef
	}

	out, err := RunGit("rev-parse", "--verify", "--quiet", refName)
	if err != nil {
		return "", nil
	}

	return out, nil
}
