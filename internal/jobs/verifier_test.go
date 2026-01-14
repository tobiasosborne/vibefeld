// Package jobs_test contains tests for the jobs package.
package jobs_test

import (
	"testing"

	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// createVerifierTestNode creates a test node with specific states for verifier testing.
func createVerifierTestNode(t *testing.T, idStr string, workflow schema.WorkflowState, epistemic schema.EpistemicState) *node.Node {
	t.Helper()
	id, err := types.Parse(idStr)
	if err != nil {
		t.Fatalf("types.Parse(%q) error: %v", idStr, err)
	}

	n, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement for "+idStr, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("node.NewNode() error: %v", err)
	}

	// Override default states for testing
	n.WorkflowState = workflow
	n.EpistemicState = epistemic
	return n
}

// buildNodeMap builds a map from node ID string to node pointer.
func buildNodeMap(nodes []*node.Node) map[string]*node.Node {
	m := make(map[string]*node.Node)
	for _, n := range nodes {
		m[n.ID.String()] = n
	}
	return m
}

// TestFindVerifierJobs_EmptyInput tests that FindVerifierJobs handles empty input.
func TestFindVerifierJobs_EmptyInput(t *testing.T) {
	// Test with nil slice
	result := jobs.FindVerifierJobs(nil, nil)
	if len(result) != 0 {
		t.Errorf("FindVerifierJobs(nil, nil) returned %d nodes, want 0", len(result))
	}

	// Test with empty slice
	result = jobs.FindVerifierJobs([]*node.Node{}, map[string]*node.Node{})
	if len(result) != 0 {
		t.Errorf("FindVerifierJobs([], {}) returned %d nodes, want 0", len(result))
	}
}

// TestFindVerifierJobs_NodeWithNoChildrenIsNotVerifierJob tests that a leaf node
// with no children is NOT a verifier job (it's a prover job that needs refinement).
func TestFindVerifierJobs_NodeWithNoChildrenIsNotVerifierJob(t *testing.T) {
	// A leaf node with no children is NOT a verifier job
	n := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	nodes := []*node.Node{n}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	if len(result) != 0 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 0 (leaf nodes are not verifier jobs)", len(result))
	}
}

// TestFindVerifierJobs_NodeWithUnvalidatedChildrenIsNotVerifierJob tests that a node
// with unvalidated children is NOT a verifier job.
func TestFindVerifierJobs_NodeWithUnvalidatedChildrenIsNotVerifierJob(t *testing.T) {
	parent := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	child1 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending) // Not validated
	child2 := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated)

	nodes := []*node.Node{parent, child1, child2}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	// Parent should NOT be a verifier job because child1 is not validated
	for _, n := range result {
		if n.ID.String() == "1" {
			t.Error("FindVerifierJobs() should not return node 1 because it has unvalidated children")
		}
	}
}

// TestFindVerifierJobs_NodeWithAllValidatedChildrenIsVerifierJob tests that a node
// with all validated children IS a verifier job.
func TestFindVerifierJobs_NodeWithAllValidatedChildrenIsVerifierJob(t *testing.T) {
	parent := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	child1 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	child2 := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated)
	child3 := createVerifierTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicValidated)

	nodes := []*node.Node{parent, child1, child2, child3}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	// Parent should be a verifier job
	found := false
	for _, n := range result {
		if n.ID.String() == "1" {
			found = true
			break
		}
	}

	if !found {
		t.Error("FindVerifierJobs() should return node 1 because all its children are validated")
	}
}

// TestFindVerifierJobs_OnlyPendingNodesCanBeVerifierJobs tests that only pending nodes
// can be verifier jobs.
func TestFindVerifierJobs_OnlyPendingNodesCanBeVerifierJobs(t *testing.T) {
	tests := []struct {
		name      string
		epistemic schema.EpistemicState
		wantJob   bool
	}{
		{"pending is verifier job", schema.EpistemicPending, true},
		{"validated is not verifier job", schema.EpistemicValidated, false},
		{"admitted is not verifier job", schema.EpistemicAdmitted, false},
		{"refuted is not verifier job", schema.EpistemicRefuted, false},
		{"archived is not verifier job", schema.EpistemicArchived, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parent node with one validated child
			parent := createVerifierTestNode(t, "1", schema.WorkflowAvailable, tt.epistemic)
			child := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
			nodes := []*node.Node{parent, child}
			nodeMap := buildNodeMap(nodes)

			result := jobs.FindVerifierJobs(nodes, nodeMap)

			gotJob := false
			for _, n := range result {
				if n.ID.String() == "1" {
					gotJob = true
					break
				}
			}
			if gotJob != tt.wantJob {
				t.Errorf("FindVerifierJobs() with epistemic=%s returned job=%v, want job=%v",
					tt.epistemic, gotJob, tt.wantJob)
			}
		})
	}
}

