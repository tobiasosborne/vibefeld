package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/service"
)

// decodeDiffOutput runs `af diff` with the given args and decodes its JSON.
func decodeDiffOutput(t *testing.T, dir string, args ...string) diffOutputJSON {
	t.Helper()
	cmd := newDiffCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(append([]string{"1.1", "--dir", dir, "-f", "json"}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff %v: %v\n%s", args, err, buf.String())
	}
	var out diffOutputJSON
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("diff output is not JSON: %v\n%s", err, buf.String())
	}
	return out
}

// TestDiffCmd_DependencyChangesFilteredByInterval verifies that a diff only
// reports the dependency amendments recorded between the two statement versions
// it compares, rather than every dependency amendment ever recorded.
func TestDiffCmd_DependencyChangesFilteredByInterval(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)
	if err := svc.ClaimNode(nid("1"), "prover1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	addAmendLeaf(t, svc, "1.1")
	addAmendLeaf(t, svc, "1.2")
	addAmendLeaf(t, svc, "1.3")

	amendDeps := func(add string, reason string) {
		t.Helper()
		if _, err := svc.AmendDeps(nid("1.1"), service.AmendDepsRequest{
			Add: []service.NodeID{nid(add)}, Owner: "prover1", Reason: reason,
		}); err != nil {
			t.Fatalf("amend-deps %s: %v", reason, err)
		}
	}
	amendStmt := func(stmt string) {
		t.Helper()
		if err := svc.AmendNode(nid("1.1"), "prover1", stmt); err != nil {
			t.Fatalf("amend %q: %v", stmt, err)
		}
	}

	amendDeps("1.2", "dep-before-stmt1")
	amendStmt("revised once")
	amendDeps("1.3", "dep-before-stmt2")
	amendStmt("revised twice")

	reasons := func(out diffOutputJSON) []string {
		var rs []string
		for _, d := range out.DependencyChanges {
			rs = append(rs, d.Reason)
		}
		return rs
	}

	// Default compares v1 -> v2, the interval containing only dep-before-stmt2.
	def := decodeDiffOutput(t, dir)
	if got := reasons(def); len(got) != 1 || got[0] != "dep-before-stmt2" {
		t.Errorf("default diff dependency changes = %v, want [dep-before-stmt2]", got)
	}
	if len(def.DependencyChanges) == 1 && def.DependencyChanges[0].Seq == 0 {
		t.Errorf("dependency change is missing its ledger seq")
	}

	// --version 1 is the same interval.
	v1 := decodeDiffOutput(t, dir, "--version", "1")
	if got := reasons(v1); len(got) != 1 || got[0] != "dep-before-stmt2" {
		t.Errorf("--version 1 dependency changes = %v, want [dep-before-stmt2]", got)
	}

	// --version 0 spans the original through the current statement, so both
	// dependency amendments belong to it.
	v0 := decodeDiffOutput(t, dir, "--version", "0")
	got := reasons(v0)
	if len(got) != 2 || got[0] != "dep-before-stmt1" || got[1] != "dep-before-stmt2" {
		t.Errorf("--version 0 dependency changes = %v, want both deps in order", got)
	}
}

// TestDiffCmd_SinceChallengeUsesSequence verifies that --since-challenge filters
// dependency amendments by the challenge's ledger sequence.
func TestDiffCmd_SinceChallengeUsesSequence(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)
	if err := svc.ClaimNode(nid("1"), "prover1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	addAmendLeaf(t, svc, "1.1")
	addAmendLeaf(t, svc, "1.2")
	addAmendLeaf(t, svc, "1.3")

	if _, err := svc.AmendDeps(nid("1.1"), service.AmendDepsRequest{
		Add: []service.NodeID{nid("1.2")}, Owner: "prover1", Reason: "before-challenge",
	}); err != nil {
		t.Fatalf("amend before: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	chal := ledger.NewChallengeRaised("ch-seq", nid("1.1"), "statement", "vague")
	if _, err := ledger.AppendBatchIfSequence(filepath.Join(svc.Path(), "ledger"), []ledger.Event{chal}, st.LatestSeq()); err != nil {
		t.Fatalf("raise challenge: %v", err)
	}

	if _, err := svc.AmendDeps(nid("1.1"), service.AmendDepsRequest{
		Add: []service.NodeID{nid("1.3")}, Owner: "prover1", Reason: "after-challenge",
	}); err != nil {
		t.Fatalf("amend after: %v", err)
	}

	cmd := newDiffCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1.1", "--dir", dir, "--since-challenge", "ch-seq", "-f", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff since-challenge: %v\n%s", err, buf.String())
	}
	var out diffOutputJSON
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if len(out.DependencyChanges) != 1 || out.DependencyChanges[0].Reason != "after-challenge" {
		t.Errorf("since-challenge dependency changes = %+v, want only after-challenge", out.DependencyChanges)
	}
}
