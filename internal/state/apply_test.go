//go:build integration

// Package state provides derived state from replaying ledger events.
package state

import (
	"testing"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestApplyProofInitialized verifies that ProofInitialized event sets up initial state.
func TestApplyProofInitialized(t *testing.T) {
	s := NewState()

	event := ledger.NewProofInitialized("Prove that 1+1=2", "test-author")

	err := Apply(s, event)
	if err != nil {
		t.Fatalf("Apply ProofInitialized failed: %v", err)
	}

	// State should remain valid (no error) after initialization
	// The conjecture and author are stored in the event, not necessarily in state
	// This test verifies the event is accepted without error
}

// TestApplyNodeCreated verifies that NodeCreated event adds node to state.
func TestApplyNodeCreated(t *testing.T) {
	s := NewState()

	// Create a test node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}

	event := ledger.NewNodeCreated(*n)

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodeCreated failed: %v", err)
	}

	// Verify the node was added to state
	got := s.GetNode(nodeID)
	if got == nil {
		t.Fatal("Node was not added to state after NodeCreated event")
	}

	if got.Statement != "Test claim statement" {
		t.Errorf("Node statement mismatch: got %q, want %q", got.Statement, "Test claim statement")
	}

	if got.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("Node workflow state mismatch: got %q, want %q", got.WorkflowState, schema.WorkflowAvailable)
	}
}

// TestApplyNodeCreatedMultiple verifies that multiple NodeCreated events add all nodes.
func TestApplyNodeCreatedMultiple(t *testing.T) {
	s := NewState()

	nodeIDs := []string{"1", "1.1", "1.2", "1.1.1"}

	for _, idStr := range nodeIDs {
		nodeID := mustParseNodeID(t, idStr)
		n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("Failed to create node %s: %v", idStr, err)
		}

		event := ledger.NewNodeCreated(*n)
		err = Apply(s, event)
		if err != nil {
			t.Fatalf("Apply NodeCreated for %s failed: %v", idStr, err)
		}
	}

	// Verify all nodes were added
	for _, idStr := range nodeIDs {
		nodeID := mustParseNodeID(t, idStr)
		got := s.GetNode(nodeID)
		if got == nil {
			t.Errorf("Node %s was not added to state", idStr)
		}
	}
}

// TestApplyNodesClaimed verifies that NodesClaimed event updates node workflow state.
func TestApplyNodesClaimed(t *testing.T) {
	s := NewState()

	// First, add a node to state
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Verify initial state is available
	if got := s.GetNode(nodeID); got.WorkflowState != schema.WorkflowAvailable {
		t.Fatalf("Initial workflow state is not available: got %q", got.WorkflowState)
	}

	// Apply NodesClaimed event
	timeout := types.Now()
	event := ledger.NewNodesClaimed([]types.NodeID{nodeID}, "agent-123", timeout)

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodesClaimed failed: %v", err)
	}

	// Verify workflow state changed to claimed
	got := s.GetNode(nodeID)
	if got.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("Workflow state after claim: got %q, want %q", got.WorkflowState, schema.WorkflowClaimed)
	}

	// Verify claim info is set
	if got.ClaimedBy != "agent-123" {
		t.Errorf("ClaimedBy mismatch: got %q, want %q", got.ClaimedBy, "agent-123")
	}
}

// TestApplyNodesClaimedMultiple verifies that NodesClaimed event can claim multiple nodes.
func TestApplyNodesClaimedMultiple(t *testing.T) {
	s := NewState()

	// Add multiple nodes to state
	nodeIDs := []types.NodeID{
		mustParseNodeID(t, "1"),
		mustParseNodeID(t, "1.1"),
		mustParseNodeID(t, "1.2"),
	}

	for _, nodeID := range nodeIDs {
		n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("Failed to create node: %v", err)
		}
		s.AddNode(n)
	}

	// Apply NodesClaimed event for all nodes
	timeout := types.Now()
	event := ledger.NewNodesClaimed(nodeIDs, "agent-456", timeout)

	err := Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodesClaimed failed: %v", err)
	}

	// Verify all nodes are claimed
	for _, nodeID := range nodeIDs {
		got := s.GetNode(nodeID)
		if got.WorkflowState != schema.WorkflowClaimed {
			t.Errorf("Node %s workflow state: got %q, want %q", nodeID.String(), got.WorkflowState, schema.WorkflowClaimed)
		}
		if got.ClaimedBy != "agent-456" {
			t.Errorf("Node %s ClaimedBy: got %q, want %q", nodeID.String(), got.ClaimedBy, "agent-456")
		}
	}
}

