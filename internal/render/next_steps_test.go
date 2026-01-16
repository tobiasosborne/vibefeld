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

// Helper to create a test node for next steps tests.
// Panics on invalid input (intended for test use only).
func makeNextStepsTestNode(
	id string,
	statement string,
	workflow schema.WorkflowState,
	epistemic schema.EpistemicState,
) *node.Node {
	nodeID, err := types.Parse(id)
	if err != nil {
		panic("invalid test node ID: " + id)
	}
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, statement, schema.InferenceModusPonens)
	if err != nil {
		panic("failed to create test node: " + err.Error())
	}
	n.WorkflowState = workflow
	n.EpistemicState = epistemic
	return n
}

// =============================================================================
// SuggestNextSteps Tests
// =============================================================================

// TestSuggestNextSteps_NilState tests that nil state is handled gracefully.
func TestSuggestNextSteps_NilState(t *testing.T) {
	ctx := Context{
		Role:           "prover",
		CurrentCommand: "status",
		State:          nil,
	}

	steps := SuggestNextSteps(ctx)

	// Should suggest init when state is nil
	if len(steps) == 0 {
		t.Fatal("SuggestNextSteps should return suggestions for nil state")
	}

	// Should suggest init command
	hasInit := false
	for _, step := range steps {
		if strings.Contains(step.Command, "init") {
			hasInit = true
			break
		}
	}
	if !hasInit {
		t.Error("SuggestNextSteps for nil state should suggest 'af init'")
	}
}

