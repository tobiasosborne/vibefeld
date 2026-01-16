//go:build !integration

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestShellCmd creates a fresh root command with the shell subcommand for testing.
func newTestShellCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	shellCmd := newShellCmd(cmd)
	cmd.AddCommand(shellCmd)

	return cmd
}

// executeShellCommand executes a cobra command with the given arguments.
func executeShellCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// =============================================================================
// Command Structure Tests
// =============================================================================

// TestShellCmd_ExpectedFlags ensures the shell command has expected flags.
func TestShellCmd_ExpectedFlags(t *testing.T) {
	rootCmd := &cobra.Command{Use: "af"}
	cmd := newShellCmd(rootCmd)

	// Check expected flags exist
	if cmd.Flags().Lookup("prompt") == nil {
		t.Error("expected shell command to have --prompt flag")
	}

	// Check short flags
	if cmd.Flags().ShorthandLookup("p") == nil {
		t.Error("expected shell command to have -p short flag for --prompt")
	}
}

// TestShellCmd_DefaultFlagValues verifies default values for flags.
func TestShellCmd_DefaultFlagValues(t *testing.T) {
	rootCmd := &cobra.Command{Use: "af"}
	cmd := newShellCmd(rootCmd)

	promptFlag := cmd.Flags().Lookup("prompt")
	if promptFlag == nil {
		t.Fatal("expected prompt flag to exist")
	}
	if promptFlag.DefValue != "af> " {
		t.Errorf("expected default prompt to be 'af> ', got %q", promptFlag.DefValue)
	}
}

// TestShellCmd_CommandMetadata verifies command metadata.
func TestShellCmd_CommandMetadata(t *testing.T) {
	rootCmd := &cobra.Command{Use: "af"}
	cmd := newShellCmd(rootCmd)

	if cmd.Use != "shell" {
		t.Errorf("expected Use to be 'shell', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if cmd.Long == "" {
		t.Error("expected Long description to be set")
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestShellCmd_Help verifies help output shows usage information.
func TestShellCmd_Help(t *testing.T) {
	cmd := newTestShellCmd()
	output, err := executeShellCommand(cmd, "shell", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	expectations := []string{
		"shell",
		"--prompt",
		"interactive",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), exp) {
			t.Errorf("expected help output to contain %q, got: %s", exp, output)
		}
	}
}

// TestShellCmd_HelpShortFlag verifies help with short flag.
func TestShellCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestShellCmd()
	output, err := executeShellCommand(cmd, "shell", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "shell") {
		t.Errorf("expected help output to mention 'shell', got: %s", output)
	}
}

// =============================================================================
// Shell Execution Tests (with mocked I/O)
// =============================================================================

// TestShellCmd_RunWithInput tests shell execution with simulated input.
func TestShellCmd_RunWithInput(t *testing.T) {
	// This test verifies the shell command can be created and configured correctly.
	// Actual interactive testing is done in internal/shell tests.
	rootCmd := &cobra.Command{Use: "af"}
	cmd := newShellCmd(rootCmd)

	// Verify command is properly configured
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

// TestShellCmd_PromptFlag verifies custom prompt flag is accepted.
func TestShellCmd_PromptFlag(t *testing.T) {
	rootCmd := &cobra.Command{Use: "af"}
	cmd := newShellCmd(rootCmd)

	// Set flag value
	err := cmd.Flags().Set("prompt", "custom> ")
	if err != nil {
		t.Fatalf("failed to set prompt flag: %v", err)
	}

	val, _ := cmd.Flags().GetString("prompt")
	if val != "custom> " {
		t.Errorf("expected prompt to be 'custom> ', got %q", val)
	}
}

// =============================================================================
// Command Integration Tests
// =============================================================================

// TestShellCmd_AddedToRoot verifies shell command is properly added.
func TestShellCmd_AddedToRoot(t *testing.T) {
	cmd := newTestShellCmd()

	// Find shell subcommand
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "shell" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected 'shell' subcommand to be added to root")
	}
}

// TestShellCmd_Aliases verifies command aliases.
func TestShellCmd_Aliases(t *testing.T) {
	rootCmd := &cobra.Command{Use: "af"}
	cmd := newShellCmd(rootCmd)

	// Check for common aliases
	hasREPL := false
	for _, alias := range cmd.Aliases {
		if alias == "repl" {
			hasREPL = true
		}
	}

	if !hasREPL {
		t.Error("expected shell command to have 'repl' alias")
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestShellCmd_EmptyPrompt verifies empty prompt is handled.
func TestShellCmd_EmptyPrompt(t *testing.T) {
	rootCmd := &cobra.Command{Use: "af"}
	cmd := newShellCmd(rootCmd)

	// Empty prompt should be allowed (for scripting)
	err := cmd.Flags().Set("prompt", "")
	if err != nil {
		t.Fatalf("should allow empty prompt: %v", err)
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestShellCmd_FlagValidation verifies flag parsing.
func TestShellCmd_FlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no flags",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "custom prompt long flag",
			args:    []string{"--prompt", "test> "},
			wantErr: false,
		},
		{
			name:    "custom prompt short flag",
			args:    []string{"-p", "test> "},
			wantErr: false,
		},
		{
			name:    "unknown flag",
			args:    []string{"--unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := &cobra.Command{Use: "af"}
			cmd := newShellCmd(rootCmd)

			// Parse flags only (don't execute)
			cmd.SetArgs(tt.args)
			err := cmd.ParseFlags(tt.args)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
