package export

import (
	"encoding/json"
	"testing"

	"github.com/tobias/vibefeld/internal/config"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// nodeByID pulls one node object out of the parsed export document.
func nodeByID(t *testing.T, out, id string) map[string]interface{} {
	t.Helper()
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("export output is not valid JSON: %v", err)
	}
	nodes, ok := doc["nodes"].([]interface{})
	if !ok {
		t.Fatalf("export has no nodes[] array")
	}
	for _, raw := range nodes {
		n := raw.(map[string]interface{})
		if n["id"] == id {
			return n
		}
	}
	t.Fatalf("node %q not found in export", id)
	return nil
}

// B3: a fully-validated tree with no open blocking challenges reports every
// node closed:true, and the root in particular.
func TestExportGraph_Closed_FullyValidatedRoot(t *testing.T) {
	s := state.NewState()
	root := addTestNode(t, s, "1", "Root", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)
	root.WorkflowState = schema.WorkflowAvailable
	c := addTestNode(t, s, "1.1", "Child", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)
	c.WorkflowState = schema.WorkflowAvailable

	out, err := ExportGraph(s, "/tmp/ws", &config.Config{})
	if err != nil {
		t.Fatalf("ExportGraph: %v", err)
	}
	if nodeByID(t, out, "1")["closed"] != true {
		t.Errorf("validated root with validated child must be closed:true, got %v", nodeByID(t, out, "1")["closed"])
	}
	if nodeByID(t, out, "1.1")["closed"] != true {
		t.Errorf("validated leaf must be closed:true")
	}
}

// B3 (the exact defect): a VALIDATED root that later acquires an open blocking
// challenge must NOT report closed — its epistemic state is unchanged, so the
// old "validated == converged" predicate would wrongly say converged.
func TestExportGraph_Closed_ValidatedButChallengedRoot(t *testing.T) {
	s := state.NewState()
	root := addTestNode(t, s, "1", "Root", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)
	root.WorkflowState = schema.WorkflowAvailable
	s.AddChallenge(&state.Challenge{
		ID: "ch-1", NodeID: root.ID, Target: "statement", Reason: "wrong after all",
		Status: state.ChallengeStatusOpen, Severity: "critical", Created: types.Now(), RaisedBy: "v1",
	})

	out, err := ExportGraph(s, "/tmp/ws", &config.Config{})
	if err != nil {
		t.Fatalf("ExportGraph: %v", err)
	}
	if got := nodeByID(t, out, "1")["closed"]; got == true {
		t.Errorf("validated root with an OPEN BLOCKING challenge must be closed:false (absent), got closed:%v", got)
	}
}

// B3: a validated parent whose child is NOT closed (e.g. child pending) must
// not itself be closed — closure is bottom-up.
func TestExportGraph_Closed_ChildNotClosedBubblesUp(t *testing.T) {
	s := state.NewState()
	root := addTestNode(t, s, "1", "Root", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicValidated, node.TaintClean)
	root.WorkflowState = schema.WorkflowAvailable
	c := addTestNode(t, s, "1.1", "Child", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	c.WorkflowState = schema.WorkflowAvailable

	out, err := ExportGraph(s, "/tmp/ws", &config.Config{})
	if err != nil {
		t.Fatalf("ExportGraph: %v", err)
	}
	if got := nodeByID(t, out, "1")["closed"]; got == true {
		t.Errorf("validated root with a PENDING child must be closed:false, got %v", got)
	}
}

// B2: a node's recorded reference dependencies are emitted in the export so a
// verifier sees the prover's exact dependency set (not a children proxy).
func TestExportGraph_NodeDependencies(t *testing.T) {
	s := state.NewState()
	root := addTestNode(t, s, "1", "Root", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	root.WorkflowState = schema.WorkflowAvailable
	c1 := addTestNode(t, s, "1.1", "Lemma A", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	c1.WorkflowState = schema.WorkflowAvailable
	c2 := addTestNode(t, s, "1.2", "Uses A", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	c2.WorkflowState = schema.WorkflowAvailable
	c2.Dependencies = []types.NodeID{c1.ID}

	out, err := ExportGraph(s, "/tmp/ws", &config.Config{})
	if err != nil {
		t.Fatalf("ExportGraph: %v", err)
	}
	deps, ok := nodeByID(t, out, "1.2")["dependencies"].([]interface{})
	if !ok || len(deps) != 1 || deps[0] != "1.1" {
		t.Errorf("node 1.2 dependencies = %v, want [1.1]", nodeByID(t, out, "1.2")["dependencies"])
	}
	if _, present := nodeByID(t, out, "1.1")["dependencies"]; present {
		t.Errorf("node 1.1 has no deps; dependencies key must be omitted")
	}
}

// FU5: the export always advertises a features[] capability list, and it names
// the closure-flag and readiness-flag capabilities an rk driver preflights.
func TestExportGraph_FeaturesCapabilityList(t *testing.T) {
	s := state.NewState()
	addTestNode(t, s, "1", "Root", schema.NodeTypeClaim, schema.InferenceModusPonens, schema.EpistemicPending, node.TaintClean)
	out, err := ExportGraph(s, "/tmp/ws", &config.Config{})
	if err != nil {
		t.Fatalf("ExportGraph: %v", err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	rawFeatures, ok := doc["features"].([]interface{})
	if !ok {
		t.Fatalf("export must always carry a features[] array; got %T", doc["features"])
	}
	have := map[string]bool{}
	for _, f := range rawFeatures {
		have[f.(string)] = true
	}
	for _, want := range []string{FeatureReadinessFlags, FeatureClosureFlag, FeatureNodeDependencies} {
		if !have[want] {
			t.Errorf("features[] missing capability %q; have %v", want, rawFeatures)
		}
	}
}
