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

// createTestChallenge creates a test challenge for a node.
func createTestChallenge(t *testing.T, id string, targetID types.NodeID, status node.ChallengeStatus) *node.Challenge {
	t.Helper()
	c, err := node.NewChallenge(id, targetID, schema.TargetStatement, "Test challenge reason")
	if err != nil {
		t.Fatalf("node.NewChallenge() error: %v", err)
	}
	c.Status = status
	return c
}

// buildNodeMap builds a map from node ID string to node pointer.
func buildNodeMap(nodes []*node.Node) map[string]*node.Node {
	m := make(map[string]*node.Node)
	for _, n := range nodes {
		m[n.ID.String()] = n
	}
	return m
}

// buildChallengeMap builds a map from node ID string to challenges on that node.
func buildChallengeMap(challenges []*node.Challenge) map[string][]*node.Challenge {
	m := make(map[string][]*node.Challenge)
	for _, c := range challenges {
		key := c.TargetID.String()
		m[key] = append(m[key], c)
	}
	return m
}

// TestFindVerifierJobs_EmptyInput tests that FindVerifierJobs handles empty input.
func TestFindVerifierJobs_EmptyInput(t *testing.T) {
	// Test with nil slice
	result := jobs.FindVerifierJobs(nil, nil, nil)
	if len(result) != 0 {
		t.Errorf("FindVerifierJobs(nil, nil, nil) returned %d nodes, want 0", len(result))
	}

	// Test with empty slice
	result = jobs.FindVerifierJobs([]*node.Node{}, map[string]*node.Node{}, map[string][]*node.Challenge{})
	if len(result) != 0 {
		t.Errorf("FindVerifierJobs([], {}, {}) returned %d nodes, want 0", len(result))
	}
}

// TestFindVerifierJobs_BreadthFirst_NewNodeIsVerifierJob tests the key behavior change:
// A new pending node with no challenges should immediately be a verifier job.
// This is the "breadth-first" model where verifiers can immediately review any new node.
func TestFindVerifierJobs_BreadthFirst_NewNodeIsVerifierJob(t *testing.T) {
	// A new node that is available and pending with no challenges should be a verifier job
	n := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	nodes := []*node.Node{n}
	nodeMap := buildNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{} // No challenges

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	if len(result) != 1 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 1", len(result))
	}
	if len(result) > 0 && result[0].ID.String() != "1" {
		t.Errorf("FindVerifierJobs() returned %s, want 1", result[0].ID.String())
	}
}

// TestFindVerifierJobs_NodeWithOpenChallengeIsNotVerifierJob tests that a node
// with an open challenge is NOT a verifier job (it's a prover job).
func TestFindVerifierJobs_NodeWithOpenChallengeIsNotVerifierJob(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	challenge := createTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusOpen)

	nodes := []*node.Node{n}
	nodeMap := buildNodeMap(nodes)
	challengeMap := buildChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	if len(result) != 0 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 0 (node has open challenge)", len(result))
	}
}

// TestFindVerifierJobs_NodeWithResolvedChallengeIsVerifierJob tests that a node
// with only resolved challenges IS a verifier job.
func TestFindVerifierJobs_NodeWithResolvedChallengeIsVerifierJob(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	challenge := createTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusResolved)

	nodes := []*node.Node{n}
	nodeMap := buildNodeMap(nodes)
	challengeMap := buildChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	if len(result) != 1 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 1 (challenge is resolved)", len(result))
	}
}

// TestFindVerifierJobs_NodeWithWithdrawnChallengeIsVerifierJob tests that a node
// with only withdrawn challenges IS a verifier job.
func TestFindVerifierJobs_NodeWithWithdrawnChallengeIsVerifierJob(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	challenge := createTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusWithdrawn)

	nodes := []*node.Node{n}
	nodeMap := buildNodeMap(nodes)
	challengeMap := buildChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	if len(result) != 1 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 1 (challenge is withdrawn)", len(result))
	}
}

