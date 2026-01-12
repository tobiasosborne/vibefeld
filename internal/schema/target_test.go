package schema

import (
	"testing"
)

// TestValidateChallengeTarget_AllValid tests that all 9 valid challenge targets pass validation.
func TestValidateChallengeTarget_AllValid(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{"statement", "statement"},
		{"inference", "inference"},
		{"context", "context"},
		{"dependencies", "dependencies"},
		{"scope", "scope"},
		{"gap", "gap"},
		{"type_error", "type_error"},
		{"domain", "domain"},
		{"completeness", "completeness"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChallengeTarget(tt.target)
			if err != nil {
				t.Errorf("ValidateChallengeTarget(%q) returned error: %v, want nil", tt.target, err)
			}
		})
	}
}

// TestValidateChallengeTarget_Invalid tests that invalid challenge targets fail validation.
func TestValidateChallengeTarget_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{"unknown_foo", "foo"},
		{"empty_string", ""},
		{"uppercase_STATEMENT", "STATEMENT"},
		{"mixed_case", "Statement"},
		{"invalid_with_space", "type error"},
		{"invalid_hyphen", "type-error"},
		{"random", "xyz123"},
		{"partial_match", "state"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChallengeTarget(tt.target)
			if err == nil {
				t.Errorf("ValidateChallengeTarget(%q) returned nil, want error", tt.target)
			}
		})
	}
}

// TestValidateChallengeTargets_AllValid tests that a list of valid targets passes validation.
func TestValidateChallengeTargets_AllValid(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
	}{
		{
			name:    "single_statement",
			targets: []string{"statement"},
		},
		{
			name:    "two_targets",
			targets: []string{"inference", "gap"},
		},
		{
			name:    "all_nine_targets",
			targets: []string{"statement", "inference", "context", "dependencies", "scope", "gap", "type_error", "domain", "completeness"},
		},
		{
			name:    "duplicates_allowed",
			targets: []string{"statement", "statement", "gap"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChallengeTargets(tt.targets)
			if err != nil {
				t.Errorf("ValidateChallengeTargets(%v) returned error: %v, want nil", tt.targets, err)
			}
		})
	}
}

// TestValidateChallengeTargets_SomeInvalid tests that a list with any invalid target fails validation.
func TestValidateChallengeTargets_SomeInvalid(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
	}{
		{
			name:    "first_invalid",
			targets: []string{"foo", "statement"},
		},
		{
			name:    "last_invalid",
			targets: []string{"statement", "bar"},
		},
		{
			name:    "middle_invalid",
			targets: []string{"statement", "invalid", "gap"},
		},
		{
			name:    "all_invalid",
			targets: []string{"foo", "bar", "baz"},
		},
		{
			name:    "empty_string_in_list",
			targets: []string{"statement", "", "gap"},
		},
		{
			name:    "uppercase_in_list",
			targets: []string{"statement", "INFERENCE", "gap"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChallengeTargets(tt.targets)
			if err == nil {
				t.Errorf("ValidateChallengeTargets(%v) returned nil, want error", tt.targets)
			}
		})
	}
}

// TestValidateChallengeTargets_Empty tests that an empty list fails validation.
func TestValidateChallengeTargets_Empty(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
	}{
		{
			name:    "nil_slice",
			targets: nil,
		},
		{
			name:    "empty_slice",
			targets: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChallengeTargets(tt.targets)
			if err == nil {
				t.Errorf("ValidateChallengeTargets(%v) returned nil, want error for empty list", tt.targets)
			}
		})
	}
}

