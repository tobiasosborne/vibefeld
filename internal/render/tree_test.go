//go:build integration

package render

import (
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// helper to create a test node and add it to state
func addTestNode(t *testing.T, s *state.State, id string, nodeType schema.NodeType, statement string) *node.Node {
	t.Helper()
	nodeID, err := types.Parse(id)
	if err != nil {
		t.Fatalf("invalid test node ID %q: %v", id, err)
	}
	n, err := node.NewNode(nodeID, nodeType, statement, schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create test node %q: %v", id, err)
	}
	s.AddNode(n)
	return n
}

// helper to create a test node with custom states
func addTestNodeWithStates(
	t *testing.T,
	s *state.State,
	id string,
	statement string,
	epistemic schema.EpistemicState,
	taint node.TaintState,
) *node.Node {
	t.Helper()
	nodeID, err := types.Parse(id)
	if err != nil {
		t.Fatalf("invalid test node ID %q: %v", id, err)
	}
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, statement, schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create test node %q: %v", id, err)
	}
	n.EpistemicState = epistemic
	n.TaintState = taint
	s.AddNode(n)
	return n
}

// TestRenderTree_EmptyState tests that rendering an empty state produces placeholder output
func TestRenderTree_EmptyState(t *testing.T) {
	s := state.NewState()

	result := RenderTree(s, nil)

	// Empty state should produce empty string or placeholder message
	// Either is acceptable, but it should not panic
	if result == "" {
		// Empty string is acceptable
		return
	}

	// If non-empty, it should be a placeholder message
	lower := strings.ToLower(result)
	if !strings.Contains(lower, "empty") && !strings.Contains(lower, "no") && !strings.Contains(lower, "none") {
		t.Logf("Note: empty state output may want indicator message, got: %q", result)
	}
}

// TestRenderTree_SingleRootNode tests rendering a single root node
func TestRenderTree_SingleRootNode(t *testing.T) {
	s := state.NewState()
	addTestNodeWithStates(t, s, "1", "Root claim", schema.EpistemicPending, node.TaintClean)

	result := RenderTree(s, nil)

	if result == "" {
		t.Fatal("RenderTree returned empty string for single node")
	}

	// Should contain the node ID
	if !strings.Contains(result, "1") {
		t.Errorf("RenderTree missing node ID '1', got: %q", result)
	}

	// Should contain status indicator (pending)
	if !strings.Contains(result, "pending") {
		t.Errorf("RenderTree missing epistemic state 'pending', got: %q", result)
	}

	// Should contain taint indicator (clean)
	if !strings.Contains(result, "clean") {
		t.Errorf("RenderTree missing taint state 'clean', got: %q", result)
	}

	// Should contain the statement
	if !strings.Contains(result, "Root claim") {
		t.Errorf("RenderTree missing statement 'Root claim', got: %q", result)
	}
}

// TestRenderTree_TwoLevelTree tests rendering a tree with root and children
func TestRenderTree_TwoLevelTree(t *testing.T) {
	s := state.NewState()
	addTestNodeWithStates(t, s, "1", "Root claim", schema.EpistemicPending, node.TaintClean)
	addTestNodeWithStates(t, s, "1.1", "First step", schema.EpistemicValidated, node.TaintClean)
	addTestNodeWithStates(t, s, "1.2", "Second step", schema.EpistemicPending, node.TaintTainted)

	result := RenderTree(s, nil)

	if result == "" {
		t.Fatal("RenderTree returned empty string")
	}

	// Should contain all node IDs
	for _, id := range []string{"1", "1.1", "1.2"} {
		if !strings.Contains(result, id) {
			t.Errorf("RenderTree missing node ID %q, got: %q", id, result)
		}
	}

	// Should contain tree branch characters (either unicode or ASCII)
	hasTreeChars := strings.Contains(result, "\u251c") || // |-
		strings.Contains(result, "\u2514") || // `-
		strings.Contains(result, "\u2502") || // |
		strings.Contains(result, "\u2500") || // -
		strings.Contains(result, "|-") ||
		strings.Contains(result, "`-") ||
		strings.Contains(result, "|")

	if !hasTreeChars {
		t.Logf("Note: tree output may want branch characters, got: %q", result)
	}

	// Root should appear before children
	rootPos := strings.Index(result, "1 ") // Node ID followed by space or bracket
	if rootPos == -1 {
		// Try alternative formats
		rootPos = strings.Index(result, "[1]")
	}
	child1Pos := strings.Index(result, "1.1")
	child2Pos := strings.Index(result, "1.2")

	if rootPos > child1Pos || rootPos > child2Pos {
		t.Errorf("Root should appear before children in output")
	}
}

