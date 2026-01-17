package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// ============================================================================
// Helper functions for creating test data
// ============================================================================

// makeTestNodeForJSON creates a test node with the given parameters.
func makeTestNodeForJSON(t *testing.T, id string, statement string) *node.Node {
	t.Helper()
	nodeID, err := types.Parse(id)
	if err != nil {
		t.Fatalf("invalid test node ID %q: %v", id, err)
	}
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, statement, schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create test node %q: %v", id, err)
	}
	return n
}

// makeTestNodeWithState creates a test node with specific workflow and epistemic states.
func makeTestNodeWithState(t *testing.T, id, statement string, workflow schema.WorkflowState, epistemic schema.EpistemicState) *node.Node {
	t.Helper()
	n := makeTestNodeForJSON(t, id, statement)
	n.WorkflowState = workflow
	n.EpistemicState = epistemic
	return n
}

// ============================================================================
// Tests for RenderNodeJSON
// ============================================================================

func TestRenderNodeJSON_NilNode(t *testing.T) {
	result := RenderNodeJSON(nil)
	if result != "{}" {
		t.Errorf("RenderNodeJSON(nil) = %q, want %q", result, "{}")
	}
}

func TestRenderNodeJSON_BasicNode(t *testing.T) {
	n := makeTestNodeForJSON(t, "1", "Base case holds")

	result := RenderNodeJSON(n)

	// Should be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("RenderNodeJSON produced invalid JSON: %v\nOutput: %s", err, result)
	}

	// Check required fields
	if parsed["id"] != "1" {
		t.Errorf("id = %v, want %q", parsed["id"], "1")
	}
	if parsed["statement"] != "Base case holds" {
		t.Errorf("statement = %v, want %q", parsed["statement"], "Base case holds")
	}
	if parsed["type"] != "claim" {
		t.Errorf("type = %v, want %q", parsed["type"], "claim")
	}
}

func TestRenderNodeJSON_AllRequiredFields(t *testing.T) {
	n := makeTestNodeForJSON(t, "1.2.3", "Test statement")

	result := RenderNodeJSON(n)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	requiredFields := []string{
		"id", "type", "statement", "inference",
		"workflow_state", "epistemic_state", "taint_state",
		"created", "content_hash",
	}

	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("RenderNodeJSON missing required field %q", field)
		}
	}
}

func TestRenderNodeJSON_NoHTMLEscape(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		wantIn    string
	}{
		{"greater than", "x > 5", "x > 5"},
		{"less than", "y < 10", "y < 10"},
		{"ampersand", "a && b", "a && b"},
		{"mixed", "x < 10 && y > 5", "x < 10 && y > 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := makeTestNodeForJSON(t, "1", tt.statement)
			result := RenderNodeJSON(n)

			// Should contain unescaped version
			if !strings.Contains(result, tt.wantIn) {
				t.Errorf("RenderNodeJSON should not escape HTML chars: want %q in %s", tt.wantIn, result)
			}

			// Should NOT contain Unicode-escaped forms
			escapedForms := []string{`\u003c`, `\u003e`, `\u0026`}
			for _, escaped := range escapedForms {
				if strings.Contains(result, escaped) {
					t.Errorf("RenderNodeJSON should not contain %q, got: %s", escaped, result)
				}
			}
		})
	}
}

func TestRenderNodeJSON_NestedNodeID(t *testing.T) {
	n := makeTestNodeForJSON(t, "1.2.3.4.5", "Deep node")

	result := RenderNodeJSON(n)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if parsed["id"] != "1.2.3.4.5" {
		t.Errorf("id = %v, want %q", parsed["id"], "1.2.3.4.5")
	}
}

// ============================================================================
// Tests for RenderNodeListJSON
// ============================================================================

func TestRenderNodeListJSON_NilList(t *testing.T) {
	result := RenderNodeListJSON(nil)
	if result != "[]" {
		t.Errorf("RenderNodeListJSON(nil) = %q, want %q", result, "[]")
	}
}

func TestRenderNodeListJSON_EmptyList(t *testing.T) {
	result := RenderNodeListJSON([]*node.Node{})
	if result != "[]" {
		t.Errorf("RenderNodeListJSON([]) = %q, want %q", result, "[]")
	}
}

func TestRenderNodeListJSON_SingleNode(t *testing.T) {
	nodes := []*node.Node{
		makeTestNodeForJSON(t, "1", "Single node"),
	}

	result := RenderNodeListJSON(nodes)

	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(parsed) != 1 {
		t.Errorf("Expected 1 node, got %d", len(parsed))
	}

	if parsed[0]["id"] != "1" {
		t.Errorf("First node id = %v, want %q", parsed[0]["id"], "1")
	}
}

