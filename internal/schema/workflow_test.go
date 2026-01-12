package schema_test

import (
	"testing"

	"github.com/tobias/vibefeld/internal/schema"
)

func TestValidateWorkflowState_AllValid(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{"available", "available"},
		{"claimed", "claimed"},
		{"blocked", "blocked"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := schema.ValidateWorkflowState(tt.state)
			if err != nil {
				t.Errorf("ValidateWorkflowState(%q) = %v, want nil", tt.state, err)
			}
		})
	}
}

func TestValidateWorkflowState_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{"invalid_foo", "foo"},
		{"empty", ""},
		{"uppercase", "AVAILABLE"},
		{"mixed_case", "Available"},
		{"partial", "claim"},
		{"extra_chars", "available "},
		{"unknown", "pending"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := schema.ValidateWorkflowState(tt.state)
			if err == nil {
				t.Errorf("ValidateWorkflowState(%q) = nil, want error", tt.state)
			}
		})
	}
}

func TestGetWorkflowStateInfo_Exists(t *testing.T) {
	tests := []struct {
		name             string
		state            schema.WorkflowState
		wantDescription  string
	}{
		{
			name:            "available",
			state:           schema.WorkflowAvailable,
			wantDescription: "Node is free for any agent to claim",
		},
		{
			name:            "claimed",
			state:           schema.WorkflowClaimed,
			wantDescription: "Node is currently owned by an agent",
		},
		{
			name:            "blocked",
			state:           schema.WorkflowBlocked,
			wantDescription: "Node cannot be worked on (e.g., awaiting dependency)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := schema.GetWorkflowStateInfo(tt.state)
			if !ok {
				t.Errorf("GetWorkflowStateInfo(%v) returned ok=false, want ok=true", tt.state)
				return
			}
			if info.ID != tt.state {
				t.Errorf("GetWorkflowStateInfo(%v).ID = %v, want %v", tt.state, info.ID, tt.state)
			}
			if info.Description != tt.wantDescription {
				t.Errorf("GetWorkflowStateInfo(%v).Description = %q, want %q",
					tt.state, info.Description, tt.wantDescription)
			}
		})
	}
}

func TestGetWorkflowStateInfo_NotExists(t *testing.T) {
	tests := []struct {
		name  string
		state schema.WorkflowState
	}{
		{"empty", schema.WorkflowState("")},
		{"invalid", schema.WorkflowState("invalid")},
		{"pending", schema.WorkflowState("pending")},
		{"validated", schema.WorkflowState("validated")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := schema.GetWorkflowStateInfo(tt.state)
			if ok {
				t.Errorf("GetWorkflowStateInfo(%v) returned ok=true, want ok=false", tt.state)
			}
		})
	}
}

func TestAllWorkflowStates_Count(t *testing.T) {
	states := schema.AllWorkflowStates()

	if len(states) != 3 {
		t.Errorf("AllWorkflowStates() returned %d states, want 3", len(states))
	}

	// Verify all expected states are present
	stateMap := make(map[schema.WorkflowState]bool)
	for _, state := range states {
		stateMap[state.ID] = true
	}

	expectedStates := []schema.WorkflowState{
		schema.WorkflowAvailable,
		schema.WorkflowClaimed,
		schema.WorkflowBlocked,
	}

	for _, expected := range expectedStates {
		if !stateMap[expected] {
			t.Errorf("AllWorkflowStates() missing state %v", expected)
		}
	}
}

func TestValidateWorkflowTransition_AvailableToClaimed(t *testing.T) {
	err := schema.ValidateWorkflowTransition(schema.WorkflowAvailable, schema.WorkflowClaimed)
	if err != nil {
		t.Errorf("ValidateWorkflowTransition(available, claimed) = %v, want nil (transition allowed)", err)
	}
}

func TestValidateWorkflowTransition_ClaimedToAvailable(t *testing.T) {
	err := schema.ValidateWorkflowTransition(schema.WorkflowClaimed, schema.WorkflowAvailable)
	if err != nil {
		t.Errorf("ValidateWorkflowTransition(claimed, available) = %v, want nil (transition allowed)", err)
	}
}

