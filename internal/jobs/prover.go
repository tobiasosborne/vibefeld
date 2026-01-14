// Package jobs provides job detection for prover and verifier agents.
// Jobs are nodes that need work from agents.
package jobs

import (
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

// FindProverJobs returns nodes available for provers to work on.
// A prover job is a node that is:
//   - WorkflowState = "available" (not claimed or blocked)
//   - EpistemicState = "pending" (needs proof work)
//   - NOT a verifier job (i.e., not a node with all children validated)
//
// Nodes with all children validated are verifier jobs, not prover jobs, even if available.
// This ensures a refined and released node appears as a verifier job, not a prover job.
//
// The nodeMap is required to check children's states. If nodeMap is nil, children are not checked
// and all available+pending nodes are returned (backward compatible behavior for tests).
//
// The returned slice preserves the order of the input nodes.
// The returned pointers are the same as the input pointers (not copies).
func FindProverJobs(nodes []*node.Node, nodeMap map[string]*node.Node) []*node.Node {
	if len(nodes) == 0 {
		return nil
	}

	var result []*node.Node
	for _, n := range nodes {
		if n.WorkflowState == schema.WorkflowAvailable &&
			n.EpistemicState == schema.EpistemicPending {
			// Exclude nodes that qualify as verifier jobs (have children, all validated)
			if nodeMap != nil && hasChildrenAllValidated(n, nodeMap) {
				continue
			}
			result = append(result, n)
		}
	}
	return result
}
