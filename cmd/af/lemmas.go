// Package main contains the af lemmas and af lemma commands for viewing lemmas.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
)

// newLemmasCmd creates the lemmas command for listing all lemmas.
func newLemmasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lemmas",
		Short: "List all lemmas in the proof",
		Long: `List all lemmas that have been extracted from the proof.

Lemmas are reusable proof fragments extracted from validated nodes.
This command displays all lemmas with their IDs, statements, and source nodes.

Examples:
  af lemmas                     List all lemmas
  af lemmas --format json       Output in JSON format
  af lemmas -d /path/to/proof   List lemmas from specific directory`,
		RunE: runLemmas,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// newLemmaCmd creates the lemma command for showing a specific lemma.
func newLemmaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lemma <id>",
		Short: "Show a specific lemma by ID",
		Long: `Show detailed information about a specific lemma.

The lemma can be identified by its full ID or a unique prefix.

Examples:
  af lemma LEM-abc123              Show lemma with ID LEM-abc123
  af lemma LEM-abc123 --format json  Show lemma in JSON format
  af lemma LEM-abc123 --full         Show full lemma details`,
		Args: cobra.ExactArgs(1),
		RunE: runLemma,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().BoolP("full", "F", false, "Show full lemma details")

	return cmd
}

// runLemmas executes the lemmas command.
func runLemmas(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")

	// Handle empty directory
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("directory path cannot be empty")
	}

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

	// Get all lemmas from state
	lemmas, err := getAllLemmasFromService(svc)
	if err != nil {
		return fmt.Errorf("error loading lemmas: %w", err)
	}

	// Output based on format
	if format == "json" {
		return outputLemmasJSON(cmd, lemmas)
	}

	return outputLemmasText(cmd, lemmas)
}

// runLemma executes the lemma command (single lemma).
func runLemma(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	full, _ := cmd.Flags().GetBool("full")

	lemmaID := args[0]
	if strings.TrimSpace(lemmaID) == "" {
		return fmt.Errorf("lemma ID cannot be empty")
	}

	// Handle empty directory
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("directory path cannot be empty")
	}

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

	// Try to find the lemma from state
	lemma, err := findLemmaFromService(svc, lemmaID)
	if err != nil {
		return err
	}

	// Output based on format
	if format == "json" {
		return outputLemmaJSON(cmd, lemma, full)
	}

	return outputLemmaText(cmd, lemma, full)
}

// getAllLemmasFromService returns all lemmas from the service's state (replayed from ledger).
func getAllLemmasFromService(svc *service.ProofService) ([]*node.Lemma, error) {
	st, err := svc.LoadState()
	if err != nil {
		return nil, err
	}
	return st.AllLemmas(), nil
}

// findLemmaFromService finds a lemma by ID or partial ID from the service's state.
func findLemmaFromService(svc *service.ProofService, searchID string) (*node.Lemma, error) {
	st, err := svc.LoadState()
	if err != nil {
		return nil, err
	}

	// First try exact match
	lem := st.GetLemma(searchID)
	if lem != nil {
		return lem, nil
	}

	// Try partial match
	allLemmas := st.AllLemmas()
	var matches []*node.Lemma
	for _, l := range allLemmas {
		if strings.HasPrefix(l.ID, searchID) {
			matches = append(matches, l)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("lemma %q not found", searchID)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous lemma ID %q: multiple matches found", searchID)
	}

	return matches[0], nil
}

// outputLemmasJSON outputs lemmas in JSON format.
func outputLemmasJSON(cmd *cobra.Command, lemmas []*node.Lemma) error {
	// Create JSON output
	output := make([]map[string]interface{}, 0, len(lemmas))
	for _, lem := range lemmas {
		output = append(output, lemmaToJSON(lem))
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputLemmasText outputs lemmas in text format.
func outputLemmasText(cmd *cobra.Command, lemmas []*node.Lemma) error {
	if len(lemmas) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No lemmas extracted yet.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af extract-lemma  - Extract a lemma from a validated node")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Lemmas (%d):\n\n", len(lemmas))

	for i, lem := range lemmas {
		// Truncate ID for display (show first 12 chars like "LEM-xxxxxxxx")
		displayID := lem.ID
		if len(displayID) > 20 {
			displayID = displayID[:20]
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %d. [%s] %s\n", i+1, displayID, lem.Statement)
		fmt.Fprintf(cmd.OutOrStdout(), "     Source: %s\n", lem.SourceNodeID.String())
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af lemma <id>  - Show details of a specific lemma")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status      - View proof status")

	return nil
}

// outputLemmaJSON outputs a single lemma in JSON format.
func outputLemmaJSON(cmd *cobra.Command, lem *node.Lemma, full bool) error {
	output := map[string]interface{}{
		"id":             lem.ID,
		"statement":      lem.Statement,
		"source_node_id": lem.SourceNodeID.String(),
	}

	if full {
		output["content_hash"] = lem.ContentHash
		output["created"] = lem.Created.String()
		if lem.Proof != "" {
			output["proof"] = lem.Proof
		}
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputLemmaText outputs a single lemma in text format.
func outputLemmaText(cmd *cobra.Command, lem *node.Lemma, full bool) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Lemma: %s\n\n", lem.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "  Statement:   %s\n", lem.Statement)
	fmt.Fprintf(cmd.OutOrStdout(), "  Source Node: %s\n", lem.SourceNodeID.String())

	if full {
		fmt.Fprintf(cmd.OutOrStdout(), "  Content Hash: %s\n", lem.ContentHash)
		fmt.Fprintf(cmd.OutOrStdout(), "  Created:      %s\n", lem.Created.String())
		if lem.Proof != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Proof:        %s\n", lem.Proof)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af lemmas    - List all lemmas")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status    - View proof status")

	return nil
}

// lemmaToJSON converts a lemma to a JSON-friendly map.
func lemmaToJSON(lem *node.Lemma) map[string]interface{} {
	result := map[string]interface{}{
		"id":             lem.ID,
		"statement":      lem.Statement,
		"source_node_id": lem.SourceNodeID.String(),
		"content_hash":   lem.ContentHash,
		"created":        lem.Created.String(),
	}

	if lem.Proof != "" {
		result["proof"] = lem.Proof
	}

	return result
}

func init() {
	rootCmd.AddCommand(newLemmasCmd())
	rootCmd.AddCommand(newLemmaCmd())
}
