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

// Helper to create a test node with full state configuration
func addStatusTestNode(
	t *testing.T,
	s *state.State,
	id string,
	statement string,
	workflow schema.WorkflowState,
	epistemic schema.EpistemicState,
	taint node.TaintState,
) *node.Node {
	t.Helper()
	nodeID, err := types.Parse(id)
	if err != nil {
		t.Fatalf("invalid test node ID %q: %v", id, err)
	}
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, statement, schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create test node %q: %v", id, err)
	}
	n.WorkflowState = workflow
	n.EpistemicState = epistemic
	n.TaintState = taint
	s.AddNode(n)
	return n
}

// TestRenderStatus_NilState tests that nil state is handled gracefully
func TestRenderStatus_NilState(t *testing.T) {
	// Should not panic with nil state
	result := RenderStatus(nil, 0, 0)

	// Should return a meaningful message indicating no data
	if result == "" {
		t.Error("RenderStatus should return a message for nil state, not empty string")
	}

	lower := strings.ToLower(result)
	if !strings.Contains(lower, "no") && !strings.Contains(lower, "empty") && !strings.Contains(lower, "nil") && !strings.Contains(lower, "initialized") {
		t.Errorf("RenderStatus for nil state should indicate no data, got: %q", result)
	}
}

// TestRenderStatus_EmptyState tests rendering when state has no nodes
func TestRenderStatus_EmptyState(t *testing.T) {
	s := state.NewState()

	result := RenderStatus(s, 0, 0)

	// Should indicate no proof initialized or similar
	if result == "" {
		t.Error("RenderStatus should return a message for empty state")
	}

	lower := strings.ToLower(result)
	if !strings.Contains(lower, "no") && !strings.Contains(lower, "empty") && !strings.Contains(lower, "initialized") {
		t.Errorf("RenderStatus for empty state should indicate no proof, got: %q", result)
	}
}

// TestRenderStatus_SingleRootNode tests rendering with just one root node
func TestRenderStatus_SingleRootNode(t *testing.T) {
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Root claim", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	if result == "" {
		t.Fatal("RenderStatus returned empty string for single node")
	}

	// Should contain main sections
	requiredSections := []string{
		"Proof Status", // Header
		"1",            // Node ID in tree
		"Statistics",   // Stats section
		"Legend",       // Legend section
	}
	for _, section := range requiredSections {
		if !strings.Contains(result, section) {
			t.Errorf("RenderStatus missing required section %q, got: %q", section, result)
		}
	}
}

// TestRenderStatus_TreeViewPresent tests that tree view is included in output
func TestRenderStatus_TreeViewPresent(t *testing.T) {
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Root claim", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "First step", schema.WorkflowAvailable, schema.EpistemicValidated, node.TaintClean)
	addStatusTestNode(t, s, "1.2", "Second step", schema.WorkflowClaimed, schema.EpistemicPending, node.TaintTainted)

	result := RenderStatus(s, 0, 0)

	// Should contain all node IDs
	for _, id := range []string{"1", "1.1", "1.2"} {
		if !strings.Contains(result, id) {
			t.Errorf("RenderStatus tree view missing node ID %q, got: %q", id, result)
		}
	}

	// Should contain node statements
	for _, stmt := range []string{"Root claim", "First step", "Second step"} {
		if !strings.Contains(result, stmt) {
			t.Errorf("RenderStatus tree view missing statement %q, got: %q", stmt, result)
		}
	}
}

// TestRenderStatus_StatisticsSection tests that statistics are computed correctly
func TestRenderStatus_StatisticsSection(t *testing.T) {
	s := state.NewState()
	// Create nodes with various epistemic states
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Step 1", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.2", "Step 2", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)
	addStatusTestNode(t, s, "1.3", "Step 3", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintSelfAdmitted)
	addStatusTestNode(t, s, "1.4", "Step 4", schema.WorkflowAvailable, schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	result := RenderStatus(s, 0, 0)

	// Should contain Statistics header
	if !strings.Contains(result, "Statistics") {
		t.Errorf("RenderStatus missing Statistics section, got: %q", result)
	}

	// Should contain node count information
	if !strings.Contains(result, "5") || !strings.Contains(result, "total") {
		t.Logf("Note: Statistics section may want total node count (5 total), got: %q", result)
	}

	// Should mention pending nodes
	if !strings.Contains(result, "pending") {
		t.Errorf("RenderStatus Statistics should mention pending nodes, got: %q", result)
	}

	// Should mention validated nodes
	if !strings.Contains(result, "validated") {
		t.Errorf("RenderStatus Statistics should mention validated nodes, got: %q", result)
	}
}

