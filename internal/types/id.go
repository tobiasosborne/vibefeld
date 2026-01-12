package types

import "errors"

// NodeID represents a hierarchical proof node identifier.
// Format: "1" (root), "1.1" (first child), "1.2.3" (grandchild), etc.
type NodeID struct {
	parts []int
}

// Parse parses a string representation of a NodeID.
// Valid formats: "1", "1.1", "1.2.3", etc.
// Returns error for invalid formats.
func Parse(s string) (NodeID, error) {
	return NodeID{}, errors.New("not implemented")
}

// String returns the string representation of the NodeID.
func (n NodeID) String() string {
	return ""
}

// Parent returns the parent NodeID and true, or false if this is the root node.
func (n NodeID) Parent() (NodeID, bool) {
	return NodeID{}, false
}

// Child returns the nth child of this NodeID.
// n must be positive (1, 2, 3, ...).
func (n NodeID) Child(num int) NodeID {
	return NodeID{}
}

// IsRoot returns true if this is the root node ("1").
func (n NodeID) IsRoot() bool {
	return false
}

// Depth returns the depth of this node in the tree.
// Root has depth 1, its children have depth 2, etc.
func (n NodeID) Depth() int {
	return 0
}

// IsAncestorOf returns true if this NodeID is an ancestor of other.
// A node is not considered an ancestor of itself.
func (n NodeID) IsAncestorOf(other NodeID) bool {
	return false
}

// CommonAncestor returns the lowest common ancestor of this NodeID and other.
// If one node is an ancestor of the other, returns that ancestor.
// If the nodes are the same, returns that node.
func (n NodeID) CommonAncestor(other NodeID) NodeID {
	return NodeID{}
}
