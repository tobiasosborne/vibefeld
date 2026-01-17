// Package fuzzy provides fuzzy string matching utilities.
package fuzzy

// Distance computes the Levenshtein edit distance between two strings.
// The Levenshtein distance is the minimum number of single-character edits
// (insertions, deletions, or substitutions) required to transform one string
// into the other.
//
// This implementation uses a space-optimized algorithm with O(min(N,M)) memory
// instead of the naive O(N*M) matrix approach.
//
// Properties:
//   - Distance(a, a) = 0 (identical strings have distance 0)
//   - Distance(a, b) = Distance(b, a) (symmetric)
//   - Distance(a, b) >= 0 (non-negative)
//   - Distance(a, b) <= max(len(a), len(b)) (upper bound)
//   - Triangle inequality: Distance(a, c) <= Distance(a, b) + Distance(b, c)
func Distance(a, b string) int {
	// Handle edge cases - early termination for exact matches
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Convert strings to runes to handle multi-byte characters correctly
	runesA := []rune(a)
	runesB := []rune(b)
	lenA := len(runesA)
	lenB := len(runesB)

	// Ensure we iterate over the shorter string to minimize memory usage
	// We only need O(min(lenA, lenB)) space
	if lenA > lenB {
		runesA, runesB = runesB, runesA
		lenA, lenB = lenB, lenA
	}

	// Space-optimized DP: we only need two rows instead of full matrix
	// prev represents dp[i-1][*], curr represents dp[i][*]
	prev := make([]int, lenB+1)
	curr := make([]int, lenB+1)

	// Initialize first row: transforming empty string to b[0:j] requires j insertions
	for j := 0; j <= lenB; j++ {
		prev[j] = j
	}

	// Fill in the DP table row by row
	for i := 1; i <= lenA; i++ {
		// First column: transforming a[0:i] to empty string requires i deletions
		curr[0] = i

		for j := 1; j <= lenB; j++ {
			// Cost of substitution: 0 if characters match, 1 otherwise
			cost := 1
			if runesA[i-1] == runesB[j-1] {
				cost = 0
			}

			// Take the minimum of three operations:
			// 1. Delete from a: prev[j] + 1
			// 2. Insert into a (delete from b): curr[j-1] + 1
			// 3. Substitute (or match): prev[j-1] + cost
			curr[j] = min3(
				prev[j]+1,      // deletion
				curr[j-1]+1,    // insertion
				prev[j-1]+cost, // substitution
			)
		}

		// Swap rows: current becomes previous for next iteration
		prev, curr = curr, prev
	}

	// Result is in prev because we swapped at the end
	return prev[lenB]
}

// min3 returns the minimum of three integers.
func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}
