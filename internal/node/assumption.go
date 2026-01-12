// Package node provides proof node types for the AF framework.
package node

import (
	"github.com/tobias/vibefeld/internal/types"
)

// Assumption represents a statement taken as given for the proof.
// Assumptions form the foundation upon which proof steps are built.
type Assumption struct {
	// ID is the unique identifier for this assumption.
	ID string `json:"id"`

	// Statement is the assumption text.
	Statement string `json:"statement"`

	// ContentHash is the SHA256 hash of the statement.
	ContentHash string `json:"content_hash"`

	// Created is the timestamp when this assumption was created.
	Created types.Timestamp `json:"created"`

	// Justification is an optional explanation for why this is assumed.
	Justification string `json:"justification,omitempty"`
}

// NewAssumption creates a new Assumption with the given statement.
// The content hash and timestamp are computed automatically.
func NewAssumption(statement string) *Assumption {
	// TODO: implement
	return nil
}

// NewAssumptionWithJustification creates a new Assumption with the
// given statement and justification.
func NewAssumptionWithJustification(statement, justification string) *Assumption {
	// TODO: implement
	return nil
}

// Validate checks if the Assumption is valid.
// Returns an error if the statement is empty or whitespace-only.
func (a *Assumption) Validate() error {
	// TODO: implement
	return nil
}
