//go:build integration

// Package jobs_test contains tests for the jobs package facade.
package jobs_test

import (
	"testing"

	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// createJobsTestNode creates a test node with specific states for jobs facade testing.
func createJobsTestNode(t *testing.T, idStr string, workflow schema.WorkflowState, epistemic schema.EpistemicState) *node.Node {
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

// buildJobsNodeMap builds a map from node ID string to node pointer.
func buildJobsNodeMap(nodes []*node.Node) map[string]*node.Node {
	m := make(map[string]*node.Node)
	for _, n := range nodes {
		m[n.ID.String()] = n
	}
	return m
}

// TestFindJobs_EmptyInput tests that FindJobs handles empty input.
func TestFindJobs_EmptyInput(t *testing.T) {
	// Test with nil slice
	result := jobs.FindJobs(nil, nil)
	if result == nil {
		t.Fatal("FindJobs(nil, nil) returned nil, want non-nil JobResult")
	}
	if len(result.ProverJobs) != 0 {
		t.Errorf("FindJobs(nil, nil).ProverJobs has %d nodes, want 0", len(result.ProverJobs))
	}
	if len(result.VerifierJobs) != 0 {
		t.Errorf("FindJobs(nil, nil).VerifierJobs has %d nodes, want 0", len(result.VerifierJobs))
	}

	// Test with empty slice
	result = jobs.FindJobs([]*node.Node{}, map[string]*node.Node{})
	if result == nil {
		t.Fatal("FindJobs([], {}) returned nil, want non-nil JobResult")
	}
	if len(result.ProverJobs) != 0 {
		t.Errorf("FindJobs([], {}).ProverJobs has %d nodes, want 0", len(result.ProverJobs))
	}
	if len(result.VerifierJobs) != 0 {
		t.Errorf("FindJobs([], {}).VerifierJobs has %d nodes, want 0", len(result.VerifierJobs))
	}
}

// TestFindJobs_ReturnsBothProverAndVerifierJobs tests that FindJobs returns both types of jobs.
func TestFindJobs_ReturnsBothProverAndVerifierJobs(t *testing.T) {
	// Create a prover job: available + pending
	proverNode := createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending)

	// Create a verifier job: claimed + pending + no children (or all children validated)
	verifierNode := createJobsTestNode(t, "1.2", schema.WorkflowClaimed, schema.EpistemicPending)

	nodes := []*node.Node{proverNode, verifierNode}
	nodeMap := buildJobsNodeMap(nodes)

	result := jobs.FindJobs(nodes, nodeMap)

	if result == nil {
		t.Fatal("FindJobs() returned nil")
	}

	// Check prover jobs
	if len(result.ProverJobs) != 1 {
		t.Errorf("FindJobs().ProverJobs has %d nodes, want 1", len(result.ProverJobs))
	} else if result.ProverJobs[0].ID.String() != "1.1" {
		t.Errorf("FindJobs().ProverJobs[0].ID = %s, want 1.1", result.ProverJobs[0].ID.String())
	}

	// Check verifier jobs
	if len(result.VerifierJobs) != 1 {
		t.Errorf("FindJobs().VerifierJobs has %d nodes, want 1", len(result.VerifierJobs))
	} else if result.VerifierJobs[0].ID.String() != "1.2" {
		t.Errorf("FindJobs().VerifierJobs[0].ID = %s, want 1.2", result.VerifierJobs[0].ID.String())
	}
}

// TestFindJobs_HandlesNodesNeitherProverNorVerifierJobs tests nodes that don't qualify for either job type.
func TestFindJobs_HandlesNodesNeitherProverNorVerifierJobs(t *testing.T) {
	// Create nodes that are neither prover nor verifier jobs
	nodes := []*node.Node{
		// Blocked + pending: not a prover job (not available), not a verifier job (not claimed)
		createJobsTestNode(t, "1", schema.WorkflowBlocked, schema.EpistemicPending),
		// Available + validated: not a prover job (not pending), not a verifier job (not claimed)
		createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated),
		// Claimed + validated: not a prover job (not available), not a verifier job (not pending)
		createJobsTestNode(t, "1.2", schema.WorkflowClaimed, schema.EpistemicValidated),
	}
	nodeMap := buildJobsNodeMap(nodes)

	result := jobs.FindJobs(nodes, nodeMap)

	if result == nil {
		t.Fatal("FindJobs() returned nil")
	}
	if len(result.ProverJobs) != 0 {
		t.Errorf("FindJobs().ProverJobs has %d nodes, want 0", len(result.ProverJobs))
	}
	if len(result.VerifierJobs) != 0 {
		t.Errorf("FindJobs().VerifierJobs has %d nodes, want 0", len(result.VerifierJobs))
	}
}

