// Package render provides human-readable formatting for AF framework types.
package render

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/types"
)

const (
	// indentSize is the number of spaces per indentation level.
	indentSize = 2
)

// RenderNode renders a node as a single-line human-readable summary.
// Format: [ID] type (state): "statement"
// Returns empty string for nil node.
// Mathematical statements are shown in full without truncation to preserve precision.
// Uses color coding for epistemic state when color is enabled.
func RenderNode(n *node.Node) string {
	if n == nil {
		return ""
	}

	// Sanitize the statement (normalize whitespace) but do NOT truncate
	// Mathematical proofs require precision - truncation is unacceptable
	stmt := sanitizeStatement(n.Statement)

	return fmt.Sprintf("[%s] %s (%s): %q",
		n.ID.String(),
		string(n.Type),
		ColorEpistemicState(n.EpistemicState),
		stmt,
	)
}

// RenderNodeVerbose renders a node with all fields in multi-line format.
// Includes: ID, type, statement, inference, workflow state, epistemic state,
// taint state, created timestamp, and optional fields (context, dependencies, scope).
// Returns empty string for nil node.
// Uses color coding for epistemic and taint states when color is enabled.
func RenderNodeVerbose(n *node.Node) string {
	if n == nil {
		return ""
	}

	var sb strings.Builder

	// Core fields
	sb.WriteString(fmt.Sprintf("ID:         %s\n", n.ID.String()))
	sb.WriteString(fmt.Sprintf("Type:       %s\n", string(n.Type)))
	sb.WriteString(fmt.Sprintf("Statement:  %s\n", n.Statement))
	sb.WriteString(fmt.Sprintf("Inference:  %s\n", string(n.Inference)))
	sb.WriteString(fmt.Sprintf("Workflow:   %s\n", string(n.WorkflowState)))
	sb.WriteString(fmt.Sprintf("Epistemic:  %s\n", ColorEpistemicState(n.EpistemicState)))
	sb.WriteString(fmt.Sprintf("Taint:      %s\n", ColorTaintState(n.TaintState)))
	sb.WriteString(fmt.Sprintf("Created:    %s\n", n.Created.String()))
	sb.WriteString(fmt.Sprintf("Hash:       %s\n", n.ContentHash))

	// Optional fields
	if len(n.Context) > 0 {
		sb.WriteString(fmt.Sprintf("Context:    %s\n", strings.Join(n.Context, ", ")))
	}

	if len(n.Dependencies) > 0 {
		sb.WriteString(fmt.Sprintf("Depends on: %s\n", strings.Join(types.ToStringSlice(n.Dependencies), ", ")))
	}

	if len(n.ValidationDeps) > 0 {
		sb.WriteString(fmt.Sprintf("Requires validated: %s\n", strings.Join(types.ToStringSlice(n.ValidationDeps), ", ")))
	}

	if len(n.Scope) > 0 {
		sb.WriteString(fmt.Sprintf("Scope:      %s\n", strings.Join(n.Scope, ", ")))
	}

	if n.ClaimedBy != "" {
		sb.WriteString(fmt.Sprintf("Claimed by: %s\n", n.ClaimedBy))
	}

	return sb.String()
}

// RenderNodeTree renders a list of nodes as an indented tree structure.
// Nodes are sorted by ID and indented based on their depth in the hierarchy.
// Returns empty string for nil or empty node list.
// Uses color coding for epistemic state when color is enabled.
func RenderNodeTree(nodes []*node.Node) string {
	if len(nodes) == 0 {
		return ""
	}

	// Filter out nil nodes
	validNodes := make([]*node.Node, 0, len(nodes))
	for _, n := range nodes {
		if n != nil {
			validNodes = append(validNodes, n)
		}
	}

	if len(validNodes) == 0 {
		return ""
	}

	// Sort nodes by their ID string (lexicographic, which works for hierarchical IDs)
	sort.Slice(validNodes, func(i, j int) bool {
		return compareNodeIDs(validNodes[i].ID.String(), validNodes[j].ID.String())
	})

	var sb strings.Builder
	for i, n := range validNodes {
		// Calculate indentation based on depth (root is depth 1, so indent = (depth-1) * indentSize)
		indent := strings.Repeat(" ", (n.Depth()-1)*indentSize)

		// Render single line for each node
		// Sanitize but do NOT truncate - mathematical formulas must be shown in full
		stmt := sanitizeStatement(n.Statement)

		sb.WriteString(fmt.Sprintf("%s[%s] %s (%s): %q",
			indent,
			n.ID.String(),
			string(n.Type),
			ColorEpistemicState(n.EpistemicState),
			stmt,
		))

		if i < len(validNodes)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// sanitizeStatement removes or replaces characters that would break single-line output.
func sanitizeStatement(s string) string {
	// Replace newlines and tabs with spaces, and collapse multiple spaces into one
	// using O(n) single-pass algorithm with strings.Builder
	var sb strings.Builder
	sb.Grow(len(s))
	prevSpace := true // Start true to trim leading spaces

	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' || r == ' ' {
			if !prevSpace {
				sb.WriteByte(' ')
				prevSpace = true
			}
		} else {
			sb.WriteRune(r)
			prevSpace = false
		}
	}

	result := sb.String()
	// Trim trailing space if present
	if len(result) > 0 && result[len(result)-1] == ' ' {
		result = result[:len(result)-1]
	}
	return result
}


// compareNodeIDs compares two node ID strings for sorting.
// Returns true if a should come before b.
func compareNodeIDs(a, b string) bool {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	minLen := len(partsA)
	if len(partsB) < minLen {
		minLen = len(partsB)
	}

	for i := 0; i < minLen; i++ {
		// Parse as integers for numeric comparison using strconv.Atoi (faster than fmt.Sscanf)
		numA, _ := strconv.Atoi(partsA[i])
		numB, _ := strconv.Atoi(partsB[i])

		if numA != numB {
			return numA < numB
		}
	}

	// If all common parts are equal, shorter ID comes first
	return len(partsA) < len(partsB)
}