// TestFindVerifierJobs_BlockedNodesCannotBeVerifierJobs tests that blocked nodes
// cannot be verifier jobs.
func TestFindVerifierJobs_BlockedNodesCannotBeVerifierJobs(t *testing.T) {
	// Blocked parent with validated child - should NOT be a verifier job
	parent := createVerifierTestNode(t, "1", schema.WorkflowBlocked, schema.EpistemicPending)
	child := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)

	nodes := []*node.Node{parent, child}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	for _, n := range result {
		if n.ID.String() == "1" {
			t.Error("FindVerifierJobs() should not return blocked node 1")
		}
	}
}

// TestFindVerifierJobs_AvailableNodesCanBeVerifierJobs tests that available nodes
// with all validated children ARE verifier jobs (bug fix for vibefeld-99ab).
func TestFindVerifierJobs_AvailableNodesCanBeVerifierJobs(t *testing.T) {
	// Available parent with all validated children - should be a verifier job
	parent := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	child1 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	child2 := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated)

	nodes := []*node.Node{parent, child1, child2}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	found := false
	for _, n := range result {
		if n.ID.String() == "1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("FindVerifierJobs() should return available node 1 with all validated children")
	}
}

// TestFindVerifierJobs_ClaimedNodesCanBeVerifierJobs tests that claimed nodes
// with all validated children ARE verifier jobs.
func TestFindVerifierJobs_ClaimedNodesCanBeVerifierJobs(t *testing.T) {
	// Claimed parent with all validated children - should be a verifier job
	parent := createVerifierTestNode(t, "1", schema.WorkflowClaimed, schema.EpistemicPending)
	child1 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	child2 := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated)

	nodes := []*node.Node{parent, child1, child2}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	found := false
	for _, n := range result {
		if n.ID.String() == "1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("FindVerifierJobs() should return claimed node 1 with all validated children")
	}
}

// TestFindVerifierJobs_ChildrenWithMixedEpistemicStates tests various child epistemic states.
func TestFindVerifierJobs_ChildrenWithMixedEpistemicStates(t *testing.T) {
	tests := []struct {
		name           string
		childEpistemic schema.EpistemicState
		wantParentJob  bool
	}{
		{"all children validated", schema.EpistemicValidated, true},
		{"child pending blocks parent", schema.EpistemicPending, false},
		{"child admitted blocks parent", schema.EpistemicAdmitted, false},
		{"child refuted blocks parent", schema.EpistemicRefuted, false},
		{"child archived blocks parent", schema.EpistemicArchived, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
			child := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, tt.childEpistemic)

			nodes := []*node.Node{parent, child}
			nodeMap := buildNodeMap(nodes)

			result := jobs.FindVerifierJobs(nodes, nodeMap)

			parentIsJob := false
			for _, n := range result {
				if n.ID.String() == "1" {
					parentIsJob = true
					break
				}
			}

			if parentIsJob != tt.wantParentJob {
				t.Errorf("FindVerifierJobs() with child epistemic=%s: parent is job=%v, want %v",
					tt.childEpistemic, parentIsJob, tt.wantParentJob)
			}
		})
	}
}

// TestFindVerifierJobs_GrandchildrenDoNotAffectParent tests that only direct children
// affect whether a node is a verifier job, not grandchildren.
func TestFindVerifierJobs_GrandchildrenDoNotAffectParent(t *testing.T) {
	// Parent: available, pending
	// Child: validated
	// Grandchild: pending (should NOT affect parent)
	parent := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	child := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	grandchild := createVerifierTestNode(t, "1.1.1", schema.WorkflowAvailable, schema.EpistemicPending)

	nodes := []*node.Node{parent, child, grandchild}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	// Parent should still be a verifier job because its direct child (1.1) is validated
	parentIsJob := false
	for _, n := range result {
		if n.ID.String() == "1" {
			parentIsJob = true
			break
		}
	}

	if !parentIsJob {
		t.Error("FindVerifierJobs() should return parent node even though grandchild is not validated")
	}
}

