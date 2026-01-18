package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
)

// newJobsCmd creates the jobs command.
func newJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "jobs",
		GroupID: GroupWorkflow,
		Short:   "List available jobs",
		Long: `List available prover and verifier jobs in the proof.

Verifier jobs are nodes ready for review (breadth-first model):
  - Has a statement
  - EpistemicState = "pending" (not yet verified)
  - WorkflowState = "available" (not claimed or blocked)
  - Has NO open/unresolved challenges

Prover jobs are nodes with challenges that need addressing:
  - EpistemicState = "pending"
  - Has one or more open challenges

Every new node is immediately a verifier job. When a verifier raises a
challenge, the node becomes a prover job. When challenges are resolved,
the node returns to verifier territory for final acceptance.

Examples:
  af jobs                     List all available jobs
  af jobs --role prover       List only prover jobs
  af jobs --role verifier     List only verifier jobs
  af jobs --format json       Output in JSON format

Workflow:
  To start working on a job, use 'af claim <node-id>' to claim it first.
  This prevents other agents from working on the same node. Once claimed,
  use the appropriate command for your role:
    Prover:   af refine <id>, af amend <id>, af resolve-challenge <id>
    Verifier: af accept <id>, af challenge <id>`,
		RunE: runJobs,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().StringP("role", "r", "", "Filter by role (prover or verifier)")

	return cmd
}

// runJobs executes the jobs command.
func runJobs(cmd *cobra.Command, args []string) error {
	// Get flags
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	role := cli.MustString(cmd, "role")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Validate role if provided (check if flag was explicitly set)
	roleSet := cmd.Flags().Changed("role")
	role = strings.ToLower(role)
	if roleSet && role != "prover" && role != "verifier" {
		return fmt.Errorf("invalid role %q: must be 'prover' or 'verifier'", role)
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

	// Get all nodes and build node map
	nodes := st.AllNodes()
	nodeMap := make(map[string]*node.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID.String()] = n
	}

	// Get challenge map using cached lookup (O(1) per node instead of O(n))
	challengeMap := st.ChallengeMapForJobs()

	// Build severity map for challenge severity counts
	severityMap := buildSeverityMap(st.AllChallenges())

	// Find jobs
	jobResult := service.FindJobs(nodes, nodeMap, challengeMap)

	// Apply role filter if specified
	if roleSet && role == "prover" {
		jobResult = &service.JobResult{
			ProverJobs:   jobResult.ProverJobs,
			VerifierJobs: nil,
		}
	} else if roleSet && role == "verifier" {
		jobResult = &service.JobResult{
			ProverJobs:   nil,
			VerifierJobs: jobResult.VerifierJobs,
		}
	}

	// Output based on format
	if format == "json" {
		output := renderJobsJSONWithSeverity(jobResult, severityMap)
		fmt.Fprintln(cmd.OutOrStdout(), output)
		return nil
	}

	// Text format
	output := renderJobsWithSeverity(jobResult, severityMap)
	fmt.Fprint(cmd.OutOrStdout(), output)

	// Add summary line showing both job type counts
	proverCount := len(jobResult.ProverJobs)
	verifierCount := len(jobResult.VerifierJobs)
	fmt.Fprintf(cmd.OutOrStdout(), "\nSummary: %d prover job(s), %d verifier job(s)\n", proverCount, verifierCount)

	return nil
}

func init() {
	rootCmd.AddCommand(newJobsCmd())
}

// severityCounts tracks the count of open challenges by severity for a node.
type severityCounts struct {
	Critical int `json:"critical,omitempty"`
	Major    int `json:"major,omitempty"`
	Minor    int `json:"minor,omitempty"`
	Note     int `json:"note,omitempty"`
}

// buildSeverityMap builds a map of nodeID -> severity counts for open challenges.
func buildSeverityMap(challenges []*state.Challenge) map[string]*severityCounts {
	result := make(map[string]*severityCounts)

	for _, c := range challenges {
		// Only count open challenges
		if c.Status != state.ChallengeStatusOpen {
			continue
		}

		nodeIDStr := c.NodeID.String()
		counts, ok := result[nodeIDStr]
		if !ok {
			counts = &severityCounts{}
			result[nodeIDStr] = counts
		}

		switch service.ChallengeSeverity(c.Severity) {
		case service.SeverityCritical:
			counts.Critical++
		case service.SeverityMajor:
			counts.Major++
		case service.SeverityMinor:
			counts.Minor++
		case service.SeverityNote:
			counts.Note++
		}
	}

	return result
}

