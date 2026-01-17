package taint

import (
	"testing"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// makeNode creates a test node with the given parameters.
// Panics if id parsing fails (only use valid IDs in tests).
func makeNode(id string, epistemic schema.EpistemicState, taint node.TaintState) *node.Node {
	nodeID, err := types.Parse(id)
	if err != nil {
		panic("invalid test node ID: " + id)
	}
	return &node.Node{
		ID:             nodeID,
		Type:           schema.NodeTypeClaim,
		Statement:      "test statement for " + id,
		Inference:      schema.InferenceAssumption,
		WorkflowState:  schema.WorkflowAvailable,
		EpistemicState: epistemic,
		TaintState:     taint,
	}
}

// ==================== PropagateTaint Tests ====================

func TestPropagateTaint_NilRoot(t *testing.T) {
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	allNodes := []*node.Node{child}

	changed := PropagateTaint(nil, allNodes)

	if len(changed) != 0 {
		t.Errorf("PropagateTaint(nil, ...) returned %d changed nodes, want 0", len(changed))
	}
}

func TestPropagateTaint_NilAllNodes(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	changed := PropagateTaint(root, nil)

	if len(changed) != 0 {
		t.Errorf("PropagateTaint(..., nil) returned %d changed nodes, want 0", len(changed))
	}
}

func TestPropagateTaint_EmptyAllNodes(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	changed := PropagateTaint(root, []*node.Node{})

	if len(changed) != 0 {
		t.Errorf("PropagateTaint(..., []) returned %d changed nodes, want 0", len(changed))
	}
}

func TestPropagateTaint_ParentToChildren(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child1 := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	child2 := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child1, child2}

	changed := PropagateTaint(root, allNodes)

	if len(changed) != 2 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 2", len(changed))
	}

	if child1.TaintState != node.TaintTainted {
		t.Errorf("child1.TaintState = %v, want %v", child1.TaintState, node.TaintTainted)
	}
	if child2.TaintState != node.TaintTainted {
		t.Errorf("child2.TaintState = %v, want %v", child2.TaintState, node.TaintTainted)
	}
}

func TestPropagateTaint_MultipleLevels(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	greatGrandchild := makeNode("1.1.1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child, grandchild, greatGrandchild}

	changed := PropagateTaint(root, allNodes)

	if len(changed) != 3 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 3", len(changed))
	}

	if child.TaintState != node.TaintTainted {
		t.Errorf("child.TaintState = %v, want %v", child.TaintState, node.TaintTainted)
	}
	if grandchild.TaintState != node.TaintTainted {
		t.Errorf("grandchild.TaintState = %v, want %v", grandchild.TaintState, node.TaintTainted)
	}
	if greatGrandchild.TaintState != node.TaintTainted {
		t.Errorf("greatGrandchild.TaintState = %v, want %v", greatGrandchild.TaintState, node.TaintTainted)
	}
}

func TestPropagateTaint_SiblingsNotAffected(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child1 := makeNode("1.1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child2 := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	grandchild1 := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child1, child2, grandchild1}

	changed := PropagateTaint(child1, allNodes)

	if len(changed) != 1 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 1", len(changed))
	}

	if child2.TaintState != node.TaintClean {
		t.Errorf("child2.TaintState = %v, want %v (siblings should not be affected)", child2.TaintState, node.TaintClean)
	}

	if grandchild1.TaintState != node.TaintTainted {
		t.Errorf("grandchild1.TaintState = %v, want %v", grandchild1.TaintState, node.TaintTainted)
	}
}

func TestPropagateTaint_ReturnsOnlyChangedNodes(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child1 := makeNode("1.1", schema.EpistemicValidated, node.TaintTainted) // Already tainted
	child2 := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)   // Will change

	allNodes := []*node.Node{root, child1, child2}

	changed := PropagateTaint(root, allNodes)

	if len(changed) != 1 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 1", len(changed))
	}

	if len(changed) > 0 && changed[0].ID.String() != "1.2" {
		t.Errorf("changed[0].ID = %v, want 1.2", changed[0].ID.String())
	}
}

func TestPropagateTaint_NoDescendants(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	allNodes := []*node.Node{root}

	changed := PropagateTaint(root, allNodes)

	if len(changed) != 0 {
		t.Errorf("PropagateTaint() with no descendants returned %d changed nodes, want 0", len(changed))
	}
}

