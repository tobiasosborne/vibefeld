// Package jobs provides job detection for prover and verifier agents.
// Jobs are nodes that need work from agents.
package jobs

import (
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
)

// FindProverJobs returns nodes available for provers to work on.
// A prover job is a node that:
//   - EpistemicState = "pending" (needs proof work)
//   - WorkflowState != "blocked" (available or claimed)
//   - Has one or more open blocking challenges (critical/major severity)
//
// This implements the challenge-driven prover model where provers work on
// nodes that have been challenged by verifiers. Only blocking challenges
// (critical/major severity) require prover attention. Minor and note
// challenges do not create prover jobs.
//
// Unchallenged nodes are verifier territory in the breadth-first adversarial model.
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
		if isProverJob(n, nodeMap, challengeMap) {
			result = append(result, n)
		}
	}
	return result
}

// IsProverJob reports whether the node currently qualifies as a prover job
// under af's own classifier (the exported form of isProverJob). External
// callers (e.g. the service's atomic record-proof kernel op) use it to enforce
// that a prover write targets a node af actually classifies as prover work,
// matching the export's prover_ready flag exactly. nodeMap maps node ID strings
// to nodes and is needed to decide whether a needs_refinement node's children
// are cleared (which hands it to the verifier).
func IsProverJob(n *node.Node, nodeMap map[string]*node.Node, challengeMap map[string][]*node.Challenge) bool {
	return isProverJob(n, nodeMap, challengeMap)
}

// isProverJob checks if a single node qualifies as a prover job.
// A prover job is a node that needs prover attention:
//   - WorkflowState != "blocked" (can be available or claimed)
//   - One of the following:
//   - EpistemicState = "pending" with at least one open blocking challenge
//   - EpistemicState = "needs_refinement" (awaiting further proof development)
//
// Provers address blocking challenges raised by verifiers. Minor and note
// challenges do not require prover attention. Once all blocking challenges
// are resolved/withdrawn, the node becomes a verifier job again.
//
// Nodes in needs_refinement state are also prover jobs - these are validated
// nodes that have been reopened for further proof development.
func isProverJob(n *node.Node, nodeMap map[string]*node.Node, challengeMap map[string][]*node.Challenge) bool {
	// Must not be blocked
	if n.WorkflowState == schema.WorkflowBlocked {
		return false
	}

	// Draft nodes are prover jobs (work in progress, need development)
	if n.EpistemicState == schema.EpistemicDraft {
		return true
	}

	// A node reopened for more proof work is a prover job until its children are
	// cleared, at which point it becomes a verifier job again (D4). A
	// needs_refinement node with no children at all stays prover work: accept
	// refuses it until refinement actually happened.
	if n.EpistemicState == schema.EpistemicNeedsRefinement {
		return !hasAnyChild(n, nodeMap) || !AllChildrenCleared(n, nodeMap)
	}

	// Must be pending (not yet verified)
	if n.EpistemicState != schema.EpistemicPending {
		return false
	}

	// Must have at least one open blocking challenge (critical/major)
	// Minor and note challenges do not create prover jobs
	return hasBlockingChallenges(n, challengeMap)
}

// hasAnyChild reports whether n has at least one direct child in nodeMap.
func hasAnyChild(n *node.Node, nodeMap map[string]*node.Node) bool {
	for _, c := range nodeMap {
		parent, ok := c.ID.Parent()
		if ok && parent.String() == n.ID.String() {
			return true
		}
	}
	return false
}
