//go:build integration
// +build integration

package lock_test

import (
	"sync"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/lock"
	"github.com/tobias/vibefeld/internal/types"
)

// TestManager_Acquire_Success verifies acquiring a lock on an unlocked node succeeds.
func TestManager_Acquire_Success(t *testing.T) {
	tests := []struct {
		name    string
		nodeID  string
		owner   string
		timeout time.Duration
	}{
		{"root node", "1", "agent-001", 5 * time.Minute},
		{"child node", "1.1", "prover-alpha", 10 * time.Minute},
		{"deep node", "1.2.3.4", "verifier-beta", 1 * time.Hour},
		{"short timeout", "1.1.1", "agent-002", 1 * time.Second},
		{"long owner name", "1", "very-long-agent-identifier-with-many-characters", 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := lock.NewManager()

			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			lk, err := mgr.Acquire(nodeID, tt.owner, tt.timeout)
			if err != nil {
				t.Fatalf("Acquire() unexpected error: %v", err)
			}

			if lk == nil {
				t.Fatal("Acquire() returned nil lock")
			}

			// Verify lock properties
			if lk.NodeID().String() != tt.nodeID {
				t.Errorf("Lock NodeID = %q, want %q", lk.NodeID().String(), tt.nodeID)
			}

			if lk.Owner() != tt.owner {
				t.Errorf("Lock Owner = %q, want %q", lk.Owner(), tt.owner)
			}

			if lk.IsExpired() {
				t.Error("Fresh lock should not be expired")
			}
		})
	}
}

// TestManager_Acquire_AlreadyLocked verifies acquiring a lock on an already locked node fails.
func TestManager_Acquire_AlreadyLocked(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("types.Parse(\"1.1\") unexpected error: %v", err)
	}

	// First acquisition should succeed
	_, err = mgr.Acquire(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("First Acquire() unexpected error: %v", err)
	}

	// Second acquisition should fail (node already locked)
	_, err = mgr.Acquire(nodeID, "agent-002", 5*time.Minute)
	if err == nil {
		t.Error("Second Acquire() on locked node expected error, got nil")
	}
}

// TestManager_Acquire_SameOwnerTwiceFails verifies same owner cannot acquire twice.
func TestManager_Acquire_SameOwnerTwiceFails(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	// First acquisition should succeed
	_, err = mgr.Acquire(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("First Acquire() unexpected error: %v", err)
	}

	// Same owner trying to acquire again should fail
	_, err = mgr.Acquire(nodeID, "agent-001", 5*time.Minute)
	if err == nil {
		t.Error("Same owner Acquire() on already locked node expected error, got nil")
	}
}

// TestManager_Acquire_DifferentNodes verifies different nodes can be locked independently.
func TestManager_Acquire_DifferentNodes(t *testing.T) {
	mgr := lock.NewManager()

	nodeID1, _ := types.Parse("1.1")
	nodeID2, _ := types.Parse("1.2")
	nodeID3, _ := types.Parse("1.3")

	// All acquisitions should succeed
	_, err := mgr.Acquire(nodeID1, "agent-001", 5*time.Minute)
	if err != nil {
		t.Errorf("Acquire(1.1) unexpected error: %v", err)
	}

	_, err = mgr.Acquire(nodeID2, "agent-002", 5*time.Minute)
	if err != nil {
		t.Errorf("Acquire(1.2) unexpected error: %v", err)
	}

	_, err = mgr.Acquire(nodeID3, "agent-003", 5*time.Minute)
	if err != nil {
		t.Errorf("Acquire(1.3) unexpected error: %v", err)
	}
}

// TestManager_Acquire_EmptyOwner verifies empty owner is rejected.
func TestManager_Acquire_EmptyOwner(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1")

	_, err := mgr.Acquire(nodeID, "", 5*time.Minute)
	if err == nil {
		t.Error("Acquire() with empty owner expected error, got nil")
	}
}

