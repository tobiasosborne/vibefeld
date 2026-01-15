package schema

import (
	"fmt"
	"sort"
)

// ChallengeSeverity represents the severity level of a challenge.
// Severity determines whether a challenge blocks node acceptance.
type ChallengeSeverity string

// Valid challenge severity levels.
const (
	// SeverityCritical indicates a fundamental error that must be fixed.
	// Critical challenges block node acceptance.
	SeverityCritical ChallengeSeverity = "critical"

	// SeverityMajor indicates a significant issue that should be addressed.
	// Major challenges block node acceptance.
	SeverityMajor ChallengeSeverity = "major"

	// SeverityMinor indicates a minor issue that could be improved.
	// Minor challenges do NOT block node acceptance.
	SeverityMinor ChallengeSeverity = "minor"

	// SeverityNote indicates a clarification request or suggestion.
	// Notes do NOT block node acceptance.
	SeverityNote ChallengeSeverity = "note"
)

// ChallengeSeverityInfo provides metadata about a challenge severity level.
type ChallengeSeverityInfo struct {
	ID               ChallengeSeverity
	Description      string
	BlocksAcceptance bool
}

// challengeSeverityRegistry maps each valid severity to its metadata.
var challengeSeverityRegistry = map[ChallengeSeverity]ChallengeSeverityInfo{
	SeverityCritical: {
		ID:               SeverityCritical,
		Description:      "Fundamental error that must be fixed",
		BlocksAcceptance: true,
	},
	SeverityMajor: {
		ID:               SeverityMajor,
		Description:      "Significant issue that should be addressed",
		BlocksAcceptance: true,
	},
	SeverityMinor: {
		ID:               SeverityMinor,
		Description:      "Minor issue that could be improved",
		BlocksAcceptance: false,
	},
	SeverityNote: {
		ID:               SeverityNote,
		Description:      "Clarification request or suggestion",
		BlocksAcceptance: false,
	},
}

// ValidateChallengeSeverity validates a challenge severity string.
// Returns an error if the severity is not one of the valid values.
func ValidateChallengeSeverity(s string) error {
	severity := ChallengeSeverity(s)
	if _, exists := challengeSeverityRegistry[severity]; !exists {
		return fmt.Errorf("invalid challenge severity: %q", s)
	}
	return nil
}

// SeverityBlocksAcceptance returns true if challenges with the given severity
// should block node acceptance.
func SeverityBlocksAcceptance(severity ChallengeSeverity) bool {
	info, exists := challengeSeverityRegistry[severity]
	if !exists {
		// Unknown severities default to blocking (fail-safe)
		return true
	}
	return info.BlocksAcceptance
}

// GetChallengeSeverityInfo returns metadata for a given challenge severity.
// The boolean return value indicates whether the severity exists.
func GetChallengeSeverityInfo(s ChallengeSeverity) (ChallengeSeverityInfo, bool) {
	info, exists := challengeSeverityRegistry[s]
	return info, exists
}

// AllChallengeSeverities returns a list of all valid challenge severities with their metadata.
// Results are sorted by ID for deterministic output.
func AllChallengeSeverities() []ChallengeSeverityInfo {
	result := make([]ChallengeSeverityInfo, 0, len(challengeSeverityRegistry))
	for _, info := range challengeSeverityRegistry {
		result = append(result, info)
	}
	// Sort by ID for consistent output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// DefaultChallengeSeverity returns the default severity for new challenges.
// The default is "major" to match the existing behavior where all challenges
// blocked acceptance.
func DefaultChallengeSeverity() ChallengeSeverity {
	return SeverityMajor
}
