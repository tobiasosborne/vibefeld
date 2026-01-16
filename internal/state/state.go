// Package state provides derived state from replaying ledger events.
package state

import (
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/scope"
	"github.com/tobias/vibefeld/internal/types"
)

// Challenge represents a challenge tracked in the state.
// This is a simplified representation of node.Challenge for state tracking.
type Challenge struct {
	ID         string          // Unique challenge identifier
	NodeID     types.NodeID    // The node being challenged
	Target     string          // What aspect of the node is challenged
	Reason     string          // Explanation of the challenge
	Status     string          // "open", "resolved", or "withdrawn"
	Severity   string          // "critical", "major", "minor", or "note"
	Created    types.Timestamp // When the challenge was raised
	Resolution string          // Resolution text (populated when status is "resolved")
}

// Amendment represents a single amendment to a node's statement.
type Amendment struct {
	Timestamp         types.Timestamp // When the amendment occurred
	PreviousStatement string          // The statement before this amendment
	NewStatement      string          // The statement after this amendment
	Owner             string          // Who made the amendment
}

// State represents the current derived state of a proof.
// It is reconstructed by replaying ledger events.
type State struct {
	// nodes maps NodeID to Node instances.
	nodes map[string]*node.Node

	// definitions maps definition ID to Definition instances.
	definitions map[string]*node.Definition

	// assumptions maps assumption ID to Assumption instances.
	assumptions map[string]*node.Assumption

	// externals maps external ID to External instances.
	externals map[string]*node.External

	// lemmas maps lemma ID to Lemma instances.
	lemmas map[string]*node.Lemma

	// challenges maps challenge ID to Challenge instances.
	challenges map[string]*Challenge

	// amendments maps NodeID string to a slice of Amendment records.
	// This tracks the history of all amendments made to each node.
	amendments map[string][]Amendment

	// scopeTracker tracks assumption scopes and which nodes are inside them.
	scopeTracker *scope.Tracker

	// latestSeq is the sequence number of the last event applied to this state.
	// Used for optimistic concurrency control (CAS) when appending new events.
	// A value of 0 means no events have been applied yet.
	latestSeq int
}

// NewState creates a new empty State with all maps initialized.
func NewState() *State {
	return &State{
		nodes:        make(map[string]*node.Node),
		definitions:  make(map[string]*node.Definition),
		assumptions:  make(map[string]*node.Assumption),
		externals:    make(map[string]*node.External),
		lemmas:       make(map[string]*node.Lemma),
		challenges:   make(map[string]*Challenge),
		amendments:   make(map[string][]Amendment),
		scopeTracker: scope.NewTracker(),
	}
}

// AddNode adds a node to the state.
// If a node with the same ID already exists, it is overwritten.
func (s *State) AddNode(n *node.Node) {
	s.nodes[n.ID.String()] = n
}

// GetNode returns the node with the given ID, or nil if not found.
func (s *State) GetNode(id types.NodeID) *node.Node {
	return s.nodes[id.String()]
}

// AddDefinition adds a definition to the state.
// If a definition with the same ID already exists, it is overwritten.
func (s *State) AddDefinition(d *node.Definition) {
	s.definitions[d.ID] = d
}

// GetDefinition returns the definition with the given ID, or nil if not found.
func (s *State) GetDefinition(id string) *node.Definition {
	return s.definitions[id]
}

// GetDefinitionByName returns the definition with the given name, or nil if not found.
// If multiple definitions have the same name, returns the first one found (arbitrary order).
func (s *State) GetDefinitionByName(name string) *node.Definition {
	for _, d := range s.definitions {
		if d.Name == name {
			return d
		}
	}
	return nil
}

// AddAssumption adds an assumption to the state.
// If an assumption with the same ID already exists, it is overwritten.
func (s *State) AddAssumption(a *node.Assumption) {
	s.assumptions[a.ID] = a
}

// GetAssumption returns the assumption with the given ID, or nil if not found.
func (s *State) GetAssumption(id string) *node.Assumption {
	return s.assumptions[id]
}

// AddExternal adds an external reference to the state.
// If an external with the same ID already exists, it is overwritten.
func (s *State) AddExternal(e *node.External) {
	s.externals[e.ID] = e
}

// GetExternal returns the external with the given ID, or nil if not found.
func (s *State) GetExternal(id string) *node.External {
	return s.externals[id]
}

// GetExternalByName returns the external with the given name, or nil if not found.
// If multiple externals have the same name, returns the first one found (arbitrary order).
func (s *State) GetExternalByName(name string) *node.External {
	for _, e := range s.externals {
		if e.Name == name {
			return e
		}
	}
	return nil
}

// AddLemma adds a lemma to the state.
// If a lemma with the same ID already exists, it is overwritten.
func (s *State) AddLemma(l *node.Lemma) {
	s.lemmas[l.ID] = l
}

// GetLemma returns the lemma with the given ID, or nil if not found.
func (s *State) GetLemma(id string) *node.Lemma {
	return s.lemmas[id]
}

