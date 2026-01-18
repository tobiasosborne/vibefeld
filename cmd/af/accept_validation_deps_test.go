//go:build integration

// Package main contains tests for validation dependency checking in accept command.
package main

import (
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// ===========================================================================
// Tests for validation dependency checking in accept command
// ===========================================================================

// TestAcceptCmd_BlockedByUnvalidatedDep tests that accepting a node fails
// when its validation dependencies are not yet validated.
func TestAcceptCmd_BlockedByUnvalidatedDep(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Claim node 1 and create two children
	rootID, _ := service.ParseNodeID("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim node 1: %v", err)
	}

	// Create child 1.1 (will be the validation dependency)
	childID1, _ := service.ParseNodeID("1.1")
	err = svc.RefineNode(rootID, "test-agent", childID1, schema.NodeTypeClaim, "First step", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node 1.1: %v", err)
	}

	// Create child 1.2 with validation dependency on 1.1
	childID2, _ := service.ParseNodeID("1.2")
	err = svc.RefineNodeWithAllDeps(rootID, "test-agent", childID2, schema.NodeTypeClaim, "Second step, needs 1.1 validated", schema.InferenceAssumption, nil, []service.NodeID{childID1})
	if err != nil {
		t.Fatalf("failed to create node 1.2: %v", err)
	}

	// Try to accept 1.2 without validating 1.1 first - should fail
	output, err := executeAcceptCommand(t, "1.2", "-d", tmpDir)

	if err == nil {
		t.Fatalf("expected error when accepting node with unvalidated deps, got nil. Output: %s", output)
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "validation") && !strings.Contains(errStr, "dependencies") && !strings.Contains(errStr, "1.1") {
		t.Errorf("expected error about unvalidated dependencies, got: %q", errStr)
	}
}

// TestAcceptCmd_SucceedsWhenDepsValidated tests that accepting a node succeeds
// when all validation dependencies are validated.
func TestAcceptCmd_SucceedsWhenDepsValidated(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Claim node 1 and create two children
	rootID, _ := service.ParseNodeID("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim node 1: %v", err)
	}

	// Create child 1.1 (will be the validation dependency)
	childID1, _ := service.ParseNodeID("1.1")
	err = svc.RefineNode(rootID, "test-agent", childID1, schema.NodeTypeClaim, "First step", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node 1.1: %v", err)
	}

	// Create child 1.2 with validation dependency on 1.1
	childID2, _ := service.ParseNodeID("1.2")
	err = svc.RefineNodeWithAllDeps(rootID, "test-agent", childID2, schema.NodeTypeClaim, "Second step, needs 1.1 validated", schema.InferenceAssumption, nil, []service.NodeID{childID1})
	if err != nil {
		t.Fatalf("failed to create node 1.2: %v", err)
	}

	// First, validate 1.1
	err = svc.AcceptNode(childID1)
	if err != nil {
		t.Fatalf("failed to accept node 1.1: %v", err)
	}

	// Now try to accept 1.2 - should succeed
	output, err := executeAcceptCommand(t, "1.2", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error when deps are validated, got: %v. Output: %s", err, output)
	}

	// Verify node was accepted
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(childID2)
	if n == nil {
		t.Fatal("node 1.2 not found")
	}

	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("expected node 1.2 to be validated, got: %s", n.EpistemicState)
	}
}

// TestAcceptCmd_BlockedByPartialValidation tests that accepting fails when
// only some validation dependencies are validated.
func TestAcceptCmd_BlockedByPartialValidation(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Claim node 1 and create four children like in the issue example
	rootID, _ := service.ParseNodeID("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim node 1: %v", err)
	}

	// Create children 1.1, 1.2, 1.3, 1.4
	childIDs := make([]service.NodeID, 4)
	idStrings := []string{"1.1", "1.2", "1.3", "1.4"}
	for i, idStr := range idStrings {
		childID, _ := service.ParseNodeID(idStr)
		childIDs[i] = childID
		err = svc.RefineNode(rootID, "test-agent", childID, schema.NodeTypeClaim, "Step", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", idStr, err)
		}
	}

	// Create 1.5 with validation deps on all four
	childID5, _ := service.ParseNodeID("1.5")
	err = svc.RefineNodeWithAllDeps(rootID, "test-agent", childID5, schema.NodeTypeClaim, "Final step", schema.InferenceAssumption, nil, childIDs)
	if err != nil {
		t.Fatalf("failed to create node 1.5: %v", err)
	}

	// Validate only some dependencies (1.1 and 1.2)
	err = svc.AcceptNode(childIDs[0])
	if err != nil {
		t.Fatalf("failed to accept 1.1: %v", err)
	}
	err = svc.AcceptNode(childIDs[1])
	if err != nil {
		t.Fatalf("failed to accept 1.2: %v", err)
	}

	// Try to accept 1.5 - should fail because 1.3 and 1.4 are not validated
	output, err := executeAcceptCommand(t, "1.5", "-d", tmpDir)

	if err == nil {
		t.Fatalf("expected error when only some deps validated, got nil. Output: %s", output)
	}

	errStr := err.Error()
	// Should mention at least one of the unvalidated deps
	if !strings.Contains(errStr, "1.3") && !strings.Contains(errStr, "1.4") {
		t.Errorf("expected error to mention unvalidated deps (1.3 or 1.4), got: %q", errStr)
	}
}

