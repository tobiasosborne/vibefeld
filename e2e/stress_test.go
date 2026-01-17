//go:build integration

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// setupStressTest creates a temporary proof directory for stress testing.
func setupStressTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-stress-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// initStressProof initializes a proof for stress testing.
func initStressProof(t *testing.T, proofDir, conjecture string) *service.ProofService {
	t.Helper()
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("failed to initialize proof dir: %v", err)
	}
	if err := service.Init(proofDir, conjecture, "stress-test-author"); err != nil {
		t.Fatalf("failed to initialize proof: %v", err)
	}
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}
	return svc
}

// TestStress_LargeProofTree tests creating a proof tree with 100+ nodes.
// This exercises the system's ability to handle:
// - Many sequential node creations
// - Deep hierarchical structures
// - State loading with many nodes
// - Status computation across large trees
func TestStress_LargeProofTree(t *testing.T) {
	proofDir, cleanup := setupStressTest(t)
	defer cleanup()

	conjecture := "Stress test: Large proof tree with 100+ nodes"
	svc := initStressProof(t, proofDir, conjecture)

	// Target: Create 100+ nodes in a hierarchical structure
	// Structure: root (1) with 10 children (1.1-1.10), each with 10 grandchildren
	// Total: 1 root + 10 children + 100 grandchildren = 111 nodes

	rootID, _ := types.Parse("1")
	proverAgent := "stress-prover"

	// ==========================================================================
	// Step 1: Claim root and create 10 children
	// ==========================================================================
	t.Log("Step 1: Create 10 children under root")

	if err := svc.ClaimNode(rootID, proverAgent, 10*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	childIDs := make([]types.NodeID, 10)
	for i := 1; i <= 10; i++ {
		childID, _ := types.Parse(fmt.Sprintf("1.%d", i))
		childIDs[i-1] = childID

		if err := svc.RefineNode(rootID, proverAgent, childID, schema.NodeTypeClaim,
			fmt.Sprintf("Child %d statement", i), schema.InferenceAssumption); err != nil {
			t.Fatalf("RefineNode (1.%d) failed: %v", i, err)
		}
	}

	if err := svc.ReleaseNode(rootID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	t.Logf("  Created %d children", len(childIDs))

	// ==========================================================================
	// Step 2: Create 10 grandchildren under each child
	// ==========================================================================
	t.Log("Step 2: Create 10 grandchildren under each child (100 total)")

	grandchildCount := 0
	for i, childID := range childIDs {
		if err := svc.ClaimNode(childID, proverAgent, 10*time.Minute); err != nil {
			t.Fatalf("ClaimNode (%s) failed: %v", childID, err)
		}

		for j := 1; j <= 10; j++ {
			grandchildID, _ := types.Parse(fmt.Sprintf("1.%d.%d", i+1, j))

			if err := svc.RefineNode(childID, proverAgent, grandchildID, schema.NodeTypeClaim,
				fmt.Sprintf("Grandchild %d.%d statement", i+1, j), schema.InferenceModusPonens); err != nil {
				t.Fatalf("RefineNode (%s) failed: %v", grandchildID, err)
			}
			grandchildCount++
		}

		if err := svc.ReleaseNode(childID, proverAgent); err != nil {
			t.Fatalf("ReleaseNode (%s) failed: %v", childID, err)
		}
	}

	t.Logf("  Created %d grandchildren", grandchildCount)

	// ==========================================================================
	// Step 3: Verify total node count
	// ==========================================================================
	t.Log("Step 3: Verify total node count")

	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	allNodes := state.AllNodes()
	expectedNodes := 1 + 10 + 100 // root + children + grandchildren
	if len(allNodes) != expectedNodes {
		t.Errorf("Expected %d nodes, got %d", expectedNodes, len(allNodes))
	}

	t.Logf("  Total nodes: %d", len(allNodes))

	// ==========================================================================
	// Step 4: Verify status computation works with many nodes
	// ==========================================================================
	t.Log("Step 4: Verify status computation")

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if status.TotalNodes != expectedNodes {
		t.Errorf("Status TotalNodes = %d, want %d", status.TotalNodes, expectedNodes)
	}

	t.Logf("  Status: %d total, %d pending, %d validated",
		status.TotalNodes, status.PendingNodes, status.ValidatedNodes)

	// ==========================================================================
	// Step 5: Accept all grandchildren
	// ==========================================================================
	t.Log("Step 5: Accept all grandchildren")

	acceptedCount := 0
	for i := 1; i <= 10; i++ {
		for j := 1; j <= 10; j++ {
			grandchildID, _ := types.Parse(fmt.Sprintf("1.%d.%d", i, j))
			if err := svc.AcceptNode(grandchildID); err != nil {
				t.Fatalf("AcceptNode (%s) failed: %v", grandchildID, err)
			}
			acceptedCount++
		}
	}

	t.Logf("  Accepted %d grandchildren", acceptedCount)

	// ==========================================================================
	// Step 6: Accept all children and root
	// ==========================================================================
	t.Log("Step 6: Accept all children and root")

	for i := 1; i <= 10; i++ {
		childID, _ := types.Parse(fmt.Sprintf("1.%d", i))
		if err := svc.AcceptNode(childID); err != nil {
			t.Fatalf("AcceptNode (%s) failed: %v", childID, err)
		}
	}

	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	t.Log("  Accepted all children and root")

	// ==========================================================================
	// Step 7: Verify all nodes are validated
	// ==========================================================================
	t.Log("Step 7: Verify all nodes are validated")

	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	validatedCount := 0
	for _, n := range state.AllNodes() {
		if n.EpistemicState == schema.EpistemicValidated {
			validatedCount++
		}
	}

	if validatedCount != expectedNodes {
		t.Errorf("Expected %d validated nodes, got %d", expectedNodes, validatedCount)
	}

	t.Log("")
	t.Log("========================================")
	t.Logf("  STRESS TEST: LARGE TREE SUCCESS")
	t.Logf("  %d nodes created and validated", expectedNodes)
	t.Log("========================================")
}

// TestStress_ConcurrentOperations tests concurrent operations on a proof with many nodes.
// This exercises:
// - Multiple agents operating simultaneously
// - Lock contention and CAS semantics
// - State consistency under concurrent modifications
func TestStress_ConcurrentOperations(t *testing.T) {
	proofDir, cleanup := setupStressTest(t)
	defer cleanup()

	conjecture := "Stress test: Concurrent operations"
	svc := initStressProof(t, proofDir, conjecture)

	rootID, _ := types.Parse("1")
	setupAgent := "setup-agent"

	// ==========================================================================
	// Setup: Create leaf nodes for concurrent operations
	// Note: System limit is 10 children per node, so we create a 2-level structure
	// ==========================================================================
	t.Log("Setup: Create leaf nodes for concurrent operations (respecting max 10 children limit)")

	if err := svc.ClaimNode(rootID, setupAgent, 10*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	// Create 10 children under root
	childIDs := make([]types.NodeID, 10)
	for i := 1; i <= 10; i++ {
		childID, _ := types.Parse(fmt.Sprintf("1.%d", i))
		childIDs[i-1] = childID

		if err := svc.RefineNode(rootID, setupAgent, childID, schema.NodeTypeClaim,
			fmt.Sprintf("Child %d for concurrent test", i), schema.InferenceAssumption); err != nil {
			t.Fatalf("RefineNode (1.%d) failed: %v", i, err)
		}
	}

	if err := svc.ReleaseNode(rootID, setupAgent); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	// Create 5 grandchildren under first 2 children (total 10 leaf nodes at level 2)
	leafIDs := make([]types.NodeID, 0, 10)
	for i := 1; i <= 2; i++ {
		parentID := childIDs[i-1]
		if err := svc.ClaimNode(parentID, setupAgent, 10*time.Minute); err != nil {
			t.Fatalf("ClaimNode (%s) failed: %v", parentID, err)
		}

		for j := 1; j <= 5; j++ {
			leafID, _ := types.Parse(fmt.Sprintf("1.%d.%d", i, j))
			leafIDs = append(leafIDs, leafID)

			if err := svc.RefineNode(parentID, setupAgent, leafID, schema.NodeTypeClaim,
				fmt.Sprintf("Leaf %d.%d for concurrent test", i, j), schema.InferenceAssumption); err != nil {
				t.Fatalf("RefineNode (1.%d.%d) failed: %v", i, j, err)
			}
		}

		if err := svc.ReleaseNode(parentID, setupAgent); err != nil {
			t.Fatalf("ReleaseNode (%s) failed: %v", parentID, err)
		}
	}

	t.Logf("  Created %d leaf nodes at level 2, plus %d at level 1", len(leafIDs), len(childIDs))

	// ==========================================================================
	// Test 1: Concurrent claims on different nodes (using level 1 children 3-10)
	// ==========================================================================
	t.Log("Test 1: Concurrent claims on different nodes")

	numAgents := 8 // Use children 3-10 (indices 2-9)
	var wg sync.WaitGroup
	wg.Add(numAgents)

	var successfulClaims int32

	start := make(chan struct{})

	for i := 0; i < numAgents; i++ {
		idx := i
		agentName := fmt.Sprintf("agent-%d", idx)
		targetNode := childIDs[idx+2] // Use children 1.3 through 1.10

		go func() {
			defer wg.Done()
			<-start
			err := svc.ClaimNode(targetNode, agentName, 5*time.Minute)
			if err == nil {
				atomic.AddInt32(&successfulClaims, 1)
			}
		}()
	}

	close(start)
	wg.Wait()

	// Some claims may fail due to CAS conflicts, but at least some should succeed
	if successfulClaims == 0 {
		t.Error("Expected at least some claims to succeed")
	}
	t.Logf("  %d/%d claims succeeded", successfulClaims, numAgents)

	// ==========================================================================
	// Test 2: Concurrent acceptances on leaf nodes
	// ==========================================================================
	t.Log("Test 2: Concurrent acceptances on leaf nodes")

	// First release any claimed nodes
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	for _, n := range state.AllNodes() {
		if n.WorkflowState == schema.WorkflowClaimed && n.ID.String() != "1" {
			// Skip if we can't release (not the owner)
			_ = svc.ReleaseNode(n.ID, n.ClaimedBy)
		}
	}

	// Now try concurrent acceptances on the leaf nodes
	wg.Add(len(leafIDs))
	var successfulAccepts int32
	start2 := make(chan struct{})

	for i := 0; i < len(leafIDs); i++ {
		idx := i
		targetNode := leafIDs[idx]

		go func() {
			defer wg.Done()
			<-start2
			err := svc.AcceptNode(targetNode)
			if err == nil {
				atomic.AddInt32(&successfulAccepts, 1)
			}
		}()
	}

	close(start2)
	wg.Wait()

	// Due to CAS semantics, some may fail but at least some should succeed
	if successfulAccepts == 0 {
		t.Error("Expected at least some acceptances to succeed")
	}
	t.Logf("  %d/%d acceptances succeeded", successfulAccepts, len(leafIDs))

	// ==========================================================================
	// Test 3: Heavy load - many agents doing mixed operations
	// ==========================================================================
	t.Log("Test 3: Heavy load - mixed operations")

	// Reload state
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Find available nodes
	availableNodes := []types.NodeID{}
	for _, n := range state.AllNodes() {
		if n.WorkflowState == schema.WorkflowAvailable &&
			n.EpistemicState == schema.EpistemicPending {
			availableNodes = append(availableNodes, n.ID)
		}
	}

	numOperations := len(availableNodes)
	if numOperations > 0 {
		wg.Add(numOperations)
		var mixedSuccess int32
		start3 := make(chan struct{})

		for i, nodeID := range availableNodes {
			idx := i
			id := nodeID
			go func() {
				defer wg.Done()
				<-start3
				// Alternate between accept operations
				err := svc.AcceptNode(id)
				if err == nil {
					atomic.AddInt32(&mixedSuccess, 1)
				}
				_ = idx // Avoid unused variable warning
			}()
		}

		close(start3)
		wg.Wait()

		t.Logf("  Mixed operations: %d/%d succeeded", mixedSuccess, numOperations)
	}

	// ==========================================================================
	// Final verification
	// ==========================================================================
	t.Log("Final: Verify state consistency")

	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	allNodes := state.AllNodes()
	expectedNodes := 1 + 10 + 10 // root + 10 children + 10 grandchildren (5 under each of first 2 children)
	if len(allNodes) != expectedNodes {
		t.Errorf("Expected %d nodes, got %d", expectedNodes, len(allNodes))
	}

	// Count by epistemic state
	stateCount := make(map[schema.EpistemicState]int)
	for _, n := range allNodes {
		stateCount[n.EpistemicState]++
	}

	t.Logf("  Final state distribution: %v", stateCount)

	t.Log("")
	t.Log("========================================")
	t.Log("  STRESS TEST: CONCURRENT OPS SUCCESS")
	t.Log("========================================")
}

// TestStress_DeepHierarchy tests a deeply nested proof tree.
// This exercises:
// - Deep hierarchical ID parsing
// - Parent-child relationship tracking
// - State traversal with deep nesting
// Note: System limit is max depth of 20, so we create 19 levels (root is level 1)
func TestStress_DeepHierarchy(t *testing.T) {
	proofDir, cleanup := setupStressTest(t)
	defer cleanup()

	conjecture := "Stress test: Deep hierarchy (19 levels)"
	svc := initStressProof(t, proofDir, conjecture)

	proverAgent := "deep-prover"

	// Create a chain of 19 levels deep (max allowed is 20, root is level 1)
	// 1 -> 1.1 -> 1.1.1 -> ... (19 times)
	depth := 19

	t.Logf("Creating chain %d levels deep (system max is 20)", depth)

	currentID, _ := types.Parse("1")

	for level := 1; level <= depth; level++ {
		// Claim current node
		if err := svc.ClaimNode(currentID, proverAgent, 10*time.Minute); err != nil {
			t.Fatalf("ClaimNode at level %d failed: %v", level, err)
		}

		// Create child (next level)
		childIDStr := currentID.String() + ".1"
		childID, err := types.Parse(childIDStr)
		if err != nil {
			t.Fatalf("Parse child ID %q at level %d failed: %v", childIDStr, level, err)
		}

		if err := svc.RefineNode(currentID, proverAgent, childID, schema.NodeTypeClaim,
			fmt.Sprintf("Level %d statement", level+1), schema.InferenceModusPonens); err != nil {
			t.Fatalf("RefineNode at level %d failed: %v", level, err)
		}

		// Release current node
		if err := svc.ReleaseNode(currentID, proverAgent); err != nil {
			t.Fatalf("ReleaseNode at level %d failed: %v", level, err)
		}

		// Move to child for next iteration
		currentID = childID
	}

	// Verify total depth
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	allNodes := state.AllNodes()
	expectedNodes := depth + 1 // root + 20 children in chain
	if len(allNodes) != expectedNodes {
		t.Errorf("Expected %d nodes, got %d", expectedNodes, len(allNodes))
	}

	// Verify the deepest node exists
	deepestIDStr := "1"
	for i := 0; i < depth; i++ {
		deepestIDStr += ".1"
	}
	deepestID, _ := types.Parse(deepestIDStr)
	deepestNode := state.GetNode(deepestID)
	if deepestNode == nil {
		t.Errorf("Deepest node %s not found", deepestIDStr)
	} else {
		t.Logf("  Deepest node: %s", deepestNode.ID)
	}

	// Accept all nodes from bottom up
	t.Log("Accepting all nodes from deepest to root")

	for level := depth; level >= 0; level-- {
		idStr := "1"
		for i := 0; i < level; i++ {
			idStr += ".1"
		}
		nodeID, _ := types.Parse(idStr)
		if err := svc.AcceptNode(nodeID); err != nil {
			t.Fatalf("AcceptNode at level %d (%s) failed: %v", level, idStr, err)
		}
	}

	// Verify all validated
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	validatedCount := 0
	for _, n := range state.AllNodes() {
		if n.EpistemicState == schema.EpistemicValidated {
			validatedCount++
		}
	}

	if validatedCount != expectedNodes {
		t.Errorf("Expected %d validated nodes, got %d", expectedNodes, validatedCount)
	}

	t.Log("")
	t.Log("========================================")
	t.Logf("  STRESS TEST: DEEP HIERARCHY SUCCESS")
	t.Logf("  %d levels deep, all validated", depth)
	t.Log("========================================")
}

// TestStress_WideTree tests a very wide proof tree (many siblings across multiple parents).
// This exercises:
// - Maximum children under multiple parents
// - Sibling enumeration
// - Status aggregation across wide structures
// Note: System limit is 10 children per node, so we create a 2-level wide tree
func TestStress_WideTree(t *testing.T) {
	proofDir, cleanup := setupStressTest(t)
	defer cleanup()

	conjecture := "Stress test: Wide tree (10x10 = 100 nodes)"
	svc := initStressProof(t, proofDir, conjecture)

	rootID, _ := types.Parse("1")
	proverAgent := "wide-prover"

	// Create 10 children under root (max allowed), then 10 grandchildren under each
	// This gives us: 1 root + 10 children + 100 grandchildren = 111 nodes
	numChildren := 10
	numGrandchildrenPerChild := 10

	t.Logf("Creating %d children under root, each with %d grandchildren", numChildren, numGrandchildrenPerChild)

	if err := svc.ClaimNode(rootID, proverAgent, 10*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	childIDs := make([]types.NodeID, numChildren)
	for i := 1; i <= numChildren; i++ {
		childID, _ := types.Parse(fmt.Sprintf("1.%d", i))
		childIDs[i-1] = childID
		if err := svc.RefineNode(rootID, proverAgent, childID, schema.NodeTypeClaim,
			fmt.Sprintf("Wide child %d", i), schema.InferenceAssumption); err != nil {
			t.Fatalf("RefineNode (1.%d) failed: %v", i, err)
		}
	}

	if err := svc.ReleaseNode(rootID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	// Create grandchildren under each child
	allGrandchildIDs := make([]types.NodeID, 0, numChildren*numGrandchildrenPerChild)
	for i, childID := range childIDs {
		if err := svc.ClaimNode(childID, proverAgent, 10*time.Minute); err != nil {
			t.Fatalf("ClaimNode (%s) failed: %v", childID, err)
		}

		for j := 1; j <= numGrandchildrenPerChild; j++ {
			grandchildID, _ := types.Parse(fmt.Sprintf("1.%d.%d", i+1, j))
			allGrandchildIDs = append(allGrandchildIDs, grandchildID)
			if err := svc.RefineNode(childID, proverAgent, grandchildID, schema.NodeTypeClaim,
				fmt.Sprintf("Wide grandchild %d.%d", i+1, j), schema.InferenceModusPonens); err != nil {
				t.Fatalf("RefineNode (1.%d.%d) failed: %v", i+1, j, err)
			}
		}

		if err := svc.ReleaseNode(childID, proverAgent); err != nil {
			t.Fatalf("ReleaseNode (%s) failed: %v", childID, err)
		}
	}

	// Verify node count
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	allNodes := state.AllNodes()
	expectedNodes := 1 + numChildren + (numChildren * numGrandchildrenPerChild)
	if len(allNodes) != expectedNodes {
		t.Errorf("Expected %d nodes, got %d", expectedNodes, len(allNodes))
	}

	t.Logf("  Created %d nodes total", len(allNodes))

	// Accept all grandchildren first, then children, then root
	t.Log("Accepting all nodes (bottom up)")

	// Accept grandchildren
	for _, gcID := range allGrandchildIDs {
		if err := svc.AcceptNode(gcID); err != nil {
			t.Fatalf("AcceptNode (%s) failed: %v", gcID, err)
		}
	}

	// Accept children
	for _, childID := range childIDs {
		if err := svc.AcceptNode(childID); err != nil {
			t.Fatalf("AcceptNode (%s) failed: %v", childID, err)
		}
	}

	// Accept root
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	// Verify final state
	status, err := svc.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if status.ValidatedNodes != expectedNodes {
		t.Errorf("Expected %d validated, got %d", expectedNodes, status.ValidatedNodes)
	}

	t.Log("")
	t.Log("========================================")
	t.Logf("  STRESS TEST: WIDE TREE SUCCESS")
	t.Logf("  %d nodes total, all validated", expectedNodes)
	t.Log("========================================")
}

// TestStress_RapidStateReloads tests rapid repeated state loading.
// This exercises:
// - State caching behavior
// - Event replay performance
// - Memory stability under repeated operations
func TestStress_RapidStateReloads(t *testing.T) {
	proofDir, cleanup := setupStressTest(t)
	defer cleanup()

	conjecture := "Stress test: Rapid state reloads"
	svc := initStressProof(t, proofDir, conjecture)

	rootID, _ := types.Parse("1")
	proverAgent := "reload-prover"

	// Create some nodes first
	if err := svc.ClaimNode(rootID, proverAgent, 10*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	for i := 1; i <= 10; i++ {
		childID, _ := types.Parse(fmt.Sprintf("1.%d", i))
		if err := svc.RefineNode(rootID, proverAgent, childID, schema.NodeTypeClaim,
			fmt.Sprintf("Node %d", i), schema.InferenceAssumption); err != nil {
			t.Fatalf("RefineNode failed: %v", err)
		}
	}

	if err := svc.ReleaseNode(rootID, proverAgent); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	// Rapid state reloads
	numReloads := 100
	t.Logf("Performing %d rapid state reloads", numReloads)

	startTime := time.Now()

	for i := 0; i < numReloads; i++ {
		state, err := svc.LoadState()
		if err != nil {
			t.Fatalf("LoadState iteration %d failed: %v", i, err)
		}
		if len(state.AllNodes()) != 11 {
			t.Errorf("Iteration %d: expected 11 nodes, got %d", i, len(state.AllNodes()))
		}
	}

	duration := time.Since(startTime)
	avgDuration := duration / time.Duration(numReloads)

	t.Logf("  %d reloads completed in %v (avg: %v/reload)", numReloads, duration, avgDuration)

	t.Log("")
	t.Log("========================================")
	t.Log("  STRESS TEST: RAPID RELOADS SUCCESS")
	t.Log("========================================")
}
