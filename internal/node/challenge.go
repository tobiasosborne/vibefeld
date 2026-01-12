package node

import (
	"errors"
	"strings"

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
// Returns an error if validation fails:
// - ID must not be empty or whitespace-only
// - Reason must not be empty or whitespace-only
// - Target must be a valid ChallengeTarget enum value
func NewChallenge(id string, targetID types.NodeID, target schema.ChallengeTarget, reason string) (*Challenge, error) {
	// Validate ID is not empty or whitespace-only
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("challenge ID must not be empty")
	}

	// Validate reason is not empty or whitespace-only
	if strings.TrimSpace(reason) == "" {
		return nil, errors.New("challenge reason must not be empty")
	}

	// Validate target is a valid enum value
	if _, exists := schema.GetChallengeTargetInfo(target); !exists {
		return nil, errors.New("invalid challenge target")
	}

	return &Challenge{
		ID:       id,
		TargetID: targetID,
		Target:   target,
		Reason:   reason,
		Raised:   types.Now(),
		Status:   ChallengeStatusOpen,
	}, nil
}

// Resolve marks the challenge as resolved with the given resolution.
// Returns an error if the challenge is not open or if resolution is empty.
func (c *Challenge) Resolve(resolution string) error {
	// Validate resolution is not empty or whitespace-only
	if strings.TrimSpace(resolution) == "" {
		return errors.New("resolution must not be empty")
	}

	// Check that challenge is still open
	if !c.IsOpen() {
		return errors.New("challenge is not open")
	}

	c.Status = ChallengeStatusResolved
	c.Resolution = resolution
	c.ResolvedAt = types.Now()
	return nil
}

// Withdraw marks the challenge as withdrawn.
// Returns an error if the challenge is not open.
func (c *Challenge) Withdraw() error {
	// Check that challenge is still open
	if !c.IsOpen() {
		return errors.New("challenge is not open")
	}

	c.Status = ChallengeStatusWithdrawn
	c.ResolvedAt = types.Now()
	return nil
}

// IsOpen returns true if the challenge is still open (not resolved or withdrawn).
func (c *Challenge) IsOpen() bool {
	return c.Status == ChallengeStatusOpen
}
