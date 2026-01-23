package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestConfirmAction(t *testing.T) {
	tests := []struct {
		name        string
		skipConfirm bool
		input       string
		stdin       *os.File // nil means use a pipe (non-terminal)
		wantConfirm bool
		wantErr     bool
		errContains string
	}{
		{
			name:        "skip confirmation with flag",
			skipConfirm: true,
			input:       "",
			wantConfirm: true,
			wantErr:     false,
		},
		{
			name:        "user confirms with y",
			skipConfirm: false,
			input:       "y\n",
			stdin:       nil, // Will use mock terminal
			wantConfirm: true,
			wantErr:     false,
		},
		{
			name:        "user confirms with yes",
			skipConfirm: false,
			input:       "yes\n",
			stdin:       nil,
			wantConfirm: true,
			wantErr:     false,
		},
		{
			name:        "user confirms with YES uppercase",
			skipConfirm: false,
			input:       "YES\n",
			stdin:       nil,
			wantConfirm: true,
			wantErr:     false,
		},
		{
			name:        "user declines with n",
			skipConfirm: false,
			input:       "n\n",
			stdin:       nil,
			wantConfirm: false,
			wantErr:     false,
		},
		{
			name:        "user declines with empty",
			skipConfirm: false,
			input:       "\n",
			stdin:       nil,
			wantConfirm: false,
			wantErr:     false,
		},
		{
			name:        "user declines with no",
			skipConfirm: false,
			input:       "no\n",
			stdin:       nil,
			wantConfirm: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			reader := strings.NewReader(tt.input)

			// Use the testable version that takes an io.Reader
			confirmed, err := ConfirmActionWithReader(&out, reader, "test action", tt.skipConfirm)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if confirmed != tt.wantConfirm {
				t.Errorf("confirmed = %v, want %v", confirmed, tt.wantConfirm)
			}
		})
	}
}

func TestConfirmActionPromptFormat(t *testing.T) {
	var out bytes.Buffer
	reader := strings.NewReader("y\n")

	_, err := ConfirmActionWithReader(&out, reader, "refute node 1.2.3", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Are you sure you want to refute node 1.2.3? [y/N]: "
	if out.String() != expected {
		t.Errorf("prompt = %q, want %q", out.String(), expected)
	}
}

func TestConfirmActionEOFError(t *testing.T) {
	var out bytes.Buffer
	reader := strings.NewReader("") // EOF immediately

	_, err := ConfirmActionWithReader(&out, reader, "test action", false)
	if err == nil {
		t.Errorf("expected error for EOF, got nil")
	}
	if !errors.Is(err, ErrNotInteractive) {
		t.Errorf("expected ErrNotInteractive, got %v", err)
	}
}

func TestRequireInteractiveStdin(t *testing.T) {
	// Test with a pipe (non-terminal)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	err = RequireInteractiveStdin(r, "refute")
	if err == nil {
		t.Errorf("expected error for non-terminal stdin")
	}
	if !strings.Contains(err.Error(), "--yes flag") {
		t.Errorf("error should mention --yes flag: %v", err)
	}
	if !strings.Contains(err.Error(), "refute") {
		t.Errorf("error should mention command: %v", err)
	}
}
