// Package node provides cycle detection for dependency graphs.
package node

import (
	"github.com/tobias/vibefeld/internal/types"
)

// NodeProvider is an interface for accessing nodes in state.
// This allows cycle detection to work with state without creating
// an import cycle.
type NodeProvider interface {
	// GetNode returns the node with the given ID, or nil if not found.
	GetNode(id types.NodeID) *Node
	// AllNodes returns all nodes in the state.
	AllNodes() []*Node
}

// color constants for DFS-based cycle detection
const (
	white = 0 // unvisited
	gray  = 1 // currently being visited (in the recursion stack)
	black = 2 // fully explored (no cycle from this node)
)

// DetectCycle checks if there is a cycle in the dependency graph
// starting from the given node ID.
// Returns (hasCycle, cyclePath) where:
//   - hasCycle is true if a cycle was detected
//   - cyclePath contains the nodes forming the cycle, with the first and last
//     node being the same to show where the cycle closes
//
// If the starting node doesn't exist, returns (false, nil).
// If a dependency doesn't exist, it's treated as a leaf node (no cycle from broken reference).
func DetectCycle(provider NodeProvider, startID types.NodeID) (bool, []types.NodeID) {
	// Get the starting node
	startNode := provider.GetNode(startID)
	if startNode == nil {
		return false, nil
	}

	// Color map for DFS: white=unvisited, gray=in-progress, black=done
	colors := make(map[string]int)

	// Path tracking for reconstructing the cycle
	path := make([]types.NodeID, 0)

	// Run DFS
	hasCycle, cyclePath := detectCycleDFS(provider, startID, colors, path)

	return hasCycle, cyclePath
}

// detectCycleDFS performs DFS-based cycle detection with path tracking.
// It uses the three-color algorithm:
//   - white (0): node not yet visited
//   - gray (1): node is being processed (in current recursion stack)
//   - black (2): node and all descendants fully processed
//
// Returns (hasCycle, cyclePath) where cyclePath shows the cycle if found.
func detectCycleDFS(provider NodeProvider, nodeID types.NodeID, colors map[string]int, path []types.NodeID) (bool, []types.NodeID) {
	idStr := nodeID.String()

	// Check current color
	switch colors[idStr] {
	case gray:
		// Found a back edge - cycle detected!
		// Build the cycle path from where we are back to this node
		cyclePath := make([]types.NodeID, 0, len(path)+1)
		cyclePath = append(cyclePath, nodeID) // Start with the cycle entry point
		inCycle := false
		for _, pathNode := range path {
			if pathNode.String() == idStr {
				inCycle = true
			}
			if inCycle {
				cyclePath = append(cyclePath, pathNode)
			}
		}
		cyclePath = append(cyclePath, nodeID) // Close the cycle
		return true, cyclePath

	case black:
		// Already fully explored, no cycle from this path
		return false, nil

	default: // white - unvisited
		// Mark as in-progress
		colors[idStr] = gray

		// Get the node
		node := provider.GetNode(nodeID)
		if node == nil {
			// Missing node - treat as leaf (no dependencies)
			colors[idStr] = black
			return false, nil
		}

		// Extend path
		newPath := append(path, nodeID)

		// Visit all dependencies
		for _, dep := range node.Dependencies {
			hasCycle, cyclePath := detectCycleDFS(provider, dep, colors, newPath)
			if hasCycle {
				return true, cyclePath
			}
		}

		// Done with this node
		colors[idStr] = black
		return false, nil
	}
}

// ValidateDependencies checks all nodes in the state for dependency cycles.
// Returns a slice of cycle paths, where each cycle path is a slice of NodeIDs
// representing a cycle (first and last elements are the same).
// Returns an empty slice if no cycles are found.
func ValidateDependencies(provider NodeProvider) [][]types.NodeID {
	cycles := make([][]types.NodeID, 0)

	// Global color map for all nodes
	colors := make(map[string]int)

	// Track which nodes are part of already-found cycles to avoid duplicates
	inFoundCycle := make(map[string]bool)

	// Check each node in the state
	for _, node := range provider.AllNodes() {
		idStr := node.ID.String()

		// Skip if already fully explored
		if colors[idStr] == black {
			continue
		}

		// Skip if already known to be in a cycle
		if inFoundCycle[idStr] {
			continue
		}

		// Run cycle detection from this node
		path := make([]types.NodeID, 0)
		hasCycle, cyclePath := detectCycleDFSForValidation(provider, node.ID, colors, path, inFoundCycle)
		if hasCycle && len(cyclePath) > 0 {
			cycles = append(cycles, cyclePath)

			// Mark all nodes in this cycle as found
			for _, cycleNode := range cyclePath {
				inFoundCycle[cycleNode.String()] = true
			}
		}
	}

	return cycles
}

// detectCycleDFSForValidation is similar to detectCycleDFS but uses a shared
// color map and inFoundCycle tracking to avoid duplicate cycle detection.
func detectCycleDFSForValidation(provider NodeProvider, nodeID types.NodeID, colors map[string]int, path []types.NodeID, inFoundCycle map[string]bool) (bool, []types.NodeID) {
	idStr := nodeID.String()

	// Check current color
	switch colors[idStr] {
	case gray:
		// Found a back edge - cycle detected!
		// Build the cycle path
		cyclePath := make([]types.NodeID, 0, len(path)+1)
		inCycle := false
		for _, pathNode := range path {
			if pathNode.String() == idStr {
				inCycle = true
			}
			if inCycle {
				cyclePath = append(cyclePath, pathNode)
			}
		}
		cyclePath = append(cyclePath, nodeID) // Close the cycle
		return true, cyclePath

	case black:
		// Already fully explored, no cycle from this path
		return false, nil

	default: // white - unvisited
		// Mark as in-progress
		colors[idStr] = gray

		// Get the node
		node := provider.GetNode(nodeID)
		if node == nil {
			// Missing node - treat as leaf (no dependencies)
			colors[idStr] = black
			return false, nil
		}

		// Extend path
		newPath := append(path, nodeID)

		// Visit all dependencies
		for _, dep := range node.Dependencies {
			// Skip if we already found this node in a cycle
			if inFoundCycle[dep.String()] {
				continue
			}

			hasCycle, cyclePath := detectCycleDFSForValidation(provider, dep, colors, newPath, inFoundCycle)
			if hasCycle {
				return true, cyclePath
			}
		}

		// Done with this node
		colors[idStr] = black
		return false, nil
	}
}
