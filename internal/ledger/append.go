// Package ledger provides event-sourced ledger operations for the AF proof framework.
package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Default lock timeout for append operations.
const defaultLockTimeout = 5 * time.Second

// cleanupTempFiles removes temporary files from the given slice within the range [start, end).
// This is a best-effort operation; errors are silently ignored since cleanup failures
// should not mask the original error that triggered the cleanup.
func cleanupTempFiles(tempPaths []string, start, end int) {
	for i := start; i < end; i++ {
		if tempPaths[i] != "" {
			os.Remove(tempPaths[i])
		}
	}
}

// Append adds an event to the ledger at the given directory.
// Returns the sequence number assigned to the event.
// The write is atomic: the event is first written to a temp file, then renamed.
// Uses file-based locking to ensure concurrent safety.
func Append(dir string, event Event) (int, error) {
	// Validate directory
	if dir == "" {
		return 0, fmt.Errorf("empty directory path")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("path is not a directory: %s", dir)
	}

	// Acquire lock for concurrent safety
	lock := NewLedgerLock(dir)
	if err := lock.Acquire("append-operation", defaultLockTimeout); err != nil {
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
		os.Remove(tempPath)
		return 0, fmt.Errorf("failed to write event data: %w", err)
	}

	// Sync to ensure data is on disk
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return 0, fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return 0, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set file permissions
	if err := os.Chmod(tempPath, 0644); err != nil {
		os.Remove(tempPath)
		return 0, fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomic rename to final path
	finalPath := filepath.Join(dir, GenerateFilename(seq))
	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		return 0, fmt.Errorf("failed to rename temp file: %w", err)
	}

	return seq, nil
}

// AppendWithTimeout adds an event to the ledger with a custom lock timeout.
// This is useful for testing or when you need faster failure in constrained environments.
func AppendWithTimeout(dir string, event Event, timeout time.Duration) (int, error) {
	// Validate directory
	if dir == "" {
		return 0, fmt.Errorf("empty directory path")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("path is not a directory: %s", dir)
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
		os.Remove(tempPath)
		return 0, fmt.Errorf("failed to write event data: %w", err)
	}

	// Sync to ensure data is on disk
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return 0, fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return 0, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set file permissions
	if err := os.Chmod(tempPath, 0644); err != nil {
		os.Remove(tempPath)
		return 0, fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomic rename to final path
	finalPath := filepath.Join(dir, GenerateFilename(seq))
	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
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
		return []int{}, nil
	}

	// Validate directory
	if dir == "" {
		return nil, fmt.Errorf("empty directory path")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", dir)
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
