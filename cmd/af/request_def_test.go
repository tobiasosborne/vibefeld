//go:build integration

// Package main contains tests for the af request-def command.
// These are TDD tests - the request-def command implementation should
// be created to satisfy these tests.
// Tests define the expected behavior for requesting definitions during proof work.
package main

import (
	"encoding/json"
	"os"
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

// setupRequestDefTest creates a temporary directory with an initialized proof
// and a node for testing the request-def command.
func setupRequestDefTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-request-def-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	// Initialize proof
	err = service.Init(tmpDir, "Test conjecture requiring definitions", "test-author")
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Note: service.Init already creates the root node "1" with the conjecture
	// as its statement, so we don't need to create it again here.

	return tmpDir, cleanup
}

// setupRequestDefTestWithMultipleNodes creates a test environment with multiple nodes.
func setupRequestDefTestWithMultipleNodes(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupRequestDefTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create child nodes
	for _, idStr := range []string{"1.1", "1.2"} {
		nodeID, _ := service.ParseNodeID(idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement "+idStr, schema.InferenceModusPonens)
		if err != nil {
			cleanup()
			t.Fatal(err)
		}
	}

	return tmpDir, cleanup
}

// newRequestDefTestCmd creates a test command hierarchy with the request-def command.
// This ensures test isolation - each test gets its own command instance.
func newRequestDefTestCmd() *cobra.Command {
	cmd := newTestRootCmd()

	requestDefCmd := newRequestDefCmd()
	cmd.AddCommand(requestDefCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// =============================================================================
// Success Case Tests
// =============================================================================

// TestRequestDefCmd_ValidRequest tests successfully requesting a definition.
func TestRequestDefCmd_ValidRequest(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	output, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", "group",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check output confirms request was created
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "pending") && !strings.Contains(lower, "request") && !strings.Contains(lower, "definition") {
		t.Errorf("expected output to confirm pending def request, got: %q", output)
	}

	// Verify the term is mentioned in output
	if !strings.Contains(output, "group") {
		t.Errorf("expected output to mention term 'group', got: %q", output)
	}
}

// TestRequestDefCmd_CreatesFilesystemEntry tests that the request creates a filesystem entry.
func TestRequestDefCmd_CreatesFilesystemEntry(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	_, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", "homomorphism",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify pending def was created in filesystem
	nodeID, _ := service.ParseNodeID("1")
	pd, err := fs.ReadPendingDef(tmpDir, nodeID)
	if err != nil {
		t.Fatalf("expected pending def to be readable: %v", err)
	}

	if pd.Term != "homomorphism" {
		t.Errorf("expected term 'homomorphism', got: %q", pd.Term)
	}

	if pd.RequestedBy.String() != "1" {
		t.Errorf("expected requested_by '1', got: %q", pd.RequestedBy.String())
	}

	if !pd.IsPending() {
		t.Error("expected pending def to be in pending status")
	}
}

// TestRequestDefCmd_ChildNode tests requesting a definition for a child node.
func TestRequestDefCmd_ChildNode(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTestWithMultipleNodes(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	output, err := executeCommand(cmd, "request-def",
		"--node", "1.1",
		"--term", "kernel",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should mention the node ID
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to mention node 1.1, got: %q", output)
	}

	// Verify pending def was created
	nodeID, _ := service.ParseNodeID("1.1")
	pd, err := fs.ReadPendingDef(tmpDir, nodeID)
	if err != nil {
		t.Fatalf("expected pending def to be readable: %v", err)
	}

	if pd.Term != "kernel" {
		t.Errorf("expected term 'kernel', got: %q", pd.Term)
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestRequestDefCmd_MissingTerm tests error when term is not provided.
func TestRequestDefCmd_MissingTerm(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	_, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--dir", tmpDir,
		// Note: --term is missing
	)

	if err == nil {
		t.Fatal("expected error for missing term, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "term") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing term, got: %q", errStr)
	}
}

// TestRequestDefCmd_MissingNodeID tests error when node ID is not provided.
func TestRequestDefCmd_MissingNodeID(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	_, err := executeCommand(cmd, "request-def",
		"--term", "group",
		"--dir", tmpDir,
		// Note: --node is missing
	)

	if err == nil {
		t.Fatal("expected error for missing node ID, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "node") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing node ID, got: %q", errStr)
	}
}

// TestRequestDefCmd_NodeNotFound tests error when node doesn't exist.
func TestRequestDefCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	_, err := executeCommand(cmd, "request-def",
		"--node", "1.999",
		"--term", "group",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for non-existent node, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "does not exist") {
		t.Errorf("expected error about node not found, got: %q", errStr)
	}
}

// TestRequestDefCmd_EmptyTerm tests error when term is empty string.
func TestRequestDefCmd_EmptyTerm(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	_, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", "",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for empty term, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "term") && !strings.Contains(errStr, "empty") {
		t.Errorf("expected error about empty term, got: %q", errStr)
	}
}

// TestRequestDefCmd_WhitespaceOnlyTerm tests error when term is whitespace only.
func TestRequestDefCmd_WhitespaceOnlyTerm(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	_, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", "   ",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for whitespace-only term, got nil")
	}
}

