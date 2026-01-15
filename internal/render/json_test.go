//go:build integration

package render

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// JSONOutput represents the expected JSON structure for testing round-trips
type JSONOutput struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Recovery []string `json:"recovery"`
	ExitCode int      `json:"exit_code"`
}

// TestFormatJSON_ValidJSONStructure tests that FormatJSON produces valid JSON
func TestFormatJSON_ValidJSONStructure(t *testing.T) {
	tests := []struct {
		name     string
		rendered RenderedError
	}{
		{
			name: "full error with all fields",
			rendered: RenderedError{
				Code:     "NODE_BLOCKED",
				Message:  "node 1.3 is blocked by pending challenge",
				Recovery: []string{"Resolve blocking challenges first", "Use 'af status' to see blockers"},
				ExitCode: 2,
			},
		},
		{
			name: "empty code",
			rendered: RenderedError{
				Code:     "",
				Message:  "something went wrong",
				Recovery: []string{},
				ExitCode: 1,
			},
		},
		{
			name: "empty message",
			rendered: RenderedError{
				Code:     "INVALID_TYPE",
				Message:  "",
				Recovery: []string{"Check node type"},
				ExitCode: 3,
			},
		},
		{
			name: "nil recovery slice",
			rendered: RenderedError{
				Code:     "ALREADY_CLAIMED",
				Message:  "node is claimed",
				Recovery: nil,
				ExitCode: 1,
			},
		},
		{
			name: "zero exit code",
			rendered: RenderedError{
				Code:     "",
				Message:  "",
				Recovery: []string{},
				ExitCode: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatJSON(tt.rendered)

			if result == "" {
				t.Fatal("FormatJSON returned empty string")
			}

			// Verify it's valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("FormatJSON produced invalid JSON: %v\nOutput: %s", err, result)
			}
		})
	}
}

// TestFormatJSON_RequiredFields tests that all required fields are present
func TestFormatJSON_RequiredFields(t *testing.T) {
	rendered := RenderedError{
		Code:     "SCOPE_VIOLATION",
		Message:  "assumption referenced outside scope",
		Recovery: []string{"Check scope boundaries"},
		ExitCode: 3,
	}

	result := FormatJSON(rendered)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	requiredFields := []string{"code", "message", "recovery", "exit_code"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("JSON missing required field %q", field)
		}
	}
}

// TestJSON_RoundtripComprehensive tests that JSON can be parsed back to matching values
func TestJSON_RoundtripComprehensive(t *testing.T) {
	tests := []struct {
		name     string
		original RenderedError
	}{
		{
			name: "retriable error",
			original: RenderedError{
				Code:     "ALREADY_CLAIMED",
				Message:  "node 1.2 is claimed by agent-xyz",
				Recovery: []string{"Wait for release", "Use 'af jobs' to find alternatives"},
				ExitCode: 1,
			},
		},
		{
			name: "blocked error",
			original: RenderedError{
				Code:     "NODE_BLOCKED",
				Message:  "node 1.3 blocked by pending definition",
				Recovery: []string{"Resolve blockers first", "Check status"},
				ExitCode: 2,
			},
		},
		{
			name: "logic error",
			original: RenderedError{
				Code:     "SCOPE_VIOLATION",
				Message:  "cannot reference assumption outside scope",
				Recovery: []string{"Check assumption is in scope", "Review scope boundaries"},
				ExitCode: 3,
			},
		},
		{
			name: "corruption error",
			original: RenderedError{
				Code:     "LEDGER_INCONSISTENT",
				Message:  "sequence gap detected at line 42",
				Recovery: []string{"Contact administrator", "Check filesystem integrity"},
				ExitCode: 4,
			},
		},
		{
			name: "single recovery suggestion",
			original: RenderedError{
				Code:     "INVALID_PARENT",
				Message:  "parent node does not exist",
				Recovery: []string{"Verify node hierarchy"},
				ExitCode: 3,
			},
		},
		{
			name: "many recovery suggestions",
			original: RenderedError{
				Code:     "NOT_CLAIM_HOLDER",
				Message:  "agent does not hold claim",
				Recovery: []string{"One", "Two", "Three", "Four", "Five"},
				ExitCode: 1,
			},
		},
		{
			name: "empty recovery list",
			original: RenderedError{
				Code:     "UNKNOWN",
				Message:  "unknown error occurred",
				Recovery: []string{},
				ExitCode: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonStr := FormatJSON(tt.original)

			var parsed JSONOutput
			if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			// Compare fields
			if parsed.Code != tt.original.Code {
				t.Errorf("Code = %q, want %q", parsed.Code, tt.original.Code)
			}

			if parsed.Message != tt.original.Message {
				t.Errorf("Message = %q, want %q", parsed.Message, tt.original.Message)
			}

			if parsed.ExitCode != tt.original.ExitCode {
				t.Errorf("ExitCode = %d, want %d", parsed.ExitCode, tt.original.ExitCode)
			}

			// Handle nil vs empty slice for recovery
			if tt.original.Recovery == nil {
				if parsed.Recovery != nil && len(parsed.Recovery) != 0 {
					t.Errorf("Recovery = %v, want nil or empty", parsed.Recovery)
				}
			} else {
				if len(parsed.Recovery) != len(tt.original.Recovery) {
					t.Fatalf("Recovery length = %d, want %d", len(parsed.Recovery), len(tt.original.Recovery))
				}
				for i, rec := range parsed.Recovery {
					if rec != tt.original.Recovery[i] {
						t.Errorf("Recovery[%d] = %q, want %q", i, rec, tt.original.Recovery[i])
					}
				}
			}
		})
	}
}

