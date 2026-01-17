// Package node_test contains edge case tests for empty vs nil dependencies.
package node_test

import (
	"encoding/json"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestNode_EmptyVsNilDependencies tests that empty slice and nil are treated
// consistently across content hash computation, JSON serialization, and validation.
func TestNode_EmptyVsNilDependencies(t *testing.T) {
	id, _ := types.Parse("1.1")

	t.Run("content hash same for nil and empty Dependencies", func(t *testing.T) {
		// Create node with nil dependencies (default from NewNode)
		nNil, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("NewNode() error: %v", err)
		}

		// Create node with explicit empty slice
		optsEmpty := node.NodeOptions{
			Dependencies: []types.NodeID{},
		}
		nEmpty, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, optsEmpty)
		if err != nil {
			t.Fatalf("NewNodeWithOptions() error: %v", err)
		}

		// Content hash should be the same since both have zero dependencies
		if nNil.ContentHash != nEmpty.ContentHash {
			t.Errorf("content hash differs: nil deps = %q, empty deps = %q", nNil.ContentHash, nEmpty.ContentHash)
		}

		// Verify hashes are valid
		if !nNil.VerifyContentHash() {
			t.Error("nil deps node should verify its content hash")
		}
		if !nEmpty.VerifyContentHash() {
			t.Error("empty deps node should verify its content hash")
		}
	})

	t.Run("content hash same for nil and empty ValidationDeps", func(t *testing.T) {
		// Create node with nil validation dependencies
		optsNil := node.NodeOptions{
			ValidationDeps: nil,
		}
		nNil, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, optsNil)
		if err != nil {
			t.Fatalf("NewNodeWithOptions() error: %v", err)
		}

		// Create node with explicit empty slice
		optsEmpty := node.NodeOptions{
			ValidationDeps: []types.NodeID{},
		}
		nEmpty, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, optsEmpty)
		if err != nil {
			t.Fatalf("NewNodeWithOptions() error: %v", err)
		}

		// Content hash should be the same
		if nNil.ContentHash != nEmpty.ContentHash {
			t.Errorf("content hash differs: nil validation deps = %q, empty validation deps = %q", nNil.ContentHash, nEmpty.ContentHash)
		}
	})

	t.Run("content hash same for nil and empty Context", func(t *testing.T) {
		// Create node with nil context
		optsNil := node.NodeOptions{
			Context: nil,
		}
		nNil, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, optsNil)
		if err != nil {
			t.Fatalf("NewNodeWithOptions() error: %v", err)
		}

		// Create node with explicit empty slice
		optsEmpty := node.NodeOptions{
			Context: []string{},
		}
		nEmpty, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, optsEmpty)
		if err != nil {
			t.Fatalf("NewNodeWithOptions() error: %v", err)
		}

		// Content hash should be the same
		if nNil.ContentHash != nEmpty.ContentHash {
			t.Errorf("content hash differs: nil context = %q, empty context = %q", nNil.ContentHash, nEmpty.ContentHash)
		}
	})

	t.Run("JSON serialization handles nil Dependencies", func(t *testing.T) {
		// Create node with nil dependencies
		nNil, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("NewNode() error: %v", err)
		}

		data, err := json.Marshal(nNil)
		if err != nil {
			t.Fatalf("json.Marshal() error: %v", err)
		}

		// Check that nil dependencies is omitted from JSON (omitempty)
		var fields map[string]interface{}
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatalf("json.Unmarshal() to map error: %v", err)
		}

		if _, exists := fields["dependencies"]; exists {
			t.Error("nil dependencies should be omitted from JSON due to omitempty")
		}
	})

	t.Run("JSON serialization handles empty Dependencies", func(t *testing.T) {
		// Create node with explicit empty dependencies
		optsEmpty := node.NodeOptions{
			Dependencies: []types.NodeID{},
		}
		nEmpty, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, optsEmpty)
		if err != nil {
			t.Fatalf("NewNodeWithOptions() error: %v", err)
		}

		data, err := json.Marshal(nEmpty)
		if err != nil {
			t.Fatalf("json.Marshal() error: %v", err)
		}

		// Check JSON structure - empty slice should also be omitted due to omitempty
		var fields map[string]interface{}
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatalf("json.Unmarshal() to map error: %v", err)
		}

		// Note: Go's omitempty treats both nil and empty slice the same for JSON
		if _, exists := fields["dependencies"]; exists {
			t.Error("empty dependencies should be omitted from JSON due to omitempty")
		}
	})

	t.Run("JSON roundtrip preserves nil vs empty semantics", func(t *testing.T) {
		// Create node with nil dependencies
		nNil, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("NewNode() error: %v", err)
		}

		// Marshal and unmarshal
		data, err := json.Marshal(nNil)
		if err != nil {
			t.Fatalf("json.Marshal() error: %v", err)
		}

		var restored node.Node
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("json.Unmarshal() error: %v", err)
		}

		// After round-trip, content hash should still verify
		// (Note: the dependencies field might become nil after unmarshal)
		computed := restored.ComputeContentHash()
		if computed != nNil.ContentHash {
			t.Errorf("content hash changed after roundtrip: original = %q, restored = %q", nNil.ContentHash, computed)
		}
	})

	t.Run("ComputeContentHash consistent after modifications", func(t *testing.T) {
		// Create a node
		n, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("NewNode() error: %v", err)
		}

		originalHash := n.ContentHash

		// Directly set Dependencies to empty slice (simulating potential code path)
		n.Dependencies = []types.NodeID{}

		// Recompute hash - should be the same
		recomputedHash := n.ComputeContentHash()
		if recomputedHash != originalHash {
			t.Errorf("hash changed when setting nil to empty: original = %q, recomputed = %q", originalHash, recomputedHash)
		}

		// Set back to nil
		n.Dependencies = nil

		// Recompute again - should still be the same
		recomputedHash2 := n.ComputeContentHash()
		if recomputedHash2 != originalHash {
			t.Errorf("hash changed when setting empty to nil: original = %q, recomputed = %q", originalHash, recomputedHash2)
		}
	})

	t.Run("all slice fields consistent nil vs empty", func(t *testing.T) {
		dep1, _ := types.Parse("1")

		// Test all combinations of nil vs empty for all slice fields
		testCases := []struct {
			name     string
			opts     node.NodeOptions
			wantHash string // Will be computed from first case
		}{
			{
				name: "all nil",
				opts: node.NodeOptions{
					Context:        nil,
					Dependencies:   nil,
					ValidationDeps: nil,
					Scope:          nil,
				},
			},
			{
				name: "all empty",
				opts: node.NodeOptions{
					Context:        []string{},
					Dependencies:   []types.NodeID{},
					ValidationDeps: []types.NodeID{},
					Scope:          []string{},
				},
			},
			{
				name: "mixed nil and empty",
				opts: node.NodeOptions{
					Context:        nil,
					Dependencies:   []types.NodeID{},
					ValidationDeps: nil,
					Scope:          []string{},
				},
			},
		}

		// Compute expected hash from first case
		firstNode, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, testCases[0].opts)
		if err != nil {
			t.Fatalf("NewNodeWithOptions() error: %v", err)
		}
		expectedHash := firstNode.ContentHash

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				n, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, tc.opts)
				if err != nil {
					t.Fatalf("NewNodeWithOptions() error: %v", err)
				}

				if n.ContentHash != expectedHash {
					t.Errorf("hash mismatch for %s: got %q, want %q", tc.name, n.ContentHash, expectedHash)
				}
			})
		}

		// Now test with actual values - these should differ from empty
		optsWithDeps := node.NodeOptions{
			Dependencies: []types.NodeID{dep1},
		}
		nWithDeps, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, optsWithDeps)
		if err != nil {
			t.Fatalf("NewNodeWithOptions() error: %v", err)
		}

		if nWithDeps.ContentHash == expectedHash {
			t.Error("node with actual dependencies should have different hash than empty")
		}
	})
}

