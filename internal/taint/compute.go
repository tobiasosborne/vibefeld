// Package taint provides taint computation and propagation logic for AF nodes.
package taint

import (
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

// ComputeTaint computes the taint state for a node based on its epistemic state
// and its ancestors' taint states.
//
// The taint computation follows these rules:
// 0. If the node is archived or refuted, return clean (severed from proof)
// 1. If the node is pending, return unresolved
// 2. If any ancestor is unresolved, return unresolved
// 3. If the node's epistemic state introduces taint (admitted), return self_admitted
// 4. If any ancestor is tainted or self_admitted, return tainted
// 5. Otherwise, return clean
func ComputeTaint(n *node.Node, ancestors []*node.Node) node.TaintState {
	// Rule 0: Archived/refuted nodes are severed from the proof — always clean.
	// They are explicitly abandoned (archived) or disproven (refuted) and
	// should not inherit taint from ancestors.
	if n.EpistemicState == schema.EpistemicArchived || n.EpistemicState == schema.EpistemicRefuted {
		return node.TaintClean
	}

	// Rule 1: If the node is pending, return unresolved
	if n.EpistemicState == schema.EpistemicPending {
		return node.TaintUnresolved
	}

	// Rule 2: If any ancestor is unresolved, return unresolved
	for _, ancestor := range ancestors {
		if ancestor.TaintState == node.TaintUnresolved {
			return node.TaintUnresolved
		}
	}

	// Rule 3: If the node's epistemic state introduces taint (admitted), return self_admitted
	if schema.IntroducesTaint(n.EpistemicState) {
		return node.TaintSelfAdmitted
	}

	// Rule 4: If any ancestor is tainted or self_admitted, return tainted
	for _, ancestor := range ancestors {
		if ancestor.TaintState == node.TaintTainted || ancestor.TaintState == node.TaintSelfAdmitted {
			return node.TaintTainted
		}
	}

	// Rule 5: Otherwise, return clean
	return node.TaintClean
}
