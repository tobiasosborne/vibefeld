// Package state provides derived state from replaying ledger events.
package state

import (
	"strings"
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

	// Verify the challenge was added to state
	c := s.GetChallenge("chal-001")
	if c == nil {
		t.Fatal("Challenge was not added to state")
	}
	if c.ID != "chal-001" {
		t.Errorf("Challenge ID: got %q, want %q", c.ID, "chal-001")
	}
	if c.NodeID.String() != "1" {
		t.Errorf("Challenge NodeID: got %q, want %q", c.NodeID.String(), "1")
	}
	if c.Target != "statement" {
		t.Errorf("Challenge Target: got %q, want %q", c.Target, "statement")
	}
	if c.Reason != "This is incorrect" {
		t.Errorf("Challenge Reason: got %q, want %q", c.Reason, "This is incorrect")
	}
	if c.Status != "open" {
		t.Errorf("Challenge Status: got %q, want %q", c.Status, "open")
	}
}

// TestApplyChallengeRaised_InvalidSeverity verifies that invalid severity is rejected.
func TestApplyChallengeRaised_InvalidSeverity(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	testCases := []struct {
		name     string
		severity string
	}{
		{"typo", "majr"},
		{"uppercase", "MAJOR"},
		{"numeric", "1"},
		{"arbitrary", "urgent"},
		{"empty spaces", "  "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := ledger.NewChallengeRaisedWithSeverity(
				"chal-"+tc.name, nodeID, "statement", "Test challenge", tc.severity, "verifier-1")
			err := Apply(s, event)
			if err == nil {
				t.Errorf("Apply should reject invalid severity %q", tc.severity)
			}
		})
	}
}

// TestApplyChallengeRaised_ValidSeverities verifies that valid severities are accepted.
func TestApplyChallengeRaised_ValidSeverities(t *testing.T) {
	validSeverities := []string{"critical", "major", "minor", "note"}

	for _, sev := range validSeverities {
		t.Run(sev, func(t *testing.T) {
			s := NewState()
			nodeID := mustParseNodeID(t, "1")
			event := ledger.NewChallengeRaisedWithSeverity(
				"chal-"+sev, nodeID, "statement", "Test challenge", sev, "verifier-1")
			err := Apply(s, event)
			if err != nil {
				t.Errorf("Apply should accept valid severity %q: %v", sev, err)
			}
			c := s.GetChallenge("chal-" + sev)
			if c == nil {
				t.Fatal("Challenge was not added to state")
			}
			if c.Severity != sev {
				t.Errorf("Severity: got %q, want %q", c.Severity, sev)
			}
		})
	}
}

// TestApplyChallengeResolved verifies that ChallengeResolved event is handled.
func TestApplyChallengeResolved(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")

	// First raise a challenge
	raiseEvent := ledger.NewChallengeRaised("chal-001", nodeID, "statement", "This is incorrect")
	err := Apply(s, raiseEvent)
	if err != nil {
		t.Fatalf("Apply ChallengeRaised failed: %v", err)
	}

	// Now resolve the challenge
	resolveEvent := ledger.NewChallengeResolved("chal-001")
	err = Apply(s, resolveEvent)
	if err != nil {
		t.Fatalf("Apply ChallengeResolved failed: %v", err)
	}

	// Verify the challenge status is resolved
	c := s.GetChallenge("chal-001")
	if c == nil {
		t.Fatal("Challenge not found in state")
	}
	if c.Status != "resolved" {
		t.Errorf("Challenge Status: got %q, want %q", c.Status, "resolved")
	}
}

// TestApplyChallengeResolved_NotFound verifies error when resolving non-existent challenge.
func TestApplyChallengeResolved_NotFound(t *testing.T) {
	s := NewState()

	// Try to resolve a challenge that doesn't exist
	event := ledger.NewChallengeResolved("chal-nonexistent")
	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply ChallengeResolved should fail for non-existent challenge")
	}
}

// TestApplyChallengeWithdrawn verifies that ChallengeWithdrawn event is handled.
func TestApplyChallengeWithdrawn(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")

	// First raise a challenge
	raiseEvent := ledger.NewChallengeRaised("chal-001", nodeID, "statement", "This is incorrect")
	err := Apply(s, raiseEvent)
	if err != nil {
		t.Fatalf("Apply ChallengeRaised failed: %v", err)
	}

	// Now withdraw the challenge
	withdrawEvent := ledger.NewChallengeWithdrawn("chal-001")
	err = Apply(s, withdrawEvent)
	if err != nil {
		t.Fatalf("Apply ChallengeWithdrawn failed: %v", err)
	}

	// Verify the challenge status is withdrawn
	c := s.GetChallenge("chal-001")
	if c == nil {
		t.Fatal("Challenge not found in state")
	}
	if c.Status != "withdrawn" {
		t.Errorf("Challenge Status: got %q, want %q", c.Status, "withdrawn")
	}
}

// TestApplyChallengeWithdrawn_NotFound verifies error when withdrawing non-existent challenge.
func TestApplyChallengeWithdrawn_NotFound(t *testing.T) {
	s := NewState()

	// Try to withdraw a challenge that doesn't exist
	event := ledger.NewChallengeWithdrawn("chal-nonexistent")
	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply ChallengeWithdrawn should fail for non-existent challenge")
	}
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

