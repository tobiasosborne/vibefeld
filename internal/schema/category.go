package schema

import (
	"fmt"
	"sort"
)

// ChallengeCategory is a typed, machine-readable classification of what kind of
// objection a challenge raises. Unlike severity (which gates acceptance), the
// category is purely descriptive: it lets tooling classify challenges exactly
// instead of grepping the free-text reason. The category is optional — an empty
// value means "uncategorised".
type ChallengeCategory string

// Valid challenge categories.
const (
	// CategoryGap: an unjustified logical step or genuine gap in the reasoning.
	CategoryGap ChallengeCategory = "gap"

	// CategoryMissing: a required fact, citation, definition, or lemma is absent.
	CategoryMissing ChallengeCategory = "missing"

	// CategoryDependency: a dependency/DAG issue (a required dependency is
	// missing, unvalidated, or wrong).
	CategoryDependency ChallengeCategory = "dependency"

	// CategoryIncorrect: the statement or inference is incorrect / false.
	CategoryIncorrect ChallengeCategory = "incorrect"

	// CategoryUnclear: the statement is ambiguous or unclear.
	CategoryUnclear ChallengeCategory = "unclear"

	// CategoryOther: a well-formed objection that fits none of the above.
	CategoryOther ChallengeCategory = "other"
)

// ChallengeCategoryInfo provides metadata about a challenge category.
type ChallengeCategoryInfo struct {
	ID          ChallengeCategory
	Description string
}

// challengeCategoryRegistry maps each valid category to its metadata.
var challengeCategoryRegistry = map[ChallengeCategory]ChallengeCategoryInfo{
	CategoryGap:        {ID: CategoryGap, Description: "Unjustified logical step or genuine gap in the reasoning"},
	CategoryMissing:    {ID: CategoryMissing, Description: "Required fact, citation, definition, or lemma is absent"},
	CategoryDependency: {ID: CategoryDependency, Description: "Dependency/DAG issue (missing, unvalidated, or wrong dependency)"},
	CategoryIncorrect:  {ID: CategoryIncorrect, Description: "Statement or inference is incorrect or false"},
	CategoryUnclear:    {ID: CategoryUnclear, Description: "Statement is ambiguous or unclear"},
	CategoryOther:      {ID: CategoryOther, Description: "Well-formed objection that fits none of the other categories"},
}

// ValidateChallengeCategory validates a challenge category string. An empty
// string is valid (the category is optional and defaults to uncategorised).
// A non-empty value must be one of the registered categories.
func ValidateChallengeCategory(s string) error {
	if s == "" {
		return nil
	}
	if _, exists := challengeCategoryRegistry[ChallengeCategory(s)]; !exists {
		return fmt.Errorf("invalid challenge category: %q", s)
	}
	return nil
}

// GetChallengeCategoryInfo returns metadata for a given challenge category.
// The boolean return value indicates whether the category exists.
func GetChallengeCategoryInfo(c ChallengeCategory) (ChallengeCategoryInfo, bool) {
	info, exists := challengeCategoryRegistry[c]
	return info, exists
}

// AllChallengeCategories returns all valid challenge categories with their
// metadata, sorted by ID for deterministic output.
func AllChallengeCategories() []ChallengeCategoryInfo {
	result := make([]ChallengeCategoryInfo, 0, len(challengeCategoryRegistry))
	for _, info := range challengeCategoryRegistry {
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// ValidChallengeCategoryStrings returns the valid category IDs as a sorted
// slice of strings, useful for building CLI error messages.
func ValidChallengeCategoryStrings() []string {
	infos := AllChallengeCategories()
	out := make([]string, 0, len(infos))
	for _, info := range infos {
		out = append(out, string(info.ID))
	}
	return out
}
