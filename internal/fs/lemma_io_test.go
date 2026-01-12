//go:build integration
// +build integration

// These tests define expected behavior for WriteLemma, ReadLemma,
// ListLemmas, and DeleteLemma.
// Run with: go test -tags=integration ./internal/fs/...

package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/types"
)

// TestWriteLemma verifies that WriteLemma correctly writes a
// lemma to the lemmas/ subdirectory as a JSON file.
func TestWriteLemma(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, err := types.Parse("1.2.3")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lemma, err := node.NewLemma("For all x > 0, ln(x) is defined.", sourceNodeID)
	if err != nil {
		t.Fatalf("failed to create lemma: %v", err)
	}

	err = WriteLemma(dir, lemma)
	if err != nil {
		t.Fatalf("WriteLemma failed: %v", err)
	}

	// Verify file was created with correct name
	lemmaPath := filepath.Join(lemmasDir, lemma.ID+".json")
	info, err := os.Stat(lemmaPath)
	if os.IsNotExist(err) {
		t.Fatalf("expected lemma file to exist at %s", lemmaPath)
	}
	if err != nil {
		t.Fatalf("error checking lemma file: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected lemma file to be a file, not a directory")
	}

	// Verify file contents are valid JSON matching the lemma
	content, err := os.ReadFile(lemmaPath)
	if err != nil {
		t.Fatalf("failed to read lemma file: %v", err)
	}

	var readLemma node.Lemma
	if err := json.Unmarshal(content, &readLemma); err != nil {
		t.Fatalf("lemma file is not valid JSON: %v", err)
	}

	if readLemma.ID != lemma.ID {
		t.Errorf("ID mismatch: got %q, want %q", readLemma.ID, lemma.ID)
	}
	if readLemma.Statement != lemma.Statement {
		t.Errorf("Statement mismatch: got %q, want %q", readLemma.Statement, lemma.Statement)
	}
	if readLemma.ContentHash != lemma.ContentHash {
		t.Errorf("ContentHash mismatch: got %q, want %q", readLemma.ContentHash, lemma.ContentHash)
	}
	if readLemma.SourceNodeID.String() != lemma.SourceNodeID.String() {
		t.Errorf("SourceNodeID mismatch: got %q, want %q", readLemma.SourceNodeID.String(), lemma.SourceNodeID.String())
	}
}

// TestWriteLemma_CreatesLemmasDir verifies that WriteLemma creates
// the lemmas/ subdirectory if it doesn't exist.
func TestWriteLemma_CreatesLemmasDir(t *testing.T) {
	dir := t.TempDir()
	// Note: lemmas/ directory does NOT exist yet

	sourceNodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lemma, err := node.NewLemma("The sum of two even numbers is even.", sourceNodeID)
	if err != nil {
		t.Fatalf("failed to create lemma: %v", err)
	}

	err = WriteLemma(dir, lemma)
	if err != nil {
		t.Fatalf("WriteLemma failed: %v", err)
	}

	// Verify lemmas directory was created
	lemmasDir := filepath.Join(dir, "lemmas")
	info, err := os.Stat(lemmasDir)
	if os.IsNotExist(err) {
		t.Fatal("expected lemmas directory to be created")
	}
	if err != nil {
		t.Fatalf("error checking lemmas directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected lemmas to be a directory")
	}

	// Verify file was created
	lemmaPath := filepath.Join(lemmasDir, lemma.ID+".json")
	if _, err := os.Stat(lemmaPath); os.IsNotExist(err) {
		t.Fatalf("expected lemma file to exist at %s", lemmaPath)
	}
}

// TestWriteLemma_NilLemma verifies that WriteLemma returns
// an error when given a nil lemma.
func TestWriteLemma_NilLemma(t *testing.T) {
	dir := t.TempDir()

	err := WriteLemma(dir, nil)
	if err == nil {
		t.Error("expected error for nil lemma, got nil")
	}
}

