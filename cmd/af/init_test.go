//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// setupInitTest creates a temporary directory for testing init command.
// Returns the temp directory path and a cleanup function.
func setupInitTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "af-init-test-*")
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }
	return tmpDir, cleanup
}

// newTestInitCmd creates a fresh root command with the init subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestInitCmd() *cobra.Command {
	cmd := newTestRootCmd()

	initCmd := newInitCmd()
	cmd.AddCommand(initCmd)

	return cmd
}

func TestInitCmd_BasicInit(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"--conjecture", "All primes greater than 2 are odd",
		"--author", "Test Author",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify ledger directory was created
	ledgerDir := filepath.Join(tmpDir, "ledger")
	if _, err := os.Stat(ledgerDir); os.IsNotExist(err) {
		t.Errorf("expected ledger directory to exist at %s", ledgerDir)
	}

	// Verify ProofInitialized event in ledger using service
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	if !status.Initialized {
		t.Error("expected proof to be initialized")
	}
}

func TestInitCmd_WithCustomDirectory(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	// Create a subdirectory
	customDir := filepath.Join(tmpDir, "my-proof")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"--conjecture", "P = NP",
		"--author", "Test Author",
		"--dir", customDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify initialization in custom directory
	ledgerDir := filepath.Join(customDir, "ledger")
	if _, err := os.Stat(ledgerDir); os.IsNotExist(err) {
		t.Errorf("expected ledger directory to exist at %s", ledgerDir)
	}
}

func TestInitCmd_MissingConjectureFlag(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	_, err := executeCommand(cmd, "init",
		"--author", "Test Author",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for missing conjecture flag, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "conjecture") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error to mention 'conjecture' or 'required', got: %q", errStr)
	}
}

func TestInitCmd_MissingAuthorFlag(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	_, err := executeCommand(cmd, "init",
		"--conjecture", "Some conjecture",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for missing author flag, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "author") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error to mention 'author' or 'required', got: %q", errStr)
	}
}

func TestInitCmd_EmptyConjecture(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	_, err := executeCommand(cmd, "init",
		"--conjecture", "",
		"--author", "Test Author",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for empty conjecture, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "conjecture") && !strings.Contains(errStr, "empty") {
		t.Errorf("expected error to mention 'conjecture' or 'empty', got: %q", errStr)
	}
}

func TestInitCmd_EmptyAuthor(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	_, err := executeCommand(cmd, "init",
		"--conjecture", "Some conjecture",
		"--author", "",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for empty author, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "author") && !strings.Contains(errStr, "empty") {
		t.Errorf("expected error to mention 'author' or 'empty', got: %q", errStr)
	}
}

func TestInitCmd_DirectoryAlreadyInitialized(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()

	// First init should succeed
	_, err := executeCommand(cmd, "init",
		"--conjecture", "First conjecture",
		"--author", "Test Author",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	// Second init should fail
	cmd2 := newTestInitCmd()
	_, err = executeCommand(cmd2, "init",
		"--conjecture", "Second conjecture",
		"--author", "Another Author",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for already initialized directory, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "already") && !strings.Contains(errStr, "initialized") {
		t.Errorf("expected error to mention 'already' or 'initialized', got: %q", errStr)
	}
}

func TestInitCmd_DirectoryDoesNotExist(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	nonExistentDir := filepath.Join(tmpDir, "does-not-exist")

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"--conjecture", "Some conjecture",
		"--author", "Test Author",
		"--dir", nonExistentDir,
	)

	// The init command should either:
	// 1. Create the directory and succeed, OR
	// 2. Return an error about directory not existing
	// Based on service.Init and fs.InitProofDir, it should create the directory.
	if err != nil {
		// If error, check it's about directory creation
		errStr := err.Error()
		if !strings.Contains(errStr, "directory") && !strings.Contains(errStr, "exist") && !strings.Contains(errStr, "create") {
			t.Errorf("unexpected error: %v\nOutput: %s", err, output)
		}
	} else {
		// If success, verify directory was created
		if _, err := os.Stat(nonExistentDir); os.IsNotExist(err) {
			t.Error("expected directory to be created")
		}
	}
}

