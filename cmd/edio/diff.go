package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/session"
	"github.com/devxdh/edio/pkg/ui"
	"github.com/spf13/cobra"
)

var diffFilePath string

var diffCmd = &cobra.Command{
	Use:   "diff [turn_number]",
	Short: "Inspect syntax-highlighted turn diffs using delta or standard git pager",
	Long: `Renders turn diffs comparing Turn N against Turn N-1.
Delegates presentation to 'delta' if installed, or your standard Git $PAGER / less.

Examples:
  edio diff              Diff for the latest turn
  edio diff 3            Diff for Turn 3
  edio diff 3 -f app.go  Diff for a single file in Turn 3`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := gitengine.EnsureGitRepo(); err != nil {
			return err
		}

		sess, err := session.LoadActiveSession()
		if err != nil {
			return fmt.Errorf("failed to load active session: %w", err)
		}

		if sess.TurnCount == 0 {
			fmt.Println("No turns recorded in active session.")
			return nil
		}

		targetTurn := sess.TurnCount
		if len(args) > 0 {
			t, err := strconv.Atoi(args[0])
			if err != nil || t < 1 {
				return fmt.Errorf("invalid turn number %q (must be >= 1)", args[0])
			}
			targetTurn = t
		}

		if targetTurn > sess.TurnCount {
			return fmt.Errorf("turn %d does not exist (session has %d turns)", targetTurn, sess.TurnCount)
		}

		targetRef := sess.ActiveRef(targetTurn)
		targetSHA, err := gitengine.GetRef(targetRef)
		if err != nil || targetSHA == "" {
			return fmt.Errorf("failed to resolve ref for turn %d: %w", targetTurn, err)
		}

		parentSHA := "HEAD"
		if targetTurn > 1 {
			parentRef := sess.ActiveRef(targetTurn - 1)
			pSHA, err := gitengine.GetRef(parentRef)
			if err == nil && pSHA != "" {
				parentSHA = pSHA
			}
		}

		// Prepare git diff arguments
		gitArgs := []string{"diff", "--color=always", parentSHA, targetSHA}
		if diffFilePath != "" {
			gitArgs = append(gitArgs, "--", diffFilePath)
		}

		// Check if delta is installed in system PATH
		deltaBin, err := exec.LookPath("delta")
		if err == nil && deltaBin != "" {
			// Pipe git diff -> delta pager
			gitCmd := exec.Command("git", gitArgs...)
			deltaCmd := exec.Command(deltaBin)

			pipeReader, pipeWriter, err := os.Pipe()
			if err != nil {
				return err
			}

			gitCmd.Stdout = pipeWriter
			gitCmd.Stderr = os.Stderr

			deltaCmd.Stdin = pipeReader
			deltaCmd.Stdout = os.Stdout
			deltaCmd.Stderr = os.Stderr

			if err := gitCmd.Start(); err != nil {
				return err
			}
			if err := deltaCmd.Start(); err != nil {
				return err
			}

			_ = gitCmd.Wait()
			_ = pipeWriter.Close()
			return deltaCmd.Wait()
		}

		// Fallback to direct git diff with standard $PAGER
		fmt.Println(ui.Header(fmt.Sprintf("Diff for %s %s:", ui.TurnBadge(targetTurn), ui.SHABadge(targetSHA))))
		gitCmd := exec.Command("git", gitArgs...)
		gitCmd.Stdin = os.Stdin
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		return gitCmd.Run()
	},
}

func init() {
	diffCmd.Flags().StringVarP(&diffFilePath, "file", "f", "", "Filter diff for a specific file path")
	rootCmd.AddCommand(diffCmd)
}
