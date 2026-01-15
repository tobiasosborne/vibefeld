//go:build integration
// +build integration

// These tests define expected behavior for stale lock detection.
// Stale locks are locks that have expired and can be safely cleaned up.
// Run with: go test -tags=integration ./internal/lock/...

package lock_test

import (
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/lock"
	"github.com/tobias/vibefeld/internal/types"
)

// TestIsStale_NilLock verifies IsStale handles nil lock gracefully
func TestIsStale_NilLock(t *testing.T) {
	// Package-level function should return true for nil lock
	// (nil lock is considered stale as it cannot be valid)
	result := lock.IsStale(nil)
	if !result {
		t.Error("IsStale(nil) = false, want true (nil lock is stale)")
	}
}

// TestIsStale_FreshLock verifies fresh locks are not stale
func TestIsStale_FreshLock(t *testing.T) {
	tests := []struct {
		name    string
		nodeID  string
		owner   string
		timeout time.Duration
	}{
		{"root node with long timeout", "1", "agent-001", 1 * time.Hour},
		{"child node with medium timeout", "1.1", "agent-002", 10 * time.Minute},
		{"deep node with short timeout", "1.2.3.4", "prover-alpha", 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			lk, err := lock.NewClaimLock(nodeID, tt.owner, tt.timeout)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			// Fresh lock should not be stale
			if lock.IsStale(lk) {
				t.Error("IsStale() = true for fresh lock, want false")
			}

			// Method version should agree
			if lk.IsStale() {
				t.Error("Lock.IsStale() = true for fresh lock, want false")
			}
		})
	}
}

// TestIsStale_ExpiredLock verifies expired locks are stale
func TestIsStale_ExpiredLock(t *testing.T) {
	nodeID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("types.Parse(\"1.1\") unexpected error: %v", err)
	}

	// Create lock with very short timeout
	lk, err := lock.NewClaimLock(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Verify lock is expired first
	if !lk.IsExpired() {
		t.Fatal("Lock should be expired but IsExpired() returned false")
	}

	// Expired lock should be stale
	if !lock.IsStale(lk) {
		t.Error("IsStale() = false for expired lock, want true")
	}

	// Method version should agree
	if !lk.IsStale() {
		t.Error("Lock.IsStale() = false for expired lock, want true")
	}
}

// TestIsStale_JustExpired verifies a lock that just expired is stale
func TestIsStale_JustExpired(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	// Create lock with 1ms timeout
	lk, err := lock.NewClaimLock(nodeID, "agent-001", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Wait just past expiration
	time.Sleep(5 * time.Millisecond)

	// Should be expired
	if !lk.IsExpired() {
		t.Fatal("Lock should be expired after 5ms wait on 1ms timeout")
	}

	// Just-expired lock should be stale
	if !lock.IsStale(lk) {
		t.Error("IsStale() = false for just-expired lock, want true")
	}

	if !lk.IsStale() {
		t.Error("Lock.IsStale() = false for just-expired lock, want true")
	}
}

// TestIsStale_AboutToExpire verifies a lock about to expire is not yet stale
func TestIsStale_AboutToExpire(t *testing.T) {
	nodeID, err := types.Parse("1.2")
	if err != nil {
		t.Fatalf("types.Parse(\"1.2\") unexpected error: %v", err)
	}

	// Create lock with 1 second timeout
	lk, err := lock.NewClaimLock(nodeID, "agent-001", 1*time.Second)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Wait 500ms - lock should have ~500ms remaining
	time.Sleep(500 * time.Millisecond)

	// Should not be expired yet
	if lk.IsExpired() {
		t.Skip("Lock expired during test - timing issue, skipping")
	}

	// Lock about to expire is not stale until actually expired
	if lock.IsStale(lk) {
		t.Error("IsStale() = true for lock about to expire, want false")
	}

	if lk.IsStale() {
		t.Error("Lock.IsStale() = true for lock about to expire, want false")
	}
}

// TestIsStale_ZeroExpiry verifies handling of zero expiry time
// A lock with zero expiry should be considered stale
func TestIsStale_ZeroExpiry(t *testing.T) {
	// This tests an edge case where a lock might have been
	// improperly initialized or corrupted
	// We can't directly create such a lock through NewLock,
	// but we can test through JSON unmarshaling

	// Create a lock via JSON with zero expiry time
	jsonData := `{
		"node_id": "1",
		"owner": "agent-001",
		"acquired_at": "2025-01-01T00:00:00Z",
		"expires_at": "0001-01-01T00:00:00Z"
	}`

	var lk lock.ClaimLock
	err := lk.UnmarshalJSON([]byte(jsonData))
	if err != nil {
		// If unmarshaling fails, that's acceptable - the implementation
		// might reject zero expiry as invalid
		t.Skip("Implementation rejects zero expiry time in JSON")
	}

	// A lock with zero (or ancient) expiry should be stale
	if !lock.IsStale(&lk) {
		t.Error("IsStale() = false for lock with zero expiry, want true")
	}

	if !lk.IsStale() {
		t.Error("Lock.IsStale() = false for lock with zero expiry, want true")
	}
}

// TestIsStale_LongExpiredLock verifies locks that expired long ago are stale
func TestIsStale_LongExpiredLock(t *testing.T) {
	// Create a lock via JSON that expired in the past
	jsonData := `{
		"node_id": "1.1.1",
		"owner": "ancient-agent",
		"acquired_at": "2020-01-01T00:00:00Z",
		"expires_at": "2020-01-01T01:00:00Z"
	}`

	var lk lock.ClaimLock
	err := lk.UnmarshalJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("UnmarshalJSON() unexpected error: %v", err)
	}

	// Lock that expired years ago should definitely be stale
	if !lock.IsStale(&lk) {
		t.Error("IsStale() = false for lock expired years ago, want true")
	}

	if !lk.IsStale() {
		t.Error("Lock.IsStale() = false for lock expired years ago, want true")
	}
}

