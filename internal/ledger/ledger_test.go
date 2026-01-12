//go:build integration

package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestNewLedger_NewDirectory verifies creating a ledger for a new directory.
func TestNewLedger_NewDirectory(t *testing.T) {
	dir := t.TempDir()

	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	if ledger == nil {
		t.Fatal("NewLedger returned nil")
	}

	if ledger.Dir() != dir {
		t.Errorf("Ledger.Dir() = %q, want %q", ledger.Dir(), dir)
	}
}

// TestNewLedger_ExistingDirectory verifies creating a ledger for an existing directory with events.
func TestNewLedger_ExistingDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create some existing events manually
	for i := 1; i <= 3; i++ {
		path := filepath.Join(dir, GenerateFilename(i))
		content := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z","conjecture":"test","author":"agent"}`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create existing event file: %v", err)
		}
	}

	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Verify existing events are accessible
	count, err := ledger.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Count = %d, want 3", count)
	}
}

// TestNewLedger_InvalidDirectory verifies error handling for invalid directories.
func TestNewLedger_InvalidDirectory(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{"non-existent directory", "/nonexistent/path/that/does/not/exist"},
		{"empty directory path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLedger(tt.dir)
			if err == nil {
				t.Error("NewLedger should return error for invalid directory")
			}
		})
	}
}

// TestNewLedger_FileNotDirectory verifies error when path is a file.
func TestNewLedger_FileNotDirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not_a_directory")

	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	_, err := NewLedger(filePath)
	if err == nil {
		t.Error("NewLedger should return error when path is a file, not a directory")
	}
}

// TestLedger_Append_SingleEvent verifies appending a single event.
func TestLedger_Append_SingleEvent(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	event := NewProofInitialized("Test conjecture", "agent-001")
	seq, err := ledger.Append(event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	if seq != 1 {
		t.Errorf("Append returned seq = %d, want 1", seq)
	}

	// Verify file exists
	filePath := filepath.Join(dir, GenerateFilename(1))
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Event file %s was not created", filePath)
	}
}

// TestLedger_Append_MultipleEvents verifies appending multiple events increments sequence.
func TestLedger_Append_MultipleEvents(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	events := []Event{
		NewProofInitialized("First", "agent-001"),
		NewChallengeResolved("chal-001"),
		NewChallengeWithdrawn("chal-002"),
	}

	for i, event := range events {
		seq, err := ledger.Append(event)
		if err != nil {
			t.Fatalf("Append %d failed: %v", i, err)
		}
		expectedSeq := i + 1
		if seq != expectedSeq {
			t.Errorf("Append %d returned seq = %d, want %d", i, seq, expectedSeq)
		}
	}

	// Verify all files exist
	for i := 1; i <= 3; i++ {
		filePath := filepath.Join(dir, GenerateFilename(i))
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Event file %s was not created", filePath)
		}
	}
}

// TestLedger_Append_ContinuesFromExisting verifies append continues from existing sequence.
func TestLedger_Append_ContinuesFromExisting(t *testing.T) {
	dir := t.TempDir()

	// Create existing events
	for i := 1; i <= 5; i++ {
		path := filepath.Join(dir, GenerateFilename(i))
		content := `{"type":"proof_initialized","timestamp":"2025-01-01T00:00:00Z"}`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create existing event file: %v", err)
		}
	}

	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	event := NewProofInitialized("New event", "agent")
	seq, err := ledger.Append(event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	if seq != 6 {
		t.Errorf("Append returned seq = %d, want 6", seq)
	}
}

// TestLedger_ReadAll_EmptyLedger verifies reading an empty ledger.
func TestLedger_ReadAll_EmptyLedger(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	events, err := ledger.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if events != nil && len(events) != 0 {
		t.Errorf("ReadAll returned %d events, want 0", len(events))
	}
}

