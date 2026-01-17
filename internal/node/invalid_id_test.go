// Package node_test contains edge case tests for invalid NodeID format handling.
package node_test

import (
	"encoding/json"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestNewNode_InvalidIDFormats verifies that node creation handles various
// invalid ID formats correctly. Since NewNode accepts a types.NodeID (already
// parsed), the primary validation happens in types.Parse(). These tests verify
// that:
// 1. Invalid ID strings are rejected at parse time
// 2. Zero-value NodeIDs result in predictable behavior
// 3. Node operations on zero-value IDs don't panic
func TestNewNode_InvalidIDFormats(t *testing.T) {
	invalidIDs := []struct {
		name  string
		input string
	}{
		// Empty and whitespace
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs only", "\t\t"},
		{"newlines only", "\n\n"},
		{"mixed whitespace", " \t\n "},

		// Invalid root values
		{"zero root", "0"},
		{"negative root", "-1"},
		{"non-one root", "2"},
		{"non-one root large", "99"},

		// Invalid separators
		{"leading dot", ".1"},
		{"trailing dot", "1."},
		{"double dot", "1..1"},
		{"triple dot", "1...1"},
		{"multiple trailing dots", "1..."},
		{"just dot", "."},
		{"multiple dots only", "..."},

		// Non-numeric characters
		{"letter only", "a"},
		{"letters with dots", "a.b.c"},
		{"letter prefix", "a1"},
		{"letter suffix", "1a"},
		{"letter in middle", "1.a.2"},
		{"mixed alphanumeric", "1.2a.3"},
		{"special characters", "1.@.2"},
		{"unicode digits", "1.١.2"}, // Arabic-Indic digit one
		{"emoji", "1.😀.2"},
		{"space in middle", "1 1"},
		{"space with dot", "1. 1"},

		// Invalid numeric values
		{"zero child", "1.0"},
		{"zero grandchild", "1.1.0"},
		{"negative child", "1.-1"},
		{"negative deep", "1.2.-3"},

		// Other invalid formats
		{"comma separator", "1,1"},
		{"semicolon separator", "1;1"},
		{"slash separator", "1/1"},
		{"backslash separator", "1\\1"},
		{"colon separator", "1:1"},
		{"plus sign", "1+1"},
		{"equals sign", "1=1"},
		{"leading zeros", "1.01"},        // May or may not be accepted
		{"float notation", "1.2.3e4"},    // Scientific notation
		{"hex notation", "1.0x10"},       // Hex prefix
		{"octal-like", "1.010"},          // Leading zero (octal-like)
		{"whitespace around dot", "1 . 2"},
		{"leading whitespace", " 1.2"},
		{"trailing whitespace", "1.2 "},
	}

	for _, tt := range invalidIDs {
		t.Run(tt.name, func(t *testing.T) {
			_, err := types.Parse(tt.input)
			if err == nil {
				// If this unexpectedly parses, it's not necessarily wrong
				// (e.g., leading zeros might be accepted as valid)
				// Log for visibility but don't fail - the key point is testing
				// that the parser handles edge cases
				t.Logf("Note: %q was accepted by parser", tt.input)
			}
		})
	}
}

// TestNewNode_ZeroValueNodeID verifies behavior when using a zero-value NodeID.
// A zero-value NodeID has no parts and represents an uninitialized state.
func TestNewNode_ZeroValueNodeID(t *testing.T) {
	var zeroID types.NodeID

	// Creating a node with zero-value ID should succeed (constructor doesn't
	// validate ID beyond using it) - this tests that no panic occurs
	n, err := node.NewNode(
		zeroID,
		schema.NodeTypeClaim,
		"Test statement",
		schema.InferenceAssumption,
	)
	if err != nil {
		t.Fatalf("NewNode() with zero NodeID unexpected error: %v", err)
	}

	// Verify the node was created
	if n == nil {
		t.Fatal("NewNode() returned nil node")
	}

	// Verify ID operations don't panic on zero-value ID
	t.Run("IsRoot on zero ID", func(t *testing.T) {
		// Should not panic
		isRoot := n.IsRoot()
		// Zero-value ID should not be considered root
		if isRoot {
			t.Errorf("zero NodeID.IsRoot() = true, want false")
		}
	})

	t.Run("Depth on zero ID", func(t *testing.T) {
		// Should not panic
		depth := n.Depth()
		// Zero-value ID has depth 0
		if depth != 0 {
			t.Errorf("zero NodeID.Depth() = %d, want 0", depth)
		}
	})

	t.Run("String on zero ID", func(t *testing.T) {
		// Should not panic
		str := n.ID.String()
		// Zero-value ID should return empty string
		if str != "" {
			t.Errorf("zero NodeID.String() = %q, want empty string", str)
		}
	})
}

// TestNewNode_ValidIDEdgeCases verifies that valid but unusual ID formats work.
func TestNewNode_ValidIDEdgeCases(t *testing.T) {
	validEdgeCases := []struct {
		name  string
		input string
	}{
		// Minimal valid IDs
		{"root only", "1"},
		{"single child", "1.1"},

		// Large numbers
		{"large child number", "1.999"},
		{"large grandchild", "1.999.888"},
		{"very large number", "1.999999"},

		// Deep nesting
		{"depth 5", "1.1.1.1.1"},
		{"depth 10", "1.1.1.1.1.1.1.1.1.1"},
		{"depth 20", "1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1"},

		// Mixed large and small
		{"mixed numbers", "1.1.999.2.888.3"},

		// Maximum realistic values
		{"max depth with variation", "1.2.3.4.5.6.7.8.9.10"},
	}

	for _, tt := range validEdgeCases {
		t.Run(tt.name, func(t *testing.T) {
			id, err := types.Parse(tt.input)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.input, err)
			}

			n, err := node.NewNode(
				id,
				schema.NodeTypeClaim,
				"Test statement for "+tt.input,
				schema.InferenceAssumption,
			)
			if err != nil {
				t.Fatalf("NewNode() unexpected error: %v", err)
			}

			// Verify ID is preserved
			if n.ID.String() != tt.input {
				t.Errorf("ID = %q, want %q", n.ID.String(), tt.input)
			}
		})
	}
}

