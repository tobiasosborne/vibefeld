package scope

import (
	"testing"

	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Tracker Tests
// =============================================================================

func TestNewTracker(t *testing.T) {
	tracker := NewTracker()

	if tracker == nil {
		t.Fatal("NewTracker returned nil")
	}

	// New tracker should have no scopes
	scopes := tracker.AllScopes()
	if len(scopes) != 0 {
		t.Errorf("expected 0 scopes, got %d", len(scopes))
	}
}

func TestTracker_OpenScope(t *testing.T) {
	tracker := NewTracker()
	nodeID := mustParseNodeID(t, "1.1")
	statement := "Assume x > 0 for contradiction"

	err := tracker.OpenScope(nodeID, statement)

	if err != nil {
		t.Fatalf("OpenScope returned unexpected error: %v", err)
	}

	// Should now have one scope
	scopes := tracker.AllScopes()
	if len(scopes) != 1 {
		t.Errorf("expected 1 scope, got %d", len(scopes))
	}

	// The scope should be retrievable
	scope := tracker.GetScope(nodeID)
	if scope == nil {
		t.Fatal("GetScope returned nil for opened scope")
	}
	if scope.NodeID.String() != "1.1" {
		t.Errorf("scope NodeID = %q, want %q", scope.NodeID.String(), "1.1")
	}
	if scope.Statement != statement {
		t.Errorf("scope Statement = %q, want %q", scope.Statement, statement)
	}
	if !scope.IsActive() {
		t.Error("newly opened scope should be active")
	}
}

func TestTracker_OpenScope_InvalidNodeID(t *testing.T) {
	tracker := NewTracker()
	var zeroNodeID types.NodeID

	err := tracker.OpenScope(zeroNodeID, "some statement")

	if err == nil {
		t.Error("OpenScope should return error for zero NodeID")
	}
}

func TestTracker_OpenScope_EmptyStatement(t *testing.T) {
	tracker := NewTracker()
	nodeID := mustParseNodeID(t, "1.1")

	err := tracker.OpenScope(nodeID, "")

	if err == nil {
		t.Error("OpenScope should return error for empty statement")
	}
}

func TestTracker_OpenScope_Duplicate(t *testing.T) {
	tracker := NewTracker()
	nodeID := mustParseNodeID(t, "1.1")

	err := tracker.OpenScope(nodeID, "First assumption")
	if err != nil {
		t.Fatalf("first OpenScope failed: %v", err)
	}

	err = tracker.OpenScope(nodeID, "Second assumption")
	if err == nil {
		t.Error("OpenScope should return error for duplicate scope")
	}
}

func TestTracker_CloseScope(t *testing.T) {
	tracker := NewTracker()
	nodeID := mustParseNodeID(t, "1.1")

	err := tracker.OpenScope(nodeID, "Assume x > 0")
	if err != nil {
		t.Fatalf("OpenScope failed: %v", err)
	}

	err = tracker.CloseScope(nodeID)
	if err != nil {
		t.Fatalf("CloseScope returned unexpected error: %v", err)
	}

	scope := tracker.GetScope(nodeID)
	if scope == nil {
		t.Fatal("GetScope returned nil after close")
	}
	if scope.IsActive() {
		t.Error("scope should not be active after close")
	}
}

func TestTracker_CloseScope_NotFound(t *testing.T) {
	tracker := NewTracker()
	nodeID := mustParseNodeID(t, "1.1")

	err := tracker.CloseScope(nodeID)

	if err == nil {
		t.Error("CloseScope should return error for non-existent scope")
	}
}

func TestTracker_CloseScope_AlreadyClosed(t *testing.T) {
	tracker := NewTracker()
	nodeID := mustParseNodeID(t, "1.1")

	_ = tracker.OpenScope(nodeID, "Assume something")
	_ = tracker.CloseScope(nodeID)

	err := tracker.CloseScope(nodeID)
	if err == nil {
		t.Error("CloseScope should return error when closing already-closed scope")
	}
}

func TestTracker_GetActiveScopes(t *testing.T) {
	tracker := NewTracker()
	nodeID1 := mustParseNodeID(t, "1.1")
	nodeID2 := mustParseNodeID(t, "1.2")
	nodeID3 := mustParseNodeID(t, "1.3")

	_ = tracker.OpenScope(nodeID1, "Assume A")
	_ = tracker.OpenScope(nodeID2, "Assume B")
	_ = tracker.OpenScope(nodeID3, "Assume C")
	_ = tracker.CloseScope(nodeID2) // Close B

	active := tracker.GetActiveScopes()

	if len(active) != 2 {
		t.Errorf("expected 2 active scopes, got %d", len(active))
	}

	// Check that only 1.1 and 1.3 are active
	activeIDs := make(map[string]bool)
	for _, s := range active {
		activeIDs[s.NodeID.String()] = true
	}
	if !activeIDs["1.1"] {
		t.Error("expected 1.1 to be active")
	}
	if !activeIDs["1.3"] {
		t.Error("expected 1.3 to be active")
	}
	if activeIDs["1.2"] {
		t.Error("1.2 should not be active")
	}
}

func TestTracker_IsInScope(t *testing.T) {
	tracker := NewTracker()
	assumeID := mustParseNodeID(t, "1.1")
	childID := mustParseNodeID(t, "1.1.1")
	grandchildID := mustParseNodeID(t, "1.1.2.3")
	siblingID := mustParseNodeID(t, "1.2")

	_ = tracker.OpenScope(assumeID, "Assume x > 0")

	testCases := []struct {
		name     string
		nodeID   types.NodeID
		expected bool
	}{
		{"direct child is in scope", childID, true},
		{"grandchild is in scope", grandchildID, true},
		{"sibling is not in scope", siblingID, false},
		{"scope node itself is not in its own scope", assumeID, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inScope := tracker.IsInScope(tc.nodeID, assumeID)
			if inScope != tc.expected {
				t.Errorf("IsInScope(%s, %s) = %v, want %v",
					tc.nodeID.String(), assumeID.String(), inScope, tc.expected)
			}
		})
	}
}

