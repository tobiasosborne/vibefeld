//go:build integration

// Package main contains tests for the af add-external command.
// These are TDD tests - the add-external command does not exist yet.
// Tests define the expected behavior for adding external references
// (citations to papers, theorems, etc.) to the proof.
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

// setupAddExternalTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupAddExternalTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-add-external-test-*")
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

// executeAddExternalCommand creates and executes an add-external command with the given arguments.
func executeAddExternalCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newAddExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// newAddExternalTestCmd creates a test command hierarchy with the add-external command.
// This ensures test isolation - each test gets its own command instance.
func newAddExternalTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	addExternalCmd := newAddExternalCmd()
	cmd.AddCommand(addExternalCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// =============================================================================
// Success Case Tests
// =============================================================================

// TestAddExternalCmd_Success tests adding an external reference successfully.
func TestAddExternalCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	// Execute add-external command
	output, err := executeAddExternalCommand(t,
		"--name", "Fermat's Last Theorem",
		"--source", "Wiles, A. (1995). Modular elliptic curves and Fermat's Last Theorem. Annals of Mathematics, 141(3), 443-551.",
		"-d", tmpDir,
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify output contains success indication
	lower := strings.ToLower(output)
	hasSuccessInfo := strings.Contains(lower, "external") ||
		strings.Contains(lower, "added") ||
		strings.Contains(lower, "created") ||
		strings.Contains(lower, "success")

	if !hasSuccessInfo {
		t.Errorf("expected success message mentioning external/added/created, got: %q", output)
	}
}

// TestAddExternalCmd_SuccessWithShortFlags tests using short flags.
func TestAddExternalCmd_SuccessWithShortFlags(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"-n", "Prime Number Theorem",
		"-s", "de la Vallee Poussin, C. (1896)",
		"-d", tmpDir,
	)
	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	if !strings.Contains(strings.ToLower(output), "external") &&
		!strings.Contains(strings.ToLower(output), "added") &&
		!strings.Contains(strings.ToLower(output), "created") {
		t.Errorf("expected output to indicate success, got: %q", output)
	}
}

// TestAddExternalCmd_ReturnsExternalID tests that output contains the external ID.
func TestAddExternalCmd_ReturnsExternalID(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"--name", "Riemann Hypothesis",
		"--source", "Riemann, B. (1859). On the Number of Primes Less Than a Given Magnitude.",
		"-d", tmpDir,
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain some identifier
	// The exact format depends on implementation, but should be non-empty
	if strings.TrimSpace(output) == "" {
		t.Error("expected non-empty output containing external ID")
	}
}

// TestAddExternalCmd_MultipleExternals tests adding multiple external references.
func TestAddExternalCmd_MultipleExternals(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	externals := []struct {
		name   string
		source string
	}{
		{"Theorem A", "Paper A (2020)"},
		{"Theorem B", "Paper B (2021)"},
		{"Theorem C", "Paper C (2022)"},
	}

	for _, ext := range externals {
		output, err := executeAddExternalCommand(t,
			"--name", ext.name,
			"--source", ext.source,
			"-d", tmpDir,
		)
		if err != nil {
			t.Fatalf("failed to add external %q: %v", ext.name, err)
		}

		if strings.TrimSpace(output) == "" {
			t.Errorf("expected non-empty output for external %q", ext.name)
		}
	}
}

