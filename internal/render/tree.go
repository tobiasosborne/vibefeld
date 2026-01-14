// Package render provides human-readable formatting for AF framework types.
package render

import (
	"sort"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
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
		renderSubtree(&sb, root, nodeMap, allNodes, "", i == len(rootNodes)-1, true, customRoot)
	}

	return sb.String()
}

// renderSubtree recursively renders a node and its children.
// isRoot indicates if this is the rendering root (the node we started rendering from).
func renderSubtree(
	sb *strings.Builder,
	n *node.Node,
	nodeMap map[string]*node.Node,
	allNodes []*node.Node,
	prefix string,
	isLast bool,
	isRoot bool,
	customRoot *types.NodeID,
) {
	// Render this node
	nodeStr := formatNode(n)

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
		renderSubtree(sb, child, nodeMap, allNodes, childPrefix, childIsLast, false, customRoot)
	}
}

// findChildren finds all direct children of a given node ID.
// If customRoot is set, only finds children that are descendants of customRoot.
func findChildren(parentID types.NodeID, allNodes []*node.Node, customRoot *types.NodeID) []*node.Node {
	var children []*node.Node
	parentStr := parentID.String()

	for _, n := range allNodes {
		// Check if this node is a direct child of the parent
		parent, hasParent := n.ID.Parent()
		if !hasParent {
			continue
		}

		if parent.String() == parentStr {
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
func isDescendantOrEqual(nodeID, ancestorID types.NodeID) bool {
	nodeStr := nodeID.String()
	ancestorStr := ancestorID.String()

	if nodeStr == ancestorStr {
		return true
	}

	// Check if nodeID starts with ancestorID followed by a dot
	return strings.HasPrefix(nodeStr, ancestorStr+".")
}

// formatNode formats a single node for tree display.
// Format: ID [epistemic/taint] statement
// Mathematical statements are shown in full without truncation to preserve precision.
func formatNode(n *node.Node) string {
	var sb strings.Builder

	// Node ID
	sb.WriteString(n.ID.String())
	sb.WriteString(" ")

	// Status bracket [epistemic/taint]
	sb.WriteString("[")
	sb.WriteString(string(n.EpistemicState))
	sb.WriteString("/")
	sb.WriteString(string(n.TaintState))
	sb.WriteString("] ")

	// Statement (sanitized but NOT truncated - mathematical formulas must be shown in full)
	stmt := sanitizeStatement(n.Statement)
	sb.WriteString(stmt)

	return sb.String()
}

// sortNodesByID sorts nodes by their hierarchical ID in numeric order.
func sortNodesByID(nodes []*node.Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return compareNodeIDs(nodes[i].ID.String(), nodes[j].ID.String())
	})
}
