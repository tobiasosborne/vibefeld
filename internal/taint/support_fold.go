package taint

import (
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/support"
)

// supportValue is the fold result for one node: the support component it
// contributes to its dependents, plus whether the node is severed. Severed is
// carried separately because a severed *dependency* target is unresolved for
// the citing node (rule 4), while a severed *child* is simply absent from the
// result-use graph and contributes nothing (rule 1). Child edges never point at
// severed nodes, so any severed target the fold sees is a dependency or
// validation dependency.
type supportValue struct {
	comp    taintComponent
	severed bool
}

// foldSupport is the D6 support-component fold over the prepared result-use
// graph. It runs in dependency-topological order via support.Walk, so each
// target's own support component is already computed when the citing node is
// folded. The rules, in order:
//
//  1. severed children contribute nothing (they are not edges in the graph);
//  2. an admitted target contributes tainted and is not descended;
//  3. a pending/draft/needs_refinement target contributes unresolved;
//  4. a severed or missing dependency target contributes unresolved;
//  5. a validated target contributes its own support component.
//
// The value returned for n is the component n contributes to its dependents,
// which is the aggregate of its targets with n's own epistemic state applied
// first (the per-node precedence own severed -> own admitted -> unresolved).
// Reference and validation dependencies now carry taint exactly like children.
// Hypothesis-use edges (local_assume targets) are absent from the result-use
// graph, so they carry nothing.
func foldSupport(n *node.Node, targets []support.Folded[supportValue]) supportValue {
	if isSevered(n) {
		return supportValue{comp: componentClean, severed: true}
	}
	if isUnresolvedState(n.EpistemicState) {
		return supportValue{comp: componentUnresolved}
	}
	if schema.IntroducesTaint(n.EpistemicState) {
		return supportValue{comp: componentTainted}
	}

	agg := componentClean
	for _, t := range targets {
		// Missing dependency target or a legacy result-use cycle: unresolved.
		if t.Cycle || t.Missing {
			agg = combineComponents(agg, componentUnresolved)
			continue
		}
		// A severed node can only be reachable as a dependency here (severed
		// children are not result-use edges): rule 4.
		if t.Value.severed {
			agg = combineComponents(agg, componentUnresolved)
			continue
		}
		agg = combineComponents(agg, t.Value.comp)
	}
	return supportValue{comp: agg}
}

// supportComponents prepares one result-use graph over allNodes and folds the
// D6 support component for every node. It is the single DAG walk D4 and D6
// share (v3.1 amendment 4): the same support.Prepare/Walk seam, one fold each.
func supportComponents(allNodes []*node.Node) map[string]supportValue {
	g := support.Prepare(support.ResultUseEdgesFromNodes(allNodes))
	return support.Walk(g, foldSupport)
}