// TestNode_EmptyVsNilDependencies_Validation tests that validation functions
// treat nil and empty dependencies the same.
func TestNode_EmptyVsNilDependencies_Validation(t *testing.T) {
	id, _ := types.Parse("1.1")

	t.Run("ValidateDepExistence nil deps", func(t *testing.T) {
		// Create a mock state that has no nodes (simulating empty state)
		mock := &mockNodeLookup{nodes: make(map[string]*node.Node)}

		nNil, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("NewNode() error: %v", err)
		}

		// Validation should pass since there are no dependencies to check
		if err := node.ValidateDepExistence(nNil, mock); err != nil {
			t.Errorf("ValidateDepExistence() should pass for nil deps: %v", err)
		}
	})

	t.Run("ValidateDepExistence empty deps", func(t *testing.T) {
		mock := &mockNodeLookup{nodes: make(map[string]*node.Node)}

		optsEmpty := node.NodeOptions{
			Dependencies: []types.NodeID{},
		}
		nEmpty, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, optsEmpty)
		if err != nil {
			t.Fatalf("NewNodeWithOptions() error: %v", err)
		}

		// Validation should pass since there are no dependencies to check
		if err := node.ValidateDepExistence(nEmpty, mock); err != nil {
			t.Errorf("ValidateDepExistence() should pass for empty deps: %v", err)
		}
	})

	t.Run("ValidateValidationDepExistence nil vs empty", func(t *testing.T) {
		mock := &mockNodeLookup{nodes: make(map[string]*node.Node)}

		// Nil validation deps
		optsNil := node.NodeOptions{
			ValidationDeps: nil,
		}
		nNil, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, optsNil)
		if err != nil {
			t.Fatalf("NewNodeWithOptions() error: %v", err)
		}

		if err := node.ValidateValidationDepExistence(nNil, mock); err != nil {
			t.Errorf("ValidateValidationDepExistence() should pass for nil validation deps: %v", err)
		}

		// Empty validation deps
		optsEmpty := node.NodeOptions{
			ValidationDeps: []types.NodeID{},
		}
		nEmpty, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption, optsEmpty)
		if err != nil {
			t.Fatalf("NewNodeWithOptions() error: %v", err)
		}

		if err := node.ValidateValidationDepExistence(nEmpty, mock); err != nil {
			t.Errorf("ValidateValidationDepExistence() should pass for empty validation deps: %v", err)
		}
	})
}

// mockNodeLookup implements node.NodeLookup for testing.
type mockNodeLookup struct {
	nodes map[string]*node.Node
}

func (m *mockNodeLookup) GetNode(id types.NodeID) *node.Node {
	return m.nodes[id.String()]
}
