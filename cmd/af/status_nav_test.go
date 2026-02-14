//go:build integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

func newTestStatusNavCmd() *cobra.Command {
	cmd := newTestRootCmd()
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newPathCmd())
	cmd.AddCommand(newNearbyCmd())
	return cmd
}

func executeNavCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// setupNavTest creates a proof with a small tree: 1 → 1.1, 1.2 → 1.1.1
func setupNavTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-nav-test-*")
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

	// Add child nodes via ledger events directly
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

	id11, _ := service.ParseNodeID("1.1")
	id12, _ := service.ParseNodeID("1.2")
	id111, _ := service.ParseNodeID("1.1.1")

	// Create node 1.1
	n11, _ := node.NewNode(id11, "claim", "First child claim", "modus_ponens")
	if _, err := ldg.Append(ledger.NewNodeCreated(*n11)); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create node 1.2
	n12, _ := node.NewNode(id12, "claim", "Second child claim", "modus_ponens")
	if _, err := ldg.Append(ledger.NewNodeCreated(*n12)); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create node 1.1.1
	n111, _ := node.NewNode(id111, "claim", "Grandchild claim", "modus_ponens")
	if _, err := ldg.Append(ledger.NewNodeCreated(*n111)); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// =============================================================================
// Status Navigation Tests
// =============================================================================

func TestStatusCmd_FocusFlag(t *testing.T) {
	proofDir, cleanup := setupNavTest(t)
	defer cleanup()

	cmd := newTestStatusNavCmd()
	output, err := executeNavCommand(cmd, "status", "--dir", proofDir, "--focus", "1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "subtree: 1.1") {
		t.Errorf("expected subtree header, got: %s", output)
	}
	// Should contain 1.1 and 1.1.1 but NOT 1.2
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected node 1.1 in output, got: %s", output)
	}
	if strings.Contains(output, "Second child") {
		t.Errorf("should NOT contain sibling 1.2's statement, got: %s", output)
	}
}

func TestStatusCmd_DepthFlag(t *testing.T) {
	proofDir, cleanup := setupNavTest(t)
	defer cleanup()

	cmd := newTestStatusNavCmd()
	output, err := executeNavCommand(cmd, "status", "--dir", proofDir, "--depth", "2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Depth 2: should show 1, 1.1, 1.2 but NOT 1.1.1 (depth 3)
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected node 1.1 at depth 2, got: %s", output)
	}
	if strings.Contains(output, "Grandchild") {
		t.Errorf("should NOT contain grandchild (depth 3), got: %s", output)
	}
}

func TestStatusCmd_CompactFlag(t *testing.T) {
	proofDir, cleanup := setupNavTest(t)
	defer cleanup()

	cmd := newTestStatusNavCmd()
	output, err := executeNavCommand(cmd, "status", "--dir", proofDir, "--compact")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Compact mode should not include the legend
	if strings.Contains(output, "Legend") {
		t.Errorf("compact mode should skip legend, got: %s", output)
	}
	// Should still contain node IDs
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected node 1.1 in compact output, got: %s", output)
	}
}

func TestStatusCmd_FocusInvalidNode(t *testing.T) {
	proofDir, cleanup := setupNavTest(t)
	defer cleanup()

	cmd := newTestStatusNavCmd()
	_, err := executeNavCommand(cmd, "status", "--dir", proofDir, "--focus", "bad!!")
	if err == nil {
		t.Fatal("expected error for invalid focus node ID")
	}
}

// =============================================================================
// Path Command Tests
// =============================================================================

func TestPathCmd_ShowsAncestry(t *testing.T) {
	proofDir, cleanup := setupNavTest(t)
	defer cleanup()

	cmd := newTestStatusNavCmd()
	output, err := executeNavCommand(cmd, "path", "--dir", proofDir, "1.1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show: 1 [...] → 1.1 [...] → 1.1.1 [...]
	if !strings.Contains(output, "→") {
		t.Errorf("expected arrow separators, got: %s", output)
	}
	if !strings.Contains(output, "1.1.1") {
		t.Errorf("expected target node in path, got: %s", output)
	}
}

func TestPathCmd_RootNode(t *testing.T) {
	proofDir, cleanup := setupNavTest(t)
	defer cleanup()

	cmd := newTestStatusNavCmd()
	output, err := executeNavCommand(cmd, "path", "--dir", proofDir, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Root has no ancestors, just itself
	if !strings.Contains(output, "1 [") {
		t.Errorf("expected root node, got: %s", output)
	}
}

func TestPathCmd_NodeNotFound(t *testing.T) {
	proofDir, cleanup := setupNavTest(t)
	defer cleanup()

	cmd := newTestStatusNavCmd()
	_, err := executeNavCommand(cmd, "path", "--dir", proofDir, "1.99")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

// =============================================================================
// Nearby Command Tests
// =============================================================================

func TestNearbyCmd_ShowsNeighborhood(t *testing.T) {
	proofDir, cleanup := setupNavTest(t)
	defer cleanup()

	cmd := newTestStatusNavCmd()
	output, err := executeNavCommand(cmd, "nearby", "--dir", proofDir, "1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Parent:") {
		t.Errorf("expected Parent section, got: %s", output)
	}
	if !strings.Contains(output, "Node:") {
		t.Errorf("expected Node section, got: %s", output)
	}
	if !strings.Contains(output, "Siblings") {
		t.Errorf("expected Siblings section, got: %s", output)
	}
	if !strings.Contains(output, "Children") {
		t.Errorf("expected Children section, got: %s", output)
	}
}

func TestNearbyCmd_LeafNode(t *testing.T) {
	proofDir, cleanup := setupNavTest(t)
	defer cleanup()

	cmd := newTestStatusNavCmd()
	output, err := executeNavCommand(cmd, "nearby", "--dir", proofDir, "1.1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "leaf node") {
		t.Errorf("expected 'leaf node' for node with no children, got: %s", output)
	}
}

func TestNearbyCmd_NodeNotFound(t *testing.T) {
	proofDir, cleanup := setupNavTest(t)
	defer cleanup()

	cmd := newTestStatusNavCmd()
	_, err := executeNavCommand(cmd, "nearby", "--dir", proofDir, "1.99")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}
