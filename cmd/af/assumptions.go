// Package main contains the af assumptions and assumption commands implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
)

// newAssumptionsCmd creates the assumptions command for listing assumptions.
func newAssumptionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "assumptions [node-id]",
		GroupID: GroupQuery,
		Short:   "List assumptions in the proof",
		Long: `List all assumptions in the proof, or assumptions in scope for a specific node.

When called without arguments, lists all global assumptions in the proof.
When called with a node ID, lists assumptions in scope for that node.

Examples:
  af assumptions                  List all assumptions
  af assumptions 1                List assumptions in scope for node 1
  af assumptions 1.2 --format json  List assumptions for node 1.2 in JSON format`,
		Args: cobra.MaximumNArgs(1),
		RunE: runAssumptions,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// newAssumptionCmd creates the assumption command for showing a single assumption.
func newAssumptionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "assumption <id>",
		GroupID: GroupQuery,
		Short:   "Show a specific assumption by ID",
		Long: `Show detailed information about a specific assumption.

The assumption can be identified by its full ID or a unique prefix.

Examples:
  af assumption abc123              Show assumption with ID abc123
  af assumption abc123 --format json  Show assumption in JSON format`,
		Args: cobra.ExactArgs(1),
		RunE: runAssumption,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// runAssumptions executes the assumptions command.
func runAssumptions(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("proof not initialized")
	}

	// Load state to check nodes if needed
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// If a node ID was provided, validate it exists
	var nodeID *service.NodeID
	if len(args) > 0 {
		id, err := service.ParseNodeID(args[0])
		if err != nil {
			return fmt.Errorf("invalid node ID %q: %v", args[0], err)
		}
		nodeID = &id

		// Check if node exists
		if st.GetNode(id) == nil {
			return fmt.Errorf("node %q does not exist", args[0])
		}
	}

	// Get all assumptions
	assumptions, err := getAllAssumptions(svc.Path())
	if err != nil {
		return fmt.Errorf("error loading assumptions: %w", err)
	}

	// Filter by scope if node ID provided
	// For now, all global assumptions are in scope for all nodes
	// (scope filtering for local assumptions would be more complex)
	_ = nodeID // Currently all global assumptions are in scope

	// Output based on format
	if format == "json" {
		return outputAssumptionsJSON(cmd, assumptions)
	}

	return outputAssumptionsText(cmd, assumptions)
}

// runAssumption executes the assumption command (single assumption).
func runAssumption(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	assumptionID := args[0]
	if strings.TrimSpace(assumptionID) == "" {
		return fmt.Errorf("assumption ID cannot be empty")
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

	// Try to find the assumption
	assumption, err := findAssumption(svc.Path(), assumptionID)
	if err != nil {
		return err
	}

	// Output based on format
	if format == "json" {
		return outputAssumptionJSON(cmd, assumption)
	}

	return outputAssumptionText(cmd, assumption)
}

// getAllAssumptions returns all assumptions from the proof directory.
func getAllAssumptions(proofDir string) ([]*node.Assumption, error) {
	// List assumption IDs from filesystem
	ids, err := fs.ListAssumptions(proofDir)
	if err != nil {
		// If assumptions directory doesn't exist, return empty list
		if strings.Contains(err.Error(), "no such file or directory") {
			return []*node.Assumption{}, nil
		}
		return nil, err
	}

	// Load each assumption
	assumptions := make([]*node.Assumption, 0, len(ids))
	for _, id := range ids {
		asm, err := fs.ReadAssumption(proofDir, id)
		if err != nil {
			return nil, err
		}
		assumptions = append(assumptions, asm)
	}

	return assumptions, nil
}

// findAssumption finds an assumption by ID or partial ID.
func findAssumption(proofDir, searchID string) (*node.Assumption, error) {
	// First try exact match
	asm, err := fs.ReadAssumption(proofDir, searchID)
	if err == nil {
		return asm, nil
	}

	// Try partial match
	ids, err := fs.ListAssumptions(proofDir)
	if err != nil {
		return nil, fmt.Errorf("assumption %q not found", searchID)
	}

	var matches []*node.Assumption
	for _, id := range ids {
		if strings.HasPrefix(id, searchID) {
			asm, err := fs.ReadAssumption(proofDir, id)
			if err == nil {
				matches = append(matches, asm)
			}
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("assumption %q not found", searchID)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous assumption ID %q: multiple matches found", searchID)
	}

	return matches[0], nil
}

// outputAssumptionsJSON outputs assumptions in JSON format.
func outputAssumptionsJSON(cmd *cobra.Command, assumptions []*node.Assumption) error {
	// Create JSON output
	output := make([]map[string]interface{}, 0, len(assumptions))
	for _, asm := range assumptions {
		output = append(output, assumptionToJSON(asm))
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputAssumptionsText outputs assumptions in text format.
func outputAssumptionsText(cmd *cobra.Command, assumptions []*node.Assumption) error {
	if len(assumptions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No assumptions in this proof.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af add-assumption  - Add an assumption to the proof")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Assumptions (%d):\n\n", len(assumptions))

	for i, asm := range assumptions {
		fmt.Fprintf(cmd.OutOrStdout(), "  %d. [%s] %s\n", i+1, asm.ID[:8], asm.Statement)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af assumption <id>  - Show details of a specific assumption")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status           - View proof status")

	return nil
}

// outputAssumptionJSON outputs a single assumption in JSON format.
func outputAssumptionJSON(cmd *cobra.Command, asm *node.Assumption) error {
	output := assumptionToJSON(asm)

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputAssumptionText outputs a single assumption in text format.
func outputAssumptionText(cmd *cobra.Command, asm *node.Assumption) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Assumption: %s\n\n", asm.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "  Statement:    %s\n", asm.Statement)
	fmt.Fprintf(cmd.OutOrStdout(), "  Content Hash: %s\n", asm.ContentHash)
	fmt.Fprintf(cmd.OutOrStdout(), "  Created:      %s\n", asm.Created.String())

	if asm.Justification != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Justification: %s\n", asm.Justification)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af assumptions   - List all assumptions")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status        - View proof status")

	return nil
}

// assumptionToJSON converts an assumption to a JSON-friendly map.
func assumptionToJSON(asm *node.Assumption) map[string]interface{} {
	result := map[string]interface{}{
		"id":           asm.ID,
		"statement":    asm.Statement,
		"content_hash": asm.ContentHash,
		"created":      asm.Created.String(),
	}

	if asm.Justification != "" {
		result["justification"] = asm.Justification
	}

	return result
}

func init() {
	rootCmd.AddCommand(newAssumptionsCmd())
	rootCmd.AddCommand(newAssumptionCmd())
}
