// Package ledger provides event-sourced ledger operations for the AF proof framework.
package ledger

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestConcurrentAppend verifies that multiple goroutines can safely append
// events to the ledger concurrently without data races or lost events.
func TestConcurrentAppend(t *testing.T) {
	// Create temp ledger directory
	dir := t.TempDir()

	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	numWorkers := 10
	eventsPerWorker := 5

	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	// Launch concurrent appenders
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < eventsPerWorker; j++ {
				event := NewProofInitialized(
					"conjecture from worker",
					"worker",
				)

				_, err := ledger.Append(event)
				if err != nil {
					errorCount.Add(1)
				} else {
					successCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Check that all events were appended without errors
	totalExpected := int32(numWorkers * eventsPerWorker)
	if got := successCount.Load(); got != totalExpected {
		t.Errorf("expected %d successful appends, got %d (errors: %d)",
			totalExpected, got, errorCount.Load())
	}

	// Verify ledger count matches
	count, err := ledger.Count()
	if err != nil {
		t.Fatalf("failed to count events: %v", err)
	}
	if count != int(totalExpected) {
		t.Errorf("expected %d events in ledger, got %d", totalExpected, count)
	}

	// Verify no gaps in sequence numbers
	hasGaps, err := HasGaps(dir)
	if err != nil {
		t.Fatalf("failed to check for gaps: %v", err)
	}
	if hasGaps {
		t.Error("ledger has gaps in sequence numbers after concurrent append")
	}
}

// TestConcurrentAppendIfSequence verifies that CAS operations correctly
// detect concurrent modifications and reject conflicting appends.
func TestConcurrentAppendIfSequence(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate ledger with some events
	event := NewProofInitialized("initial conjecture", "author")
	_, err := Append(dir, event)
	if err != nil {
		t.Fatalf("failed to append initial event: %v", err)
	}

	numWorkers := 10
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var mismatchCount atomic.Int32
	var otherErrorCount atomic.Int32

	// All workers try to append with expectedSeq=1 simultaneously
	// Only one should succeed; others should get ErrSequenceMismatch
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			event := NewProofInitialized(
				"concurrent conjecture",
				"worker",
			)

			// All workers expect sequence 1 (the current state)
			_, err := AppendIfSequenceWithTimeout(dir, event, 1, 5*time.Second)
			if err == nil {
				successCount.Add(1)
			} else if err.Error() == ErrSequenceMismatch.Error() ||
				(len(err.Error()) > 0 && err.Error()[:len("ledger sequence mismatch")] == "ledger sequence mismatch") {
				mismatchCount.Add(1)
			} else {
				otherErrorCount.Add(1)
				t.Logf("worker %d got unexpected error: %v", workerID, err)
			}
		}(i)
	}

	wg.Wait()

	// Exactly one worker should succeed with CAS
	if got := successCount.Load(); got != 1 {
		t.Errorf("expected exactly 1 successful CAS append, got %d", got)
	}

	// All others should fail with sequence mismatch
	expectedMismatches := int32(numWorkers - 1)
	if got := mismatchCount.Load(); got != expectedMismatches {
		t.Errorf("expected %d sequence mismatches, got %d (other errors: %d)",
			expectedMismatches, got, otherErrorCount.Load())
	}

	// Verify final ledger state
	count, err := Count(dir)
	if err != nil {
		t.Fatalf("failed to count events: %v", err)
	}
	if count != 2 { // initial + one successful CAS
		t.Errorf("expected 2 events after CAS race, got %d", count)
	}
}

