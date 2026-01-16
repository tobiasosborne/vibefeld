package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/patterns"
)

// setupTestProofDir initializes a proof directory for testing.
func setupTestProofDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Initialize a proof using the init command
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "Test claim", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	return dir
}

// TestPatternsCmd_NoProof tests patterns command when no proof is initialized.
func TestPatternsCmd_NoProof(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := newPatternsListCmd()
	cmd.SetArgs([]string{"--dir", tmpDir})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error for uninitialized proof")
	}

	// Error might be "not initialized" or directory access error
	errStr := err.Error()
	if !strings.Contains(errStr, "not initialized") && !strings.Contains(errStr, "error") {
		t.Errorf("Expected error for uninitialized proof, got: %v", err)
	}
}

// TestPatternsCmd_List tests the patterns list subcommand.
func TestPatternsCmd_List(t *testing.T) {
	tmpDir := setupTestProofDir(t)

	// Create a pattern library with some patterns
	lib := patterns.NewPatternLibrary()
	p1 := patterns.NewPattern(patterns.PatternLogicalGap, "Missing step", "Gap in reasoning")
	p1.Occurrences = 5
	lib.AddPattern(p1)

	p2 := patterns.NewPattern(patterns.PatternScopeViolation, "Scope issue", "Using assumption outside scope")
	p2.Occurrences = 3
	lib.AddPattern(p2)

	if err := lib.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save patterns: %v", err)
	}

	cmd := newPatternsListCmd()
	cmd.SetArgs([]string{"--dir", tmpDir})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns list error: %v", err)
	}

	output := stdout.String()

	// Should list patterns
	if !strings.Contains(output, "logical_gap") {
		t.Error("Output should contain 'logical_gap'")
	}
	if !strings.Contains(output, "scope_violation") {
		t.Error("Output should contain 'scope_violation'")
	}
}

// TestPatternsCmd_ListEmpty tests the patterns list subcommand with no patterns.
func TestPatternsCmd_ListEmpty(t *testing.T) {
	tmpDir := setupTestProofDir(t)

	cmd := newPatternsListCmd()
	cmd.SetArgs([]string{"--dir", tmpDir})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns list error: %v", err)
	}

	output := stdout.String()

	// Should indicate no patterns
	if !strings.Contains(output, "No patterns") {
		t.Error("Output should indicate no patterns found")
	}
}

// TestPatternsCmd_ListJSON tests the patterns list subcommand with JSON output.
func TestPatternsCmd_ListJSON(t *testing.T) {
	tmpDir := setupTestProofDir(t)

	// Create a pattern library
	lib := patterns.NewPatternLibrary()
	p := patterns.NewPattern(patterns.PatternLogicalGap, "Test gap", "Example")
	p.Occurrences = 2
	lib.AddPattern(p)

	if err := lib.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save patterns: %v", err)
	}

	cmd := newPatternsListCmd()
	cmd.SetArgs([]string{"--dir", tmpDir, "--json"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns list --json error: %v", err)
	}

	output := stdout.String()

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Should contain patterns array
	if _, ok := result["patterns"]; !ok {
		t.Error("JSON output should contain 'patterns' key")
	}
}

// TestPatternsCmd_Stats tests the patterns stats subcommand.
func TestPatternsCmd_Stats(t *testing.T) {
	tmpDir := setupTestProofDir(t)

	// Create a pattern library with various patterns
	lib := patterns.NewPatternLibrary()

	p1 := patterns.NewPattern(patterns.PatternLogicalGap, "Gap 1", "Example 1")
	p1.Occurrences = 10
	lib.AddPattern(p1)

	p2 := patterns.NewPattern(patterns.PatternScopeViolation, "Scope 1", "Example 2")
	p2.Occurrences = 5
	lib.AddPattern(p2)

	p3 := patterns.NewPattern(patterns.PatternCircularReasoning, "Circular 1", "Example 3")
	p3.Occurrences = 2
	lib.AddPattern(p3)

	if err := lib.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save patterns: %v", err)
	}

	cmd := newPatternsStatsCmd()
	cmd.SetArgs([]string{"--dir", tmpDir})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns stats error: %v", err)
	}

	output := stdout.String()

	// Should show statistics
	if !strings.Contains(output, "logical_gap") {
		t.Error("Output should contain 'logical_gap'")
	}
	if !strings.Contains(output, "10") { // occurrence count
		t.Error("Output should show occurrence count '10'")
	}
}

// TestPatternsCmd_StatsJSON tests the patterns stats subcommand with JSON output.
func TestPatternsCmd_StatsJSON(t *testing.T) {
	tmpDir := setupTestProofDir(t)

	// Create a pattern library
	lib := patterns.NewPatternLibrary()
	p := patterns.NewPattern(patterns.PatternLogicalGap, "Test", "Example")
	p.Occurrences = 5
	lib.AddPattern(p)

	if err := lib.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save patterns: %v", err)
	}

	cmd := newPatternsStatsCmd()
	cmd.SetArgs([]string{"--dir", tmpDir, "--json"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns stats --json error: %v", err)
	}

	output := stdout.String()

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Should contain expected fields
	if _, ok := result["total_patterns"]; !ok {
		t.Error("JSON output should contain 'total_patterns' key")
	}
	if _, ok := result["total_occurrences"]; !ok {
		t.Error("JSON output should contain 'total_occurrences' key")
	}
}

