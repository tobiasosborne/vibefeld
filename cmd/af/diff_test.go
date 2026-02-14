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
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestDiffCmd creates a fresh root command with the diff subcommand for testing.
func newTestDiffCmd() *cobra.Command {
	cmd := newTestRootCmd()
	cmd.AddCommand(newDiffCmd())
	return cmd
}

// executeDiffCommand executes a cobra command with the given arguments and returns output.
func executeDiffCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// setupDiffTest creates a temp directory with an initialized proof.
func setupDiffTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-diff-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")

	if err := service.InitProofDir(proofDir); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	err = service.Init(proofDir, "Test conjecture: P implies Q", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// setupDiffTestWithAmendments creates a proof with 2 amendments on node 1.
func setupDiffTestWithAmendments(t *testing.T) (string, func()) {
	t.Helper()

	proofDir, cleanup := setupDiffTest(t)

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	nodeID, _ := service.ParseNodeID("1")

	event1 := ledger.NewNodeAmended(nodeID, "Test conjecture: P implies Q", "Revised: P implies Q under conditions C", "prover-1")
	if _, err := ldg.Append(event1); err != nil {
		cleanup()
		t.Fatal(err)
	}

	event2 := ledger.NewNodeAmended(nodeID, "Revised: P implies Q under conditions C", "Final: P implies Q when C holds", "prover-2")
	if _, err := ldg.Append(event2); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return proofDir, cleanup
}

// =============================================================================
// Tests
// =============================================================================

func TestDiffCmd_NoAmendments(t *testing.T) {
	proofDir, cleanup := setupDiffTest(t)
	defer cleanup()

	cmd := newTestDiffCmd()
	output, err := executeDiffCommand(cmd, "diff", "--dir", proofDir, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "no amendments") {
		t.Errorf("expected 'no amendments' message, got: %s", output)
	}
}

func TestDiffCmd_DefaultPreviousVsCurrent(t *testing.T) {
	proofDir, cleanup := setupDiffTestWithAmendments(t)
	defer cleanup()

	cmd := newTestDiffCmd()
	output, err := executeDiffCommand(cmd, "diff", "--dir", proofDir, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Default shows previous (v1) vs current (v2)
	if !strings.Contains(output, "v1") && !strings.Contains(output, "v2") {
		t.Errorf("expected version numbers in output, got: %s", output)
	}
	if !strings.Contains(output, "-") || !strings.Contains(output, "+") {
		t.Errorf("expected diff markers (- and +), got: %s", output)
	}
}

func TestDiffCmd_VersionFlag(t *testing.T) {
	proofDir, cleanup := setupDiffTestWithAmendments(t)
	defer cleanup()

	cmd := newTestDiffCmd()
	output, err := executeDiffCommand(cmd, "diff", "--dir", proofDir, "--version", "0", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show diff from v0 (original) to current (v2)
	if !strings.Contains(output, "v0") {
		t.Errorf("expected v0 in output, got: %s", output)
	}
	if !strings.Contains(output, "Test conjecture") {
		t.Errorf("expected original statement in diff, got: %s", output)
	}
}

func TestDiffCmd_AllFlag(t *testing.T) {
	proofDir, cleanup := setupDiffTestWithAmendments(t)
	defer cleanup()

	cmd := newTestDiffCmd()
	output, err := executeDiffCommand(cmd, "diff", "--dir", proofDir, "--all", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "2 diff(s)") {
		t.Errorf("expected '2 diff(s)', got: %s", output)
	}
	if !strings.Contains(output, "v0") {
		t.Errorf("expected v0 in output, got: %s", output)
	}
	if !strings.Contains(output, "v1") {
		t.Errorf("expected v1 in output, got: %s", output)
	}
}

func TestDiffCmd_JSONFormat(t *testing.T) {
	proofDir, cleanup := setupDiffTestWithAmendments(t)
	defer cleanup()

	cmd := newTestDiffCmd()
	output, err := executeDiffCommand(cmd, "diff", "--dir", proofDir, "-f", "json", "--all", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result diffOutputJSON
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, output)
	}

	if result.NodeID != "1" {
		t.Errorf("expected node_id '1', got %q", result.NodeID)
	}
	if len(result.Diffs) != 2 {
		t.Errorf("expected 2 diffs, got %d", len(result.Diffs))
	}
	if !result.Diffs[0].Changed {
		t.Error("expected first diff to show changes")
	}
}

func TestDiffCmd_SinceChallenge(t *testing.T) {
	proofDir, cleanup := setupDiffTest(t)
	defer cleanup()

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatal(err)
	}

	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID, _ := service.ParseNodeID("1")

	// Raise a challenge before any amendments
	chalEvent := ledger.NewChallengeRaised("ch-test-1", nodeID, "statement", "Statement is vague")
	if _, err := ldg.Append(chalEvent); err != nil {
		t.Fatal(err)
	}

	// Amend the node after the challenge
	amendEvent := ledger.NewNodeAmended(nodeID, "Test conjecture: P implies Q", "Clarified: For all x, P(x) implies Q(x)", "prover-1")
	if _, err := ldg.Append(amendEvent); err != nil {
		t.Fatal(err)
	}

	cmd := newTestDiffCmd()
	output, err := executeDiffCommand(cmd, "diff", "--dir", proofDir, "--since-challenge", "ch-test-1", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "since challenge ch-test-1") {
		t.Errorf("expected 'since challenge' header, got: %s", output)
	}
	if !strings.Contains(output, "Clarified") {
		t.Errorf("expected new statement in diff, got: %s", output)
	}
}

func TestDiffCmd_SinceChallengeNotFound(t *testing.T) {
	proofDir, cleanup := setupDiffTestWithAmendments(t)
	defer cleanup()

	cmd := newTestDiffCmd()
	_, err := executeDiffCommand(cmd, "diff", "--dir", proofDir, "--since-challenge", "ch-nonexistent", "1")
	if err == nil {
		t.Fatal("expected error for nonexistent challenge")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestDiffCmd_InvalidNodeID(t *testing.T) {
	proofDir, cleanup := setupDiffTest(t)
	defer cleanup()

	cmd := newTestDiffCmd()
	_, err := executeDiffCommand(cmd, "diff", "--dir", proofDir, "invalid!!")
	if err == nil {
		t.Fatal("expected error for invalid node ID")
	}
}

func TestDiffCmd_NodeNotFound(t *testing.T) {
	proofDir, cleanup := setupDiffTest(t)
	defer cleanup()

	cmd := newTestDiffCmd()
	_, err := executeDiffCommand(cmd, "diff", "--dir", proofDir, "99")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestDiffCmd_InvalidVersion(t *testing.T) {
	proofDir, cleanup := setupDiffTestWithAmendments(t)
	defer cleanup()

	cmd := newTestDiffCmd()
	_, err := executeDiffCommand(cmd, "diff", "--dir", proofDir, "--version", "5", "1")
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}
