package fs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// testData is a simple struct for testing JSON read/write.
type testData struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Count   int    `json:"count"`
	Enabled bool   `json:"enabled"`
}

func TestWriteJSON_Success(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.json")

	data := testData{
		ID:      "test-1",
		Name:    "Test Data",
		Count:   42,
		Enabled: true,
	}

	err := WriteJSON(filePath, &data)
	if err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("WriteJSON did not create file")
	}

	// Verify no temp file left behind
	tempPath := filePath + ".tmp"
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatal("WriteJSON left temp file behind")
	}
}

func TestWriteJSON_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "nested", "deep", "test.json")

	data := testData{ID: "test-1"}

	err := WriteJSON(filePath, &data)
	if err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("WriteJSON did not create file with nested dirs")
	}
}

func TestWriteJSON_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.json")

	// Write first version
	data1 := testData{ID: "v1", Name: "Original"}
	if err := WriteJSON(filePath, &data1); err != nil {
		t.Fatalf("First WriteJSON failed: %v", err)
	}

	// Write second version
	data2 := testData{ID: "v2", Name: "Updated"}
	if err := WriteJSON(filePath, &data2); err != nil {
		t.Fatalf("Second WriteJSON failed: %v", err)
	}

	// Read and verify it's the updated version
	var result testData
	if err := ReadJSON(filePath, &result); err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}

	if result.ID != "v2" || result.Name != "Updated" {
		t.Errorf("Expected v2/Updated, got %s/%s", result.ID, result.Name)
	}
}

func TestWriteJSON_MarshalError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.json")

	// Use a channel which cannot be marshaled to JSON
	ch := make(chan int)
	err := WriteJSON(filePath, ch)
	if err == nil {
		t.Fatal("Expected error marshaling channel, got nil")
	}
}

func TestReadJSON_Success(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.json")

	// Write test data
	original := testData{
		ID:      "test-read",
		Name:    "Read Test",
		Count:   100,
		Enabled: false,
	}
	if err := WriteJSON(filePath, &original); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	// Read it back
	var result testData
	if err := ReadJSON(filePath, &result); err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}

	if result.ID != original.ID {
		t.Errorf("ID mismatch: expected %s, got %s", original.ID, result.ID)
	}
	if result.Name != original.Name {
		t.Errorf("Name mismatch: expected %s, got %s", original.Name, result.Name)
	}
	if result.Count != original.Count {
		t.Errorf("Count mismatch: expected %d, got %d", original.Count, result.Count)
	}
	if result.Enabled != original.Enabled {
		t.Errorf("Enabled mismatch: expected %v, got %v", original.Enabled, result.Enabled)
	}
}

func TestReadJSON_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "nonexistent.json")

	var result testData
	err := ReadJSON(filePath, &result)
	if err == nil {
		t.Fatal("Expected error for nonexistent file, got nil")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected os.ErrNotExist, got %v", err)
	}
}

func TestReadJSON_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "invalid.json")

	// Write invalid JSON
	if err := os.WriteFile(filePath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	var result testData
	err := ReadJSON(filePath, &result)
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestReadJSON_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.json")

	// Write empty file
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to write empty file: %v", err)
	}

	var result testData
	err := ReadJSON(filePath, &result)
	if err == nil {
		t.Fatal("Expected error for empty file, got nil")
	}
}

func TestWriteJSON_ReadJSON_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "roundtrip.json")

	// Test various data types
	tests := []testData{
		{ID: "1", Name: "Simple", Count: 0, Enabled: false},
		{ID: "2", Name: "With spaces in name", Count: -10, Enabled: true},
		{ID: "special-chars_123", Name: "Special: \"quoted\" & <tagged>", Count: 999999, Enabled: true},
	}

	for _, original := range tests {
		t.Run(original.ID, func(t *testing.T) {
			if err := WriteJSON(filePath, &original); err != nil {
				t.Fatalf("WriteJSON failed: %v", err)
			}

			var result testData
			if err := ReadJSON(filePath, &result); err != nil {
				t.Fatalf("ReadJSON failed: %v", err)
			}

			if result != original {
				t.Errorf("Roundtrip mismatch: expected %+v, got %+v", original, result)
			}
		})
	}
}

