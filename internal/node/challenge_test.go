package node_test

import (
	"encoding/json"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestNewChallenge_RequiredFields verifies the constructor creates a challenge with all required fields
func TestNewChallenge_RequiredFields(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		targetID string
		target   schema.ChallengeTarget
		reason   string
	}{
		{
			name:     "statement challenge",
			id:       "challenge-001",
			targetID: "1.1",
			target:   schema.TargetStatement,
			reason:   "The claim lacks sufficient justification",
		},
		{
			name:     "inference challenge",
			id:       "challenge-002",
			targetID: "1.2.3",
			target:   schema.TargetInference,
			reason:   "The inference type does not match the argument structure",
		},
		{
			name:     "gap challenge on root",
			id:       "challenge-003",
			targetID: "1",
			target:   schema.TargetGap,
			reason:   "There is a logical gap between premises and conclusion",
		},
		{
			name:     "scope challenge",
			id:       "challenge-004",
			targetID: "1.1.1.1",
			target:   schema.TargetScope,
			reason:   "Local assumption escapes its scope",
		},
		{
			name:     "type error challenge",
			id:       "challenge-005",
			targetID: "1.2",
			target:   schema.TargetTypeError,
			reason:   "Real number used where complex was expected",
		},
		{
			name:     "domain challenge",
			id:       "challenge-006",
			targetID: "1.3",
			target:   schema.TargetDomain,
			reason:   "Division by zero not handled",
		},
		{
			name:     "completeness challenge",
			id:       "challenge-007",
			targetID: "1.1.2",
			target:   schema.TargetCompleteness,
			reason:   "Case n=0 is not covered",
		},
		{
			name:     "context challenge",
			id:       "challenge-008",
			targetID: "1.2.1",
			target:   schema.TargetContext,
			reason:   "Referenced definition D1 does not exist",
		},
		{
			name:     "dependencies challenge",
			id:       "challenge-009",
			targetID: "1.1.3",
			target:   schema.TargetDependencies,
			reason:   "Node depends on unvalidated premise",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetID, err := types.Parse(tt.targetID)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.targetID, err)
			}

			ch, err := node.NewChallenge(tt.id, targetID, tt.target, tt.reason)
			if err != nil {
				t.Fatalf("NewChallenge() unexpected error: %v", err)
			}

			// Verify ID
			if ch.ID != tt.id {
				t.Errorf("Challenge.ID = %q, want %q", ch.ID, tt.id)
			}

			// Verify TargetID
			if ch.TargetID.String() != tt.targetID {
				t.Errorf("Challenge.TargetID = %q, want %q", ch.TargetID.String(), tt.targetID)
			}

			// Verify Target
			if ch.Target != tt.target {
				t.Errorf("Challenge.Target = %q, want %q", ch.Target, tt.target)
			}

			// Verify Reason
			if ch.Reason != tt.reason {
				t.Errorf("Challenge.Reason = %q, want %q", ch.Reason, tt.reason)
			}

			// Verify Raised timestamp is set (not zero)
			if ch.Raised.IsZero() {
				t.Error("Challenge.Raised should not be zero")
			}

			// Verify ResolvedAt is zero (not resolved yet)
			if !ch.ResolvedAt.IsZero() {
				t.Error("Challenge.ResolvedAt should be zero for new challenge")
			}

			// Verify Resolution is empty
			if ch.Resolution != "" {
				t.Errorf("Challenge.Resolution = %q, want empty string", ch.Resolution)
			}

			// Verify Status is open
			if ch.Status != node.ChallengeStatusOpen {
				t.Errorf("Challenge.Status = %q, want %q", ch.Status, node.ChallengeStatusOpen)
			}
		})
	}
}

