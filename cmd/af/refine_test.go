//go:build integration

package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// setupRefineTest creates a temporary proof directory with an initialized proof
// and a claimed node for testing the refine command.
// Note: service.Init() already creates node 1 with the conjecture, so we just
// need to claim it for the test agent.
func setupRefineTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-refine-test-*")
	if err != nil {
		t.Fatal(err)
	}

	// Initialize proof - this creates node 1 with "Test conjecture"
	err = service.Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Claim node 1 (already created by Init)
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
	// Note: service.Init() creates node 1, which is unclaimed by default
	tmpDir, err := os.MkdirTemp("", "af-refine-unclaimed-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	err = service.Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}
	// Note: node 1 is created by Init but NOT claimed

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

func TestRefineCmd_WhitespaceOnlyStatement(t *testing.T) {
	tests := []struct {
		name      string
		statement string
	}{
		{"spaces only", "   "},
		{"tabs only", "\t\t"},
		{"newlines only", "\n\n"},
		{"mixed whitespace", " \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, cleanup := setupRefineTest(t)
			defer cleanup()

			cmd := newRefineTestCmd()
			_, err := executeCommand(cmd, "refine", "1",
				"--owner", "test-agent",
				"--statement", tt.statement,
				"--dir", tmpDir,
			)

			if err == nil {
				t.Fatalf("expected error for whitespace-only statement %q, got nil", tt.statement)
			}

			errStr := err.Error()
			if !strings.Contains(errStr, "statement") && !strings.Contains(errStr, "empty") {
				t.Errorf("expected error about empty/whitespace statement, got: %q", errStr)
			}
		})
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
		"--justification", "invalid_justification",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for invalid justification type, got nil")
	}

	errStr := err.Error()
	// Should mention the invalid justification
	if !strings.Contains(errStr, "justification") && !strings.Contains(errStr, "invalid") {
		t.Errorf("expected error about invalid justification type, got: %q", errStr)
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

func TestRefineCmd_HelpShowsInferenceTypes(t *testing.T) {
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "--help")

	if err != nil {
		t.Fatalf("expected no error for help, got: %v", err)
	}

	// Should show valid inference types in the help output
	inferenceTypes := []string{
		"modus_ponens",
		"modus_tollens",
		"by_definition",
		"assumption",
		"local_assume",
		"local_discharge",
		"contradiction",
		"universal_instantiation",
		"existential_instantiation",
		"universal_generalization",
		"existential_generalization",
	}

	for _, infType := range inferenceTypes {
		if !strings.Contains(output, infType) {
			t.Errorf("expected help to show inference type %q, got: %q", infType, output)
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
				"--justification", inference,
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
		"-t", "claim",
		"-j", "assumption",
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

func TestRefineCmd_DefCitationValid(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Add a definition first
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.AddDefinition("group", "A group is a set with a binary operation")
	if err != nil {
		t.Fatalf("failed to add definition: %v", err)
	}

	// Now refine with a statement that cites the definition
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "By def:group, we have a binary operation",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error for valid def citation, got: %v", err)
	}

	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child ID, got: %q", output)
	}
}

func TestRefineCmd_DefCitationNotFound(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Do NOT add the definition - statement will cite a missing def

	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "By def:nonexistent-definition, we have...",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for missing definition citation, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "nonexistent-definition") {
		t.Errorf("expected error to mention the missing definition name, got: %q", errStr)
	}
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "citation") {
		t.Errorf("expected error about definition not found, got: %q", errStr)
	}
}

func TestRefineCmd_DefCitationMultipleValid(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Add multiple definitions
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.AddDefinition("group", "A group is a set with a binary operation")
	if err != nil {
		t.Fatalf("failed to add definition 'group': %v", err)
	}

	_, err = svc.AddDefinition("homomorphism", "A structure-preserving map")
	if err != nil {
		t.Fatalf("failed to add definition 'homomorphism': %v", err)
	}

	// Refine with a statement citing both definitions
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Using def:group and def:homomorphism, we can construct...",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error for valid multi-def citation, got: %v", err)
	}

	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child ID, got: %q", output)
	}
}

