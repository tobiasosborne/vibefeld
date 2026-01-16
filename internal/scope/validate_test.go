//go:build integration

package scope

import (
	"testing"

	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// Helper function to create a test node with scope
func makeTestNode(t *testing.T, idStr string, nodeType schema.NodeType, scope []string) *node.Node {
	t.Helper()
	id, err := types.Parse(idStr)
	if err != nil {
		t.Fatalf("failed to parse node ID %q: %v", idStr, err)
	}
	n, err := node.NewNodeWithOptions(
		id,
		nodeType,
		"Test statement",
		schema.InferenceAssumption,
		node.NodeOptions{
			Scope: scope,
		},
	)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	return n
}

// Helper function to create a scope entry
func makeTestEntry(t *testing.T, idStr string, discharged bool) *Entry {
	t.Helper()
	id, err := types.Parse(idStr)
	if err != nil {
		t.Fatalf("failed to parse node ID %q: %v", idStr, err)
	}
	entry, err := NewEntry(id, "Test assumption")
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}
	if discharged {
		if err := entry.Discharge(); err != nil {
			t.Fatalf("failed to discharge entry: %v", err)
		}
	}
	return entry
}

// =============================================================================
// ValidateScope Tests
// =============================================================================

func TestValidateScope_ValidScopeWithActiveEntries(t *testing.T) {
	// A node with scope references to active entries should pass validation
	entry1 := makeTestEntry(t, "1.1", false) // active
	entry2 := makeTestEntry(t, "1.2", false) // active
	activeEntries := []*Entry{entry1, entry2}

	n := makeTestNode(t, "1.3", schema.NodeTypeClaim, []string{"1.1", "1.2"})

	err := ValidateScope(n, activeEntries)

	if err != nil {
		t.Errorf("ValidateScope returned unexpected error: %v", err)
	}
}

func TestValidateScope_EmptyScopeAlwaysValid(t *testing.T) {
	// A node with no scope references should always pass
	activeEntries := []*Entry{makeTestEntry(t, "1.1", false)}

	n := makeTestNode(t, "1.2", schema.NodeTypeClaim, nil)

	err := ValidateScope(n, activeEntries)

	if err != nil {
		t.Errorf("ValidateScope returned unexpected error: %v", err)
	}
}

func TestValidateScope_EmptyScopeWithNoActiveEntries(t *testing.T) {
	// A node with no scope references and no active entries should pass
	n := makeTestNode(t, "1.1", schema.NodeTypeClaim, nil)

	err := ValidateScope(n, nil)

	if err != nil {
		t.Errorf("ValidateScope returned unexpected error: %v", err)
	}
}

func TestValidateScope_ReferenceToDischargedEntry_ScopeViolation(t *testing.T) {
	// Referencing a discharged entry should return SCOPE_VIOLATION
	entry1 := makeTestEntry(t, "1.1", true) // discharged
	activeEntries := []*Entry{entry1}

	n := makeTestNode(t, "1.2", schema.NodeTypeClaim, []string{"1.1"})

	err := ValidateScope(n, activeEntries)

	if err == nil {
		t.Fatal("ValidateScope should return error for reference to discharged entry")
	}

	code := errors.Code(err)
	if code != errors.SCOPE_VIOLATION {
		t.Errorf("expected SCOPE_VIOLATION error code, got %v", code)
	}
}

func TestValidateScope_ReferenceToNonExistentEntry_ScopeViolation(t *testing.T) {
	// Referencing an entry that doesn't exist should return SCOPE_VIOLATION
	entry1 := makeTestEntry(t, "1.1", false) // active
	activeEntries := []*Entry{entry1}

	// Node references "1.2" which doesn't exist
	n := makeTestNode(t, "1.3", schema.NodeTypeClaim, []string{"1.2"})

	err := ValidateScope(n, activeEntries)

	if err == nil {
		t.Fatal("ValidateScope should return error for reference to non-existent entry")
	}

	code := errors.Code(err)
	if code != errors.SCOPE_VIOLATION {
		t.Errorf("expected SCOPE_VIOLATION error code, got %v", code)
	}
}

func TestValidateScope_MixedValidAndInvalidReferences(t *testing.T) {
	// If any reference is invalid, the entire validation should fail
	entry1 := makeTestEntry(t, "1.1", false) // active
	entry2 := makeTestEntry(t, "1.2", true)  // discharged
	activeEntries := []*Entry{entry1, entry2}

	// Node references both valid (1.1) and invalid (1.2 - discharged)
	n := makeTestNode(t, "1.3", schema.NodeTypeClaim, []string{"1.1", "1.2"})

	err := ValidateScope(n, activeEntries)

	if err == nil {
		t.Fatal("ValidateScope should return error when any reference is invalid")
	}

	code := errors.Code(err)
	if code != errors.SCOPE_VIOLATION {
		t.Errorf("expected SCOPE_VIOLATION error code, got %v", code)
	}
}

