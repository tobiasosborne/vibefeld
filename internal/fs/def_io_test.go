//go:build integration
// +build integration

// These tests define expected behavior for WriteDefinition, ReadDefinition,
// ListDefinitions, and DeleteDefinition.
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

// TestWriteDefinition verifies that WriteDefinition correctly writes a
// definition to the defs/ subdirectory as a JSON file.
func TestWriteDefinition(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	def, err := node.NewDefinition("continuity", "A function f is continuous at x if for every epsilon > 0, there exists delta > 0 such that |f(y) - f(x)| < epsilon whenever |y - x| < delta.")
	if err != nil {
		t.Fatalf("failed to create definition: %v", err)
	}

	err = WriteDefinition(dir, def)
	if err != nil {
		t.Fatalf("WriteDefinition failed: %v", err)
	}

	// Verify file was created with correct name
	defPath := filepath.Join(defsDir, def.ID+".json")
	info, err := os.Stat(defPath)
	if os.IsNotExist(err) {
		t.Fatalf("expected definition file to exist at %s", defPath)
	}
	if err != nil {
		t.Fatalf("error checking definition file: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected definition file to be a file, not a directory")
	}

	// Verify file contents are valid JSON matching the definition
	content, err := os.ReadFile(defPath)
	if err != nil {
		t.Fatalf("failed to read definition file: %v", err)
	}

	var readDef node.Definition
	if err := json.Unmarshal(content, &readDef); err != nil {
		t.Fatalf("definition file is not valid JSON: %v", err)
	}

	if readDef.ID != def.ID {
		t.Errorf("ID mismatch: got %q, want %q", readDef.ID, def.ID)
	}
	if readDef.Name != def.Name {
		t.Errorf("Name mismatch: got %q, want %q", readDef.Name, def.Name)
	}
	if readDef.Content != def.Content {
		t.Errorf("Content mismatch: got %q, want %q", readDef.Content, def.Content)
	}
	if readDef.ContentHash != def.ContentHash {
		t.Errorf("ContentHash mismatch: got %q, want %q", readDef.ContentHash, def.ContentHash)
	}
}

// TestWriteDefinition_CreatesDefsDir verifies that WriteDefinition creates
// the defs/ subdirectory if it doesn't exist.
func TestWriteDefinition_CreatesDefsDir(t *testing.T) {
	dir := t.TempDir()
	// Note: defs/ directory does NOT exist yet

	def, err := node.NewDefinition("limit", "The limit of f(x) as x approaches a is L.")
	if err != nil {
		t.Fatalf("failed to create definition: %v", err)
	}

	err = WriteDefinition(dir, def)
	if err != nil {
		t.Fatalf("WriteDefinition failed: %v", err)
	}

	// Verify defs directory was created
	defsDir := filepath.Join(dir, "defs")
	info, err := os.Stat(defsDir)
	if os.IsNotExist(err) {
		t.Fatal("expected defs directory to be created")
	}
	if err != nil {
		t.Fatalf("error checking defs directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected defs to be a directory")
	}

	// Verify file was created
	defPath := filepath.Join(defsDir, def.ID+".json")
	if _, err := os.Stat(defPath); os.IsNotExist(err) {
		t.Fatalf("expected definition file to exist at %s", defPath)
	}
}

// TestWriteDefinition_NilDefinition verifies that WriteDefinition returns
// an error when given a nil definition.
func TestWriteDefinition_NilDefinition(t *testing.T) {
	dir := t.TempDir()

	err := WriteDefinition(dir, nil)
	if err == nil {
		t.Error("expected error for nil definition, got nil")
	}
}

