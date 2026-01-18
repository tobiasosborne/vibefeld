//go:build integration

// Package main contains tests for the af pending-defs and af pending-def commands.
// These are TDD tests - the pending-defs/pending-def commands do not exist yet.
// Tests define the expected behavior for listing and viewing pending definition requests.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupPendingDefsTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupPendingDefsTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-pending-defs-test-*")
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
	if err := service.Init(tmpDir, "Test conjecture for pending definitions", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupPendingDefsTestWithPendingDefs creates a test environment with an initialized proof
// and some pre-existing pending definition requests.
func setupPendingDefsTestWithPendingDefs(t *testing.T) (string, func(), []string) {
	t.Helper()

	tmpDir, cleanup := setupPendingDefsTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create additional nodes for testing (root node "1" is already created by Init)
	for _, idStr := range []string{"1.1", "1.2"} {
		nodeID, _ := types.Parse(idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement "+idStr, schema.InferenceModusPonens)
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

	var pendingDefIDs []string
	for _, pd := range pendingDefs {
		nodeID, _ := types.Parse(pd.nodeID)
		pendingDef, err := node.NewPendingDefWithValidation(pd.term, nodeID)
		if err != nil {
			cleanup()
			t.Fatalf("failed to create pending def for %q: %v", pd.term, err)
		}
		if err := fs.WritePendingDef(tmpDir, nodeID, pendingDef); err != nil {
			cleanup()
			t.Fatalf("failed to write pending def for %q: %v", pd.term, err)
		}
		pendingDefIDs = append(pendingDefIDs, pendingDef.ID)
	}

	return tmpDir, cleanup, pendingDefIDs
}

// newTestPendingDefsCmd creates a fresh root command with the pending-defs subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestPendingDefsCmd() *cobra.Command {
	cmd := newTestRootCmd()

	pendingDefsCmd := newPendingDefsCmd()
	cmd.AddCommand(pendingDefsCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// newTestPendingDefCmd creates a fresh root command with the pending-def subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestPendingDefCmd() *cobra.Command {
	cmd := newTestRootCmd()

	pendingDefCmd := newPendingDefCmd()
	cmd.AddCommand(pendingDefCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// executePendingDefsCommand creates and executes a pending-defs command with the given arguments.
func executePendingDefsCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newPendingDefsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// executePendingDefCommand creates and executes a pending-def command with the given arguments.
func executePendingDefCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newPendingDefCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// af pending-defs - List All Pending Definitions Tests
// =============================================================================

// TestPendingDefsCmd_ListPendingDefs tests listing all pending definitions in a proof.
func TestPendingDefsCmd_ListPendingDefs(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain all pending definition terms
	expectedTerms := []string{"group", "homomorphism", "kernel"}
	for _, term := range expectedTerms {
		if !strings.Contains(output, term) {
			t.Errorf("expected output to contain pending def term %q, got: %q", term, output)
		}
	}
}

// TestPendingDefsCmd_ListPendingDefsEmpty tests listing pending defs when none exist.
func TestPendingDefsCmd_ListPendingDefsEmpty(t *testing.T) {
	tmpDir, cleanup := setupPendingDefsTest(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should indicate no pending definitions or be empty/show a message
	lower := strings.ToLower(output)
	hasNoPendingDefsIndicator := strings.Contains(lower, "no pending") ||
		strings.Contains(lower, "none") ||
		strings.Contains(lower, "empty") ||
		strings.Contains(lower, "0") ||
		len(strings.TrimSpace(output)) == 0

	if !hasNoPendingDefsIndicator {
		t.Logf("Output when no pending defs exist: %q", output)
	}
}

// TestPendingDefsCmd_ListShowsCount tests that the listing shows a count of pending definitions.
func TestPendingDefsCmd_ListShowsCount(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain some indication of count (3 pending defs)
	if !strings.Contains(output, "3") {
		t.Logf("Output may or may not show count: %q", output)
	}
}

// TestPendingDefsCmd_ListShowsNodeIDs tests that the listing shows requesting node IDs.
func TestPendingDefsCmd_ListShowsNodeIDs(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show the node IDs that requested definitions
	expectedNodeIDs := []string{"1", "1.1", "1.2"}
	foundAny := false
	for _, nodeID := range expectedNodeIDs {
		if strings.Contains(output, nodeID) {
			foundAny = true
			break
		}
	}

	if !foundAny {
		t.Logf("Output may not show requesting node IDs: %q", output)
	}
}

// TestPendingDefsCmd_ListShowsStatus tests that the listing shows pending status.
func TestPendingDefsCmd_ListShowsStatus(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show the status (pending)
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "pending") {
		t.Logf("Output may not explicitly show 'pending' status: %q", output)
	}
}

// =============================================================================
// af pending-defs - JSON Output Tests
// =============================================================================

// TestPendingDefsCmd_JSONOutput tests JSON output format for listing pending definitions.
func TestPendingDefsCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestPendingDefsCmd_JSONOutputStructure tests the structure of JSON output.
func TestPendingDefsCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Try to unmarshal as array or object
	var arrayResult []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &arrayResult); err == nil {
		// It's an array - each item should have pending def fields
		if len(arrayResult) != 3 {
			t.Errorf("expected 3 pending defs in JSON array, got %d", len(arrayResult))
		}

		for i, pd := range arrayResult {
			if _, ok := pd["term"]; !ok {
				t.Errorf("pending def %d missing 'term' field", i)
			}
		}
	} else {
		// Try as object with pending_defs array
		var objResult map[string]interface{}
		if err := json.Unmarshal([]byte(output), &objResult); err != nil {
			t.Errorf("output is not valid JSON array or object: %v", err)
		} else {
			// Check for a pending_defs array in the object
			if pds, ok := objResult["pending_defs"]; ok {
				if pdsArr, ok := pds.([]interface{}); ok {
					if len(pdsArr) != 3 {
						t.Errorf("expected 3 pending defs, got %d", len(pdsArr))
					}
				}
			}
		}
	}
}

// TestPendingDefsCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestPendingDefsCmd_JSONOutputShortFlag(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "-f", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON with -f flag: %v\nOutput: %q", err, output)
	}
}

// TestPendingDefsCmd_JSONOutputEmpty tests JSON output when no pending defs exist.
func TestPendingDefsCmd_JSONOutputEmpty(t *testing.T) {
	tmpDir, cleanup := setupPendingDefsTest(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON (empty array or object with empty pending_defs)
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("empty output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestPendingDefsCmd_JSONOutputFields tests JSON output contains expected fields.
func TestPendingDefsCmd_JSONOutputFields(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var arrayResult []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &arrayResult); err == nil {
		if len(arrayResult) > 0 {
			pd := arrayResult[0]
			// Check for expected fields
			expectedFields := []string{"id", "term", "requested_by", "status"}
			for _, field := range expectedFields {
				if _, ok := pd[field]; !ok {
					// Try camelCase variant
					camelField := toCamelCase(field)
					if _, ok := pd[camelField]; !ok {
						t.Logf("Warning: JSON output does not contain field %q or %q", field, camelField)
					}
				}
			}
		}
	}
}

// toCamelCase converts snake_case to camelCase.
func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// =============================================================================
// af pending-def <name/id> - Show Specific Pending Definition Tests
// =============================================================================

// TestPendingDefCmd_ShowByTerm tests showing a specific pending def by term.
func TestPendingDefCmd_ShowByTerm(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "group", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain the term
	if !strings.Contains(output, "group") {
		t.Errorf("expected output to contain term 'group', got: %q", output)
	}

	// Should show it was requested by node 1
	if !strings.Contains(output, "1") {
		t.Logf("Output may not show requesting node: %q", output)
	}
}

// TestPendingDefCmd_ShowByNodeID tests showing a pending def by node ID.
func TestPendingDefCmd_ShowByNodeID(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	// Should be able to look up by node ID
	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "1.1", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show the pending def for node 1.1 (homomorphism)
	if !strings.Contains(output, "homomorphism") {
		t.Errorf("expected output to contain 'homomorphism', got: %q", output)
	}
}

// TestPendingDefCmd_ShowByID tests showing a pending def by its ID.
func TestPendingDefCmd_ShowByID(t *testing.T) {
	tmpDir, cleanup, pendingDefIDs := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	// Use the first pending def ID
	pendingDefID := pendingDefIDs[0]

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", pendingDefID, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show the pending def details
	if !strings.Contains(output, "group") {
		t.Errorf("expected output to contain term 'group', got: %q", output)
	}
}

// TestPendingDefCmd_ShowByPartialID tests showing pending def by partial ID.
func TestPendingDefCmd_ShowByPartialID(t *testing.T) {
	tmpDir, cleanup, pendingDefIDs := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	// Use first 6 characters of the first pending def ID
	partialID := pendingDefIDs[0][:6]

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", partialID, "--dir", tmpDir)

	// Should either find the pending def or provide helpful error
	if err != nil {
		// Partial matching may not be supported - that's acceptable
		t.Logf("Partial ID matching returned error: %v", err)
		return
	}

	if !strings.Contains(output, "group") {
		t.Logf("Output with partial ID: %q", output)
	}
}

// TestPendingDefCmd_ShowDifferentPendingDefs tests showing different pending definitions.
func TestPendingDefCmd_ShowDifferentPendingDefs(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	tests := []struct {
		lookup          string
		expectedContent string
	}{
		{"group", "group"},
		{"homomorphism", "homomorphism"},
		{"kernel", "kernel"},
		{"1", "group"},
		{"1.1", "homomorphism"},
		{"1.2", "kernel"},
	}

	for _, tc := range tests {
		t.Run(tc.lookup, func(t *testing.T) {
			cmd := newTestPendingDefCmd()
			output, err := executeCommand(cmd, "pending-def", tc.lookup, "--dir", tmpDir)

			if err != nil {
				t.Fatalf("expected no error for %q, got: %v", tc.lookup, err)
			}

			if !strings.Contains(output, tc.expectedContent) {
				t.Errorf("expected output for %q to contain %q, got: %q", tc.lookup, tc.expectedContent, output)
			}
		})
	}
}

// TestPendingDefCmd_ShowFull tests showing full pending def details.
func TestPendingDefCmd_ShowFull(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "group", "--full", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Full output should include:
	// - Term
	// - Requesting node ID
	// - Status
	// - ID
	// - Created timestamp (if shown)

	if !strings.Contains(output, "group") {
		t.Errorf("expected full output to contain term, got: %q", output)
	}
}

// =============================================================================
// af pending-def <name/id> - JSON Output Tests
// =============================================================================

// TestPendingDefCmd_JSONOutput tests JSON output for a specific pending definition.
func TestPendingDefCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "group", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Check for expected fields
	if _, ok := result["term"]; !ok {
		t.Error("JSON output missing 'term' field")
	}
}

// TestPendingDefCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestPendingDefCmd_JSONOutputShortFlag(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "homomorphism", "-f", "json", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestPendingDefCmd_JSONOutputFields tests JSON output contains expected fields.
func TestPendingDefCmd_JSONOutputFields(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "group", "--format", "json", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check for expected fields (with flexible naming conventions)
	expectedFields := []struct {
		name       string
		variations []string
	}{
		{"id", []string{"id", "ID"}},
		{"term", []string{"term", "Term"}},
		{"requested_by", []string{"requested_by", "requestedBy", "RequestedBy", "node_id", "nodeId"}},
		{"status", []string{"status", "Status"}},
	}

	for _, ef := range expectedFields {
		found := false
		for _, variant := range ef.variations {
			if _, ok := result[variant]; ok {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warning: JSON output does not contain %q field (checked: %v)", ef.name, ef.variations)
		}
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestPendingDefsCmd_ProofNotInitialized tests error when proof is not initialized.
func TestPendingDefsCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-pending-defs-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestPendingDefsCmd()
	_, err = executeCommand(cmd, "pending-defs", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestPendingDefsCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestPendingDefsCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestPendingDefsCmd()
	_, err := executeCommand(cmd, "pending-defs", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestPendingDefCmd_NotFound tests error when pending def doesn't exist.
func TestPendingDefCmd_NotFound(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "nonexistent", "--dir", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should indicate pending def not found
	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "not found") && !strings.Contains(lower, "does not exist") && err == nil {
		t.Errorf("expected error for non-existent pending def, got: %q", output)
	}
}

// TestPendingDefCmd_MissingArgument tests error when no argument is provided.
func TestPendingDefCmd_MissingArgument(t *testing.T) {
	tmpDir, cleanup := setupPendingDefsTest(t)
	defer cleanup()

	cmd := newTestPendingDefCmd()
	_, err := executeCommand(cmd, "pending-def", "--dir", tmpDir)

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

// TestPendingDefCmd_ProofNotInitialized tests error when proof is not initialized.
func TestPendingDefCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-pending-def-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestPendingDefCmd()
	_, err = executeCommand(cmd, "pending-def", "group", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestPendingDefCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestPendingDefCmd_DirectoryNotFound(t *testing.T) {
	cmd := newTestPendingDefCmd()
	_, err := executeCommand(cmd, "pending-def", "group", "--dir", "/nonexistent/path/12345")

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestPendingDefCmd_EmptyArgument tests error for empty argument.
func TestPendingDefCmd_EmptyArgument(t *testing.T) {
	tmpDir, cleanup := setupPendingDefsTest(t)
	defer cleanup()

	cmd := newTestPendingDefCmd()
	_, err := executeCommand(cmd, "pending-def", "", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for empty argument, got nil")
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestPendingDefsCmd_Help tests that help output shows usage information.
func TestPendingDefsCmd_Help(t *testing.T) {
	cmd := newPendingDefsCmd()
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
		"pending-defs",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestPendingDefsCmd_HelpShortFlag tests help with -h short flag.
func TestPendingDefsCmd_HelpShortFlag(t *testing.T) {
	cmd := newPendingDefsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "pending-defs") {
		t.Errorf("help output should mention 'pending-defs', got: %q", output)
	}
}

// TestPendingDefCmd_Help tests that help output shows usage information.
func TestPendingDefCmd_Help(t *testing.T) {
	cmd := newPendingDefCmd()
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
		"pending-def",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestPendingDefCmd_HelpShortFlag tests help with -h short flag.
func TestPendingDefCmd_HelpShortFlag(t *testing.T) {
	cmd := newPendingDefCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "pending-def") {
		t.Errorf("help output should mention 'pending-def', got: %q", output)
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestPendingDefsCmd_ExpectedFlags ensures the pending-defs command has expected flag structure.
func TestPendingDefsCmd_ExpectedFlags(t *testing.T) {
	cmd := newPendingDefsCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected pending-defs command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected pending-defs command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestPendingDefsCmd_DefaultFlagValues verifies default values for flags.
func TestPendingDefsCmd_DefaultFlagValues(t *testing.T) {
	cmd := newPendingDefsCmd()

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
}

// TestPendingDefCmd_ExpectedFlags ensures the pending-def command has expected flag structure.
func TestPendingDefCmd_ExpectedFlags(t *testing.T) {
	cmd := newPendingDefCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir", "full"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected pending-def command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
		"F": "full",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected pending-def command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestPendingDefCmd_DefaultFlagValues verifies default values for flags.
func TestPendingDefCmd_DefaultFlagValues(t *testing.T) {
	cmd := newPendingDefCmd()

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

	// Check default full value
	fullFlag := cmd.Flags().Lookup("full")
	if fullFlag == nil {
		t.Fatal("expected full flag to exist")
	}
	if fullFlag.DefValue != "false" {
		t.Errorf("expected default full to be 'false', got %q", fullFlag.DefValue)
	}
}

// TestPendingDefsCmd_CommandMetadata verifies command metadata.
func TestPendingDefsCmd_CommandMetadata(t *testing.T) {
	cmd := newPendingDefsCmd()

	if cmd.Use != "pending-defs" {
		t.Errorf("expected Use to be 'pending-defs', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestPendingDefCmd_CommandMetadata verifies command metadata.
func TestPendingDefCmd_CommandMetadata(t *testing.T) {
	cmd := newPendingDefCmd()

	if !strings.HasPrefix(cmd.Use, "pending-def") {
		t.Errorf("expected Use to start with 'pending-def', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Default Directory Tests
// =============================================================================

// TestPendingDefsCmd_DefaultDirectory tests pending-defs uses current directory by default.
func TestPendingDefsCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
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
	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs")

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Should list pending defs
	if !strings.Contains(output, "group") {
		t.Errorf("expected output to contain 'group', got: %q", output)
	}
}

// TestPendingDefCmd_DefaultDirectory tests pending-def uses current directory by default.
func TestPendingDefCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
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
	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "group")

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	// Should show pending def
	if !strings.Contains(output, "group") {
		t.Errorf("expected output to contain 'group', got: %q", output)
	}
}

// =============================================================================
// Format Validation Tests
// =============================================================================

// TestPendingDefsCmd_FormatValidation verifies format flag validation.
func TestPendingDefsCmd_FormatValidation(t *testing.T) {
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
			tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
			defer cleanup()

			cmd := newTestPendingDefsCmd()
			output, err := executeCommand(cmd, "pending-defs", "--format", tc.format, "--dir", tmpDir)

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

// TestPendingDefCmd_FormatValidation verifies format flag validation for pending-def command.
func TestPendingDefCmd_FormatValidation(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"valid text format", "text", false},
		{"valid json format", "json", false},
		{"invalid xml format", "xml", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
			defer cleanup()

			cmd := newTestPendingDefCmd()
			output, err := executeCommand(cmd, "pending-def", "group", "--format", tc.format, "--dir", tmpDir)

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
// Fuzzy Matching Tests
// =============================================================================

// TestPendingDefCmd_FuzzyMatchSuggestion tests fuzzy matching suggestions for typos.
func TestPendingDefCmd_FuzzyMatchSuggestion(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	// Try a typo that's close to "group"
	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "gropu", "--dir", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should either find via fuzzy match or suggest similar pending defs
	// This test documents the expected behavior for typos
	t.Logf("Typo 'gropu' result: %s (error: %v)", output, err)
}

// TestPendingDefCmd_CaseInsensitiveTerm tests if pending def lookup is case-insensitive.
func TestPendingDefCmd_CaseInsensitiveTerm(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	// Test with different cases of "group"
	testCases := []string{"group", "GROUP", "Group", "gROUP"}

	for _, term := range testCases {
		t.Run(term, func(t *testing.T) {
			cmd := newTestPendingDefCmd()
			output, err := executeCommand(cmd, "pending-def", term, "--dir", tmpDir)

			// Implementation may be case-sensitive or case-insensitive
			// This test documents the behavior
			if err != nil {
				t.Logf("Case %q returned error: %v (may be case-sensitive)", term, err)
			} else {
				if !strings.Contains(strings.ToLower(output), "group") {
					t.Errorf("expected output to contain 'group', got: %q", output)
				}
			}
		})
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestPendingDefsCmd_TableDrivenDirectories tests various directory scenarios.
func TestPendingDefsCmd_TableDrivenDirectories(t *testing.T) {
	tests := []struct {
		name    string
		dirPath string
		wantErr bool
	}{
		{
			name:    "empty path",
			dirPath: "",
			wantErr: true,
		},
		{
			name:    "nonexistent path",
			dirPath: "/nonexistent/path/12345",
			wantErr: true,
		},
		{
			name:    "path with special chars",
			dirPath: "/tmp/path with spaces",
			wantErr: true, // Likely doesn't exist
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestPendingDefsCmd()
			_, err := executeCommand(cmd, "pending-defs", "--dir", tc.dirPath)

			if tc.wantErr && err == nil {
				t.Errorf("expected error for dir %q, got nil", tc.dirPath)
			}
		})
	}
}

// TestPendingDefCmd_TableDrivenLookups tests various lookup inputs.
func TestPendingDefCmd_TableDrivenLookups(t *testing.T) {
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
		{"existing node ID 3", "1.2", true},
		{"nonexistent term", "nonexistent", false},
		{"nonexistent node ID", "1.999", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
			defer cleanup()

			cmd := newTestPendingDefCmd()
			output, err := executeCommand(cmd, "pending-def", tc.lookup, "--dir", tmpDir)

			if tc.expectFound {
				if err != nil {
					t.Errorf("expected pending def %q to be found, got error: %v", tc.lookup, err)
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

// =============================================================================
// Edge Cases
// =============================================================================

// TestPendingDefsCmd_ManyPendingDefs tests listing many pending definitions.
func TestPendingDefsCmd_ManyPendingDefs(t *testing.T) {
	tmpDir, cleanup := setupPendingDefsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create many nodes and pending defs
	for i := 0; i < 20; i++ {
		nodeIDStr := "1." + string(rune('1'+i%9))
		nodeID, err := types.Parse(nodeIDStr)
		if err != nil {
			continue
		}

		// Create node if it doesn't exist
		_ = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement "+nodeIDStr, schema.InferenceModusPonens)

		// Add pending def
		term := "term_" + string(rune('a'+i%26))
		pd, err := node.NewPendingDefWithValidation(term, nodeID)
		if err != nil {
			continue
		}
		_ = fs.WritePendingDef(tmpDir, nodeID, pd)
	}

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error listing many pending defs, got: %v", err)
	}

	t.Logf("Output length for many pending defs: %d bytes", len(output))
}

// TestPendingDefCmd_LongTerm tests showing a pending def with a long term.
func TestPendingDefCmd_LongTerm(t *testing.T) {
	tmpDir, cleanup := setupPendingDefsTest(t)
	defer cleanup()

	// Create a pending def with a long term
	longTerm := strings.Repeat("mathematical_concept_", 10)
	nodeID, _ := types.Parse("1")
	pd, err := node.NewPendingDefWithValidation(longTerm, nodeID)
	if err != nil {
		t.Logf("Could not create long-term pending def: %v", err)
		return
	}
	if err := fs.WritePendingDef(tmpDir, nodeID, pd); err != nil {
		t.Fatal(err)
	}

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", longTerm, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for long term, got: %v", err)
	}

	if !strings.Contains(output, "mathematical_concept_") {
		t.Errorf("expected output to contain long term, got: %q", output)
	}
}

// TestPendingDefCmd_SpecialCharactersInTerm tests pending def with special characters in term.
func TestPendingDefCmd_SpecialCharactersInTerm(t *testing.T) {
	tmpDir, cleanup := setupPendingDefsTest(t)
	defer cleanup()

	// Create a pending def with special characters in term
	specialTerm := "epsilon-delta_continuity"
	nodeID, _ := types.Parse("1")
	pd, err := node.NewPendingDefWithValidation(specialTerm, nodeID)
	if err != nil {
		t.Logf("Could not create special-term pending def: %v", err)
		return
	}
	if err := fs.WritePendingDef(tmpDir, nodeID, pd); err != nil {
		t.Fatal(err)
	}

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", specialTerm, "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error for special characters in term, got: %v", err)
	}

	if !strings.Contains(output, "epsilon") {
		t.Errorf("expected output to contain special term, got: %q", output)
	}
}

// =============================================================================
// Consistency Tests
// =============================================================================

// TestPendingDefsAndPendingDefConsistency tests that pending-defs and pending-def show consistent information.
func TestPendingDefsAndPendingDefConsistency(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	// Get list from pending-defs
	pendingDefsCmd := newTestPendingDefsCmd()
	pendingDefsOutput, err := executeCommand(pendingDefsCmd, "pending-defs", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("pending-defs command failed: %v", err)
	}

	// Verify each pending def from list can be retrieved with pending-def
	pendingDefTerms := []string{"group", "homomorphism", "kernel"}
	for _, term := range pendingDefTerms {
		// Verify term appears in pending-defs output
		if !strings.Contains(pendingDefsOutput, term) {
			t.Errorf("pending def term %q not found in pending-defs output", term)
		}

		// Verify pending-def can retrieve it
		pendingDefCmd := newTestPendingDefCmd()
		pendingDefOutput, err := executeCommand(pendingDefCmd, "pending-def", term, "--dir", tmpDir)
		if err != nil {
			t.Errorf("pending-def command failed for %q: %v", term, err)
		}

		if !strings.Contains(pendingDefOutput, term) {
			t.Errorf("pending-def output for %q doesn't contain the term", term)
		}
	}
}

// =============================================================================
// Status Display Tests
// =============================================================================

// TestPendingDefsCmd_ShowsStatus tests that listing shows status information.
func TestPendingDefsCmd_ShowsStatus(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show status (all are pending)
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "pending") {
		t.Logf("Output may not show status: %q", output)
	}
}

// TestPendingDefCmd_ShowsFullDetails tests that single pending def view shows full details.
func TestPendingDefCmd_ShowsFullDetails(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "group", "--full", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Full output should contain term and status
	lower := strings.ToLower(output)
	hasTermInfo := strings.Contains(output, "group")
	hasStatusInfo := strings.Contains(lower, "pending")

	if !hasTermInfo {
		t.Errorf("expected full output to contain term, got: %q", output)
	}

	if !hasStatusInfo {
		t.Logf("Full output may not explicitly show 'pending' status: %q", output)
	}
}

// =============================================================================
// Integration with request-def Tests
// =============================================================================

// TestPendingDefsCmd_AfterRequestDef tests listing pending defs after creating one via request-def.
func TestPendingDefsCmd_AfterRequestDef(t *testing.T) {
	tmpDir, cleanup := setupPendingDefsTest(t)
	defer cleanup()

	// Create a pending def using the low-level API (simulating request-def)
	nodeID, _ := types.Parse("1")
	pd, err := node.NewPendingDefWithValidation("test_term", nodeID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.WritePendingDef(tmpDir, nodeID, pd); err != nil {
		t.Fatal(err)
	}

	// List pending defs and verify the new one appears
	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "test_term") {
		t.Errorf("expected output to contain newly created pending def 'test_term', got: %q", output)
	}
}

// TestPendingDefCmd_AfterRequestDef tests showing pending def details after creating one.
func TestPendingDefCmd_AfterRequestDef(t *testing.T) {
	tmpDir, cleanup := setupPendingDefsTest(t)
	defer cleanup()

	// Create a pending def using the low-level API (simulating request-def)
	nodeID, _ := types.Parse("1")
	pd, err := node.NewPendingDefWithValidation("integration_test_term", nodeID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.WritePendingDef(tmpDir, nodeID, pd); err != nil {
		t.Fatal(err)
	}

	// Show the pending def and verify details
	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "integration_test_term", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "integration_test_term") {
		t.Errorf("expected output to contain term 'integration_test_term', got: %q", output)
	}
}

// =============================================================================
// Node ID Lookup Tests
// =============================================================================

// TestPendingDefCmd_LookupByNodeIDWithMultiple tests lookup by node ID when multiple pending defs exist.
func TestPendingDefCmd_LookupByNodeIDWithMultiple(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	// Each node has exactly one pending def, so lookup by node ID should work
	nodeIDToTerm := map[string]string{
		"1":   "group",
		"1.1": "homomorphism",
		"1.2": "kernel",
	}

	for nodeID, expectedTerm := range nodeIDToTerm {
		t.Run("node_"+nodeID, func(t *testing.T) {
			cmd := newTestPendingDefCmd()
			output, err := executeCommand(cmd, "pending-def", nodeID, "--dir", tmpDir)

			if err != nil {
				t.Fatalf("expected no error for node ID %s, got: %v", nodeID, err)
			}

			if !strings.Contains(output, expectedTerm) {
				t.Errorf("expected output for node %s to contain %q, got: %q", nodeID, expectedTerm, output)
			}
		})
	}
}

// =============================================================================
// Output Content Tests
// =============================================================================

// TestPendingDefsCmd_OutputContainsAllInfo tests text output contains expected information.
func TestPendingDefsCmd_OutputContainsAllInfo(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefsCmd()
	output, err := executeCommand(cmd, "pending-defs", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain terms
	terms := []string{"group", "homomorphism", "kernel"}
	for _, term := range terms {
		if !strings.Contains(output, term) {
			t.Errorf("expected output to contain %q, got: %q", term, output)
		}
	}
}

// TestPendingDefCmd_OutputContainsAllDetails tests single pending def output contains all details.
func TestPendingDefCmd_OutputContainsAllDetails(t *testing.T) {
	tmpDir, cleanup, _ := setupPendingDefsTestWithPendingDefs(t)
	defer cleanup()

	cmd := newTestPendingDefCmd()
	output, err := executeCommand(cmd, "pending-def", "group", "--full", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Full output should contain all relevant details
	if !strings.Contains(output, "group") {
		t.Errorf("expected full output to contain term 'group', got: %q", output)
	}
}
