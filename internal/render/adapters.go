// Package render provides human-readable formatting for AF framework types.
// This file contains adapter functions that convert domain types to view models.
// This is the ONLY file in the render package that should import domain packages.
// All other render files should work with view models only.
package render

import (
	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// NodeToView converts a node.Node to a NodeView.
// Returns empty NodeView for nil input.
func NodeToView(n *node.Node) NodeView {
	if n == nil {
		return NodeView{}
	}

	view := NodeView{
		ID:             n.ID.String(),
		Type:           string(n.Type),
		Statement:      n.Statement,
		Latex:          n.Latex,
		Inference:      string(n.Inference),
		WorkflowState:  string(n.WorkflowState),
		EpistemicState: string(n.EpistemicState),
		TaintState:     string(n.TaintState),
		ContentHash:    n.ContentHash,
		Created:        n.Created.String(),
		ClaimedBy:      n.ClaimedBy,
		Depth:          n.Depth(),
	}

	// Convert ClaimedAt
	if !n.ClaimedAt.IsZero() {
		view.ClaimedAt = n.ClaimedAt.String()
	}

	// Convert context
	if len(n.Context) > 0 {
		view.Context = make([]string, len(n.Context))
		copy(view.Context, n.Context)
	}

	// Convert dependencies
	if len(n.Dependencies) > 0 {
		view.Dependencies = types.ToStringSlice(n.Dependencies)
	}

	// Convert validation dependencies
	if len(n.ValidationDeps) > 0 {
		view.ValidationDeps = types.ToStringSlice(n.ValidationDeps)
	}

	// Convert scope
	if len(n.Scope) > 0 {
		view.Scope = make([]string, len(n.Scope))
		copy(view.Scope, n.Scope)
	}

	return view
}

// NodesToViews converts a slice of node.Node to a slice of NodeView.
func NodesToViews(nodes []*node.Node) []NodeView {
	if len(nodes) == 0 {
		return nil
	}
	views := make([]NodeView, 0, len(nodes))
	for _, n := range nodes {
		if n != nil {
			views = append(views, NodeToView(n))
		}
	}
	return views
}

// StateChallengeToView converts a state.Challenge to a ChallengeView.
func StateChallengeToView(c *state.Challenge) ChallengeView {
	if c == nil {
		return ChallengeView{}
	}
	return ChallengeView{
		ID:         c.ID,
		TargetID:   c.NodeID.String(),
		Target:     c.Target,
		Reason:     c.Reason,
		Status:     c.Status,
		Severity:   c.Severity,
		Raised:     c.Created.String(),
		Resolution: c.Resolution,
	}
}

// StateChallengesToViews converts a slice of state.Challenge to ChallengeView.
func StateChallengesToViews(challenges []*state.Challenge) []ChallengeView {
	if len(challenges) == 0 {
		return nil
	}
	views := make([]ChallengeView, 0, len(challenges))
	for _, c := range challenges {
		if c != nil {
			views = append(views, StateChallengeToView(c))
		}
	}
	return views
}

// NodeChallengeToView converts a node.Challenge to a ChallengeView.
func NodeChallengeToView(c *node.Challenge) ChallengeView {
	if c == nil {
		return ChallengeView{}
	}

	view := ChallengeView{
		ID:         c.ID,
		TargetID:   c.TargetID.String(),
		Target:     string(c.Target),
		Reason:     c.Reason,
		Status:     string(c.Status),
		Raised:     c.Raised.String(),
		Resolution: c.Resolution,
	}

	// Add target description if available
	if info, ok := schema.GetChallengeTargetInfo(c.Target); ok {
		view.TargetDesc = info.Description
	}

	return view
}

// DefinitionToView converts a node.Definition to a DefinitionView.
func DefinitionToView(d *node.Definition) DefinitionView {
	if d == nil {
		return DefinitionView{}
	}
	return DefinitionView{
		ID:      d.ID,
		Name:    d.Name,
		Content: d.Content,
	}
}

// AssumptionToView converts a node.Assumption to an AssumptionView.
func AssumptionToView(a *node.Assumption) AssumptionView {
	if a == nil {
		return AssumptionView{}
	}
	return AssumptionView{
		ID:            a.ID,
		Statement:     a.Statement,
		Justification: a.Justification,
	}
}

// ExternalToView converts a node.External to an ExternalView.
func ExternalToView(e *node.External) ExternalView {
	if e == nil {
		return ExternalView{}
	}
	return ExternalView{
		ID:     e.ID,
		Name:   e.Name,
		Source: e.Source,
		Notes:  e.Notes,
	}
}

// JobResultToView converts a jobs.JobResult to a JobListView.
func JobResultToView(jr *jobs.JobResult) JobListView {
	if jr == nil {
		return JobListView{}
	}
	return JobListView{
		ProverJobs:   NodesToViews(jr.ProverJobs),
		VerifierJobs: NodesToViews(jr.VerifierJobs),
	}
}

// StateToStatusView converts a state.State to a StatusView.
func StateToStatusView(s *state.State) StatusView {
	if s == nil {
		return StatusView{}
	}

	nodes := s.AllNodes()
	challenges := s.AllChallenges()

	// Count jobs
	proverJobs := 0
	verifierJobs := 0
	for _, n := range nodes {
		if n.WorkflowState == schema.WorkflowAvailable && n.EpistemicState == schema.EpistemicPending {
			proverJobs++
		}
		if n.WorkflowState == schema.WorkflowClaimed && n.EpistemicState == schema.EpistemicPending {
			if s.AllChildrenValidated(n.ID) {
				verifierJobs++
			}
		}
	}

	return StatusView{
		Nodes:            NodesToViews(nodes),
		Challenges:       StateChallengesToViews(challenges),
		ProverJobCount:   proverJobs,
		VerifierJobCount: verifierJobs,
	}
}

// StateToTreeView converts a state.State to a TreeView with optional custom root.
func StateToTreeView(s *state.State, customRoot *types.NodeID) TreeView {
	if s == nil {
		return TreeView{}
	}

	nodes := s.AllNodes()
	views := NodesToViews(nodes)

	// Build lookup map
	lookup := make(map[string]NodeView, len(views))
	for _, v := range views {
		lookup[v.ID] = v
	}

	var root *NodeView
	if customRoot != nil {
		if v, ok := lookup[customRoot.String()]; ok {
			root = &v
		}
	}

	return TreeView{
		Root:       root,
		Nodes:      views,
		NodeLookup: lookup,
	}
}

// BuildProverContextView builds a ProverContextView from state and node ID.
func BuildProverContextView(s *state.State, nodeID types.NodeID) ProverContextView {
	if s == nil {
		return ProverContextView{}
	}

	n := s.GetNode(nodeID)
	if n == nil {
		return ProverContextView{}
	}

	view := ProverContextView{
		Node: NodeToView(n),
	}

	// Add parent
	if parentID, hasParent := nodeID.Parent(); hasParent {
		if parent := s.GetNode(parentID); parent != nil {
			pv := NodeToView(parent)
			view.Parent = &pv
		}
	}

	// Collect siblings
	if parentID, hasParent := nodeID.Parent(); hasParent {
		parentStr := parentID.String()
		nodeStr := nodeID.String()
		for _, sib := range s.AllNodes() {
			p, ok := sib.ID.Parent()
			if !ok {
				continue
			}
			if p.String() == parentStr && sib.ID.String() != nodeStr {
				view.Siblings = append(view.Siblings, NodeToView(sib))
			}
		}
	}

	// Collect dependencies
	for _, depID := range n.Dependencies {
		if depNode := s.GetNode(depID); depNode != nil {
			view.Dependencies = append(view.Dependencies, NodeToView(depNode))
		}
	}

	// Collect definitions, assumptions, externals from context
	view.Definitions = collectDefinitionViews(s, n)
	view.Assumptions = collectAssumptionViews(s, n)
	view.Externals = collectExternalViews(s, n)

	// Collect challenges
	nodeIDStr := nodeID.String()
	for _, c := range s.AllChallenges() {
		if c.NodeID.String() == nodeIDStr {
			view.Challenges = append(view.Challenges, StateChallengeToView(c))
		}
	}

	return view
}

// BuildVerifierContextView builds a VerifierContextView from state and challenge.
func BuildVerifierContextView(s *state.State, challenge *node.Challenge) VerifierContextView {
	if s == nil || challenge == nil {
		return VerifierContextView{}
	}

	n := s.GetNode(challenge.TargetID)
	if n == nil {
		return VerifierContextView{}
	}

	view := VerifierContextView{
		Challenge: NodeChallengeToView(challenge),
		Node:      NodeToView(n),
	}

	// Add parent
	if parentID, hasParent := challenge.TargetID.Parent(); hasParent {
		if parent := s.GetNode(parentID); parent != nil {
			pv := NodeToView(parent)
			view.Parent = &pv
		}
	}

	// Collect siblings
	if parentID, hasParent := challenge.TargetID.Parent(); hasParent {
		parentStr := parentID.String()
		nodeStr := challenge.TargetID.String()
		for _, sib := range s.AllNodes() {
			p, ok := sib.ID.Parent()
			if !ok {
				continue
			}
			if p.String() == parentStr && sib.ID.String() != nodeStr {
				view.Siblings = append(view.Siblings, NodeToView(sib))
			}
		}
	}

	// Collect dependencies
	for _, depID := range n.Dependencies {
		if depNode := s.GetNode(depID); depNode != nil {
			view.Dependencies = append(view.Dependencies, NodeToView(depNode))
		}
	}

	// Collect definitions, assumptions, externals from context
	view.Definitions = collectDefinitionViews(s, n)
	view.Assumptions = collectAssumptionViews(s, n)
	view.Externals = collectExternalViews(s, n)

	return view
}

// collectDefinitionViews collects definitions referenced by a node.
func collectDefinitionViews(s *state.State, n *node.Node) []DefinitionView {
	nameSet := make(map[string]bool)

	// Check node's context and scope for def: prefixes
	for _, entry := range n.Context {
		if len(entry) > 4 && entry[:4] == "def:" {
			nameSet[entry[4:]] = true
		}
	}
	for _, entry := range n.Scope {
		if len(entry) > 4 && entry[:4] == "def:" {
			nameSet[entry[4:]] = true
		}
	}

	var views []DefinitionView
	for name := range nameSet {
		if def := s.GetDefinition(name); def != nil {
			views = append(views, DefinitionToView(def))
		}
	}
	return views
}

// collectAssumptionViews collects assumptions referenced by a node.
func collectAssumptionViews(s *state.State, n *node.Node) []AssumptionView {
	idSet := make(map[string]bool)

	// Check node's context and scope for assume: prefixes
	for _, entry := range n.Context {
		if len(entry) > 7 && entry[:7] == "assume:" {
			idSet[entry[7:]] = true
		}
	}
	for _, entry := range n.Scope {
		if len(entry) > 7 && entry[:7] == "assume:" {
			idSet[entry[7:]] = true
		}
	}

	var views []AssumptionView
	for id := range idSet {
		if assume := s.GetAssumption(id); assume != nil {
			views = append(views, AssumptionToView(assume))
		}
	}
	return views
}

// collectExternalViews collects externals referenced by a node.
func collectExternalViews(s *state.State, n *node.Node) []ExternalView {
	idSet := make(map[string]bool)

	// Check node's context and scope for ext: prefixes
	for _, entry := range n.Context {
		if len(entry) > 4 && entry[:4] == "ext:" {
			idSet[entry[4:]] = true
		}
	}
	for _, entry := range n.Scope {
		if len(entry) > 4 && entry[:4] == "ext:" {
			idSet[entry[4:]] = true
		}
	}

	var views []ExternalView
	for id := range idSet {
		if ext := s.GetExternal(id); ext != nil {
			views = append(views, ExternalToView(ext))
		}
	}
	return views
}

// IsNodeViewRoot returns true if the node view represents a root node (depth 1).
func IsNodeViewRoot(v NodeView) bool {
	return v.Depth == 1
}

// GetNodeViewParentID returns the parent ID string for a node view.
// Returns empty string and false if this is a root node.
func GetNodeViewParentID(v NodeView) (string, bool) {
	// Parse the ID to find parent
	// ID format is "1" for root, "1.2" for child, "1.2.3" for grandchild, etc.
	id := v.ID
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '.' {
			return id[:i], true
		}
	}
	return "", false
}
