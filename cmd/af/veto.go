// Package main contains the af veto command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/service"
)

func newVetoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "veto <node-id>",
		GroupID: GroupAdmin,
		Short:   "Human expert force-refute a proof node",
		Long: `Veto force-refutes a proof node by human expert authority.

Unlike 'af refute' (which only works on pending nodes within the normal
adversarial workflow), veto can override ANY non-terminal state — including
validated and admitted nodes. This is the "nuclear option" for when a domain
expert identifies a fundamental error.

A --reason is REQUIRED because veto bypasses the adversarial process and
the justification must be recorded in the audit trail.

This is a DESTRUCTIVE action. You will be prompted for confirmation unless
the --yes flag is provided.

Examples:
  af veto 1 --reason "Contradicts known theorem X"
  af veto 1.2.3 --reason "Boundary case disproves claim" --agent "expert-reviewer"
  af veto 1 --reason "Formula error" -y -f json`,
		Args: cobra.ExactArgs(1),
		RunE: runVeto,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")
	cmd.Flags().String("reason", "", "Reason for veto (required)")
	cmd.Flags().String("agent", "", "Identity of the expert who vetoed")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}

func runVeto(cmd *cobra.Command, args []string) error {
	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", nodeIDStr, err)
	}

	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	reason := cli.MustString(cmd, "reason")
	agent := cli.MustString(cmd, "agent")
	skipConfirm := cli.MustBool(cmd, "yes")

	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("--reason is required for veto (must justify expert override)")
	}

	action := fmt.Sprintf("veto (force-refute) node %s", nodeIDStr)
	confirmed, err := cli.ConfirmAction(cmd.OutOrStdout(), action, skipConfirm)
	if err != nil {
		return fmt.Errorf("stdin is not a terminal; use --yes flag to confirm veto in non-interactive mode")
	}
	if !confirmed {
		fmt.Fprintln(cmd.OutOrStdout(), "Veto cancelled.")
		return nil
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	if err := svc.VetoNode(nodeID, reason, agent); err != nil {
		return fmt.Errorf("error vetoing node: %w", err)
	}

	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id": nodeID.String(),
			"status":  "refuted",
			"vetoed":  true,
			"reason":  reason,
		}
		if agent != "" {
			result["vetoed_by"] = agent
		}
		output, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s vetoed (force-refuted).\n", nodeID.String())
		fmt.Fprintf(cmd.OutOrStdout(), "Reason: %s\n", reason)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newVetoCmd())
}
