//go:build integration

// Package main contains tests for the af admit command.
// These are TDD tests - the admit command does not exist yet.
// Tests define the expected behavior for admitting proof nodes
// (accepting without full verification, which introduces taint).
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
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// mustParseNodeIDAdmit parses a NodeID string or fails the test.
func mustParseNodeIDAdmit(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("Failed to parse NodeID %q: %v", s, err)
	}
	return id
}

// setupAdmitTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupAdmitTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-admit-test-*")
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

// setupAdmitTestWithNode creates a test environment with an initialized proof
// and a single pending node at ID "1".
func setupAdmitTestWithNode(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupAdmitTest(t)

	// Create a proof service and add a node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	nodeID := mustParseNodeIDAdmit(t, "1")
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Test goal statement", schema.InferenceAssumption)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// executeAdmitCommand creates and executes an admit command with the given arguments.
func executeAdmitCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newAdmitCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// newAdmitCmd is expected to be implemented in admit.go
// This test file uses the real implementation.

// =============================================================================
// Test Cases
// =============================================================================

// TestAdmitCmd_Success tests admitting a pending node successfully.
func TestAdmitCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	// Execute admit command
	output, err := executeAdmitCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify output contains success message
	if !strings.Contains(strings.ToLower(output), "admit") {
		t.Errorf("expected success message mentioning admit, got: %q", output)
	}

	// Verify node state changed to admitted
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseNodeIDAdmit(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after admit")
	}

	if n.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicAdmitted)
	}
}

// TestAdmitCmd_MissingNodeID tests that missing node ID produces an error.
func TestAdmitCmd_MissingNodeID(t *testing.T) {
	tmpDir, cleanup := setupAdmitTest(t)
	defer cleanup()

	// Execute without node ID
	_, err := executeAdmitCommand(t, "-d", tmpDir)
	if err == nil {
		t.Fatal("expected error for missing node ID, got nil")
	}

	// Should contain error about missing argument
	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestAdmitCmd_NodeNotFound tests error when node doesn't exist.
func TestAdmitCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupAdmitTest(t)
	defer cleanup()

	// Execute with non-existent node
	output, err := executeAdmitCommand(t, "1", "-d", tmpDir)

	// Should error or output should mention not found
	if err == nil {
		if !strings.Contains(strings.ToLower(output), "not found") && !strings.Contains(strings.ToLower(output), "error") {
			t.Errorf("expected error or 'not found' message, got: %q", output)
		}
	}
}

// TestAdmitCmd_NodeAlreadyAdmitted tests warning when node is already admitted.
func TestAdmitCmd_NodeAlreadyAdmitted(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	// First, admit the node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseNodeIDAdmit(t, "1")
	if err := svc.AdmitNode(nodeID); err != nil {
		t.Fatalf("failed to pre-admit node: %v", err)
	}

	// Try to admit again
	output, err := executeAdmitCommand(t, "1", "-d", tmpDir)

	// Should produce error or warning
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should indicate the node is already admitted or warn about redundant action
	if !strings.Contains(strings.ToLower(combined), "already") &&
		!strings.Contains(strings.ToLower(combined), "admitted") &&
		err == nil {
		t.Logf("Warning: admitting already admitted node silently succeeded. Output: %q", output)
		// This may be acceptable behavior (idempotent) but worth noting
	}
}

// TestAdmitCmd_InvalidNodeID tests error for malformed node ID.
func TestAdmitCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupAdmitTest(t)
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
			output, err := executeAdmitCommand(t, tc.nodeID, "-d", tmpDir)

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

// TestAdmitCmd_NonPendingNode tests error if node is not in pending state.
func TestAdmitCmd_NonPendingNode(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	// First, validate the node (making it non-pending)
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseNodeIDAdmit(t, "1")
	if err := svc.AcceptNode(nodeID); err != nil {
		t.Fatalf("failed to validate node: %v", err)
	}

	// Try to admit an already validated node
	output, err := executeAdmitCommand(t, "1", "-d", tmpDir)

	// Should produce error - cannot admit a non-pending node
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "validated") &&
		!strings.Contains(strings.ToLower(combined), "cannot") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		!strings.Contains(strings.ToLower(combined), "already") &&
		err == nil {
		t.Errorf("expected error for admitting non-pending (validated) node, got output: %q", output)
	}
}

// TestAdmitCmd_JSONOutput tests JSON output format support.
func TestAdmitCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	// Execute with JSON format
	output, err := executeAdmitCommand(t, "1", "-d", tmpDir, "-f", "json")
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

// TestAdmitCmd_ProofNotInitialized tests error when no proof exists.
func TestAdmitCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-admit-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Execute admit on uninitialized directory
	output, err := executeAdmitCommand(t, "1", "-d", tmpDir)

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