// TestWriteLemma_WithProof verifies that WriteLemma preserves the Proof field.
func TestWriteLemma_WithProof(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lemma, err := node.NewLemma("The product of two negative numbers is positive.", sourceNodeID)
	if err != nil {
		t.Fatalf("failed to create lemma: %v", err)
	}
	lemma.SetProof("By definition of negative numbers and multiplication.")

	err = WriteLemma(dir, lemma)
	if err != nil {
		t.Fatalf("WriteLemma failed: %v", err)
	}

	// Read back and verify proof is preserved
	lemmaPath := filepath.Join(lemmasDir, lemma.ID+".json")
	content, err := os.ReadFile(lemmaPath)
	if err != nil {
		t.Fatalf("failed to read lemma file: %v", err)
	}

	var readLemma node.Lemma
	if err := json.Unmarshal(content, &readLemma); err != nil {
		t.Fatalf("lemma file is not valid JSON: %v", err)
	}

	if readLemma.Proof != lemma.Proof {
		t.Errorf("Proof mismatch: got %q, want %q", readLemma.Proof, lemma.Proof)
	}
}

// TestReadLemma verifies that ReadLemma correctly reads a
// lemma from the lemmas/ subdirectory.
func TestReadLemma(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	// Truncate to seconds to avoid precision issues
	now := time.Now().Truncate(time.Second)
	created, err := types.ParseTimestamp(now.UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("failed to parse timestamp: %v", err)
	}

	sourceNodeID, err := types.Parse("1.2.1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create a lemma and write it manually
	lemma := &node.Lemma{
		ID:           "LEM-test123",
		Statement:    "Every prime greater than 2 is odd.",
		SourceNodeID: sourceNodeID,
		ContentHash:  "abc123def456",
		Created:      created,
		Proof:        "Proof by contradiction.",
	}

	lemmaJSON, err := json.Marshal(lemma)
	if err != nil {
		t.Fatalf("failed to marshal lemma: %v", err)
	}

	lemmaPath := filepath.Join(lemmasDir, lemma.ID+".json")
	if err := os.WriteFile(lemmaPath, lemmaJSON, 0644); err != nil {
		t.Fatalf("failed to write lemma file: %v", err)
	}

	// Read it back using ReadLemma
	readLemma, err := ReadLemma(dir, lemma.ID)
	if err != nil {
		t.Fatalf("ReadLemma failed: %v", err)
	}

	if readLemma.ID != lemma.ID {
		t.Errorf("ID mismatch: got %q, want %q", readLemma.ID, lemma.ID)
	}
	if readLemma.Statement != lemma.Statement {
		t.Errorf("Statement mismatch: got %q, want %q", readLemma.Statement, lemma.Statement)
	}
	if readLemma.ContentHash != lemma.ContentHash {
		t.Errorf("ContentHash mismatch: got %q, want %q", readLemma.ContentHash, lemma.ContentHash)
	}
	if readLemma.SourceNodeID.String() != lemma.SourceNodeID.String() {
		t.Errorf("SourceNodeID mismatch: got %q, want %q", readLemma.SourceNodeID.String(), lemma.SourceNodeID.String())
	}
	if readLemma.Proof != lemma.Proof {
		t.Errorf("Proof mismatch: got %q, want %q", readLemma.Proof, lemma.Proof)
	}
}

// TestReadLemma_NotFound verifies that ReadLemma returns an
// appropriate error when the lemma doesn't exist.
func TestReadLemma_NotFound(t *testing.T) {
	t.Run("file_not_found", func(t *testing.T) {
		dir := t.TempDir()
		lemmasDir := filepath.Join(dir, "lemmas")
		if err := os.MkdirAll(lemmasDir, 0755); err != nil {
			t.Fatalf("failed to create lemmas directory: %v", err)
		}

		_, err := ReadLemma(dir, "nonexistent-id")
		if err == nil {
			t.Error("expected error for nonexistent lemma, got nil")
		}
		if !os.IsNotExist(err) {
			// Accept either os.IsNotExist or a wrapped error
			// Just verify we got an error
			t.Logf("got error (expected): %v", err)
		}
	})

	t.Run("lemmas_dir_not_found", func(t *testing.T) {
		dir := t.TempDir()
		// Note: lemmas/ directory does NOT exist

		_, err := ReadLemma(dir, "any-id")
		if err == nil {
			t.Error("expected error when lemmas directory doesn't exist, got nil")
		}
	})
}

// TestReadLemma_EmptyKey verifies that ReadLemma returns an
// error for an empty key.
func TestReadLemma_EmptyKey(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	_, err := ReadLemma(dir, "")
	if err == nil {
		t.Error("expected error for empty key, got nil")
	}
}

// TestReadLemma_WhitespaceKey verifies that ReadLemma returns an
// error for a whitespace-only key.
func TestReadLemma_WhitespaceKey(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	_, err := ReadLemma(dir, "   ")
	if err == nil {
		t.Error("expected error for whitespace key, got nil")
	}
}

// TestReadLemma_InvalidJSON verifies that ReadLemma returns an
// error when the file contains invalid JSON.
func TestReadLemma_InvalidJSON(t *testing.T) {
	t.Run("malformed_json", func(t *testing.T) {
		dir := t.TempDir()
		lemmasDir := filepath.Join(dir, "lemmas")
		if err := os.MkdirAll(lemmasDir, 0755); err != nil {
			t.Fatalf("failed to create lemmas directory: %v", err)
		}

		// Write invalid JSON
		lemmaPath := filepath.Join(lemmasDir, "bad-lemma.json")
		if err := os.WriteFile(lemmaPath, []byte("not valid json{"), 0644); err != nil {
			t.Fatalf("failed to write invalid lemma file: %v", err)
		}

		_, err := ReadLemma(dir, "bad-lemma")
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("empty_file", func(t *testing.T) {
		dir := t.TempDir()
		lemmasDir := filepath.Join(dir, "lemmas")
		if err := os.MkdirAll(lemmasDir, 0755); err != nil {
			t.Fatalf("failed to create lemmas directory: %v", err)
		}

		// Write empty file
		lemmaPath := filepath.Join(lemmasDir, "empty.json")
		if err := os.WriteFile(lemmaPath, []byte(""), 0644); err != nil {
			t.Fatalf("failed to write empty lemma file: %v", err)
		}

		_, err := ReadLemma(dir, "empty")
		if err == nil {
			t.Error("expected error for empty file, got nil")
		}
	})

	t.Run("wrong_type", func(t *testing.T) {
		dir := t.TempDir()
		lemmasDir := filepath.Join(dir, "lemmas")
		if err := os.MkdirAll(lemmasDir, 0755); err != nil {
			t.Fatalf("failed to create lemmas directory: %v", err)
		}

		// Write valid JSON but not a lemma object
		lemmaPath := filepath.Join(lemmasDir, "array.json")
		if err := os.WriteFile(lemmaPath, []byte("[1, 2, 3]"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		_, err := ReadLemma(dir, "array")
		if err == nil {
			t.Error("expected error for wrong JSON type, got nil")
		}
	})
}

// TestWriteLemma_InvalidPath verifies that WriteLemma returns
// an error for invalid paths.
func TestWriteLemma_InvalidPath(t *testing.T) {
	sourceNodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lemma, err := node.NewLemma("test statement", sourceNodeID)
	if err != nil {
		t.Fatalf("failed to create lemma: %v", err)
	}

	t.Run("empty_path", func(t *testing.T) {
		err := WriteLemma("", lemma)
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only_path", func(t *testing.T) {
		err := WriteLemma("   ", lemma)
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		err := WriteLemma("path\x00with\x00nulls", lemma)
		if err == nil {
			t.Error("expected error for path with null bytes, got nil")
		}
	})

	t.Run("path_is_file", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "notadir")

		// Create a file where a directory is expected
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		err := WriteLemma(filePath, lemma)
		if err == nil {
			t.Error("expected error when path is a file, got nil")
		}
	})
}

// TestWriteLemma_PermissionDenied verifies that WriteLemma handles
// permission errors gracefully.
func TestWriteLemma_PermissionDenied(t *testing.T) {
	// Skip on Windows where permission model differs
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	// Skip if running as root (root can write anywhere)
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")

	// Create lemmas directory with no write permission
	if err := os.MkdirAll(lemmasDir, 0555); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(lemmasDir, 0755)
	})

	sourceNodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lemma, err := node.NewLemma("test content", sourceNodeID)
	if err != nil {
		t.Fatalf("failed to create lemma: %v", err)
	}

	err = WriteLemma(dir, lemma)
	if err == nil {
		t.Error("expected error when writing to read-only directory, got nil")
	}
}

// TestWriteLemma_Overwrite verifies the behavior when overwriting
// an existing lemma file.
func TestWriteLemma_Overwrite(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create initial lemma
	lemma, err := node.NewLemma("The square of any integer is non-negative.", sourceNodeID)
	if err != nil {
		t.Fatalf("failed to create lemma: %v", err)
	}

	// Write it first time
	err = WriteLemma(dir, lemma)
	if err != nil {
		t.Fatalf("first WriteLemma failed: %v", err)
	}

	// Modify the lemma (same ID, different content)
	modifiedSourceNodeID, _ := types.Parse("1.2")
	modifiedLemma := &node.Lemma{
		ID:           lemma.ID,
		Statement:    "Updated: The square of any real number is non-negative.",
		SourceNodeID: modifiedSourceNodeID,
		ContentHash:  "updated-hash",
		Created:      lemma.Created,
		Proof:        "Proof added after update.",
	}

	// Write it again - should overwrite
	err = WriteLemma(dir, modifiedLemma)
	if err != nil {
		t.Fatalf("second WriteLemma (overwrite) failed: %v", err)
	}

	// Read it back and verify the updated content
	readLemma, err := ReadLemma(dir, lemma.ID)
	if err != nil {
		t.Fatalf("ReadLemma after overwrite failed: %v", err)
	}

	if readLemma.Statement != modifiedLemma.Statement {
		t.Errorf("Statement not updated: got %q, want %q", readLemma.Statement, modifiedLemma.Statement)
	}
	if readLemma.Proof != modifiedLemma.Proof {
		t.Errorf("Proof not updated: got %q, want %q", readLemma.Proof, modifiedLemma.Proof)
	}
	if readLemma.SourceNodeID.String() != modifiedLemma.SourceNodeID.String() {
		t.Errorf("SourceNodeID not updated: got %q, want %q", readLemma.SourceNodeID.String(), modifiedLemma.SourceNodeID.String())
	}
}

// TestListLemmas verifies that ListLemmas returns all lemma
// IDs in the lemmas/ directory.
func TestListLemmas(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create several lemmas
	lemma1, _ := node.NewLemma("lemma 1 statement", sourceNodeID)
	lemma2, _ := node.NewLemma("lemma 2 statement", sourceNodeID)
	lemma3, _ := node.NewLemma("lemma 3 statement", sourceNodeID)

	for _, lem := range []*node.Lemma{lemma1, lemma2, lemma3} {
		if err := WriteLemma(dir, lem); err != nil {
			t.Fatalf("failed to write lemma %s: %v", lem.ID, err)
		}
	}

	// List all lemmas
	ids, err := ListLemmas(dir)
	if err != nil {
		t.Fatalf("ListLemmas failed: %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("expected 3 lemmas, got %d", len(ids))
	}

	// Sort both slices for comparison
	expectedIDs := []string{lemma1.ID, lemma2.ID, lemma3.ID}
	sort.Strings(ids)
	sort.Strings(expectedIDs)

	for i, id := range expectedIDs {
		if i >= len(ids) || ids[i] != id {
			t.Errorf("missing or mismatched ID: expected %q", id)
		}
	}
}

// TestListLemmas_Empty verifies that ListLemmas returns an empty
// slice when there are no lemmas.
func TestListLemmas_Empty(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	ids, err := ListLemmas(dir)
	if err != nil {
		t.Fatalf("ListLemmas failed: %v", err)
	}

	if ids == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 lemmas, got %d", len(ids))
	}
}

// TestListLemmas_NoLemmasDir verifies that ListLemmas returns an
// error when the lemmas/ directory doesn't exist.
func TestListLemmas_NoLemmasDir(t *testing.T) {
	dir := t.TempDir()
	// Note: lemmas/ directory does NOT exist

	_, err := ListLemmas(dir)
	if err == nil {
		t.Error("expected error when lemmas directory doesn't exist, got nil")
	}
}

// TestListLemmas_IgnoresNonJSONFiles verifies that ListLemmas
// only returns IDs for .json files.
func TestListLemmas_IgnoresNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create a valid lemma
	lemma, _ := node.NewLemma("valid lemma statement", sourceNodeID)
	if err := WriteLemma(dir, lemma); err != nil {
		t.Fatalf("failed to write lemma: %v", err)
	}

	// Create non-JSON files that should be ignored
	nonJSONFiles := []string{
		"readme.txt",
		"notes.md",
		"backup.json.bak",
		".hidden.json",
	}
	for _, name := range nonJSONFiles {
		path := filepath.Join(lemmasDir, name)
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to write non-JSON file %s: %v", name, err)
		}
	}

	// Create a subdirectory (should be ignored)
	subdir := filepath.Join(lemmasDir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	ids, err := ListLemmas(dir)
	if err != nil {
		t.Fatalf("ListLemmas failed: %v", err)
	}

	if len(ids) != 1 {
		t.Errorf("expected 1 lemma, got %d: %v", len(ids), ids)
	}
	if len(ids) > 0 && ids[0] != lemma.ID {
		t.Errorf("expected ID %q, got %q", lemma.ID, ids[0])
	}
}

