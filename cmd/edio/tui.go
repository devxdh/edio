package main

import (
	"fmt"

	"github.com/devxdh/edio/pkg/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:     "tui",
	Aliases: []string{"ui", "dashboard"},
	Short:   "Launch interactive shadow version control split-pane TUI dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI()
	},
}

func runTUI() error {
	if err := tui.Launch(); err != nil {
		return fmt.Errorf("error launching edio TUI: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
