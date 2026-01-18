package fuzzy

import (
	"reflect"
	"testing"
)

// Standard CLI commands for testing
var cliCommands = []string{
	"init", "status", "claim", "release", "refine", "accept",
	"challenge", "validate", "admit", "refute", "archive",
	"jobs", "def", "assume", "lemma", "help", "version",
}

func TestMatch_ExactMatch(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		candidates []string
		threshold  float64
		want       MatchResult
	}{
		{
			name:       "exact match status",
			input:      "status",
			candidates: []string{"status", "init"},
			threshold:  0.8,
			want: MatchResult{
				Input:       "status",
				Match:       "status",
				Distance:    0,
				AutoCorrect: true,
				Suggestions: []string{"status"},
			},
		},
		{
			name:       "exact match init",
			input:      "init",
			candidates: []string{"status", "init", "claim"},
			threshold:  0.8,
			want: MatchResult{
				Input:       "init",
				Match:       "init",
				Distance:    0,
				AutoCorrect: true,
				Suggestions: []string{"init"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.input, tt.candidates, tt.threshold)
			if !matchResultEqual(got, tt.want) {
				t.Errorf("Match() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMatch_CloseMatch(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		candidates []string
		threshold  float64
		wantMatch  string
		wantDist   int
	}{
		{
			name:       "stauts -> status",
			input:      "stauts",
			candidates: []string{"status", "init", "claim"},
			threshold:  0.8,
			wantMatch:  "status",
			wantDist:   2, // swap 'u' and 't'
		},
		{
			name:       "chalenge -> challenge",
			input:      "chalenge",
			candidates: cliCommands,
			threshold:  0.8,
			wantMatch:  "challenge",
			wantDist:   1, // missing 'l'
		},
		{
			name:       "refin -> refine",
			input:      "refin",
			candidates: []string{"refine", "refute", "release"},
			threshold:  0.8,
			wantMatch:  "refine",
			wantDist:   1, // missing 'e'
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.input, tt.candidates, tt.threshold)
			if got.Match != tt.wantMatch {
				t.Errorf("Match() Match = %v, want %v", got.Match, tt.wantMatch)
			}
			if got.Distance != tt.wantDist {
				t.Errorf("Match() Distance = %v, want %v", got.Distance, tt.wantDist)
			}
			if got.Input != tt.input {
				t.Errorf("Match() Input = %v, want %v", got.Input, tt.input)
			}
		})
	}
}

func TestMatch_MultipleClose(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		candidates []string
		threshold  float64
		wantMatch  string
		wantMinLen int // minimum suggestions
	}{
		{
			name:       "stat matches multiple",
			input:      "stat",
			candidates: []string{"status", "state", "start"},
			threshold:  0.7,
			wantMatch:  "start", // closest: 4->5 letters, distance 1
			wantMinLen: 2,       // should have at least 2 suggestions
		},
		{
			name:       "ref matches multiple",
			input:      "ref",
			candidates: []string{"refine", "refute", "release"},
			threshold:  0.7,
			wantMinLen: 2, // multiple close matches
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.input, tt.candidates, tt.threshold)
			if tt.wantMatch != "" && got.Match != tt.wantMatch {
				t.Errorf("Match() Match = %v, want %v", got.Match, tt.wantMatch)
			}
			if len(got.Suggestions) < tt.wantMinLen {
				t.Errorf("Match() Suggestions length = %v, want at least %v, got %v",
					len(got.Suggestions), tt.wantMinLen, got.Suggestions)
			}
		})
	}
}

func TestMatch_NoMatch(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		candidates []string
		threshold  float64
	}{
		{
			name:       "xyz no match",
			input:      "xyz",
			candidates: []string{"status", "init"},
			threshold:  0.8,
		},
		{
			name:       "completely different",
			input:      "foobar",
			candidates: []string{"init", "status", "claim"},
			threshold:  0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.input, tt.candidates, tt.threshold)
			if got.Match != "" {
				t.Errorf("Match() Match = %v, want empty for no match", got.Match)
			}
			if got.AutoCorrect {
				t.Errorf("Match() AutoCorrect = true, want false for no match")
			}
		})
	}
}

