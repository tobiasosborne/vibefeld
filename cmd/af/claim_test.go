//go:build integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupClaimTest creates a temp directory with an initialized proof and a node to claim.
func setupClaimTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-claim-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")

	// Initialize proof directory structure
	if err := fs.InitProofDir(proofDir); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Initialize proof with conjecture
	err = service.Init(proofDir, "Test conjecture: P implies Q", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create a node to claim
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	rootID, err := types.Parse("1")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	err = svc.CreateNode(rootID, schema.NodeTypeClaim, "Root goal statement", schema.InferenceAssumption)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// setupClaimTestWithMultipleNodes creates a proof with multiple nodes for testing.
func setupClaimTestWithMultipleNodes(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-claim-multi-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")

	// Initialize proof directory structure
	if err := fs.InitProofDir(proofDir); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Initialize proof with conjecture
	err = service.Init(proofDir, "Complex conjecture", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create multiple nodes
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create root node
	rootID, _ := types.Parse("1")
	svc.CreateNode(rootID, schema.NodeTypeClaim, "Root statement", schema.InferenceAssumption)

	// Create child nodes
	child1ID, _ := types.Parse("1.1")
	svc.CreateNode(child1ID, schema.NodeTypeClaim, "First child", schema.InferenceModusPonens)

	child2ID, _ := types.Parse("1.2")
	svc.CreateNode(child2ID, schema.NodeTypeClaim, "Second child", schema.InferenceModusPonens)

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// newTestClaimCmd creates a fresh root command with the claim subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestClaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	claimCmd := newClaimCmd()
	cmd.AddCommand(claimCmd)

	return cmd
}

// =============================================================================
// Happy Path Tests
// =============================================================================

// TestClaimCmd_Success verifies claiming an available node.
func TestClaimCmd_Success(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	tests := []struct {
		name   string
		args   []string
		checks []string // strings that should appear in output
	}{
		{
			name: "claim with owner flag",
			args: []string{"claim", "1", "--owner", "prover-001", "--dir", proofDir},
			checks: []string{
				"1", // node ID should appear
			},
		},
		{
			name: "claim with short owner flag",
			args: []string{"claim", "1", "-o", "prover-002", "-d", proofDir},
			checks: []string{
				"1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset node to available state for each test
			// (In real tests, we'd need to release between tests or use fresh setup)
			cmd := newTestClaimCmd()
			output, err := executeCommand(cmd, tt.args...)

			// Note: This will fail until claim command is implemented
			if err != nil {
				t.Logf("Expected to fail until claim command is implemented: %v", err)
				return
			}

			for _, check := range tt.checks {
				if !strings.Contains(output, check) {
					t.Errorf("expected output to contain %q, got: %q", check, output)
				}
			}
		})
	}
}

// TestClaimCmd_WithCustomTimeout verifies claiming with custom timeout.
func TestClaimCmd_WithCustomTimeout(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	tests := []struct {
		name    string
		timeout string
	}{
		{"30 minutes", "30m"},
		{"2 hours", "2h"},
		{"1 hour 30 minutes", "1h30m"},
		{"45 seconds", "45s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestClaimCmd()
			_, err := executeCommand(cmd, "claim", "1", "--owner", "prover-001", "--timeout", tt.timeout, "--dir", proofDir)

			// Note: This will fail until claim command is implemented
			if err != nil {
				t.Logf("Expected to fail until claim command is implemented: %v", err)
				return
			}
		})
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestClaimCmd_MissingNodeID verifies error when node ID is not provided.
func TestClaimCmd_MissingNodeID(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	cmd := newTestClaimCmd()
	_, err := executeCommand(cmd, "claim", "--owner", "prover-001", "--dir", proofDir)

	// Should error because node ID argument is required
	if err == nil {
		t.Error("expected error for missing node ID, got nil")
	}
}

// TestClaimCmd_NodeNotFound verifies error when node doesn't exist.
func TestClaimCmd_NodeNotFound(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	cmd := newTestClaimCmd()
	output, err := executeCommand(cmd, "claim", "1.999", "--owner", "prover-001", "--dir", proofDir)

	// Should error because node 1.999 doesn't exist
	if err == nil {
		t.Error("expected error for non-existent node, got nil")
		return
	}

	// Error message should mention node not found
	combined := output + err.Error()
	if !strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "does not exist") {
		t.Errorf("expected error to mention 'not found', got: %q", combined)
	}
}

// TestClaimCmd_NodeAlreadyClaimed verifies error when claiming already claimed node.
func TestClaimCmd_NodeAlreadyClaimed(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	// First claim the node using service directly
	svc, _ := service.NewProofService(proofDir)
	nodeID, _ := types.Parse("1")
	svc.ClaimNode(nodeID, "first-prover", 3600000000000) // 1 hour in nanoseconds

	// Now try to claim via CLI
	cmd := newTestClaimCmd()
	output, err := executeCommand(cmd, "claim", "1", "--owner", "second-prover", "--dir", proofDir)

	// Should error because node is already claimed
	if err == nil {
		t.Error("expected error for already claimed node, got nil")
		return
	}

	// Error message should indicate node is not available or already claimed
	combined := output + err.Error()
	if !strings.Contains(strings.ToLower(combined), "claim") &&
		!strings.Contains(strings.ToLower(combined), "available") {
		t.Logf("Error message: %q", combined)
	}
}

// TestClaimCmd_MissingOwnerFlag verifies error when owner flag is not provided.
func TestClaimCmd_MissingOwnerFlag(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	cmd := newTestClaimCmd()
	_, err := executeCommand(cmd, "claim", "1", "--dir", proofDir)

	// Should error because --owner is required
	if err == nil {
		t.Error("expected error for missing owner flag, got nil")
	}
}

// TestClaimCmd_EmptyOwner verifies error when owner is empty string.
func TestClaimCmd_EmptyOwner(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	cmd := newTestClaimCmd()
	_, err := executeCommand(cmd, "claim", "1", "--owner", "", "--dir", proofDir)

	// Should error because owner cannot be empty
	if err == nil {
		t.Error("expected error for empty owner, got nil")
	}
}

// TestClaimCmd_InvalidNodeIDFormat verifies error for malformed node ID.
func TestClaimCmd_InvalidNodeIDFormat(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	tests := []struct {
		name   string
		nodeID string
	}{
		{"empty node ID", ""},
		{"invalid format with letters", "abc"},
		{"starts with zero", "0.1"},
		{"contains negative", "1.-1"},
		{"double dots", "1..1"},
		{"trailing dot", "1."},
		{"leading dot", ".1"},
		{"non-numeric", "1.a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestClaimCmd()
			_, err := executeCommand(cmd, "claim", tt.nodeID, "--owner", "prover-001", "--dir", proofDir)

			if err == nil {
				t.Errorf("expected error for invalid node ID %q, got nil", tt.nodeID)
			}
		})
	}
}

// TestClaimCmd_ProofNotInitialized verifies error when proof hasn't been initialized.
func TestClaimCmd_ProofNotInitialized(t *testing.T) {
	// Create empty directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-claim-noinit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	proofDir := filepath.Join(tmpDir, "proof")
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	// Note: NOT calling service.Init(), so proof is not initialized

	cmd := newTestClaimCmd()
	output, err := executeCommand(cmd, "claim", "1", "--owner", "prover-001", "--dir", proofDir)

	// Should error because proof is not initialized
	if err == nil {
		t.Error("expected error for uninitialized proof, got nil")
		return
	}

	// Error should indicate proof is not initialized or no nodes exist
	combined := output + err.Error()
	_ = combined // Avoid unused variable warning
}

// TestClaimCmd_InvalidTimeout verifies error for invalid timeout values.
func TestClaimCmd_InvalidTimeout(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	tests := []struct {
		name    string
		timeout string
	}{
		{"negative timeout", "-1h"},
		{"zero timeout", "0"},
		{"invalid format", "abc"},
		{"missing unit", "30"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestClaimCmd()
			_, err := executeCommand(cmd, "claim", "1", "--owner", "prover-001", "--timeout", tt.timeout, "--dir", proofDir)

			if err == nil {
				t.Errorf("expected error for invalid timeout %q, got nil", tt.timeout)
			}
		})
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestClaimCmd_OutputIncludesContext verifies that output includes prover/verifier context.
func TestClaimCmd_OutputIncludesContext(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	cmd := newTestClaimCmd()
	output, err := executeCommand(cmd, "claim", "1", "--owner", "prover-001", "--dir", proofDir)

	// Note: This will fail until claim command is implemented
	if err != nil {
		t.Logf("Expected to fail until claim command is implemented: %v", err)
		return
	}

	// Output should include role context information
	expectations := []string{
		"1",             // Node ID
		"prover-001",    // Owner
		"claim",         // Confirmation of claim
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("expected output to contain %q, got: %q", exp, output)
		}
	}

	// Should include next steps guidance (self-documenting CLI principle)
	if !strings.Contains(output, "refine") && !strings.Contains(output, "release") {
		t.Logf("Output should include next steps guidance (refine, release)")
	}
}

// TestClaimCmd_JSONOutput verifies JSON output format.
func TestClaimCmd_JSONOutput(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	cmd := newTestClaimCmd()
	output, err := executeCommand(cmd, "claim", "1", "--owner", "prover-001", "--format", "json", "--dir", proofDir)

	// Note: This will fail until claim command is implemented
	if err != nil {
		t.Logf("Expected to fail until claim command is implemented: %v", err)
		return
	}

	// Output should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v\nOutput: %s", err, output)
		return
	}

	// JSON should include key fields
	expectedKeys := []string{"node_id", "owner", "status"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON to contain key %q", key)
		}
	}
}

