// Package lock provides node locking for exclusive agent access.
package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	aferrors "github.com/tobias/vibefeld/internal/errors"
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
	locks  map[string]*ClaimLock // keyed by NodeID.String()
	ledger *ledger.Ledger
}

// NewPersistentManager creates a new PersistentManager backed by the given ledger.
// It replays the ledger to reconstruct the current lock state.
func NewPersistentManager(l *ledger.Ledger) (*PersistentManager, error) {
	if l == nil {
		return nil, errors.New("ledger is required")
	}

	pm := &PersistentManager{
		locks:  make(map[string]*ClaimLock),
		ledger: l,
	}

	// Replay ledger to reconstruct lock state
	if err := pm.replayLedger(); err != nil {
		return nil, fmt.Errorf("failed to replay ledger: %w", err)
	}

	return pm, nil
}

// CorruptedEvent records details about a malformed event during replay.
type CorruptedEvent struct {
	Index     int    // Position in the event sequence
	EventType string // The event type (if parseable), or "unknown"
	Error     error  // The parsing error
}

// ReplayResult contains information about events skipped during replay.
type ReplayResult struct {
	CorruptedEvents []CorruptedEvent
}

// HasCorruption returns true if any events were skipped due to corruption.
func (r *ReplayResult) HasCorruption() bool {
	return len(r.CorruptedEvents) > 0
}

// replayLedger reads all events from the ledger and reconstructs lock state.
// Returns an error if critical lock events are corrupted (lock_acquired, lock_released, lock_reaped).
func (pm *PersistentManager) replayLedger() error {
	events, err := pm.ledger.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read ledger events: %w", err)
	}

	var corruptedEvents []CorruptedEvent

	for i, eventData := range events {
		// Parse the event type first
		var base struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(eventData, &base); err != nil {
			// Only record if this might be a lock event (we can't tell, so check raw data)
			if isLikelyLockEvent(eventData) {
				corruptedEvents = append(corruptedEvents, CorruptedEvent{
					Index:     i,
					EventType: "unknown",
					Error:     fmt.Errorf("failed to parse event type: %w", err),
				})
			}
			continue
		}

		switch ledger.EventType(base.Type) {
		case EventLockAcquired:
			var evt LockAcquired
			if err := json.Unmarshal(eventData, &evt); err != nil {
				corruptedEvents = append(corruptedEvents, CorruptedEvent{
					Index:     i,
					EventType: string(EventLockAcquired),
					Error:     fmt.Errorf("failed to parse lock_acquired event: %w", err),
				})
				continue
			}
			pm.applyLockAcquired(evt)

		case EventLockReleased:
			var evt LockReleased
			if err := json.Unmarshal(eventData, &evt); err != nil {
				corruptedEvents = append(corruptedEvents, CorruptedEvent{
					Index:     i,
					EventType: string(EventLockReleased),
					Error:     fmt.Errorf("failed to parse lock_released event: %w", err),
				})
				continue
			}
			pm.applyLockReleased(evt)

		case ledger.EventLockReaped:
			var evt ledger.LockReaped
			if err := json.Unmarshal(eventData, &evt); err != nil {
				corruptedEvents = append(corruptedEvents, CorruptedEvent{
					Index:     i,
					EventType: string(ledger.EventLockReaped),
					Error:     fmt.Errorf("failed to parse lock_reaped event: %w", err),
				})
				continue
			}
			pm.applyLockReaped(evt)
		}
		// Non-lock events are silently skipped (expected behavior)
	}

	// If any lock events were corrupted, return a corruption error
	if len(corruptedEvents) > 0 {
		return aferrors.Newf(aferrors.LEDGER_INCONSISTENT,
			"lock ledger corruption: %d lock event(s) skipped during replay: %v",
			len(corruptedEvents), formatCorruptedEvents(corruptedEvents))
	}

	return nil
}

// isLikelyLockEvent checks if raw event data might be a lock event.
// This is used to detect corruption in events that can't be parsed.
func isLikelyLockEvent(data []byte) bool {
	// Check if the raw data contains lock event type strings
	s := string(data)
	return contains(s, "lock_acquired") || contains(s, "lock_released") || contains(s, "lock_reaped")
}

