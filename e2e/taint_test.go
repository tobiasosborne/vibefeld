//go:build integration

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/taint"
	"github.com/tobias/vibefeld/internal/types"
)

// setupTaintTest creates a temporary directory for the test and returns the proof directory path
// and a cleanup function.
func setupTaintTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-taint-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// initProof initializes a proof with the given conjecture.
func initProof(t *testing.T, proofDir, conjecture string) *service.ProofService {
	t.Helper()
	err := service.Init(proofDir, conjecture, "test-author")
	if err != nil {
		t.Fatalf("failed to initialize proof: %v", err)
	}
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}
	return svc
}

// mustParseID parses a node ID string and fails the test if it fails.
func mustParseID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("failed to parse node ID %q: %v", s, err)
	}
	return id
}

// computeAndUpdateTaint computes taint for a node and updates its TaintState field.
// This helper ensures ancestors have their TaintState updated before computing child taint.
func computeAndUpdateTaint(n *node.Node, ancestors []*node.Node) node.TaintState {
	computed := taint.ComputeTaint(n, ancestors)
	n.TaintState = computed
	return computed
}

// TestTaint_AdmittedNodeIsSelfTainted tests that admitting a node (instead of validating)
// results in the node having TaintSelfAdmitted status after taint computation.
func TestTaint_AdmittedNodeIsSelfTainted(t *testing.T) {
	proofDir, cleanup := setupTaintTest(t)
	defer cleanup()

	// 1. Create proof with root node
	svc := initProof(t, proofDir, "Test conjecture for taint")

	// 2. Admit the root node (not validate)
	rootID := mustParseID(t, "1")
	err := svc.AdmitNode(rootID)
	if err != nil {
		t.Fatalf("AdmitNode failed: %v", err)
	}

	// 3. Load state and compute taint
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := st.GetNode(rootID)
	if rootNode == nil {
		t.Fatal("root node not found in state")
	}

	// Compute taint for the root node (no ancestors)
	computedTaint := taint.ComputeTaint(rootNode, nil)

	// 4. Verify node has TaintSelfAdmitted
	if computedTaint != node.TaintSelfAdmitted {
		t.Errorf("admitted node taint = %v, want %v", computedTaint, node.TaintSelfAdmitted)
	}

	// Also verify the epistemic state is admitted
	if rootNode.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("admitted node epistemic state = %v, want %v", rootNode.EpistemicState, schema.EpistemicAdmitted)
	}
}

// TestTaint_PropagationToParent tests that when a parent is admitted,
// taint propagates down to children (descendants become tainted).
// In AF's hierarchical model, children are refinements of parents,
// so taint flows from parents to children (down the tree).
func TestTaint_PropagationToParent(t *testing.T) {
	proofDir, cleanup := setupTaintTest(t)
	defer cleanup()

	// 1. Create proof: root -> child -> grandchild
	svc := initProof(t, proofDir, "Test conjecture for propagation")

	childID := mustParseID(t, "1.1")
	grandchildID := mustParseID(t, "1.1.1")

	err := svc.CreateNode(childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode for child failed: %v", err)
	}
	err = svc.CreateNode(grandchildID, schema.NodeTypeClaim, "Grandchild statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode for grandchild failed: %v", err)
	}

	// 2. Admit the root (introduces taint) and validate children
	rootID := mustParseID(t, "1")
	err = svc.AdmitNode(rootID)
	if err != nil {
		t.Fatalf("AdmitNode for root failed: %v", err)
	}
	err = svc.AcceptNode(childID)
	if err != nil {
		t.Fatalf("AcceptNode for child failed: %v", err)
	}
	err = svc.AcceptNode(grandchildID)
	if err != nil {
		t.Fatalf("AcceptNode for grandchild failed: %v", err)
	}

	// 3. Load state
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := st.GetNode(rootID)
	childNode := st.GetNode(childID)
	grandchildNode := st.GetNode(grandchildID)

	if rootNode == nil || childNode == nil || grandchildNode == nil {
		t.Fatal("nodes not found in state")
	}

	// 4. Compute taint in order (parents first, then children)
	// Root is admitted -> self_admitted
	rootTaint := computeAndUpdateTaint(rootNode, nil)
	if rootTaint != node.TaintSelfAdmitted {
		t.Errorf("root taint = %v, want %v", rootTaint, node.TaintSelfAdmitted)
	}

	// Child has admitted ancestor -> tainted
	childTaint := computeAndUpdateTaint(childNode, []*node.Node{rootNode})
	if childTaint != node.TaintTainted {
		t.Errorf("child taint = %v, want %v (parent is admitted)", childTaint, node.TaintTainted)
	}

	// Grandchild has tainted ancestor -> tainted
	grandchildTaint := computeAndUpdateTaint(grandchildNode, []*node.Node{childNode, rootNode})
	if grandchildTaint != node.TaintTainted {
		t.Errorf("grandchild taint = %v, want %v (ancestor is admitted)", grandchildTaint, node.TaintTainted)
	}
}

