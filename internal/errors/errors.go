// Package errors provides structured error types for the AF framework.
// All errors carry an ErrorCode that maps to exit codes:
// - Exit 1: retriable errors (race conditions, transient failures)
// - Exit 2: blocked errors (work cannot proceed)
// - Exit 3: logic errors (invalid input, not found, scope violations)
// - Exit 4: corruption errors (data integrity failures)
package errors

import (
	"errors"
	"fmt"
)

// ErrorCode represents a specific error condition in the AF framework.
type ErrorCode int

// Error codes grouped by category.
const (
	// Claim-related errors (retriable = exit 1)
	ALREADY_CLAIMED ErrorCode = iota + 1
	NOT_CLAIM_HOLDER

	// Blocked errors (exit 2)
	NODE_BLOCKED

	// Validation errors (logic = exit 3)
	INVALID_PARENT
	INVALID_TYPE
	INVALID_INFERENCE
	INVALID_TARGET
	EMPTY_INPUT
	INVALID_STATE
	ALREADY_EXISTS
	INVALID_TIMEOUT

	// Not found errors (logic = exit 3)
	NODE_NOT_FOUND
	PARENT_NOT_FOUND
	CHALLENGE_NOT_FOUND
	DEF_NOT_FOUND
	ASSUMPTION_NOT_FOUND
	EXTERNAL_NOT_FOUND

	// Scope errors (logic = exit 3)
	SCOPE_VIOLATION
	SCOPE_UNCLOSED
	DEPENDENCY_CYCLE

	// Corruption/integrity errors (exit 4)
	CONTENT_HASH_MISMATCH
	LEDGER_INCONSISTENT

	// Retriable validation error (exit 1)
	VALIDATION_INVARIANT_FAILED

	// Limit errors (logic = exit 3)
	DEPTH_EXCEEDED
	CHALLENGE_LIMIT_EXCEEDED
	REFINEMENT_LIMIT_EXCEEDED

	// Extraction errors (logic = exit 3)
	EXTRACTION_INVALID
)

// errorCodeNames maps error codes to their string representations.
var errorCodeNames = map[ErrorCode]string{
	ALREADY_CLAIMED:             "ALREADY_CLAIMED",
	NOT_CLAIM_HOLDER:            "NOT_CLAIM_HOLDER",
	NODE_BLOCKED:                "NODE_BLOCKED",
	INVALID_PARENT:              "INVALID_PARENT",
	INVALID_TYPE:                "INVALID_TYPE",
	INVALID_INFERENCE:           "INVALID_INFERENCE",
	INVALID_TARGET:              "INVALID_TARGET",
	EMPTY_INPUT:                 "EMPTY_INPUT",
	INVALID_STATE:               "INVALID_STATE",
	ALREADY_EXISTS:              "ALREADY_EXISTS",
	INVALID_TIMEOUT:             "INVALID_TIMEOUT",
	NODE_NOT_FOUND:              "NODE_NOT_FOUND",
	PARENT_NOT_FOUND:            "PARENT_NOT_FOUND",
	CHALLENGE_NOT_FOUND:         "CHALLENGE_NOT_FOUND",
	DEF_NOT_FOUND:               "DEF_NOT_FOUND",
	ASSUMPTION_NOT_FOUND:        "ASSUMPTION_NOT_FOUND",
	EXTERNAL_NOT_FOUND:          "EXTERNAL_NOT_FOUND",
	SCOPE_VIOLATION:             "SCOPE_VIOLATION",
	SCOPE_UNCLOSED:              "SCOPE_UNCLOSED",
	DEPENDENCY_CYCLE:            "DEPENDENCY_CYCLE",
	CONTENT_HASH_MISMATCH:       "CONTENT_HASH_MISMATCH",
	VALIDATION_INVARIANT_FAILED: "VALIDATION_INVARIANT_FAILED",
	LEDGER_INCONSISTENT:         "LEDGER_INCONSISTENT",
	DEPTH_EXCEEDED:              "DEPTH_EXCEEDED",
	CHALLENGE_LIMIT_EXCEEDED:    "CHALLENGE_LIMIT_EXCEEDED",
	REFINEMENT_LIMIT_EXCEEDED:   "REFINEMENT_LIMIT_EXCEEDED",
	EXTRACTION_INVALID:          "EXTRACTION_INVALID",
}

// String returns the string representation of an ErrorCode.
func (c ErrorCode) String() string {
	if name, ok := errorCodeNames[c]; ok {
		return name
	}
	return ""
}

