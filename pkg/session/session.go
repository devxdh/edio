// Package session handles session logic
package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/devxdh/igit/pkg/gitengine"
)

type Session struct {
	ID        string
	TurnCount int
	LatestSHA string
}

func GenerateID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return fmt.Sprintf("sess_%d_%s", time.Now().Unix(), hex.EncodeToString(bytes))
}

func NewSession() *Session {
	return &Session{
		ID:        GenerateID(),
		TurnCount: 0,
		LatestSHA: "",
	}
}

func (s *Session) ActiveRef(turn int) string {
	return fmt.Sprintf("refs/igit/active/%s/%d", s.ID, turn)
}

func (s *Session) CurrentRef() string {
	return fmt.Sprintf("refs/igit/active/%s/current", s.ID)
}

func (s *Session) RecordTurn(treeSHA, message string) (string, error) {
	if treeSHA == "" {
		return "", errors.New("cannot record turn with empty treeSHA")
	}

	nextTurn := s.TurnCount + 1
	if message == "" {
		message = fmt.Sprintf("turn %d snapshot", nextTurn)
	}

	commitSHA, err := gitengine.CommitTree(treeSHA, s.LatestSHA, message)
	if err != nil {
		return "", fmt.Errorf("failed to commit turn tree: %w", err)
	}

	turnRef := s.ActiveRef(nextTurn)
	if err := gitengine.UpdateRef(turnRef, commitSHA); err != nil {
		return "", fmt.Errorf("failed to update turn ref %s: %w", turnRef, err)
	}

	if err := gitengine.UpdateRef(s.CurrentRef(), commitSHA); err != nil {
		return "", fmt.Errorf("failed to update current ref: %w", err)
	}

	s.TurnCount = nextTurn
	s.LatestSHA = commitSHA

	return commitSHA, nil
}
