package cli

import (
	"reflect"
	"testing"
)

// Standard CLI flags for testing
var testKnownFlags = []string{
	"owner", "output", "format", "verbose", "quiet", "force",
	"json", "help", "version", "config", "recursive", "dry-run",
}

func TestFuzzyMatchFlag_ExactMatch(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		knownFlags []string
		want       FuzzyFlagResult
	}{
		{
			name:       "exact match owner",
			input:      "--owner",
			knownFlags: testKnownFlags,
			want: FuzzyFlagResult{
				Input:       "--owner",
				Match:       "owner",
				AutoCorrect: true,
				Suggestions: nil,
				IsFlag:      true,
			},
		},
		{
			name:       "exact match without dashes",
			input:      "owner",
			knownFlags: testKnownFlags,
			want: FuzzyFlagResult{
				Input:       "owner",
				Match:       "owner",
				AutoCorrect: true,
				Suggestions: nil,
				IsFlag:      false, // input doesn't start with dash
			},
		},
		{
			name:       "exact match short flag",
			input:      "-o",
			knownFlags: []string{"o", "f", "v"},
			want: FuzzyFlagResult{
				Input:       "-o",
				Match:       "o",
				AutoCorrect: true,
				Suggestions: nil,
				IsFlag:      true,
			},
		},
		{
			name:       "exact match with equals value",
			input:      "--owner=alice",
			knownFlags: testKnownFlags,
			want: FuzzyFlagResult{
				Input:       "--owner=alice",
				Match:       "owner",
				AutoCorrect: true,
				Suggestions: nil,
				IsFlag:      true,
				Value:       "alice",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuzzyMatchFlag(tt.input, tt.knownFlags)
			if got.Input != tt.want.Input {
				t.Errorf("FuzzyMatchFlag() Input = %v, want %v", got.Input, tt.want.Input)
			}
			if got.Match != tt.want.Match {
				t.Errorf("FuzzyMatchFlag() Match = %v, want %v", got.Match, tt.want.Match)
			}
			if got.AutoCorrect != tt.want.AutoCorrect {
				t.Errorf("FuzzyMatchFlag() AutoCorrect = %v, want %v", got.AutoCorrect, tt.want.AutoCorrect)
			}
			if got.IsFlag != tt.want.IsFlag {
				t.Errorf("FuzzyMatchFlag() IsFlag = %v, want %v", got.IsFlag, tt.want.IsFlag)
			}
			if got.Value != tt.want.Value {
				t.Errorf("FuzzyMatchFlag() Value = %v, want %v", got.Value, tt.want.Value)
			}
		})
	}
}

func TestFuzzyMatchFlag_TypoAutoCorrect(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		knownFlags []string
		wantMatch  string
		wantAuto   bool
	}{
		{
			name:       "ownr -> owner (missing letter)",
			input:      "--ownr",
			knownFlags: testKnownFlags,
			wantMatch:  "owner",
			wantAuto:   true,
		},
		{
			name:       "owne -> owner (missing letter at end)",
			input:      "--owne",
			knownFlags: testKnownFlags,
			wantMatch:  "owner",
			wantAuto:   true,
		},
		{
			name:       "verbos -> verbose (missing letter)",
			input:      "--verbos",
			knownFlags: testKnownFlags,
			wantMatch:  "verbose",
			wantAuto:   true,
		},
		{
			name:       "formt -> format (missing letter)",
			input:      "--formt",
			knownFlags: testKnownFlags,
			wantMatch:  "format",
			wantAuto:   true,
		},
		{
			name:       "ouput -> output (missing letter)",
			input:      "--ouput",
			knownFlags: testKnownFlags,
			wantMatch:  "output",
			wantAuto:   true,
		},
		{
			name:       "confg -> config (missing letter)",
			input:      "--confg",
			knownFlags: testKnownFlags,
			wantMatch:  "config",
			wantAuto:   true,
		},
		{
			name:       "short flag typo without dashes",
			input:      "ownr",
			knownFlags: testKnownFlags,
			wantMatch:  "owner",
			wantAuto:   true,
		},
		{
			name:       "typo with equals value",
			input:      "--ownr=alice",
			knownFlags: testKnownFlags,
			wantMatch:  "owner",
			wantAuto:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuzzyMatchFlag(tt.input, tt.knownFlags)
			if got.Match != tt.wantMatch {
				t.Errorf("FuzzyMatchFlag() Match = %v, want %v", got.Match, tt.wantMatch)
			}
			if got.AutoCorrect != tt.wantAuto {
				t.Errorf("FuzzyMatchFlag() AutoCorrect = %v, want %v", got.AutoCorrect, tt.wantAuto)
			}
		})
	}
}

