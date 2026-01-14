// Package node_test contains external tests for the node package.
package node_test

import (
	"strings"
	"testing"

	aferrors "github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// ===========================================================================
// Helper functions for test setup
// ===========================================================================

// createTestChallenge creates a challenge for testing with a unique ID.
func createTestChallenge(t *testing.T, id string, targetID string) *node.Challenge {
	t.Helper()

	nodeID, err := types.Parse(targetID)
	if err != nil {
		t.Fatalf("types.Parse(%q) error: %v", targetID, err)
	}

	ch, err := node.NewChallenge(id, nodeID, schema.TargetStatement, "Test challenge reason")
	if err != nil {
		t.Fatalf("NewChallenge() error: %v", err)
	}

	return ch
}

// createNChallenges creates n challenges all targeting the same node.
func createNChallenges(t *testing.T, n int, targetID string) []*node.Challenge {
	t.Helper()

	challenges := make([]*node.Challenge, n)
	for i := 0; i < n; i++ {
		challenges[i] = createTestChallenge(t, "ch-"+string(rune('0'+i/100))+string(rune('0'+(i/10)%10))+string(rune('0'+i%10)), targetID)
	}
	return challenges
}

// ===========================================================================
// Basic validation tests - Under limit
// ===========================================================================

// TestValidateChallengeLimit_UnderLimit tests that challenge counts under the limit pass validation.
func TestValidateChallengeLimit_UnderLimit(t *testing.T) {
	tests := []struct {
		name          string
		numChallenges int
		maxChallenges int
	}{
		{
			name:          "0 challenges with max 5",
			numChallenges: 0,
			maxChallenges: 5,
		},
		{
			name:          "1 challenge with max 5",
			numChallenges: 1,
			maxChallenges: 5,
		},
		{
			name:          "2 challenges with max 5",
			numChallenges: 2,
			maxChallenges: 5,
		},
		{
			name:          "4 challenges with max 5",
			numChallenges: 4,
			maxChallenges: 5,
		},
		{
			name:          "1 challenge with max 10",
			numChallenges: 1,
			maxChallenges: 10,
		},
		{
			name:          "5 challenges with max 10",
			numChallenges: 5,
			maxChallenges: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenges := createNChallenges(t, tt.numChallenges, "1.1")

			err := node.ValidateChallengeLimit(challenges, tt.maxChallenges)
			if err != nil {
				t.Errorf("ValidateChallengeLimit() = %v, want nil for %d challenges <= max %d",
					err, tt.numChallenges, tt.maxChallenges)
			}
		})
	}
}

// ===========================================================================
// At limit tests
// ===========================================================================

// TestValidateChallengeLimit_AtLimit tests that challenge counts exactly at the limit pass validation.
func TestValidateChallengeLimit_AtLimit(t *testing.T) {
	tests := []struct {
		name          string
		maxChallenges int
	}{
		{
			name:          "1 challenge at limit 1",
			maxChallenges: 1,
		},
		{
			name:          "5 challenges at limit 5",
			maxChallenges: 5,
		},
		{
			name:          "10 challenges at limit 10",
			maxChallenges: 10,
		},
		{
			name:          "20 challenges at limit 20",
			maxChallenges: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenges := createNChallenges(t, tt.maxChallenges, "1.1")

			err := node.ValidateChallengeLimit(challenges, tt.maxChallenges)
			if err != nil {
				t.Errorf("ValidateChallengeLimit() = %v, want nil for %d challenges == max %d",
					err, tt.maxChallenges, tt.maxChallenges)
			}
		})
	}
}

// ===========================================================================
// Over limit tests
// ===========================================================================

