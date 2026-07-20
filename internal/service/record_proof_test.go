package service

import (
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/schema"
)

// challengeRoot raises a blocking (major) challenge on node "1", turning it
// from a verifier job into a prover job.
func challengeRoot(t *testing.T, svc *ProofService) {
	t.Helper()
	if err := svc.RaiseChallengeWithBatch(parseNodeID(t, "1"), "ch-root", "statement", "needs proof", "major", "verifier-1", "gap", "b0"); err != nil {
		t.Fatalf("RaiseChallengeWithBatch: %v", err)
	}
}

func rootHash(t *testing.T, svc *ProofService) string {
	t.Helper()
	st, _ := svc.LoadState()
	return st.GetNode(parseNodeID(t, "1")).ContentHash
}

func isProverJobNow(t *testing.T, svc *ProofService, id string) bool {
	t.Helper()
	st, _ := svc.LoadState()
	return jobs.IsProverJob(st.GetNode(parseNodeID(t, id)), st.ChallengeMapForJobs())
}

// FU3 + B1: RecordProof on a prover-classified node atomically refines it,
// disposes the open blocking challenge, and (if the caller held the claim)
// releases it — so the node cycles OUT of prover territory. Its children become
// the new verifier frontier.
func TestRecordProof_RefinesDisposesChallengeAndCycles(t *testing.T) {
	svc, _ := setupTestProof(t)
	challengeRoot(t, svc)
	if !isProverJobNow(t, svc, "1") {
		t.Fatal("precondition: root should be a prover job after a blocking challenge")
	}

	res, err := svc.RecordProof(RecordProofSpec{
		ParentID:   parseNodeID(t, "1"),
		Owner:      "prover-x",
		ExpectHash: rootHash(t, svc),
		Children:   []ChildSpec{{NodeType: schema.NodeTypeClaim, Statement: "the proof step", Inference: schema.InferenceModusPonens}},
	})
	if err != nil {
		t.Fatalf("RecordProof: %v", err)
	}
	if len(res.ChildIDs) != 1 || res.ChildIDs[0].String() != "1.1" {
		t.Fatalf("ChildIDs = %v, want [1.1]", res.ChildIDs)
	}
	if len(res.ResolvedChallenges) != 1 {
		t.Errorf("ResolvedChallenges = %v, want 1", res.ResolvedChallenges)
	}

	st, _ := svc.LoadState()
	if st.GetNode(parseNodeID(t, "1.1")) == nil {
		t.Error("child 1.1 was not created")
	}
	if len(st.GetBlockingChallengesForNode(parseNodeID(t, "1"))) != 0 {
		t.Error("root still has an open blocking challenge after record-proof")
	}
	if isProverJobNow(t, svc, "1") {
		t.Error("root is still a prover job after record-proof (challenge not disposed)")
	}
}

// GAP 9: record-proof stamps the DECOMPOSED PARENT with the acting prover's
// identity as its proof-of-record author (ProofAuthor), symmetric with the
// author stamp its children already get. This is what a cross-vendor check
// reads for a decomposed node whose own Author is an unparseable init stamp.
func TestRecordProof_StampsProofAuthorOnParent(t *testing.T) {
	svc, _ := setupTestProof(t)
	challengeRoot(t, svc)

	_, err := svc.RecordProof(RecordProofSpec{
		ParentID:   parseNodeID(t, "1"),
		Owner:      "prover-x",
		ExpectHash: rootHash(t, svc),
		Children:   []ChildSpec{{NodeType: schema.NodeTypeClaim, Statement: "the proof step", Inference: schema.InferenceModusPonens}},
	})
	if err != nil {
		t.Fatalf("RecordProof: %v", err)
	}

	st, _ := svc.LoadState()
	root := st.GetNode(parseNodeID(t, "1"))
	if root.ProofAuthor != "prover-x" {
		t.Errorf("root ProofAuthor = %q, want %q (the prover that decomposed it)", root.ProofAuthor, "prover-x")
	}
}

// GAP 9: the proof-author stamp records ONLY ProofAuthor — the node's own
// content Author (here the "test-author" from init) must never be clobbered.
func TestRecordProof_DoesNotClobberExistingAuthor(t *testing.T) {
	svc, _ := setupTestProof(t)
	challengeRoot(t, svc)

	_, err := svc.RecordProof(RecordProofSpec{
		ParentID:   parseNodeID(t, "1"),
		Owner:      "prover-x",
		ExpectHash: rootHash(t, svc),
		Children:   []ChildSpec{{NodeType: schema.NodeTypeClaim, Statement: "the proof step", Inference: schema.InferenceModusPonens}},
	})
	if err != nil {
		t.Fatalf("RecordProof: %v", err)
	}

	st, _ := svc.LoadState()
	root := st.GetNode(parseNodeID(t, "1"))
	if root.Author != "test-author" {
		t.Errorf("root Author = %q, want %q unchanged (record-proof must not clobber the content author)", root.Author, "test-author")
	}
}

