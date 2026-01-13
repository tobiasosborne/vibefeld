// Package node provides data structures for proof nodes in the AF system.
package node

// ContextLookup is an interface for looking up context items by ID.
// This avoids import cycles by not depending on the state package directly.
// The state.State type implements this interface.
type ContextLookup interface {
	GetDefinition(id string) *Definition
	GetAssumption(id string) *Assumption
	GetExternal(id string) *External
}

// ValidateDefRefs checks that all definition references in a node exist in state.
// Returns nil if all references are valid, or NOT_FOUND error for missing definitions.
//
// This is a stub for TDD - implementation pending.
func ValidateDefRefs(n *Node, s ContextLookup) error {
	// TODO: Implement validation
	return nil
}

// ValidateAssnRefs checks that all assumption references in a node exist in state.
// Returns nil if all references are valid, or NOT_FOUND error for missing assumptions.
//
// This is a stub for TDD - implementation pending.
func ValidateAssnRefs(n *Node, s ContextLookup) error {
	// TODO: Implement validation
	return nil
}

// ValidateExtRefs checks that all external references in a node exist in state.
// Returns nil if all references are valid, or NOT_FOUND error for missing externals.
//
// This is a stub for TDD - implementation pending.
func ValidateExtRefs(n *Node, s ContextLookup) error {
	// TODO: Implement validation
	return nil
}
