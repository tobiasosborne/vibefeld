package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/schema"
)

// D3: accept paths record the content hash accepted and whether an expected
// hash was checked; claim tests record their hash and acceptance ignores a
// stale passing test.

// nodeValidatedEvents returns every NodeValidated event in the service's
// ledger, in ledger order.
func nodeValidatedEvents(t *testing.T, svc *ProofService) []ledger.NodeValidated {
	t.Helper()
	ldg, err := svc.getLedger()
	if err != nil {
		t.Fatalf("getLedger: %v", err)
	}
	records, err := ldg.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	var out []ledger.NodeValidated
	for _, record := range records {
		var header struct {
			Type ledger.EventType `json:"type"`
		}
		if err := json.Unmarshal(record, &header); err != nil || header.Type != ledger.EventNodeValidated {
			continue
		}
		var event ledger.NodeValidated
		if err := json.Unmarshal(record, &event); err != nil {
			t.Fatalf("unmarshal NodeValidated: %v", err)
		}
		out = append(out, event)
	}
	return out
}

// lastNodeValidatedFor returns the most recent NodeValidated for nodeID.
func lastNodeValidatedFor(t *testing.T, svc *ProofService, id string) ledger.NodeValidated {
	t.Helper()
	var found *ledger.NodeValidated
	for _, ev := range nodeValidatedEvents(t, svc) {
		if ev.NodeID.String() == id {
			e := ev
			found = &e
		}
	}
	if found == nil {
		t.Fatalf("no NodeValidated event for node %s", id)
	}
	return *found
}

func TestAcceptNode_RecordsContentHashAndCheckedFalse(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")

	st0, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	wantHash := st0.GetNode(rootID).ContentHash

	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	n := st.GetNode(rootID)
	if n.ValidatedContentHash != wantHash {
		t.Errorf("node ValidatedContentHash = %q, want %q", n.ValidatedContentHash, wantHash)
	}
	if n.ValidatedHashChecked {
		t.Error("plain accept must record ValidatedHashChecked=false")
	}

	ev := lastNodeValidatedFor(t, svc, "1")
	if ev.ContentHash != wantHash {
		t.Errorf("event ContentHash = %q, want %q", ev.ContentHash, wantHash)
	}
	if ev.ExpectedHashChecked {
		t.Error("event ExpectedHashChecked = true, want false for plain accept")
	}
}

func TestAcceptNodeWithExpectation_RecordsChecked(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")

	st0, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	h := st0.GetNode(rootID).ContentHash

	if err := svc.AcceptNodeWithExpectation(rootID, "", "verifier-1", "", h); err != nil {
		t.Fatalf("AcceptNodeWithExpectation: %v", err)
	}

	st, _ := svc.LoadState()
	n := st.GetNode(rootID)
	if n.ValidatedContentHash != h || !n.ValidatedHashChecked {
		t.Errorf("node = %q/%v, want %q/true", n.ValidatedContentHash, n.ValidatedHashChecked, h)
	}
	ev := lastNodeValidatedFor(t, svc, "1")
	if !ev.ExpectedHashChecked || ev.ContentHash != h {
		t.Errorf("event = checked:%v hash:%q, want checked:true hash:%q", ev.ExpectedHashChecked, ev.ContentHash, h)
	}
}

func TestAcceptNodeWithExpectation_MismatchRefused(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")

	err := svc.AcceptNodeWithExpectation(rootID, "", "verifier-1", "", "deadbeef")
	if err == nil {
		t.Fatal("expected a hash mismatch error, got nil")
	}

	st, _ := svc.LoadState()
	if st.GetNode(rootID).EpistemicState == schema.EpistemicValidated {
		t.Error("node was validated despite a hash mismatch")
	}
	if len(nodeValidatedEvents(t, svc)) != 0 {
		t.Error("no NodeValidated event should have been written")
	}
}

