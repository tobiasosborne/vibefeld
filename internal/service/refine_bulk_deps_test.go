package service

import (
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// B2 (rk review 2026-07-20 / rk-2zj): RefineNodeBulk must RECORD each child's
// per-child dependencies, resolving backward sibling references (#N) against
// the just-allocated child IDs. Previously ChildSpec had no Dependencies slot,
// so a prover's declared depends were silently erased.
func TestProofService_RefineNodeBulk_RecordsPerChildDependencies(t *testing.T) {
	svc, _ := setupTestProof(t)
	parentID := parseNodeID(t, "1")
	owner := "agent1"
	if err := svc.ClaimNode(parentID, owner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}

	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "First", Inference: schema.InferenceAssumption},
		{NodeType: schema.NodeTypeClaim, Statement: "Second", Inference: schema.InferenceModusPonens, Dependencies: []string{"#0"}},
		{NodeType: schema.NodeTypeClaim, Statement: "Third", Inference: schema.InferenceModusPonens, Dependencies: []string{"#0", "#1"}},
	}
	ids, err := svc.RefineNodeBulk(parentID, owner, children)
	if err != nil {
		t.Fatalf("RefineNodeBulk: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("want 3 children, got %d", len(ids))
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	dep12 := depStrings(st.GetNode(parseNodeID(t, "1.2")).Dependencies)
	if len(dep12) != 1 || dep12[0] != "1.1" {
		t.Errorf("node 1.2 Dependencies = %v, want [1.1]", dep12)
	}
	dep13 := depStrings(st.GetNode(parseNodeID(t, "1.3")).Dependencies)
	if len(dep13) != 2 || dep13[0] != "1.1" || dep13[1] != "1.2" {
		t.Errorf("node 1.3 Dependencies = %v, want [1.1 1.2]", dep13)
	}
	if d := st.GetNode(parseNodeID(t, "1.1")).Dependencies; len(d) != 0 {
		t.Errorf("node 1.1 Dependencies = %v, want empty", d)
	}
}

// B2: a per-child dependency naming an EXISTING real node id is recorded as-is.
func TestProofService_RefineNodeBulk_RecordsExistingNodeDependency(t *testing.T) {
	svc, _ := setupTestProof(t)
	parentID := parseNodeID(t, "1")
	if err := svc.ClaimNode(parentID, "agent1", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := svc.RefineNodeBulk(parentID, "agent1", []ChildSpec{{NodeType: schema.NodeTypeClaim, Statement: "one", Inference: schema.InferenceAssumption}}); err != nil {
		t.Fatalf("bulk1: %v", err)
	}
	if _, err := svc.RefineNodeBulk(parentID, "agent1", []ChildSpec{{NodeType: schema.NodeTypeClaim, Statement: "two", Inference: schema.InferenceModusPonens, Dependencies: []string{"1.1"}}}); err != nil {
		t.Fatalf("bulk2: %v", err)
	}
	st, _ := svc.LoadState()
	dep := depStrings(st.GetNode(parseNodeID(t, "1.2")).Dependencies)
	if len(dep) != 1 || dep[0] != "1.1" {
		t.Errorf("node 1.2 Dependencies = %v, want [1.1]", dep)
	}
}

// B2: invalid per-child deps are rejected, never silently dropped.
func TestProofService_RefineNodeBulk_RejectsInvalidDependencies(t *testing.T) {
	cases := map[string][]string{
		"forward sibling ref": {"#1"},
		"out of range index":  {"#9"},
		"unknown real node":   {"7.7"},
		"malformed index":     {"#x"},
	}
	for name, deps := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := setupTestProof(t)
			parentID := parseNodeID(t, "1")
			if err := svc.ClaimNode(parentID, "agent1", time.Minute); err != nil {
				t.Fatalf("claim: %v", err)
			}
			_, err := svc.RefineNodeBulk(parentID, "agent1", []ChildSpec{
				{NodeType: schema.NodeTypeClaim, Statement: "c0", Inference: schema.InferenceModusPonens, Dependencies: deps},
			})
			if err == nil {
				t.Fatalf("expected rejection for deps %v, got nil", deps)
			}
		})
	}
}

func depStrings(deps []types.NodeID) []string {
	out := make([]string, len(deps))
	for i, d := range deps {
		out[i] = d.String()
	}
	return out
}
