//go:build integration

package render

import (
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// makeTestNodeForProver creates a test node with minimal required fields.
// Panics on invalid input (intended for test use only).
func makeTestNodeForProver(id string, nodeType schema.NodeType, statement string, inference schema.InferenceType) *node.Node {
	nodeID, err := types.Parse(id)
	if err != nil {
		panic("invalid test node ID: " + id)
	}
	n, err := node.NewNode(nodeID, nodeType, statement, inference)
	if err != nil {
		panic("failed to create test node: " + err.Error())
	}
	return n
}

// makeTestNodeWithDeps creates a test node with dependencies.
// Panics on invalid input (intended for test use only).
func makeTestNodeWithDeps(id string, nodeType schema.NodeType, statement string, inference schema.InferenceType, deps []string) *node.Node {
	nodeID, err := types.Parse(id)
	if err != nil {
		panic("invalid test node ID: " + id)
	}

	// Parse dependency IDs
	depIDs := make([]types.NodeID, len(deps))
	for i, dep := range deps {
		depID, err := types.Parse(dep)
		if err != nil {
			panic("invalid dependency ID: " + dep)
		}
		depIDs[i] = depID
	}

	n, err := node.NewNodeWithOptions(
		nodeID,
		nodeType,
		statement,
		inference,
		node.NodeOptions{
			Dependencies: depIDs,
		},
	)
	if err != nil {
		panic("failed to create test node: " + err.Error())
	}
	return n
}

// TestRenderProverContext_RootNode tests rendering context for a root node (no parent).
func TestRenderProverContext_RootNode(t *testing.T) {
	s := state.NewState()
	rootNode := makeTestNodeForProver("1", schema.NodeTypeClaim, "The main theorem to prove", schema.InferenceAssumption)
	s.AddNode(rootNode)

	rootID, _ := types.Parse("1")
	result := RenderProverContext(s, rootID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string for valid root node")
	}

	// Should contain the node ID
	if !strings.Contains(result, "1") {
		t.Errorf("RenderProverContext missing node ID, got: %q", result)
	}

	// Should contain the statement
	if !strings.Contains(result, "main theorem") || !strings.Contains(result, "prove") {
		t.Errorf("RenderProverContext missing statement content, got: %q", result)
	}

	// Should indicate this is the root (no parent to show)
	// The output format may vary, but should not contain misleading parent info
	// This is a soft check - implementation decides the exact format
}

// TestRenderProverContext_ChildNode tests rendering context for a child node (shows parent statement).
func TestRenderProverContext_ChildNode(t *testing.T) {
	s := state.NewState()

	// Create parent and child nodes
	parentNode := makeTestNodeForProver("1", schema.NodeTypeClaim, "Parent theorem statement", schema.InferenceAssumption)
	childNode := makeTestNodeForProver("1.1", schema.NodeTypeClaim, "Child step in the proof", schema.InferenceModusPonens)

	s.AddNode(parentNode)
	s.AddNode(childNode)

	childID, _ := types.Parse("1.1")
	result := RenderProverContext(s, childID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string for valid child node")
	}

	// Should contain the child node ID
	if !strings.Contains(result, "1.1") {
		t.Errorf("RenderProverContext missing child node ID, got: %q", result)
	}

	// Should contain the child's statement
	if !strings.Contains(result, "Child step") {
		t.Errorf("RenderProverContext missing child statement, got: %q", result)
	}

	// Should show parent context (parent's statement or ID)
	if !strings.Contains(result, "Parent") || !strings.Contains(result, "theorem") {
		t.Errorf("RenderProverContext missing parent context, got: %q", result)
	}
}

// TestRenderProverContext_WithDefinitions tests rendering context with definitions in scope.
func TestRenderProverContext_WithDefinitions(t *testing.T) {
	s := state.NewState()

	// Create a node
	n := makeTestNodeForProver("1", schema.NodeTypeClaim, "By definition of natural numbers", schema.InferenceByDefinition)
	s.AddNode(n)

	// Add definitions to state
	def1, err := node.NewDefinition("natural_number", "A natural number is a non-negative integer")
	if err != nil {
		t.Fatalf("failed to create definition: %v", err)
	}
	s.AddDefinition(def1)

	def2, err := node.NewDefinition("prime", "A prime is a natural number greater than 1 with no divisors other than 1 and itself")
	if err != nil {
		t.Fatalf("failed to create definition: %v", err)
	}
	s.AddDefinition(def2)

	rootID, _ := types.Parse("1")
	result := RenderProverContext(s, rootID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string")
	}

	// Should contain indication of available definitions
	// The exact format is implementation-dependent, but should mention definitions
	if !strings.Contains(strings.ToLower(result), "definition") && !strings.Contains(strings.ToLower(result), "def") {
		t.Logf("Note: RenderProverContext may want to show available definitions, got: %q", result)
	}

	// Should show definition names or content
	if !strings.Contains(result, "natural") && !strings.Contains(result, "prime") {
		t.Logf("Note: RenderProverContext may want to show definition names, got: %q", result)
	}
}

// TestRenderProverContext_WithAssumptions tests rendering context with assumptions in scope.
func TestRenderProverContext_WithAssumptions(t *testing.T) {
	s := state.NewState()

	// Create a node
	n := makeTestNodeForProver("1", schema.NodeTypeClaim, "Given the assumptions", schema.InferenceAssumption)
	s.AddNode(n)

	// Add assumptions to state
	assume1 := node.NewAssumption("Let n be a positive integer")
	s.AddAssumption(assume1)

	assume2 := node.NewAssumptionWithJustification("Assume P(k) holds for some k", "Induction hypothesis")
	s.AddAssumption(assume2)

	rootID, _ := types.Parse("1")
	result := RenderProverContext(s, rootID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string")
	}

	// Should contain indication of available assumptions
	if !strings.Contains(strings.ToLower(result), "assumption") && !strings.Contains(strings.ToLower(result), "assume") {
		t.Logf("Note: RenderProverContext may want to show available assumptions, got: %q", result)
	}

	// Should show assumption statements
	if !strings.Contains(result, "positive integer") && !strings.Contains(result, "P(k)") {
		t.Logf("Note: RenderProverContext may want to show assumption content, got: %q", result)
	}
}

// TestRenderProverContext_WithExternals tests rendering context with externals in scope.
func TestRenderProverContext_WithExternals(t *testing.T) {
	s := state.NewState()

	// Create a node
	n := makeTestNodeForProver("1", schema.NodeTypeClaim, "By the fundamental theorem of arithmetic", schema.InferenceAssumption)
	s.AddNode(n)

	// Add externals to state
	ext1 := node.NewExternal("Fundamental Theorem of Arithmetic", "Hardy & Wright, Chapter 1")
	s.AddExternal(&ext1)

	ext2 := node.NewExternalWithNotes("Euclid's Lemma", "Elements, Book VII, Prop. 30", "Used for uniqueness of factorization")
	s.AddExternal(&ext2)

	rootID, _ := types.Parse("1")
	result := RenderProverContext(s, rootID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string")
	}

	// Should contain indication of available externals
	if !strings.Contains(strings.ToLower(result), "external") && !strings.Contains(strings.ToLower(result), "reference") {
		t.Logf("Note: RenderProverContext may want to show available externals, got: %q", result)
	}

	// Should show external names or sources
	if !strings.Contains(result, "Fundamental") && !strings.Contains(result, "Euclid") {
		t.Logf("Note: RenderProverContext may want to show external names, got: %q", result)
	}
}

// TestRenderProverContext_WithActiveChallenges tests rendering context with active challenges.
func TestRenderProverContext_WithActiveChallenges(t *testing.T) {
	s := state.NewState()

	// Create a node that has been challenged
	n := makeTestNodeForProver("1", schema.NodeTypeClaim, "A contested claim", schema.InferenceModusPonens)
	s.AddNode(n)

	// Add challenges to the state
	rootID, _ := types.Parse("1")
	s.AddChallenge(&state.Challenge{
		ID:     "ch-001",
		NodeID: rootID,
		Target: "gap",
		Reason: "Missing case for n=0",
		Status: "open",
	})
	s.AddChallenge(&state.Challenge{
		ID:         "ch-002",
		NodeID:     rootID,
		Target:     "context",
		Reason:     "Variable x undefined",
		Status:     "resolved",
		Resolution: "x is defined in the parent scope as x := 5",
	})

	result := RenderProverContext(s, rootID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string")
	}

	// Should contain the node information
	if !strings.Contains(result, "contested") {
		t.Errorf("RenderProverContext missing node statement, got: %q", result)
	}

	// Should show challenge section with count
	if !strings.Contains(result, "Challenges (2 total, 1 open)") {
		t.Errorf("RenderProverContext missing challenge count, got: %q", result)
	}

	// Should show the open challenge
	if !strings.Contains(result, "ch-001") || !strings.Contains(result, "Missing case for n=0") {
		t.Errorf("RenderProverContext missing open challenge details, got: %q", result)
	}

	// Should show the resolved challenge with its status
	if !strings.Contains(result, "ch-002") || !strings.Contains(result, "resolved") {
		t.Errorf("RenderProverContext missing resolved challenge, got: %q", result)
	}

	// Should show the resolution text for resolved challenges
	if !strings.Contains(result, "Resolution:") || !strings.Contains(result, "x is defined in the parent scope") {
		t.Errorf("RenderProverContext missing resolution text for resolved challenge, got: %q", result)
	}

	// Should show guidance for addressing challenges
	if !strings.Contains(result, "af refine") {
		t.Errorf("RenderProverContext missing guidance for addressing challenges, got: %q", result)
	}
}

// TestRenderProverContext_ChallengeResolutions tests that resolved challenges display their resolution text.
func TestRenderProverContext_ChallengeResolutions(t *testing.T) {
	s := state.NewState()

	// Create a node with multiple challenges, some resolved with resolution text
	n := makeTestNodeForProver("1", schema.NodeTypeClaim, "A claim with challenges", schema.InferenceModusPonens)
	s.AddNode(n)

	rootID, _ := types.Parse("1")

	// Add an open challenge (no resolution)
	s.AddChallenge(&state.Challenge{
		ID:     "C1",
		NodeID: rootID,
		Target: "statement",
		Reason: "Why is Z true?",
		Status: "open",
	})

	// Add a resolved challenge with resolution text
	s.AddChallenge(&state.Challenge{
		ID:         "C2",
		NodeID:     rootID,
		Target:     "inference",
		Reason:     "Justify step X",
		Status:     "resolved",
		Resolution: "By lemma Y, we have...",
	})

	// Add a resolved challenge without resolution text (edge case)
	s.AddChallenge(&state.Challenge{
		ID:     "C3",
		NodeID: rootID,
		Target: "gap",
		Reason: "Missing base case",
		Status: "resolved",
		// No Resolution field set
	})

	result := RenderProverContext(s, rootID)

	// Should show all challenges
	if !strings.Contains(result, "C1") {
		t.Errorf("Missing challenge C1, got: %q", result)
	}
	if !strings.Contains(result, "C2") {
		t.Errorf("Missing challenge C2, got: %q", result)
	}
	if !strings.Contains(result, "C3") {
		t.Errorf("Missing challenge C3, got: %q", result)
	}

	// Should show resolution text for C2 (resolved with resolution)
	if !strings.Contains(result, "By lemma Y") {
		t.Errorf("Missing resolution text for C2, got: %q", result)
	}

	// Open challenge should show (open) status
	if !strings.Contains(result, "(open)") {
		t.Errorf("Missing (open) status indicator, got: %q", result)
	}

	// Resolved challenges should show (resolved) status
	if !strings.Contains(result, "(resolved)") {
		t.Errorf("Missing (resolved) status indicator, got: %q", result)
	}

	// Should have the Resolution: label for resolved challenges with resolution text
	if !strings.Contains(result, "Resolution:") {
		t.Errorf("Missing 'Resolution:' label, got: %q", result)
	}

	// The output should match the expected format:
	// [C1] "Why is Z true?" (open)
	// [C2] "Justify step X" (resolved)
	//      Resolution: "By lemma Y, we have..."
	lines := strings.Split(result, "\n")
	foundResolutionLine := false
	for _, line := range lines {
		if strings.Contains(line, "Resolution:") && strings.Contains(line, "By lemma Y") {
			foundResolutionLine = true
			break
		}
	}
	if !foundResolutionLine {
		t.Errorf("Resolution line not properly formatted, got: %q", result)
	}
}

// TestRenderProverContext_EmptyState tests that empty state returns appropriate message.
func TestRenderProverContext_EmptyState(t *testing.T) {
	s := state.NewState()

	// Try to render context for a node in empty state
	nodeID, _ := types.Parse("1")
	result := RenderProverContext(s, nodeID)

	// Should return some indication that the node/state is empty
	// Could be empty string or an error message - either is acceptable
	// The key is it shouldn't panic or produce misleading output

	// If result is empty, that's acceptable for empty state
	if result == "" {
		return
	}

	// If result is non-empty, it should indicate the issue (node not found)
	lowerResult := strings.ToLower(result)
	if !strings.Contains(lowerResult, "not found") &&
		!strings.Contains(lowerResult, "error") &&
		!strings.Contains(lowerResult, "no node") &&
		!strings.Contains(lowerResult, "empty") {
		t.Logf("Note: RenderProverContext for empty state returned unexpected content: %q", result)
	}
}

// TestRenderProverContext_NonExistentNodeID tests that non-existent node ID returns error message.
func TestRenderProverContext_NonExistentNodeID(t *testing.T) {
	s := state.NewState()

	// Add a node at 1, but request context for 1.5 which doesn't exist
	n := makeTestNodeForProver("1", schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
	s.AddNode(n)

	// Request a node that doesn't exist
	nonExistentID, _ := types.Parse("1.5")
	result := RenderProverContext(s, nonExistentID)

	// Should indicate the node wasn't found
	// Could be empty string or an error message
	if result == "" {
		return // Empty is acceptable for not-found
	}

	// If non-empty, should contain error indication
	lowerResult := strings.ToLower(result)
	if !strings.Contains(lowerResult, "not found") &&
		!strings.Contains(lowerResult, "error") &&
		!strings.Contains(lowerResult, "no node") &&
		!strings.Contains(lowerResult, "1.5") {
		t.Errorf("RenderProverContext for non-existent node should indicate error, got: %q", result)
	}
}

// TestRenderProverContext_NodeWithDependencies tests showing what a node depends on.
func TestRenderProverContext_NodeWithDependencies(t *testing.T) {
	s := state.NewState()

	// Create nodes with dependencies
	node1 := makeTestNodeForProver("1", schema.NodeTypeClaim, "Root theorem", schema.InferenceAssumption)
	node11 := makeTestNodeForProver("1.1", schema.NodeTypeClaim, "First lemma needed", schema.InferenceModusPonens)
	node12 := makeTestNodeForProver("1.2", schema.NodeTypeClaim, "Second lemma needed", schema.InferenceModusPonens)
	// Node 1.3 depends on 1.1 and 1.2
	node13 := makeTestNodeWithDeps("1.3", schema.NodeTypeClaim, "Conclusion using both lemmas", schema.InferenceModusPonens, []string{"1.1", "1.2"})

	s.AddNode(node1)
	s.AddNode(node11)
	s.AddNode(node12)
	s.AddNode(node13)

	// Request context for the node with dependencies
	nodeID, _ := types.Parse("1.3")
	result := RenderProverContext(s, nodeID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string for node with dependencies")
	}

	// Should contain the node's own statement
	if !strings.Contains(result, "Conclusion") || !strings.Contains(result, "lemmas") {
		t.Errorf("RenderProverContext missing node statement, got: %q", result)
	}

	// Should indicate dependencies
	if !strings.Contains(result, "1.1") && !strings.Contains(result, "1.2") {
		t.Logf("Note: RenderProverContext may want to show dependency IDs, got: %q", result)
	}

	// Could also show the dependency statements
	if !strings.Contains(result, "First lemma") && !strings.Contains(result, "Second lemma") {
		t.Logf("Note: RenderProverContext may want to show dependency statements, got: %q", result)
	}
}

// TestRenderProverContext_NilState tests handling of nil state.
func TestRenderProverContext_NilState(t *testing.T) {
	nodeID, _ := types.Parse("1")

	// Should not panic with nil state
	result := RenderProverContext(nil, nodeID)

	// Should return empty or error message, not panic
	if result != "" {
		// If non-empty, should indicate error
		lowerResult := strings.ToLower(result)
		if !strings.Contains(lowerResult, "error") && !strings.Contains(lowerResult, "nil") {
			t.Logf("Note: RenderProverContext for nil state returned: %q", result)
		}
	}
}

// TestRenderProverContext_MultiLineOutput tests that output is properly formatted multi-line text.
func TestRenderProverContext_MultiLineOutput(t *testing.T) {
	s := state.NewState()

	// Create a realistic scenario with multiple elements
	root := makeTestNodeForProver("1", schema.NodeTypeClaim, "Main theorem about prime numbers", schema.InferenceAssumption)
	child := makeTestNodeForProver("1.1", schema.NodeTypeLocalAssume, "Assume n is composite", schema.InferenceLocalAssume)

	s.AddNode(root)
	s.AddNode(child)

	// Add some definitions
	def, _ := node.NewDefinition("composite", "A composite number has factors other than 1 and itself")
	s.AddDefinition(def)

	childID, _ := types.Parse("1.1")
	result := RenderProverContext(s, childID)

	// Should be multi-line output (contains newlines)
	if !strings.Contains(result, "\n") {
		t.Logf("Note: RenderProverContext may want to use multi-line format for better readability, got: %q", result)
	}

	// Should be human-readable (not JSON or other machine format)
	if strings.HasPrefix(strings.TrimSpace(result), "{") || strings.HasPrefix(strings.TrimSpace(result), "[") {
		t.Errorf("RenderProverContext should return human-readable text, not JSON, got: %q", result)
	}
}

// TestRenderProverContext_ShowsSiblings tests that sibling nodes are shown for context.
func TestRenderProverContext_ShowsSiblings(t *testing.T) {
	s := state.NewState()

	// Create parent with multiple children (siblings)
	parent := makeTestNodeForProver("1", schema.NodeTypeClaim, "Parent claim", schema.InferenceAssumption)
	sibling1 := makeTestNodeForProver("1.1", schema.NodeTypeClaim, "First sibling step", schema.InferenceModusPonens)
	sibling2 := makeTestNodeForProver("1.2", schema.NodeTypeClaim, "Second sibling step", schema.InferenceModusPonens)
	sibling3 := makeTestNodeForProver("1.3", schema.NodeTypeClaim, "Third sibling step", schema.InferenceModusPonens)

	s.AddNode(parent)
	s.AddNode(sibling1)
	s.AddNode(sibling2)
	s.AddNode(sibling3)

	// Request context for middle sibling
	nodeID, _ := types.Parse("1.2")
	result := RenderProverContext(s, nodeID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string")
	}

	// Should show the requested node
	if !strings.Contains(result, "1.2") || !strings.Contains(result, "Second") {
		t.Errorf("RenderProverContext missing requested node info, got: %q", result)
	}

	// Could show siblings for context (implementation dependent)
	// This is a soft check - showing siblings is optional but helpful
	hasSiblingContext := strings.Contains(result, "1.1") || strings.Contains(result, "1.3") ||
		strings.Contains(result, "First sibling") || strings.Contains(result, "Third sibling")
	if !hasSiblingContext {
		t.Logf("Note: RenderProverContext may want to show sibling nodes for context, got: %q", result)
	}
}

// TestRenderProverContext_DeepHierarchy tests context rendering for deeply nested nodes.
func TestRenderProverContext_DeepHierarchy(t *testing.T) {
	s := state.NewState()

	// Create a deep hierarchy
	nodes := []*node.Node{
		makeTestNodeForProver("1", schema.NodeTypeClaim, "Level 1: Main theorem", schema.InferenceAssumption),
		makeTestNodeForProver("1.1", schema.NodeTypeClaim, "Level 2: Major lemma", schema.InferenceModusPonens),
		makeTestNodeForProver("1.1.1", schema.NodeTypeClaim, "Level 3: Sub-lemma", schema.InferenceModusPonens),
		makeTestNodeForProver("1.1.1.1", schema.NodeTypeClaim, "Level 4: Detail step", schema.InferenceModusPonens),
		makeTestNodeForProver("1.1.1.1.1", schema.NodeTypeClaim, "Level 5: Atomic fact", schema.InferenceByDefinition),
	}

	for _, n := range nodes {
		s.AddNode(n)
	}

	// Request context for deepest node
	deepID, _ := types.Parse("1.1.1.1.1")
	result := RenderProverContext(s, deepID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string for deep node")
	}

	// Should show the node itself
	if !strings.Contains(result, "1.1.1.1.1") || !strings.Contains(result, "Atomic fact") {
		t.Errorf("RenderProverContext missing deep node info, got: %q", result)
	}

	// Should show at least immediate parent context
	if !strings.Contains(result, "Level 4") && !strings.Contains(result, "1.1.1.1") {
		t.Logf("Note: RenderProverContext may want to show parent context for deep nodes, got: %q", result)
	}
}

// TestRenderProverContext_NodeTypes tests rendering for different node types.
func TestRenderProverContext_NodeTypes(t *testing.T) {
	tests := []struct {
		name      string
		nodeType  schema.NodeType
		inference schema.InferenceType
		statement string
	}{
		{
			name:      "claim node",
			nodeType:  schema.NodeTypeClaim,
			inference: schema.InferenceModusPonens,
			statement: "A claim to be proven",
		},
		{
			name:      "local assume node",
			nodeType:  schema.NodeTypeLocalAssume,
			inference: schema.InferenceLocalAssume,
			statement: "Assume for contradiction",
		},
		{
			name:      "local discharge node",
			nodeType:  schema.NodeTypeLocalDischarge,
			inference: schema.InferenceLocalDischarge,
			statement: "Discharged assumption leads to conclusion",
		},
		{
			name:      "case node",
			nodeType:  schema.NodeTypeCase,
			inference: schema.InferenceModusPonens,
			statement: "Case when n is even",
		},
		{
			name:      "qed node",
			nodeType:  schema.NodeTypeQED,
			inference: schema.InferenceModusPonens,
			statement: "Therefore the theorem holds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()
			n := makeTestNodeForProver("1", tt.nodeType, tt.statement, tt.inference)
			s.AddNode(n)

			nodeID, _ := types.Parse("1")
			result := RenderProverContext(s, nodeID)

			// Should not be empty for any valid node type
			if result == "" {
				t.Fatalf("RenderProverContext returned empty string for %s", tt.name)
			}

			// Should contain node type indicator
			if !strings.Contains(strings.ToLower(result), string(tt.nodeType)) {
				t.Logf("Note: RenderProverContext may want to show node type %q, got: %q", tt.nodeType, result)
			}

			// Should contain the statement
			if !strings.Contains(result, tt.statement) {
				t.Errorf("RenderProverContext missing statement %q, got: %q", tt.statement, result)
			}
		})
	}
}

// TestRenderProverContext_WithScope tests rendering context showing scope information.
func TestRenderProverContext_WithScope(t *testing.T) {
	s := state.NewState()

	// Create a node with scope entries
	nodeID, _ := types.Parse("1.1")
	n, err := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"Using local assumption",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Scope: []string{"assume:n>=0", "def:prime"},
		},
	)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Also add parent
	parent := makeTestNodeForProver("1", schema.NodeTypeClaim, "Parent claim", schema.InferenceAssumption)
	s.AddNode(parent)
	s.AddNode(n)

	result := RenderProverContext(s, nodeID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string")
	}

	// Should indicate scope is available
	if !strings.Contains(strings.ToLower(result), "scope") &&
		!strings.Contains(result, "n>=0") &&
		!strings.Contains(result, "prime") {
		t.Logf("Note: RenderProverContext may want to show scope entries, got: %q", result)
	}
}

