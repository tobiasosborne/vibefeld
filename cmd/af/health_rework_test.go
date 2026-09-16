//go:build !integration

package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func addHealthTestNode(t *testing.T, st *state.State, id, stmt string, wf schema.WorkflowState, es schema.EpistemicState) *node.Node {
	t.Helper()
	nid, err := types.Parse(id)
	if err != nil {
		t.Fatalf("parse %q: %v", id, err)
	}
	n, err := node.NewNode(nid, schema.NodeTypeClaim, stmt, schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("new node %q: %v", id, err)
	}
	n.WorkflowState = wf
	n.EpistemicState = es
	st.AddNode(n)
	return n
}

// TestHealthCmd_ReworkFlags verifies the configurable thresholds exist.
func TestHealthCmd_ReworkFlags(t *testing.T) {
	cmd := newHealthCmd()
	for _, name := range []string{"hotspots", "rework-warn", "claim-stall"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("expected health to have --%s", name)
		}
	}
	if cmd.Flags().Lookup("hotspots").DefValue != "5" {
		t.Errorf("--hotspots default = %q, want 5", cmd.Flags().Lookup("hotspots").DefValue)
	}
	if cmd.Flags().Lookup("rework-warn").DefValue != "5" {
		t.Errorf("--rework-warn default = %q, want 5", cmd.Flags().Lookup("rework-warn").DefValue)
	}
	if cmd.Flags().Lookup("claim-stall").DefValue != "0s" {
		t.Errorf("--claim-stall default = %q, want 0s (the claim's own lease)", cmd.Flags().Lookup("claim-stall").DefValue)
	}
}

// TestAnalyzeHealth_ReworkIsDescriptive verifies health reports per-node rework
// with a configurable warn threshold and no falsity inference.
func TestAnalyzeHealth_ReworkIsDescriptive(t *testing.T) {
	st := state.NewState()
	n := addHealthTestNode(t, st, "1", "root claim", schema.WorkflowAvailable, schema.EpistemicPending)

	for i := 0; i < 6; i++ {
		st.AddChallenge(&state.Challenge{
			ID:       fmt.Sprintf("c%d", i),
			NodeID:   n.ID,
			Status:   state.ChallengeStatusResolved,
			Severity: "major",
		})
	}
	st.AddAmendment(n.ID, state.Amendment{Kind: state.AmendmentKindDependencies, Owner: "prover-1"})

	report := analyzeHealth(st, healthOptions{ReworkWarn: 5, Hotspots: 5})

	if len(report.Rework) != 1 {
		t.Fatalf("want 1 rework hotspot, got %d", len(report.Rework))
	}
	r := report.Rework[0]
	if r.ResolvedChallenges != 6 || r.Amendments != 1 || r.Rework != 7 {
		t.Errorf("unexpected rework metrics: %+v", r)
	}
	if !r.Warn {
		t.Errorf("node with %d rework should be warned at threshold 5", r.Rework)
	}
	if report.Statistics.ReworkWarned != 1 {
		t.Errorf("ReworkWarned = %d, want 1", report.Statistics.ReworkWarned)
	}

	out := renderHealthText(report)
	if strings.Contains(strings.ToLower(out), "probably false") {
		t.Errorf("health text must not infer falsity:\n%s", out)
	}
	if strings.Contains(out, "Fatigued") {
		t.Errorf("health text must not have a subtree fatigue alarm:\n%s", out)
	}
	if !strings.Contains(out, "not evidence of falsity") {
		t.Errorf("health text should explain that rework is not evidence of falsity:\n%s", out)
	}
	if !strings.Contains(out, "6 resolved challenge") {
		t.Errorf("health text should describe resolved challenges:\n%s", out)
	}
}

// TestAnalyzeHealth_HotspotLimit verifies --hotspots takes the top N.
func TestAnalyzeHealth_HotspotLimit(t *testing.T) {
	st := state.NewState()
	for i := 1; i <= 3; i++ {
		n := addHealthTestNode(t, st, fmt.Sprintf("1.%d", i), "step", schema.WorkflowAvailable, schema.EpistemicPending)
		for j := 0; j < i; j++ {
			st.AddChallenge(&state.Challenge{
				ID: fmt.Sprintf("c-%d-%d", i, j), NodeID: n.ID,
				Status: state.ChallengeStatusResolved, Severity: "major",
			})
		}
	}
	report := analyzeHealth(st, healthOptions{ReworkWarn: 100, Hotspots: 2})
	if len(report.Rework) != 2 {
		t.Fatalf("want 2 hotspots, got %d", len(report.Rework))
	}
	if report.Rework[0].NodeID != "1.3" || report.Rework[1].NodeID != "1.2" {
		t.Errorf("hotspots not sorted by rework desc: %+v", report.Rework)
	}
	if report.Statistics.ReworkNodes != 3 {
		t.Errorf("ReworkNodes = %d, want 3 (all reworked nodes counted)", report.Statistics.ReworkNodes)
	}
}

