//go:build integration

package e2e

import (
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/scope"
	"github.com/tobias/vibefeld/internal/types"
)

// mustParseNodeID parses a node ID string or fails the test.
func mustParseNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("failed to parse node ID %q: %v", s, err)
	}
	return id
}

// TestScope_LocalAssumption tests that a local assumption creates a scope entry.
func TestScope_LocalAssumption(t *testing.T) {
	// 1. Create a scope entry for a local assumption node
	nodeID := mustParseNodeID(t, "1.1")
	statement := "Assume n is a positive integer"

	entry, err := scope.NewEntry(nodeID, statement)
	if err != nil {
		t.Fatalf("Failed to create scope entry: %v", err)
	}

	// 2. Check that the assumption is tracked and active
	if entry == nil {
		t.Fatal("Expected non-nil scope entry")
	}
	if !entry.IsActive() {
		t.Error("New scope entry should be active")
	}
	if entry.NodeID.String() != "1.1" {
		t.Errorf("NodeID = %q, want %q", entry.NodeID.String(), "1.1")
	}
	if entry.Statement != statement {
		t.Errorf("Statement = %q, want %q", entry.Statement, statement)
	}
	if entry.Discharged != nil {
		t.Error("New entry should not be discharged")
	}

	// 3. Verify it shows up in active entries list
	entries := []*scope.Entry{entry}
	activeEntries := scope.GetActiveEntries(entries)
	if len(activeEntries) != 1 {
		t.Errorf("GetActiveEntries returned %d entries, want 1", len(activeEntries))
	}
}

// TestScope_DischargeAssumption tests that discharging removes an assumption from active scope.
func TestScope_DischargeAssumption(t *testing.T) {
	// 1. Create proof with local assumption
	nodeID := mustParseNodeID(t, "1.2")
	statement := "Assume x > 0"

	entry, err := scope.NewEntry(nodeID, statement)
	if err != nil {
		t.Fatalf("Failed to create scope entry: %v", err)
	}

	// Verify entry is active
	if !entry.IsActive() {
		t.Error("Entry should be active before discharge")
	}

	// 2. Discharge the assumption
	err = entry.Discharge()
	if err != nil {
		t.Fatalf("Failed to discharge entry: %v", err)
	}

	// 3. Verify scope is clean (entry no longer active)
	if entry.IsActive() {
		t.Error("Entry should not be active after discharge")
	}
	if entry.Discharged == nil {
		t.Error("Discharged timestamp should be set")
	}

	// 4. Verify it no longer shows up in active entries
	entries := []*scope.Entry{entry}
	activeEntries := scope.GetActiveEntries(entries)
	if len(activeEntries) != 0 {
		t.Errorf("GetActiveEntries returned %d entries, want 0 after discharge", len(activeEntries))
	}
}

// TestScope_NestedScopes tests nested assumption scopes.
func TestScope_NestedScopes(t *testing.T) {
	// 1. Create multiple nested local assumptions
	outerID := mustParseNodeID(t, "1.1")
	innerID := mustParseNodeID(t, "1.1.1")

	outerEntry, err := scope.NewEntry(outerID, "Assume P")
	if err != nil {
		t.Fatalf("Failed to create outer scope entry: %v", err)
	}

	innerEntry, err := scope.NewEntry(innerID, "Assume Q")
	if err != nil {
		t.Fatalf("Failed to create inner scope entry: %v", err)
	}

	entries := []*scope.Entry{outerEntry, innerEntry}

	// Both should be active
	activeEntries := scope.GetActiveEntries(entries)
	if len(activeEntries) != 2 {
		t.Errorf("Expected 2 active entries, got %d", len(activeEntries))
	}

	// 2. Discharge inner first
	err = innerEntry.Discharge()
	if err != nil {
		t.Fatalf("Failed to discharge inner entry: %v", err)
	}

	// 3. Verify outer still active
	if !outerEntry.IsActive() {
		t.Error("Outer entry should still be active after discharging inner")
	}
	if innerEntry.IsActive() {
		t.Error("Inner entry should not be active after discharge")
	}

	activeEntries = scope.GetActiveEntries(entries)
	if len(activeEntries) != 1 {
		t.Errorf("Expected 1 active entry, got %d", len(activeEntries))
	}
	if activeEntries[0].NodeID.String() != "1.1" {
		t.Errorf("Active entry should be outer (1.1), got %s", activeEntries[0].NodeID.String())
	}

	// 4. Discharge outer
	err = outerEntry.Discharge()
	if err != nil {
		t.Fatalf("Failed to discharge outer entry: %v", err)
	}

	// 5. Verify all clean
	if outerEntry.IsActive() {
		t.Error("Outer entry should not be active after discharge")
	}

	activeEntries = scope.GetActiveEntries(entries)
	if len(activeEntries) != 0 {
		t.Errorf("Expected 0 active entries, got %d", len(activeEntries))
	}
}

