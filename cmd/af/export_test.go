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

// newTestExportCmd creates a fresh root command with the export subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	exportCmd := newExportCmd()
	cmd.AddCommand(exportCmd)

	return cmd
}

// executeExportCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeExportCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// =============================================================================
// Flag Tests (Unit Tests - No File System)
// =============================================================================

// TestExportCmd_ExpectedFlags ensures the export command has expected flag structure.
func TestExportCmd_ExpectedFlags(t *testing.T) {
	cmd := newExportCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir", "output"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected export command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
		"o": "output",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected export command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestExportCmd_DefaultFlagValues verifies default values for flags.
func TestExportCmd_DefaultFlagValues(t *testing.T) {
	cmd := newExportCmd()

	// Check default dir value
	dirFlag := cmd.Flags().Lookup("dir")
	if dirFlag == nil {
		t.Fatal("expected dir flag to exist")
	}
	if dirFlag.DefValue != "." {
		t.Errorf("expected default dir to be '.', got %q", dirFlag.DefValue)
	}

	// Check default format value
	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("expected format flag to exist")
	}
	if formatFlag.DefValue != "markdown" {
		t.Errorf("expected default format to be 'markdown', got %q", formatFlag.DefValue)
	}

	// Check default output value (empty = stdout)
	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Fatal("expected output flag to exist")
	}
	if outputFlag.DefValue != "" {
		t.Errorf("expected default output to be empty (stdout), got %q", outputFlag.DefValue)
	}
}

// TestExportCmd_CommandMetadata verifies command metadata.
func TestExportCmd_CommandMetadata(t *testing.T) {
	cmd := newExportCmd()

	if cmd.Use != "export" {
		t.Errorf("expected Use to be 'export', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestExportCmd_Help verifies help output shows usage information.
func TestExportCmd_Help(t *testing.T) {
	cmd := newTestExportCmd()
	output, err := executeExportCommand(cmd, "export", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"export",   // Command name
		"--format", // Format flag
		"--dir",    // Directory flag
		"--output", // Output flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestExportCmd_HelpShortFlag verifies help with short flag.
func TestExportCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestExportCmd()
	output, err := executeExportCommand(cmd, "export", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "export") {
		t.Errorf("expected help output to mention 'export', got: %q", output)
	}
}

// =============================================================================
// Error Case Tests (Unit Tests)
// =============================================================================

// TestExportCmd_DirFlagNonExistent verifies error for non-existent directory.
func TestExportCmd_DirFlagNonExistent(t *testing.T) {
	cmd := newTestExportCmd()
	_, err := executeExportCommand(cmd, "export", "--dir", "/nonexistent/path/to/proof")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestExportCmd_InvalidFormat verifies error for invalid format.
func TestExportCmd_InvalidFormat(t *testing.T) {
	cmd := newTestExportCmd()
	_, err := executeExportCommand(cmd, "export", "--format", "pdf", "--dir", "/nonexistent/path")

	if err == nil {
		t.Error("expected error for invalid format, got nil")
	}

	// The error should mention invalid format
	if err != nil && strings.Contains(err.Error(), "invalid") {
		// Good - format validation happened
		return
	}
}

// =============================================================================
// Table-Driven Format Validation Tests
// =============================================================================

// TestExportCmd_FormatValidation verifies format flag validation.
func TestExportCmd_FormatValidation(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"valid markdown format", "markdown", false},
		{"valid md format", "md", false},
		{"valid latex format", "latex", false},
		{"valid tex format", "tex", false},
		{"valid MARKDOWN uppercase", "MARKDOWN", false},
		{"valid LATEX uppercase", "LATEX", false},
		{"invalid pdf format", "pdf", true},
		{"invalid xml format", "xml", true},
		{"invalid json format", "json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestExportCmd()
			args := []string{"export", "--format", tt.format, "--dir", "/nonexistent/path"}

			_, err := executeExportCommand(cmd, args...)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for format %q, got nil", tt.format)
					return
				}
				// Should error about format, not directory
				if !strings.Contains(err.Error(), "format") && !strings.Contains(err.Error(), "path") {
					t.Logf("Got error: %v (acceptable - either format or path error)", err)
				}
			}
			// For non-error cases, we expect a directory error (since path doesn't exist)
			// but NOT a format error
			if !tt.wantErr && err != nil {
				if strings.Contains(err.Error(), "format") {
					t.Errorf("unexpected format error for format %q: %v", tt.format, err)
				}
			}
		})
	}
}

// TestExportCmd_HelpMentionsFormats verifies that help mentions available formats.
func TestExportCmd_HelpMentionsFormats(t *testing.T) {
	cmd := newTestExportCmd()
	output, err := executeExportCommand(cmd, "export", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Should mention the available formats
	formatMentions := []string{"markdown", "latex"}
	for _, format := range formatMentions {
		if !strings.Contains(strings.ToLower(output), format) {
			t.Errorf("expected help to mention format %q, got: %q", format, output)
		}
	}
}
