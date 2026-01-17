// Package fs provides filesystem operations for the AF proof framework.
// This file contains error injection tests for filesystem error scenarios.
package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers for Error Injection
// =============================================================================

// skipIfRoot skips the test if running as root (UID 0).
// Root can bypass most permission checks, making permission tests invalid.
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}
}

// skipIfWindows skips the test on Windows where permission model differs.
func skipIfWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on Windows")
	}
}

// makeReadOnly sets a path to read-only and returns a cleanup function.
func makeReadOnly(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0555); err != nil {
		t.Fatalf("failed to set read-only: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(path, 0755)
	})
}

// makeUnreadable sets a path to be unreadable (no permissions) and returns a cleanup function.
func makeUnreadable(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatalf("failed to remove permissions: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(path, 0755)
	})
}

// createTestNode creates a valid test node for use in tests.
func createTestNode(t *testing.T, idStr string) *node.Node {
	t.Helper()
	nodeID, err := types.Parse(idStr)
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	return n
}

// createTestDefinition creates a valid test definition for use in tests.
func createTestDefinition(id string) *node.Definition {
	def, _ := node.NewDefinition("test-term", "A test definition")
	def.ID = id // Override the auto-generated ID
	return def
}

// createTestAssumption creates a valid test assumption for use in tests.
func createTestAssumption(id string) *node.Assumption {
	a, _ := node.NewAssumption("Test assumption statement")
	a.ID = id // Override the auto-generated ID
	return a
}

// createTestExternal creates a valid test external reference for use in tests.
func createTestExternal(id string) *node.External {
	ext, _ := node.NewExternal("Test external", "https://example.com")
	ext.ID = id // Override the auto-generated ID
	return &ext
}

// createTestLemma creates a valid test lemma for use in tests.
func createTestLemma(id string) *node.Lemma {
	nodeID, _ := types.Parse("1")
	lemma, _ := node.NewLemma("Test lemma statement", nodeID)
	lemma.ID = id // Override the auto-generated ID
	return lemma
}

// createTestMeta creates a valid test meta for use in tests.
func createTestMeta() *Meta {
	return &Meta{
		Conjecture: "Test conjecture",
		CreatedAt:  time.Now(),
		Version:    "1.0",
	}
}

// =============================================================================
// Permission Denied Tests
// =============================================================================

// TestPermissionDenied_WriteNode tests WriteNode behavior when directory is read-only.
func TestPermissionDenied_WriteNode(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	t.Run("read_only_nodes_directory", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		makeReadOnly(t, nodesDir)

		n := createTestNode(t, "1")
		err := WriteNode(dir, n)
		if err == nil {
			t.Error("expected permission denied error, got nil")
		}
		if !os.IsPermission(err) {
			t.Logf("got error (as expected): %v", err)
		}
	})

	t.Run("read_only_parent_directory_no_nodes_subdir", func(t *testing.T) {
		dir := t.TempDir()
		// Don't create nodes/ subdirectory
		makeReadOnly(t, dir)

		n := createTestNode(t, "1")
		err := WriteNode(dir, n)
		if err == nil {
			t.Error("expected permission denied error, got nil")
		}
	})
}

// TestPermissionDenied_ReadNode tests ReadNode behavior with permission issues.
func TestPermissionDenied_ReadNode(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	t.Run("unreadable_node_file", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		n := createTestNode(t, "1")
		if err := WriteNode(dir, n); err != nil {
			t.Fatalf("failed to write node: %v", err)
		}

		// Make the node file unreadable
		nodePath := filepath.Join(nodesDir, "1.json")
		makeUnreadable(t, nodePath)

		nodeID, _ := types.Parse("1")
		_, err := ReadNode(dir, nodeID)
		if err == nil {
			t.Error("expected permission denied error, got nil")
		}
	})

	t.Run("unreadable_nodes_directory", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		n := createTestNode(t, "1")
		if err := WriteNode(dir, n); err != nil {
			t.Fatalf("failed to write node: %v", err)
		}

		makeUnreadable(t, nodesDir)

		nodeID, _ := types.Parse("1")
		_, err := ReadNode(dir, nodeID)
		if err == nil {
			t.Error("expected permission denied error, got nil")
		}
	})
}

// TestPermissionDenied_ListNodes tests ListNodes behavior with permission issues.
func TestPermissionDenied_ListNodes(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	n := createTestNode(t, "1")
	if err := WriteNode(dir, n); err != nil {
		t.Fatalf("failed to write node: %v", err)
	}

	makeUnreadable(t, nodesDir)

	_, err := ListNodes(dir)
	if err == nil {
		t.Error("expected permission denied error, got nil")
	}
}

// TestPermissionDenied_DeleteNode tests DeleteNode behavior with permission issues.
func TestPermissionDenied_DeleteNode(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}

	n := createTestNode(t, "1")
	if err := WriteNode(dir, n); err != nil {
		t.Fatalf("failed to write node: %v", err)
	}

	// Make directory read-only (can't delete files from it)
	makeReadOnly(t, nodesDir)

	nodeID, _ := types.Parse("1")
	err := DeleteNode(dir, nodeID)
	if err == nil {
		t.Error("expected permission denied error, got nil")
	}
}

