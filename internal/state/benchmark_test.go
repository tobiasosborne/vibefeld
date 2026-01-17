package state

import (
	"fmt"
	"testing"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// BenchmarkFindChildrenLargeTree benchmarks finding children in a large tree.
// Creates a tree with 10K nodes and measures the cost of finding children 100 times.
func BenchmarkFindChildrenLargeTree(b *testing.B) {
	// Build a tree with 10K nodes
	s := NewState()
	nodes := buildLargeTree(b, s, 10000)

	// Select a node in the middle to find children of
	middleIdx := len(nodes) / 2
	parentID := nodes[middleIdx].ID

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Find children of the middle node 100 times
		for j := 0; j < 100; j++ {
			findChildrenBenchmark(parentID, s.AllNodes())
		}
	}
}

// BenchmarkGetBlockingChallengesNode benchmarks looking up blocking challenges.
// Creates 10K challenges and measures 100 lookups.
func BenchmarkGetBlockingChallengesNode(b *testing.B) {
	s := NewState()

	// Create a node to attach challenges to
	nodeID := mustParseNodeIDForBench(b, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		b.Fatalf("Failed to create node: %v", err)
	}
	s.AddNode(n)

	// Add 10K challenges to various nodes (create additional nodes)
	for i := 0; i < 10000; i++ {
		// Create challenges for various nodes
		targetNodeID := mustParseNodeIDForBench(b, fmt.Sprintf("1.%d", (i%100)+1))
		if s.GetNode(targetNodeID) == nil {
			targetNode, _ := node.NewNode(targetNodeID, schema.NodeTypeClaim, fmt.Sprintf("Claim %d", i), schema.InferenceAssumption)
			s.AddNode(targetNode)
		}

		severity := "major" // blocking
		if i%3 == 0 {
			severity = "minor" // non-blocking
		}

		challenge := &Challenge{
			ID:       fmt.Sprintf("chal-%d", i),
			NodeID:   targetNodeID,
			Target:   "statement",
			Reason:   fmt.Sprintf("Challenge reason %d", i),
			Status:   "open",
			Severity: severity,
			RaisedBy: "verifier-1",
		}
		s.AddChallenge(challenge)
	}

	// Pick a node that has challenges
	testNodeID := mustParseNodeIDForBench(b, "1.50")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Look up blocking challenges 100 times
		for j := 0; j < 100; j++ {
			_ = s.GetBlockingChallengesForNode(testNodeID)
		}
	}
}