// TestApplyNodesClaimedNonExistentNode verifies that claiming a non-existent node returns an error.
func TestApplyNodesClaimedNonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewNodesClaimed([]types.NodeID{nodeID}, "agent-1", types.Now())

	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when claiming non-existent node")
	}

	// Verify error message contains the node ID
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("Error message should contain node ID: got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error message should mention 'not found': got %q", err.Error())
	}
}

// TestApplyNodesReleasedNonExistentNode verifies that releasing a non-existent node returns an error.
func TestApplyNodesReleasedNonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewNodesReleased([]types.NodeID{nodeID})

	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when releasing non-existent node")
	}

	// Verify error message contains the node ID
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("Error message should contain node ID: got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error message should mention 'not found': got %q", err.Error())
	}
}

// TestApplyNodeValidatedNonExistentNode verifies that validating a non-existent node returns an error.
func TestApplyNodeValidatedNonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewNodeValidated(nodeID)

	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when validating non-existent node")
	}

	// Verify error message contains the node ID
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("Error message should contain node ID: got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error message should mention 'not found': got %q", err.Error())
	}
}

// TestApplyNodeAdmittedNonExistentNode verifies that admitting a non-existent node returns an error.
func TestApplyNodeAdmittedNonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewNodeAdmitted(nodeID)

	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when admitting non-existent node")
	}

	// Verify error message contains the node ID
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("Error message should contain node ID: got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error message should mention 'not found': got %q", err.Error())
	}
}

// TestApplyNodeRefutedNonExistentNode verifies that refuting a non-existent node returns an error.
func TestApplyNodeRefutedNonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewNodeRefuted(nodeID)

	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when refuting non-existent node")
	}

	// Verify error message contains the node ID
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("Error message should contain node ID: got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error message should mention 'not found': got %q", err.Error())
	}
}

// TestApplyNodeArchivedNonExistentNode verifies that archiving a non-existent node returns an error.
func TestApplyNodeArchivedNonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewNodeArchived(nodeID)

	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when archiving non-existent node")
	}

	// Verify error message contains the node ID
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("Error message should contain node ID: got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error message should mention 'not found': got %q", err.Error())
	}
}

// TestApplyTaintRecomputedOnNonExistentNode verifies that recomputing taint on a non-existent node returns an error.
func TestApplyTaintRecomputedOnNonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewTaintRecomputed(nodeID, node.TaintClean)

	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when recomputing taint on non-existent node")
	}

	// Verify error message contains the node ID
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("Error message should contain node ID: got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error message should mention 'not found': got %q", err.Error())
	}
}

// TestApplyNodesClaimedInvalidTransition verifies that claiming an already claimed node returns an error.
func TestApplyNodesClaimedInvalidTransition(t *testing.T) {
	s := NewState()

	// Add a node that is already claimed
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	n.WorkflowState = schema.WorkflowClaimed // Already claimed
	n.ClaimedBy = "other-agent"
	s.AddNode(n)

	// Try to claim an already claimed node
	event := ledger.NewNodesClaimed([]types.NodeID{nodeID}, "agent-123", types.Now())

	err = Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when claiming an already claimed node")
	}

	// Verify error mentions invalid workflow transition
	if !strings.Contains(err.Error(), "invalid workflow transition") {
		t.Errorf("Error should mention 'invalid workflow transition': got %q", err.Error())
	}
}

// TestApplyNodesClaimedFromBlocked verifies that claiming a blocked node returns an error.
func TestApplyNodesClaimedFromBlocked(t *testing.T) {
	s := NewState()

	// Add a blocked node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	n.WorkflowState = schema.WorkflowBlocked
	s.AddNode(n)

	// Try to claim a blocked node (not allowed - must become available first)
	event := ledger.NewNodesClaimed([]types.NodeID{nodeID}, "agent-123", types.Now())

	err = Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when claiming a blocked node")
	}

	if !strings.Contains(err.Error(), "invalid workflow transition") {
		t.Errorf("Error should mention 'invalid workflow transition': got %q", err.Error())
	}
}

// TestApplyNodesReleasedFromAvailable verifies that releasing an already available node returns an error.
func TestApplyNodesReleasedFromAvailable(t *testing.T) {
	s := NewState()

	// Add an available node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	// WorkflowState is already "available" by default
	s.AddNode(n)

	// Try to release an available node
	event := ledger.NewNodesReleased([]types.NodeID{nodeID})

	err = Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when releasing an already available node")
	}

	if !strings.Contains(err.Error(), "invalid workflow transition") {
		t.Errorf("Error should mention 'invalid workflow transition': got %q", err.Error())
	}
}

// TestApplyNodeValidatedAutoTaint verifies that validating a node auto-triggers taint computation.
func TestApplyNodeValidatedAutoTaint(t *testing.T) {
	s := NewState()

	// Add a pending node with unresolved taint
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Verify initial taint state is unresolved
	if got := s.GetNode(nodeID); got.TaintState != node.TaintUnresolved {
		t.Fatalf("Initial taint state should be unresolved: got %q", got.TaintState)
	}

	// Apply NodeValidated event
	event := ledger.NewNodeValidated(nodeID)
	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodeValidated failed: %v", err)
	}

	// Verify taint was auto-computed to clean (validated node with no tainted ancestors)
	got := s.GetNode(nodeID)
	if got.TaintState != node.TaintClean {
		t.Errorf("Taint state after validation should be clean: got %q", got.TaintState)
	}
}

