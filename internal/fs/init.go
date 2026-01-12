// Package fs provides filesystem operations for the AF proof framework.
package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// InitProofDir initializes a proof directory structure at the given path.
// It creates the following directory structure:
//   - proof/           (root)
//   - proof/ledger/    (for event files)
//   - proof/nodes/     (for node JSON files)
//   - proof/defs/      (for definitions)
//   - proof/assumptions/ (for assumptions)
//   - proof/externals/ (for external references)
//   - proof/lemmas/    (for extracted lemmas)
//   - proof/locks/     (for node lock files)
//   - proof/meta.json  (configuration file)
//
// The function is idempotent: calling it multiple times on the same path
// will not cause errors or data loss. Existing files are preserved.
//
// Returns an error if the path is invalid or if directory creation fails
// due to permission issues or other filesystem errors.
func InitProofDir(path string) error {
	// Validate path
	if path == "" {
		return errors.New("path cannot be empty")
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("path cannot be whitespace-only")
	}
	if strings.ContainsRune(path, '\x00') {
		return errors.New("path cannot contain null byte")
	}

	// Create the root proof directory
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}

	// Create all subdirectories
	subdirs := []string{"ledger", "nodes", "defs", "assumptions", "externals", "lemmas", "locks"}
	for _, sub := range subdirs {
		subPath := filepath.Join(path, sub)
		if err := os.MkdirAll(subPath, 0755); err != nil {
			return err
		}
	}

	// Create meta.json if it doesn't exist
	metaPath := filepath.Join(path, "meta.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		content := []byte(`{"version": "1.0"}`)
		if err := os.WriteFile(metaPath, content, 0644); err != nil {
			return err
		}
	}

	return nil
}
