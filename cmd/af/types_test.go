// Package main contains tests for the af types command.
package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestTypesCmd creates a fresh root command with the types subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestTypesCmd() *cobra.Command {
	cmd := newTestRootCmd()

	typesCmd := newTypesCmd()
	cmd.AddCommand(typesCmd)

	return cmd
}

// executeTypesCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeTypesCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// =============================================================================
// Happy Path Tests
// =============================================================================

// TestTypesCmd_ListTypes verifies that the types command lists all node types.
func TestTypesCmd_ListTypes(t *testing.T) {
	cmd := newTestTypesCmd()
	output, err := executeTypesCommand(cmd, "types")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain all valid node types
	expectedTypes := []string{
		"claim",
		"local_assume",
		"local_discharge",
		"case",
		"qed",
	}

	for _, nodeType := range expectedTypes {
		if !strings.Contains(output, nodeType) {
			t.Errorf("expected output to contain node type %q, got: %q", nodeType, output)
		}
	}
}

// TestTypesCmd_ShowsDescriptions verifies that descriptions are shown.
func TestTypesCmd_ShowsDescriptions(t *testing.T) {
	cmd := newTestTypesCmd()
	output, err := executeTypesCommand(cmd, "types")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain descriptive text for types
	expectations := []string{
		"assertion",       // from claim description
		"hypothesis",      // from local_assume description
		"scope",           // from local_assume or local_discharge
		"case split",      // from case description
		"concluding",      // from qed description
	}

	found := 0
	for _, exp := range expectations {
		if strings.Contains(strings.ToLower(output), exp) {
			found++
		}
	}

	// At least some descriptions should be present
	if found < 2 {
		t.Errorf("expected output to contain descriptive text, got: %q", output)
	}
}

// TestTypesCmd_ShowsCount verifies that the count of types is shown.
func TestTypesCmd_ShowsCount(t *testing.T) {
	cmd := newTestTypesCmd()
	output, err := executeTypesCommand(cmd, "types")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should indicate 5 types
	if !strings.Contains(output, "5") {
		t.Errorf("expected output to contain count '5', got: %q", output)
	}
}

// TestTypesCmd_ShowsUsageHint verifies that usage guidance is provided.
func TestTypesCmd_ShowsUsageHint(t *testing.T) {
	cmd := newTestTypesCmd()
	output, err := executeTypesCommand(cmd, "types")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show how to use with af refine
	if !strings.Contains(output, "af refine") {
		t.Errorf("expected output to show usage with 'af refine', got: %q", output)
	}
}

// =============================================================================
// JSON Output Tests
// =============================================================================

// TestTypesCmd_JSONOutput verifies JSON output format.
func TestTypesCmd_JSONOutput(t *testing.T) {
	cmd := newTestTypesCmd()
	output, err := executeTypesCommand(cmd, "types", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v\nOutput: %s", err, output)
		return
	}

	// JSON should include types array and total
	if _, ok := result["types"]; !ok {
		t.Error("expected JSON to contain 'types' key")
	}

	if _, ok := result["total"]; !ok {
		t.Error("expected JSON to contain 'total' key")
	}
}

