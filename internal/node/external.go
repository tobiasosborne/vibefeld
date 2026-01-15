// Package node provides core data structures for proof nodes.
package node

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/tobias/vibefeld/internal/types"
)

// External represents a reference to an external source (theorem, paper, etc.).
type External struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Source      string          `json:"source"`
	ContentHash string          `json:"content_hash"`
	Created     types.Timestamp `json:"created"`
	Notes       string          `json:"notes"`
}

// NewExternal creates a new External reference with the given name and source.
// The ContentHash is computed from the source, and Created is set to the current time.
// Returns an error if random ID generation fails.
func NewExternal(name, source string) (External, error) {
	id, err := generateID()
	if err != nil {
		return External{}, err
	}
	return External{
		ID:          id,
		Name:        name,
		Source:      source,
		ContentHash: computeSourceHash(source),
		Created:     types.Now(),
		Notes:       "",
	}, nil
}

// NewExternalWithNotes creates a new External reference with the given name, source, and notes.
// The ContentHash is computed from the source, and Created is set to the current time.
// Returns an error if random ID generation fails.
func NewExternalWithNotes(name, source, notes string) (External, error) {
	id, err := generateID()
	if err != nil {
		return External{}, err
	}
	return External{
		ID:          id,
		Name:        name,
		Source:      source,
		ContentHash: computeSourceHash(source),
		Created:     types.Now(),
		Notes:       notes,
	}, nil
}

// Validate checks that the External has valid required fields.
// Returns an error if name or source is empty or contains only whitespace.
func (e External) Validate() error {
	if strings.TrimSpace(e.Name) == "" {
		return errors.New("external reference name cannot be empty")
	}
	if strings.TrimSpace(e.Source) == "" {
		return errors.New("external reference source cannot be empty")
	}
	return nil
}

// computeSourceHash computes a SHA256 hash of the source string.
func computeSourceHash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

// generateID generates a unique identifier for an External reference.
// Uses random bytes for uniqueness.
// Returns an error if crypto/rand fails.
func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating external ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}
