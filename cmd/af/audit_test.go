package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/audit"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/service"
)

// TestAuditCmd_JSONSchemaVersion runs the command in JSON mode on a fresh
// workspace and asserts the report carries schema_version 1.
func TestAuditCmd_JSONSchemaVersion(t *testing.T) {
	dir, _ := setupTaintTraceTest(t)

	cmd := newAuditCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--dir", dir, "-f", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit failed: %v\n%s", err, buf.String())
	}
	var report struct {
		SchemaVersion int  `json:"schema_version"`
		Strict        bool `json:"strict"`
		Passed        bool `json:"passed"`
	}
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("audit JSON invalid: %v\n%s", err, buf.String())
	}
	if report.SchemaVersion != audit.SchemaVersion {
		t.Fatalf("schema_version = %d, want %d", report.SchemaVersion, audit.SchemaVersion)
	}
	if report.Strict || !report.Passed {
		t.Fatalf("non-strict report should pass: strict=%v passed=%v", report.Strict, report.Passed)
	}
}

// TestAuditCmd_StrictExit3 creates a strict-current finding (a validated node
// with an open blocking challenge) and asserts --strict exits 3 with
// AUDIT_FAILED.
func TestAuditCmd_StrictExit3(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)
	if err := svc.AcceptNodeWithVerifier(nid("1"), "", "verifier-1", ""); err != nil {
		t.Fatalf("accept root: %v", err)
	}
	if err := svc.RaiseChallengeWithBatch(nid("1"), "c-audit-1", "statement", "still wrong", "major", "verifier-1", "", ""); err != nil {
		t.Fatalf("challenge: %v", err)
	}

	cmd := newAuditCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--dir", dir, "--strict", "-f", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected strict audit to fail\n%s", buf.String())
	}
	if got := aferrors.Code(err); got != aferrors.AUDIT_FAILED {
		t.Fatalf("code = %v, want AUDIT_FAILED (%v)", got, err)
	}
	if got := aferrors.ExitCode(err); got != 3 {
		t.Fatalf("exit code = %d, want 3", got)
	}
	if !strings.Contains(buf.String(), `"passed": false`) {
		t.Fatalf("report should show passed:false\n%s", buf.String())
	}

	// Without --strict the same workspace exits 0.
	cmd2 := newAuditCmd()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetErr(buf2)
	cmd2.SetArgs([]string{"--dir", dir})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("non-strict audit failed: %v\n%s", err, buf2.String())
	}
}

// TestAuditCmd_FiltersAndLimit exercises the CLI-level code/status/limit flags.
func TestAuditCmd_FiltersAndLimit(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)
	if err := svc.ClaimNode(nid("1"), "prover1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"1.1", "1.2", "1.3"} {
		if err := svc.Refine(service.RefineSpec{
			ParentID: nid("1"), Owner: "prover1", ChildID: nid(id),
			NodeType: "claim", Statement: "step " + id, Inference: "modus_ponens",
		}); err != nil {
			t.Fatalf("refine %s: %v", id, err)
		}
	}
	for _, id := range []string{"1.1", "1.2", "1.3"} {
		if err := svc.AcceptNodeWithVerifier(nid(id), "", "verifier-1", ""); err != nil {
			t.Fatalf("accept %s: %v", id, err)
		}
		if err := svc.RaiseChallengeWithBatch(nid(id), "c-"+id, "statement", "still wrong", "major", "verifier-1", "", ""); err != nil {
			t.Fatalf("challenge %s: %v", id, err)
		}
	}

	cmd := newAuditCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--dir", dir, "--code", "SUPPORT_NOT_CURRENT", "--limit", "1", "-f", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit failed: %v\n%s", err, buf.String())
	}
	var report audit.Report
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("audit JSON invalid: %v\n%s", err, buf.String())
	}
	if len(report.Findings) != 1 {
		t.Fatalf("limit: got %d findings, want 1", len(report.Findings))
	}
	for _, f := range report.Findings {
		if f.Code != audit.CodeSupportNotCurrent {
			t.Fatalf("code filter leaked %s", f.Code)
		}
	}
}

// TestAuditCmd_CorpusFixtures smoke-tests the engine over every workspace in
// e2e/fixtures. Non-strict audit must never error, whatever findings it reports.
func TestAuditCmd_CorpusFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "e2e", "fixtures")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		svc, err := service.NewProofService(dir)
		if err != nil {
			t.Fatalf("NewProofService(%s): %v", e.Name(), err)
		}
		st, err := svc.LoadState()
		if err != nil {
			t.Fatalf("LoadState(%s): %v", e.Name(), err)
		}
		report := audit.Run(st, audit.Options{})
		if report.SchemaVersion != audit.SchemaVersion {
			t.Fatalf("%s: schema_version = %d", e.Name(), report.SchemaVersion)
		}
		if !report.Passed {
			t.Fatalf("%s: non-strict report must pass", e.Name())
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no fixture workspaces checked")
	}
}
