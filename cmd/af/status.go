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

Examples:
  af status                     Show proof status in current directory
  af status --dir /path/to/proof  Show status for specific proof directory
  af status --format json       Output in JSON format`,
		RunE: runStatus,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// runStatus executes the status command.
func runStatus(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")

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

	// Output based on format
	if format == "json" {
		output := render.RenderStatusJSON(st)
		fmt.Fprintln(cmd.OutOrStdout(), output)
		return nil
	}

	// Text format
	output := render.RenderStatus(st)
	fmt.Fprint(cmd.OutOrStdout(), output)

	return nil
}

func init() {
	rootCmd.AddCommand(newStatusCmd())
}