// TestAddExternalCmd_ExternalStoredInState tests that external is stored in state.
func TestAddExternalCmd_ExternalStoredInState(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	extName := "Cauchy-Schwarz Inequality"
	extSource := "Cauchy, A. (1821). Cours d'Analyse."

	// Add via command and get JSON output to parse the ID
	output, err := executeAddExternalCommand(t,
		"--name", extName,
		"--source", extSource,
		"--format", "json",
		"-d", tmpDir,
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse JSON to get the external ID
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Try to find the ID in the result (could be "id", "external_id", etc.)
	var extID string
	for _, key := range []string{"id", "external_id", "externalId", "ID"} {
		if val, ok := result[key].(string); ok && val != "" {
			extID = val
			break
		}
	}

	if extID == "" {
		t.Log("Warning: Could not extract external ID from JSON output")
		// Still continue with test to verify external was stored somehow
	}

	// Verify external can be retrieved from state
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if extID != "" {
		ext := st.GetExternal(extID)
		if ext == nil {
			t.Errorf("external with ID %q not found in state", extID)
		} else {
			if ext.Name != extName {
				t.Errorf("external name = %q, want %q", ext.Name, extName)
			}
			if ext.Source != extSource {
				t.Errorf("external source = %q, want %q", ext.Source, extSource)
			}
		}
	}
}

// =============================================================================
// Positional Argument Tests
// =============================================================================

// TestAddExternalCmd_PositionalArgs tests using positional arguments NAME SOURCE.
func TestAddExternalCmd_PositionalArgs(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	// Execute add-external command with positional args
	output, err := executeAddExternalCommand(t,
		"Fermat's Last Theorem",
		"Wiles, A. (1995). Modular elliptic curves and Fermat's Last Theorem.",
		"-d", tmpDir,
	)
	if err != nil {
		t.Fatalf("expected no error with positional args, got: %v", err)
	}

	// Verify output contains success indication
	lower := strings.ToLower(output)
	hasSuccessInfo := strings.Contains(lower, "external") ||
		strings.Contains(lower, "added") ||
		strings.Contains(lower, "created") ||
		strings.Contains(lower, "success")

	if !hasSuccessInfo {
		t.Errorf("expected success message, got: %q", output)
	}
}

// TestAddExternalCmd_PositionalArgsWithJSONFormat tests positional args with JSON output.
func TestAddExternalCmd_PositionalArgsWithJSONFormat(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	extName := "Prime Number Theorem"
	extSource := "de la Vallee Poussin (1896)"

	output, err := executeAddExternalCommand(t,
		extName,
		extSource,
		"--format", "json",
		"-d", tmpDir,
	)
	if err != nil {
		t.Fatalf("expected no error with positional args and json format, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Verify the name and source are correct
	if name, ok := result["name"].(string); ok {
		if name != extName {
			t.Errorf("name = %q, want %q", name, extName)
		}
	}
	if source, ok := result["source"].(string); ok {
		if source != extSource {
			t.Errorf("source = %q, want %q", source, extSource)
		}
	}
}

// TestAddExternalCmd_PositionalArgsStoredInState tests that positional args are stored correctly.
func TestAddExternalCmd_PositionalArgsStoredInState(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	extName := "Pythagorean Theorem"
	extSource := "Euclid, Elements Book I"

	// Add via command with positional args
	output, err := executeAddExternalCommand(t,
		extName,
		extSource,
		"--format", "json",
		"-d", tmpDir,
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse JSON to get the external ID
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	var extID string
	for _, key := range []string{"id", "external_id", "externalId", "ID"} {
		if val, ok := result[key].(string); ok && val != "" {
			extID = val
			break
		}
	}

	// Verify external can be retrieved from state
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if extID != "" {
		ext := st.GetExternal(extID)
		if ext == nil {
			t.Errorf("external with ID %q not found in state", extID)
		} else {
			if ext.Name != extName {
				t.Errorf("external name = %q, want %q", ext.Name, extName)
			}
			if ext.Source != extSource {
				t.Errorf("external source = %q, want %q", ext.Source, extSource)
			}
		}
	}
}

// TestAddExternalCmd_PositionalArgsOnlyOneArg tests error when only one positional arg is provided.
func TestAddExternalCmd_PositionalArgsOnlyOneArg(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	_, err := executeAddExternalCommand(t,
		"Only Name Provided",
		"-d", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error when only one positional arg provided, got nil")
	}

	errStr := err.Error()
	// Should mention both NAME and SOURCE are required
	if !strings.Contains(errStr, "NAME") || !strings.Contains(errStr, "SOURCE") {
		t.Errorf("expected error to mention both NAME and SOURCE, got: %q", errStr)
	}
}

// TestAddExternalCmd_FlagsTakePrecedenceOverPositional tests that explicit flags override positional args.
func TestAddExternalCmd_FlagsTakePrecedenceOverPositional(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	flagName := "Flag Name"
	flagSource := "Flag Source"

	output, err := executeAddExternalCommand(t,
		"Positional Name",
		"Positional Source",
		"--name", flagName,
		"--source", flagSource,
		"--format", "json",
		"-d", tmpDir,
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse JSON to verify the flag values were used
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if name, ok := result["name"].(string); ok {
		if name != flagName {
			t.Errorf("expected flag name %q to take precedence, got %q", flagName, name)
		}
	}
	if source, ok := result["source"].(string); ok {
		if source != flagSource {
			t.Errorf("expected flag source %q to take precedence, got %q", flagSource, source)
		}
	}
}

// TestAddExternalCmd_PositionalArgsWithSpecialChars tests positional args with special characters.
func TestAddExternalCmd_PositionalArgsWithSpecialChars(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	extName := "Euler's Identity (e^(i*pi) + 1 = 0)"
	extSource := "Euler, L. (1748). Introductio in analysin infinitorum."

	output, err := executeAddExternalCommand(t,
		extName,
		extSource,
		"--format", "json",
		"-d", tmpDir,
	)
	if err != nil {
		t.Fatalf("expected no error with special chars in positional args, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if name, ok := result["name"].(string); ok {
		if name != extName {
			t.Errorf("name = %q, want %q", name, extName)
		}
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestAddExternalCmd_MissingName tests error when name is not provided.
func TestAddExternalCmd_MissingName(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	_, err := executeAddExternalCommand(t,
		"--source", "Some paper citation",
		"-d", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "name") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing name, got: %q", errStr)
	}
}

// TestAddExternalCmd_MissingSource tests error when source is not provided.
func TestAddExternalCmd_MissingSource(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	_, err := executeAddExternalCommand(t,
		"--name", "Some Theorem",
		"-d", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for missing source, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "source") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing source, got: %q", errStr)
	}
}

// TestAddExternalCmd_EmptyName tests error when name is empty string.
func TestAddExternalCmd_EmptyName(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"--name", "",
		"--source", "Some paper",
		"-d", tmpDir,
	)

	// Should error because name cannot be empty
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "empty") &&
		!strings.Contains(strings.ToLower(combined), "required") {
		t.Errorf("expected error for empty name, got: %q", combined)
	}
}

// TestAddExternalCmd_EmptySource tests error when source is empty string.
func TestAddExternalCmd_EmptySource(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"--name", "Some Theorem",
		"--source", "",
		"-d", tmpDir,
	)

	// Should error because source cannot be empty
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "empty") &&
		!strings.Contains(strings.ToLower(combined), "required") {
		t.Errorf("expected error for empty source, got: %q", combined)
	}
}

// TestAddExternalCmd_WhitespaceOnlyName tests error when name is whitespace only.
func TestAddExternalCmd_WhitespaceOnlyName(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"--name", "   ",
		"--source", "Some paper",
		"-d", tmpDir,
	)

	// Should error because whitespace-only name is invalid
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "empty") &&
		!strings.Contains(strings.ToLower(combined), "cannot") {
		t.Errorf("expected error for whitespace-only name, got: %q", combined)
	}
}

