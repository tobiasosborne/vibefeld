// Package fs provides filesystem operations for the AF proof framework.
package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
)

const defsDirName = "defs"

// WriteDefinition writes a definition to the defs/ subdirectory as a JSON file.
// It creates the defs/ directory if it doesn't exist.
// Uses atomic write (write to temp file, then rename) to ensure integrity.
// Overwrites existing definition file if it exists.
func WriteDefinition(basePath string, def *node.Definition) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate definition
	if def == nil {
		return errors.New("definition cannot be nil")
	}
	if strings.TrimSpace(def.ID) == "" {
		return errors.New("definition ID cannot be empty")
	}

	// Compute final path
	defPath := filepath.Join(basePath, defsDirName, def.ID+".json")

	return WriteJSON(defPath, def)
}

// ReadDefinition reads a definition from the defs/ subdirectory.
// Returns os.ErrNotExist if the definition doesn't exist.
func ReadDefinition(basePath string, id string) (*node.Definition, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	// Validate id
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("definition ID cannot be empty")
	}

	// Sanitize id to prevent path traversal
	if containsPathTraversal(id) {
		return nil, os.ErrNotExist
	}

	// Build path
	defsDir := filepath.Join(basePath, defsDirName)
	defPath := filepath.Join(defsDir, id+".json")

	// Verify the path is within defs directory (belt and suspenders)
	cleanPath := filepath.Clean(defPath)
	cleanDefsDir := filepath.Clean(defsDir)
	if !strings.HasPrefix(cleanPath, cleanDefsDir+string(filepath.Separator)) {
		return nil, os.ErrNotExist
	}

	var def node.Definition
	if err := ReadJSON(defPath, &def); err != nil {
		return nil, err
	}

	return &def, nil
}

// ListDefinitions returns all definition IDs in the defs/ directory.
// Returns only IDs from .json files (not hidden files or other extensions).
// Returns an error if the defs/ directory doesn't exist.
func ListDefinitions(basePath string) ([]string, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	defsDir := filepath.Join(basePath, defsDirName)

	entries, err := os.ReadDir(defsDir)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(entries))
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

// DeleteDefinition removes a definition file from the defs/ directory.
// Returns os.ErrNotExist if the definition doesn't exist.
func DeleteDefinition(basePath string, id string) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate id
	if strings.TrimSpace(id) == "" {
		return errors.New("definition ID cannot be empty")
	}

	// Sanitize id to prevent path traversal
	if containsPathTraversal(id) {
		return os.ErrNotExist
	}

	// Build path
	defsDir := filepath.Join(basePath, defsDirName)
	defPath := filepath.Join(defsDir, id+".json")

	// Verify the path is within defs directory (belt and suspenders)
	cleanPath := filepath.Clean(defPath)
	cleanDefsDir := filepath.Clean(defsDir)
	if !strings.HasPrefix(cleanPath, cleanDefsDir+string(filepath.Separator)) {
		return os.ErrNotExist
	}

	// Remove file
	return os.Remove(defPath)
}

// validatePath checks if a path is valid for use as a base path.
func validatePath(path string) error {
	if path == "" {
		return errors.New("path cannot be empty")
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("path cannot be whitespace-only")
	}
	if strings.ContainsRune(path, '\x00') {
		return errors.New("path cannot contain null byte")
	}
	return nil
}

// containsPathTraversal checks if an ID contains path traversal attempts.
func containsPathTraversal(id string) bool {
	// Check for common path traversal patterns
	if strings.Contains(id, "..") {
		return true
	}
	if strings.Contains(id, "/") {
		return true
	}
	if strings.Contains(id, "\\") {
		return true
	}
	if strings.Contains(id, "%2f") || strings.Contains(id, "%2F") {
		return true
	}
	return false
}
