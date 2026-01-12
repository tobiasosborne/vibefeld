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

	// Not found errors (logic = exit 3)
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
