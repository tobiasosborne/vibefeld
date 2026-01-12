package lock

import (
	"errors"
	"fmt"
	"time"
)

// LockInfo contains metadata about a lock for display and serialization.
type LockInfo struct {
	NodeID    string        `json:"node_id"`
	Owner     string        `json:"owner"`
	Acquired  time.Time     `json:"acquired"`
	Expires   time.Time     `json:"expires"`
	Remaining time.Duration `json:"remaining"`
	IsExpired bool          `json:"is_expired"`
}

// GetLockInfo retrieves lock information from a Lock.
// Returns an error if the lock is nil.
func GetLockInfo(lk *Lock) (*LockInfo, error) {
	if lk == nil {
		return nil, errors.New("lock is nil")
	}

	now := time.Now().UTC()
	remaining := lk.expiresAt.Sub(now)
	isExpired := lk.IsExpired()

	return &LockInfo{
		NodeID:    lk.nodeID.String(),
		Owner:     lk.owner,
		Acquired:  lk.acquiredAt,
		Expires:   lk.expiresAt,
		Remaining: remaining,
		IsExpired: isExpired,
	}, nil
}

// Info returns lock information for this lock.
func (l *Lock) Info() (*LockInfo, error) {
	return GetLockInfo(l)
}

// String returns a human-readable representation of the lock info.
func (li *LockInfo) String() string {
	status := "active"
	if li.IsExpired {
		status = "expired"
	}

	return fmt.Sprintf("Lock{node=%s, owner=%s, status=%s, remaining=%s}",
		li.NodeID, li.Owner, status, li.Remaining.Round(time.Millisecond))
}
