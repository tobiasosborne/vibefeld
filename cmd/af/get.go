package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// newGetCmd creates the get command for retrieving node information.
func newGetCmd() *cobra.Command {
	var dir string
	var format string
	var ancestors bool
	var subtree bool
	var full bool
	var checklist bool

	cmd := &cobra.Command{
		Use:   "get <node-id>",
		Short: "Get node details by ID",
		Long: `Get detailed information about a proof node.

Retrieves node information from the proof, with optional flags to show
ancestors, subtree, or full details.

Examples:
  af get 1                    Show node 1
  af get 1.2 --ancestors      Show node 1.2 and its ancestor chain
  af get 1 --subtree          Show node 1 and all its descendants
  af get 1 --full             Show full node details
  af get 1.1 -a -F            Show ancestors with full details
  af get 1 -s -f json         Show subtree in JSON format
  af get 1.1 --checklist      Show verification checklist for node 1.1
  af get 1.1 --checklist -f json  Show verification checklist in JSON`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd, args[0], dir, format, ancestors, subtree, full, checklist)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text/json)")
	cmd.Flags().BoolVarP(&ancestors, "ancestors", "a", false, "Show ancestor chain")
	cmd.Flags().BoolVarP(&subtree, "subtree", "s", false, "Show subtree (all descendants)")
	cmd.Flags().BoolVarP(&full, "full", "F", false, "Show full node details")
	cmd.Flags().BoolVarP(&checklist, "checklist", "c", false, "Show verification checklist for the node")

	return cmd
}

func runGet(cmd *cobra.Command, nodeIDStr, dir, format string, ancestors, subtree, full, checklist bool) error {
	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Parse node ID
	nodeID, err := types.Parse(nodeIDStr)
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %v", nodeIDStr, err)
	}

	// Create service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Check if proof is initialized
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Load state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Get the target node
	targetNode := st.GetNode(nodeID)
	if targetNode == nil {
		return fmt.Errorf("node %q does not exist", nodeIDStr)
	}

	// Handle checklist flag - show verification checklist instead of normal output
	if checklist {
		if format == "json" {
			cmd.Println(render.RenderVerificationChecklistJSON(targetNode, st))
		} else {
			cmd.Print(render.RenderVerificationChecklist(targetNode, st))
		}
		return nil
	}

	// Collect nodes to display based on flags
	var nodes []*node.Node

	if ancestors {
		// Collect ancestor chain (including the target node)
		nodes = collectAncestors(st, nodeID)
	} else if subtree {
		// Collect subtree (including the target node)
		nodes = collectSubtree(st, nodeID)
	} else {
		// Just the single node
		nodes = []*node.Node{targetNode}
	}

	// Sort nodes by ID for consistent output
	sort.Slice(nodes, func(i, j int) bool {
		return compareNodeIDs(nodes[i].ID.String(), nodes[j].ID.String())
	})

	// Get all challenges from state for lookup
	allChallenges := st.AllChallenges()

	// Output based on format
	if format == "json" {
		return outputJSON(cmd, nodes, full, allChallenges, st)
	}

	return outputText(cmd, nodes, full, allChallenges, st)
}

// collectAncestors collects the target node and all its ancestors.
func collectAncestors(st interface{ GetNode(types.NodeID) *node.Node }, nodeID types.NodeID) []*node.Node {
	var nodes []*node.Node

	// Start with the target node
	currentID := nodeID
	for {
		n := st.GetNode(currentID)
		if n != nil {
			nodes = append(nodes, n)
		}

		// Get parent
		parentID, hasParent := currentID.Parent()
		if !hasParent {
			break
		}
		currentID = parentID
	}

	return nodes
}

// collectSubtree collects the target node and all its descendants.
func collectSubtree(st interface {
	GetNode(types.NodeID) *node.Node
	AllNodes() []*node.Node
}, nodeID types.NodeID) []*node.Node {
	var nodes []*node.Node

	// Get all nodes from state
	allNodes := st.AllNodes()
	targetStr := nodeID.String()

	for _, n := range allNodes {
		nStr := n.ID.String()
		// Include if it's the target node or a descendant
		if nStr == targetStr || nodeID.IsAncestorOf(n.ID) {
			nodes = append(nodes, n)
		}
	}

	return nodes
}

