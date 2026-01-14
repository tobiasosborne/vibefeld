// Package taint provides taint computation and propagation logic for AF nodes.
package taint

import (
	"sort"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
)

// PropagateTaint updates taint for a node and all its descendants.
// It uses ComputeTaint to calculate the correct taint state for each descendant
// based on their epistemic state and ancestors.
//
// Returns list of nodes whose taint actually changed.
// The root node itself is never included in the returned list.
//
// Returns nil/empty slice if:
// - root is nil
// - allNodes is nil or empty
// - root has no descendants in allNodes
func PropagateTaint(root *node.Node, allNodes []*node.Node) []*node.Node {
	if root == nil || len(allNodes) == 0 {
		return nil
	}

	// Build a map for quick lookup by node ID string
	nodeMap := make(map[string]*node.Node)
	for _, n := range allNodes {
		if n != nil {
			nodeMap[n.ID.String()] = n
		}
	}

	// Find all descendants of root
	var descendants []*node.Node
	for _, n := range allNodes {
		if n != nil && root.ID.IsAncestorOf(n.ID) {
			descendants = append(descendants, n)
		}
	}

	// Sort descendants by depth (shallower first) to process in order
	// This ensures parent taint is updated before children
	sortByDepth(descendants)

	var changed []*node.Node

	// Optimization: Build ancestors incrementally as we process nodes.
	// Since nodes are sorted by depth (shallower first), when we process a node,
	// its parent has already been processed. We cache: node -> ancestors list.
	// Each node's ancestors = parent's ancestors + parent.
	// This reduces complexity from O(N*D) to O(N) where D is tree depth.
	ancestorCache := make(map[string][]*node.Node)

	// Pre-populate cache with root's ancestors (computed once)
	rootAncestors := getAncestors(root, nodeMap)
	ancestorCache[root.ID.String()] = rootAncestors

	for _, desc := range descendants {
		// Get ancestors from cache using parent lookup
		ancestors := getAncestorsCached(desc, nodeMap, ancestorCache)

		// Compute correct taint
		newTaint := ComputeTaint(desc, ancestors)

		// If taint changed, update and record
		if desc.TaintState != newTaint {
			desc.TaintState = newTaint
			changed = append(changed, desc)
		}
	}

	return changed
}

// getAncestors returns all ancestor nodes for the given node from the nodeMap.
// This walks the tree from node to root - O(D) where D is depth.
// Used for initial computation or when cache is not available.
func getAncestors(n *node.Node, nodeMap map[string]*node.Node) []*node.Node {
	var ancestors []*node.Node
	parentID, hasParent := n.ID.Parent()
	for hasParent {
		if parent, ok := nodeMap[parentID.String()]; ok {
			ancestors = append(ancestors, parent)
		}
		parentID, hasParent = parentID.Parent()
	}
	return ancestors
}

// getAncestorsCached returns ancestors for a node using cached results.
// Since nodes are processed in depth order (shallower first), the parent's
// ancestors are always computed before the child's.
//
// Algorithm: node's ancestors = [parent] + parent's ancestors
// This gives O(1) lookup per node after parent is cached.
//
// Falls back to getAncestors if parent not in cache (handles sparse trees).
func getAncestorsCached(n *node.Node, nodeMap map[string]*node.Node, cache map[string][]*node.Node) []*node.Node {
	nodeKey := n.ID.String()

	// Check if already computed
	if cached, ok := cache[nodeKey]; ok {
		return cached
	}

	parentID, hasParent := n.ID.Parent()
	if !hasParent {
		// Node is a root, no ancestors
		cache[nodeKey] = nil
		return nil
	}

	parentKey := parentID.String()
	parent, parentInNodeMap := nodeMap[parentKey]

	// If parent's ancestors are cached, build from them
	if parentAncestors, parentCached := cache[parentKey]; parentCached {
		var ancestors []*node.Node
		if parentInNodeMap {
			// ancestors = [parent] + parent's ancestors
			ancestors = make([]*node.Node, 0, len(parentAncestors)+1)
			ancestors = append(ancestors, parent)
			ancestors = append(ancestors, parentAncestors...)
		} else {
			// Parent not in nodeMap, just use parent's ancestors
			ancestors = parentAncestors
		}
		cache[nodeKey] = ancestors
		return ancestors
	}

	// Fallback: parent not cached (shouldn't happen with depth-order processing)
	// Compute directly and cache
	ancestors := getAncestors(n, nodeMap)
	cache[nodeKey] = ancestors
	return ancestors
}

// sortByDepth sorts nodes by their ID depth (shallower first).
func sortByDepth(nodes []*node.Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID.Depth() < nodes[j].ID.Depth()
	})
}

// GenerateTaintEvents creates TaintRecomputed events for all changed nodes.
// This function should be called after PropagateTaint to generate ledger events
// for nodes whose taint state has changed.
//
// Returns a slice of TaintRecomputed events, one for each changed node.
// Returns nil if changedNodes is nil or empty.
func GenerateTaintEvents(changedNodes []*node.Node) []ledger.TaintRecomputed {
	if len(changedNodes) == 0 {
		return nil
	}

	events := make([]ledger.TaintRecomputed, 0, len(changedNodes))
	for _, n := range changedNodes {
		if n != nil {
			events = append(events, ledger.NewTaintRecomputed(n.ID, n.TaintState))
		}
	}
	return events
}

// PropagateAndGenerateEvents is a convenience function that propagates taint
// and generates TaintRecomputed events in a single call.
//
// It combines PropagateTaint and GenerateTaintEvents for common use cases
// where both operations are needed together.
//
// Returns:
//   - changedNodes: nodes whose taint state was updated
//   - events: TaintRecomputed events for each changed node
func PropagateAndGenerateEvents(root *node.Node, allNodes []*node.Node) ([]*node.Node, []ledger.TaintRecomputed) {
	changedNodes := PropagateTaint(root, allNodes)
	events := GenerateTaintEvents(changedNodes)
	return changedNodes, events
}
