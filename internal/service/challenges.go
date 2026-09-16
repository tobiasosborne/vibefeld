package service

import (
	"fmt"
	"strings"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// ResolveChallenge marks an open challenge as resolved. The existence and
// open-status checks and the appended event share one CAS-protected state
// read, so a concurrent resolve/withdraw cannot be overwritten.
func (s *ProofService) ResolveChallenge(challengeID string) error {
	if strings.TrimSpace(challengeID) == "" {
		return fmt.Errorf("%w: challenge ID", ErrEmptyInput)
	}

	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		c := st.GetChallenge(challengeID)
		if c == nil {
			return nil, fmt.Errorf("challenge %q does not exist", challengeID)
		}
		if c.Status != ChallengeStatusOpen {
			return nil, fmt.Errorf("challenge %q is not open (already %s)", challengeID, c.Status)
		}
		return []ledger.Event{ledger.NewChallengeResolved(challengeID)}, nil
	})
	return wrapSequenceMismatch(err, "ResolveChallenge")
}

// WithdrawChallenge marks an open challenge as withdrawn. Same one-read
// check-and-append contract as ResolveChallenge.
func (s *ProofService) WithdrawChallenge(challengeID string) error {
	if strings.TrimSpace(challengeID) == "" {
		return fmt.Errorf("%w: challenge ID", ErrEmptyInput)
	}

	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		c := st.GetChallenge(challengeID)
		if c == nil {
			return nil, fmt.Errorf("challenge %q does not exist", challengeID)
		}
		if c.Status != ChallengeStatusOpen {
			return nil, fmt.Errorf("challenge %q is not open (already %s)", challengeID, c.Status)
		}
		return []ledger.Event{ledger.NewChallengeWithdrawn(challengeID)}, nil
	})
	return wrapSequenceMismatch(err, "WithdrawChallenge")
}

// ReleaseNodes releases the given nodes in one commit. Used by `af reap` for
// claim-lock cleanup; releasing an already-available node is a no-op at replay
// time, so callers pass exactly the nodes they observed as claimed.
func (s *ProofService) ReleaseNodes(ids []types.NodeID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		return []ledger.Event{ledger.NewNodesReleased(ids)}, nil
	})
	return wrapSequenceMismatch(err, "ReleaseNodes")
}
