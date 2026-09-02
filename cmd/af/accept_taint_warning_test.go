package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

func TestWarnTaintedDeps_IncludesNeedsRefinementDescendant(t *testing.T) {
	_, svc := setupTaintTraceTest(t)
	refineAndClaim(t, svc, "1", "prover1")
	if err := svc.ClaimNode(nid("1.1"), "prover1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := svc.RefineNode(nid("1.1"), "prover1", nid("1.1.1"), service.NodeTypeClaim, "Detailed step", service.InferenceAssumption); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"1.1.1", "1.1", "1"} {
		if err := svc.AcceptNode(nid(id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.RequestRefinement(nid("1.1.1"), "needs more proof", "verifier"); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetErr(buf)
	warnTaintedDeps(cmd, svc, []service.NodeID{nid("1")})
	output := buf.String()
	if !strings.Contains(output, "conditional children") || !strings.Contains(output, "taint: unresolved") {
		t.Errorf("warning did not describe unresolved refinement dependency: %s", output)
	}
}
