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
		Recovery: getRecoverySuggestions(code, e.Error()),
		ExitCode: code.ExitCode(),
	}
}

// getRecoverySuggestions returns recovery suggestions for a given error code.
// These suggestions guide users toward resolving the error condition.
// The message parameter provides context for more specific suggestions.
func getRecoverySuggestions(code errors.ErrorCode, message string) []string {
	// Extract context from the message
	nodeID := extractNodeID(message)
	defName := extractQuotedValue(message)

	switch code {
	case errors.ALREADY_CLAIMED:
		suggestions := []string{
			"Wait for current holder to release the node",
			"Use 'af jobs' to find available nodes",
		}
		if nodeID != "" {
			suggestions = append(suggestions, fmt.Sprintf("Check claim status with 'af get %s'", nodeID))
		}
		return suggestions

	case errors.NOT_CLAIM_HOLDER:
		if nodeID != "" {
			return []string{
				fmt.Sprintf("Claim the node first with 'af claim %s'", nodeID),
				fmt.Sprintf("Check who owns the claim with 'af get %s'", nodeID),
			}
		}
		return []string{
			"Claim the node first with 'af claim <node>'",
			"Check who owns the claim with 'af get <node>'",
		}

	case errors.NODE_BLOCKED:
		if nodeID != "" {
			return []string{
				fmt.Sprintf("Check blockers with 'af get %s'", nodeID),
				fmt.Sprintf("View challenges with 'af challenges %s'", nodeID),
				"Use 'af pending-defs' to check for pending definitions",
			}
		}
		return []string{
			"Resolve blocking challenges first",
			"Use 'af challenges' to see open challenges",
			"Use 'af pending-defs' to check for pending definitions",
		}

	case errors.INVALID_PARENT:
		if nodeID != "" {
			return []string{
				fmt.Sprintf("Verify parent exists with 'af get %s'", nodeID),
				"Use 'af status' to view the proof tree",
			}
		}
		return []string{
			"Verify the parent node ID exists",
			"Use 'af status' to view the proof tree",
		}

	case errors.INVALID_TYPE:
		return []string{
			"Use 'af types' to see valid node types",
			"Check the node type spelling matches exactly",
		}

	case errors.INVALID_INFERENCE:
		return []string{
			"Use 'af inferences' to see valid inference rules",
			"Check the inference rule spelling matches exactly",
		}

	case errors.INVALID_TARGET:
		if nodeID != "" {
			return []string{
				fmt.Sprintf("Verify node %s exists with 'af get %s'", nodeID, nodeID),
				"Use 'af status' to view available nodes",
			}
		}
		return []string{
			"Verify the target node exists",
			"Use 'af status' to view available nodes",
		}

	case errors.CHALLENGE_NOT_FOUND:
		return []string{
			"Use 'af challenges' to list active challenges",
			"The challenge may have been resolved or withdrawn",
		}

	case errors.DEF_NOT_FOUND:
		if defName != "" {
			return []string{
				fmt.Sprintf("Request the definition with 'af request-def %s \"<description>\"'", defName),
				"Use 'af defs' to list available definitions",
				"Use 'af pending-defs' to see all pending definition requests",
				"Operators can add definitions with 'af def-add <name> \"<definition>\"'",
			}
		}
		return []string{
			"Request the definition with 'af request-def <name> \"<description>\"'",
			"Use 'af defs' to list available definitions",
			"Use 'af pending-defs' to see all pending definition requests",
			"Operators can add definitions with 'af def-add <name> \"<definition>\"'",
		}

	case errors.ASSUMPTION_NOT_FOUND:
		if defName != "" {
			return []string{
				fmt.Sprintf("Check if assumption %s is in scope with 'af scope'", defName),
				"Use 'af assumptions' to list all assumptions",
				"Assumptions are created via 'af refine' with type 'assumption'",
				"Each assumption has a scope - check 'af scope' for boundaries",
			}
		}
		return []string{
			"Use 'af scope' to check assumption scope",
			"Use 'af assumptions' to list all assumptions",
			"Assumptions are created via 'af refine' with type 'assumption'",
			"Each assumption has a scope - check 'af scope' for boundaries",
		}

	case errors.EXTERNAL_NOT_FOUND:
		if defName != "" {
			return []string{
				fmt.Sprintf("Add external reference %s with proper verification", defName),
				"Use 'af pending-refs' to see pending references",
				"External refs are theorems/lemmas from outside this proof",
				"Use 'af externals' to list all external references",
			}
		}
		return []string{
			"Use 'af pending-refs' to see pending references",
			"External references must be added and verified",
			"External refs are theorems/lemmas from outside this proof",
			"Use 'af externals' to list all external references",
		}

	case errors.SCOPE_VIOLATION:
		return []string{
			"Use 'af scope' to check current scope boundaries",
			"Assumptions can only be used within their scope",
		}

	case errors.SCOPE_UNCLOSED:
		return []string{
			"Use 'af scope' to see open assumption scopes",
			"All assumption scopes must be closed before completion",
		}

	case errors.DEPENDENCY_CYCLE:
		return []string{
			"Use 'af deps' to visualize the dependency graph",
			"Restructure dependencies to eliminate the cycle",
		}

	case errors.CONTENT_HASH_MISMATCH:
		return []string{
			"Data integrity issue detected - do not modify .af files manually",
			"Contact administrator if problem persists",
		}

	case errors.VALIDATION_INVARIANT_FAILED:
		return []string{
			"Retry the operation - this may be a transient issue",
			"If problem persists, run 'af replay' to rebuild state",
		}

	case errors.LEDGER_INCONSISTENT:
		return []string{
			"Ledger corruption detected - do not modify .af files manually",
			"Contact administrator for recovery options",
		}

	case errors.DEPTH_EXCEEDED:
		return []string{
			"Consider extracting a lemma with 'af extract-lemma'",
			"Simplify the proof structure to reduce depth",
		}

	case errors.CHALLENGE_LIMIT_EXCEEDED:
		if nodeID != "" {
			return []string{
				fmt.Sprintf("Resolve existing challenges on %s first", nodeID),
				fmt.Sprintf("View challenges with 'af challenges %s'", nodeID),
			}
		}
		return []string{
			"Resolve existing challenges before raising new ones",
			"Use 'af challenges' to see what needs resolution",
		}

	case errors.REFINEMENT_LIMIT_EXCEEDED:
		if nodeID != "" {
			return []string{
				fmt.Sprintf("Accept or refute existing children of %s", nodeID),
				fmt.Sprintf("View children with 'af get %s'", nodeID),
			}
		}
		return []string{
			"Accept or refute existing children before adding more",
			"Use 'af status' to see the current proof structure",
		}

	case errors.EXTRACTION_INVALID:
		if nodeID != "" {
			return []string{
				fmt.Sprintf("Check node %s is validated with 'af get %s'", nodeID, nodeID),
				"Only validated nodes can be extracted as lemmas",
			}
		}
		return []string{
			"Ensure the node is validated before extraction",
			"Use 'af status' to check node states",
		}

	default:
		return []string{
			"Use 'af status' to check current proof state",
			"Use 'af --help' for command reference",
		}
	}
}

