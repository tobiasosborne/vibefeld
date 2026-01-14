//go:build integration
// +build integration

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

// createRefinementTestNode creates a test node with the given ID string for refinement limit tests.
func createRefinementTestNode(t *testing.T, idStr string) *node.Node {
	t.Helper()

	id, err := types.Parse(idStr)
	if err != nil {
		t.Fatalf("types.Parse(%q) error: %v", idStr, err)
	}

	n, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement for "+idStr, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	return n
}

// ===========================================================================
// Basic validation tests - Under limit
// ===========================================================================

// TestValidateRefinementCount_UnderLimit tests that refinement counts under the limit pass validation.
func TestValidateRefinementCount_UnderLimit(t *testing.T) {
	tests := []struct {
		name            string
		refinementCount int
		maxRefinements  int
	}{
		{
			name:            "0 refinements with max 10",
			refinementCount: 0,
			maxRefinements:  10,
		},
		{
			name:            "1 refinement with max 10",
			refinementCount: 1,
			maxRefinements:  10,
		},
		{
			name:            "5 refinements with max 10",
			refinementCount: 5,
			maxRefinements:  10,
		},
		{
			name:            "9 refinements with max 10",
			refinementCount: 9,
			maxRefinements:  10,
		},
		{
			name:            "0 refinements with max 1",
			refinementCount: 0,
			maxRefinements:  1,
		},
		{
			name:            "50 refinements with max 100",
			refinementCount: 50,
			maxRefinements:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createRefinementTestNode(t, "1.1")

			err := node.ValidateRefinementCount(n, tt.refinementCount, tt.maxRefinements)
			if err != nil {
				t.Errorf("ValidateRefinementCount() = %v, want nil for count %d < maxRefinements %d",
					err, tt.refinementCount, tt.maxRefinements)
			}
		})
	}
}

// ===========================================================================
// Basic validation tests - At limit
// ===========================================================================

// TestValidateRefinementCount_AtLimit tests that refinement count exactly at the limit is blocked.
// At max = blocked because we cannot add another refinement.
func TestValidateRefinementCount_AtLimit(t *testing.T) {
	tests := []struct {
		name           string
		maxRefinements int
	}{
		{
			name:           "at limit 1",
			maxRefinements: 1,
		},
		{
			name:           "at limit 5",
			maxRefinements: 5,
		},
		{
			name:           "at limit 10",
			maxRefinements: 10,
		},
		{
			name:           "at limit 100",
			maxRefinements: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createRefinementTestNode(t, "1.2")

			// When count equals max, we're at the limit - no more refinements allowed
			err := node.ValidateRefinementCount(n, tt.maxRefinements, tt.maxRefinements)
			if err == nil {
				t.Errorf("ValidateRefinementCount() = nil, want error for count %d == maxRefinements %d",
					tt.maxRefinements, tt.maxRefinements)
			}
		})
	}
}

// ===========================================================================
// Basic validation tests - Over limit
// ===========================================================================

// TestValidateRefinementCount_OverLimit tests that refinement counts over the limit fail validation.
func TestValidateRefinementCount_OverLimit(t *testing.T) {
	tests := []struct {
		name            string
		refinementCount int
		maxRefinements  int
	}{
		{
			name:            "11 refinements exceeds max 10",
			refinementCount: 11,
			maxRefinements:  10,
		},
		{
			name:            "2 refinements exceeds max 1",
			refinementCount: 2,
			maxRefinements:  1,
		},
		{
			name:            "15 refinements exceeds max 10",
			refinementCount: 15,
			maxRefinements:  10,
		},
		{
			name:            "101 refinements exceeds max 100",
			refinementCount: 101,
			maxRefinements:  100,
		},
		{
			name:            "1000 refinements exceeds max 10",
			refinementCount: 1000,
			maxRefinements:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createRefinementTestNode(t, "1.3")

			err := node.ValidateRefinementCount(n, tt.refinementCount, tt.maxRefinements)
			if err == nil {
				t.Errorf("ValidateRefinementCount() = nil, want REFINEMENT_LIMIT_EXCEEDED error for count %d > maxRefinements %d",
					tt.refinementCount, tt.maxRefinements)
			}
		})
	}
}

// ===========================================================================
// Error code tests
// ===========================================================================

