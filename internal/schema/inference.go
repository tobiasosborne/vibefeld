// Package schema defines core types for the AF proof framework.
package schema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tobias/vibefeld/internal/fuzzy"
)

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

// inferenceRegistry is the authoritative map of all valid inference types.
var inferenceRegistry = map[InferenceType]InferenceInfo{
	InferenceModusPonens: {
		ID:   InferenceModusPonens,
		Name: "Modus Ponens",
		Form: "P, P → Q ⊢ Q",
	},
	InferenceModusTollens: {
		ID:   InferenceModusTollens,
		Name: "Modus Tollens",
		Form: "¬Q, P → Q ⊢ ¬P",
	},
	InferenceUniversalInstantiation: {
		ID:   InferenceUniversalInstantiation,
		Name: "Universal Instantiation",
		Form: "∀x.P(x) ⊢ P(t)",
	},
	InferenceExistentialInstantiation: {
		ID:   InferenceExistentialInstantiation,
		Name: "Existential Instantiation",
		Form: "∃x.P(x) ⊢ P(c) for fresh c",
	},
	InferenceUniversalGeneralization: {
		ID:   InferenceUniversalGeneralization,
		Name: "Universal Generalization",
		Form: "P(x) for arbitrary x ⊢ ∀x.P(x)",
	},
	InferenceExistentialGeneralization: {
		ID:   InferenceExistentialGeneralization,
		Name: "Existential Generalization",
		Form: "P(c) ⊢ ∃x.P(x)",
	},
	InferenceByDefinition: {
		ID:   InferenceByDefinition,
		Name: "By Definition",
		Form: "unfold definition",
	},
	InferenceAssumption: {
		ID:   InferenceAssumption,
		Name: "Assumption",
		Form: "global hypothesis",
	},
	InferenceLocalAssume: {
		ID:   InferenceLocalAssume,
		Name: "Local Assume",
		Form: "introduce local hypothesis",
	},
	InferenceLocalDischarge: {
		ID:   InferenceLocalDischarge,
		Name: "Local Discharge",
		Form: "conclude from local hypothesis",
	},
	InferenceContradiction: {
		ID:   InferenceContradiction,
		Name: "Contradiction",
		Form: "P ∧ ¬P ⊢ ⊥",
	},
}

// ValidateInference validates that the given string is a valid inference type.
// Returns an error if the inference type is not recognized.
func ValidateInference(s string) error {
	_, ok := inferenceRegistry[InferenceType(s)]
	if !ok {
		return fmt.Errorf("unknown inference type: %q", s)
	}
	return nil
}

// GetInferenceInfo returns metadata for the given inference type.
// Returns (info, true) if the type exists, (zero, false) if not found.
func GetInferenceInfo(t InferenceType) (InferenceInfo, bool) {
	info, ok := inferenceRegistry[t]
	return info, ok
}

// AllInferences returns a list of all valid inference types with their metadata.
// The results are sorted alphabetically by inference type ID for consistent ordering.
func AllInferences() []InferenceInfo {
	result := make([]InferenceInfo, 0, len(inferenceRegistry))
	for _, info := range inferenceRegistry {
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// SuggestInference performs fuzzy matching on the input string and returns
// the closest matching inference type. Returns (suggestion, true) if a close
// match is found, (zero, false) if no reasonable match exists.
//
// Matching is case-insensitive and tolerant of minor typos.
func SuggestInference(input string) (InferenceType, bool) {
	if input == "" {
		return "", false
	}

	// Normalize input to lowercase for case-insensitive matching
	normalizedInput := strings.ToLower(input)

	// Track the best match
	var bestMatch InferenceType
	bestDistance := -1
	bestScore := -1.0

	// Calculate distance to each inference type
	for inferenceType := range inferenceRegistry {
		typeStr := string(inferenceType)
		distance := fuzzy.Distance(normalizedInput, typeStr)

		// Calculate common prefix length for tie-breaking
		commonPrefixLen := 0
		for i := 0; i < len(normalizedInput) && i < len(typeStr); i++ {
			if normalizedInput[i] == typeStr[i] {
				commonPrefixLen++
			} else {
				break
			}
		}

		// Calculate a score that considers distance, common prefix, and target length
		// - Lower distance is better (primary factor)
		// - Longer common prefix is better (strong tie-breaker for similar distances)
		// - Longer target is better (weak tie-breaker for abbreviations)
		// Score = -distance + (commonPrefix * 1.5) + (targetLen / 1000.0)
		// The 1.5 weight on commonPrefix helps match abbreviations like "univ_inst" -> "universal_instantiation"
		score := float64(-distance) + (float64(commonPrefixLen) * 1.5) + (float64(len(typeStr)) / 1000.0)

		// Update best match if this has a better score
		if bestDistance == -1 || score > bestScore {
			bestDistance = distance
			bestMatch = inferenceType
			bestScore = score
		}
	}

	// Determine if the best match is "close enough"
	// A match is acceptable if the distance is reasonable relative to both
	// the input length and the target length.
	// We use a lenient threshold to handle:
	// - Exact matches (distance 0)
	// - Minor typos (1-3 edits)
	// - Abbreviations and prefixes (e.g., "by_def" -> "by_definition")
	// - Longer inputs with proportionally more errors

	inputLen := len(normalizedInput)
	targetLen := len(string(bestMatch))

	// For very short inputs (1-2 chars), be strict (threshold 1)
	if inputLen <= 2 {
		if bestDistance <= 1 {
			return bestMatch, true
		}
		return "", false
	}

	// For longer inputs, use a more sophisticated threshold:
	// - Allow edits up to roughly the length difference between target and input
	// - Plus a small buffer for typos (inputLen / 2)
	// This handles both:
	//   - Abbreviations: "by_def" (6) -> "by_definition" (13), diff=7, buffer=3, threshold=10
	//   - Typos: "modus_tone" (10) -> "modus_tollens" (13), diff=3, buffer=5, threshold=8
	//   - But rejects random strings: "foobar" (6) vs "modus_ponens" (12), diff=6, buffer=3, threshold=9 < distance(10)
	lengthDiff := targetLen - inputLen
	if lengthDiff < 0 {
		lengthDiff = -lengthDiff // absolute value
	}
	threshold := lengthDiff + (inputLen / 2)

	if bestDistance <= threshold {
		return bestMatch, true
	}

	return "", false
}
