//go:build integration

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// setupTest creates a temporary proof directory for testing.
// Returns the proof directory path and a cleanup function.
func setupTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-simple-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// TestSimpleProof_FullCompletion tests a complete simple proof workflow
// from initialization through all nodes being validated.
//
// This test exercises the core workflow:
//  1. Initialize proof with a conjecture
//  2. Claim root node as prover
//  3. Refine root into child nodes
//  4. Release root node
//  5. Accept all children as verifier
//  6. Accept root node
//  7. Verify all nodes are validated
func TestSimpleProof_FullCompletion(t *testing.T) {
	proofDir, cleanup := setupTest(t)
	defer cleanup()

	conjecture := "If n is even, then n+1 is odd"

	// ==========================================================================
	// Step 1: Initialize proof with conjecture
	// ==========================================================================
	t.Log("Step 1: Initialize proof with conjecture")

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

	// Verify root node was created
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootID, _ := types.Parse("1")
	rootNode := state.GetNode(rootID)
	if rootNode == nil {
		t.Fatal("Root node 1 was not created")
	}

	t.Logf("  Root node created with statement: %s", rootNode.Statement)

	// ==========================================================================
	// Step 2: Claim root node as prover
	// ==========================================================================
	t.Log("Step 2: Claim root node as prover")

	proverOwner := "prover-agent-001"
	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	// Verify node is claimed
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode = state.GetNode(rootID)
	if rootNode.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("Root node workflow state = %q, want %q", rootNode.WorkflowState, schema.WorkflowClaimed)
	}
	if rootNode.ClaimedBy != proverOwner {
		t.Errorf("Root node claimed by = %q, want %q", rootNode.ClaimedBy, proverOwner)
	}

	t.Log("  Root node claimed by prover")

	// ==========================================================================
	// Step 3: Refine into 2 children
	// ==========================================================================
	t.Log("Step 3: Refine root into child nodes")

	child1ID, _ := types.Parse("1.1")
	child2ID, _ := types.Parse("1.2")

	// Add first child - an assumption
	if err := svc.RefineNode(rootID, proverOwner, child1ID, schema.NodeTypeClaim,
		"Let n be an even number, so n = 2k for some integer k",
		schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode (1.1) failed: %v", err)
	}

	t.Log("  Created child node 1.1")

	// Add second child - the conclusion via modus ponens
	if err := svc.RefineNode(rootID, proverOwner, child2ID, schema.NodeTypeClaim,
		"Then n+1 = 2k+1, which is odd by definition of odd numbers",
		schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (1.2) failed: %v", err)
	}

	t.Log("  Created child node 1.2")

	// Verify children were created
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if state.GetNode(child1ID) == nil {
		t.Fatal("Child node 1.1 was not created")
	}
	if state.GetNode(child2ID) == nil {
		t.Fatal("Child node 1.2 was not created")
	}

	nodes := state.AllNodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes (root + 2 children), got %d", len(nodes))
	}

	// ==========================================================================
	// Step 4: Release root node
	// ==========================================================================
	t.Log("Step 4: Release root node")

	if err := svc.ReleaseNode(rootID, proverOwner); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	// Verify node is released
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode = state.GetNode(rootID)
	if rootNode.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("Root node workflow state = %q, want %q", rootNode.WorkflowState, schema.WorkflowAvailable)
	}

	t.Log("  Root node released")

	// ==========================================================================
	// Step 5: Accept all children (verifier)
	// ==========================================================================
	t.Log("Step 5: Verifier accepts child nodes")

	// Accept child 1.1
	if err := svc.AcceptNode(child1ID); err != nil {
		t.Fatalf("AcceptNode (1.1) failed: %v", err)
	}

	t.Log("  Node 1.1 validated")

	// Accept child 1.2
	if err := svc.AcceptNode(child2ID); err != nil {
		t.Fatalf("AcceptNode (1.2) failed: %v", err)
	}

	t.Log("  Node 1.2 validated")

	// ==========================================================================
	// Step 6: Accept root node
	// ==========================================================================
	t.Log("Step 6: Verifier accepts root node")

	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	t.Log("  Root node validated")

	// ==========================================================================
	// Step 7: Verify all nodes are validated
	// ==========================================================================
	t.Log("Step 7: Verify all nodes are validated")

	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Check each node's epistemic state
	for _, n := range state.AllNodes() {
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("Node %s epistemic state = %q, want %q",
				n.ID, n.EpistemicState, schema.EpistemicValidated)
		}
	}

	// Count validated nodes
	validatedCount := 0
	for _, n := range state.AllNodes() {
		if n.EpistemicState == schema.EpistemicValidated {
			validatedCount++
		}
	}

	if validatedCount != 3 {
		t.Errorf("Validated node count = %d, want 3", validatedCount)
	}

	t.Log("")
	t.Log("========================================")
	t.Log("  SIMPLE PROOF COMPLETION SUCCESSFUL!")
	t.Log("  All 3 nodes validated")
	t.Log("========================================")
}