// TestValidateRefinementCount_ReturnsRefinementLimitExceededError tests that the correct error code is returned.
func TestValidateRefinementCount_ReturnsRefinementLimitExceededError(t *testing.T) {
	n := createRefinementTestNode(t, "1.1")

	err := node.ValidateRefinementCount(n, 15, 10)
	if err == nil {
		t.Fatal("ValidateRefinementCount() = nil, want error")
	}

	// Check that the error code is REFINEMENT_LIMIT_EXCEEDED
	code := aferrors.Code(err)
	if code != aferrors.REFINEMENT_LIMIT_EXCEEDED {
		t.Errorf("Error code = %v, want REFINEMENT_LIMIT_EXCEEDED", code)
	}
}

// TestValidateRefinementCount_ErrorExitCode tests that the error has the correct exit code.
func TestValidateRefinementCount_ErrorExitCode(t *testing.T) {
	n := createRefinementTestNode(t, "1.2")

	err := node.ValidateRefinementCount(n, 20, 10)
	if err == nil {
		t.Fatal("ValidateRefinementCount() = nil, want error")
	}

	// REFINEMENT_LIMIT_EXCEEDED should have exit code 3 (logic error)
	exitCode := aferrors.ExitCode(err)
	if exitCode != 3 {
		t.Errorf("ExitCode = %d, want 3 (logic error)", exitCode)
	}
}

// TestValidateRefinementCount_ErrorMessage tests that the error message contains useful information.
func TestValidateRefinementCount_ErrorMessage(t *testing.T) {
	refinementCount := 12
	maxRefinements := 10
	n := createRefinementTestNode(t, "1.3")

	err := node.ValidateRefinementCount(n, refinementCount, maxRefinements)
	if err == nil {
		t.Fatal("ValidateRefinementCount() = nil, want error")
	}

	errMsg := err.Error()

	// Error should mention the refinement count value
	if !strings.Contains(errMsg, "12") {
		t.Errorf("Error message should contain refinement count '12', got: %s", errMsg)
	}

	// Error should mention the max refinements value
	if !strings.Contains(errMsg, "10") {
		t.Errorf("Error message should contain maxRefinements '10', got: %s", errMsg)
	}
}

// ===========================================================================
// Edge case tests - Zero refinements
// ===========================================================================

// TestValidateRefinementCount_ZeroRefinements tests validation with zero refinement count.
func TestValidateRefinementCount_ZeroRefinements(t *testing.T) {
	tests := []struct {
		name           string
		maxRefinements int
		expectError    bool
	}{
		{
			name:           "0 refinements with max 1",
			maxRefinements: 1,
			expectError:    false,
		},
		{
			name:           "0 refinements with max 10",
			maxRefinements: 10,
			expectError:    false,
		},
		{
			name:           "0 refinements with max 100",
			maxRefinements: 100,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createRefinementTestNode(t, "1.1")

			err := node.ValidateRefinementCount(n, 0, tt.maxRefinements)
			if tt.expectError && err == nil {
				t.Error("ValidateRefinementCount() = nil, want error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateRefinementCount() = %v, want nil", err)
			}
		})
	}
}

// ===========================================================================
// Edge case tests - Exactly max
// ===========================================================================

// TestValidateRefinementCount_ExactlyMax tests that refinement count exactly at max is rejected.
func TestValidateRefinementCount_ExactlyMax(t *testing.T) {
	tests := []struct {
		name           string
		maxRefinements int
	}{
		{
			name:           "exactly 1 of 1",
			maxRefinements: 1,
		},
		{
			name:           "exactly 10 of 10",
			maxRefinements: 10,
		},
		{
			name:           "exactly 20 of 20",
			maxRefinements: 20,
		},
		{
			name:           "exactly 100 of 100",
			maxRefinements: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createRefinementTestNode(t, "1.2")

			err := node.ValidateRefinementCount(n, tt.maxRefinements, tt.maxRefinements)
			if err == nil {
				t.Errorf("ValidateRefinementCount() = nil, want error for count %d == maxRefinements",
					tt.maxRefinements)
			}
		})
	}
}

// ===========================================================================
// Edge case tests - Max + 1
// ===========================================================================

