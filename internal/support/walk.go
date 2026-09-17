package support

import (
	"sort"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// Folded is one result-use target's contribution to a folding node. Value is
// the target's memoised fold result. Resolved is false when the target could
// not be folded to an ordinary value:
//
//   - Missing is true when the target is not a node in the graph at all
//     (a dependency on a vanished node, or an overlay-only entry). Its Value
//     is the zero value.
//   - Cycle is true when the target is a member of the same strongly connected
//     component as the node being folded (a legacy cycle). Its Value is the
//     zero value.
//
// A fold decides what these sentinels mean for its own semantics (for
// support_current a cycle is CYCLE and a missing target is
// TARGET_NOT_CURRENT). Non-sentinel targets are always Resolved and their
// Value is the fold's result for that target.
type Folded[T any] struct {
	ID       types.NodeID
	Value    T
	Resolved bool
	Missing  bool
	Cycle    bool
}

// Graph is a result-use dependency graph whose Tarjan strongly-connected
// components (and therefore its dependency-first condensation order) are
// precomputed exactly once. Several folds — support_current today, taint in D6
// — run over one prepared Graph without rerunning Tarjan or rebuilding the
// adjacency.
//
// It also retains the adjacency with edge kinds and a direct-child index, so a
// fold can inspect children the result-use relation excludes or severs (a
// local_assume child, or a refuted child dropped as a severed edge).
type Graph struct {
	nodes    map[string]*node.Node
	order    []types.NodeID
	deps     map[string][]types.NodeID
	edges    map[string][]GraphEdge
	children map[string][]*node.Node

	// sccs holds the strongly connected components in dependency-first order
	// (a component appears only after every component it can reach); cyclic is
	// parallel to sccs and marks components that are a real cycle (more than
	// one member, or a self-loop).
	sccs   [][]string
	cyclic []bool
}

// Prepare runs the one-off graph analysis (Tarjan SCCs) over a result-use
// Provider and returns a Graph that any number of folds can walk. It is
// deterministic: vertices and dependency lists are processed in stable
// hierarchical-ID order, so sccs and every fold built on it are reproducible.
func Prepare(p Provider) *Graph {
	// The vertices are the graph's real nodes, in stable ID order.
	ids := make([]types.NodeID, 0, len(p.nodes))
	for _, id := range p.order {
		if p.nodes[id.String()] != nil {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].Less(ids[j]) })

	ts := &tarjan{
		index:   make(map[string]int, len(ids)),
		lowlink: make(map[string]int, len(ids)),
		onStack: make(map[string]bool, len(ids)),
		sccOf:   make(map[string]int, len(ids)),
	}
	for _, id := range ids {
		if _, seen := ts.index[id.String()]; !seen {
			ts.strongConnect(id.String(), p)
		}
	}

	g := &Graph{
		nodes:    p.nodes,
		order:    append([]types.NodeID(nil), p.order...),
		deps:     p.deps,
		edges:    p.edges,
		children: p.children,
		sccs:     ts.sccs,
	}
	for _, comp := range ts.sccs {
		g.cyclic = append(g.cyclic, len(comp) > 1 || ts.hasSelfLoop(comp[0], p))
	}
	return g
}

// CyclicComponents returns the result-use strongly-connected components that
// are real cycles: a component with more than one member, or a single node with
// a self-loop. Components are returned in dependency-first (Tarjan) order and
// each component's IDs are sorted hierarchically. It is the read-only audit view
// of the legacy cycles the walk already handles without error.
func (g *Graph) CyclicComponents() [][]types.NodeID {
	var out [][]types.NodeID
	for ci, comp := range g.sccs {
		if !g.cyclic[ci] {
			continue
		}
		ids := make([]types.NodeID, 0, len(comp))
		for _, s := range comp {
			id, err := types.Parse(s)
			if err != nil {
				continue
			}
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i].Less(ids[j]) })
		out = append(out, ids)
	}
	return out
}

