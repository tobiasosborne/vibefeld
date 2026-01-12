//go:build integration
// +build integration

// These tests define expected behavior for WriteNode, ReadNode,
// ListNodes, and DeleteNode.
// Run with: go test -tags=integration ./internal/fs/...

package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	aferrors "github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestWriteNode verifies that WriteNode correctly writes a
// node to the nodes/ subdirectory as a JSON file.
func TestWriteNode(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	err = WriteNode(dir, n)
	if err != nil {
		t.Fatalf("WriteNode failed: %v", err)
	}

	// Verify file was created with correct name (dots replaced with underscores)
	nodePath := filepath.Join(nodesDir, "1.json")
	info, err := os.Stat(nodePath)
	if os.IsNotExist(err) {
		t.Fatalf("expected node file to exist at %s", nodePath)
	}
	if err != nil {
		t.Fatalf("error checking node file: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected node file to be a file, not a directory")
	}

	// Verify file contents are valid JSON matching the node
	content, err := os.ReadFile(nodePath)
	if err != nil {
		t.Fatalf("failed to read node file: %v", err)
	}

	var readNode node.Node
	if err := json.Unmarshal(content, &readNode); err != nil {
		t.Fatalf("node file is not valid JSON: %v", err)
	}

	if readNode.ID.String() != n.ID.String() {
		t.Errorf("ID mismatch: got %q, want %q", readNode.ID.String(), n.ID.String())
	}
	if readNode.Statement != n.Statement {
		t.Errorf("Statement mismatch: got %q, want %q", readNode.Statement, n.Statement)
	}
	if readNode.ContentHash != n.ContentHash {
		t.Errorf("ContentHash mismatch: got %q, want %q", readNode.ContentHash, n.ContentHash)
	}
}

// TestWriteNode_HierarchicalID verifies that WriteNode correctly handles
// hierarchical node IDs with dots.
func TestWriteNode_HierarchicalID(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	nodeID, err := types.Parse("1.2.3")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Nested claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	err = WriteNode(dir, n)
	if err != nil {
		t.Fatalf("WriteNode failed: %v", err)
	}

	// Verify file was created with underscores replacing dots
	nodePath := filepath.Join(nodesDir, "1_2_3.json")
	if _, err := os.Stat(nodePath); os.IsNotExist(err) {
		t.Fatalf("expected node file to exist at %s", nodePath)
	}
}

// TestWriteNode_CreatesNodesDir verifies that WriteNode creates
// the nodes/ subdirectory if it doesn't exist.
func TestWriteNode_CreatesNodesDir(t *testing.T) {
	dir := t.TempDir()
	// Note: nodes/ directory does NOT exist yet

	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	err = WriteNode(dir, n)
	if err != nil {
		t.Fatalf("WriteNode failed: %v", err)
	}

	// Verify nodes directory was created
	nodesDir := filepath.Join(dir, "nodes")
	info, err := os.Stat(nodesDir)
	if os.IsNotExist(err) {
		t.Fatal("expected nodes directory to be created")
	}
	if err != nil {
		t.Fatalf("error checking nodes directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected nodes to be a directory")
	}

	// Verify file was created
	nodePath := filepath.Join(nodesDir, "1.json")
	if _, err := os.Stat(nodePath); os.IsNotExist(err) {
		t.Fatalf("expected node file to exist at %s", nodePath)
	}
}

// TestWriteNode_NilNode verifies that WriteNode returns
// an error when given a nil node.
func TestWriteNode_NilNode(t *testing.T) {
	dir := t.TempDir()

	err := WriteNode(dir, nil)
	if err == nil {
		t.Error("expected error for nil node, got nil")
	}
}

