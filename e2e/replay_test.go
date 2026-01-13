//go:build integration

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// setupReplayTest creates a temporary directory and returns it along with a cleanup function.
func setupReplayTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-replay-*")
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }
	return tmpDir, cleanup
}

// createLedgerDir creates a ledger directory and returns a new Ledger instance.
func createLedgerDir(t *testing.T, tmpDir string) *ledger.Ledger {
	t.Helper()
	ledgerDir := filepath.Join(tmpDir, "proof", "ledger")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("failed to create ledger directory: %v", err)
	}
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}
	return ldg
}

// TestReplay_EmptyLedger tests that replay from empty ledger produces empty state.
func TestReplay_EmptyLedger(t *testing.T) {
	tmpDir, cleanup := setupReplayTest(t)
	defer cleanup()

	// Create empty ledger
	ldg := createLedgerDir(t, tmpDir)

	// Replay from empty ledger
	s, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("Replay from empty ledger should succeed: %v", err)
	}

	// Verify state is empty
	if s == nil {
		t.Fatal("Expected non-nil state from empty replay")
	}

	nodes := s.AllNodes()
	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes from empty ledger, got %d", len(nodes))
	}

	// LatestSeq should be 0 for empty ledger
	if s.LatestSeq() != 0 {
		t.Errorf("Expected LatestSeq=0 for empty ledger, got %d", s.LatestSeq())
	}
}

// TestReplay_CapturesNodeStates tests that replay captures all node states correctly.
func TestReplay_CapturesNodeStates(t *testing.T) {
	tmpDir, cleanup := setupReplayTest(t)
	defer cleanup()

	ldg := createLedgerDir(t, tmpDir)

	// Initialize proof
	initEvent := ledger.NewProofInitialized("Test conjecture", "test-author")
	if _, err := ldg.Append(initEvent); err != nil {
		t.Fatalf("failed to append proof initialized: %v", err)
	}

	// Create root node
	rootID, _ := types.Parse("1")
	rootNode, err := node.NewNode(rootID, schema.NodeTypeClaim, "Root statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create root node: %v", err)
	}
	if _, err := ldg.Append(ledger.NewNodeCreated(*rootNode)); err != nil {
		t.Fatalf("failed to append root node created: %v", err)
	}

	// Create child node
	childID, _ := types.Parse("1.1")
	childNode, err := node.NewNode(childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create child node: %v", err)
	}
	if _, err := ldg.Append(ledger.NewNodeCreated(*childNode)); err != nil {
		t.Fatalf("failed to append child node created: %v", err)
	}

	// Replay and verify
	s, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify both nodes exist
	nodes := s.AllNodes()
	if len(nodes) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(nodes))
	}

	// Verify root node
	replayedRoot := s.GetNode(rootID)
	if replayedRoot == nil {
		t.Fatal("Root node not found in replayed state")
	}
	if replayedRoot.Statement != "Root statement" {
		t.Errorf("Root statement = %q, want %q", replayedRoot.Statement, "Root statement")
	}
	if replayedRoot.Type != schema.NodeTypeClaim {
		t.Errorf("Root type = %q, want %q", replayedRoot.Type, schema.NodeTypeClaim)
	}
	if replayedRoot.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("Root workflow state = %q, want %q", replayedRoot.WorkflowState, schema.WorkflowAvailable)
	}
	if replayedRoot.EpistemicState != schema.EpistemicPending {
		t.Errorf("Root epistemic state = %q, want %q", replayedRoot.EpistemicState, schema.EpistemicPending)
	}

	// Verify child node
	replayedChild := s.GetNode(childID)
	if replayedChild == nil {
		t.Fatal("Child node not found in replayed state")
	}
	if replayedChild.Statement != "Child statement" {
		t.Errorf("Child statement = %q, want %q", replayedChild.Statement, "Child statement")
	}
	if replayedChild.Inference != schema.InferenceModusPonens {
		t.Errorf("Child inference = %q, want %q", replayedChild.Inference, schema.InferenceModusPonens)
	}
}

