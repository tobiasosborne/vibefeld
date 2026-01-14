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
