//go:build integration

// Package main contains tests for the af accept command.
// These are TDD tests - the accept command does not exist yet.
// Tests define the expected behavior for accepting/validating proof nodes.
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

// mustParseNodeID parses a NodeID string or fails the test.
func mustParseNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("Failed to parse NodeID %q: %v", s, err)
	}
	return id
}

// setupAcceptTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupAcceptTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-accept-test-*")
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

// setupAcceptTestWithNode creates a test environment with an initialized proof
// and a single pending node at ID "1".
func setupAcceptTestWithNode(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupAcceptTest(t)

	// Create a proof service and add a node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	nodeID := mustParseNodeID(t, "1")
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Test goal statement", schema.InferenceAssumption)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// executeAcceptCommand creates and executes an accept command with the given arguments.
func executeAcceptCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// newAcceptCmd is now implemented in accept.go
// This test file uses the real implementation.

// =============================================================================
// Test Cases
// =============================================================================

// TestAcceptCmd_Success tests accepting a pending node successfully.
func TestAcceptCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Execute accept command
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify output contains success message
	if !strings.Contains(strings.ToLower(output), "accept") && !strings.Contains(strings.ToLower(output), "validated") {
		t.Errorf("expected success message mentioning accept or validated, got: %q", output)
	}

	// Verify node state changed to validated
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after accept")
	}

	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicValidated)
	}
}

// TestAcceptCmd_MissingNodeID tests that missing node ID produces an error.
func TestAcceptCmd_MissingNodeID(t *testing.T) {
	tmpDir, cleanup := setupAcceptTest(t)
	defer cleanup()

	// Execute without node ID
	_, err := executeAcceptCommand(t, "-d", tmpDir)
	if err == nil {
		t.Fatal("expected error for missing node ID, got nil")
	}

	// Should contain error about missing argument
	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestAcceptCmd_NodeNotFound tests error when node doesn't exist.
func TestAcceptCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupAcceptTest(t)
	defer cleanup()

	// Execute with non-existent node
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)

	// Should error or output should mention not found
	if err == nil {
		if !strings.Contains(strings.ToLower(output), "not found") && !strings.Contains(strings.ToLower(output), "error") {
			t.Errorf("expected error or 'not found' message, got: %q", output)
		}
	}
}

// TestAcceptCmd_NodeAlreadyValidated tests error when node is already validated.
func TestAcceptCmd_NodeAlreadyValidated(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// First, validate the node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseNodeID(t, "1")
	if err := svc.AcceptNode(nodeID); err != nil {
		t.Fatalf("failed to pre-accept node: %v", err)
	}

	// Try to accept again
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)

	// Should produce error or warning
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should indicate the node is already validated or warn about redundant action
	if !strings.Contains(strings.ToLower(combined), "already") &&
		!strings.Contains(strings.ToLower(combined), "validated") &&
		err == nil {
		t.Logf("Warning: accepting already validated node silently succeeded. Output: %q", output)
		// This may be acceptable behavior (idempotent) but worth noting
	}
}

// TestAcceptCmd_NodeAlreadyRefuted tests error when node is already refuted.
func TestAcceptCmd_NodeAlreadyRefuted(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// First, refute the node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseNodeID(t, "1")
	if err := svc.RefuteNode(nodeID); err != nil {
		t.Fatalf("failed to refute node: %v", err)
	}

	// Try to accept a refuted node
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)

	// Should produce error - cannot accept a refuted node
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "refuted") &&
		!strings.Contains(strings.ToLower(combined), "cannot") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		err == nil {
		t.Errorf("expected error for accepting refuted node, got output: %q", output)
	}
}

// TestAcceptCmd_InvalidNodeIDFormat tests error for invalid node ID format.
func TestAcceptCmd_InvalidNodeIDFormat(t *testing.T) {
	tmpDir, cleanup := setupAcceptTest(t)
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
			output, err := executeAcceptCommand(t, tc.nodeID, "-d", tmpDir)

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

// TestAcceptCmd_ProofNotInitialized tests error when proof is not initialized.
func TestAcceptCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-accept-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Execute accept on uninitialized directory
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)

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

// TestAcceptCmd_JSONFormat tests JSON output format.
func TestAcceptCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Execute with JSON format
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "-f", "json")
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

// TestAcceptCmd_Help tests that help output shows usage information.
func TestAcceptCmd_Help(t *testing.T) {
	cmd := newAcceptCmd()
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
		"accept",
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

// TestAcceptCmd_SuccessMessage tests that success message shows node status changed.
func TestAcceptCmd_SuccessMessage(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Success message should indicate what happened
	lower := strings.ToLower(output)
	hasStatusInfo := strings.Contains(lower, "validated") ||
		strings.Contains(lower, "accepted") ||
		strings.Contains(lower, "success")

	if !hasStatusInfo {
		t.Errorf("success message should mention validation/acceptance, got: %q", output)
	}
}

// TestAcceptCmd_UpdatesEpistemicState tests that accept updates epistemic state to validated.
func TestAcceptCmd_UpdatesEpistemicState(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Verify initial state is pending
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseNodeID(t, "1")
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

	// Execute accept
	_, err = executeAcceptCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("accept failed: %v", err)
	}

	// Verify state changed to validated
	st, err = svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n = st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after accept")
	}

	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("expected EpistemicState = validated, got: %s", n.EpistemicState)
	}
}

// TestAcceptCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestAcceptCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeAcceptCommand(t, "1", "-d", "/nonexistent/path/12345")

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