// TestValidateChallengeLimit_OverLimit tests that challenge counts exceeding the limit fail validation.
func TestValidateChallengeLimit_OverLimit(t *testing.T) {
	tests := []struct {
		name          string
		numChallenges int
		maxChallenges int
	}{
		{
			name:          "2 challenges exceeds max 1",
			numChallenges: 2,
			maxChallenges: 1,
		},
		{
			name:          "6 challenges exceeds max 5",
			numChallenges: 6,
			maxChallenges: 5,
		},
		{
			name:          "11 challenges exceeds max 10",
			numChallenges: 11,
			maxChallenges: 10,
		},
		{
			name:          "7 challenges exceeds max 5",
			numChallenges: 7,
			maxChallenges: 5,
		},
		{
			name:          "100 challenges exceeds max 5",
			numChallenges: 100,
			maxChallenges: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenges := createNChallenges(t, tt.numChallenges, "1.1")

			err := node.ValidateChallengeLimit(challenges, tt.maxChallenges)
			if err == nil {
				t.Errorf("ValidateChallengeLimit() = nil, want CHALLENGE_LIMIT_EXCEEDED error for %d challenges > max %d",
					tt.numChallenges, tt.maxChallenges)
			}
		})
	}
}

// ===========================================================================
// Error code tests
// ===========================================================================

// TestValidateChallengeLimit_ReturnsChallengeLimitExceededError tests that the correct error code is returned.
func TestValidateChallengeLimit_ReturnsChallengeLimitExceededError(t *testing.T) {
	challenges := createNChallenges(t, 6, "1.1")

	err := node.ValidateChallengeLimit(challenges, 5)
	if err == nil {
		t.Fatal("ValidateChallengeLimit() = nil, want error")
	}

	// Check that the error code is CHALLENGE_LIMIT_EXCEEDED
	code := aferrors.Code(err)
	if code != aferrors.CHALLENGE_LIMIT_EXCEEDED {
		t.Errorf("Error code = %v, want CHALLENGE_LIMIT_EXCEEDED", code)
	}
}

// TestValidateChallengeLimit_ErrorExitCode tests that the error has exit code 3 (logic error).
func TestValidateChallengeLimit_ErrorExitCode(t *testing.T) {
	challenges := createNChallenges(t, 10, "1.1")

	err := node.ValidateChallengeLimit(challenges, 5)
	if err == nil {
		t.Fatal("ValidateChallengeLimit() = nil, want error")
	}

	// CHALLENGE_LIMIT_EXCEEDED should have exit code 3 (logic error)
	exitCode := aferrors.ExitCode(err)
	if exitCode != 3 {
		t.Errorf("ExitCode = %d, want 3 (logic error)", exitCode)
	}
}

// TestValidateChallengeLimit_ErrorMessage tests that the error message contains useful information.
func TestValidateChallengeLimit_ErrorMessage(t *testing.T) {
	numChallenges := 8
	maxChallenges := 5
	challenges := createNChallenges(t, numChallenges, "1.1")

	err := node.ValidateChallengeLimit(challenges, maxChallenges)
	if err == nil {
		t.Fatal("ValidateChallengeLimit() = nil, want error")
	}

	errMsg := err.Error()

	// Error should mention the number of challenges
	if !strings.Contains(errMsg, "8") {
		t.Errorf("Error message should contain challenge count '8', got: %s", errMsg)
	}

	// Error should mention the max challenges value
	if !strings.Contains(errMsg, "5") {
		t.Errorf("Error message should contain maxChallenges '5', got: %s", errMsg)
	}
}

// ===========================================================================
// Edge case tests - Zero challenges
// ===========================================================================

// TestValidateChallengeLimit_ZeroChallenges tests validation with zero challenges.
func TestValidateChallengeLimit_ZeroChallenges(t *testing.T) {
	tests := []struct {
		name          string
		maxChallenges int
	}{
		{
			name:          "zero challenges with max 1",
			maxChallenges: 1,
		},
		{
			name:          "zero challenges with max 5",
			maxChallenges: 5,
		},
		{
			name:          "zero challenges with max 10",
			maxChallenges: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var challenges []*node.Challenge // empty slice

			err := node.ValidateChallengeLimit(challenges, tt.maxChallenges)
			if err != nil {
				t.Errorf("ValidateChallengeLimit() = %v, want nil for 0 challenges", err)
			}
		})
	}
}

// TestValidateChallengeLimit_NilSlice tests validation with nil slice.
func TestValidateChallengeLimit_NilSlice(t *testing.T) {
	err := node.ValidateChallengeLimit(nil, 5)
	if err != nil {
		t.Errorf("ValidateChallengeLimit(nil, 5) = %v, want nil for nil slice (treated as 0 challenges)", err)
	}
}

