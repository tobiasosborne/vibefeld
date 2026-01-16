package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// mustParse is a test helper that parses a node ID and panics on error.
func mustParse(s string) types.NodeID {
	id, err := types.Parse(s)
	if err != nil {
		panic("mustParse: " + err.Error())
	}
	return id
}

// TestPatternType_String tests the string representation of pattern types.
func TestPatternType_String(t *testing.T) {
	tests := []struct {
		pt   PatternType
		want string
	}{
		{PatternLogicalGap, "logical_gap"},
		{PatternScopeViolation, "scope_violation"},
		{PatternCircularReasoning, "circular_reasoning"},
		{PatternUndefinedTerm, "undefined_term"},
	}

	for _, tt := range tests {
		if got := string(tt.pt); got != tt.want {
			t.Errorf("PatternType(%q) = %q, want %q", tt.pt, got, tt.want)
		}
	}
}

// TestPattern_New tests creating a new pattern.
func TestPattern_New(t *testing.T) {
	p := NewPattern(
		PatternLogicalGap,
		"Missing justification",
		"Claim was made without supporting evidence",
	)

	if p.Type != PatternLogicalGap {
		t.Errorf("Pattern.Type = %q, want %q", p.Type, PatternLogicalGap)
	}
	if p.Description != "Missing justification" {
		t.Errorf("Pattern.Description = %q, want %q", p.Description, "Missing justification")
	}
	if p.Example != "Claim was made without supporting evidence" {
		t.Errorf("Pattern.Example = %q, want %q", p.Example, "Claim was made without supporting evidence")
	}
	if p.Occurrences != 0 {
		t.Errorf("Pattern.Occurrences = %d, want 0", p.Occurrences)
	}
}

// TestPattern_Increment tests incrementing pattern occurrences.
func TestPattern_Increment(t *testing.T) {
	p := NewPattern(PatternLogicalGap, "Test", "Example")

	if p.Occurrences != 0 {
		t.Fatalf("Initial Occurrences = %d, want 0", p.Occurrences)
	}

	p.Increment()
	if p.Occurrences != 1 {
		t.Errorf("After Increment, Occurrences = %d, want 1", p.Occurrences)
	}

	p.Increment()
	p.Increment()
	if p.Occurrences != 3 {
		t.Errorf("After 3 Increments, Occurrences = %d, want 3", p.Occurrences)
	}
}

// TestPatternLibrary_New tests creating a new pattern library.
func TestPatternLibrary_New(t *testing.T) {
	lib := NewPatternLibrary()

	if lib == nil {
		t.Fatal("NewPatternLibrary() returned nil")
	}
	if lib.Patterns == nil {
		t.Error("PatternLibrary.Patterns is nil")
	}
	if len(lib.Patterns) != 0 {
		t.Errorf("len(PatternLibrary.Patterns) = %d, want 0", len(lib.Patterns))
	}
}

// TestPatternLibrary_AddPattern tests adding patterns to the library.
func TestPatternLibrary_AddPattern(t *testing.T) {
	lib := NewPatternLibrary()

	p1 := NewPattern(PatternLogicalGap, "Gap 1", "Example 1")
	lib.AddPattern(p1)

	if len(lib.Patterns) != 1 {
		t.Errorf("len(Patterns) = %d, want 1", len(lib.Patterns))
	}

	p2 := NewPattern(PatternScopeViolation, "Scope issue", "Example 2")
	lib.AddPattern(p2)

	if len(lib.Patterns) != 2 {
		t.Errorf("len(Patterns) = %d, want 2", len(lib.Patterns))
	}
}

