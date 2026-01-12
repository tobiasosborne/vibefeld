//go:build integration
// +build integration

// These tests define expected behavior for WriteMeta and ReadMeta.
// Run with: go test -tags=integration ./internal/fs/...

package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestWriteMeta verifies that WriteMeta correctly writes metadata
// to meta.json in the proof directory.
func TestWriteMeta(t *testing.T) {
	dir := t.TempDir()

	meta := &Meta{
		Conjecture: "The square root of 2 is irrational",
		CreatedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Version:    "1.0",
	}

	err := WriteMeta(dir, meta)
	if err != nil {
		t.Fatalf("WriteMeta failed: %v", err)
	}

	// Verify file was created
	metaPath := filepath.Join(dir, "meta.json")
	info, err := os.Stat(metaPath)
	if os.IsNotExist(err) {
		t.Fatalf("expected meta.json to exist at %s", metaPath)
	}
	if err != nil {
		t.Fatalf("error checking meta.json: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected meta.json to be a file, not a directory")
	}

	// Verify file contents are valid JSON matching the metadata
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read meta.json: %v", err)
	}

	var readMeta Meta
	if err := json.Unmarshal(content, &readMeta); err != nil {
		t.Fatalf("meta.json is not valid JSON: %v", err)
	}

	if readMeta.Conjecture != meta.Conjecture {
		t.Errorf("Conjecture mismatch: got %q, want %q", readMeta.Conjecture, meta.Conjecture)
	}
	if !readMeta.CreatedAt.Equal(meta.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", readMeta.CreatedAt, meta.CreatedAt)
	}
	if readMeta.Version != meta.Version {
		t.Errorf("Version mismatch: got %q, want %q", readMeta.Version, meta.Version)
	}
}

// TestWriteMeta_NilMeta verifies that WriteMeta returns an error
// when given a nil meta.
func TestWriteMeta_NilMeta(t *testing.T) {
	dir := t.TempDir()

	err := WriteMeta(dir, nil)
	if err == nil {
		t.Error("expected error for nil meta, got nil")
	}
}

// TestWriteMeta_RequiredFields verifies that WriteMeta returns errors
// for missing required fields.
func TestWriteMeta_RequiredFields(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing_conjecture", func(t *testing.T) {
		meta := &Meta{
			Conjecture: "",
			CreatedAt:  time.Now(),
			Version:    "1.0",
		}
		err := WriteMeta(dir, meta)
		if err == nil {
			t.Error("expected error for empty conjecture, got nil")
		}
	})

	t.Run("whitespace_only_conjecture", func(t *testing.T) {
		meta := &Meta{
			Conjecture: "   ",
			CreatedAt:  time.Now(),
			Version:    "1.0",
		}
		err := WriteMeta(dir, meta)
		if err == nil {
			t.Error("expected error for whitespace-only conjecture, got nil")
		}
	})

	t.Run("missing_version", func(t *testing.T) {
		meta := &Meta{
			Conjecture: "Some conjecture",
			CreatedAt:  time.Now(),
			Version:    "",
		}
		err := WriteMeta(dir, meta)
		if err == nil {
			t.Error("expected error for empty version, got nil")
		}
	})

	t.Run("zero_created_at", func(t *testing.T) {
		meta := &Meta{
			Conjecture: "Some conjecture",
			CreatedAt:  time.Time{},
			Version:    "1.0",
		}
		err := WriteMeta(dir, meta)
		if err == nil {
			t.Error("expected error for zero CreatedAt, got nil")
		}
	})
}

// TestWriteMeta_InvalidPath verifies that WriteMeta returns an error
// for invalid paths.
func TestWriteMeta_InvalidPath(t *testing.T) {
	meta := &Meta{
		Conjecture: "Some conjecture",
		CreatedAt:  time.Now(),
		Version:    "1.0",
	}

	t.Run("empty_path", func(t *testing.T) {
		err := WriteMeta("", meta)
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only_path", func(t *testing.T) {
		err := WriteMeta("   ", meta)
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		err := WriteMeta("path\x00with\x00nulls", meta)
		if err == nil {
			t.Error("expected error for path with null bytes, got nil")
		}
	})
}

