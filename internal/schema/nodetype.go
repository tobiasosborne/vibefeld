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

// ValidateNodeType validates that a string is a valid node type
func ValidateNodeType(s string) error {
	return fmt.Errorf("not implemented")
}

// GetNodeTypeInfo returns metadata for a node type
// Returns (info, true) if the node type exists, (zero, false) otherwise
func GetNodeTypeInfo(t NodeType) (NodeTypeInfo, bool) {
	return NodeTypeInfo{}, false
}

// AllNodeTypes returns a list of all valid node types with their metadata
func AllNodeTypes() []NodeTypeInfo {
	return nil
}

// OpensScope returns true if the node type opens a scope
func OpensScope(t NodeType) bool {
	return false
}

// ClosesScope returns true if the node type closes a scope
func ClosesScope(t NodeType) bool {
	return false
}
