package schema

import (
	"fmt"
)

// NodeType represents the type of a proof node
type NodeType string

// Node type constants
const (
	NodeTypeClaim          NodeType = "claim"
	NodeTypeLocalAssume    NodeType = "local_assume"
	NodeTypeLocalDischarge NodeType = "local_discharge"
	NodeTypeCase           NodeType = "case"
	NodeTypeQED            NodeType = "qed"
)

// NodeTypeInfo contains metadata about a node type
type NodeTypeInfo struct {
	ID          NodeType
	Description string
	OpensScope  bool // true for local_assume
	ClosesScope bool // true for local_discharge
}

// nodeTypeRegistry is the source of truth for all valid node types
var nodeTypeRegistry = map[NodeType]NodeTypeInfo{
	NodeTypeClaim: {
		ID:          NodeTypeClaim,
		Description: "A mathematical assertion to be justified",
		OpensScope:  false,
		ClosesScope: false,
	},
	NodeTypeLocalAssume: {
		ID:          NodeTypeLocalAssume,
		Description: "Introduce a local hypothesis (opens scope)",
		OpensScope:  true,
		ClosesScope: false,
	},
	NodeTypeLocalDischarge: {
		ID:          NodeTypeLocalDischarge,
		Description: "Conclude from local hypothesis (closes scope)",
		OpensScope:  false,
		ClosesScope: true,
	},
	NodeTypeCase: {
		ID:          NodeTypeCase,
		Description: "One branch of a case split",
		OpensScope:  false,
		ClosesScope: false,
	},
	NodeTypeQED: {
		ID:          NodeTypeQED,
		Description: "Final step concluding the proof or subproof",
		OpensScope:  false,
		ClosesScope: false,
	},
}

// ValidateNodeType validates that a string is a valid node type
func ValidateNodeType(s string) error {
	_, ok := nodeTypeRegistry[NodeType(s)]
	if !ok {
		return fmt.Errorf("invalid node type: %q", s)
	}
	return nil
}

// GetNodeTypeInfo returns metadata for a node type
// Returns (info, true) if the node type exists, (zero, false) otherwise
func GetNodeTypeInfo(t NodeType) (NodeTypeInfo, bool) {
	info, ok := nodeTypeRegistry[t]
	return info, ok
}

// AllNodeTypes returns a list of all valid node types with their metadata
func AllNodeTypes() []NodeTypeInfo {
	// Return in consistent order: claim, local_assume, local_discharge, case, qed
	return []NodeTypeInfo{
		nodeTypeRegistry[NodeTypeClaim],
		nodeTypeRegistry[NodeTypeLocalAssume],
		nodeTypeRegistry[NodeTypeLocalDischarge],
		nodeTypeRegistry[NodeTypeCase],
		nodeTypeRegistry[NodeTypeQED],
	}
}

// OpensScope returns true if the node type opens a scope
func OpensScope(t NodeType) bool {
	info, ok := nodeTypeRegistry[t]
	if !ok {
		return false
	}
	return info.OpensScope
}

// ClosesScope returns true if the node type closes a scope
func ClosesScope(t NodeType) bool {
	info, ok := nodeTypeRegistry[t]
	if !ok {
		return false
	}
	return info.ClosesScope
}
