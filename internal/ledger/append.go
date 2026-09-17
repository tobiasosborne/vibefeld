// Package ledger provides event-sourced ledger operations for the AF proof framework.
package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// crashAfterRenameEnv is a test-only hook: when set to a positive integer n,
// AppendBatchIfSequence exits the process immediately after the n-th successful
// rename. It lets a test re-exec the test binary to reproduce a mid-batch
// crash and assert that only a contiguous prefix of the batch was committed.
// It is never set in production.
const crashAfterRenameEnv = "AF_TEST_CRASH_AFTER_RENAME"

// maybeCrashAfterRename exits the process if the test-only crash hook asks for a
// crash after rename number n (1-based). No-op otherwise.
func maybeCrashAfterRename(n int) {
	v := os.Getenv(crashAfterRenameEnv)
	if v == "" {
		return
	}
	want, err := strconv.Atoi(v)
	if err != nil || want <= 0 {
		return
	}
	if n == want {
		os.Exit(3)
	}
}

// Default lock timeout for append operations.
const defaultLockTimeout = 5 * time.Second

// ErrSequenceMismatch is returned when an AppendIfSequence operation fails
// because the ledger has been modified since the expected sequence was observed.
// This indicates a concurrent modification and the operation should be retried
// after reloading the current state.
var ErrSequenceMismatch = errors.New("ledger sequence mismatch: concurrent modification detected")

// releaseLock releases the given lock and logs any errors that occur.
// This is used in defer statements where we cannot return the error.
// Lock release failures are serious infrastructure issues that indicate
// stale lock files may remain on disk.
func releaseLock(lock *LedgerLock, operation string) {
	if err := lock.Release(); err != nil {
		log.Printf("ERROR: failed to release lock during %s: %v (stale lock file may remain)", operation, err)
	}
}

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

// nofsyncEnv is a test-only switch for the benchmark job: when it is exactly
// "1", fsyncDir returns without syncing the directory. It exists so the
// benchmark can measure the per-event directory-fsync cost by running the same
// workload with and without it. It is UNSAFE for durability: a crash after an
// append may lose the directory entry even though the event file was written,
// so the ledger could come back short. It must never be set in production.
const nofsyncEnv = "AF_TEST_NO_FSYNC"

