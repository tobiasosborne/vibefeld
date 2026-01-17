// Package node_test contains tests for dependency cycle detection.
package node_test

import (
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// helper function to create a node with dependencies and add it to state
func createNodeWithDeps(t *testing.T, s *state.State, idStr string, depStrs ...string) {
	t.Helper()

	id, err := types.Parse(idStr)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", idStr, err)
	}

	var deps []types.NodeID
	for _, depStr := range depStrs {
		dep, err := types.Parse(depStr)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", depStr, err)
		}
		deps = append(deps, dep)
	}

	opts := node.NodeOptions{
		Dependencies: deps,
	}

	n, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceModusPonens, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}

	s.AddNode(n)
}

// helper function to create a simple node without dependencies
func createNode(t *testing.T, s *state.State, idStr string) {
	t.Helper()
	createNodeWithDeps(t, s, idStr)
}

// TestDetectCycle_NoDependencies tests that a node with no dependencies has no cycle.
func TestDetectCycle_NoDependencies(t *testing.T) {
	s := state.NewState()
	createNode(t, s, "1")

	id, _ := types.Parse("1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=true for node with no dependencies")
	}
	if len(cyclePath) != 0 {
		t.Errorf("DetectCycle() returned non-empty cyclePath %v for node with no dependencies", cyclePath)
	}
}

// TestDetectCycle_LinearChain tests a linear dependency chain (no cycle).
// A -> B -> C (no cycle)
func TestDetectCycle_LinearChain(t *testing.T) {
	s := state.NewState()

	// C has no deps
	createNode(t, s, "1.3")
	// B depends on C
	createNodeWithDeps(t, s, "1.2", "1.3")
	// A depends on B
	createNodeWithDeps(t, s, "1.1", "1.2")
	// Root
	createNode(t, s, "1")

	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=true for linear chain")
	}
	if len(cyclePath) != 0 {
		t.Errorf("DetectCycle() returned non-empty cyclePath %v for linear chain", cyclePath)
	}
}

// TestDetectCycle_SimpleCycle tests detection of a simple two-node cycle.
// A -> B -> A (cycle!)
func TestDetectCycle_SimpleCycle(t *testing.T) {
	s := state.NewState()

	// Create A depending on B
	createNodeWithDeps(t, s, "1.1", "1.2")
	// Create B depending on A (creates cycle)
	createNodeWithDeps(t, s, "1.2", "1.1")
	// Root
	createNode(t, s, "1")

	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if !hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=false for simple cycle A->B->A")
	}

	// cyclePath should show the cycle: [1.1, 1.2, 1.1] or similar
	if len(cyclePath) < 2 {
		t.Errorf("DetectCycle() cyclePath too short: %v", cyclePath)
	}

	// First and last elements should be the same (showing the cycle)
	if len(cyclePath) > 0 && cyclePath[0].String() != cyclePath[len(cyclePath)-1].String() {
		t.Errorf("DetectCycle() cyclePath should start and end with same node: %v", cyclePath)
	}
}

// TestDetectCycle_LongerCycle tests detection of a three-node cycle.
// A -> B -> C -> A (cycle!)
func TestDetectCycle_LongerCycle(t *testing.T) {
	s := state.NewState()

	// Create cycle: A -> B -> C -> A
	createNodeWithDeps(t, s, "1.1", "1.2")
	createNodeWithDeps(t, s, "1.2", "1.3")
	createNodeWithDeps(t, s, "1.3", "1.1")
	// Root
	createNode(t, s, "1")

	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if !hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=false for cycle A->B->C->A")
	}

	// cyclePath should contain at least 3 nodes plus the repeat
	if len(cyclePath) < 3 {
		t.Errorf("DetectCycle() cyclePath too short for 3-node cycle: %v", cyclePath)
	}

	// Verify cycle closes (first and last are same)
	if len(cyclePath) > 0 && cyclePath[0].String() != cyclePath[len(cyclePath)-1].String() {
		t.Errorf("DetectCycle() cyclePath should start and end with same node: %v", cyclePath)
	}
}

// TestDetectCycle_SelfReference tests detection of a self-referencing node.
// A -> A (self-cycle!)
func TestDetectCycle_SelfReference(t *testing.T) {
	s := state.NewState()

	// Create node that depends on itself
	createNodeWithDeps(t, s, "1.1", "1.1")
	// Root
	createNode(t, s, "1")

	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if !hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=false for self-reference A->A")
	}

	// cyclePath should show self-cycle: [1.1, 1.1]
	if len(cyclePath) < 2 {
		t.Errorf("DetectCycle() cyclePath too short for self-reference: %v", cyclePath)
	}
}

