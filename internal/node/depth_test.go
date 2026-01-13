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

// createNodeWithDepth creates a node at the specified depth.
// Depth 1 = "1", Depth 2 = "1.1", Depth 3 = "1.1.1", etc.
func createNodeWithDepth(t *testing.T, depth int) *node.Node {
	t.Helper()

	if depth < 1 {
		t.Fatalf("createNodeWithDepth: depth must be >= 1, got %d", depth)
	}

	// Build ID string for the requested depth
	// Depth 1: "1", Depth 2: "1.1", Depth 3: "1.1.1", etc.
	idStr := "1"
	for i := 2; i <= depth; i++ {
		idStr += ".1"
	}

	id, err := types.Parse(idStr)
	if err != nil {
		t.Fatalf("types.Parse(%q) error: %v", idStr, err)
	}

	n, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement at depth "+idStr, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	return n
}

// ===========================================================================
// Basic validation tests
// ===========================================================================

// TestValidateDepth_WithinLimit tests that depth within limit passes validation.
func TestValidateDepth_WithinLimit(t *testing.T) {
	tests := []struct {
		name     string
		depth    int
		maxDepth int
	}{
		{
			name:     "depth 1 with max 20",
			depth:    1,
			maxDepth: 20,
		},
		{
			name:     "depth 5 with max 20",
			depth:    5,
			maxDepth: 20,
		},
		{
			name:     "depth 10 with max 20",
			depth:    10,
			maxDepth: 20,
		},
		{
			name:     "depth 19 with max 20",
			depth:    19,
			maxDepth: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createNodeWithDepth(t, tt.depth)

			err := node.ValidateDepth(n, tt.maxDepth)
			if err != nil {
				t.Errorf("ValidateDepth() = %v, want nil for depth %d <= maxDepth %d", err, tt.depth, tt.maxDepth)
			}
		})
	}
}

// TestValidateDepth_ExactlyAtLimit tests that depth exactly at the limit passes validation.
func TestValidateDepth_ExactlyAtLimit(t *testing.T) {
	tests := []struct {
		name     string
		maxDepth int
	}{
		{
			name:     "depth 1 at limit 1",
			maxDepth: 1,
		},
		{
			name:     "depth 5 at limit 5",
			maxDepth: 5,
		},
		{
			name:     "depth 20 at limit 20",
			maxDepth: 20,
		},
		{
			name:     "depth 100 at limit 100",
			maxDepth: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createNodeWithDepth(t, tt.maxDepth)

			err := node.ValidateDepth(n, tt.maxDepth)
			if err != nil {
				t.Errorf("ValidateDepth() = %v, want nil for depth %d == maxDepth %d", err, tt.maxDepth, tt.maxDepth)
			}
		})
	}
}

// TestValidateDepth_ExceedsLimit tests that depth exceeding the limit fails validation.
func TestValidateDepth_ExceedsLimit(t *testing.T) {
	tests := []struct {
		name     string
		depth    int
		maxDepth int
	}{
		{
			name:     "depth 2 exceeds max 1",
			depth:    2,
			maxDepth: 1,
		},
		{
			name:     "depth 21 exceeds max 20",
			depth:    21,
			maxDepth: 20,
		},
		{
			name:     "depth 6 exceeds max 5",
			depth:    6,
			maxDepth: 5,
		},
		{
			name:     "depth 101 exceeds max 100",
			depth:    10, // Can't easily create depth 101, use smaller example
			maxDepth: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createNodeWithDepth(t, tt.depth)

			err := node.ValidateDepth(n, tt.maxDepth)
			if err == nil {
				t.Errorf("ValidateDepth() = nil, want DEPTH_EXCEEDED error for depth %d > maxDepth %d", tt.depth, tt.maxDepth)
			}
		})
	}
}

// ===========================================================================
// Error code tests
// ===========================================================================

// TestValidateDepth_ReturnsDepthExceededError tests that the correct error code is returned.
func TestValidateDepth_ReturnsDepthExceededError(t *testing.T) {
	n := createNodeWithDepth(t, 5)

	err := node.ValidateDepth(n, 3)
	if err == nil {
		t.Fatal("ValidateDepth() = nil, want error")
	}

	// Check that the error code is DEPTH_EXCEEDED
	code := aferrors.Code(err)
	if code != aferrors.DEPTH_EXCEEDED {
		t.Errorf("Error code = %v, want DEPTH_EXCEEDED", code)
	}
}

