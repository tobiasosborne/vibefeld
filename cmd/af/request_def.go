// Package main contains the af request-def command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
)

// newRequestDefCmd creates the request-def command.
func newRequestDefCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "request-def",
		GroupID: GroupProver,
		Short:   "Request a definition for a term",
		Long: `Request a definition for a term that is needed during proof work.

This is a prover action that requests a formal definition for a term.
When an agent encounters a term that requires a formal definition,
they can request that definition using this command. The request
creates a pending definition that can be fulfilled by another agent.

Examples:
  af request-def --node 1 --term "group"
  af request-def -n 1.2 -t "homomorphism" -d ./proof
  af request-def --node 1 --term "kernel" --format json`,
		RunE: runRequestDef,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().StringP("node", "n", "", "Node ID requesting the definition (required)")
	cmd.Flags().StringP("term", "t", "", "Term to define (required)")

	return cmd
}

// runRequestDef executes the request-def command.
func runRequestDef(cmd *cobra.Command, args []string) error {
	// Get flags
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	nodeIDStr := cli.MustString(cmd, "node")
	term := cli.MustString(cmd, "term")

	// Validate node ID is provided
	if strings.TrimSpace(nodeIDStr) == "" {
		return fmt.Errorf("node is required")
	}

	// Parse node ID
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", nodeIDStr, err)
	}

	// Validate term is provided and not empty/whitespace
	if strings.TrimSpace(term) == "" {
		return fmt.Errorf("term is required and cannot be empty")
	}

	// Create proof service to check state
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Load state to check if node exists
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Check if node exists
	n := st.GetNode(nodeID)
	if n == nil {
		return fmt.Errorf("node %s does not exist", nodeID.String())
	}

	// Create the pending definition request
	pd, err := node.NewPendingDefWithValidation(term, nodeID)
	if err != nil {
		return fmt.Errorf("error creating pending definition: %w", err)
	}

	// Write the pending definition to filesystem
	if err := svc.WritePendingDef(nodeID, pd); err != nil {
		return fmt.Errorf("error writing pending definition: %w", err)
	}

	// Output result based on format
	switch strings.ToLower(format) {
	case "json":
		return outputRequestDefJSON(cmd, nodeID, term, pd)
	default:
		return outputRequestDefText(cmd, nodeID, term, pd)
	}
}

// outputRequestDefJSON outputs the request-def result in JSON format.
func outputRequestDefJSON(cmd *cobra.Command, nodeID service.NodeID, term string, pd *node.PendingDef) error {
	result := map[string]interface{}{
		"node_id": nodeID.String(),
		"term":    term,
		"status":  string(pd.Status),
		"id":      pd.ID,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputRequestDefText outputs the request-def result in human-readable text format.
func outputRequestDefText(cmd *cobra.Command, nodeID service.NodeID, term string, pd *node.PendingDef) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Definition request created for node %s\n", nodeID.String())
	fmt.Fprintf(cmd.OutOrStdout(), "  Term:   %s\n", term)
	fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", pd.Status)
	fmt.Fprintf(cmd.OutOrStdout(), "  ID:     %s\n", pd.ID)
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af add-def  - Add a definition to fulfill this request")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status   - Check the pending definition status")

	return nil
}

func init() {
	rootCmd.AddCommand(newRequestDefCmd())
}
