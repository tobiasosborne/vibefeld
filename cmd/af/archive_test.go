//go:build integration

// Package main contains tests for the af archive command.
// These are TDD tests - the archive command does not exist yet.
// Tests define the expected behavior for archiving proof nodes.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// mustParseArchiveNodeID parses a NodeID string or fails the test.
func mustParseArchiveNodeID(t *testing.T, s string) service.NodeID {
	t.Helper()
	id, err := service.ParseNodeID(s)
	if err != nil {
		t.Fatalf("Failed to parse NodeID %q: %v", s, err)
	}
	return id
}

// setupArchiveTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupArchiveTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-archive-test-*")
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

// setupArchiveTestWithNode creates a test environment with an initialized proof
// and a single pending node at ID "1".
// Note: service.Init() already creates node 1 with the conjecture, so we just
// return setupArchiveTest(t).
func setupArchiveTestWithNode(t *testing.T) (string, func()) {
	t.Helper()
	return setupArchiveTest(t)
}

// executeArchiveCommand creates and executes an archive command with the given arguments.
func executeArchiveCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newArchiveCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// newArchiveCmd is expected to be implemented in archive.go
// This test file uses the real implementation once it exists.

// =============================================================================
// Test Cases
// =============================================================================

// TestArchiveCmd_Success tests archiving a pending node successfully.
func TestArchiveCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// Execute archive command
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify output contains success message
	if !strings.Contains(strings.ToLower(output), "archive") {
		t.Errorf("expected success message mentioning archive, got: %q", output)
	}

	// Verify node state changed to archived
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseArchiveNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after archive")
	}

	if n.EpistemicState != service.EpistemicArchived {
		t.Errorf("node EpistemicState = %q, want %q", n.EpistemicState, service.EpistemicArchived)
	}
}

// TestArchiveCmd_MissingNodeID tests that missing node ID produces an error.
func TestArchiveCmd_MissingNodeID(t *testing.T) {
	tmpDir, cleanup := setupArchiveTest(t)
	defer cleanup()

	// Execute without node ID
	_, err := executeArchiveCommand(t, "-d", tmpDir)
	if err == nil {
		t.Fatal("expected error for missing node ID, got nil")
	}

	// Should contain error about missing argument
	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestArchiveCmd_NodeNotFound tests error when node doesn't exist.
func TestArchiveCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupArchiveTest(t)
	defer cleanup()

	// Execute with non-existent node (node 1 exists from Init, so use node 2)
	output, err := executeArchiveCommand(t, "2", "-d", tmpDir)

	// Should error or output should mention not found
	if err == nil {
		if !strings.Contains(strings.ToLower(output), "not found") && !strings.Contains(strings.ToLower(output), "error") {
			t.Errorf("expected error or 'not found' message, got: %q", output)
		}
	}
}

// TestArchiveCmd_NodeAlreadyArchived tests warning when node is already archived.
func TestArchiveCmd_NodeAlreadyArchived(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// First, archive the node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseArchiveNodeID(t, "1")
	// ArchiveNode method will be implemented - this is TDD
	if err := svc.ArchiveNode(nodeID); err != nil {
		t.Fatalf("failed to pre-archive node: %v", err)
	}

	// Try to archive again
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir)

	// Should produce error or warning
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should indicate the node is already archived or warn about redundant action
	if !strings.Contains(strings.ToLower(combined), "already") &&
		!strings.Contains(strings.ToLower(combined), "archived") &&
		err == nil {
		t.Logf("Warning: archiving already archived node silently succeeded. Output: %q", output)
		// This may be acceptable behavior (idempotent) but worth noting
	}
}

// TestArchiveCmd_InvalidNodeID tests error for malformed node ID.
func TestArchiveCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupArchiveTest(t)
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
		{"non-root start", "2.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeArchiveCommand(t, tc.nodeID, "-d", tmpDir)

			// Should produce error for invalid format
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

// TestArchiveCmd_NonPendingNode tests error when trying to archive a non-pending node.
func TestArchiveCmd_NonPendingNode(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// First, validate the node (move from pending to validated)
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseArchiveNodeID(t, "1")
	if err := svc.AcceptNode(nodeID); err != nil {
		t.Fatalf("failed to validate node: %v", err)
	}

	// Try to archive a validated node
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir)

	// Should produce error - cannot archive a validated node
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// The behavior depends on whether we allow archiving validated nodes
	// According to schema, archive is a final state like validated/refuted
	// So transitioning from validated->archived should be invalid
	if !strings.Contains(strings.ToLower(combined), "validated") &&
		!strings.Contains(strings.ToLower(combined), "cannot") &&
		!strings.Contains(strings.ToLower(combined), "terminal") &&
		!strings.Contains(strings.ToLower(combined), "final") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		err == nil {
		t.Errorf("expected error for archiving validated node, got output: %q", output)
	}
}

