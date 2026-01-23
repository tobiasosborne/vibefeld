//go:build integration

package ledger

import (
	"encoding/json"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestEventTypeConstants verifies all event type constants are defined correctly.
func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant EventType
		expected string
	}{
		{"ProofInitialized", EventProofInitialized, "proof_initialized"},
		{"NodeCreated", EventNodeCreated, "node_created"},
		{"NodesClaimed", EventNodesClaimed, "nodes_claimed"},
		{"NodesReleased", EventNodesReleased, "nodes_released"},
		{"ChallengeRaised", EventChallengeRaised, "challenge_raised"},
		{"ChallengeResolved", EventChallengeResolved, "challenge_resolved"},
		{"ChallengeWithdrawn", EventChallengeWithdrawn, "challenge_withdrawn"},
		{"NodeValidated", EventNodeValidated, "node_validated"},
		{"NodeAdmitted", EventNodeAdmitted, "node_admitted"},
		{"NodeRefuted", EventNodeRefuted, "node_refuted"},
		{"NodeArchived", EventNodeArchived, "node_archived"},
		{"TaintRecomputed", EventTaintRecomputed, "taint_recomputed"},
		{"DefAdded", EventDefAdded, "def_added"},
		{"LemmaExtracted", EventLemmaExtracted, "lemma_extracted"},
		{"RefinementRequested", EventRefinementRequested, "refinement_requested"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("EventType %s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

// TestProofInitializedEvent tests ProofInitialized event creation and serialization.
func TestProofInitializedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		event := NewProofInitialized("P implies Q", "agent-001")

		if event.Type() != EventProofInitialized {
			t.Errorf("Type() = %q, want %q", event.Type(), EventProofInitialized)
		}
		if event.Conjecture != "P implies Q" {
			t.Errorf("Conjecture = %q, want %q", event.Conjecture, "P implies Q")
		}
		if event.Author != "agent-001" {
			t.Errorf("Author = %q, want %q", event.Author, "agent-001")
		}
		if event.Timestamp().IsZero() {
			t.Error("Timestamp should not be zero")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		original := NewProofInitialized("For all x, P(x)", "prover-alpha")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded ProofInitialized
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Type() != original.Type() {
			t.Errorf("Type mismatch: got %q, want %q", decoded.Type(), original.Type())
		}
		if decoded.Conjecture != original.Conjecture {
			t.Errorf("Conjecture mismatch: got %q, want %q", decoded.Conjecture, original.Conjecture)
		}
		if decoded.Author != original.Author {
			t.Errorf("Author mismatch: got %q, want %q", decoded.Author, original.Author)
		}
		if !decoded.Timestamp().Equal(original.Timestamp()) {
			t.Errorf("Timestamp mismatch: got %v, want %v", decoded.Timestamp(), original.Timestamp())
		}
	})
}

// TestNodeCreatedEvent tests NodeCreated event creation and serialization.
func TestNodeCreatedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1")
		n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("Failed to create node: %v", err)
		}

		event := NewNodeCreated(*n)

		if event.Type() != EventNodeCreated {
			t.Errorf("Type() = %q, want %q", event.Type(), EventNodeCreated)
		}
		if event.Node.ID.String() != "1" {
			t.Errorf("Node.ID = %q, want %q", event.Node.ID.String(), "1")
		}
		if event.Timestamp().IsZero() {
			t.Error("Timestamp should not be zero")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		nodeID, _ := types.Parse("1.2")
		n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Another statement", schema.InferenceModusPonens)
		original := NewNodeCreated(*n)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded NodeCreated
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Type() != original.Type() {
			t.Errorf("Type mismatch: got %q, want %q", decoded.Type(), original.Type())
		}
		if decoded.Node.ID.String() != original.Node.ID.String() {
			t.Errorf("Node.ID mismatch: got %q, want %q", decoded.Node.ID.String(), original.Node.ID.String())
		}
		if decoded.Node.Statement != original.Node.Statement {
			t.Errorf("Node.Statement mismatch: got %q, want %q", decoded.Node.Statement, original.Node.Statement)
		}
	})
}

