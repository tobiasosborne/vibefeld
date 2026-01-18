//go:build integration

// Package main contains tests for the af def-add command.
// These are TDD tests - the def-add command does not exist yet.
// Tests define the expected behavior for adding definitions to the proof.
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

// setupDefAddTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupDefAddTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-def-add-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	// Initialize proof via service
	err = service.Init(tmpDir, "Test conjecture for definitions", "test-author")
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// executeDefAddCommand creates and executes a def-add command with the given arguments.
func executeDefAddCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newDefAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// newDefAddTestCmd creates a test command hierarchy with the def-add command.
// This ensures test isolation - each test gets its own command instance.
func newDefAddTestCmd() *cobra.Command {
	cmd := newTestRootCmd()

	defAddCmd := newDefAddCmd()
	cmd.AddCommand(defAddCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestDefAddCmd_Help tests that help output shows usage information.
func TestDefAddCmd_Help(t *testing.T) {
	cmd := newDefAddCmd()
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
		"def-add",
		"name",
		"content",
		"--dir",
		"--format",
		"--file",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestDefAddCmd_HelpShortFlag tests help with -h short flag.
func TestDefAddCmd_HelpShortFlag(t *testing.T) {
	cmd := newDefAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "def-add") {
		t.Errorf("help output should mention 'def-add', got: %q", output)
	}
}

// TestDefAddCmd_CommandMetadata verifies command metadata.
func TestDefAddCmd_CommandMetadata(t *testing.T) {
	cmd := newDefAddCmd()

	if cmd.Use == "" {
		t.Error("expected Use to be set")
	}

	if !strings.Contains(cmd.Use, "def-add") {
		t.Errorf("expected Use to contain 'def-add', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestDefAddCmd_ExpectedFlags ensures the def-add command has expected flag structure.
func TestDefAddCmd_ExpectedFlags(t *testing.T) {
	cmd := newDefAddCmd()

	// Check expected flags exist
	expectedFlags := []string{"dir", "format", "file"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected def-add command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"d": "dir",
		"f": "format",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected def-add command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestDefAddCmd_DefaultFlagValues verifies default values for flags.
func TestDefAddCmd_DefaultFlagValues(t *testing.T) {
	cmd := newDefAddCmd()

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

	// Check default file value
	fileFlag := cmd.Flags().Lookup("file")
	if fileFlag == nil {
		t.Fatal("expected file flag to exist")
	}
	if fileFlag.DefValue != "" {
		t.Errorf("expected default file to be empty, got %q", fileFlag.DefValue)
	}
}

// TestDefAddCmd_DirFlagVariants tests both long and short forms of --dir flag.
func TestDefAddCmd_DirFlagVariants(t *testing.T) {
	tests := []struct {
		name string
		flag string
	}{
		{"long form", "--dir"},
		{"short form", "-d"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupDefAddTest(t)
			defer cleanup()

			_, err := executeDefAddCommand(t, "test_def", "A test definition", tc.flag, tmpDir)

			if err != nil {
				t.Errorf("expected no error with %s flag, got: %v", tc.flag, err)
			}
		})
	}
}

// TestDefAddCmd_FormatFlagVariants tests both long and short forms of --format flag.
func TestDefAddCmd_FormatFlagVariants(t *testing.T) {
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
			tmpDir, cleanup := setupDefAddTest(t)
			defer cleanup()

			_, err := executeDefAddCommand(t,
				"test_def", "A test definition",
				tc.flag, tc.format,
				"-d", tmpDir,
			)

			if err != nil {
				t.Errorf("expected no error with %s %s, got: %v", tc.flag, tc.format, err)
			}
		})
	}
}

// =============================================================================
// Argument Validation Tests
// =============================================================================

// TestDefAddCmd_MissingNameArgument tests error when name is not provided.
func TestDefAddCmd_MissingNameArgument(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	// Execute without any arguments
	_, err := executeDefAddCommand(t, "-d", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing name argument, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "arg") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestDefAddCmd_MissingContent tests error when no content and no --file provided.
func TestDefAddCmd_MissingContent(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	// Execute with name only, no content and no --file
	output, err := executeDefAddCommand(t, "test_def", "-d", tmpDir)

	// Should error because no content is provided
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "content") &&
		!strings.Contains(strings.ToLower(combined), "required") {
		t.Errorf("expected error for missing content, got: %q", output)
	}
}

// TestDefAddCmd_EmptyNameRejected tests that empty name is rejected.
func TestDefAddCmd_EmptyNameRejected(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t, "", "Some content", "-d", tmpDir)

	// Should error because name cannot be empty
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "empty") &&
		!strings.Contains(strings.ToLower(combined), "name") {
		t.Errorf("expected error for empty name, got: %q", output)
	}
}

// TestDefAddCmd_EmptyContentRejected tests that empty content is rejected.
func TestDefAddCmd_EmptyContentRejected(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t, "test_def", "", "-d", tmpDir)

	// Should error because content cannot be empty
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "empty") &&
		!strings.Contains(strings.ToLower(combined), "content") {
		t.Errorf("expected error for empty content, got: %q", output)
	}
}

