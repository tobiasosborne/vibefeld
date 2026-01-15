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

// Definition represents a formal definition used in a proof.
type Definition struct {
	// ID is the unique identifier for this definition.
	ID string `json:"id"`

	// Name is the name of the term being defined.
	Name string `json:"name"`

	// Content is the definition text.
	Content string `json:"content"`

	// ContentHash is the SHA256 hash of the content.
	ContentHash string `json:"content_hash"`

	// Created is the timestamp when this definition was created.
	Created types.Timestamp `json:"created"`
}

// NewDefinition creates a new Definition with the given name and content.
// Returns an error if name or content is empty or whitespace-only,
// or if random ID generation fails.
func NewDefinition(name, content string) (*Definition, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("definition name cannot be empty")
	}
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("definition content cannot be empty")
	}

	id, err := generateDefinitionID()
	if err != nil {
		return nil, err
	}

	return &Definition{
		ID:          id,
		Name:        name,
		Content:     content,
		ContentHash: computeContentHash(content),
		Created:     types.Now(),
	}, nil
}

// Validate checks if the Definition is valid.
// Returns an error if name or content is empty or whitespace-only.
func (d *Definition) Validate() error {
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("definition name cannot be empty")
	}
	if strings.TrimSpace(d.Content) == "" {
		return errors.New("definition content cannot be empty")
	}
	return nil
}

// Equal returns true if two definitions have the same content hash.
func (d *Definition) Equal(other *Definition) bool {
	if other == nil {
		return false
	}
	return d.ContentHash == other.ContentHash
}

// generateDefinitionID generates a unique identifier for a Definition.
// Uses random bytes for uniqueness.
// Returns an error if crypto/rand fails.
func generateDefinitionID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating definition ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// computeContentHash computes a SHA256 hash of the content string.
func computeContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