func TestMatch_EmptyInput(t *testing.T) {
	got := Match("", []string{"status", "init"}, 0.8)
	if got.Match != "" {
		t.Errorf("Match() with empty input Match = %v, want empty", got.Match)
	}
	if got.AutoCorrect {
		t.Errorf("Match() with empty input AutoCorrect = true, want false")
	}
	if got.Input != "" {
		t.Errorf("Match() Input = %v, want empty", got.Input)
	}
}

func TestMatch_EmptyCandidates(t *testing.T) {
	got := Match("status", []string{}, 0.8)
	if got.Match != "" {
		t.Errorf("Match() with empty candidates Match = %v, want empty", got.Match)
	}
	if got.AutoCorrect {
		t.Errorf("Match() with empty candidates AutoCorrect = true, want false")
	}
	if len(got.Suggestions) != 0 {
		t.Errorf("Match() with empty candidates Suggestions = %v, want empty slice", got.Suggestions)
	}
}

func TestMatch_CaseSensitive(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		candidates []string
		threshold  float64
		wantMatch  string
		wantExact  bool // true if should be exact match
	}{
		{
			name:       "STATUS vs status",
			input:      "STATUS",
			candidates: []string{"status", "init"},
			threshold:  0.8,
			wantMatch:  "", // case sensitive, so no exact match
			wantExact:  false,
		},
		{
			name:       "Init vs init",
			input:      "Init",
			candidates: []string{"init", "status"},
			threshold:  0.8,
			wantMatch:  "", // case sensitive
			wantExact:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.input, tt.candidates, tt.threshold)
			if tt.wantExact {
				if got.Distance != 0 {
					t.Errorf("Match() Distance = %v, want 0 for exact match", got.Distance)
				}
			}
			// For case mismatch, distance should be non-zero (each char replacement = 1)
			if !tt.wantExact && got.Distance == 0 && got.Match != "" {
				t.Errorf("Match() Distance = 0 for case mismatch, should be > 0")
			}
		})
	}
}

func TestMatch_Threshold_High(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		candidates []string
		threshold  float64
		wantAuto   bool
	}{
		{
			name:       "strict threshold rejects small typo",
			input:      "initt", // distance 1 from "init" (4->5 chars)
			candidates: []string{"init", "status"},
			threshold:  0.9, // very strict
			wantAuto:   false,
		},
		{
			name:       "strict threshold rejects st -> status",
			input:      "st",
			candidates: []string{"status", "start"},
			threshold:  0.9,
			wantAuto:   false, // too short
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.input, tt.candidates, tt.threshold)
			if got.AutoCorrect != tt.wantAuto {
				t.Errorf("Match() AutoCorrect = %v, want %v (threshold=%.1f)",
					got.AutoCorrect, tt.wantAuto, tt.threshold)
			}
		})
	}
}

func TestMatch_Threshold_Low(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		candidates []string
		threshold  float64
		wantAuto   bool
	}{
		{
			name:       "loose threshold accepts typo",
			input:      "sttus", // distance 2 from "status"
			candidates: []string{"status", "init"},
			threshold:  0.5, // loose
			wantAuto:   true,
		},
		{
			name:       "ambiguous prefix does not autocorrect",
			input:      "sta",
			candidates: []string{"status", "start"},
			threshold:  0.4,
			wantAuto:   false, // "sta" is prefix of both "status" and "start" - ambiguous
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.input, tt.candidates, tt.threshold)
			if got.AutoCorrect != tt.wantAuto {
				t.Errorf("Match() AutoCorrect = %v, want %v (threshold=%.1f)",
					got.AutoCorrect, tt.wantAuto, tt.threshold)
			}
		})
	}
}

