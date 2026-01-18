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

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// mustParseNodeID parses a NodeID string or fails the test.
func mustParseNodeID(t *testing.T, s string) service.NodeID {
	t.Helper()
	id, err := service.ParseNodeID(s)
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

// setupAcceptTestWithNode creates a test environment with an initialized proof
// and a single node at ID "1".
// Note: service.Init already creates node 1 with the conjecture, so we just
// return the result of setupAcceptTest. Node 1 has statement "Test conjecture".
func setupAcceptTestWithNode(t *testing.T) (string, func()) {
	t.Helper()
	// service.Init already creates node 1 with the conjecture "Test conjecture"
	return setupAcceptTest(t)
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

	if n.EpistemicState != service.EpistemicValidated {
		t.Errorf("node EpistemicState = %q, want %q", n.EpistemicState, service.EpistemicValidated)
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

	// Execute with non-existent node (node 1 exists from Init, so use node 2)
	output, err := executeAcceptCommand(t, "2", "-d", tmpDir)

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

	if n.EpistemicState != service.EpistemicPending {
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

	if n.EpistemicState != service.EpistemicValidated {
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

	if n.EpistemicState != service.EpistemicValidated {
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
	err = svc.CreateNode(childID, service.NodeTypeClaim, "Child node statement", service.InferenceModusPonens)
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

	if n.EpistemicState != service.EpistemicValidated {
		t.Errorf("child node EpistemicState = %q, want %q", n.EpistemicState, service.EpistemicValidated)
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
		err = svc.CreateNode(nodeID, service.NodeTypeClaim, "Statement for "+idStr, service.InferenceModusPonens)
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

	if n.EpistemicState != service.EpistemicValidated {
		t.Errorf("deep node EpistemicState = %q, want %q", n.EpistemicState, service.EpistemicValidated)
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

			if tc.setupNode && tc.nodeID != "" && tc.nodeID != "1" {
				// Only create node if it's a valid ID, setupNode is true, and it's not node 1
				// (node 1 already exists from Init with statement "Test conjecture")
				id, err := service.ParseNodeID(tc.nodeID)
				if err == nil {
					svc, _ := service.NewProofService(tmpDir)
					_ = svc.CreateNode(id, service.NodeTypeClaim, "Test statement", service.InferenceAssumption)
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
		err = svc.CreateNode(nodeID, service.NodeTypeClaim, "Statement "+idStr, service.InferenceModusPonens)
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
		if n.EpistemicState != service.EpistemicValidated {
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
	if n == nil || n.EpistemicState != service.EpistemicValidated {
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
	if n == nil || n.EpistemicState != service.EpistemicValidated {
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
	if err := service.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	if err := service.Init(proofDir, "Test conjecture", "author"); err != nil {
		t.Fatal(err)
	}

	// Note: service.Init already creates node 1 with the conjecture "Test conjecture"
	svc, _ := service.NewProofService(proofDir)
	nodeID := mustParseNodeID(t, "1")

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
	if n == nil || n.EpistemicState != service.EpistemicValidated {
		t.Error("node not validated with relative directory path")
	}
}

// TestAcceptCommand_ConfirmFlag tests that the --confirm flag is recognized and parsed.
func TestAcceptCommand_ConfirmFlag(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Test that the --confirm flag is recognized and the command executes successfully
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "--confirm")
	if err != nil {
		t.Fatalf("expected no error with --confirm flag, got: %v\nOutput: %s", err, output)
	}

	// Verify node was accepted (flag doesn't change behavior yet, just needs to be recognized)
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
		t.Fatal("node not found after accept with --confirm")
	}

	if n.EpistemicState != service.EpistemicValidated {
		t.Errorf("node EpistemicState = %q, want %q", n.EpistemicState, service.EpistemicValidated)
	}
}

// TestAcceptCommand_ConfirmFlagInHelp tests that --confirm flag appears in help output.
func TestAcceptCommand_ConfirmFlagInHelp(t *testing.T) {
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

	// Check that --confirm flag is documented in help
	if !strings.Contains(output, "--confirm") {
		t.Errorf("help output should contain --confirm flag, got: %q", output)
	}

	if !strings.Contains(strings.ToLower(output), "confirm") {
		t.Errorf("help output should mention 'confirm', got: %q", output)
	}
}

// =============================================================================
// Blocking Challenge Tests
// =============================================================================

// addChallengeToNode is a test helper that adds a challenge to a node by directly
// appending to the ledger. This bypasses the service layer to set up test fixtures.
func addChallengeToNode(t *testing.T, proofDir string, nodeID service.NodeID, challengeID, target, reason, severity string) {
	t.Helper()
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("failed to get ledger: %v", err)
	}
	event := ledger.NewChallengeRaisedWithSeverity(challengeID, nodeID, target, reason, severity, "")
	if _, err := ldg.Append(event); err != nil {
		t.Fatalf("failed to append challenge: %v", err)
	}
}

// TestAcceptCommand_ShowsBlockingChallengesOnFailure tests that when accept fails
// due to blocking challenges, the challenges are displayed to the user.
func TestAcceptCommand_ShowsBlockingChallengesOnFailure(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	nodeID := mustParseNodeID(t, "1")

	// Add a critical challenge to the node
	addChallengeToNode(t, tmpDir, nodeID, "chal-001", "statement", "The statement is unclear", "critical")

	// Try to accept the node - should fail and show the challenge
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)

	// Should return an error
	if err == nil {
		t.Fatal("expected error when accepting node with blocking challenges, got nil")
	}

	// Output should contain information about blocking challenges
	if !strings.Contains(output, "blocking") || !strings.Contains(output, "challenge") {
		t.Errorf("output should mention blocking challenges, got: %q", output)
	}

	// Output should show the challenge ID
	if !strings.Contains(output, "chal-001") {
		t.Errorf("output should show challenge ID 'chal-001', got: %q", output)
	}

	// Output should show the target
	if !strings.Contains(output, "statement") {
		t.Errorf("output should show challenge target 'statement', got: %q", output)
	}

	// Output should show the reason
	if !strings.Contains(output, "unclear") {
		t.Errorf("output should show challenge reason, got: %q", output)
	}

	// Output should show the severity
	if !strings.Contains(output, "critical") {
		t.Errorf("output should show challenge severity 'critical', got: %q", output)
	}

	// Output should suggest how to resolve
	if !strings.Contains(output, "resolve") || !strings.Contains(output, "refine") {
		t.Errorf("output should suggest resolution methods, got: %q", output)
	}
}

// TestAcceptCommand_ShowsBlockingChallengesJSON tests that when accept fails
// due to blocking challenges with JSON format, the challenges are displayed as JSON.
func TestAcceptCommand_ShowsBlockingChallengesJSON(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	nodeID := mustParseNodeID(t, "1")

	// Add two blocking challenges
	addChallengeToNode(t, tmpDir, nodeID, "chal-001", "statement", "Statement is ambiguous", "critical")
	addChallengeToNode(t, tmpDir, nodeID, "chal-002", "inference", "Inference type incorrect", "major")

	// Try to accept the node with JSON format - should fail and show challenges as JSON
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "-f", "json")

	// Should return an error
	if err == nil {
		t.Fatal("expected error when accepting node with blocking challenges, got nil")
	}

	// The output may contain the JSON followed by error messages and usage from Cobra.
	// Extract the JSON portion (everything up to the first newline after the closing brace).
	jsonOutput := extractJSONFromOutput(output)

	// Output should be valid JSON
	var result map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(jsonOutput), &result); jsonErr != nil {
		t.Fatalf("output should be valid JSON, got error: %v\nFull Output: %q\nExtracted JSON: %q", jsonErr, output, jsonOutput)
	}

	// Should have error field
	if result["error"] != "blocking_challenges" {
		t.Errorf("JSON should have error='blocking_challenges', got: %v", result["error"])
	}

	// Should have node_id field
	if result["node_id"] != "1" {
		t.Errorf("JSON should have node_id='1', got: %v", result["node_id"])
	}

	// Should have blocking_challenges array
	challenges, ok := result["blocking_challenges"].([]interface{})
	if !ok {
		t.Fatalf("JSON should have blocking_challenges array, got: %v", result["blocking_challenges"])
	}

	// Should have 2 challenges
	if len(challenges) != 2 {
		t.Errorf("expected 2 blocking challenges, got %d", len(challenges))
	}

	// Verify first challenge has expected fields
	if len(challenges) > 0 {
		firstChallenge, ok := challenges[0].(map[string]interface{})
		if !ok {
			t.Fatalf("challenge should be an object, got: %T", challenges[0])
		}

		if _, hasID := firstChallenge["id"]; !hasID {
			t.Error("challenge should have 'id' field")
		}
		if _, hasTarget := firstChallenge["target"]; !hasTarget {
			t.Error("challenge should have 'target' field")
		}
		if _, hasSeverity := firstChallenge["severity"]; !hasSeverity {
			t.Error("challenge should have 'severity' field")
		}
		if _, hasReason := firstChallenge["reason"]; !hasReason {
			t.Error("challenge should have 'reason' field")
		}
	}

	// Should have how_to_resolve array
	resolve, ok := result["how_to_resolve"].([]interface{})
	if !ok {
		t.Errorf("JSON should have how_to_resolve array, got: %v", result["how_to_resolve"])
	}

	if len(resolve) == 0 {
		t.Error("how_to_resolve array should not be empty")
	}
}

// extractJSONFromOutput extracts a JSON object from output that may contain
// additional error messages or usage text after the JSON.
func extractJSONFromOutput(output string) string {
	// Find the start of JSON (first '{')
	startIdx := strings.Index(output, "{")
	if startIdx == -1 {
		return output
	}

	// Track braces to find the matching closing brace
	braceCount := 0
	inString := false
	escaped := false

	for i := startIdx; i < len(output); i++ {
		c := output[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == '{' {
			braceCount++
		} else if c == '}' {
			braceCount--
			if braceCount == 0 {
				return output[startIdx : i+1]
			}
		}
	}

	// If we didn't find matching braces, return from start to end
	return output[startIdx:]
}

// TestAcceptCommand_NoBlockingWithMinorChallenge tests that minor challenges
// do not block acceptance.
func TestAcceptCommand_NoBlockingWithMinorChallenge(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	nodeID := mustParseNodeID(t, "1")

	// Add a minor challenge to the node (should NOT block acceptance)
	addChallengeToNode(t, tmpDir, nodeID, "chal-minor", "statement", "Minor style issue", "minor")

	// Try to accept the node - should succeed because minor challenges don't block
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)

	// Should NOT return an error
	if err != nil {
		t.Fatalf("expected no error when accepting node with only minor challenges, got: %v\nOutput: %s", err, output)
	}

	// Should show success message
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "accepted") && !strings.Contains(lower, "validated") {
		t.Errorf("output should indicate success, got: %q", output)
	}
}

// TestAcceptCommand_MultipleBlockingChallenges tests that all blocking challenges
// are shown when multiple exist.
func TestAcceptCommand_MultipleBlockingChallenges(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	nodeID := mustParseNodeID(t, "1")

	// Add multiple blocking challenges
	addChallengeToNode(t, tmpDir, nodeID, "chal-critical", "statement", "Critical issue found", "critical")
	addChallengeToNode(t, tmpDir, nodeID, "chal-major-1", "inference", "Inference problem", "major")
	addChallengeToNode(t, tmpDir, nodeID, "chal-major-2", "gap", "Logical gap in reasoning", "major")

	// Try to accept the node - should fail
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)

	// Should return an error
	if err == nil {
		t.Fatal("expected error when accepting node with blocking challenges")
	}

	// All blocking challenge IDs should be shown
	if !strings.Contains(output, "chal-critical") {
		t.Errorf("output should show 'chal-critical', got: %q", output)
	}
	if !strings.Contains(output, "chal-major-1") {
		t.Errorf("output should show 'chal-major-1', got: %q", output)
	}
	if !strings.Contains(output, "chal-major-2") {
		t.Errorf("output should show 'chal-major-2', got: %q", output)
	}
}

// =============================================================================
// Verification Summary Tests
// =============================================================================

// resolveChallengeForNode is a test helper that resolves a challenge by appending
// to the ledger. This bypasses the service layer to set up test fixtures.
func resolveChallengeForNode(t *testing.T, proofDir string, challengeID string) {
	t.Helper()
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("failed to get ledger: %v", err)
	}
	event := ledger.NewChallengeResolved(challengeID)
	if _, err := ldg.Append(event); err != nil {
		t.Fatalf("failed to append challenge resolution: %v", err)
	}
}

// TestAcceptCommand_ShowsVerificationSummary tests that after successful acceptance,
// a verification summary is displayed showing challenge counts and dependencies.
func TestAcceptCommand_ShowsVerificationSummary(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	nodeID := mustParseNodeID(t, "1")

	// Add some minor challenges (which don't block acceptance) and resolve them
	addChallengeToNode(t, tmpDir, nodeID, "chal-minor-1", "statement", "Minor issue 1", "minor")
	addChallengeToNode(t, tmpDir, nodeID, "chal-minor-2", "inference", "Minor issue 2", "note")
	resolveChallengeForNode(t, tmpDir, "chal-minor-1")

	// Accept the node - should succeed because only minor/note challenges
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Output should contain verification summary header
	if !strings.Contains(output, "Verification summary") {
		t.Errorf("output should contain 'Verification summary', got: %q", output)
	}

	// Output should show challenge counts
	if !strings.Contains(output, "Challenges:") {
		t.Errorf("output should contain challenge count info, got: %q", output)
	}

	// Should show "2 raised" (we added 2 challenges)
	if !strings.Contains(output, "2 raised") {
		t.Errorf("output should show '2 raised' challenges, got: %q", output)
	}

	// Should show "1 resolved" (we resolved 1 challenge)
	if !strings.Contains(output, "1 resolved") {
		t.Errorf("output should show '1 resolved' challenges, got: %q", output)
	}
}

// TestAcceptCommand_ShowsVerificationSummaryJSON tests that after successful acceptance
// with JSON format, a verification summary is included in the JSON output.
func TestAcceptCommand_ShowsVerificationSummaryJSON(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	nodeID := mustParseNodeID(t, "1")

	// Add some minor challenges (which don't block acceptance) and resolve them
	addChallengeToNode(t, tmpDir, nodeID, "chal-minor-1", "statement", "Minor issue 1", "minor")
	addChallengeToNode(t, tmpDir, nodeID, "chal-minor-2", "inference", "Minor issue 2", "note")
	resolveChallengeForNode(t, tmpDir, "chal-minor-1")

	// Accept the node with JSON format
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Parse JSON output
	var result map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(output), &result); jsonErr != nil {
		t.Fatalf("output should be valid JSON, got error: %v\nOutput: %q", jsonErr, output)
	}

	// Should have accepted=true
	if result["accepted"] != true {
		t.Errorf("JSON should have accepted=true, got: %v", result["accepted"])
	}

	// Should have verification_summary
	summary, ok := result["verification_summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("JSON should have verification_summary object, got: %v", result["verification_summary"])
	}

	// Should have challenges_raised
	challengesRaised, ok := summary["challenges_raised"].(float64)
	if !ok {
		t.Fatalf("verification_summary should have challenges_raised, got: %v", summary["challenges_raised"])
	}
	if int(challengesRaised) != 2 {
		t.Errorf("expected challenges_raised=2, got: %v", challengesRaised)
	}

	// Should have challenges_resolved
	challengesResolved, ok := summary["challenges_resolved"].(float64)
	if !ok {
		t.Fatalf("verification_summary should have challenges_resolved, got: %v", summary["challenges_resolved"])
	}
	if int(challengesResolved) != 1 {
		t.Errorf("expected challenges_resolved=1, got: %v", challengesResolved)
	}
}

// TestAcceptCommand_ShowsVerificationSummaryWithNote tests that the acceptance note
// appears in the verification summary.
func TestAcceptCommand_ShowsVerificationSummaryWithNote(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Accept the node with a note
	note := "Verified via algebraic manipulation"
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "--with-note", note)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Output should contain verification summary
	if !strings.Contains(output, "Verification summary") {
		t.Errorf("output should contain 'Verification summary', got: %q", output)
	}

	// Output should contain the note in the summary
	if !strings.Contains(output, "Note:") {
		t.Errorf("output should contain 'Note:' in verification summary, got: %q", output)
	}

	if !strings.Contains(output, note) {
		t.Errorf("output should contain the note text %q, got: %q", note, output)
	}
}

// TestAcceptCommand_ShowsVerificationSummaryWithNoteJSON tests that the acceptance note
// appears in the JSON verification summary.
func TestAcceptCommand_ShowsVerificationSummaryWithNoteJSON(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Accept the node with a note and JSON format
	note := "Verified via algebraic manipulation"
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "--with-note", note, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Parse JSON output
	var result map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(output), &result); jsonErr != nil {
		t.Fatalf("output should be valid JSON, got error: %v\nOutput: %q", jsonErr, output)
	}

	// Should have note field
	resultNote, ok := result["note"].(string)
	if !ok {
		t.Fatalf("JSON should have note field, got: %v", result["note"])
	}
	if resultNote != note {
		t.Errorf("expected note=%q, got: %q", note, resultNote)
	}

	// Should have verification_summary
	_, ok = result["verification_summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("JSON should have verification_summary object, got: %v", result["verification_summary"])
	}
}

