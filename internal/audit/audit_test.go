package audit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func mustID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return id
}

// addNode creates a state node with the given type, dependencies and options.
func addNode(t *testing.T, st *state.State, id string, typ schema.NodeType, deps []string, opts node.NodeOptions) *node.Node {
	t.Helper()
	parsed := make([]types.NodeID, len(deps))
	for i, d := range deps {
		parsed[i] = mustID(t, d)
	}
	opts.Dependencies = parsed
	n, err := node.NewNodeWithOptions(mustID(t, id), typ, "stmt "+id, schema.InferenceModusPonens, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions(%s): %v", id, err)
	}
	st.AddNode(n)
	return n
}

// validate marks a node validated with a recorded hash and verifier.
func validate(t *testing.T, st *state.State, id string, seq int) *node.Node {
	t.Helper()
	n := st.GetNode(mustID(t, id))
	if n == nil {
		t.Fatalf("node %s not found", id)
	}
	n.EpistemicState = schema.EpistemicValidated
	n.VerdictSeq = seq
	n.ValidatedBy = "verifier-" + id
	n.ValidatedContentHash = n.ContentHash
	return n
}

func codesOf(report Report) map[string]int {
	out := map[string]int{}
	for _, f := range report.Findings {
		out[f.Code]++
	}
	return out
}

func hasCode(report Report, code string) bool {
	for _, f := range report.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func findingFor(t *testing.T, report Report, code string) Finding {
	t.Helper()
	for _, f := range report.Findings {
		if f.Code == code {
			return f
		}
	}
	t.Fatalf("no finding with code %s: %v", code, codesOf(report))
	return Finding{}
}

// --- SUPPORT_NOT_CURRENT ---

func TestSupportNotCurrent_Positive(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	addNode(t, st, "1.1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 2)

	f := findingFor(t, Run(st, Options{}), CodeSupportNotCurrent)
	if f.Status != StatusCurrent || f.Severity != SeverityError {
		t.Fatalf("finding = %+v", f)
	}
	if f.Cause == "" {
		t.Fatalf("finding missing cause: %+v", f)
	}
	if len(f.Nodes) < 2 || f.Nodes[1].String() != "1.1" {
		t.Fatalf("responsible node = %+v, want 1.1", f.Nodes)
	}
}

func TestSupportNotCurrent_Negative(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 1)
	if hasCode(Run(st, Options{}), CodeSupportNotCurrent) {
		t.Fatalf("validated leaf should be support_current")
	}
}

// --- HASH_MISMATCH ---

func TestHashMismatch_Positive(t *testing.T) {
	st := state.NewState()
	n := addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 1)
	n.ContentHash = "moved"

	f := findingFor(t, Run(st, Options{}), CodeHashMismatch)
	if f.Status != StatusCurrent || !IsStrictCurrent(f) {
		t.Fatalf("recorded mismatch must be current and gating: %+v", f)
	}
}

func TestHashMismatch_Negative(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 1)
	if hasCode(Run(st, Options{}), CodeHashMismatch) {
		t.Fatalf("matching recorded hash must not be a finding")
	}
}

func TestHashMismatch_UnrecordedIsHistorical(t *testing.T) {
	st := state.NewState()
	n := addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	n.EpistemicState = schema.EpistemicValidated
	n.VerdictSeq = 1
	n.ValidatedBy = "v"
	// ValidatedContentHash deliberately empty.

	f := findingFor(t, Run(st, Options{}), CodeHashMismatch)
	if f.Status != StatusHistorical || IsStrictCurrent(f) {
		t.Fatalf("unrecorded hash must be historical and non-gating: %+v", f)
	}
	if !strings.Contains(f.Message, "reconstructed") {
		t.Fatalf("message should note reconstruction: %q", f.Message)
	}
}

// --- CYCLE ---

func TestCycle_Positive(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	addNode(t, st, "1.1", schema.NodeTypeClaim, []string{"1.2"}, node.NodeOptions{})
	addNode(t, st, "1.2", schema.NodeTypeClaim, []string{"1.1"}, node.NodeOptions{})

	f := findingFor(t, Run(st, Options{}), CodeCycle)
	if f.Status != StatusCurrent || len(f.Nodes) != 2 {
		t.Fatalf("finding = %+v", f)
	}
}