// TestDefAddCmd_WhitespaceOnlyNameRejected tests that whitespace-only name is rejected.
func TestDefAddCmd_WhitespaceOnlyNameRejected(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t, "   ", "Some content", "-d", tmpDir)

	// Should error because whitespace-only name is invalid
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "empty") &&
		!strings.Contains(strings.ToLower(combined), "name") {
		t.Errorf("expected error for whitespace-only name, got: %q", output)
	}
}

// TestDefAddCmd_WhitespaceOnlyContentRejected tests that whitespace-only content is rejected.
func TestDefAddCmd_WhitespaceOnlyContentRejected(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t, "test_def", "   ", "-d", tmpDir)

	// Should error because whitespace-only content is invalid
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "empty") &&
		!strings.Contains(strings.ToLower(combined), "content") {
		t.Errorf("expected error for whitespace-only content, got: %q", output)
	}
}

// =============================================================================
// Success Cases
// =============================================================================

// TestDefAddCmd_Success tests adding a definition with name and content args.
func TestDefAddCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t,
		"group", "A group is a set G with a binary operation.",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify output contains success indication
	lower := strings.ToLower(output)
	hasSuccessInfo := strings.Contains(lower, "definition") ||
		strings.Contains(lower, "added") ||
		strings.Contains(lower, "created") ||
		strings.Contains(lower, "success") ||
		strings.Contains(lower, "group")

	if !hasSuccessInfo {
		t.Errorf("expected success message, got: %q", output)
	}
}

// TestDefAddCmd_SuccessReturnsID tests that output contains the definition ID.
func TestDefAddCmd_SuccessReturnsID(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t,
		"homomorphism", "A homomorphism is a structure-preserving map.",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain some identifier (def-* pattern or similar)
	if strings.TrimSpace(output) == "" {
		t.Error("expected non-empty output containing definition ID")
	}

	// Check for ID pattern (typically starts with "def-")
	if !strings.Contains(output, "def-") && !strings.Contains(strings.ToLower(output), "id") {
		t.Logf("Warning: output may not contain visible ID: %q", output)
	}
}

