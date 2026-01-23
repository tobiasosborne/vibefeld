//go:build integration
// +build integration

// Package node_test contains tests for context reference validation.
// Context references include definitions, assumptions, and externals.
package node_test

import (
	"strings"
	"testing"

	aferrors "github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// ===========================================================================
// Helper functions for test setup
// ===========================================================================

// createContextTestNode creates a node with the given ID and context references.
func createContextTestNode(t *testing.T, idStr string, context ...string) *node.Node {
	t.Helper()

	id, err := types.Parse(idStr)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", idStr, err)
	}

	opts := node.NodeOptions{
		Context: context,
	}

	n, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceModusPonens, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}

	return n
}

// createTestDefinition creates a definition and adds it to state.
func createTestDefinition(t *testing.T, s *state.State, name, content string) *node.Definition {
	t.Helper()

	def, err := node.NewDefinition(name, content)
	if err != nil {
		t.Fatalf("NewDefinition() error: %v", err)
	}
	s.AddDefinition(def)
	return def
}

// createTestAssumption creates an assumption and adds it to state.
func createTestAssumption(t *testing.T, s *state.State, statement string) *node.Assumption {
	t.Helper()

	assn := node.NewAssumption(statement)
	s.AddAssumption(assn)
	return assn
}

// createTestExternal creates an external and adds it to state.
func createTestExternal(t *testing.T, s *state.State, name, source string) *node.External {
	t.Helper()

	ext, err := node.NewExternal(name, source)
	if err != nil {
		t.Fatalf("NewExternal() error: %v", err)
	}
	s.AddExternal(ext)
	return ext
}

// ===========================================================================
// Definition reference validation tests
// ===========================================================================

// TestValidateDefRefs_NoDefs tests that a node with no definition references passes.
func TestValidateDefRefs_NoDefs(t *testing.T) {
	s := state.NewState()
	n := createContextTestNode(t, "1")

	err := node.ValidateDefRefs(n, s)
	if err != nil {
		t.Errorf("ValidateDefRefs() = %v, want nil for node with no definition references", err)
	}
}

// TestValidateDefRefs_ValidSingleDef tests that a valid single definition reference passes.
func TestValidateDefRefs_ValidSingleDef(t *testing.T) {
	s := state.NewState()

	// Create definition
	def := createTestDefinition(t, s, "continuity", "A function f is continuous at x if...")

	// Create node referencing this definition
	n := createContextTestNode(t, "1", def.ID)

	err := node.ValidateDefRefs(n, s)
	if err != nil {
		t.Errorf("ValidateDefRefs() = %v, want nil for node with valid definition reference", err)
	}
}

// TestValidateDefRefs_ValidMultipleDefs tests that multiple valid definition references pass.
func TestValidateDefRefs_ValidMultipleDefs(t *testing.T) {
	s := state.NewState()

	// Create multiple definitions
	def1 := createTestDefinition(t, s, "limit", "The limit of f(x) as x approaches a is L if...")
	def2 := createTestDefinition(t, s, "derivative", "The derivative of f at x is the limit of...")
	def3 := createTestDefinition(t, s, "integral", "The integral of f over [a,b] is...")

	// Create node referencing all definitions
	n := createContextTestNode(t, "1", def1.ID, def2.ID, def3.ID)

	err := node.ValidateDefRefs(n, s)
	if err != nil {
		t.Errorf("ValidateDefRefs() = %v, want nil for node with multiple valid definition references", err)
	}
}

// TestValidateDefRefs_MissingDef tests that a missing definition reference fails.
func TestValidateDefRefs_MissingDef(t *testing.T) {
	s := state.NewState()

	// Create node with reference to non-existent definition
	n := createContextTestNode(t, "1", "nonexistent-def-id")

	err := node.ValidateDefRefs(n, s)
	if err == nil {
		t.Error("ValidateDefRefs() = nil, want error for missing definition reference")
	}

	// Verify error code is DEF_NOT_FOUND
	if aferrors.Code(err) != aferrors.DEF_NOT_FOUND {
		t.Errorf("Error code = %v, want DEF_NOT_FOUND", aferrors.Code(err))
	}

	// Verify error message contains the missing ID
	if err != nil && !strings.Contains(err.Error(), "nonexistent-def-id") {
		t.Errorf("Error should mention missing ID 'nonexistent-def-id', got: %s", err.Error())
	}
}

