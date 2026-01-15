// Package render provides human-readable formatting for AF framework types.
package render

import (
	"fmt"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
)

// RenderStatus renders the full proof status including tree, statistics, jobs, and legend.
// Returns a meaningful message for nil/empty state (not empty string).
func RenderStatus(s *state.State) string {
	// Handle nil state
	if s == nil {
		return "No proof state initialized."
	}

	// Handle empty state (no nodes)
	nodes := s.AllNodes()
	if len(nodes) == 0 {
		return "No proof initialized. Run 'af init' to start a new proof."
	}

	var sb strings.Builder

	// 1. Header section
	sb.WriteString("=== Proof Status ===\n\n")

	// 2. Tree view section
	treeOutput := RenderTree(s, nil)
	if treeOutput != "" {
		sb.WriteString(treeOutput)
		sb.WriteString("\n")
	}

	// 3. Statistics section
	sb.WriteString("--- Statistics ---\n")
	renderStatistics(&sb, nodes)
	sb.WriteString("\n")

	// 4. Jobs section
	sb.WriteString("--- Jobs ---\n")
	renderJobs(&sb, s, nodes)
	sb.WriteString("\n")

	// 5. Legend section
	sb.WriteString("--- Legend ---\n")
	renderLegend(&sb)

	return sb.String()
}

// renderStatistics writes the statistics section to the builder.
func renderStatistics(sb *strings.Builder, nodes []*node.Node) {
	total := len(nodes)

	// Count epistemic states
	epistemicCounts := make(map[schema.EpistemicState]int)
	for _, n := range nodes {
		epistemicCounts[n.EpistemicState]++
	}

	// Count taint states
	taintCounts := make(map[node.TaintState]int)
	for _, n := range nodes {
		taintCounts[n.TaintState]++
	}

	// Write total count
	sb.WriteString(fmt.Sprintf("Nodes: %d total\n", total))

	// Write epistemic state counts (in fixed order for determinism) with color coding
	sb.WriteString("  Epistemic: ")
	epistemicStates := []schema.EpistemicState{
		schema.EpistemicPending,
		schema.EpistemicValidated,
		schema.EpistemicAdmitted,
		schema.EpistemicRefuted,
		schema.EpistemicArchived,
	}
	epistemicParts := make([]string, len(epistemicStates))
	for i, state := range epistemicStates {
		epistemicParts[i] = fmt.Sprintf("%d %s", epistemicCounts[state], ColorEpistemicState(state))
	}
	sb.WriteString(strings.Join(epistemicParts, ", "))
	sb.WriteString("\n")

	// Write taint state counts (in fixed order for determinism) with color coding
	sb.WriteString("  Taint: ")
	taintStates := []node.TaintState{
		node.TaintClean,
		node.TaintSelfAdmitted,
		node.TaintTainted,
		node.TaintUnresolved,
	}
	taintParts := make([]string, len(taintStates))
	for i, state := range taintStates {
		taintParts[i] = fmt.Sprintf("%d %s", taintCounts[state], ColorTaintState(state))
	}
	sb.WriteString(strings.Join(taintParts, ", "))
	sb.WriteString("\n")
}

// renderJobs writes the jobs section to the builder.
func renderJobs(sb *strings.Builder, s *state.State, nodes []*node.Node) {
	proverJobs := 0
	verifierJobs := 0

	for _, n := range nodes {
		// Prover jobs: available + pending
		if n.WorkflowState == schema.WorkflowAvailable && n.EpistemicState == schema.EpistemicPending {
			proverJobs++
		}

		// Verifier jobs: claimed + pending + all children validated (or no children)
		if n.WorkflowState == schema.WorkflowClaimed && n.EpistemicState == schema.EpistemicPending {
			if s.AllChildrenValidated(n.ID) {
				verifierJobs++
			}
		}
	}

	sb.WriteString(fmt.Sprintf("  Prover: %d nodes awaiting refinement\n", proverJobs))
	sb.WriteString(fmt.Sprintf("  Verifier: %d nodes ready for review\n", verifierJobs))
}

// renderLegend writes the legend section to the builder.
// Uses color coding to visually demonstrate each state's color.
func renderLegend(sb *strings.Builder) {
	// Epistemic states legend with color coding
	sb.WriteString("Epistemic States:\n")
	sb.WriteString(fmt.Sprintf("  %s    - Awaiting proof/verification\n", ColorEpistemicState(schema.EpistemicPending)))
	sb.WriteString(fmt.Sprintf("  %s  - Verified by adversarial verifier\n", ColorEpistemicState(schema.EpistemicValidated)))
	sb.WriteString(fmt.Sprintf("  %s   - Accepted without full verification\n", ColorEpistemicState(schema.EpistemicAdmitted)))
	sb.WriteString(fmt.Sprintf("  %s    - Proven false\n", ColorEpistemicState(schema.EpistemicRefuted)))
	sb.WriteString(fmt.Sprintf("  %s   - Superseded or abandoned\n", ColorEpistemicState(schema.EpistemicArchived)))
	sb.WriteString("\n")

	// Taint states legend with color coding
	sb.WriteString("Taint States:\n")
	sb.WriteString(fmt.Sprintf("  %s         - No epistemic uncertainty\n", ColorTaintState(node.TaintClean)))
	sb.WriteString(fmt.Sprintf("  %s - Contains admitted node\n", ColorTaintState(node.TaintSelfAdmitted)))
	sb.WriteString(fmt.Sprintf("  %s       - Depends on tainted/refuted node\n", ColorTaintState(node.TaintTainted)))
	sb.WriteString(fmt.Sprintf("  %s    - Taint status not yet computed\n", ColorTaintState(node.TaintUnresolved)))
}
