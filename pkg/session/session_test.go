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

	// 1. Instantiate session
	sess := NewSession()
	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}

	// 2. Create base file state and capture initial isolated tree
	dummyFile := filepath.Join(tmpDir, "init.txt")
	if err := os.WriteFile(dummyFile, []byte("initial turn state\n"), 0o644); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}

	treeSHA1, err := gitengine.BuildIsolatedTree()
	if err != nil {
		t.Fatalf("failed to build isolated tree: %v", err)
	}

	// 3. Record Turn 1 (creates root snapshot commit)
	turn1SHA, err := sess.RecordTurn(treeSHA1, "Turn 1: initial setup")
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}

	if sess.TurnCount != 1 || sess.LatestSHA != turn1SHA {
		t.Fatalf("Turn 1 state mismatch: count=%d, latest=%s", sess.TurnCount, sess.LatestSHA)
	}

	ref1SHA, err := gitengine.GetRef(sess.ActiveRef(1))
	if err != nil || ref1SHA != turn1SHA {
		t.Fatalf("Turn 1 ref mismatch: expected %s, got %s (err: %v)", turn1SHA, ref1SHA, err)
	}

	// 4. Modify file state and capture second isolated tree
	if err := os.WriteFile(dummyFile, []byte("second turn modification\n"), 0o644); err != nil {
		t.Fatalf("failed to modify dummy file: %v", err)
	}

	treeSHA2, err := gitengine.BuildIsolatedTree()
	if err != nil {
		t.Fatalf("failed to build isolated tree for turn 2: %v", err)
	}

	// 5. Record Turn 2 (creates commit with Turn 1 as parent)
	turn2SHA, err := sess.RecordTurn(treeSHA2, "Turn 2: added logic")
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}

	if sess.TurnCount != 2 || sess.LatestSHA != turn2SHA {
		t.Fatalf("Turn 2 state mismatch: count=%d, latest=%s", sess.TurnCount, sess.LatestSHA)
	}

	// 6. Verify DAG parent link directly from Git commit object
	parentSHA, err := gitengine.RunGit("rev-parse", turn2SHA+"^")
	if err != nil || parentSHA != turn1SHA {
		t.Fatalf("Turn 2 parent mismatch: expected %s, got %s", turn1SHA, parentSHA)
	}

	// 7. Verify current pointer ref resolves to Turn 2 commit
	currentSHA, err := gitengine.GetRef(sess.CurrentRef())
	if err != nil || currentSHA != turn2SHA {
		t.Fatalf("Current ref mismatch: expected %s, got %s", turn2SHA, currentSHA)
	}
}

func TestSessionRecordTurn(t *testing.T) {
	sess := NewSession()

	// Verify recording without tree SHA fails
	_, err := sess.RecordTurn("", "empty tree test")
	if err == nil {
		t.Fatal("expected error when recording turn with empty treeSHA, got nil")
	}
}
