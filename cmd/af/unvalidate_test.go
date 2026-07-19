//go:build integration

// Package main contains tests for the af unvalidate command.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

func setupUnvalidateTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-unvalidate-test-*")
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

func setupUnvalidateTestWithValidatedNode(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupUnvalidateTest(t)

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

func executeUnvalidateCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newUnvalidateCmd()
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

func TestUnvalidateCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupUnvalidateTestWithValidatedNode(t)
	defer cleanup()

	output, err := executeUnvalidateCommand(t, "1", "-d", tmpDir, "-y")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Validation revoked") {
		t.Errorf("Output should contain 'Validation revoked', got: %s", output)
	}
	if !strings.Contains(output, "pending") {
		t.Errorf("Output should contain 'pending', got: %s", output)
	}

	// Verify node state changed
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

func TestUnvalidateCmd_WithReason(t *testing.T) {
	tmpDir, cleanup := setupUnvalidateTestWithValidatedNode(t)
	defer cleanup()

	reason := "Formula error in step 3"
	output, err := executeUnvalidateCommand(t, "1", "-d", tmpDir, "-y", "--reason", reason)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, reason) {
		t.Errorf("Output should contain reason %q, got: %s", reason, output)
	}
}

func TestUnvalidateCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupUnvalidateTestWithValidatedNode(t)
	defer cleanup()

	reason := "Error discovered"
	output, err := executeUnvalidateCommand(t, "1", "-d", tmpDir, "-y", "-f", "json", "--reason", reason)
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
	if result["previous_state"] != "validated" {
		t.Errorf("Expected previous_state 'validated', got: %v", result["previous_state"])
	}
	if result["current_state"] != "pending" {
		t.Errorf("Expected current_state 'pending', got: %v", result["current_state"])
	}
	if result["reason"] != reason {
		t.Errorf("Expected reason %q, got: %v", reason, result["reason"])
	}
}

func TestUnvalidateCmd_NotValidated(t *testing.T) {
	tmpDir, cleanup := setupUnvalidateTest(t)
	defer cleanup()

	// Node 1 is pending — cannot unvalidate
	_, err := executeUnvalidateCommand(t, "1", "-d", tmpDir, "-y")
	if err == nil {
		t.Fatal("Expected error for pending node, got success")
	}

	if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "validated") {
		t.Errorf("Error should mention invalid state, got: %v", err)
	}
}

func TestUnvalidateCmd_NonExistent(t *testing.T) {
	tmpDir, cleanup := setupUnvalidateTest(t)
	defer cleanup()

	_, err := executeUnvalidateCommand(t, "1.99", "-d", tmpDir, "-y")
	if err == nil {
		t.Fatal("Expected error for non-existent node, got success")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention node not found, got: %v", err)
	}
}

func TestUnvalidateCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupUnvalidateTest(t)
	defer cleanup()

	_, err := executeUnvalidateCommand(t, "invalid-id", "-d", tmpDir, "-y")
	if err == nil {
		t.Fatal("Expected error for invalid node ID, got success")
	}

	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Error should mention invalid node ID, got: %v", err)
	}
}

// TestUnvalidateCmd_Batch_Success is the --batch (rk PRD C3, item V3)
// counterpart to TestUnvalidateCmd_Success above. Dead under the
// `integration` tag (this suite is pre-existing broken, not touched here —
// see handoff.md), live if that suite is ever fixed. See
// cmd/af/unvalidate_batch_test.go for the default-tag equivalent that
// actually runs under `go test ./...`.
func TestUnvalidateCmd_Batch_Success(t *testing.T) {
	tmpDir, cleanup := setupUnvalidateTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	nodeID, _ := service.ParseNodeID("1")
	if err := svc.AcceptNodeWithVerifier(nodeID, "", "verifier-1", "batch-integration"); err != nil {
		t.Fatalf("Failed to validate node 1 under a batch: %v", err)
	}

	output, err := executeUnvalidateCommand(t, "--batch", "batch-integration", "-d", tmpDir, "-y")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "1 node(s) unvalidated") {
		t.Errorf("Output should report 1 node unvalidated, got: %s", output)
	}

	st, _ := svc.LoadState()
	n := st.GetNode(nodeID)
	if n.EpistemicState != service.EpistemicPending {
		t.Errorf("Node 1 should be back to pending, got: %s", n.EpistemicState)
	}
}

// TestUnvalidateCmd_Batch_NotFound mirrors TestUnvalidateCmd_NonExistent's
// place in this suite for the bulk form: a batch id matching nothing is a
// clean no-op, not an error condition that crashes the command.
func TestUnvalidateCmd_Batch_NotFound(t *testing.T) {
	tmpDir, cleanup := setupUnvalidateTest(t)
	defer cleanup()

	_, err := executeUnvalidateCommand(t, "--batch", "no-such-batch", "-d", tmpDir, "-y")
	if !errors.Is(err, service.ErrUnvalidateBatchNotFound) {
		t.Errorf("Expected ErrUnvalidateBatchNotFound, got: %v", err)
	}
	if service.ExitCode(err) != 7 {
		t.Errorf("ExitCode(err) = %d, want 7", service.ExitCode(err))
	}
}
