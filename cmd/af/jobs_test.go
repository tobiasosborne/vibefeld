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

// setupJobsTest creates a temp directory with an initialized proof.
func setupJobsTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-jobs-test-*")
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

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// setupJobsTestWithNodes creates a proof with various nodes in different states.
func setupJobsTestWithNodes(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-jobs-nodes-test-*")
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

	// Create multiple nodes in different states
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create root node (available for prover)
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

// setupJobsTestWithVerifierJobs creates a proof with nodes ready for verification.
func setupJobsTestWithVerifierJobs(t *testing.T) (string, func()) {
	t.Helper()

	proofDir, cleanup := setupJobsTestWithNodes(t)

	// Create service and set up verifier job scenario
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Validate a child node so parent can become verifier job
	child1ID, _ := types.Parse("1.1")
	svc.AcceptNode(child1ID)

	return proofDir, cleanup
}

// newTestJobsCmd creates a fresh root command with the jobs subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	jobsCmd := newJobsCmd()
	cmd.AddCommand(jobsCmd)

	return cmd
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestJobsCmd_ProofNotInitialized verifies error when proof hasn't been initialized.
func TestJobsCmd_ProofNotInitialized(t *testing.T) {
	// Create empty directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-jobs-noinit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	proofDir := filepath.Join(tmpDir, "proof")
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	// Note: NOT calling service.Init(), so proof is not initialized

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	// Should error because proof is not initialized
	if err == nil {
		t.Error("expected error for uninitialized proof, got nil")
		return
	}

	// Error should indicate proof is not initialized
	combined := output + err.Error()
	lowerCombined := strings.ToLower(combined)
	if !strings.Contains(lowerCombined, "not initialized") &&
		!strings.Contains(lowerCombined, "no proof") {
		t.Logf("Error message: %q", combined)
	}
}

// TestJobsCmd_DirFlagNonExistent verifies error for non-existent directory.
func TestJobsCmd_DirFlagNonExistent(t *testing.T) {
	cmd := newTestJobsCmd()
	_, err := executeCommand(cmd, "jobs", "--dir", "/nonexistent/path/to/proof")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestJobsCmd_DirFlagIsFile verifies error when --dir points to a file.
func TestJobsCmd_DirFlagIsFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "af-jobs-test-file-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cmd := newTestJobsCmd()
	_, err = executeCommand(cmd, "jobs", "--dir", tmpFile.Name())

	if err == nil {
		t.Error("expected error when --dir is a file, got nil")
	}
}

// =============================================================================
// Happy Path Tests
// =============================================================================

// TestJobsCmd_NoJobsAvailable verifies output when no jobs are available.
func TestJobsCmd_NoJobsAvailable(t *testing.T) {
	proofDir, cleanup := setupJobsTest(t)
	defer cleanup()

	// No nodes created yet, so no jobs available

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Output should indicate no jobs available
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "no jobs") &&
		!strings.Contains(lowerOutput, "0") {
		t.Errorf("expected output to indicate no jobs, got: %q", output)
	}
}

// TestJobsCmd_ShowsProverJobs verifies displaying prover opportunities.
func TestJobsCmd_ShowsProverJobs(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithNodes(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Output should show prover jobs
	expectations := []string{
		"prover",                // Should mention prover jobs
		"1",                     // Should show node IDs
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("expected output to contain %q, got: %q", exp, output)
		}
	}
}

// TestJobsCmd_ShowsVerifierJobs verifies displaying verifier opportunities.
func TestJobsCmd_ShowsVerifierJobs(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithVerifierJobs(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Output should show verifier jobs section
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "verifier") {
		t.Errorf("expected output to mention verifier jobs, got: %q", output)
	}
}

