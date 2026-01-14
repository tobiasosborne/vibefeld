//go:build integration

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// setupAdversarialTest creates a temporary proof directory for adversarial workflow testing.
func setupAdversarialTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-adversarial-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// buildChallengeMapFromState converts state challenges to the format expected by jobs package.
func buildChallengeMapFromState(st *state.State) map[string][]*node.Challenge {
	challengeMap := make(map[string][]*node.Challenge)
	for _, c := range st.AllChallenges() {
		nodeIDStr := c.NodeID.String()
		nc := &node.Challenge{
			ID:       c.ID,
			TargetID: c.NodeID,
			Status:   node.ChallengeStatus(c.Status),
		}
		challengeMap[nodeIDStr] = append(challengeMap[nodeIDStr], nc)
	}
	return challengeMap
}

// findJobsFromState is a helper that builds the challenge map and finds jobs.
func findJobsFromState(st *state.State) *jobs.JobResult {
	nodes := st.AllNodes()
	nodeMap := make(map[string]*node.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID.String()] = n
	}
	challengeMap := buildChallengeMapFromState(st)
	return jobs.FindJobs(nodes, nodeMap, challengeMap)
}

// TestAdversarialWorkflow_BreadthFirstCycle tests the complete adversarial breadth-first workflow:
//
// 1. Initialize proof - new node is immediately a verifier job
// 2. Verifier claims and raises challenge - node becomes prover job
// 3. Prover claims and addresses challenge with refinement
// 4. New children are verifier jobs
// 5. Challenge resolution allows acceptance
//
// This demonstrates the core adversarial verification model where:
// - Every new node starts as verifier territory (breadth-first)
// - Verifiers attack by raising challenges
// - Provers defend by addressing challenges
// - Resolution returns control to verifiers for acceptance
func TestAdversarialWorkflow_BreadthFirstCycle(t *testing.T) {
	proofDir, cleanup := setupAdversarialTest(t)
	defer cleanup()

	conjecture := "For all n, if P(n) holds, then Q(n) follows"

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

	rootID, _ := types.Parse("1")

	// ==========================================================================
	// Step 2: Verify new node is immediately a verifier job
	// ==========================================================================
	t.Log("Step 2: Verify new node is immediately a verifier job (breadth-first)")

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult := findJobsFromState(st)

	if len(jobResult.VerifierJobs) != 1 {
		t.Errorf("Expected 1 verifier job after init, got %d", len(jobResult.VerifierJobs))
	}
	if len(jobResult.ProverJobs) != 0 {
		t.Errorf("Expected 0 prover jobs after init, got %d", len(jobResult.ProverJobs))
	}
	if len(jobResult.VerifierJobs) > 0 && jobResult.VerifierJobs[0].ID.String() != rootID.String() {
		t.Errorf("Verifier job should be root node %s, got %s", rootID, jobResult.VerifierJobs[0].ID)
	}

	t.Log("  Root node is a verifier job (no challenges yet)")

	// ==========================================================================
	// Step 3: Verifier claims and raises a challenge
	// ==========================================================================
	t.Log("Step 3: Verifier claims node and raises challenge")

	verifierOwner := "verifier-agent-001"
	if err := svc.ClaimNode(rootID, verifierOwner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (verifier) failed: %v", err)
	}

	// Release the node first (verifier doesn't keep claim, just raises challenge)
	if err := svc.ReleaseNode(rootID, verifierOwner); err != nil {
		t.Fatalf("ReleaseNode (verifier) failed: %v", err)
	}

	// Raise challenge via ledger
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	challengeID := "challenge-001"
	challengeEvent := ledger.NewChallengeRaised(challengeID, rootID, "statement", "The transition from P(n) to Q(n) needs justification")
	if _, err := ldg.Append(challengeEvent); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	t.Log("  Verifier raised challenge on root node")

	// ==========================================================================
	// Step 4: Verify it becomes a prover job
	// ==========================================================================
	t.Log("Step 4: Verify challenged node becomes a prover job")

	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult = findJobsFromState(st)

	if len(jobResult.ProverJobs) != 1 {
		t.Errorf("Expected 1 prover job after challenge, got %d", len(jobResult.ProverJobs))
	}
	if len(jobResult.VerifierJobs) != 0 {
		t.Errorf("Expected 0 verifier jobs after challenge, got %d", len(jobResult.VerifierJobs))
	}
	if len(jobResult.ProverJobs) > 0 && jobResult.ProverJobs[0].ID.String() != rootID.String() {
		t.Errorf("Prover job should be root node %s, got %s", rootID, jobResult.ProverJobs[0].ID)
	}

	t.Log("  Root node is now a prover job (has open challenge)")

	// ==========================================================================
	// Step 5: Prover claims and addresses challenge with refinement
	// ==========================================================================
	t.Log("Step 5: Prover claims node and addresses challenge with refinement")

	proverOwner := "prover-agent-001"
	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (prover) failed: %v", err)
	}

	// Prover adds children to address the challenge
	child1ID, _ := types.Parse("1.1")
	child2ID, _ := types.Parse("1.2")

	if err := svc.RefineNode(rootID, proverOwner, child1ID, schema.NodeTypeClaim,
		"Given P(n), we have the necessary precondition",
		schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode (1.1) failed: %v", err)
	}

	if err := svc.RefineNode(rootID, proverOwner, child2ID, schema.NodeTypeClaim,
		"By the established lemma L1, P(n) implies Q(n)",
		schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (1.2) failed: %v", err)
	}

	t.Log("  Prover created children 1.1 and 1.2 to address challenge")

	// ==========================================================================
	// Step 6: Verify new children are verifier jobs
	// ==========================================================================
	t.Log("Step 6: Verify new children are verifier jobs")

	// Prover releases the node after refinement
	if err := svc.ReleaseNode(rootID, proverOwner); err != nil {
		t.Fatalf("ReleaseNode (prover) failed: %v", err)
	}

	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult = findJobsFromState(st)

	// Children should be verifier jobs (new, no challenges)
	// Root should still be prover job (has unresolved challenge)
	if len(jobResult.VerifierJobs) != 2 {
		t.Errorf("Expected 2 verifier jobs (children), got %d", len(jobResult.VerifierJobs))
	}
	if len(jobResult.ProverJobs) != 1 {
		t.Errorf("Expected 1 prover job (root with challenge), got %d", len(jobResult.ProverJobs))
	}

	// Verify child IDs are among verifier jobs
	verifierJobIDs := make(map[string]bool)
	for _, j := range jobResult.VerifierJobs {
		verifierJobIDs[j.ID.String()] = true
	}

	if !verifierJobIDs[child1ID.String()] {
		t.Errorf("Child 1.1 should be a verifier job")
	}
	if !verifierJobIDs[child2ID.String()] {
		t.Errorf("Child 1.2 should be a verifier job")
	}

	t.Log("  Children 1.1 and 1.2 are verifier jobs (breadth-first)")

	// ==========================================================================
	// Step 7: Resolve challenge and verify acceptance is possible
	// ==========================================================================
	t.Log("Step 7: Resolve challenge and verify root returns to verifier territory")

	// Resolve the challenge
	resolveEvent := ledger.NewChallengeResolved(challengeID)
	if _, err := ldg.Append(resolveEvent); err != nil {
		t.Fatalf("Failed to resolve challenge: %v", err)
	}

	t.Log("  Challenge resolved")

	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult = findJobsFromState(st)

	// Now root should be verifier job too (no more open challenges)
	if len(jobResult.VerifierJobs) != 3 {
		t.Errorf("Expected 3 verifier jobs after challenge resolution, got %d", len(jobResult.VerifierJobs))
	}
	if len(jobResult.ProverJobs) != 0 {
		t.Errorf("Expected 0 prover jobs after challenge resolution, got %d", len(jobResult.ProverJobs))
	}

	t.Log("  Root node is now a verifier job again (challenge resolved)")

	// ==========================================================================
	// Step 8: Complete the proof with acceptance
	// ==========================================================================
	t.Log("Step 8: Complete proof with verifier acceptance")

	// Accept children first (leaf nodes)
	if err := svc.AcceptNode(child1ID); err != nil {
		t.Fatalf("AcceptNode (1.1) failed: %v", err)
	}
	if err := svc.AcceptNode(child2ID); err != nil {
		t.Fatalf("AcceptNode (1.2) failed: %v", err)
	}

	// Accept root
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	// Verify all nodes validated
	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	validatedCount := 0
	for _, n := range st.AllNodes() {
		if n.EpistemicState == schema.EpistemicValidated {
			validatedCount++
		}
	}

	if validatedCount != 3 {
		t.Errorf("Expected 3 validated nodes, got %d", validatedCount)
	}

	t.Log("  All 3 nodes validated - proof complete!")
	t.Log("")
	t.Log("========================================")
	t.Log("  ADVERSARIAL WORKFLOW TEST COMPLETE!")
	t.Log("  Breadth-first cycle verified:")
	t.Log("    1. New node -> verifier job")
	t.Log("    2. Challenge -> prover job")
	t.Log("    3. Refinement -> children are verifier jobs")
	t.Log("    4. Resolution -> back to verifier")
	t.Log("    5. Acceptance -> validated")
	t.Log("========================================")
}

