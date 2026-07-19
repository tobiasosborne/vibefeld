// Package main contains the af unvalidate command implementation.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

func newUnvalidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unvalidate [node-id]",
		GroupID: GroupVerifier,
		Short:   "Revoke validation and revert a node to pending",
		Long: `Revoke validation on a previously validated node, reverting it to pending state.

This is a verifier action for when a validation error is discovered after
acceptance. The node returns to the verification queue.

Use this when:
- A validation mistake is discovered after acceptance
- A later finding invalidates an earlier accepted step
- Additional scrutiny is needed on a previously accepted node

Note: Unvalidating a node with validated children will cause taint to
propagate — those children will become taint-unresolved.

BULK REVOCATION (--batch): every node whose current validation carries the
given batch id is unvalidated (rk PRD C3's batch verification mode, item
V3). Each revocation is a normal, attributed NodeUnvalidated event, same as
the single-node form — never a silent state rewrite. A batch id matching no
currently-validated node is a clean no-op with exit code 7, not an error.

Examples:
  af unvalidate 1.2
  af unvalidate 1.2 --reason "Formula error in step 3"
  af unvalidate 1.2 --agent verifier-001
  af unvalidate 1.2 -f json -y
  af unvalidate --batch batch-1 --reason "Batch review overturned"
  af unvalidate --batch batch-1 -y`,
		Args: cobra.MaximumNArgs(1),
		RunE: runUnvalidate,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")
	cmd.Flags().String("reason", "", "Reason for revoking validation")
	cmd.Flags().String("agent", "", "Agent ID (verifier identity)")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().String("batch", "", "Batch id: unvalidate every node validated under this batch (bulk revocation)")

	return cmd
}

func runUnvalidate(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	reason := cli.MustString(cmd, "reason")
	agent := cli.MustString(cmd, "agent")
	yes := cli.MustBool(cmd, "yes")
	batch := cli.MustString(cmd, "batch")

	if batch != "" && len(args) > 0 {
		return render.NewUsageError("af unvalidate",
			"--batch and a node id are mutually exclusive; use one or the other",
			[]string{"af unvalidate 1.2", "af unvalidate --batch batch-1"})
	}
	if batch == "" && len(args) == 0 {
		return render.NewUsageError("af unvalidate",
			"either specify a node id or use --batch <id> for bulk revocation",
			[]string{"af unvalidate 1.2", "af unvalidate --batch batch-1"})
	}

	if batch != "" {
		return runUnvalidateBatch(cmd, dir, batch, reason, agent, format, yes)
	}

	// Parse and validate node ID
	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af unvalidate")
		return render.InvalidNodeIDError("af unvalidate", nodeIDStr, examples)
	}

	// Confirmation prompt
	msg := fmt.Sprintf("Unvalidate node %s? This reverts it to pending and may propagate taint to descendants.", nodeID.String())
	confirmed, err := cli.ConfirmAction(cmd.OutOrStdout(), msg, yes)
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	// Create service and unvalidate
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	if err := svc.UnvalidateNode(nodeID, reason, agent); err != nil {
		return fmt.Errorf("error unvalidating node: %w", err)
	}

	// Output result
	return outputUnvalidateResult(cmd, nodeID, reason, format)
}

// runUnvalidateBatch handles `af unvalidate --batch <id>` (rk PRD C3, item
// V3): bulk-revokes validation on every node currently carrying batchID.
func runUnvalidateBatch(cmd *cobra.Command, dir, batchID, reason, agent, format string, yes bool) error {
	msg := fmt.Sprintf("Unvalidate every node in batch %s? This reverts them to pending and may propagate taint to descendants.", batchID)
	confirmed, err := cli.ConfirmAction(cmd.OutOrStdout(), msg, yes)
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	report, applyErr := svc.UnvalidateBatch(batchID, reason, agent)
	if outputErr := outputUnvalidateBatchResult(cmd, report, applyErr, format); outputErr != nil {
		return outputErr
	}
	return applyErr
}

func outputUnvalidateBatchResult(cmd *cobra.Command, report *service.UnvalidateBatchReport, applyErr error, format string) error {
	if errors.Is(applyErr, service.ErrUnvalidateBatchNotFound) {
		switch strings.ToLower(format) {
		case "json":
			out, err := json.MarshalIndent(map[string]interface{}{
				"batch_id": report.BatchID,
				"count":    0,
				"message":  "no node is currently validated under this batch id",
			}, "", "  ")
			if err != nil {
				return fmt.Errorf("error marshaling JSON: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
		default:
			fmt.Fprintf(cmd.OutOrStdout(), "No node is currently validated under batch %s. Nothing to do.\n", report.BatchID)
		}
		return nil
	}

	switch strings.ToLower(format) {
	case "json":
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Batch %s: %d node(s) unvalidated\n", report.BatchID, report.Count)
		for _, item := range report.Items {
			if item.Err == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s - reverted to pending\n", item.Node.String())
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s - FAILED: %s\n", item.Node.String(), item.Err)
			}
		}
	}
	return nil
}

func outputUnvalidateResult(cmd *cobra.Command, nodeID service.NodeID, reason, format string) error {
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id":        nodeID.String(),
			"previous_state": "validated",
			"current_state":  "pending",
		}
		if reason != "" {
			result["reason"] = reason
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Validation revoked for node %s\n", nodeID.String())
		fmt.Fprintf(cmd.OutOrStdout(), "  Previous state: validated\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Current state: pending\n")
		if reason != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Reason: %s\n", reason)
		}
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "Next: A verifier should re-examine this node with 'af get %s'\n", nodeID.String())
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newUnvalidateCmd())
}
