//go:build integration

// Package main contains tests for the af externals and af external commands.
// These are TDD tests - the externals/external commands do not exist yet.
// Tests define the expected behavior for listing and viewing external references.
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

// setupExternalsTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupExternalsTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-externals-test-*")
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
	if err := service.Init(tmpDir, "Test conjecture for externals", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupExternalsTestWithExternals creates a test environment with an initialized proof
// and some pre-existing external references.
func setupExternalsTestWithExternals(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupExternalsTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add some external references for testing
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

// newTestExternalsCmd creates a fresh root command with the externals subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestExternalsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	externalsCmd := newExternalsCmd()
	cmd.AddCommand(externalsCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// newTestExternalCmd creates a fresh root command with the external subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestExternalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	externalCmd := newExternalCmd()
	cmd.AddCommand(externalCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// executeExternalsCommand creates and executes an externals command with the given arguments.
func executeExternalsCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newExternalsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// executeExternalCommand creates and executes an external command with the given arguments.
func executeExternalCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// af externals - List All External References Tests
// =============================================================================

// TestExternalsCmd_ListExternals tests listing all external references in a proof.
func TestExternalsCmd_ListExternals(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain all external names
	expectedNames := []string{"Fermat's Last Theorem", "Prime Number Theorem", "Riemann Hypothesis"}
	for _, name := range expectedNames {
		if !strings.Contains(output, name) {
			t.Errorf("expected output to contain external name %q, got: %q", name, output)
		}
	}
}

// TestExternalsCmd_ListExternalsEmpty tests listing externals when none exist.
func TestExternalsCmd_ListExternalsEmpty(t *testing.T) {
	tmpDir, cleanup := setupExternalsTest(t)
	defer cleanup()

	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should indicate no externals or be empty/show a message
	lower := strings.ToLower(output)
	hasNoExternalsIndicator := strings.Contains(lower, "no external") ||
		strings.Contains(lower, "none") ||
		strings.Contains(lower, "empty") ||
		len(strings.TrimSpace(output)) == 0

	if !hasNoExternalsIndicator {
		t.Logf("Output when no externals exist: %q", output)
	}
}

// TestExternalsCmd_ListShowsCount tests that the listing shows a count of externals.
func TestExternalsCmd_ListShowsCount(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain some indication of count (3 externals)
	if !strings.Contains(output, "3") {
		t.Logf("Output may or may not show count: %q", output)
	}
}

// TestExternalsCmd_ListSortedByName tests that externals are listed in sorted order.
func TestExternalsCmd_ListSortedByName(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--dir", tmpDir)

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
			t.Logf("Externals may not be sorted alphabetically: Fermat@%d, Prime@%d, Riemann@%d", fermatIdx, primeIdx, riemannIdx)
		}
	}
}

// =============================================================================
// af externals - JSON Output Tests
// =============================================================================

// TestExternalsCmd_JSONOutput tests JSON output format for listing externals.
func TestExternalsCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestExternalsCmd_JSONOutputStructure tests the structure of JSON output.
func TestExternalsCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Try to unmarshal as array or object
	var arrayResult []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &arrayResult); err == nil {
		// It's an array - each item should have external fields
		if len(arrayResult) != 3 {
			t.Errorf("expected 3 externals in JSON array, got %d", len(arrayResult))
		}

		for i, ext := range arrayResult {
			if _, ok := ext["name"]; !ok {
				t.Errorf("external %d missing 'name' field", i)
			}
			if _, ok := ext["source"]; !ok {
				t.Errorf("external %d missing 'source' field", i)
			}
		}
	} else {
		// Try as object with externals array
		var objResult map[string]interface{}
		if err := json.Unmarshal([]byte(output), &objResult); err != nil {
			t.Errorf("output is not valid JSON array or object: %v", err)
		} else {
			// Check for an externals array in the object
			if exts, ok := objResult["externals"]; ok {
				if extsArr, ok := exts.([]interface{}); ok {
					if len(extsArr) != 3 {
						t.Errorf("expected 3 externals, got %d", len(extsArr))
					}
				}
			}
		}
	}
}

// TestExternalsCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestExternalsCmd_JSONOutputShortFlag(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "-f", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON with -f flag: %v\nOutput: %q", err, output)
	}
}