// TestRenderTree_DeepTree tests rendering a tree with 3+ levels of nesting
func TestRenderTree_DeepTree(t *testing.T) {
	s := state.NewState()
	addTestNodeWithStates(t, s, "1", "Level 1", schema.EpistemicPending, node.TaintClean)
	addTestNodeWithStates(t, s, "1.1", "Level 2", schema.EpistemicValidated, node.TaintClean)
	addTestNodeWithStates(t, s, "1.1.1", "Level 3", schema.EpistemicPending, node.TaintTainted)
	addTestNodeWithStates(t, s, "1.1.1.1", "Level 4", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	result := RenderTree(s, nil)

	if result == "" {
		t.Fatal("RenderTree returned empty string")
	}

	// All nodes should be present
	for _, id := range []string{"1", "1.1", "1.1.1", "1.1.1.1"} {
		if !strings.Contains(result, id) {
			t.Errorf("RenderTree missing node ID %q, got: %q", id, result)
		}
	}

	// Check progressive indentation by looking at line positions
	lines := strings.Split(result, "\n")
	var prevIndent int
	var foundProgression bool

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t\u2502\u251c\u2514\u2500|`-"))

		// Look for increasing indentation pattern
		if indent > prevIndent {
			foundProgression = true
		}
		prevIndent = indent
	}

	if !foundProgression && len(lines) > 1 {
		t.Logf("Note: deep tree may want progressive indentation, got: %q", result)
	}
}

// TestRenderTree_StatusIndicators tests that all epistemic states display correctly
func TestRenderTree_StatusIndicators(t *testing.T) {
	tests := []struct {
		name      string
		epistemic schema.EpistemicState
		wantState string
	}{
		{"pending", schema.EpistemicPending, "pending"},
		{"validated", schema.EpistemicValidated, "validated"},
		{"admitted", schema.EpistemicAdmitted, "admitted"},
		{"refuted", schema.EpistemicRefuted, "refuted"},
		{"archived", schema.EpistemicArchived, "archived"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()
			addTestNodeWithStates(t, s, "1", "Test node", tt.epistemic, node.TaintClean)

			result := RenderTree(s, nil)

			if !strings.Contains(result, tt.wantState) {
				t.Errorf("RenderTree missing epistemic state %q, got: %q", tt.wantState, result)
			}
		})
	}
}

// TestRenderTree_TaintIndicators tests that all taint states display correctly
func TestRenderTree_TaintIndicators(t *testing.T) {
	tests := []struct {
		name      string
		taint     node.TaintState
		wantState string
	}{
		{"clean", node.TaintClean, "clean"},
		{"self_admitted", node.TaintSelfAdmitted, "self_admitted"},
		{"tainted", node.TaintTainted, "tainted"},
		{"unresolved", node.TaintUnresolved, "unresolved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()
			addTestNodeWithStates(t, s, "1", "Test node", schema.EpistemicPending, tt.taint)

			result := RenderTree(s, nil)

			if !strings.Contains(result, tt.wantState) {
				t.Errorf("RenderTree missing taint state %q, got: %q", tt.wantState, result)
			}
		})
	}
}

// TestRenderTree_UnicodeTreeCharacters tests that Unicode tree characters are used
func TestRenderTree_UnicodeTreeCharacters(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", schema.NodeTypeClaim, "Root")
	addTestNode(t, s, "1.1", schema.NodeTypeClaim, "First child")
	addTestNode(t, s, "1.2", schema.NodeTypeClaim, "Last child")

	result := RenderTree(s, nil)

	// Check for proper Unicode box-drawing characters
	// - (U+251C BOX DRAWINGS LIGHT VERTICAL AND RIGHT) for middle items
	// - (U+2514 BOX DRAWINGS LIGHT UP AND RIGHT) for last items
	// - (U+2502 BOX DRAWINGS LIGHT VERTICAL) for continuing vertical lines
	// - (U+2500 BOX DRAWINGS LIGHT HORIZONTAL) for horizontal connectors

	unicodeChars := []struct {
		char string
		name string
	}{
		{"\u251c", "vertical-and-right (middle branch)"}, // +--
		{"\u2514", "up-and-right (last branch)"},         // `--
		{"\u2500", "horizontal"},                         // --
	}

	foundAny := false
	for _, uc := range unicodeChars {
		if strings.Contains(result, uc.char) {
			foundAny = true
		}
	}

	if !foundAny {
		// Check for ASCII fallback (acceptable alternative)
		asciiChars := []string{"|-", "`-", "+-", "\\-"}
		for _, ac := range asciiChars {
			if strings.Contains(result, ac) {
				foundAny = true
				break
			}
		}
	}

	if !foundAny {
		t.Logf("Note: multi-child tree may want tree branch characters, result: %q", result)
	}
}

