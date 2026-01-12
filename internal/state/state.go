// Package state provides derived state from replaying ledger events.
package state

import (
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/types"
)

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
}

// NewState creates a new empty State with all maps initialized.
func NewState() *State {
	return &State{
		nodes:       make(map[string]*node.Node),
		definitions: make(map[string]*node.Definition),
		assumptions: make(map[string]*node.Assumption),
		externals:   make(map[string]*node.External),
		lemmas:      make(map[string]*node.Lemma),
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