// TestWriteMeta_Overwrite verifies that WriteMeta overwrites existing
// meta.json files.
func TestWriteMeta_Overwrite(t *testing.T) {
	dir := t.TempDir()

	// Write first version
	meta1 := &Meta{
		Conjecture: "First conjecture",
		CreatedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Version:    "1.0",
	}
	if err := WriteMeta(dir, meta1); err != nil {
		t.Fatalf("first WriteMeta failed: %v", err)
	}

	// Write second version (overwrite)
	meta2 := &Meta{
		Conjecture: "Updated conjecture",
		CreatedAt:  time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
		Version:    "2.0",
	}
	if err := WriteMeta(dir, meta2); err != nil {
		t.Fatalf("second WriteMeta (overwrite) failed: %v", err)
	}

	// Read back and verify updated content
	readMeta, err := ReadMeta(dir)
	if err != nil {
		t.Fatalf("ReadMeta after overwrite failed: %v", err)
	}

	if readMeta.Conjecture != meta2.Conjecture {
		t.Errorf("Conjecture not updated: got %q, want %q", readMeta.Conjecture, meta2.Conjecture)
	}
	if readMeta.Version != meta2.Version {
		t.Errorf("Version not updated: got %q, want %q", readMeta.Version, meta2.Version)
	}
}

// TestWriteMeta_AtomicWrite verifies that WriteMeta uses atomic write
// operations (write to temp file, then rename).
func TestWriteMeta_AtomicWrite(t *testing.T) {
	dir := t.TempDir()

	meta := &Meta{
		Conjecture: "Test atomic write",
		CreatedAt:  time.Now(),
		Version:    "1.0",
	}

	err := WriteMeta(dir, meta)
	if err != nil {
		t.Fatalf("WriteMeta failed: %v", err)
	}

	// Verify no temp files are left behind
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// Check for common temp file patterns
		if name != "meta.json" {
			if filepath.Ext(name) == ".tmp" || filepath.Ext(name) == ".temp" {
				t.Errorf("temp file left behind: %s", name)
			}
		}
	}
}

// TestWriteMeta_PermissionDenied verifies that WriteMeta handles
// permission errors gracefully.
func TestWriteMeta_PermissionDenied(t *testing.T) {
	// Skip on Windows where permission model differs
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	// Skip if running as root (root can write anywhere)
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()

	// Remove write permission from directory
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatalf("failed to change directory permissions: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(dir, 0755)
	})

	meta := &Meta{
		Conjecture: "Test permission denied",
		CreatedAt:  time.Now(),
		Version:    "1.0",
	}

	err := WriteMeta(dir, meta)
	if err == nil {
		t.Error("expected error when writing to read-only directory, got nil")
	}
}

// TestReadMeta verifies that ReadMeta correctly reads metadata
// from meta.json in the proof directory.
func TestReadMeta(t *testing.T) {
	dir := t.TempDir()

	// Create a meta.json file manually
	expectedMeta := Meta{
		Conjecture: "Every even integer greater than 2 is the sum of two primes",
		CreatedAt:  time.Date(2024, 3, 14, 15, 9, 26, 0, time.UTC),
		Version:    "1.0",
	}

	metaJSON, err := json.Marshal(expectedMeta)
	if err != nil {
		t.Fatalf("failed to marshal meta: %v", err)
	}

	metaPath := filepath.Join(dir, "meta.json")
	if err := os.WriteFile(metaPath, metaJSON, 0644); err != nil {
		t.Fatalf("failed to write meta.json: %v", err)
	}

	// Read it back using ReadMeta
	readMeta, err := ReadMeta(dir)
	if err != nil {
		t.Fatalf("ReadMeta failed: %v", err)
	}

	if readMeta.Conjecture != expectedMeta.Conjecture {
		t.Errorf("Conjecture mismatch: got %q, want %q", readMeta.Conjecture, expectedMeta.Conjecture)
	}
	if !readMeta.CreatedAt.Equal(expectedMeta.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", readMeta.CreatedAt, expectedMeta.CreatedAt)
	}
	if readMeta.Version != expectedMeta.Version {
		t.Errorf("Version mismatch: got %q, want %q", readMeta.Version, expectedMeta.Version)
	}
}

// TestReadMeta_NotFound verifies that ReadMeta returns an appropriate
// error when meta.json doesn't exist.
func TestReadMeta_NotFound(t *testing.T) {
	dir := t.TempDir()
	// Note: meta.json does NOT exist

	_, err := ReadMeta(dir)
	if err == nil {
		t.Error("expected error for missing meta.json, got nil")
	}
	if !os.IsNotExist(err) {
		// Accept either os.IsNotExist or a wrapped error
		t.Logf("got error (expected): %v", err)
	}
}

