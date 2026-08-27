package main

import (
	"fmt"
	"time"

	"github.com/devxdh/edio/pkg/session"
	"github.com/devxdh/edio/pkg/ui"
	"github.com/spf13/cobra"
)

var gcDays int

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Clean up shadow sessions older than retention limit (default: 10 days)",
	Long: `Scans active and archived shadow sessions in refs/edio/* and permanently
prunes sessions whose latest turn commit timestamp is older than the retention period.
Protects the currently active session from being pruned.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if gcDays < 1 {
			return fmt.Errorf("retention days must be at least 1")
		}

		ttl := time.Duration(gcDays) * 24 * time.Hour
		fmt.Printf("Scanning and pruning shadow sessions older than %d days...\n", gcDays)

		report, err := session.PruneExpiredSessions(ttl)
		if err != nil {
			return fmt.Errorf("garbage collection failed: %w", err)
		}

		if report.DeletedSessions == 0 {
			fmt.Println(ui.Success("All sessions are within retention limit. Zero sessions pruned."))
		} else {
			fmt.Println(ui.Success(fmt.Sprintf("Cleaned up %d expired session(s) (%d references deleted, Git unreachable objects pruned).",
				report.DeletedSessions, report.DeletedRefs)))
		}

		return nil
	},
}

func init() {
	gcCmd.Flags().IntVarP(&gcDays, "days", "d", 10, "Retention limit in days (default: 10)")
	rootCmd.AddCommand(gcCmd)
}
