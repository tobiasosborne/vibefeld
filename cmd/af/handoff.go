// Package main contains the af handoff command for generating concise handoff reports.
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
)

// HandoffReport contains the data for a handoff report.
type HandoffReport struct {
	Conjecture    string                 `json:"conjecture"`
	TotalNodes    int                    `json:"total_nodes"`
	Completion    int                    `json:"completion_percent"`
	ByState       map[string]int         `json:"by_state"`
	TaintSummary  map[string]int         `json:"taint_summary"`
	Challenges    []HandoffChallenge     `json:"challenges"`
	NextSteps     []string               `json:"next_steps"`
	RecentEvents  []HandoffRecentEvent   `json:"recent_events,omitempty"`
	ProverJobs    int                    `json:"prover_jobs"`
	VerifierJobs  int                    `json:"verifier_jobs"`
}

// HandoffChallenge summarizes open challenges on a node.
type HandoffChallenge struct {
	NodeID   string `json:"node_id"`
	Critical int    `json:"critical,omitempty"`
	Major    int    `json:"major,omitempty"`
	Minor    int    `json:"minor,omitempty"`
	Note     int    `json:"note,omitempty"`
	Total    int    `json:"total"`
}

// HandoffRecentEvent summarizes a recent ledger event.
type HandoffRecentEvent struct {
	Seq     int    `json:"seq"`
	Type    string `json:"type"`
	Summary string `json:"summary"`
}

func newHandoffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "handoff",
		GroupID: GroupSetup,
		Short:   "Generate a concise handoff report for session transitions",
		Long: `Generate a concise handoff report summarizing the current proof state,
open challenges, recommended next steps, and recent activity.

Designed for session transitions: gives the next agent (or human) a quick
overview of where things stand and what to do next.

The report includes:
  - Proof summary (conjecture, completion, node counts)
  - Open challenges grouped by node with severity counts
  - Recommended next steps based on available jobs and blockers
  - Recent activity from the ledger (with --since)

Output targets 50-100 lines, not a data dump.

Examples:
  af handoff                        Generate handoff for current proof
  af handoff --since 50             Include activity since event #50
  af handoff --format json          Output in JSON format
  af handoff -d /path/to/proof      Handoff for a specific proof directory`,
		RunE: runHandoff,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().Int("since", 0, "Include recent events since sequence number N")

	return cmd
}

