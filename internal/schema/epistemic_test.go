package schema

import (
	"testing"
)

func TestValidateEpistemicState_AllValid(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{"pending", "pending"},
		{"validated", "validated"},
		{"admitted", "admitted"},
		{"refuted", "refuted"},
		{"archived", "archived"},
		{"needs_refinement", "needs_refinement"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEpistemicState(tt.state)
			if err != nil {
				t.Errorf("ValidateEpistemicState(%q) returned error: %v", tt.state, err)
			}
		})
	}
}

func TestValidateEpistemicState_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{"random string", "foo"},
		{"empty string", ""},
		{"uppercase", "PENDING"},
		{"mixed case", "Validated"},
		{"typo", "pendin"},
		{"plural", "pendings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEpistemicState(tt.state)
			if err == nil {
				t.Errorf("ValidateEpistemicState(%q) expected error, got nil", tt.state)
			}
		})
	}
}

func TestGetEpistemicStateInfo_Exists(t *testing.T) {
	tests := []struct {
		name               string
		state              EpistemicState
		expectedDesc       string
		expectedFinal      bool
		expectedTaint      bool
	}{
		{
			name:          "pending",
			state:         EpistemicPending,
			expectedDesc:  "Awaiting verification",
			expectedFinal: false,
			expectedTaint: false,
		},
		{
			name:          "validated",
			state:         EpistemicValidated,
			expectedDesc:  "Verified correct by verifier",
			expectedFinal: true,
			expectedTaint: false,
		},
		{
			name:          "admitted",
			state:         EpistemicAdmitted,
			expectedDesc:  "Accepted without full verification (introduces taint)",
			expectedFinal: true,
			expectedTaint: true,
		},
		{
			name:          "refuted",
			state:         EpistemicRefuted,
			expectedDesc:  "Proven incorrect",
			expectedFinal: true,
			expectedTaint: false,
		},
		{
			name:          "archived",
			state:         EpistemicArchived,
			expectedDesc:  "No longer relevant (branch abandoned)",
			expectedFinal: true,
			expectedTaint: false,
		},
		{
			name:          "needs_refinement",
			state:         EpistemicNeedsRefinement,
			expectedDesc:  "Validated node reopened for refinement",
			expectedFinal: false,
			expectedTaint: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := GetEpistemicStateInfo(tt.state)
			if !ok {
				t.Fatalf("GetEpistemicStateInfo(%q) returned false, expected true", tt.state)
			}
			if info.ID != tt.state {
				t.Errorf("info.ID = %q, want %q", info.ID, tt.state)
			}
			if info.Description != tt.expectedDesc {
				t.Errorf("info.Description = %q, want %q", info.Description, tt.expectedDesc)
			}
			if info.IsFinal != tt.expectedFinal {
				t.Errorf("info.IsFinal = %v, want %v", info.IsFinal, tt.expectedFinal)
			}
			if info.IntroducesTaint != tt.expectedTaint {
				t.Errorf("info.IntroducesTaint = %v, want %v", info.IntroducesTaint, tt.expectedTaint)
			}
		})
	}
}

func TestGetEpistemicStateInfo_NotExists(t *testing.T) {
	tests := []struct {
		name  string
		state EpistemicState
	}{
		{"empty", ""},
		{"invalid", "foo"},
		{"uppercase", "PENDING"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := GetEpistemicStateInfo(tt.state)
			if ok {
				t.Errorf("GetEpistemicStateInfo(%q) returned true, expected false", tt.state)
			}
		})
	}
}

func TestAllEpistemicStates_Count(t *testing.T) {
	states := AllEpistemicStates()
	if len(states) != 6 {
		t.Errorf("AllEpistemicStates() returned %d states, want 6", len(states))
	}

	// Verify all expected states are present
	expectedStates := map[EpistemicState]bool{
		EpistemicPending:         false,
		EpistemicValidated:       false,
		EpistemicAdmitted:        false,
		EpistemicRefuted:         false,
		EpistemicArchived:        false,
		EpistemicNeedsRefinement: false,
	}

	for _, info := range states {
		if _, exists := expectedStates[info.ID]; exists {
			expectedStates[info.ID] = true
		}
	}

	for state, found := range expectedStates {
		if !found {
			t.Errorf("AllEpistemicStates() missing state: %q", state)
		}
	}
}

