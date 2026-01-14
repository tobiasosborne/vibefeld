// Package node provides data structures for proof nodes in the AF system.
package node

import (
	"github.com/tobias/vibefeld/internal/errors"
)

// ValidateRefinementCount checks if a node can accept additional refinements.
// It returns nil if refinementCount < maxRefinements (refinement is allowed).
// It returns a REFINEMENT_LIMIT_EXCEEDED error (exit code 3) if the count
// is at or over the limit.
//
// Parameters:
//   - n: The node being refined (must not be nil)
//   - refinementCount: The current number of refinements on this node
//   - maxRefinements: The maximum allowed refinements
//
// Returns an error if:
//   - n is nil
//   - refinementCount is negative
//   - maxRefinements is negative or zero
//   - refinementCount >= maxRefinements
func ValidateRefinementCount(n *Node, refinementCount, maxRefinements int) error {
	// Handle nil node
	if n == nil {
		return errors.New(errors.REFINEMENT_LIMIT_EXCEEDED, "cannot validate refinement count for nil node")
	}

	// Handle invalid refinement count
	if refinementCount < 0 {
		return errors.Newf(errors.REFINEMENT_LIMIT_EXCEEDED,
			"invalid refinement count %d: must be non-negative", refinementCount)
	}

	// Handle invalid max refinements
	if maxRefinements <= 0 {
		return errors.Newf(errors.REFINEMENT_LIMIT_EXCEEDED,
			"invalid max refinements %d: must be positive", maxRefinements)
	}

	// Check if at or over the limit
	if refinementCount >= maxRefinements {
		return errors.Newf(errors.REFINEMENT_LIMIT_EXCEEDED,
			"refinement limit exceeded: count %d >= max %d", refinementCount, maxRefinements)
	}

	return nil
}
