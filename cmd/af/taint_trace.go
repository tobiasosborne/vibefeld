package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

func newTaintTraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "taint-trace <node-id>",
		GroupID: GroupQuery,
		Short:   "Show taint propagation path for a node",
		Long: `Trace the source of taint for a proof node.

Shows the target's ancestry chain and identifies the nearest source of taint
or unresolved state in either its ancestor chain or active descendant subtree.

Taint rules:
  - Archived/refuted nodes are always clean (severed from proof)
  - Pending/draft/needs_refinement nodes are unresolved
  - Admitted nodes are self_admitted (accepted without full proof)
  - Pending/draft/needs_refinement ancestors or descendants make a node unresolved
  - Admitted ancestors or descendants taint a validated node
  - An admitted node ignores its own subtree
  - Descendant-derived taint never contaminates validated siblings
  - Validated nodes with no active source are clean

Examples:
  af taint-trace 1.6.4      Show why node 1.6.4 is tainted
  af taint-trace 1.2 -f json  JSON output for machine consumption`,
		Args: cobra.ExactArgs(1),
		RunE: runTaintTrace,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

// traceEntry represents one node in the taint trace.
type traceEntry struct {
	NodeID            string             `json:"node_id"`
	EpistemicState    string             `json:"epistemic_state"`
	TaintState        string             `json:"taint_state"`
	Reason            string             `json:"reason"`
	IsSource          bool               `json:"is_source"`
	DescendantSources []descendantSource `json:"descendant_sources,omitempty"`
}

type descendantSource struct {
	NodeID         string `json:"node_id"`
	EpistemicState string `json:"epistemic_state"`
	TaintState     string `json:"taint_state"`
}

type taintTraceIndex struct {
	nodes    map[string]*node.Node
	parent   map[string]*node.Node
	children map[string][]*node.Node
}

func runTaintTrace(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af taint-trace")
		return render.InvalidNodeIDError("af taint-trace", nodeIDStr, examples)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	target := st.GetNode(nodeID)
	if target == nil {
		return fmt.Errorf("node %s not found", nodeID.String())
	}
	allNodes := st.AllNodes()
	index := newTaintTraceIndex(allNodes)

	// Build ancestry chain (root first, target last)
	var chain []*node.Node
	for n := target; n != nil; n = index.parent[n.ID.String()] {
		chain = append([]*node.Node{n}, chain...)
	}

	// Compute trace: for each node in the chain, determine its taint reason
	entries := make([]traceEntry, len(chain))
	for i, n := range chain {
		entry := traceEntry{
			NodeID:         n.ID.String(),
			EpistemicState: string(n.EpistemicState),
			TaintState:     string(n.TaintState),
		}

		entry.Reason, entry.IsSource, entry.DescendantSources = taintReason(n, chain[:i], index)
		entries[i] = entry
	}

	switch strings.ToLower(format) {
	case "json":
		return outputTaintTraceJSON(cmd, target, entries)
	default:
		return outputTaintTraceText(cmd, target, entries)
	}
}