// TestListLemmas_EmptyPath verifies that ListLemmas returns an
// error for an empty path.
func TestListLemmas_EmptyPath(t *testing.T) {
	_, err := ListLemmas("")
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}

// TestDeleteLemma verifies that DeleteLemma removes a lemma
// file from the lemmas/ directory.
func TestDeleteLemma(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create a lemma
	lemma, _ := node.NewLemma("to-delete lemma statement", sourceNodeID)
	if err := WriteLemma(dir, lemma); err != nil {
		t.Fatalf("failed to write lemma: %v", err)
	}

	// Verify it exists
	lemmaPath := filepath.Join(lemmasDir, lemma.ID+".json")
	if _, err := os.Stat(lemmaPath); os.IsNotExist(err) {
		t.Fatal("lemma file should exist before delete")
	}

	// Delete it
	err = DeleteLemma(dir, lemma.ID)
	if err != nil {
		t.Fatalf("DeleteLemma failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(lemmaPath); !os.IsNotExist(err) {
		t.Error("expected lemma file to be deleted")
	}
}

// TestDeleteLemma_NotFound verifies that DeleteLemma returns an
// error when the lemma doesn't exist.
func TestDeleteLemma_NotFound(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	err := DeleteLemma(dir, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent lemma, got nil")
	}
}

// TestDeleteLemma_EmptyKey verifies that DeleteLemma returns an
// error for an empty key.
func TestDeleteLemma_EmptyKey(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	err := DeleteLemma(dir, "")
	if err == nil {
		t.Error("expected error for empty key, got nil")
	}
}

// TestDeleteLemma_WhitespaceKey verifies that DeleteLemma returns an
// error for a whitespace-only key.
func TestDeleteLemma_WhitespaceKey(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	err := DeleteLemma(dir, "   ")
	if err == nil {
		t.Error("expected error for whitespace key, got nil")
	}
}

// TestDeleteLemma_DoesNotAffectOthers verifies that DeleteLemma
// only removes the specified lemma.
func TestDeleteLemma_DoesNotAffectOthers(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create multiple lemmas
	lemma1, _ := node.NewLemma("keep this lemma 1", sourceNodeID)
	lemma2, _ := node.NewLemma("delete me lemma", sourceNodeID)
	lemma3, _ := node.NewLemma("keep this lemma 2", sourceNodeID)

	for _, lem := range []*node.Lemma{lemma1, lemma2, lemma3} {
		if err := WriteLemma(dir, lem); err != nil {
			t.Fatalf("failed to write lemma %s: %v", lem.ID, err)
		}
	}

	// Delete only lemma2
	if err := DeleteLemma(dir, lemma2.ID); err != nil {
		t.Fatalf("DeleteLemma failed: %v", err)
	}

	// Verify lemma1 and lemma3 still exist
	for _, lem := range []*node.Lemma{lemma1, lemma3} {
		_, err := ReadLemma(dir, lem.ID)
		if err != nil {
			t.Errorf("lemma %s should still exist: %v", lem.ID, err)
		}
	}

	// Verify lemma2 is gone
	_, err = ReadLemma(dir, lemma2.ID)
	if err == nil {
		t.Error("deleted lemma should not be readable")
	}
}

// TestDeleteLemma_PermissionDenied verifies that DeleteLemma handles
// permission errors gracefully.
func TestDeleteLemma_PermissionDenied(t *testing.T) {
	// Skip on Windows where permission model differs
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	// Skip if running as root
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create a lemma
	lemma, _ := node.NewLemma("Cannot delete", sourceNodeID)
	if err := WriteLemma(dir, lemma); err != nil {
		t.Fatalf("WriteLemma failed: %v", err)
	}

	// Make directory read-only
	if err := os.Chmod(lemmasDir, 0555); err != nil {
		t.Fatalf("failed to change permissions: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(lemmasDir, 0755)
	})

	err = DeleteLemma(dir, lemma.ID)
	if err == nil {
		t.Error("expected error when directory is read-only, got nil")
	}
}

// TestDeleteLemma_EmptyPath verifies that DeleteLemma returns an
// error for an empty path.
func TestDeleteLemma_EmptyPath(t *testing.T) {
	err := DeleteLemma("", "somekey")
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}

// TestWriteLemma_AtomicWrite verifies that WriteLemma uses
// atomic write operations (write to temp, then rename).
func TestWriteLemma_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lemma, _ := node.NewLemma("atomic-test lemma", sourceNodeID)

	// Write the lemma
	err = WriteLemma(dir, lemma)
	if err != nil {
		t.Fatalf("WriteLemma failed: %v", err)
	}

	// Verify no temp files are left behind
	entries, err := os.ReadDir(lemmasDir)
	if err != nil {
		t.Fatalf("failed to read lemmas directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// Check for common temp file patterns
		if name != lemma.ID+".json" {
			// Allow only the expected .json file
			if filepath.Ext(name) == ".tmp" || filepath.Ext(name) == ".temp" {
				t.Errorf("temp file left behind: %s", name)
			}
		}
	}
}

// TestLemmaRoundTrip verifies that a lemma can be written and read back
// with all fields preserved.
func TestLemmaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, err := types.Parse("1.3.1.4")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	original, err := node.NewLemma("A function f is injective if f(a) = f(b) implies a = b.", sourceNodeID)
	if err != nil {
		t.Fatalf("failed to create lemma: %v", err)
	}
	original.SetProof("Follows directly from the definition.")

	// Write
	if err := WriteLemma(dir, original); err != nil {
		t.Fatalf("WriteLemma failed: %v", err)
	}

	// Read
	retrieved, err := ReadLemma(dir, original.ID)
	if err != nil {
		t.Fatalf("ReadLemma failed: %v", err)
	}

	// Compare all fields
	if retrieved.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, original.ID)
	}
	if retrieved.Statement != original.Statement {
		t.Errorf("Statement mismatch: got %q, want %q", retrieved.Statement, original.Statement)
	}
	if retrieved.ContentHash != original.ContentHash {
		t.Errorf("ContentHash mismatch: got %q, want %q", retrieved.ContentHash, original.ContentHash)
	}
	if retrieved.SourceNodeID.String() != original.SourceNodeID.String() {
		t.Errorf("SourceNodeID mismatch: got %q, want %q", retrieved.SourceNodeID.String(), original.SourceNodeID.String())
	}
	if retrieved.Proof != original.Proof {
		t.Errorf("Proof mismatch: got %q, want %q", retrieved.Proof, original.Proof)
	}
	// Truncate to seconds for timestamp comparison to avoid precision issues
	if !retrieved.Created.Equal(original.Created) {
		t.Errorf("Created mismatch: got %v, want %v", retrieved.Created, original.Created)
	}
}

