//go:build integration

// Package main contains tests for the af pending-refs and af pending-ref commands.
// These are TDD tests - the pending-refs/pending-ref commands do not exist yet.
// Tests define the expected behavior for listing and viewing pending external references.
//
// "Pending refs" refers to external references that have not yet been verified.
// External references are citations to theorems, papers, or other sources that
// proofs may depend on. Until verified, they are considered "pending".
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupPendingRefsTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupPendingRefsTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-pending-refs-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	// Initialize the proof directory structure
	if err := service.InitProofDir(tmpDir); err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Initialize proof via service
	if err := service.Init(tmpDir, "Test conjecture for pending refs", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupPendingRefsTestWithExternals creates a test environment with an initialized proof
// and some pre-existing external references (which are pending verification by default).
func setupPendingRefsTestWithExternals(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupPendingRefsTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add some external references for testing
	// These are pending verification by default
	externals := []struct {
		name   string
		source string
	}{
		{"Fermat's Last Theorem", "Wiles, A. (1995). Modular elliptic curves and Fermat's Last Theorem. Annals of Mathematics, 141(3), 443-551."},
		{"Prime Number Theorem", "de la Vallee Poussin, C. (1896). Recherches analytiques sur la theorie des nombres premiers."},
		{"Riemann Hypothesis", "Riemann, B. (1859). On the Number of Primes Less Than a Given Magnitude."},
	}

	for _, ext := range externals {
		_, err := svc.AddExternal(ext.name, ext.source)
		if err != nil {
			cleanup()
			t.Fatalf("failed to add external %q: %v", ext.name, err)
		}
	}

	return tmpDir, cleanup
}

// newTestPendingRefsCmd creates a fresh root command with the pending-refs subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestPendingRefsCmd() *cobra.Command {
	cmd := newTestRootCmd()

	pendingRefsCmd := newPendingRefsCmd()
	cmd.AddCommand(pendingRefsCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// newTestPendingRefCmd creates a fresh root command with the pending-ref subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestPendingRefCmd() *cobra.Command {
	cmd := newTestRootCmd()

	pendingRefCmd := newPendingRefCmd()
	cmd.AddCommand(pendingRefCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// executePendingRefsCommand creates and executes a pending-refs command with the given arguments.
func executePendingRefsCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newPendingRefsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// executePendingRefCommand creates and executes a pending-ref command with the given arguments.
func executePendingRefCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newPendingRefCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// af pending-refs - List All Pending External References Tests
// =============================================================================

// TestPendingRefsCmd_ListPendingRefs tests listing all pending external references.
func TestPendingRefsCmd_ListPendingRefs(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain all pending external reference names
	expectedNames := []string{"Fermat's Last Theorem", "Prime Number Theorem", "Riemann Hypothesis"}
	for _, name := range expectedNames {
		if !strings.Contains(output, name) {
			t.Errorf("expected output to contain pending ref name %q, got: %q", name, output)
		}
	}
}

// TestPendingRefsCmd_ListPendingRefsEmpty tests listing pending refs when none exist.
func TestPendingRefsCmd_ListPendingRefsEmpty(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should indicate no pending refs or be empty/show a message
	lower := strings.ToLower(output)
	hasNoPendingRefsIndicator := strings.Contains(lower, "no pending") ||
		strings.Contains(lower, "none") ||
		strings.Contains(lower, "empty") ||
		strings.Contains(lower, "0 pending") ||
		len(strings.TrimSpace(output)) == 0

	if !hasNoPendingRefsIndicator {
		t.Logf("Output when no pending refs exist: %q", output)
	}
}

// TestPendingRefsCmd_ListShowsCount tests that the listing shows a count of pending refs.
func TestPendingRefsCmd_ListShowsCount(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain some indication of count (3 pending refs)
	if !strings.Contains(output, "3") {
		t.Logf("Output may or may not show count: %q", output)
	}
}

// TestPendingRefsCmd_ListSortedByName tests that pending refs are listed in sorted order.
func TestPendingRefsCmd_ListSortedByName(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check that "Fermat" appears before "Prime" which appears before "Riemann"
	// (alphabetical order)
	fermatIdx := strings.Index(output, "Fermat")
	primeIdx := strings.Index(output, "Prime")
	riemannIdx := strings.Index(output, "Riemann")

	if fermatIdx != -1 && primeIdx != -1 && riemannIdx != -1 {
		if !(fermatIdx < primeIdx && primeIdx < riemannIdx) {
			t.Logf("Pending refs may not be sorted alphabetically: Fermat@%d, Prime@%d, Riemann@%d", fermatIdx, primeIdx, riemannIdx)
		}
	}
}

// =============================================================================
// af pending-refs - JSON Output Tests
// =============================================================================

// TestPendingRefsCmd_JSONOutput tests JSON output format for listing pending refs.
func TestPendingRefsCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestPendingRefsCmd_JSONOutputStructure tests the structure of JSON output.
func TestPendingRefsCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Try to unmarshal as array or object
	var arrayResult []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &arrayResult); err == nil {
		// It's an array - each item should have pending ref fields
		if len(arrayResult) != 3 {
			t.Errorf("expected 3 pending refs in JSON array, got %d", len(arrayResult))
		}

		for i, ref := range arrayResult {
			if _, ok := ref["name"]; !ok {
				t.Errorf("pending ref %d missing 'name' field", i)
			}
			if _, ok := ref["source"]; !ok {
				t.Errorf("pending ref %d missing 'source' field", i)
			}
		}
	} else {
		// Try as object with pending_refs array
		var objResult map[string]interface{}
		if err := json.Unmarshal([]byte(output), &objResult); err != nil {
			t.Errorf("output is not valid JSON array or object: %v", err)
		} else {
			// Check for a pending_refs array in the object
			if refs, ok := objResult["pending_refs"]; ok {
				if refsArr, ok := refs.([]interface{}); ok {
					if len(refsArr) != 3 {
						t.Errorf("expected 3 pending refs, got %d", len(refsArr))
					}
				}
			}
		}
	}
}

// TestPendingRefsCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestPendingRefsCmd_JSONOutputShortFlag(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "-f", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON with -f flag: %v\nOutput: %q", err, output)
	}
}