// TestIsStale_FutureExpiry verifies locks with future expiry are not stale
func TestIsStale_FutureExpiry(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	// Create lock with very long timeout
	lk, err := lock.NewClaimLock(nodeID, "agent-001", 24*time.Hour)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Lock with future expiry should not be stale
	if lock.IsStale(lk) {
		t.Error("IsStale() = true for lock with 24h remaining, want false")
	}

	if lk.IsStale() {
		t.Error("Lock.IsStale() = true for lock with 24h remaining, want false")
	}
}

// TestIsStale_TableDriven provides comprehensive table-driven tests
func TestIsStale_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		nodeID     string
		owner      string
		timeout    time.Duration
		waitBefore time.Duration
		wantStale  bool
	}{
		{
			name:       "fresh lock with hour timeout",
			nodeID:     "1",
			owner:      "agent-001",
			timeout:    1 * time.Hour,
			waitBefore: 0,
			wantStale:  false,
		},
		{
			name:       "fresh lock with minute timeout",
			nodeID:     "1.1",
			owner:      "agent-002",
			timeout:    1 * time.Minute,
			waitBefore: 0,
			wantStale:  false,
		},
		{
			name:       "expired nanosecond lock",
			nodeID:     "1.2",
			owner:      "agent-003",
			timeout:    1 * time.Nanosecond,
			waitBefore: 10 * time.Millisecond,
			wantStale:  true,
		},
		{
			name:       "expired millisecond lock",
			nodeID:     "1.3",
			owner:      "prover",
			timeout:    1 * time.Millisecond,
			waitBefore: 10 * time.Millisecond,
			wantStale:  true,
		},
		{
			name:       "half-elapsed lock",
			nodeID:     "1.4",
			owner:      "verifier",
			timeout:    100 * time.Millisecond,
			waitBefore: 30 * time.Millisecond,
			wantStale:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			lk, err := lock.NewClaimLock(nodeID, tt.owner, tt.timeout)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			if tt.waitBefore > 0 {
				time.Sleep(tt.waitBefore)
			}

			// Package-level function
			gotStale := lock.IsStale(lk)
			if gotStale != tt.wantStale {
				t.Errorf("IsStale() = %v, want %v", gotStale, tt.wantStale)
			}

			// Method version
			gotMethodStale := lk.IsStale()
			if gotMethodStale != tt.wantStale {
				t.Errorf("Lock.IsStale() = %v, want %v", gotMethodStale, tt.wantStale)
			}

			// Both should agree
			if gotStale != gotMethodStale {
				t.Errorf("IsStale() = %v but Lock.IsStale() = %v, should be equal",
					gotStale, gotMethodStale)
			}
		})
	}
}

// TestIsStale_ConsistentWithIsExpired verifies IsStale is consistent with IsExpired
// A stale lock must be expired (but implementation could have additional criteria)
func TestIsStale_ConsistentWithIsExpired(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		wait    time.Duration
	}{
		{"fresh long lock", 1 * time.Hour, 0},
		{"fresh short lock", 1 * time.Second, 0},
		{"expired lock", 1 * time.Nanosecond, 10 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse("1")
			if err != nil {
				t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
			}

			lk, err := lock.NewClaimLock(nodeID, "agent-001", tt.timeout)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			if tt.wait > 0 {
				time.Sleep(tt.wait)
			}

			isExpired := lk.IsExpired()
			isStale := lk.IsStale()

			// If not expired, should not be stale
			// (A lock cannot be stale if it hasn't expired)
			if !isExpired && isStale {
				t.Error("Lock is stale but not expired - stale requires expired")
			}

			// Note: A lock can be expired but not yet stale in some implementations
			// (e.g., if stale requires expiry + grace period)
			// But in the basic implementation, expired == stale
		})
	}
}

