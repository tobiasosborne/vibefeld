package node_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
)

// TestNewAssumption verifies that the basic constructor creates
// an Assumption with the correct statement and computed hash.
func TestNewAssumption(t *testing.T) {
	tests := []struct {
		name      string
		statement string
	}{
		{
			name:      "simple statement",
			statement: "x is a positive integer",
		},
		{
			name:      "mathematical statement",
			statement: "For all n in N, n >= 0",
		},
		{
			name:      "statement with special characters",
			statement: "Let f: R -> R be continuous",
		},
		{
			name:      "long statement",
			statement: "Assume that the function g is differentiable on the open interval (a, b) and continuous on the closed interval [a, b]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := node.NewAssumption(tt.statement)
			if err != nil {
				t.Fatalf("NewAssumption() unexpected error: %v", err)
			}

			if a.Statement != tt.statement {
				t.Errorf("Statement = %q, want %q", a.Statement, tt.statement)
			}

			if a.ID == "" {
				t.Error("ID should not be empty")
			}

			if a.ContentHash == "" {
				t.Error("ContentHash should not be empty")
			}

			if a.Created.IsZero() {
				t.Error("Created timestamp should not be zero")
			}

			if a.Justification != "" {
				t.Errorf("Justification should be empty for basic constructor, got %q", a.Justification)
			}
		})
	}
}

// TestNewAssumptionWithJustification verifies that the constructor
// with justification creates an Assumption with all fields set.
func TestNewAssumptionWithJustification(t *testing.T) {
	tests := []struct {
		name          string
		statement     string
		justification string
	}{
		{
			name:          "axiom justification",
			statement:     "x + 0 = x",
			justification: "additive identity axiom",
		},
		{
			name:          "given in problem",
			statement:     "n is prime",
			justification: "given in problem statement",
		},
		{
			name:          "well-known result",
			statement:     "sqrt(2) is irrational",
			justification: "well-known result from classical mathematics",
		},
		{
			name:          "empty justification",
			statement:     "a = b",
			justification: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := node.NewAssumptionWithJustification(tt.statement, tt.justification)
			if err != nil {
				t.Fatalf("NewAssumptionWithJustification() unexpected error: %v", err)
			}

			if a.Statement != tt.statement {
				t.Errorf("Statement = %q, want %q", a.Statement, tt.statement)
			}

			if a.Justification != tt.justification {
				t.Errorf("Justification = %q, want %q", a.Justification, tt.justification)
			}

			if a.ID == "" {
				t.Error("ID should not be empty")
			}

			if a.ContentHash == "" {
				t.Error("ContentHash should not be empty")
			}

			if a.Created.IsZero() {
				t.Error("Created timestamp should not be zero")
			}
		})
	}
}

// TestAssumptionContentHash verifies that the content hash is
// computed correctly from the statement.
func TestAssumptionContentHash(t *testing.T) {
	tests := []struct {
		name      string
		statement string
	}{
		{
			name:      "simple statement",
			statement: "x > 0",
		},
		{
			name:      "statement with unicode",
			statement: "alpha + beta = gamma",
		},
		{
			name:      "empty statement edge case",
			statement: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := node.NewAssumption(tt.statement)
			if err != nil {
				t.Fatalf("NewAssumption() unexpected error: %v", err)
			}

			// Compute expected hash manually
			sum := sha256.Sum256([]byte(tt.statement))
			expectedHash := hex.EncodeToString(sum[:])

			if a.ContentHash != expectedHash {
				t.Errorf("ContentHash = %q, want %q", a.ContentHash, expectedHash)
			}
		})
	}
}

// TestAssumptionContentHashDeterministic verifies that the same
// statement always produces the same hash.
func TestAssumptionContentHashDeterministic(t *testing.T) {
	statement := "Let epsilon > 0 be given"

	a1, err := node.NewAssumption(statement)
	if err != nil {
		t.Fatalf("NewAssumption() a1 unexpected error: %v", err)
	}
	a2, err := node.NewAssumption(statement)
	if err != nil {
		t.Fatalf("NewAssumption() a2 unexpected error: %v", err)
	}

	if a1.ContentHash != a2.ContentHash {
		t.Errorf("Same statement produced different hashes: %q vs %q", a1.ContentHash, a2.ContentHash)
	}
}

// TestAssumptionContentHashUnique verifies that different statements
// produce different hashes.
func TestAssumptionContentHashUnique(t *testing.T) {
	a1, err := node.NewAssumption("statement one")
	if err != nil {
		t.Fatalf("NewAssumption() a1 unexpected error: %v", err)
	}
	a2, err := node.NewAssumption("statement two")
	if err != nil {
		t.Fatalf("NewAssumption() a2 unexpected error: %v", err)
	}

	if a1.ContentHash == a2.ContentHash {
		t.Error("Different statements should produce different hashes")
	}
}

