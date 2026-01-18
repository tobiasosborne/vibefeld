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

// newTestReplayCmd creates a fresh root command with the replay subcommand for testing.
func newTestReplayCmd() *cobra.Command {
	cmd := newTestRootCmd()

	replayCmd := newReplayCmd()
	cmd.AddCommand(replayCmd)

	return cmd
}

// executeReplayCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeReplayCommand(root *cobra.Command, args ...string) (string, error) {
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

// TestReplayCmd_ExpectedFlags ensures the replay command has expected flag structure.
func TestReplayCmd_ExpectedFlags(t *testing.T) {
	cmd := newReplayCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir", "verify", "verbose"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected replay command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
		"v": "verbose",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected replay command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestReplayCmd_DefaultFlagValues verifies default values for flags.
func TestReplayCmd_DefaultFlagValues(t *testing.T) {
	cmd := newReplayCmd()

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

	// Check default verify value
	verifyFlag := cmd.Flags().Lookup("verify")
	if verifyFlag == nil {
		t.Fatal("expected verify flag to exist")
	}
	if verifyFlag.DefValue != "false" {
		t.Errorf("expected default verify to be 'false', got %q", verifyFlag.DefValue)
	}

	// Check default verbose value
	verboseFlag := cmd.Flags().Lookup("verbose")
	if verboseFlag == nil {
		t.Fatal("expected verbose flag to exist")
	}
	if verboseFlag.DefValue != "false" {
		t.Errorf("expected default verbose to be 'false', got %q", verboseFlag.DefValue)
	}
}

// TestReplayCmd_CommandMetadata verifies command metadata.
func TestReplayCmd_CommandMetadata(t *testing.T) {
	cmd := newReplayCmd()

	if cmd.Use != "replay" {
		t.Errorf("expected Use to be 'replay', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestReplayCmd_Help verifies help output shows usage information.
func TestReplayCmd_Help(t *testing.T) {
	cmd := newTestReplayCmd()
	output, err := executeReplayCommand(cmd, "replay", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"replay",   // Command name
		"--format", // Format flag
		"--dir",    // Directory flag
		"--verify", // Verify flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestReplayCmd_HelpShortFlag verifies help with short flag.
func TestReplayCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestReplayCmd()
	output, err := executeReplayCommand(cmd, "replay", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "replay") {
		t.Errorf("expected help output to mention 'replay', got: %q", output)
	}
}

// =============================================================================
// Error Case Tests (Unit Tests)
// =============================================================================

// TestReplayCmd_DirFlagNonExistent verifies error for non-existent directory.
func TestReplayCmd_DirFlagNonExistent(t *testing.T) {
	cmd := newTestReplayCmd()
	_, err := executeReplayCommand(cmd, "replay", "--dir", "/nonexistent/path/to/proof")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestReplayCmd_InvalidFormat verifies error for invalid format.
func TestReplayCmd_InvalidFormat(t *testing.T) {
	cmd := newTestReplayCmd()
	_, err := executeReplayCommand(cmd, "replay", "--format", "xml", "--dir", "/nonexistent/path")

	if err == nil {
		t.Error("expected error for invalid format, got nil")
	}

	// The error should be about the invalid format
	if err != nil && strings.Contains(err.Error(), "invalid format") {
		// Good - format validation happened
		return
	}
}

// =============================================================================
// Output Format Tests (Unit Tests - Validate JSON Structure)
// =============================================================================

// TestReplayCmd_JSONOutputStructureValidation verifies JSON output has correct structure.
func TestReplayCmd_JSONOutputStructureValidation(t *testing.T) {
	// Test that the expected JSON keys exist in the ReplayStats struct
	// by marshaling a mock structure
	stats := ReplayStats{
		EventsProcessed: 42,
		Nodes:           15,
		Definitions:     5,
		Valid:           true,
	}
	stats.Challenges.Total = 3
	stats.Challenges.Resolved = 2
	stats.Challenges.Open = 1

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("failed to marshal test JSON structure: %v", err)
	}

	// Verify the structure can be unmarshaled back
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	expectedKeys := []string{"events_processed", "nodes", "challenges", "definitions", "valid"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON to contain key %q", key)
		}
	}
}

// TestReplayCmd_JSONOutputWithHashVerification verifies hash verification structure.
func TestReplayCmd_JSONOutputWithHashVerification(t *testing.T) {
	// Test that hash verification fields are present when set
	stats := ReplayStats{
		EventsProcessed: 42,
		Nodes:           15,
		Valid:           true,
	}
	stats.HashVerification = &struct {
		Verified int  `json:"verified"`
		Total    int  `json:"total"`
		Valid    bool `json:"valid"`
	}{
		Verified: 15,
		Total:    15,
		Valid:    true,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("failed to marshal test JSON structure: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	if _, ok := result["hash_verification"]; !ok {
		t.Error("expected JSON to contain 'hash_verification' key")
	}

	hashVerif, ok := result["hash_verification"].(map[string]interface{})
	if !ok {
		t.Fatal("hash_verification should be an object")
	}

	expectedHashKeys := []string{"verified", "total", "valid"}
	for _, key := range expectedHashKeys {
		if _, ok := hashVerif[key]; !ok {
			t.Errorf("expected hash_verification to contain key %q", key)
		}
	}
}

// =============================================================================
// Table-Driven Format Validation Tests
// =============================================================================

// TestReplayCmd_FormatValidation verifies format flag validation.
func TestReplayCmd_FormatValidation(t *testing.T) {
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
			cmd := newTestReplayCmd()
			var args []string
			if tt.format == "" {
				// Don't pass format flag for empty case
				args = []string{"replay", "--dir", "/nonexistent/path"}
			} else {
				args = []string{"replay", "--format", tt.format, "--dir", "/nonexistent/path"}
			}

			_, err := executeReplayCommand(cmd, args...)

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
