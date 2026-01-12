//go:build integration
// +build integration

// These tests define expected behavior for WritePendingDef, ReadPendingDef,
// ListPendingDefs, and DeletePendingDef.
// Run with: go test -tags=integration ./internal/fs/...

package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/types"
)

// TestWritePendingDef verifies that WritePendingDef correctly writes a
// pending definition request to the pending_defs/ subdirectory as a JSON file.
func TestWritePendingDef(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	nodeID, err := types.Parse("1.2")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	pd := node.NewPendingDef("continuity", nodeID)

	err = WritePendingDef(dir, nodeID, pd)
	if err != nil {
		t.Fatalf("WritePendingDef failed: %v", err)
	}

	// Verify file was created with correct name
	defPath := filepath.Join(pendingDefsDir, nodeID.String()+".json")
	info, err := os.Stat(defPath)
	if os.IsNotExist(err) {
		t.Fatalf("expected pending def file to exist at %s", defPath)
	}
	if err != nil {
		t.Fatalf("error checking pending def file: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected pending def file to be a file, not a directory")
	}

	// Verify file contents are valid JSON matching the pending def
	content, err := os.ReadFile(defPath)
	if err != nil {
		t.Fatalf("failed to read pending def file: %v", err)
	}

	var readPD node.PendingDef
	if err := json.Unmarshal(content, &readPD); err != nil {
		t.Fatalf("pending def file is not valid JSON: %v", err)
	}

	if readPD.ID != pd.ID {
		t.Errorf("ID mismatch: got %q, want %q", readPD.ID, pd.ID)
	}
	if readPD.Term != pd.Term {
		t.Errorf("Term mismatch: got %q, want %q", readPD.Term, pd.Term)
	}
	if readPD.RequestedBy.String() != pd.RequestedBy.String() {
		t.Errorf("RequestedBy mismatch: got %q, want %q", readPD.RequestedBy.String(), pd.RequestedBy.String())
	}
	if readPD.Status != pd.Status {
		t.Errorf("Status mismatch: got %q, want %q", readPD.Status, pd.Status)
	}
}

// TestWritePendingDef_CreatesPendingDefsDir verifies that WritePendingDef creates
// the .af/pending_defs/ subdirectory if it doesn't exist.
func TestWritePendingDef_CreatesPendingDefsDir(t *testing.T) {
	dir := t.TempDir()
	// Note: .af/pending_defs/ directory does NOT exist yet

	nodeID, err := types.Parse("1.3")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	pd := node.NewPendingDef("limit", nodeID)

	err = WritePendingDef(dir, nodeID, pd)
	if err != nil {
		t.Fatalf("WritePendingDef failed: %v", err)
	}

	// Verify pending_defs directory was created
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	info, err := os.Stat(pendingDefsDir)
	if os.IsNotExist(err) {
		t.Fatal("expected pending_defs directory to be created")
	}
	if err != nil {
		t.Fatalf("error checking pending_defs directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected pending_defs to be a directory")
	}

	// Verify file was created
	defPath := filepath.Join(pendingDefsDir, nodeID.String()+".json")
	if _, err := os.Stat(defPath); os.IsNotExist(err) {
		t.Fatalf("expected pending def file to exist at %s", defPath)
	}
}

// TestWritePendingDef_NilPendingDef verifies that WritePendingDef returns
// an error when given a nil pending def.
func TestWritePendingDef_NilPendingDef(t *testing.T) {
	dir := t.TempDir()

	nodeID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	err = WritePendingDef(dir, nodeID, nil)
	if err == nil {
		t.Error("expected error for nil pending def, got nil")
	}
}

// TestWritePendingDef_EmptyNodeID verifies that WritePendingDef returns
// an error when given an empty node ID.
func TestWritePendingDef_EmptyNodeID(t *testing.T) {
	dir := t.TempDir()

	nodeID, _ := types.Parse("1.1")
	pd := node.NewPendingDef("term", nodeID)

	err := WritePendingDef(dir, types.NodeID{}, pd)
	if err == nil {
		t.Error("expected error for empty node ID, got nil")
	}
}