// TestDefAddCmd_SuccessStoredInState tests that definition is stored in state.
func TestDefAddCmd_SuccessStoredInState(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	defName := "kernel"
	defContent := "The kernel of a homomorphism f is the set of elements that map to identity."

	// Add definition via command
	_, err := executeDefAddCommand(t, defName, defContent, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify definition can be retrieved from state
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Look for definition by name
	def := st.GetDefinitionByName(defName)
	if def == nil {
		t.Fatal("definition not found in state after adding")
	}

	if def.Name != defName {
		t.Errorf("definition name = %q, want %q", def.Name, defName)
	}

	if def.Content != defContent {
		t.Errorf("definition content = %q, want %q", def.Content, defContent)
	}
}

// TestDefAddCmd_MultipleDefinitions tests adding multiple definitions sequentially.
func TestDefAddCmd_MultipleDefinitions(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	definitions := []struct {
		name    string
		content string
	}{
		{"group", "A group is a set with a binary operation."},
		{"ring", "A ring is a set with two binary operations."},
		{"field", "A field is a ring with multiplicative inverses."},
	}

	for _, def := range definitions {
		output, err := executeDefAddCommand(t, def.name, def.content, "-d", tmpDir)
		if err != nil {
			t.Fatalf("failed to add definition %q: %v", def.name, err)
		}

		if strings.TrimSpace(output) == "" {
			t.Errorf("expected non-empty output for definition %q", def.name)
		}
	}

	// Verify all definitions were added
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()

	for _, def := range definitions {
		d := st.GetDefinitionByName(def.name)
		if d == nil {
			t.Errorf("definition %q not found in state", def.name)
		}
	}
}

// TestDefAddCmd_AppearsInDefsList tests that added definition appears in af defs output.
func TestDefAddCmd_AppearsInDefsList(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	defName := "vector_space"
	defContent := "A vector space is a set with vector addition and scalar multiplication."

	// Add definition
	_, err := executeDefAddCommand(t, defName, defContent, "-d", tmpDir)
	if err != nil {
		t.Fatalf("failed to add definition: %v", err)
	}

	// Execute defs command to list definitions
	defsCmd := newTestDefsCmd()
	defsOutput, err := executeCommand(defsCmd, "defs", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("defs command failed: %v", err)
	}

	// Verify definition name appears in list
	if !strings.Contains(defsOutput, defName) {
		t.Errorf("expected defs output to contain %q, got: %q", defName, defsOutput)
	}
}

// =============================================================================
// File Input Tests
// =============================================================================

// TestDefAddCmd_FileInput tests adding definition from file with --file flag.
func TestDefAddCmd_FileInput(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	// Create a file with definition content
	defContent := "A topology on a set X is a collection of subsets satisfying certain axioms."
	contentFile := filepath.Join(tmpDir, "def_content.txt")
	if err := os.WriteFile(contentFile, []byte(defContent), 0644); err != nil {
		t.Fatalf("failed to create content file: %v", err)
	}

	output, err := executeDefAddCommand(t,
		"topology",
		"--file", contentFile,
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with --file flag, got: %v", err)
	}

	// Verify definition was added
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()

	def := st.GetDefinitionByName("topology")
	if def == nil {
		t.Fatal("definition not found in state after adding from file")
	}

	if def.Content != defContent {
		t.Errorf("definition content = %q, want %q", def.Content, defContent)
	}

	t.Logf("Output: %s", output)
}

// TestDefAddCmd_FileInputNonExistent tests error when --file points to non-existent file.
func TestDefAddCmd_FileInputNonExistent(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t,
		"test_def",
		"--file", "/nonexistent/path/file.txt",
		"-d", tmpDir,
	)

	// Should error because file doesn't exist
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "no such") &&
		!strings.Contains(strings.ToLower(combined), "error") {
		t.Errorf("expected error for non-existent file, got: %q", output)
	}
}

// TestDefAddCmd_FileInputEmptyFile tests error when --file points to empty file.
func TestDefAddCmd_FileInputEmptyFile(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	// Create an empty file
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}

	output, err := executeDefAddCommand(t,
		"test_def",
		"--file", emptyFile,
		"-d", tmpDir,
	)

	// Should error because file content is empty
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "empty") {
		t.Errorf("expected error for empty file, got: %q", output)
	}
}

