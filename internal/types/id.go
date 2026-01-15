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
	parts  []int
	cached string // cached string representation, computed on first String() call
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

	return NodeID{parts: intParts, cached: s}, nil
}

// String returns the string representation of the NodeID.
// The result is cached on first computation for performance.
func (n NodeID) String() string {
	if len(n.parts) == 0 {
		return ""
	}

	// Return cached value if available
	if n.cached != "" {
		return n.cached
	}

	// Compute and return (can't cache with value receiver)
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

	// Derive parent cached string if available
	var parentCached string
	if n.cached != "" {
		// Find last dot and take prefix
		lastDot := strings.LastIndex(n.cached, ".")
		if lastDot > 0 {
			parentCached = n.cached[:lastDot]
		}
	}

	return NodeID{parts: n.parts[:len(n.parts)-1], cached: parentCached}, true
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

	// Compute cached string for new child
	var childCached string
	if n.cached != "" {
		childCached = n.cached + "." + strconv.Itoa(num)
	} else {
		// Fallback: compute full string
		strParts := make([]string, len(newParts))
		for i, part := range newParts {
			strParts[i] = strconv.Itoa(part)
		}
		childCached = strings.Join(strParts, ".")
	}

	return NodeID{parts: newParts, cached: childCached}, nil
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

	// If common ancestor is the same as n, reuse its cached string
	if commonLen == len(n.parts) && n.cached != "" {
		return NodeID{parts: n.parts[:commonLen], cached: n.cached}
	}

	// If common ancestor is the same as other, reuse its cached string
	if commonLen == len(other.parts) && other.cached != "" {
		return NodeID{parts: n.parts[:commonLen], cached: other.cached}
	}

	// Otherwise, derive from n.cached if available
	var ancestorCached string
	if n.cached != "" {
		// Find the position after commonLen-th dot (or end if commonLen equals n's depth)
		parts := strings.SplitN(n.cached, ".", commonLen+1)
		if len(parts) > commonLen {
			ancestorCached = strings.Join(parts[:commonLen], ".")
		}
	}

	return NodeID{parts: n.parts[:commonLen], cached: ancestorCached}
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

// Less returns true if this NodeID is lexicographically less than other.
// Comparison is performed directly on the internal integer parts without
// string parsing, making it efficient for sorting operations.
// Empty NodeIDs are considered less than non-empty ones.
// Examples: 1 < 1.1 < 1.2 < 1.10 < 2 < 2.1
func (n NodeID) Less(other NodeID) bool {
	// Handle empty NodeIDs
	if len(n.parts) == 0 {
		return len(other.parts) > 0
	}
	if len(other.parts) == 0 {
		return false
	}

	// Compare part by part
	minLen := len(n.parts)
	if len(other.parts) < minLen {
		minLen = len(other.parts)
	}

	for i := 0; i < minLen; i++ {
		if n.parts[i] < other.parts[i] {
			return true
		}
		if n.parts[i] > other.parts[i] {
			return false
		}
	}

	// All compared parts are equal; shorter ID is "less"
	return len(n.parts) < len(other.parts)
}
