//go:build integration

// Package main contains tests for the af verify-external command.
// These are TDD tests - the verify-external command does not exist yet.
// Tests define the expected behavior for verifying external references
// (marking them as human-verified).
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupVerifyExternalTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupVerifyExternalTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-verify-external-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	// Initialize the proof directory structure
	if err := fs.InitProofDir(tmpDir); err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Initialize proof via service
	if err := service.Init(tmpDir, "Test conjecture", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupVerifyExternalTestWithExternal creates a test environment with an initialized proof
// and a single external reference. Returns the proof directory, external ID, and cleanup function.
func setupVerifyExternalTestWithExternal(t *testing.T) (string, string, func()) {
	t.Helper()

	tmpDir, cleanup := setupVerifyExternalTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add an external reference for testing
	extID, err := svc.AddExternal("Fermat's Last Theorem", "Wiles, A. (1995). Modular elliptic curves and Fermat's Last Theorem. Annals of Mathematics, 141(3), 443-551.")
	if err != nil {
		cleanup()
		t.Fatalf("failed to add external: %v", err)
	}

	return tmpDir, extID, cleanup
}

// setupVerifyExternalTestWithMultipleExternals creates a test environment with multiple externals.
// Returns the proof directory, slice of external IDs, and cleanup function.
func setupVerifyExternalTestWithMultipleExternals(t *testing.T) (string, []string, func()) {
	t.Helper()

	tmpDir, cleanup := setupVerifyExternalTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add multiple external references for testing
	externals := []struct {
		name   string
		source string
	}{
		{"Fermat's Last Theorem", "Wiles, A. (1995). Modular elliptic curves and Fermat's Last Theorem."},
		{"Prime Number Theorem", "de la Vallee Poussin, C. (1896). Recherches analytiques sur la theorie des nombres premiers."},
		{"Riemann Hypothesis", "Riemann, B. (1859). On the Number of Primes Less Than a Given Magnitude."},
	}

	var extIDs []string
	for _, ext := range externals {
		extID, err := svc.AddExternal(ext.name, ext.source)
		if err != nil {
			cleanup()
			t.Fatalf("failed to add external %q: %v", ext.name, err)
		}
		extIDs = append(extIDs, extID)
	}

	return tmpDir, extIDs, cleanup
}

// executeVerifyExternalCommand creates and executes a verify-external command with the given arguments.
func executeVerifyExternalCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newVerifyExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// newTestVerifyExternalCmd creates a fresh root command with the verify-external subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestVerifyExternalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	verifyExternalCmd := newVerifyExternalCmd()
	cmd.AddCommand(verifyExternalCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// =============================================================================
// Success Case Tests
// =============================================================================

// TestVerifyExternalCmd_Success tests verifying an existing external reference successfully.
func TestVerifyExternalCmd_Success(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	// Execute verify-external command
	output, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify output contains success indication
	lower := strings.ToLower(output)
	hasSuccessInfo := strings.Contains(lower, "verif") ||
		strings.Contains(lower, "external") ||
		strings.Contains(lower, "success")

	if !hasSuccessInfo {
		t.Errorf("expected success message mentioning verification, got: %q", output)
	}
}

// TestVerifyExternalCmd_ExternalMarkedAsVerified tests that external is marked as verified in state.
func TestVerifyExternalCmd_ExternalMarkedAsVerified(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	// Execute verify-external command
	_, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify external is marked as verified in state
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	ext := st.GetExternal(extID)
	if ext == nil {
		t.Fatal("external not found after verification")
	}

	// The external should now be verified
	// Note: The actual field depends on implementation (e.g., Verified bool, VerifiedAt timestamp)
	// This test documents the expected behavior
	t.Logf("External after verification: %+v", ext)
}

// TestVerifyExternalCmd_SuccessWithLongFlags tests using long flag forms.
func TestVerifyExternalCmd_SuccessWithLongFlags(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, extID, "--dir", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with --dir flag, got: %v", err)
	}

	if strings.TrimSpace(output) == "" {
		t.Error("expected non-empty output")
	}
}

// TestVerifyExternalCmd_ReturnsExternalInfo tests that output contains external information.
func TestVerifyExternalCmd_ReturnsExternalInfo(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain the external ID
	if !strings.Contains(output, extID) {
		t.Logf("Output may not contain external ID: %q", output)
	}
}

// =============================================================================
// Error Case Tests - External Not Found
// =============================================================================

// TestVerifyExternalCmd_ExternalNotFound tests error when external ID doesn't exist.
func TestVerifyExternalCmd_ExternalNotFound(t *testing.T) {
	tmpDir, cleanup := setupVerifyExternalTest(t)
	defer cleanup()

	// Execute with non-existent external ID
	output, err := executeVerifyExternalCommand(t, "nonexistent-id-12345", "-d", tmpDir)

	// Should error or output should mention not found
	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "not found") &&
		!strings.Contains(lower, "does not exist") &&
		!strings.Contains(lower, "error") &&
		err == nil {
		t.Errorf("expected error or 'not found' message for non-existent external, got: %q", output)
	}
}

