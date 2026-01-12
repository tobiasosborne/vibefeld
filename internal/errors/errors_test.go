package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestErrorCodesExist verifies all required error codes are defined
func TestErrorCodesExist(t *testing.T) {
	tests := []struct {
		name string
		code ErrorCode
		want string
	}{
		// Claim-related errors
		{"ALREADY_CLAIMED", ALREADY_CLAIMED, "ALREADY_CLAIMED"},
		{"NOT_CLAIM_HOLDER", NOT_CLAIM_HOLDER, "NOT_CLAIM_HOLDER"},
		{"NODE_BLOCKED", NODE_BLOCKED, "NODE_BLOCKED"},

		// Validation errors
		{"INVALID_PARENT", INVALID_PARENT, "INVALID_PARENT"},
		{"INVALID_TYPE", INVALID_TYPE, "INVALID_TYPE"},
		{"INVALID_INFERENCE", INVALID_INFERENCE, "INVALID_INFERENCE"},
		{"INVALID_TARGET", INVALID_TARGET, "INVALID_TARGET"},

		// Not found errors
		{"CHALLENGE_NOT_FOUND", CHALLENGE_NOT_FOUND, "CHALLENGE_NOT_FOUND"},
		{"DEF_NOT_FOUND", DEF_NOT_FOUND, "DEF_NOT_FOUND"},
		{"ASSUMPTION_NOT_FOUND", ASSUMPTION_NOT_FOUND, "ASSUMPTION_NOT_FOUND"},
		{"EXTERNAL_NOT_FOUND", EXTERNAL_NOT_FOUND, "EXTERNAL_NOT_FOUND"},

		// Scope errors
		{"SCOPE_VIOLATION", SCOPE_VIOLATION, "SCOPE_VIOLATION"},
		{"SCOPE_UNCLOSED", SCOPE_UNCLOSED, "SCOPE_UNCLOSED"},
		{"DEPENDENCY_CYCLE", DEPENDENCY_CYCLE, "DEPENDENCY_CYCLE"},

		// Corruption/integrity errors
		{"CONTENT_HASH_MISMATCH", CONTENT_HASH_MISMATCH, "CONTENT_HASH_MISMATCH"},
		{"VALIDATION_INVARIANT_FAILED", VALIDATION_INVARIANT_FAILED, "VALIDATION_INVARIANT_FAILED"},
		{"LEDGER_INCONSISTENT", LEDGER_INCONSISTENT, "LEDGER_INCONSISTENT"},

		// Limit errors
		{"DEPTH_EXCEEDED", DEPTH_EXCEEDED, "DEPTH_EXCEEDED"},
		{"CHALLENGE_LIMIT_EXCEEDED", CHALLENGE_LIMIT_EXCEEDED, "CHALLENGE_LIMIT_EXCEEDED"},
		{"REFINEMENT_LIMIT_EXCEEDED", REFINEMENT_LIMIT_EXCEEDED, "REFINEMENT_LIMIT_EXCEEDED"},

		// Extraction errors
		{"EXTRACTION_INVALID", EXTRACTION_INVALID, "EXTRACTION_INVALID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code.String() != tt.want {
				t.Errorf("ErrorCode.String() = %q, want %q", tt.code.String(), tt.want)
			}
		})
	}
}

// TestExitCodes verifies exit codes are correctly mapped per the spec:
// - Exit code 1 = retriable errors
// - Exit code 2 = blocked errors
// - Exit code 3 = logic errors
// - Exit code 4 = corruption errors
func TestExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		exitCode int
	}{
		// Exit code 1 = retriable errors
		{"ALREADY_CLAIMED is retriable", ALREADY_CLAIMED, 1},
		{"NOT_CLAIM_HOLDER is retriable", NOT_CLAIM_HOLDER, 1},
		{"VALIDATION_INVARIANT_FAILED is retriable", VALIDATION_INVARIANT_FAILED, 1},

		// Exit code 2 = blocked errors
		{"NODE_BLOCKED is blocked", NODE_BLOCKED, 2},

		// Exit code 3 = logic errors
		{"INVALID_PARENT is logic error", INVALID_PARENT, 3},
		{"INVALID_TYPE is logic error", INVALID_TYPE, 3},
		{"INVALID_INFERENCE is logic error", INVALID_INFERENCE, 3},
		{"INVALID_TARGET is logic error", INVALID_TARGET, 3},
		{"CHALLENGE_NOT_FOUND is logic error", CHALLENGE_NOT_FOUND, 3},
		{"DEF_NOT_FOUND is logic error", DEF_NOT_FOUND, 3},
		{"ASSUMPTION_NOT_FOUND is logic error", ASSUMPTION_NOT_FOUND, 3},
		{"EXTERNAL_NOT_FOUND is logic error", EXTERNAL_NOT_FOUND, 3},
		{"SCOPE_VIOLATION is logic error", SCOPE_VIOLATION, 3},
		{"SCOPE_UNCLOSED is logic error", SCOPE_UNCLOSED, 3},
		{"DEPENDENCY_CYCLE is logic error", DEPENDENCY_CYCLE, 3},
		{"DEPTH_EXCEEDED is logic error", DEPTH_EXCEEDED, 3},
		{"CHALLENGE_LIMIT_EXCEEDED is logic error", CHALLENGE_LIMIT_EXCEEDED, 3},
		{"REFINEMENT_LIMIT_EXCEEDED is logic error", REFINEMENT_LIMIT_EXCEEDED, 3},
		{"EXTRACTION_INVALID is logic error", EXTRACTION_INVALID, 3},

		// Exit code 4 = corruption errors
		{"CONTENT_HASH_MISMATCH is corruption", CONTENT_HASH_MISMATCH, 4},
		{"LEDGER_INCONSISTENT is corruption", LEDGER_INCONSISTENT, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.code.ExitCode()
			if got != tt.exitCode {
				t.Errorf("%s.ExitCode() = %d, want %d", tt.code, got, tt.exitCode)
			}
		})
	}
}

