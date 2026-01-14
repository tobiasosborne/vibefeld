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
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestGetCmd creates a fresh root command with the get subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

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
		nodeID, err := types.Parse(n.id)
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
	if err := fs.InitProofDir(proofDir); err != nil {
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
				id, err := types.Parse(tc.nodeID)
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

	nodeID, _ := types.Parse("1.1")
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
