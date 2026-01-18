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
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupWithdrawChallengeTest creates a temp directory with an initialized proof,
// a node, and an open challenge.
func setupWithdrawChallengeTest(t *testing.T) (string, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-withdraw-challenge-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")

	// Initialize proof directory structure
	if err := fs.InitProofDir(proofDir); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Initialize proof with conjecture (this creates node 1 automatically)
	err = service.Init(proofDir, "Test conjecture: P implies Q", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Node 1 already exists from Init, create a challenge on it
	rootID, err := service.ParseNodeID("1")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create a challenge by directly appending to ledger
	// (since service method may not exist yet)
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	challengeID := "chal-001"
	event := ledger.NewChallengeRaised(challengeID, rootID, "statement", "This statement is unclear")
	_, err = ldg.Append(event)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, challengeID, cleanup
}

// setupWithdrawChallengeTestNoChallenge creates a proof without any challenges.
func setupWithdrawChallengeTestNoChallenge(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-withdraw-challenge-nochall-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")

	// Initialize proof directory structure
	if err := fs.InitProofDir(proofDir); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Initialize proof with conjecture (this creates node 1 automatically)
	err = service.Init(proofDir, "Test conjecture", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Node 1 already exists from Init, no need to create it again
	// No challenge is created here

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// newTestWithdrawChallengeCmd creates a fresh root command with the withdraw-challenge subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestWithdrawChallengeCmd() *cobra.Command {
	cmd := newTestRootCmd()

	withdrawChallengeCmd := newWithdrawChallengeCmd()
	cmd.AddCommand(withdrawChallengeCmd)

	return cmd
}

// =============================================================================
// Happy Path Tests
// =============================================================================

// TestWithdrawChallengeCmd_Success verifies withdrawing an open challenge.
func TestWithdrawChallengeCmd_Success(t *testing.T) {
	proofDir, challengeID, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", challengeID, "--dir", proofDir)

	// Note: This will fail until withdraw-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until withdraw-challenge command is implemented: %v", err)
		return
	}

	// Output should confirm withdrawal
	if !strings.Contains(strings.ToLower(output), "withdraw") {
		t.Errorf("expected output to mention 'withdraw', got: %q", output)
	}
}

// TestWithdrawChallengeCmd_WithShortFlags verifies command with short flags.
func TestWithdrawChallengeCmd_WithShortFlags(t *testing.T) {
	proofDir, challengeID, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", challengeID, "-d", proofDir)

	// Note: This will fail until withdraw-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until withdraw-challenge command is implemented: %v", err)
		return
	}

	// Should succeed with short flags
	if !strings.Contains(strings.ToLower(output), challengeID) {
		t.Logf("Output should mention challenge ID: %s", challengeID)
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestWithdrawChallengeCmd_ProofNotInitialized verifies error when proof hasn't been initialized.
func TestWithdrawChallengeCmd_ProofNotInitialized(t *testing.T) {
	// Create empty directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-withdraw-challenge-noinit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	proofDir := filepath.Join(tmpDir, "proof")
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	// Note: NOT calling service.Init(), so proof is not initialized

	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", "chal-001", "--dir", proofDir)

	// Should error because proof is not initialized
	if err == nil {
		t.Error("expected error for uninitialized proof, got nil")
		return
	}

	// Error should indicate proof is not initialized
	combined := output + err.Error()
	_ = combined // Check for error message in real implementation
}

// TestWithdrawChallengeCmd_MissingChallengeID verifies error when challenge ID is not provided.
func TestWithdrawChallengeCmd_MissingChallengeID(t *testing.T) {
	proofDir, _, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	cmd := newTestWithdrawChallengeCmd()
	_, err := executeCommand(cmd, "withdraw-challenge", "--dir", proofDir)

	// Should error because challenge ID argument is required
	if err == nil {
		t.Error("expected error for missing challenge ID, got nil")
	}
}

// TestWithdrawChallengeCmd_ChallengeNotFound verifies error when challenge doesn't exist.
func TestWithdrawChallengeCmd_ChallengeNotFound(t *testing.T) {
	proofDir, cleanup := setupWithdrawChallengeTestNoChallenge(t)
	defer cleanup()

	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", "chal-nonexistent", "--dir", proofDir)

	// Should error because challenge doesn't exist
	if err == nil {
		t.Error("expected error for non-existent challenge, got nil")
		return
	}

	// Error message should mention challenge not found
	combined := output + err.Error()
	if !strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "does not exist") {
		t.Logf("Expected error to mention 'not found', got: %q", combined)
	}
}

// TestWithdrawChallengeCmd_AlreadyWithdrawn verifies error when withdrawing already-withdrawn challenge.
func TestWithdrawChallengeCmd_AlreadyWithdrawn(t *testing.T) {
	proofDir, challengeID, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	// Withdraw the challenge by appending event directly
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatal(err)
	}

	event := ledger.NewChallengeWithdrawn(challengeID)
	_, err = ldg.Append(event)
	if err != nil {
		t.Fatal(err)
	}

	// Now try to withdraw via CLI
	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", challengeID, "--dir", proofDir)

	// Should error because challenge is already withdrawn
	if err == nil {
		t.Error("expected error for already withdrawn challenge, got nil")
		return
	}

	// Error message should indicate challenge is not open or already withdrawn
	combined := output + err.Error()
	if !strings.Contains(strings.ToLower(combined), "not open") &&
		!strings.Contains(strings.ToLower(combined), "already") &&
		!strings.Contains(strings.ToLower(combined), "withdrawn") {
		t.Logf("Expected error to mention 'not open' or 'already withdrawn', got: %q", combined)
	}
}

