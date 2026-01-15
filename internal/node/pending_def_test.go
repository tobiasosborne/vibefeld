package node_test

import (
	"encoding/json"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/types"
)

// TestNewPendingDef_Valid verifies that NewPendingDef creates a valid pending definition request.
func TestNewPendingDef_Valid(t *testing.T) {
	tests := []struct {
		name        string
		term        string
		requestedBy string
	}{
		{"simple term", "group", "1"},
		{"multi-word term", "abelian group", "1.1"},
		{"mathematical term", "homomorphism", "1.2.3"},
		{"term with symbols", "Z/nZ", "1.1.1"},
		{"unicode term", "epsilon-delta", "1.2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestedBy, err := types.Parse(tt.requestedBy)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.requestedBy, err)
			}

			pd, err := node.NewPendingDef(tt.term, requestedBy)
			if err != nil {
				t.Fatalf("NewPendingDef() unexpected error: %v", err)
			}

			// Verify ID is non-empty
			if pd.ID == "" {
				t.Error("NewPendingDef().ID is empty, want non-empty")
			}

			// Verify term is set correctly
			if pd.Term != tt.term {
				t.Errorf("NewPendingDef().Term = %q, want %q", pd.Term, tt.term)
			}

			// Verify requestedBy is set correctly
			if pd.RequestedBy.String() != tt.requestedBy {
				t.Errorf("NewPendingDef().RequestedBy = %q, want %q", pd.RequestedBy.String(), tt.requestedBy)
			}

			// Verify created timestamp is not zero
			if pd.Created.IsZero() {
				t.Error("NewPendingDef().Created is zero, want non-zero timestamp")
			}

			// Verify status is pending
			if pd.Status != node.PendingDefStatusPending {
				t.Errorf("NewPendingDef().Status = %q, want %q", pd.Status, node.PendingDefStatusPending)
			}

			// Verify resolvedBy is empty
			if pd.ResolvedBy != "" {
				t.Errorf("NewPendingDef().ResolvedBy = %q, want empty", pd.ResolvedBy)
			}
		})
	}
}

// TestNewPendingDef_UniqueIDs verifies that each call to NewPendingDef generates a unique ID.
func TestNewPendingDef_UniqueIDs(t *testing.T) {
	requestedBy, err := types.Parse("1")
	if err != nil {
		t.Fatalf("Parse(\"1\") unexpected error: %v", err)
	}

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		pd, err := node.NewPendingDef("test term", requestedBy)
		if err != nil {
			t.Fatalf("NewPendingDef() iteration %d unexpected error: %v", i, err)
		}
		if ids[pd.ID] {
			t.Errorf("NewPendingDef generated duplicate ID: %s", pd.ID)
		}
		ids[pd.ID] = true
	}
}

// TestPendingDef_Resolve verifies the Resolve method transitions status correctly.
func TestPendingDef_Resolve(t *testing.T) {
	tests := []struct {
		name         string
		definitionID string
	}{
		{"simple ID", "def-001"},
		{"uuid-like ID", "abc123-def456"},
		{"hash ID", "a1b2c3d4e5f6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestedBy, _ := types.Parse("1.1")
			pd, err := node.NewPendingDef("group", requestedBy)
			if err != nil {
				t.Fatalf("NewPendingDef() unexpected error: %v", err)
			}

			// Verify initial state
			if pd.Status != node.PendingDefStatusPending {
				t.Fatalf("initial Status = %q, want %q", pd.Status, node.PendingDefStatusPending)
			}

			// Resolve the pending definition
			err = pd.Resolve(tt.definitionID)
			if err != nil {
				t.Fatalf("Resolve(%q) unexpected error: %v", tt.definitionID, err)
			}

			// Verify status changed to resolved
			if pd.Status != node.PendingDefStatusResolved {
				t.Errorf("after Resolve(), Status = %q, want %q", pd.Status, node.PendingDefStatusResolved)
			}

			// Verify resolvedBy is set
			if pd.ResolvedBy != tt.definitionID {
				t.Errorf("after Resolve(), ResolvedBy = %q, want %q", pd.ResolvedBy, tt.definitionID)
			}
		})
	}
}