func runHandoff(cmd *cobra.Command, args []string) error {
	dir := service.MustString(cmd, "dir")
	format := service.MustString(cmd, "format")
	since, err := cmd.Flags().GetInt("since")
	if err != nil {
		return err
	}

	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

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

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	report := buildHandoffReport(st, dir, since)

	if format == "json" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("error encoding JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	output := renderHandoffText(report)
	fmt.Fprint(cmd.OutOrStdout(), output)
	return nil
}

func buildHandoffReport(st *service.State, dir string, since int) *HandoffReport {
	nodes := st.AllNodes()
	openChallenges := st.OpenChallenges()

	// Get conjecture from root node (shallowest node)
	conjecture := ""
	for _, n := range nodes {
		if n.ID.Depth() == 1 {
			conjecture = n.Statement
			break
		}
	}

	report := &HandoffReport{
		Conjecture: conjecture,
		TotalNodes: len(nodes),
		ByState: map[string]int{
			"pending":           0,
			"validated":         0,
			"admitted":          0,
			"refuted":           0,
			"archived":          0,
			"needs_refinement":  0,
		},
		TaintSummary: map[string]int{
			"clean":         0,
			"self_admitted": 0,
			"tainted":       0,
			"unresolved":    0,
		},
	}

	// Count nodes by epistemic state and taint
	completed := 0
	for _, n := range nodes {
		switch n.EpistemicState {
		case service.EpistemicPending:
			report.ByState["pending"]++
		case service.EpistemicValidated:
			report.ByState["validated"]++
			completed++
		case service.EpistemicAdmitted:
			report.ByState["admitted"]++
			completed++
		case service.EpistemicRefuted:
			report.ByState["refuted"]++
		case service.EpistemicArchived:
			report.ByState["archived"]++
			completed++
		case service.EpistemicNeedsRefinement:
			report.ByState["needs_refinement"]++
		}

		switch n.TaintState {
		case node.TaintClean:
			report.TaintSummary["clean"]++
		case node.TaintSelfAdmitted:
			report.TaintSummary["self_admitted"]++
		case node.TaintTainted:
			report.TaintSummary["tainted"]++
		case node.TaintUnresolved:
			report.TaintSummary["unresolved"]++
		}
	}

	if report.TotalNodes > 0 {
		report.Completion = (completed * 100) / report.TotalNodes
	}

	// Group open challenges by node
	challengesByNode := make(map[string]*HandoffChallenge)
	for _, c := range openChallenges {
		nodeID := c.NodeID.String()
		hc, ok := challengesByNode[nodeID]
		if !ok {
			hc = &HandoffChallenge{NodeID: nodeID}
			challengesByNode[nodeID] = hc
		}
		switch c.Severity {
		case "critical":
			hc.Critical++
		case "major":
			hc.Major++
		case "minor":
			hc.Minor++
		case "note":
			hc.Note++
		default:
			hc.Major++ // default severity
		}
		hc.Total++
	}

	// Sort challenges: critical-heavy nodes first, then by node ID
	report.Challenges = make([]HandoffChallenge, 0, len(challengesByNode))
	for _, hc := range challengesByNode {
		report.Challenges = append(report.Challenges, *hc)
	}
	sort.Slice(report.Challenges, func(i, j int) bool {
		if report.Challenges[i].Critical != report.Challenges[j].Critical {
			return report.Challenges[i].Critical > report.Challenges[j].Critical
		}
		if report.Challenges[i].Major != report.Challenges[j].Major {
			return report.Challenges[i].Major > report.Challenges[j].Major
		}
		return report.Challenges[i].NodeID < report.Challenges[j].NodeID
	})

	// Find jobs
	nodeMap := make(map[string]*node.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID.String()] = n
	}
	challengeMap := st.ChallengeMapForJobs()
	jobResult := service.FindJobs(nodes, nodeMap, challengeMap)
	report.ProverJobs = len(jobResult.ProverJobs)
	report.VerifierJobs = len(jobResult.VerifierJobs)

	// Build next steps
	report.NextSteps = buildNextSteps(report, openChallenges)

	// Read recent events if --since specified
	if since > 0 {
		report.RecentEvents = readRecentEvents(dir, since)
	}

	return report
}

func buildNextSteps(report *HandoffReport, openChallenges []*service.Challenge) []string {
	var steps []string

	// Critical challenges first
	criticalCount := 0
	for _, c := range report.Challenges {
		criticalCount += c.Critical
	}
	if criticalCount > 0 {
		steps = append(steps, fmt.Sprintf("Resolve %d critical challenge(s) — these block verification", criticalCount))
	}

	// Prover jobs
	if report.ProverJobs > 0 {
		steps = append(steps, fmt.Sprintf("Address %d prover job(s) — nodes with open challenges need responses", report.ProverJobs))
	}

	// Verifier jobs
	if report.VerifierJobs > 0 {
		steps = append(steps, fmt.Sprintf("Review %d node(s) ready for verification", report.VerifierJobs))
	}

	// Taint issues
	tainted := report.TaintSummary["tainted"] + report.TaintSummary["self_admitted"]
	if tainted > 0 {
		steps = append(steps, fmt.Sprintf("Address taint on %d node(s) — admitted/tainted nodes weaken proof", tainted))
	}

	// Needs refinement
	if report.ByState["needs_refinement"] > 0 {
		steps = append(steps, fmt.Sprintf("Refine %d node(s) flagged for further development", report.ByState["needs_refinement"]))
	}

	if len(steps) == 0 {
		if report.Completion == 100 {
			steps = append(steps, "Proof is complete — all nodes validated or resolved")
		} else {
			steps = append(steps, "No immediate blockers — run 'af status' to review the proof tree")
		}
	}

	return steps
}

