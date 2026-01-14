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

// createJobsTestChallenge creates a test challenge for a node.
func createJobsTestChallenge(t *testing.T, id string, targetID types.NodeID, status node.ChallengeStatus) *node.Challenge {
	t.Helper()
	c, err := node.NewChallenge(id, targetID, schema.TargetStatement, "Test challenge reason")
	if err != nil {
		t.Fatalf("node.NewChallenge() error: %v", err)
	}
	c.Status = status
	return c
}

// buildJobsNodeMap builds a map from node ID string to node pointer.
func buildJobsNodeMap(nodes []*node.Node) map[string]*node.Node {
	m := make(map[string]*node.Node)
	for _, n := range nodes {
		m[n.ID.String()] = n
	}
	return m
}

// buildJobsChallengeMap builds a map from node ID string to challenges on that node.
func buildJobsChallengeMap(challenges []*node.Challenge) map[string][]*node.Challenge {
	m := make(map[string][]*node.Challenge)
	for _, c := range challenges {
		key := c.TargetID.String()
		m[key] = append(m[key], c)
	}
	return m
}

// TestFindJobs_EmptyInput tests that FindJobs handles empty input.
func TestFindJobs_EmptyInput(t *testing.T) {
	// Test with nil slice
	result := jobs.FindJobs(nil, nil, nil)
	if result == nil {
		t.Fatal("FindJobs(nil, nil, nil) returned nil, want non-nil JobResult")
	}
	if len(result.ProverJobs) != 0 {
		t.Errorf("FindJobs(nil, nil, nil).ProverJobs has %d nodes, want 0", len(result.ProverJobs))
	}
	if len(result.VerifierJobs) != 0 {
		t.Errorf("FindJobs(nil, nil, nil).VerifierJobs has %d nodes, want 0", len(result.VerifierJobs))
	}

	// Test with empty slice
	result = jobs.FindJobs([]*node.Node{}, map[string]*node.Node{}, map[string][]*node.Challenge{})
	if result == nil {
		t.Fatal("FindJobs([], {}, {}) returned nil, want non-nil JobResult")
	}
	if len(result.ProverJobs) != 0 {
		t.Errorf("FindJobs([], {}, {}).ProverJobs has %d nodes, want 0", len(result.ProverJobs))
	}
	if len(result.VerifierJobs) != 0 {
		t.Errorf("FindJobs([], {}, {}).VerifierJobs has %d nodes, want 0", len(result.VerifierJobs))
	}
}

// TestFindJobs_BreadthFirstModel tests the new breadth-first adversarial model.
// New nodes with no challenges are verifier jobs.
// Challenged nodes are prover jobs.
func TestFindJobs_BreadthFirstModel(t *testing.T) {
	nodeID1, _ := types.Parse("1")

	// Node 1.1 has no challenges -> verifier job
	verifierNode := createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending)

	// Node 1 has an open challenge -> prover job
	proverNode := createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	challenge := createJobsTestChallenge(t, "ch-1", nodeID1, node.ChallengeStatusOpen)

	nodes := []*node.Node{verifierNode, proverNode}
	nodeMap := buildJobsNodeMap(nodes)
	challengeMap := buildJobsChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindJobs(nodes, nodeMap, challengeMap)

	if result == nil {
		t.Fatal("FindJobs() returned nil")
	}

	// Check prover jobs - only node 1 (has open challenge)
	if len(result.ProverJobs) != 1 {
		t.Errorf("FindJobs().ProverJobs has %d nodes, want 1", len(result.ProverJobs))
	} else if result.ProverJobs[0].ID.String() != "1" {
		t.Errorf("FindJobs().ProverJobs[0].ID = %s, want 1", result.ProverJobs[0].ID.String())
	}

	// Check verifier jobs - only node 1.1 (no challenges)
	if len(result.VerifierJobs) != 1 {
		t.Errorf("FindJobs().VerifierJobs has %d nodes, want 1", len(result.VerifierJobs))
	} else if result.VerifierJobs[0].ID.String() != "1.1" {
		t.Errorf("FindJobs().VerifierJobs[0].ID = %s, want 1.1", result.VerifierJobs[0].ID.String())
	}
}

