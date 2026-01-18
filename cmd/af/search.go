package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// newSearchCmd creates the search command for finding nodes by various criteria.
func newSearchCmd() *cobra.Command {
	var dir string
	var textQuery string
	var stateFilter string
	var workflowFilter string
	var defFilter string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search and filter nodes",
		Long: `Search for proof nodes by text content, state, or definition references.

Supports multiple filter criteria that can be combined:
  - Text search: Match nodes containing text in their statement
  - Epistemic state: Filter by pending, validated, admitted, refuted, or archived
  - Workflow state: Filter by available, claimed, or blocked
  - Definition reference: Find nodes referencing a specific definition

Examples:
  af search "convergence"              Search for nodes containing "convergence"
  af search --state pending            Show all pending nodes
  af search --workflow available       Show all available nodes
  af search --def "continuity"         Find nodes referencing definition "continuity"
  af search --state validated --json   Show validated nodes in JSON format
  af search -t "limit" -s pending      Combine text and state filters`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use positional argument as text query if provided and --text not specified
			query := textQuery
			if query == "" && len(args) > 0 {
				query = args[0]
			}
			return runSearch(cmd, dir, query, stateFilter, workflowFilter, defFilter, jsonOutput)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&textQuery, "text", "t", "", "Search node content/statements")
	cmd.Flags().StringVarP(&stateFilter, "state", "s", "", "Filter by epistemic state (pending/validated/admitted/refuted/archived)")
	cmd.Flags().StringVarP(&workflowFilter, "workflow", "w", "", "Filter by workflow state (available/claimed/blocked)")
	cmd.Flags().StringVar(&defFilter, "def", "", "Search nodes referencing a definition")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

func runSearch(cmd *cobra.Command, dir, textQuery, stateFilter, workflowFilter, defFilter string, jsonOutput bool) error {
	examples := render.GetExamples("af search")

	// Validate filters
	if stateFilter != "" {
		if err := schema.ValidateEpistemicState(stateFilter); err != nil {
			return render.InvalidValueError("af search", "state", stateFilter, render.ValidEpistemicStates, examples)
		}
	}

	if workflowFilter != "" {
		if err := schema.ValidateWorkflowState(workflowFilter); err != nil {
			return render.InvalidValueError("af search", "workflow", workflowFilter, render.ValidWorkflowStates, examples)
		}
	}

	// Check if at least one filter is specified
	if textQuery == "" && stateFilter == "" && workflowFilter == "" && defFilter == "" {
		return fmt.Errorf("at least one search criterion required: use --text, --state, --workflow, or --def")
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

	// Load state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Get all nodes
	allNodes := st.AllNodes()

	// Filter nodes based on criteria
	var results []render.SearchResult
	for _, n := range allNodes {
		if n == nil {
			continue
		}

		match, reason := matchesFilters(n, textQuery, stateFilter, workflowFilter, defFilter)
		if match {
			results = append(results, render.SearchResult{
				Node:        n,
				MatchReason: reason,
			})
		}
	}

	// Output results
	if jsonOutput {
		fmt.Fprintln(cmd.OutOrStdout(), render.FormatSearchResultsJSON(results))
	} else {
		fmt.Fprint(cmd.OutOrStdout(), render.FormatSearchResults(results))
	}

	return nil
}

// matchesFilters checks if a node matches all specified filters.
// Returns (matches bool, reason string).
func matchesFilters(n *node.Node, textQuery, stateFilter, workflowFilter, defFilter string) (bool, string) {
	var reasons []string

	// Text filter - case-insensitive search in statement
	if textQuery != "" {
		if !strings.Contains(strings.ToLower(n.Statement), strings.ToLower(textQuery)) {
			return false, ""
		}
		reasons = append(reasons, "text match")
	}

	// Epistemic state filter
	if stateFilter != "" {
		if string(n.EpistemicState) != stateFilter {
			return false, ""
		}
		reasons = append(reasons, "state: "+stateFilter)
	}

	// Workflow state filter
	if workflowFilter != "" {
		if string(n.WorkflowState) != workflowFilter {
			return false, ""
		}
		reasons = append(reasons, "workflow: "+workflowFilter)
	}

	// Definition reference filter - search in Context field
	if defFilter != "" {
		found := false
		lowerDef := strings.ToLower(defFilter)
		for _, ctx := range n.Context {
			// Context entries may be "def:name" or just "name"
			ctxLower := strings.ToLower(ctx)
			if strings.Contains(ctxLower, lowerDef) {
				found = true
				break
			}
		}
		if !found {
			return false, ""
		}
		reasons = append(reasons, "refs def: "+defFilter)
	}

	// If we get here, all specified filters matched
	return true, strings.Join(reasons, ", ")
}

func init() {
	rootCmd.AddCommand(newSearchCmd())
}
