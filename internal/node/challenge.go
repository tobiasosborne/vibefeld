package node

import (
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// ChallengeStatus represents the current state of a challenge.
type ChallengeStatus string

// Challenge status values.
const (
	ChallengeStatusOpen      ChallengeStatus = "open"
	ChallengeStatusResolved  ChallengeStatus = "resolved"
	ChallengeStatusWithdrawn ChallengeStatus = "withdrawn"
)

// Challenge represents a verifier's challenge against a proof node.
// Challenges are raised when a verifier identifies an issue with a node's
// statement, inference, context, dependencies, scope, or other aspect.
type Challenge struct {
	// ID is the unique identifier for this challenge.
	ID string `json:"id"`

	// TargetID is the NodeID of the node being challenged.
	TargetID types.NodeID `json:"target_id"`

	// Target identifies what aspect of the node is being challenged.
	Target schema.ChallengeTarget `json:"target"`

	// Reason explains why the challenge was raised.
	Reason string `json:"reason"`

	// Raised is the timestamp when the challenge was created.
	Raised types.Timestamp `json:"raised"`

	// ResolvedAt is the timestamp when the challenge was resolved or withdrawn.
	// Zero value if the challenge is still open.
	ResolvedAt types.Timestamp `json:"resolved_at,omitempty"`

	// Resolution is the explanation of how the challenge was addressed.
	// Empty if the challenge is still open or was withdrawn.
	Resolution string `json:"resolution,omitempty"`

	// Status is the current state of the challenge.
	Status ChallengeStatus `json:"status"`
}

// NewChallenge creates a new Challenge with the given parameters.
// Returns an error if validation fails.
// TODO: Implement validation and initialization
func NewChallenge(id string, targetID types.NodeID, target schema.ChallengeTarget, reason string) (*Challenge, error) {
	panic("not implemented")
}

// Resolve marks the challenge as resolved with the given resolution.
// Returns an error if the challenge is not open or if resolution is empty.
// TODO: Implement state transition and validation
func (c *Challenge) Resolve(resolution string) error {
	panic("not implemented")
}

// Withdraw marks the challenge as withdrawn.
// Returns an error if the challenge is not open.
// TODO: Implement state transition
func (c *Challenge) Withdraw() error {
	panic("not implemented")
}

// IsOpen returns true if the challenge is still open (not resolved or withdrawn).
// TODO: Implement status check
func (c *Challenge) IsOpen() bool {
	panic("not implemented")
}
