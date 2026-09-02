package state

import (
	"testing"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
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
	if got := replayed.GetNode(rootID).TaintState; got != node.TaintTainted {
		t.Errorf("root taint after replay = %q, want %q", got, node.TaintTainted)
	}
}
