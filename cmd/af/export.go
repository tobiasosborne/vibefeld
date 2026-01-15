package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/export"
	"github.com/tobias/vibefeld/internal/service"
)

// newExportCmd creates the export command.
func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export proof to different formats",
		Long: `Export the proof tree to various document formats.

Supported formats:
  - markdown, md: Export to Markdown format (default)
  - latex, tex: Export to LaTeX format

The export includes:
  - Hierarchical node tree structure
  - Node statements and justifications
  - Epistemic states (pending, validated, admitted, refuted, archived)
  - Node types and inference rules

Examples:
  af export                           Export to stdout in Markdown format
  af export --format latex            Export to stdout in LaTeX format
  af export -o proof.md               Export to file in Markdown format
  af export --format latex -o proof.tex  Export to LaTeX file
  af export --dir /path/to/proof      Export proof from specific directory`,
		RunE: runExport,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "markdown", "Output format (markdown, md, latex, tex)")
	cmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")

	return cmd
}

// runExport executes the export command.
func runExport(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	outputPath, _ := cmd.Flags().GetString("output")

	// Validate format first (before checking directory)
	format = strings.ToLower(format)
	if err := export.ValidateFormat(format); err != nil {
		return err
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
		return fmt.Errorf("no proof initialized in %q - run 'af init' to start a new proof", dir)
	}

	// Load current state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Export to the specified format
	output, err := export.Export(st, format)
	if err != nil {
		return fmt.Errorf("error exporting proof: %w", err)
	}

	// Output to file or stdout
	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
			return fmt.Errorf("error writing to file %q: %w", outputPath, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Proof exported to %s\n", outputPath)
	} else {
		fmt.Fprint(cmd.OutOrStdout(), output)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newExportCmd())
}
