package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// deferRelease is a test helper that releases a lock and logs any errors.
// This ensures deferred release errors are not silently ignored.
func deferRelease(t *testing.T, lock *LedgerLock) {
	t.Helper()
	if err := lock.Release(); err != nil {
		t.Logf("warning: lock release failed: %v", err)
	}
}

// TestNewLedgerLock verifies that NewLedgerLock creates a valid LedgerLock instance.
func TestNewLedgerLock(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)

	if lock == nil {
		t.Fatal("NewLedgerLock returned nil")
	}
}

// TestAcquireLock_Success verifies successful lock acquisition when no lock exists.
func TestAcquireLock_Success(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)

	err := lock.Acquire("agent-001", 1*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer deferRelease(t, lock)

	// Verify lock file exists
	lockPath := filepath.Join(dir, "ledger.lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file was not created")
	}
}

// TestAcquireLock_ContainsMetadata verifies that the lock file contains agent ID and timestamp.
func TestAcquireLock_ContainsMetadata(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)

	beforeAcquire := time.Now()
	err := lock.Acquire("test-agent-42", 1*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer deferRelease(t, lock)
	afterAcquire := time.Now()

	// Read metadata via Holder method
	agentID, acquiredAt, err := lock.Holder()
	if err != nil {
		t.Fatalf("Holder failed: %v", err)
	}

	if agentID != "test-agent-42" {
		t.Errorf("expected agent ID 'test-agent-42', got %q", agentID)
	}

	if acquiredAt.Before(beforeAcquire) || acquiredAt.After(afterAcquire) {
		t.Errorf("acquiredAt %v not within expected range [%v, %v]", acquiredAt, beforeAcquire, afterAcquire)
	}
}

// TestAcquireLock_Exclusive verifies that a second acquisition attempt fails while lock is held.
func TestAcquireLock_Exclusive(t *testing.T) {
	dir := t.TempDir()
	lock1 := NewLedgerLock(dir)
	lock2 := NewLedgerLock(dir)

	// First acquisition should succeed
	err := lock1.Acquire("agent-first", 1*time.Second)
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}
	defer deferRelease(t, lock1)

	// Second acquisition should fail (with short timeout)
	err = lock2.Acquire("agent-second", 100*time.Millisecond)
	if err == nil {
		lock2.Release()
		t.Fatal("Second acquire should have failed but succeeded")
	}
}

// TestReleaseLock_AllowsReacquisition verifies that after release, lock can be acquired again.
func TestReleaseLock_AllowsReacquisition(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)

	// Acquire and release
	err := lock.Acquire("agent-001", 1*time.Second)
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}
	err = lock.Release()
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	// Should be able to acquire again
	err = lock.Acquire("agent-002", 1*time.Second)
	if err != nil {
		t.Fatalf("Second acquire after release failed: %v", err)
	}
	defer deferRelease(t, lock)
}

// TestReleaseLock_RemovesLockFile verifies that release removes the lock file.
func TestReleaseLock_RemovesLockFile(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)

	err := lock.Acquire("agent-001", 1*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	err = lock.Release()
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	// Lock file should not exist after release
	lockPath := filepath.Join(dir, "ledger.lock")
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file still exists after release")
	}
}

// TestIsHeld verifies the IsHeld method returns correct state.
func TestIsHeld(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)

	// Initially not held
	if lock.IsHeld() {
		t.Error("IsHeld should return false before acquisition")
	}

	// After acquire, should be held
	err := lock.Acquire("agent-001", 1*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	if !lock.IsHeld() {
		t.Error("IsHeld should return true after acquisition")
	}

	// After release, should not be held
	err = lock.Release()
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	if lock.IsHeld() {
		t.Error("IsHeld should return false after release")
	}
}

// TestHolder_NoLock verifies Holder returns error when no lock exists.
func TestHolder_NoLock(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)

	_, _, err := lock.Holder()
	if err == nil {
		t.Error("Holder should return error when no lock exists")
	}
}

