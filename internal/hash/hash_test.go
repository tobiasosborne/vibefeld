package hash

import (
	"regexp"
	"testing"
)

func TestComputeNodeHash(t *testing.T) {
	tests := []struct {
		name         string
		nodeType     string
		statement    string
		latex        string
		inference    string
		context      []string
		dependencies []string
		wantErr      bool
	}{
		{
			name:         "all fields populated",
			nodeType:     "step",
			statement:    "By the triangle inequality, |a + b| <= |a| + |b|",
			latex:        "|a + b| \\leq |a| + |b|",
			inference:    "triangle_inequality",
			context:      []string{"DEF-norm", "DEF-real"},
			dependencies: []string{"1.1", "1.2"},
			wantErr:      false,
		},
		{
			name:         "empty latex",
			nodeType:     "step",
			statement:    "This follows from basic arithmetic",
			latex:        "",
			inference:    "arithmetic",
			context:      []string{"DEF-add"},
			dependencies: []string{"1.1"},
			wantErr:      false,
		},
		{
			name:         "empty context",
			nodeType:     "claim",
			statement:    "The result holds",
			latex:        "P(x)",
			inference:    "direct",
			context:      []string{},
			dependencies: []string{"1.1", "1.2", "1.3"},
			wantErr:      false,
		},
		{
			name:         "empty dependencies",
			nodeType:     "root",
			statement:    "Main theorem statement",
			latex:        "\\forall x. P(x)",
			inference:    "theorem",
			context:      []string{"DEF-P"},
			dependencies: []string{},
			wantErr:      false,
		},
		{
			name:         "empty context and dependencies",
			nodeType:     "axiom",
			statement:    "An axiom needs no justification",
			latex:        "",
			inference:    "axiom",
			context:      []string{},
			dependencies: []string{},
			wantErr:      false,
		},
		{
			name:         "nil context and dependencies",
			nodeType:     "axiom",
			statement:    "An axiom needs no justification",
			latex:        "",
			inference:    "axiom",
			context:      nil,
			dependencies: nil,
			wantErr:      false,
		},
		{
			name:         "empty statement",
			nodeType:     "step",
			statement:    "",
			latex:        "x = y",
			inference:    "equality",
			context:      []string{},
			dependencies: []string{},
			wantErr:      false,
		},
		{
			name:         "all empty strings",
			nodeType:     "",
			statement:    "",
			latex:        "",
			inference:    "",
			context:      []string{},
			dependencies: []string{},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := ComputeNodeHash(tt.nodeType, tt.statement, tt.latex, tt.inference, tt.context, tt.dependencies)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComputeNodeHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && hash == "" {
				t.Error("ComputeNodeHash() returned empty hash")
			}
		})
	}
}

func TestComputeNodeHash_Format(t *testing.T) {
	hash, err := ComputeNodeHash(
		"step",
		"Test statement",
		"x = y",
		"equality",
		[]string{"DEF-a"},
		[]string{"1.1"},
	)
	if err != nil {
		t.Fatalf("ComputeNodeHash() unexpected error: %v", err)
	}

	// SHA256 produces 64 hex characters
	if len(hash) != 64 {
		t.Errorf("ComputeNodeHash() hash length = %d, want 64", len(hash))
	}

	// Should be valid lowercase hex string
	validHex := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !validHex.MatchString(hash) {
		t.Errorf("ComputeNodeHash() hash = %q, not a valid hex string", hash)
	}
}

