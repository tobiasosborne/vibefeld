// Package main contains the af archive command implementation.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
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
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	reason, err := cmd.Flags().GetString("reason")
	if err != nil {
		return err
	}
	skipConfirm, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return err
	}

	// Handle confirmation for destructive action
	if !skipConfirm {
		// Check if stdin is a terminal (character device, not a pipe or regular file)
		stat, err := os.Stdin.Stat()
		if err != nil {
			return fmt.Errorf("stdin is not a terminal; use --yes flag to confirm archive in non-interactive mode")
		}
		mode := stat.Mode()
		isTerminal := (mode & os.ModeCharDevice) != 0
		isPipe := (mode & os.ModeNamedPipe) != 0

		if !isTerminal || isPipe {
			return fmt.Errorf("stdin is not a terminal; use --yes flag to confirm archive in non-interactive mode")
		}

		// Prompt for confirmation
		fmt.Fprintf(cmd.OutOrStdout(), "Are you sure you want to archive node %s? [y/N]: ", nodeIDStr)
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			// EOF or other error means non-interactive context
			return fmt.Errorf("stdin is not a terminal; use --yes flag to confirm archive in non-interactive mode")
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Archive cancelled.")
			return nil
		}
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
