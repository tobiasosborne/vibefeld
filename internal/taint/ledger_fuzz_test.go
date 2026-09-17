package taint

import (
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// TestDifferentialFuzz_LedgerCommandSequences is the ledger-driven half of the
// D6 differential fuzz, and replaces the old property 3 (which compared a full
// recompute with a full recompute and could never fail). Instead of sampling
// end states directly, it builds a real ledger from random COMMAND SEQUENCES on
// a small tree — accept, admit, refute, archive, request-refinement (reopen),
// amend, amend-deps (including the reopening form), unvalidate, and stale
// `taint_recomputed` audit events — replays it through state.Replay, runs the
// authoritative RecomputeAll, and compares the result against the independent
// spec in spec_test.go.
//
// What this covers that the in-memory fuzz does not: state.Apply ordering,
// TaintRecomputed audit values being replayed over derived taint (they must be
// overridden, not trusted), dependency edges rewritten after acceptance, and
// the replay path itself.
//
// This package's test binary may import internal/state: internal/state does not
// import internal/taint (the D6 import-cycle break, v3.1 amendment 4); the
// authoritative recompute is the caller's obligation, which is exactly what
// this test performs.
func TestDifferentialFuzz_LedgerCommandSequences(t *testing.T) {
	t.Setenv("AF_TEST_NO_FSYNC", "1")
	const cases = 150
	for seed := int64(0); seed < cases; seed++ {
		rng := rand.New(rand.NewSource(seed))
		dir := t.TempDir()
		ldgDir := filepath.Join(dir, "ledger")
		if err := os.MkdirAll(ldgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		ldg, err := ledger.NewLedger(ldgDir)
		if err != nil {
			t.Fatalf("seed %d: new ledger: %v", seed, err)
		}

		applied := randomCommandLedger(t, rng, ldg)

		st, err := state.Replay(ldg)
		if err != nil {
			t.Fatalf("seed %d: replay: %v\n%s", seed, err, applied)
		}
		nodes := st.AllNodes()
		RecomputeAll(nodes)

		g := specFromNodes(nodes)
		want := specAll(g)
		if diff := diffTaints(taintMap(nodes), want); diff != "" {
			t.Fatalf("seed %d: replay+RecomputeAll != spec: %s\n%s%s", seed, diff, g.describe(), applied)
		}

		// A second authoritative pass over the replayed state changes nothing.
		if changed := RecomputeAll(nodes); len(changed) != 0 {
			t.Fatalf("seed %d: RecomputeAll not idempotent after replay: %v\n%s", seed, changedIDs(changed), g.describe())
		}
	}
}

// randomCommandLedger appends a random but legal command sequence to ldg and
// returns a readable transcript. Legality is decided by production state.Apply
// against a shadow state, so the generator does not reimplement the transition
// rules: an event Apply rejects is simply not written.
func randomCommandLedger(t *testing.T, rng *rand.Rand, ldg *ledger.Ledger) string {
	t.Helper()
	shadow := state.NewState()
	transcript := ""

	emit := func(event ledger.Event, label string) bool {
		if err := state.Apply(shadow, event); err != nil {
			return false // illegal here; the CLI would refuse it too
		}
		if _, err := ldg.Append(event); err != nil {
			t.Fatalf("append %s: %v", label, err)
		}
		transcript += "  " + label + "\n"
		return true
	}

	if !emit(ledger.NewProofInitialized("Fuzz conjecture", "fuzz"), "init") {
		t.Fatal("ProofInitialized rejected")
	}

	// A small tree (5-12 nodes) with hierarchical IDs, parents created first,
	// plus reference and validation edges (some onto absent IDs).
	n := 5 + rng.Intn(8)
	ids := make([]types.NodeID, 0, n)
	childCount := make(map[string]int, n)
	for i := 0; i < n; i++ {
		idStr := "1"
		if i > 0 {
			p := ids[rng.Intn(len(ids))]
			childCount[p.String()]++
			idStr = p.String() + "." + strconv.Itoa(childCount[p.String()])
		}
		id := mustIDStr(idStr)

		typ := schema.NodeTypeClaim
		if i > 0 && rng.Intn(100) < 15 {
			typ = schema.NodeTypeLocalAssume
		}
		opts := node.NodeOptions{
			Dependencies:   randomTargets(rng, ids),
			ValidationDeps: randomTargets(rng, ids),
		}
		nn, err := node.NewNodeWithOptions(id, typ, "stmt "+idStr, schema.InferenceAssumption, opts)
		if err != nil {
			t.Fatalf("new node %s: %v", idStr, err)
		}
		if emit(ledger.NewNodeCreated(*nn), "create "+idStr+" ("+string(typ)+")") {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		t.Fatal("no node created")
	}

	// Random commands. Apply decides which of them are legal in the state the
	// previous ones produced.
	for c := 0; c < 3*len(ids); c++ {
		id := ids[rng.Intn(len(ids))]
		idStr := id.String()
		switch rng.Intn(10) {
		case 0, 1, 2:
			emit(ledger.NewNodeValidated(id), "accept "+idStr)
		case 3:
			emit(ledger.NewNodeAdmitted(id), "admit "+idStr)
		case 4:
			emit(ledger.NewNodeRefuted(id), "refute "+idStr)
		case 5:
			emit(ledger.NewNodeArchived(id), "archive "+idStr)
		case 6:
			emit(ledger.NewRefinementRequested(id, "needs detail", "verifier"), "request-refinement "+idStr)
		case 7:
			if rng.Intn(2) == 0 {
				emit(ledger.NewNodeAmended(id, "stmt "+idStr, "amended "+idStr, "prover"), "amend "+idStr)
			} else {
				emit(ledger.NewNodeAmendedReopened(id, "stmt "+idStr, "amended "+idStr, "prover"), "amend --reopen "+idStr)
			}
		case 8:
			cur := shadow.GetNode(id)
			if cur == nil {
				continue
			}
			newDeps := randomTargets(rng, ids)
			emit(ledger.NewNodeDepsAmended(id, cur.Dependencies, newDeps, cur.ValidationDeps,
				randomTargets(rng, ids), "prover", "rewire", "", rng.Intn(2) == 0), "amend-deps "+idStr)
		default:
			// A stale audit value: replay must not trust it.
			emit(ledger.NewTaintRecomputed(id, randomTaintState(rng)), "taint_recomputed(stale) "+idStr)
		}
	}
	return transcript
}

// randomTargets picks 0-2 edge targets among the existing IDs, occasionally an
// absent one.
func randomTargets(rng *rand.Rand, ids []types.NodeID) []types.NodeID {
	var out []types.NodeID
	for i := rng.Intn(3); i > 0; i-- {
		if len(ids) == 0 || rng.Intn(100) < 10 {
			out = append(out, mustIDStr("1."+strconv.Itoa(9000+rng.Intn(100))))
			continue
		}
		out = append(out, ids[rng.Intn(len(ids))])
	}
	return out
}

func randomTaintState(rng *rand.Rand) node.TaintState {
	switch rng.Intn(4) {
	case 0:
		return node.TaintClean
	case 1:
		return node.TaintTainted
	case 2:
		return node.TaintSelfAdmitted
	default:
		return node.TaintUnresolved
	}
}

// specFromNodes builds the independent spec's graph from replayed nodes: the
// spec derives taint from types, epistemic states and edges alone, so a ledger
// the commands produced is compared against the same rule list the in-memory
// fuzz uses.
func specFromNodes(nodes []*node.Node) *specGraph {
	g := &specGraph{
		nodes:  make(map[string]specNode, len(nodes)),
		parent: make(map[string]string, len(nodes)),
		dep:    make(map[string][]string, len(nodes)),
		valDep: make(map[string][]string, len(nodes)),
	}
	for _, n := range nodes {
		if n == nil {
			continue
		}
		key := n.ID.String()
		g.nodes[key] = specNode{typ: n.Type, epistemic: n.EpistemicState}
		if parent, ok := n.ID.Parent(); ok {
			g.parent[key] = parent.String()
		}
		for _, d := range n.Dependencies {
			g.dep[key] = append(g.dep[key], d.String())
		}
		for _, d := range n.ValidationDeps {
			g.valDep[key] = append(g.valDep[key], d.String())
		}
	}
	return g
}
