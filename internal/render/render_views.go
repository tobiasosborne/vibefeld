// Package render provides human-readable formatting for AF framework types.
// This file contains render functions that work with view models.
// These functions have NO imports from domain packages (node, state, jobs, schema).
package render

import (
	"fmt"
	"sort"
	"strings"
)

// RenderNodeView renders a node view as a single-line human-readable summary.
// Format: [ID] type (state): "statement"
// Returns empty string for empty/zero-value view.
func RenderNodeView(v NodeView) string {
	if v.ID == "" {
		return ""
	}

	// Sanitize the statement but do NOT truncate
	stmt := sanitizeStatement(v.Statement)

	return fmt.Sprintf("[%s] %s (%s): %q",
		v.ID,
		v.Type,
		colorEpistemicStateString(v.EpistemicState),
		stmt,
	)
}

// RenderNodeViewVerbose renders a node view with all fields in multi-line format.
func RenderNodeViewVerbose(v NodeView) string {
	if v.ID == "" {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("ID:         %s\n", v.ID))
	sb.WriteString(fmt.Sprintf("Type:       %s\n", v.Type))
	sb.WriteString(fmt.Sprintf("Statement:  %s\n", v.Statement))
	sb.WriteString(fmt.Sprintf("Inference:  %s\n", v.Inference))
	sb.WriteString(fmt.Sprintf("Workflow:   %s\n", v.WorkflowState))
	sb.WriteString(fmt.Sprintf("Epistemic:  %s\n", colorEpistemicStateString(v.EpistemicState)))
	sb.WriteString(fmt.Sprintf("Taint:      %s\n", colorTaintStateString(v.TaintState)))
	sb.WriteString(fmt.Sprintf("Created:    %s\n", v.Created))
	sb.WriteString(fmt.Sprintf("Hash:       %s\n", v.ContentHash))

	if len(v.Context) > 0 {
		sb.WriteString(fmt.Sprintf("Context:    %s\n", strings.Join(v.Context, ", ")))
	}
	if len(v.Dependencies) > 0 {
		sb.WriteString(fmt.Sprintf("Depends on: %s\n", strings.Join(v.Dependencies, ", ")))
	}
	if len(v.ValidationDeps) > 0 {
		sb.WriteString(fmt.Sprintf("Requires validated: %s\n", strings.Join(v.ValidationDeps, ", ")))
	}
	if len(v.Scope) > 0 {
		sb.WriteString(fmt.Sprintf("Scope:      %s\n", strings.Join(v.Scope, ", ")))
	}
	if v.ClaimedBy != "" {
		sb.WriteString(fmt.Sprintf("Claimed by: %s\n", v.ClaimedBy))
	}

	return sb.String()
}

// RenderJobListView renders the list of available jobs from a view model.
func RenderJobListView(jl JobListView) string {
	if jl.IsEmpty() {
		return "No jobs available.\n\nProver jobs: 0 nodes awaiting refinement\nVerifier jobs: 0 nodes ready for review"
	}

	var sb strings.Builder

	// Sort prover jobs by ID
	proverJobs := make([]NodeView, len(jl.ProverJobs))
	copy(proverJobs, jl.ProverJobs)
	sortNodeViewsByID(proverJobs)

	// Sort verifier jobs by ID
	verifierJobs := make([]NodeView, len(jl.VerifierJobs))
	copy(verifierJobs, jl.VerifierJobs)
	sortNodeViewsByID(verifierJobs)

	// Render prover jobs section
	if len(proverJobs) > 0 {
		sb.WriteString(fmt.Sprintf("=== Prover Jobs (%d available) ===\n", len(proverJobs)))
		sb.WriteString("Nodes awaiting refinement. Claim one and refine the proof.\n\n")
		for _, n := range proverJobs {
			renderJobNodeView(&sb, n)
		}
		sb.WriteString("\nNext: Run 'af claim <id>' to claim a prover job, then 'af refine <id>' to work on it.\n")
	}

	if len(proverJobs) > 0 && len(verifierJobs) > 0 {
		sb.WriteString("\n")
	}

	// Render verifier jobs section
	if len(verifierJobs) > 0 {
		sb.WriteString(fmt.Sprintf("=== Verifier Jobs (%d available) ===\n", len(verifierJobs)))
		sb.WriteString("Nodes ready for review. Verify or challenge the proof.\n\n")
		for _, n := range verifierJobs {
			renderJobNodeView(&sb, n)
		}
		sb.WriteString("\nNext: Run 'af accept <id>' to validate or 'af challenge <id>' to raise objections.\n")
	}

	return sb.String()
}