// TestManager_Acquire_WhitespaceOwner verifies whitespace-only owner is rejected.
func TestManager_Acquire_WhitespaceOwner(t *testing.T) {
	tests := []struct {
		name  string
		owner string
	}{
		{"spaces only", "   "},
		{"tabs only", "\t\t"},
		{"newlines only", "\n\n"},
		{"mixed whitespace", " \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := lock.NewManager()
			nodeID, _ := types.Parse("1")

			_, err := mgr.Acquire(nodeID, tt.owner, 5*time.Minute)
			if err == nil {
				t.Errorf("Acquire() with whitespace owner %q expected error, got nil", tt.owner)
			}
		})
	}
}

// TestManager_Acquire_ZeroTimeout verifies zero timeout is rejected.
func TestManager_Acquire_ZeroTimeout(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1")

	_, err := mgr.Acquire(nodeID, "agent-001", 0)
	if err == nil {
		t.Error("Acquire() with zero timeout expected error, got nil")
	}
}

// TestManager_Acquire_NegativeTimeout verifies negative timeout is rejected.
func TestManager_Acquire_NegativeTimeout(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1")

	_, err := mgr.Acquire(nodeID, "agent-001", -5*time.Minute)
	if err == nil {
		t.Error("Acquire() with negative timeout expected error, got nil")
	}
}

// TestManager_Release_ByOwner verifies releasing a lock by its owner succeeds.
func TestManager_Release_ByOwner(t *testing.T) {
	tests := []struct {
		name   string
		nodeID string
		owner  string
	}{
		{"root node", "1", "agent-001"},
		{"child node", "1.1", "prover-alpha"},
		{"deep node", "1.2.3.4", "verifier-beta"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := lock.NewManager()

			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			// Acquire lock
			_, err = mgr.Acquire(nodeID, tt.owner, 5*time.Minute)
			if err != nil {
				t.Fatalf("Acquire() unexpected error: %v", err)
			}

			// Release should succeed
			err = mgr.Release(nodeID, tt.owner)
			if err != nil {
				t.Errorf("Release() by owner unexpected error: %v", err)
			}

			// Node should no longer be locked
			if mgr.IsLocked(nodeID) {
				t.Error("IsLocked() = true after Release, want false")
			}
		})
	}
}

// TestManager_Release_ByNonOwner verifies releasing a lock by non-owner fails.
func TestManager_Release_ByNonOwner(t *testing.T) {
	tests := []struct {
		name         string
		lockOwner    string
		releaseOwner string
	}{
		{"different owner", "agent-001", "agent-002"},
		{"similar name", "prover", "prover-1"},
		{"case sensitive", "Agent-001", "agent-001"},
		{"prefix match", "agent", "agent-001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := lock.NewManager()

			nodeID, _ := types.Parse("1")

			// Acquire lock
			_, err := mgr.Acquire(nodeID, tt.lockOwner, 5*time.Minute)
			if err != nil {
				t.Fatalf("Acquire() unexpected error: %v", err)
			}

			// Release by non-owner should fail
			err = mgr.Release(nodeID, tt.releaseOwner)
			if err == nil {
				t.Errorf("Release() by non-owner %q expected error, got nil", tt.releaseOwner)
			}

			// Node should still be locked
			if !mgr.IsLocked(nodeID) {
				t.Error("IsLocked() = false after failed Release, want true")
			}
		})
	}
}

// TestManager_Release_UnlockedNode verifies releasing an unlocked node fails.
func TestManager_Release_UnlockedNode(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1.1")

	// Release without acquire should fail
	err := mgr.Release(nodeID, "agent-001")
	if err == nil {
		t.Error("Release() on unlocked node expected error, got nil")
	}
}

