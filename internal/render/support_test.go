package render

import (
	"strings"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func mkNode(t *testing.T, id string, es schema.EpistemicState) *node.Node {
	t.Helper()
	nid, err := types.Parse(id)
	if err != nil {
		t.Fatalf("parse %s: %v", id, err)
	}
	n, err := node.NewNode(nid, schema.NodeTypeClaim, "stmt "+id, schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	n.EpistemicState = es
	return n
}

func TestRenderStatus_SupportMarkerAndJSON(t *testing.T) {
	st := state.NewState()
	root := mkNode(t, "1", schema.EpistemicValidated)
	root.ValidatedSeq = 2
	st.AddNode(root)
	child := mkNode(t, "1.1", schema.EpistemicPending)
	st.AddNode(child)

	text := RenderTreeForNodes(st, []*node.Node{root, child})
	// The validated root relying on a pending child is marked; the pending child
	// itself is not (it was never validated).
	var rootLine string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "1 [") {
			rootLine = line
		}
		if strings.HasPrefix(strings.TrimSpace(line), "1.1 [") && strings.Contains(line, "!") {
			t.Fatalf("pending child should not carry a support marker:\n%s", text)
		}
	}
	if !strings.HasSuffix(rootLine, "!") {
		t.Fatalf("expected a support marker on the validated root, got %q in:\n%s", rootLine, text)
	}

	jsonOut := RenderStatusJSON(st, 0, 0)
	if !strings.Contains(jsonOut, `"support_current":false`) {
		t.Fatalf("status JSON missing support_current:false:\n%s", jsonOut)
	}
	if !strings.Contains(jsonOut, `"support_cause":"TARGET_PENDING"`) {
		t.Fatalf("status JSON missing support_cause TARGET_PENDING:\n%s", jsonOut)
	}
}

func TestRenderLegend_MentionsSupport(t *testing.T) {
	var sb strings.Builder
	renderLegend(&sb)
	if !strings.Contains(sb.String(), "support_current") {
		t.Fatalf("legend does not explain support_current:\n%s", sb.String())
	}
}