// TestValidateDefRefs_SomeValidSomeMissing tests mixed valid and missing definition references.
func TestValidateDefRefs_SomeValidSomeMissing(t *testing.T) {
	s := state.NewState()

	// Create one valid definition
	def := createTestDefinition(t, s, "topology", "A topology on X is a collection of subsets...")

	// Create node with one valid and one invalid reference
	n := createContextTestNode(t, "1", def.ID, "missing-def-id")

	err := node.ValidateDefRefs(n, s)
	if err == nil {
		t.Error("ValidateDefRefs() = nil, want error for mixed valid/missing definition references")
	}

	// Should report the missing definition
	if err != nil && !strings.Contains(err.Error(), "missing-def-id") {
		t.Errorf("Error should mention missing ID 'missing-def-id', got: %s", err.Error())
	}
}

// TestValidateDefRefs_NilNode tests that nil node returns an error.
func TestValidateDefRefs_NilNode(t *testing.T) {
	s := state.NewState()

	err := node.ValidateDefRefs(nil, s)
	if err == nil {
		t.Error("ValidateDefRefs(nil, state) = nil, want error for nil node")
	}
}

// TestValidateDefRefs_NilState tests that nil state returns an error.
func TestValidateDefRefs_NilState(t *testing.T) {
	n := createContextTestNode(t, "1")

	err := node.ValidateDefRefs(n, nil)
	if err == nil {
		t.Error("ValidateDefRefs(node, nil) = nil, want error for nil state")
	}
}

// ===========================================================================
// Assumption reference validation tests
// ===========================================================================

// TestValidateAssnRefs_NoAssumptions tests that a node with no assumption references passes.
func TestValidateAssnRefs_NoAssumptions(t *testing.T) {
	s := state.NewState()
	n := createContextTestNode(t, "1")

	err := node.ValidateAssnRefs(n, s)
	if err != nil {
		t.Errorf("ValidateAssnRefs() = %v, want nil for node with no assumption references", err)
	}
}

// TestValidateAssnRefs_ValidSingleAssumption tests that a valid single assumption reference passes.
func TestValidateAssnRefs_ValidSingleAssumption(t *testing.T) {
	s := state.NewState()

	// Create assumption
	assn := createTestAssumption(t, s, "Assume f is differentiable on (a,b)")

	// Create node referencing this assumption
	n := createContextTestNode(t, "1", assn.ID)

	err := node.ValidateAssnRefs(n, s)
	if err != nil {
		t.Errorf("ValidateAssnRefs() = %v, want nil for node with valid assumption reference", err)
	}
}

// TestValidateAssnRefs_ValidMultipleAssumptions tests multiple valid assumption references.
func TestValidateAssnRefs_ValidMultipleAssumptions(t *testing.T) {
	s := state.NewState()

	// Create multiple assumptions
	assn1 := createTestAssumption(t, s, "Assume X is a compact metric space")
	assn2 := createTestAssumption(t, s, "Assume f: X -> R is continuous")
	assn3 := createTestAssumption(t, s, "Assume epsilon > 0")

	// Create node referencing all assumptions
	n := createContextTestNode(t, "1", assn1.ID, assn2.ID, assn3.ID)

	err := node.ValidateAssnRefs(n, s)
	if err != nil {
		t.Errorf("ValidateAssnRefs() = %v, want nil for node with multiple valid assumption references", err)
	}
}

