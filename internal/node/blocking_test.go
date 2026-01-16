package node_test

import (
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/types"
)

// Helper to create a PendingDef with a specific ID for testing.
func makePendingDef(t *testing.T, id, term, requestedByStr string) *node.PendingDef {
	t.Helper()
	requestedBy, err := types.Parse(requestedByStr)
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", requestedByStr, err)
	}
	pd, err := node.NewPendingDef(term, requestedBy)
	if err != nil {
		t.Fatalf("NewPendingDef() unexpected error: %v", err)
	}
	// Override the auto-generated ID for test predictability
	pd.ID = id
	return pd
}

// Helper to parse NodeID or fail test.
func mustParseNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", s, err)
	}
	return id
}

// ============================================================================
// Tests for IsBlocked
// ============================================================================

func TestIsBlocked_DirectlyBlocked(t *testing.T) {
	// Node 1.1 requested a definition and is directly blocked
	pd := makePendingDef(t, "def-001", "group", "1.1")
	pendingDefs := []*node.PendingDef{pd}
	nodeID := mustParseNodeID(t, "1.1")

	got := node.IsBlocked(nodeID, pendingDefs)
	if !got {
		t.Errorf("IsBlocked(%q) = false, want true (node requested the pending def)", nodeID.String())
	}
}

func TestIsBlocked_NotBlocked_NoPendingDefs(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.1")
	pendingDefs := []*node.PendingDef{}

	got := node.IsBlocked(nodeID, pendingDefs)
	if got {
		t.Errorf("IsBlocked(%q) = true, want false (no pending defs)", nodeID.String())
	}
}

func TestIsBlocked_NotBlocked_DifferentNode(t *testing.T) {
	// Node 1.2 requested the definition, but we check 1.1
	pd := makePendingDef(t, "def-001", "group", "1.2")
	pendingDefs := []*node.PendingDef{pd}
	nodeID := mustParseNodeID(t, "1.1")

	got := node.IsBlocked(nodeID, pendingDefs)
	if got {
		t.Errorf("IsBlocked(%q) = true, want false (different node requested the def)", nodeID.String())
	}
}

func TestIsBlocked_NotBlocked_ResolvedDef(t *testing.T) {
	// Node 1.1 requested a definition but it's already resolved
	pd := makePendingDef(t, "def-001", "group", "1.1")
	_ = pd.Resolve("resolved-def-id")
	pendingDefs := []*node.PendingDef{pd}
	nodeID := mustParseNodeID(t, "1.1")

	got := node.IsBlocked(nodeID, pendingDefs)
	if got {
		t.Errorf("IsBlocked(%q) = true, want false (def is resolved)", nodeID.String())
	}
}

func TestIsBlocked_NotBlocked_CancelledDef(t *testing.T) {
	// Node 1.1 requested a definition but it was cancelled
	pd := makePendingDef(t, "def-001", "group", "1.1")
	_ = pd.Cancel()
	pendingDefs := []*node.PendingDef{pd}
	nodeID := mustParseNodeID(t, "1.1")

	got := node.IsBlocked(nodeID, pendingDefs)
	if got {
		t.Errorf("IsBlocked(%q) = true, want false (def is cancelled)", nodeID.String())
	}
}

func TestIsBlocked_MultiplePendingDefs(t *testing.T) {
	// Multiple pending defs from different nodes
	pd1 := makePendingDef(t, "def-001", "group", "1.1")
	pd2 := makePendingDef(t, "def-002", "ring", "1.2")
	pd3 := makePendingDef(t, "def-003", "field", "1.3")
	pendingDefs := []*node.PendingDef{pd1, pd2, pd3}

	tests := []struct {
		nodeID string
		want   bool
	}{
		{"1.1", true},
		{"1.2", true},
		{"1.3", true},
		{"1.4", false},
		{"1", false},
	}

	for _, tt := range tests {
		t.Run(tt.nodeID, func(t *testing.T) {
			nodeID := mustParseNodeID(t, tt.nodeID)
			got := node.IsBlocked(nodeID, pendingDefs)
			if got != tt.want {
				t.Errorf("IsBlocked(%q) = %v, want %v", tt.nodeID, got, tt.want)
			}
		})
	}
}

