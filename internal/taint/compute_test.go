package taint

import (
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// helper creates a minimal node for testing taint computation.
// Panics if id parsing fails (only use valid IDs in tests).
func makeTestNode(id string, epistemic schema.EpistemicState, taint node.TaintState) *node.Node {
	nodeID, err := types.Parse(id)
	if err != nil {
		panic("invalid test node ID: " + id)
	}
	return &node.Node{
		ID:             nodeID,
		Type:           schema.NodeTypeClaim,
		Statement:      "test statement",
		Inference:      schema.InferenceAssumption,
		WorkflowState:  schema.WorkflowAvailable,
		EpistemicState: epistemic,
		TaintState:     taint,
	}
}

func TestComputeTaint_ValidatedWithCleanAncestors(t *testing.T) {
	// A validated node with clean ancestors should be clean
	n := makeTestNode("1.1", schema.EpistemicValidated, node.TaintUnresolved)
	parent := makeTestNode("1", schema.EpistemicValidated, node.TaintClean)
	ancestors := []*node.Node{parent}

	result := ComputeTaint(n, ancestors)

	if result != node.TaintClean {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintClean)
	}
}

func TestComputeTaint_AdmittedNode(t *testing.T) {
	// An admitted node should have self_admitted taint
	n := makeTestNode("1.1", schema.EpistemicAdmitted, node.TaintUnresolved)
	parent := makeTestNode("1", schema.EpistemicValidated, node.TaintClean)
	ancestors := []*node.Node{parent}

	result := ComputeTaint(n, ancestors)

	if result != node.TaintSelfAdmitted {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintSelfAdmitted)
	}
}

func TestComputeTaint_NodeWithTaintedAncestor(t *testing.T) {
	// A node with a tainted ancestor should be tainted
	n := makeTestNode("1.1.1", schema.EpistemicValidated, node.TaintUnresolved)
	parent := makeTestNode("1.1", schema.EpistemicValidated, node.TaintClean)
	grandparent := makeTestNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	ancestors := []*node.Node{parent, grandparent}

	result := ComputeTaint(n, ancestors)

	if result != node.TaintTainted {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintTainted)
	}
}

func TestComputeTaint_PendingNode(t *testing.T) {
	// A pending node should be unresolved
	n := makeTestNode("1.1", schema.EpistemicPending, node.TaintUnresolved)
	parent := makeTestNode("1", schema.EpistemicValidated, node.TaintClean)
	ancestors := []*node.Node{parent}

	result := ComputeTaint(n, ancestors)

	if result != node.TaintUnresolved {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintUnresolved)
	}
}

func TestComputeTaint_AncestorOrderDoesNotMatter(t *testing.T) {
	// Ancestor order should not affect the result
	n := makeTestNode("1.1.1", schema.EpistemicValidated, node.TaintUnresolved)
	parent := makeTestNode("1.1", schema.EpistemicValidated, node.TaintClean)
	grandparent := makeTestNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	// Test with different orderings
	orderings := [][]*node.Node{
		{parent, grandparent},
		{grandparent, parent},
	}

	for i, ancestors := range orderings {
		result := ComputeTaint(n, ancestors)
		if result != node.TaintTainted {
			t.Errorf("ordering %d: ComputeTaint() = %v, want %v", i, result, node.TaintTainted)
		}
	}
}

func TestComputeTaint_NilAncestors(t *testing.T) {
	// Node with nil ancestors (root node validated) should be clean
	n := makeTestNode("1", schema.EpistemicValidated, node.TaintUnresolved)

	result := ComputeTaint(n, nil)

	if result != node.TaintClean {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintClean)
	}
}

func TestComputeTaint_EmptyAncestors(t *testing.T) {
	// Node with empty ancestors (root node validated) should be clean
	n := makeTestNode("1", schema.EpistemicValidated, node.TaintUnresolved)
	ancestors := []*node.Node{}

	result := ComputeTaint(n, ancestors)

	if result != node.TaintClean {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintClean)
	}
}

func TestComputeTaint_RootAdmitted(t *testing.T) {
	// Root node that is admitted should be self_admitted
	n := makeTestNode("1", schema.EpistemicAdmitted, node.TaintUnresolved)

	result := ComputeTaint(n, nil)

	if result != node.TaintSelfAdmitted {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintSelfAdmitted)
	}
}

