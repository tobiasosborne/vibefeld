package lock_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	aferrors "github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/lock"
	"github.com/tobias/vibefeld/internal/types"
)

// createTestLedger creates a temporary ledger for testing.
func createTestLedger(t *testing.T) (*ledger.Ledger, string) {
	t.Helper()
	dir := t.TempDir()
	l, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("failed to create test ledger: %v", err)
	}
	return l, dir
}

// TestPersistentManager_NewPersistentManager verifies manager creation with valid ledger.
func TestPersistentManager_NewPersistentManager(t *testing.T) {
	l, _ := createTestLedger(t)

	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	if pm == nil {
		t.Fatal("NewPersistentManager() returned nil")
	}

	// Fresh manager should have no locks
	locks := pm.ListAll()
	if len(locks) != 0 {
		t.Errorf("Fresh manager has %d locks, want 0", len(locks))
	}
}

// TestPersistentManager_NewPersistentManager_NilLedger verifies nil ledger is rejected.
func TestPersistentManager_NewPersistentManager_NilLedger(t *testing.T) {
	_, err := lock.NewPersistentManager(nil)
	if err == nil {
		t.Error("NewPersistentManager(nil) expected error, got nil")
	}
}

// TestPersistentManager_Acquire_Success verifies acquiring a lock succeeds and persists.
func TestPersistentManager_Acquire_Success(t *testing.T) {
	l, _ := createTestLedger(t)
	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")
	owner := "agent-001"
	timeout := 5 * time.Minute

	lk, err := pm.Acquire(nodeID, owner, timeout)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	if lk == nil {
		t.Fatal("Acquire() returned nil lock")
	}

	// Verify lock properties
	if lk.NodeID().String() != "1.1" {
		t.Errorf("Lock NodeID = %q, want %q", lk.NodeID().String(), "1.1")
	}

	if lk.Owner() != owner {
		t.Errorf("Lock Owner = %q, want %q", lk.Owner(), owner)
	}

	// Verify lock is recorded in ledger
	count, err := l.Count()
	if err != nil {
		t.Fatalf("Count() unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("Ledger has %d events, want 1", count)
	}
}

// TestPersistentManager_Acquire_AlreadyLocked verifies acquiring an already locked node fails.
func TestPersistentManager_Acquire_AlreadyLocked(t *testing.T) {
	l, _ := createTestLedger(t)
	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")

	// First acquisition should succeed
	_, err = pm.Acquire(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("First Acquire() unexpected error: %v", err)
	}

	// Second acquisition should fail
	_, err = pm.Acquire(nodeID, "agent-002", 5*time.Minute)
	if err == nil {
		t.Error("Second Acquire() on locked node expected error, got nil")
	}
}

// TestPersistentManager_Release_Success verifies releasing a lock succeeds and persists.
func TestPersistentManager_Release_Success(t *testing.T) {
	l, _ := createTestLedger(t)
	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")
	owner := "agent-001"

	// Acquire lock
	_, err = pm.Acquire(nodeID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Release lock
	err = pm.Release(nodeID, owner)
	if err != nil {
		t.Fatalf("Release() unexpected error: %v", err)
	}

	// Verify node is no longer locked
	if pm.IsLocked(nodeID) {
		t.Error("IsLocked() = true after Release, want false")
	}

	// Verify release is recorded in ledger (should have 2 events: acquire + release)
	count, err := l.Count()
	if err != nil {
		t.Fatalf("Count() unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("Ledger has %d events, want 2", count)
	}
}

// TestPersistentManager_Release_WrongOwner verifies releasing by wrong owner fails.
func TestPersistentManager_Release_WrongOwner(t *testing.T) {
	l, _ := createTestLedger(t)
	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")

	// Acquire lock
	_, err = pm.Acquire(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Release by different owner should fail
	err = pm.Release(nodeID, "agent-002")
	if err == nil {
		t.Error("Release() by wrong owner expected error, got nil")
	}

	// Lock should still exist
	if !pm.IsLocked(nodeID) {
		t.Error("IsLocked() = false after failed Release, want true")
	}
}

// TestPersistentManager_Persistence_SurvivesRestart verifies locks survive manager restart.
func TestPersistentManager_Persistence_SurvivesRestart(t *testing.T) {
	l, dir := createTestLedger(t)

	// Create first manager and acquire lock
	pm1, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")
	owner := "agent-001"

	_, err = pm1.Acquire(nodeID, owner, 1*time.Hour)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Simulate restart: create new ledger and manager from same directory
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() for restart unexpected error: %v", err)
	}

	pm2, err := lock.NewPersistentManager(l2)
	if err != nil {
		t.Fatalf("NewPersistentManager() for restart unexpected error: %v", err)
	}

	// Lock should still exist in new manager
	if !pm2.IsLocked(nodeID) {
		t.Error("IsLocked() = false after restart, want true (lock should persist)")
	}

	// Info should return the lock
	lk, err := pm2.Info(nodeID)
	if err != nil {
		t.Fatalf("Info() unexpected error: %v", err)
	}
	if lk == nil {
		t.Fatal("Info() returned nil for persisted lock")
	}
	if lk.Owner() != owner {
		t.Errorf("Lock Owner = %q after restart, want %q", lk.Owner(), owner)
	}
}

// TestPersistentManager_Persistence_ReleaseSurvivesRestart verifies released locks stay released.
func TestPersistentManager_Persistence_ReleaseSurvivesRestart(t *testing.T) {
	l, dir := createTestLedger(t)

	// Create first manager, acquire and release lock
	pm1, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")
	owner := "agent-001"

	_, err = pm1.Acquire(nodeID, owner, 1*time.Hour)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	err = pm1.Release(nodeID, owner)
	if err != nil {
		t.Fatalf("Release() unexpected error: %v", err)
	}

	// Simulate restart
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() for restart unexpected error: %v", err)
	}

	pm2, err := lock.NewPersistentManager(l2)
	if err != nil {
		t.Fatalf("NewPersistentManager() for restart unexpected error: %v", err)
	}

	// Lock should NOT exist in new manager (it was released)
	if pm2.IsLocked(nodeID) {
		t.Error("IsLocked() = true after restart, want false (lock was released)")
	}

	// Should be able to acquire again
	_, err = pm2.Acquire(nodeID, "agent-002", 1*time.Hour)
	if err != nil {
		t.Errorf("Acquire() after restart on released node unexpected error: %v", err)
	}
}

// TestPersistentManager_Info_ReturnsNilForUnlocked verifies Info returns nil for unlocked nodes.
func TestPersistentManager_Info_ReturnsNilForUnlocked(t *testing.T) {
	l, _ := createTestLedger(t)
	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")

	lk, err := pm.Info(nodeID)
	if err != nil {
		t.Fatalf("Info() unexpected error: %v", err)
	}
	if lk != nil {
		t.Errorf("Info() for unlocked node = %v, want nil", lk)
	}
}

// TestPersistentManager_IsLocked_ExpiredLock verifies expired locks are not considered locked.
func TestPersistentManager_IsLocked_ExpiredLock(t *testing.T) {
	l, _ := createTestLedger(t)
	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")

	// Acquire lock with very short timeout
	_, err = pm.Acquire(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Expired lock should not be considered locked
	if pm.IsLocked(nodeID) {
		t.Error("IsLocked() = true for expired lock, want false")
	}
}

// TestPersistentManager_ReapExpired_PersistsReap verifies reaping persists to ledger.
func TestPersistentManager_ReapExpired_PersistsReap(t *testing.T) {
	l, _ := createTestLedger(t)
	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")

	// Acquire lock with very short timeout
	_, err = pm.Acquire(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Reap expired locks
	reaped, err := pm.ReapExpired()
	if err != nil {
		t.Fatalf("ReapExpired() unexpected error: %v", err)
	}

	if len(reaped) != 1 {
		t.Errorf("ReapExpired() returned %d locks, want 1", len(reaped))
	}

	// Verify reap is recorded in ledger (should have 2 events: acquire + reap)
	count, err := l.Count()
	if err != nil {
		t.Fatalf("Count() unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("Ledger has %d events, want 2", count)
	}
}

// TestPersistentManager_ReapExpired_SurvivesRestart verifies reaped locks stay gone after restart.
func TestPersistentManager_ReapExpired_SurvivesRestart(t *testing.T) {
	l, dir := createTestLedger(t)
	pm1, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")

	// Acquire lock with very short timeout
	_, err = pm1.Acquire(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Wait for expiration and reap
	time.Sleep(10 * time.Millisecond)
	_, err = pm1.ReapExpired()
	if err != nil {
		t.Fatalf("ReapExpired() unexpected error: %v", err)
	}

	// Simulate restart
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() for restart unexpected error: %v", err)
	}

	pm2, err := lock.NewPersistentManager(l2)
	if err != nil {
		t.Fatalf("NewPersistentManager() for restart unexpected error: %v", err)
	}

	// Lock should NOT exist (it was reaped)
	if pm2.IsLocked(nodeID) {
		t.Error("IsLocked() = true after restart, want false (lock was reaped)")
	}
}

// TestPersistentManager_ListAll_ReturnsFreshLocks verifies ListAll returns all non-expired locks.
func TestPersistentManager_ListAll_ReturnsFreshLocks(t *testing.T) {
	l, _ := createTestLedger(t)
	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	// Acquire several locks
	nodeIDs := []string{"1", "1.1", "1.2"}
	for i, nid := range nodeIDs {
		nodeID, _ := types.Parse(nid)
		_, err := pm.Acquire(nodeID, "agent-"+string(rune('A'+i)), 1*time.Hour)
		if err != nil {
			t.Fatalf("Acquire(%s) unexpected error: %v", nid, err)
		}
	}

	locks := pm.ListAll()
	if len(locks) != 3 {
		t.Errorf("ListAll() returned %d locks, want 3", len(locks))
	}
}

// TestPersistentManager_MultipleLocks_Persistence verifies multiple locks persist correctly.
func TestPersistentManager_MultipleLocks_Persistence(t *testing.T) {
	l, dir := createTestLedger(t)
	pm1, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	// Acquire several locks
	nodeID1, _ := types.Parse("1.1")
	nodeID2, _ := types.Parse("1.2")
	nodeID3, _ := types.Parse("1.3")

	_, _ = pm1.Acquire(nodeID1, "agent-001", 1*time.Hour)
	_, _ = pm1.Acquire(nodeID2, "agent-002", 1*time.Hour)
	_, _ = pm1.Acquire(nodeID3, "agent-003", 1*time.Hour)

	// Release one
	_ = pm1.Release(nodeID2, "agent-002")

	// Simulate restart
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() for restart unexpected error: %v", err)
	}

	pm2, err := lock.NewPersistentManager(l2)
	if err != nil {
		t.Fatalf("NewPersistentManager() for restart unexpected error: %v", err)
	}

	// nodeID1 and nodeID3 should still be locked
	if !pm2.IsLocked(nodeID1) {
		t.Error("nodeID1 should still be locked")
	}
	if !pm2.IsLocked(nodeID3) {
		t.Error("nodeID3 should still be locked")
	}

	// nodeID2 should be unlocked (it was released)
	if pm2.IsLocked(nodeID2) {
		t.Error("nodeID2 should be unlocked (it was released)")
	}
}

// TestPersistentManager_AcquireExpiredLock verifies expired locks can be replaced.
func TestPersistentManager_AcquireExpiredLock(t *testing.T) {
	l, _ := createTestLedger(t)
	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")

	// Acquire lock with very short timeout
	_, err = pm.Acquire(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("First Acquire() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Acquire again should succeed (previous lock expired)
	lk, err := pm.Acquire(nodeID, "agent-002", 1*time.Hour)
	if err != nil {
		t.Fatalf("Second Acquire() on expired lock unexpected error: %v", err)
	}

	if lk.Owner() != "agent-002" {
		t.Errorf("Lock Owner = %q, want %q", lk.Owner(), "agent-002")
	}
}

// TestPersistentManager_LedgerReadError verifies handling of ledger errors.
func TestPersistentManager_LedgerReadError(t *testing.T) {
	// Create ledger with valid directory, then remove it
	dir := t.TempDir()
	l, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() unexpected error: %v", err)
	}

	// Remove the directory to cause read error
	os.RemoveAll(dir)

	// Creating manager should fail because it can't read the ledger
	_, err = lock.NewPersistentManager(l)
	if err == nil {
		t.Error("NewPersistentManager() with invalid ledger expected error, got nil")
	}
}

// TestPersistentManager_UnrelatedEvents verifies unrelated events are skipped.
func TestPersistentManager_UnrelatedEvents(t *testing.T) {
	l, _ := createTestLedger(t)

	// Append an unrelated event (e.g., ProofInitialized) to the ledger
	evt := ledger.NewProofInitialized("test conjecture", "test author")
	_, err := l.Append(evt)
	if err != nil {
		t.Fatalf("Append() unexpected error: %v", err)
	}

	// Create manager - should not fail, just skip unrelated events
	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	// Manager should have no locks (unrelated event was skipped)
	locks := pm.ListAll()
	if len(locks) != 0 {
		t.Errorf("ListAll() returned %d locks, want 0 (unrelated events should be skipped)", len(locks))
	}
}

// TestPersistentManager_CorruptedLockAcquiredEvent verifies corrupted lock_acquired events cause error.
func TestPersistentManager_CorruptedLockAcquiredEvent(t *testing.T) {
	l, dir := createTestLedger(t)

	// Write a valid lock_acquired event
	nodeID, _ := types.Parse("1.1")
	evt := lock.NewLockAcquired(nodeID, "agent-001", types.Now())
	if _, err := l.Append(evt); err != nil {
		t.Fatalf("Append() unexpected error: %v", err)
	}

	// Write valid JSON that has wrong structure for lock_acquired (missing required fields)
	// The JSON is valid, but node_id is the wrong type (number instead of string)
	corruptedEvent := []byte(`{"type":"lock_acquired","node_id":123,"owner":"test"}`)
	appendCorruptedEvent(t, dir, corruptedEvent)

	// Re-open ledger and try to create manager - should fail with corruption error
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() unexpected error: %v", err)
	}

	_, err = lock.NewPersistentManager(l2)
	if err == nil {
		t.Error("NewPersistentManager() with corrupted lock_acquired event expected error, got nil")
	}

	// Verify it's a corruption error (exit code 4)
	if !aferrors.IsCorruption(err) {
		t.Errorf("Expected corruption error, got: %v", err)
	}
}

// TestPersistentManager_CorruptedLockReleasedEvent verifies corrupted lock_released events cause error.
func TestPersistentManager_CorruptedLockReleasedEvent(t *testing.T) {
	l, dir := createTestLedger(t)

	// Write a valid lock_acquired event
	nodeID, _ := types.Parse("1.1")
	evt := lock.NewLockAcquired(nodeID, "agent-001", types.Now())
	if _, err := l.Append(evt); err != nil {
		t.Fatalf("Append() unexpected error: %v", err)
	}

	// Write valid JSON with wrong structure for lock_released (node_id is number instead of string)
	corruptedEvent := []byte(`{"type":"lock_released","node_id":123,"owner":"test"}`)
	appendCorruptedEvent(t, dir, corruptedEvent)

	// Re-open ledger and try to create manager - should fail with corruption error
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() unexpected error: %v", err)
	}

	_, err = lock.NewPersistentManager(l2)
	if err == nil {
		t.Error("NewPersistentManager() with corrupted lock_released event expected error, got nil")
	}

	// Verify it's a corruption error
	if !aferrors.IsCorruption(err) {
		t.Errorf("Expected corruption error, got: %v", err)
	}
}

// TestPersistentManager_CorruptedLockReapedEvent verifies corrupted lock_reaped events cause error.
func TestPersistentManager_CorruptedLockReapedEvent(t *testing.T) {
	l, dir := createTestLedger(t)

	// Write a valid lock_acquired event
	nodeID, _ := types.Parse("1.1")
	evt := lock.NewLockAcquired(nodeID, "agent-001", types.Now())
	if _, err := l.Append(evt); err != nil {
		t.Fatalf("Append() unexpected error: %v", err)
	}

	// Write valid JSON with wrong structure for lock_reaped (node_id is object instead of string)
	corruptedEvent := []byte(`{"type":"lock_reaped","node_id":{"invalid":"object"},"owner":"test"}`)
	appendCorruptedEvent(t, dir, corruptedEvent)

	// Re-open ledger and try to create manager - should fail with corruption error
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() unexpected error: %v", err)
	}

	_, err = lock.NewPersistentManager(l2)
	if err == nil {
		t.Error("NewPersistentManager() with corrupted lock_reaped event expected error, got nil")
	}

	// Verify it's a corruption error
	if !aferrors.IsCorruption(err) {
		t.Errorf("Expected corruption error, got: %v", err)
	}
}

// TestPersistentManager_MalformedLockAcquiredArrayField verifies lock_acquired with array node_id causes error.
func TestPersistentManager_MalformedLockAcquiredArrayField(t *testing.T) {
	l, dir := createTestLedger(t)

	// Write a valid event first
	proofEvt := ledger.NewProofInitialized("test conjecture", "test author")
	if _, err := l.Append(proofEvt); err != nil {
		t.Fatalf("Append() unexpected error: %v", err)
	}

	// Write valid JSON but with node_id as an array (wrong type)
	// This is valid JSON but will fail to unmarshal into LockAcquired struct
	corruptedEvent := []byte(`{"type":"lock_acquired","node_id":[1,2,3],"owner":"test","expires_at":"2024-01-01T00:00:00Z"}`)
	appendCorruptedEvent(t, dir, corruptedEvent)

	// Re-open ledger and try to create manager - should fail
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() unexpected error: %v", err)
	}

	_, err = lock.NewPersistentManager(l2)
	if err == nil {
		t.Error("NewPersistentManager() with malformed lock_acquired event expected error, got nil")
	}

	// Verify it's a corruption error
	if !aferrors.IsCorruption(err) {
		t.Errorf("Expected corruption error, got: %v", err)
	}
}

// TestPersistentManager_NonLockEventIgnored verifies non-lock events are silently ignored (no error).
func TestPersistentManager_NonLockEventIgnored(t *testing.T) {
	l, dir := createTestLedger(t)

	// Write a valid lock event with expiration time 1 hour in the future
	nodeID, _ := types.Parse("1.1")
	futureExpiry := types.FromTime(time.Now().Add(1 * time.Hour))
	evt := lock.NewLockAcquired(nodeID, "agent-001", futureExpiry)
	if _, err := l.Append(evt); err != nil {
		t.Fatalf("Append() unexpected error: %v", err)
	}

	// Write a non-lock event with valid JSON (some custom event type)
	nonLockEvent := []byte(`{"type":"custom_event","data":"something"}`)
	appendCorruptedEvent(t, dir, nonLockEvent)

	// Re-open ledger - should succeed because non-lock events are ignored
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() unexpected error: %v", err)
	}

	pm, err := lock.NewPersistentManager(l2)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error for non-lock event: %v", err)
	}

	// Lock should still be present
	if !pm.IsLocked(nodeID) {
		t.Error("Lock should still exist when non-lock events are present")
	}
}

