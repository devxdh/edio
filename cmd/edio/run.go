package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/session"
	"github.com/devxdh/edio/pkg/ui"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [flags] <command> [args...]",
	Short: "Execute an AI agent command and snapshot workspace upon completion",
	Long: `Wraps any agent or CLI command with interactive terminal I/O pass-through.
Automatically records an isolated shadow snapshot when the command finishes.

Examples:
  edio run claude "refactor auth package"
  edio run aider --message "add tests"
  edio run -- npm test`,
	DisableFlagParsing: true, // Passes all raw arguments to the child process
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := gitengine.EnsureGitRepo(); err != nil {
			return err
		}

		// Normalize arguments: strip leading "--" if passed
		cmdArgs := args
		if len(cmdArgs) > 0 && cmdArgs[0] == "--" {
			cmdArgs = cmdArgs[1:]
		}

		if len(cmdArgs) == 0 {
			return fmt.Errorf("usage: edio run <command> [args...]")
		}

		// Prepare child process execution with interactive stdio
		childName := cmdArgs[0]
		var childArgs []string
		if len(cmdArgs) > 1 {
			childArgs = cmdArgs[1:]
		}

		childCmd := exec.Command(childName, childArgs...)
		childCmd.Stdin = os.Stdin
		childCmd.Stdout = os.Stdout
		childCmd.Stderr = os.Stderr

		// Execute child command interactively
		execErr := childCmd.Run()

		// Attempt snapshot regardless of exit code to preserve state
		sess, err := session.LoadActiveSession()
		if err != nil {
			return fmt.Errorf("failed to load session for snapshot: %w", err)
		}

		treeSHA, err := gitengine.BuildIsolatedTree()
		if err != nil {
			return fmt.Errorf("failed to build isolated tree: %w", err)
		}

		snapshotSummary := fmt.Sprintf("run: %s", strings.Join(cmdArgs, " "))
		commitSHA, err := sess.RecordTurn(treeSHA, snapshotSummary)
		if err != nil {
			return fmt.Errorf("failed to record turn: %w", err)
		}

		if err := session.SaveActiveSession(sess); err != nil {
			return fmt.Errorf("failed to persist session: %w", err)
		}

		fmt.Printf("\n%s %s auto-snapshot recorded %s\n", ui.TurnBadge(sess.TurnCount), ui.SHABadge(commitSHA), ui.Dim(fmt.Sprintf("(run: %s)", strings.Join(cmdArgs, " "))))

		if execErr != nil {
			return fmt.Errorf("command exited with error: %w", execErr)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
