// Package ledger provides event-sourced ledger operations for the AF proof framework.
package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ScanFunc is a callback function called for each event during scanning.
// Return an error to stop scanning. The error will be returned by Scan.
// Return nil to continue scanning.
type ScanFunc func(seq int, data []byte) error

// ErrStopScan is a sentinel error that can be returned by ScanFunc
// to stop scanning without indicating an actual error.
var ErrStopScan = fmt.Errorf("stop scan")

// ReadEvent reads a single event file from the ledger directory.
// Returns the raw JSON bytes of the event.
func ReadEvent(dir string, seq int) ([]byte, error) {
	if err := validateDirectory(dir); err != nil {
		return nil, err
	}

	if seq <= 0 {
		return nil, fmt.Errorf("invalid sequence number: %d", seq)
	}

	path := EventFilePath(dir, seq)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("event %d not found", seq)
		}
		return nil, fmt.Errorf("failed to read event file: %w", err)
	}

	// Validate that it's valid JSON
	if !json.Valid(data) {
		return nil, fmt.Errorf("event %d contains invalid JSON", seq)
	}

	return data, nil
}

// ReadAll reads all events from the ledger directory in sequence order.
// Returns a slice of raw JSON bytes for each event.
// Returns an empty slice if the directory is empty (no event files).
func ReadAll(dir string) ([][]byte, error) {
	seqs, err := listEventSequences(dir)
	if err != nil {
		return nil, err
	}

	if len(seqs) == 0 {
		return nil, nil
	}

	result := make([][]byte, 0, len(seqs))
	for _, seq := range seqs {
		data, err := ReadEvent(dir, seq)
		if err != nil {
			return nil, fmt.Errorf("failed to read event %d: %w", seq, err)
		}
		result = append(result, data)
	}

	return result, nil
}

// Scan iterates over all events in sequence order, calling fn for each.
// Scanning stops if fn returns an error.
// If fn returns ErrStopScan, Scan returns nil (clean stop).
// Any other error from fn is returned by Scan.
func Scan(dir string, fn ScanFunc) error {
	seqs, err := listEventSequences(dir)
	if err != nil {
		return err
	}

	for _, seq := range seqs {
		data, err := ReadEvent(dir, seq)
		if err != nil {
			return fmt.Errorf("failed to read event %d: %w", seq, err)
		}

		if err := fn(seq, data); err != nil {
			if err == ErrStopScan {
				return nil
			}
			return err
		}
	}

	return nil
}

// Count returns the number of events in the ledger directory.
func Count(dir string) (int, error) {
	seqs, err := listEventSequences(dir)
	if err != nil {
		return 0, err
	}
	return len(seqs), nil
}

// HasGaps checks if there are any gaps in the event sequence.
// Returns true if there are gaps (e.g., 1, 2, 4 with 3 missing).
// Returns false if the sequence is contiguous or empty.
func HasGaps(dir string) (bool, error) {
	seqs, err := listEventSequences(dir)
	if err != nil {
		return false, err
	}

	if len(seqs) == 0 {
		return false, nil
	}

	// Check that sequence numbers are contiguous starting from 1
	for i, seq := range seqs {
		expected := i + 1
		if seq != expected {
			return true, nil
		}
	}

	return false, nil
}

// listEventSequences returns all valid event sequence numbers in sorted order.
func listEventSequences(dir string) ([]int, error) {
	if err := validateDirectory(dir); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var seqs []int
	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		// Try to parse the filename
		seq, err := ParseFilename(entry.Name())
		if err != nil {
			// Not a valid event file, skip it
			continue
		}

		seqs = append(seqs, seq)
	}

	// Sort by sequence number
	sort.Ints(seqs)

	return seqs, nil
}

// ReadEventTyped reads a single event and unmarshals it into the provided destination.
// The destination should be a pointer to the expected event type.
func ReadEventTyped(dir string, seq int, dest interface{}) error {
	data, err := ReadEvent(dir, seq)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to unmarshal event %d: %w", seq, err)
	}

	return nil
}

// EventFile represents an event file with its sequence number and path.
type EventFile struct {
	Seq  int
	Path string
}

// ListEventFiles returns information about all event files in the ledger.
func ListEventFiles(dir string) ([]EventFile, error) {
	seqs, err := listEventSequences(dir)
	if err != nil {
		return nil, err
	}

	files := make([]EventFile, len(seqs))
	for i, seq := range seqs {
		files[i] = EventFile{
			Seq:  seq,
			Path: filepath.Join(dir, GenerateFilename(seq)),
		}
	}

	return files, nil
}