// TestApplyNodesReleased verifies that NodesReleased event clears claim.
func TestApplyNodesReleased(t *testing.T) {
	s := NewState()

	// Add a claimed node to state
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	n.WorkflowState = schema.WorkflowClaimed
	n.ClaimedBy = "agent-123"
	n.ClaimedAt = types.Now()
	s.AddNode(n)

	// Apply NodesReleased event
	event := ledger.NewNodesReleased([]types.NodeID{nodeID})

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodesReleased failed: %v", err)
	}

	// Verify workflow state changed to available
	got := s.GetNode(nodeID)
	if got.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("Workflow state after release: got %q, want %q", got.WorkflowState, schema.WorkflowAvailable)
	}

	// Verify claim info is cleared
	if got.ClaimedBy != "" {
		t.Errorf("ClaimedBy should be cleared: got %q", got.ClaimedBy)
	}
}

// TestApplyNodesReleasedMultiple verifies that NodesReleased event can release multiple nodes.
func TestApplyNodesReleasedMultiple(t *testing.T) {
	s := NewState()

	// Add multiple claimed nodes
	nodeIDs := []types.NodeID{
		mustParseNodeID(t, "1"),
		mustParseNodeID(t, "1.1"),
	}

	for _, nodeID := range nodeIDs {
		n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("Failed to create node: %v", err)
		}
		n.WorkflowState = schema.WorkflowClaimed
		n.ClaimedBy = "agent-123"
		s.AddNode(n)
	}

	// Apply NodesReleased event
	event := ledger.NewNodesReleased(nodeIDs)

	err := Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodesReleased failed: %v", err)
	}

	// Verify all nodes are released
	for _, nodeID := range nodeIDs {
		got := s.GetNode(nodeID)
		if got.WorkflowState != schema.WorkflowAvailable {
			t.Errorf("Node %s workflow state: got %q, want %q", nodeID.String(), got.WorkflowState, schema.WorkflowAvailable)
		}
		if got.ClaimedBy != "" {
			t.Errorf("Node %s ClaimedBy should be cleared: got %q", nodeID.String(), got.ClaimedBy)
		}
	}
}

// TestApplyNodeValidated verifies that NodeValidated event updates epistemic state.
func TestApplyNodeValidated(t *testing.T) {
	s := NewState()

	// Add a pending node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Apply NodeValidated event
	event := ledger.NewNodeValidated(nodeID)

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodeValidated failed: %v", err)
	}

	// Verify epistemic state changed to validated
	got := s.GetNode(nodeID)
	if got.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Epistemic state after validation: got %q, want %q", got.EpistemicState, schema.EpistemicValidated)
	}
}

// TestApplyNodeAdmitted verifies that NodeAdmitted event updates epistemic state.
func TestApplyNodeAdmitted(t *testing.T) {
	s := NewState()

	// Add a pending node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Apply NodeAdmitted event
	event := ledger.NewNodeAdmitted(nodeID)

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodeAdmitted failed: %v", err)
	}

	// Verify epistemic state changed to admitted
	got := s.GetNode(nodeID)
	if got.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("Epistemic state after admission: got %q, want %q", got.EpistemicState, schema.EpistemicAdmitted)
	}
}

// TestApplyNodeRefuted verifies that NodeRefuted event updates epistemic state.
func TestApplyNodeRefuted(t *testing.T) {
	s := NewState()

	// Add a pending node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Apply NodeRefuted event
	event := ledger.NewNodeRefuted(nodeID)

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodeRefuted failed: %v", err)
	}

	// Verify epistemic state changed to refuted
	got := s.GetNode(nodeID)
	if got.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("Epistemic state after refutation: got %q, want %q", got.EpistemicState, schema.EpistemicRefuted)
	}
}

// TestApplyNodeArchived verifies that NodeArchived event updates epistemic state.
func TestApplyNodeArchived(t *testing.T) {
	s := NewState()

	// Add a pending node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Apply NodeArchived event
	event := ledger.NewNodeArchived(nodeID)

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodeArchived failed: %v", err)
	}

	// Verify epistemic state changed to archived
	got := s.GetNode(nodeID)
	if got.EpistemicState != schema.EpistemicArchived {
		t.Errorf("Epistemic state after archival: got %q, want %q", got.EpistemicState, schema.EpistemicArchived)
	}
}

