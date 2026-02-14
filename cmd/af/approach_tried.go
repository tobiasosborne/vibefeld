// Package main contains the af approach-tried command implementation.
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

func newApproachTriedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "approach-tried <node-id>",
		GroupID: GroupProver,
		Short:   "Record a failed proof approach",
		Long: `Record an attempted but failed proof approach for a node.

This helps track what approaches have been tried and why they failed,
preventing other agents from re-attempting the same dead ends.

Examples:
  af approach-tried 1.2 --approach "Induction on n" --outcome "Base case fails for n=0"
  af approach-tried 1.2 -a "By contradiction" -o "Leads to unfounded assumption" --agent prover-001
  af approach-tried 1.2 -a "Direct computation" -f json`,
		Args: cobra.ExactArgs(1),
		RunE: runApproachTried,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("approach", "a", "", "Description of the attempted approach (required)")
	cmd.Flags().StringP("outcome", "o", "", "Why the approach failed")
	cmd.Flags().String("agent", "", "Agent ID (prover identity)")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

func runApproachTried(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	approach := cli.MustString(cmd, "approach")
	outcome := cli.MustString(cmd, "outcome")
	agent := cli.MustString(cmd, "agent")
	format := cli.MustString(cmd, "format")

	if strings.TrimSpace(approach) == "" {
		return render.MissingFlagError("af approach-tried", "approach", []string{
			`af approach-tried 1.2 --approach "Induction on n"`,
			`af approach-tried 1.2 -a "By contradiction" -o "Leads to paradox"`,
		})
	}

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af approach-tried")
		return render.InvalidNodeIDError("af approach-tried", nodeIDStr, examples)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	if err := svc.RecordApproachTried(nodeID, approach, outcome, agent); err != nil {
		return fmt.Errorf("error recording approach: %w", err)
	}

	return outputApproachTriedResult(cmd, nodeID, approach, outcome, format)
}

func outputApproachTriedResult(cmd *cobra.Command, nodeID service.NodeID, approach, outcome, format string) error {
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id":  nodeID.String(),
			"approach": approach,
		}
		if outcome != "" {
			result["outcome"] = outcome
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Failed approach recorded for node %s\n", nodeID.String())
		fmt.Fprintf(cmd.OutOrStdout(), "  Approach: %s\n", approach)
		if outcome != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Outcome: %s\n", outcome)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newApproachTriedCmd())
}
