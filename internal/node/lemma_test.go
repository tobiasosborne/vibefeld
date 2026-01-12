package node_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/types"
)

// TestNewLemma_Valid tests creating a lemma with valid inputs
func TestNewLemma_Valid(t *testing.T) {
	tests := []struct {
		name         string
		statement    string
		sourceNodeID string
	}{
		{
			name:         "simple lemma",
			statement:    "If x > 0 and y > 0, then x + y > 0",
			sourceNodeID: "1",
		},
		{
			name:         "lemma from nested node",
			statement:    "The sum of two even numbers is even",
			sourceNodeID: "1.2.3",
		},
		{
			name:         "lemma with special characters",
			statement:    "For all epsilon > 0, there exists delta > 0 such that |x - a| < delta implies |f(x) - f(a)| < epsilon",
			sourceNodeID: "1.1",
		},
		{
			name:         "lemma with unicode",
			statement:    "Let alpha, beta be real numbers with alpha less than beta",
			sourceNodeID: "1.5.2.1",
		},
		{
			name:         "lemma from deep node",
			statement:    "By induction hypothesis, P(k) holds",
			sourceNodeID: "1.2.3.4.5.6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceID, err := types.Parse(tt.sourceNodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) error: %v", tt.sourceNodeID, err)
			}

			lemma, err := node.NewLemma(tt.statement, sourceID)
			if err != nil {
				t.Fatalf("NewLemma() unexpected error: %v", err)
			}

			// Verify statement is preserved
			if lemma.Statement != tt.statement {
				t.Errorf("Statement = %q, want %q", lemma.Statement, tt.statement)
			}

			// Verify source node ID is preserved
			if lemma.SourceNodeID.String() != tt.sourceNodeID {
				t.Errorf("SourceNodeID = %q, want %q", lemma.SourceNodeID.String(), tt.sourceNodeID)
			}

			// Verify ID is generated (non-empty)
			if lemma.ID == "" {
				t.Error("ID should not be empty")
			}

			// Verify ContentHash is generated (non-empty)
			if lemma.ContentHash == "" {
				t.Error("ContentHash should not be empty")
			}

			// Verify Created timestamp is set (not zero)
			if lemma.Created.IsZero() {
				t.Error("Created timestamp should not be zero")
			}

			// Verify Proof is empty initially
			if lemma.Proof != "" {
				t.Errorf("Proof should be empty initially, got %q", lemma.Proof)
			}
		})
	}
}

// TestNewLemma_EmptyStatement tests that empty statement is rejected
func TestNewLemma_EmptyStatement(t *testing.T) {
	tests := []struct {
		name      string
		statement string
	}{
		{"empty string", ""},
		{"only spaces", "   "},
		{"only tabs", "\t\t"},
		{"only newlines", "\n\n"},
		{"mixed whitespace", " \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceID, _ := types.Parse("1")
			_, err := node.NewLemma(tt.statement, sourceID)
			if err == nil {
				t.Errorf("NewLemma(%q, _) should return error for empty statement", tt.statement)
			}
		})
	}
}

// TestNewLemma_InvalidSourceNodeID tests that invalid source node ID is rejected
func TestNewLemma_InvalidSourceNodeID(t *testing.T) {
	// Use zero value NodeID
	var zeroNodeID types.NodeID

	_, err := node.NewLemma("Valid statement", zeroNodeID)
	if err == nil {
		t.Error("NewLemma with zero NodeID should return error")
	}
}

