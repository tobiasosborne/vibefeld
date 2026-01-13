// Package ledger provides event-sourced ledger operations for the AF proof framework.
package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Default lock timeout for append operations.
const defaultLockTimeout = 5 * time.Second

// ErrSequenceMismatch is returned when an AppendIfSequence operation fails
// because the ledger has been modified since the expected sequence was observed.
// This indicates a concurrent modification and the operation should be retried
// after reloading the current state.
var ErrSequenceMismatch = errors.New("ledger sequence mismatch: concurrent modification detected")

// cleanupTempFiles removes temporary files from the given slice within the range [start, end).
// This is a best-effort operation; errors are intentionally ignored since cleanup failures
// should not mask the original error that triggered the cleanup.
func cleanupTempFiles(tempPaths []string, start, end int) {
	for i := start; i < end; i++ {
		if tempPaths[i] != "" {
			// Intentionally ignore error: this is best-effort cleanup and the file
			// may already be removed or renamed. We don't want cleanup failures to
			// mask the original error.
			_ = os.Remove(tempPaths[i])
		}
	}
}

// validateDirectory checks that dir is a non-empty path to an existing directory.
// Returns an error if validation fails.
func validateDirectory(dir string) error {
	if dir == "" {
		return fmt.Errorf("empty directory path")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dir)
	}

	return nil
}

// Append adds an event to the ledger at the given directory.
// Returns the sequence number assigned to the event.
// The write is atomic: the event is first written to a temp file, then renamed.
// Uses file-based locking to ensure concurrent safety.
func Append(dir string, event Event) (int, error) {
	return AppendWithTimeout(dir, event, defaultLockTimeout)
}

// AppendWithTimeout adds an event to the ledger with a custom lock timeout.
// This is useful for testing or when you need faster failure in constrained environments.
func AppendWithTimeout(dir string, event Event, timeout time.Duration) (int, error) {
	if err := validateDirectory(dir); err != nil {
		return 0, err
	}

	// Acquire lock for concurrent safety with custom timeout
	lock := NewLedgerLock(dir)
	if err := lock.Acquire("append-operation", timeout); err != nil {
		return 0, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Release()

	// Get next sequence number (inside lock to ensure atomicity)
	seq, err := NextSequence(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to determine next sequence: %w", err)
	}

	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create temp file for atomic write
	tempFile, err := os.CreateTemp(dir, ".event-*.tmp")
	if err != nil {
		return 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Write data to temp file
	_, err = tempFile.Write(data)
	if err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath) // Best-effort cleanup; don't mask the write error
		return 0, fmt.Errorf("failed to write event data: %w", err)
	}

	// Sync to ensure data is on disk
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath) // Best-effort cleanup; don't mask the sync error
		return 0, fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath) // Best-effort cleanup; don't mask the close error
		return 0, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set file permissions
	if err := os.Chmod(tempPath, 0644); err != nil {
		_ = os.Remove(tempPath) // Best-effort cleanup; don't mask the chmod error
		return 0, fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomic rename to final path
	finalPath := filepath.Join(dir, GenerateFilename(seq))
	if err := os.Rename(tempPath, finalPath); err != nil {
		_ = os.Remove(tempPath) // Best-effort cleanup; don't mask the rename error
		return 0, fmt.Errorf("failed to rename temp file: %w", err)
	}

	return seq, nil
}

// AppendIfSequence adds an event to the ledger only if the current sequence
// matches the expected value. This implements Compare-And-Swap (CAS) semantics
// for optimistic concurrency control.
//
// expectedSeq should be the sequence number of the last event observed when
// the state was loaded (i.e., state.LatestSeq()). If the ledger has been
// modified since then, ErrSequenceMismatch is returned.
//
// Returns the new sequence number on success, or ErrSequenceMismatch if the
// ledger was concurrently modified. Other errors indicate infrastructure failures.
func AppendIfSequence(dir string, event Event, expectedSeq int) (int, error) {
	return AppendIfSequenceWithTimeout(dir, event, expectedSeq, defaultLockTimeout)
}