// TestReadNode verifies that ReadNode correctly reads a
// node from the nodes/ subdirectory.
func TestReadNode(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	nodeID, err := types.Parse("1.2")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create a node and write it using WriteNode
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim for reading", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if err := WriteNode(dir, n); err != nil {
		t.Fatalf("failed to write node: %v", err)
	}

	// Read it back using ReadNode
	readNode, err := ReadNode(dir, nodeID)
	if err != nil {
		t.Fatalf("ReadNode failed: %v", err)
	}

	if readNode.ID.String() != n.ID.String() {
		t.Errorf("ID mismatch: got %q, want %q", readNode.ID.String(), n.ID.String())
	}
	if readNode.Statement != n.Statement {
		t.Errorf("Statement mismatch: got %q, want %q", readNode.Statement, n.Statement)
	}
	if readNode.ContentHash != n.ContentHash {
		t.Errorf("ContentHash mismatch: got %q, want %q", readNode.ContentHash, n.ContentHash)
	}
	if readNode.Type != n.Type {
		t.Errorf("Type mismatch: got %q, want %q", readNode.Type, n.Type)
	}
	if readNode.Inference != n.Inference {
		t.Errorf("Inference mismatch: got %q, want %q", readNode.Inference, n.Inference)
	}
}

// TestReadNode_NotFound verifies that ReadNode returns an
// appropriate error when the node doesn't exist.
func TestReadNode_NotFound(t *testing.T) {
	t.Run("file_not_found", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		nodeID, _ := types.Parse("1.99")
		_, err := ReadNode(dir, nodeID)
		if err == nil {
			t.Error("expected error for nonexistent node, got nil")
		}
		if !os.IsNotExist(err) {
			// Accept either os.IsNotExist or a wrapped error
			// Just verify we got an error
			t.Logf("got error (expected): %v", err)
		}
	})

	t.Run("nodes_dir_not_found", func(t *testing.T) {
		dir := t.TempDir()
		// Note: nodes/ directory does NOT exist

		nodeID, _ := types.Parse("1")
		_, err := ReadNode(dir, nodeID)
		if err == nil {
			t.Error("expected error when nodes directory doesn't exist, got nil")
		}
	})
}

// TestReadNode_EmptyID verifies that ReadNode returns an
// error for an empty node ID.
func TestReadNode_EmptyID(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	_, err := ReadNode(dir, types.NodeID{})
	if err == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

// TestReadNode_InvalidJSON verifies that ReadNode returns an
// error when the file contains invalid JSON.
func TestReadNode_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	// Write invalid JSON
	nodePath := filepath.Join(nodesDir, "1.json")
	if err := os.WriteFile(nodePath, []byte("not valid json{"), 0644); err != nil {
		t.Fatalf("failed to write invalid node file: %v", err)
	}

	nodeID, _ := types.Parse("1")
	_, err := ReadNode(dir, nodeID)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestReadNode_ContentHashMismatch verifies that ReadNode returns
// a CONTENT_HASH_MISMATCH error when the stored hash doesn't match.
func TestReadNode_ContentHashMismatch(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create a valid node
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Corrupt the content hash
	n.ContentHash = "corrupted-hash-value"

	// Write it manually (bypassing WriteNode which doesn't validate hash)
	data, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal node: %v", err)
	}
	nodePath := filepath.Join(nodesDir, "1.json")
	if err := os.WriteFile(nodePath, data, 0644); err != nil {
		t.Fatalf("failed to write node file: %v", err)
	}

	// Read should fail with hash mismatch
	_, err = ReadNode(dir, nodeID)
	if err == nil {
		t.Fatal("expected error for content hash mismatch, got nil")
	}

	// Verify it's the correct error type
	if aferrors.Code(err) != aferrors.CONTENT_HASH_MISMATCH {
		t.Errorf("expected CONTENT_HASH_MISMATCH error, got: %v", err)
	}
}

// TestWriteNode_InvalidPath verifies that WriteNode returns
// an error for invalid paths.
func TestWriteNode_InvalidPath(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "test", schema.InferenceAssumption)

	t.Run("empty_path", func(t *testing.T) {
		err := WriteNode("", n)
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only_path", func(t *testing.T) {
		err := WriteNode("   ", n)
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		err := WriteNode("path\x00with\x00nulls", n)
		if err == nil {
			t.Error("expected error for path with null bytes, got nil")
		}
	})
}

// TestWriteNode_PermissionDenied verifies that WriteNode handles
// permission errors gracefully.
func TestWriteNode_PermissionDenied(t *testing.T) {
	// Skip on Windows where permission model differs
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	// Skip if running as root (root can write anywhere)
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")

	// Create nodes directory with no write permission
	if err := os.MkdirAll(nodesDir, 0555); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(nodesDir, 0755)
	})

	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "test", schema.InferenceAssumption)

	err := WriteNode(dir, n)
	if err == nil {
		t.Error("expected error when writing to read-only directory, got nil")
	}
}

