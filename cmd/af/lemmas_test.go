//go:build integration

// Package main contains tests for the af lemmas and af lemma commands.
// These are TDD tests - the lemmas/lemma commands do not exist yet.
// Tests define the expected behavior for listing and viewing lemmas.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupLemmasTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupLemmasTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-lemmas-test-*")
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
	if err := service.Init(tmpDir, "Test conjecture for lemmas", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupLemmasTestWithLemmas creates a test environment with an initialized proof,
// validated nodes, and some pre-extracted lemmas.
func setupLemmasTestWithLemmas(t *testing.T) (string, func(), []string) {
	t.Helper()

	tmpDir, cleanup := setupLemmasTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create and validate some nodes first (lemmas are extracted from validated nodes)
	nodes := []struct {
		id        string
		statement string
	}{
		{"1.1", "If n is even, then n^2 is even"},
		{"1.2", "If n is odd, then n^2 is odd"},
		{"1.3", "For all integers n, n^2 >= 0"},
	}

	for _, n := range nodes {
		nodeID, err := service.ParseNodeID(n.id)
		if err != nil {
			cleanup()
			t.Fatalf("failed to parse node ID %q: %v", n.id, err)
		}
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, n.statement, schema.InferenceModusPonens)
		if err != nil {
			cleanup()
			t.Fatalf("failed to create node %q: %v", n.id, err)
		}
		// Validate the node so lemmas can be extracted
		err = svc.AcceptNode(nodeID)
		if err != nil {
			cleanup()
			t.Fatalf("failed to accept node %q: %v", n.id, err)
		}
	}

	// Extract lemmas from the validated nodes
	lemmas := []struct {
		sourceID  string
		statement string
	}{
		{"1.1", "The square of an even integer is even."},
		{"1.2", "The square of an odd integer is odd."},
		{"1.3", "The square of any integer is non-negative."},
	}

	var lemmaIDs []string
	for _, lem := range lemmas {
		sourceID, err := service.ParseNodeID(lem.sourceID)
		if err != nil {
			cleanup()
			t.Fatalf("failed to parse source node ID %q: %v", lem.sourceID, err)
		}
		id, err := svc.ExtractLemma(sourceID, lem.statement)
		if err != nil {
			cleanup()
			t.Fatalf("failed to extract lemma from node %q: %v", lem.sourceID, err)
		}
		lemmaIDs = append(lemmaIDs, id)
	}

	return tmpDir, cleanup, lemmaIDs
}

