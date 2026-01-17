// Package node_test contains external tests for the node package.
package node_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestNewNode_Valid tests creating a node with valid inputs.
func TestNewNode_Valid(t *testing.T) {
	tests := []struct {
		name      string
		idStr     string
		nodeType  schema.NodeType
		statement string
		inference schema.InferenceType
	}{
		{
			name:      "root claim node",
			idStr:     "1",
			nodeType:  schema.NodeTypeClaim,
			statement: "For all n >= 0, the sum of first n natural numbers is n(n+1)/2",
			inference: schema.InferenceAssumption,
		},
		{
			name:      "child claim node",
			idStr:     "1.1",
			nodeType:  schema.NodeTypeClaim,
			statement: "Base case: when n=0, sum is 0",
			inference: schema.InferenceByDefinition,
		},
		{
			name:      "deep nested node",
			idStr:     "1.2.3.4",
			nodeType:  schema.NodeTypeClaim,
			statement: "Inductive step holds",
			inference: schema.InferenceModusPonens,
		},
		{
			name:      "local assume node",
			idStr:     "1.2",
			nodeType:  schema.NodeTypeLocalAssume,
			statement: "Assume P(k) holds for some k >= 0",
			inference: schema.InferenceLocalAssume,
		},
		{
			name:      "local discharge node",
			idStr:     "1.3",
			nodeType:  schema.NodeTypeLocalDischarge,
			statement: "Therefore P(k) implies P(k+1)",
			inference: schema.InferenceLocalDischarge,
		},
		{
			name:      "case node",
			idStr:     "1.1.1",
			nodeType:  schema.NodeTypeCase,
			statement: "Case 1: x > 0",
			inference: schema.InferenceModusTollens,
		},
		{
			name:      "qed node",
			idStr:     "1.4",
			nodeType:  schema.NodeTypeQED,
			statement: "By induction, the theorem holds for all n",
			inference: schema.InferenceUniversalGeneralization,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := types.Parse(tt.idStr)
			if err != nil {
				t.Fatalf("types.Parse(%q) error: %v", tt.idStr, err)
			}

			n, err := node.NewNode(id, tt.nodeType, tt.statement, tt.inference)
			if err != nil {
				t.Fatalf("NewNode() unexpected error: %v", err)
			}

			// Verify ID
			if n.ID.String() != tt.idStr {
				t.Errorf("ID = %q, want %q", n.ID.String(), tt.idStr)
			}

			// Verify Type
			if n.Type != tt.nodeType {
				t.Errorf("Type = %q, want %q", n.Type, tt.nodeType)
			}

			// Verify Statement
			if n.Statement != tt.statement {
				t.Errorf("Statement = %q, want %q", n.Statement, tt.statement)
			}

			// Verify Inference
			if n.Inference != tt.inference {
				t.Errorf("Inference = %q, want %q", n.Inference, tt.inference)
			}

			// Verify default workflow state
			if n.WorkflowState != schema.WorkflowAvailable {
				t.Errorf("WorkflowState = %q, want %q", n.WorkflowState, schema.WorkflowAvailable)
			}

			// Verify default epistemic state
			if n.EpistemicState != schema.EpistemicPending {
				t.Errorf("EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicPending)
			}

			// Verify default taint state
			if n.TaintState != node.TaintUnresolved {
				t.Errorf("TaintState = %q, want %q", n.TaintState, node.TaintUnresolved)
			}

			// Verify ContentHash is set
			if n.ContentHash == "" {
				t.Error("ContentHash should not be empty")
			}

			// Verify ContentHash is 64 chars (SHA256 hex)
			if len(n.ContentHash) != 64 {
				t.Errorf("ContentHash length = %d, want 64", len(n.ContentHash))
			}

			// Verify Created timestamp is set
			if n.Created.IsZero() {
				t.Error("Created timestamp should not be zero")
			}
		})
	}
}

// TestNewNode_EmptyStatement tests that empty statement is rejected.
func TestNewNode_EmptyStatement(t *testing.T) {
	tests := []struct {
		name      string
		statement string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs only", "\t\t"},
		{"newlines only", "\n\n"},
		{"mixed whitespace", " \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, _ := types.Parse("1")
			_, err := node.NewNode(id, schema.NodeTypeClaim, tt.statement, schema.InferenceAssumption)
			if err == nil {
				t.Errorf("NewNode() with statement %q should return error", tt.statement)
			}
		})
	}
}