// formatSeverityCounts returns a human-readable string of severity counts.
// Only shows non-zero counts. Example: "[1 critical, 2 minor challenges]"
func formatSeverityCounts(counts *severityCounts) string {
	if counts == nil {
		return ""
	}

	var parts []string
	if counts.Critical > 0 {
		parts = append(parts, fmt.Sprintf("%d critical", counts.Critical))
	}
	if counts.Major > 0 {
		parts = append(parts, fmt.Sprintf("%d major", counts.Major))
	}
	if counts.Minor > 0 {
		parts = append(parts, fmt.Sprintf("%d minor", counts.Minor))
	}
	if counts.Note > 0 {
		parts = append(parts, fmt.Sprintf("%d note", counts.Note))
	}

	if len(parts) == 0 {
		return ""
	}

	suffix := "challenge"
	total := counts.Critical + counts.Major + counts.Minor + counts.Note
	if total > 1 {
		suffix = "challenges"
	}

	return fmt.Sprintf("[%s %s]", strings.Join(parts, ", "), suffix)
}

// renderJobsWithSeverity renders jobs with severity counts included.
func renderJobsWithSeverity(jobResult *service.JobResult, severityMap map[string]*severityCounts) string {
	if jobResult == nil || jobResult.IsEmpty() {
		return "No jobs available.\n\nProver jobs: 0 nodes awaiting refinement\nVerifier jobs: 0 nodes ready for review"
	}

	var sb strings.Builder

	// Sort prover jobs by priority (most critical first, then by depth)
	proverJobs := make([]*node.Node, len(jobResult.ProverJobs))
	copy(proverJobs, jobResult.ProverJobs)
	sort.Slice(proverJobs, func(i, j int) bool {
		pi := proverJobPriority(proverJobs[i], severityMap[proverJobs[i].ID.String()])
		pj := proverJobPriority(proverJobs[j], severityMap[proverJobs[j].ID.String()])
		if pi != pj {
			return pi < pj
		}
		// Tiebreaker: ID for stable sort
		return proverJobs[i].ID.String() < proverJobs[j].ID.String()
	})

	// Sort verifier jobs by depth (breadth-first: shallower first)
	verifierJobs := make([]*node.Node, len(jobResult.VerifierJobs))
	copy(verifierJobs, jobResult.VerifierJobs)
	sort.Slice(verifierJobs, func(i, j int) bool {
		di := verifierJobPriority(verifierJobs[i])
		dj := verifierJobPriority(verifierJobs[j])
		if di != dj {
			return di < dj
		}
		// Tiebreaker: ID for stable sort
		return verifierJobs[i].ID.String() < verifierJobs[j].ID.String()
	})

	// Render prover jobs section
	if len(proverJobs) > 0 {
		sb.WriteString(fmt.Sprintf("=== Prover Jobs (%d available) ===\n", len(proverJobs)))
		sb.WriteString("Nodes awaiting refinement. Claim one and refine the proof.\n")
		sb.WriteString("Sorted by urgency: critical challenges first, then by depth.\n\n")
		for i, n := range proverJobs {
			isRecommended := i == 0
			renderJobNodeWithPriority(&sb, n, severityMap[n.ID.String()], isRecommended, true)
		}
		if len(proverJobs) > 0 {
			recommended := proverJobs[0]
			reason := proverPriorityReason(severityMap[recommended.ID.String()])
			sb.WriteString(fmt.Sprintf("\nRecommended: Start with [%s] (%s)\n", recommended.ID.String(), reason))
		}
		sb.WriteString("Next: Run 'af claim <id>' to claim a prover job, then 'af refine <id>' to work on it.\n")
	}

	// Add separator between sections if both have jobs
	if len(proverJobs) > 0 && len(verifierJobs) > 0 {
		sb.WriteString("\n")
	}

	// Render verifier jobs section
	if len(verifierJobs) > 0 {
		sb.WriteString(fmt.Sprintf("=== Verifier Jobs (%d available) ===\n", len(verifierJobs)))
		sb.WriteString("Nodes ready for review. Verify or challenge the proof.\n")
		sb.WriteString("Sorted by depth: breadth-first review (shallower nodes first).\n\n")
		for i, n := range verifierJobs {
			isRecommended := i == 0
			renderJobNodeWithPriority(&sb, n, severityMap[n.ID.String()], isRecommended, false)
		}
		if len(verifierJobs) > 0 {
			recommended := verifierJobs[0]
			reason := verifierPriorityReason(recommended)
			sb.WriteString(fmt.Sprintf("\nRecommended: Start with [%s] (%s)\n", recommended.ID.String(), reason))
		}
		sb.WriteString("Next: Run 'af claim <id>' to claim a verifier job, then 'af accept <id>' to validate or 'af challenge <id>' to raise objections.\n")
	}

	return sb.String()
}

