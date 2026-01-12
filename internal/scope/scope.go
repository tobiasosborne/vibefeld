package scope

import "github.com/tobias/vibefeld/internal/types"

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
	return nil, nil // stub
}

// Discharge marks the entry as discharged. Returns error if already discharged.
func (e *Entry) Discharge() error {
	return nil // stub
}

// IsActive returns true if the entry is not discharged.
func (e *Entry) IsActive() bool {
	return false // stub
}