// TestGetChallengeTargetInfo_Exists tests that valid targets return correct descriptions.
func TestGetChallengeTargetInfo_Exists(t *testing.T) {
	tests := []struct {
		name               string
		target             ChallengeTarget
		wantDescNotEmpty   bool
		wantDescContains   string
	}{
		{
			name:               "statement",
			target:             TargetStatement,
			wantDescNotEmpty:   true,
			wantDescContains:   "claim",
		},
		{
			name:               "inference",
			target:             TargetInference,
			wantDescNotEmpty:   true,
			wantDescContains:   "inference",
		},
		{
			name:               "context",
			target:             TargetContext,
			wantDescNotEmpty:   true,
			wantDescContains:   "definition",
		},
		{
			name:               "dependencies",
			target:             TargetDependencies,
			wantDescNotEmpty:   true,
			wantDescContains:   "dependencies",
		},
		{
			name:               "scope",
			target:             TargetScope,
			wantDescNotEmpty:   true,
			wantDescContains:   "scope",
		},
		{
			name:               "gap",
			target:             TargetGap,
			wantDescNotEmpty:   true,
			wantDescContains:   "gap",
		},
		{
			name:               "type_error",
			target:             TargetTypeError,
			wantDescNotEmpty:   true,
			wantDescContains:   "type",
		},
		{
			name:               "domain",
			target:             TargetDomain,
			wantDescNotEmpty:   true,
			wantDescContains:   "domain",
		},
		{
			name:               "completeness",
			target:             TargetCompleteness,
			wantDescNotEmpty:   true,
			wantDescContains:   "complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, exists := GetChallengeTargetInfo(tt.target)
			if !exists {
				t.Errorf("GetChallengeTargetInfo(%q) returned exists=false, want true", tt.target)
				return
			}
			if info.ID != tt.target {
				t.Errorf("GetChallengeTargetInfo(%q) ID = %q, want %q", tt.target, info.ID, tt.target)
			}
			if tt.wantDescNotEmpty && info.Description == "" {
				t.Errorf("GetChallengeTargetInfo(%q) Description is empty, want non-empty", tt.target)
			}
			if tt.wantDescContains != "" && !containsIgnoreCase(info.Description, tt.wantDescContains) {
				t.Errorf("GetChallengeTargetInfo(%q) Description = %q, want to contain %q",
					tt.target, info.Description, tt.wantDescContains)
			}
		})
	}
}

// TestGetChallengeTargetInfo_NotExists tests that invalid targets return false.
func TestGetChallengeTargetInfo_NotExists(t *testing.T) {
	tests := []struct {
		name   string
		target ChallengeTarget
	}{
		{"invalid_foo", ChallengeTarget("foo")},
		{"empty_string", ChallengeTarget("")},
		{"uppercase", ChallengeTarget("STATEMENT")},
		{"random", ChallengeTarget("xyz123")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, exists := GetChallengeTargetInfo(tt.target)
			if exists {
				t.Errorf("GetChallengeTargetInfo(%q) returned exists=true, want false", tt.target)
			}
		})
	}
}

// TestAllChallengeTargets_Count tests that exactly 9 targets are returned.
func TestAllChallengeTargets_Count(t *testing.T) {
	targets := AllChallengeTargets()
	if len(targets) != 9 {
		t.Errorf("AllChallengeTargets() returned %d targets, want 9", len(targets))
	}
}

// TestAllChallengeTargets_AllValid tests that all returned targets are valid and unique.
func TestAllChallengeTargets_AllValid(t *testing.T) {
	targets := AllChallengeTargets()

	// Check for duplicates
	seen := make(map[ChallengeTarget]bool)
	for _, info := range targets {
		if seen[info.ID] {
			t.Errorf("AllChallengeTargets() contains duplicate target: %q", info.ID)
		}
		seen[info.ID] = true

		// Check that each target has a non-empty description
		if info.Description == "" {
			t.Errorf("AllChallengeTargets() target %q has empty description", info.ID)
		}
	}

	// Verify all 9 expected targets are present
	expectedTargets := []ChallengeTarget{
		TargetStatement, TargetInference, TargetContext, TargetDependencies,
		TargetScope, TargetGap, TargetTypeError, TargetDomain, TargetCompleteness,
	}

	for _, expected := range expectedTargets {
		if !seen[expected] {
			t.Errorf("AllChallengeTargets() missing expected target: %q", expected)
		}
	}
}

// TestParseChallengeTargets_Single tests parsing a single target.
func TestParseChallengeTargets_Single(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ChallengeTarget
	}{
		{
			name:     "statement",
			input:    "statement",
			expected: []ChallengeTarget{TargetStatement},
		},
		{
			name:     "inference",
			input:    "inference",
			expected: []ChallengeTarget{TargetInference},
		},
		{
			name:     "gap",
			input:    "gap",
			expected: []ChallengeTarget{TargetGap},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChallengeTargets(tt.input)
			if err != nil {
				t.Errorf("ParseChallengeTargets(%q) returned error: %v, want nil", tt.input, err)
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("ParseChallengeTargets(%q) returned %d targets, want %d", tt.input, len(got), len(tt.expected))
				return
			}
			for i, target := range tt.expected {
				if got[i] != target {
					t.Errorf("ParseChallengeTargets(%q)[%d] = %q, want %q", tt.input, i, got[i], target)
				}
			}
		})
	}
}

