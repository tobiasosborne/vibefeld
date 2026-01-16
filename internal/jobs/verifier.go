// Package jobs provides job detection for prover and verifier agents.
// Jobs are nodes that need work from agents.
package jobs

import (
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

// FindVerifierJobs returns nodes ready for verifier review.
// A verifier job is a node that:
//   - Has a statement (was refined/created with content)
//   - EpistemicState = "pending" (not yet verified)
//   - WorkflowState = "available" (not claimed or blocked)
//   - Has no unresolved/open challenges
//
// This implements the "breadth-first" adversarial verification model where
// every new node is immediately reviewable by verifiers. Challenges create
// prover jobs; when challenges are resolved, the node becomes a verifier job again.
//
// The challengeMap parameter maps node ID strings to the challenges on that node.
// If challengeMap is nil, it is treated as empty (no challenges exist).
//
// The returned slice preserves the order of the input nodes.
// The returned pointers are the same as the input pointers (not copies).
func FindVerifierJobs(nodes []*node.Node, nodeMap map[string]*node.Node, challengeMap map[string][]*node.Challenge) []*node.Node {
	if len(nodes) == 0 {
		return nil
	}

	var result []*node.Node
	for _, n := range nodes {
		if isVerifierJob(n, challengeMap) {
			result = append(result, n)
		}
	}
	return result
}

// isVerifierJob checks if a single node qualifies as a verifier job.
// A verifier job is a node that is ready for verifier review:
//   - Has a statement (non-empty)
//   - EpistemicState = "pending" (not yet verified)
//   - WorkflowState = "available" (not claimed or blocked)
//   - Has no open blocking challenges (critical/major severity)
//
// This is the breadth-first model: new nodes are immediately verifiable.
// Blocking challenges move nodes to prover territory until resolved.
// Non-blocking challenges (minor/note) do not prevent verifier review.
func isVerifierJob(n *node.Node, challengeMap map[string][]*node.Challenge) bool {
	// Must have a statement (nodes are created with statements, but check anyway)
	if n.Statement == "" {
		return false
	}

	// Must be available (not claimed, not blocked)
	if n.WorkflowState != schema.WorkflowAvailable {
		return false
	}

	// Must be pending (not yet verified)
	if n.EpistemicState != schema.EpistemicPending {
		return false
	}

	// Must have no open blocking challenges (critical/major)
	// Minor and note challenges do not prevent verifier review
	return !hasBlockingChallenges(n, challengeMap)
}

// hasOpenChallenges returns true if the node has any open (unresolved) challenges.
// Deprecated: Use hasBlockingChallenges for severity-aware checking.
func hasOpenChallenges(n *node.Node, challengeMap map[string][]*node.Challenge) bool {
	if challengeMap == nil {
		return false
	}

	challenges := challengeMap[n.ID.String()]
	for _, c := range challenges {
		if c.Status == node.ChallengeStatusOpen {
			return true
		}
	}
	return false
}

// hasBlockingChallenges returns true if the node has any open challenges with
// blocking severity (critical or major). Minor and note challenges do not block.
func hasBlockingChallenges(n *node.Node, challengeMap map[string][]*node.Challenge) bool {
	if challengeMap == nil {
		return false
	}

	challenges := challengeMap[n.ID.String()]
	for _, c := range challenges {
		if c.Status == node.ChallengeStatusOpen &&
			schema.SeverityBlocksAcceptance(schema.ChallengeSeverity(c.Severity)) {
			return true
		}
	}
	return false
}
