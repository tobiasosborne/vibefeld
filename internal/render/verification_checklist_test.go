//go:build integration

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

// makeTestNodeForChecklist creates a test node with minimal required fields.
// Panics on invalid input (intended for test use only).
func makeTestNodeForChecklist(id string, nodeType schema.NodeType, statement string, inference schema.InferenceType) *node.Node {
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

// TestRenderVerificationChecklist_IncludesStatementCheck tests that the checklist includes statement precision check.
func TestRenderVerificationChecklist_IncludesStatementCheck(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "For all natural numbers n, if n is even, then n^2 is even", schema.InferenceModusPonens)
	s.AddNode(n)

	result := RenderVerificationChecklist(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklist returned empty string for valid node")
	}

	// Should include a statement precision check
	lowerResult := strings.ToLower(result)
	if !strings.Contains(lowerResult, "statement") {
		t.Errorf("RenderVerificationChecklist should include statement check, got: %q", result)
	}

	// Should show the actual statement being checked
	if !strings.Contains(result, "natural numbers") || !strings.Contains(result, "even") {
		t.Errorf("RenderVerificationChecklist should show the actual statement, got: %q", result)
	}
}

// TestRenderVerificationChecklist_IncludesInferenceType tests that the checklist shows actual inference type.
func TestRenderVerificationChecklist_IncludesInferenceType(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "A claim using modus ponens", schema.InferenceModusPonens)
	s.AddNode(n)

	result := RenderVerificationChecklist(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklist returned empty string for valid node")
	}

	// Should include inference type indicator
	lowerResult := strings.ToLower(result)
	if !strings.Contains(lowerResult, "inference") {
		t.Errorf("RenderVerificationChecklist should mention inference, got: %q", result)
	}

	// Should show the actual inference type (modus_ponens or Modus Ponens)
	if !strings.Contains(lowerResult, "modus_ponens") && !strings.Contains(lowerResult, "modus ponens") {
		t.Errorf("RenderVerificationChecklist should show the actual inference type, got: %q", result)
	}
}

// TestRenderVerificationChecklist_ListsDependencies tests that the checklist lists dependencies.
func TestRenderVerificationChecklist_ListsDependencies(t *testing.T) {
	s := state.NewState()

	// Create dependency nodes
	dep1 := makeTestNodeForChecklist("1.1", schema.NodeTypeClaim, "First premise", schema.InferenceAssumption)
	dep2 := makeTestNodeForChecklist("1.2", schema.NodeTypeClaim, "Second premise", schema.InferenceAssumption)
	s.AddNode(dep1)
	s.AddNode(dep2)

	// Create node with dependencies
	nodeID, _ := types.Parse("1.3")
	depID1, _ := types.Parse("1.1")
	depID2, _ := types.Parse("1.2")
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

	result := RenderVerificationChecklist(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklist returned empty string for node with dependencies")
	}

	// Should list the dependencies
	if !strings.Contains(result, "1.1") || !strings.Contains(result, "1.2") {
		t.Errorf("RenderVerificationChecklist should list dependencies, got: %q", result)
	}
}

// TestRenderVerificationChecklist_ShowsDependencyStatus tests that dependencies show validation status.
func TestRenderVerificationChecklist_ShowsDependencyStatus(t *testing.T) {
	s := state.NewState()

	// Create dependency nodes with different states
	dep1 := makeTestNodeForChecklist("1.1", schema.NodeTypeClaim, "Validated premise", schema.InferenceAssumption)
	dep1.EpistemicState = schema.EpistemicValidated

	dep2 := makeTestNodeForChecklist("1.2", schema.NodeTypeClaim, "Pending premise", schema.InferenceAssumption)
	// dep2 stays in pending state (default)

	s.AddNode(dep1)
	s.AddNode(dep2)

	// Create node with dependencies
	nodeID, _ := types.Parse("1.3")
	depID1, _ := types.Parse("1.1")
	depID2, _ := types.Parse("1.2")
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

	result := RenderVerificationChecklist(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklist returned empty string")
	}

	// Should show validation status indicators
	lowerResult := strings.ToLower(result)
	// Should indicate validated and pending states somehow
	hasStatusIndicators := strings.Contains(lowerResult, "validated") ||
		strings.Contains(lowerResult, "pending") ||
		strings.Contains(result, "[v]") || strings.Contains(result, "[ ]") ||
		strings.Contains(result, "(validated)") || strings.Contains(result, "(pending)")

	if !hasStatusIndicators {
		t.Errorf("RenderVerificationChecklist should show dependency validation status, got: %q", result)
	}
}