func TestFuzzyMatchFlag_MultipleSuggestions(t *testing.T) {
	// Note: The fuzzy package may auto-correct even with multiple close matches
	// if one match is clearly better. These tests verify the suggestions are
	// populated when there are multiple close options.
	tests := []struct {
		name           string
		input          string
		knownFlags     []string
		wantAutoFalse  bool
		wantMinSuggs   int
		wantContains   []string // suggestions should contain these
	}{
		{
			name:          "for ambiguous (format, force)",
			input:         "--for",
			knownFlags:    testKnownFlags,
			wantAutoFalse: true,
			wantMinSuggs:  2,
			wantContains:  []string{"format", "force"},
		},
		{
			name:          "qu ambiguous (quiet)",
			input:         "--qu",
			knownFlags:    testKnownFlags,
			wantAutoFalse: false, // Only one close match, so auto-correct
			wantMinSuggs:  0,
			wantContains:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuzzyMatchFlag(tt.input, tt.knownFlags)
			if tt.wantAutoFalse && got.AutoCorrect {
				t.Errorf("FuzzyMatchFlag() AutoCorrect = true, want false for ambiguous input")
			}
			if len(got.Suggestions) < tt.wantMinSuggs {
				t.Errorf("FuzzyMatchFlag() Suggestions = %v, want at least %d", got.Suggestions, tt.wantMinSuggs)
			}
			for _, want := range tt.wantContains {
				found := false
				for _, s := range got.Suggestions {
					if s == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("FuzzyMatchFlag() Suggestions = %v, want to contain %q", got.Suggestions, want)
				}
			}
		})
	}
}

func TestFuzzyMatchFlag_NoMatch(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		knownFlags []string
	}{
		{
			name:       "completely unrelated",
			input:      "--xyz123",
			knownFlags: testKnownFlags,
		},
		{
			name:       "gibberish",
			input:      "--asdfghjkl",
			knownFlags: testKnownFlags,
		},
		{
			name:       "empty known flags",
			input:      "--owner",
			knownFlags: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuzzyMatchFlag(tt.input, tt.knownFlags)
			if got.Match != "" {
				t.Errorf("FuzzyMatchFlag() Match = %v, want empty for no match", got.Match)
			}
			if got.AutoCorrect {
				t.Errorf("FuzzyMatchFlag() AutoCorrect = true, want false for no match")
			}
		})
	}
}

func TestFuzzyMatchFlag_ShortFlags(t *testing.T) {
	shortFlags := []string{"o", "f", "v", "h", "q"}

	tests := []struct {
		name      string
		input     string
		wantMatch string
		wantAuto  bool
	}{
		{
			name:      "exact short flag -o",
			input:     "-o",
			wantMatch: "o",
			wantAuto:  true,
		},
		{
			name:      "exact short flag -v",
			input:     "-v",
			wantMatch: "v",
			wantAuto:  true,
		},
		{
			name:      "unknown short flag",
			input:     "-x",
			wantMatch: "",
			wantAuto:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuzzyMatchFlag(tt.input, shortFlags)
			if got.Match != tt.wantMatch {
				t.Errorf("FuzzyMatchFlag() Match = %v, want %v", got.Match, tt.wantMatch)
			}
			if got.AutoCorrect != tt.wantAuto {
				t.Errorf("FuzzyMatchFlag() AutoCorrect = %v, want %v", got.AutoCorrect, tt.wantAuto)
			}
		})
	}
}

func TestFuzzyMatchFlag_BooleanFlags(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		knownFlags []string
		wantMatch  string
		wantIsFlag bool
	}{
		{
			name:       "verbose flag",
			input:      "--verbose",
			knownFlags: testKnownFlags,
			wantMatch:  "verbose",
			wantIsFlag: true,
		},
		{
			name:       "force flag",
			input:      "--force",
			knownFlags: testKnownFlags,
			wantMatch:  "force",
			wantIsFlag: true,
		},
		{
			name:       "force with explicit value",
			input:      "--force=true",
			knownFlags: testKnownFlags,
			wantMatch:  "force",
			wantIsFlag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuzzyMatchFlag(tt.input, tt.knownFlags)
			if got.Match != tt.wantMatch {
				t.Errorf("FuzzyMatchFlag() Match = %v, want %v", got.Match, tt.wantMatch)
			}
			if got.IsFlag != tt.wantIsFlag {
				t.Errorf("FuzzyMatchFlag() IsFlag = %v, want %v", got.IsFlag, tt.wantIsFlag)
			}
		})
	}
}

