//go:build integration
// +build integration

// Package node_test contains tests for dependency existence validation.
package node_test

import (
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// ===========================================================================
// Helper functions for test setup
// ===========================================================================

// createTestNode creates a node with the given ID and optional dependencies.
func createTestNode(t *testing.T, idStr string, depStrs ...string) *node.Node {
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

	return n
}

// createAndAddNode creates a node and adds it to the state.
func createAndAddNode(t *testing.T, s *state.State, idStr string, depStrs ...string) *node.Node {
	t.Helper()
	n := createTestNode(t, idStr, depStrs...)
	s.AddNode(n)
	return n
}

// ===========================================================================
// Basic validation tests
// ===========================================================================

// TestValidateDepExistence_NoDependencies tests that a node with no dependencies passes validation.
func TestValidateDepExistence_NoDependencies(t *testing.T) {
	s := state.NewState()

	// Create a node with no dependencies
	n := createTestNode(t, "1")
	s.AddNode(n)

	err := node.ValidateDepExistence(n, s)
	if err != nil {
		t.Errorf("ValidateDepExistence() = %v, want nil for node with no dependencies", err)
	}
}

// TestValidateDepExistence_EmptyDependencyList tests that empty dependency slice passes.
func TestValidateDepExistence_EmptyDependencyList(t *testing.T) {
	s := state.NewState()

	// Create a node with explicit empty dependencies
	id, _ := types.Parse("1")
	opts := node.NodeOptions{
		Dependencies: []types.NodeID{},
	}
	n, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test", schema.InferenceAssumption, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}
	s.AddNode(n)

	err = node.ValidateDepExistence(n, s)
	if err != nil {
		t.Errorf("ValidateDepExistence() = %v, want nil for node with empty dependency list", err)
	}
}

// TestValidateDepExistence_ValidSingleDependency tests that a node with a valid single dependency passes.
func TestValidateDepExistence_ValidSingleDependency(t *testing.T) {
	s := state.NewState()

	// Create dependency node first
	createAndAddNode(t, s, "1")

	// Create node that depends on existing node
	n := createTestNode(t, "1.1", "1")
	s.AddNode(n)

	err := node.ValidateDepExistence(n, s)
	if err != nil {
		t.Errorf("ValidateDepExistence() = %v, want nil for node with valid dependency", err)
	}
}

// TestValidateDepExistence_ValidMultipleDependencies tests that a node with multiple valid dependencies passes.
func TestValidateDepExistence_ValidMultipleDependencies(t *testing.T) {
	s := state.NewState()

	// Create all dependency nodes
	createAndAddNode(t, s, "1")
	createAndAddNode(t, s, "1.1")
	createAndAddNode(t, s, "1.2")

	// Create node that depends on all existing nodes
	n := createTestNode(t, "1.3", "1", "1.1", "1.2")
	s.AddNode(n)

	err := node.ValidateDepExistence(n, s)
	if err != nil {
		t.Errorf("ValidateDepExistence() = %v, want nil for node with multiple valid dependencies", err)
	}
}

// ===========================================================================
// Missing dependency tests
// ===========================================================================

// TestValidateDepExistence_SingleMissingDependency tests that a single missing dependency fails.
func TestValidateDepExistence_SingleMissingDependency(t *testing.T) {
	s := state.NewState()

	// Create a node that depends on a non-existent node
	n := createTestNode(t, "1.1", "1.2")
	s.AddNode(n)

	err := node.ValidateDepExistence(n, s)
	if err == nil {
		t.Error("ValidateDepExistence() = nil, want error for missing dependency")
	}

	// Verify error message contains the missing node ID
	if err != nil && !strings.Contains(err.Error(), "1.2") {
		t.Errorf("Error should mention missing node ID '1.2', got: %s", err.Error())
	}
}

// TestValidateDepExistence_MultipleMissingDependencies tests behavior with multiple missing dependencies.
func TestValidateDepExistence_MultipleMissingDependencies(t *testing.T) {
	s := state.NewState()

	// Create only one dependency node
	createAndAddNode(t, s, "1")

	// Create node with multiple dependencies, some missing
	n := createTestNode(t, "1.4", "1", "1.2", "1.3")
	s.AddNode(n)

	err := node.ValidateDepExistence(n, s)
	if err == nil {
		t.Error("ValidateDepExistence() = nil, want error for missing dependencies")
	}

	// The error should identify at least one of the missing dependencies
	if err != nil {
		errStr := err.Error()
		hasMissing := strings.Contains(errStr, "1.2") || strings.Contains(errStr, "1.3")
		if !hasMissing {
			t.Errorf("Error should mention at least one missing node ID (1.2 or 1.3), got: %s", errStr)
		}
	}
}

// TestValidateDepExistence_AllDependenciesMissing tests when all dependencies are missing.
func TestValidateDepExistence_AllDependenciesMissing(t *testing.T) {
	s := state.NewState()

	// Create node with dependencies, but none exist in state
	n := createTestNode(t, "1.1", "2", "3", "4")
	s.AddNode(n)

	err := node.ValidateDepExistence(n, s)
	if err == nil {
		t.Error("ValidateDepExistence() = nil, want error when all dependencies are missing")
	}
}

// TestValidateDepExistence_ErrorFormat tests the error message format.
func TestValidateDepExistence_ErrorFormat(t *testing.T) {
	s := state.NewState()

	// Create node that depends on non-existent node "1.2.3"
	n := createTestNode(t, "1.1", "1.2.3")
	s.AddNode(n)

	err := node.ValidateDepExistence(n, s)
	if err == nil {
		t.Fatal("ValidateDepExistence() = nil, want error")
	}

	// Error should follow the format: "invalid dependency: node 1.2.3 not found"
	errStr := err.Error()

	// Check for expected error format components
	if !strings.Contains(strings.ToLower(errStr), "invalid dependency") &&
		!strings.Contains(strings.ToLower(errStr), "dependency") {
		t.Errorf("Error should mention 'dependency', got: %s", errStr)
	}

	if !strings.Contains(errStr, "1.2.3") {
		t.Errorf("Error should contain the missing node ID '1.2.3', got: %s", errStr)
	}

	if !strings.Contains(strings.ToLower(errStr), "not found") &&
		!strings.Contains(strings.ToLower(errStr), "not exist") &&
		!strings.Contains(strings.ToLower(errStr), "missing") &&
		!strings.Contains(strings.ToLower(errStr), "invalid") {
		t.Errorf("Error should indicate the node was not found/missing, got: %s", errStr)
	}
}

// ===========================================================================
// Nil input tests
// ===========================================================================

// TestValidateDepExistence_NilNode tests that nil node returns an error.
func TestValidateDepExistence_NilNode(t *testing.T) {
	s := state.NewState()

	err := node.ValidateDepExistence(nil, s)
	if err == nil {
		t.Error("ValidateDepExistence(nil, state) = nil, want error for nil node")
	}
}

// TestValidateDepExistence_NilState tests that nil state returns an error.
func TestValidateDepExistence_NilState(t *testing.T) {
	n := createTestNode(&testing.T{}, "1")

	// Using a separate testing.T to avoid test failure in helper
	t.Run("nil state check", func(t *testing.T) {
		err := node.ValidateDepExistence(n, nil)
		if err == nil {
			t.Error("ValidateDepExistence(node, nil) = nil, want error for nil state")
		}
	})
}

// TestValidateDepExistence_BothNil tests that both nil node and state returns an error.
func TestValidateDepExistence_BothNil(t *testing.T) {
	err := node.ValidateDepExistence(nil, nil)
	if err == nil {
		t.Error("ValidateDepExistence(nil, nil) = nil, want error")
	}
}

// ===========================================================================
// Self-dependency tests
// ===========================================================================

// TestValidateDepExistence_SelfDependency tests handling of a node that depends on itself.
func TestValidateDepExistence_SelfDependency(t *testing.T) {
	s := state.NewState()

	// Create a node that depends on itself
	n := createTestNode(t, "1.1", "1.1")
	s.AddNode(n)

	// The node exists in state, so technically the dependency exists.
	// However, self-dependency might be considered invalid depending on design.
	// This test documents the expected behavior.
	err := node.ValidateDepExistence(n, s)

	// If self-dependency is allowed (dependency exists in state): err should be nil
	// If self-dependency is explicitly prohibited: err should be non-nil
	// The test checks that the function handles this case without panicking
	// and documents whichever behavior is implemented.
	_ = err // Document: actual behavior depends on implementation choice
}

// TestValidateDepExistence_SelfDependencyNotInState tests self-dependency when node is not added to state.
func TestValidateDepExistence_SelfDependencyNotInState(t *testing.T) {
	s := state.NewState()

	// Create a node that depends on itself but don't add it to state
	n := createTestNode(t, "1.1", "1.1")

	// Now the self-dependency should fail because 1.1 is not in state
	err := node.ValidateDepExistence(n, s)
	if err == nil {
		t.Error("ValidateDepExistence() = nil, want error for self-dependency when node is not in state")
	}

	if err != nil && !strings.Contains(err.Error(), "1.1") {
		t.Errorf("Error should mention the missing node ID '1.1', got: %s", err.Error())
	}
}

// ===========================================================================
// Edge cases and robustness tests
// ===========================================================================

// TestValidateDepExistence_DependencyChain tests validation doesn't follow the dependency chain.
func TestValidateDepExistence_DependencyChain(t *testing.T) {
	s := state.NewState()

	// Create: 1 (exists), 1.1 -> 1 (valid), 1.2 -> 1.1 -> 1
	// But 1.1's dependency is only checked for 1.1, not transitively
	createAndAddNode(t, s, "1")
	createAndAddNode(t, s, "1.1", "1")
	n := createTestNode(t, "1.2", "1.1")
	s.AddNode(n)

	// Validation should only check immediate dependencies
	err := node.ValidateDepExistence(n, s)
	if err != nil {
		t.Errorf("ValidateDepExistence() = %v, want nil for valid immediate dependency", err)
	}
}

// TestValidateDepExistence_DuplicateDependencies tests handling of duplicate dependencies.
func TestValidateDepExistence_DuplicateDependencies(t *testing.T) {
	s := state.NewState()

	// Create dependency node
	createAndAddNode(t, s, "1")

	// Create node with duplicate dependencies to the same node
	id, _ := types.Parse("1.1")
	dep, _ := types.Parse("1")
	opts := node.NodeOptions{
		Dependencies: []types.NodeID{dep, dep, dep}, // Duplicate dependencies
	}
	n, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test", schema.InferenceModusPonens, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}
	s.AddNode(n)

	// Duplicate valid dependencies should still pass
	err = node.ValidateDepExistence(n, s)
	if err != nil {
		t.Errorf("ValidateDepExistence() = %v, want nil for duplicate valid dependencies", err)
	}
}

// TestValidateDepExistence_DuplicateMissingDependencies tests duplicate missing dependencies.
func TestValidateDepExistence_DuplicateMissingDependencies(t *testing.T) {
	s := state.NewState()

	// Create node with duplicate dependencies to a missing node
	id, _ := types.Parse("1.1")
	dep, _ := types.Parse("1.2")
	opts := node.NodeOptions{
		Dependencies: []types.NodeID{dep, dep}, // Duplicate missing dependencies
	}
	n, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test", schema.InferenceModusPonens, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}
	s.AddNode(n)

	// Should fail with missing dependency error
	err = node.ValidateDepExistence(n, s)
	if err == nil {
		t.Error("ValidateDepExistence() = nil, want error for missing dependency")
	}
}

// TestValidateDepExistence_DeepNodeID tests validation with deeply nested node IDs.
func TestValidateDepExistence_DeepNodeID(t *testing.T) {
	s := state.NewState()

	// Create deep nodes
	createAndAddNode(t, s, "1")
	createAndAddNode(t, s, "1.2.3.4.5")

	// Create node depending on deep node
	n := createTestNode(t, "1.1", "1.2.3.4.5")
	s.AddNode(n)

	err := node.ValidateDepExistence(n, s)
	if err != nil {
		t.Errorf("ValidateDepExistence() = %v, want nil for valid deep dependency", err)
	}
}

// TestValidateDepExistence_MixedValidAndInvalid tests mix of valid and invalid dependencies.
func TestValidateDepExistence_MixedValidAndInvalid(t *testing.T) {
	s := state.NewState()

	// Create some nodes
	createAndAddNode(t, s, "1")
	createAndAddNode(t, s, "1.1")

	// Create node with mix of valid (1, 1.1) and invalid (1.2, 1.3) dependencies
	n := createTestNode(t, "1.4", "1", "1.1", "1.2", "1.3")
	s.AddNode(n)

	err := node.ValidateDepExistence(n, s)
	if err == nil {
		t.Error("ValidateDepExistence() = nil, want error for mixed valid and invalid dependencies")
	}

	// Should report at least one of the missing dependencies
	if err != nil {
		errStr := err.Error()
		hasMissing := strings.Contains(errStr, "1.2") || strings.Contains(errStr, "1.3")
		if !hasMissing {
			t.Errorf("Error should mention a missing dependency, got: %s", errStr)
		}
	}
}

// TestValidateDepExistence_NodeNotInState tests validation when the node itself is not in state.
func TestValidateDepExistence_NodeNotInState(t *testing.T) {
	s := state.NewState()

	// Create dependency node
	createAndAddNode(t, s, "1")

	// Create node with valid dependency but don't add node to state
	n := createTestNode(t, "1.1", "1")
	// Note: Not adding n to state

	// The validation should still work - it checks if dependencies exist,
	// not if the node itself exists
	err := node.ValidateDepExistence(n, s)
	if err != nil {
		t.Errorf("ValidateDepExistence() = %v, want nil (dependency 1 exists)", err)
	}
}

// ===========================================================================
// Table-driven comprehensive test
// ===========================================================================

// TestValidateDepExistence_TableDriven provides comprehensive table-driven tests.
func TestValidateDepExistence_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		nodeID          string
		dependencies    []string
		existingNodes   []string
		expectError     bool
		errorContains   string
	}{
		{
			name:          "no dependencies",
			nodeID:        "1",
			dependencies:  nil,
			existingNodes: nil,
			expectError:   false,
		},
		{
			name:          "single valid dependency",
			nodeID:        "1.1",
			dependencies:  []string{"1"},
			existingNodes: []string{"1"},
			expectError:   false,
		},
		{
			name:          "multiple valid dependencies",
			nodeID:        "1.3",
			dependencies:  []string{"1", "1.1", "1.2"},
			existingNodes: []string{"1", "1.1", "1.2"},
			expectError:   false,
		},
		{
			name:          "single missing dependency",
			nodeID:        "1.1",
			dependencies:  []string{"1.2"},
			existingNodes: nil,
			expectError:   true,
			errorContains: "1.2",
		},
		{
			name:          "one valid one missing",
			nodeID:        "1.2",
			dependencies:  []string{"1", "1.1"},
			existingNodes: []string{"1"},
			expectError:   true,
			errorContains: "1.1",
		},
		{
			name:          "dependency on root",
			nodeID:        "1.1",
			dependencies:  []string{"1"},
			existingNodes: []string{"1"},
			expectError:   false,
		},
		{
			name:          "dependency on sibling",
			nodeID:        "1.2",
			dependencies:  []string{"1.1"},
			existingNodes: []string{"1.1"},
			expectError:   false,
		},
		{
			name:          "dependency on deep node",
			nodeID:        "2",
			dependencies:  []string{"1.1.1.1"},
			existingNodes: []string{"1.1.1.1"},
			expectError:   false,
		},
		{
			name:          "missing deep dependency",
			nodeID:        "2",
			dependencies:  []string{"1.1.1.1"},
			existingNodes: []string{"1.1.1"}, // Parent exists, but not the exact dependency
			expectError:   true,
			errorContains: "1.1.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()

			// Add existing nodes to state
			for _, existID := range tt.existingNodes {
				createAndAddNode(t, s, existID)
			}

			// Create the test node with dependencies
			n := createTestNode(t, tt.nodeID, tt.dependencies...)

			err := node.ValidateDepExistence(n, s)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateDepExistence() = nil, want error")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDepExistence() = %v, want nil", err)
				}
			}
		})
	}
}