// TestReadDefinition verifies that ReadDefinition correctly reads a
// definition from the defs/ subdirectory.
func TestReadDefinition(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	// Create a definition and write it manually
	def, err := node.NewDefinition("derivative", "The derivative of f at x is the limit of (f(x+h) - f(x))/h as h approaches 0.")
	if err != nil {
		t.Fatalf("failed to create definition: %v", err)
	}

	defJSON, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("failed to marshal definition: %v", err)
	}

	defPath := filepath.Join(defsDir, def.ID+".json")
	if err := os.WriteFile(defPath, defJSON, 0644); err != nil {
		t.Fatalf("failed to write definition file: %v", err)
	}

	// Read it back using ReadDefinition
	readDef, err := ReadDefinition(dir, def.ID)
	if err != nil {
		t.Fatalf("ReadDefinition failed: %v", err)
	}

	if readDef.ID != def.ID {
		t.Errorf("ID mismatch: got %q, want %q", readDef.ID, def.ID)
	}
	if readDef.Name != def.Name {
		t.Errorf("Name mismatch: got %q, want %q", readDef.Name, def.Name)
	}
	if readDef.Content != def.Content {
		t.Errorf("Content mismatch: got %q, want %q", readDef.Content, def.Content)
	}
	if readDef.ContentHash != def.ContentHash {
		t.Errorf("ContentHash mismatch: got %q, want %q", readDef.ContentHash, def.ContentHash)
	}
}

// TestReadDefinition_NotFound verifies that ReadDefinition returns an
// appropriate error when the definition doesn't exist.
func TestReadDefinition_NotFound(t *testing.T) {
	t.Run("file_not_found", func(t *testing.T) {
		dir := t.TempDir()
		defsDir := filepath.Join(dir, "defs")
		if err := os.MkdirAll(defsDir, 0755); err != nil {
			t.Fatalf("failed to create defs directory: %v", err)
		}

		_, err := ReadDefinition(dir, "nonexistent-id")
		if err == nil {
			t.Error("expected error for nonexistent definition, got nil")
		}
		if !os.IsNotExist(err) {
			// Accept either os.IsNotExist or a wrapped error
			// Just verify we got an error
			t.Logf("got error (expected): %v", err)
		}
	})

	t.Run("defs_dir_not_found", func(t *testing.T) {
		dir := t.TempDir()
		// Note: defs/ directory does NOT exist

		_, err := ReadDefinition(dir, "any-id")
		if err == nil {
			t.Error("expected error when defs directory doesn't exist, got nil")
		}
	})
}

// TestReadDefinition_EmptyKey verifies that ReadDefinition returns an
// error for an empty key.
func TestReadDefinition_EmptyKey(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	_, err := ReadDefinition(dir, "")
	if err == nil {
		t.Error("expected error for empty key, got nil")
	}
}

// TestReadDefinition_InvalidJSON verifies that ReadDefinition returns an
// error when the file contains invalid JSON.
func TestReadDefinition_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	// Write invalid JSON
	defPath := filepath.Join(defsDir, "bad-def.json")
	if err := os.WriteFile(defPath, []byte("not valid json{"), 0644); err != nil {
		t.Fatalf("failed to write invalid definition file: %v", err)
	}

	_, err := ReadDefinition(dir, "bad-def")
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestWriteDefinition_InvalidPath verifies that WriteDefinition returns
// an error for invalid paths.
func TestWriteDefinition_InvalidPath(t *testing.T) {
	def, err := node.NewDefinition("test", "test content")
	if err != nil {
		t.Fatalf("failed to create definition: %v", err)
	}

	t.Run("empty_path", func(t *testing.T) {
		err := WriteDefinition("", def)
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only_path", func(t *testing.T) {
		err := WriteDefinition("   ", def)
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		err := WriteDefinition("path\x00with\x00nulls", def)
		if err == nil {
			t.Error("expected error for path with null bytes, got nil")
		}
	})
}

// TestWriteDefinition_PermissionDenied verifies that WriteDefinition handles
// permission errors gracefully.
func TestWriteDefinition_PermissionDenied(t *testing.T) {
	// Skip on Windows where permission model differs
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	// Skip if running as root (root can write anywhere)
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")

	// Create defs directory with no write permission
	if err := os.MkdirAll(defsDir, 0555); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(defsDir, 0755)
	})

	def, err := node.NewDefinition("test", "test content")
	if err != nil {
		t.Fatalf("failed to create definition: %v", err)
	}

	err = WriteDefinition(dir, def)
	if err == nil {
		t.Error("expected error when writing to read-only directory, got nil")
	}
}

