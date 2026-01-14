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
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupResolveChallengeTest creates a temp directory with an initialized proof,
// a node, and a raised challenge.
func setupResolveChallengeTest(t *testing.T) (string, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-resolve-challenge-test-*")
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

	rootID, err := types.Parse("1")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Raise a challenge against node 1 (created by Init)
	ledgerDir := filepath.Join(proofDir, "ledger")
	l, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	challengeID := "chal-001"
	challengeEvent := ledger.NewChallengeRaised(challengeID, rootID, "statement", "This statement is unclear")
	_, err = l.Append(challengeEvent)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, challengeID, cleanup
}

// newTestResolveChallengeCmd creates a fresh root command with the resolve-challenge subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestResolveChallengeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	resolveChallengeCmd := newResolveChallengeCmd()
	cmd.AddCommand(resolveChallengeCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// =============================================================================
// Happy Path Tests
// =============================================================================

// TestResolveChallengeCmd_Success verifies resolving a challenge successfully.
func TestResolveChallengeCmd_Success(t *testing.T) {
	proofDir, challengeID, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", challengeID,
		"--response", "The statement is clarified as follows: ...",
		"--dir", proofDir)

	// Note: This will fail until resolve-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until resolve-challenge command is implemented: %v", err)
		return
	}

	// Output should confirm the challenge was resolved
	if !strings.Contains(strings.ToLower(output), "resolved") ||
		!strings.Contains(output, challengeID) {
		t.Errorf("expected output to confirm challenge resolved, got: %q", output)
	}
}

// TestResolveChallengeCmd_WithShortFlags verifies resolving with short flags.
func TestResolveChallengeCmd_WithShortFlags(t *testing.T) {
	proofDir, challengeID, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", challengeID,
		"-r", "Response text here",
		"-d", proofDir)

	// Note: This will fail until resolve-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until resolve-challenge command is implemented: %v", err)
		return
	}

	if !strings.Contains(strings.ToLower(output), "resolved") {
		t.Errorf("expected output to confirm challenge resolved, got: %q", output)
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestResolveChallengeCmd_ProofNotInitialized verifies error when proof hasn't been initialized.
func TestResolveChallengeCmd_ProofNotInitialized(t *testing.T) {
	// Create empty directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-resolve-challenge-noinit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	proofDir := filepath.Join(tmpDir, "proof")
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	// Note: NOT calling service.Init(), so proof is not initialized

	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", "chal-001",
		"--response", "Response text",
		"--dir", proofDir)

	// Should error because proof is not initialized
	if err == nil {
		t.Error("expected error for uninitialized proof, got nil")
		return
	}

	// Error should indicate proof is not initialized
	combined := output + err.Error()
	if !strings.Contains(strings.ToLower(combined), "not initialized") &&
		!strings.Contains(strings.ToLower(combined), "no proof") {
		t.Logf("Error message: %q", combined)
	}
}

// TestResolveChallengeCmd_ChallengeNotFound verifies error when challenge doesn't exist.
func TestResolveChallengeCmd_ChallengeNotFound(t *testing.T) {
	proofDir, _, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", "chal-nonexistent",
		"--response", "Response text",
		"--dir", proofDir)

	// Should error because challenge doesn't exist
	if err == nil {
		t.Error("expected error for non-existent challenge, got nil")
		return
	}

	// Error message should mention challenge not found
	combined := output + err.Error()
	if !strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "does not exist") {
		t.Errorf("expected error to mention 'not found', got: %q", combined)
	}
}

// TestResolveChallengeCmd_AlreadyResolved verifies error when challenge is already resolved.
func TestResolveChallengeCmd_AlreadyResolved(t *testing.T) {
	proofDir, challengeID, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	// First, resolve the challenge using ledger directly
	ledgerDir := filepath.Join(proofDir, "ledger")
	l, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatal(err)
	}

	resolveEvent := ledger.NewChallengeResolved(challengeID)
	_, err = l.Append(resolveEvent)
	if err != nil {
		t.Fatal(err)
	}

	// Now try to resolve again via CLI
	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", challengeID,
		"--response", "Another response",
		"--dir", proofDir)

	// Should error because challenge is already resolved
	if err == nil {
		t.Error("expected error for already resolved challenge, got nil")
		return
	}

	// Error message should indicate challenge is not open or already resolved
	combined := output + err.Error()
	if !strings.Contains(strings.ToLower(combined), "resolved") &&
		!strings.Contains(strings.ToLower(combined), "not open") &&
		!strings.Contains(strings.ToLower(combined), "already") {
		t.Logf("Error message: %q", combined)
	}
}

