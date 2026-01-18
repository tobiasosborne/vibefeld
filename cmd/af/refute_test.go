//go:build integration

// Package main contains tests for the af refute command.
// These are TDD tests - the refute command does not exist yet.
// Tests define the expected behavior for refuting proof nodes.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// mustParseRefuteNodeID parses a NodeID string or fails the test.
func mustParseRefuteNodeID(t *testing.T, s string) service.NodeID {
	t.Helper()
	id, err := service.ParseNodeID(s)
	if err != nil {
		t.Fatalf("Failed to parse NodeID %q: %v", s, err)
	}
	return id
}

// setupRefuteTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupRefuteTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-refute-test-*")
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

// setupRefuteTestWithNode creates a test environment with an initialized proof
// and a single pending node at ID "1".
// Note: service.Init() already creates node 1 with the conjecture, so we just
// return the base setup.
func setupRefuteTestWithNode(t *testing.T) (string, func()) {
	t.Helper()
	return setupRefuteTest(t)
}

// executeRefuteCommand creates and executes a refute command with the given arguments.
func executeRefuteCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newRefuteCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// newRefuteCmd is expected to be implemented in refute.go
// This test file uses the real implementation when available.

// =============================================================================
// Test Cases
// =============================================================================

// TestRefuteCmd_Success tests refuting a pending node successfully.
func TestRefuteCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// Execute refute command
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify output contains success message
	if !strings.Contains(strings.ToLower(output), "refute") && !strings.Contains(strings.ToLower(output), "refuted") {
		t.Errorf("expected success message mentioning refute or refuted, got: %q", output)
	}

	// Verify node state changed to refuted
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseRefuteNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after refute")
	}

	if n.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicRefuted)
	}
}

// TestRefuteCmd_MissingNodeID tests that missing node ID produces an error.
func TestRefuteCmd_MissingNodeID(t *testing.T) {
	tmpDir, cleanup := setupRefuteTest(t)
	defer cleanup()

	// Execute without node ID
	_, err := executeRefuteCommand(t, "-d", tmpDir)
	if err == nil {
		t.Fatal("expected error for missing node ID, got nil")
	}

	// Should contain error about missing argument
	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestRefuteCmd_NodeNotFound tests error when node doesn't exist.
func TestRefuteCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupRefuteTest(t)
	defer cleanup()

	// Execute with non-existent node (node 1 exists from Init, so use node 2)
	output, err := executeRefuteCommand(t, "2", "-d", tmpDir)

	// Should error or output should mention not found
	if err == nil {
		if !strings.Contains(strings.ToLower(output), "not found") && !strings.Contains(strings.ToLower(output), "error") {
			t.Errorf("expected error or 'not found' message, got: %q", output)
		}
	}
}

// TestRefuteCmd_NodeAlreadyRefuted tests warning when node is already refuted.
func TestRefuteCmd_NodeAlreadyRefuted(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// First, refute the node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseRefuteNodeID(t, "1")
	if err := svc.RefuteNode(nodeID); err != nil {
		t.Fatalf("failed to pre-refute node: %v", err)
	}

	// Try to refute again
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir)

	// Should produce error or warning
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should indicate the node is already refuted or warn about redundant action
	if !strings.Contains(strings.ToLower(combined), "already") &&
		!strings.Contains(strings.ToLower(combined), "refuted") &&
		err == nil {
		t.Logf("Warning: refuting already refuted node silently succeeded. Output: %q", output)
		// This may be acceptable behavior (idempotent) but worth noting
	}
}

// TestRefuteCmd_InvalidNodeID tests error for malformed node ID.
func TestRefuteCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupRefuteTest(t)
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
			output, err := executeRefuteCommand(t, tc.nodeID, "-d", tmpDir)

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

// TestRefuteCmd_NonPendingNode tests error if node not in pending state.
func TestRefuteCmd_NonPendingNode(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// First, validate the node (move it out of pending state)
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseRefuteNodeID(t, "1")
	if err := svc.AcceptNode(nodeID); err != nil {
		t.Fatalf("failed to validate node: %v", err)
	}

	// Try to refute a validated node
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir)

	// Should produce error - cannot refute a validated node
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "validated") &&
		!strings.Contains(strings.ToLower(combined), "cannot") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		!strings.Contains(strings.ToLower(combined), "pending") &&
		err == nil {
		t.Errorf("expected error for refuting non-pending node, got output: %q", output)
	}
}