// TestValidateRefinementCount_MaxPlusOne tests that refinement count of max+1 is rejected.
func TestValidateRefinementCount_MaxPlusOne(t *testing.T) {
	tests := []struct {
		name           string
		maxRefinements int
	}{
		{
			name:           "2 exceeds max 1",
			maxRefinements: 1,
		},
		{
			name:           "11 exceeds max 10",
			maxRefinements: 10,
		},
		{
			name:           "21 exceeds max 20",
			maxRefinements: 20,
		},
		{
			name:           "101 exceeds max 100",
			maxRefinements: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createRefinementTestNode(t, "1.3")

			err := node.ValidateRefinementCount(n, tt.maxRefinements+1, tt.maxRefinements)
			if err == nil {
				t.Errorf("ValidateRefinementCount() = nil, want error for count %d > maxRefinements %d",
					tt.maxRefinements+1, tt.maxRefinements)
			}
		})
	}
}

// ===========================================================================
// Edge case tests - Nil node
// ===========================================================================

// TestValidateRefinementCount_NilNode tests that nil node is handled gracefully.
func TestValidateRefinementCount_NilNode(t *testing.T) {
	err := node.ValidateRefinementCount(nil, 5, 10)
	if err == nil {
		t.Error("ValidateRefinementCount(nil, 5, 10) = nil, want error for nil node")
	}
}

// ===========================================================================
// Edge case tests - Invalid max refinements
// ===========================================================================

// TestValidateRefinementCount_ZeroMaxRefinements tests behavior with maxRefinements of 0.
func TestValidateRefinementCount_ZeroMaxRefinements(t *testing.T) {
	n := createRefinementTestNode(t, "1.1")

	// maxRefinements of 0 means no refinements allowed at all
	// Even 0 refinements at max 0 should be okay (can't add more)
	// But trying to validate with count 0 at max 0 - count is not exceeding
	err := node.ValidateRefinementCount(n, 0, 0)
	if err == nil {
		t.Error("ValidateRefinementCount() = nil, want error for maxRefinements 0 (no refinements allowed)")
	}
}

// TestValidateRefinementCount_NegativeMaxRefinements tests behavior with negative maxRefinements.
func TestValidateRefinementCount_NegativeMaxRefinements(t *testing.T) {
	n := createRefinementTestNode(t, "1.1")

	// Negative maxRefinements is invalid
	err := node.ValidateRefinementCount(n, 0, -1)
	if err == nil {
		t.Error("ValidateRefinementCount() = nil, want error for negative maxRefinements")
	}
}

// TestValidateRefinementCount_NegativeRefinementCount tests behavior with negative refinement count.
func TestValidateRefinementCount_NegativeRefinementCount(t *testing.T) {
	n := createRefinementTestNode(t, "1.1")

	// Negative refinement count is invalid (should never happen)
	err := node.ValidateRefinementCount(n, -1, 10)
	if err == nil {
		t.Error("ValidateRefinementCount() = nil, want error for negative refinement count")
	}
}

// ===========================================================================
// Table-driven comprehensive tests
// ===========================================================================

// TestValidateRefinementCount_TableDriven provides comprehensive table-driven tests.
func TestValidateRefinementCount_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		refinementCount int
		maxRefinements  int
		expectError     bool
		errorCode       aferrors.ErrorCode
	}{
		{
			name:            "0 refinements max 10",
			refinementCount: 0,
			maxRefinements:  10,
			expectError:     false,
		},
		{
			name:            "5 refinements max 10",
			refinementCount: 5,
			maxRefinements:  10,
			expectError:     false,
		},
		{
			name:            "9 refinements max 10 (one below)",
			refinementCount: 9,
			maxRefinements:  10,
			expectError:     false,
		},
		{
			name:            "10 refinements max 10 (at limit)",
			refinementCount: 10,
			maxRefinements:  10,
			expectError:     true,
			errorCode:       aferrors.REFINEMENT_LIMIT_EXCEEDED,
		},
		{
			name:            "11 refinements max 10 (one over)",
			refinementCount: 11,
			maxRefinements:  10,
			expectError:     true,
			errorCode:       aferrors.REFINEMENT_LIMIT_EXCEEDED,
		},
		{
			name:            "100 refinements max 10 (way over)",
			refinementCount: 100,
			maxRefinements:  10,
			expectError:     true,
			errorCode:       aferrors.REFINEMENT_LIMIT_EXCEEDED,
		},
		{
			name:            "0 refinements max 1",
			refinementCount: 0,
			maxRefinements:  1,
			expectError:     false,
		},
		{
			name:            "1 refinement max 1 (at limit)",
			refinementCount: 1,
			maxRefinements:  1,
			expectError:     true,
			errorCode:       aferrors.REFINEMENT_LIMIT_EXCEEDED,
		},
		{
			name:            "2 refinements max 1 (over limit)",
			refinementCount: 2,
			maxRefinements:  1,
			expectError:     true,
			errorCode:       aferrors.REFINEMENT_LIMIT_EXCEEDED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createRefinementTestNode(t, "1.1")

			err := node.ValidateRefinementCount(n, tt.refinementCount, tt.maxRefinements)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateRefinementCount() = nil, want error for count %d >= maxRefinements %d",
						tt.refinementCount, tt.maxRefinements)
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
					t.Errorf("ValidateRefinementCount() = %v, want nil for count %d < maxRefinements %d",
						err, tt.refinementCount, tt.maxRefinements)
				}
			}
		})
	}
}

