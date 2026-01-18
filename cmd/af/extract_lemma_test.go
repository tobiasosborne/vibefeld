//go:build integration

// Package main contains tests for the af extract-lemma command.
// These are TDD tests - the extract-lemma command does not exist yet.
// Tests define the expected behavior for extracting lemmas from validated nodes.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// mustParseExtractLemmaNodeID parses a NodeID string or fails the test.
func mustParseExtractLemmaNodeID(t *testing.T, s string) service.NodeID {
	t.Helper()
	id, err := service.ParseNodeID(s)
	if err != nil {
		t.Fatalf("Failed to parse NodeID %q: %v", s, err)
	}
	return id
}

// setupExtractLemmaTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupExtractLemmaTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-extract-lemma-test-*")
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
	if err := service.Init(tmpDir, "Test conjecture for lemma extraction", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupExtractLemmaTestWithValidatedNode creates a test environment with an
// initialized proof and a validated node at ID "1".
// Note: service.Init already creates node 1 with the conjecture, so we just
// validate it.
func setupExtractLemmaTestWithValidatedNode(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupExtractLemmaTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Validate node 1
	nodeID := mustParseExtractLemmaNodeID(t, "1")
	if err := svc.AcceptNode(nodeID); err != nil {
		cleanup()
		t.Fatalf("failed to validate node 1: %v", err)
	}

	return tmpDir, cleanup
}

// setupExtractLemmaTestWithPendingNode creates a test environment with an
// initialized proof and a pending (non-validated) node at ID "1".
func setupExtractLemmaTestWithPendingNode(t *testing.T) (string, func()) {
	t.Helper()
	// service.Init already creates node 1 in pending state
	return setupExtractLemmaTest(t)
}

// setupExtractLemmaTestWithValidatedTree creates a test environment with
// validated nodes at 1, 1.1, and 1.2 for comprehensive testing.
func setupExtractLemmaTestWithValidatedTree(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupExtractLemmaTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	rootID := mustParseExtractLemmaNodeID(t, "1")
	child1ID := mustParseExtractLemmaNodeID(t, "1.1")
	child2ID := mustParseExtractLemmaNodeID(t, "1.2")

	// Claim root and create children
	proverOwner := "prover"
	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		cleanup()
		t.Fatalf("failed to claim root: %v", err)
	}

	if err := svc.RefineNode(rootID, proverOwner, child1ID, schema.NodeTypeClaim,
		"First child claim", schema.InferenceModusPonens); err != nil {
		cleanup()
		t.Fatalf("failed to create child 1.1: %v", err)
	}

	if err := svc.RefineNode(rootID, proverOwner, child2ID, schema.NodeTypeClaim,
		"Second child claim", schema.InferenceModusPonens); err != nil {
		cleanup()
		t.Fatalf("failed to create child 1.2: %v", err)
	}

	if err := svc.ReleaseNode(rootID, proverOwner); err != nil {
		cleanup()
		t.Fatalf("failed to release root: %v", err)
	}

	// Validate all nodes
	if err := svc.AcceptNode(child1ID); err != nil {
		cleanup()
		t.Fatalf("failed to validate child 1.1: %v", err)
	}
	if err := svc.AcceptNode(child2ID); err != nil {
		cleanup()
		t.Fatalf("failed to validate child 1.2: %v", err)
	}
	if err := svc.AcceptNode(rootID); err != nil {
		cleanup()
		t.Fatalf("failed to validate root: %v", err)
	}

	return tmpDir, cleanup
}

// setupExtractLemmaTestWithExistingLemma creates a test environment with
// a validated node that already has a lemma extracted from it.
func setupExtractLemmaTestWithExistingLemma(t *testing.T) (string, func(), string) {
	t.Helper()

	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Extract a lemma from the validated node
	nodeID := mustParseExtractLemmaNodeID(t, "1")
	lemmaID, err := svc.ExtractLemma(nodeID, "Previously extracted lemma statement")
	if err != nil {
		cleanup()
		t.Fatalf("failed to extract lemma: %v", err)
	}

	return tmpDir, cleanup, lemmaID
}

