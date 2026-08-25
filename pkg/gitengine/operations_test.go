package gitengine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devxdh/igit/pkg/testutil"
)

func TestZeroPollution(t *testing.T) {
	testutil.SetupTestRepo(t, "igit-operations-test-*")

	// 1. Capture baseline HEAD commit before any shadow turns are recorded
	baselineHeadSHA, err := GetRef("HEAD")
	if err != nil || baselineHeadSHA == "" {
		t.Fatalf("failed to get baseline HEAD SHA: %v", err)
	}

	// 2. Simulate 5 shadow turns by creating blobs, trees, and custom refs
	var parentSHA string
	for turn := range 5 {
		srcFile := "main.go"
		content := fmt.Sprintf("package main\n\n// Turn %d edits\nfunc Run() {}\n", turn)
		if err := os.WriteFile(srcFile, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write source file: %v", err)
		}

		blobSHA, err := HashBlob(srcFile)
		if err != nil || len(blobSHA) != 40 {
			t.Fatalf("[TURN #%d] HashBlob failed: %v", turn, err)
		}

		if _, err := RunGit("add", srcFile); err != nil {
			t.Fatalf("[TURN #%d] add failed: %v", turn, err)
		}

		treeSHA, err := RunGit("write-tree")
		if err != nil {
			t.Fatalf("[TURN #%d] write-tree failed: %v", turn, err)
		}

		commitSHA, err := CommitTree(treeSHA, parentSHA, fmt.Sprintf("turn %d snapshot", turn))
		if err != nil || len(commitSHA) != 40 {
			t.Fatalf("[TURN #%d] CommitTree failed: %v", turn, err)
		}

		shadowRef := fmt.Sprintf("refs/igit/sessions/sess_alpha/%d", turn)
		if err := UpdateRef(shadowRef, commitSHA); err != nil {
			t.Fatalf("turn %d: UpdateRef failed: %v", turn, err)
		}

		parentSHA = commitSHA
	}

	// 3. Verify HEAD was not moved during the entire process
	currentHeadSHA, err := GetRef("HEAD")
	if err != nil {
		t.Fatalf("failed to query HEAD: %v", err)
	}

	if currentHeadSHA != baselineHeadSHA {
		t.Fatalf("POLLUTION VIOLATION: HEAD moved from %s to %s", baselineHeadSHA, currentHeadSHA)
	}

	// 4. Verify each individual turn ref exists on disk and resolves to a valid SHA
	for turn := range 5 {
		ref := fmt.Sprintf("refs/igit/sessions/sess_alpha/%d", turn)
		sha, err := GetRef(ref)
		if err != nil || len(sha) != 40 {
			t.Errorf("turn ref %s missing or invalid: %s (err: %v)", ref, sha, err)
		}
	}
}

func BenchmarkTurnSnapshotLatency(b *testing.B) {
	tmpDir := testutil.SetupTestRepo(b, "igit-operations-bench-*")

	testFile := filepath.Join(tmpDir, "index.ts")
	if err := os.WriteFile(testFile, []byte("console.log('Hello from Benchmark');"), 0o644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}
	if _, err := RunGit("add", "index.ts"); err != nil {
		b.Fatalf("failed to stage test file: %v", err)
	}
	treeSHA, err := RunGit("write-tree")
	if err != nil {
		b.Fatalf("failed to write initial tree: %v", err)
	}

	b.ResetTimer()
	for i := range b.N {
		start := time.Now()

		if _, err := HashBlob(testFile); err != nil {
			b.Fatalf("HashBlob failed: %v", err)
		}

		commitSHA, err := CommitTree(treeSHA, "", fmt.Sprintf("bench turn %d", i))
		if err != nil {
			b.Fatalf("CommitTree failed: %v", err)
		}

		refName := fmt.Sprintf("refs/igit/bench/%d", i)
		if err := UpdateRef(refName, commitSHA); err != nil {
			b.Fatalf("UpdateRef failed: %v", err)
		}

		duration := time.Since(start)
		if duration > 10*time.Millisecond {
			b.Logf("Warning: iteration %d exceeded 10ms target: %v", i, duration)
		}
	}
}