func TestSuggestCommand_Typo(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMatch string
	}{
		{
			name:      "chalenge -> challenge",
			input:     "chalenge",
			wantMatch: "challenge",
		},
		{
			name:      "stauts -> status",
			input:     "stauts",
			wantMatch: "status",
		},
		{
			name:      "valdate -> validate",
			input:     "valdate",
			wantMatch: "validate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestCommand(tt.input, cliCommands)
			if got.Match != tt.wantMatch {
				t.Errorf("SuggestCommand() Match = %v, want %v", got.Match, tt.wantMatch)
			}
		})
	}
}

func TestSuggestCommand_Prefix(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantContains  string // Match should contain this
		wantMinSuggs  int    // minimum suggestions
	}{
		{
			name:         "ref prefix",
			input:        "ref",
			wantContains: "ref", // should match refine, refute, etc.
			wantMinSuggs: 1,
		},
		{
			name:         "cha prefix",
			input:        "cha",
			wantContains: "cha", // should match challenge, claim
			wantMinSuggs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestCommand(tt.input, cliCommands)
			// With prefix matching, we should get some match or suggestions
			if got.Match == "" && len(got.Suggestions) < tt.wantMinSuggs {
				t.Errorf("SuggestCommand() no match and suggestions = %v, want at least %v suggestions",
					got.Suggestions, tt.wantMinSuggs)
			}
		})
	}
}

func TestSuggestCommand_Multiple(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMin  int // minimum suggestions
	}{
		{
			name:    "multiple close matches for 're'",
			input:   "re",
			wantMin: 2, // refine, refute, release are all close
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestCommand(tt.input, cliCommands)
			if len(got.Suggestions) < tt.wantMin {
				t.Errorf("SuggestCommand() Suggestions = %v, want at least %v",
					len(got.Suggestions), tt.wantMin)
			}
		})
	}
}

func TestAutoCorrect_True(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		threshold float64
	}{
		{
			name:      "close typo with default threshold",
			input:     "statu", // distance 1 from "status"
			threshold: 0.8,
		},
		{
			name:      "exact match always auto-corrects",
			input:     "status",
			threshold: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.input, []string{"status", "init"}, tt.threshold)
			if !got.AutoCorrect {
				t.Errorf("Match() AutoCorrect = false, want true for close match")
			}
		})
	}
}

func TestAutoCorrect_False(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		threshold float64
	}{
		{
			name:      "distant match",
			input:     "xyz",
			threshold: 0.8,
		},
		{
			name:      "too many edits",
			input:     "sttttus",
			threshold: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.input, []string{"status", "init"}, tt.threshold)
			if got.AutoCorrect {
				t.Errorf("Match() AutoCorrect = true, want false for distant match")
			}
		})
	}
}

// Test that suggestions are ordered by distance (closest first)
func TestMatch_SuggestionsOrdered(t *testing.T) {
	input := "stat"
	candidates := []string{"status", "state", "start", "init"}
	got := Match(input, candidates, 0.6)

	if len(got.Suggestions) < 2 {
		t.Fatalf("Match() Suggestions length = %v, want at least 2", len(got.Suggestions))
	}

	// Verify suggestions are ordered by increasing distance
	for i := 0; i < len(got.Suggestions)-1; i++ {
		dist1 := Distance(input, got.Suggestions[i])
		dist2 := Distance(input, got.Suggestions[i+1])
		if dist1 > dist2 {
			t.Errorf("Match() Suggestions not ordered: %q (dist=%d) before %q (dist=%d)",
				got.Suggestions[i], dist1, got.Suggestions[i+1], dist2)
		}
	}
}

// Test threshold boundary conditions
func TestMatch_ThresholdBoundary(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		candidate string
		threshold float64
		wantAuto  bool
	}{
		{
			name:      "exactly at threshold",
			input:     "stat", // 4 chars
			candidate: "start", // 5 chars, distance 1
			threshold: 0.8,     // similarity = 1 - (1/5) = 0.8
			wantAuto:  true,
		},
		{
			name:      "just below threshold (non-prefix)",
			input:     "xy",    // 2 chars, NOT a prefix of target
			candidate: "start", // 5 chars, distance 5 (all different)
			threshold: 0.5,     // similarity = 1 - (5/5) = 0 < 0.5
			wantAuto:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.input, []string{tt.candidate}, tt.threshold)
			if got.AutoCorrect != tt.wantAuto {
				t.Errorf("Match() AutoCorrect = %v, want %v (input=%q, candidate=%q, threshold=%.1f)",
					got.AutoCorrect, tt.wantAuto, tt.input, tt.candidate, tt.threshold)
			}
		})
	}
}