// TestRefuteCmd_JSONOutput tests JSON output format support.
func TestRefuteCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// Execute with JSON format
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir, "-f", "json")
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

// TestRefuteCmd_ProofNotInitialized tests error when no proof exists.
func TestRefuteCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-refute-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Execute refute on uninitialized directory
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir)

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

// TestRefuteCmd_DirFlag tests --dir flag works correctly.
func TestRefuteCmd_DirFlag(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// Test short form -d
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -d flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was refuted
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	n := st.GetNode(mustParseRefuteNodeID(t, "1"))
	if n == nil || n.EpistemicState != schema.EpistemicRefuted {
		t.Error("node not refuted with -d short flag")
	}
}

// TestRefuteCmd_WithReason tests optionally accepts refutation reason.
func TestRefuteCmd_WithReason(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// Execute with reason flag
	reason := "The claim contradicts established theorem 3.2"
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir, "--reason", reason)
	if err != nil {
		t.Fatalf("expected no error with --reason flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was refuted
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseRefuteNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after refute")
	}

	if n.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicRefuted)
	}

	// Note: The reason may be stored in the ledger event or node metadata
	// The test primarily verifies the flag is accepted without error
}

// =============================================================================
// Additional Test Cases
// =============================================================================

// TestRefuteCmd_Help tests that help output shows usage information.
func TestRefuteCmd_Help(t *testing.T) {
	cmd := newRefuteCmd()
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
		"refute",
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

// TestRefuteCmd_SuccessMessage tests that success message shows node status changed.
func TestRefuteCmd_SuccessMessage(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	output, err := executeRefuteCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Success message should indicate what happened
	lower := strings.ToLower(output)
	hasStatusInfo := strings.Contains(lower, "refuted") ||
		strings.Contains(lower, "refute") ||
		strings.Contains(lower, "success")

	if !hasStatusInfo {
		t.Errorf("success message should mention refutation, got: %q", output)
	}
}

// TestRefuteCmd_UpdatesEpistemicState tests that refute updates epistemic state to refuted.
func TestRefuteCmd_UpdatesEpistemicState(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// Verify initial state is pending
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseRefuteNodeID(t, "1")
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	if n.EpistemicState != schema.EpistemicPending {
		t.Fatalf("expected initial state to be pending, got: %s", n.EpistemicState)
	}

	// Execute refute
	_, err = executeRefuteCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("refute failed: %v", err)
	}

	// Verify state changed to refuted
	st, err = svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n = st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after refute")
	}

	if n.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("expected EpistemicState = refuted, got: %s", n.EpistemicState)
	}
}

// TestRefuteCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestRefuteCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeRefuteCommand(t, "1", "-d", "/nonexistent/path/12345")

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

// TestRefuteCmd_DefaultDirectory tests refute uses current directory by default.
func TestRefuteCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
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
	output, err := executeRefuteCommand(t, "1")
	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Verify node was refuted
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseRefuteNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	if n.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("node not refuted when using default directory")
	}
}

// TestRefuteCmd_ChildNode tests refuting a child node (e.g., "1.1").
func TestRefuteCmd_ChildNode(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// Create a child node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	childID := mustParseRefuteNodeID(t, "1.1")
	err = svc.CreateNode(childID, schema.NodeTypeClaim, "Child node statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create child node: %v", err)
	}

	// Refute the child node
	output, err := executeRefuteCommand(t, "1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify child node was refuted
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(childID)
	if n == nil {
		t.Fatal("child node not found after refute")
	}

	if n.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("child node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicRefuted)
	}
}

