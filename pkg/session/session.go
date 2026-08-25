// Package session manages the lifecycle and turn history of an agent session.
//
// It tracks sequential prompt-response turns, chains snapshot commits into a
// Directed Acyclic Graph (DAG), and maintains custom Git reference pointers on disk.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/devxdh/igit/pkg/gitengine"
)

// Session represents an active or historical agent interaction sequence.
//
// Fields:
//   - ID: Unique identifier for the session (e.g. "sess_1772183400_a1b2c3d4").
//   - TurnCount: Number of recorded snapshots within this session.
//   - LatestSHA: The 40-character commit SHA of the most recent turn.
//     Used as the parent commit SHA when recording the next turn.
type Session struct {
	ID        string `json:"id"`
	TurnCount int    `json:"turn_count"`
	LatestSHA string `json:"latest_sha"`
}

// GenerateID produces a unique session identifier by combining the current
// Unix timestamp with 4 cryptographically secure random hex bytes.
//
// Example output: "sess_1772183400_f38a9b1c"
func GenerateID() string {
	bytes := make([]byte, 4)
	_, _ = rand.Read(bytes)
	return fmt.Sprintf("sess_%d_%s", time.Now().Unix(), hex.EncodeToString(bytes))
}

// NewSession initializes an empty in-memory Session with a new ID,
// a TurnCount of 0, and no parent commit.
func NewSession() *Session {
	return &Session{
		ID:        GenerateID(),
		TurnCount: 0,
		LatestSHA: "",
	}
}

// ActiveRef returns the full Git reference path for a specific turn number.
//
// Format: "refs/igit/active/<session_id>/<turn_number>"
// Example: "refs/igit/active/sess_1772183400_f38a9b1c/1"
func (s *Session) ActiveRef(turn int) string {
	return fmt.Sprintf("refs/igit/active/%s/%d", s.ID, turn)
}

// CurrentRef returns the Git reference path pointing to the latest turn of this session.
//
// Format: "refs/igit/active/<session_id>/current"
// This serves as an anchor pointer to easily resolve the session's tip commit.
func (s *Session) CurrentRef() string {
	return fmt.Sprintf("refs/igit/active/%s/current", s.ID)
}

// RecordTurn creates a new snapshot commit from treeSHA and advances the session state.
//
// How this works:
//  1. Increments TurnCount (e.g. Turn 0 -> Turn 1).
//  2. Commits treeSHA with s.LatestSHA set as its parent commit.
//     (If TurnCount is 1, s.LatestSHA is empty, creating a root commit).
//  3. Writes the turn reference: "refs/igit/active/<session_id>/<turn>".
//  4. Updates the current reference: "refs/igit/active/<session_id>/current".
//  5. Updates in-memory s.LatestSHA to point to the newly created commit.
//
// Invariants guaranteed:
//   - Turn N is always a direct child commit of Turn N-1.
//   - Any historical turn can be reconstructed by reading its ref.
//
// Parameters:
//   - treeSHA: 40-character SHA of the directory snapshot (from BuildIsolatedTree).
//   - message: Summary message for the turn commit. If empty, a default is generated.
//
// Returns the 40-character commit SHA of the newly created turn.
func (s *Session) RecordTurn(treeSHA, message string) (string, error) {
	if treeSHA == "" {
		return "", errors.New("cannot record turn with empty treeSHA")
	}

	nextTurn := s.TurnCount + 1
	if message == "" {
		message = fmt.Sprintf("turn %d snapshot", nextTurn)
	}

	// Commit the tree object, linking the previous turn's SHA as parent
	commitSHA, err := gitengine.CommitTree(treeSHA, s.LatestSHA, message)
	if err != nil {
		return "", fmt.Errorf("failed to commit turn tree: %w", err)
	}

	// Persist the specific turn ref: refs/igit/active/<id>/<turn>
	turnRef := s.ActiveRef(nextTurn)
	if err := gitengine.UpdateRef(turnRef, commitSHA); err != nil {
		return "", fmt.Errorf("failed to update turn ref %s: %w", turnRef, err)
	}

	// Persist the floating head pointer: refs/igit/active/<id>/current
	if err := gitengine.UpdateRef(s.CurrentRef(), commitSHA); err != nil {
		return "", fmt.Errorf("failed to update current ref: %w", err)
	}

	// Advance in-memory state
	s.TurnCount = nextTurn
	s.LatestSHA = commitSHA

	return commitSHA, nil
}
