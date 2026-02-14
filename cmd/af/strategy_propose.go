// Package main contains the af strategy-propose command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

var validNoveltyLevels = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
}

func newStrategyProposeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "strategy-propose <node-id>",
		GroupID: GroupProver,
		Short:   "Propose a proof strategy for a node",
		Long: `Record a proposed proof strategy for a node before committing to refinement.

This helps track what strategies have been considered and their novelty
assessment, enabling comparison before choosing a direction.

Novelty levels:
  low     — Applies known theorems directly (may signal wrong approach for novel problems)
  medium  — Combines known techniques in a new way
  high    — Requires genuinely new construction or insight

Examples:
  af strategy-propose 1 --strategy "Induction on n" --novelty low
  af strategy-propose 1 --strategy "Godement-Jacquet bridge" --novelty high --rationale "Avoids case splitting"
  af strategy-propose 1 -s "Direct computation" --agent prover-001 -f json`,
		Args: cobra.ExactArgs(1),
		RunE: runStrategyPropose,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("strategy", "s", "", "Description of the proposed strategy (required)")
	cmd.Flags().String("novelty", "medium", "Novelty assessment (low|medium|high)")
	cmd.Flags().String("rationale", "", "Why this strategy might work")
	cmd.Flags().String("agent", "", "Agent ID (prover identity)")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

func runStrategyPropose(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	strategy := cli.MustString(cmd, "strategy")
	novelty := cli.MustString(cmd, "novelty")
	rationale := cli.MustString(cmd, "rationale")
	agent := cli.MustString(cmd, "agent")
	format := cli.MustString(cmd, "format")

	if strings.TrimSpace(strategy) == "" {
		return render.MissingFlagError("af strategy-propose", "strategy", []string{
			`af strategy-propose 1 --strategy "Induction on n" --novelty low`,
			`af strategy-propose 1 -s "Direct computation" --novelty high`,
		})
	}

	if !validNoveltyLevels[novelty] {
		return fmt.Errorf("invalid novelty level %q: must be low, medium, or high", novelty)
	}

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af strategy-propose")
		return render.InvalidNodeIDError("af strategy-propose", nodeIDStr, examples)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	if err := svc.ProposeStrategy(nodeID, strategy, novelty, rationale, agent); err != nil {
		return fmt.Errorf("error proposing strategy: %w", err)
	}

	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id":  nodeID.String(),
			"strategy": strategy,
			"novelty":  novelty,
		}
		if rationale != "" {
			result["rationale"] = rationale
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Strategy proposed for node %s\n", nodeID.String())
		fmt.Fprintf(cmd.OutOrStdout(), "  Strategy: %s\n", strategy)
		fmt.Fprintf(cmd.OutOrStdout(), "  Novelty: %s\n", novelty)
		if rationale != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Rationale: %s\n", rationale)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newStrategyProposeCmd())
}