func TestFuzzyMatchFlag_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		knownFlags []string
		wantMatch  string
		wantIsFlag bool
	}{
		{
			name:       "empty input",
			input:      "",
			knownFlags: testKnownFlags,
			wantMatch:  "",
			wantIsFlag: false,
		},
		{
			name:       "just dashes",
			input:      "--",
			knownFlags: testKnownFlags,
			wantMatch:  "",
			wantIsFlag: false,
		},
		{
			name:       "single dash",
			input:      "-",
			knownFlags: testKnownFlags,
			wantMatch:  "",
			wantIsFlag: false,
		},
		{
			name:       "nil known flags",
			input:      "--owner",
			knownFlags: nil,
			wantMatch:  "",
			wantIsFlag: true,
		},
		{
			name:       "flag with hyphen",
			input:      "--dry-run",
			knownFlags: testKnownFlags,
			wantMatch:  "dry-run",
			wantIsFlag: true,
		},
		{
			name:       "flag with hyphen typo",
			input:      "--dryrun",
			knownFlags: testKnownFlags,
			wantMatch:  "dry-run",
			wantIsFlag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuzzyMatchFlag(tt.input, tt.knownFlags)
			if got.Match != tt.wantMatch {
				t.Errorf("FuzzyMatchFlag() Match = %v, want %v", got.Match, tt.wantMatch)
			}
			if got.IsFlag != tt.wantIsFlag {
				t.Errorf("FuzzyMatchFlag() IsFlag = %v, want %v", got.IsFlag, tt.wantIsFlag)
			}
		})
	}
}

func TestFuzzyMatchFlags_MultipleArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		knownFlags   []string
		wantCorrected []string
		wantHasError bool
	}{
		{
			name:          "all exact matches",
			args:          []string{"--owner", "alice", "--format", "json"},
			knownFlags:    testKnownFlags,
			wantCorrected: []string{"--owner", "alice", "--format", "json"},
			wantHasError:  false,
		},
		{
			name:          "single typo corrected",
			args:          []string{"--ownr", "alice"},
			knownFlags:    testKnownFlags,
			wantCorrected: []string{"--owner", "alice"},
			wantHasError:  false,
		},
		{
			name:          "multiple typos corrected",
			args:          []string{"--ownr", "alice", "--formt", "json"},
			knownFlags:    testKnownFlags,
			wantCorrected: []string{"--owner", "alice", "--format", "json"},
			wantHasError:  false,
		},
		{
			name:          "mixed positional and flags",
			args:          []string{"1.2", "--ownr", "alice", "1.3"},
			knownFlags:    testKnownFlags,
			wantCorrected: []string{"1.2", "--owner", "alice", "1.3"},
			wantHasError:  false,
		},
		{
			name:          "unknown flag left alone",
			args:          []string{"--xyz123", "value"},
			knownFlags:    testKnownFlags,
			wantCorrected: []string{"--xyz123", "value"},
			wantHasError:  false,
		},
		{
			name:          "equals syntax preserved",
			args:          []string{"--ownr=alice"},
			knownFlags:    testKnownFlags,
			wantCorrected: []string{"--owner=alice"},
			wantHasError:  false,
		},
		{
			name:          "short flags unchanged",
			args:          []string{"-o", "alice"},
			knownFlags:    []string{"o", "owner"},
			wantCorrected: []string{"-o", "alice"},
			wantHasError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuzzyMatchFlags(tt.args, tt.knownFlags)
			if !reflect.DeepEqual(got.CorrectedArgs, tt.wantCorrected) {
				t.Errorf("FuzzyMatchFlags() CorrectedArgs = %v, want %v", got.CorrectedArgs, tt.wantCorrected)
			}
			if (len(got.Errors) > 0) != tt.wantHasError {
				t.Errorf("FuzzyMatchFlags() has errors = %v, want %v", len(got.Errors) > 0, tt.wantHasError)
			}
		})
	}
}

