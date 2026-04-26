//go:build integration

// Package main contains tests for the af unadmit command.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

func setupUnadmitTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-unadmit-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	if err := service.InitProofDir(tmpDir); err != nil {
		cleanup()
		t.Fatal(err)
	}

	if err := service.Init(tmpDir, "Test conjecture", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

func setupUnadmitTestWithAdmittedNode(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupUnadmitTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	nodeID, _ := service.ParseNodeID("1")
	if err := svc.AdmitNode(nodeID); err != nil {
		cleanup()
		t.Fatalf("Failed to admit node 1: %v", err)
	}

	return tmpDir, cleanup
}

func executeUnadmitCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newUnadmitCmd()
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

func TestUnadmitCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupUnadmitTestWithAdmittedNode(t)
	defer cleanup()

	output, err := executeUnadmitCommand(t, "1", "-d", tmpDir, "-y")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Admission revoked") {
		t.Errorf("Output should contain 'Admission revoked', got: %s", output)
	}
	if !strings.Contains(output, "pending") {
		t.Errorf("Output should contain 'pending', got: %s", output)
	}

	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	nodeID, _ := service.ParseNodeID("1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("Node 1 should exist")
	}
	if n.EpistemicState != service.EpistemicPending {
		t.Errorf("Node 1 should be in pending state, got: %s", n.EpistemicState)
	}
}

func TestUnadmitCmd_WithReason(t *testing.T) {
	tmpDir, cleanup := setupUnadmitTestWithAdmittedNode(t)
	defer cleanup()

	reason := "Now rigorously verified by 1.{1..3}"
	output, err := executeUnadmitCommand(t, "1", "-d", tmpDir, "-y", "--reason", reason)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, reason) {
		t.Errorf("Output should contain reason %q, got: %s", reason, output)
	}
}

func TestUnadmitCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupUnadmitTestWithAdmittedNode(t)
	defer cleanup()

	reason := "Replaced by rigorous proof"
	output, err := executeUnadmitCommand(t, "1", "-d", tmpDir, "-y", "-f", "json", "--reason", reason)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	if result["node_id"] != "1" {
		t.Errorf("Expected node_id '1', got: %v", result["node_id"])
	}
	if result["previous_state"] != "admitted" {
		t.Errorf("Expected previous_state 'admitted', got: %v", result["previous_state"])
	}
	if result["current_state"] != "pending" {
		t.Errorf("Expected current_state 'pending', got: %v", result["current_state"])
	}
	if result["reason"] != reason {
		t.Errorf("Expected reason %q, got: %v", reason, result["reason"])
	}
}

func TestUnadmitCmd_NotAdmitted(t *testing.T) {
	tmpDir, cleanup := setupUnadmitTest(t)
	defer cleanup()

	// Node 1 is pending — cannot unadmit
	_, err := executeUnadmitCommand(t, "1", "-d", tmpDir, "-y")
	if err == nil {
		t.Fatal("Expected error for pending node, got success")
	}

	if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "admitted") {
		t.Errorf("Error should mention invalid state, got: %v", err)
	}
}

func TestUnadmitCmd_NonExistent(t *testing.T) {
	tmpDir, cleanup := setupUnadmitTest(t)
	defer cleanup()

	_, err := executeUnadmitCommand(t, "1.99", "-d", tmpDir, "-y")
	if err == nil {
		t.Fatal("Expected error for non-existent node, got success")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention node not found, got: %v", err)
	}
}

func TestUnadmitCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupUnadmitTest(t)
	defer cleanup()

	_, err := executeUnadmitCommand(t, "invalid-id", "-d", tmpDir, "-y")
	if err == nil {
		t.Fatal("Expected error for invalid node ID, got success")
	}

	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Error should mention invalid node ID, got: %v", err)
	}
}

func TestUnadmitCmd_RoundTripToAccept(t *testing.T) {
	// The whole point: admit (escape hatch) → unadmit → accept (rigorous verdict).
	tmpDir, cleanup := setupUnadmitTestWithAdmittedNode(t)
	defer cleanup()

	// Unadmit
	if _, err := executeUnadmitCommand(t, "1", "-d", tmpDir, "-y"); err != nil {
		t.Fatalf("unadmit failed: %v", err)
	}

	// Now accept should work — node is back to pending with no children
	svc, _ := service.NewProofService(tmpDir)
	nodeID, _ := service.ParseNodeID("1")
	if err := svc.AcceptNode(nodeID); err != nil {
		t.Fatalf("expected accept to succeed after unadmit, got: %v", err)
	}

	st, _ := svc.LoadState()
	if got := st.GetNode(nodeID).EpistemicState; got != service.EpistemicValidated {
		t.Errorf("after unadmit+accept, expected validated, got %s", got)
	}
}
