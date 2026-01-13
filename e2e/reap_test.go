//go:build integration

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/lock"
	"github.com/tobias/vibefeld/internal/types"
)

// setupReapTest creates a temporary directory with locks and ledger subdirectories.
// Returns the proof directory, locks directory, ledger directory, and cleanup function.
func setupReapTest(t *testing.T) (string, string, string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-reap-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")

	if err := os.MkdirAll(locksDir, 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, locksDir, ledgerDir, cleanup
}

// createTestLockFile creates a lock file in the given directory.
func createTestLockFile(t *testing.T, locksDir string, nodeID types.NodeID, owner string, timeout time.Duration) string {
	t.Helper()

	lk, err := lock.NewLock(nodeID, owner, timeout)
	if err != nil {
		t.Fatalf("NewLock(%s, %s, %v) failed: %v", nodeID, owner, timeout, err)
	}

	data, err := json.Marshal(lk)
	if err != nil {
		t.Fatalf("Marshal lock failed: %v", err)
	}

	lockPath := filepath.Join(locksDir, nodeID.String()+".lock")
	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		t.Fatalf("WriteFile(%s) failed: %v", lockPath, err)
	}

	return lockPath
}

// createTestStaleLockFile creates a lock file that is already expired.
func createTestStaleLockFile(t *testing.T, locksDir string, nodeID types.NodeID, owner string) string {
	t.Helper()

	// Create lock with very short timeout
	lk, err := lock.NewLock(nodeID, owner, 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("NewLock(%s, %s) failed: %v", nodeID, owner, err)
	}

	// Wait for it to expire
	time.Sleep(10 * time.Millisecond)

	data, err := json.Marshal(lk)
	if err != nil {
		t.Fatalf("Marshal lock failed: %v", err)
	}

	lockPath := filepath.Join(locksDir, nodeID.String()+".lock")
	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		t.Fatalf("WriteFile(%s) failed: %v", lockPath, err)
	}

	return lockPath
}

// TestReap_FreshLocksNotReaped verifies that fresh (non-expired) locks are not reaped.
func TestReap_FreshLocksNotReaped(t *testing.T) {
	_, locksDir, ledgerDir, cleanup := setupReapTest(t)
	defer cleanup()

	// Create fresh locks with long timeout (1 hour)
	nodeID1, _ := types.Parse("1")
	nodeID2, _ := types.Parse("1.1")
	nodeID3, _ := types.Parse("1.2")

	path1 := createTestLockFile(t, locksDir, nodeID1, "agent-alpha", 1*time.Hour)
	path2 := createTestLockFile(t, locksDir, nodeID2, "agent-beta", 1*time.Hour)
	path3 := createTestLockFile(t, locksDir, nodeID3, "agent-gamma", 1*time.Hour)

	// Create ledger
	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Attempt to reap
	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// Verify no locks were reaped
	if len(reaped) != 0 {
		t.Errorf("Expected 0 reaped locks, got %d", len(reaped))
	}

	// Verify all lock files still exist
	for _, path := range []string{path1, path2, path3} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Fresh lock file should still exist: %s", path)
		}
	}

	// Verify no events were generated
	count, err := lg.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 events, got %d", count)
	}

	t.Log("Fresh locks correctly preserved during reap operation")
}

// TestReap_StaleLocksReaped verifies that stale (expired) locks are reaped.
func TestReap_StaleLocksReaped(t *testing.T) {
	_, locksDir, ledgerDir, cleanup := setupReapTest(t)
	defer cleanup()

	// Create stale locks (expired)
	nodeID1, _ := types.Parse("1")
	nodeID2, _ := types.Parse("1.1")

	stalePath1 := createTestStaleLockFile(t, locksDir, nodeID1, "agent-stale-1")
	stalePath2 := createTestStaleLockFile(t, locksDir, nodeID2, "agent-stale-2")

	// Create ledger
	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Reap stale locks
	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// Verify both stale locks were reaped
	if len(reaped) != 2 {
		t.Errorf("Expected 2 reaped locks, got %d", len(reaped))
	}

	// Verify lock files were removed
	if _, err := os.Stat(stalePath1); !os.IsNotExist(err) {
		t.Error("Stale lock file 1 should have been removed")
	}
	if _, err := os.Stat(stalePath2); !os.IsNotExist(err) {
		t.Error("Stale lock file 2 should have been removed")
	}

	// Verify LockReaped events were generated
	count, err := lg.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 LockReaped events, got %d", count)
	}

	t.Log("Stale locks correctly reaped")
}