// AppendIfSequenceWithTimeout is like AppendIfSequence but with a custom lock timeout.
func AppendIfSequenceWithTimeout(dir string, event Event, expectedSeq int, timeout time.Duration) (int, error) {
	if err := validateDirectory(dir); err != nil {
		return 0, err
	}

	// Acquire lock for concurrent safety with custom timeout
	lock := NewLedgerLock(dir)
	if err := lock.Acquire("append-if-sequence-operation", timeout); err != nil {
		return 0, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Release()

	// Get current sequence number (inside lock to ensure atomicity)
	currentSeq, err := NextSequence(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to determine current sequence: %w", err)
	}

	// NextSequence returns the NEXT sequence (current + 1), so current count is currentSeq - 1
	// If expectedSeq is 0, it means we expect an empty ledger (next would be 1)
	// If expectedSeq is N, we expect the ledger to have N events (next would be N+1)
	actualLatest := currentSeq - 1
	if actualLatest != expectedSeq {
		return 0, fmt.Errorf("%w: expected sequence %d, but ledger is at %d",
			ErrSequenceMismatch, expectedSeq, actualLatest)
	}

	// Sequence matches - proceed with append (same logic as AppendWithTimeout)
	seq := currentSeq

	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create temp file for atomic write
	tempFile, err := os.CreateTemp(dir, ".event-*.tmp")
	if err != nil {
		return 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Write data to temp file
	_, err = tempFile.Write(data)
	if err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("failed to write event data: %w", err)
	}

	// Sync to ensure data is on disk
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set file permissions
	if err := os.Chmod(tempPath, 0644); err != nil {
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomic rename to final path
	finalPath := filepath.Join(dir, GenerateFilename(seq))
	if err := os.Rename(tempPath, finalPath); err != nil {
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("failed to rename temp file: %w", err)
	}

	return seq, nil
}

// AppendBatch adds multiple events atomically to the ledger.
// Returns the sequence numbers assigned to each event.
// Events are appended in order, with consecutive sequence numbers.
// Uses file-based locking to ensure concurrent safety.
func AppendBatch(dir string, events []Event) ([]int, error) {
	if len(events) == 0 {
		return nil, nil
	}

	if err := validateDirectory(dir); err != nil {
		return nil, err
	}

	// Acquire lock for concurrent safety
	lock := NewLedgerLock(dir)
	if err := lock.Acquire("append-batch-operation", defaultLockTimeout); err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Release()

	// Get starting sequence number (inside lock to ensure atomicity)
	startSeq, err := NextSequence(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to determine next sequence: %w", err)
	}

	seqs := make([]int, len(events))
	tempPaths := make([]string, len(events))

	// Create all temp files first
	for i, event := range events {
		seq := startSeq + i
		seqs[i] = seq

		// Marshal event to JSON
		data, err := json.Marshal(event)
		if err != nil {
			cleanupTempFiles(tempPaths, 0, i)
			return nil, fmt.Errorf("failed to marshal event %d: %w", i, err)
		}

		// Create temp file
		tempFile, err := os.CreateTemp(dir, ".event-*.tmp")
		if err != nil {
			cleanupTempFiles(tempPaths, 0, i)
			return nil, fmt.Errorf("failed to create temp file for event %d: %w", i, err)
		}
		tempPaths[i] = tempFile.Name()

		// Write data
		_, err = tempFile.Write(data)
		if err != nil {
			tempFile.Close()
			cleanupTempFiles(tempPaths, 0, i+1)
			return nil, fmt.Errorf("failed to write event %d: %w", i, err)
		}

		// Sync and close
		if err := tempFile.Sync(); err != nil {
			tempFile.Close()
			cleanupTempFiles(tempPaths, 0, i+1)
			return nil, fmt.Errorf("failed to sync event %d: %w", i, err)
		}

		if err := tempFile.Close(); err != nil {
			cleanupTempFiles(tempPaths, 0, i+1)
			return nil, fmt.Errorf("failed to close temp file for event %d: %w", i, err)
		}

		// Set permissions
		if err := os.Chmod(tempPaths[i], 0644); err != nil {
			cleanupTempFiles(tempPaths, 0, i+1)
			return nil, fmt.Errorf("failed to set permissions for event %d: %w", i, err)
		}
	}

	// Rename all temp files to final paths
	for i := range events {
		seq := seqs[i]
		finalPath := filepath.Join(dir, GenerateFilename(seq))
		if err := os.Rename(tempPaths[i], finalPath); err != nil {
			// Note: Partial failure here leaves some files renamed.
			// In a production system, we might want to implement rollback.
			// For now, cleanup remaining temp files.
			cleanupTempFiles(tempPaths, i, len(events))
			return nil, fmt.Errorf("failed to rename event %d: %w", i, err)
		}
	}

	return seqs, nil
}
