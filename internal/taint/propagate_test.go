//go:build integration

package taint

import (
	"testing"

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

func TestPropagateTaint_ParentToChildren(t *testing.T) {
	// When a parent's taint changes to self_admitted, children should become tainted
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child1 := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	child2 := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child1, child2}

	changed := PropagateTaint(root, allNodes)

	// Both children should have taint changed
	if len(changed) != 2 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 2", len(changed))
	}

	// Verify children are now tainted
	if child1.TaintState != node.TaintTainted {
		t.Errorf("child1.TaintState = %v, want %v", child1.TaintState, node.TaintTainted)
	}
	if child2.TaintState != node.TaintTainted {
		t.Errorf("child2.TaintState = %v, want %v", child2.TaintState, node.TaintTainted)
	}
}

func TestPropagateTaint_MultipleLevels(t *testing.T) {
	// Taint should cascade through multiple levels
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	greatGrandchild := makeNode("1.1.1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child, grandchild, greatGrandchild}

	changed := PropagateTaint(root, allNodes)

	// All descendants should be changed
	if len(changed) != 3 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 3", len(changed))
	}

	// Verify all descendants are tainted
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
	// Siblings should not be affected when a non-ancestor node changes
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child1 := makeNode("1.1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child2 := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	grandchild1 := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child1, child2, grandchild1}

	changed := PropagateTaint(child1, allNodes)

	// Only child1's descendant (grandchild1) should change
	if len(changed) != 1 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 1", len(changed))
	}

	// child2 should remain clean
	if child2.TaintState != node.TaintClean {
		t.Errorf("child2.TaintState = %v, want %v (siblings should not be affected)", child2.TaintState, node.TaintClean)
	}

	// grandchild1 should be tainted
	if grandchild1.TaintState != node.TaintTainted {
		t.Errorf("grandchild1.TaintState = %v, want %v", grandchild1.TaintState, node.TaintTainted)
	}
}

func TestPropagateTaint_ReturnsOnlyChangedNodes(t *testing.T) {
	// Only nodes whose taint actually changed should be returned
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child1 := makeNode("1.1", schema.EpistemicValidated, node.TaintTainted) // Already tainted
	child2 := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)   // Will change

	allNodes := []*node.Node{root, child1, child2}

	changed := PropagateTaint(root, allNodes)

	// Only child2 should be in changed list since child1 was already tainted
	if len(changed) != 1 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 1", len(changed))
	}

	// Verify the changed node is child2
	if len(changed) > 0 && changed[0].ID.String() != "1.2" {
		t.Errorf("changed[0].ID = %v, want 1.2", changed[0].ID.String())
	}
}

func TestPropagateTaint_NilRoot(t *testing.T) {
	// Nil root should return nil/empty slice
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	allNodes := []*node.Node{child}

	changed := PropagateTaint(nil, allNodes)

	if len(changed) != 0 {
		t.Errorf("PropagateTaint(nil, ...) returned %d changed nodes, want 0", len(changed))
	}
}

func TestPropagateTaint_NilAllNodes(t *testing.T) {
	// Nil allNodes should return nil/empty slice
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	changed := PropagateTaint(root, nil)

	if len(changed) != 0 {
		t.Errorf("PropagateTaint(..., nil) returned %d changed nodes, want 0", len(changed))
	}
}

func TestPropagateTaint_EmptyAllNodes(t *testing.T) {
	// Empty allNodes should return empty slice
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	changed := PropagateTaint(root, []*node.Node{})

	if len(changed) != 0 {
		t.Errorf("PropagateTaint(..., []) returned %d changed nodes, want 0", len(changed))
	}
}

func TestPropagateTaint_NoDescendants(t *testing.T) {
	// Root with no descendants should return empty slice
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	sibling := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)

	// allNodes contains only the root (and a sibling not in the tree hierarchy)
	allNodes := []*node.Node{root}

	changed := PropagateTaint(root, allNodes)

	if len(changed) != 0 {
		t.Errorf("PropagateTaint() with no descendants returned %d changed nodes, want 0", len(changed))
	}

	// Test with siblings that are not descendants
	nonDescendant := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	allNodes2 := []*node.Node{sibling, nonDescendant}

	changed2 := PropagateTaint(sibling, allNodes2)

	if len(changed2) != 0 {
		t.Errorf("PropagateTaint() with only siblings returned %d changed nodes, want 0", len(changed2))
	}
}

func TestPropagateTaint_UnresolvedPropagates(t *testing.T) {
	// Unresolved taint should propagate to descendants
	root := makeNode("1", schema.EpistemicPending, node.TaintUnresolved)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child, grandchild}

	changed := PropagateTaint(root, allNodes)

	// Both descendants should become unresolved
	if child.TaintState != node.TaintUnresolved {
		t.Errorf("child.TaintState = %v, want %v", child.TaintState, node.TaintUnresolved)
	}
	if grandchild.TaintState != node.TaintUnresolved {
		t.Errorf("grandchild.TaintState = %v, want %v", grandchild.TaintState, node.TaintUnresolved)
	}

	// Both should be in changed list
	if len(changed) != 2 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 2", len(changed))
	}
}

