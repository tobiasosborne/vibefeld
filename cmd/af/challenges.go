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
  --node       Show only challenges targeting a specific node
  --status     Filter by challenge status (open, resolved, withdrawn, superseded)
  --severity   Filter by severity (critical, major, minor, note)
  --category   Filter by category (gap, missing, dependency, incorrect, unclear, other)
  --active-only  Show only open challenges (shorthand for --status open)
  --summary    Show aggregate counts by node and severity instead of individual challenges

Examples:
  af challenges                         List all challenges
  af challenges --node 1.1.1            Challenges on specific node
  af challenges --status open           Only open challenges
  af challenges --active-only           Same as --status open
  af challenges --severity critical     Only critical challenges
  af challenges --category missing      Only "missing fact" challenges
  af challenges --summary               Aggregate view by node
  af challenges --format json           Machine-readable output (includes category)`,
		RunE: runChallenges,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().StringP("node", "n", "", "Filter by target node ID")
	cmd.Flags().StringP("status", "s", "", "Filter by status (open, resolved, withdrawn, superseded)")
	cmd.Flags().String("severity", "", "Filter by severity (critical, major, minor, note)")
	cmd.Flags().String("category", "", "Filter by category (gap, missing, dependency, incorrect, unclear, other)")
	cmd.Flags().Bool("active-only", false, "Show only open challenges")
	cmd.Flags().Bool("summary", false, "Show aggregate summary by node and severity")

	return cmd
}

// runChallenges executes the challenges command.
func runChallenges(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	nodeFilter, _ := cmd.Flags().GetString("node")
	statusFilter, _ := cmd.Flags().GetString("status")
	severityFilter, _ := cmd.Flags().GetString("severity")
	categoryFilter, _ := cmd.Flags().GetString("category")
	activeOnly, _ := cmd.Flags().GetBool("active-only")
	summary, _ := cmd.Flags().GetBool("summary")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// --active-only is shorthand for --status open
	if activeOnly {
		if statusFilter != "" && statusFilter != "open" {
			return fmt.Errorf("--active-only conflicts with --status %q", statusFilter)
		}
		statusFilter = "open"
	}

	// Validate status if provided
	statusFilter = strings.ToLower(statusFilter)
	if statusFilter != "" && statusFilter != "open" && statusFilter != "resolved" && statusFilter != "withdrawn" && statusFilter != "superseded" {
		return fmt.Errorf("invalid status %q: must be 'open', 'resolved', 'withdrawn', or 'superseded'", statusFilter)
	}

	// Validate severity if provided
	severityFilter = strings.ToLower(severityFilter)
	if severityFilter != "" && severityFilter != "critical" && severityFilter != "major" && severityFilter != "minor" && severityFilter != "note" {
		return fmt.Errorf("invalid severity %q: must be 'critical', 'major', 'minor', or 'note'", severityFilter)
	}

	// Validate category if provided
	categoryFilter = strings.ToLower(categoryFilter)
	if err := service.ValidateChallengeCategory(categoryFilter); err != nil {
		return fmt.Errorf("invalid category %q: must be one of %s", categoryFilter, strings.Join(service.ValidChallengeCategoryStrings(), ", "))
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
	filtered := filterChallenges(challenges, nodeID, nodeFilter != "", statusFilter, severityFilter, categoryFilter)

	// Sort challenges by node ID then by challenge ID
	sortChallenges(filtered)

	// Summary mode
	if summary {
		if format == "json" {
			output := renderChallengeSummaryJSON(filtered)
			fmt.Fprintln(cmd.OutOrStdout(), output)
			return nil
		}
		output := renderChallengeSummaryText(filtered)
		fmt.Fprint(cmd.OutOrStdout(), output)
		return nil
	}

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

// filterChallenges filters challenges based on node ID, status, severity, and category.
func filterChallenges(challenges []*service.Challenge, nodeID service.NodeID, filterByNode bool, statusFilter, severityFilter, categoryFilter string) []*service.Challenge {
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

		// Apply severity filter
		if severityFilter != "" {
			sev := c.Severity
			if sev == "" {
				sev = "major" // default severity
			}
			if sev != severityFilter {
				continue
			}
		}

		// Apply category filter (challenges with no category never match a category filter)
		if categoryFilter != "" && c.Category != categoryFilter {
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
	Category string `json:"category,omitempty"`
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
			Category: c.Category,
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

// challengeSummaryEntry represents aggregate challenge counts for one node.
type challengeSummaryEntry struct {
	NodeID   string `json:"node_id"`
	Critical int    `json:"critical"`
	Major    int    `json:"major"`
	Minor    int    `json:"minor"`
	Note     int    `json:"note"`
	Total    int    `json:"total"`
}

// buildChallengeSummary groups challenges by node and counts by severity.
func buildChallengeSummary(challenges []*service.Challenge) []challengeSummaryEntry {
	byNode := make(map[string]*challengeSummaryEntry)
	for _, c := range challenges {
		nodeID := c.NodeID.String()
		entry, ok := byNode[nodeID]
		if !ok {
			entry = &challengeSummaryEntry{NodeID: nodeID}
			byNode[nodeID] = entry
		}
		sev := c.Severity
		if sev == "" {
			sev = "major"
		}
		switch sev {
		case "critical":
			entry.Critical++
		case "major":
			entry.Major++
		case "minor":
			entry.Minor++
		case "note":
			entry.Note++
		}
		entry.Total++
	}

	entries := make([]challengeSummaryEntry, 0, len(byNode))
	for _, e := range byNode {
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Critical != entries[j].Critical {
			return entries[i].Critical > entries[j].Critical
		}
		if entries[i].Major != entries[j].Major {
			return entries[i].Major > entries[j].Major
		}
		return entries[i].NodeID < entries[j].NodeID
	})
	return entries
}

// renderChallengeSummaryText renders an aggregate summary of challenges.
func renderChallengeSummaryText(challenges []*service.Challenge) string {
	if len(challenges) == 0 {
		return "No challenges found.\n"
	}

	entries := buildChallengeSummary(challenges)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Challenge Summary: %d challenge(s) across %d node(s)\n\n", len(challenges), len(entries)))
	sb.WriteString(fmt.Sprintf("%-10s %8s %8s %8s %8s %8s\n", "NODE", "CRITICAL", "MAJOR", "MINOR", "NOTE", "TOTAL"))

	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("%-10s %8d %8d %8d %8d %8d\n",
			e.NodeID, e.Critical, e.Major, e.Minor, e.Note, e.Total))
	}

	// Totals row
	var totalCrit, totalMaj, totalMin, totalNote int
	for _, e := range entries {
		totalCrit += e.Critical
		totalMaj += e.Major
		totalMin += e.Minor
		totalNote += e.Note
	}
	sb.WriteString(fmt.Sprintf("%-10s %8d %8d %8d %8d %8d\n",
		"TOTAL", totalCrit, totalMaj, totalMin, totalNote, len(challenges)))

	return sb.String()
}

// renderChallengeSummaryJSON renders an aggregate summary as JSON.
func renderChallengeSummaryJSON(challenges []*service.Challenge) string {
	entries := buildChallengeSummary(challenges)

	result := struct {
		Nodes []challengeSummaryEntry `json:"nodes"`
		Total int                     `json:"total"`
	}{
		Nodes: entries,
		Total: len(challenges),
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal JSON: %v"}`, err)
	}
	return string(data)
}

func init() {
	rootCmd.AddCommand(newChallengesCmd())
}
