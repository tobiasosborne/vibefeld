// Package ledger provides event-sourced ledger operations for the AF proof framework.
package ledger

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// LedgerLock provides file-based mutual exclusion for ledger operations.
// It uses atomic file creation (O_CREATE|O_EXCL) to ensure only one holder.
type LedgerLock struct {
	dir      string
	held     bool
	agentID  string // The agent ID that acquired the lock (empty if not held)
	mu       sync.Mutex
	lockPath string
}

// lockMetadata stores the information written to the lock file.
//
// Legacy lock files contained only the agent ID as plain text; readLockFile
// still understands those. New lock files are JSON with agent_id, pid, and
// acquired_at. PID is 0 when unknown (legacy file), which callers treat as
// "liveness unknown" and fall back to the acquired_at age.
type lockMetadata struct {
	AgentID    string    `json:"agent_id"`
	PID        int       `json:"pid,omitempty"`
	AcquiredAt time.Time `json:"acquired_at"`
}

const lockFileName = "ledger.lock"
const pollInterval = 10 * time.Millisecond

// ErrLockHeldByDifferentAgent is returned when tryAcquire is called on a LedgerLock
// instance that is already held by a different agent. This is a fatal error that
// should not be retried - it indicates misuse of the LedgerLock instance.
var ErrLockHeldByDifferentAgent = errors.New("lock held by different agent on same instance")

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

		// Fatal error: lock held by different agent on same instance (misuse)
		if errors.Is(err, ErrLockHeldByDifferentAgent) {
			return err
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

	// Already holding this lock - verify same agent (re-entrant case)
	if l.held {
		if l.agentID == agentID {
			return nil
		}
		return ErrLockHeldByDifferentAgent
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
		PID:        os.Getpid(),
		AcquiredAt: time.Now(),
	}
	if err := json.NewEncoder(f).Encode(&meta); err != nil {
		// Best-effort cleanup: remove the lock file we just created.
		// Ignore error since we're already returning an error and don't want to mask it.
		_ = os.Remove(l.lockPath)
		return err
	}

	l.held = true
	l.agentID = agentID
	return nil
}

// Release releases the lock if held.
// Returns an error if the lock is not held, was already released, or if
// the lock file metadata doesn't match the agent ID that acquired it.
func (l *LedgerLock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.held {
		return errors.New("lock not held")
	}

	// Verify ownership by reading lock file metadata
	meta, err := readLockFile(l.lockPath)
	if err != nil {
		return errors.New("failed to read lock file: " + err.Error())
	}

	if meta.AgentID != l.agentID {
		return errors.New("lock ownership mismatch: lock held by " + meta.AgentID + ", not " + l.agentID)
	}

	err = os.Remove(l.lockPath)
	if err != nil {
		return err
	}

	l.held = false
	l.agentID = ""
	return nil
}

// IsHeld returns true if this LedgerLock instance currently holds the lock.
func (l *LedgerLock) IsHeld() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.held
}

// Holder reads the lock file and returns the agent ID and acquisition time.
// Returns an error if no lock file exists. Legacy plain-text lock files are
// understood; their acquisition time is the zero time.
func (l *LedgerLock) Holder() (agentID string, acquiredAt time.Time, err error) {
	meta, err := readLockFile(l.lockPath)
	if err != nil {
		return "", time.Time{}, err
	}

	return meta.AgentID, meta.AcquiredAt, nil
}

// Inspect returns the full metadata of the lock in dir, if one exists.
// Legacy plain-text lock files are understood (PID 0, zero acquired_at).
func Inspect(dir string) (agentID string, pid int, acquiredAt time.Time, present bool, err error) {
	meta, err := readLockFile(filepath.Join(dir, lockFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, time.Time{}, false, nil
		}
		return "", 0, time.Time{}, false, err
	}
	return meta.AgentID, meta.PID, meta.AcquiredAt, true, nil
}

// readLockFile reads and parses a lock file, tolerating legacy plain-text
// content (a bare agent ID). JSON that parses but has an empty agent_id is an
// error: a corrupt lock file must never be treated as an anonymous holder.
func readLockFile(path string) (lockMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return lockMetadata{}, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return lockMetadata{}, errors.New("empty lock file")
	}

	if strings.HasPrefix(trimmed, "{") {
		var meta lockMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			return lockMetadata{}, err
		}
		if meta.AgentID == "" {
			return lockMetadata{}, errors.New("lock file missing agent_id")
		}
		return meta, nil
	}

	// Legacy plain-text lock file: the whole content is the agent ID. There is
	// no recorded acquisition time, so fall back to the file's modification
	// time for the staleness age check.
	meta := lockMetadata{AgentID: trimmed}
	if info, statErr := os.Stat(path); statErr == nil {
		meta.AcquiredAt = info.ModTime()
	}
	return meta, nil
}

// IsProcessAlive reports whether a process with the given pid currently
// exists, using kill(pid, 0) semantics. pid <= 0 is treated as unknown/dead
// by the caller; this helper returns false for it.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, Signal(0) performs the existence/permission check without
	// delivering a signal. nil means the process exists and we may signal it;
	// EPERM means it exists but is owned by another user (still alive).
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	return errors.Is(err, syscall.EPERM)
}

// StaleLock returns whether the ledger lock in dir should be considered stale,
// and a human-readable reason. A lock is stale when its recorded PID is not
// alive, or when it has no PID and its acquired_at is older than timeout. A
// live PID is never stale. ok is false when there is no lock file.
func StaleLock(dir string, timeout time.Duration) (stale bool, reason string, ok bool, err error) {
	agentID, pid, acquiredAt, present, err := Inspect(dir)
	if err != nil {
		return false, "", false, err
	}
	if !present {
		return false, "", false, nil
	}

	if pid > 0 {
		if IsProcessAlive(pid) {
			return false, "", true, nil
		}
		return true, "holder pid " + strconv.Itoa(pid) + " is not alive", true, nil
	}

	// Legacy or missing pid: fall back to age.
	if !acquiredAt.IsZero() && timeout > 0 && time.Since(acquiredAt) > timeout {
		return true, "lock held by " + agentID + " is older than " + timeout.String(), true, nil
	}
	return false, "", true, nil
}

// RemoveLockFile removes the ledger lock file in dir unconditionally. Callers
// must have established staleness first (via StaleLock).
func RemoveLockFile(dir string) error {
	err := os.Remove(filepath.Join(dir, lockFileName))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