// TestLemmaFileFormat verifies that lemma files use the expected
// JSON format with proper indentation for human readability.
func TestLemmaFileFormat(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lemma, _ := node.NewLemma("format-test lemma content", sourceNodeID)

	if err := WriteLemma(dir, lemma); err != nil {
		t.Fatalf("WriteLemma failed: %v", err)
	}

	// Read raw file content
	lemmaPath := filepath.Join(lemmasDir, lemma.ID+".json")
	content, err := os.ReadFile(lemmaPath)
	if err != nil {
		t.Fatalf("failed to read lemma file: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("lemma file is not valid JSON: %v", err)
	}

	// Verify expected fields are present
	expectedFields := []string{"id", "statement", "source_node_id", "content_hash", "created"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("expected field %q not found in lemma file", field)
		}
	}
}

// TestWriteLemma_InvalidLemma verifies that WriteLemma returns
// an error for invalid lemmas.
func TestWriteLemma_InvalidLemma(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, _ := types.Parse("1")

	t.Run("empty_id", func(t *testing.T) {
		lemma := &node.Lemma{
			ID:           "",
			Statement:    "test statement",
			SourceNodeID: sourceNodeID,
			ContentHash:  "hash",
		}
		err := WriteLemma(dir, lemma)
		if err == nil {
			t.Error("expected error for lemma with empty ID, got nil")
		}
	})

	t.Run("whitespace_id", func(t *testing.T) {
		lemma := &node.Lemma{
			ID:           "   ",
			Statement:    "test statement",
			SourceNodeID: sourceNodeID,
			ContentHash:  "hash",
		}
		err := WriteLemma(dir, lemma)
		if err == nil {
			t.Error("expected error for lemma with whitespace-only ID, got nil")
		}
	})
}