func TestApplyVerdicts_ExpectHash_RecordsCheckedTrue(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	h := hashOf(t, svc, "1.1")
	data := `{
		"schema_version": "1", "batch_id": "b1", "verified_by": "verifier-1",
		"items": [{"node": "1.1", "verdict": "accept", "reason": "ok", "expect_hash": "` + h + `"}]
	}`
	report, err := svc.ApplyVerdicts(mustParseFile(t, data))
	if err != nil {
		t.Fatalf("ApplyVerdicts: %v", err)
	}
	if report.Items[0].Status != "applied" {
		t.Fatalf("status = %q, want applied", report.Items[0].Status)
	}

	ev := lastNodeValidatedFor(t, svc, "1.1")
	if ev.ContentHash != h {
		t.Errorf("event ContentHash = %q, want %q", ev.ContentHash, h)
	}
	if !ev.ExpectedHashChecked {
		t.Error("event ExpectedHashChecked = false, want true for an expect_hash verdict")
	}
}

// appendClaimTest writes a ClaimTested event directly so a chosen content hash
// can be recorded, without needing to run a real script.
func appendClaimTest(t *testing.T, svc *ProofService, id string, passed bool, contentHash string) {
	t.Helper()
	ev := ledger.NewClaimTested(parseNodeID(t, id), "script", "test.py", "", passed, "ok", "agent-1")
	ev.ContentHash = contentHash
	if _, err := ledger.Append(svc.ledgerDir(), ev); err != nil {
		t.Fatalf("append ClaimTested: %v", err)
	}
}