// TestNodesClaimedEvent tests NodesClaimed event creation and serialization.
func TestNodesClaimedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		id1, _ := types.Parse("1")
		id2, _ := types.Parse("1.1")
		nodeIDs := []types.NodeID{id1, id2}
		timeout := types.Now()

		event := NewNodesClaimed(nodeIDs, "agent-prover", timeout)

		if event.Type() != EventNodesClaimed {
			t.Errorf("Type() = %q, want %q", event.Type(), EventNodesClaimed)
		}
		if len(event.NodeIDs) != 2 {
			t.Errorf("NodeIDs length = %d, want 2", len(event.NodeIDs))
		}
		if event.Owner != "agent-prover" {
			t.Errorf("Owner = %q, want %q", event.Owner, "agent-prover")
		}
		if event.Timeout.IsZero() {
			t.Error("Timeout should not be zero")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		id1, _ := types.Parse("1.2.3")
		nodeIDs := []types.NodeID{id1}
		timeout := types.Now()
		original := NewNodesClaimed(nodeIDs, "verifier-beta", timeout)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded NodesClaimed
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Type() != original.Type() {
			t.Errorf("Type mismatch: got %q, want %q", decoded.Type(), original.Type())
		}
		if len(decoded.NodeIDs) != len(original.NodeIDs) {
			t.Errorf("NodeIDs length mismatch: got %d, want %d", len(decoded.NodeIDs), len(original.NodeIDs))
		}
		if decoded.Owner != original.Owner {
			t.Errorf("Owner mismatch: got %q, want %q", decoded.Owner, original.Owner)
		}
	})
}

// TestNodesReleasedEvent tests NodesReleased event creation and serialization.
func TestNodesReleasedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		id1, _ := types.Parse("1")
		nodeIDs := []types.NodeID{id1}

		event := NewNodesReleased(nodeIDs)

		if event.Type() != EventNodesReleased {
			t.Errorf("Type() = %q, want %q", event.Type(), EventNodesReleased)
		}
		if len(event.NodeIDs) != 1 {
			t.Errorf("NodeIDs length = %d, want 1", len(event.NodeIDs))
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		id1, _ := types.Parse("1.1")
		id2, _ := types.Parse("1.2")
		original := NewNodesReleased([]types.NodeID{id1, id2})

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded NodesReleased
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Type() != original.Type() {
			t.Errorf("Type mismatch: got %q, want %q", decoded.Type(), original.Type())
		}
		if len(decoded.NodeIDs) != len(original.NodeIDs) {
			t.Errorf("NodeIDs length mismatch: got %d, want %d", len(decoded.NodeIDs), len(original.NodeIDs))
		}
	})
}

