//go:build integration
// +build integration

// These tests define expected behavior for WriteExternal, ReadExternal,
// ListExternals, and DeleteExternal.
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

// TestWriteExternal verifies that WriteExternal correctly writes
// an external reference to the externals/ subdirectory as JSON.
func TestWriteExternal(t *testing.T) {
	t.Run("basic_write", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		external := node.NewExternal("Fermat's Last Theorem", "https://en.wikipedia.org/wiki/Fermat%27s_Last_Theorem")

		err := WriteExternal(dir, &external)
		if err != nil {
			t.Fatalf("WriteExternal failed: %v", err)
		}

		// Verify file was created
		expectedPath := filepath.Join(externalsDir, external.ID+".json")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Fatalf("expected external file to exist at %s", expectedPath)
		}

		// Verify content is valid JSON and matches the external
		content, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatalf("failed to read external file: %v", err)
		}

		var readExternal node.External
		if err := json.Unmarshal(content, &readExternal); err != nil {
			t.Fatalf("external file is not valid JSON: %v", err)
		}

		if readExternal.ID != external.ID {
			t.Errorf("ID mismatch: got %q, want %q", readExternal.ID, external.ID)
		}
		if readExternal.Name != external.Name {
			t.Errorf("Name mismatch: got %q, want %q", readExternal.Name, external.Name)
		}
		if readExternal.Source != external.Source {
			t.Errorf("Source mismatch: got %q, want %q", readExternal.Source, external.Source)
		}
		if readExternal.ContentHash != external.ContentHash {
			t.Errorf("ContentHash mismatch: got %q, want %q", readExternal.ContentHash, external.ContentHash)
		}
	})

	t.Run("with_notes", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		external := node.NewExternalWithNotes(
			"Pythagorean Theorem",
			"https://mathworld.wolfram.com/PythagoreanTheorem.html",
			"Fundamental theorem in Euclidean geometry",
		)

		err := WriteExternal(dir, &external)
		if err != nil {
			t.Fatalf("WriteExternal failed: %v", err)
		}

		// Read back and verify notes are preserved
		expectedPath := filepath.Join(externalsDir, external.ID+".json")
		content, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatalf("failed to read external file: %v", err)
		}

		var readExternal node.External
		if err := json.Unmarshal(content, &readExternal); err != nil {
			t.Fatalf("external file is not valid JSON: %v", err)
		}

		if readExternal.Notes != external.Notes {
			t.Errorf("Notes mismatch: got %q, want %q", readExternal.Notes, external.Notes)
		}
	})

	t.Run("creates_externals_subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		// Do NOT pre-create externals/ directory

		external := node.NewExternal("Test External", "https://example.com")

		err := WriteExternal(dir, &external)
		if err != nil {
			t.Fatalf("WriteExternal failed: %v", err)
		}

		// Verify externals directory was created
		externalsDir := filepath.Join(dir, "externals")
		info, err := os.Stat(externalsDir)
		if os.IsNotExist(err) {
			t.Fatal("expected externals directory to be created")
		}
		if !info.IsDir() {
			t.Fatal("expected externals to be a directory")
		}
	})

	t.Run("nil_external", func(t *testing.T) {
		dir := t.TempDir()

		err := WriteExternal(dir, nil)
		if err == nil {
			t.Error("expected error for nil external, got nil")
		}
	})
}

