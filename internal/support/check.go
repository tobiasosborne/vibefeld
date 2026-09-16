package support

import (
	"strings"

	"github.com/tobiasosborne/vibefeld/internal/cycle"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// ErrCycle is the sentinel matched by CycleError via errors.Is.
var ErrCycle = aferrors.New(aferrors.DEPENDENCY_CYCLE, "circular dependency detected")

// ErrScopeLeak is the sentinel matched by ScopeLeakError via errors.Is.
var ErrScopeLeak = aferrors.New(aferrors.SCOPE_LEAK, "scope leak")

// CycleError reports that the prospective node Source would create a result-use
// cycle through Path. The offending IDs are fields, not only message text.
type CycleError struct {
	Source types.NodeID
	Path   []types.NodeID
	af     *aferrors.AFError
}

func (e *CycleError) Error() string { return e.af.Error() }

// Unwrap exposes the underlying AFError so errors.Is / aferrors.Code work.
func (e *CycleError) Unwrap() error { return e.af }

// ScopeLeakError reports that prospective node Node result-uses Dep, which is
// inside the local assumption Assumption whose scope does not enclose Node.
type ScopeLeakError struct {
	Node       types.NodeID
	Dep        types.NodeID
	Assumption types.NodeID
	af         *aferrors.AFError
}

func (e *ScopeLeakError) Error() string { return e.af.Error() }

// Unwrap exposes the underlying AFError so errors.Is / aferrors.Code work.
func (e *ScopeLeakError) Unwrap() error { return e.af }

func newCycleError(source types.NodeID, path []types.NodeID) *CycleError {
	return &CycleError{
		Source: source,
		Path:   path,
		af: aferrors.Newf(aferrors.DEPENDENCY_CYCLE, "circular dependency detected: %s",
			strings.Join(types.ToStringSlice(path), " -> ")),
	}
}

func newScopeLeakError(node, dep, assumption types.NodeID) *ScopeLeakError {
	return &ScopeLeakError{
		Node:       node,
		Dep:        dep,
		Assumption: assumption,
		af: aferrors.Newf(aferrors.SCOPE_LEAK,
			"scope leak: %s cites %s which is inside the local assumption %s",
			node.String(), dep.String(), assumption.String()),
	}
}

// CheckCreation validates a complete prospective batch against one state read:
//
//	(a) a result-use cycle check from each prospective node's own position over
//	    the overlaid provider, so the error names the actual path;
//	(b) a scope check rejecting a result-use dependency on a node inside a
//	    local_assume scope that does not also enclose the citing node;
//	(c) a dependency on a local_assume node is hypothesis-use: allowed only
//	    when that assumption encloses the citing node.
//
// The batch is checked as a whole: a cycle that only exists between two nodes
// of the batch is found. A removal-only overlay is accepted even when an
// unrelated legacy cycle remains in state, because the check only walks from
// the prospective nodes.
func CheckCreation(st *state.State, batch []ProspectiveNode) error {
	u := newUniverse(st, batch)
	provider := resultUseEdges(u)

	for i := range batch {
		res := cycle.DetectCycleFrom(&provider, batch[i].ID)
		if res.HasCycle {
			return newCycleError(batch[i].ID, res.Path)
		}
	}
	for i := range batch {
		if err := checkScope(u, batch[i]); err != nil {
			return err
		}
	}
	return nil
}
