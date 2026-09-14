package lock

import "time"

// ExpireForTest moves l's expiration far enough into the past that IsExpired
// and IsStale report true, i.e. beyond ClockSkewTolerance. Tests use it
// instead of acquiring with a tiny timeout and sleeping, which stopped working
// once IsExpired gained the clock-skew grace period (a 1ns lock stays
// unexpired for ClockSkewTolerance).
//
// It mutates the lock in place, so it also works on a lock returned by
// Manager.Acquire, which stores that same pointer.
func ExpireForTest(l *ClaimLock) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.expiresAt = time.Now().UTC().Add(-2 * ClockSkewTolerance)
}
