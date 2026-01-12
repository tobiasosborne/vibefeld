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

const lemmasDirName = "lemmas"

// WriteLemma writes a lemma to the lemmas/ subdirectory as a JSON file.
// It creates the lemmas/ directory if it doesn't exist.
// Uses atomic write (write to temp file, then rename) to ensure integrity.
// Overwrites existing lemma file if it exists.
func WriteLemma(basePath string, lemma *node.Lemma) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate lemma
	if lemma == nil {
		return errors.New("lemma cannot be nil")
	}
	if strings.TrimSpace(lemma.ID) == "" {
		return errors.New("lemma ID cannot be empty")
	}

	// Ensure lemmas directory exists
	lemmasDir := filepath.Join(basePath, lemmasDirName)
	if err := os.MkdirAll(lemmasDir, 0755); err != nil {
		return err
	}

	// Marshal lemma to JSON with indentation for readability
	data, err := json.MarshalIndent(lemma, "", "  ")
	if err != nil {
		return err
	}

	// Compute final path
	lemmaPath := filepath.Join(lemmasDir, lemma.ID+".json")

	// Write to temp file first for atomic operation
	tempPath := lemmaPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// Rename temp to final (atomic on POSIX)
	if err := os.Rename(tempPath, lemmaPath); err != nil {
		// Clean up temp file on failure. Ignore error from Remove since:
		// 1. The primary error (rename failure) is more important to return
		// 2. The temp file may have already been cleaned up by another process
		// 3. Leftover .tmp files are harmless and will be overwritten on next write
		_ = os.Remove(tempPath)
		return err
	}

	return nil
}

// ReadLemma reads a lemma from the lemmas/ subdirectory.
// Returns os.ErrNotExist if the lemma doesn't exist.
func ReadLemma(basePath string, id string) (*node.Lemma, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	// Validate id
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("lemma ID cannot be empty")
	}

	// Sanitize id to prevent path traversal
	if containsPathTraversal(id) {
		return nil, os.ErrNotExist
	}

	// Build path
	lemmasDir := filepath.Join(basePath, lemmasDirName)
	lemmaPath := filepath.Join(lemmasDir, id+".json")

	// Verify the path is within lemmas directory (belt and suspenders)
	cleanPath := filepath.Clean(lemmaPath)
	cleanLemmasDir := filepath.Clean(lemmasDir)
	if !strings.HasPrefix(cleanPath, cleanLemmasDir+string(filepath.Separator)) {
		return nil, os.ErrNotExist
	}

	// Read file
	data, err := os.ReadFile(lemmaPath)
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON
	var lemma node.Lemma
	if err := json.Unmarshal(data, &lemma); err != nil {
		return nil, err
	}

	return &lemma, nil
}

// ListLemmas returns all lemma IDs in the lemmas/ directory.
// Returns only IDs from .json files (not hidden files or other extensions).
// Returns an error if the lemmas/ directory doesn't exist.
func ListLemmas(basePath string) ([]string, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	lemmasDir := filepath.Join(basePath, lemmasDirName)

	entries, err := os.ReadDir(lemmasDir)
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

// DeleteLemma removes a lemma file from the lemmas/ directory.
// Returns os.ErrNotExist if the lemma doesn't exist.
func DeleteLemma(basePath string, id string) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate id
	if strings.TrimSpace(id) == "" {
		return errors.New("lemma ID cannot be empty")
	}

	// Sanitize id to prevent path traversal
	if containsPathTraversal(id) {
		return os.ErrNotExist
	}

	// Build path
	lemmasDir := filepath.Join(basePath, lemmasDirName)
	lemmaPath := filepath.Join(lemmasDir, id+".json")

	// Verify the path is within lemmas directory (belt and suspenders)
	cleanPath := filepath.Clean(lemmaPath)
	cleanLemmasDir := filepath.Clean(lemmasDir)
	if !strings.HasPrefix(cleanPath, cleanLemmasDir+string(filepath.Separator)) {
		return os.ErrNotExist
	}

	// Remove file
	return os.Remove(lemmaPath)
}