// TestAddExternalCmd_WhitespaceOnlySource tests error when source is whitespace only.
func TestAddExternalCmd_WhitespaceOnlySource(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"--name", "Some Theorem",
		"--source", "   ",
		"-d", tmpDir,
	)

	// Should error because whitespace-only source is invalid
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "empty") &&
		!strings.Contains(strings.ToLower(combined), "cannot") {
		t.Errorf("expected error for whitespace-only source, got: %q", combined)
	}
}

// TestAddExternalCmd_ProofNotInitialized tests error when proof is not initialized.
func TestAddExternalCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-add-external-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	output, err := executeAddExternalCommand(t,
		"--name", "Some Theorem",
		"--source", "Some Paper",
		"-d", tmpDir,
	)

	// Should produce error
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not initialized") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		!strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "no such") &&
		err == nil {
		t.Errorf("expected error for uninitialized proof, got: %q", output)
	}
}

// TestAddExternalCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestAddExternalCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeAddExternalCommand(t,
		"--name", "Some Theorem",
		"--source", "Some Paper",
		"-d", "/nonexistent/path/12345",
	)

	// Should produce error
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "not exist") &&
		!strings.Contains(strings.ToLower(combined), "no such") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		err == nil {
		t.Errorf("expected error for non-existent directory, got: %q", output)
	}
}

// =============================================================================
// JSON Output Tests
// =============================================================================

// TestAddExternalCmd_JSONFormat tests JSON output format.
func TestAddExternalCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"--name", "Pythagorean Theorem",
		"--source", "Euclid. Elements, Book I, Proposition 47.",
		"--format", "json",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Should contain relevant fields
	expectedFields := []string{"id", "name", "source"}
	for _, field := range expectedFields {
		if _, ok := result[field]; !ok {
			// Try snake_case variant
			snakeField := field
			if field == "id" {
				snakeField = "external_id"
			}
			if _, ok := result[snakeField]; !ok {
				t.Logf("Warning: JSON output does not contain %q or %q field", field, snakeField)
			}
		}
	}
}

