package render

import (
	"strings"
	"testing"
)

// TestNewUsageError tests creation of usage errors with examples
func TestNewUsageError(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		message    string
		examples   []string
		wantCmd    string
		wantMsg    string
		wantExLen  int
	}{
		{
			name:      "single example",
			command:   "af claim",
			message:   "missing required argument: <node-id>",
			examples:  []string{"af claim 1 --owner agent1"},
			wantCmd:   "af claim",
			wantMsg:   "missing required argument: <node-id>",
			wantExLen: 1,
		},
		{
			name:      "multiple examples",
			command:   "af refine",
			message:   "missing required flag: --owner",
			examples:  []string{"af refine 1 --owner agent1 --statement \"First step\"", "af refine 1 -o agent1 -s \"First step\""},
			wantCmd:   "af refine",
			wantMsg:   "missing required flag: --owner",
			wantExLen: 2,
		},
		{
			name:      "no examples",
			command:   "af status",
			message:   "unknown error",
			examples:  []string{},
			wantCmd:   "af status",
			wantMsg:   "unknown error",
			wantExLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewUsageError(tt.command, tt.message, tt.examples)

			if err.Command != tt.wantCmd {
				t.Errorf("Command = %q, want %q", err.Command, tt.wantCmd)
			}

			if err.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", err.Message, tt.wantMsg)
			}

			if len(err.Examples) != tt.wantExLen {
				t.Errorf("Examples length = %d, want %d", len(err.Examples), tt.wantExLen)
			}
		})
	}
}

// TestUsageError_WithSuggestions tests adding fuzzy match suggestions
func TestUsageError_WithSuggestions(t *testing.T) {
	err := NewUsageError("af", "unknown command \"claom\"", nil)
	err = err.WithSuggestions([]string{"claim", "status", "clean"})

	if len(err.Suggestions) != 3 {
		t.Errorf("Suggestions length = %d, want 3", len(err.Suggestions))
	}

	if err.Suggestions[0] != "claim" {
		t.Errorf("Suggestions[0] = %q, want %q", err.Suggestions[0], "claim")
	}
}

// TestUsageError_WithValidValues tests adding valid values for a parameter
func TestUsageError_WithValidValues(t *testing.T) {
	err := NewUsageError("af refine", "invalid node type \"cliam\"", nil)
	err = err.WithValidValues("type", []string{"claim", "case", "qed", "local_assume", "local_discharge"})

	if err.InvalidParam != "type" {
		t.Errorf("InvalidParam = %q, want %q", err.InvalidParam, "type")
	}

	if len(err.ValidValues) != 5 {
		t.Errorf("ValidValues length = %d, want 5", len(err.ValidValues))
	}
}

// TestFormatUsageError_Basic tests basic formatting with command and message
func TestFormatUsageError_Basic(t *testing.T) {
	err := NewUsageError("af claim", "missing required argument: <node-id>", nil)
	result := FormatUsageError(err)

	if result == "" {
		t.Fatal("FormatUsageError returned empty string")
	}

	// Should contain the error message
	if !strings.Contains(result, "missing required argument") {
		t.Errorf("FormatUsageError missing message, got: %q", result)
	}
}

// TestFormatUsageError_WithExamples tests formatting with example usage
func TestFormatUsageError_WithExamples(t *testing.T) {
	err := NewUsageError("af claim", "missing required argument: <node-id>",
		[]string{"af claim 1 --owner agent1", "af claim 1.2 --owner agent1 --role verifier"})

	result := FormatUsageError(err)

	// Should contain examples section
	if !strings.Contains(result, "Example") {
		t.Errorf("FormatUsageError missing examples section, got: %q", result)
	}

	// Should contain both examples
	if !strings.Contains(result, "af claim 1 --owner agent1") {
		t.Errorf("FormatUsageError missing first example, got: %q", result)
	}

	if !strings.Contains(result, "af claim 1.2 --owner agent1 --role verifier") {
		t.Errorf("FormatUsageError missing second example, got: %q", result)
	}
}