// TestRequestDefCmd_InvalidNodeIDFormat tests error for invalid node ID formats.
func TestRequestDefCmd_InvalidNodeIDFormat(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	tests := []struct {
		name   string
		nodeID string
	}{
		{"invalid characters", "abc"},
		{"negative number", "-1"},
		{"zero", "0"},
		{"leading dot", ".1"},
		{"trailing dot", "1."},
		{"double dot", "1..2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newRequestDefTestCmd()
			_, err := executeCommand(cmd, "request-def",
				"--node", tc.nodeID,
				"--term", "test",
				"--dir", tmpDir,
			)

			if err == nil {
				t.Errorf("expected error for invalid node ID %q, got nil", tc.nodeID)
			}
		})
	}
}

// TestRequestDefCmd_ProofNotInitialized tests error when proof hasn't been initialized.
func TestRequestDefCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-request-def-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newRequestDefTestCmd()
	_, err = executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", "group",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestRequestDefCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestRequestDefCmd_DirectoryNotFound(t *testing.T) {
	cmd := newRequestDefTestCmd()
	_, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", "group",
		"--dir", "/nonexistent/path/12345",
	)

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// =============================================================================
// JSON Output Tests
// =============================================================================

// TestRequestDefCmd_JSONOutput tests JSON output format.
func TestRequestDefCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	output, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", "group",
		"--format", "json",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Check for expected fields
	if _, ok := result["node_id"]; !ok {
		if _, ok := result["nodeId"]; !ok {
			t.Logf("Warning: JSON output does not contain node_id or nodeId")
		}
	}

	if _, ok := result["term"]; !ok {
		t.Logf("Warning: JSON output does not contain term")
	}

	if _, ok := result["status"]; !ok {
		t.Logf("Warning: JSON output does not contain status")
	}
}