// TestArchiveCmd_JSONOutput tests JSON output format support.
func TestArchiveCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// Execute with JSON format
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Should contain relevant fields
	if _, ok := result["node_id"]; !ok {
		if _, ok := result["nodeId"]; !ok {
			t.Log("Warning: JSON output does not contain node_id or nodeId field")
		}
	}
}

// TestArchiveCmd_ProofNotInitialized tests error when no proof exists.
func TestArchiveCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-archive-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Execute archive on uninitialized directory
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir)

	// Should produce error
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

// TestArchiveCmd_DirFlag tests that --dir flag works correctly.
func TestArchiveCmd_DirFlag(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// Test long form --dir
	output, err := executeArchiveCommand(t, "1", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with --dir flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was archived
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	n := st.GetNode(mustParseArchiveNodeID(t, "1"))
	if n == nil || n.EpistemicState != service.EpistemicArchived {
		t.Error("node not archived with --dir long flag")
	}
}

// TestArchiveCmd_WithReason tests that --reason flag is optionally accepted.
func TestArchiveCmd_WithReason(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// Execute with reason flag
	reason := "This proof path is no longer relevant"
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir, "--reason", reason)
	if err != nil {
		t.Fatalf("expected no error with --reason flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was archived
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseArchiveNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after archive")
	}

	if n.EpistemicState != service.EpistemicArchived {
		t.Errorf("node EpistemicState = %q, want %q", n.EpistemicState, service.EpistemicArchived)
	}

	// The reason may or may not be stored in the node/ledger - this is implementation specific
	// The test documents that the flag should be accepted without error
}

// =============================================================================
// Additional Test Cases
// =============================================================================

// TestArchiveCmd_Help tests that help output shows usage information.
func TestArchiveCmd_Help(t *testing.T) {
	cmd := newArchiveCmd()
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
		"archive",
		"node-id",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestArchiveCmd_SuccessMessage tests that success message shows node status changed.
func TestArchiveCmd_SuccessMessage(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	output, err := executeArchiveCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Success message should indicate what happened
	lower := strings.ToLower(output)
	hasStatusInfo := strings.Contains(lower, "archived") ||
		strings.Contains(lower, "success")

	if !hasStatusInfo {
		t.Errorf("success message should mention archived/success, got: %q", output)
	}
}

// TestArchiveCmd_UpdatesEpistemicState tests that archive updates epistemic state to archived.
func TestArchiveCmd_UpdatesEpistemicState(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// Verify initial state is pending
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseArchiveNodeID(t, "1")
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	if n.EpistemicState != service.EpistemicPending {
		t.Fatalf("expected initial state to be pending, got: %s", n.EpistemicState)
	}

	// Execute archive
	_, err = executeArchiveCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Verify state changed to archived
	st, err = svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n = st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after archive")
	}

	if n.EpistemicState != service.EpistemicArchived {
		t.Errorf("expected EpistemicState = archived, got: %s", n.EpistemicState)
	}
}

// TestArchiveCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestArchiveCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeArchiveCommand(t, "1", "-d", "/nonexistent/path/12345")

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

// TestArchiveCmd_DefaultDirectory tests archive uses current directory by default.
func TestArchiveCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
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
	output, err := executeArchiveCommand(t, "1")
	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Verify node was archived
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseArchiveNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	if n.EpistemicState != service.EpistemicArchived {
		t.Errorf("node not archived when using default directory")
	}
}