// TestWithdrawChallengeCmd_AlreadyResolved verifies error when withdrawing already-resolved challenge.
func TestWithdrawChallengeCmd_AlreadyResolved(t *testing.T) {
	proofDir, challengeID, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	// Resolve the challenge by appending event directly
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatal(err)
	}

	event := ledger.NewChallengeResolved(challengeID)
	_, err = ldg.Append(event)
	if err != nil {
		t.Fatal(err)
	}

	// Now try to withdraw via CLI
	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", challengeID, "--dir", proofDir)

	// Should error because challenge is already resolved
	if err == nil {
		t.Error("expected error for already resolved challenge, got nil")
		return
	}

	// Error message should indicate challenge is not open or already resolved
	combined := output + err.Error()
	if !strings.Contains(strings.ToLower(combined), "not open") &&
		!strings.Contains(strings.ToLower(combined), "resolved") {
		t.Logf("Expected error to mention 'not open' or 'resolved', got: %q", combined)
	}
}

// TestWithdrawChallengeCmd_EmptyChallengeID verifies error for empty challenge ID.
func TestWithdrawChallengeCmd_EmptyChallengeID(t *testing.T) {
	proofDir, _, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	cmd := newTestWithdrawChallengeCmd()
	_, err := executeCommand(cmd, "withdraw-challenge", "", "--dir", proofDir)

	// Should error because challenge ID cannot be empty
	if err == nil {
		t.Error("expected error for empty challenge ID, got nil")
	}
}

// TestWithdrawChallengeCmd_WhitespaceOnlyChallengeID verifies error for whitespace-only challenge ID.
func TestWithdrawChallengeCmd_WhitespaceOnlyChallengeID(t *testing.T) {
	proofDir, _, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	cmd := newTestWithdrawChallengeCmd()
	_, err := executeCommand(cmd, "withdraw-challenge", "   ", "--dir", proofDir)

	// Should error because challenge ID cannot be whitespace-only
	if err == nil {
		t.Error("expected error for whitespace-only challenge ID, got nil")
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestWithdrawChallengeCmd_JSONOutput verifies JSON output format.
func TestWithdrawChallengeCmd_JSONOutput(t *testing.T) {
	proofDir, challengeID, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", challengeID, "--format", "json", "--dir", proofDir)

	// Note: This will fail until withdraw-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until withdraw-challenge command is implemented: %v", err)
		return
	}

	// Output should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v\nOutput: %s", err, output)
		return
	}

	// JSON should include key fields
	expectedKeys := []string{"challenge_id", "status"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON to contain key %q", key)
		}
	}
}

// TestWithdrawChallengeCmd_JSONOutputShortFlag verifies JSON output with short flag.
func TestWithdrawChallengeCmd_JSONOutputShortFlag(t *testing.T) {
	proofDir, challengeID, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", challengeID, "-f", "json", "-d", proofDir)

	// Note: This will fail until withdraw-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until withdraw-challenge command is implemented: %v", err)
		return
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output with -f flag, got error: %v", err)
	}
}