// TestAdmitCmd_DirFlag tests --dir flag works correctly.
func TestAdmitCmd_DirFlag(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	// Test short form -d
	t.Run("short form -d", func(t *testing.T) {
		// Create a fresh node for this sub-test
		tmpDir2, cleanup2 := setupAdmitTestWithNode(t)
		defer cleanup2()

		output, err := executeAdmitCommand(t, "1", "-d", tmpDir2)
		if err != nil {
			t.Fatalf("expected no error with -d flag, got: %v\nOutput: %s", err, output)
		}

		// Verify node was admitted
		svc, _ := service.NewProofService(tmpDir2)
		st, _ := svc.LoadState()
		n := st.GetNode(mustParseNodeIDAdmit(t, "1"))
		if n == nil || n.EpistemicState != schema.EpistemicAdmitted {
			t.Error("node not admitted with -d short flag")
		}
	})

	// Test long form --dir
	t.Run("long form --dir", func(t *testing.T) {
		output, err := executeAdmitCommand(t, "1", "--dir", tmpDir)
		if err != nil {
			t.Fatalf("expected no error with --dir flag, got: %v\nOutput: %s", err, output)
		}

		// Verify node was admitted
		svc, _ := service.NewProofService(tmpDir)
		st, _ := svc.LoadState()
		n := st.GetNode(mustParseNodeIDAdmit(t, "1"))
		if n == nil || n.EpistemicState != schema.EpistemicAdmitted {
			t.Error("node not admitted with --dir long flag")
		}
	})
}

// =============================================================================
// Additional Test Cases
// =============================================================================

// TestAdmitCmd_Help tests that help output shows usage information.
func TestAdmitCmd_Help(t *testing.T) {
	cmd := newAdmitCmd()
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
		"admit",
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

// TestAdmitCmd_NodeAlreadyRefuted tests error when node is already refuted.
func TestAdmitCmd_NodeAlreadyRefuted(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	// First, refute the node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseNodeIDAdmit(t, "1")
	if err := svc.RefuteNode(nodeID); err != nil {
		t.Fatalf("failed to refute node: %v", err)
	}

	// Try to admit a refuted node
	output, err := executeAdmitCommand(t, "1", "-d", tmpDir)

	// Should produce error - cannot admit a refuted node
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "refuted") &&
		!strings.Contains(strings.ToLower(combined), "cannot") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		err == nil {
		t.Errorf("expected error for admitting refuted node, got output: %q", output)
	}
}

// TestAdmitCmd_SuccessMessage tests that success message shows node status changed.
func TestAdmitCmd_SuccessMessage(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	output, err := executeAdmitCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Success message should indicate what happened
	lower := strings.ToLower(output)
	hasStatusInfo := strings.Contains(lower, "admitted") ||
		strings.Contains(lower, "success") ||
		strings.Contains(lower, "taint")

	if !hasStatusInfo {
		t.Errorf("success message should mention admission or taint, got: %q", output)
	}
}

// TestAdmitCmd_UpdatesEpistemicState tests that admit updates epistemic state to admitted.
func TestAdmitCmd_UpdatesEpistemicState(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	// Verify initial state is pending
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseNodeIDAdmit(t, "1")
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

	// Execute admit
	_, err = executeAdmitCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("admit failed: %v", err)
	}

	// Verify state changed to admitted
	st, err = svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n = st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after admit")
	}

	if n.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("expected EpistemicState = admitted, got: %s", n.EpistemicState)
	}
}

// TestAdmitCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestAdmitCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeAdmitCommand(t, "1", "-d", "/nonexistent/path/12345")

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

// TestAdmitCmd_DefaultDirectory tests admit uses current directory by default.
func TestAdmitCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
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
	output, err := executeAdmitCommand(t, "1")
	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Verify node was admitted
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseNodeIDAdmit(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	if n.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("node not admitted when using default directory")
	}
}

// TestAdmitCmd_ChildNode tests admitting a child node (e.g., "1.1").
func TestAdmitCmd_ChildNode(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	// Create a child node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	childID := mustParseNodeIDAdmit(t, "1.1")
	err = svc.CreateNode(childID, schema.NodeTypeClaim, "Child node statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create child node: %v", err)
	}

	// Admit the child node
	output, err := executeAdmitCommand(t, "1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify child node was admitted
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(childID)
	if n == nil {
		t.Fatal("child node not found after admit")
	}

	if n.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("child node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicAdmitted)
	}
}

