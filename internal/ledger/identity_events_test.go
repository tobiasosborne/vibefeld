// Package ledger provides event-sourced ledger operations for the AF proof framework.
//
// This file is deliberately NOT gated behind `-tags integration` (unlike
// event_test.go, which is — see vibefeld's known-broken integration suite).
// It exists so the rk PRD C3 / item V1 identity-field guarantees are
// exercised under the actual acceptance bar, `go test ./...`.
package ledger

import (
	"encoding/json"
	"testing"

	"github.com/tobias/vibefeld/internal/types"
)

// TestNodeValidatedFull_RecordsVerifierAndBatchID covers rk-9pk / PRD C3 V1:
// NodeValidated must be able to carry a verifier identity and an optional
// batch id, both driver-supplied provenance.
func TestNodeValidatedFull_RecordsVerifierAndBatchID(t *testing.T) {
	nodeID, _ := types.Parse("1.1")
	event := NewNodeValidatedFull(nodeID, "note text", "verifier-7", "batch-42")

	if event.VerifiedBy != "verifier-7" {
		t.Errorf("VerifiedBy = %q, want %q", event.VerifiedBy, "verifier-7")
	}
	if event.BatchID != "batch-42" {
		t.Errorf("BatchID = %q, want %q", event.BatchID, "batch-42")
	}

	// Plain NewNodeValidated / NewNodeValidatedWithNote must leave both empty.
	plain := NewNodeValidated(nodeID)
	if plain.VerifiedBy != "" || plain.BatchID != "" {
		t.Errorf("NewNodeValidated should leave VerifiedBy/BatchID empty, got %q/%q", plain.VerifiedBy, plain.BatchID)
	}
}

// TestNodeValidatedFull_JSONRoundTrip proves the new fields survive a
// marshal/unmarshal cycle, the same guarantee `af replay` depends on.
func TestNodeValidatedFull_JSONRoundTrip(t *testing.T) {
	nodeID, _ := types.Parse("1.4")
	original := NewNodeValidatedFull(nodeID, "", "verifier-alpha", "batch-9")

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var decoded NodeValidated
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded.VerifiedBy != original.VerifiedBy {
		t.Errorf("VerifiedBy mismatch: got %q, want %q", decoded.VerifiedBy, original.VerifiedBy)
	}
	if decoded.BatchID != original.BatchID {
		t.Errorf("BatchID mismatch: got %q, want %q", decoded.BatchID, original.BatchID)
	}
}

// TestNodeValidated_DecodesOldShapeJSON proves backward compatibility: a
// NodeValidated event written by code that predates VerifiedBy/BatchID (i.e.
// every event in the historical ledger corpus) has no verified_by or
// batch_id key at all. It must decode cleanly with both fields zero-valued,
// never error — this is the read-side half of the "old ledgers replay
// identically" requirement.
func TestNodeValidated_DecodesOldShapeJSON(t *testing.T) {
	oldJSON := `{"type":"node_validated","timestamp":"2026-01-01T00:00:00Z","node_id":"1.1","note":""}`

	var decoded NodeValidated
	if err := json.Unmarshal([]byte(oldJSON), &decoded); err != nil {
		t.Fatalf("Unmarshal of old-shape event failed: %v", err)
	}
	if decoded.VerifiedBy != "" {
		t.Errorf("VerifiedBy = %q, want empty string for old-shape event", decoded.VerifiedBy)
	}
	if decoded.BatchID != "" {
		t.Errorf("BatchID = %q, want empty string for old-shape event", decoded.BatchID)
	}
	if decoded.NodeID.String() != "1.1" {
		t.Errorf("NodeID = %q, want %q", decoded.NodeID.String(), "1.1")
	}
}

// TestChallengeRaisedWithBatch_RecordsBatchID covers the symmetric addition
// on ChallengeRaised: a batch's mixed accept/challenge verdict list (`af
// verdicts apply`, item V2) needs a batch id on whichever event kind each
// item produces, not just NodeValidated.
func TestChallengeRaisedWithBatch_RecordsBatchID(t *testing.T) {
	nodeID, _ := types.Parse("1.5")
	event := NewChallengeRaisedWithBatch("chal-batch-1", nodeID, "statement", "reason", "major", "verifier-9", "gap", "batch-7")

	if event.BatchID != "batch-7" {
		t.Errorf("BatchID = %q, want %q", event.BatchID, "batch-7")
	}
	if event.RaisedBy != "verifier-9" {
		t.Errorf("RaisedBy = %q, want %q", event.RaisedBy, "verifier-9")
	}

	// NewChallengeRaisedFull (the existing, non-batch constructor) must
	// leave BatchID empty.
	full := NewChallengeRaisedFull("chal-nobatch", nodeID, "statement", "reason", "major", "verifier-1", "")
	if full.BatchID != "" {
		t.Errorf("NewChallengeRaisedFull should leave BatchID empty, got %q", full.BatchID)
	}
}

// TestNodeValidated_WireFieldNames pins the exact JSON key names for the new
// fields (verified_by, batch_id). Unlike the round-trip test, this catches a
// struct-tag rename or typo: marshal alone must produce these exact keys.
func TestNodeValidated_WireFieldNames(t *testing.T) {
	nodeID, _ := types.Parse("1.9")
	event := NewNodeValidatedFull(nodeID, "", "verifier-99", "batch-99")

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}
	if raw["verified_by"] != "verifier-99" {
		t.Errorf(`wire field "verified_by" = %v, want "verifier-99" (got raw: %s)`, raw["verified_by"], data)
	}
	if raw["batch_id"] != "batch-99" {
		t.Errorf(`wire field "batch_id" = %v, want "batch-99" (got raw: %s)`, raw["batch_id"], data)
	}
}

// TestChallengeRaised_WireFieldName is the ChallengeRaised counterpart to
// TestNodeValidated_WireFieldNames.
func TestChallengeRaised_WireFieldName(t *testing.T) {
	nodeID, _ := types.Parse("1.10")
	event := NewChallengeRaisedWithBatch("chal-wire", nodeID, "target", "reason", "major", "verifier-1", "", "batch-88")

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}
	if raw["batch_id"] != "batch-88" {
		t.Errorf(`wire field "batch_id" = %v, want "batch-88" (got raw: %s)`, raw["batch_id"], data)
	}
}

// TestChallengeRaised_DecodesOldShapeJSON is the ChallengeRaised counterpart
// to TestNodeValidated_DecodesOldShapeJSON.
func TestChallengeRaised_DecodesOldShapeJSON(t *testing.T) {
	oldJSON := `{"type":"challenge_raised","timestamp":"2026-01-01T00:00:00Z","challenge_id":"chal-old","node_id":"1.1","target":"statement","reason":"r","severity":"major","raised_by":"v1"}`

	var decoded ChallengeRaised
	if err := json.Unmarshal([]byte(oldJSON), &decoded); err != nil {
		t.Fatalf("Unmarshal of old-shape event failed: %v", err)
	}
	if decoded.BatchID != "" {
		t.Errorf("BatchID = %q, want empty string for old-shape event", decoded.BatchID)
	}
}