func TestValidateEpistemicTransition_PendingToValidated(t *testing.T) {
	err := ValidateEpistemicTransition(EpistemicPending, EpistemicValidated)
	if err != nil {
		t.Errorf("ValidateEpistemicTransition(pending, validated) returned error: %v", err)
	}
}

func TestValidateEpistemicTransition_PendingToAdmitted(t *testing.T) {
	err := ValidateEpistemicTransition(EpistemicPending, EpistemicAdmitted)
	if err != nil {
		t.Errorf("ValidateEpistemicTransition(pending, admitted) returned error: %v", err)
	}
}

func TestValidateEpistemicTransition_PendingToRefuted(t *testing.T) {
	err := ValidateEpistemicTransition(EpistemicPending, EpistemicRefuted)
	if err != nil {
		t.Errorf("ValidateEpistemicTransition(pending, refuted) returned error: %v", err)
	}
}

func TestValidateEpistemicTransition_PendingToArchived(t *testing.T) {
	err := ValidateEpistemicTransition(EpistemicPending, EpistemicArchived)
	if err != nil {
		t.Errorf("ValidateEpistemicTransition(pending, archived) returned error: %v", err)
	}
}

func TestValidateEpistemicTransition_ValidatedToNeedsRefinement(t *testing.T) {
	err := ValidateEpistemicTransition(EpistemicValidated, EpistemicNeedsRefinement)
	if err != nil {
		t.Errorf("ValidateEpistemicTransition(validated, needs_refinement) returned error: %v", err)
	}
}

func TestValidateEpistemicTransition_ValidatedToOther(t *testing.T) {
	// validated can only transition to needs_refinement
	invalidTargets := []EpistemicState{
		EpistemicPending,
		EpistemicValidated,
		EpistemicAdmitted,
		EpistemicRefuted,
		EpistemicArchived,
	}

	for _, target := range invalidTargets {
		t.Run(string(target), func(t *testing.T) {
			err := ValidateEpistemicTransition(EpistemicValidated, target)
			if err == nil {
				t.Errorf("ValidateEpistemicTransition(validated, %q) expected error, got nil", target)
			}
		})
	}
}

func TestValidateEpistemicTransition_AdmittedToAny(t *testing.T) {
	targets := []EpistemicState{
		EpistemicPending,
		EpistemicValidated,
		EpistemicAdmitted,
		EpistemicRefuted,
		EpistemicArchived,
	}

	for _, target := range targets {
		t.Run(string(target), func(t *testing.T) {
			err := ValidateEpistemicTransition(EpistemicAdmitted, target)
			if err == nil {
				t.Errorf("ValidateEpistemicTransition(admitted, %q) expected error, got nil", target)
			}
		})
	}
}

func TestValidateEpistemicTransition_RefutedToAny(t *testing.T) {
	targets := []EpistemicState{
		EpistemicPending,
		EpistemicValidated,
		EpistemicAdmitted,
		EpistemicRefuted,
		EpistemicArchived,
	}

	for _, target := range targets {
		t.Run(string(target), func(t *testing.T) {
			err := ValidateEpistemicTransition(EpistemicRefuted, target)
			if err == nil {
				t.Errorf("ValidateEpistemicTransition(refuted, %q) expected error, got nil", target)
			}
		})
	}
}

func TestValidateEpistemicTransition_ArchivedToAny(t *testing.T) {
	targets := []EpistemicState{
		EpistemicPending,
		EpistemicValidated,
		EpistemicAdmitted,
		EpistemicRefuted,
		EpistemicArchived,
	}

	for _, target := range targets {
		t.Run(string(target), func(t *testing.T) {
			err := ValidateEpistemicTransition(EpistemicArchived, target)
			if err == nil {
				t.Errorf("ValidateEpistemicTransition(archived, %q) expected error, got nil", target)
			}
		})
	}
}

