//go:build !integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupGraphFixtureProof initializes a proof with a root node and one child
// (via the service layer, not the CLI, to keep this focused on export
// behavior) and returns the proof directory.
func setupGraphFixtureProof(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	proofDir := filepath.Join(dir, "proof")

	if err := service.Init(proofDir, "Root conjecture text", "prover-1"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	rootID, err := service.ParseNodeID("1")
	if err != nil {
		t.Fatalf("ParseNodeID failed: %v", err)
	}
	if err := svc.ClaimNode(rootID, "prover-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	childID, err := service.ParseNodeID("1.1")
	if err != nil {
		t.Fatalf("ParseNodeID failed: %v", err)
	}
	if err := svc.RefineNode(rootID, "prover-1", childID, "claim", "First lemma text", "modus_ponens"); err != nil {
		t.Fatalf("RefineNode failed: %v", err)
	}

	return proofDir
}

// =============================================================================
// Flag tests
// =============================================================================

func TestExportCmd_GraphFlagExists(t *testing.T) {
	cmd := newExportCmd()
	if cmd.Flags().Lookup("graph") == nil {
		t.Error("expected export command to have a --graph flag")
	}
}

func TestExportCmd_GraphFlagDefaultEmpty(t *testing.T) {
	cmd := newExportCmd()
	f := cmd.Flags().Lookup("graph")
	if f == nil {
		t.Fatal("expected --graph flag to exist")
	}
	if f.DefValue != "" {
		t.Errorf("expected default --graph value to be empty (disabled), got %q", f.DefValue)
	}
}

func TestExportCmd_HelpMentionsGraph(t *testing.T) {
	cmd := newTestExportCmd()
	output, err := executeExportCommand(cmd, "export", "--help")
	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}
	if !strings.Contains(output, "--graph") {
		t.Errorf("expected help output to mention --graph, got: %q", output)
	}
}

// =============================================================================
// Behavior tests
// =============================================================================

func TestExportCmd_GraphInvalidValue(t *testing.T) {
	cmd := newTestExportCmd()
	_, err := executeExportCommand(cmd, "export", "--graph", "yaml", "--dir", "/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for invalid --graph value, got nil")
	}
	if !strings.Contains(err.Error(), "graph") {
		t.Errorf("expected error to mention 'graph', got: %v", err)
	}
}

func TestExportCmd_GraphJSON_Structure(t *testing.T) {
	proofDir := setupGraphFixtureProof(t)

	cmd := newTestExportCmd()
	output, err := executeExportCommand(cmd, "export", "--graph", "json", "--dir", proofDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc service.GraphExport
	if err := json.Unmarshal([]byte(output), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	if doc.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want %q", doc.SchemaVersion, "1")
	}
	if len(doc.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(doc.Nodes))
	}

	foundRoot, foundChild := false, false
	for _, n := range doc.Nodes {
		if n.ID == "1" {
			foundRoot = true
			if n.Statement != "Root conjecture text" {
				t.Errorf("root statement = %q, want contract byte-match text", n.Statement)
			}
			if len(n.ChildIDs) != 1 || n.ChildIDs[0] != "1.1" {
				t.Errorf("root child_ids = %v, want [1.1]", n.ChildIDs)
			}
		}
		if n.ID == "1.1" {
			foundChild = true
			if n.ParentID != "1" {
				t.Errorf("1.1 parent_id = %q, want \"1\"", n.ParentID)
			}
			if n.Statement != "First lemma text" {
				t.Errorf("1.1 statement = %q, want contract byte-match text", n.Statement)
			}
		}
	}
	if !foundRoot || !foundChild {
		t.Errorf("expected both root and child nodes present, foundRoot=%v foundChild=%v", foundRoot, foundChild)
	}
}

// TestExportCmd_GraphIsReadOnly asserts the ledger file is byte-identical
// before and after `af export --graph json` runs — the verb must never
// mutate the ledger.
func TestExportCmd_GraphIsReadOnly(t *testing.T) {
	proofDir := setupGraphFixtureProof(t)
	ledgerPath := filepath.Join(proofDir, "ledger")

	before := readLedgerBytes(t, ledgerPath)

	cmd := newTestExportCmd()
	if _, err := executeExportCommand(cmd, "export", "--graph", "json", "--dir", proofDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after := readLedgerBytes(t, ledgerPath)

	if string(before) != string(after) {
		t.Error("ledger contents changed after `af export --graph json` — export must be read-only")
	}
}

// readLedgerBytes reads and concatenates every file under the ledger
// directory (or a single ledger file) in a stable, sorted order.
func readLedgerBytes(t *testing.T, ledgerPath string) []byte {
	t.Helper()

	info, err := os.Stat(ledgerPath)
	if err != nil {
		t.Fatalf("failed to stat ledger path %q: %v", ledgerPath, err)
	}

	if !info.IsDir() {
		b, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatalf("failed to read ledger file: %v", err)
		}
		return b
	}

	var all []byte
	entries, err := os.ReadDir(ledgerPath)
	if err != nil {
		t.Fatalf("failed to read ledger dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(ledgerPath, name))
		if err != nil {
			t.Fatalf("failed to read ledger entry %q: %v", name, err)
		}
		all = append(all, b...)
	}
	return all
}

// TestExportCmd_GraphDeterministic asserts two successive graph exports of
// the same unchanged proof are byte-identical.
func TestExportCmd_GraphDeterministic(t *testing.T) {
	proofDir := setupGraphFixtureProof(t)

	cmd1 := newTestExportCmd()
	out1, err := executeExportCommand(cmd1, "export", "--graph", "json", "--dir", proofDir)
	if err != nil {
		t.Fatalf("first export failed: %v", err)
	}

	cmd2 := newTestExportCmd()
	out2, err := executeExportCommand(cmd2, "export", "--graph", "json", "--dir", proofDir)
	if err != nil {
		t.Fatalf("second export failed: %v", err)
	}

	if out1 != out2 {
		t.Errorf("graph export is not deterministic:\nrun 1: %s\nrun 2: %s", out1, out2)
	}
}

// TestExportCmd_GraphOutputFile verifies --output writes the graph JSON to
// a file rather than stdout, same convention as the markdown/latex path.
func TestExportCmd_GraphOutputFile(t *testing.T) {
	proofDir := setupGraphFixtureProof(t)
	outFile := filepath.Join(t.TempDir(), "graph.json")

	cmd := newTestExportCmd()
	if _, err := executeExportCommand(cmd, "export", "--graph", "json", "--dir", proofDir, "--output", outFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}

	var doc service.GraphExport
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("output file is not valid JSON: %v", err)
	}
	if doc.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want 1", doc.SchemaVersion)
	}
}