// TestReap_ReapingAllowsNewAcquisition verifies that after reaping a stale lock,
// another agent can acquire the same node.
func TestReap_ReapingAllowsNewAcquisition(t *testing.T) {
	_, locksDir, ledgerDir, cleanup := setupReapTest(t)
	defer cleanup()

	// Create a stale lock held by agent-1
	nodeID, _ := types.Parse("1.1")
	createTestStaleLockFile(t, locksDir, nodeID, "agent-abandoned")

	// Create ledger
	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Verify the lock file exists before reap
	lockPath := filepath.Join(locksDir, nodeID.String()+".lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Fatal("Lock file should exist before reap")
	}

	// Reap stale locks
	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	if len(reaped) != 1 {
		t.Errorf("Expected 1 reaped lock, got %d", len(reaped))
	}

	// Verify lock file was removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("Stale lock file should have been removed after reap")
	}

	// Now a new agent can acquire the lock
	newLock, err := lock.NewLock(nodeID, "agent-new", 1*time.Hour)
	if err != nil {
		t.Fatalf("NewLock for new agent failed: %v", err)
	}

	// Write the new lock file
	data, err := json.Marshal(newLock)
	if err != nil {
		t.Fatalf("Marshal new lock failed: %v", err)
	}

	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		t.Fatalf("WriteFile for new lock failed: %v", err)
	}

	// Verify new lock file exists and is valid
	newData, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("ReadFile for new lock failed: %v", err)
	}

	var readLock lock.Lock
	if err := json.Unmarshal(newData, &readLock); err != nil {
		t.Fatalf("Unmarshal new lock failed: %v", err)
	}

	if readLock.Owner() != "agent-new" {
		t.Errorf("Expected owner 'agent-new', got %q", readLock.Owner())
	}
	if readLock.IsExpired() {
		t.Error("New lock should not be expired")
	}

	t.Log("Reaping stale lock successfully allowed new agent to acquire the node")
}

// TestReap_MultipleStaleLocksReaped verifies that multiple stale locks can be reaped in one operation.
func TestReap_MultipleStaleLocksReaped(t *testing.T) {
	_, locksDir, ledgerDir, cleanup := setupReapTest(t)
	defer cleanup()

	// Create multiple stale locks
	nodeIDs := []string{"1", "1.1", "1.2", "1.3", "1.1.1"}
	owners := []string{"agent-a", "agent-b", "agent-c", "agent-d", "agent-e"}

	for i, nodeIDStr := range nodeIDs {
		nodeID, _ := types.Parse(nodeIDStr)
		createTestStaleLockFile(t, locksDir, nodeID, owners[i])
	}

	// Verify all lock files exist
	files, err := os.ReadDir(locksDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 5 {
		t.Fatalf("Expected 5 lock files before reap, got %d", len(files))
	}

	// Create ledger
	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Reap all stale locks
	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// Verify all 5 locks were reaped
	if len(reaped) != 5 {
		t.Errorf("Expected 5 reaped locks, got %d", len(reaped))
	}

	// Verify no lock files remain
	files, err = os.ReadDir(locksDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	lockCount := 0
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".lock" {
			lockCount++
		}
	}
	if lockCount != 0 {
		t.Errorf("Expected 0 lock files after reap, got %d", lockCount)
	}

	// Verify 5 LockReaped events were generated
	count, err := lg.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 events, got %d", count)
	}

	// Verify each event has correct type
	events, err := lg.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	for i, eventData := range events {
		var event map[string]interface{}
		if err := json.Unmarshal(eventData, &event); err != nil {
			t.Fatalf("Unmarshal event %d failed: %v", i, err)
		}
		if event["type"] != string(ledger.EventLockReaped) {
			t.Errorf("Event %d type = %v, want %q", i, event["type"], ledger.EventLockReaped)
		}
	}

	t.Logf("Successfully reaped all %d stale locks", len(reaped))
}

