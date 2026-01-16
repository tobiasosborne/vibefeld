// Package render provides human-readable formatting for AF framework types.
// Errors are rendered with recovery suggestions to guide users toward resolution.
package render

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/tobias/vibefeld/internal/errors"
)

// RenderedError contains a human-readable error with recovery suggestions.
// It provides both CLI-friendly and JSON formatting for error display.
type RenderedError struct {
	Code       string   // Error code (e.g., "ALREADY_CLAIMED")
	Message    string   // Human-readable error message
	Recovery   []string // Suggested recovery actions
	ExitCode   int      // Process exit code (0, 1, 2, 3, or 4)
	JSONOutput string   // Pre-computed JSON representation (optional)
}

// RenderError renders any error for display.
// If the error is an AFError, it includes recovery suggestions.
// Generic errors are rendered with default exit code 1 (retriable).
// Returns a zero-value RenderedError for nil errors.
func RenderError(err error) RenderedError {
	if err == nil {
		return RenderedError{}
	}

	// Check if it's an AFError
	code := errors.Code(err)
	if code == errors.ErrorCode(0) {
		// Generic error - return basic rendering
		return RenderedError{
			Code:     "",
			Message:  err.Error(),
			Recovery: []string{},
			ExitCode: 1, // Default to retriable
		}
	}

	// It's an AFError - extract and render with recovery suggestions
	var afErr *errors.AFError
	if !stderrors.As(err, &afErr) {
		// Shouldn't happen if Code() returned non-zero, but handle gracefully
		return RenderedError{
			Code:     code.String(),
			Message:  err.Error(),
			Recovery: []string{},
			ExitCode: code.ExitCode(),
		}
	}

	return RenderAFError(afErr)
}

// RenderAFError renders an AFError with recovery suggestions.
// Recovery suggestions are tailored to each error code to guide users
// toward resolution.
func RenderAFError(e *errors.AFError) RenderedError {
	if e == nil {
		return RenderedError{}
	}

	code := errors.Code(e)

	return RenderedError{
		Code:     code.String(),
		Message:  e.Error(),
		Recovery: getRecoverySuggestions(code),
		ExitCode: code.ExitCode(),
	}
}