// TestManager_Release_EmptyOwner verifies release with empty owner fails.
func TestManager_Release_EmptyOwner(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1")

	// Acquire lock
	_, err := mgr.Acquire(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Release with empty owner should fail
	err = mgr.Release(nodeID, "")
	if err == nil {
		t.Error("Release() with empty owner expected error, got nil")
	}
}

// TestManager_Release_AllowsReacquire verifies node can be acquired after release.
func TestManager_Release_AllowsReacquire(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1")

	// Acquire lock
	_, err := mgr.Acquire(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("First Acquire() unexpected error: %v", err)
	}

	// Release lock
	err = mgr.Release(nodeID, "agent-001")
	if err != nil {
		t.Fatalf("Release() unexpected error: %v", err)
	}

	// Re-acquire should succeed (same or different owner)
	_, err = mgr.Acquire(nodeID, "agent-002", 5*time.Minute)
	if err != nil {
		t.Errorf("Re-acquire after Release() unexpected error: %v", err)
	}
}

// TestManager_Info_LockedNode verifies Info returns lock details for locked node.
func TestManager_Info_LockedNode(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1.2")
	owner := "prover-info"
	timeout := 10 * time.Minute

	// Acquire lock
	_, err := mgr.Acquire(nodeID, owner, timeout)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Get info
	lk, err := mgr.Info(nodeID)
	if err != nil {
		t.Fatalf("Info() unexpected error: %v", err)
	}

	if lk == nil {
		t.Fatal("Info() returned nil for locked node")
	}

	// Verify lock properties
	if lk.NodeID().String() != "1.2" {
		t.Errorf("Info().NodeID = %q, want %q", lk.NodeID().String(), "1.2")
	}

	if lk.Owner() != owner {
		t.Errorf("Info().Owner = %q, want %q", lk.Owner(), owner)
	}
}

// TestManager_Info_UnlockedNode verifies Info returns nil for unlocked node.
func TestManager_Info_UnlockedNode(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1.1")

	// Info on unlocked node should return nil
	lk, err := mgr.Info(nodeID)
	if err != nil {
		t.Fatalf("Info() unexpected error: %v", err)
	}

	if lk != nil {
		t.Errorf("Info() on unlocked node = %v, want nil", lk)
	}
}

// TestManager_Info_AfterRelease verifies Info returns nil after release.
func TestManager_Info_AfterRelease(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1")

	// Acquire and release
	_, err := mgr.Acquire(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	err = mgr.Release(nodeID, "agent-001")
	if err != nil {
		t.Fatalf("Release() unexpected error: %v", err)
	}

	// Info should return nil
	lk, err := mgr.Info(nodeID)
	if err != nil {
		t.Fatalf("Info() unexpected error: %v", err)
	}

	if lk != nil {
		t.Errorf("Info() after Release = %v, want nil", lk)
	}
}

// TestManager_IsLocked_LockedNode verifies IsLocked returns true for locked node.
func TestManager_IsLocked_LockedNode(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1.1")

	// Acquire lock
	_, err := mgr.Acquire(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	if !mgr.IsLocked(nodeID) {
		t.Error("IsLocked() = false for locked node, want true")
	}
}

// TestManager_IsLocked_UnlockedNode verifies IsLocked returns false for unlocked node.
func TestManager_IsLocked_UnlockedNode(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1.2")

	if mgr.IsLocked(nodeID) {
		t.Error("IsLocked() = true for unlocked node, want false")
	}
}

// TestManager_IsLocked_AfterRelease verifies IsLocked returns false after release.
func TestManager_IsLocked_AfterRelease(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1")

	// Acquire and release
	_, _ = mgr.Acquire(nodeID, "agent-001", 5*time.Minute)
	_ = mgr.Release(nodeID, "agent-001")

	if mgr.IsLocked(nodeID) {
		t.Error("IsLocked() = true after Release, want false")
	}
}

// TestManager_IsLocked_ExpiredLock verifies IsLocked returns false for expired lock.
func TestManager_IsLocked_ExpiredLock(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1.1")

	// Acquire lock with very short timeout
	_, err := mgr.Acquire(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Expired locks should not count as locked
	if mgr.IsLocked(nodeID) {
		t.Error("IsLocked() = true for expired lock, want false")
	}
}

// TestManager_ReapExpired_NoExpired verifies ReapExpired returns empty when no locks expired.
func TestManager_ReapExpired_NoExpired(t *testing.T) {
	mgr := lock.NewManager()

	// Acquire some locks with long timeout
	nodeID1, _ := types.Parse("1")
	nodeID2, _ := types.Parse("1.1")

	_, _ = mgr.Acquire(nodeID1, "agent-001", 1*time.Hour)
	_, _ = mgr.Acquire(nodeID2, "agent-002", 1*time.Hour)

	// Reap should return empty
	reaped, err := mgr.ReapExpired()
	if err != nil {
		t.Fatalf("ReapExpired() unexpected error: %v", err)
	}

	if len(reaped) != 0 {
		t.Errorf("ReapExpired() returned %d locks, want 0", len(reaped))
	}

	// Locks should still exist
	if !mgr.IsLocked(nodeID1) || !mgr.IsLocked(nodeID2) {
		t.Error("Locks should still exist after ReapExpired with no expired locks")
	}
}

// TestManager_ReapExpired_SingleExpired verifies ReapExpired removes single expired lock.
func TestManager_ReapExpired_SingleExpired(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1.1")

	// Acquire lock with very short timeout
	_, err := mgr.Acquire(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Reap should return the expired lock
	reaped, err := mgr.ReapExpired()
	if err != nil {
		t.Fatalf("ReapExpired() unexpected error: %v", err)
	}

	if len(reaped) != 1 {
		t.Errorf("ReapExpired() returned %d locks, want 1", len(reaped))
	}

	// Lock should be removed
	if mgr.IsLocked(nodeID) {
		t.Error("Lock should be removed after ReapExpired")
	}

	// Info should return nil
	lk, _ := mgr.Info(nodeID)
	if lk != nil {
		t.Error("Info() should return nil after ReapExpired")
	}
}

// TestManager_ReapExpired_MultipleExpired verifies ReapExpired removes all expired locks.
func TestManager_ReapExpired_MultipleExpired(t *testing.T) {
	mgr := lock.NewManager()

	// Acquire multiple locks with short timeout
	nodeIDs := []string{"1", "1.1", "1.2"}
	for _, nid := range nodeIDs {
		nodeID, _ := types.Parse(nid)
		_, _ = mgr.Acquire(nodeID, "agent-"+nid, 1*time.Nanosecond)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Reap should return all expired locks
	reaped, err := mgr.ReapExpired()
	if err != nil {
		t.Fatalf("ReapExpired() unexpected error: %v", err)
	}

	if len(reaped) != 3 {
		t.Errorf("ReapExpired() returned %d locks, want 3", len(reaped))
	}

	// All locks should be removed
	for _, nid := range nodeIDs {
		nodeID, _ := types.Parse(nid)
		if mgr.IsLocked(nodeID) {
			t.Errorf("Lock for %s should be removed after ReapExpired", nid)
		}
	}
}

// TestManager_ReapExpired_MixedFreshAndExpired verifies only expired locks are reaped.
func TestManager_ReapExpired_MixedFreshAndExpired(t *testing.T) {
	mgr := lock.NewManager()

	// Fresh locks (long timeout)
	freshID1, _ := types.Parse("1")
	freshID2, _ := types.Parse("1.1")
	_, _ = mgr.Acquire(freshID1, "fresh-001", 1*time.Hour)
	_, _ = mgr.Acquire(freshID2, "fresh-002", 1*time.Hour)

	// Expired locks (short timeout)
	expiredID1, _ := types.Parse("1.2")
	expiredID2, _ := types.Parse("1.3")
	_, _ = mgr.Acquire(expiredID1, "expired-001", 1*time.Nanosecond)
	_, _ = mgr.Acquire(expiredID2, "expired-002", 1*time.Nanosecond)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Reap
	reaped, err := mgr.ReapExpired()
	if err != nil {
		t.Fatalf("ReapExpired() unexpected error: %v", err)
	}

	// Only 2 expired locks should be reaped
	if len(reaped) != 2 {
		t.Errorf("ReapExpired() returned %d locks, want 2", len(reaped))
	}

	// Fresh locks should still exist
	if !mgr.IsLocked(freshID1) || !mgr.IsLocked(freshID2) {
		t.Error("Fresh locks should still exist after ReapExpired")
	}

	// Expired locks should be removed
	if mgr.IsLocked(expiredID1) || mgr.IsLocked(expiredID2) {
		t.Error("Expired locks should be removed after ReapExpired")
	}
}

// TestManager_ReapExpired_ReapedLockContainsInfo verifies reaped locks contain original info.
func TestManager_ReapExpired_ReapedLockContainsInfo(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1.2.3")
	owner := "reap-test-agent"

	_, err := mgr.Acquire(nodeID, owner, 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Reap
	reaped, err := mgr.ReapExpired()
	if err != nil {
		t.Fatalf("ReapExpired() unexpected error: %v", err)
	}

	if len(reaped) != 1 {
		t.Fatalf("ReapExpired() returned %d locks, want 1", len(reaped))
	}

	// Verify reaped lock contains correct info
	lk := reaped[0]
	if lk.NodeID().String() != "1.2.3" {
		t.Errorf("Reaped lock NodeID = %q, want %q", lk.NodeID().String(), "1.2.3")
	}

	if lk.Owner() != owner {
		t.Errorf("Reaped lock Owner = %q, want %q", lk.Owner(), owner)
	}
}

// TestManager_ReapExpired_AllowsReacquire verifies node can be acquired after reaping.
func TestManager_ReapExpired_AllowsReacquire(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1")

	// Acquire with short timeout
	_, _ = mgr.Acquire(nodeID, "agent-001", 1*time.Nanosecond)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Reap
	_, _ = mgr.ReapExpired()

	// Re-acquire should succeed
	_, err := mgr.Acquire(nodeID, "agent-002", 5*time.Minute)
	if err != nil {
		t.Errorf("Acquire after ReapExpired() unexpected error: %v", err)
	}
}

// TestManager_ListAll_Empty verifies ListAll returns empty slice when no locks.
func TestManager_ListAll_Empty(t *testing.T) {
	mgr := lock.NewManager()

	locks := mgr.ListAll()
	if len(locks) != 0 {
		t.Errorf("ListAll() returned %d locks, want 0", len(locks))
	}
}

// TestManager_ListAll_SingleLock verifies ListAll returns single lock.
func TestManager_ListAll_SingleLock(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1")
	_, _ = mgr.Acquire(nodeID, "agent-001", 5*time.Minute)

	locks := mgr.ListAll()
	if len(locks) != 1 {
		t.Errorf("ListAll() returned %d locks, want 1", len(locks))
	}
}

// TestManager_ListAll_MultipleLocks verifies ListAll returns all locks.
func TestManager_ListAll_MultipleLocks(t *testing.T) {
	mgr := lock.NewManager()

	nodeIDs := []string{"1", "1.1", "1.2", "1.1.1"}
	for i, nid := range nodeIDs {
		nodeID, _ := types.Parse(nid)
		_, _ = mgr.Acquire(nodeID, "agent-"+string(rune('A'+i)), 5*time.Minute)
	}

	locks := mgr.ListAll()
	if len(locks) != 4 {
		t.Errorf("ListAll() returned %d locks, want 4", len(locks))
	}
}

// TestManager_ListAll_ExcludesExpired verifies ListAll excludes expired locks.
func TestManager_ListAll_ExcludesExpired(t *testing.T) {
	mgr := lock.NewManager()

	// Fresh lock
	freshID, _ := types.Parse("1")
	_, _ = mgr.Acquire(freshID, "fresh-agent", 1*time.Hour)

	// Expired lock
	expiredID, _ := types.Parse("1.1")
	_, _ = mgr.Acquire(expiredID, "expired-agent", 1*time.Nanosecond)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// ListAll should only return fresh locks
	locks := mgr.ListAll()
	if len(locks) != 1 {
		t.Errorf("ListAll() returned %d locks, want 1 (fresh only)", len(locks))
	}

	if len(locks) > 0 && locks[0].NodeID().String() != "1" {
		t.Errorf("ListAll() should return fresh lock, got node %s", locks[0].NodeID().String())
	}
}

// TestManager_ListAll_AfterRelease verifies ListAll excludes released locks.
func TestManager_ListAll_AfterRelease(t *testing.T) {
	mgr := lock.NewManager()

	nodeID1, _ := types.Parse("1")
	nodeID2, _ := types.Parse("1.1")

	_, _ = mgr.Acquire(nodeID1, "agent-001", 5*time.Minute)
	_, _ = mgr.Acquire(nodeID2, "agent-002", 5*time.Minute)

	// Release one
	_ = mgr.Release(nodeID1, "agent-001")

	locks := mgr.ListAll()
	if len(locks) != 1 {
		t.Errorf("ListAll() returned %d locks after release, want 1", len(locks))
	}
}

// TestManager_ListAll_ContainsCorrectInfo verifies ListAll returns locks with correct info.
func TestManager_ListAll_ContainsCorrectInfo(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1.2.3")
	owner := "list-test-agent"

	_, _ = mgr.Acquire(nodeID, owner, 5*time.Minute)

	locks := mgr.ListAll()
	if len(locks) != 1 {
		t.Fatalf("ListAll() returned %d locks, want 1", len(locks))
	}

	lk := locks[0]
	if lk.NodeID().String() != "1.2.3" {
		t.Errorf("ListAll()[0].NodeID = %q, want %q", lk.NodeID().String(), "1.2.3")
	}

	if lk.Owner() != owner {
		t.Errorf("ListAll()[0].Owner = %q, want %q", lk.Owner(), owner)
	}
}

// TestManager_ConcurrentAcquire verifies concurrent acquire operations are safe.
func TestManager_ConcurrentAcquire(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1")

	// Multiple goroutines trying to acquire same lock
	results := make(chan error, 10)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := mgr.Acquire(nodeID, "agent-"+string(rune('A'+i)), 5*time.Minute)
			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	// Exactly one should succeed
	var successCount, errorCount int
	for err := range results {
		if err == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful acquire, got %d", successCount)
	}

	if errorCount != 9 {
		t.Errorf("Expected exactly 9 failed acquires, got %d", errorCount)
	}
}

// TestManager_ConcurrentRelease verifies concurrent release operations are safe.
func TestManager_ConcurrentRelease(t *testing.T) {
	mgr := lock.NewManager()

	nodeID, _ := types.Parse("1")
	owner := "agent-001"

	_, err := mgr.Acquire(nodeID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	// Multiple goroutines trying to release
	results := make(chan error, 10)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- mgr.Release(nodeID, owner)
		}()
	}

	wg.Wait()
	close(results)

	// Exactly one should succeed (first to release)
	var successCount int
	for err := range results {
		if err == nil {
			successCount++
		}
	}

	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful release, got %d", successCount)
	}
}

// TestManager_ConcurrentMixedOperations verifies mixed concurrent operations are safe.
func TestManager_ConcurrentMixedOperations(t *testing.T) {
	mgr := lock.NewManager()

	nodeIDs := make([]types.NodeID, 5)
	for i := 0; i < 5; i++ {
		nodeID, _ := types.Parse("1." + string(rune('1'+i)))
		nodeIDs[i] = nodeID
	}

	var wg sync.WaitGroup

	// Concurrent acquires
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = mgr.Acquire(nodeIDs[i], "agent-"+string(rune('A'+i)), 5*time.Minute)
		}(i)
	}

	// Concurrent info reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = mgr.Info(nodeIDs[i])
		}(i)
	}

	// Concurrent IsLocked checks
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = mgr.IsLocked(nodeIDs[i])
		}(i)
	}

	// Concurrent ListAll
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.ListAll()
		}()
	}

	wg.Wait()

	// Should not panic; result state is implementation-defined but consistent
}