func TestWriteJSON_WithSlice(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "slice.json")

	data := []testData{
		{ID: "1", Name: "First"},
		{ID: "2", Name: "Second"},
	}

	if err := WriteJSON(filePath, &data); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	var result []testData
	if err := ReadJSON(filePath, &result); err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(result))
	}
	if result[0].ID != "1" || result[1].ID != "2" {
		t.Error("Slice data mismatch")
	}
}

func TestWriteJSON_WithMap(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "map.json")

	data := map[string]int{
		"one":   1,
		"two":   2,
		"three": 3,
	}

	if err := WriteJSON(filePath, &data); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	var result map[string]int
	if err := ReadJSON(filePath, &result); err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}

	if result["one"] != 1 || result["two"] != 2 || result["three"] != 3 {
		t.Error("Map data mismatch")
	}
}

func TestWriteJSON_NilValue(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "nil.json")

	// Writing nil should produce "null" JSON
	err := WriteJSON(filePath, nil)
	if err != nil {
		t.Fatalf("WriteJSON with nil failed: %v", err)
	}

	// Verify file contains "null"
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("Expected 'null', got '%s'", string(data))
	}
}

// TestWriteJSON_MissingParentDirectory tests WriteJSON behavior when the parent
// directory doesn't exist and creation fails because a file blocks the path.
// This is an edge case where os.MkdirAll would fail because a file exists
// where a directory component should be.
func TestWriteJSON_MissingParentDirectory(t *testing.T) {
	t.Run("file_blocks_parent_directory_creation", func(t *testing.T) {
		dir := t.TempDir()

		// Create a file at what should be a directory in the path
		blockingFile := filepath.Join(dir, "parent")
		if err := os.WriteFile(blockingFile, []byte("blocking file"), 0644); err != nil {
			t.Fatalf("failed to create blocking file: %v", err)
		}

		// Try to write to a path where "parent" would need to be a directory
		targetPath := filepath.Join(dir, "parent", "child", "test.json")
		data := testData{ID: "test", Name: "Test"}

		err := WriteJSON(targetPath, &data)
		if err == nil {
			t.Error("expected error when parent path is blocked by a file, got nil")
		}

		// Verify the error is related to the path issue (not a directory)
		t.Logf("got expected error: %v", err)
	})

	t.Run("deeply_nested_missing_parents_success", func(t *testing.T) {
		dir := t.TempDir()

		// WriteJSON should successfully create all missing parent directories
		targetPath := filepath.Join(dir, "a", "b", "c", "d", "e", "test.json")
		data := testData{ID: "deep", Name: "Deep Test"}

		err := WriteJSON(targetPath, &data)
		if err != nil {
			t.Errorf("expected success creating deeply nested parents, got: %v", err)
		}

		// Verify file was written correctly
		var result testData
		if err := ReadJSON(targetPath, &result); err != nil {
			t.Errorf("failed to read back file: %v", err)
		}
		if result.ID != "deep" {
			t.Errorf("expected ID 'deep', got '%s'", result.ID)
		}
	})

	t.Run("parent_is_symlink_to_file", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipping symlink test on Windows")
		}

		dir := t.TempDir()

		// Create a file
		realFile := filepath.Join(dir, "realfile")
		if err := os.WriteFile(realFile, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		// Create a symlink to the file
		parentLink := filepath.Join(dir, "parentlink")
		if err := os.Symlink(realFile, parentLink); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Try to write where the symlink would need to be a directory
		targetPath := filepath.Join(parentLink, "child.json")
		data := testData{ID: "test", Name: "Test"}

		err := WriteJSON(targetPath, &data)
		if err == nil {
			t.Error("expected error when parent symlink points to file, got nil")
		}
		t.Logf("got expected error: %v", err)
	})

	t.Run("empty_path_error", func(t *testing.T) {
		// Empty path should fail
		data := testData{ID: "test", Name: "Test"}
		err := WriteJSON("", &data)
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})
}

