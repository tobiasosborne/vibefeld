// Package main contains the af accept command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

func newAcceptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept <node-id>",
		Short: "Accept a proof node (verifier action)",
		Long: `Accept validates a proof node, marking it as verified correct.

This is a verifier action that confirms the node's correctness.
The node's epistemic state changes from pending to validated.

Examples:
  af accept 1          Accept the root node
  af accept 1.2.3      Accept a specific child node
  af accept 1 -d ./proof  Accept using specific directory`,
		Args: cobra.ExactArgs(1),
		RunE: runAccept,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")

	return cmd
}

func runAccept(cmd *cobra.Command, args []string) error {
	// Parse node ID
	nodeIDStr := args[0]
	nodeID, err := types.Parse(nodeIDStr)
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", nodeIDStr, err)
	}

	// Get flags
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Accept the node
	if err := svc.AcceptNode(nodeID); err != nil {
		return fmt.Errorf("error accepting node: %w", err)
	}

	// Output result based on format
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id":  nodeID.String(),
			"status":   "validated",
			"accepted": true,
		}
		output, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		// Text format
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s accepted and validated.\n", nodeID.String())
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newAcceptCmd())
}