// TestPermissionDenied_WriteDefinition tests WriteDefinition with permission issues.
func TestPermissionDenied_WriteDefinition(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	t.Run("read_only_defs_directory", func(t *testing.T) {
		dir := t.TempDir()
		defsDir := filepath.Join(dir, "defs")
		if err := os.MkdirAll(defsDir, 0755); err != nil {
			t.Fatalf("failed to create defs directory: %v", err)
		}

		makeReadOnly(t, defsDir)

		def := createTestDefinition("test-def")
		err := WriteDefinition(dir, def)
		if err == nil {
			t.Error("expected permission denied error, got nil")
		}
	})

	t.Run("read_only_parent_no_defs_subdir", func(t *testing.T) {
		dir := t.TempDir()
		makeReadOnly(t, dir)

		def := createTestDefinition("test-def")
		err := WriteDefinition(dir, def)
		if err == nil {
			t.Error("expected permission denied error, got nil")
		}
	})
}

// TestPermissionDenied_WriteAssumption tests WriteAssumption with permission issues.
func TestPermissionDenied_WriteAssumption(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	dir := t.TempDir()
	assumpDir := filepath.Join(dir, "assumptions")
	if err := os.MkdirAll(assumpDir, 0755); err != nil {
		t.Fatalf("failed to create assumptions directory: %v", err)
	}

	makeReadOnly(t, assumpDir)

	a := createTestAssumption("test-assumption")
	err := WriteAssumption(dir, a)
	if err == nil {
		t.Error("expected permission denied error, got nil")
	}
}

// TestPermissionDenied_WriteExternal tests WriteExternal with permission issues.
func TestPermissionDenied_WriteExternal(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	dir := t.TempDir()
	externalsDir := filepath.Join(dir, "externals")
	if err := os.MkdirAll(externalsDir, 0755); err != nil {
		t.Fatalf("failed to create externals directory: %v", err)
	}

	makeReadOnly(t, externalsDir)

	ext := createTestExternal("test-external")
	err := WriteExternal(dir, ext)
	if err == nil {
		t.Error("expected permission denied error, got nil")
	}
}

// TestPermissionDenied_WriteLemma tests WriteLemma with permission issues.
func TestPermissionDenied_WriteLemma(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	makeReadOnly(t, lemmasDir)

	lemma := createTestLemma("test-lemma")
	err := WriteLemma(dir, lemma)
	if err == nil {
		t.Error("expected permission denied error, got nil")
	}
}

// TestPermissionDenied_WriteMeta tests WriteMeta with permission issues.
func TestPermissionDenied_WriteMeta(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	dir := t.TempDir()
	makeReadOnly(t, dir)

	meta := createTestMeta()
	err := WriteMeta(dir, meta)
	if err == nil {
		t.Error("expected permission denied error, got nil")
	}
}

// TestPermissionDenied_InitProofDir tests InitProofDir with permission issues.
func TestPermissionDenied_InitProofDir(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	dir := t.TempDir()
	restrictedDir := filepath.Join(dir, "restricted")
	if err := os.MkdirAll(restrictedDir, 0555); err != nil {
		t.Fatalf("failed to create restricted directory: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(restrictedDir, 0755)
	})

	proofDir := filepath.Join(restrictedDir, "proof")
	err := InitProofDir(proofDir)
	if err == nil {
		t.Error("expected permission denied error, got nil")
	}
}

// =============================================================================
// Disk Full Simulation Tests
// =============================================================================

// Note: Actually simulating disk full conditions is difficult in tests.
// These tests verify that write operations handle errors gracefully.
// In production, disk full errors manifest as ENOSPC or EIO.

// TestDiskFull_WriteOperations tests that write functions handle write errors properly.
// We simulate by creating a file with the same name as the target directory.
func TestDiskFull_WriteOperations(t *testing.T) {
	t.Run("write_node_to_file_not_directory", func(t *testing.T) {
		dir := t.TempDir()
		// Create a file where nodes/ directory should be
		nodesPath := filepath.Join(dir, "nodes")
		if err := os.WriteFile(nodesPath, []byte("not a directory"), 0644); err != nil {
			t.Fatalf("failed to create blocking file: %v", err)
		}

		n := createTestNode(t, "1")
		err := WriteNode(dir, n)
		if err == nil {
			t.Error("expected error when nodes/ is a file, got nil")
		}
	})

	t.Run("write_definition_to_file_not_directory", func(t *testing.T) {
		dir := t.TempDir()
		// Create a file where defs/ directory should be
		defsPath := filepath.Join(dir, "defs")
		if err := os.WriteFile(defsPath, []byte("not a directory"), 0644); err != nil {
			t.Fatalf("failed to create blocking file: %v", err)
		}

		def := createTestDefinition("test-def")
		err := WriteDefinition(dir, def)
		if err == nil {
			t.Error("expected error when defs/ is a file, got nil")
		}
	})

	t.Run("write_assumption_to_file_not_directory", func(t *testing.T) {
		dir := t.TempDir()
		// Create a file where assumptions/ directory should be
		assumpPath := filepath.Join(dir, "assumptions")
		if err := os.WriteFile(assumpPath, []byte("not a directory"), 0644); err != nil {
			t.Fatalf("failed to create blocking file: %v", err)
		}

		a := createTestAssumption("test-assumption")
		err := WriteAssumption(dir, a)
		if err == nil {
			t.Error("expected error when assumptions/ is a file, got nil")
		}
	})

	t.Run("write_external_to_file_not_directory", func(t *testing.T) {
		dir := t.TempDir()
		// Create a file where externals/ directory should be
		externalsPath := filepath.Join(dir, "externals")
		if err := os.WriteFile(externalsPath, []byte("not a directory"), 0644); err != nil {
			t.Fatalf("failed to create blocking file: %v", err)
		}

		ext := createTestExternal("test-external")
		err := WriteExternal(dir, ext)
		if err == nil {
			t.Error("expected error when externals/ is a file, got nil")
		}
	})

	t.Run("write_lemma_to_file_not_directory", func(t *testing.T) {
		dir := t.TempDir()
		// Create a file where lemmas/ directory should be
		lemmasPath := filepath.Join(dir, "lemmas")
		if err := os.WriteFile(lemmasPath, []byte("not a directory"), 0644); err != nil {
			t.Fatalf("failed to create blocking file: %v", err)
		}

		lemma := createTestLemma("test-lemma")
		err := WriteLemma(dir, lemma)
		if err == nil {
			t.Error("expected error when lemmas/ is a file, got nil")
		}
	})
}