// TestRefuteCmd_DeepNode tests refuting a deeply nested node (e.g., "1.1.1.1").
func TestRefuteCmd_DeepNode(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create nested nodes: 1.1, 1.1.1, 1.1.1.1
	nodes := []string{"1.1", "1.1.1", "1.1.1.1"}
	for _, idStr := range nodes {
		nodeID := mustParseRefuteNodeID(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	// Refute the deepest node
	output, err := executeRefuteCommand(t, "1.1.1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify deep node was refuted
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	deepID := mustParseRefuteNodeID(t, "1.1.1.1")
	n := st.GetNode(deepID)
	if n == nil {
		t.Fatal("deep node not found after refute")
	}

	if n.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("deep node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicRefuted)
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestRefuteCmd_TableDrivenNodeIDs tests various valid and invalid node ID inputs.
func TestRefuteCmd_TableDrivenNodeIDs(t *testing.T) {
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
			tmpDir, cleanup := setupRefuteTest(t)
			defer cleanup()

			// Note: service.Init() already creates node 1, so we don't need to
			// create it again. For setupNode=true with "1", the node already exists.
			// For other valid IDs, we would create them as children.

			output, err := executeRefuteCommand(t, tc.nodeID, "-d", tmpDir)

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

// TestRefuteCmd_OutputFormats tests different output format options.
func TestRefuteCmd_OutputFormats(t *testing.T) {
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
			tmpDir, cleanup := setupRefuteTestWithNode(t)
			defer cleanup()

			output, err := executeRefuteCommand(t, "1", "-d", tmpDir, "-f", tc.format)
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

// TestRefuteCmd_MultipleNodesSequential tests refuting multiple nodes in sequence.
func TestRefuteCmd_MultipleNodesSequential(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create additional nodes
	for _, idStr := range []string{"1.1", "1.2"} {
		nodeID := mustParseRefuteNodeID(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement "+idStr, schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	// Refute nodes in sequence
	nodes := []string{"1", "1.1", "1.2"}
	for _, idStr := range nodes {
		output, err := executeRefuteCommand(t, idStr, "-d", tmpDir)
		if err != nil {
			t.Fatalf("failed to refute node %s: %v\nOutput: %s", idStr, err, output)
		}
	}

	// Verify all nodes are refuted
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	for _, idStr := range nodes {
		nodeID := mustParseRefuteNodeID(t, idStr)
		n := st.GetNode(nodeID)
		if n == nil {
			t.Errorf("node %s not found", idStr)
			continue
		}
		if n.EpistemicState != schema.EpistemicRefuted {
			t.Errorf("node %s EpistemicState = %q, want refuted", idStr, n.EpistemicState)
		}
	}
}

// TestRefuteCmd_DirFlagShortForm tests that -d works as short form of --dir.
func TestRefuteCmd_DirFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// Test short form -d
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -d flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was refuted
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	n := st.GetNode(mustParseRefuteNodeID(t, "1"))
	if n == nil || n.EpistemicState != schema.EpistemicRefuted {
		t.Error("node not refuted with -d short flag")
	}
}

// TestRefuteCmd_DirFlagLongForm tests that --dir works.
func TestRefuteCmd_DirFlagLongForm(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// Test long form --dir
	output, err := executeRefuteCommand(t, "1", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with --dir flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was refuted
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	n := st.GetNode(mustParseRefuteNodeID(t, "1"))
	if n == nil || n.EpistemicState != schema.EpistemicRefuted {
		t.Error("node not refuted with --dir long flag")
	}
}

// TestRefuteCmd_FormatFlagShortForm tests that -f works as short form of --format.
func TestRefuteCmd_FormatFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// Test short form -f
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error with -f flag, got: %v", err)
	}

	// Should be valid JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Errorf("expected valid JSON with -f flag, got: %v\nOutput: %q", err, output)
	}
}

// TestRefuteCmd_InvalidFormat tests error for invalid format option.
func TestRefuteCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	output, err := executeRefuteCommand(t, "1", "-d", tmpDir, "-f", "invalid")

	// Should produce error or warning about invalid format
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// The behavior depends on implementation - it might error or fall back to text
	// At minimum, it shouldn't crash
	t.Logf("Output with invalid format: %s (error: %v)", output, err)
}

// TestRefuteCmd_RelativeDirectory tests using relative directory path.
func TestRefuteCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-refute-rel-*")
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

	// Note: service.Init() already creates node 1 with the conjecture
	svc, _ := service.NewProofService(proofDir)
	nodeID := mustParseRefuteNodeID(t, "1")

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	output, err := executeRefuteCommand(t, "1", "-d", "subdir/proof")
	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}

	// Verify refutation
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	n := st.GetNode(nodeID)
	if n == nil || n.EpistemicState != schema.EpistemicRefuted {
		t.Error("node not refuted with relative directory path")
	}
}

// =============================================================================
// Refute vs Accept Interaction Tests
// =============================================================================

// TestRefuteCmd_CannotRefuteValidatedNode tests that a validated node cannot be refuted.
func TestRefuteCmd_CannotRefuteValidatedNode(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// First, accept/validate the node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseRefuteNodeID(t, "1")
	if err := svc.AcceptNode(nodeID); err != nil {
		t.Fatalf("failed to accept node: %v", err)
	}

	// Try to refute the validated node
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir)

	// Should produce error - cannot refute a validated node
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Verify the node is still validated
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	n := st.GetNode(nodeID)
	if n != nil && n.EpistemicState == schema.EpistemicRefuted {
		t.Error("node should not be refuted after being validated")
	}
}

// TestRefuteCmd_CannotRefuteAdmittedNode tests that an admitted node cannot be refuted.
func TestRefuteCmd_CannotRefuteAdmittedNode(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	// First, admit the node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseRefuteNodeID(t, "1")
	if err := svc.AdmitNode(nodeID); err != nil {
		t.Fatalf("failed to admit node: %v", err)
	}

	// Try to refute the admitted node
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir)

	// Should produce error - cannot refute an admitted node
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Verify the node is still admitted
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	n := st.GetNode(nodeID)
	if n != nil && n.EpistemicState == schema.EpistemicRefuted {
		t.Error("node should not be refuted after being admitted")
	}

	// Log the combined output for debugging
	t.Logf("Refute on admitted node result: %s", combined)
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestRefuteCmd_ExpectedFlags ensures the refute command has expected flag structure.
func TestRefuteCmd_ExpectedFlags(t *testing.T) {
	cmd := newRefuteCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected refute command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected refute command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestRefuteCmd_DefaultFlagValues verifies default values for flags.
func TestRefuteCmd_DefaultFlagValues(t *testing.T) {
	cmd := newRefuteCmd()

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

// TestRefuteCmd_ReasonFlag tests that the reason flag is supported.
func TestRefuteCmd_ReasonFlag(t *testing.T) {
	cmd := newRefuteCmd()

	// Check reason flag exists
	reasonFlag := cmd.Flags().Lookup("reason")
	if reasonFlag == nil {
		t.Error("expected refute command to have --reason flag")
	}
}

// TestRefuteCmd_CommandMetadata verifies command metadata.
func TestRefuteCmd_CommandMetadata(t *testing.T) {
	cmd := newRefuteCmd()

	if !strings.HasPrefix(cmd.Use, "refute") {
		t.Errorf("expected Use to start with 'refute', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// JSON Output Structure Tests
// =============================================================================

// TestRefuteCmd_JSONOutputStructure tests the structure of JSON output.
func TestRefuteCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	output, err := executeRefuteCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Check for expected fields (at minimum, should have some indication of success)
	// Common expected fields:
	// - node_id or nodeId
	// - status or state
	// - message or success
	t.Logf("JSON output structure: %+v", result)
}

// TestRefuteCmd_JSONOutputWithReason tests JSON output includes reason if provided.
func TestRefuteCmd_JSONOutputWithReason(t *testing.T) {
	tmpDir, cleanup := setupRefuteTestWithNode(t)
	defer cleanup()

	reason := "This claim contradicts axiom 3"
	output, err := executeRefuteCommand(t, "1", "-d", tmpDir, "-f", "json", "--reason", reason)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// The reason may or may not appear in the output depending on implementation
	t.Logf("JSON output with reason: %+v", result)
}
