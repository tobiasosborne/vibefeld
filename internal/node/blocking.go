// Package node provides types and functions for proof nodes and their relationships.
package node

import (
	"github.com/tobias/vibefeld/internal/types"
)

// IsBlocked returns true if the node is blocked by a pending definition.
// A node is blocked if it has requested a definition that is still pending.
// Returns false if pendingDefs is nil or empty, or if no matching pending def is found.
func IsBlocked(nodeID types.NodeID, pendingDefs []*PendingDef) bool {
	return GetBlockingDef(nodeID, pendingDefs) != nil
}

// GetBlockingDef returns the pending definition blocking this node, if any.
// Returns nil if the node is not blocked by any pending definition.
// Only returns pending defs that are still in "pending" status (not resolved or cancelled).
// If the node has multiple pending definitions, returns the first one found.
func GetBlockingDef(nodeID types.NodeID, pendingDefs []*PendingDef) *PendingDef {
	if pendingDefs == nil {
		return nil
	}

	nodeStr := nodeID.String()
	for _, pd := range pendingDefs {
		if pd == nil {
			continue
		}
		if !pd.IsPending() {
			continue
		}
		if pd.RequestedBy.String() == nodeStr {
			return pd
		}
	}
	return nil
}

// GetBlockedNodes returns all nodes blocked by a given pending definition.
// Only considers pending defs that are still in "pending" status.
// Returns an empty slice if the pending def is nil, not pending, or no nodes match.
func GetBlockedNodes(pendingDef *PendingDef, allNodes []types.NodeID) []types.NodeID {
	if pendingDef == nil || !pendingDef.IsPending() {
		return []types.NodeID{}
	}

	requestedByStr := pendingDef.RequestedBy.String()
	var blocked []types.NodeID

	for _, nodeID := range allNodes {
		if nodeID.String() == requestedByStr {
			blocked = append(blocked, nodeID)
		}
	}

	return blocked
}

// ComputeBlockedSet computes the full set of blocked nodes considering transitivity.
// A node is blocked if:
//   - It directly requested a pending definition that is still pending, OR
//   - It depends on another node that is blocked (transitive blocking)
//
// The dependencies map should map each node ID string to the node ID strings it depends on.
// Returns a map where keys are blocked NodeID strings and values are true.
// Returns an empty (non-nil) map if there are no blocked nodes.
func ComputeBlockedSet(pendingDefs []*PendingDef, dependencies map[string][]string) map[string]bool {
	blocked := make(map[string]bool)

	if pendingDefs == nil && dependencies == nil {
		return blocked
	}

	// First pass: mark directly blocked nodes
	for _, pd := range pendingDefs {
		if pd == nil || !pd.IsPending() {
			continue
		}
		blocked[pd.RequestedBy.String()] = true
	}

	// If no directly blocked nodes, nothing to propagate
	if len(blocked) == 0 {
		return blocked
	}

	if dependencies == nil {
		return blocked
	}

	// Build reverse dependency map: for each node, which nodes depend on it
	// This allows efficient propagation of blocking status
	dependents := make(map[string][]string)
	for node, deps := range dependencies {
		for _, dep := range deps {
			dependents[dep] = append(dependents[dep], node)
		}
	}

	// Use BFS to propagate blocking through dependencies
	// A node becomes blocked if any of its dependencies are blocked
	queue := make([]string, 0, len(blocked))
	for nodeID := range blocked {
		queue = append(queue, nodeID)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Find all nodes that depend on current
		for _, dependent := range dependents[current] {
			if !blocked[dependent] {
				blocked[dependent] = true
				queue = append(queue, dependent)
			}
		}
	}

	return blocked
}

// WouldResolveBlocking checks if resolving a pending def would unblock nodes.
// Returns a slice of NodeIDs that would be unblocked if the given definition ID is resolved.
// Only considers pending defs that are still in "pending" status.
// Note: This only returns directly blocked nodes, not transitively blocked ones.
func WouldResolveBlocking(defID string, pendingDefs []*PendingDef) []types.NodeID {
	if defID == "" {
		return []types.NodeID{}
	}

	var wouldUnblock []types.NodeID
	for _, pd := range pendingDefs {
		if pd == nil || !pd.IsPending() {
			continue
		}
		if pd.ID == defID {
			wouldUnblock = append(wouldUnblock, pd.RequestedBy)
		}
	}
	return wouldUnblock
}