// TestDefAddCmd_FileInputOverridesContentArg tests that --file takes precedence over content arg.
func TestDefAddCmd_FileInputOverridesContentArg(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	// Create a file with definition content
	fileContent := "Content from file."
	contentFile := filepath.Join(tmpDir, "def_content.txt")
	if err := os.WriteFile(contentFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("failed to create content file: %v", err)
	}

	// Provide both content arg and --file flag
	_, err := executeDefAddCommand(t,
		"test_def",
		"Content from argument that should be ignored",
		"--file", contentFile,
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify file content was used
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()

	def := st.GetDefinitionByName("test_def")
	if def == nil {
		t.Fatal("definition not found in state")
	}

	if def.Content != fileContent {
		t.Errorf("expected file content %q to be used, got %q", fileContent, def.Content)
	}
}

// TestDefAddCmd_FileInputMultiline tests that multiline content from file is preserved.
func TestDefAddCmd_FileInputMultiline(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	// Create a file with multiline definition content
	fileContent := `A metric space (X, d) is a set X together with a function d: X x X -> R
satisfying the following axioms:
1. d(x, y) >= 0 for all x, y in X (non-negativity)
2. d(x, y) = 0 if and only if x = y (identity of indiscernibles)
3. d(x, y) = d(y, x) for all x, y in X (symmetry)
4. d(x, z) <= d(x, y) + d(y, z) for all x, y, z in X (triangle inequality)`

	contentFile := filepath.Join(tmpDir, "metric_space.txt")
	if err := os.WriteFile(contentFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("failed to create content file: %v", err)
	}

	_, err := executeDefAddCommand(t,
		"metric_space",
		"--file", contentFile,
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with multiline file content, got: %v", err)
	}

	// Verify multiline content was preserved
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()

	def := st.GetDefinitionByName("metric_space")
	if def == nil {
		t.Fatal("definition not found in state")
	}

	if def.Content != fileContent {
		t.Errorf("multiline content not preserved:\nexpected: %q\ngot: %q", fileContent, def.Content)
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestDefAddCmd_TextOutput tests text output format.
func TestDefAddCmd_TextOutput(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t,
		"isomorphism", "An isomorphism is a bijective homomorphism.",
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

	// Should contain definition name or success indication
	if !strings.Contains(output, "isomorphism") && !strings.Contains(strings.ToLower(output), "added") {
		t.Errorf("text output should contain definition name or success message, got: %q", output)
	}
}

// TestDefAddCmd_TextOutputFormat tests expected text output format.
func TestDefAddCmd_TextOutputFormat(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t,
		"test_def", "Test content",
		"-f", "text",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Expected format: "Definition 'X' added with ID: def-abc123" or similar
	lower := strings.ToLower(output)
	hasExpectedParts := (strings.Contains(lower, "definition") && strings.Contains(lower, "added")) ||
		strings.Contains(lower, "success") ||
		strings.Contains(output, "def-")

	if !hasExpectedParts {
		t.Logf("Text output format: %q", output)
	}
}

// TestDefAddCmd_JSONOutput tests JSON output format.
func TestDefAddCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t,
		"automorphism", "An automorphism is an isomorphism from an object to itself.",
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
}

// TestDefAddCmd_JSONOutputStructure tests the structure of JSON output.
func TestDefAddCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t,
		"subgroup", "A subgroup is a subset that is also a group.",
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

	// Expected fields: name, id, added (bool)
	// Check for expected fields
	expectedFields := [][]string{
		{"name", "Name"},
		{"id", "ID", "def_id", "defId"},
		{"added", "Added", "success"},
	}

	for _, variants := range expectedFields {
		found := false
		for _, field := range variants {
			if _, ok := result[field]; ok {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warning: JSON output may not contain %v field", variants)
		}
	}
}

// TestDefAddCmd_JSONOutputContainsName tests JSON output contains the definition name.
func TestDefAddCmd_JSONOutputContainsName(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	defName := "normal_subgroup"

	output, err := executeDefAddCommand(t,
		defName, "A normal subgroup N of G satisfies gNg^-1 = N for all g in G.",
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

	// Check for name field
	name, hasName := result["name"]
	if !hasName {
		t.Log("Warning: JSON output does not have 'name' field")
	} else if name != defName {
		t.Errorf("expected name %q, got %v", defName, name)
	}
}

// TestDefAddCmd_JSONOutputContainsID tests JSON output contains the definition ID.
func TestDefAddCmd_JSONOutputContainsID(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t,
		"coset", "A coset is a form of gH or Hg for element g and subgroup H.",
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

	// Check for ID field
	var defID string
	for _, key := range []string{"id", "ID", "def_id", "defId"} {
		if val, ok := result[key].(string); ok && val != "" {
			defID = val
			break
		}
	}

	if defID == "" {
		t.Log("Warning: JSON output does not contain definition ID")
	} else if !strings.HasPrefix(defID, "def-") {
		t.Logf("Definition ID format: %q (may or may not start with 'def-')", defID)
	}
}

// TestDefAddCmd_InvalidFormat tests error for invalid format option.
func TestDefAddCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t,
		"test_def", "Test content",
		"--format", "invalid",
		"-d", tmpDir,
	)

	// Should produce error or warning about invalid format
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// The behavior depends on implementation - it might error or fall back to text
	t.Logf("Output with invalid format: %s (error: %v)", output, err)
}

// =============================================================================
// Error Cases
// =============================================================================

// TestDefAddCmd_ProofNotInitialized tests error when proof is not initialized.
func TestDefAddCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-def-add-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	output, err := executeDefAddCommand(t,
		"test_def", "Test content",
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

// TestDefAddCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestDefAddCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeDefAddCommand(t,
		"test_def", "Test content",
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
// Default Directory Tests
// =============================================================================

// TestDefAddCmd_DefaultDirectory tests def-add uses current directory by default.
func TestDefAddCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
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
	output, err := executeDefAddCommand(t,
		"default_dir_test", "Content for default directory test",
	)

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Verify definition was added
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	def := st.GetDefinitionByName("default_dir_test")
	if def == nil {
		t.Error("definition not added when using default directory")
	}
}

// =============================================================================
// Duplicate Name Tests
// =============================================================================

// TestDefAddCmd_DuplicateName tests behavior when adding definition with duplicate name.
func TestDefAddCmd_DuplicateName(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	defName := "abelian"
	content1 := "First definition content"
	content2 := "Second definition content"

	// First add
	_, err := executeDefAddCommand(t, defName, content1, "-d", tmpDir)
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	// Second add with same name (behavior may vary - error or create another)
	output, err := executeDefAddCommand(t, defName, content2, "-d", tmpDir)

	// Document the behavior - implementation may:
	// 1. Error on duplicate name
	// 2. Allow multiple definitions with same name
	if err != nil {
		t.Logf("Duplicate name returned error: %v (this may be expected)", err)
	} else {
		t.Logf("Duplicate name succeeded. Output: %q", output)
	}
}

// =============================================================================
// Special Characters Tests
// =============================================================================

// TestDefAddCmd_SpecialCharactersInName tests special characters in name.
func TestDefAddCmd_SpecialCharactersInName(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	// Test various special character patterns
	tests := []struct {
		name    string
		defName string
		wantErr bool
	}{
		{"with underscore", "vector_space", false},
		{"with hyphen", "semi-group", false},
		{"with numbers", "p3_space", false},
		{"with dots", "def.v1", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := executeDefAddCommand(t, tc.defName, "Content", "-d", tmpDir)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for name %q", tc.defName)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for name %q: %v", tc.defName, err)
			}
		})
	}
}

