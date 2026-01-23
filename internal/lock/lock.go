// Package lock provides node locking for exclusive agent access.
package lock

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/tobias/vibefeld/internal/types"
)

// ClockSkewTolerance is the grace period added to lock expiration checks to handle
// clock drift between processes. When checking if a lock is expired, we consider
// the lock valid for an additional tolerance period beyond its nominal expiration.
//
// This prevents premature lock expiration when:
// - The reading process's clock is slightly ahead of the writing process's clock
// - NTP adjustments cause small clock jumps
// - Multiple processes on the same machine have minor clock differences
//
// A tolerance of 5 seconds handles typical NTP drift scenarios while keeping
// the window small enough that legitimate lock expiration isn't delayed significantly.
const ClockSkewTolerance = 5 * time.Second

// ClaimLock represents an agent's exclusive access to a proof node.
// ClaimLocks are time-limited and must be refreshed to maintain ownership.
// Note: Previously named "Lock", renamed to ClaimLock for clarity to distinguish
// from ledger.WriteLock which handles write serialization.
type ClaimLock struct {
	nodeID     types.NodeID
	owner      string
	acquiredAt time.Time
	expiresAt  time.Time
	released   bool
	mu         sync.Mutex
}

// NewClaimLock creates a new ClaimLock for the given node.
// Returns an error if owner is empty, timeout is zero/negative, or nodeID is invalid.
func NewClaimLock(nodeID types.NodeID, owner string, timeout time.Duration) (*ClaimLock, error) {
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
	return &ClaimLock{
		nodeID:     nodeID,
		owner:      owner,
		acquiredAt: now,
		expiresAt:  now.Add(timeout),
	}, nil
}

// NodeID returns the ID of the locked node.
func (l *ClaimLock) NodeID() types.NodeID {
	return l.nodeID
}

// Owner returns the identifier of the agent holding the lock.
func (l *ClaimLock) Owner() string {
	return l.owner
}

// AcquiredAt returns the timestamp when the lock was acquired.
func (l *ClaimLock) AcquiredAt() types.Timestamp {
	return types.FromTime(l.acquiredAt)
}

// ExpiresAt returns the timestamp when the lock expires.
func (l *ClaimLock) ExpiresAt() types.Timestamp {
	l.mu.Lock()
	defer l.mu.Unlock()
	return types.FromTime(l.expiresAt)
}

// IsExpired returns true if the lock has expired.
// The check includes a tolerance for clock skew between processes.
// A lock is considered expired only if the current time is past
// expiresAt + ClockSkewTolerance, providing a grace period that
// prevents premature expiration due to clock drift.
func (l *ClaimLock) IsExpired() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return time.Now().UTC().After(l.expiresAt.Add(ClockSkewTolerance))
}

// IsOwnedBy returns true if the lock is owned by the given owner.
func (l *ClaimLock) IsOwnedBy(owner string) bool {
	return l.owner == owner
}

// Refresh extends the lock's expiration by the given timeout from now.
// Returns an error if timeout is zero or negative.
func (l *ClaimLock) Refresh(timeout time.Duration) error {
	if timeout <= 0 {
		return errors.New("invalid timeout: must be positive")
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.expiresAt = time.Now().UTC().Add(timeout)
	return nil
}

// claimLockJSON is the JSON representation of a ClaimLock.
// This intermediate struct is required because:
// 1. ClaimLock has unexported fields that json.Marshal cannot access directly
// 2. ClaimLock contains a sync.Mutex which must not be serialized
// 3. JSON uses snake_case field names (node_id, acquired_at, expires_at)
// 4. Timestamps use RFC3339Nano format (not RFC3339) to preserve nanosecond
//    precision for accurate lock expiration tracking. This differs from
//    types.Timestamp which uses RFC3339 (second precision) for ledger events.
type claimLockJSON struct {
	NodeID     string `json:"node_id"`
	Owner      string `json:"owner"`
	AcquiredAt string `json:"acquired_at"`
	ExpiresAt  string `json:"expires_at"`
}

// MarshalJSON implements json.Marshaler.
func (l *ClaimLock) MarshalJSON() ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return json.Marshal(claimLockJSON{
		NodeID:     l.nodeID.String(),
		Owner:      l.owner,
		AcquiredAt: l.acquiredAt.Format(time.RFC3339Nano),
		ExpiresAt:  l.expiresAt.Format(time.RFC3339Nano),
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (l *ClaimLock) UnmarshalJSON(data []byte) error {
	var lj claimLockJSON
	if err := json.Unmarshal(data, &lj); err != nil {
		return err
	}

	// Validate required fields
	if lj.NodeID == "" {
		return errors.New("missing required field: node_id")
	}
	if lj.Owner == "" || strings.TrimSpace(lj.Owner) == "" {
		return errors.New("invalid owner: empty or whitespace")
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

	// Parse timestamps using RFC3339Nano (canonical format for locks).
	// No fallback parsing - we always write RFC3339Nano, so we always read it.
	acquiredAt, err := time.Parse(time.RFC3339Nano, lj.AcquiredAt)
	if err != nil {
		return errors.New("invalid acquired_at timestamp: expected RFC3339Nano format")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, lj.ExpiresAt)
	if err != nil {
		return errors.New("invalid expires_at timestamp: expected RFC3339Nano format")
	}

	l.nodeID = nodeID
	l.owner = lj.Owner
	l.acquiredAt = acquiredAt.UTC()
	l.expiresAt = expiresAt.UTC()

	return nil
}