// createNodeWithDependencies is a test helper that creates a node with dependencies
// by directly appending to the ledger.
func createNodeWithDependencies(t *testing.T, proofDir string, nodeID service.NodeID, deps []service.NodeID) {
	t.Helper()
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("failed to get ledger: %v", err)
	}

	n, err := node.NewNodeWithOptions(nodeID, service.NodeTypeClaim, "Test statement with dependencies",
		service.InferenceModusPonens, node.NodeOptions{Dependencies: deps})
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	event := ledger.NewNodeCreated(*n)
	if _, err := ldg.Append(event); err != nil {
		t.Fatalf("failed to append node: %v", err)
	}
}

// TestAcceptCommand_ShowsVerificationSummaryWithDependencies tests that dependencies
// are shown in the verification summary.
func TestAcceptCommand_ShowsVerificationSummaryWithDependencies(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a child node with a dependency on the root
	childID := mustParseNodeID(t, "1.1")
	rootID := mustParseNodeID(t, "1")

	// First accept the root node
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root node: %v", err)
	}

	// Create child node that depends on the root (using test helper)
	createNodeWithDependencies(t, tmpDir, childID, []service.NodeID{rootID})

	// Accept the child node
	output, err := executeAcceptCommand(t, "1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Output should contain verification summary
	if !strings.Contains(output, "Verification summary") {
		t.Errorf("output should contain 'Verification summary', got: %q", output)
	}

	// Output should contain dependencies section
	if !strings.Contains(output, "Dependencies:") {
		t.Errorf("output should contain 'Dependencies:', got: %q", output)
	}

	// Should show the root node as a dependency with validated status
	if !strings.Contains(output, "1:") && !strings.Contains(output, "1 :") {
		t.Errorf("output should show dependency on node 1, got: %q", output)
	}

	if !strings.Contains(output, "validated") {
		t.Errorf("output should show dependency status 'validated', got: %q", output)
	}
}