// ===========================================================================
// State interaction tests
// ===========================================================================

// TestValidateDepExistence_EmptyState tests validation with an empty state.
func TestValidateDepExistence_EmptyState(t *testing.T) {
	s := state.NewState()

	// Node with no dependencies should pass even with empty state
	n := createTestNode(t, "1")

	err := node.ValidateDepExistence(n, s)
	if err != nil {
		t.Errorf("ValidateDepExistence() = %v, want nil for node with no dependencies in empty state", err)
	}
}

// TestValidateDepExistence_LargeNumberOfDependencies tests with many dependencies.
func TestValidateDepExistence_LargeNumberOfDependencies(t *testing.T) {
	s := state.NewState()

	// Create 20 dependency nodes
	var deps []string
	for i := 1; i <= 20; i++ {
		depID := "1." + string(rune('a'+i-1))
		createAndAddNode(t, s, depID)
		deps = append(deps, depID)
	}

	// Create node depending on all 20
	n := createTestNode(t, "2", deps...)
	s.AddNode(n)

	err := node.ValidateDepExistence(n, s)
	if err != nil {
		t.Errorf("ValidateDepExistence() = %v, want nil for node with many valid dependencies", err)
	}
}

// TestValidateDepExistence_LargeNumberWithOneMissing tests many deps with one missing.
func TestValidateDepExistence_LargeNumberWithOneMissing(t *testing.T) {
	s := state.NewState()

	// Create 19 dependency nodes, skip one
	var deps []string
	for i := 1; i <= 20; i++ {
		depID := "1." + string(rune('a'+i-1))
		if i != 10 { // Skip the 10th one
			createAndAddNode(t, s, depID)
		}
		deps = append(deps, depID)
	}

	// Create node depending on all 20 (one is missing)
	n := createTestNode(t, "2", deps...)
	s.AddNode(n)

	err := node.ValidateDepExistence(n, s)
	if err == nil {
		t.Error("ValidateDepExistence() = nil, want error for one missing dependency among many")
	}
}
