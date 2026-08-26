package session

import (
	"fmt"
	"os"
	"time"

	"github.com/devxdh/edio/pkg/gitengine"
)

// Archive moves the active session references to refs/edio/archive/
// and cleans up the active session state file.
func (s *Session) Archive() error {
	if s == nil || s.ID == "" {
		return nil
	}

	// Archive latest turn commit pointer
	archiveRef := fmt.Sprintf("refs/edio/archive/%d_%s", time.Now().Unix(), s.ID)
	if s.LatestSHA != "" {
		if err := gitengine.UpdateRef(archiveRef, s.LatestSHA); err != nil {
			return fmt.Errorf("failed to write archive ref: %w", err)
		}
	}

	// Delete active session file from disk
	filePath, err := ActiveSessionPath()
	if err == nil {
		_ = os.Remove(filePath)
	}

	return nil
}