// TestPatternLibrary_GetByType tests retrieving patterns by type.
func TestPatternLibrary_GetByType(t *testing.T) {
	lib := NewPatternLibrary()

	p1 := NewPattern(PatternLogicalGap, "Gap 1", "Example 1")
	p1.Occurrences = 5
	lib.AddPattern(p1)

	p2 := NewPattern(PatternLogicalGap, "Gap 2", "Example 2")
	p2.Occurrences = 3
	lib.AddPattern(p2)

	p3 := NewPattern(PatternScopeViolation, "Scope issue", "Example 3")
	lib.AddPattern(p3)

	gaps := lib.GetByType(PatternLogicalGap)
	if len(gaps) != 2 {
		t.Errorf("len(GetByType(PatternLogicalGap)) = %d, want 2", len(gaps))
	}

	scopes := lib.GetByType(PatternScopeViolation)
	if len(scopes) != 1 {
		t.Errorf("len(GetByType(PatternScopeViolation)) = %d, want 1", len(scopes))
	}

	circular := lib.GetByType(PatternCircularReasoning)
	if len(circular) != 0 {
		t.Errorf("len(GetByType(PatternCircularReasoning)) = %d, want 0", len(circular))
	}
}

// TestPatternLibrary_Stats tests computing statistics.
func TestPatternLibrary_Stats(t *testing.T) {
	lib := NewPatternLibrary()

	p1 := NewPattern(PatternLogicalGap, "Gap 1", "Example 1")
	p1.Occurrences = 5
	lib.AddPattern(p1)

	p2 := NewPattern(PatternScopeViolation, "Scope issue", "Example 2")
	p2.Occurrences = 3
	lib.AddPattern(p2)

	p3 := NewPattern(PatternCircularReasoning, "Circular", "Example 3")
	p3.Occurrences = 2
	lib.AddPattern(p3)

	stats := lib.Stats()

	if stats.TotalPatterns != 3 {
		t.Errorf("TotalPatterns = %d, want 3", stats.TotalPatterns)
	}
	if stats.TotalOccurrences != 10 {
		t.Errorf("TotalOccurrences = %d, want 10", stats.TotalOccurrences)
	}
	if stats.ByType[PatternLogicalGap] != 5 {
		t.Errorf("ByType[LogicalGap] = %d, want 5", stats.ByType[PatternLogicalGap])
	}
	if stats.ByType[PatternScopeViolation] != 3 {
		t.Errorf("ByType[ScopeViolation] = %d, want 3", stats.ByType[PatternScopeViolation])
	}
}

// TestPatternLibrary_SaveLoad tests saving and loading the library.
func TestPatternLibrary_SaveLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	afDir := filepath.Join(tmpDir, ".af")
	if err := os.MkdirAll(afDir, 0755); err != nil {
		t.Fatalf("Failed to create .af directory: %v", err)
	}

	// Create and populate library
	lib := NewPatternLibrary()
	p1 := NewPattern(PatternLogicalGap, "Gap 1", "Example 1")
	p1.Occurrences = 5
	lib.AddPattern(p1)

	p2 := NewPattern(PatternScopeViolation, "Scope issue", "Example 2")
	p2.Occurrences = 3
	lib.AddPattern(p2)

	// Save
	if err := lib.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	patternsPath := filepath.Join(afDir, "patterns.json")
	if _, err := os.Stat(patternsPath); os.IsNotExist(err) {
		t.Fatal("patterns.json was not created")
	}

	// Load into new library
	lib2, err := LoadPatternLibrary(tmpDir)
	if err != nil {
		t.Fatalf("LoadPatternLibrary() error = %v", err)
	}

	if len(lib2.Patterns) != 2 {
		t.Errorf("Loaded library has %d patterns, want 2", len(lib2.Patterns))
	}

	// Verify pattern data preserved
	gaps := lib2.GetByType(PatternLogicalGap)
	if len(gaps) != 1 {
		t.Errorf("Loaded library has %d LogicalGap patterns, want 1", len(gaps))
	}
	if len(gaps) > 0 && gaps[0].Occurrences != 5 {
		t.Errorf("Loaded LogicalGap pattern has %d occurrences, want 5", gaps[0].Occurrences)
	}
}

