package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// TestNewCompletionCmd verifies the completion command is created properly.
func TestNewCompletionCmd(t *testing.T) {
	cmd := newCompletionCmd()

	if cmd == nil {
		t.Fatal("newCompletionCmd() returned nil")
	}

	if cmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("unexpected Use: got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if cmd.Long == "" {
		t.Error("Long description should not be empty")
	}
}

// TestCompletionBash verifies bash completion script is generated.
func TestCompletionBash(t *testing.T) {
	// Create a minimal parent command for the completion to work
	rootCmd := &cobra.Command{Use: "af"}
	completionCmd := newCompletionCmd()
	rootCmd.AddCommand(completionCmd)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"completion", "bash"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("completion bash failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "bash completion") || !strings.Contains(output, "__start_af") {
		// Check for either the header comment or the function name
		if !strings.Contains(output, "af") {
			t.Errorf("bash completion output doesn't look like valid bash completion script")
		}
	}
}

// TestCompletionZsh verifies zsh completion script is generated.
func TestCompletionZsh(t *testing.T) {
	rootCmd := &cobra.Command{Use: "af"}
	completionCmd := newCompletionCmd()
	rootCmd.AddCommand(completionCmd)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"completion", "zsh"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("completion zsh failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "compdef") && !strings.Contains(output, "_af") {
		t.Errorf("zsh completion output doesn't look like valid zsh completion script")
	}
}

// TestCompletionFish verifies fish completion script is generated.
func TestCompletionFish(t *testing.T) {
	rootCmd := &cobra.Command{Use: "af"}
	completionCmd := newCompletionCmd()
	rootCmd.AddCommand(completionCmd)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"completion", "fish"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("completion fish failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "complete") {
		t.Errorf("fish completion output doesn't look like valid fish completion script")
	}
}

// TestCompletionPowershell verifies powershell completion script is generated.
func TestCompletionPowershell(t *testing.T) {
	rootCmd := &cobra.Command{Use: "af"}
	completionCmd := newCompletionCmd()
	rootCmd.AddCommand(completionCmd)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"completion", "powershell"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("completion powershell failed: %v", err)
	}

	output := buf.String()
	// PowerShell completion scripts contain Register-ArgumentCompleter
	if !strings.Contains(output, "Register-ArgumentCompleter") && !strings.Contains(output, "af") {
		t.Errorf("powershell completion output doesn't look like valid powershell completion script")
	}
}

// TestCompletionNoArgsShowsHelp verifies that running completion without args shows help.
func TestCompletionNoArgsShowsHelp(t *testing.T) {
	rootCmd := &cobra.Command{Use: "af"}
	completionCmd := newCompletionCmd()
	rootCmd.AddCommand(completionCmd)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"completion"})

	// Should succeed with help output
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("completion with no args should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "bash") && !strings.Contains(output, "zsh") {
		t.Errorf("help output should mention supported shells, got: %s", output)
	}
}

// TestCompletionInvalidShell verifies that an invalid shell name returns an error.
func TestCompletionInvalidShell(t *testing.T) {
	rootCmd := &cobra.Command{Use: "af"}
	completionCmd := newCompletionCmd()
	rootCmd.AddCommand(completionCmd)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"completion", "invalidshell"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("completion with invalid shell should return error")
	}
}

// TestGetNodeIDsForCompletion verifies that node ID completion function works.
func TestGetNodeIDsForCompletion(t *testing.T) {
	// Create a temp directory with an initialized proof
	tmpDir := t.TempDir()

	// Initialize a proof
	if err := service.Init(tmpDir, "Test conjecture", "TestAuthor"); err != nil {
		t.Fatalf("failed to initialize proof: %v", err)
	}

	// Get node IDs
	nodeIDs := getNodeIDsForCompletion(tmpDir)

	// Should have at least the root node "1"
	if len(nodeIDs) == 0 {
		t.Error("getNodeIDsForCompletion returned empty slice for initialized proof")
	}

	found := false
	for _, id := range nodeIDs {
		if id == "1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected node ID '1' in completions, got: %v", nodeIDs)
	}
}

// TestGetNodeIDsForCompletionNoProof verifies completion works when no proof exists.
func TestGetNodeIDsForCompletionNoProof(t *testing.T) {
	// Create a temp directory without a proof
	tmpDir := t.TempDir()

	// Get node IDs - should return empty without error
	nodeIDs := getNodeIDsForCompletion(tmpDir)

	if nodeIDs == nil {
		t.Error("getNodeIDsForCompletion should return empty slice, not nil")
	}

	if len(nodeIDs) != 0 {
		t.Errorf("expected empty slice for non-proof directory, got: %v", nodeIDs)
	}
}

