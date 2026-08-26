package main

import (
	"fmt"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/session"
	"github.com/devxdh/edio/pkg/ui"
	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Display the turn history for the active session",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := gitengine.EnsureGitRepo(); err != nil {
			return err
		}

		sess, err := session.LoadActiveSession()
		if err != nil {
			return fmt.Errorf("failed to load active session: %w", err)
		}

		history, err := sess.GetTurnHistory()
		if err != nil {
			return fmt.Errorf("failed to retrieve history: %w", err)
		}

		if len(history) == 0 {
			fmt.Println("No turns recorded in active session.")
			return nil
		}

		fmt.Printf("Session %s (%d turns)\n\n", ui.Bold(sess.ID), sess.TurnCount)
		for _, record := range history {
			fmt.Printf("* %s %s %s\n", ui.TurnBadge(record.Turn), ui.SHABadge(record.SHA), record.Message)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
}
