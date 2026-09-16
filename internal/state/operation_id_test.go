package state

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
)

// TestHasOperationID_IndexedOnReplay verifies that an event carrying an
// optional operation_id is indexed during replay so a retried operation can
// find its already-committed result.
func TestHasOperationID_IndexedOnReplay(t *testing.T) {
	dir := t.TempDir()

	initEvent := ledger.NewProofInitialized("c", "a")
	if _, err := ledger.Append(dir, initEvent); err != nil {
		t.Fatalf("append init: %v", err)
	}

	pattern := ledger.NewPatternAdded("p", "d", "", "", "a")
	pattern.OperationID = "op-123"
	if _, err := ledger.Append(dir, pattern); err != nil {
		t.Fatalf("append pattern: %v", err)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	st, err := Replay(ldg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	seq, ok := st.HasOperationID("op-123")
	if !ok {
		t.Fatal("operation id op-123 not found after replay")
	}
	if seq != 2 {
		t.Fatalf("seq = %d, want 2", seq)
	}

	if _, ok := st.HasOperationID("missing"); ok {
		t.Fatal("missing operation id should not be found")
	}
	if _, ok := st.HasOperationID(""); ok {
		t.Fatal("empty operation id should not be found")
	}
}

// TestRecordOperationID_FirstWins verifies a multi-event operation maps to its
// first committed event.
func TestRecordOperationID_FirstWins(t *testing.T) {
	st := NewState()
	st.RecordOperationID("op", 3)
	st.RecordOperationID("op", 7)

	seq, ok := st.HasOperationID("op")
	if !ok || seq != 3 {
		t.Fatalf("HasOperationID = (%d, %v), want (3, true)", seq, ok)
	}
}