// TestRenderVerificationChecklist_SuggestsChallengeCommand tests that the checklist suggests the challenge command.
func TestRenderVerificationChecklist_SuggestsChallengeCommand(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1.2", schema.NodeTypeClaim, "A claim to verify", schema.InferenceModusPonens)
	s.AddNode(n)

	result := RenderVerificationChecklist(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklist returned empty string for valid node")
	}

	// Should suggest challenge command
	lowerResult := strings.ToLower(result)
	if !strings.Contains(lowerResult, "challenge") {
		t.Errorf("RenderVerificationChecklist should suggest challenge command, got: %q", result)
	}

	// Should include the node ID in the suggestion
	if !strings.Contains(result, "1.2") {
		t.Errorf("RenderVerificationChecklist should include node ID in challenge suggestion, got: %q", result)
	}
}

// TestRenderVerificationChecklist_HandlesNilNode tests handling of nil node.
func TestRenderVerificationChecklist_HandlesNilNode(t *testing.T) {
	s := state.NewState()

	// Add some data to state to ensure it doesn't cause panic
	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "A claim", schema.InferenceModusPonens)
	s.AddNode(n)

	// Should not panic with nil node
	result := RenderVerificationChecklist(nil, s)

	// Should return empty string for nil node
	if result != "" {
		t.Errorf("RenderVerificationChecklist should return empty string for nil node, got: %q", result)
	}
}

// TestRenderVerificationChecklist_HandlesNilState tests handling of nil state.
func TestRenderVerificationChecklist_HandlesNilState(t *testing.T) {
	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "A claim", schema.InferenceModusPonens)

	// Should not panic with nil state
	result := RenderVerificationChecklist(n, nil)

	// Should return empty string for nil state
	if result != "" {
		t.Errorf("RenderVerificationChecklist should return empty string for nil state, got: %q", result)
	}
}

// TestRenderVerificationChecklist_IncludesHiddenAssumptionsCheck tests that hidden assumptions check is included.
func TestRenderVerificationChecklist_IncludesHiddenAssumptionsCheck(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "A mathematical claim", schema.InferenceModusPonens)
	s.AddNode(n)

	result := RenderVerificationChecklist(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklist returned empty string for valid node")
	}

	// Should include hidden assumptions check
	lowerResult := strings.ToLower(result)
	if !strings.Contains(lowerResult, "hidden") || !strings.Contains(lowerResult, "assumption") {
		t.Errorf("RenderVerificationChecklist should include hidden assumptions check, got: %q", result)
	}
}

// TestRenderVerificationChecklist_IncludesDomainRestrictions tests that domain restrictions check is included.
func TestRenderVerificationChecklist_IncludesDomainRestrictions(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "For all x in domain D, P(x)", schema.InferenceUniversalInstantiation)
	s.AddNode(n)

	result := RenderVerificationChecklist(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklist returned empty string for valid node")
	}

	// Should include domain restrictions check
	lowerResult := strings.ToLower(result)
	if !strings.Contains(lowerResult, "domain") {
		t.Errorf("RenderVerificationChecklist should include domain restrictions check, got: %q", result)
	}
}

