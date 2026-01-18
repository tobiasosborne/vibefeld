package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newReleaseCmd creates the release command for releasing a claimed job.
func newReleaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release <node-id>",
		Short: "Release a claimed job",
		Long: `Release a claimed job, making it available for other agents.

This command releases a node that you previously claimed, returning it to
the available state so other agents can work on it.

The --owner flag must match the agent that claimed the node.

Examples:
  af release 1 --owner prover-001           Release root node
  af release 1.2.3 -o prover-001            Release using short owner flag
  af release 1 -o prover-001 -d ./proof     Release using specific directory
  af release 1 -o prover-001 -f json        Release with JSON output

Workflow:
  After releasing, the node becomes available for other agents. Use 'af jobs'
  to find other work, or 'af claim' to claim a different node.`,
		Args: cobra.ExactArgs(1),
		RunE: runRelease,
	}

	cmd.Flags().StringP("owner", "o", "", "Agent ID that owns the claim (required)")
	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")

	return cmd
}

// releaseResult represents the result of a release operation for JSON output.
type releaseResult struct {
	NodeID        string `json:"node_id"`
	Status        string `json:"status"`
	WorkflowState string `json:"workflow_state"`
	Message       string `json:"message"`
}

func runRelease(cmd *cobra.Command, args []string) error {
	examples := render.GetExamples("af release")

	// Get flags
	owner := cli.MustString(cmd, "owner")
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	// Validate owner is provided and not empty
	if owner == "" {
		return render.MissingFlagError("af release", "owner", examples)
	}
	if strings.TrimSpace(owner) == "" {
		return render.EmptyValueError("af release", "owner", examples)
	}

	// Parse node ID
	nodeIDStr := args[0]
	nodeID, err := types.Parse(nodeIDStr)
	if err != nil {
		return render.InvalidNodeIDError("af release", nodeIDStr, examples)
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		if strings.Contains(err.Error(), "not exist") {
			return fmt.Errorf("proof not initialized: %w", err)
		}
		return fmt.Errorf("failed to open proof: %w", err)
	}

	// Check if proof is initialized by loading state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("proof not initialized: %w", err)
	}

	// Check if node exists before attempting release
	node := st.GetNode(nodeID)
	if node == nil {
		return fmt.Errorf("node %s not found", nodeID.String())
	}

	// Release the node
	err = svc.ReleaseNode(nodeID, owner)
	if err != nil {
		// Map service errors to user-friendly messages with context
		errStr := err.Error()
		if strings.Contains(errStr, "not found") {
			return fmt.Errorf("node %s not found", nodeID.String())
		}
		if strings.Contains(errStr, "not claimed") {
			return fmt.Errorf("node %s is not claimed (current state: %s)\n\nHint: Only claimed nodes can be released. Use 'af status' to see node states.",
				nodeID.String(), node.WorkflowState)
		}
		if strings.Contains(errStr, "owner") || strings.Contains(errStr, "match") {
			return fmt.Errorf("owner does not match: node %s is claimed by %q, not %q",
				nodeID.String(), node.ClaimedBy, owner)
		}
		return err
	}

	// Output result
	result := releaseResult{
		NodeID:        nodeID.String(),
		Status:        "released",
		WorkflowState: "available",
		Message:       fmt.Sprintf("Node %s released and now available", nodeID.String()),
	}

	if format == "json" {
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s released and now available\n", nodeID.String())
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af claim <node-id> --owner <agent-id>  Claim an available node")
		fmt.Fprintln(cmd.OutOrStdout(), "  af status --dir <path>                 View proof status")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newReleaseCmd())
}
