package support

import (
	"errors"
	"sort"
	"strings"

	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// ErrCycle is the sentinel matched by CycleError via errors.Is.
var ErrCycle = aferrors.New(aferrors.DEPENDENCY_CYCLE, "circular dependency detected")

// ErrScopeLeak is the sentinel matched by ScopeLeakError via errors.Is.
var ErrScopeLeak = aferrors.New(aferrors.SCOPE_LEAK, "scope leak")

// CycleError reports that the prospective node Source would create a result-use
// cycle through Path. The offending IDs are fields, not only message text.
type CycleError struct {
	Source types.NodeID
	Path   []types.NodeID
	af     *aferrors.AFError
}

func (e *CycleError) Error() string { return e.af.Error() }

// Unwrap exposes the underlying AFError so errors.Is / aferrors.Code work.
func (e *CycleError) Unwrap() error { return e.af }

// ScopeLeakError reports that prospective node Node result-uses Dep, which is
// inside the local assumption Assumption whose scope does not enclose Node.
type ScopeLeakError struct {
	Node       types.NodeID
	Dep        types.NodeID
	Assumption types.NodeID
	af         *aferrors.AFError
}

func (e *ScopeLeakError) Error() string { return e.af.Error() }

// Unwrap exposes the underlying AFError so errors.Is / aferrors.Code work.
func (e *ScopeLeakError) Unwrap() error { return e.af }

func newCycleError(source types.NodeID, path []types.NodeID) *CycleError {
	return &CycleError{
		Source: source,
		Path:   path,
		af: aferrors.Newf(aferrors.DEPENDENCY_CYCLE, "circular dependency detected: %s",
			strings.Join(types.ToStringSlice(path), " -> ")),
	}
}

func newScopeLeakError(node, dep, assumption types.NodeID) *ScopeLeakError {
	return &ScopeLeakError{
		Node:       node,
		Dep:        dep,
		Assumption: assumption,
		af: aferrors.Newf(aferrors.SCOPE_LEAK,
			"scope leak: %s cites %s which is inside the local assumption %s",
			node.String(), dep.String(), assumption.String()),
	}
}

// CheckCreation validates a complete prospective batch against one state read:
//
//	(a) a result-use cycle check that rejects only cycles containing an edge
//	    NEW relative to the pre-overlay graph, so a removal-only amendment that
//	    leaves an unrelated legacy cycle in state is accepted;
//	(b) a scope check rejecting a result-use dependency on a node inside a
//	    local_assume scope that does not also enclose the citing node;
//	(c) a dependency on a local_assume node is hypothesis-use: allowed only
//	    when that assumption encloses the citing node;
//	(d) re-validation of every pre-existing node whose enclosing-assumption set
//	    changed because of the overlay, since such a node's existing
//	    hypothesis-use may have been orphaned by, e.g., an inserted discharge.
//
// The batch is checked as a whole: a cycle that only exists between two nodes
// of the batch is found.
func CheckCreation(st State, batch []ProspectiveNode) error {
	preUniverse := newUniverse(st, nil)
	preProvider := resultUseEdges(preUniverse)

	u := newUniverse(st, batch)
	provider := resultUseEdges(u)

	if err := checkNewCycles(&provider, &preProvider, batch); err != nil {
		return err
	}
	for i := range batch {
		if err := checkScope(u, batch[i]); err != nil {
			return err
		}
	}

	// Re-validate pre-existing nodes whose own scope changed. A prospective
	// discharge inserted between an assumption and an existing later sibling
	// moves that sibling out of scope without the sibling itself appearing in
	// the batch; its existing hypothesis-use must be re-checked.
	preUniverse.computeScopes()
	if !u.enclDone {
		u.computeScopes()
	}
	inBatch := make(map[string]bool, len(batch))
	for i := range batch {
		inBatch[batch[i].ID.String()] = true
	}
	ids := make([]string, 0, len(u.nodes))
	for id := range u.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		info := u.nodes[id]
		if !info.exists || inBatch[id] {
			continue
		}
		if sameIDs(preUniverse.ownEncl[id], u.ownEncl[id]) {
			continue
		}
		pn := ProspectiveNode{
			ID:             info.id,
			Type:           info.typ,
			Dependencies:   info.deps,
			ValidationDeps: info.valDeps,
		}
		if info.hasParent {
			pn.ParentID = info.parent
		}
		if err := checkScope(u, pn); err != nil {
			return err
		}
	}
	return nil
}