// TestApplyNodeAdmittedAutoTaint verifies that admitting a node auto-triggers taint computation.
func TestApplyNodeAdmittedAutoTaint(t *testing.T) {
	s := NewState()

	// Add a pending node with unresolved taint
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Verify initial taint state is unresolved
	if got := s.GetNode(nodeID); got.TaintState != node.TaintUnresolved {
		t.Fatalf("Initial taint state should be unresolved: got %q", got.TaintState)
	}

	// Apply NodeAdmitted event
	event := ledger.NewNodeAdmitted(nodeID)
	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodeAdmitted failed: %v", err)
	}

	// Verify taint was auto-computed to self_admitted (admitted node introduces taint)
	got := s.GetNode(nodeID)
	if got.TaintState != node.TaintSelfAdmitted {
		t.Errorf("Taint state after admission should be self_admitted: got %q", got.TaintState)
	}
}

// TestApplyNodeValidatedPropagatesTaint verifies that validating a parent node propagates taint to children.
func TestApplyNodeValidatedPropagatesTaint(t *testing.T) {
	s := NewState()

	// Create parent node
	parentID := mustParseNodeID(t, "1")
	parent, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create parent node: %v", err)
	}
	s.AddNode(parent)

	// Create child node with validated epistemic state but unresolved ancestor
	childID := mustParseNodeID(t, "1.1")
	child, err := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("Failed to create child node: %v", err)
	}
	// Pre-set child to validated (but taint is unresolved because parent is unresolved)
	child.EpistemicState = schema.EpistemicValidated
	s.AddNode(child)

	// Validate parent - this should propagate taint to child
	event := ledger.NewNodeValidated(parentID)
	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodeValidated failed: %v", err)
	}

	// Check that parent taint is clean
	gotParent := s.GetNode(parentID)
	if gotParent.TaintState != node.TaintClean {
		t.Errorf("Parent taint state should be clean: got %q", gotParent.TaintState)
	}

	// Check that child taint was propagated to clean (since parent is now clean and child is validated)
	gotChild := s.GetNode(childID)
	if gotChild.TaintState != node.TaintClean {
		t.Errorf("Child taint state should be clean after parent validation: got %q", gotChild.TaintState)
	}
}

// TestApplyNodeAdmittedPropagatesTaint verifies that admitting a parent propagates taint to descendants.
func TestApplyNodeAdmittedPropagatesTaint(t *testing.T) {
	s := NewState()

	// Create parent node
	parentID := mustParseNodeID(t, "1")
	parent, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create parent node: %v", err)
	}
	s.AddNode(parent)

	// Create child node that is already validated
	childID := mustParseNodeID(t, "1.1")
	child, err := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("Failed to create child node: %v", err)
	}
	child.EpistemicState = schema.EpistemicValidated
	s.AddNode(child)

	// Admit parent - this should propagate taint to child
	event := ledger.NewNodeAdmitted(parentID)
	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodeAdmitted failed: %v", err)
	}

	// Check that parent taint is self_admitted
	gotParent := s.GetNode(parentID)
	if gotParent.TaintState != node.TaintSelfAdmitted {
		t.Errorf("Parent taint state should be self_admitted: got %q", gotParent.TaintState)
	}

	// Check that child taint was propagated to tainted (since parent is self_admitted)
	gotChild := s.GetNode(childID)
	if gotChild.TaintState != node.TaintTainted {
		t.Errorf("Child taint state should be tainted after parent admission: got %q", gotChild.TaintState)
	}
}

// TestApplyChallengeSuperseded verifies that ChallengeSuperseded event is handled.
func TestApplyChallengeSuperseded(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")

	// First raise a challenge
	raiseEvent := ledger.NewChallengeRaised("chal-001", nodeID, "statement", "This is incorrect")
	err := Apply(s, raiseEvent)
	if err != nil {
		t.Fatalf("Apply ChallengeRaised failed: %v", err)
	}

	// Now supersede the challenge
	supersededEvent := ledger.NewChallengeSuperseded("chal-001", nodeID)
	err = Apply(s, supersededEvent)
	if err != nil {
		t.Fatalf("Apply ChallengeSuperseded failed: %v", err)
	}

	// Verify the challenge status is superseded
	c := s.GetChallenge("chal-001")
	if c == nil {
		t.Fatal("Challenge not found in state")
	}
	if c.Status != "superseded" {
		t.Errorf("Challenge Status: got %q, want %q", c.Status, "superseded")
	}
}

// TestApplyChallengeSuperseded_NotFound verifies error when superseding non-existent challenge.
func TestApplyChallengeSuperseded_NotFound(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")

	// Try to supersede a challenge that doesn't exist
	event := ledger.NewChallengeSuperseded("chal-nonexistent", nodeID)
	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply ChallengeSuperseded should fail for non-existent challenge")
	}
}

