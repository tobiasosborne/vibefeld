package schema

import "errors"

// ChallengeTarget represents the aspect of a proof node being challenged.
type ChallengeTarget string

// Valid challenge targets per PRD.
const (
	TargetStatement    ChallengeTarget = "statement"
	TargetInference    ChallengeTarget = "inference"
	TargetContext      ChallengeTarget = "context"
	TargetDependencies ChallengeTarget = "dependencies"
	TargetScope        ChallengeTarget = "scope"
	TargetGap          ChallengeTarget = "gap"
	TargetTypeError    ChallengeTarget = "type_error"
	TargetDomain       ChallengeTarget = "domain"
	TargetCompleteness ChallengeTarget = "completeness"
)

// ChallengeTargetInfo provides metadata about a challenge target.
type ChallengeTargetInfo struct {
	ID          ChallengeTarget
	Description string
}

// ValidateChallengeTarget validates a single challenge target.
// Returns an error if the target is not one of the valid challenge targets.
func ValidateChallengeTarget(s string) error {
	return errors.New("not implemented")
}

// ValidateChallengeTargets validates a list of challenge targets.
// Returns an error if any target is invalid or if the list is empty.
func ValidateChallengeTargets(targets []string) error {
	return errors.New("not implemented")
}

// GetChallengeTargetInfo returns metadata for a given challenge target.
// The boolean return value indicates whether the target exists.
func GetChallengeTargetInfo(t ChallengeTarget) (ChallengeTargetInfo, bool) {
	return ChallengeTargetInfo{}, false
}

// AllChallengeTargets returns a list of all valid challenge targets with their descriptions.
func AllChallengeTargets() []ChallengeTargetInfo {
	return nil
}

// ParseChallengeTargets parses a comma-separated string of challenge targets.
// Whitespace around target names is trimmed.
// Returns an error if any target is invalid.
func ParseChallengeTargets(csv string) ([]ChallengeTarget, error) {
	return nil, errors.New("not implemented")
}
