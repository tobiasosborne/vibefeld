// Package ledger provides event-sourced ledger operations for the AF proof framework.
package ledger

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LedgerLock provides file-based mutual exclusion for ledger operations.
// It uses atomic file creation (O_CREATE|O_EXCL) to ensure only one holder.
type LedgerLock struct {
	dir      string
	held     bool
	mu       sync.Mutex
	lockPath string
}

// lockMetadata stores the information written to the lock file.
type lockMetadata struct {
	AgentID    string    `json:"agent_id"`
	AcquiredAt time.Time `json:"acquired_at"`
}

const lockFileName = "ledger.lock"
const pollInterval = 10 * time.Millisecond

// NewLedgerLock creates a new LedgerLock for the given directory.
func NewLedgerLock(dir string) *LedgerLock {
	return &LedgerLock{
		dir:      dir,
		lockPath: filepath.Join(dir, lockFileName),
		held:     false,
	}
}

// Acquire attempts to acquire the lock for the given agent.
// If the lock is already held, it will retry with polling until timeout.
// An empty agentID or non-existent directory will return an error.
func (l *LedgerLock) Acquire(agentID string, timeout time.Duration) error {
	if agentID == "" {
		return errors.New("agent ID cannot be empty")
	}

	// Check if directory exists
	if _, err := os.Stat(l.dir); os.IsNotExist(err) {
		return errors.New("directory does not exist")
	}

	deadline := time.Now().Add(timeout)

	for {
		err := l.tryAcquire(agentID)
		if err == nil {
			return nil
		}

		// If we couldn't acquire and we're past the deadline, fail
		if time.Now().After(deadline) {
			return errors.New("timeout waiting for lock")
		}

		// Wait before retrying
		time.Sleep(pollInterval)
	}
}

// tryAcquire attempts a single lock acquisition.
func (l *LedgerLock) tryAcquire(agentID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Already holding this lock
	if l.held {
		return nil
	}

	// Try to create lock file exclusively
	f, err := os.OpenFile(l.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write metadata
	meta := lockMetadata{
		AgentID:    agentID,
		AcquiredAt: time.Now(),
	}
	if err := json.NewEncoder(f).Encode(&meta); err != nil {
		// Best-effort cleanup: remove the lock file we just created.
		// Ignore error since we're already returning an error and don't want to mask it.
		_ = os.Remove(l.lockPath)
		return err
	}

	l.held = true
	return nil
}

// Release releases the lock if held.
// Returns an error if the lock is not held or was already released.
func (l *LedgerLock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.held {
		return errors.New("lock not held")
	}

	err := os.Remove(l.lockPath)
	if err != nil {
		return err
	}

	l.held = false
	return nil
}

// IsHeld returns true if this LedgerLock instance currently holds the lock.
func (l *LedgerLock) IsHeld() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.held
}

// Holder reads the lock file and returns the agent ID and acquisition time.
// Returns an error if no lock file exists.
func (l *LedgerLock) Holder() (agentID string, acquiredAt time.Time, err error) {
	data, err := os.ReadFile(l.lockPath)
	if err != nil {
		return "", time.Time{}, err
	}

	var meta lockMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", time.Time{}, err
	}

	return meta.AgentID, meta.AcquiredAt, nil
}