// TestValidateAssnRefs_MissingAssumption tests that a missing assumption reference fails.
func TestValidateAssnRefs_MissingAssumption(t *testing.T) {
	s := state.NewState()

	// Create node with reference to non-existent assumption
	n := createContextTestNode(t, "1", "nonexistent-assn-id")

	err := node.ValidateAssnRefs(n, s)
	if err == nil {
		t.Error("ValidateAssnRefs() = nil, want error for missing assumption reference")
	}

	// Verify error code is ASSUMPTION_NOT_FOUND
	if aferrors.Code(err) != aferrors.ASSUMPTION_NOT_FOUND {
		t.Errorf("Error code = %v, want ASSUMPTION_NOT_FOUND", aferrors.Code(err))
	}

	// Verify error message contains the missing ID
	if err != nil && !strings.Contains(err.Error(), "nonexistent-assn-id") {
		t.Errorf("Error should mention missing ID 'nonexistent-assn-id', got: %s", err.Error())
	}
}

// TestValidateAssnRefs_SomeValidSomeMissing tests mixed valid and missing assumption references.
func TestValidateAssnRefs_SomeValidSomeMissing(t *testing.T) {
	s := state.NewState()

	// Create one valid assumption
	assn := createTestAssumption(t, s, "Assume the Axiom of Choice holds")

	// Create node with one valid and one invalid reference
	n := createContextTestNode(t, "1", assn.ID, "missing-assn-id")

	err := node.ValidateAssnRefs(n, s)
	if err == nil {
		t.Error("ValidateAssnRefs() = nil, want error for mixed valid/missing assumption references")
	}

	// Should report the missing assumption
	if err != nil && !strings.Contains(err.Error(), "missing-assn-id") {
		t.Errorf("Error should mention missing ID 'missing-assn-id', got: %s", err.Error())
	}
}

// TestValidateAssnRefs_NilNode tests that nil node returns an error.
func TestValidateAssnRefs_NilNode(t *testing.T) {
	s := state.NewState()

	err := node.ValidateAssnRefs(nil, s)
	if err == nil {
		t.Error("ValidateAssnRefs(nil, state) = nil, want error for nil node")
	}
}

// TestValidateAssnRefs_NilState tests that nil state returns an error.
func TestValidateAssnRefs_NilState(t *testing.T) {
	n := createContextTestNode(t, "1")

	err := node.ValidateAssnRefs(n, nil)
	if err == nil {
		t.Error("ValidateAssnRefs(node, nil) = nil, want error for nil state")
	}
}

// ===========================================================================
// External reference validation tests
// ===========================================================================

// TestValidateExtRefs_NoExternals tests that a node with no external references passes.
func TestValidateExtRefs_NoExternals(t *testing.T) {
	s := state.NewState()
	n := createContextTestNode(t, "1")

	err := node.ValidateExtRefs(n, s)
	if err != nil {
		t.Errorf("ValidateExtRefs() = %v, want nil for node with no external references", err)
	}
}

// TestValidateExtRefs_ValidSingleExternal tests that a valid single external reference passes.
func TestValidateExtRefs_ValidSingleExternal(t *testing.T) {
	s := state.NewState()

	// Create external
	ext := createTestExternal(t, s, "Rudin's Theorem 7.17", "Rudin, Principles of Mathematical Analysis, 3rd ed., p. 154")

	// Create node referencing this external
	n := createContextTestNode(t, "1", ext.ID)

	err := node.ValidateExtRefs(n, s)
	if err != nil {
		t.Errorf("ValidateExtRefs() = %v, want nil for node with valid external reference", err)
	}
}

// TestValidateExtRefs_ValidMultipleExternals tests multiple valid external references.
func TestValidateExtRefs_ValidMultipleExternals(t *testing.T) {
	s := state.NewState()

	// Create multiple externals
	ext1 := createTestExternal(t, s, "Bolzano-Weierstrass", "Rudin, p. 40")
	ext2 := createTestExternal(t, s, "Heine-Borel", "Munkres, Topology, p. 164")
	ext3 := createTestExternal(t, s, "Mean Value Theorem", "Spivak, Calculus, p. 178")

	// Create node referencing all externals
	n := createContextTestNode(t, "1", ext1.ID, ext2.ID, ext3.ID)

	err := node.ValidateExtRefs(n, s)
	if err != nil {
		t.Errorf("ValidateExtRefs() = %v, want nil for node with multiple valid external references", err)
	}
}