// TestTaint_CleanWhenAllValidated tests that when all nodes are validated (not admitted),
// they all have TaintClean status.
func TestTaint_CleanWhenAllValidated(t *testing.T) {
	proofDir, cleanup := setupTaintTest(t)
	defer cleanup()

	// 1. Create proof with root and children
	svc := initProof(t, proofDir, "Test conjecture for clean taint")

	child1ID := mustParseID(t, "1.1")
	child2ID := mustParseID(t, "1.2")
	err := svc.CreateNode(child1ID, schema.NodeTypeClaim, "Child 1 statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode for child1 failed: %v", err)
	}
	err = svc.CreateNode(child2ID, schema.NodeTypeClaim, "Child 2 statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode for child2 failed: %v", err)
	}

	// 2. Validate all nodes (not admit)
	rootID := mustParseID(t, "1")
	err = svc.AcceptNode(rootID)
	if err != nil {
		t.Fatalf("AcceptNode for root failed: %v", err)
	}
	err = svc.AcceptNode(child1ID)
	if err != nil {
		t.Fatalf("AcceptNode for child1 failed: %v", err)
	}
	err = svc.AcceptNode(child2ID)
	if err != nil {
		t.Fatalf("AcceptNode for child2 failed: %v", err)
	}

	// 3. Load state
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := st.GetNode(rootID)
	child1Node := st.GetNode(child1ID)
	child2Node := st.GetNode(child2ID)

	if rootNode == nil || child1Node == nil || child2Node == nil {
		t.Fatal("nodes not found in state")
	}

	// 4. Compute taint in order (root first, then children)
	rootTaint := computeAndUpdateTaint(rootNode, nil)
	child1Taint := computeAndUpdateTaint(child1Node, []*node.Node{rootNode})
	child2Taint := computeAndUpdateTaint(child2Node, []*node.Node{rootNode})

	// 5. Verify all nodes are TaintClean
	if rootTaint != node.TaintClean {
		t.Errorf("root taint = %v, want %v", rootTaint, node.TaintClean)
	}
	if child1Taint != node.TaintClean {
		t.Errorf("child1 taint = %v, want %v", child1Taint, node.TaintClean)
	}
	if child2Taint != node.TaintClean {
		t.Errorf("child2 taint = %v, want %v", child2Taint, node.TaintClean)
	}
}

