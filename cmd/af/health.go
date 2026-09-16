package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/service"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// Health status constants
const (
	HealthStatusHealthy = "healthy"
	HealthStatusStuck   = "stuck"
	HealthStatusWarning = "warning"
)

// Default thresholds for the descriptive rework section.
const (
	DefaultReworkWarnThreshold = 5
	DefaultReworkHotspots      = 5
)

// Blocker represents a condition that blocks proof progress. Severity, Age,
// Owner and Expires are optional descriptive fields added for open challenges
// and stalled/stale claims; they are omitted when not applicable so the
// original type/message/suggestion/node_ids shape is unchanged.
type Blocker struct {
	Type       string   `json:"type"`
	Level      string   `json:"level,omitempty"` // "info" or "warning"
	Message    string   `json:"message"`
	Suggestion string   `json:"suggestion"`
	NodeIDs    []string `json:"node_ids,omitempty"`
	Severity   string   `json:"severity,omitempty"`
	Age        string   `json:"age,omitempty"`
	Owner      string   `json:"owner,omitempty"`
	Expires    string   `json:"expires,omitempty"`
}

// ReworkHotspot describes descriptive per-node rework. It is deliberately not
// an alarm: repeated scrutiny is normal on a hard proof and is not evidence
// that the claim is false.
type ReworkHotspot struct {
	NodeID             string `json:"node_id"`
	ResolvedChallenges int    `json:"resolved_challenges"`
	Amendments         int    `json:"amendments"`
	RefutedChildren    int    `json:"refuted_children"`
	Rework             int    `json:"rework"`
	Warn               bool   `json:"warn,omitempty"`
}

// HealthStatistics contains proof health metrics.
type HealthStatistics struct {
	TotalNodes               int `json:"total_nodes"`
	PendingNodes             int `json:"pending_nodes"`
	ValidatedNodes           int `json:"validated_nodes"`
	AdmittedNodes            int `json:"admitted_nodes"`
	RefutedNodes             int `json:"refuted_nodes"`
	ArchivedNodes            int `json:"archived_nodes"`
	OpenChallenges           int `json:"open_challenges"`
	ProverJobs               int `json:"prover_jobs"`
	VerifierJobs             int `json:"verifier_jobs"`
	LeafNodes                int `json:"leaf_nodes"`
	BlockedLeaves            int `json:"blocked_leaves"`
	FatiguedSubtrees         int `json:"fatigued_subtrees"`
	ReworkNodes              int `json:"rework_nodes,omitempty"`
	ReworkWarned             int `json:"rework_warned,omitempty"`
	OutlineStages            int `json:"outline_stages,omitempty"`
	OutlineMapped            int `json:"outline_mapped,omitempty"`
	OutlineCriticalUntouched int `json:"outline_critical_untouched,omitempty"`
}

// HealthReport contains the complete health assessment of a proof.
type HealthReport struct {
	Status       string           `json:"status"`
	Blockers     []Blocker        `json:"blockers"`
	Rework       []ReworkHotspot  `json:"rework"`
	ReworkWarn   int              `json:"rework_warn_threshold"`
	HotspotLimit int              `json:"hotspot_limit"`
	Statistics   HealthStatistics `json:"statistics"`
}

// healthOptions carries the configurable thresholds for a health run.
type healthOptions struct {
	ReworkWarn int
	Hotspots   int
	// ClaimStall is the window after a claim's last refresh (or acquisition)
	// beyond which the claim is reported as stalled. Zero means "use each
	// claim's own lease length" (the gap between its last activity and its
	// expiry), so a claim with a long lease is not flagged merely for being
	// held a long time. This is deliberately independent of the ledger-lock
	// timeout used by Config.LockTimeout.
	ClaimStall time.Duration
}

// newHealthCmd creates the health command.
func newHealthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "health",
		GroupID: GroupSetup,
		Short:   "Check proof health and detect stuck states",
		Long: `Analyze the proof state to detect if the proof is stuck or making progress.

The health command detects:
  - All leaf nodes have open challenges (every proof path is blocked)
  - No available prover or verifier jobs (nothing to work on)
  - Open challenges, with severity and age
  - Stalled claims (no refresh within the claim-stall threshold) and stale claims (expired)
  - Untouched critical outline stages

Rework is reported, not judged. For each node it counts resolved challenges,
statement and dependency amendments, and refuted children, and lists the
top hotspots. A hotspot at or above --rework-warn is marked as a warning,
but this is rework, not evidence that the node or the conjecture is false.

Health statuses:
  - healthy: Proof has available work and no warnings
  - warning: Proof has potential issues but is not completely stuck
  - stuck: Proof cannot make progress without intervention

Output includes:
  - Overall health status
  - List of blockers with suggestions for resolution
  - Per-node rework hotspots
  - Statistics about nodes, challenges, and jobs

Examples:
  af health                      Check health in current directory
  af health --dir /path/to/proof Check health for specific proof
  af health --format json        Output in JSON format
  af health --hotspots 10        Show the 10 most-reworked nodes
  af health --rework-warn 8      Warn at 8 rework events per node
  af health --claim-stall 30m    Warn when a claim has not been refreshed in 30m`,
		RunE: runHealth,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().Int("hotspots", DefaultReworkHotspots, "Number of top rework hotspots to report")
	cmd.Flags().Int("rework-warn", DefaultReworkWarnThreshold, "Rework events per node at which a hotspot is a warning")
	cmd.Flags().Duration("claim-stall", 0, "Warn when a claim is not refreshed within this window (0 = each claim's own lease length)")

	return cmd
}

