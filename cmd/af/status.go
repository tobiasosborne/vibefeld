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
		Use:   "status",
		Short: "Show proof status and node tree",
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

Examples:
  af status                        Show proof status in current directory
  af status --dir /path/to/proof   Show status for specific proof directory
  af status --format json          Output in JSON format
  af status --limit 10             Show only the first 10 nodes
  af status --limit 10 --offset 5  Show 10 nodes, starting from the 6th`,
		RunE: runStatus,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().IntP("limit", "l", 0, "Maximum nodes to display (0 = unlimited)")
	cmd.Flags().IntP("offset", "o", 0, "Number of nodes to skip")

	return cmd
}

// runStatus executes the status command.
func runStatus(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	limit, _ := cmd.Flags().GetInt("limit")
	offset, _ := cmd.Flags().GetInt("offset")

	// Validate pagination flags
	if limit < 0 {
		return fmt.Errorf("invalid limit %d: must be non-negative", limit)
	}
	if offset < 0 {
		return fmt.Errorf("invalid offset %d: must be non-negative", offset)
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

	// Output based on format with pagination support
	if format == "json" {
		output := render.RenderStatusJSON(st, limit, offset)
		fmt.Fprintln(cmd.OutOrStdout(), output)
		return nil
	}

	// Text format with pagination support
	output := render.RenderStatus(st, limit, offset)
	fmt.Fprint(cmd.OutOrStdout(), output)

	return nil
}

func init() {
	rootCmd.AddCommand(newStatusCmd())
}
