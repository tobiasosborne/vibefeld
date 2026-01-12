// Package hash provides content hashing for proof nodes.
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// ComputeNodeHash computes a deterministic SHA256 hash for a proof node.
// The hash is computed from the node's type, statement, latex, inference,
// context definitions, and dependencies.
//
// Arrays (context and dependencies) are sorted before hashing to ensure
// order-independent results. Empty and nil slices are treated identically.
//
// Returns a 64-character lowercase hexadecimal string.
func ComputeNodeHash(nodeType, statement, latex, inference string, context, dependencies []string) (string, error) {
	// Copy and sort context to ensure order-independence
	sortedContext := copyAndSort(context)

	// Copy and sort dependencies to ensure order-independence
	sortedDeps := copyAndSort(dependencies)

	// Build the content to hash using a consistent serialization format.
	// Each field is separated by a null byte delimiter to prevent collisions.
	// Arrays are joined with unit separator (0x1F) between elements.
	var builder strings.Builder

	// Write string fields with null byte separators
	builder.WriteString(nodeType)
	builder.WriteByte(0x00)
	builder.WriteString(statement)
	builder.WriteByte(0x00)
	builder.WriteString(latex)
	builder.WriteByte(0x00)
	builder.WriteString(inference)
	builder.WriteByte(0x00)

	// Write sorted context array
	builder.WriteString(strings.Join(sortedContext, "\x1F"))
	builder.WriteByte(0x00)

	// Write sorted dependencies array
	builder.WriteString(strings.Join(sortedDeps, "\x1F"))

	// Compute SHA256 hash
	sum := sha256.Sum256([]byte(builder.String()))

	// Return lowercase hex encoding
	return hex.EncodeToString(sum[:]), nil
}

// copyAndSort creates a copy of the input slice and sorts it.
// Returns an empty slice (not nil) for nil input to ensure consistent hashing.
func copyAndSort(input []string) []string {
	if input == nil {
		return []string{}
	}

	// Make a copy to avoid mutating the input
	result := make([]string, len(input))
	copy(result, input)

	// Sort the copy
	sort.Strings(result)

	return result
}