// TestApplyNodeArchivedSupersedesOpenChallenges verifies that archiving a node auto-supersedes its open challenges.
func TestApplyNodeArchivedSupersedesOpenChallenges(t *testing.T) {
	s := NewState()

	// Add a pending node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Raise multiple challenges on the node
	raiseEvent1 := ledger.NewChallengeRaised("chal-001", nodeID, "statement", "Challenge 1")
	err = Apply(s, raiseEvent1)
	if err != nil {
		t.Fatalf("Apply ChallengeRaised 1 failed: %v", err)
	}

	raiseEvent2 := ledger.NewChallengeRaised("chal-002", nodeID, "inference", "Challenge 2")
	err = Apply(s, raiseEvent2)
	if err != nil {
		t.Fatalf("Apply ChallengeRaised 2 failed: %v", err)
	}

	// Also raise a challenge that is already resolved (should not be affected)
	raiseEvent3 := ledger.NewChallengeRaised("chal-003", nodeID, "statement", "Challenge 3")
	err = Apply(s, raiseEvent3)
	if err != nil {
		t.Fatalf("Apply ChallengeRaised 3 failed: %v", err)
	}
	resolveEvent := ledger.NewChallengeResolved("chal-003")
	err = Apply(s, resolveEvent)
	if err != nil {
		t.Fatalf("Apply ChallengeResolved failed: %v", err)
	}

	// Archive the node - this should auto-supersede open challenges
	archiveEvent := ledger.NewNodeArchived(nodeID)
	err = Apply(s, archiveEvent)
	if err != nil {
		t.Fatalf("Apply NodeArchived failed: %v", err)
	}

	// Verify the node is archived
	got := s.GetNode(nodeID)
	if got.EpistemicState != schema.EpistemicArchived {
		t.Errorf("Node epistemic state: got %q, want %q", got.EpistemicState, schema.EpistemicArchived)
	}

	// Verify the open challenges are superseded
	chal1 := s.GetChallenge("chal-001")
	if chal1.Status != "superseded" {
		t.Errorf("Challenge 1 Status: got %q, want %q", chal1.Status, "superseded")
	}

	chal2 := s.GetChallenge("chal-002")
	if chal2.Status != "superseded" {
		t.Errorf("Challenge 2 Status: got %q, want %q", chal2.Status, "superseded")
	}

	// Verify the resolved challenge remains resolved
	chal3 := s.GetChallenge("chal-003")
	if chal3.Status != "resolved" {
		t.Errorf("Challenge 3 Status should remain resolved: got %q, want %q", chal3.Status, "resolved")
	}
}

// TestApplyNodeRefutedSupersedesOpenChallenges verifies that refuting a node auto-supersedes its open challenges.
func TestApplyNodeRefutedSupersedesOpenChallenges(t *testing.T) {
	s := NewState()

	// Add a pending node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Raise a challenge on the node
	raiseEvent := ledger.NewChallengeRaised("chal-001", nodeID, "statement", "This is wrong")
	err = Apply(s, raiseEvent)
	if err != nil {
		t.Fatalf("Apply ChallengeRaised failed: %v", err)
	}

	// Also raise a challenge that is withdrawn (should not be affected)
	raiseEvent2 := ledger.NewChallengeRaised("chal-002", nodeID, "inference", "Another challenge")
	err = Apply(s, raiseEvent2)
	if err != nil {
		t.Fatalf("Apply ChallengeRaised 2 failed: %v", err)
	}
	withdrawEvent := ledger.NewChallengeWithdrawn("chal-002")
	err = Apply(s, withdrawEvent)
	if err != nil {
		t.Fatalf("Apply ChallengeWithdrawn failed: %v", err)
	}

	// Refute the node - this should auto-supersede open challenges
	refuteEvent := ledger.NewNodeRefuted(nodeID)
	err = Apply(s, refuteEvent)
	if err != nil {
		t.Fatalf("Apply NodeRefuted failed: %v", err)
	}

	// Verify the node is refuted
	got := s.GetNode(nodeID)
	if got.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("Node epistemic state: got %q, want %q", got.EpistemicState, schema.EpistemicRefuted)
	}

	// Verify the open challenge is superseded
	chal1 := s.GetChallenge("chal-001")
	if chal1.Status != "superseded" {
		t.Errorf("Challenge 1 Status: got %q, want %q", chal1.Status, "superseded")
	}

	// Verify the withdrawn challenge remains withdrawn
	chal2 := s.GetChallenge("chal-002")
	if chal2.Status != "withdrawn" {
		t.Errorf("Challenge 2 Status should remain withdrawn: got %q, want %q", chal2.Status, "withdrawn")
	}
}

// TestApplyNodeArchivedNoChallenges verifies that archiving a node with no challenges works fine.
func TestApplyNodeArchivedNoChallenges(t *testing.T) {
	s := NewState()

	// Add a pending node (no challenges)
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Archive the node - should work without any challenges to supersede
	archiveEvent := ledger.NewNodeArchived(nodeID)
	err = Apply(s, archiveEvent)
	if err != nil {
		t.Fatalf("Apply NodeArchived failed: %v", err)
	}

	// Verify the node is archived
	got := s.GetNode(nodeID)
	if got.EpistemicState != schema.EpistemicArchived {
		t.Errorf("Node epistemic state: got %q, want %q", got.EpistemicState, schema.EpistemicArchived)
	}
}

