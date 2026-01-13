//go:build integration

package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newRefineMultiTestCmd creates a test command hierarchy with the refine command.
// This ensures test isolation - each test gets its own command instance.
func newRefineMultiTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	refineCmd := newRefineCmd()
	cmd.AddCommand(refineCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// setupRefineMultiTest creates a temporary proof directory with an initialized proof
// and a claimed node for testing the multi-child refine command.
func setupRefineMultiTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-refine-multi-test-*")
	if err != nil {
		t.Fatal(err)
	}

	// Initialize proof
	err = service.Init(tmpDir, "Test conjecture for multi-child", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create and claim a node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	rootID, err := types.Parse("1")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Note: Init already creates node "1" as the root node, so we just need to claim it.
	// Don't call CreateNode here - it would fail with "node already exists".

	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return tmpDir, cleanup
}

// TestRefineMultiCmd_TwoChildren tests creating two children at once via JSON.
func TestRefineMultiCmd_TwoChildren(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	// Test creating two children at once via --children JSON flag
	childrenJSON := `[{"statement":"First subgoal"},{"statement":"Second subgoal"}]`
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should create 1.1 and 1.2
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain 1.1, got: %q", output)
	}
	if !strings.Contains(output, "1.2") {
		t.Errorf("expected output to contain 1.2, got: %q", output)
	}

	// Verify nodes were actually created
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	child1, _ := types.Parse("1.1")
	child2, _ := types.Parse("1.2")
	if st.GetNode(child1) == nil {
		t.Error("expected child node 1.1 to exist in state")
	}
	if st.GetNode(child2) == nil {
		t.Error("expected child node 1.2 to exist in state")
	}
}

// TestRefineMultiCmd_ThreeChildren tests creating three children at once via JSON.
func TestRefineMultiCmd_ThreeChildren(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	childrenJSON := `[{"statement":"Case 1"},{"statement":"Case 2"},{"statement":"Case 3"}]`
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should create 1.1, 1.2, and 1.3
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain 1.1, got: %q", output)
	}
	if !strings.Contains(output, "1.2") {
		t.Errorf("expected output to contain 1.2, got: %q", output)
	}
	if !strings.Contains(output, "1.3") {
		t.Errorf("expected output to contain 1.3, got: %q", output)
	}

	// Verify nodes were actually created
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 3; i++ {
		childID, _ := types.Parse("1." + string(rune('0'+i)))
		if st.GetNode(childID) == nil {
			t.Errorf("expected child node 1.%d to exist in state", i)
		}
	}
}

// TestRefineMultiCmd_JSONInput tests reading children from stdin JSON.
func TestRefineMultiCmd_JSONInput(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	// Test with --children flag accepting JSON array of child specifications
	// Note: using "local_assume" as the inference type since "by_cases" is not a valid inference type in the schema
	childrenJSON := `[{"statement":"Subgoal A","type":"claim","inference":"assumption"},{"statement":"Subgoal B","type":"case","inference":"local_assume"}]`

	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should create both children
	if !strings.Contains(output, "1.1") || !strings.Contains(output, "1.2") {
		t.Errorf("expected output to contain 1.1 and 1.2, got: %q", output)
	}

	// Verify nodes with correct types were created
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	child1ID, _ := types.Parse("1.1")
	child1 := st.GetNode(child1ID)
	if child1 == nil {
		t.Fatal("expected child node 1.1 to exist")
	}
	if child1.Type != schema.NodeTypeClaim {
		t.Errorf("expected child 1.1 type to be claim, got: %s", child1.Type)
	}

	child2ID, _ := types.Parse("1.2")
	child2 := st.GetNode(child2ID)
	if child2 == nil {
		t.Fatal("expected child node 1.2 to exist")
	}
	if child2.Type != schema.NodeTypeCase {
		t.Errorf("expected child 1.2 type to be case, got: %s", child2.Type)
	}
}