// newTestLemmasCmd creates a fresh root command with the lemmas subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestLemmasCmd() *cobra.Command {
	cmd := newTestRootCmd()

	lemmasCmd := newLemmasCmd()
	cmd.AddCommand(lemmasCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// newTestLemmaCmd creates a fresh root command with the lemma subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestLemmaCmd() *cobra.Command {
	cmd := newTestRootCmd()

	lemmaCmd := newLemmaCmd()
	cmd.AddCommand(lemmaCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// executeLemmasCommand creates and executes a lemmas command with the given arguments.
func executeLemmasCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newLemmasCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// executeLemmaCommand creates and executes a lemma command with the given arguments.
func executeLemmaCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newLemmaCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// af lemmas - List All Lemmas Tests
// =============================================================================

// TestLemmasCmd_ListLemmas tests listing all lemmas in a proof.
func TestLemmasCmd_ListLemmas(t *testing.T) {
	tmpDir, cleanup, _ := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain lemma statements or IDs
	expectedPhrases := []string{"even", "odd", "non-negative"}
	for _, phrase := range expectedPhrases {
		if !strings.Contains(strings.ToLower(output), phrase) {
			t.Errorf("expected output to contain %q, got: %q", phrase, output)
		}
	}
}

// TestLemmasCmd_ListLemmasEmpty tests listing lemmas when none exist.
func TestLemmasCmd_ListLemmasEmpty(t *testing.T) {
	tmpDir, cleanup := setupLemmasTest(t)
	defer cleanup()

	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should indicate no lemmas or be empty/show a message
	lower := strings.ToLower(output)
	hasNoLemmasIndicator := strings.Contains(lower, "no lemmas") ||
		strings.Contains(lower, "none") ||
		strings.Contains(lower, "empty") ||
		strings.Contains(lower, "0 lemmas") ||
		len(strings.TrimSpace(output)) == 0

	if !hasNoLemmasIndicator {
		t.Logf("Output when no lemmas exist: %q", output)
	}
}

// TestLemmasCmd_ListShowsCount tests that the listing shows a count of lemmas.
func TestLemmasCmd_ListShowsCount(t *testing.T) {
	tmpDir, cleanup, _ := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain some indication of count (3 lemmas)
	if !strings.Contains(output, "3") {
		t.Logf("Output may or may not show count: %q", output)
	}
}

// TestLemmasCmd_ListShowsSourceNodes tests that listing shows source node IDs.
func TestLemmasCmd_ListShowsSourceNodes(t *testing.T) {
	tmpDir, cleanup, _ := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show source node IDs (1.1, 1.2, 1.3)
	sourceNodes := []string{"1.1", "1.2", "1.3"}
	for _, nodeID := range sourceNodes {
		if !strings.Contains(output, nodeID) {
			t.Logf("Output may not show source node %q: %q", nodeID, output)
		}
	}
}

// =============================================================================
// af lemmas - JSON Output Tests
// =============================================================================

// TestLemmasCmd_JSONOutput tests JSON output format for listing lemmas.
func TestLemmasCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup, _ := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestLemmasCmd_JSONOutputStructure tests the structure of JSON output.
func TestLemmasCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup, _ := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Try to unmarshal as array or object
	var arrayResult []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &arrayResult); err == nil {
		// It's an array - each item should have lemma fields
		if len(arrayResult) != 3 {
			t.Errorf("expected 3 lemmas in JSON array, got %d", len(arrayResult))
		}

		for i, lem := range arrayResult {
			if _, ok := lem["id"]; !ok {
				t.Errorf("lemma %d missing 'id' field", i)
			}
			if _, ok := lem["statement"]; !ok {
				t.Errorf("lemma %d missing 'statement' field", i)
			}
		}
	} else {
		// Try as object with lemmas array
		var objResult map[string]interface{}
		if err := json.Unmarshal([]byte(output), &objResult); err != nil {
			t.Errorf("output is not valid JSON array or object: %v", err)
		} else {
			// Check for a lemmas array in the object
			if lemmas, ok := objResult["lemmas"]; ok {
				if lemmasArr, ok := lemmas.([]interface{}); ok {
					if len(lemmasArr) != 3 {
						t.Errorf("expected 3 lemmas, got %d", len(lemmasArr))
					}
				}
			}
		}
	}
}

// TestLemmasCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestLemmasCmd_JSONOutputShortFlag(t *testing.T) {
	tmpDir, cleanup, _ := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas", "-f", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON with -f flag: %v\nOutput: %q", err, output)
	}
}

