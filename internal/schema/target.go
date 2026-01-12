package schema

import (
	"errors"
	"fmt"
	"strings"
)

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

// challengeTargetRegistry maps each valid challenge target to its metadata.
var challengeTargetRegistry = map[ChallengeTarget]ChallengeTargetInfo{
	TargetStatement:    {ID: TargetStatement, Description: "The claim text itself is disputed"},
	TargetInference:    {ID: TargetInference, Description: "The inference type is inappropriate"},
	TargetContext:      {ID: TargetContext, Description: "Referenced definitions are wrong/missing"},
	TargetDependencies: {ID: TargetDependencies, Description: "Node dependencies are incorrect"},
	TargetScope:        {ID: TargetScope, Description: "Scope/local assumption issues"},
	TargetGap:          {ID: TargetGap, Description: "Logical gap in reasoning"},
	TargetTypeError:    {ID: TargetTypeError, Description: "Type mismatch in mathematical objects"},
	TargetDomain:       {ID: TargetDomain, Description: "Domain restriction violation"},
	TargetCompleteness: {ID: TargetCompleteness, Description: "Missing cases or incomplete argument"},
}

// ValidateChallengeTarget validates a single challenge target.
// Returns an error if the target is not one of the valid challenge targets.
func ValidateChallengeTarget(s string) error {
	target := ChallengeTarget(s)
	if _, exists := challengeTargetRegistry[target]; !exists {
		return fmt.Errorf("invalid challenge target: %q", s)
	}
	return nil
}

// ValidateChallengeTargets validates a list of challenge targets.
// Returns an error if any target is invalid or if the list is empty.
func ValidateChallengeTargets(targets []string) error {
	if len(targets) == 0 {
		return errors.New("challenge targets list cannot be empty")
	}
	for _, target := range targets {
		if err := ValidateChallengeTarget(target); err != nil {
			return err
		}
	}
	return nil
}

// GetChallengeTargetInfo returns metadata for a given challenge target.
// The boolean return value indicates whether the target exists.
func GetChallengeTargetInfo(t ChallengeTarget) (ChallengeTargetInfo, bool) {
	info, exists := challengeTargetRegistry[t]
	return info, exists
}

// AllChallengeTargets returns a list of all valid challenge targets with their descriptions.
func AllChallengeTargets() []ChallengeTargetInfo {
	result := make([]ChallengeTargetInfo, 0, len(challengeTargetRegistry))
	for _, info := range challengeTargetRegistry {
		result = append(result, info)
	}
	return result
}

// ParseChallengeTargets parses a comma-separated string of challenge targets.
// Whitespace around target names is trimmed.
// Returns an error if any target is invalid or if any segment is empty.
func ParseChallengeTargets(csv string) ([]ChallengeTarget, error) {
	if csv == "" {
		return nil, errors.New("no valid challenge targets found in input")
	}

	parts := strings.Split(csv, ",")
	var result []ChallengeTarget

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, errors.New("empty target in list")
		}
		if err := ValidateChallengeTarget(trimmed); err != nil {
			return nil, err
		}
		result = append(result, ChallengeTarget(trimmed))
	}

	if len(result) == 0 {
		return nil, errors.New("no valid challenge targets found in input")
	}

	return result, nil
}