func TestFuzzyMatchFlags_Corrections(t *testing.T) {
	args := []string{"--ownr", "alice", "--formt", "json"}
	knownFlags := testKnownFlags

	got := FuzzyMatchFlags(args, knownFlags)

	// Should have 2 corrections
	if len(got.Corrections) != 2 {
		t.Fatalf("FuzzyMatchFlags() Corrections count = %d, want 2", len(got.Corrections))
	}

	// Verify corrections
	foundOwner := false
	foundFormat := false
	for _, c := range got.Corrections {
		if c.Original == "--ownr" && c.Corrected == "--owner" {
			foundOwner = true
		}
		if c.Original == "--formt" && c.Corrected == "--format" {
			foundFormat = true
		}
	}

	if !foundOwner {
		t.Error("FuzzyMatchFlags() missing correction --ownr -> --owner")
	}
	if !foundFormat {
		t.Error("FuzzyMatchFlags() missing correction --formt -> --format")
	}
}

func TestFuzzyMatchFlags_Ambiguous(t *testing.T) {
	// Use --for which is ambiguous between format and force
	args := []string{"--for", "json"}
	knownFlags := testKnownFlags

	got := FuzzyMatchFlags(args, knownFlags)

	// Should have an ambiguous entry
	if len(got.Ambiguous) == 0 {
		t.Error("FuzzyMatchFlags() should have ambiguous entries for --for")
	}

	// Check ambiguous entry
	if len(got.Ambiguous) > 0 {
		amb := got.Ambiguous[0]
		if amb.Input != "--for" {
			t.Errorf("FuzzyMatchFlags() Ambiguous[0].Input = %v, want --for", amb.Input)
		}
		// Should have suggestions for format and force
		foundFormat := false
		foundForce := false
		for _, s := range amb.Suggestions {
			if s == "format" {
				foundFormat = true
			}
			if s == "force" {
				foundForce = true
			}
		}
		if !foundFormat || !foundForce {
			t.Errorf("FuzzyMatchFlags() Ambiguous suggestions = %v, want to contain format and force", amb.Suggestions)
		}
	}
}

func TestFuzzyMatchFlags_EmptyAndNil(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		knownFlags []string
	}{
		{
			name:       "empty args",
			args:       []string{},
			knownFlags: testKnownFlags,
		},
		{
			name:       "nil args",
			args:       nil,
			knownFlags: testKnownFlags,
		},
		{
			name:       "nil known flags",
			args:       []string{"--owner"},
			knownFlags: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			got := FuzzyMatchFlags(tt.args, tt.knownFlags)
			if got.CorrectedArgs == nil {
				t.Error("FuzzyMatchFlags() CorrectedArgs should not be nil")
			}
		})
	}
}

func TestFuzzyMatchFlags_DoubleDash(t *testing.T) {
	args := []string{"--ownr", "alice", "--", "--formt", "json"}
	knownFlags := testKnownFlags

	got := FuzzyMatchFlags(args, knownFlags)

	// Should correct --ownr before -- but not --formt after --
	if len(got.Corrections) != 1 {
		t.Errorf("FuzzyMatchFlags() Corrections = %d, want 1 (only before --)", len(got.Corrections))
	}

	// The args after -- should be unchanged
	expected := []string{"--owner", "alice", "--", "--formt", "json"}
	if !reflect.DeepEqual(got.CorrectedArgs, expected) {
		t.Errorf("FuzzyMatchFlags() CorrectedArgs = %v, want %v", got.CorrectedArgs, expected)
	}
}

func TestFuzzyFlagResult_String(t *testing.T) {
	tests := []struct {
		name   string
		result FuzzyFlagResult
		want   string
	}{
		{
			name: "auto corrected",
			result: FuzzyFlagResult{
				Input:       "--ownr",
				Match:       "owner",
				AutoCorrect: true,
			},
			want: "auto-corrected --ownr to --owner",
		},
		{
			name: "suggestions",
			result: FuzzyFlagResult{
				Input:       "--ow",
				Match:       "",
				AutoCorrect: false,
				Suggestions: []string{"owner", "output"},
			},
			want: "ambiguous flag --ow, did you mean: --owner, --output?",
		},
		{
			name: "no match",
			result: FuzzyFlagResult{
				Input:       "--xyz",
				Match:       "",
				AutoCorrect: false,
				Suggestions: nil,
			},
			want: "unknown flag: --xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.String()
			if got != tt.want {
				t.Errorf("FuzzyFlagResult.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
