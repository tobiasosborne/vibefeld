package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/service"
)

func addAmendLeaf(t *testing.T, svc *service.ProofService, id string) {
	t.Helper()
	if err := svc.Refine(service.RefineSpec{
		ParentID: nid("1"), Owner: "prover1", ChildID: nid(id),
		NodeType: schema.NodeTypeClaim, Statement: "leaf " + id, Inference: schema.InferenceModusPonens,
	}); err != nil {
		t.Fatalf("refine %s: %v", id, err)
	}
}

func TestAmendDepsCmd_Single(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)
	if err := svc.ClaimNode(nid("1"), "prover1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	addAmendLeaf(t, svc, "1.1")
	addAmendLeaf(t, svc, "1.2")

	cmd := newAmendDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1.1", "--add", "1.2", "--owner", "prover1", "--reason", "wrong citation", "--dir", dir, "-f", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend-deps failed: %v\n%s", err, buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"outcome": "applied"`)) {
		t.Errorf("output = %s", buf.String())
	}
	st, _ := svc.LoadState()
	if len(st.GetNode(nid("1.1")).Dependencies) != 1 {
		t.Errorf("edge not applied")
	}
}

func TestAmendDepsCmd_Reopen(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)
	if err := svc.ClaimNode(nid("1"), "prover1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	addAmendLeaf(t, svc, "1.1")
	addAmendLeaf(t, svc, "1.2")
	if err := svc.AcceptNode(nid("1.1")); err != nil {
		t.Fatalf("accept: %v", err)
	}

	cmd := newAmendDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1.1", "--add", "1.2", "--reopen", "--owner", "prover1", "--reason", "fix", "--dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend-deps --reopen failed: %v\n%s", err, buf.String())
	}
	st, _ := svc.LoadState()
	if got := st.GetNode(nid("1.1")).EpistemicState; got != schema.EpistemicPending {
		t.Errorf("state = %s, want pending", got)
	}
}

func TestAmendDepsCmd_ManifestDryRunThenRun(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)
	if err := svc.ClaimNode(nid("1"), "prover1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	addAmendLeaf(t, svc, "1.1")
	addAmendLeaf(t, svc, "1.2")

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	manifest := `{"schema_version":1,"items":[{"node":"1.1","add":["1.2"],"owner":"prover1","reason":"fix"}]}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// Dry run: no events, operation id persisted. Use a root wired with the
	// global --dry-run flag, exactly as the real CLI is.
	root := newDryRunGuardedRoot()
	root.AddCommand(newAmendDepsCmd())
	out, err := executeCommand(root, "--dry-run", "amend-deps", "--file", manifestPath, "--dir", dir)
	if err != nil {
		t.Fatalf("dry run failed: %v\n%s", err, out)
	}
	afterDry, _ := svc.LoadState()
	if len(afterDry.GetNode(nid("1.1")).Dependencies) != 0 {
		t.Errorf("dry run applied an edge")
	}
	completed, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Items []struct {
			OperationID string `json:"operation_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(completed, &parsed); err != nil {
		t.Fatalf("completed manifest invalid JSON: %v", err)
	}
	if len(parsed.Items) != 1 || parsed.Items[0].OperationID == "" {
		t.Fatalf("dry run did not persist an operation id: %s", completed)
	}

	// Real run.
	cmd2 := newAmendDepsCmd()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetErr(buf2)
	cmd2.SetArgs([]string{"--file", manifestPath, "--dir", dir, "-f", "json"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("real run failed: %v\n%s", err, buf2.String())
	}
	st, _ := svc.LoadState()
	if len(st.GetNode(nid("1.1")).Dependencies) != 1 {
		t.Errorf("real run did not apply the edge")
	}
}

func TestAmendDepsCmd_SurfacesDepsAndAmendments(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)
	if err := svc.ClaimNode(nid("1"), "prover1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	addAmendLeaf(t, svc, "1.1")
	addAmendLeaf(t, svc, "1.2")
	if _, err := svc.AmendDeps(nid("1.1"), service.AmendDepsRequest{
		Add: []service.NodeID{nid("1.2")}, Owner: "prover1", Reason: "fix",
	}); err != nil {
		t.Fatalf("amend: %v", err)
	}

	depsCmd := newDepsCmd()
	depsBuf := new(bytes.Buffer)
	depsCmd.SetOut(depsBuf)
	depsCmd.SetErr(depsBuf)
	depsCmd.SetArgs([]string{"1.1", "--dir", dir})
	if err := depsCmd.Execute(); err != nil {
		t.Fatalf("deps: %v\n%s", err, depsBuf.String())
	}
	if !bytes.Contains(depsBuf.Bytes(), []byte("(*) edge added")) {
		t.Errorf("deps output missing amendment legend:\n%s", depsBuf.String())
	}

	amendCmd := newAmendmentsCmd()
	amendBuf := new(bytes.Buffer)
	amendCmd.SetOut(amendBuf)
	amendCmd.SetErr(amendBuf)
	amendCmd.SetArgs([]string{"1.1", "--dir", dir, "-f", "json"})
	if err := amendCmd.Execute(); err != nil {
		t.Fatalf("amendments: %v\n%s", err, amendBuf.String())
	}
	if !bytes.Contains(amendBuf.Bytes(), []byte("dependency_amendments")) {
		t.Errorf("amendments output missing dependency_amendments:\n%s", amendBuf.String())
	}
}

// TestAmendDepsCmd_PersistsOperationIDsBeforeFirstCommit verifies that a real
// run writes generated operation ids to the manifest atomically before its
// first commit, so a crash mid-batch leaves a resumable on-disk manifest.
func TestAmendDepsCmd_PersistsOperationIDsBeforeFirstCommit(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)
	if err := svc.ClaimNode(nid("1"), "prover1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	addAmendLeaf(t, svc, "1.1")
	addAmendLeaf(t, svc, "1.2")
	addAmendLeaf(t, svc, "1.3")

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	manifest := `{"schema_version":1,"items":[` +
		`{"node":"1.1","add":["1.2"],"owner":"prover1","reason":"r1"},` +
		`{"node":"1.2","add":["1.3"],"owner":"prover1","reason":"r2"},` +
		`{"node":"1.1","add":["1.3"],"owner":"prover1","reason":"r3"}]}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := newAmendDepsService
	defer func() { newAmendDepsService = orig }()

	calls := 0
	idsPersistedBeforeFirstCommit := false
	newAmendDepsService = func(d string) (*service.ProofService, error) {
		s, err := service.NewProofService(d)
		if err != nil {
			return nil, err
		}
		s.SetBeforeAppendTestHook(func() {
			calls++
			if calls == 1 {
				if data, err := os.ReadFile(manifestPath); err == nil {
					var parsed struct {
						Items []struct {
							OperationID string `json:"operation_id"`
						} `json:"items"`
					}
					if json.Unmarshal(data, &parsed) == nil && len(parsed.Items) == 3 {
						idsPersistedBeforeFirstCommit = parsed.Items[0].OperationID != "" &&
							parsed.Items[1].OperationID != "" && parsed.Items[2].OperationID != ""
					}
				}
			}
			if calls == 2 {
				panic("simulated crash after item 0")
			}
		})
		return s, nil
	}

	// First run crashes after item 0 commits.
	func() {
		defer func() { _ = recover() }()
		cmd := newAmendDepsCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"--file", manifestPath, "--dir", dir})
		_ = cmd.Execute()
	}()

	if !idsPersistedBeforeFirstCommit {
		t.Fatalf("operation ids were not persisted before the first commit")
	}

	// Resume from the on-disk file.
	newAmendDepsService = orig
	cmd := newAmendDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--file", manifestPath, "--resume", "--dir", dir, "-f", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resume failed: %v\n%s", err, buf.String())
	}

	var report struct {
		Items []struct {
			Node   string `json:"node"`
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("resume output is not JSON: %v\n%s", err, buf.String())
	}
	if len(report.Items) != 3 {
		t.Fatalf("resume report has %d items, want 3", len(report.Items))
	}
	if report.Items[0].Status != "applied(already)" {
		t.Errorf("item 0 status = %q, want applied(already)", report.Items[0].Status)
	}
	for i := 1; i < 3; i++ {
		if report.Items[i].Status != "applied" {
			t.Errorf("item %d status = %q, want applied", i, report.Items[i].Status)
		}
	}

	st, _ := svc.LoadState()
	if len(st.GetNode(nid("1.1")).Dependencies) != 2 || len(st.GetNode(nid("1.2")).Dependencies) != 1 {
		t.Fatalf("edges not all applied after resume: %+v", st.AllNodes())
	}
}

