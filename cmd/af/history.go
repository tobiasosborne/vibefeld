// Package main contains the af history command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/types"
)

func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <node-id>",
		Short: "Show node evolution history",
		Long: `Display the complete history of events affecting a specific node.

Shows all events that have affected this node in chronological order,
including creation, claims, releases, validations, challenges,
and other state changes.

Each event includes its sequence number, timestamp, event type,
actor (if applicable), and key changes.

Examples:
  af history 1                Show history of root node
  af history 1.2.3            Show history of node 1.2.3
  af history 1 --json         Output in JSON format
  af history 1 -d ./proof     Use specific proof directory`,
		Args: cobra.ExactArgs(1),
		RunE: runHistory,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().Bool("json", false, "Output in JSON format")

	return cmd
}

func runHistory(cmd *cobra.Command, args []string) error {
	// Parse node ID
	nodeID, err := types.Parse(args[0])
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", args[0], err)
	}

	// Get flags
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	jsonOutput, err := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}

	// Create ledger instance
	ledgerDir := filepath.Join(dir, "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		return fmt.Errorf("error accessing ledger: %w", err)
	}

	// Collect events affecting this node
	history := &render.NodeHistory{
		NodeID:  nodeID,
		Entries: []render.HistoryEntry{},
	}

	err = ldg.Scan(func(seq int, data []byte) error {
		// Parse the event
		var eventData map[string]interface{}
		if err := json.Unmarshal(data, &eventData); err != nil {
			return fmt.Errorf("failed to parse event %d: %w", seq, err)
		}

		// Check if this event affects our node
		if affectsNode(eventData, nodeID) {
			entry := buildHistoryEntry(seq, eventData)
			history.Entries = append(history.Entries, entry)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error scanning ledger: %w", err)
	}

	// Output based on format
	if jsonOutput {
		fmt.Fprintln(cmd.OutOrStdout(), render.FormatHistoryJSON(history))
		return nil
	}

	fmt.Fprint(cmd.OutOrStdout(), render.FormatHistory(history))
	return nil
}

// affectsNode determines if an event affects the specified node.
func affectsNode(event map[string]interface{}, nodeID types.NodeID) bool {
	eventType, _ := event["type"].(string)
	nodeIDStr := nodeID.String()

	switch eventType {
	case "node_created":
		// Check if the created node matches
		if node, ok := event["node"].(map[string]interface{}); ok {
			if id, ok := node["id"].(string); ok {
				return id == nodeIDStr
			}
		}

	case "nodes_claimed", "nodes_released":
		// Check if node is in the list
		if ids, ok := event["node_ids"].([]interface{}); ok {
			for _, id := range ids {
				if idStr, ok := id.(string); ok && idStr == nodeIDStr {
					return true
				}
			}
		}

	case "node_validated", "node_admitted", "node_refuted", "node_archived", "taint_recomputed", "lock_reaped":
		// Check node_id field
		if id, ok := event["node_id"].(string); ok {
			return id == nodeIDStr
		}

	case "challenge_raised", "challenge_superseded":
		// Check if challenge is against this node
		if id, ok := event["node_id"].(string); ok {
			return id == nodeIDStr
		}

	case "lemma_extracted":
		// Check if lemma was extracted from this node
		if lemma, ok := event["lemma"].(map[string]interface{}); ok {
			if id, ok := lemma["node_id"].(string); ok {
				return id == nodeIDStr
			}
		}
	}

	return false
}

// buildHistoryEntry creates a HistoryEntry from raw event data.
func buildHistoryEntry(seq int, event map[string]interface{}) render.HistoryEntry {
	entry := render.HistoryEntry{
		Seq:     seq,
		Details: make(map[string]interface{}),
	}

	// Extract type
	if t, ok := event["type"].(string); ok {
		entry.Type = t
	}

	// Extract timestamp
	if ts, ok := event["timestamp"].(string); ok {
		timestamp, err := types.ParseTimestamp(ts)
		if err == nil {
			entry.Timestamp = timestamp
		}
	}

	// Extract actor and details based on event type
	switch entry.Type {
	case "node_created":
		if node, ok := event["node"].(map[string]interface{}); ok {
			entry.Details["node"] = node
		}

	case "nodes_claimed":
		if owner, ok := event["owner"].(string); ok {
			entry.Actor = owner
		}
		if timeout, ok := event["timeout"].(string); ok {
			entry.Details["timeout"] = timeout
		}

	case "nodes_released":
		// No actor info typically
		if ids, ok := event["node_ids"].([]interface{}); ok {
			entry.Details["node_ids"] = ids
		}

	case "challenge_raised":
		if challengeID, ok := event["challenge_id"].(string); ok {
			entry.Details["challenge_id"] = challengeID
		}
		if target, ok := event["target"].(string); ok {
			entry.Details["target"] = target
		}
		if reason, ok := event["reason"].(string); ok {
			entry.Details["reason"] = reason
		}

	case "challenge_resolved", "challenge_withdrawn":
		if challengeID, ok := event["challenge_id"].(string); ok {
			entry.Details["challenge_id"] = challengeID
		}

	case "challenge_superseded":
		if challengeID, ok := event["challenge_id"].(string); ok {
			entry.Details["challenge_id"] = challengeID
		}

	case "taint_recomputed":
		if newTaint, ok := event["new_taint"].(string); ok {
			entry.Details["new_taint"] = newTaint
		}

	case "lemma_extracted":
		if lemma, ok := event["lemma"].(map[string]interface{}); ok {
			entry.Details["lemma"] = lemma
		}

	case "lock_reaped":
		if owner, ok := event["owner"].(string); ok {
			entry.Actor = owner
		}
	}

	return entry
}

func init() {
	rootCmd.AddCommand(newHistoryCmd())
}

// Ensure formatEventType is accessible (if needed by this file)
func formatHistoryEventType(eventType string) string {
	if eventType == "" {
		return "Unknown"
	}
	parts := strings.Split(eventType, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}
