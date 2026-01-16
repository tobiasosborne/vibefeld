// Package render provides JSON formatting for AF framework types.
// This module handles JSON serialization for all renderable types in the AF system.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// marshalJSON marshals v to JSON without escaping HTML characters.
// This prevents characters like <, >, and & from being escaped to
// \u003c, \u003e, and \u0026.
func marshalJSON(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encode adds a trailing newline, so remove it
	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b, nil
}

// JSONNode represents a node in JSON format.
type JSONNode struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Statement      string   `json:"statement"`
	Latex          string   `json:"latex,omitempty"`
	Inference      string   `json:"inference"`
	WorkflowState  string   `json:"workflow_state"`
	EpistemicState string   `json:"epistemic_state"`
	TaintState     string   `json:"taint_state"`
	Created        string   `json:"created"`
	ContentHash    string   `json:"content_hash"`
	Context        []string `json:"context,omitempty"`
	Dependencies   []string `json:"dependencies,omitempty"`
	ValidationDeps []string `json:"validation_deps,omitempty"`
	Scope          []string `json:"scope,omitempty"`
	ClaimedBy      string   `json:"claimed_by,omitempty"`
	ClaimedAt      string   `json:"claimed_at,omitempty"`
}

// JSONChallenge represents a challenge in JSON format.
type JSONChallenge struct {
	ID         string `json:"id"`
	TargetID   string `json:"target_id"`
	Target     string `json:"target"`
	Reason     string `json:"reason"`
	Status     string `json:"status"`
	Severity   string `json:"severity,omitempty"`
	Raised     string `json:"raised"`
	ResolvedAt string `json:"resolved_at,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

// JSONStatus represents the proof status in JSON format.
type JSONStatus struct {
	Statistics JSONStatistics  `json:"statistics"`
	Jobs       JSONJobs        `json:"jobs"`
	Nodes      []JSONNode      `json:"nodes"`
	Challenges []JSONChallenge `json:"challenges"`
}

// JSONStatistics represents proof statistics in JSON format.
type JSONStatistics struct {
	TotalNodes      int            `json:"total_nodes"`
	EpistemicState  map[string]int `json:"epistemic_state"`
	TaintState      map[string]int `json:"taint_state"`
	TotalChallenges int            `json:"total_challenges"`
	OpenChallenges  int            `json:"open_challenges"`
}

// JSONJobs represents job counts in JSON format.
type JSONJobs struct {
	ProverJobs   int `json:"prover_jobs"`
	VerifierJobs int `json:"verifier_jobs"`
}

// JSONJobList represents a list of available jobs in JSON format.
type JSONJobList struct {
	ProverJobs   []JSONJobEntry `json:"prover_jobs"`
	VerifierJobs []JSONJobEntry `json:"verifier_jobs"`
}

// JSONJobEntry represents a single job entry in JSON format.
type JSONJobEntry struct {
	NodeID    string `json:"node_id"`
	Statement string `json:"statement"`
	Type      string `json:"type"`
	Depth     int    `json:"depth"`
}

// RenderNodeJSON renders a node as JSON.
// Returns JSON string representation of the node.
// Returns empty JSON object for nil node.
func RenderNodeJSON(n *node.Node) string {
	if n == nil {
		return "{}"
	}

	jn := nodeToJSON(n)

	data, err := marshalJSON(jn)
	if err != nil {
		// Fallback to minimal JSON on marshal error
		return fmt.Sprintf(`{"id":%q,"error":"failed to marshal node"}`, n.ID.String())
	}

	return string(data)
}

// RenderNodeListJSON renders a list of nodes as JSON array.
// Returns JSON array string. Returns "[]" for nil or empty list.
func RenderNodeListJSON(nodes []*node.Node) string {
	if len(nodes) == 0 {
		return "[]"
	}

	jsonNodes := make([]JSONNode, 0, len(nodes))
	for _, n := range nodes {
		if n != nil {
			jsonNodes = append(jsonNodes, nodeToJSON(n))
		}
	}

	data, err := marshalJSON(jsonNodes)
	if err != nil {
		return "[]"
	}

	return string(data)
}

// RenderStatusJSON renders the proof status as JSON.
// Returns JSON string representation of the status.
// Returns empty JSON object for nil state.
func RenderStatusJSON(s *state.State) string {
	if s == nil {
		return `{"error":"no proof state initialized"}`
	}

	nodes := s.AllNodes()
	if len(nodes) == 0 {
		return `{"statistics":{"total_nodes":0},"jobs":{"prover_jobs":0,"verifier_jobs":0},"nodes":[]}`
	}

	status := statusToJSON(s, nodes)

	data, err := marshalJSON(status)
	if err != nil {
		return `{"error":"failed to marshal status"}`
	}

	return string(data)
}

// RenderJobsJSON renders available jobs as JSON.
// Returns JSON string representation of the jobs.
// Returns empty JSON structure for nil job result.
func RenderJobsJSON(jobList *jobs.JobResult) string {
	if jobList == nil {
		return `{"prover_jobs":[],"verifier_jobs":[]}`
	}

	jl := JSONJobList{
		ProverJobs:   make([]JSONJobEntry, 0, len(jobList.ProverJobs)),
		VerifierJobs: make([]JSONJobEntry, 0, len(jobList.VerifierJobs)),
	}

	for _, job := range jobList.ProverJobs {
		jl.ProverJobs = append(jl.ProverJobs, JSONJobEntry{
			NodeID:    job.ID.String(),
			Statement: job.Statement,
			Type:      string(job.Type),
			Depth:     job.Depth(),
		})
	}

	for _, job := range jobList.VerifierJobs {
		jl.VerifierJobs = append(jl.VerifierJobs, JSONJobEntry{
			NodeID:    job.ID.String(),
			Statement: job.Statement,
			Type:      string(job.Type),
			Depth:     job.Depth(),
		})
	}

	data, err := marshalJSON(jl)
	if err != nil {
		return `{"prover_jobs":[],"verifier_jobs":[]}`
	}

	return string(data)
}

// nodeToJSON converts a node to its JSON representation.
func nodeToJSON(n *node.Node) JSONNode {
	jn := JSONNode{
		ID:             n.ID.String(),
		Type:           string(n.Type),
		Statement:      n.Statement,
		Latex:          n.Latex,
		Inference:      string(n.Inference),
		WorkflowState:  string(n.WorkflowState),
		EpistemicState: string(n.EpistemicState),
		TaintState:     string(n.TaintState),
		Created:        n.Created.String(),
		ContentHash:    n.ContentHash,
		ClaimedBy:      n.ClaimedBy,
	}

	// Add claimed_at if node is claimed
	if !n.ClaimedAt.IsZero() {
		jn.ClaimedAt = n.ClaimedAt.String()
	}

	// Convert context
	if len(n.Context) > 0 {
		jn.Context = make([]string, len(n.Context))
		copy(jn.Context, n.Context)
	}

	// Convert dependencies
	if len(n.Dependencies) > 0 {
		jn.Dependencies = make([]string, len(n.Dependencies))
		for i, dep := range n.Dependencies {
			jn.Dependencies[i] = dep.String()
		}
	}

	// Convert validation dependencies
	if len(n.ValidationDeps) > 0 {
		jn.ValidationDeps = make([]string, len(n.ValidationDeps))
		for i, dep := range n.ValidationDeps {
			jn.ValidationDeps[i] = dep.String()
		}
	}

	// Convert scope
	if len(n.Scope) > 0 {
		jn.Scope = make([]string, len(n.Scope))
		copy(jn.Scope, n.Scope)
	}

	return jn
}

// statusToJSON converts state to JSON status representation.
func statusToJSON(s *state.State, nodes []*node.Node) JSONStatus {
	// Build statistics
	epistemicCounts := make(map[string]int)
	taintCounts := make(map[string]int)

	for _, n := range nodes {
		epistemicCounts[string(n.EpistemicState)]++
		taintCounts[string(n.TaintState)]++
	}

	// Count jobs
	proverJobs := 0
	verifierJobs := 0
	for _, n := range nodes {
		if n.WorkflowState == "available" && n.EpistemicState == "pending" {
			proverJobs++
		}
		if n.WorkflowState == "claimed" && n.EpistemicState == "pending" {
			if s.AllChildrenValidated(n.ID) {
				verifierJobs++
			}
		}
	}

	// Convert nodes to JSON
	jsonNodes := make([]JSONNode, 0, len(nodes))
	for _, n := range nodes {
		jsonNodes = append(jsonNodes, nodeToJSON(n))
	}

	// Convert challenges to JSON
	allChallenges := s.AllChallenges()
	jsonChallenges := make([]JSONChallenge, 0, len(allChallenges))
	openChallengeCount := 0
	for _, c := range allChallenges {
		jsonChallenges = append(jsonChallenges, challengeToJSON(c))
		if c.Status == "open" {
			openChallengeCount++
		}
	}

	return JSONStatus{
		Statistics: JSONStatistics{
			TotalNodes:      len(nodes),
			EpistemicState:  epistemicCounts,
			TaintState:      taintCounts,
			TotalChallenges: len(allChallenges),
			OpenChallenges:  openChallengeCount,
		},
		Jobs: JSONJobs{
			ProverJobs:   proverJobs,
			VerifierJobs: verifierJobs,
		},
		Nodes:      jsonNodes,
		Challenges: jsonChallenges,
	}
}

// challengeToJSON converts a state.Challenge to its JSON representation.
func challengeToJSON(c *state.Challenge) JSONChallenge {
	jc := JSONChallenge{
		ID:       c.ID,
		TargetID: c.NodeID.String(),
		Target:   c.Target,
		Reason:   c.Reason,
		Status:   c.Status,
		Severity: c.Severity,
		Raised:   c.Created.String(),
	}

	// Add resolution fields if challenge is resolved
	if c.Status == "resolved" && c.Resolution != "" {
		jc.Resolution = c.Resolution
	}

	return jc
}

// RenderProverContextJSON renders prover context as JSON.
// Returns JSON string with node details and context for proving.
func RenderProverContextJSON(s *state.State, nodeID types.NodeID) string {
	if s == nil {
		return `{"error":"no state provided"}`
	}

	n := s.GetNode(nodeID)
	if n == nil {
		return fmt.Sprintf(`{"error":"node %s not found"}`, nodeID.String())
	}

	// Build context structure
	ctx := map[string]interface{}{
		"node":             nodeToJSON(n),
		"parent":           nil,
		"siblings":         []JSONNode{},
		"children":         []JSONNode{},
		"challenges":       []JSONChallenge{},
		"available_defs":   []string{},
		"available_lemmas": []string{},
	}

	// Add parent if exists
	if parentID, hasParent := nodeID.Parent(); hasParent {
		if parent := s.GetNode(parentID); parent != nil {
			ctx["parent"] = nodeToJSON(parent)
		}
	}

	// Add siblings, children (simplified - would need full state walking)
	allNodes := s.AllNodes()
	parentStr := ""
	if p, hasParent := nodeID.Parent(); hasParent {
		parentStr = p.String()
	}

	siblings := []JSONNode{}
	children := []JSONNode{}
	for _, an := range allNodes {
		if anParent, hasParent := an.ID.Parent(); hasParent {
			if anParent.String() == parentStr && an.ID.String() != nodeID.String() {
				siblings = append(siblings, nodeToJSON(an))
			}
		}
		if anParent, hasParent := an.ID.Parent(); hasParent {
			if anParent.String() == nodeID.String() {
				children = append(children, nodeToJSON(an))
			}
		}
	}
	ctx["siblings"] = siblings
	ctx["children"] = children

	// Add challenges for this node
	allChallenges := s.AllChallenges()
	nodeIDStr := nodeID.String()
	nodeChallenges := []JSONChallenge{}
	for _, c := range allChallenges {
		if c.NodeID.String() == nodeIDStr {
			nodeChallenges = append(nodeChallenges, challengeToJSON(c))
		}
	}
	ctx["challenges"] = nodeChallenges

	data, err := marshalJSON(ctx)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal context for node %s"}`, nodeID.String())
	}

	return string(data)
}