// taintReason determines why a node has its current taint state. ancestors is
// the ancestry chain before this node (root to parent).
func taintReason(n *node.Node, ancestors []*node.Node, index *taintTraceIndex) (reason string, isSource bool, descendants []descendantSource) {
	// Rule 0: Archived/refuted
	if n.EpistemicState == schema.EpistemicArchived {
		return "archived — severed from proof tree", false, nil
	}
	if n.EpistemicState == schema.EpistemicRefuted {
		return "refuted — severed from proof tree", false, nil
	}

	// Rule 1: Pending/draft/needs_refinement → unresolved (self-caused)
	if n.EpistemicState == schema.EpistemicPending {
		return "node is pending verification", true, nil
	}
	if n.EpistemicState == schema.EpistemicDraft {
		return "node is in draft state", true, nil
	}
	if n.EpistemicState == schema.EpistemicNeedsRefinement {
		return "node is reopened for refinement", true, nil
	}

	// Rule 2: Ancestor unresolved → propagated
	for i := len(ancestors) - 1; i >= 0; i-- {
		a := ancestors[i]
		if !traceSevered(a) && traceUnresolvedState(a.EpistemicState) {
			if a.EpistemicState == schema.EpistemicNeedsRefinement {
				return fmt.Sprintf("ancestor %s is reopened for refinement", a.ID.String()), false, nil
			}
			return fmt.Sprintf("ancestor %s is %s", a.ID.String(), a.EpistemicState), false, nil
		}
	}

	// Rule 3: Self-admitted
	if schema.IntroducesTaint(n.EpistemicState) {
		return fmt.Sprintf("node is %s — accepted without full proof", n.EpistemicState), true, nil
	}

	// Rule 4: Pending/draft/needs_refinement descendant → unresolved.
	if sources := nearestDescendantSources(n, index.children, node.TaintUnresolved); len(sources) > 0 {
		return descendantReason(sources), false, sources
	}

	// Rule 5: Admitted ancestor → tainted.
	for i := len(ancestors) - 1; i >= 0; i-- {
		a := ancestors[i]
		if !traceSevered(a) && schema.IntroducesTaint(a.EpistemicState) {
			return fmt.Sprintf("ancestor %s is %s", a.ID.String(), a.EpistemicState), false, nil
		}
	}

	// Rule 6: Admitted descendant → tainted.
	if sources := nearestDescendantSources(n, index.children, node.TaintTainted); len(sources) > 0 {
		return descendantReason(sources), false, sources
	}

	return "ancestor chain and active subtree contain no taint sources", false, nil
}

func newTaintTraceIndex(allNodes []*node.Node) *taintTraceIndex {
	index := &taintTraceIndex{
		nodes:    make(map[string]*node.Node, len(allNodes)),
		parent:   make(map[string]*node.Node, len(allNodes)),
		children: make(map[string][]*node.Node),
	}
	for _, candidate := range allNodes {
		if candidate == nil {
			continue
		}
		index.nodes[candidate.ID.String()] = candidate
	}

	nearestCache := make(map[string]*node.Node)
	for _, candidate := range allNodes {
		if candidate == nil {
			continue
		}
		parent := nearestTraceParent(candidate, index.nodes, nearestCache)
		index.parent[candidate.ID.String()] = parent
		if parent != nil {
			index.children[parent.ID.String()] = append(index.children[parent.ID.String()], candidate)
		}
	}
	return index
}

func nearestTraceParent(n *node.Node, nodeMap map[string]*node.Node, cache map[string]*node.Node) *node.Node {
	parentID, hasParent := n.ID.Parent()
	var missing []string
	for hasParent {
		key := parentID.String()
		if parent, ok := nodeMap[key]; ok {
			for _, missingKey := range missing {
				cache[missingKey] = parent
			}
			return parent
		}
		if parent, ok := cache[key]; ok {
			for _, missingKey := range missing {
				cache[missingKey] = parent
			}
			return parent
		}
		missing = append(missing, key)
		parentID, hasParent = parentID.Parent()
	}
	for _, missingKey := range missing {
		cache[missingKey] = nil
	}
	return nil
}

func nearestDescendantSources(target *node.Node, children map[string][]*node.Node, wanted node.TaintState) []descendantSource {
	queue := append([]*node.Node(nil), children[target.ID.String()]...)
	for len(queue) > 0 {
		levelSize := len(queue)
		var found []descendantSource
		for i := 0; i < levelSize; i++ {
			candidate := queue[0]
			queue = queue[1:]
			if traceSevered(candidate) {
				continue
			}

			matches := wanted == node.TaintUnresolved && traceUnresolvedState(candidate.EpistemicState)
			matches = matches || wanted == node.TaintTainted && schema.IntroducesTaint(candidate.EpistemicState)
			if matches {
				found = append(found, descendantSource{
					NodeID:         candidate.ID.String(),
					EpistemicState: string(candidate.EpistemicState),
					TaintState:     string(candidate.TaintState),
				})
				continue
			}

			// Unresolved and admitted nodes terminate subtree inspection even
			// when looking for the other contribution type.
			if traceUnresolvedState(candidate.EpistemicState) || schema.IntroducesTaint(candidate.EpistemicState) {
				continue
			}
			queue = append(queue, children[candidate.ID.String()]...)
		}
		if len(found) > 0 {
			sort.Slice(found, func(i, j int) bool { return found[i].NodeID < found[j].NodeID })
			return found
		}
	}
	return nil
}

