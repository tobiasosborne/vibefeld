package scope

import (
	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

// ValidateScope checks if a node's scope references are valid.
// All entries referenced in the node's Scope must exist in activeEntries
// and must be active (not discharged).
//
// Returns:
//   - nil if all scope references are valid
//   - SCOPE_VIOLATION error if any reference is to a discharged or non-existent entry
func ValidateScope(n *node.Node, activeEntries []*Entry) error {
	if n == nil {
		return nil
	}

	// Empty scope is always valid
	if len(n.Scope) == 0 {
		return nil
	}

	// Build lookup map of active entries by NodeID string
	activeMap := make(map[string]*Entry)
	for _, entry := range activeEntries {
		if entry != nil && entry.IsActive() {
			activeMap[entry.NodeID.String()] = entry
		}
	}

	// Check each scope reference
	for _, scopeRef := range n.Scope {
		if _, exists := activeMap[scopeRef]; !exists {
			return errors.Newf(errors.SCOPE_VIOLATION,
				"scope reference %q is not an active entry", scopeRef)
		}
	}

	return nil
}

// ValidateScopeBalance checks that all local_assume nodes have matching
// local_discharge nodes. This ensures that all locally opened scopes
// are properly closed.
//
// Returns:
//   - nil if all scopes are balanced
//   - SCOPE_UNCLOSED error if any local_assume has no matching discharge
func ValidateScopeBalance(nodes []*node.Node) error {
	if nodes == nil {
		return nil
	}

	// Count open scopes (local_assume without matching discharge)
	openCount := 0

	for _, n := range nodes {
		if n == nil {
			continue
		}

		if n.Type == schema.NodeTypeLocalAssume {
			openCount++
		} else if n.Type == schema.NodeTypeLocalDischarge {
			if openCount > 0 {
				openCount--
			}
			// Note: Extra discharges without matching assumes are not
			// treated as SCOPE_UNCLOSED errors in this implementation
		}
	}

	if openCount > 0 {
		return errors.Newf(errors.SCOPE_UNCLOSED,
			"%d local_assume node(s) without matching local_discharge", openCount)
	}

	return nil
}
