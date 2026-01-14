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

// createProverTestChallenge creates a test challenge for a node.
func createProverTestChallenge(t *testing.T, id string, targetID types.NodeID, status node.ChallengeStatus) *node.Challenge {
	t.Helper()
	c, err := node.NewChallenge(id, targetID, schema.TargetStatement, "Test challenge reason")
	if err != nil {
		t.Fatalf("node.NewChallenge() error: %v", err)
	}
	c.Status = status
	return c
}

// buildProverNodeMap builds a map from node ID string to node pointer.
func buildProverNodeMap(nodes []*node.Node) map[string]*node.Node {
	m := make(map[string]*node.Node)
	for _, n := range nodes {
		m[n.ID.String()] = n
	}
	return m
}

// buildProverChallengeMap builds a map from node ID string to challenges on that node.
func buildProverChallengeMap(challenges []*node.Challenge) map[string][]*node.Challenge {
	m := make(map[string][]*node.Challenge)
	for _, c := range challenges {
		key := c.TargetID.String()
		m[key] = append(m[key], c)
	}
	return m
}

// TestFindProverJobs_EmptyInput tests that FindProverJobs handles empty input.
func TestFindProverJobs_EmptyInput(t *testing.T) {
	result := jobs.FindProverJobs(nil, nil, nil)
	if len(result) != 0 {
		t.Errorf("FindProverJobs(nil, nil, nil) returned %d nodes, want 0", len(result))
	}

	result = jobs.FindProverJobs([]*node.Node{}, map[string]*node.Node{}, map[string][]*node.Challenge{})
	if len(result) != 0 {
		t.Errorf("FindProverJobs([], {}, {}) returned %d nodes, want 0", len(result))
	}
}

// TestFindProverJobs_NodeWithOpenChallengeIsProverJob tests the key behavior:
// A node with an open challenge is a prover job (provers address challenges).
func TestFindProverJobs_NodeWithOpenChallengeIsProverJob(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	challenge := createProverTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusOpen)

	nodes := []*node.Node{n}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 1 {
		t.Errorf("FindProverJobs() returned %d nodes, want 1 (node has open challenge)", len(result))
	}
	if len(result) > 0 && result[0].ID.String() != "1" {
		t.Errorf("FindProverJobs() returned %s, want 1", result[0].ID.String())
	}
}

// TestFindProverJobs_NodeWithNoChallengesIsNotProverJob tests that a node
// with no challenges is NOT a prover job (it's a verifier job).
func TestFindProverJobs_NodeWithNoChallengesIsNotProverJob(t *testing.T) {
	n := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)

	nodes := []*node.Node{n}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{} // No challenges

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 0 {
		t.Errorf("FindProverJobs() returned %d nodes, want 0 (node has no challenges)", len(result))
	}
}

// TestFindProverJobs_NodeWithResolvedChallengeIsNotProverJob tests that a node
// with only resolved challenges is NOT a prover job.
func TestFindProverJobs_NodeWithResolvedChallengeIsNotProverJob(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	challenge := createProverTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusResolved)

	nodes := []*node.Node{n}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 0 {
		t.Errorf("FindProverJobs() returned %d nodes, want 0 (challenge is resolved)", len(result))
	}
}

// TestFindProverJobs_NodeWithWithdrawnChallengeIsNotProverJob tests that a node
// with only withdrawn challenges is NOT a prover job.
func TestFindProverJobs_NodeWithWithdrawnChallengeIsNotProverJob(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	challenge := createProverTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusWithdrawn)

	nodes := []*node.Node{n}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 0 {
		t.Errorf("FindProverJobs() returned %d nodes, want 0 (challenge is withdrawn)", len(result))
	}
}

// TestFindProverJobs_NodeWithMixedChallengesIsProverJob tests that a node
// with mixed challenges (some open, some resolved) IS a prover job.
func TestFindProverJobs_NodeWithMixedChallengesIsProverJob(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	resolvedChallenge := createProverTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusResolved)
	openChallenge := createProverTestChallenge(t, "ch-2", nodeID, node.ChallengeStatusOpen)

	nodes := []*node.Node{n}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{resolvedChallenge, openChallenge})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 1 {
		t.Errorf("FindProverJobs() returned %d nodes, want 1 (node has open challenge)", len(result))
	}
}

