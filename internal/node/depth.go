// Package node provides data structures for proof nodes in the AF system.
package node

import (
	"errors"

	aferrors "github.com/tobias/vibefeld/internal/errors"
)

// DefaultMaxDepth is the default maximum depth for proof trees.
// This matches the default in config.Config.MaxDepth.
// Nodes at depth greater than this will trigger DEPTH_EXCEEDED errors.
const DefaultMaxDepth = 20

// ValidateDepth checks that a node's depth does not exceed the maximum allowed depth.
// Returns nil if depth is within limits, or DEPTH_EXCEEDED error if too deep.
//
// The maxDepth parameter specifies the maximum allowed depth:
//   - Depth 1 = root node ("1")
//   - Depth 2 = first level children ("1.1", "1.2", etc.)
//   - Depth N = nodes with N components in their ID
//
// Use this function when you have a custom max depth from configuration.
// Use CheckDepth for the default max depth.
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

// CheckDepth validates that a node's depth does not exceed the default maximum depth.
// This is a convenience function that uses DefaultMaxDepth.
//
// Returns nil if depth is within limits, or DEPTH_EXCEEDED error if too deep.
// Use ValidateDepth if you need a custom max depth from configuration.
func CheckDepth(n *Node) error {
	return ValidateDepth(n, DefaultMaxDepth)
}
