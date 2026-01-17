// Package render provides human-readable and JSON output formatting.
package render

import (
	"fmt"
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
	sb.WriteString("    Examples of issues:\n")
	sb.WriteString("      BAD:  \"x is small\" (vague, no definition of small)\n")
	sb.WriteString("      GOOD: \"x < epsilon for epsilon > 0\"\n")
	sb.WriteString("      BAD:  \"the function is continuous\" (which function? where?)\n")
	sb.WriteString("      GOOD: \"f: R -> R is continuous on [a,b]\"\n")
	sb.WriteString("      BAD:  \"for some n\" (existential should be explicit)\n")
	sb.WriteString("      GOOD: \"there exists n in N such that...\"\n")
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
	sb.WriteString("    Examples of issues:\n")
	sb.WriteString("      BAD:  Using modus_ponens but P->Q not established\n")
	sb.WriteString("      BAD:  Claiming \"by contradiction\" but no contradiction derived\n")
	sb.WriteString("      BAD:  \"Therefore\" with no logical connection to premises\n")
	sb.WriteString("      BAD:  Using universal_instantiation on a non-universal statement\n")
	sb.WriteString("      GOOD: P and P->Q established, then concluding Q by modus_ponens\n")
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
	sb.WriteString("    Examples of issues:\n")
	sb.WriteString("      BAD:  Uses result from node 1.3 but 1.3 not listed as dependency\n")
	sb.WriteString("      BAD:  Depends on node 1.2 which is still pending (unvalidated)\n")
	sb.WriteString("      BAD:  Lists 1.1 as dependency but never actually uses it\n")
	sb.WriteString("      BAD:  Circular dependency (1.2 depends on 1.3, 1.3 depends on 1.2)\n")
	sb.WriteString("      GOOD: All referenced results listed, all dependencies validated\n")
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
	sb.WriteString("    Examples of issues:\n")
	sb.WriteString("      BAD:  Assumes x > 0 without stating it (division by x)\n")
	sb.WriteString("      BAD:  Uses continuity of f without f being declared continuous\n")
	sb.WriteString("      BAD:  \"Clearly\" or \"obviously\" hiding non-trivial steps\n")
	sb.WriteString("      BAD:  Assumes set is non-empty without establishing it\n")
	sb.WriteString("      GOOD: All assumptions explicit in scope or stated as preconditions\n")
	sb.WriteString("\n")
}

// renderDomainRestrictionsCheck writes the domain restrictions check section.
func renderDomainRestrictionsCheck(sb *strings.Builder) {
	sb.WriteString("[ ] DOMAIN RESTRICTIONS\n")
	sb.WriteString("    - Are domain restrictions properly specified?\n")
	sb.WriteString("    - Are variables in the correct domains?\n")
	sb.WriteString("    - Are edge cases handled (division by zero, etc.)?\n")
	sb.WriteString("\n")
	sb.WriteString("    Examples of issues:\n")
	sb.WriteString("      BAD:  sqrt(x) used but x could be negative\n")
	sb.WriteString("      BAD:  Division by (n-1) but n=1 not excluded\n")
	sb.WriteString("      BAD:  log(x) applied to x in R instead of x > 0\n")
	sb.WriteString("      BAD:  Treating integers as reals without justification\n")
	sb.WriteString("      GOOD: Domain restrictions stated: \"for all x > 0 in R\"\n")
	sb.WriteString("\n")
}