// TestFormatJSON_SpecialCharacters tests handling of special characters in strings
func TestFormatJSON_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		rendered RenderedError
		wantIn   string // substring that should be present in output
	}{
		{
			name: "double quotes in message",
			rendered: RenderedError{
				Code:     "INVALID_TYPE",
				Message:  `expected "step" but got "assumption"`,
				Recovery: []string{},
				ExitCode: 3,
			},
			wantIn: `expected \"step\" but got \"assumption\"`,
		},
		{
			name: "backslash in message",
			rendered: RenderedError{
				Code:     "INVALID_PARENT",
				Message:  `path\to\node`,
				Recovery: []string{},
				ExitCode: 3,
			},
			wantIn: `path\\to\\node`,
		},
		{
			name: "newline in message",
			rendered: RenderedError{
				Code:     "SCOPE_VIOLATION",
				Message:  "line one\nline two",
				Recovery: []string{},
				ExitCode: 3,
			},
			wantIn: `line one\nline two`,
		},
		{
			name: "tab in message",
			rendered: RenderedError{
				Code:     "INVALID_INFERENCE",
				Message:  "column\tone",
				Recovery: []string{},
				ExitCode: 3,
			},
			wantIn: `column\tone`,
		},
		{
			name: "unicode in message",
			rendered: RenderedError{
				Code:     "DEF_NOT_FOUND",
				Message:  "definition not found",
				Recovery: []string{},
				ExitCode: 3,
			},
			wantIn: "definition not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatJSON(tt.rendered)

			// Should be valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("FormatJSON produced invalid JSON for special characters: %v\nOutput: %s", err, result)
			}

			// Should contain properly escaped string
			if !strings.Contains(result, tt.wantIn) {
				t.Errorf("JSON output missing expected content %q, got: %s", tt.wantIn, result)
			}
		})
	}
}