func TestCycle_Negative(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	addNode(t, st, "1.1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	if hasCode(Run(st, Options{}), CodeCycle) {
		t.Fatalf("acyclic tree must not report a cycle")
	}
}

// --- SCOPE_LEAK ---

func TestScopeLeak_Positive(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	addNode(t, st, "1.1", schema.NodeTypeLocalAssume, nil, node.NodeOptions{})
	addNode(t, st, "1.1.1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	addNode(t, st, "1.2", schema.NodeTypeLocalDischarge, nil, node.NodeOptions{})
	addNode(t, st, "1.3", schema.NodeTypeClaim, []string{"1.1.1"}, node.NodeOptions{})

	f := findingFor(t, Run(st, Options{}), CodeScopeLeak)
	if f.Status != StatusCurrent {
		t.Fatalf("finding = %+v", f)
	}
}

func TestScopeLeak_Negative(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	addNode(t, st, "1.1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	addNode(t, st, "1.2", schema.NodeTypeClaim, []string{"1.1"}, node.NodeOptions{})
	if hasCode(Run(st, Options{}), CodeScopeLeak) {
		t.Fatalf("in-scope citation must not leak")
	}
}

// --- CITES_SEVERED ---

func TestCitesSevered_Positive(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	addNode(t, st, "1.1", schema.NodeTypeClaim, []string{"1.2"}, node.NodeOptions{})

	f := findingFor(t, Run(st, Options{}), CodeCitesSevered)
	if f.Status != StatusCurrent || len(f.Nodes) != 2 {
		t.Fatalf("finding = %+v", f)
	}
}

func TestCitesSevered_Negative(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	addNode(t, st, "1.1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	if hasCode(Run(st, Options{}), CodeCitesSevered) {
		t.Fatalf("existing dependency must not be severed")
	}
}

// --- AMENDED_NOT_REVERIFIED ---

func TestAmendedNotReverified_Positive(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 2)
	st.AddAmendment(mustID(t, "1"), state.Amendment{Kind: state.AmendmentKindStatement, Seq: 5})

	f := findingFor(t, Run(st, Options{}), CodeAmendedNotReverified)
	if f.Status != StatusCurrent || len(f.Seqs) != 1 || f.Seqs[0] != 5 {
		t.Fatalf("finding = %+v", f)
	}
}

func TestAmendedNotReverified_Negative(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 7)
	st.AddAmendment(mustID(t, "1"), state.Amendment{Kind: state.AmendmentKindStatement, Seq: 5})
	if hasCode(Run(st, Options{}), CodeAmendedNotReverified) {
		t.Fatalf("amendment before the verdict must not be flagged")
	}
}

// --- SELF_ACCEPT ---

func TestSelfAccept_Positive(t *testing.T) {
	st := state.NewState()
	n := addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{Author: "alice"})
	validate(t, st, "1", 1)
	n.ValidatedBy = "alice"

	f := findingFor(t, Run(st, Options{}), CodeSelfAccept)
	if f.Status != StatusCurrent {
		t.Fatalf("finding = %+v", f)
	}
}

func TestSelfAccept_Negative(t *testing.T) {
	st := state.NewState()
	n := addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{Author: "alice"})
	validate(t, st, "1", 1)
	n.ValidatedBy = "bob"
	if hasCode(Run(st, Options{}), CodeSelfAccept) {
		t.Fatalf("different identity must not be self-accept")
	}
}

func TestSelfAccept_NoIdentityRecorded(t *testing.T) {
	st := state.NewState()
	n := addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	n.EpistemicState = schema.EpistemicValidated
	n.VerdictSeq = 1
	// No Author, no ValidatedBy.
	if hasCode(Run(st, Options{}), CodeSelfAccept) {
		t.Fatalf("unrecorded identities must not be self-accept")
	}
}

// --- VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE ---

