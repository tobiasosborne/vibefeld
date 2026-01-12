//go:build integration
// +build integration

// These tests define expected behavior for stale lock reaping.
// Lock reaping finds and removes stale locks, generating LockReaped events.
// Run with: go test -tags=integration ./internal/lock/...

package lock_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/lock"
	"github.com/tobias/vibefeld/internal/types"
)

// createLockFile creates a lock file in the given directory.
// Returns the path to the created lock file.
func createLockFile(t *testing.T, locksDir string, nodeID types.NodeID, owner string, timeout time.Duration) string {
	t.Helper()

	lk, err := lock.NewLock(nodeID, owner, timeout)
	if err != nil {
		t.Fatalf("NewLock(%s, %s, %v) failed: %v", nodeID, owner, timeout, err)
	}

	data, err := json.Marshal(lk)
	if err != nil {
		t.Fatalf("Marshal lock failed: %v", err)
	}

	// Lock files are named after the node ID with .lock extension
	lockPath := filepath.Join(locksDir, nodeID.String()+".lock")
	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		t.Fatalf("WriteFile(%s) failed: %v", lockPath, err)
	}

	return lockPath
}

// createStaleLockFile creates a lock file that is already expired.
func createStaleLockFile(t *testing.T, locksDir string, nodeID types.NodeID, owner string) string {
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

// TestReapStaleLocks_NoStale verifies no locks are reaped when all are fresh.
func TestReapStaleLocks_NoStale(t *testing.T) {
	// Setup directories
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	// Create fresh locks (long timeout)
	nodeID1, _ := types.Parse("1")
	nodeID2, _ := types.Parse("1.1")
	createLockFile(t, locksDir, nodeID1, "agent-001", 1*time.Hour)
	createLockFile(t, locksDir, nodeID2, "agent-002", 1*time.Hour)

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

	// No locks should be reaped
	if len(reaped) != 0 {
		t.Errorf("ReapStaleLocks returned %d reaped locks, want 0", len(reaped))
	}

	// Lock files should still exist
	files, err := os.ReadDir(locksDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("Expected 2 lock files to remain, got %d", len(files))
	}

	// No events should be generated
	count, err := lg.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 events, got %d", count)
	}
}

// TestReapStaleLocks_EmptyDirectory verifies handling of empty locks directory.
func TestReapStaleLocks_EmptyDirectory(t *testing.T) {
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	if len(reaped) != 0 {
		t.Errorf("ReapStaleLocks returned %d reaped locks, want 0", len(reaped))
	}
}

// TestReapStaleLocks_SingleStale verifies single stale lock is reaped with event.
func TestReapStaleLocks_SingleStale(t *testing.T) {
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	nodeID, _ := types.Parse("1.1")
	lockPath := createStaleLockFile(t, locksDir, nodeID, "agent-001")

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// One lock should be reaped
	if len(reaped) != 1 {
		t.Errorf("ReapStaleLocks returned %d reaped locks, want 1", len(reaped))
	}

	// Lock file should be removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("Stale lock file should have been removed")
	}

	// One LockReaped event should be generated
	count, err := lg.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}

	// Verify event content
	events, err := lg.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	var eventData map[string]interface{}
	if err := json.Unmarshal(events[0], &eventData); err != nil {
		t.Fatalf("Unmarshal event failed: %v", err)
	}

	// Verify event type
	if eventData["type"] != string(ledger.EventLockReaped) {
		t.Errorf("Event type = %v, want %q", eventData["type"], ledger.EventLockReaped)
	}

	// Verify node ID in event
	if eventData["node_id"] != "1.1" {
		t.Errorf("Event node_id = %v, want %q", eventData["node_id"], "1.1")
	}
}

// TestReapStaleLocks_MultipleStale verifies multiple stale locks are all reaped.
func TestReapStaleLocks_MultipleStale(t *testing.T) {
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	// Create multiple stale locks
	nodeID1, _ := types.Parse("1")
	nodeID2, _ := types.Parse("1.1")
	nodeID3, _ := types.Parse("1.2")

	createStaleLockFile(t, locksDir, nodeID1, "agent-001")
	createStaleLockFile(t, locksDir, nodeID2, "agent-002")
	createStaleLockFile(t, locksDir, nodeID3, "agent-003")

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// All 3 locks should be reaped
	if len(reaped) != 3 {
		t.Errorf("ReapStaleLocks returned %d reaped locks, want 3", len(reaped))
	}

	// No lock files should remain
	files, err := os.ReadDir(locksDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 lock files to remain, got %d", len(files))
	}

	// 3 LockReaped events should be generated
	count, err := lg.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 events, got %d", count)
	}
}