func TestValidateEpistemicTransition_NeedsRefinementToValidated(t *testing.T) {
	err := ValidateEpistemicTransition(EpistemicNeedsRefinement, EpistemicValidated)
	if err != nil {
		t.Errorf("ValidateEpistemicTransition(needs_refinement, validated) returned error: %v", err)
	}
}

func TestValidateEpistemicTransition_NeedsRefinementToTerminal(t *testing.T) {
	validTargets := []EpistemicState{
		EpistemicAdmitted,
		EpistemicRefuted,
		EpistemicArchived,
	}

	for _, target := range validTargets {
		t.Run(string(target), func(t *testing.T) {
			err := ValidateEpistemicTransition(EpistemicNeedsRefinement, target)
			if err != nil {
				t.Errorf("ValidateEpistemicTransition(needs_refinement, %q) returned error: %v", target, err)
			}
		})
	}
}

func TestValidateEpistemicTransition_NeedsRefinementToInvalid(t *testing.T) {
	invalidTargets := []EpistemicState{
		EpistemicPending,
		EpistemicNeedsRefinement,
	}

	for _, target := range invalidTargets {
		t.Run(string(target), func(t *testing.T) {
			err := ValidateEpistemicTransition(EpistemicNeedsRefinement, target)
			if err == nil {
				t.Errorf("ValidateEpistemicTransition(needs_refinement, %q) expected error, got nil", target)
			}
		})
	}
}

func TestValidateEpistemicTransition_PendingToPending(t *testing.T) {
	err := ValidateEpistemicTransition(EpistemicPending, EpistemicPending)
	if err == nil {
		t.Error("ValidateEpistemicTransition(pending, pending) expected error, got nil")
	}
}

func TestValidateEpistemicTransition_InvalidFromState(t *testing.T) {
	err := ValidateEpistemicTransition("invalid", EpistemicValidated)
	if err == nil {
		t.Error("ValidateEpistemicTransition(invalid, validated) expected error, got nil")
	}
}

func TestValidateEpistemicTransition_InvalidToState(t *testing.T) {
	err := ValidateEpistemicTransition(EpistemicPending, "invalid")
	if err == nil {
		t.Error("ValidateEpistemicTransition(pending, invalid) expected error, got nil")
	}
}

func TestIsFinal_Pending(t *testing.T) {
	if IsFinal(EpistemicPending) {
		t.Error("IsFinal(pending) returned true, want false")
	}
}

func TestIsFinal_NeedsRefinement(t *testing.T) {
	if IsFinal(EpistemicNeedsRefinement) {
		t.Error("IsFinal(needs_refinement) returned true, want false")
	}
}

func TestIsFinal_Terminal(t *testing.T) {
	tests := []struct {
		name  string
		state EpistemicState
	}{
		{"validated", EpistemicValidated},
		{"admitted", EpistemicAdmitted},
		{"refuted", EpistemicRefuted},
		{"archived", EpistemicArchived},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !IsFinal(tt.state) {
				t.Errorf("IsFinal(%q) returned false, want true", tt.state)
			}
		})
	}
}

func TestIsFinal_InvalidState(t *testing.T) {
	// Invalid state should return false
	if IsFinal("invalid") {
		t.Error("IsFinal(invalid) returned true, want false")
	}
}

func TestIntroducesTaint_Admitted(t *testing.T) {
	if !IntroducesTaint(EpistemicAdmitted) {
		t.Error("IntroducesTaint(admitted) returned false, want true")
	}
}

func TestIntroducesTaint_Others(t *testing.T) {
	tests := []struct {
		name  string
		state EpistemicState
	}{
		{"pending", EpistemicPending},
		{"validated", EpistemicValidated},
		{"refuted", EpistemicRefuted},
		{"archived", EpistemicArchived},
		{"needs_refinement", EpistemicNeedsRefinement},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IntroducesTaint(tt.state) {
				t.Errorf("IntroducesTaint(%q) returned true, want false", tt.state)
			}
		})
	}
}

func TestIntroducesTaint_InvalidState(t *testing.T) {
	// Invalid state should return false
	if IntroducesTaint("invalid") {
		t.Error("IntroducesTaint(invalid) returned true, want false")
	}
}