func TestTracker_GetContainingScopes(t *testing.T) {
	tracker := NewTracker()
	scope1ID := mustParseNodeID(t, "1.1")
	scope2ID := mustParseNodeID(t, "1.1.1")
	nodeID := mustParseNodeID(t, "1.1.1.2")

	_ = tracker.OpenScope(scope1ID, "Assume A")
	_ = tracker.OpenScope(scope2ID, "Assume B (nested)")

	scopes := tracker.GetContainingScopes(nodeID)

	if len(scopes) != 2 {
		t.Errorf("expected 2 containing scopes, got %d", len(scopes))
	}

	// Both scopes should contain this node
	scopeIDs := make(map[string]bool)
	for _, s := range scopes {
		scopeIDs[s.NodeID.String()] = true
	}
	if !scopeIDs["1.1"] {
		t.Error("expected 1.1 to be a containing scope")
	}
	if !scopeIDs["1.1.1"] {
		t.Error("expected 1.1.1 to be a containing scope")
	}
}

func TestTracker_GetContainingScopes_ExcludesClosedScopes(t *testing.T) {
	tracker := NewTracker()
	scope1ID := mustParseNodeID(t, "1.1")
	scope2ID := mustParseNodeID(t, "1.1.1")
	nodeID := mustParseNodeID(t, "1.1.1.2")

	_ = tracker.OpenScope(scope1ID, "Assume A")
	_ = tracker.OpenScope(scope2ID, "Assume B (nested)")
	_ = tracker.CloseScope(scope1ID) // Close outer scope

	scopes := tracker.GetContainingScopes(nodeID)

	// Only scope2 should be returned since scope1 is closed
	if len(scopes) != 1 {
		t.Errorf("expected 1 containing scope, got %d", len(scopes))
	}
	if scopes[0].NodeID.String() != "1.1.1" {
		t.Errorf("expected containing scope to be 1.1.1, got %s", scopes[0].NodeID.String())
	}
}

func TestTracker_GetScopeDepth(t *testing.T) {
	tracker := NewTracker()
	scope1ID := mustParseNodeID(t, "1.1")
	scope2ID := mustParseNodeID(t, "1.1.1")
	scope3ID := mustParseNodeID(t, "1.1.1.2")
	nodeID := mustParseNodeID(t, "1.1.1.2.1")

	_ = tracker.OpenScope(scope1ID, "Assume A")
	_ = tracker.OpenScope(scope2ID, "Assume B")
	_ = tracker.OpenScope(scope3ID, "Assume C")

	depth := tracker.GetScopeDepth(nodeID)

	if depth != 3 {
		t.Errorf("expected scope depth 3, got %d", depth)
	}
}