// TestFindJobs_HandlesMixedStates tests FindJobs with a variety of node states.
func TestFindJobs_HandlesMixedStates(t *testing.T) {
	nodeID1, _ := types.Parse("1")
	nodeID3, _ := types.Parse("1.3")

	nodes := []*node.Node{
		// Prover jobs (has open challenge, pending, not blocked)
		createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),
		createJobsTestNode(t, "1.3", schema.WorkflowClaimed, schema.EpistemicPending), // claimed but has challenge

		// Verifier jobs (no challenges, pending, available)
		createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending),
		createJobsTestNode(t, "1.5", schema.WorkflowAvailable, schema.EpistemicPending),

		// Neither (various exclusion reasons)
		createJobsTestNode(t, "1.2", schema.WorkflowBlocked, schema.EpistemicPending),     // blocked
		createJobsTestNode(t, "1.4", schema.WorkflowAvailable, schema.EpistemicValidated), // not pending
		createJobsTestNode(t, "1.6", schema.WorkflowClaimed, schema.EpistemicPending),     // claimed, no challenge
	}
	nodeMap := buildJobsNodeMap(nodes)
	challengeMap := buildJobsChallengeMap([]*node.Challenge{
		createJobsTestChallenge(t, "ch-1", nodeID1, node.ChallengeStatusOpen),
		createJobsTestChallenge(t, "ch-3", nodeID3, node.ChallengeStatusOpen),
	})

	result := jobs.FindJobs(nodes, nodeMap, challengeMap)

	if result == nil {
		t.Fatal("FindJobs() returned nil")
	}

	// Verify prover jobs (1 and 1.3 have open challenges)
	expectedProver := map[string]bool{"1": true, "1.3": true}
	if len(result.ProverJobs) != len(expectedProver) {
		t.Errorf("FindJobs().ProverJobs has %d nodes, want %d", len(result.ProverJobs), len(expectedProver))
	}
	for _, n := range result.ProverJobs {
		if !expectedProver[n.ID.String()] {
			t.Errorf("FindJobs().ProverJobs contains unexpected node %s", n.ID.String())
		}
	}

	// Verify verifier jobs (1.1 and 1.5 have no challenges, are available and pending)
	expectedVerifier := map[string]bool{"1.1": true, "1.5": true}
	if len(result.VerifierJobs) != len(expectedVerifier) {
		t.Errorf("FindJobs().VerifierJobs has %d nodes, want %d", len(result.VerifierJobs), len(expectedVerifier))
	}
	for _, n := range result.VerifierJobs {
		if !expectedVerifier[n.ID.String()] {
			t.Errorf("FindJobs().VerifierJobs contains unexpected node %s", n.ID.String())
		}
	}
}

// TestFindJobs_PreservesOrdering tests that the order of returned nodes matches the input order.
func TestFindJobs_PreservesOrdering(t *testing.T) {
	nodeID4, _ := types.Parse("1.4")
	nodeID1, _ := types.Parse("1.1")

	// Create nodes in specific order
	nodes := []*node.Node{
		createJobsTestNode(t, "1.4", schema.WorkflowAvailable, schema.EpistemicPending), // prover (has challenge)
		createJobsTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending), // verifier (no challenge)
		createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending), // prover (has challenge)
		createJobsTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending), // verifier (no challenge)
	}
	nodeMap := buildJobsNodeMap(nodes)
	challengeMap := buildJobsChallengeMap([]*node.Challenge{
		createJobsTestChallenge(t, "ch-4", nodeID4, node.ChallengeStatusOpen),
		createJobsTestChallenge(t, "ch-1", nodeID1, node.ChallengeStatusOpen),
	})

	result := jobs.FindJobs(nodes, nodeMap, challengeMap)

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