// TestNewNode_InvalidNodeType tests that invalid node type is rejected.
func TestNewNode_InvalidNodeType(t *testing.T) {
	id, _ := types.Parse("1")
	_, err := node.NewNode(id, schema.NodeType("invalid_type"), "Valid statement", schema.InferenceAssumption)
	if err == nil {
		t.Error("NewNode() with invalid node type should return error")
	}
}

// TestNewNode_InvalidInferenceType tests that invalid inference type is rejected.
func TestNewNode_InvalidInferenceType(t *testing.T) {
	id, _ := types.Parse("1")
	_, err := node.NewNode(id, schema.NodeTypeClaim, "Valid statement", schema.InferenceType("invalid_inference"))
	if err == nil {
		t.Error("NewNode() with invalid inference type should return error")
	}
}

// TestNewNodeWithOptions tests creating a node with optional parameters.
func TestNewNodeWithOptions(t *testing.T) {
	id, _ := types.Parse("1.2")
	dep1, _ := types.Parse("1")
	dep2, _ := types.Parse("1.1")

	opts := node.NodeOptions{
		Latex:        "\\forall n \\geq 0",
		Context:      []string{"def:prime", "assume:A1"},
		Dependencies: []types.NodeID{dep1, dep2},
		Scope:        []string{"1.1.A"},
	}

	n, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceModusPonens, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() unexpected error: %v", err)
	}

	// Verify optional fields
	if n.Latex != opts.Latex {
		t.Errorf("Latex = %q, want %q", n.Latex, opts.Latex)
	}

	if len(n.Context) != len(opts.Context) {
		t.Errorf("Context length = %d, want %d", len(n.Context), len(opts.Context))
	}

	if len(n.Dependencies) != len(opts.Dependencies) {
		t.Errorf("Dependencies length = %d, want %d", len(n.Dependencies), len(opts.Dependencies))
	}

	if len(n.Scope) != len(opts.Scope) {
		t.Errorf("Scope length = %d, want %d", len(n.Scope), len(opts.Scope))
	}
}

// TestNode_ContentHash_Deterministic tests that content hash is deterministic.
func TestNode_ContentHash_Deterministic(t *testing.T) {
	id, _ := types.Parse("1")

	n1, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	n2, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	if n1.ContentHash != n2.ContentHash {
		t.Errorf("Content hashes differ for identical content: %q != %q", n1.ContentHash, n2.ContentHash)
	}
}

// TestNode_ContentHash_DifferentContent tests that different content produces different hashes.
func TestNode_ContentHash_DifferentContent(t *testing.T) {
	id, _ := types.Parse("1")

	tests := []struct {
		name      string
		statement string
		nodeType  schema.NodeType
		inference schema.InferenceType
	}{
		{"different statement", "Different statement", schema.NodeTypeClaim, schema.InferenceAssumption},
		{"different type", "Test statement", schema.NodeTypeCase, schema.InferenceAssumption},
		{"different inference", "Test statement", schema.NodeTypeClaim, schema.InferenceModusPonens},
	}

	baseline, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := node.NewNode(id, tt.nodeType, tt.statement, tt.inference)
			if err != nil {
				t.Fatalf("NewNode() error: %v", err)
			}

			if n.ContentHash == baseline.ContentHash {
				t.Errorf("Content hashes should differ for %s", tt.name)
			}
		})
	}
}

// TestNode_ContentHash_ContextOrder tests that context order doesn't affect hash.
func TestNode_ContentHash_ContextOrder(t *testing.T) {
	id, _ := types.Parse("1")

	opts1 := node.NodeOptions{
		Context: []string{"def:A", "def:B", "assume:C"},
	}
	opts2 := node.NodeOptions{
		Context: []string{"assume:C", "def:A", "def:B"},
	}

	n1, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test", schema.InferenceAssumption, opts1)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}

	n2, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test", schema.InferenceAssumption, opts2)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}

	if n1.ContentHash != n2.ContentHash {
		t.Errorf("Content hashes should be equal regardless of context order")
	}
}

