// Package fs provides filesystem operations for the AF proof framework.
package fs

import (
	"os"
	"path/filepath"
	"strings"

	aferrors "github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/types"
)

const nodesDirName = "nodes"

// WriteNode writes a node to the nodes/ subdirectory as a JSON file.
// It creates the nodes/ directory if it doesn't exist.
// Uses atomic write (write to temp file, then rename) to ensure integrity.
// Overwrites existing node file if it exists.
// The filename is derived from the node ID with dots replaced by underscores
// (e.g., node ID "1.2.3" becomes "1_2_3.json").
func WriteNode(basePath string, n *node.Node) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate node
	if n == nil {
		return aferrors.New(aferrors.INVALID_TARGET, "node cannot be nil")
	}
	if n.ID.String() == "" {
		return aferrors.New(aferrors.INVALID_TARGET, "node ID cannot be empty")
	}

	// Compute final path (replace dots with underscores in filename)
	filename := nodeIDToFilename(n.ID)
	nodePath := filepath.Join(basePath, nodesDirName, filename)

	return WriteJSON(nodePath, n)
}

// ReadNode reads a node from the nodes/ subdirectory.
// Returns os.ErrNotExist if the node doesn't exist.
// Verifies the content hash on read and returns a CONTENT_HASH_MISMATCH error
// if the stored hash doesn't match the computed hash.
func ReadNode(basePath string, id types.NodeID) (*node.Node, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	// Validate id
	if id.String() == "" {
		return nil, aferrors.New(aferrors.INVALID_TARGET, "node ID cannot be empty")
	}

	// Build path
	filename := nodeIDToFilename(id)
	nodePath := filepath.Join(basePath, nodesDirName, filename)

	var n node.Node
	if err := ReadJSON(nodePath, &n); err != nil {
		return nil, err
	}

	// Verify content hash
	if !n.VerifyContentHash() {
		return nil, aferrors.Newf(aferrors.CONTENT_HASH_MISMATCH,
			"content hash mismatch for node %s: stored=%s, computed=%s",
			id.String(), n.ContentHash, n.ComputeContentHash())
	}

	return &n, nil
}

// ListNodes returns all node IDs in the nodes/ directory.
// Returns only IDs from .json files (not hidden files or other extensions).
// Returns an error if the nodes/ directory doesn't exist.
func ListNodes(basePath string) ([]types.NodeID, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	nodesDir := filepath.Join(basePath, nodesDirName)

	entries, err := os.ReadDir(nodesDir)
	if err != nil {
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

		// Extract ID (remove .json extension and convert underscores to dots)
		idStr := strings.TrimSuffix(name, ".json")
		idStr = strings.ReplaceAll(idStr, "_", ".")

		id, err := types.Parse(idStr)
		if err != nil {
			// Skip files with invalid node ID format
			continue
		}

		ids = append(ids, id)
	}

	return ids, nil
}

// DeleteNode removes a node file from the nodes/ directory.
// Returns os.ErrNotExist if the node doesn't exist.
func DeleteNode(basePath string, id types.NodeID) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate id
	if id.String() == "" {
		return aferrors.New(aferrors.INVALID_TARGET, "node ID cannot be empty")
	}

	// Build path
	nodesDir := filepath.Join(basePath, nodesDirName)
	filename := nodeIDToFilename(id)
	nodePath := filepath.Join(nodesDir, filename)

	// Remove file
	return os.Remove(nodePath)
}

// nodeIDToFilename converts a node ID to a safe filename.
// Replaces dots with underscores (e.g., "1.2.3" -> "1_2_3.json").
func nodeIDToFilename(id types.NodeID) string {
	return strings.ReplaceAll(id.String(), ".", "_") + ".json"
}