// Walk is the single memoised topological traversal over a prepared Graph. It
// folds every node once, after all of its result-use targets have been folded,
// and returns the results keyed by node ID string. Only nodes backed by a
// *node.Node are folded; targets that are not (missing dependencies or
// prospective overlay entries) are reported to the fold as Folded sentinels.
//
// Legacy cycles in the data (result-use edges are required to be acyclic for
// new writes, but old workspaces may contain them) are handled without error:
// a node folding a target in its own component receives Folded{Cycle: true}.
// The component is still folded exactly once so every node gets a result.
//
// Walk is a free function, not a method, because Go does not permit generic
// methods; the prepared Graph it takes is the "prepare once, fold many" seam.
//
// D4 (support_current) and D6 (taint) both use this one walk, each with its own
// fold; D6 adds a fold, never a second traversal (v3.1 amendment 4).
func Walk[T any](g *Graph, fold func(n *node.Node, targets []Folded[T]) T) map[string]T {
	results := make(map[string]T, len(g.nodes))
	for ci, comp := range g.sccs {
		compSet := make(map[string]bool, len(comp))
		for _, id := range comp {
			compSet[id] = true
		}
		cyclic := g.cyclic[ci]
		for _, idStr := range comp {
			n := g.nodes[idStr]
			targets := make([]Folded[T], 0, len(g.deps[idStr]))
			for _, dep := range g.deps[idStr] {
				depStr := dep.String()
				if compSet[depStr] && cyclic {
					targets = append(targets, Folded[T]{ID: dep, Cycle: true})
					continue
				}
				if g.nodes[depStr] == nil {
					targets = append(targets, Folded[T]{ID: dep, Missing: true})
					continue
				}
				targets = append(targets, Folded[T]{ID: dep, Value: results[depStr], Resolved: true})
			}
			results[idStr] = fold(n, targets)
		}
	}
	return results
}

// tarjan is the scratch state for Tarjan's strongly-connected-components
// algorithm. sccs is populated in reverse topological order (a component is
// appended only after every component it can reach), which is exactly the
// dependency-first order Walk consumes.
type tarjan struct {
	index   map[string]int
	lowlink map[string]int
	onStack map[string]bool
	stack   []string
	next    int
	sccOf   map[string]int
	sccs    [][]string
}

// frame is one vertex being expanded by the iterative strongConnect: its
// adjacency and how far through it the walk has got.
type frame struct {
	v    string
	deps []types.NodeID
	next int
}

// strongConnect is Tarjan's algorithm with an explicit frame stack rather than
// recursion, so a deep legacy dependency chain cannot exhaust the goroutine
// stack. Adjacency lists arrive sorted from the Provider, so the traversal
// order — and every fold built on it — is deterministic.
func (t *tarjan) strongConnect(root string, p Provider) {
	t.push(root)
	frames := []frame{{v: root, deps: p.deps[root]}}

	for len(frames) > 0 {
		top := &frames[len(frames)-1]
		if top.next < len(top.deps) {
			w := top.deps[top.next].String()
			top.next++
			if p.nodes[w] == nil {
				continue // missing/prospective target: not a vertex
			}
			if _, seen := t.index[w]; !seen {
				t.push(w)
				frames = append(frames, frame{v: w, deps: p.deps[w]})
				continue
			}
			if t.onStack[w] && t.index[w] < t.lowlink[top.v] {
				t.lowlink[top.v] = t.index[w]
			}
			continue
		}

		// Every target of v has been expanded: close v, then fold its lowlink
		// into the caller's, which is what the recursive form did on return.
		v := top.v
		frames = frames[:len(frames)-1]
		if t.lowlink[v] == t.index[v] {
			t.popComponent(v)
		}
		if len(frames) > 0 {
			caller := frames[len(frames)-1].v
			if t.lowlink[v] < t.lowlink[caller] {
				t.lowlink[caller] = t.lowlink[v]
			}
		}
	}
}

// push assigns v its index and puts it on the SCC stack.
func (t *tarjan) push(v string) {
	t.index[v] = t.next
	t.lowlink[v] = t.next
	t.next++
	t.stack = append(t.stack, v)
	t.onStack[v] = true
}

// popComponent pops the strongly connected component whose root is v.
func (t *tarjan) popComponent(v string) {
	var comp []string
	for {
		w := t.stack[len(t.stack)-1]
		t.stack = t.stack[:len(t.stack)-1]
		t.onStack[w] = false
		comp = append(comp, w)
		if w == v {
			break
		}
	}
	id := len(t.sccs)
	t.sccs = append(t.sccs, comp)
	for _, w := range comp {
		t.sccOf[w] = id
	}
}

func (t *tarjan) hasSelfLoop(id string, p Provider) bool {
	for _, dep := range p.deps[id] {
		if dep.String() == id {
			return true
		}
	}
	return false
}
