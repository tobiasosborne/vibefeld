package service

import (
	"os"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/types"
)

func TestAmendNode_Basic(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "af-amend-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize proof
	err = Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}

	// Create service
	svc, err := NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Claim the root node
	rootID, _ := types.Parse("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Create a child node
	childID, _ := types.Parse("1.1")
	err = svc.RefineNode(rootID, "test-agent", childID, "claim", "Original statement", "assumption")
	if err != nil {
		t.Fatal(err)
	}

	// Amend the child node
	err = svc.AmendNode(childID, "test-agent", "Corrected statement")
	if err != nil {
		t.Fatalf("AmendNode failed: %v", err)
	}

	// Verify the statement was updated
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	child := st.GetNode(childID)
	if child == nil {
		t.Fatal("child node not found")
	}

	if child.Statement != "Corrected statement" {
		t.Errorf("expected statement to be 'Corrected statement', got %q", child.Statement)
	}

	// Verify amendment history
	history := st.GetAmendmentHistory(childID)
	if len(history) != 1 {
		t.Errorf("expected 1 amendment in history, got %d", len(history))
	}

	if len(history) > 0 {
		if history[0].PreviousStatement != "Original statement" {
			t.Errorf("expected previous statement to be 'Original statement', got %q", history[0].PreviousStatement)
		}
		if history[0].NewStatement != "Corrected statement" {
			t.Errorf("expected new statement to be 'Corrected statement', got %q", history[0].NewStatement)
		}
		if history[0].Owner != "test-agent" {
			t.Errorf("expected owner to be 'test-agent', got %q", history[0].Owner)
		}
	}
}

func TestAmendNode_MultipleAmendments(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "af-amend-multi-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize proof
	err = Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}

	// Create service
	svc, err := NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Claim and create child
	rootID, _ := types.Parse("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	childID, _ := types.Parse("1.1")
	err = svc.RefineNode(rootID, "test-agent", childID, "claim", "Original statement", "assumption")
	if err != nil {
		t.Fatal(err)
	}

	// Amend multiple times
	amendments := []string{"First correction", "Second correction", "Third correction"}
	for _, stmt := range amendments {
		err = svc.AmendNode(childID, "test-agent", stmt)
		if err != nil {
			t.Fatalf("AmendNode failed: %v", err)
		}
	}

	// Verify the final statement
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	child := st.GetNode(childID)
	if child.Statement != "Third correction" {
		t.Errorf("expected statement to be 'Third correction', got %q", child.Statement)
	}

	// Verify amendment history has all amendments
	history := st.GetAmendmentHistory(childID)
	if len(history) != 3 {
		t.Errorf("expected 3 amendments in history, got %d", len(history))
	}
}

func TestAmendNode_ValidatedNodeFails(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "af-amend-validated-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize proof
	err = Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}

	// Create service
	svc, err := NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Claim and create child
	rootID, _ := types.Parse("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	childID, _ := types.Parse("1.1")
	err = svc.RefineNode(rootID, "test-agent", childID, "claim", "Original statement", "assumption")
	if err != nil {
		t.Fatal(err)
	}

	// Accept the child node
	err = svc.AcceptNode(childID)
	if err != nil {
		t.Fatal(err)
	}

	// Try to amend - should fail
	err = svc.AmendNode(childID, "test-agent", "Should fail")
	if err == nil {
		t.Fatal("expected error when amending validated node, got nil")
	}
}

func TestAmendNode_WrongOwnerFails(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "af-amend-owner-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize proof
	err = Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}

	// Create service
	svc, err := NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Claim root and create child
	rootID, _ := types.Parse("1")
	err = svc.ClaimNode(rootID, "agent1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	childID, _ := types.Parse("1.1")
	err = svc.RefineNode(rootID, "agent1", childID, "claim", "Original statement", "assumption")
	if err != nil {
		t.Fatal(err)
	}

	// Claim child with different agent
	err = svc.ClaimNode(childID, "agent2", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Try to amend as agent1 - should fail because agent2 has claimed it
	err = svc.AmendNode(childID, "agent1", "Should fail")
	if err == nil {
		t.Fatal("expected error when amending node claimed by another agent, got nil")
	}
}

func TestAmendNode_EmptyStatementFails(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "af-amend-empty-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize proof
	err = Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}

	// Create service
	svc, err := NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	rootID, _ := types.Parse("1")

	// Try to amend with empty statement
	err = svc.AmendNode(rootID, "test-agent", "")
	if err == nil {
		t.Fatal("expected error for empty statement, got nil")
	}

	// Try to amend with whitespace-only statement
	err = svc.AmendNode(rootID, "test-agent", "   ")
	if err == nil {
		t.Fatal("expected error for whitespace statement, got nil")
	}
}

func TestAmendNode_NodeNotFoundFails(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "af-amend-notfound-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize proof
	err = Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}

	// Create service
	svc, err := NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Try to amend non-existent node
	nonExistent, _ := types.Parse("1.99")
	err = svc.AmendNode(nonExistent, "test-agent", "Should fail")
	if err == nil {
		t.Fatal("expected error for non-existent node, got nil")
	}
}
