// Package lock provides node locking for exclusive agent access.
package lock

// IsStale returns true if the lock is nil or expired.
// A stale lock is one that can be safely cleaned up or overwritten.
func IsStale(l *Lock) bool {
	if l == nil {
		return true
	}
	return l.IsExpired()
}

// IsStale returns true if the lock is expired.
// A stale lock is one that can be safely cleaned up or overwritten.
func (l *Lock) IsStale() bool {
	return l.IsExpired()
}
