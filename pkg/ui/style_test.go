package ui

import (
	"os"
	"strings"
	"testing"
)

func TestUIFormatting(t *testing.T) {
	// Force color off for deterministic test checking
	colorEnabled = false

	if badge := TurnBadge(3); badge != "[Turn 3]" {
		t.Errorf("expected [Turn 3], got %q", badge)
	}

	if badge := SHABadge("1a2b3c4d5e6f"); badge != "(1a2b3c4)" {
		t.Errorf("expected (1a2b3c4), got %q", badge)
	}

	if badge := BranchBadge("main"); badge != "[main]" {
		t.Errorf("expected [main], got %q", badge)
	}

	if msg := Success("done"); msg != "success: done" {
		t.Errorf("expected 'success: done', got %q", msg)
	}

	if msg := Warning("caution"); msg != "warning: caution" {
		t.Errorf("expected 'warning: caution', got %q", msg)
	}

	if msg := Error("failed"); msg != "error: failed" {
		t.Errorf("expected 'error: failed', got %q", msg)
	}

	if bullet := Bullet("file created"); !strings.Contains(bullet, "• file created") {
		t.Errorf("expected bullet with text, got %q", bullet)
	}
}

func TestCheckColorSupport(t *testing.T) {
	os.Setenv("NO_COLOR", "1")
	if checkColorSupport() {
		t.Error("expected color support to be false when NO_COLOR is set")
	}
	os.Unsetenv("NO_COLOR")
}
