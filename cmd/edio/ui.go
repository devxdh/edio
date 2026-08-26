package main

import (
	"fmt"

	"github.com/devxdh/edio/pkg/tui"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:     "ui",
	Aliases: []string{"tui", "dashboard"},
	Short:   "Launch interactive split-pane terminal user interface",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUI()
	},
}

func runUI() error {
	if err := tui.Launch(); err != nil {
		return fmt.Errorf("error launching edio UI: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
