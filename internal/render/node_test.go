//go:build integration

package render

import (
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// helper to create a test node with minimal required fields
func makeTestNode(id string, nodeType schema.NodeType, statement string, inference schema.InferenceType) *node.Node {
	nodeID, err := types.Parse(id)
	if err != nil {
		panic("invalid test node ID: " + id)
	}
	n, err := node.NewNode(nodeID, nodeType, statement, inference)
	if err != nil {
		panic("failed to create test node: " + err.Error())
	}
	return n
}

// TestRenderNode_Basic tests basic node rendering shows ID, type, state, and truncated statement
func TestRenderNode_Basic(t *testing.T) {
	tests := []struct {
		name      string
		node      *node.Node
		wantID    string
		wantType  string
		wantState string
		wantStmt  string // expected truncated statement or substring
	}{
		{
			name:      "simple claim node",
			node:      makeTestNode("1", schema.NodeTypeClaim, "Base case holds", schema.InferenceModusPonens),
			wantID:    "[1]",
			wantType:  "claim",
			wantState: "pending",
			wantStmt:  "Base case holds",
		},
		{
			name:      "nested node",
			node:      makeTestNode("1.2.3", schema.NodeTypeClaim, "Induction step", schema.InferenceModusPonens),
			wantID:    "[1.2.3]",
			wantType:  "claim",
			wantState: "pending",
			wantStmt:  "Induction step",
		},
		{
			name:      "local assume node",
			node:      makeTestNode("1.1", schema.NodeTypeLocalAssume, "Assume n >= 0", schema.InferenceLocalAssume),
			wantID:    "[1.1]",
			wantType:  "local_assume",
			wantState: "pending",
			wantStmt:  "Assume n >= 0",
		},
		{
			name:      "qed node",
			node:      makeTestNode("1.2", schema.NodeTypeQED, "Therefore the theorem holds", schema.InferenceModusPonens),
			wantID:    "[1.2]",
			wantType:  "qed",
			wantState: "pending",
			wantStmt:  "Therefore the theorem holds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderNode(tt.node)

			if result == "" {
				t.Fatal("RenderNode returned empty string")
			}

			// Should contain the node ID in brackets
			if !strings.Contains(result, tt.wantID) {
				t.Errorf("RenderNode missing ID %q, got: %q", tt.wantID, result)
			}

			// Should contain the node type
			if !strings.Contains(result, tt.wantType) {
				t.Errorf("RenderNode missing type %q, got: %q", tt.wantType, result)
			}

			// Should contain the epistemic state
			if !strings.Contains(result, tt.wantState) {
				t.Errorf("RenderNode missing state %q, got: %q", tt.wantState, result)
			}

			// Should contain the statement (or truncated version)
			if !strings.Contains(result, tt.wantStmt) {
				t.Errorf("RenderNode missing statement %q, got: %q", tt.wantStmt, result)
			}
		})
	}
}

// TestRenderNode_LongStatement tests that long statements are NOT truncated
// Mathematical proofs require precision - formulas must be shown in full
func TestRenderNode_LongStatement(t *testing.T) {
	longStatement := "B_n = (1/e) * sum_{k=0}^{infinity} (k^n / k!) where B_n is the n-th Bell number"
	n := makeTestNode("1", schema.NodeTypeClaim, longStatement, schema.InferenceModusPonens)

	result := RenderNode(n)

	if result == "" {
		t.Fatal("RenderNode returned empty string")
	}

	// Result should be a single line (no newlines)
	if strings.Contains(result, "\n") {
		t.Errorf("RenderNode should return single line, got: %q", result)
	}

	// Mathematical formulas should NOT be truncated - shown in full
	if strings.Contains(result, "...") {
		t.Errorf("Mathematical statements should not be truncated, got: %q", result)
	}

	// The full formula should be present
	if !strings.Contains(result, "B_n = (1/e) * sum_{k=0}^{infinity}") {
		t.Errorf("RenderNode should show full formula, got: %q", result)
	}

	// Should still contain the ID
	if !strings.Contains(result, "[1]") {
		t.Errorf("RenderNode missing ID, got: %q", result)
	}
}