// TestFindVerifierJobs_NodeWithMixedChallengesNotVerifierJob tests that a node
// with mixed challenges (some open, some resolved) is NOT a verifier job.
func TestFindVerifierJobs_NodeWithMixedChallengesNotVerifierJob(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	resolvedChallenge := createTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusResolved)
	openChallenge := createTestChallenge(t, "ch-2", nodeID, node.ChallengeStatusOpen)

	nodes := []*node.Node{n}
	nodeMap := buildNodeMap(nodes)
	challengeMap := buildChallengeMap([]*node.Challenge{resolvedChallenge, openChallenge})

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	if len(result) != 0 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 0 (node has open challenge)", len(result))
	}
}

// TestFindVerifierJobs_ClaimedNodeIsNotVerifierJob tests that a claimed node
// is NOT a verifier job (someone is already working on it).
func TestFindVerifierJobs_ClaimedNodeIsNotVerifierJob(t *testing.T) {
	n := createVerifierTestNode(t, "1", schema.WorkflowClaimed, schema.EpistemicPending)
	nodes := []*node.Node{n}
	nodeMap := buildNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{}

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	if len(result) != 0 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 0 (node is claimed)", len(result))
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
			n := createVerifierTestNode(t, "1", schema.WorkflowAvailable, tt.epistemic)
			nodes := []*node.Node{n}
			nodeMap := buildNodeMap(nodes)
			challengeMap := map[string][]*node.Challenge{}

			result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

			gotJob := len(result) > 0
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
	n := createVerifierTestNode(t, "1", schema.WorkflowBlocked, schema.EpistemicPending)
	nodes := []*node.Node{n}
	nodeMap := buildNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{}

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	if len(result) != 0 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 0 (node is blocked)", len(result))
	}
}

// TestFindVerifierJobs_MultipleVerifierJobs tests finding multiple verifier jobs.
func TestFindVerifierJobs_MultipleVerifierJobs(t *testing.T) {
	node1 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending)
	node2 := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending)
	node3 := createVerifierTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending)

	nodes := []*node.Node{node1, node2, node3}
	nodeMap := buildNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{}

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	if len(result) != 3 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 3", len(result))
	}
}

// TestFindVerifierJobs_PreservesOrder tests that the order of returned nodes
// matches the input order.
func TestFindVerifierJobs_PreservesOrder(t *testing.T) {
	node3 := createVerifierTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending)
	node1 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending)
	node2 := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending)

	nodes := []*node.Node{node3, node1, node2}
	nodeMap := buildNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{}

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

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
	nodes := []*node.Node{original}
	nodeMap := buildNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{}

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

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
	nodeID1, _ := types.Parse("1")
	nodes := []*node.Node{
		createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),   // has open challenge
		createVerifierTestNode(t, "1.1", schema.WorkflowBlocked, schema.EpistemicPending),   // blocked
		createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated), // not pending
		createVerifierTestNode(t, "1.3", schema.WorkflowClaimed, schema.EpistemicPending),   // claimed
	}
	nodeMap := buildNodeMap(nodes)
	challengeMap := buildChallengeMap([]*node.Challenge{
		createTestChallenge(t, "ch-1", nodeID1, node.ChallengeStatusOpen),
	})

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	if len(result) != 0 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 0", len(result))
	}
}

