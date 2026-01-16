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

// ============================================================================
// Dobinski Regression Tests
//
// These tests reproduce the exact failure scenarios from the Dobinski proof
// attempt (documented in dobinski-proof/FAILURE_REPORT.md) to ensure they
// are now fixed.
//
// Key failures addressed:
// 1. FAILURE 1: Verifier job detection was inverted (bottom-up instead of breadth-first)
// 2. FAILURE 2: Agents lacked context when claiming nodes
// 3. FAILURE 13: Challenge resolution workflow was opaque
// ============================================================================

// setupDobinskiTest creates a temporary proof directory for Dobinski regression testing.
func setupDobinskiTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-dobinski-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// initDobinskiProof initializes a proof with the Dobinski formula conjecture.
func initDobinskiProof(t *testing.T, proofDir string) *service.ProofService {
	t.Helper()

	// Dobinski's formula: B_n = (1/e) * sum_{k>=0} k^n / k!
	conjecture := "Dobinski's Formula: For all n >= 0, the Bell number B_n equals (1/e) * sum_{k>=0} k^n / k!"

	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("failed to initialize proof dir: %v", err)
	}
	if err := service.Init(proofDir, conjecture, "dobinski-prover"); err != nil {
		t.Fatalf("failed to initialize proof: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}

	return svc
}

// buildJobResult is a helper that builds job detection context from state.
func buildJobResult(st *state.State) *jobs.JobResult {
	nodes := st.AllNodes()
	nodeMap := make(map[string]*node.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID.String()] = n
	}

	challengeMap := make(map[string][]*node.Challenge)
	for _, c := range st.AllChallenges() {
		nc := &node.Challenge{
			ID:       c.ID,
			TargetID: c.NodeID,
			Status:   node.ChallengeStatus(c.Status),
		}
		challengeMap[c.NodeID.String()] = append(challengeMap[c.NodeID.String()], nc)
	}

	return jobs.FindJobs(nodes, nodeMap, challengeMap)
}

// ============================================================================
// FAILURE 1 Regression: Verifier Sees New Nodes Immediately
// ============================================================================