func TestTracker_GetScopeDepth_NoScopes(t *testing.T) {
	tracker := NewTracker()
	nodeID := mustParseNodeID(t, "1.1")

	depth := tracker.GetScopeDepth(nodeID)

	if depth != 0 {
		t.Errorf("expected scope depth 0, got %d", depth)
	}
}

func TestTracker_GetScopeInfo(t *testing.T) {
	tracker := NewTracker()
	scopeID := mustParseNodeID(t, "1.1")
	childID := mustParseNodeID(t, "1.1.1")

	_ = tracker.OpenScope(scopeID, "Assume P for contradiction")

	info := tracker.GetScopeInfo(childID)

	if info == nil {
		t.Fatal("GetScopeInfo returned nil")
	}

	if info.Depth != 1 {
		t.Errorf("info.Depth = %d, want 1", info.Depth)
	}

	if len(info.ContainingScopes) != 1 {
		t.Errorf("expected 1 containing scope, got %d", len(info.ContainingScopes))
	}

	if info.ContainingScopes[0].NodeID.String() != "1.1" {
		t.Errorf("expected containing scope 1.1, got %s", info.ContainingScopes[0].NodeID.String())
	}
}

func TestTracker_GetScopeInfo_NoScopes(t *testing.T) {
	tracker := NewTracker()
	nodeID := mustParseNodeID(t, "1.1")

	info := tracker.GetScopeInfo(nodeID)

	if info == nil {
		t.Fatal("GetScopeInfo returned nil")
	}

	if info.Depth != 0 {
		t.Errorf("info.Depth = %d, want 0", info.Depth)
	}

	if len(info.ContainingScopes) != 0 {
		t.Errorf("expected 0 containing scopes, got %d", len(info.ContainingScopes))
	}
}

func TestTracker_GetContainingScopes_SortsOutermostFirst(t *testing.T) {
	// Test that scopes are sorted by depth (outermost first),
	// even if they're added in reverse order.
	// This ensures the swap logic in sorting is exercised.
	tracker := NewTracker()
	outerID := mustParseNodeID(t, "1")
	middleID := mustParseNodeID(t, "1.1")
	innerID := mustParseNodeID(t, "1.1.1")
	nodeID := mustParseNodeID(t, "1.1.1.2")

	// Add scopes in reverse order (innermost first) to trigger sorting
	_ = tracker.OpenScope(innerID, "Inner assume")
	_ = tracker.OpenScope(middleID, "Middle assume")
	_ = tracker.OpenScope(outerID, "Outer assume")

	scopes := tracker.GetContainingScopes(nodeID)

	if len(scopes) != 3 {
		t.Fatalf("expected 3 containing scopes, got %d", len(scopes))
	}

	// Should be sorted outermost to innermost
	if scopes[0].NodeID.String() != "1" {
		t.Errorf("expected first scope to be 1 (outermost), got %s", scopes[0].NodeID.String())
	}
	if scopes[1].NodeID.String() != "1.1" {
		t.Errorf("expected second scope to be 1.1, got %s", scopes[1].NodeID.String())
	}
	if scopes[2].NodeID.String() != "1.1.1" {
		t.Errorf("expected third scope to be 1.1.1 (innermost), got %s", scopes[2].NodeID.String())
	}
}

// =============================================================================
// ScopeInfo Tests
// =============================================================================

func TestScopeInfo_IsInAnyScope(t *testing.T) {
	info := &ScopeInfo{
		Depth: 2,
		ContainingScopes: []*Entry{
			{NodeID: mustParseNodeID(t, "1.1"), Statement: "A"},
			{NodeID: mustParseNodeID(t, "1.1.1"), Statement: "B"},
		},
	}

	if !info.IsInAnyScope() {
		t.Error("IsInAnyScope should return true when depth > 0")
	}

	infoEmpty := &ScopeInfo{Depth: 0, ContainingScopes: nil}
	if infoEmpty.IsInAnyScope() {
		t.Error("IsInAnyScope should return false when depth = 0")
	}
}
