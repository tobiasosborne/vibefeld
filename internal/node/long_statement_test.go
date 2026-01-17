// Package node_test contains edge case tests for very long statements.
package node_test

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestNode_VeryLongStatement tests creating a node with a very long statement (>1MB).
// This tests memory/performance impact of handling large proof statements.
func TestNode_VeryLongStatement(t *testing.T) {
	id, _ := types.Parse("1")

	// Create a 1MB statement (1,048,576 bytes)
	statementSize := 1024 * 1024
	longStatement := strings.Repeat("A", statementSize)

	n, err := node.NewNode(id, schema.NodeTypeClaim, longStatement, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() with 1MB statement error: %v", err)
	}

	// Verify statement is stored correctly
	if len(n.Statement) != statementSize {
		t.Errorf("Statement length = %d, want %d", len(n.Statement), statementSize)
	}

	// Verify content hash is computed (should be 64 hex chars for SHA256)
	if len(n.ContentHash) != 64 {
		t.Errorf("ContentHash length = %d, want 64", len(n.ContentHash))
	}

	// Verify the hash can be verified
	if !n.VerifyContentHash() {
		t.Error("VerifyContentHash() should return true for unmodified node")
	}
}

// TestNode_VeryLongStatement_5MB tests creating a node with a 5MB statement.
func TestNode_VeryLongStatement_5MB(t *testing.T) {
	id, _ := types.Parse("1.1")

	// Create a 5MB statement
	statementSize := 5 * 1024 * 1024
	longStatement := strings.Repeat("X", statementSize)

	n, err := node.NewNode(id, schema.NodeTypeClaim, longStatement, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() with 5MB statement error: %v", err)
	}

	if len(n.Statement) != statementSize {
		t.Errorf("Statement length = %d, want %d", len(n.Statement), statementSize)
	}
}

// TestNode_VeryLongStatement_ContentHashDeterministic tests that content hash
// is deterministic for very long statements.
func TestNode_VeryLongStatement_ContentHashDeterministic(t *testing.T) {
	id, _ := types.Parse("1")

	// Create a 1MB statement
	longStatement := strings.Repeat("B", 1024*1024)

	n1, err := node.NewNode(id, schema.NodeTypeClaim, longStatement, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() first call error: %v", err)
	}

	n2, err := node.NewNode(id, schema.NodeTypeClaim, longStatement, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() second call error: %v", err)
	}

	if n1.ContentHash != n2.ContentHash {
		t.Errorf("Content hashes differ for identical 1MB statements: %q != %q", n1.ContentHash, n2.ContentHash)
	}
}

// TestNode_VeryLongStatement_JSON_Roundtrip tests JSON serialization with large statements.
func TestNode_VeryLongStatement_JSON_Roundtrip(t *testing.T) {
	id, _ := types.Parse("1.2")

	// Create a 1MB statement
	statementSize := 1024 * 1024
	longStatement := strings.Repeat("C", statementSize)

	original, err := node.NewNode(id, schema.NodeTypeClaim, longStatement, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	// Verify JSON is larger than the statement (includes other fields)
	if len(data) < statementSize {
		t.Errorf("JSON data size = %d, expected >= %d", len(data), statementSize)
	}

	// Unmarshal back
	var restored node.Node
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	// Verify statement is preserved
	if len(restored.Statement) != statementSize {
		t.Errorf("Restored statement length = %d, want %d", len(restored.Statement), statementSize)
	}

	if restored.Statement != original.Statement {
		t.Error("Restored statement does not match original")
	}

	// Verify content hash is preserved
	if restored.ContentHash != original.ContentHash {
		t.Errorf("ContentHash mismatch: %q != %q", restored.ContentHash, original.ContentHash)
	}
}

// TestNode_VeryLongStatement_DifferentContent tests that different long statements
// produce different content hashes.
func TestNode_VeryLongStatement_DifferentContent(t *testing.T) {
	id, _ := types.Parse("1")

	// Create two 1MB statements that differ only in the last character
	baseStatement := strings.Repeat("D", 1024*1024-1)
	statement1 := baseStatement + "X"
	statement2 := baseStatement + "Y"

	n1, err := node.NewNode(id, schema.NodeTypeClaim, statement1, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() first statement error: %v", err)
	}

	n2, err := node.NewNode(id, schema.NodeTypeClaim, statement2, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() second statement error: %v", err)
	}

	if n1.ContentHash == n2.ContentHash {
		t.Error("Content hashes should differ for statements with different final character")
	}
}

// TestNode_VeryLongStatement_WithLatex tests large statement combined with large latex.
func TestNode_VeryLongStatement_WithLatex(t *testing.T) {
	id, _ := types.Parse("1")

	// Create a 500KB statement and 500KB latex
	halfMB := 512 * 1024
	longStatement := strings.Repeat("E", halfMB)
	longLatex := strings.Repeat("\\alpha ", halfMB/7) // ~500KB of latex

	opts := node.NodeOptions{
		Latex: longLatex,
	}

	n, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, longStatement, schema.InferenceAssumption, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}

	if len(n.Statement) != halfMB {
		t.Errorf("Statement length = %d, want %d", len(n.Statement), halfMB)
	}

	if len(n.Latex) < halfMB/2 {
		t.Errorf("Latex length = %d, expected at least %d", len(n.Latex), halfMB/2)
	}

	if !n.VerifyContentHash() {
		t.Error("VerifyContentHash() should return true")
	}
}

