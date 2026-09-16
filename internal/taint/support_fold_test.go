package taint

import (
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func setDeps(t *testing.T, n *node.Node, deps ...string) {
	t.Helper()
	for _, d := range deps {
		id, err := types.Parse(d)
		if err != nil {
			t.Fatalf("parse %q: %v", d, err)
		}
		n.Dependencies = append(n.Dependencies, id)
	}
}

func setValDeps(t *testing.T, n *node.Node, deps ...string) {
	t.Helper()
	for _, d := range deps {
		id, err := types.Parse(d)
		if err != nil {
			t.Fatalf("parse %q: %v", d, err)
		}
		n.ValidationDeps = append(n.ValidationDeps, id)
	}
}

func TestSupportFold_DependencyCarriesTaintLikeChild(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	mid := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	admitted := makeNode("1.1.1", schema.EpistemicAdmitted, node.TaintClean)
	consumer := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	setDeps(t, consumer, "1.1.1")

	all := []*node.Node{root, mid, admitted, consumer}
	RecomputeAll(all)

	if consumer.TaintState != node.TaintTainted {
		t.Errorf("dependency consumer = %s, want tainted", consumer.TaintState)
	}
}

func TestSupportFold_ValidationDepCarriesTaint(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	pending := makeNode("1.1", schema.EpistemicPending, node.TaintClean)
	consumer := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	setValDeps(t, consumer, "1.1")

	all := []*node.Node{root, pending, consumer}
	RecomputeAll(all)

	if consumer.TaintState != node.TaintUnresolved {
		t.Errorf("validation-dep consumer = %s, want unresolved", consumer.TaintState)
	}
}

func TestSupportFold_SeveredDependencyUnresolved(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	severed := makeNode("1.1", schema.EpistemicArchived, node.TaintClean)
	consumer := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	setDeps(t, consumer, "1.1")

	all := []*node.Node{root, severed, consumer}
	RecomputeAll(all)

	if severed.TaintState != node.TaintClean {
		t.Errorf("severed node = %s, want clean", severed.TaintState)
	}
	if consumer.TaintState != node.TaintUnresolved {
		t.Errorf("consumer of a severed dependency = %s, want unresolved", consumer.TaintState)
	}
}

func TestSupportFold_MissingDependencyUnresolved(t *testing.T) {
	consumer := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	setDeps(t, consumer, "1.9")

	RecomputeAll([]*node.Node{consumer})

	if consumer.TaintState != node.TaintUnresolved {
		t.Errorf("consumer of a missing dependency = %s, want unresolved", consumer.TaintState)
	}
}

func TestSupportFold_HypothesisUseCarriesNothing(t *testing.T) {
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	assume := makeNode("1.1", schema.EpistemicPending, node.TaintClean)
	assume.Type = schema.NodeTypeLocalAssume
	consumer := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	setDeps(t, consumer, "1.1")

	all := []*node.Node{root, assume, consumer}
	RecomputeAll(all)

	if consumer.TaintState != node.TaintClean {
		t.Errorf("hypothesis-use consumer = %s, want clean (local_assume edge carries nothing)", consumer.TaintState)
	}
}

func TestSupportFold_LegacyCycleUnresolved(t *testing.T) {
	a := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	b := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)
	setDeps(t, a, "1.2")
	setDeps(t, b, "1.1")

	RecomputeAll([]*node.Node{a, b})

	if a.TaintState != node.TaintUnresolved || b.TaintState != node.TaintUnresolved {
		t.Errorf("cycle members = %s/%s, want unresolved/unresolved", a.TaintState, b.TaintState)
	}
}

func TestSupportFold_ValidatedDependencyChain(t *testing.T) {
	// A validated node that cites a validated node which itself cites a pending
	// node is unresolved transitively through dependencies, not only through
	// the hierarchical tree.
	mid := makeNode("1.1", schema.EpistemicValidated, node.TaintClean)
	setDeps(t, mid, "1.2")
	deep := makeNode("1.2", schema.EpistemicPending, node.TaintClean)
	top := makeNode("1.3", schema.EpistemicValidated, node.TaintClean)
	setDeps(t, top, "1.1")

	RecomputeAll([]*node.Node{mid, deep, top})

	if mid.TaintState != node.TaintUnresolved {
		t.Errorf("mid = %s, want unresolved", mid.TaintState)
	}
	if top.TaintState != node.TaintUnresolved {
		t.Errorf("top = %s, want unresolved (transitive through dependency)", top.TaintState)
	}
}

func TestSupportFold_AncestorSeparation(t *testing.T) {
	// A validated sibling of an admitted node stays clean: result-derived taint
	// never crosses to a sibling and the ancestor chain is clean.
	root := makeNode("1", schema.EpistemicValidated, node.TaintClean)
	admitted := makeNode("1.1", schema.EpistemicAdmitted, node.TaintClean)
	sibling := makeNode("1.2", schema.EpistemicValidated, node.TaintClean)

	all := []*node.Node{root, admitted, sibling}
	RecomputeAll(all)

	if sibling.TaintState != node.TaintClean {
		t.Errorf("validated sibling of admitted node = %s, want clean", sibling.TaintState)
	}
}
