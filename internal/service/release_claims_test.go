package service

import (
	"errors"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// TestReleaseExpiredClaims_SelectsInCommitState verifies the expired path
// releases exactly the claimed-and-expired nodes and reports them.
func TestReleaseExpiredClaims_SelectsInCommitState(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	rootID := parseNodeID(t, "1")
	childID := parseNodeID(t, "1.1")

	// The root is claimed for an hour (not expired); the child expires almost
	// immediately.
	if err := svc.ClaimNode(childID, "prover-1", time.Nanosecond); err != nil {
		t.Fatalf("ClaimNode(child): %v", err)
	}
	time.Sleep(2 * time.Millisecond)

	released, err := svc.ReleaseExpiredClaims(time.Now())
	if err != nil {
		t.Fatalf("ReleaseExpiredClaims: %v", err)
	}
	if len(released) != 1 || released[0].String() != childID.String() {
		t.Fatalf("released = %v, want [%s]", released, childID.String())
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got := st.GetNode(childID).WorkflowState; got != schema.WorkflowAvailable {
		t.Fatalf("child workflow state = %q, want available", got)
	}
	// The long-lived root claim is untouched.
	if got := st.GetNode(rootID).WorkflowState; got != schema.WorkflowClaimed {
		t.Fatalf("root workflow state = %q, want claimed", got)
	}
}

// TestReleaseExpiredClaims_RefreshInWindowNotReleased is the item-1 race test:
// a claim refreshed between the commit's selection read and its append must
// not be released. The beforeAppend hook lands the refresh in exactly that
// window; the CAS must refuse the release.
func TestReleaseExpiredClaims_RefreshInWindowNotReleased(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	childID := parseNodeID(t, "1.1")

	if err := svc.ClaimNode(childID, "prover-1", time.Nanosecond); err != nil {
		t.Fatalf("ClaimNode(child): %v", err)
	}
	time.Sleep(2 * time.Millisecond)

	svc.beforeAppend = func() {
		if _, err := ledger.Append(svc.ledgerDir(),
			ledger.NewClaimRefreshed(childID, "prover-1", types.FromTime(time.Now().Add(time.Hour)))); err != nil {
			t.Errorf("concurrent refresh Append: %v", err)
		}
	}
	defer func() { svc.beforeAppend = nil }()

	released, err := svc.ReleaseExpiredClaims(time.Now())
	if !errors.Is(err, ErrConcurrentModification) {
		t.Fatalf("ReleaseExpiredClaims err = %v, want ErrConcurrentModification", err)
	}
	if len(released) != 0 {
		t.Fatalf("released = %v, want none on a conflict", released)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got := st.GetNode(childID).WorkflowState; got != schema.WorkflowClaimed {
		t.Fatalf("child workflow state = %q, want still claimed", got)
	}
}

// TestReleaseAllClaims_AlreadyAvailableNoEvent verifies that releasing a set
// with no claimed nodes emits no NodesReleased event (an available->available
// transition would break replay) and reports nothing released.
func TestReleaseAllClaims_AlreadyAvailableNoEvent(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	rootID := parseNodeID(t, "1")

	if err := svc.ReleaseNode(rootID, "prover-1"); err != nil {
		t.Fatalf("ReleaseNode: %v", err)
	}
	before, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	released, err := svc.ReleaseAllClaims()
	if err != nil {
		t.Fatalf("ReleaseAllClaims: %v", err)
	}
	if len(released) != 0 {
		t.Fatalf("released = %v, want none", released)
	}

	after, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState after: %v", err)
	}
	if after.LatestSeq() != before.LatestSeq() {
		t.Fatalf("no-op release appended an event: latest seq %d -> %d", before.LatestSeq(), after.LatestSeq())
	}
}

// TestReleaseAllClaims_ReleasesClaimed verifies --all semantics: all claimed
// nodes are released regardless of expiry.
func TestReleaseAllClaims_ReleasesClaimed(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	rootID := parseNodeID(t, "1")

	released, err := svc.ReleaseAllClaims()
	if err != nil {
		t.Fatalf("ReleaseAllClaims: %v", err)
	}
	if len(released) != 1 || released[0].String() != rootID.String() {
		t.Fatalf("released = %v, want [%s]", released, rootID.String())
	}
}
