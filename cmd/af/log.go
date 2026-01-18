// Package main contains the af log command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
)

func newLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "log",
		GroupID: GroupAdmin,
		Short:   "Show event ledger history",
		Long: `Display the event ledger history for the proof.

Shows all events that have been recorded in the ledger, including
proof initialization, node creation, claims, releases, validations,
and other state changes.

Each event includes its sequence number, type, timestamp, and a
summary of the event details.

Examples:
  af log                      Show all events
  af log --since 10           Show events after sequence 10
  af log -n 5                 Show only the first 5 events
  af log --reverse            Show newest events first
  af log --reverse -n 10      Show the 10 newest events
  af log -f json              Output in JSON format
  af log -d ./proof           Use specific proof directory`,
		RunE: runLog,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")
	cmd.Flags().Int("since", 0, "Show events since sequence number N")
	cmd.Flags().IntP("limit", "n", 0, "Limit output to N events (0 = unlimited)")
	cmd.Flags().Bool("reverse", false, "Show newest events first")

	return cmd
}

// logEntry represents a parsed event for display.
type logEntry struct {
	Seq       int                    `json:"seq"`
	Type      string                 `json:"type"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"-"`
	RawData   json.RawMessage        `json:"data,omitempty"`
}

func runLog(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	since, err := cmd.Flags().GetInt("since")
	if err != nil {
		return err
	}
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return err
	}
	reverse, err := cmd.Flags().GetBool("reverse")
	if err != nil {
		return err
	}

	// Validate format
	format = strings.ToLower(format)
	if format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Create ledger instance
	ledgerDir := filepath.Join(dir, "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		return fmt.Errorf("error accessing ledger: %w", err)
	}

	// Collect events
	var entries []logEntry
	err = ldg.Scan(func(seq int, data []byte) error {
		// Apply since filter
		if since > 0 && seq <= since {
			return nil
		}

		// Parse the event
		var eventData map[string]interface{}
		if err := json.Unmarshal(data, &eventData); err != nil {
			return fmt.Errorf("failed to parse event %d: %w", seq, err)
		}

		entry := logEntry{
			Seq:     seq,
			Data:    eventData,
			RawData: data,
		}

		// Extract type
		if t, ok := eventData["type"].(string); ok {
			entry.Type = t
		}

		// Extract timestamp
		if ts, ok := eventData["timestamp"].(string); ok {
			entry.Timestamp = ts
		}

		entries = append(entries, entry)
		return nil
	})

	if err != nil {
		return fmt.Errorf("error scanning ledger: %w", err)
	}

	// Apply reverse if requested
	if reverse {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Seq > entries[j].Seq
		})
	}

	// Apply limit if specified
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	// Output based on format
	if format == "json" {
		return outputLogJSON(cmd, entries)
	}

	return outputLogText(cmd, entries)
}

func outputLogJSON(cmd *cobra.Command, entries []logEntry) error {
	// Build JSON output array
	output := make([]map[string]interface{}, 0, len(entries))

	for _, entry := range entries {
		item := map[string]interface{}{
			"seq":       entry.Seq,
			"type":      entry.Type,
			"timestamp": entry.Timestamp,
		}

		// Include additional data fields from the event
		for k, v := range entry.Data {
			if k != "type" && k != "timestamp" {
				item[k] = v
			}
		}

		output = append(output, item)
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func outputLogText(cmd *cobra.Command, entries []logEntry) error {
	if len(entries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No events found.")
		return nil
	}

	for _, entry := range entries {
		// Format timestamp for display
		displayTime := formatTimestamp(entry.Timestamp)

		// Format event type for display (convert snake_case to PascalCase)
		displayType := formatEventType(entry.Type)

		// Generate summary based on event type
		summary := generateSummary(entry.Type, entry.Data)

		// Output: #seq  Type  Timestamp  Summary
		fmt.Fprintf(cmd.OutOrStdout(), "#%-3d  %-20s  %s  %s\n",
			entry.Seq, displayType, displayTime, summary)
	}

	return nil
}

// formatTimestamp formats an ISO8601 timestamp for display.
func formatTimestamp(ts string) string {
	if ts == "" {
		return "                   "
	}

	// Parse the timestamp
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		// Try parsing with nanoseconds
		t, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return ts[:min(19, len(ts))] // Return truncated original
		}
	}

	return t.Format("2006-01-02 15:04:05")
}