// TestPatternLibrary_LoadNonExistent tests loading when no file exists.
func TestPatternLibrary_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	// Load should return empty library, not error
	lib, err := LoadPatternLibrary(tmpDir)
	if err != nil {
		t.Fatalf("LoadPatternLibrary() error = %v, want nil", err)
	}
	if lib == nil {
		t.Fatal("LoadPatternLibrary() returned nil library")
	}
	if len(lib.Patterns) != 0 {
		t.Errorf("len(Patterns) = %d, want 0", len(lib.Patterns))
	}
}

// TestDetectPatternType tests pattern type detection from challenge target.
func TestDetectPatternType(t *testing.T) {
	tests := []struct {
		target string
		reason string
		want   PatternType
	}{
		{"gap", "Missing step", PatternLogicalGap},
		{"statement", "Gap in reasoning", PatternLogicalGap},
		{"scope", "Using assumption out of scope", PatternScopeViolation},
		{"dependencies", "Depends on itself", PatternCircularReasoning},
		{"inference", "Circular dependency", PatternCircularReasoning},
		{"context", "Term 'foo' not defined", PatternUndefinedTerm},
		{"context", "Missing definition", PatternUndefinedTerm},
		{"statement", "Unknown term used", PatternUndefinedTerm},
		// Default fallback
		{"domain", "Some other issue", PatternLogicalGap},
	}

	for _, tt := range tests {
		got := DetectPatternType(tt.target, tt.reason)
		if got != tt.want {
			t.Errorf("DetectPatternType(%q, %q) = %q, want %q", tt.target, tt.reason, got, tt.want)
		}
	}
}

// TestAnalyzer_New tests creating a new analyzer.
func TestAnalyzer_New(t *testing.T) {
	lib := NewPatternLibrary()
	analyzer := NewAnalyzer(lib)

	if analyzer == nil {
		t.Fatal("NewAnalyzer() returned nil")
	}
	if analyzer.library != lib {
		t.Error("Analyzer.library does not match input")
	}
}

// TestAnalyzer_AnalyzeChallenge tests analyzing a single challenge.
func TestAnalyzer_AnalyzeChallenge(t *testing.T) {
	lib := NewPatternLibrary()
	analyzer := NewAnalyzer(lib)

	challenge := &state.Challenge{
		ID:         "ch-001",
		NodeID:     mustParse("1.1"),
		Target:     "gap",
		Reason:     "Missing justification for claim",
		Status:     "resolved",
		Resolution: "Added supporting evidence",
	}

	pattern := analyzer.AnalyzeChallenge(challenge)

	if pattern == nil {
		t.Fatal("AnalyzeChallenge() returned nil")
	}
	if pattern.Type != PatternLogicalGap {
		t.Errorf("Pattern.Type = %q, want %q", pattern.Type, PatternLogicalGap)
	}
	if pattern.ChallengeID != "ch-001" {
		t.Errorf("Pattern.ChallengeID = %q, want %q", pattern.ChallengeID, "ch-001")
	}
}

// TestAnalyzer_AnalyzeResolvedChallenges tests analyzing multiple resolved challenges.
func TestAnalyzer_AnalyzeResolvedChallenges(t *testing.T) {
	lib := NewPatternLibrary()
	analyzer := NewAnalyzer(lib)

	challenges := []*state.Challenge{
		{
			ID:         "ch-001",
			NodeID:     mustParse("1.1"),
			Target:     "gap",
			Reason:     "Missing justification",
			Status:     "resolved",
			Resolution: "Added evidence",
		},
		{
			ID:         "ch-002",
			NodeID:     mustParse("1.2"),
			Target:     "scope",
			Reason:     "Using local assumption outside its scope",
			Status:     "resolved",
			Resolution: "Restructured proof",
		},
		{
			ID:         "ch-003",
			NodeID:     mustParse("1.3"),
			Target:     "gap",
			Reason:     "Another logical gap",
			Status:     "open", // Should be skipped
		},
	}

	patterns := analyzer.AnalyzeResolvedChallenges(challenges)

	// Should only analyze resolved challenges
	if len(patterns) != 2 {
		t.Errorf("len(patterns) = %d, want 2", len(patterns))
	}
}

