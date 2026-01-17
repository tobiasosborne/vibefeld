//go:build integration

package e2e

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// setupAcceptanceBlockingTest creates a temporary proof directory for testing.
func setupAcceptanceBlockingTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-acceptance-blocking-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// TestBlockingChallenge_PreventsAcceptance verifies that blocking challenges
// (critical or major severity) prevent node acceptance.
//
// This is the key test for Session 53's finding: AcceptNode must fail when
// there are unresolved blocking challenges on the node.
//
// Test scenario:
// 1. Initialize proof with a root node
// 2. Raise a blocking challenge (critical severity) on the root node
// 3. Attempt to accept the node - should fail with ErrBlockingChallenges
// 4. Verify SeverityBlocksAcceptance() is correctly used
// 5. Resolve the challenge
// 6. Accept the node - should succeed
func TestBlockingChallenge_PreventsAcceptance(t *testing.T) {
	proofDir, cleanup := setupAcceptanceBlockingTest(t)
	defer cleanup()

	conjecture := "For all n, if P(n) then Q(n)"

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

	// Verify root node was created
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := state.GetNode(rootID)
	if rootNode == nil {
		t.Fatal("Root node 1 was not created")
	}

	t.Logf("  Root node created: %s", rootNode.Statement)

	// ==========================================================================
	// Step 2: Raise a blocking challenge (critical severity)
	// ==========================================================================
	t.Log("Step 2: Raise a critical (blocking) challenge on root node")

	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	challengeID := "blocking-challenge-001"
	// Use NewChallengeRaisedWithSeverity to set critical severity
	challengeEvent := ledger.NewChallengeRaisedWithSeverity(
		challengeID,
		rootID,
		"statement",
		"The implication P(n) -> Q(n) needs justification",
		"critical", // Critical severity blocks acceptance
		"verifier-agent-001",
	)
	if _, err := ldg.Append(challengeEvent); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	t.Log("  Critical challenge raised on root node")

	// Verify challenge is open and blocking
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	blockingChallenges := state.GetBlockingChallengesForNode(rootID)
	if len(blockingChallenges) != 1 {
		t.Fatalf("Expected 1 blocking challenge, got %d", len(blockingChallenges))
	}

	// Verify SeverityBlocksAcceptance returns true for critical
	if !schema.SeverityBlocksAcceptance(schema.SeverityCritical) {
		t.Error("SeverityBlocksAcceptance(critical) should return true")
	}

	t.Log("  Verified: critical severity blocks acceptance")

	// ==========================================================================
	// Step 3: Attempt to accept node - should fail
	// ==========================================================================
	t.Log("Step 3: Attempt to accept node with blocking challenge (should fail)")

	err = svc.AcceptNode(rootID)
	if err == nil {
		t.Fatal("AcceptNode should have failed due to blocking challenge")
	}

	// Verify it's the correct error type
	if !errors.Is(err, service.ErrBlockingChallenges) {
		t.Errorf("Expected ErrBlockingChallenges, got: %v", err)
	}

	t.Logf("  AcceptNode correctly failed with: %v", err)

	// ==========================================================================
	// Step 4: Resolve the challenge
	// ==========================================================================
	t.Log("Step 4: Resolve the blocking challenge")

	resolveEvent := ledger.NewChallengeResolved(challengeID)
	if _, err := ldg.Append(resolveEvent); err != nil {
		t.Fatalf("Failed to resolve challenge: %v", err)
	}

	// Verify no more blocking challenges
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	blockingChallenges = state.GetBlockingChallengesForNode(rootID)
	if len(blockingChallenges) != 0 {
		t.Fatalf("Expected 0 blocking challenges after resolution, got %d", len(blockingChallenges))
	}

	t.Log("  Challenge resolved, no blocking challenges remain")

	// ==========================================================================
	// Step 5: Accept node - should succeed
	// ==========================================================================
	t.Log("Step 5: Accept node after challenge resolution (should succeed)")

	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode failed after challenge resolution: %v", err)
	}

	// Verify node is now validated
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode = state.GetNode(rootID)
	if rootNode.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Root node epistemic state = %q, want %q",
			rootNode.EpistemicState, schema.EpistemicValidated)
	}

	t.Log("  Root node successfully validated")
	t.Log("")
	t.Log("========================================")
	t.Log("  BLOCKING CHALLENGE TEST PASSED!")
	t.Log("  - Critical challenge blocked acceptance")
	t.Log("  - Resolution allowed acceptance")
	t.Log("========================================")
}

