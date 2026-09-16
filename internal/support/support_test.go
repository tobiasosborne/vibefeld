package support

import (
	"errors"
	"testing"

	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func mustID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return id
}

// addNode creates a state node with the given type and dependencies.
func addNode(t *testing.T, st *state.State, id string, typ schema.NodeType, deps ...string) *node.Node {
	t.Helper()
	nodeID := mustID(t, id)
	parsed := make([]types.NodeID, len(deps))
	for i, d := range deps {
		parsed[i] = mustID(t, d)
	}
	n, err := node.NewNodeWithOptions(nodeID, typ, "stmt "+id, schema.InferenceModusPonens,
		node.NodeOptions{Dependencies: parsed})
	if err != nil {
		t.Fatalf("NewNodeWithOptions(%s): %v", id, err)
	}
	st.AddNode(n)
	return n
}

// TestCheckCreation_ResultUseOfClaimAncestorIsCycle reproduces the vibefeld-0ko0
// decision: a child may NOT result-use an ancestor claim.
func TestCheckCreation_ResultUseOfClaimAncestorIsCycle(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)

	err := CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.1"),
		ParentID:     mustID(t, "1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1")},
	}})
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("error is not ErrCycle: %v", err)
	}
	if aferrors.Code(err) != aferrors.DEPENDENCY_CYCLE {
		t.Fatalf("code = %v, want DEPENDENCY_CYCLE", aferrors.Code(err))
	}
	var ce *CycleError
	if !errors.As(err, &ce) {
		t.Fatalf("error does not carry IDs: %v", err)
	}
	if ce.Source.String() != "1.1" || len(ce.Path) == 0 {
		t.Fatalf("cycle fields not populated: %+v", ce)
	}
	// The path names both nodes.
	named := false
	for _, id := range ce.Path {
		if id.String() == "1" {
			named = true
		}
	}
	if !named {
		t.Fatalf("cycle path does not name the ancestor: %v", ce.Path)
	}
}

// TestCheckCreation_HypothesisUseOfEnclosingAssumeAccepted covers the second
// half of vibefeld-0ko0: a child MAY hypothesis-use an enclosing local_assume.
func TestCheckCreation_HypothesisUseOfEnclosingAssumeAccepted(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeLocalAssume)

	err := CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.1.1"),
		ParentID:     mustID(t, "1.1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1.1")},
	}})
	if err != nil {
		t.Fatalf("hypothesis-use of enclosing local_assume rejected: %v", err)
	}

	// Hypothesis-use must not appear in the result-use graph.
	p := ResultUseEdges(st, nil)
	if deps, ok := p.GetNodeDependencies(mustID(t, "1.1")); !ok || len(deps) != 0 {
		t.Fatalf("local_assume should have no result-use edges, got %v (ok=%v)", deps, ok)
	}
}

// TestCheckCreation_SiblingCousinAccepted verifies that cross-branch
// dependency (a cousin edge with no child back-edge) is not a cycle.
func TestCheckCreation_SiblingCousinAccepted(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim)
	addNode(t, st, "1.2", schema.NodeTypeClaim)
	addNode(t, st, "1.1.1", schema.NodeTypeClaim)

	err := CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.2.1"),
		ParentID:     mustID(t, "1.2"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1.1.1")},
	}})
	if err != nil {
		t.Fatalf("cousin edge rejected: %v", err)
	}
}

// TestCheckCreation_ForeignScopeLeakRejected verifies that citing a node inside
// a local_assume block that does not enclose the citing node is a scope leak.
func TestCheckCreation_ForeignScopeLeakRejected(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeLocalAssume)
	addNode(t, st, "1.1.1", schema.NodeTypeClaim)
	addNode(t, st, "1.1.2", schema.NodeTypeLocalDischarge)

	err := CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.2"),
		ParentID:     mustID(t, "1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1.1.1")},
	}})
	if err == nil {
		t.Fatal("expected scope leak, got nil")
	}
	if !errors.Is(err, ErrScopeLeak) {
		t.Fatalf("error is not ErrScopeLeak: %v", err)
	}
	var se *ScopeLeakError
	if !errors.As(err, &se) {
		t.Fatalf("error does not carry IDs: %v", err)
	}
	if se.Node.String() != "1.2" || se.Dep.String() != "1.1.1" || se.Assumption.String() != "1.1" {
		t.Fatalf("scope leak fields wrong: %+v", se)
	}
}

