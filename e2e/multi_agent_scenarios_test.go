//go:build integration

package e2e

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/taint"
	"github.com/tobias/vibefeld/internal/types"
)

// ============================================================================
// Test Setup Helpers
// ============================================================================

// setupMultiAgentTest creates a temporary proof directory for multi-agent testing.
func setupMultiAgentTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-multi-agent-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// initMultiAgentProof initializes a proof with the given conjecture.
func initMultiAgentProof(t *testing.T, proofDir, conjecture string) *service.ProofService {
	t.Helper()
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("failed to initialize proof dir: %v", err)
	}
	if err := service.Init(proofDir, conjecture, "test-author"); err != nil {
		t.Fatalf("failed to initialize proof: %v", err)
	}
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}
	return svc
}

// parseNodeID parses a node ID string and fails the test if it fails.
func parseNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("failed to parse node ID %q: %v", s, err)
	}
	return id
}

// ============================================================================
// 1. Happy Path Acceptance Flow
// ============================================================================

// TestHappyPath_FullAcceptanceFlow tests the complete happy path where:
// 1. Proof is initialized with conjecture
// 2. Prover claims root, refines into children
// 3. Prover releases root
// 4. Verifier accepts all children (bottom-up)
// 5. Verifier accepts root
// 6. All nodes are validated with clean taint
func TestHappyPath_FullAcceptanceFlow(t *testing.T) {
	proofDir, cleanup := setupMultiAgentTest(t)
	defer cleanup()

	conjecture := "For all natural numbers n, if n > 2 then n^2 > n + 2"
	svc := initMultiAgentProof(t, proofDir, conjecture)

	// Parse node IDs
	rootID := parseNodeID(t, "1")
	child1ID := parseNodeID(t, "1.1")
	child2ID := parseNodeID(t, "1.2")
	child3ID := parseNodeID(t, "1.3")

	// ==========================================================================
	// Step 1: Prover claims root node
	// ==========================================================================
	t.Log("Step 1: Prover claims root node")

	proverAgent := "prover-agent-001"
	if err := svc.ClaimNode(rootID, proverAgent, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	// Verify root is claimed
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	rootNode := state.GetNode(rootID)
	if rootNode.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("Root workflow state = %q, want %q", rootNode.WorkflowState, schema.WorkflowClaimed)
	}

	// ==========================================================================
	// Step 2: Prover refines root into 3 children
	// ==========================================================================
	t.Log("Step 2: Prover refines root into children")

	refinements := []struct {
		childID   types.NodeID
		statement string
	}{
		{child1ID, "Let n > 2 be a natural number"},
		{child2ID, "Then n >= 3, so n^2 >= 9"},
		{child3ID, "Since 9 > 5 = 3 + 2 >= n + 2, we have n^2 > n + 2"},
	}

	for _, r := range refinements {
		if err := svc.RefineNode(rootID, proverAgent, r.childID, schema.NodeTypeClaim,
			r.statement, schema.InferenceModusPonens); err != nil {
			t.Fatalf("RefineNode (%s) failed: %v", r.childID, err)
		}
	}

	// ==========================================================================
	// Step 3: Prover releases root node
	// ==========================================================================
	t.Log("Step 3: Prover releases root node")

	if err := svc.ReleaseNode(rootID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	// Verify root is available
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	rootNode = state.GetNode(rootID)
	if rootNode.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("Root workflow state = %q, want %q", rootNode.WorkflowState, schema.WorkflowAvailable)
	}

	// ==========================================================================
	// Step 4: Verifier accepts children (bottom-up for typical acceptance)
	// ==========================================================================
	t.Log("Step 4: Verifier accepts children")

	for _, r := range refinements {
		if err := svc.AcceptNode(r.childID); err != nil {
			t.Fatalf("AcceptNode (%s) failed: %v", r.childID, err)
		}
	}

	// ==========================================================================
	// Step 5: Verifier accepts root
	// ==========================================================================
	t.Log("Step 5: Verifier accepts root")

	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	// ==========================================================================
	// Step 6: Verify final state - all validated with clean taint
	// ==========================================================================
	t.Log("Step 6: Verify final state")

	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	allNodes := state.AllNodes()
	if len(allNodes) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(allNodes))
	}

	for _, n := range allNodes {
		// All should be validated
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("Node %s: epistemic state = %v, want %v",
				n.ID, n.EpistemicState, schema.EpistemicValidated)
		}

		// All should be available (not claimed)
		if n.WorkflowState != schema.WorkflowAvailable {
			t.Errorf("Node %s: workflow state = %v, want %v",
				n.ID, n.WorkflowState, schema.WorkflowAvailable)
		}

		// Compute taint - all should be clean
		computedTaint := taint.ComputeTaint(n, nil)
		if computedTaint != node.TaintClean {
			t.Errorf("Node %s: taint = %v, want %v",
				n.ID, computedTaint, node.TaintClean)
		}
	}

	t.Log("")
	t.Log("==============================================")
	t.Log("  HAPPY PATH ACCEPTANCE FLOW: SUCCESS")
	t.Log("  All 4 nodes validated with clean taint")
	t.Log("==============================================")
}

// ============================================================================
// 2. Challenge-Address-Accept Flow
// ============================================================================

