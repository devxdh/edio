package gitengine

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/devxdh/igit/pkg/testutil"
)

type statusRecord struct {
	Staged   byte
	Unstaged byte
	Path     string
}

// getPorcelainStatus extracts machine-readable NUL-delimited status records from Git.
func getPorcelainStatus(tb testing.TB) map[string]statusRecord {
	tb.Helper()

	raw, err := RunGitRaw("status", "--porcelain=v1", "-z")
	if err != nil {
		tb.Fatalf("failed to query git status: %v", err)
	}

	records := make(map[string]statusRecord)
	if len(raw) == 0 {
		return records
	}

	for entry := range bytes.SplitSeq(raw, []byte{0}) {
		if len(entry) < 4 {
			continue
		}
		staged := entry[0]
		unstaged := entry[1]
		path := string(entry[3:])

		records[path] = statusRecord{
			Staged:   staged,
			Unstaged: unstaged,
			Path:     path,
		}
	}

	return records
}

func TestBuildIsolatedTree(t *testing.T) {
	tmpDir := testutil.SetupTestRepo(t, "igit-tree-behavior-*")

	// 1. Commit a baseline .gitignore file
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(".env\nnode_modules/\n"), 0o644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
	if _, err := RunGit("add", ".gitignore"); err != nil {
		t.Fatalf("failed to stage .gitignore: %v", err)
	}
	if _, err := RunGit("commit", "-m", "add gitignore"); err != nil {
		t.Fatalf("failed to commit .gitignore: %v", err)
	}

	// 2. User explicitly stages a file in the primary index
	userFile := filepath.Join(tmpDir, "user_staged.txt")
	if err := os.WriteFile(userFile, []byte("user intent\n"), 0o644); err != nil {
		t.Fatalf("failed to write user file: %v", err)
	}
	if _, err := RunGit("add", "user_staged.txt"); err != nil {
		t.Fatalf("failed to stage user file: %v", err)
	}

	// 3. User modifies a tracked file in the working tree without staging it
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Base Project\nUnstaged User Edit\n"), 0o644); err != nil {
		t.Fatalf("failed to edit README: %v", err)
	}

	// Assert baseline status before isolated tree build
	baseline := getPorcelainStatus(t)
	if rec, ok := baseline["user_staged.txt"]; !ok || rec.Staged != 'A' || rec.Unstaged != ' ' {
		t.Fatalf("baseline failure: expected user_staged.txt to be [A ], got [%c%c]", rec.Staged, rec.Unstaged)
	}
	if rec, ok := baseline["README.md"]; !ok || rec.Staged != ' ' || rec.Unstaged != 'M' {
		t.Fatalf("baseline failure: expected README.md to be [ M], got [%c%c]", rec.Staged, rec.Unstaged)
	}

	// 4. Agent creates a new untracked source file and an ignored secret file
	agentSource := filepath.Join(tmpDir, "agent_patch.go")
	if err := os.WriteFile(agentSource, []byte("package main\nfunc Patch(){}\n"), 0o644); err != nil {
		t.Fatalf("failed to write agent source: %v", err)
	}

	ignoredSecret := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(ignoredSecret, []byte("API_KEY=secret_12345\n"), 0o644); err != nil {
		t.Fatalf("failed to write ignored file: %v", err)
	}

	// 5. Generate isolated tree snapshot
	treeSHA, err := BuildIsolatedTree()
	if err != nil {
		t.Fatalf("BuildIsolatedTree failed: %v", err)
	}
	if len(treeSHA) != 40 {
		t.Fatalf("invalid tree SHA: %s", treeSHA)
	}

	// 6. Invariant: Staging area and working tree statuses remain unmodified
	postStatus := getPorcelainStatus(t)
	if rec, ok := postStatus["user_staged.txt"]; !ok || rec.Staged != 'A' || rec.Unstaged != ' ' {
		t.Fatalf("index pollution: user_staged.txt status mutated to [%c%c]", rec.Staged, rec.Unstaged)
	}
	if rec, ok := postStatus["README.md"]; !ok || rec.Staged != ' ' || rec.Unstaged != 'M' {
		t.Fatalf("index pollution: README.md status mutated to [%c%c]", rec.Staged, rec.Unstaged)
	}
	if rec, ok := postStatus["agent_patch.go"]; !ok || rec.Staged != '?' || rec.Unstaged != '?' {
		t.Fatalf("index pollution: agent_patch.go was staged! Status: [%c%c]", rec.Staged, rec.Unstaged)
	}

	// 7. Invariant: Tree object captures both tracked changes and new untracked files
	treeListing, err := RunGit("ls-tree", "-r", "--name-only", treeSHA)
	if err != nil {
		t.Fatalf("failed to inspect tree SHA: %v", err)
	}
	capturedFiles := strings.Split(strings.TrimSpace(treeListing), "\n")

	if !slices.Contains(capturedFiles, "agent_patch.go") {
		t.Fatalf("isolated tree missed agent_patch.go; contents: %v", capturedFiles)
	}
	if !slices.Contains(capturedFiles, "user_staged.txt") {
		t.Fatalf("isolated tree missed user_staged.txt; contents: %v", capturedFiles)
	}
	if !slices.Contains(capturedFiles, "README.md") {
		t.Fatalf("isolated tree missed README.md; contents: %v", capturedFiles)
	}

	// 8. Invariant: Ignored files are strictly excluded from the tree object
	if slices.Contains(capturedFiles, ".env") {
		t.Fatalf("isolated tree leaked ignored file .env; contents: %v", capturedFiles)
	}
}
