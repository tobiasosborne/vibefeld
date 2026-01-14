// Package lemma provides validation for lemma and citation references.
package lemma

import (
	"regexp"

	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/state"
)

// defCitePattern matches def:NAME citations in text.
// NAME can contain alphanumeric characters, hyphens, and underscores.
// Examples: def:group, def:Stirling-second-kind, def:vector_space
var defCitePattern = regexp.MustCompile(`def:([a-zA-Z0-9][a-zA-Z0-9_-]*)`)

// ParseDefCitations extracts all def:NAME citations from the given text.
// Returns a slice of unique definition names in the order they first appear.
// Returns nil if no citations are found.
func ParseDefCitations(text string) []string {
	if text == "" {
		return nil
	}

	matches := defCitePattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	// Deduplicate while preserving order
	seen := make(map[string]bool)
	var result []string
	for _, match := range matches {
		name := match[1] // capture group 1 contains the definition name
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}

	return result
}

// ValidateDefCitations validates that all def:NAME citations in the statement
// reference definitions that exist in the current state.
//
// Returns nil if all citations are valid or if there are no citations.
// Returns DEF_NOT_FOUND error if any cited definition is not found.
func ValidateDefCitations(statement string, st *state.State) error {
	citations := ParseDefCitations(statement)
	if len(citations) == 0 {
		return nil
	}

	// Cannot validate citations without state
	if st == nil {
		return errors.Newf(errors.DEF_NOT_FOUND, "cannot validate citations: state is nil")
	}

	// Check each cited definition exists
	for _, name := range citations {
		def := st.GetDefinitionByName(name)
		if def == nil {
			return errors.Newf(errors.DEF_NOT_FOUND, "definition %q not found", name)
		}
	}

	return nil
}

// CollectMissingDefCitations returns all def:NAME citations that reference
// definitions not present in the current state.
// Returns nil if all citations are valid or if there are no citations.
// This is useful for error reporting when multiple definitions are missing.
func CollectMissingDefCitations(statement string, st *state.State) []string {
	citations := ParseDefCitations(statement)
	if len(citations) == 0 || st == nil {
		return nil
	}

	var missing []string
	for _, name := range citations {
		def := st.GetDefinitionByName(name)
		if def == nil {
			missing = append(missing, name)
		}
	}

	return missing
}