// TestReplay_IdempotentMultipleReplays tests that replaying same ledger multiple times produces identical state.
func TestReplay_IdempotentMultipleReplays(t *testing.T) {
	tmpDir, cleanup := setupReplayTest(t)
	defer cleanup()

	ldg := createLedgerDir(t, tmpDir)

	// Initialize proof with multiple nodes
	initEvent := ledger.NewProofInitialized("Test conjecture", "test-author")
	if _, err := ldg.Append(initEvent); err != nil {
		t.Fatalf("failed to append proof initialized: %v", err)
	}

	rootID, _ := types.Parse("1")
	rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root statement", schema.InferenceAssumption)
	if _, err := ldg.Append(ledger.NewNodeCreated(*rootNode)); err != nil {
		t.Fatalf("failed to append root node: %v", err)
	}

	childID, _ := types.Parse("1.1")
	childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
	if _, err := ldg.Append(ledger.NewNodeCreated(*childNode)); err != nil {
		t.Fatalf("failed to append child node: %v", err)
	}

	// Claim and release nodes to add more events
	timeout := types.Now()
	if _, err := ldg.Append(ledger.NewNodesClaimed([]types.NodeID{rootID}, "agent-1", timeout)); err != nil {
		t.Fatalf("failed to append nodes claimed: %v", err)
	}
	if _, err := ldg.Append(ledger.NewNodesReleased([]types.NodeID{rootID})); err != nil {
		t.Fatalf("failed to append nodes released: %v", err)
	}

	// Replay multiple times
	state1, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("First replay failed: %v", err)
	}

	state2, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("Second replay failed: %v", err)
	}

	state3, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("Third replay failed: %v", err)
	}

	// Verify all replays produce identical state
	nodes1 := state1.AllNodes()
	nodes2 := state2.AllNodes()
	nodes3 := state3.AllNodes()

	if len(nodes1) != len(nodes2) || len(nodes2) != len(nodes3) {
		t.Fatalf("Node counts differ: %d, %d, %d", len(nodes1), len(nodes2), len(nodes3))
	}

	// Verify LatestSeq is identical
	if state1.LatestSeq() != state2.LatestSeq() || state2.LatestSeq() != state3.LatestSeq() {
		t.Fatalf("LatestSeq differs: %d, %d, %d", state1.LatestSeq(), state2.LatestSeq(), state3.LatestSeq())
	}

	// Verify node states are identical
	for _, n1 := range nodes1 {
		n2 := state2.GetNode(n1.ID)
		n3 := state3.GetNode(n1.ID)

		if n2 == nil || n3 == nil {
			t.Fatalf("Node %s missing in subsequent replays", n1.ID.String())
		}

		// Compare key fields
		if n1.Statement != n2.Statement || n2.Statement != n3.Statement {
			t.Errorf("Statement differs for node %s", n1.ID.String())
		}
		if n1.WorkflowState != n2.WorkflowState || n2.WorkflowState != n3.WorkflowState {
			t.Errorf("WorkflowState differs for node %s", n1.ID.String())
		}
		if n1.EpistemicState != n2.EpistemicState || n2.EpistemicState != n3.EpistemicState {
			t.Errorf("EpistemicState differs for node %s", n1.ID.String())
		}
		if n1.TaintState != n2.TaintState || n2.TaintState != n3.TaintState {
			t.Errorf("TaintState differs for node %s", n1.ID.String())
		}
	}
}

// TestReplay_StateAfterOperations tests that state derived from replay matches expected state after various operations.
func TestReplay_StateAfterOperations(t *testing.T) {
	tmpDir, cleanup := setupReplayTest(t)
	defer cleanup()

	ldg := createLedgerDir(t, tmpDir)

	// Initialize proof
	initEvent := ledger.NewProofInitialized("Test conjecture", "test-author")
	if _, err := ldg.Append(initEvent); err != nil {
		t.Fatalf("failed to append proof initialized: %v", err)
	}

	// Create a node
	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if _, err := ldg.Append(ledger.NewNodeCreated(*n)); err != nil {
		t.Fatalf("failed to append node created: %v", err)
	}

	// Operation 1: Claim the node
	timeout := types.Now()
	if _, err := ldg.Append(ledger.NewNodesClaimed([]types.NodeID{nodeID}, "agent-1", timeout)); err != nil {
		t.Fatalf("failed to append nodes claimed: %v", err)
	}

	// Replay and verify claimed state
	s, _ := state.Replay(ldg)
	claimedNode := s.GetNode(nodeID)
	if claimedNode.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("After claim: WorkflowState = %q, want %q", claimedNode.WorkflowState, schema.WorkflowClaimed)
	}
	if claimedNode.ClaimedBy != "agent-1" {
		t.Errorf("After claim: ClaimedBy = %q, want %q", claimedNode.ClaimedBy, "agent-1")
	}

	// Operation 2: Release the node
	if _, err := ldg.Append(ledger.NewNodesReleased([]types.NodeID{nodeID})); err != nil {
		t.Fatalf("failed to append nodes released: %v", err)
	}

	// Replay and verify released state
	s, _ = state.Replay(ldg)
	releasedNode := s.GetNode(nodeID)
	if releasedNode.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("After release: WorkflowState = %q, want %q", releasedNode.WorkflowState, schema.WorkflowAvailable)
	}
	if releasedNode.ClaimedBy != "" {
		t.Errorf("After release: ClaimedBy = %q, want empty", releasedNode.ClaimedBy)
	}

	// Operation 3: Validate the node
	if _, err := ldg.Append(ledger.NewNodeValidated(nodeID)); err != nil {
		t.Fatalf("failed to append node validated: %v", err)
	}

	// Replay and verify validated state
	s, _ = state.Replay(ldg)
	validatedNode := s.GetNode(nodeID)
	if validatedNode.EpistemicState != schema.EpistemicValidated {
		t.Errorf("After validate: EpistemicState = %q, want %q", validatedNode.EpistemicState, schema.EpistemicValidated)
	}
}

