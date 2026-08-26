package main

import (
	"fmt"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/session"
	"github.com/devxdh/edio/pkg/ui"
	"github.com/spf13/cobra"
)

var snapshotMsg string

var snapshotCmd = &cobra.Command{
	Use:     "snapshot",
	Aliases: []string{"snap", "record"},
	Short:   "Record a non-polluting shadow turn snapshot",
	Long: `Captures all working tree changes into an isolated Git Tree object
and creates a shadow turn commit linked into the active session DAG.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Verify we are in a Git repo
		if err := gitengine.EnsureGitRepo(); err != nil {
			return err
		}

		// Load or bootstrap the active session
		sess, err := session.LoadActiveSession()
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}

		// Create isolated Tree without touching user's index/staging
		treeSHA, err := gitengine.BuildIsolatedTree()
		if err != nil {
			return fmt.Errorf("failed to build isolated tree: %w", err)
		}

		// Record turn commit in refs/edio/active/*
		commitSHA, err := sess.RecordTurn(treeSHA, snapshotMsg)
		if err != nil {
			return fmt.Errorf("failed to record turn: %w", err)
		}

		// Persist the updated session state
		if err := session.SaveActiveSession(sess); err != nil {
			return fmt.Errorf("failed to persist session: %w", err)
		}

		if snapshotMsg != "" {
			fmt.Printf("%s %s snapshot recorded: %s\n", ui.TurnBadge(sess.TurnCount), ui.SHABadge(commitSHA), snapshotMsg)
		} else {
			fmt.Printf("%s %s snapshot recorded\n", ui.TurnBadge(sess.TurnCount), ui.SHABadge(commitSHA))
		}
		return nil
	},
}

func init() {
	snapshotCmd.Flags().StringVarP(&snapshotMsg, "message", "m", "", "Summary description of the turn")
	rootCmd.AddCommand(snapshotCmd)
}
