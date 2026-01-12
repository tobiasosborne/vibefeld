// Package schema_test contains tests for the schema loader.
// These tests are written TDD-style: tests first, implementation later.
package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/tobias/vibefeld/internal/schema"
)

// TestSchema_DefaultValues tests that DefaultSchema returns a schema with sensible defaults.
func TestSchema_DefaultValues(t *testing.T) {
	s := schema.DefaultSchema()

	if s == nil {
		t.Fatal("DefaultSchema() returned nil")
	}

	// Test that default schema has expected properties
	tests := []struct {
		name string
		check func() bool
		errMsg string
	}{
		{
			name: "has inference types",
			check: func() bool { return len(s.InferenceTypes) > 0 },
			errMsg: "default schema should have inference types",
		},
		{
			name: "has node types",
			check: func() bool { return len(s.NodeTypes) > 0 },
			errMsg: "default schema should have node types",
		},
		{
			name: "has challenge targets",
			check: func() bool { return len(s.ChallengeTargets) > 0 },
			errMsg: "default schema should have challenge targets",
		},
		{
			name: "has workflow states",
			check: func() bool { return len(s.WorkflowStates) > 0 },
			errMsg: "default schema should have workflow states",
		},
		{
			name: "has epistemic states",
			check: func() bool { return len(s.EpistemicStates) > 0 },
			errMsg: "default schema should have epistemic states",
		},
		{
			name: "version is set",
			check: func() bool { return s.Version != "" },
			errMsg: "default schema should have a version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check() {
				t.Error(tt.errMsg)
			}
		})
	}
}

