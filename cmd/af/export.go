package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// newExportCmd creates the export command.
func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "export",
		GroupID: GroupSetup,
		Short:   "Export proof to different formats",
		Long: `Export the proof tree to various document formats.

Supported formats:
  - markdown, md: Export to Markdown format (default)
  - latex, tex: Export to LaTeX format

The export includes:
  - Hierarchical node tree structure
  - Node statements and justifications
  - Epistemic states (pending, validated, admitted, refuted, archived)
  - Node types and inference rules

Graph projection (--graph json):
  A separate, read-only export used by external tooling (e.g. rk's
  projection layer). Ignores --format. Emits a single deterministic JSON
  document: a top-level "schema_version" field, the workspace identity,
  every node (id, statement/contract text, all three recorded axes —
  workflow/epistemic/taint state — and parent/child structure), and a
  cheap validation summary (node and challenge status counts). Node order
  is stable (hierarchical ID order); running it twice against an
  unchanged proof produces byte-identical output. See
  docs/export-graph-v1.md for the full schema.

Examples:
  af export                           Export to stdout in Markdown format
  af export --format latex            Export to stdout in LaTeX format
  af export -o proof.md               Export to file in Markdown format
  af export --format latex -o proof.tex  Export to LaTeX file
  af export --dir /path/to/proof      Export proof from specific directory
  af export --graph json              Export the graph projection to stdout
  af export --graph json -o graph.json  Export the graph projection to a file`,
		RunE: runExport,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "markdown", "Output format (markdown, md, latex, tex)")
	cmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	cmd.Flags().String("graph", "", "Export the graph projection instead (json); ignores --format")

	return cmd
}

// runExport executes the export command.
func runExport(cmd *cobra.Command, args []string) error {
	// Get flags
	dir := service.MustString(cmd, "dir")
	format := service.MustString(cmd, "format")
	outputPath := service.MustString(cmd, "output")
	graphFormat := service.MustString(cmd, "graph")

	if graphFormat != "" {
		return runExportGraph(cmd, dir, graphFormat, outputPath)
	}

	// Validate format first (before checking directory)
	format = strings.ToLower(format)
	if err := service.ValidateExportFormat(format); err != nil {
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
	output, err := service.ExportProof(st, format)
	if err != nil {
		return fmt.Errorf("error exporting proof: %w", err)
	}

	return writeExportOutput(cmd, outputPath, output)
}

// runExportGraph handles `af export --graph json`: a read-only projection
// export, entirely separate from the --format (markdown/latex) path. It
// never appends to or otherwise mutates the ledger.
func runExportGraph(cmd *cobra.Command, dir, graphFormat, outputPath string) error {
	graphFormat = strings.ToLower(graphFormat)
	if err := service.ValidateGraphFormat(graphFormat); err != nil {
		return err
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("no proof initialized in %q - run 'af init' to start a new proof", dir)
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	cfg, err := svc.Config()
	if err != nil {
		return fmt.Errorf("error loading proof config: %w", err)
	}

	// The workspace id is the resolved (absolute) proof directory path: the
	// join key rk's registry locates via its shard's `workspace:` field
	// (PRD C5) before byte-matching contract text against node statements.
	workspaceID, err := filepath.Abs(dir)
	if err != nil {
		workspaceID = dir
	}

	output, err := service.ExportGraph(st, workspaceID, cfg)
	if err != nil {
		return fmt.Errorf("error exporting graph: %w", err)
	}

	return writeExportOutput(cmd, outputPath, output)
}

// writeExportOutput writes export output to outputPath if set, else stdout.
func writeExportOutput(cmd *cobra.Command, outputPath, output string) error {
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