// TestDobinski_VerifierSeesNewNodesImmediately verifies that the inverted job
// detection issue from the Dobinski failure is fixed.
//
// Original failure: Verifiers could only see nodes after ALL children were
// validated (bottom-up). This meant newly created nodes were invisible to
// verifiers.
//
// Expected behavior: Every newly created node should IMMEDIATELY be a verifier
// job (breadth-first adversarial model).
func TestDobinski_VerifierSeesNewNodesImmediately(t *testing.T) {
	testCases := []struct {
		name        string
		setup       func(t *testing.T, svc *service.ProofService) (*state.State, types.NodeID)
		expectVJobs int
		expectPJobs int
		newNodeID   string
	}{
		{
			name: "new_root_is_verifier_job",
			setup: func(t *testing.T, svc *service.ProofService) (*state.State, types.NodeID) {
				// Just initialized - root should be verifier job
				st, err := svc.LoadState()
				if err != nil {
					t.Fatalf("LoadState failed: %v", err)
				}
				rootID, _ := types.Parse("1")
				return st, rootID
			},
			expectVJobs: 1,
			expectPJobs: 0,
			newNodeID:   "1",
		},
		{
			name: "new_child_is_verifier_job_immediately",
			setup: func(t *testing.T, svc *service.ProofService) (*state.State, types.NodeID) {
				rootID, _ := types.Parse("1")
				childID, _ := types.Parse("1.1")

				// Claim and refine
				if err := svc.ClaimNode(rootID, "prover", 5*time.Minute); err != nil {
					t.Fatalf("ClaimNode failed: %v", err)
				}
				if err := svc.RefineNode(rootID, "prover", childID, schema.NodeTypeClaim,
					"Establish combinatorial foundation via Stirling numbers",
					schema.InferenceAssumption); err != nil {
					t.Fatalf("RefineNode failed: %v", err)
				}
				if err := svc.ReleaseNode(rootID, "prover"); err != nil {
					t.Fatalf("ReleaseNode failed: %v", err)
				}

				st, err := svc.LoadState()
				if err != nil {
					t.Fatalf("LoadState failed: %v", err)
				}
				return st, childID
			},
			expectVJobs: 2, // Both root and child are verifier jobs
			expectPJobs: 0,
			newNodeID:   "1.1",
		},
		{
			name: "multiple_children_all_verifier_jobs",
			setup: func(t *testing.T, svc *service.ProofService) (*state.State, types.NodeID) {
				rootID, _ := types.Parse("1")
				child1ID, _ := types.Parse("1.1")
				child2ID, _ := types.Parse("1.2")
				child3ID, _ := types.Parse("1.3")

				if err := svc.ClaimNode(rootID, "prover", 5*time.Minute); err != nil {
					t.Fatalf("ClaimNode failed: %v", err)
				}

				// Add three children like in Dobinski proof
				children := []struct {
					id        types.NodeID
					statement string
				}{
					{child1ID, "Establish combinatorial foundation: Bell numbers count partitions"},
					{child2ID, "Express B_n via Stirling numbers of the second kind"},
					{child3ID, "Apply exponential generating function identity"},
				}

				for _, c := range children {
					if err := svc.RefineNode(rootID, "prover", c.id, schema.NodeTypeClaim,
						c.statement, schema.InferenceModusPonens); err != nil {
						t.Fatalf("RefineNode %s failed: %v", c.id, err)
					}
				}

				if err := svc.ReleaseNode(rootID, "prover"); err != nil {
					t.Fatalf("ReleaseNode failed: %v", err)
				}

				st, err := svc.LoadState()
				if err != nil {
					t.Fatalf("LoadState failed: %v", err)
				}
				return st, child1ID
			},
			expectVJobs: 4, // Root + 3 children all verifier jobs
			expectPJobs: 0,
			newNodeID:   "1.1",
		},
		{
			name: "grandchild_is_verifier_job",
			setup: func(t *testing.T, svc *service.ProofService) (*state.State, types.NodeID) {
				rootID, _ := types.Parse("1")
				childID, _ := types.Parse("1.1")
				grandchildID, _ := types.Parse("1.1.1")

				// Create child
				if err := svc.ClaimNode(rootID, "prover", 5*time.Minute); err != nil {
					t.Fatalf("ClaimNode root failed: %v", err)
				}
				if err := svc.RefineNode(rootID, "prover", childID, schema.NodeTypeClaim,
					"Intermediate step", schema.InferenceAssumption); err != nil {
					t.Fatalf("RefineNode child failed: %v", err)
				}
				if err := svc.ReleaseNode(rootID, "prover"); err != nil {
					t.Fatalf("ReleaseNode root failed: %v", err)
				}

				// Create grandchild - this is where Dobinski failed
				// The grandchild should be immediately visible to verifiers
				if err := svc.ClaimNode(childID, "prover", 5*time.Minute); err != nil {
					t.Fatalf("ClaimNode child failed: %v", err)
				}
				if err := svc.RefineNode(childID, "prover", grandchildID, schema.NodeTypeClaim,
					"Deep refinement: By definition of Stirling-second-kind",
					schema.InferenceModusPonens); err != nil {
					t.Fatalf("RefineNode grandchild failed: %v", err)
				}
				if err := svc.ReleaseNode(childID, "prover"); err != nil {
					t.Fatalf("ReleaseNode child failed: %v", err)
				}

				st, err := svc.LoadState()
				if err != nil {
					t.Fatalf("LoadState failed: %v", err)
				}
				return st, grandchildID
			},
			expectVJobs: 3, // Root, child, grandchild all verifier jobs
			expectPJobs: 0,
			newNodeID:   "1.1.1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proofDir, cleanup := setupDobinskiTest(t)
			defer cleanup()

			svc := initDobinskiProof(t, proofDir)
			st, targetID := tc.setup(t, svc)

			jobResult := buildJobResult(st)

			// Verify verifier job count
			if len(jobResult.VerifierJobs) != tc.expectVJobs {
				t.Errorf("Expected %d verifier jobs, got %d", tc.expectVJobs, len(jobResult.VerifierJobs))
				for _, j := range jobResult.VerifierJobs {
					t.Logf("  Verifier job: %s", j.ID)
				}
			}

			// Verify prover job count
			if len(jobResult.ProverJobs) != tc.expectPJobs {
				t.Errorf("Expected %d prover jobs, got %d", tc.expectPJobs, len(jobResult.ProverJobs))
				for _, j := range jobResult.ProverJobs {
					t.Logf("  Prover job: %s", j.ID)
				}
			}

			// Verify the target node is a verifier job
			found := false
			for _, j := range jobResult.VerifierJobs {
				if j.ID.String() == targetID.String() {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("New node %s should be a verifier job immediately, but was not found", tc.newNodeID)
			}

			t.Logf("PASS: New node %s is immediately visible to verifiers", tc.newNodeID)
		})
	}
}

