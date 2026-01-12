// Package lock provides node locking for exclusive agent access.
package lock

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/tobias/vibefeld/internal/types"
)

// Lock represents an agent's exclusive access to a proof node.
// Locks are time-limited and must be refreshed to maintain ownership.
type Lock struct {
	nodeID     types.NodeID
	owner      string
	acquiredAt time.Time
	expiresAt  time.Time
}

// NewLock creates a new Lock for the given node.
// Returns an error if owner is empty, timeout is zero/negative, or nodeID is invalid.
func NewLock(nodeID types.NodeID, owner string, timeout time.Duration) (*Lock, error) {
	// Validate nodeID (check if it's the zero value)
	if nodeID.String() == "" {
		return nil, errors.New("invalid node ID: empty")
	}

	// Validate owner
	if owner == "" || strings.TrimSpace(owner) == "" {
		return nil, errors.New("invalid owner: empty or whitespace")
	}

	// Validate timeout
	if timeout <= 0 {
		return nil, errors.New("invalid timeout: must be positive")
	}

	now := time.Now().UTC()
	return &Lock{
		nodeID:     nodeID,
		owner:      owner,
		acquiredAt: now,
		expiresAt:  now.Add(timeout),
	}, nil
}

// NodeID returns the ID of the locked node.
func (l *Lock) NodeID() types.NodeID {
	return l.nodeID
}

// Owner returns the identifier of the agent holding the lock.
func (l *Lock) Owner() string {
	return l.owner
}

// AcquiredAt returns the timestamp when the lock was acquired.
func (l *Lock) AcquiredAt() types.Timestamp {
	ts, _ := types.ParseTimestamp(l.acquiredAt.Format(time.RFC3339Nano))
	return ts
}

// ExpiresAt returns the timestamp when the lock expires.
func (l *Lock) ExpiresAt() types.Timestamp {
	ts, _ := types.ParseTimestamp(l.expiresAt.Format(time.RFC3339Nano))
	return ts
}

// IsExpired returns true if the lock has expired.
func (l *Lock) IsExpired() bool {
	return time.Now().UTC().After(l.expiresAt)
}

// IsOwnedBy returns true if the lock is owned by the given owner.
func (l *Lock) IsOwnedBy(owner string) bool {
	return l.owner == owner
}

// Refresh extends the lock's expiration by the given timeout from now.
// Returns an error if timeout is zero or negative.
func (l *Lock) Refresh(timeout time.Duration) error {
	if timeout <= 0 {
		return errors.New("invalid timeout: must be positive")
	}

	l.expiresAt = time.Now().UTC().Add(timeout)
	return nil
}

// lockJSON is the JSON representation of a Lock.
type lockJSON struct {
	NodeID     string `json:"node_id"`
	Owner      string `json:"owner"`
	AcquiredAt string `json:"acquired_at"`
	ExpiresAt  string `json:"expires_at"`
}

// MarshalJSON implements json.Marshaler.
func (l *Lock) MarshalJSON() ([]byte, error) {
	return json.Marshal(lockJSON{
		NodeID:     l.nodeID.String(),
		Owner:      l.owner,
		AcquiredAt: l.acquiredAt.Format(time.RFC3339Nano),
		ExpiresAt:  l.expiresAt.Format(time.RFC3339Nano),
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (l *Lock) UnmarshalJSON(data []byte) error {
	var lj lockJSON
	if err := json.Unmarshal(data, &lj); err != nil {
		return err
	}

	// Validate required fields
	if lj.NodeID == "" {
		return errors.New("missing required field: node_id")
	}
	if lj.Owner == "" {
		return errors.New("missing required field: owner")
	}
	if lj.AcquiredAt == "" {
		return errors.New("missing required field: acquired_at")
	}
	if lj.ExpiresAt == "" {
		return errors.New("missing required field: expires_at")
	}

	// Parse node ID
	nodeID, err := types.Parse(lj.NodeID)
	if err != nil {
		return err
	}

	// Parse timestamps (try RFC3339Nano first, then RFC3339)
	acquiredAt, err := time.Parse(time.RFC3339Nano, lj.AcquiredAt)
	if err != nil {
		acquiredAt, err = time.Parse(time.RFC3339, lj.AcquiredAt)
		if err != nil {
			return errors.New("invalid acquired_at timestamp")
		}
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, lj.ExpiresAt)
	if err != nil {
		expiresAt, err = time.Parse(time.RFC3339, lj.ExpiresAt)
		if err != nil {
			return errors.New("invalid expires_at timestamp")
		}
	}

	l.nodeID = nodeID
	l.owner = lj.Owner
	l.acquiredAt = acquiredAt.UTC()
	l.expiresAt = expiresAt.UTC()

	return nil
}