func TestValidateScope_NilNode(t *testing.T) {
	// Nil node should not panic and may return error or succeed
	activeEntries := []*Entry{makeTestEntry(t, "1.1", false)}

	// This tests that the function handles nil gracefully
	// Depending on implementation, it might return an error or succeed
	err := ValidateScope(nil, activeEntries)

	// We just verify it doesn't panic - either outcome is acceptable
	_ = err
}

func TestValidateScope_AllReferencesExistAndActive(t *testing.T) {
	// Multiple active scope references should all validate
	entry1 := makeTestEntry(t, "1.1", false)
	entry2 := makeTestEntry(t, "1.2", false)
	entry3 := makeTestEntry(t, "1.3", false)
	activeEntries := []*Entry{entry1, entry2, entry3}

	n := makeTestNode(t, "1.4", schema.NodeTypeClaim, []string{"1.1", "1.2", "1.3"})

	err := ValidateScope(n, activeEntries)

	if err != nil {
		t.Errorf("ValidateScope returned unexpected error: %v", err)
	}
}

// =============================================================================
// ValidateScopeBalance Tests
// =============================================================================

func TestValidateScopeBalance_EmptyNodes(t *testing.T) {
	// Empty slice of nodes should pass validation
	err := ValidateScopeBalance([]*node.Node{})

	if err != nil {
		t.Errorf("ValidateScopeBalance returned unexpected error for empty nodes: %v", err)
	}
}

func TestValidateScopeBalance_NilNodes(t *testing.T) {
	// Nil nodes should pass validation
	err := ValidateScopeBalance(nil)

	if err != nil {
		t.Errorf("ValidateScopeBalance returned unexpected error for nil nodes: %v", err)
	}
}

func TestValidateScopeBalance_NoLocalAssumeOrDischarge(t *testing.T) {
	// Nodes without any local_assume or local_discharge should pass
	n1 := makeTestNode(t, "1", schema.NodeTypeClaim, nil)
	n2 := makeTestNode(t, "1.1", schema.NodeTypeClaim, nil)
	nodes := []*node.Node{n1, n2}

	err := ValidateScopeBalance(nodes)

	if err != nil {
		t.Errorf("ValidateScopeBalance returned unexpected error: %v", err)
	}
}

func TestValidateScopeBalance_MatchedLocalAssumeAndDischarge(t *testing.T) {
	// local_assume followed by local_discharge should pass
	n1 := makeTestNode(t, "1.1", schema.NodeTypeLocalAssume, nil)
	n2 := makeTestNode(t, "1.2", schema.NodeTypeLocalDischarge, nil)
	nodes := []*node.Node{n1, n2}

	err := ValidateScopeBalance(nodes)

	if err != nil {
		t.Errorf("ValidateScopeBalance returned unexpected error: %v", err)
	}
}

func TestValidateScopeBalance_MultipleMatchedPairs(t *testing.T) {
	// Multiple local_assume/local_discharge pairs should pass
	n1 := makeTestNode(t, "1.1", schema.NodeTypeLocalAssume, nil)
	n2 := makeTestNode(t, "1.2", schema.NodeTypeLocalDischarge, nil)
	n3 := makeTestNode(t, "1.3", schema.NodeTypeLocalAssume, nil)
	n4 := makeTestNode(t, "1.4", schema.NodeTypeLocalDischarge, nil)
	nodes := []*node.Node{n1, n2, n3, n4}

	err := ValidateScopeBalance(nodes)

	if err != nil {
		t.Errorf("ValidateScopeBalance returned unexpected error: %v", err)
	}
}

func TestValidateScopeBalance_NestedLocalAssumes(t *testing.T) {
	// Nested local_assume/local_discharge should pass
	// local_assume -> local_assume -> local_discharge -> local_discharge
	n1 := makeTestNode(t, "1.1", schema.NodeTypeLocalAssume, nil)
	n2 := makeTestNode(t, "1.1.1", schema.NodeTypeLocalAssume, nil)
	n3 := makeTestNode(t, "1.1.2", schema.NodeTypeLocalDischarge, nil)
	n4 := makeTestNode(t, "1.2", schema.NodeTypeLocalDischarge, nil)
	nodes := []*node.Node{n1, n2, n3, n4}

	err := ValidateScopeBalance(nodes)

	if err != nil {
		t.Errorf("ValidateScopeBalance returned unexpected error: %v", err)
	}
}