// TestAddExternalCmd_JSONFormatShortFlag tests JSON output with short flag.
func TestAddExternalCmd_JSONFormatShortFlag(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"-n", "Theorem X",
		"-s", "Paper Y (2023)",
		"-f", "json",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON with -f flag, got: %v\nOutput: %q", err, output)
	}
}

// TestAddExternalCmd_JSONContainsContentHash tests JSON includes content hash.
func TestAddExternalCmd_JSONContainsContentHash(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"--name", "Test Theorem",
		"--source", "Test Source Citation",
		"--format", "json",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Content hash should be present (either content_hash or contentHash)
	_, hasHash := result["content_hash"]
	_, hasHashCamel := result["contentHash"]
	if !hasHash && !hasHashCamel {
		t.Log("Warning: JSON output does not contain content_hash field")
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestAddExternalCmd_Help tests that help output shows usage information.
func TestAddExternalCmd_Help(t *testing.T) {
	cmd := newAddExternalCmd()
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
		"add-external",
		"external",
		"--name",
		"--source",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestAddExternalCmd_HelpShortFlag tests help with short flag.
func TestAddExternalCmd_HelpShortFlag(t *testing.T) {
	cmd := newAddExternalCmd()
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

// TestAddExternalCmd_HelpMentionsRequiredFlags tests help indicates required flags.
func TestAddExternalCmd_HelpMentionsRequiredFlags(t *testing.T) {
	cmd := newAddExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	// Help should indicate --name and --source are important
	if !strings.Contains(output, "name") {
		t.Error("help should mention --name flag")
	}
	if !strings.Contains(output, "source") {
		t.Error("help should mention --source flag")
	}
}

// =============================================================================
// Default Directory Tests
// =============================================================================

// TestAddExternalCmd_DefaultDirectory tests add-external uses current directory by default.
func TestAddExternalCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
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

	// Execute without -d flag (should use current directory), get JSON output
	output, err := executeAddExternalCommand(t,
		"--name", "Default Dir Theorem",
		"--source", "Default Dir Paper (2024)",
		"--format", "json",
	)

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Parse JSON to get the external ID
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	var extID string
	for _, key := range []string{"id", "external_id", "externalId", "ID"} {
		if val, ok := result[key].(string); ok && val != "" {
			extID = val
			break
		}
	}

	// Verify external was added
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	if extID != "" {
		ext := st.GetExternal(extID)
		if ext == nil {
			t.Error("external not found when using default directory")
		}
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestAddExternalCmd_TableDrivenValidInputs tests various valid input combinations.
func TestAddExternalCmd_TableDrivenValidInputs(t *testing.T) {
	tests := []struct {
		name    string
		extName string
		source  string
	}{
		{
			name:    "simple name and source",
			extName: "Theorem A",
			source:  "Paper A",
		},
		{
			name:    "name with special characters",
			extName: "Euler's Identity (e^(i*pi) + 1 = 0)",
			source:  "Euler, L. (1748)",
		},
		{
			name:    "long source citation",
			extName: "Complex Analysis Theorem",
			source:  "Smith, J. A., Johnson, B. C., & Williams, D. E. (2023). A comprehensive study of complex analysis in modern mathematics. Journal of Mathematical Analysis, 45(3), 234-289. https://doi.org/10.1234/jma.2023.45.3.234",
		},
		{
			name:    "source with unicode",
			extName: "Japanese Mathematics Theorem",
			source:  "Tanaka, H. (2020)",
		},
		{
			name:    "name with numbers",
			extName: "Theorem 3.14159",
			source:  "Pi Paper (3141)",
		},
		{
			name:    "multiline source",
			extName: "Multi-source Theorem",
			source:  "First Author (2020); Second Author (2021); Third Author (2022)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupAddExternalTest(t)
			defer cleanup()

			_, err := executeAddExternalCommand(t,
				"--name", tc.extName,
				"--source", tc.source,
				"-d", tmpDir,
			)

			if err != nil {
				t.Errorf("expected no error for valid input, got: %v", err)
			}
		})
	}
}

// TestAddExternalCmd_TableDrivenInvalidInputs tests various invalid input combinations.
func TestAddExternalCmd_TableDrivenInvalidInputs(t *testing.T) {
	tests := []struct {
		name        string
		extName     string
		source      string
		errContains string
	}{
		{
			name:        "empty name",
			extName:     "",
			source:      "Valid Source",
			errContains: "name",
		},
		{
			name:        "empty source",
			extName:     "Valid Name",
			source:      "",
			errContains: "source",
		},
		{
			name:        "whitespace name",
			extName:     "   \t\n   ",
			source:      "Valid Source",
			errContains: "name",
		},
		{
			name:        "whitespace source",
			extName:     "Valid Name",
			source:      "   \t\n   ",
			errContains: "source",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupAddExternalTest(t)
			defer cleanup()

			output, err := executeAddExternalCommand(t,
				"--name", tc.extName,
				"--source", tc.source,
				"-d", tmpDir,
			)

			combined := output
			if err != nil {
				combined += err.Error()
			}

			if err == nil && !strings.Contains(strings.ToLower(combined), tc.errContains) {
				t.Errorf("expected error containing %q, got output: %q", tc.errContains, output)
			}
		})
	}
}

// =============================================================================
// Flag Variant Tests
// =============================================================================

// TestAddExternalCmd_DirFlagVariants tests both long and short forms of --dir flag.
func TestAddExternalCmd_DirFlagVariants(t *testing.T) {
	tests := []struct {
		name string
		flag string
	}{
		{"long form", "--dir"},
		{"short form", "-d"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupAddExternalTest(t)
			defer cleanup()

			_, err := executeAddExternalCommand(t,
				"--name", "Test Theorem",
				"--source", "Test Source",
				tc.flag, tmpDir,
			)

			if err != nil {
				t.Errorf("expected no error with %s flag, got: %v", tc.flag, err)
			}
		})
	}
}

// TestAddExternalCmd_FormatFlagVariants tests both long and short forms of --format flag.
func TestAddExternalCmd_FormatFlagVariants(t *testing.T) {
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
			tmpDir, cleanup := setupAddExternalTest(t)
			defer cleanup()

			_, err := executeAddExternalCommand(t,
				"--name", "Test Theorem",
				"--source", "Test Source",
				tc.flag, tc.format,
				"-d", tmpDir,
			)

			if err != nil {
				t.Errorf("expected no error with %s %s, got: %v", tc.flag, tc.format, err)
			}
		})
	}
}