// ExitCode returns the exit code for this error code.
// Exit codes follow the spec:
// - 1 = retriable errors
// - 2 = blocked errors
// - 3 = logic errors
// - 4 = corruption errors
func (c ErrorCode) ExitCode() int {
	switch c {
	// Exit 1: retriable
	case ALREADY_CLAIMED, NOT_CLAIM_HOLDER, VALIDATION_INVARIANT_FAILED:
		return 1

	// Exit 2: blocked
	case NODE_BLOCKED:
		return 2

	// Exit 4: corruption
	case CONTENT_HASH_MISMATCH, LEDGER_INCONSISTENT:
		return 4

	// Exit 3: logic errors (all others)
	default:
		return 3
	}
}

// AFError is the primary error type for the AF framework.
// It carries an error code, message, and optional wrapped error.
type AFError struct {
	code    ErrorCode
	message string
	wrapped error
}

// New creates a new AFError with the given code and message.
func New(code ErrorCode, msg string) *AFError {
	return &AFError{
		code:    code,
		message: msg,
	}
}

// Newf creates a new AFError with a formatted message.
func Newf(code ErrorCode, format string, args ...any) *AFError {
	return &AFError{
		code:    code,
		message: fmt.Sprintf(format, args...),
	}
}

// Wrap wraps an error with additional context.
func Wrap(err error, context string) *AFError {
	if err == nil {
		return nil
	}

	// Extract the code from the underlying error if it's an AFError
	code := Code(err)

	return &AFError{
		code:    code,
		message: context,
		wrapped: err,
	}
}

// Error implements the error interface.
func (e *AFError) Error() string {
	if e.wrapped != nil {
		return fmt.Sprintf("%s: %s: %v", e.code.String(), e.message, e.wrapped)
	}
	return fmt.Sprintf("%s: %s", e.code.String(), e.message)
}

// Is implements errors.Is comparison.
// Two AFErrors are considered equal if they have the same error code.
func (e *AFError) Is(target error) bool {
	var t *AFError
	if errors.As(target, &t) {
		return e.code == t.code
	}
	return false
}

// Unwrap returns the wrapped error, if any.
func (e *AFError) Unwrap() error {
	return e.wrapped
}

// Code extracts the ErrorCode from an error.
// If the error is not an AFError (or wraps one), returns the zero value.
func Code(err error) ErrorCode {
	if err == nil {
		return ErrorCode(0)
	}

	var afErr *AFError
	if errors.As(err, &afErr) {
		return afErr.code
	}

	return ErrorCode(0)
}

// IsRetriable returns true if the error is retriable (exit code 1).
func IsRetriable(err error) bool {
	if err == nil {
		return false
	}
	code := Code(err)
	if code == ErrorCode(0) {
		return false
	}
	return code.ExitCode() == 1
}

// IsBlocked returns true if the error indicates a blocked state (exit code 2).
func IsBlocked(err error) bool {
	if err == nil {
		return false
	}
	code := Code(err)
	if code == ErrorCode(0) {
		return false
	}
	return code.ExitCode() == 2
}

// IsCorruption returns true if the error indicates data corruption (exit code 4).
func IsCorruption(err error) bool {
	if err == nil {
		return false
	}
	code := Code(err)
	if code == ErrorCode(0) {
		return false
	}
	return code.ExitCode() == 4
}

// ExitCode returns the appropriate exit code for an error.
// Returns 0 for nil, 1 for non-AFError errors, or the error code's exit code.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	code := Code(err)
	if code == ErrorCode(0) {
		// Non-AFError defaults to exit code 1 (retriable)
		return 1
	}

	return code.ExitCode()
}

// afPathMarker is the directory marker used to identify AF-related paths.
const afPathMarker = ".af"

// SanitizePaths removes sensitive directory prefixes from file paths in error messages.
// It looks for paths containing ".af" and strips everything before it, keeping only
// the relative path from .af onwards. This prevents leaking filesystem structure
// in user-facing error messages.
//
// Examples:
//   - "/home/user/project/.af/ledger/0001.json" -> ".af/ledger/0001.json"
//   - "C:\Users\dev\.af\config.json" -> ".af/config.json"
//   - "failed to read .af/file.json" -> "failed to read .af/file.json" (unchanged)
func SanitizePaths(s string) string {
	if s == "" {
		return s
	}

	result := s

	// Find and replace Unix-style paths (starts with /)
	// Pattern: /.../.af/... -> .af/...
	result = sanitizeUnixPaths(result)

	// Find and replace Windows-style paths (starts with drive letter like C:\)
	// Pattern: C:\...\\.af\... -> .af/...
	result = sanitizeWindowsPaths(result)

	return result
}