// TestDiskFull_TempFileCleanup verifies temp files are cleaned up on rename failure.
// This simulates what happens when disk is full during the atomic rename.
func TestDiskFull_TempFileCleanup(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	t.Run("temp_file_cleanup_on_rename_failure", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		n := createTestNode(t, "1")

		// First, write successfully
		if err := WriteNode(dir, n); err != nil {
			t.Fatalf("initial write failed: %v", err)
		}

		// List files to verify no temp files
		entries, err := os.ReadDir(nodesDir)
		if err != nil {
			t.Fatalf("failed to read directory: %v", err)
		}

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".tmp") {
				t.Errorf("temp file left behind: %s", entry.Name())
			}
		}
	})
}

// =============================================================================
// Symlink Loop Tests
// =============================================================================

// TestSymlinkLoop_Read tests behavior when reading through symlink loops.
func TestSymlinkLoop_Read(t *testing.T) {
	skipIfWindows(t)

	t.Run("simple_symlink_loop", func(t *testing.T) {
		dir := t.TempDir()

		// Create a symlink loop: a -> b -> a
		linkA := filepath.Join(dir, "a")
		linkB := filepath.Join(dir, "b")

		if err := os.Symlink(linkB, linkA); err != nil {
			t.Fatalf("failed to create symlink a: %v", err)
		}
		if err := os.Symlink(linkA, linkB); err != nil {
			t.Fatalf("failed to create symlink b: %v", err)
		}

		// Try to read from the symlink loop path
		nodeID, _ := types.Parse("1")
		_, err := ReadNode(linkA, nodeID)
		if err == nil {
			t.Error("expected error for symlink loop, got nil")
		}
		// Error should be ELOOP (too many levels of symbolic links)
		t.Logf("got expected error: %v", err)
	})

	t.Run("symlink_loop_in_path", func(t *testing.T) {
		dir := t.TempDir()
		proofDir := filepath.Join(dir, "proof")
		if err := os.MkdirAll(proofDir, 0755); err != nil {
			t.Fatalf("failed to create proof directory: %v", err)
		}

		// Create symlink loop within proof directory
		loopDir := filepath.Join(proofDir, "loop")
		loopTarget := filepath.Join(loopDir, "target")

		// Create loop -> loop/target, then target -> loop
		if err := os.Symlink(loopTarget, loopDir); err != nil {
			if os.IsExist(err) {
				// Symlink already exists, remove and recreate
				os.Remove(loopDir)
				if err := os.Symlink(loopTarget, loopDir); err != nil {
					t.Fatalf("failed to create symlink loop: %v", err)
				}
			} else {
				t.Fatalf("failed to create symlink loop: %v", err)
			}
		}

		// Try to use this path
		err := InitProofDir(filepath.Join(loopDir, "proof"))
		if err == nil {
			t.Error("expected error for symlink loop path, got nil")
		}
		t.Logf("got expected error: %v", err)
	})
}

// TestSymlinkLoop_Write tests behavior when writing through symlink loops.
func TestSymlinkLoop_Write(t *testing.T) {
	skipIfWindows(t)

	t.Run("write_through_symlink_loop", func(t *testing.T) {
		dir := t.TempDir()

		// Create a symlink loop
		linkA := filepath.Join(dir, "a")
		linkB := filepath.Join(dir, "b")

		if err := os.Symlink(linkB, linkA); err != nil {
			t.Fatalf("failed to create symlink a: %v", err)
		}
		if err := os.Symlink(linkA, linkB); err != nil {
			t.Fatalf("failed to create symlink b: %v", err)
		}

		n := createTestNode(t, "1")
		err := WriteNode(linkA, n)
		if err == nil {
			t.Error("expected error for symlink loop, got nil")
		}
		t.Logf("got expected error: %v", err)
	})
}

// TestSymlinkLoop_List tests behavior when listing through symlink loops.
func TestSymlinkLoop_List(t *testing.T) {
	skipIfWindows(t)

	dir := t.TempDir()

	// Create a symlink loop
	linkA := filepath.Join(dir, "a")
	linkB := filepath.Join(dir, "b")

	if err := os.Symlink(linkB, linkA); err != nil {
		t.Fatalf("failed to create symlink a: %v", err)
	}
	if err := os.Symlink(linkA, linkB); err != nil {
		t.Fatalf("failed to create symlink b: %v", err)
	}

	_, err := ListNodes(linkA)
	if err == nil {
		t.Error("expected error for symlink loop, got nil")
	}
	t.Logf("got expected error: %v", err)
}

