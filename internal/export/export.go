// Package export provides proof export functionality to various formats.
package export

import (
	"fmt"
	"regexp"
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
// The output uses an indented step format matching Lamport structured proofs,
// with ket notation rendered in math mode.
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
	sb.WriteString("\\usepackage{amsmath,amssymb}\n")
	sb.WriteString("\\usepackage[most]{tcolorbox}\n")
	sb.WriteString("\\usepackage{geometry}\n")
	sb.WriteString("\\geometry{margin=2.5cm}\n\n")

	// Step macro: indented, numbered (3.5em accommodates IDs like 1.2.4)
	sb.WriteString("\\newcommand{\\step}[1]{\\par\\noindent\\hangindent=3.5em\\hangafter=1%\n")
	sb.WriteString("  \\makebox[3.5em][l]{\\textbf{#1.}}}\n")
	sb.WriteString("\\newcommand{\\steptag}[1]{\\hfill{\\small\\textsf{[#1]}}}\n\n")

	// Proof box
	sb.WriteString("\\newtcolorbox{proofbox}[1]{\n")
	sb.WriteString("  colback=gray!5, colframe=gray!60,\n")
	sb.WriteString("  fonttitle=\\bfseries, title={#1},\n")
	sb.WriteString("  breakable, enhanced,\n")
	sb.WriteString("  left=4pt, right=4pt, top=4pt, bottom=4pt,\n")
	sb.WriteString("  before skip=10pt, after skip=10pt\n")
	sb.WriteString("}\n\n")

	sb.WriteString("\\begin{document}\n\n")

	// Build tree structure
	root := buildTree(sortedNodes)
	if root != nil {
		// Title from root statement
		title := "Proof"
		if root.node != nil {
			stmt := root.node.Statement
			if len(stmt) > 60 {
				stmt = stmt[:57] + "..."
			}
			title = escapeLatex(stmt)
		}
		sb.WriteString(fmt.Sprintf("\\begin{proofbox}{%s}\n", title))
		renderLaTeXStep(&sb, root)
		sb.WriteString("\\end{proofbox}\n")
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

// renderLaTeXStep renders a node and its children as indented \step lines.
func renderLaTeXStep(sb *strings.Builder, tn *treeNode) {
	if tn == nil || tn.node == nil {
		return
	}

	n := tn.node

	// Build the step line: \step{ID} [TYPE PREFIX] statement [status tag]
	sb.WriteString(fmt.Sprintf("\\step{%s} ", n.ID.String()))

	// Add type prefix for non-claim types
	switch n.Type {
	case schema.NodeTypeLocalAssume:
		sb.WriteString("\\textsc{assume}: ")
	case schema.NodeTypeLocalDischarge:
		sb.WriteString("\\textsc{q.e.d.}: ")
	case schema.NodeTypeCase:
		sb.WriteString("\\textsc{case}: ")
	}

	// Statement text with ket notation converted to math mode
	sb.WriteString(latexKetNotation(escapeLatex(n.Statement)))

	// Status tag (only if not pending — pending is the default)
	if n.EpistemicState != "" && n.EpistemicState != schema.EpistemicPending {
		sb.WriteString(fmt.Sprintf(" \\steptag{%s}", escapeLatex(string(n.EpistemicState))))
	}

	if len(tn.children) > 0 {
		sb.WriteString(" \\\\[6pt]\n")
	} else {
		sb.WriteString(" \\\\[4pt]\n")
	}

	// Render children
	for _, child := range tn.children {
		renderLaTeXStep(sb, child)
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

// ketPattern matches Dirac ket notation like |psi>, |0>, |00>, |+>, |Phi+>, etc.
var ketPattern = regexp.MustCompile(`\|([A-Za-z0-9+\-\\^{}]+)>`)

// greekLetters maps common Greek letter names to LaTeX commands.
var greekLetters = map[string]string{
	"alpha": "\\alpha", "beta": "\\beta", "gamma": "\\gamma", "delta": "\\delta",
	"epsilon": "\\epsilon", "zeta": "\\zeta", "eta": "\\eta", "theta": "\\theta",
	"iota": "\\iota", "kappa": "\\kappa", "lambda": "\\lambda", "mu": "\\mu",
	"nu": "\\nu", "xi": "\\xi", "pi": "\\pi", "rho": "\\rho",
	"sigma": "\\sigma", "tau": "\\tau", "upsilon": "\\upsilon", "phi": "\\phi",
	"chi": "\\chi", "psi": "\\psi", "omega": "\\omega",
	"Gamma": "\\Gamma", "Delta": "\\Delta", "Theta": "\\Theta", "Lambda": "\\Lambda",
	"Xi": "\\Xi", "Pi": "\\Pi", "Sigma": "\\Sigma", "Upsilon": "\\Upsilon",
	"Phi": "\\Phi", "Psi": "\\Psi", "Omega": "\\Omega",
}

// latexKetNotation converts |X> patterns to $|X\rangle$ in LaTeX text,
// replacing Greek letter names with their LaTeX commands.
func latexKetNotation(s string) string {
	return ketPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract content between | and >
		inner := match[1 : len(match)-1]
		// Check if the content is a Greek letter name
		if cmd, ok := greekLetters[inner]; ok {
			return "$|" + cmd + "\\rangle$"
		}
		return "$|" + inner + "\\rangle$"
	})
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