// TestScope_InheritScope tests that child nodes inherit active scopes from parents.
func TestScope_InheritScope(t *testing.T) {
	// Create two entries - one active, one discharged
	activeID := mustParseNodeID(t, "1.1")
	dischargedID := mustParseNodeID(t, "1.2")

	activeEntry, err := scope.NewEntry(activeID, "Active assumption")
	if err != nil {
		t.Fatalf("Failed to create active entry: %v", err)
	}

	dischargedEntry, err := scope.NewEntry(dischargedID, "Discharged assumption")
	if err != nil {
		t.Fatalf("Failed to create discharged entry: %v", err)
	}

	// Discharge one of them
	err = dischargedEntry.Discharge()
	if err != nil {
		t.Fatalf("Failed to discharge entry: %v", err)
	}

	entries := []*scope.Entry{activeEntry, dischargedEntry}

	// Parent scope references both entries by their NodeID strings
	parentScope := []string{"1.1", "1.2"}

	// InheritScope should only return the active one
	inheritedScope := scope.InheritScope(parentScope, entries)
	if len(inheritedScope) != 1 {
		t.Errorf("Expected 1 inherited scope entry, got %d", len(inheritedScope))
	}
	if len(inheritedScope) > 0 && inheritedScope[0] != "1.1" {
		t.Errorf("Expected inherited scope to be '1.1', got %q", inheritedScope[0])
	}
}

// TestScope_ValidateScope tests validation of node scope references.
func TestScope_ValidateScope(t *testing.T) {
	// Create an active scope entry
	assumeID := mustParseNodeID(t, "1.1")
	entry, err := scope.NewEntry(assumeID, "Assume P")
	if err != nil {
		t.Fatalf("Failed to create scope entry: %v", err)
	}

	entries := []*scope.Entry{entry}

	// Create a node that references the active scope
	nodeID := mustParseNodeID(t, "1.1.1")
	n, err := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"Therefore Q",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Scope: []string{"1.1"}, // References the active assumption
		},
	)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	// Validation should pass - scope reference is to active entry
	err = scope.ValidateScope(n, entries)
	if err != nil {
		t.Errorf("ValidateScope should pass for valid scope reference: %v", err)
	}

	// Now discharge the entry
	err = entry.Discharge()
	if err != nil {
		t.Fatalf("Failed to discharge entry: %v", err)
	}

	// Validation should now fail - scope reference is to discharged entry
	err = scope.ValidateScope(n, entries)
	if err == nil {
		t.Error("ValidateScope should fail when scope references discharged entry")
	}
}

// TestScope_ValidateScopeBalance tests that all local_assume nodes must have matching discharges.
func TestScope_ValidateScopeBalance(t *testing.T) {
	// Create nodes with balanced scope (one assume, one discharge)
	assumeID := mustParseNodeID(t, "1.1")
	dischargeID := mustParseNodeID(t, "1.2")
	claimID := mustParseNodeID(t, "1.3")

	assumeNode, err := node.NewNode(
		assumeID,
		schema.NodeTypeLocalAssume,
		"Assume P for contradiction",
		schema.InferenceLocalAssume,
	)
	if err != nil {
		t.Fatalf("Failed to create assume node: %v", err)
	}

	dischargeNode, err := node.NewNode(
		dischargeID,
		schema.NodeTypeLocalDischarge,
		"Therefore not-P by contradiction",
		schema.InferenceLocalDischarge,
	)
	if err != nil {
		t.Fatalf("Failed to create discharge node: %v", err)
	}

	claimNode, err := node.NewNode(
		claimID,
		schema.NodeTypeClaim,
		"Regular claim",
		schema.InferenceModusPonens,
	)
	if err != nil {
		t.Fatalf("Failed to create claim node: %v", err)
	}

	// Balanced case: one assume, one discharge
	balancedNodes := []*node.Node{assumeNode, claimNode, dischargeNode}
	err = scope.ValidateScopeBalance(balancedNodes)
	if err != nil {
		t.Errorf("ValidateScopeBalance should pass for balanced scopes: %v", err)
	}

	// Unbalanced case: assume without discharge
	unbalancedNodes := []*node.Node{assumeNode, claimNode}
	err = scope.ValidateScopeBalance(unbalancedNodes)
	if err == nil {
		t.Error("ValidateScopeBalance should fail when assume has no matching discharge")
	}
}

