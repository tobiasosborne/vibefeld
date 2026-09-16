//go:build !integration

package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/service"
)

// TestStatusAndJobsCountsAgree is the D11 regression guard: `af status`'s
// Prover/Verifier summary and `af jobs` must report identical counts because
// both must be computed by the single internal/jobs classifier. Before the
// fix, status used a coarse workflow-only heuristic that called fresh pending
// nodes prover jobs while jobs classified them as verifier-ready.
func TestStatusAndJobsCountsAgree(t *testing.T) {
	dir := t.TempDir()
	if err := service.Init(dir, "Conjecture", "author"); err != nil {
		t.Fatalf("init: %v", err)
	}

	run := func(args ...string) string {
		t.Helper()
		cmd := newTestRootCmd()
		cmd.AddCommand(newClaimCmd(), newRefineCmd(), newChallengeCmd(), newReleaseCmd(), newStatusCmd(), newJobsCmd())
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return buf.String()
	}

	// Mix of states: root claimed, one available pending child, one child with
	// a blocking challenge (prover job), one claimed child.
	run("claim", "1", "--owner", "prover-1", "-d", dir)
	run("refine", "1", "step one", "-o", "prover-1", "-d", dir)
	run("refine", "1", "step two", "-o", "prover-1", "-d", dir)
	run("challenge", "1.1", "--reason", "gap in the argument", "--severity", "major", "-d", dir)
	run("release", "1", "--owner", "prover-1", "-d", dir)
	run("claim", "1.2", "--owner", "verifier-1", "--role", "verifier", "-d", dir)

	statusOut := run("status", "-f", "json", "-d", dir)
	jobsOut := run("jobs", "-f", "json", "-d", dir)

	var status struct {
		Jobs struct {
			ProverJobs   int `json:"prover_jobs"`
			VerifierJobs int `json:"verifier_jobs"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("status JSON: %v\n%s", err, statusOut)
	}

	var jobs struct {
		ProverJobs   []json.RawMessage `json:"prover_jobs"`
		VerifierJobs []json.RawMessage `json:"verifier_jobs"`
	}
	if err := json.Unmarshal([]byte(jobsOut), &jobs); err != nil {
		t.Fatalf("jobs JSON: %v\n%s", err, jobsOut)
	}

	if status.Jobs.ProverJobs != len(jobs.ProverJobs) {
		t.Errorf("status prover jobs = %d, jobs prover jobs = %d", status.Jobs.ProverJobs, len(jobs.ProverJobs))
	}
	if status.Jobs.VerifierJobs != len(jobs.VerifierJobs) {
		t.Errorf("status verifier jobs = %d, jobs verifier jobs = %d", status.Jobs.VerifierJobs, len(jobs.VerifierJobs))
	}
}