// TestArchiveCmd_ChildNode tests archiving a child node (e.g., "1.1").
func TestArchiveCmd_ChildNode(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// Create a child node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	childID := mustParseArchiveNodeID(t, "1.1")
	err = svc.CreateNode(childID, service.NodeTypeClaim, "Child node statement", service.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create child node: %v", err)
	}

	// Archive the child node
	output, err := executeArchiveCommand(t, "1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify child node was archived
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(childID)
	if n == nil {
		t.Fatal("child node not found after archive")
	}

	if n.EpistemicState != service.EpistemicArchived {
		t.Errorf("child node EpistemicState = %q, want %q", n.EpistemicState, service.EpistemicArchived)
	}
}

// TestArchiveCmd_DeepNode tests archiving a deeply nested node (e.g., "1.1.1.1").
func TestArchiveCmd_DeepNode(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create nested nodes: 1.1, 1.1.1, 1.1.1.1
	nodes := []string{"1.1", "1.1.1", "1.1.1.1"}
	for _, idStr := range nodes {
		nodeID := mustParseArchiveNodeID(t, idStr)
		err = svc.CreateNode(nodeID, service.NodeTypeClaim, "Statement for "+idStr, service.InferenceModusPonens)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	// Archive the deepest node
	output, err := executeArchiveCommand(t, "1.1.1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify deep node was archived
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	deepID := mustParseArchiveNodeID(t, "1.1.1.1")
	n := st.GetNode(deepID)
	if n == nil {
		t.Fatal("deep node not found after archive")
	}

	if n.EpistemicState != service.EpistemicArchived {
		t.Errorf("deep node EpistemicState = %q, want %q", n.EpistemicState, service.EpistemicArchived)
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestArchiveCmd_TableDrivenNodeIDs tests various valid and invalid node ID inputs.
func TestArchiveCmd_TableDrivenNodeIDs(t *testing.T) {
	tests := []struct {
		name        string
		nodeID      string
		setupNode   bool // whether to create the node first (node 1 already exists from Init)
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid root",
			nodeID:    "1",
			setupNode: false, // node 1 already exists from Init
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
			tmpDir, cleanup := setupArchiveTest(t)
			defer cleanup()

			if tc.setupNode && tc.nodeID != "" {
				// Only create node if it's a valid ID and setupNode is true
				id, err := service.ParseNodeID(tc.nodeID)
				if err == nil {
					svc, _ := service.NewProofService(tmpDir)
					_ = svc.CreateNode(id, service.NodeTypeClaim, "Test statement", service.InferenceAssumption)
				}
			}

			output, err := executeArchiveCommand(t, tc.nodeID, "-d", tmpDir)

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

// TestArchiveCmd_OutputFormats tests different output format options.
func TestArchiveCmd_OutputFormats(t *testing.T) {
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
				if output == "" {
					t.Error("expected non-empty text output")
				}
			},
		},
		{
			name:   "json format",
			format: "json",
			validator: func(t *testing.T, output string) {
				// JSON format should be valid JSON
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(output), &data); err != nil {
					t.Errorf("expected valid JSON, got error: %v\nOutput: %q", err, output)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupArchiveTestWithNode(t)
			defer cleanup()

			output, err := executeArchiveCommand(t, "1", "-d", tmpDir, "-f", tc.format)
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

// TestArchiveCmd_MultipleNodesSequential tests archiving multiple nodes in sequence.
func TestArchiveCmd_MultipleNodesSequential(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create additional nodes
	for _, idStr := range []string{"1.1", "1.2"} {
		nodeID := mustParseArchiveNodeID(t, idStr)
		err = svc.CreateNode(nodeID, service.NodeTypeClaim, "Statement "+idStr, service.InferenceModusPonens)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	// Archive nodes in sequence
	nodes := []string{"1.1", "1.2"}
	for _, idStr := range nodes {
		output, err := executeArchiveCommand(t, idStr, "-d", tmpDir)
		if err != nil {
			t.Fatalf("failed to archive node %s: %v\nOutput: %s", idStr, err, output)
		}
	}

	// Verify all archived nodes are archived
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	for _, idStr := range nodes {
		nodeID := mustParseArchiveNodeID(t, idStr)
		n := st.GetNode(nodeID)
		if n == nil {
			t.Errorf("node %s not found", idStr)
			continue
		}
		if n.EpistemicState != service.EpistemicArchived {
			t.Errorf("node %s EpistemicState = %q, want archived", idStr, n.EpistemicState)
		}
	}
}

// TestArchiveCmd_DirFlagShortForm tests that -d works as short form of --dir.
func TestArchiveCmd_DirFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// Test short form -d
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -d flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was archived
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	n := st.GetNode(mustParseArchiveNodeID(t, "1"))
	if n == nil || n.EpistemicState != service.EpistemicArchived {
		t.Error("node not archived with -d short flag")
	}
}

// TestArchiveCmd_FormatFlagShortForm tests that -f works as short form of --format.
func TestArchiveCmd_FormatFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// Test short form -f
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error with -f flag, got: %v", err)
	}

	// Should be valid JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Errorf("expected valid JSON with -f flag, got: %v\nOutput: %q", err, output)
	}
}

// TestArchiveCmd_InvalidFormat tests error for invalid format option.
func TestArchiveCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	output, err := executeArchiveCommand(t, "1", "-d", tmpDir, "-f", "invalid")

	// Should produce error or warning about invalid format
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// The behavior depends on implementation - it might error or fall back to text
	// At minimum, it shouldn't crash
	t.Logf("Output with invalid format: %s (error: %v)", output, err)
}

// TestArchiveCmd_RelativeDirectory tests using relative directory path.
func TestArchiveCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-archive-rel-*")
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

	// Node 1 already exists from Init, no need to create it
	svc, _ := service.NewProofService(proofDir)
	nodeID := mustParseArchiveNodeID(t, "1")

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	output, err := executeArchiveCommand(t, "1", "-d", "subdir/proof")
	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}

	// Verify archival
	st, _ := svc.LoadState()
	n := st.GetNode(nodeID)
	if n == nil || n.EpistemicState != service.EpistemicArchived {
		t.Error("node not archived with relative directory path")
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestArchiveCmd_ExpectedFlags ensures the archive command has expected flag structure.
func TestArchiveCmd_ExpectedFlags(t *testing.T) {
	cmd := newArchiveCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected archive command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected archive command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestArchiveCmd_DefaultFlagValues verifies default values for flags.
func TestArchiveCmd_DefaultFlagValues(t *testing.T) {
	cmd := newArchiveCmd()

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

// TestArchiveCmd_CommandMetadata verifies command metadata.
func TestArchiveCmd_CommandMetadata(t *testing.T) {
	cmd := newArchiveCmd()

	if !strings.HasPrefix(cmd.Use, "archive") {
		t.Errorf("expected Use to start with 'archive', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Epistemic State Transition Tests
// =============================================================================

// TestArchiveCmd_FromRefutedNode tests error when trying to archive a refuted node.
func TestArchiveCmd_FromRefutedNode(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// First, refute the node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseArchiveNodeID(t, "1")
	if err := svc.RefuteNode(nodeID); err != nil {
		t.Fatalf("failed to refute node: %v", err)
	}

	// Try to archive a refuted node
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir)

	// Should produce error - cannot archive a refuted node (both are terminal states)
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "refuted") &&
		!strings.Contains(strings.ToLower(combined), "cannot") &&
		!strings.Contains(strings.ToLower(combined), "terminal") &&
		!strings.Contains(strings.ToLower(combined), "final") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		err == nil {
		t.Errorf("expected error for archiving refuted node, got output: %q", output)
	}
}

// TestArchiveCmd_FromAdmittedNode tests error when trying to archive an admitted node.
func TestArchiveCmd_FromAdmittedNode(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// First, admit the node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseArchiveNodeID(t, "1")
	if err := svc.AdmitNode(nodeID); err != nil {
		t.Fatalf("failed to admit node: %v", err)
	}

	// Try to archive an admitted node
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir)

	// Should produce error - cannot archive an admitted node (both are terminal states)
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "admitted") &&
		!strings.Contains(strings.ToLower(combined), "cannot") &&
		!strings.Contains(strings.ToLower(combined), "terminal") &&
		!strings.Contains(strings.ToLower(combined), "final") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		err == nil {
		t.Errorf("expected error for archiving admitted node, got output: %q", output)
	}
}

// =============================================================================
// Reason Flag Tests
// =============================================================================

// TestArchiveCmd_ReasonFlagLongForm tests that --reason works.
func TestArchiveCmd_ReasonFlagLongForm(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	output, err := executeArchiveCommand(t, "1", "-d", tmpDir, "--reason", "Proof path abandoned")
	if err != nil {
		t.Fatalf("expected no error with --reason flag, got: %v", err)
	}

	// Verify node was archived
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	n := st.GetNode(mustParseArchiveNodeID(t, "1"))
	if n == nil || n.EpistemicState != service.EpistemicArchived {
		t.Error("node not archived with --reason flag")
	}

	t.Logf("Output with reason: %s", output)
}

// TestArchiveCmd_EmptyReason tests behavior with empty reason.
func TestArchiveCmd_EmptyReason(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	// Empty reason should be acceptable (reason is optional)
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir, "--reason", "")
	if err != nil {
		t.Fatalf("expected no error with empty reason, got: %v", err)
	}

	// Verify node was archived
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	n := st.GetNode(mustParseArchiveNodeID(t, "1"))
	if n == nil || n.EpistemicState != service.EpistemicArchived {
		t.Error("node not archived with empty reason")
	}

	t.Logf("Output with empty reason: %s", output)
}

// TestArchiveCmd_JSONOutputWithReason tests JSON output includes reason if provided.
func TestArchiveCmd_JSONOutputWithReason(t *testing.T) {
	tmpDir, cleanup := setupArchiveTestWithNode(t)
	defer cleanup()

	reason := "No longer pursuing this approach"
	output, err := executeArchiveCommand(t, "1", "-d", tmpDir, "-f", "json", "--reason", reason)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// The JSON may or may not include the reason - depends on implementation
	t.Logf("JSON output with reason: %s", output)
}
