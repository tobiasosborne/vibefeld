// Package schema defines core types for the AF proof framework.
package schema

import "errors"

// InferenceType represents a valid inference rule in the proof system.
type InferenceType string

// All valid inference types as constants.
const (
	InferenceModusPonens               InferenceType = "modus_ponens"
	InferenceModusTollens              InferenceType = "modus_tollens"
	InferenceUniversalInstantiation    InferenceType = "universal_instantiation"
	InferenceExistentialInstantiation  InferenceType = "existential_instantiation"
	InferenceUniversalGeneralization   InferenceType = "universal_generalization"
	InferenceExistentialGeneralization InferenceType = "existential_generalization"
	InferenceByDefinition              InferenceType = "by_definition"
	InferenceAssumption                InferenceType = "assumption"
	InferenceLocalAssume               InferenceType = "local_assume"
	InferenceLocalDischarge            InferenceType = "local_discharge"
	InferenceContradiction             InferenceType = "contradiction"
)

// InferenceInfo contains metadata about an inference type.
type InferenceInfo struct {
	ID   InferenceType // The inference type identifier
	Name string        // Human-readable name
	Form string        // Logical form notation
}

// ValidateInference validates that the given string is a valid inference type.
// Returns an error if the inference type is not recognized.
func ValidateInference(s string) error {
	return errors.New("not implemented")
}

// GetInferenceInfo returns metadata for the given inference type.
// Returns (info, true) if the type exists, (zero, false) if not found.
func GetInferenceInfo(t InferenceType) (InferenceInfo, bool) {
	return InferenceInfo{}, false
}

// AllInferences returns a list of all valid inference types with their metadata.
func AllInferences() []InferenceInfo {
	return nil
}

// SuggestInference performs fuzzy matching on the input string and returns
// the closest matching inference type. Returns (suggestion, true) if a close
// match is found, (zero, false) if no reasonable match exists.
//
// Matching is case-insensitive and tolerant of minor typos.
func SuggestInference(input string) (InferenceType, bool) {
	return "", false
}
