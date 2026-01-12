//go:build integration

package ledger

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestReadAll_EmptyDirectory verifies reading an empty ledger returns empty slice.
func TestReadAll_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	events, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if events != nil && len(events) != 0 {
		t.Errorf("ReadAll returned %d events, want 0", len(events))
	}
}

// TestReadAll_SingleEvent verifies reading a ledger with one event.
func TestReadAll_SingleEvent(t *testing.T) {
	dir := t.TempDir()

	// Create a single event file
	event := NewProofInitialized("Test conjecture", "agent-001")
	_, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	events, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("ReadAll returned %d events, want 1", len(events))
	}

	// Verify the content
	var decoded ProofInitialized
	if err := json.Unmarshal(events[0], &decoded); err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	if decoded.Type() != EventProofInitialized {
		t.Errorf("Event type = %q, want %q", decoded.Type(), EventProofInitialized)
	}
	if decoded.Conjecture != "Test conjecture" {
		t.Errorf("Conjecture = %q, want %q", decoded.Conjecture, "Test conjecture")
	}
}

// TestReadAll_MultipleEventsInSequence verifies reading events in correct order.
func TestReadAll_MultipleEventsInSequence(t *testing.T) {
	dir := t.TempDir()

	// Create multiple events
	events := []Event{
		NewProofInitialized("First", "agent-001"),
		NewChallengeResolved("chal-001"),
		NewChallengeWithdrawn("chal-002"),
	}

	for _, event := range events {
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	readEvents, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(readEvents) != 3 {
		t.Fatalf("ReadAll returned %d events, want 3", len(readEvents))
	}

	// Verify order by checking event types
	expectedTypes := []EventType{EventProofInitialized, EventChallengeResolved, EventChallengeWithdrawn}
	for i, data := range readEvents {
		var base BaseEvent
		if err := json.Unmarshal(data, &base); err != nil {
			t.Fatalf("Failed to unmarshal event %d: %v", i, err)
		}

		if base.Type() != expectedTypes[i] {
			t.Errorf("Event %d type = %q, want %q", i, base.Type(), expectedTypes[i])
		}
	}
}

// TestReadAll_SkipsNonEventFiles verifies that non-event files are ignored.
func TestReadAll_SkipsNonEventFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a valid event file
	event := NewProofInitialized("Valid event", "agent-001")
	_, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Create some non-event files
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not an event"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden file"), 0644)
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("config: value"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	events, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("ReadAll returned %d events, want 1 (should skip non-event files)", len(events))
	}
}

// TestReadAll_InvalidDirectory verifies error handling for invalid directories.
func TestReadAll_InvalidDirectory(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{"non-existent directory", "/nonexistent/path/that/does/not/exist"},
		{"empty directory path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadAll(tt.dir)
			if err == nil {
				t.Error("ReadAll should return error for invalid directory")
			}
		})
	}
}

// TestReadAll_CorruptedEventFile verifies error handling for corrupted JSON.
func TestReadAll_CorruptedEventFile(t *testing.T) {
	dir := t.TempDir()

	// Create a valid event first
	event := NewProofInitialized("Valid event", "agent-001")
	_, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Create a corrupted event file
	corruptedPath := filepath.Join(dir, "000002.json")
	os.WriteFile(corruptedPath, []byte("not valid json {{{"), 0644)

	_, err = ReadAll(dir)
	if err == nil {
		t.Error("ReadAll should return error for corrupted event file")
	}
}

// TestReadEvent_Success verifies reading a single event by sequence number.
func TestReadEvent_Success(t *testing.T) {
	dir := t.TempDir()

	event := NewProofInitialized("Single event", "agent-001")
	seq, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	data, err := ReadEvent(dir, seq)
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}

	// Verify content
	var decoded ProofInitialized
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Conjecture != "Single event" {
		t.Errorf("Conjecture = %q, want %q", decoded.Conjecture, "Single event")
	}
}