// TestMajorChallenge_PreventsAcceptance verifies that major severity challenges
// also block node acceptance.
func TestMajorChallenge_PreventsAcceptance(t *testing.T) {
	proofDir, cleanup := setupAcceptanceBlockingTest(t)
	defer cleanup()

	// Initialize proof
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	if err := service.Init(proofDir, "Major challenge test", "test-author"); err != nil {
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

	// Raise major challenge
	challengeID := "major-challenge-001"
	challengeEvent := ledger.NewChallengeRaisedWithSeverity(
		challengeID,
		rootID,
		"inference",
		"Inference rule needs clarification",
		"major", // Major severity also blocks acceptance
		"verifier-agent-002",
	)
	if _, err := ldg.Append(challengeEvent); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	// Verify SeverityBlocksAcceptance returns true for major
	if !schema.SeverityBlocksAcceptance(schema.SeverityMajor) {
		t.Error("SeverityBlocksAcceptance(major) should return true")
	}

	// Attempt to accept - should fail
	err = svc.AcceptNode(rootID)
	if err == nil {
		t.Fatal("AcceptNode should have failed due to major challenge")
	}

	if !errors.Is(err, service.ErrBlockingChallenges) {
		t.Errorf("Expected ErrBlockingChallenges, got: %v", err)
	}

	t.Log("Major severity challenge correctly blocked acceptance")
}

// TestMinorChallenge_DoesNotBlockAcceptance verifies that minor severity challenges
// do NOT block node acceptance (partial acceptance is allowed).
func TestMinorChallenge_DoesNotBlockAcceptance(t *testing.T) {
	proofDir, cleanup := setupAcceptanceBlockingTest(t)
	defer cleanup()

	// Initialize proof
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	if err := service.Init(proofDir, "Minor challenge test", "test-author"); err != nil {
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

	// Raise minor challenge
	challengeEvent := ledger.NewChallengeRaisedWithSeverity(
		"minor-challenge-001",
		rootID,
		"style",
		"Could be more elegantly phrased",
		"minor", // Minor severity does NOT block acceptance
		"verifier-agent-003",
	)
	if _, err := ldg.Append(challengeEvent); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	// Verify SeverityBlocksAcceptance returns false for minor
	if schema.SeverityBlocksAcceptance(schema.SeverityMinor) {
		t.Error("SeverityBlocksAcceptance(minor) should return false")
	}

	// Attempt to accept - should succeed despite minor challenge
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode should succeed with minor challenge, but failed: %v", err)
	}

	// Verify node is validated
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := state.GetNode(rootID)
	if rootNode.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Root node should be validated even with minor challenge")
	}

	t.Log("Minor severity challenge correctly did NOT block acceptance")
}

// TestChallengeWithdrawal_AllowsAcceptance verifies that withdrawing a blocking
// challenge allows the node to be accepted.
func TestChallengeWithdrawal_AllowsAcceptance(t *testing.T) {
	proofDir, cleanup := setupAcceptanceBlockingTest(t)
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

	// Raise critical challenge
	challengeID := "withdraw-challenge-001"
	challengeEvent := ledger.NewChallengeRaisedWithSeverity(
		challengeID,
		rootID,
		"statement",
		"Initial concern",
		"critical",
		"verifier-agent-004",
	)
	if _, err := ldg.Append(challengeEvent); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	// Verify acceptance is blocked
	err = svc.AcceptNode(rootID)
	if !errors.Is(err, service.ErrBlockingChallenges) {
		t.Fatalf("Expected ErrBlockingChallenges, got: %v", err)
	}

	// Withdraw the challenge (verifier changed their mind)
	withdrawEvent := ledger.NewChallengeWithdrawn(challengeID)
	if _, err := ldg.Append(withdrawEvent); err != nil {
		t.Fatalf("Failed to withdraw challenge: %v", err)
	}

	// Now acceptance should succeed
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode should succeed after withdrawal: %v", err)
	}

	// Verify node is validated
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := state.GetNode(rootID)
	if rootNode.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Root node should be validated after challenge withdrawal")
	}

	t.Log("Challenge withdrawal correctly allowed acceptance")
}

