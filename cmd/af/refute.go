// Package main contains the af refute command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/service"
)

func newRefuteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "refute <node-id>",
		GroupID: GroupEscape,
		Short:   "Refute a proof node (mark as disproven)",
		Long: `Refute marks a proof node as disproven or incorrect.

This is a verifier action that indicates the node's claim is false.
The node's epistemic state changes from pending to refuted.

This is a DESTRUCTIVE action. You will be prompted for confirmation unless
the --yes flag is provided. In non-interactive environments (when stdin is
not a terminal), the --yes flag is required.

Examples:
  af refute 1          Refute the root node (will prompt for confirmation)
  af refute 1 -y       Refute without confirmation
  af refute 1.2.3      Refute a specific child node
  af refute 1 -d ./proof  Refute using specific directory
  af refute 1 --reason "Contradicts theorem 3.2"  Refute with explanation`,
		Args: cobra.ExactArgs(1),
		RunE: runRefute,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")
	cmd.Flags().String("reason", "", "Reason for refutation")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}

func runRefute(cmd *cobra.Command, args []string) error {
	// Parse node ID
	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", nodeIDStr, err)
	}

	// Get flags
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	reason := cli.MustString(cmd, "reason")
	skipConfirm := cli.MustBool(cmd, "yes")

	// Handle confirmation for destructive action
	action := fmt.Sprintf("refute node %s", nodeIDStr)
	confirmed, err := cli.ConfirmAction(cmd.OutOrStdout(), action, skipConfirm)
	if err != nil {
		return fmt.Errorf("stdin is not a terminal; use --yes flag to confirm refute in non-interactive mode")
	}
	if !confirmed {
		fmt.Fprintln(cmd.OutOrStdout(), "Refute cancelled.")
		return nil
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Refute the node
	if err := svc.RefuteNode(nodeID); err != nil {
		return fmt.Errorf("error refuting node: %w", err)
	}

	// Output result based on format
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id": nodeID.String(),
			"status":  "refuted",
			"refuted": true,
		}
		if reason != "" {
			result["reason"] = reason
		}
		output, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		// Text format
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s refuted.\n", nodeID.String())
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newRefuteCmd())
}
