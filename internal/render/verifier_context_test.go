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

// makeTestNodeForVerifier creates a test node with minimal required fields.
// Panics on invalid input (intended for test use only).
func makeTestNodeForVerifier(id string, nodeType schema.NodeType, statement string, inference schema.InferenceType) *node.Node {
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

// makeTestChallenge creates a test challenge with the given parameters.
// Panics on invalid input (intended for test use only).
func makeTestChallenge(id string, targetID string, target schema.ChallengeTarget, reason string) *node.Challenge {
	nodeID, err := types.Parse(targetID)
	if err != nil {
		panic("invalid target node ID: " + targetID)
	}
	c, err := node.NewChallenge(id, nodeID, target, reason)
	if err != nil {
		panic("failed to create challenge: " + err.Error())
	}
	return c
}

// TestRenderVerifierContext_ShowsChallengeInfo tests that the verifier context shows challenge details.
func TestRenderVerifierContext_ShowsChallengeInfo(t *testing.T) {
	s := state.NewState()

	// Create a node that will be challenged
	challengedNode := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "The claim being challenged", schema.InferenceModusPonens)
	s.AddNode(challengedNode)

	// Create a challenge against the node
	challenge := makeTestChallenge("chal-001", "1", schema.TargetStatement, "The statement is unclear")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string for valid challenge")
	}

	// Should contain the challenge ID
	if !strings.Contains(result, "chal-001") {
		t.Errorf("RenderVerifierContext missing challenge ID, got: %q", result)
	}

	// Should contain the challenge reason
	if !strings.Contains(result, "unclear") || !strings.Contains(result, "statement") {
		t.Errorf("RenderVerifierContext missing challenge reason, got: %q", result)
	}

	// Should indicate the challenge target type
	if !strings.Contains(strings.ToLower(result), "statement") {
		t.Errorf("RenderVerifierContext missing challenge target type, got: %q", result)
	}
}

// TestRenderVerifierContext_ShowsChallengedNode tests that the challenged node info is displayed.
func TestRenderVerifierContext_ShowsChallengedNode(t *testing.T) {
	s := state.NewState()

	// Create the challenged node
	challengedNode := makeTestNodeForVerifier("1.2", schema.NodeTypeClaim, "The theorem to verify", schema.InferenceModusPonens)
	s.AddNode(challengedNode)

	// Create a challenge
	challenge := makeTestChallenge("chal-002", "1.2", schema.TargetInference, "Inference type is inappropriate")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string")
	}

	// Should show the challenged node ID
	if !strings.Contains(result, "1.2") {
		t.Errorf("RenderVerifierContext missing challenged node ID, got: %q", result)
	}

	// Should show the challenged node's statement
	if !strings.Contains(result, "theorem") || !strings.Contains(result, "verify") {
		t.Errorf("RenderVerifierContext missing challenged node statement, got: %q", result)
	}

	// Should show the node's inference type
	if !strings.Contains(strings.ToLower(result), "modus_ponens") && !strings.Contains(result, "Modus Ponens") {
		t.Logf("Note: RenderVerifierContext may want to show node inference type, got: %q", result)
	}
}

// TestRenderVerifierContext_ShowsParentContext tests that the parent of the challenged node is shown.
func TestRenderVerifierContext_ShowsParentContext(t *testing.T) {
	s := state.NewState()

	// Create parent and child nodes
	parentNode := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "Parent theorem statement", schema.InferenceAssumption)
	childNode := makeTestNodeForVerifier("1.1", schema.NodeTypeClaim, "Child step being challenged", schema.InferenceModusPonens)

	s.AddNode(parentNode)
	s.AddNode(childNode)

	// Create a challenge against the child
	challenge := makeTestChallenge("chal-003", "1.1", schema.TargetGap, "Logical gap in reasoning")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string")
	}

	// Should show the challenged node
	if !strings.Contains(result, "1.1") {
		t.Errorf("RenderVerifierContext missing challenged node ID, got: %q", result)
	}

	// Should show parent context
	if !strings.Contains(result, "Parent") || !strings.Contains(result, "theorem") {
		t.Errorf("RenderVerifierContext missing parent context, got: %q", result)
	}
}

