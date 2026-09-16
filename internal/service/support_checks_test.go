package service

import (
	"errors"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/support"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// TestRefineNodeBulk_RejectsAncestorResultUse verifies the D1 cycle check runs
// on the bulk creation path from the child's own position.
func TestRefineNodeBulk_RejectsAncestorResultUse(t *testing.T) {
	svc, _ := setupTestProof(t)
	parentID := parseNodeID(t, "1")
	if err := svc.ClaimNode(parentID, "agent1", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}

	_, err := svc.RefineNodeBulk(parentID, "agent1", []ChildSpec{{
		NodeType:     schema.NodeTypeClaim,
		Statement:    "cites its ancestor",
		Inference:    schema.InferenceModusPonens,
		Dependencies: []string{"1"},
	}})
	if err == nil {
		t.Fatal("expected a cycle error, got nil")
	}
	if !errors.Is(err, support.ErrCycle) {
		t.Fatalf("error is not a support cycle error: %v", err)
	}
}

// TestRefine_RejectsForeignScopeLeak verifies the D1 scope check runs on the
// single-child path: a node outside a discharged local_assume block may not
// cite a node inside it.
func TestRefine_RejectsForeignScopeLeak(t *testing.T) {
	svc, _ := setupTestProof(t)
	root := parseNodeID(t, "1")
	if err := svc.ClaimNode(root, "agent1", time.Hour); err != nil {
		t.Fatalf("claim root: %v", err)
	}

	// 1.1 local_assume
	if err := svc.Refine(RefineSpec{ParentID: root, Owner: "agent1", ChildID: parseNodeID(t, "1.1"), NodeType: schema.NodeTypeLocalAssume, Statement: "Suppose P", Inference: schema.InferenceLocalAssume}); err != nil {
		t.Fatalf("refine 1.1: %v", err)
	}
	assume := parseNodeID(t, "1.1")
	if err := svc.ClaimNode(assume, "agent1", time.Hour); err != nil {
		t.Fatalf("claim 1.1: %v", err)
	}
	// 1.1.1 claim inside the scope
	if err := svc.Refine(RefineSpec{ParentID: assume, Owner: "agent1", ChildID: parseNodeID(t, "1.1.1"), NodeType: schema.NodeTypeClaim, Statement: "Then Q", Inference: schema.InferenceModusPonens}); err != nil {
		t.Fatalf("refine 1.1.1: %v", err)
	}
	// 1.1.2 local_discharge closes the scope
	if err := svc.Refine(RefineSpec{ParentID: assume, Owner: "agent1", ChildID: parseNodeID(t, "1.1.2"), NodeType: schema.NodeTypeLocalDischarge, Statement: "Discharge P", Inference: schema.InferenceLocalDischarge}); err != nil {
		t.Fatalf("refine 1.1.2: %v", err)
	}

	// 1.2 (outside the scope) citing 1.1.1 (inside) is a leak.
	err := svc.Refine(RefineSpec{
		ParentID:     root,
		Owner:        "agent1",
		ChildID:      parseNodeID(t, "1.2"),
		NodeType:     schema.NodeTypeClaim,
		Statement:    "Uses Q from the closed block",
		Inference:    schema.InferenceModusPonens,
		Dependencies: []types.NodeID{parseNodeID(t, "1.1.1")},
	})
	if err == nil {
		t.Fatal("expected a scope leak error, got nil")
	}
	if !errors.Is(err, support.ErrScopeLeak) {
		t.Fatalf("error is not a scope leak: %v", err)
	}
}

// TestRecordProof_RejectsAncestorResultUse verifies the D1 cycle check runs on
// the record-proof path (it shares buildChildEvents).
func TestRecordProof_RejectsAncestorResultUse(t *testing.T) {
	svc, _ := setupTestProof(t)
	challengeRoot(t, svc)

	_, err := svc.RecordProof(RecordProofSpec{
		ParentID: parseNodeID(t, "1"),
		Owner:    "prover-x",
		Children: []ChildSpec{{
			NodeType:     schema.NodeTypeClaim,
			Statement:    "cites its ancestor",
			Inference:    schema.InferenceModusPonens,
			Dependencies: []string{"1"},
		}},
	})
	if err == nil {
		t.Fatal("expected a cycle error, got nil")
	}
	if !errors.Is(err, support.ErrCycle) {
		t.Fatalf("error is not a support cycle error: %v", err)
	}
}