func TestValidatedWithBlockingChallenge_Positive(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 1)
	st.AddChallenge(&state.Challenge{
		ID: "c1", NodeID: mustID(t, "1"), Status: state.ChallengeStatusOpen, Severity: "critical", Seq: 3,
	})

	f := findingFor(t, Run(st, Options{}), CodeValidatedWithOpenBlockingChallenge)
	if f.Status != StatusCurrent || len(f.Seqs) != 1 || f.Seqs[0] != 3 {
		t.Fatalf("finding = %+v", f)
	}
}

func TestValidatedWithBlockingChallenge_Negative(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 1)
	st.AddChallenge(&state.Challenge{
		ID: "c1", NodeID: mustID(t, "1"), Status: state.ChallengeStatusResolved, Severity: "critical",
	})
	if hasCode(Run(st, Options{}), CodeValidatedWithOpenBlockingChallenge) {
		t.Fatalf("resolved challenge must not block")
	}
}

// --- ADMITTED ---

func TestAdmitted_Positive(t *testing.T) {
	st := state.NewState()
	n := addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	n.EpistemicState = schema.EpistemicAdmitted

	f := findingFor(t, Run(st, Options{}), CodeAdmitted)
	if f.Status != StatusHistorical || IsStrictCurrent(f) {
		t.Fatalf("admitted must be historical and non-gating: %+v", f)
	}
}

func TestAdmitted_Negative(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	if hasCode(Run(st, Options{}), CodeAdmitted) {
		t.Fatalf("no admitted nodes")
	}
}

// --- ARCHIVED_WITH_OPEN_CHALLENGE ---

func TestArchivedWithOpenChallenge_Positive(t *testing.T) {
	st := state.NewState()
	a := addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	a.EpistemicState = schema.EpistemicArchived
	addNode(t, st, "1.1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	st.AddChallenge(&state.Challenge{
		ID: "c1", NodeID: mustID(t, "1.1"), Status: state.ChallengeStatusOpen, Severity: "minor",
	})

	f := findingFor(t, Run(st, Options{}), CodeArchivedWithOpenChallenge)
	if f.Status != StatusHistorical || IsStrictCurrent(f) {
		t.Fatalf("archived-with-challenge must be historical: %+v", f)
	}
}

func TestArchivedWithOpenChallenge_Negative(t *testing.T) {
	st := state.NewState()
	a := addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	a.EpistemicState = schema.EpistemicArchived
	if hasCode(Run(st, Options{}), CodeArchivedWithOpenChallenge) {
		t.Fatalf("no open challenge on archived subtree")
	}
}

// --- AMENDMENTS_PER_NODE ---

func TestAmendmentsPerNode_Positive(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	st.AddAmendment(mustID(t, "1"), state.Amendment{Kind: state.AmendmentKindStatement, Seq: 4})

	f := findingFor(t, Run(st, Options{}), CodeAmendmentsPerNode)
	if f.Status != StatusHistorical || f.Severity != SeverityInfo {
		t.Fatalf("finding = %+v", f)
	}
}

func TestAmendmentsPerNode_Negative(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	if hasCode(Run(st, Options{}), CodeAmendmentsPerNode) {
		t.Fatalf("no amendments")
	}
}

// --- PENDING_EXTERNAL_CITED_BY_VALIDATED ---

func TestPendingExternalCitedByValidated_Positive(t *testing.T) {
	st := state.NewState()
	n := addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 1)
	ext, err := node.NewExternal("Fermat", "Wiles 1995")
	if err != nil {
		t.Fatal(err)
	}
	st.AddExternal(ext)
	n.Context = []string{"Fermat"}

	f := findingFor(t, Run(st, Options{}), CodePendingExternalCitedByValidated)
	if f.Status != StatusHistorical || IsStrictCurrent(f) {
		t.Fatalf("pending external must be historical: %+v", f)
	}
}

func TestPendingExternalCitedByValidated_Negative(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 1)
	if hasCode(Run(st, Options{}), CodePendingExternalCitedByValidated) {
		t.Fatalf("no external cited")
	}
}

// --- UNKNOWN_PROVENANCE ---

func TestUnknownProvenance_Positive(t *testing.T) {
	st := state.NewState()
	n := addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	n.EpistemicState = schema.EpistemicValidated
	n.VerdictSeq = 1

	f := findingFor(t, Run(st, Options{}), CodeUnknownProvenance)
	if f.Status != StatusHistorical || f.Severity != SeverityInfo {
		t.Fatalf("finding = %+v", f)
	}
}