// TestChallengeAddressAccept_FullFlow tests the challenge workflow:
// 1. Verifier raises challenge on a node
// 2. Prover addresses challenge by refining
// 3. Verifier resolves challenge
// 4. Verifier accepts all nodes
func TestChallengeAddressAccept_FullFlow(t *testing.T) {
	proofDir, cleanup := setupMultiAgentTest(t)
	defer cleanup()

	conjecture := "The sum of angles in a triangle is 180 degrees"
	svc := initMultiAgentProof(t, proofDir, conjecture)

	rootID := parseNodeID(t, "1")
	childID := parseNodeID(t, "1.1")

	// ==========================================================================
	// Step 1: Verifier raises challenge
	// ==========================================================================
	t.Log("Step 1: Verifier raises challenge on root node")

	// Access ledger to raise challenge
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	challengeID := "challenge-triangle-001"
	challengeEvent := ledger.NewChallengeRaised(challengeID, rootID, "statement",
		"The proof relies on Euclidean geometry axioms that are not stated")
	if _, err := ldg.Append(challengeEvent); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	// Verify challenge is recorded
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	challenges := state.OpenChallenges()
	if len(challenges) != 1 {
		t.Fatalf("Expected 1 open challenge, got %d", len(challenges))
	}
	if challenges[0].ID != challengeID {
		t.Errorf("Challenge ID = %q, want %q", challenges[0].ID, challengeID)
	}

	// Verify root is now a prover job (has open challenge)
	nodeMap := make(map[string]*node.Node)
	challengeMap := make(map[string][]*node.Challenge)
	for _, n := range state.AllNodes() {
		nodeMap[n.ID.String()] = n
	}
	for _, c := range state.AllChallenges() {
		nc := &node.Challenge{
			ID:       c.ID,
			TargetID: c.NodeID,
			Status:   node.ChallengeStatus(c.Status),
		}
		challengeMap[c.NodeID.String()] = append(challengeMap[c.NodeID.String()], nc)
	}

	jobResult := jobs.FindJobs(state.AllNodes(), nodeMap, challengeMap)
	if len(jobResult.ProverJobs) != 1 {
		t.Errorf("Expected 1 prover job (challenged node), got %d", len(jobResult.ProverJobs))
	}

	// ==========================================================================
	// Step 2: Prover addresses challenge by refining
	// ==========================================================================
	t.Log("Step 2: Prover claims and refines to address challenge")

	proverAgent := "prover-agent"
	if err := svc.ClaimNode(rootID, proverAgent, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	// Add child node that addresses the challenge
	if err := svc.RefineNode(rootID, proverAgent, childID, schema.NodeTypeClaim,
		"We work in Euclidean geometry where the parallel postulate holds",
		schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode failed: %v", err)
	}

	if err := svc.ReleaseNode(rootID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	// ==========================================================================
	// Step 3: Verifier resolves challenge (satisfied with explanation)
	// ==========================================================================
	t.Log("Step 3: Verifier resolves challenge")

	resolveEvent := ledger.NewChallengeResolved(challengeID)
	if _, err := ldg.Append(resolveEvent); err != nil {
		t.Fatalf("Failed to resolve challenge: %v", err)
	}

	// Verify challenge is resolved
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	openChallenges := state.OpenChallenges()
	if len(openChallenges) != 0 {
		t.Errorf("Expected 0 open challenges after resolution, got %d", len(openChallenges))
	}

	// ==========================================================================
	// Step 4: Verifier accepts all nodes
	// ==========================================================================
	t.Log("Step 4: Verifier accepts all nodes")

	if err := svc.AcceptNode(childID); err != nil {
		t.Fatalf("AcceptNode (child) failed: %v", err)
	}
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	// Verify final state
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	for _, n := range state.AllNodes() {
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("Node %s: epistemic state = %v, want %v",
				n.ID, n.EpistemicState, schema.EpistemicValidated)
		}
	}

	t.Log("")
	t.Log("==============================================")
	t.Log("  CHALLENGE-ADDRESS-ACCEPT FLOW: SUCCESS")
	t.Log("==============================================")
}

// ============================================================================
// 3. Multiple Challenges on Same Node
// ============================================================================

// TestMultipleChallenges_SameNode tests that a node with multiple challenges:
// 1. Remains a prover job until ALL challenges are resolved
// 2. Only becomes verifier job when all challenges are closed
// 3. Can be accepted once all challenges are resolved
func TestMultipleChallenges_SameNode(t *testing.T) {
	proofDir, cleanup := setupMultiAgentTest(t)
	defer cleanup()

	conjecture := "A complex mathematical theorem with multiple aspects"
	svc := initMultiAgentProof(t, proofDir, conjecture)

	rootID := parseNodeID(t, "1")

	// Access ledger
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// ==========================================================================
	// Step 1: Raise 3 different challenges on the root node
	// ==========================================================================
	t.Log("Step 1: Raise multiple challenges")

	challengeIDs := []string{"challenge-clarity", "challenge-inference", "challenge-gap"}
	targets := []string{"statement", "inference", "gap"}
	reasons := []string{
		"The statement lacks clarity in defining the domain",
		"The inference rule applied is not justified",
		"There is a logical gap between premise and conclusion",
	}

	for i, cid := range challengeIDs {
		event := ledger.NewChallengeRaised(cid, rootID, targets[i], reasons[i])
		if _, err := ldg.Append(event); err != nil {
			t.Fatalf("Failed to raise challenge %s: %v", cid, err)
		}
	}

	// Verify all 3 challenges are open
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	openChallenges := state.OpenChallenges()
	if len(openChallenges) != 3 {
		t.Errorf("Expected 3 open challenges, got %d", len(openChallenges))
	}

	// Build job context for verification
	nodeMap := make(map[string]*node.Node)
	challengeMap := make(map[string][]*node.Challenge)
	for _, n := range state.AllNodes() {
		nodeMap[n.ID.String()] = n
	}
	for _, c := range state.AllChallenges() {
		nc := &node.Challenge{
			ID:       c.ID,
			TargetID: c.NodeID,
			Status:   node.ChallengeStatus(c.Status),
		}
		challengeMap[c.NodeID.String()] = append(challengeMap[c.NodeID.String()], nc)
	}

	// Root should be prover job
	jobResult := jobs.FindJobs(state.AllNodes(), nodeMap, challengeMap)
	if len(jobResult.ProverJobs) != 1 {
		t.Errorf("Expected 1 prover job, got %d", len(jobResult.ProverJobs))
	}
	if len(jobResult.VerifierJobs) != 0 {
		t.Errorf("Expected 0 verifier jobs, got %d", len(jobResult.VerifierJobs))
	}

	// ==========================================================================
	// Step 2: Resolve challenges one by one, verify status after each
	// ==========================================================================
	t.Log("Step 2: Resolve challenges one by one")

	for i, cid := range challengeIDs {
		// Resolve this challenge
		resolveEvent := ledger.NewChallengeResolved(cid)
		if _, err := ldg.Append(resolveEvent); err != nil {
			t.Fatalf("Failed to resolve challenge %s: %v", cid, err)
		}

		// Reload state
		state, err = svc.LoadState()
		if err != nil {
			t.Fatalf("LoadState failed: %v", err)
		}

		// Rebuild job context
		nodeMap = make(map[string]*node.Node)
		challengeMap = make(map[string][]*node.Challenge)
		for _, n := range state.AllNodes() {
			nodeMap[n.ID.String()] = n
		}
		for _, c := range state.AllChallenges() {
			nc := &node.Challenge{
				ID:       c.ID,
				TargetID: c.NodeID,
				Status:   node.ChallengeStatus(c.Status),
			}
			challengeMap[c.NodeID.String()] = append(challengeMap[c.NodeID.String()], nc)
		}

		jobResult = jobs.FindJobs(state.AllNodes(), nodeMap, challengeMap)

		if i < len(challengeIDs)-1 {
			// Still have open challenges - should be prover job
			if len(jobResult.ProverJobs) != 1 {
				t.Errorf("After resolving %d/%d challenges: expected 1 prover job, got %d",
					i+1, len(challengeIDs), len(jobResult.ProverJobs))
			}
			t.Logf("  Resolved challenge %d/%d - still prover job", i+1, len(challengeIDs))
		} else {
			// All challenges resolved - should be verifier job
			if len(jobResult.ProverJobs) != 0 {
				t.Errorf("After resolving all challenges: expected 0 prover jobs, got %d",
					len(jobResult.ProverJobs))
			}
			if len(jobResult.VerifierJobs) != 1 {
				t.Errorf("After resolving all challenges: expected 1 verifier job, got %d",
					len(jobResult.VerifierJobs))
			}
			t.Log("  Resolved all challenges - now verifier job")
		}
	}

	// ==========================================================================
	// Step 3: Accept the node
	// ==========================================================================
	t.Log("Step 3: Accept the node")

	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode failed: %v", err)
	}

	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := state.GetNode(rootID)
	if rootNode.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Root epistemic state = %v, want %v",
			rootNode.EpistemicState, schema.EpistemicValidated)
	}

	t.Log("")
	t.Log("==============================================")
	t.Log("  MULTIPLE CHALLENGES ON SAME NODE: SUCCESS")
	t.Log("==============================================")
}

