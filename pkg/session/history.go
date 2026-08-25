package session

import (
	"fmt"
	"strings"

	"github.com/devxdh/igit/pkg/gitengine"
)

// TurnRecord holds metadata about a single snapshot turn.
type TurnRecord struct {
	Turn    int
	SHA     string
	Message string
}

// GetTurnHistory reads and returns all recorded turns for the active session in order.
func (s *Session) GetTurnHistory() ([]TurnRecord, error) {
	if s == nil || s.TurnCount <= 0 {
		return []TurnRecord{}, nil
	}

	history := make([]TurnRecord, 0, s.TurnCount)

	for turn := 1; turn <= s.TurnCount; turn++ {
		ref := s.ActiveRef(turn)
		sha, err := gitengine.GetRef(ref)
		if err != nil {
			return nil, fmt.Errorf("git error resolving ref for turn %d: %w", turn, err)
		}
		if sha == "" {
			return nil, fmt.Errorf("ref not found for turn %d (%s)", turn, ref)
		}

		// Read commit subject line via git log plumbing
		msg, err := gitengine.RunGit("log", "-1", "--format=%s", sha)
		if err != nil {
			msg = fmt.Sprintf("turn %d snapshot", turn)
		}

		history = append(history, TurnRecord{
			Turn:    turn,
			SHA:     sha,
			Message: strings.TrimSpace(msg),
		})
	}

	return history, nil
}