func TestPropagateTaint_CleanDoesNotOverrideSelfAdmitted(t *testing.T) {
	// A clean parent should not override a self_admitted child
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child := makeNode("1.1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	allNodes := []*node.Node{root, child}

	changed := PropagateTaint(root, allNodes)

	// Child should remain self_admitted (it introduces its own taint)
	if child.TaintState != node.TaintSelfAdmitted {
		t.Errorf("child.TaintState = %v, want %v (self_admitted should not be overridden by clean ancestor)", child.TaintState, node.TaintSelfAdmitted)
	}

	// Nothing should have changed
	if len(changed) != 0 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 0 (no changes expected)", len(changed))
	}
}

func TestPropagateTaint_DeepHierarchy(t *testing.T) {
	// Test a deeper hierarchy with various taint states
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	child := makeNode("1.1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	greatGrandchild := makeNode("1.1.1.1", schema.EpistemicValidated, node.TaintClean)
	greatGreatGrandchild := makeNode("1.1.1.1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child, grandchild, greatGrandchild, greatGreatGrandchild}

	changed := PropagateTaint(child, allNodes)

	// All descendants of child (1.1) should become tainted
	if grandchild.TaintState != node.TaintTainted {
		t.Errorf("grandchild.TaintState = %v, want %v", grandchild.TaintState, node.TaintTainted)
	}
	if greatGrandchild.TaintState != node.TaintTainted {
		t.Errorf("greatGrandchild.TaintState = %v, want %v", greatGrandchild.TaintState, node.TaintTainted)
	}
	if greatGreatGrandchild.TaintState != node.TaintTainted {
		t.Errorf("greatGreatGrandchild.TaintState = %v, want %v", greatGreatGrandchild.TaintState, node.TaintTainted)
	}

	// 3 nodes should have changed
	if len(changed) != 3 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 3", len(changed))
	}
}

func TestPropagateTaint_MultipleBranches(t *testing.T) {
	// Test tree with multiple branches
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	branch1Child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	branch1Grandchild := makeNode("1.1.1", schema.EpistemicValidated, node.TaintClean)
	branch2Child := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	branch2Grandchild := makeNode("1.2.1", schema.EpistemicValidated, node.TaintClean)
	branch3Child := makeNode("1.3", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, branch1Child, branch1Grandchild, branch2Child, branch2Grandchild, branch3Child}

	changed := PropagateTaint(root, allNodes)

	// All 5 descendants should become tainted
	if len(changed) != 5 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 5", len(changed))
	}

	// Verify all descendants are tainted
	for _, n := range []*node.Node{branch1Child, branch1Grandchild, branch2Child, branch2Grandchild, branch3Child} {
		if n.TaintState != node.TaintTainted {
			t.Errorf("node %s.TaintState = %v, want %v", n.ID.String(), n.TaintState, node.TaintTainted)
		}
	}
}

func TestPropagateTaint_RootNotInChangedList(t *testing.T) {
	// The root node itself should never be in the changed list
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)

	allNodes := []*node.Node{root, child}

	changed := PropagateTaint(root, allNodes)

	// Only child should be in changed list
	for _, n := range changed {
		if n.ID.String() == "1" {
			t.Errorf("Root node should not be in the changed list")
		}
	}
}

func TestPropagateTaint_PendingChildBecomesUnresolved(t *testing.T) {
	// A pending child should compute to unresolved regardless of parent's taint
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	pendingChild := makeNode("1.1", schema.EpistemicPending, node.TaintClean) // incorrectly clean

	allNodes := []*node.Node{root, pendingChild}

	changed := PropagateTaint(root, allNodes)

	// Pending child should become unresolved
	if pendingChild.TaintState != node.TaintUnresolved {
		t.Errorf("pendingChild.TaintState = %v, want %v", pendingChild.TaintState, node.TaintUnresolved)
	}

	// Should be in changed list
	if len(changed) != 1 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 1", len(changed))
	}
}

func TestPropagateTaint_AdmittedChildUnderAdmittedParent(t *testing.T) {
	// An admitted child under an admitted parent should be self_admitted (not tainted)
	root := makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	admittedChild := makeNode("1.1", schema.EpistemicAdmitted, node.TaintClean) // incorrectly clean

	allNodes := []*node.Node{root, admittedChild}

	changed := PropagateTaint(root, allNodes)

	// Admitted child should become self_admitted (its own taint takes precedence)
	// Because ComputeTaint rule 3 (self-admitted) is checked before rule 4 (ancestor taint)
	if admittedChild.TaintState != node.TaintSelfAdmitted {
		t.Errorf("admittedChild.TaintState = %v, want %v", admittedChild.TaintState, node.TaintSelfAdmitted)
	}

	// Should be in changed list
	if len(changed) != 1 {
		t.Errorf("PropagateTaint() returned %d changed nodes, want 1", len(changed))
	}
}