// TestRefineMultiCmd_MixedTypes tests creating children with different node types.
func TestRefineMultiCmd_MixedTypes(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	// Mix of different node types
	childrenJSON := `[{"statement":"Assumption step","type":"local_assume"},{"statement":"Main claim","type":"claim"},{"statement":"QED step","type":"qed"}]`

	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should create all three children
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain 1.1, got: %q", output)
	}
	if !strings.Contains(output, "1.2") {
		t.Errorf("expected output to contain 1.2, got: %q", output)
	}
	if !strings.Contains(output, "1.3") {
		t.Errorf("expected output to contain 1.3, got: %q", output)
	}

	// Verify correct types
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	expectedTypes := []struct {
		id       string
		nodeType schema.NodeType
	}{
		{"1.1", schema.NodeTypeLocalAssume},
		{"1.2", schema.NodeTypeClaim},
		{"1.3", schema.NodeTypeQED},
	}

	for _, exp := range expectedTypes {
		nodeID, _ := types.Parse(exp.id)
		node := st.GetNode(nodeID)
		if node == nil {
			t.Errorf("expected node %s to exist", exp.id)
			continue
		}
		if node.Type != exp.nodeType {
			t.Errorf("expected node %s type to be %s, got: %s", exp.id, exp.nodeType, node.Type)
		}
	}
}

// TestRefineMultiCmd_EmptyChildren tests that empty children array returns an error.
func TestRefineMultiCmd_EmptyChildren(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	// Empty children array should fail
	childrenJSON := `[]`

	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for empty children array, got nil")
	}

	errStr := err.Error()
	// Should indicate that children array is empty or at least one child is required
	if !strings.Contains(errStr, "empty") && !strings.Contains(errStr, "required") && !strings.Contains(errStr, "at least") {
		t.Errorf("expected error about empty children, got: %q", errStr)
	}
}

// TestRefineMultiCmd_InvalidJSON tests that malformed JSON returns an error.
func TestRefineMultiCmd_InvalidJSON(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	// Invalid JSON
	invalidJSON := `[{"statement": "Missing closing bracket"`

	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", invalidJSON,
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	errStr := err.Error()
	// Should indicate JSON parsing error
	if !strings.Contains(errStr, "JSON") && !strings.Contains(errStr, "json") && !strings.Contains(errStr, "parse") && !strings.Contains(errStr, "invalid") {
		t.Errorf("expected error about invalid JSON, got: %q", errStr)
	}
}

// TestRefineMultiCmd_ChildAlreadyExists tests error when child ID would conflict.
func TestRefineMultiCmd_ChildAlreadyExists(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	// First, create a child node using single refine
	cmd1 := newRefineMultiTestCmd()
	_, err := executeCommand(cmd1, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Existing child",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create first child: %v", err)
	}

	// Now try to create multiple children - this should handle the conflict
	// The implementation should either:
	// 1. Skip existing IDs and continue with next available
	// 2. Return an error indicating conflict
	cmd2 := newRefineMultiTestCmd()
	childrenJSON := `[{"statement":"Would be 1.1"},{"statement":"Would be 1.2"}]`

	output, err := executeCommand(cmd2, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	// The behavior depends on implementation:
	// Either error out or create 1.2 and 1.3 (skipping 1.1)
	// For this test, we just check that it handles the situation gracefully
	if err != nil {
		// Error is acceptable - should mention conflict or existing
		errStr := err.Error()
		if !strings.Contains(errStr, "exist") && !strings.Contains(errStr, "conflict") {
			t.Errorf("expected error about existing child, got: %q", errStr)
		}
	} else {
		// If no error, should have created 1.2 and 1.3 (skipping 1.1)
		if !strings.Contains(output, "1.2") || !strings.Contains(output, "1.3") {
			t.Errorf("expected output to contain 1.2 and 1.3 when skipping existing 1.1, got: %q", output)
		}
	}
}

// TestRefineMultiCmd_JSONOutput tests that JSON format output works correctly.
func TestRefineMultiCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	childrenJSON := `[{"statement":"Child A"},{"statement":"Child B"}]`

	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--format", "json",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should be valid JSON
	if !strings.Contains(output, "{") || !strings.Contains(output, "}") {
		t.Errorf("expected JSON output, got: %q", output)
	}

	// Should contain success indicator
	if !strings.Contains(output, "success") && !strings.Contains(output, "true") {
		t.Errorf("expected JSON to indicate success, got: %q", output)
	}

	// Should list the created children
	if !strings.Contains(output, "1.1") || !strings.Contains(output, "1.2") {
		t.Errorf("expected JSON to contain child IDs 1.1 and 1.2, got: %q", output)
	}

	// Verify the output is valid JSON by attempting to parse it
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got parse error: %v", err)
	}
}

