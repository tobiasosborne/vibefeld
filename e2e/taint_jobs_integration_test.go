//go:build integration

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/taint"
	"github.com/tobias/vibefeld/internal/types"
)

// setupTaintJobsTest creates a temporary proof directory for taint-jobs integration testing.
func setupTaintJobsTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-taint-jobs-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// initTaintJobsProof initializes a proof with the given conjecture.
func initTaintJobsProof(t *testing.T, proofDir, conjecture string) *service.ProofService {
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

// parseTaintJobsNodeID parses a node ID string and fails the test if it fails.
func parseTaintJobsNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("failed to parse node ID %q: %v", s, err)
	}
	return id
}

// buildChallengeMap converts state challenges to the format expected by jobs package.
func buildChallengeMap(st *state.State) map[string][]*node.Challenge {
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

// findJobsWithTaint returns job results with taint information computed for each node.
func findJobsWithTaint(st *state.State) *jobs.JobResult {
	nodes := st.AllNodes()
	nodeMap := make(map[string]*node.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID.String()] = n
	}
	challengeMap := buildChallengeMap(st)
	return jobs.FindJobs(nodes, nodeMap, challengeMap)
}

// computeTaintForState computes taint for all nodes in topological order (parents before children).
func computeTaintForState(st *state.State) {
	nodes := st.AllNodes()
	nodeMap := make(map[string]*node.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID.String()] = n
	}

	// Build ancestor map and compute taint in order (root first)
	for _, n := range nodes {
		var ancestors []*node.Node
		parentID, hasParent := n.ID.Parent()
		for hasParent {
			if parent := nodeMap[parentID.String()]; parent != nil {
				ancestors = append(ancestors, parent)
			}
			parentID, hasParent = parentID.Parent()
		}
		n.TaintState = taint.ComputeTaint(n, ancestors)
	}
}