func TestRenderNodeListJSON_MultipleNodes(t *testing.T) {
	nodes := []*node.Node{
		makeTestNodeForJSON(t, "1", "First"),
		makeTestNodeForJSON(t, "1.1", "Second"),
		makeTestNodeForJSON(t, "1.2", "Third"),
	}

	result := RenderNodeListJSON(nodes)

	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(parsed) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(parsed))
	}
}

func TestRenderNodeListJSON_SkipsNilNodes(t *testing.T) {
	nodes := []*node.Node{
		makeTestNodeForJSON(t, "1", "First"),
		nil,
		makeTestNodeForJSON(t, "1.1", "Second"),
	}

	result := RenderNodeListJSON(nodes)

	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Should skip nil nodes
	if len(parsed) != 2 {
		t.Errorf("Expected 2 nodes (skipping nil), got %d", len(parsed))
	}
}

// ============================================================================
// Tests for RenderStatusJSON
// ============================================================================

func TestRenderStatusJSON_NilState(t *testing.T) {
	result := RenderStatusJSON(nil, 0, 0)
	if !strings.Contains(result, "error") {
		t.Errorf("RenderStatusJSON(nil) should return error JSON, got: %s", result)
	}
}

func TestRenderStatusJSON_EmptyState(t *testing.T) {
	s := state.NewState()

	result := RenderStatusJSON(s, 0, 0)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	stats, ok := parsed["statistics"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing statistics field")
	}

	if stats["total_nodes"].(float64) != 0 {
		t.Errorf("total_nodes = %v, want 0", stats["total_nodes"])
	}
}

func TestRenderStatusJSON_WithNodes(t *testing.T) {
	s := state.NewState()
	s.AddNode(makeTestNodeWithState(t, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending))
	s.AddNode(makeTestNodeWithState(t, "1.1", "Child", schema.WorkflowClaimed, schema.EpistemicValidated))

	result := RenderStatusJSON(s, 0, 0)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	stats := parsed["statistics"].(map[string]interface{})
	if stats["total_nodes"].(float64) != 2 {
		t.Errorf("total_nodes = %v, want 2", stats["total_nodes"])
	}

	nodes := parsed["nodes"].([]interface{})
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}
}

func TestRenderStatusJSON_HasRequiredSections(t *testing.T) {
	s := state.NewState()
	s.AddNode(makeTestNodeForJSON(t, "1", "Root"))

	result := RenderStatusJSON(s, 0, 0)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	requiredSections := []string{"statistics", "jobs", "nodes", "challenges"}
	for _, section := range requiredSections {
		if _, ok := parsed[section]; !ok {
			t.Errorf("RenderStatusJSON missing required section %q", section)
		}
	}
}

// ============================================================================
// Tests for RenderStatusJSON Pagination
// ============================================================================

func TestRenderStatusJSON_Pagination_LimitOnly(t *testing.T) {
	s := state.NewState()
	s.AddNode(makeTestNodeForJSON(t, "1", "Root"))
	s.AddNode(makeTestNodeForJSON(t, "1.1", "Child 1"))
	s.AddNode(makeTestNodeForJSON(t, "1.2", "Child 2"))
	s.AddNode(makeTestNodeForJSON(t, "1.3", "Child 3"))

	result := RenderStatusJSON(s, 2, 0)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	nodes := parsed["nodes"].([]interface{})
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes with limit=2, got %d", len(nodes))
	}

	stats := parsed["statistics"].(map[string]interface{})
	if stats["total_nodes"].(float64) != 4 {
		t.Errorf("total_nodes = %v, want 4", stats["total_nodes"])
	}
	if stats["displayed_nodes"].(float64) != 2 {
		t.Errorf("displayed_nodes = %v, want 2", stats["displayed_nodes"])
	}
}

func TestRenderStatusJSON_Pagination_OffsetOnly(t *testing.T) {
	s := state.NewState()
	s.AddNode(makeTestNodeForJSON(t, "1", "Root"))
	s.AddNode(makeTestNodeForJSON(t, "1.1", "Child 1"))
	s.AddNode(makeTestNodeForJSON(t, "1.2", "Child 2"))
	s.AddNode(makeTestNodeForJSON(t, "1.3", "Child 3"))

	result := RenderStatusJSON(s, 0, 2)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	nodes := parsed["nodes"].([]interface{})
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes with offset=2 (4 total - 2 skipped), got %d", len(nodes))
	}
}

