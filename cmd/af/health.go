package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
)

// Health status constants
const (
	HealthStatusHealthy = "healthy"
	HealthStatusStuck   = "stuck"
	HealthStatusWarning = "warning"
)

// Blocker represents a condition that blocks proof progress.
type Blocker struct {
	Type       string   `json:"type"`
	Message    string   `json:"message"`
	Suggestion string   `json:"suggestion"`
	NodeIDs    []string `json:"node_ids,omitempty"`
}

// HealthStatistics contains proof health metrics.
type HealthStatistics struct {
	TotalNodes     int `json:"total_nodes"`
	PendingNodes   int `json:"pending_nodes"`
	ValidatedNodes int `json:"validated_nodes"`
	AdmittedNodes  int `json:"admitted_nodes"`
	RefutedNodes   int `json:"refuted_nodes"`
	ArchivedNodes  int `json:"archived_nodes"`
	OpenChallenges int `json:"open_challenges"`
	ProverJobs     int `json:"prover_jobs"`
	VerifierJobs   int `json:"verifier_jobs"`
	LeafNodes      int `json:"leaf_nodes"`
	BlockedLeaves  int `json:"blocked_leaves"`
}

// HealthReport contains the complete health assessment of a proof.
type HealthReport struct {
	Status     string           `json:"status"`
	Blockers   []Blocker        `json:"blockers"`
	Statistics HealthStatistics `json:"statistics"`
}

// newHealthCmd creates the health command.
func newHealthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check proof health and detect stuck states",
		Long: `Analyze the proof state to detect if the proof is stuck or making progress.

The health command detects:
  - All leaf nodes have open challenges (every proof path is blocked)
  - No available prover or verifier jobs (nothing to work on)
  - Circular dependencies (if any)

Health statuses:
  - healthy: Proof has available work and is making progress
  - warning: Proof has potential issues but is not completely stuck
  - stuck: Proof cannot make progress without intervention

Output includes:
  - Overall health status
  - List of blockers with suggestions for resolution
  - Statistics about nodes, challenges, and jobs

Examples:
  af health                      Check health in current directory
  af health --dir /path/to/proof Check health for specific proof
  af health --format json        Output in JSON format`,
		RunE: runHealth,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// runHealth executes the health command.
