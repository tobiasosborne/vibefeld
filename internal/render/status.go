// Package render provides human-readable formatting for AF framework types.
package render

import (
	"fmt"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// RenderStatus renders the full proof status including tree, statistics, jobs, and legend.
// Supports pagination via limit and offset parameters:
//   - limit: maximum number of nodes to display (0 = unlimited)
//   - offset: number of nodes to skip before displaying (0 = no skip)
//
// Returns a meaningful message for nil/empty state (not empty string).
func RenderStatus(s *state.State, limit, offset int) string {
	// Handle nil state
	if s == nil {
		return "No proof state initialized."
	}

	// Handle empty state (no nodes)
	nodes := s.AllNodes()
	if len(nodes) == 0 {
		return "No proof initialized. Run 'af init' to start a new proof."
	}

	// Sort nodes by ID for consistent pagination
	sortNodesByID(nodes)

	// Apply pagination
	paginatedNodes := applyPagination(nodes, limit, offset)

	var sb strings.Builder

	// 1. Header section
	sb.WriteString("=== Proof Status ===\n\n")

	// 2. Tree view section (uses paginated nodes)
	treeOutput := RenderTreeForNodes(s, paginatedNodes)
	if treeOutput != "" {
		sb.WriteString(treeOutput)
		sb.WriteString("\n")
	}

	// 3. Statistics section (uses paginated nodes for display, but shows pagination info)
	sb.WriteString("--- Statistics ---\n")
	renderStatisticsWithPagination(&sb, paginatedNodes, len(nodes), limit, offset)
	sb.WriteString("\n")

	// 4. Jobs section (calculated from paginated nodes)
	sb.WriteString("--- Jobs ---\n")
	renderJobs(&sb, s, paginatedNodes)
	sb.WriteString("\n")

	// 5. Legend section
	sb.WriteString("--- Legend ---\n")
	renderLegend(&sb)

	return sb.String()
}

// stateCounts holds the counts of epistemic and taint states for a collection of nodes.
type stateCounts struct {
	epistemic map[schema.EpistemicState]int
	taint     map[node.TaintState]int
}

// countStates counts epistemic and taint states for the given nodes in a single pass.
func countStates(nodes []*node.Node) stateCounts {
	counts := stateCounts{
		epistemic: make(map[schema.EpistemicState]int),
		taint:     make(map[node.TaintState]int),
	}
	for _, n := range nodes {
		counts.epistemic[n.EpistemicState]++
		counts.taint[n.TaintState]++
	}
	return counts
}

// writeStateCounts writes epistemic and taint state counts to the builder.
func writeStateCounts(sb *strings.Builder, counts stateCounts) {
	// Write epistemic state counts (in fixed order for determinism) with color coding
	sb.WriteString("  Epistemic: ")
	epistemicStates := []schema.EpistemicState{
		schema.EpistemicDraft,
		schema.EpistemicPending,
		schema.EpistemicValidated,
		schema.EpistemicAdmitted,
		schema.EpistemicRefuted,
		schema.EpistemicArchived,
		schema.EpistemicNeedsRefinement,
	}
	epistemicParts := make([]string, len(epistemicStates))
	for i, state := range epistemicStates {
		epistemicParts[i] = fmt.Sprintf("%d %s", counts.epistemic[state], ColorEpistemicState(state))
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
		taintParts[i] = fmt.Sprintf("%d %s", counts.taint[state], ColorTaintState(state))
	}
	sb.WriteString(strings.Join(taintParts, ", "))
	sb.WriteString("\n")
}

// applyPagination applies limit and offset to a slice of nodes.
// offset=0 means no skip, limit=0 means no limit.
func applyPagination(nodes []*node.Node, limit, offset int) []*node.Node {
	total := len(nodes)

	// Apply offset
	if offset > 0 {
		if offset >= total {
			return []*node.Node{}
		}
		nodes = nodes[offset:]
	}

	// Apply limit
	if limit > 0 && limit < len(nodes) {
		nodes = nodes[:limit]
	}

	return nodes
}

// renderStatisticsWithPagination writes the statistics section including pagination info.
func renderStatisticsWithPagination(sb *strings.Builder, nodes []*node.Node, totalNodes, limit, offset int) {
	displayed := len(nodes)
	counts := countStates(nodes)

	// Write total count with pagination info
	if limit > 0 || offset > 0 {
		sb.WriteString(fmt.Sprintf("Nodes: %d displayed (of %d total, offset=%d, limit=%d)\n", displayed, totalNodes, offset, limit))
	} else {
		sb.WriteString(fmt.Sprintf("Nodes: %d total\n", totalNodes))
	}

	writeStateCounts(sb, counts)
}

// renderStatistics writes the statistics section to the builder.
func renderStatistics(sb *strings.Builder, nodes []*node.Node) {
	counts := countStates(nodes)

	sb.WriteString(fmt.Sprintf("Nodes: %d total\n", len(nodes)))
	writeStateCounts(sb, counts)
}

// renderJobs writes the jobs section to the builder.
func renderJobs(sb *strings.Builder, s *state.State, nodes []*node.Node) {
	proverJobs := 0
	verifierJobs := 0

	for _, n := range nodes {
		// Prover jobs: available + (draft OR pending OR needs_refinement)
		// Nodes in draft/needs_refinement state need further development by provers
		if n.WorkflowState == schema.WorkflowAvailable &&
			(n.EpistemicState == schema.EpistemicDraft || n.EpistemicState == schema.EpistemicPending || n.EpistemicState == schema.EpistemicNeedsRefinement) {
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
	sb.WriteString(fmt.Sprintf("  %s      - Work in progress (not submitted)\n", ColorEpistemicState(schema.EpistemicDraft)))
	sb.WriteString(fmt.Sprintf("  %s    - Awaiting proof/verification\n", ColorEpistemicState(schema.EpistemicPending)))
	sb.WriteString(fmt.Sprintf("  %s  - Verified by adversarial verifier\n", ColorEpistemicState(schema.EpistemicValidated)))
	sb.WriteString(fmt.Sprintf("  %s   - Accepted without full verification\n", ColorEpistemicState(schema.EpistemicAdmitted)))
	sb.WriteString(fmt.Sprintf("  %s    - Proven false\n", ColorEpistemicState(schema.EpistemicRefuted)))
	sb.WriteString(fmt.Sprintf("  %s   - Superseded or abandoned\n", ColorEpistemicState(schema.EpistemicArchived)))
	sb.WriteString(fmt.Sprintf("  %s - Reopened for further refinement\n", ColorEpistemicState(schema.EpistemicNeedsRefinement)))
	sb.WriteString("\n")

	// Taint states legend with color coding
	sb.WriteString("Taint States:\n")
	sb.WriteString(fmt.Sprintf("  %s         - No epistemic uncertainty\n", ColorTaintState(node.TaintClean)))
	sb.WriteString(fmt.Sprintf("  %s - This node is admitted\n", ColorTaintState(node.TaintSelfAdmitted)))
	sb.WriteString(fmt.Sprintf("  %s       - Depends on an admitted node (ancestor or descendant)\n", ColorTaintState(node.TaintTainted)))
	sb.WriteString(fmt.Sprintf("  %s    - Some node in its chain needs verification or refinement\n", ColorTaintState(node.TaintUnresolved)))
}

// UrgentItem represents a single urgent work item for display.
type UrgentItem struct {
	NodeID    string
	Statement string
	Category  string // "blocking_challenge", "prover_job", "verifier_job"
	Details   string // Additional context (e.g., challenge reason)
}

// FilterUrgentNodes returns nodes that need immediate attention:
// 1. Nodes with open blocking challenges (critical or major severity)
// 2. Available prover jobs (available + pending)
// 3. Ready verifier jobs (claimed + pending + all children validated)
func FilterUrgentNodes(s *state.State) []UrgentItem {
	if s == nil {
		return nil
	}

	var items []UrgentItem
	seenNodes := make(map[string]bool)

	// 1. Nodes with blocking challenges (highest priority)
	for _, challenge := range s.OpenChallenges() {
		if !schema.SeverityBlocksAcceptance(schema.ChallengeSeverity(challenge.Severity)) {
			continue
		}
		nodeIDStr := challenge.NodeID.String()
		if seenNodes[nodeIDStr] {
			continue
		}
		seenNodes[nodeIDStr] = true

		n := s.GetNode(challenge.NodeID)
		statement := ""
		if n != nil {
			statement = n.Statement
		}
		items = append(items, UrgentItem{
			NodeID:    nodeIDStr,
			Statement: statement,
			Category:  "blocking_challenge",
			Details:   fmt.Sprintf("[%s] %s: %s", challenge.Severity, challenge.Target, challenge.Reason),
		})
	}

	// 2. Prover jobs: available + (draft OR pending OR needs_refinement)
	for _, n := range s.AllNodes() {
		nodeIDStr := n.ID.String()
		if seenNodes[nodeIDStr] {
			continue
		}
		if n.WorkflowState == schema.WorkflowAvailable &&
			(n.EpistemicState == schema.EpistemicDraft || n.EpistemicState == schema.EpistemicPending || n.EpistemicState == schema.EpistemicNeedsRefinement) {
			seenNodes[nodeIDStr] = true
			details := "Needs refinement"
			if n.EpistemicState == schema.EpistemicNeedsRefinement {
				details = "Refinement requested (reopened)"
			}
			items = append(items, UrgentItem{
				NodeID:    nodeIDStr,
				Statement: n.Statement,
				Category:  "prover_job",
				Details:   details,
			})
		}
	}

	// 3. Verifier jobs: claimed + pending + all children validated
	for _, n := range s.AllNodes() {
		nodeIDStr := n.ID.String()
		if seenNodes[nodeIDStr] {
			continue
		}
		if n.WorkflowState == schema.WorkflowClaimed && n.EpistemicState == schema.EpistemicPending {
			if s.AllChildrenValidated(n.ID) {
				seenNodes[nodeIDStr] = true
				items = append(items, UrgentItem{
					NodeID:    nodeIDStr,
					Statement: n.Statement,
					Category:  "verifier_job",
					Details:   "Ready for verification",
				})
			}
		}
	}

	return items
}

// RenderStatusUrgent renders only urgent items that need immediate attention.
// This provides a focused view for agents to quickly find actionable work.
func RenderStatusUrgent(s *state.State) string {
	if s == nil {
		return "No proof state initialized."
	}

	nodes := s.AllNodes()
	if len(nodes) == 0 {
		return "No proof initialized. Run 'af init' to start a new proof."
	}

	urgentItems := FilterUrgentNodes(s)

	var sb strings.Builder
	sb.WriteString("=== Urgent Items ===\n\n")

	if len(urgentItems) == 0 {
		sb.WriteString("No urgent items. Proof is progressing normally.\n")
		return sb.String()
	}

	// Group by category
	var blockingChallenges, proverJobs, verifierJobs []UrgentItem
	for _, item := range urgentItems {
		switch item.Category {
		case "blocking_challenge":
			blockingChallenges = append(blockingChallenges, item)
		case "prover_job":
			proverJobs = append(proverJobs, item)
		case "verifier_job":
			verifierJobs = append(verifierJobs, item)
		}
	}

	// Render blocking challenges first (most urgent)
	if len(blockingChallenges) > 0 {
		sb.WriteString(fmt.Sprintf("--- Blocking Challenges (%d) ---\n", len(blockingChallenges)))
		for _, item := range blockingChallenges {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", item.NodeID, truncateStatement(item.Statement, 50)))
			sb.WriteString(fmt.Sprintf("    %s\n", item.Details))
		}
		sb.WriteString("\n")
	}

	// Render prover jobs
	if len(proverJobs) > 0 {
		sb.WriteString(fmt.Sprintf("--- Prover Jobs (%d) ---\n", len(proverJobs)))
		for _, item := range proverJobs {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", item.NodeID, truncateStatement(item.Statement, 50)))
		}
		sb.WriteString("\n")
	}

	// Render verifier jobs
	if len(verifierJobs) > 0 {
		sb.WriteString(fmt.Sprintf("--- Verifier Jobs (%d) ---\n", len(verifierJobs)))
		for _, item := range verifierJobs {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", item.NodeID, truncateStatement(item.Statement, 50)))
		}
		sb.WriteString("\n")
	}

	// Summary
	sb.WriteString("--- Summary ---\n")
	sb.WriteString(fmt.Sprintf("Total urgent items: %d\n", len(urgentItems)))
	if len(blockingChallenges) > 0 {
		sb.WriteString(fmt.Sprintf("  Blocking challenges: %d (resolve before accepting)\n", len(blockingChallenges)))
	}
	if len(proverJobs) > 0 {
		sb.WriteString(fmt.Sprintf("  Prover jobs: %d (claim and refine)\n", len(proverJobs)))
	}
	if len(verifierJobs) > 0 {
		sb.WriteString(fmt.Sprintf("  Verifier jobs: %d (review and accept/challenge)\n", len(verifierJobs)))
	}

	return sb.String()
}

// StatusOptions configures the status command rendering.
type StatusOptions struct {
	Limit   int
	Offset  int
	Focus   string // Node ID to focus on (show subtree only)
	Depth   int    // Max depth (0 = unlimited)
	Compact bool   // One line per node with challenge badges
}

// RenderStatusFiltered renders the proof status with navigation filtering.
func RenderStatusFiltered(s *state.State, opts StatusOptions) string {
	if s == nil {
		return "No proof state initialized."
	}

	nodes := s.AllNodes()
	if len(nodes) == 0 {
		return "No proof initialized. Run 'af init' to start a new proof."
	}

	// Apply focus filter: only include nodes in the focused subtree
	if opts.Focus != "" {
		focusID, _ := types.Parse(opts.Focus)
		var filtered []*node.Node
		for _, n := range nodes {
			if n.ID.Equal(focusID) || focusID.IsAncestorOf(n.ID) {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
		if len(nodes) == 0 {
			return fmt.Sprintf("Node %s not found.\n", opts.Focus)
		}
	}

	// Apply depth filter
	if opts.Depth > 0 {
		baseDepth := 0
		if opts.Focus != "" {
			focusID, _ := types.Parse(opts.Focus)
			baseDepth = focusID.Depth() - 1
		}
		var filtered []*node.Node
		for _, n := range nodes {
			if n.ID.Depth()-baseDepth <= opts.Depth {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}

	sortNodesByID(nodes)

	totalFiltered := len(nodes)
	paginatedNodes := applyPagination(nodes, opts.Limit, opts.Offset)

	var sb strings.Builder

	// Header
	if opts.Focus != "" {
		sb.WriteString(fmt.Sprintf("=== Proof Status (subtree: %s) ===\n\n", opts.Focus))
	} else {
		sb.WriteString("=== Proof Status ===\n\n")
	}

	// Tree view
	if opts.Compact {
		sb.WriteString(renderCompactTree(s, paginatedNodes))
	} else {
		treeOutput := RenderTreeForNodes(s, paginatedNodes)
		if treeOutput != "" {
			sb.WriteString(treeOutput)
		}
	}
	sb.WriteString("\n")

	// Statistics
	sb.WriteString("--- Statistics ---\n")
	renderStatisticsWithPagination(&sb, paginatedNodes, totalFiltered, opts.Limit, opts.Offset)
	sb.WriteString("\n")

	// Jobs
	sb.WriteString("--- Jobs ---\n")
	renderJobs(&sb, s, paginatedNodes)
	sb.WriteString("\n")

	// Legend (skip in compact mode to save space)
	if !opts.Compact {
		sb.WriteString("--- Legend ---\n")
		renderLegend(&sb)
	}

	return sb.String()
}

// renderCompactTree renders nodes in compact format: one line per node with challenge badges.
// Format: ID [epistemic/taint] statement_preview {Nc}
func renderCompactTree(s *state.State, nodes []*node.Node) string {
	var sb strings.Builder
	for _, n := range nodes {
		depth := n.ID.Depth()
		indent := strings.Repeat("  ", depth-1)
		sb.WriteString(indent)
		sb.WriteString(n.ID.String())
		sb.WriteString(" [")
		sb.WriteString(ColorEpistemicState(n.EpistemicState))
		sb.WriteString("] ")
		sb.WriteString(truncateStatement(n.Statement, 60))

		// Challenge badge
		challenges := s.GetChallengesForNode(n.ID)
		if len(challenges) > 0 {
			open := 0
			for _, c := range challenges {
				if c.Status == "open" {
					open++
				}
			}
			if open > 0 {
				sb.WriteString(fmt.Sprintf(" %s", Red(fmt.Sprintf("{%dc}", open))))
			}
		}

		sb.WriteString("\n")
	}
	return sb.String()
}

// truncateStatement truncates a statement to maxLen characters, adding "..." if needed.
func truncateStatement(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}
