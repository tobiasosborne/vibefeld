//go:build integration
// +build integration

// These tests define expected behavior for WriteAssumption, ReadAssumption,
// ListAssumptions, and DeleteAssumption.
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
)

// TestWriteAssumption verifies that WriteAssumption correctly writes
// an assumption to the assumptions/ subdirectory as JSON.
func TestWriteAssumption(t *testing.T) {
	t.Run("basic_write", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		assumption := node.NewAssumption("All natural numbers are positive")

		err := WriteAssumption(dir, assumption)
		if err != nil {
			t.Fatalf("WriteAssumption failed: %v", err)
		}

		// Verify file was created
		expectedPath := filepath.Join(assumptionsDir, assumption.ID+".json")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Fatalf("expected assumption file to exist at %s", expectedPath)
		}

		// Verify content is valid JSON and matches the assumption
		content, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatalf("failed to read assumption file: %v", err)
		}

		var readAssumption node.Assumption
		if err := json.Unmarshal(content, &readAssumption); err != nil {
			t.Fatalf("assumption file is not valid JSON: %v", err)
		}

		if readAssumption.ID != assumption.ID {
			t.Errorf("ID mismatch: got %q, want %q", readAssumption.ID, assumption.ID)
		}
		if readAssumption.Statement != assumption.Statement {
			t.Errorf("Statement mismatch: got %q, want %q", readAssumption.Statement, assumption.Statement)
		}
		if readAssumption.ContentHash != assumption.ContentHash {
			t.Errorf("ContentHash mismatch: got %q, want %q", readAssumption.ContentHash, assumption.ContentHash)
		}
	})

	t.Run("with_justification", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		assumption := node.NewAssumptionWithJustification(
			"The set S is non-empty",
			"This follows from the problem statement",
		)

		err := WriteAssumption(dir, assumption)
		if err != nil {
			t.Fatalf("WriteAssumption failed: %v", err)
		}

		// Read back and verify justification is preserved
		expectedPath := filepath.Join(assumptionsDir, assumption.ID+".json")
		content, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatalf("failed to read assumption file: %v", err)
		}

		var readAssumption node.Assumption
		if err := json.Unmarshal(content, &readAssumption); err != nil {
			t.Fatalf("assumption file is not valid JSON: %v", err)
		}

		if readAssumption.Justification != assumption.Justification {
			t.Errorf("Justification mismatch: got %q, want %q", readAssumption.Justification, assumption.Justification)
		}
	})

	t.Run("creates_assumptions_subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		// Do NOT pre-create assumptions/ directory

		assumption := node.NewAssumption("Test assumption")

		err := WriteAssumption(dir, assumption)
		if err != nil {
			t.Fatalf("WriteAssumption failed: %v", err)
		}

		// Verify assumptions directory was created
		assumptionsDir := filepath.Join(dir, "assumptions")
		info, err := os.Stat(assumptionsDir)
		if os.IsNotExist(err) {
			t.Fatal("expected assumptions directory to be created")
		}
		if !info.IsDir() {
			t.Fatal("expected assumptions to be a directory")
		}
	})

	t.Run("nil_assumption", func(t *testing.T) {
		dir := t.TempDir()

		err := WriteAssumption(dir, nil)
		if err == nil {
			t.Error("expected error for nil assumption, got nil")
		}
	})
}