// TestWriteNode_Overwrite verifies the behavior when overwriting
// an existing node file.
func TestWriteNode_Overwrite(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	nodeID, _ := types.Parse("1")

	// Create initial node
	n1, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Original statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Write it first time
	err = WriteNode(dir, n1)
	if err != nil {
		t.Fatalf("first WriteNode failed: %v", err)
	}

	// Create modified node (same ID, different content)
	n2, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Updated statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create modified node: %v", err)
	}

	// Write it again - should overwrite
	err = WriteNode(dir, n2)
	if err != nil {
		t.Fatalf("second WriteNode (overwrite) failed: %v", err)
	}

	// Read it back and verify the updated content
	readNode, err := ReadNode(dir, nodeID)
	if err != nil {
		t.Fatalf("ReadNode after overwrite failed: %v", err)
	}

	if readNode.Statement != n2.Statement {
		t.Errorf("Statement not updated: got %q, want %q", readNode.Statement, n2.Statement)
	}
}

// TestListNodes verifies that ListNodes returns all node
// IDs in the nodes/ directory.
func TestListNodes(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	// Create several nodes with different hierarchical IDs
	nodeIDs := []string{"1", "1.1", "1.2", "1.1.1"}
	for _, idStr := range nodeIDs {
		id, _ := types.Parse(idStr)
		n, _ := node.NewNode(id, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceAssumption)
		if err := WriteNode(dir, n); err != nil {
			t.Fatalf("failed to write node %s: %v", idStr, err)
		}
	}

	// List all nodes
	ids, err := ListNodes(dir)
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if len(ids) != len(nodeIDs) {
		t.Errorf("expected %d nodes, got %d", len(nodeIDs), len(ids))
	}

	// Sort both slices for comparison
	gotStrings := make([]string, len(ids))
	for i, id := range ids {
		gotStrings[i] = id.String()
	}
	sort.Strings(gotStrings)
	sort.Strings(nodeIDs)

	for i, expected := range nodeIDs {
		if i >= len(gotStrings) || gotStrings[i] != expected {
			t.Errorf("missing or mismatched ID: expected %q", expected)
		}
	}
}

// TestListNodes_Empty verifies that ListNodes returns an empty
// slice when there are no nodes.
func TestListNodes_Empty(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	ids, err := ListNodes(dir)
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if ids == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(ids))
	}
}

// TestListNodes_NoNodesDir verifies that ListNodes returns an
// error when the nodes/ directory doesn't exist.
func TestListNodes_NoNodesDir(t *testing.T) {
	dir := t.TempDir()
	// Note: nodes/ directory does NOT exist

	_, err := ListNodes(dir)
	if err == nil {
		t.Error("expected error when nodes directory doesn't exist, got nil")
	}
}

// TestListNodes_IgnoresNonJSONFiles verifies that ListNodes
// only returns IDs for .json files.
func TestListNodes_IgnoresNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	// Create a valid node
	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "valid", schema.InferenceAssumption)
	if err := WriteNode(dir, n); err != nil {
		t.Fatalf("failed to write node: %v", err)
	}

	// Create non-JSON files that should be ignored
	nonJSONFiles := []string{
		"readme.txt",
		"notes.md",
		"backup.json.bak",
		".hidden.json",
	}
	for _, name := range nonJSONFiles {
		path := filepath.Join(nodesDir, name)
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to write non-JSON file %s: %v", name, err)
		}
	}

	// Create a subdirectory (should be ignored)
	subdir := filepath.Join(nodesDir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	ids, err := ListNodes(dir)
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if len(ids) != 1 {
		t.Errorf("expected 1 node, got %d: %v", len(ids), ids)
	}
	if len(ids) > 0 && ids[0].String() != nodeID.String() {
		t.Errorf("expected ID %q, got %q", nodeID.String(), ids[0].String())
	}
}