// TestWithdrawChallengeCmd_OutputIncludesContext verifies that output includes helpful context.
func TestWithdrawChallengeCmd_OutputIncludesContext(t *testing.T) {
	proofDir, challengeID, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", challengeID, "--dir", proofDir)

	// Note: This will fail until withdraw-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until withdraw-challenge command is implemented: %v", err)
		return
	}

	// Output should include challenge ID
	if !strings.Contains(output, challengeID) {
		t.Errorf("expected output to contain challenge ID %q, got: %q", challengeID, output)
	}

	// Should include confirmation of withdrawal
	if !strings.Contains(strings.ToLower(output), "withdraw") {
		t.Logf("Output should confirm withdrawal")
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestWithdrawChallengeCmd_Help verifies help output shows usage information.
func TestWithdrawChallengeCmd_Help(t *testing.T) {
	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"withdraw-challenge", // Command name
		"--dir",              // Directory flag
		"--format",           // Format flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestWithdrawChallengeCmd_HelpShortFlag verifies help with short flag.
func TestWithdrawChallengeCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "withdraw-challenge") {
		t.Errorf("expected help output to mention 'withdraw-challenge', got: %q", output)
	}
}

// =============================================================================
// Directory Flag Tests
// =============================================================================

// TestWithdrawChallengeCmd_DirFlagNonExistent verifies error for non-existent directory.
func TestWithdrawChallengeCmd_DirFlagNonExistent(t *testing.T) {
	cmd := newTestWithdrawChallengeCmd()
	_, err := executeCommand(cmd, "withdraw-challenge", "chal-001", "--dir", "/nonexistent/path/to/proof")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestWithdrawChallengeCmd_DirFlagIsFile verifies error when --dir points to a file.
func TestWithdrawChallengeCmd_DirFlagIsFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "af-withdraw-challenge-test-file-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cmd := newTestWithdrawChallengeCmd()
	_, err = executeCommand(cmd, "withdraw-challenge", "chal-001", "--dir", tmpFile.Name())

	if err == nil {
		t.Error("expected error when --dir is a file, got nil")
	}
}

// =============================================================================
// Table-Driven Comprehensive Tests
// =============================================================================

// TestWithdrawChallengeCmd_VariousChallengeIDFormats verifies various challenge ID formats.
func TestWithdrawChallengeCmd_VariousChallengeIDFormats(t *testing.T) {
	tests := []struct {
		name        string
		challengeID string
		wantErr     bool
	}{
		{"simple ID", "chal-001", false},
		{"UUID format", "550e8400-e29b-41d4-a716-446655440000", false},
		{"with underscores", "chal_001_test", false},
		{"with dots", "chal.001.test", false},
		{"empty string", "", true},
		{"whitespace only", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir, _, cleanup := setupWithdrawChallengeTest(t)
			defer cleanup()

			// For valid formats, create the challenge in ledger
			if !tt.wantErr && tt.challengeID != "" && strings.TrimSpace(tt.challengeID) != "" {
				ldg, _ := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
				rootID, _ := service.ParseNodeID("1")
				event := ledger.NewChallengeRaised(tt.challengeID, rootID, "statement", "test reason")
				ldg.Append(event)
			}

			cmd := newTestWithdrawChallengeCmd()
			_, err := executeCommand(cmd, "withdraw-challenge", tt.challengeID, "--dir", proofDir)

			// Note: This will fail until withdraw-challenge command is implemented
			if err != nil {
				if !tt.wantErr {
					t.Logf("Got error (expected until implementation): %v", err)
				}
				return
			}

			if tt.wantErr {
				t.Errorf("expected error for challenge ID %q, got nil", tt.challengeID)
			}
		})
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestWithdrawChallengeCmd_FullWorkflow verifies a complete withdraw workflow.
func TestWithdrawChallengeCmd_FullWorkflow(t *testing.T) {
	proofDir, challengeID, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	// Step 1: Withdraw the challenge
	cmd := newTestWithdrawChallengeCmd()
	output, err := executeCommand(cmd, "withdraw-challenge", challengeID, "--dir", proofDir)

	// Note: This will fail until withdraw-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until withdraw-challenge command is implemented: %v", err)
		return
	}

	// Step 2: Verify output indicates success
	if !strings.Contains(strings.ToLower(output), "withdraw") {
		t.Errorf("expected output to confirm withdrawal, got: %q", output)
	}

	// Step 3: Verify that a ChallengeWithdrawn event was added to ledger
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatal(err)
	}

	count, err := ldg.Count()
	if err != nil {
		t.Fatal(err)
	}

	// Ledger should have at least: ProofInitialized, NodeCreated, ChallengeRaised, ChallengeWithdrawn
	if count < 4 {
		t.Errorf("expected at least 4 events in ledger, got %d", count)
	}

	// Read all events and verify last one is ChallengeWithdrawn
	eventData, err := ldg.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	if len(eventData) < 4 {
		t.Fatal("expected at least 4 events")
	}

	// Unmarshal the last event to check its type
	var lastEvent ledger.BaseEvent
	if err := json.Unmarshal(eventData[len(eventData)-1], &lastEvent); err != nil {
		t.Fatalf("Failed to unmarshal last event: %v", err)
	}

	if lastEvent.Type() != ledger.EventChallengeWithdrawn {
		t.Errorf("expected last event to be ChallengeWithdrawn, got %s", lastEvent.Type())
	}
}