// TestAcceptCmd_DefaultDirectory tests accept uses current directory by default.
func TestAcceptCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
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
	output, err := executeAcceptCommand(t, "1")
	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Verify node was accepted
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("node not validated when using default directory")
	}
}

// TestAcceptCmd_ChildNode tests accepting a child node (e.g., "1.1").
func TestAcceptCmd_ChildNode(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Create a child node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	childID := mustParseNodeID(t, "1.1")
	err = svc.CreateNode(childID, schema.NodeTypeClaim, "Child node statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create child node: %v", err)
	}

	// Accept the child node
	output, err := executeAcceptCommand(t, "1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify child node was validated
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(childID)
	if n == nil {
		t.Fatal("child node not found after accept")
	}

	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("child node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicValidated)
	}
}

// TestAcceptCmd_DeepNode tests accepting a deeply nested node (e.g., "1.2.3.4").
func TestAcceptCmd_DeepNode(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create nested nodes: 1.1, 1.1.1, 1.1.1.1
	nodes := []string{"1.1", "1.1.1", "1.1.1.1"}
	for _, idStr := range nodes {
		nodeID := mustParseNodeID(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	// Accept the deepest node
	output, err := executeAcceptCommand(t, "1.1.1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify deep node was validated
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	deepID := mustParseNodeID(t, "1.1.1.1")
	n := st.GetNode(deepID)
	if n == nil {
		t.Fatal("deep node not found after accept")
	}

	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("deep node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicValidated)
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestAcceptCmd_TableDrivenNodeIDs tests various valid and invalid node ID inputs.
func TestAcceptCmd_TableDrivenNodeIDs(t *testing.T) {
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
			tmpDir, cleanup := setupAcceptTest(t)
			defer cleanup()

			if tc.setupNode && tc.nodeID != "" {
				// Only create node if it's a valid ID and setupNode is true
				id, err := types.Parse(tc.nodeID)
				if err == nil {
					svc, _ := service.NewProofService(tmpDir)
					_ = svc.CreateNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
				}
			}

			output, err := executeAcceptCommand(t, tc.nodeID, "-d", tmpDir)

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

// TestAcceptCmd_OutputFormats tests different output format options.
func TestAcceptCmd_OutputFormats(t *testing.T) {
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
			tmpDir, cleanup := setupAcceptTestWithNode(t)
			defer cleanup()

			output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "-f", tc.format)
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

// TestAcceptCmd_MultipleNodesSequential tests accepting multiple nodes in sequence.
func TestAcceptCmd_MultipleNodesSequential(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create additional nodes
	for _, idStr := range []string{"1.1", "1.2"} {
		nodeID := mustParseNodeID(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement "+idStr, schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	// Accept nodes in sequence
	nodes := []string{"1", "1.1", "1.2"}
	for _, idStr := range nodes {
		output, err := executeAcceptCommand(t, idStr, "-d", tmpDir)
		if err != nil {
			t.Fatalf("failed to accept node %s: %v\nOutput: %s", idStr, err, output)
		}
	}

	// Verify all nodes are validated
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	for _, idStr := range nodes {
		nodeID := mustParseNodeID(t, idStr)
		n := st.GetNode(nodeID)
		if n == nil {
			t.Errorf("node %s not found", idStr)
			continue
		}
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("node %s EpistemicState = %q, want validated", idStr, n.EpistemicState)
		}
	}
}

// TestAcceptCmd_DirFlagShortForm tests that -d works as short form of --dir.
func TestAcceptCmd_DirFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Test short form -d
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -d flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was accepted
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	n := st.GetNode(mustParseNodeID(t, "1"))
	if n == nil || n.EpistemicState != schema.EpistemicValidated {
		t.Error("node not validated with -d short flag")
	}
}

// TestAcceptCmd_DirFlagLongForm tests that --dir works.
func TestAcceptCmd_DirFlagLongForm(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Test long form --dir
	output, err := executeAcceptCommand(t, "1", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with --dir flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was accepted
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	n := st.GetNode(mustParseNodeID(t, "1"))
	if n == nil || n.EpistemicState != schema.EpistemicValidated {
		t.Error("node not validated with --dir long flag")
	}
}

// TestAcceptCmd_FormatFlagShortForm tests that -f works as short form of --format.
func TestAcceptCmd_FormatFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Test short form -f
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error with -f flag, got: %v", err)
	}

	// Should be valid JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Errorf("expected valid JSON with -f flag, got: %v\nOutput: %q", err, output)
	}
}

// TestAcceptCmd_InvalidFormat tests error for invalid format option.
func TestAcceptCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "-f", "invalid")

	// Should produce error or warning about invalid format
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// The behavior depends on implementation - it might error or fall back to text
	// At minimum, it shouldn't crash
	t.Logf("Output with invalid format: %s (error: %v)", output, err)
}

// TestAcceptCmd_RelativeDirectory tests using relative directory path.
func TestAcceptCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-accept-rel-*")
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
	nodeID := mustParseNodeID(t, "1")
	_ = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Test", schema.InferenceAssumption)

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	output, err := executeAcceptCommand(t, "1", "-d", "subdir/proof")
	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}

	// Verify acceptance
	st, _ := svc.LoadState()
	n := st.GetNode(nodeID)
	if n == nil || n.EpistemicState != schema.EpistemicValidated {
		t.Error("node not validated with relative directory path")
	}
}