// renderNotationConsistencyCheck writes the notation consistency check section.
func renderNotationConsistencyCheck(sb *strings.Builder) {
	sb.WriteString("[ ] NOTATION CONSISTENCY\n")
	sb.WriteString("    - Is notation consistent with the rest of the proof?\n")
	sb.WriteString("    - Are symbols used with their defined meanings?\n")
	sb.WriteString("    - Are there naming conflicts or ambiguities?\n")
	sb.WriteString("\n")
	sb.WriteString("    Examples of issues:\n")
	sb.WriteString("      BAD:  Using 'n' for two different variables in same scope\n")
	sb.WriteString("      BAD:  f(x) here but F(x) elsewhere for same function\n")
	sb.WriteString("      BAD:  Using [a,b] for both intervals and sequences\n")
	sb.WriteString("      BAD:  Redefining epsilon mid-proof\n")
	sb.WriteString("      GOOD: Consistent symbol usage matching definitions\n")
	sb.WriteString("\n")
}

// renderChallengeCommandSuggestion writes the challenge command suggestion.
func renderChallengeCommandSuggestion(sb *strings.Builder, n *node.Node) {
	sb.WriteString("---\n")
	sb.WriteString("To raise a challenge, use:\n")
	sb.WriteString("  af challenge ")
	sb.WriteString(n.ID.String())
	sb.WriteString(" --target <target> --severity <severity> --reason \"<reason>\"\n")
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
	sb.WriteString("\n")
	renderSeverityExplanation(sb)
}

// renderSeverityExplanation writes the severity level explanation.
func renderSeverityExplanation(sb *strings.Builder) {
	sb.WriteString("Challenge severities (determines if acceptance is blocked):\n")
	sb.WriteString("  critical - Fundamental error that must be fixed       [BLOCKS ACCEPTANCE]\n")
	sb.WriteString("  major    - Significant issue that should be addressed [BLOCKS ACCEPTANCE]\n")
	sb.WriteString("  minor    - Minor issue that could be improved         [does not block]\n")
	sb.WriteString("  note     - Clarification request or suggestion        [does not block]\n")
}

// JSONVerificationChecklist represents a verification checklist in JSON format.
type JSONVerificationChecklist struct {
	NodeID           string                       `json:"node_id"`
	Items            []JSONChecklistItem          `json:"items"`
	Dependencies     []JSONChecklistDependency    `json:"dependencies"`
	ChallengeCommand string                       `json:"challenge_command"`
	Severities       []JSONChallengeSeverity      `json:"severities"`
}

// JSONChecklistItem represents a single checklist item for verification.
type JSONChecklistItem struct {
	Category    string             `json:"category"`
	Description string             `json:"description"`
	Details     string             `json:"details,omitempty"`
	Checks      []string           `json:"checks"`
	Examples    []JSONCheckExample `json:"examples,omitempty"`
}

// JSONCheckExample represents a good/bad example for verification guidance.
type JSONCheckExample struct {
	Type   string `json:"type"` // "good" or "bad"
	Text   string `json:"text"`
	Reason string `json:"reason,omitempty"`
}

// JSONChecklistDependency represents a dependency with its status for verification.
type JSONChecklistDependency struct {
	NodeID         string `json:"node_id"`
	Statement      string `json:"statement"`
	EpistemicState string `json:"epistemic_state"`
}

// JSONChallengeSeverity represents a challenge severity level with its description.
type JSONChallengeSeverity struct {
	Level            string `json:"level"`
	Description      string `json:"description"`
	BlocksAcceptance bool   `json:"blocks_acceptance"`
}

// RenderVerificationChecklistJSON generates a JSON-serializable checklist for verifiers.
// Returns a JSON string representing the verification checklist.
// Returns empty JSON object for nil node or nil state.
func RenderVerificationChecklistJSON(n *node.Node, s *state.State) string {
	// Handle nil inputs
	if n == nil || s == nil {
		return "{}"
	}

	checklist := JSONVerificationChecklist{
		NodeID:           n.ID.String(),
		Items:            buildChecklistItems(n),
		Dependencies:     buildDependenciesList(n, s),
		ChallengeCommand: buildChallengeCommand(n),
		Severities:       buildSeveritiesList(),
	}

	data, err := marshalJSON(checklist)
	if err != nil {
		return fmt.Sprintf(`{"node_id":%q,"error":"failed to marshal checklist"}`, n.ID.String())
	}

	return string(data)
}

