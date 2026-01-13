// Package state provides derived state from replaying ledger events.
package state

import (
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/types"
)

// Challenge represents a challenge tracked in the state.
// This is a simplified representation of node.Challenge for state tracking.
type Challenge struct {
	ID      string          // Unique challenge identifier
	NodeID  types.NodeID    // The node being challenged
	Target  string          // What aspect of the node is challenged
	Reason  string          // Explanation of the challenge
	Status  string          // "open", "resolved", or "withdrawn"
	Created types.Timestamp // When the challenge was raised
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

	// latestSeq is the sequence number of the last event applied to this state.
	// Used for optimistic concurrency control (CAS) when appending new events.
	// A value of 0 means no events have been applied yet.
	latestSeq int
}

// NewState creates a new empty State with all maps initialized.
func NewState() *State {
	return &State{
		nodes:       make(map[string]*node.Node),
		definitions: make(map[string]*node.Definition),
		assumptions: make(map[string]*node.Assumption),
		externals:   make(map[string]*node.External),
		lemmas:      make(map[string]*node.Lemma),
		challenges:  make(map[string]*Challenge),
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

// AddLemma adds a lemma to the state.
// If a lemma with the same ID already exists, it is overwritten.
func (s *State) AddLemma(l *node.Lemma) {
	s.lemmas[l.ID] = l
}

// GetLemma returns the lemma with the given ID, or nil if not found.
func (s *State) GetLemma(id string) *node.Lemma {
	return s.lemmas[id]
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