// TestReadEvent_NotFound verifies error when event file doesn't exist.
func TestReadEvent_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := ReadEvent(dir, 99)
	if err == nil {
		t.Error("ReadEvent should return error for non-existent event")
	}
}

// TestReadEvent_InvalidSequence verifies error for invalid sequence numbers.
func TestReadEvent_InvalidSequence(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name string
		seq  int
	}{
		{"zero", 0},
		{"negative", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadEvent(dir, tt.seq)
			if err == nil {
				t.Errorf("ReadEvent should return error for sequence %d", tt.seq)
			}
		})
	}
}

// TestReadEvent_InvalidJSON verifies error for corrupted JSON content.
func TestReadEvent_InvalidJSON(t *testing.T) {
	dir := t.TempDir()

	// Create a file with invalid JSON
	invalidPath := filepath.Join(dir, "000001.json")
	os.WriteFile(invalidPath, []byte("this is not json"), 0644)

	_, err := ReadEvent(dir, 1)
	if err == nil {
		t.Error("ReadEvent should return error for invalid JSON")
	}
}

// TestScan_EmptyDirectory verifies scanning an empty directory.
func TestScan_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	callCount := 0
	err := Scan(dir, func(seq int, data []byte) error {
		callCount++
		return nil
	})

	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if callCount != 0 {
		t.Errorf("Callback called %d times, want 0", callCount)
	}
}

// TestScan_CallsForEachEvent verifies callback is called for each event in order.
func TestScan_CallsForEachEvent(t *testing.T) {
	dir := t.TempDir()

	// Create 5 events
	for i := 0; i < 5; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	var seqsScanned []int
	err := Scan(dir, func(seq int, data []byte) error {
		seqsScanned = append(seqsScanned, seq)
		return nil
	})

	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(seqsScanned) != 5 {
		t.Fatalf("Callback called %d times, want 5", len(seqsScanned))
	}

	// Verify order
	for i, seq := range seqsScanned {
		expected := i + 1
		if seq != expected {
			t.Errorf("Sequence at position %d = %d, want %d", i, seq, expected)
		}
	}
}

// TestScan_StopsOnError verifies scanning stops when callback returns error.
func TestScan_StopsOnError(t *testing.T) {
	dir := t.TempDir()

	// Create 5 events
	for i := 0; i < 5; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	callCount := 0
	testError := errors.New("test error")

	err := Scan(dir, func(seq int, data []byte) error {
		callCount++
		if callCount == 3 {
			return testError
		}
		return nil
	})

	if err != testError {
		t.Errorf("Scan returned error %v, want %v", err, testError)
	}

	if callCount != 3 {
		t.Errorf("Callback called %d times, want 3 (should stop on error)", callCount)
	}
}

// TestScan_ErrStopScan verifies clean stop with ErrStopScan sentinel.
func TestScan_ErrStopScan(t *testing.T) {
	dir := t.TempDir()

	// Create 5 events
	for i := 0; i < 5; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	callCount := 0
	err := Scan(dir, func(seq int, data []byte) error {
		callCount++
		if callCount == 2 {
			return ErrStopScan
		}
		return nil
	})

	// ErrStopScan should result in nil error from Scan
	if err != nil {
		t.Errorf("Scan returned error %v, want nil (ErrStopScan should be clean stop)", err)
	}

	if callCount != 2 {
		t.Errorf("Callback called %d times, want 2", callCount)
	}
}

// TestScan_ReceivesValidJSON verifies callback receives valid JSON data.
func TestScan_ReceivesValidJSON(t *testing.T) {
	dir := t.TempDir()

	event := NewProofInitialized("Scan test", "agent-scan")
	_, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	err = Scan(dir, func(seq int, data []byte) error {
		if !json.Valid(data) {
			t.Error("Callback received invalid JSON")
		}

		var decoded ProofInitialized
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Failed to unmarshal: %v", err)
		}

		if decoded.Conjecture != "Scan test" {
			t.Errorf("Conjecture = %q, want %q", decoded.Conjecture, "Scan test")
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
}

// TestCount_EmptyDirectory verifies count of empty ledger.
func TestCount_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	count, err := Count(dir)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Count = %d, want 0", count)
	}
}

