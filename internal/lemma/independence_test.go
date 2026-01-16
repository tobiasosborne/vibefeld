// Package lemma provides validation for lemma and citation references.
package lemma

import (
	"testing"

	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

func TestValidateIndependence(t *testing.T) {
	tests := []struct {
		name             string
		setupState       func() (*state.State, types.NodeID)
		expectIndependent bool
		expectViolations  int
		expectLocalDeps   int
		expectTaintedDeps int
		expectErr         bool
		expectErrCode     errors.ErrorCode
	}{
		{
			name: "valid independent node - all criteria met",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")
				childID, _ := types.Parse("1.1")

				// Create validated root node
				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicValidated
				rootNode.TaintState = node.TaintClean
				st.AddNode(rootNode)

				// Create validated child node (target for lemma extraction)
				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
				childNode.EpistemicState = schema.EpistemicValidated
				childNode.TaintState = node.TaintClean
				childNode.Scope = nil // No local scope
				st.AddNode(childNode)

				return st, childID
			},
			expectIndependent: true,
			expectViolations:  0,
			expectLocalDeps:   0,
			expectTaintedDeps: 0,
			expectErr:         false,
		},
		{
			name: "node not validated",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")

				// Create pending root node
				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicPending
				rootNode.TaintState = node.TaintClean
				st.AddNode(rootNode)

				return st, rootID
			},
			expectIndependent: false,
			expectViolations:  1,
			expectLocalDeps:   0,
			expectTaintedDeps: 0,
			expectErr:         false,
		},
		{
			name: "node with local assumption in scope",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")
				assumeID, _ := types.Parse("1.1")
				childID, _ := types.Parse("1.1.1")

				// Create validated root node
				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicValidated
				rootNode.TaintState = node.TaintClean
				st.AddNode(rootNode)

				// Create local_assume node
				assumeNode, _ := node.NewNode(assumeID, schema.NodeTypeLocalAssume, "Assume P", schema.InferenceAssumption)
				assumeNode.EpistemicState = schema.EpistemicValidated
				assumeNode.TaintState = node.TaintClean
				st.AddNode(assumeNode)
				_ = st.OpenScope(assumeID, "Assume P")

				// Create child with local scope
				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child using P", schema.InferenceModusPonens)
				childNode.EpistemicState = schema.EpistemicValidated
				childNode.TaintState = node.TaintClean
				childNode.Scope = []string{assumeID.String()}
				st.AddNode(childNode)

				return st, childID
			},
			expectIndependent: false,
			expectViolations:  1,
			expectLocalDeps:   1,
			expectTaintedDeps: 0,
			expectErr:         false,
		},
		{
			name: "node with pending ancestor",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")
				childID, _ := types.Parse("1.1")

				// Create pending root node
				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicPending
				rootNode.TaintState = node.TaintClean
				st.AddNode(rootNode)

				// Create validated child node
				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
				childNode.EpistemicState = schema.EpistemicValidated
				childNode.TaintState = node.TaintClean
				st.AddNode(childNode)

				return st, childID
			},
			expectIndependent: false,
			expectViolations:  1,
			expectLocalDeps:   0,
			expectTaintedDeps: 1,
			expectErr:         false,
		},
		{
			name: "node with tainted state",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")

				// Create validated but tainted root node
				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicValidated
				rootNode.TaintState = node.TaintTainted
				st.AddNode(rootNode)

				return st, rootID
			},
			expectIndependent: false,
			expectViolations:  1,
			expectLocalDeps:   0,
			expectTaintedDeps: 0,
			expectErr:         false,
		},
		{
			name: "node with self_admitted taint",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")

				// Create validated but self_admitted root node
				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicValidated
				rootNode.TaintState = node.TaintSelfAdmitted
				st.AddNode(rootNode)

				return st, rootID
			},
			expectIndependent: false,
			expectViolations:  1,
			expectLocalDeps:   0,
			expectTaintedDeps: 0,
			expectErr:         false,
		},
		{
			name: "multiple violations",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")
				assumeID, _ := types.Parse("1.1")
				childID, _ := types.Parse("1.1.1")

				// Create pending root node
				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicPending
				rootNode.TaintState = node.TaintClean
				st.AddNode(rootNode)

				// Create local_assume node
				assumeNode, _ := node.NewNode(assumeID, schema.NodeTypeLocalAssume, "Assume P", schema.InferenceAssumption)
				assumeNode.EpistemicState = schema.EpistemicPending
				assumeNode.TaintState = node.TaintClean
				st.AddNode(assumeNode)
				_ = st.OpenScope(assumeID, "Assume P")

				// Create child with multiple violations: pending, local scope, tainted
				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
				childNode.EpistemicState = schema.EpistemicPending
				childNode.TaintState = node.TaintTainted
				childNode.Scope = []string{assumeID.String()}
				st.AddNode(childNode)

				return st, childID
			},
			expectIndependent: false,
			expectViolations:  4, // not validated, local scope, pending ancestors (root + assume), tainted
			expectLocalDeps:   1,
			expectTaintedDeps: 2,
			expectErr:         false,
		},
		{
			name: "root node with no ancestors",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")

				// Create validated root node
				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicValidated
				rootNode.TaintState = node.TaintClean
				st.AddNode(rootNode)

				return st, rootID
			},
			expectIndependent: true,
			expectViolations:  0,
			expectLocalDeps:   0,
			expectTaintedDeps: 0,
			expectErr:         false,
		},
		{
			name: "node not found",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				nonExistentID, _ := types.Parse("1.1.1")
				return st, nonExistentID
			},
			expectIndependent: false,
			expectViolations:  0,
			expectLocalDeps:   0,
			expectTaintedDeps: 0,
			expectErr:         true,
			expectErrCode:     errors.EXTRACTION_INVALID,
		},
		{
			name: "nil state",
			setupState: func() (*state.State, types.NodeID) {
				id, _ := types.Parse("1")
				return nil, id
			},
			expectIndependent: false,
			expectViolations:  0,
			expectLocalDeps:   0,
			expectTaintedDeps: 0,
			expectErr:         true,
			expectErrCode:     errors.EXTRACTION_INVALID,
		},
		{
			name: "deep ancestry chain - all validated",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				ids := []string{"1", "1.1", "1.1.1", "1.1.1.1", "1.1.1.1.1"}

				for _, idStr := range ids {
					id, _ := types.Parse(idStr)
					n, _ := node.NewNode(id, schema.NodeTypeClaim, "Claim "+idStr, schema.InferenceModusPonens)
					n.EpistemicState = schema.EpistemicValidated
					n.TaintState = node.TaintClean
					st.AddNode(n)
				}

				deepID, _ := types.Parse("1.1.1.1.1")
				return st, deepID
			},
			expectIndependent: true,
			expectViolations:  0,
			expectLocalDeps:   0,
			expectTaintedDeps: 0,
			expectErr:         false,
		},
		{
			name: "admitted ancestor is valid for lemma extraction",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")
				childID, _ := types.Parse("1.1")

				// Create admitted root node (admitted is acceptable for lemma extraction)
				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicAdmitted
				rootNode.TaintState = node.TaintClean
				st.AddNode(rootNode)

				// Create validated child node
				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
				childNode.EpistemicState = schema.EpistemicValidated
				childNode.TaintState = node.TaintClean
				st.AddNode(childNode)

				return st, childID
			},
			expectIndependent: true,
			expectViolations:  0,
			expectLocalDeps:   0,
			expectTaintedDeps: 0,
			expectErr:         false,
		},
		{
			name: "refuted ancestor is invalid",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")
				childID, _ := types.Parse("1.1")

				// Create refuted root node
				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicRefuted
				rootNode.TaintState = node.TaintClean
				st.AddNode(rootNode)

				// Create validated child node
				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
				childNode.EpistemicState = schema.EpistemicValidated
				childNode.TaintState = node.TaintClean
				st.AddNode(childNode)

				return st, childID
			},
			expectIndependent: false,
			expectViolations:  1,
			expectLocalDeps:   0,
			expectTaintedDeps: 1,
			expectErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, nodeID := tt.setupState()
			result, err := ValidateIndependence(nodeID, st)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.expectErrCode != 0 {
					code := errors.Code(err)
					if code != tt.expectErrCode {
						t.Errorf("expected error code %v, got %v", tt.expectErrCode, code)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected result, got nil")
				return
			}

			if result.IsIndependent != tt.expectIndependent {
				t.Errorf("IsIndependent = %v, want %v", result.IsIndependent, tt.expectIndependent)
			}

			if len(result.Violations) != tt.expectViolations {
				t.Errorf("Violations count = %d, want %d; violations: %v", len(result.Violations), tt.expectViolations, result.Violations)
			}

			if len(result.DependsOnLocal) != tt.expectLocalDeps {
				t.Errorf("DependsOnLocal count = %d, want %d", len(result.DependsOnLocal), tt.expectLocalDeps)
			}

			if len(result.DependsOnTainted) != tt.expectTaintedDeps {
				t.Errorf("DependsOnTainted count = %d, want %d", len(result.DependsOnTainted), tt.expectTaintedDeps)
			}
		})
	}
}