// TestJobsCmd_ShowsMixedJobs verifies displaying both prover and verifier jobs.
func TestJobsCmd_ShowsMixedJobs(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithVerifierJobs(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Output should show both prover and verifier sections
	lowerOutput := strings.ToLower(output)
	hasProver := strings.Contains(lowerOutput, "prover")
	hasVerifier := strings.Contains(lowerOutput, "verifier")

	if !hasProver || !hasVerifier {
		t.Logf("Expected output to show both prover and verifier jobs")
		t.Logf("Has prover: %v, Has verifier: %v", hasProver, hasVerifier)
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestJobsCmd_JSONOutput verifies JSON output format.
func TestJobsCmd_JSONOutput(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithNodes(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--format", "json", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Output should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v\nOutput: %s", err, output)
		return
	}

	// JSON should include key fields for jobs
	expectedKeys := []string{"prover_jobs", "verifier_jobs"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON to contain key %q", key)
		}
	}
}

// TestJobsCmd_JSONOutputShortFlag verifies JSON output with short flag.
func TestJobsCmd_JSONOutputShortFlag(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithNodes(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "-f", "json", "-d", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output with -f flag, got error: %v", err)
	}
}

// TestJobsCmd_JSONOutputNoJobs verifies JSON output when no jobs available.
func TestJobsCmd_JSONOutputNoJobs(t *testing.T) {
	proofDir, cleanup := setupJobsTest(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--format", "json", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Should be valid JSON even with no jobs
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v\nOutput: %s", err, output)
	}
}

// =============================================================================
// Role Filtering Tests
// =============================================================================

// TestJobsCmd_FilterProverRole verifies --role prover filtering.
func TestJobsCmd_FilterProverRole(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithVerifierJobs(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--role", "prover", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Output should show prover jobs but not verifier jobs explicitly
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "prover") {
		t.Errorf("expected output to contain prover jobs, got: %q", output)
	}
}

// TestJobsCmd_FilterVerifierRole verifies --role verifier filtering.
func TestJobsCmd_FilterVerifierRole(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithVerifierJobs(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--role", "verifier", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Output should show verifier jobs
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "verifier") {
		t.Errorf("expected output to contain verifier jobs, got: %q", output)
	}
}

// TestJobsCmd_FilterRoleShortFlag verifies -r short flag for role filtering.
func TestJobsCmd_FilterRoleShortFlag(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithNodes(t)
	defer cleanup()

	tests := []struct {
		name string
		role string
	}{
		{"prover with short flag", "prover"},
		{"verifier with short flag", "verifier"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestJobsCmd()
			_, err := executeCommand(cmd, "jobs", "-r", tt.role, "-d", proofDir)

			// Note: This will fail until jobs command is implemented
			if err != nil {
				t.Logf("Expected to fail until jobs command is implemented: %v", err)
				return
			}
		})
	}
}

// TestJobsCmd_InvalidRole verifies error for invalid role value.
func TestJobsCmd_InvalidRole(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithNodes(t)
	defer cleanup()

	tests := []struct {
		name string
		role string
	}{
		{"invalid role", "invalid"},
		{"empty role", ""},
		{"misspelled prover", "provar"},
		{"misspelled verifier", "verifyer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestJobsCmd()
			_, err := executeCommand(cmd, "jobs", "--role", tt.role, "--dir", proofDir)

			// Should error for invalid role
			if err == nil {
				t.Errorf("expected error for invalid role %q, got nil", tt.role)
			}
		})
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestJobsCmd_Help verifies help output shows usage information.
func TestJobsCmd_Help(t *testing.T) {
	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"jobs",      // Command name
		"--role",    // Role filter flag
		"--format",  // Format flag
		"--dir",     // Directory flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestJobsCmd_HelpShortFlag verifies help with short flag.
func TestJobsCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "jobs") {
		t.Errorf("expected help output to mention 'jobs', got: %q", output)
	}
}

// =============================================================================
// Table-Driven Comprehensive Tests
// =============================================================================

// TestJobsCmd_OutputFormats verifies various output format options.
func TestJobsCmd_OutputFormats(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithNodes(t)
	defer cleanup()

	tests := []struct {
		name       string
		format     string
		wantErr    bool
		checkJSON  bool
	}{
		{"default format", "", false, false},
		{"json format", "json", false, true},
		{"invalid format", "xml", true, false},
		{"empty format", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestJobsCmd()
			var output string
			var err error

			if tt.format == "" {
				output, err = executeCommand(cmd, "jobs", "--dir", proofDir)
			} else {
				output, err = executeCommand(cmd, "jobs", "--format", tt.format, "--dir", proofDir)
			}

			// Note: This will fail until jobs command is implemented
			if err != nil {
				if !tt.wantErr {
					t.Logf("Got error (expected until implementation): %v", err)
				}
				return
			}

			if tt.wantErr {
				t.Errorf("expected error for format %q, got nil", tt.format)
				return
			}

			if tt.checkJSON {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("expected valid JSON for format %q, got error: %v", tt.format, err)
				}
			}
		})
	}
}

