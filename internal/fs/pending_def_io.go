// Package fs provides filesystem operations for the AF proof framework.
package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/types"
)

const pendingDefsDirName = ".af/pending_defs"

// WritePendingDef writes a pending definition to the .af/pending_defs/ subdirectory as a JSON file.
// It creates the .af/pending_defs/ directory if it doesn't exist.
// Uses atomic write (write to temp file, then rename) to ensure integrity.
// Overwrites existing pending def file if it exists.
// The filename is derived from the node ID (e.g., node ID "1.2" becomes "1.2.json").
func WritePendingDef(basePath string, nodeID types.NodeID, pd *node.PendingDef) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate nodeID
	if nodeID.String() == "" {
		return errors.New("node ID cannot be empty")
	}

	// Validate pending def
	if pd == nil {
		return errors.New("pending def cannot be nil")
	}

	// Compute final path
	defPath := filepath.Join(basePath, pendingDefsDirName, nodeID.String()+".json")

	return WriteJSON(defPath, pd)
}

// ReadPendingDef reads a pending definition from the .af/pending_defs/ subdirectory.
// Returns an error if the pending def doesn't exist.
func ReadPendingDef(basePath string, nodeID types.NodeID) (*node.PendingDef, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	// Validate nodeID
	if nodeID.String() == "" {
		return nil, errors.New("node ID cannot be empty")
	}

	// Build path
	defPath := filepath.Join(basePath, pendingDefsDirName, nodeID.String()+".json")

	var pd node.PendingDef
	if err := ReadJSON(defPath, &pd); err != nil {
		return nil, err
	}

	return &pd, nil
}

// ListPendingDefs returns all pending definition node IDs in the .af/pending_defs/ directory.
// Returns only IDs from .json files (not hidden files or other extensions).
// Returns an empty slice (not an error) if the .af/pending_defs/ directory doesn't exist.
func ListPendingDefs(basePath string) ([]types.NodeID, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	pendingDefsDir := filepath.Join(basePath, pendingDefsDirName)

	entries, err := os.ReadDir(pendingDefsDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty slice, not an error, when directory doesn't exist
			return []types.NodeID{}, nil
		}
		return nil, err
	}

	ids := make([]types.NodeID, 0, len(entries))
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
		idStr := strings.TrimSuffix(name, ".json")

		id, err := types.Parse(idStr)
		if err != nil {
			// Skip files with invalid node ID format
			continue
		}

		ids = append(ids, id)
	}

	return ids, nil
}

// DeletePendingDef removes a pending definition file from the .af/pending_defs/ directory.
// This operation is idempotent: it does NOT return an error if the pending def doesn't exist.
func DeletePendingDef(basePath string, nodeID types.NodeID) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate nodeID
	if nodeID.String() == "" {
		return errors.New("node ID cannot be empty")
	}

	// Build path
	pendingDefsDir := filepath.Join(basePath, pendingDefsDirName)
	defPath := filepath.Join(pendingDefsDir, nodeID.String()+".json")

	// Remove file - treat not exist as success (idempotent)
	err := os.Remove(defPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
