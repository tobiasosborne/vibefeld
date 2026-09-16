package service

import (
	"fmt"
	"strings"

	"github.com/tobiasosborne/vibefeld/internal/jobs"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// RecordProofSpec is the input to RecordProof, the atomic prover-write kernel
// op the rk hard-tier driver uses (rk B1 + FU3). It records a prover's
// decomposition of a CHALLENGED node into children, disposes the open
// challenge(s) that classified it as prover work, and releases any claim the
// prover held — so the node cycles back into verifier territory instead of
// staying prover-classified/claimed forever.
type RecordProofSpec struct {
	// ParentID is the node being proved (must currently be a prover job).
	ParentID types.NodeID
	// Owner is the prover identity; recorded as each child's author and used
	// for the claim-ownership check / release.
	Owner string
	// Children is the decomposition to record (>= 1). Per-child Dependencies
	// are resolved like RefineNodeBulk (rk B2).
	Children []ChildSpec
	// ExpectHash, when non-empty, is the ParentID content hash the prover turn
	// was dispatched against (rk B1). If it no longer matches, the write is
	// refused — the node was amended/re-stated during the model turn.
	ExpectHash string
}

// RecordProofResult reports what RecordProof did.
type RecordProofResult struct {
	ChildIDs           []types.NodeID
	ResolvedChallenges []string
	Released           bool
}

// RecordProof records a prover's proof step. Under ONE state read it: (1)
// verifies ParentID is a current prover job (matching the export's
// prover_ready classification) — rk B1's stale-role guard, so a proof
// generated for a node no longer classified for prover work is refused;
// (2) verifies ExpectHash (if supplied) still matches ParentID's content hash —
// rk B1's stale-bytes guard; (3) refuses a node claimed by a different owner;
// (4) creates the children (with per-child dependencies, rk B2); (5) resolves
// every open challenge on ParentID (rk FU3's challenge disposition); and
// (6) releases ParentID if the caller held its claim (rk FU3's release). The
// batch is serialized by the ledger lock with a batch-wide sequence check, so a
// "recorded proof" is a single all-or-first-fails transition; a crash leaves a
// valid prefix rather than a corrupt ledger.
func (s *ProofService) RecordProof(spec RecordProofSpec) (*RecordProofResult, error) {
	if len(spec.Children) == 0 {
		return nil, fmt.Errorf("%w: at least one child specification is required", ErrEmptyInput)
	}
	if strings.TrimSpace(spec.Owner) == "" {
		return nil, fmt.Errorf("%w: owner", ErrEmptyInput)
	}
	if err := s.validateDepth(spec.ParentID.Depth() + 1); err != nil {
		return nil, err
	}

	var result *RecordProofResult
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		parent := st.GetNode(spec.ParentID)
		if parent == nil {
			return nil, fmt.Errorf("%w: %s", ErrParentNotFound, spec.ParentID.String())
		}

		// rk B1: current prover-job classification (same classifier the export's
		// prover_ready flag uses).
		challengeMap := st.ChallengeMapForJobs()
		if !jobs.IsProverJob(parent, challengeMap) {
			return nil, fmt.Errorf("%w: node %s is not a prover job (needs an open blocking challenge, or a draft/needs_refinement state) — refusing a stale-role prover write", ErrInvalidState, spec.ParentID.String())
		}

		// rk B1: expected-hash guard.
		if spec.ExpectHash != "" && parent.ContentHash != spec.ExpectHash {
			return nil, fmt.Errorf("%w: node %s content hash changed since dispatch (expected %s, current %s)", ErrInvalidState, spec.ParentID.String(), spec.ExpectHash, parent.ContentHash)
		}

		// Ownership: refuse a node claimed by someone else; note if we hold it.
		claimedByOwner := false
		if parent.WorkflowState == schema.WorkflowClaimed {
			if parent.ClaimedBy != spec.Owner {
				return nil, fmt.Errorf("%w: node %s is claimed by %s, not %s", ErrOwnerMismatch, spec.ParentID.String(), parent.ClaimedBy, spec.Owner)
			}
			claimedByOwner = true
		}

		// (4) Children (rk B2 dependency resolution inside buildChildEvents).
		events, childIDs, err := s.buildChildEvents(st, spec.ParentID, spec.Owner, spec.Children)
		if err != nil {
			return nil, err
		}

		// (4a) rk GAP 9: stamp the DECOMPOSED PARENT with the acting prover as
		// its proof-of-record author. Part of the SAME batch as the
		// children/challenge/release below.
		events = append(events, ledger.NewNodeProofAuthored(spec.ParentID, spec.Owner))

		// (5) rk FU3: dispose every open challenge on the parent.
		var resolvedIDs []string
		for _, c := range st.GetChallengesForNode(spec.ParentID) {
			if c.Status == ChallengeStatusOpen {
				events = append(events, ledger.NewChallengeResolved(c.ID))
				resolvedIDs = append(resolvedIDs, c.ID)
			}
		}

		// (6) rk FU3: release the claim if the prover held it.
		released := false
		if claimedByOwner {
			events = append(events, ledger.NewNodesReleased([]types.NodeID{spec.ParentID}))
			released = true
		}

		result = &RecordProofResult{ChildIDs: childIDs, ResolvedChallenges: resolvedIDs, Released: released}
		return events, nil
	})
	if err != nil {
		return nil, wrapSequenceMismatch(err, "RecordProof")
	}

	return result, nil
}
