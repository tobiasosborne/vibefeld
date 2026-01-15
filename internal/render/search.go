// Package render provides search result formatting for AF framework types.
package render

import (
	"sort"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
)

// SearchResult represents a node match from a search query.
type SearchResult struct {
	Node        *node.Node
	MatchReason string // Describes why this node matched (e.g., "text match", "state: pending")
}

// FormatSearchResults renders search results as human-readable text.
// Returns empty string for nil or empty results.
// Output format: one line per result with ID and brief statement.
func FormatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No matching nodes found.\n"
	}

	// Sort by node ID for consistent output
	sorted := make([]SearchResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		return compareNodeIDs(sorted[i].Node.ID.String(), sorted[j].Node.ID.String())
	})

	var sb strings.Builder
	sb.WriteString("Search Results:\n")
	sb.WriteString(strings.Repeat("-", 60))
	sb.WriteString("\n")

	for _, r := range sorted {
		if r.Node == nil {
			continue
		}

		// Format: [ID] (epistemic_state) "statement" -- match reason
		stmt := sanitizeStatement(r.Node.Statement)
		// Truncate statement for search results to keep output readable
		if len(stmt) > 60 {
			stmt = stmt[:57] + "..."
		}

		sb.WriteString("[")
		sb.WriteString(r.Node.ID.String())
		sb.WriteString("] (")
		sb.WriteString(ColorEpistemicState(r.Node.EpistemicState))
		sb.WriteString(") ")
		sb.WriteString("\"")
		sb.WriteString(stmt)
		sb.WriteString("\"")
		if r.MatchReason != "" {
			sb.WriteString(" -- ")
			sb.WriteString(r.MatchReason)
		}
		sb.WriteString("\n")
	}

	sb.WriteString(strings.Repeat("-", 60))
	sb.WriteString("\n")
	sb.WriteString("Total: ")
	sb.WriteString(formatCount(len(results)))
	sb.WriteString("\n")

	return sb.String()
}

// formatCount returns a human-readable count string.
func formatCount(count int) string {
	if count == 1 {
		return "1 node"
	}
	return formatInt(count) + " nodes"
}

// FormatSearchResultsJSON renders search results as JSON.
// Returns JSON array string. Returns "[]" for nil or empty results.
func FormatSearchResultsJSON(results []SearchResult) string {
	if len(results) == 0 {
		return `{"results":[],"total":0}`
	}

	// Sort by node ID for consistent output
	sorted := make([]SearchResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		return compareNodeIDs(sorted[i].Node.ID.String(), sorted[j].Node.ID.String())
	})

	// Build JSON manually to avoid import cycles
	var sb strings.Builder
	sb.WriteString(`{"results":[`)

	first := true
	for _, r := range sorted {
		if r.Node == nil {
			continue
		}

		if !first {
			sb.WriteString(",")
		}
		first = false

		sb.WriteString(`{"id":"`)
		sb.WriteString(escapeJSON(r.Node.ID.String()))
		sb.WriteString(`","type":"`)
		sb.WriteString(escapeJSON(string(r.Node.Type)))
		sb.WriteString(`","statement":"`)
		sb.WriteString(escapeJSON(r.Node.Statement))
		sb.WriteString(`","epistemic_state":"`)
		sb.WriteString(escapeJSON(string(r.Node.EpistemicState)))
		sb.WriteString(`","workflow_state":"`)
		sb.WriteString(escapeJSON(string(r.Node.WorkflowState)))
		sb.WriteString(`"`)
		if r.MatchReason != "" {
			sb.WriteString(`,"match_reason":"`)
			sb.WriteString(escapeJSON(r.MatchReason))
			sb.WriteString(`"`)
		}
		sb.WriteString(`}`)
	}

	sb.WriteString(`],"total":`)
	sb.WriteString(formatInt(len(results)))
	sb.WriteString(`}`)

	return sb.String()
}

// escapeJSON escapes a string for JSON output.
func escapeJSON(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			if r < 32 {
				// Control character - skip or escape
				continue
			}
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// formatInt converts an integer to string without using fmt.
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + formatInt(-n)
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
