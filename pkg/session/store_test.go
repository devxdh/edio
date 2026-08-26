package session

import (
	"os"
	"testing"

	"github.com/devxdh/edio/pkg/testutil"
)

func TestStore_BootstrapAndPersist(t *testing.T) {
	testutil.SetupTestRepo(t, "edio-store-test-*")

	// Inital load must auto-create a new active session
	sess1, err := LoadActiveSession()
	if err != nil {
		t.Fatalf("LoadActiveSession failed: %v", err)
	}

	if sess1.ID == "" || sess1.TurnCount != 0 || sess1.LatestSHA != "" {
		t.Fatalf("unexpected bootstrapped session state: %+v", sess1)
	}

	// Verify file exists on disk
	sessionFilePath, err := ActiveSessionPath()
	if err != nil {
		t.Fatalf("ActiveSessionPath failed: %v", err)
	}
	if _, err := os.Stat(sessionFilePath); err != nil {
		t.Fatalf("active session file does not exist on disk: %v", err)
	}

	// Mutate session state and persist
	sess1.TurnCount = 3
	sess1.LatestSHA = "1111222233334444555566667777888899990000"

	if err := SaveActiveSession(sess1); err != nil {
		t.Fatalf("SaveActiveSession failed: %v", err)
	}

	// Load again to simulate a separate process execution
	sess2, err := LoadActiveSession()
	if err != nil {
		t.Fatalf("second LoadActiveSession failed: %v", err)
	}

	// Invariant: Loaded state must match what was saved
	if sess2.ID != sess1.ID {
		t.Fatalf("session ID mismatch: expected %s, got %s", sess1.ID, sess2.ID)
	}
	if sess2.TurnCount != 3 {
		t.Fatalf("turn count mismatch: expected 3, got %d", sess2.TurnCount)
	}
	if sess2.LatestSHA != sess1.LatestSHA {
		t.Fatalf("latest SHA mismatch: expected %s, got %s", sess1.LatestSHA, sess2.LatestSHA)
	}
}

func TestStore_SaveNilSession(t *testing.T) {
	testutil.SetupTestRepo(t, "edio-store-nil-*")

	err := SaveActiveSession(nil)
	if err == nil {
		t.Fatal("expected error when saving nil session, got nil")
	}
}
