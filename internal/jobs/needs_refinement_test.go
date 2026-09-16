package jobs_test

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/jobs"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
)

// TestNeedsRefinement_ChildStatesDecideRole locks D4's classifier change: a
// needs_refinement node is prover work while its children are not cleared, and
// becomes verifier work once they are.
func TestNeedsRefinement_ChildStatesDecideRole(t *testing.T) {
	cases := []struct {
		name          string
		childState    schema.EpistemicState
		hasChild      bool
		wantProver    bool
		wantVerifier  bool
	}{
		{"no children stays prover", "", false, true, false},
		{"pending child stays prover", schema.EpistemicPending, true, true, false},
		{"refuted child stays prover", schema.EpistemicRefuted, true, true, false},
		{"validated child becomes verifier", schema.EpistemicValidated, true, false, true},
		{"admitted child becomes verifier", schema.EpistemicAdmitted, true, false, true},
		{"archived child becomes verifier", schema.EpistemicArchived, true, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := createTestNode(t, "1", schema.WorkflowAvailable, schema.EpistemicNeedsRefinement)
			nodes := []*node.Node{parent}
			if tc.hasChild {
				child := createTestNode(t, "1.1", schema.WorkflowAvailable, tc.childState)
				nodes = append(nodes, child)
			}
			nodeMap := buildProverNodeMap(nodes)
			challengeMap := map[string][]*node.Challenge{}

			proverJobs := jobs.FindProverJobs(nodes, nodeMap, challengeMap)
			verifierJobs := jobs.FindVerifierJobs(nodes, nodeMap, challengeMap)

			if gotProver := containsNode(proverJobs, "1"); gotProver != tc.wantProver {
				t.Errorf("prover job = %v, want %v", gotProver, tc.wantProver)
			}
			if gotVerifier := containsNode(verifierJobs, "1"); gotVerifier != tc.wantVerifier {
				t.Errorf("verifier job = %v, want %v", gotVerifier, tc.wantVerifier)
			}
		})
	}
}

func containsNode(nodes []*node.Node, id string) bool {
	for _, n := range nodes {
		if n.ID.String() == id {
			return true
		}
	}
	return false
}