// buildChecklistItems creates the list of verification checklist items.
func buildChecklistItems(n *node.Node) []JSONChecklistItem {
	items := make([]JSONChecklistItem, 0, 6)

	// 1. Statement precision
	items = append(items, JSONChecklistItem{
		Category:    "statement_precision",
		Description: "Statement Precision",
		Details:     sanitizeStatement(n.Statement),
		Checks: []string{
			"Is the statement mathematically precise?",
			"Are all terms clearly defined?",
			"Are quantifiers explicit and correct?",
		},
		Examples: []JSONCheckExample{
			{Type: "bad", Text: "x is small", Reason: "vague, no definition of small"},
			{Type: "good", Text: "x < epsilon for epsilon > 0", Reason: ""},
			{Type: "bad", Text: "the function is continuous", Reason: "which function? where?"},
			{Type: "good", Text: "f: R -> R is continuous on [a,b]", Reason: ""},
			{Type: "bad", Text: "for some n", Reason: "existential should be explicit"},
			{Type: "good", Text: "there exists n in N such that...", Reason: ""},
		},
	})

	// 2. Inference validity
	inferenceDetails := string(n.Inference)
	if info, ok := schema.GetInferenceInfo(n.Inference); ok {
		inferenceDetails = fmt.Sprintf("%s (%s) - Form: %s", string(n.Inference), info.Name, info.Form)
	}
	items = append(items, JSONChecklistItem{
		Category:    "inference_validity",
		Description: "Inference Validity",
		Details:     inferenceDetails,
		Checks: []string{
			"Does the inference rule apply correctly?",
			"Are the premises sufficient for the conclusion?",
			"Is this the most appropriate inference type?",
		},
		Examples: []JSONCheckExample{
			{Type: "bad", Text: "Using modus_ponens but P->Q not established", Reason: ""},
			{Type: "bad", Text: "Claiming contradiction but no contradiction derived", Reason: ""},
			{Type: "bad", Text: "Therefore with no logical connection to premises", Reason: ""},
			{Type: "bad", Text: "Using universal_instantiation on non-universal statement", Reason: ""},
			{Type: "good", Text: "P and P->Q established, then concluding Q by modus_ponens", Reason: ""},
		},
	})

	// 3. Dependencies (item without dependency-specific details; those go in the dependencies array)
	items = append(items, JSONChecklistItem{
		Category:    "dependencies",
		Description: "Dependencies",
		Details:     fmt.Sprintf("%d dependencies declared", len(n.Dependencies)),
		Checks: []string{
			"Are all dependencies correctly listed?",
			"Are the dependencies validated?",
			"Are there missing dependencies?",
		},
		Examples: []JSONCheckExample{
			{Type: "bad", Text: "Uses result from node 1.3 but 1.3 not listed as dependency", Reason: ""},
			{Type: "bad", Text: "Depends on node 1.2 which is still pending", Reason: "unvalidated"},
			{Type: "bad", Text: "Lists 1.1 as dependency but never actually uses it", Reason: ""},
			{Type: "bad", Text: "Circular dependency (1.2 depends on 1.3, 1.3 depends on 1.2)", Reason: ""},
			{Type: "good", Text: "All referenced results listed, all dependencies validated", Reason: ""},
		},
	})

	// 4. Hidden assumptions
	items = append(items, JSONChecklistItem{
		Category:    "hidden_assumptions",
		Description: "Hidden Assumptions",
		Checks: []string{
			"Are there unstated assumptions being used?",
			"Are all preconditions explicitly stated?",
			"Does the step rely on facts not in scope?",
		},
		Examples: []JSONCheckExample{
			{Type: "bad", Text: "Assumes x > 0 without stating it", Reason: "division by x"},
			{Type: "bad", Text: "Uses continuity of f without f being declared continuous", Reason: ""},
			{Type: "bad", Text: "Clearly or obviously hiding non-trivial steps", Reason: ""},
			{Type: "bad", Text: "Assumes set is non-empty without establishing it", Reason: ""},
			{Type: "good", Text: "All assumptions explicit in scope or stated as preconditions", Reason: ""},
		},
	})

	// 5. Domain restrictions
	items = append(items, JSONChecklistItem{
		Category:    "domain_restrictions",
		Description: "Domain Restrictions",
		Checks: []string{
			"Are domain restrictions properly specified?",
			"Are variables in the correct domains?",
			"Are edge cases handled (division by zero, etc.)?",
		},
		Examples: []JSONCheckExample{
			{Type: "bad", Text: "sqrt(x) used but x could be negative", Reason: ""},
			{Type: "bad", Text: "Division by (n-1) but n=1 not excluded", Reason: ""},
			{Type: "bad", Text: "log(x) applied to x in R instead of x > 0", Reason: ""},
			{Type: "bad", Text: "Treating integers as reals without justification", Reason: ""},
			{Type: "good", Text: "Domain restrictions stated: for all x > 0 in R", Reason: ""},
		},
	})

	// 6. Notation consistency
	items = append(items, JSONChecklistItem{
		Category:    "notation_consistency",
		Description: "Notation Consistency",
		Checks: []string{
			"Is notation consistent with the rest of the proof?",
			"Are symbols used with their defined meanings?",
			"Are there naming conflicts or ambiguities?",
		},
		Examples: []JSONCheckExample{
			{Type: "bad", Text: "Using n for two different variables in same scope", Reason: ""},
			{Type: "bad", Text: "f(x) here but F(x) elsewhere for same function", Reason: ""},
			{Type: "bad", Text: "Using [a,b] for both intervals and sequences", Reason: ""},
			{Type: "bad", Text: "Redefining epsilon mid-proof", Reason: ""},
			{Type: "good", Text: "Consistent symbol usage matching definitions", Reason: ""},
		},
	})

	return items
}