func TestIsBlocked_NilPendingDefs(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.1")

	got := node.IsBlocked(nodeID, nil)
	if got {
		t.Errorf("IsBlocked(%q, nil) = true, want false", nodeID.String())
	}
}

// ============================================================================
// Tests for GetBlockingDef
// ============================================================================

func TestGetBlockingDef_Found(t *testing.T) {
	pd := makePendingDef(t, "def-001", "group", "1.1")
	pendingDefs := []*node.PendingDef{pd}
	nodeID := mustParseNodeID(t, "1.1")

	got := node.GetBlockingDef(nodeID, pendingDefs)
	if got == nil {
		t.Fatal("GetBlockingDef() = nil, want non-nil")
	}
	if got.ID != "def-001" {
		t.Errorf("GetBlockingDef().ID = %q, want %q", got.ID, "def-001")
	}
}

func TestGetBlockingDef_NotFound(t *testing.T) {
	pd := makePendingDef(t, "def-001", "group", "1.2")
	pendingDefs := []*node.PendingDef{pd}
	nodeID := mustParseNodeID(t, "1.1")

	got := node.GetBlockingDef(nodeID, pendingDefs)
	if got != nil {
		t.Errorf("GetBlockingDef() = %v, want nil", got)
	}
}

func TestGetBlockingDef_ResolvedNotReturned(t *testing.T) {
	pd := makePendingDef(t, "def-001", "group", "1.1")
	_ = pd.Resolve("resolved-def-id")
	pendingDefs := []*node.PendingDef{pd}
	nodeID := mustParseNodeID(t, "1.1")

	got := node.GetBlockingDef(nodeID, pendingDefs)
	if got != nil {
		t.Errorf("GetBlockingDef() = %v, want nil (resolved def)", got)
	}
}

func TestGetBlockingDef_MultipleDefsReturnsFirst(t *testing.T) {
	// Same node has multiple pending defs - should return the first pending one
	pd1 := makePendingDef(t, "def-001", "group", "1.1")
	pd2 := makePendingDef(t, "def-002", "ring", "1.1")
	pendingDefs := []*node.PendingDef{pd1, pd2}
	nodeID := mustParseNodeID(t, "1.1")

	got := node.GetBlockingDef(nodeID, pendingDefs)
	if got == nil {
		t.Fatal("GetBlockingDef() = nil, want non-nil")
	}
	// Should return the first one found
	if got.ID != "def-001" {
		t.Errorf("GetBlockingDef().ID = %q, want %q", got.ID, "def-001")
	}
}

func TestGetBlockingDef_EmptyList(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.1")

	got := node.GetBlockingDef(nodeID, []*node.PendingDef{})
	if got != nil {
		t.Errorf("GetBlockingDef() = %v, want nil", got)
	}
}

func TestGetBlockingDef_NilList(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.1")

	got := node.GetBlockingDef(nodeID, nil)
	if got != nil {
		t.Errorf("GetBlockingDef(nil) = %v, want nil", got)
	}
}

// ============================================================================
// Tests for GetBlockedNodes
// ============================================================================

func TestGetBlockedNodes_SingleNode(t *testing.T) {
	pd := makePendingDef(t, "def-001", "group", "1.1")
	allNodes := []types.NodeID{
		mustParseNodeID(t, "1"),
		mustParseNodeID(t, "1.1"),
		mustParseNodeID(t, "1.2"),
	}

	got := node.GetBlockedNodes(pd, allNodes)
	if len(got) != 1 {
		t.Fatalf("GetBlockedNodes() returned %d nodes, want 1", len(got))
	}
	if got[0].String() != "1.1" {
		t.Errorf("GetBlockedNodes()[0] = %q, want %q", got[0].String(), "1.1")
	}
}

