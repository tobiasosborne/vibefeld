// Package cycle_test contains tests for dependency cycle detection.
package cycle_test

import (
	"testing"

	"github.com/tobias/vibefeld/internal/cycle"
	"github.com/tobias/vibefeld/internal/types"
)

// mockNode represents a node with its dependencies for testing.
type mockNode struct {
	id   types.NodeID
	deps []types.NodeID
}

// mockProvider implements cycle.DependencyProvider for testing.
type mockProvider struct {
	nodes map[string]*mockNode
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		nodes: make(map[string]*mockNode),
	}
}

func (p *mockProvider) addNode(idStr string, depStrs ...string) {
	id, _ := types.Parse(idStr)
	deps := make([]types.NodeID, len(depStrs))
	for i, depStr := range depStrs {
		deps[i], _ = types.Parse(depStr)
	}
	p.nodes[id.String()] = &mockNode{id: id, deps: deps}
}

func (p *mockProvider) GetNodeDependencies(id types.NodeID) ([]types.NodeID, bool) {
	node, ok := p.nodes[id.String()]
	if !ok {
		return nil, false
	}
	return node.deps, true
}

func (p *mockProvider) AllNodeIDs() []types.NodeID {
	ids := make([]types.NodeID, 0, len(p.nodes))
	for _, n := range p.nodes {
		ids = append(ids, n.id)
	}
	return ids
}

// ===========================================================================
// DetectCycleFrom tests - checks cycles starting from a specific node
// ===========================================================================

// TestDetectCycleFrom_NoDependencies tests that a node with no dependencies has no cycle.
func TestDetectCycleFrom_NoDependencies(t *testing.T) {
	p := newMockProvider()
	p.addNode("1")

	id, _ := types.Parse("1")
	result := cycle.DetectCycleFrom(p, id)

	if result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=true for node with no dependencies")
	}
	if len(result.Path) != 0 {
		t.Errorf("DetectCycleFrom() returned non-empty Path %v for node with no dependencies", result.Path)
	}
}

// TestDetectCycleFrom_LinearChain tests a linear dependency chain (no cycle).
// A -> B -> C (no cycle)
func TestDetectCycleFrom_LinearChain(t *testing.T) {
	p := newMockProvider()
	// C has no deps
	p.addNode("1.3")
	// B depends on C
	p.addNode("1.2", "1.3")
	// A depends on B
	p.addNode("1.1", "1.2")
	// Root
	p.addNode("1")

	id, _ := types.Parse("1.1")
	result := cycle.DetectCycleFrom(p, id)

	if result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=true for linear chain")
	}
	if len(result.Path) != 0 {
		t.Errorf("DetectCycleFrom() returned non-empty Path %v for linear chain", result.Path)
	}
}

// TestDetectCycleFrom_SimpleCycle tests detection of a simple two-node cycle.
// A -> B -> A (cycle!)
func TestDetectCycleFrom_SimpleCycle(t *testing.T) {
	p := newMockProvider()
	// Create A depending on B
	p.addNode("1.1", "1.2")
	// Create B depending on A (creates cycle)
	p.addNode("1.2", "1.1")
	// Root
	p.addNode("1")

	id, _ := types.Parse("1.1")
	result := cycle.DetectCycleFrom(p, id)

	if !result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=false for simple cycle A->B->A")
	}

	// Path should show the cycle: [1.1, 1.2, 1.1] or similar
	if len(result.Path) < 2 {
		t.Errorf("DetectCycleFrom() Path too short: %v", result.Path)
	}

	// First and last elements should be the same (showing the cycle)
	if len(result.Path) > 0 && result.Path[0].String() != result.Path[len(result.Path)-1].String() {
		t.Errorf("DetectCycleFrom() Path should start and end with same node: %v", result.Path)
	}
}