// TestSuggestNextSteps_EmptyState tests suggestions for empty state (no nodes).
func TestSuggestNextSteps_EmptyState(t *testing.T) {
	s := state.NewState()
	ctx := Context{
		Role:           "prover",
		CurrentCommand: "status",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	// Should suggest init when state is empty
	if len(steps) == 0 {
		t.Fatal("SuggestNextSteps should return suggestions for empty state")
	}

	// Should suggest init command
	hasInit := false
	for _, step := range steps {
		if strings.Contains(step.Command, "init") {
			hasInit = true
			break
		}
	}
	if !hasInit {
		t.Error("SuggestNextSteps for empty state should suggest 'af init'")
	}
}

// TestSuggestNextSteps_ProverAfterInit tests prover suggestions after init.
func TestSuggestNextSteps_ProverAfterInit(t *testing.T) {
	s := state.NewState()
	n := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowAvailable, schema.EpistemicPending)
	s.AddNode(n)

	ctx := Context{
		Role:           "prover",
		CurrentCommand: "init",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	if len(steps) == 0 {
		t.Fatal("SuggestNextSteps should return suggestions after init")
	}

	// Should suggest claiming the root node
	hasClaim := false
	for _, step := range steps {
		if strings.Contains(step.Command, "claim") && strings.Contains(step.Command, "1") {
			hasClaim = true
			break
		}
	}
	if !hasClaim {
		t.Error("SuggestNextSteps for prover after init should suggest 'af claim 1'")
	}
}

// TestSuggestNextSteps_ProverAfterClaim tests prover suggestions after claiming.
func TestSuggestNextSteps_ProverAfterClaim(t *testing.T) {
	s := state.NewState()
	n := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowClaimed, schema.EpistemicPending)
	n.ClaimedBy = "prover-agent"
	s.AddNode(n)

	ctx := Context{
		Role:           "prover",
		CurrentCommand: "claim",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	if len(steps) == 0 {
		t.Fatal("SuggestNextSteps should return suggestions after claim")
	}

	// Should suggest refining the claimed node
	hasRefine := false
	for _, step := range steps {
		if strings.Contains(step.Command, "refine") {
			hasRefine = true
			break
		}
	}
	if !hasRefine {
		t.Error("SuggestNextSteps for prover after claim should suggest 'af refine'")
	}
}

// TestSuggestNextSteps_ProverAfterRefine tests prover suggestions after refining.
func TestSuggestNextSteps_ProverAfterRefine(t *testing.T) {
	s := state.NewState()
	// Parent is claimed and being refined
	parent := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowClaimed, schema.EpistemicPending)
	parent.ClaimedBy = "prover-agent"
	// Children created from refinement
	child1 := makeNextStepsTestNode("1.1", "First step", schema.WorkflowAvailable, schema.EpistemicPending)
	child2 := makeNextStepsTestNode("1.2", "Second step", schema.WorkflowAvailable, schema.EpistemicPending)
	s.AddNode(parent)
	s.AddNode(child1)
	s.AddNode(child2)

	ctx := Context{
		Role:           "prover",
		CurrentCommand: "refine",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	if len(steps) == 0 {
		t.Fatal("SuggestNextSteps should return suggestions after refine")
	}

	// Should suggest releasing the node
	hasRelease := false
	for _, step := range steps {
		if strings.Contains(step.Command, "release") {
			hasRelease = true
			break
		}
	}
	if !hasRelease {
		t.Error("SuggestNextSteps for prover after refine should suggest 'af release'")
	}
}

// TestSuggestNextSteps_VerifierWithPendingNodes tests verifier suggestions with pending nodes.
func TestSuggestNextSteps_VerifierWithPendingNodes(t *testing.T) {
	s := state.NewState()
	// Node ready for verification (claimed, pending, no pending children)
	n := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowClaimed, schema.EpistemicPending)
	s.AddNode(n)

	ctx := Context{
		Role:           "verifier",
		CurrentCommand: "status",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	if len(steps) == 0 {
		t.Fatal("SuggestNextSteps should return suggestions for verifier")
	}

	// Should suggest accepting or challenging
	hasAcceptOrChallenge := false
	for _, step := range steps {
		lower := strings.ToLower(step.Command)
		if strings.Contains(lower, "accept") || strings.Contains(lower, "challenge") {
			hasAcceptOrChallenge = true
			break
		}
	}
	if !hasAcceptOrChallenge {
		t.Error("SuggestNextSteps for verifier with pending nodes should suggest accept or challenge")
	}
}

// TestSuggestNextSteps_VerifierAfterChallenge tests verifier suggestions after challenge.
func TestSuggestNextSteps_VerifierAfterChallenge(t *testing.T) {
	s := state.NewState()
	n := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowClaimed, schema.EpistemicPending)
	s.AddNode(n)

	// Add an open challenge
	ch := &state.Challenge{
		ID:     "ch-1",
		NodeID: n.ID,
		Target: "statement",
		Reason: "Unclear statement",
		Status: "open",
	}
	s.AddChallenge(ch)

	ctx := Context{
		Role:           "verifier",
		CurrentCommand: "challenge",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	if len(steps) == 0 {
		t.Fatal("SuggestNextSteps should return suggestions after challenge")
	}

	// Should suggest checking status
	hasStatus := false
	for _, step := range steps {
		if strings.Contains(step.Command, "status") {
			hasStatus = true
			break
		}
	}
	if !hasStatus {
		t.Error("SuggestNextSteps for verifier after challenge should suggest 'af status'")
	}
}

// TestSuggestNextSteps_ContextWithChallenges tests suggestions when challenges exist.
func TestSuggestNextSteps_ContextWithChallenges(t *testing.T) {
	s := state.NewState()
	n := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowClaimed, schema.EpistemicPending)
	s.AddNode(n)

	// Add an open challenge
	ch := &state.Challenge{
		ID:     "ch-1",
		NodeID: n.ID,
		Target: "statement",
		Reason: "Unclear statement",
		Status: "open",
	}
	s.AddChallenge(ch)

	ctx := Context{
		Role:           "prover",
		CurrentCommand: "status",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	if len(steps) == 0 {
		t.Fatal("SuggestNextSteps should return suggestions when challenges exist")
	}

	// Should suggest viewing the challenge
	hasShow := false
	for _, step := range steps {
		if strings.Contains(step.Command, "show") || strings.Contains(step.Command, "challenge") {
			hasShow = true
			break
		}
	}
	if !hasShow {
		t.Error("SuggestNextSteps with challenges should suggest viewing challenge details")
	}
}

// TestSuggestNextSteps_UnknownRoleDefaults tests that unknown role gets reasonable defaults.
func TestSuggestNextSteps_UnknownRoleDefaults(t *testing.T) {
	s := state.NewState()
	n := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowAvailable, schema.EpistemicPending)
	s.AddNode(n)

	ctx := Context{
		Role:           "unknown",
		CurrentCommand: "status",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	// Should return suggestions even for unknown role
	if len(steps) == 0 {
		t.Fatal("SuggestNextSteps should return suggestions for unknown role")
	}

	// Should suggest help as a fallback
	hasHelp := false
	for _, step := range steps {
		if strings.Contains(step.Command, "help") || strings.Contains(step.Command, "status") {
			hasHelp = true
			break
		}
	}
	if !hasHelp {
		t.Error("SuggestNextSteps for unknown role should suggest help or status")
	}
}

// TestSuggestNextSteps_EmptyRoleDefaults tests that empty role gets reasonable defaults.
func TestSuggestNextSteps_EmptyRoleDefaults(t *testing.T) {
	s := state.NewState()
	n := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowAvailable, schema.EpistemicPending)
	s.AddNode(n)

	ctx := Context{
		Role:           "",
		CurrentCommand: "status",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	// Should return suggestions even for empty role
	if len(steps) == 0 {
		t.Fatal("SuggestNextSteps should return suggestions for empty role")
	}
}

// TestSuggestNextSteps_PriorityOrder tests that steps are ordered by priority.
func TestSuggestNextSteps_PriorityOrder(t *testing.T) {
	s := state.NewState()
	n := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowAvailable, schema.EpistemicPending)
	s.AddNode(n)

	ctx := Context{
		Role:           "prover",
		CurrentCommand: "init",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	if len(steps) < 2 {
		t.Skip("Not enough steps to test priority order")
	}

	// Steps should be in ascending priority order (lower number = higher priority)
	for i := 1; i < len(steps); i++ {
		if steps[i-1].Priority > steps[i].Priority {
			t.Errorf("Steps not in priority order: %d > %d", steps[i-1].Priority, steps[i].Priority)
		}
	}
}

// TestSuggestNextSteps_ContainsDescriptions tests that all steps have descriptions.
func TestSuggestNextSteps_ContainsDescriptions(t *testing.T) {
	s := state.NewState()
	n := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowAvailable, schema.EpistemicPending)
	s.AddNode(n)

	ctx := Context{
		Role:           "prover",
		CurrentCommand: "init",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	for i, step := range steps {
		if step.Command == "" {
			t.Errorf("Step %d has empty command", i)
		}
		if step.Description == "" {
			t.Errorf("Step %d (%s) has empty description", i, step.Command)
		}
	}
}

// TestSuggestNextSteps_TableDriven tests various context scenarios.
func TestSuggestNextSteps_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		command        string
		setupState     func(*state.State)
		wantContains   []string // Commands that should be suggested
		wantNotContain []string // Commands that should NOT be suggested
	}{
		{
			name:    "prover with available nodes",
			role:    "prover",
			command: "status",
			setupState: func(s *state.State) {
				n := makeNextStepsTestNode("1.1", "Available step", schema.WorkflowAvailable, schema.EpistemicPending)
				s.AddNode(n)
			},
			wantContains: []string{"claim"},
		},
		{
			name:    "prover with claimed node",
			role:    "prover",
			command: "status",
			setupState: func(s *state.State) {
				n := makeNextStepsTestNode("1", "Claimed step", schema.WorkflowClaimed, schema.EpistemicPending)
				n.ClaimedBy = "prover-agent"
				s.AddNode(n)
			},
			wantContains: []string{"refine"},
		},
		{
			name:    "verifier with nodes ready for review",
			role:    "verifier",
			command: "status",
			setupState: func(s *state.State) {
				n := makeNextStepsTestNode("1", "Ready for review", schema.WorkflowClaimed, schema.EpistemicPending)
				s.AddNode(n)
			},
			wantContains: []string{"accept"},
		},
		{
			name:    "all nodes validated",
			role:    "prover",
			command: "status",
			setupState: func(s *state.State) {
				n := makeNextStepsTestNode("1", "Validated", schema.WorkflowClaimed, schema.EpistemicValidated)
				s.AddNode(n)
			},
			wantContains: []string{"status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()
			if tt.setupState != nil {
				tt.setupState(s)
			}

			ctx := Context{
				Role:           tt.role,
				CurrentCommand: tt.command,
				State:          s,
			}

			steps := SuggestNextSteps(ctx)

			// Check wantContains
			for _, want := range tt.wantContains {
				found := false
				for _, step := range steps {
					if strings.Contains(strings.ToLower(step.Command), want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestion containing %q, got: %v", want, steps)
				}
			}

			// Check wantNotContain
			for _, notWant := range tt.wantNotContain {
				for _, step := range steps {
					if strings.Contains(strings.ToLower(step.Command), notWant) {
						t.Errorf("Did not expect suggestion containing %q, got: %v", notWant, step.Command)
					}
				}
			}
		})
	}
}