// TestAcquireLock_Timeout verifies that acquisition respects timeout parameter.
func TestAcquireLock_Timeout(t *testing.T) {
	dir := t.TempDir()
	lock1 := NewLedgerLock(dir)
	lock2 := NewLedgerLock(dir)

	// Hold lock with first instance
	err := lock1.Acquire("agent-holder", 1*time.Second)
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}
	defer deferRelease(t, lock1)

	// Second acquire should timeout
	timeout := 200 * time.Millisecond
	start := time.Now()
	err = lock2.Acquire("agent-waiter", timeout)
	elapsed := time.Since(start)

	if err == nil {
		lock2.Release()
		t.Fatal("Acquire should have failed due to timeout")
	}

	// Elapsed time should be close to timeout (allow some margin)
	if elapsed < timeout-50*time.Millisecond {
		t.Errorf("Acquire returned too quickly: %v (expected ~%v)", elapsed, timeout)
	}
	if elapsed > timeout+100*time.Millisecond {
		t.Errorf("Acquire took too long: %v (expected ~%v)", elapsed, timeout)
	}
}

// TestAcquireLock_ZeroTimeout verifies immediate failure with zero timeout.
func TestAcquireLock_ZeroTimeout(t *testing.T) {
	dir := t.TempDir()
	lock1 := NewLedgerLock(dir)
	lock2 := NewLedgerLock(dir)

	// Hold lock with first instance
	err := lock1.Acquire("agent-holder", 1*time.Second)
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}
	defer deferRelease(t, lock1)

	// Zero timeout should fail immediately
	start := time.Now()
	err = lock2.Acquire("agent-waiter", 0)
	elapsed := time.Since(start)

	if err == nil {
		lock2.Release()
		t.Fatal("Acquire with zero timeout should have failed")
	}

	// Should return very quickly
	if elapsed > 50*time.Millisecond {
		t.Errorf("Zero timeout acquire took too long: %v", elapsed)
	}
}

// TestConcurrentAccess_Blocking simulates concurrent access attempts.
func TestConcurrentAccess_Blocking(t *testing.T) {
	dir := t.TempDir()

	const numGoroutines = 5
	acquired := make([]bool, numGoroutines)
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	// All goroutines try to acquire simultaneously
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			lock := NewLedgerLock(dir)
			err := lock.Acquire("agent-"+string(rune('A'+idx)), 50*time.Millisecond)
			if err == nil {
				acquired[idx] = true
				mu.Lock()
				successCount++
				mu.Unlock()
				// Hold lock briefly
				time.Sleep(10 * time.Millisecond)
				lock.Release()
			}
		}(i)
	}

	wg.Wait()

	// At least one should have succeeded
	if successCount == 0 {
		t.Error("No goroutine acquired the lock")
	}

	// Count how many acquired - with short timeout, likely only 1 will succeed
	t.Logf("Concurrent test: %d/%d goroutines acquired lock", successCount, numGoroutines)
}

// TestConcurrentAccess_Sequential verifies sequential access with longer timeouts.
func TestConcurrentAccess_Sequential(t *testing.T) {
	dir := t.TempDir()

	const numGoroutines = 3
	var wg sync.WaitGroup
	var mu sync.Mutex
	order := make([]string, 0, numGoroutines)

	// All goroutines try to acquire with longer timeout
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			lock := NewLedgerLock(dir)
			agentID := "agent-" + string(rune('A'+idx))
			err := lock.Acquire(agentID, 5*time.Second)
			if err == nil {
				mu.Lock()
				order = append(order, agentID)
				mu.Unlock()
				// Hold lock briefly
				time.Sleep(20 * time.Millisecond)
				lock.Release()
			}
		}(i)
	}

	wg.Wait()

	// All should have acquired (sequentially)
	if len(order) != numGoroutines {
		t.Errorf("Expected all %d goroutines to acquire lock, got %d", numGoroutines, len(order))
	}
	t.Logf("Sequential acquisition order: %v", order)
}

// TestRelease_WithoutAcquire verifies Release behavior when lock was never acquired.
func TestRelease_WithoutAcquire(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)

	// Release without acquire should not panic and should return error
	err := lock.Release()
	if err == nil {
		t.Error("Release without acquire should return error")
	}
}

// TestRelease_DoubleFree verifies that double release returns error.
func TestRelease_DoubleFree(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)

	err := lock.Acquire("agent-001", 1*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	err = lock.Release()
	if err != nil {
		t.Fatalf("First release failed: %v", err)
	}

	// Second release should return error
	err = lock.Release()
	if err == nil {
		t.Error("Second release should return error")
	}
}

