package taint

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// This file is the INDEPENDENT specification of D6 taint, written from the rule
// list in docs/concepts.md and docs/plans/scale-hardening.md D6 alone. It shares
// no helper with the production fold in support_fold.go or propagate.go: it is a
// plain recursive definition over a small in-memory graph struct, so a
// differential test can catch a production fold that drifts from the rules.
//
// Rules implemented here:
//
//   - own severed state (archived/refuted) -> clean, and a severed node
//     contributes nothing to a parent through a child edge (severed children
//     are not result-use edges);
//   - own admitted -> self_admitted, and a severed or admitted node is not
//     descended;
//   - own pending/draft/needs_refinement -> unresolved;
//   - a non-severed ancestor pending/draft/needs_refinement -> unresolved;
//   - an admitted target -> tainted;
//   - a pending/draft/needs_refinement target -> unresolved;
//   - a severed or missing dependency/validation target -> unresolved;
//   - a validated target -> its own support component;
//   - a local_assume cited as a dependency (hypothesis-use) carries nothing;
//   - a legacy result-use cycle: members of one strongly connected component
//     are unresolved.
//
// Result-use edges are (v3.2 amendment): every non-severed child of a node,
// whatever either node's type — a local_assume child is a step of its parent's
// decomposition and a local_assume's own children are work the enclosing proof
// relies on — plus non-local_assume dependencies and validation dependencies.
// A child whose immediate parent ID is absent attaches to its nearest present
// ancestor, the same rule the ancestor pass uses. The ancestor component is the
// chain of present ancestors by hierarchical ID (skipping absent IDs), separate
// from and never pushed into the support component.

type specGraph struct {
	nodes   map[string]specNode
	parent  map[string]string
	dep     map[string][]string
	valDep  map[string][]string
	missing map[string]bool // targets deliberately absent from the graph

	adj   map[string][]string
	reach map[string]map[string]bool
}

type specNode struct {
	typ       schema.NodeType
	epistemic schema.EpistemicState
}

type specComp uint8

const (
	specClean specComp = iota
	specTainted
	specUnresolved
)

type specResult struct {
	comp    specComp
	severed bool
}

// specAll returns the spec taint for every node, matching the production
// per-node map.
func specAll(g *specGraph) map[string]node.TaintState {
	out := make(map[string]node.TaintState, len(g.nodes))
	memo := make(map[string]specResult)
	for id := range g.nodes {
		out[id] = g.specFinal(id, memo)
	}
	return out
}

func (g *specGraph) specFinal(id string, memo map[string]specResult) node.TaintState {
	n := g.nodes[id]
	sup := g.specSupport(id, memo)

	down := specClean
	for p := g.parent[id]; p != ""; {
		if pn, ok := g.nodes[p]; ok {
			down = specCombine(down, specEpistemic(pn.epistemic))
		}
		p = g.parent[p]
	}

	switch {
	case specSevered(n.epistemic):
		return node.TaintClean
	case specIntroducesTaint(n.epistemic):
		return node.TaintSelfAdmitted
	case specUnresolvedState(n.epistemic):
		return node.TaintUnresolved
	case down == specUnresolved:
		return node.TaintUnresolved
	case sup.comp == specUnresolved:
		return node.TaintUnresolved
	case down == specTainted:
		return node.TaintTainted
	case sup.comp == specTainted:
		return node.TaintTainted
	default:
		return node.TaintClean
	}
}

