// Package fs provides filesystem operations for the AF proof framework.
package fs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
)

const externalsDirName = "externals"

// WriteExternal writes an external reference to the externals/ subdirectory as a JSON file.
// It creates the externals/ directory if it doesn't exist.
// Uses atomic write (write to temp file, then rename) to ensure integrity.
// Overwrites existing external file if it exists.
func WriteExternal(basePath string, ext *node.External) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate external
	if ext == nil {
		return errors.New("external cannot be nil")
	}
	if strings.TrimSpace(ext.ID) == "" {
		return errors.New("external ID cannot be empty")
	}

	// Ensure externals directory exists
	externalsDir := filepath.Join(basePath, externalsDirName)
	if err := os.MkdirAll(externalsDir, 0755); err != nil {
		return err
	}

	// Marshal external to JSON with indentation for readability
	data, err := json.MarshalIndent(ext, "", "  ")
	if err != nil {
		return err
	}

	// Compute final path
	extPath := filepath.Join(externalsDir, ext.ID+".json")

	// Write to temp file first for atomic operation
	tempPath := extPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// Rename temp to final (atomic on POSIX)
	if err := os.Rename(tempPath, extPath); err != nil {
		// Clean up temp file on error
		os.Remove(tempPath)
		return err
	}

	return nil
}

// ReadExternal reads an external reference from the externals/ subdirectory.
// Returns os.ErrNotExist if the external doesn't exist.
func ReadExternal(basePath string, id string) (*node.External, error) {
	// Validate id
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("external ID cannot be empty")
	}

	// Sanitize id to prevent path traversal
	if containsPathTraversal(id) {
		return nil, os.ErrNotExist
	}

	// Build path
	externalsDir := filepath.Join(basePath, externalsDirName)
	extPath := filepath.Join(externalsDir, id+".json")

	// Verify the path is within externals directory (belt and suspenders)
	cleanPath := filepath.Clean(extPath)
	cleanExternalsDir := filepath.Clean(externalsDir)
	if !strings.HasPrefix(cleanPath, cleanExternalsDir+string(filepath.Separator)) {
		return nil, os.ErrNotExist
	}

	// Read file
	data, err := os.ReadFile(extPath)
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON
	var ext node.External
	if err := json.Unmarshal(data, &ext); err != nil {
		return nil, err
	}

	return &ext, nil
}

// ListExternals returns all external IDs in the externals/ directory.
// Returns only IDs from .json files (not hidden files or other extensions).
// Returns an error if the externals/ directory doesn't exist.
func ListExternals(basePath string) ([]string, error) {
	// Validate basePath
	if strings.TrimSpace(basePath) == "" {
		return nil, errors.New("path cannot be empty")
	}

	externalsDir := filepath.Join(basePath, externalsDirName)

	entries, err := os.ReadDir(externalsDir)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0)
	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Only process .json files
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		// Extract ID (remove .json extension)
		id := strings.TrimSuffix(name, ".json")
		ids = append(ids, id)
	}

	return ids, nil
}

// DeleteExternal removes an external file from the externals/ directory.
// Returns os.ErrNotExist if the external doesn't exist.
func DeleteExternal(basePath string, id string) error {
	// Validate basePath
	if strings.TrimSpace(basePath) == "" {
		return errors.New("path cannot be empty")
	}

	// Validate id
	if strings.TrimSpace(id) == "" {
		return errors.New("external ID cannot be empty")
	}

	// Sanitize id to prevent path traversal
	if containsPathTraversal(id) {
		return os.ErrNotExist
	}

	// Build path
	externalsDir := filepath.Join(basePath, externalsDirName)
	extPath := filepath.Join(externalsDir, id+".json")

	// Verify the path is within externals directory (belt and suspenders)
	cleanPath := filepath.Clean(extPath)
	cleanExternalsDir := filepath.Clean(externalsDir)
	if !strings.HasPrefix(cleanPath, cleanExternalsDir+string(filepath.Separator)) {
		return os.ErrNotExist
	}

	// Remove file
	return os.Remove(extPath)
}