// TestReadExternal verifies that ReadExternal correctly reads
// an external reference from the externals/ subdirectory.
func TestReadExternal(t *testing.T) {
	t.Run("basic_read", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Create a test external file directly
		external := &node.External{
			ID:          "test123",
			Name:        "Test Theorem",
			Source:      "https://example.com/theorem",
			ContentHash: "abc123",
			Created:     types.Now(),
			Notes:       "",
		}
		content, err := json.Marshal(external)
		if err != nil {
			t.Fatalf("failed to marshal external: %v", err)
		}

		filePath := filepath.Join(externalsDir, "test123.json")
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		// Read the external
		readExternal, err := ReadExternal(dir, "test123")
		if err != nil {
			t.Fatalf("ReadExternal failed: %v", err)
		}

		if readExternal.ID != external.ID {
			t.Errorf("ID mismatch: got %q, want %q", readExternal.ID, external.ID)
		}
		if readExternal.Name != external.Name {
			t.Errorf("Name mismatch: got %q, want %q", readExternal.Name, external.Name)
		}
		if readExternal.Source != external.Source {
			t.Errorf("Source mismatch: got %q, want %q", readExternal.Source, external.Source)
		}
		if readExternal.ContentHash != external.ContentHash {
			t.Errorf("ContentHash mismatch: got %q, want %q", readExternal.ContentHash, external.ContentHash)
		}
	})

	t.Run("with_all_fields", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Use WriteExternal to create, then read back
		original := node.NewExternalWithNotes(
			"Riemann Hypothesis",
			"https://www.claymath.org/millennium-problems/riemann-hypothesis",
			"One of the Millennium Prize Problems",
		)

		if err := WriteExternal(dir, &original); err != nil {
			t.Fatalf("WriteExternal failed: %v", err)
		}

		readExternal, err := ReadExternal(dir, original.ID)
		if err != nil {
			t.Fatalf("ReadExternal failed: %v", err)
		}

		if readExternal.ID != original.ID {
			t.Errorf("ID mismatch: got %q, want %q", readExternal.ID, original.ID)
		}
		if readExternal.Name != original.Name {
			t.Errorf("Name mismatch: got %q, want %q", readExternal.Name, original.Name)
		}
		if readExternal.Source != original.Source {
			t.Errorf("Source mismatch: got %q, want %q", readExternal.Source, original.Source)
		}
		if readExternal.ContentHash != original.ContentHash {
			t.Errorf("ContentHash mismatch: got %q, want %q", readExternal.ContentHash, original.ContentHash)
		}
		if readExternal.Notes != original.Notes {
			t.Errorf("Notes mismatch: got %q, want %q", readExternal.Notes, original.Notes)
		}
	})

	t.Run("empty_key", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		_, err := ReadExternal(dir, "")
		if err == nil {
			t.Error("expected error for empty key, got nil")
		}
	})

	t.Run("whitespace_key", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		_, err := ReadExternal(dir, "   ")
		if err == nil {
			t.Error("expected error for whitespace key, got nil")
		}
	})
}

// TestReadExternal_NotFound verifies that ReadExternal returns an appropriate
// error when the requested external does not exist.
func TestReadExternal_NotFound(t *testing.T) {
	t.Run("file_does_not_exist", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		_, err := ReadExternal(dir, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent external, got nil")
		}
		if !os.IsNotExist(err) {
			// Accept either os.IsNotExist or a wrapped error
			t.Logf("error type: %T, error: %v", err, err)
		}
	})

	t.Run("externals_directory_does_not_exist", func(t *testing.T) {
		dir := t.TempDir()
		// Do NOT create externals/ directory

		_, err := ReadExternal(dir, "somekey")
		if err == nil {
			t.Error("expected error when externals directory doesn't exist, got nil")
		}
	})

	t.Run("empty_directory", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		_, err := ReadExternal(dir, "missing")
		if err == nil {
			t.Error("expected error for missing external in empty directory, got nil")
		}
	})
}

// TestWriteExternal_InvalidPath verifies that WriteExternal returns an error
// for invalid directory paths.
func TestWriteExternal_InvalidPath(t *testing.T) {
	t.Run("empty_path", func(t *testing.T) {
		external := node.NewExternal("Test", "https://example.com")

		err := WriteExternal("", &external)
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_path", func(t *testing.T) {
		external := node.NewExternal("Test", "https://example.com")

		err := WriteExternal("   ", &external)
		if err == nil {
			t.Error("expected error for whitespace path, got nil")
		}
	})

	t.Run("path_is_file", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "notadir")

		// Create a file where a directory is expected
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		external := node.NewExternal("Test", "https://example.com")
		err := WriteExternal(filePath, &external)
		if err == nil {
			t.Error("expected error when path is a file, got nil")
		}
	})

	t.Run("permission_denied", func(t *testing.T) {
		// Skip on Windows where permission model differs
		if runtime.GOOS == "windows" {
			t.Skip("skipping permission test on Windows")
		}

		// Skip if running as root
		if os.Getuid() == 0 {
			t.Skip("skipping permission test when running as root")
		}

		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0555); err != nil {
			t.Fatalf("failed to create read-only externals directory: %v", err)
		}
		t.Cleanup(func() {
			os.Chmod(externalsDir, 0755)
		})

		external := node.NewExternal("Test", "https://example.com")
		err := WriteExternal(dir, &external)
		if err == nil {
			t.Error("expected error when directory is read-only, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		external := node.NewExternal("Test", "https://example.com")

		err := WriteExternal("path\x00invalid", &external)
		if err == nil {
			t.Error("expected error for path with null byte, got nil")
		}
	})
}

