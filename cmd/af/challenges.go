// Package main contains the af challenges command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// newChallengesCmd creates the challenges command.
func newChallengesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "challenges",
		GroupID: GroupQuery,
		Short:   "List challenges across the proof",
		Long: `List challenges across the proof.

Challenges are verifier objections against proof nodes. A challenge
identifies an issue that the prover must address before a node can
be validated.

Filter options:
  --node     Show only challenges targeting a specific node
  --status   Filter by challenge status (open, resolved, withdrawn)

Examples:
  af challenges                    List all challenges
  af challenges --node 1.1.1       Challenges on specific node
  af challenges --status open      Only open challenges
  af challenges --format json      Machine-readable output`,
		RunE: runChallenges,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().StringP("node", "n", "", "Filter by target node ID")
	cmd.Flags().StringP("status", "s", "", "Filter by status (open, resolved, withdrawn)")

	return cmd
}

// runChallenges executes the challenges command.
func runChallenges(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	nodeFilter, _ := cmd.Flags().GetString("node")
	statusFilter, _ := cmd.Flags().GetString("status")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Validate status if provided
	statusFilter = strings.ToLower(statusFilter)
	if statusFilter != "" && statusFilter != "open" && statusFilter != "resolved" && statusFilter != "withdrawn" {
		return fmt.Errorf("invalid status %q: must be 'open', 'resolved', or 'withdrawn'", statusFilter)
	}

	// Parse node filter if provided
	var nodeID service.NodeID
	if nodeFilter != "" {
		var err error
		nodeID, err = service.ParseNodeID(nodeFilter)
		if err != nil {
			return fmt.Errorf("invalid node ID %q: %w", nodeFilter, err)
		}
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

	// Load current state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Get all challenges
	challenges := st.AllChallenges()

	// Apply filters
	filtered := filterChallenges(challenges, nodeID, nodeFilter != "", statusFilter)

	// Sort challenges by node ID then by challenge ID
	sortChallenges(filtered)

	// Output based on format
	if format == "json" {
		output := renderChallengesJSON(filtered)
		fmt.Fprintln(cmd.OutOrStdout(), output)
		return nil
	}

	// Text format
	output := renderChallengesText(filtered)
	fmt.Fprint(cmd.OutOrStdout(), output)

	// Add summary
	fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d challenge(s)\n", len(filtered))

	// Add next steps if there are open challenges
	openCount := countOpenChallenges(filtered)
	if openCount > 0 {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af resolve-challenge <id>  - Resolve a challenge with an explanation")
		fmt.Fprintln(cmd.OutOrStdout(), "  af withdraw-challenge <id> - Withdraw a challenge if no longer relevant")
	}

	return nil
}

// filterChallenges filters challenges based on node ID and status.
func filterChallenges(challenges []*service.Challenge, nodeID service.NodeID, filterByNode bool, statusFilter string) []*service.Challenge {
	var result []*service.Challenge

	for _, c := range challenges {
		// Apply node filter
		if filterByNode && c.NodeID.String() != nodeID.String() {
			continue
		}

		// Apply status filter
		if statusFilter != "" && c.Status != statusFilter {
			continue
		}

		result = append(result, c)
	}

	return result
}

// sortChallenges sorts challenges by node ID (string comparison) then by challenge ID.
func sortChallenges(challenges []*service.Challenge) {
	sort.Slice(challenges, func(i, j int) bool {
		// First compare node IDs
		nodeI := challenges[i].NodeID.String()
		nodeJ := challenges[j].NodeID.String()
		if nodeI != nodeJ {
			return nodeI < nodeJ
		}
		// Then compare challenge IDs
		return challenges[i].ID < challenges[j].ID
	})
}

// renderChallengesText renders challenges as a text table.
func renderChallengesText(challenges []*service.Challenge) string {
	if len(challenges) == 0 {
		return "No challenges found.\n"
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("%-16s %-10s %-12s %-10s %-14s %s\n",
		"CHALLENGE", "NODE", "STATUS", "SEVERITY", "TARGET", "REASON"))

	// Rows
	for _, c := range challenges {
		// Truncate challenge ID for display (show first 14 chars)
		displayID := c.ID
		if len(displayID) > 14 {
			displayID = displayID[:14]
		}

		// Truncate reason for display (show first 35 chars to make room for severity)
		displayReason := c.Reason
		if len(displayReason) > 35 {
			displayReason = displayReason[:32] + "..."
		}

		// Default severity to "major" if not set (backward compatibility)
		severity := c.Severity
		if severity == "" {
			severity = "major"
		}

		sb.WriteString(fmt.Sprintf("%-16s %-10s %-12s %-10s %-14s %s\n",
			displayID, c.NodeID.String(), c.Status, severity, c.Target, displayReason))
	}

	return sb.String()
}

// challengeJSON is the JSON representation of a challenge.
type challengeJSON struct {
	ID       string `json:"id"`
	NodeID   string `json:"node_id"`
	Status   string `json:"status"`
	Severity string `json:"severity"`
	Target   string `json:"target"`
	Reason   string `json:"reason"`
	Created  string `json:"created,omitempty"`
}

// challengesResultJSON is the JSON wrapper for challenges output.
type challengesResultJSON struct {
	Challenges []challengeJSON `json:"challenges"`
	Total      int             `json:"total"`
}

// renderChallengesJSON renders challenges as JSON.
func renderChallengesJSON(challenges []*service.Challenge) string {
	result := challengesResultJSON{
		Challenges: make([]challengeJSON, 0, len(challenges)),
		Total:      len(challenges),
	}

	for _, c := range challenges {
		// Default severity to "major" if not set (backward compatibility)
		severity := c.Severity
		if severity == "" {
			severity = "major"
		}
		cj := challengeJSON{
			ID:       c.ID,
			NodeID:   c.NodeID.String(),
			Status:   c.Status,
			Severity: severity,
			Target:   c.Target,
			Reason:   c.Reason,
			Created:  c.Created.String(),
		}
		result.Challenges = append(result.Challenges, cj)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal JSON: %v"}`, err)
	}

	return string(data)
}

// countOpenChallenges counts the number of open challenges.
func countOpenChallenges(challenges []*service.Challenge) int {
	count := 0
	for _, c := range challenges {
		if c.Status == service.ChallengeStatusOpen {
			count++
		}
	}
	return count
}

func init() {
	rootCmd.AddCommand(newChallengesCmd())
}