// TestRenderVerifierContext_ShowsSiblings tests that sibling nodes are shown for context.
func TestRenderVerifierContext_ShowsSiblings(t *testing.T) {
	s := state.NewState()

	// Create parent with multiple children (siblings)
	parent := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "Parent claim", schema.InferenceAssumption)
	sibling1 := makeTestNodeForVerifier("1.1", schema.NodeTypeClaim, "First sibling step", schema.InferenceModusPonens)
	sibling2 := makeTestNodeForVerifier("1.2", schema.NodeTypeClaim, "Second sibling step", schema.InferenceModusPonens)
	sibling3 := makeTestNodeForVerifier("1.3", schema.NodeTypeClaim, "Third sibling step", schema.InferenceModusPonens)

	s.AddNode(parent)
	s.AddNode(sibling1)
	s.AddNode(sibling2)
	s.AddNode(sibling3)

	// Create a challenge against the middle sibling
	challenge := makeTestChallenge("chal-004", "1.2", schema.TargetScope, "Scope issue")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string")
	}

	// Should show the requested node
	if !strings.Contains(result, "1.2") || !strings.Contains(result, "Second") {
		t.Errorf("RenderVerifierContext missing challenged node info, got: %q", result)
	}

	// Could show siblings for context (implementation dependent)
	hasSiblingContext := strings.Contains(result, "1.1") || strings.Contains(result, "1.3") ||
		strings.Contains(result, "First sibling") || strings.Contains(result, "Third sibling")
	if !hasSiblingContext {
		t.Logf("Note: RenderVerifierContext may want to show sibling nodes for context, got: %q", result)
	}
}

// TestRenderVerifierContext_ShowsDefinitions tests that definitions in scope are shown.
func TestRenderVerifierContext_ShowsDefinitions(t *testing.T) {
	s := state.NewState()

	// Create a node with context references to definitions
	nodeID, _ := types.Parse("1")
	n, err := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"By definition of natural numbers",
		schema.InferenceByDefinition,
		node.NodeOptions{
			Context: []string{"def:natural_number"},
		},
	)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	s.AddNode(n)

	// Add the referenced definition
	def, _ := node.NewDefinition("natural_number", "A natural number is a non-negative integer")
	s.AddDefinition(def)

	// Create a challenge
	challenge := makeTestChallenge("chal-005", "1", schema.TargetContext, "Wrong definition applied")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string")
	}

	// Should indicate definitions in scope
	if !strings.Contains(strings.ToLower(result), "definition") && !strings.Contains(strings.ToLower(result), "def") {
		t.Logf("Note: RenderVerifierContext may want to show available definitions, got: %q", result)
	}

	// Should show definition name or content
	if !strings.Contains(result, "natural") {
		t.Logf("Note: RenderVerifierContext may want to show definition names/content, got: %q", result)
	}
}

// TestRenderVerifierContext_ShowsAssumptions tests that assumptions in scope are shown.
func TestRenderVerifierContext_ShowsAssumptions(t *testing.T) {
	s := state.NewState()

	// Create a node with context references to assumptions
	nodeID, _ := types.Parse("1")
	n, err := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"Given the induction hypothesis",
		schema.InferenceAssumption,
		node.NodeOptions{
			Context: []string{"assume:induction_hyp"},
		},
	)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	s.AddNode(n)

	// Add the referenced assumption
	assume := node.NewAssumptionWithJustification("Assume P(k) holds for some k", "Induction hypothesis")
	s.AddAssumption(assume)

	// Create a challenge
	challenge := makeTestChallenge("chal-006", "1", schema.TargetDependencies, "Missing dependency")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string")
	}

	// Should indicate assumptions in scope
	if !strings.Contains(strings.ToLower(result), "assumption") && !strings.Contains(strings.ToLower(result), "assume") {
		t.Logf("Note: RenderVerifierContext may want to show available assumptions, got: %q", result)
	}
}