func readRecentEvents(dir string, since int) []HandoffRecentEvent {
	ledgerDir := filepath.Join(dir, "ledger")
	var events []HandoffRecentEvent

	_ = ledger.Scan(ledgerDir, func(seq int, data []byte) error {
		if seq <= since {
			return nil
		}

		var eventData map[string]interface{}
		if err := json.Unmarshal(data, &eventData); err != nil {
			return nil // skip unparseable events
		}

		eventType, _ := eventData["type"].(string)

		// Skip noise events (taint recomputed, claim refreshed, lock reaped)
		switch eventType {
		case "taint_recomputed", "claim_refreshed", "lock_reaped":
			return nil
		}

		events = append(events, HandoffRecentEvent{
			Seq:     seq,
			Type:    eventType,
			Summary: generateSummary(eventType, eventData),
		})

		return nil
	})

	return events
}

func renderHandoffText(report *HandoffReport) string {
	var sb strings.Builder

	sb.WriteString("=== Handoff Report ===\n\n")

	// Conjecture (truncated)
	conj := report.Conjecture
	if len(conj) > 80 {
		conj = conj[:77] + "..."
	}
	sb.WriteString(fmt.Sprintf("Conjecture: %s\n", conj))
	sb.WriteString(fmt.Sprintf("Completion: %d%% (%d nodes)\n\n", report.Completion, report.TotalNodes))

	// Node summary — compact single line
	sb.WriteString(fmt.Sprintf("Nodes: %d validated, %d pending, %d admitted, %d refuted, %d archived",
		report.ByState["validated"],
		report.ByState["pending"],
		report.ByState["admitted"],
		report.ByState["refuted"],
		report.ByState["archived"],
	))
	if report.ByState["needs_refinement"] > 0 {
		sb.WriteString(fmt.Sprintf(", %d needs_refinement", report.ByState["needs_refinement"]))
	}
	sb.WriteString("\n")

	// Taint — only if there are non-clean nodes
	nonClean := report.TaintSummary["self_admitted"] + report.TaintSummary["tainted"] + report.TaintSummary["unresolved"]
	if nonClean > 0 {
		sb.WriteString(fmt.Sprintf("Taint: %d clean, %d self_admitted, %d tainted, %d unresolved\n",
			report.TaintSummary["clean"],
			report.TaintSummary["self_admitted"],
			report.TaintSummary["tainted"],
			report.TaintSummary["unresolved"],
		))
	}
	sb.WriteString("\n")

	// Open challenges
	if len(report.Challenges) > 0 {
		totalChallenges := 0
		for _, c := range report.Challenges {
			totalChallenges += c.Total
		}
		sb.WriteString(fmt.Sprintf("Open Challenges: %d across %d node(s)\n", totalChallenges, len(report.Challenges)))
		for _, c := range report.Challenges {
			parts := []string{}
			if c.Critical > 0 {
				parts = append(parts, fmt.Sprintf("%d critical", c.Critical))
			}
			if c.Major > 0 {
				parts = append(parts, fmt.Sprintf("%d major", c.Major))
			}
			if c.Minor > 0 {
				parts = append(parts, fmt.Sprintf("%d minor", c.Minor))
			}
			if c.Note > 0 {
				parts = append(parts, fmt.Sprintf("%d note", c.Note))
			}
			sb.WriteString(fmt.Sprintf("  Node %-6s  %s\n", c.NodeID+":", strings.Join(parts, ", ")))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("Open Challenges: none\n\n")
	}

	// Jobs
	sb.WriteString(fmt.Sprintf("Available Jobs: %d prover, %d verifier\n\n", report.ProverJobs, report.VerifierJobs))

	// Next steps
	sb.WriteString("Next Steps:\n")
	for i, step := range report.NextSteps {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, step))
	}
	sb.WriteString("\n")

	// Recent events
	if len(report.RecentEvents) > 0 {
		sb.WriteString(fmt.Sprintf("Recent Activity (since #%d):\n", report.RecentEvents[0].Seq-1))
		for _, e := range report.RecentEvents {
			sb.WriteString(fmt.Sprintf("  #%-3d  %s\n", e.Seq, e.Summary))
		}
	}

	return sb.String()
}

func init() {
	rootCmd.AddCommand(newHandoffCmd())
}