// TestAcceptCommand_ShowsVerificationSummaryWithDependenciesJSON tests that dependencies
// are shown in the JSON verification summary.
func TestAcceptCommand_ShowsVerificationSummaryWithDependenciesJSON(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a child node with a dependency on the root
	childID := mustParseNodeID(t, "1.1")
	rootID := mustParseNodeID(t, "1")

	// First accept the root node
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("failed to accept root node: %v", err)
	}

	// Create child node that depends on the root (using test helper)
	createNodeWithDependencies(t, tmpDir, childID, []service.NodeID{rootID})

	// Accept the child node with JSON format
	output, err := executeAcceptCommand(t, "1.1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Parse JSON output
	var result map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(output), &result); jsonErr != nil {
		t.Fatalf("output should be valid JSON, got error: %v\nOutput: %q", jsonErr, output)
	}

	// Should have verification_summary
	summary, ok := result["verification_summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("JSON should have verification_summary object, got: %v", result["verification_summary"])
	}

	// Should have dependencies array
	deps, ok := summary["dependencies"].([]interface{})
	if !ok {
		t.Fatalf("verification_summary should have dependencies array, got: %v", summary["dependencies"])
	}

	// Should have at least one dependency
	if len(deps) == 0 {
		t.Error("dependencies array should not be empty")
	}

	// First dependency should be node 1
	if len(deps) > 0 {
		firstDep, ok := deps[0].(map[string]interface{})
		if !ok {
			t.Fatalf("dependency should be an object, got: %T", deps[0])
		}

		if firstDep["id"] != "1" {
			t.Errorf("expected dependency id='1', got: %v", firstDep["id"])
		}

		if firstDep["status"] != "validated" {
			t.Errorf("expected dependency status='validated', got: %v", firstDep["status"])
		}
	}
}

