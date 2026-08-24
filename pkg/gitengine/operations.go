// Package gitengine is responsible for all the git operations needed for igit.
package gitengine

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var (
	ErrGitNotInstalled = errors.New("git is not installed or not found in system PATH")
	ErrNotAGitRepo     = errors.New("current directory is not a git repository")
	ErrEmptyRef        = errors.New("ref name cannot be empty")
)

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

func EnsureGitRepo() error {
	_, err := RunGit("rev-parse", "--is-inside-work-tree")
	return err
}

func GetRepoRoot() (string, error) {
	return RunGit("rev-parse", "--show-toplevel")
}

func HashBlob(filePath string) (string, error) {
	if strings.TrimSpace(filePath) == "" {
		return "", errors.New("file path cannot be empty")
	}
	return RunGit("hash-object", "-w", filePath)
}

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