// Helper function to compare MatchResults
func matchResultEqual(a, b MatchResult) bool {
	return a.Input == b.Input &&
		a.Match == b.Match &&
		a.Distance == b.Distance &&
		a.AutoCorrect == b.AutoCorrect &&
		reflect.DeepEqual(a.Suggestions, b.Suggestions)
}

// Standard CLI flags for testing
var cliFlags = []string{
	"help", "version", "verbose", "format", "output", "dir", "config",
	"quiet", "debug", "json", "force", "recursive", "dry-run", "all",
}

func TestSuggestFlag_Typo(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMatch string
	}{
		{
			name:      "verbos -> verbose",
			input:     "verbos",
			wantMatch: "verbose",
		},
		{
			name:      "formt -> format",
			input:     "formt",
			wantMatch: "format",
		},
		{
			name:      "ouput -> output",
			input:     "ouput",
			wantMatch: "output",
		},
		{
			name:      "confg -> config",
			input:     "confg",
			wantMatch: "config",
		},
		{
			name:      "dirr -> dir",
			input:     "dirr",
			wantMatch: "dir",
		},
		{
			name:      "qiet -> quiet",
			input:     "qiet",
			wantMatch: "quiet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestFlag(tt.input, cliFlags)
			if got.Match != tt.wantMatch {
				t.Errorf("SuggestFlag() Match = %v, want %v", got.Match, tt.wantMatch)
			}
		})
	}
}

func TestSuggestFlag_Prefix(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains string // Match should contain this prefix
		wantMinSuggs int    // minimum suggestions
	}{
		{
			name:         "ver prefix",
			input:        "ver",
			wantContains: "ver", // should match verbose, version
			wantMinSuggs: 1,
		},
		{
			name:         "for prefix",
			input:        "for",
			wantContains: "for", // should match format, force
			wantMinSuggs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestFlag(tt.input, cliFlags)
			// With prefix matching, we should get some match or suggestions
			if got.Match == "" && len(got.Suggestions) < tt.wantMinSuggs {
				t.Errorf("SuggestFlag() no match and suggestions = %v, want at least %v suggestions",
					got.Suggestions, tt.wantMinSuggs)
			}
		})
	}
}

func TestSuggestFlag_NoMatch(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "completely unrelated",
			input: "xyz123",
		},
		{
			name:  "gibberish",
			input: "asdfghjkl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestFlag(tt.input, cliFlags)
			if got.Match != "" {
				t.Errorf("SuggestFlag() Match = %v, want empty for no match", got.Match)
			}
			if got.AutoCorrect {
				t.Errorf("SuggestFlag() AutoCorrect = true, want false for no match")
			}
		})
	}
}

