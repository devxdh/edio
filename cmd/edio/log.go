package main

import (
	"fmt"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/session"
	"github.com/devxdh/edio/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	logShowPatch bool
	logLimit     int
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

		displayCount := len(history)
		if logLimit > 0 && logLimit < displayCount {
			displayCount = logLimit
		}

		fmt.Printf("Session %s (%d turns)\n\n", ui.Bold(sess.ID), sess.TurnCount)

		for i := 0; i < displayCount; i++ {
			record := history[i]
			fmt.Printf("* %s %s %s\n", ui.TurnBadge(record.Turn), ui.SHABadge(record.SHA), record.Message)

			if logShowPatch {
				parentSHA := "HEAD"
				if record.Turn > 1 {
					parentRef := sess.ActiveRef(record.Turn - 1)
					pSHA, err := gitengine.GetRef(parentRef)
					if err == nil && pSHA != "" {
						parentSHA = pSHA
					}
				}
				patchDiff, err := gitengine.RunGit("diff", parentSHA, record.SHA)
				if err == nil && patchDiff != "" {
					fmt.Println(ui.Dim(patchDiff))
				}
			}
		}
		fmt.Println()
		return nil
	},
}

func init() {
	logCmd.Flags().BoolVarP(&logShowPatch, "patch", "p", false, "Show patch diff for each turn")
	logCmd.Flags().IntVarP(&logLimit, "number", "n", 0, "Limit maximum number of turns to display")
	rootCmd.AddCommand(logCmd)
}
