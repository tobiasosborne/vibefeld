package service

import (
	"fmt"
	"strings"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// Terminal verifier/prover actions that share two D5/D9 behaviours: they
// record the acting agent identity (driver-supplied provenance) and, when the
// caller holds the node's claim, release that claim in the same event under a
// claim-generation fence. AdmitNode/RefuteNode/ArchiveNode are the
// identity-less forms kept for internal callers and tests; the CLI calls the
// *WithAgent / *WithOptions forms.

// AdmitNodeWithAgent admits a node and records by as the acting identity.
func (s *ProofService) AdmitNodeWithAgent(id types.NodeID, by string) error {
	return s.commitThenTaint(id, func(st *state.State) ([]ledger.Event, error) {
		n := st.GetNode(id)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
		}
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicAdmitted); err != nil {
			return nil, err
		}
		ev := ledger.NewNodeAdmitted(id)
		ev.By = by
		setFencedClaimRelease(n, by, &ev.ReleaseClaim, &ev.ClaimSeq)
		return []ledger.Event{ev}, nil
	})
}

// RefuteNodeWithAgent refutes a node and records by as the acting identity.
func (s *ProofService) RefuteNodeWithAgent(id types.NodeID, by string) error {
	return s.commitThenTaint(id, func(st *state.State) ([]ledger.Event, error) {
		n := st.GetNode(id)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
		}
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicRefuted); err != nil {
			return nil, err
		}
		ev := ledger.NewNodeRefuted(id)
		ev.By = by
		setFencedClaimRelease(n, by, &ev.ReleaseClaim, &ev.ClaimSeq)
		return []ledger.Event{ev}, nil
	})
}

// ArchiveOptions configures ArchiveNodeWithOptions.
type ArchiveOptions struct {
	// Reason is recorded on the NodeArchived event (previously it was only
	// printed by the CLI and lost from the ledger).
	Reason string
	// Force abandons an open challenge obligation on the node or an active
	// descendant. It requires a non-empty Reason.
	Force bool
	// By is the acting agent identity (driver-supplied provenance).
	By string
}

// ArchiveNodeWithOptions archives a node, recording the reason, whether the
// archive was forced, and the acting agent. It refuses when a challenge is
// open on the node or an active (non-severed) descendant unless Force is set
// with a non-empty Reason (D9). When the caller holds the node's claim, the
// same event releases it under D5's claim-generation fence.
func (s *ProofService) ArchiveNodeWithOptions(id types.NodeID, opts ArchiveOptions) error {
	if opts.Force && strings.TrimSpace(opts.Reason) == "" {
		return fmt.Errorf("%w: --force requires a non-empty --reason", ErrEmptyInput)
	}

	return s.commitThenTaint(id, func(st *state.State) ([]ledger.Event, error) {
		n := st.GetNode(id)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
		}
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicArchived); err != nil {
			return nil, err
		}

		if obligations := st.OpenChallengeObligations(id); len(obligations) > 0 && !opts.Force {
			return nil, fmt.Errorf("%w: node(s) %s", ErrOpenChallengeObligation, formatObligations(obligations))
		}

		ev := ledger.NewNodeArchived(id)
		ev.Reason = opts.Reason
		ev.Forced = opts.Force
		ev.By = opts.By
		setFencedClaimRelease(n, opts.By, &ev.ReleaseClaim, &ev.ClaimSeq)
		return []ledger.Event{ev}, nil
	})
}