// TestFormatUsageError_WithSuggestions tests formatting with fuzzy suggestions
func TestFormatUsageError_WithSuggestions(t *testing.T) {
	err := NewUsageError("af", "unknown command \"claom\"", nil)
	err = err.WithSuggestions([]string{"claim"})

	result := FormatUsageError(err)

	// Should contain "Did you mean" section
	if !strings.Contains(result, "Did you mean") {
		t.Errorf("FormatUsageError missing 'Did you mean' section, got: %q", result)
	}

	// Should contain the suggestion
	if !strings.Contains(result, "claim") {
		t.Errorf("FormatUsageError missing suggestion 'claim', got: %q", result)
	}
}

// TestFormatUsageError_WithMultipleSuggestions tests formatting with multiple suggestions
func TestFormatUsageError_WithMultipleSuggestions(t *testing.T) {
	err := NewUsageError("af", "unknown command \"re\"", nil)
	err = err.WithSuggestions([]string{"refine", "release", "refute", "replay"})

	result := FormatUsageError(err)

	// Should list multiple suggestions
	for _, s := range []string{"refine", "release", "refute", "replay"} {
		if !strings.Contains(result, s) {
			t.Errorf("FormatUsageError missing suggestion %q, got: %q", s, result)
		}
	}
}

// TestFormatUsageError_WithValidValues tests formatting with valid values
func TestFormatUsageError_WithValidValues(t *testing.T) {
	err := NewUsageError("af refine", "invalid node type \"cliam\"", nil)
	err = err.WithValidValues("type", []string{"claim", "case", "qed"})

	result := FormatUsageError(err)

	// Should contain valid values section
	if !strings.Contains(result, "Valid values for") || !strings.Contains(result, "type") {
		t.Errorf("FormatUsageError missing valid values header, got: %q", result)
	}

	// Should list valid values
	for _, v := range []string{"claim", "case", "qed"} {
		if !strings.Contains(result, v) {
			t.Errorf("FormatUsageError missing valid value %q, got: %q", v, result)
		}
	}
}

// TestFormatUsageError_Complete tests complete error with all fields
func TestFormatUsageError_Complete(t *testing.T) {
	err := NewUsageError("af refine", "invalid node type \"cliam\"",
		[]string{"af refine 1 --owner agent1 --type claim --statement \"Step 1\""})
	err = err.WithSuggestions([]string{"claim"})
	err = err.WithValidValues("type", []string{"claim", "case", "qed"})

	result := FormatUsageError(err)

	// Should contain all sections
	checks := []struct {
		name    string
		content string
	}{
		{"error message", "invalid node type"},
		{"suggestion", "Did you mean"},
		{"valid values", "Valid values"},
		{"example", "Example"},
	}

	for _, check := range checks {
		if !strings.Contains(result, check.content) {
			t.Errorf("FormatUsageError missing %s (%q), got: %q", check.name, check.content, result)
		}
	}
}

// TestUsageError_Error tests the error interface implementation
func TestUsageError_Error(t *testing.T) {
	err := NewUsageError("af claim", "missing required argument: <node-id>",
		[]string{"af claim 1 --owner agent1"})

	// Should implement error interface
	var e error = err
	if e.Error() == "" {
		t.Error("Error() returned empty string")
	}

	// Error string should contain the message
	if !strings.Contains(e.Error(), "missing required argument") {
		t.Errorf("Error() missing message, got: %q", e.Error())
	}

	// Error string should contain example when provided
	if !strings.Contains(e.Error(), "Example") {
		t.Errorf("Error() missing example section, got: %q", e.Error())
	}
}

// TestUsageError_ErrorWithSuggestions tests error output with suggestions
func TestUsageError_ErrorWithSuggestions(t *testing.T) {
	err := NewUsageError("af", "unknown command \"claom\"", nil)
	err = err.WithSuggestions([]string{"claim"})

	result := err.Error()

	// Should contain the suggestion
	if !strings.Contains(result, "Did you mean") {
		t.Errorf("Error() missing 'Did you mean' section, got: %q", result)
	}
	if !strings.Contains(result, "claim") {
		t.Errorf("Error() missing suggestion 'claim', got: %q", result)
	}
}