// TestManager_TableDriven_AcquireRelease provides comprehensive table-driven tests.
func TestManager_TableDriven_AcquireRelease(t *testing.T) {
	tests := []struct {
		name           string
		nodeID         string
		acquireOwner   string
		releaseOwner   string
		timeout        time.Duration
		wantAcquireErr bool
		wantReleaseErr bool
	}{
		{
			name:           "valid acquire and release",
			nodeID:         "1",
			acquireOwner:   "agent-001",
			releaseOwner:   "agent-001",
			timeout:        5 * time.Minute,
			wantAcquireErr: false,
			wantReleaseErr: false,
		},
		{
			name:           "release by different owner",
			nodeID:         "1.1",
			acquireOwner:   "agent-001",
			releaseOwner:   "agent-002",
			timeout:        5 * time.Minute,
			wantAcquireErr: false,
			wantReleaseErr: true,
		},
		{
			name:           "empty acquire owner",
			nodeID:         "1",
			acquireOwner:   "",
			releaseOwner:   "",
			timeout:        5 * time.Minute,
			wantAcquireErr: true,
			wantReleaseErr: true, // Won't reach release if acquire fails
		},
		{
			name:           "zero timeout",
			nodeID:         "1.2",
			acquireOwner:   "agent-001",
			releaseOwner:   "agent-001",
			timeout:        0,
			wantAcquireErr: true,
			wantReleaseErr: true, // Won't reach release if acquire fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := lock.NewManager()

			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			// Acquire
			_, err = mgr.Acquire(nodeID, tt.acquireOwner, tt.timeout)
			if (err != nil) != tt.wantAcquireErr {
				t.Errorf("Acquire() error = %v, wantErr = %v", err, tt.wantAcquireErr)
			}

			// Only test release if acquire was expected to succeed
			if !tt.wantAcquireErr {
				err = mgr.Release(nodeID, tt.releaseOwner)
				if (err != nil) != tt.wantReleaseErr {
					t.Errorf("Release() error = %v, wantErr = %v", err, tt.wantReleaseErr)
				}
			}
		})
	}
}