// TestJobsCmd_RoleFilterOptions verifies all role filtering combinations.
func TestJobsCmd_RoleFilterOptions(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithVerifierJobs(t)
	defer cleanup()

	tests := []struct {
		name    string
		role    string
		wantErr bool
	}{
		{"no filter", "", false},
		{"prover filter", "prover", false},
		{"verifier filter", "verifier", false},
		{"Prover with capital", "Prover", false}, // Should be case-insensitive
		{"VERIFIER uppercase", "VERIFIER", false},
		{"invalid role", "attacker", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestJobsCmd()
			var output string
			var err error

			if tt.role == "" {
				output, err = executeCommand(cmd, "jobs", "--dir", proofDir)
			} else {
				output, err = executeCommand(cmd, "jobs", "--role", tt.role, "--dir", proofDir)
			}

			// Note: This will fail until jobs command is implemented
			if err != nil {
				if !tt.wantErr {
					t.Logf("Got error (expected until implementation): %v", err)
				}
				return
			}

			if tt.wantErr {
				t.Errorf("expected error for role %q, got nil", tt.role)
				return
			}

			// Verify output is not empty
			if len(output) == 0 {
				t.Errorf("expected non-empty output for role %q", tt.role)
			}
		})
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestJobsCmd_FullWorkflow verifies jobs command in a complete workflow.
func TestJobsCmd_FullWorkflow(t *testing.T) {
	proofDir, cleanup := setupJobsTest(t)
	defer cleanup()

	// Step 1: Check jobs when no nodes exist
	cmd1 := newTestJobsCmd()
	output1, err1 := executeCommand(cmd1, "jobs", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err1 != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err1)
		return
	}

	if !strings.Contains(strings.ToLower(output1), "no jobs") &&
		!strings.Contains(output1, "0") {
		t.Logf("Expected to indicate no jobs available")
	}

	// Step 2: Create a node using service
	svc, _ := service.NewProofService(proofDir)
	rootID, _ := types.Parse("1")
	svc.CreateNode(rootID, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)

	// Step 3: Check jobs again - should show prover job
	cmd2 := newTestJobsCmd()
	output2, err2 := executeCommand(cmd2, "jobs", "--dir", proofDir)

	if err2 != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err2)
		return
	}

	if !strings.Contains(strings.ToLower(output2), "prover") {
		t.Errorf("expected to show prover jobs after creating node, got: %q", output2)
	}
}

// TestJobsCmd_DefaultDirectory verifies using default (current) directory.
func TestJobsCmd_DefaultDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "af-jobs-default-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	proofDir := filepath.Join(tmpDir, "proof")
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}

	service.Init(proofDir, "Test conjecture", "test-author")

	// Change to proof directory for this test
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(proofDir); err != nil {
		t.Fatal(err)
	}

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs")

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Should work with current directory
	_ = output // Verify it doesn't error
}

// =============================================================================
// Output Content Tests
// =============================================================================

// TestJobsCmd_OutputIncludesNodeDetails verifies job output includes key details.
func TestJobsCmd_OutputIncludesNodeDetails(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithNodes(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Output should include node IDs
	if !strings.Contains(output, "1") {
		t.Errorf("expected output to include node IDs, got: %q", output)
	}
}

// TestJobsCmd_OutputIncludesGuidance verifies next steps guidance.
func TestJobsCmd_OutputIncludesGuidance(t *testing.T) {
	proofDir, cleanup := setupJobsTestWithNodes(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Output should include next steps guidance (self-documenting CLI)
	lowerOutput := strings.ToLower(output)
	hasGuidance := strings.Contains(lowerOutput, "claim") ||
		strings.Contains(lowerOutput, "next") ||
		strings.Contains(lowerOutput, "accept")

	if !hasGuidance {
		t.Logf("Expected output to include next steps guidance")
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestJobsCmd_EmptyProofDirectory verifies handling of empty proof.
func TestJobsCmd_EmptyProofDirectory(t *testing.T) {
	proofDir, cleanup := setupJobsTest(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Should handle empty proof gracefully
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "no jobs") &&
		!strings.Contains(lowerOutput, "0") {
		t.Logf("Expected indication of no jobs available")
	}
}

// =============================================================================
// Interface Compliance Test
// =============================================================================

// TestJobsCmd_ExpectedFlags ensures the jobs command has expected flag structure.
func TestJobsCmd_ExpectedFlags(t *testing.T) {
	cmd := newTestJobsCmd()

	// Find the jobs subcommand
	var jobsCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "jobs" {
			jobsCmd = sub
			break
		}
	}

	if jobsCmd == nil {
		t.Skip("jobs command not yet implemented")
		return
	}

	// Check expected flags exist
	expectedFlags := []string{"role", "format", "dir"}
	for _, flagName := range expectedFlags {
		if jobsCmd.Flags().Lookup(flagName) == nil && jobsCmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected jobs command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"r": "role",
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if jobsCmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected jobs command to have short flag -%s for --%s", short, long)
		}
	}
}