// RenderVerifierContextJSON renders verifier context as JSON.
// Returns JSON string with challenge details and validation context.
func RenderVerifierContextJSON(s *state.State, challenge *node.Challenge) string {
	if s == nil {
		return `{"error":"no state provided"}`
	}

	if challenge == nil {
		return `{"error":"no challenge provided"}`
	}

	// Build challenge context
	ctx := map[string]interface{}{
		"challenge_id": challenge.ID,
		"target_id":    challenge.TargetID.String(),
		"target":       string(challenge.Target),
		"reason":       challenge.Reason,
		"raised":       challenge.Raised.String(),
		"status":       string(challenge.Status),
		"children":     []JSONNode{},
	}

	// Include resolution text for resolved challenges
	if challenge.Status == node.ChallengeStatusResolved && challenge.Resolution != "" {
		ctx["resolution"] = challenge.Resolution
	}

	// Add node details
	targetNode := s.GetNode(challenge.TargetID)
	if targetNode != nil {
		ctx["node"] = nodeToJSON(targetNode)

		// Add children of the challenged node
		allNodes := s.AllNodes()
		targetIDStr := challenge.TargetID.String()
		children := []JSONNode{}
		for _, an := range allNodes {
			if anParent, hasParent := an.ID.Parent(); hasParent {
				if anParent.String() == targetIDStr {
					children = append(children, nodeToJSON(an))
				}
			}
		}
		ctx["children"] = children
	}

	data, err := marshalJSON(ctx)
	if err != nil {
		return `{"error":"failed to marshal verifier context"}`
	}

	return string(data)
}