// TestRenderVerifierContext_AllChallengeTargets tests rendering for all challenge target types.
func TestRenderVerifierContext_AllChallengeTargets(t *testing.T) {
	tests := []struct {
		name   string
		target schema.ChallengeTarget
		reason string
	}{
		{
			name:   "statement challenge",
			target: schema.TargetStatement,
			reason: "The claim text itself is disputed",
		},
		{
			name:   "inference challenge",
			target: schema.TargetInference,
			reason: "The inference type is inappropriate",
		},
		{
			name:   "context challenge",
			target: schema.TargetContext,
			reason: "Referenced definitions are wrong",
		},
		{
			name:   "dependencies challenge",
			target: schema.TargetDependencies,
			reason: "Node dependencies are incorrect",
		},
		{
			name:   "scope challenge",
			target: schema.TargetScope,
			reason: "Scope/local assumption issues",
		},
		{
			name:   "gap challenge",
			target: schema.TargetGap,
			reason: "Logical gap in reasoning",
		},
		{
			name:   "type_error challenge",
			target: schema.TargetTypeError,
			reason: "Type mismatch in mathematical objects",
		},
		{
			name:   "domain challenge",
			target: schema.TargetDomain,
			reason: "Domain restriction violation",
		},
		{
			name:   "completeness challenge",
			target: schema.TargetCompleteness,
			reason: "Missing cases in argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()
			n := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "A contested claim", schema.InferenceModusPonens)
			s.AddNode(n)

			challenge := makeTestChallenge("chal-target-test", "1", tt.target, tt.reason)
			result := RenderVerifierContext(s, challenge)

			// Should not be empty for any valid challenge target
			if result == "" {
				t.Fatalf("RenderVerifierContext returned empty string for %s", tt.name)
			}

			// Should contain challenge target type indicator
			if !strings.Contains(strings.ToLower(result), string(tt.target)) {
				t.Logf("Note: RenderVerifierContext may want to show challenge target type %q, got: %q", tt.target, result)
			}

			// Should contain the reason
			if !strings.Contains(result, tt.reason) && !strings.Contains(result, string(tt.target)) {
				t.Logf("Note: RenderVerifierContext may want to show challenge reason, got: %q", result)
			}
		})
	}
}

// TestRenderVerifierContext_NilState tests handling of nil state.
func TestRenderVerifierContext_NilState(t *testing.T) {
	challenge := makeTestChallenge("chal-nil", "1", schema.TargetStatement, "Some reason")

	// Should not panic with nil state
	result := RenderVerifierContext(nil, challenge)

	// Should return empty or error message, not panic
	if result != "" {
		// If non-empty, should indicate error
		lowerResult := strings.ToLower(result)
		if !strings.Contains(lowerResult, "error") && !strings.Contains(lowerResult, "nil") && !strings.Contains(lowerResult, "not found") {
			t.Logf("Note: RenderVerifierContext for nil state returned: %q", result)
		}
	}
}

// TestRenderVerifierContext_NilChallenge tests handling of nil challenge.
func TestRenderVerifierContext_NilChallenge(t *testing.T) {
	s := state.NewState()
	n := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "A claim", schema.InferenceModusPonens)
	s.AddNode(n)

	// Should not panic with nil challenge
	result := RenderVerifierContext(s, nil)

	// Should return empty or error message, not panic
	if result != "" {
		// If non-empty, should indicate error
		lowerResult := strings.ToLower(result)
		if !strings.Contains(lowerResult, "error") && !strings.Contains(lowerResult, "nil") {
			t.Logf("Note: RenderVerifierContext for nil challenge returned: %q", result)
		}
	}
}

