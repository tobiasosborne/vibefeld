package state

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/taint"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func TestReplay_DerivedTaintOverridesOldStyleAuditEvent(t *testing.T) {
	dir := t.TempDir()
	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatal(err)
	}

	rootID, _ := types.Parse("1")
	childID, _ := types.Parse("1.1")
	root, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
	child, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child", schema.InferenceAssumption)
	events := []ledger.Event{
		ledger.NewNodeCreated(*root),
		ledger.NewNodeCreated(*child),
		ledger.NewNodeAdmitted(childID),
		ledger.NewNodeValidated(rootID),
		ledger.NewTaintRecomputed(rootID, node.TaintClean),
	}
	for _, event := range events {
		if _, err := ldg.Append(event); err != nil {
			t.Fatalf("Append(%s): %v", event.Type(), err)
		}
	}

	replayed, err := Replay(ldg)
	if err != nil {
		t.Fatal(err)
	}
	// State replay is pure event sourcing; the authoritative taint pass is
	// applied by the service layer (and here explicitly).
	taint.RecomputeAll(replayed.AllNodes())
	if got := replayed.GetNode(rootID).TaintState; got != node.TaintTainted {
		t.Errorf("root taint after replay = %q, want %q", got, node.TaintTainted)
	}
}

// TestReplayTaint_IncrementalEqualsAuthoritative builds a small ledger with a
// dependency edge into an admitted node, then checks that applying the events
// one at a time and propagating taint incrementally converges to the same
// taints as a full Replay plus the authoritative taint pass. This is the
// "replay taint == incremental PropagateTaint" property of D6.
func TestReplayTaint_IncrementalEqualsAuthoritative(t *testing.T) {
	dir := t.TempDir()
	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatal(err)
	}

	rootID, _ := types.Parse("1")
	midID, _ := types.Parse("1.1")
	depID, _ := types.Parse("1.2")
	root, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
	mid, _ := node.NewNode(midID, schema.NodeTypeClaim, "Mid", schema.InferenceModusPonens)
	mid.Dependencies = []types.NodeID{depID}
	dep, _ := node.NewNode(depID, schema.NodeTypeClaim, "Dep", schema.InferenceAssumption)

	events := []ledger.Event{
		ledger.NewNodeCreated(*root),
		ledger.NewNodeCreated(*mid),
		ledger.NewNodeCreated(*dep),
		ledger.NewNodeAdmitted(depID),
		ledger.NewNodeValidated(midID),
		ledger.NewNodeValidated(rootID),
	}
	for _, e := range events {
		if _, err := ldg.Append(e); err != nil {
			t.Fatalf("Append(%s): %v", e.Type(), err)
		}
	}

	full, err := Replay(ldg)
	if err != nil {
		t.Fatal(err)
	}
	taint.RecomputeAll(full.AllNodes())

	incremental := NewState()
	for i, e := range events {
		if err := Apply(incremental, e); err != nil {
			t.Fatalf("Apply(%s): %v", e.Type(), err)
		}
		StampDerived(incremental, e, i+1)
		nodes := incremental.AllNodes()
		if len(nodes) > 0 {
			taint.PropagateTaint(nodes[0], nodes)
		}
	}

	for _, want := range full.AllNodes() {
		got := incremental.GetNode(want.ID)
		if got == nil {
			t.Fatalf("incremental state missing node %s", want.ID)
		}
		if got.TaintState != want.TaintState {
			t.Errorf("node %s: incremental %q != replay %q", want.ID, got.TaintState, want.TaintState)
		}
	}

	// The dependency edge must actually carry taint, so the comparison above is
	// not vacuous: the admitted 1.2 taints 1.1 through a reference dependency,
	// and 1.1 taints its parent 1.
	if got := full.GetNode(depID).TaintState; got != node.TaintSelfAdmitted {
		t.Errorf("1.2 = %q, want self_admitted", got)
	}
	if got := full.GetNode(midID).TaintState; got != node.TaintTainted {
		t.Errorf("1.1 = %q, want tainted via dependency 1.2", got)
	}
	if got := full.GetNode(rootID).TaintState; got != node.TaintTainted {
		t.Errorf("1 = %q, want tainted via child 1.1", got)
	}
}