// TestFindJobs_PreservesOrdering tests that the order of returned nodes matches the input order.
func TestFindJobs_PreservesOrdering(t *testing.T) {
	// Create nodes in specific order
	nodes := []*node.Node{
		createJobsTestNode(t, "1.4", schema.WorkflowAvailable, schema.EpistemicPending), // prover job
		createJobsTestNode(t, "1.2", schema.WorkflowClaimed, schema.EpistemicPending),   // verifier job
		createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending), // prover job
		createJobsTestNode(t, "1.3", schema.WorkflowClaimed, schema.EpistemicPending),   // verifier job
	}
	nodeMap := buildJobsNodeMap(nodes)

	result := jobs.FindJobs(nodes, nodeMap)

	if result == nil {
		t.Fatal("FindJobs() returned nil")
	}

	// Check prover jobs order: 1.4, 1.1 (in input order)
	if len(result.ProverJobs) != 2 {
		t.Errorf("FindJobs().ProverJobs has %d nodes, want 2", len(result.ProverJobs))
	} else {
		expectedProverOrder := []string{"1.4", "1.1"}
		for i, n := range result.ProverJobs {
			if n.ID.String() != expectedProverOrder[i] {
				t.Errorf("FindJobs().ProverJobs[%d].ID = %s, want %s", i, n.ID.String(), expectedProverOrder[i])
			}
		}
	}

	// Check verifier jobs order: 1.2, 1.3 (in input order)
	if len(result.VerifierJobs) != 2 {
		t.Errorf("FindJobs().VerifierJobs has %d nodes, want 2", len(result.VerifierJobs))
	} else {
		expectedVerifierOrder := []string{"1.2", "1.3"}
		for i, n := range result.VerifierJobs {
			if n.ID.String() != expectedVerifierOrder[i] {
				t.Errorf("FindJobs().VerifierJobs[%d].ID = %s, want %s", i, n.ID.String(), expectedVerifierOrder[i])
			}
		}
	}
}

// TestFindJobs_MixedNodeStates tests FindJobs with a variety of node states.
func TestFindJobs_MixedNodeStates(t *testing.T) {
	nodes := []*node.Node{
		// Prover jobs (available + pending)
		createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),
		createJobsTestNode(t, "1.5", schema.WorkflowAvailable, schema.EpistemicPending),

		// Verifier jobs (claimed + pending + no unvalidated children)
		createJobsTestNode(t, "1.1", schema.WorkflowClaimed, schema.EpistemicPending),
		createJobsTestNode(t, "1.3", schema.WorkflowClaimed, schema.EpistemicPending),

		// Neither
		createJobsTestNode(t, "1.2", schema.WorkflowBlocked, schema.EpistemicPending),
		createJobsTestNode(t, "1.4", schema.WorkflowAvailable, schema.EpistemicValidated),
		createJobsTestNode(t, "1.6", schema.WorkflowClaimed, schema.EpistemicAdmitted),
	}
	nodeMap := buildJobsNodeMap(nodes)

	result := jobs.FindJobs(nodes, nodeMap)

	if result == nil {
		t.Fatal("FindJobs() returned nil")
	}

	// Verify prover jobs
	expectedProver := map[string]bool{"1": true, "1.5": true}
	if len(result.ProverJobs) != len(expectedProver) {
		t.Errorf("FindJobs().ProverJobs has %d nodes, want %d", len(result.ProverJobs), len(expectedProver))
	}
	for _, n := range result.ProverJobs {
		if !expectedProver[n.ID.String()] {
			t.Errorf("FindJobs().ProverJobs contains unexpected node %s", n.ID.String())
		}
	}

	// Verify verifier jobs
	expectedVerifier := map[string]bool{"1.1": true, "1.3": true}
	if len(result.VerifierJobs) != len(expectedVerifier) {
		t.Errorf("FindJobs().VerifierJobs has %d nodes, want %d", len(result.VerifierJobs), len(expectedVerifier))
	}
	for _, n := range result.VerifierJobs {
		if !expectedVerifier[n.ID.String()] {
			t.Errorf("FindJobs().VerifierJobs contains unexpected node %s", n.ID.String())
		}
	}
}

