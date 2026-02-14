//go:build integration

// Package main contains tests for the af veto command.
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

func setupVetoTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-veto-test-*")
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

func setupVetoTestWithValidatedNode(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupVetoTest(t)

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

func setupVetoTestWithAdmittedNode(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupVetoTest(t)

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

func executeVetoCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newVetoCmd()
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

func TestVetoCmd_PendingNode(t *testing.T) {
	tmpDir, cleanup := setupVetoTest(t)
	defer cleanup()

	output, err := executeVetoCommand(t, "1", "-d", tmpDir, "-y", "--reason", "Contradicts known theorem")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "vetoed") {
		t.Errorf("Output should contain 'vetoed', got: %s", output)
	}

	// Verify node state changed to refuted
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	nodeID, _ := service.ParseNodeID("1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("Node 1 should exist")
	}
	if n.EpistemicState != service.EpistemicRefuted {
		t.Errorf("Node 1 should be in refuted state, got: %s", n.EpistemicState)
	}
}

func TestVetoCmd_ValidatedNode(t *testing.T) {
	tmpDir, cleanup := setupVetoTestWithValidatedNode(t)
	defer cleanup()

	output, err := executeVetoCommand(t, "1", "-d", tmpDir, "-y", "--reason", "Expert review found fatal flaw")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "vetoed") {
		t.Errorf("Output should contain 'vetoed', got: %s", output)
	}

	// Verify validated node was force-refuted
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	nodeID, _ := service.ParseNodeID("1")
	n := st.GetNode(nodeID)
	if n.EpistemicState != service.EpistemicRefuted {
		t.Errorf("Validated node should be refuted after veto, got: %s", n.EpistemicState)
	}
}

func TestVetoCmd_AdmittedNode(t *testing.T) {
	tmpDir, cleanup := setupVetoTestWithAdmittedNode(t)
	defer cleanup()

	output, err := executeVetoCommand(t, "1", "-d", tmpDir, "-y", "--reason", "Admitted claim proven false")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "vetoed") {
		t.Errorf("Output should contain 'vetoed', got: %s", output)
	}

	// Verify admitted node was force-refuted
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	nodeID, _ := service.ParseNodeID("1")
	n := st.GetNode(nodeID)
	if n.EpistemicState != service.EpistemicRefuted {
		t.Errorf("Admitted node should be refuted after veto, got: %s", n.EpistemicState)
	}
}

func TestVetoCmd_ReasonRequired(t *testing.T) {
	tmpDir, cleanup := setupVetoTest(t)
	defer cleanup()

	// No --reason flag
	_, err := executeVetoCommand(t, "1", "-d", tmpDir, "-y")
	if err == nil {
		t.Fatal("Expected error when reason is missing, got success")
	}

	if !strings.Contains(err.Error(), "reason") {
		t.Errorf("Error should mention reason is required, got: %v", err)
	}
}

func TestVetoCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupVetoTest(t)
	defer cleanup()

	reason := "Boundary case disproves claim"
	output, err := executeVetoCommand(t, "1", "-d", tmpDir, "-y", "-f", "json",
		"--reason", reason, "--agent", "expert-reviewer")
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
	if result["status"] != "refuted" {
		t.Errorf("Expected status 'refuted', got: %v", result["status"])
	}
	if result["vetoed"] != true {
		t.Errorf("Expected vetoed=true, got: %v", result["vetoed"])
	}
	if result["reason"] != reason {
		t.Errorf("Expected reason %q, got: %v", reason, result["reason"])
	}
	if result["vetoed_by"] != "expert-reviewer" {
		t.Errorf("Expected vetoed_by 'expert-reviewer', got: %v", result["vetoed_by"])
	}
}

func TestVetoCmd_AlreadyRefuted(t *testing.T) {
	tmpDir, cleanup := setupVetoTest(t)
	defer cleanup()

	// Refute node first
	svc, _ := service.NewProofService(tmpDir)
	nodeID, _ := service.ParseNodeID("1")
	if err := svc.RefuteNode(nodeID); err != nil {
		t.Fatalf("Failed to refute node: %v", err)
	}

	// Veto should fail — already refuted
	_, err := executeVetoCommand(t, "1", "-d", tmpDir, "-y", "--reason", "Already wrong")
	if err == nil {
		t.Fatal("Expected error for already refuted node, got success")
	}
}

func TestVetoCmd_NonExistent(t *testing.T) {
	tmpDir, cleanup := setupVetoTest(t)
	defer cleanup()

	_, err := executeVetoCommand(t, "1.99", "-d", tmpDir, "-y", "--reason", "Does not matter")
	if err == nil {
		t.Fatal("Expected error for non-existent node, got success")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention node not found, got: %v", err)
	}
}

func TestVetoCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupVetoTest(t)
	defer cleanup()

	_, err := executeVetoCommand(t, "invalid-id", "-d", tmpDir, "-y", "--reason", "Does not matter")
	if err == nil {
		t.Fatal("Expected error for invalid node ID, got success")
	}

	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Error should mention invalid node ID, got: %v", err)
	}
}
