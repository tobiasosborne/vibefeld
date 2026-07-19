//go:build !integration

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

func setupUnvalidateBatchCLIProof(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "proof")

	if err := service.Init(dir, "Root conjecture", "root-author"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}
	svc, err := service.NewProofService(dir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}
	rootID, _ := service.ParseNodeID("1")
	if err := svc.ClaimNode(rootID, "prover-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}
	childID, _ := service.ParseNodeID("1.1")
	if err := svc.RefineNode(rootID, "prover-1", childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode failed: %v", err)
	}
	if err := svc.AcceptNodeWithVerifier(childID, "", "verifier-1", "batch-cli"); err != nil {
		t.Fatalf("AcceptNodeWithVerifier(child) failed: %v", err)
	}
	if err := svc.AcceptNodeWithVerifier(rootID, "", "verifier-1", "batch-cli"); err != nil {
		t.Fatalf("AcceptNodeWithVerifier(root) failed: %v", err)
	}
	return dir
}

func TestUnvalidateCmd_HasBatchFlag(t *testing.T) {
	cmd := newUnvalidateCmd()
	if cmd.Flags().Lookup("batch") == nil {
		t.Error("expected --batch flag to exist")
	}
}

func TestUnvalidateCmd_BatchAndNodeIDMutuallyExclusive(t *testing.T) {
	cmd := newUnvalidateCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"1.1", "--batch", "batch-1", "-y"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when both a node id and --batch are given")
	}
}

func TestUnvalidateCmd_NeitherNodeIDNorBatch_Errors(t *testing.T) {
	cmd := newUnvalidateCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"-y"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when neither a node id nor --batch is given")
	}
}

func TestUnvalidateCmd_Batch_RevokesAllMatchingNodes(t *testing.T) {
	dir := setupUnvalidateBatchCLIProof(t)

	cmd := newUnvalidateCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--batch", "batch-cli", "--dir", dir, "-y"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "2 node(s) unvalidated") {
		t.Errorf("expected output to report 2 nodes unvalidated, got: %s", buf.String())
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	rootID, _ := service.ParseNodeID("1")
	if st.GetNode(rootID).EpistemicState != service.EpistemicPending {
		t.Errorf("expected root back to pending, got %s", st.GetNode(rootID).EpistemicState)
	}
}

func TestUnvalidateCmd_Batch_NotFound_DistinctExitCode(t *testing.T) {
	dir := setupUnvalidateBatchCLIProof(t)

	cmd := newUnvalidateCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--batch", "no-such-batch", "--dir", dir, "-y"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected ErrUnvalidateBatchNotFound")
	}
	if service.ExitCode(err) != 7 {
		t.Errorf("ExitCode(err) = %d, want 7", service.ExitCode(err))
	}
	if !strings.Contains(buf.String(), "Nothing to do") {
		t.Errorf("expected a clean no-op message, got: %s", buf.String())
	}
}

func TestUnvalidateCmd_Batch_JSONFormat(t *testing.T) {
	dir := setupUnvalidateBatchCLIProof(t)

	cmd := newUnvalidateCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--batch", "batch-cli", "--dir", dir, "--format", "json", "-y"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var report service.UnvalidateBatchReport
	if err := json.Unmarshal([]byte(buf.String()), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if report.Count != 2 {
		t.Errorf("Count = %d, want 2", report.Count)
	}
}
