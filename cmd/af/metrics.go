package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/metrics"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newMetricsCmd creates the metrics command.
func newMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "metrics",
		GroupID: GroupUtil,
		Short:   "Show proof quality metrics and reports",
		Long: `Analyze the proof and display quality metrics.

The metrics command calculates:
  - Refinement depth: Maximum depth of the proof tree
  - Challenge density: Number of challenges per node
  - Definition coverage: Percentage of referenced terms with definitions
  - Quality score: Composite score (0-100) based on all metrics

Use --node to focus metrics on a specific subtree.

Examples:
  af metrics                     Show metrics for entire proof
  af metrics --dir /path/to/proof  Show metrics for specific proof
  af metrics --node 1.2           Show metrics for subtree rooted at node 1.2
  af metrics --format json        Output in JSON format`,
		RunE: runMetrics,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().StringP("node", "n", "", "Node ID to focus metrics on (subtree)")

	return cmd
}

// runMetrics executes the metrics command.
func runMetrics(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	nodeIDStr, _ := cmd.Flags().GetString("node")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Parse node ID if provided
	var nodeID *types.NodeID
	if nodeIDStr != "" {
		id, err := types.Parse(nodeIDStr)
		if err != nil {
			return fmt.Errorf("invalid node ID %q: %w", nodeIDStr, err)
		}
		nodeID = &id
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

	// Calculate metrics
	var report *metrics.QualityReport
	if nodeID != nil {
		report = metrics.SubtreeQuality(st, *nodeID)
		// Check if the node exists
		if st.GetNode(*nodeID) == nil {
			if format == "json" {
				fmt.Fprintln(cmd.OutOrStdout(), `{"error":"node not found"}`)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Node %s not found.\n", nodeIDStr)
			return nil
		}
	} else {
		report = metrics.OverallQuality(st)
	}

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
	output := renderMetricsText(report, nodeIDStr)
	fmt.Fprint(cmd.OutOrStdout(), output)

	return nil
}

// renderMetricsText renders the quality report as text.
func renderMetricsText(report *metrics.QualityReport, nodeID string) string {
	var sb strings.Builder

	// Header
	if nodeID != "" {
		sb.WriteString(fmt.Sprintf("Quality Metrics for Node %s\n", nodeID))
	} else {
		sb.WriteString("Quality Metrics\n")
	}
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")

	// Node Statistics
	sb.WriteString("Node Statistics:\n")
	sb.WriteString(fmt.Sprintf("  Total nodes:      %d\n", report.NodeCount))
	sb.WriteString(fmt.Sprintf("  Validated:        %d\n", report.ValidatedNodes))
	sb.WriteString(fmt.Sprintf("  Pending:          %d\n", report.PendingNodes))
	sb.WriteString(fmt.Sprintf("  Admitted:         %d\n", report.AdmittedNodes))
	sb.WriteString(fmt.Sprintf("  Refuted:          %d\n", report.RefutedNodes))
	sb.WriteString(fmt.Sprintf("  Archived:         %d\n", report.ArchivedNodes))
	sb.WriteString("\n")

	// Depth Metrics
	sb.WriteString("Depth:\n")
	sb.WriteString(fmt.Sprintf("  Max depth:        %d\n", report.MaxDepth))
	sb.WriteString("\n")

	// Challenge Metrics
	sb.WriteString("Challenges:\n")
	sb.WriteString(fmt.Sprintf("  Total:            %d\n", report.TotalChallenges))
	sb.WriteString(fmt.Sprintf("  Open:             %d\n", report.OpenChallenges))
	sb.WriteString(fmt.Sprintf("  Resolved:         %d\n", report.ResolvedChallenges))
	sb.WriteString(fmt.Sprintf("  Challenge density: %.2f per node\n", report.ChallengeDensity))
	sb.WriteString("\n")

	// Definition Coverage
	sb.WriteString("Definition Coverage:\n")
	sb.WriteString(fmt.Sprintf("  Referenced terms: %d\n", report.DefinitionRefs))
	sb.WriteString(fmt.Sprintf("  Defined terms:    %d\n", report.DefinedRefs))
	sb.WriteString(fmt.Sprintf("  Coverage:         %.1f%%\n", report.DefinitionCoverage*100))
	sb.WriteString("\n")

	// Quality Score
	sb.WriteString("Quality Score:\n")
	scoreIcon := getScoreIcon(report.QualityScore)
	sb.WriteString(fmt.Sprintf("  Overall score:    %.1f/100 %s\n", report.QualityScore, scoreIcon))
	sb.WriteString("\n")

	// Interpretation
	sb.WriteString("Score Interpretation:\n")
	sb.WriteString(getScoreInterpretation(report.QualityScore))
	sb.WriteString("\n")

	// Next Steps
	sb.WriteString("Next Steps:\n")
	sb.WriteString(getMetricsNextSteps(report))

	return sb.String()
}

// getScoreIcon returns an icon based on the quality score.
func getScoreIcon(score float64) string {
	switch {
	case score >= 90:
		return "[EXCELLENT]"
	case score >= 70:
		return "[GOOD]"
	case score >= 50:
		return "[FAIR]"
	default:
		return "[NEEDS WORK]"
	}
}

// getScoreInterpretation returns a text interpretation of the quality score.
func getScoreInterpretation(score float64) string {
	switch {
	case score >= 90:
		return "  Excellent: Proof is well-validated with minimal issues.\n"
	case score >= 70:
		return "  Good: Proof is progressing well but has room for improvement.\n"
	case score >= 50:
		return "  Fair: Proof has some issues that should be addressed.\n"
	default:
		return "  Needs Work: Proof has significant issues requiring attention.\n"
	}
}

// getMetricsNextSteps returns suggested next steps based on the report.
func getMetricsNextSteps(report *metrics.QualityReport) string {
	var sb strings.Builder

	if report.NodeCount == 0 {
		sb.WriteString("  - Run 'af init' to start a new proof\n")
		return sb.String()
	}

	// Check for open challenges
	if report.OpenChallenges > 0 {
		sb.WriteString(fmt.Sprintf("  - Address %d open challenge(s)\n", report.OpenChallenges))
	}

	// Check for validation progress
	if report.PendingNodes > 0 {
		sb.WriteString(fmt.Sprintf("  - Review and validate %d pending node(s)\n", report.PendingNodes))
	}

	// Check for definition coverage
	if report.DefinitionCoverage < 1.0 && report.DefinitionRefs > 0 {
		missing := report.DefinitionRefs - report.DefinedRefs
		sb.WriteString(fmt.Sprintf("  - Add definitions for %d undefined term(s)\n", missing))
	}

	// If everything looks good
	if sb.Len() == 0 {
		sb.WriteString("  - Proof is in good shape! Continue refining or review with 'af status'\n")
	}

	return sb.String()
}

func init() {
	rootCmd.AddCommand(newMetricsCmd())
}