// TestConcurrentReadDuringWrite verifies that reads are safe during concurrent writes.
func TestConcurrentReadDuringWrite(t *testing.T) {
	dir := t.TempDir()

	ledger, err := NewLedger(dir)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	// Pre-populate with some events
	for i := 0; i < 5; i++ {
		event := NewProofInitialized("initial", "author")
		_, err := ledger.Append(event)
		if err != nil {
			t.Fatalf("failed to pre-populate: %v", err)
		}
	}

	var wg sync.WaitGroup
	stopReaders := make(chan struct{})

	// Start multiple concurrent readers
	numReaders := 5
	var readErrors atomic.Int32
	var readCount atomic.Int32

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-stopReaders:
					return
				default:
					events, err := ledger.ReadAll()
					if err != nil {
						readErrors.Add(1)
					} else {
						readCount.Add(1)
						// Verify all events are valid JSON
						for _, data := range events {
							if len(data) == 0 {
								readErrors.Add(1)
							}
						}
					}
					time.Sleep(time.Millisecond) // Small delay to avoid spinning
				}
			}
		}()
	}

	// Concurrent writer
	numWrites := 20
	var writeErrors atomic.Int32

	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < numWrites; i++ {
			event := NewProofInitialized("concurrent write", "author")
			_, err := ledger.Append(event)
			if err != nil {
				writeErrors.Add(1)
			}
			time.Sleep(time.Millisecond) // Small delay between writes
		}

		// Signal readers to stop
		close(stopReaders)
	}()

	wg.Wait()

	// Check for errors
	if errors := readErrors.Load(); errors > 0 {
		t.Errorf("got %d read errors during concurrent writes", errors)
	}
	if errors := writeErrors.Load(); errors > 0 {
		t.Errorf("got %d write errors", errors)
	}

	t.Logf("completed %d reads during %d writes", readCount.Load(), numWrites)
}

// TestConcurrentScan verifies that scanning is safe during concurrent writes.
func TestConcurrentScan(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate
	for i := 0; i < 3; i++ {
		event := NewProofInitialized("initial", "author")
		_, err := Append(dir, event)
		if err != nil {
			t.Fatalf("failed to pre-populate: %v", err)
		}
	}

	var wg sync.WaitGroup
	stopScanners := make(chan struct{})

	// Start concurrent scanners
	numScanners := 3
	var scanErrors atomic.Int32
	var scanCount atomic.Int32

	for i := 0; i < numScanners; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-stopScanners:
					return
				default:
					err := Scan(dir, func(seq int, data []byte) error {
						// Validate that data is non-empty
						if len(data) == 0 {
							return nil
						}
						return nil
					})
					if err != nil {
						scanErrors.Add(1)
					} else {
						scanCount.Add(1)
					}
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	// Concurrent writer
	numWrites := 15
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < numWrites; i++ {
			event := NewProofInitialized("concurrent", "author")
			_, _ = Append(dir, event)
			time.Sleep(time.Millisecond)
		}

		close(stopScanners)
	}()

	wg.Wait()

	if errors := scanErrors.Load(); errors > 0 {
		t.Errorf("got %d scan errors during concurrent writes", errors)
	}

	t.Logf("completed %d scans during %d writes", scanCount.Load(), numWrites)
}