// TestChallengeRaisedEvent tests ChallengeRaised event creation and serialization.
func TestChallengeRaisedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1.1")

		event := NewChallengeRaised("chal-001", nodeID, "inference", "Deduction not valid here")

		if event.Type() != EventChallengeRaised {
			t.Errorf("Type() = %q, want %q", event.Type(), EventChallengeRaised)
		}
		if event.ChallengeID != "chal-001" {
			t.Errorf("ChallengeID = %q, want %q", event.ChallengeID, "chal-001")
		}
		if event.NodeID.String() != "1.1" {
			t.Errorf("NodeID = %q, want %q", event.NodeID.String(), "1.1")
		}
		if event.Target != "inference" {
			t.Errorf("Target = %q, want %q", event.Target, "inference")
		}
		if event.Reason != "Deduction not valid here" {
			t.Errorf("Reason = %q, want %q", event.Reason, "Deduction not valid here")
		}
		// NewChallengeRaised uses default empty RaisedBy
		if event.RaisedBy != "" {
			t.Errorf("RaisedBy = %q, want empty string", event.RaisedBy)
		}
	})

	t.Run("creation with severity and raisedBy", func(t *testing.T) {
		nodeID, _ := types.Parse("1.2")

		event := NewChallengeRaisedWithSeverity("chal-002", nodeID, "statement", "Unclear wording", "critical", "verifier-42")

		if event.Type() != EventChallengeRaised {
			t.Errorf("Type() = %q, want %q", event.Type(), EventChallengeRaised)
		}
		if event.ChallengeID != "chal-002" {
			t.Errorf("ChallengeID = %q, want %q", event.ChallengeID, "chal-002")
		}
		if event.Severity != "critical" {
			t.Errorf("Severity = %q, want %q", event.Severity, "critical")
		}
		if event.RaisedBy != "verifier-42" {
			t.Errorf("RaisedBy = %q, want %q", event.RaisedBy, "verifier-42")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		nodeID, _ := types.Parse("1.2.3")
		original := NewChallengeRaised("chal-xyz", nodeID, "statement", "Statement is ambiguous")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded ChallengeRaised
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.ChallengeID != original.ChallengeID {
			t.Errorf("ChallengeID mismatch: got %q, want %q", decoded.ChallengeID, original.ChallengeID)
		}
		if decoded.NodeID.String() != original.NodeID.String() {
			t.Errorf("NodeID mismatch: got %q, want %q", decoded.NodeID.String(), original.NodeID.String())
		}
		if decoded.Target != original.Target {
			t.Errorf("Target mismatch: got %q, want %q", decoded.Target, original.Target)
		}
		if decoded.Reason != original.Reason {
			t.Errorf("Reason mismatch: got %q, want %q", decoded.Reason, original.Reason)
		}
	})

	t.Run("JSON roundtrip with RaisedBy", func(t *testing.T) {
		nodeID, _ := types.Parse("1.3.4")
		original := NewChallengeRaisedWithSeverity("chal-abc", nodeID, "inference", "Invalid deduction", "major", "agent-alpha")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded ChallengeRaised
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.ChallengeID != original.ChallengeID {
			t.Errorf("ChallengeID mismatch: got %q, want %q", decoded.ChallengeID, original.ChallengeID)
		}
		if decoded.NodeID.String() != original.NodeID.String() {
			t.Errorf("NodeID mismatch: got %q, want %q", decoded.NodeID.String(), original.NodeID.String())
		}
		if decoded.Target != original.Target {
			t.Errorf("Target mismatch: got %q, want %q", decoded.Target, original.Target)
		}
		if decoded.Reason != original.Reason {
			t.Errorf("Reason mismatch: got %q, want %q", decoded.Reason, original.Reason)
		}
		if decoded.Severity != original.Severity {
			t.Errorf("Severity mismatch: got %q, want %q", decoded.Severity, original.Severity)
		}
		if decoded.RaisedBy != original.RaisedBy {
			t.Errorf("RaisedBy mismatch: got %q, want %q", decoded.RaisedBy, original.RaisedBy)
		}
	})
}

// TestChallengeResolvedEvent tests ChallengeResolved event creation and serialization.
func TestChallengeResolvedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		event := NewChallengeResolved("chal-001")

		if event.Type() != EventChallengeResolved {
			t.Errorf("Type() = %q, want %q", event.Type(), EventChallengeResolved)
		}
		if event.ChallengeID != "chal-001" {
			t.Errorf("ChallengeID = %q, want %q", event.ChallengeID, "chal-001")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		original := NewChallengeResolved("chal-abc")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded ChallengeResolved
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.ChallengeID != original.ChallengeID {
			t.Errorf("ChallengeID mismatch: got %q, want %q", decoded.ChallengeID, original.ChallengeID)
		}
	})
}

// TestChallengeWithdrawnEvent tests ChallengeWithdrawn event creation and serialization.
func TestChallengeWithdrawnEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		event := NewChallengeWithdrawn("chal-002")

		if event.Type() != EventChallengeWithdrawn {
			t.Errorf("Type() = %q, want %q", event.Type(), EventChallengeWithdrawn)
		}
		if event.ChallengeID != "chal-002" {
			t.Errorf("ChallengeID = %q, want %q", event.ChallengeID, "chal-002")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		original := NewChallengeWithdrawn("chal-def")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded ChallengeWithdrawn
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.ChallengeID != original.ChallengeID {
			t.Errorf("ChallengeID mismatch: got %q, want %q", decoded.ChallengeID, original.ChallengeID)
		}
	})
}