// TestReadPendingDef verifies that ReadPendingDef correctly reads a
// pending definition from the .af/pending_defs/ subdirectory.
func TestReadPendingDef(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	nodeID, err := types.Parse("1.2.3")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create a pending def and write it manually
	pd := node.NewPendingDef("derivative", nodeID)

	pdJSON, err := json.Marshal(pd)
	if err != nil {
		t.Fatalf("failed to marshal pending def: %v", err)
	}

	defPath := filepath.Join(pendingDefsDir, nodeID.String()+".json")
	if err := os.WriteFile(defPath, pdJSON, 0644); err != nil {
		t.Fatalf("failed to write pending def file: %v", err)
	}

	// Read it back using ReadPendingDef
	readPD, err := ReadPendingDef(dir, nodeID)
	if err != nil {
		t.Fatalf("ReadPendingDef failed: %v", err)
	}

	if readPD.ID != pd.ID {
		t.Errorf("ID mismatch: got %q, want %q", readPD.ID, pd.ID)
	}
	if readPD.Term != pd.Term {
		t.Errorf("Term mismatch: got %q, want %q", readPD.Term, pd.Term)
	}
	if readPD.RequestedBy.String() != pd.RequestedBy.String() {
		t.Errorf("RequestedBy mismatch: got %q, want %q", readPD.RequestedBy.String(), pd.RequestedBy.String())
	}
	if readPD.Status != pd.Status {
		t.Errorf("Status mismatch: got %q, want %q", readPD.Status, pd.Status)
	}
}

// TestReadPendingDef_NotFound verifies that ReadPendingDef returns an
// appropriate error when the pending def doesn't exist.
func TestReadPendingDef_NotFound(t *testing.T) {
	t.Run("file_not_found", func(t *testing.T) {
		dir := t.TempDir()
		pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
		if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
			t.Fatalf("failed to create pending_defs directory: %v", err)
		}

		nodeID, _ := types.Parse("1.99")
		_, err := ReadPendingDef(dir, nodeID)
		if err == nil {
			t.Error("expected error for nonexistent pending def, got nil")
		}
		if !os.IsNotExist(err) {
			// Accept either os.IsNotExist or a wrapped error
			// Just verify we got an error
			t.Logf("got error (expected): %v", err)
		}
	})

	t.Run("pending_defs_dir_not_found", func(t *testing.T) {
		dir := t.TempDir()
		// Note: .af/pending_defs/ directory does NOT exist

		nodeID, _ := types.Parse("1.1")
		_, err := ReadPendingDef(dir, nodeID)
		if err == nil {
			t.Error("expected error when pending_defs directory doesn't exist, got nil")
		}
	})
}

// TestReadPendingDef_EmptyNodeID verifies that ReadPendingDef returns an
// error for an empty node ID.
func TestReadPendingDef_EmptyNodeID(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	_, err := ReadPendingDef(dir, types.NodeID{})
	if err == nil {
		t.Error("expected error for empty node ID, got nil")
	}
}

// TestReadPendingDef_InvalidJSON verifies that ReadPendingDef returns an
// error when the file contains invalid JSON.
func TestReadPendingDef_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	nodeID, _ := types.Parse("1.5")

	// Write invalid JSON
	defPath := filepath.Join(pendingDefsDir, nodeID.String()+".json")
	if err := os.WriteFile(defPath, []byte("not valid json{"), 0644); err != nil {
		t.Fatalf("failed to write invalid pending def file: %v", err)
	}

	_, err := ReadPendingDef(dir, nodeID)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestWritePendingDef_InvalidPath verifies that WritePendingDef returns
// an error for invalid paths.
func TestWritePendingDef_InvalidPath(t *testing.T) {
	nodeID, _ := types.Parse("1.1")
	pd := node.NewPendingDef("test", nodeID)

	t.Run("empty_path", func(t *testing.T) {
		err := WritePendingDef("", nodeID, pd)
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only_path", func(t *testing.T) {
		err := WritePendingDef("   ", nodeID, pd)
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		err := WritePendingDef("path\x00with\x00nulls", nodeID, pd)
		if err == nil {
			t.Error("expected error for path with null bytes, got nil")
		}
	})
}