func TestRenderStatusJSON_Pagination_LimitAndOffset(t *testing.T) {
	s := state.NewState()
	s.AddNode(makeTestNodeForJSON(t, "1", "Root"))
	s.AddNode(makeTestNodeForJSON(t, "1.1", "Child 1"))
	s.AddNode(makeTestNodeForJSON(t, "1.2", "Child 2"))
	s.AddNode(makeTestNodeForJSON(t, "1.3", "Child 3"))
	s.AddNode(makeTestNodeForJSON(t, "1.4", "Child 4"))

	result := RenderStatusJSON(s, 2, 1)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	nodes := parsed["nodes"].([]interface{})
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes with limit=2, offset=1, got %d", len(nodes))
	}

	// Verify the correct nodes are returned (skipping first, taking next 2)
	firstNode := nodes[0].(map[string]interface{})
	if firstNode["id"] != "1.1" {
		t.Errorf("First node id = %v, want 1.1 (after skipping 1)", firstNode["id"])
	}
}

func TestRenderStatusJSON_Pagination_OffsetExceedsTotal(t *testing.T) {
	s := state.NewState()
	s.AddNode(makeTestNodeForJSON(t, "1", "Root"))
	s.AddNode(makeTestNodeForJSON(t, "1.1", "Child"))

	result := RenderStatusJSON(s, 0, 100)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	nodes := parsed["nodes"].([]interface{})
	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes when offset exceeds total, got %d", len(nodes))
	}
}

// ============================================================================
// Tests for RenderJobsJSON
// ============================================================================

func TestRenderJobsJSON_NilJobResult(t *testing.T) {
	result := RenderJobsJSON(nil)

	expected := `{"prover_jobs":[],"verifier_jobs":[]}`
	if result != expected {
		t.Errorf("RenderJobsJSON(nil) = %q, want %q", result, expected)
	}
}

func TestRenderJobsJSON_EmptyJobs(t *testing.T) {
	jobResult := &jobs.JobResult{
		ProverJobs:   nil,
		VerifierJobs: nil,
	}

	result := RenderJobsJSON(jobResult)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	proverJobs := parsed["prover_jobs"].([]interface{})
	verifierJobs := parsed["verifier_jobs"].([]interface{})

	if len(proverJobs) != 0 {
		t.Errorf("Expected 0 prover jobs, got %d", len(proverJobs))
	}
	if len(verifierJobs) != 0 {
		t.Errorf("Expected 0 verifier jobs, got %d", len(verifierJobs))
	}
}

func TestRenderJobsJSON_ProverJobsOnly(t *testing.T) {
	jobResult := &jobs.JobResult{
		ProverJobs: []*node.Node{
			makeTestNodeForJSON(t, "1.1", "Prover job 1"),
			makeTestNodeForJSON(t, "1.2", "Prover job 2"),
		},
		VerifierJobs: nil,
	}

	result := RenderJobsJSON(jobResult)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	proverJobs := parsed["prover_jobs"].([]interface{})
	if len(proverJobs) != 2 {
		t.Errorf("Expected 2 prover jobs, got %d", len(proverJobs))
	}

	// Check first job structure
	job := proverJobs[0].(map[string]interface{})
	if job["node_id"] != "1.1" {
		t.Errorf("First prover job node_id = %v, want 1.1", job["node_id"])
	}
	if job["statement"] != "Prover job 1" {
		t.Errorf("First prover job statement = %v, want 'Prover job 1'", job["statement"])
	}
}

func TestRenderJobsJSON_VerifierJobsOnly(t *testing.T) {
	jobResult := &jobs.JobResult{
		ProverJobs: nil,
		VerifierJobs: []*node.Node{
			makeTestNodeForJSON(t, "1", "Verifier job"),
		},
	}

	result := RenderJobsJSON(jobResult)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	verifierJobs := parsed["verifier_jobs"].([]interface{})
	if len(verifierJobs) != 1 {
		t.Errorf("Expected 1 verifier job, got %d", len(verifierJobs))
	}
}

func TestRenderJobsJSON_MixedJobs(t *testing.T) {
	jobResult := &jobs.JobResult{
		ProverJobs: []*node.Node{
			makeTestNodeForJSON(t, "1.1", "Prover job"),
		},
		VerifierJobs: []*node.Node{
			makeTestNodeForJSON(t, "1", "Verifier job"),
		},
	}

	result := RenderJobsJSON(jobResult)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	proverJobs := parsed["prover_jobs"].([]interface{})
	verifierJobs := parsed["verifier_jobs"].([]interface{})

	if len(proverJobs) != 1 {
		t.Errorf("Expected 1 prover job, got %d", len(proverJobs))
	}
	if len(verifierJobs) != 1 {
		t.Errorf("Expected 1 verifier job, got %d", len(verifierJobs))
	}
}

func TestRenderJobsJSON_JobEntryHasRequiredFields(t *testing.T) {
	jobResult := &jobs.JobResult{
		ProverJobs: []*node.Node{
			makeTestNodeForJSON(t, "1.2.3", "Test job"),
		},
		VerifierJobs: nil,
	}

	result := RenderJobsJSON(jobResult)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	proverJobs := parsed["prover_jobs"].([]interface{})
	job := proverJobs[0].(map[string]interface{})

	requiredFields := []string{"node_id", "statement", "type", "depth"}
	for _, field := range requiredFields {
		if _, ok := job[field]; !ok {
			t.Errorf("Job entry missing required field %q", field)
		}
	}

	// Verify depth is correct for 1.2.3 (depth 3)
	if job["depth"].(float64) != 3 {
		t.Errorf("depth = %v, want 3", job["depth"])
	}
}

