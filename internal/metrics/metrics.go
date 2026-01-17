// Package metrics provides quality metrics for proofs in the AF system.
// It calculates various metrics like refinement depth, challenge density,
// and definition coverage to help assess proof quality.
package metrics

import (
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// QualityReport contains comprehensive quality metrics for a proof or subtree.
type QualityReport struct {
	// Node counts
	NodeCount      int `json:"node_count"`
	ValidatedNodes int `json:"validated_nodes"`
	PendingNodes   int `json:"pending_nodes"`
	AdmittedNodes  int `json:"admitted_nodes"`
	RefutedNodes   int `json:"refuted_nodes"`
	ArchivedNodes  int `json:"archived_nodes"`

	// Depth metrics
	MaxDepth int `json:"max_depth"`

	// Challenge metrics
	TotalChallenges    int     `json:"total_challenges"`
	OpenChallenges     int     `json:"open_challenges"`
	ResolvedChallenges int     `json:"resolved_challenges"`
	ChallengeDensity   float64 `json:"challenge_density"`

	// Definition coverage
	DefinitionRefs     int     `json:"definition_refs"`
	DefinedRefs        int     `json:"defined_refs"`
	DefinitionCoverage float64 `json:"definition_coverage"`

	// Composite score (0-100)
	QualityScore float64 `json:"quality_score"`
}

// RefinementDepth calculates the maximum depth of the refinement tree
// starting from a given node. Returns 0 if the node doesn't exist.
func RefinementDepth(st *state.State, rootID types.NodeID) int {
	rootNode := st.GetNode(rootID)
	if rootNode == nil {
		return 0
	}

	return maxDepthFromNode(st, rootID)
}

// maxDepthFromNode recursively finds the maximum depth from a node.
func maxDepthFromNode(st *state.State, nodeID types.NodeID) int {
	// Start with depth 1 for the current node
	maxDepth := 1

	// Find all direct children
	nodeIDStr := nodeID.String()
	for _, n := range st.AllNodes() {
		parent, hasParent := n.ID.Parent()
		if hasParent && parent.String() == nodeIDStr {
			// Found a child, recurse
			childDepth := maxDepthFromNode(st, n.ID)
			if childDepth+1 > maxDepth {
				maxDepth = childDepth + 1
			}
		}
	}

	return maxDepth
}

// MaxRefinementDepth returns the maximum depth across the entire proof tree.
// This is equivalent to finding the deepest node in the proof.
func MaxRefinementDepth(st *state.State) int {
	nodes := st.AllNodes()
	if len(nodes) == 0 {
		return 0
	}

	maxDepth := 0
	for _, n := range nodes {
		depth := n.ID.Depth()
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth
}

// ChallengeDensity calculates the number of challenges per node.
// Returns 0 if there are no nodes.
func ChallengeDensity(st *state.State) float64 {
	nodes := st.AllNodes()
	if len(nodes) == 0 {
		return 0
	}

	challenges := st.AllChallenges()
	return float64(len(challenges)) / float64(len(nodes))
}

// OpenChallengeDensity calculates the number of open challenges per node.
// Returns 0 if there are no nodes.
func OpenChallengeDensity(st *state.State) float64 {
	nodes := st.AllNodes()
	if len(nodes) == 0 {
		return 0
	}

	openCount := len(st.OpenChallenges())
	return float64(openCount) / float64(len(nodes))
}

// DefinitionCoverage calculates the percentage of definition references
// that have corresponding definitions in the state.
// Returns 1.0 if there are no definition references (fully covered).
func DefinitionCoverage(st *state.State) float64 {
	nodes := st.AllNodes()
	if len(nodes) == 0 {
		return 1.0
	}

	// Collect all unique definition references
	defRefs := make(map[string]bool)
	for _, n := range nodes {
		for _, ctx := range n.Context {
			if strings.HasPrefix(ctx, "def:") {
				// Extract the term name after "def:"
				term := strings.TrimPrefix(ctx, "def:")
				defRefs[term] = false // false = not found yet
			}
		}
	}

	if len(defRefs) == 0 {
		return 1.0 // No definition references, consider fully covered
	}

	// Check which definitions exist
	definedCount := 0
	for term := range defRefs {
		if st.GetDefinitionByName(term) != nil {
			definedCount++
		}
	}

	return float64(definedCount) / float64(len(defRefs))
}

// OverallQuality computes comprehensive quality metrics for the entire proof.
func OverallQuality(st *state.State) *QualityReport {
	nodes := st.AllNodes()
	challenges := st.AllChallenges()

	report := &QualityReport{
		NodeCount:          len(nodes),
		MaxDepth:           MaxRefinementDepth(st),
		TotalChallenges:    len(challenges),
		DefinitionCoverage: DefinitionCoverage(st),
	}

	// Count nodes by epistemic state
	for _, n := range nodes {
		switch n.EpistemicState {
		case schema.EpistemicPending:
			report.PendingNodes++
		case schema.EpistemicValidated:
			report.ValidatedNodes++
		case schema.EpistemicAdmitted:
			report.AdmittedNodes++
		case schema.EpistemicRefuted:
			report.RefutedNodes++
		case schema.EpistemicArchived:
			report.ArchivedNodes++
		}
	}

	// Count challenges by status
	for _, c := range challenges {
		if c.Status == state.ChallengeStatusOpen {
			report.OpenChallenges++
		} else if c.Status == state.ChallengeStatusResolved {
			report.ResolvedChallenges++
		}
	}

	// Calculate densities
	if report.NodeCount > 0 {
		report.ChallengeDensity = float64(report.TotalChallenges) / float64(report.NodeCount)
	}

	// Count definition references
	defRefs := make(map[string]bool)
	for _, n := range nodes {
		for _, ctx := range n.Context {
			if strings.HasPrefix(ctx, "def:") {
				term := strings.TrimPrefix(ctx, "def:")
				if !defRefs[term] {
					defRefs[term] = true
					report.DefinitionRefs++
					if st.GetDefinitionByName(term) != nil {
						report.DefinedRefs++
					}
				}
			}
		}
	}

	// Calculate composite quality score
	report.QualityScore = calculateQualityScore(report)

	return report
}

// SubtreeQuality computes quality metrics for a specific subtree.
// Returns an empty report if the root node doesn't exist.
func SubtreeQuality(st *state.State, rootID types.NodeID) *QualityReport {
	rootNode := st.GetNode(rootID)
	if rootNode == nil {
		return &QualityReport{
			DefinitionCoverage: 1.0,
		}
	}

	// Collect all nodes in the subtree
	subtreeNodes := collectSubtreeNodes(st, rootID)
	challenges := st.AllChallenges()

	report := &QualityReport{
		NodeCount: len(subtreeNodes),
		MaxDepth:  RefinementDepth(st, rootID),
	}

	// Build a set of subtree node IDs for quick lookup
	subtreeNodeIDs := make(map[string]bool)
	for _, n := range subtreeNodes {
		subtreeNodeIDs[n.ID.String()] = true
	}

	// Count nodes by epistemic state
	defRefs := make(map[string]bool)
	for _, n := range subtreeNodes {
		switch n.EpistemicState {
		case schema.EpistemicPending:
			report.PendingNodes++
		case schema.EpistemicValidated:
			report.ValidatedNodes++
		case schema.EpistemicAdmitted:
			report.AdmittedNodes++
		case schema.EpistemicRefuted:
			report.RefutedNodes++
		case schema.EpistemicArchived:
			report.ArchivedNodes++
		}

		// Collect definition references
		for _, ctx := range n.Context {
			if strings.HasPrefix(ctx, "def:") {
				term := strings.TrimPrefix(ctx, "def:")
				if !defRefs[term] {
					defRefs[term] = true
					report.DefinitionRefs++
					if st.GetDefinitionByName(term) != nil {
						report.DefinedRefs++
					}
				}
			}
		}
	}

	// Count challenges that target nodes in the subtree
	for _, c := range challenges {
		if subtreeNodeIDs[c.NodeID.String()] {
			report.TotalChallenges++
			if c.Status == state.ChallengeStatusOpen {
				report.OpenChallenges++
			} else if c.Status == state.ChallengeStatusResolved {
				report.ResolvedChallenges++
			}
		}
	}

	// Calculate densities
	if report.NodeCount > 0 {
		report.ChallengeDensity = float64(report.TotalChallenges) / float64(report.NodeCount)
	}

	// Calculate definition coverage
	if report.DefinitionRefs > 0 {
		report.DefinitionCoverage = float64(report.DefinedRefs) / float64(report.DefinitionRefs)
	} else {
		report.DefinitionCoverage = 1.0
	}

	// Calculate composite quality score
	report.QualityScore = calculateQualityScore(report)

	return report
}

// collectSubtreeNodes collects all nodes in the subtree rooted at the given node.
func collectSubtreeNodes(st *state.State, rootID types.NodeID) []*node.Node {
	var result []*node.Node

	rootNode := st.GetNode(rootID)
	if rootNode == nil {
		return result
	}

	result = append(result, rootNode)

	// Find all descendants
	rootIDStr := rootID.String()
	for _, n := range st.AllNodes() {
		if n.ID.String() != rootIDStr && isDescendant(n.ID, rootID) {
			result = append(result, n)
		}
	}

	return result
}

// isDescendant checks if nodeID is a descendant of ancestorID.
func isDescendant(nodeID, ancestorID types.NodeID) bool {
	return ancestorID.IsAncestorOf(nodeID)
}

// QualityScore calculates a composite quality score (0-100) for the entire proof.
func QualityScore(st *state.State) float64 {
	report := OverallQuality(st)
	return report.QualityScore
}

// calculateQualityScore computes the composite quality score from a report.
// The score is based on:
// - Validation progress (40%): Percentage of nodes validated
// - Challenge health (30%): Lower open challenge density is better
// - Definition coverage (30%): Higher coverage is better
func calculateQualityScore(report *QualityReport) float64 {
	if report.NodeCount == 0 {
		return 100.0 // Empty proof is "perfect" by default
	}

	// Validation score (0-40 points)
	// Validated + Admitted nodes contribute positively
	validatedRatio := float64(report.ValidatedNodes+report.AdmittedNodes) / float64(report.NodeCount)
	validationScore := validatedRatio * 40

	// Challenge health score (0-30 points)
	// Fewer open challenges per node is better
	// If open density > 1.0, score approaches 0
	// If open density = 0, score is 30
	openDensity := 0.0
	if report.NodeCount > 0 {
		openDensity = float64(report.OpenChallenges) / float64(report.NodeCount)
	}
	challengeScore := 30.0 * (1.0 - min(openDensity, 1.0))

	// Definition coverage score (0-30 points)
	defScore := report.DefinitionCoverage * 30

	return validationScore + challengeScore + defScore
}

// min returns the minimum of two float64 values.
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