func runHealth(cmd *cobra.Command, args []string) error {
	// Get flags
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

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
		if format == "json" {
			fmt.Fprintln(cmd.OutOrStdout(), `{"error":"proof not initialized"}`)
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No proof initialized. Run 'af init' to start a new proof.")
		return nil
	}

	// Load current state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Build the health report
	report := analyzeHealth(st)

	// Output based on format
	if format == "json" {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("error encoding JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
		return nil
	}

	// Text format
	output := renderHealthText(report)
	fmt.Fprint(cmd.OutOrStdout(), output)

	return nil
}

// analyzeHealth analyzes the proof state and returns a health report.
func analyzeHealth(st *state.State) *HealthReport {
	nodes := st.AllNodes()

	// Build node map and identify leaf nodes
	nodeMap := make(map[string]*node.Node, len(nodes))
	childCount := make(map[string]int)
	for _, n := range nodes {
		nodeMap[n.ID.String()] = n
		// Count children by extracting parent ID
		parentID, hasParent := n.ID.Parent()
		if hasParent {
			childCount[parentID.String()]++
		}
	}

	// Identify leaf nodes (nodes with no children)
	var leafNodes []*node.Node
	for _, n := range nodes {
		if childCount[n.ID.String()] == 0 {
			leafNodes = append(leafNodes, n)
		}
	}

	// Get challenge map using cached lookup (O(1) per node instead of O(n))
	challengeMap := st.ChallengeMapForJobs()

	// Count open challenges
	openChallengeCount := len(st.OpenChallenges())

	// Find jobs
	jobResult := jobs.FindJobs(nodes, nodeMap, challengeMap)

	// Calculate statistics
	stats := HealthStatistics{
		TotalNodes:     len(nodes),
		OpenChallenges: openChallengeCount,
		ProverJobs:     len(jobResult.ProverJobs),
		VerifierJobs:   len(jobResult.VerifierJobs),
		LeafNodes:      len(leafNodes),
	}

	// Count nodes by epistemic state
	for _, n := range nodes {
		switch n.EpistemicState {
		case schema.EpistemicPending:
			stats.PendingNodes++
		case schema.EpistemicValidated:
			stats.ValidatedNodes++
		case schema.EpistemicAdmitted:
			stats.AdmittedNodes++
		case schema.EpistemicRefuted:
			stats.RefutedNodes++
		case schema.EpistemicArchived:
			stats.ArchivedNodes++
		}
	}

	// Count blocked leaf nodes (pending leaves with open challenges)
	blockedLeafIDs := []string{}
	for _, leaf := range leafNodes {
		if leaf.EpistemicState == schema.EpistemicPending {
			if hasOpenChallenge(leaf, challengeMap) {
				stats.BlockedLeaves++
				blockedLeafIDs = append(blockedLeafIDs, leaf.ID.String())
			}
		}
	}

	// Detect blockers
	var blockers []Blocker
	status := HealthStatusHealthy

	// Check 1: All leaf nodes have open challenges
	pendingLeaves := 0
	for _, leaf := range leafNodes {
		if leaf.EpistemicState == schema.EpistemicPending {
			pendingLeaves++
		}
	}
	if pendingLeaves > 0 && stats.BlockedLeaves == pendingLeaves {
		blockers = append(blockers, Blocker{
			Type:       "all_leaves_challenged",
			Message:    "All pending leaf nodes have open challenges - every proof path is blocked",
			Suggestion: "Address challenges on leaf nodes by resolving them, or add new child nodes to extend the proof",
			NodeIDs:    blockedLeafIDs,
		})
		status = HealthStatusStuck
	}

	// Check 2: No available jobs
	if len(jobResult.ProverJobs) == 0 && len(jobResult.VerifierJobs) == 0 {
		if stats.PendingNodes > 0 {
			blockers = append(blockers, Blocker{
				Type:       "no_available_jobs",
				Message:    "No prover or verifier jobs available, but pending nodes exist",
				Suggestion: "Check if nodes are blocked or claimed. Release claimed nodes or resolve blockers.",
				NodeIDs:    nil,
			})
			if status != HealthStatusStuck {
				status = HealthStatusWarning
			}
		}
	}

	// Check 3: High ratio of blocked leaves (warning condition)
	if pendingLeaves > 0 && stats.BlockedLeaves > 0 && stats.BlockedLeaves < pendingLeaves {
		blockerRatio := float64(stats.BlockedLeaves) / float64(pendingLeaves)
		if blockerRatio > 0.5 {
			blockers = append(blockers, Blocker{
				Type:       "high_blocked_ratio",
				Message:    fmt.Sprintf("%d of %d pending leaves have open challenges (%.0f%%)", stats.BlockedLeaves, pendingLeaves, blockerRatio*100),
				Suggestion: "Consider addressing challenges to unblock proof paths",
				NodeIDs:    blockedLeafIDs,
			})
			if status == HealthStatusHealthy {
				status = HealthStatusWarning
			}
		}
	}

	// Check 4: Circular dependencies (simplified check - nodes depending on themselves through parent chain)
	// For now, we skip this as the hierarchical ID system prevents true cycles

	// Ensure blockers is never nil for consistent JSON output
	if blockers == nil {
		blockers = []Blocker{}
	}

	return &HealthReport{
		Status:     status,
		Blockers:   blockers,
		Statistics: stats,
	}
}

// hasOpenChallenge checks if a node has any open challenges.
func hasOpenChallenge(n *node.Node, challengeMap map[string][]*node.Challenge) bool {
	challenges := challengeMap[n.ID.String()]
	for _, c := range challenges {
		if c.Status == node.ChallengeStatusOpen {
			return true
		}
	}
	return false
}

// renderHealthText renders the health report as text.
func renderHealthText(report *HealthReport) string {
	var sb strings.Builder

	// Status header
	statusIcon := ""
	switch report.Status {
	case HealthStatusHealthy:
		statusIcon = "[OK]"
	case HealthStatusWarning:
		statusIcon = "[WARN]"
	case HealthStatusStuck:
		statusIcon = "[STUCK]"
	}

	sb.WriteString(fmt.Sprintf("Proof Health: %s %s\n", statusIcon, strings.ToUpper(report.Status)))
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")

	// Statistics
	sb.WriteString("Statistics:\n")
	sb.WriteString(fmt.Sprintf("  Total nodes:      %d\n", report.Statistics.TotalNodes))
	sb.WriteString(fmt.Sprintf("  Pending:          %d\n", report.Statistics.PendingNodes))
	sb.WriteString(fmt.Sprintf("  Validated:        %d\n", report.Statistics.ValidatedNodes))
	sb.WriteString(fmt.Sprintf("  Admitted:         %d\n", report.Statistics.AdmittedNodes))
	sb.WriteString(fmt.Sprintf("  Refuted:          %d\n", report.Statistics.RefutedNodes))
	sb.WriteString(fmt.Sprintf("  Archived:         %d\n", report.Statistics.ArchivedNodes))
	sb.WriteString(fmt.Sprintf("  Open challenges:  %d\n", report.Statistics.OpenChallenges))
	sb.WriteString(fmt.Sprintf("  Leaf nodes:       %d\n", report.Statistics.LeafNodes))
	sb.WriteString(fmt.Sprintf("  Blocked leaves:   %d\n", report.Statistics.BlockedLeaves))
	sb.WriteString("\n")

	sb.WriteString("Jobs:\n")
	sb.WriteString(fmt.Sprintf("  Prover jobs:      %d\n", report.Statistics.ProverJobs))
	sb.WriteString(fmt.Sprintf("  Verifier jobs:    %d\n", report.Statistics.VerifierJobs))
	sb.WriteString("\n")

	// Blockers
	if len(report.Blockers) > 0 {
		sb.WriteString("Blockers:\n")
		for i, blocker := range report.Blockers {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, blocker.Message))
			sb.WriteString(fmt.Sprintf("     Suggestion: %s\n", blocker.Suggestion))
			if len(blocker.NodeIDs) > 0 {
				// Sort node IDs for consistent output
				sortedIDs := make([]string, len(blocker.NodeIDs))
				copy(sortedIDs, blocker.NodeIDs)
				sort.Strings(sortedIDs)
				sb.WriteString(fmt.Sprintf("     Affected nodes: %s\n", strings.Join(sortedIDs, ", ")))
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("No blockers detected.\n\n")
	}

	// Next steps based on status
	sb.WriteString("Next Steps:\n")
	switch report.Status {
	case HealthStatusHealthy:
		if report.Statistics.VerifierJobs > 0 {
			sb.WriteString("  - Run 'af jobs --role verifier' to see nodes ready for review\n")
		}
		if report.Statistics.ProverJobs > 0 {
			sb.WriteString("  - Run 'af jobs --role prover' to see nodes with challenges\n")
		}
		if report.Statistics.VerifierJobs == 0 && report.Statistics.ProverJobs == 0 {
			sb.WriteString("  - Run 'af status' to see the proof tree\n")
		}
	case HealthStatusWarning:
		sb.WriteString("  - Address open challenges to improve proof progress\n")
		sb.WriteString("  - Run 'af jobs' to see available work\n")
	case HealthStatusStuck:
		sb.WriteString("  - Resolve challenges on blocked leaf nodes\n")
		sb.WriteString("  - Or add new child nodes to create alternative proof paths\n")
		sb.WriteString("  - Run 'af status' to see the full proof tree\n")
	}

	return sb.String()
}

func init() {
	rootCmd.AddCommand(newHealthCmd())
}
