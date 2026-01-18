//go:build integration

// Package main contains tests for the af reap command.
// These are TDD tests - the reap command does not exist yet.
// Tests define the expected behavior for reaping stale/expired locks.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// mustParseReapNodeID parses a NodeID string or fails the test.
func mustParseReapNodeID(t *testing.T, s string) service.NodeID {
	t.Helper()
	id, err := service.ParseNodeID(s)
	if err != nil {
		t.Fatalf("Failed to parse NodeID %q: %v", s, err)
	}
	return id
}

// setupReapTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupReapTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-reap-test-*")
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
	if err := service.Init(tmpDir, "Test conjecture for reap", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupReapTestWithNode creates a test environment with an initialized proof
// and a single pending node at ID "1".
// Note: service.Init() already creates node 1 with the conjecture, so we just
// return the base setup.
func setupReapTestWithNode(t *testing.T) (string, func()) {
	t.Helper()
	// Init already creates node 1 with "Test conjecture for reap"
	return setupReapTest(t)
}

// setupReapTestWithClaim creates a test environment with an initialized proof
// and a claimed node at ID "1".
func setupReapTestWithClaim(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupReapTestWithNode(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	nodeID := mustParseReapNodeID(t, "1")
	// Claim with a 1-hour timeout
	err = svc.ClaimNode(nodeID, "test-prover", 1*time.Hour)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupReapTestWithExpiredClaim creates a test environment with a claimed node
// that has an expired lock (very short timeout in the past).
func setupReapTestWithExpiredClaim(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupReapTestWithNode(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	nodeID := mustParseReapNodeID(t, "1")
	// Claim with very short timeout (1 nanosecond) - this will be expired immediately
	err = svc.ClaimNode(nodeID, "expired-prover", 1*time.Nanosecond)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Wait to ensure the lock is expired
	time.Sleep(10 * time.Millisecond)

	return tmpDir, cleanup
}

// setupReapTestWithMultipleClaims creates a test environment with multiple claimed nodes.
func setupReapTestWithMultipleClaims(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupReapTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create multiple nodes
	for _, idStr := range []string{"1.1", "1.2", "1.3"} {
		nodeID := mustParseReapNodeID(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceModusPonens)
		if err != nil {
			cleanup()
			t.Fatal(err)
		}
	}

	// Claim nodes with different timeouts
	// 1 - not claimed (available)
	// 1.1 - claimed with expired timeout
	// 1.2 - claimed with active timeout
	// 1.3 - claimed with expired timeout

	nodeID11 := mustParseReapNodeID(t, "1.1")
	err = svc.ClaimNode(nodeID11, "expired-prover-1", 1*time.Nanosecond)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	nodeID12 := mustParseReapNodeID(t, "1.2")
	err = svc.ClaimNode(nodeID12, "active-prover", 1*time.Hour)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	nodeID13 := mustParseReapNodeID(t, "1.3")
	err = svc.ClaimNode(nodeID13, "expired-prover-2", 1*time.Nanosecond)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Wait to ensure short-timeout locks are expired
	time.Sleep(10 * time.Millisecond)

	return tmpDir, cleanup
}

// executeReapCommand creates and executes a reap command with the given arguments.
func executeReapCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newReapCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// newReapCmd is expected to be implemented in reap.go
// This test file uses the real implementation when available.

// =============================================================================
// Basic Command Tests
// =============================================================================

// TestReapCmd_Help tests that help output shows usage information.
func TestReapCmd_Help(t *testing.T) {
	cmd := newReapCmd()
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
		"reap",
		"--dir",
		"--format",
		"--dry-run",
		"--all",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestReapCmd_HelpShortFlag tests help with short flag.
func TestReapCmd_HelpShortFlag(t *testing.T) {
	cmd := newReapCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(strings.ToLower(output), "reap") {
		t.Errorf("help output should mention 'reap', got: %q", output)
	}
}

// TestReapCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestReapCmd_DirectoryNotFound(t *testing.T) {
	output, err := executeReapCommand(t, "-d", "/nonexistent/path/12345")

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

// TestReapCmd_ProofNotInitialized tests error when proof hasn't been initialized.
func TestReapCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-reap-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Execute reap on uninitialized directory
	output, err := executeReapCommand(t, "-d", tmpDir)

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

// TestReapCmd_NoStaleLocks tests behavior when there are no stale locks to reap.
func TestReapCmd_NoStaleLocks(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithNode(t)
	defer cleanup()

	// No claims made, so no locks to reap
	output, err := executeReapCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error when no locks exist, got: %v", err)
	}

	// Output should indicate nothing to reap
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "no") && !strings.Contains(lower, "0") && !strings.Contains(lower, "none") {
		t.Logf("Output when no stale locks: %q", output)
	}
}

// TestReapCmd_NoStaleLocks_ActiveClaimExists tests behavior when only active (non-expired) claims exist.
func TestReapCmd_NoStaleLocks_ActiveClaimExists(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithClaim(t)
	defer cleanup()

	// Claim exists but is not expired
	output, err := executeReapCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error when only active locks exist, got: %v", err)
	}

	// Output should indicate nothing to reap (active lock should not be reaped)
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "no") && !strings.Contains(lower, "0") && !strings.Contains(lower, "none") {
		t.Logf("Output when no stale locks (active claim exists): %q", output)
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestReapCmd_ExpectedFlags ensures the reap command has expected flag structure.
func TestReapCmd_ExpectedFlags(t *testing.T) {
	cmd := newReapCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir", "dry-run", "all"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected reap command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected reap command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestReapCmd_DefaultFlagValues verifies default values for flags.
func TestReapCmd_DefaultFlagValues(t *testing.T) {
	cmd := newReapCmd()

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

	// Check default dry-run value
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Fatal("expected dry-run flag to exist")
	}
	if dryRunFlag.DefValue != "false" {
		t.Errorf("expected default dry-run to be 'false', got %q", dryRunFlag.DefValue)
	}

	// Check default all value
	allFlag := cmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Fatal("expected all flag to exist")
	}
	if allFlag.DefValue != "false" {
		t.Errorf("expected default all to be 'false', got %q", allFlag.DefValue)
	}
}

// TestReapCmd_DirFlagShortForm tests -d short form.
func TestReapCmd_DirFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithNode(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -d flag, got: %v\nOutput: %s", err, output)
	}
}

// TestReapCmd_DirFlagLongForm tests --dir long form.
func TestReapCmd_DirFlagLongForm(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithNode(t)
	defer cleanup()

	output, err := executeReapCommand(t, "--dir", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with --dir flag, got: %v\nOutput: %s", err, output)
	}
}

// TestReapCmd_FormatFlagShortForm tests -f short form.
func TestReapCmd_FormatFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithNode(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error with -f flag, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON with -f json, got: %v\nOutput: %q", err, output)
	}
}

