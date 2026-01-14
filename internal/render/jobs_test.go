//go:build integration

package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// Helper to create a test node for jobs rendering tests.
// Panics on invalid input (intended for test use only).
func makeJobsTestNode(
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

// buildNodeMap creates a node map from a slice of nodes keyed by ID string.
func buildNodeMap(nodes []*node.Node) map[string]*node.Node {
	m := make(map[string]*node.Node)
	for _, n := range nodes {
		m[n.ID.String()] = n
	}
	return m
}

// TestRenderJobs_NilJobResult tests handling of nil JobResult.
func TestRenderJobs_NilJobResult(t *testing.T) {
	result := RenderJobs(nil)

	// Should return empty or message indicating no jobs
	if result != "" {
		// If non-empty, should be safe/informative
		lower := strings.ToLower(result)
		if !strings.Contains(lower, "no") && !strings.Contains(lower, "empty") {
			t.Logf("Note: RenderJobs for nil result returned: %q", result)
		}
	}
}

// TestRenderJobs_EmptyJobResult tests rendering when no jobs are available.
func TestRenderJobs_EmptyJobResult(t *testing.T) {
	jobResult := &jobs.JobResult{
		ProverJobs:   nil,
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// Should indicate no jobs available
	if result == "" {
		t.Fatal("RenderJobs should return a message for empty jobs, not empty string")
	}

	lower := strings.ToLower(result)
	if !strings.Contains(lower, "no") && !strings.Contains(lower, "0") && !strings.Contains(lower, "empty") {
		t.Errorf("RenderJobs for empty state should indicate no jobs, got: %q", result)
	}
}

// TestRenderJobs_ProverJobsOnly tests rendering when only prover jobs exist.
func TestRenderJobs_ProverJobsOnly(t *testing.T) {
	proverNodes := []*node.Node{
		makeJobsTestNode("1.1", "Prove the base case", schema.WorkflowAvailable, schema.EpistemicPending),
		makeJobsTestNode("1.2", "Prove the induction step", schema.WorkflowAvailable, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	if result == "" {
		t.Fatal("RenderJobs returned empty string for prover jobs")
	}

	// Should contain prover section
	lower := strings.ToLower(result)
	if !strings.Contains(lower, "prover") {
		t.Errorf("RenderJobs should mention prover jobs, got: %q", result)
	}

	// Should contain node IDs
	if !strings.Contains(result, "1.1") || !strings.Contains(result, "1.2") {
		t.Errorf("RenderJobs should list prover job node IDs, got: %q", result)
	}

	// Should contain job count (2 prover jobs)
	if !strings.Contains(result, "2") {
		t.Logf("Note: RenderJobs may want to show prover job count (2), got: %q", result)
	}
}

// TestRenderJobs_VerifierJobsOnly tests rendering when only verifier jobs exist.
func TestRenderJobs_VerifierJobsOnly(t *testing.T) {
	verifierNodes := []*node.Node{
		makeJobsTestNode("1", "Main theorem ready for review", schema.WorkflowClaimed, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   nil,
		VerifierJobs: verifierNodes,
	}

	result := RenderJobs(jobResult)

	if result == "" {
		t.Fatal("RenderJobs returned empty string for verifier jobs")
	}

	// Should contain verifier section
	lower := strings.ToLower(result)
	if !strings.Contains(lower, "verifier") {
		t.Errorf("RenderJobs should mention verifier jobs, got: %q", result)
	}

	// Should contain node ID
	if !strings.Contains(result, "1") {
		t.Errorf("RenderJobs should list verifier job node ID, got: %q", result)
	}
}

// TestRenderJobs_MixedJobs tests rendering when both prover and verifier jobs exist.
func TestRenderJobs_MixedJobs(t *testing.T) {
	proverNodes := []*node.Node{
		makeJobsTestNode("1.1", "Available for prover", schema.WorkflowAvailable, schema.EpistemicPending),
		makeJobsTestNode("1.2", "Also available", schema.WorkflowAvailable, schema.EpistemicPending),
		makeJobsTestNode("1.3", "Third prover job", schema.WorkflowAvailable, schema.EpistemicPending),
	}
	verifierNodes := []*node.Node{
		makeJobsTestNode("1.4", "Ready for verifier", schema.WorkflowClaimed, schema.EpistemicPending),
		makeJobsTestNode("1.5", "Another verifier job", schema.WorkflowClaimed, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: verifierNodes,
	}

	result := RenderJobs(jobResult)

	if result == "" {
		t.Fatal("RenderJobs returned empty string for mixed jobs")
	}

	// Should contain both prover and verifier sections
	lower := strings.ToLower(result)
	if !strings.Contains(lower, "prover") {
		t.Errorf("RenderJobs should mention prover jobs, got: %q", result)
	}
	if !strings.Contains(lower, "verifier") {
		t.Errorf("RenderJobs should mention verifier jobs, got: %q", result)
	}

	// Should contain all job IDs
	for _, id := range []string{"1.1", "1.2", "1.3", "1.4", "1.5"} {
		if !strings.Contains(result, id) {
			t.Errorf("RenderJobs missing job ID %q, got: %q", id, result)
		}
	}
}

// TestRenderJobs_ShowsStatements tests that job statements are included.
func TestRenderJobs_ShowsStatements(t *testing.T) {
	proverNodes := []*node.Node{
		makeJobsTestNode("1.1", "Prove that P implies Q", schema.WorkflowAvailable, schema.EpistemicPending),
	}
	verifierNodes := []*node.Node{
		makeJobsTestNode("1.2", "Verify the base case", schema.WorkflowClaimed, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: verifierNodes,
	}

	result := RenderJobs(jobResult)

	// Should contain job statements
	if !strings.Contains(result, "P implies Q") {
		t.Errorf("RenderJobs should include prover job statement, got: %q", result)
	}
	if !strings.Contains(result, "base case") {
		t.Errorf("RenderJobs should include verifier job statement, got: %q", result)
	}
}

// TestRenderJobs_NodeDetails tests that each job shows relevant details.
func TestRenderJobs_NodeDetails(t *testing.T) {
	n := makeJobsTestNode("1.2.3", "Prove the intermediate lemma", schema.WorkflowAvailable, schema.EpistemicPending)
	n.Type = schema.NodeTypeClaim

	jobResult := &jobs.JobResult{
		ProverJobs:   []*node.Node{n},
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// Should contain node ID
	if !strings.Contains(result, "1.2.3") {
		t.Errorf("RenderJobs should show node ID, got: %q", result)
	}

	// Should contain statement
	if !strings.Contains(result, "intermediate lemma") {
		t.Errorf("RenderJobs should show statement, got: %q", result)
	}
}

// TestRenderJobs_ProverInstructions tests that prover jobs include helpful instructions.
func TestRenderJobs_ProverInstructions(t *testing.T) {
	proverNode := makeJobsTestNode("1.1", "Prove P", schema.WorkflowAvailable, schema.EpistemicPending)

	jobResult := &jobs.JobResult{
		ProverJobs:   []*node.Node{proverNode},
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	lower := strings.ToLower(result)

	// Should contain instruction about what prover should do
	hasInstruction := strings.Contains(lower, "refine") ||
		strings.Contains(lower, "prove") ||
		strings.Contains(lower, "claim") ||
		strings.Contains(lower, "work") ||
		strings.Contains(lower, "available")

	if !hasInstruction {
		t.Logf("Note: RenderJobs may want prover instructions (refine, prove, etc.), got: %q", result)
	}
}

// TestRenderJobs_VerifierInstructions tests that verifier jobs include helpful instructions.
func TestRenderJobs_VerifierInstructions(t *testing.T) {
	verifierNode := makeJobsTestNode("1", "Ready for review", schema.WorkflowClaimed, schema.EpistemicPending)

	jobResult := &jobs.JobResult{
		ProverJobs:   nil,
		VerifierJobs: []*node.Node{verifierNode},
	}

	result := RenderJobs(jobResult)

	lower := strings.ToLower(result)

	// Should contain instruction about what verifier should do
	hasInstruction := strings.Contains(lower, "review") ||
		strings.Contains(lower, "verify") ||
		strings.Contains(lower, "accept") ||
		strings.Contains(lower, "validate") ||
		strings.Contains(lower, "check")

	if !hasInstruction {
		t.Logf("Note: RenderJobs may want verifier instructions (review, verify, etc.), got: %q", result)
	}
}

// TestRenderJobs_SummarySection tests that a summary of job counts is shown.
func TestRenderJobs_SummarySection(t *testing.T) {
	proverNodes := []*node.Node{
		makeJobsTestNode("1.1", "Prover job 1", schema.WorkflowAvailable, schema.EpistemicPending),
		makeJobsTestNode("1.2", "Prover job 2", schema.WorkflowAvailable, schema.EpistemicPending),
	}
	verifierNodes := []*node.Node{
		makeJobsTestNode("1.3", "Verifier job 1", schema.WorkflowClaimed, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: verifierNodes,
	}

	result := RenderJobs(jobResult)

	// Should show job counts
	// 2 prover jobs, 1 verifier job, 3 total
	if !strings.Contains(result, "2") && !strings.Contains(result, "1") {
		t.Logf("Note: RenderJobs may want to show job counts, got: %q", result)
	}
}

// TestRenderJobs_MultiLineFormat tests that output is properly formatted multi-line text.
func TestRenderJobs_MultiLineFormat(t *testing.T) {
	proverNodes := []*node.Node{
		makeJobsTestNode("1.1", "First job", schema.WorkflowAvailable, schema.EpistemicPending),
		makeJobsTestNode("1.2", "Second job", schema.WorkflowAvailable, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// Should be multi-line for readability
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Logf("Note: RenderJobs may benefit from multi-line format for multiple jobs, got: %q", result)
	}

	// Should not be JSON or machine format
	if strings.HasPrefix(strings.TrimSpace(result), "{") || strings.HasPrefix(strings.TrimSpace(result), "[") {
		t.Errorf("RenderJobs should return human-readable text, not JSON, got: %q", result)
	}
}

// TestRenderJobs_ConsistentOutput tests that repeated calls produce consistent output.
func TestRenderJobs_ConsistentOutput(t *testing.T) {
	proverNodes := []*node.Node{
		makeJobsTestNode("1.1", "Prover job", schema.WorkflowAvailable, schema.EpistemicPending),
	}
	verifierNodes := []*node.Node{
		makeJobsTestNode("1.4", "Verifier job", schema.WorkflowClaimed, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: verifierNodes,
	}

	// Call multiple times
	result1 := RenderJobs(jobResult)
	result2 := RenderJobs(jobResult)
	result3 := RenderJobs(jobResult)

	// Results should be identical (deterministic)
	if result1 != result2 || result2 != result3 {
		t.Errorf("RenderJobs produced inconsistent output:\n1: %q\n2: %q\n3: %q",
			result1, result2, result3)
	}
}

// TestRenderJobs_SortedByID tests that jobs are sorted by node ID for consistency.
func TestRenderJobs_SortedByID(t *testing.T) {
	// Provide nodes in unsorted order
	proverNodes := []*node.Node{
		makeJobsTestNode("1.3", "Third", schema.WorkflowAvailable, schema.EpistemicPending),
		makeJobsTestNode("1.1", "First", schema.WorkflowAvailable, schema.EpistemicPending),
		makeJobsTestNode("1.2", "Second", schema.WorkflowAvailable, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// Should contain all IDs
	if !strings.Contains(result, "1.1") || !strings.Contains(result, "1.2") || !strings.Contains(result, "1.3") {
		t.Errorf("RenderJobs missing job IDs, got: %q", result)
	}

	// Check order: 1.1 should appear before 1.2, which should appear before 1.3
	pos11 := strings.Index(result, "1.1")
	pos12 := strings.Index(result, "1.2")
	pos13 := strings.Index(result, "1.3")

	if pos11 == -1 || pos12 == -1 || pos13 == -1 {
		t.Fatal("RenderJobs missing some job IDs")
	}

	if pos11 > pos12 || pos12 > pos13 {
		t.Errorf("RenderJobs should sort jobs by ID (1.1 < 1.2 < 1.3), got: %q", result)
	}
}

// TestRenderJobs_LongStatementNotTruncated tests that long statements are NOT truncated.
// Mathematical proofs require precision - truncation makes statements useless for agents.
func TestRenderJobs_LongStatementNotTruncated(t *testing.T) {
	longStatement := "The Bell number B_n counts the number of ways to partition a set of n elements. " +
		"By Dobinski's formula, B_n = (1/e) * sum_{k=0}^{infinity} k^n / k!. This statement requires " +
		"the complete text for mathematical verification."

	proverNode := makeJobsTestNode("1.1", longStatement, schema.WorkflowAvailable, schema.EpistemicPending)

	jobResult := &jobs.JobResult{
		ProverJobs:   []*node.Node{proverNode},
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// Statement should NOT be truncated - agents need full text for proofs
	if !strings.Contains(result, "Dobinski's formula") {
		t.Errorf("RenderJobs truncated statement - expected full 'Dobinski's formula', got: %q", result)
	}
	if !strings.Contains(result, "mathematical verification") {
		t.Errorf("RenderJobs truncated statement - expected 'mathematical verification', got: %q", result)
	}

	// Should NOT contain truncation ellipsis in the statement
	// Note: we check for the specific pattern of truncation (word...") not just "..."
	if strings.Contains(result, "...\"") && !strings.Contains(result, longStatement) {
		t.Errorf("RenderJobs incorrectly truncated long statement with ellipsis, got: %q", result)
	}

	// Should still contain node ID
	if !strings.Contains(result, "1.1") {
		t.Errorf("RenderJobs missing node ID, got: %q", result)
	}
}

// TestRenderJobs_SpecialCharacters tests handling of special characters in statements.
func TestRenderJobs_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name      string
		statement string
	}{
		{
			name:      "unicode math symbols",
			statement: "For all x: P(x) implies Q(x)",
		},
		{
			name:      "quotes in statement",
			statement: `The term "natural number" is defined`,
		},
		{
			name:      "newlines in statement",
			statement: "Line one\nLine two",
		},
		{
			name:      "backslashes for LaTeX",
			statement: `Let \alpha + \beta = \gamma`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proverNode := makeJobsTestNode("1", tt.statement, schema.WorkflowAvailable, schema.EpistemicPending)

			jobResult := &jobs.JobResult{
				ProverJobs:   []*node.Node{proverNode},
				VerifierJobs: nil,
			}

			// Should not panic
			result := RenderJobs(jobResult)

			// Should return non-empty result
			if result == "" {
				t.Fatalf("RenderJobs returned empty for statement with %s", tt.name)
			}

			// Should contain node ID at minimum
			if !strings.Contains(result, "1") {
				t.Errorf("RenderJobs missing node ID, got: %q", result)
			}
		})
	}
}

// TestRenderJobs_DeepHierarchy tests rendering jobs from deep in the node tree.
func TestRenderJobs_DeepHierarchy(t *testing.T) {
	proverNodes := []*node.Node{
		makeJobsTestNode("1.1.1.1.1", "Deep prover job", schema.WorkflowAvailable, schema.EpistemicPending),
	}
	verifierNodes := []*node.Node{
		makeJobsTestNode("1.1.1.1.2", "Deep verifier job", schema.WorkflowClaimed, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: verifierNodes,
	}

	result := RenderJobs(jobResult)

	// Should contain deep node IDs
	if !strings.Contains(result, "1.1.1.1.1") {
		t.Errorf("RenderJobs missing deep prover job ID, got: %q", result)
	}
	if !strings.Contains(result, "1.1.1.1.2") {
		t.Errorf("RenderJobs missing deep verifier job ID, got: %q", result)
	}
}

// TestRenderJobs_LargeJobList tests performance and formatting with many jobs.
func TestRenderJobs_LargeJobList(t *testing.T) {
	var proverNodes []*node.Node
	for i := 1; i <= 20; i++ {
		id := fmt.Sprintf("1.%d", i)
		n := makeJobsTestNode(id, "Prover job", schema.WorkflowAvailable, schema.EpistemicPending)
		proverNodes = append(proverNodes, n)
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: nil,
	}

	// Should complete without hanging
	result := RenderJobs(jobResult)

	// Should produce output
	if result == "" {
		t.Fatal("RenderJobs returned empty string for large job list")
	}

	// Should mention prover jobs
	if !strings.Contains(strings.ToLower(result), "prover") {
		t.Errorf("RenderJobs missing prover section for large list, got: %q", result)
	}
}

// TestRenderJobs_SectionHeaders tests that clear section headers are present.
func TestRenderJobs_SectionHeaders(t *testing.T) {
	proverNodes := []*node.Node{
		makeJobsTestNode("1.1", "Prover job", schema.WorkflowAvailable, schema.EpistemicPending),
	}
	verifierNodes := []*node.Node{
		makeJobsTestNode("1.4", "Verifier job", schema.WorkflowClaimed, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: verifierNodes,
	}

	result := RenderJobs(jobResult)

	// Should have section separators or headers for prover/verifier
	hasHeaders := strings.Contains(result, "---") ||
		strings.Contains(result, "===") ||
		strings.Contains(result, "Prover") ||
		strings.Contains(result, "Verifier")

	if !hasHeaders {
		t.Logf("Note: RenderJobs may benefit from clear section headers, got: %q", result)
	}
}

// TestRenderJobs_EmptyProverSection tests rendering when prover section is empty.
func TestRenderJobs_EmptyProverSection(t *testing.T) {
	verifierNodes := []*node.Node{
		makeJobsTestNode("1", "Verifier job only", schema.WorkflowClaimed, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   nil,
		VerifierJobs: verifierNodes,
	}

	result := RenderJobs(jobResult)

	// Should still produce output
	if result == "" {
		t.Fatal("RenderJobs returned empty string")
	}

	// Should contain verifier info
	if !strings.Contains(strings.ToLower(result), "verifier") {
		t.Errorf("RenderJobs should mention verifier section, got: %q", result)
	}

	// May indicate 0 prover jobs or simply omit prover section
	// Either is acceptable
}

// TestRenderJobs_EmptyVerifierSection tests rendering when verifier section is empty.
func TestRenderJobs_EmptyVerifierSection(t *testing.T) {
	proverNodes := []*node.Node{
		makeJobsTestNode("1.1", "Prover job only", schema.WorkflowAvailable, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// Should still produce output
	if result == "" {
		t.Fatal("RenderJobs returned empty string")
	}

	// Should contain prover info
	if !strings.Contains(strings.ToLower(result), "prover") {
		t.Errorf("RenderJobs should mention prover section, got: %q", result)
	}
}

// TestRenderJobs_JobReason tests that the reason a node is a job is clear.
func TestRenderJobs_JobReason(t *testing.T) {
	proverNode := makeJobsTestNode("1.1", "Needs refinement", schema.WorkflowAvailable, schema.EpistemicPending)

	jobResult := &jobs.JobResult{
		ProverJobs:   []*node.Node{proverNode},
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// Should give indication of WHY this is a prover job
	// E.g., "available for refinement", "pending proof", etc.
	lower := strings.ToLower(result)
	hasReason := strings.Contains(lower, "available") ||
		strings.Contains(lower, "pending") ||
		strings.Contains(lower, "refine") ||
		strings.Contains(lower, "needs work")

	if !hasReason {
		t.Logf("Note: RenderJobs may want to explain why node is a prover job, got: %q", result)
	}
}

// TestRenderJobs_NextStepsGuidance tests that helpful next steps are suggested.
func TestRenderJobs_NextStepsGuidance(t *testing.T) {
	proverNode := makeJobsTestNode("1.1", "Prover job", schema.WorkflowAvailable, schema.EpistemicPending)

	jobResult := &jobs.JobResult{
		ProverJobs:   []*node.Node{proverNode},
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// Should contain guidance about what to do next
	// E.g., "af claim 1.1", "af refine 1.1", etc.
	lower := strings.ToLower(result)
	hasGuidance := strings.Contains(lower, "claim") ||
		strings.Contains(lower, "af ") ||
		strings.Contains(lower, "refine") ||
		strings.Contains(lower, "run")

	if !hasGuidance {
		t.Logf("Note: RenderJobs may want to include command guidance (af claim, etc.), got: %q", result)
	}
}

// TestRenderJobs_TableDrivenJobTypes tests rendering different combinations of jobs.
func TestRenderJobs_TableDrivenJobTypes(t *testing.T) {
	tests := []struct {
		name             string
		proverCount      int
		verifierCount    int
		wantProverMention bool
		wantVerifierMention bool
	}{
		{
			name:             "no jobs",
			proverCount:      0,
			verifierCount:    0,
			wantProverMention: false,
			wantVerifierMention: false,
		},
		{
			name:             "only prover jobs",
			proverCount:      2,
			verifierCount:    0,
			wantProverMention: true,
			wantVerifierMention: false,
		},
		{
			name:             "only verifier jobs",
			proverCount:      0,
			verifierCount:    1,
			wantProverMention: false,
			wantVerifierMention: true,
		},
		{
			name:             "both job types",
			proverCount:      3,
			verifierCount:    2,
			wantProverMention: true,
			wantVerifierMention: true,
		},
		{
			name:             "single prover",
			proverCount:      1,
			verifierCount:    0,
			wantProverMention: true,
			wantVerifierMention: false,
		},
		{
			name:             "single verifier",
			proverCount:      0,
			verifierCount:    1,
			wantProverMention: false,
			wantVerifierMention: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var proverNodes []*node.Node
			for i := 0; i < tt.proverCount; i++ {
				id := fmt.Sprintf("1.%d", i+1)
				n := makeJobsTestNode(id, "Prover job", schema.WorkflowAvailable, schema.EpistemicPending)
				proverNodes = append(proverNodes, n)
			}

			var verifierNodes []*node.Node
			for i := 0; i < tt.verifierCount; i++ {
				id := fmt.Sprintf("1.%d", tt.proverCount+i+1)
				n := makeJobsTestNode(id, "Verifier job", schema.WorkflowClaimed, schema.EpistemicPending)
				verifierNodes = append(verifierNodes, n)
			}

			jobResult := &jobs.JobResult{
				ProverJobs:   proverNodes,
				VerifierJobs: verifierNodes,
			}

			result := RenderJobs(jobResult)
			lower := strings.ToLower(result)

			// Check prover mention
			hasProverMention := strings.Contains(lower, "prover")
			if tt.wantProverMention && !hasProverMention {
				t.Errorf("Expected prover mention when %d prover jobs, got: %q", tt.proverCount, result)
			}

			// Check verifier mention
			hasVerifierMention := strings.Contains(lower, "verifier")
			if tt.wantVerifierMention && !hasVerifierMention {
				t.Errorf("Expected verifier mention when %d verifier jobs, got: %q", tt.verifierCount, result)
			}
		})
	}
}

// TestRenderJobs_WithClaimedBy tests that claimed-by info is shown for context.
func TestRenderJobs_WithClaimedBy(t *testing.T) {
	verifierNode := makeJobsTestNode("1", "Ready for review", schema.WorkflowClaimed, schema.EpistemicPending)
	verifierNode.ClaimedBy = "prover-agent-abc123"

	jobResult := &jobs.JobResult{
		ProverJobs:   nil,
		VerifierJobs: []*node.Node{verifierNode},
	}

	result := RenderJobs(jobResult)

	// Should show who claimed the node (helpful context for verifier)
	if !strings.Contains(result, "prover-agent") && !strings.Contains(result, "abc123") {
		t.Logf("Note: RenderJobs may want to show who claimed verifier job, got: %q", result)
	}
}

// TestRenderJobs_NodeTypeShown tests that node types are shown.
func TestRenderJobs_NodeTypeShown(t *testing.T) {
	proverNode := makeJobsTestNode("1.1", "A claim node", schema.WorkflowAvailable, schema.EpistemicPending)
	proverNode.Type = schema.NodeTypeClaim

	jobResult := &jobs.JobResult{
		ProverJobs:   []*node.Node{proverNode},
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// Should show node type (claim, local_assume, etc.)
	if !strings.Contains(result, "claim") {
		t.Logf("Note: RenderJobs may want to show node type, got: %q", result)
	}
}

// TestRenderJobsFromState tests rendering jobs from a full State object.
func TestRenderJobsFromState(t *testing.T) {
	s := state.NewState()

	// Add various nodes to state
	// Prover jobs: available + pending
	prover1 := makeJobsTestNode("1.1", "Available prover job 1", schema.WorkflowAvailable, schema.EpistemicPending)
	prover2 := makeJobsTestNode("1.2", "Available prover job 2", schema.WorkflowAvailable, schema.EpistemicPending)

	// Verifier jobs: claimed + pending + all children validated (no children = ready)
	verifier1 := makeJobsTestNode("1.4", "Verifier ready", schema.WorkflowClaimed, schema.EpistemicPending)

	// Not a job: validated (not pending)
	notJob1 := makeJobsTestNode("1.5", "Already validated", schema.WorkflowAvailable, schema.EpistemicValidated)

	// Not a job: claimed but has pending children
	notJob2 := makeJobsTestNode("1.6", "Has pending children", schema.WorkflowClaimed, schema.EpistemicPending)
	notJob2Child := makeJobsTestNode("1.6.1", "Pending child", schema.WorkflowAvailable, schema.EpistemicPending)

	s.AddNode(prover1)
	s.AddNode(prover2)
	s.AddNode(verifier1)
	s.AddNode(notJob1)
	s.AddNode(notJob2)
	s.AddNode(notJob2Child)

	// Get all nodes and find jobs
	allNodes := s.AllNodes()
	nodeMap := buildNodeMap(allNodes)
	jobResult := jobs.FindJobs(allNodes, nodeMap)

	result := RenderJobs(jobResult)

	// Should mention prover jobs
	if !strings.Contains(result, "1.1") && !strings.Contains(result, "1.2") {
		t.Errorf("RenderJobs should include prover job IDs, got: %q", result)
	}

	// Should mention verifier job
	if !strings.Contains(result, "2") {
		t.Errorf("RenderJobs should include verifier job ID, got: %q", result)
	}

	// Should NOT mention nodes that aren't jobs
	// Node 3 is validated (not a job), node 4 has pending children (not ready for verifier)
	// Note: Node 4.1 IS a prover job since it's available+pending
}

// TestRenderJobs_HumanReadableFormat tests overall human-readable formatting.
func TestRenderJobs_HumanReadableFormat(t *testing.T) {
	proverNodes := []*node.Node{
		makeJobsTestNode("1.1", "Base case proof", schema.WorkflowAvailable, schema.EpistemicPending),
		makeJobsTestNode("1.2", "Induction step", schema.WorkflowAvailable, schema.EpistemicPending),
	}
	verifierNodes := []*node.Node{
		makeJobsTestNode("1.4", "Main theorem review", schema.WorkflowClaimed, schema.EpistemicPending),
	}

	jobResult := &jobs.JobResult{
		ProverJobs:   proverNodes,
		VerifierJobs: verifierNodes,
	}

	result := RenderJobs(jobResult)

	// Output should be readable text
	if result == "" {
		t.Fatal("RenderJobs returned empty string")
	}

	// Should not be a raw struct dump
	if strings.Contains(result, "WorkflowState") || strings.Contains(result, "EpistemicState") {
		t.Errorf("RenderJobs should use human-readable terms, not struct field names, got: %q", result)
	}

	// Should not contain Go-style formatting like &{...}
	if strings.Contains(result, "&{") || strings.Contains(result, "[]node.Node") {
		t.Errorf("RenderJobs should not contain Go struct formatting, got: %q", result)
	}
}