// TestWritePendingDef_PermissionDenied verifies that WritePendingDef handles
// permission errors gracefully.
func TestWritePendingDef_PermissionDenied(t *testing.T) {
	// Skip on Windows where permission model differs
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	// Skip if running as root (root can write anywhere)
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()
	afDir := filepath.Join(dir, ".af")
	pendingDefsDir := filepath.Join(afDir, "pending_defs")

	// Create .af directory with write permission first
	if err := os.MkdirAll(afDir, 0755); err != nil {
		t.Fatalf("failed to create .af directory: %v", err)
	}
	// Create pending_defs directory with no write permission
	if err := os.Mkdir(pendingDefsDir, 0555); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(pendingDefsDir, 0755)
	})

	nodeID, _ := types.Parse("1.1")
	pd := node.NewPendingDef("test", nodeID)

	err := WritePendingDef(dir, nodeID, pd)
	if err == nil {
		t.Error("expected error when writing to read-only directory, got nil")
	}
}

// TestWritePendingDef_Overwrite verifies the behavior when overwriting
// an existing pending def file.
func TestWritePendingDef_Overwrite(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	nodeID, _ := types.Parse("1.4")

	// Create initial pending def
	pd := node.NewPendingDef("integral", nodeID)

	// Write it first time
	err := WritePendingDef(dir, nodeID, pd)
	if err != nil {
		t.Fatalf("first WritePendingDef failed: %v", err)
	}

	// Create a new pending def with same nodeID but different term
	pd2 := node.NewPendingDef("riemann_integral", nodeID)

	// Write it again - should overwrite
	err = WritePendingDef(dir, nodeID, pd2)
	if err != nil {
		t.Fatalf("second WritePendingDef (overwrite) failed: %v", err)
	}

	// Read it back and verify the updated content
	readPD, err := ReadPendingDef(dir, nodeID)
	if err != nil {
		t.Fatalf("ReadPendingDef after overwrite failed: %v", err)
	}

	if readPD.Term != pd2.Term {
		t.Errorf("Term not updated: got %q, want %q", readPD.Term, pd2.Term)
	}
	if readPD.ID != pd2.ID {
		t.Errorf("ID not updated: got %q, want %q", readPD.ID, pd2.ID)
	}
}

// TestListPendingDefs verifies that ListPendingDefs returns all pending def
// node IDs in the .af/pending_defs/ directory.
func TestListPendingDefs(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	// Create several pending defs
	nodeID1, _ := types.Parse("1.1")
	nodeID2, _ := types.Parse("1.2")
	nodeID3, _ := types.Parse("1.3.1")

	pd1 := node.NewPendingDef("term1", nodeID1)
	pd2 := node.NewPendingDef("term2", nodeID2)
	pd3 := node.NewPendingDef("term3", nodeID3)

	for _, pair := range []struct {
		nodeID types.NodeID
		pd     *node.PendingDef
	}{{nodeID1, pd1}, {nodeID2, pd2}, {nodeID3, pd3}} {
		if err := WritePendingDef(dir, pair.nodeID, pair.pd); err != nil {
			t.Fatalf("failed to write pending def %s: %v", pair.nodeID.String(), err)
		}
	}

	// List all pending defs
	nodeIDs, err := ListPendingDefs(dir)
	if err != nil {
		t.Fatalf("ListPendingDefs failed: %v", err)
	}

	if len(nodeIDs) != 3 {
		t.Errorf("expected 3 pending defs, got %d", len(nodeIDs))
	}

	// Sort both slices for comparison
	expectedIDs := []string{nodeID1.String(), nodeID2.String(), nodeID3.String()}
	actualIDs := make([]string, len(nodeIDs))
	for i, id := range nodeIDs {
		actualIDs[i] = id.String()
	}
	sort.Strings(actualIDs)
	sort.Strings(expectedIDs)

	for i, id := range expectedIDs {
		if i >= len(actualIDs) || actualIDs[i] != id {
			t.Errorf("missing or mismatched ID: expected %q", id)
		}
	}
}