// TestLemma_SetProof tests setting the proof on a lemma
func TestLemma_SetProof(t *testing.T) {
	tests := []struct {
		name  string
		proof string
	}{
		{
			name:  "simple proof",
			proof: "This follows directly from the definition.",
		},
		{
			name:  "multi-line proof",
			proof: "Step 1: Assume x > 0.\nStep 2: By definition, x + y > y.\nStep 3: Since y > 0, x + y > 0.",
		},
		{
			name:  "proof with special characters",
			proof: "Let epsilon = delta/2. Then |f(x) - L| < epsilon/2 < epsilon.",
		},
		{
			name:  "empty proof clears previous",
			proof: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceID, _ := types.Parse("1")
			lemma, err := node.NewLemma("Test statement", sourceID)
			if err != nil {
				t.Fatalf("NewLemma() error: %v", err)
			}

			lemma.SetProof(tt.proof)

			if lemma.Proof != tt.proof {
				t.Errorf("Proof = %q, want %q", lemma.Proof, tt.proof)
			}
		})
	}
}

// TestLemma_SetProof_Overwrites tests that SetProof overwrites previous proof
func TestLemma_SetProof_Overwrites(t *testing.T) {
	sourceID, _ := types.Parse("1")
	lemma, err := node.NewLemma("Test statement", sourceID)
	if err != nil {
		t.Fatalf("NewLemma() error: %v", err)
	}

	// Set initial proof
	lemma.SetProof("Initial proof")
	if lemma.Proof != "Initial proof" {
		t.Errorf("Proof = %q, want %q", lemma.Proof, "Initial proof")
	}

	// Overwrite with new proof
	lemma.SetProof("Updated proof")
	if lemma.Proof != "Updated proof" {
		t.Errorf("Proof = %q, want %q", lemma.Proof, "Updated proof")
	}

	// Clear proof
	lemma.SetProof("")
	if lemma.Proof != "" {
		t.Errorf("Proof = %q, want empty string", lemma.Proof)
	}
}

// TestLemma_ContentHash_Deterministic tests that content hash is deterministic
func TestLemma_ContentHash_Deterministic(t *testing.T) {
	sourceID, _ := types.Parse("1.2.3")
	statement := "The sum of two prime numbers greater than 2 is even"

	lemma1, err := node.NewLemma(statement, sourceID)
	if err != nil {
		t.Fatalf("NewLemma() error: %v", err)
	}

	lemma2, err := node.NewLemma(statement, sourceID)
	if err != nil {
		t.Fatalf("NewLemma() error: %v", err)
	}

	if lemma1.ContentHash != lemma2.ContentHash {
		t.Errorf("ContentHash not deterministic: %q != %q", lemma1.ContentHash, lemma2.ContentHash)
	}
}

// TestLemma_ContentHash_FromStatement tests that content hash is computed from statement
func TestLemma_ContentHash_FromStatement(t *testing.T) {
	sourceID, _ := types.Parse("1")
	statement := "Test statement for hash verification"

	lemma, err := node.NewLemma(statement, sourceID)
	if err != nil {
		t.Fatalf("NewLemma() error: %v", err)
	}

	// Manually compute expected hash
	sum := sha256.Sum256([]byte(statement))
	expectedHash := hex.EncodeToString(sum[:])

	if lemma.ContentHash != expectedHash {
		t.Errorf("ContentHash = %q, want %q", lemma.ContentHash, expectedHash)
	}
}

// TestLemma_ContentHash_Format tests content hash format
func TestLemma_ContentHash_Format(t *testing.T) {
	sourceID, _ := types.Parse("1")
	lemma, err := node.NewLemma("Any statement", sourceID)
	if err != nil {
		t.Fatalf("NewLemma() error: %v", err)
	}

	// SHA256 produces 64 hex characters
	if len(lemma.ContentHash) != 64 {
		t.Errorf("ContentHash length = %d, want 64", len(lemma.ContentHash))
	}

	// Should be lowercase hex
	if strings.ToLower(lemma.ContentHash) != lemma.ContentHash {
		t.Errorf("ContentHash should be lowercase hex: %q", lemma.ContentHash)
	}

	// Should only contain valid hex characters
	for _, c := range lemma.ContentHash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("ContentHash contains invalid character: %c", c)
		}
	}
}

