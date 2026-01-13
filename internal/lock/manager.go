package lock

import (
	"errors"
	"sync"
	"time"

	"github.com/tobias/vibefeld/internal/types"
)

// Manager provides a facade for managing locks on proof nodes.
// It is safe for concurrent use.
type Manager struct {
	mu    sync.RWMutex
	locks map[string]*Lock // keyed by NodeID.String()
}

// NewManager creates a new Manager with an empty lock set.
func NewManager() *Manager {
	return &Manager{
		locks: make(map[string]*Lock),
	}
}

// Acquire acquires a lock on a node.
// Returns error if: node already locked and not expired, empty owner, whitespace owner, zero/negative timeout.
func (m *Manager) Acquire(nodeID types.NodeID, owner string, timeout time.Duration) (*Lock, error) {
	// Create the lock first (validates owner and timeout)
	lk, err := NewLock(nodeID, owner, timeout)
	if err != nil {
		return nil, err
	}

	key := nodeID.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already locked (and not expired)
	if existing, ok := m.locks[key]; ok {
		if !existing.IsExpired() {
			return nil, errors.New("node already locked")
		}
		// Expired lock - we can replace it
	}

	m.locks[key] = lk
	return lk, nil
}

// Release releases a lock by owner.
// Returns error if: node not locked, wrong owner, empty owner.
func (m *Manager) Release(nodeID types.NodeID, owner string) error {
	if owner == "" {
		return errors.New("invalid owner: empty")
	}

	key := nodeID.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	lk, ok := m.locks[key]
	if !ok {
		return errors.New("node not locked")
	}

	if !lk.IsOwnedBy(owner) {
		return errors.New("lock owned by different owner")
	}

	delete(m.locks, key)
	return nil
}

// Info returns lock info for a node (nil if not locked or expired).
func (m *Manager) Info(nodeID types.NodeID) (*Lock, error) {
	key := nodeID.String()

	m.mu.RLock()
	defer m.mu.RUnlock()

	lk, ok := m.locks[key]
	if !ok {
		return nil, nil
	}

	// Don't return expired locks
	if lk.IsExpired() {
		return nil, nil
	}

	return lk, nil
}

// IsLocked returns true if node is locked and not expired.
func (m *Manager) IsLocked(nodeID types.NodeID) bool {
	key := nodeID.String()

	m.mu.RLock()
	defer m.mu.RUnlock()

	lk, ok := m.locks[key]
	if !ok {
		return false
	}

	return !lk.IsExpired()
}

// ReapExpired removes all expired locks and returns them.
func (m *Manager) ReapExpired() ([]*Lock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var reaped []*Lock

	for key, lk := range m.locks {
		if lk.IsExpired() {
			reaped = append(reaped, lk)
			delete(m.locks, key)
		}
	}

	return reaped, nil
}

// ListAll returns all non-expired locks.
func (m *Manager) ListAll() []*Lock {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Lock
	for _, lk := range m.locks {
		if !lk.IsExpired() {
			result = append(result, lk)
		}
	}

	return result
}