// TestNodeValidatedEvent tests NodeValidated event creation and serialization.
func TestNodeValidatedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1.1")
		event := NewNodeValidated(nodeID)

		if event.Type() != EventNodeValidated {
			t.Errorf("Type() = %q, want %q", event.Type(), EventNodeValidated)
		}
		if event.NodeID.String() != "1.1" {
			t.Errorf("NodeID = %q, want %q", event.NodeID.String(), "1.1")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		nodeID, _ := types.Parse("1.2.3")
		original := NewNodeValidated(nodeID)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded NodeValidated
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.NodeID.String() != original.NodeID.String() {
			t.Errorf("NodeID mismatch: got %q, want %q", decoded.NodeID.String(), original.NodeID.String())
		}
	})
}

// TestNodeAdmittedEvent tests NodeAdmitted event creation and serialization.
func TestNodeAdmittedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1.3")
		event := NewNodeAdmitted(nodeID)

		if event.Type() != EventNodeAdmitted {
			t.Errorf("Type() = %q, want %q", event.Type(), EventNodeAdmitted)
		}
		if event.NodeID.String() != "1.3" {
			t.Errorf("NodeID = %q, want %q", event.NodeID.String(), "1.3")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		nodeID, _ := types.Parse("1.4.5")
		original := NewNodeAdmitted(nodeID)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded NodeAdmitted
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.NodeID.String() != original.NodeID.String() {
			t.Errorf("NodeID mismatch: got %q, want %q", decoded.NodeID.String(), original.NodeID.String())
		}
	})
}

// TestNodeRefutedEvent tests NodeRefuted event creation and serialization.
func TestNodeRefutedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1.2")
		event := NewNodeRefuted(nodeID)

		if event.Type() != EventNodeRefuted {
			t.Errorf("Type() = %q, want %q", event.Type(), EventNodeRefuted)
		}
		if event.NodeID.String() != "1.2" {
			t.Errorf("NodeID = %q, want %q", event.NodeID.String(), "1.2")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		nodeID, _ := types.Parse("1.2.1")
		original := NewNodeRefuted(nodeID)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded NodeRefuted
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.NodeID.String() != original.NodeID.String() {
			t.Errorf("NodeID mismatch: got %q, want %q", decoded.NodeID.String(), original.NodeID.String())
		}
	})
}

// TestNodeArchivedEvent tests NodeArchived event creation and serialization.
func TestNodeArchivedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1.5")
		event := NewNodeArchived(nodeID)

		if event.Type() != EventNodeArchived {
			t.Errorf("Type() = %q, want %q", event.Type(), EventNodeArchived)
		}
		if event.NodeID.String() != "1.5" {
			t.Errorf("NodeID = %q, want %q", event.NodeID.String(), "1.5")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		nodeID, _ := types.Parse("1.5.6.7")
		original := NewNodeArchived(nodeID)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded NodeArchived
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.NodeID.String() != original.NodeID.String() {
			t.Errorf("NodeID mismatch: got %q, want %q", decoded.NodeID.String(), original.NodeID.String())
		}
	})
}

// TestTaintRecomputedEvent tests TaintRecomputed event creation and serialization.
func TestTaintRecomputedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1.1")
		event := NewTaintRecomputed(nodeID, node.TaintTainted)

		if event.Type() != EventTaintRecomputed {
			t.Errorf("Type() = %q, want %q", event.Type(), EventTaintRecomputed)
		}
		if event.NodeID.String() != "1.1" {
			t.Errorf("NodeID = %q, want %q", event.NodeID.String(), "1.1")
		}
		if event.NewTaint != node.TaintTainted {
			t.Errorf("NewTaint = %q, want %q", event.NewTaint, node.TaintTainted)
		}
	})

	t.Run("JSON roundtrip for each taint state", func(t *testing.T) {
		taintStates := []node.TaintState{
			node.TaintClean,
			node.TaintSelfAdmitted,
			node.TaintTainted,
			node.TaintUnresolved,
		}

		for _, taintState := range taintStates {
			t.Run(string(taintState), func(t *testing.T) {
				nodeID, _ := types.Parse("1.2")
				original := NewTaintRecomputed(nodeID, taintState)

				data, err := json.Marshal(original)
				if err != nil {
					t.Fatalf("Marshal failed: %v", err)
				}

				var decoded TaintRecomputed
				if err := json.Unmarshal(data, &decoded); err != nil {
					t.Fatalf("Unmarshal failed: %v", err)
				}

				if decoded.NodeID.String() != original.NodeID.String() {
					t.Errorf("NodeID mismatch: got %q, want %q", decoded.NodeID.String(), original.NodeID.String())
				}
				if decoded.NewTaint != original.NewTaint {
					t.Errorf("NewTaint mismatch: got %q, want %q", decoded.NewTaint, original.NewTaint)
				}
			})
		}
	})
}

