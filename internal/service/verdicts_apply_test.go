package service

import (
	"strings"
	"testing"
	"time"

	aferrors "github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/verdicts"
)

// setupVerdictTestProof creates a proof with root node "1" (author
// "root-author") claimed and refined by "prover-1" into child "1.1"
// (author "prover-1"). Neither node is validated yet.
func setupVerdictTestProof(t *testing.T) (*ProofService, string) {
	t.Helper()
	tmpDir := t.TempDir()

	if err := Init(tmpDir, "Test conjecture", "root-author"); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	svc, err := NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("NewProofService() failed: %v", err)
	}

	rootID := parseNodeID(t, "1")
	if err := svc.ClaimNode(rootID, "prover-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode() failed: %v", err)
	}
	childID := parseNodeID(t, "1.1")
	if err := svc.RefineNode(rootID, "prover-1", childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode() failed: %v", err)
	}

	return svc, tmpDir
}

func mustParseFile(t *testing.T, data string) *verdicts.File {
	t.Helper()
	f, err := verdicts.ParseFile([]byte(data))
	if err != nil {
		t.Fatalf("ParseFile() failed: %v", err)
	}
	return f
}

// =============================================================================
// Order-dependence
// =============================================================================

func TestApplyVerdicts_OrderDependence_ParentBeforeChildBlocks(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	data := `{
		"schema_version": "1", "batch_id": "batch-1", "verified_by": "verifier-1",
		"items": [
			{"node": "1", "verdict": "accept", "reason": "parent listed first"},
			{"node": "1.1", "verdict": "accept", "reason": "child listed second"}
		]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err == nil {
		t.Fatal("expected a non-nil error for a partially-applied batch")
	}
	if aferrors.Code(err) != aferrors.VERDICTS_PARTIALLY_APPLIED {
		t.Errorf("Code(err) = %v, want VERDICTS_PARTIALLY_APPLIED", aferrors.Code(err))
	}
	if report.Items[0].Status != "blocked-by:children-not-validated" {
		t.Errorf("parent (item 0) status = %q, want blocked-by:children-not-validated", report.Items[0].Status)
	}
	if report.Items[1].Status != "applied" {
		t.Errorf("child (item 1) status = %q, want applied", report.Items[1].Status)
	}
	if report.Applied != 1 || report.Blocked != 1 {
		t.Errorf("Applied=%d Blocked=%d, want Applied=1 Blocked=1", report.Applied, report.Blocked)
	}
}

func TestApplyVerdicts_OrderDependence_ChildBeforeParentBothApply(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	data := `{
		"schema_version": "1", "batch_id": "batch-1", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "child listed first"},
			{"node": "1", "verdict": "accept", "reason": "parent listed second"}
		]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err != nil {
		t.Fatalf("expected all-applied (nil error), got: %v", err)
	}
	if report.Applied != 2 {
		t.Errorf("Applied = %d, want 2", report.Applied)
	}
	for i, it := range report.Items {
		if it.Status != "applied" {
			t.Errorf("item %d status = %q, want applied", i, it.Status)
		}
	}
}

// =============================================================================
// Mid-batch challenge blocks a later accept
// =============================================================================

func TestApplyVerdicts_MidBatchChallengeBlocksLaterAccept(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	// Challenge the child (major, blocking) earlier in the file, then try
	// to accept the parent later in the same file. The parent's accept
	// must fail because the child is still pending (never validated) —
	// exactly the scenario PRD C3 calls out: "a mid-batch challenge can
	// legitimately block later accepts."
	data := `{
		"schema_version": "1", "batch_id": "batch-1", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "challenge", "target": "inference", "severity": "major", "reason": "Inference looks wrong"},
			{"node": "1", "verdict": "accept", "reason": "parent, but child now challenged"}
		]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err == nil {
		t.Fatal("expected a non-nil error for a partially-applied batch")
	}
	if report.Items[0].Status != "applied" {
		t.Errorf("challenge item status = %q, want applied", report.Items[0].Status)
	}
	if report.Items[1].Status != "blocked-by:children-not-validated" {
		t.Errorf("parent accept status = %q, want blocked-by:children-not-validated", report.Items[1].Status)
	}

	// The child itself is still pending (not validated), confirming the
	// challenge did not silently validate or reject it.
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() failed: %v", err)
	}
	child := st.GetNode(parseNodeID(t, "1.1"))
	if child.EpistemicState != EpistemicPending {
		t.Errorf("child epistemic state = %q, want pending", child.EpistemicState)
	}
}

// TestApplyVerdicts_PreExistingBlockingChallenge_BlocksAccept covers the
// direct blocked-by:blocking-challenge classification (as opposed to the
// collateral children-not-validated path above): a challenge already exists
// on the very node an item tries to accept.
func TestApplyVerdicts_PreExistingBlockingChallenge_BlocksAccept(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	if err := svc.RaiseChallengeWithBatch(parseNodeID(t, "1.1"), "ch-pre", "inference", "pre-existing issue", "critical", "verifier-0", "", ""); err != nil {
		t.Fatalf("RaiseChallengeWithBatch() failed: %v", err)
	}

	data := `{
		"schema_version": "1", "batch_id": "batch-1", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "trying to accept despite pre-existing challenge"}
		]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err == nil {
		t.Fatal("expected a non-nil error (nothing applied)")
	}
	if aferrors.Code(err) != aferrors.VERDICTS_NONE_APPLIED {
		t.Errorf("Code(err) = %v, want VERDICTS_NONE_APPLIED", aferrors.Code(err))
	}
	if report.Items[0].Status != "blocked-by:blocking-challenge" {
		t.Errorf("status = %q, want blocked-by:blocking-challenge", report.Items[0].Status)
	}
}