// sanitizeUnixPaths finds Unix-style absolute paths containing .af and strips the prefix.
func sanitizeUnixPaths(s string) string {
	result := []byte(s)
	i := 0

	for i < len(result) {
		// Look for start of absolute path
		if result[i] != '/' {
			i++
			continue
		}

		// Found a '/', look for .af in this path
		pathStart := i
		pathEnd := pathStart

		// Find the end of this path (next space or end of string)
		for pathEnd < len(result) && result[pathEnd] != ' ' && result[pathEnd] != ':' {
			pathEnd++
		}
		// Include colon if it's part of error message like "path: error"
		// but we want to stop before the space after colon
		if pathEnd < len(result) && result[pathEnd] == ':' {
			// Keep the colon as part of path end detection but don't include it
		}

		pathStr := string(result[pathStart:pathEnd])

		// Find .af in this path
		afIdx := findAFMarker(pathStr)
		if afIdx != -1 {
			// Replace the absolute path with relative .af path
			relativePath := pathStr[afIdx:]
			newResult := make([]byte, 0, len(result)-(pathEnd-pathStart)+len(relativePath))
			newResult = append(newResult, result[:pathStart]...)
			newResult = append(newResult, relativePath...)
			newResult = append(newResult, result[pathEnd:]...)
			result = newResult
			i = pathStart + len(relativePath)
		} else {
			i = pathEnd
		}
	}

	return string(result)
}

// sanitizeWindowsPaths finds Windows-style paths containing .af and strips the prefix.
func sanitizeWindowsPaths(s string) string {
	result := []byte(s)
	i := 0

	for i < len(result)-1 {
		// Look for drive letter pattern (e.g., "C:\")
		if !isLetter(result[i]) || result[i+1] != ':' {
			i++
			continue
		}
		// Check for backslash after drive letter
		if i+2 >= len(result) || result[i+2] != '\\' {
			i++
			continue
		}

		pathStart := i
		pathEnd := pathStart

		// Find the end of this path (next space or end of string)
		for pathEnd < len(result) && result[pathEnd] != ' ' {
			pathEnd++
		}

		pathStr := string(result[pathStart:pathEnd])

		// Find .af in this path (handle both \ and /)
		afIdx := findAFMarkerWindows(pathStr)
		if afIdx != -1 {
			// Replace with Unix-style relative path
			relativePath := normalizeToUnix(pathStr[afIdx:])
			newResult := make([]byte, 0, len(result)-(pathEnd-pathStart)+len(relativePath))
			newResult = append(newResult, result[:pathStart]...)
			newResult = append(newResult, relativePath...)
			newResult = append(newResult, result[pathEnd:]...)
			result = newResult
			i = pathStart + len(relativePath)
		} else {
			i = pathEnd
		}
	}

	return string(result)
}

// findAFMarker finds the index of ".af/" or ".af" at end of path in a Unix-style path.
func findAFMarker(path string) int {
	// Look for /.af/ or /.af at end
	marker := "/" + afPathMarker + "/"
	idx := 0
	for {
		pos := indexAt(path, marker, idx)
		if pos == -1 {
			break
		}
		return pos + 1 // Skip the leading /
	}

	// Check for .af at end of path
	marker = "/" + afPathMarker
	if len(path) >= len(marker) && path[len(path)-len(marker):] == marker {
		return len(path) - len(afPathMarker)
	}

	return -1
}

// findAFMarkerWindows finds .af in a Windows path with either \ or / separators.
func findAFMarkerWindows(path string) int {
	// Look for \.af\ or /.af/
	for i := 0; i < len(path)-len(afPathMarker); i++ {
		if (path[i] == '\\' || path[i] == '/') &&
			path[i+1:i+1+len(afPathMarker)] == afPathMarker &&
			(i+1+len(afPathMarker) >= len(path) ||
				path[i+1+len(afPathMarker)] == '\\' ||
				path[i+1+len(afPathMarker)] == '/') {
			return i + 1
		}
	}
	return -1
}

// indexAt returns the index of substr in s starting from offset, or -1 if not found.
func indexAt(s, substr string, offset int) int {
	if offset >= len(s) {
		return -1
	}
	for i := offset; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// isLetter returns true if b is an ASCII letter.
func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// normalizeToUnix converts Windows backslashes to Unix forward slashes.
func normalizeToUnix(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			result[i] = '/'
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}

// SanitizeError wraps an error with sanitized file paths in its message.
// Returns nil if err is nil.
func SanitizeError(err error) error {
	if err == nil {
		return nil
	}
	sanitized := SanitizePaths(err.Error())
	if sanitized == err.Error() {
		return err // No change needed
	}
	return fmt.Errorf("%s", sanitized)
}
