//go:build !integration

package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/service"
)

// TestVerifierReadyAgreesEverywhere is the D11 review guard: `verifier_ready`
// means FilterReadyVerifierJobs semantics (a verifier job whose children are
// all cleared) on every surface — af get, af status, af jobs and af export
// --graph json. A pending parent with a pending child must be verifier-ready
// on none of them, while the pending leaf child is ready on all of them.
func TestVerifierReadyAgreesEverywhere(t *testing.T) {
	dir := t.TempDir()
	if err := service.Init(dir, "Conjecture", "author"); err != nil {
		t.Fatalf("init: %v", err)
	}

	run := func(args ...string) string {
		t.Helper()
		cmd := newTestRootCmd()
		cmd.AddCommand(newClaimCmd(), newRefineCmd(), newReleaseCmd(), newGetCmd(),
			newStatusCmd(), newJobsCmd(), newExportCmd())
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return buf.String()
	}

	// Root claimed, refined into a pending child, then released: the root has
	// a pending child, so it is a verifier job only if children are ignored.
	run("claim", "1", "--owner", "prover-1", "-d", dir)
	run("refine", "1", "child step", "-o", "prover-1", "-d", dir)
	run("release", "1", "--owner", "prover-1", "-d", dir)

	getReady := func(id string) bool {
		t.Helper()
		var node map[string]interface{}
		if err := json.Unmarshal([]byte(run("get", id, "-f", "json", "-d", dir)), &node); err != nil {
			t.Fatalf("get %s JSON: %v", id, err)
		}
		ready, _ := node["verifier_ready"].(bool)
		return ready
	}
	if getReady("1") {
		t.Errorf("af get: parent with a pending child must not be verifier_ready")
	}
	if !getReady("1.1") {
		t.Errorf("af get: pending leaf child must be verifier_ready")
	}

	var status struct {
		Jobs struct {
			VerifierJobs int `json:"verifier_jobs"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(run("status", "-f", "json", "-d", dir)), &status); err != nil {
		t.Fatalf("status JSON: %v", err)
	}

	var jobs struct {
		VerifierJobs []struct {
			NodeID string `json:"node_id"`
		} `json:"verifier_jobs"`
	}
	if err := json.Unmarshal([]byte(run("jobs", "-f", "json", "-d", dir)), &jobs); err != nil {
		t.Fatalf("jobs JSON: %v", err)
	}
	if status.Jobs.VerifierJobs != len(jobs.VerifierJobs) {
		t.Errorf("status verifier_jobs = %d, jobs verifier_jobs = %d", status.Jobs.VerifierJobs, len(jobs.VerifierJobs))
	}
	if len(jobs.VerifierJobs) != 1 || jobs.VerifierJobs[0].NodeID != "1.1" {
		t.Errorf("jobs verifier_jobs = %+v, want exactly the ready leaf 1.1", jobs.VerifierJobs)
	}

	var graph struct {
		Nodes []struct {
			ID            string `json:"id"`
			VerifierReady bool   `json:"verifier_ready"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(run("export", "--graph", "json", "-d", dir)), &graph); err != nil {
		t.Fatalf("export graph JSON: %v", err)
	}
	ready := map[string]bool{}
	for _, n := range graph.Nodes {
		ready[n.ID] = n.VerifierReady
	}
	if ready["1"] {
		t.Errorf("af export: parent with a pending child must not be verifier_ready")
	}
	if !ready["1.1"] {
		t.Errorf("af export: pending leaf child must be verifier_ready")
	}
}