// TestDetectCycle_DiamondPattern tests that diamond dependency patterns are not cycles.
//
//	  A
//	 / \
//	B   C
//	 \ /
//	  D
//
// A depends on B and C; B and C both depend on D (no cycle)
func TestDetectCycle_DiamondPattern(t *testing.T) {
	s := state.NewState()

	// D has no deps
	createNode(t, s, "1.4")
	// B depends on D
	createNodeWithDeps(t, s, "1.2", "1.4")
	// C depends on D
	createNodeWithDeps(t, s, "1.3", "1.4")
	// A depends on B and C
	createNodeWithDeps(t, s, "1.1", "1.2", "1.3")
	// Root
	createNode(t, s, "1")

	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=true for diamond pattern (no cycle): %v", cyclePath)
	}
	if len(cyclePath) != 0 {
		t.Errorf("DetectCycle() returned non-empty cyclePath %v for diamond pattern", cyclePath)
	}
}

// TestDetectCycle_EmptyState tests cycle detection on empty state.
func TestDetectCycle_EmptyState(t *testing.T) {
	s := state.NewState()

	id, _ := types.Parse("1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=true for empty state")
	}
	if len(cyclePath) != 0 {
		t.Errorf("DetectCycle() returned non-empty cyclePath %v for empty state", cyclePath)
	}
}

// TestDetectCycle_NodeNotInState tests cycle detection when starting node doesn't exist.
func TestDetectCycle_NodeNotInState(t *testing.T) {
	s := state.NewState()
	createNode(t, s, "1")
	// 1.1 does not exist

	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	// Non-existent node should not report a cycle
	if hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=true for non-existent node")
	}
	if len(cyclePath) != 0 {
		t.Errorf("DetectCycle() returned non-empty cyclePath %v for non-existent node", cyclePath)
	}
}

// TestDetectCycle_DependencyNotInState tests cycle detection when a dependency doesn't exist.
func TestDetectCycle_DependencyNotInState(t *testing.T) {
	s := state.NewState()
	// 1.1 depends on 1.2, but 1.2 doesn't exist
	createNodeWithDeps(t, s, "1.1", "1.2")
	createNode(t, s, "1")

	id, _ := types.Parse("1.1")

	// Missing dependency should not cause a cycle (it's a broken reference, not a cycle)
	hasCycle, cyclePath := node.DetectCycle(s, id)

	if hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=true when dependency is missing")
	}
	if len(cyclePath) != 0 {
		t.Errorf("DetectCycle() returned non-empty cyclePath %v when dependency is missing", cyclePath)
	}
}

// TestDetectCycle_ComplexWithNestedCycle tests a complex graph with a nested cycle.
//
//	    1.1
//	   / | \
//	1.2 1.3 1.4
//	     |   |
//	    1.5  |
//	     \--/
//	    (1.5 -> 1.4 -> 1.5 is a cycle)
func TestDetectCycle_ComplexWithNestedCycle(t *testing.T) {
	s := state.NewState()

	// Create the graph with a cycle between 1.4 and 1.5
	createNodeWithDeps(t, s, "1.1", "1.2", "1.3", "1.4")
	createNode(t, s, "1.2")
	createNodeWithDeps(t, s, "1.3", "1.5")
	createNodeWithDeps(t, s, "1.4", "1.5")
	createNodeWithDeps(t, s, "1.5", "1.4") // Creates cycle 1.4 <-> 1.5
	createNode(t, s, "1")

	// Starting from 1.1, should detect the nested cycle
	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if !hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=false for graph with nested cycle")
	}
	if len(cyclePath) < 2 {
		t.Errorf("DetectCycle() cyclePath too short: %v", cyclePath)
	}
}

// TestDetectCycle_CycleNotReachableFromStart tests starting from a node that doesn't reach the cycle.
//
//	1.1 -> 1.2 (no cycle in this path)
//	1.3 -> 1.4 -> 1.3 (cycle exists but unreachable from 1.1)
func TestDetectCycle_CycleNotReachableFromStart(t *testing.T) {
	s := state.NewState()

	// Path from 1.1 (no cycle)
	createNodeWithDeps(t, s, "1.1", "1.2")
	createNode(t, s, "1.2")

	// Separate cycle (not reachable from 1.1)
	createNodeWithDeps(t, s, "1.3", "1.4")
	createNodeWithDeps(t, s, "1.4", "1.3")

	createNode(t, s, "1")

	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=true when cycle is not reachable from start node")
	}
	if len(cyclePath) != 0 {
		t.Errorf("DetectCycle() returned non-empty cyclePath %v when cycle is not reachable", cyclePath)
	}
}

