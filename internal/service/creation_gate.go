package service

import (
	"fmt"

	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// ParentStateError is returned when a node-creation path (refine, bulk refine,
// record-proof, direct node creation) targets a parent whose epistemic state
// does not admit children. Remedy is the exact command a caller can run to make
// the parent creatable again, or empty when there is no such remedy. It is a
// typed error (errors.As-able) so the CLI can print the remedy verbatim; the
// message already includes it.
type ParentStateError struct {
	Parent types.NodeID
	State  schema.EpistemicState
	Remedy string
	Err    error
}

func (e *ParentStateError) Error() string { return e.Err.Error() }

func (e *ParentStateError) Unwrap() error { return e.Err }

// checkParentCreationGate enforces D4's creation rule: a node may be created
// under a parent only while that parent is pending, draft, or needs_refinement.
// A validated parent names `af request-refinement`; an admitted parent names
// `af unadmit`; refuted and archived parents name no remedy (the branch is
// terminal). All are exit-3 logic errors.
func checkParentCreationGate(parent *node.Node) error {
	if parent == nil {
		return nil
	}
	switch parent.EpistemicState {
	case schema.EpistemicPending, schema.EpistemicDraft, schema.EpistemicNeedsRefinement:
		return nil
	case schema.EpistemicValidated:
		remedy := fmt.Sprintf("run af request-refinement %s", parent.ID.String())
		return &ParentStateError{
			Parent: parent.ID, State: parent.EpistemicState, Remedy: remedy,
			Err: aferrors.Newf(aferrors.INVALID_STATE,
				"cannot create a node under validated parent %s: %s first", parent.ID.String(), remedy),
		}
	case schema.EpistemicAdmitted:
		remedy := fmt.Sprintf("run af unadmit %s", parent.ID.String())
		return &ParentStateError{
			Parent: parent.ID, State: parent.EpistemicState, Remedy: remedy,
			Err: aferrors.Newf(aferrors.INVALID_STATE,
				"cannot create a node under admitted parent %s: %s first", parent.ID.String(), remedy),
		}
	case schema.EpistemicRefuted:
		return &ParentStateError{
			Parent: parent.ID, State: parent.EpistemicState,
			Err: aferrors.Newf(aferrors.INVALID_STATE,
				"cannot create a node under refuted parent %s", parent.ID.String()),
		}
	case schema.EpistemicArchived:
		return &ParentStateError{
			Parent: parent.ID, State: parent.EpistemicState,
			Err: aferrors.Newf(aferrors.INVALID_STATE,
				"cannot create a node under archived parent %s", parent.ID.String()),
		}
	default:
		return &ParentStateError{
			Parent: parent.ID, State: parent.EpistemicState,
			Err: aferrors.Newf(aferrors.INVALID_STATE,
				"cannot create a node under parent %s in state %s", parent.ID.String(), parent.EpistemicState),
		}
	}
}