// TestFindVerifierJobs_MultipleVerifierJobs tests finding multiple verifier jobs.
func TestFindVerifierJobs_MultipleVerifierJobs(t *testing.T) {
	// Node 1.1 with only validated children
	node1 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending)
	node1Child := createVerifierTestNode(t, "1.1.1", schema.WorkflowAvailable, schema.EpistemicValidated)

	// Node 1.2 with only validated children
	node2 := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending)
	node2Child := createVerifierTestNode(t, "1.2.1", schema.WorkflowAvailable, schema.EpistemicValidated)

	nodes := []*node.Node{node1, node1Child, node2, node2Child}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	// Both node1.1 and node1.2 should be verifier jobs
	expectedIDs := map[string]bool{"1.1": true, "1.2": true}
	for _, n := range result {
		if expectedIDs[n.ID.String()] {
			delete(expectedIDs, n.ID.String())
		}
	}

	if len(expectedIDs) > 0 {
		for id := range expectedIDs {
			t.Errorf("FindVerifierJobs() did not return expected node %s", id)
		}
	}
}

// TestFindVerifierJobs_PreservesOrder tests that the order of returned nodes
// matches the input order.
func TestFindVerifierJobs_PreservesOrder(t *testing.T) {
	// Create nodes in specific order, all as verifier jobs (with validated children)
	node3 := createVerifierTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending)
	node3Child := createVerifierTestNode(t, "1.3.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	node1 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending)
	node1Child := createVerifierTestNode(t, "1.1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	node2 := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending)
	node2Child := createVerifierTestNode(t, "1.2.1", schema.WorkflowAvailable, schema.EpistemicValidated)

	nodes := []*node.Node{node3, node3Child, node1, node1Child, node2, node2Child}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	// Should return 3 verifier jobs in order: 1.3, 1.1, 1.2
	if len(result) != 3 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 3", len(result))
		return
	}

	expectedOrder := []string{"1.3", "1.1", "1.2"}
	for i, n := range result {
		if n.ID.String() != expectedOrder[i] {
			t.Errorf("FindVerifierJobs() result[%d] = %s, want %s", i, n.ID.String(), expectedOrder[i])
		}
	}
}

// TestFindVerifierJobs_ReturnsOriginalPointers tests that the returned slice
// contains pointers to the original nodes (not copies).
func TestFindVerifierJobs_ReturnsOriginalPointers(t *testing.T) {
	original := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	child := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	nodes := []*node.Node{original, child}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	if len(result) != 1 {
		t.Fatalf("FindVerifierJobs() returned %d nodes, want 1", len(result))
	}

	if result[0] != original {
		t.Error("FindVerifierJobs() should return pointers to original nodes")
	}
}

// TestFindVerifierJobs_AllNodesExcluded tests the case where all input nodes
// are excluded.
func TestFindVerifierJobs_AllNodesExcluded(t *testing.T) {
	nodes := []*node.Node{
		// No children (leaf nodes are not verifier jobs)
		createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),
		// Blocked
		createVerifierTestNode(t, "1.1", schema.WorkflowBlocked, schema.EpistemicPending),
		// Not pending
		createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated),
	}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	if len(result) != 0 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 0", len(result))
	}
}

// TestFindVerifierJobs_MixedStates tests FindVerifierJobs with a mix of different states.
func TestFindVerifierJobs_MixedStates(t *testing.T) {
	nodes := []*node.Node{
		createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),     // has children 1.1-1.7 below
		createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending),   // exclude: not validated child
		createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated), // validated child
		createVerifierTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending),   // exclude: not validated child
		createVerifierTestNode(t, "1.4", schema.WorkflowBlocked, schema.EpistemicPending),     // exclude: blocked child
		createVerifierTestNode(t, "1.5", schema.WorkflowAvailable, schema.EpistemicAdmitted),  // exclude: admitted child
		createVerifierTestNode(t, "1.6", schema.WorkflowAvailable, schema.EpistemicRefuted),   // exclude: refuted child
		createVerifierTestNode(t, "1.7", schema.WorkflowAvailable, schema.EpistemicPending),   // exclude: not validated child
	}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	// Node 1 should NOT be a verifier job because not all children are validated
	for _, n := range result {
		if n.ID.String() == "1" {
			t.Error("FindVerifierJobs() should not return node 1 because it has unvalidated children")
		}
	}
}

