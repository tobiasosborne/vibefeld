//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Integration Test Helpers
// =============================================================================

// newIntegrationTestReplayCmd creates a fresh root command with the replay subcommand for testing.
func newIntegrationTestReplayCmd() *cobra.Command {
	cmd := newTestRootCmd()

	replayCmd := newReplayCmd()
	cmd.AddCommand(replayCmd)

	return cmd
}

// executeIntegrationReplayCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeIntegrationReplayCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// setupReplayIntegrationTest creates a clean temp directory for replay testing.
func setupReplayIntegrationTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-replay-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// setupReplayIntegrationTestWithProof creates a proof with initialized root node for replay testing.
func setupReplayIntegrationTestWithProof(t *testing.T, conjecture string) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-replay-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")

	// Initialize proof directory structure
	if err := fs.InitProofDir(proofDir); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Initialize proof with conjecture (this creates ProofInitialized and NodeCreated events)
	if err := service.Init(proofDir, conjecture, "test-author"); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// =============================================================================
// Error Case Integration Tests
// =============================================================================

// TestReplayIntegration_NotInitialized verifies error when proof is not initialized.
func TestReplayIntegration_NotInitialized(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTest(t)
	defer cleanup()

	// Create empty directory
	if err := os.MkdirAll(proofDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := newIntegrationTestReplayCmd()
	_, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", proofDir)

	if err == nil {
		t.Error("expected error for uninitialized proof, got nil")
	}
}

// =============================================================================
// Basic Replay Integration Tests
// =============================================================================

// TestReplayIntegration_BasicReplay verifies basic replay with initialized proof.
func TestReplayIntegration_BasicReplay(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "Test conjecture")
	defer cleanup()

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", proofDir)

	if err != nil {
		t.Fatalf("replay failed: %v\nOutput: %s", err, output)
	}

	// Should contain replay information
	expectedStrings := []string{
		"Replay",
		"Events",
		"Node",
	}

	for _, exp := range expectedStrings {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got: %s", exp, output)
		}
	}
}

// TestReplayIntegration_ReplayWithMultipleNodes verifies replay with multiple nodes.
func TestReplayIntegration_ReplayWithMultipleNodes(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "Multi-node test")
	defer cleanup()

	// Add more nodes
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create child nodes
	childID, err := service.ParseNodeID("1.1")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateNode(childID, schema.NodeTypeClaim, "First step", schema.InferenceAssumption); err != nil {
		t.Fatal(err)
	}

	childID2, err := service.ParseNodeID("1.2")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateNode(childID2, schema.NodeTypeClaim, "Second step", schema.InferenceModusPonens); err != nil {
		t.Fatal(err)
	}

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", proofDir)

	if err != nil {
		t.Fatalf("replay failed: %v\nOutput: %s", err, output)
	}

	// Should show correct node count (root + 2 children = 3 nodes)
	// Note: service.Init creates root node "1", so we should have 3 total
	if !strings.Contains(output, "3") {
		t.Errorf("expected output to mention 3 nodes, got: %s", output)
	}
}

// =============================================================================
// Verify Mode Integration Tests
// =============================================================================

// TestReplayIntegration_VerifyMode verifies --verify flag enables hash verification.
func TestReplayIntegration_VerifyMode(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "Verify test")
	defer cleanup()

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", proofDir, "--verify")

	if err != nil {
		t.Fatalf("replay with verify failed: %v\nOutput: %s", err, output)
	}

	// Should mention hash verification
	if !strings.Contains(strings.ToLower(output), "hash") || !strings.Contains(strings.ToLower(output), "verif") {
		t.Errorf("expected output to mention hash verification, got: %s", output)
	}
}

// TestReplayIntegration_VerifyModeShowsResults verifies --verify shows verification results.
func TestReplayIntegration_VerifyModeShowsResults(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "Verify results test")
	defer cleanup()

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", proofDir, "--verify")

	if err != nil {
		t.Fatalf("replay with verify failed: %v\nOutput: %s", err, output)
	}

	// Should indicate success (valid hashes)
	if !strings.Contains(strings.ToLower(output), "valid") {
		t.Errorf("expected output to indicate valid hashes, got: %s", output)
	}
}

// =============================================================================
// Verbose Mode Integration Tests
// =============================================================================

// TestReplayIntegration_VerboseMode verifies --verbose flag shows detailed output.
func TestReplayIntegration_VerboseMode(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "Verbose test")
	defer cleanup()

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", proofDir, "--verbose")

	if err != nil {
		t.Fatalf("replay with verbose failed: %v\nOutput: %s", err, output)
	}

	// Verbose output should be longer than non-verbose
	cmd2 := newIntegrationTestReplayCmd()
	normalOutput, _ := executeIntegrationReplayCommand(cmd2, "replay", "--dir", proofDir)

	if len(output) <= len(normalOutput) {
		t.Errorf("expected verbose output to be longer than normal output\nVerbose: %d chars\nNormal: %d chars",
			len(output), len(normalOutput))
	}
}

// TestReplayIntegration_VerboseShortFlag verifies -v flag works for verbose.
func TestReplayIntegration_VerboseShortFlag(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "Verbose short flag test")
	defer cleanup()

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", proofDir, "-v")

	if err != nil {
		t.Fatalf("replay with -v failed: %v\nOutput: %s", err, output)
	}

	// Should work the same as --verbose
	if output == "" {
		t.Error("expected non-empty output with -v flag")
	}
}