// TestErrorInterface tests that AFError implements the error interface correctly
func TestErrorInterface(t *testing.T) {
	tests := []struct {
		name        string
		code        ErrorCode
		msg         string
		wantContain string
	}{
		{
			name:        "error message contains code",
			code:        ALREADY_CLAIMED,
			msg:         "node 1.2 is already claimed by agent xyz",
			wantContain: "ALREADY_CLAIMED",
		},
		{
			name:        "error message contains user message",
			code:        NODE_BLOCKED,
			msg:         "node 1.3 is blocked by pending challenge",
			wantContain: "blocked by pending challenge",
		},
		{
			name:        "error message for validation failure",
			code:        INVALID_TYPE,
			msg:         "expected step, got assumption",
			wantContain: "expected step",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.code, tt.msg)
			errStr := err.Error()
			if errStr == "" {
				t.Error("Error() returned empty string")
			}
			if !strings.Contains(errStr, tt.wantContain) {
				t.Errorf("Error() = %q, want to contain %q", errStr, tt.wantContain)
			}
		})
	}
}

// TestErrorsIs tests that errors.Is() works correctly for AFError
func TestErrorsIs(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{
			name:   "same error code matches",
			err:    New(ALREADY_CLAIMED, "first"),
			target: New(ALREADY_CLAIMED, "second"),
			want:   true,
		},
		{
			name:   "different error codes do not match",
			err:    New(ALREADY_CLAIMED, "msg"),
			target: New(NOT_CLAIM_HOLDER, "msg"),
			want:   false,
		},
		{
			name:   "wrapped error matches inner code",
			err:    Wrap(New(NODE_BLOCKED, "inner"), "outer context"),
			target: New(NODE_BLOCKED, "different"),
			want:   true,
		},
		{
			name:   "AFError does not match standard error",
			err:    New(INVALID_PARENT, "msg"),
			target: fmt.Errorf("some error"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errors.Is(tt.err, tt.target)
			if got != tt.want {
				t.Errorf("errors.Is(%v, %v) = %v, want %v", tt.err, tt.target, got, tt.want)
			}
		})
	}
}

// TestErrorsUnwrap tests that errors.Unwrap() works correctly for wrapped errors
func TestErrorsUnwrap(t *testing.T) {
	t.Run("unwrap returns inner error", func(t *testing.T) {
		inner := New(CONTENT_HASH_MISMATCH, "hash mismatch")
		wrapped := Wrap(inner, "while processing node 1.2")

		unwrapped := errors.Unwrap(wrapped)
		if unwrapped == nil {
			t.Fatal("Unwrap() returned nil, want inner error")
		}

		// Verify it's the same error
		if !errors.Is(unwrapped, inner) {
			t.Errorf("Unwrap() = %v, want %v", unwrapped, inner)
		}
	})

	t.Run("unwrap non-wrapped error returns nil", func(t *testing.T) {
		err := New(INVALID_TYPE, "not wrapped")
		unwrapped := errors.Unwrap(err)
		if unwrapped != nil {
			t.Errorf("Unwrap() = %v, want nil", unwrapped)
		}
	})

	t.Run("multiple wraps unwrap correctly", func(t *testing.T) {
		inner := New(LEDGER_INCONSISTENT, "sequence gap")
		wrap1 := Wrap(inner, "first wrap")
		wrap2 := Wrap(wrap1, "second wrap")

		// First unwrap
		first := errors.Unwrap(wrap2)
		if first == nil {
			t.Fatal("first Unwrap() returned nil")
		}

		// Second unwrap should get to inner
		second := errors.Unwrap(first)
		if second == nil {
			t.Fatal("second Unwrap() returned nil")
		}

		if !errors.Is(second, inner) {
			t.Errorf("double Unwrap() = %v, want %v", second, inner)
		}
	})
}