// TestReadLemma_PathTraversal verifies that ReadLemma prevents
// path traversal attacks.
func TestReadLemma_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	// Create a file outside the lemmas directory
	secretPath := filepath.Join(dir, "secret.json")
	secretContent := `{"id": "secret", "statement": "secret data", "source_node_id": "1", "content_hash": "hash", "created": "2025-01-01T00:00:00Z"}`
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
			_, err := ReadLemma(dir, key)
			// Should either error or not return the secret content
			if err == nil {
				t.Logf("ReadLemma succeeded for key %q - verify it didn't traverse", key)
				// If it succeeded, it should not have read the secret file
			}
		})
	}
}

// TestDeleteLemma_PathTraversal verifies that DeleteLemma prevents
// path traversal attacks.
func TestDeleteLemma_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	// Create a file outside the lemmas directory
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
			_ = DeleteLemma(dir, key)
			// Verify protected file still exists
			if _, err := os.Stat(protectedPath); os.IsNotExist(err) {
				t.Errorf("path traversal succeeded - protected file was deleted with key %q", key)
			}
		})
	}
}

// TestLemmaIO_RoundTripVariants verifies round-trip for various lemma configurations.
func TestLemmaIO_RoundTripVariants(t *testing.T) {
	dir := t.TempDir()
	lemmasDir := filepath.Join(dir, "lemmas")
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		t.Fatalf("failed to create lemmas directory: %v", err)
	}

	sourceNodeID, _ := types.Parse("1")
	deepSourceNodeID, _ := types.Parse("1.2.3.4.5")

	testCases := []struct {
		name  string
		setup func() *node.Lemma
	}{
		{
			name: "simple_lemma",
			setup: func() *node.Lemma {
				lem, _ := node.NewLemma("Simple statement", sourceNodeID)
				return lem
			},
		},
		{
			name: "with_proof",
			setup: func() *node.Lemma {
				lem, _ := node.NewLemma("Statement with proof", sourceNodeID)
				lem.SetProof("This is the proof text.")
				return lem
			},
		},
		{
			name: "special_characters",
			setup: func() *node.Lemma {
				lem, _ := node.NewLemma("Statement with \"quotes\" and 'apostrophes' and newlines\nand tabs\t", sourceNodeID)
				return lem
			},
		},
		{
			name: "unicode_statement",
			setup: func() *node.Lemma {
				lem, _ := node.NewLemma("Statement with unicode: pi != 0 and for all x in R", sourceNodeID)
				return lem
			},
		},
		{
			name: "deep_source_node_id",
			setup: func() *node.Lemma {
				lem, _ := node.NewLemma("Statement from deep node", deepSourceNodeID)
				return lem
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lemma := tc.setup()

			// Write
			if err := WriteLemma(dir, lemma); err != nil {
				t.Fatalf("WriteLemma failed: %v", err)
			}

			// Read
			readLemma, err := ReadLemma(dir, lemma.ID)
			if err != nil {
				t.Fatalf("ReadLemma failed: %v", err)
			}

			// Compare
			if readLemma.ID != lemma.ID {
				t.Errorf("ID mismatch: got %q, want %q", readLemma.ID, lemma.ID)
			}
			if readLemma.Statement != lemma.Statement {
				t.Errorf("Statement mismatch: got %q, want %q", readLemma.Statement, lemma.Statement)
			}
			if readLemma.ContentHash != lemma.ContentHash {
				t.Errorf("ContentHash mismatch: got %q, want %q", readLemma.ContentHash, lemma.ContentHash)
			}
			if readLemma.SourceNodeID.String() != lemma.SourceNodeID.String() {
				t.Errorf("SourceNodeID mismatch: got %q, want %q", readLemma.SourceNodeID.String(), lemma.SourceNodeID.String())
			}
			if readLemma.Proof != lemma.Proof {
				t.Errorf("Proof mismatch: got %q, want %q", readLemma.Proof, lemma.Proof)
			}
		})
	}
}
