//go:build integration

// Package main contains tests for the af recompute-taint command.
// These are TDD tests - the recompute-taint command does not exist yet.
// Tests define the expected behavior for recomputing taint across the proof tree.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// mustParseRecomputeTaintNodeID parses a NodeID string or fails the test.
func mustParseRecomputeTaintNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("Failed to parse NodeID %q: %v", s, err)
	}
	return id
}

// setupRecomputeTaintTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupRecomputeTaintTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-recompute-taint-test-*")
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
	if err := service.Init(tmpDir, "Test conjecture for taint", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupRecomputeTaintTestWithNodes creates a test environment with multiple nodes.
// Creates a proof tree with:
// - Node 1 (root)
// - Node 1.1 (child)
// - Node 1.2 (child)
// - Node 1.1.1 (grandchild)
func setupRecomputeTaintTestWithNodes(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupRecomputeTaintTest(t)

	// Create a proof service and add multiple nodes
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create child nodes: 1.1, 1.2, and grandchild 1.1.1
	nodes := []string{"1.1", "1.2", "1.1.1"}
	for _, idStr := range nodes {
		nodeID := mustParseRecomputeTaintNodeID(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceModusPonens)
		if err != nil {
			cleanup()
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	return tmpDir, cleanup
}

// executeRecomputeTaintCommand creates and executes a recompute-taint command with the given arguments.
func executeRecomputeTaintCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newRecomputeTaintCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// Help and Metadata Tests
// =============================================================================

// TestRecomputeTaintCmd_Help tests that help output shows usage information.
func TestRecomputeTaintCmd_Help(t *testing.T) {
	cmd := newRecomputeTaintCmd()
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
		"recompute-taint",
		"--dir",
		"--format",
		"--dry-run",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestRecomputeTaintCmd_HelpShortFlag tests help with short flag -h.
func TestRecomputeTaintCmd_HelpShortFlag(t *testing.T) {
	cmd := newRecomputeTaintCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help -h should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "recompute-taint") {
		t.Errorf("help output should mention 'recompute-taint', got: %q", output)
	}
}

// TestRecomputeTaintCmd_Metadata tests command metadata.
func TestRecomputeTaintCmd_Metadata(t *testing.T) {
	cmd := newRecomputeTaintCmd()

	if cmd.Use != "recompute-taint" {
		t.Errorf("expected Use to be 'recompute-taint', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestRecomputeTaintCmd_DefaultFlagValues verifies default flag values.
func TestRecomputeTaintCmd_DefaultFlagValues(t *testing.T) {
	cmd := newRecomputeTaintCmd()

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

	// Check default dry-run value
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Fatal("expected dry-run flag to exist")
	}
	if dryRunFlag.DefValue != "false" {
		t.Errorf("expected default dry-run to be 'false', got %q", dryRunFlag.DefValue)
	}

	// Check default verbose value
	verboseFlag := cmd.Flags().Lookup("verbose")
	if verboseFlag == nil {
		t.Fatal("expected verbose flag to exist")
	}
	if verboseFlag.DefValue != "false" {
		t.Errorf("expected default verbose to be 'false', got %q", verboseFlag.DefValue)
	}
}

// TestRecomputeTaintCmd_ExpectedFlags ensures the command has expected flags.
func TestRecomputeTaintCmd_ExpectedFlags(t *testing.T) {
	cmd := newRecomputeTaintCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir", "dry-run", "verbose"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected recompute-taint command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
		"v": "verbose",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected recompute-taint command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestRecomputeTaintCmd_DirFlagShortForm tests -d short form works.
func TestRecomputeTaintCmd_DirFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTest(t)
	defer cleanup()

	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -d flag, got: %v\nOutput: %s", err, output)
	}
}

// TestRecomputeTaintCmd_DirFlagLongForm tests --dir long form works.
func TestRecomputeTaintCmd_DirFlagLongForm(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTest(t)
	defer cleanup()

	output, err := executeRecomputeTaintCommand(t, "--dir", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with --dir flag, got: %v\nOutput: %s", err, output)
	}
}

// TestRecomputeTaintCmd_FormatFlagShortForm tests -f short form works.
func TestRecomputeTaintCmd_FormatFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTest(t)
	defer cleanup()

	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error with -f flag, got: %v", err)
	}

	// Should be valid JSON
	var data interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Errorf("expected valid JSON with -f json, got: %v\nOutput: %q", err, output)
	}
}

// TestRecomputeTaintCmd_VerboseFlagShortForm tests -v short form works.
func TestRecomputeTaintCmd_VerboseFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir, "-v")
	if err != nil {
		t.Fatalf("expected no error with -v flag, got: %v", err)
	}

	// Verbose output should contain more details
	if output == "" {
		t.Error("expected non-empty verbose output")
	}
}