// TestMultipleBlockingChallenges_AllMustBeResolved verifies that ALL blocking
// challenges must be resolved before a node can be accepted.
func TestMultipleBlockingChallenges_AllMustBeResolved(t *testing.T) {
	proofDir, cleanup := setupAcceptanceBlockingTest(t)
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

	// Raise multiple blocking challenges
	challengeIDs := []string{"multi-001", "multi-002"}
	for i, cid := range challengeIDs {
		severity := "critical"
		if i == 1 {
			severity = "major"
		}
		challengeEvent := ledger.NewChallengeRaisedWithSeverity(
			cid,
			rootID,
			"statement",
			"Issue "+string(rune('A'+i)),
			severity,
			"verifier-agent",
		)
		if _, err := ldg.Append(challengeEvent); err != nil {
			t.Fatalf("Failed to raise challenge %s: %v", cid, err)
		}
	}

	// Verify acceptance is blocked
	err = svc.AcceptNode(rootID)
	if !errors.Is(err, service.ErrBlockingChallenges) {
		t.Fatalf("Expected ErrBlockingChallenges with multiple challenges, got: %v", err)
	}

	// Resolve first challenge
	if _, err := ldg.Append(ledger.NewChallengeResolved(challengeIDs[0])); err != nil {
		t.Fatalf("Failed to resolve first challenge: %v", err)
	}

	// Still blocked by second challenge
	err = svc.AcceptNode(rootID)
	if !errors.Is(err, service.ErrBlockingChallenges) {
		t.Fatalf("Should still be blocked with one remaining challenge, got: %v", err)
	}

	// Resolve second challenge
	if _, err := ldg.Append(ledger.NewChallengeResolved(challengeIDs[1])); err != nil {
		t.Fatalf("Failed to resolve second challenge: %v", err)
	}

	// Now acceptance should succeed
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode should succeed after all challenges resolved: %v", err)
	}

	t.Log("Multiple blocking challenges correctly required all to be resolved")
}

// TestBlockingChallenge_FullWorkflow tests the complete adversarial workflow:
// prover creates node, verifier challenges, prover refines, verifier resolves and accepts.
func TestBlockingChallenge_FullWorkflow(t *testing.T) {
	proofDir, cleanup := setupAcceptanceBlockingTest(t)
	defer cleanup()

	// Initialize proof
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	if err := service.Init(proofDir, "Full workflow test", "test-author"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	rootID, _ := types.Parse("1")
	proverOwner := "prover-agent-001"
	verifierOwner := "verifier-agent-001"

	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	t.Log("Step 1: Verifier raises blocking challenge")

	challengeID := "workflow-challenge-001"
	challengeEvent := ledger.NewChallengeRaisedWithSeverity(
		challengeID,
		rootID,
		"statement",
		"Needs more detail",
		"critical",
		verifierOwner,
	)
	if _, err := ldg.Append(challengeEvent); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	t.Log("Step 2: Prover claims node and refines it")

	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	childID, _ := types.Parse("1.1")
	if err := svc.RefineNode(rootID, proverOwner, childID, schema.NodeTypeClaim,
		"Supporting argument to address the challenge",
		schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode failed: %v", err)
	}

	if err := svc.ReleaseNode(rootID, proverOwner); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	t.Log("Step 3: Verifier sees refinement, resolves challenge")

	resolveEvent := ledger.NewChallengeResolved(challengeID)
	if _, err := ldg.Append(resolveEvent); err != nil {
		t.Fatalf("Failed to resolve challenge: %v", err)
	}

	t.Log("Step 4: Verifier accepts child first, then root")

	if err := svc.AcceptNode(childID); err != nil {
		t.Fatalf("AcceptNode (child) failed: %v", err)
	}

	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	t.Log("Step 5: Verify all nodes validated")

	state, err := svc.LoadState()
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
	t.Log("  FULL WORKFLOW TEST PASSED!")
	t.Log("  - Challenge raised")
	t.Log("  - Prover refined")
	t.Log("  - Challenge resolved")
	t.Log("  - All nodes validated")
	t.Log("========================================")
}