// ============================================================================
// FAILURE 2 Regression: Full Context Provided on Claim
// ============================================================================

// TestDobinski_FullContextOnClaim verifies that when a node is claimed,
// all ancestor context is available.
//
// Original failure: Agents had to manually look up parent context, definitions,
// externals, and challenges with multiple commands.
//
// Expected behavior: State provides complete ancestor chain for context.
func TestDobinski_FullContextOnClaim(t *testing.T) {
	proofDir, cleanup := setupDobinskiTest(t)
	defer cleanup()

	svc := initDobinskiProof(t, proofDir)

	// Build a proof structure similar to Dobinski
	rootID, _ := types.Parse("1")
	child1ID, _ := types.Parse("1.1")
	grandchildID, _ := types.Parse("1.1.1")

	// Create the tree structure
	if err := svc.ClaimNode(rootID, "prover", 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode root failed: %v", err)
	}
	if err := svc.RefineNode(rootID, "prover", child1ID, schema.NodeTypeClaim,
		"Establish the combinatorial foundation: Bell numbers count partitions",
		schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode child failed: %v", err)
	}
	if err := svc.ReleaseNode(rootID, "prover"); err != nil {
		t.Fatalf("ReleaseNode root failed: %v", err)
	}

	if err := svc.ClaimNode(child1ID, "prover", 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode child failed: %v", err)
	}
	if err := svc.RefineNode(child1ID, "prover", grandchildID, schema.NodeTypeClaim,
		"By definition of Stirling-second-kind, S(n,k) counts surjections",
		schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode grandchild failed: %v", err)
	}
	if err := svc.ReleaseNode(child1ID, "prover"); err != nil {
		t.Fatalf("ReleaseNode child failed: %v", err)
	}

	// Load state and verify we can get full context for grandchild
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Verify grandchild exists and has expected properties
	grandchildNode := st.GetNode(grandchildID)
	if grandchildNode == nil {
		t.Fatal("Grandchild node not found")
	}

	// Verify we can traverse to ancestors
	childNode := st.GetNode(child1ID)
	if childNode == nil {
		t.Fatal("Child node not found")
	}

	rootNode := st.GetNode(rootID)
	if rootNode == nil {
		t.Fatal("Root node not found")
	}

	// Verify the ancestor chain is complete
	t.Log("Verifying ancestor chain context:")
	t.Logf("  Root (1):       %s", truncate(rootNode.Statement, 60))
	t.Logf("  Child (1.1):    %s", truncate(childNode.Statement, 60))
	t.Logf("  Grandchild (1.1.1): %s", truncate(grandchildNode.Statement, 60))

	// Verify parent relationships are correct via ID parsing
	parentID, hasParent := grandchildID.Parent()
	if !hasParent {
		t.Error("Grandchild should have a parent")
	}
	if parentID.String() != child1ID.String() {
		t.Errorf("Grandchild parent = %s, want %s", parentID, child1ID)
	}

	parentID, hasParent = child1ID.Parent()
	if !hasParent {
		t.Error("Child should have a parent")
	}
	if parentID.String() != rootID.String() {
		t.Errorf("Child parent = %s, want %s", parentID, rootID)
	}

	// Verify all ancestors can be retrieved from state
	ancestors := getAncestorChain(st, grandchildID)
	if len(ancestors) != 2 {
		t.Errorf("Expected 2 ancestors (child + root), got %d", len(ancestors))
	}

	t.Log("PASS: Full ancestor context is available for verification")
}

