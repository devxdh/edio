// Package testutil provides common test fixtures, isolated repositories, and helper utilities.
package testutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %v, stderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func SetupTestRepo(tb testing.TB, prefix string) string {
	tb.Helper()

	tmpDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		tb.Fatalf("failed to create temp repo dir: %v", err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		tb.Fatalf("failed to get working dir: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		tb.Fatalf("failed to change dir to temp repo: %v", err)
	}

	tb.Cleanup(func() {
		_ = os.Chdir(origWd)
		_ = os.RemoveAll(tmpDir)
	})

	if _, err := runGit("init"); err != nil {
		tb.Fatalf("git init failed: %v", err)
	}

	if _, err := runGit("config", "user.name", "edio-test-bot"); err != nil {
		tb.Fatalf("failed to configure test user.name: %v", err)
	}
	if _, err := runGit("config", "user.email", "bot@edio.dev"); err != nil {
		tb.Fatalf("failed to configure test user.email: %v", err)
	}

	initFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(initFile, []byte("# Base Project\n"), 0o644); err != nil {
		tb.Fatalf("failed to write initial README: %v", err)
	}
	if _, err := runGit("add", "README.md"); err != nil {
		tb.Fatalf("failed to stage README: %v", err)
	}
	if _, err := runGit("commit", "-m", "initial commit on main"); err != nil {
		tb.Fatalf("failed to make initial commit: %v", err)
	}

	return tmpDir
}