// TestReadAssumption verifies that ReadAssumption correctly reads
// an assumption from the assumptions/ subdirectory.
func TestReadAssumption(t *testing.T) {
	t.Run("basic_read", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Create a test assumption file directly
		assumption := &node.Assumption{
			ID:          "test123",
			Statement:   "The function f is continuous",
			ContentHash: "abc123",
		}
		content, err := json.Marshal(assumption)
		if err != nil {
			t.Fatalf("failed to marshal assumption: %v", err)
		}

		filePath := filepath.Join(assumptionsDir, "test123.json")
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		// Read the assumption
		readAssumption, err := ReadAssumption(dir, "test123")
		if err != nil {
			t.Fatalf("ReadAssumption failed: %v", err)
		}

		if readAssumption.ID != assumption.ID {
			t.Errorf("ID mismatch: got %q, want %q", readAssumption.ID, assumption.ID)
		}
		if readAssumption.Statement != assumption.Statement {
			t.Errorf("Statement mismatch: got %q, want %q", readAssumption.Statement, assumption.Statement)
		}
		if readAssumption.ContentHash != assumption.ContentHash {
			t.Errorf("ContentHash mismatch: got %q, want %q", readAssumption.ContentHash, assumption.ContentHash)
		}
	})

	t.Run("with_all_fields", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Use WriteAssumption to create, then read back
		original := node.NewAssumptionWithJustification(
			"All primes greater than 2 are odd",
			"Well-known mathematical fact",
		)

		if err := WriteAssumption(dir, original); err != nil {
			t.Fatalf("WriteAssumption failed: %v", err)
		}

		readAssumption, err := ReadAssumption(dir, original.ID)
		if err != nil {
			t.Fatalf("ReadAssumption failed: %v", err)
		}

		if readAssumption.ID != original.ID {
			t.Errorf("ID mismatch: got %q, want %q", readAssumption.ID, original.ID)
		}
		if readAssumption.Statement != original.Statement {
			t.Errorf("Statement mismatch: got %q, want %q", readAssumption.Statement, original.Statement)
		}
		if readAssumption.ContentHash != original.ContentHash {
			t.Errorf("ContentHash mismatch: got %q, want %q", readAssumption.ContentHash, original.ContentHash)
		}
		if readAssumption.Justification != original.Justification {
			t.Errorf("Justification mismatch: got %q, want %q", readAssumption.Justification, original.Justification)
		}
	})

	t.Run("empty_key", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		_, err := ReadAssumption(dir, "")
		if err == nil {
			t.Error("expected error for empty key, got nil")
		}
	})

	t.Run("whitespace_key", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		_, err := ReadAssumption(dir, "   ")
		if err == nil {
			t.Error("expected error for whitespace key, got nil")
		}
	})
}

// TestReadAssumption_NotFound verifies that ReadAssumption returns an appropriate
// error when the requested assumption does not exist.
func TestReadAssumption_NotFound(t *testing.T) {
	t.Run("file_does_not_exist", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		_, err := ReadAssumption(dir, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent assumption, got nil")
		}
		if !os.IsNotExist(err) {
			// Accept either os.IsNotExist or a wrapped error
			t.Logf("error type: %T, error: %v", err, err)
		}
	})

	t.Run("assumptions_directory_does_not_exist", func(t *testing.T) {
		dir := t.TempDir()
		// Do NOT create assumptions/ directory

		_, err := ReadAssumption(dir, "somekey")
		if err == nil {
			t.Error("expected error when assumptions directory doesn't exist, got nil")
		}
	})

	t.Run("empty_directory", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		_, err := ReadAssumption(dir, "missing")
		if err == nil {
			t.Error("expected error for missing assumption in empty directory, got nil")
		}
	})
}