// ============================================================================
// Tests for RenderJobs (human-readable)
// ============================================================================

func TestRenderJobs_NilJobResult(t *testing.T) {
	result := RenderJobs(nil)
	if result != "" {
		t.Errorf("RenderJobs(nil) = %q, want empty string", result)
	}
}

func TestRenderJobs_EmptyJobs(t *testing.T) {
	jobResult := &jobs.JobResult{
		ProverJobs:   nil,
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// Should indicate no jobs available
	if !strings.Contains(strings.ToLower(result), "no") {
		t.Errorf("RenderJobs for empty jobs should mention 'no' jobs, got: %q", result)
	}
}

func TestRenderJobs_ProverJobsSection(t *testing.T) {
	jobResult := &jobs.JobResult{
		ProverJobs: []*node.Node{
			makeTestNodeForJSON(t, "1.1", "Prove base case"),
		},
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// Should have prover section
	if !strings.Contains(result, "Prover") {
		t.Errorf("RenderJobs should contain 'Prover' section, got: %q", result)
	}

	// Should contain node ID
	if !strings.Contains(result, "1.1") {
		t.Errorf("RenderJobs should contain node ID '1.1', got: %q", result)
	}

	// Should contain statement
	if !strings.Contains(result, "Prove base case") {
		t.Errorf("RenderJobs should contain statement, got: %q", result)
	}
}

func TestRenderJobs_VerifierJobsSection(t *testing.T) {
	jobResult := &jobs.JobResult{
		ProverJobs: nil,
		VerifierJobs: []*node.Node{
			makeTestNodeForJSON(t, "1", "Review theorem"),
		},
	}

	result := RenderJobs(jobResult)

	// Should have verifier section
	if !strings.Contains(result, "Verifier") {
		t.Errorf("RenderJobs should contain 'Verifier' section, got: %q", result)
	}

	// Should contain node ID
	if !strings.Contains(result, "1") {
		t.Errorf("RenderJobs should contain node ID, got: %q", result)
	}
}

func TestRenderJobs_SortedByID(t *testing.T) {
	// Provide nodes out of order
	jobResult := &jobs.JobResult{
		ProverJobs: []*node.Node{
			makeTestNodeForJSON(t, "1.3", "Third"),
			makeTestNodeForJSON(t, "1.1", "First"),
			makeTestNodeForJSON(t, "1.2", "Second"),
		},
		VerifierJobs: nil,
	}

	result := RenderJobs(jobResult)

	// 1.1 should appear before 1.2, which should appear before 1.3
	pos11 := strings.Index(result, "1.1")
	pos12 := strings.Index(result, "1.2")
	pos13 := strings.Index(result, "1.3")

	if pos11 > pos12 || pos12 > pos13 {
		t.Errorf("Jobs should be sorted by ID (1.1 < 1.2 < 1.3), got: %q", result)
	}
}

func TestRenderJobs_ConsistentOutput(t *testing.T) {
	jobResult := &jobs.JobResult{
		ProverJobs: []*node.Node{
			makeTestNodeForJSON(t, "1.1", "Test job"),
		},
		VerifierJobs: nil,
	}

	// Call multiple times
	result1 := RenderJobs(jobResult)
	result2 := RenderJobs(jobResult)
	result3 := RenderJobs(jobResult)

	if result1 != result2 || result2 != result3 {
		t.Error("RenderJobs output should be deterministic")
	}
}

// ============================================================================
// Tests for applyJSONPagination
// ============================================================================

func TestApplyJSONPagination_NoLimitNoOffset(t *testing.T) {
	nodes := []*node.Node{
		makeTestNodeForJSON(t, "1", "First"),
		makeTestNodeForJSON(t, "1.1", "Second"),
		makeTestNodeForJSON(t, "1.2", "Third"),
	}

	result := applyJSONPagination(nodes, 0, 0)

	if len(result) != 3 {
		t.Errorf("Expected all 3 nodes with no pagination, got %d", len(result))
	}
}

func TestApplyJSONPagination_LimitOnly(t *testing.T) {
	nodes := []*node.Node{
		makeTestNodeForJSON(t, "1", "First"),
		makeTestNodeForJSON(t, "1.1", "Second"),
		makeTestNodeForJSON(t, "1.2", "Third"),
	}

	result := applyJSONPagination(nodes, 2, 0)

	if len(result) != 2 {
		t.Errorf("Expected 2 nodes with limit=2, got %d", len(result))
	}

	if result[0].ID.String() != "1" {
		t.Errorf("First node should be '1', got %q", result[0].ID.String())
	}
}

func TestApplyJSONPagination_OffsetOnly(t *testing.T) {
	nodes := []*node.Node{
		makeTestNodeForJSON(t, "1", "First"),
		makeTestNodeForJSON(t, "1.1", "Second"),
		makeTestNodeForJSON(t, "1.2", "Third"),
	}

	result := applyJSONPagination(nodes, 0, 1)

	if len(result) != 2 {
		t.Errorf("Expected 2 nodes with offset=1, got %d", len(result))
	}

	if result[0].ID.String() != "1.1" {
		t.Errorf("First node after offset should be '1.1', got %q", result[0].ID.String())
	}
}

func TestApplyJSONPagination_LimitAndOffset(t *testing.T) {
	nodes := []*node.Node{
		makeTestNodeForJSON(t, "1", "First"),
		makeTestNodeForJSON(t, "1.1", "Second"),
		makeTestNodeForJSON(t, "1.2", "Third"),
		makeTestNodeForJSON(t, "1.3", "Fourth"),
	}

	result := applyJSONPagination(nodes, 2, 1)

	if len(result) != 2 {
		t.Errorf("Expected 2 nodes with limit=2, offset=1, got %d", len(result))
	}

	if result[0].ID.String() != "1.1" {
		t.Errorf("First node should be '1.1', got %q", result[0].ID.String())
	}
	if result[1].ID.String() != "1.2" {
		t.Errorf("Second node should be '1.2', got %q", result[1].ID.String())
	}
}

func TestApplyJSONPagination_OffsetExceedsTotal(t *testing.T) {
	nodes := []*node.Node{
		makeTestNodeForJSON(t, "1", "First"),
		makeTestNodeForJSON(t, "1.1", "Second"),
	}

	result := applyJSONPagination(nodes, 0, 100)

	if len(result) != 0 {
		t.Errorf("Expected 0 nodes when offset exceeds total, got %d", len(result))
	}
}

func TestApplyJSONPagination_LimitExceedsRemaining(t *testing.T) {
	nodes := []*node.Node{
		makeTestNodeForJSON(t, "1", "First"),
		makeTestNodeForJSON(t, "1.1", "Second"),
	}

	result := applyJSONPagination(nodes, 100, 0)

	if len(result) != 2 {
		t.Errorf("Expected all 2 nodes when limit exceeds total, got %d", len(result))
	}
}

func TestApplyJSONPagination_EmptyInput(t *testing.T) {
	var nodes []*node.Node

	result := applyJSONPagination(nodes, 10, 0)

	if len(result) != 0 {
		t.Errorf("Expected 0 nodes for empty input, got %d", len(result))
	}
}

// ============================================================================
// Tests for Node Sorting/Comparison Helpers
// ============================================================================

func TestCompareNodeIDsForJSON_SimpleComparison(t *testing.T) {
	tests := []struct {
		a, b string
		want bool // true if a < b
	}{
		{"1", "2", true},
		{"2", "1", false},
		{"1", "1", false},
		{"10", "2", false},   // numeric comparison: 10 > 2
		{"2", "10", true},    // numeric comparison: 2 < 10
		{"1.1", "1.2", true}, // hierarchical comparison
		{"1.2", "1.1", false},
		{"1.1", "1.10", true}, // numeric: 1 < 10
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			result := compareNodeIDsForJSON(tt.a, tt.b)
			if result != tt.want {
				t.Errorf("compareNodeIDsForJSON(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.want)
			}
		})
	}
}

func TestCompareNodeIDsForJSON_HierarchicalComparison(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1", "1.1", true},     // parent before child
		{"1.1", "1", false},    // child after parent
		{"1.1", "1.1.1", true}, // parent before grandchild
		{"1.1.1", "1.2", true}, // 1.1.1 < 1.2 (depth doesn't affect ordering)
		{"1.2", "1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			result := compareNodeIDsForJSON(tt.a, tt.b)
			if result != tt.want {
				t.Errorf("compareNodeIDsForJSON(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.want)
			}
		})
	}
}

