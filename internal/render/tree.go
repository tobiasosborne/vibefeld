// Package render provides human-readable formatting for AF framework types.
package render

import (
	"sort"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// Tree drawing characters (Unicode box-drawing)
const (
	treeBranch   = "\u251c\u2500\u2500 " // ├── (middle child)
	treeLastNode = "\u2514\u2500\u2500 " // └── (last child)
	treeVertical = "\u2502   "           // │   (continuing line)
	treeSpace    = "    "                // spaces (no continuing line)
)

// RenderTree renders a proof tree as a human-readable string with tree structure.
// If customRoot is provided, only the subtree starting at that node is rendered.
// Returns an empty string for nil or empty state.
func RenderTree(s *state.State, customRoot *types.NodeID) string {
	if s == nil {
		return ""
	}

	allNodes := s.AllNodes()
	if len(allNodes) == 0 {
		return ""
	}

	// Build a map for quick lookup
	nodeMap := make(map[string]*node.Node)
	for _, n := range allNodes {
		nodeMap[n.ID.String()] = n
	}

	// Determine the root node(s) to render
	var rootNodes []*node.Node

	if customRoot != nil {
		// Find the specific root node
		rootNode := nodeMap[customRoot.String()]
		if rootNode == nil {
			return ""
		}
		rootNodes = []*node.Node{rootNode}
	} else {
		// Find all top-level root nodes (depth 1)
		for _, n := range allNodes {
			if n.ID.IsRoot() {
				rootNodes = append(rootNodes, n)
			}
		}
	}

	if len(rootNodes) == 0 {
		return ""
	}

	// Sort root nodes by ID
	sortNodesByID(rootNodes)

	// Build the tree output
	var sb strings.Builder
	for i, root := range rootNodes {
		renderSubtree(&sb, s, root, nodeMap, allNodes, "", i == len(rootNodes)-1, true, customRoot)
	}

	return sb.String()
}

// RenderTreeForNodes renders a flat list of nodes without tree structure.
// This is used for paginated output where tree hierarchy may be incomplete.
// Each node is rendered on its own line with indentation based on depth.
// Returns an empty string for nil or empty node list.
func RenderTreeForNodes(s *state.State, nodes []*node.Node) string {
	if len(nodes) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, n := range nodes {
		// Indent based on depth (2 spaces per level)
		depth := n.ID.Depth()
		indent := strings.Repeat("  ", depth-1)
		nodeStr := formatNodeWithState(n, s)
		sb.WriteString(indent)
		sb.WriteString(nodeStr)
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderSubtree recursively renders a node and its children.
// isRoot indicates if this is the rendering root (the node we started rendering from).
func renderSubtree(
	sb *strings.Builder,
	s *state.State,
	n *node.Node,
	nodeMap map[string]*node.Node,
	allNodes []*node.Node,
	prefix string,
	isLast bool,
	isRoot bool,
	customRoot *types.NodeID,
) {
	// Render this node with state context for validation dependency info
	nodeStr := formatNodeWithState(n, s)

	// For the root node, just write the node line (no branch characters)
	if isRoot {
		sb.WriteString(nodeStr)
	} else {
		// For child nodes, add the appropriate branch character
		if isLast {
			sb.WriteString(prefix + treeLastNode + nodeStr)
		} else {
			sb.WriteString(prefix + treeBranch + nodeStr)
		}
	}
	sb.WriteString("\n")

	// Find children of this node
	children := findChildren(n.ID, allNodes, customRoot)
	sortNodesByID(children)

	// Calculate the new prefix for children
	var childPrefix string
	if isRoot {
		// Root node - children start with no prefix (they'll add their branch chars)
		childPrefix = ""
	} else {
		// Non-root node - add vertical line or space based on whether we're the last child
		if isLast {
			childPrefix = prefix + treeSpace
		} else {
			childPrefix = prefix + treeVertical
		}
	}

	// Render children
	for i, child := range children {
		childIsLast := i == len(children)-1
		renderSubtree(sb, s, child, nodeMap, allNodes, childPrefix, childIsLast, false, customRoot)
	}
}

// findChildren finds all direct children of a given node ID.
// If customRoot is set, only finds children that are descendants of customRoot.
// Uses NodeID.Equal() for efficient comparison without string allocations.
func findChildren(parentID types.NodeID, allNodes []*node.Node, customRoot *types.NodeID) []*node.Node {
	var children []*node.Node

	for _, n := range allNodes {
		// Check if this node is a direct child of the parent
		parent, hasParent := n.ID.Parent()
		if !hasParent {
			continue
		}

		if parent.Equal(parentID) {
			// If customRoot is set, only include nodes that are descendants
			if customRoot != nil {
				// Check if node is under the custom root
				if !isDescendantOrEqual(n.ID, *customRoot) {
					continue
				}
			}
			children = append(children, n)
		}
	}

	return children
}

// isDescendantOrEqual returns true if nodeID is equal to or a descendant of ancestorID.
// Uses NodeID methods directly to avoid string allocations.
func isDescendantOrEqual(nodeID, ancestorID types.NodeID) bool {
	if nodeID.Equal(ancestorID) {
		return true
	}
	return ancestorID.IsAncestorOf(nodeID)
}

// formatNode formats a single node for tree display.
// Format: ID [epistemic/taint] statement
// Mathematical statements are shown in full without truncation to preserve precision.
// Uses color coding for epistemic and taint states when color is enabled.
func formatNode(n *node.Node) string {
	return formatNodeWithState(n, nil)
}

// formatNodeWithState formats a single node for tree display with state context.
// When state is provided, includes validation dependency status.
// Format: ID [epistemic/taint] statement [deps] (if has validation deps)
// Mathematical statements are shown in full without truncation to preserve precision.
// Uses color coding for epistemic and taint states when color is enabled.
func formatNodeWithState(n *node.Node, s *state.State) string {
	var sb strings.Builder

	// Node ID
	sb.WriteString(n.ID.String())
	sb.WriteString(" ")

	// Status bracket [epistemic/taint] with color coding
	sb.WriteString("[")
	sb.WriteString(ColorEpistemicState(n.EpistemicState))
	sb.WriteString("/")
	sb.WriteString(ColorTaintState(n.TaintState))
	sb.WriteString("] ")

	// Statement (sanitized but NOT truncated - mathematical formulas must be shown in full)
	stmt := sanitizeStatement(n.Statement)
	sb.WriteString(stmt)

	// Show validation dependency status if node has validation deps and state is provided
	if s != nil && len(n.ValidationDeps) > 0 {
		blockedCount := countUnvalidatedDeps(n, s)
		if blockedCount > 0 {
			sb.WriteString(" ")
			sb.WriteString(Red("[BLOCKED: "))
			sb.WriteString(Red(formatBlockedDeps(n, s)))
			sb.WriteString(Red("]"))
		}
	}

	return sb.String()
}

// countUnvalidatedDeps counts how many validation dependencies are not yet validated.
func countUnvalidatedDeps(n *node.Node, s *state.State) int {
	count := 0
	for _, depID := range n.ValidationDeps {
		depNode := s.GetNode(depID)
		if depNode == nil || (depNode.EpistemicState != schema.EpistemicValidated && depNode.EpistemicState != schema.EpistemicAdmitted) {
			count++
		}
	}
	return count
}

// formatBlockedDeps formats the list of blocking validation dependencies.
func formatBlockedDeps(n *node.Node, s *state.State) string {
	var blocked []string
	for _, depID := range n.ValidationDeps {
		depNode := s.GetNode(depID)
		if depNode == nil || (depNode.EpistemicState != schema.EpistemicValidated && depNode.EpistemicState != schema.EpistemicAdmitted) {
			blocked = append(blocked, depID.String())
		}
	}
	return strings.Join(blocked, ", ")
}

// sortNodesByID sorts nodes by their hierarchical ID in numeric order.
// Uses NodeID.Less() directly to avoid string allocations.
func sortNodesByID(nodes []*node.Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID.Less(nodes[j].ID)
	})
}
