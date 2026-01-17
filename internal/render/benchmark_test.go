package render

import (
	"fmt"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// BenchmarkTreeRendering benchmarks rendering proof trees of various sizes.
func BenchmarkTreeRendering(b *testing.B) {
	benchmarks := []struct {
		name      string
		nodeCount int
	}{
		{"10_nodes", 10},
		{"100_nodes", 100},
		{"500_nodes", 500},
		{"1000_nodes", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			s := buildTestTree(b, bm.nodeCount)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = RenderTree(s, nil)
			}
		})
	}
}

// BenchmarkTreeRenderingSubtree benchmarks rendering subtrees.
func BenchmarkTreeRenderingSubtree(b *testing.B) {
	// Build a 500-node tree
	s := buildTestTree(b, 500)

	// Pick a node in the middle of the tree for subtree rendering
	subtreeRoot := mustParseNodeIDForBench(b, "1.5")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderTree(s, &subtreeRoot)
	}
}

// BenchmarkTreeRenderingForNodes benchmarks flat node list rendering.
func BenchmarkTreeRenderingForNodes(b *testing.B) {
	benchmarks := []struct {
		name      string
		nodeCount int
	}{
		{"10_nodes", 10},
		{"100_nodes", 100},
		{"500_nodes", 500},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			s := buildTestTree(b, bm.nodeCount)
			nodes := s.AllNodes()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = RenderTreeForNodes(s, nodes)
			}
		})
	}
}

// BenchmarkFindChildren benchmarks the findChildren function directly.
func BenchmarkFindChildren(b *testing.B) {
	benchmarks := []struct {
		name      string
		nodeCount int
	}{
		{"100_nodes", 100},
		{"1000_nodes", 1000},
		{"5000_nodes", 5000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			s := buildTestTree(b, bm.nodeCount)
			allNodes := s.AllNodes()
			parentID := mustParseNodeIDForBench(b, "1")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = findChildren(parentID, allNodes, nil)
			}
		})
	}
}

// BenchmarkSortNodesByID benchmarks the sortNodesByID function.
func BenchmarkSortNodesByID(b *testing.B) {
	benchmarks := []struct {
		name      string
		nodeCount int
	}{
		{"10_nodes", 10},
		{"100_nodes", 100},
		{"1000_nodes", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			s := buildTestTree(b, bm.nodeCount)
			nodes := s.AllNodes()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Make a copy since sort modifies in place
				nodesCopy := make([]*node.Node, len(nodes))
				copy(nodesCopy, nodes)
				sortNodesByID(nodesCopy)
			}
		})
	}
}

// BenchmarkFormatNode benchmarks single node formatting.
func BenchmarkFormatNode(b *testing.B) {
	n := createTestNode(b, "1", "This is a test mathematical statement with some complexity")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = formatNode(n)
	}
}

// BenchmarkFormatNodeWithState benchmarks node formatting with state context.
func BenchmarkFormatNodeWithState(b *testing.B) {
	s := state.NewState()
	n := createTestNode(b, "1", "This is a test mathematical statement with some complexity")
	s.AddNode(n)

	// Add validation deps
	depNode := createTestNode(b, "1.1", "Dependency node")
	s.AddNode(depNode)
	n.ValidationDeps = []types.NodeID{depNode.ID}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = formatNodeWithState(n, s)
	}
}

// Helper functions

func mustParseNodeIDForBench(b *testing.B, s string) types.NodeID {
	b.Helper()
	id, err := types.Parse(s)
	if err != nil {
		b.Fatalf("Failed to parse node ID %q: %v", s, err)
	}
	return id
}

func createTestNode(b *testing.B, idStr, statement string) *node.Node {
	b.Helper()
	id := mustParseNodeIDForBench(b, idStr)
	n, err := node.NewNode(id, schema.NodeTypeClaim, statement, schema.InferenceAssumption)
	if err != nil {
		b.Fatalf("Failed to create test node: %v", err)
	}
	return n
}

// buildTestTree creates a balanced tree with the specified number of nodes.
func buildTestTree(b *testing.B, nodeCount int) *state.State {
	b.Helper()

	s := state.NewState()

	// Create root
	rootID := mustParseNodeIDForBench(b, "1")
	root, err := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim for benchmark test", schema.InferenceAssumption)
	if err != nil {
		b.Fatalf("Failed to create root: %v", err)
	}
	s.AddNode(root)

	// Build tree breadth-first with branching factor of 10
	created := 1
	queue := []types.NodeID{rootID}
	childNum := make(map[string]int) // Track child counts per parent

	for created < nodeCount && len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]

		// Add up to 10 children to each parent
		for i := 1; i <= 10 && created < nodeCount; i++ {
			childNum[parentID.String()]++
			childID, err := parentID.Child(childNum[parentID.String()])
			if err != nil {
				b.Fatalf("Failed to create child ID: %v", err)
			}

			n, err := node.NewNode(childID, schema.NodeTypeClaim, fmt.Sprintf("Claim %d for benchmark", created), schema.InferenceAssumption)
			if err != nil {
				b.Fatalf("Failed to create node %s: %v", childID, err)
			}
			s.AddNode(n)
			created++

			queue = append(queue, childID)
		}
	}

	return s
}