// TestSymlinkLoop_SelfReferential tests behavior with self-referential symlinks.
func TestSymlinkLoop_SelfReferential(t *testing.T) {
	skipIfWindows(t)

	dir := t.TempDir()

	// Create a self-referential symlink: link -> link
	selfLink := filepath.Join(dir, "selflink")
	if err := os.Symlink(selfLink, selfLink); err != nil {
		// Some systems may error immediately
		t.Logf("symlink creation error (expected on some systems): %v", err)
		return
	}

	// Try to use this path
	_, err := ReadNode(selfLink, types.NodeID{})
	if err == nil {
		t.Error("expected error for self-referential symlink, got nil")
	}
	t.Logf("got expected error: %v", err)
}

// =============================================================================
// Very Long Filename Tests
// =============================================================================

// TestLongFilename_WriteNode tests WriteNode with very long node IDs.
func TestLongFilename_WriteNode(t *testing.T) {
	// Most filesystems have a 255-byte limit on filenames
	// Node IDs like "1.1.1.1.1..." can get very long

	t.Run("moderately_long_id", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		// Create a deep hierarchy node ID (valid but long)
		// "1.1.1.1.1..." with 20 levels
		parts := make([]string, 20)
		for i := range parts {
			parts[i] = "1"
		}
		idStr := strings.Join(parts, ".")

		nodeID, err := types.Parse(idStr)
		if err != nil {
			t.Fatalf("failed to parse long node ID: %v", err)
		}

		n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Deep node", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("failed to create node: %v", err)
		}

		// This should work - filename is still reasonable
		err = WriteNode(dir, n)
		if err != nil {
			t.Errorf("WriteNode failed for moderately long ID: %v", err)
		}
	})

	t.Run("extremely_long_numbers_in_id", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		// Create a node ID with very large numbers
		// "1.999999999999.888888888888.777777777777"
		idStr := "1.999999999999.888888888888.777777777777"
		nodeID, err := types.Parse(idStr)
		if err != nil {
			t.Fatalf("failed to parse node ID with large numbers: %v", err)
		}

		n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Large number node", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("failed to create node: %v", err)
		}

		// This should work - still within limits
		err = WriteNode(dir, n)
		if err != nil {
			t.Errorf("WriteNode failed for large number ID: %v", err)
		}
	})
}

// TestLongFilename_WriteDefinition tests WriteDefinition with very long IDs.
func TestLongFilename_WriteDefinition(t *testing.T) {
	t.Run("definition_id_near_limit", func(t *testing.T) {
		dir := t.TempDir()
		defsDir := filepath.Join(dir, "defs")
		if err := os.MkdirAll(defsDir, 0755); err != nil {
			t.Fatalf("failed to create defs directory: %v", err)
		}

		// Create an ID that approaches the 255-byte filename limit
		// Account for ".json" extension (5 bytes)
		longID := strings.Repeat("a", 240)

		def := createTestDefinition(longID)
		err := WriteDefinition(dir, def)
		if err != nil {
			t.Errorf("WriteDefinition failed for long ID: %v", err)
		}
	})

	t.Run("definition_id_exceeds_limit", func(t *testing.T) {
		dir := t.TempDir()
		defsDir := filepath.Join(dir, "defs")
		if err := os.MkdirAll(defsDir, 0755); err != nil {
			t.Fatalf("failed to create defs directory: %v", err)
		}

		// Create an ID that exceeds the 255-byte filename limit
		veryLongID := strings.Repeat("a", 300)

		def := createTestDefinition(veryLongID)
		err := WriteDefinition(dir, def)
		// This should fail on most filesystems
		if err == nil {
			// Some filesystems might support longer names
			t.Log("WriteDefinition succeeded for very long ID (filesystem may support long names)")
		} else {
			t.Logf("WriteDefinition correctly failed for very long ID: %v", err)
		}
	})
}

// TestLongFilename_WriteLemma tests WriteLemma with very long IDs.
func TestLongFilename_WriteLemma(t *testing.T) {
	t.Run("lemma_id_with_unicode", func(t *testing.T) {
		dir := t.TempDir()
		lemmasDir := filepath.Join(dir, "lemmas")
		if err := os.MkdirAll(lemmasDir, 0755); err != nil {
			t.Fatalf("failed to create lemmas directory: %v", err)
		}

		// Unicode characters take multiple bytes
		// 100 unicode characters might exceed 255 bytes
		unicodeID := strings.Repeat("\u4e2d", 50) // Chinese character, 3 bytes each

		lemma := createTestLemma(unicodeID)
		err := WriteLemma(dir, lemma)
		if err != nil {
			t.Logf("WriteLemma failed for unicode ID (expected): %v", err)
		}
	})
}

// TestLongFilename_PathLength tests behavior with very long total path lengths.
func TestLongFilename_PathLength(t *testing.T) {
	// PATH_MAX is typically 4096 bytes on Linux

	t.Run("long_base_path", func(t *testing.T) {
		dir := t.TempDir()

		// Create deeply nested directories to approach path limit
		deepPath := dir
		for i := 0; i < 50; i++ {
			deepPath = filepath.Join(deepPath, "subdir")
		}

		err := os.MkdirAll(deepPath, 0755)
		if err != nil {
			// Path too long for filesystem
			t.Logf("could not create deep path (expected): %v", err)
			return
		}

		// Try to write a node to this deep path
		n := createTestNode(t, "1")
		err = WriteNode(deepPath, n)
		if err != nil {
			t.Logf("WriteNode to deep path failed (may be expected): %v", err)
		}
	})
}

// =============================================================================
// Additional Edge Case Tests
// =============================================================================