// TestDetectCycleFrom_LongerCycle tests detection of a three-node cycle.
// A -> B -> C -> A (cycle!)
func TestDetectCycleFrom_LongerCycle(t *testing.T) {
	p := newMockProvider()
	// Create cycle: A -> B -> C -> A
	p.addNode("1.1", "1.2")
	p.addNode("1.2", "1.3")
	p.addNode("1.3", "1.1")
	// Root
	p.addNode("1")

	id, _ := types.Parse("1.1")
	result := cycle.DetectCycleFrom(p, id)

	if !result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=false for cycle A->B->C->A")
	}

	// Path should contain at least 3 nodes plus the repeat
	if len(result.Path) < 3 {
		t.Errorf("DetectCycleFrom() Path too short for 3-node cycle: %v", result.Path)
	}

	// Verify cycle closes (first and last are same)
	if len(result.Path) > 0 && result.Path[0].String() != result.Path[len(result.Path)-1].String() {
		t.Errorf("DetectCycleFrom() Path should start and end with same node: %v", result.Path)
	}
}

// TestDetectCycleFrom_SelfReference tests detection of a self-referencing node.
// A -> A (self-cycle!)
func TestDetectCycleFrom_SelfReference(t *testing.T) {
	p := newMockProvider()
	// Create node that depends on itself
	p.addNode("1.1", "1.1")
	// Root
	p.addNode("1")

	id, _ := types.Parse("1.1")
	result := cycle.DetectCycleFrom(p, id)

	if !result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=false for self-reference A->A")
	}

	// Path should show self-cycle: [1.1, 1.1]
	if len(result.Path) < 2 {
		t.Errorf("DetectCycleFrom() Path too short for self-reference: %v", result.Path)
	}
}

// TestDetectCycleFrom_DiamondPattern tests that diamond dependency patterns are not cycles.
//
//	  A
//	 / \
//	B   C
//	 \ /
//	  D
//
// A depends on B and C; B and C both depend on D (no cycle)
func TestDetectCycleFrom_DiamondPattern(t *testing.T) {
	p := newMockProvider()
	// D has no deps
	p.addNode("1.4")
	// B depends on D
	p.addNode("1.2", "1.4")
	// C depends on D
	p.addNode("1.3", "1.4")
	// A depends on B and C
	p.addNode("1.1", "1.2", "1.3")
	// Root
	p.addNode("1")

	id, _ := types.Parse("1.1")
	result := cycle.DetectCycleFrom(p, id)

	if result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=true for diamond pattern (no cycle): %v", result.Path)
	}
	if len(result.Path) != 0 {
		t.Errorf("DetectCycleFrom() returned non-empty Path %v for diamond pattern", result.Path)
	}
}

// TestDetectCycleFrom_NodeNotFound tests cycle detection when starting node doesn't exist.
func TestDetectCycleFrom_NodeNotFound(t *testing.T) {
	p := newMockProvider()
	p.addNode("1")
	// 1.1 does not exist

	id, _ := types.Parse("1.1")
	result := cycle.DetectCycleFrom(p, id)

	// Non-existent node should not report a cycle
	if result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=true for non-existent node")
	}
	if len(result.Path) != 0 {
		t.Errorf("DetectCycleFrom() returned non-empty Path %v for non-existent node", result.Path)
	}
}

// TestDetectCycleFrom_DependencyNotFound tests cycle detection when a dependency doesn't exist.
func TestDetectCycleFrom_DependencyNotFound(t *testing.T) {
	p := newMockProvider()
	// 1.1 depends on 1.2, but 1.2 doesn't exist
	p.addNode("1.1", "1.2")
	p.addNode("1")

	id, _ := types.Parse("1.1")
	// Missing dependency should not cause a cycle (it's a broken reference, not a cycle)
	result := cycle.DetectCycleFrom(p, id)

	if result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=true when dependency is missing")
	}
	if len(result.Path) != 0 {
		t.Errorf("DetectCycleFrom() returned non-empty Path %v when dependency is missing", result.Path)
	}
}