// TestFindJobs_OnlyProverJobs tests when only prover jobs exist.
func TestFindJobs_OnlyProverJobs(t *testing.T) {
	nodes := []*node.Node{
		createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),
		createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending),
		createJobsTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending),
	}
	nodeMap := buildJobsNodeMap(nodes)

	result := jobs.FindJobs(nodes, nodeMap)

	if result == nil {
		t.Fatal("FindJobs() returned nil")
	}
	if len(result.ProverJobs) != 3 {
		t.Errorf("FindJobs().ProverJobs has %d nodes, want 3", len(result.ProverJobs))
	}
	if len(result.VerifierJobs) != 0 {
		t.Errorf("FindJobs().VerifierJobs has %d nodes, want 0", len(result.VerifierJobs))
	}
}

// TestFindJobs_OnlyVerifierJobs tests when only verifier jobs exist.
func TestFindJobs_OnlyVerifierJobs(t *testing.T) {
	// Use sibling leaf nodes (1.1, 1.2, 1.3) which are all claimed + pending
	// and have no children of their own, making them all valid verifier jobs.
	nodes := []*node.Node{
		createJobsTestNode(t, "1.1", schema.WorkflowClaimed, schema.EpistemicPending), // leaf, verifier job
		createJobsTestNode(t, "1.2", schema.WorkflowClaimed, schema.EpistemicPending), // leaf, verifier job
		createJobsTestNode(t, "1.3", schema.WorkflowClaimed, schema.EpistemicPending), // leaf, verifier job
	}
	nodeMap := buildJobsNodeMap(nodes)

	result := jobs.FindJobs(nodes, nodeMap)

	if result == nil {
		t.Fatal("FindJobs() returned nil")
	}
	if len(result.ProverJobs) != 0 {
		t.Errorf("FindJobs().ProverJobs has %d nodes, want 0", len(result.ProverJobs))
	}
	if len(result.VerifierJobs) != 3 {
		t.Errorf("FindJobs().VerifierJobs has %d nodes, want 3", len(result.VerifierJobs))
	}
}

// TestFindJobs_ReturnsOriginalPointers tests that the returned slices contain pointers to the original nodes.
func TestFindJobs_ReturnsOriginalPointers(t *testing.T) {
	proverNode := createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	verifierNode := createJobsTestNode(t, "1.1", schema.WorkflowClaimed, schema.EpistemicPending)

	nodes := []*node.Node{proverNode, verifierNode}
	nodeMap := buildJobsNodeMap(nodes)

	result := jobs.FindJobs(nodes, nodeMap)

	if result == nil {
		t.Fatal("FindJobs() returned nil")
	}

	// Check prover job pointer
	if len(result.ProverJobs) != 1 {
		t.Fatalf("FindJobs().ProverJobs has %d nodes, want 1", len(result.ProverJobs))
	}
	if result.ProverJobs[0] != proverNode {
		t.Error("FindJobs().ProverJobs[0] is not the same pointer as the original node")
	}

	// Check verifier job pointer
	if len(result.VerifierJobs) != 1 {
		t.Fatalf("FindJobs().VerifierJobs has %d nodes, want 1", len(result.VerifierJobs))
	}
	if result.VerifierJobs[0] != verifierNode {
		t.Error("FindJobs().VerifierJobs[0] is not the same pointer as the original node")
	}
}

