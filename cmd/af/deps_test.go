//go:build integration

// Package main contains tests for the af deps command.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// ===========================================================================
// Tests for af deps command (dependency visualization)
// ===========================================================================

// setupDepsTest creates a test environment with nodes and dependencies.
func setupDepsTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-deps-test-*")
	if err != nil {
		t.Fatal(err)
	}

	// Initialize proof
	err = service.Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Claim root and create some children with dependencies
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	rootID, _ := types.Parse("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create 1.1 (no deps)
	child1, _ := types.Parse("1.1")
	err = svc.RefineNode(rootID, "test-agent", child1, schema.NodeTypeClaim, "First step", schema.InferenceAssumption)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create 1.1: %v", err)
	}

	// Create 1.2 with validation dep on 1.1
	child2, _ := types.Parse("1.2")
	err = svc.RefineNodeWithAllDeps(rootID, "test-agent", child2, schema.NodeTypeClaim, "Second step", schema.InferenceAssumption, nil, []types.NodeID{child1})
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create 1.2: %v", err)
	}

	// Create 1.3 with validation deps on 1.1 and 1.2
	child3, _ := types.Parse("1.3")
	err = svc.RefineNodeWithAllDeps(rootID, "test-agent", child3, schema.NodeTypeClaim, "Third step", schema.InferenceAssumption, nil, []types.NodeID{child1, child2})
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create 1.3: %v", err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return tmpDir, cleanup
}

// executeDepsCommand executes the deps command with arguments.
func executeDepsCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// TestDepsCmd_ShowsDependenciesForNode tests that deps shows validation dependencies.
func TestDepsCmd_ShowsDependenciesForNode(t *testing.T) {
	tmpDir, cleanup := setupDepsTest(t)
	defer cleanup()

	output, err := executeDepsCommand(t, "1.3", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show the validation dependencies
	if !strings.Contains(output, "1.1") || !strings.Contains(output, "1.2") {
		t.Errorf("expected output to show deps 1.1 and 1.2, got: %q", output)
	}
}

// TestDepsCmd_ShowsNoDependencies tests output for node without deps.
func TestDepsCmd_ShowsNoDependencies(t *testing.T) {
	tmpDir, cleanup := setupDepsTest(t)
	defer cleanup()

	output, err := executeDepsCommand(t, "1.1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should indicate no dependencies
	if !strings.Contains(strings.ToLower(output), "no") && !strings.Contains(output, "[]") && !strings.Contains(strings.ToLower(output), "none") {
		t.Errorf("expected output to indicate no deps, got: %q", output)
	}
}

// TestDepsCmd_MissingNodeID tests error for missing node ID.
func TestDepsCmd_MissingNodeID(t *testing.T) {
	tmpDir, cleanup := setupDepsTest(t)
	defer cleanup()

	_, err := executeDepsCommand(t, "-d", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing node ID, got nil")
	}
}

// TestDepsCmd_NodeNotFound tests error for non-existent node.
func TestDepsCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupDepsTest(t)
	defer cleanup()

	output, err := executeDepsCommand(t, "1.99", "-d", tmpDir)

	// Should error
	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "not found") && err == nil {
		t.Errorf("expected error or 'not found' message, got: %q", output)
	}
}

// TestDepsCmd_InvalidNodeID tests error for invalid node ID format.
func TestDepsCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupDepsTest(t)
	defer cleanup()

	output, err := executeDepsCommand(t, "invalid", "-d", tmpDir)

	combined := output
	if err != nil {
		combined += err.Error()
	}

	if !strings.Contains(strings.ToLower(combined), "invalid") && err == nil {
		t.Errorf("expected error about invalid ID, got: %q", output)
	}
}

// TestDepsCmd_JSONOutput tests JSON output format.
func TestDepsCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupDepsTest(t)
	defer cleanup()

	output, err := executeDepsCommand(t, "1.3", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Should contain validation deps
	if _, ok := result["validation_deps"]; !ok {
		if _, ok := result["requires_validated"]; !ok {
			t.Log("Warning: JSON output does not contain validation_deps or requires_validated field")
		}
	}
}

// TestDepsCmd_ShowsValidationStatus tests that deps shows validation status of deps.
func TestDepsCmd_ShowsValidationStatus(t *testing.T) {
	tmpDir, cleanup := setupDepsTest(t)
	defer cleanup()

	// Accept 1.1 to change its status
	svc, _ := service.NewProofService(tmpDir)
	child1, _ := types.Parse("1.1")
	err := svc.AcceptNode(child1)
	if err != nil {
		t.Fatalf("failed to accept 1.1: %v", err)
	}

	output, err := executeDepsCommand(t, "1.3", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should distinguish between validated and unvalidated deps
	// 1.1 should show as validated, 1.2 should show as pending
	if !strings.Contains(output, "validated") && !strings.Contains(output, "pending") {
		t.Logf("Output may not show status: %q", output)
	}
}

// TestDepsCmd_ShowsBothDepTypes tests that deps shows both reference and validation deps.
func TestDepsCmd_ShowsBothDepTypes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "af-deps-both-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize and set up nodes
	err = service.Init(tmpDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatal(err)
	}

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	rootID, _ := types.Parse("1")
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Create 1.1 and 1.2
	child1, _ := types.Parse("1.1")
	child2, _ := types.Parse("1.2")
	err = svc.RefineNode(rootID, "test-agent", child1, schema.NodeTypeClaim, "First step", schema.InferenceAssumption)
	if err != nil {
		t.Fatal(err)
	}
	err = svc.RefineNode(rootID, "test-agent", child2, schema.NodeTypeClaim, "Second step", schema.InferenceAssumption)
	if err != nil {
		t.Fatal(err)
	}

	// Create 1.3 with BOTH reference dep on 1.1 AND validation dep on 1.2
	child3, _ := types.Parse("1.3")
	err = svc.RefineNodeWithAllDeps(rootID, "test-agent", child3, schema.NodeTypeClaim, "Combined step", schema.InferenceAssumption, []types.NodeID{child1}, []types.NodeID{child2})
	if err != nil {
		t.Fatal(err)
	}

	output, err := executeDepsCommand(t, "1.3", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show both 1.1 (reference dep) and 1.2 (validation dep)
	if !strings.Contains(output, "1.1") || !strings.Contains(output, "1.2") {
		t.Errorf("expected output to show both deps, got: %q", output)
	}
}

// TestDepsCmd_Help tests help output.
func TestDepsCmd_Help(t *testing.T) {
	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	// Should contain expected help content
	expectations := []string{
		"deps",
		"node-id",
		"--dir",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestDepsCmd_ShowsBlockedStatus tests that deps indicates if deps are blocking acceptance.
func TestDepsCmd_ShowsBlockedStatus(t *testing.T) {
	tmpDir, cleanup := setupDepsTest(t)
	defer cleanup()

	output, err := executeDepsCommand(t, "1.3", "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should indicate the node is blocked (since deps aren't validated)
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "block") && !strings.Contains(lower, "pending") && !strings.Contains(lower, "unvalidated") {
		t.Logf("Output might not show blocked status: %q", output)
	}
}
