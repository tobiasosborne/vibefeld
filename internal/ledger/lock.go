// Package ledger provides event-sourced ledger operations for the AF proof framework.
package ledger

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
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
	token    string // Unique token generated at Acquire, recorded in the lock file
	mu       sync.Mutex
	lockPath string
}

// lockMetadata stores the information written to the lock file.
//
// Legacy lock files contained only the agent ID as plain text; readLockFile
// still understands those. New lock files are JSON with agent_id, pid,
// acquired_at, and token. PID is 0 when unknown (legacy file), which callers
// treat as "liveness unknown" and fall back to the acquired_at age. Token is
// empty for legacy files.
type lockMetadata struct {
	AgentID    string    `json:"agent_id"`
	PID        int       `json:"pid,omitempty"`
	AcquiredAt time.Time `json:"acquired_at"`
	Token      string    `json:"token,omitempty"`
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
		Token:      newLockToken(),
	}
	if err := json.NewEncoder(f).Encode(&meta); err != nil {
		// Best-effort cleanup: remove the lock file we just created.
		// Ignore error since we're already returning an error and don't want to mask it.
		_ = os.Remove(l.lockPath)
		return err
	}

	l.held = true
	l.agentID = agentID
	l.token = meta.Token
	return nil
}

// newLockToken returns a random hex token unique to one acquisition. It is
// stored in the lock file so Release can remove that exact file and not one
// written by a different holder that reused the same agent id.
func newLockToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read never fails on supported platforms; fall back to a
		// time-derived value rather than returning an unusable lock.
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

// Release releases the lock if held.
// Returns an error if the lock is not held, was already released, or if the
// lock file on disk is not the one this instance acquired. Ownership is
// established by the unique token written at Acquire: if the on-disk file
// carries a different token it was re-acquired by someone else and is left
// alone. Legacy plain-text lock files have no token and fall back to comparing
// the agent id.
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

	if !lockOwnedBy(meta, l.agentID, l.token) {
		return errors.New("lock ownership mismatch: lock held by " + meta.AgentID + ", not " + l.agentID)
	}

	err = os.Remove(l.lockPath)
	if err != nil {
		return err
	}

	l.held = false
	l.agentID = ""
	l.token = ""
	return nil
}

// lockOwnedBy reports whether on-disk metadata belongs to the lock instance
// identified by agentID and token. A token mismatch is fatal (a different
// holder re-acquired); legacy metadata without a token falls back to the agent
// id for backwards compatibility.
func lockOwnedBy(meta lockMetadata, agentID, token string) bool {
	if meta.Token != "" {
		return token != "" && meta.Token == token
	}
	return meta.AgentID == agentID
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
	return parseLockData(path, data)
}

// parseLockData parses already-read lock file bytes. path is used only to stat
// the file for a legacy lock's fallback acquisition time.
func parseLockData(path string, data []byte) (lockMetadata, error) {
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

// staleLockReason reports whether meta is stale and why. A live PID is never
// stale; a lock with no PID falls back to the acquired_at age.
func staleLockReason(meta lockMetadata, timeout time.Duration) (bool, string) {
	if meta.PID > 0 {
		if IsProcessAlive(meta.PID) {
			return false, ""
		}
		return true, "holder pid " + strconv.Itoa(meta.PID) + " is not alive"
	}

	// Legacy or missing pid: fall back to age.
	if !meta.AcquiredAt.IsZero() && timeout > 0 && time.Since(meta.AcquiredAt) > timeout {
		return true, "lock held by " + meta.AgentID + " is older than " + timeout.String()
	}
	return false, ""
}

// StaleLock returns whether the ledger lock in dir should be considered stale,
// and a human-readable reason. A lock is stale when its recorded PID is not
// alive, or when it has no PID and its acquired_at is older than timeout. A
// live PID is never stale. ok is false when there is no lock file.
func StaleLock(dir string, timeout time.Duration) (stale bool, reason string, ok bool, err error) {
	meta, err := readLockFile(filepath.Join(dir, lockFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", false, nil
		}
		return false, "", false, err
	}

	stale, reason = staleLockReason(meta, timeout)
	return stale, reason, true, nil
}

// RemoveIfStale removes the ledger lock file in dir if it is stale (dead pid,
// or no pid and older than timeout). It never removes a lock whose pid is
// alive. If the lock is not stale or no lock exists, it returns (false, nil).
//
// Removal is a best-effort compare-and-remove: the file is read, judged stale,
// then re-read immediately before unlink; it is removed only if the identity is
// unchanged (same token for token-bearing files, or byte-identical content and
// mtime for legacy files). There remains a residual window between the final
// re-read and os.Remove, during which another process could replace the lock;
// a wrongly removed stale lock only forces the next writer to retry, so this is
// acceptable.
func RemoveIfStale(dir string, timeout time.Duration) (removed bool, err error) {
	path := filepath.Join(dir, lockFileName)

	before, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	meta, err := parseLockData(path, before)
	if err != nil {
		return false, err
	}
	if stale, _ := staleLockReason(meta, timeout); !stale {
		return false, nil
	}

	after, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !sameLockForRemoval(path, before, after, meta) {
		return false, nil
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// sameLockForRemoval reports whether the after contents still identify the same
// lock as before/meta: a matching token for token-bearing files, or identical
// bytes and mtime for legacy files.
func sameLockForRemoval(path string, before, after []byte, meta lockMetadata) bool {
	later, err := parseLockData(path, after)
	if err != nil {
		return false
	}
	if meta.Token != "" {
		return later.Token == meta.Token && later.AgentID == meta.AgentID
	}
	// Legacy content: require the raw bytes and the mtime (folded into the
	// parsed AcquiredAt) to be unchanged.
	return bytes.Equal(before, after) && later.AcquiredAt.Equal(meta.AcquiredAt)
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
