package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

// newStatusCmd creates the status command.
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		GroupID: GroupSetup,
		Short:   "Show proof status and node tree",
		Long: `Show the current proof status including the node tree, statistics, and available jobs.

The status command displays:
  - Node tree with hierarchical IDs
  - Epistemic state (pending, validated, admitted, refuted, archived)
  - Taint state (clean, self_admitted, tainted, unresolved)
  - Statistics summary
  - Available jobs for provers and verifiers

Pagination:
  For large proofs, use --limit and --offset to paginate the node display.
  --limit controls how many nodes to show (0 = unlimited)
  --offset skips the first N nodes before displaying

Urgent mode:
  Use --urgent to show only items needing immediate attention:
  - Nodes with blocking challenges (critical or major severity)
  - Available prover jobs (nodes needing refinement)
  - Ready verifier jobs (nodes ready for review)

Navigation:
  Use --focus to show only a subtree, --depth to limit tree depth,
  and --compact for a condensed one-line-per-node view with challenge badges.

Examples:
  af status                        Show proof status in current directory
  af status --dir /path/to/proof   Show status for specific proof directory
  af status --format json          Output in JSON format
  af status --limit 10             Show only the first 10 nodes
  af status --limit 10 --offset 5  Show 10 nodes, starting from the 6th
  af status --urgent               Show only urgent items needing attention
  af status --focus 1.6            Show only subtree rooted at node 1.6
  af status --depth 2              Show only nodes up to depth 2
  af status --compact              One line per node with challenge badges`,
		RunE: runStatus,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().IntP("limit", "l", 0, "Maximum nodes to display (0 = unlimited)")
	cmd.Flags().IntP("offset", "o", 0, "Number of nodes to skip")
	cmd.Flags().BoolP("urgent", "u", false, "Show only urgent items (blocking challenges, available jobs)")
	cmd.Flags().String("focus", "", "Show only subtree rooted at this node ID")
	cmd.Flags().Int("depth", 0, "Maximum tree depth to display (0 = unlimited)")
	cmd.Flags().BoolP("compact", "c", false, "Compact one-line-per-node view with challenge badges")

	return cmd
}

// runStatus executes the status command.
func runStatus(cmd *cobra.Command, args []string) error {
	// Get flags
	dir := service.MustString(cmd, "dir")
	format := service.MustString(cmd, "format")
	limit := service.MustInt(cmd, "limit")
	offset := service.MustInt(cmd, "offset")
	urgent := service.MustBool(cmd, "urgent")
	focus, _ := cmd.Flags().GetString("focus")
	depth, _ := cmd.Flags().GetInt("depth")
	compact := service.MustBool(cmd, "compact")

	// Validate pagination flags
	if limit < 0 {
		return fmt.Errorf("invalid limit %d: must be non-negative", limit)
	}
	if offset < 0 {
		return fmt.Errorf("invalid offset %d: must be non-negative", offset)
	}
	if depth < 0 {
		return fmt.Errorf("invalid depth %d: must be non-negative", depth)
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
		if format == "json" {
			fmt.Fprintln(cmd.OutOrStdout(), `{"error":"proof not initialized"}`)
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No proof initialized. Run 'af init' to start a new proof.")
		return nil
	}

	// Load current state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Urgent mode: show only urgent items
	if urgent {
		if format == "json" {
			output := render.RenderStatusUrgentJSON(st)
			fmt.Fprintln(cmd.OutOrStdout(), output)
			return nil
		}
		output := render.RenderStatusUrgent(st)
		fmt.Fprint(cmd.OutOrStdout(), output)
		return nil
	}

	// Build render options
	opts := render.StatusOptions{
		Limit:   limit,
		Offset:  offset,
		Focus:   focus,
		Depth:   depth,
		Compact: compact,
	}

	// Validate focus node ID if provided
	if focus != "" {
		if _, err := service.ParseNodeID(focus); err != nil {
			return fmt.Errorf("invalid focus node ID %q: %w", focus, err)
		}
	}

	// Output based on format with navigation support
	if format == "json" {
		output := render.RenderStatusJSON(st, limit, offset)
		fmt.Fprintln(cmd.OutOrStdout(), output)
		return nil
	}

	// Text format with navigation support
	output := render.RenderStatusFiltered(st, opts)
	fmt.Fprint(cmd.OutOrStdout(), output)

	return nil
}

func init() {
	rootCmd.AddCommand(newStatusCmd())
}