// outputJSON outputs nodes in JSON format.
func outputJSON(cmd *cobra.Command, nodes []*node.Node, full bool, challenges []*state.Challenge, st *state.State) error {
	if len(nodes) == 1 {
		// Single node: always show full output by default.
		// The --full flag is a no-op for single nodes (kept for backwards compatibility).
		nodeChallenges := filterChallengesForNode(challenges, nodes[0].ID)
		amendments := st.GetAmendmentHistory(nodes[0].ID)
		scopeInfo := getScopeInfoJSON(st, nodes[0].ID)
		output := nodeToJSONFull(nodes[0], nodeChallenges, amendments, scopeInfo)
		data, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		cmd.Println(string(data))
		return nil
	}

	// Multiple nodes
	if full {
		jsonNodes := make([]map[string]interface{}, 0, len(nodes))
		for _, n := range nodes {
			nodeChallenges := filterChallengesForNode(challenges, n.ID)
			amendments := st.GetAmendmentHistory(n.ID)
			scopeInfo := getScopeInfoJSON(st, n.ID)
			jsonNodes = append(jsonNodes, nodeToJSONFull(n, nodeChallenges, amendments, scopeInfo))
		}
		data, err := json.Marshal(jsonNodes)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		cmd.Println(string(data))
	} else {
		jsonNodes := make([]map[string]interface{}, 0, len(nodes))
		for _, n := range nodes {
			jsonNodes = append(jsonNodes, nodeToJSONBasic(n))
		}
		data, err := json.Marshal(jsonNodes)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		cmd.Println(string(data))
	}

	return nil
}

// getScopeInfoJSON returns scope information for a node as a JSON-friendly map.
// Returns nil if the node is not in any scope.
func getScopeInfoJSON(st *state.State, nodeID types.NodeID) map[string]interface{} {
	info := st.GetScopeInfo(nodeID)
	if info == nil || !info.IsInAnyScope() {
		return nil
	}

	result := map[string]interface{}{
		"depth": info.Depth,
	}

	if len(info.ContainingScopes) > 0 {
		scopes := make([]map[string]interface{}, len(info.ContainingScopes))
		for i, s := range info.ContainingScopes {
			scopes[i] = map[string]interface{}{
				"node_id":   s.NodeID.String(),
				"statement": s.Statement,
				"active":    s.IsActive(),
			}
		}
		result["containing_scopes"] = scopes
	}

	return result
}

// nodeToJSONBasic creates a basic JSON representation of a node.
func nodeToJSONBasic(n *node.Node) map[string]interface{} {
	return map[string]interface{}{
		"id":        n.ID.String(),
		"statement": n.Statement,
	}
}

// nodeToJSONFull creates a full JSON representation of a node.
func nodeToJSONFull(n *node.Node, challenges []*state.Challenge, amendments []state.Amendment, scopeInfo map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"id":              n.ID.String(),
		"type":            string(n.Type),
		"statement":       n.Statement,
		"inference":       string(n.Inference),
		"workflow_state":  string(n.WorkflowState),
		"epistemic_state": string(n.EpistemicState),
		"taint_state":     string(n.TaintState),
		"created":         n.Created.String(),
		"content_hash":    n.ContentHash,
	}

	if len(n.Context) > 0 {
		result["context"] = n.Context
	}

	if len(n.Dependencies) > 0 {
		deps := make([]string, len(n.Dependencies))
		for i, dep := range n.Dependencies {
			deps[i] = dep.String()
		}
		result["dependencies"] = deps
	}

	if len(n.ValidationDeps) > 0 {
		deps := make([]string, len(n.ValidationDeps))
		for i, dep := range n.ValidationDeps {
			deps[i] = dep.String()
		}
		result["validation_deps"] = deps
	}

	if len(n.Scope) > 0 {
		result["scope"] = n.Scope
	}

	if n.ClaimedBy != "" {
		result["claimed_by"] = n.ClaimedBy
	}

	// Add challenges if present
	if len(challenges) > 0 {
		challengeList := make([]map[string]interface{}, len(challenges))
		for i, c := range challenges {
			challengeList[i] = map[string]interface{}{
				"id":     c.ID,
				"status": c.Status,
				"target": c.Target,
				"reason": c.Reason,
			}
		}
		result["challenges"] = challengeList
	}

	// Add amendment history if present
	if len(amendments) > 0 {
		amendmentList := make([]map[string]interface{}, len(amendments))
		for i, a := range amendments {
			amendmentList[i] = map[string]interface{}{
				"timestamp":          a.Timestamp.String(),
				"previous_statement": a.PreviousStatement,
				"new_statement":      a.NewStatement,
				"owner":              a.Owner,
			}
		}
		result["amendment_history"] = amendmentList
	}

	// Add scope info if present
	if scopeInfo != nil && len(scopeInfo) > 0 {
		result["scope_info"] = scopeInfo
	}

	return result
}

