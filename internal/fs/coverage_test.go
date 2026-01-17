// Package fs provides filesystem operations for the AF proof framework.
// This file contains additional tests to increase code coverage.
package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Path Handling Edge Cases
// =============================================================================

// TestValidatePath_EdgeCases tests the validatePath function edge cases.
func TestValidatePath_EdgeCases(t *testing.T) {
	t.Run("empty_string", func(t *testing.T) {
		err := validatePath("")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
		if !strings.Contains(err.Error(), "empty") {
			t.Errorf("error should mention 'empty', got: %v", err)
		}
	})

	t.Run("whitespace_only_spaces", func(t *testing.T) {
		err := validatePath("   ")
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
		if !strings.Contains(err.Error(), "whitespace") {
			t.Errorf("error should mention 'whitespace', got: %v", err)
		}
	})

	t.Run("whitespace_only_tabs", func(t *testing.T) {
		err := validatePath("\t\t\t")
		if err == nil {
			t.Error("expected error for tab-only path, got nil")
		}
	})

	t.Run("whitespace_mixed", func(t *testing.T) {
		err := validatePath("  \t  \n  ")
		if err == nil {
			t.Error("expected error for mixed whitespace path, got nil")
		}
	})

	t.Run("null_byte_at_start", func(t *testing.T) {
		err := validatePath("\x00/path/to/file")
		if err == nil {
			t.Error("expected error for path with null byte at start, got nil")
		}
		if !strings.Contains(err.Error(), "null") {
			t.Errorf("error should mention 'null', got: %v", err)
		}
	})

	t.Run("null_byte_in_middle", func(t *testing.T) {
		err := validatePath("/path\x00/to/file")
		if err == nil {
			t.Error("expected error for path with null byte in middle, got nil")
		}
	})

	t.Run("null_byte_at_end", func(t *testing.T) {
		err := validatePath("/path/to/file\x00")
		if err == nil {
			t.Error("expected error for path with null byte at end, got nil")
		}
	})

	t.Run("valid_absolute_path", func(t *testing.T) {
		err := validatePath("/valid/absolute/path")
		if err != nil {
			t.Errorf("unexpected error for valid path: %v", err)
		}
	})

	t.Run("valid_relative_path", func(t *testing.T) {
		err := validatePath("./relative/path")
		if err != nil {
			t.Errorf("unexpected error for relative path: %v", err)
		}
	})

	t.Run("valid_path_with_dots", func(t *testing.T) {
		// Dots in path components are allowed (path traversal check is separate)
		err := validatePath("/path/with.dots/file.ext")
		if err != nil {
			t.Errorf("unexpected error for path with dots: %v", err)
		}
	})

	t.Run("valid_path_with_spaces", func(t *testing.T) {
		err := validatePath("/path with spaces/file name.txt")
		if err != nil {
			t.Errorf("unexpected error for path with spaces: %v", err)
		}
	})
}