// TestApplyDefAdded verifies that DefAdded event adds definition to state.
func TestApplyDefAdded(t *testing.T) {
	s := NewState()

	// Create a definition
	def := ledger.Definition{
		ID:         "def-001",
		Name:       "TestDef",
		Definition: "A test definition content",
		Created:    types.Now(),
	}

	event := ledger.NewDefAdded(def)

	err := Apply(s, event)
	if err != nil {
		t.Fatalf("Apply DefAdded failed: %v", err)
	}

	// Verify the definition was added to state
	got := s.GetDefinition("def-001")
	if got == nil {
		t.Fatal("Definition was not added to state after DefAdded event")
	}

	if got.Name != "TestDef" {
		t.Errorf("Definition name mismatch: got %q, want %q", got.Name, "TestDef")
	}

	if got.Content != "A test definition content" {
		t.Errorf("Definition content mismatch: got %q, want %q", got.Content, "A test definition content")
	}
}

// TestApplyDefAddedMultiple verifies that multiple DefAdded events add all definitions.
func TestApplyDefAddedMultiple(t *testing.T) {
	s := NewState()

	defs := []ledger.Definition{
		{ID: "def-001", Name: "Def1", Definition: "Content 1", Created: types.Now()},
		{ID: "def-002", Name: "Def2", Definition: "Content 2", Created: types.Now()},
		{ID: "def-003", Name: "Def3", Definition: "Content 3", Created: types.Now()},
	}

	for _, def := range defs {
		event := ledger.NewDefAdded(def)
		err := Apply(s, event)
		if err != nil {
			t.Fatalf("Apply DefAdded for %s failed: %v", def.ID, err)
		}
	}

	// Verify all definitions were added
	for _, def := range defs {
		got := s.GetDefinition(def.ID)
		if got == nil {
			t.Errorf("Definition %s was not added to state", def.ID)
		}
	}
}

// TestApplyLemmaExtracted verifies that LemmaExtracted event adds lemma to state.
func TestApplyLemmaExtracted(t *testing.T) {
	s := NewState()

	// Create a lemma
	sourceNodeID := mustParseNodeID(t, "1.1")
	lemma := ledger.Lemma{
		ID:        "lem-001",
		Statement: "A useful lemma statement",
		NodeID:    sourceNodeID,
		Created:   types.Now(),
	}

	event := ledger.NewLemmaExtracted(lemma)

	err := Apply(s, event)
	if err != nil {
		t.Fatalf("Apply LemmaExtracted failed: %v", err)
	}

	// Verify the lemma was added to state
	got := s.GetLemma("lem-001")
	if got == nil {
		t.Fatal("Lemma was not added to state after LemmaExtracted event")
	}

	if got.Statement != "A useful lemma statement" {
		t.Errorf("Lemma statement mismatch: got %q, want %q", got.Statement, "A useful lemma statement")
	}

	if got.SourceNodeID.String() != sourceNodeID.String() {
		t.Errorf("Lemma source node ID mismatch: got %q, want %q", got.SourceNodeID.String(), sourceNodeID.String())
	}
}

// TestApplyLemmaExtractedMultiple verifies that multiple LemmaExtracted events add all lemmas.
func TestApplyLemmaExtractedMultiple(t *testing.T) {
	s := NewState()

	sourceNodeID := mustParseNodeID(t, "1")
	lemmas := []ledger.Lemma{
		{ID: "lem-001", Statement: "Lemma 1", NodeID: sourceNodeID, Created: types.Now()},
		{ID: "lem-002", Statement: "Lemma 2", NodeID: sourceNodeID, Created: types.Now()},
	}

	for _, lemma := range lemmas {
		event := ledger.NewLemmaExtracted(lemma)
		err := Apply(s, event)
		if err != nil {
			t.Fatalf("Apply LemmaExtracted for %s failed: %v", lemma.ID, err)
		}
	}

	// Verify all lemmas were added
	for _, lemma := range lemmas {
		got := s.GetLemma(lemma.ID)
		if got == nil {
			t.Errorf("Lemma %s was not added to state", lemma.ID)
		}
	}
}

// TestApplyTaintRecomputed verifies that TaintRecomputed event updates node taint state.
func TestApplyTaintRecomputed(t *testing.T) {
	s := NewState()

	// Add a node with unresolved taint
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Verify initial taint state
	if got := s.GetNode(nodeID); got.TaintState != node.TaintUnresolved {
		t.Fatalf("Initial taint state is not unresolved: got %q", got.TaintState)
	}

	// Apply TaintRecomputed event
	event := ledger.NewTaintRecomputed(nodeID, node.TaintClean)

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply TaintRecomputed failed: %v", err)
	}

	// Verify taint state changed
	got := s.GetNode(nodeID)
	if got.TaintState != node.TaintClean {
		t.Errorf("Taint state after recompute: got %q, want %q", got.TaintState, node.TaintClean)
	}
}

