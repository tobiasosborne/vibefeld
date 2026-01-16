package render

import (
	"strings"
	"testing"
)

func TestRenderNodeView(t *testing.T) {
	// Disable color for consistent test output
	originalColor := colorEnabled
	colorEnabled = false
	defer func() { colorEnabled = originalColor }()

	tests := []struct {
		name     string
		v        NodeView
		contains []string
		isEmpty  bool
	}{
		{
			name:    "empty view",
			v:       NodeView{},
			isEmpty: true,
		},
		{
			name: "basic node view",
			v: NodeView{
				ID:             "1",
				Type:           "claim",
				Statement:      "Test statement",
				EpistemicState: "pending",
			},
			contains: []string{"[1]", "claim", "pending", "Test statement"},
		},
		{
			name: "node with multiline statement",
			v: NodeView{
				ID:             "1.2",
				Type:           "claim",
				Statement:      "Line one\nLine two\nLine three",
				EpistemicState: "validated",
			},
			contains: []string{"[1.2]", "claim", "validated", "Line one Line two Line three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderNodeView(tt.v)

			if tt.isEmpty {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
				return
			}

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("result %q should contain %q", result, s)
				}
			}
		})
	}
}

func TestRenderNodeViewVerbose(t *testing.T) {
	originalColor := colorEnabled
	colorEnabled = false
	defer func() { colorEnabled = originalColor }()

	v := NodeView{
		ID:             "1.2.3",
		Type:           "claim",
		Statement:      "A mathematical statement",
		Inference:      "modus_ponens",
		WorkflowState:  "claimed",
		EpistemicState: "pending",
		TaintState:     "clean",
		ContentHash:    "abc123",
		Created:        "2024-01-01T00:00:00Z",
		Context:        []string{"def:group"},
		Dependencies:   []string{"1.2.1", "1.2.2"},
		ValidationDeps: []string{"1.1"},
		Scope:          []string{"assume:hyp1"},
		ClaimedBy:      "agent1",
	}

	result := RenderNodeViewVerbose(v)

	expectedParts := []string{
		"ID:         1.2.3",
		"Type:       claim",
		"Statement:  A mathematical statement",
		"Inference:  modus_ponens",
		"Workflow:   claimed",
		"Epistemic:  pending",
		"Taint:      clean",
		"Hash:       abc123",
		"Created:    2024-01-01T00:00:00Z",
		"Context:    def:group",
		"Depends on: 1.2.1, 1.2.2",
		"Requires validated: 1.1",
		"Scope:      assume:hyp1",
		"Claimed by: agent1",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("result should contain %q, got:\n%s", part, result)
		}
	}
}

func TestRenderJobListView(t *testing.T) {
	originalColor := colorEnabled
	colorEnabled = false
	defer func() { colorEnabled = originalColor }()

	tests := []struct {
		name     string
		jl       JobListView
		contains []string
	}{
		{
			name:     "empty job list",
			jl:       JobListView{},
			contains: []string{"No jobs available"},
		},
		{
			name: "prover jobs only",
			jl: JobListView{
				ProverJobs: []NodeView{
					{ID: "1", Type: "claim", Statement: "Prove this"},
				},
			},
			contains: []string{"Prover Jobs (1 available)", "[1]", "claim"},
		},
		{
			name: "verifier jobs only",
			jl: JobListView{
				VerifierJobs: []NodeView{
					{ID: "1.2", Type: "claim", Statement: "Verify this", ClaimedBy: "agent1"},
				},
			},
			contains: []string{"Verifier Jobs (1 available)", "[1.2]", "claimed by: agent1"},
		},
		{
			name: "both prover and verifier jobs",
			jl: JobListView{
				ProverJobs: []NodeView{
					{ID: "1", Type: "claim", Statement: "Prove this"},
				},
				VerifierJobs: []NodeView{
					{ID: "2", Type: "claim", Statement: "Verify this"},
				},
			},
			contains: []string{"Prover Jobs (1 available)", "Verifier Jobs (1 available)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderJobListView(tt.jl)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("result should contain %q, got:\n%s", s, result)
				}
			}
		})
	}
}

