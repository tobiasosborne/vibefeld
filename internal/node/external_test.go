package node_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
)

func TestNewExternal(t *testing.T) {
	tests := []struct {
		name       string
		refName    string
		source     string
		wantName   string
		wantSource string
	}{
		{
			name:       "basic theorem reference",
			refName:    "Fermat's Last Theorem",
			source:     "Wiles, A. (1995). Modular elliptic curves and Fermat's Last Theorem.",
			wantName:   "Fermat's Last Theorem",
			wantSource: "Wiles, A. (1995). Modular elliptic curves and Fermat's Last Theorem.",
		},
		{
			name:       "URL reference",
			refName:    "Prime Number Theorem",
			source:     "https://en.wikipedia.org/wiki/Prime_number_theorem",
			wantName:   "Prime Number Theorem",
			wantSource: "https://en.wikipedia.org/wiki/Prime_number_theorem",
		},
		{
			name:       "short name and source",
			refName:    "PNT",
			source:     "Hardy & Wright",
			wantName:   "PNT",
			wantSource: "Hardy & Wright",
		},
		{
			name:       "unicode characters",
			refName:    "Gödel's Incompleteness",
			source:     "Gödel, K. (1931). Über formal unentscheidbare Sätze",
			wantName:   "Gödel's Incompleteness",
			wantSource: "Gödel, K. (1931). Über formal unentscheidbare Sätze",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, err := node.NewExternal(tt.refName, tt.source)
			if err != nil {
				t.Fatalf("NewExternal() unexpected error: %v", err)
			}

			if ext.Name != tt.wantName {
				t.Errorf("NewExternal().Name = %q, want %q", ext.Name, tt.wantName)
			}
			if ext.Source != tt.wantSource {
				t.Errorf("NewExternal().Source = %q, want %q", ext.Source, tt.wantSource)
			}
			if ext.ID == "" {
				t.Error("NewExternal().ID should not be empty")
			}
			if ext.Created.IsZero() {
				t.Error("NewExternal().Created should not be zero")
			}
			if ext.Notes != "" {
				t.Errorf("NewExternal().Notes = %q, want empty string", ext.Notes)
			}
		})
	}
}

func TestNewExternalWithNotes(t *testing.T) {
	tests := []struct {
		name       string
		refName    string
		source     string
		notes      string
		wantNotes  string
	}{
		{
			name:      "basic notes",
			refName:   "ZFC",
			source:    "Zermelo-Fraenkel set theory with Choice",
			notes:     "Standard axioms of set theory",
			wantNotes: "Standard axioms of set theory",
		},
		{
			name:      "empty notes",
			refName:   "AC",
			source:    "Axiom of Choice",
			notes:     "",
			wantNotes: "",
		},
		{
			name:      "multiline notes",
			refName:   "Riemann Hypothesis",
			source:    "https://mathworld.wolfram.com/RiemannHypothesis.html",
			notes:     "Unproven conjecture.\nCritical for number theory.",
			wantNotes: "Unproven conjecture.\nCritical for number theory.",
		},
		{
			name:      "notes with special characters",
			refName:   "Cauchy-Schwarz",
			source:    "https://arxiv.org/example",
			notes:     "Key inequality: |<u,v>| <= ||u|| * ||v||",
			wantNotes: "Key inequality: |<u,v>| <= ||u|| * ||v||",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, err := node.NewExternalWithNotes(tt.refName, tt.source, tt.notes)
			if err != nil {
				t.Fatalf("NewExternalWithNotes() unexpected error: %v", err)
			}

			if ext.Name != tt.refName {
				t.Errorf("NewExternalWithNotes().Name = %q, want %q", ext.Name, tt.refName)
			}
			if ext.Source != tt.source {
				t.Errorf("NewExternalWithNotes().Source = %q, want %q", ext.Source, tt.source)
			}
			if ext.Notes != tt.wantNotes {
				t.Errorf("NewExternalWithNotes().Notes = %q, want %q", ext.Notes, tt.wantNotes)
			}
			if ext.ID == "" {
				t.Error("NewExternalWithNotes().ID should not be empty")
			}
			if ext.Created.IsZero() {
				t.Error("NewExternalWithNotes().Created should not be zero")
			}
		})
	}
}