// TestRenderVerifierContext_ChallengeTargetNotFound tests when challenged node doesn't exist.
func TestRenderVerifierContext_ChallengeTargetNotFound(t *testing.T) {
	s := state.NewState()

	// Add a node at 1, but challenge references 1.5 which doesn't exist
	n := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
	s.AddNode(n)

	// Create a challenge targeting non-existent node
	challenge := makeTestChallenge("chal-missing", "1.5", schema.TargetStatement, "Challenge reason")

	result := RenderVerifierContext(s, challenge)

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
		t.Errorf("RenderVerifierContext for non-existent target should indicate error, got: %q", result)
	}
}

// TestRenderVerifierContext_MultiLineOutput tests that output is properly formatted multi-line text.
func TestRenderVerifierContext_MultiLineOutput(t *testing.T) {
	s := state.NewState()

	// Create a realistic scenario
	root := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "Main theorem about prime numbers", schema.InferenceAssumption)
	child := makeTestNodeForVerifier("1.1", schema.NodeTypeLocalAssume, "Assume n is composite", schema.InferenceLocalAssume)

	s.AddNode(root)
	s.AddNode(child)

	challenge := makeTestChallenge("chal-format", "1.1", schema.TargetGap, "Missing justification step")

	result := RenderVerifierContext(s, challenge)

	// Should be multi-line output (contains newlines)
	if !strings.Contains(result, "\n") {
		t.Logf("Note: RenderVerifierContext may want to use multi-line format for better readability, got: %q", result)
	}

	// Should be human-readable (not JSON or other machine format)
	if strings.HasPrefix(strings.TrimSpace(result), "{") || strings.HasPrefix(strings.TrimSpace(result), "[") {
		t.Errorf("RenderVerifierContext should return human-readable text, not JSON, got: %q", result)
	}
}

// TestRenderVerifierContext_DeepHierarchy tests context rendering for deeply nested challenged node.
func TestRenderVerifierContext_DeepHierarchy(t *testing.T) {
	s := state.NewState()

	// Create a deep hierarchy
	nodes := []*node.Node{
		makeTestNodeForVerifier("1", schema.NodeTypeClaim, "Level 1: Main theorem", schema.InferenceAssumption),
		makeTestNodeForVerifier("1.1", schema.NodeTypeClaim, "Level 2: Major lemma", schema.InferenceModusPonens),
		makeTestNodeForVerifier("1.1.1", schema.NodeTypeClaim, "Level 3: Sub-lemma", schema.InferenceModusPonens),
		makeTestNodeForVerifier("1.1.1.1", schema.NodeTypeClaim, "Level 4: Detail step", schema.InferenceModusPonens),
		makeTestNodeForVerifier("1.1.1.1.1", schema.NodeTypeClaim, "Level 5: Atomic fact", schema.InferenceByDefinition),
	}

	for _, n := range nodes {
		s.AddNode(n)
	}

	// Challenge the deepest node
	challenge := makeTestChallenge("chal-deep", "1.1.1.1.1", schema.TargetStatement, "Atomic fact is incorrect")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string for deep node challenge")
	}

	// Should show the challenged node itself
	if !strings.Contains(result, "1.1.1.1.1") || !strings.Contains(result, "Atomic fact") {
		t.Errorf("RenderVerifierContext missing deep node info, got: %q", result)
	}

	// Should show at least immediate parent context
	if !strings.Contains(result, "Level 4") && !strings.Contains(result, "1.1.1.1") {
		t.Logf("Note: RenderVerifierContext may want to show parent context for deep nodes, got: %q", result)
	}
}