// TestIsStale_AfterRefresh verifies refreshed locks are not stale
func TestIsStale_AfterRefresh(t *testing.T) {
	nodeID, err := types.Parse("1.1.1")
	if err != nil {
		t.Fatalf("types.Parse(\"1.1.1\") unexpected error: %v", err)
	}

	// Create lock with short timeout
	lk, err := lock.NewClaimLock(nodeID, "agent-001", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Wait partway through
	time.Sleep(5 * time.Millisecond)

	// Refresh before expiration
	err = lk.Refresh(1 * time.Hour)
	if err != nil {
		t.Fatalf("Refresh() unexpected error: %v", err)
	}

	// Refreshed lock should not be stale
	if lock.IsStale(lk) {
		t.Error("IsStale() = true after Refresh, want false")
	}

	if lk.IsStale() {
		t.Error("Lock.IsStale() = true after Refresh, want false")
	}
}

// TestIsStale_RefreshDoesNotFixStale verifies refresh cannot fix already-stale lock
func TestIsStale_RefreshExpiredLock(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	// Create lock with very short timeout
	lk, err := lock.NewClaimLock(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Verify stale
	if !lk.IsStale() {
		t.Fatal("Lock should be stale after expiration")
	}

	// Refresh the expired lock (this may or may not be allowed by implementation)
	// If refresh works, the lock should no longer be stale
	err = lk.Refresh(1 * time.Hour)
	if err != nil {
		// Some implementations may disallow refreshing expired locks
		t.Logf("Refresh() on expired lock returned error: %v", err)
		// Lock should still be stale if refresh failed
		if !lk.IsStale() {
			t.Error("Lock.IsStale() = false after failed refresh, want true")
		}
		return
	}

	// If refresh succeeded, lock should no longer be stale
	if lk.IsStale() {
		t.Error("Lock.IsStale() = true after successful refresh, want false")
	}
}

// TestIsStale_MultipleNodes verifies stale detection is independent per lock
func TestIsStale_MultipleNodes(t *testing.T) {
	nodeID1, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("types.Parse(\"1.1\") unexpected error: %v", err)
	}

	nodeID2, err := types.Parse("1.2")
	if err != nil {
		t.Fatalf("types.Parse(\"1.2\") unexpected error: %v", err)
	}

	// Create one fresh lock and one that will expire
	freshLock, err := lock.NewClaimLock(nodeID1, "agent-001", 1*time.Hour)
	if err != nil {
		t.Fatalf("NewLock() for fresh lock unexpected error: %v", err)
	}

	expiredLock, err := lock.NewClaimLock(nodeID2, "agent-002", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("NewLock() for expired lock unexpected error: %v", err)
	}

	// Wait for one to expire
	time.Sleep(10 * time.Millisecond)

	// Fresh lock should not be stale
	if freshLock.IsStale() {
		t.Error("Fresh lock IsStale() = true, want false")
	}

	// Expired lock should be stale
	if !expiredLock.IsStale() {
		t.Error("Expired lock IsStale() = false, want true")
	}

	// They should be independent
	if freshLock.IsStale() == expiredLock.IsStale() {
		t.Error("Both locks have same staleness, but one should be fresh and one stale")
	}
}

// TestIsStale_ConcurrentAccess verifies concurrent stale checks are safe
func TestIsStale_ConcurrentAccess(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	lk, err := lock.NewClaimLock(nodeID, "agent-001", 1*time.Hour)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Spawn multiple goroutines checking staleness
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = lock.IsStale(lk)
				_ = lk.IsStale()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestIsStale_ExpiryBoundary tests behavior exactly at expiry boundary
func TestIsStale_ExpiryBoundary(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	// Create lock with 50ms timeout
	lk, err := lock.NewClaimLock(nodeID, "agent-001", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Before expiry
	if lk.IsStale() {
		t.Error("Lock should not be stale before expiry")
	}

	// Wait past expiry
	time.Sleep(60 * time.Millisecond)

	// After expiry
	if !lk.IsStale() {
		t.Error("Lock should be stale after expiry")
	}
}
