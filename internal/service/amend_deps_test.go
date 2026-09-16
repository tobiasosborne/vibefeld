package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/support"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// newTestLeaf creates a leaf child of the claimed root.
func newTestLeaf(t *testing.T, svc *ProofService, id string) types.NodeID {
	t.Helper()
	root := parseNodeID(t, "1")
	if err := svc.Refine(RefineSpec{
		ParentID:  root,
		Owner:     "agent1",
		ChildID:   parseNodeID(t, id),
		NodeType:  schema.NodeTypeClaim,
		Statement: "leaf " + id,
		Inference: schema.InferenceModusPonens,
	}); err != nil {
		t.Fatalf("refine %s: %v", id, err)
	}
	return parseNodeID(t, id)
}

func claimRootForAmend(t *testing.T, svc *ProofService) {
	t.Helper()
	if err := svc.ClaimNode(parseNodeID(t, "1"), "agent1", time.Hour); err != nil {
		t.Fatalf("claim root: %v", err)
	}
}

func countEventType(t *testing.T, svc *ProofService, typ ledger.EventType) int {
	t.Helper()
	ldg, err := ledger.NewLedger(svc.ledgerDir())
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	n := 0
	if err := ldg.Scan(func(_ int, data []byte) error {
		var env struct {
			Type ledger.EventType `json:"type"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return err
		}
		if env.Type == typ {
			n++
		}
		return nil
	}); err != nil {
		t.Fatalf("scan ledger: %v", err)
	}
	return n
}

func TestAmendDeps_AddRemoveRefAndValidation(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	newTestLeaf(t, svc, "1.1")
	newTestLeaf(t, svc, "1.2")
	newTestLeaf(t, svc, "1.3")

	before, _ := svc.LoadState()
	target := parseNodeID(t, "1.1")
	oldHash := before.GetNode(target).ContentHash

	result, err := svc.AmendDeps(target, AmendDepsRequest{
		Add:          []types.NodeID{parseNodeID(t, "1.2")},
		AddValidated: []types.NodeID{parseNodeID(t, "1.3")},
		Owner:        "agent1",
		Reason:       "corrected citations",
		OperationID:  "op-add-remove",
	})
	if err != nil {
		t.Fatalf("AmendDeps: %v", err)
	}
	if result.Outcome != AmendDepsApplied {
		t.Fatalf("outcome = %q, want applied", result.Outcome)
	}
	if result.OldHash != oldHash {
		t.Errorf("old hash = %s, want %s", result.OldHash, oldHash)
	}
	if result.NewHash == oldHash {
		t.Errorf("new hash did not change")
	}
	if result.Seq == 0 {
		t.Errorf("expected a ledger sequence")
	}

	st, _ := svc.LoadState()
	n := st.GetNode(target)
	if len(n.Dependencies) != 1 || n.Dependencies[0].String() != "1.2" {
		t.Fatalf("dependencies = %v, want [1.2]", n.Dependencies)
	}
	if len(n.ValidationDeps) != 1 || n.ValidationDeps[0].String() != "1.3" {
		t.Fatalf("validation deps = %v, want [1.3]", n.ValidationDeps)
	}
	if n.ContentHash != result.NewHash {
		t.Errorf("state hash %s != result hash %s", n.ContentHash, result.NewHash)
	}

	hist := st.GetAmendmentHistory(target)
	if len(hist) != 1 || hist[0].Kind != AmendmentKindDependencies {
		t.Fatalf("history = %+v, want one dependency amendment", hist)
	}
	if hist[0].Reason != "corrected citations" || hist[0].Owner != "agent1" {
		t.Errorf("amendment provenance wrong: %+v", hist[0])
	}
	if countEventType(t, svc, ledger.EventNodeDepsAmended) != 1 {
		t.Errorf("expected exactly one node_deps_amended event")
	}
}

func TestAmendDeps_ReopenIsOneEvent(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")
	newTestLeaf(t, svc, "1.2")

	if err := svc.AcceptNodeWithVerifier(node, "", "verifier1", ""); err != nil {
		t.Fatalf("accept: %v", err)
	}
	pre, _ := svc.LoadState()
	if pre.GetNode(node).EpistemicState != schema.EpistemicValidated {
		t.Fatalf("node did not validate")
	}
	oldHash := pre.GetNode(node).ContentHash
	seqBefore := pre.LatestSeq()

	result, err := svc.AmendDeps(node, AmendDepsRequest{
		Add:         []types.NodeID{parseNodeID(t, "1.2")},
		Reopen:      true,
		Owner:       "agent1",
		Reason:      "wrong edge after review",
		OperationID: "op-reopen",
	})
	if err != nil {
		t.Fatalf("AmendDeps reopen: %v", err)
	}
	if !result.Reopened || !result.Reverify {
		t.Fatalf("reopened=%v reverify=%v, want both true", result.Reopened, result.Reverify)
	}

	st, _ := svc.LoadState()
	n := st.GetNode(node)
	if n.EpistemicState != schema.EpistemicPending {
		t.Fatalf("state = %s, want pending", n.EpistemicState)
	}
	if n.ValidatedBy != "" || n.ValidationBatchID != "" || n.ValidatedContentHash != "" {
		t.Errorf("validation provenance not cleared: %+v", n)
	}
	if n.ContentHash == oldHash {
		t.Errorf("hash did not change on reopen")
	}
	if len(n.Dependencies) != 1 || n.Dependencies[0].String() != "1.2" {
		t.Fatalf("dependencies = %v", n.Dependencies)
	}
	if st.LatestSeq() != seqBefore+1 {
		t.Errorf("ledger advanced by %d events, want exactly 1", st.LatestSeq()-seqBefore)
	}
	if countEventType(t, svc, ledger.EventNodeDepsAmended) != 1 {
		t.Errorf("expected exactly one node_deps_amended event")
	}
}

func TestAmendDeps_StaleExpectHashRejectedEvenWhenNoOp(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")
	newTestLeaf(t, svc, "1.2")

	if _, err := svc.AmendDeps(node, AmendDepsRequest{
		Add: []types.NodeID{parseNodeID(t, "1.2")}, Owner: "agent1", Reason: "first",
	}); err != nil {
		t.Fatalf("first amend: %v", err)
	}
	// The add is now a no-op, but a stale expect-hash must still be refused.
	_, err := svc.AmendDeps(node, AmendDepsRequest{
		Add:        []types.NodeID{parseNodeID(t, "1.2")},
		ExpectHash: "deadbeef",
		Owner:      "agent1",
		Reason:     "stale retry",
	})
	if !errors.Is(err, ErrAmendDepsHashMismatch) {
		t.Fatalf("err = %v, want ErrAmendDepsHashMismatch", err)
	}
	if aferrors.Code(err) == 0 || aferrors.Code(err).ExitCode() != 3 {
		t.Errorf("stale hash should be a logic error (exit 3), got %v", aferrors.Code(err))
	}
}

func TestAmendDeps_RetrySameOperationID(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")
	newTestLeaf(t, svc, "1.2")

	req := AmendDepsRequest{
		Add:         []types.NodeID{parseNodeID(t, "1.2")},
		Owner:       "agent1",
		Reason:      "first",
		OperationID: "op-retry",
	}
	first, err := svc.AmendDeps(node, req)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	st, _ := svc.LoadState()
	// Retry with the now-stale expect hash; the operation id short-circuits.
	req.ExpectHash = first.OldHash
	second, err := svc.AmendDeps(node, req)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if second.Outcome != AmendDepsAppliedAlready {
		t.Fatalf("outcome = %q, want applied-already", second.Outcome)
	}
	if second.Seq != first.Seq {
		t.Errorf("retry seq = %d, want first seq %d", second.Seq, first.Seq)
	}
	if second.OldHash != first.OldHash || second.NewHash != first.NewHash {
		t.Errorf("retry hashes = %s/%s, want original %s/%s",
			second.OldHash, second.NewHash, first.OldHash, first.NewHash)
	}
	if after, _ := svc.LoadState(); after.LatestSeq() != st.LatestSeq() {
		t.Errorf("retry appended events")
	}
}

// TestAmendDeps_OperationIDConflict verifies that reusing an operation id for a
// different node or a different change set is refused rather than reported as
// applied-already.
func TestAmendDeps_OperationIDConflict(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	nodeA := newTestLeaf(t, svc, "1.1")
	nodeB := newTestLeaf(t, svc, "1.2")
	newTestLeaf(t, svc, "1.3")

	first, err := svc.AmendDeps(nodeA, AmendDepsRequest{
		Add: []types.NodeID{parseNodeID(t, "1.3")}, Owner: "agent1", Reason: "first", OperationID: "op-bound",
	})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	_ = first

	// Same id, different node.
	_, err = svc.AmendDeps(nodeB, AmendDepsRequest{
		Add: []types.NodeID{parseNodeID(t, "1.3")}, Owner: "agent1", Reason: "other node", OperationID: "op-bound",
	})
	if !errors.Is(err, ErrAmendDepsOperationIDConflict) {
		t.Fatalf("different node: err = %v, want ErrAmendDepsOperationIDConflict", err)
	}
	if aferrors.Code(err).ExitCode() != 3 {
		t.Errorf("conflict should exit 3, got %v", aferrors.Code(err))
	}

	// Same id, same node, different change set.
	_, err = svc.AmendDeps(nodeA, AmendDepsRequest{
		Add: []types.NodeID{parseNodeID(t, "1.2")}, Owner: "agent1", Reason: "different change", OperationID: "op-bound",
	})
	if !errors.Is(err, ErrAmendDepsOperationIDConflict) {
		t.Fatalf("different change set: err = %v, want ErrAmendDepsOperationIDConflict", err)
	}

	// The original id still resolves to the original event's result.
	retry, err := svc.AmendDeps(nodeA, AmendDepsRequest{
		Add: []types.NodeID{parseNodeID(t, "1.3")}, Owner: "agent1", Reason: "first", OperationID: "op-bound",
	})
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retry.Outcome != AmendDepsAppliedAlready || retry.NewHash != first.NewHash {
		t.Fatalf("retry = %+v, want applied-already with new hash %s", retry, first.NewHash)
	}
}

func TestAmendDeps_UnchangedAppendsNothing(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")
	before, _ := svc.LoadState()

	result, err := svc.AmendDeps(node, AmendDepsRequest{
		Remove: []types.NodeID{parseNodeID(t, "1")}, // absent -> no-op
		Owner:  "agent1", Reason: "noop",
	})
	if err != nil {
		t.Fatalf("AmendDeps: %v", err)
	}
	if result.Outcome != AmendDepsUnchanged {
		t.Fatalf("outcome = %q, want unchanged", result.Outcome)
	}
	after, _ := svc.LoadState()
	if after.LatestSeq() != before.LatestSeq() {
		t.Errorf("unchanged amendment appended events")
	}
}

func TestAmendDeps_StrictNoOpIsError(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")

	_, err := svc.AmendDeps(node, AmendDepsRequest{
		Add:    []types.NodeID{parseNodeID(t, "1.2")},
		Strict: true,
		Owner:  "agent1", Reason: "strict add missing target is checked first",
	})
	// 1.2 does not exist: target check fires before strict. Create it and retry.
	_ = err
	newTestLeaf(t, svc, "1.2")
	_, err = svc.AmendDeps(node, AmendDepsRequest{
		Remove: []types.NodeID{parseNodeID(t, "1.2")},
		Strict: true,
		Owner:  "agent1", Reason: "strict remove absent",
	})
	if !errors.Is(err, ErrAmendDepsStrictNoChange) {
		t.Fatalf("err = %v, want ErrAmendDepsStrictNoChange", err)
	}
}

func TestAmendDeps_ContradictionRejected(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")
	newTestLeaf(t, svc, "1.2")

	_, err := svc.AmendDeps(node, AmendDepsRequest{
		Add:    []types.NodeID{parseNodeID(t, "1.2")},
		Remove: []types.NodeID{parseNodeID(t, "1.2")},
		Owner:  "agent1", Reason: "contradiction",
	})
	if !errors.Is(err, ErrAmendDepsContradiction) {
		t.Fatalf("err = %v, want ErrAmendDepsContradiction", err)
	}
}

func TestAmendDeps_StatePreconditions(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")
	if err := svc.AcceptNodeWithVerifier(node, "", "verifier1", ""); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// Validated without reopen is refused.
	_, err := svc.AmendDeps(node, AmendDepsRequest{Owner: "agent1", Reason: "no reopen"})
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("validated without reopen: %v", err)
	}

	// Admitted is refused with the unadmit remedy.
	if err := svc.UnvalidateNode(node, "test", "verifier1"); err != nil {
		t.Fatalf("unvalidate: %v", err)
	}
	if err := svc.AdmitNode(node); err != nil {
		t.Fatalf("admit: %v", err)
	}
	_, err = svc.AmendDeps(node, AmendDepsRequest{Owner: "agent1", Reason: "admitted"})
	if !errors.Is(err, ErrAmendDepsAdmitted) {
		t.Fatalf("admitted: %v", err)
	}
}

func TestAmendDeps_CycleRejectedWithPath(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")

	_, err := svc.AmendDeps(node, AmendDepsRequest{
		Add:   []types.NodeID{parseNodeID(t, "1")}, // parent -> cycle
		Owner: "agent1", Reason: "cycle",
	})
	if !errors.Is(err, support.ErrCycle) {
		t.Fatalf("err = %v, want cycle", err)
	}
	var ce *support.CycleError
	if !errors.As(err, &ce) {
		t.Fatalf("not a CycleError: %v", err)
	}
	if len(ce.Path) < 2 {
		t.Errorf("cycle path too short: %v", ce.Path)
	}
	if countEventType(t, svc, ledger.EventNodeDepsAmended) != 0 {
		t.Errorf("rejected cycle appended an event")
	}
}

func TestAmendDeps_ScopeLeakRejected(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	root := parseNodeID(t, "1")

	if err := svc.Refine(RefineSpec{ParentID: root, Owner: "agent1", ChildID: parseNodeID(t, "1.1"), NodeType: schema.NodeTypeLocalAssume, Statement: "Suppose P", Inference: schema.InferenceLocalAssume}); err != nil {
		t.Fatalf("assume: %v", err)
	}
	assume := parseNodeID(t, "1.1")
	if err := svc.ClaimNode(assume, "agent1", time.Hour); err != nil {
		t.Fatalf("claim assume: %v", err)
	}
	if err := svc.Refine(RefineSpec{ParentID: assume, Owner: "agent1", ChildID: parseNodeID(t, "1.1.1"), NodeType: schema.NodeTypeClaim, Statement: "inside", Inference: schema.InferenceModusPonens}); err != nil {
		t.Fatalf("inside: %v", err)
	}
	if err := svc.Refine(RefineSpec{ParentID: assume, Owner: "agent1", ChildID: parseNodeID(t, "1.1.2"), NodeType: schema.NodeTypeLocalDischarge, Statement: "discharge", Inference: schema.InferenceLocalDischarge}); err != nil {
		t.Fatalf("discharge: %v", err)
	}
	outside := newTestLeaf(t, svc, "1.2")

	_, err := svc.AmendDeps(outside, AmendDepsRequest{
		Add:   []types.NodeID{parseNodeID(t, "1.1.1")},
		Owner: "agent1", Reason: "foreign citation",
	})
	if !errors.Is(err, support.ErrScopeLeak) {
		t.Fatalf("err = %v, want scope leak", err)
	}
	var se *support.ScopeLeakError
	if !errors.As(err, &se) || se.Dep.String() != "1.1.1" {
		t.Fatalf("scope leak should name 1.1.1: %v", err)
	}
}

func TestAmendDeps_ClaimOwnership(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")
	// Claim the child by another agent.
	if err := svc.ClaimNode(node, "other", time.Hour); err != nil {
		t.Fatalf("claim child: %v", err)
	}
	_, err := svc.AmendDeps(node, AmendDepsRequest{Owner: "agent1", Reason: "not owner"})
	if !errors.Is(err, ErrOwnerMismatch) {
		t.Fatalf("err = %v, want ErrOwnerMismatch", err)
	}
}

func TestAmendDeps_FormatGateOnV10Workspace(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir, "Test conjecture", "test-author"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	writeMetaVersion(t, dir, "1.0")
	svc, err := NewProofService(dir)
	if err != nil {
		t.Fatalf("NewProofService: %v", err)
	}
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")
	newTestLeaf(t, svc, "1.2")

	// An actual change builds a node_deps_amended event, which requires 1.1.
	_, err = svc.AmendDeps(node, AmendDepsRequest{
		Add: []types.NodeID{parseNodeID(t, "1.2")}, Owner: "agent1", Reason: "r",
	})
	if aferrors.Code(err) != aferrors.FORMAT_TOO_NEW {
		t.Fatalf("err = %v, want FORMAT_TOO_NEW", err)
	}
	if aferrors.Code(err).ExitCode() != 3 {
		t.Errorf("format gate should exit 3")
	}
}

// TestAmendDepsManifest_DryRunPersistsOperationIDs verifies a dry run writes no
// events but assigns operation ids to every item.
// TestAmendDepsManifest_DryRunRealRunParity verifies that a dry run plans each
// item against the state produced by the previous plans, so its statuses match
// the real run item for item (including a reciprocal-edge cycle).
func TestAmendDepsManifest_DryRunRealRunParity(t *testing.T) {
	setup := func(t *testing.T) *ProofService {
		svc, _ := setupTestProof(t)
		claimRootForAmend(t, svc)
		newTestLeaf(t, svc, "1.1")
		newTestLeaf(t, svc, "1.2")
		newTestLeaf(t, svc, "1.3")
		return svc
	}
	buildManifest := func() *AmendDepsManifest {
		return &AmendDepsManifest{SchemaVersion: 1, Items: []AmendDepsManifestItem{
			{Node: "1.1", Add: []string{"1.2"}, Owner: "agent1", Reason: "r1"},
			{Node: "1.2", Add: []string{"1.1"}, Owner: "agent1", Reason: "reciprocal"},
			{Node: "1.1", Add: []string{"1.3"}, Owner: "agent1", Reason: "r3"},
			{Node: "1.3", Remove: []string{"1.2"}, Owner: "agent1", Reason: "no-op"},
		}}
	}

	drySvc := setup(t)
	dryManifest := buildManifest()
	dryReport, err := drySvc.DryRunAmendDepsManifest(dryManifest)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	// Dry run must not have touched the ledger.
	if countEventType(t, drySvc, ledger.EventNodeDepsAmended) != 0 {
		t.Fatalf("dry run wrote events")
	}

	realSvc := setup(t)
	realManifest := buildManifest()
	realReport, _ := realSvc.ApplyAmendDepsManifest(realManifest)

	if len(dryReport.Items) != len(realReport.Items) {
		t.Fatalf("item counts differ: dry %d real %d", len(dryReport.Items), len(realReport.Items))
	}
	for i := range dryReport.Items {
		d, r := dryReport.Items[i], realReport.Items[i]
		if d.Status != r.Status {
			t.Errorf("item %d (%s) status: dry %q, real %q", i, d.Node, d.Status, r.Status)
		}
		// The dry run also previews the attempted diff for a rejected item; the
		// real run only reports a rejection reason, so compare the planned
		// outcome fields only where the real run produces them.
		if strings.HasPrefix(r.Status, "rejected:") {
			continue
		}
		if d.OldHash != r.OldHash || d.NewHash != r.NewHash {
			t.Errorf("item %d (%s) hashes: dry %s/%s, real %s/%s", i, d.Node, d.OldHash, d.NewHash, r.OldHash, r.NewHash)
		}
		if strings.Join(d.Added, ",") != strings.Join(r.Added, ",") ||
			strings.Join(d.Removed, ",") != strings.Join(r.Removed, ",") {
			t.Errorf("item %d (%s) edge diff: dry %+v, real %+v", i, d.Node, d, r)
		}
	}
}

func TestAmendDepsManifest_ExitCodes(t *testing.T) {
	cases := []struct {
		name    string
		report  AmendDepsManifestReport
		wantNil bool
		want    aferrors.ErrorCode
	}{
		{"empty manifest is none-applied", AmendDepsManifestReport{}, false, aferrors.AMEND_DEPS_NONE_APPLIED},
		{"all unchanged is exit 7", AmendDepsManifestReport{Items: make([]AmendDepsManifestStatus, 2), Unchanged: 2}, false, aferrors.AMEND_DEPS_ALL_UNCHANGED},
		{"zero applied is none-applied", AmendDepsManifestReport{Items: make([]AmendDepsManifestStatus, 2), Rejected: 2}, false, aferrors.AMEND_DEPS_NONE_APPLIED},
		{"applied plus rejected is partial", AmendDepsManifestReport{Items: make([]AmendDepsManifestStatus, 2), Applied: 1, Rejected: 1}, false, aferrors.AMEND_DEPS_PARTIALLY_APPLIED},
		{"all applied is success", AmendDepsManifestReport{Items: make([]AmendDepsManifestStatus, 2), Applied: 2}, true, 0},
		{"applied plus unchanged is success", AmendDepsManifestReport{Items: make([]AmendDepsManifestStatus, 2), Applied: 1, Unchanged: 1}, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.report.exitError()
			if tc.wantNil {
				if err != nil {
					t.Fatalf("exitError() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("exitError() = nil, want %v", tc.want)
			}
			if got := aferrors.Code(err); got != tc.want {
				t.Errorf("code = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAmendDepsManifest_DryRunPersistsOperationIDs(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	newTestLeaf(t, svc, "1.1")
	newTestLeaf(t, svc, "1.2")
	before, _ := svc.LoadState()

	m := &AmendDepsManifest{
		SchemaVersion: 1,
		Items: []AmendDepsManifestItem{
			{Node: "1.1", Add: []string{"1.2"}, Owner: "agent1", Reason: "r1"},
			{Node: "1.2", Add: []string{"1.1"}, Owner: "agent1", Reason: "r2"},
		},
	}
	report, err := svc.DryRunAmendDepsManifest(m)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(report.Items) != 2 {
		t.Fatalf("report items = %d", len(report.Items))
	}
	// The dry run plans item 1 against the state left by item 0: 1.1 now
	// depends on 1.2, so 1.2 -> 1.1 is a result-use cycle.
	if report.Items[0].Status != AmendDepsApplied {
		t.Errorf("item 0 status = %q, want applied", report.Items[0].Status)
	}
	if report.Items[1].Status != "rejected:DEPENDENCY_CYCLE" {
		t.Errorf("item 1 status = %q, want rejected:DEPENDENCY_CYCLE", report.Items[1].Status)
	}
	for i, item := range m.Items {
		if item.OperationID == "" {
			t.Errorf("item %d got no operation id", i)
		}
	}
	if countEventType(t, svc, ledger.EventNodeDepsAmended) != 0 {
		t.Errorf("dry run wrote events")
	}
	after, _ := svc.LoadState()
	if after.LatestSeq() != before.LatestSeq() {
		t.Errorf("dry run advanced the ledger")
	}
}

func TestAmendDepsManifest_RealRunReportsEveryItemOnce(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	newTestLeaf(t, svc, "1.1")
	newTestLeaf(t, svc, "1.2")
	newTestLeaf(t, svc, "1.3")

	m := &AmendDepsManifest{
		SchemaVersion: 1,
		Items: []AmendDepsManifestItem{
			{Node: "1.1", Add: []string{"1.2"}, Owner: "agent1", Reason: "r1"},
			{Node: "1.1", Add: []string{"1.2"}, Owner: "agent1", Reason: "r1 again"}, // unchanged
			{Node: "1.1", Add: []string{"1.3"}, Owner: "agent1", Reason: "r2"},
		},
	}
	report, err := svc.ApplyAmendDepsManifest(m)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(report.Items) != 3 {
		t.Fatalf("report has %d items, want 3", len(report.Items))
	}
	seen := map[string]int{}
	for _, item := range report.Items {
		seen[item.Node]++
	}
	if seen["1.1"] != 3 {
		t.Errorf("per-node reporting wrong: %v", seen)
	}
	if report.Applied != 2 || report.Unchanged != 1 {
		t.Errorf("applied=%d unchanged=%d, want 2/1", report.Applied, report.Unchanged)
	}
}

// TestAmendDepsManifest_RestartFromDisk verifies that a crash after item k
// leaves the on-disk manifest carrying the operation ids of the items already
// attempted, and that a restart from that file recognises them (applied-already)
// and converges on exactly the same ledger as an uninterrupted run. Strict and
// stale-expect-hash items are included so the full event stream (operation id,
// hashes, owner, reason) is compared.
func TestAmendDepsManifest_RestartFromDisk(t *testing.T) {
	buildManifest := func() *AmendDepsManifest {
		return &AmendDepsManifest{
			SchemaVersion: 1,
			Items: []AmendDepsManifestItem{
				{Node: "1.1", Add: []string{"1.2"}, Owner: "agent1", Reason: "r1"},
				{Node: "1.2", Add: []string{"1.3"}, Owner: "agent1", Reason: "r2"},
				{Node: "1.4", Add: []string{"1.1"}, Strict: true, Owner: "agent1", Reason: "r3-strict"},
				{Node: "1.1", Add: []string{"1.3"}, ExpectHash: "deadbeef", Owner: "agent1", Reason: "stale"},
			},
		}
	}

	setup := func(t *testing.T) *ProofService {
		svc, _ := setupTestProof(t)
		claimRootForAmend(t, svc)
		newTestLeaf(t, svc, "1.1")
		newTestLeaf(t, svc, "1.2")
		newTestLeaf(t, svc, "1.3")
		newTestLeaf(t, svc, "1.4")
		return svc
	}

	// Persist the manifest (with ids) to disk, as the CLI does before its first
	// commit.
	manifest := buildManifest()
	manifest.EnsureOperationIDs()
	data, err := manifest.MarshalIndent()
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Interrupted run.
	crashSvc := setup(t)
	calls := 0
	crashSvc.beforeAppend = func() {
		calls++
		if calls == 2 {
			panic("simulated crash")
		}
	}
	func() {
		defer func() { _ = recover() }()
		_, _ = crashSvc.ApplyAmendDepsManifest(manifest)
	}()
	crashSvc.beforeAppend = nil

	// Restart from the on-disk file (the ids survive). The stale-expect-hash
	// item is intentionally rejected, so the aggregate outcome is partial.
	onDisk := readManifestFile(t, path)
	if _, err := crashSvc.ApplyAmendDepsManifest(onDisk); err != nil && aferrors.Code(err) != aferrors.AMEND_DEPS_PARTIALLY_APPLIED {
		t.Fatalf("resume apply: %v", err)
	}

	// Uninterrupted run on an identically built workspace, using the same ids.
	cleanSvc := setup(t)
	if _, err := cleanSvc.ApplyAmendDepsManifest(manifest); err != nil && aferrors.Code(err) != aferrors.AMEND_DEPS_PARTIALLY_APPLIED {
		t.Fatalf("uninterrupted apply: %v", err)
	}

	crashEvents := collectDepsAmendments(t, crashSvc)
	cleanEvents := collectDepsAmendments(t, cleanSvc)
	if len(crashEvents) != 3 {
		t.Fatalf("resumed ledger has %d amendments, want 3 (applied items)", len(crashEvents))
	}
	if len(crashEvents) != len(cleanEvents) {
		t.Fatalf("resumed ledger has %d amendments, uninterrupted has %d", len(crashEvents), len(cleanEvents))
	}
	for i := range cleanEvents {
		if crashEvents[i] != cleanEvents[i] {
			t.Errorf("amendment %d differs:\n crash=%+v\n clean=%+v", i, crashEvents[i], cleanEvents[i])
		}
	}

	st, _ := crashSvc.LoadState()
	if len(st.GetNode(parseNodeID(t, "1.1")).Dependencies) != 1 ||
		len(st.GetNode(parseNodeID(t, "1.2")).Dependencies) != 1 ||
		len(st.GetNode(parseNodeID(t, "1.4")).Dependencies) != 1 {
		t.Fatalf("edges not all applied: %+v", st.AllNodes())
	}
}

// readManifestFile re-reads and parses a persisted amend-deps manifest.
func readManifestFile(t *testing.T, path string) *AmendDepsManifest {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	m, err := ParseAmendDepsManifest(data)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

// depsAmendmentSummary is the deterministic part of one node_deps_amended event,
// including the provenance fields so a resume can be compared event for event.
type depsAmendmentSummary struct {
	Node                   string
	OperationID            string
	Owner                  string
	Reason                 string
	PreviousContentHash    string
	PreviousDependencies   string
	NewDependencies        string
	PreviousValidationDeps string
	NewValidationDeps      string
	Reopened               bool
}

func collectDepsAmendments(t *testing.T, svc *ProofService) []depsAmendmentSummary {
	t.Helper()
	ldg, err := ledger.NewLedger(svc.ledgerDir())
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	var out []depsAmendmentSummary
	if err := ldg.Scan(func(_ int, data []byte) error {
		var ev struct {
			Type                   ledger.EventType `json:"type"`
			OperationID            string           `json:"operation_id"`
			NodeID                 string           `json:"node_id"`
			Owner                  string           `json:"owner"`
			Reason                 string           `json:"reason"`
			PreviousContentHash    string           `json:"previous_content_hash"`
			PreviousDependencies   []string         `json:"previous_dependencies"`
			NewDependencies        []string         `json:"new_dependencies"`
			PreviousValidationDeps []string         `json:"previous_validation_deps"`
			NewValidationDeps      []string         `json:"new_validation_deps"`
			Reopened               bool             `json:"reopened"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return err
		}
		if ev.Type != ledger.EventNodeDepsAmended {
			return nil
		}
		out = append(out, depsAmendmentSummary{
			Node:                   ev.NodeID,
			OperationID:            ev.OperationID,
			Owner:                  ev.Owner,
			Reason:                 ev.Reason,
			PreviousContentHash:    ev.PreviousContentHash,
			PreviousDependencies:   strings.Join(ev.PreviousDependencies, ","),
			NewDependencies:        strings.Join(ev.NewDependencies, ","),
			PreviousValidationDeps: strings.Join(ev.PreviousValidationDeps, ","),
			NewValidationDeps:      strings.Join(ev.NewValidationDeps, ","),
			Reopened:               ev.Reopened,
		})
		return nil
	}); err != nil {
		t.Fatalf("scan ledger: %v", err)
	}
	return out
}

