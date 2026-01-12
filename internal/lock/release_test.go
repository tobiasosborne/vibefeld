//go:build integration
// +build integration

// These tests define expected behavior for Lock.Release and lock.Release.
// Run with: go test -tags=integration ./internal/lock/...

package lock_test

import (
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/lock"
	"github.com/tobias/vibefeld/internal/types"
)

// TestRelease_Valid verifies releasing a lock owned by the caller succeeds
func TestRelease_Valid(t *testing.T) {
	tests := []struct {
		name    string
		nodeID  string
		owner   string
		timeout time.Duration
	}{
		{"root node release", "1", "agent-001", 5 * time.Minute},
		{"child node release", "1.1", "prover-alpha", 10 * time.Minute},
		{"deep node release", "1.2.3.4", "verifier-beta", 1 * time.Hour},
		{"short timeout lock", "1.1.1", "agent-002", 1 * time.Second},
		{"long owner name", "1", "very-long-agent-identifier-with-many-characters", 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			lk, err := lock.NewLock(nodeID, tt.owner, tt.timeout)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			// Release should succeed when called by the owner
			err = lk.Release(tt.owner)
			if err != nil {
				t.Errorf("Release(%q) unexpected error: %v", tt.owner, err)
			}
		})
	}
}

// TestRelease_NotOwner verifies releasing a lock not owned by caller fails
func TestRelease_NotOwner(t *testing.T) {
	tests := []struct {
		name         string
		lockOwner    string
		releaseOwner string
	}{
		{"completely different owner", "agent-001", "agent-002"},
		{"similar name", "prover", "prover-1"},
		{"case sensitive mismatch", "Agent-001", "agent-001"},
		{"prefix of owner", "agent", "agent-001"},
		{"suffix of owner", "001", "agent-001"},
		{"owner with extra space", "agent-001", "agent-001 "},
		{"owner with leading space", "agent-001", " agent-001"},
		{"swapped names", "alice", "bob"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse("1")
			if err != nil {
				t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
			}

			lk, err := lock.NewLock(nodeID, tt.lockOwner, 5*time.Minute)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			// Release should fail when called by non-owner
			err = lk.Release(tt.releaseOwner)
			if err == nil {
				t.Errorf("Release(%q) by non-owner expected error, got nil", tt.releaseOwner)
			}
		})
	}
}

// TestRelease_ExpiredLock verifies releasing an expired lock has defined behavior
func TestRelease_ExpiredLock(t *testing.T) {
	nodeID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("types.Parse(\"1.1\") unexpected error: %v", err)
	}

	// Create lock with very short timeout
	lk, err := lock.NewLock(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Verify lock is expired
	if !lk.IsExpired() {
		t.Fatal("Lock should be expired but IsExpired() returned false")
	}

	// Releasing an expired lock should fail with error
	// (lock has effectively been abandoned and should not be releasable)
	err = lk.Release("agent-001")
	if err == nil {
		t.Error("Release() on expired lock expected error, got nil")
	}
}

// TestRelease_ExpiredLock_NotOwner verifies expired lock rejects non-owner release
func TestRelease_ExpiredLock_NotOwner(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	// Create lock with very short timeout
	lk, err := lock.NewLock(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Release by non-owner on expired lock should also fail
	err = lk.Release("other-agent")
	if err == nil {
		t.Error("Release() by non-owner on expired lock expected error, got nil")
	}
}

// TestRelease_NilLock verifies releasing nil lock handles gracefully
func TestRelease_NilLock(t *testing.T) {
	var lk *lock.Lock = nil

	// Releasing a nil lock should not panic and should return an error
	// This uses the package-level Release function if it exists
	err := lock.Release(lk, "agent-001")
	if err == nil {
		t.Error("Release(nil, owner) expected error, got nil")
	}
}

// TestRelease_EmptyOwner verifies empty owner string fails
func TestRelease_EmptyOwner(t *testing.T) {
	tests := []struct {
		name  string
		owner string
	}{
		{"empty string", ""},
		{"spaces only", "   "},
		{"tab only", "\t"},
		{"newline only", "\n"},
		{"mixed whitespace", " \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse("1")
			if err != nil {
				t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
			}

			lk, err := lock.NewLock(nodeID, "agent-001", 5*time.Minute)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			// Release with empty/whitespace owner should fail
			err = lk.Release(tt.owner)
			if err == nil {
				t.Errorf("Release(%q) with empty/whitespace owner expected error, got nil", tt.owner)
			}
		})
	}
}

// TestRelease_IdempotentFails verifies double release fails
func TestRelease_DoubleRelease(t *testing.T) {
	nodeID, err := types.Parse("1.2")
	if err != nil {
		t.Fatalf("types.Parse(\"1.2\") unexpected error: %v", err)
	}

	lk, err := lock.NewLock(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// First release should succeed
	err = lk.Release("agent-001")
	if err != nil {
		t.Fatalf("First Release() unexpected error: %v", err)
	}

	// Second release should fail (lock is already released)
	err = lk.Release("agent-001")
	if err == nil {
		t.Error("Second Release() expected error (already released), got nil")
	}
}

// TestRelease_TableDriven provides comprehensive table-driven tests
func TestRelease_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		nodeID       string
		lockOwner    string
		releaseOwner string
		timeout      time.Duration
		waitExpiry   bool
		wantErr      bool
	}{
		{
			name:         "valid release by owner",
			nodeID:       "1",
			lockOwner:    "agent-001",
			releaseOwner: "agent-001",
			timeout:      5 * time.Minute,
			waitExpiry:   false,
			wantErr:      false,
		},
		{
			name:         "release by different owner",
			nodeID:       "1.1",
			lockOwner:    "agent-001",
			releaseOwner: "agent-002",
			timeout:      5 * time.Minute,
			waitExpiry:   false,
			wantErr:      true,
		},
		{
			name:         "release with empty owner",
			nodeID:       "1",
			lockOwner:    "agent-001",
			releaseOwner: "",
			timeout:      5 * time.Minute,
			waitExpiry:   false,
			wantErr:      true,
		},
		{
			name:         "release expired lock",
			nodeID:       "1.2.3",
			lockOwner:    "prover",
			releaseOwner: "prover",
			timeout:      1 * time.Nanosecond,
			waitExpiry:   true,
			wantErr:      true,
		},
		{
			name:         "release by case-different owner",
			nodeID:       "1",
			lockOwner:    "Agent",
			releaseOwner: "agent",
			timeout:      5 * time.Minute,
			waitExpiry:   false,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			lk, err := lock.NewLock(nodeID, tt.lockOwner, tt.timeout)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			if tt.waitExpiry {
				time.Sleep(10 * time.Millisecond)
			}

			err = lk.Release(tt.releaseOwner)
			if (err != nil) != tt.wantErr {
				t.Errorf("Release(%q) error = %v, wantErr = %v", tt.releaseOwner, err, tt.wantErr)
			}
		})
	}
}