// TestReapCmd_CommandMetadata verifies command metadata.
func TestReapCmd_CommandMetadata(t *testing.T) {
	cmd := newReapCmd()

	if !strings.HasPrefix(cmd.Use, "reap") {
		t.Errorf("expected Use to start with 'reap', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Dry Run Tests
// =============================================================================

// TestReapCmd_DryRunShowsWhatWouldBeReaped tests --dry-run shows what would be reaped.
func TestReapCmd_DryRunShowsWhatWouldBeReaped(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithExpiredClaim(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "--dry-run")
	if err != nil {
		t.Fatalf("expected no error with --dry-run, got: %v", err)
	}

	// Output should indicate what would be reaped
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "would") || !strings.Contains(lower, "reap") {
		t.Logf("Dry-run output should indicate what would be reaped: %q", output)
	}

	// Should mention the node ID
	if !strings.Contains(output, "1") {
		t.Errorf("dry-run output should mention node ID '1', got: %q", output)
	}
}

// TestReapCmd_DryRunDoesNotActuallyReap tests --dry-run doesn't actually reap.
func TestReapCmd_DryRunDoesNotActuallyReap(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithExpiredClaim(t)
	defer cleanup()

	// Run with dry-run
	_, err := executeReapCommand(t, "-d", tmpDir, "--dry-run")
	if err != nil {
		t.Fatalf("expected no error with --dry-run, got: %v", err)
	}

	// Verify the claim still exists (wasn't actually reaped)
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseReapNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after dry-run")
	}

	// Node should still be in claimed state (dry-run shouldn't change state)
	if n.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("dry-run should not change workflow state, expected 'claimed', got: %s", n.WorkflowState)
	}
}

