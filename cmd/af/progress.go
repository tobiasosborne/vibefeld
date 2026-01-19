// Package main contains the af progress command for showing proof progress metrics.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
)

// ProgressMetrics contains computed progress metrics for a proof.
type ProgressMetrics struct {
	TotalNodes        int            `json:"total_nodes"`
	CompletedNodes    int            `json:"completed_nodes"`
	CompletionPercent int            `json:"completion_percent"`
	ByState           map[string]int `json:"by_state"`
	OpenChallenges    int            `json:"open_challenges"`
	PendingDefs       int            `json:"pending_definitions"`
	BlockedNodes      int            `json:"blocked_nodes"`
	CriticalPath      []string       `json:"critical_path"`
	CriticalPathDepth int            `json:"critical_path_depth"`
}

// newProgressCmd creates the progress command for showing proof progress metrics.
func newProgressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "progress",
		GroupID: GroupSetup,
		Short:   "Show proof progress metrics and completion status",
		Long: `Show the current proof progress including completion percentage, node counts by state,
blockers, and the critical path (deepest pending branch).

The progress command displays:
  - Completion percentage (validated + admitted nodes / total nodes)
  - Node counts by epistemic state (pending, validated, admitted, refuted, archived)
  - Open challenges count
  - Pending definition requests count
  - Blocked nodes count
  - Critical path (deepest branch with pending nodes)

Examples:
  af progress                     Show proof progress in current directory
  af progress --dir /path/to/proof  Show progress for specific proof directory
  af progress --format json       Output in JSON format`,
		RunE: runProgress,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// runProgress executes the progress command.
func runProgress(cmd *cobra.Command, args []string) error {
	// Get flags
	dir := service.MustString(cmd, "dir")
	format := service.MustString(cmd, "format")

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

	// Load pending definitions
	pendingDefs, err := loadPendingDefs(svc)
	if err != nil {
		return fmt.Errorf("error loading pending definitions: %w", err)
	}

	// Compute metrics
	metrics := computeProgressMetrics(st, pendingDefs)

	// Output based on format
	if format == "json" {
		return outputProgressJSON(cmd, metrics)
	}

	return outputProgressText(cmd, metrics)
}

// loadPendingDefs loads all pending definitions from the proof directory.
// Only returns definitions in "pending" status (not resolved or cancelled).
func loadPendingDefs(svc *service.ProofService) ([]*node.PendingDef, error) {
	allPendingDefs, err := svc.LoadAllPendingDefs()
	if err != nil {
		return nil, err
	}

	// Filter to only pending (not resolved or cancelled)
	pendingDefs := make([]*node.PendingDef, 0, len(allPendingDefs))
	for _, pd := range allPendingDefs {
		if pd.IsPending() {
			pendingDefs = append(pendingDefs, pd)
		}
	}

	return pendingDefs, nil
}

// computeProgressMetrics calculates progress metrics from the current state.
func computeProgressMetrics(st *service.State, pendingDefs []*node.PendingDef) *ProgressMetrics {
	nodes := st.AllNodes()
	challenges := st.OpenChallenges()

	metrics := &ProgressMetrics{
		TotalNodes:     len(nodes),
		CompletedNodes: 0,
		ByState: map[string]int{
			"pending":   0,
			"validated": 0,
			"admitted":  0,
			"refuted":   0,
			"archived":  0,
		},
		OpenChallenges: len(challenges),
		PendingDefs:    len(pendingDefs),
		BlockedNodes:   0,
		CriticalPath:   []string{},
	}

	// Count nodes by state
	for _, n := range nodes {
		switch n.EpistemicState {
		case service.EpistemicPending:
			metrics.ByState["pending"]++
		case service.EpistemicValidated:
			metrics.ByState["validated"]++
			metrics.CompletedNodes++
		case service.EpistemicAdmitted:
			metrics.ByState["admitted"]++
			metrics.CompletedNodes++
		case service.EpistemicRefuted:
			metrics.ByState["refuted"]++
		case service.EpistemicArchived:
			metrics.ByState["archived"]++
			metrics.CompletedNodes++ // Archived nodes are resolved, don't block completion
		}

		// Count blocked nodes
		if n.WorkflowState == service.WorkflowBlocked {
			metrics.BlockedNodes++
		}
	}

	// Calculate completion percentage
	if metrics.TotalNodes > 0 {
		metrics.CompletionPercent = (metrics.CompletedNodes * 100) / metrics.TotalNodes
	}

	// Find critical path (deepest pending branch)
	metrics.CriticalPath, metrics.CriticalPathDepth = findCriticalPath(nodes)

	return metrics
}

// findCriticalPath finds the deepest branch that has pending nodes.
// Returns the path as a slice of node ID strings and the depth.
func findCriticalPath(nodes []*node.Node) ([]string, int) {
	if len(nodes) == 0 {
		return []string{}, 0
	}

	// Build a map for quick lookup
	nodeMap := make(map[string]*node.Node)
	for _, n := range nodes {
		nodeMap[n.ID.String()] = n
	}

	// Find the deepest pending node
	var deepestPending *node.Node
	maxDepth := 0

	for _, n := range nodes {
		if n.EpistemicState == service.EpistemicPending {
			depth := n.ID.Depth()
			if depth > maxDepth {
				maxDepth = depth
				deepestPending = n
			}
		}
	}

	if deepestPending == nil {
		// No pending nodes - proof might be complete or all refuted/archived
		return []string{}, 0
	}

	// Build path from root to deepest pending node
	var path []string
	currentID := deepestPending.ID

	// Walk up to root, collecting IDs
	for {
		path = append([]string{currentID.String()}, path...)
		parentID, hasParent := currentID.Parent()
		if !hasParent {
			break
		}
		currentID = parentID
	}

	return path, len(path)
}

// outputProgressJSON outputs progress metrics in JSON format.
func outputProgressJSON(cmd *cobra.Command, metrics *ProgressMetrics) error {
	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputProgressText outputs progress metrics in text format.
func outputProgressText(cmd *cobra.Command, metrics *ProgressMetrics) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "=== Proof Progress ===")
	fmt.Fprintln(out)

	// Completion
	fmt.Fprintf(out, "Completion: %d%% (%d/%d nodes validated or admitted)\n",
		metrics.CompletionPercent, metrics.CompletedNodes, metrics.TotalNodes)
	fmt.Fprintln(out)

	// By State
	fmt.Fprintln(out, "By State:")

	// Order the states consistently
	stateOrder := []string{"pending", "validated", "admitted", "refuted", "archived"}
	for _, state := range stateOrder {
		// Capitalize first letter for display
		displayState := strings.ToUpper(state[:1]) + state[1:]
		fmt.Fprintf(out, "  %-10s %d\n", displayState+":", metrics.ByState[state])
	}
	fmt.Fprintln(out)

	// Blockers
	fmt.Fprintln(out, "Blockers:")
	fmt.Fprintf(out, "  Open challenges: %d\n", metrics.OpenChallenges)
	fmt.Fprintf(out, "  Pending definitions: %d\n", metrics.PendingDefs)
	if metrics.BlockedNodes > 0 {
		fmt.Fprintf(out, "  Blocked nodes: %d\n", metrics.BlockedNodes)
	}
	fmt.Fprintln(out)

	// Critical path
	if len(metrics.CriticalPath) > 0 {
		pathStr := strings.Join(metrics.CriticalPath, " -> ")
		fmt.Fprintf(out, "Critical path: %s (depth %d)\n", pathStr, metrics.CriticalPathDepth)
	} else {
		fmt.Fprintln(out, "Critical path: none (no pending nodes)")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newProgressCmd())
}