// TestFindProverJobs_OnlyPendingNodesCanBeProverJobs tests that only pending nodes
// can be prover jobs.
func TestFindProverJobs_OnlyPendingNodesCanBeProverJobs(t *testing.T) {
	tests := []struct {
		name      string
		epistemic schema.EpistemicState
		wantJob   bool
	}{
		{"pending with open challenge is prover job", schema.EpistemicPending, true},
		{"validated is not prover job", schema.EpistemicValidated, false},
		{"admitted is not prover job", schema.EpistemicAdmitted, false},
		{"refuted is not prover job", schema.EpistemicRefuted, false},
		{"archived is not prover job", schema.EpistemicArchived, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, _ := types.Parse("1")
			n := createTestNode(t, "1", schema.WorkflowAvailable, tt.epistemic)
			challenge := createProverTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusOpen)

			nodes := []*node.Node{n}
			nodeMap := buildProverNodeMap(nodes)
			challengeMap := buildProverChallengeMap([]*node.Challenge{challenge})

			result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

			gotJob := len(result) > 0
			if gotJob != tt.wantJob {
				t.Errorf("FindProverJobs() with epistemic=%s returned job=%v, want job=%v",
					tt.epistemic, gotJob, tt.wantJob)
			}
		})
	}
}

// TestFindProverJobs_BlockedNodesCannotBeProverJobs tests that blocked nodes
// cannot be prover jobs even with open challenges.
func TestFindProverJobs_BlockedNodesCannotBeProverJobs(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createTestNode(t, "1", schema.WorkflowBlocked, schema.EpistemicPending)
	challenge := createProverTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusOpen)

	nodes := []*node.Node{n}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 0 {
		t.Errorf("FindProverJobs() returned %d nodes, want 0 (node is blocked)", len(result))
	}
}

// TestFindProverJobs_ClaimedNodesCanBeProverJobs tests that claimed nodes
// with open challenges can be prover jobs (prover is working on them).
func TestFindProverJobs_ClaimedNodesCanBeProverJobs(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createTestNode(t, "1", schema.WorkflowClaimed, schema.EpistemicPending)
	challenge := createProverTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusOpen)

	nodes := []*node.Node{n}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	// Claimed nodes WITH open challenges should still be shown as prover jobs
	// because they are actively being worked on by a prover
	if len(result) != 1 {
		t.Errorf("FindProverJobs() returned %d nodes, want 1 (claimed node with open challenge is prover job)", len(result))
	}
}

// TestFindProverJobs_AvailableNodesWithChallengesAreProverJobs tests that available
// nodes with open challenges are prover jobs.
func TestFindProverJobs_AvailableNodesWithChallengesAreProverJobs(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	challenge := createProverTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusOpen)

	nodes := []*node.Node{n}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 1 {
		t.Errorf("FindProverJobs() returned %d nodes, want 1", len(result))
	}
}

// TestFindProverJobs_MultipleOpenChallenges tests a node with multiple open challenges.
func TestFindProverJobs_MultipleOpenChallenges(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	challenge1 := createProverTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusOpen)
	challenge2 := createProverTestChallenge(t, "ch-2", nodeID, node.ChallengeStatusOpen)
	challenge3 := createProverTestChallenge(t, "ch-3", nodeID, node.ChallengeStatusOpen)

	nodes := []*node.Node{n}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{challenge1, challenge2, challenge3})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 1 {
		t.Errorf("FindProverJobs() returned %d nodes, want 1 (node has multiple open challenges)", len(result))
	}
}

// TestFindProverJobs_MultipleProverJobs tests finding multiple prover jobs.
func TestFindProverJobs_MultipleProverJobs(t *testing.T) {
	nodeID1, _ := types.Parse("1.1")
	nodeID2, _ := types.Parse("1.2")
	nodeID3, _ := types.Parse("1.3")

	node1 := createTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending)
	node2 := createTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending)
	node3 := createTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending)

	challenge1 := createProverTestChallenge(t, "ch-1", nodeID1, node.ChallengeStatusOpen)
	challenge2 := createProverTestChallenge(t, "ch-2", nodeID2, node.ChallengeStatusOpen)
	challenge3 := createProverTestChallenge(t, "ch-3", nodeID3, node.ChallengeStatusOpen)

	nodes := []*node.Node{node1, node2, node3}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{challenge1, challenge2, challenge3})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 3 {
		t.Errorf("FindProverJobs() returned %d nodes, want 3", len(result))
	}
}

// TestFindProverJobs_PreservesOrder tests that the order of returned nodes
// matches the input order.
func TestFindProverJobs_PreservesOrder(t *testing.T) {
	nodeID3, _ := types.Parse("1.3")
	nodeID1, _ := types.Parse("1.1")
	nodeID2, _ := types.Parse("1.2")

	node3 := createTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending)
	node1 := createTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending)
	node2 := createTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicPending)

	challenge3 := createProverTestChallenge(t, "ch-3", nodeID3, node.ChallengeStatusOpen)
	challenge1 := createProverTestChallenge(t, "ch-1", nodeID1, node.ChallengeStatusOpen)
	challenge2 := createProverTestChallenge(t, "ch-2", nodeID2, node.ChallengeStatusOpen)

	nodes := []*node.Node{node3, node1, node2}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{challenge3, challenge1, challenge2})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 3 {
		t.Errorf("FindProverJobs() returned %d nodes, want 3", len(result))
		return
	}

	expectedOrder := []string{"1.3", "1.1", "1.2"}
	for i, n := range result {
		if n.ID.String() != expectedOrder[i] {
			t.Errorf("FindProverJobs() result[%d] = %s, want %s", i, n.ID.String(), expectedOrder[i])
		}
	}
}

