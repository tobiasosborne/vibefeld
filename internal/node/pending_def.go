package node

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
// Returns an error if random ID generation fails.
func NewPendingDef(term string, requestedBy types.NodeID) (*PendingDef, error) {
	id, err := generatePendingDefID()
	if err != nil {
		return nil, err
	}
	return &PendingDef{
		ID:          id,
		Term:        term,
		RequestedBy: requestedBy,
		Created:     types.Now(),
		ResolvedBy:  "",
		Status:      PendingDefStatusPending,
	}, nil
}

// generatePendingDefID generates a unique identifier for a PendingDef.
// Uses random bytes for uniqueness.
// Returns an error if crypto/rand fails.
func generatePendingDefID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating pending definition ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// NewPendingDefWithValidation creates a new pending definition request with validation.
// Returns an error if the term is empty/whitespace, the requestedBy NodeID is zero,
// or if random ID generation fails.
func NewPendingDefWithValidation(term string, requestedBy types.NodeID) (*PendingDef, error) {
	if isBlank(term) {
		return nil, errors.New("term cannot be empty or whitespace")
	}
	// A zero NodeID has String() == ""
	if requestedBy.String() == "" {
		return nil, errors.New("requestedBy cannot be zero NodeID")
	}
	return NewPendingDef(term, requestedBy)
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
	pd.Status = PendingDefStatusResolved
	pd.ResolvedBy = definitionID
	return nil
}

// Cancel marks the pending definition as cancelled.
// Returns an error if the pending definition is not in pending status.
func (pd *PendingDef) Cancel() error {
	if pd.Status != PendingDefStatusPending {
		return errors.New("cannot cancel: not in pending status")
	}
	pd.Status = PendingDefStatusCancelled
	return nil
}

// IsPending returns true if the pending definition is still pending.
func (pd *PendingDef) IsPending() bool {
	return pd.Status == PendingDefStatusPending
}

// Helper to check if a string is empty or whitespace only
func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// pendingDefJSON is an intermediate type for JSON serialization of PendingDef.
// It handles NodeID serialization as a string.
type pendingDefJSON struct {
	ID          string           `json:"id"`
	Term        string           `json:"term"`
	RequestedBy string           `json:"requested_by"`
	Created     types.Timestamp  `json:"created"`
	ResolvedBy  string           `json:"resolved_by"`
	Status      PendingDefStatus `json:"status"`
}

// MarshalJSON implements json.Marshaler for PendingDef.
func (pd PendingDef) MarshalJSON() ([]byte, error) {
	return json.Marshal(pendingDefJSON{
		ID:          pd.ID,
		Term:        pd.Term,
		RequestedBy: pd.RequestedBy.String(),
		Created:     pd.Created,
		ResolvedBy:  pd.ResolvedBy,
		Status:      pd.Status,
	})
}

// UnmarshalJSON implements json.Unmarshaler for PendingDef.
func (pd *PendingDef) UnmarshalJSON(data []byte) error {
	var aux pendingDefJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse the RequestedBy NodeID from string
	requestedBy, err := types.Parse(aux.RequestedBy)
	if err != nil {
		return err
	}

	pd.ID = aux.ID
	pd.Term = aux.Term
	pd.RequestedBy = requestedBy
	pd.Created = aux.Created
	pd.ResolvedBy = aux.ResolvedBy
	pd.Status = aux.Status
	return nil
}