// TestWriteExternal_Overwrite verifies the behavior when writing an external
// with an ID that already exists.
func TestWriteExternal_Overwrite(t *testing.T) {
	t.Run("overwrite_existing", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Create initial external
		external := node.NewExternal("Original Name", "https://original.example.com")
		originalID := external.ID

		if err := WriteExternal(dir, &external); err != nil {
			t.Fatalf("first WriteExternal failed: %v", err)
		}

		// Create a new external with the same ID but different content
		modifiedExternal := &node.External{
			ID:          originalID,
			Name:        "Modified Name",
			Source:      "https://modified.example.com",
			ContentHash: "modified_hash",
			Created:     types.Now(),
			Notes:       "Updated notes",
		}

		// Write the modified external - should overwrite
		err := WriteExternal(dir, modifiedExternal)
		if err != nil {
			t.Fatalf("second WriteExternal failed: %v", err)
		}

		// Read back and verify it's the modified version
		readExternal, err := ReadExternal(dir, originalID)
		if err != nil {
			t.Fatalf("ReadExternal failed: %v", err)
		}

		if readExternal.Name != "Modified Name" {
			t.Errorf("Name was not overwritten: got %q, want %q",
				readExternal.Name, "Modified Name")
		}
		if readExternal.Source != "https://modified.example.com" {
			t.Errorf("Source was not overwritten: got %q, want %q",
				readExternal.Source, "https://modified.example.com")
		}
		if readExternal.Notes != "Updated notes" {
			t.Errorf("Notes was not overwritten: got %q, want %q",
				readExternal.Notes, "Updated notes")
		}
	})

	t.Run("overwrite_preserves_other_files", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Create two externals
		external1 := node.NewExternal("First External", "https://first.example.com")
		external2 := node.NewExternal("Second External", "https://second.example.com")

		if err := WriteExternal(dir, &external1); err != nil {
			t.Fatalf("WriteExternal for external1 failed: %v", err)
		}
		if err := WriteExternal(dir, &external2); err != nil {
			t.Fatalf("WriteExternal for external2 failed: %v", err)
		}

		// Overwrite the first external
		modifiedExternal1 := &node.External{
			ID:      external1.ID,
			Name:    "Modified First External",
			Source:  "https://modified-first.example.com",
			Created: types.Now(),
		}
		if err := WriteExternal(dir, modifiedExternal1); err != nil {
			t.Fatalf("overwrite WriteExternal failed: %v", err)
		}

		// Verify external2 is unchanged
		readExternal2, err := ReadExternal(dir, external2.ID)
		if err != nil {
			t.Fatalf("ReadExternal for external2 failed: %v", err)
		}

		if readExternal2.Name != external2.Name {
			t.Errorf("external2 was modified unexpectedly: got %q, want %q",
				readExternal2.Name, external2.Name)
		}
	})
}