// TestManager_NewManager verifies NewManager creates a valid manager.
func TestManager_NewManager(t *testing.T) {
	mgr := lock.NewManager()
	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}

	// Fresh manager should have no locks
	locks := mgr.ListAll()
	if len(locks) != 0 {
		t.Errorf("Fresh manager has %d locks, want 0", len(locks))
	}
}

// TestManager_MultipleManagers verifies multiple manager instances are independent.
func TestManager_MultipleManagers(t *testing.T) {
	mgr1 := lock.NewManager()
	mgr2 := lock.NewManager()

	nodeID, _ := types.Parse("1")

	// Acquire on mgr1
	_, err := mgr1.Acquire(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("mgr1.Acquire() unexpected error: %v", err)
	}

	// mgr2 should be independent - same node should be acquirable
	_, err = mgr2.Acquire(nodeID, "agent-002", 5*time.Minute)
	if err != nil {
		t.Errorf("mgr2.Acquire() unexpected error: %v (managers should be independent)", err)
	}

	// Verify independence
	if !mgr1.IsLocked(nodeID) {
		t.Error("mgr1.IsLocked() = false, want true")
	}

	if !mgr2.IsLocked(nodeID) {
		t.Error("mgr2.IsLocked() = false, want true")
	}

	// ListAll should be independent
	if len(mgr1.ListAll()) != 1 {
		t.Errorf("mgr1.ListAll() has %d locks, want 1", len(mgr1.ListAll()))
	}

	if len(mgr2.ListAll()) != 1 {
		t.Errorf("mgr2.ListAll() has %d locks, want 1", len(mgr2.ListAll()))
	}
}

// TestManager_ZeroValueNodeID verifies handling of zero-value NodeID.
func TestManager_ZeroValueNodeID(t *testing.T) {
	mgr := lock.NewManager()

	var zeroNodeID types.NodeID // zero value

	// Operations with zero-value NodeID should be handled gracefully
	// (either error or defined behavior, but no panic)

	_, err := mgr.Acquire(zeroNodeID, "agent-001", 5*time.Minute)
	if err == nil {
		// If acquire succeeded, subsequent operations should work
		t.Log("Acquire with zero NodeID succeeded (implementation allows it)")
	} else {
		// If acquire failed, that's also valid
		t.Log("Acquire with zero NodeID failed as expected:", err)
	}

	// These should not panic
	_ = mgr.IsLocked(zeroNodeID)
	_, _ = mgr.Info(zeroNodeID)
}
