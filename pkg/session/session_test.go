package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devxdh/igit/pkg/gitengine"
	"github.com/devxdh/igit/pkg/testutil"
)

func TestSessionLifeCycle(t *testing.T) {
	tmpDir := testutil.SetupTestRepo(t, "igit-session-test-*")

	sess := NewSession()
	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}

	dummyFile := filepath.Join(tmpDir, "init.txt")
	if err := os.WriteFile(dummyFile, []byte("init"), 0o644); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}
	if _, err := gitengine.RunGit("add", "init.txt"); err != nil {
		t.Fatalf("failed to stage dummy file: %v", err)
	}

	treeSHA, err := gitengine.RunGit("write-tree")
	if err != nil {
		t.Fatalf("failed to create base tree: %v", err)
	}

	turn1SHA, err := sess.RecordTurn(treeSHA, "[Turn 1] Started task")
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}

	if sess.TurnCount != 1 || sess.LatestSHA != turn1SHA {
		t.Fatalf("Session state mismatch after Turn 1: count=%d, latest=%s", sess.TurnCount, sess.LatestSHA)
	}

	ref1SHA, err := gitengine.GetRef(sess.ActiveRef(1))
	if err != nil || ref1SHA != turn1SHA {
		t.Fatalf("Turn 1 ref mismatch: expected %s, got %s", turn1SHA, ref1SHA)
	}

	turn2SHA, err := sess.RecordTurn(treeSHA, "Turn 2: Added logic")
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}
	if sess.TurnCount != 2 || sess.LatestSHA != turn2SHA {
		t.Fatalf("Session state mismatch after Turn 2: count=%d, latest=%s", sess.TurnCount, sess.LatestSHA)
	}

	parentSHA, err := gitengine.RunGit("rev-parse", turn2SHA+"^")
	if err != nil || parentSHA != turn1SHA {
		t.Fatalf("Turn 2 parent mismatch: expected %s, got %s", turn1SHA, parentSHA)
	}

	currentSHA, err := gitengine.GetRef(sess.CurrentRef())
	if err != nil || currentSHA != turn2SHA {
		t.Fatalf("Current ref mismatch: expected %s, got %s", turn2SHA, currentSHA)
	}
}