// TestListPendingDefs_Empty verifies that ListPendingDefs returns an empty
// slice when there are no pending defs.
func TestListPendingDefs_Empty(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	nodeIDs, err := ListPendingDefs(dir)
	if err != nil {
		t.Fatalf("ListPendingDefs failed: %v", err)
	}

	if nodeIDs == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(nodeIDs) != 0 {
		t.Errorf("expected 0 pending defs, got %d", len(nodeIDs))
	}
}

// TestListPendingDefs_NoPendingDefsDir verifies that ListPendingDefs returns
// an empty slice (not an error) when the .af/pending_defs/ directory doesn't exist.
func TestListPendingDefs_NoPendingDefsDir(t *testing.T) {
	dir := t.TempDir()
	// Note: .af/pending_defs/ directory does NOT exist

	nodeIDs, err := ListPendingDefs(dir)
	if err != nil {
		t.Errorf("expected no error when pending_defs directory doesn't exist, got %v", err)
	}
	if nodeIDs == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(nodeIDs) != 0 {
		t.Errorf("expected 0 pending defs, got %d", len(nodeIDs))
	}
}

// TestListPendingDefs_IgnoresNonJSONFiles verifies that ListPendingDefs
// only returns IDs for .json files.
func TestListPendingDefs_IgnoresNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	// Create a valid pending def
	nodeID, _ := types.Parse("1.1")
	pd := node.NewPendingDef("valid", nodeID)
	if err := WritePendingDef(dir, nodeID, pd); err != nil {
		t.Fatalf("failed to write pending def: %v", err)
	}

	// Create non-JSON files that should be ignored
	nonJSONFiles := []string{
		"readme.txt",
		"notes.md",
		"backup.json.bak",
		".hidden.json",
	}
	for _, name := range nonJSONFiles {
		path := filepath.Join(pendingDefsDir, name)
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to write non-JSON file %s: %v", name, err)
		}
	}

	// Create a subdirectory (should be ignored)
	subdir := filepath.Join(pendingDefsDir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	nodeIDs, err := ListPendingDefs(dir)
	if err != nil {
		t.Fatalf("ListPendingDefs failed: %v", err)
	}

	if len(nodeIDs) != 1 {
		t.Errorf("expected 1 pending def, got %d: %v", len(nodeIDs), nodeIDs)
	}
	if len(nodeIDs) > 0 && nodeIDs[0].String() != nodeID.String() {
		t.Errorf("expected ID %q, got %q", nodeID.String(), nodeIDs[0].String())
	}
}