// GAP 9: a re-decomposition after a LATER challenge updates ProofAuthor to the
// new decomposer (the field reflects the current prover-of-record, not the
// first one).
func TestRecordProof_ReDecompositionUpdatesProofAuthor(t *testing.T) {
	svc, _ := setupTestProof(t)
	challengeRoot(t, svc)
	if _, err := svc.RecordProof(RecordProofSpec{
		ParentID:   parseNodeID(t, "1"),
		Owner:      "prover-x",
		ExpectHash: rootHash(t, svc),
		Children:   []ChildSpec{{NodeType: schema.NodeTypeClaim, Statement: "first decomposition", Inference: schema.InferenceModusPonens}},
	}); err != nil {
		t.Fatalf("first RecordProof: %v", err)
	}

	// A later challenge reopens the root as a prover job; a different prover re-proves it.
	if err := svc.RaiseChallengeWithBatch(parseNodeID(t, "1"), "ch-root-2", "statement", "still needs work", "major", "verifier-2", "gap", "b1"); err != nil {
		t.Fatalf("second RaiseChallenge: %v", err)
	}
	if _, err := svc.RecordProof(RecordProofSpec{
		ParentID:   parseNodeID(t, "1"),
		Owner:      "prover-y",
		ExpectHash: rootHash(t, svc),
		Children:   []ChildSpec{{NodeType: schema.NodeTypeClaim, Statement: "second decomposition", Inference: schema.InferenceModusPonens}},
	}); err != nil {
		t.Fatalf("second RecordProof: %v", err)
	}

	st, _ := svc.LoadState()
	root := st.GetNode(parseNodeID(t, "1"))
	if root.ProofAuthor != "prover-y" {
		t.Errorf("root ProofAuthor = %q, want %q (re-decomposition must update to the new decomposer)", root.ProofAuthor, "prover-y")
	}
}

// B1: a node that is NOT a prover job (fresh conjecture, no challenge) must be
// refused — a stale-role prover write.
func TestRecordProof_RejectsNonProverJob(t *testing.T) {
	svc, _ := setupTestProof(t)
	_, err := svc.RecordProof(RecordProofSpec{
		ParentID: parseNodeID(t, "1"),
		Owner:    "prover-x",
		Children: []ChildSpec{{NodeType: schema.NodeTypeClaim, Statement: "s", Inference: schema.InferenceModusPonens}},
	})
	if err == nil {
		t.Fatal("expected rejection: root is a verifier job, not a prover job")
	}
}

// B1: a stale expected hash is refused.
func TestRecordProof_RejectsHashMismatch(t *testing.T) {
	svc, _ := setupTestProof(t)
	challengeRoot(t, svc)
	_, err := svc.RecordProof(RecordProofSpec{
		ParentID:   parseNodeID(t, "1"),
		Owner:      "prover-x",
		ExpectHash: "deadbeef",
		Children:   []ChildSpec{{NodeType: schema.NodeTypeClaim, Statement: "s", Inference: schema.InferenceModusPonens}},
	})
	if err == nil {
		t.Fatal("expected rejection for a stale expect hash")
	}
}

// FU3: when the caller holds the claim, record-proof releases it.
func TestRecordProof_ReleasesOwnClaim(t *testing.T) {
	svc, _ := setupTestProof(t)
	challengeRoot(t, svc)
	if err := svc.ClaimNode(parseNodeID(t, "1"), "prover-x", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}
	res, err := svc.RecordProof(RecordProofSpec{
		ParentID:   parseNodeID(t, "1"),
		Owner:      "prover-x",
		ExpectHash: rootHash(t, svc),
		Children:   []ChildSpec{{NodeType: schema.NodeTypeClaim, Statement: "s", Inference: schema.InferenceModusPonens}},
	})
	if err != nil {
		t.Fatalf("RecordProof: %v", err)
	}
	if !res.Released {
		t.Error("expected the held claim to be released")
	}
	st, _ := svc.LoadState()
	if st.GetNode(parseNodeID(t, "1")).WorkflowState != schema.WorkflowAvailable {
		t.Errorf("root workflow = %q, want available after release", st.GetNode(parseNodeID(t, "1")).WorkflowState)
	}
}

// B1: refuse when the node is claimed by a DIFFERENT owner.
func TestRecordProof_RejectsClaimedByOther(t *testing.T) {
	svc, _ := setupTestProof(t)
	challengeRoot(t, svc)
	if err := svc.ClaimNode(parseNodeID(t, "1"), "other-agent", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}
	_, err := svc.RecordProof(RecordProofSpec{
		ParentID:   parseNodeID(t, "1"),
		Owner:      "prover-x",
		ExpectHash: rootHash(t, svc),
		Children:   []ChildSpec{{NodeType: schema.NodeTypeClaim, Statement: "s", Inference: schema.InferenceModusPonens}},
	})
	if err == nil {
		t.Fatal("expected rejection: node claimed by another owner")
	}
}