func TestPropagateTaint_UnresolvedPropagates(t *testing.T) {
	root := makeNode("1", schema.EpistemicPending, node.TaintUnresolved)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child, grandchild}

	changed := PropagateTaint(root, allNodes)

	if child.TaintState != node.TaintUnresolved {
		t.Errorf("child.TaintState = %v, want %v", child.TaintState, node.TaintUnresolved)
	}
	if grandchild.TaintState != node.TaintUnresolved {
		t.Errorf("grandchild.TaintState = %v, want %v", grandchild.TaintState, node.TaintUnresolved)
	}

	if len(changed) != 2 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 2", len(changed))
	}
}

func TestPropagateTaint_CleanDoesNotOverrideSelfAdmitted(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child := makeNode("1.1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	allNodes := []*node.Node{root, child}

	changed := PropagateTaint(root, allNodes)

	if child.TaintState != node.TaintSelfAdmitted {
		t.Errorf("child.TaintState = %v, want %v (self_admitted should not be overridden by clean ancestor)", child.TaintState, node.TaintSelfAdmitted)
	}

	if len(changed) != 0 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 0 (no changes expected)", len(changed))
	}
}

func TestPropagateTaint_DeepHierarchy(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child := makeNode("1.1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	greatGrandchild := makeNode("1.1.1.1", schema.EpistemicValidated, node.TaintClean)
	greatGreatGrandchild := makeNode("1.1.1.1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child, grandchild, greatGrandchild, greatGreatGrandchild}

	changed := PropagateTaint(child, allNodes)

	if grandchild.TaintState != node.TaintTainted {
		t.Errorf("grandchild.TaintState = %v, want %v", grandchild.TaintState, node.TaintTainted)
	}
	if greatGrandchild.TaintState != node.TaintTainted {
		t.Errorf("greatGrandchild.TaintState = %v, want %v", greatGrandchild.TaintState, node.TaintTainted)
	}
	if greatGreatGrandchild.TaintState != node.TaintTainted {
		t.Errorf("greatGreatGrandchild.TaintState = %v, want %v", greatGreatGrandchild.TaintState, node.TaintTainted)
	}

	if len(changed) != 3 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 3", len(changed))
	}
}

func TestPropagateTaint_MultipleBranches(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	branch1Child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	branch1Grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	branch2Child := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	branch2Grandchild := makeNode("1.2.1", schema.EpistemicValidated, node.TaintClean)
	branch3Child := makeNode("1.3", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, branch1Child, branch1Grandchild, branch2Child, branch2Grandchild, branch3Child}

	changed := PropagateTaint(root, allNodes)

	if len(changed) != 5 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 5", len(changed))
	}

	for _, n := range []*node.Node{branch1Child, branch1Grandchild, branch2Child, branch2Grandchild, branch3Child} {
		if n.TaintState != node.TaintTainted {
			t.Errorf("node %s.TaintState = %v, want %v", n.ID.String(), n.TaintState, node.TaintTainted)
		}
	}
}

func TestPropagateTaint_RootNotInChangedList(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child}

	changed := PropagateTaint(root, allNodes)

	for _, n := range changed {
		if n.ID.String() == "1" {
			t.Errorf("Root node should not be in the changed list")
		}
	}
}

func TestPropagateTaint_PendingChildBecomesUnresolved(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	pendingChild := makeNode("1.1", schema.EpistemicPending, node.TaintClean) // incorrectly clean

	allNodes := []*node.Node{root, pendingChild}

	changed := PropagateTaint(root, allNodes)

	if pendingChild.TaintState != node.TaintUnresolved {
		t.Errorf("pendingChild.TaintState = %v, want %v", pendingChild.TaintState, node.TaintUnresolved)
	}

	if len(changed) != 1 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 1", len(changed))
	}
}

func TestPropagateTaint_AdmittedChildUnderAdmittedParent(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	admittedChild := makeNode("1.1", schema.EpistemicAdmitted, node.TaintClean) // incorrectly clean

	allNodes := []*node.Node{root, admittedChild}

	changed := PropagateTaint(root, allNodes)

	// Admitted child should become self_admitted (its own taint takes precedence)
	if admittedChild.TaintState != node.TaintSelfAdmitted {
		t.Errorf("admittedChild.TaintState = %v, want %v", admittedChild.TaintState, node.TaintSelfAdmitted)
	}

	if len(changed) != 1 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 1", len(changed))
	}
}

