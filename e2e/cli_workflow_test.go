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

// setupCLIWorkflowTest creates a temporary proof directory for CLI workflow testing.
func setupCLIWorkflowTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-cli-workflow-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// TestCLIWorkflow_InitClaimRefineReleaseAccept tests the core CLI command chaining
// workflow: init -> claim -> refine -> release -> accept
//
// This test simulates the sequence of CLI commands that would be executed
// when developing a proof, verifying that each command properly chains
// with the next and the final proof state is correct.
func TestCLIWorkflow_InitClaimRefineReleaseAccept(t *testing.T) {
	proofDir, cleanup := setupCLIWorkflowTest(t)
	defer cleanup()

	conjecture := "The product of two odd numbers is odd"
	proverOwner := "prover-agent-cli"

	// ==========================================================================
	// CLI Command 1: af init "conjecture"
	// ==========================================================================
	t.Log("CLI Command 1: af init")

	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	if err := service.Init(proofDir, conjecture, "cli-user"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	// Verify initialization worked
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if !status.Initialized {
		t.Fatal("Proof should be initialized after init command")
	}
	if status.TotalNodes != 1 {
		t.Errorf("Expected 1 node after init, got %d", status.TotalNodes)
	}

	t.Log("  Proof initialized with root node")

	// ==========================================================================
	// CLI Command 2: af claim 1
	// ==========================================================================
	t.Log("CLI Command 2: af claim 1")

	rootID, _ := types.Parse("1")
	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	// Verify claim worked
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := state.GetNode(rootID)
	if rootNode.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("Root workflow state = %q, want %q", rootNode.WorkflowState, schema.WorkflowClaimed)
	}
	if rootNode.ClaimedBy != proverOwner {
		t.Errorf("Root claimed by = %q, want %q", rootNode.ClaimedBy, proverOwner)
	}

	t.Log("  Root node claimed by prover")

	// ==========================================================================
	// CLI Command 3: af refine 1 --statement "..." --inference "..."
	// ==========================================================================
	t.Log("CLI Command 3: af refine 1 (first child)")

	child1ID, _ := types.Parse("1.1")
	if err := svc.RefineNode(rootID, proverOwner, child1ID, schema.NodeTypeClaim,
		"Let a = 2m+1 and b = 2n+1 for integers m, n",
		schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode (1.1) failed: %v", err)
	}

	t.Log("  Created child node 1.1")

	// ==========================================================================
	// CLI Command 4: af refine 1 (second child)
	// ==========================================================================
	t.Log("CLI Command 4: af refine 1 (second child)")

	child2ID, _ := types.Parse("1.2")
	if err := svc.RefineNode(rootID, proverOwner, child2ID, schema.NodeTypeClaim,
		"Then a*b = (2m+1)(2n+1) = 4mn + 2m + 2n + 1 = 2(2mn+m+n) + 1, which is odd",
		schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (1.2) failed: %v", err)
	}

	t.Log("  Created child node 1.2")

	// Verify refinements worked
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if len(state.AllNodes()) != 3 {
		t.Errorf("Expected 3 nodes after refinements, got %d", len(state.AllNodes()))
	}

	// ==========================================================================
	// CLI Command 5: af release 1
	// ==========================================================================
	t.Log("CLI Command 5: af release 1")

	if err := svc.ReleaseNode(rootID, proverOwner); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	// Verify release worked
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode = state.GetNode(rootID)
	if rootNode.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("Root workflow state = %q, want %q", rootNode.WorkflowState, schema.WorkflowAvailable)
	}

	t.Log("  Root node released")

	// ==========================================================================
	// CLI Command 6: af accept 1.1
	// ==========================================================================
	t.Log("CLI Command 6: af accept 1.1")

	if err := svc.AcceptNode(child1ID); err != nil {
		t.Fatalf("AcceptNode (1.1) failed: %v", err)
	}

	t.Log("  Node 1.1 validated")

	// ==========================================================================
	// CLI Command 7: af accept 1.2
	// ==========================================================================
	t.Log("CLI Command 7: af accept 1.2")

	if err := svc.AcceptNode(child2ID); err != nil {
		t.Fatalf("AcceptNode (1.2) failed: %v", err)
	}

	t.Log("  Node 1.2 validated")

	// ==========================================================================
	// CLI Command 8: af accept 1
	// ==========================================================================
	t.Log("CLI Command 8: af accept 1")

	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	t.Log("  Root node validated")

	// ==========================================================================
	// Verify final state
	// ==========================================================================
	t.Log("Verifying final proof state")

	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// All nodes should be validated
	for _, n := range state.AllNodes() {
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("Node %s epistemic state = %q, want %q",
				n.ID, n.EpistemicState, schema.EpistemicValidated)
		}
	}

	status, err = svc.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if status.ValidatedNodes != 3 {
		t.Errorf("ValidatedNodes = %d, want 3", status.ValidatedNodes)
	}

	t.Log("")
	t.Log("========================================")
	t.Log("  CLI WORKFLOW TEST COMPLETE!")
	t.Log("  Command sequence verified:")
	t.Log("    1. init   -> proof created")
	t.Log("    2. claim  -> node locked")
	t.Log("    3. refine -> children added")
	t.Log("    4. release -> node unlocked")
	t.Log("    5. accept  -> nodes validated")
	t.Log("========================================")
}

