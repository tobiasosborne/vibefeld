package export

import (
	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

// Capability tokens advertised in GraphExport.Features. An external driver
// (rk) reads this list at startup to confirm the af binary it is driving is
// new enough to emit the flags the driver depends on — an older af that
// predates a capability simply omits its token (older af omits the whole
// features[] array), so the driver fails loudly at preflight instead of
// silently reading an absent omitempty flag as "false / nothing ready".
const (
	// FeatureReadinessFlags: per-node prover_ready/verifier_ready are emitted
	// from af's own internal/jobs classifier.
	FeatureReadinessFlags = "readiness-flags"
	// FeatureClosureFlag: per-node closed is emitted (bottom-up subtree
	// closure, the authoritative convergence signal for a root).
	FeatureClosureFlag = "closure-flag"
	// FeatureNodeDependencies: per-node dependencies[] (reference deps) are
	// emitted, so a verifier sees the exact dependency graph the prover
	// produced rather than a children-substitution proxy.
	FeatureNodeDependencies = "node-dependencies"
	// FeatureProofAuthor: per-node proof_author (the prover-of-record that
	// decomposed the node via `af record-proof`) is emitted, distinct from
	// author, so a driver's cross-vendor check can compare the decomposer's
	// family for a decomposed node (rk GAP 9).
	FeatureProofAuthor = "proof-author"
)

// GraphFeatures is the fixed capability list this af build advertises. Always
// emitted (never omitempty): its ABSENCE is the signal an older af produced
// the document. Extend it (and bump a driver's expected set) whenever a new
// export capability an external consumer must detect is added.
var GraphFeatures = []string{
	FeatureReadinessFlags,
	FeatureClosureFlag,
	FeatureNodeDependencies,
	FeatureProofAuthor,
}

// isEpistemicallyCleared reports whether a node's own epistemic state is one
// of the terminal-cleared states (validated, admitted, or archived) — the same
// allowlist af accept uses for a child's children-completeness gate. Refuted
// and pending/draft/needs_refinement are NOT cleared.
func isEpistemicallyCleared(e schema.EpistemicState) bool {
	switch e {
	case schema.EpistemicValidated, schema.EpistemicAdmitted, schema.EpistemicArchived:
		return true
	default:
		return false
	}
}

// computeClosedSet returns the set of node IDs that are CLOSED: the subtree
// rooted at the node is settled and needs no further prover or verifier work.
// A node is closed iff:
//   - its own epistemic state is cleared (validated/admitted/archived), AND
//   - its workflow state is not blocked, AND
//   - it has no open blocking (critical/major) challenge, AND
//   - every direct child is itself closed (bottom-up).
//
// This is the authoritative convergence signal a driver reads on the ROOT:
// unlike the bare epistemic state, it goes false the instant a blocking
// challenge is raised on an already-validated node (the exact defect a
// "validated == converged" predicate misses), or the instant any descendant
// falls out of a closed state.
func computeClosedSet(nodes []*node.Node, nodeMap map[string]*node.Node, challengeMap map[string][]*node.Challenge) map[string]bool {
	childrenOf := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		if parentID, ok := n.ID.Parent(); ok {
			p := parentID.String()
			childrenOf[p] = append(childrenOf[p], n.ID.String())
		}
	}

	memo := make(map[string]bool, len(nodes))
	var closed func(id string) bool
	closed = func(id string) bool {
		if v, ok := memo[id]; ok {
			return v
		}
		// Guard against a re-entrant cycle (hierarchical IDs are acyclic, but
		// be defensive): mark not-closed while computing.
		memo[id] = false
		n := nodeMap[id]
		if n == nil {
			return false
		}
		result := isEpistemicallyCleared(n.EpistemicState) &&
			n.WorkflowState != schema.WorkflowBlocked &&
			!jobs.HasBlockingChallenges(n, challengeMap)
		if result {
			for _, cid := range childrenOf[id] {
				if !closed(cid) {
					result = false
					break
				}
			}
		}
		memo[id] = result
		return result
	}

	for _, n := range nodes {
		closed(n.ID.String())
	}
	return memo
}