// TestLedger_ReadAll_WithEvents verifies reading all events.
func TestLedger_ReadAll_WithEvents(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Append some events
	testEvents := []Event{
		NewProofInitialized("First", "agent"),
		NewChallengeResolved("chal-1"),
		NewChallengeWithdrawn("chal-2"),
	}

	for _, event := range testEvents {
		_, err := ledger.Append(event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	events, err := ledger.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("ReadAll returned %d events, want 3", len(events))
	}

	// Verify order by checking event types
	expectedTypes := []EventType{EventProofInitialized, EventChallengeResolved, EventChallengeWithdrawn}
	for i, data := range events {
		var base BaseEvent
		if err := json.Unmarshal(data, &base); err != nil {
			t.Fatalf("Failed to unmarshal event %d: %v", i, err)
		}
		if base.Type() != expectedTypes[i] {
			t.Errorf("Event %d type = %q, want %q", i, base.Type(), expectedTypes[i])
		}
	}
}

// TestLedger_Scan_EmptyLedger verifies scanning an empty ledger.
func TestLedger_Scan_EmptyLedger(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	callCount := 0
	err = ledger.Scan(func(seq int, data []byte) error {
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

// TestLedger_Scan_WithEvents verifies scanning calls callback for each event.
func TestLedger_Scan_WithEvents(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Append events
	for i := 0; i < 5; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := ledger.Append(event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	var seqsScanned []int
	err = ledger.Scan(func(seq int, data []byte) error {
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

// TestLedger_Scan_StopsOnErrStopScan verifies clean stop with ErrStopScan.
func TestLedger_Scan_StopsOnErrStopScan(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Append events
	for i := 0; i < 5; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := ledger.Append(event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	callCount := 0
	err = ledger.Scan(func(seq int, data []byte) error {
		callCount++
		if callCount == 2 {
			return ErrStopScan
		}
		return nil
	})

	if err != nil {
		t.Errorf("Scan returned error %v, want nil (ErrStopScan should be clean stop)", err)
	}

	if callCount != 2 {
		t.Errorf("Callback called %d times, want 2", callCount)
	}
}

// TestLedger_Count_EmptyLedger verifies count of empty ledger.
func TestLedger_Count_EmptyLedger(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	count, err := ledger.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Count = %d, want 0", count)
	}
}

// TestLedger_Count_WithEvents verifies count with multiple events.
func TestLedger_Count_WithEvents(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Append 7 events
	for i := 0; i < 7; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := ledger.Append(event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	count, err := ledger.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 7 {
		t.Errorf("Count = %d, want 7", count)
	}
}

// TestLedger_AppendAndReadRoundTrip verifies appending and reading events.
func TestLedger_AppendAndReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Append a specific event
	event := NewProofInitialized("Round trip test", "agent-roundtrip")
	seq, err := ledger.Append(event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Read back
	events, err := ledger.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("ReadAll returned %d events, want 1", len(events))
	}

	// Verify content
	var decoded ProofInitialized
	if err := json.Unmarshal(events[0], &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Conjecture != "Round trip test" {
		t.Errorf("Conjecture = %q, want %q", decoded.Conjecture, "Round trip test")
	}
	if decoded.Author != "agent-roundtrip" {
		t.Errorf("Author = %q, want %q", decoded.Author, "agent-roundtrip")
	}
	_ = seq // used for verification
}

// TestLedger_ConcurrentAppends verifies concurrent appends don't corrupt ledger.
func TestLedger_ConcurrentAppends(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan int, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := NewProofInitialized("Concurrent test", "agent-concurrent")
			seq, err := ledger.Append(event)
			if err != nil {
				errors <- err
				return
			}
			results <- seq
		}(i)
	}

	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent append error: %v", err)
	}

	// Collect all sequence numbers
	seqSet := make(map[int]bool)
	for seq := range results {
		if seqSet[seq] {
			t.Errorf("Duplicate sequence number: %d", seq)
		}
		seqSet[seq] = true
	}

	// Verify all sequence numbers are consecutive
	if len(seqSet) != numGoroutines {
		t.Errorf("Expected %d unique sequence numbers, got %d", numGoroutines, len(seqSet))
	}

	for i := 1; i <= numGoroutines; i++ {
		if !seqSet[i] {
			t.Errorf("Missing sequence number: %d", i)
		}
	}

	// Verify count matches
	count, err := ledger.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != numGoroutines {
		t.Errorf("Count = %d, want %d", count, numGoroutines)
	}
}

// TestLedger_ScanDataMatchesReadAll verifies Scan provides same data as ReadAll.
func TestLedger_ScanDataMatchesReadAll(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Append some events
	for i := 0; i < 3; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := ledger.Append(event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Collect data from Scan
	scannedData := make(map[int][]byte)
	err = ledger.Scan(func(seq int, data []byte) error {
		scannedData[seq] = data
		return nil
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Get data from ReadAll
	readData, err := ledger.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	// Compare
	if len(scannedData) != len(readData) {
		t.Fatalf("Scan returned %d events, ReadAll returned %d", len(scannedData), len(readData))
	}

	for i, data := range readData {
		seq := i + 1
		if string(scannedData[seq]) != string(data) {
			t.Errorf("Data mismatch for seq %d", seq)
		}
	}
}

// TestLedger_MultipleLedgersOnSameDir verifies multiple Ledger instances on same dir work.
func TestLedger_MultipleLedgersOnSameDir(t *testing.T) {
	dir := t.TempDir()

	ledger1, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger 1 failed: %v", err)
	}

	ledger2, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger 2 failed: %v", err)
	}

	// Append via ledger1
	event1 := NewProofInitialized("From ledger1", "agent-1")
	seq1, err := ledger1.Append(event1)
	if err != nil {
		t.Fatalf("Append via ledger1 failed: %v", err)
	}
	if seq1 != 1 {
		t.Errorf("seq1 = %d, want 1", seq1)
	}

	// Append via ledger2
	event2 := NewProofInitialized("From ledger2", "agent-2")
	seq2, err := ledger2.Append(event2)
	if err != nil {
		t.Fatalf("Append via ledger2 failed: %v", err)
	}
	if seq2 != 2 {
		t.Errorf("seq2 = %d, want 2", seq2)
	}

	// Both ledgers should see 2 events
	count1, err := ledger1.Count()
	if err != nil {
		t.Fatalf("Count via ledger1 failed: %v", err)
	}
	if count1 != 2 {
		t.Errorf("count1 = %d, want 2", count1)
	}

	count2, err := ledger2.Count()
	if err != nil {
		t.Fatalf("Count via ledger2 failed: %v", err)
	}
	if count2 != 2 {
		t.Errorf("count2 = %d, want 2", count2)
	}
}

// TestLedger_TableDrivenEventTypes tests appending and reading various event types.
func TestLedger_TableDrivenEventTypes(t *testing.T) {
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
			ledger, err := NewLedger(dir)
			if err != nil {
				t.Fatalf("NewLedger failed: %v", err)
			}

			seq, err := ledger.Append(tt.event)
			if err != nil {
				t.Fatalf("Append failed: %v", err)
			}

			if seq != 1 {
				t.Errorf("seq = %d, want 1", seq)
			}

			// Read back and verify
			events, err := ledger.ReadAll()
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}

			if len(events) != 1 {
				t.Fatalf("ReadAll returned %d events, want 1", len(events))
			}

			var base BaseEvent
			if err := json.Unmarshal(events[0], &base); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if base.Type() != tt.eventType {
				t.Errorf("Event type = %q, want %q", base.Type(), tt.eventType)
			}
		})
	}
}

// TestLedger_IgnoresNonEventFiles verifies non-event files don't affect count or reading.
func TestLedger_IgnoresNonEventFiles(t *testing.T) {
	dir := t.TempDir()

	// Create some non-event files
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Append an event
	event := NewProofInitialized("Event", "agent")
	_, err = ledger.Append(event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	count, err := ledger.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Count = %d, want 1 (should ignore non-event files)", count)
	}

	events, err := ledger.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("ReadAll returned %d events, want 1 (should ignore non-event files)", len(events))
	}
}

// TestLedger_CountUpdatesAfterAppend verifies count increases after append.
func TestLedger_CountUpdatesAfterAppend(t *testing.T) {
	dir := t.TempDir()
	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Initial count
	count, err := ledger.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Initial count = %d, want 0", count)
	}

	// Append and check count after each
	for i := 1; i <= 3; i++ {
		event := NewProofInitialized("Event", "agent")
		_, err := ledger.Append(event)
		if err != nil {
			t.Fatalf("Append %d failed: %v", i, err)
		}

		count, err := ledger.Count()
		if err != nil {
			t.Fatalf("Count after append %d failed: %v", i, err)
		}
		if count != i {
			t.Errorf("Count after %d appends = %d, want %d", i, count, i)
		}
	}
}