// TestRenderVerifierContext_NodeTypes tests rendering for challenges against different node types.
func TestRenderVerifierContext_NodeTypes(t *testing.T) {
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
			n := makeTestNodeForVerifier("1", tt.nodeType, tt.statement, tt.inference)
			s.AddNode(n)

			challenge := makeTestChallenge("chal-nodetype", "1", schema.TargetStatement, "Statement is disputed")

			result := RenderVerifierContext(s, challenge)

			// Should not be empty for any valid node type
			if result == "" {
				t.Fatalf("RenderVerifierContext returned empty string for %s", tt.name)
			}

			// Should contain node type indicator
			if !strings.Contains(strings.ToLower(result), string(tt.nodeType)) {
				t.Logf("Note: RenderVerifierContext may want to show node type %q, got: %q", tt.nodeType, result)
			}

			// Should contain the statement
			if !strings.Contains(result, tt.statement) {
				t.Errorf("RenderVerifierContext missing statement %q, got: %q", tt.statement, result)
			}
		})
	}
}

// TestRenderVerifierContext_WithScope tests rendering context showing scope information.
func TestRenderVerifierContext_WithScope(t *testing.T) {
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
	parent := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "Parent claim", schema.InferenceAssumption)
	s.AddNode(parent)
	s.AddNode(n)

	// Create a challenge about scope
	challenge := makeTestChallenge("chal-scope", "1.1", schema.TargetScope, "Scope violation")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string")
	}

	// Should indicate scope is available (especially relevant for scope challenges)
	if !strings.Contains(strings.ToLower(result), "scope") &&
		!strings.Contains(result, "n>=0") &&
		!strings.Contains(result, "prime") {
		t.Logf("Note: RenderVerifierContext may want to show scope entries for scope challenges, got: %q", result)
	}
}

// TestRenderVerifierContext_ShowsEpistemicState tests that epistemic state is shown.
func TestRenderVerifierContext_ShowsEpistemicState(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "Pending verification node", schema.InferenceModusPonens)
	// Node starts in pending state by default
	s.AddNode(n)

	challenge := makeTestChallenge("chal-epist", "1", schema.TargetStatement, "Statement unclear")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string")
	}

	// Should show epistemic state (pending, validated, etc.)
	if !strings.Contains(strings.ToLower(result), "pending") &&
		!strings.Contains(strings.ToLower(result), "epistemic") {
		t.Logf("Note: RenderVerifierContext may want to show epistemic state, got: %q", result)
	}
}

// TestRenderVerifierContext_ShowsDependencies tests that node dependencies are displayed.
func TestRenderVerifierContext_ShowsDependencies(t *testing.T) {
	s := state.NewState()

	// Create nodes with dependencies
	node1 := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "Root theorem", schema.InferenceAssumption)
	node11 := makeTestNodeForVerifier("1.1", schema.NodeTypeClaim, "First lemma needed", schema.InferenceModusPonens)
	node12 := makeTestNodeForVerifier("1.2", schema.NodeTypeClaim, "Second lemma needed", schema.InferenceModusPonens)

	// Node 1.3 depends on 1.1 and 1.2
	nodeID, _ := types.Parse("1.3")
	depID11, _ := types.Parse("1.1")
	depID12, _ := types.Parse("1.2")
	node13, _ := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"Conclusion using both lemmas",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Dependencies: []types.NodeID{depID11, depID12},
		},
	)

	s.AddNode(node1)
	s.AddNode(node11)
	s.AddNode(node12)
	s.AddNode(node13)

	// Challenge the node with dependencies
	challenge := makeTestChallenge("chal-deps", "1.3", schema.TargetDependencies, "Dependency incorrect")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string for node with dependencies")
	}

	// Should indicate dependencies (especially relevant for dependency challenges)
	if !strings.Contains(result, "1.1") && !strings.Contains(result, "1.2") {
		t.Logf("Note: RenderVerifierContext may want to show dependency IDs, got: %q", result)
	}

	// Could also show the dependency statements
	if !strings.Contains(result, "First lemma") && !strings.Contains(result, "Second lemma") {
		t.Logf("Note: RenderVerifierContext may want to show dependency statements, got: %q", result)
	}
}