// TestWriteAssumption_InvalidPath verifies that WriteAssumption returns an error
// for invalid directory paths.
func TestWriteAssumption_InvalidPath(t *testing.T) {
	t.Run("empty_path", func(t *testing.T) {
		assumption := node.NewAssumption("Test assumption")

		err := WriteAssumption("", assumption)
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_path", func(t *testing.T) {
		assumption := node.NewAssumption("Test assumption")

		err := WriteAssumption("   ", assumption)
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

		assumption := node.NewAssumption("Test assumption")
		err := WriteAssumption(filePath, assumption)
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
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0555); err != nil {
			t.Fatalf("failed to create read-only assumptions directory: %v", err)
		}
		t.Cleanup(func() {
			os.Chmod(assumptionsDir, 0755)
		})

		assumption := node.NewAssumption("Test assumption")
		err := WriteAssumption(dir, assumption)
		if err == nil {
			t.Error("expected error when directory is read-only, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		assumption := node.NewAssumption("Test assumption")

		err := WriteAssumption("path\x00invalid", assumption)
		if err == nil {
			t.Error("expected error for path with null byte, got nil")
		}
	})
}

// TestWriteAssumption_Overwrite verifies the behavior when writing an assumption
// with an ID that already exists.
func TestWriteAssumption_Overwrite(t *testing.T) {
	t.Run("overwrite_existing", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Create initial assumption
		assumption := node.NewAssumption("Original statement")
		originalID := assumption.ID

		if err := WriteAssumption(dir, assumption); err != nil {
			t.Fatalf("first WriteAssumption failed: %v", err)
		}

		// Create a new assumption with the same ID but different content
		modifiedAssumption := &node.Assumption{
			ID:            originalID,
			Statement:     "Modified statement",
			ContentHash:   "modified_hash",
			Justification: "Updated justification",
		}

		// Write the modified assumption - should overwrite
		err := WriteAssumption(dir, modifiedAssumption)
		if err != nil {
			t.Fatalf("second WriteAssumption failed: %v", err)
		}

		// Read back and verify it's the modified version
		readAssumption, err := ReadAssumption(dir, originalID)
		if err != nil {
			t.Fatalf("ReadAssumption failed: %v", err)
		}

		if readAssumption.Statement != "Modified statement" {
			t.Errorf("Statement was not overwritten: got %q, want %q",
				readAssumption.Statement, "Modified statement")
		}
		if readAssumption.Justification != "Updated justification" {
			t.Errorf("Justification was not overwritten: got %q, want %q",
				readAssumption.Justification, "Updated justification")
		}
	})

	t.Run("overwrite_preserves_other_files", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Create two assumptions
		assumption1 := node.NewAssumption("First assumption")
		assumption2 := node.NewAssumption("Second assumption")

		if err := WriteAssumption(dir, assumption1); err != nil {
			t.Fatalf("WriteAssumption for assumption1 failed: %v", err)
		}
		if err := WriteAssumption(dir, assumption2); err != nil {
			t.Fatalf("WriteAssumption for assumption2 failed: %v", err)
		}

		// Overwrite the first assumption
		modifiedAssumption1 := &node.Assumption{
			ID:        assumption1.ID,
			Statement: "Modified first assumption",
		}
		if err := WriteAssumption(dir, modifiedAssumption1); err != nil {
			t.Fatalf("overwrite WriteAssumption failed: %v", err)
		}

		// Verify assumption2 is unchanged
		readAssumption2, err := ReadAssumption(dir, assumption2.ID)
		if err != nil {
			t.Fatalf("ReadAssumption for assumption2 failed: %v", err)
		}

		if readAssumption2.Statement != assumption2.Statement {
			t.Errorf("assumption2 was modified unexpectedly: got %q, want %q",
				readAssumption2.Statement, assumption2.Statement)
		}
	})
}

// TestListAssumptions verifies that ListAssumptions correctly lists
// all assumption IDs in the assumptions/ directory.
func TestListAssumptions(t *testing.T) {
	t.Run("empty_directory", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		keys, err := ListAssumptions(dir)
		if err != nil {
			t.Fatalf("ListAssumptions failed: %v", err)
		}

		if len(keys) != 0 {
			t.Errorf("expected empty list, got %v", keys)
		}
	})

	t.Run("single_assumption", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		assumption := node.NewAssumption("Test assumption")
		if err := WriteAssumption(dir, assumption); err != nil {
			t.Fatalf("WriteAssumption failed: %v", err)
		}

		keys, err := ListAssumptions(dir)
		if err != nil {
			t.Fatalf("ListAssumptions failed: %v", err)
		}

		if len(keys) != 1 {
			t.Fatalf("expected 1 key, got %d", len(keys))
		}
		if keys[0] != assumption.ID {
			t.Errorf("key mismatch: got %q, want %q", keys[0], assumption.ID)
		}
	})

	t.Run("multiple_assumptions", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Create multiple assumptions
		assumptions := []*node.Assumption{
			node.NewAssumption("First assumption"),
			node.NewAssumption("Second assumption"),
			node.NewAssumption("Third assumption"),
		}

		expectedKeys := make([]string, len(assumptions))
		for i, a := range assumptions {
			if err := WriteAssumption(dir, a); err != nil {
				t.Fatalf("WriteAssumption failed for assumption %d: %v", i, err)
			}
			expectedKeys[i] = a.ID
		}

		keys, err := ListAssumptions(dir)
		if err != nil {
			t.Fatalf("ListAssumptions failed: %v", err)
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
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Create a valid assumption
		assumption := node.NewAssumption("Valid assumption")
		if err := WriteAssumption(dir, assumption); err != nil {
			t.Fatalf("WriteAssumption failed: %v", err)
		}

		// Create non-JSON files that should be ignored
		nonJsonFiles := []string{"readme.txt", "backup.bak", ".hidden", "data.xml"}
		for _, name := range nonJsonFiles {
			path := filepath.Join(assumptionsDir, name)
			if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
				t.Fatalf("failed to create test file %s: %v", name, err)
			}
		}

		keys, err := ListAssumptions(dir)
		if err != nil {
			t.Fatalf("ListAssumptions failed: %v", err)
		}

		if len(keys) != 1 {
			t.Errorf("expected 1 key (ignoring non-JSON files), got %d: %v", len(keys), keys)
		}
		if len(keys) > 0 && keys[0] != assumption.ID {
			t.Errorf("key mismatch: got %q, want %q", keys[0], assumption.ID)
		}
	})

	t.Run("ignores_subdirectories", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Create a valid assumption
		assumption := node.NewAssumption("Valid assumption")
		if err := WriteAssumption(dir, assumption); err != nil {
			t.Fatalf("WriteAssumption failed: %v", err)
		}

		// Create a subdirectory that should be ignored
		subdir := filepath.Join(assumptionsDir, "subdir.json")
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		keys, err := ListAssumptions(dir)
		if err != nil {
			t.Fatalf("ListAssumptions failed: %v", err)
		}

		if len(keys) != 1 {
			t.Errorf("expected 1 key (ignoring subdirectories), got %d: %v", len(keys), keys)
		}
	})

	t.Run("directory_does_not_exist", func(t *testing.T) {
		dir := t.TempDir()
		// Do NOT create assumptions/ directory

		keys, err := ListAssumptions(dir)
		// This could either return an error or return an empty list
		// Depending on implementation choice
		if err != nil {
			// Acceptable: error when directory doesn't exist
			t.Logf("ListAssumptions returned error for non-existent directory: %v", err)
		} else if len(keys) != 0 {
			t.Errorf("expected empty list for non-existent directory, got %v", keys)
		}
	})

	t.Run("empty_path", func(t *testing.T) {
		_, err := ListAssumptions("")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})
}