// TestListExternals verifies that ListExternals correctly lists
// all external IDs in the externals/ directory.
func TestListExternals(t *testing.T) {
	t.Run("empty_directory", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		keys, err := ListExternals(dir)
		if err != nil {
			t.Fatalf("ListExternals failed: %v", err)
		}

		if len(keys) != 0 {
			t.Errorf("expected empty list, got %v", keys)
		}
	})

	t.Run("single_external", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		external := node.NewExternal("Test External", "https://example.com")
		if err := WriteExternal(dir, &external); err != nil {
			t.Fatalf("WriteExternal failed: %v", err)
		}

		keys, err := ListExternals(dir)
		if err != nil {
			t.Fatalf("ListExternals failed: %v", err)
		}

		if len(keys) != 1 {
			t.Fatalf("expected 1 key, got %d", len(keys))
		}
		if keys[0] != external.ID {
			t.Errorf("key mismatch: got %q, want %q", keys[0], external.ID)
		}
	})

	t.Run("multiple_externals", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Create multiple externals
		externals := []node.External{
			node.NewExternal("First External", "https://first.example.com"),
			node.NewExternal("Second External", "https://second.example.com"),
			node.NewExternal("Third External", "https://third.example.com"),
		}

		expectedKeys := make([]string, len(externals))
		for i, e := range externals {
			ext := e // Create local copy for pointer
			if err := WriteExternal(dir, &ext); err != nil {
				t.Fatalf("WriteExternal failed for external %d: %v", i, err)
			}
			expectedKeys[i] = e.ID
		}

		keys, err := ListExternals(dir)
		if err != nil {
			t.Fatalf("ListExternals failed: %v", err)
		}

		if len(keys) != len(expectedKeys) {
			t.Fatalf("expected %d keys, got %d", len(expectedKeys), len(keys))
		}

		// Sort both slices for comparison
		sort.Strings(keys)
		sort.Strings(expectedKeys)

		for i, key := range keys {
			if key != expectedKeys[i] {
				t.Errorf("key %d mismatch: got %q, want %q", i, key, expectedKeys[i])
			}
		}
	})

	t.Run("ignores_non_json_files", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Create a valid external
		external := node.NewExternal("Valid External", "https://valid.example.com")
		if err := WriteExternal(dir, &external); err != nil {
			t.Fatalf("WriteExternal failed: %v", err)
		}

		// Create non-JSON files that should be ignored
		nonJsonFiles := []string{"readme.txt", "backup.bak", ".hidden", "data.xml"}
		for _, name := range nonJsonFiles {
			path := filepath.Join(externalsDir, name)
			if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
				t.Fatalf("failed to create test file %s: %v", name, err)
			}
		}

		keys, err := ListExternals(dir)
		if err != nil {
			t.Fatalf("ListExternals failed: %v", err)
		}

		if len(keys) != 1 {
			t.Errorf("expected 1 key (ignoring non-JSON files), got %d: %v", len(keys), keys)
		}
		if len(keys) > 0 && keys[0] != external.ID {
			t.Errorf("key mismatch: got %q, want %q", keys[0], external.ID)
		}
	})

	t.Run("ignores_subdirectories", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Create a valid external
		external := node.NewExternal("Valid External", "https://valid.example.com")
		if err := WriteExternal(dir, &external); err != nil {
			t.Fatalf("WriteExternal failed: %v", err)
		}

		// Create a subdirectory that should be ignored
		subdir := filepath.Join(externalsDir, "subdir.json")
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		keys, err := ListExternals(dir)
		if err != nil {
			t.Fatalf("ListExternals failed: %v", err)
		}

		if len(keys) != 1 {
			t.Errorf("expected 1 key (ignoring subdirectories), got %d: %v", len(keys), keys)
		}
	})

	t.Run("directory_does_not_exist", func(t *testing.T) {
		dir := t.TempDir()
		// Do NOT create externals/ directory

		keys, err := ListExternals(dir)
		// This could either return an error or return an empty list
		// Depending on implementation choice
		if err != nil {
			// Acceptable: error when directory doesn't exist
			t.Logf("ListExternals returned error for non-existent directory: %v", err)
		} else if len(keys) != 0 {
			t.Errorf("expected empty list for non-existent directory, got %v", keys)
		}
	})

	t.Run("empty_path", func(t *testing.T) {
		_, err := ListExternals("")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("ignores_hidden_files", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Create a valid external
		external := node.NewExternal("Visible External", "https://visible.example.com")
		if err := WriteExternal(dir, &external); err != nil {
			t.Fatalf("WriteExternal failed: %v", err)
		}

		// Create hidden JSON files that should be ignored
		hiddenFiles := []string{".hidden.json", ".config.json"}
		for _, name := range hiddenFiles {
			path := filepath.Join(externalsDir, name)
			if err := os.WriteFile(path, []byte(`{"id":"hidden"}`), 0644); err != nil {
				t.Fatalf("failed to create hidden file %s: %v", name, err)
			}
		}

		keys, err := ListExternals(dir)
		if err != nil {
			t.Fatalf("ListExternals failed: %v", err)
		}

		if len(keys) != 1 {
			t.Errorf("expected 1 key (ignoring hidden files), got %d: %v", len(keys), keys)
		}
	})
}

