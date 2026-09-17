package support

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
)

// validateNode sets a node's validated verdict and validation sequence.
func validateNode(t *testing.T, st *state.State, id string, seq int) {
	t.Helper()
	n := st.GetNode(mustID(t, id))
	if n == nil {
		t.Fatalf("node %s not found", id)
	}
	n.EpistemicState = schema.EpistemicValidated
	n.VerdictSeq = seq
}

func TestCurrent_AllValidatedClean(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim)
	validateNode(t, st, "1.1", 2)
	validateNode(t, st, "1", 3)

	got := Current(st)
	if !got["1"].Current || !got["1.1"].Current {
		t.Fatalf("clean validated tree not current: %+v", got)
	}
}

func TestCurrent_NotValidated(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	got := Current(st)
	if got["1"].Current || got["1"].Cause != CauseNotValidated {
		t.Fatalf("pending node: %+v", got["1"])
	}
}

func TestCurrent_BlockingChallenge(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	validateNode(t, st, "1", 1)
	st.AddChallenge(&state.Challenge{
		ID: "c1", NodeID: mustID(t, "1"), Status: state.ChallengeStatusOpen, Severity: "major",
	})
	got := Current(st)
	if got["1"].Current || got["1"].Cause != CauseOpenBlockingChallenge {
		t.Fatalf("blocking challenge: %+v", got["1"])
	}
}

func TestCurrent_PendingChild(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim)
	validateNode(t, st, "1", 2)
	got := Current(st)
	if got["1"].Current || got["1"].Cause != CauseTargetPending {
		t.Fatalf("pending child: %+v", got["1"])
	}
	if got["1"].Node.String() != "1.1" {
		t.Fatalf("responsible node = %s, want 1.1", got["1"].Node)
	}
}

func TestCurrent_RefutedChild(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	c := addNode(t, st, "1.1", schema.NodeTypeClaim)
	validateNode(t, st, "1", 2)
	c.EpistemicState = schema.EpistemicRefuted
	got := Current(st)
	if got["1"].Current || got["1"].Cause != CauseTargetRefuted {
		t.Fatalf("refuted child: %+v", got["1"])
	}
}

func TestCurrent_ArchivedChildAllowed(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	c := addNode(t, st, "1.1", schema.NodeTypeClaim)
	validateNode(t, st, "1", 2)
	c.EpistemicState = schema.EpistemicArchived
	got := Current(st)
	if !got["1"].Current {
		t.Fatalf("archived child should not break support: %+v", got["1"])
	}
}

func TestCurrent_TargetRevised(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim)
	validateNode(t, st, "1.1", 2)
	validateNode(t, st, "1", 3)
	// A non-reopened statement amendment applied after 1 was validated.
	st.AddAmendment(mustID(t, "1.1"), state.Amendment{Kind: state.AmendmentKindStatement, Seq: 5})

	got := Current(st)
	if got["1"].Current || got["1"].Cause != CauseTargetRevised {
		t.Fatalf("target revised: %+v", got["1"])
	}
	if got["1"].Seq != 5 {
		t.Fatalf("revision seq = %d, want 5", got["1"].Seq)
	}
}

func TestCurrent_SelfRevised(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	validateNode(t, st, "1", 3)
	st.AddAmendment(mustID(t, "1"), state.Amendment{Kind: state.AmendmentKindDependencies, Seq: 6})

	got := Current(st)
	if got["1"].Current || got["1"].Cause != CauseSelfRevised {
		t.Fatalf("self revised: %+v", got["1"])
	}
}

func TestCurrent_Cycle(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim, "1.2")
	addNode(t, st, "1.2", schema.NodeTypeClaim, "1.1")
	validateNode(t, st, "1.1", 2)
	validateNode(t, st, "1.2", 3)
	validateNode(t, st, "1", 4)

	got := Current(st)
	if got["1.1"].Cause != CauseCycle && got["1.2"].Cause != CauseCycle {
		t.Fatalf("cycle members should report CYCLE: %+v", got)
	}
}

// TestCurrent_DescendantRevisionCarriesThroughCurrentTarget is the R -> B -> C
// regression an independent review found: C is revised, then C and B are
// re-accepted. B is current again, but R's older verdict predates C's revision
// and R was never re-accepted, so R must not be current. The fold carries each
// node's latest descendant revision sequence forward so R can see C's revision
// through the (now current) B.
func TestCurrent_DescendantRevisionCarriesThroughCurrentTarget(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)     // R
	addNode(t, st, "1.1", schema.NodeTypeClaim)   // B
	addNode(t, st, "1.1.1", schema.NodeTypeClaim) // C
	validateNode(t, st, "1.1.1", 2)
	validateNode(t, st, "1.1", 3)
	validateNode(t, st, "1", 4)

	// C is revised at seq 5, then re-accepted at 6; B is re-accepted at 7.
	st.AddAmendment(mustID(t, "1.1.1"), state.Amendment{Kind: state.AmendmentKindStatement, Seq: 5})
	validateNode(t, st, "1.1.1", 6)
	validateNode(t, st, "1.1", 7)

	got := Current(st)
	if !got["1.1"].Current {
		t.Fatalf("B re-accepted after C: expected current, got %+v", got["1.1"])
	}
	if got["1"].Current {
		t.Fatalf("R predates C's revision and must not be current: %+v", got["1"])
	}
	if got["1"].Cause != CauseTargetRevised {
		t.Fatalf("R cause = %q, want %q", got["1"].Cause, CauseTargetRevised)
	}
	if got["1"].Seq != 5 {
		t.Fatalf("R revision seq = %d, want 5", got["1"].Seq)
	}
}