// TestPersistentManager_MultipleCorruptedEventsReported verifies multiple corruptions are reported.
func TestPersistentManager_MultipleCorruptedEventsReported(t *testing.T) {
	_, dir := createTestLedger(t)

	// Write multiple corrupted lock events (valid JSON but wrong field types)
	corruptedEvent1 := []byte(`{"type":"lock_acquired","node_id":123,"owner":"test"}`)
	corruptedEvent2 := []byte(`{"type":"lock_released","node_id":456,"owner":"test"}`)
	corruptedEvent3 := []byte(`{"type":"lock_reaped","node_id":789,"owner":"test"}`)

	appendCorruptedEvent(t, dir, corruptedEvent1)
	appendCorruptedEvent(t, dir, corruptedEvent2)
	appendCorruptedEvent(t, dir, corruptedEvent3)

	// Re-open ledger
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() unexpected error: %v", err)
	}

	_, err = lock.NewPersistentManager(l2)
	if err == nil {
		t.Error("NewPersistentManager() with multiple corrupted events expected error, got nil")
	}

	// Error message should mention "3 lock event(s)"
	errMsg := err.Error()
	if !containsSubstr(errMsg, "3 lock event(s)") {
		t.Errorf("Error message should mention '3 lock event(s)', got: %s", errMsg)
	}
}