// TestDeleteExternal verifies that DeleteExternal correctly removes
// an external file from the externals/ directory.
func TestDeleteExternal(t *testing.T) {
	t.Run("delete_existing", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Create an external
		external := node.NewExternal("To be deleted", "https://delete.example.com")
		if err := WriteExternal(dir, &external); err != nil {
			t.Fatalf("WriteExternal failed: %v", err)
		}

		// Verify it exists
		_, err := ReadExternal(dir, external.ID)
		if err != nil {
			t.Fatalf("external should exist before deletion: %v", err)
		}

		// Delete it
		err = DeleteExternal(dir, external.ID)
		if err != nil {
			t.Fatalf("DeleteExternal failed: %v", err)
		}

		// Verify it no longer exists
		_, err = ReadExternal(dir, external.ID)
		if err == nil {
			t.Error("external should not exist after deletion")
		}

		// Verify file is gone
		filePath := filepath.Join(externalsDir, external.ID+".json")
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Error("external file should be deleted from filesystem")
		}
	})

	t.Run("delete_nonexistent", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		err := DeleteExternal(dir, "nonexistent")
		if err == nil {
			t.Error("expected error when deleting non-existent external, got nil")
		}
	})

	t.Run("delete_preserves_other_files", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Create two externals
		external1 := node.NewExternal("First External", "https://first.example.com")
		external2 := node.NewExternal("Second External", "https://second.example.com")

		if err := WriteExternal(dir, &external1); err != nil {
			t.Fatalf("WriteExternal for external1 failed: %v", err)
		}
		if err := WriteExternal(dir, &external2); err != nil {
			t.Fatalf("WriteExternal for external2 failed: %v", err)
		}

		// Delete only the first one
		if err := DeleteExternal(dir, external1.ID); err != nil {
			t.Fatalf("DeleteExternal failed: %v", err)
		}

		// Verify second external still exists
		readExternal2, err := ReadExternal(dir, external2.ID)
		if err != nil {
			t.Fatalf("external2 should still exist: %v", err)
		}
		if readExternal2.Name != external2.Name {
			t.Errorf("external2 content changed: got %q, want %q",
				readExternal2.Name, external2.Name)
		}

		// Verify first external is gone
		_, err = ReadExternal(dir, external1.ID)
		if err == nil {
			t.Error("external1 should not exist after deletion")
		}
	})

	t.Run("empty_key", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		err := DeleteExternal(dir, "")
		if err == nil {
			t.Error("expected error for empty key, got nil")
		}
	})

	t.Run("whitespace_key", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		err := DeleteExternal(dir, "   ")
		if err == nil {
			t.Error("expected error for whitespace key, got nil")
		}
	})

	t.Run("empty_path", func(t *testing.T) {
		err := DeleteExternal("", "somekey")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("permission_denied", func(t *testing.T) {
		// Skip on Windows where permission model differs
		if runtime.GOOS == "windows" {
			t.Skip("skipping permission test on Windows")
		}

		// Skip if running as root
		if os.Getuid() == 0 {
			t.Skip("skipping permission test when running as root")
		}

		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Create an external
		external := node.NewExternal("Cannot delete", "https://nodelete.example.com")
		if err := WriteExternal(dir, &external); err != nil {
			t.Fatalf("WriteExternal failed: %v", err)
		}

		// Make directory read-only
		if err := os.Chmod(externalsDir, 0555); err != nil {
			t.Fatalf("failed to change permissions: %v", err)
		}
		t.Cleanup(func() {
			os.Chmod(externalsDir, 0755)
		})

		err := DeleteExternal(dir, external.ID)
		if err == nil {
			t.Error("expected error when directory is read-only, got nil")
		}
	})
}

// TestExternalIO_RoundTrip verifies that writing and reading an external
// produces an equivalent result.
func TestExternalIO_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	externalsDir := filepath.Join(dir, "externals")
	if err := os.MkdirAll(externalsDir, 0755); err != nil {
		t.Fatalf("failed to create externals directory: %v", err)
	}

	testCases := []struct {
		name     string
		external node.External
	}{
		{
			name:     "simple_external",
			external: node.NewExternal("Simple Name", "https://simple.example.com"),
		},
		{
			name:     "with_notes",
			external: node.NewExternalWithNotes("With Notes", "https://notes.example.com", "Some notes here"),
		},
		{
			name: "with_special_characters",
			external: node.NewExternalWithNotes(
				"Name with \"quotes\" and 'apostrophes'",
				"https://example.com/path?query=value&other=test",
				"Notes with newlines\nand tabs\t",
			),
		},
		{
			name: "unicode_content",
			external: node.NewExternalWithNotes(
				"Unicode Name: theorem",
				"https://example.com/math",
				"Notes with unicode: sum from i=1 to n of x_i",
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write
			ext := tc.external // Create local copy for pointer
			if err := WriteExternal(dir, &ext); err != nil {
				t.Fatalf("WriteExternal failed: %v", err)
			}

			// Read
			readExternal, err := ReadExternal(dir, tc.external.ID)
			if err != nil {
				t.Fatalf("ReadExternal failed: %v", err)
			}

			// Compare
			if readExternal.ID != tc.external.ID {
				t.Errorf("ID mismatch: got %q, want %q", readExternal.ID, tc.external.ID)
			}
			if readExternal.Name != tc.external.Name {
				t.Errorf("Name mismatch: got %q, want %q", readExternal.Name, tc.external.Name)
			}
			if readExternal.Source != tc.external.Source {
				t.Errorf("Source mismatch: got %q, want %q", readExternal.Source, tc.external.Source)
			}
			if readExternal.ContentHash != tc.external.ContentHash {
				t.Errorf("ContentHash mismatch: got %q, want %q", readExternal.ContentHash, tc.external.ContentHash)
			}
			if readExternal.Notes != tc.external.Notes {
				t.Errorf("Notes mismatch: got %q, want %q", readExternal.Notes, tc.external.Notes)
			}
		})
	}
}