func TestPropagateTaint_NilNodeInAllNodes(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, nil, child, nil}

	changed := PropagateTaint(root, allNodes)

	// Should still process correctly, ignoring nil nodes
	if len(changed) != 1 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 1", len(changed))
	}

	if child.TaintState != node.TaintTainted {
		t.Errorf("child.TaintState = %v, want %v", child.TaintState, node.TaintTainted)
	}
}

func TestPropagateTaint_UnorderedInput(t *testing.T) {
	// Test that the function handles nodes in any order
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)

	// Provide nodes in reverse order
	allNodes := []*node.Node{grandchild, child, root}

	changed := PropagateTaint(root, allNodes)

	if len(changed) != 2 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 2", len(changed))
	}

	if child.TaintState != node.TaintTainted {
		t.Errorf("child.TaintState = %v, want %v", child.TaintState, node.TaintTainted)
	}
	if grandchild.TaintState != node.TaintTainted {
		t.Errorf("grandchild.TaintState = %v, want %v", grandchild.TaintState, node.TaintTainted)
	}
}

func TestPropagateTaint_NodesFromDifferentBranches(t *testing.T) {
	// Test propagating from a node when allNodes contains nodes from unrelated branches
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child1 := makeNode("1.1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	grandchild1 := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	child2 := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	grandchild2 := makeNode("1.2.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child1, grandchild1, child2, grandchild2}

	// Propagate from child1 only
	changed := PropagateTaint(child1, allNodes)

	// Only grandchild1 (descendant of child1) should change
	if len(changed) != 1 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 1", len(changed))
	}

	if grandchild1.TaintState != node.TaintTainted {
		t.Errorf("grandchild1.TaintState = %v, want %v", grandchild1.TaintState, node.TaintTainted)
	}

	// child2 and grandchild2 should remain unchanged
	if child2.TaintState != node.TaintClean {
		t.Errorf("child2.TaintState = %v, want %v", child2.TaintState, node.TaintClean)
	}
	if grandchild2.TaintState != node.TaintClean {
		t.Errorf("grandchild2.TaintState = %v, want %v", grandchild2.TaintState, node.TaintClean)
	}
}

func TestPropagateTaint_RootNotInAllNodes(t *testing.T) {
	// Test when root is not in allNodes (edge case)
	// The ancestors are computed from nodeMap (which is built from allNodes).
	// If root is not in allNodes, the child won't find it as an ancestor,
	// so taint from root won't propagate (because root is not in the nodeMap).
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)

	// Exclude root from allNodes
	allNodes := []*node.Node{child, grandchild}

	changed := PropagateTaint(root, allNodes)

	// Descendants are still found and processed, but root isn't in nodeMap
	// so they won't find root as an ancestor. They should still be clean
	// because they can't detect the admitted root.
	// This is actually correct behavior - the caller should include root in allNodes.
	if len(changed) != 0 {
		t.Errorf("PropagateTaint() when root not in allNodes returned %d changed nodes, want 0", len(changed))
	}
}

func TestPropagateTaint_SparseTree(t *testing.T) {
	// Test when some ancestors are missing from allNodes
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	// child 1.1 is missing
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, grandchild}

	changed := PropagateTaint(root, allNodes)

	// grandchild should still be processed and inherit taint from root
	if len(changed) != 1 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 1", len(changed))
	}

	if grandchild.TaintState != node.TaintTainted {
		t.Errorf("grandchild.TaintState = %v, want %v", grandchild.TaintState, node.TaintTainted)
	}
}