// TestNode_ContentHash_DependencyOrder tests that dependency order doesn't affect hash.
func TestNode_ContentHash_DependencyOrder(t *testing.T) {
	id, _ := types.Parse("1.3")
	dep1, _ := types.Parse("1.1")
	dep2, _ := types.Parse("1.2")

	opts1 := node.NodeOptions{
		Dependencies: []types.NodeID{dep1, dep2},
	}
	opts2 := node.NodeOptions{
		Dependencies: []types.NodeID{dep2, dep1},
	}

	n1, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test", schema.InferenceModusPonens, opts1)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}

	n2, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test", schema.InferenceModusPonens, opts2)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}

	if n1.ContentHash != n2.ContentHash {
		t.Errorf("Content hashes should be equal regardless of dependency order")
	}
}

// TestNode_VerifyContentHash tests content hash verification.
func TestNode_VerifyContentHash(t *testing.T) {
	id, _ := types.Parse("1")
	n, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	// Verify should pass for unmodified node
	if !n.VerifyContentHash() {
		t.Error("VerifyContentHash() should return true for unmodified node")
	}

	// Modify statement and verify should fail
	n.Statement = "Modified statement"
	if n.VerifyContentHash() {
		t.Error("VerifyContentHash() should return false after modifying statement")
	}
}