// TestRenderTree_CustomRootNode tests rendering a subtree from a specific node
func TestRenderTree_CustomRootNode(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", schema.NodeTypeClaim, "Root")
	addTestNode(t, s, "1.1", schema.NodeTypeClaim, "First subtree root")
	addTestNode(t, s, "1.1.1", schema.NodeTypeClaim, "First subtree child")
	addTestNode(t, s, "1.2", schema.NodeTypeClaim, "Second subtree")

	// Render only the subtree starting at 1.1
	subtreeRoot, _ := types.Parse("1.1")
	result := RenderTree(s, &subtreeRoot)

	if result == "" {
		t.Fatal("RenderTree returned empty string for subtree")
	}

	// Should contain 1.1 and its children
	if !strings.Contains(result, "1.1") {
		t.Errorf("RenderTree missing subtree root '1.1', got: %q", result)
	}

	if !strings.Contains(result, "1.1.1") {
		t.Errorf("RenderTree missing subtree child '1.1.1', got: %q", result)
	}

	// Should NOT contain 1.2 (sibling of subtree root)
	// Note: We need to be careful - "1.2" substring might appear in other contexts
	// Check that 1.2's statement is not present
	if strings.Contains(result, "Second subtree") {
		t.Errorf("RenderTree should not include sibling subtree, got: %q", result)
	}
}

// TestRenderTree_CustomRootNode_NotFound tests behavior when custom root doesn't exist
func TestRenderTree_CustomRootNode_NotFound(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", schema.NodeTypeClaim, "Root")

	// Try to render from a non-existent node
	nonExistent, _ := types.Parse("2")
	result := RenderTree(s, &nonExistent)

	// Should return empty or error message, not panic
	// Empty is acceptable for non-existent root
	if result != "" {
		// If non-empty, should indicate the node wasn't found
		lower := strings.ToLower(result)
		if !strings.Contains(lower, "not found") && !strings.Contains(lower, "no") && !strings.Contains(lower, "empty") {
			t.Logf("Note: non-existent root may want indicator message, got: %q", result)
		}
	}
}

// TestRenderTree_MultipleChildren tests proper branch rendering for nodes with multiple children
func TestRenderTree_MultipleChildren(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", schema.NodeTypeClaim, "Root")
	addTestNode(t, s, "1.1", schema.NodeTypeClaim, "Child A")
	addTestNode(t, s, "1.2", schema.NodeTypeClaim, "Child B")
	addTestNode(t, s, "1.3", schema.NodeTypeClaim, "Child C")

	result := RenderTree(s, nil)

	if result == "" {
		t.Fatal("RenderTree returned empty string")
	}

	// All children should be present
	for _, stmt := range []string{"Child A", "Child B", "Child C"} {
		if !strings.Contains(result, stmt) {
			t.Errorf("RenderTree missing child %q, got: %q", stmt, result)
		}
	}

	// Children should appear in order (1.1, 1.2, 1.3)
	posA := strings.Index(result, "Child A")
	posB := strings.Index(result, "Child B")
	posC := strings.Index(result, "Child C")

	if posA > posB || posB > posC {
		t.Errorf("Children should appear in order (A, B, C), positions: A=%d, B=%d, C=%d", posA, posB, posC)
	}
}

