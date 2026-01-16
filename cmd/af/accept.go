// Package main contains the af accept command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

func newAcceptCmd() *cobra.Command {
	var acceptAll bool
	var withNote string
	var confirm bool

	cmd := &cobra.Command{
		Use:   "accept [node-id]...",
		Short: "Accept proof nodes (verifier action)",
		Long: `Accept validates proof nodes, marking them as verified correct.

This is a verifier action that confirms the node's correctness.
The node's epistemic state changes from pending to validated.

You can accept multiple nodes at once:
  af accept 1.1 1.2 1.3    Accept nodes 1.1, 1.2, and 1.3

Use --all to accept all pending nodes:
  af accept --all          Accept all pending nodes

Use --with-note for partial acceptance (accept with a recorded note):
  af accept 1 --with-note "Minor issue but acceptable"

Notes are recorded in the ledger for the audit trail but do not
block acceptance. This allows verifiers to express nuanced feedback.

Examples:
  af accept 1              Accept the root node
  af accept 1.2.3          Accept a specific child node
  af accept 1.1 1.2        Accept multiple nodes at once
  af accept --all          Accept all pending nodes
  af accept -a             Accept all pending nodes (short form)
  af accept 1 --with-note "Consider clarifying step 2"
  af accept 1 -d ./proof   Accept using specific directory`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAccept(cmd, args, acceptAll, withNote)
		},
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")
	cmd.Flags().BoolVarP(&acceptAll, "all", "a", false, "Accept all pending nodes")
	cmd.Flags().StringVar(&withNote, "with-note", "", "Optional acceptance note for partial acceptance")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm acceptance without challenges")

	return cmd
}

func runAccept(cmd *cobra.Command, args []string, acceptAll bool, withNote string) error {
	examples := render.GetExamples("af accept")

	// Get flags
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	// Validate input: either --all or node IDs, but not both (or neither)
	hasNodeIDs := len(args) > 0

	if acceptAll && hasNodeIDs {
		return render.NewUsageError("af accept",
			"--all and node IDs are mutually exclusive; use one or the other",
			[]string{"af accept --all", "af accept 1.1 1.2 1.3"})
	}

	if !acceptAll && !hasNodeIDs {
		return render.NewUsageError("af accept",
			"either specify node IDs or use --all to accept all pending nodes",
			[]string{"af accept 1.1", "af accept 1.1 1.2 1.3", "af accept --all"})
	}

	// --with-note is only valid for single node acceptance
	if withNote != "" && (acceptAll || len(args) > 1) {
		return render.NewUsageError("af accept",
			"--with-note can only be used when accepting a single node",
			[]string{"af accept 1 --with-note \"Minor issue but acceptable\""})
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	var nodeIDs []types.NodeID

	if acceptAll {
		// Get all pending nodes
		pendingNodes, err := svc.GetPendingNodes()
		if err != nil {
			return fmt.Errorf("error getting pending nodes: %w", err)
		}

		if len(pendingNodes) == 0 {
			// No pending nodes to accept
			switch strings.ToLower(format) {
			case "json":
				result := map[string]interface{}{
					"accepted": []string{},
					"message":  "no pending nodes to accept",
				}
				output, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("error marshaling JSON: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(output))
			default:
				fmt.Fprintln(cmd.OutOrStdout(), "No pending nodes to accept.")
			}
			return nil
		}

		// Extract node IDs from pending nodes
		for _, n := range pendingNodes {
			nodeIDs = append(nodeIDs, n.ID)
		}
	} else {
		// Parse all provided node IDs
		for _, nodeIDStr := range args {
			nodeID, err := types.Parse(nodeIDStr)
			if err != nil {
				return render.InvalidNodeIDError("af accept", nodeIDStr, examples)
			}
			nodeIDs = append(nodeIDs, nodeID)
		}
	}

	// Single node: use AcceptNodeWithNote (or AcceptNode if no note)
	if len(nodeIDs) == 1 {
		var acceptErr error
		if withNote != "" {
			acceptErr = svc.AcceptNodeWithNote(nodeIDs[0], withNote)
		} else {
			acceptErr = svc.AcceptNode(nodeIDs[0])
		}
		if acceptErr != nil {
			return fmt.Errorf("error accepting node: %w", acceptErr)
		}

		// Output result based on format
		switch strings.ToLower(format) {
		case "json":
			result := map[string]interface{}{
				"node_id":  nodeIDs[0].String(),
				"status":   "validated",
				"accepted": true,
			}
			if withNote != "" {
				result["note"] = withNote
			}
			output, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("error marshaling JSON: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(output))
		default:
			if withNote != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Node %s accepted and validated (with note: %q).\n", nodeIDs[0].String(), withNote)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Node %s accepted and validated.\n", nodeIDs[0].String())
			}
		}
		return nil
	}

	// Multiple nodes: use AcceptNodeBulk
	if err := svc.AcceptNodeBulk(nodeIDs); err != nil {
		return fmt.Errorf("error accepting nodes: %w", err)
	}

	// Build list of accepted node ID strings
	acceptedStrs := make([]string, len(nodeIDs))
	for i, id := range nodeIDs {
		acceptedStrs[i] = id.String()
	}

	// Output result based on format
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"accepted": acceptedStrs,
			"count":    len(nodeIDs),
			"status":   "validated",
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Accepted %d nodes:\n", len(nodeIDs))
		for _, idStr := range acceptedStrs {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s - validated\n", idStr)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newAcceptCmd())
}
