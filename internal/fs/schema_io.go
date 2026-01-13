// Package fs provides filesystem operations for the AF proof framework.
package fs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/tobias/vibefeld/internal/schema"
)

const schemaFileName = "schema.json"

// WriteSchema writes a schema to basePath/schema.json.
// Uses atomic write (write to temp file, then rename) to ensure integrity.
// Overwrites existing schema.json if it exists.
func WriteSchema(basePath string, s *schema.Schema) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate schema
	if s == nil {
		return errors.New("schema cannot be nil")
	}

	// Marshal schema to JSON with indentation for readability
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Compute final path
	schemaPath := filepath.Join(basePath, schemaFileName)

	// Write to temp file first for atomic operation
	tempPath := schemaPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// Rename temp to final (atomic on POSIX)
	if err := os.Rename(tempPath, schemaPath); err != nil {
		// Clean up temp file on failure. Ignore error from Remove since:
		// 1. The primary error (rename failure) is more important to return
		// 2. The temp file may have already been cleaned up by another process
		// 3. Leftover .tmp files are harmless and will be overwritten on next write
		_ = os.Remove(tempPath)
		return err
	}

	return nil
}

// ReadSchema reads a schema from basePath/schema.json.
// Returns an error if the file doesn't exist, contains invalid JSON,
// or the schema fails validation.
// The returned schema has its internal caches rebuilt for O(1) lookups.
func ReadSchema(basePath string) (*schema.Schema, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	// Build path
	schemaPath := filepath.Join(basePath, schemaFileName)

	// Check if path is a directory
	info, err := os.Stat(schemaPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("schema.json is a directory, not a file")
	}

	// Read file
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, err
	}

	// Use LoadSchema which unmarshals, validates, and builds caches
	return schema.LoadSchema(data)
}
