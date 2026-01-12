package scope

import (
	"errors"
	"strings"

	"github.com/tobias/vibefeld/internal/types"
)

// Entry represents a scope entry for a local assumption.
// Local assumptions are introduced by `local_assume` nodes and
// discharged by `local_discharge` nodes. Scope is tied to the
// hierarchical NodeID structure.
type Entry struct {
	NodeID     types.NodeID
	Statement  string
	Introduced types.Timestamp
	Discharged *types.Timestamp // nil if still active
}

// NewEntry creates a new scope entry. Returns error if nodeID invalid or statement empty.
func NewEntry(nodeID types.NodeID, statement string) (*Entry, error) {
	// Check for invalid/zero NodeID
	if nodeID.String() == "" {
		return nil, errors.New("invalid node ID: zero value")
	}

	// Check for empty or whitespace-only statement
	if strings.TrimSpace(statement) == "" {
		return nil, errors.New("statement cannot be empty or whitespace-only")
	}

	return &Entry{
		NodeID:     nodeID,
		Statement:  statement,
		Introduced: types.Now(),
		Discharged: nil,
	}, nil
}

// Discharge marks the entry as discharged. Returns error if already discharged.
func (e *Entry) Discharge() error {
	if e.Discharged != nil {
		return errors.New("entry already discharged")
	}
	now := types.Now()
	e.Discharged = &now
	return nil
}

// IsActive returns true if the entry is not discharged.
func (e *Entry) IsActive() bool {
	return e.Discharged == nil
}