// TestReapCmd_DryRunWithJSONFormat tests --dry-run with JSON format.
func TestReapCmd_DryRunWithJSONFormat(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithExpiredClaim(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "--dry-run", "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("dry-run JSON output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// =============================================================================
// Reaping Tests
// =============================================================================

// TestReapCmd_ReapsExpiredLock tests that expired locks are reaped.
func TestReapCmd_ReapsExpiredLock(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithExpiredClaim(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should indicate something was reaped
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "reap") && !strings.Contains(lower, "release") {
		t.Logf("Reap output should indicate action taken: %q", output)
	}

	// Verify the node is now available (lock was reaped)
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseReapNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after reap")
	}

	if n.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("node should be available after reap, got: %s", n.WorkflowState)
	}
}

// TestReapCmd_DoesNotReapActiveLock tests that non-expired locks are not reaped.
func TestReapCmd_DoesNotReapActiveLock(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithClaim(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should indicate nothing was reaped
	_ = output

	// Verify the claim still exists
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseReapNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	if n.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("active lock should not be reaped, expected 'claimed', got: %s", n.WorkflowState)
	}
}

// TestReapCmd_AllFlagReapsRegardlessOfExpiry tests --all reaps all locks regardless of expiry.
func TestReapCmd_AllFlagReapsRegardlessOfExpiry(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithClaim(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "--all")
	if err != nil {
		t.Fatalf("expected no error with --all, got: %v", err)
	}

	// Output should indicate something was reaped
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "reap") && !strings.Contains(lower, "release") {
		t.Logf("Output with --all: %q", output)
	}

	// Verify the node is now available (even though lock wasn't expired)
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseReapNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found after reap --all")
	}

	if n.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("node should be available after reap --all, got: %s", n.WorkflowState)
	}
}

// TestReapCmd_MultipleStaleLocks tests reaping multiple stale locks at once.
func TestReapCmd_MultipleStaleLocks(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithMultipleClaims(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should indicate multiple locks were reaped
	_ = output

	// Verify state
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// 1 - should remain available (was never claimed)
	// 1.1 - should be available (expired lock reaped)
	// 1.2 - should remain claimed (active lock not reaped)
	// 1.3 - should be available (expired lock reaped)

	node11 := st.GetNode(mustParseReapNodeID(t, "1.1"))
	if node11 == nil {
		t.Error("node 1.1 not found")
	} else if node11.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("node 1.1 should be available after reap, got: %s", node11.WorkflowState)
	}

	node12 := st.GetNode(mustParseReapNodeID(t, "1.2"))
	if node12 == nil {
		t.Error("node 1.2 not found")
	} else if node12.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("node 1.2 should remain claimed (active), got: %s", node12.WorkflowState)
	}

	node13 := st.GetNode(mustParseReapNodeID(t, "1.3"))
	if node13 == nil {
		t.Error("node 1.3 not found")
	} else if node13.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("node 1.3 should be available after reap, got: %s", node13.WorkflowState)
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestReapCmd_TextOutput tests text format output.
func TestReapCmd_TextOutput(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithExpiredClaim(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "-f", "text")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Text format should be human-readable
	if output == "" {
		t.Error("expected non-empty text output")
	}

	// Should not be JSON
	var jsonTest interface{}
	if err := json.Unmarshal([]byte(output), &jsonTest); err == nil {
		if strings.HasPrefix(strings.TrimSpace(output), "{") || strings.HasPrefix(strings.TrimSpace(output), "[") {
			t.Logf("Text format might be JSON: %q", output)
		}
	}
}