// TestDeletePendingDef verifies that DeletePendingDef removes a pending def
// file from the .af/pending_defs/ directory.
func TestDeletePendingDef(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	// Create a pending def
	nodeID, _ := types.Parse("1.5")
	pd := node.NewPendingDef("to-delete", nodeID)
	if err := WritePendingDef(dir, nodeID, pd); err != nil {
		t.Fatalf("failed to write pending def: %v", err)
	}

	// Verify it exists
	defPath := filepath.Join(pendingDefsDir, nodeID.String()+".json")
	if _, err := os.Stat(defPath); os.IsNotExist(err) {
		t.Fatal("pending def file should exist before delete")
	}

	// Delete it
	err := DeletePendingDef(dir, nodeID)
	if err != nil {
		t.Fatalf("DeletePendingDef failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(defPath); !os.IsNotExist(err) {
		t.Error("expected pending def file to be deleted")
	}
}

// TestDeletePendingDef_NotFound verifies that DeletePendingDef does NOT return
// an error when the pending def doesn't exist (idempotent delete).
func TestDeletePendingDef_NotFound(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	nodeID, _ := types.Parse("1.99")
	err := DeletePendingDef(dir, nodeID)
	if err != nil {
		t.Errorf("expected no error for nonexistent pending def, got %v", err)
	}
}

// TestDeletePendingDef_EmptyNodeID verifies that DeletePendingDef returns an
// error for an empty node ID.
func TestDeletePendingDef_EmptyNodeID(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	err := DeletePendingDef(dir, types.NodeID{})
	if err == nil {
		t.Error("expected error for empty node ID, got nil")
	}
}

// TestDeletePendingDef_DoesNotAffectOthers verifies that DeletePendingDef
// only removes the specified pending def.
func TestDeletePendingDef_DoesNotAffectOthers(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	// Create multiple pending defs
	nodeID1, _ := types.Parse("1.1")
	nodeID2, _ := types.Parse("1.2")
	nodeID3, _ := types.Parse("1.3")

	pd1 := node.NewPendingDef("keep1", nodeID1)
	pd2 := node.NewPendingDef("delete-me", nodeID2)
	pd3 := node.NewPendingDef("keep2", nodeID3)

	for _, pair := range []struct {
		nodeID types.NodeID
		pd     *node.PendingDef
	}{{nodeID1, pd1}, {nodeID2, pd2}, {nodeID3, pd3}} {
		if err := WritePendingDef(dir, pair.nodeID, pair.pd); err != nil {
			t.Fatalf("failed to write pending def %s: %v", pair.nodeID.String(), err)
		}
	}

	// Delete only nodeID2
	if err := DeletePendingDef(dir, nodeID2); err != nil {
		t.Fatalf("DeletePendingDef failed: %v", err)
	}

	// Verify nodeID1 and nodeID3 still exist
	for _, nodeID := range []types.NodeID{nodeID1, nodeID3} {
		_, err := ReadPendingDef(dir, nodeID)
		if err != nil {
			t.Errorf("pending def %s should still exist: %v", nodeID.String(), err)
		}
	}

	// Verify nodeID2 is gone
	_, err := ReadPendingDef(dir, nodeID2)
	if err == nil {
		t.Error("deleted pending def should not be readable")
	}
}

// TestWritePendingDef_AtomicWrite verifies that WritePendingDef uses
// atomic write operations (write to temp, then rename).
func TestWritePendingDef_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	nodeID, _ := types.Parse("1.6")
	pd := node.NewPendingDef("atomic-test", nodeID)

	// Write the pending def
	err := WritePendingDef(dir, nodeID, pd)
	if err != nil {
		t.Fatalf("WritePendingDef failed: %v", err)
	}

	// Verify no temp files are left behind
	entries, err := os.ReadDir(pendingDefsDir)
	if err != nil {
		t.Fatalf("failed to read pending_defs directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// Check for common temp file patterns
		if name != nodeID.String()+".json" {
			// Allow only the expected .json file
			if filepath.Ext(name) == ".tmp" || filepath.Ext(name) == ".temp" {
				t.Errorf("temp file left behind: %s", name)
			}
		}
	}
}

// TestPendingDefRoundTrip verifies that a pending def can be written and read back
// with all fields preserved.
func TestPendingDefRoundTrip(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	nodeID, err := types.Parse("1.7.2")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	original := node.NewPendingDef("injective", nodeID)

	// Write
	if err := WritePendingDef(dir, nodeID, original); err != nil {
		t.Fatalf("WritePendingDef failed: %v", err)
	}

	// Read
	retrieved, err := ReadPendingDef(dir, nodeID)
	if err != nil {
		t.Fatalf("ReadPendingDef failed: %v", err)
	}

	// Compare all fields
	if retrieved.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, original.ID)
	}
	if retrieved.Term != original.Term {
		t.Errorf("Term mismatch: got %q, want %q", retrieved.Term, original.Term)
	}
	if retrieved.RequestedBy.String() != original.RequestedBy.String() {
		t.Errorf("RequestedBy mismatch: got %q, want %q", retrieved.RequestedBy.String(), original.RequestedBy.String())
	}
	if retrieved.Status != original.Status {
		t.Errorf("Status mismatch: got %q, want %q", retrieved.Status, original.Status)
	}
	if !retrieved.Created.Equal(original.Created) {
		t.Errorf("Created mismatch: got %v, want %v", retrieved.Created, original.Created)
	}
	if retrieved.ResolvedBy != original.ResolvedBy {
		t.Errorf("ResolvedBy mismatch: got %q, want %q", retrieved.ResolvedBy, original.ResolvedBy)
	}
}