// TestNode_VeryLongStatement_Validate tests that validation works on large statements.
func TestNode_VeryLongStatement_Validate(t *testing.T) {
	id, _ := types.Parse("1")

	longStatement := strings.Repeat("F", 1024*1024)

	n, err := node.NewNode(id, schema.NodeTypeClaim, longStatement, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	if err := n.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

// TestNode_VeryLongStatement_MultipleNodes tests creating multiple nodes with large statements.
func TestNode_VeryLongStatement_MultipleNodes(t *testing.T) {
	statementSize := 1024 * 1024 // 1MB each
	nodeCount := 5

	nodes := make([]*node.Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		idStr := "1"
		if i > 0 {
			idStr = "1." + string(rune('0'+i))
		}
		id, _ := types.Parse(idStr)

		// Each node gets a unique statement
		statement := strings.Repeat(string(rune('A'+i)), statementSize)

		var err error
		nodes[i], err = node.NewNode(id, schema.NodeTypeClaim, statement, schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("NewNode() for node %d error: %v", i, err)
		}
	}

	// Verify all nodes have unique content hashes
	hashes := make(map[string]int)
	for i, n := range nodes {
		if prev, exists := hashes[n.ContentHash]; exists {
			t.Errorf("Node %d has same hash as node %d", i, prev)
		}
		hashes[n.ContentHash] = i
	}

	// Verify all statements are intact
	for i, n := range nodes {
		if len(n.Statement) != statementSize {
			t.Errorf("Node %d statement length = %d, want %d", i, len(n.Statement), statementSize)
		}
	}
}

// TestNode_VeryLongStatement_Memory tests that memory is properly managed with large statements.
func TestNode_VeryLongStatement_Memory(t *testing.T) {
	// Force GC before measurement
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	id, _ := types.Parse("1")
	statementSize := 2 * 1024 * 1024 // 2MB
	longStatement := strings.Repeat("G", statementSize)

	n, err := node.NewNode(id, schema.NodeTypeClaim, longStatement, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	// Force GC after creation
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// The memory increase should be reasonable (roughly 2x the statement size
	// for the statement itself plus the hash string representation)
	// We're mainly checking that we don't have runaway memory usage
	memIncrease := m2.Alloc - m1.Alloc

	// Allow up to 10x the statement size for various allocations
	maxExpected := uint64(statementSize * 10)
	if memIncrease > maxExpected {
		t.Errorf("Memory increase = %d bytes, expected less than %d", memIncrease, maxExpected)
	}

	// Keep node referenced to prevent premature GC
	_ = n.Statement
}

// TestNode_VeryLongStatement_RepeatedHashComputation tests that repeated hash
// computation is consistent for large statements.
func TestNode_VeryLongStatement_RepeatedHashComputation(t *testing.T) {
	id, _ := types.Parse("1")
	longStatement := strings.Repeat("H", 1024*1024)

	n, err := node.NewNode(id, schema.NodeTypeClaim, longStatement, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	originalHash := n.ContentHash

	// Compute hash multiple times
	for i := 0; i < 10; i++ {
		computed := n.ComputeContentHash()
		if computed != originalHash {
			t.Errorf("Iteration %d: computed hash %q != original %q", i, computed, originalHash)
		}
	}
}

// TestNode_VeryLongStatement_Unicode tests large statements with unicode characters.
func TestNode_VeryLongStatement_Unicode(t *testing.T) {
	id, _ := types.Parse("1")

	// Create ~1MB of unicode content (each rune is 3 bytes in UTF-8)
	runeCount := 350000 // ~1MB when encoded
	longStatement := strings.Repeat("∀∃∈", runeCount/3)

	n, err := node.NewNode(id, schema.NodeTypeClaim, longStatement, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	// Verify statement byte length is approximately 1MB
	if len(n.Statement) < 900000 {
		t.Errorf("Statement byte length = %d, expected ~1MB", len(n.Statement))
	}

	if !n.VerifyContentHash() {
		t.Error("VerifyContentHash() should return true for unicode statement")
	}

	// Verify JSON roundtrip preserves unicode
	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var restored node.Node
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if restored.Statement != n.Statement {
		t.Error("Unicode statement not preserved after JSON roundtrip")
	}
}
