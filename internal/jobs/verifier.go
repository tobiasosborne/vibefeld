// Package jobs provides job detection for prover and verifier agents.
// Jobs are nodes that need work from agents.
package jobs

import (
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

// FindVerifierJobs returns nodes ready for verifier review.
// A verifier job is a node that:
//   - WorkflowState != "blocked" (not blocked by dependencies)
//   - EpistemicState = "pending" (not yet verified)
//   - Has children AND all children have EpistemicState = "validated" (refinement complete)
//
// Note: The WorkflowState can be either "claimed" or "available". After a prover
// refines a node and releases it, the node should appear as a verifier job.
// Leaf nodes (no children) are NOT verifier jobs - they are prover jobs that need refinement.
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
// A verifier job is a node that is ready for verifier review:
//   - EpistemicState = "pending" (not yet verified)
//   - WorkflowState != "blocked" (not blocked by dependencies)
//   - Has children AND all children are validated (refinement is complete)
//
// Note: The WorkflowState can be either "claimed" or "available". After a prover
// refines a node and releases it, the node becomes available but should still
// appear as a verifier job once all children are validated.
func isVerifierJob(n *node.Node, nodeMap map[string]*node.Node) bool {
	// Must not be blocked
	if n.WorkflowState == schema.WorkflowBlocked {
		return false
	}

	// Must be pending (not yet verified)
	if n.EpistemicState != schema.EpistemicPending {
		return false
	}

	// Must have children and all children must be validated.
	// A node with no children is a prover job (needs refinement), not a verifier job.
	return hasChildrenAllValidated(n, nodeMap)
}

// hasChildrenAllValidated returns true if the node has at least one direct child
// AND all direct children have EpistemicState = "validated".
// Returns false if node has no children.
func hasChildrenAllValidated(n *node.Node, nodeMap map[string]*node.Node) bool {
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

	// Only return true if we found at least one child and all were validated
	return hasChildren
}
