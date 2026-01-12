// Package fuzzy provides fuzzy string matching utilities.
package fuzzy

// Distance computes the Levenshtein edit distance between two strings.
// The Levenshtein distance is the minimum number of single-character edits
// (insertions, deletions, or substitutions) required to transform one string
// into the other.
//
// Properties:
//   - Distance(a, a) = 0 (identical strings have distance 0)
//   - Distance(a, b) = Distance(b, a) (symmetric)
//   - Distance(a, b) >= 0 (non-negative)
//   - Distance(a, b) <= max(len(a), len(b)) (upper bound)
//   - Triangle inequality: Distance(a, c) <= Distance(a, b) + Distance(b, c)
func Distance(a, b string) int {
	// Handle edge cases
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

	// Create the DP matrix
	// dp[i][j] represents the edit distance between a[0:i] and b[0:j]
	dp := make([][]int, lenA+1)
	for i := range dp {
		dp[i] = make([]int, lenB+1)
	}

	// Initialize first row: transforming empty string to b[0:j] requires j insertions
	for j := 0; j <= lenB; j++ {
		dp[0][j] = j
	}

	// Initialize first column: transforming a[0:i] to empty string requires i deletions
	for i := 0; i <= lenA; i++ {
		dp[i][0] = i
	}

	// Fill in the rest of the matrix
	for i := 1; i <= lenA; i++ {
		for j := 1; j <= lenB; j++ {
			// Cost of substitution: 0 if characters match, 1 otherwise
			cost := 1
			if runesA[i-1] == runesB[j-1] {
				cost = 0
			}

			// Take the minimum of three operations:
			// 1. Delete from a: dp[i-1][j] + 1
			// 2. Insert into a (delete from b): dp[i][j-1] + 1
			// 3. Substitute (or match): dp[i-1][j-1] + cost
			dp[i][j] = min3(
				dp[i-1][j]+1,      // deletion
				dp[i][j-1]+1,      // insertion
				dp[i-1][j-1]+cost, // substitution
			)
		}
	}

	return dp[lenA][lenB]
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