// TestAmendDeps_RemoveDanglingEdge verifies that an edge whose target does not
// exist (created by writing a node_deps_amended event directly, as a legacy or
// D1 sink) can still be removed: target existence applies to ADD lists only.
func TestAmendDeps_RemoveDanglingEdge(t *testing.T) {
	svc, _ := setupTestProof(t)
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")

	// Write a dangling edge directly into the ledger, bypassing the service's
	// add-target check.
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	n := st.GetNode(node)
	dangling := parseNodeID(t, "1.99")
	ev := ledger.NewNodeDepsAmended(node, nil, []types.NodeID{dangling}, nil, nil,
		"agent1", "legacy dangling edge", n.ContentHash, false)
	if _, err := ledger.AppendBatchIfSequence(svc.ledgerDir(), []ledger.Event{ev}, st.LatestSeq()); err != nil {
		t.Fatalf("append dangling event: %v", err)
	}

	after, err := svc.LoadState()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(after.GetNode(node).Dependencies) != 1 {
		t.Fatalf("dangling edge not written: %v", after.GetNode(node).Dependencies)
	}

	result, err := svc.AmendDeps(node, AmendDepsRequest{
		Remove: []types.NodeID{dangling}, Owner: "agent1", Reason: "clear dangling sink",
	})
	if err != nil {
		t.Fatalf("remove dangling edge: %v", err)
	}
	if result.Outcome != AmendDepsApplied {
		t.Fatalf("outcome = %q, want applied", result.Outcome)
	}
	final, _ := svc.LoadState()
	if len(final.GetNode(node).Dependencies) != 0 {
		t.Errorf("dangling edge survived: %v", final.GetNode(node).Dependencies)
	}
}