// TestFormatJSON_FromAFError tests JSON formatting of rendered AFErrors
func TestFormatJSON_FromAFError(t *testing.T) {
	tests := []struct {
		name       string
		err        *errors.AFError
		wantCode   string
		wantExit   int
		wantInMsg  string
	}{
		{
			name:      "ALREADY_CLAIMED",
			err:       errors.New(errors.ALREADY_CLAIMED, "node 1.2 is claimed"),
			wantCode:  "ALREADY_CLAIMED",
			wantExit:  1,
			wantInMsg: "claimed",
		},
		{
			name:      "NODE_BLOCKED",
			err:       errors.New(errors.NODE_BLOCKED, "node is blocked by challenge"),
			wantCode:  "NODE_BLOCKED",
			wantExit:  2,
			wantInMsg: "blocked",
		},
		{
			name:      "INVALID_PARENT",
			err:       errors.New(errors.INVALID_PARENT, "parent does not exist"),
			wantCode:  "INVALID_PARENT",
			wantExit:  3,
			wantInMsg: "parent",
		},
		{
			name:      "LEDGER_INCONSISTENT",
			err:       errors.New(errors.LEDGER_INCONSISTENT, "corruption detected"),
			wantCode:  "LEDGER_INCONSISTENT",
			wantExit:  4,
			wantInMsg: "corruption",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := RenderAFError(tt.err)
			result := FormatJSON(rendered)

			var parsed JSONOutput
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			if parsed.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", parsed.Code, tt.wantCode)
			}

			if parsed.ExitCode != tt.wantExit {
				t.Errorf("ExitCode = %d, want %d", parsed.ExitCode, tt.wantExit)
			}

			if !strings.Contains(parsed.Message, tt.wantInMsg) {
				t.Errorf("Message = %q, want to contain %q", parsed.Message, tt.wantInMsg)
			}

			// AFErrors should have recovery suggestions
			if len(parsed.Recovery) == 0 {
				t.Error("Recovery suggestions are empty, want at least one")
			}
		})
	}
}

// TestFormatJSON_FromGenericError tests JSON formatting of rendered generic errors
func TestFormatJSON_FromGenericError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantExit  int
		wantInMsg string
	}{
		{
			name:      "simple error",
			err:       fmt.Errorf("something went wrong"),
			wantExit:  1,
			wantInMsg: "something went wrong",
		},
		{
			name:      "wrapped error",
			err:       fmt.Errorf("outer: %w", fmt.Errorf("inner error")),
			wantExit:  1,
			wantInMsg: "inner error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := RenderError(tt.err)
			result := FormatJSON(rendered)

			var parsed JSONOutput
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			// Generic errors have empty code
			if parsed.Code != "" {
				t.Errorf("Code = %q, want empty string", parsed.Code)
			}

			if parsed.ExitCode != tt.wantExit {
				t.Errorf("ExitCode = %d, want %d", parsed.ExitCode, tt.wantExit)
			}

			if !strings.Contains(parsed.Message, tt.wantInMsg) {
				t.Errorf("Message = %q, want to contain %q", parsed.Message, tt.wantInMsg)
			}
		})
	}
}

// TestFormatJSON_EmptyRenderedError tests JSON formatting of zero-value RenderedError
func TestFormatJSON_EmptyRenderedError(t *testing.T) {
	rendered := RenderedError{}
	result := FormatJSON(rendered)

	if result == "" {
		t.Fatal("FormatJSON returned empty string for empty RenderedError")
	}

	var parsed JSONOutput
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if parsed.Code != "" {
		t.Errorf("Code = %q, want empty string", parsed.Code)
	}

	if parsed.Message != "" {
		t.Errorf("Message = %q, want empty string", parsed.Message)
	}

	if parsed.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", parsed.ExitCode)
	}

	// Recovery should be empty array or nil in JSON
	if len(parsed.Recovery) != 0 {
		t.Errorf("Recovery = %v, want empty", parsed.Recovery)
	}
}

