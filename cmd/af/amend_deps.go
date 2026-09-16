// Package main contains the af amend-deps command implementation (D2).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/service"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// newAmendDepsCmd creates the amend-deps command.
func newAmendDepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "amend-deps [node-id]",
		GroupID: GroupProver,
		Short:   "Correct a node's dependency edges (one event, batchable)",
		Long: `Correct a node's dependency edges with an append-only amendment.

Policy:
  - States: pending, draft and needs_refinement may be corrected directly.
    A validated node is refused unless --reopen is given. An admitted node
    must be corrected with 'af unadmit' first; refuted and archived nodes
    cannot be corrected.
  - One event: the edge replacement (and, with --reopen, the
    validated -> pending transition) is a single node_deps_amended event.
    There is no window in which the node is pending with its old edges.
  - Hash change: the content hash covers dependency IDs, so a correction
    changes the node's hash. Any verdict or claim-test authored against the
    old hash is stale and must be regenerated; --expect-hash refuses to apply
    against a node whose hash moved.
  - --strict makes an add of an existing edge or a remove of an absent one an
    error instead of a no-op.
  - --reopen re-verifies the node: it becomes pending and must be re-accepted.

Manifest form:
  af amend-deps --file manifest.json [--dry-run] [--resume] -f json
The dry run writes generated operation ids back to the file so the real run
keeps them, and prints the exact edge diff, resulting hash, and any cycle or
scope path for each item.

Exit codes:
  0  applied (single) / every manifest item applied or unchanged
  5  some manifest items applied, some rejected or blocked
  6  no manifest items applied
  7  every manifest item already unchanged
  3  rejected input (invalid state, missing target, cycle, scope leak)
  1  concurrent modification (retry)

Examples:
  af amend-deps 1.2 --add 1.5 --remove 1.3 --owner prover-1 --reason "wrong citation"
  af amend-deps 1.2 --reopen --add 1.5 --owner verifier-1 --reason "fix after review"
  af amend-deps --file corrections.json --dry-run
  af amend-deps --file corrections.json --resume -f json`,
		Args: cobra.MaximumNArgs(1),
		RunE: runAmendDeps,
	}

	cmd.Flags().StringSlice("add", nil, "Reference dependencies to add (comma-separated)")
	cmd.Flags().StringSlice("remove", nil, "Reference dependencies to remove (comma-separated)")
	cmd.Flags().StringSlice("add-validated", nil, "Validation dependencies to add (comma-separated)")
	cmd.Flags().StringSlice("remove-validated", nil, "Validation dependencies to remove (comma-separated)")
	cmd.Flags().Bool("reopen", false, "Reopen a validated node (validated -> pending) in the same event")
	cmd.Flags().String("expect-hash", "", "Refuse unless the node's current content hash equals this value")
	cmd.Flags().Bool("strict", false, "Make a no-op edge change an error")
	cmd.Flags().StringP("owner", "o", "", "Agent/owner name (required)")
	cmd.Flags().StringP("reason", "r", "", "Reason for the correction (required)")
	cmd.Flags().String("file", "", "Apply a JSON manifest of corrections")
	cmd.Flags().Bool("resume", false, "Rerun a manifest; already-applied items are recognised from the ledger")
	cmd.Flags().StringP("dir", "d", ".", "Proof directory")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")

	// This command implements --dry-run for the manifest form only; the
	// single-node form rejects the flag explicitly below.
	markDryRunSupported(cmd)

	return cmd
}

func runAmendDeps(cmd *cobra.Command, args []string) error {
	dir := service.MustString(cmd, "dir")
	format := service.MustString(cmd, "format")
	manifestPath := service.MustString(cmd, "file")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be text or json", format)
	}

	if manifestPath != "" {
		if len(args) > 0 {
			return fmt.Errorf("--file applies a manifest; do not also pass a node id")
		}
		return runAmendDepsManifest(cmd, dir, format, manifestPath, dryRun)
	}

	if dryRun {
		return fmt.Errorf("--dry-run is only supported with --file")
	}
	if len(args) != 1 {
		return fmt.Errorf("amend-deps requires a node id, or --file for a manifest")
	}
	return runAmendDepsSingle(cmd, dir, format, args[0])
}

func runAmendDepsSingle(cmd *cobra.Command, dir, format, nodeIDStr string) error {
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", nodeIDStr, err)
	}
	req, err := amendDepsRequestFromFlags(cmd)
	if err != nil {
		return err
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to load proof: %w", err)
	}
	result, err := svc.AmendDeps(nodeID, req)
	if err != nil {
		return err
	}

	if format == "json" {
		out := map[string]interface{}{
			"node_id":           nodeIDStr,
			"outcome":           result.Outcome,
			"seq":               result.Seq,
			"old_hash":          result.OldHash,
			"new_hash":          result.NewHash,
			"reopened":          result.Reopened,
			"reverify":          result.Reverify,
			"added":             service.ToStringSlice(result.Added),
			"removed":           service.ToStringSlice(result.Removed),
			"added_validated":   service.ToStringSlice(result.AddedValidated),
			"removed_validated": service.ToStringSlice(result.RemovedValidated),
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Node %s: %s\n", nodeIDStr, result.Outcome)
	if result.Seq > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  sequence:   %d\n", result.Seq)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  old hash:   %s\n", result.OldHash)
	fmt.Fprintf(cmd.OutOrStdout(), "  new hash:   %s\n", result.NewHash)
	printEdgeDiff(cmd.OutOrStdout(), result)
	if result.Reopened {
		fmt.Fprintf(cmd.OutOrStdout(), "  reopened:   node is now pending; re-accept it before relying on it\n")
	}
	return nil
}

