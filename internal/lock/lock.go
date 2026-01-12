// Package lock provides node locking for exclusive agent access.
package lock

import (
	"time"

	"github.com/tobias/vibefeld/internal/types"
)

// Lock represents an agent's exclusive access to a proof node.
// Locks are time-limited and must be refreshed to maintain ownership.
type Lock struct {
	// TODO: implement fields
}

// NewLock creates a new Lock for the given node.
// Returns an error if owner is empty, timeout is zero/negative, or nodeID is invalid.
func NewLock(nodeID types.NodeID, owner string, timeout time.Duration) (*Lock, error) {
	// TODO: implement
	panic("not implemented")
}

// NodeID returns the ID of the locked node.
func (l *Lock) NodeID() types.NodeID {
	// TODO: implement
	panic("not implemented")
}

// Owner returns the identifier of the agent holding the lock.
func (l *Lock) Owner() string {
	// TODO: implement
	panic("not implemented")
}

// AcquiredAt returns the timestamp when the lock was acquired.
func (l *Lock) AcquiredAt() types.Timestamp {
	// TODO: implement
	panic("not implemented")
}

// ExpiresAt returns the timestamp when the lock expires.
func (l *Lock) ExpiresAt() types.Timestamp {
	// TODO: implement
	panic("not implemented")
}

// IsExpired returns true if the lock has expired.
func (l *Lock) IsExpired() bool {
	// TODO: implement
	panic("not implemented")
}

// IsOwnedBy returns true if the lock is owned by the given owner.
func (l *Lock) IsOwnedBy(owner string) bool {
	// TODO: implement
	panic("not implemented")
}

// Refresh extends the lock's expiration by the given timeout from now.
// Returns an error if timeout is zero or negative.
func (l *Lock) Refresh(timeout time.Duration) error {
	// TODO: implement
	panic("not implemented")
}

// MarshalJSON implements json.Marshaler.
func (l *Lock) MarshalJSON() ([]byte, error) {
	// TODO: implement
	panic("not implemented")
}

// UnmarshalJSON implements json.Unmarshaler.
func (l *Lock) UnmarshalJSON(data []byte) error {
	// TODO: implement
	panic("not implemented")
}
