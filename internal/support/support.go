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

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// State is the read-only slice of derived state the support graph needs. It is
// satisfied by *state.State. It exists so this package does not import
// internal/state: internal/state imports internal/taint, and internal/taint
// folds over the graph this package prepares, so a support -> state import
// would close the cycle state -> taint -> support -> state (v3.1 amendment 4).
type State interface {
	AllNodes() []*node.Node
	GetNode(types.NodeID) *node.Node
	HasBlockingChallenges(types.NodeID) bool
	LatestAmendmentSeq(types.NodeID) (int, bool)
}

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

// GraphEdgeKind records why a result-use edge exists. Children are a parent's
// proof step nodes; dependency edges come from n.Dependencies; validation-dep
// edges from n.ValidationDeps (both minus local_assume targets).
type GraphEdgeKind uint8

const (
	// EdgeChild is a parent -> non-local_assume child result edge.
	EdgeChild GraphEdgeKind = iota
	// EdgeDependency is a node -> n.Dependencies target result edge.
	EdgeDependency
	// EdgeValidationDep is a node -> n.ValidationDeps target result edge.
	EdgeValidationDep
)

// GraphEdge is one result-use edge with its origin kind, retained on the
// prepared Graph so folds can tell children from explicit dependencies.
type GraphEdge struct {
	From types.NodeID
	To   types.NodeID
	Kind GraphEdgeKind
}