// TestReadJSON_PathIsFile tests ReadJSON behavior when path components that
// should be directories are actually files, or when trying to read a directory.
// This edge case can produce confusing errors that should be handled gracefully.
func TestReadJSON_PathIsFile(t *testing.T) {
	t.Run("path_is_directory_not_file", func(t *testing.T) {
		dir := t.TempDir()

		// Try to read a directory as if it were a JSON file
		var result testData
		err := ReadJSON(dir, &result)
		if err == nil {
			t.Error("expected error when reading a directory, got nil")
		}

		// On Unix, reading a directory returns "is a directory" error
		// On Windows, it may return a different error
		t.Logf("got expected error when reading directory: %v", err)
	})

	t.Run("parent_path_component_is_file", func(t *testing.T) {
		dir := t.TempDir()

		// Create a regular file where a directory component should be
		blockingFile := filepath.Join(dir, "notadir")
		if err := os.WriteFile(blockingFile, []byte("I am a file"), 0644); err != nil {
			t.Fatalf("failed to create blocking file: %v", err)
		}

		// Try to read a path where "notadir" would need to be a directory
		targetPath := filepath.Join(dir, "notadir", "data.json")
		var result testData
		err := ReadJSON(targetPath, &result)
		if err == nil {
			t.Error("expected error when parent path component is a file, got nil")
		}

		// The error should indicate the path problem (not a directory, or file not found)
		t.Logf("got expected error when parent is file: %v", err)
	})

	t.Run("deeply_nested_file_blocks_path", func(t *testing.T) {
		dir := t.TempDir()

		// Create a deeply nested structure where one component is a file
		nestedDir := filepath.Join(dir, "a", "b")
		if err := os.MkdirAll(nestedDir, 0755); err != nil {
			t.Fatalf("failed to create nested directories: %v", err)
		}

		// Create a file at what should be directory "c"
		blockingFile := filepath.Join(nestedDir, "c")
		if err := os.WriteFile(blockingFile, []byte("blocking"), 0644); err != nil {
			t.Fatalf("failed to create blocking file: %v", err)
		}

		// Try to read through this blocked path
		targetPath := filepath.Join(dir, "a", "b", "c", "d", "data.json")
		var result testData
		err := ReadJSON(targetPath, &result)
		if err == nil {
			t.Error("expected error when nested path blocked by file, got nil")
		}
		t.Logf("got expected error for deeply nested blocked path: %v", err)
	})

	t.Run("symlink_to_directory", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipping symlink test on Windows")
		}

		dir := t.TempDir()

		// Create a subdirectory with a valid JSON file
		subdir := filepath.Join(dir, "realdir")
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}
		jsonFile := filepath.Join(subdir, "data.json")
		original := testData{ID: "symlink-test", Name: "Via Symlink"}
		if err := WriteJSON(jsonFile, &original); err != nil {
			t.Fatalf("failed to write JSON: %v", err)
		}

		// Create a symlink to the directory
		symlink := filepath.Join(dir, "link")
		if err := os.Symlink(subdir, symlink); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Read through the symlink - this should work
		targetPath := filepath.Join(symlink, "data.json")
		var result testData
		if err := ReadJSON(targetPath, &result); err != nil {
			t.Errorf("expected success reading through symlink to directory, got: %v", err)
		}
		if result.ID != "symlink-test" {
			t.Errorf("expected ID 'symlink-test', got '%s'", result.ID)
		}
	})

	t.Run("symlink_to_file_as_directory", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipping symlink test on Windows")
		}

		dir := t.TempDir()

		// Create a regular file
		realFile := filepath.Join(dir, "realfile")
		if err := os.WriteFile(realFile, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		// Create a symlink to the file
		symlink := filepath.Join(dir, "filelink")
		if err := os.Symlink(realFile, symlink); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Try to read a path through the symlink as if it were a directory
		targetPath := filepath.Join(symlink, "data.json")
		var result testData
		err := ReadJSON(targetPath, &result)
		if err == nil {
			t.Error("expected error when symlink to file used as directory, got nil")
		}
		t.Logf("got expected error for symlink-to-file as directory: %v", err)
	})
}
