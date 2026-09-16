package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
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

// ReleaseNodes releases every node satisfying pred in one commit. The
// selection runs inside the commit closure against the same state read used
// for the CAS, so a node whose claim was refreshed or released in the window
// between the caller's earlier read and this commit is not blindly released.
// Only nodes that are actually claimed in that state are emitted: an
// already-available node produces no NodesReleased event (an
// available->available transition would break replay). Returns the IDs it
// actually released.
func (s *ProofService) ReleaseNodes(pred func(*node.Node) bool) ([]types.NodeID, error) {
	if pred == nil {
		return nil, nil
	}
	var released []types.NodeID
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		released = released[:0]
		for _, n := range st.AllNodes() {
			if n.WorkflowState == schema.WorkflowClaimed && pred(n) {
				released = append(released, n.ID)
			}
		}
		sort.Slice(released, func(i, j int) bool {
			return released[i].String() < released[j].String()
		})
		if len(released) == 0 {
			return nil, nil
		}
		return []ledger.Event{ledger.NewNodesReleased(released)}, nil
	})
	if err != nil {
		return nil, wrapSequenceMismatch(err, "ReleaseNodes")
	}
	return released, nil
}

// ReleaseExpiredClaims releases every claimed node whose claim expired at or
// before now, in one commit. Used by `af reap`; it returns exactly the nodes
// actually released so the CLI reports reality rather than its earlier read.
func (s *ProofService) ReleaseExpiredClaims(now time.Time) ([]types.NodeID, error) {
	expired := types.FromTime(now)
	return s.ReleaseNodes(func(n *node.Node) bool {
		return n.ClaimedAt.Before(expired)
	})
}

// ReleaseAllClaims releases every currently claimed node in one commit,
// regardless of expiry. Used by `af reap --all`.
func (s *ProofService) ReleaseAllClaims() ([]types.NodeID, error) {
	return s.ReleaseNodes(func(n *node.Node) bool { return true })
}