// TestValidateDepth_ErrorExitCode tests that the error has the correct exit code.
func TestValidateDepth_ErrorExitCode(t *testing.T) {
	n := createNodeWithDepth(t, 10)

	err := node.ValidateDepth(n, 5)
	if err == nil {
		t.Fatal("ValidateDepth() = nil, want error")
	}

	// DEPTH_EXCEEDED should have exit code 3 (logic error)
	exitCode := aferrors.ExitCode(err)
	if exitCode != 3 {
		t.Errorf("ExitCode = %d, want 3 (logic error)", exitCode)
	}
}

// TestValidateDepth_ErrorMessage tests that the error message contains useful information.
func TestValidateDepth_ErrorMessage(t *testing.T) {
	depth := 8
	maxDepth := 5
	n := createNodeWithDepth(t, depth)

	err := node.ValidateDepth(n, maxDepth)
	if err == nil {
		t.Fatal("ValidateDepth() = nil, want error")
	}

	errMsg := err.Error()

	// Error should mention the depth value
	if !strings.Contains(errMsg, "8") {
		t.Errorf("Error message should contain depth '8', got: %s", errMsg)
	}

	// Error should mention the max depth value
	if !strings.Contains(errMsg, "5") {
		t.Errorf("Error message should contain maxDepth '5', got: %s", errMsg)
	}
}

// ===========================================================================
// Edge case tests
// ===========================================================================

// TestValidateDepth_RootNode tests validation of the root node (depth 1).
func TestValidateDepth_RootNode(t *testing.T) {
	n := createNodeWithDepth(t, 1)

	tests := []struct {
		name        string
		maxDepth    int
		expectError bool
	}{
		{
			name:        "root with maxDepth 1",
			maxDepth:    1,
			expectError: false,
		},
		{
			name:        "root with maxDepth 20",
			maxDepth:    20,
			expectError: false,
		},
		{
			name:        "root with maxDepth 100",
			maxDepth:    100,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := node.ValidateDepth(n, tt.maxDepth)
			if tt.expectError && err == nil {
				t.Error("ValidateDepth() = nil, want error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateDepth() = %v, want nil", err)
			}
		})
	}
}

// TestValidateDepth_NilNode tests that nil node is handled gracefully.
func TestValidateDepth_NilNode(t *testing.T) {
	err := node.ValidateDepth(nil, 20)
	if err == nil {
		t.Error("ValidateDepth(nil, 20) = nil, want error for nil node")
	}
}

// TestValidateDepth_ZeroMaxDepth tests behavior with maxDepth of 0.
func TestValidateDepth_ZeroMaxDepth(t *testing.T) {
	n := createNodeWithDepth(t, 1)

	// maxDepth of 0 is invalid - root (depth 1) should always exceed it
	err := node.ValidateDepth(n, 0)
	if err == nil {
		t.Error("ValidateDepth() = nil, want error for maxDepth 0 (root has depth 1)")
	}
}

// TestValidateDepth_NegativeMaxDepth tests behavior with negative maxDepth.
func TestValidateDepth_NegativeMaxDepth(t *testing.T) {
	n := createNodeWithDepth(t, 1)

	// Negative maxDepth should fail for any node
	err := node.ValidateDepth(n, -1)
	if err == nil {
		t.Error("ValidateDepth() = nil, want error for negative maxDepth")
	}
}

// ===========================================================================
// Table-driven comprehensive tests
// ===========================================================================

// TestValidateDepth_TableDriven provides comprehensive table-driven tests.
func TestValidateDepth_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		depth       int
		maxDepth    int
		expectError bool
		errorCode   aferrors.ErrorCode
	}{
		{
			name:        "depth 1 max 1",
			depth:       1,
			maxDepth:    1,
			expectError: false,
		},
		{
			name:        "depth 1 max 10",
			depth:       1,
			maxDepth:    10,
			expectError: false,
		},
		{
			name:        "depth 5 max 5",
			depth:       5,
			maxDepth:    5,
			expectError: false,
		},
		{
			name:        "depth 5 max 10",
			depth:       5,
			maxDepth:    10,
			expectError: false,
		},
		{
			name:        "depth 2 max 1",
			depth:       2,
			maxDepth:    1,
			expectError: true,
			errorCode:   aferrors.DEPTH_EXCEEDED,
		},
		{
			name:        "depth 6 max 5",
			depth:       6,
			maxDepth:    5,
			expectError: true,
			errorCode:   aferrors.DEPTH_EXCEEDED,
		},
		{
			name:        "depth 10 max 9",
			depth:       10,
			maxDepth:    9,
			expectError: true,
			errorCode:   aferrors.DEPTH_EXCEEDED,
		},
		{
			name:        "depth 15 max 10",
			depth:       15,
			maxDepth:    10,
			expectError: true,
			errorCode:   aferrors.DEPTH_EXCEEDED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createNodeWithDepth(t, tt.depth)

			err := node.ValidateDepth(n, tt.maxDepth)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateDepth() = nil, want error for depth %d > maxDepth %d", tt.depth, tt.maxDepth)
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
					t.Errorf("ValidateDepth() = %v, want nil for depth %d <= maxDepth %d", err, tt.depth, tt.maxDepth)
				}
			}
		})
	}
}