// TestFindVerifierJobs_MixedStates tests FindVerifierJobs with a mix of different states.
func TestFindVerifierJobs_MixedStates(t *testing.T) {
	nodeID1, _ := types.Parse("1")
	nodeID5, _ := types.Parse("1.5")

	nodes := []*node.Node{
		createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),     // has open challenge -> not verifier
		createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending),   // no challenges -> verifier
		createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated), // not pending -> not verifier
		createVerifierTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending),   // no challenges -> verifier
		createVerifierTestNode(t, "1.4", schema.WorkflowBlocked, schema.EpistemicPending),     // blocked -> not verifier
		createVerifierTestNode(t, "1.5", schema.WorkflowAvailable, schema.EpistemicPending),   // resolved challenge -> verifier
		createVerifierTestNode(t, "1.6", schema.WorkflowClaimed, schema.EpistemicPending),     // claimed -> not verifier
	}
	nodeMap := buildNodeMap(nodes)
	challengeMap := buildChallengeMap([]*node.Challenge{
		createTestChallenge(t, "ch-1", nodeID1, node.ChallengeStatusOpen),
		createTestChallenge(t, "ch-2", nodeID5, node.ChallengeStatusResolved),
	})

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	// Nodes 1.1, 1.3, and 1.5 should be verifier jobs
	expectedIDs := map[string]bool{"1.1": true, "1.3": true, "1.5": true}
	if len(result) != len(expectedIDs) {
		t.Errorf("FindVerifierJobs() returned %d nodes, want %d", len(result), len(expectedIDs))
	}
	for _, n := range result {
		if !expectedIDs[n.ID.String()] {
			t.Errorf("FindVerifierJobs() returned unexpected node %s", n.ID.String())
		}
	}
}

// TestFindVerifierJobs_ChildrenDontAffectVerifierStatus tests that children's states
// no longer affect whether a node is a verifier job (breadth-first model).
func TestFindVerifierJobs_ChildrenDontAffectVerifierStatus(t *testing.T) {
	// In the new breadth-first model, having unvalidated children should NOT
	// prevent a node from being a verifier job. Only challenges matter.
	parent := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	child1 := createVerifierTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending) // Not validated
	child2 := createVerifierTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending) // Not validated

	nodes := []*node.Node{parent, child1, child2}
	nodeMap := buildNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{} // No challenges

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	// ALL nodes should be verifier jobs (no challenges, all pending and available)
	if len(result) != 3 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 3 (children don't affect parent in breadth-first model)", len(result))
	}
}

// TestFindVerifierJobs_LeafNodeIsVerifierJob tests that a leaf node (no children)
// IS a verifier job in the breadth-first model.
func TestFindVerifierJobs_LeafNodeIsVerifierJob(t *testing.T) {
	// In the old model, leaf nodes were NOT verifier jobs.
	// In the new breadth-first model, leaf nodes ARE verifier jobs.
	n := createVerifierTestNode(t, "1.1.1", schema.WorkflowAvailable, schema.EpistemicPending)
	nodes := []*node.Node{n}
	nodeMap := buildNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{}

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	if len(result) != 1 {
		t.Errorf("FindVerifierJobs() returned %d nodes, want 1 (leaf nodes are verifier jobs in breadth-first)", len(result))
	}
}

// TestFindVerifierJobs_NilChallengeMapTreatedAsEmpty tests that nil challengeMap
// is treated as empty (backward compatibility).
func TestFindVerifierJobs_NilChallengeMapTreatedAsEmpty(t *testing.T) {
	n := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	nodes := []*node.Node{n}
	nodeMap := buildNodeMap(nodes)

	result := jobs.FindVerifierJobs(nodes, nodeMap, nil)

	if len(result) != 1 {
		t.Errorf("FindVerifierJobs() with nil challengeMap returned %d nodes, want 1", len(result))
	}
}

// TestFindVerifierJobs_EmptyStatementNodeIsNotVerifierJob tests that a node
// with an empty statement is not a verifier job.
// Note: In practice, nodes should always have statements due to validation,
// but we test this edge case for completeness.
func TestFindVerifierJobs_NodeMustHaveStatement(t *testing.T) {
	// All nodes created via NewNode have statements, so we test the normal case
	// This test verifies that nodes with statements qualify
	n := createVerifierTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	if n.Statement == "" {
		t.Fatal("Test setup error: node should have a statement")
	}

	nodes := []*node.Node{n}
	nodeMap := buildNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{}

	result := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

	if len(result) != 1 {
		t.Errorf("FindVerifierJobs() should return node with statement")
	}
}
