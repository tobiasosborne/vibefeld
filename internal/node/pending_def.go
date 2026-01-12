package node

import (
	"errors"
	"strings"

	"github.com/tobias/vibefeld/internal/types"
)

// PendingDefStatus represents the status of a pending definition request.
type PendingDefStatus string

const (
	// PendingDefStatusPending indicates the definition request is still pending.
	PendingDefStatusPending PendingDefStatus = "pending"
	// PendingDefStatusResolved indicates the definition request has been resolved.
	PendingDefStatusResolved PendingDefStatus = "resolved"
	// PendingDefStatusCancelled indicates the definition request was cancelled.
	PendingDefStatusCancelled PendingDefStatus = "cancelled"
)

// PendingDef represents a request for a definition that doesn't exist yet.
// When a prover needs a term defined, they create a PendingDef.
type PendingDef struct {
	ID          string           `json:"id"`
	Term        string           `json:"term"`
	RequestedBy types.NodeID     `json:"requested_by"`
	Created     types.Timestamp  `json:"created"`
	ResolvedBy  string           `json:"resolved_by"`
	Status      PendingDefStatus `json:"status"`
}

// NewPendingDef creates a new pending definition request.
// The ID is automatically generated and the status is set to pending.
func NewPendingDef(term string, requestedBy types.NodeID) *PendingDef {
	// TODO: implement proper ID generation
	return &PendingDef{
		ID:          "", // TODO: generate unique ID
		Term:        term,
		RequestedBy: requestedBy,
		Created:     types.Now(),
		ResolvedBy:  "",
		Status:      PendingDefStatusPending,
	}
}

// NewPendingDefWithValidation creates a new pending definition request with validation.
// Returns an error if the term is empty/whitespace or the requestedBy NodeID is zero.
func NewPendingDefWithValidation(term string, requestedBy types.NodeID) (*PendingDef, error) {
	// TODO: implement validation
	_ = term
	_ = requestedBy
	return nil, errors.New("not implemented")
}

// Resolve marks the pending definition as resolved by the given definition ID.
// Returns an error if the pending definition is not in pending status or if definitionID is empty.
func (pd *PendingDef) Resolve(definitionID string) error {
	if definitionID == "" {
		return errors.New("definition ID cannot be empty")
	}
	if pd.Status != PendingDefStatusPending {
		return errors.New("cannot resolve: not in pending status")
	}
	// TODO: implement
	return errors.New("not implemented")
}

// Cancel marks the pending definition as cancelled.
// Returns an error if the pending definition is not in pending status.
func (pd *PendingDef) Cancel() error {
	if pd.Status != PendingDefStatusPending {
		return errors.New("cannot cancel: not in pending status")
	}
	// TODO: implement
	return errors.New("not implemented")
}

// IsPending returns true if the pending definition is still pending.
func (pd *PendingDef) IsPending() bool {
	// TODO: implement
	return false
}

// Helper to check if a string is empty or whitespace only
func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}
