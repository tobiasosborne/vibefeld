// Package taint provides taint computation and propagation logic for AF nodes.
package taint

import (
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

type taintComponent uint8

const (
	componentClean taintComponent = iota
	componentTainted
	componentUnresolved
)

type treeTaints struct {
	nodes    []*node.Node
	children map[string][]*node.Node
	down     map[string]taintComponent
	up       map[string]taintComponent
	final    map[string]node.TaintState
}

// PropagateTaint recomputes taint for root, its ancestors, and its descendants.
// Descendant-derived taint is used only while walking upward, so it cannot leak
// back down into siblings. The complete tree is inspected in linear time so an
// ancestor's subtree contribution includes every relevant branch.
//
// Returns list of nodes whose taint actually changed.
// Root is included when its taint changed.
//
// Returns nil/empty slice if:
// - root is nil
// - allNodes is nil or empty
// - no affected node changed
func PropagateTaint(root *node.Node, allNodes []*node.Node) []*node.Node {
	if root == nil || len(allNodes) == 0 {
		return nil
	}

	computed := computeTreeTaints(allNodes)
	var changed []*node.Node
	for _, n := range computed.nodes {
		if !n.ID.Equal(root.ID) && !root.ID.IsAncestorOf(n.ID) && !n.ID.IsAncestorOf(root.ID) {
			continue
		}
		newTaint := computed.final[n.ID.String()]
		if n.TaintState != newTaint {
			n.TaintState = newTaint
			changed = append(changed, n)
		}
	}

	return changed
}

// RecomputeAll recomputes and applies taint for every node in a proof tree.
// It returns every node whose stored taint changed. Both the shallow ancestor
// pass and deepest-first subtree pass are linear in the number of nodes (plus
// the size of sparse node-ID paths).
func RecomputeAll(allNodes []*node.Node) []*node.Node {
	if len(allNodes) == 0 {
		return nil
	}

	computed := computeTreeTaints(allNodes)
	var changed []*node.Node
	for _, n := range computed.nodes {
		newTaint := computed.final[n.ID.String()]
		if n.TaintState != newTaint {
			n.TaintState = newTaint
			changed = append(changed, n)
		}
	}
	return changed
}

func computeTreeTaints(allNodes []*node.Node) treeTaints {
	result := treeTaints{
		children: make(map[string][]*node.Node),
		down:     make(map[string]taintComponent),
		up:       make(map[string]taintComponent),
		final:    make(map[string]node.TaintState),
	}

	nodeMap := make(map[string]*node.Node, len(allNodes))
	seen := make(map[string]bool, len(allNodes))
	maxDepth := 0
	for _, n := range allNodes {
		if n == nil {
			continue
		}
		key := n.ID.String()
		nodeMap[key] = n
		if seen[key] {
			continue
		}
		seen[key] = true
		result.nodes = append(result.nodes, n)
		if depth := n.ID.Depth(); depth > maxDepth {
			maxDepth = depth
		}
	}

	// If distinct pointers with the same ID were supplied, consistently use the
	// map winner while preserving the first-seen ID order.
	for i, n := range result.nodes {
		result.nodes[i] = nodeMap[n.ID.String()]
	}

	byDepth := make([][]*node.Node, maxDepth+1)
	for _, n := range result.nodes {
		byDepth[n.ID.Depth()] = append(byDepth[n.ID.Depth()], n)
	}

	nearestCache := make(map[string]*node.Node)
	parentFor := make(map[string]*node.Node, len(result.nodes))
	for depth := 1; depth <= maxDepth; depth++ {
		for _, n := range byDepth[depth] {
			parent := nearestExistingParent(n, nodeMap, nearestCache)
			parentFor[n.ID.String()] = parent
			if parent != nil {
				result.children[parent.ID.String()] = append(result.children[parent.ID.String()], n)
			}
		}
	}

	// Compute the ancestor-chain component shallowest-first. chain includes the
	// current node's epistemic contribution and is inherited by its children.
	chain := make(map[string]taintComponent, len(result.nodes))
	for depth := 1; depth <= maxDepth; depth++ {
		for _, n := range byDepth[depth] {
			key := n.ID.String()
			down := componentClean
			if parent := parentFor[key]; parent != nil {
				down = chain[parent.ID.String()]
			}
			result.down[key] = down
			chain[key] = combineComponents(down, epistemicContribution(n.EpistemicState))
		}
	}

	// Compute the subtree component deepest-first. An admitted child contributes
	// tainted without inspecting its subtree; a severed child cuts off its branch.
	for depth := maxDepth; depth >= 1; depth-- {
		for _, n := range byDepth[depth] {
			up := componentClean
			for _, child := range result.children[n.ID.String()] {
				if isSevered(child) {
					continue
				}
				if isUnresolvedState(child.EpistemicState) {
					up = combineComponents(up, componentUnresolved)
				} else if schema.IntroducesTaint(child.EpistemicState) {
					up = combineComponents(up, componentTainted)
				} else {
					up = combineComponents(up, result.up[child.ID.String()])
				}
				if up == componentUnresolved {
					break
				}
			}
			result.up[n.ID.String()] = up
		}
	}

	for _, n := range result.nodes {
		key := n.ID.String()
		result.final[key] = finalTaint(n, result.down[key], result.up[key])
	}
	return result
}

func nearestExistingParent(n *node.Node, nodeMap map[string]*node.Node, cache map[string]*node.Node) *node.Node {
	parentID, hasParent := n.ID.Parent()
	var missing []string
	for hasParent {
		key := parentID.String()
		if parent, ok := nodeMap[key]; ok {
			for _, missingKey := range missing {
				cache[missingKey] = parent
			}
			return parent
		}
		if parent, ok := cache[key]; ok {
			for _, missingKey := range missing {
				cache[missingKey] = parent
			}
			return parent
		}
		missing = append(missing, key)
		parentID, hasParent = parentID.Parent()
	}
	for _, missingKey := range missing {
		cache[missingKey] = nil
	}
	return nil
}

func isSevered(n *node.Node) bool {
	return n.EpistemicState == schema.EpistemicArchived || n.EpistemicState == schema.EpistemicRefuted
}

func epistemicContribution(state schema.EpistemicState) taintComponent {
	if isUnresolvedState(state) {
		return componentUnresolved
	}
	if schema.IntroducesTaint(state) {
		return componentTainted
	}
	return componentClean
}

func isUnresolvedState(state schema.EpistemicState) bool {
	return state == schema.EpistemicPending ||
		state == schema.EpistemicDraft ||
		state == schema.EpistemicNeedsRefinement
}

func combineComponents(a, b taintComponent) taintComponent {
	if a == componentUnresolved || b == componentUnresolved {
		return componentUnresolved
	}
	if a == componentTainted || b == componentTainted {
		return componentTainted
	}
	return componentClean
}

func finalTaint(n *node.Node, down, up taintComponent) node.TaintState {
	if isSevered(n) {
		return node.TaintClean
	}
	if isUnresolvedState(n.EpistemicState) {
		return node.TaintUnresolved
	}
	if down == componentUnresolved {
		return node.TaintUnresolved
	}
	if schema.IntroducesTaint(n.EpistemicState) {
		return node.TaintSelfAdmitted
	}
	if up == componentUnresolved {
		return node.TaintUnresolved
	}
	if down == componentTainted {
		return node.TaintTainted
	}
	if up == componentTainted {
		return node.TaintTainted
	}
	return node.TaintClean
}

// GenerateTaintEvents creates TaintRecomputed events for all changed nodes.
// This function should be called after PropagateTaint to generate ledger events
// for nodes whose taint state has changed.
//
// Returns a slice of TaintRecomputed events, one for each changed node.
// Returns nil if changedNodes is nil or empty.
func GenerateTaintEvents(changedNodes []*node.Node) []ledger.TaintRecomputed {
	if len(changedNodes) == 0 {
		return nil
	}

	events := make([]ledger.TaintRecomputed, 0, len(changedNodes))
	for _, n := range changedNodes {
		if n != nil {
			events = append(events, ledger.NewTaintRecomputed(n.ID, n.TaintState))
		}
	}
	return events
}

// PropagateAndGenerateEvents is a convenience function that propagates taint
// and generates TaintRecomputed events in a single call.
//
// It combines PropagateTaint and GenerateTaintEvents for common use cases
// where both operations are needed together.
//
// Returns:
//   - changedNodes: nodes whose taint state was updated
//   - events: TaintRecomputed events for each changed node
func PropagateAndGenerateEvents(root *node.Node, allNodes []*node.Node) ([]*node.Node, []ledger.TaintRecomputed) {
	changedNodes := PropagateTaint(root, allNodes)
	events := GenerateTaintEvents(changedNodes)
	return changedNodes, events
}
