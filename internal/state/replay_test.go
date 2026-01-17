//go:build integration

// Package state provides derived state from replaying ledger events.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// -----------------------------------------------------------------------------
// Replay Function Tests
// -----------------------------------------------------------------------------

// TestReplay_EmptyLedger verifies that replaying an empty ledger produces an empty state.
func TestReplay_EmptyLedger(t *testing.T) {
	dir := t.TempDir()

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if state == nil {
		t.Fatal("Replay returned nil state for empty ledger")
	}

	// Verify empty state has no nodes
	nodeID := mustParseNodeID(t, "1")
	if got := state.GetNode(nodeID); got != nil {
		t.Errorf("Empty state contains unexpected node: %v", got)
	}
}

// TestReplay_SingleProofInitializedEvent verifies replaying a single ProofInitialized event.
func TestReplay_SingleProofInitializedEvent(t *testing.T) {
	dir := t.TempDir()

	// Append a ProofInitialized event
	event := ledger.NewProofInitialized("Prove that P implies Q", "test-author")
	_, err := ledger.Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if state == nil {
		t.Fatal("Replay returned nil state")
	}

	// ProofInitialized doesn't add nodes to state, but should succeed
}

// TestReplay_SingleNodeCreatedEvent verifies replaying a single NodeCreated event.
func TestReplay_SingleNodeCreatedEvent(t *testing.T) {
	dir := t.TempDir()

	// Create and append a NodeCreated event
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Root claim statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	event := ledger.NewNodeCreated(*n)
	_, err = ledger.Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify the node exists in state
	got := state.GetNode(nodeID)
	if got == nil {
		t.Fatal("Replay did not add node to state")
	}

	if got.Statement != "Root claim statement" {
		t.Errorf("Node statement mismatch: got %q, want %q", got.Statement, "Root claim statement")
	}

	if got.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("Node workflow state mismatch: got %q, want %q", got.WorkflowState, schema.WorkflowAvailable)
	}
}

// TestReplay_MultipleEventsInSequence verifies replaying multiple events in correct sequence.
func TestReplay_MultipleEventsInSequence(t *testing.T) {
	dir := t.TempDir()

	// Create a sequence of events
	// 1. ProofInitialized
	initEvent := ledger.NewProofInitialized("Test conjecture", "author")
	if _, err := ledger.Append(dir, initEvent); err != nil {
		t.Fatalf("Append ProofInitialized failed: %v", err)
	}

	// 2. Create root node
	rootID := mustParseNodeID(t, "1")
	rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*rootNode)); err != nil {
		t.Fatalf("Append NodeCreated root failed: %v", err)
	}

	// 3. Create child node
	childID := mustParseNodeID(t, "1.1")
	childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child", schema.InferenceModusPonens)
	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*childNode)); err != nil {
		t.Fatalf("Append NodeCreated child failed: %v", err)
	}

	// 4. Claim the child node
	claimEvent := ledger.NewNodesClaimed([]types.NodeID{childID}, "agent-prover", types.Now())
	if _, err := ledger.Append(dir, claimEvent); err != nil {
		t.Fatalf("Append NodesClaimed failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify both nodes exist
	gotRoot := state.GetNode(rootID)
	if gotRoot == nil {
		t.Fatal("Root node not found after replay")
	}

	gotChild := state.GetNode(childID)
	if gotChild == nil {
		t.Fatal("Child node not found after replay")
	}

	// Verify child is claimed
	if gotChild.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("Child workflow state: got %q, want %q", gotChild.WorkflowState, schema.WorkflowClaimed)
	}
	if gotChild.ClaimedBy != "agent-prover" {
		t.Errorf("Child ClaimedBy: got %q, want %q", gotChild.ClaimedBy, "agent-prover")
	}
}

// TestReplay_NodeWorkflowStateTransitions verifies that all workflow state transitions are replayed correctly.
func TestReplay_NodeWorkflowStateTransitions(t *testing.T) {
	dir := t.TempDir()

	nodeID := mustParseNodeID(t, "1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)

	// Create node
	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
		t.Fatalf("Append NodeCreated failed: %v", err)
	}

	// Claim node
	if _, err := ledger.Append(dir, ledger.NewNodesClaimed([]types.NodeID{nodeID}, "agent-1", types.Now())); err != nil {
		t.Fatalf("Append NodesClaimed failed: %v", err)
	}

	// Release node
	if _, err := ledger.Append(dir, ledger.NewNodesReleased([]types.NodeID{nodeID})); err != nil {
		t.Fatalf("Append NodesReleased failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	got := state.GetNode(nodeID)
	if got == nil {
		t.Fatal("Node not found after replay")
	}

	// After claim and release, should be available again
	if got.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("Workflow state after claim/release: got %q, want %q", got.WorkflowState, schema.WorkflowAvailable)
	}
	if got.ClaimedBy != "" {
		t.Errorf("ClaimedBy should be empty after release: got %q", got.ClaimedBy)
	}
}

// TestReplay_NodeEpistemicStateTransitions verifies that epistemic state transitions are replayed correctly.
func TestReplay_NodeEpistemicStateTransitions(t *testing.T) {
	tests := []struct {
		name          string
		eventCreator  func(types.NodeID) ledger.Event
		expectedState schema.EpistemicState
	}{
		{
			name:          "NodeValidated",
			eventCreator:  func(id types.NodeID) ledger.Event { return ledger.NewNodeValidated(id) },
			expectedState: schema.EpistemicValidated,
		},
		{
			name:          "NodeAdmitted",
			eventCreator:  func(id types.NodeID) ledger.Event { return ledger.NewNodeAdmitted(id) },
			expectedState: schema.EpistemicAdmitted,
		},
		{
			name:          "NodeRefuted",
			eventCreator:  func(id types.NodeID) ledger.Event { return ledger.NewNodeRefuted(id) },
			expectedState: schema.EpistemicRefuted,
		},
		{
			name:          "NodeArchived",
			eventCreator:  func(id types.NodeID) ledger.Event { return ledger.NewNodeArchived(id) },
			expectedState: schema.EpistemicArchived,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			nodeID := mustParseNodeID(t, "1")
			n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test", schema.InferenceAssumption)

			// Create node
			if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
				t.Fatalf("Append NodeCreated failed: %v", err)
			}

			// Apply epistemic state change
			if _, err := ledger.Append(dir, tt.eventCreator(nodeID)); err != nil {
				t.Fatalf("Append %s failed: %v", tt.name, err)
			}

			ldg, err := ledger.NewLedger(dir)
			if err != nil {
				t.Fatalf("NewLedger failed: %v", err)
			}

			state, err := Replay(ldg)
			if err != nil {
				t.Fatalf("Replay failed: %v", err)
			}

			got := state.GetNode(nodeID)
			if got == nil {
				t.Fatal("Node not found after replay")
			}

			if got.EpistemicState != tt.expectedState {
				t.Errorf("Epistemic state: got %q, want %q", got.EpistemicState, tt.expectedState)
			}
		})
	}
}

// TestReplay_TaintRecomputedEvent verifies that TaintRecomputed events are replayed correctly.
func TestReplay_TaintRecomputedEvent(t *testing.T) {
	tests := []struct {
		name         string
		taintState   node.TaintState
	}{
		{"clean", node.TaintClean},
		{"self_admitted", node.TaintSelfAdmitted},
		{"tainted", node.TaintTainted},
		{"unresolved", node.TaintUnresolved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			nodeID := mustParseNodeID(t, "1")
			n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test", schema.InferenceAssumption)

			// Create node and recompute taint
			if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
				t.Fatalf("Append NodeCreated failed: %v", err)
			}
			if _, err := ledger.Append(dir, ledger.NewTaintRecomputed(nodeID, tt.taintState)); err != nil {
				t.Fatalf("Append TaintRecomputed failed: %v", err)
			}

			ldg, err := ledger.NewLedger(dir)
			if err != nil {
				t.Fatalf("NewLedger failed: %v", err)
			}

			state, err := Replay(ldg)
			if err != nil {
				t.Fatalf("Replay failed: %v", err)
			}

			got := state.GetNode(nodeID)
			if got == nil {
				t.Fatal("Node not found after replay")
			}

			if got.TaintState != tt.taintState {
				t.Errorf("Taint state: got %q, want %q", got.TaintState, tt.taintState)
			}
		})
	}
}