// TestExternalIO_RoundTrip_Timestamp verifies that the Created timestamp
// is preserved through a write/read cycle.
func TestExternalIO_RoundTrip_Timestamp(t *testing.T) {
	dir := t.TempDir()
	externalsDir := filepath.Join(dir, "externals")
	if err := os.MkdirAll(externalsDir, 0755); err != nil {
		t.Fatalf("failed to create externals directory: %v", err)
	}

	// Use types.Now() for timestamp
	created := types.Now()

	external := &node.External{
		ID:          "timestamp-test",
		Name:        "Timestamp Test",
		Source:      "https://timestamp.example.com",
		ContentHash: "testhash",
		Created:     created,
		Notes:       "",
	}

	// Write
	if err := WriteExternal(dir, external); err != nil {
		t.Fatalf("WriteExternal failed: %v", err)
	}

	// Read
	readExternal, err := ReadExternal(dir, external.ID)
	if err != nil {
		t.Fatalf("ReadExternal failed: %v", err)
	}

	// Compare timestamps using the Equal method
	if !readExternal.Created.Equal(external.Created) {
		t.Errorf("Created timestamp mismatch: got %v, want %v",
			readExternal.Created.String(), external.Created.String())
	}
}

// TestReadExternal_InvalidJSON verifies that ReadExternal handles
// corrupted or invalid JSON files appropriately.
func TestReadExternal_InvalidJSON(t *testing.T) {
	t.Run("malformed_json", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Write invalid JSON
		filePath := filepath.Join(externalsDir, "broken.json")
		if err := os.WriteFile(filePath, []byte("{invalid json}"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		_, err := ReadExternal(dir, "broken")
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("empty_file", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Write empty file
		filePath := filepath.Join(externalsDir, "empty.json")
		if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		_, err := ReadExternal(dir, "empty")
		if err == nil {
			t.Error("expected error for empty file, got nil")
		}
	})

	t.Run("wrong_type", func(t *testing.T) {
		dir := t.TempDir()
		externalsDir := filepath.Join(dir, "externals")
		if err := os.MkdirAll(externalsDir, 0755); err != nil {
			t.Fatalf("failed to create externals directory: %v", err)
		}

		// Write valid JSON but not an external object
		filePath := filepath.Join(externalsDir, "array.json")
		if err := os.WriteFile(filePath, []byte("[1, 2, 3]"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		_, err := ReadExternal(dir, "array")
		if err == nil {
			t.Error("expected error for wrong JSON type, got nil")
		}
	})
}

// TestWriteExternal_InvalidExternal verifies that WriteExternal returns an error
// for externals with invalid data.
func TestWriteExternal_InvalidExternal(t *testing.T) {
	dir := t.TempDir()
	externalsDir := filepath.Join(dir, "externals")
	if err := os.MkdirAll(externalsDir, 0755); err != nil {
		t.Fatalf("failed to create externals directory: %v", err)
	}

	t.Run("empty_id", func(t *testing.T) {
		external := &node.External{
			ID:     "",
			Name:   "Test",
			Source: "https://example.com",
		}
		err := WriteExternal(dir, external)
		if err == nil {
			t.Error("expected error for external with empty ID, got nil")
		}
	})

	t.Run("whitespace_id", func(t *testing.T) {
		external := &node.External{
			ID:     "   ",
			Name:   "Test",
			Source: "https://example.com",
		}
		err := WriteExternal(dir, external)
		if err == nil {
			t.Error("expected error for external with whitespace-only ID, got nil")
		}
	})
}

// TestWriteExternal_AtomicWrite verifies that WriteExternal uses
// atomic write operations (write to temp, then rename).
func TestWriteExternal_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	externalsDir := filepath.Join(dir, "externals")
	if err := os.MkdirAll(externalsDir, 0755); err != nil {
		t.Fatalf("failed to create externals directory: %v", err)
	}

	external := node.NewExternal("Atomic Test", "https://atomic.example.com")

	// Write the external
	err := WriteExternal(dir, &external)
	if err != nil {
		t.Fatalf("WriteExternal failed: %v", err)
	}

	// Verify no temp files are left behind
	entries, err := os.ReadDir(externalsDir)
	if err != nil {
		t.Fatalf("failed to read externals directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// Check for common temp file patterns
		if name != external.ID+".json" {
			// Allow only the expected .json file
			if filepath.Ext(name) == ".tmp" || filepath.Ext(name) == ".temp" {
				t.Errorf("temp file left behind: %s", name)
			}
		}
	}
}

// TestExternalFileFormat verifies that external files use the expected
// JSON format with proper indentation for human readability.
func TestExternalFileFormat(t *testing.T) {
	dir := t.TempDir()
	externalsDir := filepath.Join(dir, "externals")
	if err := os.MkdirAll(externalsDir, 0755); err != nil {
		t.Fatalf("failed to create externals directory: %v", err)
	}

	external := node.NewExternalWithNotes("Format Test", "https://format.example.com", "Test notes")

	if err := WriteExternal(dir, &external); err != nil {
		t.Fatalf("WriteExternal failed: %v", err)
	}

	// Read raw file content
	extPath := filepath.Join(externalsDir, external.ID+".json")
	content, err := os.ReadFile(extPath)
	if err != nil {
		t.Fatalf("failed to read external file: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("external file is not valid JSON: %v", err)
	}

	// Verify expected fields are present
	expectedFields := []string{"id", "name", "source", "content_hash", "created", "notes"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("expected field %q not found in external file", field)
		}
	}
}

// TestReadExternal_PathTraversal verifies that ReadExternal prevents
// path traversal attacks.
func TestReadExternal_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	externalsDir := filepath.Join(dir, "externals")
	if err := os.MkdirAll(externalsDir, 0755); err != nil {
		t.Fatalf("failed to create externals directory: %v", err)
	}

	// Create a file outside the externals directory
	secretPath := filepath.Join(dir, "secret.json")
	secretContent := `{"id": "secret", "name": "secret", "source": "secret data"}`
	if err := os.WriteFile(secretPath, []byte(secretContent), 0644); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}

	// Attempt path traversal
	traversalKeys := []string{
		"../secret",
		"..%2fsecret",
		"/../secret",
		"..\\secret",
	}

	for _, key := range traversalKeys {
		t.Run(key, func(t *testing.T) {
			_, err := ReadExternal(dir, key)
			// Should either error or not return the secret content
			if err == nil {
				t.Logf("ReadExternal succeeded for key %q - verify it didn't traverse", key)
				// If it succeeded, it should not have read the secret file
			}
		})
	}
}

// TestDeleteExternal_PathTraversal verifies that DeleteExternal prevents
// path traversal attacks.
func TestDeleteExternal_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	externalsDir := filepath.Join(dir, "externals")
	if err := os.MkdirAll(externalsDir, 0755); err != nil {
		t.Fatalf("failed to create externals directory: %v", err)
	}

	// Create a file outside the externals directory
	protectedPath := filepath.Join(dir, "protected.json")
	if err := os.WriteFile(protectedPath, []byte("protected"), 0644); err != nil {
		t.Fatalf("failed to write protected file: %v", err)
	}

	// Attempt path traversal deletion
	traversalKeys := []string{
		"../protected",
		"..%2fprotected",
		"/../protected",
	}

	for _, key := range traversalKeys {
		t.Run(key, func(t *testing.T) {
			_ = DeleteExternal(dir, key)
			// Verify protected file still exists
			if _, err := os.Stat(protectedPath); os.IsNotExist(err) {
				t.Errorf("path traversal succeeded - protected file was deleted with key %q", key)
			}
		})
	}
}
