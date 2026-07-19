// Package export provides proof export functionality to various formats.
package export

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/config"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// ValidateGraphFormat
// =============================================================================

func TestValidateGraphFormat_JSONValid(t *testing.T) {
	if err := ValidateGraphFormat("json"); err != nil {
		t.Errorf("ValidateGraphFormat(\"json\") unexpected error: %v", err)
	}
	if err := ValidateGraphFormat("JSON"); err != nil {
		t.Errorf("ValidateGraphFormat(\"JSON\") unexpected error: %v", err)
	}
}

func TestValidateGraphFormat_InvalidRejected(t *testing.T) {
	for _, bad := range []string{"", "yaml", "xml", "markdown"} {
		if err := ValidateGraphFormat(bad); err == nil {
			t.Errorf("ValidateGraphFormat(%q) expected error, got nil", bad)
		}
	}
}

// =============================================================================
// ExportGraph — structure
// =============================================================================

// buildFixtureState creates a small three-node proof tree with a challenge,
// used as the red-corpus fixture for the graph export tests.
func buildFixtureState(t *testing.T) *state.State {
	t.Helper()
	s := state.NewState()

	root := addTestNode(t, s, "1", "Root theorem statement", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintUnresolved)
	root.WorkflowState = schema.WorkflowAvailable

	child1 := addTestNode(t, s, "1.1", "First lemma", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)
	child1.WorkflowState = schema.WorkflowAvailable

	child2 := addTestNode(t, s, "1.2", "Second lemma", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	child2.WorkflowState = schema.WorkflowClaimed
	child2.Crux = true

	sc := &state.Challenge{
		ID:       "ch-1",
		NodeID:   child2.ID,
		Target:   "soundness",
		Reason:   "looks unjustified",
		Status:   state.ChallengeStatusOpen,
		Severity: "major",
		Created:  types.Now(),
		RaisedBy: "verifier-1",
	}
	s.AddChallenge(sc)

	return s
}

func TestExportGraph_SchemaVersion(t *testing.T) {
	s := buildFixtureState(t)
	out, err := ExportGraph(s, "/tmp/fixture-workspace", &config.Config{Title: "T", Conjecture: "C"})
	if err != nil {
		t.Fatalf("ExportGraph unexpected error: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("ExportGraph output is not valid JSON: %v\noutput: %s", err, out)
	}

	if doc["schema_version"] != "1" {
		t.Errorf("expected schema_version %q, got %v", "1", doc["schema_version"])
	}
}

func TestExportGraph_Workspace(t *testing.T) {
	s := buildFixtureState(t)
	out, err := ExportGraph(s, "/tmp/fixture-workspace", &config.Config{Title: "My Proof", Conjecture: "sqrt(2) is irrational"})
	if err != nil {
		t.Fatalf("ExportGraph unexpected error: %v", err)
	}

	var doc GraphExport
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("failed to unmarshal into GraphExport: %v", err)
	}

	if doc.Workspace.ID != "/tmp/fixture-workspace" {
		t.Errorf("expected workspace id %q, got %q", "/tmp/fixture-workspace", doc.Workspace.ID)
	}
	if doc.Workspace.Title != "My Proof" {
		t.Errorf("expected workspace title %q, got %q", "My Proof", doc.Workspace.Title)
	}
	if doc.Workspace.Conjecture != "sqrt(2) is irrational" {
		t.Errorf("expected workspace conjecture %q, got %q", "sqrt(2) is irrational", doc.Workspace.Conjecture)
	}
}

func TestExportGraph_NodesIncludeAllAxesAndContract(t *testing.T) {
	s := buildFixtureState(t)
	out, err := ExportGraph(s, "ws", nil)
	if err != nil {
		t.Fatalf("ExportGraph unexpected error: %v", err)
	}

	var doc GraphExport
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(doc.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(doc.Nodes))
	}

	byID := make(map[string]GraphNode)
	for _, n := range doc.Nodes {
		byID[n.ID] = n
	}

	root, ok := byID["1"]
	if !ok {
		t.Fatal("missing root node 1")
	}
	if root.Statement != "Root theorem statement" {
		t.Errorf("root statement = %q, want contract byte-match text", root.Statement)
	}
	if root.ParentID != "" {
		t.Errorf("root should have no parent_id, got %q", root.ParentID)
	}
	if len(root.ChildIDs) != 2 || root.ChildIDs[0] != "1.1" || root.ChildIDs[1] != "1.2" {
		t.Errorf("root child_ids = %v, want [1.1 1.2]", root.ChildIDs)
	}

	child2, ok := byID["1.2"]
	if !ok {
		t.Fatal("missing node 1.2")
	}
	if child2.ParentID != "1" {
		t.Errorf("1.2 parent_id = %q, want \"1\"", child2.ParentID)
	}
	if child2.WorkflowState != "claimed" {
		t.Errorf("1.2 workflow_state = %q, want claimed", child2.WorkflowState)
	}
	if child2.EpistemicState != "admitted" {
		t.Errorf("1.2 epistemic_state = %q, want admitted", child2.EpistemicState)
	}
	if child2.TaintState != "self_admitted" {
		t.Errorf("1.2 taint_state = %q, want self_admitted", child2.TaintState)
	}
	if !child2.Crux {
		t.Error("1.2 crux = false, want true")
	}
}

