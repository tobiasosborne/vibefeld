package audit

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
)

// marshalReport renders a report as JSON for byte-for-byte comparison.
func marshalReport(t *testing.T, r Report) string {
	t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return string(data)
}

// TestCitesSevered_LiveDepAndArchivedChild verifies that a live dependency and
// an archived child are not severance, while an explicit dependency on an
// archived node is.
func TestCitesSevered_LiveDepAndArchivedChild(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	archived := addNode(t, st, "1.1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	archived.EpistemicState = schema.EpistemicArchived
	addNode(t, st, "1.2", schema.NodeTypeClaim, nil, node.NodeOptions{})
	// A live explicit dependency plus an archived child: no severance.
	addNode(t, st, "1.3", schema.NodeTypeClaim, []string{"1.2"}, node.NodeOptions{})
	if hasCode(Run(st, Options{}), CodeCitesSevered) {
		t.Fatalf("live dependency + archived child must not be CITES_SEVERED: %v", codesOf(Run(st, Options{})))
	}
	// An explicit dependency on the archived node: severed.
	addNode(t, st, "1.4", schema.NodeTypeClaim, []string{"1.1"}, node.NodeOptions{})
	if !hasCode(Run(st, Options{}), CodeCitesSevered) {
		t.Fatalf("explicit dependency on an archived node must be CITES_SEVERED: %v", codesOf(Run(st, Options{})))
	}
}

// TestNodePrefixFilter_PositiveAndEmpty asserts a matching prefix is non-empty
// and a non-matching prefix is empty.
func TestNodePrefixFilter_PositiveAndEmpty(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	addNode(t, st, "1.1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1.1", 2)
	st.AddChallenge(&state.Challenge{
		ID: "c1", NodeID: mustID(t, "1.1"), Status: state.ChallengeStatusOpen, Severity: "critical", Seq: 3,
	})

	positive := Run(st, Options{NodePrefix: "1.1"})
	if len(positive.Findings) == 0 {
		t.Fatal("expected a non-empty positive prefix result")
	}
	for _, f := range positive.Findings {
		if !findingMatchesNodePrefix(f, "1.1") {
			t.Fatalf("prefix filter leaked a finding for %v", f.Nodes)
		}
	}

	negative := Run(st, Options{NodePrefix: "9.9"})
	if len(negative.Findings) != 0 {
		t.Fatalf("non-matching prefix must be empty, got %v", codesOf(negative))
	}
}

// TestAuditPerformance_1000NodesNoReplay asserts a 1000-node audit finishes
// well under two seconds and that the engine itself never replays the ledger
// (the caller's ordered pass is the only replay).
func TestAuditPerformance_1000NodesNoReplay(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	for i := 1; i <= 1000; i++ {
		addNode(t, st, fmt.Sprintf("1.%d", i), schema.NodeTypeClaim, nil, node.NodeOptions{})
	}

	before := ReplayCount()
	start := time.Now()
	_ = Run(st, Options{})
	_ = Run(st, Options{})
	elapsed := time.Since(start)

	if got := ReplayCount() - before; got != 0 {
		t.Fatalf("Run replayed the ledger %d time(s), want 0 (a caller supplies the pass)", got)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("audit of 1000 nodes took %v, want < 2s", elapsed)
	}
}