func TestPropagateTaint_ComplexMixedTaints(t *testing.T) {
	// Test complex scenario with mixed taint states
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child1 := makeNode("1.1", schema.EpistemicPending, node.TaintUnresolved)
	child2 := makeNode("1.2", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	grandchild1 := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	grandchild2 := makeNode("1.2.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child1, child2, grandchild1, grandchild2}

	_ = PropagateTaint(root, allNodes)

	// grandchild1 should become unresolved (parent is pending)
	if grandchild1.TaintState != node.TaintUnresolved {
		t.Errorf("grandchild1.TaintState = %v, want %v", grandchild1.TaintState, node.TaintUnresolved)
	}

	// grandchild2 should become tainted (parent is admitted)
	if grandchild2.TaintState != node.TaintTainted {
		t.Errorf("grandchild2.TaintState = %v, want %v", grandchild2.TaintState, node.TaintTainted)
	}
}

// ==================== GenerateTaintEvents Tests ====================

func TestGenerateTaintEvents_CreatesEventsForAllNodes(t *testing.T) {
	child1 := makeNode("1.1", schema.EpistemicValidated, node.TaintTainted)
	child2 := makeNode("1.2", schema.EpistemicValidated, node.TaintTainted)
	changedNodes := []*node.Node{child1, child2}

	events := GenerateTaintEvents(changedNodes)

	if len(events) != 2 {
		t.Errorf("GenerateTaintEvents() returned %d events, want 2", len(events))
	}

	for i, e := range events {
		if e.Type() != ledger.EventTaintRecomputed {
			t.Errorf("events[%d].Type() = %v, want %v", i, e.Type(), ledger.EventTaintRecomputed)
		}
	}

	if events[0].NodeID.String() != "1.1" {
		t.Errorf("events[0].NodeID = %v, want 1.1", events[0].NodeID.String())
	}
	if events[0].NewTaint != node.TaintTainted {
		t.Errorf("events[0].NewTaint = %v, want %v", events[0].NewTaint, node.TaintTainted)
	}
	if events[1].NodeID.String() != "1.2" {
		t.Errorf("events[1].NodeID = %v, want 1.2", events[1].NodeID.String())
	}
}

func TestGenerateTaintEvents_NilInput(t *testing.T) {
	events := GenerateTaintEvents(nil)

	if events != nil {
		t.Errorf("GenerateTaintEvents(nil) returned %v, want nil", events)
	}
}

func TestGenerateTaintEvents_EmptyInput(t *testing.T) {
	events := GenerateTaintEvents([]*node.Node{})

	if events != nil {
		t.Errorf("GenerateTaintEvents([]) returned %v, want nil", events)
	}
}

func TestGenerateTaintEvents_SkipsNilNodes(t *testing.T) {
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintTainted)
	changedNodes := []*node.Node{nil, child, nil}

	events := GenerateTaintEvents(changedNodes)

	if len(events) != 1 {
		t.Errorf("GenerateTaintEvents() returned %d events, want 1", len(events))
	}

	if events[0].NodeID.String() != "1.1" {
		t.Errorf("events[0].NodeID = %v, want 1.1", events[0].NodeID.String())
	}
}

func TestGenerateTaintEvents_PreservesTaintState(t *testing.T) {
	testCases := []struct {
		name  string
		taint node.TaintState
	}{
		{"clean", node.TaintClean},
		{"self_admitted", node.TaintSelfAdmitted},
		{"tainted", node.TaintTainted},
		{"unresolved", node.TaintUnresolved},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			n := makeNode("1.1", schema.EpistemicValidated, tc.taint)
			events := GenerateTaintEvents([]*node.Node{n})

			if len(events) != 1 {
				t.Fatalf("GenerateTaintEvents() returned %d events, want 1", len(events))
			}

			if events[0].NewTaint != tc.taint {
				t.Errorf("events[0].NewTaint = %v, want %v", events[0].NewTaint, tc.taint)
			}
		})
	}
}

func TestGenerateTaintEvents_SingleNode(t *testing.T) {
	n := makeNode("1.1.1", schema.EpistemicValidated, node.TaintUnresolved)
	events := GenerateTaintEvents([]*node.Node{n})

	if len(events) != 1 {
		t.Fatalf("GenerateTaintEvents() returned %d events, want 1", len(events))
	}

	if events[0].NodeID.String() != "1.1.1" {
		t.Errorf("events[0].NodeID = %v, want 1.1.1", events[0].NodeID.String())
	}
	if events[0].NewTaint != node.TaintUnresolved {
		t.Errorf("events[0].NewTaint = %v, want %v", events[0].NewTaint, node.TaintUnresolved)
	}
}

