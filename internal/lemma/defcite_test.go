// Package lemma provides validation for lemma and citation references.
package lemma

import (
	"testing"

	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/state"
)

func TestParseDefCitations(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "empty text",
			text:     "",
			expected: nil,
		},
		{
			name:     "no citations",
			text:     "This is a statement without any citations.",
			expected: nil,
		},
		{
			name:     "single citation",
			text:     "By def:group, we have...",
			expected: []string{"group"},
		},
		{
			name:     "multiple citations",
			text:     "Using def:homomorphism and def:kernel, we can show...",
			expected: []string{"homomorphism", "kernel"},
		},
		{
			name:     "hyphenated name",
			text:     "By def:Stirling-second-kind, the number...",
			expected: []string{"Stirling-second-kind"},
		},
		{
			name:     "underscored name",
			text:     "Using def:vector_space, we define...",
			expected: []string{"vector_space"},
		},
		{
			name:     "citation at end of sentence",
			text:     "This follows from def:group.",
			expected: []string{"group"},
		},
		{
			name:     "citation with comma",
			text:     "By def:group, and def:ring, we have...",
			expected: []string{"group", "ring"},
		},
		{
			name:     "citation in parentheses",
			text:     "A structure (see def:field) satisfies...",
			expected: []string{"field"},
		},
		{
			name:     "duplicate citations deduplicated",
			text:     "By def:group and using def:group again...",
			expected: []string{"group"},
		},
		{
			name:     "mixed case preserved",
			text:     "By def:Cayley-Hamilton, the theorem...",
			expected: []string{"Cayley-Hamilton"},
		},
		{
			name:     "alphanumeric name",
			text:     "Using def:theorem42, we get...",
			expected: []string{"theorem42"},
		},
		{
			name:     "name with numbers and hyphens",
			text:     "By def:sigma-2-algebra, the result...",
			expected: []string{"sigma-2-algebra"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDefCitations(tt.text)

			if len(result) != len(tt.expected) {
				t.Errorf("ParseDefCitations(%q) = %v, want %v", tt.text, result, tt.expected)
				return
			}

			for i, got := range result {
				if got != tt.expected[i] {
					t.Errorf("ParseDefCitations(%q)[%d] = %q, want %q", tt.text, i, got, tt.expected[i])
				}
			}
		})
	}
}

func TestValidateDefCitations(t *testing.T) {
	// Helper to create a state with definitions
	setupState := func(defNames ...string) *state.State {
		st := state.NewState()
		for _, name := range defNames {
			def, _ := node.NewDefinition(name, "Definition of "+name)
			st.AddDefinition(def)
		}
		return st
	}

	tests := []struct {
		name          string
		statement     string
		defNames      []string
		expectErr     bool
		expectErrCode errors.ErrorCode
	}{
		{
			name:      "no citations - valid",
			statement: "This statement has no citations.",
			defNames:  nil,
			expectErr: false,
		},
		{
			name:      "single valid citation",
			statement: "By def:group, we have...",
			defNames:  []string{"group"},
			expectErr: false,
		},
		{
			name:          "single invalid citation",
			statement:     "By def:group, we have...",
			defNames:      nil,
			expectErr:     true,
			expectErrCode: errors.DEF_NOT_FOUND,
		},
		{
			name:      "multiple valid citations",
			statement: "Using def:group and def:ring, we show...",
			defNames:  []string{"group", "ring"},
			expectErr: false,
		},
		{
			name:          "one valid one invalid",
			statement:     "Using def:group and def:field, we show...",
			defNames:      []string{"group"},
			expectErr:     true,
			expectErrCode: errors.DEF_NOT_FOUND,
		},
		{
			name:      "hyphenated name valid",
			statement: "By def:Stirling-second-kind...",
			defNames:  []string{"Stirling-second-kind"},
			expectErr: false,
		},
		{
			name:          "hyphenated name not found",
			statement:     "By def:Stirling-second-kind...",
			defNames:      []string{"Stirling-first-kind"},
			expectErr:     true,
			expectErrCode: errors.DEF_NOT_FOUND,
		},
		{
			name:      "underscored name valid",
			statement: "Using def:vector_space...",
			defNames:  []string{"vector_space"},
			expectErr: false,
		},
		{
			name:      "extra definitions in state ok",
			statement: "By def:group...",
			defNames:  []string{"group", "ring", "field"},
			expectErr: false,
		},
		{
			name:      "empty statement - valid",
			statement: "",
			defNames:  nil,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := setupState(tt.defNames...)
			err := ValidateDefCitations(tt.statement, st)

			if tt.expectErr {
				if err == nil {
					t.Errorf("ValidateDefCitations(%q) expected error, got nil", tt.statement)
					return
				}
				if tt.expectErrCode != 0 {
					code := errors.Code(err)
					if code != tt.expectErrCode {
						t.Errorf("ValidateDefCitations(%q) error code = %v, want %v", tt.statement, code, tt.expectErrCode)
					}
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDefCitations(%q) unexpected error: %v", tt.statement, err)
				}
			}
		})
	}
}

func TestValidateDefCitations_NilState(t *testing.T) {
	// When state is nil, citations should fail validation
	err := ValidateDefCitations("By def:group...", nil)
	if err == nil {
		t.Error("ValidateDefCitations with nil state expected error, got nil")
	}

	// But no citations should be fine even with nil state
	err = ValidateDefCitations("No citations here", nil)
	if err != nil {
		t.Errorf("ValidateDefCitations with no citations and nil state got error: %v", err)
	}
}

func TestValidateDefCitations_ErrorMessage(t *testing.T) {
	st := state.NewState()
	err := ValidateDefCitations("By def:missing-definition...", st)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify error message contains the missing definition name
	errMsg := err.Error()
	if !contains(errMsg, "missing-definition") {
		t.Errorf("error message should contain 'missing-definition', got: %s", errMsg)
	}
}

// helper function
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
