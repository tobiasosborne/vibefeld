// Package jobs provides job detection for prover and verifier agents.
// Jobs are nodes that need work from agents.
package jobs

import (
	"github.com/tobias/vibefeld/internal/node"
)

// JobResult contains the results of finding all jobs.
// It separates prover jobs from verifier jobs.
type JobResult struct {
	// ProverJobs contains nodes available for provers to work on.
	// These are nodes with WorkflowState="available" and EpistemicState="pending".
	ProverJobs []*node.Node

	// VerifierJobs contains nodes ready for verifier review.
	// These are nodes with WorkflowState="claimed", EpistemicState="pending",
	// and all children have EpistemicState="validated".
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
// The nodeMap is required for verifier job detection (to check children's states).
// The returned slices preserve the order of the input nodes.
// The returned pointers are the same as the input pointers (not copies).
func FindJobs(nodes []*node.Node, nodeMap map[string]*node.Node) *JobResult {
	return &JobResult{
		ProverJobs:   FindProverJobs(nodes),
		VerifierJobs: FindVerifierJobs(nodes, nodeMap),
	}
}