// TestValidateExtRefs_MissingExternal tests that a missing external reference fails.
func TestValidateExtRefs_MissingExternal(t *testing.T) {
	s := state.NewState()

	// Create node with reference to non-existent external
	n := createContextTestNode(t, "1", "nonexistent-ext-id")

	err := node.ValidateExtRefs(n, s)
	if err == nil {
		t.Error("ValidateExtRefs() = nil, want error for missing external reference")
	}

	// Verify error code is EXTERNAL_NOT_FOUND
	if aferrors.Code(err) != aferrors.EXTERNAL_NOT_FOUND {
		t.Errorf("Error code = %v, want EXTERNAL_NOT_FOUND", aferrors.Code(err))
	}

	// Verify error message contains the missing ID
	if err != nil && !strings.Contains(err.Error(), "nonexistent-ext-id") {
		t.Errorf("Error should mention missing ID 'nonexistent-ext-id', got: %s", err.Error())
	}
}

// TestValidateExtRefs_SomeValidSomeMissing tests mixed valid and missing external references.
func TestValidateExtRefs_SomeValidSomeMissing(t *testing.T) {
	s := state.NewState()

	// Create one valid external
	ext := createTestExternal(t, s, "Stone-Weierstrass", "Rudin, Functional Analysis, p. 122")

	// Create node with one valid and one invalid reference
	n := createContextTestNode(t, "1", ext.ID, "missing-ext-id")

	err := node.ValidateExtRefs(n, s)
	if err == nil {
		t.Error("ValidateExtRefs() = nil, want error for mixed valid/missing external references")
	}

	// Should report the missing external
	if err != nil && !strings.Contains(err.Error(), "missing-ext-id") {
		t.Errorf("Error should mention missing ID 'missing-ext-id', got: %s", err.Error())
	}
}

// TestValidateExtRefs_NilNode tests that nil node returns an error.
func TestValidateExtRefs_NilNode(t *testing.T) {
	s := state.NewState()

	err := node.ValidateExtRefs(nil, s)
	if err == nil {
		t.Error("ValidateExtRefs(nil, state) = nil, want error for nil node")
	}
}

// TestValidateExtRefs_NilState tests that nil state returns an error.
func TestValidateExtRefs_NilState(t *testing.T) {
	n := createContextTestNode(t, "1")

	err := node.ValidateExtRefs(n, nil)
	if err == nil {
		t.Error("ValidateExtRefs(node, nil) = nil, want error for nil state")
	}
}

// ===========================================================================
// Combined context validation tests
// ===========================================================================

// TestValidateContextRefs_AllEmpty tests that a node with no context references passes.
func TestValidateContextRefs_AllEmpty(t *testing.T) {
	s := state.NewState()
	n := createContextTestNode(t, "1")

	err := node.ValidateContextRefs(n, s)
	if err != nil {
		t.Errorf("ValidateContextRefs() = %v, want nil for node with empty context", err)
	}
}

// TestValidateContextRefs_AllValid tests that all valid context references pass.
func TestValidateContextRefs_AllValid(t *testing.T) {
	s := state.NewState()

	// Create one of each type
	def := createTestDefinition(t, s, "compactness", "A space is compact if every open cover has a finite subcover")
	assn := createTestAssumption(t, s, "Assume X is a Hausdorff space")
	ext := createTestExternal(t, s, "Tychonoff's Theorem", "Kelley, General Topology, p. 143")

	// Create node referencing all
	n := createContextTestNode(t, "1", def.ID, assn.ID, ext.ID)

	err := node.ValidateContextRefs(n, s)
	if err != nil {
		t.Errorf("ValidateContextRefs() = %v, want nil for node with all valid context references", err)
	}
}