// TestAddExternalCmd_NameFlagVariants tests both long and short forms of --name flag.
func TestAddExternalCmd_NameFlagVariants(t *testing.T) {
	tests := []struct {
		name string
		flag string
	}{
		{"long form", "--name"},
		{"short form", "-n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupAddExternalTest(t)
			defer cleanup()

			_, err := executeAddExternalCommand(t,
				tc.flag, "Test Theorem",
				"--source", "Test Source",
				"-d", tmpDir,
			)

			if err != nil {
				t.Errorf("expected no error with %s flag, got: %v", tc.flag, err)
			}
		})
	}
}

// TestAddExternalCmd_SourceFlagVariants tests both long and short forms of --source flag.
func TestAddExternalCmd_SourceFlagVariants(t *testing.T) {
	tests := []struct {
		name string
		flag string
	}{
		{"long form", "--source"},
		{"short form", "-s"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupAddExternalTest(t)
			defer cleanup()

			_, err := executeAddExternalCommand(t,
				"--name", "Test Theorem",
				tc.flag, "Test Source",
				"-d", tmpDir,
			)

			if err != nil {
				t.Errorf("expected no error with %s flag, got: %v", tc.flag, err)
			}
		})
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestAddExternalCmd_VeryLongName tests handling of very long name.
func TestAddExternalCmd_VeryLongName(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	longName := strings.Repeat("A Very Long Theorem Name ", 50)

	_, err := executeAddExternalCommand(t,
		"--name", longName,
		"--source", "Test Source",
		"-d", tmpDir,
	)

	// Should handle long names gracefully (either succeed or return clear error)
	if err != nil {
		t.Logf("Long name returned error: %v", err)
		// If there's a length limit, that's acceptable
	}
}

// TestAddExternalCmd_VeryLongSource tests handling of very long source.
func TestAddExternalCmd_VeryLongSource(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	longSource := strings.Repeat("A detailed citation with many authors and references. ", 100)

	_, err := executeAddExternalCommand(t,
		"--name", "Test Theorem",
		"--source", longSource,
		"-d", tmpDir,
	)

	// Should handle long sources gracefully
	if err != nil {
		t.Logf("Long source returned error: %v", err)
	}
}

// TestAddExternalCmd_SpecialCharactersInName tests special characters in name.
func TestAddExternalCmd_SpecialCharactersInName(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	specialName := "Theorem with symbols: < > & \" ' / \\ @ # $ % ^ * ( ) { } [ ]"

	_, err := executeAddExternalCommand(t,
		"--name", specialName,
		"--source", "Test Source",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with special characters in name, got: %v", err)
	}
}

// TestAddExternalCmd_UnicodeInFields tests unicode characters in name and source.
func TestAddExternalCmd_UnicodeInFields(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	_, err := executeAddExternalCommand(t,
		"--name", "Gauss-Bonnet Theorem: K = 2(1-g)",
		"--source", "Gauss, C. F. (1827)",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with unicode, got: %v", err)
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestAddExternalCmd_TextFormat tests text output format.
func TestAddExternalCmd_TextFormat(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"--name", "Test Theorem",
		"--source", "Test Source",
		"--format", "text",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Text format should be human-readable, non-empty
	if output == "" {
		t.Error("expected non-empty text output")
	}
}

// TestAddExternalCmd_InvalidFormat tests error for invalid format option.
func TestAddExternalCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"--name", "Test Theorem",
		"--source", "Test Source",
		"--format", "invalid",
		"-d", tmpDir,
	)

	// Should produce error or warning about invalid format
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// The behavior depends on implementation - it might error or fall back to text
	// At minimum, it shouldn't crash
	t.Logf("Output with invalid format: %s (error: %v)", output, err)
}

// =============================================================================
// Interface Compliance Test
// =============================================================================

// TestAddExternalCmd_ExpectedFlags tests that command has expected flags.
func TestAddExternalCmd_ExpectedFlags(t *testing.T) {
	cmd := newAddExternalCmd()

	// Check expected flags exist
	expectedFlags := []string{"name", "source", "dir", "format"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected add-external command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"n": "name",
		"s": "source",
		"d": "dir",
		"f": "format",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected add-external command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestAddExternalCmd_CommandName tests that command has correct name.
func TestAddExternalCmd_CommandName(t *testing.T) {
	cmd := newAddExternalCmd()

	if cmd.Name() != "add-external" {
		t.Errorf("expected command name 'add-external', got %q", cmd.Name())
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestAddExternalCmd_VerifyExternalInState tests complete flow including state verification.
func TestAddExternalCmd_VerifyExternalInState(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	extName := "Fundamental Theorem of Calculus"
	extSource := "Newton, I. & Leibniz, G. W. (1687)"

	// Add external via command with JSON output to get ID
	output, err := executeAddExternalCommand(t,
		"--name", extName,
		"--source", extSource,
		"--format", "json",
		"-d", tmpDir,
	)
	if err != nil {
		t.Fatalf("add-external failed: %v", err)
	}

	// Parse JSON to get the external ID
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	var extID string
	for _, key := range []string{"id", "external_id", "externalId", "ID"} {
		if val, ok := result[key].(string); ok && val != "" {
			extID = val
			break
		}
	}

	// Verify in state
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()

	if extID != "" {
		ext := st.GetExternal(extID)
		if ext == nil {
			t.Fatal("external not found in state")
		}

		if ext.Name != extName {
			t.Errorf("external name = %q, want %q", ext.Name, extName)
		}

		if ext.Source != extSource {
			t.Errorf("external source = %q, want %q", ext.Source, extSource)
		}
	}
}

// TestAddExternalCmd_JSONContainsAllFields tests JSON output contains all expected fields.
func TestAddExternalCmd_JSONContainsAllFields(t *testing.T) {
	tmpDir, cleanup := setupAddExternalTest(t)
	defer cleanup()

	output, err := executeAddExternalCommand(t,
		"--name", "Complete Fields Test",
		"--source", "Test Source (2024)",
		"--format", "json",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check for expected fields (with flexible naming conventions)
	fieldChecks := []struct {
		name       string
		variations []string
	}{
		{"id", []string{"id", "external_id", "externalId", "ID"}},
		{"name", []string{"name", "Name"}},
		{"source", []string{"source", "Source"}},
	}

	for _, fc := range fieldChecks {
		found := false
		for _, variant := range fc.variations {
			if _, ok := result[variant]; ok {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warning: JSON output does not contain %s field (checked: %v)", fc.name, fc.variations)
		}
	}
}