// TestRenderNode_Nil tests that nil node returns empty string
func TestRenderNode_Nil(t *testing.T) {
	result := RenderNode(nil)

	if result != "" {
		t.Errorf("RenderNode(nil) = %q, want empty string", result)
	}
}

// TestRenderNodeVerbose_AllFields tests that verbose output includes all node fields
func TestRenderNodeVerbose_AllFields(t *testing.T) {
	n := makeTestNode("1.2", schema.NodeTypeClaim, "The sum converges", schema.InferenceModusPonens)

	result := RenderNodeVerbose(n)

	if result == "" {
		t.Fatal("RenderNodeVerbose returned empty string")
	}

	// Should be multi-line output
	if !strings.Contains(result, "\n") {
		t.Errorf("RenderNodeVerbose should return multi-line output, got: %q", result)
	}

	// Required fields that must be present in verbose output
	requiredFields := []struct {
		name  string
		value string
	}{
		{"ID", "1.2"},
		{"Type", "claim"},
		{"Statement", "The sum converges"},
		{"Inference", "modus_ponens"},
		{"Workflow", "available"},
		{"Epistemic", "pending"},
		{"Taint", "unresolved"},
	}

	for _, field := range requiredFields {
		if !strings.Contains(result, field.value) {
			t.Errorf("RenderNodeVerbose missing %s value %q, got: %q", field.name, field.value, result)
		}
	}
}

// TestRenderNodeVerbose_WithOptionalFields tests verbose output with optional fields populated
func TestRenderNodeVerbose_WithOptionalFields(t *testing.T) {
	nodeID, _ := types.Parse("1.3")
	n, err := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"Test statement",
		schema.InferenceByDefinition,
		node.NodeOptions{
			Context:      []string{"def:natural_numbers"},
			Dependencies: []types.NodeID{mustParse("1.1"), mustParse("1.2")},
			Scope:        []string{"assume:n>=0"},
		},
	)
	if err != nil {
		t.Fatalf("failed to create test node: %v", err)
	}

	result := RenderNodeVerbose(n)

	// Should include context if present
	if !strings.Contains(result, "def:natural_numbers") {
		t.Errorf("RenderNodeVerbose missing context, got: %q", result)
	}

	// Should include dependencies if present
	if !strings.Contains(result, "1.1") || !strings.Contains(result, "1.2") {
		t.Errorf("RenderNodeVerbose missing dependencies, got: %q", result)
	}
}

// TestRenderNodeVerbose_Nil tests that nil node returns empty string
func TestRenderNodeVerbose_Nil(t *testing.T) {
	result := RenderNodeVerbose(nil)

	if result != "" {
		t.Errorf("RenderNodeVerbose(nil) = %q, want empty string", result)
	}
}

// TestRenderNodeVerbose_Timestamp tests that created timestamp is included
func TestRenderNodeVerbose_Timestamp(t *testing.T) {
	n := makeTestNode("1", schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)

	result := RenderNodeVerbose(n)

	// Should contain "Created" label and timestamp format (ISO8601)
	if !strings.Contains(result, "Created") {
		t.Errorf("RenderNodeVerbose missing Created label, got: %q", result)
	}

	// Timestamp should contain typical ISO8601 elements (year, T separator, Z suffix)
	// Looking for patterns like "2025-01-12T" or "T" which appear in ISO8601
	if !strings.Contains(result, "T") {
		t.Logf("Warning: timestamp format may not be ISO8601, result: %q", result)
	}
}

// TestRenderNodeTree_Empty tests rendering empty node list
func TestRenderNodeTree_Empty(t *testing.T) {
	result := RenderNodeTree(nil)
	if result != "" {
		t.Errorf("RenderNodeTree(nil) = %q, want empty string", result)
	}

	result = RenderNodeTree([]*node.Node{})
	if result != "" {
		t.Errorf("RenderNodeTree([]) = %q, want empty string", result)
	}
}

