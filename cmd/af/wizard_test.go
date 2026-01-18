//go:build integration

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestWizardCmd creates a fresh root command with the wizard subcommand for testing.
func newTestWizardCmd() *cobra.Command {
	cmd := newTestRootCmd()

	wizardCmd := newWizardCmd()
	cmd.AddCommand(wizardCmd)

	return cmd
}

// setupWizardTest creates a temp directory for testing.
func setupWizardTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-wizard-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return tmpDir, cleanup
}

// setupWizardTestWithProof creates a temp directory with an initialized proof.
func setupWizardTestWithProof(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-wizard-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")

	// Initialize proof directory structure
	if err := fs.InitProofDir(proofDir); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Initialize proof with conjecture
	err = service.Init(proofDir, "Test conjecture: P implies Q", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// executeWizardWithInput executes a wizard command with simulated stdin input.
func executeWizardWithInput(root *cobra.Command, input string, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	// Create pipe for stdin simulation
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	// Write input to the pipe
	go func() {
		w.WriteString(input)
		w.Close()
	}()

	err := root.Execute()

	// Restore stdin
	os.Stdin = oldStdin

	return buf.String(), err
}

// =============================================================================
// Wizard Command Tests
// =============================================================================

func TestWizardCmd_Help(t *testing.T) {
	cmd := newTestWizardCmd()
	output, err := executeCommand(cmd, "wizard", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include subcommands and description
	expectations := []string{
		"wizard",
		"new-proof",
		"respond-challenge",
		"review",
		"guide",
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

func TestWizardCmd_NoSubcommand(t *testing.T) {
	cmd := newTestWizardCmd()
	output, err := executeCommand(cmd, "wizard")

	// Should show help or available subcommands when no subcommand provided
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should mention available wizards
	if !strings.Contains(output, "new-proof") || !strings.Contains(output, "respond-challenge") || !strings.Contains(output, "review") {
		t.Errorf("expected output to list available wizards, got: %q", output)
	}
}

// =============================================================================
// New Proof Wizard Tests
// =============================================================================

func TestWizardNewProof_Help(t *testing.T) {
	cmd := newTestWizardCmd()
	output, err := executeCommand(cmd, "wizard", "new-proof", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should describe the wizard
	expectations := []string{
		"new-proof",
		"conjecture",
		"author",
		"directory",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

func TestWizardNewProof_NonInteractive(t *testing.T) {
	tmpDir, cleanup := setupWizardTest(t)
	defer cleanup()

	proofDir := filepath.Join(tmpDir, "my-proof")
	if err := os.MkdirAll(proofDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := newTestWizardCmd()
	output, err := executeCommand(cmd, "wizard", "new-proof",
		"--conjecture", "All primes > 2 are odd",
		"--author", "Test Author",
		"--dir", proofDir,
		"--no-confirm",
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Should show success and next steps
	if !strings.Contains(strings.ToLower(output), "success") && !strings.Contains(strings.ToLower(output), "created") {
		t.Errorf("expected output to indicate success, got: %q", output)
	}

	// Verify proof was actually initialized
	svc, err := service.NewProofService(proofDir)
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

func TestWizardNewProof_PreviewOnly(t *testing.T) {
	tmpDir, cleanup := setupWizardTest(t)
	defer cleanup()

	proofDir := filepath.Join(tmpDir, "my-proof")
	if err := os.MkdirAll(proofDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := newTestWizardCmd()
	output, err := executeCommand(cmd, "wizard", "new-proof",
		"--conjecture", "Test conjecture",
		"--author", "Test Author",
		"--dir", proofDir,
		"--preview",
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show preview but NOT execute
	if !strings.Contains(strings.ToLower(output), "preview") {
		t.Errorf("expected output to show preview, got: %q", output)
	}

	// Verify proof was NOT initialized
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		// Expected - proof not initialized
		return
	}

	status, err := svc.Status()
	if err != nil {
		return // Expected - no ledger
	}

	if status.Initialized {
		t.Error("expected proof to NOT be initialized in preview mode")
	}
}

func TestWizardNewProof_MissingConjecture(t *testing.T) {
	tmpDir, cleanup := setupWizardTest(t)
	defer cleanup()

	cmd := newTestWizardCmd()
	_, err := executeCommand(cmd, "wizard", "new-proof",
		"--author", "Test Author",
		"--dir", tmpDir,
		"--no-confirm",
	)

	// Should error because conjecture is required in non-interactive mode
	if err == nil {
		t.Fatal("expected error for missing conjecture, got nil")
	}
}

func TestWizardNewProof_WithTemplate(t *testing.T) {
	tmpDir, cleanup := setupWizardTest(t)
	defer cleanup()

	proofDir := filepath.Join(tmpDir, "my-proof")
	if err := os.MkdirAll(proofDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := newTestWizardCmd()
	output, err := executeCommand(cmd, "wizard", "new-proof",
		"--conjecture", "P(n) for all n",
		"--author", "Test Author",
		"--dir", proofDir,
		"--template", "induction",
		"--no-confirm",
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify proof was initialized with template
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Template should create child nodes
	nodes := st.AllNodes()
	if len(nodes) < 2 {
		t.Errorf("expected template to create child nodes, got %d nodes", len(nodes))
	}
}

// =============================================================================
// Respond Challenge Wizard Tests
// =============================================================================

func TestWizardRespondChallenge_Help(t *testing.T) {
	cmd := newTestWizardCmd()
	output, err := executeCommand(cmd, "wizard", "respond-challenge", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should describe the wizard
	expectations := []string{
		"respond",
		"challenge",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

func TestWizardRespondChallenge_NoChallenges(t *testing.T) {
	proofDir, cleanup := setupWizardTestWithProof(t)
	defer cleanup()

	cmd := newTestWizardCmd()
	output, err := executeCommand(cmd, "wizard", "respond-challenge", "--dir", proofDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should indicate no challenges to respond to
	if !strings.Contains(strings.ToLower(output), "no") && !strings.Contains(strings.ToLower(output), "challenge") {
		t.Errorf("expected output to indicate no challenges, got: %q", output)
	}
}

func TestWizardRespondChallenge_NoProof(t *testing.T) {
	tmpDir, cleanup := setupWizardTest(t)
	defer cleanup()

	cmd := newTestWizardCmd()
	_, err := executeCommand(cmd, "wizard", "respond-challenge", "--dir", tmpDir)

	// Should error because no proof is initialized
	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// =============================================================================
// Review Wizard Tests
// =============================================================================

func TestWizardReview_Help(t *testing.T) {
	cmd := newTestWizardCmd()
	output, err := executeCommand(cmd, "wizard", "review", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should describe the wizard
	expectations := []string{
		"review",
		"verifier",
		"pending",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

func TestWizardReview_ListsPendingNodes(t *testing.T) {
	proofDir, cleanup := setupWizardTestWithProof(t)
	defer cleanup()

	cmd := newTestWizardCmd()
	output, err := executeCommand(cmd, "wizard", "review", "--dir", proofDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should list the root node as pending
	if !strings.Contains(output, "1") {
		t.Errorf("expected output to list node 1, got: %q", output)
	}
}

func TestWizardReview_NoProof(t *testing.T) {
	tmpDir, cleanup := setupWizardTest(t)
	defer cleanup()

	cmd := newTestWizardCmd()
	_, err := executeCommand(cmd, "wizard", "review", "--dir", tmpDir)

	// Should error because no proof is initialized
	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

func TestWizardCmd_ExpectedFlags(t *testing.T) {
	cmd := newTestWizardCmd()

	// Find the wizard subcommand
	var wizardCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "wizard" {
			wizardCmd = sub
			break
		}
	}

	if wizardCmd == nil {
		t.Fatal("wizard command not found")
	}

	// Check new-proof subcommand has expected flags
	var newProofCmd *cobra.Command
	for _, sub := range wizardCmd.Commands() {
		if sub.Name() == "new-proof" {
			newProofCmd = sub
			break
		}
	}

	if newProofCmd == nil {
		t.Fatal("wizard new-proof command not found")
	}

	expectedFlags := []string{"conjecture", "author", "dir", "template", "no-confirm", "preview"}
	for _, flagName := range expectedFlags {
		if newProofCmd.Flags().Lookup(flagName) == nil {
			t.Errorf("expected new-proof command to have flag %q", flagName)
		}
	}
}

func TestWizardCmd_ShortFlags(t *testing.T) {
	cmd := newTestWizardCmd()

	// Find the wizard > new-proof command
	var wizardCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "wizard" {
			wizardCmd = sub
			break
		}
	}

	var newProofCmd *cobra.Command
	for _, sub := range wizardCmd.Commands() {
		if sub.Name() == "new-proof" {
			newProofCmd = sub
			break
		}
	}

	if newProofCmd == nil {
		t.Fatal("wizard new-proof command not found")
	}

	// Check short flags
	shortFlags := map[string]string{
		"c": "conjecture",
		"a": "author",
		"d": "dir",
		"t": "template",
	}

	for short, long := range shortFlags {
		if newProofCmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected new-proof command to have short flag -%s for --%s", short, long)
		}
	}
}

// =============================================================================
// Wizard Step Logic Tests (Unit Tests)
// =============================================================================

func TestWizardStep_ValidatesConjecture(t *testing.T) {
	tests := []struct {
		name        string
		conjecture  string
		wantErr     bool
		errContains string
	}{
		{"valid conjecture", "All primes > 2 are odd", false, ""},
		{"empty conjecture", "", true, "empty"},
		{"whitespace only", "   ", true, "empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWizardConjecture(tt.conjecture)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(strings.ToLower(err.Error()), tt.errContains) {
					t.Errorf("expected error to contain %q, got: %v", tt.errContains, err)
				}
			}
		})
	}
}

func TestWizardStep_ValidatesAuthor(t *testing.T) {
	tests := []struct {
		name        string
		author      string
		wantErr     bool
		errContains string
	}{
		{"valid author", "Claude", false, ""},
		{"valid with dash", "prover-001", false, ""},
		{"empty author", "", true, "empty"},
		{"whitespace only", "   ", true, "empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWizardAuthor(tt.author)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(strings.ToLower(err.Error()), tt.errContains) {
					t.Errorf("expected error to contain %q, got: %v", tt.errContains, err)
				}
			}
		})
	}
}

func TestWizardStep_ValidatesTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{"empty template (optional)", "", false},
		{"contradiction", "contradiction", false},
		{"induction", "induction", false},
		{"cases", "cases", false},
		{"invalid template", "notatemplate", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWizardTemplate(tt.template)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// =============================================================================
// Preview Rendering Tests
// =============================================================================

func TestWizardPreview_NewProof(t *testing.T) {
	preview := renderNewProofPreview("Test conjecture", "Test Author", "/tmp/proof", "induction")

	// Preview should include all provided info
	expectations := []string{
		"Test conjecture",
		"Test Author",
		"/tmp/proof",
		"induction",
	}

	for _, exp := range expectations {
		if !strings.Contains(preview, exp) {
			t.Errorf("expected preview to contain %q, got: %q", exp, preview)
		}
	}

	// Preview should mention what will happen
	if !strings.Contains(strings.ToLower(preview), "will") || !strings.Contains(strings.ToLower(preview), "creat") {
		t.Errorf("expected preview to describe actions, got: %q", preview)
	}
}

// =============================================================================
// Interactive Input Mock Tests
// =============================================================================

// mockReader provides mock stdin for testing interactive prompts.
type mockReader struct {
	data   string
	offset int
}

func (m *mockReader) Read(p []byte) (n int, err error) {
	if m.offset >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.offset:])
	m.offset += n
	return n, nil
}

func TestWizardPrompt_ReadsLine(t *testing.T) {
	reader := &mockReader{data: "test input\n"}
	result, err := readWizardLine(reader)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result != "test input" {
		t.Errorf("expected 'test input', got: %q", result)
	}
}

func TestWizardPrompt_TrimsWhitespace(t *testing.T) {
	reader := &mockReader{data: "  test input  \n"}
	result, err := readWizardLine(reader)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result != "test input" {
		t.Errorf("expected 'test input' (trimmed), got: %q", result)
	}
}

func TestWizardConfirm_Yes(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"YES\n", true},
		{"Yes\n", true},
		{"n\n", false},
		{"N\n", false},
		{"no\n", false},
		{"NO\n", false},
		{"anything\n", false},
		{"\n", false}, // default to no
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			reader := &mockReader{data: tt.input}
			result := readWizardConfirm(reader)

			if result != tt.expected {
				t.Errorf("for input %q, expected %v, got %v", tt.input, tt.expected, result)
			}
		})
	}
}