func TestSuggestFlag_ShortInput(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		flags        []string
		wantMatch    string
		wantMinSuggs int
		wantAuto     bool
	}{
		{
			name:         "single char 'a' matches 'all'",
			input:        "a",
			flags:        []string{"all", "help", "version"},
			wantMatch:    "all",
			wantMinSuggs: 1,
			wantAuto:     true, // with lowered threshold for short inputs
		},
		{
			name:         "single char 'h' matches 'help' (prefix)",
			input:        "h",
			flags:        []string{"all", "help", "version"},
			wantMatch:    "help",
			wantMinSuggs: 1,
			wantAuto:     true, // lowered threshold 0.3 for short inputs, prefix match allows it
		},
		{
			name:         "two chars 'al' matches 'all'",
			input:        "al",
			flags:        []string{"all", "help", "version"},
			wantMatch:    "all",
			wantMinSuggs: 1,
			wantAuto:     true,
		},
		{
			name:         "three chars 'ver' is ambiguous",
			input:        "ver",
			flags:        []string{"verbose", "version", "help"},
			wantMatch:    "verbose", // alphabetically first among prefix matches
			wantMinSuggs: 2,         // both verbose and version
			wantAuto:     false,     // "ver" is prefix of both verbose and version - ambiguous
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestFlag(tt.input, tt.flags)
			if got.Match != tt.wantMatch {
				t.Errorf("SuggestFlag(%q) Match = %q, want %q", tt.input, got.Match, tt.wantMatch)
			}
			if len(got.Suggestions) < tt.wantMinSuggs {
				t.Errorf("SuggestFlag(%q) Suggestions = %v, want at least %d",
					tt.input, got.Suggestions, tt.wantMinSuggs)
			}
			if got.AutoCorrect != tt.wantAuto {
				t.Errorf("SuggestFlag(%q) AutoCorrect = %v, want %v",
					tt.input, got.AutoCorrect, tt.wantAuto)
			}
		})
	}
}

// BenchmarkFuzzyMatchCommands benchmarks fuzzy matching against CLI commands.
// Tests 50 commands with 10 misspellings each.
func BenchmarkFuzzyMatchCommands(b *testing.B) {
	// Expanded command list (50 commands)
	commands := []string{
		"init", "status", "claim", "release", "refine", "accept",
		"challenge", "validate", "admit", "refute", "archive",
		"jobs", "def", "assume", "lemma", "help", "version",
		"get", "defs", "challenges", "deps", "scope", "log",
		"replay", "reap", "recompute-taint", "def-add", "def-reject",
		"extract-lemma", "export", "progress", "health", "pending-defs",
		"pending-refs", "assumptions", "assumption", "schema", "inferences",
		"types", "extend-claim", "resolve-challenge", "withdraw-challenge",
		"amend", "search", "lemmas", "externals", "agents", "verify-ref",
		"add-external", "completion", "watch",
	}

	// Common typos/misspellings
	misspellings := []string{
		"stauts", "chalenge", "refien", "accpet", "archve",
		"relase", "valdate", "admitt", "refut", "initt",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, typo := range misspellings {
			_ = SuggestCommand(typo, commands)
		}
	}
}

// BenchmarkFuzzyMatchVaryingCandidates benchmarks with different candidate counts.
func BenchmarkFuzzyMatchVaryingCandidates(b *testing.B) {
	benchmarks := []struct {
		name       string
		candidates int
	}{
		{"10_candidates", 10},
		{"50_candidates", 50},
		{"100_candidates", 100},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Generate candidate list
			candidates := make([]string, bm.candidates)
			for i := 0; i < bm.candidates; i++ {
				candidates[i] = generateCommandName(i)
			}

			input := "stauts" // common typo

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = Match(input, candidates, 0.8)
			}
		})
	}
}

// BenchmarkFuzzyMatchVaryingInputLength benchmarks with different input lengths.
func BenchmarkFuzzyMatchVaryingInputLength(b *testing.B) {
	candidates := []string{
		"init", "status", "challenge", "validate", "accept",
		"refine", "refute", "release", "archive", "admit",
	}

	benchmarks := []struct {
		name  string
		input string
	}{
		{"short_3char", "sta"},
		{"medium_6char", "stauts"},
		{"long_10char", "challlenge"},
		{"exact_9char", "challenge"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = Match(bm.input, candidates, 0.8)
			}
		})
	}
}

// generateCommandName generates a unique command name for benchmarking.
func generateCommandName(i int) string {
	prefixes := []string{"get", "set", "add", "del", "run", "show", "list", "find", "new", "old"}
	suffixes := []string{"node", "item", "task", "job", "def", "ref", "log", "tree", "all", "one"}
	return prefixes[i%len(prefixes)] + "-" + suffixes[(i/len(prefixes))%len(suffixes)]
}