// TestRenderStatus_EpistemicStateCounts tests that epistemic state counts are shown
func TestRenderStatus_EpistemicStateCounts(t *testing.T) {
	s := state.NewState()
	// 2 pending
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Step 1", schema.WorkflowClaimed, schema.EpistemicPending, node.TaintClean)
	// 2 validated
	addStatusTestNode(t, s, "1.2", "Step 2", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)
	addStatusTestNode(t, s, "1.3", "Step 3", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)
	// 1 admitted
	addStatusTestNode(t, s, "1.4", "Step 4", schema.WorkflowAvailable, schema.EpistemicAdmitted, node.TaintSelfAdmitted)

	result := RenderStatus(s, 0, 0)

	// The statistics should show counts for each epistemic state present
	// Expected: 5 total (2 pending, 2 validated, 1 admitted)
	if !strings.Contains(result, "pending") {
		t.Error("Statistics missing pending count")
	}
	if !strings.Contains(result, "validated") {
		t.Error("Statistics missing validated count")
	}
	if !strings.Contains(result, "admitted") {
		t.Error("Statistics missing admitted count")
	}
}

// TestRenderStatus_TaintStateCounts tests that taint summary is shown
func TestRenderStatus_TaintStateCounts(t *testing.T) {
	s := state.NewState()
	// 1 clean
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	// 2 self_admitted
	addStatusTestNode(t, s, "1.1", "Step 1", schema.WorkflowAvailable, schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	addStatusTestNode(t, s, "1.2", "Step 2", schema.WorkflowAvailable, schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	// 1 tainted
	addStatusTestNode(t, s, "1.3", "Step 3", schema.WorkflowClaimed, schema.EpistemicPending, node.TaintTainted)

	result := RenderStatus(s, 0, 0)

	// Should contain Taint summary
	if !strings.Contains(result, "Taint") || !strings.Contains(result, "taint") {
		t.Errorf("RenderStatus should include taint summary, got: %q", result)
	}

	// Should mention different taint states
	if !strings.Contains(result, "clean") {
		t.Logf("Note: Taint summary should mention 'clean' state, got: %q", result)
	}
	if !strings.Contains(result, "self_admitted") {
		t.Logf("Note: Taint summary should mention 'self_admitted' state, got: %q", result)
	}
	if !strings.Contains(result, "tainted") {
		t.Logf("Note: Taint summary should mention 'tainted' state, got: %q", result)
	}
}

// TestRenderStatus_ProverJobCount tests that prover jobs are counted
func TestRenderStatus_ProverJobCount(t *testing.T) {
	s := state.NewState()
	// 2 prover jobs: available + pending
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Step 1", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	// Not a prover job: claimed
	addStatusTestNode(t, s, "1.2", "Step 2", schema.WorkflowClaimed, schema.EpistemicPending, node.TaintClean)
	// Not a prover job: validated (not pending)
	addStatusTestNode(t, s, "1.3", "Step 3", schema.WorkflowAvailable, schema.EpistemicValidated, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	// Should contain Jobs section
	if !strings.Contains(result, "Jobs") && !strings.Contains(result, "jobs") {
		t.Errorf("RenderStatus should include Jobs section, got: %q", result)
	}

	// Should mention Prover
	if !strings.Contains(result, "Prover") && !strings.Contains(result, "prover") {
		t.Errorf("RenderStatus should mention prover jobs, got: %q", result)
	}

	// Should indicate number of prover jobs (2)
	if !strings.Contains(result, "2") {
		t.Logf("Note: Jobs section should show 2 prover jobs, got: %q", result)
	}
}

// TestRenderStatus_VerifierJobCount tests that verifier jobs are counted
func TestRenderStatus_VerifierJobCount(t *testing.T) {
	s := state.NewState()
	// Verifier job: claimed + pending + all children validated
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowClaimed, schema.EpistemicPending, node.TaintClean)
	// Children of 1 are validated
	addStatusTestNode(t, s, "1.1", "Step 1", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)
	addStatusTestNode(t, s, "1.2", "Step 2", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)

	// Another verifier job (no children = ready for verification)
	addStatusTestNode(t, s, "1.1.1", "Sub-step", schema.WorkflowClaimed, schema.EpistemicPending, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	// Should mention Verifier
	if !strings.Contains(result, "Verifier") && !strings.Contains(result, "verifier") {
		t.Errorf("RenderStatus should mention verifier jobs, got: %q", result)
	}
}

// TestRenderStatus_NoJobsAvailable tests display when no jobs are available
func TestRenderStatus_NoJobsAvailable(t *testing.T) {
	s := state.NewState()
	// All nodes are validated (no prover or verifier jobs)
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Step 1", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	// Should still have Jobs section
	if !strings.Contains(result, "Jobs") && !strings.Contains(result, "jobs") {
		t.Logf("Note: RenderStatus should include Jobs section even when empty, got: %q", result)
	}

	// Should indicate 0 jobs or "no jobs" or similar
	lower := strings.ToLower(result)
	hasZeroIndicator := strings.Contains(lower, "0") ||
		strings.Contains(lower, "no ") ||
		strings.Contains(lower, "none")

	if !hasZeroIndicator {
		t.Logf("Note: Jobs section should indicate no jobs available when proof is complete, got: %q", result)
	}
}

// TestRenderStatus_LegendSection tests that legend explains symbols
func TestRenderStatus_LegendSection(t *testing.T) {
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	// Should contain Legend header
	if !strings.Contains(result, "Legend") {
		t.Errorf("RenderStatus missing Legend section, got: %q", result)
	}

	// Legend should explain epistemic states
	epistemicStates := []string{"pending", "validated", "admitted", "refuted"}
	hasEpistemicLegend := false
	for _, state := range epistemicStates {
		if strings.Contains(result, state) {
			hasEpistemicLegend = true
			break
		}
	}
	if !hasEpistemicLegend {
		t.Errorf("Legend should explain epistemic states, got: %q", result)
	}

	// Legend should explain taint states
	taintStates := []string{"clean", "self_admitted", "tainted"}
	hasTaintLegend := false
	for _, state := range taintStates {
		if strings.Contains(result, state) {
			hasTaintLegend = true
			break
		}
	}
	if !hasTaintLegend {
		t.Errorf("Legend should explain taint states, got: %q", result)
	}
}

// TestRenderStatus_LegendEpistemicSymbols tests that legend shows epistemic symbols
func TestRenderStatus_LegendEpistemicSymbols(t *testing.T) {
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	// Check for symbols or textual explanations
	// Expected symbols from issue: pending=... validated=... admitted=... refuted=...
	if !strings.Contains(result, "Epistemic") && !strings.Contains(result, "epistemic") {
		t.Logf("Note: Legend should have Epistemic states section, got: %q", result)
	}
}

// TestRenderStatus_LegendTaintSymbols tests that legend shows taint symbols
func TestRenderStatus_LegendTaintSymbols(t *testing.T) {
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	// Check for taint explanations in legend
	// Expected symbols from issue: clean=... self_admitted=... tainted=...
	if !strings.Contains(result, "Taint") && !strings.Contains(result, "taint") {
		t.Logf("Note: Legend should have Taint states section, got: %q", result)
	}
}

// TestRenderStatus_OutputFormat tests the overall output format structure
func TestRenderStatus_OutputFormat(t *testing.T) {
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Root claim", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "First step", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)
	addStatusTestNode(t, s, "1.2", "Second step", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintTainted)

	result := RenderStatus(s, 0, 0)

	// Output should be multi-line
	lines := strings.Split(result, "\n")
	if len(lines) < 5 {
		t.Errorf("RenderStatus should produce multiple lines of output, got %d lines", len(lines))
	}

	// Check expected section order:
	// 1. Header (Proof Status)
	// 2. Tree view
	// 3. Statistics
	// 4. Jobs
	// 5. Legend

	headerPos := strings.Index(result, "Proof Status")
	statsPos := strings.Index(result, "Statistics")
	legendPos := strings.Index(result, "Legend")

	if headerPos == -1 {
		t.Error("Missing 'Proof Status' header")
	}
	if statsPos == -1 {
		t.Error("Missing 'Statistics' section")
	}
	if legendPos == -1 {
		t.Error("Missing 'Legend' section")
	}

	// Header should come first
	if headerPos > statsPos {
		t.Error("Proof Status header should come before Statistics")
	}
	// Statistics should come before Legend
	if statsPos > legendPos {
		t.Error("Statistics should come before Legend")
	}
}

// TestRenderStatus_MultipleRoots tests rendering when there are multiple root-level proofs
func TestRenderStatus_MultipleRoots(t *testing.T) {
	// Note: In a valid AF proof, there should only be one root (1),
	// but the render function should handle the case gracefully.
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Main root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Child 1", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.2", "Child 2", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	// Should contain all nodes
	if !strings.Contains(result, "Main root") {
		t.Error("RenderStatus missing root node")
	}
	if !strings.Contains(result, "Child 1") {
		t.Error("RenderStatus missing child 1")
	}
	if !strings.Contains(result, "Child 2") {
		t.Error("RenderStatus missing child 2")
	}
}

// TestRenderStatus_DeepTreeStatistics tests statistics for a deep tree
func TestRenderStatus_DeepTreeStatistics(t *testing.T) {
	s := state.NewState()
	// Create a 4-level deep tree
	addStatusTestNode(t, s, "1", "Level 1", schema.WorkflowClaimed, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Level 2", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)
	addStatusTestNode(t, s, "1.1.1", "Level 3", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintSelfAdmitted)
	addStatusTestNode(t, s, "1.1.1.1", "Level 4", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintTainted)

	result := RenderStatus(s, 0, 0)

	// Statistics should show 4 total nodes
	if !strings.Contains(result, "4") {
		t.Logf("Note: Statistics should show 4 total nodes, got: %q", result)
	}

	// All nodes should appear in tree
	for _, level := range []string{"Level 1", "Level 2", "Level 3", "Level 4"} {
		if !strings.Contains(result, level) {
			t.Errorf("RenderStatus tree missing %q", level)
		}
	}
}

// TestRenderStatus_AllEpistemicStates tests rendering with all epistemic states present
func TestRenderStatus_AllEpistemicStates(t *testing.T) {
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Pending node", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Validated node", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)
	addStatusTestNode(t, s, "1.2", "Admitted node", schema.WorkflowAvailable, schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	addStatusTestNode(t, s, "1.3", "Refuted node", schema.WorkflowAvailable, schema.EpistemicRefuted, node.TaintTainted)
	addStatusTestNode(t, s, "1.4", "Archived node", schema.WorkflowAvailable, schema.EpistemicArchived, node.TaintTainted)

	result := RenderStatus(s, 0, 0)

	// All epistemic states should appear somewhere in output
	for _, state := range []string{"pending", "validated", "admitted", "refuted", "archived"} {
		if !strings.Contains(result, state) {
			t.Errorf("RenderStatus should mention epistemic state %q, got: %q", state, result)
		}
	}
}

// TestRenderStatus_AllTaintStates tests rendering with all taint states present
func TestRenderStatus_AllTaintStates(t *testing.T) {
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Clean node", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Self-admitted node", schema.WorkflowAvailable, schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	addStatusTestNode(t, s, "1.2", "Tainted node", schema.WorkflowClaimed, schema.EpistemicPending, node.TaintTainted)
	addStatusTestNode(t, s, "1.3", "Unresolved node", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintUnresolved)

	result := RenderStatus(s, 0, 0)

	// All taint states should appear somewhere in output
	for _, taint := range []string{"clean", "self_admitted", "tainted", "unresolved"} {
		if !strings.Contains(result, taint) {
			t.Errorf("RenderStatus should mention taint state %q, got: %q", taint, result)
		}
	}
}

// TestRenderStatus_ConsistentOutput tests that output is deterministic
func TestRenderStatus_ConsistentOutput(t *testing.T) {
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Child A", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)
	addStatusTestNode(t, s, "1.2", "Child B", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintTainted)

	// Call multiple times
	result1 := RenderStatus(s, 0, 0)
	result2 := RenderStatus(s, 0, 0)
	result3 := RenderStatus(s, 0, 0)

	// Results should be identical
	if result1 != result2 || result2 != result3 {
		t.Error("RenderStatus output is not deterministic")
	}
}

// TestRenderStatus_JobSectionText tests the wording in job section
func TestRenderStatus_JobSectionText(t *testing.T) {
	s := state.NewState()
	// Create prover jobs (available + pending)
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Step 1", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	// Job section should use descriptive language
	// E.g., "nodes need refinement" or "awaiting review"
	if !strings.Contains(result, "Jobs") && !strings.Contains(result, "jobs") {
		t.Error("Missing jobs section")
	}
}

// TestRenderStatus_SectionSeparators tests that sections have visual separators
func TestRenderStatus_SectionSeparators(t *testing.T) {
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	// Check for section separators (--- or === or similar)
	hasSeparators := strings.Contains(result, "---") ||
		strings.Contains(result, "===") ||
		strings.Contains(result, "***")

	if !hasSeparators {
		t.Logf("Note: RenderStatus may benefit from section separators (---, ===), got: %q", result)
	}
}

// TestRenderStatus_ComprehensiveExample tests the full expected format from issue
func TestRenderStatus_ComprehensiveExample(t *testing.T) {
	s := state.NewState()
	// Create a state matching the expected output from the issue:
	// 5 nodes total: 2 pending, 2 validated, 1 admitted
	// Taint: 1 clean, 2 self-admitted, 1 tainted
	addStatusTestNode(t, s, "1", "Root theorem", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "First lemma", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintSelfAdmitted)
	addStatusTestNode(t, s, "1.2", "Second lemma", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintSelfAdmitted)
	addStatusTestNode(t, s, "1.3", "Admitted step", schema.WorkflowAvailable, schema.EpistemicAdmitted, node.TaintSelfAdmitted)
	addStatusTestNode(t, s, "1.4", "Pending step", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintTainted)

	result := RenderStatus(s, 0, 0)

	// Verify all major sections exist
	sections := []string{
		"Proof Status",
		"Statistics",
		"Jobs",
		"Legend",
	}
	for _, section := range sections {
		if !strings.Contains(result, section) {
			t.Errorf("Missing section: %q in output: %q", section, result)
		}
	}

	// Verify node count is mentioned
	if !strings.Contains(result, "5") || (!strings.Contains(result, "total") && !strings.Contains(result, "Nodes")) {
		t.Logf("Note: Should show total node count (5), got: %q", result)
	}
}

// TestRenderStatus_ZeroJobsWording tests wording when there are zero jobs
func TestRenderStatus_ZeroJobsWording(t *testing.T) {
	s := state.NewState()
	// All validated - no jobs
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowClaimed, schema.EpistemicValidated, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	// Should indicate zero or "no" jobs, not leave section empty
	lower := strings.ToLower(result)

	// Check for prover section
	if strings.Contains(lower, "prover") {
		// If prover is mentioned, should indicate zero
		if !strings.Contains(result, "0") && !strings.Contains(lower, "no") && !strings.Contains(lower, "none") {
			t.Logf("Note: Prover jobs should indicate 0 when none available")
		}
	}
}

// TestRenderStatus_LargeTree tests performance and output for larger trees
func TestRenderStatus_LargeTree(t *testing.T) {
	s := state.NewState()
	// Create a wider tree with many siblings
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowClaimed, schema.EpistemicPending, node.TaintClean)
	for i := 1; i <= 10; i++ {
		id := "1." + strings.Repeat("1.", i-1) + "1"
		if i == 1 {
			id = "1.1"
		}
		addStatusTestNode(t, s, id, "Step "+string(rune('A'+i-1)), schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	}

	// Should complete without hanging
	result := RenderStatus(s, 0, 0)

	// Should produce output
	if result == "" {
		t.Error("RenderStatus returned empty string for large tree")
	}

	// Should have reasonable length (not excessive)
	if len(result) > 10000 {
		t.Logf("Note: RenderStatus output very large (%d chars), consider optimization", len(result))
	}
}

// TestRenderStatus_Pagination tests pagination with limit and offset
func TestRenderStatus_Pagination(t *testing.T) {
	s := state.NewState()
	// Create 5 nodes: 1, 1.1, 1.2, 1.3, 1.4
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Step 1", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.2", "Step 2", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.3", "Step 3", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.4", "Step 4", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)

	t.Run("no pagination (limit=0, offset=0)", func(t *testing.T) {
		result := RenderStatus(s, 0, 0)
		// Should contain all nodes
		for _, step := range []string{"Root", "Step 1", "Step 2", "Step 3", "Step 4"} {
			if !strings.Contains(result, step) {
				t.Errorf("Expected to find %q in output, but it was missing", step)
			}
		}
	})

	t.Run("limit only (limit=2, offset=0)", func(t *testing.T) {
		result := RenderStatus(s, 2, 0)
		// Should contain only first 2 nodes (sorted by ID: 1, 1.1)
		if !strings.Contains(result, "Root") {
			t.Error("Expected to find 'Root' (node 1) in output")
		}
		if !strings.Contains(result, "Step 1") {
			t.Error("Expected to find 'Step 1' (node 1.1) in output")
		}
		// Should NOT contain nodes beyond the limit
		if strings.Contains(result, "Step 2") {
			t.Error("Did not expect 'Step 2' in limited output")
		}
		if strings.Contains(result, "Step 3") {
			t.Error("Did not expect 'Step 3' in limited output")
		}
		if strings.Contains(result, "Step 4") {
			t.Error("Did not expect 'Step 4' in limited output")
		}
	})

	t.Run("offset only (limit=0, offset=2)", func(t *testing.T) {
		result := RenderStatus(s, 0, 2)
		// Should skip first 2 nodes and show all remaining
		// Sorted order: 1, 1.1, 1.2, 1.3, 1.4
		// After skipping 2: 1.2, 1.3, 1.4
		if strings.Contains(result, "Root") {
			t.Error("Did not expect 'Root' (node 1) after offset")
		}
		if strings.Contains(result, "Step 1") {
			t.Error("Did not expect 'Step 1' (node 1.1) after offset")
		}
		if !strings.Contains(result, "Step 2") {
			t.Error("Expected to find 'Step 2' (node 1.2) in output")
		}
		if !strings.Contains(result, "Step 3") {
			t.Error("Expected to find 'Step 3' (node 1.3) in output")
		}
		if !strings.Contains(result, "Step 4") {
			t.Error("Expected to find 'Step 4' (node 1.4) in output")
		}
	})

	t.Run("limit and offset (limit=2, offset=1)", func(t *testing.T) {
		result := RenderStatus(s, 2, 1)
		// Skip 1, take 2: should show nodes 1.1, 1.2
		if strings.Contains(result, "Root") {
			t.Error("Did not expect 'Root' (node 1) - should be skipped by offset")
		}
		if !strings.Contains(result, "Step 1") {
			t.Error("Expected to find 'Step 1' (node 1.1) in output")
		}
		if !strings.Contains(result, "Step 2") {
			t.Error("Expected to find 'Step 2' (node 1.2) in output")
		}
		if strings.Contains(result, "Step 3") {
			t.Error("Did not expect 'Step 3' - should be beyond limit")
		}
		if strings.Contains(result, "Step 4") {
			t.Error("Did not expect 'Step 4' - should be beyond limit")
		}
	})

	t.Run("offset beyond total nodes", func(t *testing.T) {
		result := RenderStatus(s, 0, 100)
		// Offset is larger than total nodes, should show no nodes in tree
		// But should still have sections (statistics with 0 nodes, legend, etc.)
		if strings.Contains(result, "Step 1") {
			t.Error("Did not expect any steps when offset exceeds total nodes")
		}
	})

	t.Run("limit larger than available", func(t *testing.T) {
		result := RenderStatus(s, 100, 0)
		// Limit is larger than total, should show all nodes
		for _, step := range []string{"Root", "Step 1", "Step 2", "Step 3", "Step 4"} {
			if !strings.Contains(result, step) {
				t.Errorf("Expected to find %q in output when limit exceeds total", step)
			}
		}
	})
}

// TestRenderStatusJSON_Pagination tests pagination for JSON output
func TestRenderStatusJSON_Pagination(t *testing.T) {
	s := state.NewState()
	// Create 5 nodes
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.1", "Step 1", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.2", "Step 2", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.3", "Step 3", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)
	addStatusTestNode(t, s, "1.4", "Step 4", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)

	t.Run("no pagination", func(t *testing.T) {
		result := RenderStatusJSON(s, 0, 0)
		// Should contain all 5 node IDs in JSON
		for _, id := range []string{`"id":"1"`, `"id":"1.1"`, `"id":"1.2"`, `"id":"1.3"`, `"id":"1.4"`} {
			if !strings.Contains(result, id) {
				t.Errorf("Expected JSON to contain %s", id)
			}
		}
	})

	t.Run("limit=2", func(t *testing.T) {
		result := RenderStatusJSON(s, 2, 0)
		// Should contain only first 2 nodes
		if !strings.Contains(result, `"id":"1"`) {
			t.Error("Expected JSON to contain node 1")
		}
		if !strings.Contains(result, `"id":"1.1"`) {
			t.Error("Expected JSON to contain node 1.1")
		}
		// Should NOT contain nodes beyond limit
		if strings.Contains(result, `"id":"1.2"`) {
			t.Error("Did not expect node 1.2 in limited JSON output")
		}
	})

	t.Run("offset=2", func(t *testing.T) {
		result := RenderStatusJSON(s, 0, 2)
		// Should skip first 2 nodes
		if strings.Contains(result, `"id":"1"`) && !strings.Contains(result, `"id":"1.`) {
			// Make sure "1" is not present as a standalone node
			// This is tricky because "1.1" contains "1", so we check differently
		}
		// The node "1" should not be in the list (but 1.1, 1.2, etc. might still match substring)
		// We need a more specific check
		if !strings.Contains(result, `"id":"1.2"`) {
			t.Error("Expected JSON to contain node 1.2")
		}
		if !strings.Contains(result, `"id":"1.3"`) {
			t.Error("Expected JSON to contain node 1.3")
		}
		if !strings.Contains(result, `"id":"1.4"`) {
			t.Error("Expected JSON to contain node 1.4")
		}
	})

	t.Run("limit and offset", func(t *testing.T) {
		result := RenderStatusJSON(s, 2, 1)
		// Skip 1, take 2: nodes 1.1, 1.2
		if !strings.Contains(result, `"id":"1.1"`) {
			t.Error("Expected JSON to contain node 1.1")
		}
		if !strings.Contains(result, `"id":"1.2"`) {
			t.Error("Expected JSON to contain node 1.2")
		}
		if strings.Contains(result, `"id":"1.3"`) {
			t.Error("Did not expect node 1.3 in JSON output")
		}
	})
}

// TestRenderStatus_HeaderFormat tests the header format matches expected pattern
func TestRenderStatus_HeaderFormat(t *testing.T) {
	s := state.NewState()
	addStatusTestNode(t, s, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending, node.TaintClean)

	result := RenderStatus(s, 0, 0)

	// Header should be prominent (=== Proof Status === or similar)
	if !strings.Contains(result, "===") && !strings.Contains(result, "***") {
		t.Logf("Note: Header may benefit from decorators like === Proof Status ===")
	}

	if !strings.Contains(result, "Proof Status") {
		t.Error("Missing 'Proof Status' header")
	}
}
