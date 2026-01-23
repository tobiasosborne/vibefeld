// Package service provides the proof service facade for coordinating
// proof operations across ledger, state, locks, and filesystem.
package service

import (
	"github.com/tobias/vibefeld/internal/cycle"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// stateDependencyProvider adapts state.State to implement cycle.DependencyProvider.
// This allows the cycle detection package to work with proof state without
// creating import cycles.
type stateDependencyProvider struct {
	st *state.State
}

// GetNodeDependencies implements cycle.DependencyProvider.
func (p *stateDependencyProvider) GetNodeDependencies(id types.NodeID) ([]types.NodeID, bool) {
	n := p.st.GetNode(id)
	if n == nil {
		return nil, false
	}
	// Combine regular dependencies and validation dependencies
	deps := make([]types.NodeID, 0, len(n.Dependencies)+len(n.ValidationDeps))
	deps = append(deps, n.Dependencies...)
	deps = append(deps, n.ValidationDeps...)
	return deps, true
}

// AllNodeIDs implements cycle.DependencyProvider.
func (p *stateDependencyProvider) AllNodeIDs() []types.NodeID {
	nodes := p.st.AllNodes()
	ids := make([]types.NodeID, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}

// CheckCycles checks if there is a cycle in the dependency graph starting from
// the given node ID. This is used to validate refinements don't introduce
// circular reasoning.
//
// Returns cycle.CycleResult with HasCycle=true if a cycle is detected,
// including the cycle path. Returns HasCycle=false if no cycle exists.
//
// If the starting node doesn't exist, returns CycleResult{HasCycle: false}.
func (s *ProofService) CheckCycles(nodeID types.NodeID) (cycle.CycleResult, error) {
	st, err := s.LoadState()
	if err != nil {
		return cycle.CycleResult{}, err
	}

	provider := &stateDependencyProvider{st: st}
	return cycle.DetectCycleFrom(provider, nodeID), nil
}

// CheckAllCycles checks all nodes in the proof for dependency cycles.
// Returns a slice of cycle.CycleResult, one for each unique cycle found.
// Returns an empty slice if no cycles are found.
//
// This is useful for validating the entire proof structure.
func (s *ProofService) CheckAllCycles() ([]cycle.CycleResult, error) {
	st, err := s.LoadState()
	if err != nil {
		return nil, err
	}

	provider := &stateDependencyProvider{st: st}
	return cycle.DetectAllCycles(provider), nil
}

// WouldCreateCycle checks if adding a dependency from fromID to toID would
// create a cycle in the proof's dependency graph.
//
// This is useful for validating proposed dependencies before adding them
// (e.g., when a node is refined with logical dependencies on other nodes).
//
// Returns cycle.CycleResult indicating whether the proposed dependency would
// create a cycle, and the cycle path if so.
func (s *ProofService) WouldCreateCycle(fromID, toID types.NodeID) (cycle.CycleResult, error) {
	st, err := s.LoadState()
	if err != nil {
		return cycle.CycleResult{}, err
	}

	provider := &stateDependencyProvider{st: st}
	return cycle.WouldCreateCycle(provider, fromID, toID), nil
}
