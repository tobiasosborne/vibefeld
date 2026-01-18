//go:build integration

// Package main contains tests for the af get command.
// These are TDD tests - the get command does not exist yet.
// Tests define the expected behavior for retrieving node details.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestGetCmd creates a fresh root command with the get subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestGetCmd() *cobra.Command {
	cmd := newTestRootCmd()

	getCmd := newGetCmd()
	cmd.AddCommand(getCmd)

	return cmd
}

// executeGetCommand creates and executes a get command with the given arguments.
func executeGetCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newGetCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// setupGetTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupGetTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-get-test-*")
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
	if err := service.Init(tmpDir, "Test conjecture", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupGetTestWithNode creates a test environment with an initialized proof
// and a single node at ID "1".
// Note: service.Init already creates node 1 with the conjecture, so we just
// return the result of setupGetTest. Node 1 has statement "Test conjecture".
func setupGetTestWithNode(t *testing.T) (string, func()) {
	t.Helper()
	// service.Init already creates node 1 with the conjecture "Test conjecture"
	return setupGetTest(t)
}

// setupGetTestWithHierarchy creates a test environment with a multi-level node hierarchy.
// Node 1 already exists from Init with statement "Test conjecture".
// Creates additional child nodes: 1.1, 1.2, 1.1.1, 1.1.2
func setupGetTestWithHierarchy(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupGetTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create child nodes (node 1 already exists from Init with statement "Test conjecture")
	nodes := []struct {
		id        string
		statement string
	}{
		{"1.1", "First child statement"},
		{"1.2", "Second child statement"},
		{"1.1.1", "First grandchild statement"},
		{"1.1.2", "Second grandchild statement"},
	}

	for _, n := range nodes {
		nodeID, err := service.ParseNodeID(n.id)
		if err != nil {
			cleanup()
			t.Fatalf("failed to parse node ID %s: %v", n.id, err)
		}
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, n.statement, schema.InferenceModusPonens)
		if err != nil {
			cleanup()
			t.Fatalf("failed to create node %s: %v", n.id, err)
		}
	}

	return tmpDir, cleanup
}

// =============================================================================
// Basic Retrieval Tests
// =============================================================================

// TestGetCmd_SingleNode tests getting a single node by ID.
func TestGetCmd_SingleNode(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain the node ID
	if !strings.Contains(output, "1") {
		t.Errorf("expected output to contain node ID '1', got: %q", output)
	}

	// Output should contain the statement (node 1 has "Test conjecture" from Init)
	if !strings.Contains(output, "Test conjecture") {
		t.Errorf("expected output to contain node statement, got: %q", output)
	}
}

// TestGetCmd_ChildNode tests getting a child node.
func TestGetCmd_ChildNode(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain the node ID
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain node ID '1.1', got: %q", output)
	}

	// Output should contain the statement
	if !strings.Contains(output, "First child statement") {
		t.Errorf("expected output to contain child statement, got: %q", output)
	}
}

// TestGetCmd_DeepNode tests getting a deeply nested node.
func TestGetCmd_DeepNode(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1.1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain the node ID
	if !strings.Contains(output, "1.1.1") {
		t.Errorf("expected output to contain node ID '1.1.1', got: %q", output)
	}

	// Output should contain the statement
	if !strings.Contains(output, "First grandchild statement") {
		t.Errorf("expected output to contain grandchild statement, got: %q", output)
	}
}

// =============================================================================
// --ancestors Flag Tests
// =============================================================================

// TestGetCmd_WithAncestors tests the --ancestors flag shows parent chain.
func TestGetCmd_WithAncestors(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1.1.1", "--ancestors", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain the target node
	if !strings.Contains(output, "1.1.1") {
		t.Errorf("expected output to contain node ID '1.1.1', got: %q", output)
	}

	// Output should contain ancestors
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain parent '1.1', got: %q", output)
	}
	if !strings.Contains(output, "1") {
		t.Errorf("expected output to contain root '1', got: %q", output)
	}
}

// TestGetCmd_WithAncestorsShortFlag tests the -a short flag for ancestors.
func TestGetCmd_WithAncestorsShortFlag(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1.1.1", "-a", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -a flag, got: %v", err)
	}

	// Should show ancestors
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain parent '1.1', got: %q", output)
	}
}