// formatEventType converts snake_case event type to PascalCase for display.
func formatEventType(eventType string) string {
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

	return strings.Join(parts, "")
}

// generateSummary creates a human-readable summary for an event.
func generateSummary(eventType string, data map[string]interface{}) string {
	switch eventType {
	case "proof_initialized":
		if conj, ok := data["conjecture"].(string); ok {
			// Truncate long conjectures
			if len(conj) > 50 {
				conj = conj[:47] + "..."
			}
			return fmt.Sprintf("Initialized proof: %q", conj)
		}
		return "Initialized proof"

	case "node_created":
		if node, ok := data["node"].(map[string]interface{}); ok {
			id := ""
			nodeType := ""
			if idVal, ok := node["id"].(string); ok {
				id = idVal
			}
			if typeVal, ok := node["type"].(string); ok {
				nodeType = typeVal
			}
			return fmt.Sprintf("Created node %s: %s", id, nodeType)
		}
		return "Created node"

	case "nodes_claimed":
		if ids, ok := data["node_ids"].([]interface{}); ok {
			idStrs := make([]string, 0, len(ids))
			for _, id := range ids {
				if s, ok := id.(string); ok {
					idStrs = append(idStrs, s)
				}
			}
			return fmt.Sprintf("Claimed nodes: %s", strings.Join(idStrs, ", "))
		}
		return "Claimed nodes"

	case "nodes_released":
		if ids, ok := data["node_ids"].([]interface{}); ok {
			idStrs := make([]string, 0, len(ids))
			for _, id := range ids {
				if s, ok := id.(string); ok {
					idStrs = append(idStrs, s)
				}
			}
			return fmt.Sprintf("Released nodes: %s", strings.Join(idStrs, ", "))
		}
		return "Released nodes"

	case "node_validated":
		if id, ok := data["node_id"].(string); ok {
			return fmt.Sprintf("Validated node %s", id)
		}
		return "Validated node"

	case "node_admitted":
		if id, ok := data["node_id"].(string); ok {
			return fmt.Sprintf("Admitted node %s", id)
		}
		return "Admitted node"

	case "node_refuted":
		if id, ok := data["node_id"].(string); ok {
			return fmt.Sprintf("Refuted node %s", id)
		}
		return "Refuted node"

	case "node_archived":
		if id, ok := data["node_id"].(string); ok {
			return fmt.Sprintf("Archived node %s", id)
		}
		return "Archived node"

	case "challenge_raised":
		if id, ok := data["challenge_id"].(string); ok {
			nodeID := ""
			if nid, ok := data["node_id"].(string); ok {
				nodeID = nid
			}
			return fmt.Sprintf("Challenge %s raised on node %s", id, nodeID)
		}
		return "Challenge raised"

	case "challenge_resolved":
		if id, ok := data["challenge_id"].(string); ok {
			return fmt.Sprintf("Challenge %s resolved", id)
		}
		return "Challenge resolved"

	case "challenge_withdrawn":
		if id, ok := data["challenge_id"].(string); ok {
			return fmt.Sprintf("Challenge %s withdrawn", id)
		}
		return "Challenge withdrawn"

	case "def_added":
		if def, ok := data["definition"].(map[string]interface{}); ok {
			if name, ok := def["name"].(string); ok {
				return fmt.Sprintf("Added definition: %s", name)
			}
		}
		return "Added definition"

	case "lemma_extracted":
		if lemma, ok := data["lemma"].(map[string]interface{}); ok {
			if id, ok := lemma["id"].(string); ok {
				return fmt.Sprintf("Extracted lemma %s", id)
			}
		}
		return "Extracted lemma"

	case "taint_recomputed":
		if id, ok := data["node_id"].(string); ok {
			newTaint := ""
			if t, ok := data["new_taint"].(string); ok {
				newTaint = t
			}
			return fmt.Sprintf("Recomputed taint for node %s: %s", id, newTaint)
		}
		return "Recomputed taint"

	case "lock_reaped":
		if id, ok := data["node_id"].(string); ok {
			return fmt.Sprintf("Reaped lock on node %s", id)
		}
		return "Reaped lock"

	default:
		return eventType
	}
}

func init() {
	rootCmd.AddCommand(newLogCmd())
}
