package render

import (
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

func TestFormatSearchResults_Empty(t *testing.T) {
	result := FormatSearchResults(nil)
	if !strings.Contains(result, "No matching nodes found") {
		t.Errorf("Expected 'No matching nodes found' message, got: %s", result)
	}

	result = FormatSearchResults([]SearchResult{})
	if !strings.Contains(result, "No matching nodes found") {
		t.Errorf("Expected 'No matching nodes found' message for empty slice, got: %s", result)
	}
}

func TestFormatSearchResults_SingleResult(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)

	results := []SearchResult{
		{Node: n, MatchReason: "text match"},
	}

	output := FormatSearchResults(results)

	// Verify key elements are present
	if !strings.Contains(output, "[1]") {
		t.Error("Expected node ID [1] in output")
	}
	if !strings.Contains(output, "Test statement") {
		t.Error("Expected statement in output")
	}
	if !strings.Contains(output, "text match") {
		t.Error("Expected match reason in output")
	}
	if !strings.Contains(output, "1 node") {
		t.Error("Expected '1 node' count in output")
	}
}

func TestFormatSearchResults_MultipleResults(t *testing.T) {
	node1ID, _ := types.Parse("1")
	node2ID, _ := types.Parse("1.1")
	node3ID, _ := types.Parse("1.2")

	n1, _ := node.NewNode(node1ID, schema.NodeTypeClaim, "First claim", schema.InferenceAssumption)
	n2, _ := node.NewNode(node2ID, schema.NodeTypeClaim, "Second claim", schema.InferenceAssumption)
	n3, _ := node.NewNode(node3ID, schema.NodeTypeClaim, "Third claim", schema.InferenceAssumption)

	results := []SearchResult{
		{Node: n3, MatchReason: "state: pending"},
		{Node: n1, MatchReason: "state: pending"},
		{Node: n2, MatchReason: "state: pending"},
	}

	output := FormatSearchResults(results)

	// Verify all nodes are present
	if !strings.Contains(output, "[1]") {
		t.Error("Expected node ID [1] in output")
	}
	if !strings.Contains(output, "[1.1]") {
		t.Error("Expected node ID [1.1] in output")
	}
	if !strings.Contains(output, "[1.2]") {
		t.Error("Expected node ID [1.2] in output")
	}

	// Verify order: nodes should be sorted by ID
	idx1 := strings.Index(output, "[1]")
	idx11 := strings.Index(output, "[1.1]")
	idx12 := strings.Index(output, "[1.2]")

	if idx1 > idx11 || idx11 > idx12 {
		t.Error("Expected nodes to be sorted by ID")
	}
}

func TestFormatSearchResults_LongStatement(t *testing.T) {
	nodeID, _ := types.Parse("1")
	longStatement := strings.Repeat("x", 100)
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, longStatement, schema.InferenceAssumption)

	results := []SearchResult{
		{Node: n, MatchReason: "text match"},
	}

	output := FormatSearchResults(results)

	// Verify truncation with ellipsis
	if !strings.Contains(output, "...") {
		t.Error("Expected truncated statement with ellipsis")
	}
}

func TestFormatSearchResultsJSON_Empty(t *testing.T) {
	result := FormatSearchResultsJSON(nil)
	if result != `{"results":[],"total":0}` {
		t.Errorf("Expected empty JSON result, got: %s", result)
	}

	result = FormatSearchResultsJSON([]SearchResult{})
	if result != `{"results":[],"total":0}` {
		t.Errorf("Expected empty JSON result for empty slice, got: %s", result)
	}
}

func TestFormatSearchResultsJSON_SingleResult(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)

	results := []SearchResult{
		{Node: n, MatchReason: "text match"},
	}

	output := FormatSearchResultsJSON(results)

	// Verify key elements are present
	if !strings.Contains(output, `"id":"1"`) {
		t.Error("Expected node ID in JSON output")
	}
	if !strings.Contains(output, `"statement":"Test statement"`) {
		t.Error("Expected statement in JSON output")
	}
	if !strings.Contains(output, `"match_reason":"text match"`) {
		t.Error("Expected match_reason in JSON output")
	}
	if !strings.Contains(output, `"total":1`) {
		t.Error("Expected total:1 in JSON output")
	}
}

func TestFormatSearchResultsJSON_MultipleResults(t *testing.T) {
	node1ID, _ := types.Parse("1")
	node2ID, _ := types.Parse("1.1")

	n1, _ := node.NewNode(node1ID, schema.NodeTypeClaim, "First", schema.InferenceAssumption)
	n2, _ := node.NewNode(node2ID, schema.NodeTypeClaim, "Second", schema.InferenceAssumption)

	results := []SearchResult{
		{Node: n2, MatchReason: ""},
		{Node: n1, MatchReason: ""},
	}

	output := FormatSearchResultsJSON(results)

	// Verify total count
	if !strings.Contains(output, `"total":2`) {
		t.Error("Expected total:2 in JSON output")
	}

	// Verify order: 1 should come before 1.1 in sorted output
	idx1 := strings.Index(output, `"id":"1"`)
	idx11 := strings.Index(output, `"id":"1.1"`)

	if idx1 > idx11 {
		t.Error("Expected nodes to be sorted by ID in JSON output")
	}
}

func TestFormatSearchResultsJSON_EscapesSpecialCharacters(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Statement with \"quotes\" and\nnewline", schema.InferenceAssumption)

	results := []SearchResult{
		{Node: n, MatchReason: "test"},
	}

	output := FormatSearchResultsJSON(results)

	// Verify escaping
	if strings.Contains(output, "\n") && !strings.Contains(output, `\n`) {
		t.Error("Expected newlines to be escaped in JSON output")
	}
	if !strings.Contains(output, `\"quotes\"`) {
		t.Error("Expected quotes to be escaped in JSON output")
	}
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{`with "quotes"`, `with \"quotes\"`},
		{"with\nnewline", `with\nnewline`},
		{"with\ttab", `with\ttab`},
		{`with \\ backslash`, `with \\\\ backslash`},
	}

	for _, tt := range tests {
		result := escapeJSON(tt.input)
		if result != tt.expected {
			t.Errorf("escapeJSON(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{42, "42"},
		{100, "100"},
		{999, "999"},
	}

	for _, tt := range tests {
		result := formatInt(tt.input)
		if result != tt.expected {
			t.Errorf("formatInt(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