// extractNodeID extracts a node ID (like "1.2.3" or "1") from an error message.
func extractNodeID(message string) string {
	// Look for patterns like "node 1.2.3" or "1.2.3 is" or just standalone IDs
	words := strings.Fields(message)
	for i, word := range words {
		// Clean trailing punctuation
		cleaned := strings.TrimRight(word, ",:;")

		// Check if it looks like a node ID (digits and dots)
		if isNodeID(cleaned) {
			return cleaned
		}

		// Check the word after "node"
		if strings.EqualFold(word, "node") && i+1 < len(words) {
			next := strings.TrimRight(words[i+1], ",:;")
			if isNodeID(next) {
				return next
			}
		}
	}
	return ""
}

// isNodeID returns true if s looks like a node ID (e.g., "1", "1.2", "1.2.3").
func isNodeID(s string) bool {
	if s == "" {
		return false
	}
	// Must start with a digit
	if s[0] < '0' || s[0] > '9' {
		return false
	}
	// Must only contain digits and dots, not end with dot, no consecutive dots
	prevWasDot := false
	for i, c := range s {
		if c == '.' {
			if prevWasDot {
				return false // No consecutive dots
			}
			if i == len(s)-1 {
				return false // Cannot end with dot
			}
			prevWasDot = true
		} else if c >= '0' && c <= '9' {
			prevWasDot = false
		} else {
			return false // Invalid character
		}
	}
	return true
}

// extractQuotedValue extracts a quoted value like "foo" from an error message.
func extractQuotedValue(message string) string {
	// Look for "value" pattern
	start := strings.Index(message, "\"")
	if start == -1 {
		return ""
	}
	end := strings.Index(message[start+1:], "\"")
	if end == -1 {
		return ""
	}
	return message[start+1 : start+1+end]
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