func TestGenerateTaintEvents_ManyNodes(t *testing.T) {
	nodes := make([]*node.Node, 9)
	for i := 0; i < 9; i++ {
		// Use valid child IDs: "1.1", "1.2", ..., "1.9"
		id := "1." + string(rune('1'+i))
		nodes[i] = makeNode(id, schema.EpistemicValidated, node.TaintTainted)
	}

	events := GenerateTaintEvents(nodes)

	if len(events) != 9 {
		t.Errorf("GenerateTaintEvents() returned %d events, want 9", len(events))
	}
}

// ==================== PropagateAndGenerateEvents Tests ====================

func TestPropagateAndGenerateEvents_BasicFlow(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child}

	changedNodes, events := PropagateAndGenerateEvents(root, allNodes)

	if len(changedNodes) != 1 {
		t.Errorf("PropagateAndGenerateEvents() returned %d changed nodes, want 1", len(changedNodes))
	}

	if len(events) != 1 {
		t.Errorf("PropagateAndGenerateEvents() returned %d events, want 1", len(events))
	}

	if events[0].NodeID.String() != "1.1" {
		t.Errorf("events[0].NodeID = %v, want 1.1", events[0].NodeID.String())
	}
	if events[0].NewTaint != node.TaintTainted {
		t.Errorf("events[0].NewTaint = %v, want %v", events[0].NewTaint, node.TaintTainted)
	}
}

func TestPropagateAndGenerateEvents_MultipleChanges(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child1 := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	child2 := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child1, child2, grandchild}

	changedNodes, events := PropagateAndGenerateEvents(root, allNodes)

	if len(changedNodes) != 3 {
		t.Errorf("PropagateAndGenerateEvents() returned %d changed nodes, want 3", len(changedNodes))
	}

	if len(events) != 3 {
		t.Errorf("PropagateAndGenerateEvents() returned %d events, want 3", len(events))
	}

	for i, e := range events {
		if e.Type() != ledger.EventTaintRecomputed {
			t.Errorf("events[%d].Type() = %v, want %v", i, e.Type(), ledger.EventTaintRecomputed)
		}
	}
}

func TestPropagateAndGenerateEvents_NoChanges(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintTainted) // Already correct

	allNodes := []*node.Node{root, child}

	changedNodes, events := PropagateAndGenerateEvents(root, allNodes)

	if len(changedNodes) != 0 {
		t.Errorf("PropagateAndGenerateEvents() returned %d changed nodes, want 0", len(changedNodes))
	}

	if events != nil {
		t.Errorf("PropagateAndGenerateEvents() returned %v events, want nil", events)
	}
}

func TestPropagateAndGenerateEvents_NilRoot(t *testing.T) {
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	allNodes := []*node.Node{child}

	changedNodes, events := PropagateAndGenerateEvents(nil, allNodes)

	if changedNodes != nil {
		t.Errorf("PropagateAndGenerateEvents(nil, ...) returned changed nodes, want nil")
	}
	if events != nil {
		t.Errorf("PropagateAndGenerateEvents(nil, ...) returned events, want nil")
	}
}

func TestPropagateAndGenerateEvents_NilAllNodes(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	changedNodes, events := PropagateAndGenerateEvents(root, nil)

	if changedNodes != nil {
		t.Errorf("PropagateAndGenerateEvents(..., nil) returned changed nodes, want nil")
	}
	if events != nil {
		t.Errorf("PropagateAndGenerateEvents(..., nil) returned events, want nil")
	}
}

func TestPropagateAndGenerateEvents_EmptyAllNodes(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	changedNodes, events := PropagateAndGenerateEvents(root, []*node.Node{})

	if changedNodes != nil {
		t.Errorf("PropagateAndGenerateEvents(..., []) returned changed nodes, want nil")
	}
	if events != nil {
		t.Errorf("PropagateAndGenerateEvents(..., []) returned events, want nil")
	}
}

func TestPropagateAndGenerateEvents_ConsistencyBetweenNodesAndEvents(t *testing.T) {
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child1 := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	child2 := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child1, child2}

	changedNodes, events := PropagateAndGenerateEvents(root, allNodes)

	// changedNodes and events should have matching length
	if len(changedNodes) != len(events) {
		t.Errorf("changedNodes length (%d) != events length (%d)", len(changedNodes), len(events))
	}

	// Each event should correspond to a changed node
	for i, n := range changedNodes {
		if events[i].NodeID.String() != n.ID.String() {
			t.Errorf("events[%d].NodeID = %v, want %v", i, events[i].NodeID.String(), n.ID.String())
		}
		if events[i].NewTaint != n.TaintState {
			t.Errorf("events[%d].NewTaint = %v, want %v", i, events[i].NewTaint, n.TaintState)
		}
	}
}