// =============================================================================
// Reviewer != author
// =============================================================================

func TestApplyVerdicts_RejectsAcceptWhenReviewerEqualsAuthor(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	// Root node's author is "root-author" (see setupVerdictTestProof /
	// Init's author argument). A verdict file whose verified_by matches
	// that must have its accept rejected, per-item, without touching the
	// ledger.
	data := `{
		"schema_version": "1", "batch_id": "batch-1", "verified_by": "root-author",
		"items": [
			{"node": "1", "verdict": "accept", "reason": "self-review attempt"}
		]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err == nil {
		t.Fatal("expected a non-nil error (nothing applied)")
	}
	if report.Items[0].Status != "rejected:reviewer-equals-author" {
		t.Errorf("status = %q, want rejected:reviewer-equals-author", report.Items[0].Status)
	}

	// Confirm the ledger truly was not touched: root node is still pending.
	st, loadErr := svc.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState() failed: %v", loadErr)
	}
	root := st.GetNode(parseNodeID(t, "1"))
	if root.EpistemicState != EpistemicPending {
		t.Errorf("root epistemic state = %q, want pending (unchanged)", root.EpistemicState)
	}
}

func TestApplyVerdicts_DifferentReviewerCanAccept(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	// Accept the child (authored by "prover-1") with a distinct verifier —
	// must succeed, confirming the reviewer==author check doesn't misfire
	// when the identities genuinely differ.
	data := `{
		"schema_version": "1", "batch_id": "batch-1", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "independent review"}
		]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err != nil {
		t.Fatalf("expected all-applied, got error: %v", err)
	}
	if report.Items[0].Status != "applied" {
		t.Errorf("status = %q, want applied", report.Items[0].Status)
	}

	st, loadErr := svc.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState() failed: %v", loadErr)
	}
	child := st.GetNode(parseNodeID(t, "1.1"))
	if child.ValidatedBy != "verifier-1" {
		t.Errorf("ValidatedBy = %q, want verifier-1", child.ValidatedBy)
	}
	if child.ValidationBatchID != "batch-1" {
		t.Errorf("ValidationBatchID = %q, want batch-1", child.ValidationBatchID)
	}
}

// rk-qxp: a single-item, non-batch verdict file (no batch_id) applies normally
// and records NO batch provenance on the node — ValidationBatchID stays empty.
// This is the per-node apply rk's driver emits; recording a (sentinel) batch id
// here would trip rk's critical-path Check 13, so af must record none.
func TestApplyVerdicts_NoBatchID_AppliesWithoutBatchProvenance(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	data := `{
		"schema_version": "1", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "per-node apply, no batch"}
		]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err != nil {
		t.Fatalf("expected all-applied for a no-batch single item, got error: %v", err)
	}
	if report.Items[0].Status != "applied" {
		t.Errorf("status = %q, want applied", report.Items[0].Status)
	}

	st, loadErr := svc.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState() failed: %v", loadErr)
	}
	child := st.GetNode(parseNodeID(t, "1.1"))
	if child.ValidatedBy != "verifier-1" {
		t.Errorf("ValidatedBy = %q, want verifier-1", child.ValidatedBy)
	}
	if child.ValidationBatchID != "" {
		t.Errorf("ValidationBatchID = %q, want empty (no batch provenance for a non-batch apply)", child.ValidationBatchID)
	}
}

// rk-qxp: a no-batch single-item CHALLENGE likewise applies and records no
// batch id on the raised challenge event.
func TestApplyVerdicts_NoBatchID_ChallengeApplies(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	data := `{
		"schema_version": "1", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "challenge", "target": "completeness", "severity": "major", "reason": "missing case"}
		]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err != nil {
		t.Fatalf("expected all-applied for a no-batch challenge, got error: %v", err)
	}
	if report.Items[0].Status != "applied" {
		t.Errorf("status = %q, want applied", report.Items[0].Status)
	}
}

