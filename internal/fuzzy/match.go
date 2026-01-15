package fuzzy

import "sort"

// MatchResult contains the result of fuzzy matching.
type MatchResult struct {
	Input       string
	Match       string   // Best match (empty if none)
	Distance    int      // Edit distance to best match
	AutoCorrect bool     // True if close enough to auto-correct
	Suggestions []string // Other close matches
}

// candidateScore holds a candidate and its distance for sorting.
type candidateScore struct {
	candidate string
	distance  int
}

// Match finds the best match for input among candidates.
// threshold is a similarity ratio (0.0-1.0) - higher means stricter matching.
// AutoCorrect is true if similarity >= threshold.
// Similarity is calculated as: 1 - (distance / max(len(input), len(match)))
func Match(input string, candidates []string, threshold float64) MatchResult {
	// Handle edge cases
	if input == "" || len(candidates) == 0 {
		return MatchResult{
			Input:       input,
			Match:       "",
			Distance:    0,
			AutoCorrect: false,
			Suggestions: []string{},
		}
	}

	// Calculate distances for all candidates
	scores := make([]candidateScore, 0, len(candidates))
	for _, c := range candidates {
		dist := Distance(input, c)
		scores = append(scores, candidateScore{candidate: c, distance: dist})
	}

	// Sort by distance (ascending), then alphabetically for stable ordering
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].distance != scores[j].distance {
			return scores[i].distance < scores[j].distance
		}
		return scores[i].candidate < scores[j].candidate
	})

	// Find best match
	best := scores[0]

	// Calculate similarity for the best match
	maxLen := len(input)
	if len(best.candidate) > maxLen {
		maxLen = len(best.candidate)
	}
	similarity := 1.0 - float64(best.distance)/float64(maxLen)

	// Determine if we should autocorrect
	autoCorrect := similarity >= threshold

	// If similarity is too low, return no match
	// Use a minimum similarity threshold of 0.3 to filter out completely unrelated matches
	if similarity < 0.3 {
		return MatchResult{
			Input:       input,
			Match:       "",
			Distance:    0,
			AutoCorrect: false,
			Suggestions: []string{},
		}
	}

	// Collect suggestions: all candidates within reasonable distance
	// Include candidates where the input is a prefix, or within a reasonable edit distance
	// For very short inputs, be more lenient to allow prefix matching
	maxSuggestionDistance := len(input) + 3 // allow up to 3 extra chars beyond input length
	if maxSuggestionDistance < 4 {
		maxSuggestionDistance = 4
	}

	suggestions := make([]string, 0, len(candidates))
	for _, s := range scores {
		// Include if distance is small enough relative to input length
		// This allows prefix matching (e.g., "re" -> "refine", "refute", "release")
		if s.distance <= maxSuggestionDistance {
			maxL := len(input)
			if len(s.candidate) > maxL {
				maxL = len(s.candidate)
			}
			sim := 1.0 - float64(s.distance)/float64(maxL)
			// Use a more lenient threshold for suggestions
			suggestionThreshold := threshold - 0.5
			if suggestionThreshold < 0.1 {
				suggestionThreshold = 0.1
			}
			if sim >= suggestionThreshold {
				suggestions = append(suggestions, s.candidate)
			}
		}
	}

	return MatchResult{
		Input:       input,
		Match:       best.candidate,
		Distance:    best.distance,
		AutoCorrect: autoCorrect,
		Suggestions: suggestions,
	}
}

// SuggestCommand is a convenience wrapper with default threshold 0.8.
func SuggestCommand(input string, commands []string) MatchResult {
	return Match(input, commands, 0.8)
}

// SuggestFlag suggests similar flags for a mistyped flag name.
// It handles both long flags (--foo) and short flags (-f).
// The threshold is slightly lower (0.7) than commands to be more forgiving.
func SuggestFlag(input string, flags []string) MatchResult {
	return Match(input, flags, 0.7)
}
