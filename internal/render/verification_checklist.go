// Package render provides human-readable and JSON output formatting.
package render

import (
	"sort"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// RenderVerificationChecklist generates a checklist for verifiers reviewing a proof node.
// The checklist helps verifiers systematically check all aspects of the node for correctness.
// Returns empty string for nil node or nil state.
func RenderVerificationChecklist(n *node.Node, s *state.State) string {
	// Handle nil inputs
	if n == nil || s == nil {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString("=== Verification Checklist for Node ")
	sb.WriteString(n.ID.String())
	sb.WriteString(" ===\n\n")

	// 1. Statement precision check
	renderStatementCheck(&sb, n)

	// 2. Inference validity check
	renderInferenceCheck(&sb, n)

	// 3. Dependencies check
	renderDependenciesCheck(&sb, n, s)

	// 4. Hidden assumptions check
	renderHiddenAssumptionsCheck(&sb)

	// 5. Domain restrictions check
	renderDomainRestrictionsCheck(&sb)

	// 6. Notation consistency check
	renderNotationConsistencyCheck(&sb)

	// 7. Suggest challenge command
	renderChallengeCommandSuggestion(&sb, n)

	return sb.String()
}

// renderStatementCheck writes the statement precision check section.
func renderStatementCheck(sb *strings.Builder, n *node.Node) {
	sb.WriteString("[ ] STATEMENT PRECISION\n")
	sb.WriteString("    Statement: ")
	sb.WriteString(sanitizeStatement(n.Statement))
	sb.WriteString("\n")
	sb.WriteString("    - Is the statement mathematically precise?\n")
	sb.WriteString("    - Are all terms clearly defined?\n")
	sb.WriteString("    - Are quantifiers explicit and correct?\n")
	sb.WriteString("\n")
}

// renderInferenceCheck writes the inference validity check section.
func renderInferenceCheck(sb *strings.Builder, n *node.Node) {
	sb.WriteString("[ ] INFERENCE VALIDITY\n")
	sb.WriteString("    Inference type: ")
	sb.WriteString(string(n.Inference))

	// Get inference info for human-readable name and form
	if info, ok := schema.GetInferenceInfo(n.Inference); ok {
		sb.WriteString(" (")
		sb.WriteString(info.Name)
		sb.WriteString(")\n")
		sb.WriteString("    Logical form: ")
		sb.WriteString(info.Form)
		sb.WriteString("\n")
	} else {
		sb.WriteString("\n")
	}

	sb.WriteString("    - Does the inference rule apply correctly?\n")
	sb.WriteString("    - Are the premises sufficient for the conclusion?\n")
	sb.WriteString("    - Is this the most appropriate inference type?\n")
	sb.WriteString("\n")
}

// renderDependenciesCheck writes the dependencies check section.
func renderDependenciesCheck(sb *strings.Builder, n *node.Node, s *state.State) {
	sb.WriteString("[ ] DEPENDENCIES\n")

	if len(n.Dependencies) == 0 {
		sb.WriteString("    (no dependencies declared)\n")
		sb.WriteString("    - Should this node have dependencies?\n")
		sb.WriteString("\n")
		return
	}

	sb.WriteString("    Listed dependencies:\n")

	// Sort dependencies for deterministic output
	deps := make([]types.NodeID, len(n.Dependencies))
	copy(deps, n.Dependencies)
	sort.Slice(deps, func(i, j int) bool {
		return compareNodeIDs(deps[i].String(), deps[j].String())
	})

	for _, depID := range deps {
		sb.WriteString("      ")
		sb.WriteString(depID.String())

		// Get dependency node to show status
		depNode := s.GetNode(depID)
		if depNode != nil {
			sb.WriteString(" ")
			sb.WriteString(renderDependencyStatus(depNode.EpistemicState))
			sb.WriteString(": ")
			// Truncate statement for readability
			stmt := sanitizeStatement(depNode.Statement)
			if len(stmt) > 50 {
				stmt = stmt[:47] + "..."
			}
			sb.WriteString(stmt)
		} else {
			sb.WriteString(" (not found)")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("    - Are all dependencies correctly listed?\n")
	sb.WriteString("    - Are the dependencies validated?\n")
	sb.WriteString("    - Are there missing dependencies?\n")
	sb.WriteString("\n")
}

// renderDependencyStatus returns a human-readable status indicator for a dependency's epistemic state.
func renderDependencyStatus(es schema.EpistemicState) string {
	switch es {
	case schema.EpistemicValidated:
		return "(validated)"
	case schema.EpistemicAdmitted:
		return "(admitted)"
	case schema.EpistemicPending:
		return "(pending)"
	case schema.EpistemicRefuted:
		return "(REFUTED)"
	case schema.EpistemicArchived:
		return "(archived)"
	default:
		return "(unknown)"
	}
}

// renderHiddenAssumptionsCheck writes the hidden assumptions check section.
func renderHiddenAssumptionsCheck(sb *strings.Builder) {
	sb.WriteString("[ ] HIDDEN ASSUMPTIONS\n")
	sb.WriteString("    - Are there unstated assumptions being used?\n")
	sb.WriteString("    - Are all preconditions explicitly stated?\n")
	sb.WriteString("    - Does the step rely on facts not in scope?\n")
	sb.WriteString("\n")
}

// renderDomainRestrictionsCheck writes the domain restrictions check section.
func renderDomainRestrictionsCheck(sb *strings.Builder) {
	sb.WriteString("[ ] DOMAIN RESTRICTIONS\n")
	sb.WriteString("    - Are domain restrictions properly specified?\n")
	sb.WriteString("    - Are variables in the correct domains?\n")
	sb.WriteString("    - Are edge cases handled (division by zero, etc.)?\n")
	sb.WriteString("\n")
}

// renderNotationConsistencyCheck writes the notation consistency check section.
func renderNotationConsistencyCheck(sb *strings.Builder) {
	sb.WriteString("[ ] NOTATION CONSISTENCY\n")
	sb.WriteString("    - Is notation consistent with the rest of the proof?\n")
	sb.WriteString("    - Are symbols used with their defined meanings?\n")
	sb.WriteString("    - Are there naming conflicts or ambiguities?\n")
	sb.WriteString("\n")
}

// renderChallengeCommandSuggestion writes the challenge command suggestion.
func renderChallengeCommandSuggestion(sb *strings.Builder, n *node.Node) {
	sb.WriteString("---\n")
	sb.WriteString("To raise a challenge, use:\n")
	sb.WriteString("  af challenge ")
	sb.WriteString(n.ID.String())
	sb.WriteString(" --target <statement|inference|dependencies|gap|domain|scope|context|type_error|completeness> --reason \"<reason>\"\n")
	sb.WriteString("\n")
	sb.WriteString("Challenge targets:\n")
	sb.WriteString("  statement    - The claim text itself is disputed\n")
	sb.WriteString("  inference    - The inference rule is misapplied\n")
	sb.WriteString("  dependencies - Node dependencies are incorrect or incomplete\n")
	sb.WriteString("  gap          - Logical gap in reasoning\n")
	sb.WriteString("  domain       - Domain restriction violation\n")
	sb.WriteString("  scope        - Scope/local assumption issues\n")
	sb.WriteString("  context      - Referenced definitions or externals are wrong\n")
	sb.WriteString("  type_error   - Type mismatch in mathematical objects\n")
	sb.WriteString("  completeness - Missing cases in argument\n")
}