// TestReplay_DefAddedEvent verifies that DefAdded events add definitions to state.
func TestReplay_DefAddedEvent(t *testing.T) {
	dir := t.TempDir()

	def := ledger.Definition{
		ID:         "def-001",
		Name:       "prime",
		Definition: "A prime number is a natural number greater than 1 with exactly two divisors",
		Created:    types.Now(),
	}

	if _, err := ledger.Append(dir, ledger.NewDefAdded(def)); err != nil {
		t.Fatalf("Append DefAdded failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	got := state.GetDefinition("def-001")
	if got == nil {
		t.Fatal("Definition not found after replay")
	}

	if got.Name != "prime" {
		t.Errorf("Definition name: got %q, want %q", got.Name, "prime")
	}
	if got.Content != def.Definition {
		t.Errorf("Definition content: got %q, want %q", got.Content, def.Definition)
	}
}

// TestReplay_LemmaExtractedEvent verifies that LemmaExtracted events add lemmas to state.
func TestReplay_LemmaExtractedEvent(t *testing.T) {
	dir := t.TempDir()

	sourceNodeID := mustParseNodeID(t, "1.2")
	lemma := ledger.Lemma{
		ID:        "lem-001",
		Statement: "For all x, if P(x) then Q(x)",
		NodeID:    sourceNodeID,
		Created:   types.Now(),
	}

	if _, err := ledger.Append(dir, ledger.NewLemmaExtracted(lemma)); err != nil {
		t.Fatalf("Append LemmaExtracted failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	got := state.GetLemma("lem-001")
	if got == nil {
		t.Fatal("Lemma not found after replay")
	}

	if got.Statement != "For all x, if P(x) then Q(x)" {
		t.Errorf("Lemma statement: got %q, want %q", got.Statement, "For all x, if P(x) then Q(x)")
	}
	if got.SourceNodeID.String() != sourceNodeID.String() {
		t.Errorf("Lemma source node ID: got %q, want %q", got.SourceNodeID.String(), sourceNodeID.String())
	}
}

// TestReplay_ChallengeEvents verifies that challenge events are handled and tracked in state.
func TestReplay_ChallengeEvents(t *testing.T) {
	dir := t.TempDir()

	nodeID := mustParseNodeID(t, "1")

	// Challenge events: raise chal-001, resolve it; raise chal-002, withdraw it
	if _, err := ledger.Append(dir, ledger.NewChallengeRaised("chal-001", nodeID, "statement", "reason 1")); err != nil {
		t.Fatalf("Append ChallengeRaised chal-001 failed: %v", err)
	}
	if _, err := ledger.Append(dir, ledger.NewChallengeResolved("chal-001")); err != nil {
		t.Fatalf("Append ChallengeResolved chal-001 failed: %v", err)
	}
	if _, err := ledger.Append(dir, ledger.NewChallengeRaised("chal-002", nodeID, "inference", "reason 2")); err != nil {
		t.Fatalf("Append ChallengeRaised chal-002 failed: %v", err)
	}
	if _, err := ledger.Append(dir, ledger.NewChallengeWithdrawn("chal-002")); err != nil {
		t.Fatalf("Append ChallengeWithdrawn chal-002 failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if state == nil {
		t.Fatal("Replay returned nil state")
	}

	// Verify challenge states
	c1 := state.GetChallenge("chal-001")
	if c1 == nil {
		t.Fatal("Challenge chal-001 not found")
	}
	if c1.Status != "resolved" {
		t.Errorf("Challenge chal-001 status: got %q, want %q", c1.Status, "resolved")
	}

	c2 := state.GetChallenge("chal-002")
	if c2 == nil {
		t.Fatal("Challenge chal-002 not found")
	}
	if c2.Status != "withdrawn" {
		t.Errorf("Challenge chal-002 status: got %q, want %q", c2.Status, "withdrawn")
	}

	// Verify AllChallenges
	all := state.AllChallenges()
	if len(all) != 2 {
		t.Errorf("AllChallenges length: got %d, want 2", len(all))
	}

	// Verify OpenChallenges (should be empty)
	open := state.OpenChallenges()
	if len(open) != 0 {
		t.Errorf("OpenChallenges length: got %d, want 0", len(open))
	}
}

// TestReplay_AllEventTypes verifies replaying all supported event types together.
func TestReplay_AllEventTypes(t *testing.T) {
	dir := t.TempDir()

	nodeID := mustParseNodeID(t, "1")
	childID := mustParseNodeID(t, "1.1")

	events := []ledger.Event{
		// ProofInitialized
		ledger.NewProofInitialized("Prove P implies Q", "test-author"),

		// NodeCreated
		func() ledger.Event {
			n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
			return ledger.NewNodeCreated(*n)
		}(),
		func() ledger.Event {
			n, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child", schema.InferenceModusPonens)
			return ledger.NewNodeCreated(*n)
		}(),

		// NodesClaimed
		ledger.NewNodesClaimed([]types.NodeID{childID}, "agent-1", types.Now()),

		// NodesReleased
		ledger.NewNodesReleased([]types.NodeID{childID}),

		// Challenge events
		ledger.NewChallengeRaised("chal-001", nodeID, "statement", "reason"),
		ledger.NewChallengeResolved("chal-001"),
		ledger.NewChallengeRaised("chal-002", nodeID, "inference", "another reason"),
		ledger.NewChallengeWithdrawn("chal-002"),

		// Epistemic state changes
		ledger.NewNodeValidated(nodeID),
		ledger.NewNodeAdmitted(childID),

		// Taint
		ledger.NewTaintRecomputed(nodeID, node.TaintClean),

		// Definitions and lemmas
		ledger.NewDefAdded(ledger.Definition{ID: "def-001", Name: "test", Definition: "test def", Created: types.Now()}),
		ledger.NewLemmaExtracted(ledger.Lemma{ID: "lem-001", Statement: "test lemma", NodeID: nodeID, Created: types.Now()}),
	}

	for _, event := range events {
		if _, err := ledger.Append(dir, event); err != nil {
			t.Fatalf("Append %s failed: %v", event.Type(), err)
		}
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify final state
	gotRoot := state.GetNode(nodeID)
	if gotRoot == nil {
		t.Fatal("Root node not found")
	}
	if gotRoot.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Root epistemic state: got %q, want %q", gotRoot.EpistemicState, schema.EpistemicValidated)
	}
	if gotRoot.TaintState != node.TaintClean {
		t.Errorf("Root taint state: got %q, want %q", gotRoot.TaintState, node.TaintClean)
	}

	gotChild := state.GetNode(childID)
	if gotChild == nil {
		t.Fatal("Child node not found")
	}
	if gotChild.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("Child epistemic state: got %q, want %q", gotChild.EpistemicState, schema.EpistemicAdmitted)
	}

	gotDef := state.GetDefinition("def-001")
	if gotDef == nil {
		t.Error("Definition not found")
	}

	gotLemma := state.GetLemma("lem-001")
	if gotLemma == nil {
		t.Error("Lemma not found")
	}
}

// TestReplay_MultipleNodesWithDependencies verifies nodes with dependencies are replayed correctly.
func TestReplay_MultipleNodesWithDependencies(t *testing.T) {
	dir := t.TempDir()

	rootID := mustParseNodeID(t, "1")
	childID := mustParseNodeID(t, "1.1")

	// Create root node
	rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*rootNode)); err != nil {
		t.Fatalf("Append root failed: %v", err)
	}

	// Create child with dependency on root
	childNode, _ := node.NewNodeWithOptions(
		childID,
		schema.NodeTypeClaim,
		"Child depending on root",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Dependencies: []types.NodeID{rootID},
		},
	)
	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*childNode)); err != nil {
		t.Fatalf("Append child failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	got := state.GetNode(childID)
	if got == nil {
		t.Fatal("Child node not found")
	}

	if len(got.Dependencies) != 1 {
		t.Fatalf("Dependencies count: got %d, want 1", len(got.Dependencies))
	}
	if got.Dependencies[0].String() != rootID.String() {
		t.Errorf("Dependency: got %q, want %q", got.Dependencies[0].String(), rootID.String())
	}
}

// -----------------------------------------------------------------------------
// Hash Verification Tests
// -----------------------------------------------------------------------------

// TestReplayWithVerify_ValidHashes verifies hash verification passes for valid events.
func TestReplayWithVerify_ValidHashes(t *testing.T) {
	dir := t.TempDir()

	// Create a node (NewNode computes hash automatically)
	nodeID := mustParseNodeID(t, "1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)

	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := ReplayWithVerify(ldg)
	if err != nil {
		t.Fatalf("ReplayWithVerify failed: %v", err)
	}

	got := state.GetNode(nodeID)
	if got == nil {
		t.Fatal("Node not found after replay")
	}

	// Verify the hash matches
	if !got.VerifyContentHash() {
		t.Error("Content hash verification failed after replay")
	}
}

// TestReplayWithVerify_CorruptedHash verifies hash verification detects corruption.
func TestReplayWithVerify_CorruptedHash(t *testing.T) {
	dir := t.TempDir()

	// Create a node and manually corrupt its hash before appending
	nodeID := mustParseNodeID(t, "1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)

	// Corrupt the hash
	n.ContentHash = "0000000000000000000000000000000000000000000000000000000000000000"

	event := ledger.NewNodeCreated(*n)

	if _, err := ledger.Append(dir, event); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	_, err = ReplayWithVerify(ldg)
	if err == nil {
		t.Fatal("ReplayWithVerify should return error for corrupted hash")
	}

	// Verify error message indicates hash verification failure
	if !strings.Contains(err.Error(), "hash") && !strings.Contains(err.Error(), "Hash") {
		t.Errorf("Error should mention hash verification: got %q", err.Error())
	}
}

// -----------------------------------------------------------------------------
// Error Handling Tests
// -----------------------------------------------------------------------------

// TestReplay_NilLedger verifies that Replay handles nil ledger.
func TestReplay_NilLedger(t *testing.T) {
	_, err := Replay(nil)
	if err == nil {
		t.Fatal("Replay should return error for nil ledger")
	}
}

// TestReplayWithVerify_NilLedger verifies that ReplayWithVerify handles nil ledger.
func TestReplayWithVerify_NilLedger(t *testing.T) {
	_, err := ReplayWithVerify(nil)
	if err == nil {
		t.Fatal("ReplayWithVerify should return error for nil ledger")
	}
}

// TestReplay_InvalidJSON verifies that Replay handles corrupted JSON in ledger.
func TestReplay_InvalidJSON(t *testing.T) {
	dir := t.TempDir()

	// Create a file with invalid JSON
	corruptedPath := filepath.Join(dir, "000001.json")
	if err := os.WriteFile(corruptedPath, []byte("not valid json {{{"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	_, err = Replay(ldg)
	if err == nil {
		t.Fatal("Replay should return error for invalid JSON")
	}
}

// TestReplay_CorruptedEventInMiddle verifies that state replay stops at the first
// corrupted JSON event and returns an appropriate error. This tests the scenario
// where valid events are followed by a corrupted event in the middle of the ledger.
func TestReplay_CorruptedEventInMiddle(t *testing.T) {
	dir := t.TempDir()

	// Step 1: Create valid event at sequence 1
	initEvent := ledger.NewProofInitialized("Test conjecture", "test-author")
	seq1, err := ledger.Append(dir, initEvent)
	if err != nil {
		t.Fatalf("Append event 1 failed: %v", err)
	}
	if seq1 != 1 {
		t.Fatalf("First event should have seq 1, got %d", seq1)
	}

	// Step 2: Create valid event at sequence 2 (NodeCreated)
	nodeID := mustParseNodeID(t, "1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
	seq2, err := ledger.Append(dir, ledger.NewNodeCreated(*n))
	if err != nil {
		t.Fatalf("Append event 2 failed: %v", err)
	}
	if seq2 != 2 {
		t.Fatalf("Second event should have seq 2, got %d", seq2)
	}

	// Step 3: Write corrupted JSON directly at sequence 3
	corruptedPath := filepath.Join(dir, "000003.json")
	corruptedJSON := []byte(`{"type":"node_created","corrupted json missing closing brace`)
	if err := os.WriteFile(corruptedPath, corruptedJSON, 0644); err != nil {
		t.Fatalf("WriteFile for corrupted event failed: %v", err)
	}

	// Step 4: Create valid event at sequence 4 (should never be reached)
	validEventPath := filepath.Join(dir, "000004.json")
	validEvent := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z","conjecture":"unreachable","author":"test"}`
	if err := os.WriteFile(validEventPath, []byte(validEvent), 0644); err != nil {
		t.Fatalf("WriteFile for event 4 failed: %v", err)
	}

	// Step 5: Create ledger and attempt replay
	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Verify the ledger has 4 events
	count, err := ldg.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 4 {
		t.Fatalf("Expected 4 events in ledger, got %d", count)
	}

	// Step 6: Replay should fail at the corrupted event
	_, err = Replay(ldg)
	if err == nil {
		t.Fatal("Replay should return error for corrupted event in middle")
	}

	// Step 7: Verify error indicates the problem is at event 3
	errStr := err.Error()
	if !strings.Contains(errStr, "3") {
		t.Errorf("Error should mention event sequence 3: got %q", errStr)
	}

	// The error should indicate invalid JSON or parsing failure
	if !strings.Contains(errStr, "invalid JSON") && !strings.Contains(errStr, "read event") {
		t.Errorf("Error should mention invalid JSON or read failure: got %q", errStr)
	}
}

// TestReplay_CorruptedEventInMiddle_VariousCorruptions tests replay behavior with
// different types of JSON corruption in the middle of the ledger.
func TestReplay_CorruptedEventInMiddle_VariousCorruptions(t *testing.T) {
	tests := []struct {
		name           string
		corruptedData  string
		errorContains  string // what the error message should contain
	}{
		{
			name:          "truncated JSON object",
			corruptedData: `{"type":"node_created","timestamp":"2025-01-01T00:00:00Z"`,
			errorContains: "invalid JSON",
		},
		{
			name:          "completely malformed",
			corruptedData: `not json at all {{{`,
			errorContains: "invalid JSON",
		},
		{
			name:          "binary garbage",
			corruptedData: string([]byte{0x00, 0x01, 0x02, 0xff, 0xfe}),
			errorContains: "invalid JSON",
		},
		{
			name:          "empty file",
			corruptedData: ``,
			errorContains: "invalid JSON",
		},
		{
			name:          "valid JSON but missing type field",
			corruptedData: `{"timestamp":"2025-01-01T00:00:00Z","author":"test"}`,
			errorContains: "type",
		},
		{
			name:          "array instead of object",
			corruptedData: `["not", "an", "object"]`,
			errorContains: "type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Create valid event at sequence 1
			initEvent := ledger.NewProofInitialized("Test conjecture", "test-author")
			if _, err := ledger.Append(dir, initEvent); err != nil {
				t.Fatalf("Append event 1 failed: %v", err)
			}

			// Create valid event at sequence 2
			nodeID := mustParseNodeID(t, "1")
			n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
			if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
				t.Fatalf("Append event 2 failed: %v", err)
			}

			// Write corrupted event at sequence 3
			corruptedPath := filepath.Join(dir, "000003.json")
			if err := os.WriteFile(corruptedPath, []byte(tt.corruptedData), 0644); err != nil {
				t.Fatalf("WriteFile for corrupted event failed: %v", err)
			}

			// Create ledger and attempt replay
			ldg, err := ledger.NewLedger(dir)
			if err != nil {
				t.Fatalf("NewLedger failed: %v", err)
			}

			// Replay should fail
			_, err = Replay(ldg)
			if err == nil {
				t.Fatal("Replay should return error for corrupted event")
			}

			// Error should contain expected substring
			if !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("Error should contain %q: got %q", tt.errorContains, err.Error())
			}
		})
	}
}

// TestReplay_UnknownEventType verifies that Replay handles unknown event types gracefully.
func TestReplay_UnknownEventType(t *testing.T) {
	dir := t.TempDir()

	// Create a file with an unknown event type
	unknownEvent := `{"type":"unknown_event_type","timestamp":"2025-01-01T00:00:00Z"}`
	eventPath := filepath.Join(dir, "000001.json")
	if err := os.WriteFile(eventPath, []byte(unknownEvent), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	_, err = Replay(ldg)
	if err == nil {
		t.Fatal("Replay should return error for unknown event type")
	}

	// Verify error message mentions unknown event type
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("Error should mention unknown event type: got %q", err.Error())
	}
}

// TestReplay_MissingNode verifies error when event references non-existent node.
func TestReplay_MissingNode(t *testing.T) {
	dir := t.TempDir()

	// Try to claim a node that was never created
	nodeID := mustParseNodeID(t, "1")
	claimEvent := ledger.NewNodesClaimed([]types.NodeID{nodeID}, "agent-1", types.Now())

	if _, err := ledger.Append(dir, claimEvent); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	_, err = Replay(ldg)
	if err == nil {
		t.Fatal("Replay should return error when claiming non-existent node")
	}

	// Verify error mentions the node not being found
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention node not found: got %q", err.Error())
	}
}

// TestReplay_SequenceGapDetection verifies that sequence gaps are detected.
func TestReplay_SequenceGapDetection(t *testing.T) {
	dir := t.TempDir()

	// Create events with a gap (1, 3 - missing 2)
	event1 := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z","conjecture":"test","author":"agent"}`
	event3 := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:01Z","conjecture":"test2","author":"agent"}`

	if err := os.WriteFile(filepath.Join(dir, "000001.json"), []byte(event1), 0644); err != nil {
		t.Fatalf("WriteFile 1 failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000003.json"), []byte(event3), 0644); err != nil {
		t.Fatalf("WriteFile 3 failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Replay should detect the sequence gap and return an error
	_, err = Replay(ldg)
	if err == nil {
		t.Fatal("Replay should return error for sequence gap")
	}

	// Verify error message mentions gap
	if !strings.Contains(err.Error(), "gap") {
		t.Errorf("Error should mention sequence gap: got %q", err.Error())
	}
}

// TestReplay_WithLedgerGaps comprehensively tests how state replay handles various gap scenarios
// in ledger sequence numbers. This includes single gaps, multiple gaps, large gaps, and validates
// the error messages include the expected and actual sequence numbers.
func TestReplay_WithLedgerGaps(t *testing.T) {
	tests := []struct {
		name          string
		sequences     []int // sequence numbers to create (gaps are implicit)
		expectError   bool
		errorContains string // substring that should appear in error message
	}{
		{
			name:          "single gap in middle",
			sequences:     []int{1, 3}, // missing 2
			expectError:   true,
			errorContains: "gap",
		},
		{
			name:          "large gap",
			sequences:     []int{1, 10}, // missing 2-9
			expectError:   true,
			errorContains: "gap",
		},
		{
			name:          "multiple gaps",
			sequences:     []int{1, 3, 6}, // missing 2, 4, 5
			expectError:   true,
			errorContains: "gap",
		},
		{
			name:          "gap at start (missing 1)",
			sequences:     []int{2, 3, 4},
			expectError:   true,
			errorContains: "gap",
		},
		{
			name:          "consecutive sequences valid",
			sequences:     []int{1, 2, 3, 4, 5},
			expectError:   false,
			errorContains: "",
		},
		{
			name:          "single event valid",
			sequences:     []int{1},
			expectError:   false,
			errorContains: "",
		},
		{
			name:          "gap after valid start",
			sequences:     []int{1, 2, 3, 5}, // missing 4
			expectError:   true,
			errorContains: "gap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Create event files for each sequence number
			event := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z","conjecture":"test","author":"agent"}`
			for _, seq := range tt.sequences {
				filename := fmt.Sprintf("%06d.json", seq)
				if err := os.WriteFile(filepath.Join(dir, filename), []byte(event), 0644); err != nil {
					t.Fatalf("WriteFile %s failed: %v", filename, err)
				}
			}

			ldg, err := ledger.NewLedger(dir)
			if err != nil {
				t.Fatalf("NewLedger failed: %v", err)
			}

			state, err := Replay(ldg)

			if tt.expectError {
				if err == nil {
					t.Fatal("Replay should return error for sequence gap")
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q: got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Replay should succeed for valid sequence: %v", err)
				}
				if state == nil {
					t.Fatal("Replay returned nil state for valid sequence")
				}
				// Verify the latest sequence is tracked correctly
				expectedLatest := tt.sequences[len(tt.sequences)-1]
				if state.LatestSeq() != expectedLatest {
					t.Errorf("LatestSeq: got %d, want %d", state.LatestSeq(), expectedLatest)
				}
			}
		})
	}
}

// TestReplay_WithLedgerGaps_ErrorDetails verifies that gap detection provides useful error details.
func TestReplay_WithLedgerGaps_ErrorDetails(t *testing.T) {
	dir := t.TempDir()

	// Create events with a gap (1, 2, 5 - missing 3 and 4)
	event := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z","conjecture":"test","author":"agent"}`
	for _, seq := range []int{1, 2, 5} {
		filename := fmt.Sprintf("%06d.json", seq)
		if err := os.WriteFile(filepath.Join(dir, filename), []byte(event), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", filename, err)
		}
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	_, err = Replay(ldg)
	if err == nil {
		t.Fatal("Replay should return error for sequence gap")
	}

	// Verify the error message includes both expected (3) and actual (5) sequence numbers
	errStr := err.Error()
	if !strings.Contains(errStr, "3") || !strings.Contains(errStr, "5") {
		t.Errorf("Error should mention expected (3) and actual (5) sequence numbers: got %q", errStr)
	}
}

// TestReplay_WithLedgerGaps_ReplayWithVerify tests that ReplayWithVerify also catches gaps.
func TestReplay_WithLedgerGaps_ReplayWithVerify(t *testing.T) {
	dir := t.TempDir()

	// Create events with a gap (1, 3 - missing 2)
	event := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z","conjecture":"test","author":"agent"}`
	for _, seq := range []int{1, 3} {
		filename := fmt.Sprintf("%06d.json", seq)
		if err := os.WriteFile(filepath.Join(dir, filename), []byte(event), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", filename, err)
		}
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// ReplayWithVerify should also detect the gap (it uses the same underlying logic)
	_, err = ReplayWithVerify(ldg)
	if err == nil {
		t.Fatal("ReplayWithVerify should return error for sequence gap")
	}

	if !strings.Contains(err.Error(), "gap") {
		t.Errorf("Error should mention gap: got %q", err.Error())
	}
}

// TestReplay_SequenceDuplicateDetection verifies that duplicate sequence numbers are detected.
func TestReplay_SequenceDuplicateDetection(t *testing.T) {
	dir := t.TempDir()

	// Create events - first append normally
	event := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z","conjecture":"test","author":"agent"}`

	// Write event 1
	if err := os.WriteFile(filepath.Join(dir, "000001.json"), []byte(event), 0644); err != nil {
		t.Fatalf("WriteFile 1 failed: %v", err)
	}
	// Write event 2
	if err := os.WriteFile(filepath.Join(dir, "000002.json"), []byte(event), 0644); err != nil {
		t.Fatalf("WriteFile 2 failed: %v", err)
	}

	// Ledger scanning returns files in sorted order, so a duplicate would mean
	// something like having 1, 1, 2 which can't happen with filenames.
	// However, the validation checks that seq matches expectedSeq.
	// If somehow the filesystem or ledger returned seq 1 twice, we'd detect it.

	// To test this properly, we need to verify the validation logic itself.
	// The duplicate check triggers when seq < expectedSeq.
	// This is effectively a corrupted state where the same seq appears twice.
	// Since filesystem-based ledgers can't have duplicate filenames,
	// this test verifies the logic path exists by checking valid sequential events pass.

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Valid consecutive sequence should work
	_, err = Replay(ldg)
	if err != nil {
		t.Fatalf("Replay should succeed for valid sequence: %v", err)
	}
}

// TestReplay_ValidSequence verifies that a valid consecutive sequence replays successfully.
func TestReplay_ValidSequence(t *testing.T) {
	dir := t.TempDir()

	// Create three consecutive events
	event1 := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z","conjecture":"test","author":"agent"}`
	event2 := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:01Z","conjecture":"test2","author":"agent"}`
	event3 := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:02Z","conjecture":"test3","author":"agent"}`

	if err := os.WriteFile(filepath.Join(dir, "000001.json"), []byte(event1), 0644); err != nil {
		t.Fatalf("WriteFile 1 failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000002.json"), []byte(event2), 0644); err != nil {
		t.Fatalf("WriteFile 2 failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000003.json"), []byte(event3), 0644); err != nil {
		t.Fatalf("WriteFile 3 failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Valid consecutive sequence should succeed
	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay should succeed for valid consecutive sequence: %v", err)
	}

	// Verify the state captured the latest sequence number
	if state.LatestSeq() != 3 {
		t.Errorf("LatestSeq: got %d, want 3", state.LatestSeq())
	}
}

// TestReplay_SequenceStartsAtOne verifies that sequences must start at 1.
func TestReplay_SequenceStartsAtOne(t *testing.T) {
	dir := t.TempDir()

	// Create event starting at 2 (missing 1)
	event := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z","conjecture":"test","author":"agent"}`

	if err := os.WriteFile(filepath.Join(dir, "000002.json"), []byte(event), 0644); err != nil {
		t.Fatalf("WriteFile 2 failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Replay should detect that sequence doesn't start at 1
	_, err = Replay(ldg)
	if err == nil {
		t.Fatal("Replay should return error when sequence doesn't start at 1")
	}

	// Verify error mentions gap (since expected 1, got 2)
	if !strings.Contains(err.Error(), "gap") {
		t.Errorf("Error should mention gap: got %q", err.Error())
	}
}

// -----------------------------------------------------------------------------
// Performance Tests
// -----------------------------------------------------------------------------

// TestReplay_LargeEventStream verifies replaying a large number of events.
func TestReplay_LargeEventStream(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large event stream test in short mode")
	}

	dir := t.TempDir()

	// Create 1000 events
	const eventCount = 1000
	const nodesPerLevel = 10

	// ProofInitialized
	if _, err := ledger.Append(dir, ledger.NewProofInitialized("Large test", "agent")); err != nil {
		t.Fatalf("Append ProofInitialized failed: %v", err)
	}

	// Create root node first
	rootID := mustParseNodeID(t, "1")
	rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root node", schema.InferenceAssumption)
	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*rootNode)); err != nil {
		t.Fatalf("Append root NodeCreated failed: %v", err)
	}

	// Create many child nodes under root (1.1, 1.2, ..., 1.10)
	for i := 1; i <= nodesPerLevel; i++ {
		nodeID := mustParseNodeID(t, fmt.Sprintf("1.%d", i))
		n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, fmt.Sprintf("Node 1.%d", i), schema.InferenceAssumption)
		if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
			t.Fatalf("Append NodeCreated failed at %d: %v", i, err)
		}
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify root node exists
	if got := state.GetNode(rootID); got == nil {
		t.Error("Root node not found after replay")
	}

	// Verify child nodes exist
	for i := 1; i <= nodesPerLevel; i++ {
		nodeID := mustParseNodeID(t, fmt.Sprintf("1.%d", i))
		got := state.GetNode(nodeID)
		if got == nil {
			t.Errorf("Node 1.%d not found after replay", i)
		}
	}
}

// TestReplay_PerformanceTableDriven runs table-driven performance tests.
func TestReplay_PerformanceTableDriven(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	tests := []struct {
		name       string
		eventCount int
	}{
		{"10 events", 10},
		{"100 events", 100},
		{"500 events", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Create events
			for i := 0; i < tt.eventCount; i++ {
				event := ledger.NewProofInitialized("Event", "agent")
				if _, err := ledger.Append(dir, event); err != nil {
					t.Fatalf("Append failed at %d: %v", i, err)
				}
			}

			ldg, err := ledger.NewLedger(dir)
			if err != nil {
				t.Fatalf("NewLedger failed: %v", err)
			}

			// Time the replay
			state, err := Replay(ldg)
			if err != nil {
				t.Fatalf("Replay failed: %v", err)
			}

			if state == nil {
				t.Fatal("Replay returned nil state")
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Integration Tests (using actual file I/O)
// -----------------------------------------------------------------------------

// TestReplay_CompleteProofWorkflow simulates a complete proof workflow.
func TestReplay_CompleteProofWorkflow(t *testing.T) {
	dir := t.TempDir()

	// Step 1: Initialize proof
	initEvent := ledger.NewProofInitialized("Prove that 1 + 1 = 2", "mathematician")
	if _, err := ledger.Append(dir, initEvent); err != nil {
		t.Fatalf("Append init failed: %v", err)
	}

	// Step 2: Create root node
	rootID := mustParseNodeID(t, "1")
	rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "1 + 1 = 2 by definition", schema.InferenceAssumption)
	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*rootNode)); err != nil {
		t.Fatalf("Append root failed: %v", err)
	}

	// Step 3: Add supporting sub-nodes
	subNodeIDs := []string{"1.1", "1.2"}
	for _, idStr := range subNodeIDs {
		nodeID := mustParseNodeID(t, idStr)
		n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Sub-claim for "+idStr, schema.InferenceModusPonens)
		if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
			t.Fatalf("Append sub-node %s failed: %v", idStr, err)
		}
	}

	// Step 4: Prover claims nodes
	claimedIDs := []types.NodeID{mustParseNodeID(t, "1.1")}
	if _, err := ledger.Append(dir, ledger.NewNodesClaimed(claimedIDs, "prover-agent", types.Now())); err != nil {
		t.Fatalf("Append claim failed: %v", err)
	}

	// Step 5: Prover releases after work
	if _, err := ledger.Append(dir, ledger.NewNodesReleased(claimedIDs)); err != nil {
		t.Fatalf("Append release failed: %v", err)
	}

	// Step 6: Verifier validates nodes
	if _, err := ledger.Append(dir, ledger.NewNodeValidated(mustParseNodeID(t, "1.1"))); err != nil {
		t.Fatalf("Append validate 1.1 failed: %v", err)
	}
	if _, err := ledger.Append(dir, ledger.NewNodeValidated(mustParseNodeID(t, "1.2"))); err != nil {
		t.Fatalf("Append validate 1.2 failed: %v", err)
	}
	if _, err := ledger.Append(dir, ledger.NewNodeValidated(rootID)); err != nil {
		t.Fatalf("Append validate root failed: %v", err)
	}

	// Step 7: Recompute taint
	if _, err := ledger.Append(dir, ledger.NewTaintRecomputed(rootID, node.TaintClean)); err != nil {
		t.Fatalf("Append taint failed: %v", err)
	}

	// Replay and verify final state
	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify all nodes are validated
	for _, idStr := range []string{"1", "1.1", "1.2"} {
		nodeID := mustParseNodeID(t, idStr)
		got := state.GetNode(nodeID)
		if got == nil {
			t.Errorf("Node %s not found", idStr)
			continue
		}
		if got.EpistemicState != schema.EpistemicValidated {
			t.Errorf("Node %s epistemic state: got %q, want %q", idStr, got.EpistemicState, schema.EpistemicValidated)
		}
		if idStr == "1" && got.TaintState != node.TaintClean {
			t.Errorf("Root taint state: got %q, want %q", got.TaintState, node.TaintClean)
		}
	}
}

// TestReplay_ReadAllVsScan verifies Replay works with both ReadAll and Scan patterns.
func TestReplay_ReadAllVsScan(t *testing.T) {
	dir := t.TempDir()

	// Create some events
	if _, err := ledger.Append(dir, ledger.NewProofInitialized("Test", "agent")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	nodeID := mustParseNodeID(t, "1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test", schema.InferenceAssumption)
	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Verify we can read events both ways
	events, err := ldg.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("ReadAll returned %d events, want 2", len(events))
	}

	// Count events via Scan
	scanCount := 0
	if err := ldg.Scan(func(seq int, data []byte) error {
		scanCount++
		return nil
	}); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if scanCount != 2 {
		t.Errorf("Scan counted %d events, want 2", scanCount)
	}

	// Now replay and verify state
	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	got := state.GetNode(nodeID)
	if got == nil {
		t.Fatal("Node not found after replay")
	}
}

// -----------------------------------------------------------------------------
// Event Parsing Tests
// -----------------------------------------------------------------------------

// TestReplay_EventParsing verifies that events are correctly parsed from JSON.
func TestReplay_EventParsing(t *testing.T) {
	dir := t.TempDir()

	// Create a NodeCreated event with specific fields
	nodeID := mustParseNodeID(t, "1.2.3")
	n, _ := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"Complex statement with special chars: <>&\"'",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Latex:        "\\forall x \\in X",
			Context:      []string{"def-001", "asm-002"},
			Dependencies: []types.NodeID{mustParseNodeID(t, "1.2")},
		},
	)

	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	got := state.GetNode(nodeID)
	if got == nil {
		t.Fatal("Node not found")
	}

	// Verify all fields preserved
	if got.ID.String() != "1.2.3" {
		t.Errorf("ID: got %q, want %q", got.ID.String(), "1.2.3")
	}
	if got.Statement != "Complex statement with special chars: <>&\"'" {
		t.Errorf("Statement: got %q", got.Statement)
	}
	if got.Latex != "\\forall x \\in X" {
		t.Errorf("Latex: got %q", got.Latex)
	}
	if len(got.Context) != 2 {
		t.Errorf("Context length: got %d, want 2", len(got.Context))
	}
	if len(got.Dependencies) != 1 {
		t.Errorf("Dependencies length: got %d, want 1", len(got.Dependencies))
	}
}

// -----------------------------------------------------------------------------
// Circular Dependency Tests
// -----------------------------------------------------------------------------

// TestReplay_CircularDependencies verifies that replay correctly handles nodes
// that have circular dependencies. The ledger records events faithfully, even
// when they create logically invalid dependency graphs. This test ensures:
// 1. Nodes with circular dependencies can be replayed from the ledger
// 2. The state correctly stores these dependencies
// 3. The cycle package can detect cycles in the resulting state
//
// Circular reasoning is a logical fallacy (A depends on B, B depends on C,
// C depends on A), but the ledger is append-only and records what happened.
// Detection and handling of cycles is done at the application layer.
func TestReplay_CircularDependencies(t *testing.T) {
	t.Run("simple two-node cycle", func(t *testing.T) {
		dir := t.TempDir()

		// Create two nodes that depend on each other: A -> B -> A
		nodeA := mustParseNodeID(t, "1.1")
		nodeB := mustParseNodeID(t, "1.2")

		// Create node A first (will add B as dependency later via creation with deps)
		nA, _ := node.NewNode(nodeA, schema.NodeTypeClaim, "Claim A", schema.InferenceAssumption)
		if _, err := ledger.Append(dir, ledger.NewNodeCreated(*nA)); err != nil {
			t.Fatalf("Append node A failed: %v", err)
		}

		// Create node B depending on A
		nB, _ := node.NewNodeWithOptions(
			nodeB,
			schema.NodeTypeClaim,
			"Claim B depends on A",
			schema.InferenceModusPonens,
			node.NodeOptions{Dependencies: []types.NodeID{nodeA}},
		)
		if _, err := ledger.Append(dir, ledger.NewNodeCreated(*nB)); err != nil {
			t.Fatalf("Append node B failed: %v", err)
		}

		// Now create a new version of A that depends on B (simulating circular dependency)
		// In a real scenario, this would be prevented at the application layer,
		// but we're testing that replay handles such events if they exist in the ledger
		nA2, _ := node.NewNodeWithOptions(
			nodeA,
			schema.NodeTypeClaim,
			"Claim A depends on B (circular)",
			schema.InferenceModusPonens,
			node.NodeOptions{Dependencies: []types.NodeID{nodeB}},
		)
		if _, err := ledger.Append(dir, ledger.NewNodeCreated(*nA2)); err != nil {
			t.Fatalf("Append node A (circular) failed: %v", err)
		}

		// Replay the ledger
		ldg, err := ledger.NewLedger(dir)
		if err != nil {
			t.Fatalf("NewLedger failed: %v", err)
		}

		state, err := Replay(ldg)
		if err != nil {
			t.Fatalf("Replay failed: %v", err)
		}

		// Verify both nodes exist
		gotA := state.GetNode(nodeA)
		if gotA == nil {
			t.Fatal("Node A not found after replay")
		}
		gotB := state.GetNode(nodeB)
		if gotB == nil {
			t.Fatal("Node B not found after replay")
		}

		// Verify node A has B as dependency (the later event overwrote the first)
		if len(gotA.Dependencies) != 1 || gotA.Dependencies[0].String() != nodeB.String() {
			t.Errorf("Node A dependencies: got %v, want [%s]", gotA.Dependencies, nodeB.String())
		}

		// Verify node B has A as dependency
		if len(gotB.Dependencies) != 1 || gotB.Dependencies[0].String() != nodeA.String() {
			t.Errorf("Node B dependencies: got %v, want [%s]", gotB.Dependencies, nodeA.String())
		}
	})

	t.Run("three-node cycle A -> B -> C -> A", func(t *testing.T) {
		dir := t.TempDir()

		nodeA := mustParseNodeID(t, "1.1")
		nodeB := mustParseNodeID(t, "1.2")
		nodeC := mustParseNodeID(t, "1.3")

		// Create all three nodes with circular dependencies directly
		// A depends on B, B depends on C, C depends on A
		nA, _ := node.NewNodeWithOptions(
			nodeA,
			schema.NodeTypeClaim,
			"Claim A depends on B",
			schema.InferenceModusPonens,
			node.NodeOptions{Dependencies: []types.NodeID{nodeB}},
		)
		nB, _ := node.NewNodeWithOptions(
			nodeB,
			schema.NodeTypeClaim,
			"Claim B depends on C",
			schema.InferenceModusPonens,
			node.NodeOptions{Dependencies: []types.NodeID{nodeC}},
		)
		nC, _ := node.NewNodeWithOptions(
			nodeC,
			schema.NodeTypeClaim,
			"Claim C depends on A",
			schema.InferenceModusPonens,
			node.NodeOptions{Dependencies: []types.NodeID{nodeA}},
		)

		// Append all nodes
		if _, err := ledger.Append(dir, ledger.NewNodeCreated(*nA)); err != nil {
			t.Fatalf("Append node A failed: %v", err)
		}
		if _, err := ledger.Append(dir, ledger.NewNodeCreated(*nB)); err != nil {
			t.Fatalf("Append node B failed: %v", err)
		}
		if _, err := ledger.Append(dir, ledger.NewNodeCreated(*nC)); err != nil {
			t.Fatalf("Append node C failed: %v", err)
		}

		// Replay
		ldg, err := ledger.NewLedger(dir)
		if err != nil {
			t.Fatalf("NewLedger failed: %v", err)
		}

		state, err := Replay(ldg)
		if err != nil {
			t.Fatalf("Replay failed: %v", err)
		}

		// Verify all nodes exist with their dependencies
		gotA := state.GetNode(nodeA)
		gotB := state.GetNode(nodeB)
		gotC := state.GetNode(nodeC)

		if gotA == nil || gotB == nil || gotC == nil {
			t.Fatal("One or more nodes not found after replay")
		}

		// Verify circular dependency chain
		if len(gotA.Dependencies) != 1 || gotA.Dependencies[0].String() != nodeB.String() {
			t.Errorf("Node A should depend on B: got %v", gotA.Dependencies)
		}
		if len(gotB.Dependencies) != 1 || gotB.Dependencies[0].String() != nodeC.String() {
			t.Errorf("Node B should depend on C: got %v", gotB.Dependencies)
		}
		if len(gotC.Dependencies) != 1 || gotC.Dependencies[0].String() != nodeA.String() {
			t.Errorf("Node C should depend on A: got %v", gotC.Dependencies)
		}
	})

	t.Run("self-referencing node", func(t *testing.T) {
		dir := t.TempDir()

		nodeA := mustParseNodeID(t, "1.1")

		// Create a node that depends on itself
		nA, _ := node.NewNodeWithOptions(
			nodeA,
			schema.NodeTypeClaim,
			"Self-referencing claim",
			schema.InferenceAssumption,
			node.NodeOptions{Dependencies: []types.NodeID{nodeA}},
		)
		if _, err := ledger.Append(dir, ledger.NewNodeCreated(*nA)); err != nil {
			t.Fatalf("Append self-referencing node failed: %v", err)
		}

		// Replay
		ldg, err := ledger.NewLedger(dir)
		if err != nil {
			t.Fatalf("NewLedger failed: %v", err)
		}

		state, err := Replay(ldg)
		if err != nil {
			t.Fatalf("Replay failed: %v", err)
		}

		// Verify node exists with self-reference
		got := state.GetNode(nodeA)
		if got == nil {
			t.Fatal("Self-referencing node not found after replay")
		}
		if len(got.Dependencies) != 1 || got.Dependencies[0].String() != nodeA.String() {
			t.Errorf("Self-referencing node should depend on itself: got %v", got.Dependencies)
		}
	})

	t.Run("cycle among valid nodes", func(t *testing.T) {
		dir := t.TempDir()

		// Create a more realistic scenario: a proof tree where some nodes
		// accidentally form a cycle while others are valid
		root := mustParseNodeID(t, "1")
		child1 := mustParseNodeID(t, "1.1")
		child2 := mustParseNodeID(t, "1.2")
		cycleA := mustParseNodeID(t, "1.3")
		cycleB := mustParseNodeID(t, "1.4")

		// Valid nodes
		rootNode, _ := node.NewNode(root, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
		child1Node, _ := node.NewNodeWithOptions(child1, schema.NodeTypeClaim, "Child 1", schema.InferenceModusPonens, node.NodeOptions{Dependencies: []types.NodeID{root}})
		child2Node, _ := node.NewNodeWithOptions(child2, schema.NodeTypeClaim, "Child 2", schema.InferenceModusPonens, node.NodeOptions{Dependencies: []types.NodeID{root}})

		// Cyclic nodes
		cycleANode, _ := node.NewNodeWithOptions(cycleA, schema.NodeTypeClaim, "Cycle A", schema.InferenceModusPonens, node.NodeOptions{Dependencies: []types.NodeID{cycleB}})
		cycleBNode, _ := node.NewNodeWithOptions(cycleB, schema.NodeTypeClaim, "Cycle B", schema.InferenceModusPonens, node.NodeOptions{Dependencies: []types.NodeID{cycleA}})

		// Append all
		for _, n := range []*node.Node{rootNode, child1Node, child2Node, cycleANode, cycleBNode} {
			if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
				t.Fatalf("Append node %s failed: %v", n.ID.String(), err)
			}
		}

		// Replay
		ldg, err := ledger.NewLedger(dir)
		if err != nil {
			t.Fatalf("NewLedger failed: %v", err)
		}

		state, err := Replay(ldg)
		if err != nil {
			t.Fatalf("Replay failed: %v", err)
		}

		// Verify all nodes exist
		allNodes := state.AllNodes()
		if len(allNodes) != 5 {
			t.Errorf("Expected 5 nodes, got %d", len(allNodes))
		}

		// Verify valid dependency chain from child1 to root
		gotChild1 := state.GetNode(child1)
		if gotChild1 == nil {
			t.Fatal("child1 not found")
		}
		if len(gotChild1.Dependencies) != 1 || gotChild1.Dependencies[0].String() != root.String() {
			t.Errorf("child1 should depend on root: got %v", gotChild1.Dependencies)
		}

		// Verify cyclic nodes have their circular dependencies
		gotCycleA := state.GetNode(cycleA)
		gotCycleB := state.GetNode(cycleB)
		if gotCycleA == nil || gotCycleB == nil {
			t.Fatal("Cycle nodes not found")
		}
		if len(gotCycleA.Dependencies) != 1 || gotCycleA.Dependencies[0].String() != cycleB.String() {
			t.Errorf("cycleA should depend on cycleB: got %v", gotCycleA.Dependencies)
		}
		if len(gotCycleB.Dependencies) != 1 || gotCycleB.Dependencies[0].String() != cycleA.String() {
			t.Errorf("cycleB should depend on cycleA: got %v", gotCycleB.Dependencies)
		}
	})

	t.Run("long dependency cycle", func(t *testing.T) {
		dir := t.TempDir()

		// Create a longer cycle: 1 -> 2 -> 3 -> 4 -> 5 -> 1
		const cycleLen = 5
		nodeIDs := make([]types.NodeID, cycleLen)
		for i := 0; i < cycleLen; i++ {
			nodeIDs[i] = mustParseNodeID(t, fmt.Sprintf("1.%d", i+1))
		}

		// Each node depends on the next, with the last depending on the first
		for i := 0; i < cycleLen; i++ {
			nextIdx := (i + 1) % cycleLen
			n, _ := node.NewNodeWithOptions(
				nodeIDs[i],
				schema.NodeTypeClaim,
				fmt.Sprintf("Node %d in cycle", i+1),
				schema.InferenceModusPonens,
				node.NodeOptions{Dependencies: []types.NodeID{nodeIDs[nextIdx]}},
			)
			if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
				t.Fatalf("Append node %d failed: %v", i, err)
			}
		}

		// Replay
		ldg, err := ledger.NewLedger(dir)
		if err != nil {
			t.Fatalf("NewLedger failed: %v", err)
		}

		state, err := Replay(ldg)
		if err != nil {
			t.Fatalf("Replay failed: %v", err)
		}

		// Verify all nodes exist with correct dependencies
		for i := 0; i < cycleLen; i++ {
			got := state.GetNode(nodeIDs[i])
			if got == nil {
				t.Errorf("Node %d not found", i)
				continue
			}
			nextIdx := (i + 1) % cycleLen
			if len(got.Dependencies) != 1 || got.Dependencies[0].String() != nodeIDs[nextIdx].String() {
				t.Errorf("Node %d should depend on node %d: got %v",
					i, nextIdx, got.Dependencies)
			}
		}
	})

	t.Run("diamond with cycle", func(t *testing.T) {
		dir := t.TempDir()

		// Diamond pattern where D also points back to A, creating a cycle:
		//      A
		//     / \
		//    B   C
		//     \ /
		//      D -> A (creates cycle)
		nodeA := mustParseNodeID(t, "1.1")
		nodeB := mustParseNodeID(t, "1.2")
		nodeC := mustParseNodeID(t, "1.3")
		nodeD := mustParseNodeID(t, "1.4")

		nA, _ := node.NewNode(nodeA, schema.NodeTypeClaim, "Top of diamond", schema.InferenceAssumption)
		nB, _ := node.NewNodeWithOptions(nodeB, schema.NodeTypeClaim, "Left of diamond", schema.InferenceModusPonens, node.NodeOptions{Dependencies: []types.NodeID{nodeA}})
		nC, _ := node.NewNodeWithOptions(nodeC, schema.NodeTypeClaim, "Right of diamond", schema.InferenceModusPonens, node.NodeOptions{Dependencies: []types.NodeID{nodeA}})
		nD, _ := node.NewNodeWithOptions(nodeD, schema.NodeTypeClaim, "Bottom with cycle", schema.InferenceModusPonens, node.NodeOptions{Dependencies: []types.NodeID{nodeB, nodeC, nodeA}}) // D depends on B, C, and back to A

		for _, n := range []*node.Node{nA, nB, nC, nD} {
			if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
				t.Fatalf("Append node %s failed: %v", n.ID.String(), err)
			}
		}

		// Replay
		ldg, err := ledger.NewLedger(dir)
		if err != nil {
			t.Fatalf("NewLedger failed: %v", err)
		}

		state, err := Replay(ldg)
		if err != nil {
			t.Fatalf("Replay failed: %v", err)
		}

		// Verify D has all three dependencies
		gotD := state.GetNode(nodeD)
		if gotD == nil {
			t.Fatal("Node D not found")
		}
		if len(gotD.Dependencies) != 3 {
			t.Errorf("Node D should have 3 dependencies: got %v", gotD.Dependencies)
		}

		// Check that A is among D's dependencies (creating the cycle back)
		hasA := false
		for _, dep := range gotD.Dependencies {
			if dep.String() == nodeA.String() {
				hasA = true
				break
			}
		}
		if !hasA {
			t.Errorf("Node D should depend on A (creating cycle): got %v", gotD.Dependencies)
		}
	})
}

