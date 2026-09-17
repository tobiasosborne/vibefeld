package taint

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
)

// TestDifferentialFuzz_SupportTaintRules is the D6 differential fuzz test. It
// generates seeded random DAGs/trees (5-40 nodes) with mixed child, reference
// and validation edges, local_assume scopes, missing targets, legacy cycles and
// random epistemic states (the outcomes of amend/reopen/archive/refute/admit/
// unadmit/accept/veto), then asserts:
//
//  1. the production fold equals the independent spec in spec_test.go;
//  2. RecomputeAll is idempotent;
//  3. no clean node has a support path through a non-validated result;
//  4. a validated sibling of an admitted node is not tainted by it.
//
// A mismatch reports the offending graph in a readable form. The former
// property "an incremental PropagateTaint from a random root equals the full
// derivation" is gone: PropagateTaint is a documented wrapper over RecomputeAll,
// so it compared a full recompute with itself. Its replacement is the
// ledger-driven differential in ledger_fuzz_test.go, which drives random
// command sequences through a real ledger and compares replay + RecomputeAll
// against the same spec.
func TestDifferentialFuzz_SupportTaintRules(t *testing.T) {
	const cases = 3000
	for seed := int64(0); seed < cases; seed++ {
		g := randomSpecGraph(rand.New(rand.NewSource(seed)))
		nodes := g.toNodes()

		RecomputeAll(nodes)
		prod := taintMap(nodes)
		want := specAll(g)
		if diff := diffTaints(prod, want); diff != "" {
			t.Fatalf("seed %d: production != spec: %s\n%s", seed, diff, g.describe())
		}

		// Idempotence: a second authoritative pass changes nothing.
		if changed := RecomputeAll(nodes); len(changed) != 0 {
			t.Fatalf("seed %d: RecomputeAll not idempotent, changed %v\n%s", seed, changedIDs(changed), g.describe())
		}

		// A stale starting point does not change the derivation: every node is
		// rederived from epistemic states and edges alone.
		for _, n := range nodes {
			n.TaintState = node.TaintUnresolved
		}
		RecomputeAll(nodes)
		if diff := diffTaints(taintMap(nodes), want); diff != "" {
			t.Fatalf("seed %d: recompute from stale taint != spec: %s\n%s", seed, diff, g.describe())
		}

		if bad := cleanPathThroughUnvalidated(g, want); bad != "" {
			t.Fatalf("seed %d: clean node has a support path through a non-validated result: %s\n%s", seed, bad, g.describe())
		}
		if bad := admittedSiblingTaintsValidated(g, want); bad != "" {
			t.Fatalf("seed %d: ancestor separation violated: %s\n%s", seed, bad, g.describe())
		}
	}
}

func taintMap(nodes []*node.Node) map[string]node.TaintState {
	out := make(map[string]node.TaintState, len(nodes))
	for _, n := range nodes {
		if n != nil {
			out[n.ID.String()] = n.TaintState
		}
	}
	return out
}

func diffTaints(a, b map[string]node.TaintState) string {
	for id, av := range a {
		if bv, ok := b[id]; !ok {
			return fmt.Sprintf("%s present in production but not spec", id)
		} else if av != bv {
			return fmt.Sprintf("%s production=%s spec=%s", id, av, bv)
		}
	}
	for id := range b {
		if _, ok := a[id]; !ok {
			return fmt.Sprintf("%s present in spec but not production", id)
		}
	}
	return ""
}

func changedIDs(nodes []*node.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n != nil {
			out = append(out, n.ID.String())
		}
	}
	return out
}

// cleanPathThroughUnvalidated walks result-use edges from every clean node and
// reports the first target that is not a validated node. Severed children are
// not result-use edges; a local_assume cited as a dependency is hypothesis-use.
// local_assume nodes and their subtrees are walked like any other child (v3.2
// amendment): an admitted step under a hypothesis must not leave the enclosing
// proof clean. A missing target, a severed dependency, an admitted or pending
// target would all make the citing node non-clean, so a clean node must only
// rest on validated results.
func cleanPathThroughUnvalidated(g *specGraph, taints map[string]node.TaintState) string {
	var walk func(id string, seen map[string]bool) string
	walk = func(id string, seen map[string]bool) string {
		if seen[id] {
			return ""
		}
		seen[id] = true
		n := g.nodes[id]
		if !specSevered(n.epistemic) {
			for child := range g.nodes {
				if g.effParent(child) != id {
					continue
				}
				cn := g.nodes[child]
				if specSevered(cn.epistemic) {
					continue
				}
				if cn.epistemic != schema.EpistemicValidated {
					return fmt.Sprintf("%s -> child %s (%s)", id, child, cn.epistemic)
				}
				if bad := walk(child, seen); bad != "" {
					return bad
				}
			}
			for _, deps := range [][]string{g.dep[id], g.valDep[id]} {
				for _, tgt := range deps {
					tn, ok := g.nodes[tgt]
					if !ok {
						return fmt.Sprintf("%s -> missing %s", id, tgt)
					}
					if tn.typ == schema.NodeTypeLocalAssume {
						continue
					}
					if tn.epistemic != schema.EpistemicValidated {
						return fmt.Sprintf("%s -> dependency %s (%s)", id, tgt, tn.epistemic)
					}
					if bad := walk(tgt, seen); bad != "" {
						return bad
					}
				}
			}
		}
		return ""
	}
	for id, taint := range taints {
		if taint != node.TaintClean {
			continue
		}
		if bad := walk(id, map[string]bool{}); bad != "" {
			return bad
		}
	}
	return ""
}

