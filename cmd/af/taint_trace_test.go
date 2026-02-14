package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

func nid(s string) types.NodeID {
	id, _ := service.ParseNodeID(s)
	return id
}

func setupTaintTraceTest(t *testing.T) (string, *service.ProofService) {
	t.Helper()
	dir := t.TempDir()
	if err := service.InitProofDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := service.Init(dir, "Test conjecture", "prover1"); err != nil {
		t.Fatal(err)
	}
	svc, err := service.NewProofService(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, svc
}

func refineAndClaim(t *testing.T, svc *service.ProofService, parentID, owner string) {
	t.Helper()
	if err := svc.ClaimNode(nid(parentID), owner, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	children := []service.ChildSpec{
		{Statement: "Step", NodeType: "claim", Inference: "modus_ponens"},
	}
	if _, err := svc.RefineNodeBulk(nid(parentID), owner, children); err != nil {
		t.Fatal(err)
	}
}

// TestTaintTraceCmd_CleanNode tests taint-trace on a fully validated subtree.
func TestTaintTraceCmd_CleanNode(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)

	refineAndClaim(t, svc, "1", "prover1")

	// Accept both 1.1 and 1 — full chain validated
	if err := svc.AcceptNode(nid("1.1")); err != nil {
		t.Fatal(err)
	}
	if err := svc.AcceptNode(nid("1")); err != nil {
		t.Fatal(err)
	}

	cmd := newTaintTraceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1.1", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "clean") {
		t.Errorf("expected 'clean' in output, got: %s", output)
	}
}

// TestTaintTraceCmd_SelfAdmittedNode tests taint-trace on an admitted node
// with a validated root (so self_admitted is the dominant taint).
func TestTaintTraceCmd_SelfAdmittedNode(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)

	refineAndClaim(t, svc, "1", "prover1")

	// Admit both — root admitted so it doesn't block with "children not validated"
	// 1.1 admitted introduces self_admitted taint
	if err := svc.AdmitNode(nid("1.1")); err != nil {
		t.Fatal(err)
	}
	if err := svc.AdmitNode(nid("1")); err != nil {
		t.Fatal(err)
	}

	cmd := newTaintTraceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1.1", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "self_admitted") {
		t.Errorf("expected 'self_admitted' in output, got: %s", output)
	}
	if !strings.Contains(output, "admitted") {
		t.Errorf("expected 'admitted' in output (reason), got: %s", output)
	}
}

// TestTaintTraceCmd_TaintedDescendant tests that a descendant of an admitted node
// shows the taint source.
func TestTaintTraceCmd_TaintedDescendant(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)

	refineAndClaim(t, svc, "1", "prover1")

	// Admit 1.1, then admit root
	if err := svc.AdmitNode(nid("1.1")); err != nil {
		t.Fatal(err)
	}
	if err := svc.AdmitNode(nid("1")); err != nil {
		t.Fatal(err)
	}

	// Refine 1.1 further
	refineAndClaim(t, svc, "1.1", "prover1")

	// Accept 1.1.1 — should be tainted via parent 1.1
	if err := svc.AcceptNode(nid("1.1.1")); err != nil {
		t.Fatal(err)
	}

	cmd := newTaintTraceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1.1.1", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "tainted") {
		t.Errorf("expected 'tainted' in output, got: %s", output)
	}
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected taint source '1.1' in output, got: %s", output)
	}
}

// TestTaintTraceCmd_PendingNode tests taint-trace on a pending (unresolved) node.
func TestTaintTraceCmd_PendingNode(t *testing.T) {
	dir, _ := setupTaintTraceTest(t)

	cmd := newTaintTraceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "unresolved") {
		t.Errorf("expected 'unresolved' in output, got: %s", output)
	}
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' reason in output, got: %s", output)
	}
}

// TestTaintTraceCmd_JSONOutput tests JSON format output.
func TestTaintTraceCmd_JSONOutput(t *testing.T) {
	dir, _ := setupTaintTraceTest(t)

	cmd := newTaintTraceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1", "--dir", dir, "-f", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if _, ok := result["node_id"]; !ok {
		t.Error("expected 'node_id' in JSON output")
	}
	if _, ok := result["taint_state"]; !ok {
		t.Error("expected 'taint_state' in JSON output")
	}
	if _, ok := result["trace"]; !ok {
		t.Error("expected 'trace' in JSON output")
	}
}

// TestTaintTraceCmd_NodeNotFound tests error for non-existent node.
func TestTaintTraceCmd_NodeNotFound(t *testing.T) {
	dir, _ := setupTaintTraceTest(t)

	cmd := newTaintTraceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1.99", "--dir", dir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent node")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// TestTaintTraceCmd_InvalidNodeID tests error for invalid node ID format.
func TestTaintTraceCmd_InvalidNodeID(t *testing.T) {
	dir, _ := setupTaintTraceTest(t)

	cmd := newTaintTraceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"not-a-node", "--dir", dir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid node ID")
	}
}

// TestTaintTraceCmd_NoProof tests error when no proof exists.
func TestTaintTraceCmd_NoProof(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0o755)

	cmd := newTaintTraceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1", "--dir", dir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no proof exists")
	}
}
