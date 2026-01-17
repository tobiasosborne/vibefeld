package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/scope"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newScopeCmd creates the scope command for showing scope information.
func newScopeCmd() *cobra.Command {
	var dir string
	var format string
	var showAll bool

	cmd := &cobra.Command{
		Use:   "scope [node-id]",
		Short: "Show scope information for a node",
		Long: `Show assumption scope information for a proof node.

When called with a node ID, shows:
  - Whether the node is inside any assumption scope
  - The scope depth (number of nested scopes)
  - List of containing assumption scopes

When called with --all, shows all active assumption scopes in the proof.

Scopes are opened by local_assume nodes and closed when the assumption
is discharged (e.g., when a contradiction is derived).

Examples:
  af scope 1.2.3              Show scope info for node 1.2.3
  af scope 1.2.3 --format json  Show scope info in JSON format
  af scope --all              Show all active scopes
  af scope --all --format json  Show all scopes in JSON format`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if showAll {
				return runScopeAll(cmd, dir, format)
			}
			if len(args) == 0 {
				return fmt.Errorf("node ID required (or use --all to show all scopes)")
			}
			return runScope(cmd, args[0], dir, format)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text/json)")
	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all active scopes")

	return cmd
}

// runScope shows scope information for a specific node.
func runScope(cmd *cobra.Command, nodeIDStr, dir, format string) error {
	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Parse node ID
	nodeID, err := types.Parse(nodeIDStr)
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %v", nodeIDStr, err)
	}

	// Create service
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

	// Load state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Check if node exists
	n := st.GetNode(nodeID)
	if n == nil {
		return fmt.Errorf("node %q does not exist", nodeIDStr)
	}

	// Get scope info
	info := st.GetScopeInfo(nodeID)

	// Output based on format
	if format == "json" {
		return outputScopeJSON(cmd, nodeID, info)
	}

	return outputScopeText(cmd, nodeID, info)
}

// runScopeAll shows all active scopes in the proof.
func runScopeAll(cmd *cobra.Command, dir, format string) error {
	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Create service
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

	// Load state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Get all scopes
	allScopes := st.GetAllScopes()
	activeScopes := st.GetActiveScopes()

	// Output based on format
	if format == "json" {
		return outputAllScopesJSON(cmd, allScopes, activeScopes)
	}

	return outputAllScopesText(cmd, allScopes, activeScopes)
}

// outputScopeJSON outputs scope info for a node in JSON format.
func outputScopeJSON(cmd *cobra.Command, nodeID types.NodeID, info *scope.ScopeInfo) error {
	result := map[string]interface{}{
		"node_id": nodeID.String(),
		"depth":   info.Depth,
		"in_scope": info.IsInAnyScope(),
	}

	if len(info.ContainingScopes) > 0 {
		scopes := make([]map[string]interface{}, len(info.ContainingScopes))
		for i, s := range info.ContainingScopes {
			scopes[i] = map[string]interface{}{
				"node_id":   s.NodeID.String(),
				"statement": s.Statement,
				"active":    s.IsActive(),
			}
		}
		result["containing_scopes"] = scopes
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	cmd.Println(string(data))
	return nil
}

// outputScopeText outputs scope info for a node in text format.
func outputScopeText(cmd *cobra.Command, nodeID types.NodeID, info *scope.ScopeInfo) error {
	cmd.Printf("Scope information for node %s:\n", nodeID.String())
	cmd.Printf("  Scope depth: %d\n", info.Depth)

	if !info.IsInAnyScope() {
		cmd.Println("  Node is not inside any scope.")
		return nil
	}

	cmd.Println("  Containing scopes (outermost to innermost):")
	for i, s := range info.ContainingScopes {
		status := "active"
		if !s.IsActive() {
			status = "closed"
		}
		cmd.Printf("    %d. [%s] %s: %q\n", i+1, s.NodeID.String(), status, s.Statement)
	}

	return nil
}

// outputAllScopesJSON outputs all scopes in JSON format.
func outputAllScopesJSON(cmd *cobra.Command, allScopes, activeScopes []*scope.Entry) error {
	result := map[string]interface{}{
		"total_count":  len(allScopes),
		"active_count": len(activeScopes),
	}

	if len(allScopes) > 0 {
		scopes := make([]map[string]interface{}, len(allScopes))
		for i, s := range allScopes {
			scopes[i] = map[string]interface{}{
				"node_id":    s.NodeID.String(),
				"statement":  s.Statement,
				"active":     s.IsActive(),
				"introduced": s.Introduced.String(),
			}
			if s.Discharged != nil {
				scopes[i]["discharged"] = s.Discharged.String()
			}
		}
		result["scopes"] = scopes
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	cmd.Println(string(data))
	return nil
}

// outputAllScopesText outputs all scopes in text format.
func outputAllScopesText(cmd *cobra.Command, allScopes, activeScopes []*scope.Entry) error {
	cmd.Printf("Assumption Scopes: %d total, %d active\n\n", len(allScopes), len(activeScopes))

	if len(allScopes) == 0 {
		cmd.Println("No assumption scopes in this proof.")
		return nil
	}

	// Show active scopes
	if len(activeScopes) > 0 {
		cmd.Println("Active Scopes:")
		for _, s := range activeScopes {
			cmd.Printf("  [%s] %q\n", s.NodeID.String(), s.Statement)
		}
		cmd.Println()
	}

	// Show closed scopes
	var closedScopes []*scope.Entry
	for _, s := range allScopes {
		if !s.IsActive() {
			closedScopes = append(closedScopes, s)
		}
	}

	if len(closedScopes) > 0 {
		cmd.Println("Closed Scopes:")
		for _, s := range closedScopes {
			cmd.Printf("  [%s] %q (closed)\n", s.NodeID.String(), s.Statement)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newScopeCmd())
}
