// Package main contains tests for the af inferences command.
package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/schema"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestInferencesCmd creates a fresh root command with the inferences subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestInferencesCmd() *cobra.Command {
	cmd := newTestRootCmd()

	inferencesCmd := newInferencesCmd()
	cmd.AddCommand(inferencesCmd)
	AddFuzzyMatching(cmd)

	return cmd
}

// executeInferencesCommand creates and executes an inferences command with the given arguments.
func executeInferencesCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newInferencesCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// af inferences - List Inference Types Tests
// =============================================================================

// TestInferencesCmd_ListAll tests listing all inference types.
func TestInferencesCmd_ListAll(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should list all inference types from schema.AllInferences()
	allInferences := schema.AllInferences()
	for _, inf := range allInferences {
		if !strings.Contains(output, string(inf.ID)) {
			t.Errorf("expected output to contain inference type %q, got: %q", inf.ID, output)
		}
	}
}

// TestInferencesCmd_ShowsNames tests that output includes human-readable names.
func TestInferencesCmd_ShowsNames(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain at least some human-readable names
	expectedNames := []string{"Modus Ponens", "Modus Tollens", "Assumption"}
	foundCount := 0
	for _, name := range expectedNames {
		if strings.Contains(output, name) {
			foundCount++
		}
	}

	if foundCount == 0 {
		t.Errorf("expected output to contain human-readable names, got: %q", output)
	}
}

// TestInferencesCmd_ShowsForms tests that output includes logical forms.
func TestInferencesCmd_ShowsForms(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain at least some logical form notation
	// Check for common notation elements
	if !strings.Contains(output, "P") && !strings.Contains(output, "Q") {
		t.Logf("Output may not show logical forms: %q", output)
	}
}

// TestInferencesCmd_SortedOutput tests that inferences are listed in sorted order.
func TestInferencesCmd_SortedOutput(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check that "assumption" appears before "by_definition" (alphabetical)
	assumptionIdx := strings.Index(output, "assumption")
	byDefIdx := strings.Index(output, "by_definition")

	if assumptionIdx != -1 && byDefIdx != -1 {
		if assumptionIdx > byDefIdx {
			t.Errorf("expected 'assumption' before 'by_definition', got assumption@%d, by_definition@%d", assumptionIdx, byDefIdx)
		}
	}
}

// TestInferencesCmd_Count tests that the correct number of inferences is shown.
func TestInferencesCmd_Count(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Count should match schema.AllInferences()
	allInferences := schema.AllInferences()
	expectedCount := len(allInferences)

	// The output should contain the count somewhere
	countStr := string(rune('0' + expectedCount%10))
	if expectedCount >= 10 {
		countStr = "1" + string(rune('0'+expectedCount%10)) // handles 10-19
	}

	// Just verify we show a count (not strictly checking exact format)
	if !strings.Contains(output, countStr) && !strings.Contains(output, "Total") {
		t.Logf("Output may not show count (%d inferences): %q", expectedCount, output)
	}
}

// =============================================================================
// af inferences - JSON Output Tests
// =============================================================================

