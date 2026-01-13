// Package node provides data structures for proof nodes in the AF system.
package node

import (
	"errors"

	aferrors "github.com/tobias/vibefeld/internal/errors"
)

// ValidateDepth checks that a node's depth does not exceed the maximum allowed depth.
// Returns nil if depth is within limits, or DEPTH_EXCEEDED error if too deep.
//
// This is a stub for TDD - implementation pending.
func ValidateDepth(n *Node, maxDepth int) error {
	if n == nil {
		return errors.New("node cannot be nil")
	}

	depth := n.ID.Depth()
	if depth > maxDepth {
		return aferrors.Newf(aferrors.DEPTH_EXCEEDED, "node depth %d exceeds maximum %d", depth, maxDepth)
	}

	return nil
}