// TestNodeIDCompletionFunction verifies the Cobra ValidArgsFunction works.
func TestNodeIDCompletionFunction(t *testing.T) {
	// Create a temp directory with an initialized proof
	tmpDir := t.TempDir()

	// Initialize a proof
	if err := service.Init(tmpDir, "Test conjecture", "TestAuthor"); err != nil {
		t.Fatalf("failed to initialize proof: %v", err)
	}

	// Create the completion function
	completeFn := createNodeIDCompletionFunc()

	// Create a mock command with the --dir flag
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringP("dir", "d", tmpDir, "Proof directory")

	// Call the completion function
	completions, directive := completeFn(cmd, []string{}, "")

	// Should return completions with no error
	if directive == cobra.ShellCompDirectiveError {
		t.Error("completion function returned error directive")
	}

	// Should have the root node
	found := false
	for _, c := range completions {
		if c == "1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '1' in completions, got: %v", completions)
	}
}

// TestNodeIDCompletionWithPrefix verifies prefix filtering works.
func TestNodeIDCompletionWithPrefix(t *testing.T) {
	// Create a temp directory with an initialized proof
	tmpDir := t.TempDir()

	// Initialize a proof
	if err := service.Init(tmpDir, "Test conjecture", "TestAuthor"); err != nil {
		t.Fatalf("failed to initialize proof: %v", err)
	}

	// Add some child nodes
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Load state to get root ID
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Claim root node and add children
	rootNode := st.AllNodes()[0]
	if err := svc.ClaimNode(rootNode.ID, "test-owner", 1*60*1e9); err != nil {
		t.Fatalf("failed to claim root: %v", err)
	}

	// Parse the root ID for creating children
	rootID := rootNode.ID

	// Create child nodes 1.1 and 1.2
	childID1, _ := rootID.Child(1)
	if err := svc.CreateNode(childID1, "claim", "Child 1", "assumption"); err != nil {
		t.Fatalf("failed to create child 1: %v", err)
	}
	childID2, _ := rootID.Child(2)
	if err := svc.CreateNode(childID2, "claim", "Child 2", "assumption"); err != nil {
		t.Fatalf("failed to create child 2: %v", err)
	}

	// Create the completion function
	completeFn := createNodeIDCompletionFunc()

	// Create a mock command
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringP("dir", "d", tmpDir, "Proof directory")

	// Call with prefix "1."
	completions, _ := completeFn(cmd, []string{}, "1.")

	// Should return children starting with "1."
	childCount := 0
	for _, c := range completions {
		if strings.HasPrefix(c, "1.") {
			childCount++
		}
	}

	if childCount < 2 {
		t.Errorf("expected at least 2 completions starting with '1.', got completions: %v", completions)
	}
}

// TestGetChallengeIDsForCompletion verifies challenge ID completion.
func TestGetChallengeIDsForCompletion(t *testing.T) {
	// Create a temp directory without a proof
	tmpDir := t.TempDir()

	// Get challenge IDs - should return empty without error
	challengeIDs := getChallengeIDsForCompletion(tmpDir)

	if challengeIDs == nil {
		t.Error("getChallengeIDsForCompletion should return empty slice, not nil")
	}
}

// TestCompletionRegistration verifies the completion command is registered.
func TestCompletionRegistration(t *testing.T) {
	// Note: In the actual CLI, the command is registered via init()
	// Here we just verify the command can be created and has proper structure

	cmd := newCompletionCmd()

	// Verify subcommands exist
	subCmds := cmd.Commands()

	// Should have no subcommands (it uses args, not subcommands)
	if len(subCmds) != 0 {
		t.Errorf("completion should not have subcommands, got: %d", len(subCmds))
	}

	// Verify ValidArgs contains supported shells
	if cmd.ValidArgs == nil || len(cmd.ValidArgs) == 0 {
		t.Error("completion command should have ValidArgs for shell names")
	}

	shellsFound := map[string]bool{"bash": false, "zsh": false, "fish": false, "powershell": false}
	for _, arg := range cmd.ValidArgs {
		if _, ok := shellsFound[arg]; ok {
			shellsFound[arg] = true
		}
	}

	for shell, found := range shellsFound {
		if !found {
			t.Errorf("expected %q in ValidArgs", shell)
		}
	}
}

