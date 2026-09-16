package ledger

import (
	"encoding/json"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/types"
)

func TestNodeDepsAmended_MinFormatIsV11(t *testing.T) {
	if got := EventNodeDepsAmended.MinFormat(); got != "1.1" {
		t.Fatalf("MinFormat = %q, want 1.1", got)
	}
	// Legacy types stay 1.0.
	if got := EventNodeAmended.MinFormat(); got != "1.0" {
		t.Fatalf("EventNodeAmended MinFormat = %q, want 1.0", got)
	}
}

func TestNodeDepsAmended_JSONShape(t *testing.T) {
	id, _ := types.Parse("1.1")
	dep, _ := types.Parse("1.2")
	ev := NewNodeDepsAmended(id, nil, []types.NodeID{dep}, nil, nil, "owner", "reason", "oldhash", true)
	ev.OperationID = "op-1"

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, key := range []string{"type", "node_id", "new_dependencies", "owner", "reason", "previous_content_hash", "reopened", "operation_id"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing key %q in %s", key, data)
		}
	}
	if raw["type"] != string(EventNodeDepsAmended) {
		t.Errorf("type = %v", raw["type"])
	}

	// Round-trip back into the typed event.
	var back NodeDepsAmended
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	if !back.Reopened || back.OperationID != "op-1" || len(back.NewDependencies) != 1 {
		t.Errorf("round-trip mismatch: %+v", back)
	}
}

func TestNodeDepsAmended_OmitsEmptyFields(t *testing.T) {
	id, _ := types.Parse("1.1")
	ev := NewNodeDepsAmended(id, nil, nil, nil, nil, "owner", "", "", false)
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, key := range []string{"new_dependencies", "previous_dependencies", "previous_content_hash", "reason", "reopened", "operation_id"} {
		if _, ok := raw[key]; ok {
			t.Errorf("empty field %q should be omitted: %s", key, data)
		}
	}
}