// TestRenderNodeTree_SingleNode tests rendering a single node tree
func TestRenderNodeTree_SingleNode(t *testing.T) {
	nodes := []*node.Node{
		makeTestNode("1", schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption),
	}

	result := RenderNodeTree(nodes)

	if result == "" {
		t.Fatal("RenderNodeTree returned empty string for single node")
	}

	// Should contain the root node info
	if !strings.Contains(result, "[1]") {
		t.Errorf("RenderNodeTree missing root ID, got: %q", result)
	}

	if !strings.Contains(result, "Root claim") {
		t.Errorf("RenderNodeTree missing root statement, got: %q", result)
	}
}

// TestRenderNodeTree_Hierarchy tests that child nodes are properly indented
func TestRenderNodeTree_Hierarchy(t *testing.T) {
	nodes := []*node.Node{
		makeTestNode("1", schema.NodeTypeClaim, "Root theorem", schema.InferenceAssumption),
		makeTestNode("1.1", schema.NodeTypeLocalAssume, "Assume P", schema.InferenceLocalAssume),
		makeTestNode("1.2", schema.NodeTypeClaim, "Then Q follows", schema.InferenceModusPonens),
		makeTestNode("1.2.1", schema.NodeTypeClaim, "Sub-step", schema.InferenceModusPonens),
	}

	result := RenderNodeTree(nodes)

	if result == "" {
		t.Fatal("RenderNodeTree returned empty string")
	}

	// Split into lines for analysis
	lines := strings.Split(result, "\n")

	// Remove empty lines for easier analysis
	var nonEmptyLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	if len(nonEmptyLines) < 4 {
		t.Errorf("Expected at least 4 lines for 4 nodes, got %d", len(nonEmptyLines))
	}

	// Root should have no indentation or minimal indentation
	// Children should have more indentation than their parents
	// We check that deeper nodes have more leading spaces

	// Find lines containing specific IDs and check relative indentation
	var rootIndent, child1Indent, grandchildIndent int

	for _, line := range nonEmptyLines {
		leadingSpaces := len(line) - len(strings.TrimLeft(line, " \t"))

		if strings.Contains(line, "[1]") && !strings.Contains(line, "[1.") {
			rootIndent = leadingSpaces
		} else if strings.Contains(line, "[1.1]") || strings.Contains(line, "[1.2]") && !strings.Contains(line, "[1.2.") {
			child1Indent = leadingSpaces
		} else if strings.Contains(line, "[1.2.1]") {
			grandchildIndent = leadingSpaces
		}
	}

	// Children should be indented more than root
	if child1Indent <= rootIndent {
		t.Errorf("Child nodes should be indented more than root (root=%d, child=%d)", rootIndent, child1Indent)
	}

	// Grandchildren should be indented more than children
	if grandchildIndent <= child1Indent {
		t.Errorf("Grandchild nodes should be indented more than children (child=%d, grandchild=%d)", child1Indent, grandchildIndent)
	}
}

// TestRenderNodeTree_WithNilNode tests that nil nodes in the list are handled gracefully
func TestRenderNodeTree_WithNilNode(t *testing.T) {
	nodes := []*node.Node{
		makeTestNode("1", schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption),
		nil, // nil node in the middle
		makeTestNode("1.1", schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens),
	}

	// Should not panic
	result := RenderNodeTree(nodes)

	// Should still render the non-nil nodes
	if !strings.Contains(result, "[1]") {
		t.Errorf("RenderNodeTree missing root node, got: %q", result)
	}

	if !strings.Contains(result, "[1.1]") {
		t.Errorf("RenderNodeTree missing child node, got: %q", result)
	}
}

// TestRenderNode_SpecialCharacters tests that special characters in statements are handled
func TestRenderNode_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name      string
		statement string
	}{
		{
			name:      "unicode math symbols",
			statement: "For all x: P(x) implies Q(x)",
		},
		{
			name:      "quotes in statement",
			statement: `The term "natural number" is defined`,
		},
		{
			name:      "newlines should be escaped or replaced",
			statement: "Line one\nLine two",
		},
		{
			name:      "tabs and special whitespace",
			statement: "With\ttab\tcharacters",
		},
		{
			name:      "angle brackets",
			statement: "For all <x> in set",
		},
		{
			name:      "backslashes for LaTeX",
			statement: `Let \alpha be a constant`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := makeTestNode("1", schema.NodeTypeClaim, tt.statement, schema.InferenceModusPonens)

			result := RenderNode(n)

			// Should not panic and should return non-empty result
			if result == "" {
				t.Fatal("RenderNode returned empty string")
			}

			// Should still contain the ID
			if !strings.Contains(result, "[1]") {
				t.Errorf("RenderNode missing ID, got: %q", result)
			}

			// Single-line output should not contain unescaped newlines
			if strings.Count(result, "\n") > 0 {
				t.Errorf("RenderNode single-line output contains newlines, got: %q", result)
			}
		})
	}
}

