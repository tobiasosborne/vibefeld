//go:build integration

package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCleanupTempFiles_RemovesFilesInRange verifies cleanup removes files in specified range.
func TestCleanupTempFiles_RemovesFilesInRange(t *testing.T) {
	dir := t.TempDir()

	// Create 5 temp files
	tempPaths := make([]string, 5)
	for i := 0; i < 5; i++ {
		f, err := os.CreateTemp(dir, "cleanup-test-*.tmp")
		if err != nil {
			t.Fatalf("Failed to create temp file %d: %v", i, err)
		}
		tempPaths[i] = f.Name()
		f.Close()
	}

	// Cleanup files at indices 1, 2, 3 (range [1, 4))
	cleanupTempFiles(tempPaths, 1, 4)

	// Verify files 0 and 4 still exist
	for _, idx := range []int{0, 4} {
		if _, err := os.Stat(tempPaths[idx]); os.IsNotExist(err) {
			t.Errorf("File %d should still exist but was removed", idx)
		}
	}

	// Verify files 1, 2, 3 were removed
	for _, idx := range []int{1, 2, 3} {
		if _, err := os.Stat(tempPaths[idx]); !os.IsNotExist(err) {
			t.Errorf("File %d should have been removed but still exists", idx)
		}
	}
}

// TestCleanupTempFiles_HandlesEmptyRange verifies cleanup handles empty range gracefully.
func TestCleanupTempFiles_HandlesEmptyRange(t *testing.T) {
	dir := t.TempDir()

	// Create a temp file
	f, err := os.CreateTemp(dir, "cleanup-test-*.tmp")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempPath := f.Name()
	f.Close()

	tempPaths := []string{tempPath}

	// Cleanup with empty range (start == end)
	cleanupTempFiles(tempPaths, 0, 0)

	// File should still exist
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		t.Error("File should still exist after empty range cleanup")
	}
}

// TestCleanupTempFiles_SkipsEmptyPaths verifies cleanup skips empty strings in slice.
func TestCleanupTempFiles_SkipsEmptyPaths(t *testing.T) {
	dir := t.TempDir()

	// Create one real file
	f, err := os.CreateTemp(dir, "cleanup-test-*.tmp")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	realPath := f.Name()
	f.Close()

	// Mix of empty paths and real path
	tempPaths := []string{"", realPath, ""}

	// Should not panic and should remove the real file
	cleanupTempFiles(tempPaths, 0, 3)

	// Real file should be removed
	if _, err := os.Stat(realPath); !os.IsNotExist(err) {
		t.Error("Real file should have been removed")
	}
}

// TestCleanupTempFiles_IgnoresNonexistentFiles verifies cleanup silently handles missing files.
func TestCleanupTempFiles_IgnoresNonexistentFiles(t *testing.T) {
	// Paths that don't exist
	tempPaths := []string{
		"/nonexistent/path/file1.tmp",
		"/nonexistent/path/file2.tmp",
	}

	// Should not panic or return error
	cleanupTempFiles(tempPaths, 0, 2)
}

// TestAppend_SingleEvent verifies that appending a single event creates a file with correct sequence.
func TestAppend_SingleEvent(t *testing.T) {
	dir := t.TempDir()

	event := NewProofInitialized("Test conjecture", "agent-001")
	seq, err := Append(dir, event)

	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	if seq != 1 {
		t.Errorf("Append returned seq = %d, want 1", seq)
	}

	// Verify file exists
	filePath := filepath.Join(dir, "000001.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Event file %s was not created", filePath)
	}
}