// TestCLIWorkflow_ClaimExtendRelease tests the claim extension workflow:
// claim -> extend-claim -> release
func TestCLIWorkflow_ClaimExtendRelease(t *testing.T) {
	proofDir, cleanup := setupCLIWorkflowTest(t)
	defer cleanup()

	// Initialize proof
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}
	if err := service.Init(proofDir, "Claim extension test", "cli-user"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	rootID, _ := types.Parse("1")
	owner := "long-running-agent"

	// ==========================================================================
	// CLI Command 1: af claim 1 --timeout 1m
	// ==========================================================================
	t.Log("CLI Command 1: af claim 1 --timeout 1m")

	if err := svc.ClaimNode(rootID, owner, 1*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := state.GetNode(rootID)
	if rootNode.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("Root workflow state = %q, want %q", rootNode.WorkflowState, schema.WorkflowClaimed)
	}

	t.Log("  Node claimed with 1 minute timeout")

	// ==========================================================================
	// CLI Command 2: af extend-claim 1 --timeout 5m
	// ==========================================================================
	t.Log("CLI Command 2: af extend-claim 1 --timeout 5m")

	if err := svc.RefreshClaim(rootID, owner, 5*time.Minute); err != nil {
		t.Fatalf("RefreshClaim failed: %v", err)
	}

	t.Log("  Claim extended to 5 minutes")

	// ==========================================================================
	// CLI Command 3: af release 1
	// ==========================================================================
	t.Log("CLI Command 3: af release 1")

	if err := svc.ReleaseNode(rootID, owner); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode = state.GetNode(rootID)
	if rootNode.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("Root workflow state = %q, want %q", rootNode.WorkflowState, schema.WorkflowAvailable)
	}

	t.Log("  Node released successfully")
	t.Log("  Claim extension workflow verified")
}