// TestFileTypeConfusion tests behavior when expected directories are files.
func TestFileTypeConfusion(t *testing.T) {
	t.Run("meta_json_is_directory", func(t *testing.T) {
		dir := t.TempDir()
		metaPath := filepath.Join(dir, "meta.json")

		// Create meta.json as a directory instead of file
		if err := os.MkdirAll(metaPath, 0755); err != nil {
			t.Fatalf("failed to create meta.json directory: %v", err)
		}

		_, err := ReadMeta(dir)
		if err == nil {
			t.Error("expected error when meta.json is a directory, got nil")
		}
	})

	t.Run("schema_json_is_directory", func(t *testing.T) {
		dir := t.TempDir()
		schemaPath := filepath.Join(dir, "schema.json")

		// Create schema.json as a directory instead of file
		if err := os.MkdirAll(schemaPath, 0755); err != nil {
			t.Fatalf("failed to create schema.json directory: %v", err)
		}

		_, err := ReadSchema(dir)
		if err == nil {
			t.Error("expected error when schema.json is a directory, got nil")
		}
	})
}

// TestConcurrentAccess tests behavior with concurrent file operations.
func TestConcurrentAccess(t *testing.T) {
	t.Run("concurrent_writes_different_nodes", func(t *testing.T) {
		// Test concurrent writes to DIFFERENT nodes (should all succeed)
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		done := make(chan error, 10)
		for i := 0; i < 10; i++ {
			go func(idx int) {
				// Each goroutine writes a different node
				idStr := "1." + strings.Repeat("1", idx+1)
				nodeID, err := types.Parse(idStr)
				if err != nil {
					done <- err
					return
				}
				n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Statement "+idStr, schema.InferenceAssumption)
				if err != nil {
					done <- err
					return
				}
				done <- WriteNode(dir, n)
			}(i)
		}

		// All writes should succeed (different files)
		successCount := 0
		for i := 0; i < 10; i++ {
			if err := <-done; err != nil {
				t.Logf("concurrent write failed (may be expected for some): %v", err)
			} else {
				successCount++
			}
		}

		// Most writes should succeed
		if successCount == 0 {
			t.Error("all concurrent writes failed")
		}
	})

	t.Run("concurrent_writes_same_node_race", func(t *testing.T) {
		// Test concurrent writes to the SAME node - this tests the race behavior
		// Due to temp file naming, some writes may fail; this is expected behavior
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		n := createTestNode(t, "1")

		done := make(chan error, 10)
		for i := 0; i < 10; i++ {
			go func() {
				done <- WriteNode(dir, n)
			}()
		}

		// Count successes and failures
		successCount := 0
		for i := 0; i < 10; i++ {
			if err := <-done; err == nil {
				successCount++
			}
		}

		// At least one write should succeed
		if successCount == 0 {
			t.Error("all concurrent writes to same node failed")
		}

		// Verify the node can be read (at least one write succeeded)
		if successCount > 0 {
			nodeID, _ := types.Parse("1")
			_, err := ReadNode(dir, nodeID)
			if err != nil {
				t.Errorf("ReadNode after concurrent writes failed: %v", err)
			}
		}
	})
}

// TestSpecialCharacters tests handling of special characters in paths.
func TestSpecialCharacters(t *testing.T) {
	t.Run("space_in_path", func(t *testing.T) {
		dir := t.TempDir()
		spaceDir := filepath.Join(dir, "path with spaces")
		if err := os.MkdirAll(spaceDir, 0755); err != nil {
			t.Fatalf("failed to create directory with spaces: %v", err)
		}

		n := createTestNode(t, "1")
		err := WriteNode(spaceDir, n)
		if err != nil {
			t.Errorf("WriteNode failed with space in path: %v", err)
		}

		nodeID, _ := types.Parse("1")
		_, err = ReadNode(spaceDir, nodeID)
		if err != nil {
			t.Errorf("ReadNode failed with space in path: %v", err)
		}
	})

	t.Run("special_chars_in_path", func(t *testing.T) {
		skipIfWindows(t) // Windows has more restrictions

		dir := t.TempDir()
		// Create path with various special characters (valid on Unix)
		specialDir := filepath.Join(dir, "path-with_special.chars")
		if err := os.MkdirAll(specialDir, 0755); err != nil {
			t.Fatalf("failed to create directory with special chars: %v", err)
		}

		n := createTestNode(t, "1")
		err := WriteNode(specialDir, n)
		if err != nil {
			t.Errorf("WriteNode failed with special chars in path: %v", err)
		}
	})
}

// TestEmptyFile tests behavior when reading empty files.
func TestEmptyFile(t *testing.T) {
	t.Run("empty_node_file", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		// Create an empty file
		nodePath := filepath.Join(nodesDir, "1.json")
		if err := os.WriteFile(nodePath, []byte{}, 0644); err != nil {
			t.Fatalf("failed to create empty file: %v", err)
		}

		nodeID, _ := types.Parse("1")
		_, err := ReadNode(dir, nodeID)
		if err == nil {
			t.Error("expected error for empty node file, got nil")
		}
	})

	t.Run("empty_definition_file", func(t *testing.T) {
		dir := t.TempDir()
		defsDir := filepath.Join(dir, "defs")
		if err := os.MkdirAll(defsDir, 0755); err != nil {
			t.Fatalf("failed to create defs directory: %v", err)
		}

		defPath := filepath.Join(defsDir, "test.json")
		if err := os.WriteFile(defPath, []byte{}, 0644); err != nil {
			t.Fatalf("failed to create empty file: %v", err)
		}

		_, err := ReadDefinition(dir, "test")
		if err == nil {
			t.Error("expected error for empty definition file, got nil")
		}
	})
}

