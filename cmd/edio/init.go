package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/session"
	"github.com/devxdh/edio/pkg/ui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize edio in the repository and configure AI agent hooks",
	Long: `Sets up the .git/edio storage directory, configures automated lifecycle
hooks for supported agents (e.g. Claude Code), and writes EDIO.md operational rules.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := gitengine.EnsureGitRepo(); err != nil {
			return err
		}

		// 1. Ensure .git/edio directory exists
		edioDir, err := session.GetEdioDir()
		if err != nil {
			return fmt.Errorf("failed to initialize edio directory: %w", err)
		}
		_ = edioDir

		fmt.Println("Initialized edio shadow storage in .git/edio/")

		// 2. Configure Claude Code settings hook (.claude/settings.json)
		if err := configureClaudeHooks(); err != nil {
			fmt.Println(ui.Warning(fmt.Sprintf("failed to configure Claude hooks: %v", err)))
		} else {
			fmt.Println(ui.Bullet("Configured Claude Code lifecycle hooks (.claude/settings.json)"))
		}

		// 3. Generate EDIO.md agent instruction context
		if err := writeAgentRules(); err != nil {
			fmt.Println(ui.Warning(fmt.Sprintf("failed to generate EDIO.md: %v", err)))
		} else {
			fmt.Println(ui.Bullet("Generated EDIO.md operational guidelines for AI agents"))
		}

		fmt.Printf("\n%s\n", ui.Bold("Ready for AI agent sessions."))
		return nil
	},
}

// configureClaudeHooks injects the Stop hook into .claude/settings.json
func configureClaudeHooks() error {
	repoRoot, err := gitengine.GetRepoRoot()
	if err != nil {
		return err
	}

	claudeDir := filepath.Join(repoRoot, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return err
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	var settings map[string]any

	// Read existing settings if present
	if data, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = make(map[string]any)
	}

	// Build the hooks map
	hooksMap, ok := settings["hooks"].(map[string]any)
	if !ok {
		hooksMap = make(map[string]any)
	}

	// Standard Stop event hook: triggers snapshot on prompt completion
	stopHook := []map[string]any{
		{
			"type":    "command",
			"command": "edio snapshot -m \"prompt turn completed\"",
		},
	}

	hooksMap["Stop"] = stopHook
	settings["hooks"] = hooksMap

	// Write formatted JSON back to .claude/settings.json
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, data, 0o644)
}

// writeAgentRules generates the operational cheat-sheet context file in repo root
func writeAgentRules() error {
	repoRoot, err := gitengine.GetRepoRoot()
	if err != nil {
		return err
	}

	rulesContent := `# edio Operational Rules for AI Agents

You are operating in a codebase tracked by **edio**.
edio maintains an isolated Shadow DAG to record iterative turn progress without dirtying git log.

## Mandatory Execution Guidelines:
1. **Never run raw git commit or git add** during iterative development turns.
2. After finishing a task, sub-task, or bug fix sequence, run:
   ` + "`edio snapshot -m \"<concise description of changes>\"`" + `
3. If code breaks or tests fail, inspect prior turns via:
   ` + "`edio log`" + `
4. To revert the entire workspace to a previous working turn:
   ` + "`edio restore <turn_number>`" + `
5. When the user explicitly requests to finalize/commit the feature:
   ` + "`edio accept \"<semantic commit message>\"`" + `
`

	targetPath := filepath.Join(repoRoot, "EDIO.md")
	return os.WriteFile(targetPath, []byte(rulesContent), 0o644)
}

func init() {
	rootCmd.AddCommand(initCmd)
}