// contains checks if s contains substr (simple substring check).
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// formatCorruptedEvents formats corrupted events for error messages.
func formatCorruptedEvents(events []CorruptedEvent) string {
	if len(events) == 0 {
		return ""
	}
	if len(events) == 1 {
		e := events[0]
		return fmt.Sprintf("[event %d (%s): %v]", e.Index, e.EventType, e.Error)
	}
	// For multiple events, summarize
	result := "["
	for i, e := range events {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("event %d (%s)", e.Index, e.EventType)
		if i >= 2 && len(events) > 3 {
			result += fmt.Sprintf(", ... and %d more", len(events)-3)
			break
		}
	}
	result += "]"
	return result
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

	claimLock := &ClaimLock{
		nodeID:     evt.NodeID,
		owner:      evt.Owner,
		acquiredAt: acquiredTime,
		expiresAt:  expiresTime,
	}

	pm.locks[key] = claimLock
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

// verifyLockHolder verifies that after writing a lock_acquired event at sequence seq,
// we are the actual current holder of the lock. This handles TOCTOU races where
// another process might have acquired the same lock between our in-memory check
// and our ledger write.
//
// Returns nil if we are the valid lock holder, or an error if:
// - Another process acquired the lock for the same node before our event
// - We cannot read the ledger to verify
func (pm *PersistentManager) verifyLockHolder(nodeID types.NodeID, owner string, seq int) error {
	events, err := pm.ledger.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to verify lock holder: %w", err)
	}

	targetKey := nodeID.String()

	// Track lock state for the target node as we scan through events
	type lockState struct {
		owner    string
		sequence int
	}
	var currentLock *lockState

	for i, eventData := range events {
		eventSeq := i + 1 // Events are 1-indexed

		// Parse the event type
		var base struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(eventData, &base); err != nil {
			continue // Skip unparseable events (handled by replayLedger)
		}

		switch ledger.EventType(base.Type) {
		case EventLockAcquired:
			var evt LockAcquired
			if err := json.Unmarshal(eventData, &evt); err != nil {
				continue
			}
			if evt.NodeID.String() == targetKey {
				currentLock = &lockState{owner: evt.Owner, sequence: eventSeq}
			}

		case EventLockReleased:
			var evt LockReleased
			if err := json.Unmarshal(eventData, &evt); err != nil {
				continue
			}
			if evt.NodeID.String() == targetKey {
				currentLock = nil
			}

		case ledger.EventLockReaped:
			var evt ledger.LockReaped
			if err := json.Unmarshal(eventData, &evt); err != nil {
				continue
			}
			if evt.NodeID.String() == targetKey {
				currentLock = nil
			}
		}
	}

	// Verify we are the current lock holder
	if currentLock == nil {
		// Lock was released or reaped after we wrote - this shouldn't happen
		// but indicates a concurrent modification
		return errors.New("lock was released concurrently")
	}

	if currentLock.sequence != seq || currentLock.owner != owner {
		// Another process acquired the lock before or after our write
		return fmt.Errorf("lock acquisition conflict: node locked by %s", currentLock.owner)
	}

	return nil
}

// Acquire acquires a lock on a node, persisting to the ledger.
// Returns error if: node already locked and not expired, empty owner, whitespace owner, zero/negative timeout.
func (pm *PersistentManager) Acquire(nodeID types.NodeID, owner string, timeout time.Duration) (*ClaimLock, error) {
	// Create the lock first (validates owner and timeout)
	lk, err := NewClaimLock(nodeID, owner, timeout)
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
	seq, err := pm.ledger.Append(evt)
	if err != nil {
		return nil, fmt.Errorf("failed to persist lock: %w", err)
	}

	// Verify we are the actual lock holder by checking ledger state.
	// This handles TOCTOU race where another process may have acquired
	// the same lock between our in-memory check and ledger write.
	if err := pm.verifyLockHolder(nodeID, owner, seq); err != nil {
		return nil, err
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
func (pm *PersistentManager) Info(nodeID types.NodeID) (*ClaimLock, error) {
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
func (pm *PersistentManager) ReapExpired() ([]*ClaimLock, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var reaped []*ClaimLock

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
func (pm *PersistentManager) ListAll() []*ClaimLock {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*ClaimLock
	for _, lk := range pm.locks {
		if !lk.IsExpired() {
			result = append(result, lk)
		}
	}

	return result
}
