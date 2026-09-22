package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devxdh/edio/pkg/gitengine"
)

// Restore reverts the workspace (or a single file) to a specific turn snapshot.
//
// Parameters:
//   - targetTurn: The turn number (1-indexed) to restore back to.
//   - filePath: If non-empty, restores ONLY this specific file. If empty, restores full workspace.
//
// Invariants guaranteed:
//   - Staging index (.git/index) is untouched (zero index pollution).
//   - Files introduced in turns after targetTurn are safely removed from the worktree.
//   - Pre-existing untracked user files (e.g. .env, untracked configs) are preserved.
//
// Returns the resolved commit SHA of the target turn, or an error if restoration fails.
func (s *Session) Restore(targetTurn int, filePath string) (string, error) {
	if targetTurn < 1 || targetTurn > s.TurnCount {
		return "", fmt.Errorf("turn %d does not exist (current session has %d turns)", targetTurn, s.TurnCount)
	}

	targetRef := s.ActiveRef(targetTurn)
	targetSHA, err := gitengine.GetRef(targetRef)
	if err != nil || targetSHA == "" {
		return "", fmt.Errorf("failed to resolve target turn ref %s: %w", targetRef, err)
	}

	// 1. Single file restoration
	if filePath != "" {
		_, err = gitengine.RunGit("restore", "--source="+targetSHA, "--worktree", "--", filePath)
		if err != nil {
			return "", fmt.Errorf("failed to restore file %s: %w", filePath, err)
		}
		return targetSHA, nil
	}

	// 2. Full workspace restoration
	repoRoot, err := gitengine.GetRepoRoot()
	if err != nil {
		return "", fmt.Errorf("failed to determine repository root: %w", err)
	}

	// Step 2a: Remove files introduced in turns AFTER targetTurn
	if s.LatestSHA != "" && s.LatestSHA != targetSHA {
		addedFilesRaw, err := gitengine.RunGit(
			"diff-tree", "-r", "--no-commit-id", "--name-only", "--diff-filter=A",
			targetSHA, s.LatestSHA,
		)
		if err == nil && strings.TrimSpace(addedFilesRaw) != "" {
			for _, file := range strings.Split(strings.TrimSpace(addedFilesRaw), "\n") {
				trimmed := strings.TrimSpace(file)
				if trimmed == "" {
					continue
				}

				var fullPath string
				if filepath.IsAbs(trimmed) {
					fullPath = trimmed
				} else {
					fullPath = filepath.Join(repoRoot, trimmed)
				}

				if err := os.Remove(fullPath); err == nil {
					// Clean up empty parent directories up to repo root
					dir := filepath.Dir(fullPath)
					for dir != "" && dir != repoRoot && strings.HasPrefix(dir, repoRoot) {
						if err := os.Remove(dir); err != nil {
							break
						}
						dir = filepath.Dir(dir)
					}
				}
			}
		}
	}

	// Step 2b: Restore all tracked files to the working directory without staging
	_, err = gitengine.RunGit("restore", "--source="+targetSHA, "--worktree", "--", ".")
	if err != nil {
		return "", fmt.Errorf("failed to restore workspace: %w", err)
	}

	return targetSHA, nil
}