// TestDetectCycleFrom_ComplexWithNestedCycle tests a complex graph with a nested cycle.
//
//	    1.1
//	   / | \
//	1.2 1.3 1.4
//	     |   |
//	    1.5  |
//	     \--/
//	    (1.5 -> 1.4 -> 1.5 is a cycle)
func TestDetectCycleFrom_ComplexWithNestedCycle(t *testing.T) {
	p := newMockProvider()

	// Create the graph with a cycle between 1.4 and 1.5
	p.addNode("1.1", "1.2", "1.3", "1.4")
	p.addNode("1.2")
	p.addNode("1.3", "1.5")
	p.addNode("1.4", "1.5")
	p.addNode("1.5", "1.4") // Creates cycle 1.4 <-> 1.5
	p.addNode("1")

	// Starting from 1.1, should detect the nested cycle
	id, _ := types.Parse("1.1")
	result := cycle.DetectCycleFrom(p, id)

	if !result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=false for graph with nested cycle")
	}
	if len(result.Path) < 2 {
		t.Errorf("DetectCycleFrom() Path too short: %v", result.Path)
	}
}

// TestDetectCycleFrom_CycleNotReachableFromStart tests starting from a node that doesn't reach the cycle.
//
//	1.1 -> 1.2 (no cycle in this path)
//	1.3 -> 1.4 -> 1.3 (cycle exists but unreachable from 1.1)
func TestDetectCycleFrom_CycleNotReachableFromStart(t *testing.T) {
	p := newMockProvider()

	// Path from 1.1 (no cycle)
	p.addNode("1.1", "1.2")
	p.addNode("1.2")

	// Separate cycle (not reachable from 1.1)
	p.addNode("1.3", "1.4")
	p.addNode("1.4", "1.3")

	p.addNode("1")

	id, _ := types.Parse("1.1")
	result := cycle.DetectCycleFrom(p, id)

	if result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=true when cycle is not reachable from start node")
	}
	if len(result.Path) != 0 {
		t.Errorf("DetectCycleFrom() returned non-empty Path %v when cycle is not reachable", result.Path)
	}
}

// TestDetectCycleFrom_MultipleDependencies tests node with multiple dependencies, one leading to cycle.
func TestDetectCycleFrom_MultipleDependencies(t *testing.T) {
	p := newMockProvider()

	// 1.1 depends on 1.2 and 1.3
	// 1.2 is fine (no cycle)
	// 1.3 leads to a cycle
	p.addNode("1.1", "1.2", "1.3")
	p.addNode("1.2")
	p.addNode("1.3", "1.4")
	p.addNode("1.4", "1.3") // Cycle: 1.3 -> 1.4 -> 1.3
	p.addNode("1")

	id, _ := types.Parse("1.1")
	result := cycle.DetectCycleFrom(p, id)

	if !result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=false when one of multiple dependencies has a cycle")
	}
	if len(result.Path) < 2 {
		t.Errorf("DetectCycleFrom() Path too short: %v", result.Path)
	}
}

// TestDetectCycleFrom_CyclePathContents verifies the cycle path contains expected nodes.
func TestDetectCycleFrom_CyclePathContents(t *testing.T) {
	p := newMockProvider()

	// Create simple cycle: 1.1 -> 1.2 -> 1.3 -> 1.1
	p.addNode("1.1", "1.2")
	p.addNode("1.2", "1.3")
	p.addNode("1.3", "1.1")
	p.addNode("1")

	id, _ := types.Parse("1.1")
	result := cycle.DetectCycleFrom(p, id)

	if !result.HasCycle {
		t.Fatalf("DetectCycleFrom() should detect cycle")
	}

	// Build a set of nodes in the path
	pathSet := make(map[string]bool)
	for _, nodeID := range result.Path {
		pathSet[nodeID.String()] = true
	}

	// All cycle nodes should be in the path
	expectedNodes := []string{"1.1", "1.2", "1.3"}
	for _, expected := range expectedNodes {
		if !pathSet[expected] {
			t.Errorf("DetectCycleFrom() Path missing expected node %s: got %v", expected, result.Path)
		}
	}
}

