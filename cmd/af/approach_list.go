// Package main contains the af approach-list command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
)

func newApproachListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "approach-list <node-id>",
		Aliases: []string{"approaches"},
		GroupID: GroupQuery,
		Short:   "List failed proof approaches for a node",
		Long: `Show all attempted but failed proof approaches recorded for a node.

This helps agents understand what has already been tried and avoid
re-attempting dead ends.

Examples:
  af approach-list 1.2
  af approach-list 1.2 -f json`,
		Args: cobra.ExactArgs(1),
		RunE: runApproachList,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

func runApproachList(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af approach-list")
		return render.InvalidNodeIDError("af approach-list", nodeIDStr, examples)
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

	approaches := st.GetFailedApproaches(nodeID)
	return outputApproachList(cmd, nodeID, approaches, format)
}

func outputApproachList(cmd *cobra.Command, nodeID service.NodeID, approaches []state.FailedApproach, format string) error {
	switch strings.ToLower(format) {
	case "json":
		type jsonApproach struct {
			Approach  string `json:"approach"`
			Outcome   string `json:"outcome,omitempty"`
			TriedBy   string `json:"tried_by,omitempty"`
			Timestamp string `json:"timestamp"`
		}
		items := make([]jsonApproach, len(approaches))
		for i, a := range approaches {
			items[i] = jsonApproach{
				Approach:  a.Approach,
				Outcome:   a.Outcome,
				TriedBy:   a.TriedBy,
				Timestamp: a.Timestamp.String(),
			}
		}
		result := map[string]interface{}{
			"node_id":    nodeID.String(),
			"count":      len(approaches),
			"approaches": items,
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		if len(approaches) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No failed approaches recorded for node %s\n", nodeID.String())
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Failed approaches for node %s (%d total):\n\n", nodeID.String(), len(approaches))
		for i, a := range approaches {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, a.Approach)
			if a.Outcome != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Outcome: %s\n", a.Outcome)
			}
			if a.TriedBy != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Tried by: %s\n", a.TriedBy)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "     When: %s\n", a.Timestamp.String())
			if i < len(approaches)-1 {
				fmt.Fprintln(cmd.OutOrStdout())
			}
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newApproachListCmd())
}
