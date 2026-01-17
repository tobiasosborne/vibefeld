//go:build integration

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/scope"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// setupScopeAcceptanceTest creates a temporary proof directory for testing.
func setupScopeAcceptanceTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-scope-acceptance-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// TestScopeBalance_UnbalancedScopeFails tests that nodes with unbalanced scopes
// (local_assume without matching local_discharge) are detected as invalid.
//
// This test verifies the scope validation invariant from the PRD:
// "All scope entries opened by n (if local_assume) are closed by a descendant".
//
// Test scenario:
// 1. Create a proof with a local_assume node that opens a scope
// 2. Do NOT create a local_discharge node to close the scope
// 3. Verify ValidateScopeBalance detects the unbalanced scope
func TestScopeBalance_UnbalancedScopeFails(t *testing.T) {
	// Create nodes representing an unbalanced proof structure
	assumeID := mustParseNodeID(t, "1.1")
	claimID := mustParseNodeID(t, "1.2")

	// Create a local_assume node (opens scope)
	assumeNode, err := node.NewNode(
		assumeID,
		schema.NodeTypeLocalAssume,
		"Assume P for proof by contradiction",
		schema.InferenceLocalAssume,
	)
	if err != nil {
		t.Fatalf("Failed to create assume node: %v", err)
	}

	// Create a regular claim node (does NOT close scope)
	claimNode, err := node.NewNode(
		claimID,
		schema.NodeTypeClaim,
		"Some intermediate step",
		schema.InferenceModusPonens,
	)
	if err != nil {
		t.Fatalf("Failed to create claim node: %v", err)
	}

	// Nodes without a matching discharge - unbalanced!
	unbalancedNodes := []*node.Node{assumeNode, claimNode}

	// ValidateScopeBalance should detect the unbalanced scope
	err = scope.ValidateScopeBalance(unbalancedNodes)
	if err == nil {
		t.Error("ValidateScopeBalance should fail for unbalanced scope (assume without discharge)")
	}

	t.Logf("Correctly detected unbalanced scope: %v", err)
}

// TestScopeBalance_BalancedScopeSucceeds tests that nodes with balanced scopes
// (local_assume with matching local_discharge) pass validation.
func TestScopeBalance_BalancedScopeSucceeds(t *testing.T) {
	// Create nodes representing a balanced proof structure
	assumeID := mustParseNodeID(t, "1.1")
	claimID := mustParseNodeID(t, "1.2")
	dischargeID := mustParseNodeID(t, "1.3")

	// Create a local_assume node (opens scope)
	assumeNode, err := node.NewNode(
		assumeID,
		schema.NodeTypeLocalAssume,
		"Assume P for proof by contradiction",
		schema.InferenceLocalAssume,
	)
	if err != nil {
		t.Fatalf("Failed to create assume node: %v", err)
	}

	// Create a regular claim node
	claimNode, err := node.NewNode(
		claimID,
		schema.NodeTypeClaim,
		"Derive contradiction from P",
		schema.InferenceModusPonens,
	)
	if err != nil {
		t.Fatalf("Failed to create claim node: %v", err)
	}

	// Create a local_discharge node (closes scope)
	dischargeNode, err := node.NewNode(
		dischargeID,
		schema.NodeTypeLocalDischarge,
		"Therefore not-P by contradiction",
		schema.InferenceLocalDischarge,
	)
	if err != nil {
		t.Fatalf("Failed to create discharge node: %v", err)
	}

	// Nodes with matching assume and discharge - balanced!
	balancedNodes := []*node.Node{assumeNode, claimNode, dischargeNode}

	// ValidateScopeBalance should pass for balanced scope
	err = scope.ValidateScopeBalance(balancedNodes)
	if err != nil {
		t.Errorf("ValidateScopeBalance should pass for balanced scope: %v", err)
	}

	t.Log("Correctly validated balanced scope")
}