// TestReapStaleLocks_MixedFreshAndStale verifies only stale locks are reaped.
func TestReapStaleLocks_MixedFreshAndStale(t *testing.T) {
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	// Create mix of fresh and stale locks
	nodeID1, _ := types.Parse("1")
	nodeID2, _ := types.Parse("1.1")
	nodeID3, _ := types.Parse("1.2")

	freshPath := createLockFile(t, locksDir, nodeID1, "agent-001", 1*time.Hour) // Fresh
	createStaleLockFile(t, locksDir, nodeID2, "agent-002")                       // Stale
	createStaleLockFile(t, locksDir, nodeID3, "agent-003")                       // Stale

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// Only 2 stale locks should be reaped
	if len(reaped) != 2 {
		t.Errorf("ReapStaleLocks returned %d reaped locks, want 2", len(reaped))
	}

	// Fresh lock file should still exist
	if _, err := os.Stat(freshPath); os.IsNotExist(err) {
		t.Error("Fresh lock file should still exist")
	}

	// Only 1 lock file should remain
	files, err := os.ReadDir(locksDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 lock file to remain, got %d", len(files))
	}

	// 2 LockReaped events should be generated
	count, err := lg.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 events, got %d", count)
	}
}

// TestReapStaleLocks_DirectoryNotExist verifies error on non-existent directory.
func TestReapStaleLocks_DirectoryNotExist(t *testing.T) {
	proofDir := t.TempDir()
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Try to reap from non-existent directory
	_, err = lock.ReapStaleLocks("/nonexistent/path/to/locks", lg)
	if err == nil {
		t.Error("ReapStaleLocks should return error for non-existent directory")
	}
}

// TestReapStaleLocks_EventContainsNodeID verifies LockReaped event has correct node ID.
func TestReapStaleLocks_EventContainsNodeID(t *testing.T) {
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	// Create stale lock with specific node ID
	nodeID, _ := types.Parse("1.2.3.4")
	createStaleLockFile(t, locksDir, nodeID, "agent-deep")

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	_, err = lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// Verify event has correct node ID
	events, err := lg.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	var eventData map[string]interface{}
	if err := json.Unmarshal(events[0], &eventData); err != nil {
		t.Fatalf("Unmarshal event failed: %v", err)
	}

	if eventData["node_id"] != "1.2.3.4" {
		t.Errorf("Event node_id = %v, want %q", eventData["node_id"], "1.2.3.4")
	}
}

// TestReapStaleLocks_EventContainsOwner verifies LockReaped event has owner info.
func TestReapStaleLocks_EventContainsOwner(t *testing.T) {
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	nodeID, _ := types.Parse("1")
	createStaleLockFile(t, locksDir, nodeID, "prover-alpha-007")

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	_, err = lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	events, err := lg.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	var eventData map[string]interface{}
	if err := json.Unmarshal(events[0], &eventData); err != nil {
		t.Fatalf("Unmarshal event failed: %v", err)
	}

	// Verify owner is included in event
	if eventData["owner"] != "prover-alpha-007" {
		t.Errorf("Event owner = %v, want %q", eventData["owner"], "prover-alpha-007")
	}
}

// TestReapStaleLocks_ReturnedPathsMatch verifies returned paths match reaped files.
func TestReapStaleLocks_ReturnedPathsMatch(t *testing.T) {
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	nodeID1, _ := types.Parse("1.1")
	nodeID2, _ := types.Parse("1.2")

	path1 := createStaleLockFile(t, locksDir, nodeID1, "agent-001")
	path2 := createStaleLockFile(t, locksDir, nodeID2, "agent-002")

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	if len(reaped) != 2 {
		t.Fatalf("Expected 2 reaped paths, got %d", len(reaped))
	}

	// Verify the returned paths match the original lock file paths
	reapedSet := make(map[string]bool)
	for _, p := range reaped {
		reapedSet[p] = true
	}

	if !reapedSet[path1] {
		t.Errorf("Expected path %s in reaped list", path1)
	}
	if !reapedSet[path2] {
		t.Errorf("Expected path %s in reaped list", path2)
	}
}

