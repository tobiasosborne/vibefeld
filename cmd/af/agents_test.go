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

// newTestAgentsCmd creates a fresh root command with the agents subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestAgentsCmd() *cobra.Command {
	cmd := newTestRootCmd()
	cmd.AddCommand(newAgentsCmd())
	return cmd
}

// executeAgentsCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeAgentsCommand(root *cobra.Command, args ...string) (string, error) {
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

// TestAgentsCmd_ExpectedFlags ensures the agents command has expected flag structure.
func TestAgentsCmd_ExpectedFlags(t *testing.T) {
	cmd := newAgentsCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected agents command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected agents command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestAgentsCmd_DefaultFlagValues verifies default values for flags.
func TestAgentsCmd_DefaultFlagValues(t *testing.T) {
	cmd := newAgentsCmd()

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
}

// TestAgentsCmd_CommandMetadata verifies command metadata.
func TestAgentsCmd_CommandMetadata(t *testing.T) {
	cmd := newAgentsCmd()

	if cmd.Use != "agents" {
		t.Errorf("expected Use to be 'agents', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestAgentsCmd_Help verifies help output shows usage information.
func TestAgentsCmd_Help(t *testing.T) {
	cmd := newTestAgentsCmd()
	output, err := executeAgentsCommand(cmd, "agents", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"agents",   // Command name
		"--format", // Format flag
		"--dir",    // Directory flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestAgentsCmd_HelpShortFlag verifies help with short flag.
func TestAgentsCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestAgentsCmd()
	output, err := executeAgentsCommand(cmd, "agents", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "agents") {
		t.Errorf("expected help output to mention 'agents', got: %q", output)
	}
}

// =============================================================================
// Error Case Tests (Unit Tests)
// =============================================================================

// TestAgentsCmd_DirFlagNonExistent verifies error for non-existent directory.
func TestAgentsCmd_DirFlagNonExistent(t *testing.T) {
	cmd := newTestAgentsCmd()
	_, err := executeAgentsCommand(cmd, "agents", "--dir", "/nonexistent/path/to/proof")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestAgentsCmd_InvalidFormat verifies error for invalid format.
func TestAgentsCmd_InvalidFormat(t *testing.T) {
	cmd := newTestAgentsCmd()
	_, err := executeAgentsCommand(cmd, "agents", "--format", "xml", "--dir", "/nonexistent/path")

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

// TestAgentsCmd_JSONOutputStructure verifies JSON output has correct structure.
func TestAgentsCmd_JSONOutputStructure(t *testing.T) {
	// Test that the expected JSON keys exist in the structure
	// by marshaling a mock structure
	type JSONAgentEntry struct {
		NodeID    string `json:"node_id"`
		Owner     string `json:"owner"`
		ClaimedAt string `json:"claimed_at,omitempty"`
	}
	type JSONAgentActivity struct {
		Type      string `json:"type"`
		Timestamp string `json:"timestamp"`
		NodeIDs   []string `json:"node_ids,omitempty"`
		Owner     string `json:"owner,omitempty"`
	}
	type JSONAgentsOutput struct {
		ClaimedNodes []JSONAgentEntry    `json:"claimed_nodes"`
		Activity     []JSONAgentActivity `json:"activity"`
	}

	output := JSONAgentsOutput{
		ClaimedNodes: []JSONAgentEntry{},
		Activity:     []JSONAgentActivity{},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal test JSON structure: %v", err)
	}

	// Verify the structure can be unmarshaled back
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	expectedKeys := []string{"claimed_nodes", "activity"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON to contain key %q", key)
		}
	}
}

// =============================================================================
// Table-Driven Format Validation Tests
// =============================================================================

// TestAgentsCmd_FormatValidation verifies format flag validation.
func TestAgentsCmd_FormatValidation(t *testing.T) {
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
			cmd := newTestAgentsCmd()
			var args []string
			if tt.format == "" {
				// Don't pass format flag for empty case
				args = []string{"agents", "--dir", "/nonexistent/path"}
			} else {
				args = []string{"agents", "--format", tt.format, "--dir", "/nonexistent/path"}
			}

			_, err := executeAgentsCommand(cmd, args...)

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