// getRecoverySuggestions returns recovery suggestions for a given error code.
// These suggestions guide users toward resolving the error condition.
func getRecoverySuggestions(code errors.ErrorCode) []string {
	switch code {
	case errors.ALREADY_CLAIMED:
		return []string{
			"Wait for current holder to release the node",
			"Use 'af jobs' to find available nodes",
			"Check claim status with 'af status <node>'",
		}

	case errors.NOT_CLAIM_HOLDER:
		return []string{
			"Claim the node first with 'af claim <node>'",
			"Verify you are working on the correct node",
		}

	case errors.NODE_BLOCKED:
		return []string{
			"Resolve blocking challenges first",
			"Use 'af status <node>' to see blockers",
			"Address pending challenges or definitions",
		}

	case errors.INVALID_PARENT:
		return []string{
			"Check parent node exists with 'af status'",
			"Verify the parent node ID is correct",
			"Use 'af status' to view the proof tree",
		}

	case errors.INVALID_TYPE:
		return []string{
			"Review the node type requirements",
			"Check the proof structure with 'af status'",
			"Consult the AF documentation for valid node types",
		}

	case errors.INVALID_INFERENCE:
		return []string{
			"Review the inference rule requirements",
			"Check that premises support the conclusion",
			"Use 'af status' to verify node relationships",
		}

	case errors.INVALID_TARGET:
		return []string{
			"Verify the target node exists",
			"Check the target node ID is correct",
			"Use 'af status' to view available nodes",
		}

	case errors.CHALLENGE_NOT_FOUND:
		return []string{
			"Verify the challenge ID is correct",
			"Use 'af status' to list active challenges",
			"The challenge may have been resolved or withdrawn",
		}

	case errors.DEF_NOT_FOUND:
		return []string{
			"Add the required definition with 'af define'",
			"Check the definition key is spelled correctly",
			"Use 'af status' to list available definitions",
		}

	case errors.ASSUMPTION_NOT_FOUND:
		return []string{
			"Verify the assumption exists in the current scope",
			"Check the assumption ID is correct",
			"Use 'af status' to view assumptions in scope",
		}

	case errors.EXTERNAL_NOT_FOUND:
		return []string{
			"Add the required external reference",
			"Verify the external reference key",
			"Use 'af status' to list external references",
		}

	case errors.SCOPE_VIOLATION:
		return []string{
			"Ensure assumptions are referenced within their scope",
			"Check scope boundaries with 'af status'",
			"Review the proof structure for scope violations",
		}

	case errors.SCOPE_UNCLOSED:
		return []string{
			"Close open assumption scopes",
			"Verify all assumptions are properly discharged",
			"Use 'af status' to check scope state",
		}

	case errors.DEPENDENCY_CYCLE:
		return []string{
			"Review node dependencies for circular references",
			"Restructure the proof to eliminate cycles",
			"Use 'af status' to visualize dependencies",
		}

	case errors.CONTENT_HASH_MISMATCH:
		return []string{
			"Contact administrator - data integrity issue detected",
			"Check filesystem integrity",
			"Do not modify ledger files manually",
		}

	case errors.VALIDATION_INVARIANT_FAILED:
		return []string{
			"Retry the operation - this may be a transient issue",
			"Check for concurrent modifications",
			"Contact administrator if problem persists",
		}

	case errors.LEDGER_INCONSISTENT:
		return []string{
			"Contact administrator - ledger corruption detected",
			"Check filesystem integrity",
			"Verify no manual ledger modifications were made",
		}

	case errors.DEPTH_EXCEEDED:
		return []string{
			"Simplify the proof structure",
			"Consider breaking complex proofs into lemmas",
			"Review depth limit configuration",
		}

	case errors.CHALLENGE_LIMIT_EXCEEDED:
		return []string{
			"Resolve existing challenges before raising new ones",
			"Consider withdrawing resolved challenges",
			"Review challenge limit configuration",
		}

	case errors.REFINEMENT_LIMIT_EXCEEDED:
		return []string{
			"Accept or refute current refinements",
			"Consider the proof step may need restructuring",
			"Review refinement limit configuration",
		}

	case errors.EXTRACTION_INVALID:
		return []string{
			"Review the lemma extraction criteria",
			"Ensure the target node is suitable for extraction",
			"Check the proof is complete and validated",
		}

	default:
		// Fallback for unknown error codes
		return []string{
			"Use 'af status' to check current state",
			"Review recent operations for issues",
			"Consult AF documentation or contact support",
		}
	}
}

// FormatCLI formats a rendered error for CLI display.
// The format is human-readable with clear sections for error details
// and recovery suggestions.
func FormatCLI(r RenderedError) string {
	var sb strings.Builder

	// Error header
	if r.Code != "" {
		sb.WriteString(fmt.Sprintf("Error: %s\n", r.Code))
	} else {
		sb.WriteString("Error\n")
	}

	// Error message
	if r.Message != "" {
		sb.WriteString(fmt.Sprintf("\n%s\n", r.Message))
	}

	// Recovery suggestions
	if len(r.Recovery) > 0 {
		sb.WriteString("\nSuggested actions:\n")
		for i, suggestion := range r.Recovery {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, suggestion))
		}
	}

	return sb.String()
}

// FormatJSON formats a rendered error as JSON.
// The JSON structure includes all error fields for programmatic consumption.
func FormatJSON(r RenderedError) string {
	// Create a JSON-friendly structure
	output := struct {
		Code     string   `json:"code"`
		Message  string   `json:"message"`
		Recovery []string `json:"recovery"`
		ExitCode int      `json:"exit_code"`
	}{
		Code:     r.Code,
		Message:  r.Message,
		Recovery: r.Recovery,
		ExitCode: r.ExitCode,
	}

	// Marshal to JSON (ignore errors for stub - real implementation would handle)
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		// Fallback to basic JSON if marshaling fails
		return fmt.Sprintf(`{"code":%q,"message":%q,"exit_code":%d}`, r.Code, r.Message, r.ExitCode)
	}

	return string(jsonBytes)
}
