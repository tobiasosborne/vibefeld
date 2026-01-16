// Package fs provides filesystem operations for the AF proof framework.
package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WriteJSON writes a value to a JSON file using atomic write semantics.
// It ensures the parent directory exists, marshals the value with indentation,
// writes to a temp file, and renames atomically (POSIX atomic on same filesystem).
//
// Parameters:
//   - filePath: the full path to the target JSON file
//   - v: the value to marshal (must not be nil for struct pointers)
//
// The function creates all parent directories as needed.
// Overwrites any existing file at the target path.
func WriteJSON(filePath string, v any) error {
	// Ensure parent directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file first for atomic operation
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// Rename temp to final (atomic on POSIX)
	if err := os.Rename(tempPath, filePath); err != nil {
		// Clean up temp file on failure. Ignore error from Remove since:
		// 1. The primary error (rename failure) is more important to return
		// 2. The temp file may have already been cleaned up by another process
		// 3. Leftover .tmp files are harmless and will be overwritten on next write
		_ = os.Remove(tempPath)
		return err
	}

	return nil
}

// ReadJSON reads a JSON file and unmarshals it into the provided value.
//
// Parameters:
//   - filePath: the full path to the JSON file
//   - v: a pointer to the value to unmarshal into
//
// Returns os.ErrNotExist if the file doesn't exist.
// Returns an error if the file contains invalid JSON.
func ReadJSON(filePath string, v any) error {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Unmarshal JSON
	return json.Unmarshal(data, v)
}
