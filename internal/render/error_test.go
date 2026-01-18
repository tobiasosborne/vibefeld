package render

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/errors"
)

// TestRenderError_AFError tests rendering of AFError types
func TestRenderError_AFError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantCode     string
		wantExitCode int
		wantRecovery bool // whether recovery suggestions should exist
	}{
		{
			name:         "ALREADY_CLAIMED error",
			err:          errors.New(errors.ALREADY_CLAIMED, "node 1.2 is claimed by agent-xyz"),
			wantCode:     "ALREADY_CLAIMED",
			wantExitCode: 1,
			wantRecovery: true,
		},
		{
			name:         "NOT_CLAIM_HOLDER error",
			err:          errors.New(errors.NOT_CLAIM_HOLDER, "agent-abc does not hold claim on node 1.2"),
			wantCode:     "NOT_CLAIM_HOLDER",
			wantExitCode: 1,
			wantRecovery: true,
		},
		{
			name:         "NODE_BLOCKED error",
			err:          errors.New(errors.NODE_BLOCKED, "node 1.3 is blocked by pending challenge"),
			wantCode:     "NODE_BLOCKED",
			wantExitCode: 2,
			wantRecovery: true,
		},
		{
			name:         "INVALID_PARENT error",
			err:          errors.New(errors.INVALID_PARENT, "parent node 1.5 does not exist"),
			wantCode:     "INVALID_PARENT",
			wantExitCode: 3,
			wantRecovery: true,
		},
		{
			name:         "LEDGER_INCONSISTENT error",
			err:          errors.New(errors.LEDGER_INCONSISTENT, "sequence gap detected at line 42"),
			wantCode:     "LEDGER_INCONSISTENT",
			wantExitCode: 4,
			wantRecovery: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderError(tt.err)

			if result.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", result.Code, tt.wantCode)
			}

			if result.ExitCode != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tt.wantExitCode)
			}

			if result.Message == "" {
				t.Error("Message is empty, want non-empty message")
			}

			if tt.wantRecovery && len(result.Recovery) == 0 {
				t.Error("Recovery suggestions are empty, want at least one suggestion")
			}
		})
	}
}

// TestRenderError_GenericError tests rendering of non-AFError types
func TestRenderError_GenericError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantExitCode int
	}{
		{
			name:         "standard error",
			err:          fmt.Errorf("something went wrong"),
			wantExitCode: 1, // generic errors default to exit 1 (retriable)
		},
		{
			name:         "wrapped standard error",
			err:          fmt.Errorf("outer: %w", fmt.Errorf("inner error")),
			wantExitCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderError(tt.err)

			if result.ExitCode != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tt.wantExitCode)
			}

			if result.Message == "" {
				t.Error("Message is empty, want error message")
			}

			if !strings.Contains(result.Message, tt.err.Error()) {
				t.Errorf("Message = %q, want to contain %q", result.Message, tt.err.Error())
			}
		})
	}
}

// TestRenderError_NilError tests rendering of nil error
func TestRenderError_NilError(t *testing.T) {
	result := RenderError(nil)

	if result.Code != "" {
		t.Errorf("Code = %q, want empty string", result.Code)
	}

	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}

	if result.Message != "" {
		t.Errorf("Message = %q, want empty string", result.Message)
	}

	if len(result.Recovery) != 0 {
		t.Errorf("Recovery = %v, want empty slice", result.Recovery)
	}
}

// TestRenderAFError_AlreadyClaimed tests rendering with specific recovery suggestions
func TestRenderAFError_AlreadyClaimed(t *testing.T) {
	err := errors.New(errors.ALREADY_CLAIMED, "node 1.2 is claimed")
	result := RenderAFError(err)

	if result.Code != "ALREADY_CLAIMED" {
		t.Errorf("Code = %q, want ALREADY_CLAIMED", result.Code)
	}

	if len(result.Recovery) == 0 {
		t.Fatal("Recovery suggestions are empty")
	}

	// Check for expected recovery suggestions
	recoveryText := fmt.Sprintf("%v", result.Recovery)
	wantSuggestions := []string{
		"Wait for current holder to release",
		"af jobs",
	}

	for _, want := range wantSuggestions {
		if !strings.Contains(recoveryText, want) {
			t.Errorf("Recovery suggestions missing %q, got: %v", want, result.Recovery)
		}
	}
}

// TestRenderAFError_NotClaimHolder tests NOT_CLAIM_HOLDER recovery suggestions
func TestRenderAFError_NotClaimHolder(t *testing.T) {
	err := errors.New(errors.NOT_CLAIM_HOLDER, "agent-abc does not hold claim")
	result := RenderAFError(err)

	if result.Code != "NOT_CLAIM_HOLDER" {
		t.Errorf("Code = %q, want NOT_CLAIM_HOLDER", result.Code)
	}

	if len(result.Recovery) == 0 {
		t.Fatal("Recovery suggestions are empty")
	}

	recoveryText := fmt.Sprintf("%v", result.Recovery)
	if !strings.Contains(recoveryText, "af claim") {
		t.Errorf("Recovery suggestions missing 'af claim', got: %v", result.Recovery)
	}
}

