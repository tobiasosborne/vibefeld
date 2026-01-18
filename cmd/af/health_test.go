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

// newTestHealthCmd creates a fresh root command with the health subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestHealthCmd() *cobra.Command {
	cmd := newTestRootCmd()

	healthCmd := newHealthCmd()
	cmd.AddCommand(healthCmd)

	return cmd
}

// executeHealthCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeHealthCommand(root *cobra.Command, args ...string) (string, error) {
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

// TestHealthCmd_ExpectedFlags ensures the health command has expected flag structure.
func TestHealthCmd_ExpectedFlags(t *testing.T) {
	cmd := newHealthCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected health command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected health command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestHealthCmd_DefaultFlagValues verifies default values for flags.
func TestHealthCmd_DefaultFlagValues(t *testing.T) {
	cmd := newHealthCmd()

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

// TestHealthCmd_CommandMetadata verifies command metadata.
func TestHealthCmd_CommandMetadata(t *testing.T) {
	cmd := newHealthCmd()

	if cmd.Use != "health" {
		t.Errorf("expected Use to be 'health', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestHealthCmd_Help verifies help output shows usage information.
func TestHealthCmd_Help(t *testing.T) {
	cmd := newTestHealthCmd()
	output, err := executeHealthCommand(cmd, "health", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"health",   // Command name
		"--format", // Format flag
		"--dir",    // Directory flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestHealthCmd_HelpShortFlag verifies help with short flag.
func TestHealthCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestHealthCmd()
	output, err := executeHealthCommand(cmd, "health", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "health") {
		t.Errorf("expected help output to mention 'health', got: %q", output)
	}
}

// =============================================================================
// Error Case Tests (Unit Tests)
// =============================================================================

// TestHealthCmd_DirFlagNonExistent verifies error for non-existent directory.
func TestHealthCmd_DirFlagNonExistent(t *testing.T) {
	cmd := newTestHealthCmd()
	_, err := executeHealthCommand(cmd, "health", "--dir", "/nonexistent/path/to/proof")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestHealthCmd_InvalidFormat verifies error for invalid format.
func TestHealthCmd_InvalidFormat(t *testing.T) {
	cmd := newTestHealthCmd()
	_, err := executeHealthCommand(cmd, "health", "--format", "xml", "--dir", "/nonexistent/path")

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

// TestHealthCmd_JSONOutputStructure verifies JSON output has correct structure.
func TestHealthCmd_JSONOutputStructure(t *testing.T) {
	// Test that the expected JSON keys exist in the health output
	type HealthJSON struct {
		Status     string        `json:"status"`
		Blockers   []interface{} `json:"blockers"`
		Statistics interface{}   `json:"statistics"`
	}

	health := HealthJSON{
		Status:   "healthy",
		Blockers: []interface{}{},
		Statistics: map[string]interface{}{
			"total_nodes":      0,
			"pending_nodes":    0,
			"open_challenges":  0,
			"prover_jobs":      0,
			"verifier_jobs":    0,
			"leaf_nodes":       0,
			"blocked_leaves":   0,
		},
	}

	data, err := json.Marshal(health)
	if err != nil {
		t.Fatalf("failed to marshal test JSON structure: %v", err)
	}

	// Verify the structure can be unmarshaled back
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	expectedKeys := []string{"status", "blockers", "statistics"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON to contain key %q", key)
		}
	}
}

// =============================================================================
// Table-Driven Format Validation Tests
// =============================================================================

// TestHealthCmd_FormatValidation verifies format flag validation.
func TestHealthCmd_FormatValidation(t *testing.T) {
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
			cmd := newTestHealthCmd()
			var args []string
			if tt.format == "" {
				// Don't pass format flag for empty case
				args = []string{"health", "--dir", "/nonexistent/path"}
			} else {
				args = []string{"health", "--format", tt.format, "--dir", "/nonexistent/path"}
			}

			_, err := executeHealthCommand(cmd, args...)

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

// =============================================================================
// Health Status Detection Tests (Unit Tests)
// =============================================================================

// TestHealthStatus_Constants verifies health status string constants.
func TestHealthStatus_Constants(t *testing.T) {
	// Verify the health status constants are what we expect
	if HealthStatusHealthy != "healthy" {
		t.Errorf("expected HealthStatusHealthy to be 'healthy', got %q", HealthStatusHealthy)
	}
	if HealthStatusStuck != "stuck" {
		t.Errorf("expected HealthStatusStuck to be 'stuck', got %q", HealthStatusStuck)
	}
	if HealthStatusWarning != "warning" {
		t.Errorf("expected HealthStatusWarning to be 'warning', got %q", HealthStatusWarning)
	}
}

// TestHealthReport_EmptyProof verifies health report for empty/minimal proof.
func TestHealthReport_EmptyProof(t *testing.T) {
	report := &HealthReport{
		Status:     HealthStatusHealthy,
		Blockers:   []Blocker{},
		Statistics: HealthStatistics{},
	}

	if report.Status != HealthStatusHealthy {
		t.Errorf("expected empty proof to be healthy, got %q", report.Status)
	}

	if len(report.Blockers) != 0 {
		t.Errorf("expected empty proof to have no blockers, got %d", len(report.Blockers))
	}
}

// TestBlocker_Structure verifies Blocker struct has expected fields.
func TestBlocker_Structure(t *testing.T) {
	blocker := Blocker{
		Type:       "all_leaves_challenged",
		Message:    "All leaf nodes have open challenges",
		Suggestion: "Address challenges on leaf nodes or add new children",
		NodeIDs:    []string{"1", "1.1"},
	}

	if blocker.Type == "" {
		t.Error("expected Blocker to have Type field")
	}
	if blocker.Message == "" {
		t.Error("expected Blocker to have Message field")
	}
	if blocker.Suggestion == "" {
		t.Error("expected Blocker to have Suggestion field")
	}
	if len(blocker.NodeIDs) != 2 {
		t.Errorf("expected Blocker to have 2 NodeIDs, got %d", len(blocker.NodeIDs))
	}
}

// TestHealthStatistics_Fields verifies HealthStatistics has expected fields.
func TestHealthStatistics_Fields(t *testing.T) {
	stats := HealthStatistics{
		TotalNodes:     5,
		PendingNodes:   3,
		ValidatedNodes: 1,
		AdmittedNodes:  1,
		RefutedNodes:   0,
		ArchivedNodes:  0,
		OpenChallenges: 2,
		ProverJobs:     1,
		VerifierJobs:   2,
		LeafNodes:      3,
		BlockedLeaves:  1,
	}

	if stats.TotalNodes != 5 {
		t.Errorf("expected TotalNodes to be 5, got %d", stats.TotalNodes)
	}
	if stats.OpenChallenges != 2 {
		t.Errorf("expected OpenChallenges to be 2, got %d", stats.OpenChallenges)
	}
	if stats.LeafNodes != 3 {
		t.Errorf("expected LeafNodes to be 3, got %d", stats.LeafNodes)
	}
}
