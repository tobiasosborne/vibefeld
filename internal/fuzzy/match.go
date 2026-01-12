package fuzzy

// MatchResult contains the result of fuzzy matching.
type MatchResult struct {
	Input       string
	Match       string   // Best match (empty if none)
	Distance    int      // Edit distance to best match
	AutoCorrect bool     // True if close enough to auto-correct
	Suggestions []string // Other close matches
}

// Match finds the best match for input among candidates.
// threshold is a similarity ratio (0.0-1.0) - higher means stricter matching.
// AutoCorrect is true if similarity >= threshold.
// Similarity is calculated as: 1 - (distance / max(len(input), len(match)))
func Match(input string, candidates []string, threshold float64) MatchResult {
	// TODO: implement
	return MatchResult{
		Input:       input,
		Match:       "",
		Distance:    0,
		AutoCorrect: false,
		Suggestions: nil,
	}
}

// SuggestCommand is a convenience wrapper with default threshold 0.8.
func SuggestCommand(input string, commands []string) MatchResult {
	return Match(input, commands, 0.8)
}