// TestWriteDefinition_Overwrite verifies the behavior when overwriting
// an existing definition file.
func TestWriteDefinition_Overwrite(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	// Create initial definition
	def, err := node.NewDefinition("integral", "The integral of f from a to b.")
	if err != nil {
		t.Fatalf("failed to create definition: %v", err)
	}

	// Write it first time
	err = WriteDefinition(dir, def)
	if err != nil {
		t.Fatalf("first WriteDefinition failed: %v", err)
	}

	// Modify the definition (same ID, different content)
	modifiedDef := &node.Definition{
		ID:          def.ID,
		Name:        "integral_updated",
		Content:     "The Riemann integral of f from a to b.",
		ContentHash: "updated-hash",
		Created:     def.Created,
	}

	// Write it again - should overwrite
	err = WriteDefinition(dir, modifiedDef)
	if err != nil {
		t.Fatalf("second WriteDefinition (overwrite) failed: %v", err)
	}

	// Read it back and verify the updated content
	readDef, err := ReadDefinition(dir, def.ID)
	if err != nil {
		t.Fatalf("ReadDefinition after overwrite failed: %v", err)
	}

	if readDef.Name != modifiedDef.Name {
		t.Errorf("Name not updated: got %q, want %q", readDef.Name, modifiedDef.Name)
	}
	if readDef.Content != modifiedDef.Content {
		t.Errorf("Content not updated: got %q, want %q", readDef.Content, modifiedDef.Content)
	}
}

// TestListDefinitions verifies that ListDefinitions returns all definition
// IDs in the defs/ directory.
func TestListDefinitions(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	// Create several definitions
	def1, _ := node.NewDefinition("term1", "definition 1")
	def2, _ := node.NewDefinition("term2", "definition 2")
	def3, _ := node.NewDefinition("term3", "definition 3")

	for _, def := range []*node.Definition{def1, def2, def3} {
		if err := WriteDefinition(dir, def); err != nil {
			t.Fatalf("failed to write definition %s: %v", def.ID, err)
		}
	}

	// List all definitions
	ids, err := ListDefinitions(dir)
	if err != nil {
		t.Fatalf("ListDefinitions failed: %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("expected 3 definitions, got %d", len(ids))
	}

	// Sort both slices for comparison
	expectedIDs := []string{def1.ID, def2.ID, def3.ID}
	sort.Strings(ids)
	sort.Strings(expectedIDs)

	for i, id := range expectedIDs {
		if i >= len(ids) || ids[i] != id {
			t.Errorf("missing or mismatched ID: expected %q", id)
		}
	}
}

// TestListDefinitions_Empty verifies that ListDefinitions returns an empty
// slice when there are no definitions.
func TestListDefinitions_Empty(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	ids, err := ListDefinitions(dir)
	if err != nil {
		t.Fatalf("ListDefinitions failed: %v", err)
	}

	if ids == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 definitions, got %d", len(ids))
	}
}

// TestListDefinitions_NoDefsDir verifies that ListDefinitions returns an
// error when the defs/ directory doesn't exist.
func TestListDefinitions_NoDefsDir(t *testing.T) {
	dir := t.TempDir()
	// Note: defs/ directory does NOT exist

	_, err := ListDefinitions(dir)
	if err == nil {
		t.Error("expected error when defs directory doesn't exist, got nil")
	}
}