func TestSplitNodeID(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1", []int{1}},
		{"1.2", []int{1, 2}},
		{"1.2.3", []int{1, 2, 3}},
		{"1.10.100", []int{1, 10, 100}},
		{"", []int{0}}, // empty string parses to [0]
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitNodeID(tt.input)
			if len(result) != len(tt.want) {
				t.Fatalf("splitNodeID(%q) length = %d, want %d", tt.input, len(result), len(tt.want))
			}
			for i, v := range result {
				if v != tt.want[i] {
					t.Errorf("splitNodeID(%q)[%d] = %d, want %d", tt.input, i, v, tt.want[i])
				}
			}
		})
	}
}

func TestSortNodesForJSON(t *testing.T) {
	nodes := []*node.Node{
		makeTestNodeForJSON(t, "1.10", "Tenth child"),
		makeTestNodeForJSON(t, "1.2", "Second child"),
		makeTestNodeForJSON(t, "1", "Root"),
		makeTestNodeForJSON(t, "1.1", "First child"),
		makeTestNodeForJSON(t, "1.1.1", "Grandchild"),
	}

	sortNodesForJSON(nodes)

	expectedOrder := []string{"1", "1.1", "1.1.1", "1.2", "1.10"}
	for i, expected := range expectedOrder {
		if nodes[i].ID.String() != expected {
			t.Errorf("After sorting, nodes[%d].ID = %q, want %q", i, nodes[i].ID.String(), expected)
		}
	}
}