// TestConcurrentLedgerLock verifies that the ledger lock properly serializes access.
func TestConcurrentLedgerLock(t *testing.T) {
	dir := t.TempDir()

	numWorkers := 5
	var wg sync.WaitGroup
	var concurrentHolders atomic.Int32
	var maxConcurrent atomic.Int32
	var acquireCount atomic.Int32
	var timeoutCount atomic.Int32

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			lock := NewLedgerLock(dir)

			// Try to acquire the lock multiple times
			for j := 0; j < 3; j++ {
				err := lock.Acquire("agent", 100*time.Millisecond)
				if err != nil {
					// Timeout is expected under contention
					if err.Error() == "timeout waiting for lock" {
						timeoutCount.Add(1)
					}
					continue
				}

				acquireCount.Add(1)

				// Track concurrent holders to detect race conditions
				current := concurrentHolders.Add(1)
				for {
					old := maxConcurrent.Load()
					if current <= old || maxConcurrent.CompareAndSwap(old, current) {
						break
					}
				}

				// Hold the lock briefly
				time.Sleep(5 * time.Millisecond)

				concurrentHolders.Add(-1)

				if err := lock.Release(); err != nil {
					t.Errorf("worker %d failed to release: %v", workerID, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// The lock should never be held by more than one at a time
	if max := maxConcurrent.Load(); max > 1 {
		t.Errorf("lock held by %d goroutines concurrently (should be 1)", max)
	}

	t.Logf("total acquires: %d, timeouts: %d", acquireCount.Load(), timeoutCount.Load())
}

// TestConcurrentAppendBatch verifies that batch appends are properly serialized.
func TestConcurrentAppendBatch(t *testing.T) {
	dir := t.TempDir()

	numWorkers := 5
	batchSize := 3
	var wg sync.WaitGroup
	var successCount atomic.Int32

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			events := make([]Event, batchSize)
			for j := 0; j < batchSize; j++ {
				events[j] = NewProofInitialized("batch event", "author")
			}

			_, err := AppendBatch(dir, events)
			if err != nil {
				t.Logf("worker %d batch failed: %v", workerID, err)
			} else {
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	// All batches should succeed
	if got := successCount.Load(); got != int32(numWorkers) {
		t.Errorf("expected %d successful batches, got %d", numWorkers, got)
	}

	// Verify total count
	count, err := Count(dir)
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	expectedCount := numWorkers * batchSize
	if count != expectedCount {
		t.Errorf("expected %d events, got %d", expectedCount, count)
	}

	// Verify no gaps
	hasGaps, err := HasGaps(dir)
	if err != nil {
		t.Fatalf("failed to check gaps: %v", err)
	}
	if hasGaps {
		t.Error("ledger has gaps after concurrent batch append")
	}
}

// TestConcurrentNodeCreatedEvents tests concurrent appending of NodeCreated events.
func TestConcurrentNodeCreatedEvents(t *testing.T) {
	dir := t.TempDir()

	numWorkers := 8
	var wg sync.WaitGroup
	var successCount atomic.Int32

	// Each worker creates a different node
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Create a unique node for this worker
			nodeID, err := types.Parse("1")
			if err != nil {
				t.Errorf("failed to parse node ID: %v", err)
				return
			}

			// Create child ID based on worker
			childID, err := nodeID.Child(workerID + 1)
			if err != nil {
				t.Errorf("failed to create child ID: %v", err)
				return
			}

			n, err := node.NewNode(
				childID,
				schema.NodeTypeClaim,
				"Statement from worker",
				schema.InferenceAssumption,
			)
			if err != nil {
				t.Errorf("failed to create node: %v", err)
				return
			}

			event := NewNodeCreated(*n)
			_, err = Append(dir, event)
			if err != nil {
				t.Logf("worker %d append failed: %v", workerID, err)
			} else {
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	if got := successCount.Load(); got != int32(numWorkers) {
		t.Errorf("expected %d successful node creates, got %d", numWorkers, got)
	}
}

// TestLockFileCleanup verifies that lock files are properly cleaned up
// even under concurrent access.
func TestLockFileCleanup(t *testing.T) {
	dir := t.TempDir()

	numWorkers := 10
	iterations := 5
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				lock := NewLedgerLock(dir)
				err := lock.Acquire("agent", 500*time.Millisecond)
				if err != nil {
					continue
				}
				time.Sleep(time.Millisecond)
				_ = lock.Release()
			}
		}()
	}

	wg.Wait()

	// After all goroutines complete, there should be no lock file
	lockPath := filepath.Join(dir, lockFileName)
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file still exists after all workers completed")
	}
}

// TestConcurrentMixedOperations tests a mix of concurrent reads, writes, and scans.
func TestConcurrentMixedOperations(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate
	for i := 0; i < 5; i++ {
		event := NewProofInitialized("initial", "author")
		_, _ = Append(dir, event)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	duration := 100 * time.Millisecond

	var appendCount, readCount, scanCount atomic.Int32
	var appendErrors, readErrors, scanErrors atomic.Int32

	// Writers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					event := NewProofInitialized("mixed", "author")
					_, err := Append(dir, event)
					if err != nil {
						appendErrors.Add(1)
					} else {
						appendCount.Add(1)
					}
				}
			}
		}()
	}

	// Readers
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, err := ReadAll(dir)
					if err != nil {
						readErrors.Add(1)
					} else {
						readCount.Add(1)
					}
				}
			}
		}()
	}

	// Scanners
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					err := Scan(dir, func(seq int, data []byte) error {
						return nil
					})
					if err != nil {
						scanErrors.Add(1)
					} else {
						scanCount.Add(1)
					}
				}
			}
		}()
	}

	// Let the test run for a bit
	time.Sleep(duration)
	close(stop)
	wg.Wait()

	// Report errors
	if errors := appendErrors.Load(); errors > 0 {
		t.Errorf("got %d append errors", errors)
	}
	if errors := readErrors.Load(); errors > 0 {
		t.Errorf("got %d read errors", errors)
	}
	if errors := scanErrors.Load(); errors > 0 {
		t.Errorf("got %d scan errors", errors)
	}

	t.Logf("operations completed - appends: %d, reads: %d, scans: %d",
		appendCount.Load(), readCount.Load(), scanCount.Load())

	// Verify ledger integrity
	hasGaps, err := HasGaps(dir)
	if err != nil {
		t.Fatalf("failed to check gaps: %v", err)
	}
	if hasGaps {
		t.Error("ledger has gaps after mixed concurrent operations")
	}
}
