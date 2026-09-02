package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

func newRepairStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repair-stats [node-id]",
		GroupID: GroupSetup,
		Short:   "Show repair fatigue metrics for a node or subtree",
		Long: `Analyze repair fatigue by showing challenge, amendment, and refutation metrics.

Without a node ID, scans the entire proof for fatigued subtrees.
With a node ID, shows detailed metrics for that node and its subtree.

Repair cycles = resolved challenges + amendments (proxy for rework effort).
Warning threshold (default 3): subtree may have deeper issues.
Alarm threshold (default 5): repeated repairs suggest the claim or root conjecture may be false.

Examples:
  af repair-stats              Scan entire proof for fatigue
  af repair-stats 1.3          Show metrics for node 1.3 and its subtree
  af repair-stats --format json  JSON output`,
		RunE: runRepairStats,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().Int("warning", state.DefaultRepairWarningThreshold, "Warning threshold for repair cycles")
	cmd.Flags().Int("alarm", state.DefaultRepairAlarmThreshold, "Alarm threshold for repair cycles")

	return cmd
}

func runRepairStats(cmd *cobra.Command, args []string) error {
	dir := service.MustString(cmd, "dir")
	format := strings.ToLower(service.MustString(cmd, "format"))
	warnThresh, _ := cmd.Flags().GetInt("warning")
	alarmThresh, _ := cmd.Flags().GetInt("alarm")

	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	if len(st.AllNodes()) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No proof initialized.")
		return nil
	}

	// Specific node mode
	if len(args) > 0 {
		nodeID, err := types.Parse(args[0])
		if err != nil {
			return fmt.Errorf("invalid node ID %q: %w", args[0], err)
		}
		if st.GetNode(nodeID) == nil {
			return fmt.Errorf("node %s not found", args[0])
		}
		return renderNodeRepairStats(cmd, st, nodeID, warnThresh, alarmThresh, format)
	}

	// Scan mode: find all fatigued subtrees
	return renderScanRepairStats(cmd, st, warnThresh, alarmThresh, format)
}

func renderNodeRepairStats(cmd *cobra.Command, st *service.State, nodeID types.NodeID, warnThresh, alarmThresh int, format string) error {
	nodeMetrics := st.GetRepairMetrics(nodeID)
	subtreeMetrics := st.GetSubtreeRepairMetrics(nodeID)
	level := state.ClassifyFatigue(subtreeMetrics.RepairCycles, warnThresh, alarmThresh)

	if format == "json" {
		output := map[string]interface{}{
			"node":    nodeMetrics,
			"subtree": subtreeMetrics,
			"fatigue_level": map[string]interface{}{
				"level":             fatigueLabel(level),
				"warning_threshold": warnThresh,
				"alarm_threshold":   alarmThresh,
			},
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Repair Stats for Node %s\n", nodeID.String()))
	sb.WriteString(strings.Repeat("=", 40) + "\n\n")

	sb.WriteString("Node Metrics:\n")
	sb.WriteString(fmt.Sprintf("  Challenges:    %d total (%d open, %d resolved, %d withdrawn)\n",
		nodeMetrics.ChallengeCount, nodeMetrics.OpenChallengeCount, nodeMetrics.ResolvedCount, nodeMetrics.WithdrawnCount))
	sb.WriteString(fmt.Sprintf("  Amendments:    %d\n", nodeMetrics.AmendmentCount))
	sb.WriteString("\n")

	sb.WriteString("Subtree Metrics:\n")
	sb.WriteString(fmt.Sprintf("  Nodes:         %d\n", subtreeMetrics.NodeCount))
	sb.WriteString(fmt.Sprintf("  Challenges:    %d total (%d open)\n", subtreeMetrics.TotalChallenges, subtreeMetrics.TotalOpenChallenges))
	sb.WriteString(fmt.Sprintf("  Amendments:    %d\n", subtreeMetrics.TotalAmendments))
	sb.WriteString(fmt.Sprintf("  Resolved:      %d\n", subtreeMetrics.TotalResolved))
	sb.WriteString(fmt.Sprintf("  Repair cycles: %d\n", subtreeMetrics.RepairCycles))
	sb.WriteString(fmt.Sprintf("  Refuted:       %d\n", subtreeMetrics.RefutedDescendants))
	sb.WriteString(fmt.Sprintf("  Archived:      %d\n", subtreeMetrics.ArchivedDescendants))
	if subtreeMetrics.HighestNodeID != "" {
		sb.WriteString(fmt.Sprintf("  Most challenged: %s (%d challenges)\n",
			subtreeMetrics.HighestNodeID, subtreeMetrics.HighestNodeChallenges))
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("Fatigue Level: %s\n", fatigueLabel(level)))
	if level == state.FatigueAlarm {
		sb.WriteString("  ALARM: Repeated repairs may indicate the parent claim or root conjecture is false.\n")
		sb.WriteString("  Consider re-examining foundational assumptions.\n")
	} else if level == state.FatigueWarning {
		sb.WriteString("  WARNING: Subtree shows signs of chronic instability.\n")
	}

	fmt.Fprint(cmd.OutOrStdout(), sb.String())
	return nil
}

func renderScanRepairStats(cmd *cobra.Command, st *service.State, warnThresh, alarmThresh int, format string) error {
	fatigued := st.FindFatiguedSubtrees(warnThresh, alarmThresh)

	if format == "json" {
		output := map[string]interface{}{
			"fatigued_subtrees": fatigued,
			"warning_threshold": warnThresh,
			"alarm_threshold":   alarmThresh,
			"count":             len(fatigued),
		}
		if fatigued == nil {
			output["fatigued_subtrees"] = []state.FatiguedSubtree{}
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	var sb strings.Builder
	sb.WriteString("Repair Fatigue Scan\n")
	sb.WriteString(strings.Repeat("=", 40) + "\n\n")

	if len(fatigued) == 0 {
		sb.WriteString("No subtrees show repair fatigue.\n")
		sb.WriteString(fmt.Sprintf("(thresholds: warning=%d, alarm=%d repair cycles)\n", warnThresh, alarmThresh))
		fmt.Fprint(cmd.OutOrStdout(), sb.String())
		return nil
	}

	sb.WriteString(fmt.Sprintf("Found %d fatigued subtree(s):\n\n", len(fatigued)))

	for _, f := range fatigued {
		label := fatigueLabel(f.Level)
		sb.WriteString(fmt.Sprintf("  %s [%s] %d repair cycles (%d challenges, %d amendments, %d refuted)\n",
			f.NodeID, label, f.RepairCycles,
			f.Metrics.TotalChallenges, f.Metrics.TotalAmendments, f.Metrics.RefutedDescendants))
	}

	sb.WriteString(fmt.Sprintf("\n(thresholds: warning=%d, alarm=%d repair cycles)\n", warnThresh, alarmThresh))

	fmt.Fprint(cmd.OutOrStdout(), sb.String())
	return nil
}

func fatigueLabel(level state.FatigueLevel) string {
	switch level {
	case state.FatigueAlarm:
		return "ALARM"
	case state.FatigueWarning:
		return "WARNING"
	default:
		return "OK"
	}
}

func init() {
	rootCmd.AddCommand(newRepairStatsCmd())
}