// TestAdversarialWorkflow_MultipleChallenges tests that a node with multiple
// challenges remains a prover job until ALL challenges are resolved.
func TestAdversarialWorkflow_MultipleChallenges(t *testing.T) {
	proofDir, cleanup := setupAdversarialTest(t)
	defer cleanup()

	// Initialize proof
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}
	if err := service.Init(proofDir, "Multiple challenges test", "test-author"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	rootID, _ := types.Parse("1")

	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Raise multiple challenges
	challengeIDs := []string{"challenge-001", "challenge-002", "challenge-003"}
	targets := []string{"statement", "inference", "gap"}
	reasons := []string{
		"Statement is unclear",
		"Inference rule does not apply",
		"Logical gap in reasoning",
	}

	for i, cid := range challengeIDs {
		event := ledger.NewChallengeRaised(cid, rootID, targets[i], reasons[i])
		if _, err := ldg.Append(event); err != nil {
			t.Fatalf("Failed to raise challenge %s: %v", cid, err)
		}
	}

	// Verify node is prover job with 3 open challenges
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	openChallenges := st.OpenChallenges()
	if len(openChallenges) != 3 {
		t.Errorf("Expected 3 open challenges, got %d", len(openChallenges))
	}

	jobResult := findJobsFromState(st)
	if len(jobResult.ProverJobs) != 1 {
		t.Errorf("Expected 1 prover job with multiple challenges, got %d", len(jobResult.ProverJobs))
	}
	if len(jobResult.VerifierJobs) != 0 {
		t.Errorf("Expected 0 verifier jobs with open challenges, got %d", len(jobResult.VerifierJobs))
	}

	// Resolve challenges one by one and verify node stays prover job until all resolved
	for i, cid := range challengeIDs {
		event := ledger.NewChallengeResolved(cid)
		if _, err := ldg.Append(event); err != nil {
			t.Fatalf("Failed to resolve challenge %s: %v", cid, err)
		}

		st, err = svc.LoadState()
		if err != nil {
			t.Fatalf("LoadState failed: %v", err)
		}

		jobResult = findJobsFromState(st)

		if i < len(challengeIDs)-1 {
			// Still have open challenges
			if len(jobResult.ProverJobs) != 1 {
				t.Errorf("After resolving %d challenges: expected 1 prover job, got %d", i+1, len(jobResult.ProverJobs))
			}
			if len(jobResult.VerifierJobs) != 0 {
				t.Errorf("After resolving %d challenges: expected 0 verifier jobs, got %d", i+1, len(jobResult.VerifierJobs))
			}
		} else {
			// All challenges resolved
			if len(jobResult.ProverJobs) != 0 {
				t.Errorf("After resolving all challenges: expected 0 prover jobs, got %d", len(jobResult.ProverJobs))
			}
			if len(jobResult.VerifierJobs) != 1 {
				t.Errorf("After resolving all challenges: expected 1 verifier job, got %d", len(jobResult.VerifierJobs))
			}
		}
	}

	t.Log("Multiple challenges test passed: node stays prover job until ALL resolved")
}