// RenderChallengeJSON renders a single challenge as JSON.
// Returns JSON string representation of the challenge.
// Returns empty JSON object for nil challenge.
func RenderChallengeJSON(c *state.Challenge) string {
	if c == nil {
		return "{}"
	}

	jc := challengeToJSON(c)

	data, err := marshalJSON(jc)
	if err != nil {
		return fmt.Sprintf(`{"id":%q,"error":"failed to marshal challenge"}`, c.ID)
	}

	return string(data)
}

// RenderChallengesJSON renders a list of challenges as JSON array.
// Returns JSON array string. Returns "[]" for nil or empty list.
func RenderChallengesJSON(challenges []*state.Challenge) string {
	if len(challenges) == 0 {
		return "[]"
	}

	jsonChallenges := make([]JSONChallenge, 0, len(challenges))
	for _, c := range challenges {
		if c != nil {
			jsonChallenges = append(jsonChallenges, challengeToJSON(c))
		}
	}

	data, err := marshalJSON(jsonChallenges)
	if err != nil {
		return "[]"
	}

	return string(data)
}

// RenderNodeChallengesJSON renders all challenges for a specific node as JSON.
// Returns JSON array string. Returns "[]" if no challenges exist for the node.
func RenderNodeChallengesJSON(s *state.State, nodeID types.NodeID) string {
	if s == nil {
		return "[]"
	}

	allChallenges := s.AllChallenges()
	nodeIDStr := nodeID.String()

	var nodeChallenges []*state.Challenge
	for _, c := range allChallenges {
		if c.NodeID.String() == nodeIDStr {
			nodeChallenges = append(nodeChallenges, c)
		}
	}

	return RenderChallengesJSON(nodeChallenges)
}