// TestReplay_AllEventTypes tests that replay handles all event types correctly.
func TestReplay_AllEventTypes(t *testing.T) {
	tmpDir, cleanup := setupReplayTest(t)
	defer cleanup()

	ldg := createLedgerDir(t, tmpDir)

	// Event 1: ProofInitialized
	initEvent := ledger.NewProofInitialized("Test conjecture", "test-author")
	seq, err := ldg.Append(initEvent)
	if err != nil {
		t.Fatalf("failed to append ProofInitialized: %v", err)
	}
	t.Logf("ProofInitialized: seq=%d", seq)

	// Event 2: NodeCreated
	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	seq, err = ldg.Append(ledger.NewNodeCreated(*n))
	if err != nil {
		t.Fatalf("failed to append NodeCreated: %v", err)
	}
	t.Logf("NodeCreated: seq=%d", seq)

	// Event 3: NodesClaimed
	timeout := types.Now()
	seq, err = ldg.Append(ledger.NewNodesClaimed([]types.NodeID{nodeID}, "agent-1", timeout))
	if err != nil {
		t.Fatalf("failed to append NodesClaimed: %v", err)
	}
	t.Logf("NodesClaimed: seq=%d", seq)

	// Event 4: NodesReleased
	seq, err = ldg.Append(ledger.NewNodesReleased([]types.NodeID{nodeID}))
	if err != nil {
		t.Fatalf("failed to append NodesReleased: %v", err)
	}
	t.Logf("NodesReleased: seq=%d", seq)

	// Event 5: ChallengeRaised
	seq, err = ldg.Append(ledger.NewChallengeRaised("challenge-001", nodeID, "statement", "Needs more justification"))
	if err != nil {
		t.Fatalf("failed to append ChallengeRaised: %v", err)
	}
	t.Logf("ChallengeRaised: seq=%d", seq)

	// Event 6: ChallengeResolved
	seq, err = ldg.Append(ledger.NewChallengeResolved("challenge-001"))
	if err != nil {
		t.Fatalf("failed to append ChallengeResolved: %v", err)
	}
	t.Logf("ChallengeResolved: seq=%d", seq)

	// Event 7: ChallengeWithdrawn (for a new challenge)
	seq, err = ldg.Append(ledger.NewChallengeRaised("challenge-002", nodeID, "inference", "Unclear"))
	if err != nil {
		t.Fatalf("failed to append second ChallengeRaised: %v", err)
	}
	seq, err = ldg.Append(ledger.NewChallengeWithdrawn("challenge-002"))
	if err != nil {
		t.Fatalf("failed to append ChallengeWithdrawn: %v", err)
	}
	t.Logf("ChallengeWithdrawn: seq=%d", seq)

	// Event 8: NodeValidated
	seq, err = ldg.Append(ledger.NewNodeValidated(nodeID))
	if err != nil {
		t.Fatalf("failed to append NodeValidated: %v", err)
	}
	t.Logf("NodeValidated: seq=%d", seq)

	// Create a second node for other epistemic states
	node2ID, _ := types.Parse("1.1")
	n2, _ := node.NewNode(node2ID, schema.NodeTypeClaim, "Second statement", schema.InferenceModusPonens)
	seq, err = ldg.Append(ledger.NewNodeCreated(*n2))
	if err != nil {
		t.Fatalf("failed to append second NodeCreated: %v", err)
	}

	// Event 9: NodeAdmitted
	seq, err = ldg.Append(ledger.NewNodeAdmitted(node2ID))
	if err != nil {
		t.Fatalf("failed to append NodeAdmitted: %v", err)
	}
	t.Logf("NodeAdmitted: seq=%d", seq)

	// Create third node for refuted state
	node3ID, _ := types.Parse("1.2")
	n3, _ := node.NewNode(node3ID, schema.NodeTypeClaim, "Third statement", schema.InferenceModusTollens)
	seq, err = ldg.Append(ledger.NewNodeCreated(*n3))
	if err != nil {
		t.Fatalf("failed to append third NodeCreated: %v", err)
	}

	// Event 10: NodeRefuted
	seq, err = ldg.Append(ledger.NewNodeRefuted(node3ID))
	if err != nil {
		t.Fatalf("failed to append NodeRefuted: %v", err)
	}
	t.Logf("NodeRefuted: seq=%d", seq)

	// Create fourth node for archived state
	node4ID, _ := types.Parse("1.3")
	n4, _ := node.NewNode(node4ID, schema.NodeTypeClaim, "Fourth statement", schema.InferenceByDefinition)
	seq, err = ldg.Append(ledger.NewNodeCreated(*n4))
	if err != nil {
		t.Fatalf("failed to append fourth NodeCreated: %v", err)
	}

	// Event 11: NodeArchived
	seq, err = ldg.Append(ledger.NewNodeArchived(node4ID))
	if err != nil {
		t.Fatalf("failed to append NodeArchived: %v", err)
	}
	t.Logf("NodeArchived: seq=%d", seq)

	// Event 12: TaintRecomputed
	seq, err = ldg.Append(ledger.NewTaintRecomputed(nodeID, node.TaintClean))
	if err != nil {
		t.Fatalf("failed to append TaintRecomputed: %v", err)
	}
	t.Logf("TaintRecomputed: seq=%d", seq)

	// Event 13: DefAdded
	def := ledger.Definition{
		ID:         "def-001",
		Name:       "prime",
		Definition: "A positive integer with exactly two divisors",
		Created:    types.Now(),
	}
	seq, err = ldg.Append(ledger.NewDefAdded(def))
	if err != nil {
		t.Fatalf("failed to append DefAdded: %v", err)
	}
	t.Logf("DefAdded: seq=%d", seq)

	// Event 14: LemmaExtracted
	lemma := ledger.Lemma{
		ID:        "lemma-001",
		Statement: "All primes greater than 2 are odd",
		NodeID:    nodeID,
		Created:   types.Now(),
	}
	seq, err = ldg.Append(ledger.NewLemmaExtracted(lemma))
	if err != nil {
		t.Fatalf("failed to append LemmaExtracted: %v", err)
	}
	t.Logf("LemmaExtracted: seq=%d", seq)

	// Replay and verify all events were processed
	s, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify node states
	replayedNode1 := s.GetNode(nodeID)
	if replayedNode1 == nil {
		t.Fatal("Node 1 not found in replayed state")
	}
	if replayedNode1.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Node 1: EpistemicState = %q, want %q", replayedNode1.EpistemicState, schema.EpistemicValidated)
	}
	if replayedNode1.TaintState != node.TaintClean {
		t.Errorf("Node 1: TaintState = %q, want %q", replayedNode1.TaintState, node.TaintClean)
	}

	replayedNode2 := s.GetNode(node2ID)
	if replayedNode2 == nil {
		t.Fatal("Node 2 not found in replayed state")
	}
	if replayedNode2.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("Node 2: EpistemicState = %q, want %q", replayedNode2.EpistemicState, schema.EpistemicAdmitted)
	}

	replayedNode3 := s.GetNode(node3ID)
	if replayedNode3 == nil {
		t.Fatal("Node 3 not found in replayed state")
	}
	if replayedNode3.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("Node 3: EpistemicState = %q, want %q", replayedNode3.EpistemicState, schema.EpistemicRefuted)
	}

	replayedNode4 := s.GetNode(node4ID)
	if replayedNode4 == nil {
		t.Fatal("Node 4 not found in replayed state")
	}
	if replayedNode4.EpistemicState != schema.EpistemicArchived {
		t.Errorf("Node 4: EpistemicState = %q, want %q", replayedNode4.EpistemicState, schema.EpistemicArchived)
	}

	// Verify definition was added
	replayedDef := s.GetDefinition("def-001")
	if replayedDef == nil {
		t.Fatal("Definition not found in replayed state")
	}
	if replayedDef.Name != "prime" {
		t.Errorf("Definition name = %q, want %q", replayedDef.Name, "prime")
	}
	if replayedDef.Content != "A positive integer with exactly two divisors" {
		t.Errorf("Definition content = %q, want %q", replayedDef.Content, "A positive integer with exactly two divisors")
	}

	// Verify lemma was extracted
	replayedLemma := s.GetLemma("lemma-001")
	if replayedLemma == nil {
		t.Fatal("Lemma not found in replayed state")
	}
	if replayedLemma.Statement != "All primes greater than 2 are odd" {
		t.Errorf("Lemma statement = %q, want %q", replayedLemma.Statement, "All primes greater than 2 are odd")
	}
	if replayedLemma.SourceNodeID.String() != nodeID.String() {
		t.Errorf("Lemma source node = %q, want %q", replayedLemma.SourceNodeID.String(), nodeID.String())
	}

	// Verify LatestSeq matches the number of events
	count, _ := ldg.Count()
	if s.LatestSeq() != count {
		t.Errorf("LatestSeq = %d, want %d (event count)", s.LatestSeq(), count)
	}
}

