// Package lemma provides validation for lemma and citation references.
package lemma

import (
	"fmt"

	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// IndependenceResult represents the result of independence validation.
// It contains information about whether a node can be extracted as a lemma
// and any violations that prevent extraction.
type IndependenceResult struct {
	// IsIndependent indicates whether the node can be extracted as a lemma.
	IsIndependent bool

	// Violations contains a list of independence violations found.
	// Each entry describes a specific reason why the node cannot be extracted.
	Violations []string

	// DependsOnLocal contains Node IDs of local assumptions this node depends on.
	// A node depending on local assumptions cannot be extracted as a lemma
	// because it relies on context-specific hypotheses.
	DependsOnLocal []string

	// DependsOnTainted contains Node IDs of ancestors that are not validated/admitted.
	// These are ancestors with epistemic states other than validated or admitted.
	DependsOnTainted []string
}

// ValidateIndependence checks if a node can be extracted as a lemma.
// A node is independent if:
// 1. It is validated (EpistemicState == validated)
// 2. It has no local assumptions in its scope
// 3. All its ancestors are validated or admitted
// 4. It is not tainted
//
// Returns an IndependenceResult containing the validation outcome and any violations.
// Returns an error if the state is nil or the node is not found.
func ValidateIndependence(nodeID types.NodeID, st *state.State) (*IndependenceResult, error) {
	// Validate inputs
	if st == nil {
		return nil, errors.Newf(errors.EXTRACTION_INVALID, "cannot validate independence: state is nil")
	}

	n := st.GetNode(nodeID)
	if n == nil {
		return nil, errors.Newf(errors.EXTRACTION_INVALID, "node %q not found", nodeID.String())
	}

	result := &IndependenceResult{
		IsIndependent:    true,
		Violations:       make([]string, 0),
		DependsOnLocal:   make([]string, 0),
		DependsOnTainted: make([]string, 0),
	}

	// Check 1: Node must be validated
	if n.EpistemicState != schema.EpistemicValidated {
		result.IsIndependent = false
		result.Violations = append(result.Violations,
			fmt.Sprintf("node %q is not validated (current state: %s)", nodeID.String(), n.EpistemicState))
	}

	// Check 2: No local assumptions in scope
	localDeps := CheckLocalDependencies(nodeID, st)
	if len(localDeps) > 0 {
		result.IsIndependent = false
		for _, dep := range localDeps {
			result.DependsOnLocal = append(result.DependsOnLocal, dep.String())
		}
		result.Violations = append(result.Violations,
			fmt.Sprintf("node %q depends on %d local assumption(s)", nodeID.String(), len(localDeps)))
	}

	// Check 3: All ancestors must be validated or admitted
	invalidAncestors := CheckAncestorValidity(nodeID, st)
	if len(invalidAncestors) > 0 {
		result.IsIndependent = false
		for _, ancestor := range invalidAncestors {
			result.DependsOnTainted = append(result.DependsOnTainted, ancestor.String())
		}
		result.Violations = append(result.Violations,
			fmt.Sprintf("node %q has %d ancestor(s) that are not validated or admitted", nodeID.String(), len(invalidAncestors)))
	}

	// Check 4: Node must not be tainted
	if n.TaintState != node.TaintClean {
		result.IsIndependent = false
		result.Violations = append(result.Violations,
			fmt.Sprintf("node %q has taint state %q (must be clean)", nodeID.String(), n.TaintState))
	}

	return result, nil
}

// CheckLocalDependencies returns any local assumptions the node depends on.
// A node depends on a local assumption if it appears in the node's Scope field.
// Returns an empty slice if the node has no local dependencies, state is nil,
// or the node is not found.
func CheckLocalDependencies(nodeID types.NodeID, st *state.State) []types.NodeID {
	if st == nil {
		return nil
	}

	n := st.GetNode(nodeID)
	if n == nil {
		return nil
	}

	// If no scope entries, no local dependencies
	if len(n.Scope) == 0 {
		return nil
	}

	deps := make([]types.NodeID, 0, len(n.Scope))
	for _, scopeRef := range n.Scope {
		// Parse the scope reference as a NodeID
		scopeID, err := types.Parse(scopeRef)
		if err != nil {
			// Skip invalid scope references
			continue
		}
		deps = append(deps, scopeID)
	}

	return deps
}

// CheckAncestorValidity returns any ancestors that are not validated or admitted.
// An ancestor is valid for lemma extraction if its epistemic state is either
// validated or admitted. Any other state (pending, refuted, archived) is invalid.
// Returns an empty slice if all ancestors are valid, state is nil, or node is not found.
func CheckAncestorValidity(nodeID types.NodeID, st *state.State) []types.NodeID {
	if st == nil {
		return nil
	}

	n := st.GetNode(nodeID)
	if n == nil {
		return nil
	}

	var invalidAncestors []types.NodeID

	// Walk up the ancestor chain
	currentID := nodeID
	for {
		parentID, hasParent := currentID.Parent()
		if !hasParent {
			// Reached the root, stop
			break
		}

		ancestor := st.GetNode(parentID)
		if ancestor == nil {
			// Ancestor not found in state - treat as invalid
			invalidAncestors = append(invalidAncestors, parentID)
		} else if !isValidForLemmaExtraction(ancestor.EpistemicState) {
			invalidAncestors = append(invalidAncestors, parentID)
		}

		currentID = parentID
	}

	return invalidAncestors
}

// isValidForLemmaExtraction checks if an epistemic state is acceptable for lemma extraction.
// Validated and admitted states are acceptable; pending, refuted, and archived are not.
func isValidForLemmaExtraction(es schema.EpistemicState) bool {
	return es == schema.EpistemicValidated || es == schema.EpistemicAdmitted
}
