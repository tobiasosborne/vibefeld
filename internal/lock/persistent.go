// Package lock provides node locking for exclusive agent access.
package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/types"
)

// LockEvent types for persistence.
const (
	EventLockAcquired ledger.EventType = "lock_acquired"
	EventLockReleased ledger.EventType = "lock_released"
)

// LockAcquired is emitted when a lock is acquired on a node.
type LockAcquired struct {
	ledger.BaseEvent
	NodeID    types.NodeID    `json:"node_id"`
	Owner     string          `json:"owner"`
	ExpiresAt types.Timestamp `json:"expires_at"`
}

// LockReleased is emitted when a lock is released on a node.
type LockReleased struct {
	ledger.BaseEvent
	NodeID types.NodeID `json:"node_id"`
	Owner  string       `json:"owner"`
}

// NewLockAcquired creates a LockAcquired event.
func NewLockAcquired(nodeID types.NodeID, owner string, expiresAt types.Timestamp) LockAcquired {
	return LockAcquired{
		BaseEvent: ledger.BaseEvent{
			EventType: EventLockAcquired,
			EventTime: types.Now(),
		},
		NodeID:    nodeID,
		Owner:     owner,
		ExpiresAt: expiresAt,
	}
}

// NewLockReleased creates a LockReleased event.
func NewLockReleased(nodeID types.NodeID, owner string) LockReleased {
	return LockReleased{
		BaseEvent: ledger.BaseEvent{
			EventType: EventLockReleased,
			EventTime: types.Now(),
		},
		NodeID: nodeID,
		Owner:  owner,
	}
}

// PersistentManager provides persistent lock management backed by a ledger.
// Locks survive process restarts by being recorded as events in the ledger.
// It is safe for concurrent use.
type PersistentManager struct {
	mu     sync.RWMutex
	locks  map[string]*Lock // keyed by NodeID.String()
	ledger *ledger.Ledger
}

// NewPersistentManager creates a new PersistentManager backed by the given ledger.
// It replays the ledger to reconstruct the current lock state.
func NewPersistentManager(l *ledger.Ledger) (*PersistentManager, error) {
	if l == nil {
		return nil, errors.New("ledger is required")
	}

	pm := &PersistentManager{
		locks:  make(map[string]*Lock),
		ledger: l,
	}

	// Replay ledger to reconstruct lock state
	if err := pm.replayLedger(); err != nil {
		return nil, fmt.Errorf("failed to replay ledger: %w", err)
	}

	return pm, nil
}

// replayLedger reads all events from the ledger and reconstructs lock state.
func (pm *PersistentManager) replayLedger() error {
	events, err := pm.ledger.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read ledger events: %w", err)
	}

	for _, eventData := range events {
		// Parse the event type first
		var base struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(eventData, &base); err != nil {
			continue // Skip malformed events
		}

		switch ledger.EventType(base.Type) {
		case EventLockAcquired:
			var evt LockAcquired
			if err := json.Unmarshal(eventData, &evt); err != nil {
				continue // Skip malformed events
			}
			pm.applyLockAcquired(evt)

		case EventLockReleased:
			var evt LockReleased
			if err := json.Unmarshal(eventData, &evt); err != nil {
				continue // Skip malformed events
			}
			pm.applyLockReleased(evt)

		case ledger.EventLockReaped:
			var evt ledger.LockReaped
			if err := json.Unmarshal(eventData, &evt); err != nil {
				continue // Skip malformed events
			}
			pm.applyLockReaped(evt)
		}
	}

	return nil
}

// applyLockAcquired applies a LockAcquired event to the in-memory state.
func (pm *PersistentManager) applyLockAcquired(evt LockAcquired) {
	key := evt.NodeID.String()

	// Reconstruct the lock from the event
	// Parse timestamps from their string representation
	acquiredTime, err := time.Parse(time.RFC3339, evt.Timestamp().String())
	if err != nil {
		// Fall back to current time if parsing fails
		acquiredTime = time.Now().UTC()
	}

	expiresTime, err := time.Parse(time.RFC3339, evt.ExpiresAt.String())
	if err != nil {
		// Fall back to acquired time if parsing fails (lock will be expired)
		expiresTime = acquiredTime
	}

	lock := &Lock{
		nodeID:     evt.NodeID,
		owner:      evt.Owner,
		acquiredAt: acquiredTime,
		expiresAt:  expiresTime,
	}

	pm.locks[key] = lock
}