// TestTypesCmd_JSONOutputStructure verifies JSON structure.
func TestTypesCmd_JSONOutputStructure(t *testing.T) {
	cmd := newTestTypesCmd()
	output, err := executeTypesCommand(cmd, "types", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse JSON and check structure
	var result struct {
		Types []struct {
			Type        string `json:"type"`
			Description string `json:"description"`
			OpensScope  bool   `json:"opens_scope"`
			ClosesScope bool   `json:"closes_scope"`
		} `json:"types"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("expected total of 5 types, got %d", result.Total)
	}

	if len(result.Types) != 5 {
		t.Errorf("expected 5 types in array, got %d", len(result.Types))
	}

	// Check each type has required fields
	for i, nt := range result.Types {
		if nt.Type == "" {
			t.Errorf("type %d missing 'type' field", i)
		}
		if nt.Description == "" {
			t.Errorf("type %d missing 'description' field", i)
		}
	}
}

// TestTypesCmd_JSONOutputContainsAllTypes verifies all types are in JSON output.
func TestTypesCmd_JSONOutputContainsAllTypes(t *testing.T) {
	cmd := newTestTypesCmd()
	output, err := executeTypesCommand(cmd, "types", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse JSON
	var result struct {
		Types []struct {
			Type string `json:"type"`
		} `json:"types"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Check all expected types are present
	expectedTypes := map[string]bool{
		"claim":            false,
		"local_assume":     false,
		"local_discharge":  false,
		"case":             false,
		"qed":              false,
	}

	for _, nt := range result.Types {
		if _, ok := expectedTypes[nt.Type]; ok {
			expectedTypes[nt.Type] = true
		}
	}

	for typeName, found := range expectedTypes {
		if !found {
			t.Errorf("expected JSON to contain type %q", typeName)
		}
	}
}

// TestTypesCmd_JSONOutputScopeInfo verifies scope information in JSON output.
func TestTypesCmd_JSONOutputScopeInfo(t *testing.T) {
	cmd := newTestTypesCmd()
	output, err := executeTypesCommand(cmd, "types", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse JSON
	var result struct {
		Types []struct {
			Type        string `json:"type"`
			OpensScope  bool   `json:"opens_scope"`
			ClosesScope bool   `json:"closes_scope"`
		} `json:"types"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Find local_assume and local_discharge
	var localAssume, localDischarge *struct {
		Type        string `json:"type"`
		OpensScope  bool   `json:"opens_scope"`
		ClosesScope bool   `json:"closes_scope"`
	}

	for i := range result.Types {
		if result.Types[i].Type == "local_assume" {
			localAssume = &result.Types[i]
		}
		if result.Types[i].Type == "local_discharge" {
			localDischarge = &result.Types[i]
		}
	}

	if localAssume == nil {
		t.Fatal("expected to find local_assume in types")
	}
	if !localAssume.OpensScope {
		t.Error("expected local_assume to have opens_scope=true")
	}

	if localDischarge == nil {
		t.Fatal("expected to find local_discharge in types")
	}
	if !localDischarge.ClosesScope {
		t.Error("expected local_discharge to have closes_scope=true")
	}
}

// TestTypesCmd_JSONOutputShortFlag verifies JSON output with short flag.
func TestTypesCmd_JSONOutputShortFlag(t *testing.T) {
	cmd := newTestTypesCmd()
	output, err := executeTypesCommand(cmd, "types", "-f", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output with -f flag, got error: %v", err)
	}
}

// =============================================================================
// Format Validation Tests
// =============================================================================

// TestTypesCmd_InvalidFormat verifies error for invalid format.
func TestTypesCmd_InvalidFormat(t *testing.T) {
	cmd := newTestTypesCmd()
	_, err := executeTypesCommand(cmd, "types", "--format", "xml")

	if err == nil {
		t.Error("expected error for invalid format, got nil")
	}
}

// TestTypesCmd_FormatValidation verifies format flag validation.
func TestTypesCmd_FormatValidation(t *testing.T) {
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
		{"invalid csv format", "csv", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestTypesCmd()
			_, err := executeTypesCommand(cmd, "types", "--format", tc.format)

			if tc.wantErr && err == nil {
				t.Errorf("expected error for format %q, got nil", tc.format)
			}

			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for format %q: %v", tc.format, err)
			}
		})
	}
}

// =============================================================================
// Help and Usage Tests
// =============================================================================

// TestTypesCmd_Help verifies help output shows usage information.
func TestTypesCmd_Help(t *testing.T) {
	cmd := newTestTypesCmd()
	output, err := executeTypesCommand(cmd, "types", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"types",    // Command name
		"--format", // Format flag
		"refine",   // Mention of refine command
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestTypesCmd_HelpShortFlag verifies help with short flag.
func TestTypesCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestTypesCmd()
	output, err := executeTypesCommand(cmd, "types", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "types") {
		t.Errorf("expected help output to mention 'types', got: %q", output)
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestTypesCmd_ExpectedFlags ensures the types command has expected flag structure.
func TestTypesCmd_ExpectedFlags(t *testing.T) {
	cmd := newTypesCmd()

	// Check expected flags exist
	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Error("expected types command to have 'format' flag")
	}

	// Check short flag
	if cmd.Flags().ShorthandLookup("f") == nil {
		t.Error("expected types command to have short flag -f for --format")
	}
}

// TestTypesCmd_DefaultFlagValues verifies default values for flags.
func TestTypesCmd_DefaultFlagValues(t *testing.T) {
	cmd := newTypesCmd()

	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("expected format flag to exist")
	}
	if formatFlag.DefValue != "text" {
		t.Errorf("expected default format to be 'text', got %q", formatFlag.DefValue)
	}
}

// TestTypesCmd_CommandMetadata verifies command metadata.
func TestTypesCmd_CommandMetadata(t *testing.T) {
	cmd := newTypesCmd()

	if cmd.Use != "types" {
		t.Errorf("expected Use to be 'types', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if cmd.Long == "" {
		t.Error("expected Long description to be set")
	}
}

// =============================================================================
// No Proof Required Test
// =============================================================================

// TestTypesCmd_NoProofRequired verifies types command works without initialized proof.
// This is important because types are static schema information.
func TestTypesCmd_NoProofRequired(t *testing.T) {
	// Execute types command without any proof directory setup
	cmd := newTypesCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected types command to work without proof, got: %v", err)
	}

	output := buf.String()

	// Should still list all types
	if !strings.Contains(output, "claim") {
		t.Errorf("expected output to contain 'claim', got: %q", output)
	}
}

// =============================================================================
// Type Order Tests
// =============================================================================

// TestTypesCmd_ConsistentOrder verifies types are listed in consistent order.
func TestTypesCmd_ConsistentOrder(t *testing.T) {
	// Run command multiple times and verify order is consistent
	var firstOutput string

	for i := 0; i < 3; i++ {
		cmd := newTestTypesCmd()
		output, err := executeTypesCommand(cmd, "types")

		if err != nil {
			t.Fatalf("run %d: expected no error, got: %v", i, err)
		}

		if i == 0 {
			firstOutput = output
		} else if output != firstOutput {
			t.Errorf("run %d: output differs from first run\nFirst: %q\nThis: %q", i, firstOutput, output)
		}
	}
}

// TestTypesCmd_JSONConsistentOrder verifies JSON types are in consistent order.
func TestTypesCmd_JSONConsistentOrder(t *testing.T) {
	// Run command multiple times and verify JSON order is consistent
	var firstTypes []string

	for i := 0; i < 3; i++ {
		cmd := newTestTypesCmd()
		output, err := executeTypesCommand(cmd, "types", "-f", "json")

		if err != nil {
			t.Fatalf("run %d: expected no error, got: %v", i, err)
		}

		var result struct {
			Types []struct {
				Type string `json:"type"`
			} `json:"types"`
		}

		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("run %d: failed to parse JSON: %v", i, err)
		}

		var types []string
		for _, nt := range result.Types {
			types = append(types, nt.Type)
		}

		if i == 0 {
			firstTypes = types
		} else {
			if len(types) != len(firstTypes) {
				t.Errorf("run %d: type count differs", i)
				continue
			}
			for j := range types {
				if types[j] != firstTypes[j] {
					t.Errorf("run %d: type order differs at position %d: %q vs %q", i, j, types[j], firstTypes[j])
				}
			}
		}
	}
}