// TestReplay_SequenceNumberTracking tests that replay correctly tracks sequence numbers.
func TestReplay_SequenceNumberTracking(t *testing.T) {
	tmpDir, cleanup := setupReplayTest(t)
	defer cleanup()

	ldg := createLedgerDir(t, tmpDir)

	// Append events and track sequence numbers
	var expectedSeq int

	seq, _ := ldg.Append(ledger.NewProofInitialized("Test", "author"))
	expectedSeq = seq

	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Statement", schema.InferenceAssumption)
	seq, _ = ldg.Append(ledger.NewNodeCreated(*n))
	expectedSeq = seq

	// Replay and verify LatestSeq
	s, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if s.LatestSeq() != expectedSeq {
		t.Errorf("LatestSeq = %d, want %d", s.LatestSeq(), expectedSeq)
	}

	// Add more events
	timeout := types.Now()
	seq, _ = ldg.Append(ledger.NewNodesClaimed([]types.NodeID{nodeID}, "agent", timeout))
	expectedSeq = seq

	// Replay again
	s, err = state.Replay(ldg)
	if err != nil {
		t.Fatalf("Second replay failed: %v", err)
	}

	if s.LatestSeq() != expectedSeq {
		t.Errorf("LatestSeq after more events = %d, want %d", s.LatestSeq(), expectedSeq)
	}
}