// TestUsageError_ErrorWithValidValues tests error output with valid values
func TestUsageError_ErrorWithValidValues(t *testing.T) {
	err := NewUsageError("af refine", "invalid node type", nil)
	err = err.WithValidValues("type", []string{"claim", "case", "qed"})

	result := err.Error()

	// Should contain valid values section
	if !strings.Contains(result, "Valid values for --type") {
		t.Errorf("Error() missing valid values header, got: %q", result)
	}

	// Should list valid values
	for _, v := range []string{"claim", "case", "qed"} {
		if !strings.Contains(result, v) {
			t.Errorf("Error() missing valid value %q, got: %q", v, result)
		}
	}
}

// TestMissingArgError tests the helper for missing argument errors
func TestMissingArgError(t *testing.T) {
	err := MissingArgError("af claim", "node-id",
		[]string{"af claim 1 --owner agent1"})

	if err.Command != "af claim" {
		t.Errorf("Command = %q, want %q", err.Command, "af claim")
	}

	if !strings.Contains(err.Message, "node-id") {
		t.Errorf("Message should mention the missing argument, got: %q", err.Message)
	}

	if !strings.Contains(err.Message, "required") {
		t.Errorf("Message should indicate requirement, got: %q", err.Message)
	}
}

// TestMissingFlagError tests the helper for missing flag errors
func TestMissingFlagError(t *testing.T) {
	err := MissingFlagError("af refine", "owner",
		[]string{"af refine 1 --owner agent1 --statement \"Step\""})

	if err.Command != "af refine" {
		t.Errorf("Command = %q, want %q", err.Command, "af refine")
	}

	if !strings.Contains(err.Message, "--owner") {
		t.Errorf("Message should mention the flag with --, got: %q", err.Message)
	}
}

// TestInvalidValueError tests the helper for invalid value errors
func TestInvalidValueError(t *testing.T) {
	err := InvalidValueError("af refine", "type", "cliam",
		[]string{"claim", "case", "qed"},
		[]string{"af refine 1 --owner agent1 --type claim"})

	if err.InvalidParam != "type" {
		t.Errorf("InvalidParam = %q, want %q", err.InvalidParam, "type")
	}

	if !strings.Contains(err.Message, "cliam") {
		t.Errorf("Message should contain the invalid value, got: %q", err.Message)
	}

	if len(err.ValidValues) != 3 {
		t.Errorf("ValidValues length = %d, want 3", len(err.ValidValues))
	}
}

// TestInvalidValueError_WithFuzzySuggestion tests that invalid values get fuzzy suggestions
func TestInvalidValueError_WithFuzzySuggestion(t *testing.T) {
	err := InvalidValueError("af refine", "type", "cliam",
		[]string{"claim", "case", "qed", "local_assume", "local_discharge"},
		nil)

	// Should have suggestions based on fuzzy match
	if len(err.Suggestions) == 0 {
		t.Error("InvalidValueError should provide fuzzy suggestions")
	}

	// "cliam" should fuzzy match to "claim"
	found := false
	for _, s := range err.Suggestions {
		if s == "claim" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Suggestions should include 'claim' for input 'cliam', got: %v", err.Suggestions)
	}
}

// TestFormatUsageErrorJSON tests JSON formatting of usage errors
func TestFormatUsageErrorJSON(t *testing.T) {
	err := NewUsageError("af claim", "missing required argument: <node-id>",
		[]string{"af claim 1 --owner agent1"})
	err = err.WithSuggestions([]string{"claim"})

	result := FormatUsageErrorJSON(err)

	if result == "" {
		t.Fatal("FormatUsageErrorJSON returned empty string")
	}

	// Should be valid JSON (starts with {)
	if !strings.HasPrefix(result, "{") {
		t.Errorf("FormatUsageErrorJSON should return JSON object, got: %q", result)
	}

	// Should contain key fields
	if !strings.Contains(result, "command") {
		t.Error("JSON missing 'command' field")
	}
	if !strings.Contains(result, "message") {
		t.Error("JSON missing 'message' field")
	}
	if !strings.Contains(result, "examples") {
		t.Error("JSON missing 'examples' field")
	}
}
