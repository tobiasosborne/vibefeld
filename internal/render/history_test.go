package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/types"
)

func TestFormatHistory_EmptyHistory(t *testing.T) {
	nodeID, _ := types.Parse("1")
	history := &NodeHistory{
		NodeID:  nodeID,
		Entries: []HistoryEntry{},
	}

	result := FormatHistory(history)
	if !strings.Contains(result, "No history found") {
		t.Errorf("Expected 'No history found' message, got: %s", result)
	}
}

func TestFormatHistory_WithEntries(t *testing.T) {
	nodeID, _ := types.Parse("1.2")
	ts, _ := types.ParseTimestamp("2025-01-11T10:05:00Z")

	history := &NodeHistory{
		NodeID: nodeID,
		Entries: []HistoryEntry{
			{
				Seq:       1,
				Type:      "node_created",
				Timestamp: ts,
				Details: map[string]interface{}{
					"node": map[string]interface{}{
						"type":      "claim",
						"statement": "Test statement",
					},
				},
			},
			{
				Seq:       2,
				Type:      "nodes_claimed",
				Timestamp: ts,
				Actor:     "agent-1",
				Details: map[string]interface{}{
					"timeout": "2025-01-11T11:05:00Z",
				},
			},
		},
	}

	result := FormatHistory(history)

	// Check for expected content
	if !strings.Contains(result, "1.2") {
		t.Errorf("Expected node ID 1.2 in output, got: %s", result)
	}
	if !strings.Contains(result, "2 events") {
		t.Errorf("Expected '2 events' in output, got: %s", result)
	}
	if !strings.Contains(result, "Node Created") {
		t.Errorf("Expected 'Node Created' in output, got: %s", result)
	}
	if !strings.Contains(result, "Nodes Claimed") {
		t.Errorf("Expected 'Nodes Claimed' in output, got: %s", result)
	}
	if !strings.Contains(result, "agent-1") {
		t.Errorf("Expected actor 'agent-1' in output, got: %s", result)
	}
}

func TestFormatHistoryJSON_NilHistory(t *testing.T) {
	result := FormatHistoryJSON(nil)
	expected := `{"node_id":"","entries":[]}`
	if result != expected {
		t.Errorf("Expected %s, got: %s", expected, result)
	}
}

func TestFormatHistoryJSON_WithEntries(t *testing.T) {
	nodeID, _ := types.Parse("1")
	ts, _ := types.ParseTimestamp("2025-01-11T10:05:00Z")

	history := &NodeHistory{
		NodeID: nodeID,
		Entries: []HistoryEntry{
			{
				Seq:       1,
				Type:      "node_created",
				Timestamp: ts,
			},
		},
	}

	result := FormatHistoryJSON(history)

	// Parse JSON to verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if parsed["node_id"] != "1" {
		t.Errorf("Expected node_id '1', got: %v", parsed["node_id"])
	}

	entries, ok := parsed["entries"].([]interface{})
	if !ok || len(entries) != 1 {
		t.Errorf("Expected 1 entry, got: %v", parsed["entries"])
	}
}

func TestFormatHistoryEventType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"node_created", "Node Created"},
		{"nodes_claimed", "Nodes Claimed"},
		{"challenge_raised", "Challenge Raised"},
		{"", "Unknown"},
	}

	for _, tt := range tests {
		result := formatHistoryEventType(tt.input)
		if result != tt.expected {
			t.Errorf("formatHistoryEventType(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatHistoryDetails(t *testing.T) {
	tests := []struct {
		eventType string
		data      map[string]interface{}
		contains  string
	}{
		{
			eventType: "nodes_claimed",
			data:      map[string]interface{}{"timeout": "2025-01-11T11:00:00Z"},
			contains:  "Timeout:",
		},
		{
			eventType: "challenge_raised",
			data: map[string]interface{}{
				"target": "statement",
				"reason": "unclear",
			},
			contains: "Target: statement",
		},
		{
			eventType: "taint_recomputed",
			data:      map[string]interface{}{"new_taint": "clean"},
			contains:  "New taint: clean",
		},
	}

	for _, tt := range tests {
		result := formatHistoryDetails(tt.eventType, tt.data)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("formatHistoryDetails(%q, ...) = %q, want to contain %q", tt.eventType, result, tt.contains)
		}
	}
}
