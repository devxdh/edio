package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/devxdh/edio/pkg/testutil"
)

func TestConfigureClaudeHooks(t *testing.T) {
	tmpDir := testutil.SetupTestRepo(t, "edio-init-test-*")

	// Pre-create .claude/settings.local.json with an existing custom hook
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}

	settingsLocalPath := filepath.Join(claudeDir, "settings.local.json")
	initialSettings := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"type":    "command",
					"command": "echo 'custom user linter'",
				},
			},
		},
	}
	initialData, _ := json.MarshalIndent(initialSettings, "", "  ")
	if err := os.WriteFile(settingsLocalPath, initialData, 0o644); err != nil {
		t.Fatalf("failed to write initial settings: %v", err)
	}

	// 1. Run configureClaudeHooks
	if err := configureClaudeHooks(); err != nil {
		t.Fatalf("configureClaudeHooks failed: %v", err)
	}

	// Read and verify settings.local.json
	data, err := os.ReadFile(settingsLocalPath)
	if err != nil {
		t.Fatalf("failed to read settings.local.json: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	hooksMap, ok := parsed["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks map missing")
	}

	stopList, ok := hooksMap["Stop"].([]any)
	if !ok || len(stopList) != 2 {
		t.Fatalf("expected 2 hooks in Stop list, got %d (list: %v)", len(stopList), stopList)
	}

	// Verify first hook is the preserved custom hook
	hook1, ok := stopList[0].(map[string]any)
	if !ok || hook1["command"] != "echo 'custom user linter'" {
		t.Fatalf("custom hook was modified or overwritten: %v", hook1)
	}

	// Verify second hook is edio snapshot
	hook2, ok := stopList[1].(map[string]any)
	if !ok || hook2["command"] != "edio snapshot -m \"prompt turn completed\"" {
		t.Fatalf("edio hook missing or wrong: %v", hook2)
	}

	// 2. Test Idempotency: run configureClaudeHooks again
	if err := configureClaudeHooks(); err != nil {
		t.Fatalf("second configureClaudeHooks failed: %v", err)
	}

	data2, _ := os.ReadFile(settingsLocalPath)
	var parsed2 map[string]any
	_ = json.Unmarshal(data2, &parsed2)
	hooksMap2 := parsed2["hooks"].(map[string]any)
	stopList2 := hooksMap2["Stop"].([]any)
	if len(stopList2) != 2 {
		t.Fatalf("expected still 2 hooks after second run (idempotent), got %d", len(stopList2))
	}
}