// TestCheckCreation_LaterSiblingScopeLeak verifies the sibling part of the
// structural scope: a later sibling is inside an earlier local_assume's scope
// until a local_discharge closes it.
func TestCheckCreation_LaterSiblingScopeLeak(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeLocalAssume)
	addNode(t, st, "1.2", schema.NodeTypeClaim) // inside 1.1's scope
	addNode(t, st, "1.3", schema.NodeTypeLocalDischarge)
	addNode(t, st, "1.4", schema.NodeTypeClaim) // after discharge: outside

	// Citing 1.2 (inside the scope) from 1.4 must leak.
	err := CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.5"),
		ParentID:     mustID(t, "1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1.2")},
	}})
	if err == nil || !errors.Is(err, ErrScopeLeak) {
		t.Fatalf("expected scope leak for later sibling, got %v", err)
	}

	// Citing 1.4 (after the discharge) is fine.
	err = CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.5"),
		ParentID:     mustID(t, "1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: []types.NodeID{mustID(t, "1.4")},
	}})
	if err != nil {
		t.Fatalf("citing a node after discharge should be allowed: %v", err)
	}
}

// TestCheckCreation_BatchMutualCycle verifies a cycle formed only between two
// prospective nodes of the same batch is rejected.
func TestCheckCreation_BatchMutualCycle(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)

	err := CheckCreation(st, []ProspectiveNode{
		{
			ID:           mustID(t, "1.1"),
			ParentID:     mustID(t, "1"),
			Type:         schema.NodeTypeClaim,
			Dependencies: []types.NodeID{mustID(t, "1.2")},
		},
		{
			ID:           mustID(t, "1.2"),
			ParentID:     mustID(t, "1"),
			Type:         schema.NodeTypeClaim,
			Dependencies: []types.NodeID{mustID(t, "1.1")},
		},
	})
	if err == nil || !errors.Is(err, ErrCycle) {
		t.Fatalf("expected batch cycle, got %v", err)
	}
}

// TestCheckCreation_RemovalOnlyWithLegacyCycle verifies that an overlay entry
// REPLACES the node's edges, so removing the edge that closes a legacy cycle is
// accepted even though the cycle exists in state.
func TestCheckCreation_RemovalOnlyWithLegacyCycle(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	// Legacy cycle 1.1 -> 1.2 -> 1.1 built directly into state.
	addNode(t, st, "1.1", schema.NodeTypeClaim, "1.2")
	addNode(t, st, "1.2", schema.NodeTypeClaim, "1.1")

	// Removal-only amendment of 1.1: drop its dependency on 1.2.
	err := CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.1"),
		ParentID:     mustID(t, "1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: nil,
	}})
	if err != nil {
		t.Fatalf("removal-only amendment rejected: %v", err)
	}

	// But an unrelated cycle elsewhere must not be surfaced either.
	addNode(t, st, "1.3", schema.NodeTypeClaim, "1.4")
	addNode(t, st, "1.4", schema.NodeTypeClaim, "1.3")
	err = CheckCreation(st, []ProspectiveNode{{
		ID:           mustID(t, "1.1"),
		ParentID:     mustID(t, "1"),
		Type:         schema.NodeTypeClaim,
		Dependencies: nil,
	}})
	if err != nil {
		t.Fatalf("unrelated legacy cycle surfaced: %v", err)
	}
}

// TestResultUseEdges_SeveredAndDangling verifies severed children are excluded,
// severed/missing dependencies are sinks, and both are reported.
func TestResultUseEdges_SeveredAndDangling(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	severed := addNode(t, st, "1.1", schema.NodeTypeClaim)
	severed.EpistemicState = schema.EpistemicArchived
	addNode(t, st, "1.2", schema.NodeTypeClaim, "1.1", "1.9")

	p := ResultUseEdges(st, nil)

	// 1's severed child 1.1 is not a result-use edge.
	deps, _ := p.GetNodeDependencies(mustID(t, "1"))
	for _, d := range deps {
		if d.String() == "1.1" {
			t.Fatalf("severed child should be excluded from result-use edges: %v", deps)
		}
	}

	// 1.2's dependency on severed 1.1 and missing 1.9 are kept as sink edges.
	deps, _ = p.GetNodeDependencies(mustID(t, "1.2"))
	want := map[string]bool{"1.1": true, "1.9": true}
	if len(deps) != 2 {
		t.Fatalf("expected both dangling edges kept, got %v", deps)
	}
	for _, d := range deps {
		delete(want, d.String())
	}
	if len(want) != 0 {
		t.Fatalf("missing dangling edges: %v", want)
	}

	dangling := DanglingDeps(st, nil)
	if len(dangling) != 2 {
		t.Fatalf("expected 2 dangling deps, got %d: %+v", len(dangling), dangling)
	}
	severedSeen, missingSeen := false, false
	for _, d := range dangling {
		if d.To.String() == "1.1" && d.Severed {
			severedSeen = true
		}
		if d.To.String() == "1.9" && !d.Severed {
			missingSeen = true
		}
	}
	if !severedSeen || !missingSeen {
		t.Fatalf("dangling classification wrong: %+v", dangling)
	}
}