// =============================================================================
// Basic Command Tests
// =============================================================================

// TestRecomputeTaintCmd_EmptyProof tests behavior with a proof with only root node.
func TestRecomputeTaintCmd_EmptyProof(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTest(t)
	defer cleanup()

	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error for proof with only root, got: %v", err)
	}

	// Should complete without error, may show "no changes" or similar
	if output == "" {
		t.Log("Output is empty for proof with only root node (acceptable)")
	}
}

// TestRecomputeTaintCmd_ProofNotInitialized tests error when proof is not initialized.
func TestRecomputeTaintCmd_ProofNotInitialized(t *testing.T) {
	// Create directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-recompute-taint-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize directory structure but not the proof
	if err := fs.InitProofDir(tmpDir); err != nil {
		t.Fatal(err)
	}

	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)

	// Should produce error
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not initialized") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		!strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "empty") &&
		err == nil {
		t.Errorf("expected error for uninitialized proof, got: %q", output)
	}
}

// TestRecomputeTaintCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestRecomputeTaintCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeRecomputeTaintCommand(t, "-d", "/nonexistent/path/12345")

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

// TestRecomputeTaintCmd_DefaultDirectory tests using current directory by default.
func TestRecomputeTaintCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTest(t)
	defer cleanup()

	// Change to proof directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Execute without -d flag (should use current directory)
	output, err := executeRecomputeTaintCommand(t)
	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}
}

// =============================================================================
// Taint Computation Tests
// =============================================================================

// TestRecomputeTaintCmd_CleanNodeStaysClean tests that a clean node stays clean.
func TestRecomputeTaintCmd_CleanNodeStaysClean(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	// Accept all nodes to make them validated (clean)
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept root and children
	for _, idStr := range []string{"1", "1.1", "1.2", "1.1.1"} {
		nodeID := mustParseRecomputeTaintNodeID(t, idStr)
		if err := svc.AcceptNode(nodeID); err != nil {
			t.Fatalf("failed to accept node %s: %v", idStr, err)
		}
	}

	// Run recompute-taint
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify all nodes are clean
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	for _, idStr := range []string{"1", "1.1", "1.2", "1.1.1"} {
		nodeID := mustParseRecomputeTaintNodeID(t, idStr)
		n := st.GetNode(nodeID)
		if n == nil {
			t.Errorf("node %s not found", idStr)
			continue
		}
		if n.TaintState != node.TaintClean {
			t.Errorf("node %s TaintState = %q, want %q", idStr, n.TaintState, node.TaintClean)
		}
	}

	t.Logf("Output: %s", output)
}

// TestRecomputeTaintCmd_AdmittedNodeBecomesSelfAdmitted tests that an admitted node becomes self_admitted.
func TestRecomputeTaintCmd_AdmittedNodeBecomesSelfAdmitted(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept root, admit node 1.1
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	nodeID := mustParseRecomputeTaintNodeID(t, "1.1")
	if err := svc.AdmitNode(nodeID); err != nil {
		t.Fatalf("failed to admit node 1.1: %v", err)
	}

	// Run recompute-taint
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify node 1.1 is self_admitted
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node 1.1 not found")
	}

	if n.TaintState != node.TaintSelfAdmitted {
		t.Errorf("node 1.1 TaintState = %q, want %q", n.TaintState, node.TaintSelfAdmitted)
	}

	t.Logf("Output: %s", output)
}

