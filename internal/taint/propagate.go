// Package taint provides taint computation and propagation logic for AF nodes.
package taint

import (
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

	for _, desc := range descendants {
		// Build ancestor list for this descendant
		ancestors := getAncestors(desc, nodeMap)

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

// sortByDepth sorts nodes by their ID depth (shallower first).
func sortByDepth(nodes []*node.Node) {
	// Simple bubble sort - good enough for typical proof trees
	for i := 0; i < len(nodes)-1; i++ {
		for j := 0; j < len(nodes)-i-1; j++ {
			if nodes[j].ID.Depth() > nodes[j+1].ID.Depth() {
				nodes[j], nodes[j+1] = nodes[j+1], nodes[j]
			}
		}
	}
}
