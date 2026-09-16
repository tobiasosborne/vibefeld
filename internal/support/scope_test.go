package support

import (
	"errors"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// TestCheckCreation_NestedUndischargedAssumeDoesNotEncloseLaterSibling
// verifies that a scope opened inside one branch (1.1.1) is closed when that
// branch's child list ends, so it does not enclose the later sibling 1.2.
func TestCheckCreation_NestedUndischargedAssumeDoesNotEncloseLaterSibling(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim)
	addNode(t, st, "1.1.1", schema.NodeTypeLocalAssume) // never discharged
	addNode(t, st, "1.1.1.1", schema.NodeTypeClaim)
	addNode(t, st, "1.2", schema.NodeTypeClaim)

	err := CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.2.1"),
		ParentID:     mustID(t, "1.2"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1.1.1.1")},
	}})
	if err == nil || !errors.Is(err, ErrScopeLeak) {
		t.Fatalf("nested undischarged assumption leaked into 1.2: %v", err)
	}
}

// TestCheckCreation_ForeignDischargeCannotCloseSiblingScope verifies that a
// discharge nested under one branch (1.2.1) cannot close a scope opened by a
// different branch (1.1), so 1.1 still encloses the later sibling 1.4.
func TestCheckCreation_ForeignDischargeCannotCloseSiblingScope(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeLocalAssume)
	addNode(t, st, "1.2", schema.NodeTypeClaim)
	addNode(t, st, "1.2.1", schema.NodeTypeLocalDischarge)
	addNode(t, st, "1.3", schema.NodeTypeClaim)

	// 1.1's scope is still open for later siblings, so this hypothesis-use is
	// accepted. If 1.2.1 had wrongly popped 1.1, this would leak.
	err := CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.4"),
		ParentID:     mustID(t, "1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1.1")},
	}})
	if err != nil {
		t.Fatalf("foreign discharge wrongly closed 1.1's scope: %v", err)
	}
}

// TestCheckCreation_DischargePreAndPostContext covers the two-sided context of
// a local_discharge: its own premises see the pre-close scope, while nodes
// citing it see the post-close scope.
func TestCheckCreation_DischargePreAndPostContext(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeLocalAssume)
	addNode(t, st, "1.1.1", schema.NodeTypeClaim)
	addNode(t, st, "1.1.2", schema.NodeTypeLocalDischarge)

	// The discharge's own dependency on a node inside the assumption it closes
	// is allowed (pre-close context).
	err := CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.1.2"),
		ParentID:     mustID(t, "1.1"),
		Type:         schema.NodeTypeLocalDischarge,
		Dependencies: []types.NodeID{mustID(t, "1.1.1")},
	}})
	if err != nil {
		t.Fatalf("discharge premise from inside its assumption rejected: %v", err)
	}

	// A later sibling citing the discharge's result is accepted (post-close).
	err = CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.2"),
		ParentID:     mustID(t, "1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1.1.2")},
	}})
	if err != nil {
		t.Fatalf("later sibling citing the discharge rejected: %v", err)
	}

	// A later sibling citing a node still inside the closed assumption leaks.
	err = CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.3"),
		ParentID:     mustID(t, "1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1.1.1")},
	}})
	if err == nil || !errors.Is(err, ErrScopeLeak) {
		t.Fatalf("later sibling citing inside the closed assumption not rejected: %v", err)
	}
}

// TestResultUseEdges_LocalAssumeOnEitherSideExcluded verifies the decision that
// child result-use edges are excluded both when the child is a local_assume and
// when the parent is a local_assume.
func TestResultUseEdges_LocalAssumeOnEitherSideExcluded(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim)
	addNode(t, st, "1.1.1", schema.NodeTypeLocalAssume) // child is an assume
	addNode(t, st, "1.2", schema.NodeTypeLocalAssume)   // parent is an assume
	addNode(t, st, "1.2.1", schema.NodeTypeClaim)       // non-assume child of an assume

	p := ResultUseEdges(st, nil)

	// Child condition: 1.1's local_assume child contributes no result edge.
	deps, _ := p.GetNodeDependencies(mustID(t, "1.1"))
	for _, d := range deps {
		if d.String() == "1.1.1" {
			t.Fatalf("local_assume child contributed a result edge: %v", deps)
		}
	}
	// Parent condition: 1.2 is a local_assume, so its claim child is not a
	// result edge of 1.2.
	deps, _ = p.GetNodeDependencies(mustID(t, "1.2"))
	for _, d := range deps {
		if d.String() == "1.2.1" {
			t.Fatalf("local_assume parent contributed a child result edge: %v", deps)
		}
	}
}