// Provider is the result-use adjacency over state with an optional overlay of
// prospective nodes. It implements cycle.DependencyProvider.
type Provider struct {
	deps     map[string][]types.NodeID
	order    []types.NodeID
	dangling []DanglingDep

	// nodes maps a node ID string to the state-backed node, when one exists.
	// Overlay-only (prospective) nodes have no entry. The memoised Walk uses
	// it to hand the fold the *node.Node it is folding.
	nodes map[string]*node.Node

	// edges lists every result-use edge with its origin kind, and children is
	// the direct-child index (all children, including severed and local_assume
	// ones), sorted by hierarchical ID. Both are retained so Prepare can build
	// the prepared Graph once for several folds.
	edges    map[string][]GraphEdge
	children map[string][]*node.Node
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

// EdgesFrom returns the result-use edges leaving id, with their origin kind.
// It is the read-only view folds and traces use to name an edge (child,
// reference dependency, validation dependency).
func (p *Provider) EdgesFrom(id types.NodeID) []GraphEdge {
	return p.edges[id.String()]
}

// ResultUseEdges builds the result-use graph over state plus the prospective
// overlay. Only result-use edges are present; local_assume targets are
// hypothesis-use and are therefore absent.
func ResultUseEdges(st State, overlay []ProspectiveNode) Provider {
	return resultUseEdges(newUniverse(st, overlay))
}

// ResultUseEdgesFromNodes builds the result-use graph from a bare node slice,
// with no state handle. It is what internal/taint uses to fold over the same
// prepared graph without importing internal/state.
func ResultUseEdgesFromNodes(nodes []*node.Node) Provider {
	return resultUseEdges(newUniverseFromNodes(nodes, nil))
}

// resultUseEdges builds the adjacency for an already-constructed universe.
func resultUseEdges(u *universe) Provider {
	p := Provider{
		deps:     make(map[string][]types.NodeID),
		nodes:    make(map[string]*node.Node),
		edges:    make(map[string][]GraphEdge),
		children: make(map[string][]*node.Node),
	}
	for id, info := range u.nodes {
		if info.node != nil {
			p.nodes[id] = info.node
		}
	}

	children := make(map[string][]*nodeInfo)
	for _, info := range u.nodes {
		// A node whose immediate parent ID is absent attaches to its nearest
		// present ancestor, the same rule the taint ancestor pass uses
		// (taint.nearestExistingParent), so the two components agree on what a
		// parent is even when the ID space has a hole.
		if parent, ok := u.effParent(info); ok {
			key := parent.String()
			children[key] = append(children[key], info)
		}
	}
	for parent, siblings := range children {
		sort.Slice(siblings, func(i, j int) bool { return siblings[i].id.Less(siblings[j].id) })
		// Retain the state-backed child pointers for folds (Current) that must
		// inspect children the result-use relation severs or excludes.
		for _, info := range siblings {
			if info.node != nil {
				p.children[parent] = append(p.children[parent], info.node)
			}
		}
	}

	for _, info := range u.nodes {
		edges := make([]types.NodeID, 0, len(info.deps)+len(info.valDeps))
		var gEdges []GraphEdge
		add := func(to types.NodeID, kind GraphEdgeKind) {
			edges = append(edges, to)
			gEdges = append(gEdges, GraphEdge{From: info.id, To: to, Kind: kind})
		}
		dangling := func(to types.NodeID) {
			if !u.exists(to) {
				p.dangling = append(p.dangling, DanglingDep{From: info.id, To: to})
			} else if u.isSevered(to) {
				p.dangling = append(p.dangling, DanglingDep{From: info.id, To: to, Severed: true})
			}
		}
		if !info.severed {
			// (i) children: a parent's proof is its children, whatever either
			// node's type (v3.2 amendment). A local_assume child is a step of
			// its parent's decomposition, and a local_assume's own children are
			// the derivation under the hypothesis, which the enclosing proof
			// relies on; excluding either direction hid an admitted step under
			// a hypothesis from the taint fold. Only the *hypothesis-use* edge
			// -- citing a local_assume as a dependency, clause (ii) -- carries
			// nothing.
			for _, c := range children[info.id.String()] {
				if c.severed {
					continue
				}
				add(c.id, EdgeChild)
			}
			// (ii) explicit dependencies, minus local_assume targets.
			for _, t := range info.deps {
				if u.isLocalAssume(t) {
					continue
				}
				add(t, EdgeDependency)
				dangling(t)
			}
			for _, t := range info.valDeps {
				if u.isLocalAssume(t) {
					continue
				}
				add(t, EdgeValidationDep)
				dangling(t)
			}
		}
		p.deps[info.id.String()] = dedupe(edges)
		p.edges[info.id.String()] = dedupeGraphEdges(gEdges)
	}

	for _, info := range u.nodes {
		p.order = append(p.order, info.id)
	}
	sort.Slice(p.order, func(i, j int) bool { return p.order[i].Less(p.order[j]) })
	return p
}

// dedupeGraphEdges drops later edges to the same target, keeping the first
// origin kind. Dependency target order is not meaningful (the content hash
// sorts), so this matches dedupe while carrying kind metadata.
func dedupeGraphEdges(edges []GraphEdge) []GraphEdge {
	if len(edges) <= 1 {
		return edges
	}
	seen := make(map[string]bool, len(edges))
	out := edges[:0]
	for _, e := range edges {
		key := e.To.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	return out
}

// DanglingDeps reports result-use dependencies on missing or severed nodes for
// the given state and overlay. It is a thin wrapper over ResultUseEdges for
// callers (e.g. audit) that only need the blockers.
func DanglingDeps(st State, overlay []ProspectiveNode) []DanglingDep {
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
	// node is the state-backed node pointer when this entry came from state;
	// nil for an overlay-only prospective node.
	node *node.Node
}

// universe is the node set over state + overlay, keyed by ID string.
type universe struct {
	nodes    map[string]*nodeInfo
	encl     map[string][]types.NodeID // context others see when citing a node
	ownEncl  map[string][]types.NodeID // context a node's own dependencies see
	enclDone bool
}

func newUniverse(st State, overlay []ProspectiveNode) *universe {
	if st == nil {
		return newUniverseFromNodes(nil, overlay)
	}
	return newUniverseFromNodes(st.AllNodes(), overlay)
}

func newUniverseFromNodes(nodes []*node.Node, overlay []ProspectiveNode) *universe {
	u := &universe{nodes: make(map[string]*nodeInfo)}
	for _, n := range nodes {
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
			node:      n,
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

// effParent returns the nearest present ancestor of a node: its immediate
// parent when that ID exists in the universe, otherwise the nearest ancestor
// that does. It mirrors taint.nearestExistingParent so the support graph's
// child edges and the taint ancestor chain agree on parenthood when an
// intermediate node is missing from the ledger. Reports false when no ancestor
// is present (the root, or a node whose whole chain is absent).
func (u *universe) effParent(info *nodeInfo) (types.NodeID, bool) {
	if !info.hasParent {
		return types.NodeID{}, false
	}
	id := info.parent
	for {
		if _, ok := u.nodes[id.String()]; ok {
			return id, true
		}
		next, ok := id.Parent()
		if !ok {
			return types.NodeID{}, false
		}
		id = next
	}
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
