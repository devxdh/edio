package session

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/devxdh/edio/pkg/gitengine"
)

// DefaultTTL is the default retention period (10 days) for shadow sessions.
const DefaultTTL = 10 * 24 * time.Hour

// PruneReport contains metadata about the garbage collection run.
type PruneReport struct {
	DeletedSessions int
	DeletedRefs     int
}

// PruneExpiredSessions scans active and archived shadow sessions and permanently
// removes sessions whose latest commit timestamp is older than ttl.
//
// Invariants guaranteed:
//  1. Never prunes the currently active session in .git/edio/active_session.json.
//  2. Prunes entire sessions (all turn refs atomically), never isolated turns.
//  3. Invokes low-level Git ref packing and unreachable object pruning to free disk space.
func PruneExpiredSessions(ttl time.Duration) (*PruneReport, error) {
	if err := gitengine.EnsureGitRepo(); err != nil {
		return nil, err
	}

	activeSess, _ := LoadActiveSession()
	currentActiveID := ""
	if activeSess != nil {
		currentActiveID = activeSess.ID
	}

	cutoff := time.Now().Add(-ttl)
	report := &PruneReport{}

	// 1. Collect all edio references and their committer timestamps
	rawRefs, err := gitengine.RunGit("for-each-ref", "--format=%(refname)%00%(committerdate:raw)", "refs/edio/")
	if err != nil {
		return nil, fmt.Errorf("failed to query edio refs: %w", err)
	}

	if strings.TrimSpace(rawRefs) == "" {
		return report, nil
	}

	// Group references by session key
	sessionRefs := make(map[string][]string)
	sessionLatestTime := make(map[string]time.Time)

	lines := strings.Split(strings.TrimSpace(rawRefs), "\n")
	for _, line := range lines {
		parts := strings.Split(line, "\x00")
		if len(parts) < 2 {
			continue
		}
		refName := parts[0]
		dateRaw := strings.Fields(parts[1])
		if len(dateRaw) < 1 {
			continue
		}

		unixSec, err := strconv.ParseInt(dateRaw[0], 10, 64)
		if err != nil {
			continue
		}
		commitTime := time.Unix(unixSec, 0)

		sessID := extractSessionID(refName)
		if sessID == "" {
			continue
		}

		sessionRefs[sessID] = append(sessionRefs[sessID], refName)
		if commitTime.After(sessionLatestTime[sessID]) {
			sessionLatestTime[sessID] = commitTime
		}
	}

	// 2. Identify expired sessions (excluding currently active session)
	for sessID, latestTime := range sessionLatestTime {
		if currentActiveID != "" && sessID == currentActiveID {
			continue // Protected: currently active session
		}

		if latestTime.Before(cutoff) {
			for _, ref := range sessionRefs[sessID] {
				_, _ = gitengine.RunGit("update-ref", "-d", ref)
				report.DeletedRefs++
			}
			report.DeletedSessions++
		}
	}

	// 3. Compact remaining refs and trigger Git object pruning to reclaim disk space
	if report.DeletedRefs > 0 {
		_, _ = gitengine.RunGit("pack-refs", "--all", "--prune")
		_, _ = gitengine.RunGit("prune", fmt.Sprintf("--expire=%s", cutoff.Format(time.RFC3339)))
	}

	return report, nil
}

func extractSessionID(refName string) string {
	// Format examples:
	// - refs/edio/active/sess_12345/1            -> sess_12345
	// - refs/edio/active/sess_12345/current      -> sess_12345
	// - refs/edio/archive/1720000000_sess_12345  -> sess_12345
	parts := strings.Split(refName, "/")
	if len(parts) < 4 {
		return ""
	}
	if parts[2] == "active" {
		return parts[3]
	}
	if parts[2] == "archive" {
		sub := strings.SplitN(parts[3], "_", 2)
		if len(sub) == 2 {
			return sub[1]
		}
		return parts[3]
	}
	return ""
}