// TestCurrent_ChildArchivedAfterVerdictBreaksParent is the reviewer's
// regression: a validated parent whose child was reopened is non-current, and
// archiving that child must not silently restore the parent's old verdict.
func TestCurrent_ChildArchivedAfterVerdictBreaksParent(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	c := addNode(t, st, "1.1", schema.NodeTypeClaim)
	validateNode(t, st, "1", 2)
	c.EpistemicState = schema.EpistemicArchived
	c.ArchivedSeq = 5

	got := Current(st)
	if got["1"].Current {
		t.Fatalf("parent of a child archived after its verdict must not be current: %+v", got["1"])
	}
	if got["1"].Cause != CauseChildArchivedAfterVerdict {
		t.Fatalf("cause = %q, want %q", got["1"].Cause, CauseChildArchivedAfterVerdict)
	}
	if got["1"].Node.String() != "1.1" || got["1"].Seq != 5 {
		t.Fatalf("responsible = %s/%d, want 1.1/5", got["1"].Node, got["1"].Seq)
	}
}

// A parent re-accepted after the child's archival (verdict seq > ArchivedSeq)
// is current: the fresh accept covers the abandoned branch.
func TestCurrent_ParentReacceptedAfterChildArchiveIsCurrent(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	c := addNode(t, st, "1.1", schema.NodeTypeClaim)
	c.EpistemicState = schema.EpistemicArchived
	c.ArchivedSeq = 5
	validateNode(t, st, "1", 6)

	got := Current(st)
	if !got["1"].Current {
		t.Fatalf("parent re-accepted after the archive should be current: %+v", got["1"])
	}
}

// A descendant archival propagates to an ancestor through a target that was
// itself re-accepted afterwards, exactly like an amendment revision.
func TestCurrent_DescendantArchiveCarriesThroughCurrentTarget(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)          // R
	addNode(t, st, "1.1", schema.NodeTypeClaim)        // B
	c := addNode(t, st, "1.1.1", schema.NodeTypeClaim) // C
	validateNode(t, st, "1.1.1", 2)
	validateNode(t, st, "1.1", 3)
	validateNode(t, st, "1", 4)

	// C is archived at seq 5; B is re-accepted at 6, but R's older verdict
	// predates the archival and must remain non-current.
	c.EpistemicState = schema.EpistemicArchived
	c.ArchivedSeq = 5
	validateNode(t, st, "1.1", 6)

	got := Current(st)
	if !got["1.1"].Current {
		t.Fatalf("B re-accepted after C's archive: expected current, got %+v", got["1.1"])
	}
	if got["1"].Current {
		t.Fatalf("R predates C's archive and must not be current: %+v", got["1"])
	}
	if got["1"].Cause != CauseTargetRevised || got["1"].Seq != 5 {
		t.Fatalf("R = %+v, want TARGET_REVISED seq 5", got["1"])
	}
}

// TestCurrent_PendingLocalAssumeChildBreaksParent locks the v3.2 amendment on
// the D4 side: a local_assume child is an ordinary result-use child edge, so a
// pending hypothesis (or a pending step under one) leaves its parent's recorded
// verdict TARGET_PENDING, exactly as any other pending child would. Citing a
// local_assume as a *dependency* is still hypothesis-use and carries nothing
// (TestResultUseEdges_LocalAssumeChildEdgesKept).
func TestCurrent_PendingLocalAssumeChildBreaksParent(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeLocalAssume)
	validateNode(t, st, "1", 2)

	got := Current(st)
	if got["1"].Current || got["1"].Cause != CauseTargetPending || got["1"].Node.String() != "1.1" {
		t.Fatalf("pending local_assume child must break the parent: %+v", got["1"])
	}

	// The same holds for the children of a local_assume parent: the derivation
	// under the hypothesis is work the local_assume's verdict rests on.
	addNode(t, st, "1.1.1", schema.NodeTypeClaim)
	validateNode(t, st, "1.1", 3)
	got = Current(st)
	if got["1.1"].Current || got["1.1"].Cause != CauseTargetPending || got["1.1"].Node.String() != "1.1.1" {
		t.Fatalf("pending child of a local_assume must break it: %+v", got["1.1"])
	}

	// With both hypothesis nodes validated the parent is current again.
	validateNode(t, st, "1.1.1", 4)
	validateNode(t, st, "1.1", 5)
	validateNode(t, st, "1", 6)
	got = Current(st)
	if !got["1"].Current {
		t.Fatalf("validated hypothesis subtree should be current: %+v", got["1"])
	}
}

// TestCurrent_AdmittedConsumerTargetRevised locks that an admitted node's
// verdict baseline is compared against later target revisions exactly like a
// validated one.
func TestCurrent_AdmittedConsumerTargetRevised(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim)
	addNode(t, st, "1.1", schema.NodeTypeClaim)
	validateNode(t, st, "1.1", 2)
	p := st.GetNode(mustID(t, "1"))
	p.EpistemicState = schema.EpistemicAdmitted
	p.VerdictSeq = 5
	st.AddAmendment(mustID(t, "1.1"), state.Amendment{Kind: state.AmendmentKindStatement, Seq: 6})

	got := Current(st)
	if got["1"].Current || got["1"].Cause != CauseTargetRevised {
		t.Fatalf("admitted consumer with a later target revision: %+v", got["1"])
	}
}
