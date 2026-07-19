//go:build !integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// setupVerdictsCLIProof initializes a proof with root "1" (author
// "root-author") claimed and refined into child "1.1" (author "prover-1"),
// mirroring internal/service's own verdicts fixture but built through the
// service layer for a CLI-level test.
func setupVerdictsCLIProof(t *testing.T) string {
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
	return dir
}

func writeVerdictFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "verdicts.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write verdict file: %v", err)
	}
	return path
}

func TestVerdictsApplyCmd_FlagsExist(t *testing.T) {
	cmd := newVerdictsApplyCmd()
	for _, name := range []string{"dir", "format"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("expected --%s flag to exist", name)
		}
	}
}

func TestVerdictsApplyCmd_RequiresExactlyOneArg(t *testing.T) {
	cmd := newVerdictsApplyCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for two args")
	}
}

func TestVerdictsApplyCmd_AllApplied_ExitsZero(t *testing.T) {
	dir := setupVerdictsCLIProof(t)
	file := writeVerdictFile(t, `{
		"schema_version": "1", "batch_id": "batch-1", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "child first"},
			{"node": "1", "verdict": "accept", "reason": "parent second"}
		]
	}`)

	cmd := newVerdictsApplyCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{file, "--dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error (all applied), got: %v", err)
	}
	if !strings.Contains(buf.String(), "2 applied") {
		t.Errorf("expected output to report 2 applied, got: %s", buf.String())
	}
}

func TestVerdictsApplyCmd_PartiallyApplied_ReturnsError(t *testing.T) {
	dir := setupVerdictsCLIProof(t)
	file := writeVerdictFile(t, `{
		"schema_version": "1", "batch_id": "batch-1", "verified_by": "verifier-1",
		"items": [
			{"node": "1", "verdict": "accept", "reason": "parent before child"},
			{"node": "1.1", "verdict": "accept", "reason": "child second"}
		]
	}`)

	cmd := newVerdictsApplyCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{file, "--dir", dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a non-nil error for a partially-applied batch")
	}
	if service.ExitCode(err) != 5 {
		t.Errorf("ExitCode(err) = %d, want 5", service.ExitCode(err))
	}
}

func TestVerdictsApplyCmd_InvalidFile_ExitsThree(t *testing.T) {
	dir := setupVerdictsCLIProof(t)
	file := writeVerdictFile(t, `{not valid json`)

	cmd := newVerdictsApplyCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{file, "--dir", dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid file")
	}
	if service.ExitCode(err) != 3 {
		t.Errorf("ExitCode(err) = %d, want 3", service.ExitCode(err))
	}
}

func TestVerdictsApplyCmd_MissingFile_ExitsThree(t *testing.T) {
	dir := setupVerdictsCLIProof(t)

	cmd := newVerdictsApplyCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{filepath.Join(dir, "does-not-exist.json"), "--dir", dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if service.ExitCode(err) != 3 {
		t.Errorf("ExitCode(err) = %d, want 3", service.ExitCode(err))
	}
}

func TestVerdictsApplyCmd_JSONFormat_ProducesParsableReport(t *testing.T) {
	dir := setupVerdictsCLIProof(t)
	file := writeVerdictFile(t, `{
		"schema_version": "1", "batch_id": "batch-json", "verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "ok"}
		]
	}`)

	cmd := newVerdictsApplyCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{file, "--dir", dir, "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var report service.VerdictReport
	if err := json.Unmarshal([]byte(buf.String()), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if report.BatchID != "batch-json" {
		t.Errorf("BatchID = %q, want batch-json", report.BatchID)
	}
	if report.Applied != 1 {
		t.Errorf("Applied = %d, want 1", report.Applied)
	}
}

func TestVerdictsCmd_HasApplySubcommand(t *testing.T) {
	cmd := newVerdictsCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "apply" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'af verdicts' to have an 'apply' subcommand")
	}
}
