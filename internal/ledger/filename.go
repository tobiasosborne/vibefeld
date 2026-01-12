// Package ledger provides event-sourced ledger operations for the AF proof framework.
package ledger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GenerateFilename creates a filename for an event with the given sequence number.
// Format: {seq:06d}.json (e.g., 000001.json, 000042.json)
// The sequence number is zero-padded to at least 6 digits.
func GenerateFilename(seq int) string {
	return fmt.Sprintf("%06d.json", seq)
}

// ParseFilename extracts the sequence number from an event filename.
// Returns an error if the filename is not in the expected format.
func ParseFilename(name string) (int, error) {
	if name == "" {
		return 0, errors.New("empty filename")
	}

	if !strings.HasSuffix(name, ".json") {
		return 0, errors.New("filename must have .json extension")
	}

	// Remove the .json extension
	base := strings.TrimSuffix(name, ".json")
	if base == "" {
		return 0, errors.New("filename has no numeric portion")
	}

	// Parse as integer - this will fail for non-numeric strings
	seq, err := strconv.Atoi(base)
	if err != nil {
		return 0, fmt.Errorf("invalid sequence number: %w", err)
	}

	// Reject negative numbers (strconv.Atoi accepts "-1")
	if seq < 0 {
		return 0, errors.New("sequence number cannot be negative")
	}

	return seq, nil
}

// NextSequence returns the next sequence number for event files in the given directory.
// If the directory is empty (no valid event files), it returns 1.
// Otherwise, it returns max(existing sequence numbers) + 1.
func NextSequence(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to read directory: %w", err)
	}

	maxSeq := 0
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

		if seq > maxSeq {
			maxSeq = seq
		}
	}

	return maxSeq + 1, nil
}

// IsEventFile returns true if the given filename looks like a valid event file.
func IsEventFile(name string) bool {
	_, err := ParseFilename(name)
	return err == nil
}

// EventFilePath returns the full path to an event file in the given directory.
func EventFilePath(dir string, seq int) string {
	return filepath.Join(dir, GenerateFilename(seq))
}