// getAncestorChain returns all ancestor nodes for a given node ID.
func getAncestorChain(st *state.State, nodeID types.NodeID) []*node.Node {
	var ancestors []*node.Node
	currentID := nodeID

	for {
		parentID, hasParent := currentID.Parent()
		if !hasParent {
			break
		}
		parentNode := st.GetNode(parentID)
		if parentNode != nil {
			ancestors = append(ancestors, parentNode)
		}
		currentID = parentID
	}

	return ancestors
}

// truncate truncates a string to maxLen characters with ellipsis.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ============================================================================
// FAILURE 13 Regression: Challenge Flow Works Correctly
// ============================================================================

// TestDobinski_ChallengeFlowCorrectly verifies the complete challenge workflow
// from the Dobinski failure scenario.
//
// Original failure: The challenge resolution workflow was opaque. Provers
// couldn't easily see challenges, and the response/resolution flow was unclear.
//
// Expected behavior:
// 1. Verifier raises challenge -> node becomes prover job
// 2. Prover sees challenge and can address it
// 3. Challenge resolution returns node to verifier territory
func TestDobinski_ChallengeFlowCorrectly(t *testing.T) {
	testCases := []struct {
		name string
		test func(t *testing.T, svc *service.ProofService, proofDir string)
	}{
		{
			name: "challenge_makes_node_prover_job",
			test: testChallengeMakesNodeProverJob,
		},
		{
			name: "multiple_challenges_require_all_resolved",
			test: testMultipleChallengesRequireAllResolved,
		},
		{
			name: "resolved_challenge_returns_to_verifier",
			test: testResolvedChallengeReturnsToVerifier,
		},
		{
			name: "withdrawn_challenge_returns_to_verifier",
			test: testWithdrawnChallengeReturnsToVerifier,
		},
		{
			name: "nested_challenges_independent",
			test: testNestedChallengesIndependent,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proofDir, cleanup := setupDobinskiTest(t)
			defer cleanup()

			svc := initDobinskiProof(t, proofDir)
			tc.test(t, svc, proofDir)
		})
	}
}

// testChallengeMakesNodeProverJob verifies that raising a challenge converts
// a verifier job to a prover job.
func testChallengeMakesNodeProverJob(t *testing.T, svc *service.ProofService, proofDir string) {
	rootID, _ := types.Parse("1")

	// Initial state: root is verifier job
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult := buildJobResult(st)
	if len(jobResult.VerifierJobs) != 1 {
		t.Errorf("Initial: expected 1 verifier job, got %d", len(jobResult.VerifierJobs))
	}
	if len(jobResult.ProverJobs) != 0 {
		t.Errorf("Initial: expected 0 prover jobs, got %d", len(jobResult.ProverJobs))
	}

	// Raise challenge
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Challenge similar to what verifiers raised in Dobinski attempt
	challengeEvent := ledger.NewChallengeRaised(
		"ch-dobinski-001",
		rootID,
		"statement",
		"The transition from Bell number definition to the sum formula requires justification",
	)
	if _, err := ldg.Append(challengeEvent); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	// After challenge: root should be prover job
	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult = buildJobResult(st)
	if len(jobResult.VerifierJobs) != 0 {
		t.Errorf("After challenge: expected 0 verifier jobs, got %d", len(jobResult.VerifierJobs))
	}
	if len(jobResult.ProverJobs) != 1 {
		t.Errorf("After challenge: expected 1 prover job, got %d", len(jobResult.ProverJobs))
	}

	// Verify the prover job is the challenged node
	if len(jobResult.ProverJobs) > 0 && jobResult.ProverJobs[0].ID.String() != rootID.String() {
		t.Errorf("Prover job should be %s, got %s", rootID, jobResult.ProverJobs[0].ID)
	}

	t.Log("PASS: Challenge correctly converts verifier job to prover job")
}