// TestRenderVerifierContext_ChallengeStatus tests handling of different challenge statuses.
func TestRenderVerifierContext_ChallengeStatus(t *testing.T) {
	tests := []struct {
		name       string
		setupChallenge func(c *node.Challenge)
		expectStatus   string
	}{
		{
			name: "open challenge",
			setupChallenge: func(c *node.Challenge) {
				// Challenge is open by default
			},
			expectStatus: "open",
		},
		{
			name: "resolved challenge",
			setupChallenge: func(c *node.Challenge) {
				c.Resolve("Fixed the issue by clarifying the statement")
			},
			expectStatus: "resolved",
		},
		{
			name: "withdrawn challenge",
			setupChallenge: func(c *node.Challenge) {
				c.Withdraw()
			},
			expectStatus: "withdrawn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()
			n := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "A claim", schema.InferenceModusPonens)
			s.AddNode(n)

			challenge := makeTestChallenge("chal-status", "1", schema.TargetStatement, "Some reason")
			tt.setupChallenge(challenge)

			result := RenderVerifierContext(s, challenge)

			// Should not be empty
			if result == "" {
				t.Fatalf("RenderVerifierContext returned empty string for %s", tt.name)
			}

			// Should indicate challenge status
			if !strings.Contains(strings.ToLower(result), tt.expectStatus) {
				t.Logf("Note: RenderVerifierContext may want to show challenge status %q, got: %q", tt.expectStatus, result)
			}
		})
	}
}

// TestRenderVerifierContext_ConsistentOutput tests that repeated calls produce consistent output.
func TestRenderVerifierContext_ConsistentOutput(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "Deterministic output test", schema.InferenceModusPonens)
	s.AddNode(n)

	challenge := makeTestChallenge("chal-consistent", "1", schema.TargetStatement, "Test reason")

	// Call multiple times
	result1 := RenderVerifierContext(s, challenge)
	result2 := RenderVerifierContext(s, challenge)
	result3 := RenderVerifierContext(s, challenge)

	// All calls should produce identical output (deterministic)
	if result1 != result2 || result2 != result3 {
		t.Errorf("RenderVerifierContext produced inconsistent output:\n1: %q\n2: %q\n3: %q",
			result1, result2, result3)
	}
}

// TestRenderVerifierContext_SpecialCharactersInReason tests handling of special characters.
func TestRenderVerifierContext_SpecialCharactersInReason(t *testing.T) {
	tests := []struct {
		name   string
		reason string
	}{
		{
			name:   "unicode math symbols",
			reason: "Missing proof that x in set: P(x) implies Q(x)",
		},
		{
			name:   "quotes",
			reason: `The term "natural number" is misused`,
		},
		{
			name:   "newlines in reason",
			reason: "First issue\nSecond issue\nThird issue",
		},
		{
			name:   "tabs and whitespace",
			reason: "With\ttabs\tand  multiple  spaces",
		},
		{
			name:   "backslashes for LaTeX",
			reason: `Missing proof for \alpha + \beta = \gamma`,
		},
		{
			name:   "angle brackets",
			reason: "Type mismatch: expected <Int> got <String>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()
			n := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "A claim", schema.InferenceModusPonens)
			s.AddNode(n)

			challenge := makeTestChallenge("chal-special", "1", schema.TargetStatement, tt.reason)

			// Should not panic
			result := RenderVerifierContext(s, challenge)

			// Should return non-empty result
			if result == "" {
				t.Fatalf("RenderVerifierContext returned empty for reason with %s", tt.name)
			}

			// Should contain challenge ID at minimum
			if !strings.Contains(result, "chal-special") {
				t.Errorf("RenderVerifierContext missing challenge ID, got: %q", result)
			}
		})
	}
}