// TestListNodes_IgnoresInvalidFilenames verifies that ListNodes
// skips files with invalid node ID formats.
func TestListNodes_IgnoresInvalidFilenames(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	// Create a valid node
	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "valid", schema.InferenceAssumption)
	if err := WriteNode(dir, n); err != nil {
		t.Fatalf("failed to write node: %v", err)
	}

	// Create files with invalid node ID formats
	invalidFiles := []string{
		"invalid.json",       // Not a valid node ID
		"0.json",             // Root must be 1
		"1_0_2.json",         // Zero is invalid
		"abc_def.json",       // Non-numeric
		"2.json",             // Root must be 1
		"negative_-1.json",   // Negative invalid
	}
	for _, name := range invalidFiles {
		path := filepath.Join(nodesDir, name)
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatalf("failed to write file %s: %v", name, err)
		}
	}

	ids, err := ListNodes(dir)
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if len(ids) != 1 {
		t.Errorf("expected 1 valid node, got %d: %v", len(ids), ids)
	}
}

// TestDeleteNode verifies that DeleteNode removes a node
// file from the nodes/ directory.
func TestDeleteNode(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	// Create a node
	nodeID, _ := types.Parse("1.2")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "to-delete", schema.InferenceAssumption)
	if err := WriteNode(dir, n); err != nil {
		t.Fatalf("failed to write node: %v", err)
	}

	// Verify it exists
	nodePath := filepath.Join(nodesDir, "1_2.json")
	if _, err := os.Stat(nodePath); os.IsNotExist(err) {
		t.Fatal("node file should exist before delete")
	}

	// Delete it
	err := DeleteNode(dir, nodeID)
	if err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(nodePath); !os.IsNotExist(err) {
		t.Error("expected node file to be deleted")
	}
}

// TestDeleteNode_NotFound verifies that DeleteNode returns an
// error when the node doesn't exist.
func TestDeleteNode_NotFound(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	nodeID, _ := types.Parse("1.99")
	err := DeleteNode(dir, nodeID)
	if err == nil {
		t.Error("expected error for nonexistent node, got nil")
	}
}

// TestDeleteNode_EmptyID verifies that DeleteNode returns an
// error for an empty node ID.
func TestDeleteNode_EmptyID(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	err := DeleteNode(dir, types.NodeID{})
	if err == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

// TestDeleteNode_DoesNotAffectOthers verifies that DeleteNode
// only removes the specified node.
func TestDeleteNode_DoesNotAffectOthers(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	// Create multiple nodes
	nodeIDs := []string{"1", "1.1", "1.2"}
	for _, idStr := range nodeIDs {
		id, _ := types.Parse(idStr)
		n, _ := node.NewNode(id, schema.NodeTypeClaim, "node "+idStr, schema.InferenceAssumption)
		if err := WriteNode(dir, n); err != nil {
			t.Fatalf("failed to write node %s: %v", idStr, err)
		}
	}

	// Delete only 1.1
	deleteID, _ := types.Parse("1.1")
	if err := DeleteNode(dir, deleteID); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}

	// Verify 1 and 1.2 still exist
	for _, idStr := range []string{"1", "1.2"} {
		id, _ := types.Parse(idStr)
		_, err := ReadNode(dir, id)
		if err != nil {
			t.Errorf("node %s should still exist: %v", idStr, err)
		}
	}

	// Verify 1.1 is gone
	_, err := ReadNode(dir, deleteID)
	if err == nil {
		t.Error("deleted node should not be readable")
	}
}

// TestWriteNode_AtomicWrite verifies that WriteNode uses
// atomic write operations (write to temp, then rename).
func TestWriteNode_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "atomic-test", schema.InferenceAssumption)

	// Write the node
	err := WriteNode(dir, n)
	if err != nil {
		t.Fatalf("WriteNode failed: %v", err)
	}

	// Verify no temp files are left behind
	entries, err := os.ReadDir(nodesDir)
	if err != nil {
		t.Fatalf("failed to read nodes directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// Check for common temp file patterns
		if name != "1.json" {
			// Allow only the expected .json file
			if filepath.Ext(name) == ".tmp" || filepath.Ext(name) == ".temp" {
				t.Errorf("temp file left behind: %s", name)
			}
		}
	}
}

