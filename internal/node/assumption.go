// Package node provides proof node types for the AF framework.
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
// Returns an error if random ID generation fails.
func NewAssumption(statement string) (*Assumption, error) {
	return NewAssumptionWithJustification(statement, "")
}

// NewAssumptionWithJustification creates a new Assumption with the
// given statement and justification.
// Returns an error if random ID generation fails.
func NewAssumptionWithJustification(statement, justification string) (*Assumption, error) {
	// Compute content hash from statement
	sum := sha256.Sum256([]byte(statement))
	contentHash := hex.EncodeToString(sum[:])

	// Generate unique ID using random bytes.
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("generating assumption ID: %w", err)
	}
	id := hex.EncodeToString(randomBytes)

	return &Assumption{
		ID:            id,
		Statement:     statement,
		ContentHash:   contentHash,
		Created:       types.Now(),
		Justification: justification,
	}, nil
}

// Validate checks if the Assumption is valid.
// Returns an error if the statement is empty or whitespace-only.
func (a *Assumption) Validate() error {
	if strings.TrimSpace(a.Statement) == "" {
		return errors.New("assumption statement cannot be empty")
	}
	return nil
}