// renderJobNodeView renders a single job node entry.
func renderJobNodeView(sb *strings.Builder, v NodeView) {
	stmt := sanitizeStatement(v.Statement)
	sb.WriteString(fmt.Sprintf("  [%s] %s: %q\n", v.ID, v.Type, stmt))

	if v.ClaimedBy != "" {
		sb.WriteString(fmt.Sprintf("         claimed by: %s\n", v.ClaimedBy))
	}
}

// RenderStatusView renders the full proof status from a view model.
func RenderStatusView(sv StatusView) string {
	if len(sv.Nodes) == 0 {
		return "No proof initialized. Run 'af init' to start a new proof."
	}

	var sb strings.Builder

	// 1. Header section
	sb.WriteString("=== Proof Status ===\n\n")

	// 2. Tree view section
	treeOutput := RenderTreeView(TreeView{Nodes: sv.Nodes, NodeLookup: buildNodeViewLookup(sv.Nodes)})
	if treeOutput != "" {
		sb.WriteString(treeOutput)
		sb.WriteString("\n")
	}

	// 3. Statistics section
	sb.WriteString("--- Statistics ---\n")
	renderStatisticsView(&sb, sv.Nodes)
	sb.WriteString("\n")

	// 4. Jobs section
	sb.WriteString("--- Jobs ---\n")
	sb.WriteString(fmt.Sprintf("  Prover: %d nodes awaiting refinement\n", sv.ProverJobCount))
	sb.WriteString(fmt.Sprintf("  Verifier: %d nodes ready for review\n", sv.VerifierJobCount))
	sb.WriteString("\n")

	// 5. Legend section
	sb.WriteString("--- Legend ---\n")
	renderLegendView(&sb)

	return sb.String()
}

// buildNodeViewLookup builds a lookup map from node views.
func buildNodeViewLookup(nodes []NodeView) map[string]NodeView {
	lookup := make(map[string]NodeView, len(nodes))
	for _, n := range nodes {
		lookup[n.ID] = n
	}
	return lookup
}

// renderStatisticsView writes the statistics section from node views.
func renderStatisticsView(sb *strings.Builder, nodes []NodeView) {
	total := len(nodes)

	// Count epistemic states
	epistemicCounts := make(map[string]int)
	for _, n := range nodes {
		epistemicCounts[n.EpistemicState]++
	}

	// Count taint states
	taintCounts := make(map[string]int)
	for _, n := range nodes {
		taintCounts[n.TaintState]++
	}

	// Write total count
	sb.WriteString(fmt.Sprintf("Nodes: %d total\n", total))

	// Write epistemic state counts
	sb.WriteString("  Epistemic: ")
	epistemicStates := []string{"pending", "validated", "admitted", "refuted", "archived"}
	epistemicParts := make([]string, len(epistemicStates))
	for i, state := range epistemicStates {
		epistemicParts[i] = fmt.Sprintf("%d %s", epistemicCounts[state], colorEpistemicStateString(state))
	}
	sb.WriteString(strings.Join(epistemicParts, ", "))
	sb.WriteString("\n")

	// Write taint state counts
	sb.WriteString("  Taint: ")
	taintStates := []string{"clean", "self_admitted", "tainted", "unresolved"}
	taintParts := make([]string, len(taintStates))
	for i, state := range taintStates {
		taintParts[i] = fmt.Sprintf("%d %s", taintCounts[state], colorTaintStateString(state))
	}
	sb.WriteString(strings.Join(taintParts, ", "))
	sb.WriteString("\n")
}

