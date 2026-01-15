// Package lock provides node locking for exclusive agent access.
package lock

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/types"
)

// TestLockRefreshRace verifies that ClaimLock.Refresh() is now thread-safe.
// This test was previously skipped due to a race condition that has been fixed.
func TestLockRefreshRace(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lock, err := NewClaimLock(nodeID, "agent", 5*time.Second)
	if err != nil {
		t.Fatalf("failed to create lock: %v", err)
	}

	var wg sync.WaitGroup

	// Concurrent refreshers - should be safe now with mutex protection
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = lock.Refresh(5 * time.Second)
			}
		}()
	}

	wg.Wait()
	// If we reach here without race detector complaints, the fix is working
}

// TestLockRefreshRaceDetection verifies concurrent Refresh() and IsExpired() calls
// are now safe. Run with: go test -race ./internal/lock
func TestLockRefreshRaceDetection(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lock, err := NewClaimLock(nodeID, "agent", 5*time.Second)
	if err != nil {
		t.Fatalf("failed to create lock: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Concurrent refreshers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = lock.Refresh(5 * time.Second)
				}
			}
		}()
	}

	// Concurrent readers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = lock.IsExpired()
					_ = lock.ExpiresAt()
				}
			}
		}()
	}

	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestExpiresAtRace verifies IsExpired() is now thread-safe with Refresh().
func TestExpiresAtRace(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lock, err := NewClaimLock(nodeID, "agent", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create lock: %v", err)
	}

	var wg sync.WaitGroup
	var expiredCount, refreshCount atomic.Int32

	// Goroutine checking expiration
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			if lock.IsExpired() {
				expiredCount.Add(1)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	// Goroutine refreshing
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = lock.Refresh(100 * time.Millisecond)
			refreshCount.Add(1)
			time.Sleep(2 * time.Millisecond)
		}
	}()

	wg.Wait()

	t.Logf("expired checks: %d, refreshes: %d", expiredCount.Load(), refreshCount.Load())
}

// TestLockFieldRaces tests for races between various ClaimLock methods.
func TestLockFieldRaces(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lock, err := NewClaimLock(nodeID, "agent", time.Second)
	if err != nil {
		t.Fatalf("failed to create lock: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	var readCount, refreshCount atomic.Int32

	// Reader checking expiration
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = lock.IsExpired()
					_ = lock.ExpiresAt()
					readCount.Add(1)
				}
			}
		}()
	}

	// Writer refreshing
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = lock.Refresh(time.Second)
					refreshCount.Add(1)
				}
			}
		}()
	}

	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()

	t.Logf("completed %d reads, %d refreshes", readCount.Load(), refreshCount.Load())
}

// TestLockMutexConsistency verifies that ClaimLock's mutex is consistently used
// for operations that should be atomic.
func TestLockMutexConsistency(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Test that multiple ClaimLock instances can be created and used concurrently
	var wg sync.WaitGroup
	var createCount atomic.Int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			lock, err := NewClaimLock(nodeID, "agent", time.Second)
			if err != nil {
				return
			}

			// These operations should be thread-safe
			_ = lock.NodeID()
			_ = lock.Owner()
			_ = lock.AcquiredAt()
			_ = lock.ExpiresAt()
			_ = lock.IsExpired()
			_ = lock.Refresh(time.Second)

			createCount.Add(1)
		}()
	}

	wg.Wait()

	if got := createCount.Load(); got != 10 {
		t.Errorf("expected 10 locks created, got %d", got)
	}
}

// TestLockMarshalJSONRace verifies MarshalJSON is thread-safe with Refresh.
func TestLockMarshalJSONRace(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	lock, err := NewClaimLock(nodeID, "agent", time.Second)
	if err != nil {
		t.Fatalf("failed to create lock: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Marshaler
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = lock.MarshalJSON()
			}
		}
	}()

	// Refresher
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = lock.Refresh(time.Second)
			}
		}
	}()

	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()
}