// AllLemmas returns a slice of all lemmas in the state.
// The order of lemmas is not guaranteed.
func (s *State) AllLemmas() []*node.Lemma {
	lemmas := make([]*node.Lemma, 0, len(s.lemmas))
	for _, l := range s.lemmas {
		lemmas = append(lemmas, l)
	}
	return lemmas
}

// AddChallenge adds a challenge to the state.
// If a challenge with the same ID already exists, it is overwritten.
func (s *State) AddChallenge(c *Challenge) {
	s.challenges[c.ID] = c
}

// GetChallenge returns the challenge with the given ID, or nil if not found.
func (s *State) GetChallenge(id string) *Challenge {
	return s.challenges[id]
}

// AllChallenges returns a slice of all challenges in the state.
// The order of challenges is not guaranteed.
func (s *State) AllChallenges() []*Challenge {
	challenges := make([]*Challenge, 0, len(s.challenges))
	for _, c := range s.challenges {
		challenges = append(challenges, c)
	}
	return challenges
}

// OpenChallenges returns a slice of all open challenges in the state.
// The order of challenges is not guaranteed.
func (s *State) OpenChallenges() []*Challenge {
	var open []*Challenge
	for _, c := range s.challenges {
		if c.Status == "open" {
			open = append(open, c)
		}
	}
	return open
}

// GetBlockingChallengesForNode returns open challenges with blocking severity
// (critical or major) for the specified node.
// This is used to determine if a node can be accepted - nodes with blocking
// challenges cannot be accepted until those challenges are resolved.
func (s *State) GetBlockingChallengesForNode(nodeID types.NodeID) []*Challenge {
	var blocking []*Challenge
	nodeIDStr := nodeID.String()
	for _, c := range s.challenges {
		// Must be on the specified node
		if c.NodeID.String() != nodeIDStr {
			continue
		}
		// Must be open (not resolved or withdrawn)
		if c.Status != "open" {
			continue
		}
		// Must be a blocking severity (critical or major)
		if schema.SeverityBlocksAcceptance(schema.ChallengeSeverity(c.Severity)) {
			blocking = append(blocking, c)
		}
	}
	return blocking
}

// HasBlockingChallenges returns true if the node has any open challenges
// with Critical or Major severity that block acceptance.
func (s *State) HasBlockingChallenges(nodeID types.NodeID) bool {
	return len(s.GetBlockingChallengesForNode(nodeID)) > 0
}

// AllNodes returns a slice of all nodes in the state.
// The order of nodes is not guaranteed.
func (s *State) AllNodes() []*node.Node {
	nodes := make([]*node.Node, 0, len(s.nodes))
	for _, n := range s.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// LatestSeq returns the sequence number of the last event applied to this state.
// Returns 0 if no events have been applied yet.
// This is used for optimistic concurrency control when appending new events.
func (s *State) LatestSeq() int {
	return s.latestSeq
}

// SetLatestSeq sets the sequence number of the last applied event.
// This should only be called by the replay mechanism.
func (s *State) SetLatestSeq(seq int) {
	s.latestSeq = seq
}

// AllChildrenValidated returns true if all direct children of the node are validated.
// Returns true if the node has no children.
// This is used to determine if a node is ready for verifier review.
func (s *State) AllChildrenValidated(parentID types.NodeID) bool {
	parentStr := parentID.String()

	for _, n := range s.nodes {
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

// AddAmendment adds an amendment record for a node.
func (s *State) AddAmendment(nodeID types.NodeID, amendment Amendment) {
	key := nodeID.String()
	s.amendments[key] = append(s.amendments[key], amendment)
}

// GetAmendmentHistory returns the amendment history for a node.
// Returns an empty slice if no amendments have been made.
func (s *State) GetAmendmentHistory(nodeID types.NodeID) []Amendment {
	return s.amendments[nodeID.String()]
}

// OpenScope opens a new assumption scope at the given node.
// This should be called when a local_assume node is created.
func (s *State) OpenScope(nodeID types.NodeID, statement string) error {
	return s.scopeTracker.OpenScope(nodeID, statement)
}

// CloseScope closes the assumption scope at the given node.
// This should be called when the assumption is discharged.
func (s *State) CloseScope(nodeID types.NodeID) error {
	return s.scopeTracker.CloseScope(nodeID)
}

// GetScopeInfo returns scope information for a node.
// This includes the number of active scopes containing this node
// and the list of those scopes.
func (s *State) GetScopeInfo(nodeID types.NodeID) *scope.ScopeInfo {
	return s.scopeTracker.GetScopeInfo(nodeID)
}

// GetActiveScopes returns all currently active (open) assumption scopes.
func (s *State) GetActiveScopes() []*scope.Entry {
	return s.scopeTracker.GetActiveScopes()
}

// GetAllScopes returns all scopes (both active and closed).
func (s *State) GetAllScopes() []*scope.Entry {
	return s.scopeTracker.AllScopes()
}

// GetScope returns the scope entry for the given assumption node.
func (s *State) GetScope(nodeID types.NodeID) *scope.Entry {
	return s.scopeTracker.GetScope(nodeID)
}

// ScopeTracker returns the underlying scope tracker.
// This provides access to more advanced scope operations.
func (s *State) ScopeTracker() *scope.Tracker {
	return s.scopeTracker
}
