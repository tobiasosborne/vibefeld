// Package main contains the af types command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/schema"
)

// newTypesCmd creates the types command.
func newTypesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "types",
		Short: "List valid node types for proof refinement",
		Long: `List all valid node types that can be used with 'af refine -T TYPE'.

Node types define the role of each step in a proof:

  claim            - A mathematical assertion to be justified
  local_assume     - Introduce a local hypothesis (opens scope)
  local_discharge  - Conclude from local hypothesis (closes scope)
  case             - One branch of a case split
  qed              - Final step concluding the proof or subproof

Examples:
  af types                List all node types with descriptions
  af types --format json  Output in JSON format for scripting`,
		RunE: runTypes,
	}

	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// runTypes executes the types command.
func runTypes(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Get all node types from schema
	nodeTypes := schema.AllNodeTypes()

	// Output based on format
	if format == "json" {
		return outputTypesJSON(cmd, nodeTypes)
	}

	return outputTypesText(cmd, nodeTypes)
}

// typesOutputJSON is the JSON representation of a node type for the types command.
type typesOutputJSON struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	OpensScope  bool   `json:"opens_scope"`
	ClosesScope bool   `json:"closes_scope"`
}

// typesResultJSON is the JSON wrapper for types output.
type typesResultJSON struct {
	Types []typesOutputJSON `json:"types"`
	Total int               `json:"total"`
}

// outputTypesJSON outputs node types in JSON format.
func outputTypesJSON(cmd *cobra.Command, nodeTypes []schema.NodeTypeInfo) error {
	result := typesResultJSON{
		Types: make([]typesOutputJSON, 0, len(nodeTypes)),
		Total: len(nodeTypes),
	}

	for _, nt := range nodeTypes {
		result.Types = append(result.Types, typesOutputJSON{
			Type:        string(nt.ID),
			Description: nt.Description,
			OpensScope:  nt.OpensScope,
			ClosesScope: nt.ClosesScope,
		})
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputTypesText outputs node types in human-readable text format.
func outputTypesText(cmd *cobra.Command, nodeTypes []schema.NodeTypeInfo) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Valid Node Types (%d):\n\n", len(nodeTypes))

	for _, nt := range nodeTypes {
		// Show type name
		fmt.Fprintf(cmd.OutOrStdout(), "  %-18s %s\n", string(nt.ID), nt.Description)

		// Add scope info if relevant
		if nt.OpensScope {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-18s (opens scope)\n", "")
		}
		if nt.ClosesScope {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-18s (closes scope)\n", "")
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Usage:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af refine -T <type> <node> <statement>")

	return nil
}

func init() {
	rootCmd.AddCommand(newTypesCmd())
}
