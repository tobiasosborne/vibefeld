// Package cli provides utilities for CLI argument parsing and user interaction.
package cli

import (
	"fmt"
	"strings"

	"github.com/tobias/vibefeld/internal/fuzzy"
)

// FuzzyFlagResult contains the result of fuzzy matching a single flag.
type FuzzyFlagResult struct {
	Input       string   // Original input (e.g., "--ownr")
	Match       string   // Best match flag name without dashes (e.g., "owner")
	AutoCorrect bool     // True if confident enough to auto-correct
	Suggestions []string // Alternative suggestions when not auto-correcting
	IsFlag      bool     // True if input looks like a flag (starts with -)
	Value       string   // Value part if input was --flag=value syntax
}

// String returns a human-readable description of the result.
func (r FuzzyFlagResult) String() string {
	if r.AutoCorrect && r.Match != "" {
		return fmt.Sprintf("auto-corrected %s to --%s", r.Input, r.Match)
	}
	if len(r.Suggestions) > 0 {
		suggestions := make([]string, len(r.Suggestions))
		for i, s := range r.Suggestions {
			suggestions[i] = "--" + s
		}
		return fmt.Sprintf("ambiguous flag %s, did you mean: %s?", r.Input, strings.Join(suggestions, ", "))
	}
	return fmt.Sprintf("unknown flag: %s", r.Input)
}

// FlagCorrection records a single flag correction made during processing.
type FlagCorrection struct {
	Original  string // Original flag (e.g., "--ownr")
	Corrected string // Corrected flag (e.g., "--owner")
}

// AmbiguousFlag records a flag with multiple possible matches.
type AmbiguousFlag struct {
	Input       string   // Original input
	Suggestions []string // Possible flag names
}

// FuzzyFlagsResult contains the result of processing multiple flags.
type FuzzyFlagsResult struct {
	CorrectedArgs []string         // Args with corrections applied
	Corrections   []FlagCorrection // List of corrections made
	Ambiguous     []AmbiguousFlag  // Flags that couldn't be auto-corrected
	Errors        []string         // Any errors encountered
}

// FuzzyMatchFlag matches a user-provided flag against known flags using fuzzy matching.
// Returns the best match if confident enough for auto-correction, otherwise returns suggestions.
//
// The input can be in any of these formats:
//   - "--flag" (long flag)
//   - "-f" (short flag)
//   - "--flag=value" (flag with inline value)
//   - "flag" (flag name without dashes)
func FuzzyMatchFlag(input string, knownFlags []string) FuzzyFlagResult {
	result := FuzzyFlagResult{
		Input: input,
	}

	// Handle empty input
	if input == "" {
		return result
	}

	// Handle nil known flags
	if knownFlags == nil {
		knownFlags = []string{}
	}

	// Extract flag name and determine if it's a flag
	flagName, value, isFlag := extractFlagNameForFuzzy(input)
	result.IsFlag = isFlag
	result.Value = value

	// If we couldn't extract a flag name, return empty result
	if flagName == "" {
		return result
	}

	// If no known flags, return result with just the parsed info
	if len(knownFlags) == 0 {
		return result
	}

	// Use fuzzy matching from the fuzzy package
	matchResult := fuzzy.SuggestFlag(flagName, knownFlags)

	result.Match = matchResult.Match
	result.AutoCorrect = matchResult.AutoCorrect

	// Only include suggestions if we're not auto-correcting and there are close matches
	if !matchResult.AutoCorrect && len(matchResult.Suggestions) > 0 {
		result.Suggestions = matchResult.Suggestions
	}

	return result
}

// extractFlagNameForFuzzy extracts the flag name from an argument.
// Similar to ExtractFlagName but handles the case where input might not have dashes.
// Returns the name (without leading dashes), any inline value (for --flag=value syntax),
// and whether the argument looks like a flag.
func extractFlagNameForFuzzy(arg string) (name string, value string, isFlag bool) {
	// Handle empty or just dashes
	if arg == "" || arg == "-" || arg == "--" {
		return "", "", false
	}

	// Check if it starts with dashes
	if strings.HasPrefix(arg, "--") {
		isFlag = true
		arg = arg[2:]
	} else if strings.HasPrefix(arg, "-") {
		isFlag = true
		arg = arg[1:]
	}

	// Handle empty after stripping dashes
	if arg == "" {
		return "", "", false
	}

	// Check for equals sign
	if idx := strings.Index(arg, "="); idx >= 0 {
		return arg[:idx], arg[idx+1:], isFlag
	}

	return arg, "", isFlag
}

// FuzzyMatchFlags processes multiple flags with fuzzy matching.
// It corrects typos when confident, collects ambiguous flags, and preserves
// the structure of the arguments.
//
// Double dash (--) terminates flag processing - everything after is preserved as-is.
func FuzzyMatchFlags(args []string, knownFlags []string) FuzzyFlagsResult {
	result := FuzzyFlagsResult{
		CorrectedArgs: make([]string, 0, len(args)),
		Corrections:   make([]FlagCorrection, 0),
		Ambiguous:     make([]AmbiguousFlag, 0),
		Errors:        make([]string, 0),
	}

	if args == nil {
		return result
	}

	if knownFlags == nil {
		knownFlags = []string{}
	}

	// Build a set of known flags for quick lookup
	knownSet := make(map[string]bool)
	for _, f := range knownFlags {
		knownSet[f] = true
	}

	flagsTerminated := false

	for _, arg := range args {
		// Double dash terminates flag parsing
		if arg == "--" {
			flagsTerminated = true
			result.CorrectedArgs = append(result.CorrectedArgs, arg)
			continue
		}

		// After --, preserve everything as-is
		if flagsTerminated {
			result.CorrectedArgs = append(result.CorrectedArgs, arg)
			continue
		}

		// Check if this looks like a flag
		if !IsFlag(arg) {
			result.CorrectedArgs = append(result.CorrectedArgs, arg)
			continue
		}

		// Try fuzzy matching
		matchResult := FuzzyMatchFlag(arg, knownFlags)

		// If exact match, keep as-is
		flagName, value, _ := extractFlagNameForFuzzy(arg)
		if knownSet[flagName] {
			result.CorrectedArgs = append(result.CorrectedArgs, arg)
			continue
		}

		// If auto-correct is confident
		if matchResult.AutoCorrect && matchResult.Match != "" {
			// Build the corrected flag
			var corrected string
			if strings.HasPrefix(arg, "--") {
				if value != "" {
					corrected = "--" + matchResult.Match + "=" + value
				} else if strings.Contains(arg, "=") {
					// Handle --flag= case (empty value)
					corrected = "--" + matchResult.Match + "="
				} else {
					corrected = "--" + matchResult.Match
				}
			} else if strings.HasPrefix(arg, "-") {
				corrected = "-" + matchResult.Match
			} else {
				corrected = matchResult.Match
			}

			result.CorrectedArgs = append(result.CorrectedArgs, corrected)
			result.Corrections = append(result.Corrections, FlagCorrection{
				Original:  arg,
				Corrected: corrected,
			})
			continue
		}

		// If we have suggestions but not confident enough to auto-correct
		if len(matchResult.Suggestions) > 0 && !matchResult.AutoCorrect {
			result.Ambiguous = append(result.Ambiguous, AmbiguousFlag{
				Input:       arg,
				Suggestions: matchResult.Suggestions,
			})
		}

		// Keep the original arg (don't change unknown flags)
		result.CorrectedArgs = append(result.CorrectedArgs, arg)
	}

	return result
}