// TestApplyTaintRecomputedToSelfAdmitted verifies TaintRecomputed with self_admitted taint.
func TestApplyTaintRecomputedToSelfAdmitted(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Apply TaintRecomputed event with self_admitted
	event := ledger.NewTaintRecomputed(nodeID, node.TaintSelfAdmitted)

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply TaintRecomputed failed: %v", err)
	}

	got := s.GetNode(nodeID)
	if got.TaintState != node.TaintSelfAdmitted {
		t.Errorf("Taint state: got %q, want %q", got.TaintState, node.TaintSelfAdmitted)
	}
}

// TestApplyTaintRecomputedToTainted verifies TaintRecomputed with tainted taint.
func TestApplyTaintRecomputedToTainted(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Apply TaintRecomputed event with tainted
	event := ledger.NewTaintRecomputed(nodeID, node.TaintTainted)

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply TaintRecomputed failed: %v", err)
	}

	got := s.GetNode(nodeID)
	if got.TaintState != node.TaintTainted {
		t.Errorf("Taint state: got %q, want %q", got.TaintState, node.TaintTainted)
	}
}

// TestApplyChallengeRaised verifies that ChallengeRaised event is handled.
func TestApplyChallengeRaised(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")

	// Apply ChallengeRaised event
	event := ledger.NewChallengeRaised("chal-001", nodeID, "statement", "This is incorrect")

	err := Apply(s, event)
	if err != nil {
		t.Fatalf("Apply ChallengeRaised failed: %v", err)
	}

	// The event should be accepted without error
	// Challenge tracking may be implemented separately
}

// TestApplyChallengeResolved verifies that ChallengeResolved event is handled.
func TestApplyChallengeResolved(t *testing.T) {
	s := NewState()

	// Apply ChallengeResolved event
	event := ledger.NewChallengeResolved("chal-001")

	err := Apply(s, event)
	if err != nil {
		t.Fatalf("Apply ChallengeResolved failed: %v", err)
	}

	// The event should be accepted without error
}

// TestApplyChallengeWithdrawn verifies that ChallengeWithdrawn event is handled.
func TestApplyChallengeWithdrawn(t *testing.T) {
	s := NewState()

	// Apply ChallengeWithdrawn event
	event := ledger.NewChallengeWithdrawn("chal-001")

	err := Apply(s, event)
	if err != nil {
		t.Fatalf("Apply ChallengeWithdrawn failed: %v", err)
	}

	// The event should be accepted without error
}

// TestApplyUnknownEventType verifies that unknown event type returns error.
func TestApplyUnknownEventType(t *testing.T) {
	s := NewState()

	// Create an unknown event type using the base event
	unknownEvent := &unknownTestEvent{
		BaseEvent: ledger.BaseEvent{
			EventType: ledger.EventType("unknown_event"),
			EventTime: types.Now(),
		},
	}

	err := Apply(s, unknownEvent)
	if err == nil {
		t.Fatal("Apply should return error for unknown event type")
	}

	// Verify error message mentions unknown event type
	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

// unknownTestEvent is a test event type that implements ledger.Event but is not recognized.
type unknownTestEvent struct {
	ledger.BaseEvent
}

// TestApplyEventSequence verifies a sequence of events can be applied in order.
func TestApplyEventSequence(t *testing.T) {
	s := NewState()

	// 1. Initialize proof
	initEvent := ledger.NewProofInitialized("Test conjecture", "author")
	if err := Apply(s, initEvent); err != nil {
		t.Fatalf("Apply ProofInitialized failed: %v", err)
	}

	// 2. Create root node
	rootID := mustParseNodeID(t, "1")
	rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
	createRootEvent := ledger.NewNodeCreated(*rootNode)
	if err := Apply(s, createRootEvent); err != nil {
		t.Fatalf("Apply NodeCreated for root failed: %v", err)
	}

	// 3. Create child node
	childID := mustParseNodeID(t, "1.1")
	childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
	createChildEvent := ledger.NewNodeCreated(*childNode)
	if err := Apply(s, createChildEvent); err != nil {
		t.Fatalf("Apply NodeCreated for child failed: %v", err)
	}

	// 4. Claim the child node
	claimEvent := ledger.NewNodesClaimed([]types.NodeID{childID}, "agent-1", types.Now())
	if err := Apply(s, claimEvent); err != nil {
		t.Fatalf("Apply NodesClaimed failed: %v", err)
	}

	// 5. Release the child node
	releaseEvent := ledger.NewNodesReleased([]types.NodeID{childID})
	if err := Apply(s, releaseEvent); err != nil {
		t.Fatalf("Apply NodesReleased failed: %v", err)
	}

	// 6. Validate the child node
	validateEvent := ledger.NewNodeValidated(childID)
	if err := Apply(s, validateEvent); err != nil {
		t.Fatalf("Apply NodeValidated failed: %v", err)
	}

	// Verify final state
	gotRoot := s.GetNode(rootID)
	if gotRoot == nil {
		t.Fatal("Root node not found in state")
	}

	gotChild := s.GetNode(childID)
	if gotChild == nil {
		t.Fatal("Child node not found in state")
	}

	if gotChild.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("Child workflow state: got %q, want %q", gotChild.WorkflowState, schema.WorkflowAvailable)
	}

	if gotChild.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Child epistemic state: got %q, want %q", gotChild.EpistemicState, schema.EpistemicValidated)
	}
}