// TestReplay_WithVerifyContentHash tests replay with content hash verification.
func TestReplay_WithVerifyContentHash(t *testing.T) {
	tmpDir, cleanup := setupReplayTest(t)
	defer cleanup()

	ldg := createLedgerDir(t, tmpDir)

	// Initialize proof
	if _, err := ldg.Append(ledger.NewProofInitialized("Test", "author")); err != nil {
		t.Fatalf("failed to append proof initialized: %v", err)
	}

	// Create node with valid content hash
	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if _, err := ldg.Append(ledger.NewNodeCreated(*n)); err != nil {
		t.Fatalf("failed to append node created: %v", err)
	}

	// ReplayWithVerify should succeed for valid content hashes
	s, err := state.ReplayWithVerify(ldg)
	if err != nil {
		t.Fatalf("ReplayWithVerify failed: %v", err)
	}

	// Verify node was replayed correctly
	replayedNode := s.GetNode(nodeID)
	if replayedNode == nil {
		t.Fatal("Node not found in replayed state")
	}

	// Verify content hash matches
	if !replayedNode.VerifyContentHash() {
		t.Error("Content hash verification failed for replayed node")
	}
}

// TestReplay_MultipleNodesWithDependencies tests replay with nodes that have dependencies.
func TestReplay_MultipleNodesWithDependencies(t *testing.T) {
	tmpDir, cleanup := setupReplayTest(t)
	defer cleanup()

	ldg := createLedgerDir(t, tmpDir)

	// Initialize proof
	if _, err := ldg.Append(ledger.NewProofInitialized("Test with dependencies", "author")); err != nil {
		t.Fatalf("failed to append proof initialized: %v", err)
	}

	// Create root node (no dependencies)
	rootID, _ := types.Parse("1")
	rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Premise A", schema.InferenceAssumption)
	if _, err := ldg.Append(ledger.NewNodeCreated(*rootNode)); err != nil {
		t.Fatalf("failed to append root node: %v", err)
	}

	// Create first child node - "Premise B implies C" (no dependencies)
	child1ID, _ := types.Parse("1.1")
	child1Node, _ := node.NewNode(child1ID, schema.NodeTypeClaim, "Premise B implies C", schema.InferenceAssumption)
	if _, err := ldg.Append(ledger.NewNodeCreated(*child1Node)); err != nil {
		t.Fatalf("failed to append first child node: %v", err)
	}

	// Create second child node with dependencies on root and first child
	child2ID, _ := types.Parse("1.2")
	child2Node, _ := node.NewNodeWithOptions(
		child2ID,
		schema.NodeTypeClaim,
		"Therefore C",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Dependencies: []types.NodeID{rootID, child1ID},
		},
	)
	if _, err := ldg.Append(ledger.NewNodeCreated(*child2Node)); err != nil {
		t.Fatalf("failed to append second child node: %v", err)
	}

	// Replay and verify dependencies are preserved
	s, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify all nodes exist
	if len(s.AllNodes()) != 3 {
		t.Fatalf("Expected 3 nodes, got %d", len(s.AllNodes()))
	}

	// Verify second child node dependencies
	replayedChild2 := s.GetNode(child2ID)
	if replayedChild2 == nil {
		t.Fatal("Second child node not found")
	}
	if len(replayedChild2.Dependencies) != 2 {
		t.Errorf("Child dependencies count = %d, want 2", len(replayedChild2.Dependencies))
	}

	// Verify dependency IDs
	depStrings := make(map[string]bool)
	for _, dep := range replayedChild2.Dependencies {
		depStrings[dep.String()] = true
	}
	if !depStrings["1"] {
		t.Error("Child missing dependency on node 1")
	}
	if !depStrings["1.1"] {
		t.Error("Child missing dependency on node 1.1")
	}
}