// TestRenderAFError_NodeBlocked tests NODE_BLOCKED recovery suggestions
func TestRenderAFError_NodeBlocked(t *testing.T) {
	err := errors.New(errors.NODE_BLOCKED, "node 1.3 is blocked")
	result := RenderAFError(err)

	if result.Code != "NODE_BLOCKED" {
		t.Errorf("Code = %q, want NODE_BLOCKED", result.Code)
	}

	if len(result.Recovery) == 0 {
		t.Fatal("Recovery suggestions are empty")
	}

	recoveryText := fmt.Sprintf("%v", result.Recovery)
	// Context-aware suggestions should include the node ID from the error message
	wantSuggestions := []string{
		"af get 1.3",         // Context-specific command with node ID
		"af challenges 1.3",  // Context-specific command with node ID
		"af pending-defs",    // General suggestion
	}

	for _, want := range wantSuggestions {
		if !strings.Contains(recoveryText, want) {
			t.Errorf("Recovery suggestions missing %q, got: %v", want, result.Recovery)
		}
	}
}

// TestRenderAFError_InvalidParent tests INVALID_PARENT recovery suggestions
func TestRenderAFError_InvalidParent(t *testing.T) {
	err := errors.New(errors.INVALID_PARENT, "parent node does not exist")
	result := RenderAFError(err)

	if result.Code != "INVALID_PARENT" {
		t.Errorf("Code = %q, want INVALID_PARENT", result.Code)
	}

	if len(result.Recovery) == 0 {
		t.Fatal("Recovery suggestions are empty")
	}

	recoveryText := fmt.Sprintf("%v", result.Recovery)
	if !strings.Contains(recoveryText, "af status") {
		t.Errorf("Recovery suggestions missing 'af status', got: %v", result.Recovery)
	}
}

// TestRenderAFError_LedgerCorrupted tests corruption error recovery suggestions
func TestRenderAFError_LedgerCorrupted(t *testing.T) {
	tests := []struct {
		name string
		code errors.ErrorCode
		want string
	}{
		{
			name: "CONTENT_HASH_MISMATCH",
			code: errors.CONTENT_HASH_MISMATCH,
			want: "do not modify .af files manually",
		},
		{
			name: "LEDGER_INCONSISTENT",
			code: errors.LEDGER_INCONSISTENT,
			want: "Contact administrator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.code, "corruption detected")
			result := RenderAFError(err)

			if len(result.Recovery) == 0 {
				t.Fatal("Recovery suggestions are empty")
			}

			recoveryText := fmt.Sprintf("%v", result.Recovery)
			if !strings.Contains(recoveryText, tt.want) {
				t.Errorf("Recovery suggestions missing %q, got: %v", tt.want, result.Recovery)
			}
		})
	}
}

// TestRenderAFError_ExitCodes tests that exit codes are correctly mapped
func TestRenderAFError_ExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		wantExit int
	}{
		{"ALREADY_CLAIMED returns 1", errors.ALREADY_CLAIMED, 1},
		{"NOT_CLAIM_HOLDER returns 1", errors.NOT_CLAIM_HOLDER, 1},
		{"VALIDATION_INVARIANT_FAILED returns 1", errors.VALIDATION_INVARIANT_FAILED, 1},
		{"NODE_BLOCKED returns 2", errors.NODE_BLOCKED, 2},
		{"INVALID_PARENT returns 3", errors.INVALID_PARENT, 3},
		{"INVALID_TYPE returns 3", errors.INVALID_TYPE, 3},
		{"SCOPE_VIOLATION returns 3", errors.SCOPE_VIOLATION, 3},
		{"CONTENT_HASH_MISMATCH returns 4", errors.CONTENT_HASH_MISMATCH, 4},
		{"LEDGER_INCONSISTENT returns 4", errors.LEDGER_INCONSISTENT, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.code, "test message")
			result := RenderAFError(err)

			if result.ExitCode != tt.wantExit {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tt.wantExit)
			}
		})
	}
}