func TestCheckLocalDependencies(t *testing.T) {
	tests := []struct {
		name       string
		setupState func() (*state.State, types.NodeID)
		expectDeps int
	}{
		{
			name: "no local dependencies",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")

				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.Scope = nil
				st.AddNode(rootNode)

				return st, rootID
			},
			expectDeps: 0,
		},
		{
			name: "single local dependency",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")
				assumeID, _ := types.Parse("1.1")
				childID, _ := types.Parse("1.1.1")

				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				st.AddNode(rootNode)

				assumeNode, _ := node.NewNode(assumeID, schema.NodeTypeLocalAssume, "Assume P", schema.InferenceAssumption)
				st.AddNode(assumeNode)

				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
				childNode.Scope = []string{assumeID.String()}
				st.AddNode(childNode)

				return st, childID
			},
			expectDeps: 1,
		},
		{
			name: "multiple local dependencies",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")
				assume1ID, _ := types.Parse("1.1")
				assume2ID, _ := types.Parse("1.1.1")
				childID, _ := types.Parse("1.1.1.1")

				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				st.AddNode(rootNode)

				assume1Node, _ := node.NewNode(assume1ID, schema.NodeTypeLocalAssume, "Assume P", schema.InferenceAssumption)
				st.AddNode(assume1Node)

				assume2Node, _ := node.NewNode(assume2ID, schema.NodeTypeLocalAssume, "Assume Q", schema.InferenceAssumption)
				st.AddNode(assume2Node)

				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
				childNode.Scope = []string{assume1ID.String(), assume2ID.String()}
				st.AddNode(childNode)

				return st, childID
			},
			expectDeps: 2,
		},
		{
			name: "nil state returns empty",
			setupState: func() (*state.State, types.NodeID) {
				id, _ := types.Parse("1")
				return nil, id
			},
			expectDeps: 0,
		},
		{
			name: "node not found returns empty",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				nonExistentID, _ := types.Parse("1.1.1")
				return st, nonExistentID
			},
			expectDeps: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, nodeID := tt.setupState()
			deps := CheckLocalDependencies(nodeID, st)

			if len(deps) != tt.expectDeps {
				t.Errorf("CheckLocalDependencies returned %d deps, want %d", len(deps), tt.expectDeps)
			}
		})
	}
}

