// Package render provides history rendering functions for AF framework.
package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/tobias/vibefeld/internal/types"
)

// HistoryEntry represents a single event in a node's history.
type HistoryEntry struct {
	Seq       int                    `json:"seq"`
	Type      string                 `json:"type"`
	Timestamp types.Timestamp        `json:"timestamp"`
	Actor     string                 `json:"actor,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// NodeHistory represents the complete history of a node.
type NodeHistory struct {
	NodeID  types.NodeID   `json:"node_id"`
	Entries []HistoryEntry `json:"entries"`
}

// FormatHistory renders a node's history as human-readable text.
// Returns a formatted string showing all events affecting the node in chronological order.
func FormatHistory(history *NodeHistory) string {
	if history == nil || len(history.Entries) == 0 {
		return fmt.Sprintf("No history found for node %s\n", history.NodeID.String())
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("History for node %s (%d events):\n", history.NodeID.String(), len(history.Entries)))
	sb.WriteString(strings.Repeat("-", 80))
	sb.WriteString("\n")

	for _, entry := range history.Entries {
		// Format timestamp
		displayTime := formatHistoryTimestamp(entry.Timestamp)

		// Format event type (convert snake_case to Title Case)
		displayType := formatHistoryEventType(entry.Type)

		// Build details string
		details := formatHistoryDetails(entry.Type, entry.Details)

		// Actor info
		actorStr := ""
		if entry.Actor != "" {
			actorStr = fmt.Sprintf(" by %s", entry.Actor)
		}

		// Output format: #seq  Timestamp  EventType  [Actor]  Details
		sb.WriteString(fmt.Sprintf("#%-4d  %s  %-20s%s\n", entry.Seq, displayTime, displayType, actorStr))
		if details != "" {
			sb.WriteString(fmt.Sprintf("       %s\n", details))
		}
	}

	return sb.String()
}

// FormatHistoryJSON renders a node's history as JSON.
// Returns a JSON string representation of the history.
func FormatHistoryJSON(history *NodeHistory) string {
	if history == nil {
		return `{"node_id":"","entries":[]}`
	}

	data, err := marshalJSON(history)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal history: %s"}`, err.Error())
	}

	return string(data)
}

// formatHistoryTimestamp formats a timestamp for history display.
func formatHistoryTimestamp(ts types.Timestamp) string {
	tsStr := ts.String()
	if tsStr == "" {
		return "                   "
	}

	// Parse the timestamp
	t, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		// Try parsing with nanoseconds
		t, err = time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			if len(tsStr) > 19 {
				return tsStr[:19]
			}
			return tsStr
		}
	}

	return t.Format("2006-01-02 15:04:05")
}

// formatHistoryEventType converts snake_case event type to Title Case for display.
func formatHistoryEventType(eventType string) string {
	if eventType == "" {
		return "Unknown"
	}

	// Split by underscore and capitalize each part
	parts := strings.Split(eventType, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}

	return strings.Join(parts, " ")
}

// formatHistoryDetails creates a human-readable details string for an event.
func formatHistoryDetails(eventType string, data map[string]interface{}) string {
	if data == nil {
		return ""
	}

	switch eventType {
	case "node_created":
		if node, ok := data["node"].(map[string]interface{}); ok {
			nodeType := ""
			stmt := ""
			if t, ok := node["type"].(string); ok {
				nodeType = t
			}
			if s, ok := node["statement"].(string); ok {
				stmt = s
				if len(stmt) > 60 {
					stmt = stmt[:57] + "..."
				}
			}
			return fmt.Sprintf("Type: %s, Statement: %q", nodeType, stmt)
		}

	case "nodes_claimed":
		if timeout, ok := data["timeout"].(string); ok {
			return fmt.Sprintf("Timeout: %s", timeout)
		}

	case "node_validated", "node_admitted", "node_refuted", "node_archived":
		// These events don't need additional details beyond the event type
		return ""

	case "challenge_raised":
		var parts []string
		if target, ok := data["target"].(string); ok {
			parts = append(parts, fmt.Sprintf("Target: %s", target))
		}
		if reason, ok := data["reason"].(string); ok {
			if len(reason) > 50 {
				reason = reason[:47] + "..."
			}
			parts = append(parts, fmt.Sprintf("Reason: %q", reason))
		}
		return strings.Join(parts, ", ")

	case "challenge_resolved", "challenge_withdrawn":
		if challengeID, ok := data["challenge_id"].(string); ok {
			return fmt.Sprintf("Challenge: %s", challengeID)
		}

	case "taint_recomputed":
		if newTaint, ok := data["new_taint"].(string); ok {
			return fmt.Sprintf("New taint: %s", newTaint)
		}

	case "lemma_extracted":
		if lemma, ok := data["lemma"].(map[string]interface{}); ok {
			if id, ok := lemma["id"].(string); ok {
				return fmt.Sprintf("Lemma ID: %s", id)
			}
		}

	case "lock_reaped":
		// Details are already in the actor field
		return ""
	}

	return ""
}