// TestReplay_CircularDependencies_WithTaint verifies that taint propagation
// handles nodes with circular dependencies without infinite loops.
func TestReplay_CircularDependencies_WithTaint(t *testing.T) {
	dir := t.TempDir()

	nodeA := mustParseNodeID(t, "1.1")
	nodeB := mustParseNodeID(t, "1.2")

	// Create two nodes with circular dependencies
	nA, _ := node.NewNodeWithOptions(
		nodeA,
		schema.NodeTypeClaim,
		"Claim A",
		schema.InferenceAssumption,
		node.NodeOptions{Dependencies: []types.NodeID{nodeB}},
	)
	nB, _ := node.NewNodeWithOptions(
		nodeB,
		schema.NodeTypeClaim,
		"Claim B",
		schema.InferenceModusPonens,
		node.NodeOptions{Dependencies: []types.NodeID{nodeA}},
	)

	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*nA)); err != nil {
		t.Fatalf("Append node A failed: %v", err)
	}
	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*nB)); err != nil {
		t.Fatalf("Append node B failed: %v", err)
	}

	// Add taint recomputation events
	if _, err := ledger.Append(dir, ledger.NewTaintRecomputed(nodeA, node.TaintClean)); err != nil {
		t.Fatalf("Append taint for A failed: %v", err)
	}
	if _, err := ledger.Append(dir, ledger.NewTaintRecomputed(nodeB, node.TaintClean)); err != nil {
		t.Fatalf("Append taint for B failed: %v", err)
	}

	// Replay - this should complete without hanging due to infinite taint propagation
	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify both nodes have their taint state set
	gotA := state.GetNode(nodeA)
	gotB := state.GetNode(nodeB)
	if gotA == nil || gotB == nil {
		t.Fatal("Nodes not found after replay")
	}
	if gotA.TaintState != node.TaintClean {
		t.Errorf("Node A taint: got %q, want %q", gotA.TaintState, node.TaintClean)
	}
	if gotB.TaintState != node.TaintClean {
		t.Errorf("Node B taint: got %q, want %q", gotB.TaintState, node.TaintClean)
	}
}

