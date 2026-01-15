//go:build integration

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// ===========================================================================
// Tests for --requires-validated flag (validation dependency tracking)
// ===========================================================================

func TestRefineCmd_WithRequiresValidated_ValidSingleDep(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// First, create a sibling node to depend on
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "First subgoal",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create first child: %v", err)
	}

	// Now refine with a validation dependency on the sibling
	cmd2 := newRefineTestCmd()
	output, err := executeCommand(cmd2, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Depends on 1.1 being validated",
		"--requires-validated", "1.1",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should show the new child (1.2)
	if !strings.Contains(output, "1.2") {
		t.Errorf("expected output to contain child ID '1.2', got: %q", output)
	}

	// Verify the validation dependency was recorded
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	childID, _ := types.Parse("1.2")
	child := st.GetNode(childID)
	if child == nil {
		t.Fatal("expected child node 1.2 to exist")
	}

	if len(child.ValidationDeps) != 1 {
		t.Errorf("expected 1 validation dependency, got %d", len(child.ValidationDeps))
	}

	if len(child.ValidationDeps) > 0 && child.ValidationDeps[0].String() != "1.1" {
		t.Errorf("expected validation dependency on '1.1', got %q", child.ValidationDeps[0].String())
	}
}

func TestRefineCmd_WithRequiresValidated_MultipleDeps(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Create multiple sibling nodes to depend on
	for i := 1; i <= 4; i++ {
		cmd := newRefineTestCmd()
		_, err := executeCommand(cmd, "refine", "1",
			"--owner", "test-agent",
			"--statement", "Subgoal",
			"--dir", tmpDir,
		)
		if err != nil {
			t.Fatalf("failed to create child %d: %v", i, err)
		}
	}

	// Now refine with validation dependencies on all siblings (like the issue example)
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Node 1.5 depends on 1.1-1.4 being validated",
		"--requires-validated", "1.1,1.2,1.3,1.4",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "1.5") {
		t.Errorf("expected output to contain child ID '1.5', got: %q", output)
	}

	// Verify all validation dependencies were recorded
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	childID, _ := types.Parse("1.5")
	child := st.GetNode(childID)
	if child == nil {
		t.Fatal("expected child node 1.5 to exist")
	}

	if len(child.ValidationDeps) != 4 {
		t.Errorf("expected 4 validation dependencies, got %d", len(child.ValidationDeps))
	}
}

func TestRefineCmd_WithRequiresValidated_NonExistentDep(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Try to refine with a validation dependency on a non-existent node
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Depends on non-existent 1.99",
		"--requires-validated", "1.99",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for non-existent validation dependency, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "1.99") {
		t.Errorf("expected error to mention the missing node '1.99', got: %q", errStr)
	}
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "does not exist") && !strings.Contains(errStr, "invalid") {
		t.Errorf("expected error about validation dependency not found, got: %q", errStr)
	}
}

func TestRefineCmd_WithRequiresValidated_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Try to refine with an invalid validation dependency ID format
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Some statement",
		"--requires-validated", "invalid.id.format",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for invalid validation dependency ID format, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "invalid") && !strings.Contains(errStr, "parse") {
		t.Errorf("expected error about invalid ID format, got: %q", errStr)
	}
}

func TestRefineCmd_WithRequiresValidated_EmptyString(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Empty requires-validated flag should be treated as no validation dependencies (not an error)
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Statement without validation dependencies",
		"--requires-validated", "",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error for empty requires-validated, got: %v", err)
	}

	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child ID '1.1', got: %q", output)
	}
}

func TestRefineCmd_WithRequiresValidated_MixedValidAndInvalid(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Create one valid dependency
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "First subgoal",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create first child: %v", err)
	}

	// Try to refine with one valid and one invalid validation dependency
	cmd2 := newRefineTestCmd()
	_, err = executeCommand(cmd2, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Depends on 1.1 and 1.99 being validated",
		"--requires-validated", "1.1,1.99",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for mixed valid/invalid validation dependencies, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "1.99") {
		t.Errorf("expected error to mention the missing node '1.99', got: %q", errStr)
	}
}