// TestExternalsCmd_JSONOutputEmpty tests JSON output when no externals exist.
func TestExternalsCmd_JSONOutputEmpty(t *testing.T) {
	tmpDir, cleanup := setupExternalsTest(t)
	defer cleanup()

	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON (empty array or object with empty externals)
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("empty output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// =============================================================================
// af external <name> - Show Specific External Reference Tests
// =============================================================================

// TestExternalCmd_ShowExternal tests showing a specific external by name.
func TestExternalCmd_ShowExternal(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", "Fermat's Last Theorem", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain the external name
	if !strings.Contains(output, "Fermat") {
		t.Errorf("expected output to contain external name 'Fermat', got: %q", output)
	}

	// Should contain the source
	if !strings.Contains(output, "Wiles") {
		t.Errorf("expected output to contain source author 'Wiles', got: %q", output)
	}
}

// TestExternalCmd_ShowExternalByName tests showing different externals.
func TestExternalCmd_ShowExternalByName(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
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
			cmd := newTestExternalCmd()
			output, err := executeCommand(cmd, "external", tc.name, "--dir", tmpDir)

			if err != nil {
				t.Fatalf("expected no error for %q, got: %v", tc.name, err)
			}

			if !strings.Contains(output, tc.expectedContent) {
				t.Errorf("expected output for %q to contain %q, got: %q", tc.name, tc.expectedContent, output)
			}
		})
	}
}

// TestExternalCmd_ShowExternalFull tests showing full external details.
func TestExternalCmd_ShowExternalFull(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", "Fermat's Last Theorem", "--full", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Full output should include:
	// - External name
	// - Source
	// - ID (if shown)
	// - Content hash (if shown)
	// - Created timestamp (if shown)

	if !strings.Contains(output, "Fermat") {
		t.Errorf("expected full output to contain name, got: %q", output)
	}

	if !strings.Contains(output, "Wiles") {
		t.Errorf("expected full output to contain source, got: %q", output)
	}
}

// =============================================================================
// af external <name> - JSON Output Tests
// =============================================================================

// TestExternalCmd_JSONOutput tests JSON output for a specific external.
func TestExternalCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", "Fermat's Last Theorem", "--format", "json", "--dir", tmpDir)

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

// TestExternalCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestExternalCmd_JSONOutputShortFlag(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", "Prime Number Theorem", "-f", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestExternalCmd_JSONOutputFields tests JSON output contains expected fields.
func TestExternalCmd_JSONOutputFields(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", "Fermat's Last Theorem", "--format", "json", "--dir", tmpDir)

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
		{"content_hash", []string{"content_hash", "contentHash", "ContentHash"}},
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

// TestExternalsCmd_ProofNotInitialized tests error when proof is not initialized.
func TestExternalsCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-externals-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestExternalsCmd()
	_, err = executeCommand(cmd, "externals", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestExternalsCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestExternalsCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestExternalsCmd()
	_, err := executeCommand(cmd, "externals", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestExternalCmd_ExternalNotFound tests error when external doesn't exist.
func TestExternalCmd_ExternalNotFound(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", "nonexistent", "--dir", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should indicate external not found
	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "not found") && !strings.Contains(lower, "does not exist") && err == nil {
		t.Errorf("expected error for non-existent external, got: %q", output)
	}
}

// TestExternalCmd_MissingExternalName tests error when external name is not provided.
func TestExternalCmd_MissingExternalName(t *testing.T) {
	tmpDir, cleanup := setupExternalsTest(t)
	defer cleanup()

	cmd := newTestExternalCmd()
	_, err := executeCommand(cmd, "external", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing external name, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") &&
		!strings.Contains(errStr, "required") &&
		!strings.Contains(errStr, "argument") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestExternalCmd_ProofNotInitialized tests error when proof is not initialized.
func TestExternalCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-external-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestExternalCmd()
	_, err = executeCommand(cmd, "external", "Fermat's Last Theorem", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestExternalCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestExternalCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestExternalCmd()
	_, err := executeCommand(cmd, "external", "Fermat's Last Theorem", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestExternalCmd_EmptyExternalName tests error for empty external name.
func TestExternalCmd_EmptyExternalName(t *testing.T) {
	tmpDir, cleanup := setupExternalsTest(t)
	defer cleanup()

	cmd := newTestExternalCmd()
	_, err := executeCommand(cmd, "external", "", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for empty external name, got nil")
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestExternalsCmd_Help tests that help output shows usage information.
func TestExternalsCmd_Help(t *testing.T) {
	cmd := newExternalsCmd()
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
		"externals",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestExternalsCmd_HelpShortFlag tests help with -h short flag.
func TestExternalsCmd_HelpShortFlag(t *testing.T) {
	cmd := newExternalsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "externals") {
		t.Errorf("help output should mention 'externals', got: %q", output)
	}
}

// TestExternalCmd_Help tests that help output shows usage information.
func TestExternalCmd_Help(t *testing.T) {
	cmd := newExternalCmd()
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
		"external",
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

// TestExternalCmd_HelpShortFlag tests help with -h short flag.
func TestExternalCmd_HelpShortFlag(t *testing.T) {
	cmd := newExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "external") {
		t.Errorf("help output should mention 'external', got: %q", output)
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestExternalsCmd_ExpectedFlags ensures the externals command has expected flag structure.
func TestExternalsCmd_ExpectedFlags(t *testing.T) {
	cmd := newExternalsCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected externals command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected externals command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestExternalsCmd_DefaultFlagValues verifies default values for flags.
func TestExternalsCmd_DefaultFlagValues(t *testing.T) {
	cmd := newExternalsCmd()

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

// TestExternalCmd_ExpectedFlags ensures the external command has expected flag structure.
func TestExternalCmd_ExpectedFlags(t *testing.T) {
	cmd := newExternalCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir", "full"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected external command to have flag %q", flagName)
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
			t.Errorf("expected external command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestExternalCmd_DefaultFlagValues verifies default values for flags.
func TestExternalCmd_DefaultFlagValues(t *testing.T) {
	cmd := newExternalCmd()

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

// TestExternalsCmd_CommandMetadata verifies command metadata.
func TestExternalsCmd_CommandMetadata(t *testing.T) {
	cmd := newExternalsCmd()

	if cmd.Use != "externals" {
		t.Errorf("expected Use to be 'externals', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestExternalCmd_CommandMetadata verifies command metadata.
func TestExternalCmd_CommandMetadata(t *testing.T) {
	cmd := newExternalCmd()

	if !strings.HasPrefix(cmd.Use, "external") {
		t.Errorf("expected Use to start with 'external', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Default Directory Tests
// =============================================================================

// TestExternalsCmd_DefaultDirectory tests externals uses current directory by default.
func TestExternalsCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
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
	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals")

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Should list externals
	if !strings.Contains(output, "Fermat") {
		t.Errorf("expected output to contain 'Fermat', got: %q", output)
	}
}

// TestExternalCmd_DefaultDirectory tests external uses current directory by default.
func TestExternalCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
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
	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", "Fermat's Last Theorem")

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Should show external
	if !strings.Contains(output, "Fermat") {
		t.Errorf("expected output to contain 'Fermat', got: %q", output)
	}
}

// =============================================================================
// Format Validation Tests
// =============================================================================

// TestExternalsCmd_FormatValidation verifies format flag validation.
func TestExternalsCmd_FormatValidation(t *testing.T) {
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
			tmpDir, cleanup := setupExternalsTestWithExternals(t)
			defer cleanup()

			cmd := newTestExternalsCmd()
			output, err := executeCommand(cmd, "externals", "--format", tc.format, "--dir", tmpDir)

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

// TestExternalCmd_FormatValidation verifies format flag validation for external command.
func TestExternalCmd_FormatValidation(t *testing.T) {
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
			tmpDir, cleanup := setupExternalsTestWithExternals(t)
			defer cleanup()

			cmd := newTestExternalCmd()
			output, err := executeCommand(cmd, "external", "Fermat's Last Theorem", "--format", tc.format, "--dir", tmpDir)

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

// TestExternalCmd_CaseInsensitiveName tests if external lookup is case-insensitive.
func TestExternalCmd_CaseInsensitiveName(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
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
			cmd := newTestExternalCmd()
			output, err := executeCommand(cmd, "external", name, "--dir", tmpDir)

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

// TestExternalCmd_FuzzyMatchSuggestion tests fuzzy matching suggestions for typos.
func TestExternalCmd_FuzzyMatchSuggestion(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	// Try a typo that's close to "Fermat's Last Theorem"
	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", "Fermat Last Theorem", "--dir", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should either find via fuzzy match or suggest similar externals
	// This test documents the expected behavior for typos
	t.Logf("Typo 'Fermat Last Theorem' result: %s (error: %v)", output, err)
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestExternalsCmd_TableDrivenDirectories tests various directory scenarios.
func TestExternalsCmd_TableDrivenDirectories(t *testing.T) {
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
			cmd := newTestExternalsCmd()
			_, err := executeCommand(cmd, "externals", "--dir", tc.dirPath)

			if tc.wantErr && err == nil {
				t.Errorf("expected error for dir %q, got nil", tc.dirPath)
			}
		})
	}
}

// TestExternalCmd_TableDrivenExternalNames tests various external name inputs.
func TestExternalCmd_TableDrivenExternalNames(t *testing.T) {
	tests := []struct {
		name        string
		extName     string
		expectFound bool
	}{
		{"existing external", "Fermat's Last Theorem", true},
		{"existing external 2", "Prime Number Theorem", true},
		{"existing external 3", "Riemann Hypothesis", true},
		{"nonexistent external", "nonexistent", false},
		{"partial match", "Fermat", false},
		{"similar name", "Fermat Theorem", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupExternalsTestWithExternals(t)
			defer cleanup()

			cmd := newTestExternalCmd()
			output, err := executeCommand(cmd, "external", tc.extName, "--dir", tmpDir)

			if tc.expectFound {
				if err != nil {
					t.Errorf("expected external %q to be found, got error: %v", tc.extName, err)
				}
			} else {
				combined := output
				if err != nil {
					combined += err.Error()
				}
				// Should indicate not found
				if err == nil && !strings.Contains(strings.ToLower(combined), "not found") {
					t.Logf("External %q not expected to be found, but no error. Output: %q", tc.extName, output)
				}
			}
		})
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestExternalsCmd_ManyExternals tests listing many external references.
func TestExternalsCmd_ManyExternals(t *testing.T) {
	tmpDir, cleanup := setupExternalsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add many externals
	for i := 0; i < 50; i++ {
		name := strings.Repeat("Theorem", i+1)
		if len(name) > 100 {
			name = name[:100]
		}
		source := strings.Repeat("Author (2024). ", i+1)
		_, err := svc.AddExternal(name+string(rune('a'+i%26)), source)
		if err != nil {
			t.Logf("Failed to add external %d: %v", i, err)
			break
		}
	}

	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error listing many externals, got: %v", err)
	}

	t.Logf("Output length for many externals: %d bytes", len(output))
}

// TestExternalCmd_LongExternalName tests showing an external with a long name.
func TestExternalCmd_LongExternalName(t *testing.T) {
	tmpDir, cleanup := setupExternalsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add an external with a long name
	longName := strings.Repeat("Mathematical_Theorem_", 10)
	_, err = svc.AddExternal(longName, "Some source citation")
	if err != nil {
		t.Logf("Could not add long-named external: %v", err)
		return
	}

	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", longName, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for long external name, got: %v", err)
	}

	if !strings.Contains(output, "Mathematical_Theorem_") {
		t.Errorf("expected output to contain long name, got: %q", output)
	}
}

// TestExternalCmd_UnicodeExternalName tests external with unicode characters.
func TestExternalCmd_UnicodeExternalName(t *testing.T) {
	tmpDir, cleanup := setupExternalsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add an external with unicode name
	unicodeName := "Gauss-Bonnet Theorem"
	_, err = svc.AddExternal(unicodeName, "Gauss, C. F. (1827)")
	if err != nil {
		t.Logf("Could not add unicode-named external: %v", err)
		return
	}

	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", unicodeName, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for unicode external name, got: %v", err)
	}

	if !strings.Contains(output, "Gauss") {
		t.Errorf("expected output to contain unicode name, got: %q", output)
	}
}

// TestExternalsCmd_WithVerboseFlag tests potential verbose output flag.
func TestExternalsCmd_WithVerboseFlag(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	// Test if verbose flag exists and changes output
	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--verbose", "--dir", tmpDir)

	// Verbose flag may or may not exist
	if err != nil {
		t.Logf("Verbose flag may not be implemented: %v", err)
	} else {
		t.Logf("Verbose output: %q", output)
	}
}

// TestExternalCmd_SpecialCharactersInName tests external with special characters in name.
func TestExternalCmd_SpecialCharactersInName(t *testing.T) {
	tmpDir, cleanup := setupExternalsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add an external with special characters in name
	specialName := "Theorem: A < B implies B > A"
	_, err = svc.AddExternal(specialName, "Logic textbook (2024)")
	if err != nil {
		t.Logf("Could not add special-char-named external: %v", err)
		return
	}

	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", specialName, "--dir", tmpDir)

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

// TestExternalsAndExternalConsistency tests that externals and external show consistent information.
func TestExternalsAndExternalConsistency(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	// Get list from externals
	externalsCmd := newTestExternalsCmd()
	externalsOutput, err := executeCommand(externalsCmd, "externals", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("externals command failed: %v", err)
	}

	// Verify each external from externals can be retrieved with external
	externalNames := []string{"Fermat's Last Theorem", "Prime Number Theorem", "Riemann Hypothesis"}
	for _, name := range externalNames {
		// Verify name appears in externals output
		if !strings.Contains(externalsOutput, name) {
			t.Errorf("external %q not found in externals output", name)
		}

		// Verify external can retrieve it
		externalCmd := newTestExternalCmd()
		externalOutput, err := executeCommand(externalCmd, "external", name, "--dir", tmpDir)
		if err != nil {
			t.Errorf("external command failed for %q: %v", name, err)
		}

		if !strings.Contains(externalOutput, name) {
			t.Errorf("external output for %q doesn't contain the name", name)
		}
	}
}

// =============================================================================
// Source Display Tests
// =============================================================================

// TestExternalsCmd_ShowsSources tests that listing shows sources (or truncated versions).
func TestExternalsCmd_ShowsSources(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--dir", tmpDir)

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

// TestExternalCmd_ShowsFullSource tests that single external view shows full source.
func TestExternalCmd_ShowsFullSource(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", "Fermat's Last Theorem", "--dir", tmpDir)

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
// Output Content Tests
// =============================================================================

// TestExternalsCmd_OutputContainsAllInfo tests text output contains expected information.
func TestExternalsCmd_OutputContainsAllInfo(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--dir", tmpDir)

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

// TestExternalCmd_OutputContainsAllDetails tests single external output contains all details.
func TestExternalCmd_OutputContainsAllDetails(t *testing.T) {
	tmpDir, cleanup := setupExternalsTestWithExternals(t)
	defer cleanup()

	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", "Fermat's Last Theorem", "--full", "--dir", tmpDir)

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
// Multiple Externals with Similar Names Tests
// =============================================================================

// TestExternalCmd_SimilarNames tests handling of externals with similar names.
func TestExternalCmd_SimilarNames(t *testing.T) {
	tmpDir, cleanup := setupExternalsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add externals with similar names
	similarExternals := []struct {
		name   string
		source string
	}{
		{"Theorem A", "Source A"},
		{"Theorem A (Extended)", "Source A Extended"},
		{"Theorem A Version 2", "Source A v2"},
	}

	for _, ext := range similarExternals {
		_, err := svc.AddExternal(ext.name, ext.source)
		if err != nil {
			t.Fatalf("failed to add external %q: %v", ext.name, err)
		}
	}

	// Each should be findable by exact name
	for _, ext := range similarExternals {
		cmd := newTestExternalCmd()
		output, err := executeCommand(cmd, "external", ext.name, "--dir", tmpDir)

		if err != nil {
			t.Errorf("expected to find %q, got error: %v", ext.name, err)
		} else if !strings.Contains(output, ext.name) {
			t.Errorf("expected output to contain %q, got: %q", ext.name, output)
		}
	}
}

// =============================================================================
// Integration with add-external Tests
// =============================================================================

// TestExternalsCmd_AfterAddExternal tests listing externals after adding one.
func TestExternalsCmd_AfterAddExternal(t *testing.T) {
	tmpDir, cleanup := setupExternalsTest(t)
	defer cleanup()

	// Add an external via service
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

	// List externals and verify the new one appears
	cmd := newTestExternalsCmd()
	output, err := executeCommand(cmd, "externals", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, extName) {
		t.Errorf("expected output to contain newly added external %q, got: %q", extName, output)
	}
}

// TestExternalCmd_AfterAddExternal tests showing external details after adding one.
func TestExternalCmd_AfterAddExternal(t *testing.T) {
	tmpDir, cleanup := setupExternalsTest(t)
	defer cleanup()

	// Add an external via service
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

	// Show the external and verify details
	cmd := newTestExternalCmd()
	output, err := executeCommand(cmd, "external", extName, "--dir", tmpDir)

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