// TestRenderVerifierContext_WithExternals tests rendering context with externals in scope.
func TestRenderVerifierContext_WithExternals(t *testing.T) {
	s := state.NewState()

	// Create a node with context references to externals
	nodeID, _ := types.Parse("1")
	n, err := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"By the fundamental theorem of arithmetic",
		schema.InferenceAssumption,
		node.NodeOptions{
			Context: []string{"ext:fta"},
		},
	)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	s.AddNode(n)

	// Add externals to state
	ext := node.NewExternalWithNotes("Fundamental Theorem of Arithmetic", "Hardy & Wright, Chapter 1", "Uniqueness of factorization")
	s.AddExternal(&ext)

	// Create a challenge about the external reference
	challenge := makeTestChallenge("chal-ext", "1", schema.TargetContext, "External reference is incorrect")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string")
	}

	// Should indicate externals in scope
	if !strings.Contains(strings.ToLower(result), "external") && !strings.Contains(result, "Fundamental") {
		t.Logf("Note: RenderVerifierContext may want to show available externals, got: %q", result)
	}
}

// TestRenderVerifierContext_Header tests that output includes appropriate header.
func TestRenderVerifierContext_Header(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "A claim", schema.InferenceModusPonens)
	s.AddNode(n)

	challenge := makeTestChallenge("chal-header", "1", schema.TargetStatement, "Reason")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string")
	}

	// Should include some form of header indicating this is verifier context
	if !strings.Contains(strings.ToLower(result), "verifier") && !strings.Contains(strings.ToLower(result), "challenge") {
		t.Logf("Note: RenderVerifierContext may want to include a header, got: %q", result)
	}
}

// TestRenderVerifierContext_RootNodeChallenge tests context for a challenge against root node.
func TestRenderVerifierContext_RootNodeChallenge(t *testing.T) {
	s := state.NewState()
	rootNode := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "The main theorem to prove", schema.InferenceAssumption)
	s.AddNode(rootNode)

	challenge := makeTestChallenge("chal-root", "1", schema.TargetStatement, "Main theorem is unclear")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string for valid root node challenge")
	}

	// Should contain the node ID
	if !strings.Contains(result, "1") {
		t.Errorf("RenderVerifierContext missing node ID, got: %q", result)
	}

	// Should contain the statement
	if !strings.Contains(result, "main theorem") || !strings.Contains(result, "prove") {
		t.Errorf("RenderVerifierContext missing statement content, got: %q", result)
	}

	// Should indicate this is the root (no parent to show)
	// The output format may vary, but should not contain misleading parent info
}

// TestRenderVerifierContext_ChallengeWithResolution tests that resolved challenges show resolution.
func TestRenderVerifierContext_ChallengeWithResolution(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "A claim that was challenged", schema.InferenceModusPonens)
	s.AddNode(n)

	challenge := makeTestChallenge("chal-resolved", "1", schema.TargetStatement, "Statement was unclear")
	_ = challenge.Resolve("Clarified the statement by adding formal definition")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string")
	}

	// Should show resolution for resolved challenges
	if !strings.Contains(result, "Clarified") && !strings.Contains(result, "resolution") {
		t.Logf("Note: RenderVerifierContext may want to show resolution for resolved challenges, got: %q", result)
	}
}

// TestRenderVerifierContext_WorkflowState tests that workflow state is appropriately shown.
func TestRenderVerifierContext_WorkflowState(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForVerifier("1", schema.NodeTypeClaim, "Claimed node", schema.InferenceModusPonens)
	// Simulate claimed state
	n.WorkflowState = schema.WorkflowClaimed
	n.ClaimedBy = "prover-agent-123"

	s.AddNode(n)

	challenge := makeTestChallenge("chal-workflow", "1", schema.TargetStatement, "Statement needs clarification")

	result := RenderVerifierContext(s, challenge)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerifierContext returned empty string")
	}

	// Should show workflow/claim information
	// This helps the verifier understand who is working on this node
	if !strings.Contains(strings.ToLower(result), "claimed") &&
		!strings.Contains(result, "prover-agent") {
		t.Logf("Note: RenderVerifierContext may want to show claim status, got: %q", result)
	}
}