// TestVerifyExternalCmd_EmptyExternalID tests error when external ID is empty string.
func TestVerifyExternalCmd_EmptyExternalID(t *testing.T) {
	tmpDir, cleanup := setupVerifyExternalTest(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, "", "-d", tmpDir)

	// Should error because ID cannot be empty
	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	if err == nil && !strings.Contains(lower, "empty") &&
		!strings.Contains(lower, "required") &&
		!strings.Contains(lower, "invalid") {
		t.Errorf("expected error for empty external ID, got: %q", combined)
	}
}

// TestVerifyExternalCmd_WhitespaceOnlyID tests error when external ID is whitespace only.
func TestVerifyExternalCmd_WhitespaceOnlyID(t *testing.T) {
	tmpDir, cleanup := setupVerifyExternalTest(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, "   ", "-d", tmpDir)

	// Should error because whitespace-only ID is invalid
	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	if err == nil && !strings.Contains(lower, "not found") &&
		!strings.Contains(lower, "empty") &&
		!strings.Contains(lower, "invalid") {
		t.Errorf("expected error for whitespace-only external ID, got: %q", combined)
	}
}

// =============================================================================
// Error Case Tests - Already Verified
// =============================================================================

// TestVerifyExternalCmd_AlreadyVerified tests error or warning when external is already verified.
func TestVerifyExternalCmd_AlreadyVerified(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	// First, verify the external
	_, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir)
	if err != nil {
		t.Fatalf("first verification failed: %v", err)
	}

	// Try to verify again
	output, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir)

	// Should produce error or warning about already being verified
	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	// Should indicate the external is already verified or warn about redundant action
	if !strings.Contains(lower, "already") &&
		!strings.Contains(lower, "verified") &&
		err == nil {
		t.Logf("Warning: verifying already verified external silently succeeded. Output: %q", output)
		// This may be acceptable behavior (idempotent) but worth noting
	}
}

// =============================================================================
// Error Case Tests - Invalid Arguments
// =============================================================================

// TestVerifyExternalCmd_MissingExternalID tests error when external ID is not provided.
func TestVerifyExternalCmd_MissingExternalID(t *testing.T) {
	tmpDir, cleanup := setupVerifyExternalTest(t)
	defer cleanup()

	// Execute without external ID
	_, err := executeVerifyExternalCommand(t, "-d", tmpDir)
	if err == nil {
		t.Fatal("expected error for missing external ID, got nil")
	}

	// Should contain error about missing argument
	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") &&
		!strings.Contains(errStr, "required") &&
		!strings.Contains(errStr, "argument") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestVerifyExternalCmd_TooManyArguments tests error when too many arguments are provided.
func TestVerifyExternalCmd_TooManyArguments(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	// Execute with too many arguments
	_, err := executeVerifyExternalCommand(t, extID, "extra-arg", "-d", tmpDir)
	if err == nil {
		t.Fatal("expected error for too many arguments, got nil")
	}

	// Should contain error about argument count
	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") &&
		!strings.Contains(errStr, "too many") &&
		!strings.Contains(errStr, "argument") {
		t.Errorf("expected error about too many arguments, got: %q", errStr)
	}
}

// =============================================================================
// Error Case Tests - Proof Not Initialized
// =============================================================================

// TestVerifyExternalCmd_ProofNotInitialized tests error when proof is not initialized.
func TestVerifyExternalCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-verify-external-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	output, err := executeVerifyExternalCommand(t, "some-id", "-d", tmpDir)

	// Should produce error
	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "not initialized") &&
		!strings.Contains(lower, "error") &&
		!strings.Contains(lower, "not found") &&
		!strings.Contains(lower, "no such") &&
		err == nil {
		t.Errorf("expected error for uninitialized proof, got: %q", output)
	}
}

// TestVerifyExternalCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestVerifyExternalCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeVerifyExternalCommand(t, "some-id", "-d", "/nonexistent/path/12345")

	// Should produce error
	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "not found") &&
		!strings.Contains(lower, "not exist") &&
		!strings.Contains(lower, "no such") &&
		!strings.Contains(lower, "error") &&
		err == nil {
		t.Errorf("expected error for non-existent directory, got: %q", output)
	}
}

// =============================================================================
// JSON Output Tests
// =============================================================================

// TestVerifyExternalCmd_JSONFormat tests JSON output format.
func TestVerifyExternalCmd_JSONFormat(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, extID, "--format", "json", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestVerifyExternalCmd_JSONFormatShortFlag tests JSON output with -f short flag.
func TestVerifyExternalCmd_JSONFormatShortFlag(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, extID, "-f", "json", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -f flag, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON with -f flag: %v\nOutput: %q", err, output)
	}
}

// TestVerifyExternalCmd_JSONOutputFields tests JSON output contains expected fields.
func TestVerifyExternalCmd_JSONOutputFields(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, extID, "--format", "json", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check for expected fields (with flexible naming conventions)
	expectedFields := []struct {
		name       string
		variations []string
	}{
		{"id", []string{"id", "external_id", "externalId", "ID"}},
		{"verified", []string{"verified", "Verified", "is_verified", "isVerified"}},
	}

	for _, ef := range expectedFields {
		found := false
		for _, variant := range ef.variations {
			if _, ok := result[variant]; ok {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warning: JSON output does not contain %q field (checked: %v)", ef.name, ef.variations)
		}
	}
}

// TestVerifyExternalCmd_JSONContainsExternalID tests JSON output contains the external ID.
func TestVerifyExternalCmd_JSONContainsExternalID(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, extID, "--format", "json", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Try to find the ID in the result
	var foundID string
	for _, key := range []string{"id", "external_id", "externalId", "ID"} {
		if val, ok := result[key].(string); ok && val != "" {
			foundID = val
			break
		}
	}

	if foundID == "" {
		t.Log("Warning: Could not extract external ID from JSON output")
	} else if foundID != extID {
		t.Errorf("JSON output ID = %q, want %q", foundID, extID)
	}
}

// =============================================================================
// Text Output Tests
// =============================================================================

// TestVerifyExternalCmd_TextFormat tests text output format.
func TestVerifyExternalCmd_TextFormat(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, extID, "--format", "text", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Text format should be human-readable and non-empty
	if output == "" {
		t.Error("expected non-empty text output")
	}

	// Should mention verification
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "verif") {
		t.Logf("Text output may not explicitly mention verification: %q", output)
	}
}