func TestGetBlockedNodes_NoMatch(t *testing.T) {
	pd := makePendingDef(t, "def-001", "group", "1.3")
	allNodes := []types.NodeID{
		mustParseNodeID(t, "1"),
		mustParseNodeID(t, "1.1"),
		mustParseNodeID(t, "1.2"),
	}

	got := node.GetBlockedNodes(pd, allNodes)
	if len(got) != 0 {
		t.Errorf("GetBlockedNodes() = %v, want empty slice", got)
	}
}

func TestGetBlockedNodes_ResolvedDefReturnsEmpty(t *testing.T) {
	pd := makePendingDef(t, "def-001", "group", "1.1")
	_ = pd.Resolve("resolved-id")
	allNodes := []types.NodeID{
		mustParseNodeID(t, "1.1"),
	}

	got := node.GetBlockedNodes(pd, allNodes)
	if len(got) != 0 {
		t.Errorf("GetBlockedNodes() = %v, want empty (resolved def)", got)
	}
}

func TestGetBlockedNodes_NilPendingDef(t *testing.T) {
	allNodes := []types.NodeID{
		mustParseNodeID(t, "1.1"),
	}

	got := node.GetBlockedNodes(nil, allNodes)
	if len(got) != 0 {
		t.Errorf("GetBlockedNodes(nil, ...) = %v, want empty", got)
	}
}

func TestGetBlockedNodes_EmptyNodeList(t *testing.T) {
	pd := makePendingDef(t, "def-001", "group", "1.1")

	got := node.GetBlockedNodes(pd, []types.NodeID{})
	if len(got) != 0 {
		t.Errorf("GetBlockedNodes() = %v, want empty", got)
	}
}

// ============================================================================
// Tests for ComputeBlockedSet
// ============================================================================

func TestComputeBlockedSet_DirectBlocking(t *testing.T) {
	// Node 1.1 requests a definition
	pd := makePendingDef(t, "def-001", "group", "1.1")
	pendingDefs := []*node.PendingDef{pd}
	deps := map[string][]string{}

	got := node.ComputeBlockedSet(pendingDefs, deps)

	if !got["1.1"] {
		t.Error("ComputeBlockedSet() should mark 1.1 as blocked")
	}
	if len(got) != 1 {
		t.Errorf("ComputeBlockedSet() blocked %d nodes, want 1", len(got))
	}
}

func TestComputeBlockedSet_TransitiveBlocking(t *testing.T) {
	// Node 1.1 requests a definition
	// Node 1.2 depends on 1.1
	// Node 1.3 depends on 1.2
	// Therefore 1.1, 1.2, and 1.3 should all be blocked
	pd := makePendingDef(t, "def-001", "group", "1.1")
	pendingDefs := []*node.PendingDef{pd}

	deps := map[string][]string{
		"1.2": {"1.1"},       // 1.2 depends on 1.1
		"1.3": {"1.2"},       // 1.3 depends on 1.2
	}

	got := node.ComputeBlockedSet(pendingDefs, deps)

	if !got["1.1"] {
		t.Error("ComputeBlockedSet() should mark 1.1 as blocked (direct)")
	}
	if !got["1.2"] {
		t.Error("ComputeBlockedSet() should mark 1.2 as blocked (transitive via 1.1)")
	}
	if !got["1.3"] {
		t.Error("ComputeBlockedSet() should mark 1.3 as blocked (transitive via 1.2)")
	}
}

