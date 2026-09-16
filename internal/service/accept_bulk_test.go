package service

import (
	"strings"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// claimRoot claims node 1 for bulk-accept fixtures.
func claimRoot(t *testing.T, svc *ProofService) {
	t.Helper()
	if err := svc.ClaimNode(parseNodeID(t, "1"), "agent1", time.Hour); err != nil {
		t.Fatalf("claim root: %v", err)
	}
}

// refinePendingChild claims the root (once) and creates the named pending child.
func refinePendingChild(t *testing.T, svc *ProofService, id string) {
	t.Helper()
	if err := svc.Refine(RefineSpec{
		ParentID:  parseNodeID(t, "1"),
		Owner:     "agent1",
		ChildID:   parseNodeID(t, id),
		NodeType:  schema.NodeTypeClaim,
		Statement: "child " + id,
		Inference: schema.InferenceModusPonens,
	}); err != nil {
		t.Fatalf("refine %s: %v", id, err)
	}
}

// TestAcceptNodesBulk_SchedulesChildBeforeParent covers the vibefeld-hspn and
// D4 scheduling requirement: `af accept 1 1.2` where 1.2 is a pending child of
// 1 accepts 1.2 first and then 1, in one batch.
func TestAcceptNodesBulk_SchedulesChildBeforeParent(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRoot(t, svc)
	refinePendingChild(t, svc, "1.2")

	report, err := svc.AcceptNodesBulk([]types.NodeID{parseNodeID(t, "1"), parseNodeID(t, "1.2")}, "verifier", "")
	if err != nil {
		t.Fatalf("AcceptNodesBulk: %v (report %+v)", err, report)
	}
	if report.Applied != 2 || report.Blocked != 0 || report.Rejected != 0 {
		t.Fatalf("report = %+v, want 2 applied", report)
	}
	// The child must be scheduled first even though the parent sorts first.
	if len(report.Items) != 2 || report.Items[0].Node != "1.2" || report.Items[1].Node != "1" {
		t.Fatalf("schedule order = %+v, want 1.2 then 1", report.Items)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	for _, id := range []string{"1", "1.2"} {
		if got := st.GetNode(parseNodeID(t, id)).EpistemicState; got != schema.EpistemicValidated {
			t.Errorf("node %s = %s, want validated", id, got)
		}
	}
}

// TestAcceptNodesBulk_BlockedPrerequisitePending covers vibefeld-hspn: a parent
// whose pending child is not in the batch is blocked, never accepted.
func TestAcceptNodesBulk_BlockedPrerequisitePending(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRoot(t, svc)
	refinePendingChild(t, svc, "1.2")

	report, err := svc.AcceptNodesBulk([]types.NodeID{parseNodeID(t, "1")}, "verifier", "")
	if err == nil {
		t.Fatal("expected a partial/none exit error, got nil")
	}
	if report.Applied != 0 || report.Blocked != 1 {
		t.Fatalf("report = %+v, want 0 applied / 1 blocked", report)
	}
	if report.Items[0].Status != "blocked:prerequisite-pending" {
		t.Fatalf("status = %q, want blocked:prerequisite-pending", report.Items[0].Status)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got := st.GetNode(parseNodeID(t, "1")).EpistemicState; got != schema.EpistemicPending {
		t.Errorf("blocked parent = %s, want pending (never validated)", got)
	}
}

// TestAcceptNodesBulk_ValidationDepPendingIsBlocked covers the validation-dep
// half of prerequisite scheduling.
func TestAcceptNodesBulk_ValidationDepPendingIsBlocked(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRoot(t, svc)
	refinePendingChild(t, svc, "1.1")
	refinePendingChild(t, svc, "1.2")
	// 1.1.1 cites the pending 1.2 as a validation dependency.
	if err := svc.ClaimNode(parseNodeID(t, "1.1"), "agent1", time.Hour); err != nil {
		t.Fatalf("claim 1.1: %v", err)
	}
	if err := svc.Refine(RefineSpec{
		ParentID:       parseNodeID(t, "1.1"),
		Owner:          "agent1",
		ChildID:        parseNodeID(t, "1.1.1"),
		NodeType:       schema.NodeTypeClaim,
		Statement:      "dep target",
		Inference:      schema.InferenceModusPonens,
		ValidationDeps: []types.NodeID{parseNodeID(t, "1.2")},
	}); err != nil {
		t.Fatalf("refine 1.1.1: %v", err)
	}

	report, err := svc.AcceptNodesBulk([]types.NodeID{parseNodeID(t, "1.1.1")}, "verifier", "")
	if err == nil {
		t.Fatal("expected blocked outcome")
	}
	if report.Items[0].Status != "blocked:prerequisite-pending" {
		t.Fatalf("status = %q, want blocked:prerequisite-pending", report.Items[0].Status)
	}
}

// TestCheckAcceptEligibility_TypedErrors locks the blocked/rejected split.
func TestCheckAcceptEligibility_TypedErrors(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRoot(t, svc)
	refinePendingChild(t, svc, "1.1")

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	// Parent with a pending child is a blocked prerequisite.
	err = checkAcceptEligibility(st, st.GetNode(parseNodeID(t, "1")), AcceptOptions{})
	blocked, ok := asAcceptBlocked(err)
	if !ok || !blocked.IsPrerequisitePending() || blocked.Code != CodeChildrenNotValidated {
		t.Fatalf("parent error = %v, want blocked children-not-validated", err)
	}

	// A hash mismatch is rejected, not blocked.
	err = checkAcceptEligibility(st, st.GetNode(parseNodeID(t, "1.1")), AcceptOptions{ExpectHash: "nope"})
	rejected, ok := asAcceptRejected(err)
	if !ok || rejected.Code != CodeContentHashMismatch {
		t.Fatalf("hash error = %v, want rejected content-hash-mismatch", err)
	}
	if !strings.Contains(err.Error(), "content hash") {
		t.Fatalf("hash error message lacks detail: %v", err)
	}
}