// TestDefAddedEvent tests DefAdded event creation and serialization.
func TestDefAddedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		def := Definition{
			ID:         "def-001",
			Name:       "prime",
			Definition: "A natural number greater than 1 with exactly two divisors",
			Created:    types.Now(),
		}
		event := NewDefAdded(def)

		if event.Type() != EventDefAdded {
			t.Errorf("Type() = %q, want %q", event.Type(), EventDefAdded)
		}
		if event.Definition.ID != "def-001" {
			t.Errorf("Definition.ID = %q, want %q", event.Definition.ID, "def-001")
		}
		if event.Definition.Name != "prime" {
			t.Errorf("Definition.Name = %q, want %q", event.Definition.Name, "prime")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		def := Definition{
			ID:         "def-abc",
			Name:       "continuous",
			Definition: "A function f is continuous at x if...",
			Created:    types.Now(),
		}
		original := NewDefAdded(def)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded DefAdded
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Definition.ID != original.Definition.ID {
			t.Errorf("Definition.ID mismatch: got %q, want %q", decoded.Definition.ID, original.Definition.ID)
		}
		if decoded.Definition.Name != original.Definition.Name {
			t.Errorf("Definition.Name mismatch: got %q, want %q", decoded.Definition.Name, original.Definition.Name)
		}
		if decoded.Definition.Definition != original.Definition.Definition {
			t.Errorf("Definition.Definition mismatch: got %q, want %q", decoded.Definition.Definition, original.Definition.Definition)
		}
	})
}

// TestLemmaExtractedEvent tests LemmaExtracted event creation and serialization.
func TestLemmaExtractedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1.3")
		lemma := Lemma{
			ID:        "lemma-001",
			Statement: "For all x, if P(x) then Q(x)",
			NodeID:    nodeID,
			Created:   types.Now(),
		}
		event := NewLemmaExtracted(lemma)

		if event.Type() != EventLemmaExtracted {
			t.Errorf("Type() = %q, want %q", event.Type(), EventLemmaExtracted)
		}
		if event.Lemma.ID != "lemma-001" {
			t.Errorf("Lemma.ID = %q, want %q", event.Lemma.ID, "lemma-001")
		}
		if event.Lemma.Statement != "For all x, if P(x) then Q(x)" {
			t.Errorf("Lemma.Statement = %q, want %q", event.Lemma.Statement, "For all x, if P(x) then Q(x)")
		}
		if event.Lemma.NodeID.String() != "1.3" {
			t.Errorf("Lemma.NodeID = %q, want %q", event.Lemma.NodeID.String(), "1.3")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		nodeID, _ := types.Parse("1.2.4")
		lemma := Lemma{
			ID:        "lemma-xyz",
			Statement: "The limit exists and equals L",
			NodeID:    nodeID,
			Created:   types.Now(),
		}
		original := NewLemmaExtracted(lemma)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded LemmaExtracted
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Lemma.ID != original.Lemma.ID {
			t.Errorf("Lemma.ID mismatch: got %q, want %q", decoded.Lemma.ID, original.Lemma.ID)
		}
		if decoded.Lemma.Statement != original.Lemma.Statement {
			t.Errorf("Lemma.Statement mismatch: got %q, want %q", decoded.Lemma.Statement, original.Lemma.Statement)
		}
		if decoded.Lemma.NodeID.String() != original.Lemma.NodeID.String() {
			t.Errorf("Lemma.NodeID mismatch: got %q, want %q", decoded.Lemma.NodeID.String(), original.Lemma.NodeID.String())
		}
	})
}