// =============================================================================
// RenderNextSteps Tests
// =============================================================================

// TestRenderNextSteps_EmptySteps tests rendering empty step list.
func TestRenderNextSteps_EmptySteps(t *testing.T) {
	result := RenderNextSteps(nil)

	// Should return empty or minimal output for no steps
	if result != "" && !strings.Contains(result, "No") && !strings.Contains(result, "no") {
		t.Logf("Note: RenderNextSteps for empty may want to indicate no suggestions, got: %q", result)
	}
}

// TestRenderNextSteps_SingleStep tests rendering a single step.
func TestRenderNextSteps_SingleStep(t *testing.T) {
	steps := []NextStep{
		{Command: "af claim 1", Description: "Claim the root node", Priority: 0},
	}

	result := RenderNextSteps(steps)

	if result == "" {
		t.Fatal("RenderNextSteps returned empty string for single step")
	}

	// Should contain the command
	if !strings.Contains(result, "af claim 1") {
		t.Errorf("RenderNextSteps missing command, got: %q", result)
	}

	// Should contain the description
	if !strings.Contains(result, "Claim the root node") {
		t.Errorf("RenderNextSteps missing description, got: %q", result)
	}
}

// TestRenderNextSteps_MultipleSteps tests rendering multiple steps.
func TestRenderNextSteps_MultipleSteps(t *testing.T) {
	steps := []NextStep{
		{Command: "af claim 1.2", Description: "Claim the available node", Priority: 0},
		{Command: "af status", Description: "Check proof status", Priority: 1},
	}

	result := RenderNextSteps(steps)

	if result == "" {
		t.Fatal("RenderNextSteps returned empty string for multiple steps")
	}

	// Should contain all commands
	if !strings.Contains(result, "af claim 1.2") {
		t.Errorf("RenderNextSteps missing first command, got: %q", result)
	}
	if !strings.Contains(result, "af status") {
		t.Errorf("RenderNextSteps missing second command, got: %q", result)
	}

	// Should be multi-line
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Logf("Note: RenderNextSteps may benefit from multi-line format, got: %q", result)
	}
}

