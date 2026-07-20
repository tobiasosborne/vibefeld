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