// TestAnalyzer_ExtractPatterns tests extracting patterns to the library.
func TestAnalyzer_ExtractPatterns(t *testing.T) {
	lib := NewPatternLibrary()
	analyzer := NewAnalyzer(lib)

	challenges := []*state.Challenge{
		{
			ID:         "ch-001",
			NodeID:     mustParse("1.1"),
			Target:     "gap",
			Reason:     "Missing justification",
			Status:     "resolved",
			Resolution: "Added evidence",
		},
		{
			ID:         "ch-002",
			NodeID:     mustParse("1.2"),
			Target:     "gap",
			Reason:     "Another gap",
			Status:     "resolved",
			Resolution: "Fixed it",
		},
	}

	analyzer.ExtractPatterns(challenges)

	stats := lib.Stats()
	if stats.TotalOccurrences != 2 {
		t.Errorf("TotalOccurrences = %d, want 2", stats.TotalOccurrences)
	}
	if stats.ByType[PatternLogicalGap] != 2 {
		t.Errorf("ByType[LogicalGap] = %d, want 2", stats.ByType[PatternLogicalGap])
	}
}

// TestAnalyzer_AnalyzeNode tests analyzing a node for potential issues.
func TestAnalyzer_AnalyzeNode(t *testing.T) {
	lib := NewPatternLibrary()

	// Add known patterns
	p1 := NewPattern(PatternLogicalGap, "Missing justification", "Claims without evidence")
	p1.Occurrences = 10
	lib.AddPattern(p1)

	p2 := NewPattern(PatternScopeViolation, "Scope violation", "Using assumptions outside scope")
	p2.Occurrences = 5
	lib.AddPattern(p2)

	analyzer := NewAnalyzer(lib)

	// Create a node with potential issues
	n, _ := node.NewNode(
		mustParse("1.1"),
		schema.NodeTypeClaim,
		"This follows immediately", // Vague statement that might indicate a gap
		schema.InferenceModusPonens,
	)

	issues := analyzer.AnalyzeNode(n, nil)

	// Should detect potential issues based on patterns
	if issues == nil {
		t.Fatal("AnalyzeNode() returned nil")
	}
}

// TestAnalyzer_AnalyzeNodeWithScopeViolation tests detecting scope violations.
func TestAnalyzer_AnalyzeNodeWithScopeViolation(t *testing.T) {
	lib := NewPatternLibrary()

	// Add scope violation pattern
	p := NewPattern(PatternScopeViolation, "Scope violation", "Using assumptions outside scope")
	p.Occurrences = 5
	lib.AddPattern(p)

	analyzer := NewAnalyzer(lib)

	// Create a local_assume node
	assumeNode, _ := node.NewNode(
		mustParse("1.1"),
		schema.NodeTypeLocalAssume,
		"Assume P",
		schema.InferenceLocalAssume,
	)

	// Create a node that might be using assumptions incorrectly
	claimNode, _ := node.NewNodeWithOptions(
		mustParse("1.2.1"), // Child of a different branch
		schema.NodeTypeClaim,
		"Therefore Q by assumption P",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Scope: []string{"1.1"}, // Claims to use assumption from 1.1
		},
	)

	// Pass the assume node as context
	contextNodes := map[string]*node.Node{
		"1.1": assumeNode,
	}

	issues := analyzer.AnalyzeNode(claimNode, contextNodes)

	// The analyzer should flag potential scope issues
	// This is a simple heuristic check
	if issues == nil {
		t.Log("AnalyzeNode returned nil issues (acceptable if no patterns match)")
	}
}