// specSupport computes the component the node contributes to its dependents.
// The recursion is over the condensation DAG only: a target in the same cyclic
// strongly connected component is unresolved and is not descended, so the
// recursion terminates even on legacy cycles.
func (g *specGraph) specSupport(id string, memo map[string]specResult) specResult {
	if r, ok := memo[id]; ok {
		return r
	}
	n, ok := g.nodes[id]
	if !ok {
		return specResult{comp: specUnresolved}
	}
	if specSevered(n.epistemic) {
		return specResult{comp: specClean, severed: true}
	}
	if specUnresolvedState(n.epistemic) {
		return specResult{comp: specUnresolved}
	}
	if specIntroducesTaint(n.epistemic) {
		return specResult{comp: specTainted}
	}

	agg := specClean
	for _, t := range g.targets(id) {
		if g.sameSCC(id, t) {
			agg = specCombine(agg, specUnresolved)
			continue
		}
		if _, ok := g.nodes[t]; !ok {
			agg = specCombine(agg, specUnresolved)
			continue
		}
		r := g.specSupport(t, memo)
		if r.severed {
			agg = specCombine(agg, specUnresolved)
		} else {
			agg = specCombine(agg, r.comp)
		}
	}
	r := specResult{comp: agg}
	memo[id] = r
	return r
}

// targets lists the result-use targets of id: every non-severed child (the
// v3.2 amendment dropped the local_assume exclusion from clause (i) in both
// directions), plus non-local_assume dependency and validation targets (missing
// targets included, matching the production graph).
func (g *specGraph) targets(id string) []string {
	if g.adj != nil {
		if out, ok := g.adj[id]; ok {
			return out
		}
	}
	n := g.nodes[id]
	var out []string
	if !specSevered(n.epistemic) {
		for child := range g.nodes {
			if g.effParent(child) != id {
				continue
			}
			if specSevered(g.nodes[child].epistemic) {
				continue
			}
			out = append(out, child)
		}
		for _, deps := range [][]string{g.dep[id], g.valDep[id]} {
			for _, t := range deps {
				if tn, ok := g.nodes[t]; ok && tn.typ == schema.NodeTypeLocalAssume {
					continue
				}
				out = append(out, t)
			}
		}
	}
	if g.adj == nil {
		g.adj = make(map[string][]string)
	}
	g.adj[id] = out
	return out
}

// effParent is the nearest present ancestor of id. A node whose immediate
// parent ID is absent from the graph is a child of the nearest ancestor that is
// present, which is the rule the ancestor pass uses.
func (g *specGraph) effParent(id string) string {
	for p := g.parent[id]; p != ""; p = g.parent[p] {
		if _, ok := g.nodes[p]; ok {
			return p
		}
	}
	return ""
}

// sameSCC reports whether two nodes are mutually reachable over result-use
// edges. Equal singleton nodes are the same SCC only when they have a
// self-loop, which reach(id,id) detects.
func (g *specGraph) sameSCC(a, b string) bool {
	return g.canReach(a, b) && g.canReach(b, a)
}

func (g *specGraph) canReach(from, to string) bool {
	if g.reach == nil {
		g.reach = make(map[string]map[string]bool)
	}
	if m, ok := g.reach[from]; ok {
		return m[to]
	}
	seen := map[string]bool{}
	stack := append([]string(nil), g.targets(from)...)
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[cur] {
			continue
		}
		seen[cur] = true
		stack = append(stack, g.targets(cur)...)
	}
	g.reach[from] = seen
	return seen[to]
}

// specIntroducesTaint is written from the rule list ("own admitted") rather
// than reusing the production predicate, so the spec shares no decision helper
// with the fold.
func specIntroducesTaint(es schema.EpistemicState) bool {
	return es == schema.EpistemicAdmitted
}

func specEpistemic(es schema.EpistemicState) specComp {
	if specUnresolvedState(es) {
		return specUnresolved
	}
	if specIntroducesTaint(es) {
		return specTainted
	}
	return specClean
}

func specUnresolvedState(es schema.EpistemicState) bool {
	return es == schema.EpistemicPending || es == schema.EpistemicDraft || es == schema.EpistemicNeedsRefinement
}

func specSevered(es schema.EpistemicState) bool {
	return es == schema.EpistemicArchived || es == schema.EpistemicRefuted
}

func specCombine(a, b specComp) specComp {
	if a == specUnresolved || b == specUnresolved {
		return specUnresolved
	}
	if a == specTainted || b == specTainted {
		return specTainted
	}
	return specClean
}