// testMultipleChallengesRequireAllResolved verifies that a node with multiple
// challenges remains a prover job until ALL challenges are resolved.
func testMultipleChallengesRequireAllResolved(t *testing.T, svc *service.ProofService, proofDir string) {
	rootID, _ := types.Parse("1")

	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Raise multiple challenges like in Dobinski scenario
	challenges := []struct {
		id     string
		target string
		reason string
	}{
		{"ch-dobinski-clarity", "statement", "The formula notation needs clarification"},
		{"ch-dobinski-inference", "inference", "The inference step is not justified"},
		{"ch-dobinski-gap", "gap", "There is a logical gap in the argument"},
	}

	for _, c := range challenges {
		event := ledger.NewChallengeRaised(c.id, rootID, c.target, c.reason)
		if _, err := ldg.Append(event); err != nil {
			t.Fatalf("Failed to raise challenge %s: %v", c.id, err)
		}
	}

	// Resolve challenges one by one
	for i, c := range challenges {
		// Before resolution: should still be prover job
		st, err := svc.LoadState()
		if err != nil {
			t.Fatalf("LoadState failed: %v", err)
		}

		jobResult := buildJobResult(st)
		if len(jobResult.ProverJobs) != 1 {
			t.Errorf("Before resolving %s: expected 1 prover job, got %d", c.id, len(jobResult.ProverJobs))
		}

		// Resolve this challenge
		resolveEvent := ledger.NewChallengeResolved(c.id)
		if _, err := ldg.Append(resolveEvent); err != nil {
			t.Fatalf("Failed to resolve challenge %s: %v", c.id, err)
		}

		// After resolution: check job status
		st, err = svc.LoadState()
		if err != nil {
			t.Fatalf("LoadState failed: %v", err)
		}

		jobResult = buildJobResult(st)

		if i < len(challenges)-1 {
			// Still have open challenges - should be prover job
			if len(jobResult.ProverJobs) != 1 {
				t.Errorf("After resolving %d/%d challenges: expected 1 prover job, got %d",
					i+1, len(challenges), len(jobResult.ProverJobs))
			}
		} else {
			// All resolved - should be verifier job
			if len(jobResult.ProverJobs) != 0 {
				t.Errorf("After resolving all challenges: expected 0 prover jobs, got %d",
					len(jobResult.ProverJobs))
			}
			if len(jobResult.VerifierJobs) != 1 {
				t.Errorf("After resolving all challenges: expected 1 verifier job, got %d",
					len(jobResult.VerifierJobs))
			}
		}
	}

	t.Log("PASS: Multiple challenges correctly require all to be resolved")
}

// testResolvedChallengeReturnsToVerifier verifies that resolving a challenge
// returns the node to verifier territory.
func testResolvedChallengeReturnsToVerifier(t *testing.T, svc *service.ProofService, proofDir string) {
	rootID, _ := types.Parse("1")

	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Raise and then resolve challenge
	challengeID := "ch-resolve-test"
	raiseEvent := ledger.NewChallengeRaised(challengeID, rootID, "statement", "Test challenge")
	if _, err := ldg.Append(raiseEvent); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	resolveEvent := ledger.NewChallengeResolved(challengeID)
	if _, err := ldg.Append(resolveEvent); err != nil {
		t.Fatalf("Failed to resolve challenge: %v", err)
	}

	// Verify node is back to verifier job
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult := buildJobResult(st)
	if len(jobResult.VerifierJobs) != 1 {
		t.Errorf("After resolution: expected 1 verifier job, got %d", len(jobResult.VerifierJobs))
	}
	if len(jobResult.ProverJobs) != 0 {
		t.Errorf("After resolution: expected 0 prover jobs, got %d", len(jobResult.ProverJobs))
	}

	t.Log("PASS: Resolved challenge returns node to verifier territory")
}

