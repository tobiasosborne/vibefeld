// Package cycle provides cycle detection for dependency graphs in proofs.
//
// This package implements DFS-based cycle detection using the three-color
// algorithm (white/gray/black) to identify circular reasoning in logical
// dependencies between proof nodes.
//
// Circular reasoning (e.g., A depends on B, B depends on C, C depends on A)
// is a logical fallacy that must be detected and prevented in valid proofs.
package cycle

import (
	"strings"

	"github.com/tobias/vibefeld/internal/types"
)

// DependencyProvider is an interface for accessing node dependencies.
// This abstraction allows cycle detection to work with any graph structure
// without creating import cycles with the state package.
type DependencyProvider interface {
	// GetNodeDependencies returns the dependencies for a node.
	// Returns (deps, true) if node exists, (nil, false) if not found.
	GetNodeDependencies(id types.NodeID) ([]types.NodeID, bool)

	// AllNodeIDs returns all node IDs in the graph.
	AllNodeIDs() []types.NodeID
}

// CycleResult contains the result of a cycle detection operation.
type CycleResult struct {
	// HasCycle is true if a cycle was detected.
	HasCycle bool

	// Path contains the nodes forming the cycle, with the first and last
	// node being the same to show where the cycle closes.
	// Empty if no cycle was detected.
	Path []types.NodeID
}

// Error returns a human-readable error message if a cycle exists.
// Returns empty string if no cycle.
func (r CycleResult) Error() string {
	if !r.HasCycle {
		return ""
	}

	// Build path string
	pathStrs := make([]string, len(r.Path))
	for i, id := range r.Path {
		pathStrs[i] = id.String()
	}

	return "circular dependency detected: " + strings.Join(pathStrs, " -> ")
}

// color constants for DFS-based cycle detection using three-color algorithm
const (
	white = 0 // unvisited
	gray  = 1 // currently being visited (in the recursion stack)
	black = 2 // fully explored (no cycle from this node)
)

// DetectCycleFrom checks if there is a cycle in the dependency graph
// starting from the given node ID.
//
// The algorithm uses DFS with three-color marking:
//   - white: node not yet visited
//   - gray: node is being processed (in current recursion stack)
//   - black: node and all descendants fully processed (no cycle)
//
// A cycle exists when we encounter a gray node during traversal,
// meaning we've found a back edge to a node still in our recursion stack.
//
// If the starting node doesn't exist, returns CycleResult{HasCycle: false}.
// If a dependency doesn't exist, it's treated as a leaf node (no cycle from broken reference).
func DetectCycleFrom(provider DependencyProvider, startID types.NodeID) CycleResult {
	// Check if starting node exists
	if _, ok := provider.GetNodeDependencies(startID); !ok {
		return CycleResult{HasCycle: false}
	}

	// Color map for DFS: white=unvisited, gray=in-progress, black=done
	colors := make(map[string]int)

	// Path tracking for reconstructing the cycle
	path := make([]types.NodeID, 0)

	// Run DFS
	hasCycle, cyclePath := detectCycleDFS(provider, startID, colors, path)

	return CycleResult{
		HasCycle: hasCycle,
		Path:     cyclePath,
	}
}

