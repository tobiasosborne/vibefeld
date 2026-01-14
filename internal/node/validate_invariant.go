// Package node provides data structures for proof nodes in the AF system.
package node

import (
	"fmt"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// ChallengeStatusSuperseded is the status for challenges that became moot
// because the parent node was archived or refuted.
// Note: This constant is defined here to complement the existing challenge
// status constants in challenge.go (open, resolved, withdrawn).
const ChallengeStatusSuperseded ChallengeStatus = "superseded"

// CheckValidationInvariant verifies the validation invariant for a node.
// The validation invariant states:
//  1. All children must be validated OR admitted (admitted is the escape hatch).
//  2. All challenges on the node must have state in {resolved, withdrawn, superseded}.
//
// Parameters:
//   - node: The node to check. If nil, returns nil (no violation).
//   - getChildren: A function that returns the direct children of a node given its ID.
//   - getChallenges: A function that returns challenges for a node. May be nil (skips challenge check).
//
// Returns:
//   - nil if the invariant holds or if the check does not apply
//   - An error with details if the invariant is violated
//
// The check only applies to nodes in the EpistemicValidated state. For nodes
// in other epistemic states (pending, admitted, refuted, archived), this
// function returns nil as the validation invariant does not apply.
func CheckValidationInvariant(n *Node, getChildren func(types.NodeID) []*Node, getChallenges func(types.NodeID) []*Challenge) error {
	// Nil node - no violation
	if n == nil {
		return nil
	}

	// Check only applies to validated nodes
	if n.EpistemicState != schema.EpistemicValidated {
		return nil
	}

	// Get direct children and check they are all validated or admitted
	children := getChildren(n.ID)
	var invalidChildren []string
	for _, child := range children {
		if child.EpistemicState != schema.EpistemicValidated && child.EpistemicState != schema.EpistemicAdmitted {
			invalidChildren = append(invalidChildren, fmt.Sprintf("%s (%s)", child.ID.String(), child.EpistemicState))
		}
	}

	if len(invalidChildren) > 0 {
		return fmt.Errorf("validation invariant violated for node %s: children not validated/admitted: %v", n.ID.String(), invalidChildren)
	}

	// Check all challenges are in acceptable states (resolved, withdrawn, superseded)
	if getChallenges != nil {
		challenges := getChallenges(n.ID)
		var openChallenges []string
		for _, ch := range challenges {
			if !isAcceptableChallengeState(ch.Status) {
				openChallenges = append(openChallenges, fmt.Sprintf("%s (%s)", ch.ID, ch.Status))
			}
		}
		if len(openChallenges) > 0 {
			return fmt.Errorf("validation invariant violated for node %s: challenges not resolved/withdrawn/superseded: %v", n.ID.String(), openChallenges)
		}
	}

	return nil
}

// isAcceptableChallengeState returns true if the challenge status is acceptable
// for validation (resolved, withdrawn, or superseded).
func isAcceptableChallengeState(status ChallengeStatus) bool {
	return status == ChallengeStatusResolved ||
		status == ChallengeStatusWithdrawn ||
		status == ChallengeStatusSuperseded
}