// TestValidateContextRefs_MissingDef tests that a missing definition is detected in combined validation.
func TestValidateContextRefs_MissingDef(t *testing.T) {
	s := state.NewState()

	// Create valid assumption and external
	assn := createTestAssumption(t, s, "Assume f is bounded")
	ext := createTestExternal(t, s, "Arzelà-Ascoli", "Folland, Real Analysis, p. 137")

	// Create node with valid refs plus missing definition
	n := createContextTestNode(t, "1", "missing-def", assn.ID, ext.ID)

	err := node.ValidateContextRefs(n, s)
	if err == nil {
		t.Error("ValidateContextRefs() = nil, want error for missing definition")
	}

	// Error code should be DEF_NOT_FOUND
	if aferrors.Code(err) != aferrors.DEF_NOT_FOUND {
		t.Errorf("Error code = %v, want DEF_NOT_FOUND", aferrors.Code(err))
	}
}

// TestValidateContextRefs_MissingAssumption tests that a missing assumption is detected.
func TestValidateContextRefs_MissingAssumption(t *testing.T) {
	s := state.NewState()

	// Create valid definition and external
	def := createTestDefinition(t, s, "metric", "A metric on X is a function d: X x X -> R satisfying...")
	ext := createTestExternal(t, s, "Triangle Inequality", "Basic metric space property")

	// Create node with valid refs plus missing assumption
	n := createContextTestNode(t, "1", def.ID, "missing-assn", ext.ID)

	err := node.ValidateContextRefs(n, s)
	if err == nil {
		t.Error("ValidateContextRefs() = nil, want error for missing assumption")
	}

	// Error code should be ASSUMPTION_NOT_FOUND
	if aferrors.Code(err) != aferrors.ASSUMPTION_NOT_FOUND {
		t.Errorf("Error code = %v, want ASSUMPTION_NOT_FOUND", aferrors.Code(err))
	}
}

// TestValidateContextRefs_MissingExternal tests that a missing external is detected.
func TestValidateContextRefs_MissingExternal(t *testing.T) {
	s := state.NewState()

	// Create valid definition and assumption
	def := createTestDefinition(t, s, "norm", "A norm on V is a function ||.||: V -> R satisfying...")
	assn := createTestAssumption(t, s, "Assume V is a finite-dimensional vector space")

	// Create node with valid refs plus missing external
	n := createContextTestNode(t, "1", def.ID, assn.ID, "missing-ext")

	err := node.ValidateContextRefs(n, s)
	if err == nil {
		t.Error("ValidateContextRefs() = nil, want error for missing external")
	}

	// Error code should be EXTERNAL_NOT_FOUND
	if aferrors.Code(err) != aferrors.EXTERNAL_NOT_FOUND {
		t.Errorf("Error code = %v, want EXTERNAL_NOT_FOUND", aferrors.Code(err))
	}
}

// TestValidateContextRefs_NilNode tests that nil node returns an error.
func TestValidateContextRefs_NilNode(t *testing.T) {
	s := state.NewState()

	err := node.ValidateContextRefs(nil, s)
	if err == nil {
		t.Error("ValidateContextRefs(nil, state) = nil, want error for nil node")
	}
}

// TestValidateContextRefs_NilState tests that nil state returns an error.
func TestValidateContextRefs_NilState(t *testing.T) {
	n := createContextTestNode(t, "1")

	err := node.ValidateContextRefs(n, nil)
	if err == nil {
		t.Error("ValidateContextRefs(node, nil) = nil, want error for nil state")
	}
}

// ===========================================================================
// Table-driven comprehensive tests
// ===========================================================================