func TestValidateScopeBalance_UnclosedLocalAssume_ScopeUnclosed(t *testing.T) {
	// local_assume without matching discharge should return SCOPE_UNCLOSED
	n1 := makeTestNode(t, "1.1", schema.NodeTypeLocalAssume, nil)
	n2 := makeTestNode(t, "1.2", schema.NodeTypeClaim, nil)
	nodes := []*node.Node{n1, n2}

	err := ValidateScopeBalance(nodes)

	if err == nil {
		t.Fatal("ValidateScopeBalance should return error for unclosed local_assume")
	}

	code := errors.Code(err)
	if code != errors.SCOPE_UNCLOSED {
		t.Errorf("expected SCOPE_UNCLOSED error code, got %v", code)
	}
}

func TestValidateScopeBalance_MultipleUnclosedLocalAssumes(t *testing.T) {
	// Multiple local_assume without any discharge should return SCOPE_UNCLOSED
	n1 := makeTestNode(t, "1.1", schema.NodeTypeLocalAssume, nil)
	n2 := makeTestNode(t, "1.2", schema.NodeTypeLocalAssume, nil)
	nodes := []*node.Node{n1, n2}

	err := ValidateScopeBalance(nodes)

	if err == nil {
		t.Fatal("ValidateScopeBalance should return error for unclosed local_assumes")
	}

	code := errors.Code(err)
	if code != errors.SCOPE_UNCLOSED {
		t.Errorf("expected SCOPE_UNCLOSED error code, got %v", code)
	}
}

func TestValidateScopeBalance_PartiallyClosedScopes(t *testing.T) {
	// Two local_assume with only one local_discharge should fail
	n1 := makeTestNode(t, "1.1", schema.NodeTypeLocalAssume, nil)
	n2 := makeTestNode(t, "1.2", schema.NodeTypeLocalAssume, nil)
	n3 := makeTestNode(t, "1.3", schema.NodeTypeLocalDischarge, nil)
	nodes := []*node.Node{n1, n2, n3}

	err := ValidateScopeBalance(nodes)

	if err == nil {
		t.Fatal("ValidateScopeBalance should return error for partially closed scopes")
	}

	code := errors.Code(err)
	if code != errors.SCOPE_UNCLOSED {
		t.Errorf("expected SCOPE_UNCLOSED error code, got %v", code)
	}
}

func TestValidateScopeBalance_ExtraDischargeWithoutAssume(t *testing.T) {
	// local_discharge without matching local_assume might be handled differently
	// This tests the behavior when there's an unmatched discharge
	n1 := makeTestNode(t, "1.1", schema.NodeTypeLocalDischarge, nil)
	nodes := []*node.Node{n1}

	// This may or may not be an error depending on implementation
	// The spec mentions SCOPE_UNCLOSED for unbalanced assume/discharge
	err := ValidateScopeBalance(nodes)

	// Check that the function handles this case (either error or success)
	_ = err
}

func TestValidateScopeBalance_MixedNodeTypes(t *testing.T) {
	// Mix of claim, local_assume, and local_discharge should work when balanced
	n1 := makeTestNode(t, "1", schema.NodeTypeClaim, nil)
	n2 := makeTestNode(t, "1.1", schema.NodeTypeLocalAssume, nil)
	n3 := makeTestNode(t, "1.2", schema.NodeTypeClaim, nil)
	n4 := makeTestNode(t, "1.3", schema.NodeTypeLocalDischarge, nil)
	n5 := makeTestNode(t, "1.4", schema.NodeTypeClaim, nil)
	nodes := []*node.Node{n1, n2, n3, n4, n5}

	err := ValidateScopeBalance(nodes)

	if err != nil {
		t.Errorf("ValidateScopeBalance returned unexpected error: %v", err)
	}
}

func TestValidateScopeBalance_NilEntriesInSlice(t *testing.T) {
	// Nil entries in the nodes slice should be handled gracefully
	n1 := makeTestNode(t, "1.1", schema.NodeTypeLocalAssume, nil)
	n2 := makeTestNode(t, "1.2", schema.NodeTypeLocalDischarge, nil)
	nodes := []*node.Node{n1, nil, n2}

	// Function should handle nil entries without panicking
	err := ValidateScopeBalance(nodes)

	// Either succeeds (ignoring nil) or returns an error
	_ = err
}