// TestApplyChallengeResolvedInvalidatesCache verifies that resolving a challenge invalidates
// the challengesByNode cache so that subsequent lookups reflect the new status.
func TestApplyChallengeResolvedInvalidatesCache(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a challenge
	raiseEvent := ledger.NewChallengeRaised("chal-001", nodeID, "statement", "This is incorrect")
	if err := Apply(s, raiseEvent); err != nil {
		t.Fatalf("Apply ChallengeRaised failed: %v", err)
	}

	// Access the cache to populate it
	_ = s.ChallengesByNodeID()

	// Verify the challenge is open in the state
	c := s.GetChallenge("chal-001")
	if c.Status != "open" {
		t.Fatalf("Challenge status should be open, got %q", c.Status)
	}

	// Get blocking challenges (should include our challenge since it defaults to major severity)
	blocking := s.GetBlockingChallengesForNode(nodeID)
	if len(blocking) != 1 {
		t.Fatalf("Expected 1 blocking challenge before resolve, got %d", len(blocking))
	}

	// Now resolve the challenge
	resolveEvent := ledger.NewChallengeResolved("chal-001")
	if err := Apply(s, resolveEvent); err != nil {
		t.Fatalf("Apply ChallengeResolved failed: %v", err)
	}

	// The cache should be invalidated, so GetBlockingChallengesForNode should return 0
	// (the challenge is now resolved, not open)
	blocking = s.GetBlockingChallengesForNode(nodeID)
	if len(blocking) != 0 {
		t.Errorf("Expected 0 blocking challenges after resolve, got %d (cache not invalidated?)", len(blocking))
	}
}

// TestApplyChallengeWithdrawnInvalidatesCache verifies that withdrawing a challenge invalidates
// the challengesByNode cache so that subsequent lookups reflect the new status.
func TestApplyChallengeWithdrawnInvalidatesCache(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a challenge
	raiseEvent := ledger.NewChallengeRaised("chal-001", nodeID, "statement", "This is incorrect")
	if err := Apply(s, raiseEvent); err != nil {
		t.Fatalf("Apply ChallengeRaised failed: %v", err)
	}

	// Access the cache to populate it
	_ = s.ChallengesByNodeID()

	// Get blocking challenges before withdrawal
	blocking := s.GetBlockingChallengesForNode(nodeID)
	if len(blocking) != 1 {
		t.Fatalf("Expected 1 blocking challenge before withdraw, got %d", len(blocking))
	}

	// Now withdraw the challenge
	withdrawEvent := ledger.NewChallengeWithdrawn("chal-001")
	if err := Apply(s, withdrawEvent); err != nil {
		t.Fatalf("Apply ChallengeWithdrawn failed: %v", err)
	}

	// The cache should be invalidated, so GetBlockingChallengesForNode should return 0
	blocking = s.GetBlockingChallengesForNode(nodeID)
	if len(blocking) != 0 {
		t.Errorf("Expected 0 blocking challenges after withdraw, got %d (cache not invalidated?)", len(blocking))
	}
}

// TestApplyChallengeSupersededInvalidatesCache verifies that superseding a challenge invalidates
// the challengesByNode cache so that subsequent lookups reflect the new status.
func TestApplyChallengeSupersededInvalidatesCache(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a challenge
	raiseEvent := ledger.NewChallengeRaised("chal-001", nodeID, "statement", "This is incorrect")
	if err := Apply(s, raiseEvent); err != nil {
		t.Fatalf("Apply ChallengeRaised failed: %v", err)
	}

	// Access the cache to populate it
	_ = s.ChallengesByNodeID()

	// Get blocking challenges before supersede
	blocking := s.GetBlockingChallengesForNode(nodeID)
	if len(blocking) != 1 {
		t.Fatalf("Expected 1 blocking challenge before supersede, got %d", len(blocking))
	}

	// Now supersede the challenge
	supersededEvent := ledger.NewChallengeSuperseded("chal-001", nodeID)
	if err := Apply(s, supersededEvent); err != nil {
		t.Fatalf("Apply ChallengeSuperseded failed: %v", err)
	}

	// The cache should be invalidated, so GetBlockingChallengesForNode should return 0
	blocking = s.GetBlockingChallengesForNode(nodeID)
	if len(blocking) != 0 {
		t.Errorf("Expected 0 blocking challenges after supersede, got %d (cache not invalidated?)", len(blocking))
	}
}