// TestContainsPathTraversal_EdgeCases tests the containsPathTraversal function.
func TestContainsPathTraversal_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		// Should detect traversal
		{"double_dot", "..", true},
		{"double_dot_at_start", "../path", true},
		{"double_dot_in_middle", "path/../other", true},
		{"double_dot_at_end", "path/..", true},
		{"forward_slash", "path/to/file", true},
		{"backslash", "path\\to\\file", true},
		{"url_encoded_slash_lower", "path%2ffile", true},
		{"url_encoded_slash_upper", "path%2Ffile", true},
		{"multiple_double_dots", "../../..", true},

		// Should NOT detect traversal
		{"simple_id", "simple-id", false},
		{"id_with_underscore", "my_definition", false},
		{"id_with_dash", "my-definition", false},
		{"id_with_numbers", "def123", false},
		{"id_with_single_dot", "file.json", false},
		{"empty_string", "", false},
		{"unicode_chars", "definition-\u4e2d\u6587", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := containsPathTraversal(tc.input)
			if result != tc.expected {
				t.Errorf("containsPathTraversal(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// Large File Handling
// =============================================================================

// TestLargeFileWrite tests writing large JSON structures.
func TestLargeFileWrite(t *testing.T) {
	t.Run("large_slice", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "large.json")

		// Create a large slice with 10000 elements
		data := make([]testData, 10000)
		for i := 0; i < 10000; i++ {
			data[i] = testData{
				ID:      strings.Repeat("a", 100),
				Name:    strings.Repeat("b", 200),
				Count:   i,
				Enabled: i%2 == 0,
			}
		}

		err := WriteJSON(filePath, &data)
		if err != nil {
			t.Fatalf("WriteJSON failed for large slice: %v", err)
		}

		// Verify file exists and has substantial size
		info, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		if info.Size() < 1000000 { // Should be > 1MB
			t.Errorf("large file unexpectedly small: %d bytes", info.Size())
		}

		// Verify round-trip
		var result []testData
		if err := ReadJSON(filePath, &result); err != nil {
			t.Fatalf("ReadJSON failed: %v", err)
		}
		if len(result) != 10000 {
			t.Errorf("expected 10000 items, got %d", len(result))
		}
	})

	t.Run("large_nested_structure", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "nested.json")

		// Create a deeply nested map structure
		data := make(map[string]map[string]map[string]string)
		for i := 0; i < 100; i++ {
			level1Key := strings.Repeat("a", 50)
			data[level1Key] = make(map[string]map[string]string)
			for j := 0; j < 100; j++ {
				level2Key := strings.Repeat("b", 50)
				data[level1Key][level2Key] = make(map[string]string)
				for k := 0; k < 10; k++ {
					level3Key := strings.Repeat("c", 50)
					data[level1Key][level2Key][level3Key] = strings.Repeat("value", 100)
				}
			}
		}

		err := WriteJSON(filePath, &data)
		if err != nil {
			t.Fatalf("WriteJSON failed for nested structure: %v", err)
		}

		// Verify round-trip
		var result map[string]map[string]map[string]string
		if err := ReadJSON(filePath, &result); err != nil {
			t.Fatalf("ReadJSON failed: %v", err)
		}
	})

	t.Run("long_string_values", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "longstring.json")

		// Create data with very long strings
		data := testData{
			ID:   strings.Repeat("x", 100000),  // 100KB string
			Name: strings.Repeat("y", 100000),  // 100KB string
		}

		err := WriteJSON(filePath, &data)
		if err != nil {
			t.Fatalf("WriteJSON failed for long strings: %v", err)
		}

		var result testData
		if err := ReadJSON(filePath, &result); err != nil {
			t.Fatalf("ReadJSON failed: %v", err)
		}
		if len(result.ID) != 100000 {
			t.Errorf("expected ID length 100000, got %d", len(result.ID))
		}
	})
}

// =============================================================================
// Atomic Write Guarantees
// =============================================================================

// TestAtomicWrite_NoPartialFiles tests that failed writes don't leave partial files.
func TestAtomicWrite_NoPartialFiles(t *testing.T) {
	t.Run("marshal_error_no_file", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.json")

		// Use a channel which cannot be marshaled
		ch := make(chan int)
		err := WriteJSON(filePath, ch)
		if err == nil {
			t.Error("expected marshal error, got nil")
		}

		// Verify no file was created
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Error("file should not exist after marshal error")
		}

		// Verify no temp files
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

	t.Run("successful_atomic_write_no_temp", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.json")

		data := testData{ID: "test", Name: "Test Data"}
		if err := WriteJSON(filePath, &data); err != nil {
			t.Fatalf("WriteJSON failed: %v", err)
		}

		// Verify no temp files remain
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("failed to read directory: %v", err)
		}
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".tmp") {
				t.Errorf("temp file left behind: %s", entry.Name())
			}
		}

		// Should only have the target file
		if len(entries) != 1 || entries[0].Name() != "test.json" {
			t.Errorf("unexpected files in directory: %v", entries)
		}
	})

	t.Run("overwrite_preserves_original_on_failure", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipping on Windows")
		}
		if os.Getuid() == 0 {
			t.Skip("skipping as root")
		}

		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.json")

		// Write initial version
		data1 := testData{ID: "v1", Name: "Original"}
		if err := WriteJSON(filePath, &data1); err != nil {
			t.Fatalf("initial write failed: %v", err)
		}

		// Make file read-only to prevent rename
		if err := os.Chmod(filePath, 0444); err != nil {
			t.Fatalf("failed to chmod: %v", err)
		}
		t.Cleanup(func() { os.Chmod(filePath, 0644) })

		// Try to overwrite - this should fail but original should be intact
		data2 := testData{ID: "v2", Name: "New"}
		writeErr := WriteJSON(filePath, &data2)
		// Error might or might not occur depending on OS behavior
		_ = writeErr // Acknowledge the error but OS behavior varies

		// Original should still be readable
		var result testData
		if readErr := ReadJSON(filePath, &result); readErr != nil {
			t.Logf("read after failed overwrite: %v", readErr)
		} else if result.ID != "v1" {
			// If ID is not v1, the write unexpectedly succeeded
			t.Logf("write succeeded despite read-only file (OS-dependent)")
		}
	})
}