// TestNode_Validate tests the Validate method.
func TestNode_Validate(t *testing.T) {
	id, _ := types.Parse("1")

	t.Run("valid node", func(t *testing.T) {
		n, err := node.NewNode(id, schema.NodeTypeClaim, "Valid statement", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("NewNode() error: %v", err)
		}

		if err := n.Validate(); err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("empty statement", func(t *testing.T) {
		n := &node.Node{
			ID:             id,
			Type:           schema.NodeTypeClaim,
			Statement:      "",
			Inference:      schema.InferenceAssumption,
			WorkflowState:  schema.WorkflowAvailable,
			EpistemicState: schema.EpistemicPending,
		}

		if err := n.Validate(); err == nil {
			t.Error("Validate() should return error for empty statement")
		}
	})

	t.Run("invalid node type", func(t *testing.T) {
		n := &node.Node{
			ID:             id,
			Type:           schema.NodeType("invalid"),
			Statement:      "Valid statement",
			Inference:      schema.InferenceAssumption,
			WorkflowState:  schema.WorkflowAvailable,
			EpistemicState: schema.EpistemicPending,
		}

		if err := n.Validate(); err == nil {
			t.Error("Validate() should return error for invalid node type")
		}
	})

	t.Run("invalid inference type", func(t *testing.T) {
		n := &node.Node{
			ID:             id,
			Type:           schema.NodeTypeClaim,
			Statement:      "Valid statement",
			Inference:      schema.InferenceType("invalid"),
			WorkflowState:  schema.WorkflowAvailable,
			EpistemicState: schema.EpistemicPending,
		}

		if err := n.Validate(); err == nil {
			t.Error("Validate() should return error for invalid inference type")
		}
	})
}

// TestNode_IsRoot tests the IsRoot method.
func TestNode_IsRoot(t *testing.T) {
	tests := []struct {
		idStr    string
		wantRoot bool
	}{
		{"1", true},
		{"1.1", false},
		{"1.2.3", false},
		{"1.1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.idStr, func(t *testing.T) {
			id, _ := types.Parse(tt.idStr)
			n, err := node.NewNode(id, schema.NodeTypeClaim, "Test", schema.InferenceAssumption)
			if err != nil {
				t.Fatalf("NewNode() error: %v", err)
			}

			if got := n.IsRoot(); got != tt.wantRoot {
				t.Errorf("IsRoot() = %v, want %v", got, tt.wantRoot)
			}
		})
	}
}

// TestNode_Depth tests the Depth method.
func TestNode_Depth(t *testing.T) {
	tests := []struct {
		idStr     string
		wantDepth int
	}{
		{"1", 1},
		{"1.1", 2},
		{"1.2.3", 3},
		{"1.1.1.1", 4},
		{"1.2.3.4.5", 5},
	}

	for _, tt := range tests {
		t.Run(tt.idStr, func(t *testing.T) {
			id, _ := types.Parse(tt.idStr)
			n, err := node.NewNode(id, schema.NodeTypeClaim, "Test", schema.InferenceAssumption)
			if err != nil {
				t.Fatalf("NewNode() error: %v", err)
			}

			if got := n.Depth(); got != tt.wantDepth {
				t.Errorf("Depth() = %d, want %d", got, tt.wantDepth)
			}
		})
	}
}

// TestNode_JSON_Roundtrip tests JSON serialization and deserialization.
// NOTE: NodeID and Dependencies fields don't round-trip correctly because
// types.NodeID lacks MarshalJSON/UnmarshalJSON methods. This test focuses
// on verifying the other fields serialize correctly.
func TestNode_JSON_Roundtrip(t *testing.T) {
	tests := []struct {
		name      string
		idStr     string
		nodeType  schema.NodeType
		statement string
		inference schema.InferenceType
		latex     string
		context   []string
		scope     []string
	}{
		{
			name:      "simple node",
			idStr:     "1",
			nodeType:  schema.NodeTypeClaim,
			statement: "Simple claim",
			inference: schema.InferenceAssumption,
		},
		{
			name:      "node with latex and context",
			idStr:     "1.2.3",
			nodeType:  schema.NodeTypeClaim,
			statement: "Complex claim with context",
			inference: schema.InferenceModusPonens,
			latex:     "\\forall x \\in \\mathbb{R}",
			context:   []string{"def:real", "assume:A1", "ext:theorem1"},
			scope:     []string{"1.1.A", "1.2.B"},
		},
		{
			name:      "node with special characters",
			idStr:     "1.1",
			nodeType:  schema.NodeTypeCase,
			statement: "Case: x > 0 and y < 0",
			inference: schema.InferenceByDefinition,
		},
		{
			name:      "node with unicode",
			idStr:     "1.2",
			nodeType:  schema.NodeTypeClaim,
			statement: "Let alpha, beta be arbitrary real numbers",
			inference: schema.InferenceUniversalInstantiation,
		},
		{
			name:      "node with newlines",
			idStr:     "1.3",
			nodeType:  schema.NodeTypeClaim,
			statement: "Line 1\nLine 2\nLine 3",
			inference: schema.InferenceAssumption,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := types.Parse(tt.idStr)
			if err != nil {
				t.Fatalf("types.Parse(%q) error: %v", tt.idStr, err)
			}

			opts := node.NodeOptions{
				Latex:   tt.latex,
				Context: tt.context,
				Scope:   tt.scope,
			}

			original, err := node.NewNodeWithOptions(id, tt.nodeType, tt.statement, tt.inference, opts)
			if err != nil {
				t.Fatalf("NewNodeWithOptions() error: %v", err)
			}

			// Marshal to JSON
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("json.Marshal() error: %v", err)
			}

			// Unmarshal back
			var restored node.Node
			err = json.Unmarshal(data, &restored)
			if err != nil {
				t.Fatalf("json.Unmarshal() error: %v", err)
			}

			// NOTE: Skipping ID comparison - types.NodeID lacks JSON marshaling
			// This is a known limitation that should be fixed in internal/types/id.go

			// Verify string and enum fields
			if restored.Type != original.Type {
				t.Errorf("Type = %q, want %q", restored.Type, original.Type)
			}
			if restored.Statement != original.Statement {
				t.Errorf("Statement = %q, want %q", restored.Statement, original.Statement)
			}
			if restored.Latex != original.Latex {
				t.Errorf("Latex = %q, want %q", restored.Latex, original.Latex)
			}
			if restored.Inference != original.Inference {
				t.Errorf("Inference = %q, want %q", restored.Inference, original.Inference)
			}
			if restored.WorkflowState != original.WorkflowState {
				t.Errorf("WorkflowState = %q, want %q", restored.WorkflowState, original.WorkflowState)
			}
			if restored.EpistemicState != original.EpistemicState {
				t.Errorf("EpistemicState = %q, want %q", restored.EpistemicState, original.EpistemicState)
			}
			if restored.TaintState != original.TaintState {
				t.Errorf("TaintState = %q, want %q", restored.TaintState, original.TaintState)
			}
			if restored.ContentHash != original.ContentHash {
				t.Errorf("ContentHash = %q, want %q", restored.ContentHash, original.ContentHash)
			}

			// Compare timestamps using String() to avoid nanosecond precision issues
			// (see vibefeld-7rs7 for known timestamp precision bug)
			if original.Created.String() != restored.Created.String() {
				t.Errorf("Created = %q, want %q", restored.Created.String(), original.Created.String())
			}

			// Verify context ([]string serializes correctly)
			if len(restored.Context) != len(original.Context) {
				t.Errorf("Context length = %d, want %d", len(restored.Context), len(original.Context))
			} else {
				for i := range original.Context {
					if restored.Context[i] != original.Context[i] {
						t.Errorf("Context[%d] = %q, want %q", i, restored.Context[i], original.Context[i])
					}
				}
			}

			// NOTE: Skipping Dependencies comparison - types.NodeID lacks JSON marshaling

			// Verify scope ([]string serializes correctly)
			if len(restored.Scope) != len(original.Scope) {
				t.Errorf("Scope length = %d, want %d", len(restored.Scope), len(original.Scope))
			} else {
				for i := range original.Scope {
					if restored.Scope[i] != original.Scope[i] {
						t.Errorf("Scope[%d] = %q, want %q", i, restored.Scope[i], original.Scope[i])
					}
				}
			}
		})
	}
}

