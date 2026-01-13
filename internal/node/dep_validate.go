// Package node provides data structures for proof nodes in the AF system.
package node

import (
	"errors"
	"fmt"

	"github.com/tobias/vibefeld/internal/types"
)

// NodeLookup is an interface for looking up nodes by ID.
// This avoids import cycles by not depending on the state package directly.
// The state.State type implements this interface.
type NodeLookup interface {
	GetNode(id types.NodeID) *Node
}

// ValidateDepExistence checks that all dependencies of a node exist in state.
// Returns error if any dependency does not exist.
// Returns error if node is nil or state is nil.
// Does NOT check self-dependency or transitive dependencies - only immediate.
func ValidateDepExistence(n *Node, s NodeLookup) error {
	if n == nil {
		return errors.New("node cannot be nil")
	}
	if s == nil {
		return errors.New("state cannot be nil")
	}

	// No dependencies means validation passes
	if len(n.Dependencies) == 0 {
		return nil
	}

	// Check each dependency exists in state
	for _, depID := range n.Dependencies {
		if s.GetNode(depID) == nil {
			return fmt.Errorf("invalid dependency: node %s not found", depID.String())
		}
	}

	return nil
}