// TestPendingDef_Resolve_AlreadyResolved verifies Resolve fails if already resolved.
func TestPendingDef_Resolve_AlreadyResolved(t *testing.T) {
	requestedBy, _ := types.Parse("1")
	pd, err := node.NewPendingDef("group", requestedBy)
	if err != nil {
		t.Fatalf("NewPendingDef() unexpected error: %v", err)
	}

	// Resolve once
	err = pd.Resolve("def-001")
	if err != nil {
		t.Fatalf("first Resolve() unexpected error: %v", err)
	}

	// Try to resolve again
	err = pd.Resolve("def-002")
	if err == nil {
		t.Error("second Resolve() expected error, got nil")
	}
}

// TestPendingDef_Resolve_AlreadyCancelled verifies Resolve fails if cancelled.
func TestPendingDef_Resolve_AlreadyCancelled(t *testing.T) {
	requestedBy, _ := types.Parse("1")
	pd, err := node.NewPendingDef("group", requestedBy)
	if err != nil {
		t.Fatalf("NewPendingDef() unexpected error: %v", err)
	}

	// Cancel first
	err = pd.Cancel()
	if err != nil {
		t.Fatalf("Cancel() unexpected error: %v", err)
	}

	// Try to resolve
	err = pd.Resolve("def-001")
	if err == nil {
		t.Error("Resolve() after Cancel() expected error, got nil")
	}
}

// TestPendingDef_Resolve_EmptyDefinitionID verifies Resolve fails with empty ID.
func TestPendingDef_Resolve_EmptyDefinitionID(t *testing.T) {
	requestedBy, _ := types.Parse("1")
	pd, err := node.NewPendingDef("group", requestedBy)
	if err != nil {
		t.Fatalf("NewPendingDef() unexpected error: %v", err)
	}

	err = pd.Resolve("")
	if err == nil {
		t.Error("Resolve(\"\") expected error, got nil")
	}
}

// TestPendingDef_Cancel verifies the Cancel method transitions status correctly.
func TestPendingDef_Cancel(t *testing.T) {
	requestedBy, _ := types.Parse("1.2.3")
	pd, err := node.NewPendingDef("homomorphism", requestedBy)
	if err != nil {
		t.Fatalf("NewPendingDef() unexpected error: %v", err)
	}

	// Verify initial state
	if pd.Status != node.PendingDefStatusPending {
		t.Fatalf("initial Status = %q, want %q", pd.Status, node.PendingDefStatusPending)
	}

	// Cancel the pending definition
	err = pd.Cancel()
	if err != nil {
		t.Fatalf("Cancel() unexpected error: %v", err)
	}

	// Verify status changed to cancelled
	if pd.Status != node.PendingDefStatusCancelled {
		t.Errorf("after Cancel(), Status = %q, want %q", pd.Status, node.PendingDefStatusCancelled)
	}

	// ResolvedBy should remain empty
	if pd.ResolvedBy != "" {
		t.Errorf("after Cancel(), ResolvedBy = %q, want empty", pd.ResolvedBy)
	}
}

// TestPendingDef_Cancel_AlreadyResolved verifies Cancel fails if already resolved.
func TestPendingDef_Cancel_AlreadyResolved(t *testing.T) {
	requestedBy, _ := types.Parse("1")
	pd, err := node.NewPendingDef("group", requestedBy)
	if err != nil {
		t.Fatalf("NewPendingDef() unexpected error: %v", err)
	}

	// Resolve first
	err = pd.Resolve("def-001")
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	// Try to cancel
	err = pd.Cancel()
	if err == nil {
		t.Error("Cancel() after Resolve() expected error, got nil")
	}
}