// TestAdmitCmd_DeepNode tests admitting a deeply nested node (e.g., "1.1.1.1").
func TestAdmitCmd_DeepNode(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create nested nodes: 1.1, 1.1.1, 1.1.1.1
	nodes := []string{"1.1", "1.1.1", "1.1.1.1"}
	for _, idStr := range nodes {
		nodeID := mustParseNodeIDAdmit(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	// Admit the deepest node
	output, err := executeAdmitCommand(t, "1.1.1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify deep node was admitted
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	deepID := mustParseNodeIDAdmit(t, "1.1.1.1")
	n := st.GetNode(deepID)
	if n == nil {
		t.Fatal("deep node not found after admit")
	}

	if n.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("deep node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicAdmitted)
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestAdmitCmd_TableDrivenNodeIDs tests various valid and invalid node ID inputs.
func TestAdmitCmd_TableDrivenNodeIDs(t *testing.T) {
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
			tmpDir, cleanup := setupAdmitTest(t)
			defer cleanup()

			if tc.setupNode && tc.nodeID != "" {
				// Only create node if it's a valid ID and setupNode is true
				id, err := types.Parse(tc.nodeID)
				if err == nil {
					svc, _ := service.NewProofService(tmpDir)
					_ = svc.CreateNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
				}
			}

			output, err := executeAdmitCommand(t, tc.nodeID, "-d", tmpDir)

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

// TestAdmitCmd_OutputFormats tests different output format options.
func TestAdmitCmd_OutputFormats(t *testing.T) {
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
			tmpDir, cleanup := setupAdmitTestWithNode(t)
			defer cleanup()

			output, err := executeAdmitCommand(t, "1", "-d", tmpDir, "-f", tc.format)
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

// TestAdmitCmd_MultipleNodesSequential tests admitting multiple nodes in sequence.
func TestAdmitCmd_MultipleNodesSequential(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create additional nodes
	for _, idStr := range []string{"1.1", "1.2"} {
		nodeID := mustParseNodeIDAdmit(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement "+idStr, schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	// Admit nodes in sequence
	nodes := []string{"1", "1.1", "1.2"}
	for _, idStr := range nodes {
		output, err := executeAdmitCommand(t, idStr, "-d", tmpDir)
		if err != nil {
			t.Fatalf("failed to admit node %s: %v\nOutput: %s", idStr, err, output)
		}
	}

	// Verify all nodes are admitted
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	for _, idStr := range nodes {
		nodeID := mustParseNodeIDAdmit(t, idStr)
		n := st.GetNode(nodeID)
		if n == nil {
			t.Errorf("node %s not found", idStr)
			continue
		}
		if n.EpistemicState != schema.EpistemicAdmitted {
			t.Errorf("node %s EpistemicState = %q, want admitted", idStr, n.EpistemicState)
		}
	}
}

// TestAdmitCmd_FormatFlagShortForm tests that -f works as short form of --format.
func TestAdmitCmd_FormatFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	// Test short form -f
	output, err := executeAdmitCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error with -f flag, got: %v", err)
	}

	// Should be valid JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Errorf("expected valid JSON with -f flag, got: %v\nOutput: %q", err, output)
	}
}

// TestAdmitCmd_InvalidFormat tests error for invalid format option.
func TestAdmitCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	output, err := executeAdmitCommand(t, "1", "-d", tmpDir, "-f", "invalid")

	// Should produce error or warning about invalid format
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// The behavior depends on implementation - it might error or fall back to text
	// At minimum, it shouldn't crash
	t.Logf("Output with invalid format: %s (error: %v)", output, err)
}

// TestAdmitCmd_RelativeDirectory tests using relative directory path.
func TestAdmitCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-admit-rel-*")
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
	nodeID := mustParseNodeIDAdmit(t, "1")
	_ = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Test", schema.InferenceAssumption)

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	output, err := executeAdmitCommand(t, "1", "-d", "subdir/proof")
	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}

	// Verify admission
	st, _ := svc.LoadState()
	n := st.GetNode(nodeID)
	if n == nil || n.EpistemicState != schema.EpistemicAdmitted {
		t.Error("node not admitted with relative directory path")
	}
}

// =============================================================================
// Taint-Specific Tests
// =============================================================================

// TestAdmitCmd_IntroducesTaint verifies that admitting a node introduces taint.
func TestAdmitCmd_IntroducesTaint(t *testing.T) {
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	// Execute admit command
	_, err := executeAdmitCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify that the epistemic state is admitted (which introduces taint per schema)
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseNodeIDAdmit(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	// Verify EpistemicAdmitted introduces taint per schema
	if !schema.IntroducesTaint(n.EpistemicState) {
		t.Errorf("admitted node should introduce taint, but IntroducesTaint(%q) = false", n.EpistemicState)
	}
}

// TestAdmitCmd_DifferentFromAccept verifies admit produces different result than accept.
func TestAdmitCmd_DifferentFromAccept(t *testing.T) {
	// Test that admit results in "admitted" state, not "validated" state
	tmpDir, cleanup := setupAdmitTestWithNode(t)
	defer cleanup()

	// Execute admit command
	_, err := executeAdmitCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify node is admitted (not validated)
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseNodeIDAdmit(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	if n.EpistemicState == schema.EpistemicValidated {
		t.Error("admit should result in 'admitted' state, not 'validated'")
	}

	if n.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("admit should result in 'admitted' state, got: %s", n.EpistemicState)
	}
}