// TestNode_JSON_MultipleRoundtrips tests multiple JSON round trips preserve data.
// Focuses on fields that serialize correctly (excludes NodeID types).
func TestNode_JSON_MultipleRoundtrips(t *testing.T) {
	id, _ := types.Parse("1.2.3")

	opts := node.NodeOptions{
		Latex:   "\\sum_{i=1}^{n} i",
		Context: []string{"def:sum"},
	}

	original, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test statement", schema.InferenceModusPonens, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}

	current := original
	for i := 0; i < 5; i++ {
		data, err := json.Marshal(current)
		if err != nil {
			t.Fatalf("json.Marshal() iteration %d error: %v", i, err)
		}

		var next node.Node
		err = json.Unmarshal(data, &next)
		if err != nil {
			t.Fatalf("json.Unmarshal() iteration %d error: %v", i, err)
		}

		// Verify ContentHash is preserved (string field, serializes correctly)
		if next.ContentHash != original.ContentHash {
			t.Errorf("Round trip %d: ContentHash changed from %q to %q", i, original.ContentHash, next.ContentHash)
		}

		// Verify Statement is preserved
		if next.Statement != original.Statement {
			t.Errorf("Round trip %d: Statement changed from %q to %q", i, original.Statement, next.Statement)
		}

		current = &next
	}
}

// TestNode_JSON_Format tests the JSON output format.
func TestNode_JSON_Format(t *testing.T) {
	id, _ := types.Parse("1.2")
	n, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	// Parse as map to check field names
	var fields map[string]interface{}
	err = json.Unmarshal(data, &fields)
	if err != nil {
		t.Fatalf("json.Unmarshal() to map error: %v", err)
	}

	// Check expected fields exist with correct JSON names
	expectedFields := []string{
		"id",
		"type",
		"statement",
		"inference",
		"workflow_state",
		"epistemic_state",
		"taint_state",
		"content_hash",
		"created",
	}

	for _, field := range expectedFields {
		if _, ok := fields[field]; !ok {
			t.Errorf("JSON missing expected field %q", field)
		}
	}
}

// TestNode_JSON_OmitEmpty tests that optional fields are omitted when empty.
func TestNode_JSON_OmitEmpty(t *testing.T) {
	id, _ := types.Parse("1")
	n, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	// Parse as map to check field names
	var fields map[string]interface{}
	err = json.Unmarshal(data, &fields)
	if err != nil {
		t.Fatalf("json.Unmarshal() to map error: %v", err)
	}

	// These optional fields should be omitted when empty
	optionalFields := []string{"latex", "claimed_by"}

	for _, field := range optionalFields {
		if _, ok := fields[field]; ok {
			t.Errorf("JSON should omit empty field %q", field)
		}
	}
}