// ScopeLeaks reports every existing node whose recorded dependencies violate
// the scope rule, reusing checkScope over the state-backed graph. It is the
// read-only audit counterpart of CheckCreation: it reports every offending node
// instead of failing on the first, and it never rejects the workspace. Results
// are ordered by citing node ID.
func ScopeLeaks(st State) []ScopeLeakError {
	if st == nil {
		return nil
	}
	u := newUniverse(st, nil)
	u.computeScopes()

	ids := make([]string, 0, len(u.nodes))
	for id := range u.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var leaks []ScopeLeakError
	for _, id := range ids {
		info := u.nodes[id]
		if !info.exists {
			continue
		}
		pn := ProspectiveNode{
			ID:             info.id,
			Type:           info.typ,
			Dependencies:   info.deps,
			ValidationDeps: info.valDeps,
		}
		if info.hasParent {
			pn.ParentID = info.parent
		}
		if err := checkScope(u, pn); err != nil {
			var leak *ScopeLeakError
			if errors.As(err, &leak) {
				leaks = append(leaks, *leak)
			}
		}
	}
	return leaks
}

// checkNewCycles rejects a batch only when the overlay actually adds an edge
// that closes a cycle. Legacy cycles already present in the pre-overlay graph
// are ignored, which makes a removal-only amendment (or one that merely keeps a
// path into a legacy cycle) acceptable.
//
// It does so edge-wise: for every edge present in the post-overlay graph but
// absent from the pre-overlay graph, if the edge's target can already reach its
// source in the post-overlay graph, that new edge closes a cycle.
func checkNewCycles(post, pre *Provider, batch []ProspectiveNode) error {
	type edge struct{ from, to types.NodeID }

	preSet := make(map[string]map[string]bool, len(pre.deps))
	for from, deps := range pre.deps {
		set := make(map[string]bool, len(deps))
		for _, to := range deps {
			set[to.String()] = true
		}
		preSet[from] = set
	}

	var newEdges []edge
	for _, from := range post.order {
		for _, to := range post.deps[from.String()] {
			if preSet[from.String()][to.String()] {
				continue
			}
			newEdges = append(newEdges, edge{from: from, to: to})
		}
	}
	sort.Slice(newEdges, func(i, j int) bool {
		if !newEdges[i].from.Equal(newEdges[j].from) {
			return newEdges[i].from.Less(newEdges[j].from)
		}
		return newEdges[i].to.Less(newEdges[j].to)
	})

	inBatch := make(map[string]bool, len(batch))
	for i := range batch {
		inBatch[batch[i].ID.String()] = true
	}

	for _, e := range newEdges {
		tail, ok := reachPath(post, e.to, e.from)
		if !ok {
			continue
		}
		cycle := append([]types.NodeID{e.from}, tail...)
		source := e.from
		for _, id := range cycle {
			if inBatch[id.String()] {
				source = id
				break
			}
		}
		return newCycleError(source, cycle)
	}
	return nil
}

// reachPath returns a path from start to goal inclusive, following post's
// result-use edges, and whether goal is reachable. The search is
// deterministic: dependencies are visited in node order.
func reachPath(post *Provider, start, goal types.NodeID) ([]types.NodeID, bool) {
	if start.String() == goal.String() {
		return []types.NodeID{start}, true
	}
	visited := map[string]bool{start.String(): true}
	var path []types.NodeID
	var dfs func(id types.NodeID) bool
	dfs = func(id types.NodeID) bool {
		if id.String() == goal.String() {
			path = append(path, id)
			return true
		}
		deps, ok := post.GetNodeDependencies(id)
		if !ok {
			return false
		}
		sorted := append([]types.NodeID(nil), deps...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Less(sorted[j]) })
		for _, dep := range sorted {
			if visited[dep.String()] {
				continue
			}
			visited[dep.String()] = true
			if dfs(dep) {
				path = append([]types.NodeID{id}, path...)
				return true
			}
		}
		return false
	}
	if dfs(start) {
		return path, true
	}
	return nil, false
}

// sameIDs reports whether two ID slices are equal elementwise.
func sameIDs(a, b []types.NodeID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(b[i]) {
			return false
		}
	}
	return true
}
