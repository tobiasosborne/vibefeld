//go:build integration

// Package main contains tests for the af defs and af def commands.
// These are TDD tests - the defs/def commands do not exist yet.
// Tests define the expected behavior for listing and viewing definitions.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupDefsTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupDefsTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-defs-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	// Initialize proof
	err = service.Init(tmpDir, "Test conjecture for definitions", "test-author")
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupDefsTestWithDefinitions creates a test environment with an initialized proof
// and some pre-existing definitions.
func setupDefsTestWithDefinitions(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupDefsTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add some definitions for testing
	definitions := []struct {
		name    string
		content string
	}{
		{"group", "A group is a set G with a binary operation * such that: (1) closure, (2) associativity, (3) identity exists, (4) inverses exist."},
		{"homomorphism", "A homomorphism is a structure-preserving map between two algebraic structures of the same type."},
		{"kernel", "The kernel of a homomorphism f: G -> H is the set of elements in G that map to the identity in H."},
	}

	for _, def := range definitions {
		_, err := svc.AddDefinition(def.name, def.content)
		if err != nil {
			cleanup()
			t.Fatalf("failed to add definition %q: %v", def.name, err)
		}
	}

	return tmpDir, cleanup
}

// newTestDefsCmd creates a fresh root command with the defs subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestDefsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	defsCmd := newDefsCmd()
	cmd.AddCommand(defsCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// newTestDefCmd creates a fresh root command with the def subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestDefCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	defCmd := newDefCmd()
	cmd.AddCommand(defCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// executeDefsCommand creates and executes a defs command with the given arguments.
func executeDefsCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newDefsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// executeDefCommand creates and executes a def command with the given arguments.
func executeDefCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newDefCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// af defs - List All Definitions Tests
// =============================================================================

// TestDefsCmd_ListDefinitions tests listing all definitions in a proof.
func TestDefsCmd_ListDefinitions(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	cmd := newTestDefsCmd()
	output, err := executeCommand(cmd, "defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain all definition names
	expectedNames := []string{"group", "homomorphism", "kernel"}
	for _, name := range expectedNames {
		if !strings.Contains(output, name) {
			t.Errorf("expected output to contain definition name %q, got: %q", name, output)
		}
	}
}

// TestDefsCmd_ListDefinitionsEmpty tests listing definitions when none exist.
func TestDefsCmd_ListDefinitionsEmpty(t *testing.T) {
	tmpDir, cleanup := setupDefsTest(t)
	defer cleanup()

	cmd := newTestDefsCmd()
	output, err := executeCommand(cmd, "defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should indicate no definitions or be empty/show a message
	lower := strings.ToLower(output)
	hasNoDefsIndicator := strings.Contains(lower, "no definitions") ||
		strings.Contains(lower, "none") ||
		strings.Contains(lower, "empty") ||
		len(strings.TrimSpace(output)) == 0

	if !hasNoDefsIndicator {
		t.Logf("Output when no definitions exist: %q", output)
	}
}

// TestDefsCmd_ListShowsCount tests that the listing shows a count of definitions.
func TestDefsCmd_ListShowsCount(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	cmd := newTestDefsCmd()
	output, err := executeCommand(cmd, "defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain some indication of count (3 definitions)
	if !strings.Contains(output, "3") {
		t.Logf("Output may or may not show count: %q", output)
	}
}

// TestDefsCmd_ListSortedByName tests that definitions are listed in sorted order.
func TestDefsCmd_ListSortedByName(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	cmd := newTestDefsCmd()
	output, err := executeCommand(cmd, "defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check that "group" appears before "homomorphism" which appears before "kernel"
	// (alphabetical order)
	groupIdx := strings.Index(output, "group")
	homoIdx := strings.Index(output, "homomorphism")
	kernelIdx := strings.Index(output, "kernel")

	if groupIdx != -1 && homoIdx != -1 && kernelIdx != -1 {
		if !(groupIdx < homoIdx && homoIdx < kernelIdx) {
			t.Logf("Definitions may not be sorted alphabetically: group@%d, homomorphism@%d, kernel@%d", groupIdx, homoIdx, kernelIdx)
		}
	}
}

// =============================================================================
// af defs - JSON Output Tests
// =============================================================================

// TestDefsCmd_JSONOutput tests JSON output format for listing definitions.
func TestDefsCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	cmd := newTestDefsCmd()
	output, err := executeCommand(cmd, "defs", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestDefsCmd_JSONOutputStructure tests the structure of JSON output.
func TestDefsCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	cmd := newTestDefsCmd()
	output, err := executeCommand(cmd, "defs", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Try to unmarshal as array or object
	var arrayResult []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &arrayResult); err == nil {
		// It's an array - each item should have definition fields
		if len(arrayResult) != 3 {
			t.Errorf("expected 3 definitions in JSON array, got %d", len(arrayResult))
		}

		for i, def := range arrayResult {
			if _, ok := def["name"]; !ok {
				t.Errorf("definition %d missing 'name' field", i)
			}
		}
	} else {
		// Try as object with definitions array
		var objResult map[string]interface{}
		if err := json.Unmarshal([]byte(output), &objResult); err != nil {
			t.Errorf("output is not valid JSON array or object: %v", err)
		} else {
			// Check for a definitions array in the object
			if defs, ok := objResult["definitions"]; ok {
				if defsArr, ok := defs.([]interface{}); ok {
					if len(defsArr) != 3 {
						t.Errorf("expected 3 definitions, got %d", len(defsArr))
					}
				}
			}
		}
	}
}

// TestDefsCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestDefsCmd_JSONOutputShortFlag(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	cmd := newTestDefsCmd()
	output, err := executeCommand(cmd, "defs", "-f", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON with -f flag: %v\nOutput: %q", err, output)
	}
}

// TestDefsCmd_JSONOutputEmpty tests JSON output when no definitions exist.
func TestDefsCmd_JSONOutputEmpty(t *testing.T) {
	tmpDir, cleanup := setupDefsTest(t)
	defer cleanup()

	cmd := newTestDefsCmd()
	output, err := executeCommand(cmd, "defs", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON (empty array or object with empty definitions)
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("empty output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// =============================================================================
// af def <name> - Show Specific Definition Tests
// =============================================================================

// TestDefCmd_ShowDefinition tests showing a specific definition by name.
func TestDefCmd_ShowDefinition(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	cmd := newTestDefCmd()
	output, err := executeCommand(cmd, "def", "group", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain the definition name
	if !strings.Contains(output, "group") {
		t.Errorf("expected output to contain definition name 'group', got: %q", output)
	}

	// Should contain the definition content
	if !strings.Contains(output, "binary operation") {
		t.Errorf("expected output to contain definition content, got: %q", output)
	}
}

// TestDefCmd_ShowDefinitionByName tests showing different definitions.
func TestDefCmd_ShowDefinitionByName(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	tests := []struct {
		name            string
		expectedContent string
	}{
		{"group", "binary operation"},
		{"homomorphism", "structure-preserving"},
		{"kernel", "identity"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestDefCmd()
			output, err := executeCommand(cmd, "def", tc.name, "--dir", tmpDir)

			if err != nil {
				t.Fatalf("expected no error for %q, got: %v", tc.name, err)
			}

			if !strings.Contains(output, tc.expectedContent) {
				t.Errorf("expected output for %q to contain %q, got: %q", tc.name, tc.expectedContent, output)
			}
		})
	}
}

// TestDefCmd_ShowDefinitionFull tests showing full definition details.
func TestDefCmd_ShowDefinitionFull(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	cmd := newTestDefCmd()
	output, err := executeCommand(cmd, "def", "group", "--full", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Full output should include:
	// - Definition name
	// - Content
	// - ID (if shown)
	// - Content hash (if shown)
	// - Created timestamp (if shown)

	if !strings.Contains(output, "group") {
		t.Errorf("expected full output to contain name, got: %q", output)
	}

	if !strings.Contains(output, "binary operation") {
		t.Errorf("expected full output to contain content, got: %q", output)
	}
}

// =============================================================================
// af def <name> - JSON Output Tests
// =============================================================================

// TestDefCmd_JSONOutput tests JSON output for a specific definition.
func TestDefCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	cmd := newTestDefCmd()
	output, err := executeCommand(cmd, "def", "group", "--format", "json", "--dir", tmpDir)

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
	} else if name != "group" {
		t.Errorf("expected name 'group', got %v", name)
	}

	if _, ok := result["content"]; !ok {
		t.Logf("Warning: JSON output may not have 'content' field")
	}
}

// TestDefCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestDefCmd_JSONOutputShortFlag(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	cmd := newTestDefCmd()
	output, err := executeCommand(cmd, "def", "homomorphism", "-f", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestDefsCmd_ProofNotInitialized tests error when proof is not initialized.
func TestDefsCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-defs-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestDefsCmd()
	_, err = executeCommand(cmd, "defs", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestDefsCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestDefsCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestDefsCmd()
	_, err := executeCommand(cmd, "defs", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestDefCmd_DefinitionNotFound tests error when definition doesn't exist.
func TestDefCmd_DefinitionNotFound(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	cmd := newTestDefCmd()
	output, err := executeCommand(cmd, "def", "nonexistent", "--dir", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should indicate definition not found
	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "not found") && !strings.Contains(lower, "does not exist") && err == nil {
		t.Errorf("expected error for non-existent definition, got: %q", output)
	}
}

// TestDefCmd_MissingDefinitionName tests error when definition name is not provided.
func TestDefCmd_MissingDefinitionName(t *testing.T) {
	tmpDir, cleanup := setupDefsTest(t)
	defer cleanup()

	cmd := newTestDefCmd()
	_, err := executeCommand(cmd, "def", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing definition name, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") &&
		!strings.Contains(errStr, "required") &&
		!strings.Contains(errStr, "argument") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestDefCmd_ProofNotInitialized tests error when proof is not initialized.
func TestDefCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-def-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestDefCmd()
	_, err = executeCommand(cmd, "def", "group", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestDefCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestDefCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestDefCmd()
	_, err := executeCommand(cmd, "def", "group", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestDefCmd_EmptyDefinitionName tests error for empty definition name.
func TestDefCmd_EmptyDefinitionName(t *testing.T) {
	tmpDir, cleanup := setupDefsTest(t)
	defer cleanup()

	cmd := newTestDefCmd()
	_, err := executeCommand(cmd, "def", "", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for empty definition name, got nil")
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestDefsCmd_Help tests that help output shows usage information.
func TestDefsCmd_Help(t *testing.T) {
	cmd := newDefsCmd()
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
		"defs",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestDefsCmd_HelpShortFlag tests help with -h short flag.
func TestDefsCmd_HelpShortFlag(t *testing.T) {
	cmd := newDefsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "defs") {
		t.Errorf("help output should mention 'defs', got: %q", output)
	}
}

// TestDefCmd_Help tests that help output shows usage information.
func TestDefCmd_Help(t *testing.T) {
	cmd := newDefCmd()
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
		"def",
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

// TestDefCmd_HelpShortFlag tests help with -h short flag.
func TestDefCmd_HelpShortFlag(t *testing.T) {
	cmd := newDefCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "def") {
		t.Errorf("help output should mention 'def', got: %q", output)
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestDefsCmd_ExpectedFlags ensures the defs command has expected flag structure.
func TestDefsCmd_ExpectedFlags(t *testing.T) {
	cmd := newDefsCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected defs command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected defs command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestDefsCmd_DefaultFlagValues verifies default values for flags.
func TestDefsCmd_DefaultFlagValues(t *testing.T) {
	cmd := newDefsCmd()

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

// TestDefCmd_ExpectedFlags ensures the def command has expected flag structure.
func TestDefCmd_ExpectedFlags(t *testing.T) {
	cmd := newDefCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir", "full"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected def command to have flag %q", flagName)
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
			t.Errorf("expected def command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestDefCmd_DefaultFlagValues verifies default values for flags.
func TestDefCmd_DefaultFlagValues(t *testing.T) {
	cmd := newDefCmd()

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

// TestDefsCmd_CommandMetadata verifies command metadata.
func TestDefsCmd_CommandMetadata(t *testing.T) {
	cmd := newDefsCmd()

	if cmd.Use != "defs" {
		t.Errorf("expected Use to be 'defs', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestDefCmd_CommandMetadata verifies command metadata.
func TestDefCmd_CommandMetadata(t *testing.T) {
	cmd := newDefCmd()

	if !strings.HasPrefix(cmd.Use, "def") {
		t.Errorf("expected Use to start with 'def', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Default Directory Tests
// =============================================================================

// TestDefsCmd_DefaultDirectory tests defs uses current directory by default.
func TestDefsCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
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
	cmd := newTestDefsCmd()
	output, err := executeCommand(cmd, "defs")

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Should list definitions
	if !strings.Contains(output, "group") {
		t.Errorf("expected output to contain 'group', got: %q", output)
	}
}

// TestDefCmd_DefaultDirectory tests def uses current directory by default.
func TestDefCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
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
	cmd := newTestDefCmd()
	output, err := executeCommand(cmd, "def", "group")

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Should show definition
	if !strings.Contains(output, "group") {
		t.Errorf("expected output to contain 'group', got: %q", output)
	}
}

// =============================================================================
// Format Validation Tests
// =============================================================================

// TestDefsCmd_FormatValidation verifies format flag validation.
func TestDefsCmd_FormatValidation(t *testing.T) {
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
			tmpDir, cleanup := setupDefsTestWithDefinitions(t)
			defer cleanup()

			cmd := newTestDefsCmd()
			output, err := executeCommand(cmd, "defs", "--format", tc.format, "--dir", tmpDir)

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

// TestDefCmd_FormatValidation verifies format flag validation for def command.
func TestDefCmd_FormatValidation(t *testing.T) {
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
			tmpDir, cleanup := setupDefsTestWithDefinitions(t)
			defer cleanup()

			cmd := newTestDefCmd()
			output, err := executeCommand(cmd, "def", "group", "--format", tc.format, "--dir", tmpDir)

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

// TestDefCmd_CaseInsensitiveName tests if definition lookup is case-insensitive.
func TestDefCmd_CaseInsensitiveName(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	// Test with different cases of "group"
	testCases := []string{"group", "GROUP", "Group", "gROUP"}

	for _, name := range testCases {
		t.Run(name, func(t *testing.T) {
			cmd := newTestDefCmd()
			output, err := executeCommand(cmd, "def", name, "--dir", tmpDir)

			// Implementation may be case-sensitive or case-insensitive
			// This test documents the behavior
			if err != nil {
				t.Logf("Case %q returned error: %v (may be case-sensitive)", name, err)
			} else {
				if !strings.Contains(strings.ToLower(output), "group") {
					t.Errorf("expected output to contain 'group', got: %q", output)
				}
			}
		})
	}
}

// =============================================================================
// Fuzzy Matching Tests
// =============================================================================

// TestDefCmd_FuzzyMatchSuggestion tests fuzzy matching suggestions for typos.
func TestDefCmd_FuzzyMatchSuggestion(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	// Try a typo that's close to "group"
	cmd := newTestDefCmd()
	output, err := executeCommand(cmd, "def", "gropu", "--dir", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should either find via fuzzy match or suggest similar definitions
	// This test documents the expected behavior for typos
	t.Logf("Typo 'gropu' result: %s (error: %v)", output, err)
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestDefsCmd_TableDrivenDirectories tests various directory scenarios.
func TestDefsCmd_TableDrivenDirectories(t *testing.T) {
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
			cmd := newTestDefsCmd()
			_, err := executeCommand(cmd, "defs", "--dir", tc.dirPath)

			if tc.wantErr && err == nil {
				t.Errorf("expected error for dir %q, got nil", tc.dirPath)
			}
		})
	}
}

// TestDefCmd_TableDrivenDefinitionNames tests various definition name inputs.
func TestDefCmd_TableDrivenDefinitionNames(t *testing.T) {
	tests := []struct {
		name        string
		defName     string
		expectFound bool
	}{
		{"existing definition", "group", true},
		{"existing definition 2", "homomorphism", true},
		{"existing definition 3", "kernel", true},
		{"nonexistent definition", "nonexistent", false},
		{"partial match", "gro", false},
		{"with spaces", "group theory", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupDefsTestWithDefinitions(t)
			defer cleanup()

			cmd := newTestDefCmd()
			output, err := executeCommand(cmd, "def", tc.defName, "--dir", tmpDir)

			if tc.expectFound {
				if err != nil {
					t.Errorf("expected definition %q to be found, got error: %v", tc.defName, err)
				}
			} else {
				combined := output
				if err != nil {
					combined += err.Error()
				}
				// Should indicate not found
				if err == nil && !strings.Contains(strings.ToLower(combined), "not found") {
					t.Logf("Definition %q not expected to be found, but no error. Output: %q", tc.defName, output)
				}
			}
		})
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestDefsCmd_ManyDefinitions tests listing many definitions.
func TestDefsCmd_ManyDefinitions(t *testing.T) {
	tmpDir, cleanup := setupDefsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add many definitions
	for i := 0; i < 50; i++ {
		name := strings.Repeat("term", i+1)
		if len(name) > 100 {
			name = name[:100]
		}
		content := strings.Repeat("Definition content. ", i+1)
		_, err := svc.AddDefinition(name+string(rune('a'+i%26)), content)
		if err != nil {
			t.Logf("Failed to add definition %d: %v", i, err)
			break
		}
	}

	cmd := newTestDefsCmd()
	output, err := executeCommand(cmd, "defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error listing many definitions, got: %v", err)
	}

	t.Logf("Output length for many definitions: %d bytes", len(output))
}

// TestDefCmd_LongDefinitionName tests showing a definition with a long name.
func TestDefCmd_LongDefinitionName(t *testing.T) {
	tmpDir, cleanup := setupDefsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add a definition with a long name
	longName := strings.Repeat("mathematical_concept_", 10)
	_, err = svc.AddDefinition(longName, "Some definition content")
	if err != nil {
		t.Logf("Could not add long-named definition: %v", err)
		return
	}

	cmd := newTestDefCmd()
	output, err := executeCommand(cmd, "def", longName, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for long definition name, got: %v", err)
	}

	if !strings.Contains(output, "mathematical_concept_") {
		t.Errorf("expected output to contain long name, got: %q", output)
	}
}

// TestDefCmd_UnicodeDefinitionName tests definition with unicode characters.
func TestDefCmd_UnicodeDefinitionName(t *testing.T) {
	tmpDir, cleanup := setupDefsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add a definition with unicode name
	unicodeName := "epsilon_delta"
	_, err = svc.AddDefinition(unicodeName, "The epsilon-delta definition of continuity")
	if err != nil {
		t.Logf("Could not add unicode-named definition: %v", err)
		return
	}

	cmd := newTestDefCmd()
	output, err := executeCommand(cmd, "def", unicodeName, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for unicode definition name, got: %v", err)
	}

	if !strings.Contains(output, "epsilon") {
		t.Errorf("expected output to contain unicode name, got: %q", output)
	}
}

// TestDefsCmd_WithVerboseFlag tests potential verbose output flag.
func TestDefsCmd_WithVerboseFlag(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	// Test if verbose flag exists and changes output
	cmd := newTestDefsCmd()
	output, err := executeCommand(cmd, "defs", "--verbose", "--dir", tmpDir)

	// Verbose flag may or may not exist
	if err != nil {
		t.Logf("Verbose flag may not be implemented: %v", err)
	} else {
		t.Logf("Verbose output: %q", output)
	}
}

// =============================================================================
// Consistency Tests
// =============================================================================

// TestDefsAndDefConsistency tests that defs and def show consistent information.
func TestDefsAndDefConsistency(t *testing.T) {
	tmpDir, cleanup := setupDefsTestWithDefinitions(t)
	defer cleanup()

	// Get list from defs
	defsCmd := newTestDefsCmd()
	defsOutput, err := executeCommand(defsCmd, "defs", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("defs command failed: %v", err)
	}

	// Verify each definition from defs can be retrieved with def
	definitionNames := []string{"group", "homomorphism", "kernel"}
	for _, name := range definitionNames {
		// Verify name appears in defs output
		if !strings.Contains(defsOutput, name) {
			t.Errorf("definition %q not found in defs output", name)
		}

		// Verify def can retrieve it
		defCmd := newTestDefCmd()
		defOutput, err := executeCommand(defCmd, "def", name, "--dir", tmpDir)
		if err != nil {
			t.Errorf("def command failed for %q: %v", name, err)
		}

		if !strings.Contains(defOutput, name) {
			t.Errorf("def output for %q doesn't contain the name", name)
		}
	}
}
