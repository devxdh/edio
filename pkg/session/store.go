package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devxdh/igit/pkg/gitengine"
)

const (
	// IgitDirName is the directory inside .git where igit metadata is stored.
	IgitDirName = "igit"

	// ActiveSessionFileName is the JSON file tracking the currently active session.
	ActiveSessionFileName = "active_session.json"
)

// GetIgitDir returns the absolute path to the .git/igit directory,
// creating it if it does not already exist.
func GetIgitDir() (string, error) {
	if err := gitengine.EnsureGitRepo(); err != nil {
		return "", err
	}

	gitDir, err := gitengine.RunGit("rev-parse", "--git-dir")
	if err != nil {
		return "", fmt.Errorf("failed to locate git directory: %w", err)
	}

	// Resolves absolute path in case gitDir is relative (e.g. ".git")
	absGitDir, err := filepath.Abs(gitDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute git directory: %w", err)
	}

	igitDir := filepath.Join(absGitDir, IgitDirName)
	if err := os.MkdirAll(igitDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create igit directory: %w", err)
	}

	return igitDir, nil
}

// ActiveSessionPath returns the absolute path to .git/igit/active_session.json.
func ActiveSessionPath() (string, error) {
	igitDir, err := GetIgitDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(igitDir, ActiveSessionFileName), nil
}

// SaveActiveSession writes the Session state to .git/igit/active_session.json.
//
// It uses an atomic write pattern (writing to a temp file and renaming)
// to prevent data corruption if the process is terminated mid-write.
func SaveActiveSession(sess *Session) error {
	if sess == nil {
		return errors.New("failed to save session: session doesn't exist")
	}

	targetPath, err := ActiveSessionPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(sess, "", " ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Write to temporary file in the same directory for atomic rename
	tempPath := fmt.Sprintf("%s.tmp.%d", targetPath, os.Getpid())
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write temporary session file: %w", err)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to commit session file: %w", err)
	}

	return nil
}

// LoadActiveSession reads the active session from .git/igit/active_session.json.
//
// If the file does not exist, it initializes a new Session, persists it to disk,
// and returns the newly created instance.
func LoadActiveSession() (*Session, error) {
	filePath, err := ActiveSessionPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			newSess := NewSession()
			if saveErr := SaveActiveSession(newSess); saveErr != nil {
				return nil, fmt.Errorf("failed to bootstrap new session: %w", saveErr)
			}
			return newSess, nil
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &sess, nil
}