// TestDetectCycle_MultipleDependencies tests node with multiple dependencies, one leading to cycle.
func TestDetectCycle_MultipleDependencies(t *testing.T) {
	s := state.NewState()

	// 1.1 depends on 1.2 and 1.3
	// 1.2 is fine (no cycle)
	// 1.3 leads to a cycle
	createNodeWithDeps(t, s, "1.1", "1.2", "1.3")
	createNode(t, s, "1.2")
	createNodeWithDeps(t, s, "1.3", "1.4")
	createNodeWithDeps(t, s, "1.4", "1.3") // Cycle: 1.3 -> 1.4 -> 1.3
	createNode(t, s, "1")

	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if !hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=false when one of multiple dependencies has a cycle")
	}
	if len(cyclePath) < 2 {
		t.Errorf("DetectCycle() cyclePath too short: %v", cyclePath)
	}
}

// TestDetectCycle_CyclePathContents verifies the cycle path contains expected nodes.
func TestDetectCycle_CyclePathContents(t *testing.T) {
	s := state.NewState()

	// Create simple cycle: 1.1 -> 1.2 -> 1.3 -> 1.1
	createNodeWithDeps(t, s, "1.1", "1.2")
	createNodeWithDeps(t, s, "1.2", "1.3")
	createNodeWithDeps(t, s, "1.3", "1.1")
	createNode(t, s, "1")

	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if !hasCycle {
		t.Fatalf("DetectCycle() should detect cycle")
	}

	// Build a set of nodes in the path
	pathSet := make(map[string]bool)
	for _, nodeID := range cyclePath {
		pathSet[nodeID.String()] = true
	}

	// All cycle nodes should be in the path
	expectedNodes := []string{"1.1", "1.2", "1.3"}
	for _, expected := range expectedNodes {
		if !pathSet[expected] {
			t.Errorf("DetectCycle() cyclePath missing expected node %s: got %v", expected, cyclePath)
		}
	}
}

// ===========================================================================
// ValidateDependencies tests - checks ALL nodes in state for cycles
// ===========================================================================

// TestValidateDependencies_EmptyState tests validation on empty state.
func TestValidateDependencies_EmptyState(t *testing.T) {
	s := state.NewState()

	cycles := node.ValidateDependencies(s)

	if len(cycles) != 0 {
		t.Errorf("ValidateDependencies() returned %d cycles for empty state, want 0", len(cycles))
	}
}

// TestValidateDependencies_NoCycles tests validation when no cycles exist.
func TestValidateDependencies_NoCycles(t *testing.T) {
	s := state.NewState()

	createNode(t, s, "1")
	createNodeWithDeps(t, s, "1.1", "1")
	createNodeWithDeps(t, s, "1.2", "1")
	createNodeWithDeps(t, s, "1.1.1", "1.1")

	cycles := node.ValidateDependencies(s)

	if len(cycles) != 0 {
		t.Errorf("ValidateDependencies() returned %d cycles, want 0", len(cycles))
	}
}

// TestValidateDependencies_SingleCycle tests detection of a single cycle.
func TestValidateDependencies_SingleCycle(t *testing.T) {
	s := state.NewState()

	createNode(t, s, "1")
	createNodeWithDeps(t, s, "1.1", "1.2")
	createNodeWithDeps(t, s, "1.2", "1.1")

	cycles := node.ValidateDependencies(s)

	if len(cycles) == 0 {
		t.Errorf("ValidateDependencies() returned 0 cycles, expected at least 1")
	}
}

// TestValidateDependencies_MultipleSeparateCycles tests detection of multiple independent cycles.
func TestValidateDependencies_MultipleSeparateCycles(t *testing.T) {
	s := state.NewState()

	createNode(t, s, "1")

	// Cycle 1: 1.1 <-> 1.2
	createNodeWithDeps(t, s, "1.1", "1.2")
	createNodeWithDeps(t, s, "1.2", "1.1")

	// Cycle 2: 1.3 -> 1.4 -> 1.5 -> 1.3
	createNodeWithDeps(t, s, "1.3", "1.4")
	createNodeWithDeps(t, s, "1.4", "1.5")
	createNodeWithDeps(t, s, "1.5", "1.3")

	// Non-cyclic node
	createNode(t, s, "1.6")

	cycles := node.ValidateDependencies(s)

	// Should detect at least 2 distinct cycles
	if len(cycles) < 2 {
		t.Errorf("ValidateDependencies() returned %d cycles, expected at least 2", len(cycles))
	}
}