// appendCorruptedEvent creates a corrupted event file directly in the ledger directory.
// It finds the next sequence number and writes a corrupted JSON file.
func appendCorruptedEvent(t *testing.T, dir string, data []byte) {
	t.Helper()

	// Find next sequence number by counting existing event files
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read ledger directory: %v", err)
	}

	maxSeq := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Parse filenames like "000001.json"
		if len(name) > 5 && name[len(name)-5:] == ".json" {
			base := name[:len(name)-5]
			var seq int
			if _, err := fmt.Sscanf(base, "%d", &seq); err == nil {
				if seq > maxSeq {
					maxSeq = seq
				}
			}
		}
	}

	nextSeq := maxSeq + 1
	filename := fmt.Sprintf("%06d.json", nextSeq)
	filepath := dir + "/" + filename

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		t.Fatalf("Failed to write corrupted event file: %v", err)
	}
}

// containsSubstr checks if s contains substr.
func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestPersistentManager_Acquire_DetectsConflictFromOtherProcess tests that when
// another process has already acquired a lock (simulated by directly writing to
// the ledger), our acquire fails with a conflict error.
func TestPersistentManager_Acquire_DetectsConflictFromOtherProcess(t *testing.T) {
	l, dir := createTestLedger(t)

	// Simulate another process acquiring the lock by writing directly to ledger
	nodeID, _ := types.Parse("1.1")
	otherOwner := "other-process"
	futureExpiry := types.FromTime(time.Now().Add(1 * time.Hour))
	otherEvt := lock.NewLockAcquired(nodeID, otherOwner, futureExpiry)
	if _, err := l.Append(otherEvt); err != nil {
		t.Fatalf("Failed to write competing lock event: %v", err)
	}

	// Create a new manager that doesn't know about the existing lock
	// (simulating a process that checked its in-memory state before the other wrote)
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() unexpected error: %v", err)
	}

	pm, err := lock.NewPersistentManager(l2)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	// Our manager replayed the ledger and sees the lock, so normal acquire should fail
	_, err = pm.Acquire(nodeID, "our-agent", 5*time.Minute)
	if err == nil {
		t.Error("Acquire() should fail when node is already locked by another process")
	}
}

