package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// claimGeneration returns the node's current derived claim generation.
func claimGeneration(t *testing.T, svc *ProofService, id string) int {
	t.Helper()
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	n := st.GetNode(parseNodeID(t, id))
	if n == nil {
		t.Fatalf("node %s not found", id)
	}
	return n.ClaimSeq
}

// workflowState returns the node's workflow state after replay.
func workflowState(t *testing.T, svc *ProofService, id string) schema.WorkflowState {
	t.Helper()
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	n := st.GetNode(parseNodeID(t, id))
	if n == nil {
		t.Fatalf("node %s not found", id)
	}
	return n.WorkflowState
}

// D5: an accept by the claim holder releases the claim in the same event and
// replay leaves the node available.
func TestAcceptNode_ReleasesHeldClaimInSameEvent(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")

	if err := svc.ClaimNode(rootID, "verifier-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}
	gen := claimGeneration(t, svc, "1")
	if gen == 0 {
		t.Fatal("claim generation = 0 after claim")
	}

	if err := svc.AcceptNodeInteractive(rootID, "", "verifier-1", "", false); err != nil {
		t.Fatalf("AcceptNodeInteractive: %v", err)
	}

	ev := lastNodeValidatedFor(t, svc, "1")
	if !ev.ReleaseClaim {
		t.Error("NodeValidated.ReleaseClaim = false, want true")
	}
	if ev.ClaimSeq != gen {
		t.Errorf("NodeValidated.ClaimSeq = %d, want %d", ev.ClaimSeq, gen)
	}
	if got := workflowState(t, svc, "1"); got != schema.WorkflowAvailable {
		t.Errorf("workflow state after accept = %q, want %q", got, schema.WorkflowAvailable)
	}
	if got := claimGeneration(t, svc, "1"); got != 0 {
		t.Errorf("claim generation after accept = %d, want 0", got)
	}
}

// D5: accept by a caller who does not hold the claim does not release it.
func TestAcceptNode_DoesNotReleaseOthersClaim(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")

	if err := svc.ClaimNode(rootID, "prover-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}
	gen := claimGeneration(t, svc, "1")

	if err := svc.AcceptNodeInteractive(rootID, "", "verifier-1", "", false); err != nil {
		t.Fatalf("AcceptNodeInteractive: %v", err)
	}
	ev := lastNodeValidatedFor(t, svc, "1")
	if ev.ReleaseClaim {
		t.Error("NodeValidated.ReleaseClaim = true, want false for a non-holder")
	}
	if got := claimGeneration(t, svc, "1"); got != gen {
		t.Errorf("claim generation = %d, want unchanged %d", got, gen)
	}
	if got := workflowState(t, svc, "1"); got != schema.WorkflowClaimed {
		t.Errorf("workflow state = %q, want %q", got, schema.WorkflowClaimed)
	}
}

// D5: a hand-built terminal event carrying a stale claim_seq must not release
// the node's current (later) claim when replayed.
func TestReplay_StaleClaimSeqDoesNotRelease(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")

	if err := svc.ClaimNode(rootID, "agent-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}
	gen := claimGeneration(t, svc, "1")

	ev := ledger.NewNodeValidated(rootID)
	ev.ClaimSeq = gen + 100 // a generation that never existed
	ev.ReleaseClaim = true
	if _, err := ledger.Append(svc.ledgerDir(), ev); err != nil {
		t.Fatalf("append stale-release event: %v", err)
	}

	if got := workflowState(t, svc, "1"); got != schema.WorkflowClaimed {
		t.Errorf("stale claim_seq released the claim: workflow = %q, want %q", got, schema.WorkflowClaimed)
	}
	if got := claimGeneration(t, svc, "1"); got != gen {
		t.Errorf("claim generation = %d, want unchanged %d", got, gen)
	}
}

// D5: a hand-built terminal event whose claim_seq matches the current claim
// generation releases it.
func TestReplay_MatchingClaimSeqReleases(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")

	if err := svc.ClaimNode(rootID, "agent-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}
	gen := claimGeneration(t, svc, "1")

	ev := ledger.NewNodeValidated(rootID)
	ev.ClaimSeq = gen
	ev.ReleaseClaim = true
	if _, err := ledger.Append(svc.ledgerDir(), ev); err != nil {
		t.Fatalf("append release event: %v", err)
	}

	if got := workflowState(t, svc, "1"); got != schema.WorkflowAvailable {
		t.Errorf("workflow = %q, want %q", got, schema.WorkflowAvailable)
	}
}

// D5: legacy terminal events without the release fields leave the claim in
// place, exactly as before.
func TestReplay_LegacyTerminalEventKeepsClaim(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")

	if err := svc.ClaimNode(rootID, "agent-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}

	if _, err := ledger.Append(svc.ledgerDir(), ledger.NewNodeValidated(rootID)); err != nil {
		t.Fatalf("append legacy event: %v", err)
	}

	if got := workflowState(t, svc, "1"); got != schema.WorkflowClaimed {
		t.Errorf("legacy terminal event changed workflow to %q, want %q", got, schema.WorkflowClaimed)
	}
}

// D5: admit/refute/archive also auto-release when the acting identity holds
// the claim.
func TestTerminalActions_ReleaseHeldClaim(t *testing.T) {
	cases := []struct {
		name string
		run  func(svc *ProofService, id types.NodeID) error
	}{
		{"admit", func(svc *ProofService, id types.NodeID) error {
			return svc.AdmitNodeWithAgent(id, "agent-1")
		}},
		{"refute", func(svc *ProofService, id types.NodeID) error {
			return svc.RefuteNodeWithAgent(id, "agent-1")
		}},
		{"archive", func(svc *ProofService, id types.NodeID) error {
			return svc.ArchiveNodeWithOptions(id, ArchiveOptions{By: "agent-1"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := setupTestProof(t)
			rootID := parseNodeID(t, "1")
			if err := svc.ClaimNode(rootID, "agent-1", time.Hour); err != nil {
				t.Fatalf("ClaimNode: %v", err)
			}
			if err := tc.run(svc, rootID); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if got := workflowState(t, svc, "1"); got != schema.WorkflowAvailable {
				t.Errorf("workflow after %s = %q, want %q", tc.name, got, schema.WorkflowAvailable)
			}
		})
	}
}

// D5: when the node is re-claimed after the accept built its event but before
// the append, the commit's CAS refuses the stale release rather than evicting
// the new claim.
func TestAcceptNode_ReclaimedBeforeAppendIsCASRefused(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")

	if err := svc.ClaimNode(rootID, "agent-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}

	svc.beforeAppend = func() {
		svc.beforeAppend = nil
		if _, err := ledger.Append(svc.ledgerDir(), ledger.NewNodesReleased([]types.NodeID{rootID})); err != nil {
			t.Errorf("concurrent release: %v", err)
		}
		if _, err := ledger.Append(svc.ledgerDir(), ledger.NewNodesClaimed(
			[]types.NodeID{rootID}, "agent-1", types.FromTime(time.Now().Add(time.Hour)))); err != nil {
			t.Errorf("concurrent re-claim: %v", err)
		}
	}
	defer func() { svc.beforeAppend = nil }()

	err := svc.AcceptNodeInteractive(rootID, "", "agent-1", "", false)
	if !errors.Is(err, ErrConcurrentModification) {
		t.Fatalf("accept error = %v, want ErrConcurrentModification", err)
	}

	// The new claim must survive: workflow claimed and a fresh generation.
	if got := workflowState(t, svc, "1"); got != schema.WorkflowClaimed {
		t.Errorf("workflow = %q, want %q", got, schema.WorkflowClaimed)
	}
	if got := claimGeneration(t, svc, "1"); got == 0 {
		t.Error("claim generation = 0, want the re-claim's generation")
	}
}

// D9: --allow-self is recorded, and the reviewer check compares against the
// node author, proof author and amendment owners.
func TestReviewerContributorChecks(t *testing.T) {
	t.Run("author rejected without allow-self", func(t *testing.T) {
		svc, _ := setupTestProof(t)
		err := svc.AcceptNodeInteractive(parseNodeID(t, "1"), "", "test-author", "", false)
		if !errors.Is(err, errVerdictReviewerIsAuthor) {
			t.Fatalf("error = %v, want reviewer-is-author", err)
		}
	})

	t.Run("author allowed and recorded with allow-self", func(t *testing.T) {
		svc, _ := setupTestProof(t)
		if err := svc.AcceptNodeInteractive(parseNodeID(t, "1"), "", "test-author", "", true); err != nil {
			t.Fatalf("AcceptNodeInteractive --allow-self: %v", err)
		}
		ev := lastNodeValidatedFor(t, svc, "1")
		if !ev.SelfAccepted {
			t.Error("SelfAccepted = false, want true")
		}
	})

	t.Run("proof author rejected", func(t *testing.T) {
		svc, _ := setupTestProof(t)
		rootID := parseNodeID(t, "1")
		if _, err := ledger.Append(svc.ledgerDir(), ledger.NewNodeProofAuthored(rootID, "prover-7")); err != nil {
			t.Fatalf("append proof author: %v", err)
		}
		err := svc.AcceptNodeInteractive(rootID, "", "prover-7", "", false)
		if !errors.Is(err, errVerdictReviewerIsAuthor) {
			t.Fatalf("error = %v, want reviewer-is-contributor", err)
		}
	})

	t.Run("amender rejected", func(t *testing.T) {
		svc, _ := setupTestProof(t)
		rootID := parseNodeID(t, "1")
		if err := svc.AmendNode(rootID, "editor-3", "restated conjecture"); err != nil {
			t.Fatalf("AmendNode: %v", err)
		}
		err := svc.AcceptNodeInteractive(rootID, "", "editor-3", "", false)
		if !errors.Is(err, errVerdictReviewerIsAuthor) {
			t.Fatalf("error = %v, want reviewer-is-contributor", err)
		}
	})
}

// D9: verdict files cannot opt out of the reviewer check (no AllowSelf path),
// and the broader contributor check applies to them too.
func TestVerdicts_CannotAllowSelf(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")
	if _, err := ledger.Append(svc.ledgerDir(), ledger.NewNodeProofAuthored(rootID, "prover-7")); err != nil {
		t.Fatalf("append proof author: %v", err)
	}
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	err = checkAcceptEligibility(st, st.GetNode(rootID), AcceptOptions{
		VerifiedBy:          "prover-7",
		CheckReviewerAuthor: true,
	})
	if !errors.Is(err, errVerdictReviewerIsAuthor) {
		t.Fatalf("verdict-style check error = %v, want reviewer-is-contributor", err)
	}
}

// The open-challenge archive guard runs against the node and any active
// descendant, and --force --reason records the override.
func TestArchiveNode_RefusesOpenChallengeUnlessForced(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")

	if err := svc.RaiseChallengeWithBatch(rootID, "ch-1", "statement", "wrong", "major", "verifier-9", "", ""); err != nil {
		t.Fatalf("RaiseChallengeWithBatch: %v", err)
	}

	err := svc.ArchiveNodeWithOptions(rootID, ArchiveOptions{})
	if !errors.Is(err, ErrOpenChallengeObligation) {
		t.Fatalf("archive error = %v, want ErrOpenChallengeObligation", err)
	}

	err = svc.ArchiveNodeWithOptions(rootID, ArchiveOptions{Force: true})
	if !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("force without reason error = %v, want ErrEmptyInput", err)
	}

	if err := svc.ArchiveNodeWithOptions(rootID, ArchiveOptions{Force: true, Reason: "abandon", By: "agent-1"}); err != nil {
		t.Fatalf("forced archive: %v", err)
	}
	ev := lastNodeArchivedFor(t, svc, "1")
	if !ev.Forced || ev.Reason != "abandon" || ev.By != "agent-1" {
		t.Errorf("NodeArchived = %+v, want forced/reason/by recorded", ev)
	}
}

// A challenge on an active descendant blocks archiving the parent too.
func TestArchiveNode_RefusesChallengeOnActiveDescendant(t *testing.T) {
	svc, _ := setupTestProof(t)
	rootID := parseNodeID(t, "1")
	if err := svc.ClaimNode(rootID, "prover-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}
	ids, err := svc.RefineNodeBulk(rootID, "prover-1", []ChildSpec{{
		NodeType:  schema.NodeTypeClaim,
		Statement: "child step",
		Inference: schema.InferenceModusPonens,
	}})
	if err != nil {
		t.Fatalf("RefineNodeBulk: %v", err)
	}
	childID := ids[0]
	if err := svc.RaiseChallengeWithBatch(childID, "ch-child", "gap", "missing", "major", "verifier-9", "", ""); err != nil {
		t.Fatalf("RaiseChallengeWithBatch: %v", err)
	}

	err = svc.ArchiveNodeWithOptions(rootID, ArchiveOptions{})
	if !errors.Is(err, ErrOpenChallengeObligation) {
		t.Fatalf("archive error = %v, want ErrOpenChallengeObligation naming the child", err)
	}
	if !strings.Contains(err.Error(), childID.String()) {
		t.Errorf("error %q should name child %s", err.Error(), childID)
	}
}

// lastNodeArchivedFor returns the most recent NodeArchived event for nodeID.
func lastNodeArchivedFor(t *testing.T, svc *ProofService, id string) ledger.NodeArchived {
	t.Helper()
	ldg, err := svc.getLedger()
	if err != nil {
		t.Fatalf("getLedger: %v", err)
	}
	records, err := ldg.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	var found *ledger.NodeArchived
	for _, record := range records {
		var header struct {
			Type ledger.EventType `json:"type"`
		}
		if err := json.Unmarshal(record, &header); err != nil || header.Type != ledger.EventNodeArchived {
			continue
		}
		var ev ledger.NodeArchived
		if err := json.Unmarshal(record, &ev); err != nil {
			t.Fatalf("unmarshal NodeArchived: %v", err)
		}
		if ev.NodeID.String() == id {
			e := ev
			found = &e
		}
	}
	if found == nil {
		t.Fatalf("no NodeArchived event for %s", id)
	}
	return *found
}
