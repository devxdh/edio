package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/testutil"
)

func TestSessionRestore(t *testing.T) {
	tmpDir := testutil.SetupTestRepo(t, "edio-restore-test-*")

	// Create initial base commit in the repo so HEAD exists
	baseFile := filepath.Join(tmpDir, "base.txt")
	if err := os.WriteFile(baseFile, []byte("base content\n"), 0o644); err != nil {
		t.Fatalf("failed to write base file: %v", err)
	}
	if _, err := gitengine.RunGit("add", "base.txt"); err != nil {
		t.Fatalf("failed to stage base file: %v", err)
	}
	if _, err := gitengine.RunGit("commit", "-m", "initial commit"); err != nil {
		t.Fatalf("failed to commit base file: %v", err)
	}

	sess := NewSession()

	// --- Turn 1: Modify base.txt and create turn1.txt ---
	turn1File := filepath.Join(tmpDir, "turn1.txt")
	if err := os.WriteFile(turn1File, []byte("turn 1 content\n"), 0o644); err != nil {
		t.Fatalf("failed to write turn1 file: %v", err)
	}
	if err := os.WriteFile(baseFile, []byte("base modified in turn 1\n"), 0o644); err != nil {
		t.Fatalf("failed to modify base file: %v", err)
	}

	tree1, err := gitengine.BuildIsolatedTree()
	if err != nil {
		t.Fatalf("failed to build isolated tree 1: %v", err)
	}
	turn1SHA, err := sess.RecordTurn(tree1, "Turn 1")
	if err != nil {
		t.Fatalf("failed to record turn 1: %v", err)
	}

	// --- Turn 2: Modify turn1.txt, add turn2.txt, add nested/file.txt ---
	turn2File := filepath.Join(tmpDir, "turn2.txt")
	if err := os.WriteFile(turn2File, []byte("turn 2 rogue file\n"), 0o644); err != nil {
		t.Fatalf("failed to write turn2 file: %v", err)
	}
	nestedDir := filepath.Join(tmpDir, "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	nestedFile := filepath.Join(nestedDir, "deep.txt")
	if err := os.WriteFile(nestedFile, []byte("nested rogue file\n"), 0o644); err != nil {
		t.Fatalf("failed to write nested file: %v", err)
	}
	if err := os.WriteFile(turn1File, []byte("turn 1 modified in turn 2\n"), 0o644); err != nil {
		t.Fatalf("failed to modify turn1 file: %v", err)
	}

	tree2, err := gitengine.BuildIsolatedTree()
	if err != nil {
		t.Fatalf("failed to build isolated tree 2: %v", err)
	}
	_, err = sess.RecordTurn(tree2, "Turn 2")
	if err != nil {
		t.Fatalf("failed to record turn 2: %v", err)
	}

	// Create an untracked user file that was NOT snapshotted (e.g. .env or personal notes)
	untrackedUserFile := filepath.Join(tmpDir, "user_scratchpad.txt")
	if err := os.WriteFile(untrackedUserFile, []byte("keep this user file\n"), 0o644); err != nil {
		t.Fatalf("failed to write untracked user file: %v", err)
	}

	// Manually stage a file to test zero index pollution
	manualStagedFile := filepath.Join(tmpDir, "manual_staged.txt")
	if err := os.WriteFile(manualStagedFile, []byte("manually staged content\n"), 0o644); err != nil {
		t.Fatalf("failed to write manual staged file: %v", err)
	}
	if _, err := gitengine.RunGit("add", "manual_staged.txt"); err != nil {
		t.Fatalf("failed to stage manual file: %v", err)
	}

	// --- Execute Full Workspace Restore to Turn 1 ---
	resSHA, err := sess.Restore(1, "")
	if err != nil {
		t.Fatalf("failed to restore to Turn 1: %v", err)
	}
	if resSHA != turn1SHA {
		t.Fatalf("expected resolved SHA %s, got %s", turn1SHA, resSHA)
	}

	// 1. Verify files from Turn 1 are restored correctly
	turn1Content, err := os.ReadFile(turn1File)
	if err != nil || string(turn1Content) != "turn 1 content\n" {
		t.Fatalf("turn1.txt content mismatch: got %q, err: %v", string(turn1Content), err)
	}
	baseContent, err := os.ReadFile(baseFile)
	if err != nil || string(baseContent) != "base modified in turn 1\n" {
		t.Fatalf("base.txt content mismatch: got %q, err: %v", string(baseContent), err)
	}

	// 2. Verify files created in Turn 2 were DELETED
	if _, err := os.Stat(turn2File); !os.IsNotExist(err) {
		t.Fatalf("expected turn2.txt to be deleted, but it still exists")
	}
	if _, err := os.Stat(nestedFile); !os.IsNotExist(err) {
		t.Fatalf("expected nested/deep.txt to be deleted, but it still exists")
	}
	if _, err := os.Stat(nestedDir); !os.IsNotExist(err) {
		t.Fatalf("expected empty nested/ directory to be cleaned up, but it still exists")
	}

	// 3. Verify untracked user file was PRESERVED
	if _, err := os.Stat(untrackedUserFile); err != nil {
		t.Fatalf("expected user_scratchpad.txt to be preserved, got err: %v", err)
	}

	// 4. Verify ZERO staging index pollution:
	// manual_staged.txt MUST still be staged
	// turn1.txt MUST NOT be staged
	statusOut, err := gitengine.RunGit("status", "--porcelain")
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}

	lines := strings.Split(statusOut, "\n")
	stagedFound := false
	turn1Staged := false
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		indexStatus := l[0]
		fileName := strings.TrimSpace(l[3:])
		if fileName == "manual_staged.txt" && indexStatus == 'A' {
			stagedFound = true
		}
		if fileName == "turn1.txt" && indexStatus != ' ' && indexStatus != '?' {
			turn1Staged = true
		}
	}

	if !stagedFound {
		t.Fatalf("expected manual_staged.txt to remain staged (indexStatus 'A'), status:\n%s", statusOut)
	}
	if turn1Staged {
		t.Fatalf("expected turn1.txt to NOT be staged in index, status:\n%s", statusOut)
	}
}

func TestSessionRestoreSingleFile(t *testing.T) {
	tmpDir := testutil.SetupTestRepo(t, "edio-restore-single-test-*")

	fileA := filepath.Join(tmpDir, "fileA.txt")
	fileB := filepath.Join(tmpDir, "fileB.txt")
	_ = os.WriteFile(fileA, []byte("version 1 A\n"), 0o644)
	_ = os.WriteFile(fileB, []byte("version 1 B\n"), 0o644)

	sess := NewSession()
	tree1, _ := gitengine.BuildIsolatedTree()
	_, _ = sess.RecordTurn(tree1, "Turn 1")

	_ = os.WriteFile(fileA, []byte("version 2 A\n"), 0o644)
	_ = os.WriteFile(fileB, []byte("version 2 B\n"), 0o644)
	tree2, _ := gitengine.BuildIsolatedTree()
	_, _ = sess.RecordTurn(tree2, "Turn 2")

	// Restore ONLY fileA from Turn 1
	_, err := sess.Restore(1, "fileA.txt")
	if err != nil {
		t.Fatalf("failed to restore fileA: %v", err)
	}

	contentA, _ := os.ReadFile(fileA)
	if string(contentA) != "version 1 A\n" {
		t.Fatalf("fileA was not restored: got %q", string(contentA))
	}

	contentB, _ := os.ReadFile(fileB)
	if string(contentB) != "version 2 B\n" {
		t.Fatalf("fileB should remain at version 2: got %q", string(contentB))
	}
}
