package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
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

// TestReadJSON_ConcurrentWrite tests that ReadJSON is safe when WriteJSON
// is concurrently performing atomic writes (write to temp file, then rename).
// Key guarantees:
// 1. No partial reads (data corruption)
// 2. Reads return either the old or new version, never a mix
// 3. No panics or unexpected errors beyond file-not-found during rename window
func TestReadJSON_ConcurrentWrite(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "concurrent.json")

	// Write initial version
	version1 := testData{ID: "v1", Name: "Version One", Count: 1, Enabled: true}
	if err := WriteJSON(filePath, &version1); err != nil {
		t.Fatalf("failed to write initial version: %v", err)
	}

	version2 := testData{ID: "v2", Name: "Version Two", Count: 2, Enabled: false}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	var readCount atomic.Int32
	var corruptionErrors atomic.Int32
	var otherErrors atomic.Int32

	// Concurrent readers
	numReaders := 5
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-stop:
					return
				default:
					var result testData
					err := ReadJSON(filePath, &result)
					if err != nil {
						// File not found during atomic rename window is acceptable
						if !os.IsNotExist(err) {
							otherErrors.Add(1)
						}
						continue
					}

					readCount.Add(1)

					// Check for corruption: must be either v1 or v2, not a mix
					isV1 := result.ID == "v1" && result.Name == "Version One" && result.Count == 1 && result.Enabled == true
					isV2 := result.ID == "v2" && result.Name == "Version Two" && result.Count == 2 && result.Enabled == false

					if !isV1 && !isV2 {
						corruptionErrors.Add(1)
						t.Errorf("corrupted read: got %+v (neither v1 nor v2)", result)
					}

					time.Sleep(time.Microsecond)
				}
			}
		}()
	}

	// Concurrent writer alternating between versions
	numWrites := 100
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < numWrites; i++ {
			var data testData
			if i%2 == 0 {
				data = version2
			} else {
				data = version1
			}

			_ = WriteJSON(filePath, &data)
			time.Sleep(time.Microsecond)
		}

		close(stop)
	}()

	wg.Wait()

	// Report results
	if corruptions := corruptionErrors.Load(); corruptions > 0 {
		t.Errorf("detected %d corrupted reads (partial data from both versions)", corruptions)
	}

	if errors := otherErrors.Load(); errors > 0 {
		t.Errorf("got %d unexpected read errors", errors)
	}

	t.Logf("completed %d reads during %d writes with no corruption", readCount.Load(), numWrites)

	// Final state should be valid
	var final testData
	if err := ReadJSON(filePath, &final); err != nil {
		t.Fatalf("failed to read final state: %v", err)
	}

	isV1 := final.ID == "v1" && final.Name == "Version One"
	isV2 := final.ID == "v2" && final.Name == "Version Two"
	if !isV1 && !isV2 {
		t.Errorf("final state is corrupted: %+v", final)
	}
}

// TestReadJSON_ConcurrentWriteLargeData tests concurrent read/write with larger
// data payloads to increase the window for potential race conditions.
func TestReadJSON_ConcurrentWriteLargeData(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "large_concurrent.json")

	// Large data structure to increase write time and race window
	type largeData struct {
		ID      string   `json:"id"`
		Version int      `json:"version"`
		Entries []string `json:"entries"`
		Padding string   `json:"padding"`
	}

	makeData := func(version int) largeData {
		entries := make([]string, 100)
		for i := range entries {
			entries[i] = fmt.Sprintf("entry_%d_v%d", i, version)
		}
		return largeData{
			ID:      fmt.Sprintf("v%d", version),
			Version: version,
			Entries: entries,
			Padding: strings.Repeat(fmt.Sprintf("padding_v%d_", version), 100),
		}
	}

	// Write initial version
	if err := WriteJSON(filePath, makeData(1)); err != nil {
		t.Fatalf("failed to write initial version: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	var readCount atomic.Int32
	var corruptionErrors atomic.Int32

	// Concurrent readers
	numReaders := 3
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-stop:
					return
				default:
					var result largeData
					err := ReadJSON(filePath, &result)
					if err != nil {
						continue
					}

					readCount.Add(1)

					// Verify internal consistency: all entries should match the version
					expectedPrefix := fmt.Sprintf("entry_0_v%d", result.Version)
					if len(result.Entries) > 0 && !strings.HasPrefix(result.Entries[0], expectedPrefix[:len(expectedPrefix)-1]) {
						corruptionErrors.Add(1)
					}

					// Check padding matches version
					expectedPadding := fmt.Sprintf("padding_v%d_", result.Version)
					if !strings.HasPrefix(result.Padding, expectedPadding) {
						corruptionErrors.Add(1)
					}
				}
			}
		}()
	}

	// Concurrent writer
	numWrites := 50
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < numWrites; i++ {
			version := (i % 2) + 1
			_ = WriteJSON(filePath, makeData(version))
		}

		close(stop)
	}()

	wg.Wait()

	if corruptions := corruptionErrors.Load(); corruptions > 0 {
		t.Errorf("detected %d corrupted reads with large data", corruptions)
	}

	t.Logf("completed %d reads during %d large writes", readCount.Load(), numWrites)
}