// TestCount_WithEvents verifies count with multiple events.
func TestCount_WithEvents(t *testing.T) {
	dir := t.TempDir()

	// Create 7 events
	for i := 0; i < 7; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	count, err := Count(dir)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 7 {
		t.Errorf("Count = %d, want 7", count)
	}
}

// TestCount_IgnoresNonEventFiles verifies non-event files aren't counted.
func TestCount_IgnoresNonEventFiles(t *testing.T) {
	dir := t.TempDir()

	// Create 2 valid events
	for i := 0; i < 2; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Create non-event files
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(dir, "data.xml"), []byte("<xml/>"), 0644)

	count, err := Count(dir)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Count = %d, want 2 (should ignore non-event files)", count)
	}
}

// TestHasGaps_EmptyDirectory verifies no gaps in empty ledger.
func TestHasGaps_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	hasGaps, err := HasGaps(dir)
	if err != nil {
		t.Fatalf("HasGaps failed: %v", err)
	}

	if hasGaps {
		t.Error("HasGaps = true, want false for empty directory")
	}
}

// TestHasGaps_ContiguousSequence verifies no gaps with contiguous sequence.
func TestHasGaps_ContiguousSequence(t *testing.T) {
	dir := t.TempDir()

	// Create 5 contiguous events
	for i := 0; i < 5; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	hasGaps, err := HasGaps(dir)
	if err != nil {
		t.Fatalf("HasGaps failed: %v", err)
	}

	if hasGaps {
		t.Error("HasGaps = true, want false for contiguous sequence")
	}
}

// TestHasGaps_WithGap verifies detection of gaps in sequence.
func TestHasGaps_WithGap(t *testing.T) {
	dir := t.TempDir()

	// Create events with gap (1, 2, 4, 5 - missing 3)
	for _, seq := range []int{1, 2, 4, 5} {
		path := filepath.Join(dir, GenerateFilename(seq))
		content := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z","conjecture":"test","author":"agent"}`
		os.WriteFile(path, []byte(content), 0644)
	}

	hasGaps, err := HasGaps(dir)
	if err != nil {
		t.Fatalf("HasGaps failed: %v", err)
	}

	if !hasGaps {
		t.Error("HasGaps = false, want true for sequence with gap")
	}
}

// TestHasGaps_NotStartingFromOne verifies gap detection when not starting from 1.
func TestHasGaps_NotStartingFromOne(t *testing.T) {
	dir := t.TempDir()

	// Create events starting from 3 (missing 1, 2)
	for _, seq := range []int{3, 4, 5} {
		path := filepath.Join(dir, GenerateFilename(seq))
		content := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z","conjecture":"test","author":"agent"}`
		os.WriteFile(path, []byte(content), 0644)
	}

	hasGaps, err := HasGaps(dir)
	if err != nil {
		t.Fatalf("HasGaps failed: %v", err)
	}

	if !hasGaps {
		t.Error("HasGaps = false, want true when sequence doesn't start from 1")
	}
}

// TestReadEventTyped_Success verifies typed event reading.
func TestReadEventTyped_Success(t *testing.T) {
	dir := t.TempDir()

	event := NewProofInitialized("Typed read test", "agent-typed")
	seq, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	var decoded ProofInitialized
	err = ReadEventTyped(dir, seq, &decoded)
	if err != nil {
		t.Fatalf("ReadEventTyped failed: %v", err)
	}

	if decoded.Type() != EventProofInitialized {
		t.Errorf("Type = %q, want %q", decoded.Type(), EventProofInitialized)
	}
	if decoded.Conjecture != "Typed read test" {
		t.Errorf("Conjecture = %q, want %q", decoded.Conjecture, "Typed read test")
	}
	if decoded.Author != "agent-typed" {
		t.Errorf("Author = %q, want %q", decoded.Author, "agent-typed")
	}
}