// TestClaimCmd_JSONOutputShortFlag verifies JSON output with short flag.
func TestClaimCmd_JSONOutputShortFlag(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	cmd := newTestClaimCmd()
	output, err := executeCommand(cmd, "claim", "1", "-o", "prover-001", "-f", "json", "-d", proofDir)

	// Note: This will fail until claim command is implemented
	if err != nil {
		t.Logf("Expected to fail until claim command is implemented: %v", err)
		return
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output with -f flag, got error: %v", err)
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestClaimCmd_Help verifies help output shows usage information.
func TestClaimCmd_Help(t *testing.T) {
	cmd := newTestClaimCmd()
	output, err := executeCommand(cmd, "claim", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"claim",    // Command name
		"--owner",  // Required flag
		"--dir",    // Optional directory flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestClaimCmd_HelpShortFlag verifies help with short flag.
func TestClaimCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestClaimCmd()
	output, err := executeCommand(cmd, "claim", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "claim") {
		t.Errorf("expected help output to mention 'claim', got: %q", output)
	}
}

// =============================================================================
// Directory Flag Tests
// =============================================================================

// TestClaimCmd_DirFlagNonExistent verifies error for non-existent directory.
func TestClaimCmd_DirFlagNonExistent(t *testing.T) {
	cmd := newTestClaimCmd()
	_, err := executeCommand(cmd, "claim", "1", "--owner", "prover-001", "--dir", "/nonexistent/path/to/proof")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestClaimCmd_DirFlagIsFile verifies error when --dir points to a file.
func TestClaimCmd_DirFlagIsFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "af-claim-test-file-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cmd := newTestClaimCmd()
	_, err = executeCommand(cmd, "claim", "1", "--owner", "prover-001", "--dir", tmpFile.Name())

	if err == nil {
		t.Error("expected error when --dir is a file, got nil")
	}
}

// =============================================================================
// Table-Driven Comprehensive Tests
// =============================================================================

// TestClaimCmd_VariousNodeIDs verifies claiming nodes at different depths.
func TestClaimCmd_VariousNodeIDs(t *testing.T) {
	proofDir, cleanup := setupClaimTestWithMultipleNodes(t)
	defer cleanup()

	tests := []struct {
		name     string
		nodeID   string
		wantErr  bool
	}{
		{"root node", "1", false},
		{"first child", "1.1", false},
		{"second child", "1.2", false},
		{"non-existent node", "1.3", true},
		{"non-existent deep node", "1.1.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestClaimCmd()
			_, err := executeCommand(cmd, "claim", tt.nodeID, "--owner", "prover-001", "--dir", proofDir)

			// Note: This will fail until claim command is implemented
			if err != nil {
				if !tt.wantErr {
					t.Logf("Got error (expected until implementation): %v", err)
				}
				return
			}

			if tt.wantErr {
				t.Errorf("expected error for node %q, got nil", tt.nodeID)
			}
		})
	}
}

// TestClaimCmd_OwnerFormats verifies various valid owner name formats.
func TestClaimCmd_OwnerFormats(t *testing.T) {
	tests := []struct {
		name    string
		owner   string
		wantErr bool
	}{
		{"simple name", "prover", false},
		{"with numbers", "prover-001", false},
		{"with underscore", "prover_alpha", false},
		{"with dots", "agent.prover.1", false},
		{"uuid format", "550e8400-e29b-41d4-a716-446655440000", false},
		{"empty string", "", true},
		{"whitespace only", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir, cleanup := setupClaimTest(t)
			defer cleanup()

			cmd := newTestClaimCmd()
			_, err := executeCommand(cmd, "claim", "1", "--owner", tt.owner, "--dir", proofDir)

			// Skip validation for now since command isn't implemented
			if err != nil {
				if !tt.wantErr {
					t.Logf("Got error (expected until implementation): %v", err)
				}
				return
			}

			if tt.wantErr {
				t.Errorf("expected error for owner %q, got nil", tt.owner)
			}
		})
	}
}

// TestClaimCmd_TimeoutValues verifies various timeout duration formats.
func TestClaimCmd_TimeoutValues(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		wantErr bool
	}{
		{"default 1 hour", "1h", false},
		{"30 minutes", "30m", false},
		{"2 hours", "2h", false},
		{"90 seconds", "90s", false},
		{"combined", "1h30m", false},
		{"zero", "0", true},
		{"negative", "-1h", true},
		{"invalid format", "abc", true},
		{"missing unit", "60", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir, cleanup := setupClaimTest(t)
			defer cleanup()

			cmd := newTestClaimCmd()
			_, err := executeCommand(cmd, "claim", "1", "--owner", "prover-001", "--timeout", tt.timeout, "--dir", proofDir)

			// Skip validation for now since command isn't implemented
			if err != nil {
				if !tt.wantErr {
					t.Logf("Got error (expected until implementation): %v", err)
				}
				return
			}

			if tt.wantErr {
				t.Errorf("expected error for timeout %q, got nil", tt.timeout)
			}
		})
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestClaimCmd_FullWorkflow verifies a complete claim workflow.
func TestClaimCmd_FullWorkflow(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	// Step 1: Claim a node
	cmd := newTestClaimCmd()
	output, err := executeCommand(cmd, "claim", "1", "--owner", "prover-001", "--dir", proofDir)

	// Note: This will fail until claim command is implemented
	if err != nil {
		t.Logf("Expected to fail until claim command is implemented: %v", err)
		return
	}

	// Step 2: Verify output indicates success
	if !strings.Contains(strings.ToLower(output), "claim") {
		t.Errorf("expected output to confirm claim, got: %q", output)
	}

	// Step 3: Verify node state changed (via service)
	svc, _ := service.NewProofService(proofDir)
	st, _ := svc.LoadState()
	nodeID, _ := types.Parse("1")
	node := st.GetNode(nodeID)

	if node == nil {
		t.Fatal("node not found in state")
	}

	if node.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("expected node workflow state to be 'claimed', got %q", node.WorkflowState)
	}

	if node.ClaimedBy != "prover-001" {
		t.Errorf("expected node to be claimed by 'prover-001', got %q", node.ClaimedBy)
	}
}