// admittedSiblingTaintsValidated is the ancestor-separation property: an
// admitted node's validated sibling must not inherit the admitted node's taint
// through the support relation, which never links siblings. It checks the
// weakest form: a validated sibling with a clean ancestor chain, no
// dependencies and no children as evidence cannot be tainted by the admitted
// sibling.
func admittedSiblingTaintsValidated(g *specGraph, taints map[string]node.TaintState) string {
	for id, n := range g.nodes {
		if specSevered(n.epistemic) || schema.IntroducesTaint(n.epistemic) {
			continue
		}
		if len(g.dep[id]) > 0 || len(g.valDep[id]) > 0 {
			continue
		}
		// Only a leaf with a clean ancestor chain, so the admitted sibling is
		// the only possible taint source.
		hasChild := false
		for child := range g.nodes {
			if g.effParent(child) == id {
				hasChild = true
				break
			}
		}
		if hasChild {
			continue
		}
		downClean := true
		for p := g.effParent(id); p != ""; {
			pn := g.nodes[p]
			if pn.epistemic != schema.EpistemicValidated {
				downClean = false
				break
			}
			p = g.effParent(p)
		}
		if !downClean {
			continue
		}
		for sib := range g.nodes {
			if sib == id || g.effParent(sib) != g.effParent(id) || !schema.IntroducesTaint(g.nodes[sib].epistemic) {
				continue
			}
			if taints[id] == node.TaintTainted {
				return fmt.Sprintf("validated %s tainted by admitted sibling %s", id, sib)
			}
		}
	}
	return ""
}

// randomSpecGraph builds a random tree plus random cross edges. IDs are
// hierarchical, so a node's parent is its ID prefix and the production support
// graph sees the same child edges.
func randomSpecGraph(rng *rand.Rand) *specGraph {
	n := 5 + rng.Intn(36) // 5..40
	g := &specGraph{
		nodes:  make(map[string]specNode, n),
		parent: make(map[string]string, n),
		dep:    make(map[string][]string, n),
		valDep: make(map[string][]string, n),
	}
	ids := make([]string, 0, n)
	childCount := make(map[string]int, n)
	for i := 0; i < n; i++ {
		id := "1"
		if i > 0 {
			p := ids[rng.Intn(len(ids))]
			childCount[p]++
			id = p + "." + strconv.Itoa(childCount[p])
			g.parent[id] = p
		}
		ids = append(ids, id)

		typ := schema.NodeTypeClaim
		if i > 0 && rng.Intn(100) < 15 {
			typ = schema.NodeTypeLocalAssume
		}
		g.nodes[id] = specNode{typ: typ, epistemic: randomEpistemic(rng)}
	}

	// Random cross edges: reference and validation dependencies, including
	// missing targets and legacy cycles (picked among all IDs, including
	// descendants/ancestors).
	for _, id := range ids {
		edges := rng.Intn(4)
		for e := 0; e < edges; e++ {
			var tgt string
			switch {
			case rng.Intn(100) < 8:
				// A missing target (valid ID shape that is absent from the graph).
				tgt = "1." + strconv.Itoa(9000+rng.Intn(1000))
			default:
				tgt = ids[rng.Intn(len(ids))]
			}
			if rng.Intn(2) == 0 {
				g.dep[id] = append(g.dep[id], tgt)
			} else {
				g.valDep[id] = append(g.valDep[id], tgt)
			}
		}
	}

	// One graph in four has a hole in the ID space: a non-root, non-leaf node is
	// deleted while its descendants stay. Such a node is not a vertex, and its
	// children attach to the nearest present ancestor in both the ancestor pass
	// and the support graph. Legacy or hand-edited ledgers can look like this.
	if rng.Intn(4) == 0 {
		var candidates []string
		for _, id := range ids {
			if id == "1" || childCount[id] == 0 {
				continue
			}
			candidates = append(candidates, id)
		}
		if len(candidates) > 0 {
			victim := candidates[rng.Intn(len(candidates))]
			delete(g.nodes, victim)
			g.deleted = append(g.deleted, victim)
		}
	}
	return g
}

func randomEpistemic(rng *rand.Rand) schema.EpistemicState {
	switch rng.Intn(100) {
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9:
		return schema.EpistemicPending
	case 10, 11, 12:
		return schema.EpistemicDraft
	case 13, 14, 15:
		return schema.EpistemicNeedsRefinement
	case 16, 17, 18, 19:
		return schema.EpistemicAdmitted
	case 20, 21, 22:
		return schema.EpistemicArchived
	case 23, 24:
		return schema.EpistemicRefuted
	default:
		return schema.EpistemicValidated
	}
}

// describe renders a graph deterministically for a mismatch report.
func (g *specGraph) describe() string {
	var out string
	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	// Simple insertion sort keeps the report dependency-free.
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	for _, id := range ids {
		n := g.nodes[id]
		out += fmt.Sprintf("  %s type=%s state=%s deps=%v valdeps=%v\n", id, n.typ, n.epistemic, g.dep[id], g.valDep[id])
	}
	if len(g.deleted) > 0 {
		out += fmt.Sprintf("  (deleted from the graph: %v)\n", g.deleted)
	}
	return out
}