// TestSchema_LoadFromJSON tests loading schema configuration from JSON data.
func TestSchema_LoadFromJSON(t *testing.T) {
	tests := []struct {
		name      string
		jsonData  string
		wantErr   bool
		errSubstr string
		validate  func(t *testing.T, s *schema.Schema)
	}{
		{
			name: "valid minimal schema",
			jsonData: `{
				"version": "1.0",
				"inference_types": ["modus_ponens"],
				"node_types": ["claim"],
				"challenge_targets": ["statement"],
				"workflow_states": ["available"],
				"epistemic_states": ["pending"]
			}`,
			wantErr: false,
			validate: func(t *testing.T, s *schema.Schema) {
				if s.Version != "1.0" {
					t.Errorf("Version = %q, want %q", s.Version, "1.0")
				}
			},
		},
		{
			name: "valid full schema",
			jsonData: `{
				"version": "1.0",
				"inference_types": [
					"modus_ponens", "modus_tollens", "universal_instantiation",
					"existential_instantiation", "universal_generalization",
					"existential_generalization", "by_definition", "assumption",
					"local_assume", "local_discharge", "contradiction"
				],
				"node_types": ["claim", "local_assume", "local_discharge", "case", "qed"],
				"challenge_targets": [
					"statement", "inference", "context", "dependencies",
					"scope", "gap", "type_error", "domain", "completeness"
				],
				"workflow_states": ["available", "claimed", "blocked"],
				"epistemic_states": ["pending", "validated", "admitted", "refuted", "archived"]
			}`,
			wantErr: false,
			validate: func(t *testing.T, s *schema.Schema) {
				if len(s.InferenceTypes) != 11 {
					t.Errorf("len(InferenceTypes) = %d, want 11", len(s.InferenceTypes))
				}
				if len(s.NodeTypes) != 5 {
					t.Errorf("len(NodeTypes) = %d, want 5", len(s.NodeTypes))
				}
				if len(s.ChallengeTargets) != 9 {
					t.Errorf("len(ChallengeTargets) = %d, want 9", len(s.ChallengeTargets))
				}
				if len(s.WorkflowStates) != 3 {
					t.Errorf("len(WorkflowStates) = %d, want 3", len(s.WorkflowStates))
				}
				if len(s.EpistemicStates) != 5 {
					t.Errorf("len(EpistemicStates) = %d, want 5", len(s.EpistemicStates))
				}
			},
		},
		{
			name:      "empty JSON",
			jsonData:  `{}`,
			wantErr:   true,
			errSubstr: "version",
		},
		{
			name:      "invalid JSON",
			jsonData:  `{invalid json`,
			wantErr:   true,
			errSubstr: "invalid",
		},
		{
			name: "invalid inference type",
			jsonData: `{
				"version": "1.0",
				"inference_types": ["invalid_inference"],
				"node_types": ["claim"],
				"challenge_targets": ["statement"],
				"workflow_states": ["available"],
				"epistemic_states": ["pending"]
			}`,
			wantErr:   true,
			errSubstr: "inference",
		},
		{
			name: "invalid node type",
			jsonData: `{
				"version": "1.0",
				"inference_types": ["modus_ponens"],
				"node_types": ["invalid_node"],
				"challenge_targets": ["statement"],
				"workflow_states": ["available"],
				"epistemic_states": ["pending"]
			}`,
			wantErr:   true,
			errSubstr: "node",
		},
		{
			name: "invalid challenge target",
			jsonData: `{
				"version": "1.0",
				"inference_types": ["modus_ponens"],
				"node_types": ["claim"],
				"challenge_targets": ["invalid_target"],
				"workflow_states": ["available"],
				"epistemic_states": ["pending"]
			}`,
			wantErr:   true,
			errSubstr: "challenge",
		},
		{
			name: "invalid workflow state",
			jsonData: `{
				"version": "1.0",
				"inference_types": ["modus_ponens"],
				"node_types": ["claim"],
				"challenge_targets": ["statement"],
				"workflow_states": ["invalid_state"],
				"epistemic_states": ["pending"]
			}`,
			wantErr:   true,
			errSubstr: "workflow",
		},
		{
			name: "invalid epistemic state",
			jsonData: `{
				"version": "1.0",
				"inference_types": ["modus_ponens"],
				"node_types": ["claim"],
				"challenge_targets": ["statement"],
				"workflow_states": ["available"],
				"epistemic_states": ["invalid_state"]
			}`,
			wantErr:   true,
			errSubstr: "epistemic",
		},
		{
			name: "missing version",
			jsonData: `{
				"inference_types": ["modus_ponens"],
				"node_types": ["claim"],
				"challenge_targets": ["statement"],
				"workflow_states": ["available"],
				"epistemic_states": ["pending"]
			}`,
			wantErr:   true,
			errSubstr: "version",
		},
		{
			name: "empty arrays",
			jsonData: `{
				"version": "1.0",
				"inference_types": [],
				"node_types": [],
				"challenge_targets": [],
				"workflow_states": [],
				"epistemic_states": []
			}`,
			wantErr:   true,
			errSubstr: "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := schema.LoadSchema([]byte(tt.jsonData))

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadSchema() error = nil, want error containing %q", tt.errSubstr)
					return
				}
				if tt.errSubstr != "" && !containsIgnoreCase(err.Error(), tt.errSubstr) {
					t.Errorf("LoadSchema() error = %v, want error containing %q", err, tt.errSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("LoadSchema() unexpected error: %v", err)
			}

			if s == nil {
				t.Fatal("LoadSchema() returned nil schema without error")
			}

			if tt.validate != nil {
				tt.validate(t, s)
			}
		})
	}
}

// TestSchema_ValidateAllEnums tests that all enum values in a schema are valid.
func TestSchema_ValidateAllEnums(t *testing.T) {
	s := schema.DefaultSchema()

	t.Run("all inference types valid", func(t *testing.T) {
		for _, it := range s.InferenceTypes {
			if err := schema.ValidateInference(string(it)); err != nil {
				t.Errorf("invalid inference type in default schema: %s", it)
			}
		}
	})

	t.Run("all node types valid", func(t *testing.T) {
		for _, nt := range s.NodeTypes {
			if err := schema.ValidateNodeType(string(nt)); err != nil {
				t.Errorf("invalid node type in default schema: %s", nt)
			}
		}
	})

	t.Run("all challenge targets valid", func(t *testing.T) {
		for _, ct := range s.ChallengeTargets {
			if err := schema.ValidateChallengeTarget(string(ct)); err != nil {
				t.Errorf("invalid challenge target in default schema: %s", ct)
			}
		}
	})

	t.Run("all workflow states valid", func(t *testing.T) {
		for _, ws := range s.WorkflowStates {
			if err := schema.ValidateWorkflowState(string(ws)); err != nil {
				t.Errorf("invalid workflow state in default schema: %s", ws)
			}
		}
	})

	t.Run("all epistemic states valid", func(t *testing.T) {
		for _, es := range s.EpistemicStates {
			if err := schema.ValidateEpistemicState(string(es)); err != nil {
				t.Errorf("invalid epistemic state in default schema: %s", es)
			}
		}
	})
}

