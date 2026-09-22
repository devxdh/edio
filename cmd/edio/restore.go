package main

import (
	"fmt"
	"strconv"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/session"
	"github.com/devxdh/edio/pkg/ui"
	"github.com/spf13/cobra"
)

var restoreFilePath string

var restoreCmd = &cobra.Command{
	Use:   "restore <turn_number>",
	Short: "Restore working tree or a single file to a specific turn snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := gitengine.EnsureGitRepo(); err != nil {
			return err
		}

		targetTurn, err := strconv.Atoi(args[0])
		if err != nil || targetTurn < 1 {
			return fmt.Errorf("invalid turn number: %q (must be >= 1)", args[0])
		}

		sess, err := session.LoadActiveSession()
		if err != nil {
			return fmt.Errorf("failed to load active session: %w", err)
		}

		targetSHA, err := sess.Restore(targetTurn, restoreFilePath)
		if err != nil {
			return err
		}

		if restoreFilePath != "" {
			fmt.Printf(
				"Restored %s from %s %s\n",
				ui.Bold(restoreFilePath),
				ui.TurnBadge(targetTurn),
				ui.SHABadge(targetSHA),
			)
			return nil
		}

		fmt.Printf("Restored workspace to %s %s\n", ui.TurnBadge(targetTurn), ui.SHABadge(targetSHA))
		return nil
	},
}

func init() {
	restoreCmd.Flags().StringVarP(
		&restoreFilePath,
		"file",
		"f",
		"",
		"Restore only this specific file path",
	)
	rootCmd.AddCommand(restoreCmd)
}
