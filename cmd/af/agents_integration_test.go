//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Integration Test Helpers
// =============================================================================

// newTestAgentsCmdIntegration creates a fresh root command for integration tests.
func newTestAgentsCmdIntegration() *cobra.Command {
	cmd := newTestRootCmd()

	agentsCmd := newAgentsCmd()
	cmd.AddCommand(agentsCmd)

	return cmd
}

// executeAgentsCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeAgentsCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// setupAgentsTest creates a temp directory with an initialized proof.
func setupAgentsTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-agents-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")

	// Initialize proof directory structure
	if err := service.InitProofDir(proofDir); err != nil {
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

// setupAgentsTestWithClaims creates a proof with claimed nodes.
func setupAgentsTestWithClaims(t *testing.T) (string, func()) {
	t.Helper()

	proofDir, cleanup := setupAgentsTest(t)

	// Create service and claim the root node
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Claim node 1 (root) by agent "agent-1"
	rootID, _ := service.ParseNodeID("1")
	err = svc.ClaimNode(rootID, "agent-1", 5*time.Minute)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	return proofDir, cleanup
}

// setupAgentsTestWithHistory creates a proof with claim/release history.
func setupAgentsTestWithHistory(t *testing.T) (string, func()) {
	t.Helper()

	proofDir, cleanup := setupAgentsTestWithClaims(t)

	// Create service to add more history
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create child node 1.1
	child1ID, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(child1ID, service.NodeTypeClaim, "First child node", service.InferenceModusPonens)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Claim and then release node 1.1 by agent-2
	err = svc.ClaimNode(child1ID, "agent-2", 5*time.Minute)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	err = svc.ReleaseNode(child1ID, "agent-2")
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	return proofDir, cleanup
}

// =============================================================================
// Happy Path Tests
// =============================================================================

// TestAgentsCmd_NoActivity verifies output when no nodes are claimed.
func TestAgentsCmd_NoActivity(t *testing.T) {
	proofDir, cleanup := setupAgentsTest(t)
	defer cleanup()

	cmd := newTestAgentsCmdIntegration()
	output, err := executeAgentsCommand(cmd, "agents", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should indicate no agents are active
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "no") || !strings.Contains(lowerOutput, "claimed") {
		// Either "no claimed nodes" or similar message
		t.Logf("Output: %s", output)
	}
}

// TestAgentsCmd_ShowsClaimedNodes verifies displaying claimed nodes.
func TestAgentsCmd_ShowsClaimedNodes(t *testing.T) {
	proofDir, cleanup := setupAgentsTestWithClaims(t)
	defer cleanup()

	cmd := newTestAgentsCmdIntegration()
	output, err := executeAgentsCommand(cmd, "agents", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should show the claimed node and agent
	expectations := []string{
		"1",       // Node ID
		"agent-1", // Owner
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got: %q", exp, output)
		}
	}
}

// TestAgentsCmd_ShowsActivity verifies displaying claim/release activity.
func TestAgentsCmd_ShowsActivity(t *testing.T) {
	proofDir, cleanup := setupAgentsTestWithHistory(t)
	defer cleanup()

	cmd := newTestAgentsCmdIntegration()
	output, err := executeAgentsCommand(cmd, "agents", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should show activity events
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "claim") && !strings.Contains(lowerOutput, "release") {
		t.Logf("Expected activity section in output: %s", output)
	}

	// Should show agent-2's activity (claim and release)
	if !strings.Contains(output, "agent-2") {
		t.Errorf("expected output to show agent-2's activity, got: %q", output)
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestAgentsCmd_JSONOutput verifies JSON output format.
func TestAgentsCmd_JSONOutput(t *testing.T) {
	proofDir, cleanup := setupAgentsTestWithHistory(t)
	defer cleanup()

	cmd := newTestAgentsCmdIntegration()
	output, err := executeAgentsCommand(cmd, "agents", "--format", "json", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v\nOutput: %s", err, output)
		return
	}

	// JSON should include key fields
	expectedKeys := []string{"claimed_nodes", "activity"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON to contain key %q", key)
		}
	}
}

// TestAgentsCmd_JSONOutputShortFlag verifies JSON output with short flag.
func TestAgentsCmd_JSONOutputShortFlag(t *testing.T) {
	proofDir, cleanup := setupAgentsTestWithClaims(t)
	defer cleanup()

	cmd := newTestAgentsCmdIntegration()
	output, err := executeAgentsCommand(cmd, "agents", "-f", "json", "-d", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output with -f flag, got error: %v", err)
	}
}

// TestAgentsCmd_JSONOutputEmpty verifies JSON output with no claims.
func TestAgentsCmd_JSONOutputEmpty(t *testing.T) {
	proofDir, cleanup := setupAgentsTest(t)
	defer cleanup()

	cmd := newTestAgentsCmdIntegration()
	output, err := executeAgentsCommand(cmd, "agents", "--format", "json", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON with empty arrays
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v\nOutput: %s", err, output)
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestAgentsCmd_ProofNotInitialized verifies error when proof not initialized.
func TestAgentsCmd_ProofNotInitialized(t *testing.T) {
	// Create empty directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-agents-noinit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	proofDir := filepath.Join(tmpDir, "proof")
	if err := service.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}
	// Note: NOT calling service.Init(), so proof is not initialized

	cmd := newTestAgentsCmdIntegration()
	output, err := executeAgentsCommand(cmd, "agents", "--dir", proofDir)

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

// TestAgentsCmd_DirFlagIsFile verifies error when --dir points to a file.
func TestAgentsCmd_DirFlagIsFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "af-agents-test-file-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cmd := newTestAgentsCmdIntegration()
	_, err = executeAgentsCommand(cmd, "agents", "--dir", tmpFile.Name())

	if err == nil {
		t.Error("expected error when --dir is a file, got nil")
	}
}

// =============================================================================
// Activity Limit Tests
// =============================================================================

// TestAgentsCmd_ActivityLimitFlag verifies --limit flag for activity history.
func TestAgentsCmd_ActivityLimitFlag(t *testing.T) {
	proofDir, cleanup := setupAgentsTestWithHistory(t)
	defer cleanup()

	// Test with limit flag
	cmd := newTestAgentsCmdIntegration()
	output, err := executeAgentsCommand(cmd, "agents", "--limit", "5", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should exist (limit doesn't filter out everything)
	if len(output) == 0 {
		t.Error("expected non-empty output with --limit flag")
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestAgentsCmd_DefaultDirectory verifies using default (current) directory.
func TestAgentsCmd_DefaultDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "af-agents-default-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	proofDir := filepath.Join(tmpDir, "proof")
	if err := service.InitProofDir(proofDir); err != nil {
		t.Fatal(err)
	}

	service.Init(proofDir, "Test conjecture", "test-author")

	// Change to proof directory for this test
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(proofDir); err != nil {
		t.Fatal(err)
	}

	cmd := newTestAgentsCmdIntegration()
	output, err := executeAgentsCommand(cmd, "agents")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should work with current directory
	_ = output // Verify it doesn't error
}

// TestAgentsCmd_MultipleAgents verifies handling of multiple agents.
func TestAgentsCmd_MultipleAgents(t *testing.T) {
	proofDir, cleanup := setupAgentsTest(t)
	defer cleanup()

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create multiple child nodes
	child1ID, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(child1ID, service.NodeTypeClaim, "First child", service.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	child2ID, _ := service.ParseNodeID("1.2")
	err = svc.CreateNode(child2ID, service.NodeTypeClaim, "Second child", service.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	// Claim nodes by different agents
	err = svc.ClaimNode(child1ID, "agent-alpha", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	err = svc.ClaimNode(child2ID, "agent-beta", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	cmd := newTestAgentsCmdIntegration()
	output, err := executeAgentsCommand(cmd, "agents", "--dir", proofDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show both agents
	if !strings.Contains(output, "agent-alpha") {
		t.Errorf("expected output to show agent-alpha, got: %q", output)
	}
	if !strings.Contains(output, "agent-beta") {
		t.Errorf("expected output to show agent-beta, got: %q", output)
	}
}