// TestRelease_PreservesLockInfo verifies release doesn't corrupt lock data
// (for cases where release fails and lock should remain intact)
func TestRelease_PreservesLockInfo(t *testing.T) {
	nodeID, err := types.Parse("1.3.2")
	if err != nil {
		t.Fatalf("types.Parse(\"1.3.2\") unexpected error: %v", err)
	}

	lk, err := lock.NewLock(nodeID, "agent-owner", 1*time.Hour)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	originalNodeID := lk.NodeID().String()
	originalOwner := lk.Owner()
	originalAcquired := lk.AcquiredAt()
	originalExpires := lk.ExpiresAt()

	// Attempt release by wrong owner (should fail)
	err = lk.Release("wrong-agent")
	if err == nil {
		t.Fatal("Release() by wrong owner expected error, got nil")
	}

	// Verify lock info is unchanged after failed release
	if lk.NodeID().String() != originalNodeID {
		t.Errorf("NodeID changed after failed release: got %q, want %q",
			lk.NodeID().String(), originalNodeID)
	}

	if lk.Owner() != originalOwner {
		t.Errorf("Owner changed after failed release: got %q, want %q",
			lk.Owner(), originalOwner)
	}

	if !lk.AcquiredAt().Equal(originalAcquired) {
		t.Errorf("AcquiredAt changed after failed release: got %v, want %v",
			lk.AcquiredAt(), originalAcquired)
	}

	if !lk.ExpiresAt().Equal(originalExpires) {
		t.Errorf("ExpiresAt changed after failed release: got %v, want %v",
			lk.ExpiresAt(), originalExpires)
	}
}

// TestRelease_AfterRefresh verifies release works after lock refresh
func TestRelease_AfterRefresh(t *testing.T) {
	nodeID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("types.Parse(\"1.1\") unexpected error: %v", err)
	}

	lk, err := lock.NewLock(nodeID, "agent-001", 1*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Refresh the lock
	err = lk.Refresh(30 * time.Minute)
	if err != nil {
		t.Fatalf("Refresh() unexpected error: %v", err)
	}

	// Release should still work after refresh
	err = lk.Release("agent-001")
	if err != nil {
		t.Errorf("Release() after Refresh() unexpected error: %v", err)
	}
}

// TestRelease_ConcurrentAttempts verifies concurrent release attempts are safe
func TestRelease_ConcurrentAttempts(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	lk, err := lock.NewLock(nodeID, "agent-001", 1*time.Hour)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Multiple goroutines attempting release (only one should succeed)
	results := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			results <- lk.Release("agent-001")
		}()
	}

	// Collect results
	var successCount, errorCount int
	for i := 0; i < 10; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	// Exactly one release should succeed (the first one)
	// All subsequent releases should fail (already released)
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful release, got %d", successCount)
	}

	if errorCount != 9 {
		t.Errorf("Expected exactly 9 failed releases, got %d", errorCount)
	}
}

// TestRelease_ErrorMessages verifies error messages are descriptive
func TestRelease_ErrorMessages(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	lk, err := lock.NewLock(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Test error from non-owner
	err = lk.Release("other-agent")
	if err == nil {
		t.Fatal("Release() by non-owner expected error, got nil")
	}

	// Error message should be non-empty and descriptive
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message should not be empty")
	}
}

// TestReleaseFunc_NilLock tests the package-level Release function with nil
func TestReleaseFunc_NilLock(t *testing.T) {
	// This tests the package-level function signature: Release(lock *Lock, owner string) error
	err := lock.Release(nil, "agent-001")
	if err == nil {
		t.Error("Release(nil, owner) expected error, got nil")
	}
}

// TestReleaseFunc_EmptyOwner tests the package-level Release function with empty owner
func TestReleaseFunc_EmptyOwner(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	lk, err := lock.NewLock(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Package-level function with empty owner
	err = lock.Release(lk, "")
	if err == nil {
		t.Error("Release(lock, \"\") expected error, got nil")
	}
}

// TestReleaseFunc_Valid tests the package-level Release function succeeds for valid input
func TestReleaseFunc_Valid(t *testing.T) {
	nodeID, err := types.Parse("1.2")
	if err != nil {
		t.Fatalf("types.Parse(\"1.2\") unexpected error: %v", err)
	}

	lk, err := lock.NewLock(nodeID, "prover-alpha", 10*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Package-level function should succeed
	err = lock.Release(lk, "prover-alpha")
	if err != nil {
		t.Errorf("Release(lock, owner) unexpected error: %v", err)
	}
}
