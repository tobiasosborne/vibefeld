// Package main contains the af request-refinement command implementation.
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

func newRequestRefinementCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "request-refinement <node-id>",
		GroupID: GroupVerifier,
		Short:   "Request more detailed proof for a validated node",
		Long: `Request that a validated node be refined with more detailed child nodes.

This transitions the node from 'validated' to 'needs_refinement' state.
The node will appear in the prover jobs list until it is refined and
re-validated.

Use this when:
- A proof step is correct but could be more detailed
- You want to break down a complex step for clarity
- A reviewer requests deeper justification

Examples:
  af request-refinement 1.2
  af request-refinement 1.2 --reason "Need explicit algebra steps"
  af request-refinement 1.2 --agent verifier-001
  af request-refinement 1.2 -f json

Workflow:
  After requesting refinement, a prover should add child nodes with
  'af refine 1.2 ...' and then a verifier can re-validate with 'af accept'.`,
		Args: cobra.ExactArgs(1),
		RunE: runRequestRefinement,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")
	cmd.Flags().String("reason", "", "Reason for requesting refinement")
	cmd.Flags().String("agent", "", "Agent ID (verifier identity)")

	return cmd
}

func runRequestRefinement(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	reason := cli.MustString(cmd, "reason")
	agent := cli.MustString(cmd, "agent")

	// Parse and validate node ID
	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af request-refinement")
		return render.InvalidNodeIDError("af request-refinement", nodeIDStr, examples)
	}

	// Create service and request refinement
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	if err := svc.RequestRefinement(nodeID, reason, agent); err != nil {
		return fmt.Errorf("error requesting refinement: %w", err)
	}

	// Output result
	return outputRequestRefinementResult(cmd, nodeID, reason, format)
}

func outputRequestRefinementResult(cmd *cobra.Command, nodeID service.NodeID, reason, format string) error {
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id":        nodeID.String(),
			"previous_state": "validated",
			"current_state":  "needs_refinement",
		}
		if reason != "" {
			result["reason"] = reason
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Refinement requested for node %s\n", nodeID.String())
		fmt.Fprintf(cmd.OutOrStdout(), "  Previous state: validated\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Current state: needs_refinement\n")
		if reason != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Reason: %s\n", reason)
		}
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "Next: A prover should add child nodes with 'af refine %s ...'\n", nodeID.String())
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newRequestRefinementCmd())
}
