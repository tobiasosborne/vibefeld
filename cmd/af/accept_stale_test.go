package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/service"
)

// D3 review fix: a single-node accept of a crux node whose only passing
// claim-test is stale must surface the service's staleness diagnosis, not the
// generic "no passing claim-test" message.
func TestAcceptCmd_StaleCruxClaimTestNamesStaleness(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)

	rootID := nid("1")
	if err := svc.ClaimNode(rootID, "prover-1", time.Hour); err != nil {
		t.Fatalf("ClaimNode: %v", err)
	}
	ids, err := svc.RefineNodeBulk(rootID, "prover-1", []service.ChildSpec{
		{Statement: "Crux step", NodeType: schema.NodeTypeClaim, Inference: schema.InferenceModusPonens, Crux: true},
	})
	if err != nil {
		t.Fatalf("RefineNodeBulk: %v", err)
	}
	childID := ids[0]

	// Fabricate a passing test recorded against a different content hash.
	ldg, err := ledger.NewLedger(filepath.Join(svc.Path(), "ledger"))
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	ev := ledger.NewClaimTested(childID, "script", "test.py", "", true, "ok", "agent-1")
	ev.ContentHash = "stale-hash"
	if _, err := ldg.Append(ev); err != nil {
		t.Fatalf("append ClaimTested: %v", err)
	}

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{childID.String(), "--dir", dir})
	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected accept to fail for a stale claim-test, got nil")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("CLI error should name the staleness, got: %v", err)
	}
	if strings.Contains(err.Error(), "has no passing claim-test") {
		t.Errorf("CLI fell back to the generic no-passing-test message: %v", err)
	}
}