// renderLegendView writes the legend section.
func renderLegendView(sb *strings.Builder) {
	sb.WriteString("Epistemic States:\n")
	sb.WriteString(fmt.Sprintf("  %s    - Awaiting proof/verification\n", colorEpistemicStateString("pending")))
	sb.WriteString(fmt.Sprintf("  %s  - Verified by adversarial verifier\n", colorEpistemicStateString("validated")))
	sb.WriteString(fmt.Sprintf("  %s   - Accepted without full verification\n", colorEpistemicStateString("admitted")))
	sb.WriteString(fmt.Sprintf("  %s    - Proven false\n", colorEpistemicStateString("refuted")))
	sb.WriteString(fmt.Sprintf("  %s   - Superseded or abandoned\n", colorEpistemicStateString("archived")))
	sb.WriteString("\n")

	sb.WriteString("Taint States:\n")
	sb.WriteString(fmt.Sprintf("  %s         - No epistemic uncertainty\n", colorTaintStateString("clean")))
	sb.WriteString(fmt.Sprintf("  %s - Contains admitted node\n", colorTaintStateString("self_admitted")))
	sb.WriteString(fmt.Sprintf("  %s       - Depends on tainted/refuted node\n", colorTaintStateString("tainted")))
	sb.WriteString(fmt.Sprintf("  %s    - Taint status not yet computed\n", colorTaintStateString("unresolved")))
}

// RenderTreeView renders a proof tree from a view model.
func RenderTreeView(tv TreeView) string {
	if len(tv.Nodes) == 0 {
		return ""
	}

	// Determine the root node(s) to render
	var rootNodes []NodeView

	if tv.Root != nil {
		// Use the specific root
		rootNodes = []NodeView{*tv.Root}
	} else {
		// Find all top-level root nodes (depth 1)
		for _, n := range tv.Nodes {
			if n.Depth == 1 {
				rootNodes = append(rootNodes, n)
			}
		}
	}

	if len(rootNodes) == 0 {
		return ""
	}

	// Sort root nodes by ID
	sortNodeViewsByID(rootNodes)

	// Build the tree output
	var sb strings.Builder
	for i, root := range rootNodes {
		renderSubtreeView(&sb, root, tv.NodeLookup, tv.Nodes, "", i == len(rootNodes)-1, true, tv.Root)
	}

	return sb.String()
}

// renderSubtreeView recursively renders a node and its children from view models.
func renderSubtreeView(
	sb *strings.Builder,
	v NodeView,
	nodeLookup map[string]NodeView,
	allNodes []NodeView,
	prefix string,
	isLast bool,
	isRoot bool,
	customRoot *NodeView,
) {
	// Format node line
	nodeStr := formatNodeView(v, nodeLookup)

	if isRoot {
		sb.WriteString(nodeStr)
	} else {
		if isLast {
			sb.WriteString(prefix + treeLastNode + nodeStr)
		} else {
			sb.WriteString(prefix + treeBranch + nodeStr)
		}
	}
	sb.WriteString("\n")

	// Find children
	children := findChildrenView(v.ID, allNodes, customRoot)
	sortNodeViewsByID(children)

	// Calculate new prefix for children
	var childPrefix string
	if isRoot {
		childPrefix = ""
	} else {
		if isLast {
			childPrefix = prefix + treeSpace
		} else {
			childPrefix = prefix + treeVertical
		}
	}

	// Render children
	for i, child := range children {
		childIsLast := i == len(children)-1
		renderSubtreeView(sb, child, nodeLookup, allNodes, childPrefix, childIsLast, false, customRoot)
	}
}

// findChildrenView finds all direct children of a given node ID from views.
func findChildrenView(parentID string, allNodes []NodeView, customRoot *NodeView) []NodeView {
	var children []NodeView

	for _, n := range allNodes {
		// Check if this node is a direct child of the parent
		if pid, hasParent := GetNodeViewParentID(n); hasParent {
			if pid == parentID {
				// If customRoot is set, only include nodes that are descendants
				if customRoot != nil {
					if !isDescendantOrEqualView(n.ID, customRoot.ID) {
						continue
					}
				}
				children = append(children, n)
			}
		}
	}

	return children
}

// isDescendantOrEqualView returns true if nodeID is equal to or a descendant of ancestorID.
func isDescendantOrEqualView(nodeID, ancestorID string) bool {
	if nodeID == ancestorID {
		return true
	}
	return strings.HasPrefix(nodeID, ancestorID+".")
}

