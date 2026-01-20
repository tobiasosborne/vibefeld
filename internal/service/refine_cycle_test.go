package service

import (
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

func TestRefine_DetectsCircularDependency(t *testing.T) {
	// Setup service
	svc, _ := setupTestProof(t)
	owner := "agent1"

	// Helper to refine
	refine := func(parent, child string, deps []string) {
		pID := parseNodeID(t, parent)
		cID := parseNodeID(t, child)

		var dIDs []types.NodeID
		for _, d := range deps {
			dIDs = append(dIDs, parseNodeID(t, d))
		}

		err := svc.Refine(RefineSpec{
			ParentID:     pID,
			ChildID:      cID,
			Owner:        owner,
			NodeType:     schema.NodeTypeClaim,
			Statement:    "step",
			Inference:    schema.InferenceModusPonens,
			Dependencies: dIDs,
		})
		if err != nil {
			t.Fatalf("Refine %s -> %s failed: %v", parent, child, err)
		}
	}

	// Build structure:
	// 1 (Root)
	//   1.1 (Child1)
	//   1.2 (Child2)
	//     1.2.1 (Grandchild2) -> depends on 1.1

	// Claim Root 1
	if err := svc.ClaimNode(parseNodeID(t, "1"), owner, time.Hour); err != nil {
		t.Fatalf("Claim root failed: %v", err)
	}

	// Add 1.1
	refine("1", "1.1", nil)

	// Add 1.2
	refine("1", "1.2", nil)

	// Claim 1.2 to add 1.2.1
	if err := svc.ClaimNode(parseNodeID(t, "1.2"), owner, time.Hour); err != nil {
		t.Fatalf("Claim 1.2 failed: %v", err)
	}
	// Add 1.2.1 depending on 1.1
	refine("1.2", "1.2.1", []string{"1.1"})

	// Now attempt to add 1.1.2 to 1.1, depending on 1.2.1.
	// Cycle: 1.1 -> 1.1.2 (child) -> 1.2.1 (dep) -> 1.1 (dep)
	
	// Must claim 1.1
	if err := svc.ClaimNode(parseNodeID(t, "1.1"), owner, time.Hour); err != nil {
		t.Fatalf("Claim 1.1 failed: %v", err)
	}

	pID := parseNodeID(t, "1.1")
	cID := parseNodeID(t, "1.1.2")
	dIDs := []types.NodeID{parseNodeID(t, "1.2.1")}

	err := svc.Refine(RefineSpec{
		ParentID:     pID,
		ChildID:      cID,
		Owner:        owner,
		NodeType:     schema.NodeTypeClaim,
		Statement:    "cycle step",
		Inference:    schema.InferenceModusPonens,
		Dependencies: dIDs,
	})

	if err == nil {
		t.Error("Expected circular dependency error, got nil")
	} else if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("Expected circular dependency error, got: %v", err)
	}
}