func TestValidateWorkflowTransition_ClaimedToBlocked(t *testing.T) {
	err := schema.ValidateWorkflowTransition(schema.WorkflowClaimed, schema.WorkflowBlocked)
	if err != nil {
		t.Errorf("ValidateWorkflowTransition(claimed, blocked) = %v, want nil (transition allowed)", err)
	}
}

func TestValidateWorkflowTransition_BlockedToAvailable(t *testing.T) {
	err := schema.ValidateWorkflowTransition(schema.WorkflowBlocked, schema.WorkflowAvailable)
	if err != nil {
		t.Errorf("ValidateWorkflowTransition(blocked, available) = %v, want nil (transition allowed)", err)
	}
}

func TestValidateWorkflowTransition_AvailableToBlocked(t *testing.T) {
	err := schema.ValidateWorkflowTransition(schema.WorkflowAvailable, schema.WorkflowBlocked)
	if err == nil {
		t.Errorf("ValidateWorkflowTransition(available, blocked) = nil, want error (transition not allowed)")
	}
}

func TestValidateWorkflowTransition_BlockedToClaimed(t *testing.T) {
	err := schema.ValidateWorkflowTransition(schema.WorkflowBlocked, schema.WorkflowClaimed)
	if err == nil {
		t.Errorf("ValidateWorkflowTransition(blocked, claimed) = nil, want error (transition not allowed)")
	}
}

func TestValidateWorkflowTransition_InvalidStates(t *testing.T) {
	tests := []struct {
		name string
		from schema.WorkflowState
		to   schema.WorkflowState
	}{
		{"from_empty", schema.WorkflowState(""), schema.WorkflowAvailable},
		{"to_empty", schema.WorkflowAvailable, schema.WorkflowState("")},
		{"from_invalid", schema.WorkflowState("invalid"), schema.WorkflowAvailable},
		{"to_invalid", schema.WorkflowAvailable, schema.WorkflowState("invalid")},
		{"both_invalid", schema.WorkflowState("foo"), schema.WorkflowState("bar")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := schema.ValidateWorkflowTransition(tt.from, tt.to)
			if err == nil {
				t.Errorf("ValidateWorkflowTransition(%v, %v) = nil, want error (invalid state)", tt.from, tt.to)
			}
		})
	}
}

func TestValidateWorkflowTransition_SameState(t *testing.T) {
	tests := []struct {
		name  string
		state schema.WorkflowState
	}{
		{"available_to_available", schema.WorkflowAvailable},
		{"claimed_to_claimed", schema.WorkflowClaimed},
		{"blocked_to_blocked", schema.WorkflowBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := schema.ValidateWorkflowTransition(tt.state, tt.state)
			if err == nil {
				t.Errorf("ValidateWorkflowTransition(%v, %v) = nil, want error (same state transition)", tt.state, tt.state)
			}
		})
	}
}

func TestCanClaim_Available(t *testing.T) {
	result := schema.CanClaim(schema.WorkflowAvailable)
	if !result {
		t.Errorf("CanClaim(available) = false, want true")
	}
}

func TestCanClaim_Claimed(t *testing.T) {
	result := schema.CanClaim(schema.WorkflowClaimed)
	if result {
		t.Errorf("CanClaim(claimed) = true, want false")
	}
}

func TestCanClaim_Blocked(t *testing.T) {
	result := schema.CanClaim(schema.WorkflowBlocked)
	if result {
		t.Errorf("CanClaim(blocked) = true, want false")
	}
}

func TestCanClaim_InvalidState(t *testing.T) {
	tests := []struct {
		name  string
		state schema.WorkflowState
	}{
		{"empty", schema.WorkflowState("")},
		{"invalid", schema.WorkflowState("invalid")},
		{"unknown", schema.WorkflowState("unknown")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := schema.CanClaim(tt.state)
			if result {
				t.Errorf("CanClaim(%v) = true, want false (invalid state)", tt.state)
			}
		})
	}
}
