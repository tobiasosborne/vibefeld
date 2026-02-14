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
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

func newTestCriticalPathCmd() *cobra.Command {
	cmd := newTestRootCmd()
	cmd.AddCommand(newCriticalPathCmd())
	return cmd
}

func executeCPCommand(root *cobra.Command, args ...string) (string, error) {
	render.DisableColor()
	defer render.EnableColor()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// setupCriticalPathTest creates a proof with a deeper tree for critical path testing:
// 1 → 1.1 → 1.1.1 → 1.1.1.1 (deep pending chain)
// 1 → 1.2 (short branch)
func setupCriticalPathTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-cp-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")
	if err := service.InitProofDir(proofDir); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	err = service.Init(proofDir, "Test conjecture", "test-author")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create nodes for a deeper tree
	ids := []string{"1.1", "1.2", "1.1.1", "1.1.1.1"}
	stmts := []string{"First child", "Second child", "Grandchild", "Great-grandchild"}

	for i, idStr := range ids {
		id, _ := service.ParseNodeID(idStr)
		n, _ := node.NewNode(id, "claim", stmts[i], "modus_ponens")
		if _, err := ldg.Append(ledger.NewNodeCreated(*n)); err != nil {
			os.RemoveAll(tmpDir)
			t.Fatal(err)
		}
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

func TestCriticalPathCmd_Basic(t *testing.T) {
	proofDir, cleanup := setupCriticalPathTest(t)
	defer cleanup()

	cmd := newTestCriticalPathCmd()
	output, err := executeCPCommand(cmd, "critical-path", "--dir", proofDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Critical Paths") {
		t.Errorf("expected header, got: %s", output)
	}
	if !strings.Contains(output, "Path 1") {
		t.Errorf("expected at least one path, got: %s", output)
	}
	// The deepest path (1 → 1.1 → 1.1.1 → 1.1.1.1) should be path 1
	if !strings.Contains(output, "1.1.1.1") {
		t.Errorf("expected deepest node in output, got: %s", output)
	}
}

func TestCriticalPathCmd_JSON(t *testing.T) {
	proofDir, cleanup := setupCriticalPathTest(t)
	defer cleanup()

	cmd := newTestCriticalPathCmd()
	output, err := executeCPCommand(cmd, "critical-path", "--dir", proofDir, "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result struct {
		CriticalPaths []CriticalPath `json:"critical_paths"`
		Count         int            `json:"count"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}

	if result.Count == 0 {
		t.Fatal("expected at least one critical path")
	}

	// First path should be the longest unvalidated chain
	first := result.CriticalPaths[0]
	if first.UnvalidatedCount < 2 {
		t.Errorf("expected multiple unvalidated nodes on worst path, got %d", first.UnvalidatedCount)
	}
}

func TestCriticalPathCmd_LimitFlag(t *testing.T) {
	proofDir, cleanup := setupCriticalPathTest(t)
	defer cleanup()

	cmd := newTestCriticalPathCmd()
	output, err := executeCPCommand(cmd, "critical-path", "--dir", proofDir, "--limit", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only show Path 1, not Path 2
	if !strings.Contains(output, "Path 1") {
		t.Errorf("expected Path 1, got: %s", output)
	}
	if strings.Contains(output, "Path 2") {
		t.Errorf("should not show Path 2 with --limit 1, got: %s", output)
	}
}

func TestCriticalPathCmd_FocusFlag(t *testing.T) {
	proofDir, cleanup := setupCriticalPathTest(t)
	defer cleanup()

	cmd := newTestCriticalPathCmd()
	output, err := executeCPCommand(cmd, "critical-path", "--dir", proofDir, "--focus", "1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "subtree: 1.1") {
		t.Errorf("expected subtree header, got: %s", output)
	}
	// Should NOT contain node 1.2 (it's outside the focus subtree)
	if strings.Contains(output, "Second child") {
		t.Errorf("should not contain sibling branch, got: %s", output)
	}
}

func TestCriticalPathCmd_InvalidFocus(t *testing.T) {
	proofDir, cleanup := setupCriticalPathTest(t)
	defer cleanup()

	cmd := newTestCriticalPathCmd()
	_, err := executeCPCommand(cmd, "critical-path", "--dir", proofDir, "--focus", "bad!!")
	if err == nil {
		t.Fatal("expected error for invalid focus node ID")
	}
}

func TestCriticalPathCmd_NoProof(t *testing.T) {
	dir := t.TempDir()
	if err := service.InitProofDir(dir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	cmd := newTestCriticalPathCmd()
	output, err := executeCPCommand(cmd, "critical-path", "--dir", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "No proof initialized") {
		t.Errorf("expected 'No proof initialized', got: %s", output)
	}
}

func TestCriticalPathCmd_Recommendation(t *testing.T) {
	proofDir, cleanup := setupCriticalPathTest(t)
	defer cleanup()

	cmd := newTestCriticalPathCmd()
	output, err := executeCPCommand(cmd, "critical-path", "--dir", proofDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Recommendation") {
		t.Errorf("expected recommendation section, got: %s", output)
	}
	if !strings.Contains(output, "Focus on node") {
		t.Errorf("expected focus recommendation, got: %s", output)
	}
}

func TestCriticalPathCmd_InvalidFormat(t *testing.T) {
	proofDir, cleanup := setupCriticalPathTest(t)
	defer cleanup()

	cmd := newTestCriticalPathCmd()
	_, err := executeCPCommand(cmd, "critical-path", "--dir", proofDir, "--format", "xml")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}
