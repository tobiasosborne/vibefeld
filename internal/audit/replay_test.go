package audit

import (
	"strings"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// appendEvent appends one event to a fresh ledger directory.
func appendEvent(t *testing.T, dir string, ev ledger.Event) {
	t.Helper()
	if _, err := ledger.Append(dir, ev); err != nil {
		t.Fatalf("append %s: %v", ev.Type(), err)
	}
}

// replayLedger opens dir and builds the final state and ordered pass exactly
// as the audit CLI does.
func replayLedger(t *testing.T, dir string) (*state.State, *Pass) {
	t.Helper()
	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	st, pass, err := BuildPass(ldg)
	if err != nil {
		t.Fatalf("BuildPass: %v", err)
	}
	return st, pass
}

// mkNode builds a claim node for a ledger event.
func mkNode(t *testing.T, id string, deps []string, author string) node.Node {
	t.Helper()
	parsed := make([]types.NodeID, len(deps))
	for i, d := range deps {
		parsed[i] = mustID(t, d)
	}
	n, err := node.NewNodeWithOptions(mustID(t, id), schema.NodeTypeClaim, "stmt "+id, schema.InferenceModusPonens, node.NodeOptions{
		Dependencies: parsed,
		Author:       author,
	})
	if err != nil {
		t.Fatalf("NewNodeWithOptions(%s): %v", id, err)
	}
	return *n
}

// --- HASH_MISMATCH reconstructed at the acceptance sequence ---

func TestReplayHashMismatch_ReconstructedMatchNoFinding(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1", nil, "alice")))
	// Validated without a recorded hash: the pass reconstructs it and it
	// matches the unchanged content, so there is no finding at all.
	appendEvent(t, dir, ledger.NewNodeValidated(mustID(t, "1")))

	st, pass := replayLedger(t, dir)
	if hasCode(RunWithPass(st, pass, Options{}), CodeHashMismatch) {
		t.Fatalf("matching reconstructed hash must not be a finding: %v", codesOf(RunWithPass(st, pass, Options{})))
	}
}

func TestReplayHashMismatch_ReconstructedMismatchIsHistorical(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeValidated(mustID(t, "1")))
	appendEvent(t, dir, ledger.NewNodeAmended(mustID(t, "1"), "stmt 1", "stmt 1 revised", "alice"))

	st, pass := replayLedger(t, dir)
	f := findingFor(t, RunWithPass(st, pass, Options{}), CodeHashMismatch)
	if f.Status != StatusHistorical || IsStrictCurrent(f) {
		t.Fatalf("reconstructed mismatch must be historical and non-gating: %+v", f)
	}
	if !strings.Contains(f.Message, "reconstructed") {
		t.Fatalf("message should say reconstructed: %q", f.Message)
	}
}

func TestReplayHashMismatch_RecordedMismatchStaysCurrentAndStrict(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	root := mkNode(t, "1", nil, "alice")
	appendEvent(t, dir, ledger.NewNodeCreated(root))
	// Record the hash of the content at validation time, then amend it.
	appendEvent(t, dir, ledger.NewNodeValidatedWithHash(mustID(t, "1"), "", "verifier", "", root.ContentHash, false))
	appendEvent(t, dir, ledger.NewNodeAmended(mustID(t, "1"), "stmt 1", "stmt 1 revised", "alice"))

	st, pass := replayLedger(t, dir)
	f := findingFor(t, RunWithPass(st, pass, Options{}), CodeHashMismatch)
	if f.Status != StatusCurrent || !IsStrictCurrent(f) {
		t.Fatalf("recorded mismatch must stay current and strict: %+v", f)
	}
}

// --- AMENDED_NOT_REVERIFIED against the consumer's verdict ---