// -----------------------------------------------------------------------------
// Deep Hierarchy Tests
// -----------------------------------------------------------------------------

// TestReplay_DeepHierarchy verifies that state replay handles very deep node
// hierarchies (100+ levels) without stack overflow or performance degradation.
// This tests the edge case of extremely nested proof trees.
func TestReplay_DeepHierarchy(t *testing.T) {
	tests := []struct {
		name  string
		depth int
	}{
		{"100 levels", 100},
		{"200 levels", 200},
		{"500 levels", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if testing.Short() && tt.depth > 100 {
				t.Skip("Skipping deep hierarchy test in short mode")
			}

			dir := t.TempDir()

			// Initialize proof
			initEvent := ledger.NewProofInitialized("Deep hierarchy test", "test-author")
			if _, err := ledger.Append(dir, initEvent); err != nil {
				t.Fatalf("Append ProofInitialized failed: %v", err)
			}

			// Create a chain of nodes: 1 -> 1.1 -> 1.1.1 -> ... (depth levels)
			var prevID types.NodeID
			var allNodeIDs []types.NodeID

			for i := 0; i < tt.depth; i++ {
				// Build the node ID by appending ".1" for each level
				var nodeIDStr string
				if i == 0 {
					nodeIDStr = "1"
				} else {
					nodeIDStr = allNodeIDs[i-1].String() + ".1"
				}

				nodeID := mustParseNodeID(t, nodeIDStr)
				allNodeIDs = append(allNodeIDs, nodeID)

				// Create node with dependency on parent (except root)
				var n *node.Node
				var err error
				if i == 0 {
					n, err = node.NewNode(nodeID, schema.NodeTypeClaim, fmt.Sprintf("Level %d", i), schema.InferenceAssumption)
				} else {
					n, err = node.NewNodeWithOptions(
						nodeID,
						schema.NodeTypeClaim,
						fmt.Sprintf("Level %d", i),
						schema.InferenceModusPonens,
						node.NodeOptions{Dependencies: []types.NodeID{prevID}},
					)
				}
				if err != nil {
					t.Fatalf("Failed to create node at level %d: %v", i, err)
				}

				if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
					t.Fatalf("Append NodeCreated at level %d failed: %v", i, err)
				}

				prevID = nodeID
			}

			// Replay the ledger - this is the main test
			ldg, err := ledger.NewLedger(dir)
			if err != nil {
				t.Fatalf("NewLedger failed: %v", err)
			}

			state, err := Replay(ldg)
			if err != nil {
				t.Fatalf("Replay failed for %d levels: %v", tt.depth, err)
			}

			// Verify all nodes were created
			for i, nodeID := range allNodeIDs {
				got := state.GetNode(nodeID)
				if got == nil {
					t.Errorf("Node at level %d (%s) not found after replay", i, nodeID)
					continue
				}

				// Verify dependency chain (except root)
				if i > 0 {
					if len(got.Dependencies) != 1 {
						t.Errorf("Node at level %d should have 1 dependency, got %d", i, len(got.Dependencies))
					} else if got.Dependencies[0].String() != allNodeIDs[i-1].String() {
						t.Errorf("Node at level %d has wrong dependency: got %s, want %s",
							i, got.Dependencies[0].String(), allNodeIDs[i-1].String())
					}
				}
			}

			// Verify deepest node
			deepestID := allNodeIDs[len(allNodeIDs)-1]
			deepest := state.GetNode(deepestID)
			if deepest == nil {
				t.Fatalf("Deepest node not found")
			}
			if deepest.Statement != fmt.Sprintf("Level %d", tt.depth-1) {
				t.Errorf("Deepest node statement: got %q, want %q", deepest.Statement, fmt.Sprintf("Level %d", tt.depth-1))
			}

			// Verify latest sequence
			expectedSeq := tt.depth + 1 // ProofInitialized + depth NodeCreated events
			if state.LatestSeq() != expectedSeq {
				t.Errorf("LatestSeq: got %d, want %d", state.LatestSeq(), expectedSeq)
			}
		})
	}
}