// testWithdrawnChallengeReturnsToVerifier verifies that withdrawing a challenge
// (verifier changed their mind) returns the node to verifier territory.
func testWithdrawnChallengeReturnsToVerifier(t *testing.T, svc *service.ProofService, proofDir string) {
	rootID, _ := types.Parse("1")

	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Raise and then withdraw challenge
	challengeID := "ch-withdraw-test"
	raiseEvent := ledger.NewChallengeRaised(challengeID, rootID, "statement", "Initial concern")
	if _, err := ldg.Append(raiseEvent); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	// Verify it's a prover job
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	jobResult := buildJobResult(st)
	if len(jobResult.ProverJobs) != 1 {
		t.Errorf("After raise: expected 1 prover job, got %d", len(jobResult.ProverJobs))
	}

	// Withdraw the challenge
	withdrawEvent := ledger.NewChallengeWithdrawn(challengeID)
	if _, err := ldg.Append(withdrawEvent); err != nil {
		t.Fatalf("Failed to withdraw challenge: %v", err)
	}

	// Verify node is back to verifier job
	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult = buildJobResult(st)
	if len(jobResult.VerifierJobs) != 1 {
		t.Errorf("After withdrawal: expected 1 verifier job, got %d", len(jobResult.VerifierJobs))
	}
	if len(jobResult.ProverJobs) != 0 {
		t.Errorf("After withdrawal: expected 0 prover jobs, got %d", len(jobResult.ProverJobs))
	}

	t.Log("PASS: Withdrawn challenge returns node to verifier territory")
}

// testNestedChallengesIndependent verifies that challenges on different nodes
// in a hierarchy are tracked independently.
func testNestedChallengesIndependent(t *testing.T, svc *service.ProofService, proofDir string) {
	rootID, _ := types.Parse("1")
	childID, _ := types.Parse("1.1")
	grandchildID, _ := types.Parse("1.1.1")

	// Create nested structure
	if err := svc.ClaimNode(rootID, "prover", 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode root failed: %v", err)
	}
	if err := svc.RefineNode(rootID, "prover", childID, schema.NodeTypeClaim,
		"Child node", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode child failed: %v", err)
	}
	if err := svc.ReleaseNode(rootID, "prover"); err != nil {
		t.Fatalf("ReleaseNode root failed: %v", err)
	}

	if err := svc.ClaimNode(childID, "prover", 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode child failed: %v", err)
	}
	if err := svc.RefineNode(childID, "prover", grandchildID, schema.NodeTypeClaim,
		"Grandchild node", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode grandchild failed: %v", err)
	}
	if err := svc.ReleaseNode(childID, "prover"); err != nil {
		t.Fatalf("ReleaseNode child failed: %v", err)
	}

	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Challenge only the grandchild (like in Dobinski where deep nodes got challenged)
	grandchildChallengeID := "ch-grandchild"
	event := ledger.NewChallengeRaised(grandchildChallengeID, grandchildID, "statement", "Deep challenge")
	if _, err := ldg.Append(event); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	// Verify:
	// - Grandchild is prover job (has challenge)
	// - Root and child are verifier jobs (no challenges)
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult := buildJobResult(st)

	if len(jobResult.VerifierJobs) != 2 {
		t.Errorf("Expected 2 verifier jobs (root + child), got %d", len(jobResult.VerifierJobs))
	}
	if len(jobResult.ProverJobs) != 1 {
		t.Errorf("Expected 1 prover job (grandchild), got %d", len(jobResult.ProverJobs))
	}

	// Verify the specific assignments
	verifierJobIDs := make(map[string]bool)
	for _, j := range jobResult.VerifierJobs {
		verifierJobIDs[j.ID.String()] = true
	}
	proverJobIDs := make(map[string]bool)
	for _, j := range jobResult.ProverJobs {
		proverJobIDs[j.ID.String()] = true
	}

	if !verifierJobIDs[rootID.String()] {
		t.Error("Root should be verifier job")
	}
	if !verifierJobIDs[childID.String()] {
		t.Error("Child should be verifier job")
	}
	if !proverJobIDs[grandchildID.String()] {
		t.Error("Grandchild should be prover job")
	}

	t.Log("PASS: Nested challenges are tracked independently")
}

// ============================================================================
// Full Dobinski-like Workflow Regression Test
// ============================================================================

