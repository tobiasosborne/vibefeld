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

// newTestAmendmentsCmd creates a fresh root command with the amendments subcommand for testing.
func newTestAmendmentsCmd() *cobra.Command {
	cmd := newTestRootCmd()
	cmd.AddCommand(newAmendmentsCmd())
	return cmd
}

// executeAmendmentsCommand executes a cobra command with the given arguments and returns output.
func executeAmendmentsCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// setupAmendmentsTest creates a temp directory with an initialized proof.
func setupAmendmentsTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-amendments-test-*")
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

// setupAmendmentsTestWithAmendments creates a proof with amendments on node 1.
func setupAmendmentsTestWithAmendments(t *testing.T) (string, func()) {
	t.Helper()

	proofDir, cleanup := setupAmendmentsTest(t)

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

	// First amendment
	event1 := ledger.NewNodeAmended(nodeID, "Test conjecture: P implies Q", "Revised: P implies Q under conditions C", "prover-1")
	if _, err := ldg.Append(event1); err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Second amendment
	event2 := ledger.NewNodeAmended(nodeID, "Revised: P implies Q under conditions C", "Final: P implies Q when C holds and D is satisfied", "prover-2")
	if _, err := ldg.Append(event2); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return proofDir, cleanup
}

// =============================================================================
// Tests
// =============================================================================

func TestAmendmentsCmd_NoAmendments(t *testing.T) {
	proofDir, cleanup := setupAmendmentsTest(t)
	defer cleanup()

	cmd := newTestAmendmentsCmd()
	output, err := executeAmendmentsCommand(cmd, "amendments", "--dir", proofDir, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "no amendments") {
		t.Errorf("expected 'no amendments' message, got: %s", output)
	}
	if !strings.Contains(output, "Test conjecture") {
		t.Errorf("expected current statement in output, got: %s", output)
	}
}

func TestAmendmentsCmd_WithAmendments(t *testing.T) {
	proofDir, cleanup := setupAmendmentsTestWithAmendments(t)
	defer cleanup()

	cmd := newTestAmendmentsCmd()
	output, err := executeAmendmentsCommand(cmd, "amendments", "--dir", proofDir, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "2 amendment(s)") {
		t.Errorf("expected '2 amendment(s)', got: %s", output)
	}
	if !strings.Contains(output, "[v0] Original") {
		t.Errorf("expected '[v0] Original', got: %s", output)
	}
	if !strings.Contains(output, "Test conjecture: P implies Q") {
		t.Errorf("expected original statement, got: %s", output)
	}
	if !strings.Contains(output, "[v1]") {
		t.Errorf("expected '[v1]', got: %s", output)
	}
	if !strings.Contains(output, "prover-1") {
		t.Errorf("expected 'prover-1', got: %s", output)
	}
	if !strings.Contains(output, "[v2]") {
		t.Errorf("expected '[v2]', got: %s", output)
	}
	if !strings.Contains(output, "Final:") {
		t.Errorf("expected final statement, got: %s", output)
	}
}

func TestAmendmentsCmd_JSONFormat(t *testing.T) {
	proofDir, cleanup := setupAmendmentsTestWithAmendments(t)
	defer cleanup()

	cmd := newTestAmendmentsCmd()
	output, err := executeAmendmentsCommand(cmd, "amendments", "--dir", proofDir, "-f", "json", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result amendmentsOutputJSON
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	if result.NodeID != "1" {
		t.Errorf("expected node_id '1', got %q", result.NodeID)
	}
	if result.TotalAmendments != 2 {
		t.Errorf("expected total_amendments 2, got %d", result.TotalAmendments)
	}
	if len(result.Versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(result.Versions))
	}
	if result.Versions[0].Version != 0 {
		t.Errorf("expected first version to be 0, got %d", result.Versions[0].Version)
	}
	if result.Versions[0].Statement != "Test conjecture: P implies Q" {
		t.Errorf("expected original statement in version 0, got %q", result.Versions[0].Statement)
	}
}

func TestAmendmentsCmd_JSONNoAmendments(t *testing.T) {
	proofDir, cleanup := setupAmendmentsTest(t)
	defer cleanup()

	cmd := newTestAmendmentsCmd()
	output, err := executeAmendmentsCommand(cmd, "amendments", "--dir", proofDir, "-f", "json", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result amendmentsOutputJSON
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, output)
	}

	if result.TotalAmendments != 0 {
		t.Errorf("expected 0 amendments, got %d", result.TotalAmendments)
	}
	if len(result.Versions) != 1 {
		t.Errorf("expected 1 version (current), got %d", len(result.Versions))
	}
}

func TestAmendmentsCmd_InvalidNodeID(t *testing.T) {
	proofDir, cleanup := setupAmendmentsTest(t)
	defer cleanup()

	cmd := newTestAmendmentsCmd()
	_, err := executeAmendmentsCommand(cmd, "amendments", "--dir", proofDir, "invalid!!")
	if err == nil {
		t.Fatal("expected error for invalid node ID")
	}
}

func TestAmendmentsCmd_NodeNotFound(t *testing.T) {
	proofDir, cleanup := setupAmendmentsTest(t)
	defer cleanup()

	cmd := newTestAmendmentsCmd()
	_, err := executeAmendmentsCommand(cmd, "amendments", "--dir", proofDir, "1.99")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestAmendmentsCmd_InvalidFormat(t *testing.T) {
	proofDir, cleanup := setupAmendmentsTest(t)
	defer cleanup()

	cmd := newTestAmendmentsCmd()
	_, err := executeAmendmentsCommand(cmd, "amendments", "--dir", proofDir, "-f", "xml", "1")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}
