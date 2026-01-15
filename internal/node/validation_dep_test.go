// Package node provides data structures for proof nodes in the AF system.
package node

import (
	"testing"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestNode_ValidationDeps tests that nodes can have validation dependencies.
func TestNode_ValidationDeps(t *testing.T) {
	id, _ := types.Parse("1.1")
	dep1, _ := types.Parse("1.2")
	dep2, _ := types.Parse("1.3")

	opts := NodeOptions{
		ValidationDeps: []types.NodeID{dep1, dep2},
	}

	node, err := NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, opts)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if len(node.ValidationDeps) != 2 {
		t.Errorf("expected 2 validation deps, got %d", len(node.ValidationDeps))
	}

	// Verify the deps are correct
	if node.ValidationDeps[0].String() != "1.2" {
		t.Errorf("expected first validation dep to be 1.2, got %s", node.ValidationDeps[0].String())
	}
	if node.ValidationDeps[1].String() != "1.3" {
		t.Errorf("expected second validation dep to be 1.3, got %s", node.ValidationDeps[1].String())
	}
}

// TestNode_ValidationDepsEmpty tests that nodes work without validation dependencies.
func TestNode_ValidationDepsEmpty(t *testing.T) {
	id, _ := types.Parse("1.1")

	node, err := NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if len(node.ValidationDeps) != 0 {
		t.Errorf("expected 0 validation deps for new node, got %d", len(node.ValidationDeps))
	}
}

// TestNode_BothDepsAndValidationDeps tests that nodes can have both regular deps and validation deps.
func TestNode_BothDepsAndValidationDeps(t *testing.T) {
	id, _ := types.Parse("1.5")
	refDep, _ := types.Parse("1.1")
	valDep1, _ := types.Parse("1.2")
	valDep2, _ := types.Parse("1.3")

	opts := NodeOptions{
		Dependencies:   []types.NodeID{refDep},
		ValidationDeps: []types.NodeID{valDep1, valDep2},
	}

	node, err := NewNodeWithOptions(id, schema.NodeTypeClaim, "Combined statement", schema.InferenceModusPonens, opts)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Check regular dependencies
	if len(node.Dependencies) != 1 {
		t.Errorf("expected 1 regular dep, got %d", len(node.Dependencies))
	}
	if node.Dependencies[0].String() != "1.1" {
		t.Errorf("expected regular dep to be 1.1, got %s", node.Dependencies[0].String())
	}

	// Check validation dependencies
	if len(node.ValidationDeps) != 2 {
		t.Errorf("expected 2 validation deps, got %d", len(node.ValidationDeps))
	}
}

// TestNode_ContentHashIncludesValidationDeps tests that validation deps are included in content hash.
func TestNode_ContentHashIncludesValidationDeps(t *testing.T) {
	id, _ := types.Parse("1.1")
	dep1, _ := types.Parse("1.2")

	// Node without validation deps
	node1, err := NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node1: %v", err)
	}

	// Node with validation deps
	opts := NodeOptions{
		ValidationDeps: []types.NodeID{dep1},
	}
	node2, err := NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, opts)
	if err != nil {
		t.Fatalf("failed to create node2: %v", err)
	}

	// Content hashes should be different
	if node1.ContentHash == node2.ContentHash {
		t.Error("content hashes should differ when validation deps differ")
	}
}

// TestValidateValidationDepExistence tests validation of validation dependencies.
func TestValidateValidationDepExistence(t *testing.T) {
	// Create a mock state lookup
	lookup := &mockNodeLookup{
		nodes: make(map[string]*Node),
	}

	// Add some nodes to the lookup
	node1, _ := NewNode(mustParse(t, "1"), schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
	node2, _ := NewNode(mustParse(t, "1.1"), schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
	lookup.nodes["1"] = node1
	lookup.nodes["1.1"] = node2

	// Test node with valid validation dep
	validNode, _ := NewNodeWithOptions(mustParse(t, "1.2"), schema.NodeTypeClaim, "Child 2", schema.InferenceAssumption, NodeOptions{
		ValidationDeps: []types.NodeID{mustParse(t, "1.1")},
	})
	if err := ValidateValidationDepExistence(validNode, lookup); err != nil {
		t.Errorf("expected no error for valid validation dep, got: %v", err)
	}

	// Test node with invalid validation dep
	invalidNode, _ := NewNodeWithOptions(mustParse(t, "1.3"), schema.NodeTypeClaim, "Child 3", schema.InferenceAssumption, NodeOptions{
		ValidationDeps: []types.NodeID{mustParse(t, "1.99")}, // doesn't exist
	})
	if err := ValidateValidationDepExistence(invalidNode, lookup); err == nil {
		t.Error("expected error for non-existent validation dep, got nil")
	}
}

// TestValidateValidationDepExistence_NilInputs tests error handling for nil inputs.
func TestValidateValidationDepExistence_NilInputs(t *testing.T) {
	lookup := &mockNodeLookup{nodes: make(map[string]*Node)}
	node, _ := NewNode(mustParse(t, "1"), schema.NodeTypeClaim, "Test", schema.InferenceAssumption)

	// Test nil node
	if err := ValidateValidationDepExistence(nil, lookup); err == nil {
		t.Error("expected error for nil node")
	}

	// Test nil lookup
	if err := ValidateValidationDepExistence(node, nil); err == nil {
		t.Error("expected error for nil lookup")
	}
}

// TestValidateValidationDepExistence_EmptyDeps tests that empty deps is valid.
func TestValidateValidationDepExistence_EmptyDeps(t *testing.T) {
	lookup := &mockNodeLookup{nodes: make(map[string]*Node)}
	node, _ := NewNode(mustParse(t, "1"), schema.NodeTypeClaim, "Test", schema.InferenceAssumption)

	// No validation deps should pass
	if err := ValidateValidationDepExistence(node, lookup); err != nil {
		t.Errorf("expected no error for empty validation deps, got: %v", err)
	}
}

// mockNodeLookup is a test helper that implements NodeLookup.
type mockNodeLookup struct {
	nodes map[string]*Node
}

func (m *mockNodeLookup) GetNode(id types.NodeID) *Node {
	return m.nodes[id.String()]
}

// mustParse is a test helper to parse NodeID or fail.
func mustParse(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("failed to parse %q: %v", s, err)
	}
	return id
}