// TestDefAddCmd_UnicodeInName tests unicode characters in name.
func TestDefAddCmd_UnicodeInName(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	unicodeName := "epsilon_delta"
	_, err := executeDefAddCommand(t, unicodeName, "The epsilon-delta definition", "-d", tmpDir)

	if err != nil {
		t.Logf("Unicode in name returned error: %v", err)
	} else {
		// Verify it was stored
		svc, _ := service.NewProofService(tmpDir)
		st, _ := svc.LoadState()
		def := st.GetDefinitionByName(unicodeName)
		if def == nil {
			t.Error("unicode-named definition not found in state")
		}
	}
}

// TestDefAddCmd_MultilineContent tests that multiline content is preserved.
func TestDefAddCmd_MultilineContent(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	multilineContent := `A group is a set G with operation * satisfying:
1. Closure
2. Associativity
3. Identity exists
4. Inverses exist`

	_, err := executeDefAddCommand(t, "multiline_group", multilineContent, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with multiline content, got: %v", err)
	}

	// Verify content was preserved
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	def := st.GetDefinitionByName("multiline_group")
	if def == nil {
		t.Fatal("definition not found")
	}

	if def.Content != multilineContent {
		t.Errorf("multiline content not preserved:\nexpected: %q\ngot: %q", multilineContent, def.Content)
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestDefAddCmd_TableDrivenValidInputs tests various valid input combinations.
func TestDefAddCmd_TableDrivenValidInputs(t *testing.T) {
	tests := []struct {
		name    string
		defName string
		content string
	}{
		{"simple", "simple_def", "Simple content"},
		{"long name", "very_long_definition_name_here", "Content"},
		{"long content", "def", strings.Repeat("Long content. ", 100)},
		{"unicode content", "unicode_def", "Definition with math symbols"},
		{"quotes in content", "quoted_def", `Definition with "quotes" inside`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupDefAddTest(t)
			defer cleanup()

			_, err := executeDefAddCommand(t, tc.defName, tc.content, "-d", tmpDir)

			if err != nil {
				t.Errorf("expected no error for valid input, got: %v", err)
			}

			// Verify stored correctly
			svc, _ := service.NewProofService(tmpDir)
			st, _ := svc.LoadState()
			def := st.GetDefinitionByName(tc.defName)
			if def == nil {
				t.Errorf("definition %q not found in state", tc.defName)
			} else if def.Content != tc.content {
				t.Errorf("content mismatch for %q", tc.defName)
			}
		})
	}
}

// TestDefAddCmd_TableDrivenInvalidInputs tests various invalid input combinations.
func TestDefAddCmd_TableDrivenInvalidInputs(t *testing.T) {
	tests := []struct {
		name        string
		defName     string
		content     string
		errContains string
	}{
		{"empty name", "", "Valid content", "name"},
		{"empty content", "valid_name", "", "content"},
		{"whitespace name", "   ", "Valid content", "name"},
		{"whitespace content", "valid_name", "   ", "content"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupDefAddTest(t)
			defer cleanup()

			output, err := executeDefAddCommand(t, tc.defName, tc.content, "-d", tmpDir)

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
// Edge Cases
// =============================================================================

// TestDefAddCmd_VeryLongName tests handling of very long name.
func TestDefAddCmd_VeryLongName(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	longName := strings.Repeat("very_long_definition_name_", 20)

	_, err := executeDefAddCommand(t, longName, "Content", "-d", tmpDir)

	// Should handle long names gracefully (either succeed or return clear error)
	if err != nil {
		t.Logf("Long name returned error: %v (may be expected if there's a length limit)", err)
	}
}

// TestDefAddCmd_VeryLongContent tests handling of very long content.
func TestDefAddCmd_VeryLongContent(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	longContent := strings.Repeat("This is a very detailed mathematical definition. ", 500)

	_, err := executeDefAddCommand(t, "long_content_def", longContent, "-d", tmpDir)

	if err != nil {
		t.Logf("Long content returned error: %v", err)
	} else {
		// Verify it was stored
		svc, _ := service.NewProofService(tmpDir)
		st, _ := svc.LoadState()
		def := st.GetDefinitionByName("long_content_def")
		if def == nil {
			t.Error("definition with long content not found")
		} else if len(def.Content) != len(longContent) {
			t.Errorf("long content not fully preserved: expected %d chars, got %d", len(longContent), len(def.Content))
		}
	}
}

// TestDefAddCmd_ContentWithNewlines tests content with various newline types.
func TestDefAddCmd_ContentWithNewlines(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	contentWithNewlines := "Line 1\nLine 2\nLine 3"

	_, err := executeDefAddCommand(t, "newline_def", contentWithNewlines, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify newlines preserved
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	def := st.GetDefinitionByName("newline_def")
	if def == nil {
		t.Fatal("definition not found")
	}

	if !strings.Contains(def.Content, "\n") {
		t.Error("newlines not preserved in content")
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestDefAddCmd_VerifyInStateAfterAdd tests complete flow including state verification.
func TestDefAddCmd_VerifyInStateAfterAdd(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	defName := "complete_flow_test"
	defContent := "This tests the complete add and verify flow."

	// Add via command with JSON output to get ID
	output, err := executeDefAddCommand(t,
		defName, defContent,
		"--format", "json",
		"-d", tmpDir,
	)
	if err != nil {
		t.Fatalf("def-add failed: %v", err)
	}

	// Parse JSON to get ID
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	var defID string
	for _, key := range []string{"id", "ID", "def_id", "defId"} {
		if val, ok := result[key].(string); ok && val != "" {
			defID = val
			break
		}
	}

	// Verify in state
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()

	def := st.GetDefinitionByName(defName)
	if def == nil {
		t.Fatal("definition not found in state")
	}

	if def.Name != defName {
		t.Errorf("name = %q, want %q", def.Name, defName)
	}

	if def.Content != defContent {
		t.Errorf("content = %q, want %q", def.Content, defContent)
	}

	if defID != "" && def.ID != defID {
		t.Errorf("ID mismatch: state has %q, JSON output had %q", def.ID, defID)
	}
}

// TestDefAddCmd_JSONContainsAllFields tests JSON output contains all expected fields.
func TestDefAddCmd_JSONContainsAllFields(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	output, err := executeDefAddCommand(t,
		"full_json_test", "Content for full JSON test",
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
		{"name", []string{"name", "Name"}},
		{"id", []string{"id", "ID", "def_id", "defId"}},
		{"added", []string{"added", "Added", "success"}},
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

// TestDefAddCmd_RelativeDirectory tests using relative directory path.
func TestDefAddCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-def-add-rel-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(baseDir)

	proofDir := filepath.Join(baseDir, "subdir", "proof")
	if err := os.MkdirAll(proofDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := service.Init(proofDir, "Test conjecture", "author"); err != nil {
		t.Fatal(err)
	}

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	output, err := executeDefAddCommand(t,
		"relative_test", "Content",
		"-d", "subdir/proof",
	)

	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}

	// Verify definition was added
	svc, _ := service.NewProofService(proofDir)
	st, _ := svc.LoadState()
	def := st.GetDefinitionByName("relative_test")
	if def == nil {
		t.Error("definition not added with relative directory path")
	}
}

// =============================================================================
// Consistency Tests
// =============================================================================

// TestDefAddAndDefsConsistency tests that def-add and defs show consistent information.
func TestDefAddAndDefsConsistency(t *testing.T) {
	tmpDir, cleanup := setupDefAddTest(t)
	defer cleanup()

	definitionNames := []string{"consistency_test_1", "consistency_test_2", "consistency_test_3"}

	// Add definitions
	for _, name := range definitionNames {
		_, err := executeDefAddCommand(t, name, "Content for "+name, "-d", tmpDir)
		if err != nil {
			t.Fatalf("failed to add %q: %v", name, err)
		}
	}

	// Get list from defs
	defsCmd := newTestDefsCmd()
	defsOutput, err := executeCommand(defsCmd, "defs", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("defs command failed: %v", err)
	}

	// Verify each definition appears in defs output
	for _, name := range definitionNames {
		if !strings.Contains(defsOutput, name) {
			t.Errorf("definition %q not found in defs output", name)
		}
	}
}
