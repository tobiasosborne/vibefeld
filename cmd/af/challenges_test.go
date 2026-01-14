//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestChallengesCmd creates a fresh root command with the challenges subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestChallengesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	challengesCmd := newChallengesCmd()
	cmd.AddCommand(challengesCmd)

	return cmd
}

// executeChallengesCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeChallengesCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// setupChallengesTest creates a temp directory with an initialized proof.
func setupChallengesTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-challenges-test-*")
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

// setupChallengesTestWithChallenges creates a proof with some challenges.
func setupChallengesTestWithChallenges(t *testing.T) (string, func()) {
	t.Helper()

	proofDir, cleanup := setupChallengesTest(t)

	// Create service and add a challenge via ledger
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add challenges via the ledger
	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add an open challenge on node 1
	nodeID1, _ := types.Parse("1")
	event1 := ledger.NewChallengeRaised("ch-001", nodeID1, "gap", "Missing case for n=0")
	if _, err := ldg.Append(event1); err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add another open challenge on node 1
	event2 := ledger.NewChallengeRaised("ch-002", nodeID1, "statement", "Statement is unclear")
	if _, err := ldg.Append(event2); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return proofDir, cleanup
}

// setupChallengesTestWithMultipleNodes creates a proof with challenges on multiple nodes.
func setupChallengesTestWithMultipleNodes(t *testing.T) (string, func()) {
	t.Helper()

	proofDir, cleanup := setupChallengesTest(t)

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add challenges via the ledger
	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add challenge on node 1
	nodeID1, _ := types.Parse("1")
	event1 := ledger.NewChallengeRaised("ch-abc123", nodeID1, "gap", "Missing case for n=0")
	if _, err := ldg.Append(event1); err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Resolve the challenge
	event2 := ledger.NewChallengeResolved("ch-abc123")
	if _, err := ldg.Append(event2); err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add another open challenge
	event3 := ledger.NewChallengeRaised("ch-def456", nodeID1, "context", "Undefined variable")
	if _, err := ldg.Append(event3); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return proofDir, cleanup
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestChallengesCmd_ProofNotInitialized verifies error when proof hasn't been initialized.
func TestChallengesCmd_ProofNotInitialized(t *testing.T) {
	// Create empty directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-challenges-noinit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	proofDir := filepath.Join(tmpDir, "proof")
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	// Note: NOT calling service.Init(), so proof is not initialized

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--dir", proofDir)

	// Should error because proof is not initialized
	if err == nil {
		t.Error("expected error for uninitialized proof, got nil")
		return
	}

	// Error should indicate proof is not initialized
	combined := output + err.Error()
	lowerCombined := strings.ToLower(combined)
	if !strings.Contains(lowerCombined, "not initialized") &&
		!strings.Contains(lowerCombined, "no proof") {
		t.Logf("Error message: %q", combined)
	}
}

// TestChallengesCmd_DirFlagNonExistent verifies error for non-existent directory.
func TestChallengesCmd_DirFlagNonExistent(t *testing.T) {
	cmd := newTestChallengesCmd()
	_, err := executeChallengesCommand(cmd, "challenges", "--dir", "/nonexistent/path/to/proof")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestChallengesCmd_InvalidFormat verifies error for invalid format.
func TestChallengesCmd_InvalidFormat(t *testing.T) {
	proofDir, cleanup := setupChallengesTest(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	_, err := executeChallengesCommand(cmd, "challenges", "--format", "xml", "--dir", proofDir)

	if err == nil {
		t.Error("expected error for invalid format, got nil")
	}
}

// TestChallengesCmd_InvalidStatus verifies error for invalid status filter.
func TestChallengesCmd_InvalidStatus(t *testing.T) {
	proofDir, cleanup := setupChallengesTest(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	_, err := executeChallengesCommand(cmd, "challenges", "--status", "invalid", "--dir", proofDir)

	if err == nil {
		t.Error("expected error for invalid status, got nil")
	}
}

// TestChallengesCmd_InvalidNodeID verifies error for invalid node ID filter.
func TestChallengesCmd_InvalidNodeID(t *testing.T) {
	proofDir, cleanup := setupChallengesTest(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	_, err := executeChallengesCommand(cmd, "challenges", "--node", "invalid-id", "--dir", proofDir)

	if err == nil {
		t.Error("expected error for invalid node ID, got nil")
	}
}

// =============================================================================
// Happy Path Tests
// =============================================================================

// TestChallengesCmd_NoChallenges verifies output when no challenges exist.
func TestChallengesCmd_NoChallenges(t *testing.T) {
	proofDir, cleanup := setupChallengesTest(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should indicate no challenges
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "no challenges") &&
		!strings.Contains(lowerOutput, "total: 0") {
		t.Errorf("expected output to indicate no challenges, got: %q", output)
	}
}

// TestChallengesCmd_WithChallenges verifies output when challenges exist.
func TestChallengesCmd_WithChallenges(t *testing.T) {
	proofDir, cleanup := setupChallengesTestWithChallenges(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show challenge information
	expectations := []string{
		"ch-001",
		"ch-002",
		"gap",
		"statement",
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got: %q", exp, output)
		}
	}
}

// TestChallengesCmd_ShowsHeader verifies table header is displayed.
func TestChallengesCmd_ShowsHeader(t *testing.T) {
	proofDir, cleanup := setupChallengesTestWithChallenges(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show table header
	headerWords := []string{"CHALLENGE", "NODE", "STATUS", "TARGET", "REASON"}
	for _, word := range headerWords {
		if !strings.Contains(output, word) {
			t.Errorf("expected header to contain %q, got: %q", word, output)
		}
	}
}

// =============================================================================
// Filter Tests
// =============================================================================

// TestChallengesCmd_FilterByStatus verifies --status filtering.
func TestChallengesCmd_FilterByStatus(t *testing.T) {
	proofDir, cleanup := setupChallengesTestWithMultipleNodes(t)
	defer cleanup()

	tests := []struct {
		name      string
		status    string
		expectIDs []string
		notIDs    []string
	}{
		{
			name:      "open only",
			status:    "open",
			expectIDs: []string{"ch-def456"},
			notIDs:    []string{"ch-abc123"},
		},
		{
			name:      "resolved only",
			status:    "resolved",
			expectIDs: []string{"ch-abc123"},
			notIDs:    []string{"ch-def456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestChallengesCmd()
			output, err := executeChallengesCommand(cmd, "challenges", "--status", tt.status, "--dir", proofDir)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, id := range tt.expectIDs {
				if !strings.Contains(output, id) {
					t.Errorf("expected output to contain %q for status %q, got: %q", id, tt.status, output)
				}
			}

			for _, id := range tt.notIDs {
				if strings.Contains(output, id) {
					t.Errorf("expected output NOT to contain %q for status %q, got: %q", id, tt.status, output)
				}
			}
		})
	}
}

// TestChallengesCmd_FilterByNode verifies --node filtering.
func TestChallengesCmd_FilterByNode(t *testing.T) {
	proofDir, cleanup := setupChallengesTestWithChallenges(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--node", "1", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show challenges for node 1
	if !strings.Contains(output, "ch-001") {
		t.Errorf("expected output to contain challenges for node 1, got: %q", output)
	}
}

// TestChallengesCmd_FilterByNonExistentNode verifies filtering by node with no challenges.
func TestChallengesCmd_FilterByNonExistentNode(t *testing.T) {
	proofDir, cleanup := setupChallengesTestWithChallenges(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--node", "2", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show no challenges
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "no challenges") {
		t.Errorf("expected no challenges for node 2, got: %q", output)
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestChallengesCmd_JSONOutput verifies JSON output format.
func TestChallengesCmd_JSONOutput(t *testing.T) {
	proofDir, cleanup := setupChallengesTestWithChallenges(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--format", "json", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v\nOutput: %s", err, output)
		return
	}

	// JSON should include challenges array and total
	if _, ok := result["challenges"]; !ok {
		t.Error("expected JSON to contain 'challenges' key")
	}

	if _, ok := result["total"]; !ok {
		t.Error("expected JSON to contain 'total' key")
	}
}

// TestChallengesCmd_JSONOutputStructure verifies JSON structure.
func TestChallengesCmd_JSONOutputStructure(t *testing.T) {
	proofDir, cleanup := setupChallengesTestWithChallenges(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--format", "json", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse JSON and check structure
	var result struct {
		Challenges []struct {
			ID     string `json:"id"`
			NodeID string `json:"node_id"`
			Status string `json:"status"`
			Target string `json:"target"`
			Reason string `json:"reason"`
		} `json:"challenges"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("expected total of 2 challenges, got %d", result.Total)
	}

	if len(result.Challenges) != 2 {
		t.Errorf("expected 2 challenges in array, got %d", len(result.Challenges))
	}

	// Check each challenge has required fields
	for i, c := range result.Challenges {
		if c.ID == "" {
			t.Errorf("challenge %d missing ID", i)
		}
		if c.NodeID == "" {
			t.Errorf("challenge %d missing node_id", i)
		}
		if c.Status == "" {
			t.Errorf("challenge %d missing status", i)
		}
	}
}

// TestChallengesCmd_JSONOutputNoChallenges verifies JSON output when no challenges.
func TestChallengesCmd_JSONOutputNoChallenges(t *testing.T) {
	proofDir, cleanup := setupChallengesTest(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--format", "json", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still be valid JSON
	var result struct {
		Challenges []interface{} `json:"challenges"`
		Total      int           `json:"total"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v", err)
		return
	}

	if result.Total != 0 {
		t.Errorf("expected total of 0, got %d", result.Total)
	}

	if len(result.Challenges) != 0 {
		t.Errorf("expected empty challenges array, got %d items", len(result.Challenges))
	}
}

// TestChallengesCmd_JSONOutputShortFlag verifies JSON output with short flag.
func TestChallengesCmd_JSONOutputShortFlag(t *testing.T) {
	proofDir, cleanup := setupChallengesTestWithChallenges(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "-f", "json", "-d", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output with -f flag, got error: %v", err)
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestChallengesCmd_Help verifies help output shows usage information.
func TestChallengesCmd_Help(t *testing.T) {
	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"challenges", // Command name
		"--status",   // Status filter flag
		"--node",     // Node filter flag
		"--format",   // Format flag
		"--dir",      // Directory flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestChallengesCmd_HelpShortFlag verifies help with short flag.
func TestChallengesCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "challenges") {
		t.Errorf("expected help output to mention 'challenges', got: %q", output)
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestChallengesCmd_ExpectedFlags ensures the challenges command has expected flag structure.
func TestChallengesCmd_ExpectedFlags(t *testing.T) {
	cmd := newChallengesCmd()

	// Check expected flags exist
	expectedFlags := []string{"status", "node", "format", "dir"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected challenges command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"s": "status",
		"n": "node",
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected challenges command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestChallengesCmd_DefaultFlagValues verifies default values for flags.
func TestChallengesCmd_DefaultFlagValues(t *testing.T) {
	cmd := newChallengesCmd()

	// Check default dir value
	dirFlag := cmd.Flags().Lookup("dir")
	if dirFlag == nil {
		t.Fatal("expected dir flag to exist")
	}
	if dirFlag.DefValue != "." {
		t.Errorf("expected default dir to be '.', got %q", dirFlag.DefValue)
	}

	// Check default format value
	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("expected format flag to exist")
	}
	if formatFlag.DefValue != "text" {
		t.Errorf("expected default format to be 'text', got %q", formatFlag.DefValue)
	}

	// Check default status value (empty - no filter)
	statusFlag := cmd.Flags().Lookup("status")
	if statusFlag == nil {
		t.Fatal("expected status flag to exist")
	}
	if statusFlag.DefValue != "" {
		t.Errorf("expected default status to be empty, got %q", statusFlag.DefValue)
	}

	// Check default node value (empty - no filter)
	nodeFlag := cmd.Flags().Lookup("node")
	if nodeFlag == nil {
		t.Fatal("expected node flag to exist")
	}
	if nodeFlag.DefValue != "" {
		t.Errorf("expected default node to be empty, got %q", nodeFlag.DefValue)
	}
}

// =============================================================================
// Table-Driven Comprehensive Tests
// =============================================================================

// TestChallengesCmd_StatusFilterValues verifies all valid status filter values.
func TestChallengesCmd_StatusFilterValues(t *testing.T) {
	proofDir, cleanup := setupChallengesTest(t)
	defer cleanup()

	tests := []struct {
		name    string
		status  string
		wantErr bool
	}{
		{"open status", "open", false},
		{"resolved status", "resolved", false},
		{"withdrawn status", "withdrawn", false},
		{"Open uppercase", "Open", false},
		{"RESOLVED uppercase", "RESOLVED", false},
		{"invalid status", "pending", true},
		{"invalid status2", "superseded", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestChallengesCmd()
			_, err := executeChallengesCommand(cmd, "challenges", "--status", tt.status, "--dir", proofDir)

			if tt.wantErr && err == nil {
				t.Errorf("expected error for status %q, got nil", tt.status)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for status %q: %v", tt.status, err)
			}
		})
	}
}

// TestChallengesCmd_FormatValues verifies all valid format values.
func TestChallengesCmd_FormatValues(t *testing.T) {
	proofDir, cleanup := setupChallengesTest(t)
	defer cleanup()

	tests := []struct {
		name      string
		format    string
		wantErr   bool
		checkJSON bool
	}{
		{"default format", "", false, false},
		{"text format", "text", false, false},
		{"json format", "json", false, true},
		{"TEXT uppercase", "TEXT", false, false},
		{"JSON uppercase", "JSON", false, true},
		{"invalid format", "xml", true, false},
		{"invalid format2", "yaml", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestChallengesCmd()
			var output string
			var err error

			if tt.format == "" {
				output, err = executeChallengesCommand(cmd, "challenges", "--dir", proofDir)
			} else {
				output, err = executeChallengesCommand(cmd, "challenges", "--format", tt.format, "--dir", proofDir)
			}

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for format %q, got nil", tt.format)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for format %q: %v", tt.format, err)
				return
			}

			if tt.checkJSON {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("expected valid JSON for format %q, got error: %v", tt.format, err)
				}
			}
		})
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestChallengesCmd_CombinedFilters verifies combining multiple filters.
func TestChallengesCmd_CombinedFilters(t *testing.T) {
	proofDir, cleanup := setupChallengesTestWithMultipleNodes(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--node", "1", "--status", "open", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show only open challenges for node 1
	if !strings.Contains(output, "ch-def456") {
		t.Errorf("expected open challenge ch-def456 for node 1, got: %q", output)
	}

	// Should NOT show resolved challenge
	if strings.Contains(output, "ch-abc123") {
		t.Errorf("expected resolved challenge ch-abc123 to be filtered out, got: %q", output)
	}
}

// TestChallengesCmd_OutputShowsSummary verifies summary line in text output.
func TestChallengesCmd_OutputShowsSummary(t *testing.T) {
	proofDir, cleanup := setupChallengesTestWithChallenges(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show total count
	if !strings.Contains(strings.ToLower(output), "total") {
		t.Errorf("expected output to show total count, got: %q", output)
	}
}

// TestChallengesCmd_OutputShowsNextSteps verifies next steps guidance for open challenges.
func TestChallengesCmd_OutputShowsNextSteps(t *testing.T) {
	proofDir, cleanup := setupChallengesTestWithChallenges(t)
	defer cleanup()

	cmd := newTestChallengesCmd()
	output, err := executeChallengesCommand(cmd, "challenges", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show next steps guidance when there are open challenges
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "next steps") ||
		!strings.Contains(lowerOutput, "resolve-challenge") {
		t.Errorf("expected output to show next steps guidance, got: %q", output)
	}
}

// TestChallengesCmd_CommandMetadata verifies command metadata.
func TestChallengesCmd_CommandMetadata(t *testing.T) {
	cmd := newChallengesCmd()

	if cmd.Use != "challenges" {
		t.Errorf("expected Use to be 'challenges', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if cmd.Long == "" {
		t.Error("expected Long description to be set")
	}
}