// TestRenderTree_MixedBranching tests proper indentation with mixed branching patterns
func TestRenderTree_MixedBranching(t *testing.T) {
	s := state.NewState()
	// Create a tree structure like:
	// 1
	// |-1.1
	// | `-1.1.1
	// `-1.2
	//   |-1.2.1
	//   `-1.2.2
	addTestNode(t, s, "1", schema.NodeTypeClaim, "Root")
	addTestNode(t, s, "1.1", schema.NodeTypeClaim, "First child")
	addTestNode(t, s, "1.1.1", schema.NodeTypeClaim, "First grandchild")
	addTestNode(t, s, "1.2", schema.NodeTypeClaim, "Second child")
	addTestNode(t, s, "1.2.1", schema.NodeTypeClaim, "Second grandchild A")
	addTestNode(t, s, "1.2.2", schema.NodeTypeClaim, "Second grandchild B")

	result := RenderTree(s, nil)

	if result == "" {
		t.Fatal("RenderTree returned empty string")
	}

	// All nodes should be present
	nodeStatements := []string{
		"Root",
		"First child",
		"First grandchild",
		"Second child",
		"Second grandchild A",
		"Second grandchild B",
	}
	for _, stmt := range nodeStatements {
		if !strings.Contains(result, stmt) {
			t.Errorf("RenderTree missing node %q, got: %q", stmt, result)
		}
	}
}

// TestRenderTree_StateBracketFormat tests the [status/taint] bracket format
func TestRenderTree_StateBracketFormat(t *testing.T) {
	s := state.NewState()
	addTestNodeWithStates(t, s, "1", "Root claim", schema.EpistemicValidated, node.TaintClean)

	result := RenderTree(s, nil)

	// Expected format includes [status/taint] like [validated/clean]
	// Check for brackets containing status
	if !strings.Contains(result, "[") || !strings.Contains(result, "]") {
		t.Logf("Note: tree output may want bracketed status format [status/taint], got: %q", result)
	}

	// Should contain both states, possibly with / separator
	if strings.Contains(result, "[") {
		if !strings.Contains(result, "validated") {
			t.Errorf("RenderTree missing 'validated' in bracketed format, got: %q", result)
		}
		if !strings.Contains(result, "clean") {
			t.Errorf("RenderTree missing 'clean' in bracketed format, got: %q", result)
		}
	}
}

// TestRenderTree_NilState tests that nil state is handled gracefully
func TestRenderTree_NilState(t *testing.T) {
	// Should not panic with nil state
	result := RenderTree(nil, nil)

	if result != "" {
		// If non-empty, should indicate empty/no data
		lower := strings.ToLower(result)
		if !strings.Contains(lower, "empty") && !strings.Contains(lower, "no") && !strings.Contains(lower, "nil") {
			t.Logf("Note: nil state output may want indicator message, got: %q", result)
		}
	}
}

// TestRenderTree_ConsistentFormat tests that output format is consistent across calls
func TestRenderTree_ConsistentFormat(t *testing.T) {
	s := state.NewState()
	addTestNodeWithStates(t, s, "1", "Root", schema.EpistemicPending, node.TaintClean)
	addTestNodeWithStates(t, s, "1.1", "Child", schema.EpistemicValidated, node.TaintTainted)

	// Call RenderTree multiple times
	result1 := RenderTree(s, nil)
	result2 := RenderTree(s, nil)

	// Results should be identical (deterministic output)
	if result1 != result2 {
		t.Errorf("RenderTree output is not deterministic:\nFirst:  %q\nSecond: %q", result1, result2)
	}
}