// TestAmendDepsCmd_DepsMarksOnlyAddedEdges verifies that removing one edge does
// not mark every surviving edge as amended, and that the removed edge is listed
// separately.
func TestAmendDepsCmd_DepsMarksOnlyAddedEdges(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)
	if err := svc.ClaimNode(nid("1"), "prover1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	addAmendLeaf(t, svc, "1.1")
	addAmendLeaf(t, svc, "1.2")
	addAmendLeaf(t, svc, "1.3")

	if _, err := svc.AmendDeps(nid("1.1"), service.AmendDepsRequest{
		Add: []service.NodeID{nid("1.2"), nid("1.3")}, Owner: "prover1", Reason: "add both",
	}); err != nil {
		t.Fatalf("amend add: %v", err)
	}
	if _, err := svc.AmendDeps(nid("1.1"), service.AmendDepsRequest{
		Remove: []service.NodeID{nid("1.2")}, Owner: "prover1", Reason: "drop one",
	}); err != nil {
		t.Fatalf("amend remove: %v", err)
	}

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1.1", "--dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("deps: %v\n%s", err, buf.String())
	}
	out := buf.String()
	// 1.3 is the surviving added edge: it keeps the mark.
	if !bytes.Contains([]byte(out), []byte("*1.3")) {
		t.Errorf("surviving added edge 1.3 not marked:\n%s", out)
	}
	// 1.2 was removed and is not listed as a current dependency.
	if bytes.Contains([]byte(out), []byte("*1.2")) {
		t.Errorf("removed edge 1.2 is marked as current:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte("removed by amendment: 1.2")) {
		t.Errorf("removed edge 1.2 not listed separately:\n%s", out)
	}
}
