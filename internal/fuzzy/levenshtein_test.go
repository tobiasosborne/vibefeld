package fuzzy

import (
	"testing"
)

func TestDistance(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected int
	}{
		// Identical strings
		{
			name:     "identical strings",
			a:        "hello",
			b:        "hello",
			expected: 0,
		},
		{
			name:     "identical empty strings",
			a:        "",
			b:        "",
			expected: 0,
		},
		{
			name:     "identical single char",
			a:        "a",
			b:        "a",
			expected: 0,
		},

		// Empty strings
		{
			name:     "empty to non-empty abc",
			a:        "",
			b:        "abc",
			expected: 3,
		},
		{
			name:     "non-empty abc to empty",
			a:        "abc",
			b:        "",
			expected: 3,
		},
		{
			name:     "empty to single char",
			a:        "",
			b:        "x",
			expected: 1,
		},
		{
			name:     "single char to empty",
			a:        "x",
			b:        "",
			expected: 1,
		},

		// Single character difference - substitution
		{
			name:     "single substitution cat/bat",
			a:        "cat",
			b:        "bat",
			expected: 1,
		},
		{
			name:     "single substitution sit/sat",
			a:        "sit",
			b:        "sat",
			expected: 1,
		},
		{
			name:     "single substitution at end",
			a:        "abc",
			b:        "abd",
			expected: 1,
		},

		// Single character difference - insertion
		{
			name:     "single insertion cat/cats",
			a:        "cat",
			b:        "cats",
			expected: 1,
		},
		{
			name:     "single insertion at beginning",
			a:        "at",
			b:        "cat",
			expected: 1,
		},
		{
			name:     "single insertion in middle",
			a:        "ct",
			b:        "cat",
			expected: 1,
		},

		// Single character difference - deletion
		{
			name:     "single deletion cats/cat",
			a:        "cats",
			b:        "cat",
			expected: 1,
		},
		{
			name:     "single deletion from beginning",
			a:        "cat",
			b:        "at",
			expected: 1,
		},
		{
			name:     "single deletion from middle",
			a:        "cat",
			b:        "ct",
			expected: 1,
		},

		// Multiple operations - classic examples
		{
			name:     "kitten to sitting",
			a:        "kitten",
			b:        "sitting",
			expected: 3,
		},
		{
			name:     "saturday to sunday",
			a:        "saturday",
			b:        "sunday",
			expected: 3,
		},
		{
			name:     "flaw to lawn",
			a:        "flaw",
			b:        "lawn",
			expected: 2,
		},
		{
			name:     "gumbo to gambol",
			a:        "gumbo",
			b:        "gambol",
			expected: 2,
		},

		// Case sensitivity
		{
			name:     "case difference single char",
			a:        "Hello",
			b:        "hello",
			expected: 1,
		},
		{
			name:     "case difference multiple chars",
			a:        "HELLO",
			b:        "hello",
			expected: 5,
		},
		{
			name:     "mixed case",
			a:        "HeLLo",
			b:        "hello",
			expected: 3,
		},

		// Longer strings
		{
			name:     "longer strings similar",
			a:        "algorithm",
			b:        "altruistic",
			expected: 6,
		},
		{
			name:     "completely different",
			a:        "abcdef",
			b:        "ghijkl",
			expected: 6,
		},
		{
			name:     "reversed string short",
			a:        "abc",
			b:        "cba",
			expected: 2,
		},

		// Real typos for CLI fuzzy matching
		{
			name:     "typo chalenge/challenge",
			a:        "chalenge",
			b:        "challenge",
			expected: 1,
		},
		{
			name:     "typo stauts/status",
			a:        "stauts",
			b:        "status",
			expected: 2,
		},
		{
			name:     "typo refien/refine",
			a:        "refien",
			b:        "refine",
			expected: 2,
		},
		{
			name:     "typo calim/claim",
			a:        "calim",
			b:        "claim",
			expected: 2,
		},
		{
			name:     "typo relase/release",
			a:        "relase",
			b:        "release",
			expected: 1,
		},
		{
			name:     "typo accpet/accept",
			a:        "accpet",
			b:        "accept",
			expected: 2,
		},
		{
			name:     "typo initt/init",
			a:        "initt",
			b:        "init",
			expected: 1,
		},
		{
			name:     "typo verfiy/verify",
			a:        "verfiy",
			b:        "verify",
			expected: 2,
		},

		// Edge cases
		{
			name:     "single char different",
			a:        "a",
			b:        "b",
			expected: 1,
		},
		{
			name:     "repeated chars",
			a:        "aaa",
			b:        "aaaa",
			expected: 1,
		},
		{
			name:     "prefix match",
			a:        "pre",
			b:        "prefix",
			expected: 3,
		},
		{
			name:     "suffix match",
			a:        "fix",
			b:        "prefix",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Distance(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("Distance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestDistance_Symmetry(t *testing.T) {
	// Distance should be symmetric: Distance(a, b) == Distance(b, a)
	pairs := []struct {
		a string
		b string
	}{
		{"hello", "world"},
		{"cat", "cats"},
		{"kitten", "sitting"},
		{"", "abc"},
		{"saturday", "sunday"},
		{"chalenge", "challenge"},
		{"a", "b"},
		{"abc", "xyz"},
		{"algorithm", "altruistic"},
	}

	for _, pair := range pairs {
		t.Run(pair.a+"_"+pair.b, func(t *testing.T) {
			d1 := Distance(pair.a, pair.b)
			d2 := Distance(pair.b, pair.a)
			if d1 != d2 {
				t.Errorf("Distance is not symmetric: Distance(%q, %q) = %d, Distance(%q, %q) = %d",
					pair.a, pair.b, d1, pair.b, pair.a, d2)
			}
		})
	}
}

func TestDistance_NonNegative(t *testing.T) {
	// Distance should always be non-negative
	testCases := []struct {
		a string
		b string
	}{
		{"", ""},
		{"a", ""},
		{"", "a"},
		{"hello", "world"},
		{"same", "same"},
		{"ABC", "abc"},
		{"kitten", "sitting"},
	}

	for _, tc := range testCases {
		t.Run(tc.a+"_"+tc.b, func(t *testing.T) {
			d := Distance(tc.a, tc.b)
			if d < 0 {
				t.Errorf("Distance(%q, %q) = %d, expected non-negative", tc.a, tc.b, d)
			}
		})
	}
}

func TestDistance_SameStringZero(t *testing.T) {
	// Distance(a, a) should always be 0
	strings := []string{
		"",
		"a",
		"hello",
		"Hello World",
		"The quick brown fox jumps over the lazy dog",
		"12345",
		"!@#$%",
		"mixed123!@#",
	}

	for _, s := range strings {
		t.Run("self_"+s, func(t *testing.T) {
			d := Distance(s, s)
			if d != 0 {
				t.Errorf("Distance(%q, %q) = %d, expected 0", s, s, d)
			}
		})
	}
}

func TestDistance_UpperBound(t *testing.T) {
	// Distance(a, b) should be <= max(len(a), len(b))
	pairs := []struct {
		a string
		b string
	}{
		{"", "abc"},
		{"abc", ""},
		{"hello", "world"},
		{"a", "bcdefghij"},
		{"short", "muchlongerstring"},
		{"same", "same"},
		{"x", "y"},
		{"", ""},
	}

	for _, pair := range pairs {
		t.Run(pair.a+"_"+pair.b, func(t *testing.T) {
			d := Distance(pair.a, pair.b)
			maxLen := len(pair.a)
			if len(pair.b) > maxLen {
				maxLen = len(pair.b)
			}
			if d > maxLen {
				t.Errorf("Distance(%q, %q) = %d, exceeds max length %d",
					pair.a, pair.b, d, maxLen)
			}
		})
	}
}

func TestDistance_TriangleInequality(t *testing.T) {
	// Triangle inequality: Distance(a, c) <= Distance(a, b) + Distance(b, c)
	triples := []struct {
		a string
		b string
		c string
	}{
		{"cat", "bat", "bar"},
		{"hello", "hallo", "hullo"},
		{"abc", "abd", "aed"},
		{"kitten", "mitten", "sitting"},
	}

	for _, triple := range triples {
		t.Run(triple.a+"_"+triple.b+"_"+triple.c, func(t *testing.T) {
			dAC := Distance(triple.a, triple.c)
			dAB := Distance(triple.a, triple.b)
			dBC := Distance(triple.b, triple.c)

			if dAC > dAB+dBC {
				t.Errorf("Triangle inequality violated: Distance(%q, %q) = %d > Distance(%q, %q) + Distance(%q, %q) = %d + %d = %d",
					triple.a, triple.c, dAC,
					triple.a, triple.b,
					triple.b, triple.c,
					dAB, dBC, dAB+dBC)
			}
		})
	}
}

// Benchmark for performance testing
func BenchmarkDistance(b *testing.B) {
	benchmarks := []struct {
		name string
		a    string
		b    string
	}{
		{"short_same", "hello", "hello"},
		{"short_diff", "hello", "world"},
		{"medium", "kitten", "sitting"},
		{"long", "the quick brown fox", "the lazy brown dog"},
		{"very_long", "algorithm implementation", "algorithmic implemantation"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Distance(bm.a, bm.b)
			}
		})
	}
}
