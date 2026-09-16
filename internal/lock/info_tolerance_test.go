package lock

import (
	"sync"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/types"
)

// uj18: GetLockInfo must apply the same ClockSkewTolerance grace period as
// ClaimLock.IsExpired and read the lock's fields under its mutex.
func TestGetLockInfo_AppliesClockSkewTolerance(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("parse node: %v", err)
	}

	lk, err := NewClaimLock(nodeID, "agent-1", time.Nanosecond)
	if err != nil {
		t.Fatalf("NewClaimLock: %v", err)
	}

	// A nominally expired but tolerance-fresh lock is not expired.
	info, err := GetLockInfo(lk)
	if err != nil {
		t.Fatalf("GetLockInfo: %v", err)
	}
	if info.IsExpired {
		t.Error("IsExpired = true for a lock within ClockSkewTolerance, want false")
	}
	if info.IsExpired != lk.IsExpired() {
		t.Errorf("GetLockInfo.IsExpired = %v, ClaimLock.IsExpired = %v; want equal", info.IsExpired, lk.IsExpired())
	}

	// Beyond the tolerance it is expired.
	ExpireForTest(lk)
	info, err = GetLockInfo(lk)
	if err != nil {
		t.Fatalf("GetLockInfo: %v", err)
	}
	if !info.IsExpired {
		t.Error("IsExpired = false for a lock beyond ClockSkewTolerance, want true")
	}

	// A released lock is always expired.
	lk2, err := NewClaimLock(nodeID, "agent-2", time.Hour)
	if err != nil {
		t.Fatalf("NewClaimLock: %v", err)
	}
	lk2.MarkReleased()
	info, err = GetLockInfo(lk2)
	if err != nil {
		t.Fatalf("GetLockInfo: %v", err)
	}
	if !info.IsExpired {
		t.Error("IsExpired = false for a released lock, want true")
	}
}

// GetLockInfo racing Refresh and MarkReleased must be clean under -race.
func TestGetLockInfo_ConcurrentWithRefresh(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("parse node: %v", err)
	}
	lk, err := NewClaimLock(nodeID, "agent-1", time.Minute)
	if err != nil {
		t.Fatalf("NewClaimLock: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = lk.Refresh(time.Minute)
		}()
		go func() {
			defer wg.Done()
			if _, err := GetLockInfo(lk); err != nil {
				t.Errorf("GetLockInfo: %v", err)
			}
		}()
	}
	wg.Wait()
}

// MarkReleased running concurrently with GetLockInfo must be clean under -race.
// A released lock must always be reported expired once the release lands.
func TestGetLockInfo_ConcurrentWithMarkReleased(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("parse node: %v", err)
	}
	lk, err := NewClaimLock(nodeID, "agent-1", time.Hour)
	if err != nil {
		t.Fatalf("NewClaimLock: %v", err)
	}

	var wg sync.WaitGroup
	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := GetLockInfo(lk); err != nil {
				t.Errorf("GetLockInfo: %v", err)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		lk.MarkReleased()
		close(done)
	}()
	wg.Wait()
	<-done

	info, err := GetLockInfo(lk)
	if err != nil {
		t.Fatalf("GetLockInfo: %v", err)
	}
	if !info.IsExpired {
		t.Error("IsExpired = false for a released lock, want true")
	}
}
