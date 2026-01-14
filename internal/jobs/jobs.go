// Package jobs provides job detection for prover and verifier agents.
// Jobs are nodes that need work from agents.
package jobs

import (
	"github.com/tobias/vibefeld/internal/node"
)

// JobResult contains the results of finding all jobs.
// It separates prover jobs from verifier jobs.
type JobResult struct {
	// ProverJobs contains nodes that need prover attention.
	// These are nodes with EpistemicState="pending", WorkflowState!="blocked",
	// and having at least one open/unresolved challenge.
	// Provers address challenges raised by verifiers.
	ProverJobs []*node.Node

	// VerifierJobs contains nodes ready for verifier review.
	// These are nodes with EpistemicState="pending", WorkflowState="available",
	// having a statement, and no open challenges.
	// In the breadth-first model, every new node is immediately a verifier job.
	VerifierJobs []*node.Node
}

// IsEmpty returns true if there are no jobs of either type.
func (r *JobResult) IsEmpty() bool {
	return len(r.ProverJobs) == 0 && len(r.VerifierJobs) == 0
}

// TotalCount returns the total number of jobs across both types.
func (r *JobResult) TotalCount() int {
	return len(r.ProverJobs) + len(r.VerifierJobs)
}

// FindJobs finds all prover and verifier jobs from the given nodes.
// It combines FindProverJobs and FindVerifierJobs into a single result.
//
// The nodeMap is provided for consistency but is not currently used
// (challenge-based detection doesn't need children lookup).
//
// The challengeMap maps node ID strings to the challenges on that node.
// It is required to determine which nodes have open challenges.
//
// The returned slices preserve the order of the input nodes.
// The returned pointers are the same as the input pointers (not copies).
func FindJobs(nodes []*node.Node, nodeMap map[string]*node.Node, challengeMap map[string][]*node.Challenge) *JobResult {
	return &JobResult{
		ProverJobs:   FindProverJobs(nodes, nodeMap, challengeMap),
		VerifierJobs: FindVerifierJobs(nodes, nodeMap, challengeMap),
	}
}