// TestAppend_MultipleIncrementsSequence verifies that multiple appends increment the sequence number.
func TestAppend_MultipleIncrementsSequence(t *testing.T) {
	dir := t.TempDir()

	// Append first event
	event1 := NewProofInitialized("First conjecture", "agent-001")
	seq1, err := Append(dir, event1)
	if err != nil {
		t.Fatalf("First append failed: %v", err)
	}
	if seq1 != 1 {
		t.Errorf("First append returned seq = %d, want 1", seq1)
	}

	// Append second event
	event2 := NewChallengeResolved("chal-001")
	seq2, err := Append(dir, event2)
	if err != nil {
		t.Fatalf("Second append failed: %v", err)
	}
	if seq2 != 2 {
		t.Errorf("Second append returned seq = %d, want 2", seq2)
	}

	// Append third event
	event3 := NewChallengeWithdrawn("chal-002")
	seq3, err := Append(dir, event3)
	if err != nil {
		t.Fatalf("Third append failed: %v", err)
	}
	if seq3 != 3 {
		t.Errorf("Third append returned seq = %d, want 3", seq3)
	}

	// Verify all files exist
	for i := 1; i <= 3; i++ {
		filePath := EventFilePath(dir, i)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Event file %s was not created", filePath)
		}
	}
}

// TestAppend_EventJSONIsValid verifies that the event file contains valid JSON.
func TestAppend_EventJSONIsValid(t *testing.T) {
	dir := t.TempDir()

	event := NewProofInitialized("JSON test conjecture", "agent-json")
	seq, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Read the file content
	filePath := EventFilePath(dir, seq)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read event file: %v", err)
	}

	// Verify it's valid JSON
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Errorf("Event file contains invalid JSON: %v", err)
	}

	// Verify expected fields are present
	if _, ok := decoded["type"]; !ok {
		t.Error("JSON missing 'type' field")
	}
	if _, ok := decoded["timestamp"]; !ok {
		t.Error("JSON missing 'timestamp' field")
	}
	if _, ok := decoded["conjecture"]; !ok {
		t.Error("JSON missing 'conjecture' field")
	}
	if _, ok := decoded["author"]; !ok {
		t.Error("JSON missing 'author' field")
	}
}

// TestAppend_EventJSONContent verifies that the event JSON matches the event data.
func TestAppend_EventJSONContent(t *testing.T) {
	dir := t.TempDir()

	event := NewProofInitialized("Content test", "agent-content")
	seq, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Read and decode the file
	filePath := EventFilePath(dir, seq)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read event file: %v", err)
	}

	var decoded ProofInitialized
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	// Verify content matches
	if decoded.Type() != EventProofInitialized {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type(), EventProofInitialized)
	}
	if decoded.Conjecture != "Content test" {
		t.Errorf("Conjecture mismatch: got %q, want %q", decoded.Conjecture, "Content test")
	}
	if decoded.Author != "agent-content" {
		t.Errorf("Author mismatch: got %q, want %q", decoded.Author, "agent-content")
	}
}

// TestAppend_AtomicWrite verifies that writes are atomic using temp file + rename.
func TestAppend_AtomicWrite(t *testing.T) {
	dir := t.TempDir()

	event := NewProofInitialized("Atomic test", "agent-atomic")
	seq, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// After successful append, there should be no temp files remaining
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// Temp files typically have .tmp suffix or start with .
		if strings.HasSuffix(name, ".tmp") || strings.HasPrefix(name, ".") {
			t.Errorf("Temp file %q remains after append", name)
		}
	}

	// Verify final file exists
	filePath := EventFilePath(dir, seq)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Final event file does not exist")
	}
}

// TestAppend_AtomicWriteNoPartialFile verifies no partial file on complete operation.
func TestAppend_AtomicWriteNoPartialFile(t *testing.T) {
	dir := t.TempDir()

	// Perform multiple appends
	for i := 0; i < 5; i++ {
		event := NewProofInitialized("Atomic batch test", "agent-batch")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("Append %d failed: %v", i, err)
		}
	}

	// Count event files
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	eventCount := 0
	for _, entry := range entries {
		if IsEventFile(entry.Name()) {
			eventCount++
		}
	}

	if eventCount != 5 {
		t.Errorf("Expected 5 event files, got %d", eventCount)
	}
}

// TestAppend_ErrorOnInvalidDir verifies that append fails on invalid directory.
func TestAppend_ErrorOnInvalidDir(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{"non-existent directory", "/nonexistent/path/that/does/not/exist"},
		{"empty directory path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewProofInitialized("Error test", "agent-error")
			_, err := Append(tt.dir, event)

			if err == nil {
				t.Error("Append should return error for invalid directory")
			}
		})
	}
}