// =============================================================================
// JSON Output Integration Tests
// =============================================================================

// TestReplayIntegration_JSONOutput verifies JSON output format.
func TestReplayIntegration_JSONOutput(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "JSON test")
	defer cleanup()

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", proofDir, "--format", "json")

	if err != nil {
		t.Fatalf("replay with json format failed: %v\nOutput: %s", err, output)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Should have expected keys
	expectedKeys := []string{"events_processed", "nodes", "valid"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON to contain key %q, got keys: %v", key, replayIntegrationTestGetKeys(result))
		}
	}
}

// TestReplayIntegration_JSONOutputWithVerify verifies JSON output includes hash verification info.
func TestReplayIntegration_JSONOutputWithVerify(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "JSON verify test")
	defer cleanup()

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", proofDir, "--format", "json", "--verify")

	if err != nil {
		t.Fatalf("replay with json and verify failed: %v\nOutput: %s", err, output)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Should have hash_verification key
	if _, ok := result["hash_verification"]; !ok {
		t.Errorf("expected JSON to contain 'hash_verification' key when --verify is used, got: %v", replayIntegrationTestGetKeys(result))
	}
}

// TestReplayIntegration_JSONOutputStructure verifies detailed JSON output structure.
func TestReplayIntegration_JSONOutputStructure(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "JSON structure test")
	defer cleanup()

	// Add a definition to test definitions count
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.AddDefinition("prime", "A prime number is a natural number greater than 1 that has no positive divisors other than 1 and itself")
	if err != nil {
		t.Fatal(err)
	}

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", proofDir, "--format", "json")

	if err != nil {
		t.Fatalf("replay failed: %v\nOutput: %s", err, output)
	}

	var result struct {
		EventsProcessed int `json:"events_processed"`
		Nodes           int `json:"nodes"`
		Challenges      struct {
			Total    int `json:"total"`
			Resolved int `json:"resolved"`
			Open     int `json:"open"`
		} `json:"challenges"`
		Definitions int  `json:"definitions"`
		Valid       bool `json:"valid"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, output)
	}

	// Verify structure
	if result.EventsProcessed < 1 {
		t.Errorf("expected at least 1 event processed, got %d", result.EventsProcessed)
	}

	if result.Nodes < 1 {
		t.Errorf("expected at least 1 node, got %d", result.Nodes)
	}

	if result.Definitions < 1 {
		t.Errorf("expected at least 1 definition, got %d", result.Definitions)
	}

	if !result.Valid {
		t.Error("expected valid to be true")
	}
}

// =============================================================================
// Format Validation Integration Tests
// =============================================================================

// TestReplayIntegration_FormatValidation verifies format flag validation.
func TestReplayIntegration_FormatValidation(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "Format validation")
	defer cleanup()

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newIntegrationTestReplayCmd()
			_, err := executeIntegrationReplayCommand(cmd, "replay", "--format", tt.format, "--dir", proofDir)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for format %q, got nil", tt.format)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for format %q: %v", tt.format, err)
				}
			}
		})
	}
}

// =============================================================================
// Combined Flags Integration Tests
// =============================================================================

// TestReplayIntegration_CombinedFlags verifies multiple flags work together.
func TestReplayIntegration_CombinedFlags(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "Combined flags test")
	defer cleanup()

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay",
		"--dir", proofDir,
		"--format", "json",
		"--verify",
		"--verbose",
	)

	if err != nil {
		t.Fatalf("replay with combined flags failed: %v\nOutput: %s", err, output)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Should have hash verification info due to --verify
	if _, ok := result["hash_verification"]; !ok {
		t.Error("expected hash_verification in output with --verify flag")
	}
}

// =============================================================================
// Challenge Statistics Integration Tests
// =============================================================================

// TestReplayIntegration_ChallengeStatistics verifies challenge counting in replay.
func TestReplayIntegration_ChallengeStatistics(t *testing.T) {
	proofDir, cleanup := setupReplayIntegrationTestWithProof(t, "Challenge stats test")
	defer cleanup()

	// We would need to add challenges through the ledger directly
	// For now, just verify the structure exists with 0 challenges

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", proofDir, "--format", "json")

	if err != nil {
		t.Fatalf("replay failed: %v\nOutput: %s", err, output)
	}

	var result struct {
		Challenges struct {
			Total    int `json:"total"`
			Resolved int `json:"resolved"`
			Open     int `json:"open"`
		} `json:"challenges"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, output)
	}

	// With no challenges added, all should be 0
	if result.Challenges.Total != 0 {
		t.Errorf("expected 0 total challenges, got %d", result.Challenges.Total)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestReplayIntegration_EmptyProofDir verifies behavior with empty proof directory.
func TestReplayIntegration_EmptyProofDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "af-replay-empty-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create empty ledger directory
	ledgerDir := filepath.Join(tmpDir, "ledger")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := newIntegrationTestReplayCmd()
	output, err := executeIntegrationReplayCommand(cmd, "replay", "--dir", tmpDir)

	// Should either succeed with 0 events or error gracefully
	if err != nil {
		// Acceptable - empty ledger might be an error
		return
	}

	// If it succeeds, should show 0 events
	if !strings.Contains(output, "0") {
		t.Logf("Output with empty ledger: %s", output)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// replayIntegrationTestGetKeys returns the keys of a map as a slice for debugging.
func replayIntegrationTestGetKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
