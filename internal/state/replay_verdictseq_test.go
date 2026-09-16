package state

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// TestReplay_VerdictSeq covers D4's derived verdict baseline: replay stamps
// VerdictSeq on both NodeValidated and NodeAdmitted, and clears it when the
// verdict is revoked (refinement request, unvalidate, unadmit). It lives in a
// file without the integration build tag so the default `go test ./...` runs it.
func TestReplay_VerdictSeq(t *testing.T) {
	cases := []struct {
		name       string
		verdict    func(types.NodeID) ledger.Event
		revoke     func(types.NodeID) ledger.Event
		wantAfter  int // the verdict event is always the 2nd event (seq 2)
		wantRevoke int
	}{
		{
			name:       "validated then refinement requested",
			verdict:    func(id types.NodeID) ledger.Event { return ledger.NewNodeValidated(id) },
			revoke:     func(id types.NodeID) ledger.Event { return ledger.NewRefinementRequested(id, "more work", "v") },
			wantAfter:  2,
			wantRevoke: 0,
		},
		{
			name:       "validated then unvalidated",
			verdict:    func(id types.NodeID) ledger.Event { return ledger.NewNodeValidated(id) },
			revoke:     func(id types.NodeID) ledger.Event { return ledger.NewNodeUnvalidated(id, "bad", "v") },
			wantAfter:  2,
			wantRevoke: 0,
		},
		{
			name:       "admitted stamps seq and unadmit clears it",
			verdict:    func(id types.NodeID) ledger.Event { return ledger.NewNodeAdmitted(id) },
			revoke:     func(id types.NodeID) ledger.Event { return ledger.NewNodeUnadmitted(id, "bad", "v") },
			wantAfter:  2,
			wantRevoke: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			nodeID := mustParseNodeID(t, "1")
			n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "claim", schema.InferenceAssumption)
			if _, err := ledger.Append(dir, ledger.NewNodeCreated(*n)); err != nil {
				t.Fatalf("Append NodeCreated: %v", err)
			}
			if _, err := ledger.Append(dir, tc.verdict(nodeID)); err != nil {
				t.Fatalf("Append verdict: %v", err)
			}
			ldg, err := ledger.NewLedger(dir)
			if err != nil {
				t.Fatalf("NewLedger: %v", err)
			}
			st, err := Replay(ldg)
			if err != nil {
				t.Fatalf("Replay: %v", err)
			}
			if got := st.GetNode(nodeID).VerdictSeq; got != tc.wantAfter {
				t.Fatalf("VerdictSeq after verdict = %d, want %d", got, tc.wantAfter)
			}

			if _, err := ledger.Append(dir, tc.revoke(nodeID)); err != nil {
				t.Fatalf("Append revoke: %v", err)
			}
			ldg, err = ledger.NewLedger(dir)
			if err != nil {
				t.Fatalf("NewLedger: %v", err)
			}
			st, err = Replay(ldg)
			if err != nil {
				t.Fatalf("Replay: %v", err)
			}
			if got := st.GetNode(nodeID).VerdictSeq; got != tc.wantRevoke {
				t.Fatalf("VerdictSeq after revoke = %d, want %d", got, tc.wantRevoke)
			}
		})
	}
}