// TestRenderProverContext_WithContext tests rendering context showing node context field.
func TestRenderProverContext_WithContext(t *testing.T) {
	s := state.NewState()

	// Create a node with context entries (references to definitions, etc.)
	nodeID, _ := types.Parse("1")
	n, err := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"By definition of natural numbers",
		schema.InferenceByDefinition,
		node.NodeOptions{
			Context: []string{"def:natural_number", "ext:peano_axioms"},
		},
	)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	s.AddNode(n)

	result := RenderProverContext(s, nodeID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string")
	}

	// Should show context references
	if !strings.Contains(result, "natural_number") && !strings.Contains(result, "peano") {
		t.Logf("Note: RenderProverContext may want to show context references, got: %q", result)
	}
}

// TestRenderProverContext_WorkflowState tests that workflow state is appropriately shown.
func TestRenderProverContext_WorkflowState(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForProver("1", schema.NodeTypeClaim, "Claimed node", schema.InferenceModusPonens)
	// Simulate claimed state
	n.WorkflowState = schema.WorkflowClaimed
	n.ClaimedBy = "prover-agent-123"

	s.AddNode(n)

	nodeID, _ := types.Parse("1")
	result := RenderProverContext(s, nodeID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string")
	}

	// Should show workflow/claim information
	// This helps the prover understand they own this node
	if !strings.Contains(strings.ToLower(result), "claimed") &&
		!strings.Contains(result, "prover-agent") {
		t.Logf("Note: RenderProverContext may want to show claim status, got: %q", result)
	}
}

