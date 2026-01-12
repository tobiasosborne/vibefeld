// Package node_test contains external tests for the node package.
package node_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
)

// TestNewDefinition tests the NewDefinition constructor.
func TestNewDefinition(t *testing.T) {
	tests := []struct {
		name        string
		defName     string
		content     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid definition",
			defName: "prime number",
			content: "A natural number greater than 1 that has no positive divisors other than 1 and itself.",
			wantErr: false,
		},
		{
			name:    "simple definition",
			defName: "even",
			content: "An integer divisible by 2.",
			wantErr: false,
		},
		{
			name:    "definition with math notation",
			defName: "norm",
			content: "For a vector v, ||v|| denotes its Euclidean norm.",
			wantErr: false,
		},
		{
			name:    "definition with unicode",
			defName: "epsilon-delta",
			content: "For all epsilon > 0, there exists delta > 0 such that...",
			wantErr: false,
		},
		{
			name:        "empty name",
			defName:     "",
			content:     "Some content here.",
			wantErr:     true,
			errContains: "name",
		},
		{
			name:        "whitespace only name",
			defName:     "   ",
			content:     "Some content here.",
			wantErr:     true,
			errContains: "name",
		},
		{
			name:        "empty content",
			defName:     "empty",
			content:     "",
			wantErr:     true,
			errContains: "content",
		},
		{
			name:        "whitespace only content",
			defName:     "whitespace",
			content:     "   \t\n  ",
			wantErr:     true,
			errContains: "content",
		},
		{
			name:        "both empty",
			defName:     "",
			content:     "",
			wantErr:     true,
			errContains: "", // could be name or content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := node.NewDefinition(tt.defName, tt.content)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewDefinition(%q, %q) expected error, got nil", tt.defName, tt.content)
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("NewDefinition(%q, %q) error = %q, want error containing %q",
						tt.defName, tt.content, err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewDefinition(%q, %q) unexpected error: %v", tt.defName, tt.content, err)
			}

			// Verify the definition has correct fields
			if def.Name != tt.defName {
				t.Errorf("NewDefinition() Name = %q, want %q", def.Name, tt.defName)
			}
			if def.Content != tt.content {
				t.Errorf("NewDefinition() Content = %q, want %q", def.Content, tt.content)
			}
			if def.ID == "" {
				t.Error("NewDefinition() ID is empty, want non-empty")
			}
			if def.ContentHash == "" {
				t.Error("NewDefinition() ContentHash is empty, want non-empty")
			}
			if def.Created.IsZero() {
				t.Error("NewDefinition() Created is zero, want non-zero timestamp")
			}
		})
	}
}

// TestDefinitionContentHash tests that content hashes are computed correctly.
func TestDefinitionContentHash(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "simple content",
			content: "A simple definition.",
		},
		{
			name:    "content with newlines",
			content: "Line 1\nLine 2\nLine 3",
		},
		{
			name:    "content with special characters",
			content: "Let f: X -> Y be a function such that f(x) = y.",
		},
		{
			name:    "unicode content",
			content: "For all epsilon > 0, there exists delta > 0.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := node.NewDefinition("test-def", tt.content)
			if err != nil {
				t.Fatalf("NewDefinition() unexpected error: %v", err)
			}

			// Manually compute expected hash for the content
			expectedHash := computeSHA256(tt.content)

			if def.ContentHash != expectedHash {
				t.Errorf("ContentHash = %q, want %q", def.ContentHash, expectedHash)
			}

			// Hash should be 64 characters (SHA256 hex)
			if len(def.ContentHash) != 64 {
				t.Errorf("ContentHash length = %d, want 64", len(def.ContentHash))
			}
		})
	}
}

// TestDefinitionContentHashDeterministic tests that the same content always produces the same hash.
func TestDefinitionContentHashDeterministic(t *testing.T) {
	name := "deterministic-test"
	content := "This is the test content for deterministic hashing."

	// Create multiple definitions with the same content
	hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		def, err := node.NewDefinition(name, content)
		if err != nil {
			t.Fatalf("NewDefinition() iteration %d unexpected error: %v", i, err)
		}
		hashes[i] = def.ContentHash
	}

	// All hashes should be identical
	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("ContentHash is not deterministic: hash[0]=%q, hash[%d]=%q", hashes[0], i, hashes[i])
		}
	}
}

