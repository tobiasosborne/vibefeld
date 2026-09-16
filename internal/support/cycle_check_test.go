package support

import (
	"errors"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// TestCheckCreation_InsertedDischargeOrphansExistingHypothesisUse verifies
// that re-validation catches a pre-existing node whose scope changed: inserting
// a discharge before it (via the overlay) moves it out of the assumption's
// scope, orphaning its existing hypothesis-use.
func TestCheckCreation_InsertedDischargeOrphansExistingHypothesisUse(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeLocalAssume)
	addNode(t, st, "1.5", schema.NodeTypeClaim, "1.1") // hypothesis-use, in scope

	err := CheckCreation(st, []ProspectiveNode{{
		ID:       mustID(t, "1.2"),
		ParentID: mustID(t, "1"),
		Type:     schema.NodeTypeLocalDischarge,
	}})
	if err == nil || !errors.Is(err, ErrScopeLeak) {
		t.Fatalf("inserted discharge did not orphan existing hypothesis-use: %v", err)
	}
	var se *ScopeLeakError
	if !errors.As(err, &se) {
		t.Fatalf("error does not carry IDs: %v", err)
	}
	if se.Node.String() != "1.5" || se.Assumption.String() != "1.1" {
		t.Fatalf("scope leak should name the orphaned node: %+v", se)
	}
}

// TestCheckCreation_RemovalOnlyKeepsLegacyCyclePath verifies that a
// removal-only amendment that still has a path into a pre-existing,
// unrelated cycle is accepted: no edge relative to the pre-overlay graph is
// new.
func TestCheckCreation_RemovalOnlyKeepsLegacyCyclePath(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	// Legacy cycle 1.5 <-> 1.6, unrelated to the amendment.
	addNode(t, st, "1.5", schema.NodeTypeClaim, "1.6")
	addNode(t, st, "1.6", schema.NodeTypeClaim, "1.5")
	// 1.1 depends on the legacy cycle and on 1.7.
	addNode(t, st, "1.7", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim, "1.5", "1.7")

	// Removal-only: drop the 1.7 edge, keep the path into the legacy cycle.
	err := CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.1"),
		ParentID:     mustID(t, "1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1.5")},
	}})
	if err != nil {
		t.Fatalf("removal-only amendment with a legacy cycle path rejected: %v", err)
	}
}

// TestCheckCreation_AddedEdgeClosingNewCycleRejected verifies that adding an
// edge which closes a cycle is still rejected.
func TestCheckCreation_AddedEdgeClosingNewCycleRejected(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim)
	addNode(t, st, "1.2", schema.NodeTypeClaim, "1.1")

	err := CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.1"),
		ParentID:     mustID(t, "1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1.2")},
	}})
	if err == nil || !errors.Is(err, ErrCycle) {
		t.Fatalf("newly added closing edge accepted: %v", err)
	}
}