// TestFormatJSON_AllErrorCodes tests JSON output for all defined error codes
func TestFormatJSON_AllErrorCodes(t *testing.T) {
	errorCodes := []errors.ErrorCode{
		errors.ALREADY_CLAIMED,
		errors.NOT_CLAIM_HOLDER,
		errors.NODE_BLOCKED,
		errors.INVALID_PARENT,
		errors.INVALID_TYPE,
		errors.INVALID_INFERENCE,
		errors.INVALID_TARGET,
		errors.CHALLENGE_NOT_FOUND,
		errors.DEF_NOT_FOUND,
		errors.ASSUMPTION_NOT_FOUND,
		errors.EXTERNAL_NOT_FOUND,
		errors.SCOPE_VIOLATION,
		errors.SCOPE_UNCLOSED,
		errors.DEPENDENCY_CYCLE,
		errors.CONTENT_HASH_MISMATCH,
		errors.VALIDATION_INVARIANT_FAILED,
		errors.LEDGER_INCONSISTENT,
		errors.DEPTH_EXCEEDED,
		errors.CHALLENGE_LIMIT_EXCEEDED,
		errors.REFINEMENT_LIMIT_EXCEEDED,
		errors.EXTRACTION_INVALID,
	}

	for _, code := range errorCodes {
		t.Run(code.String(), func(t *testing.T) {
			err := errors.New(code, "test message")
			rendered := RenderAFError(err)
			result := FormatJSON(rendered)

			var parsed JSONOutput
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON for %s: %v", code.String(), err)
			}

			if parsed.Code != code.String() {
				t.Errorf("Code = %q, want %q", parsed.Code, code.String())
			}

			// Exit code should match the error code's exit code
			expectedExit := code.ExitCode()
			if parsed.ExitCode != expectedExit {
				t.Errorf("ExitCode = %d, want %d", parsed.ExitCode, expectedExit)
			}

			// All error codes should have recovery suggestions
			if len(parsed.Recovery) == 0 {
				t.Errorf("Recovery suggestions are empty for %s", code.String())
			}
		})
	}
}

// TestFormatJSON_ConsistentOutput tests that output is deterministic
func TestFormatJSON_ConsistentOutput(t *testing.T) {
	rendered := RenderedError{
		Code:     "INVALID_TYPE",
		Message:  "expected step, got assumption",
		Recovery: []string{"Check node type", "Review proof structure"},
		ExitCode: 3,
	}

	// Generate multiple times
	results := make([]string, 10)
	for i := 0; i < 10; i++ {
		results[i] = FormatJSON(rendered)
	}

	// All should be identical
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("FormatJSON output not deterministic: got %q and %q", results[0], results[i])
		}
	}
}

// TestFormatJSON_LongMessage tests handling of very long messages
func TestFormatJSON_LongMessage(t *testing.T) {
	longMessage := strings.Repeat("This is a long error message. ", 100)
	rendered := RenderedError{
		Code:     "INVALID_INFERENCE",
		Message:  longMessage,
		Recovery: []string{"Review inference"},
		ExitCode: 3,
	}

	result := FormatJSON(rendered)

	var parsed JSONOutput
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON for long message: %v", err)
	}

	if parsed.Message != longMessage {
		t.Error("Long message was truncated or modified")
	}
}

// TestFormatJSON_ManyRecoverySuggestions tests handling of many recovery suggestions
func TestFormatJSON_ManyRecoverySuggestions(t *testing.T) {
	var recovery []string
	for i := 0; i < 50; i++ {
		recovery = append(recovery, fmt.Sprintf("Suggestion %d: do something specific", i))
	}

	rendered := RenderedError{
		Code:     "COMPLEX_ERROR",
		Message:  "complex error with many suggestions",
		Recovery: recovery,
		ExitCode: 3,
	}

	result := FormatJSON(rendered)

	var parsed JSONOutput
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON for many suggestions: %v", err)
	}

	if len(parsed.Recovery) != len(recovery) {
		t.Errorf("Recovery count = %d, want %d", len(parsed.Recovery), len(recovery))
	}
}

// TestFormatJSON_WrappedAFError tests JSON formatting of wrapped AFErrors
func TestFormatJSON_WrappedAFError(t *testing.T) {
	inner := errors.New(errors.INVALID_TYPE, "expected step, got assumption")
	wrapped := errors.Wrap(inner, "while validating node 1.2")

	rendered := RenderError(wrapped)
	result := FormatJSON(rendered)

	var parsed JSONOutput
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if parsed.Code != "INVALID_TYPE" {
		t.Errorf("Code = %q, want INVALID_TYPE", parsed.Code)
	}

	if parsed.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", parsed.ExitCode)
	}

	// Message should contain context from wrapping
	if !strings.Contains(parsed.Message, "validating") {
		t.Errorf("Message missing wrapping context, got: %q", parsed.Message)
	}
}