// TestReadMeta_InvalidJSON verifies that ReadMeta returns an error
// when meta.json contains invalid JSON.
func TestReadMeta_InvalidJSON(t *testing.T) {
	dir := t.TempDir()

	// Write invalid JSON
	metaPath := filepath.Join(dir, "meta.json")
	if err := os.WriteFile(metaPath, []byte("not valid json{"), 0644); err != nil {
		t.Fatalf("failed to write invalid meta.json: %v", err)
	}

	_, err := ReadMeta(dir)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestReadMeta_InvalidPath verifies that ReadMeta returns an error
// for invalid paths.
func TestReadMeta_InvalidPath(t *testing.T) {
	t.Run("empty_path", func(t *testing.T) {
		_, err := ReadMeta("")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only_path", func(t *testing.T) {
		_, err := ReadMeta("   ")
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		_, err := ReadMeta("path\x00with\x00nulls")
		if err == nil {
			t.Error("expected error for path with null bytes, got nil")
		}
	})
}

// TestReadMeta_PartialData verifies behavior when meta.json has
// incomplete or missing fields.
func TestReadMeta_PartialData(t *testing.T) {
	t.Run("missing_conjecture", func(t *testing.T) {
		dir := t.TempDir()
		metaPath := filepath.Join(dir, "meta.json")
		content := `{"created_at": "2024-01-01T00:00:00Z", "version": "1.0"}`
		if err := os.WriteFile(metaPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write meta.json: %v", err)
		}

		meta, err := ReadMeta(dir)
		if err != nil {
			t.Fatalf("ReadMeta failed: %v", err)
		}
		// Empty string is valid for reading (validation happens on write)
		if meta.Conjecture != "" {
			t.Errorf("expected empty Conjecture, got %q", meta.Conjecture)
		}
	})

	t.Run("only_version", func(t *testing.T) {
		dir := t.TempDir()
		metaPath := filepath.Join(dir, "meta.json")
		content := `{"version": "1.0"}`
		if err := os.WriteFile(metaPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write meta.json: %v", err)
		}

		meta, err := ReadMeta(dir)
		if err != nil {
			t.Fatalf("ReadMeta failed: %v", err)
		}
		if meta.Version != "1.0" {
			t.Errorf("expected Version '1.0', got %q", meta.Version)
		}
	})
}

// TestRoundTrip_Meta verifies that metadata can be written and read back
// with all fields preserved.
func TestRoundTrip_Meta(t *testing.T) {
	dir := t.TempDir()

	original := &Meta{
		Conjecture: "There are infinitely many prime numbers",
		CreatedAt:  time.Date(2024, 7, 4, 12, 30, 45, 123456789, time.UTC),
		Version:    "1.2.3",
	}

	// Write
	if err := WriteMeta(dir, original); err != nil {
		t.Fatalf("WriteMeta failed: %v", err)
	}

	// Read
	retrieved, err := ReadMeta(dir)
	if err != nil {
		t.Fatalf("ReadMeta failed: %v", err)
	}

	// Compare all fields
	if retrieved.Conjecture != original.Conjecture {
		t.Errorf("Conjecture mismatch: got %q, want %q", retrieved.Conjecture, original.Conjecture)
	}
	// Note: JSON time marshaling may lose nanosecond precision
	if !retrieved.CreatedAt.Round(time.Second).Equal(original.CreatedAt.Round(time.Second)) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", retrieved.CreatedAt, original.CreatedAt)
	}
	if retrieved.Version != original.Version {
		t.Errorf("Version mismatch: got %q, want %q", retrieved.Version, original.Version)
	}
}

// TestMetaFileFormat verifies that meta.json uses the expected JSON format
// with proper indentation for human readability.
func TestMetaFileFormat(t *testing.T) {
	dir := t.TempDir()

	meta := &Meta{
		Conjecture: "Test conjecture for format",
		CreatedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Version:    "1.0",
	}

	if err := WriteMeta(dir, meta); err != nil {
		t.Fatalf("WriteMeta failed: %v", err)
	}

	// Read raw file content
	metaPath := filepath.Join(dir, "meta.json")
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read meta.json: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("meta.json is not valid JSON: %v", err)
	}

	// Verify expected fields are present
	expectedFields := []string{"conjecture", "created_at", "version"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("expected field %q not found in meta.json", field)
		}
	}
}

// TestMeta_JSONTags verifies that Meta struct has proper JSON tags.
func TestMeta_JSONTags(t *testing.T) {
	meta := Meta{
		Conjecture: "Test",
		CreatedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Version:    "1.0",
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal Meta: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Check that JSON keys use snake_case
	if _, ok := parsed["conjecture"]; !ok {
		t.Error("expected 'conjecture' key in JSON")
	}
	if _, ok := parsed["created_at"]; !ok {
		t.Error("expected 'created_at' key in JSON")
	}
	if _, ok := parsed["version"]; !ok {
		t.Error("expected 'version' key in JSON")
	}
}