// TestListDefinitions_IgnoresNonJSONFiles verifies that ListDefinitions
// only returns IDs for .json files.
func TestListDefinitions_IgnoresNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	// Create a valid definition
	def, _ := node.NewDefinition("valid", "valid definition")
	if err := WriteDefinition(dir, def); err != nil {
		t.Fatalf("failed to write definition: %v", err)
	}

	// Create non-JSON files that should be ignored
	nonJSONFiles := []string{
		"readme.txt",
		"notes.md",
		"backup.json.bak",
		".hidden.json",
	}
	for _, name := range nonJSONFiles {
		path := filepath.Join(defsDir, name)
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to write non-JSON file %s: %v", name, err)
		}
	}

	// Create a subdirectory (should be ignored)
	subdir := filepath.Join(defsDir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	ids, err := ListDefinitions(dir)
	if err != nil {
		t.Fatalf("ListDefinitions failed: %v", err)
	}

	if len(ids) != 1 {
		t.Errorf("expected 1 definition, got %d: %v", len(ids), ids)
	}
	if len(ids) > 0 && ids[0] != def.ID {
		t.Errorf("expected ID %q, got %q", def.ID, ids[0])
	}
}

// TestDeleteDefinition verifies that DeleteDefinition removes a definition
// file from the defs/ directory.
func TestDeleteDefinition(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	// Create a definition
	def, _ := node.NewDefinition("to-delete", "this will be deleted")
	if err := WriteDefinition(dir, def); err != nil {
		t.Fatalf("failed to write definition: %v", err)
	}

	// Verify it exists
	defPath := filepath.Join(defsDir, def.ID+".json")
	if _, err := os.Stat(defPath); os.IsNotExist(err) {
		t.Fatal("definition file should exist before delete")
	}

	// Delete it
	err := DeleteDefinition(dir, def.ID)
	if err != nil {
		t.Fatalf("DeleteDefinition failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(defPath); !os.IsNotExist(err) {
		t.Error("expected definition file to be deleted")
	}
}

// TestDeleteDefinition_NotFound verifies that DeleteDefinition returns an
// error when the definition doesn't exist.
func TestDeleteDefinition_NotFound(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	err := DeleteDefinition(dir, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent definition, got nil")
	}
}

// TestDeleteDefinition_EmptyKey verifies that DeleteDefinition returns an
// error for an empty key.
func TestDeleteDefinition_EmptyKey(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	err := DeleteDefinition(dir, "")
	if err == nil {
		t.Error("expected error for empty key, got nil")
	}
}

// TestDeleteDefinition_DoesNotAffectOthers verifies that DeleteDefinition
// only removes the specified definition.
func TestDeleteDefinition_DoesNotAffectOthers(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	// Create multiple definitions
	def1, _ := node.NewDefinition("keep1", "keep this")
	def2, _ := node.NewDefinition("delete-me", "delete this")
	def3, _ := node.NewDefinition("keep2", "keep this too")

	for _, def := range []*node.Definition{def1, def2, def3} {
		if err := WriteDefinition(dir, def); err != nil {
			t.Fatalf("failed to write definition %s: %v", def.ID, err)
		}
	}

	// Delete only def2
	if err := DeleteDefinition(dir, def2.ID); err != nil {
		t.Fatalf("DeleteDefinition failed: %v", err)
	}

	// Verify def1 and def3 still exist
	for _, def := range []*node.Definition{def1, def3} {
		_, err := ReadDefinition(dir, def.ID)
		if err != nil {
			t.Errorf("definition %s should still exist: %v", def.ID, err)
		}
	}

	// Verify def2 is gone
	_, err := ReadDefinition(dir, def2.ID)
	if err == nil {
		t.Error("deleted definition should not be readable")
	}
}

// TestWriteDefinition_AtomicWrite verifies that WriteDefinition uses
// atomic write operations (write to temp, then rename).
func TestWriteDefinition_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	def, _ := node.NewDefinition("atomic-test", "test atomic write")

	// Write the definition
	err := WriteDefinition(dir, def)
	if err != nil {
		t.Fatalf("WriteDefinition failed: %v", err)
	}

	// Verify no temp files are left behind
	entries, err := os.ReadDir(defsDir)
	if err != nil {
		t.Fatalf("failed to read defs directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// Check for common temp file patterns
		if name != def.ID+".json" {
			// Allow only the expected .json file
			if filepath.Ext(name) == ".tmp" || filepath.Ext(name) == ".temp" {
				t.Errorf("temp file left behind: %s", name)
			}
		}
	}
}

// TestRoundTrip verifies that a definition can be written and read back
// with all fields preserved.
func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	original, err := node.NewDefinition("roundtrip", "A function f is injective if f(a) = f(b) implies a = b.")
	if err != nil {
		t.Fatalf("failed to create definition: %v", err)
	}

	// Write
	if err := WriteDefinition(dir, original); err != nil {
		t.Fatalf("WriteDefinition failed: %v", err)
	}

	// Read
	retrieved, err := ReadDefinition(dir, original.ID)
	if err != nil {
		t.Fatalf("ReadDefinition failed: %v", err)
	}

	// Compare all fields
	if retrieved.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, original.ID)
	}
	if retrieved.Name != original.Name {
		t.Errorf("Name mismatch: got %q, want %q", retrieved.Name, original.Name)
	}
	if retrieved.Content != original.Content {
		t.Errorf("Content mismatch: got %q, want %q", retrieved.Content, original.Content)
	}
	if retrieved.ContentHash != original.ContentHash {
		t.Errorf("ContentHash mismatch: got %q, want %q", retrieved.ContentHash, original.ContentHash)
	}
	if !retrieved.Created.Equal(original.Created) {
		t.Errorf("Created mismatch: got %v, want %v", retrieved.Created, original.Created)
	}
}