// TestAcceptCmd_NoValidationDepsAcceptsFine tests that nodes without
// validation dependencies can be accepted normally.
func TestAcceptCmd_NoValidationDepsAcceptsFine(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	// Accept root node (has no validation deps)
	output, err := executeAcceptCommand(t, "1", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v. Output: %s", err, output)
	}

	// Verify node was accepted
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	rootID, _ := service.ParseNodeID("1")
	n := st.GetNode(rootID)

	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("expected node to be validated, got: %s", n.EpistemicState)
	}
}

// TestAcceptCmd_BulkBlockedByUnvalidatedDeps tests that bulk accept
// correctly handles validation dependencies.
func TestAcceptCmd_BulkBlockedByUnvalidatedDeps(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Claim node 1 and create children
	rootID, _ := service.ParseNodeID("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim node 1: %v", err)
	}

	// Create 1.1 (no validation deps)
	childID1, _ := service.ParseNodeID("1.1")
	err = svc.RefineNode(rootID, "test-agent", childID1, schema.NodeTypeClaim, "First step", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node 1.1: %v", err)
	}

	// Create 1.2 with validation dep on 1.1
	childID2, _ := service.ParseNodeID("1.2")
	err = svc.RefineNodeWithAllDeps(rootID, "test-agent", childID2, schema.NodeTypeClaim, "Second step", schema.InferenceAssumption, nil, []service.NodeID{childID1})
	if err != nil {
		t.Fatalf("failed to create node 1.2: %v", err)
	}

	// Try to bulk accept both 1.1 and 1.2 together
	// 1.2 depends on 1.1, so this should fail because we're trying to accept
	// 1.2 while 1.1 is still pending (even though 1.1 is in the same batch)
	output, err := executeAcceptCommand(t, "1.1", "1.2", "-d", tmpDir)

	// This is a policy decision: bulk accept could either:
	// 1. Process in order and succeed (1.1 validated, then 1.2 can be validated)
	// 2. Validate deps before all nodes and fail
	// We'll document whatever behavior we get for now
	t.Logf("Bulk accept result: output=%s, err=%v", output, err)
}

// TestAcceptCmd_CrossBranchValidationDep tests that cross-branch validation
// dependencies are properly enforced.
func TestAcceptCmd_CrossBranchValidationDep(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	rootID, _ := service.ParseNodeID("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim node 1: %v", err)
	}

	// Create 1.1 (branch A)
	childID1, _ := service.ParseNodeID("1.1")
	err = svc.RefineNode(rootID, "test-agent", childID1, schema.NodeTypeClaim, "Branch A start", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create 1.1: %v", err)
	}

	// Claim 1.1 and create 1.1.1
	err = svc.ClaimNode(childID1, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim 1.1: %v", err)
	}

	childID11, _ := service.ParseNodeID("1.1.1")
	err = svc.RefineNode(childID1, "test-agent", childID11, schema.NodeTypeClaim, "Branch A step 2", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create 1.1.1: %v", err)
	}

	// Create 1.2 (branch B) with cross-branch validation dep on 1.1.1
	childID2, _ := service.ParseNodeID("1.2")
	err = svc.RefineNodeWithAllDeps(rootID, "test-agent", childID2, schema.NodeTypeClaim, "Branch B, needs 1.1.1", schema.InferenceAssumption, nil, []service.NodeID{childID11})
	if err != nil {
		t.Fatalf("failed to create 1.2: %v", err)
	}

	// Try to accept 1.2 without validating 1.1.1 - should fail
	_, err = executeAcceptCommand(t, "1.2", "-d", tmpDir)

	if err == nil {
		t.Fatal("expected error for cross-branch unvalidated dep, got nil")
	}

	// Now validate 1.1.1
	err = svc.AcceptNode(childID11)
	if err != nil {
		t.Fatalf("failed to accept 1.1.1: %v", err)
	}

	// Now accepting 1.2 should succeed
	output, err := executeAcceptCommand(t, "1.2", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected success after validating cross-branch dep, got: %v. Output: %s", err, output)
	}
}

// TestAcceptCmd_ShowsBlockingDepsInError tests that the error message
// lists which validation dependencies are blocking acceptance.
func TestAcceptCmd_ShowsBlockingDepsInError(t *testing.T) {
	tmpDir, cleanup := setupAcceptTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	rootID, _ := service.ParseNodeID("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim node 1: %v", err)
	}

	// Create 1.1 and 1.2
	childID1, _ := service.ParseNodeID("1.1")
	childID2, _ := service.ParseNodeID("1.2")
	err = svc.RefineNode(rootID, "test-agent", childID1, schema.NodeTypeClaim, "Step 1", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create 1.1: %v", err)
	}
	err = svc.RefineNode(rootID, "test-agent", childID2, schema.NodeTypeClaim, "Step 2", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create 1.2: %v", err)
	}

	// Create 1.3 with validation deps on both 1.1 and 1.2
	childID3, _ := service.ParseNodeID("1.3")
	err = svc.RefineNodeWithAllDeps(rootID, "test-agent", childID3, schema.NodeTypeClaim, "Combined step", schema.InferenceAssumption, nil, []service.NodeID{childID1, childID2})
	if err != nil {
		t.Fatalf("failed to create 1.3: %v", err)
	}

	// Try to accept 1.3 - error should list blocking deps
	_, err = executeAcceptCommand(t, "1.3", "-d", tmpDir)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errStr := err.Error()
	// Should mention the blocking dependencies
	if !strings.Contains(errStr, "1.1") || !strings.Contains(errStr, "1.2") {
		t.Errorf("expected error to list blocking deps 1.1 and 1.2, got: %q", errStr)
	}
}