// TestExportGraph_IncludesAuthorAndVerifierFields covers rk-9pk / PRD C3 V1:
// author/validated_by/validation_batch_id are additive fields on GraphNode
// (docs/export-graph-v1.md's future-additive rule — no schema_version bump)
// that round-trip through `af export --graph json` when set, and are
// omitted (not just empty-string) when not.
func TestExportGraph_IncludesAuthorAndVerifierFields(t *testing.T) {
	s := buildFixtureState(t)
	root := s.GetNode(mustParseGraphNodeID(t, "1"))
	root.Author = "prover-1"
	child1 := s.GetNode(mustParseGraphNodeID(t, "1.1"))
	child1.Author = "prover-2"
	child1.ValidatedBy = "verifier-3"
	child1.ValidationBatchID = "batch-9"

	out, err := ExportGraph(s, "ws", nil)
	if err != nil {
		t.Fatalf("ExportGraph unexpected error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	if raw["schema_version"] != "1" {
		t.Errorf("schema_version = %v, want \"1\" (additive fields must not bump it)", raw["schema_version"])
	}

	var doc GraphExport
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	byID := make(map[string]GraphNode)
	for _, n := range doc.Nodes {
		byID[n.ID] = n
	}

	if byID["1"].Author != "prover-1" {
		t.Errorf("root author = %q, want %q", byID["1"].Author, "prover-1")
	}
	n11 := byID["1.1"]
	if n11.Author != "prover-2" {
		t.Errorf("1.1 author = %q, want %q", n11.Author, "prover-2")
	}
	if n11.ValidatedBy != "verifier-3" {
		t.Errorf("1.1 validated_by = %q, want %q", n11.ValidatedBy, "verifier-3")
	}
	if n11.ValidationBatchID != "batch-9" {
		t.Errorf("1.1 validation_batch_id = %q, want %q", n11.ValidationBatchID, "batch-9")
	}

	// Node "1.2" never had any of these set: the fields must be omitted
	// from the wire JSON entirely (omitempty), not merely empty, so a proof
	// replayed from a ledger that predates this addition re-exports
	// byte-identically to before.
	rawNodes, _ := raw["nodes"].([]interface{})
	var n12Raw map[string]interface{}
	for _, rn := range rawNodes {
		m := rn.(map[string]interface{})
		if m["id"] == "1.2" {
			n12Raw = m
		}
	}
	if n12Raw == nil {
		t.Fatal("node 1.2 missing from raw output")
	}
	for _, key := range []string{"author", "validated_by", "validation_batch_id"} {
		if _, present := n12Raw[key]; present {
			t.Errorf("node 1.2 raw JSON has key %q, want omitted (omitempty)", key)
		}
	}
}

func mustParseGraphNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("failed to parse node id %q: %v", s, err)
	}
	return id
}

func TestExportGraph_ValidationSummary(t *testing.T) {
	s := buildFixtureState(t)
	out, err := ExportGraph(s, "ws", nil)
	if err != nil {
		t.Fatalf("ExportGraph unexpected error: %v", err)
	}

	var doc GraphExport
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if doc.Validation.TotalNodes != 3 {
		t.Errorf("total_nodes = %d, want 3", doc.Validation.TotalNodes)
	}
	if doc.Validation.EpistemicCounts["validated"] != 1 {
		t.Errorf("epistemic_counts[validated] = %d, want 1", doc.Validation.EpistemicCounts["validated"])
	}
	if doc.Validation.TaintCounts["self_admitted"] != 1 {
		t.Errorf("taint_counts[self_admitted] = %d, want 1", doc.Validation.TaintCounts["self_admitted"])
	}
	if doc.Validation.TotalChallenges != 1 {
		t.Errorf("total_challenges = %d, want 1", doc.Validation.TotalChallenges)
	}
	if doc.Validation.ChallengeStatusCounts["open"] != 1 {
		t.Errorf("challenge_status_counts[open] = %d, want 1", doc.Validation.ChallengeStatusCounts["open"])
	}
}

func TestExportGraph_NilState(t *testing.T) {
	out, err := ExportGraph(nil, "ws", nil)
	if err != nil {
		t.Fatalf("ExportGraph(nil, ...) unexpected error: %v", err)
	}

	var doc GraphExport
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if doc.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want 1 even for nil state", doc.SchemaVersion)
	}
	if len(doc.Nodes) != 0 {
		t.Errorf("expected no nodes for nil state, got %d", len(doc.Nodes))
	}
}

// =============================================================================
// Determinism
// =============================================================================

// TestExportGraph_Deterministic asserts that exporting the same state twice
// produces byte-identical output — no timestamps-of-export, no map-iteration
// non-determinism, stable node ordering.
func TestExportGraph_Deterministic(t *testing.T) {
	s := buildFixtureState(t)
	cfg := &config.Config{Title: "T", Conjecture: "C"}

	out1, err := ExportGraph(s, "ws-1", cfg)
	if err != nil {
		t.Fatalf("first ExportGraph unexpected error: %v", err)
	}
	out2, err := ExportGraph(s, "ws-1", cfg)
	if err != nil {
		t.Fatalf("second ExportGraph unexpected error: %v", err)
	}

	if out1 != out2 {
		t.Errorf("ExportGraph is not deterministic:\nrun 1: %s\nrun 2: %s", out1, out2)
	}
}

// TestExportGraph_NoGenerationTimestamp asserts the export carries no
// export-time timestamp field mixed into identity-bearing content.
func TestExportGraph_NoGenerationTimestamp(t *testing.T) {
	s := buildFixtureState(t)
	out, err := ExportGraph(s, "ws", nil)
	if err != nil {
		t.Fatalf("ExportGraph unexpected error: %v", err)
	}

	for _, forbidden := range []string{"generated_at", "exported_at", "generation_time"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("output unexpectedly contains generation timestamp field %q", forbidden)
		}
	}
}