func TestExternal_ContentHashComputedFromSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "simple source",
			source: "Hardy & Wright (1979)",
		},
		{
			name:   "URL source",
			source: "https://example.com/paper.pdf",
		},
		{
			name:   "long source",
			source: "Smith, J. et al. (2023). On the distribution of prime numbers in arithmetic progressions. Journal of Number Theory, 234(5), 1234-1289. DOI: 10.1000/example",
		},
		{
			name:   "unicode source",
			source: "Müller, H. (2020). Über die Verteilung der Primzahlen",
		},
		{
			name:   "empty source edge case",
			source: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, err := node.NewExternal("Test", tt.source)
			if err != nil {
				t.Fatalf("NewExternal() unexpected error: %v", err)
			}

			// Compute expected hash directly
			sum := sha256.Sum256([]byte(tt.source))
			expectedHash := hex.EncodeToString(sum[:])

			if ext.ContentHash != expectedHash {
				t.Errorf("ContentHash = %q, want %q (computed from source)", ext.ContentHash, expectedHash)
			}
		})
	}
}

func TestExternal_DifferentSourcesDifferentHashes(t *testing.T) {
	ext1, err := node.NewExternal("Ref1", "Source One")
	if err != nil {
		t.Fatalf("NewExternal() ext1 unexpected error: %v", err)
	}
	ext2, err := node.NewExternal("Ref1", "Source Two")
	if err != nil {
		t.Fatalf("NewExternal() ext2 unexpected error: %v", err)
	}

	if ext1.ContentHash == ext2.ContentHash {
		t.Error("Different sources should produce different content hashes")
	}
}

func TestExternal_SameSourceSameHash(t *testing.T) {
	source := "Identical source text"
	ext1, err := node.NewExternal("Name1", source)
	if err != nil {
		t.Fatalf("NewExternal() ext1 unexpected error: %v", err)
	}
	ext2, err := node.NewExternal("Name2", source)
	if err != nil {
		t.Fatalf("NewExternal() ext2 unexpected error: %v", err)
	}

	if ext1.ContentHash != ext2.ContentHash {
		t.Error("Same sources should produce identical content hashes regardless of name")
	}
}

func TestExternal_UniqueIDs(t *testing.T) {
	// Create multiple externals and verify IDs are unique
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		ext, err := node.NewExternal("Test", "Source")
		if err != nil {
			t.Fatalf("NewExternal() iteration %d unexpected error: %v", i, err)
		}
		if seen[ext.ID] {
			t.Errorf("Duplicate ID generated: %s", ext.ID)
		}
		seen[ext.ID] = true
	}
}

func TestExternal_JSONSerialization(t *testing.T) {
	tests := []struct {
		name   string
		ext    func() (*node.External, error)
	}{
		{
			name: "basic external",
			ext: func() (*node.External, error) {
				return node.NewExternal("Test Theorem", "https://example.com/proof")
			},
		},
		{
			name: "external with notes",
			ext: func() (*node.External, error) {
				return node.NewExternalWithNotes("Important Result", "Smith (2023)", "Key lemma for main proof")
			},
		},
		{
			name: "external with unicode",
			ext: func() (*node.External, error) {
				return node.NewExternalWithNotes("Gödel", "Über formal unentscheidbare Sätze", "Gödel's paper")
			},
		},
		{
			name: "external with empty notes",
			ext: func() (*node.External, error) {
				return node.NewExternalWithNotes("AC", "Axiom of Choice", "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original, err := tt.ext()
			if err != nil {
				t.Fatalf("ext() unexpected error: %v", err)
			}

			// Marshal to JSON
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("json.Marshal error: %v", err)
			}

			// Unmarshal from JSON
			var restored node.External
			err = json.Unmarshal(data, &restored)
			if err != nil {
				t.Fatalf("json.Unmarshal error: %v", err)
			}

			// Compare fields
			if restored.ID != original.ID {
				t.Errorf("ID mismatch: got %q, want %q", restored.ID, original.ID)
			}
			if restored.Name != original.Name {
				t.Errorf("Name mismatch: got %q, want %q", restored.Name, original.Name)
			}
			if restored.Source != original.Source {
				t.Errorf("Source mismatch: got %q, want %q", restored.Source, original.Source)
			}
			if restored.ContentHash != original.ContentHash {
				t.Errorf("ContentHash mismatch: got %q, want %q", restored.ContentHash, original.ContentHash)
			}
			if restored.Notes != original.Notes {
				t.Errorf("Notes mismatch: got %q, want %q", restored.Notes, original.Notes)
			}
			if !restored.Created.Equal(original.Created) {
				t.Errorf("Created mismatch: got %v, want %v", restored.Created, original.Created)
			}
		})
	}
}