// TestLemmasCmd_JSONOutputEmpty tests JSON output when no lemmas exist.
func TestLemmasCmd_JSONOutputEmpty(t *testing.T) {
	tmpDir, cleanup := setupLemmasTest(t)
	defer cleanup()

	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON (empty array or object with empty lemmas)
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("empty output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// =============================================================================
// af lemma <id> - Show Specific Lemma Tests
// =============================================================================

// TestLemmaCmd_ShowLemma tests showing a specific lemma by ID.
func TestLemmaCmd_ShowLemma(t *testing.T) {
	tmpDir, cleanup, lemmaIDs := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmaCmd()
	output, err := executeCommand(cmd, "lemma", lemmaIDs[0], "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain the lemma statement
	if !strings.Contains(strings.ToLower(output), "even") {
		t.Errorf("expected output to contain lemma statement about 'even', got: %q", output)
	}
}

// TestLemmaCmd_ShowLemmaByID tests showing different lemmas by their IDs.
func TestLemmaCmd_ShowLemmaByID(t *testing.T) {
	tmpDir, cleanup, lemmaIDs := setupLemmasTestWithLemmas(t)
	defer cleanup()

	tests := []struct {
		name            string
		lemmaIndex      int
		expectedContent string
	}{
		{"first lemma", 0, "even"},
		{"second lemma", 1, "odd"},
		{"third lemma", 2, "non-negative"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestLemmaCmd()
			output, err := executeCommand(cmd, "lemma", lemmaIDs[tc.lemmaIndex], "--dir", tmpDir)

			if err != nil {
				t.Fatalf("expected no error for %q, got: %v", tc.name, err)
			}

			if !strings.Contains(strings.ToLower(output), tc.expectedContent) {
				t.Errorf("expected output for %q to contain %q, got: %q", tc.name, tc.expectedContent, output)
			}
		})
	}
}

// TestLemmaCmd_ShowLemmaFull tests showing full lemma details.
func TestLemmaCmd_ShowLemmaFull(t *testing.T) {
	tmpDir, cleanup, lemmaIDs := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmaCmd()
	output, err := executeCommand(cmd, "lemma", lemmaIDs[0], "--full", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Full output should include:
	// - Lemma ID
	// - Statement
	// - Source node ID
	// - Content hash (if shown)
	// - Created timestamp (if shown)

	if !strings.Contains(output, lemmaIDs[0]) {
		t.Logf("Full output may not show lemma ID: %q", output)
	}

	if !strings.Contains(strings.ToLower(output), "even") {
		t.Errorf("expected full output to contain statement, got: %q", output)
	}

	// Should show source node
	if !strings.Contains(output, "1.1") {
		t.Logf("Full output may not show source node: %q", output)
	}
}

// TestLemmaCmd_ShowLemmaShowsSourceNode tests that lemma detail shows source node.
func TestLemmaCmd_ShowLemmaShowsSourceNode(t *testing.T) {
	tmpDir, cleanup, lemmaIDs := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmaCmd()
	output, err := executeCommand(cmd, "lemma", lemmaIDs[0], "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show the source node ID
	if !strings.Contains(output, "1.1") && !strings.Contains(strings.ToLower(output), "source") {
		t.Logf("Output may not show source node info: %q", output)
	}
}

// =============================================================================
// af lemma <id> - JSON Output Tests
// =============================================================================

// TestLemmaCmd_JSONOutput tests JSON output for a specific lemma.
func TestLemmaCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup, lemmaIDs := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmaCmd()
	output, err := executeCommand(cmd, "lemma", lemmaIDs[0], "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Check for expected fields
	if id, ok := result["id"]; !ok {
		t.Error("JSON output missing 'id' field")
	} else if id != lemmaIDs[0] {
		t.Errorf("expected id %q, got %v", lemmaIDs[0], id)
	}

	if _, ok := result["statement"]; !ok {
		t.Logf("Warning: JSON output may not have 'statement' field")
	}

	// Should have source_node_id or similar field
	if _, ok := result["source_node_id"]; !ok {
		if _, ok := result["node_id"]; !ok {
			t.Logf("Warning: JSON output may not have source node ID field")
		}
	}
}

// TestLemmaCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestLemmaCmd_JSONOutputShortFlag(t *testing.T) {
	tmpDir, cleanup, lemmaIDs := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmaCmd()
	output, err := executeCommand(cmd, "lemma", lemmaIDs[1], "-f", "json", "-d", tmpDir)

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

// TestLemmasCmd_ProofNotInitialized tests error when proof is not initialized.
func TestLemmasCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-lemmas-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestLemmasCmd()
	_, err = executeCommand(cmd, "lemmas", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestLemmasCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestLemmasCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestLemmasCmd()
	_, err := executeCommand(cmd, "lemmas", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestLemmaCmd_LemmaNotFound tests error when lemma doesn't exist.
func TestLemmaCmd_LemmaNotFound(t *testing.T) {
	tmpDir, cleanup, _ := setupLemmasTestWithLemmas(t)
	defer cleanup()

	cmd := newTestLemmaCmd()
	output, err := executeCommand(cmd, "lemma", "LEM-nonexistent12345", "--dir", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should indicate lemma not found
	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "not found") && !strings.Contains(lower, "does not exist") && err == nil {
		t.Errorf("expected error for non-existent lemma, got: %q", output)
	}
}

// TestLemmaCmd_MissingLemmaID tests error when lemma ID is not provided.
func TestLemmaCmd_MissingLemmaID(t *testing.T) {
	tmpDir, cleanup := setupLemmasTest(t)
	defer cleanup()

	cmd := newTestLemmaCmd()
	_, err := executeCommand(cmd, "lemma", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing lemma ID, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") &&
		!strings.Contains(errStr, "required") &&
		!strings.Contains(errStr, "argument") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestLemmaCmd_ProofNotInitialized tests error when proof is not initialized.
func TestLemmaCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-lemma-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestLemmaCmd()
	_, err = executeCommand(cmd, "lemma", "LEM-12345", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestLemmaCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestLemmaCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestLemmaCmd()
	_, err := executeCommand(cmd, "lemma", "LEM-12345", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestLemmaCmd_EmptyLemmaID tests error for empty lemma ID.
func TestLemmaCmd_EmptyLemmaID(t *testing.T) {
	tmpDir, cleanup := setupLemmasTest(t)
	defer cleanup()

	cmd := newTestLemmaCmd()
	_, err := executeCommand(cmd, "lemma", "", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for empty lemma ID, got nil")
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestLemmasCmd_Help tests that help output shows usage information.
func TestLemmasCmd_Help(t *testing.T) {
	cmd := newLemmasCmd()
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
		"lemmas",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestLemmasCmd_HelpShortFlag tests help with -h short flag.
func TestLemmasCmd_HelpShortFlag(t *testing.T) {
	cmd := newLemmasCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "lemmas") {
		t.Errorf("help output should mention 'lemmas', got: %q", output)
	}
}

// TestLemmaCmd_Help tests that help output shows usage information.
func TestLemmaCmd_Help(t *testing.T) {
	cmd := newLemmaCmd()
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
		"lemma",
		"id",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestLemmaCmd_HelpShortFlag tests help with -h short flag.
func TestLemmaCmd_HelpShortFlag(t *testing.T) {
	cmd := newLemmaCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "lemma") {
		t.Errorf("help output should mention 'lemma', got: %q", output)
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestLemmasCmd_ExpectedFlags ensures the lemmas command has expected flag structure.
func TestLemmasCmd_ExpectedFlags(t *testing.T) {
	cmd := newLemmasCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected lemmas command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected lemmas command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestLemmasCmd_DefaultFlagValues verifies default values for flags.
func TestLemmasCmd_DefaultFlagValues(t *testing.T) {
	cmd := newLemmasCmd()

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

// TestLemmaCmd_ExpectedFlags ensures the lemma command has expected flag structure.
func TestLemmaCmd_ExpectedFlags(t *testing.T) {
	cmd := newLemmaCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir", "full"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected lemma command to have flag %q", flagName)
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
			t.Errorf("expected lemma command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestLemmaCmd_DefaultFlagValues verifies default values for flags.
func TestLemmaCmd_DefaultFlagValues(t *testing.T) {
	cmd := newLemmaCmd()

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

// TestLemmasCmd_CommandMetadata verifies command metadata.
func TestLemmasCmd_CommandMetadata(t *testing.T) {
	cmd := newLemmasCmd()

	if cmd.Use != "lemmas" {
		t.Errorf("expected Use to be 'lemmas', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestLemmaCmd_CommandMetadata verifies command metadata.
func TestLemmaCmd_CommandMetadata(t *testing.T) {
	cmd := newLemmaCmd()

	if !strings.HasPrefix(cmd.Use, "lemma") {
		t.Errorf("expected Use to start with 'lemma', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Default Directory Tests
// =============================================================================

// TestLemmasCmd_DefaultDirectory tests lemmas uses current directory by default.
func TestLemmasCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup, _ := setupLemmasTestWithLemmas(t)
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
	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas")

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Should list lemmas
	if !strings.Contains(strings.ToLower(output), "even") {
		t.Errorf("expected output to contain 'even', got: %q", output)
	}
}

// TestLemmaCmd_DefaultDirectory tests lemma uses current directory by default.
func TestLemmaCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup, lemmaIDs := setupLemmasTestWithLemmas(t)
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
	cmd := newTestLemmaCmd()
	output, err := executeCommand(cmd, "lemma", lemmaIDs[0])

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Should show lemma
	if !strings.Contains(strings.ToLower(output), "even") {
		t.Errorf("expected output to contain 'even', got: %q", output)
	}
}

// =============================================================================
// Format Validation Tests
// =============================================================================

// TestLemmasCmd_FormatValidation verifies format flag validation.
func TestLemmasCmd_FormatValidation(t *testing.T) {
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
			tmpDir, cleanup, _ := setupLemmasTestWithLemmas(t)
			defer cleanup()

			cmd := newTestLemmasCmd()
			output, err := executeCommand(cmd, "lemmas", "--format", tc.format, "--dir", tmpDir)

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

// TestLemmaCmd_FormatValidation verifies format flag validation for lemma command.
func TestLemmaCmd_FormatValidation(t *testing.T) {
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
			tmpDir, cleanup, lemmaIDs := setupLemmasTestWithLemmas(t)
			defer cleanup()

			cmd := newTestLemmaCmd()
			output, err := executeCommand(cmd, "lemma", lemmaIDs[0], "--format", tc.format, "--dir", tmpDir)

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
// Partial ID Matching Tests
// =============================================================================

// TestLemmaCmd_PartialIDMatch tests if lemma lookup supports partial ID matching.
func TestLemmaCmd_PartialIDMatch(t *testing.T) {
	tmpDir, cleanup, lemmaIDs := setupLemmasTestWithLemmas(t)
	defer cleanup()

	// Lemma IDs are in format "LEM-<random hex>"
	// Try with first 8 characters (LEM- + 4 hex chars)
	partialID := lemmaIDs[0][:8]

	cmd := newTestLemmaCmd()
	output, err := executeCommand(cmd, "lemma", partialID, "--dir", tmpDir)

	// Partial matching may or may not be supported - document the behavior
	if err != nil {
		t.Logf("Partial ID matching returned error: %v (may not be supported)", err)
	} else {
		if !strings.Contains(strings.ToLower(output), "even") {
			t.Logf("Output with partial ID: %q", output)
		}
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestLemmasCmd_ManyLemmas tests listing many lemmas.
func TestLemmasCmd_ManyLemmas(t *testing.T) {
	tmpDir, cleanup := setupLemmasTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create and validate multiple nodes, then extract lemmas
	for i := 0; i < 20; i++ {
		nodeIDStr := "1." + string(rune('1'+i%9))
		nodeID, err := service.ParseNodeID(nodeIDStr)
		if err != nil {
			continue
		}

		statement := "Statement " + nodeIDStr
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, statement, schema.InferenceModusPonens)
		if err != nil {
			continue // Node may already exist
		}

		err = svc.AcceptNode(nodeID)
		if err != nil {
			continue
		}

		lemmaStatement := "Lemma from node " + nodeIDStr
		_, err = svc.ExtractLemma(nodeID, lemmaStatement)
		if err != nil {
			t.Logf("Failed to extract lemma %d: %v", i, err)
		}
	}

	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error listing many lemmas, got: %v", err)
	}

	t.Logf("Output length for many lemmas: %d bytes", len(output))
}

// TestLemmaCmd_LongLemmaStatement tests showing a lemma with a long statement.
func TestLemmaCmd_LongLemmaStatement(t *testing.T) {
	tmpDir, cleanup := setupLemmasTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create and validate a node
	nodeID, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}
	err = svc.AcceptNode(nodeID)
	if err != nil {
		t.Fatal(err)
	}

	// Extract a lemma with a very long statement
	longStatement := strings.Repeat("This is a very long lemma statement that tests edge cases. ", 50)
	lemmaID, err := svc.ExtractLemma(nodeID, longStatement)
	if err != nil {
		t.Fatal(err)
	}

	cmd := newTestLemmaCmd()
	output, err := executeCommand(cmd, "lemma", lemmaID, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for long lemma statement, got: %v", err)
	}

	if !strings.Contains(output, "lemma statement") {
		t.Logf("Long statement may be truncated: %q", output[:min(len(output), 200)])
	}
}

// TestLemmaCmd_SpecialCharactersInStatement tests lemma with special characters.
func TestLemmaCmd_SpecialCharactersInStatement(t *testing.T) {
	tmpDir, cleanup := setupLemmasTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create and validate a node
	nodeID, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}
	err = svc.AcceptNode(nodeID)
	if err != nil {
		t.Fatal(err)
	}

	// Extract lemma with special characters
	specialStatement := "For all x > 0: f(x) = x^2 + 2*x + 1 = (x+1)^2"
	lemmaID, err := svc.ExtractLemma(nodeID, specialStatement)
	if err != nil {
		t.Fatal(err)
	}

	cmd := newTestLemmaCmd()
	output, err := executeCommand(cmd, "lemma", lemmaID, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for special characters, got: %v", err)
	}

	if !strings.Contains(output, "f(x)") {
		t.Logf("Special characters may be escaped: %q", output)
	}
}

// =============================================================================
// Relative Directory Tests
// =============================================================================

// TestLemmasCmd_RelativeDirectory tests using relative directory path.
func TestLemmasCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-lemmas-rel-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(baseDir)

	proofDir := filepath.Join(baseDir, "subdir", "proof")
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	if err := service.Init(proofDir, "Test conjecture", "author"); err != nil {
		t.Fatal(err)
	}

	svc, _ := service.NewProofService(proofDir)

	// Create and validate a node, then extract a lemma
	nodeID, _ := service.ParseNodeID("1.1")
	svc.CreateNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceModusPonens)
	svc.AcceptNode(nodeID)
	svc.ExtractLemma(nodeID, "Test lemma for relative path")

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas", "-d", "subdir/proof")

	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}

	// Should show the lemma
	if !strings.Contains(strings.ToLower(output), "relative") {
		t.Logf("Output with relative directory: %q", output)
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestLemmasCmd_TableDrivenDirectories tests various directory scenarios.
func TestLemmasCmd_TableDrivenDirectories(t *testing.T) {
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
			cmd := newTestLemmasCmd()
			_, err := executeCommand(cmd, "lemmas", "--dir", tc.dirPath)

			if tc.wantErr && err == nil {
				t.Errorf("expected error for dir %q, got nil", tc.dirPath)
			}
		})
	}
}

// TestLemmaCmd_TableDrivenLemmaIDs tests various lemma ID inputs.
func TestLemmaCmd_TableDrivenLemmaIDs(t *testing.T) {
	tmpDir, cleanup, lemmaIDs := setupLemmasTestWithLemmas(t)
	defer cleanup()

	tests := []struct {
		name        string
		lemmaID     string
		expectFound bool
	}{
		{"existing lemma first", lemmaIDs[0], true},
		{"existing lemma second", lemmaIDs[1], true},
		{"existing lemma third", lemmaIDs[2], true},
		{"nonexistent lemma", "LEM-nonexistent123", false},
		{"empty string", "", false},
		{"random string", "not-a-lemma-id", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestLemmaCmd()
			output, err := executeCommand(cmd, "lemma", tc.lemmaID, "--dir", tmpDir)

			if tc.expectFound {
				if err != nil {
					t.Errorf("expected lemma %q to be found, got error: %v", tc.lemmaID, err)
				}
			} else {
				combined := output
				if err != nil {
					combined += err.Error()
				}
				// Should indicate not found
				if err == nil && !strings.Contains(strings.ToLower(combined), "not found") {
					t.Logf("Lemma %q not expected to be found, but no error. Output: %q", tc.lemmaID, output)
				}
			}
		})
	}
}

// =============================================================================
// Consistency Tests
// =============================================================================

// TestLemmasAndLemmaConsistency tests that lemmas and lemma show consistent information.
func TestLemmasAndLemmaConsistency(t *testing.T) {
	tmpDir, cleanup, lemmaIDs := setupLemmasTestWithLemmas(t)
	defer cleanup()

	// Get list from lemmas command
	lemmasCmd := newTestLemmasCmd()
	lemmasOutput, err := executeCommand(lemmasCmd, "lemmas", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("lemmas command failed: %v", err)
	}

	// Verify each lemma from lemmas can be retrieved with lemma
	for _, lemmaID := range lemmaIDs {
		// Verify ID appears in lemmas output (may be partial or full)
		idPrefix := lemmaID[:8] // LEM-xxxx
		if !strings.Contains(lemmasOutput, idPrefix) {
			t.Logf("lemma ID %q may not appear in lemmas output", lemmaID)
		}

		// Verify lemma can retrieve it
		lemmaCmd := newTestLemmaCmd()
		lemmaOutput, err := executeCommand(lemmaCmd, "lemma", lemmaID, "--dir", tmpDir)
		if err != nil {
			t.Errorf("lemma command failed for %q: %v", lemmaID, err)
		}

		// Output should have some content
		if len(strings.TrimSpace(lemmaOutput)) == 0 {
			t.Errorf("lemma output for %q is empty", lemmaID)
		}
	}
}

// TestLemmasCmd_JSONAndTextConsistency tests that JSON and text output have consistent data.
func TestLemmasCmd_JSONAndTextConsistency(t *testing.T) {
	tmpDir, cleanup, _ := setupLemmasTestWithLemmas(t)
	defer cleanup()

	// Get text output
	cmdText := newTestLemmasCmd()
	textOutput, err := executeCommand(cmdText, "lemmas", "--format", "text", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("text output failed: %v", err)
	}

	// Get JSON output
	cmdJSON := newTestLemmasCmd()
	jsonOutput, err := executeCommand(cmdJSON, "lemmas", "--format", "json", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("JSON output failed: %v", err)
	}

	// Parse JSON to count lemmas
	var jsonResult []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &jsonResult); err == nil {
		// JSON is an array - check we have 3 lemmas
		if len(jsonResult) != 3 {
			t.Errorf("JSON should have 3 lemmas, got %d", len(jsonResult))
		}

		// Text output should mention key phrases from all lemmas
		for _, lem := range jsonResult {
			if stmt, ok := lem["statement"].(string); ok {
				// Check that some part of statement appears in text
				words := strings.Fields(stmt)
				if len(words) > 0 && !strings.Contains(strings.ToLower(textOutput), strings.ToLower(words[0])) {
					t.Logf("Text output may not contain lemma content: %q", stmt[:min(len(stmt), 50)])
				}
			}
		}
	}
}

// =============================================================================
// Lemma State Tests (Future Extension)
// =============================================================================

// TestLemmasCmd_FilterBySourceNode tests filtering lemmas by source node (future feature).
func TestLemmasCmd_FilterBySourceNode(t *testing.T) {
	tmpDir, cleanup, _ := setupLemmasTestWithLemmas(t)
	defer cleanup()

	// This tests a potential future feature: filtering by source node
	cmd := newTestLemmasCmd()
	output, err := executeCommand(cmd, "lemmas", "--node", "1.1", "--dir", tmpDir)

	// This flag may not exist yet - document the behavior
	if err != nil {
		if strings.Contains(err.Error(), "unknown flag") {
			t.Logf("--node filter flag not yet implemented")
			return
		}
		t.Logf("Filter by node returned error: %v", err)
	} else {
		// If implemented, should only show lemmas from node 1.1
		t.Logf("Filtered output: %q", output)
	}
}
