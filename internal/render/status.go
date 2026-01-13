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

	// Write epistemic state counts (in fixed order for determinism)
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
		epistemicParts[i] = fmt.Sprintf("%d %s", epistemicCounts[state], state)
	}
	sb.WriteString(strings.Join(epistemicParts, ", "))
	sb.WriteString("\n")

	// Write taint state counts (in fixed order for determinism)
	sb.WriteString("  Taint: ")
	taintStates := []node.TaintState{
		node.TaintClean,
		node.TaintSelfAdmitted,
		node.TaintTainted,
		node.TaintUnresolved,
	}
	taintParts := make([]string, len(taintStates))
	for i, state := range taintStates {
		taintParts[i] = fmt.Sprintf("%d %s", taintCounts[state], state)
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
			if allChildrenValidated(s, n, nodes) {
				verifierJobs++
			}
		}
	}

	sb.WriteString(fmt.Sprintf("  Prover: %d nodes awaiting refinement\n", proverJobs))
	sb.WriteString(fmt.Sprintf("  Verifier: %d nodes ready for review\n", verifierJobs))
}

// allChildrenValidated returns true if all direct children of the node are validated.
// Returns true if the node has no children.
func allChildrenValidated(s *state.State, parent *node.Node, allNodes []*node.Node) bool {
	parentStr := parent.ID.String()

	for _, n := range allNodes {
		// Check if n is a direct child of parent
		p, hasParent := n.ID.Parent()
		if !hasParent {
			continue
		}

		if p.String() == parentStr {
			if n.EpistemicState != schema.EpistemicValidated {
				return false
			}
		}
	}

	// If we got here, either no children exist or all children are validated
	return true
}

// renderLegend writes the legend section to the builder.
func renderLegend(sb *strings.Builder) {
	// Epistemic states legend
	sb.WriteString("Epistemic States:\n")
	sb.WriteString("  pending    - Awaiting proof/verification\n")
	sb.WriteString("  validated  - Verified by adversarial verifier\n")
	sb.WriteString("  admitted   - Accepted without full verification\n")
	sb.WriteString("  refuted    - Proven false\n")
	sb.WriteString("  archived   - Superseded or abandoned\n")
	sb.WriteString("\n")

	// Taint states legend
	sb.WriteString("Taint States:\n")
	sb.WriteString("  clean         - No epistemic uncertainty\n")
	sb.WriteString("  self_admitted - Contains admitted node\n")
	sb.WriteString("  tainted       - Depends on tainted/refuted node\n")
	sb.WriteString("  unresolved    - Taint status not yet computed\n")
}
