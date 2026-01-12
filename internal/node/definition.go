package node

import (
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
// Returns an error if name or content is empty or whitespace-only.
func NewDefinition(name, content string) (*Definition, error) {
	// TODO: implement
	return nil, nil
}

// Validate checks if the Definition is valid.
// Returns an error if name or content is empty or whitespace-only.
func (d *Definition) Validate() error {
	// TODO: implement
	return nil
}

// Equal returns true if two definitions have the same content hash.
func (d *Definition) Equal(other *Definition) bool {
	// TODO: implement
	return false
}