// TestApplyNodeCreatedWithDependencies verifies NodeCreated preserves dependencies.
func TestApplyNodeCreatedWithDependencies(t *testing.T) {
	s := NewState()

	// Create root first
	rootID := mustParseNodeID(t, "1")
	rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
	rootEvent := ledger.NewNodeCreated(*rootNode)
	if err := Apply(s, rootEvent); err != nil {
		t.Fatalf("Apply root NodeCreated failed: %v", err)
	}

	// Create child with dependency on root
	childID := mustParseNodeID(t, "1.1")
	childNode, _ := node.NewNodeWithOptions(
		childID,
		schema.NodeTypeClaim,
		"Child depending on root",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Dependencies: []types.NodeID{rootID},
		},
	)
	childEvent := ledger.NewNodeCreated(*childNode)
	if err := Apply(s, childEvent); err != nil {
		t.Fatalf("Apply child NodeCreated failed: %v", err)
	}

	// Verify dependencies are preserved
	got := s.GetNode(childID)
	if len(got.Dependencies) != 1 {
		t.Fatalf("Dependencies count: got %d, want 1", len(got.Dependencies))
	}

	if got.Dependencies[0].String() != rootID.String() {
		t.Errorf("Dependency mismatch: got %q, want %q", got.Dependencies[0].String(), rootID.String())
	}
}

// TestApplyNilState verifies that Apply handles nil state gracefully.
func TestApplyNilState(t *testing.T) {
	event := ledger.NewProofInitialized("Test", "author")

	err := Apply(nil, event)
	if err == nil {
		t.Fatal("Apply should return error for nil state")
	}
}

// TestApplyNilEvent verifies that Apply handles nil event gracefully.
func TestApplyNilEvent(t *testing.T) {
	s := NewState()

	err := Apply(s, nil)
	if err == nil {
		t.Fatal("Apply should return error for nil event")
	}
}

// TestApplyNodesClaimedNonExistentNode verifies behavior when claiming non-existent node.
func TestApplyNodesClaimedNonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewNodesClaimed([]types.NodeID{nodeID}, "agent-1", types.Now())

	// This should either error or be a no-op - implementation dependent
	// The key is it should not panic
	_ = Apply(s, event)
}

// TestApplyNodesReleasedNonExistentNode verifies behavior when releasing non-existent node.
func TestApplyNodesReleasedNonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewNodesReleased([]types.NodeID{nodeID})

	// This should either error or be a no-op - implementation dependent
	// The key is it should not panic
	_ = Apply(s, event)
}

// TestApplyEpistemicStateChangeOnNonExistentNode verifies behavior when changing epistemic state on non-existent node.
func TestApplyEpistemicStateChangeOnNonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewNodeValidated(nodeID)

	// This should either error or be a no-op - implementation dependent
	// The key is it should not panic
	_ = Apply(s, event)
}

// TestApplyTaintRecomputedOnNonExistentNode verifies behavior when recomputing taint on non-existent node.
func TestApplyTaintRecomputedOnNonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewTaintRecomputed(nodeID, node.TaintClean)

	// This should either error or be a no-op - implementation dependent
	// The key is it should not panic
	_ = Apply(s, event)
}