// TestAppend_ErrorOnReadOnlyDir verifies that append fails on read-only directory.
func TestAppend_ErrorOnReadOnlyDir(t *testing.T) {
	dir := t.TempDir()

	// Make directory read-only
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatalf("Failed to make directory read-only: %v", err)
	}
	defer os.Chmod(dir, 0755) // Restore for cleanup

	event := NewProofInitialized("Read-only test", "agent-readonly")
	_, err := AppendWithTimeout(dir, event, 100*time.Millisecond)

	if err == nil {
		t.Error("Append should return error for read-only directory")
	}
}

// TestAppend_PreservesExistingFiles verifies that append doesn't overwrite existing files.
func TestAppend_PreservesExistingFiles(t *testing.T) {
	dir := t.TempDir()

	// Create an existing event file manually
	existingPath := filepath.Join(dir, "000001.json")
	existingContent := []byte(`{"type":"existing","timestamp":"2025-01-01T00:00:00Z"}`)
	if err := os.WriteFile(existingPath, existingContent, 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Append should use next sequence number
	event := NewProofInitialized("New event", "agent-new")
	seq, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	if seq != 2 {
		t.Errorf("Append returned seq = %d, want 2", seq)
	}

	// Verify original file is unchanged
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("Failed to read existing file: %v", err)
	}
	if string(data) != string(existingContent) {
		t.Error("Existing file was modified")
	}
}

// TestAppendBatch_MultipleEvents verifies batch append creates files for all events.
func TestAppendBatch_MultipleEvents(t *testing.T) {
	dir := t.TempDir()

	events := []Event{
		NewProofInitialized("Batch event 1", "agent-batch"),
		NewChallengeResolved("chal-batch-1"),
		NewChallengeWithdrawn("chal-batch-2"),
	}

	seqs, err := AppendBatch(dir, events)
	if err != nil {
		t.Fatalf("AppendBatch failed: %v", err)
	}

	if len(seqs) != 3 {
		t.Errorf("AppendBatch returned %d sequence numbers, want 3", len(seqs))
	}

	// Verify sequence numbers are consecutive starting from 1
	expectedSeqs := []int{1, 2, 3}
	for i, expected := range expectedSeqs {
		if seqs[i] != expected {
			t.Errorf("seqs[%d] = %d, want %d", i, seqs[i], expected)
		}
	}

	// Verify all files exist
	for _, seq := range seqs {
		filePath := EventFilePath(dir, seq)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Event file %s was not created", filePath)
		}
	}
}

// TestAppendBatch_EmptySlice verifies batch append with empty slice returns empty result.
func TestAppendBatch_EmptySlice(t *testing.T) {
	dir := t.TempDir()

	seqs, err := AppendBatch(dir, []Event{})

	if err != nil {
		t.Errorf("AppendBatch with empty slice returned error: %v", err)
	}

	if len(seqs) != 0 {
		t.Errorf("AppendBatch with empty slice returned %d sequence numbers, want 0", len(seqs))
	}
}

// TestAppendBatch_SingleEvent verifies batch append works with single event.
func TestAppendBatch_SingleEvent(t *testing.T) {
	dir := t.TempDir()

	events := []Event{
		NewProofInitialized("Single batch event", "agent-single"),
	}

	seqs, err := AppendBatch(dir, events)
	if err != nil {
		t.Fatalf("AppendBatch failed: %v", err)
	}

	if len(seqs) != 1 {
		t.Errorf("AppendBatch returned %d sequence numbers, want 1", len(seqs))
	}
	if seqs[0] != 1 {
		t.Errorf("seqs[0] = %d, want 1", seqs[0])
	}
}

// TestAppendBatch_Atomic verifies batch append is atomic (all or nothing).
func TestAppendBatch_Atomic(t *testing.T) {
	dir := t.TempDir()

	events := []Event{
		NewProofInitialized("Atomic batch 1", "agent-atomic"),
		NewChallengeResolved("chal-atomic"),
		NewChallengeWithdrawn("chal-atomic-2"),
	}

	_, err := AppendBatch(dir, events)
	if err != nil {
		t.Fatalf("AppendBatch failed: %v", err)
	}

	// After successful batch, no temp files should remain
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".tmp") || strings.HasPrefix(name, ".") {
			t.Errorf("Temp file %q remains after batch append", name)
		}
	}
}