func TestInitCmd_InvalidDirectoryPath(t *testing.T) {
	cmd := newTestInitCmd()

	// Test with null byte in path
	_, err := executeCommand(cmd, "init",
		"--conjecture", "Some conjecture",
		"--author", "Test Author",
		"--dir", "/invalid\x00path",
	)

	if err == nil {
		t.Fatal("expected error for invalid directory path, got nil")
	}
}

func TestInitCmd_HelpOutput(t *testing.T) {
	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init", "--help")

	if err != nil {
		t.Fatalf("expected no error for help, got: %v", err)
	}

	// Check that help shows usage information
	expectations := []string{
		"conjecture",
		"author",
		"dir",
		"--help",
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

func TestInitCmd_ShortFlags(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"-c", "Test conjecture with short flag",
		"-a", "Test Author",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v\nOutput: %s", err, output)
	}

	// Verify initialization succeeded
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	if !status.Initialized {
		t.Error("expected proof to be initialized with short flags")
	}
}

func TestInitCmd_WhitespaceOnlyConjecture(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	_, err := executeCommand(cmd, "init",
		"--conjecture", "   ",
		"--author", "Test Author",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for whitespace-only conjecture, got nil")
	}
}

func TestInitCmd_WhitespaceOnlyAuthor(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	_, err := executeCommand(cmd, "init",
		"--conjecture", "Some conjecture",
		"--author", "   ",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for whitespace-only author, got nil")
	}
}

func TestInitCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	// Change to temp directory for this test
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"--conjecture", "Test conjecture",
		"--author", "Test Author",
		// Omit --dir to use default (current directory)
	)

	if err != nil {
		t.Fatalf("expected no error with default directory, got: %v\nOutput: %s", err, output)
	}

	// Verify initialization in current directory
	ledgerDir := filepath.Join(tmpDir, "ledger")
	if _, err := os.Stat(ledgerDir); os.IsNotExist(err) {
		t.Errorf("expected ledger directory to exist at %s", ledgerDir)
	}
}

func TestInitCmd_SuccessOutput(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"--conjecture", "Test conjecture",
		"--author", "Test Author",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check that success output contains useful information
	// The exact format depends on implementation, but should mention something
	// about initialization or success
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "init") && !strings.Contains(lowerOutput, "success") && !strings.Contains(lowerOutput, "created") && !strings.Contains(lowerOutput, "proof") {
		t.Errorf("expected success output to mention init/success/created/proof, got: %q", output)
	}
}

func TestInitCmd_LongConjecture(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	// Test with a reasonably long conjecture
	longConjecture := strings.Repeat("This is a long mathematical statement. ", 50)

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"--conjecture", longConjecture,
		"--author", "Test Author",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with long conjecture, got: %v\nOutput: %s", err, output)
	}

	// Verify initialization succeeded
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	if !status.Initialized {
		t.Error("expected proof to be initialized with long conjecture")
	}
}

func TestInitCmd_SpecialCharactersInConjecture(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	// Test with special characters commonly used in mathematics
	specialConjecture := "For all x > 0, if x^2 + y^2 = z^2, then x + y > z (Pythagorean triples)"

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"--conjecture", specialConjecture,
		"--author", "Test Author",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with special characters, got: %v\nOutput: %s", err, output)
	}

	// Verify initialization succeeded
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	if !status.Initialized {
		t.Error("expected proof to be initialized with special characters")
	}
}

func TestInitCmd_ListTemplates(t *testing.T) {
	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init", "--list-templates")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check that all expected templates are listed
	expectedTemplates := []string{
		"contradiction",
		"induction",
		"cases",
	}

	for _, template := range expectedTemplates {
		if !strings.Contains(output, template) {
			t.Errorf("expected template %q in output, got: %q", template, output)
		}
	}
}