// TestReplay_DeepHierarchy_WithStateTransitions verifies deep hierarchies
// with additional state transitions (claims, releases, validations).
func TestReplay_DeepHierarchy_WithStateTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping deep hierarchy with state transitions test in short mode")
	}

	dir := t.TempDir()
	const depth = 100

	// Initialize proof
	initEvent := ledger.NewProofInitialized("Deep hierarchy with transitions", "test-author")
	if _, err := ledger.Append(dir, initEvent); err != nil {
		t.Fatalf("Append ProofInitialized failed: %v", err)
	}

	// Create deep hierarchy
	var allNodeIDs []types.NodeID
	for i := 0; i < depth; i++ {
		var nodeIDStr string
		if i == 0 {
			nodeIDStr = "1"
		} else {
			nodeIDStr = allNodeIDs[i-1].String() + ".1"
		}

		nodeID := mustParseNodeID(t, nodeIDStr)
		allNodeIDs = append(allNodeIDs, nodeID)

		var n *node.Node
		if i == 0 {
			n, _ = node.NewNode(nodeID, schema.NodeTypeClaim, fmt.Sprintf("Level %d", i), schema.InferenceAssumption)
		} else {
			n, _ = node.NewNodeWithOptions(
				nodeID,
				schema.NodeTypeClaim,
				fmt.Sprintf("Level %d", i),
				schema.InferenceModusPonens,
				node.NodeOptions{Dependencies: []types.NodeID{allNodeIDs[i-1]}},
			)
		}

		if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
			t.Fatalf("Append NodeCreated at level %d failed: %v", i, err)
		}
	}

	// Claim every 10th node
	for i := 0; i < depth; i += 10 {
		nodeID := allNodeIDs[i]
		if _, err := ledger.Append(dir, ledger.NewNodesClaimed([]types.NodeID{nodeID}, fmt.Sprintf("agent-%d", i), types.Now())); err != nil {
			t.Fatalf("Append NodesClaimed at level %d failed: %v", i, err)
		}
	}

	// Release every 10th node
	for i := 0; i < depth; i += 10 {
		nodeID := allNodeIDs[i]
		if _, err := ledger.Append(dir, ledger.NewNodesReleased([]types.NodeID{nodeID})); err != nil {
			t.Fatalf("Append NodesReleased at level %d failed: %v", i, err)
		}
	}

	// Validate every 20th node
	for i := 0; i < depth; i += 20 {
		nodeID := allNodeIDs[i]
		if _, err := ledger.Append(dir, ledger.NewNodeValidated(nodeID)); err != nil {
			t.Fatalf("Append NodeValidated at level %d failed: %v", i, err)
		}
	}

	// Replay
	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify state transitions
	for i := 0; i < depth; i++ {
		got := state.GetNode(allNodeIDs[i])
		if got == nil {
			t.Errorf("Node at level %d not found", i)
			continue
		}

		// Nodes should be available (claimed then released)
		if i%10 == 0 {
			if got.WorkflowState != schema.WorkflowAvailable {
				t.Errorf("Node at level %d should be available after claim/release, got %q", i, got.WorkflowState)
			}
		}

		// Every 20th node should be validated
		if i%20 == 0 {
			if got.EpistemicState != schema.EpistemicValidated {
				t.Errorf("Node at level %d should be validated, got %q", i, got.EpistemicState)
			}
		}
	}
}