// TestCode tests extracting error code from AFError
func TestCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorCode
	}{
		{
			name: "direct AFError",
			err:  New(SCOPE_VIOLATION, "msg"),
			want: SCOPE_VIOLATION,
		},
		{
			name: "wrapped AFError",
			err:  Wrap(New(DEPENDENCY_CYCLE, "msg"), "context"),
			want: DEPENDENCY_CYCLE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Code(tt.err)
			if got != tt.want {
				t.Errorf("Code() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCodeFromNonAFError tests Code() behavior with non-AFError
func TestCodeFromNonAFError(t *testing.T) {
	t.Run("returns zero value for standard error", func(t *testing.T) {
		err := fmt.Errorf("standard error")
		got := Code(err)
		// Should return zero value or a designated "unknown" code
		if got != ErrorCode(0) && got.String() != "" {
			t.Errorf("Code() on non-AFError = %v, want zero value", got)
		}
	})

	t.Run("returns zero value for nil", func(t *testing.T) {
		got := Code(nil)
		if got != ErrorCode(0) {
			t.Errorf("Code(nil) = %v, want zero value", got)
		}
	})
}

// TestNewf tests creating errors with formatted messages
func TestNewf(t *testing.T) {
	tests := []struct {
		name   string
		code   ErrorCode
		format string
		args   []any
		want   string
	}{
		{
			name:   "single argument",
			code:   ALREADY_CLAIMED,
			format: "node %s is claimed",
			args:   []any{"1.2"},
			want:   "1.2",
		},
		{
			name:   "multiple arguments",
			code:   NOT_CLAIM_HOLDER,
			format: "agent %s cannot release node %s owned by %s",
			args:   []any{"agent1", "1.2", "agent2"},
			want:   "agent1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Newf(tt.code, tt.format, tt.args...)
			errStr := err.Error()
			if !strings.Contains(errStr, tt.want) {
				t.Errorf("Newf().Error() = %q, want to contain %q", errStr, tt.want)
			}
		})
	}
}

// TestIsRetriable tests the IsRetriable helper function
func TestIsRetriable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"ALREADY_CLAIMED is retriable", New(ALREADY_CLAIMED, ""), true},
		{"NOT_CLAIM_HOLDER is retriable", New(NOT_CLAIM_HOLDER, ""), true},
		{"VALIDATION_INVARIANT_FAILED is retriable", New(VALIDATION_INVARIANT_FAILED, ""), true},
		{"NODE_BLOCKED is not retriable", New(NODE_BLOCKED, ""), false},
		{"INVALID_TYPE is not retriable", New(INVALID_TYPE, ""), false},
		{"LEDGER_INCONSISTENT is not retriable", New(LEDGER_INCONSISTENT, ""), false},
		{"nil is not retriable", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetriable(tt.err)
			if got != tt.want {
				t.Errorf("IsRetriable() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsBlocked tests the IsBlocked helper function
func TestIsBlocked(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"NODE_BLOCKED is blocked", New(NODE_BLOCKED, ""), true},
		{"ALREADY_CLAIMED is not blocked", New(ALREADY_CLAIMED, ""), false},
		{"INVALID_TYPE is not blocked", New(INVALID_TYPE, ""), false},
		{"nil is not blocked", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBlocked(tt.err)
			if got != tt.want {
				t.Errorf("IsBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsCorruption tests the IsCorruption helper function
func TestIsCorruption(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"CONTENT_HASH_MISMATCH is corruption", New(CONTENT_HASH_MISMATCH, ""), true},
		{"LEDGER_INCONSISTENT is corruption", New(LEDGER_INCONSISTENT, ""), true},
		{"ALREADY_CLAIMED is not corruption", New(ALREADY_CLAIMED, ""), false},
		{"nil is not corruption", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCorruption(tt.err)
			if got != tt.want {
				t.Errorf("IsCorruption() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExitCodeFunc tests the ExitCode helper function
func TestExitCodeFunc(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"AFError returns correct exit code", New(NODE_BLOCKED, ""), 2},
		{"wrapped AFError returns correct exit code", Wrap(New(LEDGER_INCONSISTENT, ""), "ctx"), 4},
		{"nil returns 0", nil, 0},
		{"standard error returns 1", fmt.Errorf("standard"), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCode(tt.err)
			if got != tt.wantCode {
				t.Errorf("ExitCode() = %d, want %d", got, tt.wantCode)
			}
		})
	}
}

