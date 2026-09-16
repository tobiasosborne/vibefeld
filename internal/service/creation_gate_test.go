package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// TestRefineNodeBulk_ValidatedParentRefused wires the gate into bulk refine.
func TestRefineNodeBulk_ValidatedParentRefused(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRoot(t, svc)
	refinePendingChild(t, svc, "1.1")
	if _, err := svc.AcceptNodesBulk([]types.NodeID{parseNodeID(t, "1.1"), parseNodeID(t, "1")}, "verifier", ""); err != nil {
		t.Fatalf("accept fixtures: %v", err)
	}

	_, err := svc.RefineNodeBulk(parseNodeID(t, "1"), "agent1", []ChildSpec{{Statement: "child", NodeType: schema.NodeTypeClaim, Inference: schema.InferenceModusPonens}})
	var pse *ParentStateError
	if !errors.As(err, &pse) || pse.Remedy != "run af request-refinement 1" {
		t.Fatalf("error = %v, want ParentStateError remedy 'run af request-refinement 1'", err)
	}
}

// TestCreateNode_ValidatedParentRefused wires the gate into direct creation.
func TestCreateNode_ValidatedParentRefused(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRoot(t, svc)
	refinePendingChild(t, svc, "1.1")
	if _, err := svc.AcceptNodesBulk([]types.NodeID{parseNodeID(t, "1.1"), parseNodeID(t, "1")}, "verifier", ""); err != nil {
		t.Fatalf("accept fixtures: %v", err)
	}

	err := svc.CreateNode(parseNodeID(t, "1.2"), schema.NodeTypeClaim, "child", schema.InferenceModusPonens)
	var pse *ParentStateError
	if !errors.As(err, &pse) || pse.Remedy != "run af request-refinement 1" {
		t.Fatalf("error = %v, want ParentStateError remedy 'run af request-refinement 1'", err)
	}
}

// TestCheckParentCreationGate locks the D4 creation rule: children are allowed
// only under pending, draft or needs_refinement parents, and the refusals name
// the exact remedy where one exists.
func TestCheckParentCreationGate(t *testing.T) {
	cases := []struct {
		state     schema.EpistemicState
		wantErr   bool
		wantRemed string
	}{
		{schema.EpistemicPending, false, ""},
		{schema.EpistemicDraft, false, ""},
		{schema.EpistemicNeedsRefinement, false, ""},
		{schema.EpistemicValidated, true, "run af request-refinement 1.9"},
		{schema.EpistemicAdmitted, true, "run af unadmit 1.9"},
		{schema.EpistemicRefuted, true, ""},
		{schema.EpistemicArchived, true, ""},
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			n, err := node.NewNode(parseNodeID(t, "1.9"), schema.NodeTypeClaim, "parent", schema.InferenceModusPonens)
			if err != nil {
				t.Fatalf("NewNode: %v", err)
			}
			n.EpistemicState = tc.state

			gateErr := checkParentCreationGate(n)
			if tc.wantErr && gateErr == nil {
				t.Fatalf("state %s: expected refusal, got nil", tc.state)
			}
			if !tc.wantErr && gateErr != nil {
				t.Fatalf("state %s: expected allowed, got %v", tc.state, gateErr)
			}
			if !tc.wantErr {
				return
			}
			var pse *ParentStateError
			if !errors.As(gateErr, &pse) {
				t.Fatalf("state %s: error is not *ParentStateError: %v", tc.state, gateErr)
			}
			if pse.Remedy != tc.wantRemed {
				t.Errorf("state %s: remedy = %q, want %q", tc.state, pse.Remedy, tc.wantRemed)
			}
			if tc.wantRemed != "" && !strings.Contains(gateErr.Error(), tc.wantRemed) {
				t.Errorf("state %s: message %q does not contain remedy %q", tc.state, gateErr.Error(), tc.wantRemed)
			}
		})
	}
}

// TestRefine_ValidatedParentNeedsRequestRefinement wires the gate into the
// refine path (vibefeld-gxa7): refining a validated parent is refused and names
// `af request-refinement`, and the parent stays validated.
func TestRefine_ValidatedParentNeedsRequestRefinement(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRoot(t, svc)
	refinePendingChild(t, svc, "1.1")

	// Validate the child then the parent so the parent is validated.
	if _, err := svc.AcceptNodesBulk([]types.NodeID{parseNodeID(t, "1.1"), parseNodeID(t, "1")}, "verifier", ""); err != nil {
		t.Fatalf("accept fixtures: %v", err)
	}

	err := svc.Refine(RefineSpec{
		ParentID:  parseNodeID(t, "1"),
		Owner:     "agent1",
		ChildID:   parseNodeID(t, "1.2"),
		NodeType:  schema.NodeTypeClaim,
		Statement: "new child",
		Inference: schema.InferenceModusPonens,
	})
	if err == nil {
		t.Fatal("expected refine under a validated parent to be refused")
	}
	var pse *ParentStateError
	if !errors.As(err, &pse) {
		t.Fatalf("error is not *ParentStateError: %v", err)
	}
	if pse.Remedy != "run af request-refinement 1" {
		t.Errorf("remedy = %q", pse.Remedy)
	}

	// The refused refine must not have written a child or changed the parent.
	st, lerr := svc.LoadState()
	if lerr != nil {
		t.Fatalf("LoadState: %v", lerr)
	}
	if st.GetNode(parseNodeID(t, "1.2")) != nil {
		t.Error("refused refine created node 1.2")
	}
	if got := st.GetNode(parseNodeID(t, "1")).EpistemicState; got != schema.EpistemicValidated {
		t.Errorf("parent state = %s, want still validated", got)
	}
}

// TestRecordProof_ValidatedParentNamesRemedy wires the gate into record-proof.
func TestRecordProof_ValidatedParentNamesRemedy(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRoot(t, svc)
	refinePendingChild(t, svc, "1.1")
	if _, err := svc.AcceptNodesBulk([]types.NodeID{parseNodeID(t, "1.1"), parseNodeID(t, "1")}, "verifier", ""); err != nil {
		t.Fatalf("accept fixtures: %v", err)
	}

	_, err := svc.RecordProof(RecordProofSpec{
		ParentID: parseNodeID(t, "1"),
		Owner:    "agent1",
		Children: []ChildSpec{{Statement: "child", Inference: schema.InferenceModusPonens}},
	})
	if err == nil {
		t.Fatal("expected record-proof under a validated parent to be refused")
	}
	var pse *ParentStateError
	if !errors.As(err, &pse) {
		t.Fatalf("error is not *ParentStateError: %v", err)
	}
}

// TestRefine_AdmittedParentNamesUnadmit covers the admitted remedy.
func TestRefine_AdmittedParentNamesUnadmit(t *testing.T) {
	svc, _ := setupTestProof(t)
	if err := svc.AdmitNode(parseNodeID(t, "1")); err != nil {
		t.Fatalf("AdmitNode: %v", err)
	}
	err := svc.Refine(RefineSpec{
		ParentID:  parseNodeID(t, "1"),
		Owner:     "agent1",
		ChildID:   parseNodeID(t, "1.1"),
		NodeType:  schema.NodeTypeClaim,
		Statement: "child",
		Inference: schema.InferenceModusPonens,
	})
	var pse *ParentStateError
	if !errors.As(err, &pse) || pse.Remedy != "run af unadmit 1" {
		t.Fatalf("error = %v, want ParentStateError remedy 'run af unadmit 1'", err)
	}
}
