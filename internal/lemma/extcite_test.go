// Package lemma provides validation for lemma and citation references.
package lemma

import (
	"testing"

	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/state"
)

func TestParseExtCitations(t *testing.T) {
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
			text:     "By external:axiom1, we have...",
			expected: []string{"axiom1"},
		},
		{
			name:     "multiple citations",
			text:     "Using external:ZFC and external:AC, we can show...",
			expected: []string{"ZFC", "AC"},
		},
		{
			name:     "hyphenated name",
			text:     "By external:Fermat-last-theorem, the result...",
			expected: []string{"Fermat-last-theorem"},
		},
		{
			name:     "underscored name",
			text:     "Using external:prime_number_theorem, we define...",
			expected: []string{"prime_number_theorem"},
		},
		{
			name:     "citation at end of sentence",
			text:     "This follows from external:axiom1.",
			expected: []string{"axiom1"},
		},
		{
			name:     "citation with comma",
			text:     "By external:AC, and external:CH, we have...",
			expected: []string{"AC", "CH"},
		},
		{
			name:     "citation in parentheses",
			text:     "A result (see external:Riemann) satisfies...",
			expected: []string{"Riemann"},
		},
		{
			name:     "duplicate citations deduplicated",
			text:     "By external:axiom1 and using external:axiom1 again...",
			expected: []string{"axiom1"},
		},
		{
			name:     "mixed case preserved",
			text:     "By external:Cauchy-Schwarz, the theorem...",
			expected: []string{"Cauchy-Schwarz"},
		},
		{
			name:     "alphanumeric name",
			text:     "Using external:theorem42, we get...",
			expected: []string{"theorem42"},
		},
		{
			name:     "name with numbers and hyphens",
			text:     "By external:sigma-2-algebra, the result...",
			expected: []string{"sigma-2-algebra"},
		},
		{
			name:     "mixed def and external citations",
			text:     "By def:group and external:axiom1, we show...",
			expected: []string{"axiom1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseExtCitations(tt.text)

			if len(result) != len(tt.expected) {
				t.Errorf("ParseExtCitations(%q) = %v, want %v", tt.text, result, tt.expected)
				return
			}

			for i, got := range result {
				if got != tt.expected[i] {
					t.Errorf("ParseExtCitations(%q)[%d] = %q, want %q", tt.text, i, got, tt.expected[i])
				}
			}
		})
	}
}

func TestValidateExtCitations(t *testing.T) {
	// Helper to create a state with externals
	setupState := func(extNames ...string) *state.State {
		st := state.NewState()
		for _, name := range extNames {
			ext := node.NewExternal(name, "Source for "+name)
			st.AddExternal(&ext)
		}
		return st
	}

	tests := []struct {
		name          string
		statement     string
		extNames      []string
		expectErr     bool
		expectErrCode errors.ErrorCode
	}{
		{
			name:      "no citations - valid",
			statement: "This statement has no citations.",
			extNames:  nil,
			expectErr: false,
		},
		{
			name:      "single valid citation",
			statement: "By external:axiom1, we have...",
			extNames:  []string{"axiom1"},
			expectErr: false,
		},
		{
			name:          "single invalid citation",
			statement:     "By external:axiom1, we have...",
			extNames:      nil,
			expectErr:     true,
			expectErrCode: errors.EXTERNAL_NOT_FOUND,
		},
		{
			name:      "multiple valid citations",
			statement: "Using external:ZFC and external:AC, we show...",
			extNames:  []string{"ZFC", "AC"},
			expectErr: false,
		},
		{
			name:          "one valid one invalid",
			statement:     "Using external:ZFC and external:AC, we show...",
			extNames:      []string{"ZFC"},
			expectErr:     true,
			expectErrCode: errors.EXTERNAL_NOT_FOUND,
		},
		{
			name:      "hyphenated name valid",
			statement: "By external:Fermat-last-theorem...",
			extNames:  []string{"Fermat-last-theorem"},
			expectErr: false,
		},
		{
			name:          "hyphenated name not found",
			statement:     "By external:Fermat-last-theorem...",
			extNames:      []string{"Fermat-little-theorem"},
			expectErr:     true,
			expectErrCode: errors.EXTERNAL_NOT_FOUND,
		},
		{
			name:      "underscored name valid",
			statement: "Using external:prime_number_theorem...",
			extNames:  []string{"prime_number_theorem"},
			expectErr: false,
		},
		{
			name:      "extra externals in state ok",
			statement: "By external:axiom1...",
			extNames:  []string{"axiom1", "axiom2", "axiom3"},
			expectErr: false,
		},
		{
			name:      "empty statement - valid",
			statement: "",
			extNames:  nil,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := setupState(tt.extNames...)
			err := ValidateExtCitations(tt.statement, st)

			if tt.expectErr {
				if err == nil {
					t.Errorf("ValidateExtCitations(%q) expected error, got nil", tt.statement)
					return
				}
				if tt.expectErrCode != 0 {
					code := errors.Code(err)
					if code != tt.expectErrCode {
						t.Errorf("ValidateExtCitations(%q) error code = %v, want %v", tt.statement, code, tt.expectErrCode)
					}
				}
			} else {
				if err != nil {
					t.Errorf("ValidateExtCitations(%q) unexpected error: %v", tt.statement, err)
				}
			}
		})
	}
}