// BenchmarkStateReplay benchmarks replaying events from a ledger.
// Creates a ledger with 1000 events and measures full replay time.
func BenchmarkStateReplay(b *testing.B) {
	benchmarks := []struct {
		name       string
		eventCount int
	}{
		{"100_events", 100},
		{"500_events", 500},
		{"1000_events", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Create a temporary ledger with events
			dir := b.TempDir()
			ldg, err := ledger.NewLedger(dir)
			if err != nil {
				b.Fatalf("Failed to create ledger: %v", err)
			}

			// Generate events
			if err := generateEvents(ldg, bm.eventCount); err != nil {
				b.Fatalf("Failed to generate events: %v", err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := Replay(ldg)
				if err != nil {
					b.Fatalf("Replay failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkChallengesByNodeID benchmarks the cached challenge lookup by node.
func BenchmarkChallengesByNodeID(b *testing.B) {
	s := NewState()

	// Add challenges to 100 different nodes
	for i := 0; i < 100; i++ {
		nodeID := mustParseNodeIDForBench(b, fmt.Sprintf("1.%d", i+1))
		n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, fmt.Sprintf("Claim %d", i), schema.InferenceAssumption)
		s.AddNode(n)

		// Add 10 challenges per node (1000 total)
		for j := 0; j < 10; j++ {
			challenge := &Challenge{
				ID:       fmt.Sprintf("chal-%d-%d", i, j),
				NodeID:   nodeID,
				Target:   "statement",
				Reason:   fmt.Sprintf("Reason %d-%d", i, j),
				Status:   "open",
				Severity: "major",
				RaisedBy: "verifier-1",
			}
			s.AddChallenge(challenge)
		}
	}

	testNodeID := mustParseNodeIDForBench(b, "1.50")

	b.Run("cached_lookup", func(b *testing.B) {
		// First call builds the cache
		_ = s.ChallengesByNodeID()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = s.GetChallengesForNode(testNodeID)
		}
	})

	b.Run("invalidated_cache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Invalidate cache each time
			s.InvalidateChallengeCache()
			_ = s.GetChallengesForNode(testNodeID)
		}
	})
}

// BenchmarkAllChildrenValidated benchmarks checking if all children are validated.
func BenchmarkAllChildrenValidated(b *testing.B) {
	s := NewState()

	// Build a tree: 1 root with 100 children
	rootID := mustParseNodeIDForBench(b, "1")
	root, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
	root.EpistemicState = schema.EpistemicPending
	s.AddNode(root)

	for i := 1; i <= 100; i++ {
		childID := mustParseNodeIDForBench(b, fmt.Sprintf("1.%d", i))
		child, _ := node.NewNode(childID, schema.NodeTypeClaim, fmt.Sprintf("Child %d", i), schema.InferenceAssumption)
		child.EpistemicState = schema.EpistemicValidated
		s.AddNode(child)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.AllChildrenValidated(rootID)
	}
}

// Helper functions

func mustParseNodeIDForBench(b *testing.B, s string) types.NodeID {
	b.Helper()
	id, err := types.Parse(s)
	if err != nil {
		b.Fatalf("Failed to parse node ID %q: %v", s, err)
	}
	return id
}

// buildLargeTree creates a balanced tree with the specified number of nodes.
// Returns the slice of created nodes.
func buildLargeTree(b *testing.B, s *State, nodeCount int) []*node.Node {
	b.Helper()

	nodes := make([]*node.Node, 0, nodeCount)

	// Create root
	rootID := mustParseNodeIDForBench(b, "1")
	root, err := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
	if err != nil {
		b.Fatalf("Failed to create root: %v", err)
	}
	s.AddNode(root)
	nodes = append(nodes, root)

	// Build tree breadth-first with branching factor of 10
	created := 1
	queue := []types.NodeID{rootID}
	childNum := make(map[string]int) // Track child counts per parent

	for created < nodeCount && len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]

		// Add up to 10 children to each parent
		for i := 1; i <= 10 && created < nodeCount; i++ {
			childNum[parentID.String()]++
			childID, _ := parentID.Child(childNum[parentID.String()])

			n, err := node.NewNode(childID, schema.NodeTypeClaim, fmt.Sprintf("Claim %d", created), schema.InferenceAssumption)
			if err != nil {
				b.Fatalf("Failed to create node %s: %v", childID, err)
			}
			s.AddNode(n)
			nodes = append(nodes, n)
			created++

			queue = append(queue, childID)
		}
	}

	return nodes
}

// findChildrenBenchmark is the function under test for finding children.
// This mimics what render.findChildren does.
func findChildrenBenchmark(parentID types.NodeID, allNodes []*node.Node) []*node.Node {
	var children []*node.Node
	for _, n := range allNodes {
		parent, hasParent := n.ID.Parent()
		if !hasParent {
			continue
		}
		if parent.Equal(parentID) {
			children = append(children, n)
		}
	}
	return children
}

// generateEvents creates a series of ledger events for benchmarking.
func generateEvents(ldg *ledger.Ledger, count int) error {
	// Start with proof initialization
	initEvent := ledger.NewProofInitialized("Benchmark conjecture", "benchmark")
	if err := appendEvent(ldg, initEvent); err != nil {
		return err
	}

	// Create the root node
	rootID, _ := types.Parse("1")
	rootNode, _ := node.NewNode(rootID, schema.NodeTypeClaim, "Root claim for benchmark", schema.InferenceAssumption)
	createEvent := ledger.NewNodeCreated(*rootNode)
	if err := appendEvent(ldg, createEvent); err != nil {
		return err
	}

	eventsCreated := 2

	// Generate remaining events as node creates
	// This is the simplest approach that guarantees state consistency
	nodeNum := 1
	for eventsCreated < count {
		// Create more child nodes using a deeper hierarchy
		// Use a pattern like 1.1, 1.2, ..., 1.1.1, 1.1.2, etc.
		parentID := rootID
		depth := (nodeNum-1)/10 + 1
		for d := 1; d < depth && d <= 5; d++ {
			parentID, _ = parentID.Child(((nodeNum - 1) % 10) + 1)
		}
		childID, _ := parentID.Child(((nodeNum - 1) % 10) + 1)

		childNode, _ := node.NewNode(childID, schema.NodeTypeClaim, fmt.Sprintf("Claim %d", nodeNum), schema.InferenceAssumption)
		createEvent := ledger.NewNodeCreated(*childNode)
		if err := appendEvent(ldg, createEvent); err != nil {
			return err
		}
		nodeNum++
		eventsCreated++
	}

	return nil
}

// appendEvent appends an event to the ledger.
func appendEvent(ldg *ledger.Ledger, event ledger.Event) error {
	_, err := ldg.Append(event)
	return err
}