// TestPendingDef_Cancel_AlreadyCancelled verifies Cancel fails if already cancelled.
func TestPendingDef_Cancel_AlreadyCancelled(t *testing.T) {
	requestedBy, _ := types.Parse("1")
	pd, err := node.NewPendingDef("group", requestedBy)
	if err != nil {
		t.Fatalf("NewPendingDef() unexpected error: %v", err)
	}

	// Cancel once
	err = pd.Cancel()
	if err != nil {
		t.Fatalf("first Cancel() unexpected error: %v", err)
	}

	// Try to cancel again
	err = pd.Cancel()
	if err == nil {
		t.Error("second Cancel() expected error, got nil")
	}
}

// TestPendingDef_IsPending verifies the IsPending method.
func TestPendingDef_IsPending(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*node.PendingDef)
		wantPending bool
	}{
		{
			name:        "new pending def is pending",
			setup:       func(pd *node.PendingDef) {},
			wantPending: true,
		},
		{
			name: "resolved pending def is not pending",
			setup: func(pd *node.PendingDef) {
				_ = pd.Resolve("def-001")
			},
			wantPending: false,
		},
		{
			name: "cancelled pending def is not pending",
			setup: func(pd *node.PendingDef) {
				_ = pd.Cancel()
			},
			wantPending: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestedBy, _ := types.Parse("1")
			pd, err := node.NewPendingDef("group", requestedBy)
			if err != nil {
				t.Fatalf("NewPendingDef() unexpected error: %v", err)
			}

			tt.setup(pd)

			got := pd.IsPending()
			if got != tt.wantPending {
				t.Errorf("IsPending() = %v, want %v", got, tt.wantPending)
			}
		})
	}
}

// TestPendingDef_JSON_Roundtrip verifies JSON serialization and deserialization.
func TestPendingDef_JSON_Roundtrip(t *testing.T) {
	tests := []struct {
		name        string
		term        string
		requestedBy string
		setup       func(*node.PendingDef)
	}{
		{
			name:        "pending state",
			term:        "group",
			requestedBy: "1",
			setup:       func(pd *node.PendingDef) {},
		},
		{
			name:        "resolved state",
			term:        "homomorphism",
			requestedBy: "1.2.3",
			setup: func(pd *node.PendingDef) {
				_ = pd.Resolve("def-abc123")
			},
		},
		{
			name:        "cancelled state",
			term:        "isomorphism",
			requestedBy: "1.1",
			setup: func(pd *node.PendingDef) {
				_ = pd.Cancel()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestedBy, err := types.Parse(tt.requestedBy)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.requestedBy, err)
			}

			original, err := node.NewPendingDef(tt.term, requestedBy)
			if err != nil {
				t.Fatalf("NewPendingDef() unexpected error: %v", err)
			}
			tt.setup(original)

			// Marshal to JSON
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("json.Marshal() unexpected error: %v", err)
			}

			// Unmarshal from JSON
			var restored node.PendingDef
			err = json.Unmarshal(data, &restored)
			if err != nil {
				t.Fatalf("json.Unmarshal() unexpected error: %v", err)
			}

			// Verify all fields match
			if restored.ID != original.ID {
				t.Errorf("ID: got %q, want %q", restored.ID, original.ID)
			}
			if restored.Term != original.Term {
				t.Errorf("Term: got %q, want %q", restored.Term, original.Term)
			}
			if restored.RequestedBy.String() != original.RequestedBy.String() {
				t.Errorf("RequestedBy: got %q, want %q", restored.RequestedBy.String(), original.RequestedBy.String())
			}
			if !restored.Created.Equal(original.Created) {
				t.Errorf("Created: got %v, want %v", restored.Created, original.Created)
			}
			if restored.ResolvedBy != original.ResolvedBy {
				t.Errorf("ResolvedBy: got %q, want %q", restored.ResolvedBy, original.ResolvedBy)
			}
			if restored.Status != original.Status {
				t.Errorf("Status: got %q, want %q", restored.Status, original.Status)
			}
		})
	}
}