func TestRefineCmd_DefCitationOneInvalid(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Add only one of the cited definitions
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.AddDefinition("group", "A group is a set with a binary operation")
	if err != nil {
		t.Fatalf("failed to add definition 'group': %v", err)
	}
	// Note: NOT adding 'ring' definition

	cmd := newRefineTestCmd()
	_, err = executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Using def:group and def:ring, we show...",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for partially invalid def citation, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "ring") {
		t.Errorf("expected error to mention the missing definition 'ring', got: %q", errStr)
	}
}

func TestRefineCmd_DefCitationNoCitations(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Statement without any def: citations should be fine
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "This statement has no definition citations",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error for statement without citations, got: %v", err)
	}

	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child ID, got: %q", output)
	}
}

func TestRefineCmd_DefCitationHyphenatedName(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Add a definition with a hyphenated name
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.AddDefinition("Stirling-second-kind", "Stirling numbers of the second kind count...")
	if err != nil {
		t.Fatalf("failed to add definition: %v", err)
	}

	// Refine with a statement citing the hyphenated definition
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "By def:Stirling-second-kind, the number of partitions...",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error for valid hyphenated def citation, got: %v", err)
	}

	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child ID, got: %q", output)
	}
}

// ===========================================================================
// Tests for --depends flag (cross-reference validation)
// ===========================================================================

func TestRefineCmd_WithDependsFlag_ValidSingleDep(t *testing.T) {
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

	// Now refine with a dependency on the sibling
	cmd2 := newRefineTestCmd()
	output, err := executeCommand(cmd2, "refine", "1",
		"--owner", "test-agent",
		"--statement", "By step 1.1, we have...",
		"--depends", "1.1",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should show the new child (1.2)
	if !strings.Contains(output, "1.2") {
		t.Errorf("expected output to contain child ID '1.2', got: %q", output)
	}

	// Verify the dependency was recorded
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

	if len(child.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(child.Dependencies))
	}

	if len(child.Dependencies) > 0 && child.Dependencies[0].String() != "1.1" {
		t.Errorf("expected dependency on '1.1', got %q", child.Dependencies[0].String())
	}
}

func TestRefineCmd_WithDependsFlag_MultipleDeps(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Create two sibling nodes to depend on
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

	// Now refine with dependencies on both siblings
	cmd3 := newRefineTestCmd()
	output, err := executeCommand(cmd3, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Combining steps 1.1 and 1.2, we get...",
		"--depends", "1.1,1.2",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "1.3") {
		t.Errorf("expected output to contain child ID '1.3', got: %q", output)
	}

	// Verify both dependencies were recorded
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

	if len(child.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(child.Dependencies))
	}
}

func TestRefineCmd_WithDependsFlag_NonExistentDep(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Try to refine with a dependency on a non-existent node
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "By step 1.5, we have...",
		"--depends", "1.5",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for non-existent dependency, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "1.5") {
		t.Errorf("expected error to mention the missing node '1.5', got: %q", errStr)
	}
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "does not exist") && !strings.Contains(errStr, "invalid") {
		t.Errorf("expected error about dependency not found, got: %q", errStr)
	}
}

func TestRefineCmd_WithDependsFlag_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Try to refine with an invalid dependency ID format
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Some statement",
		"--depends", "invalid.id.format",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for invalid dependency ID format, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "invalid") && !strings.Contains(errStr, "parse") {
		t.Errorf("expected error about invalid ID format, got: %q", errStr)
	}
}

func TestRefineCmd_WithDependsFlag_EmptyString(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Empty depends flag should be treated as no dependencies (not an error)
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Statement without dependencies",
		"--depends", "",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error for empty depends, got: %v", err)
	}

	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child ID '1.1', got: %q", output)
	}
}