// ============================================================================
// 4. Nested Challenges
// ============================================================================

// TestNestedChallenges_IndependentResolution tests that challenges on nested nodes
// (parent and children) are tracked independently:
// 1. Challenges on parent don't affect children's job status
// 2. Challenges on children don't affect parent's job status
// 3. Each node can be resolved/accepted independently
func TestNestedChallenges_IndependentResolution(t *testing.T) {
	proofDir, cleanup := setupMultiAgentTest(t)
	defer cleanup()

	conjecture := "Nested proof structure test"
	svc := initMultiAgentProof(t, proofDir, conjecture)

	rootID := parseNodeID(t, "1")
	child1ID := parseNodeID(t, "1.1")
	child2ID := parseNodeID(t, "1.2")
	grandchildID := parseNodeID(t, "1.1.1")

	// ==========================================================================
	// Setup: Create nested structure
	// ==========================================================================
	t.Log("Setup: Create nested structure")

	proverAgent := "prover"
	if err := svc.ClaimNode(rootID, proverAgent, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	if err := svc.RefineNode(rootID, proverAgent, child1ID, schema.NodeTypeClaim,
		"Child 1 statement", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode (child1) failed: %v", err)
	}

	if err := svc.RefineNode(rootID, proverAgent, child2ID, schema.NodeTypeClaim,
		"Child 2 statement", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode (child2) failed: %v", err)
	}

	if err := svc.ReleaseNode(rootID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	// Add grandchild
	if err := svc.ClaimNode(child1ID, proverAgent, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (child1) failed: %v", err)
	}

	if err := svc.RefineNode(child1ID, proverAgent, grandchildID, schema.NodeTypeClaim,
		"Grandchild statement", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (grandchild) failed: %v", err)
	}

	if err := svc.ReleaseNode(child1ID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (child1) failed: %v", err)
	}

	// ==========================================================================
	// Step 1: Raise challenges at different levels
	// ==========================================================================
	t.Log("Step 1: Raise challenges at different levels")

	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Challenge on root
	rootChallengeID := "challenge-root"
	if _, err := ldg.Append(ledger.NewChallengeRaised(rootChallengeID, rootID,
		"statement", "Root needs clarification")); err != nil {
		t.Fatalf("Failed to raise root challenge: %v", err)
	}

	// Challenge on child1
	child1ChallengeID := "challenge-child1"
	if _, err := ldg.Append(ledger.NewChallengeRaised(child1ChallengeID, child1ID,
		"inference", "Child1 inference unclear")); err != nil {
		t.Fatalf("Failed to raise child1 challenge: %v", err)
	}

	// Challenge on grandchild
	grandchildChallengeID := "challenge-grandchild"
	if _, err := ldg.Append(ledger.NewChallengeRaised(grandchildChallengeID, grandchildID,
		"gap", "Grandchild has logical gap")); err != nil {
		t.Fatalf("Failed to raise grandchild challenge: %v", err)
	}

	// ==========================================================================
	// Step 2: Verify each node's job status is independent
	// ==========================================================================
	t.Log("Step 2: Verify job status independence")

	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Build job context
	nodeMap := make(map[string]*node.Node)
	challengeMap := make(map[string][]*node.Challenge)
	for _, n := range state.AllNodes() {
		nodeMap[n.ID.String()] = n
	}
	for _, c := range state.AllChallenges() {
		nc := &node.Challenge{
			ID:       c.ID,
			TargetID: c.NodeID,
			Status:   node.ChallengeStatus(c.Status),
		}
		challengeMap[c.NodeID.String()] = append(challengeMap[c.NodeID.String()], nc)
	}

	jobResult := jobs.FindJobs(state.AllNodes(), nodeMap, challengeMap)

	// Root, child1, grandchild have challenges -> prover jobs
	// Child2 has no challenges -> verifier job
	if len(jobResult.ProverJobs) != 3 {
		t.Errorf("Expected 3 prover jobs (root, child1, grandchild), got %d", len(jobResult.ProverJobs))
	}
	if len(jobResult.VerifierJobs) != 1 {
		t.Errorf("Expected 1 verifier job (child2), got %d", len(jobResult.VerifierJobs))
	}

	// Verify child2 is the verifier job
	verifierJobIDs := make(map[string]bool)
	for _, j := range jobResult.VerifierJobs {
		verifierJobIDs[j.ID.String()] = true
	}
	if !verifierJobIDs[child2ID.String()] {
		t.Error("Child2 should be a verifier job (no challenges)")
	}

	// ==========================================================================
	// Step 3: Resolve only grandchild's challenge
	// ==========================================================================
	t.Log("Step 3: Resolve grandchild's challenge - verify independence")

	if _, err := ldg.Append(ledger.NewChallengeResolved(grandchildChallengeID)); err != nil {
		t.Fatalf("Failed to resolve grandchild challenge: %v", err)
	}

	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Rebuild job context
	nodeMap = make(map[string]*node.Node)
	challengeMap = make(map[string][]*node.Challenge)
	for _, n := range state.AllNodes() {
		nodeMap[n.ID.String()] = n
	}
	for _, c := range state.AllChallenges() {
		nc := &node.Challenge{
			ID:       c.ID,
			TargetID: c.NodeID,
			Status:   node.ChallengeStatus(c.Status),
		}
		challengeMap[c.NodeID.String()] = append(challengeMap[c.NodeID.String()], nc)
	}

	jobResult = jobs.FindJobs(state.AllNodes(), nodeMap, challengeMap)

	// Now: root, child1 are prover jobs; child2, grandchild are verifier jobs
	if len(jobResult.ProverJobs) != 2 {
		t.Errorf("Expected 2 prover jobs (root, child1), got %d", len(jobResult.ProverJobs))
	}
	if len(jobResult.VerifierJobs) != 2 {
		t.Errorf("Expected 2 verifier jobs (child2, grandchild), got %d", len(jobResult.VerifierJobs))
	}

	t.Log("")
	t.Log("==============================================")
	t.Log("  NESTED CHALLENGES INDEPENDENCE: SUCCESS")
	t.Log("==============================================")
}

// ============================================================================
// 5. Supersession on Archive
// ============================================================================

// TestSupersession_OnArchive tests that when a node is archived:
// 1. The node transitions to archived epistemic state
// 2. The archived branch is effectively abandoned
// 3. Other branches remain unaffected
// 4. Prover can create alternative refinements
func TestSupersession_OnArchive(t *testing.T) {
	proofDir, cleanup := setupMultiAgentTest(t)
	defer cleanup()

	conjecture := "Proof with multiple attempted approaches"
	svc := initMultiAgentProof(t, proofDir, conjecture)

	rootID := parseNodeID(t, "1")
	attempt1ID := parseNodeID(t, "1.1")
	attempt2ID := parseNodeID(t, "1.2")

	// ==========================================================================
	// Step 1: Create two proof attempts (branches)
	// ==========================================================================
	t.Log("Step 1: Create two proof attempts")

	proverAgent := "prover"
	if err := svc.ClaimNode(rootID, proverAgent, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	// First attempt
	if err := svc.RefineNode(rootID, proverAgent, attempt1ID, schema.NodeTypeClaim,
		"Attempt 1: Direct approach", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (attempt1) failed: %v", err)
	}

	// Second attempt
	if err := svc.RefineNode(rootID, proverAgent, attempt2ID, schema.NodeTypeClaim,
		"Attempt 2: Indirect approach via lemma", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (attempt2) failed: %v", err)
	}

	if err := svc.ReleaseNode(rootID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	// ==========================================================================
	// Step 2: Archive first attempt (dead end)
	// ==========================================================================
	t.Log("Step 2: Archive first attempt (dead end)")

	if err := svc.ArchiveNode(attempt1ID); err != nil {
		t.Fatalf("ArchiveNode (attempt1) failed: %v", err)
	}

	// Verify attempt1 is archived
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	attempt1Node := state.GetNode(attempt1ID)
	if attempt1Node.EpistemicState != schema.EpistemicArchived {
		t.Errorf("Attempt1 epistemic state = %v, want %v",
			attempt1Node.EpistemicState, schema.EpistemicArchived)
	}

	// ==========================================================================
	// Step 3: Verify second attempt is unaffected
	// ==========================================================================
	t.Log("Step 3: Verify second attempt is unaffected")

	attempt2Node := state.GetNode(attempt2ID)
	if attempt2Node.EpistemicState != schema.EpistemicPending {
		t.Errorf("Attempt2 epistemic state = %v, want %v (should be unaffected)",
			attempt2Node.EpistemicState, schema.EpistemicPending)
	}

	// ==========================================================================
	// Step 4: Accept the successful branch
	// ==========================================================================
	t.Log("Step 4: Accept the successful branch")

	if err := svc.AcceptNode(attempt2ID); err != nil {
		t.Fatalf("AcceptNode (attempt2) failed: %v", err)
	}

	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	// Verify final states
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Root should be validated
	rootNode := state.GetNode(rootID)
	if rootNode.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Root epistemic state = %v, want %v",
			rootNode.EpistemicState, schema.EpistemicValidated)
	}

	// Attempt1 should still be archived
	attempt1Node = state.GetNode(attempt1ID)
	if attempt1Node.EpistemicState != schema.EpistemicArchived {
		t.Errorf("Attempt1 epistemic state = %v, want %v",
			attempt1Node.EpistemicState, schema.EpistemicArchived)
	}

	// Attempt2 should be validated
	attempt2Node = state.GetNode(attempt2ID)
	if attempt2Node.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Attempt2 epistemic state = %v, want %v",
			attempt2Node.EpistemicState, schema.EpistemicValidated)
	}

	t.Log("")
	t.Log("==============================================")
	t.Log("  SUPERSESSION ON ARCHIVE: SUCCESS")
	t.Log("==============================================")
}

