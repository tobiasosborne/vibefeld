package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/cli"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/render"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/service"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/support"
	"github.com/tobiasosborne/vibefeld/internal/taint"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func newTaintTraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "taint-trace <node-id>",
		GroupID: GroupQuery,
		Short:   "Show taint propagation path for a node",
		Long: `Trace the source of taint for a proof node.

The trace follows the D6 support relation: child, reference-dependency and
validation-dependency edges all carry taint, and each line names the edge kind
and the source's revision (its recorded verdict sequence, or its latest
statement/dependency amendment). Hypothesis-use edges (local_assume targets)
carry nothing, a severed dependency is unresolved, and an admitted result is
taken on faith without descending.

Taint rules:
  - Archived/refuted nodes are always clean (severed from proof)
  - Pending/draft/needs_refinement nodes are unresolved
  - Admitted nodes are self_admitted (accepted without full proof)
  - Pending/draft/needs_refinement ancestors or results make a node unresolved
  - Admitted ancestors or results taint a validated node
  - An admitted node ignores its own subtree
  - Result-derived taint never contaminates validated siblings
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

// traceEntry represents one node on the ancestry chain of the target.
type traceEntry struct {
	NodeID         string `json:"node_id"`
	EpistemicState string `json:"epistemic_state"`
	TaintState     string `json:"taint_state"`
	Reason         string `json:"reason,omitempty"`
	IsSource       bool   `json:"is_source"`
}

// supportSource is one nearest cause of the target's taint or unresolved state,
// reached through the support relation. Edge is "self", "ancestor", "child",
// "dependency", "validation_dep", "cycle" or "missing". Contributes is the
// component this source hands its dependents: "tainted", "unresolved" or
// "missing".
type supportSource struct {
	SourceID    string   `json:"source_id"`
	Edge        string   `json:"edge"`
	Contributes string   `json:"contributes,omitempty"`
	Path        []string `json:"path,omitempty"`
	Cycle       []string `json:"cycle,omitempty"`
	State       string   `json:"state"`
	Taint       string   `json:"taint,omitempty"`
	VerdictSeq  int      `json:"verdict_seq,omitempty"`
	RevisionSeq int      `json:"revision_seq,omitempty"`
	Missing     bool     `json:"missing,omitempty"`
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

	// LoadState already ran the authoritative taint pass; keep it explicit so
	// the trace cannot diverge from derived state.
	taint.RecomputeAll(st.AllNodes())

	index := newTaintTraceIndex(st.AllNodes())
	chain := ancestryChain(target, index)
	entries := make([]traceEntry, len(chain))
	for i, n := range chain {
		entries[i] = traceEntry{
			NodeID:         n.ID.String(),
			EpistemicState: string(n.EpistemicState),
			TaintState:     string(n.TaintState),
		}
		entries[i].Reason, entries[i].IsSource = chainReason(n, chain[:i])
	}

	provider := support.ResultUseEdges(st, nil)
	sources := traceSupportSources(st, provider, target)

	switch strings.ToLower(format) {
	case "json":
		return outputTaintTraceJSON(cmd, target, entries, sources)
	default:
		return outputTaintTraceText(cmd, target, entries, sources)
	}
}

// chainReason explains a node on the ancestry chain from its own state and its
// non-severed ancestors (ancestor-context component only).
func chainReason(n *node.Node, ancestors []*node.Node) (string, bool) {
	if n.EpistemicState == schema.EpistemicArchived {
		return "archived — severed from proof tree", false
	}
	if n.EpistemicState == schema.EpistemicRefuted {
		return "refuted — severed from proof tree", false
	}
	if n.EpistemicState == schema.EpistemicPending {
		return "node is pending verification", true
	}
	if n.EpistemicState == schema.EpistemicDraft {
		return "node is in draft state", true
	}
	if n.EpistemicState == schema.EpistemicNeedsRefinement {
		return "node is reopened for refinement", true
	}
	if schema.IntroducesTaint(n.EpistemicState) {
		return fmt.Sprintf("node is %s — accepted without full proof", n.EpistemicState), true
	}
	for i := len(ancestors) - 1; i >= 0; i-- {
		a := ancestors[i]
		if traceSevered(a) {
			continue
		}
		if traceUnresolvedState(a.EpistemicState) {
			if a.EpistemicState == schema.EpistemicNeedsRefinement {
				return fmt.Sprintf("ancestor %s is reopened for refinement", a.ID.String()), false
			}
			return fmt.Sprintf("ancestor %s is %s", a.ID.String(), a.EpistemicState), false
		}
		if schema.IntroducesTaint(a.EpistemicState) {
			return fmt.Sprintf("ancestor %s is %s", a.ID.String(), a.EpistemicState), false
		}
	}
	return "", false
}

// traceSupportSources walks the result-use relation from target and reports the
// nearest sources of taint or unresolved state: the node's own state, a
// non-severed ancestor, or a child/dependency/validation result. It does not
// descend an admitted, pending or severed target, matching the fold.
func traceSupportSources(st *state.State, p support.Provider, target *node.Node) []supportSource {
	var out []supportSource
	seen := make(map[string]bool)

	// Legacy result-use cycles are the one source the fold reports that has no
	// non-validated node behind it: every member is validated and the walk
	// below would exhaust itself on `seen`. Index the cyclic components so a
	// walk that enters one names it.
	cycleOf := cyclicComponentIndex(p)

	// Own state and ancestor-context sources first.
	if s, ok := selfSource(st, target); ok {
		out = append(out, s)
	}
	if s, ok := ancestorSource(st, target); ok {
		out = append(out, s)
	}

	var walk func(id types.NodeID, path []string, via string)
	walk = func(id types.NodeID, path []string, via string) {
		key := id.String()
		if seen[key] {
			return
		}
		seen[key] = true

		n := st.GetNode(id)
		if n == nil {
			out = append(out, supportSource{SourceID: key, Edge: via, Contributes: "missing", Path: path, Missing: true, State: "missing"})
			return
		}
		if members, ok := cycleOf[key]; ok {
			s := makeSource(st, n, "cycle", path)
			s.Contributes = "unresolved"
			s.Cycle = cyclePath(p, id, members)
			out = append(out, s)
			return
		}
		if traceSevered(n) {
			// A severed node only reaches here as a dependency target.
			out = append(out, makeSource(st, n, via, path))
			return
		}
		if traceUnresolvedState(n.EpistemicState) || schema.IntroducesTaint(n.EpistemicState) {
			out = append(out, makeSource(st, n, via, path))
			return
		}
		for _, e := range sortedEdges(p.EdgesFrom(id)) {
			childPath := append(append([]string(nil), path...), e.To.String())
			walk(e.To, childPath, graphEdgeKindName(e.Kind))
		}
	}

	// Start from the direct result-use edges of the target. The target itself
	// may be a member of a legacy cycle, which is why it is checked first.
	startPath := []string{target.ID.String()}
	if !traceSevered(target) && !traceUnresolvedState(target.EpistemicState) && !schema.IntroducesTaint(target.EpistemicState) {
		if members, ok := cycleOf[target.ID.String()]; ok {
			s := makeSource(st, target, "cycle", startPath)
			s.Contributes = "unresolved"
			s.Cycle = cyclePath(p, target.ID, members)
			return append(out, s)
		}
		for _, e := range sortedEdges(p.EdgesFrom(target.ID)) {
			walk(e.To, append(append([]string(nil), startPath...), e.To.String()), graphEdgeKindName(e.Kind))
		}
	}
	return out
}

// cyclicComponentIndex maps every node in a legacy result-use cycle to the set
// of its component's members.
func cyclicComponentIndex(p support.Provider) map[string]map[string]bool {
	out := make(map[string]map[string]bool)
	for _, comp := range support.Prepare(p).CyclicComponents() {
		members := make(map[string]bool, len(comp))
		for _, id := range comp {
			members[id.String()] = true
		}
		for _, id := range comp {
			out[id.String()] = members
		}
	}
	return out
}

// cyclePath returns one cycle through start inside its component, as IDs, with
// start repeated at the end (e.g. 1.3 -> 1.5 -> 1.3). It is a depth-first
// search restricted to the component, so a path back to start always exists.
func cyclePath(p support.Provider, start types.NodeID, members map[string]bool) []string {
	startKey := start.String()
	visited := map[string]bool{startKey: true}
	var dfs func(id types.NodeID, path []string) []string
	dfs = func(id types.NodeID, path []string) []string {
		for _, e := range sortedEdges(p.EdgesFrom(id)) {
			key := e.To.String()
			if !members[key] {
				continue
			}
			if key == startKey {
				return append(append([]string(nil), path...), startKey)
			}
			if visited[key] {
				continue
			}
			visited[key] = true
			if found := dfs(e.To, append(append([]string(nil), path...), key)); found != nil {
				return found
			}
		}
		return nil
	}
	if found := dfs(start, []string{startKey}); found != nil {
		return found
	}
	return []string{startKey}
}

func selfSource(st *state.State, n *node.Node) (supportSource, bool) {
	if traceSevered(n) {
		return supportSource{SourceID: n.ID.String(), Edge: "self", Contributes: "unresolved", State: string(n.EpistemicState)}, true
	}
	if traceUnresolvedState(n.EpistemicState) || schema.IntroducesTaint(n.EpistemicState) {
		return makeSource(st, n, "self", []string{n.ID.String()}), true
	}
	return supportSource{}, false
}

func ancestorSource(st *state.State, n *node.Node) (supportSource, bool) {
	for id := n.ID; ; {
		parentID, ok := id.Parent()
		if !ok {
			break
		}
		id = parentID
		an := st.GetNode(id)
		if an == nil || traceSevered(an) {
			continue
		}
		if traceUnresolvedState(an.EpistemicState) || schema.IntroducesTaint(an.EpistemicState) {
			return makeSource(st, an, "ancestor", []string{an.ID.String()}), true
		}
	}
	return supportSource{}, false
}

func makeSource(st *state.State, n *node.Node, edge string, path []string) supportSource {
	verdict, revision := revisionSeqs(st, n)
	return supportSource{
		SourceID:    n.ID.String(),
		Edge:        edge,
		Contributes: contributedComponent(n),
		Path:        path,
		State:       string(n.EpistemicState),
		Taint:       string(n.TaintState),
		VerdictSeq:  verdict,
		RevisionSeq: revision,
	}
}

// contributedComponent is the component this source hands its dependents, which
// is what the fold does with it: an admitted result is taken on faith and
// contributes `tainted`; a pending/draft/needs_refinement result, or a severed
// node reached as a dependency, is `unresolved`. It is not the source's own
// taint state (an admitted node's own taint is `self_admitted`).
func contributedComponent(n *node.Node) string {
	switch {
	case traceSevered(n):
		return "unresolved"
	case traceUnresolvedState(n.EpistemicState):
		return "unresolved"
	case schema.IntroducesTaint(n.EpistemicState):
		return "tainted"
	default:
		return "clean"
	}
}

// revisionSeqs returns the node's recorded verdict sequence (its latest
// admission/validation) and its latest content-revision sequence.
func revisionSeqs(st *state.State, n *node.Node) (verdict, revision int) {
	verdict = n.VerdictSeq
	if seq, ok := st.LatestAmendmentSeq(n.ID); ok {
		revision = seq
	}
	return verdict, revision
}

func graphEdgeKindName(k support.GraphEdgeKind) string {
	switch k {
	case support.EdgeChild:
		return "child"
	case support.EdgeDependency:
		return "dependency"
	case support.EdgeValidationDep:
		return "validation_dep"
	default:
		return "edge"
	}
}

func sortedEdges(edges []support.GraphEdge) []support.GraphEdge {
	out := append([]support.GraphEdge(nil), edges...)
	sort.Slice(out, func(i, j int) bool {
		if !out[i].To.Equal(out[j].To) {
			return out[i].To.Less(out[j].To)
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// ancestryChain returns root-first present ancestors of n, including n.
func ancestryChain(n *node.Node, index *taintTraceIndex) []*node.Node {
	var chain []*node.Node
	for cur := n; cur != nil; cur = index.parent[cur.ID.String()] {
		chain = append([]*node.Node{cur}, chain...)
	}
	return chain
}

type taintTraceIndex struct {
	nodes  map[string]*node.Node
	parent map[string]*node.Node
}

func newTaintTraceIndex(allNodes []*node.Node) *taintTraceIndex {
	index := &taintTraceIndex{
		nodes:  make(map[string]*node.Node, len(allNodes)),
		parent: make(map[string]*node.Node, len(allNodes)),
	}
	for _, candidate := range allNodes {
		if candidate == nil {
			continue
		}
		index.nodes[candidate.ID.String()] = candidate
	}
	for _, candidate := range allNodes {
		if candidate == nil {
			continue
		}
		parentID, hasParent := candidate.ID.Parent()
		for hasParent {
			if parent, ok := index.nodes[parentID.String()]; ok {
				index.parent[candidate.ID.String()] = parent
				break
			}
			parentID, hasParent = parentID.Parent()
		}
	}
	return index
}

func traceUnresolvedState(state schema.EpistemicState) bool {
	return state == schema.EpistemicPending ||
		state == schema.EpistemicDraft ||
		state == schema.EpistemicNeedsRefinement
}

func traceSevered(n *node.Node) bool {
	return n.EpistemicState == schema.EpistemicArchived || n.EpistemicState == schema.EpistemicRefuted
}

func outputTaintTraceJSON(cmd *cobra.Command, target *node.Node, entries []traceEntry, sources []supportSource) error {
	result := map[string]interface{}{
		"node_id":         target.ID.String(),
		"taint_state":     string(target.TaintState),
		"trace":           entries,
		"support_sources": sources,
	}
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}

func outputTaintTraceText(cmd *cobra.Command, target *node.Node, entries []traceEntry, sources []supportSource) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Taint trace for node %s\n", target.ID.String())
	fmt.Fprintf(w, "Current taint: %s\n\n", render.ColorTaintState(target.TaintState))

	if target.TaintState == node.TaintClean {
		fmt.Fprintln(w, "This node is clean — no taint in its ancestor chain or support relation.")
		fmt.Fprintln(w)
	} else {
		fmt.Fprintln(w, "Support source(s):")
		for _, s := range sources {
			fmt.Fprintf(w, "  %s — %s\n", s.SourceID, formatSource(s))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Ancestry (root to target):")
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

// formatSource renders one source line. The verb is the component the source
// actually contributes ("unresolved via ..." for a pending or severed source,
// "tainted via ..." for an admitted one), and a legacy result-use cycle names
// the cycle instead of a single edge.
func formatSource(s supportSource) string {
	if s.Missing {
		return fmt.Sprintf("missing result via %s", s.Edge)
	}
	verb := s.Contributes
	if verb == "" || verb == "clean" {
		verb = "tainted"
	}
	if s.Edge == "cycle" {
		return fmt.Sprintf("%s via cycle %s", verb, strings.Join(s.Cycle, " -> "))
	}
	detail := s.State
	if s.Taint != "" {
		detail += ", taint " + s.Taint
	}
	if s.VerdictSeq > 0 {
		detail += fmt.Sprintf(", verdict seq %d", s.VerdictSeq)
	}
	if s.RevisionSeq > 0 {
		detail += fmt.Sprintf(", revision seq %d", s.RevisionSeq)
	}
	return fmt.Sprintf("%s via %s %s (%s)", verb, s.Edge, s.SourceID, detail)
}

func init() {
	rootCmd.AddCommand(newTaintTraceCmd())
}
