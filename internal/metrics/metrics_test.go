// Package metrics provides quality metrics for proofs.
package metrics

import (
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createTestNode creates a test node with given ID and options.
func createTestNode(t *testing.T, idStr string, opts ...func(*node.Node)) *node.Node {
	t.Helper()
	id, err := types.Parse(idStr)
	if err != nil {
		t.Fatalf("failed to parse node ID %q: %v", idStr, err)
	}
	n, err := node.NewNode(id, schema.NodeTypeClaim, "test statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

// createTestState creates a test state with optional nodes.
func createTestState(t *testing.T, nodes ...*node.Node) *state.State {
	t.Helper()
	st := state.NewState()
	for _, n := range nodes {
		st.AddNode(n)
	}
	return st
}

// =============================================================================
// RefinementDepth Tests
// =============================================================================

func TestRefinementDepth_SingleNode(t *testing.T) {
	// Root node only - depth is 1
	root := createTestNode(t, "1")
	st := createTestState(t, root)

	depth := RefinementDepth(st, root.ID)
	if depth != 1 {
		t.Errorf("RefinementDepth for single root node: got %d, want 1", depth)
	}
}

func TestRefinementDepth_LinearChain(t *testing.T) {
	// Linear chain: 1 -> 1.1 -> 1.1.1 -> 1.1.1.1 (depth 4)
	root := createTestNode(t, "1")
	child1 := createTestNode(t, "1.1")
	child2 := createTestNode(t, "1.1.1")
	child3 := createTestNode(t, "1.1.1.1")
	st := createTestState(t, root, child1, child2, child3)

	depth := RefinementDepth(st, root.ID)
	if depth != 4 {
		t.Errorf("RefinementDepth for linear chain: got %d, want 4", depth)
	}
}

func TestRefinementDepth_BranchingTree(t *testing.T) {
	// Branching tree:
	//       1
	//      / \
	//    1.1  1.2
	//    /      \
	//  1.1.1   1.2.1
	//           |
	//         1.2.1.1
	// Max depth from root is 4 (1 -> 1.2 -> 1.2.1 -> 1.2.1.1)
	root := createTestNode(t, "1")
	child11 := createTestNode(t, "1.1")
	child12 := createTestNode(t, "1.2")
	child111 := createTestNode(t, "1.1.1")
	child121 := createTestNode(t, "1.2.1")
	child1211 := createTestNode(t, "1.2.1.1")
	st := createTestState(t, root, child11, child12, child111, child121, child1211)

	depth := RefinementDepth(st, root.ID)
	if depth != 4 {
		t.Errorf("RefinementDepth for branching tree: got %d, want 4", depth)
	}
}

func TestRefinementDepth_Subtree(t *testing.T) {
	// Test depth from a subtree node
	root := createTestNode(t, "1")
	child12 := createTestNode(t, "1.2")
	child121 := createTestNode(t, "1.2.1")
	child1211 := createTestNode(t, "1.2.1.1")
	st := createTestState(t, root, child12, child121, child1211)

	// From 1.2, depth should be 3 (1.2 -> 1.2.1 -> 1.2.1.1)
	depth := RefinementDepth(st, child12.ID)
	if depth != 3 {
		t.Errorf("RefinementDepth from subtree: got %d, want 3", depth)
	}
}

func TestRefinementDepth_NonExistentNode(t *testing.T) {
	st := createTestState(t)
	id, _ := types.Parse("1.999")

	depth := RefinementDepth(st, id)
	if depth != 0 {
		t.Errorf("RefinementDepth for non-existent node: got %d, want 0", depth)
	}
}

// =============================================================================
// MaxRefinementDepth Tests
// =============================================================================

func TestMaxRefinementDepth_EmptyState(t *testing.T) {
	st := createTestState(t)

	depth := MaxRefinementDepth(st)
	if depth != 0 {
		t.Errorf("MaxRefinementDepth for empty state: got %d, want 0", depth)
	}
}

func TestMaxRefinementDepth_SingleNode(t *testing.T) {
	root := createTestNode(t, "1")
	st := createTestState(t, root)

	depth := MaxRefinementDepth(st)
	if depth != 1 {
		t.Errorf("MaxRefinementDepth for single node: got %d, want 1", depth)
	}
}

func TestMaxRefinementDepth_MultipleRoots(t *testing.T) {
	// This shouldn't happen in practice (only one root "1") but test the logic
	root := createTestNode(t, "1")
	child := createTestNode(t, "1.1")
	grandchild := createTestNode(t, "1.1.1")
	st := createTestState(t, root, child, grandchild)

	depth := MaxRefinementDepth(st)
	if depth != 3 {
		t.Errorf("MaxRefinementDepth: got %d, want 3", depth)
	}
}

// =============================================================================
// ChallengeDensity Tests
// =============================================================================

func TestChallengeDensity_NoNodes(t *testing.T) {
	st := createTestState(t)

	density := ChallengeDensity(st)
	if density != 0 {
		t.Errorf("ChallengeDensity for empty state: got %f, want 0", density)
	}
}

func TestChallengeDensity_NoChallenges(t *testing.T) {
	root := createTestNode(t, "1")
	child := createTestNode(t, "1.1")
	st := createTestState(t, root, child)

	density := ChallengeDensity(st)
	if density != 0 {
		t.Errorf("ChallengeDensity with no challenges: got %f, want 0", density)
	}
}

func TestChallengeDensity_OneChallengePerNode(t *testing.T) {
	root := createTestNode(t, "1")
	child := createTestNode(t, "1.1")
	st := createTestState(t, root, child)

	// Add 2 challenges for 2 nodes -> density = 1.0
	st.AddChallenge(&state.Challenge{ID: "c1", NodeID: root.ID, Status: "open"})
	st.AddChallenge(&state.Challenge{ID: "c2", NodeID: child.ID, Status: "open"})

	density := ChallengeDensity(st)
	if density != 1.0 {
		t.Errorf("ChallengeDensity: got %f, want 1.0", density)
	}
}

func TestChallengeDensity_MultipleChallengesPerNode(t *testing.T) {
	root := createTestNode(t, "1")
	st := createTestState(t, root)

	// Add 3 challenges for 1 node -> density = 3.0
	st.AddChallenge(&state.Challenge{ID: "c1", NodeID: root.ID, Status: "open"})
	st.AddChallenge(&state.Challenge{ID: "c2", NodeID: root.ID, Status: "resolved"})
	st.AddChallenge(&state.Challenge{ID: "c3", NodeID: root.ID, Status: "open"})

	density := ChallengeDensity(st)
	if density != 3.0 {
		t.Errorf("ChallengeDensity: got %f, want 3.0", density)
	}
}

// =============================================================================
// OpenChallengeDensity Tests
// =============================================================================

func TestOpenChallengeDensity_NoNodes(t *testing.T) {
	st := createTestState(t)

	density := OpenChallengeDensity(st)
	if density != 0 {
		t.Errorf("OpenChallengeDensity for empty state: got %f, want 0", density)
	}
}

func TestOpenChallengeDensity_OnlyOpenChallenges(t *testing.T) {
	root := createTestNode(t, "1")
	child := createTestNode(t, "1.1")
	st := createTestState(t, root, child)

	// Add 2 open challenges for 2 nodes
	st.AddChallenge(&state.Challenge{ID: "c1", NodeID: root.ID, Status: "open"})
	st.AddChallenge(&state.Challenge{ID: "c2", NodeID: child.ID, Status: "open"})

	density := OpenChallengeDensity(st)
	if density != 1.0 {
		t.Errorf("OpenChallengeDensity: got %f, want 1.0", density)
	}
}

func TestOpenChallengeDensity_MixedChallenges(t *testing.T) {
	root := createTestNode(t, "1")
	child := createTestNode(t, "1.1")
	st := createTestState(t, root, child)

	// Add 1 open, 2 resolved challenges -> open density = 0.5
	st.AddChallenge(&state.Challenge{ID: "c1", NodeID: root.ID, Status: "open"})
	st.AddChallenge(&state.Challenge{ID: "c2", NodeID: root.ID, Status: "resolved"})
	st.AddChallenge(&state.Challenge{ID: "c3", NodeID: child.ID, Status: "resolved"})

	density := OpenChallengeDensity(st)
	if density != 0.5 {
		t.Errorf("OpenChallengeDensity: got %f, want 0.5", density)
	}
}

// =============================================================================
// DefinitionCoverage Tests
// =============================================================================

func TestDefinitionCoverage_NoNodes(t *testing.T) {
	st := createTestState(t)

	coverage := DefinitionCoverage(st)
	if coverage != 1.0 {
		t.Errorf("DefinitionCoverage for empty state: got %f, want 1.0", coverage)
	}
}

func TestDefinitionCoverage_NoContextReferences(t *testing.T) {
	root := createTestNode(t, "1")
	st := createTestState(t, root)

	coverage := DefinitionCoverage(st)
	if coverage != 1.0 {
		t.Errorf("DefinitionCoverage with no context refs: got %f, want 1.0", coverage)
	}
}

func TestDefinitionCoverage_AllDefined(t *testing.T) {
	root := createTestNode(t, "1", func(n *node.Node) {
		n.Context = []string{"def:foo", "def:bar"}
	})
	st := createTestState(t, root)

	// Add definitions for both terms
	def1 := &node.Definition{ID: "def1", Name: "foo", Content: "foo definition"}
	def2 := &node.Definition{ID: "def2", Name: "bar", Content: "bar definition"}
	st.AddDefinition(def1)
	st.AddDefinition(def2)

	coverage := DefinitionCoverage(st)
	if coverage != 1.0 {
		t.Errorf("DefinitionCoverage with all defined: got %f, want 1.0", coverage)
	}
}

func TestDefinitionCoverage_PartiallyDefined(t *testing.T) {
	root := createTestNode(t, "1", func(n *node.Node) {
		n.Context = []string{"def:foo", "def:bar"}
	})
	st := createTestState(t, root)

	// Add definition for only one term
	def1 := &node.Definition{ID: "def1", Name: "foo", Content: "foo definition"}
	st.AddDefinition(def1)

	coverage := DefinitionCoverage(st)
	if coverage != 0.5 {
		t.Errorf("DefinitionCoverage partially defined: got %f, want 0.5", coverage)
	}
}

func TestDefinitionCoverage_NoDefined(t *testing.T) {
	root := createTestNode(t, "1", func(n *node.Node) {
		n.Context = []string{"def:foo", "def:bar"}
	})
	st := createTestState(t, root)

	coverage := DefinitionCoverage(st)
	if coverage != 0.0 {
		t.Errorf("DefinitionCoverage with none defined: got %f, want 0.0", coverage)
	}
}

func TestDefinitionCoverage_IgnoresNonDefRefs(t *testing.T) {
	root := createTestNode(t, "1", func(n *node.Node) {
		n.Context = []string{"def:foo", "ext:somebook", "assume:a1"}
	})
	st := createTestState(t, root)

	// Only the def:foo reference counts
	coverage := DefinitionCoverage(st)
	if coverage != 0.0 {
		t.Errorf("DefinitionCoverage ignoring non-def refs: got %f, want 0.0", coverage)
	}

	// Add the definition
	def1 := &node.Definition{ID: "def1", Name: "foo", Content: "foo definition"}
	st.AddDefinition(def1)

	coverage = DefinitionCoverage(st)
	if coverage != 1.0 {
		t.Errorf("DefinitionCoverage with def added: got %f, want 1.0", coverage)
	}
}

// =============================================================================
// OverallQuality Tests
// =============================================================================

func TestOverallQuality_EmptyState(t *testing.T) {
	st := createTestState(t)

	report := OverallQuality(st)

	if report.NodeCount != 0 {
		t.Errorf("NodeCount: got %d, want 0", report.NodeCount)
	}
	if report.MaxDepth != 0 {
		t.Errorf("MaxDepth: got %d, want 0", report.MaxDepth)
	}
	if report.ChallengeDensity != 0 {
		t.Errorf("ChallengeDensity: got %f, want 0", report.ChallengeDensity)
	}
	if report.DefinitionCoverage != 1.0 {
		t.Errorf("DefinitionCoverage: got %f, want 1.0", report.DefinitionCoverage)
	}
}

func TestOverallQuality_WithData(t *testing.T) {
	root := createTestNode(t, "1", func(n *node.Node) {
		n.Context = []string{"def:foo"}
		n.EpistemicState = schema.EpistemicValidated
	})
	child := createTestNode(t, "1.1", func(n *node.Node) {
		n.EpistemicState = schema.EpistemicPending
	})
	grandchild := createTestNode(t, "1.1.1", func(n *node.Node) {
		n.EpistemicState = schema.EpistemicPending
	})
	st := createTestState(t, root, child, grandchild)

	// Add a definition
	def1 := &node.Definition{ID: "def1", Name: "foo", Content: "foo definition"}
	st.AddDefinition(def1)

	// Add some challenges
	st.AddChallenge(&state.Challenge{ID: "c1", NodeID: root.ID, Status: "open"})
	st.AddChallenge(&state.Challenge{ID: "c2", NodeID: child.ID, Status: "resolved"})

	report := OverallQuality(st)

	if report.NodeCount != 3 {
		t.Errorf("NodeCount: got %d, want 3", report.NodeCount)
	}
	if report.MaxDepth != 3 {
		t.Errorf("MaxDepth: got %d, want 3", report.MaxDepth)
	}
	if report.TotalChallenges != 2 {
		t.Errorf("TotalChallenges: got %d, want 2", report.TotalChallenges)
	}
	if report.OpenChallenges != 1 {
		t.Errorf("OpenChallenges: got %d, want 1", report.OpenChallenges)
	}
	if report.ValidatedNodes != 1 {
		t.Errorf("ValidatedNodes: got %d, want 1", report.ValidatedNodes)
	}
	if report.PendingNodes != 2 {
		t.Errorf("PendingNodes: got %d, want 2", report.PendingNodes)
	}
	if report.DefinitionCoverage != 1.0 {
		t.Errorf("DefinitionCoverage: got %f, want 1.0", report.DefinitionCoverage)
	}
}

func TestOverallQuality_SubtreeMetrics(t *testing.T) {
	root := createTestNode(t, "1")
	child11 := createTestNode(t, "1.1")
	child12 := createTestNode(t, "1.2")
	child121 := createTestNode(t, "1.2.1")
	st := createTestState(t, root, child11, child12, child121)

	st.AddChallenge(&state.Challenge{ID: "c1", NodeID: child12.ID, Status: "open"})
	st.AddChallenge(&state.Challenge{ID: "c2", NodeID: child121.ID, Status: "open"})

	// Get metrics for subtree rooted at 1.2
	report := SubtreeQuality(st, child12.ID)

	if report.NodeCount != 2 {
		t.Errorf("Subtree NodeCount: got %d, want 2", report.NodeCount)
	}
	if report.MaxDepth != 2 {
		t.Errorf("Subtree MaxDepth: got %d, want 2", report.MaxDepth)
	}
	if report.TotalChallenges != 2 {
		t.Errorf("Subtree TotalChallenges: got %d, want 2", report.TotalChallenges)
	}
}

func TestOverallQuality_SubtreeMetrics_NonExistentNode(t *testing.T) {
	st := createTestState(t)
	id, _ := types.Parse("1.999")

	report := SubtreeQuality(st, id)

	if report.NodeCount != 0 {
		t.Errorf("Subtree NodeCount for non-existent: got %d, want 0", report.NodeCount)
	}
}

// =============================================================================
// QualityScore Tests
// =============================================================================

func TestQualityScore_EmptyState(t *testing.T) {
	st := createTestState(t)

	score := QualityScore(st)

	// Empty state should have a neutral/good score
	if score < 0 || score > 100 {
		t.Errorf("QualityScore out of range: got %f, want 0-100", score)
	}
}

func TestQualityScore_HealthyProof(t *testing.T) {
	// Create a well-structured proof with good metrics
	root := createTestNode(t, "1", func(n *node.Node) {
		n.EpistemicState = schema.EpistemicValidated
	})
	child := createTestNode(t, "1.1", func(n *node.Node) {
		n.EpistemicState = schema.EpistemicValidated
	})
	st := createTestState(t, root, child)

	score := QualityScore(st)

	// Should have a high score (well-validated, no open challenges)
	if score < 70 {
		t.Errorf("QualityScore for healthy proof too low: got %f, want >= 70", score)
	}
}

func TestQualityScore_ProblematicProof(t *testing.T) {
	// Create a proof with issues
	root := createTestNode(t, "1", func(n *node.Node) {
		n.EpistemicState = schema.EpistemicPending
		n.Context = []string{"def:undefined_term"}
	})
	st := createTestState(t, root)

	// Add open challenges
	st.AddChallenge(&state.Challenge{ID: "c1", NodeID: root.ID, Status: "open"})
	st.AddChallenge(&state.Challenge{ID: "c2", NodeID: root.ID, Status: "open"})

	score := QualityScore(st)

	// Should have a lower score
	if score > 50 {
		t.Errorf("QualityScore for problematic proof too high: got %f, want <= 50", score)
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkRefinementDepth(b *testing.B) {
	// Create a deep tree
	st := state.NewState()
	var nodes []*node.Node
	current := "1"
	for i := 0; i < 20; i++ {
		id, _ := types.Parse(current)
		n, _ := node.NewNode(id, schema.NodeTypeClaim, "statement", schema.InferenceModusPonens)
		nodes = append(nodes, n)
		st.AddNode(n)
		current = current + ".1"
	}

	rootID, _ := types.Parse("1")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RefinementDepth(st, rootID)
	}
}

func BenchmarkOverallQuality(b *testing.B) {
	st := state.NewState()
	for i := 1; i <= 100; i++ {
		idStr := "1"
		for j := 1; j < i%10+1; j++ {
			idStr += ".1"
		}
		id, _ := types.Parse(idStr)
		n, _ := node.NewNode(id, schema.NodeTypeClaim, "statement", schema.InferenceModusPonens)
		st.AddNode(n)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		OverallQuality(st)
	}
}