// toNodes converts a spec graph to the production node slice. Child edges are
// implicit in the hierarchical IDs, so only type, state and explicit
// dependencies are copied.
func (g *specGraph) toNodes() []*node.Node {
	nodes := make([]*node.Node, 0, len(g.nodes))
	for id, sn := range g.nodes {
		n, err := node.NewNodeWithOptions(mustIDStr(id), sn.typ, "stmt "+id,
			schema.InferenceAssumption, node.NodeOptions{
				Dependencies:   parseIDs(g.dep[id]),
				ValidationDeps: parseIDs(g.valDep[id]),
			})
		if err != nil {
			panic("toNodes: " + err.Error())
		}
		n.EpistemicState = sn.epistemic
		nodes = append(nodes, n)
	}
	return nodes
}

func parseIDs(ids []string) []types.NodeID {
	out := make([]types.NodeID, 0, len(ids))
	for _, id := range ids {
		parsed, err := types.Parse(id)
		if err != nil {
			panic("parseIDs: " + err.Error())
		}
		out = append(out, parsed)
	}
	return out
}

func mustIDStr(id string) types.NodeID {
	parsed, err := types.Parse(id)
	if err != nil {
		panic("mustIDStr: " + err.Error())
	}
	return parsed
}

// TestSpec_SelfConsistency pins the independent spec on canonical examples so
// the differential test has a known-good oracle.
func TestSpec_SelfConsistency(t *testing.T) {
	g := &specGraph{
		nodes: map[string]specNode{
			"1":       {schema.NodeTypeClaim, schema.EpistemicValidated},
			"1.1":     {schema.NodeTypeClaim, schema.EpistemicAdmitted},
			"1.2":     {schema.NodeTypeClaim, schema.EpistemicValidated},
			"1.3":     {schema.NodeTypeClaim, schema.EpistemicValidated},
			"1.3.1":   {schema.NodeTypeClaim, schema.EpistemicValidated},
			"1.3.1.1": {schema.NodeTypeClaim, schema.EpistemicPending},
		},
		parent: map[string]string{
			"1.1": "1", "1.2": "1", "1.3": "1", "1.3.1": "1.3", "1.3.1.1": "1.3.1",
		},
		dep: map[string][]string{
			"1.3": {"1.1"}, // reference a sibling's admitted result
		},
	}
	got := specAll(g)
	want := map[string]node.TaintState{
		"1":       node.TaintUnresolved, // admitted child, plus 1.3's pending descendant
		"1.1":     node.TaintSelfAdmitted,
		"1.2":     node.TaintClean, // sibling separation
		"1.3":     node.TaintUnresolved,
		"1.3.1":   node.TaintUnresolved, // pending child
		"1.3.1.1": node.TaintUnresolved,
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("specAll()[%s] = %s, want %s", id, got[id], w)
		}
	}
}

// TestSpec_LegacyCycle is the spec's own cycle check: a two-node legacy
// result-use cycle is unresolved for both members.
func TestSpec_LegacyCycle(t *testing.T) {
	g := &specGraph{
		nodes: map[string]specNode{
			"1":   {schema.NodeTypeClaim, schema.EpistemicValidated},
			"1.1": {schema.NodeTypeClaim, schema.EpistemicValidated},
			"1.2": {schema.NodeTypeClaim, schema.EpistemicValidated},
		},
		parent: map[string]string{"1.1": "1", "1.2": "1"},
		dep: map[string][]string{
			"1.1": {"1.2"},
			"1.2": {"1.1"},
		},
	}
	got := specAll(g)
	if got["1.1"] != node.TaintUnresolved || got["1.2"] != node.TaintUnresolved {
		t.Fatalf("cycle members: 1.1=%s 1.2=%s, want unresolved", got["1.1"], got["1.2"])
	}
	if got["1"] != node.TaintUnresolved {
		t.Fatalf("parent of a cycle: %s, want unresolved", got["1"])
	}
}