// ============================================================================
// Tests for marshalJSON (HTML escaping)
// ============================================================================

func TestMarshalJSON_NoHTMLEscape(t *testing.T) {
	data := map[string]string{
		"statement": "x < 10 && y > 5",
	}

	result, err := marshalJSON(data)
	if err != nil {
		t.Fatalf("marshalJSON failed: %v", err)
	}

	// Should contain literal characters, not escaped
	resultStr := string(result)
	if !strings.Contains(resultStr, "<") {
		t.Errorf("marshalJSON should not escape '<', got: %s", resultStr)
	}
	if !strings.Contains(resultStr, ">") {
		t.Errorf("marshalJSON should not escape '>', got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "&&") {
		t.Errorf("marshalJSON should not escape '&', got: %s", resultStr)
	}

	// Should NOT contain Unicode escapes
	if strings.Contains(resultStr, `\u003c`) || strings.Contains(resultStr, `\u003e`) || strings.Contains(resultStr, `\u0026`) {
		t.Errorf("marshalJSON should not produce Unicode escapes, got: %s", resultStr)
	}
}

func TestMarshalJSON_ValidOutput(t *testing.T) {
	data := map[string]interface{}{
		"id":        "1.2.3",
		"statement": "Test with special chars: <>&",
	}

	result, err := marshalJSON(data)
	if err != nil {
		t.Fatalf("marshalJSON failed: %v", err)
	}

	// Should be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("marshalJSON produced invalid JSON: %v", err)
	}
}

// ============================================================================
// Tests for nodeToJSON
// ============================================================================

func TestNodeToJSON_BasicFields(t *testing.T) {
	n := makeTestNodeForJSON(t, "1.2.3", "Test statement")

	result := nodeToJSON(n)

	if result.ID != "1.2.3" {
		t.Errorf("ID = %q, want %q", result.ID, "1.2.3")
	}
	if result.Statement != "Test statement" {
		t.Errorf("Statement = %q, want %q", result.Statement, "Test statement")
	}
	if result.Type != "claim" {
		t.Errorf("Type = %q, want %q", result.Type, "claim")
	}
	if result.Inference != "modus_ponens" {
		t.Errorf("Inference = %q, want %q", result.Inference, "modus_ponens")
	}
}

func TestNodeToJSON_WithContext(t *testing.T) {
	n := makeTestNodeForJSON(t, "1", "Test")
	n.Context = []string{"def:natural_number", "def:prime"}

	result := nodeToJSON(n)

	if len(result.Context) != 2 {
		t.Errorf("Context length = %d, want 2", len(result.Context))
	}
	if result.Context[0] != "def:natural_number" {
		t.Errorf("Context[0] = %q, want %q", result.Context[0], "def:natural_number")
	}
}

func TestNodeToJSON_WithDependencies(t *testing.T) {
	n := makeTestNodeForJSON(t, "1.3", "Test")
	dep1, _ := types.Parse("1.1")
	dep2, _ := types.Parse("1.2")
	n.Dependencies = []types.NodeID{dep1, dep2}

	result := nodeToJSON(n)

	if len(result.Dependencies) != 2 {
		t.Errorf("Dependencies length = %d, want 2", len(result.Dependencies))
	}
	if result.Dependencies[0] != "1.1" {
		t.Errorf("Dependencies[0] = %q, want %q", result.Dependencies[0], "1.1")
	}
}

func TestNodeToJSON_WithValidationDeps(t *testing.T) {
	n := makeTestNodeForJSON(t, "1.3", "Test")
	dep, _ := types.Parse("1.2")
	n.ValidationDeps = []types.NodeID{dep}

	result := nodeToJSON(n)

	if len(result.ValidationDeps) != 1 {
		t.Errorf("ValidationDeps length = %d, want 1", len(result.ValidationDeps))
	}
	if result.ValidationDeps[0] != "1.2" {
		t.Errorf("ValidationDeps[0] = %q, want %q", result.ValidationDeps[0], "1.2")
	}
}