// TestDeleteAssumption verifies that DeleteAssumption correctly removes
// an assumption file from the assumptions/ directory.
func TestDeleteAssumption(t *testing.T) {
	t.Run("delete_existing", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Create an assumption
		assumption := node.NewAssumption("To be deleted")
		if err := WriteAssumption(dir, assumption); err != nil {
			t.Fatalf("WriteAssumption failed: %v", err)
		}

		// Verify it exists
		_, err := ReadAssumption(dir, assumption.ID)
		if err != nil {
			t.Fatalf("assumption should exist before deletion: %v", err)
		}

		// Delete it
		err = DeleteAssumption(dir, assumption.ID)
		if err != nil {
			t.Fatalf("DeleteAssumption failed: %v", err)
		}

		// Verify it no longer exists
		_, err = ReadAssumption(dir, assumption.ID)
		if err == nil {
			t.Error("assumption should not exist after deletion")
		}

		// Verify file is gone
		filePath := filepath.Join(assumptionsDir, assumption.ID+".json")
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Error("assumption file should be deleted from filesystem")
		}
	})

	t.Run("delete_nonexistent", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		err := DeleteAssumption(dir, "nonexistent")
		if err == nil {
			t.Error("expected error when deleting non-existent assumption, got nil")
		}
	})

	t.Run("delete_preserves_other_files", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Create two assumptions
		assumption1 := node.NewAssumption("First assumption")
		assumption2 := node.NewAssumption("Second assumption")

		if err := WriteAssumption(dir, assumption1); err != nil {
			t.Fatalf("WriteAssumption for assumption1 failed: %v", err)
		}
		if err := WriteAssumption(dir, assumption2); err != nil {
			t.Fatalf("WriteAssumption for assumption2 failed: %v", err)
		}

		// Delete only the first one
		if err := DeleteAssumption(dir, assumption1.ID); err != nil {
			t.Fatalf("DeleteAssumption failed: %v", err)
		}

		// Verify second assumption still exists
		readAssumption2, err := ReadAssumption(dir, assumption2.ID)
		if err != nil {
			t.Fatalf("assumption2 should still exist: %v", err)
		}
		if readAssumption2.Statement != assumption2.Statement {
			t.Errorf("assumption2 content changed: got %q, want %q",
				readAssumption2.Statement, assumption2.Statement)
		}

		// Verify first assumption is gone
		_, err = ReadAssumption(dir, assumption1.ID)
		if err == nil {
			t.Error("assumption1 should not exist after deletion")
		}
	})

	t.Run("empty_key", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		err := DeleteAssumption(dir, "")
		if err == nil {
			t.Error("expected error for empty key, got nil")
		}
	})

	t.Run("whitespace_key", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		err := DeleteAssumption(dir, "   ")
		if err == nil {
			t.Error("expected error for whitespace key, got nil")
		}
	})

	t.Run("empty_path", func(t *testing.T) {
		err := DeleteAssumption("", "somekey")
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
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Create an assumption
		assumption := node.NewAssumption("Cannot delete")
		if err := WriteAssumption(dir, assumption); err != nil {
			t.Fatalf("WriteAssumption failed: %v", err)
		}

		// Make directory read-only
		if err := os.Chmod(assumptionsDir, 0555); err != nil {
			t.Fatalf("failed to change permissions: %v", err)
		}
		t.Cleanup(func() {
			os.Chmod(assumptionsDir, 0755)
		})

		err := DeleteAssumption(dir, assumption.ID)
		if err == nil {
			t.Error("expected error when directory is read-only, got nil")
		}
	})
}