// TestReap_AtomicBehavior verifies that lock reaping is atomic - each lock is either
// fully reaped (file removed + event generated) or not touched at all.
func TestReap_AtomicBehavior(t *testing.T) {
	_, locksDir, ledgerDir, cleanup := setupReapTest(t)
	defer cleanup()

	// Create a mix of fresh and stale locks
	freshID, _ := types.Parse("1")
	staleID1, _ := types.Parse("1.1")
	staleID2, _ := types.Parse("1.2")

	freshPath := createTestLockFile(t, locksDir, freshID, "agent-fresh", 1*time.Hour)
	stalePath1 := createTestStaleLockFile(t, locksDir, staleID1, "agent-stale-1")
	stalePath2 := createTestStaleLockFile(t, locksDir, staleID2, "agent-stale-2")

	// Create ledger
	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Reap stale locks
	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// Verify only stale locks were reaped
	if len(reaped) != 2 {
		t.Errorf("Expected 2 reaped locks, got %d", len(reaped))
	}

	// Verify fresh lock file still exists (atomic: not touched)
	if _, err := os.Stat(freshPath); os.IsNotExist(err) {
		t.Error("Fresh lock file should not have been touched")
	}

	// Verify stale lock files were removed (atomic: both file and event)
	if _, err := os.Stat(stalePath1); !os.IsNotExist(err) {
		t.Error("Stale lock file 1 should have been removed")
	}
	if _, err := os.Stat(stalePath2); !os.IsNotExist(err) {
		t.Error("Stale lock file 2 should have been removed")
	}

	// Verify events match reaped files
	events, err := lg.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events (matching reaped files), got %d", len(events))
	}

	// Verify returned paths match what was actually removed
	reapedSet := make(map[string]bool)
	for _, p := range reaped {
		reapedSet[p] = true
	}

	if !reapedSet[stalePath1] {
		t.Errorf("Expected %s in reaped list", stalePath1)
	}
	if !reapedSet[stalePath2] {
		t.Errorf("Expected %s in reaped list", stalePath2)
	}
	if reapedSet[freshPath] {
		t.Error("Fresh lock path should not be in reaped list")
	}

	t.Log("Reap operation exhibited correct atomic behavior")
}

// TestReap_ConcurrentReapSafety verifies that concurrent reap operations are safe.
func TestReap_ConcurrentReapSafety(t *testing.T) {
	_, locksDir, ledgerDir, cleanup := setupReapTest(t)
	defer cleanup()

	// Create stale locks
	for i := 1; i <= 10; i++ {
		nodeIDStr := "1." + string(rune('0'+i%10))
		if i > 10 {
			nodeIDStr = "1.1." + string(rune('0'+i%10))
		}
		nodeID, err := types.Parse(nodeIDStr)
		if err != nil {
			// Handle multi-digit by using different approach
			nodeID, _ = types.Parse("1")
		}
		createTestStaleLockFile(t, locksDir, nodeID, "agent-concurrent-"+string(rune('A'+i)))
	}

	// Create ledger
	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Run multiple concurrent reap operations
	var wg sync.WaitGroup
	numGoroutines := 5
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := lock.ReapStaleLocks(locksDir, lg)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent reap error: %v", err)
	}

	// Verify no lock files remain
	files, err := os.ReadDir(locksDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	lockCount := 0
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".lock" {
			lockCount++
		}
	}
	if lockCount != 0 {
		t.Errorf("Expected 0 lock files after concurrent reaps, got %d", lockCount)
	}

	t.Log("Concurrent reap operations completed safely")
}

// TestReap_EventsContainCorrectMetadata verifies that LockReaped events contain
// correct node ID and owner information.
func TestReap_EventsContainCorrectMetadata(t *testing.T) {
	_, locksDir, ledgerDir, cleanup := setupReapTest(t)
	defer cleanup()

	// Create a stale lock with specific metadata
	nodeID, _ := types.Parse("1.2.3")
	owner := "prover-special-agent"
	createTestStaleLockFile(t, locksDir, nodeID, owner)

	// Create ledger
	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Reap
	_, err = lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// Read and verify event
	events, err := lg.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	var event map[string]interface{}
	if err := json.Unmarshal(events[0], &event); err != nil {
		t.Fatalf("Unmarshal event failed: %v", err)
	}

	// Verify event type
	if event["type"] != string(ledger.EventLockReaped) {
		t.Errorf("Event type = %v, want %q", event["type"], ledger.EventLockReaped)
	}

	// Verify node ID
	if event["node_id"] != "1.2.3" {
		t.Errorf("Event node_id = %v, want %q", event["node_id"], "1.2.3")
	}

	// Verify owner
	if event["owner"] != owner {
		t.Errorf("Event owner = %v, want %q", event["owner"], owner)
	}

	// Verify timestamp exists
	if _, ok := event["timestamp"]; !ok {
		t.Error("Event missing timestamp field")
	}

	t.Log("LockReaped event contains correct metadata")
}

