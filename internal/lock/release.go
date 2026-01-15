// Package lock provides node locking for exclusive agent access.
package lock

import (
	"errors"
	"strings"
)

// ErrNilLock is returned when attempting to release a nil lock.
var ErrNilLock = errors.New("cannot release nil lock")

// ErrNotOwner is returned when the release requester is not the lock owner.
var ErrNotOwner = errors.New("lock not owned by requester")

// ErrEmptyOwner is returned when the owner string is empty or whitespace.
var ErrEmptyOwner = errors.New("owner cannot be empty or whitespace")

// ErrLockExpired is returned when attempting to release an expired lock.
var ErrLockExpired = errors.New("cannot release expired lock")

// ErrAlreadyReleased is returned when attempting to release an already released lock.
var ErrAlreadyReleased = errors.New("lock already released")

// Release releases the lock if the given owner matches the lock's owner.
// Returns an error if:
// - owner is empty or whitespace only
// - owner does not match the lock's owner (case-sensitive)
// - the lock has expired
// - the lock has already been released
//
// This method is thread-safe.
func (l *ClaimLock) Release(owner string) error {
	// Validate owner is not empty or whitespace
	if owner == "" || strings.TrimSpace(owner) == "" {
		return ErrEmptyOwner
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if already released
	if l.released {
		return ErrAlreadyReleased
	}

	// Check if lock has expired
	if l.IsExpired() {
		return ErrLockExpired
	}

	// Check owner matches (case-sensitive)
	if l.owner != owner {
		return ErrNotOwner
	}

	// Mark as released
	l.released = true
	return nil
}

// Release is a package-level function that releases a lock for the given owner.
// Returns an error if:
// - l is nil
// - owner is empty or whitespace only
// - owner does not match the lock's owner
// - the lock has expired
// - the lock has already been released
func Release(l *ClaimLock, owner string) error {
	if l == nil {
		return ErrNilLock
	}
	return l.Release(owner)
}