// TestValidateDependencies_SelfCycle tests detection of self-referencing nodes.
func TestValidateDependencies_SelfCycle(t *testing.T) {
	s := state.NewState()

	createNode(t, s, "1")
	createNodeWithDeps(t, s, "1.1", "1.1") // Self-cycle

	cycles := node.ValidateDependencies(s)

	if len(cycles) == 0 {
		t.Errorf("ValidateDependencies() returned 0 cycles for self-referencing node")
	}
}

// TestValidateDependencies_CyclePathFormat tests that returned cycles have correct format.
func TestValidateDependencies_CyclePathFormat(t *testing.T) {
	s := state.NewState()

	createNode(t, s, "1")
	createNodeWithDeps(t, s, "1.1", "1.2")
	createNodeWithDeps(t, s, "1.2", "1.1")

	cycles := node.ValidateDependencies(s)

	if len(cycles) == 0 {
		t.Fatalf("ValidateDependencies() should detect cycle")
	}

	// Each cycle should be a valid path
	for i, cycle := range cycles {
		if len(cycle) < 2 {
			t.Errorf("Cycle %d is too short: %v", i, cycle)
		}

		// First and last should be the same (closed cycle)
		if cycle[0].String() != cycle[len(cycle)-1].String() {
			t.Errorf("Cycle %d doesn't close properly: %v", i, cycle)
		}
	}
}

// TestValidateDependencies_LargeGraph tests validation on a larger graph.
func TestValidateDependencies_LargeGraph(t *testing.T) {
	s := state.NewState()

	// Create a tree structure with no cycles
	createNode(t, s, "1")
	for i := 1; i <= 5; i++ {
		parent := "1"
		for j := 1; j <= 3; j++ {
			id := parent + "." + string(rune('0'+j))
			if j == 1 {
				createNodeWithDeps(t, s, id, parent)
			} else {
				createNode(t, s, id)
			}
		}
	}

	cycles := node.ValidateDependencies(s)

	if len(cycles) != 0 {
		t.Errorf("ValidateDependencies() returned %d cycles for acyclic tree", len(cycles))
	}
}

// TestValidateDependencies_NestedCycleInLargerGraph tests cycle detection in complex graph.
func TestValidateDependencies_NestedCycleInLargerGraph(t *testing.T) {
	s := state.NewState()

	// Create a larger graph with one cycle hidden inside
	createNode(t, s, "1")
	createNodeWithDeps(t, s, "1.1", "1")
	createNodeWithDeps(t, s, "1.2", "1")
	createNodeWithDeps(t, s, "1.1.1", "1.1")
	createNodeWithDeps(t, s, "1.1.2", "1.1")
	createNodeWithDeps(t, s, "1.2.1", "1.2")
	createNodeWithDeps(t, s, "1.2.2", "1.2")

	// Add a cycle deep in the graph
	createNodeWithDeps(t, s, "1.1.1.1", "1.1.1", "1.1.1.2")
	createNodeWithDeps(t, s, "1.1.1.2", "1.1.1.1") // Creates cycle!

	cycles := node.ValidateDependencies(s)

	if len(cycles) == 0 {
		t.Errorf("ValidateDependencies() failed to detect nested cycle")
	}
}

// ===========================================================================
// Edge cases and robustness tests
// ===========================================================================

// TestDetectCycle_DeepChainNoCycle tests a very deep dependency chain without cycles.
func TestDetectCycle_DeepChainNoCycle(t *testing.T) {
	s := state.NewState()

	// Create a chain: 1.1 -> 1.2 -> 1.3 -> ... -> 1.10
	createNode(t, s, "1")
	prev := "1"
	for i := 1; i <= 10; i++ {
		current := "1." + string(rune('0'+i))
		if i > 9 {
			current = "1.1.1.1.1.1.1.1.1.1" // Deep nesting for i=10
		}
		createNodeWithDeps(t, s, current, prev)
		prev = current
	}

	id, _ := types.Parse("1.1.1.1.1.1.1.1.1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=true for deep chain without cycle: %v", cyclePath)
	}
}

// TestDetectCycle_MultiplePathsToSameNode tests multiple paths converging (no cycle).
func TestDetectCycle_MultiplePathsToSameNode(t *testing.T) {
	s := state.NewState()

	// Create diamond: 1.1 -> {1.2, 1.3} -> 1.4
	createNode(t, s, "1")
	createNode(t, s, "1.4")
	createNodeWithDeps(t, s, "1.2", "1.4")
	createNodeWithDeps(t, s, "1.3", "1.4")
	createNodeWithDeps(t, s, "1.1", "1.2", "1.3")

	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=true for diamond pattern (multiple paths, no cycle): %v", cyclePath)
	}
}