// TestApplyNodeArchivedSupersedeInvalidatesCache verifies that archiving a node and superseding
// its challenges invalidates the cache correctly.
func TestApplyNodeArchivedSupersedeInvalidatesCache(t *testing.T) {
	s := NewState()

	// Add a node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Add a challenge
	raiseEvent := ledger.NewChallengeRaised("chal-001", nodeID, "statement", "This is incorrect")
	if err := Apply(s, raiseEvent); err != nil {
		t.Fatalf("Apply ChallengeRaised failed: %v", err)
	}

	// Populate the cache
	_ = s.ChallengesByNodeID()

	// Verify we have 1 blocking challenge
	blocking := s.GetBlockingChallengesForNode(nodeID)
	if len(blocking) != 1 {
		t.Fatalf("Expected 1 blocking challenge before archive, got %d", len(blocking))
	}

	// Archive the node (should auto-supersede the challenge)
	archiveEvent := ledger.NewNodeArchived(nodeID)
	if err := Apply(s, archiveEvent); err != nil {
		t.Fatalf("Apply NodeArchived failed: %v", err)
	}

	// Cache should be invalidated, blocking challenges should now be 0
	blocking = s.GetBlockingChallengesForNode(nodeID)
	if len(blocking) != 0 {
		t.Errorf("Expected 0 blocking challenges after archive, got %d (cache not invalidated?)", len(blocking))
	}
}

// TestApplyNodeRefutedOnlyAffectsMatchingChallenges verifies that refuting a node only supersedes challenges on that specific node.
func TestApplyNodeRefutedOnlyAffectsMatchingChallenges(t *testing.T) {
	s := NewState()

	// Add two nodes
	node1ID := mustParseNodeID(t, "1")
	n1, err := node.NewNode(node1ID, schema.NodeTypeClaim, "Node 1", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create node 1: %v", err)
	}
	s.AddNode(n1)

	node2ID := mustParseNodeID(t, "1.1")
	n2, err := node.NewNode(node2ID, schema.NodeTypeClaim, "Node 2", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("Failed to create node 2: %v", err)
	}
	s.AddNode(n2)

	// Raise challenges on both nodes
	raiseEvent1 := ledger.NewChallengeRaised("chal-node1", node1ID, "statement", "Challenge on node 1")
	err = Apply(s, raiseEvent1)
	if err != nil {
		t.Fatalf("Apply ChallengeRaised for node 1 failed: %v", err)
	}

	raiseEvent2 := ledger.NewChallengeRaised("chal-node2", node2ID, "statement", "Challenge on node 2")
	err = Apply(s, raiseEvent2)
	if err != nil {
		t.Fatalf("Apply ChallengeRaised for node 2 failed: %v", err)
	}

	// Refute node 1 only
	refuteEvent := ledger.NewNodeRefuted(node1ID)
	err = Apply(s, refuteEvent)
	if err != nil {
		t.Fatalf("Apply NodeRefuted failed: %v", err)
	}

	// Verify challenge on node 1 is superseded
	chalNode1 := s.GetChallenge("chal-node1")
	if chalNode1.Status != "superseded" {
		t.Errorf("Challenge on node 1 should be superseded: got %q", chalNode1.Status)
	}

	// Verify challenge on node 2 is still open
	chalNode2 := s.GetChallenge("chal-node2")
	if chalNode2.Status != "open" {
		t.Errorf("Challenge on node 2 should still be open: got %q", chalNode2.Status)
	}
}

// TestApplyClaimRefreshed verifies that ClaimRefreshed event updates claim timeout.
func TestApplyClaimRefreshed(t *testing.T) {
	s := NewState()

	// Add a claimed node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	originalTimeout := types.Now()
	n.WorkflowState = schema.WorkflowClaimed
	n.ClaimedBy = "agent-123"
	n.ClaimedAt = originalTimeout
	s.AddNode(n)

	// Apply ClaimRefreshed event with new timeout
	newTimeout := types.Now()
	event := ledger.NewClaimRefreshed(nodeID, "agent-123", newTimeout)

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply ClaimRefreshed failed: %v", err)
	}

	// Verify timeout was updated
	got := s.GetNode(nodeID)
	if got.ClaimedAt != newTimeout {
		t.Errorf("ClaimedAt not updated: got %v, want %v", got.ClaimedAt, newTimeout)
	}

	// Verify workflow state remains claimed
	if got.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("WorkflowState changed unexpectedly: got %q, want %q", got.WorkflowState, schema.WorkflowClaimed)
	}

	// Verify owner remains the same
	if got.ClaimedBy != "agent-123" {
		t.Errorf("ClaimedBy changed unexpectedly: got %q, want %q", got.ClaimedBy, "agent-123")
	}
}

// TestApplyClaimRefreshed_NonExistentNode verifies error when refreshing claim on non-existent node.
func TestApplyClaimRefreshed_NonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewClaimRefreshed(nodeID, "agent-123", types.Now())

	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when refreshing claim on non-existent node")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention 'not found': got %q", err.Error())
	}
}

// TestApplyClaimRefreshed_NotClaimed verifies error when refreshing claim on unclaimed node.
func TestApplyClaimRefreshed_NotClaimed(t *testing.T) {
	s := NewState()

	// Add an unclaimed node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	// Node is available by default
	s.AddNode(n)

	event := ledger.NewClaimRefreshed(nodeID, "agent-123", types.Now())

	err = Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when refreshing claim on unclaimed node")
	}

	if !strings.Contains(err.Error(), "not claimed") {
		t.Errorf("Error should mention 'not claimed': got %q", err.Error())
	}
}

