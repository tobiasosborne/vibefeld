// Package main contains the af unadmit command implementation.
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

func newUnadmitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unadmit <node-id>",
		GroupID: GroupVerifier,
		Short:   "Revoke an admission and revert a node to pending",
		Long: `Revoke an admission on a previously admitted node, reverting it to pending state.

Admit is a temporary, taint-introducing escape hatch ("accepted without full
verification"). Once the underlying claim has been rigorously verified — for
example, after a sub-tree was developed and validated — unadmit clears the
admission so the node can be properly accepted with 'af accept'.

Use this when:
- Admit was used to bypass a temporary blocker that has now been resolved
- A rigorous proof has replaced a previously admitted step
- The verifier wants to remove the self_admitted taint from the lineage

Note: Unadmitting a node will recompute taint downward — the self_admitted
source is gone, so descendants that inherited it become taint-unresolved
until re-evaluated.

Examples:
  af unadmit 1.2
  af unadmit 1.2 --reason "Step now rigorously verified by 1.2.{1..4}"
  af unadmit 1.2 --agent verifier-001
  af unadmit 1.2 -f json -y`,
		Args: cobra.ExactArgs(1),
		RunE: runUnadmit,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")
	cmd.Flags().String("reason", "", "Reason for revoking admission")
	cmd.Flags().String("agent", "", "Agent ID (verifier identity)")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}

func runUnadmit(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	reason := cli.MustString(cmd, "reason")
	agent := cli.MustString(cmd, "agent")
	yes := cli.MustBool(cmd, "yes")

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af unadmit")
		return render.InvalidNodeIDError("af unadmit", nodeIDStr, examples)
	}

	msg := fmt.Sprintf("Unadmit node %s? This reverts it to pending and may propagate taint to descendants.", nodeID.String())
	confirmed, err := cli.ConfirmAction(cmd.OutOrStdout(), msg, yes)
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	if err := svc.UnadmitNode(nodeID, reason, agent); err != nil {
		return fmt.Errorf("error unadmitting node: %w", err)
	}

	return outputUnadmitResult(cmd, nodeID, reason, format)
}

func outputUnadmitResult(cmd *cobra.Command, nodeID service.NodeID, reason, format string) error {
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id":        nodeID.String(),
			"previous_state": "admitted",
			"current_state":  "pending",
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
		fmt.Fprintf(cmd.OutOrStdout(), "Admission revoked for node %s\n", nodeID.String())
		fmt.Fprintf(cmd.OutOrStdout(), "  Previous state: admitted\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Current state: pending\n")
		if reason != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Reason: %s\n", reason)
		}
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "Next: A verifier can now 'af accept %s' once children are validated.\n", nodeID.String())
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newUnadmitCmd())
}