// TestNodeRoundTrip verifies that a node can be written and read back
// with all fields preserved.
func TestNodeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	nodeID, _ := types.Parse("1.3.2")

	original, err := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"A complex mathematical statement with dependencies",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Latex:        "f(x) = \\int_0^x g(t) dt",
			Context:      []string{"def:continuity", "def:integral"},
			Dependencies: []types.NodeID{},
			Scope:        []string{"assume:x>0"},
		},
	)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Write
	if err := WriteNode(dir, original); err != nil {
		t.Fatalf("WriteNode failed: %v", err)
	}

	// Read
	retrieved, err := ReadNode(dir, original.ID)
	if err != nil {
		t.Fatalf("ReadNode failed: %v", err)
	}

	// Compare all fields
	if retrieved.ID.String() != original.ID.String() {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID.String(), original.ID.String())
	}
	if retrieved.Type != original.Type {
		t.Errorf("Type mismatch: got %q, want %q", retrieved.Type, original.Type)
	}
	if retrieved.Statement != original.Statement {
		t.Errorf("Statement mismatch: got %q, want %q", retrieved.Statement, original.Statement)
	}
	if retrieved.Latex != original.Latex {
		t.Errorf("Latex mismatch: got %q, want %q", retrieved.Latex, original.Latex)
	}
	if retrieved.Inference != original.Inference {
		t.Errorf("Inference mismatch: got %q, want %q", retrieved.Inference, original.Inference)
	}
	if retrieved.ContentHash != original.ContentHash {
		t.Errorf("ContentHash mismatch: got %q, want %q", retrieved.ContentHash, original.ContentHash)
	}
	if retrieved.WorkflowState != original.WorkflowState {
		t.Errorf("WorkflowState mismatch: got %q, want %q", retrieved.WorkflowState, original.WorkflowState)
	}
	if retrieved.EpistemicState != original.EpistemicState {
		t.Errorf("EpistemicState mismatch: got %q, want %q", retrieved.EpistemicState, original.EpistemicState)
	}
	if retrieved.TaintState != original.TaintState {
		t.Errorf("TaintState mismatch: got %q, want %q", retrieved.TaintState, original.TaintState)
	}

	// Compare contexts
	if len(retrieved.Context) != len(original.Context) {
		t.Errorf("Context length mismatch: got %d, want %d", len(retrieved.Context), len(original.Context))
	} else {
		for i, ctx := range original.Context {
			if retrieved.Context[i] != ctx {
				t.Errorf("Context[%d] mismatch: got %q, want %q", i, retrieved.Context[i], ctx)
			}
		}
	}

	// Compare scope
	if len(retrieved.Scope) != len(original.Scope) {
		t.Errorf("Scope length mismatch: got %d, want %d", len(retrieved.Scope), len(original.Scope))
	} else {
		for i, s := range original.Scope {
			if retrieved.Scope[i] != s {
				t.Errorf("Scope[%d] mismatch: got %q, want %q", i, retrieved.Scope[i], s)
			}
		}
	}
}

// TestNodeFileFormat verifies that node files use the expected
// JSON format with proper indentation for human readability.
func TestNodeFileFormat(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "format-test", schema.InferenceAssumption)

	if err := WriteNode(dir, n); err != nil {
		t.Fatalf("WriteNode failed: %v", err)
	}

	// Read raw file content
	nodePath := filepath.Join(nodesDir, "1.json")
	content, err := os.ReadFile(nodePath)
	if err != nil {
		t.Fatalf("failed to read node file: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("node file is not valid JSON: %v", err)
	}

	// Verify expected fields are present
	expectedFields := []string{"id", "type", "statement", "inference", "workflow_state", "epistemic_state", "taint_state", "content_hash", "created"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("expected field %q not found in node file", field)
		}
	}
}