// TestAcquireLock_TableDriven uses table-driven tests for various scenarios.
func TestAcquireLock_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		agentID   string
		timeout   time.Duration
		preLock   bool // if true, another lock holds the file
		wantErr   bool
	}{
		{
			name:    "valid agent ID and timeout",
			agentID: "agent-123",
			timeout: 1 * time.Second,
			preLock: false,
			wantErr: false,
		},
		{
			name:    "empty agent ID",
			agentID: "",
			timeout: 1 * time.Second,
			preLock: false,
			wantErr: true, // empty agent ID should fail
		},
		{
			name:    "lock already held",
			agentID: "agent-waiting",
			timeout: 50 * time.Millisecond,
			preLock: true,
			wantErr: true,
		},
		{
			name:    "agent ID with special chars",
			agentID: "agent_with-special.chars:123",
			timeout: 1 * time.Second,
			preLock: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			var preLock *LedgerLock
			if tt.preLock {
				preLock = NewLedgerLock(dir)
				if err := preLock.Acquire("pre-holder", 1*time.Second); err != nil {
					t.Fatalf("Failed to set up pre-lock: %v", err)
				}
				defer deferRelease(t, preLock)
			}

			lock := NewLedgerLock(dir)
			err := lock.Acquire(tt.agentID, tt.timeout)

			if (err != nil) != tt.wantErr {
				t.Errorf("Acquire() error = %v, wantErr = %v", err, tt.wantErr)
			}

			if err == nil {
				lock.Release()
			}
		})
	}
}

// TestLockFilePermissions verifies the lock file has appropriate permissions.
func TestLockFilePermissions(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)

	err := lock.Acquire("agent-001", 1*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer deferRelease(t, lock)

	lockPath := filepath.Join(dir, "ledger.lock")
	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("Failed to stat lock file: %v", err)
	}

	// Lock file should be readable/writable by owner
	mode := info.Mode().Perm()
	if mode&0600 != 0600 {
		t.Errorf("Lock file should have at least 0600 permissions, got %o", mode)
	}
}

// TestMultipleLocksInDifferentDirs verifies locks in different directories are independent.
func TestMultipleLocksInDifferentDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	lock1 := NewLedgerLock(dir1)
	lock2 := NewLedgerLock(dir2)

	// Both should be acquirable simultaneously
	err1 := lock1.Acquire("agent-001", 1*time.Second)
	if err1 != nil {
		t.Fatalf("Acquire lock1 failed: %v", err1)
	}
	defer deferRelease(t, lock1)

	err2 := lock2.Acquire("agent-002", 1*time.Second)
	if err2 != nil {
		t.Fatalf("Acquire lock2 failed: %v", err2)
	}
	defer deferRelease(t, lock2)

	// Both should report as held
	if !lock1.IsHeld() {
		t.Error("lock1 should be held")
	}
	if !lock2.IsHeld() {
		t.Error("lock2 should be held")
	}
}

// TestAcquireLock_NonExistentDir verifies behavior with non-existent directory.
func TestAcquireLock_NonExistentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent", "subdir")
	lock := NewLedgerLock(dir)

	// Should fail since directory doesn't exist
	err := lock.Acquire("agent-001", 100*time.Millisecond)
	if err == nil {
		lock.Release()
		t.Error("Acquire should fail with non-existent directory")
	}
}

// TestRelease_VerifiesOwnership verifies that Release checks lock file ownership.
func TestRelease_VerifiesOwnership(t *testing.T) {
	dir := t.TempDir()
	lock1 := NewLedgerLock(dir)

	// Agent 1 acquires the lock
	err := lock1.Acquire("agent-001", 1*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Manually overwrite the lock file with a different agent ID
	// This simulates a scenario where the lock file has been replaced
	lockPath := filepath.Join(dir, "ledger.lock")
	meta := `{"agent_id":"agent-INTRUDER","acquired_at":"2025-01-01T00:00:00Z"}`
	err = os.WriteFile(lockPath, []byte(meta), 0600)
	if err != nil {
		t.Fatalf("Failed to overwrite lock file: %v", err)
	}

	// Release should fail because lock file now has different agent ID
	err = lock1.Release()
	if err == nil {
		t.Error("Release should fail when lock file ownership doesn't match")
	}
	if err != nil && !strings.Contains(err.Error(), "ownership mismatch") {
		t.Errorf("Expected ownership mismatch error, got: %v", err)
	}
}

// TestRelease_OwnershipMatchSucceeds verifies that Release succeeds when ownership matches.
func TestRelease_OwnershipMatchSucceeds(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)

	err := lock.Acquire("agent-owner", 1*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Release should succeed because we acquired with the same agent ID
	err = lock.Release()
	if err != nil {
		t.Errorf("Release should succeed when ownership matches: %v", err)
	}

	// Verify lock file is removed
	lockPath := filepath.Join(dir, "ledger.lock")
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("Lock file should be removed after successful release")
	}
}
