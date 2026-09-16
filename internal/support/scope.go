package support

import (
	"sort"

	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// computeScopes derives, for every node in the universe, the list of
// local_assume IDs whose scope encloses it. Scope is structural: a
// local_assume opens a scope over its own subtree and over its later siblings,
// and a local_discharge closes the innermost open scope. A discharge inside the
// local_assume's own subtree therefore also closes it for later siblings.
//
// The traversal is a preorder walk of the (single) node tree; the stack holds
// the scopes currently open. A node's enclosing set is the stack snapshot at
// entry, so a local_assume is not inside its own scope and a discharge is
// inside the scope it closes (it is a descendant of the assumption).
func (u *universe) computeScopes() {
	u.encl = make(map[string][]types.NodeID)
	u.enclDone = true

	var roots []*nodeInfo
	for _, info := range u.nodes {
		if !info.hasParent {
			roots = append(roots, info)
			continue
		}
		if _, ok := u.nodes[info.parent.String()]; !ok {
			roots = append(roots, info)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].id.Less(roots[j].id) })

	var stack []types.NodeID
	var visit func(info *nodeInfo)
	visit = func(info *nodeInfo) {
		snapshot := make([]types.NodeID, len(stack))
		copy(snapshot, stack)
		u.encl[info.id.String()] = snapshot

		switch info.typ {
		case schema.NodeTypeLocalDischarge:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case schema.NodeTypeLocalAssume:
			stack = append(stack, info.id)
			// A local_assume's scope stays open over its later siblings until a
			// discharge; do not pop after its subtree.
		}

		for _, c := range u.childrenOf(info.id) {
			visit(c)
		}
	}
	for _, r := range roots {
		visit(r)
	}
}

// enclosingAssumptions returns the sorted local_assume IDs whose scope encloses n.
func enclosingAssumptions(u *universe, n types.NodeID) []types.NodeID {
	if _, ok := u.nodes[n.String()]; !ok {
		return nil
	}
	if !u.enclDone {
		u.computeScopes()
	}
	return u.encl[n.String()]
}

// checkScope rejects a prospective node n that result-uses a node inside a
// local_assume scope which does not also enclose n. A dependency on a
// local_assume node is hypothesis-use and is allowed exactly when that
// assumption encloses n.
func checkScope(u *universe, pn ProspectiveNode) error {
	enclosingN := make(map[string]bool)
	for _, a := range enclosingAssumptions(u, pn.ID) {
		enclosingN[a.String()] = true
	}

	deps := make([]types.NodeID, 0, len(pn.Dependencies)+len(pn.ValidationDeps))
	deps = append(deps, pn.Dependencies...)
	deps = append(deps, pn.ValidationDeps...)

	for _, t := range deps {
		if u.isLocalAssume(t) {
			if !enclosingN[t.String()] {
				return newScopeLeakError(pn.ID, t, t)
			}
			continue
		}
		for _, a := range enclosingAssumptions(u, t) {
			if !enclosingN[a.String()] {
				return newScopeLeakError(pn.ID, t, a)
			}
		}
	}
	return nil
}