func TestExternal_JSONFieldNames(t *testing.T) {
	ext, err := node.NewExternalWithNotes("Test", "Source", "Notes")
	if err != nil {
		t.Fatalf("NewExternalWithNotes() unexpected error: %v", err)
	}

	data, err := json.Marshal(ext)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	// Parse as generic JSON to check field names
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("json.Unmarshal to map error: %v", err)
	}

	// Check expected field names exist (using snake_case as per Go conventions)
	expectedFields := []string{"id", "name", "source", "content_hash", "created", "notes"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("Expected JSON field %q not found. JSON: %s", field, string(data))
		}
	}
}

func TestExternal_Validation_EmptyName(t *testing.T) {
	tests := []struct {
		name    string
		refName string
		source  string
	}{
		{
			name:    "empty name",
			refName: "",
			source:  "Valid Source",
		},
		{
			name:    "whitespace only name",
			refName: "   ",
			source:  "Valid Source",
		},
		{
			name:    "tab only name",
			refName: "\t",
			source:  "Valid Source",
		},
		{
			name:    "newline only name",
			refName: "\n",
			source:  "Valid Source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, createErr := node.NewExternal(tt.refName, tt.source)
			if createErr != nil {
				t.Fatalf("NewExternal() unexpected error: %v", createErr)
			}
			err := ext.Validate()
			if err == nil {
				t.Error("Validate() should return error for empty/whitespace name")
			}
		})
	}
}

func TestExternal_Validation_EmptySource(t *testing.T) {
	tests := []struct {
		name    string
		refName string
		source  string
	}{
		{
			name:    "empty source",
			refName: "Valid Name",
			source:  "",
		},
		{
			name:    "whitespace only source",
			refName: "Valid Name",
			source:  "   ",
		},
		{
			name:    "tab only source",
			refName: "Valid Name",
			source:  "\t",
		},
		{
			name:    "newline only source",
			refName: "Valid Name",
			source:  "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, createErr := node.NewExternal(tt.refName, tt.source)
			if createErr != nil {
				t.Fatalf("NewExternal() unexpected error: %v", createErr)
			}
			err := ext.Validate()
			if err == nil {
				t.Error("Validate() should return error for empty/whitespace source")
			}
		})
	}
}

func TestExternal_Validation_BothEmpty(t *testing.T) {
	ext, err := node.NewExternal("", "")
	if err != nil {
		t.Fatalf("NewExternal() unexpected error: %v", err)
	}
	err = ext.Validate()
	if err == nil {
		t.Error("Validate() should return error when both name and source are empty")
	}
}

func TestExternal_Validation_Valid(t *testing.T) {
	tests := []struct {
		name    string
		refName string
		source  string
		notes   string
	}{
		{
			name:    "basic valid",
			refName: "Theorem",
			source:  "Citation",
			notes:   "",
		},
		{
			name:    "with notes",
			refName: "Lemma 3.1",
			source:  "Smith (2023)",
			notes:   "Important for section 4",
		},
		{
			name:    "minimal",
			refName: "A",
			source:  "B",
			notes:   "",
		},
		{
			name:    "unicode valid",
			refName: "Théorème",
			source:  "Référence française",
			notes:   "Note en français",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ext *node.External
			var createErr error
			if tt.notes == "" {
				ext, createErr = node.NewExternal(tt.refName, tt.source)
			} else {
				ext, createErr = node.NewExternalWithNotes(tt.refName, tt.source, tt.notes)
			}
			if createErr != nil {
				t.Fatalf("NewExternal() unexpected error: %v", createErr)
			}

			err := ext.Validate()
			if err != nil {
				t.Errorf("Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestExternal_CreatedTimestamp(t *testing.T) {
	ext1, err := node.NewExternal("First", "Source1")
	if err != nil {
		t.Fatalf("NewExternal() ext1 unexpected error: %v", err)
	}

	// Small delay to ensure different timestamps
	ext2, err := node.NewExternal("Second", "Source2")
	if err != nil {
		t.Fatalf("NewExternal() ext2 unexpected error: %v", err)
	}

	// Both should have non-zero timestamps
	if ext1.Created.IsZero() {
		t.Error("ext1.Created should not be zero")
	}
	if ext2.Created.IsZero() {
		t.Error("ext2.Created should not be zero")
	}

	// ext2 should be created at the same time or after ext1
	if ext2.Created.Before(ext1.Created) {
		t.Errorf("ext2.Created (%v) should not be before ext1.Created (%v)",
			ext2.Created, ext1.Created)
	}
}
