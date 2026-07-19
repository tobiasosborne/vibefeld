// Package main contains the af verdicts command group implementation.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/service"
)

// newVerdictsCmd creates the "af verdicts" parent command.
func newVerdictsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "verdicts",
		GroupID: GroupVerifier,
		Short:   "Batch verdict operations (schema-validated ingestion)",
		Long: `Operations on batch verdict files.

A verdict file is a single JSON document listing per-node accept/challenge
decisions for a batch of verification-ready items (rk PRD C3's batched
verification mode). See 'af verdicts apply --help' and
docs/verdicts-apply.md for the file schema.`,
	}
	cmd.AddCommand(newVerdictsApplyCmd())
	return cmd
}

func newVerdictsApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply <file>",
		Short: "Apply a schema-validated batch verdict file",
		Long: `Apply applies every verdict in a batch verdict file, in file order.

The file is a single JSON document: a schema_version, a batch_id, a
verified_by identity, and a list of items. Each item is either
{"node": "1.2", "verdict": "accept", "reason": "..."} or
{"node": "1.2", "verdict": "challenge", "target": "...", "severity": "...",
"reason": "..."}. Every item requires a non-empty "reason" — there is no
blanket accept.

Accepts are ORDER-DEPENDENT: a parent node's children must all be cleared
(validated/admitted/archived) before the parent can be accepted, and this
command does not reorder the file — list children before their parent. A
challenge raised earlier in the file can legitimately block an accept later
in the same file (e.g. it leaves a child pending, which blocks the parent).

A verifier ("verified_by") can never accept a node they are the recorded
author of — that item is rejected, not silently skipped.

This command applies what it can: it does not stop at the first blocked or
rejected item. It reports a per-item outcome — applied, blocked-by:<reason>,
or rejected:<reason> — and never leaves the ledger in an ambiguous state.
Exit codes distinguish the batch's aggregate outcome:
  0  every item applied
  5  some items applied, some blocked or rejected (partial)
  6  the file was valid but zero items applied
  3  the file itself is schema-invalid (nothing was attempted)

See docs/verdicts-apply.md for the full file schema and outcome vocabulary.

Examples:
  af verdicts apply batch-1.json
  af verdicts apply batch-1.json --format json
  af verdicts apply batch-1.json -d ./proof`,
		Args: cobra.ExactArgs(1),
		RunE: runVerdictsApply,
	}
	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")
	return cmd
}

func runVerdictsApply(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	path := args[0]

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return service.NewVerdictsFileReadError(path, readErr)
	}

	f, parseErr := service.ParseVerdictFile(data)
	if parseErr != nil {
		return fmt.Errorf("verdict file %s is invalid: %w", path, parseErr)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	report, applyErr := svc.ApplyVerdicts(f)
	if outputErr := outputVerdictReport(cmd, report, format); outputErr != nil {
		return outputErr
	}
	return applyErr
}

func outputVerdictReport(cmd *cobra.Command, report *service.VerdictReport, format string) error {
	switch strings.ToLower(format) {
	case "json":
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Batch %s (verified by %s):\n", report.BatchID, report.VerifiedBy)
		for _, item := range report.Items {
			line := fmt.Sprintf("  %-10s %-10s %s", item.Node, item.Verdict, item.Status)
			if item.Detail != "" {
				line += " (" + item.Detail + ")"
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n%d applied, %d blocked, %d rejected (of %d)\n",
			report.Applied, report.Blocked, report.Rejected, len(report.Items))
		if report.Aborted {
			fmt.Fprintf(cmd.OutOrStdout(), "Batch aborted: %s\n", report.AbortReason)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newVerdictsCmd())
}
