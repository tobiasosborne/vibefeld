// Package main contains the af pending-defs and af pending-def commands for viewing pending definitions.
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
	"github.com/tobias/vibefeld/internal/types"
)

// newPendingDefsCmd creates the pending-defs command for listing all pending definitions.
func newPendingDefsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending-defs",
		Short: "List all pending definition requests in the proof",
		Long: `List all pending definition requests that have been made in the proof.

Pending definitions are requests for terms that need to be defined.
When a prover needs a term defined, they create a pending definition request.
This command displays all pending definitions with their terms and requesting nodes.

Examples:
  af pending-defs                     List all pending definitions
  af pending-defs --format json       Output in JSON format
  af pending-defs -d /path/to/proof   List pending definitions from specific directory`,
		RunE: runPendingDefs,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// newPendingDefCmd creates the pending-def command for showing a specific pending definition.
func newPendingDefCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending-def <term|node-id|id>",
		Short: "Show a specific pending definition by term, node ID, or ID",
		Long: `Show details of a specific pending definition request.

You can look up a pending definition by:
- Term name (e.g., "group")
- Node ID that requested it (e.g., "1.1")
- Pending definition ID (full or partial)

Examples:
  af pending-def group                Show the pending def for term "group"
  af pending-def 1.1                  Show the pending def requested by node 1.1
  af pending-def abc123               Show the pending def with ID starting with "abc123"
  af pending-def homomorphism -F      Show full details
  af pending-def kernel -f json       Output in JSON format`,
		Args: cobra.ExactArgs(1),
		RunE: runPendingDef,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().BoolP("full", "F", false, "Show full pending definition details")

	return cmd
}

// runPendingDefs executes the pending-defs command.
func runPendingDefs(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Check for empty dir
	if dir == "" {
		return fmt.Errorf("directory path cannot be empty")
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

	// Get all pending definitions from filesystem
	pendingDefs, err := getAllPendingDefs(svc.Path())
	if err != nil {
		return fmt.Errorf("error loading pending definitions: %w", err)
	}

	// Sort by term alphabetically
	sort.Slice(pendingDefs, func(i, j int) bool {
		return pendingDefs[i].Term < pendingDefs[j].Term
	})

	// Output based on format
	if format == "json" {
		return outputPendingDefsJSON(cmd, pendingDefs)
	}

	return outputPendingDefsText(cmd, pendingDefs)
}

// runPendingDef executes the pending-def command.
func runPendingDef(cmd *cobra.Command, args []string) error {
	lookup := args[0]
	if strings.TrimSpace(lookup) == "" {
		return fmt.Errorf("pending definition identifier cannot be empty")
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

	// Get all pending definitions
	pendingDefs, err := getAllPendingDefs(svc.Path())
	if err != nil {
		return fmt.Errorf("error loading pending definitions: %w", err)
	}

	// Find pending definition by various methods
	pd := findPendingDef(pendingDefs, lookup)

	if pd == nil {
		return fmt.Errorf("pending definition %q not found", lookup)
	}

	// Output based on format
	if format == "json" {
		return outputPendingDefJSON(cmd, pd, full)
	}

	return outputPendingDefText(cmd, pd, full)
}

// getAllPendingDefs returns all pending definitions from the proof directory.
func getAllPendingDefs(proofDir string) ([]*node.PendingDef, error) {
	// List pending def node IDs from filesystem
	nodeIDs, err := fs.ListPendingDefs(proofDir)
	if err != nil {
		return nil, err
	}

	// Load each pending def
	pendingDefs := make([]*node.PendingDef, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		pd, err := fs.ReadPendingDef(proofDir, nodeID)
		if err != nil {
			return nil, err
		}
		pendingDefs = append(pendingDefs, pd)
	}

	return pendingDefs, nil
}

// findPendingDef searches for a pending definition by term, node ID, or pending def ID.
func findPendingDef(pendingDefs []*node.PendingDef, lookup string) *node.PendingDef {
	// First, try exact term match
	for _, pd := range pendingDefs {
		if pd.Term == lookup {
			return pd
		}
	}

	// Try node ID match
	nodeID, err := types.Parse(lookup)
	if err == nil {
		for _, pd := range pendingDefs {
			if pd.RequestedBy.String() == nodeID.String() {
				return pd
			}
		}
	}

	// Try exact ID match
	for _, pd := range pendingDefs {
		if pd.ID == lookup {
			return pd
		}
	}

	// Try partial ID match (prefix)
	for _, pd := range pendingDefs {
		if strings.HasPrefix(pd.ID, lookup) {
			return pd
		}
	}

	// Try case-insensitive term match
	lowerLookup := strings.ToLower(lookup)
	for _, pd := range pendingDefs {
		if strings.ToLower(pd.Term) == lowerLookup {
			return pd
		}
	}

	return nil
}

// outputPendingDefsJSON outputs pending definitions in JSON format.
func outputPendingDefsJSON(cmd *cobra.Command, pendingDefs []*node.PendingDef) error {
	output := make([]map[string]interface{}, 0, len(pendingDefs))
	for _, pd := range pendingDefs {
		output = append(output, pendingDefToJSON(pd))
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputPendingDefsText outputs pending definitions in text format.
func outputPendingDefsText(cmd *cobra.Command, pendingDefs []*node.PendingDef) error {
	if len(pendingDefs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No pending definition requests.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af request-def  - Request a new definition")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Pending Definition Requests (%d):\n\n", len(pendingDefs))

	for _, pd := range pendingDefs {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (node %s) - %s\n", pd.Term, pd.RequestedBy.String(), pd.Status)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af pending-def <term>  - Show details of a specific pending definition")
	fmt.Fprintln(cmd.OutOrStdout(), "  af add-def             - Add a definition to resolve a pending request")

	return nil
}

// outputPendingDefJSON outputs a single pending definition in JSON format.
func outputPendingDefJSON(cmd *cobra.Command, pd *node.PendingDef, full bool) error {
	output := pendingDefToJSON(pd)

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputPendingDefText outputs a single pending definition in text format.
func outputPendingDefText(cmd *cobra.Command, pd *node.PendingDef, full bool) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Pending Definition: %s\n\n", pd.Term)
	fmt.Fprintf(cmd.OutOrStdout(), "  Requested by: node %s\n", pd.RequestedBy.String())
	fmt.Fprintf(cmd.OutOrStdout(), "  Status:       %s\n", pd.Status)

	if full {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "  ID:           %s\n", pd.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Created:      %s\n", pd.Created.String())
		if pd.ResolvedBy != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Resolved by:  %s\n", pd.ResolvedBy)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af pending-defs  - List all pending definitions")
	fmt.Fprintln(cmd.OutOrStdout(), "  af add-def       - Add a definition to resolve this request")

	return nil
}

// pendingDefToJSON converts a pending definition to a JSON-friendly map.
func pendingDefToJSON(pd *node.PendingDef) map[string]interface{} {
	result := map[string]interface{}{
		"id":           pd.ID,
		"term":         pd.Term,
		"requested_by": pd.RequestedBy.String(),
		"status":       string(pd.Status),
		"created":      pd.Created.String(),
	}

	if pd.ResolvedBy != "" {
		result["resolved_by"] = pd.ResolvedBy
	}

	return result
}

func init() {
	rootCmd.AddCommand(newPendingDefsCmd())
	rootCmd.AddCommand(newPendingDefCmd())
}
