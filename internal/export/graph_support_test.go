package export

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func TestGraphExport_SupportCurrentCapabilityAndFields(t *testing.T) {
	st := state.NewState()
	rootID, _ := types.Parse("1")
	root, err := node.NewNode(rootID, schema.NodeTypeClaim, "root", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}
	root.EpistemicState = schema.EpistemicValidated
	root.VerdictSeq = 2
	st.AddNode(root)

	childID, _ := types.Parse("1.1")
	child, err := node.NewNode(childID, schema.NodeTypeClaim, "child", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}
	child.EpistemicState = schema.EpistemicPending
	st.AddNode(child)

	ge := BuildGraphExport(st, "ws", nil)

	foundCap := false
	for _, f := range ge.Features {
		if f == FeatureSupportCurrent {
			foundCap = true
		}
	}
	if !foundCap {
		t.Fatalf("features missing %q: %v", FeatureSupportCurrent, ge.Features)
	}

	var rootNode, childNode *GraphNode
	for i := range ge.Nodes {
		switch ge.Nodes[i].ID {
		case "1":
			rootNode = &ge.Nodes[i]
		case "1.1":
			childNode = &ge.Nodes[i]
		}
	}
	if rootNode == nil || childNode == nil {
		t.Fatalf("nodes not exported: %+v", ge.Nodes)
	}
	if rootNode.SupportCurrent {
		t.Error("root with a pending child should not be support_current")
	}
	if rootNode.SupportCause != "TARGET_PENDING" {
		t.Errorf("root support_cause = %q, want TARGET_PENDING", rootNode.SupportCause)
	}
	if childNode.SupportCurrent {
		t.Error("pending child should not be support_current")
	}
}

// TestGraphExport_SupportCurrentSerializedFalse locks that support_current is
// emitted even when false: an external driver must be able to read the
// negative signal, so the field is not omitempty. The assertion is on the
// marshalled JSON, not the Go struct (where false and absent are
// indistinguishable).
func TestGraphExport_SupportCurrentSerializedFalse(t *testing.T) {
	st := state.NewState()
	rootID, _ := types.Parse("1")
	root, err := node.NewNode(rootID, schema.NodeTypeClaim, "root", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}
	// Not validated, so support_current is false.
	st.AddNode(root)

	ge := BuildGraphExport(st, "ws", nil)
	data, err := json.Marshal(ge)
	if err != nil {
		t.Fatal(err)
	}
	js := string(data)
	if !strings.Contains(js, `"support_current":false`) {
		t.Fatalf("marshalled export drops support_current=false:\n%s", js)
	}
	// support_cause keeps omitempty, so an empty cause is absent.
	if strings.Contains(js, `"support_cause":""`) {
		t.Fatalf("empty support_cause should stay omitted:\n%s", js)
	}
}
