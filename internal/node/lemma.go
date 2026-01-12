// Package node provides data structures for proof nodes in the AF system.
package node

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/tobias/vibefeld/internal/types"
)

// Lemma represents an extracted reusable proof fragment.
// A lemma is created from a proof node and can be referenced
// by other proofs.
type Lemma struct {
	// ID is the unique identifier for this lemma.
	ID string `json:"id"`

	// Statement is the lemma statement text.
	Statement string `json:"statement"`

	// SourceNodeID is the NodeID of the node this lemma was extracted from.
	SourceNodeID types.NodeID `json:"source_node_id"`

	// ContentHash is the SHA256 hash of the statement.
	ContentHash string `json:"content_hash"`

	// Created is the timestamp when the lemma was created.
	Created types.Timestamp `json:"created"`

	// Proof is the optional proof text (may be filled later).
	Proof string `json:"proof,omitempty"`
}

// NewLemma creates a new Lemma with the given statement and source node ID.
// Returns an error if the statement is empty or the source node ID is invalid.
func NewLemma(statement string, sourceNodeID types.NodeID) (*Lemma, error) {
	// Validate statement is not empty or whitespace only
	if strings.TrimSpace(statement) == "" {
		return nil, errors.New("lemma statement cannot be empty")
	}

	// Validate source node ID is not zero value
	if sourceNodeID.String() == "" {
		return nil, errors.New("source node ID cannot be empty")
	}

	// Compute content hash from statement
	sum := sha256.Sum256([]byte(statement))
	contentHash := hex.EncodeToString(sum[:])

	// Generate unique ID using random bytes
	now := types.Now()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	id := "LEM-" + hex.EncodeToString(randomBytes)

	return &Lemma{
		ID:           id,
		Statement:    statement,
		SourceNodeID: sourceNodeID,
		ContentHash:  contentHash,
		Created:      now,
		Proof:        "",
	}, nil
}

// SetProof sets the proof text for this lemma.
// An empty string clears the proof.
func (l *Lemma) SetProof(proof string) {
	l.Proof = proof
}