// TestScopeClosure_LocalAssumeRequiresDischarge tests that a local_assume node
// cannot be validated if its scope entry is still active (not discharged).
func TestScopeClosure_LocalAssumeRequiresDischarge(t *testing.T) {
	assumeID := mustParseNodeID(t, "1.1")

	// Create a local_assume node
	assumeNode, err := node.NewNode(
		assumeID,
		schema.NodeTypeLocalAssume,
		"Assume P for proof",
		schema.InferenceLocalAssume,
	)
	if err != nil {
		t.Fatalf("Failed to create assume node: %v", err)
	}

	// Create a scope entry that is still active (not discharged)
	scopeEntry, err := scope.NewEntry(assumeID, "Assume P for proof")
	if err != nil {
		t.Fatalf("Failed to create scope entry: %v", err)
	}

	// Entry is active - ValidateScopeClosure should fail
	err = scope.ValidateScopeClosure(assumeNode, scopeEntry)
	if err == nil {
		t.Error("ValidateScopeClosure should fail when scope is still active")
	}

	t.Logf("Correctly detected unclosed scope: %v", err)

	// Now discharge the entry
	if err := scopeEntry.Discharge(); err != nil {
		t.Fatalf("Failed to discharge entry: %v", err)
	}

	// After discharge, ValidateScopeClosure should pass
	err = scope.ValidateScopeClosure(assumeNode, scopeEntry)
	if err != nil {
		t.Errorf("ValidateScopeClosure should pass after discharge: %v", err)
	}

	t.Log("Correctly validated after scope closure")
}

// TestScopeAcceptance_FullWorkflow tests the full workflow of creating a proof
// with local assumptions and verifying scope balance before acceptance.
func TestScopeAcceptance_FullWorkflow(t *testing.T) {
	proofDir, cleanup := setupScopeAcceptanceTest(t)
	defer cleanup()

	conjecture := "If P implies Q and P, then Q (modus ponens)"

	// Initialize proof
	t.Log("Step 1: Initialize proof")
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	if err := service.Init(proofDir, conjecture, "test-author"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	rootID, _ := types.Parse("1")
	proverOwner := "prover-agent-001"

	// Claim root and refine with a local assumption structure
	t.Log("Step 2: Create proof structure with local assumption")
	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	// Create a local_assume child node
	assumeID, _ := types.Parse("1.1")
	if err := svc.RefineNode(rootID, proverOwner, assumeID, schema.NodeTypeLocalAssume,
		"Assume P for proof",
		schema.InferenceLocalAssume); err != nil {
		t.Fatalf("RefineNode (assume) failed: %v", err)
	}

	// Create an intermediate claim
	claimID, _ := types.Parse("1.2")
	if err := svc.RefineNode(rootID, proverOwner, claimID, schema.NodeTypeClaim,
		"From P, derive Q",
		schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (claim) failed: %v", err)
	}

	// Create a local_discharge to close the scope
	dischargeID, _ := types.Parse("1.3")
	if err := svc.RefineNode(rootID, proverOwner, dischargeID, schema.NodeTypeLocalDischarge,
		"Discharge assumption P, conclude P implies Q",
		schema.InferenceLocalDischarge); err != nil {
		t.Fatalf("RefineNode (discharge) failed: %v", err)
	}

	if err := svc.ReleaseNode(rootID, proverOwner); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	// Load state and verify structure
	t.Log("Step 3: Verify proof structure")
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	nodes := state.AllNodes()
	if len(nodes) != 4 {
		t.Errorf("Expected 4 nodes (root + 3 children), got %d", len(nodes))
	}

	// Verify scope balance of the child nodes
	var childNodes []*node.Node
	for _, n := range nodes {
		if n.ID.String() != "1" {
			childNodes = append(childNodes, n)
		}
	}

	err = scope.ValidateScopeBalance(childNodes)
	if err != nil {
		t.Errorf("Scope should be balanced with assume and discharge: %v", err)
	}

	// Accept all nodes
	t.Log("Step 4: Accept all nodes")
	for _, nodeID := range []types.NodeID{assumeID, claimID, dischargeID, rootID} {
		if err := svc.AcceptNode(nodeID); err != nil {
			t.Fatalf("AcceptNode (%s) failed: %v", nodeID, err)
		}
	}

	// Verify all nodes validated
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	for _, n := range state.AllNodes() {
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("Node %s should be validated, got %s", n.ID, n.EpistemicState)
		}
	}

	t.Log("")
	t.Log("========================================")
	t.Log("  SCOPE ACCEPTANCE TEST PASSED!")
	t.Log("  - Created proof with local assumption")
	t.Log("  - Properly discharged scope")
	t.Log("  - All nodes validated")
	t.Log("========================================")
}