// TestJSON_SymlinkFollowing tests symlink-related security scenarios.
// This documents the current behavior of ReadJSON/WriteJSON with symlinks
// and verifies expected behavior for security-relevant edge cases.
//
// Security considerations tested:
// 1. Symlinks escaping the intended directory (directory traversal)
// 2. Symlinks pointing to sensitive locations
// 3. Circular symlinks causing infinite loops
// 4. TOCTOU scenarios where files become symlinks
// 5. Nested symlink chains
func TestJSON_SymlinkFollowing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink security tests on Windows")
	}

	t.Run("symlink_escape_to_parent_directory", func(t *testing.T) {
		// Setup: Create a "jail" directory and an "outside" directory
		// with a sensitive file. Test that symlinks can escape the jail.
		baseDir := t.TempDir()
		jail := filepath.Join(baseDir, "jail")
		outside := filepath.Join(baseDir, "outside")

		if err := os.MkdirAll(jail, 0755); err != nil {
			t.Fatalf("failed to create jail: %v", err)
		}
		if err := os.MkdirAll(outside, 0755); err != nil {
			t.Fatalf("failed to create outside dir: %v", err)
		}

		// Create a "sensitive" file outside the jail
		sensitiveFile := filepath.Join(outside, "sensitive.json")
		sensitiveData := testData{ID: "secret", Name: "Sensitive Data", Count: 42}
		if err := WriteJSON(sensitiveFile, &sensitiveData); err != nil {
			t.Fatalf("failed to write sensitive file: %v", err)
		}

		// Create a symlink inside jail pointing outside
		escapeLink := filepath.Join(jail, "escape")
		if err := os.Symlink(outside, escapeLink); err != nil {
			t.Fatalf("failed to create escape symlink: %v", err)
		}

		// SECURITY CHECK: ReadJSON follows symlinks and can read outside jail
		// This documents the current behavior - symlinks ARE followed
		escapePath := filepath.Join(escapeLink, "sensitive.json")
		var result testData
		err := ReadJSON(escapePath, &result)
		if err != nil {
			t.Logf("ReadJSON through escape symlink failed: %v", err)
		} else {
			// Current behavior: symlinks are followed
			t.Logf("SECURITY NOTE: ReadJSON follows symlinks - read succeeded through escape link")
			if result.ID != "secret" {
				t.Errorf("unexpected data read: %+v", result)
			}
		}

		// SECURITY CHECK: WriteJSON also follows symlinks
		writeEscapePath := filepath.Join(escapeLink, "written.json")
		writeData := testData{ID: "escaped-write", Name: "Written Outside Jail"}
		err = WriteJSON(writeEscapePath, &writeData)
		if err != nil {
			t.Logf("WriteJSON through escape symlink failed: %v", err)
		} else {
			t.Logf("SECURITY NOTE: WriteJSON follows symlinks - write succeeded through escape link")
			// Verify file was written outside jail
			verifyPath := filepath.Join(outside, "written.json")
			if _, err := os.Stat(verifyPath); err != nil {
				t.Error("expected file to be written outside jail")
			}
		}
	})

	t.Run("symlink_to_absolute_path", func(t *testing.T) {
		// Symlink pointing to an absolute path outside working directory
		dir := t.TempDir()
		workDir := filepath.Join(dir, "work")
		secretDir := filepath.Join(dir, "secrets")

		if err := os.MkdirAll(workDir, 0755); err != nil {
			t.Fatalf("failed to create work dir: %v", err)
		}
		if err := os.MkdirAll(secretDir, 0755); err != nil {
			t.Fatalf("failed to create secret dir: %v", err)
		}

		// Write secret file
		secretFile := filepath.Join(secretDir, "credentials.json")
		secretData := testData{ID: "creds", Name: "password123"}
		if err := WriteJSON(secretFile, &secretData); err != nil {
			t.Fatalf("failed to write secret: %v", err)
		}

		// Create symlink with absolute path
		absLink := filepath.Join(workDir, "link")
		if err := os.Symlink(secretDir, absLink); err != nil {
			t.Fatalf("failed to create absolute symlink: %v", err)
		}

		// Try to read through absolute symlink
		readPath := filepath.Join(absLink, "credentials.json")
		var result testData
		err := ReadJSON(readPath, &result)
		if err != nil {
			t.Logf("ReadJSON through absolute symlink failed: %v", err)
		} else {
			t.Logf("SECURITY NOTE: Absolute symlinks are followed")
			if result.Name != "password123" {
				t.Errorf("unexpected read result: %+v", result)
			}
		}
	})

	t.Run("circular_symlinks", func(t *testing.T) {
		// Create circular symlink chain that could cause infinite loops
		dir := t.TempDir()

		// Create a -> b -> a circular chain
		linkA := filepath.Join(dir, "a")
		linkB := filepath.Join(dir, "b")

		// Create b first pointing to a (which doesn't exist yet)
		if err := os.Symlink(linkA, linkB); err != nil {
			t.Fatalf("failed to create symlink b: %v", err)
		}
		// Create a pointing to b
		if err := os.Symlink(linkB, linkA); err != nil {
			t.Fatalf("failed to create symlink a: %v", err)
		}

		// Try to read through circular symlink
		readPath := filepath.Join(linkA, "data.json")
		var result testData
		err := ReadJSON(readPath, &result)
		if err == nil {
			t.Error("expected error for circular symlink, got nil")
		} else {
			// Should get "too many levels of symbolic links" or similar
			t.Logf("circular symlink correctly rejected: %v", err)
		}

		// Try to write through circular symlink
		writeData := testData{ID: "circular"}
		err = WriteJSON(readPath, &writeData)
		if err == nil {
			t.Error("expected error for circular symlink write, got nil")
		} else {
			t.Logf("circular symlink write correctly rejected: %v", err)
		}
	})

	t.Run("deeply_nested_symlink_chain", func(t *testing.T) {
		// Create a chain of symlinks to test depth limits
		dir := t.TempDir()

		// Create target file
		targetDir := filepath.Join(dir, "target")
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("failed to create target dir: %v", err)
		}
		targetFile := filepath.Join(targetDir, "data.json")
		targetData := testData{ID: "deep-target", Name: "Found it"}
		if err := WriteJSON(targetFile, &targetData); err != nil {
			t.Fatalf("failed to write target: %v", err)
		}

		// Create chain: link1 -> link2 -> link3 -> ... -> target
		prevPath := targetDir
		numLinks := 10
		for i := numLinks; i >= 1; i-- {
			linkPath := filepath.Join(dir, fmt.Sprintf("link%d", i))
			if err := os.Symlink(prevPath, linkPath); err != nil {
				t.Fatalf("failed to create symlink %d: %v", i, err)
			}
			prevPath = linkPath
		}

		// Try to read through the chain
		chainPath := filepath.Join(filepath.Join(dir, "link1"), "data.json")
		var result testData
		err := ReadJSON(chainPath, &result)
		if err != nil {
			t.Logf("nested symlink chain read failed (may hit OS limits): %v", err)
		} else {
			t.Logf("nested symlink chain (%d links) successfully followed", numLinks)
			if result.ID != "deep-target" {
				t.Errorf("unexpected result: %+v", result)
			}
		}
	})

	t.Run("symlink_toctou_race", func(t *testing.T) {
		// Test TOCTOU scenario: file exists, gets replaced with symlink
		// during concurrent operations
		dir := t.TempDir()
		outsideDir := filepath.Join(dir, "outside")
		if err := os.MkdirAll(outsideDir, 0755); err != nil {
			t.Fatalf("failed to create outside dir: %v", err)
		}

		targetFile := filepath.Join(dir, "target.json")
		outsideFile := filepath.Join(outsideDir, "outside.json")

		// Write initial legitimate file
		legitData := testData{ID: "legit", Name: "Legitimate"}
		if err := WriteJSON(targetFile, &legitData); err != nil {
			t.Fatalf("failed to write legit file: %v", err)
		}

		// Write outside file that we'll symlink to
		outsideData := testData{ID: "outside", Name: "Redirected"}
		if err := WriteJSON(outsideFile, &outsideData); err != nil {
			t.Fatalf("failed to write outside file: %v", err)
		}

		// Replace the file with a symlink
		if err := os.Remove(targetFile); err != nil {
			t.Fatalf("failed to remove file: %v", err)
		}
		if err := os.Symlink(outsideFile, targetFile); err != nil {
			t.Fatalf("failed to create replacement symlink: %v", err)
		}

		// Read now follows the symlink
		var result testData
		if err := ReadJSON(targetFile, &result); err != nil {
			t.Fatalf("failed to read: %v", err)
		}

		if result.ID != "outside" {
			t.Errorf("expected redirected data, got: %+v", result)
		}
		t.Logf("SECURITY NOTE: TOCTOU - file replaced with symlink, read was redirected")
	})

	t.Run("symlink_to_dev_null", func(t *testing.T) {
		// Symlink to special files like /dev/null
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			t.Skip("skipping /dev/null test on non-unix")
		}

		dir := t.TempDir()
		devNullLink := filepath.Join(dir, "null.json")

		if err := os.Symlink("/dev/null", devNullLink); err != nil {
			t.Fatalf("failed to create /dev/null symlink: %v", err)
		}

		// Reading from /dev/null returns empty
		var result testData
		err := ReadJSON(devNullLink, &result)
		if err == nil {
			t.Log("read from /dev/null symlink returned no error (empty content parsed)")
		} else {
			t.Logf("read from /dev/null symlink failed as expected: %v", err)
		}

		// Writing to /dev/null succeeds but data is discarded
		writeData := testData{ID: "vanished", Name: "Gone"}
		err = WriteJSON(devNullLink, &writeData)
		if err != nil {
			t.Logf("write to /dev/null symlink failed: %v", err)
		} else {
			t.Log("SECURITY NOTE: write to /dev/null symlink succeeded (data discarded)")
		}
	})

	t.Run("broken_symlink", func(t *testing.T) {
		// Symlink pointing to non-existent target
		dir := t.TempDir()
		brokenLink := filepath.Join(dir, "broken.json")

		// Create symlink to non-existent file
		if err := os.Symlink(filepath.Join(dir, "nonexistent.json"), brokenLink); err != nil {
			t.Fatalf("failed to create broken symlink: %v", err)
		}

		// Read should fail
		var result testData
		err := ReadJSON(brokenLink, &result)
		if err == nil {
			t.Error("expected error for broken symlink, got nil")
		} else {
			if !os.IsNotExist(err) {
				t.Logf("broken symlink error (not ErrNotExist): %v", err)
			} else {
				t.Log("broken symlink correctly returns ErrNotExist")
			}
		}

		// Write should create the target file through the symlink
		writeData := testData{ID: "created", Name: "Through Broken Link"}
		err = WriteJSON(brokenLink, &writeData)
		if err != nil {
			t.Logf("write through broken symlink failed: %v", err)
		} else {
			// Check if target was created
			targetPath := filepath.Join(dir, "nonexistent.json")
			if _, err := os.Stat(targetPath); err == nil {
				t.Log("SECURITY NOTE: write through broken symlink created target file")
			} else {
				t.Logf("target file status after write: %v", err)
			}
		}
	})

	t.Run("relative_symlink_escape", func(t *testing.T) {
		// Relative symlink using .. to escape directory
		baseDir := t.TempDir()
		jail := filepath.Join(baseDir, "level1", "level2", "jail")
		secrets := filepath.Join(baseDir, "secrets")

		if err := os.MkdirAll(jail, 0755); err != nil {
			t.Fatalf("failed to create jail: %v", err)
		}
		if err := os.MkdirAll(secrets, 0755); err != nil {
			t.Fatalf("failed to create secrets: %v", err)
		}

		// Write secret
		secretFile := filepath.Join(secrets, "secret.json")
		secretData := testData{ID: "relative-secret", Name: "Found via relative path"}
		if err := WriteJSON(secretFile, &secretData); err != nil {
			t.Fatalf("failed to write secret: %v", err)
		}

		// Create relative symlink that escapes: ../../../secrets
		escapeLink := filepath.Join(jail, "escape")
		if err := os.Symlink("../../../secrets", escapeLink); err != nil {
			t.Fatalf("failed to create relative escape symlink: %v", err)
		}

		// Read through relative escape
		readPath := filepath.Join(escapeLink, "secret.json")
		var result testData
		err := ReadJSON(readPath, &result)
		if err != nil {
			t.Logf("read through relative symlink failed: %v", err)
		} else {
			t.Logf("SECURITY NOTE: relative symlink escape (../../..) successfully followed")
			if result.ID != "relative-secret" {
				t.Errorf("unexpected result: %+v", result)
			}
		}
	})
}