// TestAppendBatch_ErrorOnInvalidDir verifies batch append fails on invalid directory.
func TestAppendBatch_ErrorOnInvalidDir(t *testing.T) {
	events := []Event{
		NewProofInitialized("Error batch", "agent-error"),
	}

	_, err := AppendBatch("/nonexistent/path/that/does/not/exist", events)
	if err == nil {
		t.Error("AppendBatch should return error for invalid directory")
	}
}

// TestAppendBatch_ContinuesFromExisting verifies batch continues from existing sequence.
func TestAppendBatch_ContinuesFromExisting(t *testing.T) {
	dir := t.TempDir()

	// Create existing files
	for i := 1; i <= 3; i++ {
		filePath := EventFilePath(dir, i)
		if err := os.WriteFile(filePath, []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create existing file: %v", err)
		}
	}

	events := []Event{
		NewProofInitialized("Continue batch 1", "agent-continue"),
		NewChallengeResolved("chal-continue"),
	}

	seqs, err := AppendBatch(dir, events)
	if err != nil {
		t.Fatalf("AppendBatch failed: %v", err)
	}

	expectedSeqs := []int{4, 5}
	for i, expected := range expectedSeqs {
		if seqs[i] != expected {
			t.Errorf("seqs[%d] = %d, want %d", i, seqs[i], expected)
		}
	}
}

// TestAppend_TableDriven uses table-driven tests for various event types.
func TestAppend_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		event Event
	}{
		{"ProofInitialized", NewProofInitialized("test", "agent")},
		{"ChallengeResolved", NewChallengeResolved("chal-1")},
		{"ChallengeWithdrawn", NewChallengeWithdrawn("chal-2")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			seq, err := Append(dir, tt.event)
			if err != nil {
				t.Fatalf("Append failed: %v", err)
			}

			if seq != 1 {
				t.Errorf("seq = %d, want 1", seq)
			}

			// Verify file exists and contains valid JSON
			filePath := EventFilePath(dir, seq)
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("Failed to read file: %v", err)
			}

			var decoded map[string]interface{}
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Errorf("Invalid JSON: %v", err)
			}

			// Verify type field matches
			if typeStr, ok := decoded["type"].(string); !ok || typeStr != string(tt.event.Type()) {
				t.Errorf("Type field = %v, want %q", decoded["type"], tt.event.Type())
			}
		})
	}
}

// TestAppend_ConcurrentWrites verifies concurrent appends don't corrupt the ledger.
func TestAppend_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan int, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := NewProofInitialized("Concurrent test", "agent-concurrent")
			seq, err := Append(dir, event)
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

	// Verify all sequence numbers are consecutive (1 to numGoroutines)
	if len(seqSet) != numGoroutines {
		t.Errorf("Expected %d unique sequence numbers, got %d", numGoroutines, len(seqSet))
	}

	for i := 1; i <= numGoroutines; i++ {
		if !seqSet[i] {
			t.Errorf("Missing sequence number: %d", i)
		}
	}
}