// TestRequestDefCmd_JSONFormatShortFlag tests JSON output with short flag.
func TestRequestDefCmd_JSONFormatShortFlag(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	output, err := executeCommand(cmd, "request-def",
		"-n", "1",
		"-t", "group",
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

// =============================================================================
// Multiple Requests Tests
// =============================================================================

// TestRequestDefCmd_MultipleRequests tests requesting definitions for multiple nodes.
func TestRequestDefCmd_MultipleRequests(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTestWithMultipleNodes(t)
	defer cleanup()

	// Request definitions for multiple nodes
	terms := map[string]string{
		"1":   "group",
		"1.1": "homomorphism",
		"1.2": "kernel",
	}

	for nodeID, term := range terms {
		cmd := newRequestDefTestCmd()
		_, err := executeCommand(cmd, "request-def",
			"--node", nodeID,
			"--term", term,
			"--dir", tmpDir,
		)

		if err != nil {
			t.Fatalf("failed to request definition for node %s: %v", nodeID, err)
		}
	}

	// Verify all pending defs were created
	nodeIDs, err := fs.ListPendingDefs(tmpDir)
	if err != nil {
		t.Fatalf("ListPendingDefs failed: %v", err)
	}

	if len(nodeIDs) != 3 {
		t.Errorf("expected 3 pending definitions, got %d", len(nodeIDs))
	}

	// Verify each pending def has correct term
	for nodeIDStr, expectedTerm := range terms {
		nodeID, _ := service.ParseNodeID(nodeIDStr)
		pd, err := fs.ReadPendingDef(tmpDir, nodeID)
		if err != nil {
			t.Errorf("failed to read pending def for node %s: %v", nodeIDStr, err)
			continue
		}
		if pd.Term != expectedTerm {
			t.Errorf("node %s: expected term %q, got %q", nodeIDStr, expectedTerm, pd.Term)
		}
	}
}

// TestRequestDefCmd_OverwriteExisting tests behavior when a pending def already exists.
func TestRequestDefCmd_OverwriteExisting(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	// First request
	cmd1 := newRequestDefTestCmd()
	_, err := executeCommand(cmd1, "request-def",
		"--node", "1",
		"--term", "group",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	// Second request for same node (should overwrite or error gracefully)
	cmd2 := newRequestDefTestCmd()
	_, err = executeCommand(cmd2, "request-def",
		"--node", "1",
		"--term", "ring", // different term
		"--dir", tmpDir,
	)

	// Behavior depends on implementation:
	// - Could succeed (overwrites existing)
	// - Could error (only one pending def per node)
	// Either is acceptable; test documents the behavior
	if err != nil {
		t.Logf("Second request on same node returned error: %v", err)
		t.Logf("This may be expected if only one pending def per node is allowed")
	} else {
		t.Logf("Second request on same node succeeded")
		// Verify the term was updated
		nodeID, _ := service.ParseNodeID("1")
		pd, err := fs.ReadPendingDef(tmpDir, nodeID)
		if err != nil {
			t.Fatalf("failed to read pending def: %v", err)
		}
		if pd.Term != "ring" {
			t.Logf("Term not updated: expected 'ring', got '%s'", pd.Term)
		}
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestRequestDefCmd_Help tests that help output shows usage information.
func TestRequestDefCmd_Help(t *testing.T) {
	cmd := newRequestDefTestCmd()
	output, err := executeCommand(cmd, "request-def", "--help")

	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	// Check for expected help content
	expectations := []string{
		"request-def",
		"--node",
		"--term",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestRequestDefCmd_HelpShortFlag tests help with short flag.
func TestRequestDefCmd_HelpShortFlag(t *testing.T) {
	cmd := newRequestDefTestCmd()
	output, err := executeCommand(cmd, "request-def", "-h")

	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	if !strings.Contains(output, "request-def") {
		t.Errorf("help output should mention 'request-def', got: %q", output)
	}
}

// =============================================================================
// Short Flags Tests
// =============================================================================

// TestRequestDefCmd_ShortFlags tests that short flags work correctly.
func TestRequestDefCmd_ShortFlags(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	output, err := executeCommand(cmd, "request-def",
		"-n", "1",
		"-t", "vector_space",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	// Verify the request was created
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "pending") && !strings.Contains(lower, "request") && !strings.Contains(lower, "definition") {
		t.Errorf("expected output to confirm request, got: %q", output)
	}

	// Verify filesystem entry
	nodeID, _ := service.ParseNodeID("1")
	pd, err := fs.ReadPendingDef(tmpDir, nodeID)
	if err != nil {
		t.Fatalf("expected pending def to be readable: %v", err)
	}

	if pd.Term != "vector_space" {
		t.Errorf("expected term 'vector_space', got: %q", pd.Term)
	}
}

// =============================================================================
// Default Directory Tests
// =============================================================================

// TestRequestDefCmd_DefaultDirectory tests request-def uses current directory by default.
func TestRequestDefCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
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
	cmd := newRequestDefTestCmd()
	output, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", "group",
	)

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Verify the request was created
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "pending") && !strings.Contains(lower, "request") && !strings.Contains(lower, "definition") {
		t.Errorf("expected output to confirm request, got: %q", output)
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestRequestDefCmd_TableDrivenTerms tests various term formats.
func TestRequestDefCmd_TableDrivenTerms(t *testing.T) {
	tests := []struct {
		name    string
		term    string
		wantErr bool
	}{
		{
			name:    "simple term",
			term:    "group",
			wantErr: false,
		},
		{
			name:    "term with underscore",
			term:    "vector_space",
			wantErr: false,
		},
		{
			name:    "term with hyphen",
			term:    "well-formed",
			wantErr: false,
		},
		{
			name:    "term with spaces",
			term:    "linear map",
			wantErr: false,
		},
		{
			name:    "term with special chars",
			term:    "n-dimensional",
			wantErr: false,
		},
		{
			name:    "empty term",
			term:    "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			term:    "   ",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupRequestDefTest(t)
			defer cleanup()

			cmd := newRequestDefTestCmd()
			_, err := executeCommand(cmd, "request-def",
				"--node", "1",
				"--term", tc.term,
				"--dir", tmpDir,
			)

			if tc.wantErr && err == nil {
				t.Errorf("expected error for term %q, got nil", tc.term)
			}

			if !tc.wantErr && err != nil {
				t.Errorf("expected no error for term %q, got: %v", tc.term, err)
			}
		})
	}
}

// TestRequestDefCmd_TableDrivenNodeIDs tests various node IDs.
func TestRequestDefCmd_TableDrivenNodeIDs(t *testing.T) {
	tests := []struct {
		name      string
		nodeID    string
		setupNode bool // whether to create the node first (uses setupRequestDefTestWithMultipleNodes)
		wantErr   bool
	}{
		{
			name:      "valid root",
			nodeID:    "1",
			setupNode: true,
			wantErr:   false,
		},
		{
			name:      "valid child",
			nodeID:    "1.1",
			setupNode: true,
			wantErr:   false,
		},
		{
			name:      "non-existent node",
			nodeID:    "1.999",
			setupNode: false,
			wantErr:   true,
		},
		{
			name:      "invalid format",
			nodeID:    "abc",
			setupNode: false,
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var tmpDir string
			var cleanup func()

			if tc.setupNode && strings.Contains(tc.nodeID, ".") {
				tmpDir, cleanup = setupRequestDefTestWithMultipleNodes(t)
			} else {
				tmpDir, cleanup = setupRequestDefTest(t)
			}
			defer cleanup()

			cmd := newRequestDefTestCmd()
			_, err := executeCommand(cmd, "request-def",
				"--node", tc.nodeID,
				"--term", "test",
				"--dir", tmpDir,
			)

			if tc.wantErr && err == nil {
				t.Errorf("expected error for node ID %q, got nil", tc.nodeID)
			}

			if !tc.wantErr && err != nil {
				t.Errorf("expected no error for node ID %q, got: %v", tc.nodeID, err)
			}
		})
	}
}

// =============================================================================
// Success Message Tests
// =============================================================================

// TestRequestDefCmd_SuccessMessage tests that success message is informative.
func TestRequestDefCmd_SuccessMessage(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	cmd := newRequestDefTestCmd()
	output, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", "isomorphism",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Success message should indicate what happened
	lower := strings.ToLower(output)
	hasStatusInfo := strings.Contains(lower, "pending") ||
		strings.Contains(lower, "request") ||
		strings.Contains(lower, "definition") ||
		strings.Contains(lower, "created")

	if !hasStatusInfo {
		t.Errorf("success message should mention pending/request/definition/created, got: %q", output)
	}

	// Should suggest next steps or provide useful info
	hasNodeInfo := strings.Contains(output, "1") || strings.Contains(output, "node")
	hasTermInfo := strings.Contains(output, "isomorphism") || strings.Contains(output, "term")

	if !hasNodeInfo && !hasTermInfo {
		t.Logf("Warning: success message doesn't include node or term info: %q", output)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestRequestDefCmd_LongTerm tests requesting a definition with a very long term.
func TestRequestDefCmd_LongTerm(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	// Create a very long but valid term
	longTerm := strings.Repeat("mathematical_concept_", 50)

	cmd := newRequestDefTestCmd()
	_, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", longTerm,
		"--dir", tmpDir,
	)

	// Should handle long terms gracefully (either succeed or return clear error)
	if err != nil {
		t.Logf("Long term returned error: %v", err)
		// If there's a length limit, that's acceptable
	} else {
		// Verify it was saved
		nodeID, _ := service.ParseNodeID("1")
		pd, err := fs.ReadPendingDef(tmpDir, nodeID)
		if err != nil {
			t.Fatalf("failed to read pending def: %v", err)
		}
		if pd.Term != longTerm {
			t.Errorf("term not saved correctly")
		}
	}
}

// TestRequestDefCmd_SpecialCharactersInTerm tests special characters in term.
func TestRequestDefCmd_SpecialCharactersInTerm(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	specialTerm := "epsilon-delta_continuity"

	cmd := newRequestDefTestCmd()
	_, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", specialTerm,
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with special characters in term, got: %v", err)
	}

	// Verify the term was saved correctly
	nodeID, _ := service.ParseNodeID("1")
	pd, err := fs.ReadPendingDef(tmpDir, nodeID)
	if err != nil {
		t.Fatalf("failed to read pending def: %v", err)
	}

	if pd.Term != specialTerm {
		t.Errorf("term mismatch: expected %q, got %q", specialTerm, pd.Term)
	}
}

// TestRequestDefCmd_UnicodeInTerm tests unicode characters in term.
func TestRequestDefCmd_UnicodeInTerm(t *testing.T) {
	tmpDir, cleanup := setupRequestDefTest(t)
	defer cleanup()

	unicodeTerm := "epsilon"

	cmd := newRequestDefTestCmd()
	_, err := executeCommand(cmd, "request-def",
		"--node", "1",
		"--term", unicodeTerm,
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with unicode in term, got: %v", err)
	}

	// Verify the term was saved correctly
	nodeID, _ := service.ParseNodeID("1")
	pd, err := fs.ReadPendingDef(tmpDir, nodeID)
	if err != nil {
		t.Fatalf("failed to read pending def: %v", err)
	}

	if pd.Term != unicodeTerm {
		t.Errorf("term mismatch: expected %q, got %q", unicodeTerm, pd.Term)
	}
}