// TestPatternsCmd_Analyze tests the patterns analyze subcommand.
func TestPatternsCmd_Analyze(t *testing.T) {
	tmpDir := setupTestProofDir(t)

	// Create a pattern library with known patterns
	lib := patterns.NewPatternLibrary()
	p := patterns.NewPattern(patterns.PatternLogicalGap, "Vague justification", "Using 'trivially' without proof")
	p.Occurrences = 10
	lib.AddPattern(p)

	if err := lib.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save patterns: %v", err)
	}

	cmd := newPatternsAnalyzeCmd()
	cmd.SetArgs([]string{"--dir", tmpDir})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns analyze error: %v", err)
	}

	// Output can show issues or indicate no issues found
	// Both are valid depending on the test data
	output := stdout.String()
	if output == "" {
		t.Error("patterns analyze should produce output")
	}
}

// TestPatternsCmd_AnalyzeJSON tests the patterns analyze subcommand with JSON output.
func TestPatternsCmd_AnalyzeJSON(t *testing.T) {
	tmpDir := setupTestProofDir(t)

	// Create a pattern library
	lib := patterns.NewPatternLibrary()
	p := patterns.NewPattern(patterns.PatternLogicalGap, "Test pattern", "Example")
	p.Occurrences = 5
	lib.AddPattern(p)

	if err := lib.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save patterns: %v", err)
	}

	cmd := newPatternsAnalyzeCmd()
	cmd.SetArgs([]string{"--dir", tmpDir, "--json"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns analyze --json error: %v", err)
	}

	output := stdout.String()

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Should contain issues array
	if _, ok := result["issues"]; !ok {
		t.Error("JSON output should contain 'issues' key")
	}
}

// TestPatternsCmd_TypeFilter tests filtering patterns by type.
func TestPatternsCmd_TypeFilter(t *testing.T) {
	tmpDir := setupTestProofDir(t)

	// Create a pattern library with multiple types
	lib := patterns.NewPatternLibrary()

	p1 := patterns.NewPattern(patterns.PatternLogicalGap, "Gap", "Example gap")
	p1.Occurrences = 5
	lib.AddPattern(p1)

	p2 := patterns.NewPattern(patterns.PatternScopeViolation, "Scope", "Example scope")
	p2.Occurrences = 3
	lib.AddPattern(p2)

	if err := lib.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save patterns: %v", err)
	}

	cmd := newPatternsListCmd()
	cmd.SetArgs([]string{"--dir", tmpDir, "--type", "logical_gap"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns list --type error: %v", err)
	}

	output := stdout.String()

	// Should only show logical_gap patterns
	if !strings.Contains(output, "logical_gap") {
		t.Error("Output should contain 'logical_gap'")
	}
	// Should not show scope_violation
	if strings.Contains(output, "scope_violation") {
		t.Error("Output should not contain 'scope_violation' when filtered")
	}
}

// TestPatternsCmd_InvalidType tests with an invalid pattern type filter.
func TestPatternsCmd_InvalidType(t *testing.T) {
	tmpDir := setupTestProofDir(t)

	cmd := newPatternsListCmd()
	cmd.SetArgs([]string{"--dir", tmpDir, "--type", "invalid_type"})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error for invalid pattern type")
	}

	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Expected 'invalid' in error message, got: %v", err)
	}
}

// TestPatternsCmd_Extract tests the patterns extract subcommand.
func TestPatternsCmd_Extract(t *testing.T) {
	tmpDir := setupTestProofDir(t)

	cmd := newPatternsExtractCmd()
	cmd.SetArgs([]string{"--dir", tmpDir})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns extract error: %v", err)
	}

	output := stdout.String()

	// Should indicate extraction was done
	if !strings.Contains(strings.ToLower(output), "extract") && !strings.Contains(strings.ToLower(output), "pattern") {
		t.Error("Output should mention pattern extraction")
	}

	// Verify patterns were saved
	lib, err := patterns.LoadPatternLibrary(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load patterns after extract: %v", err)
	}
	// Library may be empty if no resolved challenges
	if lib == nil {
		t.Error("Pattern library should not be nil")
	}
}

// TestPatternsCmd_ExtractJSON tests the patterns extract subcommand with JSON output.
func TestPatternsCmd_ExtractJSON(t *testing.T) {
	tmpDir := setupTestProofDir(t)

	cmd := newPatternsExtractCmd()
	cmd.SetArgs([]string{"--dir", tmpDir, "--json"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns extract --json error: %v", err)
	}

	output := stdout.String()

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Should contain expected fields
	if _, ok := result["challenges_analyzed"]; !ok {
		t.Error("JSON output should contain 'challenges_analyzed' key")
	}
	if _, ok := result["patterns_extracted"]; !ok {
		t.Error("JSON output should contain 'patterns_extracted' key")
	}
}

// TestPatternsCmd_Help tests that the patterns command shows help.
func TestPatternsCmd_Help(t *testing.T) {
	cmd := newPatternsCmd()
	cmd.SetArgs([]string{})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns help error: %v", err)
	}

	output := stdout.String()

	// Should show subcommands
	if !strings.Contains(output, "list") {
		t.Error("Help should mention 'list' subcommand")
	}
	if !strings.Contains(output, "analyze") {
		t.Error("Help should mention 'analyze' subcommand")
	}
	if !strings.Contains(output, "stats") {
		t.Error("Help should mention 'stats' subcommand")
	}
}