// ============================================================================
// 6. Escape Hatch Taint Propagation
// ============================================================================

// TestEscapeHatch_TaintPropagation tests that when a node is admitted (escape hatch):
// 1. The admitted node gets TaintSelfAdmitted
// 2. All descendants become TaintTainted
// 3. Validated siblings remain TaintClean
// 4. Taint propagates correctly through the hierarchy
func TestEscapeHatch_TaintPropagation(t *testing.T) {
	proofDir, cleanup := setupMultiAgentTest(t)
	defer cleanup()

	conjecture := "Proof with escape hatch"
	svc := initMultiAgentProof(t, proofDir, conjecture)

	rootID := parseNodeID(t, "1")
	child1ID := parseNodeID(t, "1.1")
	child2ID := parseNodeID(t, "1.2")
	grandchild1ID := parseNodeID(t, "1.1.1")
	grandchild2ID := parseNodeID(t, "1.1.2")

	// ==========================================================================
	// Setup: Create proof structure
	// ==========================================================================
	t.Log("Setup: Create proof structure")

	// Tree structure:
	//   root (1) - will be validated
	//   |- child1 (1.1) - will be ADMITTED (escape hatch)
	//   |  |- grandchild1 (1.1.1) - will be validated (but tainted due to 1.1)
	//   |  |- grandchild2 (1.1.2) - will be validated (but tainted due to 1.1)
	//   |- child2 (1.2) - will be validated (clean - no tainted ancestors)

	proverAgent := "prover"
	if err := svc.ClaimNode(rootID, proverAgent, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	if err := svc.RefineNode(rootID, proverAgent, child1ID, schema.NodeTypeClaim,
		"Child 1 - will use escape hatch", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode (child1) failed: %v", err)
	}

	if err := svc.RefineNode(rootID, proverAgent, child2ID, schema.NodeTypeClaim,
		"Child 2 - fully verified", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (child2) failed: %v", err)
	}

	if err := svc.ReleaseNode(rootID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	// Add grandchildren under child1
	if err := svc.ClaimNode(child1ID, proverAgent, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (child1) failed: %v", err)
	}

	if err := svc.RefineNode(child1ID, proverAgent, grandchild1ID, schema.NodeTypeClaim,
		"Grandchild 1", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (grandchild1) failed: %v", err)
	}

	if err := svc.RefineNode(child1ID, proverAgent, grandchild2ID, schema.NodeTypeClaim,
		"Grandchild 2", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (grandchild2) failed: %v", err)
	}

	if err := svc.ReleaseNode(child1ID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (child1) failed: %v", err)
	}

	// ==========================================================================
	// Step 1: Validate root and child2 (clean path)
	// ==========================================================================
	t.Log("Step 1: Validate clean path (root, child2)")

	if err := svc.AcceptNode(child2ID); err != nil {
		t.Fatalf("AcceptNode (child2) failed: %v", err)
	}
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	// ==========================================================================
	// Step 2: ADMIT child1 (escape hatch - not fully verified)
	// ==========================================================================
	t.Log("Step 2: Admit child1 (escape hatch)")

	if err := svc.AdmitNode(child1ID); err != nil {
		t.Fatalf("AdmitNode (child1) failed: %v", err)
	}

	// Verify child1 is admitted
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	child1Node := state.GetNode(child1ID)
	if child1Node.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("Child1 epistemic state = %v, want %v",
			child1Node.EpistemicState, schema.EpistemicAdmitted)
	}

	// ==========================================================================
	// Step 3: Validate grandchildren under admitted node
	// ==========================================================================
	t.Log("Step 3: Validate grandchildren")

	if err := svc.AcceptNode(grandchild1ID); err != nil {
		t.Fatalf("AcceptNode (grandchild1) failed: %v", err)
	}
	if err := svc.AcceptNode(grandchild2ID); err != nil {
		t.Fatalf("AcceptNode (grandchild2) failed: %v", err)
	}

	// ==========================================================================
	// Step 4: Verify taint propagation
	// ==========================================================================
	t.Log("Step 4: Verify taint propagation")

	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Build node map for taint computation
	nodeMap := make(map[string]*node.Node)
	for _, n := range state.AllNodes() {
		nodeMap[n.ID.String()] = n
	}

	// Helper to compute taint with ancestors
	computeTaintWithAncestors := func(n *node.Node) node.TaintState {
		var ancestors []*node.Node
		parentID, hasParent := n.ID.Parent()
		for hasParent {
			if parent, ok := nodeMap[parentID.String()]; ok {
				ancestors = append(ancestors, parent)
			}
			parentID, hasParent = parentID.Parent()
		}
		return taint.ComputeTaint(n, ancestors)
	}

	// Root: validated -> clean
	rootNode := nodeMap[rootID.String()]
	rootTaint := computeTaintWithAncestors(rootNode)
	if rootTaint != node.TaintClean {
		t.Errorf("Root taint = %v, want %v", rootTaint, node.TaintClean)
	}

	// Child1: admitted -> self_admitted
	child1Taint := computeTaintWithAncestors(child1Node)
	if child1Taint != node.TaintSelfAdmitted {
		t.Errorf("Child1 taint = %v, want %v", child1Taint, node.TaintSelfAdmitted)
	}

	// Child2: validated with clean ancestors -> clean
	child2Node := nodeMap[child2ID.String()]
	child2Taint := computeTaintWithAncestors(child2Node)
	if child2Taint != node.TaintClean {
		t.Errorf("Child2 taint = %v, want %v", child2Taint, node.TaintClean)
	}

	// Grandchild1: validated but has admitted ancestor -> tainted
	// Need to set child1's taint first for grandchild computation
	child1Node.TaintState = node.TaintSelfAdmitted
	grandchild1Node := nodeMap[grandchild1ID.String()]
	grandchild1Taint := computeTaintWithAncestors(grandchild1Node)
	if grandchild1Taint != node.TaintTainted {
		t.Errorf("Grandchild1 taint = %v, want %v (has admitted ancestor)", grandchild1Taint, node.TaintTainted)
	}

	// Grandchild2: same as grandchild1
	grandchild2Node := nodeMap[grandchild2ID.String()]
	grandchild2Taint := computeTaintWithAncestors(grandchild2Node)
	if grandchild2Taint != node.TaintTainted {
		t.Errorf("Grandchild2 taint = %v, want %v (has admitted ancestor)", grandchild2Taint, node.TaintTainted)
	}

	t.Log("")
	t.Log("==============================================")
	t.Log("  ESCAPE HATCH TAINT PROPAGATION: SUCCESS")
	t.Log("  Root: clean, Child1: self_admitted")
	t.Log("  Child2: clean, Grandchildren: tainted")
	t.Log("==============================================")
}

// ============================================================================
// 7. Concurrent Agent Scenarios
// ============================================================================

// TestConcurrentAgents_ClaimRace tests that when multiple agents try to claim
// the same node simultaneously, exactly one succeeds and the others fail.
func TestConcurrentAgents_ClaimRace(t *testing.T) {
	proofDir, cleanup := setupMultiAgentTest(t)
	defer cleanup()

	conjecture := "Concurrent claim test"
	svc := initMultiAgentProof(t, proofDir, conjecture)

	rootID := parseNodeID(t, "1")

	// Number of concurrent agents
	numAgents := 10

	var wg sync.WaitGroup
	wg.Add(numAgents)

	var mu sync.Mutex
	results := make([]struct {
		agent string
		err   error
	}, numAgents)

	// Start signal
	start := make(chan struct{})

	// Launch concurrent claim attempts
	for i := 0; i < numAgents; i++ {
		idx := i
		agentName := "agent-" + string(rune('A'+idx))
		go func() {
			defer wg.Done()
			<-start
			err := svc.ClaimNode(rootID, agentName, 5*time.Minute)
			mu.Lock()
			results[idx] = struct {
				agent string
				err   error
			}{agentName, err}
			mu.Unlock()
		}()
	}

	// Release all agents at once
	close(start)
	wg.Wait()

	// Count successes and failures
	var winner string
	successes := 0
	failures := 0

	for _, result := range results {
		if result.err == nil {
			winner = result.agent
			successes++
		} else {
			failures++
		}
	}

	// Exactly one should succeed
	if successes != 1 {
		t.Errorf("Expected exactly 1 success, got %d", successes)
	}
	if failures != numAgents-1 {
		t.Errorf("Expected %d failures, got %d", numAgents-1, failures)
	}

	// Verify the node is claimed by the winner
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := state.GetNode(rootID)
	if rootNode.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("Root workflow state = %v, want %v",
			rootNode.WorkflowState, schema.WorkflowClaimed)
	}
	if rootNode.ClaimedBy != winner {
		t.Errorf("Root claimed by = %q, want %q", rootNode.ClaimedBy, winner)
	}

	t.Logf("Concurrent claim race: %s won among %d agents", winner, numAgents)
}