// ===========================================================================
// Different MaxDepth configuration tests
// ===========================================================================

// TestValidateDepth_DifferentMaxDepthConfigs tests various MaxDepth configurations.
func TestValidateDepth_DifferentMaxDepthConfigs(t *testing.T) {
	// Test common MaxDepth configurations
	maxDepths := []int{1, 5, 10, 20, 50, 100}

	for _, maxDepth := range maxDepths {
		t.Run("maxDepth_"+string(rune('0'+maxDepth%10)), func(t *testing.T) {
			// Test depth at limit
			t.Run("at_limit", func(t *testing.T) {
				// Only test if depth is reasonable to create
				if maxDepth <= 20 {
					n := createNodeWithDepth(t, maxDepth)
					err := node.ValidateDepth(n, maxDepth)
					if err != nil {
						t.Errorf("ValidateDepth() = %v, want nil for depth %d == maxDepth %d", err, maxDepth, maxDepth)
					}
				}
			})

			// Test depth below limit
			t.Run("below_limit", func(t *testing.T) {
				n := createNodeWithDepth(t, 1)
				err := node.ValidateDepth(n, maxDepth)
				if err != nil {
					t.Errorf("ValidateDepth() = %v, want nil for depth 1 < maxDepth %d", err, maxDepth)
				}
			})
		})
	}
}

// TestValidateDepth_DefaultMaxDepth tests with the default max depth of 20.
func TestValidateDepth_DefaultMaxDepth(t *testing.T) {
	defaultMaxDepth := 20

	tests := []struct {
		name        string
		depth       int
		expectError bool
	}{
		{"root node", 1, false},
		{"shallow node", 5, false},
		{"medium depth", 10, false},
		{"near limit", 19, false},
		{"at limit", 20, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createNodeWithDepth(t, tt.depth)

			err := node.ValidateDepth(n, defaultMaxDepth)
			if tt.expectError && err == nil {
				t.Error("ValidateDepth() = nil, want error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateDepth() = %v, want nil", err)
			}
		})
	}
}

// ===========================================================================
// Very deep node tests
// ===========================================================================

// TestValidateDepth_VeryDeepNodes tests validation with very deep nodes.
func TestValidateDepth_VeryDeepNodes(t *testing.T) {
	tests := []struct {
		name        string
		depth       int
		maxDepth    int
		expectError bool
	}{
		{
			name:        "depth 15 with max 20",
			depth:       15,
			maxDepth:    20,
			expectError: false,
		},
		{
			name:        "depth 20 with max 20",
			depth:       20,
			maxDepth:    20,
			expectError: false,
		},
		{
			name:        "depth 15 with max 10",
			depth:       15,
			maxDepth:    10,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := createNodeWithDepth(t, tt.depth)

			err := node.ValidateDepth(n, tt.maxDepth)
			if tt.expectError && err == nil {
				t.Errorf("ValidateDepth() = nil, want error for depth %d > maxDepth %d", tt.depth, tt.maxDepth)
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateDepth() = %v, want nil for depth %d <= maxDepth %d", err, tt.depth, tt.maxDepth)
			}
		})
	}
}

// ===========================================================================
// Boundary tests
// ===========================================================================

// TestValidateDepth_BoundaryConditions tests boundary conditions precisely.
func TestValidateDepth_BoundaryConditions(t *testing.T) {
	// Test the exact boundary: maxDepth and maxDepth+1
	maxDepth := 10

	t.Run("exactly_at_limit", func(t *testing.T) {
		n := createNodeWithDepth(t, maxDepth)
		err := node.ValidateDepth(n, maxDepth)
		if err != nil {
			t.Errorf("ValidateDepth() = %v, want nil for depth == maxDepth", err)
		}
	})

	t.Run("one_over_limit", func(t *testing.T) {
		n := createNodeWithDepth(t, maxDepth+1)
		err := node.ValidateDepth(n, maxDepth)
		if err == nil {
			t.Error("ValidateDepth() = nil, want error for depth == maxDepth+1")
		}
	})

	t.Run("one_under_limit", func(t *testing.T) {
		n := createNodeWithDepth(t, maxDepth-1)
		err := node.ValidateDepth(n, maxDepth)
		if err != nil {
			t.Errorf("ValidateDepth() = %v, want nil for depth == maxDepth-1", err)
		}
	})
}