// TestFormatCLI_SingleRecovery tests CLI formatting with one recovery suggestion
func TestFormatCLI_SingleRecovery(t *testing.T) {
	rendered := RenderedError{
		Code:     "INVALID_PARENT",
		Message:  "parent node 1.5 does not exist",
		Recovery: []string{"Check parent node exists with 'af status'"},
		ExitCode: 3,
	}

	result := FormatCLI(rendered)

	if result == "" {
		t.Fatal("FormatCLI returned empty string")
	}

	// Should contain the error code
	if !strings.Contains(result, "INVALID_PARENT") {
		t.Errorf("FormatCLI missing code, got: %q", result)
	}

	// Should contain the message
	if !strings.Contains(result, "parent node 1.5 does not exist") {
		t.Errorf("FormatCLI missing message, got: %q", result)
	}

	// Should contain the recovery suggestion
	if !strings.Contains(result, "Check parent node exists") {
		t.Errorf("FormatCLI missing recovery suggestion, got: %q", result)
	}
}

// TestFormatCLI_MultipleRecovery tests CLI formatting with multiple recovery suggestions
func TestFormatCLI_MultipleRecovery(t *testing.T) {
	rendered := RenderedError{
		Code:    "ALREADY_CLAIMED",
		Message: "node 1.2 is already claimed",
		Recovery: []string{
			"Wait for current holder to release",
			"Use 'af jobs' to find available nodes",
			"Check claim status with 'af status'",
		},
		ExitCode: 1,
	}

	result := FormatCLI(rendered)

	if result == "" {
		t.Fatal("FormatCLI returned empty string")
	}

	// Should contain all recovery suggestions
	for _, suggestion := range rendered.Recovery {
		if !strings.Contains(result, suggestion) {
			t.Errorf("FormatCLI missing suggestion %q, got: %q", suggestion, result)
		}
	}
}

// TestFormatCLI_NoRecovery tests CLI formatting with no recovery suggestions
func TestFormatCLI_NoRecovery(t *testing.T) {
	rendered := RenderedError{
		Code:     "UNKNOWN_ERROR",
		Message:  "an unknown error occurred",
		Recovery: []string{},
		ExitCode: 1,
	}

	result := FormatCLI(rendered)

	if result == "" {
		t.Fatal("FormatCLI returned empty string")
	}

	// Should still contain code and message
	if !strings.Contains(result, "UNKNOWN_ERROR") {
		t.Errorf("FormatCLI missing code, got: %q", result)
	}

	if !strings.Contains(result, "an unknown error occurred") {
		t.Errorf("FormatCLI missing message, got: %q", result)
	}
}

// TestFormatJSON_ValidJSON tests that JSON output is valid
func TestFormatJSON_ValidJSON(t *testing.T) {
	rendered := RenderedError{
		Code:    "NODE_BLOCKED",
		Message: "node 1.3 is blocked by pending challenge",
		Recovery: []string{
			"Resolve blocking challenges first",
			"Use 'af status' to see blockers",
		},
		ExitCode: 2,
	}

	result := FormatJSON(rendered)

	if result == "" {
		t.Fatal("FormatJSON returned empty string")
	}

	// Try to parse as JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatJSON produced invalid JSON: %v\nOutput: %s", err, result)
	}

	// Check required fields exist
	if _, ok := parsed["code"]; !ok {
		t.Error("JSON missing 'code' field")
	}

	if _, ok := parsed["message"]; !ok {
		t.Error("JSON missing 'message' field")
	}

	if _, ok := parsed["exit_code"]; !ok {
		t.Error("JSON missing 'exit_code' field")
	}
}