// TestRefineMultiCmd_MissingStatement tests that children without statements fail.
func TestRefineMultiCmd_MissingStatement(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	// Children with missing statement should fail
	childrenJSON := `[{"type":"claim"},{"statement":"Valid child"}]`

	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for child without statement, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "statement") && !strings.Contains(errStr, "required") && !strings.Contains(errStr, "empty") {
		t.Errorf("expected error about missing statement, got: %q", errStr)
	}
}

// TestRefineMultiCmd_InvalidChildType tests that invalid child type returns error.
func TestRefineMultiCmd_InvalidChildType(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	// Invalid node type in children
	childrenJSON := `[{"statement":"Child 1","type":"invalid_type"}]`

	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for invalid child type, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "type") && !strings.Contains(errStr, "invalid") {
		t.Errorf("expected error about invalid type, got: %q", errStr)
	}
}

// TestRefineMultiCmd_ConflictWithSingleStatement tests that --children and --statement are mutually exclusive.
func TestRefineMultiCmd_ConflictWithSingleStatement(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	childrenJSON := `[{"statement":"Child 1"}]`

	// Using both --statement and --children should be an error
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Single statement",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error when using both --statement and --children, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "exclusive") && !strings.Contains(errStr, "both") && !strings.Contains(errStr, "conflict") {
		t.Errorf("expected error about conflicting flags, got: %q", errStr)
	}
}

// TestRefineMultiCmd_ParentNotClaimed tests that refining unclaimed parent fails.
func TestRefineMultiCmd_ParentNotClaimed(t *testing.T) {
	// Create a proof with an unclaimed node
	tmpDir, err := os.MkdirTemp("", "af-refine-multi-unclaimed-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Init creates the root node "1" as an unclaimed node - exactly what we need for this test
	err = service.Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}

	// Note: node "1" is NOT claimed - Init creates it but doesn't claim it

	cmd := newRefineMultiTestCmd()
	childrenJSON := `[{"statement":"Child 1"},{"statement":"Child 2"}]`

	_, err = executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for unclaimed parent, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not claimed") && !strings.Contains(errStr, "unclaimed") && !strings.Contains(errStr, "claim") {
		t.Errorf("expected error about node not claimed, got: %q", errStr)
	}
}

// TestRefineMultiCmd_WrongOwner tests that wrong owner returns error.
func TestRefineMultiCmd_WrongOwner(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	childrenJSON := `[{"statement":"Child 1"},{"statement":"Child 2"}]`

	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "wrong-agent", // Different from "test-agent"
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for wrong owner, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "owner") && !strings.Contains(errStr, "match") {
		t.Errorf("expected error about owner mismatch, got: %q", errStr)
	}
}

// TestRefineMultiCmd_DefaultInference tests that inference defaults to assumption.
func TestRefineMultiCmd_DefaultInference(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	// Children without inference should default to assumption
	childrenJSON := `[{"statement":"Child without inference specified"}]`

	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain 1.1, got: %q", output)
	}

	// Verify the node has default inference type
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	childID, _ := types.Parse("1.1")
	child := st.GetNode(childID)
	if child == nil {
		t.Fatal("expected child node 1.1 to exist")
	}

	// Default inference should be assumption
	if child.Inference != schema.InferenceAssumption {
		t.Errorf("expected default inference to be assumption, got: %s", child.Inference)
	}
}

// TestRefineMultiCmd_DefaultType tests that type defaults to claim.
func TestRefineMultiCmd_DefaultType(t *testing.T) {
	tmpDir, cleanup := setupRefineMultiTest(t)
	defer cleanup()

	cmd := newRefineMultiTestCmd()
	// Children without type should default to claim
	childrenJSON := `[{"statement":"Child without type specified"}]`

	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--children", childrenJSON,
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain 1.1, got: %q", output)
	}

	// Verify the node has default type
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	childID, _ := types.Parse("1.1")
	child := st.GetNode(childID)
	if child == nil {
		t.Fatal("expected child node 1.1 to exist")
	}

	// Default type should be claim
	if child.Type != schema.NodeTypeClaim {
		t.Errorf("expected default type to be claim, got: %s", child.Type)
	}
}
