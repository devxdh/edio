package main

import (
	"fmt"
	"os"

	"github.com/devxdh/edio/pkg/ui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "edio",
	Short: "edio: Shadow DAG version control for AI coding agents",
	Long: `edio is a high-speed, branchless shadow version control tool for AI agents.
It captures turn-by-turn workspace state in isolated Git namespaces without
polluting your staging index or git commit history.`,
	Example: `  edio init                     Configure agent hooks in repository
  edio snapshot -m "added JWT"  Record an isolated turn snapshot
  edio run claude "fix bug"     Execute agent and auto-snapshot on exit
  edio log                      Display active turn history
  edio diff 2                   View syntax-highlighted diff for Turn 2
  edio restore 2                Restore workspace to Turn 2 state
  edio accept "feat: add auth"  Squash active turns into a commit on main`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}
}
