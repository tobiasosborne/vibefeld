//go:build integration

// Package main contains tests for the af request-refinement command.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupRequestRefinementTest creates a temporary directory with an initialized proof.
func setupRequestRefinementTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-request-refinement-test-*")
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

// setupRequestRefinementTestWithValidatedNode creates a test environment with
// an initialized proof and a validated node at ID "1".
func setupRequestRefinementTestWithValidatedNode(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupRequestRefinementTest(t)

	// Validate node 1
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	nodeID, _ := service.ParseNodeID("1")
	if err := svc.AcceptNode(nodeID); err != nil {
		cleanup()
		t.Fatalf("Failed to validate node 1: %v", err)
	}

	return tmpDir, cleanup
}

// executeRequestRefinementCommand creates and executes a request-refinement command.
func executeRequestRefinementCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newRequestRefinementCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// Test Cases
// =============================================================================

// TestRequestRefinementCmd_Success tests requesting refinement on a validated node.
func TestRequestRefinementCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupRequestRefinementTestWithValidatedNode(t)
	defer cleanup()

	// Execute request-refinement command
	output, err := executeRequestRefinementCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	// Verify output contains expected information
	if !strings.Contains(output, "Refinement requested") {
		t.Errorf("Output should contain 'Refinement requested', got: %s", output)
	}
	if !strings.Contains(output, "needs_refinement") {
		t.Errorf("Output should contain 'needs_refinement', got: %s", output)
	}

	// Verify node state changed
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	nodeID, _ := service.ParseNodeID("1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("Node 1 should exist")
	}
	if n.EpistemicState != schema.EpistemicNeedsRefinement {
		t.Errorf("Node 1 should be in needs_refinement state, got: %s", n.EpistemicState)
	}
}

// TestRequestRefinementCmd_WithReason tests requesting refinement with a reason.
func TestRequestRefinementCmd_WithReason(t *testing.T) {
	tmpDir, cleanup := setupRequestRefinementTestWithValidatedNode(t)
	defer cleanup()

	reason := "Need explicit algebra steps"
	output, err := executeRequestRefinementCommand(t, "1", "-d", tmpDir, "--reason", reason)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	// Verify output contains the reason
	if !strings.Contains(output, reason) {
		t.Errorf("Output should contain reason %q, got: %s", reason, output)
	}
}

// TestRequestRefinementCmd_JSONFormat tests JSON output format.
func TestRequestRefinementCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupRequestRefinementTestWithValidatedNode(t)
	defer cleanup()

	reason := "Need more detail"
	output, err := executeRequestRefinementCommand(t, "1", "-d", tmpDir, "-f", "json", "--reason", reason)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Verify JSON fields
	if result["node_id"] != "1" {
		t.Errorf("Expected node_id '1', got: %v", result["node_id"])
	}
	if result["previous_state"] != "validated" {
		t.Errorf("Expected previous_state 'validated', got: %v", result["previous_state"])
	}
	if result["current_state"] != "needs_refinement" {
		t.Errorf("Expected current_state 'needs_refinement', got: %v", result["current_state"])
	}
	if result["reason"] != reason {
		t.Errorf("Expected reason %q, got: %v", reason, result["reason"])
	}
}

// TestRequestRefinementCmd_NonExistent tests error when requesting refinement on non-existent node.
func TestRequestRefinementCmd_NonExistent(t *testing.T) {
	tmpDir, cleanup := setupRequestRefinementTest(t)
	defer cleanup()

	// Use a valid node ID format that doesn't exist (1.99 is child of root that doesn't exist)
	output, err := executeRequestRefinementCommand(t, "1.99", "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error for non-existent node, got success")
	}

	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "ErrNodeNotFound") {
		t.Errorf("Error should mention node not found, got: %v", err)
	}
	_ = output // Avoid unused variable warning
}

// TestRequestRefinementCmd_NotValidated tests error when requesting refinement on pending node.
func TestRequestRefinementCmd_NotValidated(t *testing.T) {
	tmpDir, cleanup := setupRequestRefinementTest(t)
	defer cleanup()

	// Node 1 is pending (not validated yet)
	output, err := executeRequestRefinementCommand(t, "1", "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error for pending node, got success")
	}

	if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "validated") {
		t.Errorf("Error should mention invalid state or that node must be validated, got: %v", err)
	}
	_ = output
}

// TestRequestRefinementCmd_InvalidNodeID tests error for invalid node ID format.
func TestRequestRefinementCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupRequestRefinementTest(t)
	defer cleanup()

	output, err := executeRequestRefinementCommand(t, "invalid-id", "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error for invalid node ID, got success")
	}

	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Error should mention invalid node ID, got: %v", err)
	}
	_ = output
}

// TestRequestRefinementCmd_NoArgs tests error when no node ID is provided.
func TestRequestRefinementCmd_NoArgs(t *testing.T) {
	tmpDir, cleanup := setupRequestRefinementTest(t)
	defer cleanup()

	output, err := executeRequestRefinementCommand(t, "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error for missing node ID, got success")
	}

	// Cobra should report "accepts 1 arg(s), received 0"
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Errorf("Error should mention required argument, got: %v", err)
	}
	_ = output
}

// TestRequestRefinementCmd_WithAgent tests requesting refinement with agent ID.
func TestRequestRefinementCmd_WithAgent(t *testing.T) {
	tmpDir, cleanup := setupRequestRefinementTestWithValidatedNode(t)
	defer cleanup()

	output, err := executeRequestRefinementCommand(t, "1", "-d", tmpDir, "--agent", "verifier-001")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	// Command should succeed with agent specified
	if !strings.Contains(output, "Refinement requested") {
		t.Errorf("Output should contain 'Refinement requested', got: %s", output)
	}
}