// ===========================================================================
// Edge case tests - Max challenges edge values
// ===========================================================================

// TestValidateChallengeLimit_MaxChallengesOne tests with maxChallenges of 1.
func TestValidateChallengeLimit_MaxChallengesOne(t *testing.T) {
	tests := []struct {
		name          string
		numChallenges int
		expectError   bool
	}{
		{
			name:          "0 challenges with max 1",
			numChallenges: 0,
			expectError:   false,
		},
		{
			name:          "1 challenge with max 1",
			numChallenges: 1,
			expectError:   false,
		},
		{
			name:          "2 challenges with max 1",
			numChallenges: 2,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenges := createNChallenges(t, tt.numChallenges, "1.1")

			err := node.ValidateChallengeLimit(challenges, 1)
			if tt.expectError && err == nil {
				t.Error("ValidateChallengeLimit() = nil, want error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateChallengeLimit() = %v, want nil", err)
			}
		})
	}
}

// TestValidateChallengeLimit_ZeroMaxChallenges tests behavior with maxChallenges of 0.
func TestValidateChallengeLimit_ZeroMaxChallenges(t *testing.T) {
	// maxChallenges of 0 means no challenges allowed
	t.Run("zero challenges with max 0", func(t *testing.T) {
		var challenges []*node.Challenge
		err := node.ValidateChallengeLimit(challenges, 0)
		if err != nil {
			t.Errorf("ValidateChallengeLimit() = %v, want nil for 0 challenges with max 0", err)
		}
	})

	t.Run("one challenge with max 0", func(t *testing.T) {
		challenges := createNChallenges(t, 1, "1.1")
		err := node.ValidateChallengeLimit(challenges, 0)
		if err == nil {
			t.Error("ValidateChallengeLimit() = nil, want error for 1 challenge with max 0")
		}
	})
}

// TestValidateChallengeLimit_NegativeMaxChallenges tests behavior with negative maxChallenges.
func TestValidateChallengeLimit_NegativeMaxChallenges(t *testing.T) {
	challenges := createNChallenges(t, 1, "1.1")

	// Negative maxChallenges should fail for any positive number of challenges
	err := node.ValidateChallengeLimit(challenges, -1)
	if err == nil {
		t.Error("ValidateChallengeLimit() = nil, want error for negative maxChallenges")
	}
}

// ===========================================================================
// Boundary tests
// ===========================================================================

// TestValidateChallengeLimit_BoundaryConditions tests boundary conditions precisely.
func TestValidateChallengeLimit_BoundaryConditions(t *testing.T) {
	maxChallenges := 5

	t.Run("exactly_at_limit", func(t *testing.T) {
		challenges := createNChallenges(t, maxChallenges, "1.1")
		err := node.ValidateChallengeLimit(challenges, maxChallenges)
		if err != nil {
			t.Errorf("ValidateChallengeLimit() = %v, want nil for count == max", err)
		}
	})

	t.Run("one_over_limit", func(t *testing.T) {
		challenges := createNChallenges(t, maxChallenges+1, "1.1")
		err := node.ValidateChallengeLimit(challenges, maxChallenges)
		if err == nil {
			t.Error("ValidateChallengeLimit() = nil, want error for count == max+1")
		}
	})

	t.Run("one_under_limit", func(t *testing.T) {
		challenges := createNChallenges(t, maxChallenges-1, "1.1")
		err := node.ValidateChallengeLimit(challenges, maxChallenges)
		if err != nil {
			t.Errorf("ValidateChallengeLimit() = %v, want nil for count == max-1", err)
		}
	})
}

// ===========================================================================
// Table-driven comprehensive tests
// ===========================================================================

