// Package main contains the af pending-refs and af pending-ref commands for viewing
// pending external references (externals that haven't been verified yet).
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
)

// newPendingRefsCmd creates the pending-refs command for listing all pending external references.
func newPendingRefsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending-refs",
		Short: "List all pending external references in the proof",
		Long: `List all pending external references that have not yet been verified.

Pending external references are citations to theorems, papers, or other sources
that proofs may depend on. Until verified, they are considered "pending".

Examples:
  af pending-refs                     List all pending references
  af pending-refs --format json       Output in JSON format
  af pending-refs -d /path/to/proof   List pending refs from specific directory`,
		RunE: runPendingRefs,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().Bool("verbose", false, "Show verbose output")

	return cmd
}

// newPendingRefCmd creates the pending-ref command for showing a specific pending reference.
func newPendingRefCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending-ref <name>",
		Short: "Show a specific pending external reference by name",
		Long: `Show details of a specific pending external reference.

Retrieves and displays the pending reference with the given name, including
its source and metadata.

Examples:
  af pending-ref "Fermat's Last Theorem"    Show the pending reference by name
  af pending-ref "Prime Number Theorem" -F  Show full details
  af pending-ref "Riemann Hypothesis" -f json  Output in JSON format`,
		Args: cobra.ExactArgs(1),
		RunE: runPendingRef,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().BoolP("full", "F", false, "Show full pending reference details")

	return cmd
}

// runPendingRefs executes the pending-refs command.
func runPendingRefs(cmd *cobra.Command, args []string) error {
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

	// Get all pending refs (externals that are not verified)
	pendingRefs, err := getPendingExternals(svc.Path())
	if err != nil {
		return fmt.Errorf("error loading pending references: %w", err)
	}

	// Sort by name
	sort.Slice(pendingRefs, func(i, j int) bool {
		return pendingRefs[i].Name < pendingRefs[j].Name
	})

	// Output based on format
	if format == "json" {
		return outputPendingRefsJSON(cmd, pendingRefs)
	}

	return outputPendingRefsText(cmd, pendingRefs)
}

// runPendingRef executes the pending-ref command.
func runPendingRef(cmd *cobra.Command, args []string) error {
	name := args[0]
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("pending reference name cannot be empty")
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

	// Get all pending refs and find by name or ID
	pendingRefs, err := getPendingExternals(svc.Path())
	if err != nil {
		return fmt.Errorf("error loading pending references: %w", err)
	}

	// Find pending ref by name or ID (exact match, case-sensitive)
	var ref *node.External
	for _, r := range pendingRefs {
		if r.Name == name || r.ID == name {
			ref = r
			break
		}
	}

	if ref == nil {
		return fmt.Errorf("pending reference %q not found", name)
	}

	// Output based on format
	if format == "json" {
		return outputPendingRefJSON(cmd, ref, full)
	}

	return outputPendingRefText(cmd, ref, full)
}

// getPendingExternals returns all externals that are pending verification.
// Currently, all externals are considered pending since verification is not yet implemented.
func getPendingExternals(proofDir string) ([]*node.External, error) {
	// List external IDs from filesystem
	ids, err := fs.ListExternals(proofDir)
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
		ext, err := fs.ReadExternal(proofDir, id)
		if err != nil {
			return nil, err
		}
		// All externals are pending until verified
		// (verification is not yet implemented, so all are pending)
		externals = append(externals, ext)
	}

	return externals, nil
}

// outputPendingRefsJSON outputs pending refs in JSON format.
func outputPendingRefsJSON(cmd *cobra.Command, refs []*node.External) error {
	// Create JSON output
	output := make([]map[string]interface{}, 0, len(refs))
	for _, ref := range refs {
		output = append(output, pendingRefToJSON(ref))
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputPendingRefsText outputs pending refs in text format.
func outputPendingRefsText(cmd *cobra.Command, refs []*node.External) error {
	if len(refs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No pending external references in this proof.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af add-external  - Add an external reference to the proof")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Pending External References (%d):\n\n", len(refs))

	for _, ref := range refs {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", ref.Name)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af pending-ref <name>  - Show details of a specific pending reference")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status              - View proof status")

	return nil
}

// outputPendingRefJSON outputs a single pending ref in JSON format.
func outputPendingRefJSON(cmd *cobra.Command, ref *node.External, full bool) error {
	output := pendingRefToJSON(ref)

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputPendingRefText outputs a single pending ref in text format.
func outputPendingRefText(cmd *cobra.Command, ref *node.External, full bool) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Pending Reference: %s\n\n", ref.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Source: %s\n", ref.Source)
	fmt.Fprintf(cmd.OutOrStdout(), "  Status: pending verification\n")

	if full {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "  ID:           %s\n", ref.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Content Hash: %s\n", ref.ContentHash)
		fmt.Fprintf(cmd.OutOrStdout(), "  Created:      %s\n", ref.Created.String())
		if ref.Notes != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Notes:        %s\n", ref.Notes)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af pending-refs  - List all pending external references")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status        - View proof status")

	return nil
}

// pendingRefToJSON converts a pending ref to a JSON-friendly map.
func pendingRefToJSON(ref *node.External) map[string]interface{} {
	result := map[string]interface{}{
		"id":           ref.ID,
		"name":         ref.Name,
		"source":       ref.Source,
		"content_hash": ref.ContentHash,
		"created":      ref.Created.String(),
		"status":       "pending",
	}

	if ref.Notes != "" {
		result["notes"] = ref.Notes
	}

	return result
}

func init() {
	rootCmd.AddCommand(newPendingRefsCmd())
	rootCmd.AddCommand(newPendingRefCmd())
}