// TestNodeIO_RoundTripVariants verifies round-trip for various node configurations.
func TestNodeIO_RoundTripVariants(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	rootID, _ := types.Parse("1")
	childID, _ := types.Parse("1.1")
	deepID, _ := types.Parse("1.2.3.4.5")

	testCases := []struct {
		name  string
		setup func() (*node.Node, error)
	}{
		{
			name: "root_claim",
			setup: func() (*node.Node, error) {
				return node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
			},
		},
		{
			name: "child_with_latex",
			setup: func() (*node.Node, error) {
				return node.NewNodeWithOptions(childID, schema.NodeTypeClaim, "With latex", schema.InferenceAssumption,
					node.NodeOptions{Latex: "x^2 + y^2 = z^2"})
			},
		},
		{
			name: "deep_nested_node",
			setup: func() (*node.Node, error) {
				return node.NewNode(deepID, schema.NodeTypeClaim, "Deep nested", schema.InferenceModusPonens)
			},
		},
		{
			name: "node_with_context",
			setup: func() (*node.Node, error) {
				return node.NewNodeWithOptions(rootID, schema.NodeTypeClaim, "With context", schema.InferenceAssumption,
					node.NodeOptions{Context: []string{"def:test1", "def:test2"}})
			},
		},
		{
			name: "node_with_scope",
			setup: func() (*node.Node, error) {
				return node.NewNodeWithOptions(rootID, schema.NodeTypeLocalAssume, "With scope", schema.InferenceLocalAssume,
					node.NodeOptions{Scope: []string{"assume:x>0", "assume:y<10"}})
			},
		},
		{
			name: "special_characters",
			setup: func() (*node.Node, error) {
				return node.NewNode(rootID, schema.NodeTypeClaim, "Statement with \"quotes\" and 'apostrophes' and newlines\nand tabs\t", schema.InferenceAssumption)
			},
		},
		{
			name: "unicode_statement",
			setup: func() (*node.Node, error) {
				return node.NewNode(rootID, schema.NodeTypeClaim, "Statement with unicode: forall x in R, x^2 >= 0", schema.InferenceAssumption)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := tc.setup()
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			// Write
			if err := WriteNode(dir, n); err != nil {
				t.Fatalf("WriteNode failed: %v", err)
			}

			// Read
			readNode, err := ReadNode(dir, n.ID)
			if err != nil {
				t.Fatalf("ReadNode failed: %v", err)
			}

			// Compare key fields
			if readNode.ID.String() != n.ID.String() {
				t.Errorf("ID mismatch: got %q, want %q", readNode.ID.String(), n.ID.String())
			}
			if readNode.Statement != n.Statement {
				t.Errorf("Statement mismatch: got %q, want %q", readNode.Statement, n.Statement)
			}
			if readNode.ContentHash != n.ContentHash {
				t.Errorf("ContentHash mismatch: got %q, want %q", readNode.ContentHash, n.ContentHash)
			}
			if readNode.Type != n.Type {
				t.Errorf("Type mismatch: got %q, want %q", readNode.Type, n.Type)
			}
			if readNode.Inference != n.Inference {
				t.Errorf("Inference mismatch: got %q, want %q", readNode.Inference, n.Inference)
			}
		})
	}
}

// TestDeleteNode_PermissionDenied verifies that DeleteNode handles
// permission errors gracefully.
func TestDeleteNode_PermissionDenied(t *testing.T) {
	// Skip on Windows where permission model differs
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	// Skip if running as root
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	// Create a node
	nodeID, _ := types.Parse("1")
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Cannot delete", schema.InferenceAssumption)
	if err := WriteNode(dir, n); err != nil {
		t.Fatalf("WriteNode failed: %v", err)
	}

	// Make directory read-only
	if err := os.Chmod(nodesDir, 0555); err != nil {
		t.Fatalf("failed to change permissions: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(nodesDir, 0755)
	})

	err := DeleteNode(dir, nodeID)
	if err == nil {
		t.Error("expected error when directory is read-only, got nil")
	}
}

// TestDeleteNode_EmptyPath verifies that DeleteNode returns an
// error for an empty path.
func TestDeleteNode_EmptyPath(t *testing.T) {
	nodeID, _ := types.Parse("1")
	err := DeleteNode("", nodeID)
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}

// TestListNodes_EmptyPath verifies that ListNodes returns an
// error for an empty path.
func TestListNodes_EmptyPath(t *testing.T) {
	_, err := ListNodes("")
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}

// TestReadNode_EmptyPath verifies that ReadNode returns an
// error for an empty path.
func TestReadNode_EmptyPath(t *testing.T) {
	nodeID, _ := types.Parse("1")
	_, err := ReadNode("", nodeID)
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}

// TestNodeIDToFilename verifies the filename conversion.
func TestNodeIDToFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1", "1.json"},
		{"1.1", "1_1.json"},
		{"1.2.3", "1_2_3.json"},
		{"1.10.100", "1_10_100.json"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			id, err := types.Parse(tc.input)
			if err != nil {
				t.Fatalf("failed to parse ID: %v", err)
			}
			result := nodeIDToFilename(id)
			if result != tc.expected {
				t.Errorf("nodeIDToFilename(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