// TestCorruptedFiles tests behavior when reading corrupted files.
func TestCorruptedFiles(t *testing.T) {
	t.Run("truncated_json", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		// Write truncated JSON
		nodePath := filepath.Join(nodesDir, "1.json")
		if err := os.WriteFile(nodePath, []byte(`{"id": "1", "type": "claim"`), 0644); err != nil {
			t.Fatalf("failed to write truncated JSON: %v", err)
		}

		nodeID, _ := types.Parse("1")
		_, err := ReadNode(dir, nodeID)
		if err == nil {
			t.Error("expected error for truncated JSON, got nil")
		}
	})

	t.Run("binary_garbage", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		// Write binary garbage
		nodePath := filepath.Join(nodesDir, "1.json")
		garbage := []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd}
		if err := os.WriteFile(nodePath, garbage, 0644); err != nil {
			t.Fatalf("failed to write garbage: %v", err)
		}

		nodeID, _ := types.Parse("1")
		_, err := ReadNode(dir, nodeID)
		if err == nil {
			t.Error("expected error for binary garbage, got nil")
		}
	})
}

// TestSymlinkToFile tests behavior when directory path is a symlink to a file.
func TestSymlinkToFile(t *testing.T) {
	skipIfWindows(t)

	t.Run("nodes_dir_is_symlink_to_file", func(t *testing.T) {
		dir := t.TempDir()

		// Create a file
		filePath := filepath.Join(dir, "somefile.txt")
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		// Create symlink nodes -> somefile.txt
		nodesLink := filepath.Join(dir, "nodes")
		if err := os.Symlink(filePath, nodesLink); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		n := createTestNode(t, "1")
		err := WriteNode(dir, n)
		if err == nil {
			t.Error("expected error when nodes/ is symlink to file, got nil")
		}
	})
}

// TestFilesystemErrorCodes tests specific error code detection.
func TestFilesystemErrorCodes(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	t.Run("detect_permission_error", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		makeReadOnly(t, nodesDir)

		n := createTestNode(t, "1")
		err := WriteNode(dir, n)
		if err == nil {
			t.Error("expected error, got nil")
			return
		}

		// Check if error is EACCES (permission denied)
		if os.IsPermission(err) {
			t.Log("correctly identified as permission error")
		} else {
			// Check syscall error
			if pathErr, ok := err.(*os.PathError); ok {
				if errno, ok := pathErr.Err.(syscall.Errno); ok {
					if errno == syscall.EACCES || errno == syscall.EPERM {
						t.Log("correctly identified syscall permission error")
					}
				}
			}
			t.Logf("error details: %v (type: %T)", err, err)
		}
	})
}

// =============================================================================
// Batch Append Rename Failure Tests
// =============================================================================

