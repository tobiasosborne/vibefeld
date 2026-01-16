// Package fs provides filesystem operations for the AF proof framework.
package fs

import (
	"errors"
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

	// Compute final path
	metaPath := filepath.Join(basePath, metaFileName)

	return WriteJSON(metaPath, meta)
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

	var meta Meta
	if err := ReadJSON(metaPath, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}
