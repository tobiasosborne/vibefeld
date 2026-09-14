//go:build integration

package e2e

import (
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/lock"
)

// lockExpiryMargin is slack added on top of a lock's timeout and
// lock.ClockSkewTolerance before a test relies on the lock being expired.
const lockExpiryMargin = 250 * time.Millisecond

// waitForLockExpiry sleeps until a lock acquired just now with the given
// timeout is expired as far as ClaimLock.IsExpired is concerned: past its
// nominal expiry plus lock.ClockSkewTolerance (5s). Sleeping only the nominal
// timeout, as these tests did before the tolerance existed, leaves the lock
// valid.
//
// Tests that call it take several seconds, so they run in parallel with each
// other (each uses its own manager, ledger and temp dir) and are skipped in
// -short mode, like the equivalent tests in internal/lock.
func waitForLockExpiry(t *testing.T, timeout time.Duration) {
	t.Helper()
	time.Sleep(timeout + lock.ClockSkewTolerance + lockExpiryMargin)
}

// slowLockExpiryTest marks a test that waits out lock.ClockSkewTolerance.
func slowLockExpiryTest(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping: waits for lock expiry plus lock.ClockSkewTolerance")
	}
	t.Parallel()
}