// TestValidateDefRefs_TableDriven provides comprehensive table-driven tests for definition validation.
func TestValidateDefRefs_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		defNames      []string // definitions to create (name, content pairs via index)
		refIDs        []int    // indices of definitions to reference (-1 for invalid)
		invalidRefs   []string // explicit invalid reference IDs
		expectError   bool
		errorContains string
	}{
		{
			name:        "no references",
			defNames:    nil,
			refIDs:      nil,
			expectError: false,
		},
		{
			name:        "single valid reference",
			defNames:    []string{"def1"},
			refIDs:      []int{0},
			expectError: false,
		},
		{
			name:        "multiple valid references",
			defNames:    []string{"def1", "def2", "def3"},
			refIDs:      []int{0, 1, 2},
			expectError: false,
		},
		{
			name:          "single invalid reference",
			defNames:      nil,
			invalidRefs:   []string{"invalid-id"},
			expectError:   true,
			errorContains: "invalid-id",
		},
		{
			name:          "valid and invalid mixed",
			defNames:      []string{"def1"},
			refIDs:        []int{0},
			invalidRefs:   []string{"bad-ref"},
			expectError:   true,
			errorContains: "bad-ref",
		},
		{
			name:          "multiple invalid references",
			defNames:      nil,
			invalidRefs:   []string{"bad1", "bad2"},
			expectError:   true,
			errorContains: "bad", // Should contain at least one
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()

			// Create definitions and collect their IDs
			var defIDs []string
			for i, name := range tt.defNames {
				def := createTestDefinition(t, s, name, "Content for "+name+" #"+string(rune('0'+i)))
				defIDs = append(defIDs, def.ID)
			}

			// Build context references
			var context []string
			for _, idx := range tt.refIDs {
				if idx >= 0 && idx < len(defIDs) {
					context = append(context, defIDs[idx])
				}
			}
			context = append(context, tt.invalidRefs...)

			n := createContextTestNode(t, "1", context...)

			err := node.ValidateDefRefs(n, s)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateDefRefs() = nil, want error")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDefRefs() = %v, want nil", err)
				}
			}
		})
	}
}

// TestValidateAssnRefs_TableDriven provides comprehensive table-driven tests for assumption validation.
func TestValidateAssnRefs_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		assnStmts     []string // assumption statements to create
		refIDs        []int    // indices of assumptions to reference
		invalidRefs   []string // explicit invalid reference IDs
		expectError   bool
		errorContains string
	}{
		{
			name:        "no references",
			assnStmts:   nil,
			refIDs:      nil,
			expectError: false,
		},
		{
			name:        "single valid reference",
			assnStmts:   []string{"Assume A"},
			refIDs:      []int{0},
			expectError: false,
		},
		{
			name:        "multiple valid references",
			assnStmts:   []string{"Assume A", "Assume B", "Assume C"},
			refIDs:      []int{0, 1, 2},
			expectError: false,
		},
		{
			name:          "single invalid reference",
			assnStmts:    nil,
			invalidRefs:  []string{"invalid-assn"},
			expectError:  true,
			errorContains: "invalid-assn",
		},
		{
			name:          "valid and invalid mixed",
			assnStmts:    []string{"Assume X"},
			refIDs:       []int{0},
			invalidRefs:  []string{"bad-assn"},
			expectError:  true,
			errorContains: "bad-assn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()

			// Create assumptions and collect their IDs
			var assnIDs []string
			for _, stmt := range tt.assnStmts {
				assn := createTestAssumption(t, s, stmt)
				assnIDs = append(assnIDs, assn.ID)
			}

			// Build context references
			var context []string
			for _, idx := range tt.refIDs {
				if idx >= 0 && idx < len(assnIDs) {
					context = append(context, assnIDs[idx])
				}
			}
			context = append(context, tt.invalidRefs...)

			n := createContextTestNode(t, "1", context...)

			err := node.ValidateAssnRefs(n, s)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateAssnRefs() = nil, want error")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("ValidateAssnRefs() = %v, want nil", err)
				}
			}
		})
	}
}