// TestBaseEventInterface verifies that all event types implement the Event interface.
func TestBaseEventInterface(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test", schema.InferenceModusPonens)

	// Create instances of each event type
	events := []Event{
		NewProofInitialized("conjecture", "author"),
		NewNodeCreated(*n),
		NewNodesClaimed([]types.NodeID{nodeID}, "owner", types.Now()),
		NewNodesReleased([]types.NodeID{nodeID}),
		NewChallengeRaised("chal", nodeID, "target", "reason"),
		NewChallengeResolved("chal"),
		NewChallengeWithdrawn("chal"),
		NewNodeValidated(nodeID),
		NewNodeAdmitted(nodeID),
		NewNodeRefuted(nodeID),
		NewNodeArchived(nodeID),
		NewTaintRecomputed(nodeID, node.TaintClean),
		NewDefAdded(Definition{ID: "def", Name: "name", Definition: "def", Created: types.Now()}),
		NewLemmaExtracted(Lemma{ID: "lemma", Statement: "stmt", NodeID: nodeID, Created: types.Now()}),
		NewRefinementRequested(nodeID, "reason", "requester"),
	}

	for _, e := range events {
		t.Run(string(e.Type()), func(t *testing.T) {
			// Verify Type() returns non-empty
			if e.Type() == "" {
				t.Error("Type() returned empty string")
			}
			// Verify Timestamp() returns non-zero
			if e.Timestamp().IsZero() {
				t.Error("Timestamp() returned zero value")
			}
		})
	}
}

// TestEventJSONFieldNames verifies that JSON field names match expected conventions.
func TestEventJSONFieldNames(t *testing.T) {
	t.Run("ProofInitialized fields", func(t *testing.T) {
		event := NewProofInitialized("test", "author")
		data, _ := json.Marshal(event)
		jsonStr := string(data)

		expectedFields := []string{`"type"`, `"timestamp"`, `"conjecture"`, `"author"`}
		for _, field := range expectedFields {
			if !contains(jsonStr, field) {
				t.Errorf("JSON missing field %s: %s", field, jsonStr)
			}
		}
	})

	t.Run("NodesClaimed fields", func(t *testing.T) {
		nodeID, _ := types.Parse("1")
		event := NewNodesClaimed([]types.NodeID{nodeID}, "owner", types.Now())
		data, _ := json.Marshal(event)
		jsonStr := string(data)

		expectedFields := []string{`"type"`, `"timestamp"`, `"node_ids"`, `"owner"`, `"timeout"`}
		for _, field := range expectedFields {
			if !contains(jsonStr, field) {
				t.Errorf("JSON missing field %s: %s", field, jsonStr)
			}
		}
	})

	t.Run("ChallengeRaised fields", func(t *testing.T) {
		nodeID, _ := types.Parse("1")
		event := NewChallengeRaisedWithSeverity("chal-id", nodeID, "target", "reason", "major", "verifier-1")
		data, _ := json.Marshal(event)
		jsonStr := string(data)

		expectedFields := []string{`"type"`, `"timestamp"`, `"challenge_id"`, `"node_id"`, `"target"`, `"reason"`, `"severity"`, `"raised_by"`}
		for _, field := range expectedFields {
			if !contains(jsonStr, field) {
				t.Errorf("JSON missing field %s: %s", field, jsonStr)
			}
		}
	})

	t.Run("TaintRecomputed fields", func(t *testing.T) {
		nodeID, _ := types.Parse("1")
		event := NewTaintRecomputed(nodeID, node.TaintClean)
		data, _ := json.Marshal(event)
		jsonStr := string(data)

		expectedFields := []string{`"type"`, `"timestamp"`, `"node_id"`, `"new_taint"`}
		for _, field := range expectedFields {
			if !contains(jsonStr, field) {
				t.Errorf("JSON missing field %s: %s", field, jsonStr)
			}
		}
	})
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestScopeOpenedEvent tests ScopeOpened event creation and serialization.
func TestScopeOpenedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1.2")
		event := NewScopeOpened(nodeID, "Assume P(x) for some x")

		if event.Type() != EventScopeOpened {
			t.Errorf("Type() = %q, want %q", event.Type(), EventScopeOpened)
		}
		if event.NodeID.String() != "1.2" {
			t.Errorf("NodeID = %q, want %q", event.NodeID.String(), "1.2")
		}
		if event.Statement != "Assume P(x) for some x" {
			t.Errorf("Statement = %q, want %q", event.Statement, "Assume P(x) for some x")
		}
		if event.Timestamp().IsZero() {
			t.Error("Timestamp should not be zero")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		nodeID, _ := types.Parse("1.3.4")
		original := NewScopeOpened(nodeID, "Assume n is even")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded ScopeOpened
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Type() != original.Type() {
			t.Errorf("Type mismatch: got %q, want %q", decoded.Type(), original.Type())
		}
		if decoded.NodeID.String() != original.NodeID.String() {
			t.Errorf("NodeID mismatch: got %q, want %q", decoded.NodeID.String(), original.NodeID.String())
		}
		if decoded.Statement != original.Statement {
			t.Errorf("Statement mismatch: got %q, want %q", decoded.Statement, original.Statement)
		}
	})
}