func TestRenderStatusView(t *testing.T) {
	originalColor := colorEnabled
	colorEnabled = false
	defer func() { colorEnabled = originalColor }()

	tests := []struct {
		name     string
		sv       StatusView
		contains []string
	}{
		{
			name:     "empty status",
			sv:       StatusView{},
			contains: []string{"No proof initialized"},
		},
		{
			name: "status with nodes",
			sv: StatusView{
				Nodes: []NodeView{
					{ID: "1", Type: "claim", Statement: "Root", Depth: 1, EpistemicState: "pending", TaintState: "clean"},
				},
				ProverJobCount:   1,
				VerifierJobCount: 0,
			},
			contains: []string{
				"Proof Status",
				"Statistics",
				"Nodes: 1 total",
				"Prover: 1 nodes awaiting refinement",
				"Legend",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderStatusView(tt.sv)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("result should contain %q, got:\n%s", s, result)
				}
			}
		})
	}
}

func TestRenderTreeView(t *testing.T) {
	originalColor := colorEnabled
	colorEnabled = false
	defer func() { colorEnabled = originalColor }()

	tests := []struct {
		name     string
		tv       TreeView
		contains []string
		isEmpty  bool
	}{
		{
			name:    "empty tree",
			tv:      TreeView{},
			isEmpty: true,
		},
		{
			name: "single root node",
			tv: TreeView{
				Nodes: []NodeView{
					{ID: "1", Depth: 1, EpistemicState: "pending", TaintState: "clean", Statement: "Root"},
				},
				NodeLookup: map[string]NodeView{
					"1": {ID: "1", Depth: 1, EpistemicState: "pending", TaintState: "clean", Statement: "Root"},
				},
			},
			contains: []string{"1 [pending/clean] Root"},
		},
		{
			name: "tree with children",
			tv: TreeView{
				Nodes: []NodeView{
					{ID: "1", Depth: 1, EpistemicState: "pending", TaintState: "clean", Statement: "Root"},
					{ID: "1.1", Depth: 2, EpistemicState: "validated", TaintState: "clean", Statement: "Child 1"},
					{ID: "1.2", Depth: 2, EpistemicState: "pending", TaintState: "clean", Statement: "Child 2"},
				},
				NodeLookup: map[string]NodeView{
					"1":   {ID: "1", Depth: 1, EpistemicState: "pending", TaintState: "clean", Statement: "Root"},
					"1.1": {ID: "1.1", Depth: 2, EpistemicState: "validated", TaintState: "clean", Statement: "Child 1"},
					"1.2": {ID: "1.2", Depth: 2, EpistemicState: "pending", TaintState: "clean", Statement: "Child 2"},
				},
			},
			contains: []string{"1 [pending/clean] Root", "1.1 [validated/clean] Child 1", "1.2 [pending/clean] Child 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderTreeView(tt.tv)

			if tt.isEmpty {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
				return
			}

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("result should contain %q, got:\n%s", s, result)
				}
			}
		})
	}
}

func TestRenderProverContextView(t *testing.T) {
	originalColor := colorEnabled
	colorEnabled = false
	defer func() { colorEnabled = originalColor }()

	tests := []struct {
		name     string
		ctx      ProverContextView
		contains []string
		isEmpty  bool
	}{
		{
			name:    "empty context",
			ctx:     ProverContextView{},
			isEmpty: true,
		},
		{
			name: "basic prover context",
			ctx: ProverContextView{
				Node: NodeView{
					ID:             "1.2",
					Type:           "claim",
					Statement:      "Prove this",
					WorkflowState:  "claimed",
					EpistemicState: "pending",
					TaintState:     "clean",
					ClaimedBy:      "agent1",
				},
				Parent: &NodeView{
					ID:        "1",
					Statement: "Parent statement",
				},
				Siblings: []NodeView{
					{ID: "1.1", Statement: "Sibling statement"},
				},
				Dependencies: []NodeView{
					{ID: "1.1", Statement: "Dependency statement"},
				},
			},
			contains: []string{
				"Prover Context for Node 1.2",
				"Statement: Prove this",
				"claimed by: agent1",
				"Parent (1)",
				"Parent statement",
				"Siblings:",
				"1.1:",
			},
		},
		{
			name: "context with definitions and challenges",
			ctx: ProverContextView{
				Node: NodeView{
					ID:             "1",
					Type:           "claim",
					Statement:      "Main claim",
					EpistemicState: "pending",
					TaintState:     "clean",
				},
				Definitions: []DefinitionView{
					{ID: "def1", Name: "group", Content: "A group is a set with a binary operation"},
				},
				Challenges: []ChallengeView{
					{ID: "ch1", Reason: "Needs more detail", Status: "open"},
				},
			},
			contains: []string{
				"Definitions in scope:",
				"group: A group is a set",
				"Challenges (1 total, 1 open):",
				"[ch1]",
				"Needs more detail",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderProverContextView(tt.ctx)

			if tt.isEmpty {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
				return
			}

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("result should contain %q, got:\n%s", s, result)
				}
			}
		})
	}
}

