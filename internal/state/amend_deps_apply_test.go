package state

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func mustID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return id
}

func addNode(t *testing.T, st *State, id string) *node.Node {
	t.Helper()
	n, err := node.NewNode(mustID(t, id), schema.NodeTypeClaim, "statement "+id, schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	st.AddNode(n)
	return n
}

func TestApplyNodeDepsAmended_ReplacesEdgesAndHistory(t *testing.T) {
	st := NewState()
	n := addNode(t, st, "1.1")
	addNode(t, st, "1.2")
	oldHash := n.ContentHash

	ev := ledger.NewNodeDepsAmended(mustID(t, "1.1"), nil, []types.NodeID{mustID(t, "1.2")}, nil, []types.NodeID{mustID(t, "1.2")},
		"owner", "reason", oldHash, false)
	if err := Apply(st, ev); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := st.GetNode(mustID(t, "1.1"))
	if len(got.Dependencies) != 1 || got.Dependencies[0].String() != "1.2" {
		t.Errorf("dependencies = %v", got.Dependencies)
	}
	if len(got.ValidationDeps) != 1 || got.ValidationDeps[0].String() != "1.2" {
		t.Errorf("validation deps = %v", got.ValidationDeps)
	}
	if got.ContentHash == oldHash {
		t.Errorf("hash did not change")
	}
	if got.ContentHash != got.ComputeContentHash() {
		t.Errorf("stored hash does not verify")
	}
	hist := st.GetAmendmentHistory(mustID(t, "1.1"))
	if len(hist) != 1 || hist[0].Kind != AmendmentKindDependencies || hist[0].Reason != "reason" {
		t.Fatalf("history = %+v", hist)
	}
}

func TestApplyNodeDepsAmended_PreviousMismatchIsReplayError(t *testing.T) {
	st := NewState()
	n := addNode(t, st, "1.1")
	addNode(t, st, "1.2")

	// Event claims a previous dependency that the state does not hold.
	ev := ledger.NewNodeDepsAmended(mustID(t, "1.1"), []types.NodeID{mustID(t, "1.2")}, []types.NodeID{mustID(t, "1.2")}, nil, nil,
		"owner", "reason", n.ContentHash, false)
	if err := Apply(st, ev); err == nil {
		t.Fatal("expected a replay mismatch error, got nil")
	}
	if len(n.Dependencies) != 0 {
		t.Errorf("mismatch overwrote dependencies: %v", n.Dependencies)
	}
	if len(st.GetAmendmentHistory(mustID(t, "1.1"))) != 0 {
		t.Errorf("mismatch recorded an amendment")
	}
}

func TestApplyNodeDepsAmended_ReopenClearsValidation(t *testing.T) {
	st := NewState()
	n := addNode(t, st, "1.1")
	addNode(t, st, "1.2")
	n.EpistemicState = schema.EpistemicValidated
	n.ValidatedBy = "v"
	n.ValidationBatchID = "b"
	n.ValidatedContentHash = n.ContentHash
	n.ValidatedHashChecked = true

	ev := ledger.NewNodeDepsAmended(mustID(t, "1.1"), nil, []types.NodeID{mustID(t, "1.2")}, nil, nil,
		"owner", "reason", n.ContentHash, true)
	if err := Apply(st, ev); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if n.EpistemicState != schema.EpistemicPending {
		t.Errorf("state = %s, want pending", n.EpistemicState)
	}
	if n.ValidatedBy != "" || n.ValidationBatchID != "" || n.ValidatedContentHash != "" || n.ValidatedHashChecked {
		t.Errorf("validation provenance not cleared: %+v", n)
	}
}

func TestApplyNodeAmended_ReopenClearsValidation(t *testing.T) {
	st := NewState()
	n := addNode(t, st, "1.1")
	n.EpistemicState = schema.EpistemicValidated
	n.ValidatedBy = "v"
	n.ValidatedContentHash = n.ContentHash

	ev := ledger.NewNodeAmended(mustID(t, "1.1"), "statement 1.1", "corrected", "owner")
	ev.Reopened = true
	if err := Apply(st, ev); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n.EpistemicState != schema.EpistemicPending {
		t.Errorf("state = %s, want pending", n.EpistemicState)
	}
	if n.Statement != "corrected" {
		t.Errorf("statement = %q", n.Statement)
	}
	if n.ValidatedBy != "" || n.ValidatedContentHash != "" {
		t.Errorf("validation provenance not cleared")
	}
}