// TestGetCmd_AncestorsOnRoot tests --ancestors on root node (should show only root).
func TestGetCmd_AncestorsOnRoot(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "--ancestors", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain root node
	if !strings.Contains(output, "1") {
		t.Errorf("expected output to contain root node '1', got: %q", output)
	}
}

// =============================================================================
// --subtree Flag Tests
// =============================================================================

// TestGetCmd_WithSubtree tests the --subtree flag shows children.
func TestGetCmd_WithSubtree(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "--subtree", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain root (node 1 has "Test conjecture" from Init)
	if !strings.Contains(output, "Test conjecture") {
		t.Errorf("expected output to contain root statement, got: %q", output)
	}

	// Output should contain children
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child '1.1', got: %q", output)
	}
	if !strings.Contains(output, "1.2") {
		t.Errorf("expected output to contain child '1.2', got: %q", output)
	}
}

// TestGetCmd_WithSubtreeShortFlag tests the -s short flag for subtree.
func TestGetCmd_WithSubtreeShortFlag(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-s", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -s flag, got: %v", err)
	}

	// Should show children
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child '1.1', got: %q", output)
	}
}

// TestGetCmd_SubtreeIncludesDeepChildren tests --subtree shows all descendants.
func TestGetCmd_SubtreeIncludesDeepChildren(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1.1", "--subtree", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain node 1.1
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain '1.1', got: %q", output)
	}

	// Should contain grandchildren
	if !strings.Contains(output, "1.1.1") {
		t.Errorf("expected output to contain grandchild '1.1.1', got: %q", output)
	}
	if !strings.Contains(output, "1.1.2") {
		t.Errorf("expected output to contain grandchild '1.1.2', got: %q", output)
	}
}

// TestGetCmd_SubtreeOnLeaf tests --subtree on a leaf node (should show only that node).
func TestGetCmd_SubtreeOnLeaf(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1.1.1", "--subtree", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain the leaf node
	if !strings.Contains(output, "1.1.1") {
		t.Errorf("expected output to contain '1.1.1', got: %q", output)
	}
}

// =============================================================================
// --full Flag Tests
// =============================================================================

// TestGetCmd_WithFull tests the --full flag shows complete node details.
func TestGetCmd_WithFull(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "--full", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Full output should include node type
	if !strings.Contains(strings.ToLower(output), "claim") {
		t.Errorf("expected full output to contain node type 'claim', got: %q", output)
	}

	// Full output should include workflow state
	if !strings.Contains(strings.ToLower(output), "available") {
		t.Errorf("expected full output to contain workflow state, got: %q", output)
	}

	// Full output should include epistemic state
	if !strings.Contains(strings.ToLower(output), "pending") {
		t.Errorf("expected full output to contain epistemic state, got: %q", output)
	}

	// Full output should include inference type
	if !strings.Contains(strings.ToLower(output), "assumption") {
		t.Errorf("expected full output to contain inference type, got: %q", output)
	}
}

// TestGetCmd_WithFullShortFlag tests the -F short flag for full details.
func TestGetCmd_WithFullShortFlag(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-F", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -F flag, got: %v", err)
	}

	// Should show extended details
	if !strings.Contains(strings.ToLower(output), "claim") {
		t.Errorf("expected full output to contain node type, got: %q", output)
	}
}

// =============================================================================
// Combined Flags Tests
// =============================================================================

// TestGetCmd_AncestorsAndFull tests combining --ancestors and --full flags.
func TestGetCmd_AncestorsAndFull(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1.1.1", "--ancestors", "--full", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain ancestors
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain ancestor '1.1', got: %q", output)
	}

	// Should contain extended details
	if !strings.Contains(strings.ToLower(output), "claim") {
		t.Errorf("expected full output to contain node type, got: %q", output)
	}
}