// TestFindVerifierJobs_DeeplyNestedStructure tests verifier jobs in a deeply nested tree.
func TestFindVerifierJobs_DeeplyNestedStructure(t *testing.T) {
	// Create a deep tree:
	// 1 (available, pending) -> 1.1 (validated) -> 1.1.1 (available, pending) -> 1.1.1.1 (validated)
	node1 := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	node11 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	node111 := createVerifierTestNode(t, "1.1.1", schema.WorkflowAvailable, schema.EpistemicPending)
	node1111 := createVerifierTestNode(t, "1.1.1.1", schema.WorkflowAvailable, schema.EpistemicValidated)

	nodes := []*node.Node{node1, node11, node111, node1111}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	// Both node1 (child 1.1 is validated) and node111 (child 1.1.1.1 is validated) should be jobs
	expectedIDs := map[string]bool{"1": true, "1.1.1": true}
	for _, n := range result {
		if !expectedIDs[n.ID.String()] {
			t.Errorf("FindVerifierJobs() returned unexpected node %s", n.ID.String())
		}
		delete(expectedIDs, n.ID.String())
	}

	if len(expectedIDs) > 0 {
		for id := range expectedIDs {
			t.Errorf("FindVerifierJobs() did not return expected node %s", id)
		}
	}
}

// TestFindVerifierJobs_SomeChildrenValidatedSomeNot tests that ALL children must be validated.
func TestFindVerifierJobs_SomeChildrenValidatedSomeNot(t *testing.T) {
	parent := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	child1 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	child2 := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated)
	child3 := createVerifierTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending) // NOT validated

	nodes := []*node.Node{parent, child1, child2, child3}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	// Parent should NOT be a verifier job because child3 is not validated
	for _, n := range result {
		if n.ID.String() == "1" {
			t.Error("FindVerifierJobs() should not return node 1 because not all children are validated")
		}
	}
}

// TestFindVerifierJobs_EmptyNodeMap tests behavior with empty node map but non-empty nodes.
func TestFindVerifierJobs_EmptyNodeMap(t *testing.T) {
	// If nodeMap is empty, nodes will appear to have no children, so they won't be verifier jobs
	n := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	nodes := []*node.Node{n}
	emptyMap := map[string]*node.Node{}

	result := jobs.FindVerifierJobs(nodes, emptyMap)

	// Node 1 should NOT be a verifier job (no children found in empty map)
	if len(result) != 0 {
		t.Errorf("FindVerifierJobs() with empty nodeMap returned %d nodes, want 0 (leaf nodes are not verifier jobs)", len(result))
	}
}

// TestFindVerifierJobs_NodeMapContainsExtraNodes tests that extra nodes in nodeMap don't affect results.
func TestFindVerifierJobs_NodeMapContainsExtraNodes(t *testing.T) {
	// nodeMap has extra nodes not in the input slice
	node1 := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	node11 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	// extraNode is at 1.2, which is a sibling of 1.1, making it a child of 1
	extraNode := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated)

	// Only node1 is in the input slice
	nodes := []*node.Node{node1}
	// nodeMap contains all nodes including children 1.1 and 1.2 (both validated)
	nodeMap := buildNodeMap([]*node.Node{node1, node11, extraNode})

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	// node1 should be a verifier job because both children (1.1 and 1.2) are validated
	if len(result) != 1 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 1", len(result))
	}
	if len(result) > 0 && result[0].ID.String() != "1" {
		t.Errorf("FindVerifierJobs() returned %s, want 1", result[0].ID.String())
	}
}

// TestFindVerifierJobs_ReleasedRefinedNodeIsVerifierJob tests the main bug fix for vibefeld-99ab:
// after a prover refines a node and releases it, the node should appear as a verifier job
// once all children are validated.
func TestFindVerifierJobs_ReleasedRefinedNodeIsVerifierJob(t *testing.T) {
	// Simulate the workflow:
	// 1. Prover claimed node 1, refined it (added children 1.1, 1.2), then released it
	// 2. Children 1.1 and 1.2 have been validated
	// 3. Node 1 should now be a verifier job (available + pending + all children validated)
	parent := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	child1 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	child2 := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated)

	nodes := []*node.Node{parent, child1, child2}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap)

	// Node 1 should be a verifier job
	found := false
	for _, n := range result {
		if n.ID.String() == "1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("FindVerifierJobs() should return node 1 (available + pending + all children validated) - this is the bug fix for vibefeld-99ab")
	}
}
