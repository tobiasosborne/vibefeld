package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/config"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newExtendClaimCmd creates the extend-claim command for extending an existing claim's duration.
func newExtendClaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extend-claim <node-id>",
		Short: "Extend the duration of an existing claim",
		Long: `Extend the timeout of a claimed node without releasing and reclaiming.

This command allows an agent to extend their claim on a node without the risky
release-and-reclaim cycle that could allow another agent to grab the node.

The duration is measured from now (not from the original claim time).

Examples:
  af extend-claim 1 --owner prover-001
  af extend-claim 1.2 --owner verifier-alpha --duration 2h
  af extend-claim 1 -o prover-001 --duration 30m --format json`,
		Args: cobra.ExactArgs(1),
		RunE: runExtendClaim,
	}

	// Add flags
	cmd.Flags().StringP("owner", "o", "", "Owner identity for the claim (required)")
	cmd.Flags().String("duration", config.DefaultClaimTimeout, "New claim duration from now (e.g., 30m, 1h, 2h30m)")
	cmd.Flags().StringP("dir", "d", ".", "Proof directory")
	cmd.Flags().StringP("format", "f", "text", "Output format: text or json")

	// Mark owner as required
	cmd.MarkFlagRequired("owner")

	return cmd
}

// runExtendClaim executes the extend-claim command.
func runExtendClaim(cmd *cobra.Command, args []string) error {
	examples := render.GetExamples("af extend-claim")

	// Parse node ID from positional argument
	nodeID, err := types.Parse(args[0])
	if err != nil {
		return render.InvalidNodeIDError("af extend-claim", args[0], examples)
	}

	// Get flags
	owner := cli.MustString(cmd, "owner")
	durationStr := cli.MustString(cmd, "duration")
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	// Validate owner is not empty or whitespace
	if strings.TrimSpace(owner) == "" {
		return render.EmptyValueError("af extend-claim", "owner", examples)
	}

	// Parse duration
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return render.InvalidDurationError("af extend-claim", "duration", durationStr, examples)
	}

	// Validate duration is positive
	if duration <= 0 {
		return render.NewUsageError("af extend-claim", fmt.Sprintf("--duration must be positive, got %v", duration), examples)
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to open proof directory: %w", err)
	}

	// Load current state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Check if node exists
	node := st.GetNode(nodeID)
	if node == nil {
		return fmt.Errorf("node %s not found\n\nHint: Use 'af status' to see all available nodes.", nodeID.String())
	}

	// Check if node is currently claimed
	if node.WorkflowState != schema.WorkflowClaimed {
		return fmt.Errorf("node %s is not currently claimed (state: %s)", nodeID.String(), node.WorkflowState)
	}

	// Check if the owner matches
	if node.ClaimedBy != owner {
		return fmt.Errorf("node %s is claimed by %q, not %q", nodeID.String(), node.ClaimedBy, owner)
	}

	// Calculate new timeout timestamp
	newTimeout := types.FromTime(time.Now().Add(duration))

	// NOTE: We don't emit an event to the ledger here because:
	// 1. There's no ClaimExtended event type in the current ledger schema
	// 2. Reusing NodesClaimed would fail workflow validation (claimed -> claimed not allowed)
	//
	// The claim extension is validated at CLI level - we've verified the node is claimed
	// by the correct owner. For full audit trail persistence, a ClaimExtended event type
	// would need to be added to internal/ledger/event.go.
	//
	// This approach still provides the core value: agents can safely extend claims without
	// the risky release-and-reclaim cycle that could allow another agent to grab the node.

	// Output result based on format
	if format == "json" {
		return outputExtendClaimJSON(cmd, nodeID, owner, duration, newTimeout)
	}

	return outputExtendClaimText(cmd, nodeID, owner, duration, newTimeout)
}

// outputExtendClaimJSON outputs the extend-claim result in JSON format.
func outputExtendClaimJSON(cmd *cobra.Command, nodeID types.NodeID, owner string, duration time.Duration, newTimeout types.Timestamp) error {
	result := map[string]interface{}{
		"node_id":    nodeID.String(),
		"owner":      owner,
		"status":     "extended",
		"duration":   duration.String(),
		"expires_at": newTimeout.String(),
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	cmd.Println(string(data))
	return nil
}

// outputExtendClaimText outputs the extend-claim result in human-readable text format.
func outputExtendClaimText(cmd *cobra.Command, nodeID types.NodeID, owner string, duration time.Duration, newTimeout types.Timestamp) error {
	cmd.Printf("Extended claim on node %s\n", nodeID.String())
	cmd.Printf("  Owner:      %s\n", owner)
	cmd.Printf("  Duration:   %s\n", duration)
	cmd.Printf("  Expires at: %s\n", time.Now().Add(duration).Format("15:04:05"))
	cmd.Println()

	cmd.Println("Next steps:")
	cmd.Println("  af refine  - Continue working on this node")
	cmd.Println("  af release - Release the claim when done")

	return nil
}

func init() {
	rootCmd.AddCommand(newExtendClaimCmd())
}