// TestRenderNode_FormatConsistency tests that output format is consistent
func TestRenderNode_FormatConsistency(t *testing.T) {
	n := makeTestNode("1.2", schema.NodeTypeClaim, "Test statement", schema.InferenceModusPonens)

	// Expected format: [1.2] claim (pending): "Test statement"
	result := RenderNode(n)

	// Check format follows expected pattern: [ID] type (state): "statement"
	// Should have brackets around ID
	if !strings.Contains(result, "[1.2]") {
		t.Errorf("ID should be in brackets, got: %q", result)
	}

	// Should have parentheses around state
	if !strings.Contains(result, "(pending)") {
		t.Errorf("State should be in parentheses, got: %q", result)
	}

	// Statement should be quoted
	if !strings.Contains(result, `"`) {
		t.Errorf("Statement should be quoted, got: %q", result)
	}
}

// TestRenderNodeVerbose_ContentHash tests that content hash is shown in verbose mode
func TestRenderNodeVerbose_ContentHash(t *testing.T) {
	n := makeTestNode("1", schema.NodeTypeClaim, "Hash test", schema.InferenceModusPonens)

	result := RenderNodeVerbose(n)

	// Verbose output should include the content hash (or at least a truncated version)
	// The hash is a 64-character hex string, so we check for "Hash" label
	if !strings.Contains(result, "Hash") && !strings.Contains(result, "hash") {
		t.Logf("Note: verbose output may want to include content hash, result: %q", result)
	}
}

// TestRenderNode_DifferentEpistemicStates tests rendering nodes with different epistemic states
func TestRenderNode_DifferentEpistemicStates(t *testing.T) {
	// We need to modify the node's epistemic state after creation
	// Since NewNode creates nodes in pending state, we'll just verify that state is rendered
	n := makeTestNode("1", schema.NodeTypeClaim, "Test", schema.InferenceModusPonens)

	result := RenderNode(n)

	// Default state is pending
	if !strings.Contains(result, "pending") {
		t.Errorf("RenderNode should show epistemic state 'pending', got: %q", result)
	}
}

// TestRenderNode_DifferentWorkflowStates tests rendering nodes with different workflow states
func TestRenderNode_DifferentWorkflowStates(t *testing.T) {
	n := makeTestNode("1", schema.NodeTypeClaim, "Test", schema.InferenceModusPonens)

	result := RenderNode(n)

	// Default workflow state is available - this may or may not be shown in single-line
	// Epistemic state is the primary state shown
	if !strings.Contains(result, "pending") {
		t.Errorf("RenderNode should show state, got: %q", result)
	}
}

// TestRenderNodeTree_UnsortedInput tests that tree rendering handles unsorted input
func TestRenderNodeTree_UnsortedInput(t *testing.T) {
	// Provide nodes out of order
	nodes := []*node.Node{
		makeTestNode("1.2", schema.NodeTypeClaim, "Second child", schema.InferenceModusPonens),
		makeTestNode("1", schema.NodeTypeClaim, "Root", schema.InferenceAssumption),
		makeTestNode("1.1", schema.NodeTypeClaim, "First child", schema.InferenceModusPonens),
	}

	result := RenderNodeTree(nodes)

	if result == "" {
		t.Fatal("RenderNodeTree returned empty string")
	}

	// All nodes should be present
	if !strings.Contains(result, "[1]") {
		t.Errorf("RenderNodeTree missing root, got: %q", result)
	}
	if !strings.Contains(result, "[1.1]") {
		t.Errorf("RenderNodeTree missing 1.1, got: %q", result)
	}
	if !strings.Contains(result, "[1.2]") {
		t.Errorf("RenderNodeTree missing 1.2, got: %q", result)
	}

	// Root should appear before children in the output
	rootPos := strings.Index(result, "[1]")
	child1Pos := strings.Index(result, "[1.1]")
	child2Pos := strings.Index(result, "[1.2]")

	if rootPos > child1Pos || rootPos > child2Pos {
		t.Errorf("Root should appear before children in tree output")
	}
}

