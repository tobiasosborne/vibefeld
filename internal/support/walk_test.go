package support

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
)

// foldVisits is a tiny fold that records visit order and returns the longest
// target chain below the node, so tests can assert one visit per node and
// dependency-first ordering.
type foldVisits struct {
	order []string
	seen  map[string]int
}

func (f *foldVisits) fold(n *node.Node, targets []Folded[int]) int {
	f.order = append(f.order, n.ID.String())
	f.seen[n.ID.String()]++
	best := 0
	for _, t := range targets {
		if t.Resolved && t.Value > best {
			best = t.Value
		}
	}
	return best + 1
}

func TestWalk_DAG(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim)
	addNode(t, st, "1.2", schema.NodeTypeClaim)
	addNode(t, st, "1.1.1", schema.NodeTypeClaim, "1.2")

	f := &foldVisits{seen: map[string]int{}}
	depth := Walk(Prepare(ResultUseEdges(st, nil)), f.fold)

	for _, id := range []string{"1", "1.1", "1.2", "1.1.1"} {
		if f.seen[id] != 1 {
			t.Errorf("node %s folded %d times, want 1 (order %v)", id, f.seen[id], f.order)
		}
	}
	// 1 -> 1.1 -> 1.1.1 -> 1.2 is the longest chain.
	if depth["1"] != 4 {
		t.Errorf("depth[1] = %d, want 4 (order %v)", depth["1"], f.order)
	}
	// Every target is folded before the node that cites it.
	pos := map[string]int{}
	for i, id := range f.order {
		pos[id] = i
	}
	if pos["1.1.1"] > pos["1.1"] {
		t.Errorf("1.1 folded before its dependency 1.1.1: %v", f.order)
	}
}

func TestWalk_Diamond(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim)
	addNode(t, st, "1.2", schema.NodeTypeClaim, "1.1.1")
	addNode(t, st, "1.1.1", schema.NodeTypeClaim)

	f := &foldVisits{seen: map[string]int{}}
	depth := Walk(Prepare(ResultUseEdges(st, nil)), f.fold)

	for _, id := range []string{"1", "1.1", "1.2", "1.1.1"} {
		if f.seen[id] != 1 {
			t.Errorf("diamond node %s folded %d times, want 1 (order %v)", id, f.seen[id], f.order)
		}
	}
	// Child 1.1.1 is shared only via tree structure here; root depth is 3
	// (1 -> 1.1 -> 1.1.1).
	if depth["1"] != 3 {
		t.Errorf("depth[1] = %d, want 3 (order %v)", depth["1"], f.order)
	}
}

func TestWalk_LegacyCycle(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim, "1.2")
	addNode(t, st, "1.2", schema.NodeTypeClaim, "1.1")

	f := &foldVisits{seen: map[string]int{}}
	depth := Walk(Prepare(ResultUseEdges(st, nil)), f.fold)

	// The walk terminates and visits every node exactly once despite the cycle.
	for _, id := range []string{"1", "1.1", "1.2"} {
		if f.seen[id] != 1 {
			t.Errorf("cycle node %s folded %d times, want 1", id, f.seen[id])
		}
	}
	// The cycle member is reported as an unresolved sentinel to the fold, so no
	// depth propagates through it.
	if depth["1.1"] < 1 || depth["1.2"] < 1 {
		t.Errorf("cycle members should still fold: %v", depth)
	}
}

func TestWalk_SelfLoop(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim, "1.1")

	sawCycle := false
	Walk(Prepare(ResultUseEdges(st, nil)), func(n *node.Node, targets []Folded[int]) int {
		for _, t := range targets {
			if t.ID.String() == "1.1" && t.Cycle {
				sawCycle = true
			}
		}
		return 0
	})
	if !sawCycle {
		t.Fatal("self-loop was not reported as a cycle sentinel")
	}
}

// TestWalk_MultipleFoldsOnePreparedGraph locks D4's "prepare once, fold many"
// seam: one Prepare call (one Tarjan pass) backs two independent folds, and the
// prepared graph retains edge kinds and the direct-child index.
func TestWalk_MultipleFoldsOnePreparedGraph(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim)
	addNode(t, st, "1.2", schema.NodeTypeClaim, "1.1")

	g := Prepare(ResultUseEdges(st, nil))

	// Two different folds over the same graph, no second Prepare.
	depth := Walk(g, func(n *node.Node, targets []Folded[int]) int {
		best := 0
		for _, t := range targets {
			if t.Resolved && t.Value > best {
				best = t.Value
			}
		}
		return best + 1
	})
	count := Walk(g, func(n *node.Node, targets []Folded[int]) int {
		return len(targets)
	})
	if depth["1"] != 3 {
		t.Errorf("depth[1] = %d, want 3", depth["1"])
	}
	if count["1"] != 2 || count["1.2"] != 1 {
		t.Errorf("target counts wrong: %+v", count)
	}

	// Edge kinds are retained: the child edge of 1 and the dependency edge of
	// 1.2 both land on 1.1 but for different reasons.
	kinds := map[GraphEdgeKind]int{}
	for _, e := range g.edges["1"] {
		if e.To.String() == "1.1" {
			kinds[e.Kind]++
		}
	}
	if kinds[EdgeChild] != 1 {
		t.Errorf("expected a child edge 1 -> 1.1, got %+v", g.edges["1"])
	}
	depKinds := map[GraphEdgeKind]int{}
	for _, e := range g.edges["1.2"] {
		if e.To.String() == "1.1" {
			depKinds[e.Kind]++
		}
	}
	if depKinds[EdgeDependency] != 1 {
		t.Errorf("expected a dependency edge 1.2 -> 1.1, got %+v", g.edges["1.2"])
	}

	// The child index includes direct children for folds that need them.
	if len(g.children["1"]) != 2 || g.children["1"][0].ID.String() != "1.1" || g.children["1"][1].ID.String() != "1.2" {
		t.Errorf("child index wrong: %+v", g.children["1"])
	}
}