// TestChallenge_Resolve verifies resolving a challenge sets correct fields
func TestChallenge_Resolve(t *testing.T) {
	tests := []struct {
		name       string
		resolution string
	}{
		{
			name:       "simple resolution",
			resolution: "The inference was corrected to use modus ponens",
		},
		{
			name:       "detailed resolution",
			resolution: "The gap was filled by adding intermediate step 1.1.2 which establishes the connection",
		},
		{
			name:       "acknowledgment resolution",
			resolution: "The type error was fixed by adding explicit type cast",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetID, err := types.Parse("1.1")
			if err != nil {
				t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
			}

			ch, err := node.NewChallenge("ch-resolve", targetID, schema.TargetGap, "Initial reason")
			if err != nil {
				t.Fatalf("NewChallenge() unexpected error: %v", err)
			}

			// Resolve the challenge
			err = ch.Resolve(tt.resolution)
			if err != nil {
				t.Fatalf("Challenge.Resolve() unexpected error: %v", err)
			}

			// Verify status changed
			if ch.Status != node.ChallengeStatusResolved {
				t.Errorf("Challenge.Status = %q, want %q", ch.Status, node.ChallengeStatusResolved)
			}

			// Verify resolution text is set
			if ch.Resolution != tt.resolution {
				t.Errorf("Challenge.Resolution = %q, want %q", ch.Resolution, tt.resolution)
			}

			// Verify ResolvedAt is set (not zero)
			if ch.ResolvedAt.IsZero() {
				t.Error("Challenge.ResolvedAt should not be zero after resolve")
			}

			// Verify ResolvedAt is after Raised
			if ch.ResolvedAt.Before(ch.Raised) {
				t.Error("Challenge.ResolvedAt should not be before Raised")
			}
		})
	}
}

// TestChallenge_Resolve_AlreadyResolved verifies resolving already-resolved challenge returns error
func TestChallenge_Resolve_AlreadyResolved(t *testing.T) {
	targetID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
	}

	ch, err := node.NewChallenge("ch-double", targetID, schema.TargetStatement, "Reason")
	if err != nil {
		t.Fatalf("NewChallenge() unexpected error: %v", err)
	}

	// First resolve should succeed
	err = ch.Resolve("First resolution")
	if err != nil {
		t.Fatalf("First Resolve() unexpected error: %v", err)
	}

	// Second resolve should fail
	err = ch.Resolve("Second resolution")
	if err == nil {
		t.Error("Second Resolve() expected error, got nil")
	}
}

// TestChallenge_Resolve_AfterWithdraw verifies resolving withdrawn challenge returns error
func TestChallenge_Resolve_AfterWithdraw(t *testing.T) {
	targetID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
	}

	ch, err := node.NewChallenge("ch-withdraw-resolve", targetID, schema.TargetInference, "Reason")
	if err != nil {
		t.Fatalf("NewChallenge() unexpected error: %v", err)
	}

	// Withdraw first
	err = ch.Withdraw()
	if err != nil {
		t.Fatalf("Withdraw() unexpected error: %v", err)
	}

	// Resolve should fail
	err = ch.Resolve("Resolution after withdraw")
	if err == nil {
		t.Error("Resolve() after Withdraw() expected error, got nil")
	}
}

// TestChallenge_Withdraw verifies withdrawing a challenge sets correct fields
func TestChallenge_Withdraw(t *testing.T) {
	targetID, err := types.Parse("1.2")
	if err != nil {
		t.Fatalf("Parse(\"1.2\") unexpected error: %v", err)
	}

	ch, err := node.NewChallenge("ch-withdraw", targetID, schema.TargetContext, "Context is missing")
	if err != nil {
		t.Fatalf("NewChallenge() unexpected error: %v", err)
	}

	// Withdraw the challenge
	err = ch.Withdraw()
	if err != nil {
		t.Fatalf("Challenge.Withdraw() unexpected error: %v", err)
	}

	// Verify status changed
	if ch.Status != node.ChallengeStatusWithdrawn {
		t.Errorf("Challenge.Status = %q, want %q", ch.Status, node.ChallengeStatusWithdrawn)
	}

	// Verify ResolvedAt is set (marks when withdrawn)
	if ch.ResolvedAt.IsZero() {
		t.Error("Challenge.ResolvedAt should not be zero after withdraw")
	}
}

