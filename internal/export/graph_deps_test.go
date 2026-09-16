package export

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func TestBuildGraphExport_DependencyFields(t *testing.T) {
	st := state.NewState()
	id, _ := types.Parse("1.1")
	dep, _ := types.Parse("1.2")
	n, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "s", schema.InferenceModusPonens, node.NodeOptions{
		ValidationDeps: []types.NodeID{dep},
	})
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	st.AddNode(n)
	st.AddAmendment(id, state.Amendment{
		Kind:                 state.AmendmentKindDependencies,
		Seq:                  7,
		Owner:                "owner",
		Reason:               "reason",
		PreviousDependencies: []types.NodeID{dep},
		NewDependencies:      nil,
		Reopened:             true,
	})

	ge := BuildGraphExport(st, "ws", nil)

	var gn *GraphNode
	for i := range ge.Nodes {
		if ge.Nodes[i].ID == "1.1" {
			gn = &ge.Nodes[i]
		}
	}
	if gn == nil {
		t.Fatal("node 1.1 missing from export")
	}
	if len(gn.ValidationDeps) != 1 || gn.ValidationDeps[0] != "1.2" {
		t.Errorf("validation_deps = %v", gn.ValidationDeps)
	}
	if len(gn.DependencyAmendments) != 1 {
		t.Fatalf("dependency_amendments = %+v", gn.DependencyAmendments)
	}
	da := gn.DependencyAmendments[0]
	if da.Seq != 7 || da.Owner != "owner" || !da.Reopened {
		t.Errorf("amendment fields wrong: %+v", da)
	}

	foundVal, foundAmend := false, false
	for _, f := range ge.Features {
		if f == FeatureValidationDeps {
			foundVal = true
		}
		if f == FeatureDependencyAmendments {
			foundAmend = true
		}
	}
	if !foundVal || !foundAmend {
		t.Errorf("features = %v, want validation-deps and dependency-amendments", ge.Features)
	}
}