// TestRecomputeTaintCmd_ChildOfTaintedBecomesTainted tests that child of tainted node becomes tainted.
func TestRecomputeTaintCmd_ChildOfTaintedBecomesTainted(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept root, admit node 1.1, accept grandchild 1.1.1
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	node11ID := mustParseRecomputeTaintNodeID(t, "1.1")
	if err := svc.AdmitNode(node11ID); err != nil {
		t.Fatalf("failed to admit node 1.1: %v", err)
	}

	node111ID := mustParseRecomputeTaintNodeID(t, "1.1.1")
	if err := svc.AcceptNode(node111ID); err != nil {
		t.Fatalf("failed to accept node 1.1.1: %v", err)
	}

	// Run recompute-taint
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify taint states
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	// Node 1.1 should be self_admitted
	n11 := st.GetNode(node11ID)
	if n11 == nil {
		t.Fatal("node 1.1 not found")
	}
	if n11.TaintState != node.TaintSelfAdmitted {
		t.Errorf("node 1.1 TaintState = %q, want %q", n11.TaintState, node.TaintSelfAdmitted)
	}

	// Node 1.1.1 should be tainted (child of self_admitted)
	n111 := st.GetNode(node111ID)
	if n111 == nil {
		t.Fatal("node 1.1.1 not found")
	}
	if n111.TaintState != node.TaintTainted {
		t.Errorf("node 1.1.1 TaintState = %q, want %q", n111.TaintState, node.TaintTainted)
	}

	t.Logf("Output: %s", output)
}

// TestRecomputeTaintCmd_ValidatedNodeBecomesClean tests that validated node becomes clean.
func TestRecomputeTaintCmd_ValidatedNodeBecomesClean(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept the root node
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	// Run recompute-taint
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify root is clean
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(rootID)
	if n == nil {
		t.Fatal("root node not found")
	}

	if n.TaintState != node.TaintClean {
		t.Errorf("root TaintState = %q, want %q", n.TaintState, node.TaintClean)
	}

	t.Logf("Output: %s", output)
}

// TestRecomputeTaintCmd_PendingNodeIsUnresolved tests that pending node has unresolved taint.
func TestRecomputeTaintCmd_PendingNodeIsUnresolved(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	// Don't accept any nodes - they should all be pending

	// Run recompute-taint
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify all nodes are unresolved
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	for _, idStr := range []string{"1", "1.1", "1.2", "1.1.1"} {
		nodeID := mustParseRecomputeTaintNodeID(t, idStr)
		n := st.GetNode(nodeID)
		if n == nil {
			t.Errorf("node %s not found", idStr)
			continue
		}
		if n.TaintState != node.TaintUnresolved {
			t.Errorf("node %s TaintState = %q, want %q (node is pending)", idStr, n.TaintState, node.TaintUnresolved)
		}
	}

	t.Logf("Output: %s", output)
}

// TestRecomputeTaintCmd_RefutedNodeAffectsTaint tests how refuted nodes affect taint.
func TestRecomputeTaintCmd_RefutedNodeAffectsTaint(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Refute the root node
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.RefuteNode(rootID); err != nil {
		t.Fatalf("failed to refute root: %v", err)
	}

	// Run recompute-taint
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Refuted node should not introduce taint (it's final but doesn't introduce taint)
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(rootID)
	if n == nil {
		t.Fatal("root node not found")
	}

	// Refuted node should be clean (not self_admitted, not tainted)
	if n.TaintState != node.TaintClean {
		t.Errorf("refuted root TaintState = %q, want %q", n.TaintState, node.TaintClean)
	}

	t.Logf("Output: %s", output)
}

