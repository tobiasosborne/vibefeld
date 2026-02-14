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

func newHintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "hint <node-id>",
		GroupID: GroupAdmin,
		Short:   "Add a domain expert hint to a node",
		Long: `Add a directional hint from a domain expert to a proof node.

Hints provide guidance that provers must address — either incorporate the
suggestion or explain why it does not apply. Hints are recorded in the
ledger with full audit trail.

Examples:
  af hint 1.2 --text "Consider the Godement-Jacquet functional equation"
  af hint 1.6 --text "This ratio identity is likely false" --agent expert1
  af hint 1 -t "Use indexing systems instead of families" -f json`,
		Args: cobra.ExactArgs(1),
		RunE: runHint,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("text", "t", "", "Hint text (required)")
	cmd.Flags().StringP("agent", "a", "", "Agent/expert ID providing the hint")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")
	cmd.MarkFlagRequired("text")

	return cmd
}

func newHintsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "hints <node-id>",
		GroupID: GroupQuery,
		Short:   "List domain expert hints for a node",
		Long: `Show all domain expert hints attached to a proof node.

Examples:
  af hints 1.2
  af hints 1.2 -f json`,
		Args: cobra.ExactArgs(1),
		RunE: runHints,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

func runHint(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	text := cli.MustString(cmd, "text")
	agent := cli.MustString(cmd, "agent")
	format := cli.MustString(cmd, "format")

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af hint")
		return render.InvalidNodeIDError("af hint", nodeIDStr, examples)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	if err := svc.AddHint(nodeID, text, agent); err != nil {
		return err
	}

	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id": nodeID.String(),
			"text":    text,
			"hint_by": agent,
			"status":  "added",
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Hint added to node %s\n", nodeID.String())
	}

	return nil
}

func runHints(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af hints")
		return render.InvalidNodeIDError("af hints", nodeIDStr, examples)
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

	hints := st.GetHints(nodeID)
	return outputHintsList(cmd, nodeID, hints, format)
}

func outputHintsList(cmd *cobra.Command, nodeID service.NodeID, hints []state.Hint, format string) error {
	switch strings.ToLower(format) {
	case "json":
		type jsonHint struct {
			Text      string `json:"text"`
			HintBy    string `json:"hint_by,omitempty"`
			Timestamp string `json:"timestamp"`
		}
		items := make([]jsonHint, len(hints))
		for i, h := range hints {
			items[i] = jsonHint{
				Text:      h.Text,
				HintBy:    h.HintBy,
				Timestamp: h.Timestamp.String(),
			}
		}
		result := map[string]interface{}{
			"node_id": nodeID.String(),
			"count":   len(hints),
			"hints":   items,
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		if len(hints) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No hints for node %s\n", nodeID.String())
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Hints for node %s (%d total):\n\n", nodeID.String(), len(hints))
		for i, h := range hints {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, h.Text)
			if h.HintBy != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     From: %s\n", h.HintBy)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "     When: %s\n", h.Timestamp.String())
			if i < len(hints)-1 {
				fmt.Fprintln(cmd.OutOrStdout())
			}
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newHintCmd())
	rootCmd.AddCommand(newHintsCmd())
}
