package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/testutil"
)

func TestPruneExpiredSessions(t *testing.T) {
	tmpDir := testutil.SetupTestRepo(t, "edio-gc-test-*")

	// 1. Create a session and record turns
	sess1 := NewSession()
	dummyFile := filepath.Join(tmpDir, "file1.txt")
	_ = os.WriteFile(dummyFile, []byte("content 1\n"), 0o644)
	tree1, _ := gitengine.BuildIsolatedTree()
	_, err := sess1.RecordTurn(tree1, "turn 1")
	if err != nil {
		t.Fatalf("failed to record turn in sess1: %v", err)
	}

	// Archive sess1
	if err := sess1.Archive(); err != nil {
		t.Fatalf("failed to archive sess1: %v", err)
	}

	// 2. Create an active session sess2
	sess2 := NewSession()
	if err := SaveActiveSession(sess2); err != nil {
		t.Fatalf("failed to save active sess2: %v", err)
	}
	_ = os.WriteFile(dummyFile, []byte("content 2\n"), 0o644)
	tree2, _ := gitengine.BuildIsolatedTree()
	_, err = sess2.RecordTurn(tree2, "turn 2")
	if err != nil {
		t.Fatalf("failed to record turn in sess2: %v", err)
	}

	// Test 1: With large TTL (e.g. 10 days), nothing should be pruned
	report1, err := PruneExpiredSessions(10 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("PruneExpiredSessions failed: %v", err)
	}
	if report1.DeletedSessions != 0 || report1.DeletedRefs != 0 {
		t.Fatalf("expected 0 deleted sessions with 10d TTL, got %d", report1.DeletedSessions)
	}

	// Test 2: With 0 TTL (all non-active expired), archived sess1 should be pruned, while active sess2 is protected
	report2, err := PruneExpiredSessions(0)
	if err != nil {
		t.Fatalf("PruneExpiredSessions with 0 TTL failed: %v", err)
	}
	if report2.DeletedSessions != 1 {
		t.Fatalf("expected 1 deleted session (sess1), got %d", report2.DeletedSessions)
	}

	// Verify active sess2 is still intact and reachable
	activeRefSHA, err := gitengine.GetRef(sess2.CurrentRef())
	if err != nil || activeRefSHA == "" {
		t.Fatalf("active session sess2 was improperly deleted: %v", err)
	}
}