func TestRefineCmd_WithDependsFlag_MixedValidAndInvalid(t *testing.T) {
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

	// Try to refine with one valid and one invalid dependency
	cmd2 := newRefineTestCmd()
	_, err = executeCommand(cmd2, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Using steps 1.1 and 1.99...",
		"--depends", "1.1,1.99",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for mixed valid/invalid dependencies, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "1.99") {
		t.Errorf("expected error to mention the missing node '1.99', got: %q", errStr)
	}
}

func TestRefineCmd_WithDependsFlag_DependOnParent(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Should be able to depend on the parent node
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "From the conjecture (node 1), we derive...",
		"--depends", "1",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error when depending on parent, got: %v", err)
	}

	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to contain child ID '1.1', got: %q", output)
	}

	// Verify the dependency was recorded
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

	if len(child.Dependencies) != 1 || child.Dependencies[0].String() != "1" {
		t.Errorf("expected dependency on '1', got %v", child.Dependencies)
	}
}

func TestRefineCmd_WithDependsFlag_JSONOutput(t *testing.T) {
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

	// Refine with dependency and JSON output
	cmd2 := newRefineTestCmd()
	output, err := executeCommand(cmd2, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Using step 1.1...",
		"--depends", "1.1",
		"--format", "json",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// JSON output should contain dependencies
	if !strings.Contains(output, "depends") || !strings.Contains(output, "1.1") {
		t.Errorf("expected JSON output to show dependencies, got: %q", output)
	}
}

func TestRefineCmd_WithDependsFlag_ShowsInHelp(t *testing.T) {
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "--help")

	if err != nil {
		t.Fatalf("expected no error for help, got: %v", err)
	}

	// Help should show the --depends flag
	if !strings.Contains(output, "--depends") {
		t.Errorf("expected help to show --depends flag, got: %q", output)
	}
}

// ===========================================================================
// Issue 1 (vibefeld-yu7j): Tests for breadth-first Next steps suggestions
// ===========================================================================

func TestRefineCommand_NextStepsShowsBreadthFirst(t *testing.T) {
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

	// Output should show sibling option first with "recommended for breadth"
	if !strings.Contains(output, "--sibling") {
		t.Errorf("expected Next steps to show --sibling flag, got: %q", output)
	}
	if !strings.Contains(output, "recommended for breadth") {
		t.Errorf("expected Next steps to recommend breadth, got: %q", output)
	}
	// Sibling option should come before depth-first option
	siblingIdx := strings.Index(output, "--sibling")
	depthIdx := strings.Index(output, "depth-first")
	if siblingIdx < 0 || depthIdx < 0 {
		t.Errorf("expected both sibling and depth-first in output, got: %q", output)
	}
	if siblingIdx > depthIdx {
		t.Errorf("expected sibling option before depth-first, got: %q", output)
	}
}

func TestRefineCommand_NextStepsShowsParentID(t *testing.T) {
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

	// Output should show the child ID in the Next steps (1.1 in this case)
	// which is the node where siblings would be added or children
	if !strings.Contains(output, "af refine 1.1") {
		t.Errorf("expected Next steps to show 'af refine 1.1', got: %q", output)
	}
}

// ===========================================================================
// Issue 2 (vibefeld-80uy): Tests for depth warning
// ===========================================================================

func TestRefineCommand_WarnsAtDepth4(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create nodes at depth 2, 3 to get to depth 4
	// 1 (root, depth 1)
	// 1.1 (depth 2, already created by first refine below)
	cmd := newRefineTestCmd()
	_, err = executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Level 2",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create 1.1: %v", err)
	}

	// Claim 1.1 and create 1.1.1 (depth 3)
	id11, _ := types.Parse("1.1")
	err = svc.ClaimNode(id11, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim 1.1: %v", err)
	}

	cmd2 := newRefineTestCmd()
	_, err = executeCommand(cmd2, "refine", "1.1",
		"--owner", "test-agent",
		"--statement", "Level 3",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create 1.1.1: %v", err)
	}

	// Claim 1.1.1 and create 1.1.1.1 (depth 4) - should warn
	id111, _ := types.Parse("1.1.1")
	err = svc.ClaimNode(id111, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim 1.1.1: %v", err)
	}

	cmd3 := newRefineTestCmd()
	output, err := executeCommand(cmd3, "refine", "1.1.1",
		"--owner", "test-agent",
		"--statement", "Level 4 - should warn",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create 1.1.1.1: %v", err)
	}

	// Should contain warning about depth
	if !strings.Contains(output, "Warning") {
		t.Errorf("expected warning at depth 4, got: %q", output)
	}
	if !strings.Contains(output, "depth 4") {
		t.Errorf("expected warning to mention 'depth 4', got: %q", output)
	}
	if !strings.Contains(output, "Consider adding siblings") {
		t.Errorf("expected warning to suggest siblings, got: %q", output)
	}
}