// TestAssumptionIO_RoundTrip verifies that writing and reading an assumption
// produces an equivalent result.
func TestAssumptionIO_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	assumptionsDir := filepath.Join(dir, "assumptions")
	if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
		t.Fatalf("failed to create assumptions directory: %v", err)
	}

	testCases := []struct {
		name       string
		assumption *node.Assumption
	}{
		{
			name:       "simple_assumption",
			assumption: node.NewAssumption("Simple statement"),
		},
		{
			name:       "with_justification",
			assumption: node.NewAssumptionWithJustification("Statement with justification", "Because it's needed"),
		},
		{
			name: "with_special_characters",
			assumption: node.NewAssumption(
				"Statement with \"quotes\" and 'apostrophes' and newlines\nand tabs\t",
			),
		},
		{
			name: "unicode_statement",
			assumption: node.NewAssumption(
				"Statement with unicode: \u03c0 \u2260 0 and \u2200x \u2208 \u211d",
			),
		},
		{
			name:       "long_statement",
			assumption: node.NewAssumption(string(make([]byte, 10000))),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write
			if err := WriteAssumption(dir, tc.assumption); err != nil {
				t.Fatalf("WriteAssumption failed: %v", err)
			}

			// Read
			readAssumption, err := ReadAssumption(dir, tc.assumption.ID)
			if err != nil {
				t.Fatalf("ReadAssumption failed: %v", err)
			}

			// Compare
			if readAssumption.ID != tc.assumption.ID {
				t.Errorf("ID mismatch: got %q, want %q", readAssumption.ID, tc.assumption.ID)
			}
			if readAssumption.Statement != tc.assumption.Statement {
				t.Errorf("Statement mismatch: got %q, want %q", readAssumption.Statement, tc.assumption.Statement)
			}
			if readAssumption.ContentHash != tc.assumption.ContentHash {
				t.Errorf("ContentHash mismatch: got %q, want %q", readAssumption.ContentHash, tc.assumption.ContentHash)
			}
			if readAssumption.Justification != tc.assumption.Justification {
				t.Errorf("Justification mismatch: got %q, want %q", readAssumption.Justification, tc.assumption.Justification)
			}
		})
	}
}

// TestReadAssumption_InvalidJSON verifies that ReadAssumption handles
// corrupted or invalid JSON files appropriately.
func TestReadAssumption_InvalidJSON(t *testing.T) {
	t.Run("malformed_json", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Write invalid JSON
		filePath := filepath.Join(assumptionsDir, "broken.json")
		if err := os.WriteFile(filePath, []byte("{invalid json}"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		_, err := ReadAssumption(dir, "broken")
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("empty_file", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Write empty file
		filePath := filepath.Join(assumptionsDir, "empty.json")
		if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		_, err := ReadAssumption(dir, "empty")
		if err == nil {
			t.Error("expected error for empty file, got nil")
		}
	})

	t.Run("wrong_type", func(t *testing.T) {
		dir := t.TempDir()
		assumptionsDir := filepath.Join(dir, "assumptions")
		if err := os.MkdirAll(assumptionsDir, 0755); err != nil {
			t.Fatalf("failed to create assumptions directory: %v", err)
		}

		// Write valid JSON but not an assumption object
		filePath := filepath.Join(assumptionsDir, "array.json")
		if err := os.WriteFile(filePath, []byte("[1, 2, 3]"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		_, err := ReadAssumption(dir, "array")
		if err == nil {
			t.Error("expected error for wrong JSON type, got nil")
		}
	})
}
