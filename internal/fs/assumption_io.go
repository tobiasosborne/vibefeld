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

const assumptionsDir = "assumptions"

// WriteAssumption writes an assumption to the assumptions/ subdirectory as JSON.
// It creates the assumptions/ directory if it doesn't exist.
// The file is named {assumption.ID}.json.
// Returns an error if:
//   - basePath is empty, whitespace-only, or contains null bytes
//   - assumption is nil
//   - basePath is a file instead of a directory
//   - there are permission issues or other filesystem errors
func WriteAssumption(basePath string, a *node.Assumption) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate assumption
	if a == nil {
		return errors.New("assumption cannot be nil")
	}

	// Validate assumption ID
	if strings.TrimSpace(a.ID) == "" {
		return errors.New("assumption ID cannot be empty")
	}

	// Check if basePath is a file
	info, err := os.Stat(basePath)
	if err == nil && !info.IsDir() {
		return errors.New("basePath is a file, not a directory")
	}

	// Create assumptions directory if it doesn't exist
	assumpDir := filepath.Join(basePath, assumptionsDir)
	if err := os.MkdirAll(assumpDir, 0755); err != nil {
		return err
	}

	// Marshal assumption to JSON with indentation for readability
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}

	// Write to file atomically (write to temp then rename)
	filePath := filepath.Join(assumpDir, a.ID+".json")
	tempPath := filePath + ".tmp"

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		// Clean up temp file on failure
		os.Remove(tempPath)
		return err
	}

	return nil
}

// ReadAssumption reads an assumption from the assumptions/ subdirectory.
// The id parameter should be the assumption ID (without .json extension).
// Returns an error if:
//   - basePath is empty or whitespace-only
//   - id is empty or whitespace-only
//   - the assumption file doesn't exist
//   - the file contains invalid JSON
func ReadAssumption(basePath string, id string) (*node.Assumption, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	// Validate id
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("assumption ID cannot be empty")
	}

	// Build file path - use only the base name of the ID to prevent traversal
	cleanID := filepath.Base(id)
	filePath := filepath.Join(basePath, assumptionsDir, cleanID+".json")

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Handle empty file
	if len(data) == 0 {
		return nil, errors.New("assumption file is empty")
	}

	// Unmarshal JSON
	var a node.Assumption
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}

	// Verify it's an object (not array or primitive)
	// json.Unmarshal into struct will succeed for arrays/primitives with zero values
	// Check that we got a valid ID back
	if a.ID == "" {
		return nil, errors.New("invalid assumption file: missing ID field")
	}

	return &a, nil
}

// ListAssumptions returns a list of all assumption IDs in the assumptions/ directory.
// Only .json files are considered; non-JSON files and subdirectories are ignored.
// Returns an error if:
//   - basePath is empty
//   - the assumptions directory doesn't exist
func ListAssumptions(basePath string) ([]string, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	assumpDir := filepath.Join(basePath, assumptionsDir)

	// Read directory
	entries, err := os.ReadDir(assumpDir)
	if err != nil {
		return nil, err
	}

	var ids []string
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

		// Only include .json files
		if filepath.Ext(name) != ".json" {
			continue
		}

		// Extract ID (remove .json extension)
		id := strings.TrimSuffix(name, ".json")
		ids = append(ids, id)
	}

	// Return empty slice instead of nil if no assumptions found
	if ids == nil {
		ids = []string{}
	}

	return ids, nil
}

// DeleteAssumption removes an assumption file from the assumptions/ directory.
// Returns an error if:
//   - basePath is empty
//   - id is empty or whitespace-only
//   - the assumption file doesn't exist
//   - there are permission issues
func DeleteAssumption(basePath string, id string) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate id
	if strings.TrimSpace(id) == "" {
		return errors.New("assumption ID cannot be empty")
	}

	// Build file path - use only the base name of the ID to prevent traversal
	cleanID := filepath.Base(id)
	filePath := filepath.Join(basePath, assumptionsDir, cleanID+".json")

	// Check if file exists first
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return err
	}

	// Remove file
	return os.Remove(filePath)
}

// Note: validatePath and containsPathTraversal are defined in def_io.go
