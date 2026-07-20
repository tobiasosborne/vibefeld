//go:build integration

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

func newRecordProofTestCmd() *cobra.Command {
	cmd := newTestRootCmd()
	cmd.AddCommand(newRecordProofCmd())
	AddFuzzyMatching(cmd)
	return cmd
}

// setupChallengedRoot inits a proof and raises a blocking challenge on root "1"
// so it is a prover job (record-proof's precondition).
func setupChallengedRoot(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "af-recordproof-test-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Init(tmpDir, "Test conjecture", "root-author"); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	rootID, _ := service.ParseNodeID("1")
	if err := svc.RaiseChallengeWithBatch(rootID, "ch-root", "statement", "prove it", "major", "verifier-1", "gap", "b0"); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	return tmpDir, func() { os.RemoveAll(tmpDir) }
}

func TestRecordProofCmd_RefinesAndDisposesChallenge(t *testing.T) {
	tmpDir, cleanup := setupChallengedRoot(t)
	defer cleanup()

	cmd := newRecordProofTestCmd()
	out, err := executeCommand(cmd, "record-proof", "1",
		"--owner", "prover-1",
		"--children", `[{"statement":"Lemma A"},{"statement":"Uses A","depends":["#0"]}]`,
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("record-proof error: %v (out: %s)", err, out)
	}
	if !strings.Contains(out, "1.1") || !strings.Contains(out, "1.2") {
		t.Errorf("expected children 1.1 and 1.2 in output, got: %q", out)
	}

	// The challenge must be disposed: root is no longer a prover job.
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	rootID, _ := service.ParseNodeID("1")
	if len(st.GetBlockingChallengesForNode(rootID)) != 0 {
		t.Error("root still has an open blocking challenge after record-proof")
	}
	// 1.2 recorded its sibling dependency on 1.1.
	depID, _ := service.ParseNodeID("1.2")
	deps := st.GetNode(depID).Dependencies
	if len(deps) != 1 || deps[0].String() != "1.1" {
		t.Errorf("node 1.2 dependencies = %v, want [1.1]", deps)
	}
}

// TestRecordProofCmd_FreeTextJustification is the GAP-6 live reproduction: a
// prover decomposes a challenged node and justifies a child with a real math
// label ("multiplication_by_positive") that is NOT in af's known inference
// registry. record-proof must accept it, store it VERBATIM, and it must survive
// `af export --graph json` unchanged. (Previously exited 1: invalid --justification.)
func TestRecordProofCmd_FreeTextJustification(t *testing.T) {
	tmpDir, cleanup := setupChallengedRoot(t)
	defer cleanup()

	cmd := newRecordProofTestCmd()
	_, err := executeCommand(cmd, "record-proof", "1",
		"--owner", "prover-1",
		"--children", `[{"statement":"weighted step","inference":"multiplication_by_positive"}]`,
		"--dir", tmpDir,
	)
	if err != nil {
		t.Fatalf("record-proof with free-text justification should succeed, got: %v", err)
	}

	// Stored verbatim on the recorded node.
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	childID, _ := service.ParseNodeID("1.1")
	child := st.GetNode(childID)
	if child == nil {
		t.Fatal("child 1.1 was not recorded")
	}
	if string(child.Inference) != "multiplication_by_positive" {
		t.Errorf("recorded inference = %q, want verbatim %q", child.Inference, "multiplication_by_positive")
	}

	// Survives export verbatim, and does not break export/closure.
	exportCmd := newTestRootCmd()
	exportCmd.AddCommand(newExportCmd())
	out, err := executeCommand(exportCmd, "export", "--graph", "json", "--dir", tmpDir)
	if err != nil {
		t.Fatalf("export --graph json failed on free-text inference: %v", err)
	}
	if !strings.Contains(out, "multiplication_by_positive") {
		t.Errorf("free-text inference did not survive export verbatim; export:\n%s", out)
	}
}

// TestRecordProofCmd_BlankJustificationRejected: an explicitly blank
// (whitespace-only) justification is still rejected — the default only applies
// to an OMITTED justification, which becomes "assumption" (unchanged).
func TestRecordProofCmd_BlankJustificationRejected(t *testing.T) {
	tmpDir, cleanup := setupChallengedRoot(t)
	defer cleanup()

	cmd := newRecordProofTestCmd()
	_, err := executeCommand(cmd, "record-proof", "1",
		"--owner", "prover-1",
		"--children", `[{"statement":"x","inference":"   "}]`,
		"--dir", tmpDir,
	)
	if err == nil {
		t.Fatal("record-proof with a blank (whitespace) justification should be rejected")
	}
}

// TestRecordProofCmd_OmittedJustificationDefaultsAssumption: an OMITTED
// justification still defaults to "assumption" — the value rk's proofless-root
// predicate keys on. A free-text-justified node is never coerced to this.
func TestRecordProofCmd_OmittedJustificationDefaultsAssumption(t *testing.T) {
	tmpDir, cleanup := setupChallengedRoot(t)
	defer cleanup()

	cmd := newRecordProofTestCmd()
	if _, err := executeCommand(cmd, "record-proof", "1",
		"--owner", "prover-1",
		"--children", `[{"statement":"no justification given"}]`,
		"--dir", tmpDir,
	); err != nil {
		t.Fatalf("record-proof error: %v", err)
	}
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	childID, _ := service.ParseNodeID("1.1")
	if got := string(st.GetNode(childID).Inference); got != "assumption" {
		t.Errorf("omitted justification defaulted to %q, want \"assumption\"", got)
	}
}

func TestRecordProofCmd_RejectsNonProverJob(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "af-recordproof-nojob-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	if err := service.Init(tmpDir, "Test conjecture", "root-author"); err != nil {
		t.Fatal(err)
	}
	cmd := newRecordProofTestCmd()
	_, err = executeCommand(cmd, "record-proof", "1",
		"--owner", "prover-1",
		"--children", `[{"statement":"x"}]`,
		"--dir", tmpDir,
	)
	if err == nil {
		t.Fatal("expected error: fresh root is a verifier job, not a prover job")
	}
}