// buildDependenciesList creates the list of dependencies with their status.
func buildDependenciesList(n *node.Node, s *state.State) []JSONChecklistDependency {
	if len(n.Dependencies) == 0 {
		return []JSONChecklistDependency{}
	}

	// Sort dependencies for deterministic output
	deps := make([]types.NodeID, len(n.Dependencies))
	copy(deps, n.Dependencies)
	sort.Slice(deps, func(i, j int) bool {
		return compareNodeIDs(deps[i].String(), deps[j].String())
	})

	result := make([]JSONChecklistDependency, 0, len(deps))
	for _, depID := range deps {
		dep := JSONChecklistDependency{
			NodeID:         depID.String(),
			Statement:      "",
			EpistemicState: "unknown",
		}

		if depNode := s.GetNode(depID); depNode != nil {
			dep.Statement = depNode.Statement
			dep.EpistemicState = string(depNode.EpistemicState)
		}

		result = append(result, dep)
	}

	return result
}

// buildChallengeCommand constructs the suggested challenge command string.
func buildChallengeCommand(n *node.Node) string {
	return fmt.Sprintf("af challenge %s --target <target> --severity <severity> --reason \"<reason>\"", n.ID.String())
}

// buildSeveritiesList creates the list of challenge severity levels with descriptions.
func buildSeveritiesList() []JSONChallengeSeverity {
	return []JSONChallengeSeverity{
		{
			Level:            "critical",
			Description:      "Fundamental error that must be fixed",
			BlocksAcceptance: true,
		},
		{
			Level:            "major",
			Description:      "Significant issue that should be addressed",
			BlocksAcceptance: true,
		},
		{
			Level:            "minor",
			Description:      "Minor issue that could be improved",
			BlocksAcceptance: false,
		},
		{
			Level:            "note",
			Description:      "Clarification request or suggestion",
			BlocksAcceptance: false,
		},
	}
}