// TestFindProverJobs_ReturnsOriginalPointers tests that the returned slice
// contains pointers to the original nodes (not copies).
func TestFindProverJobs_ReturnsOriginalPointers(t *testing.T) {
	nodeID, _ := types.Parse("1")
	original := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	challenge := createProverTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusOpen)

	nodes := []*node.Node{original}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 1 {
		t.Fatalf("FindProverJobs() returned %d nodes, want 1", len(result))
	}

	if result[0] != original {
		t.Error("FindProverJobs() should return pointers to original nodes")
	}
}

// TestFindProverJobs_AllNodesExcluded tests the case where all input nodes
// are excluded (no open challenges).
func TestFindProverJobs_AllNodesExcluded(t *testing.T) {
	nodes := []*node.Node{
		createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),   // no challenges
		createTestNode(t, "1.1", schema.WorkflowBlocked, schema.EpistemicPending),   // blocked
		createTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated), // not pending
	}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := map[string][]*node.Challenge{} // No challenges

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	if len(result) != 0 {
		t.Errorf("FindProverJobs() returned %d nodes, want 0", len(result))
	}
}

// TestFindProverJobs_MixedStates tests FindProverJobs with a mix of different states.
func TestFindProverJobs_MixedStates(t *testing.T) {
	nodeID1, _ := types.Parse("1")
	nodeID3, _ := types.Parse("1.3")

	nodes := []*node.Node{
		createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending),     // open challenge -> prover
		createTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicPending),   // no challenges -> not prover
		createTestNode(t, "1.2", schema.WorkflowAvailable, schema.EpistemicValidated), // not pending -> not prover
		createTestNode(t, "1.3", schema.WorkflowAvailable, schema.EpistemicPending),   // open challenge -> prover
		createTestNode(t, "1.4", schema.WorkflowBlocked, schema.EpistemicPending),     // blocked -> not prover
	}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{
		createProverTestChallenge(t, "ch-1", nodeID1, node.ChallengeStatusOpen),
		createProverTestChallenge(t, "ch-3", nodeID3, node.ChallengeStatusOpen),
	})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	// Nodes 1 and 1.3 should be prover jobs
	expectedIDs := map[string]bool{"1": true, "1.3": true}
	if len(result) != len(expectedIDs) {
		t.Errorf("FindProverJobs() returned %d nodes, want %d", len(result), len(expectedIDs))
	}
	for _, n := range result {
		if !expectedIDs[n.ID.String()] {
			t.Errorf("FindProverJobs() returned unexpected node %s", n.ID.String())
		}
	}
}

// TestFindProverJobs_NilChallengeMapTreatedAsEmpty tests that nil challengeMap
// is treated as empty (no prover jobs).
func TestFindProverJobs_NilChallengeMapTreatedAsEmpty(t *testing.T) {
	n := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	nodes := []*node.Node{n}
	nodeMap := buildProverNodeMap(nodes)

	result := jobs.FindProverJobs(nodes, nodeMap, nil)

	// With nil challengeMap, no challenges exist, so no prover jobs
	if len(result) != 0 {
		t.Errorf("FindProverJobs() with nil challengeMap returned %d nodes, want 0", len(result))
	}
}

// TestFindProverJobs_ChildrenDontAffectProverStatus tests that children's states
// don't affect whether a node is a prover job (only challenges matter).
func TestFindProverJobs_ChildrenDontAffectProverStatus(t *testing.T) {
	nodeID, _ := types.Parse("1")
	parent := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicPending)
	child := createTestNode(t, "1.1", schema.WorkflowAvailable, schema.EpistemicValidated) // validated child
	challenge := createProverTestChallenge(t, "ch-1", nodeID, node.ChallengeStatusOpen)

	nodes := []*node.Node{parent, child}
	nodeMap := buildProverNodeMap(nodes)
	challengeMap := buildProverChallengeMap([]*node.Challenge{challenge})

	result := jobs.FindProverJobs(nodes, nodeMap, challengeMap)

	// Only parent should be prover job (has challenge), child has no challenge
	if len(result) != 1 {
		t.Errorf("FindProverJobs() returned %d nodes, want 1", len(result))
	}
	if len(result) > 0 && result[0].ID.String() != "1" {
		t.Errorf("FindProverJobs() returned %s, want 1", result[0].ID.String())
	}
}