// TestAppend_FilePermissions verifies event files have appropriate permissions.
func TestAppend_FilePermissions(t *testing.T) {
	dir := t.TempDir()

	event := NewProofInitialized("Permission test", "agent-perm")
	seq, err := Append(dir, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	filePath := EventFilePath(dir, seq)
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	// File should be readable/writable by owner
	mode := info.Mode().Perm()
	if mode&0600 != 0600 {
		t.Errorf("File should have at least 0600 permissions, got %o", mode)
	}
}

// =============================================================================
// AppendIfSequence CAS (Compare-And-Swap) Tests
// =============================================================================

// TestAppendIfSequence_SuccessOnMatchingSequence verifies CAS succeeds when sequence matches.
func TestAppendIfSequence_SuccessOnMatchingSequence(t *testing.T) {
	dir := t.TempDir()

	// Append first event using regular append
	event1 := NewProofInitialized("CAS test 1", "agent-cas")
	seq1, err := Append(dir, event1)
	if err != nil {
		t.Fatalf("First append failed: %v", err)
	}
	if seq1 != 1 {
		t.Errorf("First append: seq = %d, want 1", seq1)
	}

	// Use AppendIfSequence with correct expected sequence (1)
	event2 := NewChallengeResolved("chal-cas")
	seq2, err := AppendIfSequence(dir, event2, 1)
	if err != nil {
		t.Fatalf("AppendIfSequence failed: %v", err)
	}
	if seq2 != 2 {
		t.Errorf("AppendIfSequence: seq = %d, want 2", seq2)
	}

	// Verify both files exist
	if _, err := os.Stat(EventFilePath(dir, 1)); os.IsNotExist(err) {
		t.Error("Event 1 file should exist")
	}
	if _, err := os.Stat(EventFilePath(dir, 2)); os.IsNotExist(err) {
		t.Error("Event 2 file should exist")
	}
}

// TestAppendIfSequence_FailsOnMismatchedSequence verifies CAS fails when sequence doesn't match.
func TestAppendIfSequence_FailsOnMismatchedSequence(t *testing.T) {
	dir := t.TempDir()

	// Append two events
	event1 := NewProofInitialized("CAS mismatch test 1", "agent-cas")
	if _, err := Append(dir, event1); err != nil {
		t.Fatalf("First append failed: %v", err)
	}

	event2 := NewChallengeResolved("chal-cas-mismatch")
	if _, err := Append(dir, event2); err != nil {
		t.Fatalf("Second append failed: %v", err)
	}

	// Now ledger is at sequence 2. Try AppendIfSequence with expected sequence 1.
	event3 := NewChallengeWithdrawn("chal-cas-mismatch-2")
	_, err := AppendIfSequence(dir, event3, 1) // Expecting seq 1, but ledger is at 2

	if err == nil {
		t.Fatal("AppendIfSequence should fail when sequence mismatches")
	}

	if !strings.Contains(err.Error(), "sequence mismatch") {
		t.Errorf("Error should mention sequence mismatch, got: %v", err)
	}

	// Verify the event was NOT appended (still only 2 events)
	count, err := Count(dir)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Ledger should still have 2 events, got %d", count)
	}
}

// TestAppendIfSequence_FailsOnEmptyLedgerWithNonZeroExpected verifies CAS fails
// when expecting non-zero sequence on empty ledger.
func TestAppendIfSequence_FailsOnEmptyLedgerWithNonZeroExpected(t *testing.T) {
	dir := t.TempDir()

	// Try AppendIfSequence on empty ledger expecting sequence 5
	event := NewProofInitialized("Empty ledger CAS test", "agent-cas")
	_, err := AppendIfSequence(dir, event, 5)

	if err == nil {
		t.Fatal("AppendIfSequence should fail when expecting non-zero on empty ledger")
	}

	if !strings.Contains(err.Error(), "sequence mismatch") {
		t.Errorf("Error should mention sequence mismatch, got: %v", err)
	}

	// Verify nothing was appended
	count, err := Count(dir)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Ledger should be empty, got %d events", count)
	}
}

// TestAppendIfSequence_SuccessOnEmptyLedgerWithZeroExpected verifies CAS succeeds
// when expecting zero sequence on empty ledger.
func TestAppendIfSequence_SuccessOnEmptyLedgerWithZeroExpected(t *testing.T) {
	dir := t.TempDir()

	// AppendIfSequence on empty ledger expecting sequence 0 should succeed
	event := NewProofInitialized("Empty ledger CAS test", "agent-cas")
	seq, err := AppendIfSequence(dir, event, 0)

	if err != nil {
		t.Fatalf("AppendIfSequence should succeed on empty ledger with expected 0: %v", err)
	}
	if seq != 1 {
		t.Errorf("seq = %d, want 1", seq)
	}

	// Verify event was appended
	if _, err := os.Stat(EventFilePath(dir, 1)); os.IsNotExist(err) {
		t.Error("Event file should exist")
	}
}