// TestRenderVerificationChecklist_IncludesNotationConsistency tests that notation consistency check is included.
func TestRenderVerificationChecklist_IncludesNotationConsistency(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "Let f: A -> B be a function", schema.InferenceByDefinition)
	s.AddNode(n)

	result := RenderVerificationChecklist(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklist returned empty string for valid node")
	}

	// Should include notation consistency check
	lowerResult := strings.ToLower(result)
	if !strings.Contains(lowerResult, "notation") {
		t.Errorf("RenderVerificationChecklist should include notation consistency check, got: %q", result)
	}
}

// TestRenderVerificationChecklist_ShowsInferenceForm tests that the inference logical form is shown.
func TestRenderVerificationChecklist_ShowsInferenceForm(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "Using modus ponens", schema.InferenceModusPonens)
	s.AddNode(n)

	result := RenderVerificationChecklist(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklist returned empty string for valid node")
	}

	// Should show the logical form for the inference type
	// Modus Ponens form is "P, P -> Q |-- Q"
	if !strings.Contains(result, "P") || !strings.Contains(result, "Q") {
		t.Logf("Note: RenderVerificationChecklist may want to show inference logical form, got: %q", result)
	}
}

// TestRenderVerificationChecklist_NoDependencies tests checklist for node with no dependencies.
func TestRenderVerificationChecklist_NoDependencies(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "A claim with no dependencies", schema.InferenceAssumption)
	s.AddNode(n)

	result := RenderVerificationChecklist(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklist returned empty string for valid node")
	}

	// Should indicate no dependencies or handle gracefully
	lowerResult := strings.ToLower(result)
	if !strings.Contains(lowerResult, "dependenc") {
		t.Logf("Note: RenderVerificationChecklist may want to mention dependencies section even if empty, got: %q", result)
	}
}

// TestRenderVerificationChecklist_ConsistentOutput tests that repeated calls produce consistent output.
func TestRenderVerificationChecklist_ConsistentOutput(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "Deterministic output test", schema.InferenceModusPonens)
	s.AddNode(n)

	// Call multiple times
	result1 := RenderVerificationChecklist(n, s)
	result2 := RenderVerificationChecklist(n, s)
	result3 := RenderVerificationChecklist(n, s)

	// All calls should produce identical output (deterministic)
	if result1 != result2 || result2 != result3 {
		t.Errorf("RenderVerificationChecklist produced inconsistent output:\n1: %q\n2: %q\n3: %q",
			result1, result2, result3)
	}
}

// TestRenderVerificationChecklist_MultiLineOutput tests that output is properly formatted multi-line text.
func TestRenderVerificationChecklist_MultiLineOutput(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "A claim for formatting test", schema.InferenceModusPonens)
	s.AddNode(n)

	result := RenderVerificationChecklist(n, s)

	// Should be multi-line output (contains newlines)
	if !strings.Contains(result, "\n") {
		t.Errorf("RenderVerificationChecklist should use multi-line format, got: %q", result)
	}

	// Should be human-readable (not JSON or other machine format)
	if strings.HasPrefix(strings.TrimSpace(result), "{") || strings.HasPrefix(strings.TrimSpace(result), "[") {
		t.Errorf("RenderVerificationChecklist should return human-readable text, not JSON, got: %q", result)
	}
}

// TestRenderVerificationChecklist_IncludesHeader tests that output includes appropriate header.
func TestRenderVerificationChecklist_IncludesHeader(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1.2", schema.NodeTypeClaim, "A claim", schema.InferenceModusPonens)
	s.AddNode(n)

	result := RenderVerificationChecklist(n, s)

	// Should not be empty
	if result == "" {
		t.Fatal("RenderVerificationChecklist returned empty string")
	}

	// Should include some form of header indicating this is a verification checklist
	lowerResult := strings.ToLower(result)
	if !strings.Contains(lowerResult, "verification") && !strings.Contains(lowerResult, "checklist") {
		t.Logf("Note: RenderVerificationChecklist may want to include a header, got: %q", result)
	}
}

