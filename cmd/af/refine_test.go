//go:build integration

package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// setupRefineTest creates a temporary proof directory with an initialized proof
// and a claimed node for testing the refine command.
func setupRefineTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-refine-test-*")
	if err != nil {
		t.Fatal(err)
	}

	// Initialize proof
	err = service.Init(tmpDir, "Test conjecture", "test-author")
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

	err = svc.CreateNode(rootID, schema.NodeTypeClaim, "Test goal", schema.InferenceAssumption)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return tmpDir, cleanup
}

// newRefineTestCmd creates a test command hierarchy with the refine command.
// This ensures test isolation - each test gets its own command instance.
func newRefineTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	refineCmd := newRefineCmd()
	cmd.AddCommand(refineCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

func TestRefineCmd_ValidRefinement(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "First subgoal",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should show the new child node
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child ID '1.1', got: %q", output)
	}
}

func TestRefineCmd_MultipleChildren(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()

	// Add first child
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "First subgoal",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("first refinement error: %v", err)
	}

	// Add second child
	cmd2 := newRefineTestCmd()
	output, err := executeCommand(cmd2, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Second subgoal",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("second refinement error: %v", err)
	}

	// Output should show the second child node
	if !strings.Contains(output, "1.2") {
		t.Errorf("expected output to contain child ID '1.2', got: %q", output)
	}
}

func TestRefineCmd_MissingParentID(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine",
		"--owner", "test-agent",
		"--statement", "Some statement",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for missing parent ID, got nil")
	}

	// Should error about missing argument
	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") && !strings.Contains(errStr, "argument") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

func TestRefineCmd_ParentNotFound(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1.99",
		"--owner", "test-agent",
		"--statement", "Some statement",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for parent not found, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "does not exist") {
		t.Errorf("expected error about parent not found, got: %q", errStr)
	}
}

func TestRefineCmd_ParentNotClaimed(t *testing.T) {
	// Create a proof with an unclaimed node
	tmpDir, err := os.MkdirTemp("", "af-refine-unclaimed-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	err = service.Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	rootID, _ := types.Parse("1")
	err = svc.CreateNode(rootID, schema.NodeTypeClaim, "Test goal", schema.InferenceAssumption)
	if err != nil {
		t.Fatal(err)
	}
	// Note: node is NOT claimed

	cmd := newRefineTestCmd()
	_, err = executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Some statement",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for unclaimed parent, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not claimed") && !strings.Contains(errStr, "unclaimed") {
		t.Errorf("expected error about node not claimed, got: %q", errStr)
	}
}

func TestRefineCmd_WrongOwner(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "wrong-agent", // Different from "test-agent"
		"--statement", "Some statement",
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

func TestRefineCmd_MissingStatement(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--dir", tmpDir,
		// Note: --statement is missing
	)

	if err == nil {
		t.Fatal("expected error for missing statement, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "statement") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing statement, got: %q", errStr)
	}
}

func TestRefineCmd_EmptyStatement(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for empty statement, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "statement") && !strings.Contains(errStr, "empty") {
		t.Errorf("expected error about empty statement, got: %q", errStr)
	}
}

func TestRefineCmd_InvalidNodeType(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Some statement",
		"--type", "invalid_type",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for invalid node type, got nil")
	}

	errStr := err.Error()
	// Should suggest valid node types
	if !strings.Contains(errStr, "invalid") && !strings.Contains(errStr, "type") {
		t.Errorf("expected error about invalid node type, got: %q", errStr)
	}

	// Should suggest valid alternatives
	validTypes := []string{"claim", "local_assume", "case", "qed"}
	foundSuggestion := false
	for _, validType := range validTypes {
		if strings.Contains(errStr, validType) {
			foundSuggestion = true
			break
		}
	}
	if !foundSuggestion {
		t.Errorf("expected error to suggest valid node types, got: %q", errStr)
	}
}

func TestRefineCmd_InvalidInferenceType(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Some statement",
		"--inference", "invalid_inference",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for invalid inference type, got nil")
	}

	errStr := err.Error()
	// Should mention the invalid inference
	if !strings.Contains(errStr, "inference") && !strings.Contains(errStr, "invalid") {
		t.Errorf("expected error about invalid inference type, got: %q", errStr)
	}
}

func TestRefineCmd_ChildIDAutoGenerated(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "First subgoal",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Child ID should be auto-generated as 1.1
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected child ID '1.1' in output, got: %q", output)
	}

	// Verify node was actually created
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
		t.Error("expected child node 1.1 to exist in state")
	}
}

func TestRefineCmd_OutputShowsNewChild(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "My subgoal statement",
		"--type", "claim",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should include the child ID
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child ID, got: %q", output)
	}

	// Output should include the statement (or at least acknowledge creation)
	if !strings.Contains(output, "subgoal") && !strings.Contains(output, "created") && !strings.Contains(output, "refined") {
		t.Errorf("expected output to confirm creation, got: %q", output)
	}
}

func TestRefineCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "First subgoal",
		"--format", "json",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// JSON output should be valid JSON-like structure
	if !strings.Contains(output, "{") || !strings.Contains(output, "}") {
		t.Errorf("expected JSON output, got: %q", output)
	}

	// Should contain the child ID field
	if !strings.Contains(output, "1.1") && !strings.Contains(output, "id") {
		t.Errorf("expected JSON to contain child ID, got: %q", output)
	}
}

func TestRefineCmd_Help(t *testing.T) {
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "--help")

	if err != nil {
		t.Fatalf("expected no error for help, got: %v", err)
	}

	// Should show usage information
	expectations := []string{
		"refine",
		"--owner",
		"--statement",
		"parent",
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help to contain %q, got: %q", exp, output)
		}
	}
}

func TestRefineCmd_InvalidParentID(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "invalid.id.format",
		"--owner", "test-agent",
		"--statement", "Some statement",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for invalid parent ID, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "invalid") && !strings.Contains(errStr, "ID") && !strings.Contains(errStr, "parse") {
		t.Errorf("expected error about invalid ID format, got: %q", errStr)
	}
}

func TestRefineCmd_WithAllNodeTypes(t *testing.T) {
	// Test that all valid node types can be used
	nodeTypes := []string{"claim", "local_assume", "local_discharge", "case", "qed"}

	for _, nodeType := range nodeTypes {
		t.Run("type_"+nodeType, func(t *testing.T) {
			tmpDir, cleanup := setupRefineTest(t)
			defer cleanup()

			cmd := newRefineTestCmd()
			output, err := executeCommand(cmd, "refine", "1",
				"--owner", "test-agent",
				"--statement", "Statement for "+nodeType,
				"--type", nodeType,
				"--dir", tmpDir,
			)

			if err != nil {
				t.Fatalf("expected no error for type %q, got: %v", nodeType, err)
			}

			if !strings.Contains(output, "1.1") {
				t.Errorf("expected output to contain child ID for type %q, got: %q", nodeType, output)
			}
		})
	}
}

func TestRefineCmd_WithValidInferenceTypes(t *testing.T) {
	// Test a few representative inference types
	inferenceTypes := []string{
		"modus_ponens",
		"assumption",
		"by_definition",
		"local_assume",
	}

	for _, inference := range inferenceTypes {
		t.Run("inference_"+inference, func(t *testing.T) {
			tmpDir, cleanup := setupRefineTest(t)
			defer cleanup()

			cmd := newRefineTestCmd()
			output, err := executeCommand(cmd, "refine", "1",
				"--owner", "test-agent",
				"--statement", "Statement with "+inference,
				"--inference", inference,
				"--dir", tmpDir,
			)

			if err != nil {
				t.Fatalf("expected no error for inference %q, got: %v", inference, err)
			}

			if !strings.Contains(output, "1.1") {
				t.Errorf("expected output to contain child ID for inference %q, got: %q", inference, output)
			}
		})
	}
}

func TestRefineCmd_DeepRefinement(t *testing.T) {
	// Test refining a grandchild (1.1.1)
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// First, refine root to create 1.1
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "First child",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("first refinement error: %v", err)
	}

	// Claim node 1.1
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	childID, _ := types.Parse("1.1")
	err = svc.ClaimNode(childID, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim child node: %v", err)
	}

	// Now refine 1.1 to create 1.1.1
	cmd2 := newRefineTestCmd()
	output, err := executeCommand(cmd2, "refine", "1.1",
		"--owner", "test-agent",
		"--statement", "Grandchild",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("deep refinement error: %v", err)
	}

	if !strings.Contains(output, "1.1.1") {
		t.Errorf("expected output to contain grandchild ID '1.1.1', got: %q", output)
	}
}

func TestRefineCmd_MissingOwnerFlag(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--statement", "Some statement",
		"--dir", tmpDir,
		// Note: --owner is missing
	)

	if err == nil {
		t.Fatal("expected error for missing owner, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "owner") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing owner, got: %q", errStr)
	}
}

func TestRefineCmd_ShortFlags(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"-o", "test-agent",
		"-s", "Short flag statement",
		"-T", "claim",
		"-i", "assumption",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child ID with short flags, got: %q", output)
	}
}

func TestRefineCmd_ProofNotInitialized(t *testing.T) {
	// Create empty temp directory (no proof initialized)
	tmpDir, err := os.MkdirTemp("", "af-refine-noinit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newRefineTestCmd()
	_, err = executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Some statement",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "initialized") && !strings.Contains(errStr, "init") {
		t.Errorf("expected error about uninitialized proof, got: %q", errStr)
	}
}

func TestRefineCmd_FuzzyCommandMatching(t *testing.T) {
	// Test that fuzzy matching works for the refine command
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refin") // Missing 'e'

	if err == nil {
		t.Fatal("expected error for misspelled command, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "refine") {
		t.Errorf("expected suggestion for 'refine', got: %q", errStr)
	}
}