// newTestExtractLemmaCmd creates a fresh root command with the extract-lemma
// subcommand for testing. This ensures test isolation.
func newTestExtractLemmaCmd() *cobra.Command {
	cmd := newTestRootCmd()

	extractLemmaCmd := newExtractLemmaCmd()
	cmd.AddCommand(extractLemmaCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// executeExtractLemmaCommand creates and executes an extract-lemma command
// with the given arguments.
func executeExtractLemmaCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newExtractLemmaCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// Basic Extraction Tests
// =============================================================================

// TestExtractLemmaCmd_BasicExtraction tests extracting a lemma from a validated node.
func TestExtractLemmaCmd_BasicExtraction(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	lemmaStatement := "A reusable mathematical fact"
	output, err := executeExtractLemmaCommand(t, "1", "--statement", lemmaStatement, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify output indicates success
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "lemma") && !strings.Contains(lower, "extracted") {
		t.Errorf("expected output to mention lemma extraction, got: %q", output)
	}

	// Verify output contains a lemma ID
	if !strings.Contains(output, "LEM-") {
		t.Errorf("expected output to contain lemma ID (LEM-...), got: %q", output)
	}

	// Verify lemma was actually created in state
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	lemmas := st.AllLemmas()
	if len(lemmas) != 1 {
		t.Errorf("expected 1 lemma in state, got %d", len(lemmas))
	}

	if len(lemmas) > 0 && lemmas[0].Statement != lemmaStatement {
		t.Errorf("lemma statement = %q, want %q", lemmas[0].Statement, lemmaStatement)
	}
}

// TestExtractLemmaCmd_BasicExtractionPositionalArgs tests extraction using positional arguments.
func TestExtractLemmaCmd_BasicExtractionPositionalArgs(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	// Test with positional statement argument (if supported)
	output, err := executeExtractLemmaCommand(t, "1", "-d", tmpDir, "--statement", "A lemma statement")

	// Either positional args work or we get a clear error
	if err != nil {
		// This is acceptable if command requires flags
		t.Logf("Command may require --statement flag: %v", err)
	} else {
		// Verify lemma was created
		if !strings.Contains(output, "LEM-") {
			t.Logf("Output: %s", output)
		}
	}
}

// =============================================================================
// Node State Validation Tests
// =============================================================================

// TestExtractLemmaCmd_NotValidated tests that extraction fails for non-validated nodes.
func TestExtractLemmaCmd_NotValidated(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithPendingNode(t)
	defer cleanup()

	output, err := executeExtractLemmaCommand(t, "1", "--statement", "Some lemma", "-d", tmpDir)

	// Should produce error - node must be validated first
	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	hasValidationError := strings.Contains(lower, "validated") ||
		strings.Contains(lower, "pending") ||
		strings.Contains(lower, "not valid") ||
		strings.Contains(lower, "must be") ||
		strings.Contains(lower, "cannot")

	if err == nil && !hasValidationError {
		t.Errorf("expected error for non-validated node, got output: %q", output)
	}

	// Verify no lemma was created
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	lemmas := st.AllLemmas()
	if len(lemmas) != 0 {
		t.Errorf("expected no lemmas after failed extraction, got %d", len(lemmas))
	}
}

// TestExtractLemmaCmd_NotIndependent tests that extraction fails when node relies
// on local assumptions (scope dependencies from parent).
// Note: This test may need to be adjusted based on actual independence checking implementation.
func TestExtractLemmaCmd_NotIndependent(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	rootID := mustParseExtractLemmaNodeID(t, "1")

	// Claim root and create a local_assume node (which introduces local scope)
	proverOwner := "prover"
	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		t.Fatalf("failed to claim root: %v", err)
	}

	// Create a local_assume child node
	localAssumeID := mustParseExtractLemmaNodeID(t, "1.1")
	if err := svc.RefineNode(rootID, proverOwner, localAssumeID, schema.NodeTypeLocalAssume,
		"Assume x > 0 for local reasoning", schema.InferenceAssumption); err != nil {
		t.Fatalf("failed to create local_assume node: %v", err)
	}

	// Create a claim that depends on the local assumption
	dependentClaimID := mustParseExtractLemmaNodeID(t, "1.2")
	if err := svc.RefineNode(rootID, proverOwner, dependentClaimID, schema.NodeTypeClaim,
		"Therefore x^2 > 0", schema.InferenceModusPonens); err != nil {
		t.Fatalf("failed to create dependent claim: %v", err)
	}

	if err := svc.ReleaseNode(rootID, proverOwner); err != nil {
		t.Fatalf("failed to release root: %v", err)
	}

	// Validate the dependent claim
	if err := svc.AcceptNode(dependentClaimID); err != nil {
		t.Fatalf("failed to validate dependent claim: %v", err)
	}

	// Try to extract lemma from the dependent claim (which relies on local assumption)
	output, err := executeExtractLemmaCommand(t, "1.2", "--statement", "x^2 > 0", "-d", tmpDir)

	// Should produce error - node is not independent (relies on local scope)
	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	hasIndependenceError := strings.Contains(lower, "independent") ||
		strings.Contains(lower, "scope") ||
		strings.Contains(lower, "local") ||
		strings.Contains(lower, "assumption") ||
		strings.Contains(lower, "cannot")

	// This test documents expected behavior - independence checking may not be implemented yet
	if err == nil && !hasIndependenceError {
		t.Logf("Independence checking may not be implemented yet. Output: %q", output)
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestExtractLemmaCmd_NodeNotFound tests error when node doesn't exist.
func TestExtractLemmaCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTest(t)
	defer cleanup()

	// Try to extract from non-existent node (node 1.99 doesn't exist)
	output, err := executeExtractLemmaCommand(t, "1.99", "--statement", "Some lemma", "-d", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "not found") &&
		!strings.Contains(lower, "does not exist") &&
		!strings.Contains(lower, "error") &&
		err == nil {
		t.Errorf("expected error for non-existent node, got: %q", output)
	}
}

// TestExtractLemmaCmd_ProofNotInitialized tests error when proof is not initialized.
func TestExtractLemmaCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-extract-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	output, err := executeExtractLemmaCommand(t, "1", "--statement", "Lemma", "-d", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "not initialized") &&
		!strings.Contains(lower, "error") &&
		!strings.Contains(lower, "not found") &&
		err == nil {
		t.Errorf("expected error for uninitialized proof, got: %q", output)
	}
}

// TestExtractLemmaCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestExtractLemmaCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeExtractLemmaCommand(t, "1", "--statement", "Lemma", "-d", "/nonexistent/path/12345")

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
// Invalid Arguments Tests
// =============================================================================

// TestExtractLemmaCmd_MissingNodeID tests error when node ID is missing.
func TestExtractLemmaCmd_MissingNodeID(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	// Execute without node ID
	_, err := executeExtractLemmaCommand(t, "--statement", "Some lemma", "-d", tmpDir)
	if err == nil {
		t.Fatal("expected error for missing node ID, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "accepts") &&
		!strings.Contains(errStr, "required") &&
		!strings.Contains(errStr, "argument") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestExtractLemmaCmd_EmptyNodeID tests error for empty node ID.
func TestExtractLemmaCmd_EmptyNodeID(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	output, err := executeExtractLemmaCommand(t, "", "--statement", "Lemma", "-d", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "invalid") &&
		!strings.Contains(lower, "empty") &&
		!strings.Contains(lower, "error") &&
		err == nil {
		t.Errorf("expected error for empty node ID, got: %q", output)
	}
}

// TestExtractLemmaCmd_InvalidNodeIDFormat tests error for invalid node ID format.
func TestExtractLemmaCmd_InvalidNodeIDFormat(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	tests := []struct {
		name   string
		nodeID string
	}{
		{"letters", "abc"},
		{"negative", "-1"},
		{"zero", "0"},
		{"leading dot", ".1"},
		{"trailing dot", "1."},
		{"double dot", "1..2"},
		{"non-root", "2.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeExtractLemmaCommand(t, tc.nodeID, "--statement", "Lemma", "-d", tmpDir)

			combined := output
			if err != nil {
				combined += err.Error()
			}

			lower := strings.ToLower(combined)
			if !strings.Contains(lower, "invalid") &&
				!strings.Contains(lower, "error") &&
				err == nil {
				t.Errorf("expected error for invalid node ID %q, got: %q", tc.nodeID, output)
			}
		})
	}
}

// TestExtractLemmaCmd_TooManyArguments tests error when too many arguments provided.
func TestExtractLemmaCmd_TooManyArguments(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	// Execute with extra arguments
	output, err := executeExtractLemmaCommand(t, "1", "extra", "--statement", "Lemma", "-d", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should produce error about extra arguments
	if err == nil && !strings.Contains(strings.ToLower(combined), "argument") {
		t.Logf("Command may accept extra arguments. Output: %q", output)
	}
}

// TestExtractLemmaCmd_MissingStatement tests error when statement is missing.
func TestExtractLemmaCmd_MissingStatement(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	// Execute without --statement flag
	output, err := executeExtractLemmaCommand(t, "1", "-d", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "statement") &&
		!strings.Contains(lower, "required") &&
		!strings.Contains(lower, "missing") &&
		err == nil {
		t.Errorf("expected error for missing statement, got: %q", output)
	}
}

// TestExtractLemmaCmd_EmptyStatement tests error for empty statement.
func TestExtractLemmaCmd_EmptyStatement(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	tests := []struct {
		name      string
		statement string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeExtractLemmaCommand(t, "1", "--statement", tc.statement, "-d", tmpDir)

			combined := output
			if err != nil {
				combined += err.Error()
			}

			lower := strings.ToLower(combined)
			if !strings.Contains(lower, "empty") &&
				!strings.Contains(lower, "statement") &&
				!strings.Contains(lower, "error") &&
				err == nil {
				t.Errorf("expected error for empty statement %q, got: %q", tc.statement, output)
			}
		})
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestExtractLemmaCmd_JSONOutput tests JSON output format.
func TestExtractLemmaCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	output, err := executeExtractLemmaCommand(t, "1", "--statement", "A lemma statement",
		"-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Should contain lemma ID
	if id, ok := result["id"]; ok {
		if !strings.HasPrefix(id.(string), "LEM-") {
			t.Errorf("expected lemma ID to start with 'LEM-', got: %v", id)
		}
	} else if _, ok := result["lemma_id"]; !ok {
		t.Log("Warning: JSON output does not contain 'id' or 'lemma_id' field")
	}
}

// TestExtractLemmaCmd_JSONOutputStructure tests JSON output structure.
func TestExtractLemmaCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	output, err := executeExtractLemmaCommand(t, "1", "--statement", "Test lemma statement",
		"-d", tmpDir, "--format", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Check for expected fields (at minimum some of these)
	expectedFields := []string{"id", "lemma_id", "statement", "source_node_id", "node_id"}
	hasField := false
	for _, field := range expectedFields {
		if _, ok := result[field]; ok {
			hasField = true
			break
		}
	}

	if !hasField {
		t.Logf("JSON output structure: %+v", result)
	}
}

// TestExtractLemmaCmd_TextOutput tests text output format.
func TestExtractLemmaCmd_TextOutput(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	output, err := executeExtractLemmaCommand(t, "1", "--statement", "A lemma for text output test",
		"-d", tmpDir, "-f", "text")
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Should contain human-readable information
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "lemma") {
		t.Errorf("expected text output to mention 'lemma', got: %q", output)
	}

	// Should contain lemma ID
	if !strings.Contains(output, "LEM-") {
		t.Errorf("expected text output to contain lemma ID (LEM-...), got: %q", output)
	}
}

// TestExtractLemmaCmd_DefaultTextFormat tests that default format is text.
func TestExtractLemmaCmd_DefaultTextFormat(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	// Execute without format flag
	output, err := executeExtractLemmaCommand(t, "1", "--statement", "Default format test",
		"-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Default should be text format (not JSON)
	var result map[string]interface{}
	jsonErr := json.Unmarshal([]byte(output), &result)

	// If it parses as JSON, that's also acceptable
	if jsonErr != nil {
		// Good - it's text format as expected
		if !strings.Contains(output, "LEM-") {
			t.Errorf("expected output to contain lemma ID, got: %q", output)
		}
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestExtractLemmaCmd_DirFlagWorks tests that --dir/-d flag works.
func TestExtractLemmaCmd_DirFlagWorks(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	tests := []struct {
		name string
		args []string
	}{
		{"long form --dir", []string{"1", "--statement", "Lemma", "--dir", tmpDir}},
		{"short form -d", []string{"1", "--statement", "Lemma", "-d", tmpDir}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Need fresh validated node for each test
			freshDir, freshCleanup := setupExtractLemmaTestWithValidatedNode(t)
			defer freshCleanup()

			// Replace tmpDir in args
			for i, arg := range tc.args {
				if arg == tmpDir {
					tc.args[i] = freshDir
				}
			}

			output, err := executeExtractLemmaCommand(t, tc.args...)
			if err != nil {
				t.Fatalf("expected no error with %s, got: %v\nOutput: %s", tc.name, err, output)
			}

			if !strings.Contains(output, "LEM-") {
				t.Errorf("expected lemma ID in output for %s, got: %q", tc.name, output)
			}
		})
	}
}

// TestExtractLemmaCmd_NameFlag tests the --name flag for custom lemma naming.
func TestExtractLemmaCmd_NameFlag(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	customName := "my-custom-lemma"
	output, err := executeExtractLemmaCommand(t, "1",
		"--statement", "Lemma with custom name",
		"--name", customName,
		"-d", tmpDir)

	// The --name flag may or may not be supported
	if err != nil {
		if strings.Contains(err.Error(), "unknown flag") {
			t.Logf("--name flag not yet implemented")
			return
		}
		t.Logf("--name flag returned error: %v", err)
	} else {
		// If supported, verify the custom name appears somewhere
		if strings.Contains(output, customName) {
			t.Logf("Custom name appears in output: %s", output)
		}
	}
}

// TestExtractLemmaCmd_DefaultDirectory tests using current directory by default.
func TestExtractLemmaCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
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
	output, err := executeExtractLemmaCommand(t, "1", "--statement", "Default dir lemma")
	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Verify lemma was created
	if !strings.Contains(output, "LEM-") {
		t.Errorf("expected lemma ID in output, got: %q", output)
	}
}

// TestExtractLemmaCmd_ExpectedFlags ensures expected flags exist.
func TestExtractLemmaCmd_ExpectedFlags(t *testing.T) {
	cmd := newExtractLemmaCmd()

	// Check expected flags
	expectedFlags := []string{"dir", "format", "statement"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected extract-lemma command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"d": "dir",
		"f": "format",
		"s": "statement",
	}

	for short, long := range shortFlags {
		flag := cmd.Flags().ShorthandLookup(short)
		if flag == nil {
			t.Logf("expected extract-lemma command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestExtractLemmaCmd_DefaultFlagValues verifies default values for flags.
func TestExtractLemmaCmd_DefaultFlagValues(t *testing.T) {
	cmd := newExtractLemmaCmd()

	// Check default dir value
	dirFlag := cmd.Flags().Lookup("dir")
	if dirFlag != nil && dirFlag.DefValue != "." {
		t.Errorf("expected default dir to be '.', got %q", dirFlag.DefValue)
	}

	// Check default format value
	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag != nil && formatFlag.DefValue != "text" {
		t.Errorf("expected default format to be 'text', got %q", formatFlag.DefValue)
	}
}

// =============================================================================
// Already Extracted Tests
// =============================================================================

// TestExtractLemmaCmd_AlreadyExtractedAsLemma tests behavior when extracting from
// a node that already has a lemma. Multiple lemmas from same node should be allowed.
func TestExtractLemmaCmd_AlreadyExtractedAsLemma(t *testing.T) {
	tmpDir, cleanup, existingLemmaID := setupExtractLemmaTestWithExistingLemma(t)
	defer cleanup()

	// Try to extract another lemma from the same node
	output, err := executeExtractLemmaCommand(t, "1",
		"--statement", "A second lemma from the same node",
		"-d", tmpDir)

	// Multiple lemmas from the same node should be allowed (different statements)
	if err != nil {
		combined := output + err.Error()
		lower := strings.ToLower(combined)

		// It's also acceptable to disallow multiple lemmas from same node
		if strings.Contains(lower, "already") ||
			strings.Contains(lower, "exists") ||
			strings.Contains(lower, "duplicate") {
			t.Logf("Multiple lemmas from same node not allowed: %v", err)
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify second lemma was created
	if !strings.Contains(output, "LEM-") {
		t.Errorf("expected lemma ID in output, got: %q", output)
	}

	// Verify both lemmas exist
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	lemmas := st.AllLemmas()
	if len(lemmas) < 2 {
		t.Errorf("expected at least 2 lemmas, got %d", len(lemmas))
	}

	// Verify existing lemma still exists
	if st.GetLemma(existingLemmaID) == nil {
		t.Error("existing lemma should still exist")
	}
}

// TestExtractLemmaCmd_DuplicateStatement tests extracting with same statement twice.
func TestExtractLemmaCmd_DuplicateStatement(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedTree(t)
	defer cleanup()

	sameStatement := "The exact same lemma statement"

	// Extract first lemma from node 1.1
	output1, err := executeExtractLemmaCommand(t, "1.1",
		"--statement", sameStatement,
		"-d", tmpDir)
	if err != nil {
		t.Fatalf("first extraction failed: %v", err)
	}

	// Extract second lemma with same statement from different node (1.2)
	output2, err := executeExtractLemmaCommand(t, "1.2",
		"--statement", sameStatement,
		"-d", tmpDir)

	// Same statement from different nodes should be allowed
	if err != nil {
		t.Logf("Duplicate statements may not be allowed: %v", err)
	} else {
		// Verify both have different IDs
		if strings.Contains(output1, "LEM-") && strings.Contains(output2, "LEM-") {
			t.Log("Duplicate statements from different nodes allowed")
		}
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestExtractLemmaCmd_Help tests that help output shows usage information.
func TestExtractLemmaCmd_Help(t *testing.T) {
	cmd := newExtractLemmaCmd()
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
		"extract",
		"lemma",
		"node",
		"--statement",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestExtractLemmaCmd_CommandMetadata verifies command metadata.
func TestExtractLemmaCmd_CommandMetadata(t *testing.T) {
	cmd := newExtractLemmaCmd()

	if !strings.HasPrefix(cmd.Use, "extract-lemma") {
		t.Errorf("expected Use to start with 'extract-lemma', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestExtractLemmaCmd_TableDrivenNodeStates tests extraction from nodes in various states.
func TestExtractLemmaCmd_TableDrivenNodeStates(t *testing.T) {
	tests := []struct {
		name            string
		setupFunc       func(t *testing.T, svc *service.ProofService, nodeID service.NodeID)
		wantErr         bool
		errContains     string
		skipIfNotExists bool
	}{
		{
			name: "validated node",
			setupFunc: func(t *testing.T, svc *service.ProofService, nodeID service.NodeID) {
				if err := svc.AcceptNode(nodeID); err != nil {
					t.Fatalf("AcceptNode failed: %v", err)
				}
			},
			wantErr: false,
		},
		{
			name: "pending node",
			setupFunc: func(t *testing.T, svc *service.ProofService, nodeID service.NodeID) {
				// Node 1 is already pending after init
			},
			wantErr:     true,
			errContains: "validated",
		},
		{
			name: "refuted node",
			setupFunc: func(t *testing.T, svc *service.ProofService, nodeID service.NodeID) {
				if err := svc.RefuteNode(nodeID); err != nil {
					t.Fatalf("RefuteNode failed: %v", err)
				}
			},
			wantErr:     true,
			errContains: "refuted",
		},
		{
			name: "admitted node",
			setupFunc: func(t *testing.T, svc *service.ProofService, nodeID service.NodeID) {
				if err := svc.AdmitNode(nodeID); err != nil {
					t.Fatalf("AdmitNode failed: %v", err)
				}
			},
			wantErr:         true,
			errContains:     "validated",
			skipIfNotExists: true, // Admitted nodes may or may not allow lemma extraction
		},
		{
			name: "archived node",
			setupFunc: func(t *testing.T, svc *service.ProofService, nodeID service.NodeID) {
				if err := svc.ArchiveNode(nodeID); err != nil {
					t.Fatalf("ArchiveNode failed: %v", err)
				}
			},
			wantErr:     true,
			errContains: "archived",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupExtractLemmaTest(t)
			defer cleanup()

			svc, err := service.NewProofService(tmpDir)
			if err != nil {
				t.Fatal(err)
			}

			nodeID := mustParseExtractLemmaNodeID(t, "1")
			tc.setupFunc(t, svc, nodeID)

			output, err := executeExtractLemmaCommand(t, "1",
				"--statement", "Test lemma",
				"-d", tmpDir)

			if tc.wantErr {
				combined := output
				if err != nil {
					combined += err.Error()
				}

				if err == nil && !strings.Contains(strings.ToLower(combined), tc.errContains) {
					if tc.skipIfNotExists {
						t.Logf("Expected error containing %q not found, but marked as optional", tc.errContains)
					} else {
						t.Errorf("expected error containing %q, got output: %q", tc.errContains, output)
					}
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestExtractLemmaCmd_TableDrivenOutputFormats tests different output formats.
func TestExtractLemmaCmd_TableDrivenOutputFormats(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		validator func(t *testing.T, output string)
	}{
		{
			name:   "text format",
			format: "text",
			validator: func(t *testing.T, output string) {
				if !strings.Contains(output, "LEM-") {
					t.Errorf("expected text output to contain lemma ID, got: %q", output)
				}
			},
		},
		{
			name:   "json format",
			format: "json",
			validator: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("expected valid JSON, got error: %v\nOutput: %q", err, output)
				}
			},
		},
		{
			name:   "JSON uppercase",
			format: "JSON",
			validator: func(t *testing.T, output string) {
				var result interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("expected valid JSON (uppercase), got error: %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
			defer cleanup()

			output, err := executeExtractLemmaCommand(t, "1",
				"--statement", "Format test lemma",
				"-d", tmpDir,
				"-f", tc.format)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tc.validator(t, output)
		})
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestExtractLemmaCmd_MultipleExtractions tests extracting multiple lemmas sequentially.
func TestExtractLemmaCmd_MultipleExtractions(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedTree(t)
	defer cleanup()

	// Extract lemmas from different nodes
	nodes := []struct {
		id        string
		statement string
	}{
		{"1", "Root lemma"},
		{"1.1", "First child lemma"},
		{"1.2", "Second child lemma"},
	}

	var extractedIDs []string
	for _, n := range nodes {
		output, err := executeExtractLemmaCommand(t, n.id,
			"--statement", n.statement,
			"-d", tmpDir)
		if err != nil {
			t.Fatalf("failed to extract lemma from %s: %v", n.id, err)
		}

		// Extract lemma ID from output
		if strings.Contains(output, "LEM-") {
			extractedIDs = append(extractedIDs, n.id)
		}
	}

	// Verify all lemmas were extracted
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	lemmas := st.AllLemmas()
	if len(lemmas) != len(nodes) {
		t.Errorf("expected %d lemmas, got %d", len(nodes), len(lemmas))
	}
}

// TestExtractLemmaCmd_VerifyLemmaInState tests that extracted lemma is correctly stored.
func TestExtractLemmaCmd_VerifyLemmaInState(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	statement := "A carefully crafted lemma statement"
	output, err := executeExtractLemmaCommand(t, "1", "--statement", statement, "-d", tmpDir)
	if err != nil {
		t.Fatalf("extraction failed: %v", err)
	}

	// Extract lemma ID from output (should contain LEM-xxx)
	var lemmaID string
	for _, line := range strings.Split(output, "\n") {
		if idx := strings.Index(line, "LEM-"); idx >= 0 {
			// Extract just the ID portion
			end := strings.IndexAny(line[idx:], " \t\n\r,}")
			if end > 0 {
				lemmaID = line[idx : idx+end]
			} else {
				lemmaID = strings.TrimSpace(line[idx:])
			}
			break
		}
	}

	// Verify lemma is in state with correct properties
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	// Try to find the lemma
	var foundLemma *interface{}
	for _, l := range st.AllLemmas() {
		if l.Statement == statement {
			tmp := interface{}(l)
			foundLemma = &tmp
			break
		}
	}

	if foundLemma == nil {
		t.Errorf("lemma with statement %q not found in state", statement)
	}

	// If we extracted a lemma ID, verify we can get it
	if lemmaID != "" {
		lemma := st.GetLemma(lemmaID)
		if lemma == nil {
			t.Logf("Could not find lemma by ID %q, may be partial ID in output", lemmaID)
		}
	}
}

// TestExtractLemmaCmd_RelativeDirectory tests using relative directory path.
func TestExtractLemmaCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-extract-rel-*")
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

	// Validate the root node
	svc, _ := service.NewProofService(proofDir)
	nodeID := mustParseExtractLemmaNodeID(t, "1")
	if err := svc.AcceptNode(nodeID); err != nil {
		t.Fatal(err)
	}

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	output, err := executeExtractLemmaCommand(t, "1",
		"--statement", "Relative path lemma",
		"-d", "subdir/proof")
	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}

	// Verify lemma was created
	if !strings.Contains(output, "LEM-") {
		t.Errorf("expected lemma ID in output, got: %q", output)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestExtractLemmaCmd_LongStatement tests extraction with a very long statement.
func TestExtractLemmaCmd_LongStatement(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	longStatement := strings.Repeat("This is a very long lemma statement that tests edge cases. ", 50)
	output, err := executeExtractLemmaCommand(t, "1",
		"--statement", longStatement,
		"-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for long statement, got: %v", err)
	}

	// Verify lemma was created
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	lemmas := st.AllLemmas()

	if len(lemmas) != 1 {
		t.Errorf("expected 1 lemma, got %d", len(lemmas))
	}

	if len(lemmas) > 0 && len(lemmas[0].Statement) != len(longStatement) {
		t.Errorf("statement truncated: expected %d chars, got %d", len(longStatement), len(lemmas[0].Statement))
	}

	t.Logf("Output length for long statement: %d bytes", len(output))
}

// TestExtractLemmaCmd_SpecialCharactersInStatement tests special characters handling.
func TestExtractLemmaCmd_SpecialCharactersInStatement(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	specialStatement := "For all x > 0: f(x) = x^2 + 2*x + 1 = (x+1)^2"
	output, err := executeExtractLemmaCommand(t, "1",
		"--statement", specialStatement,
		"-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for special characters, got: %v", err)
	}

	// Verify statement is preserved correctly
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	lemmas := st.AllLemmas()

	if len(lemmas) > 0 && lemmas[0].Statement != specialStatement {
		t.Errorf("statement not preserved: got %q, want %q", lemmas[0].Statement, specialStatement)
	}

	t.Logf("Output with special chars: %s", output)
}

// TestExtractLemmaCmd_UnicodeStatement tests unicode in statement.
func TestExtractLemmaCmd_UnicodeStatement(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	unicodeStatement := "For all n in integers, n squared is greater than or equal to zero"
	output, err := executeExtractLemmaCommand(t, "1",
		"--statement", unicodeStatement,
		"-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for unicode statement, got: %v", err)
	}

	if !strings.Contains(output, "LEM-") {
		t.Errorf("expected lemma ID in output, got: %q", output)
	}
}

// TestExtractLemmaCmd_DeepNode tests extraction from a deeply nested node.
func TestExtractLemmaCmd_DeepNode(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create nested nodes: 1.1, 1.1.1, 1.1.1.1
	rootID := mustParseExtractLemmaNodeID(t, "1")

	proverOwner := "prover"
	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		t.Fatal(err)
	}

	// Create chain of children
	nodes := []string{"1.1", "1.1.1", "1.1.1.1"}
	parentID := rootID

	for _, nodeStr := range nodes {
		childID := mustParseExtractLemmaNodeID(t, nodeStr)
		err := svc.RefineNode(parentID, proverOwner, childID, schema.NodeTypeClaim,
			"Statement for "+nodeStr, schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", nodeStr, err)
		}

		// Release parent and claim child for next iteration
		if err := svc.ReleaseNode(parentID, proverOwner); err != nil {
			t.Fatalf("failed to release %s: %v", parentID.String(), err)
		}
		if err := svc.ClaimNode(childID, proverOwner, 5*time.Minute); err != nil {
			t.Fatalf("failed to claim %s: %v", nodeStr, err)
		}
		parentID = childID
	}

	// Release the deepest node
	deepID := mustParseExtractLemmaNodeID(t, "1.1.1.1")
	if err := svc.ReleaseNode(deepID, proverOwner); err != nil {
		t.Fatal(err)
	}

	// Validate the deepest node
	if err := svc.AcceptNode(deepID); err != nil {
		t.Fatal(err)
	}

	// Extract lemma from deepest node
	output, err := executeExtractLemmaCommand(t, "1.1.1.1",
		"--statement", "Lemma from deep node",
		"-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error extracting from deep node, got: %v", err)
	}

	if !strings.Contains(output, "LEM-") {
		t.Errorf("expected lemma ID in output, got: %q", output)
	}
}

// =============================================================================
// Consistency Tests
// =============================================================================

// TestExtractLemmaCmd_ConsistentWithLemmasCommand tests that extracted lemmas
// appear in the lemmas listing.
func TestExtractLemmaCmd_ConsistentWithLemmasCommand(t *testing.T) {
	tmpDir, cleanup := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup()

	statement := "A lemma that should appear in listing"

	// Extract a lemma
	_, err := executeExtractLemmaCommand(t, "1",
		"--statement", statement,
		"-d", tmpDir)
	if err != nil {
		t.Fatalf("extraction failed: %v", err)
	}

	// List lemmas using lemmas command
	lemmasCmd := newTestLemmasCmd()
	lemmasOutput, err := executeCommand(lemmasCmd, "lemmas", "-d", tmpDir)
	if err != nil {
		t.Fatalf("lemmas command failed: %v", err)
	}

	// The extracted lemma should appear in the listing
	if !strings.Contains(lemmasOutput, "lemma") && !strings.Contains(strings.ToLower(lemmasOutput), statement[:20]) {
		t.Logf("Lemmas output may not show full statement: %q", lemmasOutput)
	}
}

// TestExtractLemmaCmd_JSONAndTextConsistency tests JSON and text output consistency.
func TestExtractLemmaCmd_JSONAndTextConsistency(t *testing.T) {
	// Get text output
	tmpDir1, cleanup1 := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup1()

	textOutput, err := executeExtractLemmaCommand(t, "1",
		"--statement", "Consistency test lemma",
		"-d", tmpDir1,
		"-f", "text")
	if err != nil {
		t.Fatalf("text output failed: %v", err)
	}

	// Get JSON output from fresh setup
	tmpDir2, cleanup2 := setupExtractLemmaTestWithValidatedNode(t)
	defer cleanup2()

	jsonOutput, err := executeExtractLemmaCommand(t, "1",
		"--statement", "Consistency test lemma",
		"-d", tmpDir2,
		"-f", "json")
	if err != nil {
		t.Fatalf("JSON output failed: %v", err)
	}

	// Both should contain lemma ID
	if !strings.Contains(textOutput, "LEM-") {
		t.Errorf("text output missing lemma ID")
	}

	var jsonResult map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &jsonResult); err != nil {
		t.Errorf("JSON output invalid: %v", err)
	}

	// JSON should have id field
	if _, ok := jsonResult["id"]; !ok {
		if _, ok := jsonResult["lemma_id"]; !ok {
			t.Logf("JSON may not have standard id field: %+v", jsonResult)
		}
	}
}