// TestCompletionWithDirFlag verifies completion respects --dir flag.
func TestCompletionWithDirFlag(t *testing.T) {
	// Create two temp directories with proofs
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	// Initialize proofs in both
	if err := service.Init(tmpDir1, "Conjecture 1", "Author1"); err != nil {
		t.Fatalf("failed to init proof 1: %v", err)
	}
	if err := service.Init(tmpDir2, "Conjecture 2", "Author2"); err != nil {
		t.Fatalf("failed to init proof 2: %v", err)
	}

	// Test completion from dir 1
	ids1 := getNodeIDsForCompletion(tmpDir1)
	if len(ids1) == 0 {
		t.Error("dir 1 should have node IDs")
	}

	// Test completion from dir 2
	ids2 := getNodeIDsForCompletion(tmpDir2)
	if len(ids2) == 0 {
		t.Error("dir 2 should have node IDs")
	}
}

// TestCompletionScriptInstallInstructions verifies help text includes install instructions.
func TestCompletionScriptInstallInstructions(t *testing.T) {
	cmd := newCompletionCmd()

	// The Long description should include installation instructions
	long := cmd.Long

	// Should mention how to use the completion
	if !strings.Contains(long, "source") && !strings.Contains(long, "eval") {
		t.Error("Long description should include installation instructions")
	}

	// Should mention at least one shell
	if !strings.Contains(long, "bash") {
		t.Error("Long description should mention bash")
	}
}

// TestCompletionFromCurrentDir verifies completion works with default dir.
func TestCompletionFromCurrentDir(t *testing.T) {
	// Save current dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(origDir)

	// Create and cd to temp directory with proof
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Initialize a proof in current directory
	if err := service.Init(".", "Test conjecture", "TestAuthor"); err != nil {
		t.Fatalf("failed to initialize proof: %v", err)
	}

	// Get node IDs using current directory (default)
	nodeIDs := getNodeIDsForCompletion(".")

	// Should have at least the root node
	if len(nodeIDs) == 0 {
		t.Error("getNodeIDsForCompletion should find nodes in current directory")
	}
}

// TestGetDefinitionIDsForCompletion verifies definition ID completion.
func TestGetDefinitionIDsForCompletion(t *testing.T) {
	tmpDir := t.TempDir()

	// Without proof, should return empty
	defIDs := getDefinitionIDsForCompletion(tmpDir)
	if len(defIDs) != 0 {
		t.Errorf("expected empty slice for non-proof dir, got: %v", defIDs)
	}

	// Initialize proof
	if err := service.Init(tmpDir, "Test", "Author"); err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	// Still no definitions
	defIDs = getDefinitionIDsForCompletion(tmpDir)
	if len(defIDs) != 0 {
		t.Errorf("expected empty slice for proof without definitions, got: %v", defIDs)
	}
}

// TestMultipleNodeIDCompletion verifies completion for commands that accept multiple node IDs.
func TestMultipleNodeIDCompletion(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize proof
	if err := service.Init(tmpDir, "Test", "Author"); err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	completeFn := createNodeIDCompletionFunc()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringP("dir", "d", tmpDir, "Proof directory")

	// Call with one existing arg
	completions, directive := completeFn(cmd, []string{"1"}, "")

	// Should still offer completions for additional IDs
	if directive == cobra.ShellCompDirectiveError {
		t.Error("should not return error directive")
	}

	// Root should still be in completions (can reference same node multiple times in some contexts)
	if len(completions) == 0 {
		t.Error("should offer completions even with existing args")
	}
}

// TestGetExternalIDsForCompletion verifies external reference completion.
func TestGetExternalIDsForCompletion(t *testing.T) {
	tmpDir := t.TempDir()

	// Without proof, should return empty
	extIDs := getExternalIDsForCompletion(tmpDir)
	if len(extIDs) != 0 {
		t.Errorf("expected empty slice for non-proof dir, got: %v", extIDs)
	}

	// Initialize proof
	if err := service.Init(tmpDir, "Test", "Author"); err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	// Still no externals
	extIDs = getExternalIDsForCompletion(tmpDir)
	if len(extIDs) != 0 {
		t.Errorf("expected empty slice for proof without externals, got: %v", extIDs)
	}
}

// TestCompletionWithAfProof verifies completion finds proof in .af directory.
func TestCompletionWithAfProof(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .af subdirectory (the proof directory)
	afDir := filepath.Join(tmpDir, ".af")
	if err := os.MkdirAll(afDir, 0755); err != nil {
		t.Fatalf("failed to create .af dir: %v", err)
	}

	// Initialize proof in .af directory
	if err := service.Init(afDir, "Test", "Author"); err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	// Get completions from the .af directory
	nodeIDs := getNodeIDsForCompletion(afDir)
	if len(nodeIDs) == 0 {
		t.Error("should find nodes in .af directory")
	}
}