// TestReapCmd_JSONOutput tests JSON format output.
func TestReapCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithExpiredClaim(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output should be valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestReapCmd_JSONOutputStructure tests the structure of JSON output.
func TestReapCmd_JSONOutputStructure(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithExpiredClaim(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		// Try as array
		var arrayResult []interface{}
		if err2 := json.Unmarshal([]byte(output), &arrayResult); err2 != nil {
			t.Fatalf("output is not valid JSON object or array: %v\nOutput: %q", err, output)
		}
		t.Logf("JSON output is an array with %d elements", len(arrayResult))
		return
	}

	// Log the structure for inspection
	t.Logf("JSON output structure: %+v", result)

	// Common expected fields might include:
	// - reaped (list of reaped locks)
	// - count (number of reaped locks)
	// - dry_run (boolean)
}

// TestReapCmd_JSONOutputWithNoLocks tests JSON output when no locks exist.
func TestReapCmd_JSONOutputWithNoLocks(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithNode(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON even with no locks
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output should be valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestReapCmd_InvalidFormat tests behavior with invalid format.
func TestReapCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithNode(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "-f", "invalid")

	// Should either error or fall back to text format
	if err != nil {
		// Error is acceptable for invalid format
		if !strings.Contains(strings.ToLower(err.Error()), "format") &&
			!strings.Contains(strings.ToLower(err.Error()), "invalid") {
			t.Logf("Error for invalid format: %v", err)
		}
		return
	}

	// If no error, output should be text format (not JSON)
	if strings.HasPrefix(strings.TrimSpace(output), "{") || strings.HasPrefix(strings.TrimSpace(output), "[") {
		t.Error("invalid format should not produce JSON output")
	}
}

// =============================================================================
// Default Directory Tests
// =============================================================================

// TestReapCmd_DefaultDirectory tests reap uses current directory by default.
func TestReapCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithNode(t)
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
	output, err := executeReapCommand(t)
	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}
}

// TestReapCmd_RelativeDirectory tests using relative directory path.
func TestReapCmd_RelativeDirectory(t *testing.T) {
	// Create nested directory structure
	baseDir, err := os.MkdirTemp("", "af-reap-rel-*")
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

	// Change to base directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(baseDir)

	// Use relative path
	output, err := executeReapCommand(t, "-d", "subdir/proof")
	if err != nil {
		t.Fatalf("expected no error with relative path, got: %v\nOutput: %s", err, output)
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestReapCmd_OutputFormats tests different output format options.
func TestReapCmd_OutputFormats(t *testing.T) {
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
				var data interface{}
				if err := json.Unmarshal([]byte(output), &data); err != nil {
					t.Errorf("expected valid JSON, got error: %v\nOutput: %q", err, output)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupReapTestWithNode(t)
			defer cleanup()

			output, err := executeReapCommand(t, "-d", tmpDir, "-f", tc.format)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tc.validator(t, output)
		})
	}
}

// TestReapCmd_DryRunVsActual tests difference between dry-run and actual execution.
func TestReapCmd_DryRunVsActual(t *testing.T) {
	tests := []struct {
		name           string
		dryRun         bool
		expectedReaped bool
	}{
		{
			name:           "dry-run should not reap",
			dryRun:         true,
			expectedReaped: false,
		},
		{
			name:           "actual run should reap",
			dryRun:         false,
			expectedReaped: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupReapTestWithExpiredClaim(t)
			defer cleanup()

			args := []string{"-d", tmpDir}
			if tc.dryRun {
				args = append(args, "--dry-run")
			}

			_, err := executeReapCommand(t, args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check if node was actually reaped
			svc, err := service.NewProofService(tmpDir)
			if err != nil {
				t.Fatalf("failed to create service: %v", err)
			}

			st, err := svc.LoadState()
			if err != nil {
				t.Fatalf("failed to load state: %v", err)
			}

			nodeID := mustParseReapNodeID(t, "1")
			n := st.GetNode(nodeID)
			if n == nil {
				t.Fatal("node not found")
			}

			wasReaped := n.WorkflowState == schema.WorkflowAvailable
			if wasReaped != tc.expectedReaped {
				t.Errorf("expected reaped=%v, got reaped=%v (state=%s)", tc.expectedReaped, wasReaped, n.WorkflowState)
			}
		})
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestReapCmd_EmptyProof tests behavior with empty proof (no nodes except root).
func TestReapCmd_EmptyProof(t *testing.T) {
	tmpDir, cleanup := setupReapTest(t)
	defer cleanup()

	// Proof is initialized but only has root node, no claims
	output, err := executeReapCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error for empty proof, got: %v", err)
	}

	// Should indicate nothing to reap
	_ = output
}

// TestReapCmd_DryRunWithAll tests --dry-run combined with --all.
func TestReapCmd_DryRunWithAll(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithClaim(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "--dry-run", "--all")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show what would be reaped with --all
	_ = output

	// Verify the claim still exists (wasn't actually reaped)
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseReapNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	if n.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("dry-run with --all should not change state, expected 'claimed', got: %s", n.WorkflowState)
	}
}

