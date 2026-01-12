package scope

import (
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/types"
)

func mustParseNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("failed to parse node ID %q: %v", s, err)
	}
	return id
}

func TestNewEntry_Valid(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.2.3")
	statement := "Assume x > 0"

	entry, err := NewEntry(nodeID, statement)

	if err != nil {
		t.Fatalf("NewEntry returned unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("NewEntry returned nil entry")
	}
	if entry.NodeID.String() != "1.2.3" {
		t.Errorf("NodeID = %q, want %q", entry.NodeID.String(), "1.2.3")
	}
	if entry.Statement != statement {
		t.Errorf("Statement = %q, want %q", entry.Statement, statement)
	}
	if entry.Introduced.IsZero() {
		t.Error("Introduced timestamp should not be zero")
	}
	if entry.Discharged != nil {
		t.Error("Discharged should be nil for new entry")
	}
}

func TestNewEntry_ValidRoot(t *testing.T) {
	nodeID := mustParseNodeID(t, "1")
	statement := "Base assumption"

	entry, err := NewEntry(nodeID, statement)

	if err != nil {
		t.Fatalf("NewEntry returned unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("NewEntry returned nil entry")
	}
	if entry.NodeID.String() != "1" {
		t.Errorf("NodeID = %q, want %q", entry.NodeID.String(), "1")
	}
}

func TestNewEntry_ValidDeeplyNested(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.2.3.4.5.6")
	statement := "Deep nested assumption"

	entry, err := NewEntry(nodeID, statement)

	if err != nil {
		t.Fatalf("NewEntry returned unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("NewEntry returned nil entry")
	}
	if entry.NodeID.String() != "1.2.3.4.5.6" {
		t.Errorf("NodeID = %q, want %q", entry.NodeID.String(), "1.2.3.4.5.6")
	}
}

func TestNewEntry_EmptyStatement(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.1")
	statement := ""

	entry, err := NewEntry(nodeID, statement)

	if err == nil {
		t.Error("NewEntry should return error for empty statement")
	}
	if entry != nil {
		t.Error("NewEntry should return nil entry for empty statement")
	}
}

func TestNewEntry_WhitespaceOnlyStatement(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.1")

	testCases := []struct {
		name      string
		statement string
	}{
		{"single space", " "},
		{"multiple spaces", "   "},
		{"tab", "\t"},
		{"newline", "\n"},
		{"mixed whitespace", " \t\n "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := NewEntry(nodeID, tc.statement)

			if err == nil {
				t.Errorf("NewEntry should return error for whitespace-only statement %q", tc.statement)
			}
			if entry != nil {
				t.Error("NewEntry should return nil entry for whitespace-only statement")
			}
		})
	}
}

func TestNewEntry_InvalidNodeID(t *testing.T) {
	// Create an invalid/zero NodeID by using a zero-value struct
	var zeroNodeID types.NodeID
	statement := "Some assumption"

	entry, err := NewEntry(zeroNodeID, statement)

	if err == nil {
		t.Error("NewEntry should return error for invalid (zero) NodeID")
	}
	if entry != nil {
		t.Error("NewEntry should return nil entry for invalid NodeID")
	}
}

func TestEntry_Discharge(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.2")
	statement := "Temporary assumption"

	entry, err := NewEntry(nodeID, statement)
	if err != nil {
		t.Fatalf("NewEntry returned unexpected error: %v", err)
	}

	// Record time before discharge
	beforeDischarge := types.Now()

	// Small delay to ensure timestamp difference
	time.Sleep(time.Millisecond)

	err = entry.Discharge()

	if err != nil {
		t.Fatalf("Discharge returned unexpected error: %v", err)
	}
	if entry.Discharged == nil {
		t.Fatal("Discharged should not be nil after Discharge()")
	}
	if entry.Discharged.Before(beforeDischarge) {
		t.Error("Discharged timestamp should be after time before discharge call")
	}
	if entry.Discharged.Before(entry.Introduced) {
		t.Error("Discharged timestamp should not be before Introduced timestamp")
	}
}

func TestEntry_Discharge_AlreadyDischarged(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.3")
	statement := "Once discharged"

	entry, err := NewEntry(nodeID, statement)
	if err != nil {
		t.Fatalf("NewEntry returned unexpected error: %v", err)
	}

	// First discharge should succeed
	err = entry.Discharge()
	if err != nil {
		t.Fatalf("First Discharge returned unexpected error: %v", err)
	}

	// Second discharge should fail
	err = entry.Discharge()
	if err == nil {
		t.Error("Second Discharge should return error")
	}
}

func TestEntry_Discharge_PreservesFirstTimestamp(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.4")
	statement := "Timestamp preservation test"

	entry, err := NewEntry(nodeID, statement)
	if err != nil {
		t.Fatalf("NewEntry returned unexpected error: %v", err)
	}

	err = entry.Discharge()
	if err != nil {
		t.Fatalf("Discharge returned unexpected error: %v", err)
	}

	firstDischargeTime := *entry.Discharged

	// Small delay
	time.Sleep(time.Millisecond)

	// Try to discharge again (should fail but verify timestamp unchanged)
	_ = entry.Discharge()

	if !entry.Discharged.Equal(firstDischargeTime) {
		t.Error("Discharged timestamp should not change after failed second discharge")
	}
}

func TestEntry_IsActive(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.5")
	statement := "Activity test"

	entry, err := NewEntry(nodeID, statement)
	if err != nil {
		t.Fatalf("NewEntry returned unexpected error: %v", err)
	}

	// Should be active when first created
	if !entry.IsActive() {
		t.Error("Entry should be active when first created")
	}

	// Discharge the entry
	err = entry.Discharge()
	if err != nil {
		t.Fatalf("Discharge returned unexpected error: %v", err)
	}

	// Should not be active after discharge
	if entry.IsActive() {
		t.Error("Entry should not be active after discharge")
	}
}

func TestEntry_IsActive_MultipleChecks(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.6")
	statement := "Multiple activity checks"

	entry, err := NewEntry(nodeID, statement)
	if err != nil {
		t.Fatalf("NewEntry returned unexpected error: %v", err)
	}

	// Multiple checks while active should all return true
	for i := 0; i < 3; i++ {
		if !entry.IsActive() {
			t.Errorf("Entry should be active on check %d", i+1)
		}
	}

	err = entry.Discharge()
	if err != nil {
		t.Fatalf("Discharge returned unexpected error: %v", err)
	}

	// Multiple checks while discharged should all return false
	for i := 0; i < 3; i++ {
		if entry.IsActive() {
			t.Errorf("Entry should not be active on check %d after discharge", i+1)
		}
	}
}

func TestEntry_Timestamps(t *testing.T) {
	beforeCreation := types.Now()

	// Small delay to ensure timestamp difference
	time.Sleep(time.Millisecond)

	nodeID := mustParseNodeID(t, "1.7")
	statement := "Timestamp verification"

	entry, err := NewEntry(nodeID, statement)
	if err != nil {
		t.Fatalf("NewEntry returned unexpected error: %v", err)
	}

	// Small delay
	time.Sleep(time.Millisecond)

	afterCreation := types.Now()

	// Verify Introduced timestamp is set and in expected range
	if entry.Introduced.IsZero() {
		t.Error("Introduced timestamp should not be zero")
	}
	if entry.Introduced.Before(beforeCreation) {
		t.Error("Introduced timestamp should not be before creation started")
	}
	if entry.Introduced.After(afterCreation) {
		t.Error("Introduced timestamp should not be after creation completed")
	}

	// Verify Discharged is nil before discharge
	if entry.Discharged != nil {
		t.Error("Discharged should be nil before Discharge() is called")
	}

	// Small delay
	time.Sleep(time.Millisecond)

	beforeDischarge := types.Now()

	time.Sleep(time.Millisecond)

	err = entry.Discharge()
	if err != nil {
		t.Fatalf("Discharge returned unexpected error: %v", err)
	}

	time.Sleep(time.Millisecond)

	afterDischarge := types.Now()

	// Verify Discharged timestamp is set and in expected range
	if entry.Discharged == nil {
		t.Fatal("Discharged should not be nil after Discharge()")
	}
	if entry.Discharged.IsZero() {
		t.Error("Discharged timestamp should not be zero")
	}
	if entry.Discharged.Before(beforeDischarge) {
		t.Error("Discharged timestamp should not be before discharge started")
	}
	if entry.Discharged.After(afterDischarge) {
		t.Error("Discharged timestamp should not be after discharge completed")
	}

	// Verify Discharged is after Introduced
	if entry.Discharged.Before(entry.Introduced) {
		t.Error("Discharged timestamp should be after Introduced timestamp")
	}
}

func TestEntry_Timestamps_IntroducedImmutable(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.8")
	statement := "Immutability test"

	entry, err := NewEntry(nodeID, statement)
	if err != nil {
		t.Fatalf("NewEntry returned unexpected error: %v", err)
	}

	originalIntroduced := entry.Introduced

	// Discharge the entry
	err = entry.Discharge()
	if err != nil {
		t.Fatalf("Discharge returned unexpected error: %v", err)
	}

	// Introduced timestamp should not change
	if !entry.Introduced.Equal(originalIntroduced) {
		t.Error("Introduced timestamp should not change after discharge")
	}
}

func TestNewEntry_StatementWithSpecialCharacters(t *testing.T) {
	nodeID := mustParseNodeID(t, "1.9")

	testCases := []struct {
		name      string
		statement string
	}{
		{"unicode", "Assume x \u2208 \u211d"},
		{"math symbols", "Let f: A -> B be a bijection"},
		{"newlines", "Assume:\n  1. x > 0\n  2. y > 0"},
		{"tabs", "Assume\tx > 0"},
		{"quotes", `Assume "x" is positive`},
		{"long statement", "This is a very long assumption statement that contains many words and might test buffer handling or truncation logic if any exists in the implementation"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := NewEntry(nodeID, tc.statement)

			if err != nil {
				t.Fatalf("NewEntry returned unexpected error: %v", err)
			}
			if entry == nil {
				t.Fatal("NewEntry returned nil entry")
			}
			if entry.Statement != tc.statement {
				t.Errorf("Statement = %q, want %q", entry.Statement, tc.statement)
			}
		})
	}
}

func TestNewEntry_StatementWithLeadingTrailingWhitespace(t *testing.T) {
	// Statements with leading/trailing whitespace but non-empty content should be valid
	nodeID := mustParseNodeID(t, "1.10")

	testCases := []struct {
		name      string
		statement string
	}{
		{"leading space", " Assume x > 0"},
		{"trailing space", "Assume x > 0 "},
		{"both spaces", " Assume x > 0 "},
		{"leading tab", "\tAssume x > 0"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := NewEntry(nodeID, tc.statement)

			if err != nil {
				t.Fatalf("NewEntry returned unexpected error: %v", err)
			}
			if entry == nil {
				t.Fatal("NewEntry returned nil entry")
			}
			// Statement should be preserved exactly (no trimming)
			if entry.Statement != tc.statement {
				t.Errorf("Statement = %q, want %q", entry.Statement, tc.statement)
			}
		})
	}
}