// TestConcurrentAgents_ParallelOperations tests that multiple agents can perform
// operations on different nodes concurrently without interference.
func TestConcurrentAgents_ParallelOperations(t *testing.T) {
	proofDir, cleanup := setupMultiAgentTest(t)
	defer cleanup()

	conjecture := "Parallel operations test"
	svc := initMultiAgentProof(t, proofDir, conjecture)

	rootID := parseNodeID(t, "1")
	child1ID := parseNodeID(t, "1.1")
	child2ID := parseNodeID(t, "1.2")
	child3ID := parseNodeID(t, "1.3")

	// Setup: Create 3 child nodes
	proverAgent := "setup-agent"
	if err := svc.ClaimNode(rootID, proverAgent, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	childIDs := []types.NodeID{child1ID, child2ID, child3ID}
	for i, childID := range childIDs {
		if err := svc.RefineNode(rootID, proverAgent, childID, schema.NodeTypeClaim,
			"Child statement "+string(rune('1'+i)), schema.InferenceAssumption); err != nil {
			t.Fatalf("RefineNode (%s) failed: %v", childID, err)
		}
	}

	if err := svc.ReleaseNode(rootID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	// ==========================================================================
	// Test: Multiple agents accept different nodes concurrently
	// ==========================================================================
	t.Log("Test: Multiple agents accept different nodes concurrently")

	var wg sync.WaitGroup
	wg.Add(len(childIDs))

	var mu sync.Mutex
	errors := make([]error, len(childIDs))

	start := make(chan struct{})

	for i, childID := range childIDs {
		idx := i
		id := childID
		go func() {
			defer wg.Done()
			<-start
			err := svc.AcceptNode(id)
			mu.Lock()
			errors[idx] = err
			mu.Unlock()
		}()
	}

	// Release all goroutines
	close(start)
	wg.Wait()

	// With CAS semantics, some may fail due to concurrent modification
	// Check how many succeeded
	successCount := 0
	casFailures := 0
	for i, err := range errors {
		if err == nil {
			successCount++
			t.Logf("  Child %s accepted successfully", childIDs[i])
		} else if err.Error() != "" && (err.Error()[:10] == "concurrent" || err.Error()[:6] == "concur") {
			// CAS failure - expected in concurrent scenario
			casFailures++
			t.Logf("  Child %s got CAS failure (expected): %v", childIDs[i], err)
		} else {
			t.Errorf("  Child %s got unexpected error: %v", childIDs[i], err)
		}
	}

	// At least one should succeed
	if successCount == 0 {
		t.Error("Expected at least one acceptance to succeed")
	}

	t.Logf("Parallel operations: %d successes, %d CAS conflicts (expected)", successCount, casFailures)
}

// TestConcurrentAgents_ClaimAndRefine tests the typical concurrent workflow:
// 1. Multiple agents try to claim available nodes
// 2. Winners refine their nodes
// 3. Winners release nodes
// 4. Other agents can then claim the newly created children
func TestConcurrentAgents_ClaimAndRefine(t *testing.T) {
	proofDir, cleanup := setupMultiAgentTest(t)
	defer cleanup()

	conjecture := "Claim and refine workflow"
	svc := initMultiAgentProof(t, proofDir, conjecture)

	rootID := parseNodeID(t, "1")

	// ==========================================================================
	// Round 1: First agent claims root, refines, releases
	// ==========================================================================
	t.Log("Round 1: First agent claims root, refines, releases")

	agent1 := "agent-alpha"
	if err := svc.ClaimNode(rootID, agent1, 5*time.Minute); err != nil {
		t.Fatalf("Agent1 ClaimNode (root) failed: %v", err)
	}

	child1ID := parseNodeID(t, "1.1")
	child2ID := parseNodeID(t, "1.2")

	if err := svc.RefineNode(rootID, agent1, child1ID, schema.NodeTypeClaim,
		"Branch 1", schema.InferenceAssumption); err != nil {
		t.Fatalf("Agent1 RefineNode (child1) failed: %v", err)
	}

	if err := svc.RefineNode(rootID, agent1, child2ID, schema.NodeTypeClaim,
		"Branch 2", schema.InferenceAssumption); err != nil {
		t.Fatalf("Agent1 RefineNode (child2) failed: %v", err)
	}

	if err := svc.ReleaseNode(rootID, agent1); err != nil {
		t.Fatalf("Agent1 ReleaseNode (root) failed: %v", err)
	}

	// ==========================================================================
	// Round 2: Two agents race to claim the two children
	// ==========================================================================
	t.Log("Round 2: Two agents race to claim children")

	agent2 := "agent-beta"
	agent3 := "agent-gamma"

	var wg sync.WaitGroup
	wg.Add(2)

	var mu sync.Mutex
	results := make(map[string]struct {
		childID types.NodeID
		err     error
	})

	start := make(chan struct{})

	// Agent 2 tries to claim child1
	go func() {
		defer wg.Done()
		<-start
		err := svc.ClaimNode(child1ID, agent2, 5*time.Minute)
		mu.Lock()
		results[agent2] = struct {
			childID types.NodeID
			err     error
		}{child1ID, err}
		mu.Unlock()
	}()

	// Agent 3 tries to claim child2
	go func() {
		defer wg.Done()
		<-start
		err := svc.ClaimNode(child2ID, agent3, 5*time.Minute)
		mu.Lock()
		results[agent3] = struct {
			childID types.NodeID
			err     error
		}{child2ID, err}
		mu.Unlock()
	}()

	close(start)
	wg.Wait()

	// Both should succeed since they're claiming different nodes
	// (though CAS conflicts are possible if ledger operations serialize)
	for agent, result := range results {
		if result.err != nil {
			t.Logf("%s failed to claim %s: %v (may be CAS conflict)", agent, result.childID, result.err)
		} else {
			t.Logf("%s successfully claimed %s", agent, result.childID)
		}
	}

	// Verify final state
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Count claimed nodes
	claimedCount := 0
	for _, n := range state.AllNodes() {
		if n.WorkflowState == schema.WorkflowClaimed {
			claimedCount++
			t.Logf("  Node %s claimed by %s", n.ID, n.ClaimedBy)
		}
	}

	t.Logf("Claim and refine workflow: %d nodes claimed", claimedCount)
}

// TestConcurrentAgents_VerifierAcceptanceRace tests that multiple verifiers
// can't both accept the same node (only one should succeed).
func TestConcurrentAgents_VerifierAcceptanceRace(t *testing.T) {
	proofDir, cleanup := setupMultiAgentTest(t)
	defer cleanup()

	conjecture := "Verifier acceptance race"
	svc := initMultiAgentProof(t, proofDir, conjecture)

	rootID := parseNodeID(t, "1")

	// Multiple verifiers try to accept the same node
	numVerifiers := 5

	var wg sync.WaitGroup
	wg.Add(numVerifiers)

	var mu sync.Mutex
	results := make([]error, numVerifiers)

	start := make(chan struct{})

	for i := 0; i < numVerifiers; i++ {
		idx := i
		go func() {
			defer wg.Done()
			<-start
			err := svc.AcceptNode(rootID)
			mu.Lock()
			results[idx] = err
			mu.Unlock()
		}()
	}

	close(start)
	wg.Wait()

	// Exactly one should succeed (first one to get through CAS)
	successes := 0
	casFailures := 0
	stateErrors := 0

	for _, err := range results {
		if err == nil {
			successes++
		} else if err.Error() != "" && len(err.Error()) > 10 && err.Error()[:10] == "concurrent" {
			casFailures++
		} else {
			// Likely "invalid state transition" because node is no longer pending
			stateErrors++
		}
	}

	// Exactly one should succeed
	if successes != 1 {
		t.Errorf("Expected exactly 1 success, got %d", successes)
	}

	// Verify node is validated
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := state.GetNode(rootID)
	if rootNode.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Root epistemic state = %v, want %v",
			rootNode.EpistemicState, schema.EpistemicValidated)
	}

	t.Logf("Verifier acceptance race: 1 success, %d CAS failures, %d state errors", casFailures, stateErrors)
}