// TestDetectCycleFrom_DeepChainNoCycle tests a very deep dependency chain without cycles.
func TestDetectCycleFrom_DeepChainNoCycle(t *testing.T) {
	p := newMockProvider()

	// Create a chain: 1 -> 1.1 -> 1.1.1 -> 1.1.1.1 -> ...
	p.addNode("1")
	p.addNode("1.1", "1")
	p.addNode("1.1.1", "1.1")
	p.addNode("1.1.1.1", "1.1.1")
	p.addNode("1.1.1.1.1", "1.1.1.1")
	p.addNode("1.1.1.1.1.1", "1.1.1.1.1")
	p.addNode("1.1.1.1.1.1.1", "1.1.1.1.1.1")
	p.addNode("1.1.1.1.1.1.1.1", "1.1.1.1.1.1.1")

	id, _ := types.Parse("1.1.1.1.1.1.1.1")
	result := cycle.DetectCycleFrom(p, id)

	if result.HasCycle {
		t.Errorf("DetectCycleFrom() returned HasCycle=true for deep chain without cycle: %v", result.Path)
	}
}

// ===========================================================================
// DetectAllCycles tests - checks ALL nodes in provider for cycles
// ===========================================================================

// TestDetectAllCycles_EmptyProvider tests validation on empty provider.
func TestDetectAllCycles_EmptyProvider(t *testing.T) {
	p := newMockProvider()

	cycles := cycle.DetectAllCycles(p)

	if len(cycles) != 0 {
		t.Errorf("DetectAllCycles() returned %d cycles for empty provider, want 0", len(cycles))
	}
}

// TestDetectAllCycles_NoCycles tests validation when no cycles exist.
func TestDetectAllCycles_NoCycles(t *testing.T) {
	p := newMockProvider()

	p.addNode("1")
	p.addNode("1.1", "1")
	p.addNode("1.2", "1")
	p.addNode("1.1.1", "1.1")

	cycles := cycle.DetectAllCycles(p)

	if len(cycles) != 0 {
		t.Errorf("DetectAllCycles() returned %d cycles, want 0", len(cycles))
	}
}

// TestDetectAllCycles_SingleCycle tests detection of a single cycle.
func TestDetectAllCycles_SingleCycle(t *testing.T) {
	p := newMockProvider()

	p.addNode("1")
	p.addNode("1.1", "1.2")
	p.addNode("1.2", "1.1")

	cycles := cycle.DetectAllCycles(p)

	if len(cycles) == 0 {
		t.Errorf("DetectAllCycles() returned 0 cycles, expected at least 1")
	}
}

// TestDetectAllCycles_MultipleSeparateCycles tests detection of multiple independent cycles.
func TestDetectAllCycles_MultipleSeparateCycles(t *testing.T) {
	p := newMockProvider()

	p.addNode("1")

	// Cycle 1: 1.1 <-> 1.2
	p.addNode("1.1", "1.2")
	p.addNode("1.2", "1.1")

	// Cycle 2: 1.3 -> 1.4 -> 1.5 -> 1.3
	p.addNode("1.3", "1.4")
	p.addNode("1.4", "1.5")
	p.addNode("1.5", "1.3")

	// Non-cyclic node
	p.addNode("1.6")

	cycles := cycle.DetectAllCycles(p)

	// Should detect at least 2 distinct cycles
	if len(cycles) < 2 {
		t.Errorf("DetectAllCycles() returned %d cycles, expected at least 2", len(cycles))
	}
}

// TestDetectAllCycles_SelfCycle tests detection of self-referencing nodes.
func TestDetectAllCycles_SelfCycle(t *testing.T) {
	p := newMockProvider()

	p.addNode("1")
	p.addNode("1.1", "1.1") // Self-cycle

	cycles := cycle.DetectAllCycles(p)

	if len(cycles) == 0 {
		t.Errorf("DetectAllCycles() returned 0 cycles for self-referencing node")
	}
}

// TestDetectAllCycles_CyclePathFormat tests that returned cycles have correct format.
func TestDetectAllCycles_CyclePathFormat(t *testing.T) {
	p := newMockProvider()

	p.addNode("1")
	p.addNode("1.1", "1.2")
	p.addNode("1.2", "1.1")

	cycles := cycle.DetectAllCycles(p)

	if len(cycles) == 0 {
		t.Fatalf("DetectAllCycles() should detect cycle")
	}

	// Each cycle should be a valid CycleResult
	for i, c := range cycles {
		if !c.HasCycle {
			t.Errorf("Cycle %d has HasCycle=false", i)
		}
		if len(c.Path) < 2 {
			t.Errorf("Cycle %d is too short: %v", i, c.Path)
		}

		// First and last should be the same (closed cycle)
		if c.Path[0].String() != c.Path[len(c.Path)-1].String() {
			t.Errorf("Cycle %d doesn't close properly: %v", i, c.Path)
		}
	}
}

