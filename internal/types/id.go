package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// NodeID represents a hierarchical proof node identifier.
// Format: "1" (root), "1.1" (first child), "1.2.3" (grandchild), etc.
type NodeID struct {
	parts []int
}

// Parse parses a string representation of a NodeID.
// Valid formats: "1", "1.1", "1.2.3", etc.
// Returns error for invalid formats.
func Parse(s string) (NodeID, error) {
	// Reject empty string
	if s == "" {
		return NodeID{}, fmt.Errorf("invalid node ID: empty string")
	}

	// Reject strings with only whitespace
	if strings.TrimSpace(s) != s || strings.TrimSpace(s) == "" {
		return NodeID{}, fmt.Errorf("invalid node ID: whitespace")
	}

	// Split by dots
	parts := strings.Split(s, ".")

	// Check for empty parts (e.g., ".1", "1.", "1..1")
	for _, part := range parts {
		if part == "" {
			return NodeID{}, fmt.Errorf("invalid node ID: empty part")
		}
	}

	// Parse each part
	intParts := make([]int, len(parts))
	for i, part := range parts {
		// Check for non-numeric characters
		num, err := strconv.Atoi(part)
		if err != nil {
			return NodeID{}, fmt.Errorf("invalid node ID: non-numeric part %q", part)
		}

		// Check for zero or negative numbers
		if num <= 0 {
			return NodeID{}, fmt.Errorf("invalid node ID: part must be positive, got %d", num)
		}

		intParts[i] = num
	}

	// Root must be "1"
	if intParts[0] != 1 {
		return NodeID{}, fmt.Errorf("invalid node ID: root must be 1, got %d", intParts[0])
	}

	return NodeID{parts: intParts}, nil
}

// String returns the string representation of the NodeID.
func (n NodeID) String() string {
	if len(n.parts) == 0 {
		return ""
	}

	strParts := make([]string, len(n.parts))
	for i, part := range n.parts {
		strParts[i] = strconv.Itoa(part)
	}
	return strings.Join(strParts, ".")
}

// Parent returns the parent NodeID and true, or false if this is the root node.
func (n NodeID) Parent() (NodeID, bool) {
	if len(n.parts) <= 1 {
		return NodeID{}, false
	}

	return NodeID{parts: n.parts[:len(n.parts)-1]}, true
}

// Child returns the nth child of this NodeID.
// n must be positive (1, 2, 3, ...).
// Returns an error if num is not positive.
func (n NodeID) Child(num int) (NodeID, error) {
	if num <= 0 {
		return NodeID{}, fmt.Errorf("child number must be positive, got %d", num)
	}

	newParts := make([]int, len(n.parts)+1)
	copy(newParts, n.parts)
	newParts[len(n.parts)] = num
	return NodeID{parts: newParts}, nil
}

// IsRoot returns true if this is the root node ("1").
func (n NodeID) IsRoot() bool {
	return len(n.parts) == 1 && n.parts[0] == 1
}

// Depth returns the depth of this node in the tree.
// Root has depth 1, its children have depth 2, etc.
func (n NodeID) Depth() int {
	return len(n.parts)
}

// IsAncestorOf returns true if this NodeID is an ancestor of other.
// A node is not considered an ancestor of itself.
func (n NodeID) IsAncestorOf(other NodeID) bool {
	// Must be strictly shorter (not equal length)
	if len(n.parts) >= len(other.parts) {
		return false
	}

	// Check if all parts match
	for i := 0; i < len(n.parts); i++ {
		if n.parts[i] != other.parts[i] {
			return false
		}
	}

	return true
}

// CommonAncestor returns the lowest common ancestor of this NodeID and other.
// If one node is an ancestor of the other, returns that ancestor.
// If the nodes are the same, returns that node.
func (n NodeID) CommonAncestor(other NodeID) NodeID {
	// Find the length of common prefix
	minLen := len(n.parts)
	if len(other.parts) < minLen {
		minLen = len(other.parts)
	}

	commonLen := 0
	for i := 0; i < minLen; i++ {
		if n.parts[i] == other.parts[i] {
			commonLen++
		} else {
			break
		}
	}

	// If no common parts, this shouldn't happen (all IDs start with 1)
	// but return empty NodeID as safety
	if commonLen == 0 {
		return NodeID{}
	}

	return NodeID{parts: n.parts[:commonLen]}
}

// MarshalJSON implements json.Marshaler.
// NodeIDs are serialized as their string representation.
func (n NodeID) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.String())
}

// UnmarshalJSON implements json.Unmarshaler.
// Expects a NodeID string (e.g., "1", "1.2.3").
func (n *NodeID) UnmarshalJSON(data []byte) error {
	// Handle null case
	if string(data) == "null" {
		return nil
	}

	// Handle empty string case
	if string(data) == `""` {
		*n = NodeID{}
		return nil
	}

	// Strip quotes from JSON string
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid JSON NodeID: not a string")
	}
	s := string(data[1 : len(data)-1])

	// Parse the NodeID
	parsed, err := Parse(s)
	if err != nil {
		return err
	}

	*n = parsed
	return nil
}
