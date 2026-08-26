package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "edio",
	Short: "edio: Shadow version control & time-machine for AI coding agents",
	Long: `edio creates isolated, non-polluting turn snapshots of your workspace
during AI agent workflows without dirtying your git staging index or branch history.`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
