package service

import (
	"fmt"
	"testing"
	"time"
)

// B1 (rk review 2026-07-20): `af verdicts apply` must re-check an expected
// content hash AND current verifier-ready classification (workflow
// availability) under its own state read, so a node edited/claimed/blocked
// between a verifier's dispatch and this apply cannot be accepted from a stale
// snapshot. Enforcement is opt-in via the item's expect_hash: rk always
// supplies it (it bound the verdict to that hash).

func hashOf(t *testing.T, svc *ProofService, id string) string {
	t.Helper()
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	n := st.GetNode(parseNodeID(t, id))
	if n == nil {
		t.Fatalf("node %s missing", id)
	}
	return n.ContentHash
}

func TestApplyVerdicts_ExpectHash_MatchApplies(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	h := hashOf(t, svc, "1.1")
	data := fmt.Sprintf(`{
		"schema_version": "1", "batch_id": "b1", "verified_by": "verifier-1",
		"items": [{"node": "1.1", "verdict": "accept", "reason": "ok", "expect_hash": %q}]
	}`, h)
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err != nil {
		t.Fatalf("expected all-applied, got %v", err)
	}
	if report.Items[0].Status != "applied" {
		t.Errorf("status = %q, want applied", report.Items[0].Status)
	}
}

func TestApplyVerdicts_ExpectHash_MismatchRejected(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	data := `{
		"schema_version": "1", "batch_id": "b1", "verified_by": "verifier-1",
		"items": [{"node": "1.1", "verdict": "accept", "reason": "ok", "expect_hash": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef0"}]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err == nil {
		t.Fatal("expected a non-nil error (nothing applied)")
	}
	if report.Items[0].Status != "rejected:content-hash-mismatch" {
		t.Errorf("status = %q, want rejected:content-hash-mismatch", report.Items[0].Status)
	}
	// The node must NOT have been validated.
	st, _ := svc.LoadState()
	if string(st.GetNode(parseNodeID(t, "1.1")).EpistemicState) == "validated" {
		t.Error("node 1.1 was validated despite a hash mismatch")
	}
}

// A node that became CLAIMED (not available) between dispatch and apply must be
// rejected as not verifier-ready, even though its content hash is unchanged.
func TestApplyVerdicts_ExpectHash_NotAvailableRejected(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	h := hashOf(t, svc, "1.1")
	if err := svc.ClaimNode(parseNodeID(t, "1.1"), "someone-else", time.Hour); err != nil {
		t.Fatalf("ClaimNode 1.1: %v", err)
	}
	data := fmt.Sprintf(`{
		"schema_version": "1", "batch_id": "b1", "verified_by": "verifier-1",
		"items": [{"node": "1.1", "verdict": "accept", "reason": "ok", "expect_hash": %q}]
	}`, h)
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err == nil {
		t.Fatal("expected a non-nil error (nothing applied)")
	}
	if report.Items[0].Status != "rejected:not-verifier-ready" {
		t.Errorf("status = %q, want rejected:not-verifier-ready", report.Items[0].Status)
	}
}

// A challenge item with a stale expect_hash is also discarded — the verifier
// challenged bytes that no longer exist.
func TestApplyVerdicts_ExpectHash_ChallengeMismatchRejected(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	data := `{
		"schema_version": "1", "batch_id": "b1", "verified_by": "verifier-1",
		"items": [{"node": "1.1", "verdict": "challenge", "severity": "major", "reason": "stale", "expect_hash": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef0"}]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err == nil {
		t.Fatal("expected a non-nil error (nothing applied)")
	}
	if report.Items[0].Status != "rejected:content-hash-mismatch" {
		t.Errorf("status = %q, want rejected:content-hash-mismatch", report.Items[0].Status)
	}
}

// Back-compat: an item WITHOUT expect_hash keeps the legacy behavior (no hash
// or availability gate) so existing callers/tests are unaffected.
func TestApplyVerdicts_NoExpectHash_LegacyBehavior(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	data := `{
		"schema_version": "1", "batch_id": "b1", "verified_by": "verifier-1",
		"items": [{"node": "1.1", "verdict": "accept", "reason": "ok"}]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err != nil {
		t.Fatalf("expected all-applied, got %v", err)
	}
	if report.Items[0].Status != "applied" {
		t.Errorf("status = %q, want applied", report.Items[0].Status)
	}
}
