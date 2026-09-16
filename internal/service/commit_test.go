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
// build func simulates a concurrent writer landing in the window between
// commit's state read and its append. The batch-wide sequence check must refuse
// the commit (ErrConcurrentModification) and append none of its events.
func TestCommit_Barrier_ConcurrentWriterCausesMismatch(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	childID := parseNodeID(t, "1.1")

	_, err := svc.commit(func(st *state.State) ([]ledger.Event, error) {
		// Concurrent writer: append straight to the ledger, bypassing commit,
		// exactly as another process would.
		if _, err := ledger.Append(svc.ledgerDir(), ledger.NewHintAdded(childID, "raced", "other-agent")); err != nil {
			return nil, err
		}
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
// verdicts-apply race test: the node is amended after the verdict was authored
// against its old hash, so the accept must be rejected with the hash message
// and no NodeValidated event may be appended.
func TestApplyVerdicts_AmendBetweenAuthoringAndApplyingRejected(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	childID := parseNodeID(t, "1.1")
	authoredHash := hashOf(t, svc, "1.1")

	// Amend between authoring and applying.
	if err := svc.AmendNode(childID, "prover-1", "Amended statement"); err != nil {
		t.Fatalf("AmendNode: %v", err)
	}

	data := fmt.Sprintf(`{
		"schema_version": "1", "batch_id": "b1", "verified_by": "verifier-1",
		"items": [{"node": "1.1", "verdict": "accept", "reason": "ok", "expect_hash": %q}]
	}`, authoredHash)
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err == nil {
		t.Fatal("expected nothing-applied error")
	}
	if got := report.Items[0].Status; got != "rejected:content-hash-mismatch" {
		t.Fatalf("status = %q, want rejected:content-hash-mismatch", got)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.GetNode(childID).EpistemicState == schema.EpistemicValidated {
		t.Fatal("node 1.1 was validated despite the stale hash")
	}
}