func TestComputeTaint_RootPending(t *testing.T) {
	// Root node that is pending should be unresolved
	n := makeTestNode("1", schema.EpistemicPending, node.TaintUnresolved)

	result := ComputeTaint(n, nil)

	if result != node.TaintUnresolved {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintUnresolved)
	}
}

func TestComputeTaint_AncestorWithUnresolved(t *testing.T) {
	// Node with an unresolved ancestor should propagate unresolved
	n := makeTestNode("1.1", schema.EpistemicValidated, node.TaintUnresolved)
	parent := makeTestNode("1", schema.EpistemicPending, node.TaintUnresolved)
	ancestors := []*node.Node{parent}

	result := ComputeTaint(n, ancestors)

	if result != node.TaintUnresolved {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintUnresolved)
	}
}

func TestComputeTaint_MultipleAncestorsOneAdmitted(t *testing.T) {
	// If any ancestor is self_admitted, node should be tainted
	n := makeTestNode("1.1.1.1", schema.EpistemicValidated, node.TaintUnresolved)
	greatGrandparent := makeTestNode("1", schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	grandparent := makeTestNode("1.1", schema.EpistemicValidated, node.TaintTainted)
	parent := makeTestNode("1.1.1", schema.EpistemicValidated, node.TaintTainted)
	ancestors := []*node.Node{parent, grandparent, greatGrandparent}

	result := ComputeTaint(n, ancestors)

	if result != node.TaintTainted {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintTainted)
	}
}

func TestComputeTaint_RefutedNode(t *testing.T) {
	// A refuted node with clean ancestors should be clean (refuted doesn't introduce taint)
	n := makeTestNode("1.1", schema.EpistemicRefuted, node.TaintUnresolved)
	parent := makeTestNode("1", schema.EpistemicValidated, node.TaintClean)
	ancestors := []*node.Node{parent}

	result := ComputeTaint(n, ancestors)

	if result != node.TaintClean {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintClean)
	}
}

func TestComputeTaint_ArchivedNode(t *testing.T) {
	// An archived node with clean ancestors should be clean
	n := makeTestNode("1.1", schema.EpistemicArchived, node.TaintUnresolved)
	parent := makeTestNode("1", schema.EpistemicValidated, node.TaintClean)
	ancestors := []*node.Node{parent}

	result := ComputeTaint(n, ancestors)

	if result != node.TaintClean {
		t.Errorf("ComputeTaint() = %v, want %v", result, node.TaintClean)
	}
}

func TestComputeTaint_NilNode(t *testing.T) {
	// ComputeTaint should handle nil node gracefully by panicking
	// (defensive behavior - caller must provide valid node)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("ComputeTaint(nil, ...) should panic but did not")
		}
	}()

	ancestors := []*node.Node{}
	ComputeTaint(nil, ancestors)
}

func TestComputeTaint_NilAncestorsList(t *testing.T) {
	// Test that nil ancestors list is treated identically to empty slice
	// for all epistemic states. This documents that nil is a valid input
	// representing a root node with no ancestors.
	tests := []struct {
		name      string
		epistemic schema.EpistemicState
		want      node.TaintState
	}{
		{
			name:      "pending node with nil ancestors is unresolved",
			epistemic: schema.EpistemicPending,
			want:      node.TaintUnresolved,
		},
		{
			name:      "validated node with nil ancestors is clean",
			epistemic: schema.EpistemicValidated,
			want:      node.TaintClean,
		},
		{
			name:      "admitted node with nil ancestors is self_admitted",
			epistemic: schema.EpistemicAdmitted,
			want:      node.TaintSelfAdmitted,
		},
		{
			name:      "refuted node with nil ancestors is clean",
			epistemic: schema.EpistemicRefuted,
			want:      node.TaintClean,
		},
		{
			name:      "archived node with nil ancestors is clean",
			epistemic: schema.EpistemicArchived,
			want:      node.TaintClean,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := makeTestNode("1", tt.epistemic, node.TaintUnresolved)

			// Pass nil explicitly (not empty slice) to verify nil handling
			result := ComputeTaint(n, nil)

			if result != tt.want {
				t.Errorf("ComputeTaint() = %v, want %v", result, tt.want)
			}

			// Also verify nil behaves identically to empty slice
			resultEmpty := ComputeTaint(n, []*node.Node{})
			if result != resultEmpty {
				t.Errorf("nil vs empty mismatch: nil=%v, empty=%v", result, resultEmpty)
			}
		})
	}
}