// TestFindJobs_OnlyProverJobs tests when only prover jobs exist (all have challenges).
func TestFindJobs_OnlyProverJobs(t *testing.T) {
	nodeID1, _ := types.Parse("1")
	nodeID2, _ := types.Parse("1.1")
	nodeID3, _ := types.Parse("1.2")

	nodes := []*node.Node{
		createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),
		createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending),
		createJobsTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending),
	}
	nodeMap := buildJobsNodeMap(nodes)
	challengeMap := buildJobsChallengeMap([]*node.Challenge{
		createJobsTestChallenge(t, "ch-1", nodeID1, node.ChallengeStatusOpen),
		createJobsTestChallenge(t, "ch-2", nodeID2, node.ChallengeStatusOpen),
		createJobsTestChallenge(t, "ch-3", nodeID3, node.ChallengeStatusOpen),
	})

	result := jobs.FindJobs(nodes, nodeMap, challengeMap)

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

// TestFindJobs_OnlyVerifierJobs tests when only verifier jobs exist (none have challenges).
func TestFindJobs_OnlyVerifierJobs(t *testing.T) {
	nodes := []*node.Node{
		createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending),
		createJobsTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending),
		createJobsTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending),
	}
	nodeMap := buildJobsNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{} // No challenges

	result := jobs.FindJobs(nodes, nodeMap, challengeMap)

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
	nodeID, _ := types.Parse("1")
	proverNode := createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	verifierNode := createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending)
	challenge := createJobsTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusOpen)

	nodes := []*node.Node{proverNode, verifierNode}
	nodeMap := buildJobsNodeMap(nodes)
	challengeMap := buildJobsChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindJobs(nodes, nodeMap, challengeMap)

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

// TestFindJobs_ChallengeResolutionTransitionsJob tests that when a challenge is resolved,
// the node transitions from prover job to verifier job.
func TestFindJobs_ChallengeResolutionTransitionsJob(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)

	nodes := []*node.Node{n}
	nodeMap := buildJobsNodeMap(nodes)

	// With open challenge: prover job
	openChallenge := createJobsTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusOpen)
	challengeMap := buildJobsChallengeMap([]*node.Challenge{openChallenge})

	result := jobs.FindJobs(nodes, nodeMap, challengeMap)
	if len(result.ProverJobs) != 1 || len(result.VerifierJobs) != 0 {
		t.Errorf("With open challenge: got %d prover, %d verifier; want 1 prover, 0 verifier",
			len(result.ProverJobs), len(result.VerifierJobs))
	}

	// With resolved challenge: verifier job
	resolvedChallenge := createJobsTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusResolved)
	challengeMap = buildJobsChallengeMap([]*node.Challenge{resolvedChallenge})

	result = jobs.FindJobs(nodes, nodeMap, challengeMap)
	if len(result.ProverJobs) != 0 || len(result.VerifierJobs) != 1 {
		t.Errorf("With resolved challenge: got %d prover, %d verifier; want 0 prover, 1 verifier",
			len(result.ProverJobs), len(result.VerifierJobs))
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
		VerifierJobs: []*node.Node{createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending)},
	}
	if verifierResult.IsEmpty() {
		t.Error("JobResult.IsEmpty() should return false when VerifierJobs is non-empty")
	}

	// Result with both
	bothResult := &jobs.JobResult{
		ProverJobs:   []*node.Node{createJobsTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)},
		VerifierJobs: []*node.Node{createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending)},
	}
	if bothResult.IsEmpty() {
		t.Error("JobResult.IsEmpty() should return false when both job slices are non-empty")
	}
}

// TestJobResult_TotalCount tests the TotalCount method on JobResult.
func TestJobResult_TotalCount(t *testing.T) {
	tests := []struct {
		name          string
		proverCount   int
		verifierCount int
		expectedTotal int
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
					createJobsTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending))
			}

			if got := result.TotalCount(); got != tt.expectedTotal {
				t.Errorf("JobResult.TotalCount() = %d, want %d", got, tt.expectedTotal)
			}
		})
	}
}
