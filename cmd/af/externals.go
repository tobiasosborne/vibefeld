// Package main contains the af externals and af external commands for viewing external references.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
)

// newExternalsCmd creates the externals command for listing all external references.
func newExternalsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "externals",
		GroupID: GroupQuery,
		Short:   "List all external references in the proof",
		Long: `List all external references that have been added to the proof.

External references cite theorems, papers, or other sources that can be
referenced in proof steps. This command displays all externals with their names.

Examples:
  af externals                     List all externals
  af externals --format json       Output in JSON format
  af externals -d /path/to/proof   List externals from specific directory`,
		RunE: runExternals,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().Bool("verbose", false, "Show verbose output")

	return cmd
}

// newExternalCmd creates the external command for showing a specific external reference.
func newExternalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "external <name>",
		GroupID: GroupQuery,
		Short:   "Show a specific external reference by name",
		Long: `Show details of a specific external reference.

Retrieves and displays the external reference with the given name, including
its source and metadata.

Examples:
  af external "Fermat's Last Theorem"    Show the external reference by name
  af external "Prime Number Theorem" -F  Show full details
  af external "Riemann Hypothesis" -f json  Output in JSON format`,
		Args: cobra.ExactArgs(1),
		RunE: runExternal,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().BoolP("full", "F", false, "Show full external details")

	return cmd
}

// runExternals executes the externals command.
func runExternals(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Create proof service to verify initialization
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
		return fmt.Errorf("proof not initialized")
	}

	// Get all externals from service
	externals, err := getAllExternals(svc)
	if err != nil {
		return fmt.Errorf("error loading externals: %w", err)
	}

	// Sort by name
	sort.Slice(externals, func(i, j int) bool {
		return externals[i].Name < externals[j].Name
	})

	// Output based on format
	if format == "json" {
		return outputExternalsJSON(cmd, externals)
	}

	return outputExternalsText(cmd, externals)
}

// runExternal executes the external command.
func runExternal(cmd *cobra.Command, args []string) error {
	name := args[0]
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("external name cannot be empty")
	}

	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	full, _ := cmd.Flags().GetBool("full")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Create proof service to verify initialization
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
		return fmt.Errorf("proof not initialized")
	}

	// Get all externals and find by name
	externals, err := getAllExternals(svc)
	if err != nil {
		return fmt.Errorf("error loading externals: %w", err)
	}

	// Find external by name (exact match, case-sensitive)
	var ext *node.External
	for _, e := range externals {
		if e.Name == name {
			ext = e
			break
		}
	}

	if ext == nil {
		return fmt.Errorf("external %q not found", name)
	}

	// Output based on format
	if format == "json" {
		return outputExternalJSON(cmd, ext, full)
	}

	return outputExternalText(cmd, ext, full)
}

// getAllExternals returns all externals from the proof directory.
func getAllExternals(svc *service.ProofService) ([]*node.External, error) {
	// List external IDs from service
	ids, err := svc.ListExternals()
	if err != nil {
		// If externals directory doesn't exist, return empty list
		if strings.Contains(err.Error(), "no such file or directory") {
			return []*node.External{}, nil
		}
		return nil, err
	}

	// Load each external
	externals := make([]*node.External, 0, len(ids))
	for _, id := range ids {
		ext, err := svc.ReadExternal(id)
		if err != nil {
			return nil, err
		}
		externals = append(externals, ext)
	}

	return externals, nil
}

// outputExternalsJSON outputs externals in JSON format.
func outputExternalsJSON(cmd *cobra.Command, externals []*node.External) error {
	// Create JSON output
	output := make([]map[string]interface{}, 0, len(externals))
	for _, ext := range externals {
		output = append(output, externalToJSON(ext))
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputExternalsText outputs externals in text format.
func outputExternalsText(cmd *cobra.Command, externals []*node.External) error {
	if len(externals) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No external references in this proof.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af add-external  - Add an external reference to the proof")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "External References (%d):\n\n", len(externals))

	for _, ext := range externals {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", ext.Name)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af external <name>  - Show details of a specific external")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status           - View proof status")

	return nil
}

// outputExternalJSON outputs a single external in JSON format.
func outputExternalJSON(cmd *cobra.Command, ext *node.External, full bool) error {
	output := externalToJSON(ext)

	// If not full, we still include all basic fields
	// The full flag mainly affects text output

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputExternalText outputs a single external in text format.
func outputExternalText(cmd *cobra.Command, ext *node.External, full bool) error {
	fmt.Fprintf(cmd.OutOrStdout(), "External: %s\n\n", ext.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Source: %s\n", ext.Source)

	if full {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "  ID:           %s\n", ext.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Content Hash: %s\n", ext.ContentHash)
		fmt.Fprintf(cmd.OutOrStdout(), "  Created:      %s\n", ext.Created.String())
		if ext.Notes != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Notes:        %s\n", ext.Notes)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af externals  - List all external references")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status     - View proof status")

	return nil
}

// externalToJSON converts an external to a JSON-friendly map.
func externalToJSON(ext *node.External) map[string]interface{} {
	result := map[string]interface{}{
		"id":           ext.ID,
		"name":         ext.Name,
		"source":       ext.Source,
		"content_hash": ext.ContentHash,
		"created":      ext.Created.String(),
	}

	if ext.Notes != "" {
		result["notes"] = ext.Notes
	}

	return result
}

func init() {
	rootCmd.AddCommand(newExternalsCmd())
	rootCmd.AddCommand(newExternalCmd())
}