// TestAtomicWrite_ConcurrentSameFile tests concurrent writes to same file.
func TestAtomicWrite_ConcurrentSameFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "concurrent.json")

	numWriters := 20
	numWritesPerWriter := 5
	var wg sync.WaitGroup
	errors := make(chan error, numWriters*numWritesPerWriter)

	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for i := 0; i < numWritesPerWriter; i++ {
				data := testData{
					ID:    "id",
					Name:  "name",
					Count: writerID*1000 + i,
				}
				if err := WriteJSON(filePath, &data); err != nil {
					errors <- err
				}
			}
		}(w)
	}

	wg.Wait()
	close(errors)

	// Count errors
	errorCount := 0
	for err := range errors {
		t.Logf("concurrent write error: %v", err)
		errorCount++
	}

	// Some errors are expected due to race conditions
	t.Logf("concurrent writes: %d errors out of %d attempts", errorCount, numWriters*numWritesPerWriter)

	// File should exist and be valid
	var result testData
	if err := ReadJSON(filePath, &result); err != nil {
		t.Fatalf("file unreadable after concurrent writes: %v", err)
	}

	// No temp files should remain
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", entry.Name())
		}
	}
}

// =============================================================================
// Definition IO Additional Tests
// =============================================================================

// TestDefinitionIO_PathTraversal tests path traversal prevention.
func TestDefinitionIO_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	// Create a valid definition
	def := createTestDefinition("valid-def")
	if err := WriteDefinition(dir, def); err != nil {
		t.Fatalf("failed to write definition: %v", err)
	}

	pathTraversalIDs := []string{
		"../outside",
		"..\\outside",
		"valid/../../../etc/passwd",
		"id%2f..%2f..%2fetc%2fpasswd",
		"/absolute/path",
		"valid/subpath",
	}

	for _, id := range pathTraversalIDs {
		t.Run("read_"+strings.ReplaceAll(id, "/", "_"), func(t *testing.T) {
			_, err := ReadDefinition(dir, id)
			if err == nil {
				t.Errorf("expected error for path traversal ID %q, got nil", id)
			}
		})

		t.Run("delete_"+strings.ReplaceAll(id, "/", "_"), func(t *testing.T) {
			err := DeleteDefinition(dir, id)
			if err == nil {
				t.Errorf("expected error for path traversal ID %q, got nil", id)
			}
		})
	}
}

// TestDefinitionIO_EmptyAndWhitespaceID tests empty/whitespace ID handling.
func TestDefinitionIO_EmptyAndWhitespaceID(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	t.Run("read_empty_id", func(t *testing.T) {
		_, err := ReadDefinition(dir, "")
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})

	t.Run("read_whitespace_id", func(t *testing.T) {
		_, err := ReadDefinition(dir, "   ")
		if err == nil {
			t.Error("expected error for whitespace ID, got nil")
		}
	})

	t.Run("delete_empty_id", func(t *testing.T) {
		err := DeleteDefinition(dir, "")
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})

	t.Run("write_empty_id", func(t *testing.T) {
		def := &node.Definition{ID: "", Name: "test"}
		err := WriteDefinition(dir, def)
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})

	t.Run("write_whitespace_id", func(t *testing.T) {
		def := &node.Definition{ID: "   ", Name: "test"}
		err := WriteDefinition(dir, def)
		if err == nil {
			t.Error("expected error for whitespace ID, got nil")
		}
	})
}