// TestSchema_AllNodeTypes verifies all known node types exist in the default schema.
func TestSchema_AllNodeTypes(t *testing.T) {
	s := schema.DefaultSchema()

	expectedNodeTypes := []schema.NodeType{
		schema.NodeTypeClaim,
		schema.NodeTypeLocalAssume,
		schema.NodeTypeLocalDischarge,
		schema.NodeTypeCase,
		schema.NodeTypeQED,
	}

	nodeTypeSet := make(map[schema.NodeType]bool)
	for _, nt := range s.NodeTypes {
		nodeTypeSet[nt] = true
	}

	for _, expected := range expectedNodeTypes {
		t.Run(string(expected), func(t *testing.T) {
			if !nodeTypeSet[expected] {
				t.Errorf("node type %q not found in default schema", expected)
			}
		})
	}

	// Verify count matches
	if len(s.NodeTypes) != len(expectedNodeTypes) {
		t.Errorf("NodeTypes count = %d, want %d", len(s.NodeTypes), len(expectedNodeTypes))
	}
}

// TestSchema_AllInferenceTypes verifies all known inference types exist in the default schema.
func TestSchema_AllInferenceTypes(t *testing.T) {
	s := schema.DefaultSchema()

	expectedInferenceTypes := []schema.InferenceType{
		schema.InferenceModusPonens,
		schema.InferenceModusTollens,
		schema.InferenceUniversalInstantiation,
		schema.InferenceExistentialInstantiation,
		schema.InferenceUniversalGeneralization,
		schema.InferenceExistentialGeneralization,
		schema.InferenceByDefinition,
		schema.InferenceAssumption,
		schema.InferenceLocalAssume,
		schema.InferenceLocalDischarge,
		schema.InferenceContradiction,
	}

	inferenceTypeSet := make(map[schema.InferenceType]bool)
	for _, it := range s.InferenceTypes {
		inferenceTypeSet[it] = true
	}

	for _, expected := range expectedInferenceTypes {
		t.Run(string(expected), func(t *testing.T) {
			if !inferenceTypeSet[expected] {
				t.Errorf("inference type %q not found in default schema", expected)
			}
		})
	}

	// Verify count matches
	if len(s.InferenceTypes) != len(expectedInferenceTypes) {
		t.Errorf("InferenceTypes count = %d, want %d", len(s.InferenceTypes), len(expectedInferenceTypes))
	}
}

// TestSchema_AllChallengeTargets verifies all known challenge targets exist in the default schema.
func TestSchema_AllChallengeTargets(t *testing.T) {
	s := schema.DefaultSchema()

	expectedChallengeTargets := []schema.ChallengeTarget{
		schema.TargetStatement,
		schema.TargetInference,
		schema.TargetContext,
		schema.TargetDependencies,
		schema.TargetScope,
		schema.TargetGap,
		schema.TargetTypeError,
		schema.TargetDomain,
		schema.TargetCompleteness,
	}

	challengeTargetSet := make(map[schema.ChallengeTarget]bool)
	for _, ct := range s.ChallengeTargets {
		challengeTargetSet[ct] = true
	}

	for _, expected := range expectedChallengeTargets {
		t.Run(string(expected), func(t *testing.T) {
			if !challengeTargetSet[expected] {
				t.Errorf("challenge target %q not found in default schema", expected)
			}
		})
	}

	// Verify count matches
	if len(s.ChallengeTargets) != len(expectedChallengeTargets) {
		t.Errorf("ChallengeTargets count = %d, want %d", len(s.ChallengeTargets), len(expectedChallengeTargets))
	}
}