// TestParseChallengeTargets_Multiple tests parsing multiple comma-separated targets.
func TestParseChallengeTargets_Multiple(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ChallengeTarget
	}{
		{
			name:     "two_targets",
			input:    "inference,gap",
			expected: []ChallengeTarget{TargetInference, TargetGap},
		},
		{
			name:     "three_targets",
			input:    "statement,context,scope",
			expected: []ChallengeTarget{TargetStatement, TargetContext, TargetScope},
		},
		{
			name:     "all_nine_targets",
			input:    "statement,inference,context,dependencies,scope,gap,type_error,domain,completeness",
			expected: []ChallengeTarget{
				TargetStatement, TargetInference, TargetContext, TargetDependencies,
				TargetScope, TargetGap, TargetTypeError, TargetDomain, TargetCompleteness,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChallengeTargets(tt.input)
			if err != nil {
				t.Errorf("ParseChallengeTargets(%q) returned error: %v, want nil", tt.input, err)
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("ParseChallengeTargets(%q) returned %d targets, want %d", tt.input, len(got), len(tt.expected))
				return
			}
			for i, target := range tt.expected {
				if got[i] != target {
					t.Errorf("ParseChallengeTargets(%q)[%d] = %q, want %q", tt.input, i, got[i], target)
				}
			}
		})
	}
}

// TestParseChallengeTargets_Whitespace tests that whitespace around targets is handled correctly.
func TestParseChallengeTargets_Whitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ChallengeTarget
	}{
		{
			name:     "spaces_after_comma",
			input:    "inference, gap",
			expected: []ChallengeTarget{TargetInference, TargetGap},
		},
		{
			name:     "spaces_before_comma",
			input:    "inference ,gap",
			expected: []ChallengeTarget{TargetInference, TargetGap},
		},
		{
			name:     "spaces_both_sides",
			input:    "inference , gap",
			expected: []ChallengeTarget{TargetInference, TargetGap},
		},
		{
			name:     "leading_trailing_spaces",
			input:    "  statement,gap  ",
			expected: []ChallengeTarget{TargetStatement, TargetGap},
		},
		{
			name:     "multiple_spaces",
			input:    "statement  ,  gap  ,  scope",
			expected: []ChallengeTarget{TargetStatement, TargetGap, TargetScope},
		},
		{
			name:     "tabs_and_spaces",
			input:    "statement\t,\tgap",
			expected: []ChallengeTarget{TargetStatement, TargetGap},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChallengeTargets(tt.input)
			if err != nil {
				t.Errorf("ParseChallengeTargets(%q) returned error: %v, want nil", tt.input, err)
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("ParseChallengeTargets(%q) returned %d targets, want %d", tt.input, len(got), len(tt.expected))
				return
			}
			for i, target := range tt.expected {
				if got[i] != target {
					t.Errorf("ParseChallengeTargets(%q)[%d] = %q, want %q", tt.input, i, got[i], target)
				}
			}
		})
	}
}

// TestParseChallengeTargets_Invalid tests that invalid targets in CSV return an error.
func TestParseChallengeTargets_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "first_invalid",
			input: "foo,inference",
		},
		{
			name:  "last_invalid",
			input: "inference,bar",
		},
		{
			name:  "middle_invalid",
			input: "inference,foo,gap",
		},
		{
			name:  "all_invalid",
			input: "foo,bar,baz",
		},
		{
			name:  "empty_string",
			input: "",
		},
		{
			name:  "only_comma",
			input: ",",
		},
		{
			name:  "empty_between_commas",
			input: "inference,,gap",
		},
		{
			name:  "uppercase",
			input: "INFERENCE,gap",
		},
		{
			name:  "mixed_case",
			input: "Inference,Gap",
		},
		{
			name:  "trailing_comma",
			input: "inference,gap,",
		},
		{
			name:  "leading_comma",
			input: ",inference,gap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseChallengeTargets(tt.input)
			if err == nil {
				t.Errorf("ParseChallengeTargets(%q) returned nil error, want error", tt.input)
			}
		})
	}
}

// TestParseChallengeTargets_EmptyStringsInList tests that empty strings after splitting are handled.
func TestParseChallengeTargets_EmptyStringsInList(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "double_comma",
			input: "inference,,gap",
		},
		{
			name:  "comma_at_end",
			input: "inference,",
		},
		{
			name:  "comma_at_start",
			input: ",inference",
		},
		{
			name:  "only_commas",
			input: ",,",
		},
		{
			name:  "spaces_only_between_commas",
			input: "inference,  ,gap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseChallengeTargets(tt.input)
			if err == nil {
				t.Errorf("ParseChallengeTargets(%q) returned nil error, want error for empty target", tt.input)
			}
		})
	}
}

// Helper function to check if a string contains a substring (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return contains(s, substr)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