// TestReapStaleLocks_IgnoresNonLockFiles verifies non-.lock files are ignored.
func TestReapStaleLocks_IgnoresNonLockFiles(t *testing.T) {
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	// Create non-lock files that should be ignored
	otherFile := filepath.Join(locksDir, "readme.txt")
	if err := os.WriteFile(otherFile, []byte("not a lock"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	dotFile := filepath.Join(locksDir, ".gitkeep")
	if err := os.WriteFile(dotFile, []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	reaped, err := lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}

	// No locks should be reaped (only non-lock files exist)
	if len(reaped) != 0 {
		t.Errorf("ReapStaleLocks returned %d reaped locks, want 0", len(reaped))
	}

	// Non-lock files should still exist
	if _, err := os.Stat(otherFile); os.IsNotExist(err) {
		t.Error("Non-lock file should not be removed")
	}
	if _, err := os.Stat(dotFile); os.IsNotExist(err) {
		t.Error("Dot file should not be removed")
	}
}

// TestReapStaleLocks_HandlesCorruptedLockFile verifies graceful handling of corrupted lock files.
func TestReapStaleLocks_HandlesCorruptedLockFile(t *testing.T) {
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	// Create a corrupted lock file (invalid JSON)
	corruptPath := filepath.Join(locksDir, "1.1.lock")
	if err := os.WriteFile(corruptPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Should not panic; may return error or skip corrupted file
	_, err = lock.ReapStaleLocks(locksDir, lg)
	// The implementation can either:
	// 1. Skip corrupted files (err == nil)
	// 2. Return an error explaining the issue
	// Both are acceptable behaviors
	t.Logf("ReapStaleLocks with corrupted file: err=%v", err)
}

// TestReapStaleLocks_TableDriven provides comprehensive table-driven tests.
func TestReapStaleLocks_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		freshLocks     []string // Node IDs for fresh locks
		staleLocks     []string // Node IDs for stale locks
		wantReaped     int      // Expected number of reaped locks
		wantEvents     int      // Expected number of events
		wantRemaining  int      // Expected remaining lock files
	}{
		{
			name:          "no locks",
			freshLocks:    nil,
			staleLocks:    nil,
			wantReaped:    0,
			wantEvents:    0,
			wantRemaining: 0,
		},
		{
			name:          "all fresh",
			freshLocks:    []string{"1", "1.1", "1.2"},
			staleLocks:    nil,
			wantReaped:    0,
			wantEvents:    0,
			wantRemaining: 3,
		},
		{
			name:          "all stale",
			freshLocks:    nil,
			staleLocks:    []string{"1", "1.1", "1.2", "1.3"},
			wantReaped:    4,
			wantEvents:    4,
			wantRemaining: 0,
		},
		{
			name:          "mixed",
			freshLocks:    []string{"1", "1.1"},
			staleLocks:    []string{"1.2", "1.3", "1.4"},
			wantReaped:    3,
			wantEvents:    3,
			wantRemaining: 2,
		},
		{
			name:          "single stale",
			freshLocks:    nil,
			staleLocks:    []string{"1"},
			wantReaped:    1,
			wantEvents:    1,
			wantRemaining: 0,
		},
		{
			name:          "deep node IDs",
			freshLocks:    []string{"1.1.1.1"},
			staleLocks:    []string{"1.2.3.4.5"},
			wantReaped:    1,
			wantEvents:    1,
			wantRemaining: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup directories
			proofDir := t.TempDir()
			locksDir := filepath.Join(proofDir, "locks")
			ledgerDir := filepath.Join(proofDir, "ledger")
			if err := os.MkdirAll(locksDir, 0755); err != nil {
				t.Fatalf("MkdirAll(locks) failed: %v", err)
			}
			if err := os.MkdirAll(ledgerDir, 0755); err != nil {
				t.Fatalf("MkdirAll(ledger) failed: %v", err)
			}

			// Create fresh locks
			for i, nodeIDStr := range tt.freshLocks {
				nodeID, err := types.Parse(nodeIDStr)
				if err != nil {
					t.Fatalf("Parse(%q) failed: %v", nodeIDStr, err)
				}
				createLockFile(t, locksDir, nodeID, "agent-fresh-"+string(rune('A'+i)), 1*time.Hour)
			}

			// Create stale locks
			for i, nodeIDStr := range tt.staleLocks {
				nodeID, err := types.Parse(nodeIDStr)
				if err != nil {
					t.Fatalf("Parse(%q) failed: %v", nodeIDStr, err)
				}
				createStaleLockFile(t, locksDir, nodeID, "agent-stale-"+string(rune('A'+i)))
			}

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

			// Verify reaped count
			if len(reaped) != tt.wantReaped {
				t.Errorf("ReapStaleLocks returned %d reaped, want %d", len(reaped), tt.wantReaped)
			}

			// Verify event count
			count, err := lg.Count()
			if err != nil {
				t.Fatalf("Count failed: %v", err)
			}
			if count != tt.wantEvents {
				t.Errorf("Event count = %d, want %d", count, tt.wantEvents)
			}

			// Verify remaining files
			files, err := os.ReadDir(locksDir)
			if err != nil {
				t.Fatalf("ReadDir failed: %v", err)
			}
			lockFileCount := 0
			for _, f := range files {
				if filepath.Ext(f.Name()) == ".lock" {
					lockFileCount++
				}
			}
			if lockFileCount != tt.wantRemaining {
				t.Errorf("Remaining lock files = %d, want %d", lockFileCount, tt.wantRemaining)
			}
		})
	}
}

