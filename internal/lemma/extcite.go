// Package lemma provides validation for lemma and citation references.
package lemma

import (
	"regexp"

	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/state"
)

// extCitePattern matches external:NAME citations in text.
// NAME can contain alphanumeric characters, hyphens, and underscores.
// Examples: external:axiom1, external:ZFC-axiom, external:prime_theorem
var extCitePattern = regexp.MustCompile(`external:([a-zA-Z0-9][a-zA-Z0-9_-]*)`)

// ParseExtCitations extracts all external:NAME citations from the given text.
// Returns a slice of unique external names in the order they first appear.
// Returns nil if no citations are found.
func ParseExtCitations(text string) []string {
	if text == "" {
		return nil
	}

	matches := extCitePattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	// Deduplicate while preserving order
	seen := make(map[string]bool)
	var result []string
	for _, match := range matches {
		name := match[1] // capture group 1 contains the external name
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}

	return result
}

// ValidateExtCitations validates that all external:NAME citations in the statement
// reference externals that exist in the current state.
//
// Returns nil if all citations are valid or if there are no citations.
// Returns EXTERNAL_NOT_FOUND error if any cited external is not found.
func ValidateExtCitations(statement string, st *state.State) error {
	citations := ParseExtCitations(statement)
	if len(citations) == 0 {
		return nil
	}

	// Cannot validate citations without state
	if st == nil {
		return errors.Newf(errors.EXTERNAL_NOT_FOUND, "cannot validate citations: state is nil")
	}

	// Check each cited external exists
	for _, name := range citations {
		ext := st.GetExternalByName(name)
		if ext == nil {
			return errors.Newf(errors.EXTERNAL_NOT_FOUND, "external %q not found", name)
		}
	}

	return nil
}

// CollectMissingExtCitations returns all external:NAME citations that reference
// externals not present in the current state.
// Returns nil if all citations are valid or if there are no citations.
// This is useful for error reporting when multiple externals are missing.
func CollectMissingExtCitations(statement string, st *state.State) []string {
	citations := ParseExtCitations(statement)
	if len(citations) == 0 || st == nil {
		return nil
	}

	var missing []string
	for _, name := range citations {
		ext := st.GetExternalByName(name)
		if ext == nil {
			missing = append(missing, name)
		}
	}

	return missing
}