func TestComputeBlockedSet_MultipleBlockingDefs(t *testing.T) {
	// Two separate blocking chains
	pd1 := makePendingDef(t, "def-001", "group", "1.1")
	pd2 := makePendingDef(t, "def-002", "ring", "1.2")
	pendingDefs := []*node.PendingDef{pd1, pd2}

	deps := map[string][]string{
		"1.3": {"1.1", "1.2"}, // 1.3 depends on both 1.1 and 1.2
	}

	got := node.ComputeBlockedSet(pendingDefs, deps)

	if !got["1.1"] {
		t.Error("ComputeBlockedSet() should mark 1.1 as blocked")
	}
	if !got["1.2"] {
		t.Error("ComputeBlockedSet() should mark 1.2 as blocked")
	}
	if !got["1.3"] {
		t.Error("ComputeBlockedSet() should mark 1.3 as blocked (depends on blocked nodes)")
	}
}

func TestComputeBlockedSet_UnblockedNode(t *testing.T) {
	// Node 1.1 is blocked, 1.2 is independent
	pd := makePendingDef(t, "def-001", "group", "1.1")
	pendingDefs := []*node.PendingDef{pd}

	deps := map[string][]string{
		// 1.2 has no dependencies
	}

	got := node.ComputeBlockedSet(pendingDefs, deps)

	if !got["1.1"] {
		t.Error("ComputeBlockedSet() should mark 1.1 as blocked")
	}
	if got["1.2"] {
		t.Error("ComputeBlockedSet() should NOT mark 1.2 as blocked (no dependencies)")
	}
}

func TestComputeBlockedSet_EmptyInputs(t *testing.T) {
	got := node.ComputeBlockedSet([]*node.PendingDef{}, map[string][]string{})
	if len(got) != 0 {
		t.Errorf("ComputeBlockedSet() = %v, want empty map", got)
	}
}

func TestComputeBlockedSet_NilInputs(t *testing.T) {
	got := node.ComputeBlockedSet(nil, nil)
	if got == nil {
		t.Error("ComputeBlockedSet(nil, nil) = nil, want empty map")
	}
	if len(got) != 0 {
		t.Errorf("ComputeBlockedSet(nil, nil) = %v, want empty map", got)
	}
}

func TestComputeBlockedSet_ResolvedDefsIgnored(t *testing.T) {
	// Resolved def should not cause blocking
	pd := makePendingDef(t, "def-001", "group", "1.1")
	_ = pd.Resolve("resolved-id")
	pendingDefs := []*node.PendingDef{pd}

	deps := map[string][]string{
		"1.2": {"1.1"},
	}

	got := node.ComputeBlockedSet(pendingDefs, deps)

	if got["1.1"] {
		t.Error("ComputeBlockedSet() should NOT mark 1.1 as blocked (def resolved)")
	}
	if got["1.2"] {
		t.Error("ComputeBlockedSet() should NOT mark 1.2 as blocked (dependency unblocked)")
	}
}

func TestComputeBlockedSet_CyclicDependencies(t *testing.T) {
	// Test that cyclic dependencies don't cause infinite loops
	pd := makePendingDef(t, "def-001", "group", "1.1")
	pendingDefs := []*node.PendingDef{pd}

	// Create a cycle: 1.2 -> 1.3 -> 1.2
	deps := map[string][]string{
		"1.2": {"1.1", "1.3"}, // 1.2 depends on 1.1 and 1.3
		"1.3": {"1.2"},        // 1.3 depends on 1.2 (cycle!)
	}

	// Should not hang
	got := node.ComputeBlockedSet(pendingDefs, deps)

	if !got["1.1"] {
		t.Error("ComputeBlockedSet() should mark 1.1 as blocked")
	}
	// Both 1.2 and 1.3 are in a cycle that depends on blocked 1.1
	if !got["1.2"] {
		t.Error("ComputeBlockedSet() should mark 1.2 as blocked")
	}
	if !got["1.3"] {
		t.Error("ComputeBlockedSet() should mark 1.3 as blocked")
	}
}

// ============================================================================
// Tests for WouldResolveBlocking
// ============================================================================