// TestFormatJSON_NilErrorRendered tests JSON output for rendered nil error
func TestFormatJSON_NilErrorRendered(t *testing.T) {
	rendered := RenderError(nil)
	result := FormatJSON(rendered)

	var parsed JSONOutput
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if parsed.Code != "" {
		t.Errorf("Code = %q, want empty string", parsed.Code)
	}

	if parsed.Message != "" {
		t.Errorf("Message = %q, want empty string", parsed.Message)
	}

	if parsed.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", parsed.ExitCode)
	}
}

// TestFormatJSON_ExitCodesByCategory tests that exit codes are correctly categorized
func TestFormatJSON_ExitCodesByCategory(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		wantExit int
		category string
	}{
		// Exit 1: retriable
		{"ALREADY_CLAIMED is retriable", errors.ALREADY_CLAIMED, 1, "retriable"},
		{"NOT_CLAIM_HOLDER is retriable", errors.NOT_CLAIM_HOLDER, 1, "retriable"},
		{"VALIDATION_INVARIANT_FAILED is retriable", errors.VALIDATION_INVARIANT_FAILED, 1, "retriable"},

		// Exit 2: blocked
		{"NODE_BLOCKED is blocked", errors.NODE_BLOCKED, 2, "blocked"},

		// Exit 3: logic errors
		{"INVALID_PARENT is logic error", errors.INVALID_PARENT, 3, "logic"},
		{"INVALID_TYPE is logic error", errors.INVALID_TYPE, 3, "logic"},
		{"SCOPE_VIOLATION is logic error", errors.SCOPE_VIOLATION, 3, "logic"},
		{"DEPTH_EXCEEDED is logic error", errors.DEPTH_EXCEEDED, 3, "logic"},

		// Exit 4: corruption
		{"CONTENT_HASH_MISMATCH is corruption", errors.CONTENT_HASH_MISMATCH, 4, "corruption"},
		{"LEDGER_INCONSISTENT is corruption", errors.LEDGER_INCONSISTENT, 4, "corruption"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.code, "test message")
			rendered := RenderAFError(err)
			result := FormatJSON(rendered)

			var parsed JSONOutput
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			if parsed.ExitCode != tt.wantExit {
				t.Errorf("ExitCode = %d, want %d (%s)", parsed.ExitCode, tt.wantExit, tt.category)
			}
		})
	}
}

// TestFormatJSON_NoExtraFields tests that JSON contains only expected fields
func TestFormatJSON_NoExtraFields(t *testing.T) {
	rendered := RenderedError{
		Code:       "TEST",
		Message:    "test message",
		Recovery:   []string{"suggestion"},
		ExitCode:   1,
		JSONOutput: "ignored", // This field should not appear in output
	}

	result := FormatJSON(rendered)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	expectedFields := map[string]bool{
		"code":      true,
		"message":   true,
		"recovery":  true,
		"exit_code": true,
	}

	for key := range parsed {
		if !expectedFields[key] {
			t.Errorf("Unexpected field in JSON: %q", key)
		}
	}

	for field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("Missing expected field: %q", field)
		}
	}
}

// TestFormatJSON_RecoveryArrayType tests that recovery is always an array in JSON
func TestFormatJSON_RecoveryArrayType(t *testing.T) {
	tests := []struct {
		name     string
		recovery []string
	}{
		{"nil recovery", nil},
		{"empty recovery", []string{}},
		{"single item", []string{"one"}},
		{"multiple items", []string{"one", "two", "three"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := RenderedError{
				Code:     "TEST",
				Message:  "test",
				Recovery: tt.recovery,
				ExitCode: 1,
			}

			result := FormatJSON(rendered)

			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			recovery, ok := parsed["recovery"]
			if !ok {
				t.Fatal("Missing recovery field")
			}

			// Recovery should always be an array (or null which becomes nil slice)
			switch v := recovery.(type) {
			case []interface{}:
				// Expected - it's an array
			case nil:
				// Acceptable for nil input
			default:
				t.Errorf("Recovery is type %T, want array", v)
			}
		})
	}
}

