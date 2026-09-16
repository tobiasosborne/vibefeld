// Package main contains the af reap command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/service"
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
	cmd.Flags().Bool("ledger-lock", false, "Reap the ledger write lock instead of node claim locks")

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
	ledgerLock, err := cmd.Flags().GetBool("ledger-lock")
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

	// Ledger-lock mode is a separate operation from node claim reaping.
	if ledgerLock {
		return runReapLedgerLock(cmd, svc, format, dryRun)
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
	now := service.FromTime(time.Now())
	var toReap []service.NodeID

	for _, n := range st.AllNodes() {
		if n.WorkflowState != service.WorkflowClaimed {
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
		Reaped: service.ToStringSlice(toReap),
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

// releaseNodes releases the given nodes through the service's commit primitive.
func releaseNodes(svc *service.ProofService, nodeIDs []service.NodeID) error {
	return svc.ReleaseNodes(nodeIDs)
}

// ledgerLockResult is the machine-readable outcome of `af reap --ledger-lock`.
type ledgerLockResult struct {
	DryRun     bool   `json:"dry_run"`
	Present    bool   `json:"present"`
	Reaped     bool   `json:"reaped"`
	Stale      bool   `json:"stale"`
	AgentID    string `json:"agent_id,omitempty"`
	PID        int    `json:"pid,omitempty"`
	AcquiredAt string `json:"acquired_at,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Message    string `json:"message,omitempty"`
}

// runReapLedgerLock reports or clears a stale ledger write lock. A lock whose
// recorded pid is alive is never removed; one whose pid is dead, or which has
// no pid and is older than the configured lock timeout, is removed (unless
// --dry-run). No lock file at all is a clean no-op.
func runReapLedgerLock(cmd *cobra.Command, svc *service.ProofService, format string, dryRun bool) error {
	ledgerDir := filepath.Join(svc.Path(), "ledger")

	agentID, pid, acquiredAt, present, err := ledger.Inspect(ledgerDir)
	if err != nil {
		return fmt.Errorf("error reading ledger lock: %w", err)
	}

	result := ledgerLockResult{DryRun: dryRun, Present: present, AgentID: agentID, PID: pid}
	if present && !acquiredAt.IsZero() {
		result.AcquiredAt = acquiredAt.UTC().Format(time.RFC3339Nano)
	}

	if !present {
		result.Message = "no ledger lock present"
		return outputLedgerLockResult(cmd, result, format)
	}

	timeout, err := svc.LockTimeout()
	if err != nil {
		return fmt.Errorf("error reading lock timeout: %w", err)
	}

	stale, reason, _, err := ledger.StaleLock(ledgerDir, timeout)
	if err != nil {
		return fmt.Errorf("error inspecting ledger lock: %w", err)
	}
	result.Stale = stale
	result.Reason = reason

	if !stale {
		result.Message = fmt.Sprintf("ledger lock is held by %q (pid %d); not reaping a live lock", agentID, pid)
		if err := outputLedgerLockResult(cmd, result, format); err != nil {
			return err
		}
		return fmt.Errorf("ledger lock is held by %q (pid %d): %s", agentID, pid, reason)
	}

	if dryRun {
		result.Message = "would reap stale ledger lock: " + reason
		return outputLedgerLockResult(cmd, result, format)
	}

	if err := ledger.RemoveLockFile(ledgerDir); err != nil {
		return fmt.Errorf("error removing ledger lock: %w", err)
	}
	result.Reaped = true
	result.Message = "reaped stale ledger lock: " + reason
	return outputLedgerLockResult(cmd, result, format)
}

// outputLedgerLockResult emits the ledger-lock result in text or JSON.
func outputLedgerLockResult(cmd *cobra.Command, result ledgerLockResult, format string) error {
	if format == "json" {
		out, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}
	if result.Message != "" {
		fmt.Fprintln(cmd.OutOrStdout(), result.Message)
	}
	return nil
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