// TestScopeClosedEvent tests ScopeClosed event creation and serialization.
func TestScopeClosedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1.2")
		dischargeNodeID, _ := types.Parse("1.2.3")
		event := NewScopeClosed(nodeID, dischargeNodeID)

		if event.Type() != EventScopeClosed {
			t.Errorf("Type() = %q, want %q", event.Type(), EventScopeClosed)
		}
		if event.NodeID.String() != "1.2" {
			t.Errorf("NodeID = %q, want %q", event.NodeID.String(), "1.2")
		}
		if event.DischargeNodeID.String() != "1.2.3" {
			t.Errorf("DischargeNodeID = %q, want %q", event.DischargeNodeID.String(), "1.2.3")
		}
		if event.Timestamp().IsZero() {
			t.Error("Timestamp should not be zero")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		nodeID, _ := types.Parse("1.4")
		dischargeNodeID, _ := types.Parse("1.4.5.6")
		original := NewScopeClosed(nodeID, dischargeNodeID)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded ScopeClosed
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Type() != original.Type() {
			t.Errorf("Type mismatch: got %q, want %q", decoded.Type(), original.Type())
		}
		if decoded.NodeID.String() != original.NodeID.String() {
			t.Errorf("NodeID mismatch: got %q, want %q", decoded.NodeID.String(), original.NodeID.String())
		}
		if decoded.DischargeNodeID.String() != original.DischargeNodeID.String() {
			t.Errorf("DischargeNodeID mismatch: got %q, want %q", decoded.DischargeNodeID.String(), original.DischargeNodeID.String())
		}
	})
}

// TestClaimRefreshedEvent tests ClaimRefreshed event creation and serialization.
func TestClaimRefreshedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1.5")
		newTimeout := types.Now()
		event := NewClaimRefreshed(nodeID, "agent-prover-42", newTimeout)

		if event.Type() != EventClaimRefreshed {
			t.Errorf("Type() = %q, want %q", event.Type(), EventClaimRefreshed)
		}
		if event.NodeID.String() != "1.5" {
			t.Errorf("NodeID = %q, want %q", event.NodeID.String(), "1.5")
		}
		if event.Owner != "agent-prover-42" {
			t.Errorf("Owner = %q, want %q", event.Owner, "agent-prover-42")
		}
		if event.NewTimeout.IsZero() {
			t.Error("NewTimeout should not be zero")
		}
		if event.Timestamp().IsZero() {
			t.Error("Timestamp should not be zero")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		nodeID, _ := types.Parse("1.6.7")
		newTimeout := types.Now()
		original := NewClaimRefreshed(nodeID, "verifier-99", newTimeout)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded ClaimRefreshed
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Type() != original.Type() {
			t.Errorf("Type mismatch: got %q, want %q", decoded.Type(), original.Type())
		}
		if decoded.NodeID.String() != original.NodeID.String() {
			t.Errorf("NodeID mismatch: got %q, want %q", decoded.NodeID.String(), original.NodeID.String())
		}
		if decoded.Owner != original.Owner {
			t.Errorf("Owner mismatch: got %q, want %q", decoded.Owner, original.Owner)
		}
		if !decoded.NewTimeout.Equal(original.NewTimeout) {
			t.Errorf("NewTimeout mismatch: got %v, want %v", decoded.NewTimeout, original.NewTimeout)
		}
	})

	t.Run("JSON fields", func(t *testing.T) {
		nodeID, _ := types.Parse("1")
		event := NewClaimRefreshed(nodeID, "owner", types.Now())
		data, _ := json.Marshal(event)
		jsonStr := string(data)

		expectedFields := []string{`"type"`, `"timestamp"`, `"node_id"`, `"owner"`, `"new_timeout"`}
		for _, field := range expectedFields {
			if !contains(jsonStr, field) {
				t.Errorf("JSON missing field %s: %s", field, jsonStr)
			}
		}
	})
}