func TestReplayAmendedNotReverified_TargetAmendedAndReAcceptedFlagsConsumer(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1.1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1.2", []string{"1.1"}, "alice")))

	// Consumer 1.2 is validated while target 1.1 is on its first revision.
	appendEvent(t, dir, ledger.NewNodeValidated(mustID(t, "1.2")))
	// The target is amended and then re-accepted (so it is current again).
	appendEvent(t, dir, ledger.NewNodeAmended(mustID(t, "1.1"), "stmt 1.1", "stmt 1.1 revised", "bob"))
	appendEvent(t, dir, ledger.NewNodeValidated(mustID(t, "1.1")))

	st, pass := replayLedger(t, dir)
	report := RunWithPass(st, pass, Options{})
	found := false
	for _, f := range report.Findings {
		if f.Code != CodeAmendedNotReverified {
			continue
		}
		found = true
		if len(f.Nodes) != 2 || f.Nodes[0].String() != "1.2" || f.Nodes[1].String() != "1.1" {
			t.Fatalf("consumer finding should name 1.2 relying on 1.1: %+v", f)
		}
	}
	if !found {
		t.Fatalf("re-accepting the amended target must still flag its earlier consumer: %v",
			codesOf(report))
	}
}

// --- SELF_ACCEPT at the verdict sequence, not final identities ---

func TestReplaySelfAccept_LaterAmenderIsNotSelfAcceptForEarlierVerdict(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1", nil, "alice")))
	// Dave validates, then amends afterwards. Dave did not contribute to the
	// revision that was accepted, so this is not a self-accept.
	appendEvent(t, dir, ledger.NewNodeValidatedFull(mustID(t, "1"), "", "dave", ""))
	appendEvent(t, dir, ledger.NewNodeAmended(mustID(t, "1"), "stmt 1", "stmt 1 revised", "dave"))

	st, pass := replayLedger(t, dir)
	if hasCode(RunWithPass(st, pass, Options{}), CodeSelfAccept) {
		t.Fatalf("a later amender must not be a self-accept for an earlier verdict: %v",
			codesOf(RunWithPass(st, pass, Options{})))
	}
}

func TestReplaySelfAccept_AuthorVerifierAtVerdictFlags(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeValidatedFull(mustID(t, "1"), "", "alice", ""))

	st, pass := replayLedger(t, dir)
	f := findingFor(t, RunWithPass(st, pass, Options{}), CodeSelfAccept)
	if f.Status != StatusCurrent {
		t.Fatalf("finding = %+v", f)
	}
}

// --- ARCHIVED_WITH_OPEN_CHALLENGE at the archival sequence ---

func TestReplayArchivedWithOpenChallenge_OpenAtArchiveSurvivesLaterResolve(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1.1", nil, "alice")))
	appendEvent(t, dir, ledger.NewChallengeRaised("c1", mustID(t, "1.1"), "statement", "wrong"))
	appendEvent(t, dir, ledger.NewNodeArchived(mustID(t, "1.1")))
	// Resolving after the archive must not erase the archival-time obligation.
	appendEvent(t, dir, ledger.NewChallengeResolved("c1"))

	st, pass := replayLedger(t, dir)
	if !hasCode(RunWithPass(st, pass, Options{}), CodeArchivedWithOpenChallenge) {
		t.Fatalf("open-at-archive challenge must be reported even after a later resolve: %v",
			codesOf(RunWithPass(st, pass, Options{})))
	}
	// Final-state-only reasoning loses it: demonstrates the ordered pass.
	if hasCode(Run(st, Options{}), CodeArchivedWithOpenChallenge) {
		t.Fatal("state-only audit should not see a challenge resolved after archival")
	}
}

func TestReplayArchivedWithOpenChallenge_OpenedAfterArchiveNotReported(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1.1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeArchived(mustID(t, "1.1")))
	// A challenge that opens after the archive is not an archival obligation.
	appendEvent(t, dir, ledger.NewChallengeRaised("c1", mustID(t, "1.1"), "statement", "wrong"))

	st, pass := replayLedger(t, dir)
	if hasCode(RunWithPass(st, pass, Options{}), CodeArchivedWithOpenChallenge) {
		t.Fatalf("challenge opened after archival must not be an archival obligation: %v",
			codesOf(RunWithPass(st, pass, Options{})))
	}
}