func descendantReason(sources []descendantSource) string {
	if len(sources) == 1 {
		if sources[0].EpistemicState == string(schema.EpistemicNeedsRefinement) {
			return fmt.Sprintf("descendant %s is reopened for refinement", sources[0].NodeID)
		}
		return fmt.Sprintf("descendant %s is %s", sources[0].NodeID, sources[0].EpistemicState)
	}
	ids := make([]string, len(sources))
	for i, source := range sources {
		ids[i] = source.NodeID
	}
	return fmt.Sprintf("descendants %s include %s nodes", strings.Join(ids, ", "), sources[0].EpistemicState)
}

func traceUnresolvedState(state schema.EpistemicState) bool {
	return state == schema.EpistemicPending ||
		state == schema.EpistemicDraft ||
		state == schema.EpistemicNeedsRefinement
}

func traceSevered(n *node.Node) bool {
	return n.EpistemicState == schema.EpistemicArchived || n.EpistemicState == schema.EpistemicRefuted
}

func outputTaintTraceJSON(cmd *cobra.Command, target *node.Node, entries []traceEntry) error {
	result := map[string]interface{}{
		"node_id":            target.ID.String(),
		"taint_state":        string(target.TaintState),
		"trace":              entries,
		"descendant_sources": entries[len(entries)-1].DescendantSources,
	}
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}

func outputTaintTraceText(cmd *cobra.Command, target *node.Node, entries []traceEntry) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Taint trace for node %s\n", target.ID.String())
	fmt.Fprintf(w, "Current taint: %s\n\n", render.ColorTaintState(target.TaintState))

	if target.TaintState == node.TaintClean {
		fmt.Fprintln(w, "This node is clean — no taint in its ancestor chain or active subtree.")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Ancestry:")
		for _, e := range entries {
			fmt.Fprintf(w, "  %s [%s] %s\n", e.NodeID,
				render.ColorEpistemicState(schema.EpistemicState(e.EpistemicState)),
				render.ColorTaintState(node.TaintState(e.TaintState)))
		}
		return nil
	}

	// Find taint sources
	var sources []traceEntry
	for _, e := range entries {
		if e.IsSource {
			sources = append(sources, e)
		}
	}

	if len(sources) > 0 {
		fmt.Fprintln(w, "Taint source(s):")
		for _, s := range sources {
			fmt.Fprintf(w, "  %s — %s\n", s.NodeID, s.Reason)
		}
		fmt.Fprintln(w)
	}

	if len(entries) > 0 && len(entries[len(entries)-1].DescendantSources) > 0 {
		fmt.Fprintln(w, "Descendant source(s):")
		for _, source := range entries[len(entries)-1].DescendantSources {
			fmt.Fprintf(w, "  %s — %s (%s)\n", source.NodeID, source.EpistemicState, source.TaintState)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Full trace (root to target):")
	for _, e := range entries {
		marker := "  "
		if e.IsSource {
			marker = "> "
		}
		fmt.Fprintf(w, "%s%s [%s] %s",
			marker,
			e.NodeID,
			render.ColorEpistemicState(schema.EpistemicState(e.EpistemicState)),
			render.ColorTaintState(node.TaintState(e.TaintState)))
		if e.Reason != "" {
			fmt.Fprintf(w, " — %s", e.Reason)
		}
		fmt.Fprintln(w)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newTaintTraceCmd())
}
