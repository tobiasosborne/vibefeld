package service

import (
	"encoding/json"
	"errors"
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
	if after, _ := svc.LoadState(); after.LatestSeq() != st.LatestSeq() {
		t.Errorf("retry appended events")
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

// TestAmendDepsManifest_CrashResume verifies that aborting after item k and
// rerunning the same manifest yields the same final graph and the same ordered
// amendment events as an uninterrupted run, using the operation ids persisted
// by the first attempt.
func TestAmendDepsManifest_CrashResume(t *testing.T) {
	buildManifest := func() *AmendDepsManifest {
		m := &AmendDepsManifest{
			SchemaVersion: 1,
			Items: []AmendDepsManifestItem{
				{Node: "1.1", Add: []string{"1.2"}, Owner: "agent1", Reason: "r1"},
				{Node: "1.1", Add: []string{"1.3"}, Owner: "agent1", Reason: "r2"},
				{Node: "1.2", Add: []string{"1.3"}, Owner: "agent1", Reason: "r3"},
			},
		}
		m.EnsureOperationIDs()
		return m
	}

	setup := func(t *testing.T) *ProofService {
		svc, _ := setupTestProof(t)
		claimRootForAmend(t, svc)
		newTestLeaf(t, svc, "1.1")
		newTestLeaf(t, svc, "1.2")
		newTestLeaf(t, svc, "1.3")
		return svc
	}

	// Interrupted run.
	crashSvc := setup(t)
	mCrash := buildManifest()
	calls := 0
	crashSvc.beforeAppend = func() {
		calls++
		if calls == 2 {
			panic("simulated crash")
		}
	}
	func() {
		defer func() { _ = recover() }()
		_, _ = crashSvc.ApplyAmendDepsManifest(mCrash)
	}()
	crashSvc.beforeAppend = nil
	if _, err := crashSvc.ApplyAmendDepsManifest(mCrash); err != nil {
		t.Fatalf("resume apply: %v", err)
	}

	// Uninterrupted run on an identically built workspace.
	cleanSvc := setup(t)
	if _, err := cleanSvc.ApplyAmendDepsManifest(buildManifest()); err != nil {
		t.Fatalf("uninterrupted apply: %v", err)
	}

	crashEvents := collectDepsAmendments(t, crashSvc)
	cleanEvents := collectDepsAmendments(t, cleanSvc)
	if len(crashEvents) != len(cleanEvents) {
		t.Fatalf("resumed ledger has %d amendments, uninterrupted has %d", len(crashEvents), len(cleanEvents))
	}
	for i := range cleanEvents {
		if crashEvents[i] != cleanEvents[i] {
			t.Errorf("amendment %d differs: crash=%+v clean=%+v", i, crashEvents[i], cleanEvents[i])
		}
	}

	st, _ := crashSvc.LoadState()
	if len(st.GetNode(parseNodeID(t, "1.1")).Dependencies) != 2 ||
		len(st.GetNode(parseNodeID(t, "1.2")).Dependencies) != 1 {
		t.Fatalf("edges not all applied: %+v", st.AllNodes())
	}
}

// depsAmendmentSummary is the deterministic part of one node_deps_amended event.
type depsAmendmentSummary struct {
	Node                   string
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
			NodeID                 string           `json:"node_id"`
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
