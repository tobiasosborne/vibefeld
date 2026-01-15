// Package lock provides node locking for exclusive agent access.
package lock

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/tobias/vibefeld/internal/ledger"
)

// ReapStaleLocks finds and removes stale locks from the given directory.
// For each stale lock removed, a LockReaped event is appended to the ledger.
// Returns a slice of paths for the lock files that were reaped.
//
// The function only processes files with .lock extension and ignores other files.
// Corrupted lock files (invalid JSON) are skipped and an error is logged.
//
// Returns an error if:
// - locksDir is empty
// - locksDir does not exist or is not a directory
// - lg is nil
func ReapStaleLocks(locksDir string, lg *ledger.Ledger) ([]string, error) {
	// Validate inputs
	if locksDir == "" {
		return nil, errors.New("empty locks directory path")
	}
	if lg == nil {
		return nil, errors.New("ledger cannot be nil")
	}

	// Check directory exists
	info, err := os.Stat(locksDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("locks path is not a directory")
	}

	// Read all entries in the directory
	entries, err := os.ReadDir(locksDir)
	if err != nil {
		return nil, err
	}

	var reaped []string

	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		// Only process .lock files
		if !strings.HasSuffix(entry.Name(), ".lock") {
			continue
		}

		lockPath := filepath.Join(locksDir, entry.Name())

		// Read and parse the lock file
		data, err := os.ReadFile(lockPath)
		if err != nil {
			// Skip files we can't read
			continue
		}

		var lk ClaimLock
		if err := json.Unmarshal(data, &lk); err != nil {
			// Skip corrupted lock files
			continue
		}

		// Check if lock is stale (expired)
		if !lk.IsStale() {
			continue
		}

		// Remove the stale lock file
		if err := os.Remove(lockPath); err != nil {
			// If file doesn't exist, it was already removed (concurrent reap)
			if os.IsNotExist(err) {
				continue
			}
			// For other errors, skip this file
			continue
		}

		// Generate LockReaped event
		event := ledger.NewLockReaped(lk.NodeID(), lk.Owner())
		if _, err := lg.Append(event); err != nil {
			// Event append failed, but lock is already removed.
			// Continue processing other locks.
			continue
		}

		reaped = append(reaped, lockPath)
	}

	return reaped, nil
}