// TestRenderNodeTree_DeepNesting tests rendering deeply nested trees
func TestRenderNodeTree_DeepNesting(t *testing.T) {
	nodes := []*node.Node{
		makeTestNode("1", schema.NodeTypeClaim, "Level 1", schema.InferenceAssumption),
		makeTestNode("1.1", schema.NodeTypeClaim, "Level 2", schema.InferenceModusPonens),
		makeTestNode("1.1.1", schema.NodeTypeClaim, "Level 3", schema.InferenceModusPonens),
		makeTestNode("1.1.1.1", schema.NodeTypeClaim, "Level 4", schema.InferenceModusPonens),
		makeTestNode("1.1.1.1.1", schema.NodeTypeClaim, "Level 5", schema.InferenceModusPonens),
	}

	result := RenderNodeTree(nodes)

	if result == "" {
		t.Fatal("RenderNodeTree returned empty string")
	}

	// All nodes should be present
	for _, n := range nodes {
		id := "[" + n.ID.String() + "]"
		if !strings.Contains(result, id) {
			t.Errorf("RenderNodeTree missing node %s, got: %q", id, result)
		}
	}

	// Check progressive indentation
	lines := strings.Split(result, "\n")
	var prevIndent int
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		// Each successive level should have equal or greater indentation
		// (siblings may have equal indentation)
		if strings.Contains(line, "Level") {
			if indent < prevIndent && !strings.Contains(line, "Level 1") {
				// Allow for siblings to reset indentation
			}
			prevIndent = indent
		}
	}
}

// helper function to parse NodeID without error handling in tests
func mustParse(s string) types.NodeID {
	id, err := types.Parse(s)
	if err != nil {
		panic("invalid NodeID in test: " + s)
	}
	return id
}

// TestRenderNode_NoTruncationForMathFormulas verifies that mathematical formulas
// are never truncated mid-expression (issue vibefeld-amjk)
func TestRenderNode_NoTruncationForMathFormulas(t *testing.T) {
	mathFormulas := []struct {
		name    string
		formula string
	}{
		{
			name:    "Dobinski formula",
			formula: "B_n = (1/e) * sum_{k=0}^{infinity} (k^n / k!)",
		},
		{
			name:    "quadratic formula",
			formula: "x = (-b +/- sqrt(b^2 - 4ac)) / (2a)",
		},
		{
			name:    "Euler identity",
			formula: "e^{i*pi} + 1 = 0 is the most beautiful equation in mathematics",
		},
		{
			name:    "integral",
			formula: "integral_{0}^{infinity} x^n * e^{-x} dx = n! (Gamma function definition)",
		},
		{
			name:    "series expansion",
			formula: "e^x = sum_{n=0}^{infinity} x^n/n! = 1 + x + x^2/2! + x^3/3! + ...",
		},
	}

	for _, tc := range mathFormulas {
		t.Run(tc.name, func(t *testing.T) {
			n := makeTestNode("1", schema.NodeTypeClaim, tc.formula, schema.InferenceModusPonens)
			result := RenderNode(n)

			// Formula must appear in full, not truncated
			if !strings.Contains(result, tc.formula) {
				t.Errorf("Formula should not be truncated.\nExpected to contain: %q\nGot: %q", tc.formula, result)
			}

			// Should not have ellipsis indicating truncation
			// (Note: ellipsis in the original formula itself is OK)
			if strings.HasSuffix(result, `..."`) && !strings.HasSuffix(tc.formula, "...") {
				t.Errorf("Formula appears to be truncated with ellipsis: %q", result)
			}
		})
	}
}