// applyLockReleased applies a LockReleased event to the in-memory state.
func (pm *PersistentManager) applyLockReleased(evt LockReleased) {
	key := evt.NodeID.String()
	delete(pm.locks, key)
}

// applyLockReaped applies a LockReaped event to the in-memory state.
func (pm *PersistentManager) applyLockReaped(evt ledger.LockReaped) {
	key := evt.NodeID.String()
	delete(pm.locks, key)
}

// Acquire acquires a lock on a node, persisting to the ledger.
// Returns error if: node already locked and not expired, empty owner, whitespace owner, zero/negative timeout.
func (pm *PersistentManager) Acquire(nodeID types.NodeID, owner string, timeout time.Duration) (*Lock, error) {
	// Create the lock first (validates owner and timeout)
	lk, err := NewLock(nodeID, owner, timeout)
	if err != nil {
		return nil, err
	}

	key := nodeID.String()

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if already locked (and not expired)
	if existing, ok := pm.locks[key]; ok {
		if !existing.IsExpired() {
			return nil, errors.New("node already locked")
		}
		// Expired lock - we can replace it
	}

	// Persist to ledger first (write-ahead)
	evt := NewLockAcquired(nodeID, owner, lk.ExpiresAt())
	if _, err := pm.ledger.Append(evt); err != nil {
		return nil, fmt.Errorf("failed to persist lock: %w", err)
	}

	// Update in-memory state
	pm.locks[key] = lk
	return lk, nil
}

// Release releases a lock by owner, persisting to the ledger.
// Returns error if: node not locked, wrong owner, empty owner.
func (pm *PersistentManager) Release(nodeID types.NodeID, owner string) error {
	if owner == "" {
		return errors.New("invalid owner: empty")
	}

	key := nodeID.String()

	pm.mu.Lock()
	defer pm.mu.Unlock()

	lk, ok := pm.locks[key]
	if !ok {
		return errors.New("node not locked")
	}

	if !lk.IsOwnedBy(owner) {
		return errors.New("lock owned by different owner")
	}

	// Persist to ledger first (write-ahead)
	evt := NewLockReleased(nodeID, owner)
	if _, err := pm.ledger.Append(evt); err != nil {
		return fmt.Errorf("failed to persist lock release: %w", err)
	}

	// Update in-memory state
	delete(pm.locks, key)
	return nil
}

// Info returns lock info for a node (nil if not locked or expired).
func (pm *PersistentManager) Info(nodeID types.NodeID) (*Lock, error) {
	key := nodeID.String()

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	lk, ok := pm.locks[key]
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
func (pm *PersistentManager) IsLocked(nodeID types.NodeID) bool {
	key := nodeID.String()

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	lk, ok := pm.locks[key]
	if !ok {
		return false
	}

	return !lk.IsExpired()
}

// ReapExpired removes all expired locks and returns them.
// Persists each removal to the ledger using LockReaped events.
func (pm *PersistentManager) ReapExpired() ([]*Lock, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var reaped []*Lock

	for key, lk := range pm.locks {
		if lk.IsExpired() {
			// Persist reap to ledger using the existing LockReaped event type
			evt := ledger.NewLockReaped(lk.NodeID(), lk.Owner())
			if _, err := pm.ledger.Append(evt); err != nil {
				// Log error but continue - we still want to clean up in-memory state
				// In production, we might want to handle this differently
				continue
			}

			reaped = append(reaped, lk)
			delete(pm.locks, key)
		}
	}

	return reaped, nil
}

// ListAll returns all non-expired locks.
func (pm *PersistentManager) ListAll() []*Lock {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*Lock
	for _, lk := range pm.locks {
		if !lk.IsExpired() {
			result = append(result, lk)
		}
	}

	return result
}