func TestValidateScopeBalance_SingleLocalAssume_ScopeUnclosed(t *testing.T) {
	// Single local_assume with no other nodes should fail
	n1 := makeTestNode(t, "1", schema.NodeTypeLocalAssume, nil)
	nodes := []*node.Node{n1}

	err := ValidateScopeBalance(nodes)

	if err == nil {
		t.Fatal("ValidateScopeBalance should return error for single unclosed local_assume")
	}

	code := errors.Code(err)
	if code != errors.SCOPE_UNCLOSED {
		t.Errorf("expected SCOPE_UNCLOSED error code, got %v", code)
	}
}

func TestValidateScopeBalance_LocalAssumeAtEnd_ScopeUnclosed(t *testing.T) {
	// local_assume at the end without discharge should fail
	n1 := makeTestNode(t, "1.1", schema.NodeTypeClaim, nil)
	n2 := makeTestNode(t, "1.2", schema.NodeTypeLocalAssume, nil)
	nodes := []*node.Node{n1, n2}

	err := ValidateScopeBalance(nodes)

	if err == nil {
		t.Fatal("ValidateScopeBalance should return error for unclosed local_assume at end")
	}

	code := errors.Code(err)
	if code != errors.SCOPE_UNCLOSED {
		t.Errorf("expected SCOPE_UNCLOSED error code, got %v", code)
	}
}

// =============================================================================
// ValidateScopeClosure Tests
// =============================================================================

func TestValidateScopeClosure_NilNode(t *testing.T) {
	// Nil node should pass validation (nothing to check)
	err := ValidateScopeClosure(nil, nil)

	if err != nil {
		t.Errorf("ValidateScopeClosure(nil, nil) = %v, want nil", err)
	}
}

func TestValidateScopeClosure_NonLocalAssumeNode(t *testing.T) {
	// Non-local_assume nodes should pass validation (nothing to check)
	nodeTypes := []schema.NodeType{
		schema.NodeTypeClaim,
		schema.NodeTypeLocalDischarge,
		schema.NodeTypeCase,
		schema.NodeTypeQED,
	}

	for _, nt := range nodeTypes {
		t.Run(string(nt), func(t *testing.T) {
			n := makeTestNode(t, "1.1", nt, nil)
			err := ValidateScopeClosure(n, nil)

			if err != nil {
				t.Errorf("ValidateScopeClosure(%s, nil) = %v, want nil", nt, err)
			}
		})
	}
}

func TestValidateScopeClosure_LocalAssumeWithClosedScope(t *testing.T) {
	// local_assume with discharged scope should pass validation
	n := makeTestNode(t, "1.1", schema.NodeTypeLocalAssume, nil)
	entry := makeTestEntry(t, "1.1", true) // discharged

	err := ValidateScopeClosure(n, entry)

	if err != nil {
		t.Errorf("ValidateScopeClosure() = %v, want nil for discharged scope", err)
	}
}

func TestValidateScopeClosure_LocalAssumeWithOpenScope_ScopeUnclosed(t *testing.T) {
	// local_assume with active scope should return SCOPE_UNCLOSED
	n := makeTestNode(t, "1.1", schema.NodeTypeLocalAssume, nil)
	entry := makeTestEntry(t, "1.1", false) // active (not discharged)

	err := ValidateScopeClosure(n, entry)

	if err == nil {
		t.Fatal("ValidateScopeClosure should return error for active scope")
	}

	code := errors.Code(err)
	if code != errors.SCOPE_UNCLOSED {
		t.Errorf("expected SCOPE_UNCLOSED error code, got %v", code)
	}
}

func TestValidateScopeClosure_LocalAssumeWithNilScope_ScopeUnclosed(t *testing.T) {
	// local_assume with nil scope entry should return SCOPE_UNCLOSED
	n := makeTestNode(t, "1.1", schema.NodeTypeLocalAssume, nil)

	err := ValidateScopeClosure(n, nil)

	if err == nil {
		t.Fatal("ValidateScopeClosure should return error for nil scope entry")
	}

	code := errors.Code(err)
	if code != errors.SCOPE_UNCLOSED {
		t.Errorf("expected SCOPE_UNCLOSED error code, got %v", code)
	}
}

func TestValidateScopeClosure_ErrorMessageContainsNodeID(t *testing.T) {
	// Error message should contain the node ID for debugging
	n := makeTestNode(t, "1.2.3", schema.NodeTypeLocalAssume, nil)
	entry := makeTestEntry(t, "1.2.3", false) // active

	err := ValidateScopeClosure(n, entry)

	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	errMsg := err.Error()
	if !containsSubstring(errMsg, "1.2.3") {
		t.Errorf("Error message should contain node ID '1.2.3', got: %s", errMsg)
	}
}

// containsSubstring is a helper to check if a string contains a substring.
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
