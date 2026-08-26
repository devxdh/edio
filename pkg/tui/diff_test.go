package tui

import (
	"strings"
	"testing"
)

func TestFormatDiff_BinaryGuardrail(t *testing.T) {
	binaryDiff := "diff --git a/image.png b/image.png\nBinary files differ"
	formatted := FormatDiff(binaryDiff)
	if !strings.Contains(formatted, "Binary file changes detected") {
		t.Fatalf("expected binary detection warning, got %q", formatted)
	}

	gitBinaryDiff := "GIT binary patch\nliteral 1024..."
	formatted2 := FormatDiff(gitBinaryDiff)
	if !strings.Contains(formatted2, "Binary file changes detected") {
		t.Fatalf("expected binary detection warning for git binary patch, got %q", formatted2)
	}
}

func TestFormatDiff_LargePayloadGuardrail(t *testing.T) {
	// Create a large payload > 200KB
	largeDiff := strings.Repeat("+ line of code with some content\n", 8000)
	if len(largeDiff) <= maxDiffLength {
		t.Fatalf("expected largeDiff > %d bytes, got %d", maxDiffLength, len(largeDiff))
	}

	formatted := FormatDiff(largeDiff)
	if !strings.Contains(formatted, "Diff payload exceeds 200KB") {
		t.Fatalf("expected payload truncation warning, got %q", formatted[:200])
	}
}

func TestFormatDiff_CleanFormatting(t *testing.T) {
	sampleDiff := `diff --git a/main.go b/main.go
index 1234567..89abcdef 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@ func main()
 package main
 
+func Health() string { return "OK" }
-func Dead() string { return "NO" }
`
	formatted := FormatDiff(sampleDiff)

	// Invariants:
	// 1. Git plumbing stripped
	if strings.Contains(formatted, "diff --git") || strings.Contains(formatted, "index ") || strings.Contains(formatted, "--- a/") {
		t.Fatalf("git plumbing was not stripped: %s", formatted)
	}

	// 2. Cyan File Header present
	if !strings.Contains(formatted, "▾ main.go") {
		t.Fatalf("missing file header in formatted diff: %s", formatted)
	}

	// 3. Additions and deletions have background tags
	if !strings.Contains(formatted, "[:#16331c]") {
		t.Fatalf("missing addition background tag: %s", formatted)
	}
	if !strings.Contains(formatted, "[:#3d1a11]") {
		t.Fatalf("missing deletion background tag: %s", formatted)
	}
}