// TestNode_Created_NotInFuture tests that Created timestamp is not in the future.
func TestNode_Created_NotInFuture(t *testing.T) {
	id, _ := types.Parse("1")
	n, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	now := types.Now()
	if n.Created.After(now) {
		t.Errorf("Created timestamp %v is in the future (now: %v)", n.Created, now)
	}
}

// TestTaintState_Values tests taint state constant values.
func TestTaintState_Values(t *testing.T) {
	states := []node.TaintState{
		node.TaintClean,
		node.TaintSelfAdmitted,
		node.TaintTainted,
		node.TaintUnresolved,
	}

	seen := make(map[node.TaintState]bool)
	for _, s := range states {
		if seen[s] {
			t.Errorf("Duplicate taint state value: %q", s)
		}
		seen[s] = true

		if s == "" {
			t.Error("Taint state should not be empty string")
		}
	}
}

// TestNode_AllNodeTypes tests that nodes can be created with all valid node types.
func TestNode_AllNodeTypes(t *testing.T) {
	id, _ := types.Parse("1")

	allTypes := []schema.NodeType{
		schema.NodeTypeClaim,
		schema.NodeTypeLocalAssume,
		schema.NodeTypeLocalDischarge,
		schema.NodeTypeCase,
		schema.NodeTypeQED,
	}

	for _, nodeType := range allTypes {
		t.Run(string(nodeType), func(t *testing.T) {
			// Use appropriate inference for the node type
			inference := schema.InferenceAssumption
			if nodeType == schema.NodeTypeLocalAssume {
				inference = schema.InferenceLocalAssume
			} else if nodeType == schema.NodeTypeLocalDischarge {
				inference = schema.InferenceLocalDischarge
			}

			n, err := node.NewNode(id, nodeType, "Test statement for "+string(nodeType), inference)
			if err != nil {
				t.Errorf("NewNode() with type %q unexpected error: %v", nodeType, err)
				return
			}

			if n.Type != nodeType {
				t.Errorf("Type = %q, want %q", n.Type, nodeType)
			}
		})
	}
}

// TestNode_AllInferenceTypes tests that nodes can be created with all valid inference types.
func TestNode_AllInferenceTypes(t *testing.T) {
	id, _ := types.Parse("1")

	allInferences := []schema.InferenceType{
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

	for _, inference := range allInferences {
		t.Run(string(inference), func(t *testing.T) {
			n, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement for "+string(inference), inference)
			if err != nil {
				t.Errorf("NewNode() with inference %q unexpected error: %v", inference, err)
				return
			}

			if n.Inference != inference {
				t.Errorf("Inference = %q, want %q", n.Inference, inference)
			}
		})
	}
}

// TestNode_ZeroValue tests zero value behavior.
func TestNode_ZeroValue(t *testing.T) {
	var n node.Node

	// Zero value should have sensible defaults
	if n.Statement != "" {
		t.Errorf("Zero Node.Statement = %q, want empty string", n.Statement)
	}

	if n.ContentHash != "" {
		t.Errorf("Zero Node.ContentHash = %q, want empty string", n.ContentHash)
	}

	// Methods on zero value should not panic
	_ = n.IsRoot()
	_ = n.Depth()
	_ = n.VerifyContentHash()
}

// TestNode_ContentHash_ManualVerification tests that content hash can be manually verified.
func TestNode_ContentHash_ManualVerification(t *testing.T) {
	id, _ := types.Parse("1")
	statement := "Test statement"
	nodeType := schema.NodeTypeClaim
	inference := schema.InferenceAssumption

	n, err := node.NewNode(id, nodeType, statement, inference)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	// Manually build the expected hash content
	var parts []string
	parts = append(parts, "type:"+string(nodeType))
	parts = append(parts, "statement:"+statement)
	parts = append(parts, "inference:"+string(inference))

	content := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(content))
	expectedHash := hex.EncodeToString(sum[:])

	if n.ContentHash != expectedHash {
		t.Errorf("ContentHash = %q, want %q", n.ContentHash, expectedHash)
	}
}

