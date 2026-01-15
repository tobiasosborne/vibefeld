// Package main contains the af agents command implementation.
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
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// AgentEntry represents a currently claimed node and its owner.
type AgentEntry struct {
	NodeID    string          `json:"node_id"`
	Owner     string          `json:"owner"`
	ClaimedAt types.Timestamp `json:"claimed_at,omitempty"`
}

// ActivityEntry represents a historical claim/release activity.
type ActivityEntry struct {
	Seq       int             `json:"seq"`
	Type      string          `json:"type"`
	Timestamp types.Timestamp `json:"timestamp"`
	NodeIDs   []string        `json:"node_ids,omitempty"`
	Owner     string          `json:"owner,omitempty"`
}

// AgentsOutput represents the complete agents output data.
type AgentsOutput struct {
	ClaimedNodes []AgentEntry    `json:"claimed_nodes"`
	Activity     []ActivityEntry `json:"activity"`
}

func newAgentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Show agent activity and claimed nodes",
		Long: `Display agent activity including currently claimed nodes and historical claim/release events.

The agents command shows:
  - Currently claimed nodes with agent identifiers
  - Historical claim and release activity from the ledger
  - Active sessions summary

This helps monitor which agents are working on which nodes and track
claim/release patterns over time.

Examples:
  af agents                     Show agent activity in current directory
  af agents --dir /path/to/proof  Show activity for specific proof directory
  af agents --format json       Output in JSON format
  af agents --limit 20          Limit activity history to 20 most recent events`,
		RunE: runAgents,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().IntP("limit", "l", 50, "Limit activity history to N most recent events")

	return cmd
}

func runAgents(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	limit, _ := cmd.Flags().GetInt("limit")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Check if proof is initialized
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Load current state to get claimed nodes
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Collect currently claimed nodes
	var claimedNodes []AgentEntry
	for _, n := range st.AllNodes() {
		if n.WorkflowState == schema.WorkflowClaimed && n.ClaimedBy != "" {
			claimedNodes = append(claimedNodes, AgentEntry{
				NodeID:    n.ID.String(),
				Owner:     n.ClaimedBy,
				ClaimedAt: n.ClaimedAt,
			})
		}
	}

	// Sort claimed nodes by node ID for consistent output
	sort.Slice(claimedNodes, func(i, j int) bool {
		return claimedNodes[i].NodeID < claimedNodes[j].NodeID
	})

	// Collect claim/release activity from ledger
	ledgerDir := filepath.Join(dir, "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		return fmt.Errorf("error accessing ledger: %w", err)
	}

	var activity []ActivityEntry
	err = ldg.Scan(func(seq int, data []byte) error {
		// Parse the event
		var eventData map[string]interface{}
		if err := json.Unmarshal(data, &eventData); err != nil {
			return nil // Skip unparseable events
		}

		// Check if this is a claim or release event
		eventType, _ := eventData["type"].(string)
		if eventType != "nodes_claimed" && eventType != "nodes_released" && eventType != "lock_reaped" {
			return nil // Not an agent activity event
		}

		entry := ActivityEntry{
			Seq:  seq,
			Type: eventType,
		}

		// Extract timestamp
		if ts, ok := eventData["timestamp"].(string); ok {
			timestamp, err := types.ParseTimestamp(ts)
			if err == nil {
				entry.Timestamp = timestamp
			}
		}

		// Extract node IDs
		if ids, ok := eventData["node_ids"].([]interface{}); ok {
			for _, id := range ids {
				if idStr, ok := id.(string); ok {
					entry.NodeIDs = append(entry.NodeIDs, idStr)
				}
			}
		}
		// For lock_reaped, extract single node_id
		if eventType == "lock_reaped" {
			if nodeID, ok := eventData["node_id"].(string); ok {
				entry.NodeIDs = []string{nodeID}
			}
		}

		// Extract owner
		if owner, ok := eventData["owner"].(string); ok {
			entry.Owner = owner
		}

		activity = append(activity, entry)
		return nil
	})

	if err != nil {
		return fmt.Errorf("error scanning ledger: %w", err)
	}

	// Sort activity by sequence number descending (most recent first)
	sort.Slice(activity, func(i, j int) bool {
		return activity[i].Seq > activity[j].Seq
	})

	// Apply limit
	if limit > 0 && len(activity) > limit {
		activity = activity[:limit]
	}

	// Build output
	output := AgentsOutput{
		ClaimedNodes: claimedNodes,
		Activity:     activity,
	}

	// Output based on format
	if format == "json" {
		jsonData, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))
		return nil
	}

	// Text format
	fmt.Fprint(cmd.OutOrStdout(), formatAgentsText(output))
	return nil
}

// formatAgentsText formats the agents output as human-readable text.
func formatAgentsText(output AgentsOutput) string {
	var sb strings.Builder

	// Header
	sb.WriteString("=== Agent Activity ===\n\n")

	// Currently claimed nodes section
	sb.WriteString(fmt.Sprintf("Claimed Nodes (%d):\n", len(output.ClaimedNodes)))
	sb.WriteString(strings.Repeat("-", 60))
	sb.WriteString("\n")

	if len(output.ClaimedNodes) == 0 {
		sb.WriteString("  No nodes currently claimed.\n")
	} else {
		for _, entry := range output.ClaimedNodes {
			sb.WriteString(fmt.Sprintf("  [%s] claimed by %s", entry.NodeID, entry.Owner))
			if entry.ClaimedAt.String() != "" {
				sb.WriteString(fmt.Sprintf(" (expires: %s)", formatAgentTimestamp(entry.ClaimedAt)))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")

	// Activity history section
	sb.WriteString(fmt.Sprintf("Recent Activity (%d events):\n", len(output.Activity)))
	sb.WriteString(strings.Repeat("-", 60))
	sb.WriteString("\n")

	if len(output.Activity) == 0 {
		sb.WriteString("  No agent activity recorded.\n")
	} else {
		for _, entry := range output.Activity {
			// Format timestamp
			ts := formatAgentTimestamp(entry.Timestamp)

			// Format event type
			eventDesc := formatActivityType(entry.Type)

			// Format node IDs
			nodeIDs := strings.Join(entry.NodeIDs, ", ")
			if nodeIDs == "" {
				nodeIDs = "(unknown)"
			}

			// Build line
			sb.WriteString(fmt.Sprintf("  #%-4d %s  %s: %s", entry.Seq, ts, eventDesc, nodeIDs))
			if entry.Owner != "" {
				sb.WriteString(fmt.Sprintf(" by %s", entry.Owner))
			}
			sb.WriteString("\n")
		}
	}

	// Summary
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Summary: %d active claims, %d activity events shown\n",
		len(output.ClaimedNodes), len(output.Activity)))

	// Guidance
	sb.WriteString("\nNext: Run 'af status' to see proof tree, or 'af jobs' to see available work.\n")

	return sb.String()
}

// formatAgentTimestamp formats a types.Timestamp for display.
func formatAgentTimestamp(ts types.Timestamp) string {
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

// formatActivityType converts event type to human-readable form.
func formatActivityType(eventType string) string {
	switch eventType {
	case "nodes_claimed":
		return "Claimed"
	case "nodes_released":
		return "Released"
	case "lock_reaped":
		return "Lock Reaped"
	default:
		return eventType
	}
}

func init() {
	rootCmd.AddCommand(newAgentsCmd())
}