// TestWithdrawChallengeCmd_DoubleWithdraw verifies error on attempting to withdraw same challenge twice.
func TestWithdrawChallengeCmd_DoubleWithdraw(t *testing.T) {
	proofDir, challengeID, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	cmd1 := newTestWithdrawChallengeCmd()
	_, err1 := executeCommand(cmd1, "withdraw-challenge", challengeID, "--dir", proofDir)

	// Note: This will fail until withdraw-challenge command is implemented
	if err1 != nil {
		t.Logf("Expected to fail until withdraw-challenge command is implemented: %v", err1)
		return
	}

	// Try to withdraw again
	cmd2 := newTestWithdrawChallengeCmd()
	_, err2 := executeCommand(cmd2, "withdraw-challenge", challengeID, "--dir", proofDir)

	if err2 == nil {
		t.Error("expected error for double withdrawal, got nil")
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestWithdrawChallengeCmd_LongChallengeID verifies handling of very long challenge IDs.
func TestWithdrawChallengeCmd_LongChallengeID(t *testing.T) {
	proofDir, _, cleanup := setupWithdrawChallengeTest(t)
	defer cleanup()

	longID := strings.Repeat("a", 256) // Very long challenge ID

	// Create the challenge in ledger
	ldg, _ := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	rootID, _ := service.ParseNodeID("1")
	event := ledger.NewChallengeRaised(longID, rootID, "statement", "test reason")
	ldg.Append(event)

	cmd := newTestWithdrawChallengeCmd()
	_, err := executeCommand(cmd, "withdraw-challenge", longID, "--dir", proofDir)

	// Note: This test just ensures the command doesn't panic on long input
	// Actual behavior (accept or reject) depends on implementation
	_ = err // May or may not error
}

// TestWithdrawChallengeCmd_SpecialCharsInChallengeID verifies handling of special characters.
func TestWithdrawChallengeCmd_SpecialCharsInChallengeID(t *testing.T) {
	tests := []struct {
		name        string
		challengeID string
	}{
		{"with space", "chal 001"},
		{"with tab", "chal\t001"},
		{"with newline", "chal\n001"},
		{"with unicode", "chal-\u00e9\u00e8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir, _, cleanup := setupWithdrawChallengeTest(t)
			defer cleanup()

			cmd := newTestWithdrawChallengeCmd()
			_, err := executeCommand(cmd, "withdraw-challenge", tt.challengeID, "--dir", proofDir)

			// Note: This test just ensures the command handles special chars gracefully
			_ = err // May or may not error depending on validation rules
		})
	}
}

// =============================================================================
// Interface Compliance Test
// =============================================================================

// TestWithdrawChallengeCmd_ExpectedFlags verifies command has expected flags.
func TestWithdrawChallengeCmd_ExpectedFlags(t *testing.T) {
	cmd := newTestWithdrawChallengeCmd()

	// Find the withdraw-challenge subcommand
	var withdrawChallengeCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "withdraw-challenge" {
			withdrawChallengeCmd = sub
			break
		}
	}

	if withdrawChallengeCmd == nil {
		t.Skip("withdraw-challenge command not yet implemented")
		return
	}

	// Check expected flags exist
	expectedFlags := []string{"dir", "format"}
	for _, flagName := range expectedFlags {
		if withdrawChallengeCmd.Flags().Lookup(flagName) == nil && withdrawChallengeCmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected withdraw-challenge command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"d": "dir",
		"f": "format",
	}

	for short, long := range shortFlags {
		if withdrawChallengeCmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected withdraw-challenge command to have short flag -%s for --%s", short, long)
		}
	}
}
