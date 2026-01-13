// Package jobs_test contains tests for the jobs package.
package jobs_test

import (
	"testing"

	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// helper function to create a test node with specific states
func createTestNode(t *testing.T, idStr string, workflow schema.WorkflowState, epistemic schema.EpistemicState) *node.Node {
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

// TestFindProverJobs_FindsAvailablePendingNodes tests that FindProverJobs returns
// nodes that are available and pending.
func TestFindProverJobs_FindsAvailablePendingNodes(t *testing.T) {
	nodes := []*node.Node{
		createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),
		createTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending),
		createTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending),
	}

	result := jobs.FindProverJobs(nodes)

	if len(result) != 3 {
		t.Errorf("FindProverJobs() returned %d nodes, want 3", len(result))
	}

	// Verify all returned nodes are available + pending
	for _, n := range result {
		if n.WorkflowState != schema.WorkflowAvailable {
			t.Errorf("Node %s WorkflowState = %q, want %q", n.ID.String(), n.WorkflowState, schema.WorkflowAvailable)
		}
		if n.EpistemicState != schema.EpistemicPending {
			t.Errorf("Node %s EpistemicState = %q, want %q", n.ID.String(), n.EpistemicState, schema.EpistemicPending)
		}
	}
}

// TestFindProverJobs_ExcludesClaimedNodes tests that claimed nodes are not
// returned as prover jobs.
func TestFindProverJobs_ExcludesClaimedNodes(t *testing.T) {
	nodes := []*node.Node{
		createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),   // should be included
		createTestNode(t, "1.1", schema.WorkflowClaimed, schema.EpistemicPending),   // should be excluded
		createTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending), // should be included
	}

	result := jobs.FindProverJobs(nodes)

	if len(result) != 2 {
		t.Errorf("FindProverJobs() returned %d nodes, want 2", len(result))
	}

	// Verify claimed node is not in result
	for _, n := range result {
		if n.ID.String() == "1.1" {
			t.Error("FindProverJobs() should not return claimed node 1.1")
		}
	}
}

// TestFindProverJobs_ExcludesValidatedNodes tests that validated nodes are not
// returned as prover jobs.
func TestFindProverJobs_ExcludesValidatedNodes(t *testing.T) {
	nodes := []*node.Node{
		createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),   // should be included
		createTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated), // should be excluded
	}

	result := jobs.FindProverJobs(nodes)

	if len(result) != 1 {
		t.Errorf("FindProverJobs() returned %d nodes, want 1", len(result))
	}

	if len(result) > 0 && result[0].ID.String() != "1" {
		t.Errorf("FindProverJobs() returned wrong node, got %s, want 1", result[0].ID.String())
	}
}

// TestFindProverJobs_ExcludesAdmittedNodes tests that admitted nodes are not
// returned as prover jobs.
func TestFindProverJobs_ExcludesAdmittedNodes(t *testing.T) {
	nodes := []*node.Node{
		createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),  // should be included
		createTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicAdmitted), // should be excluded
	}

	result := jobs.FindProverJobs(nodes)

	if len(result) != 1 {
		t.Errorf("FindProverJobs() returned %d nodes, want 1", len(result))
	}

	if len(result) > 0 && result[0].ID.String() != "1" {
		t.Errorf("FindProverJobs() returned wrong node, got %s, want 1", result[0].ID.String())
	}
}

// TestFindProverJobs_ExcludesRefutedNodes tests that refuted nodes are not
// returned as prover jobs.
func TestFindProverJobs_ExcludesRefutedNodes(t *testing.T) {
	nodes := []*node.Node{
		createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending), // should be included
		createTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicRefuted), // should be excluded
	}

	result := jobs.FindProverJobs(nodes)

	if len(result) != 1 {
		t.Errorf("FindProverJobs() returned %d nodes, want 1", len(result))
	}

	if len(result) > 0 && result[0].ID.String() != "1" {
		t.Errorf("FindProverJobs() returned wrong node, got %s, want 1", result[0].ID.String())
	}
}

// TestFindProverJobs_ExcludesArchivedNodes tests that archived nodes are not
// returned as prover jobs.
func TestFindProverJobs_ExcludesArchivedNodes(t *testing.T) {
	nodes := []*node.Node{
		createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),  // should be included
		createTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicArchived), // should be excluded
	}

	result := jobs.FindProverJobs(nodes)

	if len(result) != 1 {
		t.Errorf("FindProverJobs() returned %d nodes, want 1", len(result))
	}

	if len(result) > 0 && result[0].ID.String() != "1" {
		t.Errorf("FindProverJobs() returned wrong node, got %s, want 1", result[0].ID.String())
	}
}

