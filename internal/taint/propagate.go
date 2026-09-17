// Package taint provides taint computation and propagation logic for AF nodes.
package taint

import (
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
)

type taintComponent uint8

const (
	componentClean taintComponent = iota
	componentTainted
	componentUnresolved
)

type treeTaints struct {
	nodes []*node.Node
	down  map[string]taintComponent
	up    map[string]taintComponent // D6 support component (result-use fold)
	final map[string]node.TaintState
}

// PropagateTaint is a thin wrapper over RecomputeAll kept for its callers'
// signatures. There is no affected set: since D6 made reference and validation
// dependencies carry taint, a change can reach ancestors, descendants and every
// transitive reverse dependent, so EVERY node in allNodes is rederived and the
// nodes whose stored taint actually changed are applied and returned. root is
// used only as a nil guard and to scope the caller's event emission; it does
// not restrict what is recomputed, and passing a different root cannot change
// the result.
//
// Returns nil when root is nil, when allNodes is empty, or when no node
// changed.
func PropagateTaint(root *node.Node, allNodes []*node.Node) []*node.Node {
	if root == nil {
		return nil
	}
	return RecomputeAll(allNodes)
}

// RecomputeAll recomputes and applies taint for every node in a proof tree.
// It returns every node whose stored taint changed. The ancestor pass is linear
// in the number of nodes (plus the size of sparse node-ID paths), and the D6
// support component is one fold over one prepared result-use graph.
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
		down:  make(map[string]taintComponent),
		up:    make(map[string]taintComponent),
		final: make(map[string]node.TaintState),
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

	// Compute the support component (children, reference and validation
	// dependencies) with the shared D6 fold over one prepared result-use graph.
	// This replaces the 0.1.7 subtree walk: reference and validation targets now
	// carry taint exactly like children, severed dependency targets and legacy
	// cycles are unresolved, and admitted targets are not descended.
	supportVals := supportComponents(result.nodes)
	for key, v := range supportVals {
		result.up[key] = v.comp
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

// finalTaint applies the D6 per-node precedence: own severed state, then own
// admitted (self_admitted), then unresolved (own state first, then the support
// component and ancestors), then tainted (ancestors then support), then clean.
// An admitted node is a deliberate escape hatch, so its own verdict is not
// overridden by an ancestor's unresolved state; the ancestor component is
// applied only after the node's own state.
func finalTaint(n *node.Node, down, up taintComponent) node.TaintState {
	if isSevered(n) {
		return node.TaintClean
	}
	if schema.IntroducesTaint(n.EpistemicState) {
		return node.TaintSelfAdmitted
	}
	if isUnresolvedState(n.EpistemicState) {
		return node.TaintUnresolved
	}
	if down == componentUnresolved {
		return node.TaintUnresolved
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
