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

// Walk is the single memoised topological traversal over the result-use graph.
// It folds every node once, after all of its result-use targets have been
// folded, and returns the results keyed by node ID string. Only nodes backed by
// a *node.Node are folded; targets that are not (missing dependencies or
// prospective overlay entries) are reported to the fold as Folded sentinels.
//
// Legacy cycles in the data (result-use edges are required to be acyclic for
// new writes, but old workspaces may contain them) are handled without error:
// the strongly connected components are found with Tarjan's algorithm, and a
// node folding a target in its own component receives Folded{Cycle: true}. The
// component is still folded exactly once so every node gets a result.
//
// D4 (support_current) and D6 (taint) both use this one walk, each with its own
// fold; D6 adds a fold, never a second traversal (v3.1 amendment 4).
func Walk[T any](p Provider, fold func(n *node.Node, targets []Folded[T]) T) map[string]T {
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

	results := make(map[string]T, len(ids))
	for _, comp := range ts.sccs {
		compSet := make(map[string]bool, len(comp))
		for _, id := range comp {
			compSet[id] = true
		}
		cyclic := len(comp) > 1 || ts.hasSelfLoop(comp[0], p)
		for _, idStr := range comp {
			n := p.nodes[idStr]
			targets := make([]Folded[T], 0, len(p.deps[idStr]))
			for _, dep := range sortedDeps(p.deps[idStr]) {
				depStr := dep.String()
				if compSet[depStr] && cyclic {
					targets = append(targets, Folded[T]{ID: dep, Cycle: true})
					continue
				}
				if p.nodes[depStr] == nil {
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

func (t *tarjan) strongConnect(v string, p Provider) {
	t.index[v] = t.next
	t.lowlink[v] = t.next
	t.next++
	t.stack = append(t.stack, v)
	t.onStack[v] = true

	for _, dep := range sortedDeps(p.deps[v]) {
		w := dep.String()
		if p.nodes[w] == nil {
			continue // missing/prospective target: not a vertex
		}
		if _, seen := t.index[w]; !seen {
			t.strongConnect(w, p)
			if t.lowlink[w] < t.lowlink[v] {
				t.lowlink[v] = t.lowlink[w]
			}
		} else if t.onStack[w] {
			if t.index[w] < t.lowlink[v] {
				t.lowlink[v] = t.index[w]
			}
		}
	}

	if t.lowlink[v] == t.index[v] {
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
}

func (t *tarjan) hasSelfLoop(id string, p Provider) bool {
	for _, dep := range p.deps[id] {
		if dep.String() == id {
			return true
		}
	}
	return false
}

// sortedDeps returns a copy of deps in stable hierarchical-ID order so the
// walk — and therefore every fold built on it — is deterministic.
func sortedDeps(deps []types.NodeID) []types.NodeID {
	out := append([]types.NodeID(nil), deps...)
	sort.Slice(out, func(i, j int) bool { return out[i].Less(out[j]) })
	return out
}