// TestAnalyzeHealth_ClaimBlockers verifies stalled and stale claims are
// reported with owner and expiry. The stall detector uses the dedicated
// claim-stall window / last claim activity, never the ledger lock timeout.
func TestAnalyzeHealth_ClaimBlockers(t *testing.T) {
	now := time.Now()
	st := state.NewState()

	stale := addHealthTestNode(t, st, "1.1", "stale", schema.WorkflowClaimed, schema.EpistemicPending)
	stale.ClaimedBy = "owner-stale"
	stale.ClaimedAt = types.FromTime(now.Add(-time.Minute))
	stale.ClaimLastActive = types.FromTime(now.Add(-10 * time.Minute))

	stalled := addHealthTestNode(t, st, "1.2", "stalled", schema.WorkflowClaimed, schema.EpistemicPending)
	stalled.ClaimedBy = "owner-stalled"
	stalled.ClaimedAt = types.FromTime(now.Add(time.Hour))
	stalled.ClaimedSince = types.FromTime(now.Add(-2 * time.Hour))
	stalled.ClaimLastActive = types.FromTime(now.Add(-10 * time.Minute))

	report := analyzeHealth(st, healthOptions{ReworkWarn: 5, Hotspots: 5, ClaimStall: time.Minute})

	var sawStale, sawStalled bool
	for _, b := range report.Blockers {
		switch b.Type {
		case "stale_claim":
			sawStale = true
			if b.Owner != "owner-stale" || b.Expires == "" {
				t.Errorf("stale claim missing owner/expiry: %+v", b)
			}
		case "stalled_claim":
			sawStalled = true
			if b.Owner != "owner-stalled" || b.Expires == "" {
				t.Errorf("stalled claim missing owner/expiry: %+v", b)
			}
		}
	}
	if !sawStale {
		t.Errorf("expected a stale_claim blocker")
	}
	if !sawStalled {
		t.Errorf("expected a stalled_claim blocker")
	}
}

// TestAnalyzeHealth_RefreshClearsStall verifies a refreshed claim is not
// reported as stalled even when it has been held for far longer than the
// ledger lock timeout, and that the default stall window is the claim's own
// lease rather than a fixed lock timeout.
func TestAnalyzeHealth_RefreshClearsStall(t *testing.T) {
	now := time.Now()
	st := state.NewState()

	// Acquired two hours ago, refreshed a moment ago, expires in an hour.
	// ClaimedSince is old, so a ClaimedSince-based detector (the old bug)
	// would flag it; ClaimLastActive is fresh, so it must not be stalled.
	refreshed := addHealthTestNode(t, st, "1.1", "refreshed", schema.WorkflowClaimed, schema.EpistemicPending)
	refreshed.ClaimedBy = "owner-refreshed"
	refreshed.ClaimedSince = types.FromTime(now.Add(-2 * time.Hour))
	refreshed.ClaimLastActive = types.FromTime(now.Add(-time.Second))
	refreshed.ClaimedAt = types.FromTime(now.Add(time.Hour))

	// Default window: the claim's own lease (ClaimedAt - ClaimLastActive,
	// about an hour). An old last-activity without a refresh is still not a
	// stall when the lease is longer, so a long-held claim is not flagged.
	longLease := addHealthTestNode(t, st, "1.2", "long-lease", schema.WorkflowClaimed, schema.EpistemicPending)
	longLease.ClaimedBy = "owner-long"
	longLease.ClaimLastActive = types.FromTime(now.Add(-10 * time.Minute))
	longLease.ClaimedAt = types.FromTime(now.Add(2 * time.Hour))

	report := analyzeHealth(st, healthOptions{ReworkWarn: 5, Hotspots: 5})
	for _, b := range report.Blockers {
		if b.Type == "stalled_claim" {
			t.Errorf("freshly refreshed / long-lease claim must not be stalled: %+v", b)
		}
	}

	// An explicit short window still catches the long-lease claim.
	report = analyzeHealth(st, healthOptions{ReworkWarn: 5, Hotspots: 5, ClaimStall: time.Minute})
	var sawLongLease bool
	for _, b := range report.Blockers {
		if b.Type == "stalled_claim" && b.Owner == "owner-long" {
			sawLongLease = true
		}
		if b.Type == "stalled_claim" && b.Owner == "owner-refreshed" {
			t.Errorf("refreshed claim flagged under explicit short window: %+v", b)
		}
	}
	if !sawLongLease {
		t.Errorf("explicit --claim-stall did not flag the long-held claim")
	}
}

// TestAnalyzeHealth_OpenChallengesHaveSeverityAndAge verifies open challenges
// are reported descriptively.
func TestAnalyzeHealth_OpenChallengesHaveSeverityAndAge(t *testing.T) {
	st := state.NewState()
	n := addHealthTestNode(t, st, "1", "root", schema.WorkflowAvailable, schema.EpistemicPending)
	st.AddChallenge(&state.Challenge{
		ID: "c1", NodeID: n.ID, Status: state.ChallengeStatusOpen,
		Severity: "critical", Reason: "gap", Created: types.FromTime(time.Now().Add(-2 * time.Hour)),
	})

	report := analyzeHealth(st, healthOptions{ReworkWarn: 5, Hotspots: 5})
	var found bool
	for _, b := range report.Blockers {
		if b.Type == "open_challenge" {
			found = true
			if b.Severity != "critical" || b.Age == "" {
				t.Errorf("open challenge blocker missing severity/age: %+v", b)
			}
			if b.Level != "info" {
				t.Errorf("open challenge should be info, got %q", b.Level)
			}
		}
	}
	if !found {
		t.Errorf("expected an open_challenge blocker")
	}
}