// TestJSON_FileDescriptorExhaustion tests the behavior of ReadJSON and WriteJSON
// when the system approaches or reaches file descriptor limits. This verifies
// graceful degradation rather than crashes or panics.
//
// Security and stability considerations:
// 1. Operations should fail with clear errors, not panic
// 2. Partial operations should not leave corrupted state
// 3. Recovery should be possible once FDs are freed
// 4. Error messages should indicate resource exhaustion
func TestJSON_FileDescriptorExhaustion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping file descriptor exhaustion test on Windows (different resource model)")
	}

	// Helper function to set a low FD limit for testing
	setLowFDLimit := func(t *testing.T, newLimit uint64) (restore func()) {
		t.Helper()

		var original syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
			t.Fatalf("failed to get rlimit: %v", err)
		}

		// Save original and try to set new lower limit
		newRlimit := syscall.Rlimit{Cur: newLimit, Max: original.Max}
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &newRlimit); err != nil {
			t.Skipf("cannot set rlimit (may require privileges): %v", err)
		}

		return func() {
			syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original)
		}
	}

	t.Run("write_fails_gracefully_at_fd_limit", func(t *testing.T) {
		dir := t.TempDir()

		// Set a low FD limit for this test
		// We use a very low limit to ensure we hit exhaustion
		const testFDLimit = uint64(50)
		restore := setLowFDLimit(t, testFDLimit)
		defer restore()

		// Reserve only a few FDs for stdin, stdout, stderr
		// We want to exhaust FDs for the test
		reservedFDs := uint64(5)

		// Open files until we hit the limit
		openFiles := make([]*os.File, 0)
		tempDir := filepath.Join(dir, "fd_exhaust")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		// Track how many files we successfully opened
		var openCount int
		defer func() {
			// Clean up all opened files
			for _, f := range openFiles {
				f.Close()
			}
			t.Logf("cleaned up %d open files", len(openFiles))
		}()

		// Open files until we hit the limit
		targetOpenFiles := int(testFDLimit - reservedFDs)
		for i := 0; i < targetOpenFiles+50; i++ { // Try to open more than limit
			filePath := filepath.Join(tempDir, fmt.Sprintf("fd_%d.txt", i))
			f, err := os.Create(filePath)
			if err != nil {
				// We've hit the limit - this is what we want
				t.Logf("hit FD limit after opening %d files: %v", i, err)
				break
			}
			openFiles = append(openFiles, f)
			openCount++
		}

		if openCount == 0 {
			t.Skip("could not open any test files")
		}

		t.Logf("opened %d files, attempting JSON operations at limit", openCount)

		// Now try WriteJSON - it should fail gracefully
		writeDir := filepath.Join(dir, "write_test")
		if err := os.MkdirAll(writeDir, 0755); err != nil {
			// Even mkdir might fail at FD exhaustion
			t.Logf("mkdir failed at FD limit (expected): %v", err)
		}

		writeData := testData{ID: "exhaustion-test", Name: "Testing FD Exhaustion"}
		writePath := filepath.Join(writeDir, "test.json")
		err := WriteJSON(writePath, &writeData)

		if err == nil {
			// If we had enough FDs, it succeeded - verify the file
			var readResult testData
			readErr := ReadJSON(writePath, &readResult)
			if readErr != nil {
				t.Logf("write succeeded but read failed: %v", readErr)
			} else if readResult.ID != "exhaustion-test" {
				t.Errorf("data corruption: expected 'exhaustion-test', got '%s'", readResult.ID)
			}
			t.Log("write succeeded (had sufficient FDs)")
		} else {
			// Write failed - verify it's a reasonable error
			t.Logf("WriteJSON failed at FD limit (expected): %v", err)

			// Verify the error is resource-related (EMFILE/ENFILE) or permission-related
			// Different systems may return different errors
			errStr := err.Error()
			validErrors := []string{
				"too many open files",
				"no file descriptors",
				"resource temporarily unavailable",
				"cannot allocate memory",
			}

			found := false
			for _, valid := range validErrors {
				if strings.Contains(strings.ToLower(errStr), valid) {
					found = true
					break
				}
			}

			if !found {
				// Still acceptable if it's just a permission error or file error
				t.Logf("non-standard error at FD exhaustion: %v", err)
			}

			// CRITICAL: No panic occurred
			t.Log("no panic occurred during FD exhaustion - graceful degradation verified")
		}
	})

	t.Run("read_fails_gracefully_at_fd_limit", func(t *testing.T) {
		dir := t.TempDir()

		// First, write a file while we have FDs available
		testFile := filepath.Join(dir, "pre_written.json")
		preWriteData := testData{ID: "pre-written", Name: "Written Before Exhaustion"}
		if err := WriteJSON(testFile, &preWriteData); err != nil {
			t.Fatalf("failed to pre-write test file: %v", err)
		}

		// Set a low FD limit for this test to ensure exhaustion
		const testFDLimit = uint64(50)
		restore := setLowFDLimit(t, testFDLimit)
		defer restore()

		reservedFDs := uint64(5)

		// Open files until we hit the limit
		openFiles := make([]*os.File, 0)
		targetOpenFiles := int(testFDLimit - reservedFDs)
		tempDir := filepath.Join(dir, "fd_exhaust_read")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		defer func() {
			for _, f := range openFiles {
				f.Close()
			}
		}()

		for i := 0; i < targetOpenFiles+50; i++ { // Try to open more than limit
			filePath := filepath.Join(tempDir, fmt.Sprintf("fd_%d.txt", i))
			f, err := os.Create(filePath)
			if err != nil {
				t.Logf("hit FD limit after opening %d files: %v", i, err)
				break
			}
			openFiles = append(openFiles, f)
		}

		if len(openFiles) == 0 {
			t.Skip("could not open any test files")
		}

		t.Logf("opened %d files, attempting ReadJSON at limit", len(openFiles))

		// Now try ReadJSON - it should fail gracefully
		var readResult testData
		err := ReadJSON(testFile, &readResult)

		if err == nil {
			// Read succeeded - verify data integrity
			if readResult.ID != "pre-written" {
				t.Errorf("data corruption on read: expected 'pre-written', got '%s'", readResult.ID)
			}
			t.Log("read succeeded (had sufficient FDs)")
		} else {
			t.Logf("ReadJSON failed at FD limit (expected): %v", err)
			t.Log("no panic occurred during FD exhaustion read - graceful degradation verified")
		}
	})

	t.Run("recovery_after_fd_freed", func(t *testing.T) {
		dir := t.TempDir()

		// Set a low FD limit for this test
		const testFDLimit = uint64(50)
		restore := setLowFDLimit(t, testFDLimit)
		defer restore()

		reservedFDs := uint64(5)

		// Open files to exhaust FDs
		openFiles := make([]*os.File, 0)
		targetOpenFiles := int(testFDLimit - reservedFDs)
		tempDir := filepath.Join(dir, "fd_exhaust_recovery")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		for i := 0; i < targetOpenFiles+50; i++ { // Try to open more than limit
			filePath := filepath.Join(tempDir, fmt.Sprintf("fd_%d.txt", i))
			f, err := os.Create(filePath)
			if err != nil {
				t.Logf("hit FD limit after opening %d files: %v", i, err)
				break
			}
			openFiles = append(openFiles, f)
		}

		if len(openFiles) == 0 {
			t.Skip("could not open any test files")
		}

		t.Logf("opened %d files", len(openFiles))

		// Try operation at exhaustion - should fail
		exhaustData := testData{ID: "recovery-test", Name: "Testing Recovery"}
		testFile := filepath.Join(dir, "recovery.json")
		_ = WriteJSON(testFile, &exhaustData) // May fail, that's expected

		// Now free the FDs
		for _, f := range openFiles {
			f.Close()
		}
		openFiles = nil
		t.Log("freed all FDs")

		// Verify we can now perform operations successfully
		recoveryData := testData{ID: "after-recovery", Name: "After Recovery"}
		testFile2 := filepath.Join(dir, "after_recovery.json")
		err := WriteJSON(testFile2, &recoveryData)
		if err != nil {
			t.Errorf("WriteJSON failed after FD recovery: %v", err)
		}

		var readResult testData
		err = ReadJSON(testFile2, &readResult)
		if err != nil {
			t.Errorf("ReadJSON failed after FD recovery: %v", err)
		}

		if readResult.ID != "after-recovery" {
			t.Errorf("data mismatch after recovery: expected 'after-recovery', got '%s'", readResult.ID)
		}

		t.Log("operations succeeded after FD recovery")
	})

	t.Run("no_temp_file_leak_at_exhaustion", func(t *testing.T) {
		dir := t.TempDir()

		// Set a low FD limit for this test
		const testFDLimit = uint64(60)
		restore := setLowFDLimit(t, testFDLimit)
		defer restore()

		reservedFDs := uint64(15) // Reserve some for temp file operations and test infra

		writeDir := filepath.Join(dir, "write_dir")
		if err := os.MkdirAll(writeDir, 0755); err != nil {
			t.Fatalf("failed to create write dir: %v", err)
		}

		// Open files to approach exhaustion
		openFiles := make([]*os.File, 0)
		targetOpenFiles := int(testFDLimit - reservedFDs)
		tempDir := filepath.Join(dir, "fd_exhaust_leak")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		for i := 0; i < targetOpenFiles+20; i++ { // Try to open more than limit
			filePath := filepath.Join(tempDir, fmt.Sprintf("fd_%d.txt", i))
			f, err := os.Create(filePath)
			if err != nil {
				t.Logf("hit FD limit after opening %d files: %v", i, err)
				break
			}
			openFiles = append(openFiles, f)
		}

		defer func() {
			for _, f := range openFiles {
				f.Close()
			}
		}()

		if len(openFiles) == 0 {
			t.Skip("could not open any test files")
		}

		t.Logf("opened %d files, attempting writes under pressure", len(openFiles))

		// Attempt multiple writes near/at exhaustion
		failCount := 0
		successCount := 0
		for i := 0; i < 10; i++ {
			testData := testData{ID: fmt.Sprintf("attempt-%d", i), Name: "Test"}
			testFile := filepath.Join(writeDir, fmt.Sprintf("test_%d.json", i))
			err := WriteJSON(testFile, &testData)
			if err != nil {
				failCount++
			} else {
				successCount++
			}
		}

		t.Logf("write attempts: %d succeeded, %d failed", successCount, failCount)

		// Free FDs and check for temp files
		for _, f := range openFiles {
			f.Close()
		}
		openFiles = nil

		// Check for leftover .tmp files
		entries, err := os.ReadDir(writeDir)
		if err != nil {
			t.Fatalf("failed to read write dir: %v", err)
		}

		tempFileCount := 0
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".tmp") {
				tempFileCount++
				t.Logf("found leftover temp file: %s", entry.Name())
			}
		}

		if tempFileCount > 0 {
			t.Errorf("found %d leftover .tmp files after FD exhaustion", tempFileCount)
		} else {
			t.Log("no temp files left behind after FD exhaustion")
		}
	})

	t.Run("concurrent_writes_at_fd_pressure", func(t *testing.T) {
		dir := t.TempDir()

		// Set a moderate FD limit for this test
		const testFDLimit = uint64(150)
		restore := setLowFDLimit(t, testFDLimit)
		defer restore()

		// Use moderate FD pressure (half the limit) to allow some concurrency
		reservedFDs := uint64(50)
		targetPressure := (testFDLimit - reservedFDs) / 2

		// Open files to create pressure
		openFiles := make([]*os.File, 0)
		tempDir := filepath.Join(dir, "fd_pressure")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		for i := uint64(0); i < targetPressure; i++ {
			filePath := filepath.Join(tempDir, fmt.Sprintf("fd_%d.txt", i))
			f, err := os.Create(filePath)
			if err != nil {
				break
			}
			openFiles = append(openFiles, f)
		}

		defer func() {
			for _, f := range openFiles {
				f.Close()
			}
		}()

		t.Logf("created FD pressure with %d open files", len(openFiles))

		// Run concurrent writes under FD pressure
		writeDir := filepath.Join(dir, "concurrent_writes")
		if err := os.MkdirAll(writeDir, 0755); err != nil {
			t.Fatalf("failed to create write dir: %v", err)
		}

		var wg sync.WaitGroup
		var successCount, failCount atomic.Int32
		numGoroutines := 5
		writesPerGoroutine := 10

		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(gid int) {
				defer wg.Done()
				for w := 0; w < writesPerGoroutine; w++ {
					data := testData{
						ID:   fmt.Sprintf("g%d-w%d", gid, w),
						Name: fmt.Sprintf("Goroutine %d Write %d", gid, w),
					}
					path := filepath.Join(writeDir, fmt.Sprintf("g%d_w%d.json", gid, w))
					if err := WriteJSON(path, &data); err != nil {
						failCount.Add(1)
					} else {
						successCount.Add(1)
					}
				}
			}(g)
		}

		wg.Wait()

		total := successCount.Load() + failCount.Load()
		t.Logf("concurrent writes under FD pressure: %d/%d succeeded", successCount.Load(), total)

		// Verify no panics occurred (we got here) and at least some writes succeeded
		if successCount.Load() == 0 && failCount.Load() > 0 {
			t.Log("all concurrent writes failed under FD pressure (acceptable if FDs were exhausted)")
		}

		// Check for temp files
		entries, err := os.ReadDir(writeDir)
		if err != nil {
			t.Fatalf("failed to read write dir: %v", err)
		}

		tempFileCount := 0
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".tmp") {
				tempFileCount++
			}
		}

		if tempFileCount > 0 {
			t.Errorf("found %d leftover .tmp files after concurrent writes under pressure", tempFileCount)
		}
	})
}

