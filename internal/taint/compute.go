// Package taint provides taint computation and propagation logic for AF nodes.
package taint

import (
	"github.com/tobias/vibefeld/internal/node"
)

// ComputeTaint computes the taint state for a node from its epistemic state and
// the epistemic states of its ancestors. It is the ancestor-chain-only form of
// the computation; use ComputeTaintInTree when descendants are available.
//
// The taint computation follows these rules:
// 0. If the node is archived or refuted, return clean (severed from proof)
// 1. If the node is pending, draft, or needs_refinement, return unresolved
// 2. If any non-severed ancestor is pending, draft, or needs_refinement, return unresolved
// 3. If the node's epistemic state introduces taint (admitted), return self_admitted
// 4. If any non-severed ancestor is admitted, return tainted
// 5. Otherwise, return clean
//
// Ancestors' stored TaintState values are deliberately ignored: taint is a
// derived property, and historical ledgers may contain stale audit values.
func ComputeTaint(n *node.Node, ancestors []*node.Node) node.TaintState {
	down := componentClean
	for _, ancestor := range ancestors {
		if ancestor == nil || isSevered(ancestor) {
			continue
		}
		down = combineComponents(down, epistemicContribution(ancestor.EpistemicState))
	}
	return finalTaint(n, down, componentClean)
}

// ComputeTaintInTree computes a node's complete taint state, including both
// ancestor-chain and descendant-subtree contributions. The computation is
// independent of every node's stored TaintState. It is O(N) per call and is
// intended for one-off queries; use RecomputeAll when deriving the whole tree.
func ComputeTaintInTree(n *node.Node, allNodes []*node.Node) node.TaintState {
	if n == nil {
		panic("ComputeTaintInTree called with nil node")
	}

	nodes := allNodes
	found := false
	for _, candidate := range allNodes {
		if candidate != nil && candidate.ID.Equal(n.ID) {
			found = true
			break
		}
	}
	if !found {
		nodes = append(append([]*node.Node(nil), allNodes...), n)
	}

	computed := computeTreeTaints(nodes)
	if result, ok := computed.final[n.ID.String()]; ok {
		return result
	}

	return ComputeTaint(n, nil)
}
