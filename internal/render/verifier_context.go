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

// RenderVerifierContext renders the context for a verifier examining a challenge.
// Shows the challenge details, the challenged node, and relevant context.
// Returns empty string for nil state or nil challenge.
func RenderVerifierContext(s *state.State, challenge *node.Challenge) string {
	// Handle nil inputs
	if s == nil || challenge == nil {
		return ""
	}

	// Get the challenged node
	challengedNode := s.GetNode(challenge.TargetID)
	if challengedNode == nil {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString("=== Verifier Context for Challenge ")
	sb.WriteString(challenge.ID)
	sb.WriteString(" ===\n\n")

	// Challenge info section
	renderChallengeInfo(&sb, challenge)

	// Challenged node section
	renderChallengedNode(&sb, challengedNode)

	// Parent context section
	renderVerifierParentContext(&sb, s, challenge.TargetID)

	// Siblings section
	renderVerifierSiblings(&sb, s, challenge.TargetID)

	// Dependencies section
	renderVerifierDependencies(&sb, s, challengedNode)

	// Scope section (especially relevant for scope challenges)
	renderVerifierScope(&sb, challengedNode)

	// Context references section
	renderVerifierContextRefs(&sb, challengedNode)

	// Definitions section
	renderVerifierDefinitions(&sb, s, challengedNode)

	// Assumptions section
	renderVerifierAssumptions(&sb, s, challengedNode)

	// Externals section
	renderVerifierExternals(&sb, s, challengedNode)

	return sb.String()
}

// renderChallengeInfo writes the challenge information section.
func renderChallengeInfo(sb *strings.Builder, c *node.Challenge) {
	sb.WriteString("Challenge: ")
	sb.WriteString(c.ID)
	sb.WriteString("\n")

	sb.WriteString("Target Node: ")
	sb.WriteString(c.TargetID.String())
	sb.WriteString("\n")

	sb.WriteString("Target Aspect: ")
	sb.WriteString(string(c.Target))
	// Add description for the challenge target
	if info, ok := schema.GetChallengeTargetInfo(c.Target); ok {
		sb.WriteString(" (")
		sb.WriteString(info.Description)
		sb.WriteString(")")
	}
	sb.WriteString("\n")

	sb.WriteString("Reason: ")
	sb.WriteString(c.Reason)
	sb.WriteString("\n")

	sb.WriteString("Status: ")
	sb.WriteString(string(c.Status))
	sb.WriteString("\n")

	// Show resolution for resolved challenges
	if c.Status == node.ChallengeStatusResolved && c.Resolution != "" {
		sb.WriteString("Resolution: ")
		sb.WriteString(c.Resolution)
		sb.WriteString("\n")
	}
}

// renderChallengedNode writes the challenged node information section.
func renderChallengedNode(sb *strings.Builder, n *node.Node) {
	sb.WriteString("\nChallenged Node:\n")

	sb.WriteString("  ID: ")
	sb.WriteString(n.ID.String())
	sb.WriteString("\n")

	sb.WriteString("  Type: ")
	sb.WriteString(string(n.Type))
	sb.WriteString("\n")

	sb.WriteString("  Statement: ")
	sb.WriteString(n.Statement)
	sb.WriteString("\n")

	sb.WriteString("  Inference: ")
	sb.WriteString(string(n.Inference))
	sb.WriteString("\n")

	sb.WriteString("  Epistemic: ")
	sb.WriteString(string(n.EpistemicState))
	sb.WriteString("\n")

	sb.WriteString("  Workflow: ")
	sb.WriteString(string(n.WorkflowState))
	if n.ClaimedBy != "" {
		sb.WriteString(" (claimed by: ")
		sb.WriteString(n.ClaimedBy)
		sb.WriteString(")")
	}
	sb.WriteString("\n")

	sb.WriteString("  Taint: ")
	sb.WriteString(string(n.TaintState))
	sb.WriteString("\n")
}

// renderVerifierParentContext writes the parent context section for non-root nodes.
func renderVerifierParentContext(sb *strings.Builder, s *state.State, nodeID types.NodeID) {
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

	sb.WriteString("  Type: ")
	sb.WriteString(string(parentNode.Type))
	sb.WriteString("\n")
}

// renderVerifierSiblings writes sibling nodes for context.
func renderVerifierSiblings(sb *strings.Builder, s *state.State, nodeID types.NodeID) {
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
		if len(stmt) > 50 {
			stmt = stmt[:47] + "..."
		}
		sb.WriteString(stmt)
		sb.WriteString("\n")
	}
}