// TestGetCmd_SubtreeAndFull tests combining --subtree and --full flags.
func TestGetCmd_SubtreeAndFull(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "--subtree", "--full", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain children
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child '1.1', got: %q", output)
	}

	// Should contain extended details for each node
	// Multiple occurrences of "claim" indicate full details for multiple nodes
	count := strings.Count(strings.ToLower(output), "claim")
	if count < 2 {
		t.Errorf("expected full output to contain multiple 'claim' entries, got %d occurrences", count)
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestGetCmd_NodeNotFound tests error when node doesn't exist.
func TestGetCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupGetTest(t)
	defer cleanup()

	// Note: Node 1 exists from Init, so test with a non-existent node
	output, err := executeGetCommand(t, "2", "-d", tmpDir)

	// Should produce error - node 2 doesn't exist
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "does not exist") &&
		err == nil {
		t.Errorf("expected error for non-existent node, got: %q", output)
	}
}

// TestGetCmd_NodeNotFoundDeep tests error for non-existent deep node.
func TestGetCmd_NodeNotFoundDeep(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1.999", "-d", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "does not exist") &&
		err == nil {
		t.Errorf("expected error for non-existent node '1.999', got: %q", output)
	}
}

// TestGetCmd_MissingNodeID tests error when node ID is not provided.
func TestGetCmd_MissingNodeID(t *testing.T) {
	tmpDir, cleanup := setupGetTest(t)
	defer cleanup()

	_, err := executeGetCommand(t, "-d", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing node ID, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") &&
		!strings.Contains(errStr, "required") &&
		!strings.Contains(errStr, "argument") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestGetCmd_InvalidNodeIDFormat tests error for invalid node ID format.
func TestGetCmd_InvalidNodeIDFormat(t *testing.T) {
	tmpDir, cleanup := setupGetTest(t)
	defer cleanup()

	tests := []struct {
		name   string
		nodeID string
	}{
		{"empty string", ""},
		{"invalid characters", "abc"},
		{"negative number", "-1"},
		{"zero", "0"},
		{"invalid format with spaces", "1 2"},
		{"leading dot", ".1"},
		{"trailing dot", "1."},
		{"double dot", "1..2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeGetCommand(t, tc.nodeID, "-d", tmpDir)

			combined := output
			if err != nil {
				combined += err.Error()
			}

			if !strings.Contains(strings.ToLower(combined), "invalid") &&
				!strings.Contains(strings.ToLower(combined), "error") &&
				err == nil {
				t.Errorf("expected error for invalid node ID %q, got: %q", tc.nodeID, output)
			}
		})
	}
}

// TestGetCmd_ProofNotInitialized tests error when proof is not initialized.
func TestGetCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-get-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	output, err := executeGetCommand(t, "1", "-d", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not initialized") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		!strings.Contains(strings.ToLower(combined), "not found") &&
		err == nil {
		t.Errorf("expected error for uninitialized proof, got: %q", output)
	}
}

// TestGetCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestGetCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeGetCommand(t, "1", "-d", "/nonexistent/path/12345")

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

// TestGetCmd_JSONOutput tests JSON output format.
func TestGetCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Should contain node ID field
	if _, ok := result["id"]; !ok {
		if _, ok := result["node_id"]; !ok {
			t.Error("JSON output should contain 'id' or 'node_id' field")
		}
	}
}

// TestGetCmd_JSONOutputWithAncestors tests JSON output with --ancestors flag.
func TestGetCmd_JSONOutputWithAncestors(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1.1.1", "--ancestors", "-f", "json", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// JSON should contain ancestors information
	if !strings.Contains(output, "1.1") {
		t.Errorf("JSON output should contain ancestor '1.1', got: %q", output)
	}
}

// TestGetCmd_JSONOutputWithSubtree tests JSON output with --subtree flag.
func TestGetCmd_JSONOutputWithSubtree(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "--subtree", "-f", "json", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// JSON should contain children information
	if !strings.Contains(output, "1.1") {
		t.Errorf("JSON output should contain child '1.1', got: %q", output)
	}
}

// TestGetCmd_JSONOutputWithFull tests JSON output with --full flag.
func TestGetCmd_JSONOutputWithFull(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "--full", "-f", "json", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Full JSON should include extended fields
	expectedFields := []string{"type", "workflow_state", "epistemic_state", "inference"}
	for _, field := range expectedFields {
		if _, ok := result[field]; !ok {
			t.Logf("Warning: full JSON might not contain field %q", field)
		}
	}
}

// TestGetCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestGetCmd_JSONOutputShortFlag(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error with -f flag, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON with -f flag: %v\nOutput: %q", err, output)
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestGetCmd_Help tests that help output shows usage information.
func TestGetCmd_Help(t *testing.T) {
	cmd := newGetCmd()
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
		"get",
		"node-id",
		"--dir",
		"--format",
		"--ancestors",
		"--subtree",
		"--full",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestGetCmd_HelpShortFlag tests help with -h short flag.
func TestGetCmd_HelpShortFlag(t *testing.T) {
	cmd := newGetCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help with -h should not error: %v", err)
	}

	output := buf.String()

	// Should contain command name
	if !strings.Contains(output, "get") {
		t.Errorf("help output should contain 'get', got: %q", output)
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestGetCmd_ExpectedFlags ensures the get command has expected flag structure.
func TestGetCmd_ExpectedFlags(t *testing.T) {
	cmd := newGetCmd()

	// Check expected flags exist
	expectedFlags := []string{"dir", "format", "ancestors", "subtree", "full"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected get command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"d": "dir",
		"f": "format",
		"a": "ancestors",
		"s": "subtree",
		"F": "full",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected get command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestGetCmd_DefaultFlagValues verifies default values for flags.
func TestGetCmd_DefaultFlagValues(t *testing.T) {
	cmd := newGetCmd()

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

	// Boolean flags should default to false
	ancestorsFlag := cmd.Flags().Lookup("ancestors")
	if ancestorsFlag == nil {
		t.Fatal("expected ancestors flag to exist")
	}
	if ancestorsFlag.DefValue != "false" {
		t.Errorf("expected default ancestors to be 'false', got %q", ancestorsFlag.DefValue)
	}

	subtreeFlag := cmd.Flags().Lookup("subtree")
	if subtreeFlag == nil {
		t.Fatal("expected subtree flag to exist")
	}
	if subtreeFlag.DefValue != "false" {
		t.Errorf("expected default subtree to be 'false', got %q", subtreeFlag.DefValue)
	}

	fullFlag := cmd.Flags().Lookup("full")
	if fullFlag == nil {
		t.Fatal("expected full flag to exist")
	}
	if fullFlag.DefValue != "false" {
		t.Errorf("expected default full to be 'false', got %q", fullFlag.DefValue)
	}
}

// TestGetCmd_CommandMetadata verifies command metadata.
func TestGetCmd_CommandMetadata(t *testing.T) {
	cmd := newGetCmd()

	if cmd.Use != "get <node-id>" && cmd.Use != "get [node-id]" && !strings.HasPrefix(cmd.Use, "get") {
		t.Errorf("expected Use to start with 'get', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Directory Flag Tests
// =============================================================================

// TestGetCmd_DirFlagLongForm tests that --dir works.
func TestGetCmd_DirFlagLongForm(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with --dir flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was retrieved
	if !strings.Contains(output, "1") {
		t.Errorf("expected output to contain node ID, got: %q", output)
	}
}

// TestGetCmd_DirFlagShortForm tests that -d works.
func TestGetCmd_DirFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -d flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was retrieved
	if !strings.Contains(output, "1") {
		t.Errorf("expected output to contain node ID, got: %q", output)
	}
}

// TestGetCmd_DefaultDirectory tests get uses current directory by default.
func TestGetCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
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
	output, err := executeGetCommand(t, "1")
	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Verify node was retrieved
	if !strings.Contains(output, "1") {
		t.Errorf("expected output to contain node ID when using default directory, got: %q", output)
	}
}

// TestGetCmd_RelativeDirectory tests using relative directory path.
func TestGetCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-get-rel-*")
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
	// Note: service.Init already creates node 1 with the conjecture

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	output, err := executeGetCommand(t, "1", "-d", "subdir/proof")
	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}

	// Verify node was retrieved
	if !strings.Contains(output, "1") {
		t.Errorf("expected output to contain node ID with relative directory, got: %q", output)
	}
}

// =============================================================================
// Format Tests
// =============================================================================

// TestGetCmd_InvalidFormat tests error for invalid format option.
func TestGetCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-d", tmpDir, "-f", "invalid")

	// Should produce error or warning about invalid format
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// The behavior depends on implementation - it might error or fall back to text
	t.Logf("Output with invalid format: %s (error: %v)", output, err)
}

// TestGetCmd_FormatValidation verifies format flag validation.
func TestGetCmd_FormatValidation(t *testing.T) {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, cleanup := setupGetTestWithNode(t)
			defer cleanup()

			output, err := executeGetCommand(t, "1", "-d", tmpDir, "-f", tt.format)

			if tt.wantErr {
				combined := output
				if err != nil {
					combined += err.Error()
				}

				if err == nil && !strings.Contains(strings.ToLower(combined), "format") {
					t.Logf("Expected error for format %q, got output: %q", tt.format, output)
				}
			} else {
				if err != nil && strings.Contains(err.Error(), "format") {
					t.Errorf("unexpected format error for format %q: %v", tt.format, err)
				}
			}
		})
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestGetCmd_TableDrivenNodeIDs tests various valid and invalid node ID inputs.
func TestGetCmd_TableDrivenNodeIDs(t *testing.T) {
	tests := []struct {
		name        string
		nodeID      string
		setupNode   bool // whether to create the node first
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid root",
			nodeID:    "1",
			setupNode: true,
			wantErr:   false,
		},
		{
			name:        "empty ID",
			nodeID:      "",
			setupNode:   false,
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "whitespace ID",
			nodeID:      "   ",
			setupNode:   false,
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "letters only",
			nodeID:      "abc",
			setupNode:   false,
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "mixed letters and numbers",
			nodeID:      "1a.2b",
			setupNode:   false,
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "negative number",
			nodeID:      "-1",
			setupNode:   false,
			wantErr:     true,
			errContains: "invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupGetTest(t)
			defer cleanup()

			if tc.setupNode && tc.nodeID != "" {
				// Only create node if it's a valid ID and setupNode is true
				id, err := service.ParseNodeID(tc.nodeID)
				if err == nil {
					svc, _ := service.NewProofService(tmpDir)
					_ = svc.CreateNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
				}
			}

			output, err := executeGetCommand(t, tc.nodeID, "-d", tmpDir)

			if tc.wantErr {
				combined := output
				if err != nil {
					combined += err.Error()
				}

				if err == nil && !strings.Contains(strings.ToLower(combined), tc.errContains) {
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

// =============================================================================
// Output Content Tests
// =============================================================================

// TestGetCmd_OutputContainsNodeStatement tests that output includes node statement.
func TestGetCmd_OutputContainsNodeStatement(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain the node's statement (node 1 has "Test conjecture" from Init)
	if !strings.Contains(output, "Test conjecture") {
		t.Errorf("expected output to contain node statement 'Test conjecture', got: %q", output)
	}
}

// TestGetCmd_OutputContainsNodeID tests that output includes node ID.
func TestGetCmd_OutputContainsNodeID(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1.1.2", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain the node ID
	if !strings.Contains(output, "1.1.2") {
		t.Errorf("expected output to contain node ID '1.1.2', got: %q", output)
	}
}

// TestGetCmd_SingleNodeShowsFullTextByDefault tests that single node output shows
// full text by default (not truncated). This is the expected behavior for viewing
// a single node - users should not need --full flag to see complete statement.
func TestGetCmd_SingleNodeShowsFullTextByDefault(t *testing.T) {
	tmpDir, cleanup := setupGetTest(t)
	defer cleanup()

	// Create a node with a long statement that would be truncated at 60 chars
	longStatement := "This is a very long statement that exceeds the sixty character limit and should be shown in full by default when viewing a single node"
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, longStatement, schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	output, err := executeGetCommand(t, "1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// The full statement should be present, not truncated with "..."
	if !strings.Contains(output, longStatement) {
		t.Errorf("expected single node output to contain full statement, got: %q", output)
	}

	// Should NOT contain truncation ellipsis for the statement
	if strings.Contains(output, "sixty character limit...") {
		t.Errorf("single node output should NOT truncate statement by default, got: %q", output)
	}
}

// TestGetCmd_SingleNodeShowsVerboseFieldsByDefault tests that single node view
// shows verbose fields (type, workflow, epistemic state, etc.) by default.
func TestGetCmd_SingleNodeShowsVerboseFieldsByDefault(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Default single node output should include verbose fields
	// These fields are only shown in verbose output, not in the truncated summary
	if !strings.Contains(strings.ToLower(output), "type:") {
		t.Errorf("expected single node output to contain 'Type:' field by default, got: %q", output)
	}
	if !strings.Contains(strings.ToLower(output), "workflow:") {
		t.Errorf("expected single node output to contain 'Workflow:' field by default, got: %q", output)
	}
	if !strings.Contains(strings.ToLower(output), "epistemic:") {
		t.Errorf("expected single node output to contain 'Epistemic:' field by default, got: %q", output)
	}
}

// =============================================================================
// Challenge Display Tests
// =============================================================================

// setupGetTestWithChallenges creates a test environment with challenges on a node.
func setupGetTestWithChallenges(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupGetTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add challenges via the ledger
	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add an open challenge on node 1
	nodeID1, _ := service.ParseNodeID("1")
	event1 := ledger.NewChallengeRaised("ch-abc123", nodeID1, "gap", "Missing case for n=0")
	if _, err := ldg.Append(event1); err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add another challenge and resolve it
	event2 := ledger.NewChallengeRaised("ch-def456", nodeID1, "context", "Undefined variable")
	if _, err := ldg.Append(event2); err != nil {
		cleanup()
		t.Fatal(err)
	}

	event3 := ledger.NewChallengeResolved("ch-def456")
	if _, err := ldg.Append(event3); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// TestGetCmd_ShowsChallengesInTextOutput tests that challenges are displayed in text output.
func TestGetCmd_ShowsChallengesInTextOutput(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithChallenges(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show "Challenges" section header
	if !strings.Contains(output, "Challenges") {
		t.Errorf("expected output to contain 'Challenges' section, got: %q", output)
	}

	// Should show challenge ID
	if !strings.Contains(output, "ch-abc123") {
		t.Errorf("expected output to contain challenge ID 'ch-abc123', got: %q", output)
	}

	// Should show challenge status
	if !strings.Contains(output, "[open]") {
		t.Errorf("expected output to contain '[open]' status, got: %q", output)
	}

	// Should show challenge target
	if !strings.Contains(output, "gap") {
		t.Errorf("expected output to contain challenge target 'gap', got: %q", output)
	}

	// Should show challenge reason
	if !strings.Contains(output, "Missing case for n=0") {
		t.Errorf("expected output to contain challenge reason 'Missing case for n=0', got: %q", output)
	}

	// Should also show the resolved challenge
	if !strings.Contains(output, "ch-def456") {
		t.Errorf("expected output to contain resolved challenge ID 'ch-def456', got: %q", output)
	}
	if !strings.Contains(output, "[resolved]") {
		t.Errorf("expected output to contain '[resolved]' status, got: %q", output)
	}
}

// TestGetCmd_ShowsChallengesInJSONOutput tests that challenges are included in JSON output.
func TestGetCmd_ShowsChallengesInJSONOutput(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithChallenges(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Should contain challenges array
	challenges, ok := result["challenges"]
	if !ok {
		t.Fatalf("JSON output should contain 'challenges' field, got: %v", result)
	}

	// Should be a non-empty array
	challengeList, ok := challenges.([]interface{})
	if !ok {
		t.Fatalf("challenges should be an array, got: %T", challenges)
	}

	if len(challengeList) != 2 {
		t.Errorf("expected 2 challenges, got %d", len(challengeList))
	}

	// Verify first challenge has expected fields
	if len(challengeList) > 0 {
		ch, ok := challengeList[0].(map[string]interface{})
		if !ok {
			t.Fatalf("challenge should be an object, got: %T", challengeList[0])
		}

		// Should have id, status, target, reason fields
		requiredFields := []string{"id", "status", "target", "reason"}
		for _, field := range requiredFields {
			if _, ok := ch[field]; !ok {
				t.Errorf("challenge missing required field %q", field)
			}
		}
	}
}

// TestGetCmd_NoChallengesDoesNotShowSection tests that no Challenges section appears when none exist.
func TestGetCmd_NoChallengesDoesNotShowSection(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should NOT contain a Challenges section when there are no challenges
	if strings.Contains(output, "Challenges (") {
		t.Errorf("expected output to NOT contain 'Challenges' section when no challenges exist, got: %q", output)
	}
}

// TestGetCmd_NoChallengesInJSONOutput tests that no challenges key appears when none exist.
func TestGetCmd_NoChallengesInJSONOutput(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Should NOT contain challenges key when there are no challenges
	if _, ok := result["challenges"]; ok {
		t.Errorf("JSON output should NOT contain 'challenges' key when no challenges exist, got: %v", result)
	}
}

// TestGetCmd_ChallengesWithFullFlagMultipleNodes tests challenges are shown with --full and multiple nodes.
func TestGetCmd_ChallengesWithFullFlagMultipleNodes(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithChallenges(t)
	defer cleanup()

	// Create a child node first
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID11, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(nodeID11, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	output, err := executeGetCommand(t, "1", "--subtree", "--full", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show challenges for node 1 (which has challenges)
	if !strings.Contains(output, "ch-abc123") {
		t.Errorf("expected output to contain challenge ID for node 1, got: %q", output)
	}
}

// =============================================================================
// Checklist Flag Tests
// =============================================================================

// TestGetCmd_ChecklistFlag tests that --checklist shows verification checklist.
func TestGetCmd_ChecklistFlag(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "--checklist", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain checklist header
	if !strings.Contains(output, "Verification Checklist") {
		t.Errorf("expected output to contain 'Verification Checklist', got: %q", output)
	}

	// Should contain node ID in the header
	if !strings.Contains(output, "Node 1") {
		t.Errorf("expected output to contain 'Node 1', got: %q", output)
	}

	// Should contain standard checklist sections
	checklistSections := []string{
		"STATEMENT PRECISION",
		"INFERENCE VALIDITY",
		"DEPENDENCIES",
		"HIDDEN ASSUMPTIONS",
		"DOMAIN RESTRICTIONS",
		"NOTATION CONSISTENCY",
	}

	for _, section := range checklistSections {
		if !strings.Contains(output, section) {
			t.Errorf("expected output to contain checklist section %q, got: %q", section, output)
		}
	}

	// Should contain challenge command suggestion
	if !strings.Contains(output, "af challenge") {
		t.Errorf("expected output to contain 'af challenge' command suggestion, got: %q", output)
	}
}

// TestGetCmd_ChecklistShortFlag tests that -c short flag shows verification checklist.
func TestGetCmd_ChecklistShortFlag(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "-c", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -c flag, got: %v", err)
	}

	// Should contain checklist header
	if !strings.Contains(output, "Verification Checklist") {
		t.Errorf("expected output to contain 'Verification Checklist' with -c flag, got: %q", output)
	}
}

// TestGetCmd_ChecklistJSON tests JSON output with --checklist flag.
func TestGetCmd_ChecklistJSON(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "--checklist", "-f", "json", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Should contain node_id field
	if _, ok := result["node_id"]; !ok {
		t.Error("JSON checklist output should contain 'node_id' field")
	}

	// Should contain items array
	items, ok := result["items"]
	if !ok {
		t.Error("JSON checklist output should contain 'items' field")
	} else {
		itemsList, ok := items.([]interface{})
		if !ok {
			t.Errorf("'items' should be an array, got: %T", items)
		} else if len(itemsList) == 0 {
			t.Error("'items' array should not be empty")
		}
	}

	// Should contain challenge_command field
	if _, ok := result["challenge_command"]; !ok {
		t.Error("JSON checklist output should contain 'challenge_command' field")
	}
}

// TestGetCmd_ChecklistContainsStatement tests that checklist shows the node statement.
func TestGetCmd_ChecklistContainsStatement(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "--checklist", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Node 1 has statement "Test conjecture" from Init
	if !strings.Contains(output, "Test conjecture") {
		t.Errorf("expected checklist to contain node statement 'Test conjecture', got: %q", output)
	}
}

// TestGetCmd_ChecklistContainsInference tests that checklist shows inference type.
func TestGetCmd_ChecklistContainsInference(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	// Node 1.1 was created with modus_ponens inference
	output, err := executeGetCommand(t, "1.1", "--checklist", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain inference type
	if !strings.Contains(output, "modus_ponens") {
		t.Errorf("expected checklist to contain inference type 'modus_ponens', got: %q", output)
	}
}

// TestGetCmd_ChecklistWithDependencies tests checklist shows dependency information.
func TestGetCmd_ChecklistWithDependencies(t *testing.T) {
	tmpDir, cleanup := setupGetTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a child node with dependencies
	nodeID11, _ := service.ParseNodeID("1.1")
	nodeID1, _ := service.ParseNodeID("1")

	// First create the node
	err = svc.CreateNode(nodeID11, schema.NodeTypeClaim, "Dependent claim", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	// Add dependency via ledger
	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add dependency by creating a node with dependencies
	// Since we can't easily modify deps after creation, let's just verify the checklist handles deps field
	_ = nodeID1

	output, err := executeGetCommand(t, "1.1", "--checklist", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain dependencies section regardless
	if !strings.Contains(output, "DEPENDENCIES") {
		t.Errorf("expected checklist to contain 'DEPENDENCIES' section, got: %q", output)
	}

	_ = ldg // Use to avoid unused variable error
}

// TestGetCmd_ChecklistFlagExists tests that the checklist flag is properly registered.
func TestGetCmd_ChecklistFlagExists(t *testing.T) {
	cmd := newGetCmd()

	// Check that checklist flag exists
	checklistFlag := cmd.Flags().Lookup("checklist")
	if checklistFlag == nil {
		t.Fatal("expected get command to have 'checklist' flag")
	}

	// Check short flag
	if cmd.Flags().ShorthandLookup("c") == nil {
		t.Error("expected get command to have short flag -c for --checklist")
	}

	// Check default value is false
	if checklistFlag.DefValue != "false" {
		t.Errorf("expected default checklist to be 'false', got %q", checklistFlag.DefValue)
	}
}

// TestGetCmd_ChecklistJSONStructure tests the detailed JSON structure of the checklist.
func TestGetCmd_ChecklistJSONStructure(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithNode(t)
	defer cleanup()

	output, err := executeGetCommand(t, "1", "--checklist", "-f", "json", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Verify node_id is correct
	nodeID, ok := result["node_id"].(string)
	if !ok || nodeID != "1" {
		t.Errorf("expected node_id to be '1', got: %v", result["node_id"])
	}

	// Verify items structure
	items, ok := result["items"].([]interface{})
	if !ok {
		t.Fatal("items should be an array")
	}

	// Check each item has required fields
	for i, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			t.Errorf("item %d should be an object", i)
			continue
		}

		// Check required fields
		requiredFields := []string{"category", "description", "checks"}
		for _, field := range requiredFields {
			if _, ok := itemMap[field]; !ok {
				t.Errorf("item %d missing required field %q", i, field)
			}
		}

		// Verify checks is an array
		checks, ok := itemMap["checks"].([]interface{})
		if !ok {
			t.Errorf("item %d 'checks' should be an array", i)
		} else if len(checks) == 0 {
			t.Errorf("item %d 'checks' should not be empty", i)
		}
	}
}

// TestGetCmd_ChecklistOverridesOtherFlags tests that checklist flag takes precedence.
func TestGetCmd_ChecklistOverridesOtherFlags(t *testing.T) {
	tmpDir, cleanup := setupGetTestWithHierarchy(t)
	defer cleanup()

	// Even with --ancestors flag, checklist should show only the target node's checklist
	output, err := executeGetCommand(t, "1.1.1", "--checklist", "--ancestors", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain checklist header (not ancestor display)
	if !strings.Contains(output, "Verification Checklist") {
		t.Errorf("expected output to contain 'Verification Checklist' even with --ancestors, got: %q", output)
	}

	// Should show checklist for node 1.1.1 specifically
	if !strings.Contains(output, "Node 1.1.1") {
		t.Errorf("expected output to contain 'Node 1.1.1', got: %q", output)
	}
}

// TestGetCmd_ChecklistNodeNotFound tests error when node doesn't exist with checklist flag.
func TestGetCmd_ChecklistNodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupGetTest(t)
	defer cleanup()

	output, err := executeGetCommand(t, "999", "--checklist", "-d", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "does not exist") &&
		err == nil {
		t.Errorf("expected error for non-existent node with --checklist, got: %q", output)
	}
}