func TestComputeNodeHash_Deterministic(t *testing.T) {
	// Same inputs should always produce same hash
	inputs := struct {
		nodeType     string
		statement    string
		latex        string
		inference    string
		context      []string
		dependencies []string
	}{
		nodeType:     "step",
		statement:    "Deterministic test",
		latex:        "D(x)",
		inference:    "test",
		context:      []string{"DEF-a", "DEF-b"},
		dependencies: []string{"1.1", "1.2"},
	}

	hash1, err := ComputeNodeHash(inputs.nodeType, inputs.statement, inputs.latex, inputs.inference, inputs.context, inputs.dependencies)
	if err != nil {
		t.Fatalf("First ComputeNodeHash() unexpected error: %v", err)
	}

	hash2, err := ComputeNodeHash(inputs.nodeType, inputs.statement, inputs.latex, inputs.inference, inputs.context, inputs.dependencies)
	if err != nil {
		t.Fatalf("Second ComputeNodeHash() unexpected error: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("ComputeNodeHash() not deterministic: %q != %q", hash1, hash2)
	}

	// Call multiple times to ensure consistency
	for i := 0; i < 10; i++ {
		hash, err := ComputeNodeHash(inputs.nodeType, inputs.statement, inputs.latex, inputs.inference, inputs.context, inputs.dependencies)
		if err != nil {
			t.Fatalf("ComputeNodeHash() iteration %d unexpected error: %v", i, err)
		}
		if hash != hash1 {
			t.Errorf("ComputeNodeHash() iteration %d: %q != %q", i, hash, hash1)
		}
	}
}

func TestComputeNodeHash_DeterministicOrdering_Context(t *testing.T) {
	// Context arrays with same elements in different order should produce same hash
	tests := []struct {
		name     string
		context1 []string
		context2 []string
	}{
		{
			name:     "two elements reversed",
			context1: []string{"DEF-a", "DEF-b"},
			context2: []string{"DEF-b", "DEF-a"},
		},
		{
			name:     "three elements shuffled",
			context1: []string{"DEF-a", "DEF-b", "DEF-c"},
			context2: []string{"DEF-c", "DEF-a", "DEF-b"},
		},
		{
			name:     "many elements different order",
			context1: []string{"DEF-1", "DEF-2", "DEF-3", "DEF-4", "DEF-5"},
			context2: []string{"DEF-5", "DEF-3", "DEF-1", "DEF-4", "DEF-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1, err := ComputeNodeHash("step", "statement", "latex", "inference", tt.context1, []string{"1.1"})
			if err != nil {
				t.Fatalf("ComputeNodeHash() with context1 error: %v", err)
			}

			hash2, err := ComputeNodeHash("step", "statement", "latex", "inference", tt.context2, []string{"1.1"})
			if err != nil {
				t.Fatalf("ComputeNodeHash() with context2 error: %v", err)
			}

			if hash1 != hash2 {
				t.Errorf("ComputeNodeHash() context order matters: context1=%v hash=%q, context2=%v hash=%q",
					tt.context1, hash1, tt.context2, hash2)
			}
		})
	}
}

func TestComputeNodeHash_DeterministicOrdering_Dependencies(t *testing.T) {
	// Dependencies arrays with same elements in different order should produce same hash
	tests := []struct {
		name string
		deps1 []string
		deps2 []string
	}{
		{
			name:  "two elements reversed",
			deps1: []string{"1.1", "1.2"},
			deps2: []string{"1.2", "1.1"},
		},
		{
			name:  "three elements shuffled",
			deps1: []string{"1.1", "1.2", "1.3"},
			deps2: []string{"1.3", "1.1", "1.2"},
		},
		{
			name:  "complex node IDs different order",
			deps1: []string{"1.1.1", "1.1.2", "1.2.1", "2.1"},
			deps2: []string{"2.1", "1.1.2", "1.2.1", "1.1.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1, err := ComputeNodeHash("step", "statement", "latex", "inference", []string{"DEF-a"}, tt.deps1)
			if err != nil {
				t.Fatalf("ComputeNodeHash() with deps1 error: %v", err)
			}

			hash2, err := ComputeNodeHash("step", "statement", "latex", "inference", []string{"DEF-a"}, tt.deps2)
			if err != nil {
				t.Fatalf("ComputeNodeHash() with deps2 error: %v", err)
			}

			if hash1 != hash2 {
				t.Errorf("ComputeNodeHash() dependency order matters: deps1=%v hash=%q, deps2=%v hash=%q",
					tt.deps1, hash1, tt.deps2, hash2)
			}
		})
	}
}

func TestComputeNodeHash_DeterministicOrdering_Both(t *testing.T) {
	// Both context and dependencies in different order should produce same hash
	hash1, err := ComputeNodeHash(
		"step",
		"statement",
		"latex",
		"inference",
		[]string{"DEF-z", "DEF-a", "DEF-m"},
		[]string{"3.1", "1.1", "2.1"},
	)
	if err != nil {
		t.Fatalf("ComputeNodeHash() hash1 error: %v", err)
	}

	hash2, err := ComputeNodeHash(
		"step",
		"statement",
		"latex",
		"inference",
		[]string{"DEF-a", "DEF-m", "DEF-z"},
		[]string{"1.1", "2.1", "3.1"},
	)
	if err != nil {
		t.Fatalf("ComputeNodeHash() hash2 error: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("ComputeNodeHash() order of both context and deps matters: hash1=%q, hash2=%q", hash1, hash2)
	}
}

func TestComputeNodeHash_DifferentInputs(t *testing.T) {
	// Different inputs should produce different hashes
	baseHash, err := ComputeNodeHash(
		"step",
		"Base statement",
		"B(x)",
		"base_inference",
		[]string{"DEF-base"},
		[]string{"1.1"},
	)
	if err != nil {
		t.Fatalf("ComputeNodeHash() base hash error: %v", err)
	}

	tests := []struct {
		name         string
		nodeType     string
		statement    string
		latex        string
		inference    string
		context      []string
		dependencies []string
	}{
		{
			name:         "different nodeType",
			nodeType:     "claim",
			statement:    "Base statement",
			latex:        "B(x)",
			inference:    "base_inference",
			context:      []string{"DEF-base"},
			dependencies: []string{"1.1"},
		},
		{
			name:         "different statement",
			nodeType:     "step",
			statement:    "Different statement",
			latex:        "B(x)",
			inference:    "base_inference",
			context:      []string{"DEF-base"},
			dependencies: []string{"1.1"},
		},
		{
			name:         "different latex",
			nodeType:     "step",
			statement:    "Base statement",
			latex:        "D(x)",
			inference:    "base_inference",
			context:      []string{"DEF-base"},
			dependencies: []string{"1.1"},
		},
		{
			name:         "different inference",
			nodeType:     "step",
			statement:    "Base statement",
			latex:        "B(x)",
			inference:    "different_inference",
			context:      []string{"DEF-base"},
			dependencies: []string{"1.1"},
		},
		{
			name:         "different context",
			nodeType:     "step",
			statement:    "Base statement",
			latex:        "B(x)",
			inference:    "base_inference",
			context:      []string{"DEF-different"},
			dependencies: []string{"1.1"},
		},
		{
			name:         "additional context",
			nodeType:     "step",
			statement:    "Base statement",
			latex:        "B(x)",
			inference:    "base_inference",
			context:      []string{"DEF-base", "DEF-extra"},
			dependencies: []string{"1.1"},
		},
		{
			name:         "different dependencies",
			nodeType:     "step",
			statement:    "Base statement",
			latex:        "B(x)",
			inference:    "base_inference",
			context:      []string{"DEF-base"},
			dependencies: []string{"1.2"},
		},
		{
			name:         "additional dependencies",
			nodeType:     "step",
			statement:    "Base statement",
			latex:        "B(x)",
			inference:    "base_inference",
			context:      []string{"DEF-base"},
			dependencies: []string{"1.1", "1.2"},
		},
		{
			name:         "empty vs non-empty context",
			nodeType:     "step",
			statement:    "Base statement",
			latex:        "B(x)",
			inference:    "base_inference",
			context:      []string{},
			dependencies: []string{"1.1"},
		},
		{
			name:         "empty vs non-empty dependencies",
			nodeType:     "step",
			statement:    "Base statement",
			latex:        "B(x)",
			inference:    "base_inference",
			context:      []string{"DEF-base"},
			dependencies: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := ComputeNodeHash(tt.nodeType, tt.statement, tt.latex, tt.inference, tt.context, tt.dependencies)
			if err != nil {
				t.Fatalf("ComputeNodeHash() error: %v", err)
			}

			if hash == baseHash {
				t.Errorf("ComputeNodeHash() produced same hash as base for %s: %q", tt.name, hash)
			}
		})
	}
}

func TestComputeNodeHash_EmptyVsNil(t *testing.T) {
	// Empty slice and nil slice should produce the same hash
	hashNil, err := ComputeNodeHash("step", "statement", "latex", "inference", nil, nil)
	if err != nil {
		t.Fatalf("ComputeNodeHash() with nil slices error: %v", err)
	}

	hashEmpty, err := ComputeNodeHash("step", "statement", "latex", "inference", []string{}, []string{})
	if err != nil {
		t.Fatalf("ComputeNodeHash() with empty slices error: %v", err)
	}

	if hashNil != hashEmpty {
		t.Errorf("ComputeNodeHash() nil vs empty slices produce different hashes: nil=%q, empty=%q", hashNil, hashEmpty)
	}
}

func TestComputeNodeHash_SpecialCharacters(t *testing.T) {
	// Hash should handle special characters correctly
	tests := []struct {
		name      string
		statement string
		latex     string
	}{
		{
			name:      "unicode characters",
			statement: "Let epsilon be arbitrary",
			latex:     "\\varepsilon > 0",
		},
		{
			name:      "newlines",
			statement: "Line 1\nLine 2\nLine 3",
			latex:     "x =\n  y",
		},
		{
			name:      "tabs",
			statement: "Column1\tColumn2",
			latex:     "a\tb",
		},
		{
			name:      "quotes",
			statement: `He said "hello"`,
			latex:     `"x"`,
		},
		{
			name:      "backslashes",
			statement: `Path: C:\Users\test`,
			latex:     `\\frac{a}{b}`,
		},
		{
			name:      "null bytes",
			statement: "before\x00after",
			latex:     "\x00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := ComputeNodeHash("step", tt.statement, tt.latex, "inference", []string{}, []string{})
			if err != nil {
				t.Fatalf("ComputeNodeHash() error: %v", err)
			}

			// Should still produce valid 64-char hex hash
			if len(hash) != 64 {
				t.Errorf("ComputeNodeHash() hash length = %d, want 64", len(hash))
			}

			validHex := regexp.MustCompile(`^[0-9a-f]{64}$`)
			if !validHex.MatchString(hash) {
				t.Errorf("ComputeNodeHash() hash = %q, not a valid hex string", hash)
			}
		})
	}
}

func TestComputeNodeHash_DoesNotMutateInput(t *testing.T) {
	// The function should not modify the input slices
	context := []string{"DEF-b", "DEF-a", "DEF-c"}
	dependencies := []string{"1.3", "1.1", "1.2"}

	// Make copies to compare after
	contextCopy := make([]string, len(context))
	depsCopy := make([]string, len(dependencies))
	copy(contextCopy, context)
	copy(depsCopy, dependencies)

	_, err := ComputeNodeHash("step", "statement", "latex", "inference", context, dependencies)
	if err != nil {
		t.Fatalf("ComputeNodeHash() error: %v", err)
	}

	// Verify original slices unchanged
	for i, v := range context {
		if v != contextCopy[i] {
			t.Errorf("ComputeNodeHash() mutated context slice: original[%d]=%q, now=%q", i, contextCopy[i], v)
		}
	}

	for i, v := range dependencies {
		if v != depsCopy[i] {
			t.Errorf("ComputeNodeHash() mutated dependencies slice: original[%d]=%q, now=%q", i, depsCopy[i], v)
		}
	}
}