func TestEntry_FullLifecycle(t *testing.T) {
	// Test a complete lifecycle: create -> check active -> discharge -> check inactive
	nodeID := mustParseNodeID(t, "1.11.1")
	statement := "Full lifecycle test assumption"

	// Step 1: Create
	entry, err := NewEntry(nodeID, statement)
	if err != nil {
		t.Fatalf("NewEntry returned unexpected error: %v", err)
	}

	// Step 2: Verify initial state
	if entry.NodeID.String() != "1.11.1" {
		t.Errorf("NodeID = %q, want %q", entry.NodeID.String(), "1.11.1")
	}
	if entry.Statement != statement {
		t.Errorf("Statement = %q, want %q", entry.Statement, statement)
	}
	if entry.Introduced.IsZero() {
		t.Error("Introduced should be set")
	}
	if entry.Discharged != nil {
		t.Error("Discharged should be nil initially")
	}
	if !entry.IsActive() {
		t.Error("Should be active initially")
	}

	// Step 3: Discharge
	err = entry.Discharge()
	if err != nil {
		t.Fatalf("Discharge returned unexpected error: %v", err)
	}

	// Step 4: Verify discharged state
	if entry.Discharged == nil {
		t.Error("Discharged should be set")
	}
	if entry.IsActive() {
		t.Error("Should not be active after discharge")
	}

	// Step 5: Verify cannot discharge again
	err = entry.Discharge()
	if err == nil {
		t.Error("Should not be able to discharge twice")
	}
}