// TestReapCmd_RepeatedReap tests reaping twice has no effect second time.
func TestReapCmd_RepeatedReap(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithExpiredClaim(t)
	defer cleanup()

	// First reap
	_, err := executeReapCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("first reap failed: %v", err)
	}

	// Second reap
	output, err := executeReapCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("second reap failed: %v", err)
	}

	// Second reap should have nothing to do
	_ = output

	// Node should still be available
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID := mustParseReapNodeID(t, "1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	if n.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("node should remain available after repeated reap, got: %s", n.WorkflowState)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestReapCmd_FullWorkflow tests a complete reap workflow.
func TestReapCmd_FullWorkflow(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithNode(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID := mustParseReapNodeID(t, "1")

	// Step 1: Claim a node with very short timeout
	err = svc.ClaimNode(nodeID, "test-prover", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("failed to claim node: %v", err)
	}

	// Wait for lock to expire
	time.Sleep(10 * time.Millisecond)

	// Step 2: Verify node is claimed but expired
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}
	if n.WorkflowState != schema.WorkflowClaimed {
		t.Fatalf("expected node to be claimed, got: %s", n.WorkflowState)
	}

	// Step 3: Dry-run reap to see what would happen
	dryRunOutput, err := executeReapCommand(t, "-d", tmpDir, "--dry-run")
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	t.Logf("Dry-run output: %s", dryRunOutput)

	// Verify still claimed after dry-run
	st, _ = svc.LoadState()
	n = st.GetNode(nodeID)
	if n.WorkflowState != schema.WorkflowClaimed {
		t.Error("node should still be claimed after dry-run")
	}

	// Step 4: Actually reap
	reapOutput, err := executeReapCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("reap failed: %v", err)
	}
	t.Logf("Reap output: %s", reapOutput)

	// Step 5: Verify node is now available
	st, _ = svc.LoadState()
	n = st.GetNode(nodeID)
	if n.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("expected node to be available after reap, got: %s", n.WorkflowState)
	}
}

// TestReapCmd_ReapCountInOutput tests that output indicates count of reaped locks.
func TestReapCmd_ReapCountInOutput(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithMultipleClaims(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		// Try parsing as array
		var arrayResult []interface{}
		if err2 := json.Unmarshal([]byte(output), &arrayResult); err2 != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		// If array, check length
		t.Logf("Result is array with %d items", len(arrayResult))
		return
	}

	// Check for count field
	if count, ok := result["count"]; ok {
		t.Logf("Reap count: %v", count)
	}
	if reaped, ok := result["reaped"]; ok {
		t.Logf("Reaped nodes: %v", reaped)
	}
}

// TestReapCmd_JSONOutputWithDryRun tests JSON structure includes dry_run indicator.
func TestReapCmd_JSONOutputWithDryRun(t *testing.T) {
	tmpDir, cleanup := setupReapTestWithExpiredClaim(t)
	defer cleanup()

	output, err := executeReapCommand(t, "-d", tmpDir, "--dry-run", "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Logf("Output: %s", output)
		return
	}

	// Check if dry_run field is present
	if dryRun, ok := result["dry_run"]; ok {
		if dryRun != true {
			t.Errorf("expected dry_run to be true, got: %v", dryRun)
		}
	} else {
		t.Log("JSON output does not contain 'dry_run' field (may be acceptable)")
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

// TestReapCmd_InvalidDirType tests error when --dir points to a file.
func TestReapCmd_InvalidDirType(t *testing.T) {
	// Create a temporary file (not directory)
	tmpFile, err := os.CreateTemp("", "af-reap-test-file-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	_, err = executeReapCommand(t, "-d", tmpFile.Name())

	if err == nil {
		t.Error("expected error when --dir is a file")
	}
}

// TestReapCmd_PermissionDenied tests handling of permission errors.
func TestReapCmd_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	tmpDir, cleanup := setupReapTestWithNode(t)
	defer cleanup()

	// Remove read permissions
	oldMode, _ := os.Stat(tmpDir)
	os.Chmod(tmpDir, 0000)
	defer os.Chmod(tmpDir, oldMode.Mode())

	_, err := executeReapCommand(t, "-d", tmpDir)

	if err == nil {
		t.Error("expected error for permission denied")
	}
}