// TestRenderNextSteps_ContainsHeader tests that output has a header.
func TestRenderNextSteps_ContainsHeader(t *testing.T) {
	steps := []NextStep{
		{Command: "af claim 1", Description: "Claim the root node", Priority: 0},
	}

	result := RenderNextSteps(steps)

	// Should contain "Next steps" header or similar
	lower := strings.ToLower(result)
	if !strings.Contains(lower, "next") {
		t.Logf("Note: RenderNextSteps may benefit from 'Next steps' header, got: %q", result)
	}
}

// TestRenderNextSteps_ContainsArrow tests that output uses arrow formatting.
func TestRenderNextSteps_ContainsArrow(t *testing.T) {
	steps := []NextStep{
		{Command: "af claim 1", Description: "Claim the root node", Priority: 0},
	}

	result := RenderNextSteps(steps)

	// Expected format uses arrow: "  -> af claim 1"
	// This is optional but preferred
	if !strings.Contains(result, "->") && !strings.Contains(result, "=>") && !strings.Contains(result, "-") {
		t.Logf("Note: RenderNextSteps may benefit from arrow formatting, got: %q", result)
	}
}

// TestRenderNextSteps_OutputFormat tests the expected output format.
func TestRenderNextSteps_OutputFormat(t *testing.T) {
	steps := []NextStep{
		{Command: "af claim 1.2", Description: "Claim the available node for work", Priority: 0},
		{Command: "af status", Description: "Check proof status", Priority: 1},
	}

	result := RenderNextSteps(steps)

	// Should produce human-readable output
	if result == "" {
		t.Fatal("RenderNextSteps returned empty string")
	}

	// Should not be JSON
	if strings.HasPrefix(strings.TrimSpace(result), "{") || strings.HasPrefix(strings.TrimSpace(result), "[") {
		t.Errorf("RenderNextSteps should produce human-readable text, not JSON, got: %q", result)
	}

	// Should not contain Go struct formatting
	if strings.Contains(result, "&{") || strings.Contains(result, "NextStep") {
		t.Errorf("RenderNextSteps should not contain struct formatting, got: %q", result)
	}
}