// detectCycleDFS performs DFS-based cycle detection with path tracking.
// Returns (hasCycle, cyclePath) where cyclePath shows the cycle if found.
func detectCycleDFS(provider DependencyProvider, nodeID types.NodeID, colors map[string]int, path []types.NodeID) (bool, []types.NodeID) {
	idStr := nodeID.String()

	// Check current color
	switch colors[idStr] {
	case gray:
		// Found a back edge - cycle detected!
		// Build the cycle path from where we are back to this node
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

		// Get the node's dependencies
		deps, ok := provider.GetNodeDependencies(nodeID)
		if !ok {
			// Missing node - treat as leaf (no dependencies)
			colors[idStr] = black
			return false, nil
		}

		// Extend path
		newPath := append(path, nodeID)

		// Visit all dependencies
		for _, dep := range deps {
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

// DetectAllCycles checks all nodes in the provider for dependency cycles.
// Returns a slice of CycleResult, one for each unique cycle found.
// Returns an empty slice if no cycles are found.
//
// This function finds all distinct cycles in the graph, avoiding
// duplicate detection of the same cycle from different starting points.
func DetectAllCycles(provider DependencyProvider) []CycleResult {
	cycles := make([]CycleResult, 0)

	// Global color map for all nodes
	colors := make(map[string]int)

	// Track which nodes are part of already-found cycles to avoid duplicates
	inFoundCycle := make(map[string]bool)

	// Check each node
	for _, nodeID := range provider.AllNodeIDs() {
		idStr := nodeID.String()

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
		hasCycle, cyclePath := detectCycleDFSForAll(provider, nodeID, colors, path, inFoundCycle)
		if hasCycle && len(cyclePath) > 0 {
			cycles = append(cycles, CycleResult{
				HasCycle: true,
				Path:     cyclePath,
			})

			// Mark all nodes in this cycle as found
			for _, cycleNode := range cyclePath {
				inFoundCycle[cycleNode.String()] = true
			}
		}
	}

	return cycles
}

// detectCycleDFSForAll is similar to detectCycleDFS but uses a shared
// color map and inFoundCycle tracking to avoid duplicate cycle detection.
func detectCycleDFSForAll(provider DependencyProvider, nodeID types.NodeID, colors map[string]int, path []types.NodeID, inFoundCycle map[string]bool) (bool, []types.NodeID) {
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

		// Get the node's dependencies
		deps, ok := provider.GetNodeDependencies(nodeID)
		if !ok {
			// Missing node - treat as leaf (no dependencies)
			colors[idStr] = black
			return false, nil
		}

		// Extend path
		newPath := append(path, nodeID)

		// Visit all dependencies
		for _, dep := range deps {
			// Skip if we already found this node in a cycle
			if inFoundCycle[dep.String()] {
				continue
			}

			hasCycle, cyclePath := detectCycleDFSForAll(provider, dep, colors, newPath, inFoundCycle)
			if hasCycle {
				return true, cyclePath
			}
		}

		// Done with this node
		colors[idStr] = black
		return false, nil
	}
}

// WouldCreateCycle checks if adding a dependency from fromID to toID
// would create a cycle in the graph.
//
// This is useful for validating proposed dependencies before adding them.
// It simulates adding the dependency and checks for cycles starting from
// the target node (toID) to see if it can reach back to fromID.
//
// Returns CycleResult indicating whether the proposed dependency would
// create a cycle, and the cycle path if so.
func WouldCreateCycle(provider DependencyProvider, fromID, toID types.NodeID) CycleResult {
	// Self-reference is always a cycle
	if fromID.String() == toID.String() {
		return CycleResult{
			HasCycle: true,
			Path:     []types.NodeID{fromID, toID},
		}
	}

	// If fromID doesn't exist in the provider, it has no existing deps,
	// so adding a dependency from it cannot create a cycle
	if _, ok := provider.GetNodeDependencies(fromID); !ok {
		return CycleResult{HasCycle: false}
	}

	// Create a wrapper provider that includes the proposed dependency
	wrapper := &dependencyWrapper{
		provider: provider,
		fromID:   fromID,
		toID:     toID,
	}

	// Check if adding this dependency creates a cycle starting from fromID
	return DetectCycleFrom(wrapper, fromID)
}

// dependencyWrapper wraps a DependencyProvider to add a proposed dependency
// for cycle checking purposes.
type dependencyWrapper struct {
	provider DependencyProvider
	fromID   types.NodeID
	toID     types.NodeID
}

func (w *dependencyWrapper) GetNodeDependencies(id types.NodeID) ([]types.NodeID, bool) {
	deps, ok := w.provider.GetNodeDependencies(id)
	if !ok {
		return nil, false
	}

	// If this is the fromID, add the proposed dependency
	if id.String() == w.fromID.String() {
		// Check if toID is already in deps
		for _, dep := range deps {
			if dep.String() == w.toID.String() {
				return deps, true // Already exists
			}
		}
		// Add the proposed dependency
		newDeps := make([]types.NodeID, len(deps)+1)
		copy(newDeps, deps)
		newDeps[len(deps)] = w.toID
		return newDeps, true
	}

	return deps, ok
}

func (w *dependencyWrapper) AllNodeIDs() []types.NodeID {
	return w.provider.AllNodeIDs()
}