// TestRenderVerificationChecklist_DifferentInferenceTypes tests checklist for different inference types.
func TestRenderVerificationChecklist_DifferentInferenceTypes(t *testing.T) {
	tests := []struct {
		name      string
		inference schema.InferenceType
	}{
		{"modus_ponens", schema.InferenceModusPonens},
		{"modus_tollens", schema.InferenceModusTollens},
		{"universal_instantiation", schema.InferenceUniversalInstantiation},
		{"existential_instantiation", schema.InferenceExistentialInstantiation},
		{"universal_generalization", schema.InferenceUniversalGeneralization},
		{"existential_generalization", schema.InferenceExistentialGeneralization},
		{"by_definition", schema.InferenceByDefinition},
		{"assumption", schema.InferenceAssumption},
		{"local_assume", schema.InferenceLocalAssume},
		{"local_discharge", schema.InferenceLocalDischarge},
		{"contradiction", schema.InferenceContradiction},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := state.NewState()
			n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "A claim using "+tt.name, tt.inference)
			s.AddNode(n)

			result := RenderVerificationChecklist(n, s)

			// Should not be empty for any valid inference type
			if result == "" {
				t.Fatalf("RenderVerificationChecklist returned empty string for %s", tt.name)
			}

			// Should contain the inference type somewhere
			lowerResult := strings.ToLower(result)
			if !strings.Contains(lowerResult, strings.ReplaceAll(tt.name, "_", " ")) &&
				!strings.Contains(lowerResult, tt.name) {
				t.Logf("Note: RenderVerificationChecklist may want to show inference type %q, got: %q", tt.name, result)
			}
		})
	}
}

// ============================================================================
// JSON Format Tests
// ============================================================================

// TestRenderVerificationChecklistJSON_ValidJSON verifies output is valid JSON.
func TestRenderVerificationChecklistJSON_ValidJSON(t *testing.T) {
	s := state.NewState()

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "For all n, if n is even, then n^2 is even", schema.InferenceModusPonens)
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
	dep1 := makeTestNodeForChecklist("1.1", schema.NodeTypeClaim, "First premise", schema.InferenceAssumption)
	dep1.EpistemicState = schema.EpistemicValidated
	dep2 := makeTestNodeForChecklist("1.2", schema.NodeTypeClaim, "Second premise", schema.InferenceAssumption)
	s.AddNode(dep1)
	s.AddNode(dep2)

	// Create node with dependencies
	nodeID, _ := types.Parse("1.3")
	depID1, _ := types.Parse("1.1")
	depID2, _ := types.Parse("1.2")
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
		"statement_precision": false,
		"inference_validity":  false,
		"dependencies":        false,
		"hidden_assumptions":  false,
		"domain_restrictions": false,
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
	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "A claim", schema.InferenceModusPonens)

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

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "No deps", schema.InferenceAssumption)
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
	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, statement, schema.InferenceModusPonens)
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

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "A claim", schema.InferenceModusPonens)
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

	n := makeTestNodeForChecklist("1", schema.NodeTypeClaim, "Deterministic test", schema.InferenceModusPonens)
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
	dep3 := makeTestNodeForChecklist("1.3", schema.NodeTypeClaim, "Third", schema.InferenceAssumption)
	dep1 := makeTestNodeForChecklist("1.1", schema.NodeTypeClaim, "First", schema.InferenceAssumption)
	dep2 := makeTestNodeForChecklist("1.2", schema.NodeTypeClaim, "Second", schema.InferenceAssumption)
	s.AddNode(dep1)
	s.AddNode(dep2)
	s.AddNode(dep3)

	// Create node with dependencies in non-sorted order
	nodeID, _ := types.Parse("1.4")
	depID1, _ := types.Parse("1.1")
	depID2, _ := types.Parse("1.2")
	depID3, _ := types.Parse("1.3")
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
