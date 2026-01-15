// Package export provides proof export functionality to various formats.
package export

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// ValidateFormat checks if the given format string is valid.
// Valid formats: markdown, md, latex, tex (case-insensitive).
func ValidateFormat(format string) error {
	f := strings.ToLower(format)
	switch f {
	case "markdown", "md", "latex", "tex":
		return nil
	default:
		return fmt.Errorf("invalid export format %q: must be one of: markdown, md, latex, tex", format)
	}
}

// Export exports the proof state to the specified format.
// Returns an error if the format is invalid.
func Export(s *state.State, format string) (string, error) {
	if err := ValidateFormat(format); err != nil {
		return "", err
	}

	f := strings.ToLower(format)
	switch f {
	case "markdown", "md":
		return ToMarkdown(s), nil
	case "latex", "tex":
		return ToLaTeX(s), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// ToMarkdown exports the proof state to Markdown format.
func ToMarkdown(s *state.State) string {
	if s == nil {
		return "# No Proof Data\n\nNo proof data available to export.\n"
	}

	nodes := s.AllNodes()
	if len(nodes) == 0 {
		return "# No Proof Data\n\nNo nodes in the proof tree.\n"
	}

	// Sort nodes by ID for deterministic output
	sortedNodes := sortNodesByID(nodes)

	var sb strings.Builder
	sb.WriteString("# Proof Export\n\n")

	// Build tree structure
	root := buildTree(sortedNodes)
	if root != nil {
		renderMarkdownNode(&sb, root, 1)
	}

	return sb.String()
}

// ToLaTeX exports the proof state to LaTeX format.
func ToLaTeX(s *state.State) string {
	if s == nil {
		return latexDocument("No proof data available to export.")
	}

	nodes := s.AllNodes()
	if len(nodes) == 0 {
		return latexDocument("No nodes in the proof tree.")
	}

	// Sort nodes by ID for deterministic output
	sortedNodes := sortNodesByID(nodes)

	var sb strings.Builder
	sb.WriteString("\\documentclass{article}\n")
	sb.WriteString("\\usepackage[utf8]{inputenc}\n")
	sb.WriteString("\\usepackage{amsmath}\n")
	sb.WriteString("\\usepackage{amssymb}\n")
	sb.WriteString("\\usepackage{enumitem}\n\n")
	sb.WriteString("\\title{Proof Export}\n")
	sb.WriteString("\\date{}\n\n")
	sb.WriteString("\\begin{document}\n")
	sb.WriteString("\\maketitle\n\n")

	// Build tree structure
	root := buildTree(sortedNodes)
	if root != nil {
		renderLaTeXNode(&sb, root, 0)
	}

	sb.WriteString("\n\\end{document}\n")
	return sb.String()
}

// =============================================================================
// Tree Building
// =============================================================================

// treeNode represents a node in the export tree structure.
type treeNode struct {
	node     *node.Node
	children []*treeNode
}

// buildTree builds a hierarchical tree from a flat list of nodes.
func buildTree(nodes []*node.Node) *treeNode {
	if len(nodes) == 0 {
		return nil
	}

	// Create map of node ID string -> node
	nodeMap := make(map[string]*node.Node)
	for _, n := range nodes {
		nodeMap[n.ID.String()] = n
	}

	// Create treeNode map
	treeMap := make(map[string]*treeNode)
	for _, n := range nodes {
		treeMap[n.ID.String()] = &treeNode{node: n}
	}

	// Link children to parents
	var root *treeNode
	for _, n := range nodes {
		tn := treeMap[n.ID.String()]
		if n.ID.IsRoot() {
			root = tn
		} else {
			parentID, ok := n.ID.Parent()
			if ok {
				if parent, exists := treeMap[parentID.String()]; exists {
					parent.children = append(parent.children, tn)
				}
			}
		}
	}

	// Sort children at each level for deterministic output
	sortTreeChildren(root)

	return root
}

// sortTreeChildren recursively sorts children at each level by their ID.
func sortTreeChildren(tn *treeNode) {
	if tn == nil {
		return
	}

	sort.Slice(tn.children, func(i, j int) bool {
		return compareNodeIDs(tn.children[i].node.ID, tn.children[j].node.ID) < 0
	})

	for _, child := range tn.children {
		sortTreeChildren(child)
	}
}

// =============================================================================
// Markdown Rendering
// =============================================================================

// renderMarkdownNode renders a node and its children in Markdown format.
func renderMarkdownNode(sb *strings.Builder, tn *treeNode, depth int) {
	if tn == nil || tn.node == nil {
		return
	}

	n := tn.node

	// Create header level based on depth (## for root, ### for children, etc.)
	headerLevel := depth + 1
	if headerLevel > 6 {
		headerLevel = 6 // Max markdown header level
	}
	header := strings.Repeat("#", headerLevel)

	// Write node header
	sb.WriteString(fmt.Sprintf("%s Node %s\n\n", header, n.ID.String()))

	// Write node details
	sb.WriteString(fmt.Sprintf("**Statement:** %s\n\n", n.Statement))
	sb.WriteString(fmt.Sprintf("**Type:** %s\n\n", formatNodeType(n.Type)))
	sb.WriteString(fmt.Sprintf("**Inference:** %s\n\n", formatInference(n.Inference)))
	sb.WriteString(fmt.Sprintf("**Status:** %s\n\n", n.EpistemicState))

	if n.TaintState != "" {
		sb.WriteString(fmt.Sprintf("**Taint:** %s\n\n", n.TaintState))
	}

	// Render children
	for _, child := range tn.children {
		renderMarkdownNode(sb, child, depth+1)
	}
}

// =============================================================================
// LaTeX Rendering
// =============================================================================

// renderLaTeXNode renders a node and its children in LaTeX format.
func renderLaTeXNode(sb *strings.Builder, tn *treeNode, depth int) {
	if tn == nil || tn.node == nil {
		return
	}

	n := tn.node

	// Use sections for top-level, subsections for children
	var sectionCmd string
	switch depth {
	case 0:
		sectionCmd = "\\section*"
	case 1:
		sectionCmd = "\\subsection*"
	case 2:
		sectionCmd = "\\subsubsection*"
	default:
		sectionCmd = "\\paragraph*"
	}

	// Write section header
	sb.WriteString(fmt.Sprintf("%s{Node %s}\n\n", sectionCmd, escapeLatex(n.ID.String())))

	// Write node content
	sb.WriteString(fmt.Sprintf("\\textbf{Statement:} %s\n\n", escapeLatex(n.Statement)))
	sb.WriteString(fmt.Sprintf("\\textbf{Type:} %s\n\n", escapeLatex(formatNodeType(n.Type))))
	sb.WriteString(fmt.Sprintf("\\textbf{Inference:} %s\n\n", escapeLatex(formatInference(n.Inference))))
	sb.WriteString(fmt.Sprintf("\\textbf{Status:} %s\n\n", escapeLatex(string(n.EpistemicState))))

	if n.TaintState != "" {
		sb.WriteString(fmt.Sprintf("\\textbf{Taint:} %s\n\n", escapeLatex(string(n.TaintState))))
	}

	// Render children
	if len(tn.children) > 0 {
		sb.WriteString("\\begin{enumerate}\n")
		for _, child := range tn.children {
			sb.WriteString("\\item ")
			renderLaTeXNodeAsItem(sb, child, depth+1)
		}
		sb.WriteString("\\end{enumerate}\n")
	}
}

// renderLaTeXNodeAsItem renders a node as a list item (for nested children).
func renderLaTeXNodeAsItem(sb *strings.Builder, tn *treeNode, depth int) {
	if tn == nil || tn.node == nil {
		return
	}

	n := tn.node

	// Write node content as item
	sb.WriteString(fmt.Sprintf("\\textbf{Node %s:} %s\n\n",
		escapeLatex(n.ID.String()),
		escapeLatex(n.Statement)))

	sb.WriteString(fmt.Sprintf("  Type: %s, Inference: %s, Status: %s",
		escapeLatex(formatNodeType(n.Type)),
		escapeLatex(formatInference(n.Inference)),
		escapeLatex(string(n.EpistemicState))))

	if n.TaintState != "" {
		sb.WriteString(fmt.Sprintf(", Taint: %s", escapeLatex(string(n.TaintState))))
	}
	sb.WriteString("\n\n")

	// Render children as nested list
	if len(tn.children) > 0 {
		sb.WriteString("\\begin{enumerate}\n")
		for _, child := range tn.children {
			sb.WriteString("\\item ")
			renderLaTeXNodeAsItem(sb, child, depth+1)
		}
		sb.WriteString("\\end{enumerate}\n")
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// latexDocument wraps content in a minimal LaTeX document.
func latexDocument(content string) string {
	return fmt.Sprintf(`\documentclass{article}
\usepackage[utf8]{inputenc}
\begin{document}
%s
\end{document}
`, content)
}

// escapeLatex escapes special LaTeX characters.
func escapeLatex(s string) string {
	// Order matters: backslash first
	replacements := []struct {
		old, new string
	}{
		{"\\", "\\textbackslash{}"},
		{"{", "\\{"},
		{"}", "\\}"},
		{"$", "\\$"},
		{"&", "\\&"},
		{"%", "\\%"},
		{"#", "\\#"},
		{"_", "\\_"},
		{"~", "\\textasciitilde{}"},
		{"^", "\\textasciicircum{}"},
	}

	result := s
	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.old, r.new)
	}
	return result
}

// formatNodeType returns a human-readable node type string.
func formatNodeType(t schema.NodeType) string {
	return string(t)
}

// formatInference returns a human-readable inference type string.
func formatInference(i schema.InferenceType) string {
	return string(i)
}

// sortNodesByID sorts nodes by their hierarchical ID.
func sortNodesByID(nodes []*node.Node) []*node.Node {
	sorted := make([]*node.Node, len(nodes))
	copy(sorted, nodes)

	sort.Slice(sorted, func(i, j int) bool {
		return compareNodeIDs(sorted[i].ID, sorted[j].ID) < 0
	})

	return sorted
}

// compareNodeIDs compares two NodeIDs lexicographically by their parts.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareNodeIDs(a, b types.NodeID) int {
	aStr := a.String()
	bStr := b.String()

	aParts := strings.Split(aStr, ".")
	bParts := strings.Split(bStr, ".")

	minLen := len(aParts)
	if len(bParts) < minLen {
		minLen = len(bParts)
	}

	for i := 0; i < minLen; i++ {
		aNum := 0
		bNum := 0
		fmt.Sscanf(aParts[i], "%d", &aNum)
		fmt.Sscanf(bParts[i], "%d", &bNum)

		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}

	// Same prefix - shorter ID comes first
	if len(aParts) < len(bParts) {
		return -1
	}
	if len(aParts) > len(bParts) {
		return 1
	}
	return 0
}