func TestNodeToJSON_WithScope(t *testing.T) {
	n := makeTestNodeForJSON(t, "1", "Test")
	n.Scope = []string{"assume:n>=0", "assume:m>=0"}

	result := nodeToJSON(n)

	if len(result.Scope) != 2 {
		t.Errorf("Scope length = %d, want 2", len(result.Scope))
	}
	if result.Scope[0] != "assume:n>=0" {
		t.Errorf("Scope[0] = %q, want %q", result.Scope[0], "assume:n>=0")
	}
}

func TestNodeToJSON_WithClaimedBy(t *testing.T) {
	n := makeTestNodeForJSON(t, "1", "Test")
	n.ClaimedBy = "agent-xyz123"

	result := nodeToJSON(n)

	if result.ClaimedBy != "agent-xyz123" {
		t.Errorf("ClaimedBy = %q, want %q", result.ClaimedBy, "agent-xyz123")
	}
}

// ============================================================================
// Tests for RenderChallengeJSON
// ============================================================================

func TestRenderChallengeJSON_NilChallenge(t *testing.T) {
	result := RenderChallengeJSON(nil)
	if result != "{}" {
		t.Errorf("RenderChallengeJSON(nil) = %q, want %q", result, "{}")
	}
}

func TestRenderChallengeJSON_BasicChallenge(t *testing.T) {
	nodeID, _ := types.Parse("1.2")
	c := &state.Challenge{
		ID:       "ch-001",
		NodeID:   nodeID,
		Target:   "Statement unclear",
		Reason:   "The proof step is not justified",
		Status:   "open",
		Severity: "high",
	}

	result := RenderChallengeJSON(c)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("RenderChallengeJSON produced invalid JSON: %v", err)
	}

	if parsed["id"] != "ch-001" {
		t.Errorf("id = %v, want %q", parsed["id"], "ch-001")
	}
	if parsed["target_id"] != "1.2" {
		t.Errorf("target_id = %v, want %q", parsed["target_id"], "1.2")
	}
	if parsed["status"] != "open" {
		t.Errorf("status = %v, want %q", parsed["status"], "open")
	}
}

// ============================================================================
// Tests for RenderChallengesJSON
// ============================================================================

func TestRenderChallengesJSON_NilList(t *testing.T) {
	result := RenderChallengesJSON(nil)
	if result != "[]" {
		t.Errorf("RenderChallengesJSON(nil) = %q, want %q", result, "[]")
	}
}

func TestRenderChallengesJSON_EmptyList(t *testing.T) {
	result := RenderChallengesJSON([]*state.Challenge{})
	if result != "[]" {
		t.Errorf("RenderChallengesJSON([]) = %q, want %q", result, "[]")
	}
}

func TestRenderChallengesJSON_MultiChallenges(t *testing.T) {
	nodeID1, _ := types.Parse("1.1")
	nodeID2, _ := types.Parse("1.2")
	challenges := []*state.Challenge{
		{
			ID:     "ch-001",
			NodeID: nodeID1,
			Status: "open",
		},
		{
			ID:     "ch-002",
			NodeID: nodeID2,
			Status: "resolved",
		},
	}

	result := RenderChallengesJSON(challenges)

	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("RenderChallengesJSON produced invalid JSON: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("Expected 2 challenges, got %d", len(parsed))
	}
}

func TestRenderChallengesJSON_SkipsNilChallenges(t *testing.T) {
	nodeID, _ := types.Parse("1.1")
	challenges := []*state.Challenge{
		{
			ID:     "ch-001",
			NodeID: nodeID,
			Status: "open",
		},
		nil,
	}

	result := RenderChallengesJSON(challenges)

	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("RenderChallengesJSON produced invalid JSON: %v", err)
	}

	if len(parsed) != 1 {
		t.Errorf("Expected 1 challenge (skipping nil), got %d", len(parsed))
	}
}

// ============================================================================
// Tests for RenderNodeChallengesJSON
// ============================================================================

func TestRenderNodeChallengesJSON_NilState(t *testing.T) {
	nodeID, _ := types.Parse("1")
	result := RenderNodeChallengesJSON(nil, nodeID)
	if result != "[]" {
		t.Errorf("RenderNodeChallengesJSON(nil, ...) = %q, want %q", result, "[]")
	}
}

func TestRenderNodeChallengesJSON_NoChallenges(t *testing.T) {
	s := state.NewState()
	s.AddNode(makeTestNodeForJSON(t, "1", "Root"))
	nodeID, _ := types.Parse("1")

	result := RenderNodeChallengesJSON(s, nodeID)
	if result != "[]" {
		t.Errorf("RenderNodeChallengesJSON with no challenges = %q, want %q", result, "[]")
	}
}

// ============================================================================
// Tests for RenderProverContextJSON
// ============================================================================

