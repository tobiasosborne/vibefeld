// Package main contains the af strategy-list command for listing proposed strategies.
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

func newStrategyListProposedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "strategy-list <node-id>",
		Aliases: []string{"proposed-strategies"},
		GroupID: GroupQuery,
		Short:   "List proposed proof strategies for a node",
		Long: `Show all proposed proof strategies recorded for a node.

This helps agents compare strategies before committing to a proof direction,
and review what has already been considered.

Examples:
  af strategy-list 1
  af strategy-list 1.2 -f json`,
		Args: cobra.ExactArgs(1),
		RunE: runStrategyListProposed,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

func runStrategyListProposed(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af strategy-list")
		return render.InvalidNodeIDError("af strategy-list", nodeIDStr, examples)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	n := st.GetNode(nodeID)
	if n == nil {
		return fmt.Errorf("node %s not found", nodeID.String())
	}

	strategies := st.GetProposedStrategies(nodeID)

	switch strings.ToLower(format) {
	case "json":
		type jsonStrategy struct {
			Strategy   string `json:"strategy"`
			Novelty    string `json:"novelty"`
			Rationale  string `json:"rationale,omitempty"`
			ProposedBy string `json:"proposed_by,omitempty"`
			Timestamp  string `json:"timestamp"`
		}
		items := make([]jsonStrategy, len(strategies))
		for i, s := range strategies {
			items[i] = jsonStrategy{
				Strategy:   s.Strategy,
				Novelty:    s.Novelty,
				Rationale:  s.Rationale,
				ProposedBy: s.ProposedBy,
				Timestamp:  s.Timestamp.String(),
			}
		}
		result := map[string]interface{}{
			"node_id":    nodeID.String(),
			"count":      len(strategies),
			"strategies": items,
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		if len(strategies) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No strategies proposed for node %s\n", nodeID.String())
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Proposed strategies for node %s (%d total):\n\n", nodeID.String(), len(strategies))
		for i, s := range strategies {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s [novelty: %s]\n", i+1, s.Strategy, s.Novelty)
			if s.Rationale != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Rationale: %s\n", s.Rationale)
			}
			if s.ProposedBy != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Proposed by: %s\n", s.ProposedBy)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "     When: %s\n", s.Timestamp.String())
			if i < len(strategies)-1 {
				fmt.Fprintln(cmd.OutOrStdout())
			}
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newStrategyListProposedCmd())
}