// TestValidateChallengeLimit_TableDriven provides comprehensive table-driven tests.
func TestValidateChallengeLimit_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		numChallenges int
		maxChallenges int
		expectError   bool
		errorCode     aferrors.ErrorCode
	}{
		{
			name:          "0 challenges max 5",
			numChallenges: 0,
			maxChallenges: 5,
			expectError:   false,
		},
		{
			name:          "1 challenge max 5",
			numChallenges: 1,
			maxChallenges: 5,
			expectError:   false,
		},
		{
			name:          "5 challenges max 5",
			numChallenges: 5,
			maxChallenges: 5,
			expectError:   false,
		},
		{
			name:          "6 challenges max 5",
			numChallenges: 6,
			maxChallenges: 5,
			expectError:   true,
			errorCode:     aferrors.CHALLENGE_LIMIT_EXCEEDED,
		},
		{
			name:          "10 challenges max 5",
			numChallenges: 10,
			maxChallenges: 5,
			expectError:   true,
			errorCode:     aferrors.CHALLENGE_LIMIT_EXCEEDED,
		},
		{
			name:          "1 challenge max 1",
			numChallenges: 1,
			maxChallenges: 1,
			expectError:   false,
		},
		{
			name:          "2 challenges max 1",
			numChallenges: 2,
			maxChallenges: 1,
			expectError:   true,
			errorCode:     aferrors.CHALLENGE_LIMIT_EXCEEDED,
		},
		{
			name:          "0 challenges max 0",
			numChallenges: 0,
			maxChallenges: 0,
			expectError:   false,
		},
		{
			name:          "1 challenge max 0",
			numChallenges: 1,
			maxChallenges: 0,
			expectError:   true,
			errorCode:     aferrors.CHALLENGE_LIMIT_EXCEEDED,
		},
		{
			name:          "10 challenges max 10",
			numChallenges: 10,
			maxChallenges: 10,
			expectError:   false,
		},
		{
			name:          "11 challenges max 10",
			numChallenges: 11,
			maxChallenges: 10,
			expectError:   true,
			errorCode:     aferrors.CHALLENGE_LIMIT_EXCEEDED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenges := createNChallenges(t, tt.numChallenges, "1.1")

			err := node.ValidateChallengeLimit(challenges, tt.maxChallenges)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateChallengeLimit() = nil, want error for %d challenges > max %d",
						tt.numChallenges, tt.maxChallenges)
					return
				}

				if tt.errorCode != 0 {
					code := aferrors.Code(err)
					if code != tt.errorCode {
						t.Errorf("Error code = %v, want %v", code, tt.errorCode)
					}
				}
			} else {
				if err != nil {
					t.Errorf("ValidateChallengeLimit() = %v, want nil for %d challenges <= max %d",
						err, tt.numChallenges, tt.maxChallenges)
				}
			}
		})
	}
}

// ===========================================================================
// Default max challenges tests (5 is common default)
// ===========================================================================

// TestValidateChallengeLimit_DefaultMax tests with the typical default max of 5.
func TestValidateChallengeLimit_DefaultMax(t *testing.T) {
	defaultMax := 5

	tests := []struct {
		name          string
		numChallenges int
		expectError   bool
	}{
		{"zero challenges", 0, false},
		{"one challenge", 1, false},
		{"two challenges", 2, false},
		{"three challenges", 3, false},
		{"four challenges", 4, false},
		{"five challenges (at limit)", 5, false},
		{"six challenges (over limit)", 6, true},
		{"seven challenges", 7, true},
		{"ten challenges", 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenges := createNChallenges(t, tt.numChallenges, "1.1")

			err := node.ValidateChallengeLimit(challenges, defaultMax)
			if tt.expectError && err == nil {
				t.Error("ValidateChallengeLimit() = nil, want error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateChallengeLimit() = %v, want nil", err)
			}
		})
	}
}

// ===========================================================================
// Different node targets
// ===========================================================================

// TestValidateChallengeLimit_DifferentTargetNodes tests challenges targeting different nodes.
func TestValidateChallengeLimit_DifferentTargetNodes(t *testing.T) {
	// Even if challenges target different nodes, the count should still be validated
	// This tests that the function counts all challenges in the slice regardless of target
	tests := []struct {
		name     string
		targetID string
	}{
		{"root node", "1"},
		{"first child", "1.1"},
		{"nested child", "1.1.1"},
		{"deep child", "1.1.1.1.1"},
		{"second child", "1.2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Under limit
			challenges := createNChallenges(t, 3, tt.targetID)
			err := node.ValidateChallengeLimit(challenges, 5)
			if err != nil {
				t.Errorf("ValidateChallengeLimit() = %v, want nil for 3 challenges", err)
			}

			// Over limit
			challenges = createNChallenges(t, 6, tt.targetID)
			err = node.ValidateChallengeLimit(challenges, 5)
			if err == nil {
				t.Error("ValidateChallengeLimit() = nil, want error for 6 challenges")
			}
		})
	}
}