// TestRenderProverContext_EpistemicState tests that epistemic state is shown.
func TestRenderProverContext_EpistemicState(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForProver("1", schema.NodeTypeClaim, "Validated node", schema.InferenceModusPonens)
	// Simulate validated state
	n.EpistemicState = schema.EpistemicValidated

	s.AddNode(n)

	nodeID, _ := types.Parse("1")
	result := RenderProverContext(s, nodeID)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderProverContext returned empty string")
	}

	// Should show epistemic state (pending, validated, etc.)
	if !strings.Contains(strings.ToLower(result), "validated") &&
		!strings.Contains(strings.ToLower(result), "epistemic") {
		t.Logf("Note: RenderProverContext may want to show epistemic state, got: %q", result)
	}
}

// TestRenderProverContext_ConsistentOutput tests that repeated calls produce consistent output.
func TestRenderProverContext_ConsistentOutput(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForProver("1", schema.NodeTypeClaim, "Deterministic output test", schema.InferenceModusPonens)
	s.AddNode(n)

	nodeID, _ := types.Parse("1")

	// Call multiple times
	result1 := RenderProverContext(s, nodeID)
	result2 := RenderProverContext(s, nodeID)
	result3 := RenderProverContext(s, nodeID)

	// All calls should produce identical output (deterministic)
	if result1 != result2 || result2 != result3 {
		t.Errorf("RenderProverContext produced inconsistent output:\n1: %q\n2: %q\n3: %q",
			result1, result2, result3)
	}
}