func TestReplayArchivedWithOpenChallenge_DescendantOpenAtArchive(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1.1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1.1.1", nil, "alice")))
	// A challenge on the descendant is open when 1.1 is archived.
	appendEvent(t, dir, ledger.NewChallengeRaised("c1", mustID(t, "1.1.1"), "statement", "wrong"))
	appendEvent(t, dir, ledger.NewNodeArchived(mustID(t, "1.1")))

	st, pass := replayLedger(t, dir)
	f := findingFor(t, RunWithPass(st, pass, Options{}), CodeArchivedWithOpenChallenge)
	if len(f.Nodes) < 2 || f.Nodes[0].String() != "1.1" || f.Nodes[1].String() != "1.1.1" {
		t.Fatalf("descendant obligation should be attributed to the archived ancestor: %+v", f)
	}
}

func TestReplayArchivedWithOpenChallenge_LaterDescendantChallengeNotReported(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1.1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1.1.1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeArchived(mustID(t, "1.1")))
	// Opened on a descendant only after the archival: not an archival obligation.
	appendEvent(t, dir, ledger.NewChallengeRaised("c1", mustID(t, "1.1.1"), "statement", "wrong"))

	st, pass := replayLedger(t, dir)
	if hasCode(RunWithPass(st, pass, Options{}), CodeArchivedWithOpenChallenge) {
		t.Fatalf("a descendant challenge opened after archival must not be reported: %v",
			codesOf(RunWithPass(st, pass, Options{})))
	}
}

func TestReplayArchivedWithOpenChallenge_RecordedAbandonedObligationsWin(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1.1", nil, "alice")))
	// The event carries the durable D9 snapshot even though no challenge was
	// open at the archival sequence (e.g. a forced archive recorded elsewhere).
	archived := ledger.NewNodeArchived(mustID(t, "1.1"))
	archived.AbandonedObligations = []string{"1.1", "1.1.1"}
	appendEvent(t, dir, archived)

	st, pass := replayLedger(t, dir)
	f := findingFor(t, RunWithPass(st, pass, Options{}), CodeArchivedWithOpenChallenge)
	if len(f.Nodes) != 3 || f.Nodes[1].String() != "1.1" || f.Nodes[2].String() != "1.1.1" {
		t.Fatalf("recorded abandoned_obligations should be used verbatim: %+v", f)
	}
}

// --- determinism ---

func TestRunWithPass_DeterministicJSON(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, ledger.NewProofInitialized("c", "a"))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1.1", nil, "alice")))
	appendEvent(t, dir, ledger.NewNodeCreated(mkNode(t, "1.2", nil, "alice")))
	// Several open blocking challenges on one node exercise map iteration order.
	appendEvent(t, dir, ledger.NewChallengeRaisedWithSeverity("c1", mustID(t, "1.1"), "t", "r", "critical", "v"))
	appendEvent(t, dir, ledger.NewChallengeRaisedWithSeverity("c2", mustID(t, "1.1"), "t", "r", "major", "v"))
	appendEvent(t, dir, ledger.NewChallengeRaisedWithSeverity("c3", mustID(t, "1.2"), "t", "r", "critical", "v"))
	appendEvent(t, dir, ledger.NewNodeValidatedFull(mustID(t, "1.1"), "", "v", ""))
	appendEvent(t, dir, ledger.NewNodeValidatedFull(mustID(t, "1.2"), "", "author-1.2", ""))

	st, pass := replayLedger(t, dir)
	first := marshalReport(t, RunWithPass(st, pass, Options{}))
	second := marshalReport(t, RunWithPass(st, pass, Options{}))
	if first != second {
		t.Fatalf("audit JSON not deterministic:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}
