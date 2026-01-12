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
//
// The returned slice preserves the order of the input nodes.
// The returned pointers are the same as the input pointers (not copies).
func FindProverJobs(nodes []*node.Node) []*node.Node {
	if len(nodes) == 0 {
		return nil
	}

	var result []*node.Node
	for _, n := range nodes {
		if n.WorkflowState == schema.WorkflowAvailable &&
			n.EpistemicState == schema.EpistemicPending {
			result = append(result, n)
		}
	}
	return result
}
