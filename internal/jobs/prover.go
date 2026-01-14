// Package jobs provides job detection for prover and verifier agents.
// Jobs are nodes that need work from agents.
package jobs

import (
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

// FindProverJobs returns nodes available for provers to work on.
// A prover job is a node that:
//   - EpistemicState = "pending" (needs proof work)
//   - WorkflowState != "blocked" (available or claimed)
//   - Has one or more open/unresolved challenges that need addressing
//
// This implements the challenge-driven prover model where provers work on
// nodes that have been challenged by verifiers. Unchallenged nodes are
// verifier territory in the breadth-first adversarial model.
//
// The challengeMap parameter maps node ID strings to the challenges on that node.
// If challengeMap is nil, it is treated as empty (no challenges exist, so no prover jobs).
//
// The returned slice preserves the order of the input nodes.
// The returned pointers are the same as the input pointers (not copies).
func FindProverJobs(nodes []*node.Node, nodeMap map[string]*node.Node, challengeMap map[string][]*node.Challenge) []*node.Node {
	if len(nodes) == 0 {
		return nil
	}

	var result []*node.Node
	for _, n := range nodes {
		if isProverJob(n, challengeMap) {
			result = append(result, n)
		}
	}
	return result
}

// isProverJob checks if a single node qualifies as a prover job.
// A prover job is a node that needs prover attention:
//   - EpistemicState = "pending" (not yet verified)
//   - WorkflowState != "blocked" (can be available or claimed)
//   - Has at least one open challenge
//
// Provers address challenges raised by verifiers. Once all challenges
// are resolved/withdrawn, the node becomes a verifier job again.
func isProverJob(n *node.Node, challengeMap map[string][]*node.Challenge) bool {
	// Must not be blocked
	if n.WorkflowState == schema.WorkflowBlocked {
		return false
	}

	// Must be pending (not yet verified)
	if n.EpistemicState != schema.EpistemicPending {
		return false
	}

	// Must have at least one open challenge
	return hasOpenChallenges(n, challengeMap)
}