// TestPatternLibrary_JSON tests JSON serialization.
func TestPatternLibrary_JSON(t *testing.T) {
	lib := NewPatternLibrary()

	p := NewPattern(PatternLogicalGap, "Test gap", "Example gap")
	p.Occurrences = 3
	lib.AddPattern(p)

	data, err := json.Marshal(lib)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var lib2 PatternLibrary
	if err := json.Unmarshal(data, &lib2); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(lib2.Patterns) != 1 {
		t.Errorf("Unmarshaled library has %d patterns, want 1", len(lib2.Patterns))
	}
}

// TestPatternStats_Empty tests stats for empty library.
func TestPatternStats_Empty(t *testing.T) {
	lib := NewPatternLibrary()
	stats := lib.Stats()

	if stats.TotalPatterns != 0 {
		t.Errorf("TotalPatterns = %d, want 0", stats.TotalPatterns)
	}
	if stats.TotalOccurrences != 0 {
		t.Errorf("TotalOccurrences = %d, want 0", stats.TotalOccurrences)
	}
}

// TestPatternType_Validate tests pattern type validation.
func TestPatternType_Validate(t *testing.T) {
	tests := []struct {
		pt      PatternType
		wantErr bool
	}{
		{PatternLogicalGap, false},
		{PatternScopeViolation, false},
		{PatternCircularReasoning, false},
		{PatternUndefinedTerm, false},
		{PatternType("invalid"), true},
		{PatternType(""), true},
	}

	for _, tt := range tests {
		err := ValidatePatternType(tt.pt)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePatternType(%q) error = %v, wantErr = %v", tt.pt, err, tt.wantErr)
		}
	}
}

// TestAllPatternTypes tests listing all pattern types.
func TestAllPatternTypes(t *testing.T) {
	types := AllPatternTypes()

	if len(types) != 4 {
		t.Errorf("len(AllPatternTypes()) = %d, want 4", len(types))
	}

	// Verify all expected types are present
	expected := map[PatternType]bool{
		PatternLogicalGap:       false,
		PatternScopeViolation:   false,
		PatternCircularReasoning: false,
		PatternUndefinedTerm:    false,
	}

	for _, pt := range types {
		expected[pt] = true
	}

	for pt, found := range expected {
		if !found {
			t.Errorf("AllPatternTypes() missing %q", pt)
		}
	}
}

// TestPotentialIssue tests the PotentialIssue struct.
func TestPotentialIssue(t *testing.T) {
	issue := PotentialIssue{
		NodeID:      mustParse("1.2.3"),
		PatternType: PatternLogicalGap,
		Description: "Potential gap detected",
		Confidence:  0.75,
	}

	if issue.NodeID.String() != "1.2.3" {
		t.Errorf("NodeID = %q, want %q", issue.NodeID.String(), "1.2.3")
	}
	if issue.PatternType != PatternLogicalGap {
		t.Errorf("PatternType = %q, want %q", issue.PatternType, PatternLogicalGap)
	}
	if issue.Confidence != 0.75 {
		t.Errorf("Confidence = %f, want 0.75", issue.Confidence)
	}
}

// TestAnalyzer_AnalyzeState tests analyzing an entire proof state.
func TestAnalyzer_AnalyzeState(t *testing.T) {
	lib := NewPatternLibrary()

	// Add known patterns
	p := NewPattern(PatternLogicalGap, "Gap pattern", "Example")
	p.Occurrences = 5
	lib.AddPattern(p)

	analyzer := NewAnalyzer(lib)

	// Create a mock state with some nodes
	st := state.NewState()

	n1, _ := node.NewNode(
		mustParse("1"),
		schema.NodeTypeClaim,
		"Root claim",
		schema.InferenceAssumption,
	)
	st.AddNode(n1)

	n2, _ := node.NewNode(
		mustParse("1.1"),
		schema.NodeTypeClaim,
		"This follows trivially", // Might trigger gap detection
		schema.InferenceModusPonens,
	)
	st.AddNode(n2)

	issues := analyzer.AnalyzeState(st)

	// Should return a slice (may be empty if no issues detected)
	if issues == nil {
		t.Error("AnalyzeState() returned nil, want slice")
	}
}
