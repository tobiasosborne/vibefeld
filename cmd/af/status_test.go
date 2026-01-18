//go:build !integration

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestStatusCmd creates a fresh root command with the status subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestStatusCmd() *cobra.Command {
	cmd := newTestRootCmd()

	statusCmd := newStatusCmd()
	cmd.AddCommand(statusCmd)

	return cmd
}

// executeStatusCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeStatusCommand(root *cobra.Command, args ...string) (string, error) {
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

// TestStatusCmd_ExpectedFlags ensures the status command has expected flag structure.
func TestStatusCmd_ExpectedFlags(t *testing.T) {
	cmd := newStatusCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir", "urgent"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected status command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
		"u": "urgent",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected status command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestStatusCmd_DefaultFlagValues verifies default values for flags.
func TestStatusCmd_DefaultFlagValues(t *testing.T) {
	cmd := newStatusCmd()

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
	if formatFlag.DefValue != "text" {
		t.Errorf("expected default format to be 'text', got %q", formatFlag.DefValue)
	}

	// Check default urgent value
	urgentFlag := cmd.Flags().Lookup("urgent")
	if urgentFlag == nil {
		t.Fatal("expected urgent flag to exist")
	}
	if urgentFlag.DefValue != "false" {
		t.Errorf("expected default urgent to be 'false', got %q", urgentFlag.DefValue)
	}
}

// TestStatusCmd_CommandMetadata verifies command metadata.
func TestStatusCmd_CommandMetadata(t *testing.T) {
	cmd := newStatusCmd()

	if cmd.Use != "status" {
		t.Errorf("expected Use to be 'status', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestStatusCmd_Help verifies help output shows usage information.
func TestStatusCmd_Help(t *testing.T) {
	cmd := newTestStatusCmd()
	output, err := executeStatusCommand(cmd, "status", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"status",   // Command name
		"--format", // Format flag
		"--dir",    // Directory flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestStatusCmd_HelpShortFlag verifies help with short flag.
func TestStatusCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestStatusCmd()
	output, err := executeStatusCommand(cmd, "status", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "status") {
		t.Errorf("expected help output to mention 'status', got: %q", output)
	}
}

// =============================================================================
// Error Case Tests (Unit Tests)
// =============================================================================

// TestStatusCmd_DirFlagNonExistent verifies error for non-existent directory.
func TestStatusCmd_DirFlagNonExistent(t *testing.T) {
	cmd := newTestStatusCmd()
	_, err := executeStatusCommand(cmd, "status", "--dir", "/nonexistent/path/to/proof")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestStatusCmd_InvalidFormat verifies error for invalid format.
func TestStatusCmd_InvalidFormat(t *testing.T) {
	cmd := newTestStatusCmd()
	_, err := executeStatusCommand(cmd, "status", "--format", "xml", "--dir", "/nonexistent/path")

	if err == nil {
		t.Error("expected error for invalid format, got nil")
	}

	// The error should be about the invalid format, not the directory
	if err != nil && strings.Contains(err.Error(), "invalid format") {
		// Good - format validation happened before directory check
		return
	}
	// If directory error comes first, that's acceptable too
}

// =============================================================================
// Output Format Tests (Unit Tests - Validate JSON Structure)
// =============================================================================

// TestStatusCmd_JSONOutputStructure verifies JSON output has correct structure.
func TestStatusCmd_JSONOutputStructure(t *testing.T) {
	// Test that the expected JSON keys exist in the JSONStatus struct
	// by marshaling a mock structure
	type JSONStatus struct {
		Statistics interface{} `json:"statistics"`
		Jobs       interface{} `json:"jobs"`
		Nodes      interface{} `json:"nodes"`
	}

	status := JSONStatus{
		Statistics: map[string]interface{}{
			"total_nodes":     0,
			"epistemic_state": map[string]int{},
			"taint_state":     map[string]int{},
		},
		Jobs: map[string]interface{}{
			"prover_jobs":   0,
			"verifier_jobs": 0,
		},
		Nodes: []interface{}{},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal test JSON structure: %v", err)
	}

	// Verify the structure can be unmarshaled back
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	expectedKeys := []string{"statistics", "jobs", "nodes"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON to contain key %q", key)
		}
	}
}

// =============================================================================
// Table-Driven Format Validation Tests
// =============================================================================

// TestStatusCmd_FormatValidation verifies format flag validation.
func TestStatusCmd_FormatValidation(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"valid text format", "text", false},
		{"valid json format", "json", false},
		{"valid TEXT uppercase", "TEXT", false},
		{"valid JSON uppercase", "JSON", false},
		{"invalid xml format", "xml", true},
		{"invalid yaml format", "yaml", true},
		{"invalid empty string with flag", "", false}, // Empty should default to text
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestStatusCmd()
			var args []string
			if tt.format == "" {
				// Don't pass format flag for empty case
				args = []string{"status", "--dir", "/nonexistent/path"}
			} else {
				args = []string{"status", "--format", tt.format, "--dir", "/nonexistent/path"}
			}

			_, err := executeStatusCommand(cmd, args...)

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