// ===========================================================================
// Error formatting tests
// ===========================================================================

// TestCycleResult_Error tests the error message generation.
func TestCycleResult_Error(t *testing.T) {
	// No cycle case
	noCycle := cycle.CycleResult{HasCycle: false}
	if noCycle.Error() != "" {
		t.Errorf("CycleResult.Error() should return empty string when no cycle")
	}

	// With cycle
	id1, _ := types.Parse("1.1")
	id2, _ := types.Parse("1.2")
	id3, _ := types.Parse("1.1")
	withCycle := cycle.CycleResult{
		HasCycle: true,
		Path:     []types.NodeID{id1, id2, id3},
	}
	errMsg := withCycle.Error()
	if errMsg == "" {
		t.Errorf("CycleResult.Error() should return non-empty string when cycle exists")
	}
	// Should contain the path nodes
	if !contains(errMsg, "1.1") || !contains(errMsg, "1.2") {
		t.Errorf("CycleResult.Error() should contain path nodes, got: %s", errMsg)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) && (s == substr || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ===========================================================================
// Would-create-cycle validation tests
// ===========================================================================

// TestWouldCreateCycle_NoCycle tests adding a valid dependency.
func TestWouldCreateCycle_NoCycle(t *testing.T) {
	p := newMockProvider()

	p.addNode("1")
	p.addNode("1.1")
	p.addNode("1.2")

	// Adding 1.1 -> 1.2 should not create a cycle
	from, _ := types.Parse("1.1")
	to, _ := types.Parse("1.2")
	result := cycle.WouldCreateCycle(p, from, to)

	if result.HasCycle {
		t.Errorf("WouldCreateCycle() returned HasCycle=true for non-cyclic dependency")
	}
}

// TestWouldCreateCycle_WouldCreate tests detecting a would-be cycle.
func TestWouldCreateCycle_WouldCreate(t *testing.T) {
	p := newMockProvider()

	p.addNode("1")
	p.addNode("1.1", "1.2") // 1.1 depends on 1.2
	p.addNode("1.2", "1.3") // 1.2 depends on 1.3
	p.addNode("1.3")        // 1.3 has no deps

	// Adding 1.3 -> 1.1 would create a cycle: 1.1 -> 1.2 -> 1.3 -> 1.1
	from, _ := types.Parse("1.3")
	to, _ := types.Parse("1.1")
	result := cycle.WouldCreateCycle(p, from, to)

	if !result.HasCycle {
		t.Errorf("WouldCreateCycle() returned HasCycle=false when adding dependency would create cycle")
	}
}

// TestWouldCreateCycle_SelfReference tests self-reference detection.
func TestWouldCreateCycle_SelfReference(t *testing.T) {
	p := newMockProvider()

	p.addNode("1")
	p.addNode("1.1")

	// Adding 1.1 -> 1.1 is a self-cycle
	from, _ := types.Parse("1.1")
	to, _ := types.Parse("1.1")
	result := cycle.WouldCreateCycle(p, from, to)

	if !result.HasCycle {
		t.Errorf("WouldCreateCycle() returned HasCycle=false for self-reference")
	}
}

// TestWouldCreateCycle_MissingNode tests when source node doesn't exist.
func TestWouldCreateCycle_MissingNode(t *testing.T) {
	p := newMockProvider()

	p.addNode("1")
	p.addNode("1.2")

	// 1.1 doesn't exist - adding 1.1 -> 1.2 should return no cycle
	// (since 1.1 has no existing dependencies, can't form a cycle)
	from, _ := types.Parse("1.1")
	to, _ := types.Parse("1.2")
	result := cycle.WouldCreateCycle(p, from, to)

	if result.HasCycle {
		t.Errorf("WouldCreateCycle() returned HasCycle=true when source doesn't exist")
	}
}

// ===========================================================================
// Self-dependency edge case tests
// ===========================================================================

// TestDetectCycle_SelfDependency verifies that cycle detection catches when a node
// depends on itself. This is the simplest form of circular dependency - a single
// node forming a cycle of length 1.
//
// Graph: A -> A (self-loop)
//
// This tests the edge case where the dependency graph contains a self-loop,
// which should be detected as a cycle immediately.
func TestDetectCycle_SelfDependency(t *testing.T) {
	t.Run("DetectCycleFrom detects self-dependency", func(t *testing.T) {
		p := newMockProvider()

		// Create a node that depends on itself
		p.addNode("1.1", "1.1")
		p.addNode("1")

		id, _ := types.Parse("1.1")
		result := cycle.DetectCycleFrom(p, id)

		// Must detect the self-dependency as a cycle
		if !result.HasCycle {
			t.Error("DetectCycleFrom() should detect self-dependency A->A as a cycle")
		}

		// Path should contain at least 2 entries showing the self-loop
		if len(result.Path) < 2 {
			t.Errorf("cycle path should have at least 2 entries for self-dependency: got %v", result.Path)
		}

		// Path should start and end with the same node (the self-referencing node)
		if len(result.Path) >= 2 {
			if result.Path[0].String() != result.Path[len(result.Path)-1].String() {
				t.Errorf("cycle path should start and end with same node: got %v", result.Path)
			}
			// The node in the path should be "1.1"
			if result.Path[0].String() != "1.1" {
				t.Errorf("cycle path should contain node 1.1: got %v", result.Path)
			}
		}

		// Error message should be generated
		errMsg := result.Error()
		if errMsg == "" {
			t.Error("CycleResult.Error() should return non-empty error message for self-dependency")
		}
		if !contains(errMsg, "1.1") {
			t.Errorf("error message should mention the self-dependent node: got %q", errMsg)
		}
	})

	t.Run("DetectAllCycles detects self-dependency", func(t *testing.T) {
		p := newMockProvider()

		// Create a node that depends on itself
		p.addNode("1.1", "1.1")
		p.addNode("1")

		cycles := cycle.DetectAllCycles(p)

		// Must detect at least one cycle
		if len(cycles) == 0 {
			t.Error("DetectAllCycles() should detect self-dependency as a cycle")
		}

		// Find the cycle containing 1.1
		foundSelfLoop := false
		for _, c := range cycles {
			if c.HasCycle {
				for _, nodeID := range c.Path {
					if nodeID.String() == "1.1" {
						foundSelfLoop = true
						break
					}
				}
			}
			if foundSelfLoop {
				break
			}
		}

		if !foundSelfLoop {
			t.Error("DetectAllCycles() should find cycle containing the self-dependent node 1.1")
		}
	})

	t.Run("WouldCreateCycle detects proposed self-dependency", func(t *testing.T) {
		p := newMockProvider()

		// Create a node without self-dependency
		p.addNode("1.1")
		p.addNode("1")

		// Check if adding 1.1 -> 1.1 would create a cycle
		id, _ := types.Parse("1.1")
		result := cycle.WouldCreateCycle(p, id, id)

		if !result.HasCycle {
			t.Error("WouldCreateCycle() should detect proposed self-dependency as a cycle")
		}

		// Path should show the self-reference
		if len(result.Path) < 2 {
			t.Errorf("cycle path should have at least 2 entries: got %v", result.Path)
		}
	})

	t.Run("self-dependency among other dependencies", func(t *testing.T) {
		p := newMockProvider()

		// Create a node that has multiple dependencies including itself
		// This tests that self-dependency is detected even when mixed with valid deps
		p.addNode("1.3") // valid dependency target
		p.addNode("1.2") // valid dependency target
		p.addNode("1.1", "1.2", "1.1", "1.3") // depends on 1.2, itself, and 1.3
		p.addNode("1")

		id, _ := types.Parse("1.1")
		result := cycle.DetectCycleFrom(p, id)

		if !result.HasCycle {
			t.Error("DetectCycleFrom() should detect self-dependency even among other valid dependencies")
		}
	})
}