// TestDefinitionContentHashUniqueness tests that different content produces different hashes.
func TestDefinitionContentHashUniqueness(t *testing.T) {
	contents := []string{
		"Definition A: first version",
		"Definition A: second version",
		"Definition B",
		"Definition A: first version ", // trailing space
		" Definition A: first version", // leading space
	}

	hashes := make(map[string]string) // content -> hash
	for _, content := range contents {
		def, err := node.NewDefinition("test", content)
		if err != nil {
			t.Fatalf("NewDefinition() unexpected error for content %q: %v", content, err)
		}

		if existing, ok := hashes[def.ContentHash]; ok {
			t.Errorf("Hash collision: %q and %q both hash to %q", existing, content, def.ContentHash)
		}
		hashes[def.ContentHash] = content
	}
}

// TestDefinitionJSONSerialization tests JSON marshaling and unmarshaling.
func TestDefinitionJSONSerialization(t *testing.T) {
	tests := []struct {
		name    string
		defName string
		content string
	}{
		{
			name:    "simple definition",
			defName: "test-def",
			content: "A simple test definition.",
		},
		{
			name:    "definition with quotes",
			defName: "quoted",
			content: `A "quoted" definition.`,
		},
		{
			name:    "definition with newlines",
			defName: "multiline",
			content: "Line 1\nLine 2\nLine 3",
		},
		{
			name:    "definition with unicode",
			defName: "greek",
			content: "Alpha, beta, gamma",
		},
		{
			name:    "definition with backslashes",
			defName: "latex",
			content: `\frac{a}{b} = \int_0^1 f(x) dx`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create original definition
			original, err := node.NewDefinition(tt.defName, tt.content)
			if err != nil {
				t.Fatalf("NewDefinition() unexpected error: %v", err)
			}

			// Marshal to JSON
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("json.Marshal() error: %v", err)
			}

			// Unmarshal back
			var restored node.Definition
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatalf("json.Unmarshal() error: %v", err)
			}

			// Verify all fields match
			if restored.ID != original.ID {
				t.Errorf("Restored ID = %q, want %q", restored.ID, original.ID)
			}
			if restored.Name != original.Name {
				t.Errorf("Restored Name = %q, want %q", restored.Name, original.Name)
			}
			if restored.Content != original.Content {
				t.Errorf("Restored Content = %q, want %q", restored.Content, original.Content)
			}
			if restored.ContentHash != original.ContentHash {
				t.Errorf("Restored ContentHash = %q, want %q", restored.ContentHash, original.ContentHash)
			}
			if !restored.Created.Equal(original.Created) {
				t.Errorf("Restored Created = %v, want %v", restored.Created, original.Created)
			}
		})
	}
}

// TestDefinitionJSONRoundTrip tests multiple JSON round trips preserve data.
func TestDefinitionJSONRoundTrip(t *testing.T) {
	original, err := node.NewDefinition("round-trip", "Testing round trip serialization.")
	if err != nil {
		t.Fatalf("NewDefinition() unexpected error: %v", err)
	}

	current := original
	for i := 0; i < 5; i++ {
		data, err := json.Marshal(current)
		if err != nil {
			t.Fatalf("json.Marshal() iteration %d error: %v", i, err)
		}

		var next node.Definition
		if err := json.Unmarshal(data, &next); err != nil {
			t.Fatalf("json.Unmarshal() iteration %d error: %v", i, err)
		}

		if next.ContentHash != original.ContentHash {
			t.Errorf("Round trip %d: ContentHash changed from %q to %q", i, original.ContentHash, next.ContentHash)
		}

		current = &next
	}
}

// TestDefinitionValidation tests the Validate method if it exists.
func TestDefinitionValidation(t *testing.T) {
	tests := []struct {
		name        string
		defName     string
		content     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid definition",
			defName: "valid",
			content: "A valid definition with content.",
			wantErr: false,
		},
		{
			name:        "empty name validation",
			defName:     "",
			content:     "Has content but no name.",
			wantErr:     true,
			errContains: "name",
		},
		{
			name:        "empty content validation",
			defName:     "has-name",
			content:     "",
			wantErr:     true,
			errContains: "content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.NewDefinition(tt.defName, tt.content)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewDefinition(%q, %q) expected error, got nil", tt.defName, tt.content)
				}
			} else {
				if err != nil {
					t.Errorf("NewDefinition(%q, %q) unexpected error: %v", tt.defName, tt.content, err)
				}
			}
		})
	}
}