// TestPendingDefFileFormat verifies that pending def files use the expected
// JSON format with proper indentation for human readability.
func TestPendingDefFileFormat(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	nodeID, _ := types.Parse("1.8")
	pd := node.NewPendingDef("format-test", nodeID)

	if err := WritePendingDef(dir, nodeID, pd); err != nil {
		t.Fatalf("WritePendingDef failed: %v", err)
	}

	// Read raw file content
	defPath := filepath.Join(pendingDefsDir, nodeID.String()+".json")
	content, err := os.ReadFile(defPath)
	if err != nil {
		t.Fatalf("failed to read pending def file: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("pending def file is not valid JSON: %v", err)
	}

	// Verify expected fields are present
	expectedFields := []string{"id", "term", "requested_by", "created", "status"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("expected field %q not found in pending def file", field)
		}
	}
}

// TestReadPendingDef_InvalidPath verifies that ReadPendingDef returns
// an error for invalid paths.
func TestReadPendingDef_InvalidPath(t *testing.T) {
	nodeID, _ := types.Parse("1.1")

	t.Run("empty_path", func(t *testing.T) {
		_, err := ReadPendingDef("", nodeID)
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only_path", func(t *testing.T) {
		_, err := ReadPendingDef("   ", nodeID)
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		_, err := ReadPendingDef("path\x00with\x00nulls", nodeID)
		if err == nil {
			t.Error("expected error for path with null bytes, got nil")
		}
	})
}

// TestDeletePendingDef_InvalidPath verifies that DeletePendingDef returns
// an error for invalid paths.
func TestDeletePendingDef_InvalidPath(t *testing.T) {
	nodeID, _ := types.Parse("1.1")

	t.Run("empty_path", func(t *testing.T) {
		err := DeletePendingDef("", nodeID)
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only_path", func(t *testing.T) {
		err := DeletePendingDef("   ", nodeID)
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})
}

// TestListPendingDefs_InvalidPath verifies that ListPendingDefs returns
// an error for invalid paths.
func TestListPendingDefs_InvalidPath(t *testing.T) {
	t.Run("empty_path", func(t *testing.T) {
		_, err := ListPendingDefs("")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only_path", func(t *testing.T) {
		_, err := ListPendingDefs("   ")
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})
}

// TestPendingDefWithResolvedStatus verifies that pending defs with resolved
// status are correctly written and read back.
func TestPendingDefWithResolvedStatus(t *testing.T) {
	dir := t.TempDir()
	pendingDefsDir := filepath.Join(dir, ".af", "pending_defs")
	if err := os.MkdirAll(pendingDefsDir, 0755); err != nil {
		t.Fatalf("failed to create pending_defs directory: %v", err)
	}

	nodeID, _ := types.Parse("1.9")
	pd := node.NewPendingDef("resolved-term", nodeID)

	// Resolve the pending def
	if err := pd.Resolve("def-123"); err != nil {
		t.Fatalf("failed to resolve pending def: %v", err)
	}

	// Write
	if err := WritePendingDef(dir, nodeID, pd); err != nil {
		t.Fatalf("WritePendingDef failed: %v", err)
	}

	// Read back
	retrieved, err := ReadPendingDef(dir, nodeID)
	if err != nil {
		t.Fatalf("ReadPendingDef failed: %v", err)
	}

	if retrieved.Status != node.PendingDefStatusResolved {
		t.Errorf("Status mismatch: got %q, want %q", retrieved.Status, node.PendingDefStatusResolved)
	}
	if retrieved.ResolvedBy != "def-123" {
		t.Errorf("ResolvedBy mismatch: got %q, want %q", retrieved.ResolvedBy, "def-123")
	}
}