// TestTaint_RefutedPropagation tests taint behavior with refuted nodes.
// Refuted nodes do not introduce taint (only admitted does).
// Also tests that pending nodes result in unresolved taint.
func TestTaint_RefutedPropagation(t *testing.T) {
	proofDir, cleanup := setupTaintTest(t)
	defer cleanup()

	// 1. Create proof: root -> child
	svc := initProof(t, proofDir, "Test conjecture for refuted propagation")

	childID := mustParseID(t, "1.1")
	err := svc.CreateNode(childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	// Validate the root
	rootID := mustParseID(t, "1")
	err = svc.AcceptNode(rootID)
	if err != nil {
		t.Fatalf("AcceptNode for root failed: %v", err)
	}

	// 2. Refute the child
	err = svc.RefuteNode(childID)
	if err != nil {
		t.Fatalf("RefuteNode failed: %v", err)
	}

	// 3. Load state and compute taint
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := st.GetNode(rootID)
	childNode := st.GetNode(childID)

	if rootNode == nil || childNode == nil {
		t.Fatal("nodes not found in state")
	}

	// Verify the child is refuted
	if childNode.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("child epistemic state = %v, want %v", childNode.EpistemicState, schema.EpistemicRefuted)
	}

	// Compute taint in order
	rootTaint := computeAndUpdateTaint(rootNode, nil)
	childTaint := computeAndUpdateTaint(childNode, []*node.Node{rootNode})

	// Root is validated -> clean
	if rootTaint != node.TaintClean {
		t.Errorf("root taint = %v, want %v", rootTaint, node.TaintClean)
	}

	// Refuted is a final state but doesn't introduce taint
	// So a refuted node with clean ancestors should be clean
	if childTaint != node.TaintClean {
		t.Errorf("refuted child taint = %v, want %v (refuted doesn't introduce taint)", childTaint, node.TaintClean)
	}

	// 4. Now test with a pending parent - pending causes unresolved
	proofDir2, cleanup2 := setupTaintTest(t)
	defer cleanup2()

	svc2 := initProof(t, proofDir2, "Test conjecture for unresolved")
	child2ID := mustParseID(t, "1.1")
	err = svc2.CreateNode(child2ID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	// Root stays pending (not validated or admitted)
	st2, err := svc2.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	root2Node := st2.GetNode(mustParseID(t, "1"))
	child2Node := st2.GetNode(child2ID)

	// Verify root is pending
	if root2Node.EpistemicState != schema.EpistemicPending {
		t.Errorf("root epistemic state = %v, want %v", root2Node.EpistemicState, schema.EpistemicPending)
	}

	// Compute taint in order
	root2Taint := computeAndUpdateTaint(root2Node, nil)
	child2Taint := computeAndUpdateTaint(child2Node, []*node.Node{root2Node})

	// Pending node -> unresolved taint
	if root2Taint != node.TaintUnresolved {
		t.Errorf("pending root taint = %v, want %v", root2Taint, node.TaintUnresolved)
	}

	// Child under pending parent -> unresolved (due to unresolved ancestor)
	if child2Taint != node.TaintUnresolved {
		t.Errorf("child under pending parent taint = %v, want %v", child2Taint, node.TaintUnresolved)
	}
}

// TestTaint_PropagateTaintFunction tests the PropagateTaint function from the taint package.
// PropagateTaint propagates taint from a root node to all its descendants.
func TestTaint_PropagateTaintFunction(t *testing.T) {
	proofDir, cleanup := setupTaintTest(t)
	defer cleanup()

	// Create proof with multiple levels
	svc := initProof(t, proofDir, "Test conjecture for propagation function")

	childID := mustParseID(t, "1.1")
	grandchildID := mustParseID(t, "1.1.1")

	err := svc.CreateNode(childID, schema.NodeTypeClaim, "Child", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode for child failed: %v", err)
	}
	err = svc.CreateNode(grandchildID, schema.NodeTypeClaim, "Grandchild", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode for grandchild failed: %v", err)
	}

	// Admit the root, validate the rest
	rootID := mustParseID(t, "1")
	err = svc.AdmitNode(rootID)
	if err != nil {
		t.Fatalf("AdmitNode for root failed: %v", err)
	}
	err = svc.AcceptNode(childID)
	if err != nil {
		t.Fatalf("AcceptNode for child failed: %v", err)
	}
	err = svc.AcceptNode(grandchildID)
	if err != nil {
		t.Fatalf("AcceptNode for grandchild failed: %v", err)
	}

	// Load state
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := st.GetNode(rootID)
	childNode := st.GetNode(childID)
	grandchildNode := st.GetNode(grandchildID)

	// Set root's taint to self_admitted (as it should be after admission)
	rootNode.TaintState = node.TaintSelfAdmitted

	// Initially child and grandchild have default taint (unresolved from creation)
	// Now use PropagateTaint to propagate from root
	allNodes := []*node.Node{rootNode, childNode, grandchildNode}
	changed := taint.PropagateTaint(rootNode, allNodes)

	// Both child and grandchild should have changed
	if len(changed) != 2 {
		t.Errorf("PropagateTaint returned %d changed nodes, want 2", len(changed))
	}

	// Verify taint states after propagation
	if childNode.TaintState != node.TaintTainted {
		t.Errorf("child taint after propagation = %v, want %v", childNode.TaintState, node.TaintTainted)
	}
	if grandchildNode.TaintState != node.TaintTainted {
		t.Errorf("grandchild taint after propagation = %v, want %v", grandchildNode.TaintState, node.TaintTainted)
	}
}

// TestTaint_MixedAdmittedAndValidated tests a scenario where some nodes are admitted
// and some are validated, checking correct taint propagation.
func TestTaint_MixedAdmittedAndValidated(t *testing.T) {
	proofDir, cleanup := setupTaintTest(t)
	defer cleanup()

	// Create a tree:
	//   root (1) - validated (clean)
	//   |- child1 (1.1) - admitted (self_admitted)
	//   |  |- grandchild1 (1.1.1) - validated (but tainted due to 1.1)
	//   |- child2 (1.2) - validated (clean)
	//      |- grandchild2 (1.2.1) - validated (clean)

	svc := initProof(t, proofDir, "Mixed taint test")

	// Create all nodes
	child1ID := mustParseID(t, "1.1")
	child2ID := mustParseID(t, "1.2")
	grandchild1ID := mustParseID(t, "1.1.1")
	grandchild2ID := mustParseID(t, "1.2.1")

	err := svc.CreateNode(child1ID, schema.NodeTypeClaim, "Child 1", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	err = svc.CreateNode(child2ID, schema.NodeTypeClaim, "Child 2", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	err = svc.CreateNode(grandchild1ID, schema.NodeTypeClaim, "Grandchild 1", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	err = svc.CreateNode(grandchild2ID, schema.NodeTypeClaim, "Grandchild 2", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	// Set epistemic states
	rootID := mustParseID(t, "1")
	_ = svc.AcceptNode(rootID)        // validated
	_ = svc.AdmitNode(child1ID)       // admitted
	_ = svc.AcceptNode(child2ID)      // validated
	_ = svc.AcceptNode(grandchild1ID) // validated
	_ = svc.AcceptNode(grandchild2ID) // validated

	// Load state
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := st.GetNode(rootID)
	child1Node := st.GetNode(child1ID)
	child2Node := st.GetNode(child2ID)
	grandchild1Node := st.GetNode(grandchild1ID)
	grandchild2Node := st.GetNode(grandchild2ID)

	// Compute taint in topological order (parents before children)
	rootTaint := computeAndUpdateTaint(rootNode, nil)
	child1Taint := computeAndUpdateTaint(child1Node, []*node.Node{rootNode})
	child2Taint := computeAndUpdateTaint(child2Node, []*node.Node{rootNode})
	grandchild1Taint := computeAndUpdateTaint(grandchild1Node, []*node.Node{child1Node, rootNode})
	grandchild2Taint := computeAndUpdateTaint(grandchild2Node, []*node.Node{child2Node, rootNode})

	// Verify expected taint states
	if rootTaint != node.TaintClean {
		t.Errorf("root taint = %v, want %v", rootTaint, node.TaintClean)
	}
	if child1Taint != node.TaintSelfAdmitted {
		t.Errorf("child1 taint = %v, want %v", child1Taint, node.TaintSelfAdmitted)
	}
	if child2Taint != node.TaintClean {
		t.Errorf("child2 taint = %v, want %v", child2Taint, node.TaintClean)
	}
	if grandchild1Taint != node.TaintTainted {
		t.Errorf("grandchild1 taint = %v, want %v (ancestor 1.1 is admitted)", grandchild1Taint, node.TaintTainted)
	}
	if grandchild2Taint != node.TaintClean {
		t.Errorf("grandchild2 taint = %v, want %v", grandchild2Taint, node.TaintClean)
	}
}
