package state

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// D3: applyNodeValidated must copy the recorded content hash/checked flag onto
// the node, and applyClaimTested must copy the test's content hash. Legacy
// events (no fields) leave the node/test empty.

func newD3StateNode(t *testing.T, id string) (*State, types.NodeID) {
	t.Helper()
	s := NewState()
	nodeID := mustParseNodeID(t, id)
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	s.AddNode(n)
	return s, nodeID
}

func TestApplyNodeValidated_RecordsContentHashAndChecked(t *testing.T) {
	s, nodeID := newD3StateNode(t, "1")

	event := ledger.NewNodeValidatedWithHash(nodeID, "", "verifier-9", "batch-3", "hash-abc", true)
	if err := Apply(s, event); err != nil {
		t.Fatalf("Apply NodeValidated: %v", err)
	}

	got := s.GetNode(nodeID)
	if got.ValidatedContentHash != "hash-abc" {
		t.Errorf("ValidatedContentHash = %q, want %q", got.ValidatedContentHash, "hash-abc")
	}
	if !got.ValidatedHashChecked {
		t.Error("ValidatedHashChecked = false, want true")
	}

	// Unvalidate clears both, like ValidatedBy/ValidationBatchID.
	if err := Apply(s, ledger.NewNodeUnvalidated(nodeID, "re-review", "verifier-9")); err != nil {
		t.Fatalf("Apply NodeUnvalidated: %v", err)
	}
	got = s.GetNode(nodeID)
	if got.ValidatedContentHash != "" || got.ValidatedHashChecked {
		t.Errorf("after unvalidate got %q/%v, want empty/false", got.ValidatedContentHash, got.ValidatedHashChecked)
	}
}

func TestApplyNodeValidated_LegacyEventLeavesFieldsEmpty(t *testing.T) {
	s, nodeID := newD3StateNode(t, "1")

	// NewNodeValidatedFull is the pre-D3 shape: no hash fields.
	if err := Apply(s, ledger.NewNodeValidatedFull(nodeID, "", "verifier-9", "")); err != nil {
		t.Fatalf("Apply NodeValidated: %v", err)
	}

	got := s.GetNode(nodeID)
	if got.ValidatedContentHash != "" {
		t.Errorf("ValidatedContentHash = %q, want empty for legacy event", got.ValidatedContentHash)
	}
	if got.ValidatedHashChecked {
		t.Error("ValidatedHashChecked = true, want false for legacy event")
	}
}

// TestReplayLegacyNodeValidated_LeavesFieldsEmpty exercises the actual replay
// path (parseEvent + Apply) on old-shape JSON with no content_hash or
// expected_hash_checked keys.
func TestReplayLegacyNodeValidated_LeavesFieldsEmpty(t *testing.T) {
	s, nodeID := newD3StateNode(t, "1")
	raw := []byte(`{"type":"node_validated","timestamp":"2026-01-01T00:00:00Z","node_id":"1","note":"","verified_by":"verifier-9","batch_id":"batch-3"}`)

	event, err := parseEvent(raw)
	if err != nil {
		t.Fatalf("parseEvent: %v", err)
	}
	if err := Apply(s, event); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := s.GetNode(nodeID)
	if got.ValidatedContentHash != "" {
		t.Errorf("ValidatedContentHash = %q, want empty after legacy replay", got.ValidatedContentHash)
	}
	if got.ValidatedHashChecked {
		t.Error("ValidatedHashChecked = true, want false after legacy replay")
	}
}

func TestApplyClaimTested_RecordsContentHash(t *testing.T) {
	s, nodeID := newD3StateNode(t, "1")

	event := ledger.NewClaimTested(nodeID, "script", "t.py", "", true, "ok", "agent-1")
	event.ContentHash = "test-hash"
	if err := Apply(s, event); err != nil {
		t.Fatalf("Apply ClaimTested: %v", err)
	}

	tests := s.GetClaimTests(nodeID)
	if len(tests) != 1 {
		t.Fatalf("got %d claim tests, want 1", len(tests))
	}
	if tests[0].ContentHash != "test-hash" {
		t.Errorf("ContentHash = %q, want %q", tests[0].ContentHash, "test-hash")
	}
}

// TestHasPassingClaimTestForContent covers D3's acceptance rule: a passing
// test counts when its hash is a legacy empty string or matches the current
// content; a passing test with a different non-empty hash is stale and
// ignored. HasStalePassingClaimTest names the stale case.
func TestHasPassingClaimTestForContent(t *testing.T) {
	s, nodeID := newD3StateNode(t, "1")

	// No tests: nothing passes.
	if s.HasPassingClaimTestForContent(nodeID, "current") {
		t.Error("no tests: HasPassingClaimTestForContent = true, want false")
	}

	// Stale passing test only.
	s.AddClaimTest(nodeID, ClaimTestResult{Passed: true, ContentHash: "old"})
	if s.HasPassingClaimTestForContent(nodeID, "current") {
		t.Error("stale test must not count")
	}
	if !s.HasStalePassingClaimTest(nodeID, "current") {
		t.Error("HasStalePassingClaimTest = false, want true")
	}

	// Legacy passing test (no hash) always counts.
	s.AddClaimTest(nodeID, ClaimTestResult{Passed: true})
	if !s.HasPassingClaimTestForContent(nodeID, "current") {
		t.Error("legacy test must count")
	}
}

// TestHasPassingClaimTest_CountsHashBearingPass is a regression test for the
// legacy wrapper: it must count any passing test, including one with a
// non-empty recorded hash, even though it does not take a current hash.
func TestHasPassingClaimTest_CountsHashBearingPass(t *testing.T) {
	s, nodeID := newD3StateNode(t, "1")

	if s.HasPassingClaimTest(nodeID) {
		t.Error("no tests: HasPassingClaimTest = true, want false")
	}

	s.AddClaimTest(nodeID, ClaimTestResult{Passed: false, ContentHash: "hash-x"})
	if s.HasPassingClaimTest(nodeID) {
		t.Error("failing hash-bearing test must not count")
	}

	s.AddClaimTest(nodeID, ClaimTestResult{Passed: true, ContentHash: "hash-x"})
	if !s.HasPassingClaimTest(nodeID) {
		t.Error("passing hash-bearing test must count for the legacy wrapper")
	}
}

func TestHasPassingClaimTestForContent_MatchingHashCounts(t *testing.T) {
	s, nodeID := newD3StateNode(t, "1")
	s.AddClaimTest(nodeID, ClaimTestResult{Passed: true, ContentHash: "current"})

	if !s.HasPassingClaimTestForContent(nodeID, "current") {
		t.Error("matching-hash test must count")
	}
	if s.HasStalePassingClaimTest(nodeID, "current") {
		t.Error("matching-hash test must not be stale")
	}
}