// TestAppendIfSequence_ConcurrentClaimSimulation simulates two agents trying to claim
// the same node - only one should succeed.
func TestAppendIfSequence_ConcurrentClaimSimulation(t *testing.T) {
	dir := t.TempDir()

	// Setup: Create initial event
	initEvent := NewProofInitialized("Concurrent claim test", "agent-init")
	if _, err := Append(dir, initEvent); err != nil {
		t.Fatalf("Init append failed: %v", err)
	}

	// Current ledger sequence is 1
	currentSeq := 1

	// Two goroutines both try to append with expected sequence 1
	// Only one should succeed; the other should get sequence mismatch
	var wg sync.WaitGroup
	successes := make(chan int, 2)
	failures := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(agentNum int) {
			defer wg.Done()

			// Both agents "read" the same sequence
			expectedSeq := currentSeq

			// Try to append (claim) with their expected sequence
			event := NewChallengeResolved("claim-" + string(rune('A'+agentNum)))
			seq, err := AppendIfSequence(dir, event, expectedSeq)

			if err != nil {
				failures <- err
			} else {
				successes <- seq
			}
		}(i)
	}

	wg.Wait()
	close(successes)
	close(failures)

	// Count results
	successCount := 0
	failureCount := 0

	for range successes {
		successCount++
	}
	for range failures {
		failureCount++
	}

	// Exactly one should succeed, one should fail
	if successCount != 1 {
		t.Errorf("Expected exactly 1 success, got %d", successCount)
	}
	if failureCount != 1 {
		t.Errorf("Expected exactly 1 failure, got %d", failureCount)
	}

	// Ledger should have exactly 2 events (init + one successful claim)
	count, err := Count(dir)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Ledger should have 2 events, got %d", count)
	}
}

// TestAppendIfSequence_ErrorTypes verifies the error wrapping includes ErrSequenceMismatch.
func TestAppendIfSequence_ErrorTypes(t *testing.T) {
	dir := t.TempDir()

	// Create one event
	event1 := NewProofInitialized("Error type test", "agent-error")
	if _, err := Append(dir, event1); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Try with wrong expected sequence
	event2 := NewChallengeResolved("chal-error")
	_, err := AppendIfSequence(dir, event2, 0) // Wrong: ledger is at 1

	if err == nil {
		t.Fatal("Expected error")
	}

	// Verify error contains ErrSequenceMismatch
	if !strings.Contains(err.Error(), ErrSequenceMismatch.Error()) {
		t.Errorf("Error should wrap ErrSequenceMismatch, got: %v", err)
	}
}

// TestAppendIfSequence_InvalidDirectory verifies proper error on invalid directory.
func TestAppendIfSequence_InvalidDirectory(t *testing.T) {
	event := NewProofInitialized("Invalid dir test", "agent-invalid")
	_, err := AppendIfSequence("/nonexistent/directory/that/does/not/exist", event, 0)

	if err == nil {
		t.Fatal("Expected error for invalid directory")
	}
}

// TestAppendIfSequence_ChainedOperations verifies multiple CAS operations in sequence.
func TestAppendIfSequence_ChainedOperations(t *testing.T) {
	dir := t.TempDir()

	// Chain multiple AppendIfSequence calls, each using the previous sequence
	events := []Event{
		NewProofInitialized("Chain 1", "agent-chain"),
		NewChallengeResolved("chal-chain-1"),
		NewChallengeWithdrawn("chal-chain-2"),
	}

	expectedSeq := 0 // Start with empty ledger
	for i, event := range events {
		seq, err := AppendIfSequence(dir, event, expectedSeq)
		if err != nil {
			t.Fatalf("AppendIfSequence %d failed: %v", i, err)
		}
		if seq != i+1 {
			t.Errorf("AppendIfSequence %d: seq = %d, want %d", i, seq, i+1)
		}
		expectedSeq = seq // Update for next iteration
	}

	// Verify all events were appended
	count, err := Count(dir)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Ledger should have 3 events, got %d", count)
	}
}