// TestPendingDef_JSON_Fields verifies JSON output contains expected field names.
func TestPendingDef_JSON_Fields(t *testing.T) {
	requestedBy, _ := types.Parse("1.2")
	pd, err := node.NewPendingDef("group", requestedBy)
	if err != nil {
		t.Fatalf("NewPendingDef() unexpected error: %v", err)
	}
	_ = pd.Resolve("def-001")

	data, err := json.Marshal(pd)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	// Parse as generic map to check field names
	var fields map[string]interface{}
	err = json.Unmarshal(data, &fields)
	if err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	expectedFields := []string{"id", "term", "requested_by", "created", "resolved_by", "status"}
	for _, field := range expectedFields {
		if _, ok := fields[field]; !ok {
			t.Errorf("JSON missing expected field %q", field)
		}
	}
}

// TestNewPendingDef_Validation_EmptyTerm verifies that empty term is rejected.
func TestNewPendingDef_Validation_EmptyTerm(t *testing.T) {
	requestedBy, _ := types.Parse("1")

	// Using NewPendingDefWithValidation or checking validation
	pd, err := node.NewPendingDefWithValidation("", requestedBy)
	if err == nil {
		t.Error("NewPendingDefWithValidation(\"\", ...) expected error, got nil")
	}
	if pd != nil {
		t.Error("NewPendingDefWithValidation(\"\", ...) expected nil result on error")
	}
}

// TestNewPendingDef_Validation_WhitespaceTerm verifies that whitespace-only term is rejected.
func TestNewPendingDef_Validation_WhitespaceTerm(t *testing.T) {
	requestedBy, _ := types.Parse("1")

	tests := []string{
		" ",
		"  ",
		"\t",
		"\n",
		"   \t\n  ",
	}

	for _, term := range tests {
		t.Run("whitespace", func(t *testing.T) {
			pd, err := node.NewPendingDefWithValidation(term, requestedBy)
			if err == nil {
				t.Errorf("NewPendingDefWithValidation(%q, ...) expected error, got nil", term)
			}
			if pd != nil {
				t.Error("NewPendingDefWithValidation expected nil result on error")
			}
		})
	}
}

// TestNewPendingDef_Validation_ZeroNodeID verifies that zero NodeID is rejected.
func TestNewPendingDef_Validation_ZeroNodeID(t *testing.T) {
	var zeroNodeID types.NodeID

	pd, err := node.NewPendingDefWithValidation("group", zeroNodeID)
	if err == nil {
		t.Error("NewPendingDefWithValidation with zero NodeID expected error, got nil")
	}
	if pd != nil {
		t.Error("NewPendingDefWithValidation expected nil result on error")
	}
}

// TestPendingDef_StatusValues verifies the status constants exist.
func TestPendingDef_StatusValues(t *testing.T) {
	// These should be defined as constants
	statuses := []node.PendingDefStatus{
		node.PendingDefStatusPending,
		node.PendingDefStatusResolved,
		node.PendingDefStatusCancelled,
	}

	// Each status should have a non-empty string value
	for _, status := range statuses {
		if string(status) == "" {
			t.Errorf("status constant has empty string value")
		}
	}

	// Statuses should be distinct
	seen := make(map[node.PendingDefStatus]bool)
	for _, status := range statuses {
		if seen[status] {
			t.Errorf("duplicate status value: %q", status)
		}
		seen[status] = true
	}
}

// TestPendingDef_TermPreserved verifies term is preserved exactly as given.
func TestPendingDef_TermPreserved(t *testing.T) {
	tests := []string{
		"simple",
		"with spaces",
		"UPPERCASE",
		"MixedCase",
		"with-dashes",
		"with_underscores",
		"with.dots",
		"123numeric",
		"unicode: \u03b5-\u03b4",
		"quotes: \"quoted\"",
	}

	requestedBy, _ := types.Parse("1")

	for _, term := range tests {
		t.Run(term, func(t *testing.T) {
			pd, err := node.NewPendingDef(term, requestedBy)
			if err != nil {
				t.Fatalf("NewPendingDef() unexpected error: %v", err)
			}
			if pd.Term != term {
				t.Errorf("Term = %q, want %q", pd.Term, term)
			}
		})
	}
}