func TestInitCmd_ContradictionTemplate(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"--conjecture", "All primes greater than 2 are odd",
		"--author", "Test Author",
		"--template", "contradiction",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify initialization succeeded and nodes were created
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodes := st.AllNodes()
	// Root (1) + 2 children (1.1, 1.2) = 3 nodes
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes for contradiction template, got %d", len(nodes))
	}

	// Verify node 1.1 exists with local_assume type
	node11 := st.GetNode(mustParseNodeID(t, "1.1"))
	if node11 == nil {
		t.Fatal("expected node 1.1 to exist")
	}
	if node11.Type != schema.NodeTypeLocalAssume {
		t.Errorf("expected node 1.1 to be local_assume, got %s", node11.Type)
	}

	// Verify node 1.2 exists with claim type
	node12 := st.GetNode(mustParseNodeID(t, "1.2"))
	if node12 == nil {
		t.Fatal("expected node 1.2 to exist")
	}
	if node12.Type != schema.NodeTypeClaim {
		t.Errorf("expected node 1.2 to be claim, got %s", node12.Type)
	}
}

func TestInitCmd_InductionTemplate(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"--conjecture", "P(n) holds for all n",
		"--author", "Test Author",
		"--template", "induction",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify initialization succeeded and nodes were created
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodes := st.AllNodes()
	// Root (1) + 2 children (1.1 base case, 1.2 inductive step) = 3 nodes
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes for induction template, got %d", len(nodes))
	}

	// Verify node 1.1 exists (base case)
	node11 := st.GetNode(mustParseNodeID(t, "1.1"))
	if node11 == nil {
		t.Fatal("expected node 1.1 to exist (base case)")
	}
	if !strings.Contains(strings.ToLower(node11.Statement), "base") {
		t.Errorf("expected node 1.1 statement to contain 'base', got %q", node11.Statement)
	}

	// Verify node 1.2 exists (inductive step)
	node12 := st.GetNode(mustParseNodeID(t, "1.2"))
	if node12 == nil {
		t.Fatal("expected node 1.2 to exist (inductive step)")
	}
	if !strings.Contains(strings.ToLower(node12.Statement), "inductive") {
		t.Errorf("expected node 1.2 statement to contain 'inductive', got %q", node12.Statement)
	}
}

func TestInitCmd_CasesTemplate(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"--conjecture", "For all x, P(x)",
		"--author", "Test Author",
		"--template", "cases",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify initialization succeeded and nodes were created
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodes := st.AllNodes()
	// Root (1) + 2 children (1.1 case 1, 1.2 case 2) = 3 nodes minimum
	if len(nodes) < 3 {
		t.Errorf("expected at least 3 nodes for cases template, got %d", len(nodes))
	}

	// Verify at least one case node exists with case type
	node11 := st.GetNode(mustParseNodeID(t, "1.1"))
	if node11 == nil {
		t.Fatal("expected node 1.1 to exist")
	}
	if node11.Type != schema.NodeTypeCase {
		t.Errorf("expected node 1.1 to be case, got %s", node11.Type)
	}
}

func TestInitCmd_InvalidTemplate(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	_, err := executeCommand(cmd, "init",
		"--conjecture", "Some conjecture",
		"--author", "Test Author",
		"--template", "nonexistent",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for invalid template, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "template") && !strings.Contains(errStr, "unknown") {
		t.Errorf("expected error to mention template issue, got: %q", errStr)
	}
}

func TestInitCmd_TemplateWithShortFlag(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"-c", "Some conjecture",
		"-a", "Test Author",
		"-t", "contradiction",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with short template flag, got: %v\nOutput: %s", err, output)
	}

	// Verify template was applied
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Should have 3 nodes (root + 2 children from contradiction template)
	nodes := st.AllNodes()
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes with template, got %d", len(nodes))
	}
}

func TestInitCmd_TemplateOutputShowsStructure(t *testing.T) {
	tmpDir, cleanup := setupInitTest(t)
	defer cleanup()

	cmd := newTestInitCmd()
	output, err := executeCommand(cmd, "init",
		"--conjecture", "Some conjecture",
		"--author", "Test Author",
		"--template", "induction",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Output should mention the template that was applied
	if !strings.Contains(strings.ToLower(output), "induction") {
		t.Errorf("expected output to mention 'induction' template, got: %q", output)
	}
}

// Note: mustParseNodeID is defined in accept_test.go