// runHealth executes the health command.
func runHealth(cmd *cobra.Command, args []string) error {
	// Get flags
	dir := service.MustString(cmd, "dir")
	format := service.MustString(cmd, "format")
	hotspots := service.MustInt(cmd, "hotspots")
	reworkWarn := service.MustInt(cmd, "rework-warn")
	claimStall, err := cmd.Flags().GetDuration("claim-stall")
	if err != nil {
		return fmt.Errorf("error reading claim-stall: %w", err)
	}

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}
	if hotspots < 0 {
		return fmt.Errorf("invalid hotspots %d: must be non-negative", hotspots)
	}
	if reworkWarn < 0 {
		return fmt.Errorf("invalid rework-warn %d: must be non-negative", reworkWarn)
	}
	if claimStall < 0 {
		return fmt.Errorf("invalid claim-stall %s: must be non-negative", claimStall)
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
	report := analyzeHealth(st, healthOptions{
		ReworkWarn: reworkWarn,
		Hotspots:   hotspots,
		ClaimStall: claimStall,
	})

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
func analyzeHealth(st *service.State, opts healthOptions) *HealthReport {
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
	openChallenges := st.OpenChallenges()

	// Find jobs using the one authoritative classifier.
	jobResult := service.FindJobs(nodes, nodeMap, challengeMap)

	// Calculate statistics
	stats := HealthStatistics{
		TotalNodes:     len(nodes),
		OpenChallenges: len(openChallenges),
		ProverJobs:     len(jobResult.ProverJobs),
		VerifierJobs:   len(jobResult.VerifierJobs),
		LeafNodes:      len(leafNodes),
	}

	// Count nodes by epistemic state
	for _, n := range nodes {
		switch n.EpistemicState {
		case service.EpistemicPending:
			stats.PendingNodes++
		case service.EpistemicValidated:
			stats.ValidatedNodes++
		case service.EpistemicAdmitted:
			stats.AdmittedNodes++
		case service.EpistemicRefuted:
			stats.RefutedNodes++
		case service.EpistemicArchived:
			stats.ArchivedNodes++
		}
	}

	// Count blocked leaf nodes (pending leaves with open challenges)
	blockedLeafIDs := []string{}
	for _, leaf := range leafNodes {
		if leaf.EpistemicState == service.EpistemicPending {
			if hasOpenChallenge(leaf, challengeMap) {
				stats.BlockedLeaves++
				blockedLeafIDs = append(blockedLeafIDs, leaf.ID.String())
			}
		}
	}

	// Per-node rework (descriptive, not an alarm).
	rework := collectRework(st, nodes, opts.ReworkWarn)
	stats.ReworkNodes = len(rework)
	for _, r := range rework {
		if r.Warn {
			stats.ReworkWarned++
		}
	}

	// Detect blockers
	var blockers []Blocker
	status := HealthStatusHealthy

	// Check 1: All leaf nodes have open challenges
	pendingLeaves := 0
	for _, leaf := range leafNodes {
		if leaf.EpistemicState == service.EpistemicPending {
			pendingLeaves++
		}
	}
	if pendingLeaves > 0 && stats.BlockedLeaves == pendingLeaves {
		blockers = append(blockers, Blocker{
			Type:       "all_leaves_challenged",
			Level:      "warning",
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
				Level:      "warning",
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
				Level:      "warning",
				Message:    fmt.Sprintf("%d of %d pending leaves have open challenges (%.0f%%)", stats.BlockedLeaves, pendingLeaves, blockerRatio*100),
				Suggestion: "Consider addressing challenges to unblock proof paths",
				NodeIDs:    blockedLeafIDs,
			})
			if status == HealthStatusHealthy {
				status = HealthStatusWarning
			}
		}
	}

	// Check 4: Open challenges, with severity and age. These are informational:
	// a challenge is normal adversarial scrutiny, not a defect in the proof.
	blockers = append(blockers, openChallengeBlockers(openChallenges, time.Now())...)

	// Check 5: Stalled and stale claims.
	stalled, stale := claimBlockers(nodes, opts.ClaimStall, time.Now())
	blockers = append(blockers, stalled...)
	blockers = append(blockers, stale...)
	if len(stalled) > 0 || len(stale) > 0 {
		if status == HealthStatusHealthy {
			status = HealthStatusWarning
		}
	}

	// Check 6: Untouched critical outline stages
	if st.HasOutline() {
		coverageReport := st.GetOutlineCoverage()
		stats.OutlineStages = coverageReport.StagesTotal
		stats.OutlineMapped = coverageReport.StagesMapped
		stats.OutlineCriticalUntouched = len(coverageReport.CriticalUntouched)

		if len(coverageReport.CriticalUntouched) > 0 {
			labels := strings.Join(coverageReport.CriticalUntouched, ", ")
			blockers = append(blockers, Blocker{
				Type:       "critical_stages_untouched",
				Level:      "warning",
				Message:    fmt.Sprintf("%d critical outline stage(s) not started: %s", len(coverageReport.CriticalUntouched), labels),
				Suggestion: "Map stages to nodes with 'af outline map' and begin work",
			})
			if status == HealthStatusHealthy {
				status = HealthStatusWarning
			}
		}
	}

	// Ensure blockers is never nil for consistent JSON output
	if blockers == nil {
		blockers = []Blocker{}
	}

	// Hotspots: the top N reworked nodes, by rework descending.
	hotspots := selectHotspots(rework, opts.Hotspots)
	if hotspots == nil {
		hotspots = []ReworkHotspot{}
	}

	return &HealthReport{
		Status:       status,
		Blockers:     blockers,
		Rework:       hotspots,
		ReworkWarn:   opts.ReworkWarn,
		HotspotLimit: opts.Hotspots,
		Statistics:   stats,
	}
}