// TestSimpleProof_ThreeChildRefinement tests refining into 3 children
// and completing the proof.
func TestSimpleProof_ThreeChildRefinement(t *testing.T) {
	proofDir, cleanup := setupTest(t)
	defer cleanup()

	conjecture := "The sum of two even numbers is even"

	// Initialize
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
	proverOwner := "prover"

	// Claim root
	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	// Refine into 3 children
	child1ID, _ := types.Parse("1.1")
	child2ID, _ := types.Parse("1.2")
	child3ID, _ := types.Parse("1.3")

	refinements := []struct {
		childID   types.NodeID
		statement string
		inference schema.InferenceType
	}{
		{child1ID, "Let a = 2m for some integer m", schema.InferenceAssumption},
		{child2ID, "Let b = 2n for some integer n", schema.InferenceAssumption},
		{child3ID, "Then a + b = 2(m+n), which is even", schema.InferenceModusPonens},
	}

	for _, r := range refinements {
		if err := svc.RefineNode(rootID, proverOwner, r.childID, schema.NodeTypeClaim,
			r.statement, r.inference); err != nil {
			t.Fatalf("RefineNode (%s) failed: %v", r.childID, err)
		}
	}

	// Release root
	if err := svc.ReleaseNode(rootID, proverOwner); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	// Accept all nodes
	allNodes := []types.NodeID{child1ID, child2ID, child3ID, rootID}
	for _, nodeID := range allNodes {
		if err := svc.AcceptNode(nodeID); err != nil {
			t.Fatalf("AcceptNode (%s) failed: %v", nodeID, err)
		}
	}

	// Verify final state
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	nodes := state.AllNodes()
	if len(nodes) != 4 {
		t.Errorf("Expected 4 nodes (root + 3 children), got %d", len(nodes))
	}

	for _, n := range nodes {
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("Node %s epistemic state = %q, want %q",
				n.ID, n.EpistemicState, schema.EpistemicValidated)
		}
	}

	t.Logf("3-child refinement completed: %d nodes validated", len(nodes))
}

// TestSimpleProof_ProofStatus tests that the service status reflects
// the proof's progress correctly.
func TestSimpleProof_ProofStatus(t *testing.T) {
	proofDir, cleanup := setupTest(t)
	defer cleanup()

	// Initialize
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	if err := service.Init(proofDir, "Test status", "test-author"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	// Check initial status
	status, err := svc.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if !status.Initialized {
		t.Error("Expected proof to be initialized")
	}
	if status.TotalNodes != 1 {
		t.Errorf("TotalNodes = %d, want 1", status.TotalNodes)
	}
	if status.PendingNodes != 1 {
		t.Errorf("PendingNodes = %d, want 1", status.PendingNodes)
	}
	if status.ValidatedNodes != 0 {
		t.Errorf("ValidatedNodes = %d, want 0", status.ValidatedNodes)
	}

	// Claim and refine
	rootID, _ := types.Parse("1")
	if err := svc.ClaimNode(rootID, "prover", 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	child1ID, _ := types.Parse("1.1")
	if err := svc.RefineNode(rootID, "prover", child1ID, schema.NodeTypeClaim,
		"Child step", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode failed: %v", err)
	}

	// Check status after refinement
	status, err = svc.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if status.TotalNodes != 2 {
		t.Errorf("TotalNodes = %d, want 2", status.TotalNodes)
	}
	if status.ClaimedNodes != 1 {
		t.Errorf("ClaimedNodes = %d, want 1", status.ClaimedNodes)
	}

	// Release and validate
	if err := svc.ReleaseNode(rootID, "prover"); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	if err := svc.AcceptNode(child1ID); err != nil {
		t.Fatalf("AcceptNode failed: %v", err)
	}

	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode failed: %v", err)
	}

	// Check final status
	status, err = svc.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if status.ValidatedNodes != 2 {
		t.Errorf("ValidatedNodes = %d, want 2", status.ValidatedNodes)
	}
	if status.PendingNodes != 0 {
		t.Errorf("PendingNodes = %d, want 0", status.PendingNodes)
	}

	t.Log("Status tracking verified throughout proof lifecycle")
}