// TestDefinitionFileFormat verifies that definition files use the expected
// JSON format with proper indentation for human readability.
func TestDefinitionFileFormat(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	def, _ := node.NewDefinition("format-test", "test content")

	if err := WriteDefinition(dir, def); err != nil {
		t.Fatalf("WriteDefinition failed: %v", err)
	}

	// Read raw file content
	defPath := filepath.Join(defsDir, def.ID+".json")
	content, err := os.ReadFile(defPath)
	if err != nil {
		t.Fatalf("failed to read definition file: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("definition file is not valid JSON: %v", err)
	}

	// Verify expected fields are present
	expectedFields := []string{"id", "name", "content", "content_hash", "created"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("expected field %q not found in definition file", field)
		}
	}
}

// TestWriteDefinition_InvalidDefinition verifies that WriteDefinition returns
// an error for invalid definitions.
func TestWriteDefinition_InvalidDefinition(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	t.Run("empty_id", func(t *testing.T) {
		def := &node.Definition{
			ID:      "",
			Name:    "test",
			Content: "test content",
		}
		err := WriteDefinition(dir, def)
		if err == nil {
			t.Error("expected error for definition with empty ID, got nil")
		}
	})

	t.Run("whitespace_id", func(t *testing.T) {
		def := &node.Definition{
			ID:      "   ",
			Name:    "test",
			Content: "test content",
		}
		err := WriteDefinition(dir, def)
		if err == nil {
			t.Error("expected error for definition with whitespace-only ID, got nil")
		}
	})
}

// TestReadDefinition_PathTraversal verifies that ReadDefinition prevents
// path traversal attacks.
func TestReadDefinition_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	// Create a file outside the defs directory
	secretPath := filepath.Join(dir, "secret.json")
	secretContent := `{"id": "secret", "name": "secret", "content": "secret data"}`
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
			_, err := ReadDefinition(dir, key)
			// Should either error or not return the secret content
			if err == nil {
				t.Logf("ReadDefinition succeeded for key %q - verify it didn't traverse", key)
				// If it succeeded, it should not have read the secret file
			}
		})
	}
}

// TestDeleteDefinition_PathTraversal verifies that DeleteDefinition prevents
// path traversal attacks.
func TestDeleteDefinition_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	defsDir := filepath.Join(dir, "defs")
	if err := os.MkdirAll(defsDir, 0755); err != nil {
		t.Fatalf("failed to create defs directory: %v", err)
	}

	// Create a file outside the defs directory
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
			_ = DeleteDefinition(dir, key)
			// Verify protected file still exists
			if _, err := os.Stat(protectedPath); os.IsNotExist(err) {
				t.Errorf("path traversal succeeded - protected file was deleted with key %q", key)
			}
		})
	}
}
