package render

import (
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// mustParseNodeID is a test helper that panics on parse error.
func mustParseNodeID(s string) types.NodeID {
	id, err := types.Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}

func TestNodeToView(t *testing.T) {
	tests := []struct {
		name     string
		node     *node.Node
		expected NodeView
	}{
		{
			name:     "nil node",
			node:     nil,
			expected: NodeView{},
		},
		{
			name: "basic node",
			node: &node.Node{
				ID:             mustParseNodeID("1.2.3"),
				Type:           schema.NodeTypeClaim,
				Statement:      "Test statement",
				Inference:      schema.InferenceModusPonens,
				WorkflowState:  schema.WorkflowClaimed,
				EpistemicState: schema.EpistemicPending,
				TaintState:     node.TaintClean,
				ContentHash:    "abc123",
				Created:        types.Timestamp{},
				Context:        []string{"def:group"},
				Dependencies:   []types.NodeID{mustParseNodeID("1.1")},
				ValidationDeps: []types.NodeID{mustParseNodeID("1.2.1")},
				Scope:          []string{"assume:hyp"},
				ClaimedBy:      "agent1",
			},
			expected: NodeView{
				ID:             "1.2.3",
				Type:           "claim",
				Statement:      "Test statement",
				Inference:      "modus_ponens",
				WorkflowState:  "claimed",
				EpistemicState: "pending",
				TaintState:     "clean",
				ContentHash:    "abc123",
				Context:        []string{"def:group"},
				Dependencies:   []string{"1.1"},
				ValidationDeps: []string{"1.2.1"},
				Scope:          []string{"assume:hyp"},
				ClaimedBy:      "agent1",
				Depth:          3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NodeToView(tt.node)

			if result.ID != tt.expected.ID {
				t.Errorf("ID = %q, want %q", result.ID, tt.expected.ID)
			}
			if result.Type != tt.expected.Type {
				t.Errorf("Type = %q, want %q", result.Type, tt.expected.Type)
			}
			if result.Statement != tt.expected.Statement {
				t.Errorf("Statement = %q, want %q", result.Statement, tt.expected.Statement)
			}
			if result.WorkflowState != tt.expected.WorkflowState {
				t.Errorf("WorkflowState = %q, want %q", result.WorkflowState, tt.expected.WorkflowState)
			}
			if result.EpistemicState != tt.expected.EpistemicState {
				t.Errorf("EpistemicState = %q, want %q", result.EpistemicState, tt.expected.EpistemicState)
			}
			if result.TaintState != tt.expected.TaintState {
				t.Errorf("TaintState = %q, want %q", result.TaintState, tt.expected.TaintState)
			}
			if result.ClaimedBy != tt.expected.ClaimedBy {
				t.Errorf("ClaimedBy = %q, want %q", result.ClaimedBy, tt.expected.ClaimedBy)
			}
			if result.Depth != tt.expected.Depth {
				t.Errorf("Depth = %d, want %d", result.Depth, tt.expected.Depth)
			}
		})
	}
}

func TestNodesToViews(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []*node.Node
		expected int
	}{
		{
			name:     "nil slice",
			nodes:    nil,
			expected: 0,
		},
		{
			name:     "empty slice",
			nodes:    []*node.Node{},
			expected: 0,
		},
		{
			name: "slice with nodes",
			nodes: []*node.Node{
				{ID: mustParseNodeID("1")},
				{ID: mustParseNodeID("1.1")},
			},
			expected: 2,
		},
		{
			name: "slice with nil node",
			nodes: []*node.Node{
				{ID: mustParseNodeID("1")},
				nil,
				{ID: mustParseNodeID("1.1")},
			},
			expected: 2, // nil nodes are filtered out
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NodesToViews(tt.nodes)
			if len(result) != tt.expected {
				t.Errorf("NodesToViews() len = %d, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestNodeChallengeToView(t *testing.T) {
	tests := []struct {
		name      string
		challenge *node.Challenge
		expected  ChallengeView
	}{
		{
			name:      "nil challenge",
			challenge: nil,
			expected:  ChallengeView{},
		},
		{
			name: "basic challenge",
			challenge: &node.Challenge{
				ID:       "ch-abc123",
				TargetID: mustParseNodeID("1.2"),
				Target:   schema.TargetStatement,
				Reason:   "Test reason",
				Status:   node.ChallengeStatusOpen,
			},
			expected: ChallengeView{
				ID:       "ch-abc123",
				TargetID: "1.2",
				Target:   "statement",
				Reason:   "Test reason",
				Status:   "open",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NodeChallengeToView(tt.challenge)

			if result.ID != tt.expected.ID {
				t.Errorf("ID = %q, want %q", result.ID, tt.expected.ID)
			}
			if result.TargetID != tt.expected.TargetID {
				t.Errorf("TargetID = %q, want %q", result.TargetID, tt.expected.TargetID)
			}
			if result.Target != tt.expected.Target {
				t.Errorf("Target = %q, want %q", result.Target, tt.expected.Target)
			}
			if result.Reason != tt.expected.Reason {
				t.Errorf("Reason = %q, want %q", result.Reason, tt.expected.Reason)
			}
			if result.Status != tt.expected.Status {
				t.Errorf("Status = %q, want %q", result.Status, tt.expected.Status)
			}
		})
	}
}

func TestDefinitionToView(t *testing.T) {
	tests := []struct {
		name     string
		def      *node.Definition
		expected DefinitionView
	}{
		{
			name:     "nil definition",
			def:      nil,
			expected: DefinitionView{},
		},
		{
			name: "basic definition",
			def: &node.Definition{
				ID:      "def-123",
				Name:    "group",
				Content: "A group is a set with a binary operation",
			},
			expected: DefinitionView{
				ID:      "def-123",
				Name:    "group",
				Content: "A group is a set with a binary operation",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DefinitionToView(tt.def)
			if result.ID != tt.expected.ID {
				t.Errorf("ID = %q, want %q", result.ID, tt.expected.ID)
			}
			if result.Name != tt.expected.Name {
				t.Errorf("Name = %q, want %q", result.Name, tt.expected.Name)
			}
			if result.Content != tt.expected.Content {
				t.Errorf("Content = %q, want %q", result.Content, tt.expected.Content)
			}
		})
	}
}

func TestAssumptionToView(t *testing.T) {
	tests := []struct {
		name     string
		assume   *node.Assumption
		expected AssumptionView
	}{
		{
			name:     "nil assumption",
			assume:   nil,
			expected: AssumptionView{},
		},
		{
			name: "basic assumption",
			assume: &node.Assumption{
				ID:            "assume-123",
				Statement:     "x > 0",
				Justification: "Given in hypothesis",
			},
			expected: AssumptionView{
				ID:            "assume-123",
				Statement:     "x > 0",
				Justification: "Given in hypothesis",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AssumptionToView(tt.assume)
			if result.ID != tt.expected.ID {
				t.Errorf("ID = %q, want %q", result.ID, tt.expected.ID)
			}
			if result.Statement != tt.expected.Statement {
				t.Errorf("Statement = %q, want %q", result.Statement, tt.expected.Statement)
			}
			if result.Justification != tt.expected.Justification {
				t.Errorf("Justification = %q, want %q", result.Justification, tt.expected.Justification)
			}
		})
	}
}

func TestExternalToView(t *testing.T) {
	tests := []struct {
		name     string
		ext      *node.External
		expected ExternalView
	}{
		{
			name:     "nil external",
			ext:      nil,
			expected: ExternalView{},
		},
		{
			name: "basic external",
			ext: &node.External{
				ID:     "ext-123",
				Name:   "Theorem 3.1",
				Source: "Rudin, Chapter 3",
				Notes:  "Well-known result",
			},
			expected: ExternalView{
				ID:     "ext-123",
				Name:   "Theorem 3.1",
				Source: "Rudin, Chapter 3",
				Notes:  "Well-known result",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExternalToView(tt.ext)
			if result.ID != tt.expected.ID {
				t.Errorf("ID = %q, want %q", result.ID, tt.expected.ID)
			}
			if result.Name != tt.expected.Name {
				t.Errorf("Name = %q, want %q", result.Name, tt.expected.Name)
			}
			if result.Source != tt.expected.Source {
				t.Errorf("Source = %q, want %q", result.Source, tt.expected.Source)
			}
			if result.Notes != tt.expected.Notes {
				t.Errorf("Notes = %q, want %q", result.Notes, tt.expected.Notes)
			}
		})
	}
}

func TestIsNodeViewRoot(t *testing.T) {
	tests := []struct {
		name     string
		v        NodeView
		expected bool
	}{
		{
			name:     "root node",
			v:        NodeView{ID: "1", Depth: 1},
			expected: true,
		},
		{
			name:     "child node",
			v:        NodeView{ID: "1.2", Depth: 2},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNodeViewRoot(tt.v); got != tt.expected {
				t.Errorf("IsNodeViewRoot() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetNodeViewParentID_Adapters(t *testing.T) {
	tests := []struct {
		name      string
		v         NodeView
		wantID    string
		wantFound bool
	}{
		{
			name:      "root has no parent",
			v:         NodeView{ID: "1"},
			wantID:    "",
			wantFound: false,
		},
		{
			name:      "child has parent",
			v:         NodeView{ID: "1.2"},
			wantID:    "1",
			wantFound: true,
		},
		{
			name:      "grandchild has parent",
			v:         NodeView{ID: "1.2.3"},
			wantID:    "1.2",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotFound := GetNodeViewParentID(tt.v)
			if gotID != tt.wantID {
				t.Errorf("GetNodeViewParentID() ID = %q, want %q", gotID, tt.wantID)
			}
			if gotFound != tt.wantFound {
				t.Errorf("GetNodeViewParentID() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}
