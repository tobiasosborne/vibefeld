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
	n.ValidatedSeq = seq
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