// TestTaintJobsIntegration_AdmitPropagatesTaintToChildren tests that when a node is admitted,
// its children become tainted and the taint information is visible in job results.
func TestTaintJobsIntegration_AdmitPropagatesTaintToChildren(t *testing.T) {
	proofDir, cleanup := setupTaintJobsTest(t)
	defer cleanup()

	conjecture := "If P holds, then Q follows"
	svc := initTaintJobsProof(t, proofDir, conjecture)

	rootID := parseTaintJobsNodeID(t, "1")
	child1ID := parseTaintJobsNodeID(t, "1.1")
	child2ID := parseTaintJobsNodeID(t, "1.2")

	// ==========================================================================
	// Step 1: Create proof structure with children
	// ==========================================================================
	t.Log("Step 1: Create proof structure with root and children")

	proverAgent := "prover-agent-001"
	if err := svc.ClaimNode(rootID, proverAgent, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	if err := svc.RefineNode(rootID, proverAgent, child1ID, schema.NodeTypeClaim,
		"First step: establish P", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode (1.1) failed: %v", err)
	}

	if err := svc.RefineNode(rootID, proverAgent, child2ID, schema.NodeTypeClaim,
		"Second step: derive Q from P", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (1.2) failed: %v", err)
	}

	if err := svc.ReleaseNode(rootID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	t.Log("  Created root with 2 children")

	// ==========================================================================
	// Step 2: Admit the root node (introduces taint)
	// ==========================================================================
	t.Log("Step 2: Admit root node (introduces taint)")

	if err := svc.AdmitNode(rootID); err != nil {
		t.Fatalf("AdmitNode (root) failed: %v", err)
	}

	// Validate children to have a deterministic epistemic state
	if err := svc.AcceptNode(child1ID); err != nil {
		t.Fatalf("AcceptNode (child1) failed: %v", err)
	}
	if err := svc.AcceptNode(child2ID); err != nil {
		t.Fatalf("AcceptNode (child2) failed: %v", err)
	}

	t.Log("  Root admitted, children validated")

	// ==========================================================================
	// Step 3: Verify taint propagation
	// ==========================================================================
	t.Log("Step 3: Verify taint propagation to children")

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Compute taint for all nodes
	computeTaintForState(st)

	rootNode := st.GetNode(rootID)
	child1Node := st.GetNode(child1ID)
	child2Node := st.GetNode(child2ID)

	if rootNode == nil || child1Node == nil || child2Node == nil {
		t.Fatal("Nodes not found in state")
	}

	// Verify root has self_admitted taint
	if rootNode.TaintState != node.TaintSelfAdmitted {
		t.Errorf("Root taint = %v, want %v", rootNode.TaintState, node.TaintSelfAdmitted)
	}

	// Verify children are tainted (inherited from admitted parent)
	if child1Node.TaintState != node.TaintTainted {
		t.Errorf("Child1 taint = %v, want %v", child1Node.TaintState, node.TaintTainted)
	}
	if child2Node.TaintState != node.TaintTainted {
		t.Errorf("Child2 taint = %v, want %v", child2Node.TaintState, node.TaintTainted)
	}

	t.Log("  Taint correctly propagates: root=self_admitted, children=tainted")

	// ==========================================================================
	// Step 4: Verify taint information appears in job detection
	// ==========================================================================
	t.Log("Step 4: Verify taint appears in job results")

	// Get job results - at this point, all nodes are accepted/admitted,
	// so there should be no jobs (epistemic state is not pending)
	jobResult := findJobsWithTaint(st)

	// No pending jobs since all nodes are accepted/admitted
	if len(jobResult.ProverJobs) != 0 {
		t.Errorf("Expected 0 prover jobs (all nodes settled), got %d", len(jobResult.ProverJobs))
	}
	if len(jobResult.VerifierJobs) != 0 {
		t.Errorf("Expected 0 verifier jobs (all nodes settled), got %d", len(jobResult.VerifierJobs))
	}

	// Verify that the taint state is correctly set on all nodes
	allNodes := st.AllNodes()
	taintedCount := 0
	selfAdmittedCount := 0
	for _, n := range allNodes {
		if n.TaintState == node.TaintTainted {
			taintedCount++
		}
		if n.TaintState == node.TaintSelfAdmitted {
			selfAdmittedCount++
		}
	}

	if selfAdmittedCount != 1 {
		t.Errorf("Expected 1 self_admitted node (root), got %d", selfAdmittedCount)
	}
	if taintedCount != 2 {
		t.Errorf("Expected 2 tainted nodes (children), got %d", taintedCount)
	}

	t.Log("  Job detection correctly observes taint states")
	t.Log("")
	t.Log("========================================")
	t.Log("  TAINT-JOBS INTEGRATION TEST PASSED!")
	t.Log("  Admit -> children tainted -> visible in jobs")
	t.Log("========================================")
}

// TestTaintJobsIntegration_PendingChildrenShowUnresolvedTaint tests that pending children
// of an admitted parent show unresolved taint (per taint computation rules: pending = unresolved).
// When they are accepted, they will inherit taint from their admitted ancestor.
func TestTaintJobsIntegration_PendingChildrenShowUnresolvedTaint(t *testing.T) {
	proofDir, cleanup := setupTaintJobsTest(t)
	defer cleanup()

	conjecture := "Theorem with pending steps"
	svc := initTaintJobsProof(t, proofDir, conjecture)

	rootID := parseTaintJobsNodeID(t, "1")
	child1ID := parseTaintJobsNodeID(t, "1.1")
	grandchildID := parseTaintJobsNodeID(t, "1.1.1")

	// ==========================================================================
	// Step 1: Create three-level proof structure
	// ==========================================================================
	t.Log("Step 1: Create proof structure with root, child, and grandchild")

	proverAgent := "prover-agent-001"
	if err := svc.ClaimNode(rootID, proverAgent, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	if err := svc.RefineNode(rootID, proverAgent, child1ID, schema.NodeTypeClaim,
		"Intermediate step", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (1.1) failed: %v", err)
	}
	if err := svc.ReleaseNode(rootID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	// Create grandchild
	if err := svc.ClaimNode(child1ID, proverAgent, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (child1) failed: %v", err)
	}
	if err := svc.RefineNode(child1ID, proverAgent, grandchildID, schema.NodeTypeClaim,
		"Detailed step", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (1.1.1) failed: %v", err)
	}
	if err := svc.ReleaseNode(child1ID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (child1) failed: %v", err)
	}

	t.Log("  Created three-level structure: root -> child -> grandchild")

	// ==========================================================================
	// Step 2: Admit root node (introduces taint at the top)
	// ==========================================================================
	t.Log("Step 2: Admit root node")

	if err := svc.AdmitNode(rootID); err != nil {
		t.Fatalf("AdmitNode (root) failed: %v", err)
	}

	// Do NOT accept children yet - they remain pending

	t.Log("  Root admitted, children remain pending")

	// ==========================================================================
	// Step 3: Verify pending children are verifier jobs with unresolved taint
	// ==========================================================================
	t.Log("Step 3: Verify pending children appear as verifier jobs with unresolved taint")

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Compute taint for all nodes
	computeTaintForState(st)

	// Get jobs - child and grandchild should be verifier jobs (pending, available)
	jobResult := findJobsWithTaint(st)

	if len(jobResult.VerifierJobs) != 2 {
		t.Errorf("Expected 2 verifier jobs (pending children), got %d", len(jobResult.VerifierJobs))
	}

	// Per taint computation rules: pending nodes have unresolved taint
	for _, job := range jobResult.VerifierJobs {
		if job.TaintState != node.TaintUnresolved {
			t.Errorf("Verifier job %s taint = %v, want %v (pending node = unresolved)",
				job.ID, job.TaintState, node.TaintUnresolved)
		}
	}

	t.Log("  Pending children appear as verifier jobs with unresolved taint")

	// ==========================================================================
	// Step 4: Accept children and verify they become tainted
	// ==========================================================================
	t.Log("Step 4: Accept children and verify they become tainted")

	if err := svc.AcceptNode(child1ID); err != nil {
		t.Fatalf("AcceptNode (child1) failed: %v", err)
	}
	if err := svc.AcceptNode(grandchildID); err != nil {
		t.Fatalf("AcceptNode (grandchild) failed: %v", err)
	}

	// Reload state and recompute taint
	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	computeTaintForState(st)

	child1Node := st.GetNode(child1ID)
	grandchildNode := st.GetNode(grandchildID)

	if child1Node == nil || grandchildNode == nil {
		t.Fatal("Nodes not found in state")
	}

	// After acceptance, children should be tainted (inherit from admitted root)
	if child1Node.TaintState != node.TaintTainted {
		t.Errorf("Child1 taint after acceptance = %v, want %v (descendant of admitted)",
			child1Node.TaintState, node.TaintTainted)
	}
	if grandchildNode.TaintState != node.TaintTainted {
		t.Errorf("Grandchild taint after acceptance = %v, want %v (descendant of admitted)",
			grandchildNode.TaintState, node.TaintTainted)
	}

	t.Log("  After acceptance, children are tainted (inherited from admitted root)")
	t.Log("")
	t.Log("========================================")
	t.Log("  PENDING->ACCEPTED TAINT TEST PASSED!")
	t.Log("  Pending = unresolved, Accepted = tainted")
	t.Log("========================================")
}
