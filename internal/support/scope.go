package support

import (
	"sort"

	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// computeScopes derives, for every node in the universe, the list of
// local_assume IDs whose scope encloses it. Scope is structural: a
// local_assume opens a scope over its own subtree and over its later siblings
// in the same child list, and a local_discharge closes the innermost scope it
// is allowed to close. A scope opened inside a node's child list is closed when
// that child list ends, so an undischarged assumption nested under one branch
// never leaks into a later sibling branch.
//
// A local_discharge may close the innermost open scope only when that scope
// either structurally encloses the discharge (the discharge is one of its
// descendants; this is the docs/concepts.md form) or was opened in the same
// child list as the discharge (a sibling discharge; the plan form). A discharge
// in one branch therefore cannot close a foreign scope opened in another
// branch.
//
// Two views are recorded because a discharge sits on the boundary:
//
//   - ownEncl is the context a node's own dependencies are checked against. For
//     a local_discharge this is the pre-close context, so it may cite a premise
//     from inside the assumption it closes.
//   - encl is the context every other node sees when it cites the node. For a
//     local_discharge this is the post-close context, so a later sibling can
//     safely cite the discharge's result.
func (u *universe) computeScopes() {
	u.encl = make(map[string][]types.NodeID)
	u.ownEncl = make(map[string][]types.NodeID)
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

	// scopeEntry is one open local_assume. ancestor is true while the
	// assumption structurally encloses the node being visited (that is, we are
	// inside its subtree); it becomes false once we move to its later siblings,
	// whose subtrees may use the scope but may not close it. listID identifies
	// the child list that opened the scope so a discharge can tell a
	// same-list sibling scope from a foreign one.
	type scopeEntry struct {
		id       types.NodeID
		ancestor bool
		listID   int
	}

	var stack []scopeEntry
	nextListID := 0

	snapshot := func() []types.NodeID {
		out := make([]types.NodeID, len(stack))
		for i := range stack {
			out[i] = stack[i].id
		}
		return out
	}

	var visitList func(children []*nodeInfo)
	visitList = func(children []*nodeInfo) {
		listID := nextListID
		nextListID++

		for _, c := range children {
			pre := snapshot()
			u.ownEncl[c.id.String()] = pre

			switch c.typ {
			case schema.NodeTypeLocalDischarge:
				// Pop only the innermost scope that encloses this discharge or
				// was opened in this very child list.
				if n := len(stack); n > 0 {
					top := stack[n-1]
					if top.ancestor || top.listID == listID {
						stack = stack[:n-1]
					}
				}
				u.encl[c.id.String()] = snapshot()
				visitList(u.childrenOf(c.id))
			case schema.NodeTypeLocalAssume:
				u.encl[c.id.String()] = pre
				stack = append(stack, scopeEntry{id: c.id, ancestor: true, listID: listID})
				visitList(u.childrenOf(c.id))
				// Keep the scope open over later siblings in this list, but
				// those siblings' subtrees may not close it.
				if n := len(stack); n > 0 && stack[n-1].id.String() == c.id.String() {
					stack[n-1].ancestor = false
				}
			default:
				u.encl[c.id.String()] = pre
				visitList(u.childrenOf(c.id))
			}
		}

		// Close every scope this child list opened. Those entries are the
		// topmost survivors, so drop the contiguous top run bearing this list's
		// id; scopes inherited from ancestors are left untouched.
		n := len(stack)
		for n > 0 && stack[n-1].listID == listID {
			n--
		}
		stack = stack[:n]
	}

	visitList(roots)
}

// enclosingAssumptions returns the sorted local_assume IDs whose scope encloses
// n from the point of view of another node citing n (post-close for a
// local_discharge).
func enclosingAssumptions(u *universe, n types.NodeID) []types.NodeID {
	if _, ok := u.nodes[n.String()]; !ok {
		return nil
	}
	if !u.enclDone {
		u.computeScopes()
	}
	return u.encl[n.String()]
}

// ownEnclosingAssumptions returns the local_assume IDs in scope for n's own
// dependencies (pre-close for a local_discharge).
func ownEnclosingAssumptions(u *universe, n types.NodeID) []types.NodeID {
	if _, ok := u.nodes[n.String()]; !ok {
		return nil
	}
	if !u.enclDone {
		u.computeScopes()
	}
	return u.ownEncl[n.String()]
}

// checkScope rejects a prospective node n that result-uses a node inside a
// local_assume scope which does not also enclose n. A dependency on a
// local_assume node is hypothesis-use and is allowed exactly when that
// assumption encloses n.
func checkScope(u *universe, pn ProspectiveNode) error {
	enclosingN := make(map[string]bool)
	for _, a := range ownEnclosingAssumptions(u, pn.ID) {
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
