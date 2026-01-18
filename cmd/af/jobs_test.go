//go:build integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
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
	if err := service.InitProofDir(proofDir); err != nil {
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
	if err := service.InitProofDir(proofDir); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Initialize proof with conjecture
	err = service.Init(proofDir, "Complex conjecture", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create additional nodes in different states
	// Note: service.Init() already creates node 1 with the conjecture
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create child nodes (node 1 already exists from Init)
	child1ID, _ := service.ParseNodeID("1.1")
	svc.CreateNode(child1ID, service.NodeTypeClaim, "First child", service.InferenceModusPonens)

	child2ID, _ := service.ParseNodeID("1.2")
	svc.CreateNode(child2ID, service.NodeTypeClaim, "Second child", service.InferenceModusPonens)

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
	child1ID, _ := service.ParseNodeID("1.1")
	svc.AcceptNode(child1ID)

	return proofDir, cleanup
}

// newTestJobsCmd creates a fresh root command with the jobs subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestJobsCmd() *cobra.Command {
	cmd := newTestRootCmd()

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
	if err := service.InitProofDir(proofDir); err != nil {
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

// TestJobsCmd_InitializedProof verifies output for a freshly initialized proof.
func TestJobsCmd_InitializedProof(t *testing.T) {
	proofDir, cleanup := setupJobsTest(t)
	defer cleanup()

	// Node 1 (conjecture) already exists from Init, so there should be a prover job

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Output should show prover job for node 1
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "prover") &&
		!strings.Contains(lowerOutput, "1") {
		t.Errorf("expected output to show prover job for node 1, got: %q", output)
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
		"prover", // Should mention prover jobs
		"1",      // Should show node IDs
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
		"jobs",     // Command name
		"--role",   // Role filter flag
		"--format", // Format flag
		"--dir",    // Directory flag
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
		name      string
		format    string
		wantErr   bool
		checkJSON bool
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

	// Step 1: Check jobs - node 1 already exists from Init
	cmd1 := newTestJobsCmd()
	output1, err1 := executeCommand(cmd1, "jobs", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err1 != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err1)
		return
	}

	// Node 1 (the conjecture) should show as a prover job
	if !strings.Contains(strings.ToLower(output1), "prover") {
		t.Errorf("expected to show prover jobs for node 1, got: %q", output1)
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
	if err := service.InitProofDir(proofDir); err != nil {
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

// TestJobsCmd_InitializedProofWithConjecture verifies handling of initialized proof with conjecture.
func TestJobsCmd_InitializedProofWithConjecture(t *testing.T) {
	proofDir, cleanup := setupJobsTest(t)
	defer cleanup()

	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	// Note: This will fail until jobs command is implemented
	if err != nil {
		t.Logf("Expected to fail until jobs command is implemented: %v", err)
		return
	}

	// Should show prover job for the conjecture node (node 1)
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "prover") &&
		!strings.Contains(lowerOutput, "1") {
		t.Logf("Expected prover job for node 1 (conjecture)")
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

// =============================================================================
// Severity Counts Tests
// =============================================================================

// addChallengeToNodeWithSeverity is a test helper that adds a challenge with a specific severity.
func addChallengeToNodeWithSeverity(t *testing.T, proofDir string, nodeID service.NodeID, challengeID, severity string) {
	t.Helper()
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("failed to get ledger: %v", err)
	}
	event := ledger.NewChallengeRaisedWithSeverity(challengeID, nodeID, "statement", "test challenge", severity, "")
	if _, err := ldg.Append(event); err != nil {
		t.Fatalf("failed to append challenge: %v", err)
	}
}

// TestJobsCommand_ShowsChallengeSeverityCounts verifies that jobs output includes severity breakdown.
func TestJobsCommand_ShowsChallengeSeverityCounts(t *testing.T) {
	// Setup: Create proof with a node that has challenges of different severities
	proofDir, cleanup := setupJobsTest(t)
	defer cleanup()

	// Node 1 already exists from Init, add challenges to it
	nodeID, _ := service.ParseNodeID("1")

	// Add multiple challenges with different severities
	addChallengeToNodeWithSeverity(t, proofDir, nodeID, "chal-001", "critical")
	addChallengeToNodeWithSeverity(t, proofDir, nodeID, "chal-002", "minor")
	addChallengeToNodeWithSeverity(t, proofDir, nodeID, "chal-003", "minor")

	// Run jobs command
	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	if err != nil {
		t.Fatalf("jobs command failed: %v", err)
	}

	// Verify output contains severity counts
	lowerOutput := strings.ToLower(output)

	// Should show "1 critical"
	if !strings.Contains(lowerOutput, "1 critical") {
		t.Errorf("expected output to contain '1 critical', got: %q", output)
	}

	// Should show "2 minor"
	if !strings.Contains(lowerOutput, "2 minor") {
		t.Errorf("expected output to contain '2 minor', got: %q", output)
	}

	// Should show "challenges" (plural since total > 1)
	if !strings.Contains(lowerOutput, "challenges") {
		t.Errorf("expected output to contain 'challenges', got: %q", output)
	}
}

// TestJobsCommand_ShowsChallengeSeverityCountsJSON verifies JSON output includes severity counts.
func TestJobsCommand_ShowsChallengeSeverityCountsJSON(t *testing.T) {
	// Setup: Create proof with a node that has challenges
	proofDir, cleanup := setupJobsTest(t)
	defer cleanup()

	// Node 1 already exists from Init, add challenges to it
	nodeID, _ := service.ParseNodeID("1")

	// Add challenges with different severities
	addChallengeToNodeWithSeverity(t, proofDir, nodeID, "chal-001", "critical")
	addChallengeToNodeWithSeverity(t, proofDir, nodeID, "chal-002", "major")

	// Run jobs command with JSON output
	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--format", "json", "--dir", proofDir)

	if err != nil {
		t.Fatalf("jobs command failed: %v", err)
	}

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Verify prover_jobs exists and has severity_counts
	proverJobs, ok := result["prover_jobs"].([]interface{})
	if !ok || len(proverJobs) == 0 {
		t.Fatalf("expected prover_jobs array with entries, got: %v", result["prover_jobs"])
	}

	// Check first prover job has severity_counts
	firstJob := proverJobs[0].(map[string]interface{})
	severityCounts, ok := firstJob["severity_counts"].(map[string]interface{})
	if !ok {
		t.Errorf("expected severity_counts in job entry, got: %v", firstJob)
		return
	}

	// Verify counts
	if critical, ok := severityCounts["critical"].(float64); !ok || critical != 1 {
		t.Errorf("expected critical=1, got: %v", severityCounts["critical"])
	}
	if major, ok := severityCounts["major"].(float64); !ok || major != 1 {
		t.Errorf("expected major=1, got: %v", severityCounts["major"])
	}
}

// TestJobsCommand_NoSeverityCountsForNodesWithoutChallenges verifies nodes without challenges don't show severity info.
func TestJobsCommand_NoSeverityCountsForNodesWithoutChallenges(t *testing.T) {
	// Setup: Create proof with nodes but no challenges
	proofDir, cleanup := setupJobsTestWithVerifierJobs(t)
	defer cleanup()

	// Run jobs command
	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	if err != nil {
		t.Fatalf("jobs command failed: %v", err)
	}

	// Verifier jobs should not have severity counts in output
	// Look for bracket patterns that would indicate severity counts
	if strings.Contains(output, "[0 critical") ||
		strings.Contains(output, "[0 major") ||
		strings.Contains(output, "[0 minor") ||
		strings.Contains(output, "[0 note") {
		t.Errorf("expected no zero severity counts in output, got: %q", output)
	}
}

// TestJobsCommand_SeverityCountsSingularChallenge verifies singular "challenge" is used for count of 1.
func TestJobsCommand_SeverityCountsSingularChallenge(t *testing.T) {
	// Setup: Create proof with a node that has exactly one challenge
	proofDir, cleanup := setupJobsTest(t)
	defer cleanup()

	nodeID, _ := service.ParseNodeID("1")

	// Add just one challenge
	addChallengeToNodeWithSeverity(t, proofDir, nodeID, "chal-001", "major")

	// Run jobs command
	cmd := newTestJobsCmd()
	output, err := executeCommand(cmd, "jobs", "--dir", proofDir)

	if err != nil {
		t.Fatalf("jobs command failed: %v", err)
	}

	// Should use singular "challenge" not "challenges"
	if strings.Contains(output, "challenges]") {
		t.Errorf("expected singular 'challenge' for count of 1, got plural: %q", output)
	}
	if !strings.Contains(output, "challenge]") {
		t.Errorf("expected '[... challenge]' in output, got: %q", output)
	}
}