// fsyncDir flushes the directory entry created by a rename so that the new
// event file survives a crash. On platforms where directory fsync is not
// supported (e.g. some filesystems return EINVAL), the error is ignored: the
// rename itself is still atomic, and losing the explicit directory fsync only
// affects durability, not consistency.
func fsyncDir(dir string) error {
	if os.Getenv(nofsyncEnv) == "1" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("failed to open ledger directory for fsync: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		// Some platforms/filesystems do not support fsync on a directory
		// (EINVAL) or do not implement it (ENOTSUP). Treat those as best-effort
		// rather than failing the append; the rename itself is still atomic.
		if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) {
			return nil
		}
		return fmt.Errorf("failed to fsync ledger directory: %w", err)
	}
	return nil
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
	defer releaseLock(lock, "append")

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

	// Make the rename durable before reporting success.
	if err := fsyncDir(dir); err != nil {
		return 0, err
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
	defer releaseLock(lock, "append-if-sequence")

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

	// Make the rename durable before reporting success.
	if err := fsyncDir(dir); err != nil {
		return 0, err
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
	defer releaseLock(lock, "append-batch")

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

	// Rename all temp files to final paths, making each one durable before the
	// next. If any rename fails, rollback all previously renamed files (this
	// function keeps its historical all-or-nothing semantics) and fsync the
	// rollback so the durable set is never a torn mixture.
	finalPaths := make([]string, len(events))
	for i := range events {
		seq := seqs[i]
		finalPaths[i] = filepath.Join(dir, GenerateFilename(seq))
		if err := os.Rename(tempPaths[i], finalPaths[i]); err != nil {
			// Rollback: remove all previously renamed files
			for j := 0; j < i; j++ {
				_ = os.Remove(finalPaths[j])
			}
			// Cleanup remaining temp files
			cleanupTempFiles(tempPaths, i, len(events))
			_ = fsyncDir(dir)
			return nil, fmt.Errorf("failed to rename event %d: %w", i, err)
		}
		// Make each rename durable immediately so a crash after rename i leaves
		// exactly the contiguous prefix [1, i].
		if err := fsyncDir(dir); err != nil {
			for j := 0; j <= i; j++ {
				_ = os.Remove(finalPaths[j])
			}
			cleanupTempFiles(tempPaths, i+1, len(events))
			return nil, err
		}
	}

	return seqs, nil
}

// AppendBatchIfSequence appends a batch of events only if the ledger is still
// at expectedSeq (the sequence observed when state was loaded). Unlike
// AppendBatch, the sequence check covers the whole batch: it is taken under the
// same exclusive lock that serializes the writes, so a concurrent writer that
// landed between the caller's state read and this call causes the whole batch
// to be refused with ErrSequenceMismatch.
//
// On a mid-batch write failure the already-renamed prefix is left in place (a
// valid ledger prefix), remaining temp files are removed, and the error is
// returned. Replay of the ledger is always valid up to the last renamed event.
//
// Returns the sequence numbers assigned to each event, or ErrSequenceMismatch.
func AppendBatchIfSequence(dir string, events []Event, expectedSeq int) ([]int, error) {
	return AppendBatchIfSequenceWithTimeout(dir, events, expectedSeq, defaultLockTimeout)
}

// AppendBatchIfSequenceWithTimeout is like AppendBatchIfSequence but with a
// custom lock timeout.
func AppendBatchIfSequenceWithTimeout(dir string, events []Event, expectedSeq int, timeout time.Duration) ([]int, error) {
	if len(events) == 0 {
		return nil, nil
	}

	if err := validateDirectory(dir); err != nil {
		return nil, err
	}

	// Acquire lock for concurrent safety.
	lock := NewLedgerLock(dir)
	if err := lock.Acquire("append-batch-if-sequence-operation", timeout); err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer releaseLock(lock, "append-batch-if-sequence")

	// Get current sequence number (inside lock to ensure atomicity).
	currentSeq, err := NextSequence(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to determine current sequence: %w", err)
	}

	actualLatest := currentSeq - 1
	if actualLatest != expectedSeq {
		return nil, fmt.Errorf("%w: expected sequence %d, but ledger is at %d",
			ErrSequenceMismatch, expectedSeq, actualLatest)
	}

	seqs := make([]int, len(events))
	tempPaths := make([]string, len(events))

	// Write all temp files first.
	for i, event := range events {
		seq := currentSeq + i
		seqs[i] = seq

		data, err := json.Marshal(event)
		if err != nil {
			cleanupTempFiles(tempPaths, 0, i)
			return nil, fmt.Errorf("failed to marshal event %d: %w", i, err)
		}

		tempFile, err := os.CreateTemp(dir, ".event-*.tmp")
		if err != nil {
			cleanupTempFiles(tempPaths, 0, i)
			return nil, fmt.Errorf("failed to create temp file for event %d: %w", i, err)
		}
		tempPaths[i] = tempFile.Name()

		if _, err = tempFile.Write(data); err != nil {
			tempFile.Close()
			cleanupTempFiles(tempPaths, 0, i+1)
			return nil, fmt.Errorf("failed to write event %d: %w", i, err)
		}
		if err := tempFile.Sync(); err != nil {
			tempFile.Close()
			cleanupTempFiles(tempPaths, 0, i+1)
			return nil, fmt.Errorf("failed to sync event %d: %w", i, err)
		}
		if err := tempFile.Close(); err != nil {
			cleanupTempFiles(tempPaths, 0, i+1)
			return nil, fmt.Errorf("failed to close temp file for event %d: %w", i, err)
		}
		if err := os.Chmod(tempPaths[i], 0644); err != nil {
			cleanupTempFiles(tempPaths, 0, i+1)
			return nil, fmt.Errorf("failed to set permissions for event %d: %w", i, err)
		}
	}

	// Rename sequentially, making each rename durable before the next. On
	// failure, keep the valid prefix already renamed (crash/partial-write
	// semantics: a valid prefix, never a corrupt ledger), fsync it, and clean
	// up the remaining temp files. The test-only crash hook fires right after
	// the n-th rename to reproduce a mid-batch process death.
	for i := range events {
		finalPath := filepath.Join(dir, GenerateFilename(seqs[i]))
		if err := os.Rename(tempPaths[i], finalPath); err != nil {
			cleanupTempFiles(tempPaths, i, len(events))
			_ = fsyncDir(dir)
			return seqs[:i], fmt.Errorf("failed to rename event %d: %w", i, err)
		}
		maybeCrashAfterRename(i + 1)
		if err := fsyncDir(dir); err != nil {
			cleanupTempFiles(tempPaths, i+1, len(events))
			return seqs[:i+1], err
		}
	}

	return seqs, nil
}