// collectRework computes descriptive rework for every node with a nonzero
// count, sorted by rework descending (ties broken by node ID for stability).
func collectRework(st *service.State, nodes []*node.Node, warnThreshold int) []ReworkHotspot {
	var out []ReworkHotspot
	for _, n := range nodes {
		m := st.GetReworkMetrics(n.ID)
		if m.Rework == 0 {
			continue
		}
		out = append(out, ReworkHotspot{
			NodeID:             m.NodeID,
			ResolvedChallenges: m.ResolvedChallenges,
			Amendments:         m.Amendments,
			RefutedChildren:    m.RefutedChildren,
			Rework:             m.Rework,
			Warn:               warnThreshold > 0 && m.Rework >= warnThreshold,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rework != out[j].Rework {
			return out[i].Rework > out[j].Rework
		}
		return out[i].NodeID < out[j].NodeID
	})
	return out
}

// selectHotspots returns the top limit nodes, or all of them when limit is 0.
func selectHotspots(rework []ReworkHotspot, limit int) []ReworkHotspot {
	if limit <= 0 || limit >= len(rework) {
		return rework
	}
	return rework[:limit]
}

// openChallengeBlockers returns one informational blocker per open challenge,
// naming its severity and age.
func openChallengeBlockers(challenges []*state.Challenge, now time.Time) []Blocker {
	blockers := make([]Blocker, 0, len(challenges))
	for _, c := range challenges {
		if c.Status != state.ChallengeStatusOpen {
			continue
		}
		age := ""
		if !c.Created.IsZero() {
			age = formatAge(now.Sub(c.Created.Time()))
		}
		blockers = append(blockers, Blocker{
			Type:       "open_challenge",
			Level:      "info",
			Message:    fmt.Sprintf("Open %s challenge %s on node %s: %s", c.Severity, c.ID, c.NodeID.String(), c.Reason),
			Suggestion: "Address the challenge, or withdraw it if it is no longer relevant",
			NodeIDs:    []string{c.NodeID.String()},
			Severity:   c.Severity,
			Age:        age,
		})
	}
	return blockers
}

// claimBlockers returns stalled and stale claim blockers. A stale claim is one
// whose expiry has passed. A stalled claim is still active but has not been
// refreshed (ClaimLastActive) within the claim-stall window. When stall is
// zero the window is the claim's own lease length (expiry minus last activity),
// so a long lease is not itself a stall; passing a positive stall overrides it.
func claimBlockers(nodes []*node.Node, stall time.Duration, now time.Time) (stalled, stale []Blocker) {
	for _, n := range nodes {
		if n.WorkflowState != service.WorkflowClaimed {
			continue
		}
		blk := Blocker{
			NodeIDs: []string{n.ID.String()},
			Owner:   n.ClaimedBy,
		}
		if !n.ClaimedAt.IsZero() {
			blk.Expires = n.ClaimedAt.String()
		}

		// Last claim activity: the most recent refresh, falling back to the
		// acquisition time for claims replayed from pre-ClaimLastActive ledgers.
		lastActive := n.ClaimLastActive
		if lastActive.IsZero() {
			lastActive = n.ClaimedSince
		}
		if !lastActive.IsZero() {
			blk.Age = formatAge(now.Sub(lastActive.Time()))
		}

		if !n.ClaimedAt.IsZero() && n.ClaimedAt.Before(types.FromTime(now)) {
			blk.Type = "stale_claim"
			blk.Level = "warning"
			blk.Message = fmt.Sprintf("Claim on node %s by %s expired at %s", n.ID.String(), n.ClaimedBy, blk.Expires)
			blk.Suggestion = "Release it with 'af release <id>' or reap it with 'af reap'"
			stale = append(stale, blk)
			continue
		}

		// Stall window: an explicit --claim-stall wins; otherwise use the
		// claim's own lease length (expiry minus last activity).
		threshold := stall
		if threshold <= 0 && !lastActive.IsZero() && !n.ClaimedAt.IsZero() {
			threshold = n.ClaimedAt.Time().Sub(lastActive.Time())
		}
		if threshold > 0 && !lastActive.IsZero() && now.Sub(lastActive.Time()) > threshold {
			blk.Type = "stalled_claim"
			blk.Level = "warning"
			blk.Message = fmt.Sprintf("Claim on node %s by %s has not been refreshed for %s (longer than the %s claim-stall window)",
				n.ID.String(), n.ClaimedBy, blk.Age, threshold)
			blk.Suggestion = "Check the owner is still alive; release it with 'af release <id>' or reap it"
			stalled = append(stalled, blk)
		}
	}
	return stalled, stale
}

// formatAge renders a duration as a compact human-readable age.
func formatAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
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
	if report.Statistics.ReworkNodes > 0 {
		sb.WriteString(fmt.Sprintf("  Reworked nodes:   %d (%d at or above the warn threshold)\n", report.Statistics.ReworkNodes, report.Statistics.ReworkWarned))
	}
	if report.Statistics.OutlineStages > 0 {
		sb.WriteString(fmt.Sprintf("  Outline stages:   %d (%d mapped, %d critical untouched)\n", report.Statistics.OutlineStages, report.Statistics.OutlineMapped, report.Statistics.OutlineCriticalUntouched))
	}
	sb.WriteString("\n")

	sb.WriteString("Jobs:\n")
	sb.WriteString(fmt.Sprintf("  Prover jobs:      %d\n", report.Statistics.ProverJobs))
	sb.WriteString(fmt.Sprintf("  Verifier jobs:    %d\n", report.Statistics.VerifierJobs))
	sb.WriteString("\n")

	// Rework hotspots (descriptive, never an alarm)
	if len(report.Rework) > 0 {
		sb.WriteString(fmt.Sprintf("Rework hotspots (top %d by rework; warn at %d per node):\n", report.HotspotLimit, report.ReworkWarn))
		sb.WriteString("  Rework is repeated scrutiny of a hard node. It is not evidence the claim is false.\n")
		for _, r := range report.Rework {
			flag := ""
			if r.Warn {
				flag = " [warn]"
			}
			sb.WriteString(fmt.Sprintf("  %s%s: %d rework events (%d resolved challenges, %d amendments, %d refuted children)\n",
				r.NodeID, flag, r.Rework, r.ResolvedChallenges, r.Amendments, r.RefutedChildren))
			sb.WriteString(fmt.Sprintf("     %d resolved challenge(s) on this node; this is rework, not evidence of falsity\n", r.ResolvedChallenges))
		}
		sb.WriteString("\n")
	}

	// Blockers
	if len(report.Blockers) > 0 {
		sb.WriteString("Blockers:\n")
		for i, blocker := range report.Blockers {
			level := ""
			if blocker.Level != "" {
				level = fmt.Sprintf(" (%s)", blocker.Level)
			}
			sb.WriteString(fmt.Sprintf("  %d. %s%s\n", i+1, blocker.Message, level))
			sb.WriteString(fmt.Sprintf("     Suggestion: %s\n", blocker.Suggestion))
			if blocker.Severity != "" || blocker.Age != "" {
				detail := []string{}
				if blocker.Severity != "" {
					detail = append(detail, "severity: "+blocker.Severity)
				}
				if blocker.Age != "" {
					detail = append(detail, "age: "+blocker.Age)
				}
				sb.WriteString(fmt.Sprintf("     %s\n", strings.Join(detail, ", ")))
			}
			if blocker.Owner != "" || blocker.Expires != "" {
				detail := []string{}
				if blocker.Owner != "" {
					detail = append(detail, "owner: "+blocker.Owner)
				}
				if blocker.Expires != "" {
					detail = append(detail, "expires: "+blocker.Expires)
				}
				sb.WriteString(fmt.Sprintf("     %s\n", strings.Join(detail, ", ")))
			}
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
