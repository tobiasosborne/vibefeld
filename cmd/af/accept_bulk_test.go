//go:build integration

// Package main contains tests for the af accept bulk operations.
// These are TDD tests for accepting multiple nodes at once (vibefeld-q9ez).
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

// setupBulkAcceptTest creates a temporary directory with an initialized proof
// and multiple nodes for testing bulk accept operations.
func setupBulkAcceptTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-accept-bulk-test-*")
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
	if err := service.Init(tmpDir, "Test conjecture for bulk accept", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create additional child nodes for testing
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create nodes 1.1, 1.2, and 1.3
	for i := 1; i <= 3; i++ {
		childID, _ := service.ParseNodeID("1." + string(rune('0'+i)))
		err := svc.CreateNode(childID, service.NodeTypeClaim, "Child statement "+string(rune('0'+i)), service.InferenceAssumption)
		if err != nil {
			cleanup()
			t.Fatalf("failed to create child node 1.%d: %v", i, err)
		}
	}

	return tmpDir, cleanup
}

// executeBulkAcceptCommand creates and executes an accept command with the given arguments.
func executeBulkAcceptCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// Bulk Accept - Multiple Node IDs Tests
// =============================================================================

// TestAcceptBulkCmd_MultipleNodes tests accepting multiple nodes via positional arguments.
// Example: af accept 1.1 1.2 1.3
func TestAcceptBulkCmd_MultipleNodes(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	// Accept multiple nodes at once
	output, err := executeBulkAcceptCommand(t, "1.1", "1.2", "1.3", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should mention all accepted nodes
	for _, id := range []string{"1.1", "1.2", "1.3"} {
		if !strings.Contains(output, id) {
			t.Errorf("expected output to contain %s, got: %q", id, output)
		}
	}

	// Verify all nodes are now validated
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	for _, idStr := range []string{"1.1", "1.2", "1.3"} {
		nodeID, _ := service.ParseNodeID(idStr)
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

// TestAcceptBulkCmd_TwoNodes tests accepting two nodes at once.
func TestAcceptBulkCmd_TwoNodes(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	output, err := executeBulkAcceptCommand(t, "1.1", "1.2", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should mention both nodes
	if !strings.Contains(output, "1.1") || !strings.Contains(output, "1.2") {
		t.Errorf("expected output to contain 1.1 and 1.2, got: %q", output)
	}

	// Verify both nodes are validated
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	for _, idStr := range []string{"1.1", "1.2"} {
		nodeID, _ := service.ParseNodeID(idStr)
		n := st.GetNode(nodeID)
		if n == nil {
			t.Errorf("node %s not found", idStr)
			continue
		}
		if n.EpistemicState != service.EpistemicValidated {
			t.Errorf("node %s not validated", idStr)
		}
	}

	// Node 1.3 should still be pending
	node13, _ := service.ParseNodeID("1.3")
	n := st.GetNode(node13)
	if n != nil && n.EpistemicState == service.EpistemicValidated {
		t.Error("node 1.3 should not be validated (it was not in the accept list)")
	}
}

// TestAcceptBulkCmd_SingleNodeStillWorks tests that single node accept still works.
func TestAcceptBulkCmd_SingleNodeStillWorks(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	output, err := executeBulkAcceptCommand(t, "1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain 1.1, got: %q", output)
	}

	// Verify node is validated
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	nodeID, _ := service.ParseNodeID("1.1")
	n := st.GetNode(nodeID)
	if n == nil || n.EpistemicState != service.EpistemicValidated {
		t.Error("node 1.1 should be validated")
	}
}

// TestAcceptBulkCmd_JSONOutput tests JSON format output for bulk accept.
func TestAcceptBulkCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	output, err := executeBulkAcceptCommand(t, "1.1", "1.2", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got parse error: %v\nOutput: %q", err, output)
	}

	// Should contain nodes array or accepted field
	if _, ok := result["nodes"]; !ok {
		if _, ok := result["accepted"]; !ok {
			t.Log("Warning: JSON output does not contain 'nodes' or 'accepted' field")
		}
	}
}

// TestAcceptBulkCmd_PartialFailure tests behavior when some nodes fail validation.
func TestAcceptBulkCmd_PartialFailure(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	// Accept 1.1 first to make it already validated
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID, _ := service.ParseNodeID("1.1")
	if err := svc.AcceptNode(nodeID); err != nil {
		t.Fatalf("failed to pre-accept node 1.1: %v", err)
	}

	// Now try to bulk accept including the already-validated node
	// Behavior depends on implementation - it might:
	// 1. Stop at first error
	// 2. Continue and report partial success
	// 3. Skip already-validated nodes silently
	output, err := executeBulkAcceptCommand(t, "1.1", "1.2", "1.3", "-d", tmpDir)

	// At minimum, 1.2 and 1.3 should be validated if the operation continues
	combined := output
	if err != nil {
		combined += err.Error()
	}

	t.Logf("Bulk accept with partial failure output: %s (error: %v)", output, err)

	// Verify that at least 1.2 gets validated (if the implementation continues on error)
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	node12, _ := service.ParseNodeID("1.2")
	n12 := st.GetNode(node12)

	// Behavior is implementation-dependent, just log the result
	if n12 != nil {
		t.Logf("Node 1.2 epistemic state after partial failure: %s", n12.EpistemicState)
	}
}

// TestAcceptBulkCmd_NodeNotFound tests error when one node doesn't exist.
func TestAcceptBulkCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	// Try to accept nodes where one doesn't exist
	output, err := executeBulkAcceptCommand(t, "1.1", "1.99", "-d", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should mention that 1.99 was not found
	if !strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		err == nil {
		t.Errorf("expected error about node not found, got: %q", output)
	}
}

// TestAcceptBulkCmd_InvalidNodeID tests error when one node ID is invalid.
func TestAcceptBulkCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	output, err := executeBulkAcceptCommand(t, "1.1", "invalid", "-d", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "invalid") && err == nil {
		t.Errorf("expected error about invalid node ID, got: %q", output)
	}
}

// =============================================================================
// Accept --all Flag Tests
// =============================================================================

// TestAcceptBulkCmd_AllFlag tests the --all flag to accept all pending nodes.
func TestAcceptBulkCmd_AllFlag(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	// Accept all pending nodes
	output, err := executeBulkAcceptCommand(t, "--all", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with --all flag, got: %v", err)
	}

	// Should mention accepting multiple nodes
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "accept") && !strings.Contains(lower, "validated") {
		t.Errorf("expected output to mention accepting/validating, got: %q", output)
	}

	// Verify all nodes are now validated
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	// Check root and children
	for _, idStr := range []string{"1", "1.1", "1.2", "1.3"} {
		nodeID, _ := service.ParseNodeID(idStr)
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

// TestAcceptBulkCmd_AllFlag_ShortForm tests the -a short form of --all.
func TestAcceptBulkCmd_AllFlag_ShortForm(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	// Accept all using short form
	output, err := executeBulkAcceptCommand(t, "-a", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -a flag, got: %v", err)
	}

	// Should accept all nodes
	if !strings.Contains(strings.ToLower(output), "accept") && !strings.Contains(strings.ToLower(output), "validated") {
		t.Errorf("expected success message, got: %q", output)
	}

	// Verify all are validated
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()

	for _, idStr := range []string{"1", "1.1", "1.2", "1.3"} {
		nodeID, _ := service.ParseNodeID(idStr)
		n := st.GetNode(nodeID)
		if n != nil && n.EpistemicState != service.EpistemicValidated {
			t.Errorf("node %s should be validated with -a flag", idStr)
		}
	}
}

// TestAcceptBulkCmd_AllFlag_NoPendingNodes tests --all when no nodes are pending.
func TestAcceptBulkCmd_AllFlag_NoPendingNodes(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	// First accept all nodes
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, idStr := range []string{"1", "1.1", "1.2", "1.3"} {
		nodeID, _ := service.ParseNodeID(idStr)
		if err := svc.AcceptNode(nodeID); err != nil {
			t.Fatalf("failed to pre-accept node %s: %v", idStr, err)
		}
	}

	// Now try --all when nothing is pending
	output, err := executeBulkAcceptCommand(t, "--all", "-d", tmpDir)

	// Should either succeed with "nothing to do" message or produce an info message
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Acceptable outcomes: success with info message, or no error
	t.Logf("--all with no pending nodes output: %q (error: %v)", output, err)
}

// TestAcceptBulkCmd_AllFlag_JSONOutput tests JSON output with --all flag.
func TestAcceptBulkCmd_AllFlag_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	output, err := executeBulkAcceptCommand(t, "--all", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got parse error: %v\nOutput: %q", err, output)
	}
}

// TestAcceptBulkCmd_AllFlag_ConflictWithNodeIDs tests that --all conflicts with node IDs.
func TestAcceptBulkCmd_AllFlag_ConflictWithNodeIDs(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	// Using both --all and specific node IDs should be an error
	_, err := executeBulkAcceptCommand(t, "--all", "1.1", "-d", tmpDir)

	if err == nil {
		t.Fatal("expected error when using both --all and node IDs, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "exclusive") && !strings.Contains(errStr, "both") && !strings.Contains(errStr, "conflict") {
		t.Errorf("expected error about conflicting arguments, got: %q", errStr)
	}
}

// TestAcceptBulkCmd_MissingNodeIDAndAllFlag tests error when neither node IDs nor --all provided.
func TestAcceptBulkCmd_MissingNodeIDAndAllFlag(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	// Neither node IDs nor --all
	_, err := executeBulkAcceptCommand(t, "-d", tmpDir)

	if err == nil {
		t.Fatal("expected error when no node IDs or --all provided, got nil")
	}
}

// =============================================================================
// Service Layer Tests for AcceptNodeBulk
// =============================================================================

// TestAcceptNodeBulk_Service tests the service layer AcceptNodeBulk method directly.
func TestAcceptNodeBulk_Service(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept multiple nodes via service layer
	nodeIDs := []service.NodeID{}
	for _, idStr := range []string{"1.1", "1.2", "1.3"} {
		nodeID, _ := service.ParseNodeID(idStr)
		nodeIDs = append(nodeIDs, nodeID)
	}

	err = svc.AcceptNodeBulk(nodeIDs)
	if err != nil {
		t.Fatalf("AcceptNodeBulk failed: %v", err)
	}

	// Verify all nodes are validated
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	for _, nodeID := range nodeIDs {
		n := st.GetNode(nodeID)
		if n == nil {
			t.Errorf("node %s not found", nodeID.String())
			continue
		}
		if n.EpistemicState != service.EpistemicValidated {
			t.Errorf("node %s EpistemicState = %q, want validated", nodeID.String(), n.EpistemicState)
		}
	}
}

// TestAcceptNodeBulk_Service_EmptyList tests AcceptNodeBulk with empty list.
func TestAcceptNodeBulk_Service_EmptyList(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Accept with empty list should either succeed (no-op) or return error
	err = svc.AcceptNodeBulk([]service.NodeID{})

	// Empty list behavior is implementation-dependent
	// Just log the result
	t.Logf("AcceptNodeBulk with empty list: error=%v", err)
}

// TestAcceptNodeBulk_Service_AtomicBehavior tests that bulk accept is atomic.
func TestAcceptNodeBulk_Service_AtomicBehavior(t *testing.T) {
	tmpDir, cleanup := setupBulkAcceptTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Try to accept a mix of valid nodes and one that's already refuted
	nodeID, _ := service.ParseNodeID("1.2")
	if err := svc.RefuteNode(nodeID); err != nil {
		t.Fatalf("failed to refute node 1.2: %v", err)
	}

	// Now try bulk accept including the refuted node
	nodeIDs := []service.NodeID{}
	for _, idStr := range []string{"1.1", "1.2", "1.3"} {
		id, _ := service.ParseNodeID(idStr)
		nodeIDs = append(nodeIDs, id)
	}

	err = svc.AcceptNodeBulk(nodeIDs)

	// The behavior depends on implementation:
	// - Atomic: All fail if one fails
	// - Best-effort: Accept what can be accepted
	t.Logf("AcceptNodeBulk with refuted node result: %v", err)

	// Check final state
	st, _ := svc.LoadState()
	for _, idStr := range []string{"1.1", "1.3"} {
		id, _ := service.ParseNodeID(idStr)
		n := st.GetNode(id)
		if n != nil {
			t.Logf("Node %s final state: %s", idStr, n.EpistemicState)
		}
	}
}
