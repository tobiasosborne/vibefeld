// Package main contains the af extract-lemma command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newExtractLemmaCmd creates the extract-lemma command for extracting lemmas from validated nodes.
func newExtractLemmaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract-lemma <node-id>",
		Short: "Extract a lemma from a validated node",
		Long: `Extract a reusable lemma from a validated proof node.

The node must be in 'validated' epistemic state before extraction.
The command validates that the node is independent (doesn't rely on
local assumptions from parent scopes).

The extracted lemma can be referenced by other proofs using its ID.

Examples:
  af extract-lemma 1 --statement "For all n >= 0, n! >= 1"
  af extract-lemma 1.2 -s "P implies Q" -d ./proof
  af extract-lemma 1.1.1 --statement "Base case" --format json`,
		Args: cobra.ExactArgs(1),
		RunE: runExtractLemma,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("statement", "s", "", "Lemma statement (required)")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")
	cmd.Flags().StringP("name", "n", "", "Custom lemma name (optional)")

	return cmd
}

// runExtractLemma executes the extract-lemma command.
func runExtractLemma(cmd *cobra.Command, args []string) error {
	examples := render.GetExamples("af extract-lemma")

	// Get flags
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	statement, err := cmd.Flags().GetString("statement")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	// Validate directory
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("directory path cannot be empty")
	}

	// Validate node ID argument
	nodeIDStr := args[0]
	if strings.TrimSpace(nodeIDStr) == "" {
		return render.InvalidNodeIDError("af extract-lemma", nodeIDStr, examples)
	}

	nodeID, err := types.Parse(nodeIDStr)
	if err != nil {
		return render.InvalidNodeIDError("af extract-lemma", nodeIDStr, examples)
	}

	// Validate statement is provided and not empty
	if strings.TrimSpace(statement) == "" {
		return render.NewUsageError("af extract-lemma",
			"--statement is required and cannot be empty",
			[]string{
				"af extract-lemma 1 --statement \"Your lemma statement\"",
				"af extract-lemma 1.2 -s \"P implies Q\"",
			})
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
		return fmt.Errorf("proof not initialized")
	}

	// Load state to check node exists and is validated
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	// Check if node exists
	node := st.GetNode(nodeID)
	if node == nil {
		return fmt.Errorf("node %s not found\n\nHint: Use 'af status' to see all available nodes.", nodeID.String())
	}

	// Check if node is validated
	if node.EpistemicState != schema.EpistemicValidated {
		return fmt.Errorf("node %s is not validated (current state: %s); only validated nodes can be extracted as lemmas",
			nodeID.String(), node.EpistemicState)
	}

	// Check for independence: node should not depend on local assumptions from parent scope
	// A node is independent if it has no scope entries (local assumptions)
	if len(node.Scope) > 0 {
		return fmt.Errorf("node %s is not independent: it depends on local assumptions (%s); lemmas cannot rely on local scope",
			nodeID.String(), strings.Join(node.Scope, ", "))
	}

	// Extract the lemma
	lemmaID, err := svc.ExtractLemma(nodeID, statement)
	if err != nil {
		return fmt.Errorf("error extracting lemma: %w", err)
	}

	// Output result based on format
	switch format {
	case "json":
		result := map[string]interface{}{
			"id":             lemmaID,
			"statement":      statement,
			"source_node_id": nodeID.String(),
			"extracted":      true,
		}
		output, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Lemma extracted successfully.\n\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  ID:          %s\n", lemmaID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Statement:   %s\n", statement)
		fmt.Fprintf(cmd.OutOrStdout(), "  Source Node: %s\n", nodeID.String())
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af lemmas     - List all lemmas")
		fmt.Fprintf(cmd.OutOrStdout(), "  af lemma %s - Show this lemma's details\n", lemmaID)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newExtractLemmaCmd())
}