// TestDetectCycle_MultipleEntryPointsToCycle tests detecting a cycle reachable from multiple paths.
func TestDetectCycle_MultipleEntryPointsToCycle(t *testing.T) {
	s := state.NewState()

	// 1.1 -> 1.2 -> 1.3
	//        1.2 -> 1.4 -> 1.2 (cycle)
	createNode(t, s, "1")
	createNodeWithDeps(t, s, "1.1", "1.2")
	createNodeWithDeps(t, s, "1.2", "1.3", "1.4")
	createNode(t, s, "1.3")
	createNodeWithDeps(t, s, "1.4", "1.2") // Creates cycle

	id, _ := types.Parse("1.1")

	hasCycle, cyclePath := node.DetectCycle(s, id)

	if !hasCycle {
		t.Errorf("DetectCycle() returned hasCycle=false for graph with cycle reachable via multiple paths")
	}
	if len(cyclePath) < 2 {
		t.Errorf("DetectCycle() cyclePath too short: %v", cyclePath)
	}
}

// TestDetectCycle_TransitiveCircular explicitly tests the A->B->C->A transitive cycle pattern.
// This is a classic three-node circular dependency where the cycle requires following
// the full transitive chain to detect.
func TestDetectCycle_TransitiveCircular(t *testing.T) {
	s := state.NewState()

	// Create root
	createNode(t, s, "1")

	// Create transitive cycle: A(1.1) -> B(1.2) -> C(1.3) -> A(1.1)
	// This is a classic circular dependency that requires following all three edges to detect.
	createNodeWithDeps(t, s, "1.1", "1.2") // A depends on B
	createNodeWithDeps(t, s, "1.2", "1.3") // B depends on C
	createNodeWithDeps(t, s, "1.3", "1.1") // C depends on A (closes the cycle)

	// Test detection from each node in the cycle

	t.Run("detect from A", func(t *testing.T) {
		id, _ := types.Parse("1.1")
		hasCycle, cyclePath := node.DetectCycle(s, id)

		if !hasCycle {
			t.Error("DetectCycle() should detect A->B->C->A cycle starting from A")
		}

		// Verify cycle path contains all three nodes
		if len(cyclePath) < 3 {
			t.Errorf("cyclePath should contain at least 3 nodes for A->B->C->A: got %v", cyclePath)
		}

		// Verify the cycle closes (first and last elements should be the same)
		if len(cyclePath) > 0 && cyclePath[0].String() != cyclePath[len(cyclePath)-1].String() {
			t.Errorf("cyclePath should close (first and last same): got %v", cyclePath)
		}

		// Verify all cycle participants are present
		pathNodes := make(map[string]bool)
		for _, n := range cyclePath {
			pathNodes[n.String()] = true
		}
		for _, expected := range []string{"1.1", "1.2", "1.3"} {
			if !pathNodes[expected] {
				t.Errorf("cyclePath should contain %s: got %v", expected, cyclePath)
			}
		}
	})

	t.Run("detect from B", func(t *testing.T) {
		id, _ := types.Parse("1.2")
		hasCycle, cyclePath := node.DetectCycle(s, id)

		if !hasCycle {
			t.Error("DetectCycle() should detect cycle starting from B")
		}

		// Cycle should still be detected and contain all three nodes
		if len(cyclePath) < 3 {
			t.Errorf("cyclePath should contain at least 3 nodes: got %v", cyclePath)
		}
	})

	t.Run("detect from C", func(t *testing.T) {
		id, _ := types.Parse("1.3")
		hasCycle, cyclePath := node.DetectCycle(s, id)

		if !hasCycle {
			t.Error("DetectCycle() should detect cycle starting from C")
		}

		// Cycle should still be detected and contain all three nodes
		if len(cyclePath) < 3 {
			t.Errorf("cyclePath should contain at least 3 nodes: got %v", cyclePath)
		}
	})

	// Test that ValidateDependencies also finds this cycle
	t.Run("validate dependencies finds cycle", func(t *testing.T) {
		cycles := node.ValidateDependencies(s)

		if len(cycles) == 0 {
			t.Error("ValidateDependencies() should find the transitive cycle")
		}

		// Verify at least one cycle contains all three nodes
		foundComplete := false
		for _, cycle := range cycles {
			pathNodes := make(map[string]bool)
			for _, n := range cycle {
				pathNodes[n.String()] = true
			}
			if pathNodes["1.1"] && pathNodes["1.2"] && pathNodes["1.3"] {
				foundComplete = true
				break
			}
		}

		if !foundComplete {
			t.Errorf("ValidateDependencies() should find cycle with all three nodes: got %v", cycles)
		}
	})
}
