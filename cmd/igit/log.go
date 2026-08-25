package main

import (
	"fmt"

	"github.com/devxdh/igit/pkg/gitengine"
	"github.com/devxdh/igit/pkg/session"
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
			fmt.Println("No turns recorded in the active session.")
			return nil
		}

		fmt.Printf("Session: %s (%d turns recorded)\n\n", sess.ID, sess.TurnCount)
		for _, record := range history {
			shortSHA := record.SHA
			if len(shortSHA) >= 7 {
				shortSHA = shortSHA[:7]
			}
			fmt.Printf("  ● Turn %-2d  [%s]  %s\n", record.Turn, shortSHA, record.Message)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
}