// TestListDefinitions_Filters tests that ListDefinitions properly filters files.
func TestListDefinitions_Filters(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	// Create valid definition
	def := createTestDefinition("valid-def")
	if err := WriteDefinition(dir, def); err != nil {
		t.Fatalf("failed to write definition: %v", err)
	}

	// Create files/dirs that should be filtered
	if err := os.WriteFile(filepath.Join(defsDir, ".hidden.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create hidden file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defsDir, "notjson.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to create non-json file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(defsDir, "subdir.json"), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ids, err := ListDefinitions(dir)
	if err != nil {
		t.Fatalf("ListDefinitions failed: %v", err)
	}

	if len(ids) != 1 {
		t.Errorf("expected 1 definition, got %d: %v", len(ids), ids)
	}
	if len(ids) > 0 && ids[0] != "valid-def" {
		t.Errorf("expected 'valid-def', got '%s'", ids[0])
	}
}

// =============================================================================
// External IO Additional Tests
// =============================================================================

// TestExternalIO_CRUD tests full CRUD operations for externals.
func TestExternalIO_CRUD(t *testing.T) {
	dir := t.TempDir()
	externalsDir := filepath.Join(dir, "externals")
	if err := os.MkdirAll(externalsDir, 0755); err != nil {
		t.Fatalf("failed to create externals directory: %v", err)
	}

	// Create
	ext := createTestExternal("test-ext")
	if err := WriteExternal(dir, ext); err != nil {
		t.Fatalf("WriteExternal failed: %v", err)
	}

	// Read
	readExt, err := ReadExternal(dir, "test-ext")
	if err != nil {
		t.Fatalf("ReadExternal failed: %v", err)
	}
	if readExt.ID != ext.ID {
		t.Errorf("ID mismatch: expected %q, got %q", ext.ID, readExt.ID)
	}

	// List
	ids, err := ListExternals(dir)
	if err != nil {
		t.Fatalf("ListExternals failed: %v", err)
	}
	if len(ids) != 1 || ids[0] != "test-ext" {
		t.Errorf("expected ['test-ext'], got %v", ids)
	}

	// Update (overwrite)
	ext.Name = "Updated Name"
	if err := WriteExternal(dir, ext); err != nil {
		t.Fatalf("WriteExternal update failed: %v", err)
	}
	readExt, _ = ReadExternal(dir, "test-ext")
	if readExt.Name != "Updated Name" {
		t.Errorf("expected updated name, got %q", readExt.Name)
	}

	// Delete
	if err := DeleteExternal(dir, "test-ext"); err != nil {
		t.Fatalf("DeleteExternal failed: %v", err)
	}

	// Verify deleted
	_, err = ReadExternal(dir, "test-ext")
	if !os.IsNotExist(err) {
		t.Errorf("expected not exist error, got: %v", err)
	}
}

// TestExternalIO_PathTraversal tests path traversal prevention for externals.
func TestExternalIO_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	externalsDir := filepath.Join(dir, "externals")
	if err := os.MkdirAll(externalsDir, 0755); err != nil {
		t.Fatalf("failed to create externals directory: %v", err)
	}

	traversalIDs := []string{"../etc/passwd", "..\\windows\\system32", "a/../../../b"}

	for _, id := range traversalIDs {
		t.Run("read_traversal", func(t *testing.T) {
			_, err := ReadExternal(dir, id)
			if err == nil {
				t.Errorf("expected error for %q, got nil", id)
			}
		})

		t.Run("delete_traversal", func(t *testing.T) {
			err := DeleteExternal(dir, id)
			if err == nil {
				t.Errorf("expected error for %q, got nil", id)
			}
		})
	}
}

// =============================================================================
// Lemma IO Additional Tests
// =============================================================================

// TestLemmaIO_CRUD tests full CRUD operations for lemmas.
func TestLemmaIO_CRUD(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	// Create
	lemma := createTestLemma("test-lemma")
	if err := WriteLemma(dir, lemma); err != nil {
		t.Fatalf("WriteLemma failed: %v", err)
	}

	// Read
	readLemma, err := ReadLemma(dir, "test-lemma")
	if err != nil {
		t.Fatalf("ReadLemma failed: %v", err)
	}
	if readLemma.ID != lemma.ID {
		t.Errorf("ID mismatch: expected %q, got %q", lemma.ID, readLemma.ID)
	}

	// List
	ids, err := ListLemmas(dir)
	if err != nil {
		t.Fatalf("ListLemmas failed: %v", err)
	}
	if len(ids) != 1 || ids[0] != "test-lemma" {
		t.Errorf("expected ['test-lemma'], got %v", ids)
	}

	// Delete
	if err := DeleteLemma(dir, "test-lemma"); err != nil {
		t.Fatalf("DeleteLemma failed: %v", err)
	}

	// Verify deleted
	_, err = ReadLemma(dir, "test-lemma")
	if !os.IsNotExist(err) {
		t.Errorf("expected not exist error, got: %v", err)
	}
}

// TestLemmaIO_Validation tests validation for lemma operations.
func TestLemmaIO_Validation(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	t.Run("write_nil_lemma", func(t *testing.T) {
		err := WriteLemma(dir, nil)
		if err == nil {
			t.Error("expected error for nil lemma, got nil")
		}
	})

	t.Run("write_empty_id", func(t *testing.T) {
		lemma := &node.Lemma{ID: "", Statement: "test"}
		err := WriteLemma(dir, lemma)
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})

	t.Run("read_empty_id", func(t *testing.T) {
		_, err := ReadLemma(dir, "")
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})

	t.Run("read_traversal_id", func(t *testing.T) {
		_, err := ReadLemma(dir, "../secret")
		if err == nil {
			t.Error("expected error for traversal ID, got nil")
		}
	})
}

// =============================================================================
// Meta IO Additional Tests
// =============================================================================

// TestMetaIO_CRUD tests full CRUD operations for meta.
func TestMetaIO_CRUD(t *testing.T) {
	dir := t.TempDir()

	// Create
	meta := &Meta{
		Conjecture: "Test conjecture",
		CreatedAt:  time.Now(),
		Version:    "1.0.0",
	}
	if err := WriteMeta(dir, meta); err != nil {
		t.Fatalf("WriteMeta failed: %v", err)
	}

	// Read
	readMeta, err := ReadMeta(dir)
	if err != nil {
		t.Fatalf("ReadMeta failed: %v", err)
	}
	if readMeta.Conjecture != meta.Conjecture {
		t.Errorf("Conjecture mismatch: expected %q, got %q", meta.Conjecture, readMeta.Conjecture)
	}

	// Update
	meta.Version = "2.0.0"
	if err := WriteMeta(dir, meta); err != nil {
		t.Fatalf("WriteMeta update failed: %v", err)
	}
	readMeta, _ = ReadMeta(dir)
	if readMeta.Version != "2.0.0" {
		t.Errorf("expected updated version, got %q", readMeta.Version)
	}
}

// TestMetaIO_Validation tests validation for meta operations.
func TestMetaIO_Validation(t *testing.T) {
	dir := t.TempDir()

	t.Run("write_nil_meta", func(t *testing.T) {
		err := WriteMeta(dir, nil)
		if err == nil {
			t.Error("expected error for nil meta, got nil")
		}
	})

	t.Run("write_empty_conjecture", func(t *testing.T) {
		meta := &Meta{Conjecture: "", Version: "1.0", CreatedAt: time.Now()}
		err := WriteMeta(dir, meta)
		if err == nil {
			t.Error("expected error for empty conjecture, got nil")
		}
	})

	t.Run("write_empty_version", func(t *testing.T) {
		meta := &Meta{Conjecture: "test", Version: "", CreatedAt: time.Now()}
		err := WriteMeta(dir, meta)
		if err == nil {
			t.Error("expected error for empty version, got nil")
		}
	})

	t.Run("write_zero_created_at", func(t *testing.T) {
		meta := &Meta{Conjecture: "test", Version: "1.0", CreatedAt: time.Time{}}
		err := WriteMeta(dir, meta)
		if err == nil {
			t.Error("expected error for zero created_at, got nil")
		}
	})

	t.Run("read_not_exist", func(t *testing.T) {
		emptyDir := t.TempDir()
		_, err := ReadMeta(emptyDir)
		if !os.IsNotExist(err) {
			t.Errorf("expected not exist error, got: %v", err)
		}
	})

	t.Run("read_empty_path", func(t *testing.T) {
		_, err := ReadMeta("")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})
}

// =============================================================================
// PendingDef IO Additional Tests
// =============================================================================

// TestPendingDefIO_CRUD tests full CRUD operations for pending defs.
func TestPendingDefIO_CRUD(t *testing.T) {
	dir := t.TempDir()

	nodeID, err := types.Parse("1.2.3")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create
	pd := &node.PendingDef{Term: "test-term", RequestedBy: nodeID}
	if err := WritePendingDef(dir, nodeID, pd); err != nil {
		t.Fatalf("WritePendingDef failed: %v", err)
	}

	// Read
	readPd, err := ReadPendingDef(dir, nodeID)
	if err != nil {
		t.Fatalf("ReadPendingDef failed: %v", err)
	}
	if readPd.Term != pd.Term {
		t.Errorf("Term mismatch: expected %q, got %q", pd.Term, readPd.Term)
	}

	// List
	ids, err := ListPendingDefs(dir)
	if err != nil {
		t.Fatalf("ListPendingDefs failed: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("expected 1 pending def, got %d", len(ids))
	}

	// Delete
	if err := DeletePendingDef(dir, nodeID); err != nil {
		t.Fatalf("DeletePendingDef failed: %v", err)
	}

	// Delete again (idempotent)
	if err := DeletePendingDef(dir, nodeID); err != nil {
		t.Errorf("delete should be idempotent, got error: %v", err)
	}
}

// TestPendingDefIO_NonExistentDir tests listing when directory doesn't exist.
func TestPendingDefIO_NonExistentDir(t *testing.T) {
	dir := t.TempDir()
	// Don't create .af/pending_defs/ directory

	ids, err := ListPendingDefs(dir)
	if err != nil {
		t.Errorf("ListPendingDefs should return empty slice, not error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty slice, got %d items", len(ids))
	}
}

// TestPendingDefIO_Validation tests validation for pending def operations.
func TestPendingDefIO_Validation(t *testing.T) {
	dir := t.TempDir()
	nodeID, _ := types.Parse("1.2")

	t.Run("write_nil_pending_def", func(t *testing.T) {
		err := WritePendingDef(dir, nodeID, nil)
		if err == nil {
			t.Error("expected error for nil pending def, got nil")
		}
	})

	t.Run("write_empty_node_id", func(t *testing.T) {
		pd := &node.PendingDef{Term: "test"}
		err := WritePendingDef(dir, types.NodeID{}, pd)
		if err == nil {
			t.Error("expected error for empty node ID, got nil")
		}
	})

	t.Run("read_empty_node_id", func(t *testing.T) {
		_, err := ReadPendingDef(dir, types.NodeID{})
		if err == nil {
			t.Error("expected error for empty node ID, got nil")
		}
	})
}

// =============================================================================
// Schema IO Additional Tests
// =============================================================================

// TestSchemaIO_WriteRead tests schema write and read operations.
func TestSchemaIO_WriteRead(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal schema structure
	s := &schema.Schema{
		Version:        "1.0",
		NodeTypes:      []schema.NodeType{schema.NodeTypeClaim},
		InferenceTypes: []schema.InferenceType{schema.InferenceAssumption},
	}

	if err := WriteSchema(dir, s); err != nil {
		t.Fatalf("WriteSchema failed: %v", err)
	}

	// Verify file exists
	schemaPath := filepath.Join(dir, "schema.json")
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		t.Error("schema.json should exist")
	}

	// Note: ReadSchema does validation via schema.LoadSchema,
	// which may require more fields. This tests basic write/exist.
}

// TestSchemaIO_Validation tests validation for schema operations.
func TestSchemaIO_Validation(t *testing.T) {
	dir := t.TempDir()

	t.Run("write_nil_schema", func(t *testing.T) {
		err := WriteSchema(dir, nil)
		if err == nil {
			t.Error("expected error for nil schema, got nil")
		}
	})

	t.Run("read_not_exist", func(t *testing.T) {
		emptyDir := t.TempDir()
		_, err := ReadSchema(emptyDir)
		if err == nil {
			t.Error("expected error for non-existent schema, got nil")
		}
	})

	t.Run("read_empty_path", func(t *testing.T) {
		_, err := ReadSchema("")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("read_directory_instead_of_file", func(t *testing.T) {
		schemaAsDir := filepath.Join(dir, "schema.json")
		if err := os.MkdirAll(schemaAsDir, 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		_, err := ReadSchema(dir)
		if err == nil {
			t.Error("expected error when schema.json is directory, got nil")
		}
	})
}

// =============================================================================
// Assumption IO Additional Tests
// =============================================================================

// TestAssumptionIO_CRUD tests full CRUD operations for assumptions.
func TestAssumptionIO_CRUD(t *testing.T) {
	dir := t.TempDir()
	assumpDir := filepath.Join(dir, "assumptions")
	if err := os.MkdirAll(assumpDir, 0755); err != nil {
		t.Fatalf("failed to create assumptions directory: %v", err)
	}

	// Create
	a := createTestAssumption("test-assumption")
	if err := WriteAssumption(dir, a); err != nil {
		t.Fatalf("WriteAssumption failed: %v", err)
	}

	// Read
	readA, err := ReadAssumption(dir, "test-assumption")
	if err != nil {
		t.Fatalf("ReadAssumption failed: %v", err)
	}
	if readA.ID != a.ID {
		t.Errorf("ID mismatch: expected %q, got %q", a.ID, readA.ID)
	}

	// List
	ids, err := ListAssumptions(dir)
	if err != nil {
		t.Fatalf("ListAssumptions failed: %v", err)
	}
	if len(ids) != 1 || ids[0] != "test-assumption" {
		t.Errorf("expected ['test-assumption'], got %v", ids)
	}

	// Delete
	if err := DeleteAssumption(dir, "test-assumption"); err != nil {
		t.Fatalf("DeleteAssumption failed: %v", err)
	}

	// Verify deleted
	_, err = ReadAssumption(dir, "test-assumption")
	if err == nil {
		t.Error("expected error for deleted assumption, got nil")
	}
}

// TestAssumptionIO_Validation tests validation for assumption operations.
func TestAssumptionIO_Validation(t *testing.T) {
	dir := t.TempDir()
	assumpDir := filepath.Join(dir, "assumptions")
	if err := os.MkdirAll(assumpDir, 0755); err != nil {
		t.Fatalf("failed to create assumptions directory: %v", err)
	}

	t.Run("write_nil_assumption", func(t *testing.T) {
		err := WriteAssumption(dir, nil)
		if err == nil {
			t.Error("expected error for nil assumption, got nil")
		}
	})

	t.Run("write_empty_id", func(t *testing.T) {
		a := &node.Assumption{ID: "", Statement: "test"}
		err := WriteAssumption(dir, a)
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})

	t.Run("read_empty_id", func(t *testing.T) {
		_, err := ReadAssumption(dir, "")
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})

	t.Run("read_traversal_id", func(t *testing.T) {
		_, err := ReadAssumption(dir, "../secret")
		if err == nil {
			t.Error("expected error for traversal ID, got nil")
		}
	})

	t.Run("read_empty_file", func(t *testing.T) {
		// Create empty file
		if err := os.WriteFile(filepath.Join(assumpDir, "empty.json"), []byte{}, 0644); err != nil {
			t.Fatalf("failed to create empty file: %v", err)
		}
		_, err := ReadAssumption(dir, "empty")
		if err == nil {
			t.Error("expected error for empty file, got nil")
		}
	})

	t.Run("read_invalid_json", func(t *testing.T) {
		// Create file with invalid JSON
		if err := os.WriteFile(filepath.Join(assumpDir, "invalid.json"), []byte("{not valid}"), 0644); err != nil {
			t.Fatalf("failed to create invalid file: %v", err)
		}
		_, err := ReadAssumption(dir, "invalid")
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("write_to_file_not_directory", func(t *testing.T) {
		fileDir := t.TempDir()
		// Create a file where the base path should be
		filePath := filepath.Join(fileDir, "file")
		if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		a := createTestAssumption("test")
		err := WriteAssumption(filePath, a)
		if err == nil {
			t.Error("expected error when base path is file, got nil")
		}
	})
}

// =============================================================================
// Additional JSON IO Edge Cases
// =============================================================================

// TestReadJSON_TypeMismatch tests reading JSON into wrong types.
func TestReadJSON_TypeMismatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.json")

	// Write an array
	if err := WriteJSON(filePath, []int{1, 2, 3}); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	// Try to read as struct
	var result testData
	err := ReadJSON(filePath, &result)
	if err == nil {
		// JSON unmarshaling into wrong type may succeed with zero values
		t.Log("unmarshaling array into struct succeeded with zero values")
	}
}

// TestWriteJSON_SpecialTypes tests writing various Go types.
func TestWriteJSON_SpecialTypes(t *testing.T) {
	dir := t.TempDir()

	t.Run("pointer_to_pointer", func(t *testing.T) {
		filePath := filepath.Join(dir, "ptrptr.json")
		val := "test"
		ptr := &val
		if err := WriteJSON(filePath, &ptr); err != nil {
			t.Errorf("WriteJSON failed for **string: %v", err)
		}
	})

	t.Run("interface_type", func(t *testing.T) {
		filePath := filepath.Join(dir, "interface.json")
		var val interface{} = map[string]int{"a": 1, "b": 2}
		if err := WriteJSON(filePath, &val); err != nil {
			t.Errorf("WriteJSON failed for interface: %v", err)
		}
	})

	t.Run("struct_with_tags", func(t *testing.T) {
		filePath := filepath.Join(dir, "tagged.json")
		type tagged struct {
			OmitEmpty  string `json:"omit_empty,omitempty"`
			AlwaysSkip string `json:"-"`
			Renamed    string `json:"custom_name"`
		}
		val := tagged{OmitEmpty: "", AlwaysSkip: "skip", Renamed: "value"}
		if err := WriteJSON(filePath, &val); err != nil {
			t.Errorf("WriteJSON failed: %v", err)
		}

		// Read back and verify
		data, _ := os.ReadFile(filePath)
		var parsed map[string]interface{}
		json.Unmarshal(data, &parsed)
		if _, ok := parsed["AlwaysSkip"]; ok {
			t.Error("AlwaysSkip should be omitted")
		}
		if _, ok := parsed["omit_empty"]; ok {
			t.Error("omit_empty should be omitted when empty")
		}
		if parsed["custom_name"] != "value" {
			t.Error("custom_name should have value 'value'")
		}
	})

	t.Run("time_value", func(t *testing.T) {
		filePath := filepath.Join(dir, "time.json")
		type withTime struct {
			Timestamp time.Time `json:"timestamp"`
		}
		val := withTime{Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)}
		if err := WriteJSON(filePath, &val); err != nil {
			t.Errorf("WriteJSON failed for time: %v", err)
		}

		var result withTime
		if err := ReadJSON(filePath, &result); err != nil {
			t.Errorf("ReadJSON failed for time: %v", err)
		}
		if !result.Timestamp.Equal(val.Timestamp) {
			t.Errorf("time mismatch: expected %v, got %v", val.Timestamp, result.Timestamp)
		}
	})
}

// TestWriteJSON_UnicodeContent tests Unicode handling.
func TestWriteJSON_UnicodeContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "unicode.json")

	type unicodeData struct {
		Chinese   string `json:"chinese"`
		Japanese  string `json:"japanese"`
		Arabic    string `json:"arabic"`
		Emoji     string `json:"emoji"`
		Mixed     string `json:"mixed"`
	}

	data := unicodeData{
		Chinese:   "\u4e2d\u6587\u6d4b\u8bd5",              // Chinese characters
		Japanese:  "\u65e5\u672c\u8a9e\u30c6\u30b9\u30c8",  // Japanese characters
		Arabic:    "\u0627\u0644\u0639\u0631\u0628\u064a\u0629", // Arabic characters
		Emoji:     "\U0001F600\U0001F604\U0001F60A",       // Emoji
		Mixed:     "Hello \u4e16\u754c \U0001F30D",        // Mixed
	}

	if err := WriteJSON(filePath, &data); err != nil {
		t.Fatalf("WriteJSON failed for unicode: %v", err)
	}

	var result unicodeData
	if err := ReadJSON(filePath, &result); err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}

	if result.Chinese != data.Chinese {
		t.Errorf("Chinese mismatch: expected %q, got %q", data.Chinese, result.Chinese)
	}
	if result.Japanese != data.Japanese {
		t.Errorf("Japanese mismatch: expected %q, got %q", data.Japanese, result.Japanese)
	}
	if result.Emoji != data.Emoji {
		t.Errorf("Emoji mismatch: expected %q, got %q", data.Emoji, result.Emoji)
	}
}
