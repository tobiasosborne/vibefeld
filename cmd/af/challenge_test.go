//go:build integration

// Package main contains tests for the af challenge command.
// These are TDD tests - the challenge command does not exist yet.
// Tests define the expected behavior for raising challenges against proof nodes.
package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupChallengeTest creates a temporary directory with an initialized proof
// for testing the challenge command. Node 1 is created by Init with the conjecture.
func setupChallengeTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-challenge-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	// Initialize proof - this creates node 1 with the conjecture
	err = service.Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupChallengeTestWithMultipleNodes creates a test environment with multiple nodes.
func setupChallengeTestWithMultipleNodes(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupChallengeTest(t)

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create child nodes
	for _, idStr := range []string{"1.1", "1.2"} {
		nodeID, _ := types.Parse(idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement "+idStr, schema.InferenceModusPonens)
		if err != nil {
			cleanup()
			t.Fatal(err)
		}
	}

	return tmpDir, cleanup
}

// newChallengeTestCmd creates a test command hierarchy with the challenge command.
// This ensures test isolation - each test gets its own command instance.
func newChallengeTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	challengeCmd := newChallengeCmd()
	cmd.AddCommand(challengeCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// =============================================================================
// Error Case Tests (Critical for TDD)
// =============================================================================

// TestChallengeCmd_ProofNotInitialized tests error when proof hasn't been initialized.
func TestChallengeCmd_ProofNotInitialized(t *testing.T) {
	// Create a directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-challenge-uninit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newChallengeTestCmd()
	_, err = executeCommand(cmd, "challenge", "1",
		"--reason", "Test challenge reason",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}
}

// TestChallengeCmd_NodeNotFound tests error when node doesn't exist.
func TestChallengeCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	_, err := executeCommand(cmd, "challenge", "1.999",
		"--reason", "Test challenge reason",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for non-existent node, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "does not exist") {
		t.Errorf("expected error about node not found, got: %q", errStr)
	}
}

// TestChallengeCmd_MissingNodeID tests error when node ID is not provided.
func TestChallengeCmd_MissingNodeID(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	_, err := executeCommand(cmd, "challenge",
		"--reason", "Test challenge reason",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for missing node ID, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "accepts 1 arg") && !strings.Contains(errStr, "argument") {
		t.Errorf("expected error about missing argument, got: %q", errStr)
	}
}

// TestChallengeCmd_MissingReason tests error when reason is not provided.
func TestChallengeCmd_MissingReason(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	_, err := executeCommand(cmd, "challenge", "1",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for missing reason, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "reason") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing reason, got: %q", errStr)
	}
}

// TestChallengeCmd_EmptyReason tests error when reason is empty string.
func TestChallengeCmd_EmptyReason(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	_, err := executeCommand(cmd, "challenge", "1",
		"--reason", "",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for empty reason, got nil")
	}
}

// TestChallengeCmd_WhitespaceOnlyReason tests error when reason is whitespace only.
func TestChallengeCmd_WhitespaceOnlyReason(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	_, err := executeCommand(cmd, "challenge", "1",
		"--reason", "   ",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for whitespace-only reason, got nil")
	}
}

// TestChallengeCmd_InvalidNodeIDFormat tests error for invalid node ID formats.
func TestChallengeCmd_InvalidNodeIDFormat(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	tests := []struct {
		name   string
		nodeID string
	}{
		{"empty string", ""},
		{"invalid characters", "abc"},
		{"negative number", "-1"},
		{"zero", "0"},
		{"invalid format with spaces", "1 2"},
		{"leading dot", ".1"},
		{"trailing dot", "1."},
		{"double dot", "1..2"},
		{"non-root start", "2.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newChallengeTestCmd()
			_, err := executeCommand(cmd, "challenge", tc.nodeID,
				"--reason", "Test reason",
				"--dir", tmpDir,
			)

			if err == nil {
				t.Errorf("expected error for invalid node ID %q, got nil", tc.nodeID)
			}
		})
	}
}

