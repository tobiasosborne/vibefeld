package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/support"
	"github.com/tobiasosborne/vibefeld/internal/types"
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

// TestTaint_AdmittedStepUnderHypothesisTaintsRoot is the regression test for
// the D6 blocker found in review: with the pre-amendment support relation a
// local_assume subtree was disconnected from the fold in both directions, so an
// admitted step under a hypothesis left the enclosing proof validated/clean.
// It reproduces the reviewer's eight-command scenario through the service and
// asserts the derived taint after a full ledger replay.
func TestTaint_AdmittedStepUnderHypothesisTaintsRoot(t *testing.T) {
	svc, _ := setupTestProof(t)
	root := parseNodeID(t, "1")
	assume := parseNodeID(t, "1.1")
	discharge := parseNodeID(t, "1.2")
	deep := parseNodeID(t, "1.1.1")

	if err := svc.ClaimNode(root, "prover", time.Hour); err != nil {
		t.Fatalf("claim root: %v", err)
	}
	if err := svc.Refine(RefineSpec{
		ParentID: root, Owner: "prover", ChildID: assume,
		NodeType: schema.NodeTypeLocalAssume, Statement: "Assume not R",
		Inference: schema.InferenceLocalAssume,
	}); err != nil {
		t.Fatalf("refine 1.1: %v", err)
	}
	if err := svc.Refine(RefineSpec{
		ParentID: root, Owner: "prover", ChildID: discharge,
		NodeType: schema.NodeTypeLocalDischarge, Statement: "Discharge",
		Inference: schema.InferenceLocalDischarge, Dependencies: []types.NodeID{assume},
	}); err != nil {
		t.Fatalf("refine 1.2: %v", err)
	}
	if err := svc.ReleaseNode(root, "prover"); err != nil {
		t.Fatalf("release root: %v", err)
	}
	if err := svc.ClaimNode(assume, "prover", time.Hour); err != nil {
		t.Fatalf("claim 1.1: %v", err)
	}
	if err := svc.Refine(RefineSpec{
		ParentID: assume, Owner: "prover", ChildID: deep,
		NodeType: schema.NodeTypeClaim, Statement: "Deep step under the hypothesis",
		Inference: schema.InferenceModusPonens,
	}); err != nil {
		t.Fatalf("refine 1.1.1: %v", err)
	}
	if err := svc.ReleaseNode(assume, "prover"); err != nil {
		t.Fatalf("release 1.1: %v", err)
	}
	if err := svc.AdmitNode(deep); err != nil {
		t.Fatalf("admit 1.1.1: %v", err)
	}
	for _, id := range []types.NodeID{assume, discharge, root} {
		if err := svc.AcceptNode(id); err != nil {
			t.Fatalf("accept %s: %v", id.String(), err)
		}
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	want := map[string]node.TaintState{
		"1":     node.TaintTainted,      // the admitted step is under its hypothesis
		"1.1":   node.TaintTainted,      // the hypothesis's own derivation is admitted
		"1.1.1": node.TaintSelfAdmitted, // the escape hatch itself
		"1.2":   node.TaintClean,        // the discharge cites the hypothesis, not the step
	}
	for id, w := range want {
		n := st.GetNode(parseNodeID(t, id))
		if n == nil {
			t.Fatalf("node %s missing after replay", id)
		}
		if n.TaintState != w {
			t.Errorf("node %s taint = %s, want %s", id, n.TaintState, w)
		}
	}
}

// TestRefutedChild_TaintCleanButSupportNotCurrent pins the deliberate division
// of labour between the two signals (D6 review item 2): after a child is
// refuted the parent's taint is clean -- the severed step is no longer part of
// what the proof rests on -- while support_current reports TARGET_REFUTED, which
// is what tells a reader the parent's recorded verdict must be re-earned.
func TestRefutedChild_TaintCleanButSupportNotCurrent(t *testing.T) {
	svc, _ := setupTestProof(t)
	root := parseNodeID(t, "1")
	child := parseNodeID(t, "1.1")

	if err := svc.CreateNode(child, schema.NodeTypeClaim, "Step", schema.InferenceAssumption); err != nil {
		t.Fatal(err)
	}
	if err := svc.AcceptNode(child); err != nil {
		t.Fatalf("accept child: %v", err)
	}
	if err := svc.AcceptNode(root); err != nil {
		t.Fatalf("accept root: %v", err)
	}
	if err := svc.RequestRefinement(child, "counterexample found", "verifier1"); err != nil {
		t.Fatalf("request refinement: %v", err)
	}
	if err := svc.RefuteNode(child); err != nil {
		t.Fatalf("refute child: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if got := st.GetNode(root).TaintState; got != node.TaintClean {
		t.Errorf("root taint = %s, want clean (a refuted child is severed)", got)
	}
	if got := st.GetNode(child).TaintState; got != node.TaintClean {
		t.Errorf("refuted child taint = %s, want clean", got)
	}
	sup := support.Current(st)["1"]
	if sup.Current || sup.Cause != support.CauseTargetRefuted || sup.Node.String() != "1.1" {
		t.Errorf("root support_current = %+v, want not-current TARGET_REFUTED on 1.1", sup)
	}
}