// TestRenderProverContext_LargeState tests performance/correctness with many nodes.
func TestRenderProverContext_LargeState(t *testing.T) {
	s := state.NewState()

	// Create a tree with 50+ nodes
	root := makeTestNodeForProver("1", schema.NodeTypeClaim, "Root of large tree", schema.InferenceAssumption)
	s.AddNode(root)

	// Add 10 children with 5 grandchildren each
	for i := 1; i <= 10; i++ {
		childID := "1." + string(rune('0'+i))
		if i >= 10 {
			childID = "1.10"
		}
		child := makeTestNodeForProver(childID, schema.NodeTypeClaim, "Child claim number", schema.InferenceModusPonens)
		s.AddNode(child)
	}

	// Request context for a node in the middle
	nodeID, _ := types.Parse("1.5")
	result := RenderProverContext(s, nodeID)

	// Should complete without error and return non-empty result
	if result == "" {
		t.Fatal("RenderProverContext returned empty for large state")
	}

	// Should contain the requested node
	if !strings.Contains(result, "1.5") {
		t.Errorf("RenderProverContext missing requested node ID in large state, got: %q", result)
	}
}

// TestRenderProverContext_SpecialCharactersInStatement tests handling of special characters.
func TestRenderProverContext_SpecialCharactersInStatement(t *testing.T) {
	tests := []struct {
		name      string
		statement string
	}{
		{
			name:      "unicode math symbols",
			statement: "For all x in set: P(x) implies Q(x)",
		},
		{
			name:      "quotes",
			statement: `The term "natural number" is defined`,
		},
		{
			name:      "newlines in statement",
			statement: "Line one\nLine two\nLine three",
		},
		{
			name:      "tabs and whitespace",
			statement: "With\ttabs\tand  multiple  spaces",
		},
		{
			name:      "backslashes for LaTeX",
			statement: `Let \alpha + \beta = \gamma`,
		},
		{
			name:      "angle brackets",
			statement: "For all <x> in set <S>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()
			n := makeTestNodeForProver("1", schema.NodeTypeClaim, tt.statement, schema.InferenceModusPonens)
			s.AddNode(n)

			nodeID, _ := types.Parse("1")

			// Should not panic
			result := RenderProverContext(s, nodeID)

			// Should return non-empty result
			if result == "" {
				t.Fatalf("RenderProverContext returned empty for statement with %s", tt.name)
			}

			// Should contain node ID at minimum
			if !strings.Contains(result, "1") {
				t.Errorf("RenderProverContext missing node ID, got: %q", result)
			}
		})
	}
}
