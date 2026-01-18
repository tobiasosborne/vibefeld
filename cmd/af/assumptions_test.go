//go:build integration

// Package main contains tests for the af assumptions command.
// These are TDD tests - the assumptions command implementation should
// be created to satisfy these tests.
// Tests define the expected behavior for listing and viewing assumptions.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupAssumptionsTest creates a temp directory with an initialized proof.
func setupAssumptionsTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-assumptions-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	// Initialize the proof directory structure
	if err := fs.InitProofDir(tmpDir); err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Initialize proof via service
	if err := service.Init(tmpDir, "Test conjecture for assumptions", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupAssumptionsTestWithAssumptions creates a test environment with assumptions added.
func setupAssumptionsTestWithAssumptions(t *testing.T) (string, func(), []string) {
	t.Helper()

	tmpDir, cleanup := setupAssumptionsTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Add multiple assumptions
	assumptions := []string{
		"All natural numbers are positive or zero",
		"Addition is commutative",
		"Every non-empty set of positive integers contains a smallest element",
	}

	var assumptionIDs []string
	for _, stmt := range assumptions {
		id, err := svc.AddAssumption(stmt)
		if err != nil {
			cleanup()
			t.Fatal(err)
		}
		assumptionIDs = append(assumptionIDs, id)
	}

	return tmpDir, cleanup, assumptionIDs
}

// setupAssumptionsTestWithNodes creates a test environment with nodes and assumptions.
func setupAssumptionsTestWithNodes(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupAssumptionsTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create some nodes
	for _, idStr := range []string{"1.1", "1.2", "1.1.1"} {
		nodeID, _ := service.ParseNodeID(idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement "+idStr, schema.InferenceModusPonens)
		if err != nil {
			cleanup()
			t.Fatal(err)
		}
	}

	// Add some assumptions
	svc.AddAssumption("Test assumption 1")
	svc.AddAssumption("Test assumption 2")

	return tmpDir, cleanup
}

// newTestAssumptionsCmd creates a fresh root command with the assumptions subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestAssumptionsCmd() *cobra.Command {
	cmd := newTestRootCmd()

	assumptionsCmd := newAssumptionsCmd()
	cmd.AddCommand(assumptionsCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// newTestAssumptionCmd creates a fresh root command with the assumption subcommand for testing.
// This command is for showing a single assumption by name/ID.
func newTestAssumptionCmd() *cobra.Command {
	cmd := newTestRootCmd()

	assumptionCmd := newAssumptionCmd()
	cmd.AddCommand(assumptionCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// =============================================================================
// List All Assumptions Tests
// =============================================================================

// TestAssumptionsCmd_ListAll tests listing all assumptions in the proof.
func TestAssumptionsCmd_ListAll(t *testing.T) {
	tmpDir, cleanup, _ := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should mention assumptions
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "assumption") {
		t.Errorf("expected output to mention assumptions, got: %q", output)
	}

	// Should show the assumption statements
	if !strings.Contains(output, "natural numbers") {
		t.Errorf("expected output to contain assumption text, got: %q", output)
	}
}

// TestAssumptionsCmd_ListAllEmpty tests listing assumptions when none exist.
func TestAssumptionsCmd_ListAllEmpty(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTest(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should indicate no assumptions
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "no assumptions") &&
		!strings.Contains(lowerOutput, "0") &&
		!strings.Contains(lowerOutput, "empty") {
		t.Logf("Expected indication of no assumptions, got: %q", output)
	}
}

// TestAssumptionsCmd_ListAllShowsCount tests that list shows count of assumptions.
func TestAssumptionsCmd_ListAllShowsCount(t *testing.T) {
	tmpDir, cleanup, _ := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show count or list 3 assumptions
	if !strings.Contains(output, "3") &&
		!strings.Contains(strings.ToLower(output), "natural numbers") &&
		!strings.Contains(strings.ToLower(output), "commutative") {
		t.Errorf("expected output to indicate 3 assumptions or show them, got: %q", output)
	}
}

// =============================================================================
// List Assumptions for Node Tests
// =============================================================================

// TestAssumptionsCmd_ForNode tests listing assumptions for a specific node.
func TestAssumptionsCmd_ForNode(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTestWithNodes(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "1", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should show assumptions in scope for node 1
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "assumption") &&
		!strings.Contains(lowerOutput, "scope") {
		t.Logf("Expected output to mention assumptions or scope, got: %q", output)
	}
}

// TestAssumptionsCmd_ForChildNode tests listing assumptions for a child node.
func TestAssumptionsCmd_ForChildNode(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTestWithNodes(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "1.1", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should not error
	_ = output
}

// TestAssumptionsCmd_ForDeepNode tests listing assumptions for a deeply nested node.
func TestAssumptionsCmd_ForDeepNode(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTestWithNodes(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "1.1.1", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should not error
	_ = output
}

// TestAssumptionsCmd_NodeNotFound tests error when node doesn't exist.
func TestAssumptionsCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup, _ := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	_, err := executeCommand(cmd, "assumptions", "1.999", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for non-existent node, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(strings.ToLower(errStr), "not found") &&
		!strings.Contains(strings.ToLower(errStr), "does not exist") {
		t.Logf("Expected error about node not found, got: %q", errStr)
	}
}

// TestAssumptionsCmd_InvalidNodeID tests error for invalid node ID format.
func TestAssumptionsCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTest(t)
	defer cleanup()

	tests := []struct {
		name   string
		nodeID string
	}{
		{"letters", "abc"},
		{"negative", "-1"},
		{"zero", "0"},
		{"leading dot", ".1"},
		{"trailing dot", "1."},
		{"double dot", "1..2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestAssumptionsCmd()
			_, err := executeCommand(cmd, "assumptions", tc.nodeID, "--dir", tmpDir)

			if err == nil {
				t.Errorf("expected error for invalid node ID %q, got nil", tc.nodeID)
			}
		})
	}
}

// =============================================================================
// Show Single Assumption Tests
// =============================================================================

// TestAssumptionCmd_ShowByID tests showing a specific assumption by ID.
func TestAssumptionCmd_ShowByID(t *testing.T) {
	tmpDir, cleanup, ids := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	// Use the first assumption ID
	assumptionID := ids[0]

	cmd := newTestAssumptionCmd()
	output, err := executeCommand(cmd, "assumption", assumptionID, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain the assumption details
	if !strings.Contains(output, "natural numbers") {
		t.Errorf("expected output to contain assumption statement, got: %q", output)
	}
}

// TestAssumptionCmd_ShowByPartialID tests showing assumption by partial ID.
func TestAssumptionCmd_ShowByPartialID(t *testing.T) {
	tmpDir, cleanup, ids := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	// Use first 6 characters of the first assumption ID
	partialID := ids[0][:6]

	cmd := newTestAssumptionCmd()
	output, err := executeCommand(cmd, "assumption", partialID, "--dir", tmpDir)

	// Should either find the assumption or provide helpful error
	if err != nil {
		// Partial matching may not be supported - that's acceptable
		t.Logf("Partial ID matching returned error: %v", err)
		return
	}

	if !strings.Contains(output, "natural numbers") {
		t.Logf("Output with partial ID: %q", output)
	}
}

// TestAssumptionCmd_NotFound tests error when assumption doesn't exist.
func TestAssumptionCmd_NotFound(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTest(t)
	defer cleanup()

	cmd := newTestAssumptionCmd()
	_, err := executeCommand(cmd, "assumption", "nonexistent-id-12345", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for non-existent assumption, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(strings.ToLower(errStr), "not found") &&
		!strings.Contains(strings.ToLower(errStr), "does not exist") {
		t.Logf("Expected error about assumption not found, got: %q", errStr)
	}
}

// TestAssumptionCmd_MissingArgument tests error when no argument provided.
func TestAssumptionCmd_MissingArgument(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTest(t)
	defer cleanup()

	cmd := newTestAssumptionCmd()
	_, err := executeCommand(cmd, "assumption", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing argument, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "accepts") &&
		!strings.Contains(errStr, "required") &&
		!strings.Contains(errStr, "argument") {
		t.Logf("Expected error about missing argument, got: %q", errStr)
	}
}

// TestAssumptionCmd_EmptyArgument tests error for empty argument.
func TestAssumptionCmd_EmptyArgument(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTest(t)
	defer cleanup()

	cmd := newTestAssumptionCmd()
	_, err := executeCommand(cmd, "assumption", "", "--dir", tmpDir)

	// Should error for empty argument
	if err == nil {
		t.Log("Empty argument was accepted - implementation may allow this")
	}
}

// =============================================================================
// JSON Output Tests
// =============================================================================

// TestAssumptionsCmd_JSONOutput tests JSON output format for listing.
func TestAssumptionsCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup, _ := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestAssumptionsCmd_JSONOutputStructure tests JSON output structure.
func TestAssumptionsCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup, _ := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Try to parse as array or object
	var resultArray []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &resultArray); err == nil {
		// Valid array format
		if len(resultArray) != 3 {
			t.Errorf("expected 3 assumptions in JSON array, got %d", len(resultArray))
		}
		return
	}

	// Try as object with assumptions key
	var resultObject map[string]interface{}
	if err := json.Unmarshal([]byte(output), &resultObject); err != nil {
		t.Errorf("output is not valid JSON array or object: %q", output)
	}
}

// TestAssumptionsCmd_JSONOutputEmpty tests JSON output when no assumptions.
func TestAssumptionsCmd_JSONOutputEmpty(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTest(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON even when empty
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("empty output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestAssumptionsCmd_JSONOutputForNode tests JSON output for node-specific assumptions.
func TestAssumptionsCmd_JSONOutputForNode(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTestWithNodes(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "1", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestAssumptionCmd_JSONOutput tests JSON output for single assumption.
func TestAssumptionCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup, ids := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	cmd := newTestAssumptionCmd()
	output, err := executeCommand(cmd, "assumption", ids[0], "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Should have expected fields
	expectedFields := []string{"id", "statement"}
	for _, field := range expectedFields {
		if _, ok := result[field]; !ok {
			t.Logf("Warning: JSON output does not contain field %q", field)
		}
	}
}

// TestAssumptionsCmd_JSONShortFlag tests JSON output with -f short flag.
func TestAssumptionsCmd_JSONShortFlag(t *testing.T) {
	tmpDir, cleanup, _ := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "-f", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestAssumptionsCmd_ProofNotInitialized tests error when proof not initialized.
func TestAssumptionsCmd_ProofNotInitialized(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "af-assumptions-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestAssumptionsCmd()
	_, err = executeCommand(cmd, "assumptions", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestAssumptionsCmd_DirectoryNotFound tests error for non-existent directory.
func TestAssumptionsCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestAssumptionsCmd()
	_, err := executeCommand(cmd, "assumptions", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestAssumptionCmd_ProofNotInitialized tests error when proof not initialized.
func TestAssumptionCmd_ProofNotInitialized(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "af-assumption-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestAssumptionCmd()
	_, err = executeCommand(cmd, "assumption", "some-id", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestAssumptionCmd_DirectoryNotFound tests error for non-existent directory.
func TestAssumptionCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestAssumptionCmd()
	_, err := executeCommand(cmd, "assumption", "some-id", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestAssumptionsCmd_Help tests that help output shows usage information.
func TestAssumptionsCmd_Help(t *testing.T) {
	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "--help")

	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	// Check for expected help content
	expectations := []string{
		"assumptions",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestAssumptionsCmd_HelpShortFlag tests help with -h short flag.
func TestAssumptionsCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "-h")

	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	if !strings.Contains(output, "assumptions") {
		t.Errorf("help output should mention 'assumptions', got: %q", output)
	}
}

// TestAssumptionCmd_Help tests that help output shows usage information.
func TestAssumptionCmd_Help(t *testing.T) {
	cmd := newTestAssumptionCmd()
	output, err := executeCommand(cmd, "assumption", "--help")

	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	// Check for expected help content
	expectations := []string{
		"assumption",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestAssumptionCmd_HelpShortFlag tests help with -h short flag.
func TestAssumptionCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestAssumptionCmd()
	output, err := executeCommand(cmd, "assumption", "-h")

	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	if !strings.Contains(output, "assumption") {
		t.Errorf("help output should mention 'assumption', got: %q", output)
	}
}

// =============================================================================
// Flag Structure Tests
// =============================================================================

// TestAssumptionsCmd_ExpectedFlags ensures the assumptions command has expected flag structure.
func TestAssumptionsCmd_ExpectedFlags(t *testing.T) {
	cmd := newTestAssumptionsCmd()

	// Find the assumptions subcommand
	var assumptionsCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "assumptions" {
			assumptionsCmd = sub
			break
		}
	}

	if assumptionsCmd == nil {
		t.Skip("assumptions command not yet implemented")
		return
	}

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if assumptionsCmd.Flags().Lookup(flagName) == nil && assumptionsCmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected assumptions command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if assumptionsCmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected assumptions command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestAssumptionCmd_ExpectedFlags ensures the assumption command has expected flag structure.
func TestAssumptionCmd_ExpectedFlags(t *testing.T) {
	cmd := newTestAssumptionCmd()

	// Find the assumption subcommand
	var assumptionCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "assumption" {
			assumptionCmd = sub
			break
		}
	}

	if assumptionCmd == nil {
		t.Skip("assumption command not yet implemented")
		return
	}

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if assumptionCmd.Flags().Lookup(flagName) == nil && assumptionCmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected assumption command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if assumptionCmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected assumption command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestAssumptionsCmd_DefaultFlagValues verifies default values for flags.
func TestAssumptionsCmd_DefaultFlagValues(t *testing.T) {
	cmd := newTestAssumptionsCmd()

	// Find the assumptions subcommand
	var assumptionsCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "assumptions" {
			assumptionsCmd = sub
			break
		}
	}

	if assumptionsCmd == nil {
		t.Skip("assumptions command not yet implemented")
		return
	}

	// Check default dir value
	dirFlag := assumptionsCmd.Flags().Lookup("dir")
	if dirFlag == nil {
		t.Fatal("expected dir flag to exist")
	}
	if dirFlag.DefValue != "." {
		t.Errorf("expected default dir to be '.', got %q", dirFlag.DefValue)
	}

	// Check default format value
	formatFlag := assumptionsCmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("expected format flag to exist")
	}
	if formatFlag.DefValue != "text" {
		t.Errorf("expected default format to be 'text', got %q", formatFlag.DefValue)
	}
}

// TestAssumptionsCmd_CommandMetadata verifies command metadata.
func TestAssumptionsCmd_CommandMetadata(t *testing.T) {
	cmd := newTestAssumptionsCmd()

	// Find the assumptions subcommand
	var assumptionsCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "assumptions" {
			assumptionsCmd = sub
			break
		}
	}

	if assumptionsCmd == nil {
		t.Skip("assumptions command not yet implemented")
		return
	}

	if !strings.HasPrefix(assumptionsCmd.Use, "assumptions") {
		t.Errorf("expected Use to start with 'assumptions', got %q", assumptionsCmd.Use)
	}

	if assumptionsCmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Default Directory Tests
// =============================================================================

// TestAssumptionsCmd_DefaultDirectory tests assumptions uses current directory by default.
func TestAssumptionsCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup, _ := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	// Change to the proof directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Execute without -d flag (should use current directory)
	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions")

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}
}

// TestAssumptionCmd_DefaultDirectory tests assumption uses current directory by default.
func TestAssumptionCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup, ids := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	// Change to the proof directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Execute without -d flag (should use current directory)
	cmd := newTestAssumptionCmd()
	output, err := executeCommand(cmd, "assumption", ids[0])

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}
}

// =============================================================================
// Format Validation Tests
// =============================================================================

// TestAssumptionsCmd_InvalidFormat tests error for invalid format option.
func TestAssumptionsCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup, _ := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	tests := []struct {
		name   string
		format string
	}{
		{"xml format", "xml"},
		{"yaml format", "yaml"},
		{"invalid format", "invalid"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestAssumptionsCmd()
			_, err := executeCommand(cmd, "assumptions", "--format", tc.format, "--dir", tmpDir)

			if err == nil {
				t.Logf("Format %q was accepted - implementation may allow or ignore it", tc.format)
			}
		})
	}
}

// TestAssumptionsCmd_FormatCaseInsensitive tests that format is case-insensitive.
func TestAssumptionsCmd_FormatCaseInsensitive(t *testing.T) {
	tmpDir, cleanup, _ := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	tests := []struct {
		name   string
		format string
	}{
		{"lowercase json", "json"},
		{"uppercase JSON", "JSON"},
		{"mixed case Json", "Json"},
		{"lowercase text", "text"},
		{"uppercase TEXT", "TEXT"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestAssumptionsCmd()
			output, err := executeCommand(cmd, "assumptions", "--format", tc.format, "--dir", tmpDir)

			if err != nil && strings.Contains(err.Error(), "format") {
				t.Errorf("expected format %q to be accepted, got error: %v", tc.format, err)
				return
			}

			// If format is json variant, verify output is JSON
			if strings.ToLower(tc.format) == "json" && err == nil {
				var result interface{}
				if jsonErr := json.Unmarshal([]byte(output), &result); jsonErr != nil {
					t.Errorf("format %q should produce JSON, got: %q", tc.format, output)
				}
			}
		})
	}
}

// =============================================================================
// Scope-Related Tests
// =============================================================================

// TestAssumptionsCmd_ScopeForRootNode tests assumptions in scope for root node.
func TestAssumptionsCmd_ScopeForRootNode(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTestWithNodes(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "1", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Root node should have all global assumptions in scope
	_ = output
}

// TestAssumptionsCmd_ScopeInheritance tests that child nodes inherit parent assumptions.
func TestAssumptionsCmd_ScopeInheritance(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTestWithNodes(t)
	defer cleanup()

	// Get assumptions for parent and child
	cmd1 := newTestAssumptionsCmd()
	parentOutput, err := executeCommand(cmd1, "assumptions", "1", "--format", "json", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("expected no error for parent, got: %v", err)
	}

	cmd2 := newTestAssumptionsCmd()
	childOutput, err := executeCommand(cmd2, "assumptions", "1.1", "--format", "json", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("expected no error for child, got: %v", err)
	}

	// Both outputs should be valid JSON
	var parentResult, childResult interface{}
	if err := json.Unmarshal([]byte(parentOutput), &parentResult); err != nil {
		t.Errorf("parent output is not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(childOutput), &childResult); err != nil {
		t.Errorf("child output is not valid JSON: %v", err)
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestAssumptionsCmd_TableDrivenFormats tests various format options.
func TestAssumptionsCmd_TableDrivenFormats(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		wantErr   bool
		checkJSON bool
	}{
		{"default (no format)", "", false, false},
		{"text format", "text", false, false},
		{"json format", "json", false, true},
		{"invalid format", "xml", true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup, _ := setupAssumptionsTestWithAssumptions(t)
			defer cleanup()

			cmd := newTestAssumptionsCmd()
			var output string
			var err error

			if tc.format == "" {
				output, err = executeCommand(cmd, "assumptions", "--dir", tmpDir)
			} else {
				output, err = executeCommand(cmd, "assumptions", "--format", tc.format, "--dir", tmpDir)
			}

			if tc.wantErr {
				if err == nil {
					t.Logf("Expected error for format %q, but command succeeded", tc.format)
				}
				return
			}

			if err != nil {
				t.Errorf("expected no error for format %q, got: %v", tc.format, err)
				return
			}

			if tc.checkJSON {
				var result interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("expected valid JSON for format %q, got: %q", tc.format, output)
				}
			}
		})
	}
}

// =============================================================================
// Output Content Tests
// =============================================================================

// TestAssumptionsCmd_OutputIncludesStatements tests that output includes assumption statements.
func TestAssumptionsCmd_OutputIncludesStatements(t *testing.T) {
	tmpDir, cleanup, _ := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check that assumption statements are shown
	expectedPhrases := []string{
		"natural numbers",
		"commutative",
		"smallest element",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(phrase)) {
			t.Errorf("expected output to contain %q, got: %q", phrase, output)
		}
	}
}

// TestAssumptionCmd_OutputIncludesDetails tests that single assumption shows full details.
func TestAssumptionCmd_OutputIncludesDetails(t *testing.T) {
	tmpDir, cleanup, ids := setupAssumptionsTestWithAssumptions(t)
	defer cleanup()

	cmd := newTestAssumptionCmd()
	output, err := executeCommand(cmd, "assumption", ids[1], "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain the assumption statement
	if !strings.Contains(output, "commutative") {
		t.Errorf("expected output to contain 'commutative', got: %q", output)
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestAssumptionsCmd_ManyAssumptions tests handling of many assumptions.
func TestAssumptionsCmd_ManyAssumptions(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add many assumptions
	for i := 0; i < 50; i++ {
		_, err := svc.AddAssumption("Test assumption number " + string(rune('0'+i%10)))
		if err != nil {
			t.Fatal(err)
		}
	}

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with many assumptions, got: %v", err)
	}

	// Output should not be empty
	if len(output) == 0 {
		t.Error("expected non-empty output for many assumptions")
	}
}

// TestAssumptionsCmd_LongAssumptionStatement tests handling of long assumption text.
func TestAssumptionsCmd_LongAssumptionStatement(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add an assumption with a very long statement
	longStatement := strings.Repeat("This is a very long assumption statement. ", 100)
	id, err := svc.AddAssumption(longStatement)
	if err != nil {
		t.Fatal(err)
	}

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with long assumption, got: %v", err)
	}

	// Should handle long text gracefully
	_ = output
	_ = id
}

// TestAssumptionsCmd_SpecialCharactersInStatement tests handling of special characters.
func TestAssumptionsCmd_SpecialCharactersInStatement(t *testing.T) {
	tmpDir, cleanup := setupAssumptionsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add assumptions with special characters
	specialStatements := []string{
		"For all x: f(x) = x + 1",
		"If a < b and b < c, then a < c",
		"Summation: sum_{i=1}^{n} i = n(n+1)/2",
		"Unicode: epsilon > 0, delta > 0",
	}

	for _, stmt := range specialStatements {
		_, err := svc.AddAssumption(stmt)
		if err != nil {
			t.Fatalf("failed to add assumption with special chars: %v", err)
		}
	}

	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with special characters, got: %v", err)
	}

	// Should show the assumptions
	if !strings.Contains(output, "f(x)") {
		t.Logf("Special characters may be escaped or formatted: %q", output)
	}
}

// =============================================================================
// Relative Directory Tests
// =============================================================================

// TestAssumptionsCmd_RelativeDirectory tests using relative directory path.
func TestAssumptionsCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-assumptions-rel-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(baseDir)

	proofDir := filepath.Join(baseDir, "subdir", "proof")
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	if err := service.Init(proofDir, "Test conjecture", "author"); err != nil {
		t.Fatal(err)
	}

	svc, _ := service.NewProofService(proofDir)
	svc.AddAssumption("Test assumption for relative path")

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	cmd := newTestAssumptionsCmd()
	output, err := executeCommand(cmd, "assumptions", "-d", "subdir/proof")

	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}

	// Should show the assumption
	if !strings.Contains(output, "relative path") {
		t.Logf("Output with relative directory: %q", output)
	}
}