func TestRenderProverContextJSON_NilState(t *testing.T) {
	nodeID, _ := types.Parse("1")
	result := RenderProverContextJSON(nil, nodeID)
	if !strings.Contains(result, "error") {
		t.Errorf("RenderProverContextJSON(nil, ...) should return error JSON, got: %s", result)
	}
}

func TestRenderProverContextJSON_NodeNotFound(t *testing.T) {
	s := state.NewState()
	nodeID, _ := types.Parse("1.99")

	result := RenderProverContextJSON(s, nodeID)
	if !strings.Contains(result, "not found") {
		t.Errorf("RenderProverContextJSON for missing node should mention 'not found', got: %s", result)
	}
}

func TestRenderProverContextJSON_ValidNode(t *testing.T) {
	s := state.NewState()
	s.AddNode(makeTestNodeForJSON(t, "1", "Root"))
	s.AddNode(makeTestNodeForJSON(t, "1.1", "Child"))
	nodeID, _ := types.Parse("1")

	result := RenderProverContextJSON(s, nodeID)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("RenderProverContextJSON produced invalid JSON: %v", err)
	}

	// Should have node field
	if _, ok := parsed["node"]; !ok {
		t.Error("RenderProverContextJSON missing 'node' field")
	}

	// Should have children field
	if _, ok := parsed["children"]; !ok {
		t.Error("RenderProverContextJSON missing 'children' field")
	}
}

// ============================================================================
// Tests for RenderVerifierContextJSON
// ============================================================================

func TestRenderVerifierContextJSON_NilState(t *testing.T) {
	challenge := &node.Challenge{
		ID: "ch-001",
	}
	result := RenderVerifierContextJSON(nil, challenge)
	if !strings.Contains(result, "error") {
		t.Errorf("RenderVerifierContextJSON(nil, ...) should return error JSON, got: %s", result)
	}
}

func TestRenderVerifierContextJSON_NilChallenge(t *testing.T) {
	s := state.NewState()
	result := RenderVerifierContextJSON(s, nil)
	if !strings.Contains(result, "error") {
		t.Errorf("RenderVerifierContextJSON(..., nil) should return error JSON, got: %s", result)
	}
}

func TestRenderVerifierContextJSON_ValidChallenge(t *testing.T) {
	s := state.NewState()
	s.AddNode(makeTestNodeForJSON(t, "1", "Root"))
	targetID, _ := types.Parse("1")

	challenge := &node.Challenge{
		ID:       "ch-001",
		TargetID: targetID,
		Target:   schema.TargetStatement,
		Reason:   "Statement unclear",
		Status:   node.ChallengeStatusOpen,
	}

	result := RenderVerifierContextJSON(s, challenge)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("RenderVerifierContextJSON produced invalid JSON: %v", err)
	}

	if parsed["challenge_id"] != "ch-001" {
		t.Errorf("challenge_id = %v, want %q", parsed["challenge_id"], "ch-001")
	}
}

// ============================================================================
// Additional edge case tests
// ============================================================================

func TestRenderStatusJSON_WithJobCounts(t *testing.T) {
	s := state.NewState()
	// Prover job: available + pending
	s.AddNode(makeTestNodeWithState(t, "1", "Root", schema.WorkflowAvailable, schema.EpistemicPending))
	s.AddNode(makeTestNodeWithState(t, "1.1", "Child prover", schema.WorkflowAvailable, schema.EpistemicPending))
	// Not a prover job: claimed
	s.AddNode(makeTestNodeWithState(t, "1.2", "Claimed", schema.WorkflowClaimed, schema.EpistemicPending))

	result := RenderStatusJSON(s, 0, 0)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	jobs := parsed["jobs"].(map[string]interface{})
	proverJobs := int(jobs["prover_jobs"].(float64))

	// 2 nodes are available + pending = prover jobs
	if proverJobs != 2 {
		t.Errorf("prover_jobs = %d, want 2", proverJobs)
	}
}

func TestRenderStatusJSON_NodesSortedByID(t *testing.T) {
	s := state.NewState()
	// Add nodes out of order
	s.AddNode(makeTestNodeForJSON(t, "1.3", "Third"))
	s.AddNode(makeTestNodeForJSON(t, "1", "Root"))
	s.AddNode(makeTestNodeForJSON(t, "1.1", "First"))
	s.AddNode(makeTestNodeForJSON(t, "1.2", "Second"))

	result := RenderStatusJSON(s, 0, 0)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	nodes := parsed["nodes"].([]interface{})
	expectedOrder := []string{"1", "1.1", "1.2", "1.3"}

	for i, expected := range expectedOrder {
		node := nodes[i].(map[string]interface{})
		if node["id"] != expected {
			t.Errorf("nodes[%d].id = %v, want %q", i, node["id"], expected)
		}
	}
}