// TestSchema_AllWorkflowStates verifies all known workflow states exist in the default schema.
func TestSchema_AllWorkflowStates(t *testing.T) {
	s := schema.DefaultSchema()

	expectedWorkflowStates := []schema.WorkflowState{
		schema.WorkflowAvailable,
		schema.WorkflowClaimed,
		schema.WorkflowBlocked,
	}

	workflowStateSet := make(map[schema.WorkflowState]bool)
	for _, ws := range s.WorkflowStates {
		workflowStateSet[ws] = true
	}

	for _, expected := range expectedWorkflowStates {
		t.Run(string(expected), func(t *testing.T) {
			if !workflowStateSet[expected] {
				t.Errorf("workflow state %q not found in default schema", expected)
			}
		})
	}

	// Verify count matches
	if len(s.WorkflowStates) != len(expectedWorkflowStates) {
		t.Errorf("WorkflowStates count = %d, want %d", len(s.WorkflowStates), len(expectedWorkflowStates))
	}
}

// TestSchema_AllEpistemicStates verifies all known epistemic states exist in the default schema.
func TestSchema_AllEpistemicStates(t *testing.T) {
	s := schema.DefaultSchema()

	expectedEpistemicStates := []schema.EpistemicState{
		schema.EpistemicPending,
		schema.EpistemicValidated,
		schema.EpistemicAdmitted,
		schema.EpistemicRefuted,
		schema.EpistemicArchived,
	}

	epistemicStateSet := make(map[schema.EpistemicState]bool)
	for _, es := range s.EpistemicStates {
		epistemicStateSet[es] = true
	}

	for _, expected := range expectedEpistemicStates {
		t.Run(string(expected), func(t *testing.T) {
			if !epistemicStateSet[expected] {
				t.Errorf("epistemic state %q not found in default schema", expected)
			}
		})
	}

	// Verify count matches
	if len(s.EpistemicStates) != len(expectedEpistemicStates) {
		t.Errorf("EpistemicStates count = %d, want %d", len(s.EpistemicStates), len(expectedEpistemicStates))
	}
}

// TestSchema_ToJSON tests that a schema can be serialized to JSON and back.
func TestSchema_ToJSON(t *testing.T) {
	s := schema.DefaultSchema()

	// Serialize to JSON
	data, err := s.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	// Parse the JSON to verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON output is invalid: %v", err)
	}

	// Verify expected keys exist
	expectedKeys := []string{"version", "inference_types", "node_types", "challenge_targets", "workflow_states", "epistemic_states"}
	for _, key := range expectedKeys {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON output missing key %q", key)
		}
	}

	// Round-trip test: load the serialized JSON
	s2, err := schema.LoadSchema(data)
	if err != nil {
		t.Fatalf("LoadSchema() error on round-trip: %v", err)
	}

	// Verify the loaded schema matches the original
	if s2.Version != s.Version {
		t.Errorf("round-trip Version = %q, want %q", s2.Version, s.Version)
	}
	if len(s2.InferenceTypes) != len(s.InferenceTypes) {
		t.Errorf("round-trip InferenceTypes count = %d, want %d", len(s2.InferenceTypes), len(s.InferenceTypes))
	}
	if len(s2.NodeTypes) != len(s.NodeTypes) {
		t.Errorf("round-trip NodeTypes count = %d, want %d", len(s2.NodeTypes), len(s.NodeTypes))
	}
	if len(s2.ChallengeTargets) != len(s.ChallengeTargets) {
		t.Errorf("round-trip ChallengeTargets count = %d, want %d", len(s2.ChallengeTargets), len(s.ChallengeTargets))
	}
	if len(s2.WorkflowStates) != len(s.WorkflowStates) {
		t.Errorf("round-trip WorkflowStates count = %d, want %d", len(s2.WorkflowStates), len(s.WorkflowStates))
	}
	if len(s2.EpistemicStates) != len(s.EpistemicStates) {
		t.Errorf("round-trip EpistemicStates count = %d, want %d", len(s2.EpistemicStates), len(s.EpistemicStates))
	}
}