// TestRecomputeTaintCmd_ComplexTreePropagation tests taint propagation in complex tree.
func TestRecomputeTaintCmd_ComplexTreePropagation(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create more nodes for complex tree
	moreNodes := []string{"1.2.1", "1.2.2"}
	for _, idStr := range moreNodes {
		nodeID := mustParseRecomputeTaintNodeID(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	// Accept root and 1.2, admit 1.1
	// Tree:
	//       1 (validated, clean)
	//      / \
	//   1.1   1.2 (validated, clean)
	//  (admitted, self_admitted)  / \
	//   |                       1.2.1 1.2.2 (pending, unresolved)
	// 1.1.1 (pending, unresolved)

	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	node11ID := mustParseRecomputeTaintNodeID(t, "1.1")
	if err := svc.AdmitNode(node11ID); err != nil {
		t.Fatalf("failed to admit node 1.1: %v", err)
	}

	node12ID := mustParseRecomputeTaintNodeID(t, "1.2")
	if err := svc.AcceptNode(node12ID); err != nil {
		t.Fatalf("failed to accept node 1.2: %v", err)
	}

	// Run recompute-taint
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify taint states
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	expectedTaint := map[string]node.TaintState{
		"1":     node.TaintClean,        // validated
		"1.1":   node.TaintSelfAdmitted, // admitted
		"1.2":   node.TaintClean,        // validated
		"1.1.1": node.TaintUnresolved,   // pending (ancestor unresolved check doesn't apply when node itself is pending)
		"1.2.1": node.TaintUnresolved,   // pending
		"1.2.2": node.TaintUnresolved,   // pending
	}

	for idStr, expected := range expectedTaint {
		nodeID := mustParseRecomputeTaintNodeID(t, idStr)
		n := st.GetNode(nodeID)
		if n == nil {
			t.Errorf("node %s not found", idStr)
			continue
		}
		if n.TaintState != expected {
			t.Errorf("node %s TaintState = %q, want %q", idStr, n.TaintState, expected)
		}
	}

	t.Logf("Output: %s", output)
}

// =============================================================================
// Dry Run Tests
// =============================================================================

// TestRecomputeTaintCmd_DryRunShowsChanges tests that --dry-run shows what would change.
func TestRecomputeTaintCmd_DryRunShowsChanges(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept all nodes
	for _, idStr := range []string{"1", "1.1", "1.2", "1.1.1"} {
		nodeID := mustParseRecomputeTaintNodeID(t, idStr)
		if err := svc.AcceptNode(nodeID); err != nil {
			t.Fatalf("failed to accept node %s: %v", idStr, err)
		}
	}

	// Run with --dry-run
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir, "--dry-run")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should indicate it's a dry run or show what would change
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "dry") && !strings.Contains(lower, "would") && !strings.Contains(lower, "preview") {
		t.Logf("Warning: dry-run output may not indicate it's a preview: %q", output)
	}
}

// TestRecomputeTaintCmd_DryRunDoesNotModify tests that --dry-run doesn't modify state.
func TestRecomputeTaintCmd_DryRunDoesNotModify(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept all nodes
	for _, idStr := range []string{"1", "1.1", "1.2", "1.1.1"} {
		nodeID := mustParseRecomputeTaintNodeID(t, idStr)
		if err := svc.AcceptNode(nodeID); err != nil {
			t.Fatalf("failed to accept node %s: %v", idStr, err)
		}
	}

	// Get state before dry-run
	stBefore, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	// Collect taint states before
	taintBefore := make(map[string]node.TaintState)
	for _, n := range stBefore.AllNodes() {
		taintBefore[n.ID.String()] = n.TaintState
	}

	// Run with --dry-run
	_, err = executeRecomputeTaintCommand(t, "-d", tmpDir, "--dry-run")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Get state after dry-run
	stAfter, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	// Verify taint states are unchanged
	for _, n := range stAfter.AllNodes() {
		before := taintBefore[n.ID.String()]
		if n.TaintState != before {
			t.Errorf("node %s taint changed after dry-run: %q -> %q", n.ID.String(), before, n.TaintState)
		}
	}
}