func TestRefineCommand_NoWarningAtDepth3(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create 1.1 (depth 2)
	cmd := newRefineTestCmd()
	_, err = executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Level 2",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create 1.1: %v", err)
	}

	// Claim 1.1 and create 1.1.1 (depth 3) - should NOT warn
	id11, _ := types.Parse("1.1")
	err = svc.ClaimNode(id11, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim 1.1: %v", err)
	}

	cmd2 := newRefineTestCmd()
	output, err := executeCommand(cmd2, "refine", "1.1",
		"--owner", "test-agent",
		"--statement", "Level 3 - no warning",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create 1.1.1: %v", err)
	}

	// Should NOT contain warning
	if strings.Contains(output, "Warning") {
		t.Errorf("expected no warning at depth 3, got: %q", output)
	}
}

// ===========================================================================
// Issue 3 (vibefeld-1r6h): Tests for MaxDepth enforcement
// ===========================================================================

func TestRefineCommand_RejectsExceedingMaxDepth(t *testing.T) {
	// Create a proof with a very low MaxDepth (2) to make testing easy
	tmpDir, err := os.MkdirTemp("", "af-refine-maxdepth-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize proof
	err = service.Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}

	// Write a custom meta.json with MaxDepth=2
	metaContent := `{
		"title": "Test Proof",
		"conjecture": "Test conjecture",
		"lock_timeout": 300000000000,
		"max_depth": 2,
		"max_children": 10,
		"auto_correct_threshold": 0.8,
		"version": "1.0",
		"created": "2024-01-01T00:00:00Z"
	}`
	metaPath := tmpDir + "/meta.json"
	err = os.WriteFile(metaPath, []byte(metaContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Claim root
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	rootID, _ := types.Parse("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Create 1.1 (depth 2) - should succeed (at MaxDepth)
	cmd := newRefineTestCmd()
	_, err = executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Level 2",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("expected success at MaxDepth, got: %v", err)
	}

	// Claim 1.1 and try to create 1.1.1 (depth 3) - should FAIL
	id11, _ := types.Parse("1.1")
	err = svc.ClaimNode(id11, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim 1.1: %v", err)
	}

	cmd2 := newRefineTestCmd()
	_, err = executeCommand(cmd2, "refine", "1.1",
		"--owner", "test-agent",
		"--statement", "Level 3 - should fail",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error when exceeding MaxDepth, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "exceeds MaxDepth") && !strings.Contains(errStr, "depth 3") {
		t.Errorf("expected error about MaxDepth exceeded, got: %q", errStr)
	}
	if !strings.Contains(errStr, "add breadth instead") {
		t.Errorf("expected error to suggest adding breadth, got: %q", errStr)
	}
}

func TestRefineCommand_AllowsAtMaxDepth(t *testing.T) {
	// Create a proof with MaxDepth=3
	tmpDir, err := os.MkdirTemp("", "af-refine-atmaxdepth-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize proof
	err = service.Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}

	// Write a custom meta.json with MaxDepth=3
	metaContent := `{
		"title": "Test Proof",
		"conjecture": "Test conjecture",
		"lock_timeout": 300000000000,
		"max_depth": 3,
		"max_children": 10,
		"auto_correct_threshold": 0.8,
		"version": "1.0",
		"created": "2024-01-01T00:00:00Z"
	}`
	metaPath := tmpDir + "/meta.json"
	err = os.WriteFile(metaPath, []byte(metaContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Claim root
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	rootID, _ := types.Parse("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Create 1.1 (depth 2)
	cmd := newRefineTestCmd()
	_, err = executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "Level 2",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create 1.1: %v", err)
	}

	// Claim 1.1 and create 1.1.1 (depth 3) - should succeed (at MaxDepth)
	id11, _ := types.Parse("1.1")
	err = svc.ClaimNode(id11, "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("failed to claim 1.1: %v", err)
	}

	cmd2 := newRefineTestCmd()
	output, err := executeCommand(cmd2, "refine", "1.1",
		"--owner", "test-agent",
		"--statement", "Level 3 - should succeed",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected success at MaxDepth, got: %v", err)
	}

	// Should successfully create 1.1.1
	if !strings.Contains(output, "1.1.1") {
		t.Errorf("expected output to contain '1.1.1', got: %q", output)
	}
}

// ===========================================================================
// Issue 4 (vibefeld-cunz): Tests for --sibling flag
// ===========================================================================

func TestRefineCommand_SiblingFlag_CreatesAtSameLevel(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Create 1.1 first
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "First child",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create 1.1: %v", err)
	}

	// Now use --sibling on 1.1 to create 1.2 (sibling of 1.1)
	cmd2 := newRefineTestCmd()
	output, err := executeCommand(cmd2, "refine", "1.1",
		"--sibling",
		"--owner", "test-agent",
		"--statement", "Sibling of 1.1",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with --sibling, got: %v", err)
	}

	// Should create 1.2, not 1.1.1
	if !strings.Contains(output, "1.2") {
		t.Errorf("expected sibling at 1.2, got: %q", output)
	}
	if strings.Contains(output, "1.1.1") {
		t.Errorf("expected sibling (1.2), not child (1.1.1), got: %q", output)
	}

	// Verify in state
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	siblingID, _ := types.Parse("1.2")
	sibling := st.GetNode(siblingID)
	if sibling == nil {
		t.Error("expected sibling node 1.2 to exist")
	}
	if sibling != nil && sibling.Statement != "Sibling of 1.1" {
		t.Errorf("expected statement 'Sibling of 1.1', got %q", sibling.Statement)
	}
}

func TestRefineCommand_SiblingFlag_ErrorOnRoot(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Try to use --sibling on root node
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--sibling",
		"--owner", "test-agent",
		"--statement", "Cannot be sibling of root",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error when using --sibling on root, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "root") && !strings.Contains(errStr, "sibling") {
		t.Errorf("expected error about root having no sibling, got: %q", errStr)
	}
}

func TestRefineCommand_SiblingFlag_ShortForm(t *testing.T) {
	tmpDir, cleanup := setupRefineTest(t)
	defer cleanup()

	// Create 1.1 first
	cmd := newRefineTestCmd()
	_, err := executeCommand(cmd, "refine", "1",
		"--owner", "test-agent",
		"--statement", "First child",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("failed to create 1.1: %v", err)
	}

	// Use short form -b for --sibling
	cmd2 := newRefineTestCmd()
	output, err := executeCommand(cmd2, "refine", "1.1",
		"-b",
		"-o", "test-agent",
		"-s", "Sibling via short flag",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with -b flag, got: %v", err)
	}

	// Should create 1.2
	if !strings.Contains(output, "1.2") {
		t.Errorf("expected sibling at 1.2 with -b flag, got: %q", output)
	}
}

func TestRefineCommand_SiblingFlag_ShowsInHelp(t *testing.T) {
	cmd := newRefineTestCmd()
	output, err := executeCommand(cmd, "refine", "--help")

	if err != nil {
		t.Fatalf("expected no error for help, got: %v", err)
	}

	// Help should show the --sibling flag
	if !strings.Contains(output, "--sibling") {
		t.Errorf("expected help to show --sibling flag, got: %q", output)
	}
	if !strings.Contains(output, "-b") {
		t.Errorf("expected help to show -b short flag, got: %q", output)
	}
}
