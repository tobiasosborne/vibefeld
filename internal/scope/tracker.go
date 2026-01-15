package scope

import (
	"errors"
	"sync"

	"github.com/tobias/vibefeld/internal/types"
)

// Tracker manages assumption scopes for a proof.
// It tracks which assumption nodes have opened scopes and which nodes
// are contained within those scopes.
//
// Thread Safety: Tracker uses a read-write mutex for concurrent access.
// This allows multiple concurrent reads but ensures write operations
// are serialized.
type Tracker struct {
	mu     sync.RWMutex
	scopes map[string]*Entry // keyed by NodeID.String()
}

// ScopeInfo contains information about a node's scope context.
type ScopeInfo struct {
	// Depth is the number of active scopes containing this node.
	Depth int

	// ContainingScopes is the list of active scopes that contain this node,
	// ordered from outermost to innermost.
	ContainingScopes []*Entry
}

// IsInAnyScope returns true if the node is inside any active scope.
func (s *ScopeInfo) IsInAnyScope() bool {
	return s.Depth > 0
}

// NewTracker creates a new empty scope tracker.
func NewTracker() *Tracker {
	return &Tracker{
		scopes: make(map[string]*Entry),
	}
}

// OpenScope opens a new scope for the given assumption node.
// Returns an error if:
// - nodeID is invalid (zero value)
// - statement is empty
// - a scope is already open for this nodeID
func (t *Tracker) OpenScope(nodeID types.NodeID, statement string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate nodeID
	if nodeID.String() == "" {
		return errors.New("invalid node ID: zero value")
	}

	key := nodeID.String()

	// Check for duplicate
	if _, exists := t.scopes[key]; exists {
		return errors.New("scope already exists for node " + key)
	}

	// Create the entry
	entry, err := NewEntry(nodeID, statement)
	if err != nil {
		return err
	}

	t.scopes[key] = entry
	return nil
}

// CloseScope closes the scope opened by the given assumption node.
// Returns an error if:
// - no scope exists for this nodeID
// - the scope is already closed
func (t *Tracker) CloseScope(nodeID types.NodeID) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := nodeID.String()

	entry, exists := t.scopes[key]
	if !exists {
		return errors.New("no scope exists for node " + key)
	}

	return entry.Discharge()
}

// GetScope returns the scope entry for the given assumption node,
// or nil if no scope exists.
func (t *Tracker) GetScope(nodeID types.NodeID) *Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.scopes[nodeID.String()]
}

// AllScopes returns all scope entries (both active and closed).
func (t *Tracker) AllScopes() []*Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*Entry, 0, len(t.scopes))
	for _, entry := range t.scopes {
		result = append(result, entry)
	}
	return result
}

// GetActiveScopes returns all currently active (non-discharged) scopes.
func (t *Tracker) GetActiveScopes() []*Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return GetActiveEntries(t.AllScopesUnlocked())
}

// AllScopesUnlocked returns all scopes without acquiring the lock.
// For internal use only when lock is already held.
func (t *Tracker) AllScopesUnlocked() []*Entry {
	result := make([]*Entry, 0, len(t.scopes))
	for _, entry := range t.scopes {
		result = append(result, entry)
	}
	return result
}

// IsInScope returns true if nodeID is a descendant of scopeNodeID.
// A node is not considered to be in its own scope.
func (t *Tracker) IsInScope(nodeID types.NodeID, scopeNodeID types.NodeID) bool {
	// A node is in scope if the scope node is an ancestor of it
	return scopeNodeID.IsAncestorOf(nodeID)
}

// GetContainingScopes returns all active scopes that contain the given node.
// Returns scopes ordered from outermost (ancestor closest to root) to innermost.
func (t *Tracker) GetContainingScopes(nodeID types.NodeID) []*Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*Entry

	for _, entry := range t.scopes {
		// Only consider active scopes
		if !entry.IsActive() {
			continue
		}

		// Check if the scope node is an ancestor of the given node
		if entry.NodeID.IsAncestorOf(nodeID) {
			result = append(result, entry)
		}
	}

	// Sort by depth (outermost first)
	// Scopes with smaller depth come first
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].NodeID.Depth() > result[j].NodeID.Depth() {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// GetScopeDepth returns the number of active scopes containing the given node.
func (t *Tracker) GetScopeDepth(nodeID types.NodeID) int {
	return len(t.GetContainingScopes(nodeID))
}

// GetScopeInfo returns detailed information about a node's scope context.
func (t *Tracker) GetScopeInfo(nodeID types.NodeID) *ScopeInfo {
	scopes := t.GetContainingScopes(nodeID)
	return &ScopeInfo{
		Depth:            len(scopes),
		ContainingScopes: scopes,
	}
}