// TestRenderNextSteps_ConsistentOutput tests that output is deterministic.
func TestRenderNextSteps_ConsistentOutput(t *testing.T) {
	steps := []NextStep{
		{Command: "af claim 1", Description: "Claim the root node", Priority: 0},
		{Command: "af status", Description: "Check status", Priority: 1},
	}

	result1 := RenderNextSteps(steps)
	result2 := RenderNextSteps(steps)
	result3 := RenderNextSteps(steps)

	if result1 != result2 || result2 != result3 {
		t.Error("RenderNextSteps output is not deterministic")
	}
}

// TestRenderNextSteps_Alignment tests that commands and descriptions are aligned.
func TestRenderNextSteps_Alignment(t *testing.T) {
	steps := []NextStep{
		{Command: "af claim 1", Description: "Short desc", Priority: 0},
		{Command: "af challenge 1.2.3", Description: "Longer description here", Priority: 1},
	}

	result := RenderNextSteps(steps)

	// Should produce readable output (alignment is nice-to-have)
	lines := strings.Split(strings.TrimSpace(result), "\n")

	// Skip header line if present
	contentLines := 0
	for _, line := range lines {
		if strings.Contains(line, "af ") {
			contentLines++
		}
	}

	if contentLines < 2 {
		t.Logf("Note: Expected at least 2 content lines with commands, got %d lines total", len(lines))
	}
}

// TestRenderNextSteps_SpecialCharacters tests handling of special characters.
func TestRenderNextSteps_SpecialCharacters(t *testing.T) {
	steps := []NextStep{
		{Command: "af show ch-abc123", Description: "View challenge details", Priority: 0},
		{Command: "af refine 1.2.3 -s 'statement'", Description: "Refine with statement", Priority: 1},
	}

	// Should not panic
	result := RenderNextSteps(steps)

	// Should produce output
	if result == "" {
		t.Error("RenderNextSteps returned empty for special characters")
	}

	// Should contain the commands
	if !strings.Contains(result, "ch-abc123") {
		t.Errorf("RenderNextSteps missing special char command, got: %q", result)
	}
}

// =============================================================================
// NextStep Type Tests
// =============================================================================

// TestNextStep_ZeroValue tests the zero value of NextStep.
func TestNextStep_ZeroValue(t *testing.T) {
	var step NextStep

	// Zero value should have empty strings and 0 priority
	if step.Command != "" {
		t.Errorf("Zero value Command should be empty, got %q", step.Command)
	}
	if step.Description != "" {
		t.Errorf("Zero value Description should be empty, got %q", step.Description)
	}
	if step.Priority != 0 {
		t.Errorf("Zero value Priority should be 0, got %d", step.Priority)
	}
}