// TestFindProverJobs_ExcludesBlockedNodes tests that blocked nodes are not
// returned as prover jobs.
func TestFindProverJobs_ExcludesBlockedNodes(t *testing.T) {
	nodes := []*node.Node{
		createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending), // should be included
		createTestNode(t, "1.1", schema.WorkflowBlocked, schema.EpistemicPending), // should be excluded
	}

	result := jobs.FindProverJobs(nodes)

	if len(result) != 1 {
		t.Errorf("FindProverJobs() returned %d nodes, want 1", len(result))
	}

	if len(result) > 0 && result[0].ID.String() != "1" {
		t.Errorf("FindProverJobs() returned wrong node, got %s, want 1", result[0].ID.String())
	}
}

// TestFindProverJobs_EmptyInput tests that FindProverJobs handles empty input.
func TestFindProverJobs_EmptyInput(t *testing.T) {
	result := jobs.FindProverJobs(nil)
	if len(result) != 0 {
		t.Errorf("FindProverJobs(nil) returned %d nodes, want 0", len(result))
	}

	result = jobs.FindProverJobs([]*node.Node{})
	if len(result) != 0 {
		t.Errorf("FindProverJobs([]) returned %d nodes, want 0", len(result))
	}
}

// TestFindProverJobs_MixedStates tests FindProverJobs with a mix of different states.
func TestFindProverJobs_MixedStates(t *testing.T) {
	nodes := []*node.Node{
		createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),     // include
		createTestNode(t, "1.1", schema.WorkflowClaimed, schema.EpistemicPending),     // exclude: claimed
		createTestNode(t, "1.2", schema.WorkflowBlocked, schema.EpistemicPending),     // exclude: blocked
		createTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicValidated), // exclude: validated
		createTestNode(t, "1.4", schema.WorkflowAvailable, schema.EpistemicAdmitted),  // exclude: admitted
		createTestNode(t, "1.5", schema.WorkflowAvailable, schema.EpistemicRefuted),   // exclude: refuted
		createTestNode(t, "1.6", schema.WorkflowAvailable, schema.EpistemicArchived),  // exclude: archived
		createTestNode(t, "1.7", schema.WorkflowAvailable, schema.EpistemicPending),   // include
		createTestNode(t, "1.8", schema.WorkflowClaimed, schema.EpistemicValidated),   // exclude: both
	}

	result := jobs.FindProverJobs(nodes)

	// Only nodes 1 and 1.7 should be included
	if len(result) != 2 {
		t.Errorf("FindProverJobs() returned %d nodes, want 2", len(result))
	}

	// Verify the correct nodes are returned
	expectedIDs := map[string]bool{"1": true, "1.7": true}
	for _, n := range result {
		if !expectedIDs[n.ID.String()] {
			t.Errorf("FindProverJobs() returned unexpected node %s", n.ID.String())
		}
		delete(expectedIDs, n.ID.String())
	}

	if len(expectedIDs) > 0 {
		for id := range expectedIDs {
			t.Errorf("FindProverJobs() did not return expected node %s", id)
		}
	}
}

// TestFindProverJobs_PreservesOrder tests that the order of returned nodes
// matches the input order (for nodes that qualify).
func TestFindProverJobs_PreservesOrder(t *testing.T) {
	nodes := []*node.Node{
		createTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending),
		createTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending),
		createTestNode(t, "1.2", schema.WorkflowClaimed, schema.EpistemicPending),   // excluded
		createTestNode(t, "1.4", schema.WorkflowAvailable, schema.EpistemicPending),
	}

	result := jobs.FindProverJobs(nodes)

	if len(result) != 3 {
		t.Errorf("FindProverJobs() returned %d nodes, want 3", len(result))
		return
	}

	// Verify order matches input (excluding filtered nodes)
	expectedOrder := []string{"1.3", "1.1", "1.4"}
	for i, n := range result {
		if n.ID.String() != expectedOrder[i] {
			t.Errorf("FindProverJobs() result[%d] = %s, want %s", i, n.ID.String(), expectedOrder[i])
		}
	}
}

// TestFindProverJobs_ReturnsOriginalPointers tests that the returned slice
// contains pointers to the original nodes (not copies).
func TestFindProverJobs_ReturnsOriginalPointers(t *testing.T) {
	original := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	nodes := []*node.Node{original}

	result := jobs.FindProverJobs(nodes)

	if len(result) != 1 {
		t.Fatalf("FindProverJobs() returned %d nodes, want 1", len(result))
	}

	// Verify it's the same pointer
	if result[0] != original {
		t.Error("FindProverJobs() should return pointers to original nodes")
	}
}

// TestFindProverJobs_AllNodesExcluded tests the case where all input nodes
// are excluded.
func TestFindProverJobs_AllNodesExcluded(t *testing.T) {
	nodes := []*node.Node{
		createTestNode(t, "1", schema.WorkflowClaimed, schema.EpistemicPending),
		createTestNode(t, "1.1", schema.WorkflowBlocked, schema.EpistemicPending),
		createTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated),
	}

	result := jobs.FindProverJobs(nodes)

	if len(result) != 0 {
		t.Errorf("FindProverJobs() returned %d nodes, want 0", len(result))
	}
}