// TestLemma_ContentHash_DifferentStatements tests different statements produce different hashes
func TestLemma_ContentHash_DifferentStatements(t *testing.T) {
	sourceID, _ := types.Parse("1")

	statements := []string{
		"Statement A",
		"Statement B",
		"Statement A ", // trailing space
		" Statement A", // leading space
		"STATEMENT A",  // different case
	}

	hashes := make(map[string]string)
	for _, stmt := range statements {
		lemma, err := node.NewLemma(stmt, sourceID)
		if err != nil {
			t.Fatalf("NewLemma(%q, _) error: %v", stmt, err)
		}

		if existing, ok := hashes[lemma.ContentHash]; ok {
			t.Errorf("Hash collision between %q and %q: %s", existing, stmt, lemma.ContentHash)
		}
		hashes[lemma.ContentHash] = stmt
	}
}

// TestLemma_JSON_Roundtrip tests JSON serialization and deserialization
func TestLemma_JSON_Roundtrip(t *testing.T) {
	tests := []struct {
		name         string
		statement    string
		sourceNodeID string
		proof        string
	}{
		{
			name:         "simple lemma without proof",
			statement:    "If n is even, then n^2 is even",
			sourceNodeID: "1",
			proof:        "",
		},
		{
			name:         "lemma with proof",
			statement:    "The product of two odd numbers is odd",
			sourceNodeID: "1.2",
			proof:        "Let a = 2k+1 and b = 2m+1. Then ab = 4km + 2k + 2m + 1 = 2(2km + k + m) + 1, which is odd.",
		},
		{
			name:         "lemma with special characters",
			statement:    "For all x in R, |x| >= 0",
			sourceNodeID: "1.3.4.5",
			proof:        "By definition of absolute value: |x| = x if x >= 0, |x| = -x if x < 0. Both cases yield non-negative values.",
		},
		{
			name:         "lemma with newlines",
			statement:    "Line 1\nLine 2\nLine 3",
			sourceNodeID: "1.1.1",
			proof:        "Step 1.\nStep 2.\nConclusion.",
		},
		{
			name:         "lemma with unicode",
			statement:    "alpha + beta = gamma",
			sourceNodeID: "1.2.3",
			proof:        "By Greek letter arithmetic.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceID, err := types.Parse(tt.sourceNodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) error: %v", tt.sourceNodeID, err)
			}

			original, err := node.NewLemma(tt.statement, sourceID)
			if err != nil {
				t.Fatalf("NewLemma() error: %v", err)
			}

			if tt.proof != "" {
				original.SetProof(tt.proof)
			}

			// Marshal to JSON
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("json.Marshal error: %v", err)
			}

			// Unmarshal from JSON
			var restored node.Lemma
			err = json.Unmarshal(data, &restored)
			if err != nil {
				t.Fatalf("json.Unmarshal error: %v", err)
			}

			// Verify all fields match
			if restored.ID != original.ID {
				t.Errorf("ID = %q, want %q", restored.ID, original.ID)
			}
			if restored.Statement != original.Statement {
				t.Errorf("Statement = %q, want %q", restored.Statement, original.Statement)
			}
			if restored.SourceNodeID.String() != original.SourceNodeID.String() {
				t.Errorf("SourceNodeID = %q, want %q", restored.SourceNodeID.String(), original.SourceNodeID.String())
			}
			if restored.ContentHash != original.ContentHash {
				t.Errorf("ContentHash = %q, want %q", restored.ContentHash, original.ContentHash)
			}
			if !restored.Created.Equal(original.Created) {
				t.Errorf("Created = %v, want %v", restored.Created, original.Created)
			}
			if restored.Proof != original.Proof {
				t.Errorf("Proof = %q, want %q", restored.Proof, original.Proof)
			}
		})
	}
}

