// Package jobs provides job detection for prover and verifier agents.
// Jobs are nodes that need work from agents.
package jobs

import (
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

// FindVerifierJobs returns nodes ready for verifier review.
// A verifier job is a node that:
//   - WorkflowState = "claimed" (being worked on by a prover)
//   - EpistemicState = "pending" (not yet verified)
//   - All children have EpistemicState = "validated" (proof complete)
//
// This function requires a map of all nodes (keyed by ID string) to check children's states.
// The returned slice preserves the order of the input nodes.
// The returned pointers are the same as the input pointers (not copies).
func FindVerifierJobs(nodes []*node.Node, nodeMap map[string]*node.Node) []*node.Node {
	if len(nodes) == 0 {
		return nil
	}

	var result []*node.Node
	for _, n := range nodes {
		if isVerifierJob(n, nodeMap) {
			result = append(result, n)
		}
	}
	return result
}

// isVerifierJob checks if a single node qualifies as a verifier job.
func isVerifierJob(n *node.Node, nodeMap map[string]*node.Node) bool {
	// Must be claimed (being worked on by prover)
	if n.WorkflowState != schema.WorkflowClaimed {
		return false
	}

	// Must be pending (not yet verified)
	if n.EpistemicState != schema.EpistemicPending {
		return false
	}

	// Check all children are validated
	return allChildrenValidated(n, nodeMap)
}

// allChildrenValidated returns true if all direct children of the node
// have EpistemicState = "validated". Returns true if node has no children.
func allChildrenValidated(n *node.Node, nodeMap map[string]*node.Node) bool {
	// Find direct children: nodes whose ID starts with n.ID and has depth n.ID.Depth() + 1
	parentDepth := n.ID.Depth()
	hasChildren := false

	for _, child := range nodeMap {
		// Check if this node is a direct child
		if child.ID.Depth() == parentDepth+1 && n.ID.IsAncestorOf(child.ID) {
			hasChildren = true
			if child.EpistemicState != schema.EpistemicValidated {
				return false
			}
		}
	}

	// Node with no children is ready for verification
	_ = hasChildren // Silence unused variable warning; no-children is valid
	return true
}
