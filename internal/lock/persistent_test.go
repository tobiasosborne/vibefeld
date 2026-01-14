package lock_test

import (
	"os"
	"testing"
	"time"

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