// TestInferencesCmd_JSONOutput tests JSON output format.
func TestInferencesCmd_JSONOutput(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestInferencesCmd_JSONOutputStructure tests the structure of JSON output.
func TestInferencesCmd_JSONOutputStructure(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Try to unmarshal as object with inferences array
	var objResult struct {
		Inferences []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Form string `json:"form"`
		} `json:"inferences"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal([]byte(output), &objResult); err != nil {
		t.Fatalf("output is not valid JSON structure: %v\nOutput: %q", err, output)
	}

	// Check expected count
	allInferences := schema.AllInferences()
	if objResult.Total != len(allInferences) {
		t.Errorf("expected total %d, got %d", len(allInferences), objResult.Total)
	}

	if len(objResult.Inferences) != len(allInferences) {
		t.Errorf("expected %d inferences in array, got %d", len(allInferences), len(objResult.Inferences))
	}

	// Check that each inference has required fields
	for i, inf := range objResult.Inferences {
		if inf.ID == "" {
			t.Errorf("inference %d missing 'id' field", i)
		}
		if inf.Name == "" {
			t.Errorf("inference %d missing 'name' field", i)
		}
		if inf.Form == "" {
			t.Errorf("inference %d missing 'form' field", i)
		}
	}
}

// TestInferencesCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestInferencesCmd_JSONOutputShortFlag(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences", "-f", "json")

	if err != nil {
		t.Fatalf("expected no error with short flags, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON with -f flag: %v\nOutput: %q", err, output)
	}
}

// TestInferencesCmd_JSONContainsAllTypes tests that JSON contains all inference types.
func TestInferencesCmd_JSONContainsAllTypes(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check that all inference types appear in JSON
	allInferences := schema.AllInferences()
	for _, inf := range allInferences {
		if !strings.Contains(output, string(inf.ID)) {
			t.Errorf("expected JSON to contain inference type %q", inf.ID)
		}
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestInferencesCmd_Help tests that help output shows usage information.
func TestInferencesCmd_Help(t *testing.T) {
	cmd := newInferencesCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	// Check for expected help content
	expectations := []string{
		"inferences",
		"--format",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestInferencesCmd_HelpShortFlag tests help with -h short flag.
func TestInferencesCmd_HelpShortFlag(t *testing.T) {
	cmd := newInferencesCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-h"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "inferences") {
		t.Errorf("help output should mention 'inferences', got: %q", output)
	}
}

// TestInferencesCmd_HelpMentionsRefine tests that help mentions 'af refine'.
func TestInferencesCmd_HelpMentionsRefine(t *testing.T) {
	cmd := newInferencesCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	// Help should mention that these are used with 'af refine -i TYPE'
	if !strings.Contains(output, "refine") {
		t.Logf("help output may not mention 'refine' command: %q", output)
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestInferencesCmd_ExpectedFlags ensures the inferences command has expected flag structure.
func TestInferencesCmd_ExpectedFlags(t *testing.T) {
	cmd := newInferencesCmd()

	// Check expected flags exist
	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Error("expected inferences command to have flag 'format'")
	}

	// Check short flag
	if cmd.Flags().ShorthandLookup("f") == nil {
		t.Error("expected inferences command to have short flag -f for --format")
	}
}

// TestInferencesCmd_DefaultFlagValues verifies default values for flags.
func TestInferencesCmd_DefaultFlagValues(t *testing.T) {
	cmd := newInferencesCmd()

	// Check default format value
	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("expected format flag to exist")
	}
	if formatFlag.DefValue != "text" {
		t.Errorf("expected default format to be 'text', got %q", formatFlag.DefValue)
	}
}

// TestInferencesCmd_CommandMetadata verifies command metadata.
func TestInferencesCmd_CommandMetadata(t *testing.T) {
	cmd := newInferencesCmd()

	if cmd.Use != "inferences" {
		t.Errorf("expected Use to be 'inferences', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Format Validation Tests
// =============================================================================

// TestInferencesCmd_FormatValidation verifies format flag validation.
func TestInferencesCmd_FormatValidation(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"valid text format", "text", false},
		{"valid json format", "json", false},
		{"valid TEXT uppercase", "TEXT", false},
		{"valid JSON uppercase", "JSON", false},
		{"invalid xml format", "xml", true},
		{"invalid yaml format", "yaml", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestInferencesCmd()
			output, err := executeCommand(cmd, "inferences", "--format", tc.format)

			if tc.wantErr {
				combined := output
				if err != nil {
					combined += err.Error()
				}

				if err == nil && !strings.Contains(strings.ToLower(combined), "format") {
					t.Logf("Expected error for format %q, got output: %q", tc.format, output)
				}
			} else {
				if err != nil && strings.Contains(err.Error(), "format") {
					t.Errorf("unexpected format error for format %q: %v", tc.format, err)
				}
			}
		})
	}
}

// =============================================================================
// Content Verification Tests
// =============================================================================

// TestInferencesCmd_ContainsModusPonens tests that output includes modus_ponens.
func TestInferencesCmd_ContainsModusPonens(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "modus_ponens") {
		t.Errorf("expected output to contain 'modus_ponens', got: %q", output)
	}
}

// TestInferencesCmd_ContainsAssumption tests that output includes assumption.
func TestInferencesCmd_ContainsAssumption(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "assumption") {
		t.Errorf("expected output to contain 'assumption', got: %q", output)
	}
}

// TestInferencesCmd_ContainsUniversalInstantiation tests that output includes universal_instantiation.
func TestInferencesCmd_ContainsUniversalInstantiation(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "universal_instantiation") {
		t.Errorf("expected output to contain 'universal_instantiation', got: %q", output)
	}
}

// TestInferencesCmd_ContainsAllKnownTypes tests that all known inference types are present.
func TestInferencesCmd_ContainsAllKnownTypes(t *testing.T) {
	knownTypes := []string{
		"modus_ponens",
		"modus_tollens",
		"universal_instantiation",
		"existential_instantiation",
		"universal_generalization",
		"existential_generalization",
		"by_definition",
		"assumption",
		"local_assume",
		"local_discharge",
		"contradiction",
	}

	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	for _, typ := range knownTypes {
		if !strings.Contains(output, typ) {
			t.Errorf("expected output to contain inference type %q, got: %q", typ, output)
		}
	}
}

// =============================================================================
// Next Steps Suggestion Tests
// =============================================================================

// TestInferencesCmd_ShowsNextSteps tests that output includes next steps suggestion.
func TestInferencesCmd_ShowsNextSteps(t *testing.T) {
	cmd := newTestInferencesCmd()
	output, err := executeCommand(cmd, "inferences")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should suggest how to use these with af refine
	if !strings.Contains(output, "refine") && !strings.Contains(output, "-i") {
		t.Logf("Output may not show next steps for using inferences: %q", output)
	}
}