// renderJobNodeWithSeverity renders a single job node entry with severity counts.
func renderJobNodeWithSeverity(sb *strings.Builder, n *node.Node, counts *severityCounts) {
	// Sanitize statement (remove control chars, normalize whitespace) but do NOT truncate
	stmt := sanitizeJobStatement(n.Statement)

	// Build the line with severity counts if present
	severityStr := formatSeverityCounts(counts)
	if severityStr != "" {
		sb.WriteString(fmt.Sprintf("  [%s] %s: %q %s\n", n.ID.String(), string(n.Type), stmt, severityStr))
	} else {
		sb.WriteString(fmt.Sprintf("  [%s] %s: %q\n", n.ID.String(), string(n.Type), stmt))
	}

	// Show claimed-by info for verifier jobs
	if n.ClaimedBy != "" {
		sb.WriteString(fmt.Sprintf("         claimed by: %s\n", n.ClaimedBy))
	}
}

// renderJobNodeWithPriority renders a single job node entry with priority indicator.
// isRecommended marks the recommended starting job with a star.
// isProver determines whether to show prover-specific or verifier-specific info.
func renderJobNodeWithPriority(sb *strings.Builder, n *node.Node, counts *severityCounts, isRecommended bool, isProver bool) {
	// Sanitize statement (remove control chars, normalize whitespace) but do NOT truncate
	stmt := sanitizeJobStatement(n.Statement)

	// Priority indicator
	prefix := "  "
	if isRecommended {
		prefix = "* "
	}

	// Build the line with severity counts if present
	severityStr := formatSeverityCounts(counts)
	if severityStr != "" {
		sb.WriteString(fmt.Sprintf("%s[%s] %s: %q %s\n", prefix, n.ID.String(), string(n.Type), stmt, severityStr))
	} else {
		sb.WriteString(fmt.Sprintf("%s[%s] %s: %q\n", prefix, n.ID.String(), string(n.Type), stmt))
	}

	// Show claimed-by info
	if n.ClaimedBy != "" {
		sb.WriteString(fmt.Sprintf("         claimed by: %s\n", n.ClaimedBy))
	}
}

// proverJobPriority calculates a priority score for a prover job.
// Lower score = higher priority. Factors:
// - Critical challenges (weight 1000 per challenge)
// - Major challenges (weight 100 per challenge)
// - Shallower depth (weight 1 per level)
func proverJobPriority(n *node.Node, counts *severityCounts) int {
	score := 0
	if counts != nil {
		// Critical challenges are most urgent
		score -= counts.Critical * 1000
		// Major challenges next
		score -= counts.Major * 100
	}
	// Add depth as tiebreaker (prefer shallower nodes)
	score += n.Depth()
	return score
}

// proverPriorityReason explains why a prover job is prioritized.
func proverPriorityReason(counts *severityCounts) string {
	if counts == nil {
		return "oldest pending job"
	}
	if counts.Critical > 0 {
		return "has critical challenge(s)"
	}
	if counts.Major > 0 {
		return "has major challenge(s)"
	}
	return "shallowest depth"
}

// verifierJobPriority calculates a priority score for a verifier job.
// Lower score = higher priority. Uses depth (breadth-first review).
func verifierJobPriority(n *node.Node) int {
	return n.Depth()
}