// TestContext_ZeroValue tests the zero value of Context.
func TestContext_ZeroValue(t *testing.T) {
	var ctx Context

	// Zero value should have empty strings and nil state
	if ctx.Role != "" {
		t.Errorf("Zero value Role should be empty, got %q", ctx.Role)
	}
	if ctx.CurrentCommand != "" {
		t.Errorf("Zero value CurrentCommand should be empty, got %q", ctx.CurrentCommand)
	}
	if ctx.State != nil {
		t.Errorf("Zero value State should be nil, got %v", ctx.State)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestSuggestAndRender_Integration tests the full workflow of suggesting and rendering.
func TestSuggestAndRender_Integration(t *testing.T) {
	s := state.NewState()
	n := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowAvailable, schema.EpistemicPending)
	s.AddNode(n)

	ctx := Context{
		Role:           "prover",
		CurrentCommand: "init",
		State:          s,
	}

	// Generate suggestions
	steps := SuggestNextSteps(ctx)

	if len(steps) == 0 {
		t.Fatal("No suggestions generated")
	}

	// Render the suggestions
	output := RenderNextSteps(steps)

	if output == "" {
		t.Fatal("Rendered output is empty")
	}

	// Output should contain suggestion content
	if !strings.Contains(output, "af") {
		t.Errorf("Rendered output should contain 'af' commands, got: %q", output)
	}
}

// TestSuggestNextSteps_ProverWorkflow tests full prover workflow suggestions.
func TestSuggestNextSteps_ProverWorkflow(t *testing.T) {
	// Scenario: proof with multiple nodes in different states
	s := state.NewState()

	// Root is claimed
	root := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowClaimed, schema.EpistemicPending)
	root.ClaimedBy = "prover-agent"

	// Some children available
	child1 := makeNextStepsTestNode("1.1", "First lemma", schema.WorkflowAvailable, schema.EpistemicPending)
	child2 := makeNextStepsTestNode("1.2", "Second lemma", schema.WorkflowClaimed, schema.EpistemicValidated)

	s.AddNode(root)
	s.AddNode(child1)
	s.AddNode(child2)

	ctx := Context{
		Role:           "prover",
		CurrentCommand: "status",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	// Should have suggestions
	if len(steps) == 0 {
		t.Fatal("No suggestions for complex prover state")
	}

	// Should suggest claiming available node (1.1)
	hasClaim := false
	for _, step := range steps {
		if strings.Contains(step.Command, "claim") {
			hasClaim = true
			break
		}
	}
	if !hasClaim {
		t.Error("Should suggest claiming available node 1.1")
	}
}

// TestSuggestNextSteps_VerifierWorkflow tests full verifier workflow suggestions.
func TestSuggestNextSteps_VerifierWorkflow(t *testing.T) {
	// Scenario: proof with nodes ready for verification
	s := state.NewState()

	// Node ready for verification (all children validated)
	parent := makeNextStepsTestNode("1", "Main theorem", schema.WorkflowClaimed, schema.EpistemicPending)
	child1 := makeNextStepsTestNode("1.1", "First lemma", schema.WorkflowClaimed, schema.EpistemicValidated)
	child2 := makeNextStepsTestNode("1.2", "Second lemma", schema.WorkflowClaimed, schema.EpistemicValidated)

	s.AddNode(parent)
	s.AddNode(child1)
	s.AddNode(child2)

	ctx := Context{
		Role:           "verifier",
		CurrentCommand: "status",
		State:          s,
	}

	steps := SuggestNextSteps(ctx)

	// Should have suggestions
	if len(steps) == 0 {
		t.Fatal("No suggestions for verifier workflow")
	}

	// Should suggest accepting the parent node (all children validated)
	hasAccept := false
	for _, step := range steps {
		if strings.Contains(strings.ToLower(step.Command), "accept") {
			hasAccept = true
			break
		}
	}
	if !hasAccept {
		t.Error("Should suggest accepting node 1 (all children validated)")
	}
}