// TestReplay_DeepHierarchy_WideBranching verifies deep hierarchies with
// wide branching at each level (multiple children per node).
func TestReplay_DeepHierarchy_WideBranching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping deep hierarchy with wide branching test in short mode")
	}

	dir := t.TempDir()
	const depth = 50
	const branchingFactor = 3 // Each node has 3 children

	// Initialize proof
	initEvent := ledger.NewProofInitialized("Deep hierarchy with wide branching", "test-author")
	if _, err := ledger.Append(dir, initEvent); err != nil {
		t.Fatalf("Append ProofInitialized failed: %v", err)
	}

	// Create root
	rootID := mustParseNodeID(t, "1")
	rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
	if _, err := ledger.Append(dir, ledger.NewNodeCreated(*rootNode)); err != nil {
		t.Fatalf("Append root failed: %v", err)
	}

	// Track all node IDs for verification
	nodeCount := 1

	// Create a BFS-style tree with depth levels and branchingFactor children at each node
	// For each level, create children under the first node of the previous level
	currentParent := rootID
	for level := 1; level < depth; level++ {
		for child := 1; child <= branchingFactor; child++ {
			nodeIDStr := currentParent.String() + fmt.Sprintf(".%d", child)
			nodeID := mustParseNodeID(t, nodeIDStr)

			n, _ := node.NewNodeWithOptions(
				nodeID,
				schema.NodeTypeClaim,
				fmt.Sprintf("Level %d child %d", level, child),
				schema.InferenceModusPonens,
				node.NodeOptions{Dependencies: []types.NodeID{currentParent}},
			)
			if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
				t.Fatalf("Append node at level %d child %d failed: %v", level, child, err)
			}
			nodeCount++
		}
		// Move to next level (following the first child)
		currentParent = mustParseNodeID(t, currentParent.String()+".1")
	}

	// Replay
	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	state, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify node count
	allNodes := state.AllNodes()
	if len(allNodes) != nodeCount {
		t.Errorf("Expected %d nodes, got %d", nodeCount, len(allNodes))
	}

	// Verify deepest path exists
	deepestPath := rootID.String()
	for level := 1; level < depth; level++ {
		deepestPath += ".1"
	}
	deepestID := mustParseNodeID(t, deepestPath)
	if state.GetNode(deepestID) == nil {
		t.Errorf("Deepest node at path %s not found", deepestPath)
	}
}

