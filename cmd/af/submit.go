package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

func newSubmitCmd() *cobra.Command {
	var owner string
	var dir string
	var format string

	cmd := &cobra.Command{
		Use:     "submit <node-id>",
		GroupID: GroupProver,
		Short:   "Submit a draft node for verification",
		Long: `Submit a draft node for formal verification.

This transitions a node from draft to pending state, making it visible
to verifiers. Any existing challenges on the node become active.

Only nodes in draft state can be submitted.

Examples:
  af submit 1.1 --owner agent1
  af submit 1.2 -o agent1 --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSubmit(cmd, args[0], owner, dir, format)
		},
	}

	cmd.Flags().StringVarP(&owner, "owner", "o", "", "Agent/owner name (required)")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text/json)")

	return cmd
}

func runSubmit(cmd *cobra.Command, nodeIDStr, owner, dir, format string) error {
	examples := []string{
		"af submit 1.1 --owner agent1",
	}

	if strings.TrimSpace(owner) == "" {
		return render.MissingFlagError("af submit", "owner", examples)
	}

	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return render.InvalidNodeIDError("af submit", nodeIDStr, examples)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to open proof: %w", err)
	}

	if err := svc.SubmitNode(nodeID, owner); err != nil {
		return err
	}

	if format == "json" {
		cmd.Printf(`{"success":true,"node_id":%q,"previous_state":"draft","new_state":"pending"}`, nodeIDStr)
		cmd.Println()
		return nil
	}

	cmd.Printf("Node %s submitted for verification.\n", nodeIDStr)
	cmd.Printf("  State: draft → pending\n")
	cmd.Println("\nNext steps:")
	cmd.Printf("  af status           - View proof status\n")
	cmd.Printf("  af challenge %s   - Challenge this node\n", nodeIDStr)
	cmd.Printf("  af accept %s      - Accept this node\n", nodeIDStr)

	return nil
}

func init() {
	rootCmd.AddCommand(newSubmitCmd())
}