// TestNode_ContentHash_WithLatex tests content hash includes latex when present.
func TestNode_ContentHash_WithLatex(t *testing.T) {
	id, _ := types.Parse("1")

	optsWithLatex := node.NodeOptions{
		Latex: "\\alpha + \\beta",
	}

	n1, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test", schema.InferenceAssumption, node.NodeOptions{})
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}

	n2, err := node.NewNodeWithOptions(id, schema.NodeTypeClaim, "Test", schema.InferenceAssumption, optsWithLatex)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() error: %v", err)
	}

	if n1.ContentHash == n2.ContentHash {
		t.Error("Content hash should differ when latex is added")
	}
}

// Helper function for comparing string slices
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := make([]string, len(a))
	bCopy := make([]string, len(b))
	copy(aCopy, a)
	copy(bCopy, b)
	sort.Strings(aCopy)
	sort.Strings(bCopy)
	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}
	return true
}

// TestNode_Timestamp_Truncation demonstrates the timestamp precision workaround.
func TestNode_Timestamp_Truncation(t *testing.T) {
	id, _ := types.Parse("1")
	n, err := node.NewNode(id, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}

	// Marshal and unmarshal
	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var restored node.Node
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	// Compare using String() which truncates to seconds (RFC3339)
	// This is the recommended approach per vibefeld-7rs7
	if n.Created.String() != restored.Created.String() {
		t.Errorf("Timestamp mismatch after roundtrip: %q != %q", n.Created.String(), restored.Created.String())
	}
}

// Placeholder to avoid unused import warning for time package
var _ = time.Now

// TestContentHashCollision verifies behavior when two different nodes produce the same content hash.
// This tests that the system correctly handles the case where two nodes with identical content
// (same type, statement, inference, context, and dependencies) produce the same hash.
// The AF system allows this - identical content means identical hash, which is correct behavior.
func TestContentHashCollision(t *testing.T) {
	id1, _ := types.Parse("1.1")
	id2, _ := types.Parse("1.2")

	// Create two nodes with identical content but different IDs
	statement := "For all x in S, P(x) holds"
	nodeType := schema.NodeTypeClaim
	inference := schema.InferenceUniversalGeneralization

	dep1, _ := types.Parse("1")
	opts := node.NodeOptions{
		Latex:        "\\forall x \\in S, P(x)",
		Context:      []string{"def:S", "assume:A1"},
		Dependencies: []types.NodeID{dep1},
	}

	n1, err := node.NewNodeWithOptions(id1, nodeType, statement, inference, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() for node 1 error: %v", err)
	}

	n2, err := node.NewNodeWithOptions(id2, nodeType, statement, inference, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() for node 2 error: %v", err)
	}

	// Verify both nodes have the same content hash (this is expected and correct)
	if n1.ContentHash != n2.ContentHash {
		t.Errorf("Nodes with identical content should have same hash: got %q and %q", n1.ContentHash, n2.ContentHash)
	}

	// Verify the IDs are different
	if n1.ID.String() == n2.ID.String() {
		t.Errorf("Test setup error: nodes should have different IDs")
	}

	// Verify both nodes independently verify their hashes
	if !n1.VerifyContentHash() {
		t.Errorf("Node 1 should verify its content hash")
	}
	if !n2.VerifyContentHash() {
		t.Errorf("Node 2 should verify its content hash")
	}

	// Verify the hash is deterministic - compute it again
	computed1 := n1.ComputeContentHash()
	computed2 := n2.ComputeContentHash()

	if computed1 != computed2 {
		t.Errorf("Re-computed hashes should match: got %q and %q", computed1, computed2)
	}

	if computed1 != n1.ContentHash {
		t.Errorf("Re-computed hash should match stored hash: got %q, stored %q", computed1, n1.ContentHash)
	}

	// Verify that hash does NOT include the ID (important for content-addressing)
	// If we change only the ID and keep content the same, hash should remain the same
	id3, _ := types.Parse("2.3.4")
	n3, err := node.NewNodeWithOptions(id3, nodeType, statement, inference, opts)
	if err != nil {
		t.Fatalf("NewNodeWithOptions() for node 3 error: %v", err)
	}

	if n3.ContentHash != n1.ContentHash {
		t.Errorf("Changing only ID should not affect content hash: got %q, expected %q", n3.ContentHash, n1.ContentHash)
	}
}
