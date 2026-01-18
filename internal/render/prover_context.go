// Package render provides human-readable formatting for AF framework types.
package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// Display truncation constants for consistent output formatting.
const (
	// maxStatementDisplay is the maximum length for statement text in summary views.
	maxStatementDisplay = 50
	// maxContentDisplay is the maximum length for definition/assumption content.
	maxContentDisplay = 60
)

// RenderProverContext renders the context for a prover working on a node.
// Shows: node info, parent context, dependencies, definitions, assumptions, externals.
// Returns empty string for nil state or node not found.
func RenderProverContext(s *state.State, nodeID types.NodeID) string {
	// Handle nil state
	if s == nil {
		return ""
	}

	// Get the target node
	n := s.GetNode(nodeID)
	if n == nil {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString("=== Prover Context for Node ")
	sb.WriteString(nodeID.String())
	sb.WriteString(" ===\n\n")

	// Node info section
	renderNodeInfo(&sb, n)

	// Parent context section (for non-root nodes)
	renderParentContext(&sb, s, nodeID)

	// Siblings section
	renderSiblings(&sb, s, nodeID)

	// Dependencies section
	renderDependencies(&sb, s, n)

	// Scope section (if present)
	renderScope(&sb, n)

	// Context references section (if present)
	renderContextRefs(&sb, n)

	// Definitions section (from all nodes since we can't iterate State.definitions directly)
	renderDefinitions(&sb, s, n)

	// Assumptions section
	renderAssumptions(&sb, s, n)

	// Externals section
	renderExternals(&sb, s, n)

	// Challenges section - critical for provers to know what to address
	renderChallenges(&sb, s, nodeID)

	return sb.String()
}

// renderNodeInfo writes the node information section.
func renderNodeInfo(sb *strings.Builder, n *node.Node) {
	sb.WriteString("Node: ")
	sb.WriteString(n.ID.String())
	sb.WriteString(" [")
	sb.WriteString(string(n.Type))
	sb.WriteString("]\n")

	sb.WriteString("Statement: ")
	sb.WriteString(n.Statement)
	sb.WriteString("\n")

	sb.WriteString("Epistemic: ")
	sb.WriteString(string(n.EpistemicState))
	sb.WriteString("\n")

	sb.WriteString("Taint: ")
	sb.WriteString(string(n.TaintState))
	sb.WriteString("\n")

	// Show workflow state and claimed by if relevant
	sb.WriteString("Workflow: ")
	sb.WriteString(string(n.WorkflowState))
	if n.ClaimedBy != "" {
		sb.WriteString(" (claimed by: ")
		sb.WriteString(n.ClaimedBy)
		sb.WriteString(")")
	}
	sb.WriteString("\n")
}

// renderParentContext writes the parent context section for non-root nodes.
func renderParentContext(sb *strings.Builder, s *state.State, nodeID types.NodeID) {
	parentID, hasParent := nodeID.Parent()
	if !hasParent {
		sb.WriteString("\nParent: (root node - no parent)\n")
		return
	}

	sb.WriteString("\nParent (")
	sb.WriteString(parentID.String())
	sb.WriteString("):\n")

	parentNode := s.GetNode(parentID)
	if parentNode == nil {
		sb.WriteString("  (parent node not found)\n")
		return
	}

	sb.WriteString("  Statement: ")
	sb.WriteString(parentNode.Statement)
	sb.WriteString("\n")
}

// renderSiblings writes sibling nodes for context.
func renderSiblings(sb *strings.Builder, s *state.State, nodeID types.NodeID) {
	parentID, hasParent := nodeID.Parent()
	if !hasParent {
		return // Root node has no siblings
	}

	parentStr := parentID.String()
	nodeStr := nodeID.String()

	// Collect siblings
	var siblings []*node.Node
	for _, n := range s.AllNodes() {
		p, ok := n.ID.Parent()
		if !ok {
			continue
		}
		if p.String() == parentStr && n.ID.String() != nodeStr {
			siblings = append(siblings, n)
		}
	}

	if len(siblings) == 0 {
		return
	}

	// Sort siblings by ID for deterministic output
	sort.Slice(siblings, func(i, j int) bool {
		return compareNodeIDs(siblings[i].ID.String(), siblings[j].ID.String())
	})

	sb.WriteString("\nSiblings:\n")
	for _, sib := range siblings {
		sb.WriteString("  ")
		sb.WriteString(sib.ID.String())
		sb.WriteString(": ")
		// Truncate statement for readability
		stmt := sanitizeStatement(sib.Statement)
		if len(stmt) > maxStatementDisplay {
			stmt = stmt[:maxStatementDisplay-3] + "..."
		}
		sb.WriteString(stmt)
		sb.WriteString("\n")
	}
}

// renderDependencies writes the dependencies section.
func renderDependencies(sb *strings.Builder, s *state.State, n *node.Node) {
	sb.WriteString("\nDependencies: ")

	if len(n.Dependencies) == 0 {
		sb.WriteString("(none)\n")
		return
	}

	sb.WriteString("\n")

	// Sort dependencies for deterministic output
	deps := make([]types.NodeID, len(n.Dependencies))
	copy(deps, n.Dependencies)
	sort.Slice(deps, func(i, j int) bool {
		return compareNodeIDs(deps[i].String(), deps[j].String())
	})

	for _, depID := range deps {
		sb.WriteString("  ")
		sb.WriteString(depID.String())

		// Try to get the dependency node for more context
		depNode := s.GetNode(depID)
		if depNode != nil {
			sb.WriteString(": ")
			stmt := sanitizeStatement(depNode.Statement)
			if len(stmt) > maxStatementDisplay {
				stmt = stmt[:maxStatementDisplay-3] + "..."
			}
			sb.WriteString(stmt)
		}
		sb.WriteString("\n")
	}
}

// renderScope writes the scope section if the node has scope entries.
func renderScope(sb *strings.Builder, n *node.Node) {
	if len(n.Scope) == 0 {
		return
	}

	sb.WriteString("\nScope:\n")

	// Sort scope entries for deterministic output
	scopeEntries := make([]string, len(n.Scope))
	copy(scopeEntries, n.Scope)
	sort.Strings(scopeEntries)

	for _, entry := range scopeEntries {
		sb.WriteString("  - ")
		sb.WriteString(entry)
		sb.WriteString("\n")
	}
}

// renderContextRefs writes the context references section if the node has context entries.
func renderContextRefs(sb *strings.Builder, n *node.Node) {
	if len(n.Context) == 0 {
		return
	}

	sb.WriteString("\nContext references:\n")

	// Sort context entries for deterministic output
	contextEntries := make([]string, len(n.Context))
	copy(contextEntries, n.Context)
	sort.Strings(contextEntries)

	for _, entry := range contextEntries {
		sb.WriteString("  - ")
		sb.WriteString(entry)
		sb.WriteString("\n")
	}
}

// renderDefinitions writes the definitions section.
// Since State doesn't expose an iterator for definitions, we look them up via
// the node's context field (entries like "def:name") and also check all nodes
// for context references.
func renderDefinitions(sb *strings.Builder, s *state.State, n *node.Node) {
	// Collect definition names from context references
	defNames := collectDefinitionNames(s, n)

	if len(defNames) == 0 {
		sb.WriteString("\nDefinitions in scope: (none found)\n")
		return
	}

	sb.WriteString("\nDefinitions in scope:\n")

	// Sort for deterministic output
	sort.Strings(defNames)

	for _, name := range defNames {
		def := s.GetDefinition(name)
		if def != nil {
			sb.WriteString("  - ")
			sb.WriteString(def.Name)
			sb.WriteString(": ")
			content := def.Content
			if len(content) > maxContentDisplay {
				content = content[:maxContentDisplay-3] + "..."
			}
			sb.WriteString(content)
			sb.WriteString("\n")
		} else {
			// Definition name found in context but not in state - show name anyway
			sb.WriteString("  - ")
			sb.WriteString(name)
			sb.WriteString(" (definition not found)\n")
		}
	}
}

// collectDefinitionNames extracts definition names from node context fields.
//
// This function uses two collection passes for distinct purposes:
// 1. First pass (collectContextEntries): Collects ALL definition references from the
//    target node's context/scope, including references to definitions not yet in state.
//    This ensures "definition not found" can be displayed for missing definitions.
// 2. Second pass (addDefinitionNamesFromNode): Iterates all nodes to find definitions
//    that exist in state, ensuring we show all relevant definitions from the proof.
//
// The target node may be processed in both passes, but this is intentional:
// the first pass collects references unconditionally, while the second only adds
// definitions that actually exist in state.
func collectDefinitionNames(s *state.State, targetNode *node.Node) []string {
	// Pass 1: Get all definition references from target node (including missing ones)
	names := collectContextEntries("def:", targetNode)
	nameSet := make(map[string]bool, len(names))
	for _, name := range names {
		nameSet[name] = true
	}

	// Pass 2: Add definitions that exist in state from any node's context
	for _, n := range s.AllNodes() {
		addDefinitionNamesFromNode(s, n, nameSet)
	}

	// Convert map to slice
	result := make([]string, 0, len(nameSet))
	for name := range nameSet {
		result = append(result, name)
	}

	return result
}

// addDefinitionNamesFromNode extracts definition names from a single node's context
// and adds them to nameSet if the definition exists in state.
func addDefinitionNamesFromNode(s *state.State, n *node.Node, nameSet map[string]bool) {
	for _, entry := range n.Context {
		if !strings.HasPrefix(entry, "def:") {
			continue
		}
		name := strings.TrimPrefix(entry, "def:")
		if s.GetDefinition(name) == nil {
			continue
		}
		nameSet[name] = true
	}
}

// renderAssumptions writes the assumptions section.
func renderAssumptions(sb *strings.Builder, s *state.State, n *node.Node) {
	// Collect assumption IDs from context references
	assumeIDs := collectAssumptionIDs(s, n)

	if len(assumeIDs) == 0 {
		sb.WriteString("\nAssumptions in scope: (none found)\n")
		return
	}

	sb.WriteString("\nAssumptions in scope:\n")

	// Sort for deterministic output
	sort.Strings(assumeIDs)

	for _, id := range assumeIDs {
		assume := s.GetAssumption(id)
		if assume != nil {
			sb.WriteString("  - ")
			stmt := assume.Statement
			if len(stmt) > maxContentDisplay {
				stmt = stmt[:maxContentDisplay-3] + "..."
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
}

// collectContextEntries extracts IDs from node context and scope fields with a given prefix.
// For example, prefix "assume:" extracts assumption IDs, prefix "ext:" extracts external IDs.
func collectContextEntries(prefix string, targetNode *node.Node) []string {
	idSet := make(map[string]bool)

	// Check target node's context
	for _, entry := range targetNode.Context {
		if strings.HasPrefix(entry, prefix) {
			id := strings.TrimPrefix(entry, prefix)
			idSet[id] = true
		}
	}

	// Check target node's scope
	for _, entry := range targetNode.Scope {
		if strings.HasPrefix(entry, prefix) {
			id := strings.TrimPrefix(entry, prefix)
			idSet[id] = true
		}
	}

	// Convert map to slice
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	return ids
}

// collectAssumptionIDs extracts assumption IDs from node context fields.
func collectAssumptionIDs(_ *state.State, targetNode *node.Node) []string {
	return collectContextEntries("assume:", targetNode)
}

// renderExternals writes the externals section.
func renderExternals(sb *strings.Builder, s *state.State, n *node.Node) {
	// Collect external IDs from context references
	extIDs := collectExternalIDs(s, n)

	if len(extIDs) == 0 {
		sb.WriteString("\nExternals in scope: (none found)\n")
		return
	}

	sb.WriteString("\nExternals in scope:\n")

	// Sort for deterministic output
	sort.Strings(extIDs)

	for _, id := range extIDs {
		ext := s.GetExternal(id)
		if ext != nil {
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
}

// collectExternalIDs extracts external IDs from node context fields.
func collectExternalIDs(_ *state.State, targetNode *node.Node) []string {
	return collectContextEntries("ext:", targetNode)
}

// isBlockingSeverity returns true if the severity blocks node acceptance.
// Critical and major severities are blocking; minor and note are non-blocking.
func isBlockingSeverity(severity string) bool {
	return severity == "critical" || severity == "major"
}

// severityOrder returns a numeric order for sorting: critical=0, major=1, minor=2, note=3, unknown=4.
func severityOrder(severity string) int {
	switch severity {
	case "critical":
		return 0
	case "major":
		return 1
	case "minor":
		return 2
	case "note":
		return 3
	default:
		return 4
	}
}

// renderChallenges writes the challenges section for a node.
// This is critical for provers to understand what issues need to be addressed.
func renderChallenges(sb *strings.Builder, s *state.State, nodeID types.NodeID) {
	// Get all challenges and filter for this node
	allChallenges := s.AllChallenges()
	nodeIDStr := nodeID.String()

	var nodeChallenges []*state.Challenge
	for _, c := range allChallenges {
		if c.NodeID.String() == nodeIDStr {
			nodeChallenges = append(nodeChallenges, c)
		}
	}

	if len(nodeChallenges) == 0 {
		sb.WriteString("\nChallenges: (none)\n")
		return
	}

	// Count open and blocking challenges
	openCount := 0
	blockingCount := 0
	for _, c := range nodeChallenges {
		if c.Status == state.ChallengeStatusOpen {
			openCount++
			if isBlockingSeverity(c.Severity) {
				blockingCount++
			}
		}
	}

	sb.WriteString("\nChallenges (")
	sb.WriteString(fmt.Sprintf("%d total, %d open", len(nodeChallenges), openCount))
	if blockingCount > 0 {
		sb.WriteString(fmt.Sprintf(", %d blocking", blockingCount))
	}
	sb.WriteString("):\n")

	// Sort by: status (open first), then severity (critical > major > minor > note), then ID
	sort.Slice(nodeChallenges, func(i, j int) bool {
		// Open challenges come first
		if nodeChallenges[i].Status == state.ChallengeStatusOpen && nodeChallenges[j].Status != state.ChallengeStatusOpen {
			return true
		}
		if nodeChallenges[i].Status != state.ChallengeStatusOpen && nodeChallenges[j].Status == state.ChallengeStatusOpen {
			return false
		}
		// Within same status, sort by severity (more severe first)
		sevI := severityOrder(nodeChallenges[i].Severity)
		sevJ := severityOrder(nodeChallenges[j].Severity)
		if sevI != sevJ {
			return sevI < sevJ
		}
		return nodeChallenges[i].ID < nodeChallenges[j].ID
	})

	for _, c := range nodeChallenges {
		// Format: [ID] severity (BLOCKING) "reason" (status)
		sb.WriteString("  [")
		sb.WriteString(c.ID)
		sb.WriteString("] ")

		// Show severity if set
		if c.Severity != "" {
			sb.WriteString(c.Severity)
			// Mark blocking challenges clearly
			if c.Status == state.ChallengeStatusOpen && isBlockingSeverity(c.Severity) {
				sb.WriteString(" (BLOCKING)")
			}
			sb.WriteString(" - ")
		}

		sb.WriteString("\"")
		sb.WriteString(c.Reason)
		sb.WriteString("\" (")
		sb.WriteString(c.Status)
		sb.WriteString(")\n")

		// Show resolution text for resolved challenges
		if c.Status == state.ChallengeStatusResolved && c.Resolution != "" {
			sb.WriteString("       Resolution: \"")
			sb.WriteString(c.Resolution)
			sb.WriteString("\"\n")
		}

		// Show actionable guidance for open challenges
		if c.Status == state.ChallengeStatusOpen {
			sb.WriteString("       -> Address with: af refine ")
			sb.WriteString(c.NodeID.String())
			sb.WriteString(" --children '[{\"statement\":\"...\",\"addresses_challenges\":[\"")
			sb.WriteString(c.ID)
			sb.WriteString("\"]}]'\n")
		}
	}

	// Add summary guidance for open challenges
	if openCount > 0 {
		if blockingCount > 0 {
			sb.WriteString("\n  ⚠ Blocking challenges (critical/major) must be resolved before acceptance.\n")
		}
		sb.WriteString("  Add child nodes with 'addresses_challenges' to respond to open challenges.\n")
		sb.WriteString("  Once addressed, the verifier can resolve them with 'af resolve-challenge'.\n")
	}
}