// verifierPriorityReason explains why a verifier job is prioritized.
func verifierPriorityReason(n *node.Node) string {
	if n.Depth() == 0 {
		return "root node"
	}
	return "shallowest pending node"
}

// sanitizeJobStatement sanitizes statement text for display.
func sanitizeJobStatement(s string) string {
	// Replace control characters and normalize whitespace
	var sb strings.Builder
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\t' {
			sb.WriteRune(' ')
		} else {
			sb.WriteRune(r)
		}
	}
	// Collapse multiple spaces into one
	result := sb.String()
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	return strings.TrimSpace(result)
}

// jobsJSONJobEntry represents a single job entry with severity info in JSON format.
type jobsJSONJobEntry struct {
	NodeID         string          `json:"node_id"`
	Statement      string          `json:"statement"`
	Type           string          `json:"type"`
	Depth          int             `json:"depth"`
	SeverityCounts *severityCounts `json:"severity_counts,omitempty"`
	Recommended    bool            `json:"recommended,omitempty"`
	PriorityReason string          `json:"priority_reason,omitempty"`
}

// jobsJSONOutput represents the JSON output for jobs command with severity info.
type jobsJSONOutput struct {
	ProverJobs   []jobsJSONJobEntry `json:"prover_jobs"`
	VerifierJobs []jobsJSONJobEntry `json:"verifier_jobs"`
}

// renderJobsJSONWithSeverity renders jobs as JSON with severity counts included.
// Jobs are sorted by priority and include recommended flags.
func renderJobsJSONWithSeverity(jobResult *service.JobResult, severityMap map[string]*severityCounts) string {
	if jobResult == nil {
		return `{"prover_jobs":[],"verifier_jobs":[]}`
	}

	// Sort prover jobs by priority
	proverJobs := make([]*node.Node, len(jobResult.ProverJobs))
	copy(proverJobs, jobResult.ProverJobs)
	sort.Slice(proverJobs, func(i, j int) bool {
		pi := proverJobPriority(proverJobs[i], severityMap[proverJobs[i].ID.String()])
		pj := proverJobPriority(proverJobs[j], severityMap[proverJobs[j].ID.String()])
		if pi != pj {
			return pi < pj
		}
		return proverJobs[i].ID.String() < proverJobs[j].ID.String()
	})

	// Sort verifier jobs by depth
	verifierJobs := make([]*node.Node, len(jobResult.VerifierJobs))
	copy(verifierJobs, jobResult.VerifierJobs)
	sort.Slice(verifierJobs, func(i, j int) bool {
		di := verifierJobPriority(verifierJobs[i])
		dj := verifierJobPriority(verifierJobs[j])
		if di != dj {
			return di < dj
		}
		return verifierJobs[i].ID.String() < verifierJobs[j].ID.String()
	})

	output := jobsJSONOutput{
		ProverJobs:   make([]jobsJSONJobEntry, 0, len(proverJobs)),
		VerifierJobs: make([]jobsJSONJobEntry, 0, len(verifierJobs)),
	}

	for i, job := range proverJobs {
		counts := severityMap[job.ID.String()]
		entry := jobsJSONJobEntry{
			NodeID:    job.ID.String(),
			Statement: job.Statement,
			Type:      string(job.Type),
			Depth:     job.Depth(),
		}
		if counts != nil {
			entry.SeverityCounts = counts
		}
		if i == 0 {
			entry.Recommended = true
			entry.PriorityReason = proverPriorityReason(counts)
		}
		output.ProverJobs = append(output.ProverJobs, entry)
	}

	for i, job := range verifierJobs {
		counts := severityMap[job.ID.String()]
		entry := jobsJSONJobEntry{
			NodeID:    job.ID.String(),
			Statement: job.Statement,
			Type:      string(job.Type),
			Depth:     job.Depth(),
		}
		if counts != nil {
			entry.SeverityCounts = counts
		}
		if i == 0 {
			entry.Recommended = true
			entry.PriorityReason = verifierPriorityReason(job)
		}
		output.VerifierJobs = append(output.VerifierJobs, entry)
	}

	data, err := json.Marshal(output)
	if err != nil {
		return `{"prover_jobs":[],"verifier_jobs":[]}`
	}

	return string(data)
}