func TestUnknownProvenance_Negative(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 1)
	if hasCode(Run(st, Options{}), CodeUnknownProvenance) {
		t.Fatalf("verifier recorded")
	}
}

// --- contract: strict, filters, limit, schema ---

func TestStrict_PassedOnlyWhenNoStrictCurrent(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 1)

	passed := Run(st, Options{Strict: true})
	if !passed.Passed || passed.Summary.StrictCurrent != 0 {
		t.Fatalf("clean strict report = %+v", passed.Summary)
	}

	// Break the hash to create a strict-current finding.
	st.GetNode(mustID(t, "1")).ContentHash = "moved"
	failed := Run(st, Options{Strict: true})
	if failed.Passed || failed.Summary.StrictCurrent == 0 {
		t.Fatalf("broken strict report = %+v", failed.Summary)
	}

	// Without --strict the same state still passes.
	nonStrict := Run(st, Options{})
	if !nonStrict.Passed {
		t.Fatalf("non-strict report should always pass")
	}
}

func TestFilters_CodeNodeStatus(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	addNode(t, st, "1.1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	validate(t, st, "1", 2)

	// Code filter.
	onlySupport := Run(st, Options{Codes: []string{CodeSupportNotCurrent}})
	if len(onlySupport.Findings) == 0 {
		t.Fatal("expected support findings")
	}
	for _, f := range onlySupport.Findings {
		if f.Code != CodeSupportNotCurrent {
			t.Fatalf("code filter leaked %s", f.Code)
		}
	}

	// Status filter.
	historical := Run(st, Options{Status: StatusHistorical})
	for _, f := range historical.Findings {
		if f.Status != StatusHistorical {
			t.Fatalf("status filter leaked %s", f.Status)
		}
	}

	// Node prefix filter with dotted-boundary semantics.
	atRoot := Run(st, Options{NodePrefix: "1"})
	if len(atRoot.Findings) == 0 {
		t.Fatal("expected findings at/under 1")
	}
	notAChild := Run(st, Options{NodePrefix: "1.2"})
	for _, f := range notAChild.Findings {
		for _, n := range f.Nodes {
			s := n.String()
			if s != "1.2" && !strings.HasPrefix(s, "1.2.") {
				t.Fatalf("prefix filter leaked node %s in %s", s, f.Code)
			}
		}
	}
}

func TestLimit(t *testing.T) {
	st := state.NewState()
	for i := 1; i <= 5; i++ {
		n := addNode(t, st, "1."+string(rune('0'+i)), schema.NodeTypeClaim, nil, node.NodeOptions{})
		n.EpistemicState = schema.EpistemicValidated
		n.VerdictSeq = 1
		n.ValidatedBy = "verifier"
		n.ValidatedContentHash = "expected"
		n.ContentHash = "actual"
	}
	full := Run(st, Options{})
	if len(full.Findings) < 3 {
		t.Fatalf("need several findings, got %d", len(full.Findings))
	}
	limited := Run(st, Options{Limit: 2})
	if len(limited.Findings) != 2 {
		t.Fatalf("limit: got %d findings, want 2", len(limited.Findings))
	}
	if limited.Summary.Total != full.Summary.Total {
		t.Fatalf("summary should count pre-limit: got %d want %d", limited.Summary.Total, full.Summary.Total)
	}
}

func TestJSON_SchemaVersion(t *testing.T) {
	st := state.NewState()
	addNode(t, st, "1", schema.NodeTypeClaim, nil, node.NodeOptions{})
	data, err := json.Marshal(Run(st, Options{}))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %d, want %d", decoded.SchemaVersion, SchemaVersion)
	}
	if !strings.Contains(string(data), `"findings"`) {
		t.Fatalf("findings field missing: %s", data)
	}
}

func TestAllCodes_HaveRemediation(t *testing.T) {
	for _, c := range AllCodes() {
		if RemediationFor(c) == "" {
			t.Errorf("code %s has no remediation", c)
		}
	}
}