// TestReadEventTyped_NotFound verifies error for non-existent event.
func TestReadEventTyped_NotFound(t *testing.T) {
	dir := t.TempDir()

	var decoded ProofInitialized
	err := ReadEventTyped(dir, 99, &decoded)
	if err == nil {
		t.Error("ReadEventTyped should return error for non-existent event")
	}
}

// TestReadEventTyped_WrongType verifies behavior with wrong destination type.
func TestReadEventTyped_WrongType(t *testing.T) {
	dir := t.TempDir()

	// Create a ProofInitialized event
	event := NewProofInitialized("Wrong type test", "agent")
	seq, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Try to read into a different type - this will partially succeed
	// because JSON unmarshaling is lenient
	var decoded ChallengeResolved
	err = ReadEventTyped(dir, seq, &decoded)
	if err != nil {
		t.Fatalf("ReadEventTyped failed: %v", err)
	}

	// The base fields should be populated
	if decoded.Type() != EventProofInitialized {
		t.Errorf("Type = %q, want %q (base fields should be populated)", decoded.Type(), EventProofInitialized)
	}
}

// TestListEventFiles_EmptyDirectory verifies listing empty directory.
func TestListEventFiles_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	files, err := ListEventFiles(dir)
	if err != nil {
		t.Fatalf("ListEventFiles failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("ListEventFiles returned %d files, want 0", len(files))
	}
}

// TestListEventFiles_WithEvents verifies listing events with correct paths.
func TestListEventFiles_WithEvents(t *testing.T) {
	dir := t.TempDir()

	// Create 3 events
	for i := 0; i < 3; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	files, err := ListEventFiles(dir)
	if err != nil {
		t.Fatalf("ListEventFiles failed: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("ListEventFiles returned %d files, want 3", len(files))
	}

	// Verify sequence numbers and paths
	for i, f := range files {
		expectedSeq := i + 1
		if f.Seq != expectedSeq {
			t.Errorf("files[%d].Seq = %d, want %d", i, f.Seq, expectedSeq)
		}

		expectedPath := filepath.Join(dir, GenerateFilename(expectedSeq))
		if f.Path != expectedPath {
			t.Errorf("files[%d].Path = %q, want %q", i, f.Path, expectedPath)
		}

		// Verify file exists
		if _, err := os.Stat(f.Path); os.IsNotExist(err) {
			t.Errorf("File at %s does not exist", f.Path)
		}
	}
}

// TestListEventFiles_IgnoresNonEventFiles verifies non-event files are ignored.
func TestListEventFiles_IgnoresNonEventFiles(t *testing.T) {
	dir := t.TempDir()

	// Create 1 valid event
	event := NewProofInitialized("Event", "agent")
	_, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Create non-event files
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("notes"), 0644)
	os.WriteFile(filepath.Join(dir, ".lock"), []byte("lock"), 0644)

	files, err := ListEventFiles(dir)
	if err != nil {
		t.Fatalf("ListEventFiles failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("ListEventFiles returned %d files, want 1 (should ignore non-event files)", len(files))
	}
}

// TestReadAll_LargeEventCount verifies handling of many events.
func TestReadAll_LargeEventCount(t *testing.T) {
	dir := t.TempDir()

	// Create 100 events
	const eventCount = 100
	for i := 0; i < eventCount; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append %d failed: %v", i, err)
		}
	}

	events, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(events) != eventCount {
		t.Errorf("ReadAll returned %d events, want %d", len(events), eventCount)
	}
}