// TestChallengeCmd_DirectoryNotFound tests error when directory doesn't exist.
func TestChallengeCmd_DirectoryNotFound(t *testing.T) {
	cmd := newChallengeTestCmd()
	_, err := executeCommand(cmd, "challenge", "1",
		"--reason", "Test reason",
		"--dir", "/nonexistent/path/12345",
	)

	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// =============================================================================
// Success Case Tests
// =============================================================================

// TestChallengeCmd_Success tests successfully challenging a pending node.
func TestChallengeCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	output, err := executeCommand(cmd, "challenge", "1",
		"--reason", "The inference is invalid",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should mention challenge
	if !strings.Contains(strings.ToLower(output), "challenge") {
		t.Errorf("expected output to mention challenge, got: %q", output)
	}
}

// TestChallengeCmd_WithTarget tests challenging with a specific target.
func TestChallengeCmd_WithTarget(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	output, err := executeCommand(cmd, "challenge", "1",
		"--reason", "The statement is unclear",
		"--target", "statement",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should mention challenge
	if !strings.Contains(strings.ToLower(output), "challenge") {
		t.Errorf("expected output to mention challenge, got: %q", output)
	}
}

// TestChallengeCmd_WithReasonText tests challenge with longer reason text.
func TestChallengeCmd_WithReasonText(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	longReason := "This inference is invalid because it assumes x > 0 without proof. " +
		"The parent node does not establish this constraint, and the local scope " +
		"does not include this assumption. This creates a logical gap."

	cmd := newChallengeTestCmd()
	output, err := executeCommand(cmd, "challenge", "1",
		"--reason", longReason,
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should mention challenge
	if !strings.Contains(strings.ToLower(output), "challenge") {
		t.Errorf("expected output to mention challenge, got: %q", output)
	}
}

// TestChallengeCmd_ChildNode tests challenging a child node.
func TestChallengeCmd_ChildNode(t *testing.T) {
	tmpDir, cleanup := setupChallengeTestWithMultipleNodes(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	output, err := executeCommand(cmd, "challenge", "1.1",
		"--reason", "This subgoal is not sufficient",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should mention the node ID
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected output to mention node 1.1, got: %q", output)
	}
}

// =============================================================================
// JSON Output Tests
// =============================================================================

// TestChallengeCmd_JSONFormat tests JSON output format.
func TestChallengeCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	output, err := executeCommand(cmd, "challenge", "1",
		"--reason", "Test challenge",
		"--format", "json",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Should contain relevant fields
	expectedKeys := []string{"node_id", "reason"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			// Try camelCase variant
			camelKey := strings.ReplaceAll(key, "_", "")
			if _, ok := result[camelKey]; !ok {
				t.Logf("Warning: JSON output does not contain %q or camelCase variant", key)
			}
		}
	}
}

// TestChallengeCmd_JSONFormatShortFlag tests JSON output with short flag.
func TestChallengeCmd_JSONFormatShortFlag(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	output, err := executeCommand(cmd, "challenge", "1",
		"--reason", "Test challenge",
		"-f", "json",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON with -f flag, got: %v\nOutput: %q", err, output)
	}
}

// =============================================================================
// Challenge Target Tests
// =============================================================================

// TestChallengeCmd_ValidTargets tests all valid challenge targets.
func TestChallengeCmd_ValidTargets(t *testing.T) {
	validTargets := []string{
		"statement",
		"inference",
		"context",
		"dependencies",
		"scope",
		"gap",
		"type_error",
		"domain",
		"completeness",
	}

	for _, target := range validTargets {
		t.Run(target, func(t *testing.T) {
			tmpDir, cleanup := setupChallengeTest(t)
			defer cleanup()

			cmd := newChallengeTestCmd()
			_, err := executeCommand(cmd, "challenge", "1",
				"--reason", "Test reason for "+target,
				"--target", target,
				"--dir", tmpDir,
			)

			if err != nil {
				t.Errorf("expected no error for valid target %q, got: %v", target, err)
			}
		})
	}
}

// TestChallengeCmd_InvalidTarget tests error for invalid challenge target.
func TestChallengeCmd_InvalidTarget(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	_, err := executeCommand(cmd, "challenge", "1",
		"--reason", "Test reason",
		"--target", "invalid_target",
		"--dir", tmpDir,
	)

	if err == nil {
		t.Fatal("expected error for invalid target, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "invalid") && !strings.Contains(errStr, "target") {
		t.Errorf("expected error about invalid target, got: %q", errStr)
	}
}

// TestChallengeCmd_EmptyTarget tests behavior when target is empty (should use default).
func TestChallengeCmd_EmptyTarget(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	_, err := executeCommand(cmd, "challenge", "1",
		"--reason", "Test reason",
		"--target", "",
		"--dir", tmpDir,
	)

	// Empty target might be treated as invalid or might use default
	// The exact behavior depends on implementation
	// At minimum, it should not panic
	_ = err
}

// =============================================================================
// Already Challenged Node Tests
// =============================================================================

// TestChallengeCmd_AlreadyChallenged tests challenging an already-challenged node.
// This should either succeed (multiple challenges allowed) or error gracefully.
func TestChallengeCmd_AlreadyChallenged(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	// First challenge
	cmd1 := newChallengeTestCmd()
	_, err := executeCommand(cmd1, "challenge", "1",
		"--reason", "First challenge",
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("first challenge failed: %v", err)
	}

	// Second challenge
	cmd2 := newChallengeTestCmd()
	_, err = executeCommand(cmd2, "challenge", "1",
		"--reason", "Second challenge",
		"--dir", tmpDir,
	)

	// Behavior depends on implementation:
	// - Could succeed (multiple challenges allowed per node)
	// - Could error (only one challenge per node)
	// Either is acceptable; test documents the behavior
	if err != nil {
		t.Logf("Second challenge on same node returned error: %v", err)
		t.Logf("This may be expected if only one challenge per node is allowed")
	} else {
		t.Logf("Second challenge on same node succeeded")
		t.Logf("This suggests multiple challenges per node are allowed")
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestChallengeCmd_Help tests that help output shows usage information.
func TestChallengeCmd_Help(t *testing.T) {
	cmd := newChallengeTestCmd()
	output, err := executeCommand(cmd, "challenge", "--help")

	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	// Check for expected help content
	expectations := []string{
		"challenge",
		"node-id",
		"--reason",
		"--target",
		"--dir",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestChallengeCmd_HelpShortFlag tests help with short flag.
func TestChallengeCmd_HelpShortFlag(t *testing.T) {
	cmd := newChallengeTestCmd()
	output, err := executeCommand(cmd, "challenge", "-h")

	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	if !strings.Contains(output, "challenge") {
		t.Errorf("help output should mention 'challenge', got: %q", output)
	}
}

// =============================================================================
// Short Flags Tests
// =============================================================================

// TestChallengeCmd_ShortFlags tests that short flags work correctly.
func TestChallengeCmd_ShortFlags(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	output, err := executeCommand(cmd, "challenge", "1",
		"-r", "Test reason with short flag",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	if !strings.Contains(strings.ToLower(output), "challenge") {
		t.Errorf("expected output to mention challenge, got: %q", output)
	}
}

// TestChallengeCmd_ShortFlagsWithTarget tests short flags with target.
func TestChallengeCmd_ShortFlagsWithTarget(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	_, err := executeCommand(cmd, "challenge", "1",
		"-r", "Test reason",
		"-t", "inference",
		"-d", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with short flags and target, got: %v", err)
	}
}

// =============================================================================
// Default Directory Tests
// =============================================================================

// TestChallengeCmd_DefaultDirectory tests challenge uses current directory by default.
func TestChallengeCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
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
	cmd := newChallengeTestCmd()
	output, err := executeCommand(cmd, "challenge", "1",
		"--reason", "Test reason",
	)

	if err != nil {
		t.Fatalf("expected no error when using current directory, got: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(strings.ToLower(output), "challenge") {
		t.Errorf("expected output to mention challenge, got: %q", output)
	}
}

// =============================================================================
// Table-Driven Comprehensive Tests
// =============================================================================

// TestChallengeCmd_TableDrivenReasons tests various reason formats.
func TestChallengeCmd_TableDrivenReasons(t *testing.T) {
	tests := []struct {
		name    string
		reason  string
		wantErr bool
	}{
		{
			name:    "simple reason",
			reason:  "Invalid inference",
			wantErr: false,
		},
		{
			name:    "detailed reason",
			reason:  "The inference assumes x > 0 without establishing this constraint",
			wantErr: false,
		},
		{
			name: "multiline reason",
			reason: `This proof step has multiple issues:
1. Missing definition reference
2. Invalid scope assumption
3. Incomplete case coverage`,
			wantErr: false,
		},
		{
			name:    "reason with special chars",
			reason:  "Uses x^2 + y^2 = z^2 without proving x, y, z ∈ ℝ",
			wantErr: false,
		},
		{
			name:    "empty reason",
			reason:  "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			reason:  "   ",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupChallengeTest(t)
			defer cleanup()

			cmd := newChallengeTestCmd()
			_, err := executeCommand(cmd, "challenge", "1",
				"--reason", tc.reason,
				"--dir", tmpDir,
			)

			if tc.wantErr && err == nil {
				t.Errorf("expected error for reason %q, got nil", tc.reason)
			}

			if !tc.wantErr && err != nil {
				t.Errorf("expected no error for reason %q, got: %v", tc.reason, err)
			}
		})
	}
}

// TestChallengeCmd_TableDrivenNodeIDs tests various node IDs.
func TestChallengeCmd_TableDrivenNodeIDs(t *testing.T) {
	tests := []struct {
		name      string
		nodeID    string
		setupNode bool // whether to create the node first
		wantErr   bool
	}{
		{
			name:      "valid root",
			nodeID:    "1",
			setupNode: true,
			wantErr:   false,
		},
		{
			name:      "valid child",
			nodeID:    "1.1",
			setupNode: true,
			wantErr:   false,
		},
		{
			name:      "non-existent node",
			nodeID:    "1.999",
			setupNode: false,
			wantErr:   true,
		},
		{
			name:      "invalid format",
			nodeID:    "abc",
			setupNode: false,
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var tmpDir string
			var cleanup func()

			if tc.setupNode {
				if strings.Contains(tc.nodeID, ".") {
					tmpDir, cleanup = setupChallengeTestWithMultipleNodes(t)
				} else {
					tmpDir, cleanup = setupChallengeTest(t)
				}
			} else {
				tmpDir, cleanup = setupChallengeTest(t)
			}
			defer cleanup()

			cmd := newChallengeTestCmd()
			_, err := executeCommand(cmd, "challenge", tc.nodeID,
				"--reason", "Test reason",
				"--dir", tmpDir,
			)

			if tc.wantErr && err == nil {
				t.Errorf("expected error for node ID %q, got nil", tc.nodeID)
			}

			if !tc.wantErr && err != nil {
				t.Errorf("expected no error for node ID %q, got: %v", tc.nodeID, err)
			}
		})
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestChallengeCmd_MultipleNodesSequential tests challenging multiple nodes in sequence.
func TestChallengeCmd_MultipleNodesSequential(t *testing.T) {
	tmpDir, cleanup := setupChallengeTestWithMultipleNodes(t)
	defer cleanup()

	nodes := []string{"1", "1.1", "1.2"}
	for _, nodeID := range nodes {
		cmd := newChallengeTestCmd()
		output, err := executeCommand(cmd, "challenge", nodeID,
			"--reason", "Challenge for node "+nodeID,
			"--dir", tmpDir,
		)

		if err != nil {
			t.Fatalf("failed to challenge node %s: %v\nOutput: %s", nodeID, err, output)
		}
	}
}

// TestChallengeCmd_DirFlagVariants tests both long and short forms of --dir flag.
func TestChallengeCmd_DirFlagVariants(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "long form",
			args: []string{"--dir"},
		},
		{
			name: "short form",
			args: []string{"-d"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupChallengeTest(t)
			defer cleanup()

			args := []string{"challenge", "1", "--reason", "Test reason"}
			args = append(args, tc.args...)
			args = append(args, tmpDir)

			cmd := newChallengeTestCmd()
			_, err := executeCommand(cmd, args...)

			if err != nil {
				t.Errorf("expected no error with %s flag, got: %v", tc.name, err)
			}
		})
	}
}

// =============================================================================
// Success Message Tests
// =============================================================================

// TestChallengeCmd_SuccessMessage tests that success message is informative.
func TestChallengeCmd_SuccessMessage(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	cmd := newChallengeTestCmd()
	output, err := executeCommand(cmd, "challenge", "1",
		"--reason", "Test challenge reason",
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Success message should indicate what happened
	lower := strings.ToLower(output)
	hasStatusInfo := strings.Contains(lower, "challenge") ||
		strings.Contains(lower, "raised") ||
		strings.Contains(lower, "created")

	if !hasStatusInfo {
		t.Errorf("success message should mention challenge/raised/created, got: %q", output)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestChallengeCmd_VeryLongReason tests challenge with very long reason text.
func TestChallengeCmd_VeryLongReason(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	// Create a very long but valid reason
	longReason := strings.Repeat("This is a detailed explanation of the challenge. ", 100)

	cmd := newChallengeTestCmd()
	_, err := executeCommand(cmd, "challenge", "1",
		"--reason", longReason,
		"--dir", tmpDir,
	)

	// Should handle long reasons gracefully (either succeed or return clear error)
	if err != nil {
		t.Logf("Long reason returned error: %v", err)
		// If there's a length limit, that's acceptable
	}
}

// TestChallengeCmd_SpecialCharactersInReason tests special characters in reason.
func TestChallengeCmd_SpecialCharactersInReason(t *testing.T) {
	tmpDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	specialReason := "Uses symbols: ∀x ∈ ℝ, ∃y: x² + y² = 1, but doesn't prove x ≠ 0"

	cmd := newChallengeTestCmd()
	_, err := executeCommand(cmd, "challenge", "1",
		"--reason", specialReason,
		"--dir", tmpDir,
	)

	if err != nil {
		t.Fatalf("expected no error with special characters in reason, got: %v", err)
	}
}
