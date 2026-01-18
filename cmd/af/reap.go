// Package main contains the af reap command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// reapResult holds the result of a reap operation for output.
type reapResult struct {
	DryRun  bool     `json:"dry_run"`
	Count   int      `json:"count"`
	Reaped  []string `json:"reaped"`
	Message string   `json:"message,omitempty"`
}

func newReapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reap",
		GroupID: GroupAdmin,
		Short:   "Clean up stale/expired locks",
		Long: `Reap cleans up stale or expired locks from claimed nodes.

When an agent claims a node, it acquires a time-limited lock. If the agent
fails to complete its work before the lock expires, the node becomes stale.
The reap command identifies these stale locks and releases them, making
the nodes available for other agents to claim.

By default, only expired locks are reaped. Use --all to reap all locks
regardless of expiration time.

Use --dry-run to preview what would be reaped without actually making changes.

Examples:
  af reap                    Reap expired locks in current directory
  af reap --dry-run          Preview what would be reaped
  af reap --all              Reap all locks regardless of expiration
  af reap -d ./proof         Reap in specific proof directory
  af reap -f json            Output results in JSON format`,
		RunE: runReap,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")
	cmd.Flags().Bool("dry-run", false, "Preview what would be reaped without making changes")
	cmd.Flags().Bool("all", false, "Reap all locks regardless of expiration")

	return cmd
}

func runReap(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return err
	}
	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Check if proof is initialized
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Load current state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Find nodes to reap
	now := types.FromTime(time.Now())
	var toReap []types.NodeID

	for _, n := range st.AllNodes() {
		if n.WorkflowState != schema.WorkflowClaimed {
			continue
		}

		// Check if lock is expired or if --all flag is set
		if all {
			toReap = append(toReap, n.ID)
		} else {
			// Check if the claim has expired
			// ClaimedAt stores the timeout timestamp (when the claim expires)
			if n.ClaimedAt.Before(now) {
				toReap = append(toReap, n.ID)
			}
		}
	}

	// Build result
	result := reapResult{
		DryRun: dryRun,
		Count:  len(toReap),
		Reaped: types.ToStringSlice(toReap),
	}

	// If not dry run, actually release the nodes
	if !dryRun && len(toReap) > 0 {
		if err := releaseNodes(svc, toReap); err != nil {
			return fmt.Errorf("error releasing nodes: %w", err)
		}
	}

	// Output result based on format
	return outputReapResult(cmd, result, format, dryRun)
}

// releaseNodes releases the given nodes by appending NodesReleased events.
func releaseNodes(svc *service.ProofService, nodeIDs []types.NodeID) error {
	// Get the ledger directly
	ldg, err := ledger.NewLedger(svc.Path() + "/ledger")
	if err != nil {
		return err
	}

	// Release nodes one at a time or in a batch
	event := ledger.NewNodesReleased(nodeIDs)
	_, err = ldg.Append(event)
	return err
}

// outputReapResult formats and outputs the reap result.
func outputReapResult(cmd *cobra.Command, result reapResult, format string, dryRun bool) error {
	switch format {
	case "json":
		output, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))

	default:
		// Text format
		if result.Count == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No stale locks found.")
		} else if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "Would reap %d stale lock(s):\n", result.Count)
			for _, id := range result.Reaped {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", id)
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Reaped %d stale lock(s):\n", result.Count)
			for _, id := range result.Reaped {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", id)
			}
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newReapCmd())
}