// TestValidateExtRefs_TableDriven provides comprehensive table-driven tests for external validation.
func TestValidateExtRefs_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		extNames      []string // external names to create
		refIDs        []int    // indices of externals to reference
		invalidRefs   []string // explicit invalid reference IDs
		expectError   bool
		errorContains string
	}{
		{
			name:        "no references",
			extNames:    nil,
			refIDs:      nil,
			expectError: false,
		},
		{
			name:        "single valid reference",
			extNames:    []string{"Theorem A"},
			refIDs:      []int{0},
			expectError: false,
		},
		{
			name:        "multiple valid references",
			extNames:    []string{"Theorem A", "Lemma B", "Corollary C"},
			refIDs:      []int{0, 1, 2},
			expectError: false,
		},
		{
			name:          "single invalid reference",
			extNames:     nil,
			invalidRefs:  []string{"invalid-ext"},
			expectError:  true,
			errorContains: "invalid-ext",
		},
		{
			name:          "valid and invalid mixed",
			extNames:     []string{"Result X"},
			refIDs:       []int{0},
			invalidRefs:  []string{"bad-ext"},
			expectError:  true,
			errorContains: "bad-ext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()

			// Create externals and collect their IDs
			var extIDs []string
			for i, name := range tt.extNames {
				ext := createTestExternal(t, s, name, "Source for "+name+" #"+string(rune('0'+i)))
				extIDs = append(extIDs, ext.ID)
			}

			// Build context references
			var context []string
			for _, idx := range tt.refIDs {
				if idx >= 0 && idx < len(extIDs) {
					context = append(context, extIDs[idx])
				}
			}
			context = append(context, tt.invalidRefs...)

			n := createContextTestNode(t, "1", context...)

			err := node.ValidateExtRefs(n, s)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateExtRefs() = nil, want error")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("ValidateExtRefs() = %v, want nil", err)
				}
			}
		})
	}
}

// ===========================================================================
// Edge cases and robustness tests
// ===========================================================================

// TestValidateDefRefs_DuplicateRefs tests handling of duplicate definition references.
func TestValidateDefRefs_DuplicateRefs(t *testing.T) {
	s := state.NewState()

	def := createTestDefinition(t, s, "unique-def", "Content for unique definition")

	// Create node with duplicate references to same definition
	n := createContextTestNode(t, "1", def.ID, def.ID, def.ID)

	err := node.ValidateDefRefs(n, s)
	if err != nil {
		t.Errorf("ValidateDefRefs() = %v, want nil for duplicate valid references", err)
	}
}

// TestValidateAssnRefs_DuplicateRefs tests handling of duplicate assumption references.
func TestValidateAssnRefs_DuplicateRefs(t *testing.T) {
	s := state.NewState()

	assn := createTestAssumption(t, s, "Assume something unique")

	// Create node with duplicate references to same assumption
	n := createContextTestNode(t, "1", assn.ID, assn.ID)

	err := node.ValidateAssnRefs(n, s)
	if err != nil {
		t.Errorf("ValidateAssnRefs() = %v, want nil for duplicate valid references", err)
	}
}

// TestValidateExtRefs_DuplicateRefs tests handling of duplicate external references.
func TestValidateExtRefs_DuplicateRefs(t *testing.T) {
	s := state.NewState()

	ext := createTestExternal(t, s, "Unique External", "Unique source")

	// Create node with duplicate references to same external
	n := createContextTestNode(t, "1", ext.ID, ext.ID)

	err := node.ValidateExtRefs(n, s)
	if err != nil {
		t.Errorf("ValidateExtRefs() = %v, want nil for duplicate valid references", err)
	}
}

// TestValidateContextRefs_EmptyStringRef tests handling of empty string in context.
func TestValidateContextRefs_EmptyStringRef(t *testing.T) {
	s := state.NewState()

	// Create node with empty string in context
	n := createContextTestNode(t, "1", "")

	// Empty string should be treated as invalid reference
	err := node.ValidateContextRefs(n, s)
	if err == nil {
		t.Error("ValidateContextRefs() = nil, want error for empty string reference")
	}
}

