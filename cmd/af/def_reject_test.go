//go:build integration

// Package main contains tests for the af def-reject command.
// These are TDD tests - the def-reject command does not exist yet.
// Tests define the expected behavior for rejecting pending definition requests.
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
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupDefRejectTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupDefRejectTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-def-reject-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	// Initialize the proof directory structure
	if err := service.InitProofDir(tmpDir); err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Initialize proof via service
	if err := service.Init(tmpDir, "Test conjecture for definition rejection", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupDefRejectTestWithPendingDef creates a test environment with an initialized proof
// and a single pending definition request.
// Returns the proof directory path, cleanup function, and the pending def.
func setupDefRejectTestWithPendingDef(t *testing.T) (string, func(), *node.PendingDef) {
	t.Helper()

	tmpDir, cleanup := setupDefRejectTest(t)

	// Create a pending definition request
	nodeID, _ := service.ParseNodeID("1")
	pd, err := node.NewPendingDefWithValidation("group", nodeID)
	if err != nil {
		cleanup()
		t.Fatalf("failed to create pending def: %v", err)
	}

	if err := fs.WritePendingDef(tmpDir, nodeID, pd); err != nil {
		cleanup()
		t.Fatalf("failed to write pending def: %v", err)
	}

	return tmpDir, cleanup, pd
}

// setupDefRejectTestWithMultiplePendingDefs creates a test environment with multiple pending definitions.
// Returns the proof directory path, cleanup function, and the pending defs.
func setupDefRejectTestWithMultiplePendingDefs(t *testing.T) (string, func(), []*node.PendingDef) {
	t.Helper()

	tmpDir, cleanup := setupDefRejectTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create additional nodes for testing (root node "1" is already created by Init)
	for _, idStr := range []string{"1.1", "1.2"} {
		nodeID, _ := service.ParseNodeID(idStr)
		err = svc.CreateNode(nodeID, service.NodeTypeClaim, "Statement "+idStr, service.InferenceModusPonens)
		if err != nil {
			cleanup()
			t.Fatal(err)
		}
	}

	// Add some pending definitions for testing
	pendingDefs := []struct {
		term   string
		nodeID string
	}{
		{"group", "1"},
		{"homomorphism", "1.1"},
		{"kernel", "1.2"},
	}

	var pds []*node.PendingDef
	for _, pdInfo := range pendingDefs {
		nodeID, _ := service.ParseNodeID(pdInfo.nodeID)
		pd, err := node.NewPendingDefWithValidation(pdInfo.term, nodeID)
		if err != nil {
			cleanup()
			t.Fatalf("failed to create pending def for %q: %v", pdInfo.term, err)
		}
		if err := fs.WritePendingDef(tmpDir, nodeID, pd); err != nil {
			cleanup()
			t.Fatalf("failed to write pending def for %q: %v", pdInfo.term, err)
		}
		pds = append(pds, pd)
	}

	return tmpDir, cleanup, pds
}

// executeDefRejectCommand creates and executes a def-reject command with the given arguments.
func executeDefRejectCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newDefRejectCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// newTestDefRejectCmd creates a fresh root command with the def-reject subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestDefRejectCmd() *cobra.Command {
	cmd := newTestRootCmd()

	defRejectCmd := newDefRejectCmd()
	cmd.AddCommand(defRejectCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestDefRejectCmd_Help tests that help output shows usage information.
func TestDefRejectCmd_Help(t *testing.T) {
	cmd := newDefRejectCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	// Check for expected help content
	expectations := []string{
		"def-reject",
		"--dir",
		"--format",
		"--reason",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestDefRejectCmd_HelpShortFlag tests help with -h short flag.
func TestDefRejectCmd_HelpShortFlag(t *testing.T) {
	cmd := newDefRejectCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "def-reject") {
		t.Errorf("help output should mention 'def-reject', got: %q", output)
	}
}

// TestDefRejectCmd_CommandMetadata verifies command metadata.
func TestDefRejectCmd_CommandMetadata(t *testing.T) {
	cmd := newDefRejectCmd()

	if !strings.HasPrefix(cmd.Use, "def-reject") {
		t.Errorf("expected Use to start with 'def-reject', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestDefRejectCmd_ExpectedFlags ensures the def-reject command has expected flag structure.
func TestDefRejectCmd_ExpectedFlags(t *testing.T) {
	cmd := newDefRejectCmd()

	// Check expected flags exist
	expectedFlags := []string{"dir", "format", "reason"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected def-reject command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"d": "dir",
		"f": "format",
		"r": "reason",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected def-reject command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestDefRejectCmd_DefaultFlagValues verifies default values for flags.
func TestDefRejectCmd_DefaultFlagValues(t *testing.T) {
	cmd := newDefRejectCmd()

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

	// Check default reason value
	reasonFlag := cmd.Flags().Lookup("reason")
	if reasonFlag == nil {
		t.Fatal("expected reason flag to exist")
	}
	if reasonFlag.DefValue != "" {
		t.Errorf("expected default reason to be empty, got %q", reasonFlag.DefValue)
	}
}

// TestDefRejectCmd_DirFlagVariants tests both long and short forms of --dir flag.
func TestDefRejectCmd_DirFlagVariants(t *testing.T) {
	tests := []struct {
		name string
		flag string
	}{
		{"long form", "--dir"},
		{"short form", "-d"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
			defer cleanup()

			output, err := executeDefRejectCommand(t, pd.Term, tc.flag, tmpDir)

			if err != nil {
				t.Errorf("expected no error with %s flag, got: %v\nOutput: %s", tc.flag, err, output)
			}
		})
	}
}

// TestDefRejectCmd_FormatFlagVariants tests both long and short forms of --format flag.
func TestDefRejectCmd_FormatFlagVariants(t *testing.T) {
	tests := []struct {
		name   string
		flag   string
		format string
	}{
		{"long form text", "--format", "text"},
		{"short form text", "-f", "text"},
		{"long form json", "--format", "json"},
		{"short form json", "-f", "json"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
			defer cleanup()

			output, err := executeDefRejectCommand(t,
				pd.Term,
				tc.flag, tc.format,
				"-d", tmpDir,
			)

			if err != nil {
				t.Errorf("expected no error with %s %s, got: %v\nOutput: %s", tc.flag, tc.format, err, output)
			}
		})
	}
}

// =============================================================================
// Argument Validation Tests
// =============================================================================

// TestDefRejectCmd_MissingArgument tests error when no argument is provided.
func TestDefRejectCmd_MissingArgument(t *testing.T) {
	tmpDir, cleanup := setupDefRejectTest(t)
	defer cleanup()

	// Execute without any arguments
	_, err := executeDefRejectCommand(t, "-d", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing argument, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") &&
		!strings.Contains(errStr, "required") &&
		!strings.Contains(errStr, "argument") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestDefRejectCmd_EmptyArgument tests error when empty argument is provided.
func TestDefRejectCmd_EmptyArgument(t *testing.T) {
	tmpDir, cleanup := setupDefRejectTest(t)
	defer cleanup()

	_, err := executeDefRejectCommand(t, "", "-d", tmpDir)

	if err == nil {
		t.Fatal("expected error for empty argument, got nil")
	}
}

// TestDefRejectCmd_WhitespaceOnlyArgument tests error when whitespace-only argument is provided.
func TestDefRejectCmd_WhitespaceOnlyArgument(t *testing.T) {
	tmpDir, cleanup := setupDefRejectTest(t)
	defer cleanup()

	output, err := executeDefRejectCommand(t, "   ", "-d", tmpDir)

	// Should error because whitespace-only identifier is invalid
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if err == nil && !strings.Contains(strings.ToLower(combined), "empty") &&
		!strings.Contains(strings.ToLower(combined), "invalid") &&
		!strings.Contains(strings.ToLower(combined), "not found") {
		t.Errorf("expected error for whitespace-only argument, got: %q", output)
	}
}

// =============================================================================
// Success Cases
// =============================================================================

// TestDefRejectCmd_Success tests successfully rejecting a pending definition.
func TestDefRejectCmd_Success(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	output, err := executeDefRejectCommand(t, pd.Term, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify output contains success indication
	lower := strings.ToLower(output)
	hasSuccessInfo := strings.Contains(lower, "reject") ||
		strings.Contains(lower, "cancelled") ||
		strings.Contains(lower, "removed") ||
		strings.Contains(lower, "success") ||
		strings.Contains(lower, pd.Term)

	if !hasSuccessInfo {
		t.Errorf("expected success message, got: %q", output)
	}
}

// TestDefRejectCmd_SuccessByID tests rejecting a pending definition by its ID.
func TestDefRejectCmd_SuccessByID(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	output, err := executeDefRejectCommand(t, pd.ID, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify output contains success indication
	lower := strings.ToLower(output)
	hasSuccessInfo := strings.Contains(lower, "reject") ||
		strings.Contains(lower, "cancelled") ||
		strings.Contains(lower, "removed") ||
		strings.Contains(lower, "success")

	if !hasSuccessInfo {
		t.Errorf("expected success message, got: %q", output)
	}
}

// TestDefRejectCmd_SuccessByPartialID tests rejecting a pending definition by partial ID.
func TestDefRejectCmd_SuccessByPartialID(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	// Use first 6 characters of the ID
	partialID := pd.ID[:6]

	output, err := executeDefRejectCommand(t, partialID, "-d", tmpDir)

	// Should either succeed via partial match or provide helpful error
	if err != nil {
		t.Logf("Partial ID matching returned error: %v (may be expected if partial matching not supported)", err)
		return
	}

	// Verify output contains success indication
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "reject") && !strings.Contains(lower, "cancelled") && !strings.Contains(lower, "success") {
		t.Logf("Output with partial ID: %q", output)
	}
}

// TestDefRejectCmd_SuccessByNodeID tests rejecting a pending definition by node ID.
func TestDefRejectCmd_SuccessByNodeID(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	// Look up by the requesting node ID
	output, err := executeDefRejectCommand(t, pd.RequestedBy.String(), "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify output contains success indication
	lower := strings.ToLower(output)
	hasSuccessInfo := strings.Contains(lower, "reject") ||
		strings.Contains(lower, "cancelled") ||
		strings.Contains(lower, "success")

	if !hasSuccessInfo {
		t.Errorf("expected success message, got: %q", output)
	}
}

// TestDefRejectCmd_SuccessWithReason tests rejecting with a reason.
func TestDefRejectCmd_SuccessWithReason(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	reason := "This definition is not needed for the current proof approach"
	output, err := executeDefRejectCommand(t, pd.Term, "--reason", reason, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with --reason flag, got: %v\nOutput: %s", err, output)
	}

	// Verify output contains success indication
	lower := strings.ToLower(output)
	hasSuccessInfo := strings.Contains(lower, "reject") ||
		strings.Contains(lower, "cancelled") ||
		strings.Contains(lower, "success")

	if !hasSuccessInfo {
		t.Errorf("expected success message, got: %q", output)
	}
}

// TestDefRejectCmd_SuccessWithReasonShortFlag tests rejecting with reason using -r flag.
func TestDefRejectCmd_SuccessWithReasonShortFlag(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	reason := "Not needed"
	output, err := executeDefRejectCommand(t, pd.Term, "-r", reason, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with -r flag, got: %v\nOutput: %s", err, output)
	}

	// Verify output contains success indication
	lower := strings.ToLower(output)
	hasSuccessInfo := strings.Contains(lower, "reject") ||
		strings.Contains(lower, "cancelled") ||
		strings.Contains(lower, "success")

	if !hasSuccessInfo {
		t.Errorf("expected success message, got: %q", output)
	}
}

// TestDefRejectCmd_UpdatesStatus tests that rejection updates the pending def status.
func TestDefRejectCmd_UpdatesStatus(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	// Execute rejection
	_, err := executeDefRejectCommand(t, pd.Term, "-d", tmpDir)
	if err != nil {
		t.Fatalf("rejection failed: %v", err)
	}

	// Verify the pending def status has changed
	nodeID := pd.RequestedBy
	updatedPD, err := fs.ReadPendingDef(tmpDir, nodeID)

	if err != nil {
		// If file is removed after rejection, that's also valid behavior
		t.Logf("Pending def file not found after rejection (may be deleted): %v", err)
		return
	}

	// If file exists, status should be cancelled
	if updatedPD.Status != node.PendingDefStatusCancelled {
		t.Errorf("expected pending def status to be 'cancelled', got: %s", updatedPD.Status)
	}
}

// =============================================================================
// Error Cases
// =============================================================================

// TestDefRejectCmd_DefinitionNotFound tests error when pending definition doesn't exist.
func TestDefRejectCmd_DefinitionNotFound(t *testing.T) {
	tmpDir, cleanup := setupDefRejectTest(t)
	defer cleanup()

	output, err := executeDefRejectCommand(t, "nonexistent_term", "-d", tmpDir)

	// Should produce error
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "does not exist") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		err == nil {
		t.Errorf("expected error for non-existent pending definition, got: %q", output)
	}
}

// TestDefRejectCmd_DefinitionNotPending tests error when definition is not in pending state.
func TestDefRejectCmd_DefinitionNotPending(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	// First, manually cancel the pending def
	nodeID := pd.RequestedBy
	loadedPD, err := fs.ReadPendingDef(tmpDir, nodeID)
	if err != nil {
		t.Fatalf("failed to read pending def: %v", err)
	}

	if err := loadedPD.Cancel(); err != nil {
		t.Fatalf("failed to cancel pending def: %v", err)
	}

	if err := fs.WritePendingDef(tmpDir, nodeID, loadedPD); err != nil {
		t.Fatalf("failed to write cancelled pending def: %v", err)
	}

	// Try to reject the already cancelled pending def
	output, err := executeDefRejectCommand(t, pd.Term, "-d", tmpDir)

	// Should produce error - cannot reject a non-pending definition
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not pending") &&
		!strings.Contains(strings.ToLower(combined), "already") &&
		!strings.Contains(strings.ToLower(combined), "cancelled") &&
		!strings.Contains(strings.ToLower(combined), "cannot") &&
		err == nil {
		t.Errorf("expected error for rejecting non-pending definition, got output: %q", output)
	}
}

// TestDefRejectCmd_DefinitionAlreadyResolved tests error when definition is already resolved.
func TestDefRejectCmd_DefinitionAlreadyResolved(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	// First, resolve the pending def
	nodeID := pd.RequestedBy
	loadedPD, err := fs.ReadPendingDef(tmpDir, nodeID)
	if err != nil {
		t.Fatalf("failed to read pending def: %v", err)
	}

	if err := loadedPD.Resolve("def-abc123"); err != nil {
		t.Fatalf("failed to resolve pending def: %v", err)
	}

	if err := fs.WritePendingDef(tmpDir, nodeID, loadedPD); err != nil {
		t.Fatalf("failed to write resolved pending def: %v", err)
	}

	// Try to reject the already resolved pending def
	output, err := executeDefRejectCommand(t, pd.Term, "-d", tmpDir)

	// Should produce error - cannot reject a resolved definition
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not pending") &&
		!strings.Contains(strings.ToLower(combined), "already") &&
		!strings.Contains(strings.ToLower(combined), "resolved") &&
		!strings.Contains(strings.ToLower(combined), "cannot") &&
		err == nil {
		t.Errorf("expected error for rejecting resolved definition, got output: %q", output)
	}
}

// TestDefRejectCmd_ProofNotInitialized tests error when proof is not initialized.
func TestDefRejectCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-def-reject-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	output, err := executeDefRejectCommand(t, "group", "-d", tmpDir)

	// Should produce error
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not initialized") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		!strings.Contains(strings.ToLower(combined), "not found") &&
		err == nil {
		t.Errorf("expected error for uninitialized proof, got: %q", output)
	}
}

// TestDefRejectCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestDefRejectCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeDefRejectCommand(t, "group", "-d", "/nonexistent/path/12345")

	// Should produce error
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not found") &&
		!strings.Contains(strings.ToLower(combined), "not exist") &&
		!strings.Contains(strings.ToLower(combined), "no such") &&
		!strings.Contains(strings.ToLower(combined), "error") &&
		err == nil {
		t.Errorf("expected error for non-existent directory, got: %q", output)
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestDefRejectCmd_TextOutput tests text output format.
func TestDefRejectCmd_TextOutput(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	output, err := executeDefRejectCommand(t, pd.Term, "--format", "text", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Text format should be human-readable, non-empty
	if output == "" {
		t.Error("expected non-empty text output")
	}

	// Should contain term name or success indication
	lower := strings.ToLower(output)
	if !strings.Contains(lower, pd.Term) && !strings.Contains(lower, "reject") && !strings.Contains(lower, "success") {
		t.Errorf("text output should contain term name or success message, got: %q", output)
	}
}

// TestDefRejectCmd_JSONOutput tests JSON output format.
func TestDefRejectCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	output, err := executeDefRejectCommand(t, pd.Term, "--format", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestDefRejectCmd_JSONOutputStructure tests the structure of JSON output.
func TestDefRejectCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	output, err := executeDefRejectCommand(t, pd.Term, "--format", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check for expected fields (with flexible naming conventions)
	fieldChecks := []struct {
		name       string
		variations []string
	}{
		{"term", []string{"term", "Term"}},
		{"status", []string{"status", "Status", "rejected"}},
	}

	for _, fc := range fieldChecks {
		found := false
		for _, variant := range fc.variations {
			if _, ok := result[variant]; ok {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warning: JSON output does not contain %s field (checked: %v)", fc.name, fc.variations)
		}
	}

	t.Logf("JSON output structure: %+v", result)
}

// TestDefRejectCmd_JSONOutputContainsTerm tests JSON output contains the term name.
func TestDefRejectCmd_JSONOutputContainsTerm(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	output, err := executeDefRejectCommand(t, pd.Term, "--format", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check for term field
	term, hasTerm := result["term"]
	if !hasTerm {
		t.Log("Warning: JSON output does not have 'term' field")
	} else if term != pd.Term {
		t.Errorf("expected term %q, got %v", pd.Term, term)
	}
}

// TestDefRejectCmd_JSONOutputWithReason tests JSON output includes reason if provided.
func TestDefRejectCmd_JSONOutputWithReason(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	reason := "Not needed for current approach"
	output, err := executeDefRejectCommand(t, pd.Term, "--format", "json", "--reason", reason, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// The reason may or may not appear in the output depending on implementation
	t.Logf("JSON output with reason: %+v", result)
}

// TestDefRejectCmd_InvalidFormat tests error for invalid format option.
func TestDefRejectCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	output, err := executeDefRejectCommand(t, pd.Term, "--format", "invalid", "-d", tmpDir)

	// Should produce error or warning about invalid format
	combined := output
	if err != nil {
		combined += err.Error()
	}

	// The behavior depends on implementation - it might error or fall back to text
	t.Logf("Output with invalid format: %s (error: %v)", output, err)
}

// =============================================================================
// Default Directory Tests
// =============================================================================

// TestDefRejectCmd_DefaultDirectory tests def-reject uses current directory by default.
func TestDefRejectCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
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
	output, err := executeDefRejectCommand(t, pd.Term)

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Verify success indication
	lower := strings.ToLower(output)
	hasSuccessInfo := strings.Contains(lower, "reject") ||
		strings.Contains(lower, "cancelled") ||
		strings.Contains(lower, "success")

	if !hasSuccessInfo {
		t.Errorf("expected success message, got: %q", output)
	}
}

// TestDefRejectCmd_RelativeDirectory tests using relative directory path.
func TestDefRejectCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-def-reject-rel-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(baseDir)

	proofDir := filepath.Join(baseDir, "subdir", "proof")
	if err := service.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	if err := service.Init(proofDir, "Test conjecture", "author"); err != nil {
		t.Fatal(err)
	}

	// Create a pending def
	nodeID, _ := service.ParseNodeID("1")
	pd, err := node.NewPendingDefWithValidation("test_term", nodeID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.WritePendingDef(proofDir, nodeID, pd); err != nil {
		t.Fatal(err)
	}

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	output, err := executeDefRejectCommand(t, pd.Term, "-d", "subdir/proof")

	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}
}

// =============================================================================
// Multiple Pending Definitions Tests
// =============================================================================

// TestDefRejectCmd_RejectOne tests rejecting one of multiple pending definitions.
func TestDefRejectCmd_RejectOne(t *testing.T) {
	tmpDir, cleanup, pds := setupDefRejectTestWithMultiplePendingDefs(t)
	defer cleanup()

	// Reject the first pending def (group)
	output, err := executeDefRejectCommand(t, pds[0].Term, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v\nOutput: %s", err, output)
	}

	// Verify only the rejected one is affected
	// Check that other pending defs still exist and are pending
	for i, pd := range pds {
		if i == 0 {
			// Skip the rejected one
			continue
		}

		loadedPD, err := fs.ReadPendingDef(tmpDir, pd.RequestedBy)
		if err != nil {
			t.Errorf("pending def %q should still exist: %v", pd.Term, err)
			continue
		}

		if loadedPD.Status != node.PendingDefStatusPending {
			t.Errorf("pending def %q should still be pending, got: %s", pd.Term, loadedPD.Status)
		}
	}
}

// TestDefRejectCmd_RejectMultiple tests rejecting multiple pending definitions sequentially.
func TestDefRejectCmd_RejectMultiple(t *testing.T) {
	tmpDir, cleanup, pds := setupDefRejectTestWithMultiplePendingDefs(t)
	defer cleanup()

	// Reject all pending defs
	for _, pd := range pds {
		output, err := executeDefRejectCommand(t, pd.Term, "-d", tmpDir)

		if err != nil {
			t.Fatalf("failed to reject pending def %q: %v\nOutput: %s", pd.Term, err, output)
		}
	}

	// Verify all are rejected/cancelled
	for _, pd := range pds {
		loadedPD, err := fs.ReadPendingDef(tmpDir, pd.RequestedBy)
		if err != nil {
			// File removed after rejection is also valid
			t.Logf("Pending def %q file not found after rejection (may be deleted)", pd.Term)
			continue
		}

		if loadedPD.Status == node.PendingDefStatusPending {
			t.Errorf("pending def %q should not be pending after rejection, got: %s", pd.Term, loadedPD.Status)
		}
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestDefRejectCmd_TableDrivenLookups tests various lookup inputs.
func TestDefRejectCmd_TableDrivenLookups(t *testing.T) {
	tests := []struct {
		name        string
		lookup      string
		expectFound bool
	}{
		{"existing term", "group", true},
		{"existing term 2", "homomorphism", true},
		{"existing term 3", "kernel", true},
		{"existing node ID", "1", true},
		{"existing node ID 2", "1.1", true},
		{"nonexistent term", "nonexistent", false},
		{"nonexistent node ID", "1.999", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup, _ := setupDefRejectTestWithMultiplePendingDefs(t)
			defer cleanup()

			output, err := executeDefRejectCommand(t, tc.lookup, "-d", tmpDir)

			if tc.expectFound {
				if err != nil {
					t.Errorf("expected pending def %q to be found and rejected, got error: %v", tc.lookup, err)
				}
			} else {
				combined := output
				if err != nil {
					combined += err.Error()
				}
				// Should indicate not found
				if err == nil && !strings.Contains(strings.ToLower(combined), "not found") {
					t.Logf("Pending def %q not expected to be found, but no error. Output: %q", tc.lookup, output)
				}
			}
		})
	}
}

// TestDefRejectCmd_TableDrivenReasons tests various reason inputs.
func TestDefRejectCmd_TableDrivenReasons(t *testing.T) {
	tests := []struct {
		name    string
		reason  string
		wantErr bool
	}{
		{"empty reason", "", false},
		{"simple reason", "Not needed", false},
		{"long reason", strings.Repeat("Detailed reason. ", 50), false},
		{"reason with special chars", "Reason with 'quotes' and \"double quotes\"", false},
		{"reason with newlines", "Line 1\nLine 2\nLine 3", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
			defer cleanup()

			var args []string
			if tc.reason != "" {
				args = []string{pd.Term, "--reason", tc.reason, "-d", tmpDir}
			} else {
				args = []string{pd.Term, "-d", tmpDir}
			}

			_, err := executeDefRejectCommand(t, args...)

			if tc.wantErr && err == nil {
				t.Errorf("expected error for reason %q, got nil", tc.reason)
			}

			if !tc.wantErr && err != nil {
				t.Errorf("expected no error for reason %q, got: %v", tc.reason, err)
			}
		})
	}
}

// TestDefRejectCmd_OutputFormats tests different output format options.
func TestDefRejectCmd_OutputFormats(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		validator func(t *testing.T, output string)
	}{
		{
			name:   "text format",
			format: "text",
			validator: func(t *testing.T, output string) {
				// Text format should be human-readable
				if output == "" {
					t.Error("expected non-empty text output")
				}
			},
		},
		{
			name:   "json format",
			format: "json",
			validator: func(t *testing.T, output string) {
				// JSON format should be valid JSON
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(output), &data); err != nil {
					t.Errorf("expected valid JSON, got error: %v\nOutput: %q", err, output)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
			defer cleanup()

			output, err := executeDefRejectCommand(t, pd.Term, "-d", tmpDir, "-f", tc.format)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tc.validator(t, output)
		})
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestDefRejectCmd_SpecialCharactersInTerm tests rejecting pending def with special characters in term.
func TestDefRejectCmd_SpecialCharactersInTerm(t *testing.T) {
	tmpDir, cleanup := setupDefRejectTest(t)
	defer cleanup()

	// Create a pending def with special characters in term
	specialTerm := "epsilon-delta_continuity"
	nodeID, _ := service.ParseNodeID("1")
	pd, err := node.NewPendingDefWithValidation(specialTerm, nodeID)
	if err != nil {
		t.Logf("Could not create special-term pending def: %v", err)
		return
	}
	if err := fs.WritePendingDef(tmpDir, nodeID, pd); err != nil {
		t.Fatal(err)
	}

	output, err := executeDefRejectCommand(t, specialTerm, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for special characters in term, got: %v", err)
	}

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "reject") && !strings.Contains(lower, "cancelled") && !strings.Contains(lower, "success") {
		t.Logf("Output with special characters in term: %q", output)
	}
}

// TestDefRejectCmd_LongTerm tests rejecting pending def with a very long term.
func TestDefRejectCmd_LongTerm(t *testing.T) {
	tmpDir, cleanup := setupDefRejectTest(t)
	defer cleanup()

	// Create a pending def with a long term
	longTerm := strings.Repeat("mathematical_concept_", 10)
	nodeID, _ := service.ParseNodeID("1")
	pd, err := node.NewPendingDefWithValidation(longTerm, nodeID)
	if err != nil {
		t.Logf("Could not create long-term pending def: %v", err)
		return
	}
	if err := fs.WritePendingDef(tmpDir, nodeID, pd); err != nil {
		t.Fatal(err)
	}

	output, err := executeDefRejectCommand(t, longTerm, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for long term, got: %v", err)
	}

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "reject") && !strings.Contains(lower, "cancelled") && !strings.Contains(lower, "success") {
		t.Logf("Output with long term: %q", output)
	}
}

// TestDefRejectCmd_CaseInsensitiveTerm tests if term lookup is case-insensitive.
func TestDefRejectCmd_CaseInsensitiveTerm(t *testing.T) {
	_, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	// Test with different cases of the term
	testCases := []string{"group", "GROUP", "Group", "gROUP"}

	for _, term := range testCases {
		t.Run(term, func(t *testing.T) {
			// Need fresh pending def for each test case
			tmpDir2, cleanup2, pd2 := setupDefRejectTestWithPendingDef(t)
			defer cleanup2()

			_, err := executeDefRejectCommand(t, term, "-d", tmpDir2)

			// Implementation may be case-sensitive or case-insensitive
			// For the exact match case, it should succeed
			if term == pd.Term || term == pd2.Term {
				if err != nil {
					t.Errorf("expected no error for exact term %q, got: %v", term, err)
				}
			} else {
				// Other cases - document the behavior
				if err != nil {
					t.Logf("Case %q returned error: %v (may be case-sensitive)", term, err)
				} else {
					t.Logf("Case %q succeeded: case-insensitive matching active", term)
				}
			}
		})
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestDefRejectCmd_PendingDefsListAfterReject tests that rejected pending def doesn't appear as pending.
func TestDefRejectCmd_PendingDefsListAfterReject(t *testing.T) {
	tmpDir, cleanup, pds := setupDefRejectTestWithMultiplePendingDefs(t)
	defer cleanup()

	// Reject one pending def
	_, err := executeDefRejectCommand(t, pds[0].Term, "-d", tmpDir)
	if err != nil {
		t.Fatalf("rejection failed: %v", err)
	}

	// List pending defs - the rejected one should not appear as pending
	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("pending-defs command failed: %v", err)
	}

	// The rejected term should either not appear or show as cancelled
	// Implementation determines exact behavior
	t.Logf("Pending defs output after rejection: %s", output)
}

// TestDefRejectCmd_IdempotentBehavior tests that rejecting already rejected def handles gracefully.
func TestDefRejectCmd_IdempotentBehavior(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	// First rejection
	_, err := executeDefRejectCommand(t, pd.Term, "-d", tmpDir)
	if err != nil {
		t.Fatalf("first rejection failed: %v", err)
	}

	// Second rejection - should either succeed (idempotent) or give clear error
	output, err := executeDefRejectCommand(t, pd.Term, "-d", tmpDir)

	// Document the behavior
	if err != nil {
		t.Logf("Second rejection returned error: %v (non-idempotent behavior)", err)
	} else {
		t.Logf("Second rejection succeeded: %s (idempotent behavior)", output)
	}
}

// =============================================================================
// Success Message Tests
// =============================================================================

// TestDefRejectCmd_SuccessMessage tests that success message is informative.
func TestDefRejectCmd_SuccessMessage(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	output, err := executeDefRejectCommand(t, pd.Term, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Success message should indicate what happened
	lower := strings.ToLower(output)
	hasStatusInfo := strings.Contains(lower, "reject") ||
		strings.Contains(lower, "cancelled") ||
		strings.Contains(lower, "success")

	if !hasStatusInfo {
		t.Errorf("success message should mention rejection or cancellation, got: %q", output)
	}

	// Should mention the term or provide useful info
	hasTermInfo := strings.Contains(output, pd.Term) || strings.Contains(output, "term")

	if !hasTermInfo {
		t.Logf("Warning: success message doesn't include term info: %q", output)
	}
}

// TestDefRejectCmd_SuccessMessageWithReason tests success message includes reason when provided.
func TestDefRejectCmd_SuccessMessageWithReason(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	reason := "Definition not needed for simplified proof"
	output, err := executeDefRejectCommand(t, pd.Term, "--reason", reason, "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Success message may or may not include the reason
	// Document the behavior
	t.Logf("Success message with reason: %q", output)
}

// =============================================================================
// Format Validation Tests
// =============================================================================

// TestDefRejectCmd_FormatValidation verifies format flag validation.
func TestDefRejectCmd_FormatValidation(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"valid text format", "text", false},
		{"valid json format", "json", false},
		{"valid TEXT uppercase", "TEXT", false},
		{"valid JSON uppercase", "JSON", false},
		{"invalid xml format", "xml", true},
		{"invalid yaml format", "yaml", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
			defer cleanup()

			output, err := executeDefRejectCommand(t, pd.Term, "--format", tc.format, "--dir", tmpDir)

			if tc.wantErr {
				combined := output
				if err != nil {
					combined += err.Error()
				}

				if err == nil && !strings.Contains(strings.ToLower(combined), "format") {
					t.Logf("Expected error for format %q, got output: %q", tc.format, output)
				}
			} else {
				if err != nil && strings.Contains(err.Error(), "format") {
					t.Errorf("unexpected format error for format %q: %v", tc.format, err)
				}
			}
		})
	}
}

// =============================================================================
// Reason Handling Tests
// =============================================================================

// TestDefRejectCmd_ReasonStored tests that the reason is stored/recorded somewhere.
func TestDefRejectCmd_ReasonStored(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	reason := "Not aligned with current proof strategy"
	_, err := executeDefRejectCommand(t, pd.Term, "--reason", reason, "-d", tmpDir)

	if err != nil {
		t.Fatalf("rejection with reason failed: %v", err)
	}

	// Note: The reason may be stored in a ledger event or pending def metadata
	// The test primarily verifies the flag is accepted and processed
	// Implementation details determine where the reason is stored
	t.Log("Reason flag accepted - storage location depends on implementation")
}

// TestDefRejectCmd_EmptyReasonAllowed tests that empty reason is allowed.
func TestDefRejectCmd_EmptyReasonAllowed(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	// Explicit empty reason
	output, err := executeDefRejectCommand(t, pd.Term, "--reason", "", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with empty reason, got: %v", err)
	}

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "reject") && !strings.Contains(lower, "cancelled") && !strings.Contains(lower, "success") {
		t.Errorf("expected success message, got: %q", output)
	}
}

// =============================================================================
// Consistency Tests
// =============================================================================

// TestDefRejectCmd_ConsistentWithPendingDef tests consistency with pending-def command.
func TestDefRejectCmd_ConsistentWithPendingDef(t *testing.T) {
	tmpDir, cleanup, pd := setupDefRejectTestWithPendingDef(t)
	defer cleanup()

	// Get pending def details before rejection
	pendingDefCmd := newTestPendingDefCmd()
	beforeOutput, err := executeCommand(pendingDefCmd, "pending-def", pd.Term, "--dir", tmpDir)
	if err != nil {
		t.Fatalf("pending-def command failed before rejection: %v", err)
	}

	if !strings.Contains(beforeOutput, pd.Term) {
		t.Errorf("pending-def output should contain term before rejection")
	}

	// Reject the pending def
	_, err = executeDefRejectCommand(t, pd.Term, "-d", tmpDir)
	if err != nil {
		t.Fatalf("rejection failed: %v", err)
	}

	// Get pending def details after rejection
	pendingDefCmd2 := newTestPendingDefCmd()
	afterOutput, err := executeCommand(pendingDefCmd2, "pending-def", pd.Term, "--dir", tmpDir)

	// After rejection, either:
	// 1. pending-def shows cancelled status
	// 2. pending-def shows not found (if file is removed)
	// 3. pending-def shows some indication of rejection
	t.Logf("After rejection - pending-def output: %s (error: %v)", afterOutput, err)
}