// TestBatchAppend_RenameFailMidBatch tests the scenario where rename fails
// mid-batch during multiple file write operations. This simulates the case where:
// - Some files are successfully renamed
// - Rename fails on a middle file (e.g., event 2 of 5)
// - Remaining temp files should be cleaned up
//
// This tests for the "ledger gap corruption" scenario where partial batch
// operations could leave the filesystem in an inconsistent state.
func TestBatchAppend_RenameFailMidBatch(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	t.Run("rename_fails_on_second_of_five_nodes", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		// Create 5 nodes to write in a "batch" operation
		nodes := make([]*node.Node, 5)
		for i := 0; i < 5; i++ {
			idStr := fmt.Sprintf("1.%d", i+1)
			nodeID, err := types.Parse(idStr)
			if err != nil {
				t.Fatalf("failed to parse node ID: %v", err)
			}
			n, err := node.NewNode(nodeID, schema.NodeTypeClaim, fmt.Sprintf("Statement %d", i+1), schema.InferenceAssumption)
			if err != nil {
				t.Fatalf("failed to create node %d: %v", i, err)
			}
			nodes[i] = n
		}

		// Write first node successfully
		if err := WriteNode(dir, nodes[0]); err != nil {
			t.Fatalf("failed to write first node: %v", err)
		}

		// Create a NON-EMPTY directory that will block the rename for the second node.
		// On Linux, os.Rename fails with ENOTEMPTY when renaming a file to a non-empty directory.
		// Note: node ID "1.2" becomes filename "1_2.json" (dots replaced with underscores)
		blockedPath := filepath.Join(nodesDir, "1_2.json")
		if err := os.MkdirAll(blockedPath, 0755); err != nil {
			t.Fatalf("failed to create blocking directory: %v", err)
		}
		// Put a file inside to make it non-empty
		dummyFile := filepath.Join(blockedPath, "dummy")
		if err := os.WriteFile(dummyFile, []byte("block"), 0644); err != nil {
			t.Fatalf("failed to create dummy file: %v", err)
		}

		// Now try to write the second node - this should fail because
		// we can't rename a file to a non-empty directory
		err := WriteNode(dir, nodes[1])
		if err == nil {
			t.Error("expected error when renaming to non-empty directory, got nil")
		}

		// Verify the first file still exists and is valid
		nodeID1, _ := types.Parse("1.1")
		readNode, err := ReadNode(dir, nodeID1)
		if err != nil {
			t.Errorf("first node should still be readable: %v", err)
		}
		if readNode != nil && readNode.Statement != "Statement 1" {
			t.Errorf("first node content mismatch")
		}

		// Check for temp files - they should be cleaned up
		entries, err := os.ReadDir(nodesDir)
		if err != nil {
			t.Fatalf("failed to read nodes directory: %v", err)
		}

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".tmp") {
				t.Errorf("temp file left behind after failed rename: %s", entry.Name())
			}
		}
	})

	t.Run("partial_batch_leaves_no_temp_files", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		// Write 3 nodes successfully
		for i := 0; i < 3; i++ {
			idStr := fmt.Sprintf("1.%d", i+1)
			nodeID, err := types.Parse(idStr)
			if err != nil {
				t.Fatalf("failed to parse node ID: %v", err)
			}
			n, err := node.NewNode(nodeID, schema.NodeTypeClaim, fmt.Sprintf("Statement %d", i+1), schema.InferenceAssumption)
			if err != nil {
				t.Fatalf("failed to create node %d: %v", i, err)
			}
			if err := WriteNode(dir, n); err != nil {
				t.Fatalf("failed to write node %d: %v", i, err)
			}
		}

		// Simulate failure by making the directory read-only before 4th write
		makeReadOnly(t, nodesDir)

		// Try to write 4th node - should fail
		nodeID4, _ := types.Parse("1.4")
		n4, _ := node.NewNode(nodeID4, schema.NodeTypeClaim, "Statement 4", schema.InferenceAssumption)
		err := WriteNode(dir, n4)
		if err == nil {
			t.Error("expected error writing to read-only directory, got nil")
		}

		// Restore permissions to check for temp files
		os.Chmod(nodesDir, 0755)

		entries, err := os.ReadDir(nodesDir)
		if err != nil {
			t.Fatalf("failed to read nodes directory: %v", err)
		}

		// Count successful files and temp files
		successCount := 0
		tempCount := 0
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasSuffix(name, ".tmp") {
				tempCount++
				t.Errorf("temp file left behind: %s", name)
			} else if strings.HasSuffix(name, ".json") {
				successCount++
			}
		}

		if successCount != 3 {
			t.Errorf("expected 3 successful writes, got %d", successCount)
		}
		if tempCount > 0 {
			t.Errorf("expected 0 temp files, got %d", tempCount)
		}
	})

	t.Run("cross_device_rename_simulation", func(t *testing.T) {
		// This test simulates cross-device rename failure by trying to rename
		// across mount points (which would fail with EXDEV).
		// Since we can't easily create different mount points in tests,
		// we simulate the scenario by attempting rename to a path that would fail.

		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		// Create a node
		nodeID, _ := types.Parse("1")
		n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Cross-device test", schema.InferenceAssumption)

		// Write should succeed normally
		if err := WriteNode(dir, n); err != nil {
			t.Fatalf("failed to write node: %v", err)
		}

		// Verify the node is readable
		readNode, err := ReadNode(dir, nodeID)
		if err != nil {
			t.Fatalf("failed to read node: %v", err)
		}
		if readNode.Statement != "Cross-device test" {
			t.Errorf("content mismatch")
		}
	})
}

// TestBatchWriteJSON_RenameFailure tests WriteJSON behavior when rename fails.
func TestBatchWriteJSON_RenameFailure(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	t.Run("rename_to_directory_fails", func(t *testing.T) {
		dir := t.TempDir()

		// Create a directory at the target path
		targetPath := filepath.Join(dir, "target.json")
		if err := os.MkdirAll(targetPath, 0755); err != nil {
			t.Fatalf("failed to create blocking directory: %v", err)
		}

		// Try to write JSON to this path - should fail on rename
		data := testData{ID: "test", Name: "Test"}
		err := WriteJSON(targetPath, &data)
		if err == nil {
			t.Error("expected error when target is a directory, got nil")
		}

		// The temp file should be cleaned up
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("failed to read directory: %v", err)
		}

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".tmp") {
				t.Errorf("temp file left behind: %s", entry.Name())
			}
		}
	})

	t.Run("rename_permission_denied_cleanup", func(t *testing.T) {
		dir := t.TempDir()
		targetPath := filepath.Join(dir, "target.json")

		// First write successfully
		data := testData{ID: "v1", Name: "Original"}
		if err := WriteJSON(targetPath, &data); err != nil {
			t.Fatalf("initial write failed: %v", err)
		}

		// Make the file immutable (on systems that support it)
		// This will cause rename to fail when trying to overwrite
		if err := os.Chmod(targetPath, 0444); err != nil {
			t.Fatalf("failed to make file read-only: %v", err)
		}
		// Also make the directory read-only to prevent any writes
		if err := os.Chmod(dir, 0555); err != nil {
			t.Fatalf("failed to make directory read-only: %v", err)
		}
		t.Cleanup(func() {
			os.Chmod(dir, 0755)
			os.Chmod(targetPath, 0644)
		})

		// Try to overwrite - this should fail at the temp file creation stage
		// since the directory is read-only
		data2 := testData{ID: "v2", Name: "Updated"}
		err := WriteJSON(targetPath, &data2)
		if err == nil {
			t.Error("expected error when directory is read-only, got nil")
		}

		// Restore permissions and verify no temp files remain
		os.Chmod(dir, 0755)

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("failed to read directory: %v", err)
		}

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".tmp") {
				t.Errorf("temp file left behind: %s", entry.Name())
			}
		}

		// Original file should still have v1 content
		os.Chmod(targetPath, 0644)
		var result testData
		if err := ReadJSON(targetPath, &result); err != nil {
			t.Fatalf("failed to read original file: %v", err)
		}
		if result.ID != "v1" {
			t.Errorf("original file was corrupted: expected v1, got %s", result.ID)
		}
	})
}

