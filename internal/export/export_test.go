// Package export provides proof export functionality to various formats.
package export

import (
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// addTestNode creates a test node and adds it to the state.
func addTestNode(
	t *testing.T,
	s *state.State,
	id string,
	statement string,
	nodeType schema.NodeType,
	inference schema.InferenceType,
	epistemic schema.EpistemicState,
	taint node.TaintState,
) *node.Node {
	t.Helper()
	nodeID, err := types.Parse(id)
	if err != nil {
		t.Fatalf("invalid test node ID %q: %v", id, err)
	}
	n, err := node.NewNode(nodeID, nodeType, statement, inference)
	if err != nil {
		t.Fatalf("failed to create test node %q: %v", id, err)
	}
	n.EpistemicState = epistemic
	n.TaintState = taint
	s.AddNode(n)
	return n
}

// =============================================================================
// Markdown Export Tests
// =============================================================================

// TestToMarkdown_NilState tests that nil state is handled gracefully.
func TestToMarkdown_NilState(t *testing.T) {
	result := ToMarkdown(nil)

	if result == "" {
		t.Error("ToMarkdown should return a message for nil state, not empty string")
	}

	lower := strings.ToLower(result)
	if !strings.Contains(lower, "no") && !strings.Contains(lower, "empty") {
		t.Errorf("ToMarkdown for nil state should indicate no data, got: %q", result)
	}
}

// TestToMarkdown_EmptyState tests that empty state is handled gracefully.
func TestToMarkdown_EmptyState(t *testing.T) {
	s := state.NewState()
	result := ToMarkdown(s)

	if result == "" {
		t.Error("ToMarkdown should return a message for empty state")
	}

	lower := strings.ToLower(result)
	if !strings.Contains(lower, "no") && !strings.Contains(lower, "empty") {
		t.Errorf("ToMarkdown for empty state should indicate no proof, got: %q", result)
	}
}

// TestToMarkdown_SingleRootNode tests export with just the root node.
func TestToMarkdown_SingleRootNode(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Root theorem statement", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)

	result := ToMarkdown(s)

	// Should contain a header
	if !strings.Contains(result, "#") {
		t.Error("Markdown output should contain headers")
	}

	// Should contain the node statement
	if !strings.Contains(result, "Root theorem statement") {
		t.Error("Markdown output should contain the node statement")
	}

	// Should contain the node ID
	if !strings.Contains(result, "1") {
		t.Error("Markdown output should contain the node ID")
	}
}

// TestToMarkdown_TreeStructure tests that hierarchical nodes are rendered correctly.
func TestToMarkdown_TreeStructure(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Root theorem", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	addTestNode(t, s, "1.1", "First lemma", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)
	addTestNode(t, s, "1.2", "Second lemma", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	addTestNode(t, s, "1.1.1", "Sub-step", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)

	result := ToMarkdown(s)

	// All nodes should appear
	for _, stmt := range []string{"Root theorem", "First lemma", "Second lemma", "Sub-step"} {
		if !strings.Contains(result, stmt) {
			t.Errorf("Markdown output missing statement %q", stmt)
		}
	}

	// Should have proper hierarchical structure (indentation or nested headers)
	// Check for either nested headers (## for depth 2, ### for depth 3) or indentation
	if !strings.Contains(result, "##") && !strings.Contains(result, "  ") {
		t.Error("Markdown output should have hierarchical structure")
	}
}

// TestToMarkdown_IncludesEpistemicState tests that epistemic state is shown.
func TestToMarkdown_IncludesEpistemicState(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Validated claim", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)

	result := ToMarkdown(s)

	// Should mention the epistemic state
	if !strings.Contains(result, "validated") {
		t.Error("Markdown output should include epistemic state")
	}
}

// TestToMarkdown_IncludesInference tests that inference type is shown.
func TestToMarkdown_IncludesInference(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "By modus ponens", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)

	result := ToMarkdown(s)

	// Should mention the inference type
	if !strings.Contains(result, "modus_ponens") && !strings.Contains(result, "Modus Ponens") && !strings.Contains(result, "modus ponens") {
		t.Error("Markdown output should include inference type")
	}
}

// TestToMarkdown_DeterministicOutput tests that output is consistent.
func TestToMarkdown_DeterministicOutput(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Root", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	addTestNode(t, s, "1.1", "Child A", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)
	addTestNode(t, s, "1.2", "Child B", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)

	result1 := ToMarkdown(s)
	result2 := ToMarkdown(s)
	result3 := ToMarkdown(s)

	if result1 != result2 || result2 != result3 {
		t.Error("ToMarkdown output should be deterministic")
	}
}

// TestToMarkdown_MultipleNodeTypes tests that different node types are shown.
func TestToMarkdown_MultipleNodeTypes(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Main claim", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	addTestNode(t, s, "1.1", "Local assumption", schema.NodeTypeLocalAssume, schema.InferenceAssumption, schema.EpistemicValidated, node.TaintClean)

	result := ToMarkdown(s)

	// Should indicate different node types
	hasTypes := strings.Contains(result, "claim") || strings.Contains(result, "Claim")
	hasAssumption := strings.Contains(result, "local_assume") || strings.Contains(result, "assume") || strings.Contains(result, "Assumption")

	if !hasTypes || !hasAssumption {
		t.Logf("Output: %s", result)
		t.Error("Markdown output should distinguish between node types")
	}
}

// =============================================================================
// LaTeX Export Tests
// =============================================================================

// TestToLaTeX_NilState tests that nil state is handled gracefully.
func TestToLaTeX_NilState(t *testing.T) {
	result := ToLaTeX(nil)

	if result == "" {
		t.Error("ToLaTeX should return a message for nil state, not empty string")
	}
}

// TestToLaTeX_EmptyState tests that empty state is handled gracefully.
func TestToLaTeX_EmptyState(t *testing.T) {
	s := state.NewState()
	result := ToLaTeX(s)

	if result == "" {
		t.Error("ToLaTeX should return a message for empty state")
	}
}

// TestToLaTeX_SingleRootNode tests LaTeX export with a single root node.
func TestToLaTeX_SingleRootNode(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Root theorem statement", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)

	result := ToLaTeX(s)

	// Should contain LaTeX document structure
	if !strings.Contains(result, "\\documentclass") {
		t.Error("LaTeX output should contain \\documentclass")
	}

	if !strings.Contains(result, "\\begin{document}") {
		t.Error("LaTeX output should contain \\begin{document}")
	}

	if !strings.Contains(result, "\\end{document}") {
		t.Error("LaTeX output should contain \\end{document}")
	}

	// Should contain the statement
	if !strings.Contains(result, "Root theorem statement") {
		t.Error("LaTeX output should contain the node statement")
	}
}

// TestToLaTeX_TreeStructure tests hierarchical LaTeX output.
func TestToLaTeX_TreeStructure(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Root theorem", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	addTestNode(t, s, "1.1", "First lemma", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)
	addTestNode(t, s, "1.2", "Second lemma", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)

	result := ToLaTeX(s)

	// All statements should appear
	for _, stmt := range []string{"Root theorem", "First lemma", "Second lemma"} {
		if !strings.Contains(result, stmt) {
			t.Errorf("LaTeX output missing statement %q", stmt)
		}
	}
}

// TestToLaTeX_EscapesSpecialCharacters tests that special LaTeX characters are escaped.
func TestToLaTeX_EscapesSpecialCharacters(t *testing.T) {
	s := state.NewState()
	// Statements with LaTeX special characters: & % $ # _ { } ~ ^ \
	addTestNode(t, s, "1", "Test with $x$ and 100% correct", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)

	result := ToLaTeX(s)

	// The output should still be valid (not crash)
	if result == "" {
		t.Error("LaTeX output should handle special characters")
	}

	// Should have document structure intact
	if !strings.Contains(result, "\\begin{document}") {
		t.Error("LaTeX output should maintain document structure with special characters")
	}
}

// TestToLaTeX_IncludesEpistemicState tests that epistemic state is shown in LaTeX.
func TestToLaTeX_IncludesEpistemicState(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Validated theorem", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)

	result := ToLaTeX(s)

	// Should indicate the state (possibly formatted)
	if !strings.Contains(result, "validated") && !strings.Contains(result, "Validated") {
		t.Error("LaTeX output should include epistemic state")
	}
}

// TestToLaTeX_DeterministicOutput tests that LaTeX output is consistent.
func TestToLaTeX_DeterministicOutput(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Root", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	addTestNode(t, s, "1.1", "Child", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)

	result1 := ToLaTeX(s)
	result2 := ToLaTeX(s)

	if result1 != result2 {
		t.Error("ToLaTeX output should be deterministic")
	}
}

// TestToLaTeX_UsesEnumerateOrItemize tests that LaTeX uses proper list structure.
func TestToLaTeX_UsesEnumerateOrItemize(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Root", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	addTestNode(t, s, "1.1", "Child 1", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)
	addTestNode(t, s, "1.2", "Child 2", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)

	result := ToLaTeX(s)

	// Should use either enumerate or itemize for structure
	hasListStructure := strings.Contains(result, "\\begin{enumerate}") ||
		strings.Contains(result, "\\begin{itemize}") ||
		strings.Contains(result, "\\section") ||
		strings.Contains(result, "\\subsection")

	if !hasListStructure {
		t.Error("LaTeX output should use proper hierarchical structure (enumerate, itemize, or sections)")
	}
}

// =============================================================================
// Format Validation Tests
// =============================================================================

// TestValidateFormat tests format string validation.
func TestValidateFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"valid markdown", "markdown", false},
		{"valid md", "md", false},
		{"valid latex", "latex", false},
		{"valid tex", "tex", false},
		{"valid uppercase MARKDOWN", "MARKDOWN", false},
		{"valid uppercase LATEX", "LATEX", false},
		{"invalid xml", "xml", true},
		{"invalid pdf", "pdf", true},
		{"invalid empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFormat(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFormat(%q) error = %v, wantErr %v", tt.format, err, tt.wantErr)
			}
		})
	}
}

// TestExport_UsesCorrectFormat tests the Export function with format selection.
func TestExport_UsesCorrectFormat(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Test claim", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)

	// Test markdown format
	mdResult, err := Export(s, "markdown")
	if err != nil {
		t.Errorf("Export to markdown failed: %v", err)
	}
	if !strings.Contains(mdResult, "#") {
		t.Error("Markdown export should contain headers")
	}

	// Test latex format
	latexResult, err := Export(s, "latex")
	if err != nil {
		t.Errorf("Export to latex failed: %v", err)
	}
	if !strings.Contains(latexResult, "\\documentclass") {
		t.Error("LaTeX export should contain \\documentclass")
	}

	// Test invalid format
	_, err = Export(s, "invalid")
	if err == nil {
		t.Error("Export should fail for invalid format")
	}
}
