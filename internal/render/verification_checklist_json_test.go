package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// makeTestNodeJSON creates a test node for JSON tests.
func makeTestNodeJSON(id string, nodeType schema.NodeType, statement string, inference schema.InferenceType) *node.Node {
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

// TestRenderVerificationChecklistJSON_ValidJSON verifies output is valid JSON.
func TestRenderVerificationChecklistJSON_ValidJSON(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeJSON("1", schema.NodeTypeClaim, "For all n, if n is even, then n^2 is even", schema.InferenceModusPonens)
	s.AddNode(n)

	result := RenderVerificationChecklistJSON(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklistJSON returned empty string for valid node")
	}

	// Should be valid JSON - try to unmarshal into a map
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(result), &parsed)
	if err != nil {
		t.Errorf("RenderVerificationChecklistJSON returned invalid JSON: %v\nOutput: %s", err, result)
	}
}

// TestRenderVerificationChecklistJSON_IncludesAllFields verifies all required fields are present.
func TestRenderVerificationChecklistJSON_IncludesAllFields(t *testing.T) {
	s := state.NewState()

	// Create dependency nodes
	depID1, _ := types.Parse("1.1")
	dep1, _ := node.NewNode(depID1, schema.NodeTypeClaim, "First premise", schema.InferenceAssumption)
	dep1.EpistemicState = schema.EpistemicValidated
	s.AddNode(dep1)

	depID2, _ := types.Parse("1.2")
	dep2, _ := node.NewNode(depID2, schema.NodeTypeClaim, "Second premise", schema.InferenceAssumption)
	s.AddNode(dep2)

	// Create node with dependencies
	nodeID, _ := types.Parse("1.3")
	n, _ := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"Conclusion from premises",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Dependencies: []types.NodeID{depID1, depID2},
		},
	)
	s.AddNode(n)

	result := RenderVerificationChecklistJSON(n, s)

	// Parse the JSON into the struct
	var checklist JSONVerificationChecklist
	err := json.Unmarshal([]byte(result), &checklist)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v\nOutput: %s", err, result)
	}

	// Check node_id field
	if checklist.NodeID != "1.3" {
		t.Errorf("node_id = %q, want %q", checklist.NodeID, "1.3")
	}

	// Check items array is present and has expected categories
	if len(checklist.Items) == 0 {
		t.Error("items array is empty, expected checklist items")
	}

	expectedCategories := map[string]bool{
		"statement_precision":  false,
		"inference_validity":   false,
		"dependencies":         false,
		"hidden_assumptions":   false,
		"domain_restrictions":  false,
		"notation_consistency": false,
	}

	for _, item := range checklist.Items {
		if _, exists := expectedCategories[item.Category]; exists {
			expectedCategories[item.Category] = true
		}
		// Verify each item has description and checks
		if item.Description == "" {
			t.Errorf("Item %q has empty description", item.Category)
		}
		if len(item.Checks) == 0 {
			t.Errorf("Item %q has no checks", item.Category)
		}
	}

	for category, found := range expectedCategories {
		if !found {
			t.Errorf("Missing expected category: %s", category)
		}
	}

	// Check dependencies array
	if len(checklist.Dependencies) != 2 {
		t.Errorf("dependencies count = %d, want 2", len(checklist.Dependencies))
	}

	// Verify dependency info
	depMap := make(map[string]JSONChecklistDependency)
	for _, dep := range checklist.Dependencies {
		depMap[dep.NodeID] = dep
	}

	if dep, ok := depMap["1.1"]; ok {
		if dep.EpistemicState != "validated" {
			t.Errorf("dependency 1.1 epistemic_state = %q, want %q", dep.EpistemicState, "validated")
		}
		if dep.Statement != "First premise" {
			t.Errorf("dependency 1.1 statement = %q, want %q", dep.Statement, "First premise")
		}
	} else {
		t.Error("Missing dependency 1.1 in dependencies array")
	}

	if _, ok := depMap["1.2"]; !ok {
		t.Error("Missing dependency 1.2 in dependencies array")
	}

	// Check challenge_command field
	if checklist.ChallengeCommand == "" {
		t.Error("challenge_command is empty")
	}
	if !strings.Contains(checklist.ChallengeCommand, "1.3") {
		t.Errorf("challenge_command should contain node ID, got: %s", checklist.ChallengeCommand)
	}
	if !strings.Contains(checklist.ChallengeCommand, "af challenge") {
		t.Errorf("challenge_command should contain 'af challenge', got: %s", checklist.ChallengeCommand)
	}
}

// TestRenderVerificationChecklistJSON_NilNode verifies handling of nil node.
func TestRenderVerificationChecklistJSON_NilNode(t *testing.T) {
	s := state.NewState()

	result := RenderVerificationChecklistJSON(nil, s)

	if result != "{}" {
		t.Errorf("RenderVerificationChecklistJSON(nil, s) = %q, want %q", result, "{}")
	}

	// Should still be valid JSON
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(result), &parsed)
	if err != nil {
		t.Errorf("Result is not valid JSON: %v", err)
	}
}

// TestRenderVerificationChecklistJSON_NilState verifies handling of nil state.
func TestRenderVerificationChecklistJSON_NilState(t *testing.T) {
	n := makeTestNodeJSON("1", schema.NodeTypeClaim, "A claim", schema.InferenceModusPonens)

	result := RenderVerificationChecklistJSON(n, nil)

	if result != "{}" {
		t.Errorf("RenderVerificationChecklistJSON(n, nil) = %q, want %q", result, "{}")
	}

	// Should still be valid JSON
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(result), &parsed)
	if err != nil {
		t.Errorf("Result is not valid JSON: %v", err)
	}
}

