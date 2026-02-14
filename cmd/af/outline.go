package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/service"
)

func newOutlineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "outline",
		GroupID: GroupSetup,
		Short:   "Show proof outline and per-stage coverage",
		Long: `Show the current proof outline with coverage metrics for each stage.

If no outline has been set, suggests using 'af outline set'.

Use subcommands to define or modify the outline:
  af outline set   Define the proof skeleton
  af outline map   Link a stage to a proof node`,
		RunE: runOutlineShow,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	cmd.AddCommand(newOutlineSetCmd())
	cmd.AddCommand(newOutlineMapCmd())

	return cmd
}

func newOutlineSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Record a high-level proof skeleton",
		Long: `Define the proof outline as a sequence of named stages.

Each stage has a label, criticality level, and description.
Format: --stage "LABEL:CRITICALITY:DESCRIPTION"

Criticality levels:
  critical  - Must be completed; health warnings if untouched
  important - Should be completed for a rigorous proof
  routine   - Nice to have but not essential

This replaces any previously set outline.

Examples:
  af outline set \
    --stage "A:critical:Establish base case" \
    --stage "B:critical:Prove inductive step" \
    --stage "C:important:Handle edge cases" \
    --stage "D:critical:Derive conclusion"`,
		RunE: runOutlineSet,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringArrayP("stage", "s", nil, `Stage spec: "LABEL:CRITICALITY:DESCRIPTION"`)
	cmd.Flags().String("agent", "", "Agent ID setting the outline")
	_ = cmd.MarkFlagRequired("stage")

	return cmd
}

func newOutlineMapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "map <label> <node-id>",
		Short: "Link an outline stage to a subtree root node",
		Long: `Map an outline stage label to a node in the proof tree.

The specified node becomes the root of the subtree associated with that stage.
Coverage metrics will be computed over all nodes in that subtree.

Examples:
  af outline map A 1.1
  af outline map D 1.4`,
		Args: cobra.ExactArgs(2),
		RunE: runOutlineMap,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")

	return cmd
}

func runOutlineShow(cmd *cobra.Command, args []string) error {
	dir := service.MustString(cmd, "dir")
	format := service.MustString(cmd, "format")

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	if !st.HasOutline() {
		fmt.Fprintln(cmd.OutOrStdout(), "No outline set. Use 'af outline set' to define proof stages.")
		return nil
	}

	report := st.GetOutlineCoverage()

	if strings.ToLower(format) == "json" {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("error encoding JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
		return nil
	}

	// Text format
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Proof Outline (%d stages)\n", report.StagesTotal)
	fmt.Fprintln(out, strings.Repeat("=", 50))
	fmt.Fprintf(out, "Mapped: %d  Started: %d  Complete: %d\n\n", report.StagesMapped, report.StagesStarted, report.StagesComplete)

	for _, s := range report.Stages {
		prefix := " "
		if s.Criticality == "critical" && !s.Started {
			prefix = "!"
		}
		coverage := "unmapped"
		if s.Mapped {
			if s.TotalNodes == 0 {
				coverage = fmt.Sprintf("%s — 0 nodes", s.NodeID)
			} else {
				coverage = fmt.Sprintf("%s — %d/%d validated (%.0f%%)", s.NodeID, s.Validated, s.TotalNodes, s.Fraction*100)
			}
		}
		fmt.Fprintf(out, "  [%s] %s (%s) — %s — %s\n", prefix, s.Label, s.Criticality, coverage, s.Description)
	}

	if len(report.CriticalUntouched) > 0 {
		fmt.Fprintf(out, "\nWarning: %d critical stage(s) untouched: %s\n", len(report.CriticalUntouched), strings.Join(report.CriticalUntouched, ", "))
	}

	return nil
}

func runOutlineSet(cmd *cobra.Command, args []string) error {
	dir := service.MustString(cmd, "dir")
	stageSpecs, _ := cmd.Flags().GetStringArray("stage")
	agent, _ := cmd.Flags().GetString("agent")

	stages := make([]ledger.OutlineStage, 0, len(stageSpecs))
	for _, spec := range stageSpecs {
		stage, err := parseStageSpec(spec)
		if err != nil {
			return err
		}
		stages = append(stages, stage)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	if err := svc.SetOutline(stages, agent); err != nil {
		return fmt.Errorf("error setting outline: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Outline set with %d stages\n", len(stages))
	return nil
}

func runOutlineMap(cmd *cobra.Command, args []string) error {
	dir := service.MustString(cmd, "dir")
	label := args[0]
	nodeIDStr := args[1]

	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", nodeIDStr, err)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	if err := svc.LinkOutlineStage(label, nodeID); err != nil {
		return fmt.Errorf("error mapping stage: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Stage %q mapped to node %s\n", label, nodeIDStr)
	return nil
}

// parseStageSpec parses "LABEL:CRITICALITY:DESCRIPTION" — splits on first two colons.
func parseStageSpec(spec string) (ledger.OutlineStage, error) {
	parts := strings.SplitN(spec, ":", 3)
	if len(parts) < 3 {
		return ledger.OutlineStage{}, fmt.Errorf("invalid stage spec %q: expected LABEL:CRITICALITY:DESCRIPTION", spec)
	}
	return ledger.OutlineStage{
		Label:       strings.TrimSpace(parts[0]),
		Criticality: strings.TrimSpace(parts[1]),
		Description: strings.TrimSpace(parts[2]),
	}, nil
}

func init() {
	rootCmd.AddCommand(newOutlineCmd())
}