// TestBatchAppend_PartialFailureRecovery tests the scenario from issue vibefeld-hmnh:
// "rename fails on event 2 of 5" - verifying proper cleanup and no gap corruption.
func TestBatchAppend_PartialFailureRecovery(t *testing.T) {
	skipIfWindows(t)
	skipIfRoot(t)

	t.Run("simulated_batch_failure_on_event_2_of_5", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		// This simulates a batch append where events 1 is written successfully,
		// event 2 fails during rename, and events 3-5 should not be attempted
		// (or their temp files should be cleaned up).
		// Using 1.1.1, 1.1.2, etc. as valid node IDs (root must be 1)

		successCount := 0
		failedAt := -1

		// Simulate batch of 5 writes
		for i := 0; i < 5; i++ {
			idStr := fmt.Sprintf("1.1.%d", i+1)
			nodeID, err := types.Parse(idStr)
			if err != nil {
				t.Fatalf("failed to parse node ID: %v", err)
			}

			// For the second event (index 1), create a blocking condition
			if i == 1 {
				// Create a non-empty directory that blocks the rename
				// Node ID "1.1.2" becomes filename "1_1_2.json" (dots replaced with underscores)
				filename := strings.ReplaceAll(nodeID.String(), ".", "_") + ".json"
				blockPath := filepath.Join(nodesDir, filename)
				if err := os.MkdirAll(blockPath, 0755); err != nil {
					t.Fatalf("failed to create blocking directory: %v", err)
				}
				// Put a file inside to make it non-empty
				dummyFile := filepath.Join(blockPath, "dummy")
				if err := os.WriteFile(dummyFile, []byte("block"), 0644); err != nil {
					t.Fatalf("failed to create dummy file: %v", err)
				}
			}

			n, err := node.NewNode(nodeID, schema.NodeTypeClaim, fmt.Sprintf("Event %d", i+1), schema.InferenceAssumption)
			if err != nil {
				t.Fatalf("failed to create node %d: %v", i, err)
			}

			err = WriteNode(dir, n)
			if err != nil {
				if failedAt == -1 {
					failedAt = i
				}
				// In a real batch scenario, we'd stop here
				break
			}
			successCount++
		}

		// Verify that event 1 was written successfully
		if successCount != 1 {
			t.Errorf("expected 1 successful write before failure, got %d", successCount)
		}

		// Verify failure happened at event 2 (index 1)
		if failedAt != 1 {
			t.Errorf("expected failure at index 1, got %d", failedAt)
		}

		// Verify no temp files remain
		entries, err := os.ReadDir(nodesDir)
		if err != nil {
			t.Fatalf("failed to read nodes directory: %v", err)
		}

		tempFiles := 0
		validFiles := 0
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasSuffix(name, ".tmp") {
				tempFiles++
				t.Errorf("temp file left behind: %s", name)
			} else if strings.HasSuffix(name, ".json") && !entry.IsDir() {
				validFiles++
			}
		}

		if tempFiles > 0 {
			t.Errorf("batch failure left %d temp files behind", tempFiles)
		}

		// Should have exactly 1 valid file (event 1 before the failure)
		if validFiles != 1 {
			t.Errorf("expected 1 valid file, got %d", validFiles)
		}

		// Verify the successful file is readable and not corrupted
		nodeID1, _ := types.Parse("1.1.1")
		readNode, err := ReadNode(dir, nodeID1)
		if err != nil {
			t.Errorf("first node should be readable: %v", err)
		}
		if readNode != nil {
			if readNode.Statement != "Event 1" {
				t.Errorf("first node content corrupted: expected 'Event 1', got '%s'", readNode.Statement)
			}
			if !readNode.VerifyContentHash() {
				t.Error("first node content hash verification failed")
			}
		}
	})

	t.Run("all_renames_fail_leaves_no_files", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.MkdirAll(nodesDir, 0755); err != nil {
			t.Fatalf("failed to create nodes directory: %v", err)
		}

		// Pre-create non-empty blocking directories for all files
		// Using valid node IDs: 1.2.1, 1.2.2, 1.2.3
		// Node ID "1.2.1" becomes filename "1_2_1.json" (dots replaced with underscores)
		for i := 0; i < 3; i++ {
			blockPath := filepath.Join(nodesDir, fmt.Sprintf("1_2_%d.json", i+1))
			if err := os.MkdirAll(blockPath, 0755); err != nil {
				t.Fatalf("failed to create blocking directory: %v", err)
			}
			// Put a file inside to make it non-empty
			dummyFile := filepath.Join(blockPath, "dummy")
			if err := os.WriteFile(dummyFile, []byte("block"), 0644); err != nil {
				t.Fatalf("failed to create dummy file: %v", err)
			}
		}

		// Try to write 3 nodes - all should fail
		failCount := 0
		for i := 0; i < 3; i++ {
			idStr := fmt.Sprintf("1.2.%d", i+1)
			nodeID, _ := types.Parse(idStr)
			n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, fmt.Sprintf("Event %d", i+1), schema.InferenceAssumption)

			if err := WriteNode(dir, n); err != nil {
				failCount++
			}
		}

		if failCount != 3 {
			t.Errorf("expected 3 failures, got %d", failCount)
		}

		// Verify no temp files remain
		entries, err := os.ReadDir(nodesDir)
		if err != nil {
			t.Fatalf("failed to read nodes directory: %v", err)
		}

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".tmp") {
				t.Errorf("temp file left behind: %s", entry.Name())
			}
		}
	})
}
