// This file pins the D3 additive event fields: NodeValidated.content_hash /
// expected_hash_checked and ClaimTested.content_hash. All three are optional
// (omitempty) and absent on legacy events, which must decode cleanly.
package ledger

import (
	"encoding/json"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/types"
)

// TestNodeValidatedWithHash_RecordsFields covers D3: an accept can record the
// content hash it accepted and whether a caller-supplied expectation was
// compared. Plain constructors leave both unrecorded.
func TestNodeValidatedWithHash_RecordsFields(t *testing.T) {
	nodeID, _ := types.Parse("1.1")
	event := NewNodeValidatedWithHash(nodeID, "note", "verifier-7", "batch-42", "abc123", true)

	if event.ContentHash != "abc123" {
		t.Errorf("ContentHash = %q, want %q", event.ContentHash, "abc123")
	}
	if !event.ExpectedHashChecked {
		t.Error("ExpectedHashChecked = false, want true")
	}

	plain := NewNodeValidatedFull(nodeID, "", "verifier-7", "batch-42")
	if plain.ContentHash != "" || plain.ExpectedHashChecked {
		t.Errorf("NewNodeValidatedFull should leave ContentHash/ExpectedHashChecked unset, got %q/%v",
			plain.ContentHash, plain.ExpectedHashChecked)
	}
}

// TestNodeValidatedWithHash_WireFieldNames pins the exact JSON keys so a tag
// rename cannot slip through.
func TestNodeValidatedWithHash_WireFieldNames(t *testing.T) {
	nodeID, _ := types.Parse("1.9")
	data, err := json.Marshal(NewNodeValidatedWithHash(nodeID, "", "v", "", "deadbeef", true))
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}
	if raw["content_hash"] != "deadbeef" {
		t.Errorf(`wire field "content_hash" = %v, want "deadbeef" (raw: %s)`, raw["content_hash"], data)
	}
	if raw["expected_hash_checked"] != true {
		t.Errorf(`wire field "expected_hash_checked" = %v, want true (raw: %s)`, raw["expected_hash_checked"], data)
	}

	// Omitted when unset (additive field contract).
	plain, _ := json.Marshal(NewNodeValidatedFull(nodeID, "", "", ""))
	var rawPlain map[string]interface{}
	if err := json.Unmarshal(plain, &rawPlain); err != nil {
		t.Fatalf("Unmarshal old-shape failed: %v", err)
	}
	if _, present := rawPlain["content_hash"]; present {
		t.Errorf("plain NodeValidated must omit content_hash, got %v", rawPlain["content_hash"])
	}
	if _, present := rawPlain["expected_hash_checked"]; present {
		t.Errorf("plain NodeValidated must omit expected_hash_checked, got %v", rawPlain["expected_hash_checked"])
	}
}

// TestNodeValidated_DecodesOldShapeJSONWithD3Fields proves an event written
// before D3 decodes with the new fields zero-valued.
func TestNodeValidated_DecodesOldShapeJSONWithD3Fields(t *testing.T) {
	oldJSON := `{"type":"node_validated","timestamp":"2026-01-01T00:00:00Z","node_id":"1.1","note":"","verified_by":"v1","batch_id":"b1"}`

	var decoded NodeValidated
	if err := json.Unmarshal([]byte(oldJSON), &decoded); err != nil {
		t.Fatalf("Unmarshal of old-shape event failed: %v", err)
	}
	if decoded.ContentHash != "" {
		t.Errorf("ContentHash = %q, want empty for old-shape event", decoded.ContentHash)
	}
	if decoded.ExpectedHashChecked {
		t.Error("ExpectedHashChecked = true, want false for old-shape event")
	}
}

// TestClaimTested_ContentHashRoundTrip covers D3's ClaimTested.content_hash:
// recorded at test time, omitted when empty, and decoded as empty from legacy
// JSON.
func TestClaimTested_ContentHashRoundTrip(t *testing.T) {
	nodeID, _ := types.Parse("1.2")
	event := NewClaimTested(nodeID, "script", "t.py", "", true, "ok", "agent-1")
	event.ContentHash = "cafebabe"

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}
	if raw["content_hash"] != "cafebabe" {
		t.Errorf(`wire field "content_hash" = %v, want "cafebabe" (raw: %s)`, raw["content_hash"], data)
	}

	legacy := `{"type":"claim_tested","timestamp":"2026-01-01T00:00:00Z","node_id":"1.2","engine":"script","passed":true}`
	var decoded ClaimTested
	if err := json.Unmarshal([]byte(legacy), &decoded); err != nil {
		t.Fatalf("Unmarshal of legacy event failed: %v", err)
	}
	if decoded.ContentHash != "" {
		t.Errorf("ContentHash = %q, want empty for legacy event", decoded.ContentHash)
	}
}
