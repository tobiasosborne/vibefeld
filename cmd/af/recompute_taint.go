// Package main contains the af recompute-taint command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// newRecomputeTaintCmd creates the recompute-taint command.
func newRecomputeTaintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "recompute-taint",
		GroupID: GroupAdmin,
		Short:   "Re-sync derived taint audit records",
		Long: `Re-sync TaintRecomputed audit records for all nodes in the proof tree.

Taint propagates through the proof tree based on epistemic states:
- Admitted nodes are self_admitted
- Admitted ancestors or active descendants taint validated nodes
- Pending/draft/needs_refinement ancestors or active descendants make nodes unresolved
- Archived/refuted child branches are severed from upward propagation
- Descendant-derived taint does not flow back down into siblings

Proof state is always derived correctly during replay. This command compares
that state with each node's last TaintRecomputed ledger event (or its
NodeCreated baseline) and appends corrected audit events where they differ.
Use --dry-run to preview stale audit records without appending corrections.
Use --verbose for detailed output.

Examples:
  af recompute-taint                    Re-sync audit records in current directory
  af recompute-taint --dir /path        Re-sync audit records in a specific directory
  af recompute-taint --dry-run          Preview stale audit records
  af recompute-taint -v                 Verbose output with details
  af recompute-taint -f json            Output in JSON format`,
		RunE: runRecomputeTaint,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().Bool("dry-run", false, "Show what would change without applying")
	cmd.Flags().BoolP("verbose", "v", false, "Verbose output with details")
	markDryRunSupported(cmd)

	return cmd
}

// runRecomputeTaint executes the recompute-taint command.
func runRecomputeTaint(cmd *cobra.Command, args []string) error {
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
	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return err
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Recompute taint through service layer
	result, err := svc.RecomputeAllTaint(dryRun)
	if err != nil {
		return fmt.Errorf("error recomputing taint: %w", err)
	}

	// Output result
	return outputRecomputeTaintResult(cmd, result, verbose, format)
}

// outputRecomputeTaintResult outputs the result based on format.
func outputRecomputeTaintResult(cmd *cobra.Command, result *service.RecomputeTaintResult, verbose bool, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return outputRecomputeTaintJSON(cmd, result)
	default:
		return outputRecomputeTaintText(cmd, result, verbose)
	}
}

// outputRecomputeTaintJSON outputs the result in JSON format.
func outputRecomputeTaintJSON(cmd *cobra.Command, result *service.RecomputeTaintResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputRecomputeTaintText outputs the result in text format.
func outputRecomputeTaintText(cmd *cobra.Command, result *service.RecomputeTaintResult, verbose bool) error {
	if result.DryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "[Dry run] Taint recomputation preview:")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Taint recomputation complete.")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Total nodes: %d\n", result.TotalNodes)
	fmt.Fprintf(cmd.OutOrStdout(), "Nodes changed: %d\n", result.NodesChanged)

	if verbose && len(result.Changes) > 0 {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Changes:")
		for _, change := range result.Changes {
			fmt.Fprintf(cmd.OutOrStdout(), "  Node %s: %s -> %s\n",
				change.NodeID, change.OldTaint, change.NewTaint)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newRecomputeTaintCmd())
}