// TestPersistentManager_Acquire_VerifyLockHolder tests the lock verification
// detects when a conflicting lock was written after our ledger write.
// This simulates the TOCTOU race condition.
func TestPersistentManager_Acquire_VerifyLockHolder(t *testing.T) {
	l, dir := createTestLedger(t)

	// Create manager
	pm, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	nodeID, _ := types.Parse("1.1")

	// Acquire a lock successfully
	lk, err := pm.Acquire(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}
	if lk == nil {
		t.Fatal("Acquire() returned nil lock")
	}

	// Verify the lock was recorded in the ledger
	count, err := l.Count()
	if err != nil {
		t.Fatalf("Count() unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("Ledger should have 1 event, got %d", count)
	}

	// Now simulate a second process trying to acquire
	// Create a new ledger handle and manager for the "second process"
	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() for second process unexpected error: %v", err)
	}

	pm2, err := lock.NewPersistentManager(l2)
	if err != nil {
		t.Fatalf("NewPersistentManager() for second process unexpected error: %v", err)
	}

	// Second process's acquire should fail because the lock is held
	_, err = pm2.Acquire(nodeID, "agent-002", 5*time.Minute)
	if err == nil {
		t.Error("Second Acquire() should fail when lock is held by first process")
	}
}

// TestPersistentManager_Acquire_SeparateNodes tests that acquiring locks on
// different nodes succeeds even with concurrent processes.
func TestPersistentManager_Acquire_SeparateNodes(t *testing.T) {
	l, dir := createTestLedger(t)

	// Create two managers sharing the same ledger directory (simulating two processes)
	pm1, err := lock.NewPersistentManager(l)
	if err != nil {
		t.Fatalf("NewPersistentManager() unexpected error: %v", err)
	}

	l2, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger() unexpected error: %v", err)
	}
	pm2, err := lock.NewPersistentManager(l2)
	if err != nil {
		t.Fatalf("NewPersistentManager() for second manager unexpected error: %v", err)
	}

	nodeID1, _ := types.Parse("1.1")
	nodeID2, _ := types.Parse("1.2")

	// Both should succeed on different nodes
	lk1, err := pm1.Acquire(nodeID1, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("Acquire(nodeID1) unexpected error: %v", err)
	}
	if lk1 == nil {
		t.Fatal("Acquire(nodeID1) returned nil lock")
	}

	lk2, err := pm2.Acquire(nodeID2, "agent-002", 5*time.Minute)
	if err != nil {
		t.Fatalf("Acquire(nodeID2) unexpected error: %v", err)
	}
	if lk2 == nil {
		t.Fatal("Acquire(nodeID2) returned nil lock")
	}

	// Verify both locks are recorded
	count, err := l.Count()
	if err != nil {
		t.Fatalf("Count() unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("Ledger should have 2 events, got %d", count)
	}
}