// TestRefinementRequestedEvent tests RefinementRequested event creation and serialization.
func TestRefinementRequestedEvent(t *testing.T) {
	t.Run("creation with valid data", func(t *testing.T) {
		nodeID, _ := types.Parse("1.2")
		event := NewRefinementRequested(nodeID, "Needs more detail on step 2", "verifier-42")

		if event.Type() != EventRefinementRequested {
			t.Errorf("Type() = %q, want %q", event.Type(), EventRefinementRequested)
		}
		if event.NodeID.String() != "1.2" {
			t.Errorf("NodeID = %q, want %q", event.NodeID.String(), "1.2")
		}
		if event.Reason != "Needs more detail on step 2" {
			t.Errorf("Reason = %q, want %q", event.Reason, "Needs more detail on step 2")
		}
		if event.RequestedBy != "verifier-42" {
			t.Errorf("RequestedBy = %q, want %q", event.RequestedBy, "verifier-42")
		}
		if event.Timestamp().IsZero() {
			t.Error("Timestamp should not be zero")
		}
	})

	t.Run("creation with empty RequestedBy", func(t *testing.T) {
		nodeID, _ := types.Parse("1.3")
		event := NewRefinementRequested(nodeID, "Clarification needed", "")

		if event.Type() != EventRefinementRequested {
			t.Errorf("Type() = %q, want %q", event.Type(), EventRefinementRequested)
		}
		if event.RequestedBy != "" {
			t.Errorf("RequestedBy = %q, want empty string", event.RequestedBy)
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		nodeID, _ := types.Parse("1.4.5")
		original := NewRefinementRequested(nodeID, "More justification required", "verifier-99")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded RefinementRequested
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Type() != original.Type() {
			t.Errorf("Type mismatch: got %q, want %q", decoded.Type(), original.Type())
		}
		if decoded.NodeID.String() != original.NodeID.String() {
			t.Errorf("NodeID mismatch: got %q, want %q", decoded.NodeID.String(), original.NodeID.String())
		}
		if decoded.Reason != original.Reason {
			t.Errorf("Reason mismatch: got %q, want %q", decoded.Reason, original.Reason)
		}
		if decoded.RequestedBy != original.RequestedBy {
			t.Errorf("RequestedBy mismatch: got %q, want %q", decoded.RequestedBy, original.RequestedBy)
		}
		if !decoded.Timestamp().Equal(original.Timestamp()) {
			t.Errorf("Timestamp mismatch: got %v, want %v", decoded.Timestamp(), original.Timestamp())
		}
	})

	t.Run("JSON fields", func(t *testing.T) {
		nodeID, _ := types.Parse("1")
		event := NewRefinementRequested(nodeID, "reason", "requester")
		data, _ := json.Marshal(event)
		jsonStr := string(data)

		expectedFields := []string{`"type"`, `"timestamp"`, `"node_id"`, `"reason"`, `"requested_by"`}
		for _, field := range expectedFields {
			if !contains(jsonStr, field) {
				t.Errorf("JSON missing field %s: %s", field, jsonStr)
			}
		}
	})
}