// TestRenderVerificationChecklistJSON_NoDependencies verifies output when node has no dependencies.
func TestRenderVerificationChecklistJSON_NoDependencies(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeJSON("1", schema.NodeTypeClaim, "No deps", schema.InferenceAssumption)
	s.AddNode(n)

	result := RenderVerificationChecklistJSON(n, s)

	var checklist JSONVerificationChecklist
	err := json.Unmarshal([]byte(result), &checklist)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Dependencies should be empty array, not nil
	if checklist.Dependencies == nil {
		t.Error("dependencies should be empty array, not nil")
	}
	if len(checklist.Dependencies) != 0 {
		t.Errorf("dependencies count = %d, want 0", len(checklist.Dependencies))
	}
}

// TestRenderVerificationChecklistJSON_StatementIncluded verifies the statement is included in items.
func TestRenderVerificationChecklistJSON_StatementIncluded(t *testing.T) {
	s := state.NewState()

	statement := "For all natural numbers n, if n is even, then n^2 is even"
	n := makeTestNodeJSON("1", schema.NodeTypeClaim, statement, schema.InferenceModusPonens)
	s.AddNode(n)

	result := RenderVerificationChecklistJSON(n, s)

	var checklist JSONVerificationChecklist
	err := json.Unmarshal([]byte(result), &checklist)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Find statement_precision item and verify it contains the statement
	var found bool
	for _, item := range checklist.Items {
		if item.Category == "statement_precision" {
			found = true
			if item.Details != statement {
				t.Errorf("statement_precision details = %q, want %q", item.Details, statement)
			}
			break
		}
	}
	if !found {
		t.Error("statement_precision item not found")
	}
}

// TestRenderVerificationChecklistJSON_InferenceIncluded verifies inference type is included.
func TestRenderVerificationChecklistJSON_InferenceIncluded(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeJSON("1", schema.NodeTypeClaim, "A claim", schema.InferenceModusPonens)
	s.AddNode(n)

	result := RenderVerificationChecklistJSON(n, s)

	var checklist JSONVerificationChecklist
	err := json.Unmarshal([]byte(result), &checklist)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Find inference_validity item and verify it contains the inference type
	var found bool
	for _, item := range checklist.Items {
		if item.Category == "inference_validity" {
			found = true
			if !strings.Contains(item.Details, "modus_ponens") {
				t.Errorf("inference_validity details should contain 'modus_ponens', got: %q", item.Details)
			}
			break
		}
	}
	if !found {
		t.Error("inference_validity item not found")
	}
}

// TestRenderVerificationChecklistJSON_ConsistentOutput verifies repeated calls produce consistent output.
func TestRenderVerificationChecklistJSON_ConsistentOutput(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeJSON("1", schema.NodeTypeClaim, "Deterministic test", schema.InferenceModusPonens)
	s.AddNode(n)

	result1 := RenderVerificationChecklistJSON(n, s)
	result2 := RenderVerificationChecklistJSON(n, s)
	result3 := RenderVerificationChecklistJSON(n, s)

	if result1 != result2 || result2 != result3 {
		t.Error("RenderVerificationChecklistJSON produced inconsistent output on repeated calls")
	}
}

// TestRenderVerificationChecklistJSON_DependencySorted verifies dependencies are sorted.
func TestRenderVerificationChecklistJSON_DependencySorted(t *testing.T) {
	s := state.NewState()

	// Create dependencies in reverse order
	depID3, _ := types.Parse("1.3")
	dep3, _ := node.NewNode(depID3, schema.NodeTypeClaim, "Third", schema.InferenceAssumption)
	s.AddNode(dep3)

	depID1, _ := types.Parse("1.1")
	dep1, _ := node.NewNode(depID1, schema.NodeTypeClaim, "First", schema.InferenceAssumption)
	s.AddNode(dep1)

	depID2, _ := types.Parse("1.2")
	dep2, _ := node.NewNode(depID2, schema.NodeTypeClaim, "Second", schema.InferenceAssumption)
	s.AddNode(dep2)

	// Create node with dependencies in non-sorted order
	nodeID, _ := types.Parse("1.4")
	n, _ := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"Conclusion",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Dependencies: []types.NodeID{depID3, depID1, depID2}, // Deliberately unsorted
		},
	)
	s.AddNode(n)

	result := RenderVerificationChecklistJSON(n, s)

	var checklist JSONVerificationChecklist
	err := json.Unmarshal([]byte(result), &checklist)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify dependencies are sorted
	if len(checklist.Dependencies) != 3 {
		t.Fatalf("Expected 3 dependencies, got %d", len(checklist.Dependencies))
	}
	if checklist.Dependencies[0].NodeID != "1.1" {
		t.Errorf("First dependency = %q, want %q", checklist.Dependencies[0].NodeID, "1.1")
	}
	if checklist.Dependencies[1].NodeID != "1.2" {
		t.Errorf("Second dependency = %q, want %q", checklist.Dependencies[1].NodeID, "1.2")
	}
	if checklist.Dependencies[2].NodeID != "1.3" {
		t.Errorf("Third dependency = %q, want %q", checklist.Dependencies[2].NodeID, "1.3")
	}
}