// ===========================================================================
// Different max refinements configuration tests
// ===========================================================================

// TestValidateRefinementCount_DifferentMaxConfigs tests various max refinement configurations.
func TestValidateRefinementCount_DifferentMaxConfigs(t *testing.T) {
	maxRefinements := []int{1, 5, 10, 20, 50, 100}

	for _, max := range maxRefinements {
		t.Run("max_"+string(rune('0'+max%10)), func(t *testing.T) {
			// Test below limit
			t.Run("below_limit", func(t *testing.T) {
				n := createRefinementTestNode(t, "1.1")
				err := node.ValidateRefinementCount(n, 0, max)
				if err != nil {
					t.Errorf("ValidateRefinementCount() = %v, want nil for count 0 < maxRefinements %d", err, max)
				}
			})

			// Test at limit (blocked)
			t.Run("at_limit", func(t *testing.T) {
				n := createRefinementTestNode(t, "1.2")
				err := node.ValidateRefinementCount(n, max, max)
				if err == nil {
					t.Errorf("ValidateRefinementCount() = nil, want error for count %d == maxRefinements %d", max, max)
				}
			})

			// Test over limit
			t.Run("over_limit", func(t *testing.T) {
				n := createRefinementTestNode(t, "1.3")
				err := node.ValidateRefinementCount(n, max+1, max)
				if err == nil {
					t.Errorf("ValidateRefinementCount() = nil, want error for count %d > maxRefinements %d", max+1, max)
				}
			})
		})
	}
}

// TestValidateRefinementCount_DefaultMaxRefinements tests with the default max refinements of 10.
func TestValidateRefinementCount_DefaultMaxRefinements(t *testing.T) {
	defaultMaxRefinements := 10

	tests := []struct {
		name            string
		refinementCount int
		expectError     bool
	}{
		{"zero refinements", 0, false},
		{"few refinements", 3, false},
		{"half of max", 5, false},
		{"near limit", 9, false},
		{"at limit", 10, true},
		{"over limit", 11, true},
		{"way over limit", 50, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createRefinementTestNode(t, "1.1")

			err := node.ValidateRefinementCount(n, tt.refinementCount, defaultMaxRefinements)
			if tt.expectError && err == nil {
				t.Error("ValidateRefinementCount() = nil, want error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateRefinementCount() = %v, want nil", err)
			}
		})
	}
}

// ===========================================================================
// Boundary tests
// ===========================================================================

// TestValidateRefinementCount_BoundaryConditions tests boundary conditions precisely.
func TestValidateRefinementCount_BoundaryConditions(t *testing.T) {
	// Test the exact boundary: max-1, max, max+1
	maxRefinements := 10

	t.Run("one_under_limit", func(t *testing.T) {
		n := createRefinementTestNode(t, "1.1")
		err := node.ValidateRefinementCount(n, maxRefinements-1, maxRefinements)
		if err != nil {
			t.Errorf("ValidateRefinementCount() = %v, want nil for count == max-1", err)
		}
	})

	t.Run("exactly_at_limit", func(t *testing.T) {
		n := createRefinementTestNode(t, "1.2")
		err := node.ValidateRefinementCount(n, maxRefinements, maxRefinements)
		if err == nil {
			t.Error("ValidateRefinementCount() = nil, want error for count == max")
		}
	})

	t.Run("one_over_limit", func(t *testing.T) {
		n := createRefinementTestNode(t, "1.3")
		err := node.ValidateRefinementCount(n, maxRefinements+1, maxRefinements)
		if err == nil {
			t.Error("ValidateRefinementCount() = nil, want error for count == max+1")
		}
	})
}