// TestAdversarialWorkflow_ChallengeWithdrawal tests that withdrawn challenges
// (as opposed to resolved) also return the node to verifier territory.
func TestAdversarialWorkflow_ChallengeWithdrawal(t *testing.T) {
	proofDir, cleanup := setupAdversarialTest(t)
	defer cleanup()

	// Initialize proof
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}
	if err := service.Init(proofDir, "Challenge withdrawal test", "test-author"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	rootID, _ := types.Parse("1")

	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Raise challenge
	challengeID := "challenge-withdraw-001"
	raiseEvent := ledger.NewChallengeRaised(challengeID, rootID, "statement", "Initial concern about statement")
	if _, err := ldg.Append(raiseEvent); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	// Verify it's a prover job
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult := findJobsFromState(st)
	if len(jobResult.ProverJobs) != 1 {
		t.Errorf("Expected 1 prover job after challenge, got %d", len(jobResult.ProverJobs))
	}

	// Withdraw challenge (verifier changed their mind)
	withdrawEvent := ledger.NewChallengeWithdrawn(challengeID)
	if _, err := ldg.Append(withdrawEvent); err != nil {
		t.Fatalf("Failed to withdraw challenge: %v", err)
	}

	// Verify it returns to verifier job
	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult = findJobsFromState(st)
	if len(jobResult.ProverJobs) != 0 {
		t.Errorf("Expected 0 prover jobs after withdrawal, got %d", len(jobResult.ProverJobs))
	}
	if len(jobResult.VerifierJobs) != 1 {
		t.Errorf("Expected 1 verifier job after withdrawal, got %d", len(jobResult.VerifierJobs))
	}

	t.Log("Challenge withdrawal test passed: withdrawn challenge returns node to verifier")
}