// ==================== sortByDepth Tests ====================

func TestSortByDepth_AlreadySorted(t *testing.T) {
	n1 := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	n2 := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	n3 := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)

	nodes := []*node.Node{n1, n2, n3}
	sortByDepth(nodes)

	if nodes[0].ID.String() != "1" || nodes[1].ID.String() != "1.1" || nodes[2].ID.String() != "1.1.1" {
		t.Errorf("sortByDepth did not maintain order: got %v, %v, %v", nodes[0].ID, nodes[1].ID, nodes[2].ID)
	}
}

func TestSortByDepth_ReverseSorted(t *testing.T) {
	n1 := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	n2 := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	n3 := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)

	nodes := []*node.Node{n3, n2, n1}
	sortByDepth(nodes)

	if nodes[0].ID.String() != "1" || nodes[1].ID.String() != "1.1" || nodes[2].ID.String() != "1.1.1" {
		t.Errorf("sortByDepth did not sort correctly: got %v, %v, %v", nodes[0].ID, nodes[1].ID, nodes[2].ID)
	}
}

func TestSortByDepth_MixedOrder(t *testing.T) {
	n1 := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	n2 := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	n3 := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	n4 := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)

	nodes := []*node.Node{n3, n1, n4, n2}
	sortByDepth(nodes)

	// Depth 1 should come first
	if nodes[0].ID.Depth() != 1 {
		t.Errorf("nodes[0] depth = %v, want 1", nodes[0].ID.Depth())
	}

	// Depth 2 nodes should come next
	if nodes[1].ID.Depth() != 2 || nodes[2].ID.Depth() != 2 {
		t.Errorf("middle nodes should have depth 2")
	}

	// Depth 3 should come last
	if nodes[3].ID.Depth() != 3 {
		t.Errorf("nodes[3] depth = %v, want 3", nodes[3].ID.Depth())
	}
}

func TestSortByDepth_Empty(t *testing.T) {
	nodes := []*node.Node{}
	sortByDepth(nodes) // Should not panic
}

func TestSortByDepth_SingleNode(t *testing.T) {
	n1 := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	nodes := []*node.Node{n1}
	sortByDepth(nodes)

	if len(nodes) != 1 || nodes[0].ID.String() != "1.1.1" {
		t.Errorf("sortByDepth with single node failed")
	}
}

// ==================== getAncestors Tests ====================

func TestGetAncestors_RootNode(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	nodeMap := map[string]*node.Node{"1": root}

	ancestors := getAncestors(root, nodeMap)

	if len(ancestors) != 0 {
		t.Errorf("getAncestors for root returned %d ancestors, want 0", len(ancestors))
	}
}

func TestGetAncestors_ChildNode(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	nodeMap := map[string]*node.Node{"1": root, "1.1": child}

	ancestors := getAncestors(child, nodeMap)

	if len(ancestors) != 1 {
		t.Errorf("getAncestors for child returned %d ancestors, want 1", len(ancestors))
	}
	if ancestors[0].ID.String() != "1" {
		t.Errorf("ancestor ID = %v, want 1", ancestors[0].ID.String())
	}
}

func TestGetAncestors_DeeplyNested(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	greatGrandchild := makeNode("1.1.1.1", schema.EpistemicValidated, node.TaintClean)
	nodeMap := map[string]*node.Node{
		"1":       root,
		"1.1":     child,
		"1.1.1":   grandchild,
		"1.1.1.1": greatGrandchild,
	}

	ancestors := getAncestors(greatGrandchild, nodeMap)

	if len(ancestors) != 3 {
		t.Errorf("getAncestors for greatGrandchild returned %d ancestors, want 3", len(ancestors))
	}

	// Ancestors should be ordered from closest to furthest
	if ancestors[0].ID.String() != "1.1.1" {
		t.Errorf("ancestors[0] = %v, want 1.1.1", ancestors[0].ID.String())
	}
	if ancestors[1].ID.String() != "1.1" {
		t.Errorf("ancestors[1] = %v, want 1.1", ancestors[1].ID.String())
	}
	if ancestors[2].ID.String() != "1" {
		t.Errorf("ancestors[2] = %v, want 1", ancestors[2].ID.String())
	}
}