// =============================================================================
// Node not found / aggregate outcome classification
// =============================================================================

func TestApplyVerdicts_NodeNotFound_Rejected(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	data := `{
		"schema_version": "1", "batch_id": "batch-1", "verified_by": "verifier-1",
		"items": [
			{"node": "1.9", "verdict": "accept", "reason": "does not exist"}
		]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if aferrors.Code(err) != aferrors.VERDICTS_NONE_APPLIED {
		t.Errorf("Code(err) = %v, want VERDICTS_NONE_APPLIED", aferrors.Code(err))
	}
	if report.Items[0].Status != "rejected:node-not-found" {
		t.Errorf("status = %q, want rejected:node-not-found", report.Items[0].Status)
	}
}

func TestApplyVerdicts_PartiallyApplied_MixOfAppliedAndRejected(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	data := `{
		"schema_version": "1", "batch_id": "batch-1", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "valid child"},
			{"node": "1.9", "verdict": "accept", "reason": "does not exist"}
		]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if aferrors.Code(err) != aferrors.VERDICTS_PARTIALLY_APPLIED {
		t.Errorf("Code(err) = %v, want VERDICTS_PARTIALLY_APPLIED", aferrors.Code(err))
	}
	if report.Applied != 1 || report.Rejected != 1 {
		t.Errorf("Applied=%d Rejected=%d, want 1 and 1", report.Applied, report.Rejected)
	}
}

func TestApplyVerdicts_AllApplied_NilError(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)

	data := `{
		"schema_version": "1", "batch_id": "batch-1", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "valid child"},
			{"node": "1", "verdict": "accept", "reason": "parent after child"}
		]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err != nil {
		t.Fatalf("expected nil error for all-applied batch, got: %v", err)
	}
	if report.Applied != 2 || report.Blocked != 0 || report.Rejected != 0 {
		t.Errorf("unexpected counts: applied=%d blocked=%d rejected=%d", report.Applied, report.Blocked, report.Rejected)
	}
}

// =============================================================================
// Report aggregation / exit-code selection (pure, no service call)
// =============================================================================

func TestVerdictReport_ExitError_AllApplied(t *testing.T) {
	r := &VerdictReport{}
	r.record(verdicts.Item{Node: "1", Verdict: verdicts.VerdictAccept}, "applied", "")
	if err := r.exitError(); err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

func TestVerdictReport_ExitError_NoneApplied(t *testing.T) {
	r := &VerdictReport{}
	r.record(verdicts.Item{Node: "1", Verdict: verdicts.VerdictAccept}, "rejected:node-not-found", "")
	err := r.exitError()
	if err == nil || aferrors.Code(err) != aferrors.VERDICTS_NONE_APPLIED {
		t.Errorf("expected VERDICTS_NONE_APPLIED, got: %v", err)
	}
}

func TestVerdictReport_ExitError_PartiallyApplied(t *testing.T) {
	r := &VerdictReport{}
	r.record(verdicts.Item{Node: "1", Verdict: verdicts.VerdictAccept}, "applied", "")
	r.record(verdicts.Item{Node: "2", Verdict: verdicts.VerdictAccept}, "blocked-by:batch-aborted", "")
	err := r.exitError()
	if err == nil || aferrors.Code(err) != aferrors.VERDICTS_PARTIALLY_APPLIED {
		t.Errorf("expected VERDICTS_PARTIALLY_APPLIED, got: %v", err)
	}
	if r.Applied != 1 || r.Blocked != 1 {
		t.Errorf("Applied=%d Blocked=%d, want 1 and 1", r.Applied, r.Blocked)
	}
}

func TestParseVerdictFile_InvalidFileGetsFileInvalidCode(t *testing.T) {
	_, err := ParseVerdictFile([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected error")
	}
	if aferrors.Code(err) != aferrors.VERDICTS_FILE_INVALID {
		t.Errorf("Code(err) = %v, want VERDICTS_FILE_INVALID", aferrors.Code(err))
	}
	if !strings.Contains(err.Error(), "VERDICTS_FILE_INVALID") {
		t.Errorf("error string %q does not mention the code", err.Error())
	}
}