func TestRefineCmd_WithRequiresValidated_BothDependsAndRequiresValidated(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Create siblings
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "First subgoal",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create first child: %v", err)
	}

	cmd2 := newRefineTestCmd()
	_, err = executeCommand(cmd2, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Second subgoal",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create second child: %v", err)
	}

	// Refine with both --depends (reference dep) and --requires-validated (validation dep)
	cmd3 := newRefineTestCmd()
	output, err := executeCommand(cmd3, "refine", "1",
		"--owner", "test-agent",
		"--statement", "References 1.1, requires 1.2 validated",
		"--depends", "1.1",
		"--requires-validated", "1.2",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "1.3") {
		t.Errorf("expected output to contain child ID '1.3', got: %q", output)
	}

	// Verify both dependency types were recorded
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	childID, _ := types.Parse("1.3")
	child := st.GetNode(childID)
	if child == nil {
		t.Fatal("expected child node 1.3 to exist")
	}

	// Check regular dependencies
	if len(child.Dependencies) != 1 || child.Dependencies[0].String() != "1.1" {
		t.Errorf("expected regular dependency on '1.1', got %v", child.Dependencies)
	}

	// Check validation dependencies
	if len(child.ValidationDeps) != 1 || child.ValidationDeps[0].String() != "1.2" {
		t.Errorf("expected validation dependency on '1.2', got %v", child.ValidationDeps)
	}
}

func TestRefineCmd_WithRequiresValidated_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Create a dependency first
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "First subgoal",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create first child: %v", err)
	}

	// Refine with validation dependency and JSON output
	cmd2 := newRefineTestCmd()
	output, err := executeCommand(cmd2, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Requires 1.1 validated",
		"--requires-validated", "1.1",
		"--format", "json",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// JSON output should contain validation dependencies
	if !strings.Contains(output, "requires_validated") && !strings.Contains(output, "validation_deps") {
		t.Errorf("expected JSON output to show validation dependencies, got: %q", output)
	}
}

func TestRefineCmd_WithRequiresValidated_ShowsInHelp(t *testing.T) {
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "--help")

	if err != nil {
		t.Fatalf("expected no error for help, got: %v", err)
	}

	// Help should show the --requires-validated flag
	if !strings.Contains(output, "--requires-validated") {
		t.Errorf("expected help to show --requires-validated flag, got: %q", output)
	}
}

func TestRefineCmd_WithRequiresValidated_DependOnSelf(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Try to create a node that depends on itself being validated
	// This should error or be prevented since the node doesn't exist yet
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Self-referential validation dep",
		"--requires-validated", "1.1", // This is the ID it will get
		"--dir", tmpDir,
	)

	// This should fail since 1.1 doesn't exist yet
	if err == nil {
		t.Fatal("expected error for self-referential validation dependency, got nil")
	}
}

func TestRefineCmd_WithRequiresValidated_CrossBranch(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Create node 1.1
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Branch A step 1",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create 1.1: %v", err)
	}

	// Claim node 1.1 and create a child 1.1.1
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	node11, _ := types.Parse("1.1")
	err = svc.ClaimNode(node11, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim 1.1: %v", err)
	}

	cmd2 := newRefineTestCmd()
	_, err = executeCommand(cmd2, "refine", "1.1",
		"--owner", "test-agent",
		"--statement", "Branch A step 2",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create 1.1.1: %v", err)
	}

	// Create a node 1.2 that requires 1.1.1 (cross-branch) to be validated
	cmd3 := newRefineTestCmd()
	output, err := executeCommand(cmd3, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Branch B, requires 1.1.1 validated",
		"--requires-validated", "1.1.1",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error for cross-branch validation dep, got: %v", err)
	}

	if !strings.Contains(output, "1.2") {
		t.Errorf("expected output to contain child ID '1.2', got: %q", output)
	}

	// Verify the cross-branch validation dependency was recorded
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	childID, _ := types.Parse("1.2")
	child := st.GetNode(childID)
	if child == nil {
		t.Fatal("expected child node 1.2 to exist")
	}

	if len(child.ValidationDeps) != 1 || child.ValidationDeps[0].String() != "1.1.1" {
		t.Errorf("expected validation dependency on '1.1.1', got %v", child.ValidationDeps)
	}
}