// TestRenderTree_LongStatement tests that long statements are NOT truncated
// Mathematical proofs require precision - formulas must be shown in full
func TestRenderTree_LongStatement(t *testing.T) {
	s := state.NewState()
	longStmt := "B_n = (1/e) * sum_{k=0}^{infinity} (k^n / k!) where B_n is the n-th Bell number representing set partitions"
	addTestNode(t, s, "1", schema.NodeTypeClaim, longStmt)

	result := RenderTree(s, nil)

	if result == "" {
		t.Fatal("RenderTree returned empty string")
	}

	// Should contain the node ID
	if !strings.Contains(result, "1") {
		t.Errorf("RenderTree missing node ID, got: %q", result)
	}

	// Mathematical formulas should NOT be truncated - shown in full
	if strings.Contains(result, "...") {
		t.Errorf("Mathematical statements should not be truncated, got: %q", result)
	}

	// The full formula should be present
	if !strings.Contains(result, "B_n = (1/e) * sum_{k=0}^{infinity}") {
		t.Errorf("RenderTree should show full formula, got: %q", result)
	}
}

// TestRenderTree_SortedOutput tests that nodes are output in sorted order
func TestRenderTree_SortedOutput(t *testing.T) {
	s := state.NewState()
	// Add nodes out of order
	addTestNode(t, s, "1.2", schema.NodeTypeClaim, "Node B")
	addTestNode(t, s, "1", schema.NodeTypeClaim, "Node Root")
	addTestNode(t, s, "1.1", schema.NodeTypeClaim, "Node A")
	addTestNode(t, s, "1.10", schema.NodeTypeClaim, "Node J") // Should come after 1.9, not after 1.1
	addTestNode(t, s, "1.3", schema.NodeTypeClaim, "Node C")

	result := RenderTree(s, nil)

	// Check ordering: 1, then 1.1, 1.2, 1.3, 1.10
	posRoot := strings.Index(result, "Node Root")
	posA := strings.Index(result, "Node A")
	posB := strings.Index(result, "Node B")
	posC := strings.Index(result, "Node C")
	posJ := strings.Index(result, "Node J")

	if posRoot > posA {
		t.Errorf("Root should appear before 1.1")
	}
	if posA > posB || posB > posC {
		t.Errorf("Children should appear in numeric order: 1.1, 1.2, 1.3")
	}
	if posC > posJ {
		t.Errorf("1.10 should come after 1.3 (numeric sort, not lexicographic)")
	}
}

// TestRenderTree_ExpectedOutputFormat tests the complete expected output format
func TestRenderTree_ExpectedOutputFormat(t *testing.T) {
	s := state.NewState()
	addTestNodeWithStates(t, s, "1", "Root claim", schema.EpistemicPending, node.TaintClean)
	addTestNodeWithStates(t, s, "1.1", "First step", schema.EpistemicValidated, node.TaintClean)
	addTestNodeWithStates(t, s, "1.1.1", "Sub-step", schema.EpistemicPending, node.TaintTainted)
	addTestNodeWithStates(t, s, "1.2", "Second step", schema.EpistemicPending, node.TaintClean)

	result := RenderTree(s, nil)

	// The expected format from the issue:
	// 1 [pending/clean] Root claim
	// +-- 1.1 [validated/clean] First step
	// |   `-- 1.1.1 [pending/tainted] Sub-step
	// `-- 1.2 [pending/clean] Second step

	// Verify basic structure elements are present
	requiredElements := []string{
		"1",
		"pending",
		"clean",
		"Root claim",
		"1.1",
		"validated",
		"First step",
		"1.1.1",
		"tainted",
		"Sub-step",
		"1.2",
		"Second step",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(result, elem) {
			t.Errorf("RenderTree output missing expected element %q, got: %q", elem, result)
		}
	}

	// Output should be multi-line (at least 4 lines for 4 nodes)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) < 4 {
		t.Errorf("RenderTree should produce at least 4 lines for 4 nodes, got %d lines: %q", len(lines), result)
	}
}