func runAmendDepsManifest(cmd *cobra.Command, dir, format, path string, dryRun bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read manifest %s: %w", path, err)
	}
	manifest, err := service.ParseAmendDepsManifest(data)
	if err != nil {
		return err
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to load proof: %w", err)
	}

	if dryRun {
		report, err := svc.DryRunAmendDepsManifest(manifest)
		if err != nil {
			return err
		}
		// Persist generated operation ids so the real run keeps them.
		completed, err := manifest.MarshalIndent()
		if err != nil {
			return fmt.Errorf("failed to render completed manifest: %w", err)
		}
		if err := os.WriteFile(path, append(completed, '\n'), 0o644); err != nil {
			return fmt.Errorf("failed to write completed manifest %s: %w", path, err)
		}
		return outputAmendDepsReport(cmd, report, format, true)
	}

	report, applyErr := svc.ApplyAmendDepsManifest(manifest)
	if outputErr := outputAmendDepsReport(cmd, report, format, false); outputErr != nil {
		return outputErr
	}
	return applyErr
}

// amendDepsRequestFromFlags builds a single-node request from the command flags.
func amendDepsRequestFromFlags(cmd *cobra.Command) (service.AmendDepsRequest, error) {
	owner := service.MustString(cmd, "owner")
	reason := service.MustString(cmd, "reason")
	add, err := parseFlagIDs(cmd, "add")
	if err != nil {
		return service.AmendDepsRequest{}, err
	}
	remove, err := parseFlagIDs(cmd, "remove")
	if err != nil {
		return service.AmendDepsRequest{}, err
	}
	addVal, err := parseFlagIDs(cmd, "add-validated")
	if err != nil {
		return service.AmendDepsRequest{}, err
	}
	removeVal, err := parseFlagIDs(cmd, "remove-validated")
	if err != nil {
		return service.AmendDepsRequest{}, err
	}
	return service.AmendDepsRequest{
		Add:             add,
		Remove:          remove,
		AddValidated:    addVal,
		RemoveValidated: removeVal,
		Reopen:          service.MustBool(cmd, "reopen"),
		ExpectHash:      service.MustString(cmd, "expect-hash"),
		Strict:          service.MustBool(cmd, "strict"),
		Owner:           owner,
		Reason:          reason,
	}, nil
}

func parseFlagIDs(cmd *cobra.Command, name string) ([]types.NodeID, error) {
	raw, err := cmd.Flags().GetStringSlice(name)
	if err != nil {
		return nil, err
	}
	out := make([]types.NodeID, 0, len(raw))
	for _, s := range raw {
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := types.Parse(part)
			if err != nil {
				return nil, fmt.Errorf("invalid --%s id %q: %w", name, part, err)
			}
			out = append(out, id)
		}
	}
	return out, nil
}

// outputAmendDepsReport renders a manifest report; dryRun controls the header.
func outputAmendDepsReport(cmd *cobra.Command, report *service.AmendDepsManifestReport, format string, dryRun bool) error {
	if format == "json" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	header := "amend-deps manifest"
	if dryRun {
		header = "amend-deps manifest (dry run — no writes)"
	}
	fmt.Fprintln(cmd.OutOrStdout(), header+":")
	for _, item := range report.Items {
		line := fmt.Sprintf("  %-8s %s", item.Node, item.Status)
		if item.Seq > 0 {
			line += fmt.Sprintf(" seq=%d", item.Seq)
		}
		line += edgeDiffSuffix(item)
		if item.Detail != "" {
			line += " (" + item.Detail + ")"
		}
		fmt.Fprintln(cmd.OutOrStdout(), line)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d applied, %d unchanged, %d rejected, %d blocked (of %d)\n",
		report.Applied, report.Unchanged, report.Rejected, report.Blocked, len(report.Items))
	if report.Aborted {
		fmt.Fprintf(cmd.OutOrStdout(), "Batch aborted: %s\n", report.AbortReason)
	}

	if len(report.Reverify) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nRe-verification work list (reopen):")
		for _, r := range report.Reverify {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s  new_hash=%s  reason=%s\n", r.Node, r.NewHash, r.Reason)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "\nRe-verify with:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af verdicts apply <verdict-file>")
		fmt.Fprintln(cmd.OutOrStdout(), "  af export --graph json")
	}
	return nil
}

func printEdgeDiff(w interface{ Write([]byte) (int, error) }, result service.AmendDepsResult) {
	if s := edgeDiffSuffix(service.AmendDepsManifestStatus{
		Added:            service.ToStringSlice(result.Added),
		Removed:          service.ToStringSlice(result.Removed),
		AddedValidated:   service.ToStringSlice(result.AddedValidated),
		RemovedValidated: service.ToStringSlice(result.RemovedValidated),
	}); s != "" {
		fmt.Fprintf(w, "  diff:%s\n", s)
	}
}

func edgeDiffSuffix(item service.AmendDepsManifestStatus) string {
	var parts []string
	for _, id := range item.Added {
		parts = append(parts, "+"+id)
	}
	for _, id := range item.Removed {
		parts = append(parts, "-"+id)
	}
	for _, id := range item.AddedValidated {
		parts = append(parts, "+v:"+id)
	}
	for _, id := range item.RemovedValidated {
		parts = append(parts, "-v:"+id)
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

func init() {
	rootCmd.AddCommand(newAmendDepsCmd())
}
