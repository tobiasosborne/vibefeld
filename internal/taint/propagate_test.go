//go:build integration

package taint

// Benchmarks for PropagateTaint. The unit tests that used to live in this
// file were an older copy of those in propagate_unit_test.go (which runs with
// and without the integration tag, and was updated for bottom-up propagation
// in 0.1.7); makeNode and itoa come from that file.

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
)

// buildDeepTree creates a linear chain of nodes: 1 -> 1.1 -> 1.1.1 -> ... (depth levels)
func buildDeepTree(depth int) (*node.Node, []*node.Node) {
	nodes := make([]*node.Node, depth)

	// Build IDs: "1", "1.1", "1.1.1", etc.
	id := "1"
	for i := 0; i < depth; i++ {
		if i > 0 {
			id = id + ".1"
		}
		nodes[i] = makeNode(id, schema.EpistemicValidated, node.TaintClean)
	}

	// Root is admitted (to trigger taint propagation)
	nodes[0].EpistemicState = schema.EpistemicAdmitted
	nodes[0].TaintState = node.TaintSelfAdmitted

	return nodes[0], nodes
}

// buildWideTree creates a tree with one root and many direct children: 1 -> 1.1, 1.2, ..., 1.N
func buildWideTree(width int) (*node.Node, []*node.Node) {
	nodes := make([]*node.Node, width+1)

	// Root
	nodes[0] = makeNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	// Children
	for i := 1; i <= width; i++ {
		id := "1." + string(rune('0'+i%10)) // Simplified ID generation
		if i >= 10 {
			id = "1." + itoa(i)
		}
		nodes[i] = makeNode(id, schema.EpistemicValidated, node.TaintClean)
	}

	return nodes[0], nodes
}

// BenchmarkPropagateTaint_DeepTree benchmarks taint propagation on deep hierarchies.
// This was the problematic case with O(n*d) complexity.
// With the optimization, it should now be O(n).
func BenchmarkPropagateTaint_DeepTree(b *testing.B) {
	depths := []int{10, 50, 100, 200}

	for _, depth := range depths {
		b.Run(itoa(depth)+"_deep", func(b *testing.B) {
			root, allNodes := buildDeepTree(depth)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Reset taint states for each iteration
				for j := 1; j < len(allNodes); j++ {
					allNodes[j].TaintState = node.TaintClean
				}
				PropagateTaint(root, allNodes)
			}
		})
	}
}

// BenchmarkPropagateTaint_WideTree benchmarks taint propagation on wide hierarchies.
// Wide trees were less affected by the O(n*d) issue since depth is constant.
func BenchmarkPropagateTaint_WideTree(b *testing.B) {
	widths := []int{10, 50, 100, 200}

	for _, width := range widths {
		b.Run(itoa(width)+"_wide", func(b *testing.B) {
			root, allNodes := buildWideTree(width)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Reset taint states for each iteration
				for j := 1; j < len(allNodes); j++ {
					allNodes[j].TaintState = node.TaintClean
				}
				PropagateTaint(root, allNodes)
			}
		})
	}
}