// TestSchema_Validate tests the schema validation method.
func TestSchema_Validate(t *testing.T) {
	tests := []struct {
		name    string
		schema  *schema.Schema
		wantErr bool
	}{
		{
			name:    "default schema is valid",
			schema:  schema.DefaultSchema(),
			wantErr: false,
		},
		{
			name: "schema with all fields populated",
			schema: &schema.Schema{
				Version:          "1.0",
				InferenceTypes:   []schema.InferenceType{schema.InferenceModusPonens},
				NodeTypes:        []schema.NodeType{schema.NodeTypeClaim},
				ChallengeTargets: []schema.ChallengeTarget{schema.TargetStatement},
				WorkflowStates:   []schema.WorkflowState{schema.WorkflowAvailable},
				EpistemicStates:  []schema.EpistemicState{schema.EpistemicPending},
			},
			wantErr: false,
		},
		{
			name: "schema with empty version",
			schema: &schema.Schema{
				Version:          "",
				InferenceTypes:   []schema.InferenceType{schema.InferenceModusPonens},
				NodeTypes:        []schema.NodeType{schema.NodeTypeClaim},
				ChallengeTargets: []schema.ChallengeTarget{schema.TargetStatement},
				WorkflowStates:   []schema.WorkflowState{schema.WorkflowAvailable},
				EpistemicStates:  []schema.EpistemicState{schema.EpistemicPending},
			},
			wantErr: true,
		},
		{
			name: "schema with empty inference types",
			schema: &schema.Schema{
				Version:          "1.0",
				InferenceTypes:   []schema.InferenceType{},
				NodeTypes:        []schema.NodeType{schema.NodeTypeClaim},
				ChallengeTargets: []schema.ChallengeTarget{schema.TargetStatement},
				WorkflowStates:   []schema.WorkflowState{schema.WorkflowAvailable},
				EpistemicStates:  []schema.EpistemicState{schema.EpistemicPending},
			},
			wantErr: true,
		},
		{
			name: "schema with empty node types",
			schema: &schema.Schema{
				Version:          "1.0",
				InferenceTypes:   []schema.InferenceType{schema.InferenceModusPonens},
				NodeTypes:        []schema.NodeType{},
				ChallengeTargets: []schema.ChallengeTarget{schema.TargetStatement},
				WorkflowStates:   []schema.WorkflowState{schema.WorkflowAvailable},
				EpistemicStates:  []schema.EpistemicState{schema.EpistemicPending},
			},
			wantErr: true,
		},
		{
			name: "schema with invalid inference type",
			schema: &schema.Schema{
				Version:          "1.0",
				InferenceTypes:   []schema.InferenceType{"invalid"},
				NodeTypes:        []schema.NodeType{schema.NodeTypeClaim},
				ChallengeTargets: []schema.ChallengeTarget{schema.TargetStatement},
				WorkflowStates:   []schema.WorkflowState{schema.WorkflowAvailable},
				EpistemicStates:  []schema.EpistemicState{schema.EpistemicPending},
			},
			wantErr: true,
		},
		{
			name: "schema with invalid node type",
			schema: &schema.Schema{
				Version:          "1.0",
				InferenceTypes:   []schema.InferenceType{schema.InferenceModusPonens},
				NodeTypes:        []schema.NodeType{"invalid"},
				ChallengeTargets: []schema.ChallengeTarget{schema.TargetStatement},
				WorkflowStates:   []schema.WorkflowState{schema.WorkflowAvailable},
				EpistemicStates:  []schema.EpistemicState{schema.EpistemicPending},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSchema_HasInferenceType tests checking if a schema contains a specific inference type.
func TestSchema_HasInferenceType(t *testing.T) {
	s := schema.DefaultSchema()

	tests := []struct {
		inferenceType schema.InferenceType
		want          bool
	}{
		{schema.InferenceModusPonens, true},
		{schema.InferenceModusTollens, true},
		{schema.InferenceContradiction, true},
		{"nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.inferenceType), func(t *testing.T) {
			if got := s.HasInferenceType(tt.inferenceType); got != tt.want {
				t.Errorf("HasInferenceType(%q) = %v, want %v", tt.inferenceType, got, tt.want)
			}
		})
	}
}

// TestSchema_HasNodeType tests checking if a schema contains a specific node type.
func TestSchema_HasNodeType(t *testing.T) {
	s := schema.DefaultSchema()

	tests := []struct {
		nodeType schema.NodeType
		want     bool
	}{
		{schema.NodeTypeClaim, true},
		{schema.NodeTypeQED, true},
		{"nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.nodeType), func(t *testing.T) {
			if got := s.HasNodeType(tt.nodeType); got != tt.want {
				t.Errorf("HasNodeType(%q) = %v, want %v", tt.nodeType, got, tt.want)
			}
		})
	}
}

// TestSchema_HasChallengeTarget tests checking if a schema contains a specific challenge target.
func TestSchema_HasChallengeTarget(t *testing.T) {
	s := schema.DefaultSchema()

	tests := []struct {
		target schema.ChallengeTarget
		want   bool
	}{
		{schema.TargetStatement, true},
		{schema.TargetGap, true},
		{"nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.target), func(t *testing.T) {
			if got := s.HasChallengeTarget(tt.target); got != tt.want {
				t.Errorf("HasChallengeTarget(%q) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

// TestSchema_HasWorkflowState tests checking if a schema contains a specific workflow state.
func TestSchema_HasWorkflowState(t *testing.T) {
	s := schema.DefaultSchema()

	tests := []struct {
		state schema.WorkflowState
		want  bool
	}{
		{schema.WorkflowAvailable, true},
		{schema.WorkflowClaimed, true},
		{schema.WorkflowBlocked, true},
		{"nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := s.HasWorkflowState(tt.state); got != tt.want {
				t.Errorf("HasWorkflowState(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// TestSchema_HasEpistemicState tests checking if a schema contains a specific epistemic state.
func TestSchema_HasEpistemicState(t *testing.T) {
	s := schema.DefaultSchema()

	tests := []struct {
		state schema.EpistemicState
		want  bool
	}{
		{schema.EpistemicPending, true},
		{schema.EpistemicValidated, true},
		{schema.EpistemicAdmitted, true},
		{schema.EpistemicRefuted, true},
		{schema.EpistemicArchived, true},
		{"nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := s.HasEpistemicState(tt.state); got != tt.want {
				t.Errorf("HasEpistemicState(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// TestSchema_Clone tests that cloning a schema creates an independent copy.
func TestSchema_Clone(t *testing.T) {
	original := schema.DefaultSchema()
	cloned := original.Clone()

	// Verify the clone is not nil
	if cloned == nil {
		t.Fatal("Clone() returned nil")
	}

	// Verify basic equality
	if cloned.Version != original.Version {
		t.Errorf("Clone().Version = %q, want %q", cloned.Version, original.Version)
	}
	if len(cloned.InferenceTypes) != len(original.InferenceTypes) {
		t.Errorf("Clone().InferenceTypes length = %d, want %d", len(cloned.InferenceTypes), len(original.InferenceTypes))
	}

	// Verify independence: modifications to clone don't affect original
	originalInferenceCount := len(original.InferenceTypes)
	cloned.InferenceTypes = cloned.InferenceTypes[:1] // truncate
	if len(original.InferenceTypes) != originalInferenceCount {
		t.Error("modifying clone affected original schema")
	}
}

// containsIgnoreCase checks if substr is contained in s (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	sLower := make([]byte, len(s))
	substrLower := make([]byte, len(substr))
	for i := range s {
		if s[i] >= 'A' && s[i] <= 'Z' {
			sLower[i] = s[i] + 32
		} else {
			sLower[i] = s[i]
		}
	}
	for i := range substr {
		if substr[i] >= 'A' && substr[i] <= 'Z' {
			substrLower[i] = substr[i] + 32
		} else {
			substrLower[i] = substr[i]
		}
	}
	return contains(string(sLower), string(substrLower))
}

// contains checks if substr is contained in s.
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
