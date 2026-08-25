package main

import (
	"fmt"

	"github.com/devxdh/igit/pkg/gitengine"
	"github.com/devxdh/igit/pkg/session"
	"github.com/spf13/cobra"
)

var acceptCmd = &cobra.Command{
	Use:   "accept <commit_message>",
	Short: "Promote latest session snapshot into a clean commit on current branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		commitMsg := args[0]

		if err := gitengine.EnsureGitRepo(); err != nil {
			return err
		}

		sess, err := session.LoadActiveSession()
		if err != nil {
			return fmt.Errorf("failed to load active session: %w", err)
		}

		if sess.TurnCount == 0 || sess.LatestSHA == "" {
			return fmt.Errorf("cannot accept: active session has no recorded turns")
		}

		// Get the tree SHA associated with the latest turn commit
		latestTreeSHA, err := gitengine.RunGit("rev-parse", sess.LatestSHA+"^{tree}")
		if err != nil {
			return fmt.Errorf("failed to extract tree from turn snapshot: %w", err)
		}

		// Resolve current HEAD commit SHA (parent for the official commit)
		headSHA, err := gitengine.GetRef("HEAD")
		if err != nil {
			return fmt.Errorf("failed to resolve HEAD: %w", err)
		}

		// Create the official clean commit on the working branch
		officialCommitSHA, err := gitengine.CommitTree(latestTreeSHA, headSHA, commitMsg)
		if err != nil {
			return fmt.Errorf("failed to create official commit: %w", err)
		}

		// Advance the active branch (or HEAD) to the new commit
		// "git merge-base" or simple update-ref HEAD updates current branch safely
		currentBranch, err := gitengine.RunGit("symbolic-ref", "--short", "HEAD")
		if err == nil && currentBranch != "" {
			branchRef := fmt.Sprintf("refs/heads/%s", currentBranch)
			if err := gitengine.UpdateRef(branchRef, officialCommitSHA); err != nil {
				return fmt.Errorf("failed to advance branch %s: %w", currentBranch, err)
			}
		} else {
			// Detached HEAD fallback
			if err := gitengine.UpdateRef("HEAD", officialCommitSHA); err != nil {
				return fmt.Errorf("failed to advance HEAD: %w", err)
			}
		}

		// Update index/worktree to match the new HEAD commit cleanly
		_, err = gitengine.RunGit("read-tree", "HEAD")
		if err != nil {
			return fmt.Errorf("failed to sync staging index with HEAD: %w", err)
		}

		// Archive session & cleanup
		if err := sess.Archive(); err != nil {
			return fmt.Errorf("warning: failed to archive session: %w", err)
		}

		shortSHA := officialCommitSHA
		if len(shortSHA) >= 7 {
			shortSHA = shortSHA[:7]
		}

		fmt.Printf("✔ Session accepted: %d turns squashed into commit %s\n", sess.TurnCount, shortSHA)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(acceptCmd)
}