// TestAdversarialWorkflow_NestedChallenges tests challenges on nested nodes
// (parent and children) work independently.
func TestAdversarialWorkflow_NestedChallenges(t *testing.T) {
	proofDir, cleanup := setupAdversarialTest(t)
	defer cleanup()

	// Initialize proof
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}
	if err := service.Init(proofDir, "Nested challenges test", "test-author"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	rootID, _ := types.Parse("1")
	childID, _ := types.Parse("1.1")

	// Claim root and create child
	if err := svc.ClaimNode(rootID, "prover", 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}
	if err := svc.RefineNode(rootID, "prover", childID, schema.NodeTypeClaim,
		"Child statement", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode failed: %v", err)
	}
	if err := svc.ReleaseNode(rootID, "prover"); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Both should be verifier jobs initially
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult := findJobsFromState(st)
	if len(jobResult.VerifierJobs) != 2 {
		t.Errorf("Expected 2 verifier jobs (root + child), got %d", len(jobResult.VerifierJobs))
	}

	// Challenge only the child
	childChallengeID := "child-challenge-001"
	event := ledger.NewChallengeRaised(childChallengeID, childID, "statement", "Child needs clarification")
	if _, err := ldg.Append(event); err != nil {
		t.Fatalf("Failed to raise challenge on child: %v", err)
	}

	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult = findJobsFromState(st)

	// Root should be verifier job, child should be prover job
	if len(jobResult.VerifierJobs) != 1 {
		t.Errorf("Expected 1 verifier job (root), got %d", len(jobResult.VerifierJobs))
	}
	if len(jobResult.ProverJobs) != 1 {
		t.Errorf("Expected 1 prover job (child), got %d", len(jobResult.ProverJobs))
	}

	// Verify the right nodes are in the right categories
	if len(jobResult.VerifierJobs) > 0 && jobResult.VerifierJobs[0].ID.String() != rootID.String() {
		t.Errorf("Verifier job should be root %s, got %s", rootID, jobResult.VerifierJobs[0].ID)
	}
	if len(jobResult.ProverJobs) > 0 && jobResult.ProverJobs[0].ID.String() != childID.String() {
		t.Errorf("Prover job should be child %s, got %s", childID, jobResult.ProverJobs[0].ID)
	}

	t.Log("Nested challenges test passed: challenges on different nodes are independent")
}