// TestChallenge_Withdraw_AlreadyWithdrawn verifies withdrawing already-withdrawn challenge returns error
func TestChallenge_Withdraw_AlreadyWithdrawn(t *testing.T) {
	targetID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
	}

	ch, err := node.NewChallenge("ch-double-withdraw", targetID, schema.TargetGap, "Gap exists")
	if err != nil {
		t.Fatalf("NewChallenge() unexpected error: %v", err)
	}

	// First withdraw should succeed
	err = ch.Withdraw()
	if err != nil {
		t.Fatalf("First Withdraw() unexpected error: %v", err)
	}

	// Second withdraw should fail
	err = ch.Withdraw()
	if err == nil {
		t.Error("Second Withdraw() expected error, got nil")
	}
}

// TestChallenge_Withdraw_AfterResolve verifies withdrawing resolved challenge returns error
func TestChallenge_Withdraw_AfterResolve(t *testing.T) {
	targetID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
	}

	ch, err := node.NewChallenge("ch-resolve-withdraw", targetID, schema.TargetScope, "Scope issue")
	if err != nil {
		t.Fatalf("NewChallenge() unexpected error: %v", err)
	}

	// Resolve first
	err = ch.Resolve("Issue was fixed")
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	// Withdraw should fail
	err = ch.Withdraw()
	if err == nil {
		t.Error("Withdraw() after Resolve() expected error, got nil")
	}
}

// TestChallenge_IsOpen verifies IsOpen returns correct value for each status
func TestChallenge_IsOpen(t *testing.T) {
	tests := []struct {
		name     string
		action   string // "none", "resolve", "withdraw"
		wantOpen bool
	}{
		{
			name:     "new challenge is open",
			action:   "none",
			wantOpen: true,
		},
		{
			name:     "resolved challenge is not open",
			action:   "resolve",
			wantOpen: false,
		},
		{
			name:     "withdrawn challenge is not open",
			action:   "withdraw",
			wantOpen: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetID, err := types.Parse("1.1")
			if err != nil {
				t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
			}

			ch, err := node.NewChallenge("ch-isopen", targetID, schema.TargetStatement, "Reason")
			if err != nil {
				t.Fatalf("NewChallenge() unexpected error: %v", err)
			}

			switch tt.action {
			case "resolve":
				if err := ch.Resolve("Resolution"); err != nil {
					t.Fatalf("Resolve() unexpected error: %v", err)
				}
			case "withdraw":
				if err := ch.Withdraw(); err != nil {
					t.Fatalf("Withdraw() unexpected error: %v", err)
				}
			}

			if got := ch.IsOpen(); got != tt.wantOpen {
				t.Errorf("Challenge.IsOpen() = %v, want %v", got, tt.wantOpen)
			}
		})
	}
}

// TestChallenge_JSONRoundtrip verifies JSON serialization and deserialization
func TestChallenge_JSONRoundtrip(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		target schema.ChallengeTarget
		reason string
	}{
		{
			name:   "statement challenge",
			id:     "ch-json-001",
			target: schema.TargetStatement,
			reason: "Statement is unclear",
		},
		{
			name:   "gap challenge with special chars",
			id:     "ch-json-002",
			target: schema.TargetGap,
			reason: "Gap between steps: A -> B requires clarification",
		},
		{
			name:   "challenge with unicode",
			id:     "ch-json-003",
			target: schema.TargetTypeError,
			reason: "Type mismatch: expected R but got Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetID, err := types.Parse("1.2.3")
			if err != nil {
				t.Fatalf("Parse(\"1.2.3\") unexpected error: %v", err)
			}

			original, err := node.NewChallenge(tt.id, targetID, tt.target, tt.reason)
			if err != nil {
				t.Fatalf("NewChallenge() unexpected error: %v", err)
			}

			// Serialize to JSON
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("json.Marshal() unexpected error: %v", err)
			}

			// Deserialize from JSON
			var decoded node.Challenge
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Fatalf("json.Unmarshal() unexpected error: %v", err)
			}

			// Verify fields match
			if decoded.ID != original.ID {
				t.Errorf("decoded.ID = %q, want %q", decoded.ID, original.ID)
			}
			if decoded.TargetID.String() != original.TargetID.String() {
				t.Errorf("decoded.TargetID = %q, want %q", decoded.TargetID.String(), original.TargetID.String())
			}
			if decoded.Target != original.Target {
				t.Errorf("decoded.Target = %q, want %q", decoded.Target, original.Target)
			}
			if decoded.Reason != original.Reason {
				t.Errorf("decoded.Reason = %q, want %q", decoded.Reason, original.Reason)
			}
			if decoded.Status != original.Status {
				t.Errorf("decoded.Status = %q, want %q", decoded.Status, original.Status)
			}
			if !decoded.Raised.Equal(original.Raised) {
				t.Errorf("decoded.Raised = %v, want %v", decoded.Raised, original.Raised)
			}
		})
	}
}

