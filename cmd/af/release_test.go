//go:build integration

package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newTestReleaseCmd creates a fresh root command with the release subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestReleaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	releaseCmd := newReleaseCmd()
	cmd.AddCommand(releaseCmd)

	return cmd
}

// setupReleaseTest creates a temporary proof directory with an initialized proof,
// a node, and claims that node for testing release operations.
func setupReleaseTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-release-test-*")
	if err != nil {
		t.Fatal(err)
	}

	// Initialize proof
	err = service.Init(tmpDir, "Test conjecture for release tests", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create and claim a node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create a root node
	rootID, err := types.Parse("1")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	err = svc.CreateNode(rootID, schema.NodeTypeClaim, "Test goal statement", schema.InferenceAssumption)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Claim the node
	err = svc.ClaimNode(rootID, "test-agent", time.Hour)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return tmpDir, cleanup
}

// setupReleaseTestWithUnclaimedNode creates a proof with an available (unclaimed) node.
func setupReleaseTestWithUnclaimedNode(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-release-test-*")
	if err != nil {
		t.Fatal(err)
	}

	// Initialize proof
	err = service.Init(tmpDir, "Test conjecture for release tests", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create a node but don't claim it
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	rootID, err := types.Parse("1")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	err = svc.CreateNode(rootID, schema.NodeTypeClaim, "Test goal statement", schema.InferenceAssumption)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return tmpDir, cleanup
}

func TestReleaseCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	output, err := executeCommand(cmd, "release", "1", "--owner", "test-agent", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify output indicates success
	if !strings.Contains(output, "released") && !strings.Contains(output, "available") {
		t.Errorf("expected output to indicate node was released, got: %q", output)
	}
}

func TestReleaseCmd_MissingNodeID(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	_, err := executeCommand(cmd, "release", "--owner", "test-agent", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing node ID, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "node") && !strings.Contains(errStr, "argument") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing node ID, got: %q", errStr)
	}
}

func TestReleaseCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	_, err := executeCommand(cmd, "release", "1.999", "--owner", "test-agent", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for non-existent node, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not found") {
		t.Errorf("expected 'not found' error, got: %q", errStr)
	}
}

func TestReleaseCmd_NodeNotClaimed(t *testing.T) {
	tmpDir, cleanup := setupReleaseTestWithUnclaimedNode(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	_, err := executeCommand(cmd, "release", "1", "--owner", "test-agent", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for unclaimed node, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not claimed") {
		t.Errorf("expected 'not claimed' error, got: %q", errStr)
	}
}

func TestReleaseCmd_WrongOwner(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	_, err := executeCommand(cmd, "release", "1", "--owner", "wrong-agent", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for wrong owner, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "owner") && !strings.Contains(errStr, "match") {
		t.Errorf("expected owner mismatch error, got: %q", errStr)
	}
}

func TestReleaseCmd_MissingOwnerFlag(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	_, err := executeCommand(cmd, "release", "1", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing owner flag, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "owner") && !strings.Contains(errStr, "required") {
		t.Errorf("expected error about missing owner, got: %q", errStr)
	}
}

func TestReleaseCmd_EmptyOwner(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	_, err := executeCommand(cmd, "release", "1", "--owner", "", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for empty owner, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "owner") && !strings.Contains(errStr, "empty") {
		t.Errorf("expected error about empty owner, got: %q", errStr)
	}
}

func TestReleaseCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	tests := []struct {
		name   string
		nodeID string
	}{
		{"non-numeric", "abc"},
		{"negative", "-1"},
		{"zero", "0"},
		{"invalid format", "1..2"},
		{"wrong root", "2"},
		{"empty", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestReleaseCmd()
			_, err := executeCommand(cmd, "release", tc.nodeID, "--owner", "test-agent", "--dir", tmpDir)

			if err == nil {
				t.Fatalf("expected error for invalid node ID %q, got nil", tc.nodeID)
			}

			errStr := err.Error()
			if !strings.Contains(errStr, "invalid") && !strings.Contains(errStr, "node") {
				t.Errorf("expected invalid node ID error for %q, got: %q", tc.nodeID, errStr)
			}
		})
	}
}

func TestReleaseCmd_ProofNotInitialized(t *testing.T) {
	// Create empty directory without initializing proof
	tmpDir, err := os.MkdirTemp("", "af-release-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newTestReleaseCmd()
	_, err = executeCommand(cmd, "release", "1", "--owner", "test-agent", "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not initialized") && !strings.Contains(errStr, "no proof") {
		t.Errorf("expected 'not initialized' error, got: %q", errStr)
	}
}

func TestReleaseCmd_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	output, err := executeCommand(cmd, "release", "1", "--owner", "test-agent", "--dir", tmpDir, "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check for JSON structure markers
	if !strings.Contains(output, "{") || !strings.Contains(output, "}") {
		t.Errorf("expected JSON output, got: %q", output)
	}

	// Should contain node-related fields
	if !strings.Contains(output, "node") && !strings.Contains(output, "id") {
		t.Errorf("expected JSON to contain node information, got: %q", output)
	}
}

func TestReleaseCmd_Help(t *testing.T) {
	cmd := newTestReleaseCmd()
	output, err := executeCommand(cmd, "release", "--help")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check for expected help content
	expectations := []string{
		"release",
		"--owner", "-o",
		"--dir", "-d",
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help to contain %q, got: %q", exp, output)
		}
	}
}

func TestReleaseCmd_SuccessMessage(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	output, err := executeCommand(cmd, "release", "1", "--owner", "test-agent", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify output indicates node is now available
	if !strings.Contains(output, "available") && !strings.Contains(output, "released") {
		t.Errorf("expected success message to indicate node is available again, got: %q", output)
	}
}

func TestReleaseCmd_ShortFlags(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	output, err := executeCommand(cmd, "release", "1", "-o", "test-agent", "-d", tmpDir)

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	// Verify release succeeded
	if !strings.Contains(output, "released") && !strings.Contains(output, "available") {
		t.Errorf("expected output to indicate node was released, got: %q", output)
	}
}

func TestReleaseCmd_ShortFormatFlag(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	output, err := executeCommand(cmd, "release", "1", "-o", "test-agent", "-d", tmpDir, "-f", "json")

	if err != nil {
		t.Fatalf("expected no error with short format flag, got: %v", err)
	}

	// Check for JSON structure markers
	if !strings.Contains(output, "{") || !strings.Contains(output, "}") {
		t.Errorf("expected JSON output with short flag, got: %q", output)
	}
}

func TestReleaseCmd_NodeStateAfterRelease(t *testing.T) {
	tmpDir, cleanup := setupReleaseTest(t)
	defer cleanup()

	cmd := newTestReleaseCmd()
	_, err := executeCommand(cmd, "release", "1", "--owner", "test-agent", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify the node state changed to available
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID, _ := types.Parse("1")
	node := st.GetNode(nodeID)
	if node == nil {
		t.Fatal("node should exist after release")
	}

	if node.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("expected node workflow state to be 'available', got: %q", node.WorkflowState)
	}
}