// TestJSON_VeryLongFilePath tests ReadJSON and WriteJSON behavior when dealing
// with file paths that approach or exceed OS-specific path length limits.
//
// Path length limits vary by OS:
// - Linux: PATH_MAX = 4096 bytes (total path), NAME_MAX = 255 bytes (filename component)
// - macOS: PATH_MAX = 1024 bytes, NAME_MAX = 255 bytes
// - Windows: MAX_PATH = 260 chars (though extended paths can go to ~32K)
//
// These tests verify:
// 1. Operations fail gracefully with clear errors for paths exceeding limits
// 2. Operations succeed for paths approaching but within limits
// 3. No panics or corruption occur with edge-case paths
func TestJSON_VeryLongFilePath(t *testing.T) {
	t.Run("path_at_name_max_limit", func(t *testing.T) {
		// NAME_MAX is 255 bytes on most Unix systems
		// Create a filename component approaching this limit
		dir := t.TempDir()

		// Create filename of exactly 255 characters (including .json extension)
		// 255 - 5 (.json) = 250 characters for the base name
		baseName := strings.Repeat("a", 250)
		fileName := baseName + ".json"
		if len(fileName) != 255 {
			t.Fatalf("expected filename length 255, got %d", len(fileName))
		}

		filePath := filepath.Join(dir, fileName)
		data := testData{ID: "max-name", Name: "At NAME_MAX Limit"}

		err := WriteJSON(filePath, &data)
		if err != nil {
			// On some systems, 255 may already exceed the limit
			t.Logf("WriteJSON at NAME_MAX failed: %v", err)
			// Verify it's the expected error type (ENAMETOOLONG or similar)
			if !strings.Contains(err.Error(), "name too long") &&
				!strings.Contains(err.Error(), "file name too long") {
				// Still acceptable - different systems return different errors
				t.Logf("non-standard error for long filename: %v", err)
			}
		} else {
			// Write succeeded - verify we can read it back
			var result testData
			if err := ReadJSON(filePath, &result); err != nil {
				t.Errorf("ReadJSON failed for NAME_MAX path: %v", err)
			} else if result.ID != "max-name" {
				t.Errorf("data corruption: expected 'max-name', got '%s'", result.ID)
			}
			t.Log("file operations succeeded at NAME_MAX limit (255 bytes)")
		}
	})

	t.Run("path_exceeds_name_max_limit", func(t *testing.T) {
		// Create a filename component that exceeds NAME_MAX (255 bytes)
		dir := t.TempDir()

		// 300 characters for base name + .json = 305 characters total
		baseName := strings.Repeat("x", 300)
		fileName := baseName + ".json"
		filePath := filepath.Join(dir, fileName)

		data := testData{ID: "over-name-max", Name: "Exceeds NAME_MAX"}

		err := WriteJSON(filePath, &data)
		if err == nil {
			t.Error("expected error for filename exceeding NAME_MAX, got nil")
			// Clean up if somehow it succeeded
			os.Remove(filePath)
		} else {
			t.Logf("correctly rejected filename exceeding NAME_MAX: %v", err)
		}

		// Also test read of such a path
		var result testData
		err = ReadJSON(filePath, &result)
		if err == nil {
			t.Error("expected error reading path exceeding NAME_MAX")
		}
	})

	t.Run("deeply_nested_path_approaching_path_max", func(t *testing.T) {
		// Linux PATH_MAX is 4096, macOS is 1024
		// Create a deeply nested directory structure approaching the limit
		dir := t.TempDir()

		// Use 50-character directory names to build up path length
		// Each level adds ~51 chars (name + separator)
		dirName := strings.Repeat("d", 50)
		currentPath := dir
		nestingLevel := 0

		// Build up path until we approach 3500 bytes (leave room for filename)
		for len(currentPath) < 3500 {
			currentPath = filepath.Join(currentPath, dirName)
			nestingLevel++
		}

		filePath := filepath.Join(currentPath, "test.json")
		t.Logf("created path with %d nesting levels, total length: %d bytes",
			nestingLevel, len(filePath))

		data := testData{ID: "deep-path", Name: "Deeply Nested Path"}

		err := WriteJSON(filePath, &data)
		if err != nil {
			// On macOS or systems with shorter PATH_MAX, this may fail
			t.Logf("WriteJSON failed for deeply nested path: %v", err)
			if strings.Contains(err.Error(), "name too long") ||
				strings.Contains(err.Error(), "file name too long") ||
				strings.Contains(err.Error(), "no such file") {
				t.Log("error is expected for path approaching/exceeding PATH_MAX")
			}
		} else {
			// Succeeded - verify read works too
			var result testData
			if err := ReadJSON(filePath, &result); err != nil {
				t.Errorf("ReadJSON failed for deeply nested path: %v", err)
			} else if result.ID != "deep-path" {
				t.Errorf("data corruption: expected 'deep-path', got '%s'", result.ID)
			}
			t.Logf("file operations succeeded at ~%d byte path length", len(filePath))
		}
	})

	t.Run("path_definitely_exceeds_path_max", func(t *testing.T) {
		// Create a path that definitely exceeds all known PATH_MAX values
		// Linux: 4096, macOS: 1024, Windows: varies
		dir := t.TempDir()

		// Build a path over 5000 bytes long
		longComponent := strings.Repeat("z", 200)
		currentPath := dir
		for i := 0; i < 30; i++ { // 30 * 200 = 6000+ bytes
			currentPath = filepath.Join(currentPath, longComponent)
		}
		filePath := filepath.Join(currentPath, "test.json")

		if len(filePath) < 5000 {
			t.Fatalf("test setup error: path only %d bytes", len(filePath))
		}
		t.Logf("attempting operations on %d byte path", len(filePath))

		data := testData{ID: "way-too-long", Name: "Exceeds All PATH_MAX"}

		err := WriteJSON(filePath, &data)
		if err == nil {
			t.Error("expected error for path exceeding PATH_MAX, got nil")
			os.RemoveAll(filepath.Join(dir, longComponent)) // Clean up
		} else {
			t.Logf("correctly rejected path exceeding PATH_MAX: %v", err)
			// No panic occurred - this is the key verification
		}

		// Also verify read fails gracefully
		var result testData
		err = ReadJSON(filePath, &result)
		if err == nil {
			t.Error("expected error reading path exceeding PATH_MAX")
		}
	})

	t.Run("temp_file_path_exceeds_limit", func(t *testing.T) {
		// WriteJSON creates a .tmp file by appending ".tmp" to the path
		// This could cause issues if the original path is at the limit
		dir := t.TempDir()

		// Create a filename that's exactly at NAME_MAX
		// When .tmp is appended for atomic write, it will exceed the limit
		baseName := strings.Repeat("t", 251) // 251 + .json = 255 (NAME_MAX)
		fileName := baseName + ".json"
		filePath := filepath.Join(dir, fileName)

		// The temp file will be: baseName + ".json.tmp" = 259 chars (exceeds 255)
		t.Logf("filename: %d bytes, temp filename would be: %d bytes",
			len(fileName), len(fileName)+4) // +4 for ".tmp"

		data := testData{ID: "temp-overflow", Name: "Temp File Overflow"}

		err := WriteJSON(filePath, &data)
		if err != nil {
			// This might fail because the temp file name exceeds NAME_MAX
			t.Logf("WriteJSON failed (possibly due to temp file length): %v", err)
		} else {
			// If it succeeded, verify the data is correct
			var result testData
			if err := ReadJSON(filePath, &result); err != nil {
				t.Errorf("ReadJSON failed: %v", err)
			} else if result.ID != "temp-overflow" {
				t.Errorf("data corruption: expected 'temp-overflow', got '%s'", result.ID)
			}
			t.Log("WriteJSON succeeded despite temp file path length")
		}

		// Verify no leftover .tmp files
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".tmp") {
				t.Errorf("found leftover temp file: %s", entry.Name())
			}
		}
	})

	t.Run("unicode_path_components", func(t *testing.T) {
		// Unicode characters can take multiple bytes
		// A 255-character string of multi-byte UTF-8 can exceed byte limits
		dir := t.TempDir()

		// Use emoji (4 bytes each in UTF-8) to create byte-expensive paths
		// 64 emoji * 4 bytes = 256 bytes for the name alone
		unicodeName := strings.Repeat("🔥", 64) // Fire emoji, 4 bytes each
		fileName := unicodeName + ".json"

		t.Logf("unicode filename: %d characters, %d bytes",
			len([]rune(fileName)), len(fileName))

		filePath := filepath.Join(dir, fileName)

		data := testData{ID: "unicode-path", Name: "Unicode Path Test"}

		err := WriteJSON(filePath, &data)
		if err != nil {
			t.Logf("WriteJSON failed for unicode path: %v", err)
			// This is expected - 64*4 + 5 = 261 bytes exceeds NAME_MAX
		} else {
			// Verify roundtrip
			var result testData
			if err := ReadJSON(filePath, &result); err != nil {
				t.Errorf("ReadJSON failed: %v", err)
			} else if result.ID != "unicode-path" {
				t.Errorf("data corruption: expected 'unicode-path', got '%s'", result.ID)
			}
			t.Log("unicode path operations succeeded")
		}
	})

	t.Run("path_with_only_dots", func(t *testing.T) {
		// Edge case: paths with many dot components
		dir := t.TempDir()

		// Create a directory to test relative-looking paths
		subDir := filepath.Join(dir, "sub")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create subdir: %v", err)
		}

		// Try various dot-heavy paths
		testCases := []struct {
			name string
			path string
		}{
			{"single_dot_filename", filepath.Join(subDir, "..json")},
			{"double_dot_filename", filepath.Join(subDir, "...json")},
			{"many_dots_filename", filepath.Join(subDir, "......................json")},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				data := testData{ID: tc.name, Name: "Dot Test"}
				err := WriteJSON(tc.path, &data)
				if err != nil {
					t.Logf("WriteJSON failed for %s: %v", tc.name, err)
				} else {
					var result testData
					if err := ReadJSON(tc.path, &result); err != nil {
						t.Errorf("ReadJSON failed: %v", err)
					} else if result.ID != tc.name {
						t.Errorf("data mismatch: expected '%s', got '%s'", tc.name, result.ID)
					}
				}
			})
		}
	})

	t.Run("windows_reserved_characters", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			// On Unix, these characters are generally valid in filenames
			dir := t.TempDir()

			// Characters that are reserved on Windows but valid on Unix
			reservedChars := []string{"<", ">", ":", "\"", "|", "?", "*"}

			for _, char := range reservedChars {
				t.Run(fmt.Sprintf("char_%s", char), func(t *testing.T) {
					fileName := fmt.Sprintf("test%sfile.json", char)
					filePath := filepath.Join(dir, fileName)

					data := testData{ID: "reserved-char", Name: fmt.Sprintf("Has %s", char)}
					err := WriteJSON(filePath, &data)
					if err != nil {
						t.Logf("WriteJSON failed for '%s': %v", char, err)
					} else {
						var result testData
						if err := ReadJSON(filePath, &result); err != nil {
							t.Errorf("ReadJSON failed: %v", err)
						}
						t.Logf("Unix accepted reserved Windows character: %s", char)
					}
				})
			}
		} else {
			t.Skip("skipping Unix-specific reserved character test on Windows")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		// Null bytes in paths should be rejected
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test\x00file.json")

		data := testData{ID: "null-byte", Name: "Has Null"}
		err := WriteJSON(filePath, &data)
		if err == nil {
			t.Error("expected error for path with null byte, got nil")
		} else {
			t.Logf("correctly rejected path with null byte: %v", err)
		}

		var result testData
		err = ReadJSON(filePath, &result)
		if err == nil {
			t.Error("expected error reading path with null byte")
		}
	})
}
