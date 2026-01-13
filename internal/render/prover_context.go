// Package render provides human-readable formatting for AF framework types.
package render

import (
	"sort"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
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
		if len(stmt) > 50 {
			stmt = stmt[:47] + "..."
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
			if len(stmt) > 50 {
				stmt = stmt[:47] + "..."
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

// collectDefinitionNames extracts definition names from node context fields.
func collectDefinitionNames(s *state.State, targetNode *node.Node) []string {
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

	// Also try to find definitions by their Name field (not just ID)
	// by scanning all nodes and looking up by common patterns
	allNodes := s.AllNodes()
	for _, n := range allNodes {
		for _, entry := range n.Context {
			if strings.HasPrefix(entry, "def:") {
				name := strings.TrimPrefix(entry, "def:")
				def := s.GetDefinition(name)
				if def != nil && !nameSet[name] {
					names = append(names, name)
					nameSet[name] = true
				}
			}
		}
	}

	return names
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

// collectAssumptionIDs extracts assumption IDs from node context fields.
func collectAssumptionIDs(s *state.State, targetNode *node.Node) []string {
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

	// Also check all assumptions by scanning nodes - but we need the IDs
	// Since assumptions are added directly to state and we can't iterate,
	// we need to rely on context references or try common patterns

	// Convert map to slice
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	return ids
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
func collectExternalIDs(s *state.State, targetNode *node.Node) []string {
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