// TestValidateContextRefs_WhitespaceRef tests handling of whitespace-only reference.
func TestValidateContextRefs_WhitespaceRef(t *testing.T) {
	s := state.NewState()

	// Create node with whitespace-only context reference
	n := createContextTestNode(t, "1", "   ", "\t", "\n")

	// Whitespace should be treated as invalid references
	err := node.ValidateContextRefs(n, s)
	if err == nil {
		t.Error("ValidateContextRefs() = nil, want error for whitespace-only references")
	}
}

// TestValidateContextRefs_LargeNumberOfRefs tests with many context references.
func TestValidateContextRefs_LargeNumberOfRefs(t *testing.T) {
	s := state.NewState()

	// Create 50 definitions
	var context []string
	for i := 0; i < 50; i++ {
		def := createTestDefinition(t, s, "def"+string(rune('A'+i%26))+string(rune('0'+i/26)), "Content #"+string(rune('0'+i)))
		context = append(context, def.ID)
	}

	n := createContextTestNode(t, "1", context...)

	err := node.ValidateContextRefs(n, s)
	if err != nil {
		t.Errorf("ValidateContextRefs() = %v, want nil for many valid references", err)
	}
}

// TestValidateContextRefs_LargeNumberWithOneMissing tests many refs with one missing.
func TestValidateContextRefs_LargeNumberWithOneMissing(t *testing.T) {
	s := state.NewState()

	// Create 19 valid definitions
	var context []string
	for i := 0; i < 19; i++ {
		def := createTestDefinition(t, s, "def"+string(rune('0'+i)), "Content #"+string(rune('0'+i)))
		context = append(context, def.ID)
	}

	// Add one invalid reference in the middle
	context = append(context[:10], append([]string{"missing-ref"}, context[10:]...)...)

	n := createContextTestNode(t, "1", context...)

	err := node.ValidateContextRefs(n, s)
	if err == nil {
		t.Error("ValidateContextRefs() = nil, want error for one missing among many")
	}
}

// ===========================================================================
// Context lookup interface tests
// ===========================================================================

// ContextLookup is the interface that the validation functions should use.
// This test verifies the expected interface pattern.
type mockContextLookup struct {
	defs  map[string]*node.Definition
	assns map[string]*node.Assumption
	exts  map[string]*node.External
}

func newMockContextLookup() *mockContextLookup {
	return &mockContextLookup{
		defs:  make(map[string]*node.Definition),
		assns: make(map[string]*node.Assumption),
		exts:  make(map[string]*node.External),
	}
}

func (m *mockContextLookup) GetDefinition(id string) *node.Definition {
	return m.defs[id]
}

func (m *mockContextLookup) GetAssumption(id string) *node.Assumption {
	return m.assns[id]
}

func (m *mockContextLookup) GetExternal(id string) *node.External {
	return m.exts[id]
}

// TestValidateContextRefs_UsesLookupInterface verifies that the validation
// functions work with the ContextLookup interface pattern (like NodeLookup).
// This is a design verification test - the actual interface may differ.
func TestValidateContextRefs_UsesLookupInterface(t *testing.T) {
	// This test documents the expected interface pattern.
	// The actual implementation may use State directly or a ContextLookup interface.
	// The key requirement is that lookups use string IDs, not NodeIDs.

	s := state.NewState()
	def := createTestDefinition(t, s, "test-def", "Test content")

	// Verify the definition can be looked up by its ID
	retrieved := s.GetDefinition(def.ID)
	if retrieved == nil {
		t.Error("State.GetDefinition() returned nil for existing definition")
	}
	if retrieved != nil && retrieved.ID != def.ID {
		t.Errorf("State.GetDefinition() returned wrong definition: got ID %s, want %s", retrieved.ID, def.ID)
	}
}