func TestWouldResolveBlocking_SingleNode(t *testing.T) {
	pd := makePendingDef(t, "def-001", "group", "1.1")
	pendingDefs := []*node.PendingDef{pd}

	got := node.WouldResolveBlocking("def-001", pendingDefs)

	if len(got) != 1 {
		t.Fatalf("WouldResolveBlocking() returned %d nodes, want 1", len(got))
	}
	if got[0].String() != "1.1" {
		t.Errorf("WouldResolveBlocking()[0] = %q, want %q", got[0].String(), "1.1")
	}
}

func TestWouldResolveBlocking_MultipleNodesWithSameDef(t *testing.T) {
	// Multiple nodes requested the same term
	pd1 := makePendingDef(t, "def-001", "group", "1.1")
	pd2 := makePendingDef(t, "def-001", "group", "1.2") // Same ID means same definition being resolved
	pendingDefs := []*node.PendingDef{pd1, pd2}

	got := node.WouldResolveBlocking("def-001", pendingDefs)

	if len(got) != 2 {
		t.Fatalf("WouldResolveBlocking() returned %d nodes, want 2", len(got))
	}

	// Check both nodes are present (order may vary)
	found11, found12 := false, false
	for _, n := range got {
		if n.String() == "1.1" {
			found11 = true
		}
		if n.String() == "1.2" {
			found12 = true
		}
	}
	if !found11 || !found12 {
		t.Errorf("WouldResolveBlocking() = %v, want nodes 1.1 and 1.2", got)
	}
}

func TestWouldResolveBlocking_NoMatch(t *testing.T) {
	pd := makePendingDef(t, "def-001", "group", "1.1")
	pendingDefs := []*node.PendingDef{pd}

	got := node.WouldResolveBlocking("def-999", pendingDefs)

	if len(got) != 0 {
		t.Errorf("WouldResolveBlocking() = %v, want empty", got)
	}
}

func TestWouldResolveBlocking_AlreadyResolved(t *testing.T) {
	pd := makePendingDef(t, "def-001", "group", "1.1")
	_ = pd.Resolve("resolved-id")
	pendingDefs := []*node.PendingDef{pd}

	got := node.WouldResolveBlocking("def-001", pendingDefs)

	if len(got) != 0 {
		t.Errorf("WouldResolveBlocking() = %v, want empty (already resolved)", got)
	}
}

func TestWouldResolveBlocking_EmptyList(t *testing.T) {
	got := node.WouldResolveBlocking("def-001", []*node.PendingDef{})
	if len(got) != 0 {
		t.Errorf("WouldResolveBlocking() = %v, want empty", got)
	}
}

func TestWouldResolveBlocking_NilList(t *testing.T) {
	got := node.WouldResolveBlocking("def-001", nil)
	if len(got) != 0 {
		t.Errorf("WouldResolveBlocking(nil) = %v, want empty", got)
	}
}

func TestWouldResolveBlocking_EmptyDefID(t *testing.T) {
	pd := makePendingDef(t, "def-001", "group", "1.1")
	pendingDefs := []*node.PendingDef{pd}

	got := node.WouldResolveBlocking("", pendingDefs)

	if len(got) != 0 {
		t.Errorf("WouldResolveBlocking(\"\") = %v, want empty", got)
	}
}

func TestWouldResolveBlocking_PartialMatch(t *testing.T) {
	// Only some pending defs match the ID
	pd1 := makePendingDef(t, "def-001", "group", "1.1")
	pd2 := makePendingDef(t, "def-002", "ring", "1.2")
	pd3 := makePendingDef(t, "def-001", "group", "1.3") // Same ID as pd1
	pendingDefs := []*node.PendingDef{pd1, pd2, pd3}

	got := node.WouldResolveBlocking("def-001", pendingDefs)

	if len(got) != 2 {
		t.Fatalf("WouldResolveBlocking() returned %d nodes, want 2", len(got))
	}

	// 1.2 should not be in the result
	for _, n := range got {
		if n.String() == "1.2" {
			t.Error("WouldResolveBlocking() should not include 1.2 (different def ID)")
		}
	}
}