// TestReap_MixedFreshAndStale verifies that only stale locks are reaped
// when both fresh and stale locks exist.
func TestReap_MixedFreshAndStale(t *testing.T) {
	_, locksDir, ledgerDir, cleanup := setupReapTest(t)
	defer cleanup()

	// Create mix: 2 fresh, 3 stale
	freshID1, _ := types.Parse("1")
	freshID2, _ := types.Parse("1.1")
	staleID1, _ := types.Parse("1.2")
	staleID2, _ := types.Parse("1.3")
	staleID3, _ := types.Parse("1.4")

	freshPath1 := createTestLockFile(t, locksDir, freshID1, "fresh-agent-1", 1*time.Hour)
	freshPath2 := createTestLockFile(t, locksDir, freshID2, "fresh-agent-2", 1*time.Hour)
	createTestStaleLockFile(t, locksDir, staleID1, "stale-agent-1")
	createTestStaleLockFile(t, locksDir, staleID2, "stale-agent-2")
	createTestStaleLockFile(t, locksDir, staleID3, "stale-agent-3")

	// Create ledger
	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Reap
	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// Verify only 3 stale locks were reaped
	if len(reaped) != 3 {
		t.Errorf("Expected 3 reaped locks, got %d", len(reaped))
	}

	// Verify fresh locks still exist
	if _, err := os.Stat(freshPath1); os.IsNotExist(err) {
		t.Error("Fresh lock 1 should still exist")
	}
	if _, err := os.Stat(freshPath2); os.IsNotExist(err) {
		t.Error("Fresh lock 2 should still exist")
	}

	// Verify remaining files count
	files, err := os.ReadDir(locksDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	lockCount := 0
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".lock" {
			lockCount++
		}
	}
	if lockCount != 2 {
		t.Errorf("Expected 2 lock files remaining, got %d", lockCount)
	}

	// Verify 3 events generated
	count, err := lg.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 events, got %d", count)
	}

	t.Log("Mixed reap correctly preserved fresh locks and removed stale locks")
}

// TestReap_EmptyDirectory verifies graceful handling of empty locks directory.
func TestReap_EmptyDirectory(t *testing.T) {
	_, locksDir, ledgerDir, cleanup := setupReapTest(t)
	defer cleanup()

	// Create ledger (directory already empty - no locks)
	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Reap
	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks on empty directory failed: %v", err)
	}

	if len(reaped) != 0 {
		t.Errorf("Expected 0 reaped locks from empty directory, got %d", len(reaped))
	}

	count, err := lg.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 events, got %d", count)
	}

	t.Log("Empty directory handled gracefully")
}

// TestReap_IgnoresNonLockFiles verifies that non-.lock files are ignored during reap.
func TestReap_IgnoresNonLockFiles(t *testing.T) {
	_, locksDir, ledgerDir, cleanup := setupReapTest(t)
	defer cleanup()

	// Create non-lock files
	readmePath := filepath.Join(locksDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Locks Directory"), 0644); err != nil {
		t.Fatalf("WriteFile README failed: %v", err)
	}

	gitkeepPath := filepath.Join(locksDir, ".gitkeep")
	if err := os.WriteFile(gitkeepPath, []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile .gitkeep failed: %v", err)
	}

	// Create one stale lock to verify reaping still works
	nodeID, _ := types.Parse("1")
	createTestStaleLockFile(t, locksDir, nodeID, "agent-test")

	// Create ledger
	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Reap
	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// Verify only the lock was reaped
	if len(reaped) != 1 {
		t.Errorf("Expected 1 reaped lock, got %d", len(reaped))
	}

	// Verify non-lock files still exist
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Error("README.md should not have been removed")
	}
	if _, err := os.Stat(gitkeepPath); os.IsNotExist(err) {
		t.Error(".gitkeep should not have been removed")
	}

	t.Log("Non-lock files correctly ignored during reap")
}