func TestRenderVerifierContextView(t *testing.T) {
	originalColor := colorEnabled
	colorEnabled = false
	defer func() { colorEnabled = originalColor }()

	tests := []struct {
		name     string
		ctx      VerifierContextView
		contains []string
		isEmpty  bool
	}{
		{
			name:    "empty context",
			ctx:     VerifierContextView{},
			isEmpty: true,
		},
		{
			name: "basic verifier context",
			ctx: VerifierContextView{
				Challenge: ChallengeView{
					ID:         "ch-abc123",
					TargetID:   "1.2",
					Target:     "statement",
					TargetDesc: "The claim itself",
					Reason:     "Unclear reasoning",
					Status:     "open",
				},
				Node: NodeView{
					ID:             "1.2",
					Type:           "claim",
					Statement:      "A statement under review",
					Inference:      "modus_ponens",
					EpistemicState: "pending",
					WorkflowState:  "claimed",
					TaintState:     "clean",
				},
			},
			contains: []string{
				"Verifier Context for Challenge ch-abc123",
				"Challenge: ch-abc123",
				"Target Node: 1.2",
				"Target Aspect: statement (The claim itself)",
				"Reason: Unclear reasoning",
				"Status: open",
				"Challenged Node:",
				"Statement: A statement under review",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderVerifierContextView(tt.ctx)

			if tt.isEmpty {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
				return
			}

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("result should contain %q, got:\n%s", s, result)
				}
			}
		})
	}
}

func TestRenderSearchResultViews(t *testing.T) {
	originalColor := colorEnabled
	colorEnabled = false
	defer func() { colorEnabled = originalColor }()

	tests := []struct {
		name     string
		results  []SearchResultView
		contains []string
	}{
		{
			name:     "empty results",
			results:  nil,
			contains: []string{"No matching nodes found"},
		},
		{
			name: "single result",
			results: []SearchResultView{
				{
					Node: NodeView{
						ID:             "1.2",
						Type:           "claim",
						Statement:      "Found node",
						EpistemicState: "pending",
					},
					MatchReason: "text match",
				},
			},
			contains: []string{"Search Results:", "[1.2]", "pending", "Found node", "text match", "Total: 1 node"},
		},
		{
			name: "multiple results",
			results: []SearchResultView{
				{
					Node: NodeView{ID: "1", Type: "claim", Statement: "First", EpistemicState: "pending"},
				},
				{
					Node: NodeView{ID: "2", Type: "claim", Statement: "Second", EpistemicState: "validated"},
				},
			},
			contains: []string{"[1]", "[2]", "Total: 2 nodes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderSearchResultViews(tt.results)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("result should contain %q, got:\n%s", s, result)
				}
			}
		})
	}
}

func TestColorEpistemicStateString(t *testing.T) {
	// Test with color enabled
	originalColor := colorEnabled
	colorEnabled = true
	defer func() { colorEnabled = originalColor }()

	tests := []struct {
		state    string
		expected string
	}{
		{"pending", Yellow("pending")},
		{"validated", Green("validated")},
		{"admitted", Cyan("admitted")},
		{"refuted", Red("refuted")},
		{"archived", Gray("archived")},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := colorEpistemicStateString(tt.state); got != tt.expected {
				t.Errorf("colorEpistemicStateString(%q) = %q, want %q", tt.state, got, tt.expected)
			}
		})
	}
}

func TestColorTaintStateString(t *testing.T) {
	originalColor := colorEnabled
	colorEnabled = true
	defer func() { colorEnabled = originalColor }()

	tests := []struct {
		state    string
		expected string
	}{
		{"clean", Green("clean")},
		{"self_admitted", Cyan("self_admitted")},
		{"tainted", Red("tainted")},
		{"unresolved", Yellow("unresolved")},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := colorTaintStateString(tt.state); got != tt.expected {
				t.Errorf("colorTaintStateString(%q) = %q, want %q", tt.state, got, tt.expected)
			}
		})
	}
}

func TestSortNodeViewsByID(t *testing.T) {
	nodes := []NodeView{
		{ID: "1.10"},
		{ID: "1.2"},
		{ID: "1.1"},
		{ID: "2.1"},
		{ID: "1"},
	}

	sortNodeViewsByID(nodes)

	expected := []string{"1", "1.1", "1.2", "1.10", "2.1"}
	for i, n := range nodes {
		if n.ID != expected[i] {
			t.Errorf("sortNodeViewsByID() index %d = %q, want %q", i, n.ID, expected[i])
		}
	}
}