// TestRecomputeTaintCmd_DryRunWithJSON tests --dry-run with JSON format.
func TestRecomputeTaintCmd_DryRunWithJSON(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept root, admit 1.1
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	node11ID := mustParseRecomputeTaintNodeID(t, "1.1")
	if err := svc.AdmitNode(node11ID); err != nil {
		t.Fatalf("failed to admit node 1.1: %v", err)
	}

	// Run with --dry-run and JSON format
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir, "--dry-run", "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var data interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Errorf("dry-run with JSON should produce valid JSON: %v\nOutput: %q", err, output)
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestRecomputeTaintCmd_TextOutputFormat tests text output format.
func TestRecomputeTaintCmd_TextOutputFormat(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept root to trigger some taint changes
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	// Run with text format (default)
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir, "-f", "text")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Text output should be human-readable
	if output == "" {
		t.Log("Text output is empty")
	}

	// Should not be JSON
	var jsonTest interface{}
	if err := json.Unmarshal([]byte(output), &jsonTest); err == nil {
		// If it parses as JSON array, that's wrong for text format
		if strings.HasPrefix(strings.TrimSpace(output), "[") || strings.HasPrefix(strings.TrimSpace(output), "{") {
			t.Log("Warning: text format might be JSON-like")
		}
	}
}

// TestRecomputeTaintCmd_JSONOutputFormat tests JSON output format.
func TestRecomputeTaintCmd_JSONOutputFormat(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept root
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	// Run with JSON format
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var data interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Errorf("output should be valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestRecomputeTaintCmd_JSONOutputStructure tests JSON output contains expected fields.
func TestRecomputeTaintCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept root, admit 1.1
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	node11ID := mustParseRecomputeTaintNodeID(t, "1.1")
	if err := svc.AdmitNode(node11ID); err != nil {
		t.Fatalf("failed to admit node 1.1: %v", err)
	}

	// Run with JSON format
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		// Might be an array
		var arr []interface{}
		if err2 := json.Unmarshal([]byte(output), &arr); err2 != nil {
			t.Fatalf("output is not valid JSON object or array: %v\nOutput: %q", err, output)
		}
		return // Array output is acceptable
	}

	// Expected fields might include: changes, nodes_changed, summary, etc.
	// Check for common fields
	possibleKeys := []string{"changes", "nodes_changed", "changed", "summary", "total", "nodes"}
	hasExpectedKey := false
	for _, key := range possibleKeys {
		if _, ok := result[key]; ok {
			hasExpectedKey = true
			break
		}
	}

	if !hasExpectedKey && len(result) == 0 {
		t.Log("JSON output has no recognizable structure keys")
	}
}

// TestRecomputeTaintCmd_VerboseOutputDetails tests verbose output includes details.
func TestRecomputeTaintCmd_VerboseOutputDetails(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept root, admit 1.1 to cause taint changes
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	node11ID := mustParseRecomputeTaintNodeID(t, "1.1")
	if err := svc.AdmitNode(node11ID); err != nil {
		t.Fatalf("failed to admit node 1.1: %v", err)
	}

	// Run without verbose
	normalOutput, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Create fresh test environment for verbose comparison
	tmpDir2, cleanup2 := setupRecomputeTaintTestWithNodes(t)
	defer cleanup2()

	svc2, err := service.NewProofService(tmpDir2)
	if err != nil {
		t.Fatal(err)
	}

	rootID2 := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc2.AcceptNode(rootID2); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	node11ID2 := mustParseRecomputeTaintNodeID(t, "1.1")
	if err := svc2.AdmitNode(node11ID2); err != nil {
		t.Fatalf("failed to admit node 1.1: %v", err)
	}

	// Run with verbose
	verboseOutput, err := executeRecomputeTaintCommand(t, "-d", tmpDir2, "-v")
	if err != nil {
		t.Fatalf("expected no error with verbose, got: %v", err)
	}

	// Verbose output should typically be longer or contain more detail
	// This is a soft check - implementation may vary
	t.Logf("Normal output length: %d, Verbose output length: %d", len(normalOutput), len(verboseOutput))
}