// TestRenderNodeJSON_NoHTMLEscape tests that HTML characters are not escaped in JSON output.
// This addresses issue vibefeld-9cd0 where 'k>=0' was being rendered as 'k\u003e=0'.
func TestRenderNodeJSON_NoHTMLEscape(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		wantIn    string   // substring that should be present in output
		wantNotIn []string // substrings that should NOT be present (escaped forms)
	}{
		{
			name:      "greater than symbol",
			statement: "k>=0",
			wantIn:    `"statement":"k>=0"`,
			wantNotIn: []string{`\u003e`, `\u003E`},
		},
		{
			name:      "less than symbol",
			statement: "x<10",
			wantIn:    `"statement":"x<10"`,
			wantNotIn: []string{`\u003c`, `\u003C`},
		},
		{
			name:      "ampersand symbol",
			statement: "a && b",
			wantIn:    `"statement":"a && b"`,
			wantNotIn: []string{`\u0026`},
		},
		{
			name:      "mixed HTML characters",
			statement: "x<10 && y>5",
			wantIn:    `"statement":"x<10 && y>5"`,
			wantNotIn: []string{`\u003c`, `\u003e`, `\u0026`},
		},
		{
			name:      "HTML tag-like content",
			statement: "for all <T> where T implements Comparable",
			wantIn:    "<T>",
			wantNotIn: []string{`\u003c`, `\u003e`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse("1")
			if err != nil {
				t.Fatalf("failed to parse node ID: %v", err)
			}
			n, err := node.NewNode(nodeID, schema.NodeTypeClaim, tt.statement, schema.InferenceModusPonens)
			if err != nil {
				t.Fatalf("failed to create node: %v", err)
			}

			result := RenderNodeJSON(n)

			// Should be valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("RenderNodeJSON produced invalid JSON: %v\nOutput: %s", err, result)
			}

			// Should contain the unescaped version
			if !strings.Contains(result, tt.wantIn) {
				t.Errorf("JSON output missing expected content %q\ngot: %s", tt.wantIn, result)
			}

			// Should NOT contain escaped forms
			for _, notWant := range tt.wantNotIn {
				if strings.Contains(result, notWant) {
					t.Errorf("JSON output should not contain escaped form %q\ngot: %s", notWant, result)
				}
			}

			// Verify the statement round-trips correctly
			if statement, ok := parsed["statement"].(string); ok {
				if statement != tt.statement {
					t.Errorf("Statement = %q, want %q", statement, tt.statement)
				}
			} else {
				t.Error("Missing or invalid statement field in JSON")
			}
		})
	}
}

// TestRenderNodeListJSON_NoHTMLEscape tests that HTML characters are not escaped in node list JSON.
func TestRenderNodeListJSON_NoHTMLEscape(t *testing.T) {
	nodeID1, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}
	nodeID2, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	n1, err := node.NewNode(nodeID1, schema.NodeTypeClaim, "x >= 0", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create node 1: %v", err)
	}
	n2, err := node.NewNode(nodeID2, schema.NodeTypeClaim, "y < 10 && z > 0", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create node 2: %v", err)
	}

	nodes := []*node.Node{n1, n2}

	result := RenderNodeListJSON(nodes)

	// Should be valid JSON
	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("RenderNodeListJSON produced invalid JSON: %v\nOutput: %s", err, result)
	}

	// Should NOT contain any Unicode-escaped HTML characters
	escapedForms := []string{`\u003c`, `\u003e`, `\u0026`, `\u003C`, `\u003E`}
	for _, escaped := range escapedForms {
		if strings.Contains(result, escaped) {
			t.Errorf("JSON output should not contain escaped form %q\ngot: %s", escaped, result)
		}
	}

	// Verify statements round-trip correctly
	if len(parsed) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(parsed))
	}

	if statement := parsed[0]["statement"].(string); statement != "x >= 0" {
		t.Errorf("Node 1 statement = %q, want %q", statement, "x >= 0")
	}

	if statement := parsed[1]["statement"].(string); statement != "y < 10 && z > 0" {
		t.Errorf("Node 2 statement = %q, want %q", statement, "y < 10 && z > 0")
	}
}