// TestChallenge_JSONRoundtrip_Resolved verifies resolved challenge JSON serialization
func TestChallenge_JSONRoundtrip_Resolved(t *testing.T) {
	targetID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
	}

	original, err := node.NewChallenge("ch-json-resolved", targetID, schema.TargetInference, "Inference is wrong")
	if err != nil {
		t.Fatalf("NewChallenge() unexpected error: %v", err)
	}

	// Resolve the challenge
	err = original.Resolve("Fixed by correcting inference type")
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	// Serialize to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	// Deserialize from JSON
	var decoded node.Challenge
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	// Verify resolved fields
	if decoded.Status != node.ChallengeStatusResolved {
		t.Errorf("decoded.Status = %q, want %q", decoded.Status, node.ChallengeStatusResolved)
	}
	if decoded.Resolution != original.Resolution {
		t.Errorf("decoded.Resolution = %q, want %q", decoded.Resolution, original.Resolution)
	}
	if decoded.ResolvedAt.IsZero() {
		t.Error("decoded.ResolvedAt should not be zero")
	}
	if !decoded.ResolvedAt.Equal(original.ResolvedAt) {
		t.Errorf("decoded.ResolvedAt = %v, want %v", decoded.ResolvedAt, original.ResolvedAt)
	}
}

// TestNewChallenge_Validation_EmptyReason verifies empty reason is rejected
func TestNewChallenge_Validation_EmptyReason(t *testing.T) {
	tests := []struct {
		name   string
		reason string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs only", "\t\t"},
		{"newlines only", "\n\n"},
		{"mixed whitespace", " \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetID, err := types.Parse("1.1")
			if err != nil {
				t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
			}

			_, err = node.NewChallenge("ch-empty", targetID, schema.TargetStatement, tt.reason)
			if err == nil {
				t.Error("NewChallenge() with empty reason expected error, got nil")
			}
		})
	}
}

// TestNewChallenge_Validation_EmptyID verifies empty ID is rejected
func TestNewChallenge_Validation_EmptyID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetID, err := types.Parse("1.1")
			if err != nil {
				t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
			}

			_, err = node.NewChallenge(tt.id, targetID, schema.TargetStatement, "Valid reason")
			if err == nil {
				t.Error("NewChallenge() with empty ID expected error, got nil")
			}
		})
	}
}

// TestNewChallenge_Validation_InvalidTarget verifies invalid target is rejected
func TestNewChallenge_Validation_InvalidTarget(t *testing.T) {
	tests := []struct {
		name   string
		target schema.ChallengeTarget
	}{
		{"empty target", schema.ChallengeTarget("")},
		{"invalid target", schema.ChallengeTarget("invalid")},
		{"unknown target", schema.ChallengeTarget("unknown_target")},
		{"typo in target", schema.ChallengeTarget("statment")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetID, err := types.Parse("1.1")
			if err != nil {
				t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
			}

			_, err = node.NewChallenge("ch-invalid", targetID, tt.target, "Valid reason")
			if err == nil {
				t.Errorf("NewChallenge() with invalid target %q expected error, got nil", tt.target)
			}
		})
	}
}