// formatNodeView formats a single node view for tree display.
func formatNodeView(v NodeView, nodeLookup map[string]NodeView) string {
	var sb strings.Builder

	sb.WriteString(v.ID)
	sb.WriteString(" ")

	// Status bracket [epistemic/taint]
	sb.WriteString("[")
	sb.WriteString(colorEpistemicStateString(v.EpistemicState))
	sb.WriteString("/")
	sb.WriteString(colorTaintStateString(v.TaintState))
	sb.WriteString("] ")

	// Statement (sanitized but NOT truncated)
	stmt := sanitizeStatement(v.Statement)
	sb.WriteString(stmt)

	// Show validation dependency status if present
	if len(v.ValidationDeps) > 0 {
		blockedCount := 0
		var blocked []string
		for _, depID := range v.ValidationDeps {
			if dep, ok := nodeLookup[depID]; ok {
				if dep.EpistemicState != "validated" && dep.EpistemicState != "admitted" {
					blockedCount++
					blocked = append(blocked, depID)
				}
			} else {
				// Dep not found - consider it blocked
				blockedCount++
				blocked = append(blocked, depID)
			}
		}
		if blockedCount > 0 {
			sb.WriteString(" ")
			sb.WriteString(Red("[BLOCKED: "))
			sb.WriteString(Red(strings.Join(blocked, ", ")))
			sb.WriteString(Red("]"))
		}
	}

	return sb.String()
}

// sortNodeViewsByID sorts node views by their hierarchical ID in numeric order.
func sortNodeViewsByID(nodes []NodeView) {
	sort.Slice(nodes, func(i, j int) bool {
		return compareNodeIDs(nodes[i].ID, nodes[j].ID)
	})
}

