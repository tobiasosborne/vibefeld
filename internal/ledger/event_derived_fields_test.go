package ledger

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// TestNodeCreatedEvent_OmitsDerivedVerdictSeq locks that the derived verdict
// baseline (node.Node.VerdictSeq) never leaks into the wire event. node.Node is
// embedded in NodeCreated, so without an explicit json:"-" tag the derived
// field would silently change the event shape. Asserted on the marshalled JSON.
func TestNodeCreatedEvent_OmitsDerivedVerdictSeq(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	// Populate the derived field to prove the tag, not the zero value, hides it.
	n.VerdictSeq = 42

	data, err := json.Marshal(NewNodeCreated(*n))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	js := string(data)
	for _, banned := range []string{"verdict_seq", "validated_seq"} {
		if strings.Contains(js, banned) {
			t.Fatalf("NodeCreated JSON leaks derived field %q:\n%s", banned, js)
		}
	}
	if !strings.Contains(js, `"statement":"Test statement"`) {
		t.Fatalf("NodeCreated JSON missing the node payload:\n%s", js)
	}
}