// TestRead_TableDrivenEventTypes tests reading various event types.
func TestRead_TableDrivenEventTypes(t *testing.T) {
	tests := []struct {
		name      string
		event     Event
		eventType EventType
	}{
		{"ProofInitialized", NewProofInitialized("test", "agent"), EventProofInitialized},
		{"ChallengeResolved", NewChallengeResolved("chal-1"), EventChallengeResolved},
		{"ChallengeWithdrawn", NewChallengeWithdrawn("chal-2"), EventChallengeWithdrawn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			seq, err := Append(dir, tt.event)
			if err != nil {
				t.Fatalf("Append failed: %v", err)
			}

			data, err := ReadEvent(dir, seq)
			if err != nil {
				t.Fatalf("ReadEvent failed: %v", err)
			}

			var base BaseEvent
			if err := json.Unmarshal(data, &base); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if base.Type() != tt.eventType {
				t.Errorf("Event type = %q, want %q", base.Type(), tt.eventType)
			}
		})
	}
}

// TestScan_InvalidDirectory verifies error for invalid directory.
func TestScan_InvalidDirectory(t *testing.T) {
	err := Scan("/nonexistent/path", func(seq int, data []byte) error {
		return nil
	})

	if err == nil {
		t.Error("Scan should return error for invalid directory")
	}
}

// TestReadAll_PreservesEventOrder verifies events are returned in sequence order.
func TestReadAll_PreservesEventOrder(t *testing.T) {
	dir := t.TempDir()

	// Create events with identifiable conjectures
	conjectures := []string{"First", "Second", "Third", "Fourth", "Fifth"}
	for _, conj := range conjectures {
		event := NewProofInitialized(conj, "agent")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	events, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	// Verify order matches
	for i, data := range events {
		var decoded ProofInitialized
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Failed to unmarshal event %d: %v", i, err)
		}

		if decoded.Conjecture != conjectures[i] {
			t.Errorf("Event %d conjecture = %q, want %q", i, decoded.Conjecture, conjectures[i])
		}
	}
}

// TestReadEvent_FileIsNotDirectory verifies handling when path is not a directory.
func TestReadEvent_FileIsNotDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create a file instead of directory
	filePath := filepath.Join(dir, "not_a_dir")
	os.WriteFile(filePath, []byte("content"), 0644)

	_, err := ReadEvent(filePath, 1)
	if err == nil {
		t.Error("ReadEvent should return error when dir is actually a file")
	}
}

// TestCount_InvalidDirectory verifies error for invalid directory.
func TestCount_InvalidDirectory(t *testing.T) {
	_, err := Count("/nonexistent/path")
	if err == nil {
		t.Error("Count should return error for invalid directory")
	}
}

// TestHasGaps_InvalidDirectory verifies error for invalid directory.
func TestHasGaps_InvalidDirectory(t *testing.T) {
	_, err := HasGaps("/nonexistent/path")
	if err == nil {
		t.Error("HasGaps should return error for invalid directory")
	}
}

// TestListEventFiles_InvalidDirectory verifies error for invalid directory.
func TestListEventFiles_InvalidDirectory(t *testing.T) {
	_, err := ListEventFiles("/nonexistent/path")
	if err == nil {
		t.Error("ListEventFiles should return error for invalid directory")
	}
}

// TestScan_DataMatchesReadEvent verifies Scan provides same data as ReadEvent.
func TestScan_DataMatchesReadEvent(t *testing.T) {
	dir := t.TempDir()

	// Create some events
	for i := 0; i < 3; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Collect data from Scan
	scannedData := make(map[int][]byte)
	err := Scan(dir, func(seq int, data []byte) error {
		scannedData[seq] = data
		return nil
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Compare with ReadEvent
	for seq, scanData := range scannedData {
		readData, err := ReadEvent(dir, seq)
		if err != nil {
			t.Fatalf("ReadEvent failed for seq %d: %v", seq, err)
		}

		if string(scanData) != string(readData) {
			t.Errorf("Data mismatch for seq %d:\nScan: %s\nRead: %s", seq, scanData, readData)
		}
	}
}
