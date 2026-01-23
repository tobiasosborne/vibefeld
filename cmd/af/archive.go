// Package main contains the af archive command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/service"
)

func newArchiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "archive <node-id>",
		GroupID: GroupEscape,
		Short:   "Archive a proof node (abandon the branch)",
		Long: `Archive marks a proof node as archived, abandoning the branch.

This action indicates the proof branch is no longer being pursued.
The node's epistemic state changes from pending to archived.

Use this when you want to abandon a proof path without marking it as
incorrect. Archived nodes are preserved in the ledger history.

This is a DESTRUCTIVE action. You will be prompted for confirmation unless
the --yes flag is provided. In non-interactive environments (when stdin is
not a terminal), the --yes flag is required.

Examples:
  af archive 1          Archive the root node (will prompt for confirmation)
  af archive 1 -y       Archive without confirmation
  af archive 1.2.3      Archive a specific child node
  af archive 1 -d ./proof  Archive using specific directory
  af archive 1 --reason "Taking different approach"  Archive with explanation`,
		Args: cobra.ExactArgs(1),
		RunE: runArchive,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")
	cmd.Flags().String("reason", "", "Reason for archiving")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}

func runArchive(cmd *cobra.Command, args []string) error {
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
	action := fmt.Sprintf("archive node %s", nodeIDStr)
	confirmed, err := cli.ConfirmAction(cmd.OutOrStdout(), action, skipConfirm)
	if err != nil {
		return fmt.Errorf("stdin is not a terminal; use --yes flag to confirm archive in non-interactive mode")
	}
	if !confirmed {
		fmt.Fprintln(cmd.OutOrStdout(), "Archive cancelled.")
		return nil
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Archive the node
	if err := svc.ArchiveNode(nodeID); err != nil {
		return fmt.Errorf("error archiving node: %w", err)
	}

	// Output result based on format
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id":  nodeID.String(),
			"status":   "archived",
			"archived": true,
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
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s archived.\n", nodeID.String())
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newArchiveCmd())
}