// RenderProverContextView renders prover context from a view model.
func RenderProverContextView(ctx ProverContextView) string {
	if ctx.Node.ID == "" {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString("=== Prover Context for Node ")
	sb.WriteString(ctx.Node.ID)
	sb.WriteString(" ===\n\n")

	// Node info section
	renderNodeInfoView(&sb, ctx.Node)

	// Parent context section
	renderParentContextView(&sb, ctx.Parent, ctx.Node.ID)

	// Siblings section
	renderSiblingsView(&sb, ctx.Siblings)

	// Dependencies section
	renderDependenciesView(&sb, ctx.Dependencies)

	// Scope section
	renderScopeView(&sb, ctx.Node.Scope)

	// Context references section
	renderContextRefsView(&sb, ctx.Node.Context)

	// Definitions section
	renderDefinitionsView(&sb, ctx.Definitions)

	// Assumptions section
	renderAssumptionsView(&sb, ctx.Assumptions)

	// Externals section
	renderExternalsView(&sb, ctx.Externals)

	// Challenges section
	renderChallengesView(&sb, ctx.Challenges)

	return sb.String()
}

// renderNodeInfoView writes the node information section from a view.
func renderNodeInfoView(sb *strings.Builder, v NodeView) {
	sb.WriteString("Node: ")
	sb.WriteString(v.ID)
	sb.WriteString(" [")
	sb.WriteString(v.Type)
	sb.WriteString("]\n")

	sb.WriteString("Statement: ")
	sb.WriteString(v.Statement)
	sb.WriteString("\n")

	sb.WriteString("Epistemic: ")
	sb.WriteString(v.EpistemicState)
	sb.WriteString("\n")

	sb.WriteString("Taint: ")
	sb.WriteString(v.TaintState)
	sb.WriteString("\n")

	sb.WriteString("Workflow: ")
	sb.WriteString(v.WorkflowState)
	if v.ClaimedBy != "" {
		sb.WriteString(" (claimed by: ")
		sb.WriteString(v.ClaimedBy)
		sb.WriteString(")")
	}
	sb.WriteString("\n")
}

// renderParentContextView writes the parent context section from a view.
func renderParentContextView(sb *strings.Builder, parent *NodeView, nodeID string) {
	if parent == nil {
		sb.WriteString("\nParent: (root node - no parent)\n")
		return
	}

	sb.WriteString("\nParent (")
	sb.WriteString(parent.ID)
	sb.WriteString("):\n")

	sb.WriteString("  Statement: ")
	sb.WriteString(parent.Statement)
	sb.WriteString("\n")
}

// renderSiblingsView writes sibling nodes from views.
func renderSiblingsView(sb *strings.Builder, siblings []NodeView) {
	if len(siblings) == 0 {
		return
	}

	// Sort for deterministic output
	sorted := make([]NodeView, len(siblings))
	copy(sorted, siblings)
	sortNodeViewsByID(sorted)

	sb.WriteString("\nSiblings:\n")
	for _, sib := range sorted {
		sb.WriteString("  ")
		sb.WriteString(sib.ID)
		sb.WriteString(": ")
		stmt := sanitizeStatement(sib.Statement)
		if len(stmt) > 50 {
			stmt = stmt[:47] + "..."
		}
		sb.WriteString(stmt)
		sb.WriteString("\n")
	}
}

// renderDependenciesView writes the dependencies section from views.
func renderDependenciesView(sb *strings.Builder, deps []NodeView) {
	sb.WriteString("\nDependencies: ")

	if len(deps) == 0 {
		sb.WriteString("(none)\n")
		return
	}

	sb.WriteString("\n")

	// Sort for deterministic output
	sorted := make([]NodeView, len(deps))
	copy(sorted, deps)
	sortNodeViewsByID(sorted)

	for _, dep := range sorted {
		sb.WriteString("  ")
		sb.WriteString(dep.ID)
		sb.WriteString(": ")
		stmt := sanitizeStatement(dep.Statement)
		if len(stmt) > 50 {
			stmt = stmt[:47] + "..."
		}
		sb.WriteString(stmt)
		sb.WriteString("\n")
	}
}

// renderScopeView writes the scope section.
func renderScopeView(sb *strings.Builder, scope []string) {
	if len(scope) == 0 {
		return
	}

	sb.WriteString("\nScope:\n")

	sorted := make([]string, len(scope))
	copy(sorted, scope)
	sort.Strings(sorted)

	for _, entry := range sorted {
		sb.WriteString("  - ")
		sb.WriteString(entry)
		sb.WriteString("\n")
	}
}

// renderContextRefsView writes the context references section.
func renderContextRefsView(sb *strings.Builder, context []string) {
	if len(context) == 0 {
		return
	}

	sb.WriteString("\nContext references:\n")

	sorted := make([]string, len(context))
	copy(sorted, context)
	sort.Strings(sorted)

	for _, entry := range sorted {
		sb.WriteString("  - ")
		sb.WriteString(entry)
		sb.WriteString("\n")
	}
}

// renderDefinitionsView writes the definitions section from views.
func renderDefinitionsView(sb *strings.Builder, defs []DefinitionView) {
	if len(defs) == 0 {
		sb.WriteString("\nDefinitions in scope: (none found)\n")
		return
	}

	sb.WriteString("\nDefinitions in scope:\n")

	// Sort by name for deterministic output
	sorted := make([]DefinitionView, len(defs))
	copy(sorted, defs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	for _, def := range sorted {
		sb.WriteString("  - ")
		sb.WriteString(def.Name)
		sb.WriteString(": ")
		content := def.Content
		if len(content) > 60 {
			content = content[:57] + "..."
		}
		sb.WriteString(content)
		sb.WriteString("\n")
	}
}

// renderAssumptionsView writes the assumptions section from views.
func renderAssumptionsView(sb *strings.Builder, assumptions []AssumptionView) {
	if len(assumptions) == 0 {
		sb.WriteString("\nAssumptions in scope: (none found)\n")
		return
	}

	sb.WriteString("\nAssumptions in scope:\n")

	// Sort by ID for deterministic output
	sorted := make([]AssumptionView, len(assumptions))
	copy(sorted, assumptions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	for _, assume := range sorted {
		sb.WriteString("  - ")
		stmt := assume.Statement
		if len(stmt) > 60 {
			stmt = stmt[:57] + "..."
		}
		sb.WriteString(stmt)
		if assume.Justification != "" {
			sb.WriteString(" (")
			sb.WriteString(assume.Justification)
			sb.WriteString(")")
		}
		sb.WriteString("\n")
	}
}

// renderExternalsView writes the externals section from views.
func renderExternalsView(sb *strings.Builder, externals []ExternalView) {
	if len(externals) == 0 {
		sb.WriteString("\nExternals in scope: (none found)\n")
		return
	}

	sb.WriteString("\nExternals in scope:\n")

	// Sort by ID for deterministic output
	sorted := make([]ExternalView, len(externals))
	copy(sorted, externals)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	for _, ext := range sorted {
		sb.WriteString("  - ")
		sb.WriteString(ext.Name)
		sb.WriteString(" (")
		sb.WriteString(ext.Source)
		sb.WriteString(")")
		if ext.Notes != "" {
			sb.WriteString(" - ")
			sb.WriteString(ext.Notes)
		}
		sb.WriteString("\n")
	}
}

// renderChallengesView writes the challenges section from views.
func renderChallengesView(sb *strings.Builder, challenges []ChallengeView) {
	if len(challenges) == 0 {
		sb.WriteString("\nChallenges: (none)\n")
		return
	}

	// Count open challenges
	openCount := 0
	for _, c := range challenges {
		if c.Status == ChallengeStatusOpen {
			openCount++
		}
	}

	sb.WriteString("\nChallenges (")
	sb.WriteString(fmt.Sprintf("%d total, %d open", len(challenges), openCount))
	sb.WriteString("):\n")

	// Sort by status (open first), then by ID
	sorted := make([]ChallengeView, len(challenges))
	copy(sorted, challenges)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Status == ChallengeStatusOpen && sorted[j].Status != ChallengeStatusOpen {
			return true
		}
		if sorted[i].Status != ChallengeStatusOpen && sorted[j].Status == ChallengeStatusOpen {
			return false
		}
		return sorted[i].ID < sorted[j].ID
	})

	for _, c := range sorted {
		sb.WriteString("  [")
		sb.WriteString(c.ID)
		sb.WriteString("] ")
		sb.WriteString("\"")
		sb.WriteString(c.Reason)
		sb.WriteString("\" (")
		sb.WriteString(c.Status)
		sb.WriteString(")\n")

		if c.Status == ChallengeStatusResolved && c.Resolution != "" {
			sb.WriteString("       Resolution: \"")
			sb.WriteString(c.Resolution)
			sb.WriteString("\"\n")
		}

		// Show actionable guidance for open challenges
		if c.Status == ChallengeStatusOpen {
			sb.WriteString("       -> Address with: af refine ")
			sb.WriteString(c.TargetID)
			sb.WriteString(" --children '[{\"statement\":\"...\",\"addresses_challenges\":[\"")
			sb.WriteString(c.ID)
			sb.WriteString("\"]}]'\n")
		}
	}

	if openCount > 0 {
		sb.WriteString("\n  Add child nodes with 'addresses_challenges' to respond to open challenges.\n")
		sb.WriteString("  Once addressed, the verifier can resolve them with 'af resolve-challenge'.\n")
	}
}

// RenderVerifierContextView renders verifier context from a view model.
func RenderVerifierContextView(ctx VerifierContextView) string {
	if ctx.Challenge.ID == "" {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString("=== Verifier Context for Challenge ")
	sb.WriteString(ctx.Challenge.ID)
	sb.WriteString(" ===\n\n")

	// Challenge info section
	renderChallengeInfoView(&sb, ctx.Challenge)

	// Challenged node section
	renderChallengedNodeView(&sb, ctx.Node)

	// Parent context section
	renderParentContextView(&sb, ctx.Parent, ctx.Node.ID)

	// Siblings section
	renderSiblingsView(&sb, ctx.Siblings)

	// Dependencies section
	renderDependenciesView(&sb, ctx.Dependencies)

	// Scope section
	renderScopeView(&sb, ctx.Node.Scope)

	// Context references section
	renderContextRefsView(&sb, ctx.Node.Context)

	// Definitions section
	renderDefinitionsView(&sb, ctx.Definitions)

	// Assumptions section
	renderAssumptionsView(&sb, ctx.Assumptions)

	// Externals section
	renderExternalsView(&sb, ctx.Externals)

	return sb.String()
}

// renderChallengeInfoView writes the challenge information section.
func renderChallengeInfoView(sb *strings.Builder, c ChallengeView) {
	sb.WriteString("Challenge: ")
	sb.WriteString(c.ID)
	sb.WriteString("\n")

	sb.WriteString("Target Node: ")
	sb.WriteString(c.TargetID)
	sb.WriteString("\n")

	sb.WriteString("Target Aspect: ")
	sb.WriteString(c.Target)
	if c.TargetDesc != "" {
		sb.WriteString(" (")
		sb.WriteString(c.TargetDesc)
		sb.WriteString(")")
	}
	sb.WriteString("\n")

	sb.WriteString("Reason: ")
	sb.WriteString(c.Reason)
	sb.WriteString("\n")

	sb.WriteString("Status: ")
	sb.WriteString(c.Status)
	sb.WriteString("\n")

	if c.Status == ChallengeStatusResolved && c.Resolution != "" {
		sb.WriteString("Resolution: ")
		sb.WriteString(c.Resolution)
		sb.WriteString("\n")
	}
}

// renderChallengedNodeView writes the challenged node information section.
func renderChallengedNodeView(sb *strings.Builder, v NodeView) {
	sb.WriteString("\nChallenged Node:\n")

	sb.WriteString("  ID: ")
	sb.WriteString(v.ID)
	sb.WriteString("\n")

	sb.WriteString("  Type: ")
	sb.WriteString(v.Type)
	sb.WriteString("\n")

	sb.WriteString("  Statement: ")
	sb.WriteString(v.Statement)
	sb.WriteString("\n")

	sb.WriteString("  Inference: ")
	sb.WriteString(v.Inference)
	sb.WriteString("\n")

	sb.WriteString("  Epistemic: ")
	sb.WriteString(v.EpistemicState)
	sb.WriteString("\n")

	sb.WriteString("  Workflow: ")
	sb.WriteString(v.WorkflowState)
	if v.ClaimedBy != "" {
		sb.WriteString(" (claimed by: ")
		sb.WriteString(v.ClaimedBy)
		sb.WriteString(")")
	}
	sb.WriteString("\n")

	sb.WriteString("  Taint: ")
	sb.WriteString(v.TaintState)
	sb.WriteString("\n")
}

// RenderSearchResultViews formats search results from view models.
func RenderSearchResultViews(results []SearchResultView) string {
	if len(results) == 0 {
		return "No matching nodes found.\n"
	}

	// Sort by node ID for consistent output
	sorted := make([]SearchResultView, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		return compareNodeIDs(sorted[i].Node.ID, sorted[j].Node.ID)
	})

	var sb strings.Builder
	sb.WriteString("Search Results:\n")
	sb.WriteString(strings.Repeat("-", 60))
	sb.WriteString("\n")

	for _, r := range sorted {
		if r.Node.ID == "" {
			continue
		}

		stmt := sanitizeStatement(r.Node.Statement)
		if len(stmt) > 60 {
			stmt = stmt[:57] + "..."
		}

		sb.WriteString("[")
		sb.WriteString(r.Node.ID)
		sb.WriteString("] (")
		sb.WriteString(colorEpistemicStateString(r.Node.EpistemicState))
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
	if len(results) == 1 {
		sb.WriteString("1 node")
	} else {
		sb.WriteString(formatInt(len(results)))
		sb.WriteString(" nodes")
	}
	sb.WriteString("\n")

	return sb.String()
}

// colorEpistemicStateString returns the epistemic state string with color coding.
func colorEpistemicStateString(state string) string {
	switch state {
	case "pending":
		return Yellow(state)
	case "validated":
		return Green(state)
	case "admitted":
		return Cyan(state)
	case "refuted":
		return Red(state)
	case "archived":
		return Gray(state)
	default:
		return state
	}
}

// colorTaintStateString returns the taint state string with color coding.
func colorTaintStateString(state string) string {
	switch state {
	case "clean":
		return Green(state)
	case "self_admitted":
		return Cyan(state)
	case "tainted":
		return Red(state)
	case "unresolved":
		return Yellow(state)
	default:
		return state
	}
}