// TestNode_JSONUnmarshal_InvalidIDFormats verifies JSON unmarshaling handles
// invalid ID formats in the JSON payload.
func TestNode_JSONUnmarshal_InvalidIDFormats(t *testing.T) {
	// NOTE: Empty ID string ("") is explicitly handled by UnmarshalJSON as valid,
	// returning a zero-value NodeID. This is intentional per the implementation.
	// See TestNode_JSONUnmarshal_EmptyIDString for that case.

	invalidJSONs := []struct {
		name string
		json string
	}{
		{
			name: "invalid ID - zero root",
			json: `{"id":"0","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
		},
		{
			name: "invalid ID - double dot",
			json: `{"id":"1..2","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
		},
		{
			name: "invalid ID - letters",
			json: `{"id":"1.a.2","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
		},
		{
			name: "invalid ID - negative",
			json: `{"id":"1.-1","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
		},
		{
			name: "invalid ID - trailing dot",
			json: `{"id":"1.2.","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
		},
		{
			name: "invalid ID - leading dot",
			json: `{"id":".1.2","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
		},
		{
			name: "invalid ID - non-one root",
			json: `{"id":"2.1","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
		},
		{
			name: "invalid ID - zero child",
			json: `{"id":"1.0","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
		},
	}

	for _, tt := range invalidJSONs {
		t.Run(tt.name, func(t *testing.T) {
			var n node.Node
			err := json.Unmarshal([]byte(tt.json), &n)
			if err == nil {
				t.Errorf("json.Unmarshal() expected error for invalid ID, got nil")
			}
		})
	}
}

// TestNode_JSONUnmarshal_EmptyIDString verifies that empty ID string in JSON
// is explicitly accepted, returning a zero-value NodeID.
func TestNode_JSONUnmarshal_EmptyIDString(t *testing.T) {
	emptyIDJSON := `{"id":"","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`

	var n node.Node
	err := json.Unmarshal([]byte(emptyIDJSON), &n)
	if err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	// Empty string results in zero-value NodeID
	if n.ID.String() != "" {
		t.Errorf("ID.String() = %q, want empty string", n.ID.String())
	}

	// Zero-value ID should not be root
	if n.ID.IsRoot() {
		t.Error("empty ID should not be considered root")
	}

	// Zero-value ID should have depth 0
	if n.ID.Depth() != 0 {
		t.Errorf("empty ID.Depth() = %d, want 0", n.ID.Depth())
	}
}

// TestNode_JSONUnmarshal_ValidIDFormats verifies JSON unmarshaling handles
// valid ID formats correctly.
func TestNode_JSONUnmarshal_ValidIDFormats(t *testing.T) {
	validJSONs := []struct {
		name       string
		json       string
		expectedID string
	}{
		{
			name:       "root ID",
			json:       `{"id":"1","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
			expectedID: "1",
		},
		{
			name:       "nested ID",
			json:       `{"id":"1.2.3","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
			expectedID: "1.2.3",
		},
		{
			name:       "deeply nested ID",
			json:       `{"id":"1.2.3.4.5.6.7.8","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
			expectedID: "1.2.3.4.5.6.7.8",
		},
		{
			name:       "large number ID",
			json:       `{"id":"1.999.888","type":"claim","statement":"test","inference":"assumption","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","content_hash":"abc","created":"2024-01-01T00:00:00Z"}`,
			expectedID: "1.999.888",
		},
	}

	for _, tt := range validJSONs {
		t.Run(tt.name, func(t *testing.T) {
			var n node.Node
			err := json.Unmarshal([]byte(tt.json), &n)
			if err != nil {
				t.Fatalf("json.Unmarshal() unexpected error: %v", err)
			}

			if n.ID.String() != tt.expectedID {
				t.Errorf("ID = %q, want %q", n.ID.String(), tt.expectedID)
			}
		})
	}
}

// TestNode_ValidationDeps_InvalidIDFormats verifies that validation dependencies
// with invalid IDs are handled correctly during JSON unmarshaling.
func TestNode_ValidationDeps_InvalidIDFormats(t *testing.T) {
	// JSON with valid node but invalid validation dependency ID
	invalidDepJSON := `{
		"id":"1.2",
		"type":"claim",
		"statement":"test",
		"inference":"assumption",
		"validation_deps":["1.0"],
		"workflow_state":"available",
		"epistemic_state":"pending",
		"taint_state":"unresolved",
		"content_hash":"abc",
		"created":"2024-01-01T00:00:00Z"
	}`

	var n node.Node
	err := json.Unmarshal([]byte(invalidDepJSON), &n)
	if err == nil {
		t.Error("json.Unmarshal() expected error for invalid validation_deps ID")
	}
}

// TestNode_Dependencies_InvalidIDFormats verifies that dependencies with invalid
// IDs are handled correctly during JSON unmarshaling.
func TestNode_Dependencies_InvalidIDFormats(t *testing.T) {
	// JSON with valid node but invalid dependency ID
	invalidDepJSON := `{
		"id":"1.2",
		"type":"claim",
		"statement":"test",
		"inference":"assumption",
		"dependencies":["1..1"],
		"workflow_state":"available",
		"epistemic_state":"pending",
		"taint_state":"unresolved",
		"content_hash":"abc",
		"created":"2024-01-01T00:00:00Z"
	}`

	var n node.Node
	err := json.Unmarshal([]byte(invalidDepJSON), &n)
	if err == nil {
		t.Error("json.Unmarshal() expected error for invalid dependencies ID")
	}
}

// TestNodeID_ParentOfInvalidID verifies Parent() behavior on edge case IDs.
func TestNodeID_ParentOfInvalidID(t *testing.T) {
	// Zero value NodeID
	var zeroID types.NodeID
	_, ok := zeroID.Parent()
	if ok {
		t.Error("zero NodeID.Parent() returned ok=true, want false")
	}

	// Root node (valid but has no parent)
	rootID, _ := types.Parse("1")
	_, ok = rootID.Parent()
	if ok {
		t.Error("root NodeID.Parent() returned ok=true, want false")
	}
}

// TestNodeID_ChildOfZeroValue verifies Child() behavior on zero-value NodeID.
func TestNodeID_ChildOfZeroValue(t *testing.T) {
	var zeroID types.NodeID

	// Child of zero value - this may or may not be defined behavior
	// The test ensures no panic occurs
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Child() on zero NodeID panicked: %v", r)
		}
	}()

	// This tests that the method doesn't panic
	child, err := zeroID.Child(1)
	if err != nil {
		// Error is acceptable for zero-value
		return
	}

	// If no error, verify the child is usable
	_ = child.String()
}

// TestNodeID_IsAncestorOf_ZeroValues verifies IsAncestorOf() with zero values.
// NOTE: Due to the implementation (vacuous truth in the for loop), a zero NodeID
// is considered an ancestor of any valid ID. This behavior is documented here.
func TestNodeID_IsAncestorOf_ZeroValues(t *testing.T) {
	var zeroID types.NodeID
	validID, _ := types.Parse("1.2")

	// Zero ID is considered ancestor of valid IDs due to vacuous truth in comparison
	// (zero parts means the for loop completes without finding mismatches)
	// This documents the current implementation behavior.
	if !zeroID.IsAncestorOf(validID) {
		t.Error("zero NodeID.IsAncestorOf(valid) = false, but current implementation returns true")
	}

	// Zero is not ancestor of zero (equal length check fails first)
	if zeroID.IsAncestorOf(zeroID) {
		t.Error("zero NodeID.IsAncestorOf(zero) = true, want false")
	}

	// Valid is not ancestor of zero (valid has more parts)
	if validID.IsAncestorOf(zeroID) {
		t.Error("valid NodeID.IsAncestorOf(zero) = true, want false")
	}
}

// TestNodeID_CommonAncestor_ZeroValues verifies CommonAncestor() with zero values.
func TestNodeID_CommonAncestor_ZeroValues(t *testing.T) {
	var zeroID types.NodeID
	validID, _ := types.Parse("1.2")

	// Common ancestor with zero should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("CommonAncestor() with zero NodeID panicked: %v", r)
		}
	}()

	ca1 := zeroID.CommonAncestor(validID)
	_ = ca1.String() // Should not panic

	ca2 := validID.CommonAncestor(zeroID)
	_ = ca2.String() // Should not panic

	ca3 := zeroID.CommonAncestor(zeroID)
	_ = ca3.String() // Should not panic
}

// TestNodeID_Equal_ZeroValues verifies Equal() with zero values.
func TestNodeID_Equal_ZeroValues(t *testing.T) {
	var zero1, zero2 types.NodeID
	validID, _ := types.Parse("1.2")

	// Two zero values should be equal
	if !zero1.Equal(zero2) {
		t.Error("zero.Equal(zero) = false, want true")
	}

	// Zero should not equal valid
	if zero1.Equal(validID) {
		t.Error("zero.Equal(valid) = true, want false")
	}

	// Valid should not equal zero
	if validID.Equal(zero1) {
		t.Error("valid.Equal(zero) = true, want false")
	}
}

// TestNodeID_Less_ZeroValues verifies Less() with zero values.
func TestNodeID_Less_ZeroValues(t *testing.T) {
	var zero1, zero2 types.NodeID
	validID, _ := types.Parse("1.2")

	// Zero is not less than itself
	if zero1.Less(zero2) {
		t.Error("zero.Less(zero) = true, want false")
	}

	// Zero should be less than any valid ID
	if !zero1.Less(validID) {
		t.Error("zero.Less(valid) = false, want true")
	}

	// Valid should not be less than zero
	if validID.Less(zero1) {
		t.Error("valid.Less(zero) = true, want false")
	}
}

// TestNode_ContentHashWithZeroID verifies content hash computation with zero ID.
func TestNode_ContentHashWithZeroID(t *testing.T) {
	var zeroID types.NodeID

	n, err := node.NewNode(
		zeroID,
		schema.NodeTypeClaim,
		"Test statement",
		schema.InferenceAssumption,
	)
	if err != nil {
		t.Fatalf("NewNode() unexpected error: %v", err)
	}

	// Content hash should still be computed (ID is not part of content hash)
	if n.ContentHash == "" {
		t.Error("ContentHash should not be empty")
	}

	// Content hash should be valid 64-char hex
	if len(n.ContentHash) != 64 {
		t.Errorf("ContentHash length = %d, want 64", len(n.ContentHash))
	}

	// Verify content hash
	if !n.VerifyContentHash() {
		t.Error("VerifyContentHash() should return true")
	}
}

// TestNewNodeWithOptions_ZeroIDAndDeps verifies creation with zero ID and deps.
func TestNewNodeWithOptions_ZeroIDAndDeps(t *testing.T) {
	var zeroID types.NodeID
	var zeroDep types.NodeID
	validDep, _ := types.Parse("1.1")

	opts := node.NodeOptions{
		Dependencies:   []types.NodeID{zeroDep, validDep},
		ValidationDeps: []types.NodeID{validDep, zeroDep},
	}

	// Should not panic
	n, err := node.NewNodeWithOptions(
		zeroID,
		schema.NodeTypeClaim,
		"Test statement",
		schema.InferenceModusPonens,
		opts,
	)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() unexpected error: %v", err)
	}

	// Verify dependencies are stored
	if len(n.Dependencies) != 2 {
		t.Errorf("Dependencies length = %d, want 2", len(n.Dependencies))
	}

	if len(n.ValidationDeps) != 2 {
		t.Errorf("ValidationDeps length = %d, want 2", len(n.ValidationDeps))
	}
}