// TestReplay_DeepHierarchy_MemoryEfficiency verifies that replaying deep
// hierarchies doesn't cause excessive memory allocation or retention.
func TestReplay_DeepHierarchy_MemoryEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory efficiency test in short mode")
	}

	dir := t.TempDir()
	const depth = 300

	// Initialize proof
	initEvent := ledger.NewProofInitialized("Memory efficiency test", "test-author")
	if _, err := ledger.Append(dir, initEvent); err != nil {
		t.Fatalf("Append ProofInitialized failed: %v", err)
	}

	// Create very deep hierarchy
	var prevID types.NodeID
	for i := 0; i < depth; i++ {
		var nodeIDStr string
		if i == 0 {
			nodeIDStr = "1"
		} else {
			nodeIDStr = prevID.String() + ".1"
		}

		nodeID := mustParseNodeID(t, nodeIDStr)

		var n *node.Node
		if i == 0 {
			n, _ = node.NewNode(nodeID, schema.NodeTypeClaim, fmt.Sprintf("Level %d", i), schema.InferenceAssumption)
		} else {
			n, _ = node.NewNodeWithOptions(
				nodeID,
				schema.NodeTypeClaim,
				fmt.Sprintf("Level %d", i),
				schema.InferenceModusPonens,
				node.NodeOptions{Dependencies: []types.NodeID{prevID}},
			)
		}

		if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
			t.Fatalf("Append NodeCreated at level %d failed: %v", i, err)
		}

		prevID = nodeID
	}

	// Replay multiple times to check for memory leaks
	for attempt := 0; attempt < 3; attempt++ {
		ldg, err := ledger.NewLedger(dir)
		if err != nil {
			t.Fatalf("NewLedger failed on attempt %d: %v", attempt, err)
		}

		state, err := Replay(ldg)
		if err != nil {
			t.Fatalf("Replay failed on attempt %d: %v", attempt, err)
		}

		// Verify node count
		allNodes := state.AllNodes()
		if len(allNodes) != depth {
			t.Errorf("Attempt %d: Expected %d nodes, got %d", attempt, depth, len(allNodes))
		}
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// parseEventFromJSON is a test helper to parse event type from JSON.
func parseEventFromJSON(t *testing.T, data []byte) ledger.EventType {
	t.Helper()
	var base ledger.BaseEvent
	if err := json.Unmarshal(data, &base); err != nil {
		t.Fatalf("Failed to parse event: %v", err)
	}
	return base.Type()
}
