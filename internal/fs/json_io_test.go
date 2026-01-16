package fs

import (
	"os"
	"path/filepath"
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