func TestCheckAncestorValidity(t *testing.T) {
	tests := []struct {
		name        string
		setupState  func() (*state.State, types.NodeID)
		expectCount int
	}{
		{
			name: "root node - no ancestors",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")

				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicValidated
				st.AddNode(rootNode)

				return st, rootID
			},
			expectCount: 0,
		},
		{
			name: "all ancestors validated",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")
				childID, _ := types.Parse("1.1")

				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicValidated
				st.AddNode(rootNode)

				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
				childNode.EpistemicState = schema.EpistemicValidated
				st.AddNode(childNode)

				return st, childID
			},
			expectCount: 0,
		},
		{
			name: "one pending ancestor",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")
				childID, _ := types.Parse("1.1")

				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicPending
				st.AddNode(rootNode)

				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
				childNode.EpistemicState = schema.EpistemicValidated
				st.AddNode(childNode)

				return st, childID
			},
			expectCount: 1,
		},
		{
			name: "deep chain with multiple invalid ancestors",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				ids := []string{"1", "1.1", "1.1.1", "1.1.1.1"}
				states := []schema.EpistemicState{
					schema.EpistemicPending,   // invalid
					schema.EpistemicValidated, // valid
					schema.EpistemicRefuted,   // invalid
					schema.EpistemicValidated, // target node
				}

				for i, idStr := range ids {
					id, _ := types.Parse(idStr)
					n, _ := node.NewNode(id, schema.NodeTypeClaim, "Claim "+idStr, schema.InferenceModusPonens)
					n.EpistemicState = states[i]
					st.AddNode(n)
				}

				deepID, _ := types.Parse("1.1.1.1")
				return st, deepID
			},
			expectCount: 2, // root (pending) and 1.1.1 (refuted)
		},
		{
			name: "admitted ancestor is valid",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				rootID, _ := types.Parse("1")
				childID, _ := types.Parse("1.1")

				rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
				rootNode.EpistemicState = schema.EpistemicAdmitted
				st.AddNode(rootNode)

				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
				childNode.EpistemicState = schema.EpistemicValidated
				st.AddNode(childNode)

				return st, childID
			},
			expectCount: 0, // admitted is acceptable
		},
		{
			name: "nil state returns empty",
			setupState: func() (*state.State, types.NodeID) {
				id, _ := types.Parse("1.1")
				return nil, id
			},
			expectCount: 0,
		},
		{
			name: "node not found returns empty",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				nonExistentID, _ := types.Parse("1.1.1")
				return st, nonExistentID
			},
			expectCount: 0,
		},
		{
			name: "missing ancestor in state",
			setupState: func() (*state.State, types.NodeID) {
				st := state.NewState()
				// Only add the child, not the root
				childID, _ := types.Parse("1.1")
				childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
				childNode.EpistemicState = schema.EpistemicValidated
				st.AddNode(childNode)

				return st, childID
			},
			expectCount: 1, // missing ancestor counts as invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, nodeID := tt.setupState()
			invalid := CheckAncestorValidity(nodeID, st)

			if len(invalid) != tt.expectCount {
				t.Errorf("CheckAncestorValidity returned %d invalid ancestors, want %d; got: %v", len(invalid), tt.expectCount, invalid)
			}
		})
	}
}

func TestIndependenceResult_ViolationsContent(t *testing.T) {
	// Test that violation messages are descriptive
	st := state.NewState()
	rootID, _ := types.Parse("1")

	rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
	rootNode.EpistemicState = schema.EpistemicPending
	rootNode.TaintState = node.TaintTainted
	st.AddNode(rootNode)

	result, err := ValidateIndependence(rootID, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsIndependent {
		t.Error("expected node to be non-independent")
	}

	// Check that we have violations with meaningful content
	if len(result.Violations) == 0 {
		t.Error("expected at least one violation")
	}

	// Check that violation messages mention the specific issues
	foundValidated := false
	foundTainted := false
	for _, v := range result.Violations {
		if contains(v, "validated") || contains(v, "pending") {
			foundValidated = true
		}
		if contains(v, "taint") {
			foundTainted = true
		}
	}

	if !foundValidated {
		t.Error("expected violation message to mention validation state")
	}
	if !foundTainted {
		t.Error("expected violation message to mention taint state")
	}
}