// TestApplyClaimRefreshed_WrongOwner verifies error when refreshing claim with wrong owner.
func TestApplyClaimRefreshed_WrongOwner(t *testing.T) {
	s := NewState()

	// Add a claimed node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	n.WorkflowState = schema.WorkflowClaimed
	n.ClaimedBy = "agent-123"
	n.ClaimedAt = types.Now()
	s.AddNode(n)

	// Try to refresh with a different owner
	event := ledger.NewClaimRefreshed(nodeID, "agent-456", types.Now())

	err = Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when refreshing claim with wrong owner")
	}

	if !strings.Contains(err.Error(), "claimed by") {
		t.Errorf("Error should mention ownership mismatch: got %q", err.Error())
	}
}

// TestApplyNodeAmended verifies that NodeAmended event updates node statement.
func TestApplyNodeAmended(t *testing.T) {
	s := NewState()

	// Add a node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Original statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Apply NodeAmended event
	event := ledger.NewNodeAmended(nodeID, "Original statement", "Amended statement", "prover-agent")

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply NodeAmended failed: %v", err)
	}

	// Verify statement was updated
	got := s.GetNode(nodeID)
	if got.Statement != "Amended statement" {
		t.Errorf("Statement not updated: got %q, want %q", got.Statement, "Amended statement")
	}

	// Verify content hash was recomputed
	expectedHash := got.ComputeContentHash()
	if got.ContentHash != expectedHash {
		t.Errorf("ContentHash not recomputed: got %q, want %q", got.ContentHash, expectedHash)
	}
}

// TestApplyNodeAmended_NonExistentNode verifies error when amending non-existent node.
func TestApplyNodeAmended_NonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewNodeAmended(nodeID, "old", "new", "owner")

	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when amending non-existent node")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention 'not found': got %q", err.Error())
	}
}

// TestApplyNodeAmended_RecordsAmendmentHistory verifies that amendments are tracked.
func TestApplyNodeAmended_RecordsAmendmentHistory(t *testing.T) {
	s := NewState()

	// Add a node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Original statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Apply multiple amendments
	event1 := ledger.NewNodeAmended(nodeID, "Original statement", "First amendment", "owner1")
	if err := Apply(s, event1); err != nil {
		t.Fatalf("Apply first NodeAmended failed: %v", err)
	}

	event2 := ledger.NewNodeAmended(nodeID, "First amendment", "Second amendment", "owner2")
	if err := Apply(s, event2); err != nil {
		t.Fatalf("Apply second NodeAmended failed: %v", err)
	}

	// Verify amendment history
	history := s.GetAmendmentHistory(nodeID)
	if len(history) != 2 {
		t.Fatalf("Expected 2 amendments in history, got %d", len(history))
	}

	if history[0].PreviousStatement != "Original statement" {
		t.Errorf("First amendment previous: got %q, want %q", history[0].PreviousStatement, "Original statement")
	}
	if history[0].NewStatement != "First amendment" {
		t.Errorf("First amendment new: got %q, want %q", history[0].NewStatement, "First amendment")
	}
	if history[0].Owner != "owner1" {
		t.Errorf("First amendment owner: got %q, want %q", history[0].Owner, "owner1")
	}

	if history[1].PreviousStatement != "First amendment" {
		t.Errorf("Second amendment previous: got %q, want %q", history[1].PreviousStatement, "First amendment")
	}
	if history[1].NewStatement != "Second amendment" {
		t.Errorf("Second amendment new: got %q, want %q", history[1].NewStatement, "Second amendment")
	}
}

// TestApplyScopeOpened verifies that ScopeOpened event opens a scope.
func TestApplyScopeOpened(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")

	// Apply ScopeOpened event
	event := ledger.NewScopeOpened(nodeID, "Assume P(x) holds")

	err := Apply(s, event)
	if err != nil {
		t.Fatalf("Apply ScopeOpened failed: %v", err)
	}

	// Verify scope was opened
	scope := s.GetScope(nodeID)
	if scope == nil {
		t.Fatal("Scope not found after ScopeOpened event")
	}

	if scope.Discharged != nil {
		t.Error("Scope should be active (Discharged=nil) after opening")
	}

	if scope.Statement != "Assume P(x) holds" {
		t.Errorf("Scope statement: got %q, want %q", scope.Statement, "Assume P(x) holds")
	}

	// Verify active scopes list
	activeScopes := s.GetActiveScopes()
	if len(activeScopes) != 1 {
		t.Errorf("Expected 1 active scope, got %d", len(activeScopes))
	}
}

// TestApplyScopeClosed verifies that ScopeClosed event closes a scope.
func TestApplyScopeClosed(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")

	// First open a scope
	openEvent := ledger.NewScopeOpened(nodeID, "Assume P(x) holds")
	if err := Apply(s, openEvent); err != nil {
		t.Fatalf("Apply ScopeOpened failed: %v", err)
	}

	// Verify scope is active
	if len(s.GetActiveScopes()) != 1 {
		t.Fatal("Scope should be active before closing")
	}

	// Apply ScopeClosed event
	dischargeID := mustParseNodeID(t, "1.1")
	closeEvent := ledger.NewScopeClosed(nodeID, dischargeID)

	err := Apply(s, closeEvent)
	if err != nil {
		t.Fatalf("Apply ScopeClosed failed: %v", err)
	}

	// Verify scope is now closed
	scope := s.GetScope(nodeID)
	if scope == nil {
		t.Fatal("Scope not found after ScopeClosed event")
	}

	if scope.Discharged == nil {
		t.Error("Scope should be inactive (Discharged!=nil) after closing")
	}

	// Verify no active scopes remain
	activeScopes := s.GetActiveScopes()
	if len(activeScopes) != 0 {
		t.Errorf("Expected 0 active scopes after closing, got %d", len(activeScopes))
	}

	// Verify scope is still in all scopes list
	allScopes := s.GetAllScopes()
	if len(allScopes) != 1 {
		t.Errorf("Expected 1 scope in all scopes, got %d", len(allScopes))
	}
}

