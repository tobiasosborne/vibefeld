// Package node provides the core node types for the AF framework.
package node

import (
	"github.com/tobias/vibefeld/internal/errors"
)

// ValidateChallengeLimit checks whether the number of challenges exceeds the maximum allowed.
// It returns nil if the challenge count is within the limit (len(challenges) <= maxChallenges),
// or a CHALLENGE_LIMIT_EXCEEDED error if the limit is exceeded.
//
// A nil slice is treated as 0 challenges.
// The error message includes both the current count and the maximum limit.
func ValidateChallengeLimit(challenges []*Challenge, maxChallenges int) error {
	count := len(challenges) // len(nil) returns 0, so nil slice is handled gracefully

	if count > maxChallenges {
		return errors.Newf(
			errors.CHALLENGE_LIMIT_EXCEEDED,
			"challenge count %d exceeds maximum %d",
			count,
			maxChallenges,
		)
	}

	return nil
}