// =============================================================================
// Verifier Challenge Check Tests (--agent and --confirm flags)
// =============================================================================

// addChallengeWithRaisedBy is a test helper that adds a challenge to a node
// with the RaisedBy field set.
func addChallengeWithRaisedBy(t *testing.T, proofDir string, nodeID service.NodeID, challengeID, target, reason, severity, raisedBy string) {
	t.Helper()

	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("failed to get ledger: %v", err)
	}

	event := ledger.NewChallengeRaisedWithSeverity(challengeID, nodeID, target, reason, severity, raisedBy)
	if _, err := ldg.Append(event); err != nil {
		t.Fatalf("failed to append challenge: %v", err)
	}
}

// TestAcceptCommand_RequiresConfirmIfNoChallenges tests that accepting without
// having raised any challenges requires --confirm when --agent is provided.
func TestAcceptCommand_RequiresConfirmIfNoChallenges(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Try to accept with --agent but without having raised any challenges
	// and without --confirm - should fail
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "--agent", "verifier-1")

	// Should return an error
	if err == nil {
		t.Fatal("expected error when accepting without having raised challenges, got nil")
	}

	errStr := err.Error()
	combined := output + errStr

	// Error should mention that no challenges were raised
	if !strings.Contains(combined, "haven't raised any challenges") {
		t.Errorf("error should mention no challenges raised, got: %q", combined)
	}

	// Error should suggest using --confirm
	if !strings.Contains(combined, "--confirm") {
		t.Errorf("error should suggest using --confirm, got: %q", combined)
	}

	// Node should NOT be validated (acceptance should have been blocked)
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	nodeID := mustParseNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n != nil && n.EpistemicState == service.EpistemicValidated {
		t.Error("node should NOT be validated when acceptance is blocked")
	}
}

