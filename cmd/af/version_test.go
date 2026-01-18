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

// newTestVersionCmd creates a fresh root command with the version subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestVersionCmd() *cobra.Command {
	cmd := newTestRootCmd()

	versionCmd := newVersionCmd()
	cmd.AddCommand(versionCmd)

	return cmd
}

// executeVersionCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeVersionCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// =============================================================================
// Command Registration Tests
// =============================================================================

// TestVersionCmd_CommandMetadata verifies command metadata.
func TestVersionCmd_CommandMetadata(t *testing.T) {
	cmd := newVersionCmd()

	if cmd.Use != "version" {
		t.Errorf("expected Use to be 'version', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestVersionCmd_ExpectedFlags ensures the version command has expected flag structure.
func TestVersionCmd_ExpectedFlags(t *testing.T) {
	cmd := newVersionCmd()

	// Check expected flags exist
	if cmd.Flags().Lookup("json") == nil {
		t.Error("expected version command to have flag --json")
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestVersionCmd_Help verifies help output shows usage information.
func TestVersionCmd_Help(t *testing.T) {
	cmd := newTestVersionCmd()
	output, err := executeVersionCommand(cmd, "version", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"version", // Command name
		"--json",  // JSON flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestVersionCmd_HelpShortFlag verifies help with short flag.
func TestVersionCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestVersionCmd()
	output, err := executeVersionCommand(cmd, "version", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "version") {
		t.Errorf("expected help output to mention 'version', got: %q", output)
	}
}

// =============================================================================
// Default Output Format Tests
// =============================================================================

// TestVersionCmd_DefaultOutput verifies the default text output format.
func TestVersionCmd_DefaultOutput(t *testing.T) {
	cmd := newTestVersionCmd()
	output, err := executeVersionCommand(cmd, "version")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain version header
	if !strings.Contains(output, "af version") {
		t.Errorf("expected output to contain 'af version', got: %q", output)
	}

	// Should contain Commit line
	if !strings.Contains(output, "Commit:") {
		t.Errorf("expected output to contain 'Commit:', got: %q", output)
	}

	// Should contain Built line
	if !strings.Contains(output, "Built:") {
		t.Errorf("expected output to contain 'Built:', got: %q", output)
	}

	// Should contain Go line
	if !strings.Contains(output, "Go:") {
		t.Errorf("expected output to contain 'Go:', got: %q", output)
	}
}

// TestVersionCmd_DefaultOutputFormatStructure verifies the structure of default output.
func TestVersionCmd_DefaultOutputFormatStructure(t *testing.T) {
	cmd := newTestVersionCmd()
	output, err := executeVersionCommand(cmd, "version")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have at least 4 lines (version header + 3 info lines)
	if len(lines) < 4 {
		t.Errorf("expected at least 4 lines of output, got %d: %q", len(lines), output)
	}

	// First line should start with "af version"
	if !strings.HasPrefix(lines[0], "af version") {
		t.Errorf("expected first line to start with 'af version', got: %q", lines[0])
	}

	// Check indentation of subsequent lines (should have leading spaces)
	for i, line := range lines[1:] {
		if line != "" && !strings.HasPrefix(line, "  ") {
			t.Errorf("expected line %d to be indented with 2 spaces, got: %q", i+2, line)
		}
	}
}

// =============================================================================
// JSON Output Format Tests
// =============================================================================

// TestVersionCmd_JSONOutput verifies the JSON output format.
func TestVersionCmd_JSONOutput(t *testing.T) {
	cmd := newTestVersionCmd()
	output, err := executeVersionCommand(cmd, "version", "--json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v, output: %q", err, output)
	}

	// Check required fields exist
	expectedKeys := []string{"version", "commit", "build_date", "go_version"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON output to contain key %q", key)
		}
	}
}

// TestVersionCmd_JSONOutputStructure verifies the JSON output has the correct structure.
func TestVersionCmd_JSONOutputStructure(t *testing.T) {
	// Define expected JSON structure
	type VersionJSON struct {
		Version   string `json:"version"`
		Commit    string `json:"commit"`
		BuildDate string `json:"build_date"`
		GoVersion string `json:"go_version"`
	}

	cmd := newTestVersionCmd()
	output, err := executeVersionCommand(cmd, "version", "--json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var versionInfo VersionJSON
	if err := json.Unmarshal([]byte(output), &versionInfo); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// All fields should have values (even if defaults)
	if versionInfo.Version == "" {
		t.Error("expected version field to have a value")
	}
	if versionInfo.Commit == "" {
		t.Error("expected commit field to have a value")
	}
	if versionInfo.BuildDate == "" {
		t.Error("expected build_date field to have a value")
	}
	if versionInfo.GoVersion == "" {
		t.Error("expected go_version field to have a value")
	}
}

// =============================================================================
// Version Variables Tests
// =============================================================================

// TestVersionVariables_Defaults verifies the default values of version variables.
func TestVersionVariables_Defaults(t *testing.T) {
	// When not set via ldflags, these should have default values
	// Note: VersionInfo is "dev", GitCommit is "unknown", BuildDate is "unknown"

	if VersionInfo == "" {
		t.Error("expected VersionInfo to have a default value")
	}

	if GitCommit == "" {
		t.Error("expected GitCommit to have a default value")
	}

	if BuildDate == "" {
		t.Error("expected BuildDate to have a default value")
	}
}

// TestVersionVariables_GoVersionRuntime verifies GoVersion is populated at runtime.
func TestVersionVariables_GoVersionRuntime(t *testing.T) {
	// GoVersion should be populated from runtime.Version()
	cmd := newTestVersionCmd()
	output, err := executeVersionCommand(cmd, "version", "--json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// GoVersion should start with "go"
	if !strings.HasPrefix(result["go_version"], "go") {
		t.Errorf("expected go_version to start with 'go', got: %q", result["go_version"])
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestVersionCmd_OutputFormats tests various output format scenarios.
func TestVersionCmd_OutputFormats(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantErr   bool
		checkJSON bool
		contains  []string
	}{
		{
			name:      "default text output",
			args:      []string{"version"},
			wantErr:   false,
			checkJSON: false,
			contains:  []string{"af version", "Commit:", "Built:", "Go:"},
		},
		{
			name:      "json output",
			args:      []string{"version", "--json"},
			wantErr:   false,
			checkJSON: true,
			contains:  []string{"version", "commit", "build_date", "go_version"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestVersionCmd()
			output, err := executeVersionCommand(cmd, tt.args...)

			if (err != nil) != tt.wantErr {
				t.Errorf("expected error=%v, got error=%v", tt.wantErr, err)
				return
			}

			if tt.checkJSON {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("expected valid JSON, got error: %v", err)
					return
				}
			}

			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain %q, got: %q", expected, output)
				}
			}
		})
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestVersionCmd_NoArgs verifies command works with no arguments.
func TestVersionCmd_NoArgs(t *testing.T) {
	cmd := newTestVersionCmd()
	output, err := executeVersionCommand(cmd, "version")

	if err != nil {
		t.Errorf("expected no error with no arguments, got: %v", err)
	}

	if output == "" {
		t.Error("expected non-empty output")
	}
}

// TestVersionCmd_UnknownFlag verifies unknown flags are rejected.
func TestVersionCmd_UnknownFlag(t *testing.T) {
	cmd := newTestVersionCmd()
	_, err := executeVersionCommand(cmd, "version", "--unknown-flag")

	if err == nil {
		t.Error("expected error for unknown flag, got nil")
	}
}

// TestVersionCmd_ExtraArgs verifies extra arguments don't cause errors.
func TestVersionCmd_ExtraArgs(t *testing.T) {
	cmd := newTestVersionCmd()
	output, err := executeVersionCommand(cmd, "version", "extra", "args")

	// The command should either succeed (ignoring extra args) or fail gracefully
	// Based on Cobra default behavior, it should succeed and ignore extra args
	if err != nil {
		t.Logf("command returned error with extra args: %v (acceptable)", err)
		return
	}

	if output == "" {
		t.Error("expected non-empty output")
	}
}
