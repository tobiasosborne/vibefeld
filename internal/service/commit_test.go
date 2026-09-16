package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
)

// TestCommit_Barrier_ConcurrentWriterCausesMismatch is the D0 barrier test: the
// beforeAppend hook appends a concurrent writer's event in the exact window
// between commit's state read and its CAS append. The batch-wide sequence check
// must refuse the commit (ErrConcurrentModification) and append none of its
// events.
func TestCommit_Barrier_ConcurrentWriterCausesMismatch(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	childID := parseNodeID(t, "1.1")

	// Concurrent writer: append straight to the ledger, bypassing commit,
	// exactly as another process would. The hook runs after build validated
	// against the stale read and before AppendBatchIfSequence.
	svc.beforeAppend = func() {
		if _, err := ledger.Append(svc.ledgerDir(), ledger.NewHintAdded(childID, "raced", "other-agent")); err != nil {
			t.Errorf("concurrent Append: %v", err)
		}
	}
	defer func() { svc.beforeAppend = nil }()

	_, err := svc.commit(func(st *state.State) ([]ledger.Event, error) {
		return []ledger.Event{ledger.NewNodeValidatedFull(childID, "", "verifier-1", "")}, nil
	})
	if !errors.Is(err, ErrConcurrentModification) {
		t.Fatalf("commit err = %v, want ErrConcurrentModification", err)
	}

	// The commit's NodeValidated must not have landed.
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	n := st.GetNode(childID)
	if n == nil {
		t.Fatal("node 1.1 missing")
	}
	if n.EpistemicState == schema.EpistemicValidated {
		t.Fatal("commit appended its event despite the sequence mismatch")
	}
	// The concurrent writer's event is visible.
	if len(st.GetHints(childID)) == 0 {
		t.Fatal("concurrent writer's hint should be present")
	}
}

// findOperation is a small test helper for the lost-response recovery path D2
// builds on: it looks up the sequence of the first event carrying opID.
func (s *ProofService) findOperation(opID string) (int, bool) {
	st, err := s.LoadState()
	if err != nil {
		return 0, false
	}
	return st.HasOperationID(opID)
}

// TestCommit_OperationIDRetryShortCircuits verifies that an operation id
// recorded by a committed batch is discoverable via LoadState/HasOperationID,
// and that a re-attempt which checks it first appends nothing. This is the
// lost-response recovery path D2 will build on.
func TestCommit_OperationIDRetryShortCircuits(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	parent := parseNodeID(t, "1")
	child := parseNodeID(t, "1.1")

	childEvent := ledger.NewNodeValidatedFull(child, "", "verifier-1", "")
	childEvent.OperationID = "op-1"
	parentEvent := ledger.NewNodeValidatedFull(parent, "", "verifier-1", "")
	parentEvent.OperationID = "op-1"

	seqs, err := svc.commit(func(st *state.State) ([]ledger.Event, error) {
		return []ledger.Event{childEvent, parentEvent}, nil
	})
	if err != nil {
		t.Fatalf("first commit: %v", err)
	}

	seq, ok := svc.findOperation("op-1")
	if !ok {
		t.Fatal("findOperation(op-1) = not found, want found")
	}
	if seq != seqs[0] {
		t.Fatalf("findOperation(op-1) = %d, want first event seq %d", seq, seqs[0])
	}

	before, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	// A retry whose build checks HasOperationID first must be a no-op.
	seqs2, err := svc.commit(func(st *state.State) ([]ledger.Event, error) {
		if _, ok := st.HasOperationID("op-1"); ok {
			return nil, nil
		}
		return []ledger.Event{childEvent}, nil
	})
	if err != nil {
		t.Fatalf("retry commit: %v", err)
	}
	if seqs2 != nil {
		t.Fatalf("retry appended %v, want nothing", seqs2)
	}

	after, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState after: %v", err)
	}
	if after.LatestSeq() != before.LatestSeq() {
		t.Fatalf("retry changed LatestSeq from %d to %d", before.LatestSeq(), after.LatestSeq())
	}
}

// TestCommit_SuccessAppendsBatch verifies the happy path appends all events
// from one build against one state read.
func TestCommit_SuccessAppendsBatch(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	parent := parseNodeID(t, "1")
	child := parseNodeID(t, "1.1")

	seqs, err := svc.commit(func(st *state.State) ([]ledger.Event, error) {
		return []ledger.Event{
			ledger.NewNodeValidatedFull(child, "", "verifier-1", ""),
			ledger.NewNodeValidatedFull(parent, "", "verifier-1", ""),
		}, nil
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(seqs) != 2 || seqs[1] != seqs[0]+1 {
		t.Fatalf("seqs = %v, want two consecutive", seqs)
	}
}

// TestApplyVerdicts_AmendBetweenAuthoringAndApplyingRejected is the D0
// verdicts-apply race test: the node is amended after the accept's hash check
// passed against the stale state read but before its CAS append (via the
// beforeAppend hook), exactly the race the old two-read code had. The accept
// must be rejected as a concurrent modification and no NodeValidated event may
// be appended.
func TestApplyVerdicts_AmendBetweenAuthoringAndApplyingRejected(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	childID := parseNodeID(t, "1.1")
	authoredHash := hashOf(t, svc, "1.1")

	// Amend after the hash check passed on the stale read and before the
	// append. Append raw, not via AmendNode, to avoid re-entering commit.
	svc.beforeAppend = func() {
		if _, err := ledger.Append(svc.ledgerDir(), ledger.NewNodeAmended(childID, "Child statement", "Amended statement", "prover-1")); err != nil {
			t.Errorf("concurrent Append: %v", err)
		}
	}
	defer func() { svc.beforeAppend = nil }()

	data := fmt.Sprintf(`{
		"schema_version": "1", "batch_id": "b1", "verified_by": "verifier-1",
		"items": [{"node": "1.1", "verdict": "accept", "reason": "ok", "expect_hash": %q}]
	}`, authoredHash)
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err == nil {
		t.Fatal("expected nothing-applied error")
	}
	if got := report.Items[0].Status; got != "rejected:concurrent-modification" {
		t.Fatalf("status = %q, want rejected:concurrent-modification", got)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.GetNode(childID).EpistemicState == schema.EpistemicValidated {
		t.Fatal("node 1.1 was validated despite the concurrent amendment")
	}
}