// ===========================================================================
// Large numbers test
// ===========================================================================

// TestValidateChallengeLimit_LargeNumbers tests with larger numbers of challenges.
func TestValidateChallengeLimit_LargeNumbers(t *testing.T) {
	tests := []struct {
		name          string
		numChallenges int
		maxChallenges int
		expectError   bool
	}{
		{
			name:          "50 challenges with max 100",
			numChallenges: 50,
			maxChallenges: 100,
			expectError:   false,
		},
		{
			name:          "100 challenges with max 100",
			numChallenges: 100,
			maxChallenges: 100,
			expectError:   false,
		},
		{
			name:          "101 challenges with max 100",
			numChallenges: 101,
			maxChallenges: 100,
			expectError:   true,
		},
		{
			name:          "200 challenges with max 50",
			numChallenges: 200,
			maxChallenges: 50,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenges := createNChallenges(t, tt.numChallenges, "1.1")

			err := node.ValidateChallengeLimit(challenges, tt.maxChallenges)
			if tt.expectError && err == nil {
				t.Errorf("ValidateChallengeLimit() = nil, want error for %d > %d",
					tt.numChallenges, tt.maxChallenges)
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateChallengeLimit() = %v, want nil for %d <= %d",
					err, tt.numChallenges, tt.maxChallenges)
			}
		})
	}
}

// ===========================================================================
// Mixed challenge states (open vs resolved)
// ===========================================================================

// TestValidateChallengeLimit_OnlyCountsOpenChallenges tests that resolved/withdrawn challenges may or may not count.
// Note: This test documents the expected behavior - implementation should clarify
// whether all challenges count or only open ones.
func TestValidateChallengeLimit_MixedChallengeStates(t *testing.T) {
	// Create some challenges
	challenges := createNChallenges(t, 5, "1.1")

	// Resolve some of them
	if err := challenges[0].Resolve("Fixed"); err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if err := challenges[1].Withdraw(); err != nil {
		t.Fatalf("Withdraw() error: %v", err)
	}

	// Now we have: 3 open, 1 resolved, 1 withdrawn
	// The implementation should document whether it counts all 5 or only the 3 open ones

	// Test that validation still works with mixed states
	// This is a basic sanity check - actual counting behavior TBD by implementation
	err := node.ValidateChallengeLimit(challenges, 5)
	// Either way this should pass since total is 5 and open is 3
	if err != nil {
		t.Errorf("ValidateChallengeLimit() = %v, want nil for 5 total challenges (3 open) with max 5", err)
	}
}

// ===========================================================================
// Exact boundary max+1 test
// ===========================================================================

// TestValidateChallengeLimit_ExactlyMaxPlusOne tests the exact transition point.
func TestValidateChallengeLimit_ExactlyMaxPlusOne(t *testing.T) {
	maxValues := []int{1, 3, 5, 10, 20}

	for _, max := range maxValues {
		t.Run("max_"+string(rune('0'+max%10)), func(t *testing.T) {
			// At limit should pass
			t.Run("at_limit", func(t *testing.T) {
				challenges := createNChallenges(t, max, "1.1")
				err := node.ValidateChallengeLimit(challenges, max)
				if err != nil {
					t.Errorf("ValidateChallengeLimit() = %v, want nil at limit %d", err, max)
				}
			})

			// Max+1 should fail
			t.Run("max_plus_one", func(t *testing.T) {
				challenges := createNChallenges(t, max+1, "1.1")
				err := node.ValidateChallengeLimit(challenges, max)
				if err == nil {
					t.Errorf("ValidateChallengeLimit() = nil, want error for %d > %d", max+1, max)
				}
			})
		})
	}
}