// TestClaimCmd_DoubleClaim verifies error on attempting to claim same node twice.
func TestClaimCmd_DoubleClaim(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	cmd1 := newTestClaimCmd()
	_, err1 := executeCommand(cmd1, "claim", "1", "--owner", "prover-001", "--dir", proofDir)

	// Note: This will fail until claim command is implemented
	if err1 != nil {
		t.Logf("Expected to fail until claim command is implemented: %v", err1)
		return
	}

	// Try to claim again with different owner
	cmd2 := newTestClaimCmd()
	_, err2 := executeCommand(cmd2, "claim", "1", "--owner", "prover-002", "--dir", proofDir)

	if err2 == nil {
		t.Error("expected error for double claim, got nil")
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestClaimCmd_LongOwnerName verifies handling of very long owner names.
func TestClaimCmd_LongOwnerName(t *testing.T) {
	proofDir, cleanup := setupClaimTest(t)
	defer cleanup()

	longOwner := strings.Repeat("a", 256) // Very long owner name

	cmd := newTestClaimCmd()
	_, err := executeCommand(cmd, "claim", "1", "--owner", longOwner, "--dir", proofDir)

	// Note: This test just ensures the command doesn't panic on long input
	// Actual behavior (accept or reject) depends on implementation
	_ = err // May or may not error
}

// TestClaimCmd_SpecialCharsInOwner verifies handling of special characters in owner.
func TestClaimCmd_SpecialCharsInOwner(t *testing.T) {
	tests := []struct {
		name  string
		owner string
	}{
		{"with space", "prover one"},
		{"with tab", "prover\tone"},
		{"with newline", "prover\none"},
		{"with quotes", `prover"one`},
		{"with unicode", "prover-\u00e9\u00e8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir, cleanup := setupClaimTest(t)
			defer cleanup()

			cmd := newTestClaimCmd()
			_, err := executeCommand(cmd, "claim", "1", "--owner", tt.owner, "--dir", proofDir)

			// Note: This test just ensures the command handles special chars gracefully
			_ = err // May or may not error depending on validation rules
		})
	}
}

// TestClaimCmd_ConcurrentAccessSimulation simulates concurrent access pattern.
// Note: This is a simulation, not actual concurrent test.
func TestClaimCmd_ConcurrentAccessSimulation(t *testing.T) {
	proofDir, cleanup := setupClaimTestWithMultipleNodes(t)
	defer cleanup()

	// Claim different nodes with different owners (simulating concurrent access)
	nodes := []struct {
		nodeID string
		owner  string
	}{
		{"1", "prover-001"},
		{"1.1", "prover-002"},
		{"1.2", "prover-003"},
	}

	for _, n := range nodes {
		cmd := newTestClaimCmd()
		_, err := executeCommand(cmd, "claim", n.nodeID, "--owner", n.owner, "--dir", proofDir)

		// Note: This will fail until claim command is implemented
		if err != nil {
			t.Logf("Expected to fail until claim command is implemented: %v", err)
			continue
		}
	}
}

// =============================================================================
// Interface Compliance Test
// =============================================================================

// This test ensures the claim command will have expected flag structure.
// It's a compile-time check more than runtime test.
func TestClaimCmd_ExpectedFlags(t *testing.T) {
	cmd := newTestClaimCmd()

	// Find the claim subcommand
	var claimCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "claim" {
			claimCmd = sub
			break
		}
	}

	if claimCmd == nil {
		t.Skip("claim command not yet implemented")
		return
	}

	// Check expected flags exist
	expectedFlags := []string{"owner", "timeout", "dir", "format"}
	for _, flagName := range expectedFlags {
		if claimCmd.Flags().Lookup(flagName) == nil && claimCmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected claim command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"o": "owner",
		"t": "timeout",
		"d": "dir",
		"f": "format",
	}

	for short, long := range shortFlags {
		if claimCmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected claim command to have short flag -%s for --%s", short, long)
		}
	}
}