// TestDefinitionEquality tests equality comparison based on content hash.
func TestDefinitionEquality(t *testing.T) {
	t.Run("same content equals", func(t *testing.T) {
		def1, err := node.NewDefinition("def-a", "Same content here.")
		if err != nil {
			t.Fatalf("NewDefinition() def1 error: %v", err)
		}

		def2, err := node.NewDefinition("def-b", "Same content here.")
		if err != nil {
			t.Fatalf("NewDefinition() def2 error: %v", err)
		}

		// Same content should have same content hash
		if def1.ContentHash != def2.ContentHash {
			t.Errorf("Same content should have equal ContentHash: %q != %q",
				def1.ContentHash, def2.ContentHash)
		}

		// But different IDs (they are different definitions)
		// Note: IDs might be the same if based on content hash, or different if using UUID
		// The test should verify they can coexist
	})

	t.Run("different content not equals", func(t *testing.T) {
		def1, err := node.NewDefinition("def-a", "Content version 1")
		if err != nil {
			t.Fatalf("NewDefinition() def1 error: %v", err)
		}

		def2, err := node.NewDefinition("def-a", "Content version 2")
		if err != nil {
			t.Fatalf("NewDefinition() def2 error: %v", err)
		}

		if def1.ContentHash == def2.ContentHash {
			t.Errorf("Different content should have different ContentHash: both are %q",
				def1.ContentHash)
		}
	})

	t.Run("same name different content", func(t *testing.T) {
		def1, err := node.NewDefinition("shared-name", "First definition content.")
		if err != nil {
			t.Fatalf("NewDefinition() def1 error: %v", err)
		}

		def2, err := node.NewDefinition("shared-name", "Second definition content.")
		if err != nil {
			t.Fatalf("NewDefinition() def2 error: %v", err)
		}

		// Same name but different content = different hashes
		if def1.ContentHash == def2.ContentHash {
			t.Errorf("Same name but different content should have different hashes")
		}
	})
}

// TestDefinitionEqualMethod tests the Equal method if implemented.
func TestDefinitionEqualMethod(t *testing.T) {
	def1, err := node.NewDefinition("equal-test", "Test content for equality.")
	if err != nil {
		t.Fatalf("NewDefinition() def1 error: %v", err)
	}

	def2, err := node.NewDefinition("equal-test", "Test content for equality.")
	if err != nil {
		t.Fatalf("NewDefinition() def2 error: %v", err)
	}

	def3, err := node.NewDefinition("different", "Different content entirely.")
	if err != nil {
		t.Fatalf("NewDefinition() def3 error: %v", err)
	}

	// Test Equal method if it exists
	if def1.Equal(def2) != true {
		t.Errorf("def1.Equal(def2) = false, want true (same content)")
	}

	if def1.Equal(def3) != false {
		t.Errorf("def1.Equal(def3) = true, want false (different content)")
	}

	// Equal should be symmetric
	if def2.Equal(def1) != def1.Equal(def2) {
		t.Errorf("Equal is not symmetric")
	}

	// Equal to self
	if def1.Equal(def1) != true {
		t.Errorf("def1.Equal(def1) = false, want true (reflexive)")
	}
}

// TestDefinitionIDGeneration tests that IDs are generated correctly.
func TestDefinitionIDGeneration(t *testing.T) {
	def1, err := node.NewDefinition("test-1", "First definition.")
	if err != nil {
		t.Fatalf("NewDefinition() def1 error: %v", err)
	}

	def2, err := node.NewDefinition("test-2", "Second definition.")
	if err != nil {
		t.Fatalf("NewDefinition() def2 error: %v", err)
	}

	// IDs should be non-empty
	if def1.ID == "" {
		t.Error("def1.ID is empty")
	}
	if def2.ID == "" {
		t.Error("def2.ID is empty")
	}

	// IDs should be different for different definitions
	if def1.ID == def2.ID {
		t.Errorf("Different definitions have same ID: %q", def1.ID)
	}
}

// TestDefinitionTimestamp tests that Created timestamp is set correctly.
func TestDefinitionTimestamp(t *testing.T) {
	def, err := node.NewDefinition("timestamp-test", "Testing timestamp.")
	if err != nil {
		t.Fatalf("NewDefinition() error: %v", err)
	}

	// Timestamp should not be zero
	if def.Created.IsZero() {
		t.Error("Created timestamp is zero")
	}

	// Create another definition and verify it has a later or equal timestamp
	def2, err := node.NewDefinition("timestamp-test-2", "Second timestamp test.")
	if err != nil {
		t.Fatalf("NewDefinition() error: %v", err)
	}

	// def2 should be created at the same time or after def1
	if def2.Created.Before(def.Created) {
		t.Errorf("Second definition Created (%v) is before first (%v)",
			def2.Created, def.Created)
	}
}

// Helper functions

// computeSHA256 computes the SHA256 hash of content and returns it as a hex string.
func computeSHA256(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