// TestAssumptionJSONSerialization verifies that Assumption can be
// serialized to and deserialized from JSON correctly.
func TestAssumptionJSONSerialization(t *testing.T) {
	tests := []struct {
		name          string
		statement     string
		justification string
	}{
		{
			name:          "with justification",
			statement:     "x is real",
			justification: "given",
		},
		{
			name:          "without justification",
			statement:     "y is positive",
			justification: "",
		},
		{
			name:          "special characters",
			statement:     "f(x) = x^2 for x in [0, 1]",
			justification: "definition of f",
		},
		{
			name:          "unicode content",
			statement:     "delta < epsilon",
			justification: "epsilon-delta definition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original, err := node.NewAssumptionWithJustification(tt.statement, tt.justification)
			if err != nil {
				t.Fatalf("NewAssumptionWithJustification() unexpected error: %v", err)
			}

			// Serialize to JSON
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			// Deserialize from JSON
			var restored node.Assumption
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Verify all fields match
			if restored.ID != original.ID {
				t.Errorf("ID: got %q, want %q", restored.ID, original.ID)
			}
			if restored.Statement != original.Statement {
				t.Errorf("Statement: got %q, want %q", restored.Statement, original.Statement)
			}
			if restored.ContentHash != original.ContentHash {
				t.Errorf("ContentHash: got %q, want %q", restored.ContentHash, original.ContentHash)
			}
			if restored.Justification != original.Justification {
				t.Errorf("Justification: got %q, want %q", restored.Justification, original.Justification)
			}
			if !restored.Created.Equal(original.Created) {
				t.Errorf("Created: got %v, want %v", restored.Created, original.Created)
			}
		})
	}
}

// TestAssumptionJSONRoundTrip verifies that JSON serialization
// preserves all data through multiple round trips.
func TestAssumptionJSONRoundTrip(t *testing.T) {
	original, err := node.NewAssumptionWithJustification(
		"The sequence converges uniformly",
		"by Weierstrass M-test",
	)
	if err != nil {
		t.Fatalf("NewAssumptionWithJustification() unexpected error: %v", err)
	}

	// First round trip
	data1, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("First marshal failed: %v", err)
	}

	var restored1 node.Assumption
	if err := json.Unmarshal(data1, &restored1); err != nil {
		t.Fatalf("First unmarshal failed: %v", err)
	}

	// Second round trip
	data2, err := json.Marshal(restored1)
	if err != nil {
		t.Fatalf("Second marshal failed: %v", err)
	}

	var restored2 node.Assumption
	if err := json.Unmarshal(data2, &restored2); err != nil {
		t.Fatalf("Second unmarshal failed: %v", err)
	}

	// Verify data is unchanged after two round trips
	if restored2.ID != original.ID {
		t.Errorf("ID changed after round trips")
	}
	if restored2.Statement != original.Statement {
		t.Errorf("Statement changed after round trips")
	}
	if restored2.ContentHash != original.ContentHash {
		t.Errorf("ContentHash changed after round trips")
	}
	if restored2.Justification != original.Justification {
		t.Errorf("Justification changed after round trips")
	}
}

// TestAssumptionValidation verifies that Validate correctly
// identifies invalid Assumptions.
func TestAssumptionValidation(t *testing.T) {
	tests := []struct {
		name        string
		statement   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid statement",
			statement: "x > 0",
			wantErr:   false,
		},
		{
			name:        "empty statement",
			statement:   "",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:        "whitespace only statement",
			statement:   "   ",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:        "tab and newline only",
			statement:   "\t\n",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:      "statement with leading whitespace is valid",
			statement: "  valid statement",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, createErr := node.NewAssumption(tt.statement)
			if createErr != nil {
				t.Fatalf("NewAssumption() unexpected error: %v", createErr)
			}
			err := a.Validate()

			if tt.wantErr {
				if err == nil {
					t.Error("Validate() should return error for invalid assumption")
				} else if tt.errContains != "" {
					errStr := err.Error()
					if !contains(errStr, tt.errContains) {
						t.Errorf("Error %q should contain %q", errStr, tt.errContains)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

// TestAssumptionIDGeneration verifies that each Assumption
// receives a unique ID.
func TestAssumptionIDGeneration(t *testing.T) {
	seen := make(map[string]bool)

	// Create multiple assumptions and verify unique IDs
	for i := 0; i < 100; i++ {
		a, err := node.NewAssumption("same statement for all")
		if err != nil {
			t.Fatalf("NewAssumption() iteration %d unexpected error: %v", i, err)
		}
		if seen[a.ID] {
			t.Errorf("Duplicate ID generated: %s", a.ID)
		}
		seen[a.ID] = true
	}
}

// TestAssumptionIDFormat verifies that generated IDs have
// the expected format.
func TestAssumptionIDFormat(t *testing.T) {
	a, err := node.NewAssumption("test statement")
	if err != nil {
		t.Fatalf("NewAssumption() unexpected error: %v", err)
	}

	if len(a.ID) == 0 {
		t.Error("ID should not be empty")
	}

	// IDs should be non-empty strings (exact format depends on implementation)
	// At minimum, they should be printable ASCII
	for _, c := range a.ID {
		if c < 32 || c > 126 {
			t.Errorf("ID contains non-printable character: %q", a.ID)
			break
		}
	}
}

// contains checks if substr is in s (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(substr) == 0 ||
		(len(s) > 0 && containsIgnoreCase(s, substr)))
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
