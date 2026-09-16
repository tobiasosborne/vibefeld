// Package support defines the typed support relation over proof nodes and the
// acyclicity / scope checks that creation paths run over a complete prospective
// batch.
//
// Two kinds of support edge exist:
//
//   - Result-use n -> t: t is a result n relies on. Sources are (i) every
//     non-local_assume child of n, and (ii) every t in n.Dependencies or
//     n.ValidationDeps that is not a local_assume.
//   - Hypothesis-use n -> h: h is a local_assume that encloses n. Hypotheses
//     are introduced, not established; they carry no taint and never form a
//     cycle.
//
// Acyclicity is required over result-use edges only. The Provider implements
// cycle.DependencyProvider so internal/cycle is reused unchanged.
package support

import (
	"sort"

	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// EdgeKind distinguishes the two support edges.
type EdgeKind int

const (
	// ResultUse means the target is a result the source relies on. Result-use
	// edges must be acyclic.
	ResultUse EdgeKind = iota
	// HypothesisUse means the target is a local_assume enclosing the source.
	// Hypothesis-use edges are never part of the cycle graph.
	HypothesisUse
)

// Edge is a single typed support edge.
type Edge struct {
	From types.NodeID
	To   types.NodeID
	Kind EdgeKind
}

// ProspectiveNode describes a node that may not exist in state yet. An overlay
// entry whose ID already exists in state REPLACES that node's edges (its
// ParentID, Dependencies and ValidationDeps); this is what makes a removal-only
// amendment expressible. ParentID is optional: when empty it is derived from
// ID.Parent().
type ProspectiveNode struct {
	ID             types.NodeID
	ParentID       types.NodeID
	Type           schema.NodeType
	Dependencies   []types.NodeID
	ValidationDeps []types.NodeID
}

// DanglingDep is a result-use dependency on a node that is missing from the
// ledger or severed (archived/refuted). It is kept as a sink edge in the
// result-use graph (no cycle can pass through it) and reported here so audit
// can surface it later.
type DanglingDep struct {
	From types.NodeID
	To   types.NodeID
	// Severed is true when To exists in state but is archived or refuted;
	// false when To is entirely absent.
	Severed bool
}

// Provider is the result-use adjacency over state with an optional overlay of
// prospective nodes. It implements cycle.DependencyProvider.
type Provider struct {
	deps     map[string][]types.NodeID
	order    []types.NodeID
	dangling []DanglingDep
}

// GetNodeDependencies implements cycle.DependencyProvider.
func (p *Provider) GetNodeDependencies(id types.NodeID) ([]types.NodeID, bool) {
	d, ok := p.deps[id.String()]
	if !ok {
		return nil, false
	}
	return d, true
}

// AllNodeIDs implements cycle.DependencyProvider.
func (p *Provider) AllNodeIDs() []types.NodeID {
	return p.order
}

// Dangling returns the dangling dependency edges discovered while building the
// graph (dependencies on missing or severed nodes).
func (p *Provider) Dangling() []DanglingDep {
	return p.dangling
}

// ResultUseEdges builds the result-use graph over state plus the prospective
// overlay. Only result-use edges are present; local_assume targets are
// hypothesis-use and are therefore absent.
func ResultUseEdges(st *state.State, overlay []ProspectiveNode) Provider {
	return resultUseEdges(newUniverse(st, overlay))
}

// resultUseEdges builds the adjacency for an already-constructed universe.
func resultUseEdges(u *universe) Provider {
	p := Provider{deps: make(map[string][]types.NodeID)}

	children := make(map[string][]*nodeInfo)
	for _, info := range u.nodes {
		if info.hasParent {
			key := info.parent.String()
			children[key] = append(children[key], info)
		}
	}
	for _, siblings := range children {
		sort.Slice(siblings, func(i, j int) bool { return siblings[i].id.Less(siblings[j].id) })
	}

	for _, info := range u.nodes {
		edges := make([]types.NodeID, 0, len(info.deps)+len(info.valDeps))
		if !info.severed {
			// (i) children: a parent's proof is its non-local_assume children.
			for _, c := range children[info.id.String()] {
				if c.typ == schema.NodeTypeLocalAssume || c.severed {
					continue
				}
				edges = append(edges, c.id)
			}
			// (ii) explicit dependencies, minus local_assume targets.
			for _, t := range info.allDeps() {
				if u.isLocalAssume(t) {
					continue
				}
				edges = append(edges, t)
				if !u.exists(t) {
					p.dangling = append(p.dangling, DanglingDep{From: info.id, To: t})
				} else if u.isSevered(t) {
					p.dangling = append(p.dangling, DanglingDep{From: info.id, To: t, Severed: true})
				}
			}
		}
		p.deps[info.id.String()] = dedupe(edges)
	}

	for _, info := range u.nodes {
		p.order = append(p.order, info.id)
	}
	sort.Slice(p.order, func(i, j int) bool { return p.order[i].Less(p.order[j]) })
	return p
}

// DanglingDeps reports result-use dependencies on missing or severed nodes for
// the given state and overlay. It is a thin wrapper over ResultUseEdges for
// callers (e.g. audit) that only need the blockers.
func DanglingDeps(st *state.State, overlay []ProspectiveNode) []DanglingDep {
	p := ResultUseEdges(st, overlay)
	return p.dangling
}

// nodeInfo is the union of a state node and an overlay entry.
type nodeInfo struct {
	id        types.NodeID
	typ       schema.NodeType
	parent    types.NodeID
	hasParent bool
	severed   bool
	exists    bool
	deps      []types.NodeID
	valDeps   []types.NodeID
}

func (n *nodeInfo) allDeps() []types.NodeID {
	out := make([]types.NodeID, 0, len(n.deps)+len(n.valDeps))
	out = append(out, n.deps...)
	out = append(out, n.valDeps...)
	return out
}

// universe is the node set over state + overlay, keyed by ID string.
type universe struct {
	nodes    map[string]*nodeInfo
	encl     map[string][]types.NodeID
	enclDone bool
}

func newUniverse(st *state.State, overlay []ProspectiveNode) *universe {
	u := &universe{nodes: make(map[string]*nodeInfo)}
	if st != nil {
		for _, n := range st.AllNodes() {
			if n == nil {
				continue
			}
			parent, hasParent := n.ID.Parent()
			u.nodes[n.ID.String()] = &nodeInfo{
				id:        n.ID,
				typ:       n.Type,
				parent:    parent,
				hasParent: hasParent,
				severed:   isSeveredState(n.EpistemicState),
				exists:    true,
				deps:      n.Dependencies,
				valDeps:   n.ValidationDeps,
			}
		}
	}
	for i := range overlay {
		pn := overlay[i]
		parent := pn.ParentID
		hasParent := parent.String() != ""
		if !hasParent {
			parent, hasParent = pn.ID.Parent()
		}
		info := &nodeInfo{
			id:        pn.ID,
			typ:       pn.Type,
			parent:    parent,
			hasParent: hasParent,
			deps:      pn.Dependencies,
			valDeps:   pn.ValidationDeps,
		}
		if existing, ok := u.nodes[pn.ID.String()]; ok {
			info.severed = existing.severed
			info.exists = true
		}
		u.nodes[pn.ID.String()] = info
	}
	return u
}

func (u *universe) exists(id types.NodeID) bool {
	info, ok := u.nodes[id.String()]
	return ok && info.exists
}

func (u *universe) isSevered(id types.NodeID) bool {
	info, ok := u.nodes[id.String()]
	return ok && info.severed
}

func (u *universe) isLocalAssume(id types.NodeID) bool {
	info, ok := u.nodes[id.String()]
	return ok && info.typ == schema.NodeTypeLocalAssume
}

func (u *universe) isLocalDischarge(id types.NodeID) bool {
	info, ok := u.nodes[id.String()]
	return ok && info.typ == schema.NodeTypeLocalDischarge
}

// childrenOf returns the children of parent sorted by child number.
func (u *universe) childrenOf(parent types.NodeID) []*nodeInfo {
	var out []*nodeInfo
	for _, info := range u.nodes {
		if info.hasParent && info.parent.String() == parent.String() {
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id.Less(out[j].id) })
	return out
}

func isSeveredState(es schema.EpistemicState) bool {
	return es == schema.EpistemicArchived || es == schema.EpistemicRefuted
}

func dedupe(ids []types.NodeID) []types.NodeID {
	if len(ids) <= 1 {
		return ids
	}
	seen := make(map[string]bool, len(ids))
	out := ids[:0]
	for _, id := range ids {
		if seen[id.String()] {
			continue
		}
		seen[id.String()] = true
		out = append(out, id)
	}
	return out
}
