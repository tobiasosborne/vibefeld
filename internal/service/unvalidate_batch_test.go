package service

import (
	"errors"
	"testing"

	aferrors "github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/verdicts"
)

func TestUnvalidateBatch_NotFound_CleanNoOp(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	report, err := svc.UnvalidateBatch("no-such-batch", "cleanup", "verifier-1")
	if err == nil {
		t.Fatal("expected ErrUnvalidateBatchNotFound, got nil")
	}
	if !errors.Is(err, ErrUnvalidateBatchNotFound) {
		t.Errorf("expected ErrUnvalidateBatchNotFound, got: %v", err)
	}
	if aferrors.Code(err) != aferrors.UNVALIDATE_BATCH_NOT_FOUND {
		t.Errorf("Code(err) = %v, want UNVALIDATE_BATCH_NOT_FOUND", aferrors.Code(err))
	}
	if report == nil || report.Count != 0 {
		t.Errorf("expected a report with Count==0, got: %+v", report)
	}
}

func TestUnvalidateBatch_EmptyBatchID_Errors(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	if _, err := svc.UnvalidateBatch("", "reason", "agent"); err == nil {
		t.Fatal("expected error for empty batch id")
	}
}

// TestUnvalidateBatch_RoundTrip is the V3 acceptance criterion: apply a
// batch (validating nodes under a shared batch id via ApplyVerdicts), then
// unvalidate that batch, and confirm the derived state returns to its
// pre-batch shape — both nodes back to pending, ValidatedBy/
// ValidationBatchID cleared.
func TestUnvalidateBatch_RoundTrip(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	preState, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() failed: %v", err)
	}
	preRoot := preState.GetNode(parseNodeID(t, "1"))
	preChild := preState.GetNode(parseNodeID(t, "1.1"))
	if preRoot.EpistemicState != EpistemicPending || preChild.EpistemicState != EpistemicPending {
		t.Fatalf("fixture precondition failed: expected both nodes pending before the batch")
	}

	data := `{
		"schema_version": "1", "batch_id": "batch-rt", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "child first"},
			{"node": "1", "verdict": "accept", "reason": "parent second"}
		]
	}`
	f, err := verdicts.ParseFile([]byte(data))
	if err != nil {
		t.Fatalf("ParseFile() failed: %v", err)
	}
	if _, err := svc.ApplyVerdicts(f); err != nil {
		t.Fatalf("ApplyVerdicts() failed: %v", err)
	}

	mid, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() failed: %v", err)
	}
	midRoot := mid.GetNode(parseNodeID(t, "1"))
	midChild := mid.GetNode(parseNodeID(t, "1.1"))
	if midRoot.EpistemicState != EpistemicValidated || midChild.EpistemicState != EpistemicValidated {
		t.Fatalf("fixture precondition failed: expected both nodes validated after the batch, got root=%s child=%s",
			midRoot.EpistemicState, midChild.EpistemicState)
	}
	if midRoot.ValidationBatchID != "batch-rt" || midChild.ValidationBatchID != "batch-rt" {
		t.Fatalf("expected both nodes to carry batch-rt, got root=%q child=%q", midRoot.ValidationBatchID, midChild.ValidationBatchID)
	}

	report, err := svc.UnvalidateBatch("batch-rt", "batch revoked in test", "verifier-1")
	if err != nil {
		t.Fatalf("UnvalidateBatch() failed: %v", err)
	}
	if report.Count != 2 {
		t.Errorf("Count = %d, want 2", report.Count)
	}

	post, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() failed: %v", err)
	}
	postRoot := post.GetNode(parseNodeID(t, "1"))
	postChild := post.GetNode(parseNodeID(t, "1.1"))

	if postRoot.EpistemicState != EpistemicPending || postChild.EpistemicState != EpistemicPending {
		t.Errorf("expected both nodes back to pending, got root=%s child=%s", postRoot.EpistemicState, postChild.EpistemicState)
	}
	if postRoot.ValidatedBy != "" || postRoot.ValidationBatchID != "" {
		t.Errorf("expected root's ValidatedBy/ValidationBatchID cleared, got %q/%q", postRoot.ValidatedBy, postRoot.ValidationBatchID)
	}
	if postChild.ValidatedBy != "" || postChild.ValidationBatchID != "" {
		t.Errorf("expected child's ValidatedBy/ValidationBatchID cleared, got %q/%q", postChild.ValidatedBy, postChild.ValidationBatchID)
	}

	// A second unvalidate of the same (now-empty) batch id is a clean no-op.
	_, err = svc.UnvalidateBatch("batch-rt", "second attempt", "verifier-1")
	if !errors.Is(err, ErrUnvalidateBatchNotFound) {
		t.Errorf("expected ErrUnvalidateBatchNotFound on second call, got: %v", err)
	}
}

// TestUnvalidateBatch_OnlyTargetsMatchingBatchID confirms a singly-validated
// node (no batch id, or a different batch id) is left untouched.
func TestUnvalidateBatch_OnlyTargetsMatchingBatchID(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	// Validate the child individually (no batch id) via the plain kernel
	// surface, then validate... wait, root requires the child cleared
	// first; validate child without a batch, leave root alone.
	if err := svc.AcceptNodeWithVerifier(parseNodeID(t, "1.1"), "", "verifier-1", ""); err != nil {
		t.Fatalf("AcceptNodeWithVerifier() failed: %v", err)
	}

	report, err := svc.UnvalidateBatch("some-batch", "reason", "verifier-1")
	if !errors.Is(err, ErrUnvalidateBatchNotFound) {
		t.Fatalf("expected ErrUnvalidateBatchNotFound (no node carries some-batch), got: %v", err)
	}
	if report.Count != 0 {
		t.Errorf("Count = %d, want 0", report.Count)
	}

	// The individually-validated child must remain validated.
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() failed: %v", err)
	}
	child := st.GetNode(parseNodeID(t, "1.1"))
	if child.EpistemicState != EpistemicValidated {
		t.Errorf("expected child to remain validated, got %s", child.EpistemicState)
	}
}