// TestReapStaleLocks_NilLedger verifies error when ledger is nil.
func TestReapStaleLocks_NilLedger(t *testing.T) {
	locksDir := t.TempDir()

	_, err := lock.ReapStaleLocks(locksDir, nil)
	if err == nil {
		t.Error("ReapStaleLocks should return error for nil ledger")
	}
}

// TestReapStaleLocks_EmptyLocksDir verifies empty string directory path returns error.
func TestReapStaleLocks_EmptyLocksDir(t *testing.T) {
	proofDir := t.TempDir()
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	_, err = lock.ReapStaleLocks("", lg)
	if err == nil {
		t.Error("ReapStaleLocks should return error for empty directory path")
	}
}

// TestReapStaleLocks_EventHasTimestamp verifies LockReaped events have timestamps.
func TestReapStaleLocks_EventHasTimestamp(t *testing.T) {
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	nodeID, _ := types.Parse("1")
	createStaleLockFile(t, locksDir, nodeID, "agent-time")

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	beforeReap := time.Now().UTC()
	_, err = lock.ReapStaleLocks(locksDir, lg)
	if err != nil {
		t.Fatalf("ReapStaleLocks failed: %v", err)
	}
	afterReap := time.Now().UTC()

	events, err := lg.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	var eventData map[string]interface{}
	if err := json.Unmarshal(events[0], &eventData); err != nil {
		t.Fatalf("Unmarshal event failed: %v", err)
	}

	timestampStr, ok := eventData["timestamp"].(string)
	if !ok {
		t.Fatal("Event missing timestamp field")
	}

	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		t.Fatalf("Failed to parse timestamp: %v", err)
	}

	// Verify timestamp is within reap window
	if timestamp.Before(beforeReap) || timestamp.After(afterReap) {
		t.Errorf("Event timestamp %v not within reap window [%v, %v]",
			timestamp, beforeReap, afterReap)
	}
}

// TestReapStaleLocks_ConcurrentSafe verifies reaping is safe under concurrent calls.
func TestReapStaleLocks_ConcurrentSafe(t *testing.T) {
	proofDir := t.TempDir()
	locksDir := filepath.Join(proofDir, "locks")
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("MkdirAll(locks) failed: %v", err)
	}
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("MkdirAll(ledger) failed: %v", err)
	}

	// Create some stale locks
	for i := 1; i <= 5; i++ {
		nodeIDStr := "1." + string(rune('0'+i))
		nodeID, _ := types.Parse(nodeIDStr)
		createStaleLockFile(t, locksDir, nodeID, "agent-concurrent")
	}

	lg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Run multiple concurrent reaps - should not panic
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			_, _ = lock.ReapStaleLocks(locksDir, lg)
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	// After all concurrent reaps, no lock files should remain
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
}