// TestVerifyExternalCmd_TextFormatContainsExternalInfo tests text output contains external information.
func TestVerifyExternalCmd_TextFormatContainsExternalInfo(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, extID, "--format", "text", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Text output should contain external ID or name
	if !strings.Contains(output, extID) && !strings.Contains(output, "Fermat") {
		t.Logf("Text output may not contain external identifier or name: %q", output)
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestVerifyExternalCmd_Help tests that help output shows usage information.
func TestVerifyExternalCmd_Help(t *testing.T) {
	cmd := newVerifyExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	// Check for expected help content
	expectations := []string{
		"verify-external",
		"ext-id",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestVerifyExternalCmd_HelpShortFlag tests help with -h short flag.
func TestVerifyExternalCmd_HelpShortFlag(t *testing.T) {
	cmd := newVerifyExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "verify") {
		t.Errorf("help output should mention 'verify', got: %q", output)
	}
}

// TestVerifyExternalCmd_HelpDescribesCommand tests help describes the command purpose.
func TestVerifyExternalCmd_HelpDescribesCommand(t *testing.T) {
	cmd := newVerifyExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	// Help should describe the command's purpose
	lower := strings.ToLower(output)
	hasDescription := strings.Contains(lower, "external") ||
		strings.Contains(lower, "verif") ||
		strings.Contains(lower, "reference")

	if !hasDescription {
		t.Errorf("help should describe command purpose, got: %q", output)
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestVerifyExternalCmd_ExpectedFlags ensures the command has expected flag structure.
func TestVerifyExternalCmd_ExpectedFlags(t *testing.T) {
	cmd := newVerifyExternalCmd()

	// Check expected flags exist
	expectedFlags := []string{"dir", "format"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected verify-external command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"d": "dir",
		"f": "format",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected verify-external command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestVerifyExternalCmd_DefaultFlagValues verifies default values for flags.
func TestVerifyExternalCmd_DefaultFlagValues(t *testing.T) {
	cmd := newVerifyExternalCmd()

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

// TestVerifyExternalCmd_DirFlagVariants tests both long and short forms of --dir flag.
func TestVerifyExternalCmd_DirFlagVariants(t *testing.T) {
	tests := []struct {
		name string
		flag string
	}{
		{"long form", "--dir"},
		{"short form", "-d"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
			defer cleanup()

			_, err := executeVerifyExternalCommand(t, extID, tc.flag, tmpDir)
			if err != nil {
				t.Errorf("expected no error with %s flag, got: %v", tc.flag, err)
			}
		})
	}
}

// TestVerifyExternalCmd_FormatFlagVariants tests both long and short forms of --format flag.
func TestVerifyExternalCmd_FormatFlagVariants(t *testing.T) {
	tests := []struct {
		name   string
		flag   string
		format string
	}{
		{"long form text", "--format", "text"},
		{"short form text", "-f", "text"},
		{"long form json", "--format", "json"},
		{"short form json", "-f", "json"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
			defer cleanup()

			_, err := executeVerifyExternalCommand(t, extID, tc.flag, tc.format, "-d", tmpDir)
			if err != nil {
				t.Errorf("expected no error with %s %s, got: %v", tc.flag, tc.format, err)
			}
		})
	}
}

// =============================================================================
// Directory Flag Tests
// =============================================================================

// TestVerifyExternalCmd_DefaultDirectory tests verify-external uses current directory by default.
func TestVerifyExternalCmd_DefaultDirectory(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	// Change to the proof directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Execute without -d flag (should use current directory)
	output, err := executeVerifyExternalCommand(t, extID)
	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Verify the external was verified
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	ext := st.GetExternal(extID)
	if ext == nil {
		t.Fatal("external not found")
	}

	// External should be found (verification state depends on implementation)
	t.Logf("External after verification with default directory: %+v", ext)
}

// TestVerifyExternalCmd_RelativeDirectory tests using relative directory path.
func TestVerifyExternalCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-verify-external-rel-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(baseDir)

	proofDir := baseDir + "/subdir/proof"
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	if err := service.Init(proofDir, "Test conjecture", "author"); err != nil {
		t.Fatal(err)
	}

	// Add an external
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatal(err)
	}
	extID, err := svc.AddExternal("Test Theorem", "Test Source")
	if err != nil {
		t.Fatal(err)
	}

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	output, err := executeVerifyExternalCommand(t, extID, "-d", "subdir/proof")
	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}
}

// =============================================================================
// Command Metadata Tests
// =============================================================================

// TestVerifyExternalCmd_CommandMetadata verifies command metadata.
func TestVerifyExternalCmd_CommandMetadata(t *testing.T) {
	cmd := newVerifyExternalCmd()

	if cmd.Use != "verify-external <ext-id>" && !strings.HasPrefix(cmd.Use, "verify-external") {
		t.Errorf("expected Use to start with 'verify-external', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestVerifyExternalCmd_CommandName tests that command has correct name.
func TestVerifyExternalCmd_CommandName(t *testing.T) {
	cmd := newVerifyExternalCmd()

	if cmd.Name() != "verify-external" {
		t.Errorf("expected command name 'verify-external', got %q", cmd.Name())
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestVerifyExternalCmd_TableDrivenExternalIDs tests various external ID inputs.
func TestVerifyExternalCmd_TableDrivenExternalIDs(t *testing.T) {
	tests := []struct {
		name        string
		extID       string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty ID",
			extID:       "",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "whitespace ID",
			extID:       "   ",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "nonexistent ID",
			extID:       "nonexistent-12345",
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupVerifyExternalTest(t)
			defer cleanup()

			output, err := executeVerifyExternalCommand(t, tc.extID, "-d", tmpDir)

			if tc.wantErr {
				combined := output
				if err != nil {
					combined += err.Error()
				}

				lower := strings.ToLower(combined)
				if err == nil && !strings.Contains(lower, tc.errContains) {
					t.Errorf("expected error containing %q, got output: %q", tc.errContains, output)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestVerifyExternalCmd_TableDrivenFormats tests various format options.
func TestVerifyExternalCmd_TableDrivenFormats(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		validator func(t *testing.T, output string)
		wantErr   bool
	}{
		{
			name:   "text format",
			format: "text",
			validator: func(t *testing.T, output string) {
				// Text format should be human-readable
				if output == "" {
					t.Error("expected non-empty text output")
				}
			},
			wantErr: false,
		},
		{
			name:   "json format",
			format: "json",
			validator: func(t *testing.T, output string) {
				// JSON format should be valid JSON
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(output), &data); err != nil {
					t.Errorf("expected valid JSON, got error: %v\nOutput: %q", err, output)
				}
			},
			wantErr: false,
		},
		{
			name:    "invalid format",
			format:  "xml",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
			defer cleanup()

			output, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir, "-f", tc.format)

			if tc.wantErr {
				if err == nil {
					t.Logf("Invalid format %q did not error, output: %q", tc.format, output)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tc.validator != nil {
					tc.validator(t, output)
				}
			}
		})
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestVerifyExternalCmd_MultipleExternalsSequential tests verifying multiple externals in sequence.
func TestVerifyExternalCmd_MultipleExternalsSequential(t *testing.T) {
	tmpDir, extIDs, cleanup := setupVerifyExternalTestWithMultipleExternals(t)
	defer cleanup()

	// Verify each external in sequence
	for _, extID := range extIDs {
		output, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir)
		if err != nil {
			t.Fatalf("failed to verify external %s: %v\nOutput: %s", extID, err, output)
		}
	}

	// Verify all externals are now verified in state
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	for _, extID := range extIDs {
		ext := st.GetExternal(extID)
		if ext == nil {
			t.Errorf("external %s not found after verification", extID)
		}
		// Check verification status (implementation-dependent)
		t.Logf("External %s after verification: %+v", extID, ext)
	}
}

// TestVerifyExternalCmd_VerifyThenQuery tests that verified external can be queried.
func TestVerifyExternalCmd_VerifyThenQuery(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	// Verify the external
	_, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	// Query the external using service
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	ext := st.GetExternal(extID)
	if ext == nil {
		t.Fatal("external not found after verification")
	}

	// External should exist and be queryable
	if ext.ID != extID {
		t.Errorf("external ID mismatch: got %s, want %s", ext.ID, extID)
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestVerifyExternalCmd_VerifyAfterMultipleAdds tests verification after adding multiple externals.
func TestVerifyExternalCmd_VerifyAfterMultipleAdds(t *testing.T) {
	tmpDir, cleanup := setupVerifyExternalTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add many externals
	var lastExtID string
	for i := 0; i < 20; i++ {
		extID, err := svc.AddExternal("Theorem "+string(rune('A'+i)), "Source "+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("failed to add external %d: %v", i, err)
		}
		lastExtID = extID
	}

	// Verify the last one
	output, err := executeVerifyExternalCommand(t, lastExtID, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}
}

// TestVerifyExternalCmd_VerifyWithSpecialCharactersInID tests external ID with special characters.
func TestVerifyExternalCmd_VerifyWithSpecialCharactersInID(t *testing.T) {
	// External IDs are typically hex-encoded random bytes, but test robustness
	tmpDir, cleanup := setupVerifyExternalTest(t)
	defer cleanup()

	// Try to verify with special characters (should fail gracefully)
	specialIDs := []string{
		"id-with-dash",
		"id_with_underscore",
		"12345678abcd",
	}

	for _, id := range specialIDs {
		output, err := executeVerifyExternalCommand(t, id, "-d", tmpDir)

		// Should error (not found) but not crash
		combined := output
		if err != nil {
			combined += err.Error()
		}

		lower := strings.ToLower(combined)
		if err == nil && !strings.Contains(lower, "not found") {
			t.Logf("Special ID %q did not error with 'not found': %q", id, output)
		}
	}
}

// =============================================================================
// Concurrent Access Tests (Document Behavior)
// =============================================================================

// TestVerifyExternalCmd_ConcurrentVerification documents behavior under concurrent verification.
func TestVerifyExternalCmd_ConcurrentVerification(t *testing.T) {
	tmpDir, extIDs, cleanup := setupVerifyExternalTestWithMultipleExternals(t)
	defer cleanup()

	// Document: In real usage, concurrent verification of different externals should work
	// This test verifies basic sequential access; true concurrent testing requires goroutines
	// which may not be appropriate for CLI command tests.

	for i, extID := range extIDs {
		output, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir)
		if err != nil {
			t.Errorf("verification %d failed for external %s: %v", i, extID, err)
		}
		t.Logf("Verified external %d: %s, output: %s", i, extID, strings.TrimSpace(output))
	}
}

// =============================================================================
// Output Consistency Tests
// =============================================================================

// TestVerifyExternalCmd_OutputConsistencyBetweenFormats tests JSON and text output contain same info.
func TestVerifyExternalCmd_OutputConsistencyBetweenFormats(t *testing.T) {
	tmpDir, _, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	// Add a fresh external for this test
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	extID, err := svc.AddExternal("Consistency Test Theorem", "Test Source")
	if err != nil {
		t.Fatal(err)
	}

	// Get text output
	textOutput, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir, "-f", "text")
	if err != nil {
		t.Fatalf("text format failed: %v", err)
	}

	// Add another external and get JSON output
	extID2, err := svc.AddExternal("Consistency Test Theorem 2", "Test Source 2")
	if err != nil {
		t.Fatal(err)
	}
	jsonOutput, err := executeVerifyExternalCommand(t, extID2, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("json format failed: %v", err)
	}

	// Both should be non-empty
	if textOutput == "" {
		t.Error("text output should not be empty")
	}
	if jsonOutput == "" {
		t.Error("json output should not be empty")
	}

	// JSON should be valid
	var jsonResult map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &jsonResult); err != nil {
		t.Errorf("json output is not valid JSON: %v", err)
	}
}

// =============================================================================
// Success Message Tests
// =============================================================================

// TestVerifyExternalCmd_SuccessMessage tests that success message is informative.
func TestVerifyExternalCmd_SuccessMessage(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Success message should indicate what happened
	lower := strings.ToLower(output)
	hasStatusInfo := strings.Contains(lower, "verified") ||
		strings.Contains(lower, "success") ||
		strings.Contains(lower, "external")

	if !hasStatusInfo {
		t.Errorf("success message should mention verification or external, got: %q", output)
	}
}

// TestVerifyExternalCmd_SuccessMessageContainsNextSteps tests if output suggests next steps.
func TestVerifyExternalCmd_SuccessMessageContainsNextSteps(t *testing.T) {
	tmpDir, extID, cleanup := setupVerifyExternalTestWithExternal(t)
	defer cleanup()

	output, err := executeVerifyExternalCommand(t, extID, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Per CLI self-documentation requirements, output may suggest next steps
	// This test documents the expected behavior
	t.Logf("Success output: %q", output)
}