// TestRecomputeTaintCmd_InvalidFormat tests error for invalid format option.
func TestRecomputeTaintCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTest(t)
	defer cleanup()

	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir, "-f", "invalid")

	// Should produce error or warning about invalid format
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// The behavior depends on implementation - it might error or fall back to text
	t.Logf("Output with invalid format: %s (error: %v)", output, err)
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestRecomputeTaintCmd_NotADirectory tests error when path is not a directory.
func TestRecomputeTaintCmd_NotADirectory(t *testing.T) {
	// Create a temporary file (not directory)
	tmpFile, err := os.CreateTemp("", "af-recompute-taint-test-file-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	_, err = executeRecomputeTaintCommand(t, "-d", tmpFile.Name())

	if err == nil {
		t.Error("expected error when path is a file not directory")
	}
}

// =============================================================================
// Summary/Statistics Tests
// =============================================================================

// TestRecomputeTaintCmd_SummaryOutput tests that output includes summary/statistics.
func TestRecomputeTaintCmd_SummaryOutput(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept all nodes
	for _, idStr := range []string{"1", "1.1", "1.2", "1.1.1"} {
		nodeID := mustParseRecomputeTaintNodeID(t, idStr)
		if err := svc.AcceptNode(nodeID); err != nil {
			t.Fatalf("failed to accept node %s: %v", idStr, err)
		}
	}

	// Run recompute-taint
	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain some summary or count of nodes processed/changed
	lower := strings.ToLower(output)
	hasSummaryInfo := strings.Contains(lower, "node") ||
		strings.Contains(lower, "change") ||
		strings.Contains(lower, "taint") ||
		strings.Contains(lower, "complet") ||
		strings.Contains(lower, "success")

	if !hasSummaryInfo && output != "" {
		t.Logf("Output may not contain summary info: %q", output)
	}
}

// TestRecomputeTaintCmd_NoChangesOutput tests output when no taint changes needed.
func TestRecomputeTaintCmd_NoChangesOutput(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept root - it's already in initial state
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	// Run recompute-taint twice - second run should have no changes
	_, err = executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	// Second run output may indicate no changes or be empty
	t.Logf("Second run output (should show no/few changes): %q", output)
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestRecomputeTaintCmd_TableDrivenTaintStates tests various taint state scenarios.
func TestRecomputeTaintCmd_TableDrivenTaintStates(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(t *testing.T, svc *service.ProofService) // Setup before recompute
		expectedTaint map[string]node.TaintState                    // Expected taint states after
	}{
		{
			name: "all pending",
			setupFunc: func(t *testing.T, svc *service.ProofService) {
				// Do nothing - all nodes stay pending
			},
			expectedTaint: map[string]node.TaintState{
				"1":     node.TaintUnresolved,
				"1.1":   node.TaintUnresolved,
				"1.2":   node.TaintUnresolved,
				"1.1.1": node.TaintUnresolved,
			},
		},
		{
			name: "all validated",
			setupFunc: func(t *testing.T, svc *service.ProofService) {
				for _, idStr := range []string{"1", "1.1", "1.2", "1.1.1"} {
					nodeID := mustParseRecomputeTaintNodeID(t, idStr)
					if err := svc.AcceptNode(nodeID); err != nil {
						t.Fatalf("failed to accept %s: %v", idStr, err)
					}
				}
			},
			expectedTaint: map[string]node.TaintState{
				"1":     node.TaintClean,
				"1.1":   node.TaintClean,
				"1.2":   node.TaintClean,
				"1.1.1": node.TaintClean,
			},
		},
		{
			name: "root admitted propagates to children",
			setupFunc: func(t *testing.T, svc *service.ProofService) {
				rootID := mustParseRecomputeTaintNodeID(t, "1")
				if err := svc.AdmitNode(rootID); err != nil {
					t.Fatalf("failed to admit root: %v", err)
				}
				// Accept children
				for _, idStr := range []string{"1.1", "1.2", "1.1.1"} {
					nodeID := mustParseRecomputeTaintNodeID(t, idStr)
					if err := svc.AcceptNode(nodeID); err != nil {
						t.Fatalf("failed to accept %s: %v", idStr, err)
					}
				}
			},
			expectedTaint: map[string]node.TaintState{
				"1":     node.TaintSelfAdmitted,
				"1.1":   node.TaintTainted, // child of self_admitted
				"1.2":   node.TaintTainted, // child of self_admitted
				"1.1.1": node.TaintTainted, // grandchild of self_admitted
			},
		},
		{
			name: "sibling not affected by admitted sibling",
			setupFunc: func(t *testing.T, svc *service.ProofService) {
				// Accept root
				rootID := mustParseRecomputeTaintNodeID(t, "1")
				if err := svc.AcceptNode(rootID); err != nil {
					t.Fatalf("failed to accept root: %v", err)
				}
				// Admit 1.1
				node11ID := mustParseRecomputeTaintNodeID(t, "1.1")
				if err := svc.AdmitNode(node11ID); err != nil {
					t.Fatalf("failed to admit 1.1: %v", err)
				}
				// Accept 1.2 (sibling)
				node12ID := mustParseRecomputeTaintNodeID(t, "1.2")
				if err := svc.AcceptNode(node12ID); err != nil {
					t.Fatalf("failed to accept 1.2: %v", err)
				}
			},
			expectedTaint: map[string]node.TaintState{
				"1":     node.TaintClean,        // validated
				"1.1":   node.TaintSelfAdmitted, // admitted
				"1.2":   node.TaintClean,        // validated, not affected by sibling
				"1.1.1": node.TaintUnresolved,   // pending
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
			defer cleanup()

			svc, err := service.NewProofService(tmpDir)
			if err != nil {
				t.Fatal(err)
			}

			// Run setup
			tc.setupFunc(t, svc)

			// Run recompute-taint
			output, err := executeRecomputeTaintCommand(t, "-d", tmpDir)
			if err != nil {
				t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
			}

			// Verify expected taint states
			st, err := svc.LoadState()
			if err != nil {
				t.Fatal(err)
			}

			for idStr, expected := range tc.expectedTaint {
				nodeID := mustParseRecomputeTaintNodeID(t, idStr)
				n := st.GetNode(nodeID)
				if n == nil {
					t.Errorf("node %s not found", idStr)
					continue
				}
				if n.TaintState != expected {
					t.Errorf("node %s TaintState = %q, want %q", idStr, n.TaintState, expected)
				}
			}
		})
	}
}

// TestRecomputeTaintCmd_OutputFormatVariants tests different output format variations.
func TestRecomputeTaintCmd_OutputFormatVariants(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		validator func(t *testing.T, output string)
	}{
		{
			name:   "text format",
			format: "text",
			validator: func(t *testing.T, output string) {
				// Text format should be human-readable
				// Not necessarily JSON
			},
		},
		{
			name:   "json format",
			format: "json",
			validator: func(t *testing.T, output string) {
				// JSON format should be valid JSON
				var data interface{}
				if err := json.Unmarshal([]byte(output), &data); err != nil {
					t.Errorf("expected valid JSON, got error: %v\nOutput: %q", err, output)
				}
			},
		},
		{
			name:   "TEXT uppercase",
			format: "TEXT",
			validator: func(t *testing.T, output string) {
				// Should accept uppercase TEXT
			},
		},
		{
			name:   "JSON uppercase",
			format: "JSON",
			validator: func(t *testing.T, output string) {
				// Should accept uppercase JSON
				var data interface{}
				if err := json.Unmarshal([]byte(output), &data); err != nil {
					t.Errorf("expected valid JSON with uppercase format, got error: %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupRecomputeTaintTest(t)
			defer cleanup()

			output, err := executeRecomputeTaintCommand(t, "-d", tmpDir, "-f", tc.format)
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

// TestRecomputeTaintCmd_IdempotentOperation tests that running twice produces same result.
func TestRecomputeTaintCmd_IdempotentOperation(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept root, admit 1.1
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	node11ID := mustParseRecomputeTaintNodeID(t, "1.1")
	if err := svc.AdmitNode(node11ID); err != nil {
		t.Fatalf("failed to admit 1.1: %v", err)
	}

	// First run
	_, err = executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	// Get state after first run
	st1, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	taint1 := make(map[string]node.TaintState)
	for _, n := range st1.AllNodes() {
		taint1[n.ID.String()] = n.TaintState
	}

	// Second run
	_, err = executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	// Get state after second run
	st2, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	// Verify taint states are the same
	for _, n := range st2.AllNodes() {
		if n.TaintState != taint1[n.ID.String()] {
			t.Errorf("taint changed between runs for node %s: %q -> %q",
				n.ID.String(), taint1[n.ID.String()], n.TaintState)
		}
	}
}

// TestRecomputeTaintCmd_AfterStateChange tests recompute after state changes.
func TestRecomputeTaintCmd_AfterStateChange(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTestWithNodes(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// First: accept root only
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	// Run recompute
	_, err = executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("first recompute error: %v", err)
	}

	// Now admit 1.1
	node11ID := mustParseRecomputeTaintNodeID(t, "1.1")
	if err := svc.AdmitNode(node11ID); err != nil {
		t.Fatalf("failed to admit 1.1: %v", err)
	}

	// Run recompute again
	_, err = executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("second recompute error: %v", err)
	}

	// Verify 1.1 is now self_admitted
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(node11ID)
	if n == nil {
		t.Fatal("node 1.1 not found")
	}

	if n.TaintState != node.TaintSelfAdmitted {
		t.Errorf("node 1.1 TaintState = %q, want %q", n.TaintState, node.TaintSelfAdmitted)
	}
}

// TestRecomputeTaintCmd_DeeplyNestedTree tests taint propagation in deeply nested tree.
func TestRecomputeTaintCmd_DeeplyNestedTree(t *testing.T) {
	tmpDir, cleanup := setupRecomputeTaintTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create deep tree: 1 -> 1.1 -> 1.1.1 -> 1.1.1.1 -> 1.1.1.1.1
	deepNodes := []string{"1.1", "1.1.1", "1.1.1.1", "1.1.1.1.1"}
	for _, idStr := range deepNodes {
		nodeID := mustParseRecomputeTaintNodeID(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Deep node "+idStr, schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	// Accept root, admit 1.1.1 (middle node)
	rootID := mustParseRecomputeTaintNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root: %v", err)
	}

	node11ID := mustParseRecomputeTaintNodeID(t, "1.1")
	if err := svc.AcceptNode(node11ID); err != nil {
		t.Fatalf("failed to accept 1.1: %v", err)
	}

	node111ID := mustParseRecomputeTaintNodeID(t, "1.1.1")
	if err := svc.AdmitNode(node111ID); err != nil {
		t.Fatalf("failed to admit 1.1.1: %v", err)
	}

	// Accept descendants
	node1111ID := mustParseRecomputeTaintNodeID(t, "1.1.1.1")
	if err := svc.AcceptNode(node1111ID); err != nil {
		t.Fatalf("failed to accept 1.1.1.1: %v", err)
	}

	node11111ID := mustParseRecomputeTaintNodeID(t, "1.1.1.1.1")
	if err := svc.AcceptNode(node11111ID); err != nil {
		t.Fatalf("failed to accept 1.1.1.1.1: %v", err)
	}

	// Run recompute
	_, err = executeRecomputeTaintCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("recompute error: %v", err)
	}

	// Verify taint propagation
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]node.TaintState{
		"1":         node.TaintClean,        // validated
		"1.1":       node.TaintClean,        // validated
		"1.1.1":     node.TaintSelfAdmitted, // admitted
		"1.1.1.1":   node.TaintTainted,      // child of admitted
		"1.1.1.1.1": node.TaintTainted,      // grandchild of admitted
	}

	for idStr, expectedTaint := range expected {
		nodeID := mustParseRecomputeTaintNodeID(t, idStr)
		n := st.GetNode(nodeID)
		if n == nil {
			t.Errorf("node %s not found", idStr)
			continue
		}
		if n.TaintState != expectedTaint {
			t.Errorf("node %s TaintState = %q, want %q", idStr, n.TaintState, expectedTaint)
		}
	}
}
