package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// CriticalPath represents a root-to-leaf path ranked by unvalidated node count.
type CriticalPath struct {
	Path             []string `json:"path"`
	TotalDepth       int      `json:"total_depth"`
	UnvalidatedCount int      `json:"unvalidated_count"`
	EpistemicStates  []string `json:"epistemic_states"`
}

func newCriticalPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "critical-path",
		GroupID: GroupQuery,
		Short:   "Show longest unvalidated dependency chains blocking proof completion",
		Long: `Show the critical paths in the proof tree — the longest chains of unvalidated
nodes from root to leaf. These represent the bottlenecks blocking proof completion.

Each path is ranked by how many unvalidated nodes it contains (pending, draft,
or needs_refinement). Validated, admitted, archived, and refuted nodes don't count.

Examples:
  af critical-path                     Show top 3 critical paths
  af critical-path --limit 5           Show top 5 critical paths
  af critical-path --focus 1.3         Critical paths within subtree 1.3
  af critical-path --format json       Output in JSON format`,
		RunE: runCriticalPath,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().IntP("limit", "l", 3, "Maximum number of paths to display")
	cmd.Flags().String("focus", "", "Show only paths within subtree rooted at this node")

	return cmd
}

func runCriticalPath(cmd *cobra.Command, args []string) error {
	dir := service.MustString(cmd, "dir")
	format := service.MustString(cmd, "format")
	limit := service.MustInt(cmd, "limit")
	focus, _ := cmd.Flags().GetString("focus")

	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}
	if limit < 1 {
		return fmt.Errorf("invalid limit %d: must be at least 1", limit)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		if format == "json" {
			fmt.Fprintln(cmd.OutOrStdout(), `{"error":"proof not initialized"}`)
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No proof initialized. Run 'af init' to start a new proof.")
		return nil
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	nodes := st.AllNodes()
	if len(nodes) == 0 {
		if format == "json" {
			fmt.Fprintln(cmd.OutOrStdout(), `{"critical_paths":[],"summary":"No nodes in proof."}`)
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No nodes in proof.")
		return nil
	}

	// Apply focus filter
	if focus != "" {
		focusID, err := service.ParseNodeID(focus)
		if err != nil {
			return fmt.Errorf("invalid focus node ID %q: %w", focus, err)
		}
		var filtered []*node.Node
		for _, n := range nodes {
			if n.ID.Equal(focusID) || focusID.IsAncestorOf(n.ID) {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
		if len(nodes) == 0 {
			return fmt.Errorf("node %s not found", focus)
		}
	}

	paths := findCriticalPaths(nodes, st)

	if limit < len(paths) {
		paths = paths[:limit]
	}

	if format == "json" {
		return outputCriticalPathJSON(cmd, paths)
	}
	return outputCriticalPathText(cmd, paths, st, focus)
}

// findCriticalPaths finds all root-to-leaf paths and ranks them by unvalidated node count.
func findCriticalPaths(nodes []*node.Node, st *service.State) []CriticalPath {
	// Build set of all node IDs for child detection
	nodeSet := make(map[string]*node.Node, len(nodes))
	for _, n := range nodes {
		nodeSet[n.ID.String()] = n
	}

	// Find leaf nodes (nodes with no children in the current node set)
	hasChild := make(map[string]bool)
	for _, n := range nodes {
		parent, ok := n.ID.Parent()
		if ok {
			hasChild[parent.String()] = true
		}
	}

	var paths []CriticalPath
	for _, n := range nodes {
		if hasChild[n.ID.String()] {
			continue // not a leaf
		}

		// Build path from root to this leaf
		path := buildPathToRoot(n.ID, nodeSet)
		unvalidated := 0
		epistemicStates := make([]string, len(path))
		for i, id := range path {
			pn := nodeSet[id]
			if pn != nil {
				epistemicStates[i] = string(pn.EpistemicState)
				if isUnvalidated(pn.EpistemicState) {
					unvalidated++
				}
			}
		}

		paths = append(paths, CriticalPath{
			Path:             path,
			TotalDepth:       len(path),
			UnvalidatedCount: unvalidated,
			EpistemicStates:  epistemicStates,
		})
	}

	// Sort: most unvalidated first, then by total depth (longest first)
	sort.Slice(paths, func(i, j int) bool {
		if paths[i].UnvalidatedCount != paths[j].UnvalidatedCount {
			return paths[i].UnvalidatedCount > paths[j].UnvalidatedCount
		}
		return paths[i].TotalDepth > paths[j].TotalDepth
	})

	return paths
}

// buildPathToRoot walks from a node up to the root, collecting IDs.
// Only includes nodes present in nodeSet (respects focus filtering).
func buildPathToRoot(id types.NodeID, nodeSet map[string]*node.Node) []string {
	var path []string
	current := id
	for {
		if nodeSet[current.String()] != nil {
			path = append([]string{current.String()}, path...)
		}
		parent, ok := current.Parent()
		if !ok {
			break
		}
		current = parent
	}
	return path
}

// isUnvalidated returns true if the epistemic state represents incomplete work.
func isUnvalidated(state schema.EpistemicState) bool {
	switch state {
	case schema.EpistemicPending, schema.EpistemicDraft, schema.EpistemicNeedsRefinement:
		return true
	default:
		return false
	}
}

func outputCriticalPathJSON(cmd *cobra.Command, paths []CriticalPath) error {
	result := struct {
		CriticalPaths []CriticalPath `json:"critical_paths"`
		Count         int            `json:"count"`
	}{
		CriticalPaths: paths,
		Count:         len(paths),
	}
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func outputCriticalPathText(cmd *cobra.Command, paths []CriticalPath, st *service.State, focus string) error {
	out := cmd.OutOrStdout()

	if focus != "" {
		fmt.Fprintf(out, "=== Critical Paths (subtree: %s) ===\n\n", focus)
	} else {
		fmt.Fprintln(out, "=== Critical Paths ===")
		fmt.Fprintln(out)
	}

	if len(paths) == 0 {
		fmt.Fprintln(out, "No critical paths found. All branches are complete.")
		return nil
	}

	// Check if any paths have unvalidated nodes
	allComplete := true
	for _, p := range paths {
		if p.UnvalidatedCount > 0 {
			allComplete = false
			break
		}
	}

	if allComplete {
		fmt.Fprintln(out, "All paths are fully validated. Proof is complete!")
		return nil
	}

	for i, p := range paths {
		if p.UnvalidatedCount == 0 {
			continue // skip fully validated paths
		}
		fmt.Fprintf(out, "Path %d (depth %d, %d unvalidated):\n", i+1, p.TotalDepth, p.UnvalidatedCount)

		// Render each node in the path with colored state
		var parts []string
		for _, idStr := range p.Path {
			nodeID, _ := service.ParseNodeID(idStr)
			n := st.GetNode(nodeID)
			if n != nil {
				parts = append(parts, fmt.Sprintf("%s [%s]", idStr, render.ColorEpistemicState(n.EpistemicState)))
			} else {
				parts = append(parts, idStr)
			}
		}
		fmt.Fprintf(out, "  %s\n\n", strings.Join(parts, " → "))
	}

	// Recommendation: identify the most impactful node to work on
	if len(paths) > 0 && paths[0].UnvalidatedCount > 0 {
		fmt.Fprintln(out, "--- Recommendation ---")
		// Find the deepest unvalidated node on the worst path
		worst := paths[0]
		for i := len(worst.Path) - 1; i >= 0; i-- {
			if i < len(worst.EpistemicStates) && isUnvalidatedStr(worst.EpistemicStates[i]) {
				fmt.Fprintf(out, "Focus on node %s (%s) — deepest unvalidated on worst path.\n", worst.Path[i], worst.EpistemicStates[i])
				break
			}
		}
	}

	return nil
}

func isUnvalidatedStr(state string) bool {
	switch schema.EpistemicState(state) {
	case schema.EpistemicPending, schema.EpistemicDraft, schema.EpistemicNeedsRefinement:
		return true
	default:
		return false
	}
}

func init() {
	rootCmd.AddCommand(newCriticalPathCmd())
}