// TestApplyRefinementRequested verifies that RefinementRequested event updates epistemic state.
func TestApplyRefinementRequested(t *testing.T) {
	s := NewState()

	// Add a validated node (only validated nodes can be refined)
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	n.EpistemicState = schema.EpistemicValidated
	s.AddNode(n)

	// Apply RefinementRequested event
	event := ledger.NewRefinementRequested(nodeID, "Needs more detailed proof", "verifier-1")

	err = Apply(s, event)
	if err != nil {
		t.Fatalf("Apply RefinementRequested failed: %v", err)
	}

	// Verify epistemic state changed to needs_refinement
	got := s.GetNode(nodeID)
	if got.EpistemicState != schema.EpistemicNeedsRefinement {
		t.Errorf("Epistemic state after refinement request: got %q, want %q", got.EpistemicState, schema.EpistemicNeedsRefinement)
	}
}

// TestApplyRefinementRequested_NonExistentNode verifies error when requesting refinement on non-existent node.
func TestApplyRefinementRequested_NonExistentNode(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")
	event := ledger.NewRefinementRequested(nodeID, "Reason", "verifier-1")

	err := Apply(s, event)
	if err == nil {
		t.Fatal("Apply should return error when requesting refinement on non-existent node")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention 'not found': got %q", err.Error())
	}
}

// TestApplyRefinementRequested_OnlyValidatedNodesCanBeRefined verifies that only validated nodes can be refined.
func TestApplyRefinementRequested_OnlyValidatedNodesCanBeRefined(t *testing.T) {
	tests := []struct {
		name      string
		epistemic schema.EpistemicState
		wantError bool
	}{
		{"pending cannot be refined", schema.EpistemicPending, true},
		{"validated can be refined", schema.EpistemicValidated, false},
		{"admitted cannot be refined", schema.EpistemicAdmitted, true},
		{"refuted cannot be refined", schema.EpistemicRefuted, true},
		{"archived cannot be refined", schema.EpistemicArchived, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewState()

			nodeID := mustParseNodeID(t, "1")
			n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
			if err != nil {
				t.Fatalf("Failed to create test node: %v", err)
			}
			n.EpistemicState = tt.epistemic
			s.AddNode(n)

			event := ledger.NewRefinementRequested(nodeID, "Reason", "verifier-1")
			err = Apply(s, event)

			if tt.wantError {
				if err == nil {
					t.Errorf("Apply should return error for %s state", tt.epistemic)
				} else if !strings.Contains(err.Error(), "invalid transition") {
					t.Errorf("Error should mention 'invalid transition': got %q", err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Apply should succeed for %s state: %v", tt.epistemic, err)
				}
			}
		})
	}
}

// TestGetNodesNeedingRefinement verifies the query method for refinement nodes.
func TestGetNodesNeedingRefinement(t *testing.T) {
	s := NewState()

	// Add nodes in various states
	node1ID := mustParseNodeID(t, "1")
	n1, _ := node.NewNode(node1ID, schema.NodeTypeClaim, "Node 1", schema.InferenceAssumption)
	n1.EpistemicState = schema.EpistemicNeedsRefinement
	s.AddNode(n1)

	node2ID := mustParseNodeID(t, "1.1")
	n2, _ := node.NewNode(node2ID, schema.NodeTypeClaim, "Node 2", schema.InferenceAssumption)
	n2.EpistemicState = schema.EpistemicValidated
	s.AddNode(n2)

	node3ID := mustParseNodeID(t, "1.2")
	n3, _ := node.NewNode(node3ID, schema.NodeTypeClaim, "Node 3", schema.InferenceAssumption)
	n3.EpistemicState = schema.EpistemicNeedsRefinement
	s.AddNode(n3)

	node4ID := mustParseNodeID(t, "1.3")
	n4, _ := node.NewNode(node4ID, schema.NodeTypeClaim, "Node 4", schema.InferenceAssumption)
	n4.EpistemicState = schema.EpistemicPending
	s.AddNode(n4)

	// Get nodes needing refinement
	result := s.GetNodesNeedingRefinement()

	if len(result) != 2 {
		t.Errorf("GetNodesNeedingRefinement() returned %d nodes, want 2", len(result))
	}

	// Verify only needs_refinement nodes are returned
	resultIDs := make(map[string]bool)
	for _, n := range result {
		resultIDs[n.ID.String()] = true
	}

	if !resultIDs["1"] {
		t.Error("GetNodesNeedingRefinement() should include node 1")
	}
	if !resultIDs["1.2"] {
		t.Error("GetNodesNeedingRefinement() should include node 1.2")
	}
}