// TestAmendNodeWithReopen_FormatGateOnV10Workspace verifies that `af amend
// --reopen` emits the format-1.1 node_amended_reopened event and is refused on
// a 1.0 workspace, while a plain amend still emits the format-1.0 node_amended.
func TestAmendNodeWithReopen_FormatGateOnV10Workspace(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir, "Test conjecture", "test-author"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	writeMetaVersion(t, dir, "1.0")
	svc, err := NewProofService(dir)
	if err != nil {
		t.Fatalf("NewProofService: %v", err)
	}
	claimRootForAmend(t, svc)
	node := newTestLeaf(t, svc, "1.1")

	// A plain amend is a 1.0 event and must still work.
	if err := svc.AmendNode(node, "agent1", "plain correction"); err != nil {
		t.Fatalf("plain amend on 1.0: %v", err)
	}
	if countEventType(t, svc, ledger.EventNodeAmended) != 1 {
		t.Errorf("plain amend did not emit node_amended")
	}

	// Validate the node, then try a reopened amend: the reopened event is 1.1.
	if err := svc.AcceptNodeWithVerifier(node, "", "verifier1", ""); err != nil {
		t.Fatalf("accept: %v", err)
	}
	err = svc.AmendNodeWithReopen(node, "agent1", "reopened correction", true)
	if aferrors.Code(err) != aferrors.FORMAT_TOO_NEW {
		t.Fatalf("reopened amend on 1.0: err = %v, want FORMAT_TOO_NEW", err)
	}
	if countEventType(t, svc, ledger.EventNodeAmendedReopened) != 0 {
		t.Errorf("reopened amend wrote a 1.1 event on a 1.0 workspace")
	}
}