func TestGetAncestors_MissingParent(t *testing.T) {
	// Test when parent is missing from nodeMap
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	// child 1.1 is missing
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	nodeMap := map[string]*node.Node{
		"1":     root,
		"1.1.1": grandchild,
	}

	ancestors := getAncestors(grandchild, nodeMap)

	// Should only find root, skipping missing child
	if len(ancestors) != 1 {
		t.Errorf("getAncestors with missing parent returned %d ancestors, want 1", len(ancestors))
	}
	if ancestors[0].ID.String() != "1" {
		t.Errorf("ancestor ID = %v, want 1", ancestors[0].ID.String())
	}
}

// ==================== getAncestorsCached Tests ====================

func TestGetAncestorsCached_UsesCache(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	nodeMap := map[string]*node.Node{
		"1":     root,
		"1.1":   child,
		"1.1.1": grandchild,
	}

	cache := make(map[string][]*node.Node)
	// Pre-populate cache for parent
	cache["1.1"] = []*node.Node{root}

	ancestors := getAncestorsCached(grandchild, nodeMap, cache)

	if len(ancestors) != 2 {
		t.Errorf("getAncestorsCached returned %d ancestors, want 2", len(ancestors))
	}

	// Should include child and root
	if ancestors[0].ID.String() != "1.1" {
		t.Errorf("ancestors[0] = %v, want 1.1", ancestors[0].ID.String())
	}
	if ancestors[1].ID.String() != "1" {
		t.Errorf("ancestors[1] = %v, want 1", ancestors[1].ID.String())
	}

	// Result should be cached
	if _, ok := cache["1.1.1"]; !ok {
		t.Error("getAncestorsCached did not cache result")
	}
}

func TestGetAncestorsCached_RootNode(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	nodeMap := map[string]*node.Node{"1": root}
	cache := make(map[string][]*node.Node)

	ancestors := getAncestorsCached(root, nodeMap, cache)

	if ancestors != nil {
		t.Errorf("getAncestorsCached for root returned %v, want nil", ancestors)
	}
}

func TestGetAncestorsCached_FallbackToGetAncestors(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	nodeMap := map[string]*node.Node{
		"1":     root,
		"1.1":   child,
		"1.1.1": grandchild,
	}

	// Empty cache - should trigger fallback
	cache := make(map[string][]*node.Node)

	ancestors := getAncestorsCached(grandchild, nodeMap, cache)

	if len(ancestors) != 2 {
		t.Errorf("getAncestorsCached fallback returned %d ancestors, want 2", len(ancestors))
	}
}

func TestGetAncestorsCached_ParentNotInNodeMap(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	// child 1.1 is missing from nodeMap
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	nodeMap := map[string]*node.Node{
		"1":     root,
		"1.1.1": grandchild,
	}

	cache := make(map[string][]*node.Node)
	cache["1.1"] = []*node.Node{root} // Cache exists for parent ID even though node doesn't

	ancestors := getAncestorsCached(grandchild, nodeMap, cache)

	// Should use parent's cached ancestors (root) without adding parent
	if len(ancestors) != 1 {
		t.Errorf("getAncestorsCached with missing parent returned %d ancestors, want 1", len(ancestors))
	}
	if ancestors[0].ID.String() != "1" {
		t.Errorf("ancestor = %v, want 1", ancestors[0].ID.String())
	}
}

func TestGetAncestorsCached_AlreadyCached(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	nodeMap := map[string]*node.Node{"1": root, "1.1": child}

	cache := make(map[string][]*node.Node)
	cache["1.1"] = []*node.Node{root}

	// Request already cached node
	ancestors := getAncestorsCached(child, nodeMap, cache)

	if len(ancestors) != 1 {
		t.Errorf("getAncestorsCached for cached node returned %d ancestors, want 1", len(ancestors))
	}
	if ancestors[0].ID.String() != "1" {
		t.Errorf("ancestor = %v, want 1", ancestors[0].ID.String())
	}
}