// renderVerifierDependencies writes the dependencies section.
func renderVerifierDependencies(sb *strings.Builder, s *state.State, n *node.Node) {
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
			if len(stmt) > 50 {
				stmt = stmt[:47] + "..."
			}
			sb.WriteString(stmt)
		}
		sb.WriteString("\n")
	}
}

// renderVerifierScope writes the scope section if the node has scope entries.
func renderVerifierScope(sb *strings.Builder, n *node.Node) {
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

// renderVerifierContextRefs writes the context references section if the node has context entries.
func renderVerifierContextRefs(sb *strings.Builder, n *node.Node) {
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

// renderVerifierDefinitions writes the definitions section.
func renderVerifierDefinitions(sb *strings.Builder, s *state.State, n *node.Node) {
	// Collect definition names from context references
	defNames := collectVerifierDefinitionNames(s, n)

	if len(defNames) == 0 {
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
			if len(content) > 60 {
				content = content[:57] + "..."
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

// collectVerifierDefinitionNames extracts definition names from node context fields.
func collectVerifierDefinitionNames(s *state.State, targetNode *node.Node) []string {
	nameSet := make(map[string]bool)

	// Check target node's context
	for _, entry := range targetNode.Context {
		if strings.HasPrefix(entry, "def:") {
			name := strings.TrimPrefix(entry, "def:")
			nameSet[name] = true
		}
	}

	// Check target node's scope
	for _, entry := range targetNode.Scope {
		if strings.HasPrefix(entry, "def:") {
			name := strings.TrimPrefix(entry, "def:")
			nameSet[name] = true
		}
	}

	// Also check all definitions added to state by looking at all nodes' contexts
	for _, n := range s.AllNodes() {
		for _, entry := range n.Context {
			if strings.HasPrefix(entry, "def:") {
				name := strings.TrimPrefix(entry, "def:")
				// Only include if definition exists in state
				if s.GetDefinition(name) != nil {
					nameSet[name] = true
				}
			}
		}
	}

	// Convert map to slice
	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}

	return names
}

// renderVerifierAssumptions writes the assumptions section.
func renderVerifierAssumptions(sb *strings.Builder, s *state.State, n *node.Node) {
	// Collect assumption IDs from context references
	assumeIDs := collectVerifierAssumptionIDs(n)

	if len(assumeIDs) == 0 {
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
}

// collectVerifierAssumptionIDs extracts assumption IDs from node context fields.
func collectVerifierAssumptionIDs(targetNode *node.Node) []string {
	idSet := make(map[string]bool)

	// Check target node's context
	for _, entry := range targetNode.Context {
		if strings.HasPrefix(entry, "assume:") {
			id := strings.TrimPrefix(entry, "assume:")
			idSet[id] = true
		}
	}

	// Check target node's scope
	for _, entry := range targetNode.Scope {
		if strings.HasPrefix(entry, "assume:") {
			id := strings.TrimPrefix(entry, "assume:")
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

// renderVerifierExternals writes the externals section.
func renderVerifierExternals(sb *strings.Builder, s *state.State, n *node.Node) {
	// Collect external IDs from context references
	extIDs := collectVerifierExternalIDs(n)

	if len(extIDs) == 0 {
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

// collectVerifierExternalIDs extracts external IDs from node context fields.
func collectVerifierExternalIDs(targetNode *node.Node) []string {
	idSet := make(map[string]bool)

	// Check target node's context
	for _, entry := range targetNode.Context {
		if strings.HasPrefix(entry, "ext:") {
			id := strings.TrimPrefix(entry, "ext:")
			idSet[id] = true
		}
	}

	// Check target node's scope
	for _, entry := range targetNode.Scope {
		if strings.HasPrefix(entry, "ext:") {
			id := strings.TrimPrefix(entry, "ext:")
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