// TestScope_MultipleAssumptions tests multiple assumptions at the same level.
func TestScope_MultipleAssumptions(t *testing.T) {
	// Create multiple assumptions at the same proof level
	id1 := mustParseNodeID(t, "1.1")
	id2 := mustParseNodeID(t, "1.2")
	id3 := mustParseNodeID(t, "1.3")

	entry1, _ := scope.NewEntry(id1, "Assume A")
	entry2, _ := scope.NewEntry(id2, "Assume B")
	entry3, _ := scope.NewEntry(id3, "Assume C")

	entries := []*scope.Entry{entry1, entry2, entry3}

	// All three should be active
	activeEntries := scope.GetActiveEntries(entries)
	if len(activeEntries) != 3 {
		t.Errorf("Expected 3 active entries, got %d", len(activeEntries))
	}

	// Discharge the middle one
	_ = entry2.Discharge()

	activeEntries = scope.GetActiveEntries(entries)
	if len(activeEntries) != 2 {
		t.Errorf("Expected 2 active entries after discharging one, got %d", len(activeEntries))
	}

	// Verify which ones are still active
	activeIDs := make(map[string]bool)
	for _, e := range activeEntries {
		activeIDs[e.NodeID.String()] = true
	}
	if !activeIDs["1.1"] {
		t.Error("Entry 1.1 should still be active")
	}
	if activeIDs["1.2"] {
		t.Error("Entry 1.2 should be discharged")
	}
	if !activeIDs["1.3"] {
		t.Error("Entry 1.3 should still be active")
	}
}

// TestScope_NilHandling tests that scope functions handle nil inputs gracefully.
func TestScope_NilHandling(t *testing.T) {
	// GetActiveEntries with nil
	activeEntries := scope.GetActiveEntries(nil)
	if len(activeEntries) != 0 {
		t.Errorf("GetActiveEntries(nil) should return empty slice, got %d entries", len(activeEntries))
	}

	// GetActiveEntries with nil entries in slice
	entries := []*scope.Entry{nil, nil}
	activeEntries = scope.GetActiveEntries(entries)
	if len(activeEntries) != 0 {
		t.Errorf("GetActiveEntries with nil entries should return empty slice, got %d entries", len(activeEntries))
	}

	// InheritScope with nil entries
	inherited := scope.InheritScope([]string{"1.1"}, nil)
	if len(inherited) != 0 {
		t.Errorf("InheritScope with nil entries should return empty slice, got %d", len(inherited))
	}

	// ValidateScope with nil node
	err := scope.ValidateScope(nil, nil)
	if err != nil {
		t.Errorf("ValidateScope(nil, nil) should return nil, got %v", err)
	}

	// ValidateScopeBalance with nil nodes
	err = scope.ValidateScopeBalance(nil)
	if err != nil {
		t.Errorf("ValidateScopeBalance(nil) should return nil, got %v", err)
	}
}

// TestScope_CannotDischargeZeroNodeID tests that zero NodeID cannot create a scope entry.
func TestScope_CannotDischargeZeroNodeID(t *testing.T) {
	var zeroID types.NodeID
	_, err := scope.NewEntry(zeroID, "Some statement")
	if err == nil {
		t.Error("NewEntry with zero NodeID should return error")
	}
}

// TestScope_CannotCreateEmptyStatement tests that empty statement cannot create a scope entry.
func TestScope_CannotCreateEmptyStatement(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.1")
	_, err := scope.NewEntry(nodeID, "")
	if err == nil {
		t.Error("NewEntry with empty statement should return error")
	}

	_, err = scope.NewEntry(nodeID, "   ")
	if err == nil {
		t.Error("NewEntry with whitespace-only statement should return error")
	}
}

// TestScope_DoubleDischargeError tests that discharging twice returns an error.
func TestScope_DoubleDischargeError(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.1")
	entry, _ := scope.NewEntry(nodeID, "Assume P")

	// First discharge succeeds
	err := entry.Discharge()
	if err != nil {
		t.Fatalf("First discharge should succeed: %v", err)
	}

	// Second discharge fails
	err = entry.Discharge()
	if err == nil {
		t.Error("Second discharge should return error")
	}
}