// outputText outputs nodes in text format.
func outputText(cmd *cobra.Command, nodes []*node.Node, full bool, challenges []*state.Challenge, st *state.State) error {
	if len(nodes) == 1 {
		// Single node: always show full/verbose output by default.
		// The --full flag is a no-op for single nodes (kept for backwards compatibility).
		cmd.Print(render.RenderNodeVerbose(nodes[0]))
		// Show challenges for this node
		nodeChallenges := filterChallengesForNode(challenges, nodes[0].ID)
		if len(nodeChallenges) > 0 {
			cmd.Printf("\nChallenges (%d):\n", len(nodeChallenges))
			for _, c := range nodeChallenges {
				cmd.Printf("  %s [%s] %s: %s\n", c.ID, c.Status, c.Target, c.Reason)
			}
		}
		// Show amendment history for this node
		amendments := st.GetAmendmentHistory(nodes[0].ID)
		if len(amendments) > 0 {
			cmd.Printf("\nAmendment History (%d):\n", len(amendments))
			for i, a := range amendments {
				cmd.Printf("  [%d] %s by %s\n", i+1, a.Timestamp.String(), a.Owner)
				cmd.Printf("      Previous: %s\n", truncateForDisplay(a.PreviousStatement, 50))
				cmd.Printf("      New:      %s\n", truncateForDisplay(a.NewStatement, 50))
			}
		}
		// Show scope information
		scopeInfo := st.GetScopeInfo(nodes[0].ID)
		if scopeInfo != nil && scopeInfo.IsInAnyScope() {
			cmd.Printf("\nScope Info:\n")
			cmd.Printf("  Depth: %d\n", scopeInfo.Depth)
			cmd.Println("  Containing scopes:")
			for _, s := range scopeInfo.ContainingScopes {
				status := "active"
				if !s.IsActive() {
					status = "closed"
				}
				cmd.Printf("    [%s] %s: %q\n", s.NodeID.String(), status, s.Statement)
			}
		}
		return nil
	}

	// Multiple nodes
	if full {
		for i, n := range nodes {
			if i > 0 {
				cmd.Println("---")
			}
			cmd.Print(render.RenderNodeVerbose(n))
			// Show challenges for this node
			nodeChallenges := filterChallengesForNode(challenges, n.ID)
			if len(nodeChallenges) > 0 {
				cmd.Printf("\nChallenges (%d):\n", len(nodeChallenges))
				for _, c := range nodeChallenges {
					cmd.Printf("  %s [%s] %s: %s\n", c.ID, c.Status, c.Target, c.Reason)
				}
			}
			// Show amendment history for this node
			amendments := st.GetAmendmentHistory(n.ID)
			if len(amendments) > 0 {
				cmd.Printf("\nAmendment History (%d):\n", len(amendments))
				for j, a := range amendments {
					cmd.Printf("  [%d] %s by %s\n", j+1, a.Timestamp.String(), a.Owner)
					cmd.Printf("      Previous: %s\n", truncateForDisplay(a.PreviousStatement, 50))
					cmd.Printf("      New:      %s\n", truncateForDisplay(a.NewStatement, 50))
				}
			}
			// Show scope information
			scopeInfo := st.GetScopeInfo(n.ID)
			if scopeInfo != nil && scopeInfo.IsInAnyScope() {
				cmd.Printf("\nScope Info:\n")
				cmd.Printf("  Depth: %d\n", scopeInfo.Depth)
				cmd.Println("  Containing scopes:")
				for _, s := range scopeInfo.ContainingScopes {
					status := "active"
					if !s.IsActive() {
						status = "closed"
					}
					cmd.Printf("    [%s] %s: %q\n", s.NodeID.String(), status, s.Statement)
				}
			}
		}
	} else {
		cmd.Println(render.RenderNodeTree(nodes))
	}

	return nil
}

// truncateForDisplay truncates a string for display, adding "..." if truncated.
func truncateForDisplay(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// filterChallengesForNode returns challenges that target the given node.
func filterChallengesForNode(challenges []*state.Challenge, nodeID types.NodeID) []*state.Challenge {
	var result []*state.Challenge
	for _, c := range challenges {
		if c.NodeID.String() == nodeID.String() {
			result = append(result, c)
		}
	}
	return result
}

// compareNodeIDs compares two node ID strings for sorting.
// Returns true if a should come before b.
func compareNodeIDs(a, b string) bool {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	minLen := len(partsA)
	if len(partsB) < minLen {
		minLen = len(partsB)
	}

	for i := 0; i < minLen; i++ {
		// Parse as integers for numeric comparison
		var numA, numB int
		fmt.Sscanf(partsA[i], "%d", &numA)
		fmt.Sscanf(partsB[i], "%d", &numB)

		if numA != numB {
			return numA < numB
		}
	}

	// If all common parts are equal, shorter ID comes first
	return len(partsA) < len(partsB)
}

func init() {
	rootCmd.AddCommand(newGetCmd())
}