// TestReplay_NilLedger tests that replay from nil ledger returns error.
func TestReplay_NilLedger(t *testing.T) {
	_, err := state.Replay(nil)
	if err == nil {
		t.Error("Replay(nil) should return error")
	}
}

// TestReplay_HierarchicalNodes tests replay with deeply nested hierarchical nodes.
func TestReplay_HierarchicalNodes(t *testing.T) {
	tmpDir, cleanup := setupReplayTest(t)
	defer cleanup()

	ldg := createLedgerDir(t, tmpDir)

	// Initialize proof
	if _, err := ldg.Append(ledger.NewProofInitialized("Hierarchical test", "author")); err != nil {
		t.Fatalf("failed to append proof initialized: %v", err)
	}

	// Create a chain of hierarchical nodes: 1 -> 1.1 -> 1.1.1 -> 1.1.1.1
	nodeIDs := []string{"1", "1.1", "1.1.1", "1.1.1.1"}
	for _, idStr := range nodeIDs {
		id, _ := types.Parse(idStr)
		n, _ := node.NewNode(id, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceModusPonens)
		if _, err := ldg.Append(ledger.NewNodeCreated(*n)); err != nil {
			t.Fatalf("failed to append node %s: %v", idStr, err)
		}
	}

	// Replay and verify
	s, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify all nodes exist with correct hierarchy
	for _, idStr := range nodeIDs {
		id, _ := types.Parse(idStr)
		n := s.GetNode(id)
		if n == nil {
			t.Errorf("Node %s not found in replayed state", idStr)
			continue
		}

		// Verify depth matches hierarchy
		expectedDepth := len(idStr)/2 + 1 // "1"=1, "1.1"=2, "1.1.1"=3, "1.1.1.1"=4
		if n.Depth() != expectedDepth {
			t.Errorf("Node %s: depth = %d, want %d", idStr, n.Depth(), expectedDepth)
		}

		// Verify statement
		expectedStatement := "Statement for " + idStr
		if n.Statement != expectedStatement {
			t.Errorf("Node %s: statement = %q, want %q", idStr, n.Statement, expectedStatement)
		}
	}
}