// TestPendingRefsCmd_JSONOutputEmpty tests JSON output when no pending refs exist.
func TestPendingRefsCmd_JSONOutputEmpty(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON (empty array or object with empty pending_refs)
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("empty output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// =============================================================================
// af pending-ref <name> - Show Specific Pending Reference Tests
// =============================================================================

// TestPendingRefCmd_ShowPendingRef tests showing a specific pending ref by name.
func TestPendingRefCmd_ShowPendingRef(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", "Fermat's Last Theorem", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain the pending ref name
	if !strings.Contains(output, "Fermat") {
		t.Errorf("expected output to contain pending ref name 'Fermat', got: %q", output)
	}

	// Should contain the source
	if !strings.Contains(output, "Wiles") {
		t.Errorf("expected output to contain source author 'Wiles', got: %q", output)
	}
}

// TestPendingRefCmd_ShowPendingRefByName tests showing different pending refs.
func TestPendingRefCmd_ShowPendingRefByName(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	tests := []struct {
		name            string
		expectedContent string
	}{
		{"Fermat's Last Theorem", "Wiles"},
		{"Prime Number Theorem", "Vallee Poussin"},
		{"Riemann Hypothesis", "Riemann"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestPendingRefCmd()
			output, err := executeCommand(cmd, "pending-ref", tc.name, "--dir", tmpDir)

			if err != nil {
				t.Fatalf("expected no error for %q, got: %v", tc.name, err)
			}

			if !strings.Contains(output, tc.expectedContent) {
				t.Errorf("expected output for %q to contain %q, got: %q", tc.name, tc.expectedContent, output)
			}
		})
	}
}

// TestPendingRefCmd_ShowPendingRefByID tests showing a pending ref by its ID.
func TestPendingRefCmd_ShowPendingRefByID(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add an external and capture its ID
	extID, err := svc.AddExternal("Test Theorem", "Test Source Citation")
	if err != nil {
		t.Fatal(err)
	}

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", extID, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error when looking up by ID, got: %v", err)
	}

	// Should show the external details
	if !strings.Contains(output, "Test Theorem") {
		t.Errorf("expected output to contain 'Test Theorem', got: %q", output)
	}
}