// ===========================================================================
// Error verification tests
// ===========================================================================

// TestValidateRefinementCount_AtLimit_HasCorrectErrorCode tests error code at limit.
func TestValidateRefinementCount_AtLimit_HasCorrectErrorCode(t *testing.T) {
	n := createRefinementTestNode(t, "1.1")

	err := node.ValidateRefinementCount(n, 10, 10)
	if err == nil {
		t.Fatal("ValidateRefinementCount() = nil, want error")
	}

	code := aferrors.Code(err)
	if code != aferrors.REFINEMENT_LIMIT_EXCEEDED {
		t.Errorf("Error code = %v, want REFINEMENT_LIMIT_EXCEEDED", code)
	}

	exitCode := aferrors.ExitCode(err)
	if exitCode != 3 {
		t.Errorf("ExitCode = %d, want 3 (logic error)", exitCode)
	}
}

// TestValidateRefinementCount_OverLimit_HasCorrectErrorCode tests error code over limit.
func TestValidateRefinementCount_OverLimit_HasCorrectErrorCode(t *testing.T) {
	n := createRefinementTestNode(t, "1.2")

	err := node.ValidateRefinementCount(n, 15, 10)
	if err == nil {
		t.Fatal("ValidateRefinementCount() = nil, want error")
	}

	code := aferrors.Code(err)
	if code != aferrors.REFINEMENT_LIMIT_EXCEEDED {
		t.Errorf("Error code = %v, want REFINEMENT_LIMIT_EXCEEDED", code)
	}

	exitCode := aferrors.ExitCode(err)
	if exitCode != 3 {
		t.Errorf("ExitCode = %d, want 3 (logic error)", exitCode)
	}
}

// ===========================================================================
// Different node ID tests
// ===========================================================================

// TestValidateRefinementCount_DifferentNodeIDs tests validation with various node IDs.
func TestValidateRefinementCount_DifferentNodeIDs(t *testing.T) {
	nodeIDs := []string{"1", "1.1", "1.2.3", "1.1.1.1", "1.2.3.4.5"}

	for _, idStr := range nodeIDs {
		t.Run("node_"+idStr, func(t *testing.T) {
			n := createRefinementTestNode(t, idStr)

			// Under limit should pass
			err := node.ValidateRefinementCount(n, 5, 10)
			if err != nil {
				t.Errorf("ValidateRefinementCount() = %v for node %s, want nil", err, idStr)
			}

			// At limit should fail
			err = node.ValidateRefinementCount(n, 10, 10)
			if err == nil {
				t.Errorf("ValidateRefinementCount() = nil for node %s at limit, want error", idStr)
			}
		})
	}
}

// ===========================================================================
// Semantic tests - Allowed vs Blocked
// ===========================================================================

// TestValidateRefinementCount_AllowsRefinement tests that validation allows refinement under limit.
func TestValidateRefinementCount_AllowsRefinement(t *testing.T) {
	n := createRefinementTestNode(t, "1.1")

	// With 5 refinements and max 10, we should be allowed to refine further
	err := node.ValidateRefinementCount(n, 5, 10)
	if err != nil {
		t.Errorf("ValidateRefinementCount() = %v, want nil (refinement should be allowed)", err)
	}
}

// TestValidateRefinementCount_BlocksRefinement tests that validation blocks refinement at/over limit.
func TestValidateRefinementCount_BlocksRefinement(t *testing.T) {
	n := createRefinementTestNode(t, "1.2")

	// With 10 refinements and max 10, we should NOT be allowed to refine further
	err := node.ValidateRefinementCount(n, 10, 10)
	if err == nil {
		t.Error("ValidateRefinementCount() = nil, want error (refinement should be blocked)")
	}
}