// TestFormatJSON_Roundtrip tests that JSON can be parsed back correctly
func TestFormatJSON_Roundtrip(t *testing.T) {
	original := RenderedError{
		Code:    "SCOPE_VIOLATION",
		Message: "cannot reference assumption outside scope",
		Recovery: []string{
			"Check assumption is in scope",
			"Review scope boundaries",
		},
		ExitCode: 3,
	}

	jsonStr := FormatJSON(original)

	// Parse back
	var parsed struct {
		Code     string   `json:"code"`
		Message  string   `json:"message"`
		Recovery []string `json:"recovery"`
		ExitCode int      `json:"exit_code"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Compare fields
	if parsed.Code != original.Code {
		t.Errorf("Code = %q, want %q", parsed.Code, original.Code)
	}

	if parsed.Message != original.Message {
		t.Errorf("Message = %q, want %q", parsed.Message, original.Message)
	}

	if parsed.ExitCode != original.ExitCode {
		t.Errorf("ExitCode = %d, want %d", parsed.ExitCode, original.ExitCode)
	}

	if len(parsed.Recovery) != len(original.Recovery) {
		t.Fatalf("Recovery length = %d, want %d", len(parsed.Recovery), len(original.Recovery))
	}

	for i, rec := range parsed.Recovery {
		if rec != original.Recovery[i] {
			t.Errorf("Recovery[%d] = %q, want %q", i, rec, original.Recovery[i])
		}
	}
}

// TestRenderError_WrappedAFError tests rendering wrapped AFErrors
func TestRenderError_WrappedAFError(t *testing.T) {
	inner := errors.New(errors.INVALID_TYPE, "expected step, got assumption")
	wrapped := errors.Wrap(inner, "while validating node 1.2")

	result := RenderError(wrapped)

	if result.Code != "INVALID_TYPE" {
		t.Errorf("Code = %q, want INVALID_TYPE", result.Code)
	}

	if result.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", result.ExitCode)
	}

	// Message should contain context from wrapping
	if !strings.Contains(result.Message, "validating") {
		t.Errorf("Message missing wrapping context, got: %q", result.Message)
	}
}

// TestRenderError_AllErrorCodes tests that all error codes have recovery suggestions
func TestRenderError_AllErrorCodes(t *testing.T) {
	// Test all defined error codes to ensure they have recovery suggestions
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
			result := RenderAFError(err)

			if result.Code != code.String() {
				t.Errorf("Code = %q, want %q", result.Code, code.String())
			}

			// All error codes should have at least one recovery suggestion
			if len(result.Recovery) == 0 {
				t.Errorf("Error code %s has no recovery suggestions", code.String())
			}
		})
	}
}

// TestExtractNodeID tests node ID extraction from error messages
func TestExtractNodeID(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		{"node 1.2 is claimed", "1.2"},
		{"node 1.2.3 is blocked", "1.2.3"},
		{"cannot refine node 1", "1"},
		{"node 10.20.30 exceeded limit", "10.20.30"},
		{"no node ID here", ""},
		{"invalid 1. ID", ""},
		{"just a number 42 not a node", "42"},
		{"NODE_BLOCKED: node 1.3 is blocked by challenge", "1.3"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			got := extractNodeID(tt.message)
			if got != tt.want {
				t.Errorf("extractNodeID(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

// TestIsNodeID tests the node ID pattern matcher
func TestIsNodeID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1", true},
		{"1.2", true},
		{"1.2.3", true},
		{"10.20.30", true},
		{"", false},
		{"a.b", false},
		{"1.", false},
		{"1.2.", false},
		{".1", false},
		{"1..2", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isNodeID(tt.input)
			if got != tt.want {
				t.Errorf("isNodeID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestExtractQuotedValue tests quoted value extraction from error messages
func TestExtractQuotedValue(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		{`definition "continuity" not found`, "continuity"},
		{`assumption "hyp1" not in scope`, "hyp1"},
		{`external "ZFC" not found`, "ZFC"},
		{"no quotes here", ""},
		{`only "one quote`, ""},
		{`empty ""`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			got := extractQuotedValue(tt.message)
			if got != tt.want {
				t.Errorf("extractQuotedValue(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

// TestContextAwareRecovery tests that recovery suggestions incorporate context
func TestContextAwareRecovery(t *testing.T) {
	tests := []struct {
		name    string
		code    errors.ErrorCode
		message string
		want    string // Substring that should appear in suggestions
	}{
		{
			name:    "ALREADY_CLAIMED with node ID",
			code:    errors.ALREADY_CLAIMED,
			message: "node 1.5 is claimed by agent-xyz",
			want:    "af get 1.5",
		},
		{
			name:    "NOT_CLAIM_HOLDER with node ID",
			code:    errors.NOT_CLAIM_HOLDER,
			message: "node 2.1 must be claimed first",
			want:    "af claim 2.1",
		},
		{
			name:    "DEF_NOT_FOUND with quoted name",
			code:    errors.DEF_NOT_FOUND,
			message: `definition "continuity" not found`,
			want:    "af request-def continuity",
		},
		{
			name:    "CHALLENGE_LIMIT_EXCEEDED with node ID",
			code:    errors.CHALLENGE_LIMIT_EXCEEDED,
			message: "node 1.2.3 has reached challenge limit",
			want:    "af challenges 1.2.3",
		},
		{
			name:    "DEF_NOT_FOUND mentions pending-defs",
			code:    errors.DEF_NOT_FOUND,
			message: `definition "continuity" not found`,
			want:    "af pending-defs",
		},
		{
			name:    "DEF_NOT_FOUND mentions def-add",
			code:    errors.DEF_NOT_FOUND,
			message: `definition "limit" not found`,
			want:    "af def-add",
		},
		{
			name:    "ASSUMPTION_NOT_FOUND mentions scope",
			code:    errors.ASSUMPTION_NOT_FOUND,
			message: `assumption "hyp1" not found`,
			want:    "af scope",
		},
		{
			name:    "EXTERNAL_NOT_FOUND mentions externals",
			code:    errors.EXTERNAL_NOT_FOUND,
			message: `external "FLT" not found`,
			want:    "af externals",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := getRecoverySuggestions(tt.code, tt.message)
			found := false
			for _, s := range suggestions {
				if strings.Contains(s, tt.want) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Recovery suggestions for %s should contain %q, got: %v",
					tt.name, tt.want, suggestions)
			}
		})
	}
}