func TestAcceptNode_CruxStaleClaimTestIgnored(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")
	if err := svc.ClaimNode(rootID, "prover-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}
	ids, err := svc.RefineNodeBulk(rootID, "prover-1", []ChildSpec{{
		NodeType:  schema.NodeTypeClaim,
		Statement: "Crux step",
		Inference: schema.InferenceModusPonens,
		Crux:      true,
	}})
	if err != nil {
		t.Fatalf("RefineNodeBulk: %v", err)
	}
	childID := ids[0]

	// A passing test recorded against a different hash must not gate acceptance.
	appendClaimTest(t, svc, childID.String(), true, "stale-hash")
	err = svc.AcceptNodeWithVerifier(childID, "", "verifier-1", "")
	if err == nil {
		t.Fatal("expected acceptance to be refused for a stale claim-test, got nil")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("error should name the stale test, got: %v", err)
	}

	// A legacy passing test (no recorded hash) still counts.
	appendClaimTest(t, svc, childID.String(), true, "")
	if err := svc.AcceptNodeWithVerifier(childID, "", "verifier-1", ""); err != nil {
		t.Fatalf("legacy claim-test must count, got: %v", err)
	}
}

// writePassingScript writes an executable script that exits 0, so RunClaimTest
// records a real passing test rather than a fabricated event.
func writePassingScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pass.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

// refineCruxChild claims the root and refines it into a single crux child,
// returning the child's ID.
func refineCruxChild(t *testing.T, svc *ProofService) string {
	t.Helper()
	rootID := parseNodeID(t, "1")
	if err := svc.ClaimNode(rootID, "prover-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}
	ids, err := svc.RefineNodeBulk(rootID, "prover-1", []ChildSpec{{
		NodeType:  schema.NodeTypeClaim,
		Statement: "Crux step",
		Inference: schema.InferenceModusPonens,
		Crux:      true,
	}})
	if err != nil {
		t.Fatalf("RefineNodeBulk: %v", err)
	}
	return ids[0].String()
}

// TestRunClaimTest_RecordsHashAndAcceptRejectsAfterAmend exercises the real
// recording path: RunClaimTest must store the node's current ContentHash, and
// once the node is amended the test becomes stale and cannot gate acceptance.
func TestRunClaimTest_RecordsHashAndAcceptRejectsAfterAmend(t *testing.T) {
	svc, _ := setupTestProof(t)
	childIDStr := refineCruxChild(t, svc)
	childID := parseNodeID(t, childIDStr)

	stBefore, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	wantHash := stBefore.GetNode(childID).ContentHash

	passed, _, err := svc.RunClaimTest(childID, "script", writePassingScript(t), "", "agent-1")
	if err != nil {
		t.Fatalf("RunClaimTest: %v", err)
	}
	if !passed {
		t.Fatal("passing script recorded as failed")
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	tests := st.GetClaimTests(childID)
	if len(tests) != 1 {
		t.Fatalf("got %d claim tests, want 1", len(tests))
	}
	if tests[0].ContentHash != wantHash {
		t.Errorf("recorded test hash = %q, want node content hash %q", tests[0].ContentHash, wantHash)
	}

	// Amending the node changes its content hash, making the test stale.
	if err := svc.AmendNode(childID, "prover-1", "Amended crux step"); err != nil {
		t.Fatalf("AmendNode: %v", err)
	}
	stAmended, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState after amend: %v", err)
	}
	if stAmended.GetNode(childID).ContentHash == wantHash {
		t.Fatal("amend did not change the content hash; test cannot be stale")
	}

	err = svc.AcceptNodeWithVerifier(childID, "", "verifier-1", "")
	if err == nil {
		t.Fatal("expected acceptance to be refused for a stale claim-test, got nil")
	}
	if !errors.Is(err, ErrClaimTestStale) {
		t.Errorf("error %v does not wrap ErrClaimTestStale", err)
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("error should name the stale test, got: %v", err)
	}
}

// TestAcceptNodeBulk_RecordsHashCheckedFalse verifies the bulk path records
// each accepted node's hash with ExpectedHashChecked=false.
func TestAcceptNodeBulk_RecordsHashCheckedFalse(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")
	if err := svc.ClaimNode(rootID, "prover-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}
	ids, err := svc.RefineNodeBulk(rootID, "prover-1", []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "Step A", Inference: schema.InferenceModusPonens},
		{NodeType: schema.NodeTypeClaim, Statement: "Step B", Inference: schema.InferenceModusPonens},
	})
	if err != nil {
		t.Fatalf("RefineNodeBulk: %v", err)
	}

	st0, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	wantHashes := make(map[string]string, len(ids))
	for _, id := range ids {
		wantHashes[id.String()] = st0.GetNode(id).ContentHash
	}

	if err := svc.AcceptNodeBulk(ids); err != nil {
		t.Fatalf("AcceptNodeBulk: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	events := nodeValidatedEvents(t, svc)
	if len(events) != len(ids) {
		t.Fatalf("got %d NodeValidated events, want %d", len(events), len(ids))
	}
	for _, ev := range events {
		want, ok := wantHashes[ev.NodeID.String()]
		if !ok {
			t.Fatalf("unexpected NodeValidated event for node %s", ev.NodeID.String())
		}
		if ev.ContentHash != want {
			t.Errorf("event %s ContentHash = %q, want %q", ev.NodeID.String(), ev.ContentHash, want)
		}
		if ev.ExpectedHashChecked {
			t.Errorf("event %s ExpectedHashChecked = true, want false for bulk accept", ev.NodeID.String())
		}
		n := st.GetNode(ev.NodeID)
		if n.ValidatedContentHash != want || n.ValidatedHashChecked {
			t.Errorf("node %s = %q/%v, want %q/false", ev.NodeID.String(), n.ValidatedContentHash, n.ValidatedHashChecked, want)
		}
	}
}

// TestAcceptNodeBulk_CruxStaleClaimTestRejected verifies the bulk crux gate
// refuses a node whose only passing claim-test is stale.
func TestAcceptNodeBulk_CruxStaleClaimTestRejected(t *testing.T) {
	svc, _ := setupTestProof(t)
	childIDStr := refineCruxChild(t, svc)

	appendClaimTest(t, svc, childIDStr, true, "stale-hash")
	err := svc.AcceptNodeBulk([]NodeID{parseNodeID(t, childIDStr)})
	if err == nil {
		t.Fatal("expected bulk acceptance to be refused for a stale claim-test, got nil")
	}
	if !errors.Is(err, ErrClaimTestStale) {
		t.Errorf("error %v does not wrap ErrClaimTestStale", err)
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("error should name the stale test, got: %v", err)
	}
	if len(nodeValidatedEvents(t, svc)) != 0 {
		t.Error("no NodeValidated event should have been written")
	}
}