func TestValidateExtCitations_NilState(t *testing.T) {
	// When state is nil, citations should fail validation
	err := ValidateExtCitations("By external:axiom1...", nil)
	if err == nil {
		t.Error("ValidateExtCitations with nil state expected error, got nil")
	}

	// But no citations should be fine even with nil state
	err = ValidateExtCitations("No citations here", nil)
	if err != nil {
		t.Errorf("ValidateExtCitations with no citations and nil state got error: %v", err)
	}
}

func TestValidateExtCitations_ErrorMessage(t *testing.T) {
	st := state.NewState()
	err := ValidateExtCitations("By external:missing-external...", st)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify error message contains the missing external name
	errMsg := err.Error()
	if !contains(errMsg, "missing-external") {
		t.Errorf("error message should contain 'missing-external', got: %s", errMsg)
	}
}

func TestCollectMissingExtCitations(t *testing.T) {
	// Helper to create a state with externals
	setupState := func(extNames ...string) *state.State {
		st := state.NewState()
		for _, name := range extNames {
			ext := node.NewExternal(name, "Source for "+name)
			st.AddExternal(&ext)
		}
		return st
	}

	tests := []struct {
		name            string
		statement       string
		extNames        []string
		expectedMissing []string
	}{
		{
			name:            "no citations",
			statement:       "No external citations here.",
			extNames:        nil,
			expectedMissing: nil,
		},
		{
			name:            "all citations valid",
			statement:       "By external:axiom1 and external:axiom2...",
			extNames:        []string{"axiom1", "axiom2"},
			expectedMissing: nil,
		},
		{
			name:            "one missing",
			statement:       "By external:axiom1 and external:axiom2...",
			extNames:        []string{"axiom1"},
			expectedMissing: []string{"axiom2"},
		},
		{
			name:            "multiple missing",
			statement:       "By external:a, external:b, external:c...",
			extNames:        []string{},
			expectedMissing: []string{"a", "b", "c"},
		},
		{
			name:            "nil state returns nil",
			statement:       "By external:axiom1...",
			extNames:        nil,
			expectedMissing: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var st *state.State
			if tt.name != "nil state returns nil" {
				st = setupState(tt.extNames...)
			}

			result := CollectMissingExtCitations(tt.statement, st)

			if len(result) != len(tt.expectedMissing) {
				t.Errorf("CollectMissingExtCitations(%q) = %v, want %v", tt.statement, result, tt.expectedMissing)
				return
			}

			for i, got := range result {
				if got != tt.expectedMissing[i] {
					t.Errorf("CollectMissingExtCitations(%q)[%d] = %q, want %q", tt.statement, i, got, tt.expectedMissing[i])
				}
			}
		})
	}
}