// TestLemma_JSON_Format tests the JSON output format
func TestLemma_JSON_Format(t *testing.T) {
	sourceID, _ := types.Parse("1.2")
	lemma, err := node.NewLemma("Test statement", sourceID)
	if err != nil {
		t.Fatalf("NewLemma() error: %v", err)
	}
	lemma.SetProof("Test proof")

	data, err := json.Marshal(lemma)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	// Unmarshal into a map to check field names
	var fields map[string]interface{}
	err = json.Unmarshal(data, &fields)
	if err != nil {
		t.Fatalf("json.Unmarshal to map error: %v", err)
	}

	// Check expected fields exist
	expectedFields := []string{"id", "statement", "source_node_id", "content_hash", "created", "proof"}
	for _, field := range expectedFields {
		if _, ok := fields[field]; !ok {
			t.Errorf("JSON missing field %q", field)
		}
	}
}

// TestLemma_JSON_UnmarshalInvalid tests handling of invalid JSON
func TestLemma_JSON_UnmarshalInvalid(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"empty object", "{}"},
		{"malformed json", "{invalid}"},
		{"missing closing brace", `{"id": "test"`},
		{"null", "null"},
		{"array instead of object", "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lemma node.Lemma
			err := json.Unmarshal([]byte(tt.json), &lemma)
			// Either returns an error or results in partially populated struct
			// Both are acceptable - we just verify it doesn't panic
			_ = err
		})
	}
}

// TestLemma_ID_Unique tests that lemma IDs are unique
func TestLemma_ID_Unique(t *testing.T) {
	sourceID, _ := types.Parse("1")
	ids := make(map[string]bool)

	// Create multiple lemmas and verify unique IDs
	for i := 0; i < 100; i++ {
		lemma, err := node.NewLemma("Test statement", sourceID)
		if err != nil {
			t.Fatalf("NewLemma() error at iteration %d: %v", i, err)
		}

		if ids[lemma.ID] {
			t.Errorf("Duplicate ID generated: %q at iteration %d", lemma.ID, i)
		}
		ids[lemma.ID] = true
	}
}

// TestLemma_ID_Format tests that lemma ID has expected format
func TestLemma_ID_Format(t *testing.T) {
	sourceID, _ := types.Parse("1")
	lemma, err := node.NewLemma("Test statement", sourceID)
	if err != nil {
		t.Fatalf("NewLemma() error: %v", err)
	}

	// ID should be non-empty
	if lemma.ID == "" {
		t.Error("ID should not be empty")
	}

	// ID should not contain whitespace
	if strings.ContainsAny(lemma.ID, " \t\n\r") {
		t.Errorf("ID should not contain whitespace: %q", lemma.ID)
	}
}

// TestLemma_Created_NotInFuture tests that Created timestamp is not in the future
func TestLemma_Created_NotInFuture(t *testing.T) {
	sourceID, _ := types.Parse("1")
	lemma, err := node.NewLemma("Test statement", sourceID)
	if err != nil {
		t.Fatalf("NewLemma() error: %v", err)
	}

	now := types.Now()
	if lemma.Created.After(now) {
		t.Errorf("Created timestamp %v is in the future (now: %v)", lemma.Created, now)
	}
}

// TestLemma_Validation_StatementWithLeadingTrailingSpaces tests statement with only internal whitespace
func TestLemma_Validation_StatementWithContent(t *testing.T) {
	sourceID, _ := types.Parse("1")

	// Statements with actual content (even with surrounding whitespace) should be valid
	// unless the implementation trims and rejects
	tests := []struct {
		name        string
		statement   string
		shouldError bool
	}{
		{"normal statement", "This is a valid statement", false},
		{"statement with internal spaces", "This has   multiple   spaces", false},
		{"single character", "x", false},
		{"number only", "42", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.NewLemma(tt.statement, sourceID)
			if tt.shouldError && err == nil {
				t.Errorf("NewLemma(%q, _) should return error", tt.statement)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("NewLemma(%q, _) unexpected error: %v", tt.statement, err)
			}
		})
	}
}