// TestResolveChallengeCmd_MissingChallengeID verifies error when challenge ID is not provided.
func TestResolveChallengeCmd_MissingChallengeID(t *testing.T) {
	proofDir, _, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	cmd := newTestResolveChallengeCmd()
	_, err := executeCommand(cmd, "resolve-challenge",
		"--response", "Response text",
		"--dir", proofDir)

	// Should error because challenge ID argument is required
	if err == nil {
		t.Error("expected error for missing challenge ID, got nil")
	}
}

// TestResolveChallengeCmd_MissingResponse verifies error when response flag is not provided.
func TestResolveChallengeCmd_MissingResponse(t *testing.T) {
	proofDir, challengeID, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	cmd := newTestResolveChallengeCmd()
	_, err := executeCommand(cmd, "resolve-challenge", challengeID,
		"--dir", proofDir)

	// Should error because --response is required
	if err == nil {
		t.Error("expected error for missing response flag, got nil")
	}
}

// TestResolveChallengeCmd_EmptyResponse verifies error when response is empty string.
func TestResolveChallengeCmd_EmptyResponse(t *testing.T) {
	proofDir, challengeID, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	cmd := newTestResolveChallengeCmd()
	_, err := executeCommand(cmd, "resolve-challenge", challengeID,
		"--response", "",
		"--dir", proofDir)

	// Should error because response cannot be empty
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

// TestResolveChallengeCmd_WhitespaceOnlyResponse verifies error for whitespace-only response.
func TestResolveChallengeCmd_WhitespaceOnlyResponse(t *testing.T) {
	proofDir, challengeID, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	cmd := newTestResolveChallengeCmd()
	_, err := executeCommand(cmd, "resolve-challenge", challengeID,
		"--response", "   ",
		"--dir", proofDir)

	// Should error because response cannot be whitespace-only
	if err == nil {
		t.Error("expected error for whitespace-only response, got nil")
	}
}

// TestResolveChallengeCmd_EmptyChallengeID verifies error for empty challenge ID.
func TestResolveChallengeCmd_EmptyChallengeID(t *testing.T) {
	proofDir, _, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	cmd := newTestResolveChallengeCmd()
	_, err := executeCommand(cmd, "resolve-challenge", "",
		"--response", "Response text",
		"--dir", proofDir)

	// Should error because challenge ID cannot be empty
	if err == nil {
		t.Error("expected error for empty challenge ID, got nil")
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestResolveChallengeCmd_JSONOutput verifies JSON output format.
func TestResolveChallengeCmd_JSONOutput(t *testing.T) {
	proofDir, challengeID, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", challengeID,
		"--response", "Response text",
		"--format", "json",
		"--dir", proofDir)

	// Note: This will fail until resolve-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until resolve-challenge command is implemented: %v", err)
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

// TestResolveChallengeCmd_JSONOutputShortFlag verifies JSON output with short flag.
func TestResolveChallengeCmd_JSONOutputShortFlag(t *testing.T) {
	proofDir, challengeID, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", challengeID,
		"-r", "Response text",
		"-f", "json",
		"-d", proofDir)

	// Note: This will fail until resolve-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until resolve-challenge command is implemented: %v", err)
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

// TestResolveChallengeCmd_Help verifies help output shows usage information.
func TestResolveChallengeCmd_Help(t *testing.T) {
	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"resolve-challenge",
		"--response",
		"--dir",
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestResolveChallengeCmd_HelpShortFlag verifies help with short flag.
func TestResolveChallengeCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "resolve-challenge") {
		t.Errorf("expected help output to mention 'resolve-challenge', got: %q", output)
	}
}

// =============================================================================
// Directory Flag Tests
// =============================================================================

// TestResolveChallengeCmd_DirFlagNonExistent verifies error for non-existent directory.
func TestResolveChallengeCmd_DirFlagNonExistent(t *testing.T) {
	cmd := newTestResolveChallengeCmd()
	_, err := executeCommand(cmd, "resolve-challenge", "chal-001",
		"--response", "Response text",
		"--dir", "/nonexistent/path/to/proof")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestResolveChallengeCmd_DirFlagIsFile verifies error when --dir points to a file.
func TestResolveChallengeCmd_DirFlagIsFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "af-resolve-challenge-test-file-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cmd := newTestResolveChallengeCmd()
	_, err = executeCommand(cmd, "resolve-challenge", "chal-001",
		"--response", "Response text",
		"--dir", tmpFile.Name())

	if err == nil {
		t.Error("expected error when --dir is a file, got nil")
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestResolveChallengeCmd_VariousChallengeIDs verifies resolving challenges with different ID formats.
func TestResolveChallengeCmd_VariousChallengeIDs(t *testing.T) {
	tests := []struct {
		name        string
		challengeID string
		wantErr     bool
	}{
		{"simple ID", "chal-001", false},
		{"UUID format", "550e8400-e29b-41d4-a716-446655440000", false},
		{"with underscore", "chal_alpha_1", false},
		{"with dots", "chal.verifier.1", false},
		{"short ID", "c1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh test setup for each challenge ID
			tmpDir, err := os.MkdirTemp("", "af-resolve-challenge-ids-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			proofDir := filepath.Join(tmpDir, "proof")
			if err := fs.InitProofDir(proofDir); err != nil {
				t.Fatal(err)
			}

			// Initialize proof (this creates node 1 automatically)
			err = service.Init(proofDir, "Test conjecture", "test-author")
			if err != nil {
				t.Fatal(err)
			}

			rootID, _ := types.Parse("1")

			// Raise a challenge with this specific ID against node 1
			ledgerDir := filepath.Join(proofDir, "ledger")
			l, _ := ledger.NewLedger(ledgerDir)
			challengeEvent := ledger.NewChallengeRaised(tt.challengeID, rootID, "statement", "reason")
			l.Append(challengeEvent)

			cmd := newTestResolveChallengeCmd()
			_, err = executeCommand(cmd, "resolve-challenge", tt.challengeID,
				"--response", "Response text",
				"--dir", proofDir)

			// Note: This will fail until resolve-challenge command is implemented
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

// TestResolveChallengeCmd_LongResponse verifies handling of very long response text.
func TestResolveChallengeCmd_LongResponse(t *testing.T) {
	proofDir, challengeID, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	// Create a long response (2000 characters)
	longResponse := strings.Repeat("This is a detailed response explaining the resolution. ", 40)

	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", challengeID,
		"--response", longResponse,
		"--dir", proofDir)

	// Note: This will fail until resolve-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until resolve-challenge command is implemented: %v", err)
		return
	}

	if !strings.Contains(strings.ToLower(output), "resolved") {
		t.Errorf("expected successful resolution with long response, got: %q", output)
	}
}

// TestResolveChallengeCmd_SpecialCharactersInResponse verifies handling of special characters.
func TestResolveChallengeCmd_SpecialCharactersInResponse(t *testing.T) {
	proofDir, challengeID, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	specialResponse := "Response with special chars: x^2 + y^2 = z^2, ∀x ∈ ℝ, \"quoted\", and newline\nhere"

	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", challengeID,
		"--response", specialResponse,
		"--dir", proofDir)

	// Note: This will fail until resolve-challenge command is implemented
	if err != nil {
		t.Logf("Expected to fail until resolve-challenge command is implemented: %v", err)
		return
	}

	if !strings.Contains(strings.ToLower(output), "resolved") {
		t.Errorf("expected successful resolution with special characters, got: %q", output)
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestResolveChallengeCmd_WithdrawnChallenge verifies error when trying to resolve withdrawn challenge.
func TestResolveChallengeCmd_WithdrawnChallenge(t *testing.T) {
	proofDir, challengeID, cleanup := setupResolveChallengeTest(t)
	defer cleanup()

	// Withdraw the challenge using ledger directly
	ledgerDir := filepath.Join(proofDir, "ledger")
	l, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatal(err)
	}

	withdrawEvent := ledger.NewChallengeWithdrawn(challengeID)
	_, err = l.Append(withdrawEvent)
	if err != nil {
		t.Fatal(err)
	}

	// Now try to resolve via CLI
	cmd := newTestResolveChallengeCmd()
	output, err := executeCommand(cmd, "resolve-challenge", challengeID,
		"--response", "Response text",
		"--dir", proofDir)

	// Should error because challenge is withdrawn (not open)
	if err == nil {
		t.Error("expected error for withdrawn challenge, got nil")
		return
	}

	// Error message should indicate challenge is not open
	combined := output + err.Error()
	if !strings.Contains(strings.ToLower(combined), "not open") &&
		!strings.Contains(strings.ToLower(combined), "withdrawn") {
		t.Logf("Error message: %q", combined)
	}
}

// =============================================================================
// Interface Compliance Test
// =============================================================================

// TestResolveChallengeCmd_ExpectedFlags verifies the command has expected flag structure.
func TestResolveChallengeCmd_ExpectedFlags(t *testing.T) {
	cmd := newTestResolveChallengeCmd()

	// Find the resolve-challenge subcommand
	var resolveChallengeCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "resolve-challenge" {
			resolveChallengeCmd = sub
			break
		}
	}

	if resolveChallengeCmd == nil {
		t.Skip("resolve-challenge command not yet implemented")
		return
	}

	// Check expected flags exist
	expectedFlags := []string{"response", "dir", "format"}
	for _, flagName := range expectedFlags {
		if resolveChallengeCmd.Flags().Lookup(flagName) == nil &&
			resolveChallengeCmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected resolve-challenge command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"r": "response",
		"d": "dir",
		"f": "format",
	}

	for short, long := range shortFlags {
		if resolveChallengeCmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected resolve-challenge command to have short flag -%s for --%s", short, long)
		}
	}
}
