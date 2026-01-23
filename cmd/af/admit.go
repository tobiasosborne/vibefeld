// Package main contains the af admit command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/service"
)

func newAdmitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "admit <node-id>",
		GroupID: GroupEscape,
		Short:   "Admit a proof node without full verification (introduces taint)",
		Long: `Admit accepts a proof node without full verification.

This action introduces epistemic taint, marking uncertainty about the node's
correctness. The node's epistemic state changes from pending to admitted.

Use this when you want to accept a claim provisionally without rigorous
verification. Any nodes that depend on an admitted node will inherit taint.

Examples:
  af admit 1          Admit the root node
  af admit 1.2.3      Admit a specific child node
  af admit 1 -d ./proof  Admit using specific directory

Workflow:
  After admitting, use 'af status' to see the taint propagation. Consider
  returning later to properly verify the node with 'af accept'.`,
		Args: cobra.ExactArgs(1),
		RunE: runAdmit,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")

	return cmd
}

func runAdmit(cmd *cobra.Command, args []string) error {
	// Parse node ID
	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", nodeIDStr, err)
	}

	// Get flags
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Admit the node
	if err := svc.AdmitNode(nodeID); err != nil {
		return fmt.Errorf("error admitting node: %w", err)
	}

	// Output result based on format
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id":  nodeID.String(),
			"status":   "admitted",
			"admitted": true,
		}
		output, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		// Text format
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s admitted.\n", nodeID.String())
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newAdmitCmd())
}