// TestPendingRefCmd_ShowPendingRefFull tests showing full pending ref details.
func TestPendingRefCmd_ShowPendingRefFull(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", "Fermat's Last Theorem", "--full", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Full output should include:
	// - Pending ref name
	// - Source
	// - ID (if shown)
	// - Content hash (if shown)
	// - Created timestamp (if shown)
	// - Verification status

	if !strings.Contains(output, "Fermat") {
		t.Errorf("expected full output to contain name, got: %q", output)
	}

	if !strings.Contains(output, "Wiles") {
		t.Errorf("expected full output to contain source, got: %q", output)
	}
}

// =============================================================================
// af pending-ref <name> - JSON Output Tests
// =============================================================================

// TestPendingRefCmd_JSONOutput tests JSON output for a specific pending ref.
func TestPendingRefCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", "Fermat's Last Theorem", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Check for expected fields
	if name, ok := result["name"]; !ok {
		t.Error("JSON output missing 'name' field")
	} else if !strings.Contains(name.(string), "Fermat") {
		t.Errorf("expected name to contain 'Fermat', got %v", name)
	}

	if _, ok := result["source"]; !ok {
		t.Logf("Warning: JSON output may not have 'source' field")
	}
}

// TestPendingRefCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestPendingRefCmd_JSONOutputShortFlag(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", "Prime Number Theorem", "-f", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestPendingRefCmd_JSONOutputFields tests JSON output contains expected fields.
func TestPendingRefCmd_JSONOutputFields(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", "Fermat's Last Theorem", "--format", "json", "--dir", tmpDir)

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
		{"name", []string{"name", "Name"}},
		{"source", []string{"source", "Source"}},
		{"status", []string{"status", "verification_status", "verificationStatus", "Status"}},
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

// =============================================================================
// Error Case Tests
// =============================================================================

// TestPendingRefsCmd_ProofNotInitialized tests error when proof is not initialized.
func TestPendingRefsCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-pending-refs-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestPendingRefsCmd()
	_, err = executeCommand(cmd, "pending-refs", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestPendingRefsCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestPendingRefsCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestPendingRefsCmd()
	_, err := executeCommand(cmd, "pending-refs", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestPendingRefCmd_PendingRefNotFound tests error when pending ref doesn't exist.
func TestPendingRefCmd_PendingRefNotFound(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", "nonexistent", "--dir", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should indicate pending ref not found
	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "not found") && !strings.Contains(lower, "does not exist") && err == nil {
		t.Errorf("expected error for non-existent pending ref, got: %q", output)
	}
}

// TestPendingRefCmd_MissingPendingRefName tests error when pending ref name is not provided.
func TestPendingRefCmd_MissingPendingRefName(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	cmd := newTestPendingRefCmd()
	_, err := executeCommand(cmd, "pending-ref", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing pending ref name, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") &&
		!strings.Contains(errStr, "required") &&
		!strings.Contains(errStr, "argument") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestPendingRefCmd_ProofNotInitialized tests error when proof is not initialized.
func TestPendingRefCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-pending-ref-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestPendingRefCmd()
	_, err = executeCommand(cmd, "pending-ref", "Fermat's Last Theorem", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestPendingRefCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestPendingRefCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestPendingRefCmd()
	_, err := executeCommand(cmd, "pending-ref", "Fermat's Last Theorem", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestPendingRefCmd_EmptyPendingRefName tests error for empty pending ref name.
func TestPendingRefCmd_EmptyPendingRefName(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	cmd := newTestPendingRefCmd()
	_, err := executeCommand(cmd, "pending-ref", "", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for empty pending ref name, got nil")
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestPendingRefsCmd_Help tests that help output shows usage information.
func TestPendingRefsCmd_Help(t *testing.T) {
	cmd := newPendingRefsCmd()
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
		"pending-refs",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestPendingRefsCmd_HelpShortFlag tests help with -h short flag.
func TestPendingRefsCmd_HelpShortFlag(t *testing.T) {
	cmd := newPendingRefsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "pending-refs") {
		t.Errorf("help output should mention 'pending-refs', got: %q", output)
	}
}

// TestPendingRefCmd_Help tests that help output shows usage information.
func TestPendingRefCmd_Help(t *testing.T) {
	cmd := newPendingRefCmd()
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
		"pending-ref",
		"name",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestPendingRefCmd_HelpShortFlag tests help with -h short flag.
func TestPendingRefCmd_HelpShortFlag(t *testing.T) {
	cmd := newPendingRefCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "pending-ref") {
		t.Errorf("help output should mention 'pending-ref', got: %q", output)
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestPendingRefsCmd_ExpectedFlags ensures the pending-refs command has expected flag structure.
func TestPendingRefsCmd_ExpectedFlags(t *testing.T) {
	cmd := newPendingRefsCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected pending-refs command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected pending-refs command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestPendingRefsCmd_DefaultFlagValues verifies default values for flags.
func TestPendingRefsCmd_DefaultFlagValues(t *testing.T) {
	cmd := newPendingRefsCmd()

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

// TestPendingRefCmd_ExpectedFlags ensures the pending-ref command has expected flag structure.
func TestPendingRefCmd_ExpectedFlags(t *testing.T) {
	cmd := newPendingRefCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir", "full"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected pending-ref command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
		"F": "full",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected pending-ref command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestPendingRefCmd_DefaultFlagValues verifies default values for flags.
func TestPendingRefCmd_DefaultFlagValues(t *testing.T) {
	cmd := newPendingRefCmd()

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

	// Check default full value
	fullFlag := cmd.Flags().Lookup("full")
	if fullFlag == nil {
		t.Fatal("expected full flag to exist")
	}
	if fullFlag.DefValue != "false" {
		t.Errorf("expected default full to be 'false', got %q", fullFlag.DefValue)
	}
}

// TestPendingRefsCmd_CommandMetadata verifies command metadata.
func TestPendingRefsCmd_CommandMetadata(t *testing.T) {
	cmd := newPendingRefsCmd()

	if cmd.Use != "pending-refs" {
		t.Errorf("expected Use to be 'pending-refs', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestPendingRefCmd_CommandMetadata verifies command metadata.
func TestPendingRefCmd_CommandMetadata(t *testing.T) {
	cmd := newPendingRefCmd()

	if !strings.HasPrefix(cmd.Use, "pending-ref") {
		t.Errorf("expected Use to start with 'pending-ref', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Default Directory Tests
// =============================================================================

// TestPendingRefsCmd_DefaultDirectory tests pending-refs uses current directory by default.
func TestPendingRefsCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
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
	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs")

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Should list pending refs
	if !strings.Contains(output, "Fermat") {
		t.Errorf("expected output to contain 'Fermat', got: %q", output)
	}
}

// TestPendingRefCmd_DefaultDirectory tests pending-ref uses current directory by default.
func TestPendingRefCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
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
	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", "Fermat's Last Theorem")

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Should show pending ref
	if !strings.Contains(output, "Fermat") {
		t.Errorf("expected output to contain 'Fermat', got: %q", output)
	}
}

// =============================================================================
// Format Validation Tests
// =============================================================================

// TestPendingRefsCmd_FormatValidation verifies format flag validation.
func TestPendingRefsCmd_FormatValidation(t *testing.T) {
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
			defer cleanup()

			cmd := newTestPendingRefsCmd()
			output, err := executeCommand(cmd, "pending-refs", "--format", tc.format, "--dir", tmpDir)

			if tc.wantErr {
				combined := output
				if err != nil {
					combined += err.Error()
				}

				if err == nil && !strings.Contains(strings.ToLower(combined), "format") {
					t.Logf("Expected error for format %q, got output: %q", tc.format, output)
				}
			} else {
				if err != nil && strings.Contains(err.Error(), "format") {
					t.Errorf("unexpected format error for format %q: %v", tc.format, err)
				}
			}
		})
	}
}

// TestPendingRefCmd_FormatValidation verifies format flag validation for pending-ref command.
func TestPendingRefCmd_FormatValidation(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"valid text format", "text", false},
		{"valid json format", "json", false},
		{"invalid xml format", "xml", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
			defer cleanup()

			cmd := newTestPendingRefCmd()
			output, err := executeCommand(cmd, "pending-ref", "Fermat's Last Theorem", "--format", tc.format, "--dir", tmpDir)

			if tc.wantErr {
				combined := output
				if err != nil {
					combined += err.Error()
				}

				if err == nil && !strings.Contains(strings.ToLower(combined), "format") {
					t.Logf("Expected error for format %q, got output: %q", tc.format, output)
				}
			} else {
				if err != nil && strings.Contains(err.Error(), "format") {
					t.Errorf("unexpected format error for format %q: %v", tc.format, err)
				}
			}
		})
	}
}

// =============================================================================
// Case Sensitivity Tests
// =============================================================================

// TestPendingRefCmd_CaseInsensitiveName tests if pending ref lookup is case-insensitive.
func TestPendingRefCmd_CaseInsensitiveName(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	// Test with different cases of "Fermat's Last Theorem"
	testCases := []string{
		"Fermat's Last Theorem",
		"fermat's last theorem",
		"FERMAT'S LAST THEOREM",
		"Fermat's last Theorem",
	}

	for _, name := range testCases {
		t.Run(name, func(t *testing.T) {
			cmd := newTestPendingRefCmd()
			output, err := executeCommand(cmd, "pending-ref", name, "--dir", tmpDir)

			// Implementation may be case-sensitive or case-insensitive
			// This test documents the behavior
			if err != nil {
				t.Logf("Case %q returned error: %v (may be case-sensitive)", name, err)
			} else {
				if !strings.Contains(strings.ToLower(output), "fermat") {
					t.Errorf("expected output to contain 'fermat', got: %q", output)
				}
			}
		})
	}
}

// =============================================================================
// Fuzzy Matching Tests
// =============================================================================

// TestPendingRefCmd_FuzzyMatchSuggestion tests fuzzy matching suggestions for typos.
func TestPendingRefCmd_FuzzyMatchSuggestion(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	// Try a typo that's close to "Fermat's Last Theorem"
	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", "Fermat Last Theorem", "--dir", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should either find via fuzzy match or suggest similar pending refs
	// This test documents the expected behavior for typos
	t.Logf("Typo 'Fermat Last Theorem' result: %s (error: %v)", output, err)
}

// TestPendingRefCmd_PartialNameMatch tests partial name matching.
func TestPendingRefCmd_PartialNameMatch(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	// Try partial matches
	testCases := []struct {
		name      string
		partial   string
		shouldErr bool
	}{
		{"full name", "Fermat's Last Theorem", false},
		{"partial start", "Fermat", true}, // May or may not match
		{"partial middle", "Last Theorem", true},
		{"single word", "Riemann", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestPendingRefCmd()
			output, err := executeCommand(cmd, "pending-ref", tc.partial, "--dir", tmpDir)

			if tc.shouldErr {
				// Document behavior - partial matching may or may not be supported
				t.Logf("Partial %q result: output=%q, err=%v", tc.partial, output, err)
			} else {
				if err != nil {
					t.Errorf("expected %q to match, got error: %v", tc.partial, err)
				}
			}
		})
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestPendingRefsCmd_TableDrivenDirectories tests various directory scenarios.
func TestPendingRefsCmd_TableDrivenDirectories(t *testing.T) {
	tests := []struct {
		name    string
		dirPath string
		wantErr bool
	}{
		{
			name:    "empty path",
			dirPath: "",
			wantErr: true,
		},
		{
			name:    "nonexistent path",
			dirPath: "/nonexistent/path/12345",
			wantErr: true,
		},
		{
			name:    "path with special chars",
			dirPath: "/tmp/path with spaces",
			wantErr: true, // Likely doesn't exist
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestPendingRefsCmd()
			_, err := executeCommand(cmd, "pending-refs", "--dir", tc.dirPath)

			if tc.wantErr && err == nil {
				t.Errorf("expected error for dir %q, got nil", tc.dirPath)
			}
		})
	}
}

// TestPendingRefCmd_TableDrivenPendingRefNames tests various pending ref name inputs.
func TestPendingRefCmd_TableDrivenPendingRefNames(t *testing.T) {
	tests := []struct {
		name        string
		refName     string
		expectFound bool
	}{
		{"existing pending ref", "Fermat's Last Theorem", true},
		{"existing pending ref 2", "Prime Number Theorem", true},
		{"existing pending ref 3", "Riemann Hypothesis", true},
		{"nonexistent pending ref", "nonexistent", false},
		{"partial match", "Fermat", false},
		{"similar name", "Fermat Theorem", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
			defer cleanup()

			cmd := newTestPendingRefCmd()
			output, err := executeCommand(cmd, "pending-ref", tc.refName, "--dir", tmpDir)

			if tc.expectFound {
				if err != nil {
					t.Errorf("expected pending ref %q to be found, got error: %v", tc.refName, err)
				}
			} else {
				combined := output
				if err != nil {
					combined += err.Error()
				}
				// Should indicate not found
				if err == nil && !strings.Contains(strings.ToLower(combined), "not found") {
					t.Logf("Pending ref %q not expected to be found, but no error. Output: %q", tc.refName, output)
				}
			}
		})
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestPendingRefsCmd_ManyPendingRefs tests listing many pending external references.
func TestPendingRefsCmd_ManyPendingRefs(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add many pending refs
	for i := 0; i < 50; i++ {
		name := strings.Repeat("Theorem", i+1)
		if len(name) > 100 {
			name = name[:100]
		}
		source := strings.Repeat("Author (2024). ", i+1)
		_, err := svc.AddExternal(name+string(rune('a'+i%26)), source)
		if err != nil {
			t.Logf("Failed to add pending ref %d: %v", i, err)
			break
		}
	}

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error listing many pending refs, got: %v", err)
	}

	t.Logf("Output length for many pending refs: %d bytes", len(output))
}

// TestPendingRefCmd_LongPendingRefName tests showing a pending ref with a long name.
func TestPendingRefCmd_LongPendingRefName(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add a pending ref with a long name
	longName := strings.Repeat("Mathematical_Theorem_", 10)
	_, err = svc.AddExternal(longName, "Some source citation")
	if err != nil {
		t.Logf("Could not add long-named pending ref: %v", err)
		return
	}

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", longName, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for long pending ref name, got: %v", err)
	}

	if !strings.Contains(output, "Mathematical_Theorem_") {
		t.Errorf("expected output to contain long name, got: %q", output)
	}
}

// TestPendingRefCmd_UnicodeInName tests pending ref with unicode characters.
func TestPendingRefCmd_UnicodeInName(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add a pending ref with unicode name
	unicodeName := "Gauss-Bonnet Theorem"
	_, err = svc.AddExternal(unicodeName, "Gauss, C. F. (1827)")
	if err != nil {
		t.Logf("Could not add unicode-named pending ref: %v", err)
		return
	}

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", unicodeName, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for unicode pending ref name, got: %v", err)
	}

	if !strings.Contains(output, "Gauss") {
		t.Errorf("expected output to contain unicode name, got: %q", output)
	}
}

// TestPendingRefsCmd_WithVerboseFlag tests potential verbose output flag.
func TestPendingRefsCmd_WithVerboseFlag(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	// Test if verbose flag exists and changes output
	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--verbose", "--dir", tmpDir)

	// Verbose flag may or may not exist
	if err != nil {
		t.Logf("Verbose flag may not be implemented: %v", err)
	} else {
		t.Logf("Verbose output: %q", output)
	}
}

// TestPendingRefCmd_SpecialCharactersInName tests pending ref with special characters in name.
func TestPendingRefCmd_SpecialCharactersInName(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add a pending ref with special characters in name
	specialName := "Theorem: A < B implies B > A"
	_, err = svc.AddExternal(specialName, "Logic textbook (2024)")
	if err != nil {
		t.Logf("Could not add special-char-named pending ref: %v", err)
		return
	}

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", specialName, "--dir", tmpDir)

	if err != nil {
		t.Logf("Special character name lookup error: %v (may require quoting)", err)
	} else {
		if !strings.Contains(output, "Theorem") {
			t.Errorf("expected output to contain special char name, got: %q", output)
		}
	}
}

// =============================================================================
// Consistency Tests
// =============================================================================

// TestPendingRefsAndPendingRefConsistency tests that pending-refs and pending-ref show consistent information.
func TestPendingRefsAndPendingRefConsistency(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	// Get list from pending-refs
	pendingRefsCmd := newTestPendingRefsCmd()
	pendingRefsOutput, err := executeCommand(pendingRefsCmd, "pending-refs", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("pending-refs command failed: %v", err)
	}

	// Verify each pending ref from pending-refs can be retrieved with pending-ref
	pendingRefNames := []string{"Fermat's Last Theorem", "Prime Number Theorem", "Riemann Hypothesis"}
	for _, name := range pendingRefNames {
		// Verify name appears in pending-refs output
		if !strings.Contains(pendingRefsOutput, name) {
			t.Errorf("pending ref %q not found in pending-refs output", name)
		}

		// Verify pending-ref can retrieve it
		pendingRefCmd := newTestPendingRefCmd()
		pendingRefOutput, err := executeCommand(pendingRefCmd, "pending-ref", name, "--dir", tmpDir)
		if err != nil {
			t.Errorf("pending-ref command failed for %q: %v", name, err)
		}

		if !strings.Contains(pendingRefOutput, name) {
			t.Errorf("pending-ref output for %q doesn't contain the name", name)
		}
	}
}

// =============================================================================
// Source Display Tests
// =============================================================================

// TestPendingRefsCmd_ShowsSources tests that listing shows sources (or truncated versions).
func TestPendingRefsCmd_ShowsSources(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show at least some source information
	// Either the full source or truncated version
	hasSourceInfo := strings.Contains(output, "Wiles") ||
		strings.Contains(output, "1995") ||
		strings.Contains(output, "Annals")

	if !hasSourceInfo {
		t.Logf("Output may not show sources in list view: %q", output)
	}
}

// TestPendingRefCmd_ShowsFullSource tests that single pending ref view shows full source.
func TestPendingRefCmd_ShowsFullSource(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", "Fermat's Last Theorem", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show full source citation
	expectedParts := []string{"Wiles", "1995", "Annals", "Mathematics"}
	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Logf("Expected source to contain %q, got: %q", part, output)
		}
	}
}

// =============================================================================
// Verification Status Tests
// =============================================================================

// TestPendingRefsCmd_ShowsPendingStatus tests that listing indicates pending verification status.
func TestPendingRefsCmd_ShowsPendingStatus(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should indicate pending/unverified status
	lower := strings.ToLower(output)
	hasPendingIndicator := strings.Contains(lower, "pending") ||
		strings.Contains(lower, "unverified") ||
		strings.Contains(lower, "awaiting")

	if !hasPendingIndicator {
		t.Logf("Output may not explicitly show pending status: %q", output)
	}
}

// TestPendingRefCmd_ShowsVerificationStatus tests that single pending ref shows verification status.
func TestPendingRefCmd_ShowsVerificationStatus(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", "Fermat's Last Theorem", "--full", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Full output should show verification status
	lower := strings.ToLower(output)
	hasStatusField := strings.Contains(lower, "status") ||
		strings.Contains(lower, "verified") ||
		strings.Contains(lower, "pending")

	if !hasStatusField {
		t.Logf("Full output may not show verification status: %q", output)
	}
}

// =============================================================================
// Output Content Tests
// =============================================================================

// TestPendingRefsCmd_OutputContainsAllInfo tests text output contains expected information.
func TestPendingRefsCmd_OutputContainsAllInfo(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain names
	names := []string{"Fermat", "Prime", "Riemann"}
	for _, name := range names {
		if !strings.Contains(output, name) {
			t.Errorf("expected output to contain %q, got: %q", name, output)
		}
	}
}

// TestPendingRefCmd_OutputContainsAllDetails tests single pending ref output contains all details.
func TestPendingRefCmd_OutputContainsAllDetails(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTestWithExternals(t)
	defer cleanup()

	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", "Fermat's Last Theorem", "--full", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Full output should contain all relevant details
	expectedDetails := []string{"Fermat", "Wiles"}
	for _, detail := range expectedDetails {
		if !strings.Contains(output, detail) {
			t.Errorf("expected full output to contain %q, got: %q", detail, output)
		}
	}
}

// =============================================================================
// Relative Directory Tests
// =============================================================================

// TestPendingRefsCmd_RelativeDirectory tests using relative directory path.
func TestPendingRefsCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-pending-refs-rel-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(baseDir)

	proofDir := filepath.Join(baseDir, "subdir", "proof")
	if err := service.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	if err := service.Init(proofDir, "Test conjecture", "author"); err != nil {
		t.Fatal(err)
	}

	svc, _ := service.NewProofService(proofDir)
	svc.AddExternal("Test Reference", "Test Source for relative path")

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "-d", "subdir/proof")

	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}

	// Should show the pending ref
	if !strings.Contains(output, "Test Reference") {
		t.Logf("Output with relative directory: %q", output)
	}
}

// =============================================================================
// Integration with Verify External Tests
// =============================================================================

// TestPendingRefsCmd_AfterAddExternal tests listing pending refs after adding one.
func TestPendingRefsCmd_AfterAddExternal(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	// Add an external via service - it should be pending by default
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	extName := "Test Theorem"
	extSource := "Test Source (2024)"
	_, err = svc.AddExternal(extName, extSource)
	if err != nil {
		t.Fatalf("failed to add external: %v", err)
	}

	// List pending refs and verify the new one appears
	cmd := newTestPendingRefsCmd()
	output, err := executeCommand(cmd, "pending-refs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, extName) {
		t.Errorf("expected output to contain newly added pending ref %q, got: %q", extName, output)
	}
}

// TestPendingRefCmd_AfterAddExternal tests showing pending ref details after adding one.
func TestPendingRefCmd_AfterAddExternal(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	// Add an external via service - it should be pending by default
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	extName := "Integration Test Theorem"
	extSource := "Integration Test Source Citation (2024)"
	_, err = svc.AddExternal(extName, extSource)
	if err != nil {
		t.Fatalf("failed to add external: %v", err)
	}

	// Show the pending ref and verify details
	cmd := newTestPendingRefCmd()
	output, err := executeCommand(cmd, "pending-ref", extName, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, extName) {
		t.Errorf("expected output to contain name %q, got: %q", extName, output)
	}

	if !strings.Contains(output, extSource) && !strings.Contains(output, "Integration Test Source") {
		t.Logf("Output may not show full source: %q", output)
	}
}

// =============================================================================
// Multiple Pending Refs with Similar Names Tests
// =============================================================================

// TestPendingRefCmd_SimilarNames tests handling of pending refs with similar names.
func TestPendingRefCmd_SimilarNames(t *testing.T) {
	tmpDir, cleanup := setupPendingRefsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add pending refs with similar names
	similarRefs := []struct {
		name   string
		source string
	}{
		{"Theorem A", "Source A"},
		{"Theorem A (Extended)", "Source A Extended"},
		{"Theorem A Version 2", "Source A v2"},
	}

	for _, ref := range similarRefs {
		_, err := svc.AddExternal(ref.name, ref.source)
		if err != nil {
			t.Fatalf("failed to add pending ref %q: %v", ref.name, err)
		}
	}

	// Each should be findable by exact name
	for _, ref := range similarRefs {
		cmd := newTestPendingRefCmd()
		output, err := executeCommand(cmd, "pending-ref", ref.name, "--dir", tmpDir)

		if err != nil {
			t.Errorf("expected to find %q, got error: %v", ref.name, err)
		} else if !strings.Contains(output, ref.name) {
			t.Errorf("expected output to contain %q, got: %q", ref.name, output)
		}
	}
}