// TestChallenge_Resolve_EmptyResolution verifies empty resolution is rejected
func TestChallenge_Resolve_EmptyResolution(t *testing.T) {
	tests := []struct {
		name       string
		resolution string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs only", "\t\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetID, err := types.Parse("1.1")
			if err != nil {
				t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
			}

			ch, err := node.NewChallenge("ch-empty-res", targetID, schema.TargetGap, "Valid reason")
			if err != nil {
				t.Fatalf("NewChallenge() unexpected error: %v", err)
			}

			err = ch.Resolve(tt.resolution)
			if err == nil {
				t.Error("Resolve() with empty resolution expected error, got nil")
			}

			// Status should still be open
			if ch.Status != node.ChallengeStatusOpen {
				t.Errorf("Challenge.Status = %q after failed resolve, want %q", ch.Status, node.ChallengeStatusOpen)
			}
		})
	}
}

// TestChallengeStatus_Values verifies status constants have expected values
func TestChallengeStatus_Values(t *testing.T) {
	// Verify status values exist and are distinct
	statuses := []node.ChallengeStatus{
		node.ChallengeStatusOpen,
		node.ChallengeStatusResolved,
		node.ChallengeStatusWithdrawn,
	}

	seen := make(map[node.ChallengeStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("Duplicate status value: %q", s)
		}
		seen[s] = true

		// Verify status is not empty
		if s == "" {
			t.Error("Status should not be empty string")
		}
	}
}

// TestChallenge_ZeroValue verifies zero value behavior
func TestChallenge_ZeroValue(t *testing.T) {
	var ch node.Challenge

	// Zero value should have sensible defaults
	if ch.ID != "" {
		t.Errorf("Zero Challenge.ID = %q, want empty string", ch.ID)
	}

	if ch.Reason != "" {
		t.Errorf("Zero Challenge.Reason = %q, want empty string", ch.Reason)
	}

	// IsOpen on zero value should return consistent result
	_ = ch.IsOpen() // Should not panic
}

// TestChallenge_AllTargetTypes verifies challenges can be created for all valid target types
func TestChallenge_AllTargetTypes(t *testing.T) {
	allTargets := []schema.ChallengeTarget{
		schema.TargetStatement,
		schema.TargetInference,
		schema.TargetContext,
		schema.TargetDependencies,
		schema.TargetScope,
		schema.TargetGap,
		schema.TargetTypeError,
		schema.TargetDomain,
		schema.TargetCompleteness,
	}

	targetID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("Parse(\"1.1\") unexpected error: %v", err)
	}

	for _, target := range allTargets {
		t.Run(string(target), func(t *testing.T) {
			ch, err := node.NewChallenge("ch-all-targets", targetID, target, "Test reason for "+string(target))
			if err != nil {
				t.Errorf("NewChallenge() with target %q unexpected error: %v", target, err)
				return
			}

			if ch.Target != target {
				t.Errorf("Challenge.Target = %q, want %q", ch.Target, target)
			}
		})
	}
}

// TestChallenge_JSONFormat verifies JSON output has expected structure
func TestChallenge_JSONFormat(t *testing.T) {
	targetID, err := types.Parse("1.2")
	if err != nil {
		t.Fatalf("Parse(\"1.2\") unexpected error: %v", err)
	}

	ch, err := node.NewChallenge("ch-format", targetID, schema.TargetStatement, "Test reason")
	if err != nil {
		t.Fatalf("NewChallenge() unexpected error: %v", err)
	}

	data, err := json.Marshal(ch)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	// Parse as generic map to check field names
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal() to map unexpected error: %v", err)
	}

	// Verify expected fields exist
	expectedFields := []string{"id", "target_id", "target", "reason", "raised", "status"}
	for _, field := range expectedFields {
		if _, exists := m[field]; !exists {
			t.Errorf("JSON missing expected field %q", field)
		}
	}
}