// TestCLIWorkflow_MultipleRefinements tests sequential refinements to build
// a deeper proof tree: init -> claim -> refine -> refine (nested) -> release -> accept all
func TestCLIWorkflow_MultipleRefinements(t *testing.T) {
	proofDir, cleanup := setupCLIWorkflowTest(t)
	defer cleanup()

	// Initialize proof
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}
	if err := service.Init(proofDir, "Deep proof tree test", "cli-user"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	prover := "prover-agent"

	// ==========================================================================
	// Build tree: 1 -> 1.1 -> 1.1.1
	// ==========================================================================
	t.Log("Building proof tree: 1 -> 1.1 -> 1.1.1")

	rootID, _ := types.Parse("1")
	child1ID, _ := types.Parse("1.1")
	grandchildID, _ := types.Parse("1.1.1")

	// Claim and refine root
	if err := svc.ClaimNode(rootID, prover, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	if err := svc.RefineNode(rootID, prover, child1ID, schema.NodeTypeClaim,
		"First level claim", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode (1.1) failed: %v", err)
	}

	if err := svc.ReleaseNode(rootID, prover); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	t.Log("  Created 1 -> 1.1")

	// Claim and refine child
	if err := svc.ClaimNode(child1ID, prover, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (1.1) failed: %v", err)
	}

	if err := svc.RefineNode(child1ID, prover, grandchildID, schema.NodeTypeClaim,
		"Second level claim", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (1.1.1) failed: %v", err)
	}

	if err := svc.ReleaseNode(child1ID, prover); err != nil {
		t.Fatalf("ReleaseNode (1.1) failed: %v", err)
	}

	t.Log("  Created 1.1 -> 1.1.1")

	// ==========================================================================
	// Accept from leaves to root
	// ==========================================================================
	t.Log("Accepting nodes from leaves to root")

	// Accept grandchild first
	if err := svc.AcceptNode(grandchildID); err != nil {
		t.Fatalf("AcceptNode (1.1.1) failed: %v", err)
	}
	t.Log("  Accepted 1.1.1")

	// Accept child
	if err := svc.AcceptNode(child1ID); err != nil {
		t.Fatalf("AcceptNode (1.1) failed: %v", err)
	}
	t.Log("  Accepted 1.1")

	// Accept root
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}
	t.Log("  Accepted 1")

	// ==========================================================================
	// Verify final state
	// ==========================================================================
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if len(state.AllNodes()) != 3 {
		t.Errorf("Expected 3 nodes in tree, got %d", len(state.AllNodes()))
	}

	for _, n := range state.AllNodes() {
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("Node %s epistemic state = %q, want %q",
				n.ID, n.EpistemicState, schema.EpistemicValidated)
		}
	}

	t.Log("  Deep proof tree fully validated")
}

// TestCLIWorkflow_ErrorRecovery tests that the workflow handles errors gracefully:
// attempting operations out of order or with invalid parameters
func TestCLIWorkflow_ErrorRecovery(t *testing.T) {
	proofDir, cleanup := setupCLIWorkflowTest(t)
	defer cleanup()

	// Initialize proof
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}
	if err := service.Init(proofDir, "Error recovery test", "cli-user"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	rootID, _ := types.Parse("1")
	nonExistentID, _ := types.Parse("99")
	owner := "test-agent"
	wrongOwner := "wrong-agent"

	// ==========================================================================
	// Test 1: Cannot refine without claiming
	// ==========================================================================
	t.Log("Test 1: Cannot refine without claiming")

	childID, _ := types.Parse("1.1")
	err = svc.RefineNode(rootID, owner, childID, schema.NodeTypeClaim,
		"Should fail", schema.InferenceAssumption)
	if err == nil {
		t.Error("Expected error when refining unclaimed node")
	} else {
		t.Log("  Correctly rejected: refine without claim")
	}

	// ==========================================================================
	// Test 2: Cannot release unclaimed node
	// ==========================================================================
	t.Log("Test 2: Cannot release unclaimed node")

	err = svc.ReleaseNode(rootID, owner)
	if err == nil {
		t.Error("Expected error when releasing unclaimed node")
	} else {
		t.Log("  Correctly rejected: release without claim")
	}

	// ==========================================================================
	// Test 3: Cannot claim non-existent node
	// ==========================================================================
	t.Log("Test 3: Cannot claim non-existent node")

	err = svc.ClaimNode(nonExistentID, owner, 5*time.Minute)
	if err == nil {
		t.Error("Expected error when claiming non-existent node")
	} else {
		t.Log("  Correctly rejected: claim non-existent node")
	}

	// ==========================================================================
	// Test 4: Cannot release with wrong owner
	// ==========================================================================
	t.Log("Test 4: Cannot release with wrong owner")

	if err := svc.ClaimNode(rootID, owner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	err = svc.ReleaseNode(rootID, wrongOwner)
	if err == nil {
		t.Error("Expected error when releasing with wrong owner")
	} else {
		t.Log("  Correctly rejected: release with wrong owner")
	}

	// Cleanup: release with correct owner
	if err := svc.ReleaseNode(rootID, owner); err != nil {
		t.Fatalf("ReleaseNode (cleanup) failed: %v", err)
	}

	t.Log("  Error recovery tests passed")
}