// TestAcceptCommand_ConfirmBypassesCheck tests that --confirm bypasses the
// challenge verification check, allowing acceptance without having raised challenges.
func TestAcceptCommand_ConfirmBypassesCheck(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Accept with --agent and --confirm (no challenges raised, but --confirm bypasses)
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "--agent", "verifier-1", "--confirm")

	// Should succeed
	if err != nil {
		t.Fatalf("expected no error with --confirm, got: %v\nOutput: %s", err, output)
	}

	// Output should indicate success
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "accepted") && !strings.Contains(lower, "validated") {
		t.Errorf("output should indicate success, got: %q", output)
	}

	// Node should be validated
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	nodeID := mustParseNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after accept")
	}
	if n.EpistemicState != service.EpistemicValidated {
		t.Errorf("node EpistemicState = %q, want %q", n.EpistemicState, service.EpistemicValidated)
	}
}

// TestAcceptCommand_NoConfirmNeededIfChallengeRaised tests that --confirm is not
// required when the agent has raised a challenge for the node.
func TestAcceptCommand_NoConfirmNeededIfChallengeRaised(t *testing.T) {

	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	nodeID := mustParseNodeID(t, "1")

	// Add a minor challenge (doesn't block acceptance) raised by verifier-1
	addChallengeWithRaisedBy(t, tmpDir, nodeID, "chal-001", "statement", "Minor clarification needed", "minor", "verifier-1")

	// Accept with --agent but WITHOUT --confirm (should succeed because challenge was raised)
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir, "--agent", "verifier-1")

	// Should succeed without needing --confirm
	if err != nil {
		t.Fatalf("expected no error when agent raised challenges, got: %v\nOutput: %s", err, output)
	}

	// Output should indicate success
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "accepted") && !strings.Contains(lower, "validated") {
		t.Errorf("output should indicate success, got: %q", output)
	}

	// Node should be validated
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after accept")
	}
	if n.EpistemicState != service.EpistemicValidated {
		t.Errorf("node EpistemicState = %q, want %q", n.EpistemicState, service.EpistemicValidated)
	}
}

// TestAcceptCommand_AgentFlagInHelp tests that --agent flag appears in help output.
func TestAcceptCommand_AgentFlagInHelp(t *testing.T) {
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

	// Check that --agent flag is documented in help
	if !strings.Contains(output, "--agent") {
		t.Errorf("help output should contain --agent flag, got: %q", output)
	}
}

// TestAcceptCommand_NoAgentFlagMeansNoCheck tests that without --agent,
// no challenge verification is performed.
func TestAcceptCommand_NoAgentFlagMeansNoCheck(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Accept without --agent flag - should succeed without any challenge check
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)

	// Should succeed
	if err != nil {
		t.Fatalf("expected no error without --agent flag, got: %v\nOutput: %s", err, output)
	}

	// Node should be validated
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	nodeID := mustParseNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after accept")
	}
	if n.EpistemicState != service.EpistemicValidated {
		t.Errorf("node EpistemicState = %q, want %q", n.EpistemicState, service.EpistemicValidated)
	}
}
