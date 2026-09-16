package service

import (
	"errors"
	"testing"

	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/verdicts"
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

// TestUnvalidateBatch_BatchChangedBeforeCommitLeavesNodes is the item-3 race
// test. Batch b1 covers two nodes. In the window between the commit closure's
// discovery read and its CAS append, one node is unvalidated and revalidated
// under b2. The single CAS batch must refuse to run: neither node may be
// revoked from a stale selection, and both remain validated.
//
// The hook is one-shot: with the old per-node implementation the root's later
// UnvalidateNode would then succeed on a fresh read, so a non-one-shot hook
// would not distinguish the fix from the bug. Asserting the root is still
// validated catches the partial stale revoke.
func TestUnvalidateBatch_BatchChangedBeforeCommitLeavesNodes(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	rootID := parseNodeID(t, "1")
	childID := parseNodeID(t, "1.1")

	data := `{
		"schema_version": "1", "batch_id": "b1", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "child first"},
			{"node": "1", "verdict": "accept", "reason": "parent second"}
		]
	}`
	if _, err := svc.ApplyVerdicts(mustParseFile(t, data)); err != nil {
		t.Fatalf("ApplyVerdicts: %v", err)
	}

	// Concurrent writer: the child leaves b1 and is revalidated under b2 in the
	// window between the bulk closure's discovery read and its append. Fire
	// once, then disarm, so a regression to per-node commits can be detected.
	svc.beforeAppend = func() {
		svc.beforeAppend = nil
		if _, err := ledger.Append(svc.ledgerDir(), ledger.NewNodeUnvalidated(childID, "moved", "verifier-2")); err != nil {
			t.Errorf("concurrent unvalidate Append: %v", err)
		}
		if _, err := ledger.Append(svc.ledgerDir(), ledger.NewNodeValidatedFull(childID, "", "verifier-2", "b2")); err != nil {
			t.Errorf("concurrent revalidate Append: %v", err)
		}
	}
	defer func() { svc.beforeAppend = nil }()

	_, err := svc.UnvalidateBatch("b1", "revoke b1", "verifier-1")
	if !errors.Is(err, ErrConcurrentModification) {
		t.Fatalf("UnvalidateBatch err = %v, want ErrConcurrentModification", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got := st.GetNode(rootID).EpistemicState; got != "validated" {
		t.Fatalf("root state = %q, want still validated (b1 must not partially apply)", got)
	}
	if got := st.GetNode(childID).ValidationBatchID; got != "b2" {
		t.Fatalf("child batch = %q, want b2 (the concurrent revalidation stands)", got)
	}
}

// TestUnvalidateBatch_RechecksBatchIDPerNode verifies the commit closure
// re-checks each target's batch id and transition rather than trusting an
// earlier read.
func TestUnvalidateBatch_RechecksBatchIDPerNode(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	child := parseNodeID(t, "1.1")

	if err := svc.AcceptNodeWithVerifier(child, "", "verifier-1", "batch-z"); err != nil {
		t.Fatalf("AcceptNodeWithVerifier: %v", err)
	}

	report, err := svc.UnvalidateBatch("batch-z", "clean sweep", "verifier-1")
	if err != nil {
		t.Fatalf("UnvalidateBatch: %v", err)
	}
	if report.Count != 1 || len(report.Items) != 1 || report.Items[0].Err != "" {
		t.Fatalf("report = %+v, want one clean item", report)
	}

	if _, err := svc.UnvalidateBatch("batch-z", "second sweep", "verifier-1"); !errors.Is(err, ErrUnvalidateBatchNotFound) {
		t.Fatalf("second UnvalidateBatch err = %v, want ErrUnvalidateBatchNotFound", err)
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
