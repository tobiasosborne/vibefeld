// Package node provides data structures for proof nodes in the AF system.
package node

import (
	"fmt"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// CheckValidationInvariant verifies the validation invariant for a node.
// The validation invariant states: "A node can only be validated if ALL its
// children are validated."
//
// Parameters:
//   - node: The node to check. If nil, returns nil (no violation).
//   - getChildren: A function that returns the direct children of a node given its ID.
//
// Returns:
//   - nil if the invariant holds or if the check does not apply
//   - An error with details if the invariant is violated
//
// The check only applies to nodes in the EpistemicValidated state. For nodes
// in other epistemic states (pending, admitted, refuted, archived), this
// function returns nil as the validation invariant does not apply.
func CheckValidationInvariant(n *Node, getChildren func(types.NodeID) []*Node) error {
	// Nil node - no violation
	if n == nil {
		return nil
	}

	// Check only applies to validated nodes
	if n.EpistemicState != schema.EpistemicValidated {
		return nil
	}

	// Get direct children
	children := getChildren(n.ID)
	if len(children) == 0 {
		// No children - invariant trivially holds
		return nil
	}

	// Check all children are validated
	var unvalidatedChildren []string
	for _, child := range children {
		if child.EpistemicState != schema.EpistemicValidated {
			unvalidatedChildren = append(unvalidatedChildren, fmt.Sprintf("%s (%s)", child.ID.String(), child.EpistemicState))
		}
	}

	if len(unvalidatedChildren) > 0 {
		return fmt.Errorf("validation invariant violated for node %s: children not validated: %v", n.ID.String(), unvalidatedChildren)
	}

	return nil
}