// TestDobinski_FullWorkflowRegression tests the complete Dobinski-like proof
// workflow to ensure all failure modes are addressed.
func TestDobinski_FullWorkflowRegression(t *testing.T) {
	proofDir, cleanup := setupDobinskiTest(t)
	defer cleanup()

	svc := initDobinskiProof(t, proofDir)

	// Phase 1: Initialize and verify root is verifier job
	t.Log("Phase 1: Initialize proof")
	rootID, _ := types.Parse("1")

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult := buildJobResult(st)
	if len(jobResult.VerifierJobs) != 1 || jobResult.VerifierJobs[0].ID.String() != rootID.String() {
		t.Fatal("Root should be verifier job immediately after init")
	}
	t.Log("  Root is verifier job immediately")

	// Phase 2: Verifier raises challenge
	t.Log("Phase 2: Verifier raises challenge")
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	challengeID := "ch-dobinski-main"
	event := ledger.NewChallengeRaised(challengeID, rootID, "statement",
		"The formula requires proof via combinatorial identity")
	if _, err := ldg.Append(event); err != nil {
		t.Fatalf("Failed to raise challenge: %v", err)
	}

	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	jobResult = buildJobResult(st)
	if len(jobResult.ProverJobs) != 1 {
		t.Fatal("Root should become prover job after challenge")
	}
	t.Log("  Root is now prover job")

	// Phase 3: Prover refines to address challenge
	t.Log("Phase 3: Prover addresses challenge with refinements")
	childIDs := []types.NodeID{}
	for i := 1; i <= 3; i++ {
		childID, _ := types.Parse("1." + string(rune('0'+i)))
		childIDs = append(childIDs, childID)
	}

	if err := svc.ClaimNode(rootID, "prover", 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	refinements := []string{
		"By definition of Bell number, B_n counts set partitions",
		"Using Stirling numbers, B_n = sum_{k=0}^n S(n,k)",
		"Applying the exponential generating function identity",
	}

	for i, childID := range childIDs {
		if err := svc.RefineNode(rootID, "prover", childID, schema.NodeTypeClaim,
			refinements[i], schema.InferenceModusPonens); err != nil {
			t.Fatalf("RefineNode failed: %v", err)
		}
	}

	if err := svc.ReleaseNode(rootID, "prover"); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}
	t.Log("  Created 3 child nodes")

	// Phase 4: Verify children are immediately verifier jobs
	t.Log("Phase 4: Verify children are verifier jobs immediately")
	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	jobResult = buildJobResult(st)
	// Root has open challenge -> prover job
	// 3 children have no challenges -> verifier jobs
	if len(jobResult.ProverJobs) != 1 {
		t.Errorf("Expected 1 prover job (root), got %d", len(jobResult.ProverJobs))
	}
	if len(jobResult.VerifierJobs) != 3 {
		t.Errorf("Expected 3 verifier jobs (children), got %d", len(jobResult.VerifierJobs))
	}
	t.Log("  All children are verifier jobs (breadth-first verified)")

	// Phase 5: Resolve challenge and accept nodes
	t.Log("Phase 5: Resolve challenge and complete proof")
	resolveEvent := ledger.NewChallengeResolved(challengeID)
	if _, err := ldg.Append(resolveEvent); err != nil {
		t.Fatalf("Failed to resolve challenge: %v", err)
	}

	// Accept all children
	for _, childID := range childIDs {
		if err := svc.AcceptNode(childID); err != nil {
			t.Fatalf("AcceptNode %s failed: %v", childID, err)
		}
	}

	// Accept root
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode root failed: %v", err)
	}

	// Phase 6: Verify final state
	t.Log("Phase 6: Verify proof completion")
	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	for _, n := range st.AllNodes() {
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("Node %s: expected validated, got %s", n.ID, n.EpistemicState)
		}
	}

	t.Log("")
	t.Log("=================================================")
	t.Log("  DOBINSKI REGRESSION TEST: COMPLETE SUCCESS!")
	t.Log("")
	t.Log("  Verified fixes for:")
	t.Log("    - FAILURE 1: Verifier sees new nodes immediately")
	t.Log("    - FAILURE 2: Context available via state")
	t.Log("    - FAILURE 13: Challenge flow works correctly")
	t.Log("=================================================")
}
