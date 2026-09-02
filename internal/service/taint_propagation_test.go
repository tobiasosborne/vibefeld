package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

func TestTaintRecomputedEventsIncludeChangedAncestors(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")
	childID := parseNodeID(t, "1.1")
	if err := svc.CreateNode(childID, schema.NodeTypeClaim, "Admitted step", schema.InferenceAssumption); err != nil {
		t.Fatal(err)
	}
	if err := svc.AdmitNode(childID); err != nil {
		t.Fatal(err)
	}
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatal(err)
	}

	ldg, err := svc.getLedger()
	if err != nil {
		t.Fatal(err)
	}
	records, err := ldg.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	got := make(map[string]node.TaintState)
	for _, record := range records {
		var header struct {
			Type ledger.EventType `json:"type"`
		}
		if err := json.Unmarshal(record, &header); err != nil || header.Type != ledger.EventTaintRecomputed {
			continue
		}
		var event ledger.TaintRecomputed
		if err := json.Unmarshal(record, &event); err != nil {
			t.Fatal(err)
		}
		got[event.NodeID.String()] = event.NewTaint
	}

	if got["1"] != node.TaintTainted {
		t.Errorf("root TaintRecomputed event = %q, want %q", got["1"], node.TaintTainted)
	}
	if got["1.1"] != node.TaintSelfAdmitted {
		t.Errorf("child TaintRecomputed event = %q, want %q", got["1.1"], node.TaintSelfAdmitted)
	}
}

func TestRecomputeAllTaint_ResyncsStaleAuditTrail(t *testing.T) {
	svc, _ := setupTestProof(t)
	ldg, err := svc.getLedger()
	if err != nil {
		t.Fatal(err)
	}

	rootID := parseNodeID(t, "1")
	childID := parseNodeID(t, "1.1")
	child, err := node.NewNode(childID, schema.NodeTypeClaim, "Admitted child", schema.InferenceAssumption)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []ledger.Event{
		ledger.NewNodeCreated(*child),
		ledger.NewNodeAdmitted(childID),
		ledger.NewNodeValidated(rootID),
		ledger.NewTaintRecomputed(rootID, node.TaintClean),
	} {
		if _, err := ldg.Append(event); err != nil {
			t.Fatalf("Append(%s): %v", event.Type(), err)
		}
	}

	dryRun, err := svc.RecomputeAllTaint(true)
	if err != nil {
		t.Fatal(err)
	}
	assertTaintChange(t, dryRun.Changes, "1", node.TaintClean, node.TaintTainted)

	before, err := ldg.Count()
	if err != nil {
		t.Fatal(err)
	}
	applied, err := svc.RecomputeAllTaint(false)
	if err != nil {
		t.Fatal(err)
	}
	assertTaintChange(t, applied.Changes, "1", node.TaintClean, node.TaintTainted)
	after, err := ldg.Count()
	if err != nil {
		t.Fatal(err)
	}
	if after-before != applied.NodesChanged {
		t.Errorf("ledger grew by %d events, want %d", after-before, applied.NodesChanged)
	}

	second, err := svc.RecomputeAllTaint(false)
	if err != nil {
		t.Fatal(err)
	}
	if second.NodesChanged != 0 {
		t.Errorf("second recompute changed %d nodes, want 0: %#v", second.NodesChanged, second.Changes)
	}
}

func TestRefineEmitsChangedAncestorTaintAudit(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")
	childID := parseNodeID(t, "1.1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatal(err)
	}
	if err := svc.RequestRefinement(rootID, "show details", "verifier"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClaimNode(rootID, "prover", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := svc.RefineNode(rootID, "prover", childID, schema.NodeTypeClaim, "Pending detail", schema.InferenceAssumption); err != nil {
		t.Fatal(err)
	}

	ldg, err := svc.getLedger()
	if err != nil {
		t.Fatal(err)
	}
	audited, err := lastAuditedTaintStates(ldg)
	if err != nil {
		t.Fatal(err)
	}
	if got := audited["1"]; got != node.TaintUnresolved {
		t.Errorf("root audit after refinement = %q, want %q", got, node.TaintUnresolved)
	}
}

func TestRefineNodeBulkEmitsChangedAncestorTaintAudit(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatal(err)
	}
	if err := svc.RequestRefinement(rootID, "show cases", "verifier"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClaimNode(rootID, "prover", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	children := []ChildSpec{
		{Statement: "Pending case one", NodeType: schema.NodeTypeClaim, Inference: schema.InferenceAssumption},
		{Statement: "Pending case two", NodeType: schema.NodeTypeClaim, Inference: schema.InferenceAssumption},
	}
	childIDs, err := svc.RefineNodeBulk(rootID, "prover", children)
	if err != nil {
		t.Fatal(err)
	}

	ldg, err := svc.getLedger()
	if err != nil {
		t.Fatal(err)
	}
	audited, err := lastAuditedTaintStates(ldg)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range append([]types.NodeID{rootID}, childIDs...) {
		if got := audited[id.String()]; got != node.TaintUnresolved {
			t.Errorf("audit for %s after bulk refinement = %q, want %q", id, got, node.TaintUnresolved)
		}
	}
}

func TestRequestRefinementEmitsUnresolvedAuditAndReacceptClearsIt(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")
	childID := parseNodeID(t, "1.1")
	grandchildID := parseNodeID(t, "1.1.1")
	if err := svc.CreateNode(childID, schema.NodeTypeClaim, "Reopen this step", schema.InferenceAssumption); err != nil {
		t.Fatal(err)
	}
	if err := svc.AcceptNode(childID); err != nil {
		t.Fatal(err)
	}
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatal(err)
	}
	if err := svc.RequestRefinement(childID, "needs more proof", "verifier"); err != nil {
		t.Fatal(err)
	}

	ldg, err := svc.getLedger()
	if err != nil {
		t.Fatal(err)
	}
	audited, err := lastAuditedTaintStates(ldg)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []types.NodeID{rootID, childID} {
		if got := audited[id.String()]; got != node.TaintUnresolved {
			t.Errorf("audit for %s after request-refinement = %q, want %q", id, got, node.TaintUnresolved)
		}
	}

	if err := svc.ClaimNode(childID, "prover", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := svc.RefineNode(childID, "prover", grandchildID, schema.NodeTypeClaim, "Added proof", schema.InferenceAssumption); err != nil {
		t.Fatal(err)
	}
	if err := svc.AcceptNode(grandchildID); err != nil {
		t.Fatal(err)
	}
	if err := svc.AcceptNode(childID); err != nil {
		t.Fatal(err)
	}

	audited, err = lastAuditedTaintStates(ldg)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []types.NodeID{rootID, childID, grandchildID} {
		if got := audited[id.String()]; got != node.TaintClean {
			t.Errorf("audit for %s after re-acceptance = %q, want %q", id, got, node.TaintClean)
		}
	}
}

func assertTaintChange(t *testing.T, changes []TaintChange, nodeID string, oldTaint, newTaint node.TaintState) {
	t.Helper()
	for _, change := range changes {
		if change.NodeID == nodeID {
			if change.OldTaint != TaintState(oldTaint) || change.NewTaint != TaintState(newTaint) {
				t.Errorf("change for %s = %s→%s, want %s→%s", nodeID, change.OldTaint, change.NewTaint, oldTaint, newTaint)
			}
			return
		}
	}
	t.Errorf("no taint change found for %s in %#v", nodeID, changes)
}
