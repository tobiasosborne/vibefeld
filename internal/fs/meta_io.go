// Package fs provides filesystem operations for the AF proof framework.
package fs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const metaFileName = "meta.json"

// Meta represents proof metadata stored in meta.json.
type Meta struct {
	Conjecture string    `json:"conjecture"`
	CreatedAt  time.Time `json:"created_at"`
	Version    string    `json:"version"`
}

// WriteMeta writes metadata to meta.json in the proof directory.
// Uses atomic write (write to temp file, then rename) to ensure integrity.
// Overwrites existing meta.json if it exists.
func WriteMeta(basePath string, meta *Meta) error {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return err
	}

	// Validate meta
	if meta == nil {
		return errors.New("meta cannot be nil")
	}
	if strings.TrimSpace(meta.Conjecture) == "" {
		return errors.New("conjecture cannot be empty")
	}
	if strings.TrimSpace(meta.Version) == "" {
		return errors.New("version cannot be empty")
	}
	if meta.CreatedAt.IsZero() {
		return errors.New("created_at cannot be zero")
	}

	// Marshal meta to JSON with indentation for readability
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	// Compute final path
	metaPath := filepath.Join(basePath, metaFileName)

	// Write to temp file first for atomic operation
	tempPath := metaPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// Rename temp to final (atomic on POSIX)
	if err := os.Rename(tempPath, metaPath); err != nil {
		// Clean up temp file on failure. Ignore error from Remove since:
		// 1. The primary error (rename failure) is more important to return
		// 2. The temp file may have already been cleaned up by another process
		// 3. Leftover .tmp files are harmless and will be overwritten on next write
		_ = os.Remove(tempPath)
		return err
	}

	return nil
}

// ReadMeta reads metadata from meta.json in the proof directory.
// Returns os.ErrNotExist if meta.json doesn't exist.
func ReadMeta(basePath string) (*Meta, error) {
	// Validate basePath
	if err := validatePath(basePath); err != nil {
		return nil, err
	}

	// Build path
	metaPath := filepath.Join(basePath, metaFileName)

	// Read file
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}