// TestFindJobs_VerifierJobsRequireValidatedChildren tests that verifier jobs must have all children validated.
func TestFindJobs_VerifierJobsRequireValidatedChildren(t *testing.T) {
	// Parent is claimed + pending, but child is not validated
	parent := createJobsTestNode(t, "1", schema.WorkflowClaimed, schema.EpistemicPending)
	child := createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending) // Not validated

	nodes := []*node.Node{parent, child}
	nodeMap := buildJobsNodeMap(nodes)

	result := jobs.FindJobs(nodes, nodeMap)

	if result == nil {
		t.Fatal("FindJobs() returned nil")
	}

	// Parent should NOT be a verifier job because child is not validated
	for _, n := range result.VerifierJobs {
		if n.ID.String() == "1" {
			t.Error("FindJobs().VerifierJobs should not contain node 1 because it has unvalidated children")
		}
	}

	// Child should be a prover job
	found := false
	for _, n := range result.ProverJobs {
		if n.ID.String() == "1.1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("FindJobs().ProverJobs should contain node 1.1")
	}
}

// TestFindJobs_VerifierJobWithValidatedChildren tests that a node with all validated children is a verifier job.
func TestFindJobs_VerifierJobWithValidatedChildren(t *testing.T) {
	// Parent is claimed + pending, all children are validated
	parent := createJobsTestNode(t, "1", schema.WorkflowClaimed, schema.EpistemicPending)
	child1 := createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated)
	child2 := createJobsTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated)

	nodes := []*node.Node{parent, child1, child2}
	nodeMap := buildJobsNodeMap(nodes)

	result := jobs.FindJobs(nodes, nodeMap)

	if result == nil {
		t.Fatal("FindJobs() returned nil")
	}

	// Parent should be a verifier job because all children are validated
	found := false
	for _, n := range result.VerifierJobs {
		if n.ID.String() == "1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("FindJobs().VerifierJobs should contain node 1 because all its children are validated")
	}
}

// TestJobResult_IsEmpty tests the IsEmpty method on JobResult.
func TestJobResult_IsEmpty(t *testing.T) {
	// Empty result
	emptyResult := &jobs.JobResult{}
	if !emptyResult.IsEmpty() {
		t.Error("JobResult.IsEmpty() should return true for empty result")
	}

	// Result with prover jobs
	proverResult := &jobs.JobResult{
		ProverJobs: []*node.Node{createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)},
	}
	if proverResult.IsEmpty() {
		t.Error("JobResult.IsEmpty() should return false when ProverJobs is non-empty")
	}

	// Result with verifier jobs
	verifierResult := &jobs.JobResult{
		VerifierJobs: []*node.Node{createJobsTestNode(t, "1.1", schema.WorkflowClaimed, schema.EpistemicPending)},
	}
	if verifierResult.IsEmpty() {
		t.Error("JobResult.IsEmpty() should return false when VerifierJobs is non-empty")
	}

	// Result with both
	bothResult := &jobs.JobResult{
		ProverJobs:   []*node.Node{createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)},
		VerifierJobs: []*node.Node{createJobsTestNode(t, "1.1", schema.WorkflowClaimed, schema.EpistemicPending)},
	}
	if bothResult.IsEmpty() {
		t.Error("JobResult.IsEmpty() should return false when both job slices are non-empty")
	}
}

// TestJobResult_TotalCount tests the TotalCount method on JobResult.
func TestJobResult_TotalCount(t *testing.T) {
	tests := []struct {
		name           string
		proverCount    int
		verifierCount  int
		expectedTotal  int
	}{
		{"empty", 0, 0, 0},
		{"only prover", 3, 0, 3},
		{"only verifier", 0, 2, 2},
		{"both", 2, 3, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &jobs.JobResult{}

			// Add prover jobs
			for i := 0; i < tt.proverCount; i++ {
				result.ProverJobs = append(result.ProverJobs,
					createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending))
			}

			// Add verifier jobs
			for i := 0; i < tt.verifierCount; i++ {
				result.VerifierJobs = append(result.VerifierJobs,
					createJobsTestNode(t, "1.1", schema.WorkflowClaimed, schema.EpistemicPending))
			}

			if got := result.TotalCount(); got != tt.expectedTotal {
				t.Errorf("JobResult.TotalCount() = %d, want %d", got, tt.expectedTotal)
			}
		})
	}
}
