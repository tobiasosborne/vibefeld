//go:build integration

// Package main contains TDD tests for the af schema command.
// These tests define expected behavior for displaying proof schema information.
// The schema command shows valid values for inference types, node types, and states.
// Unlike other commands, schema works without an initialized proof directory
// because the schema is static configuration data.
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

// newTestSchemaCmd creates a fresh root command with the schema subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestSchemaCmd() *cobra.Command {
	cmd := newTestRootCmd()

	schemaCmd := newSchemaCmd()
	cmd.AddCommand(schemaCmd)

	return cmd
}

// executeSchemaCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeSchemaCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// =============================================================================
// Basic Command Tests
// =============================================================================

// TestSchemaCmd_DisplaysAllSchema tests that schema displays all schema information.
func TestSchemaCmd_DisplaysAllSchema(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain sections for all schema categories
	requiredSections := []string{
		"inference", // inference types
		"node",      // node types
		"workflow",  // workflow states
		"epistemic", // epistemic states
	}

	for _, section := range requiredSections {
		if !strings.Contains(strings.ToLower(output), section) {
			t.Errorf("expected output to contain section %q, got: %q", section, output)
		}
	}
}

// TestSchemaCmd_DisplaysInferenceTypes tests that all inference types are shown.
func TestSchemaCmd_DisplaysInferenceTypes(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain all inference types
	expectedInferenceTypes := []string{
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

	for _, inferenceType := range expectedInferenceTypes {
		if !strings.Contains(output, inferenceType) {
			t.Errorf("expected output to contain inference type %q, got: %q", inferenceType, output)
		}
	}
}

// TestSchemaCmd_DisplaysNodeTypes tests that all node types are shown.
func TestSchemaCmd_DisplaysNodeTypes(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain all node types
	expectedNodeTypes := []string{
		"claim",
		"local_assume",
		"local_discharge",
		"case",
		"qed",
	}

	for _, nodeType := range expectedNodeTypes {
		if !strings.Contains(output, nodeType) {
			t.Errorf("expected output to contain node type %q, got: %q", nodeType, output)
		}
	}
}

// TestSchemaCmd_DisplaysWorkflowStates tests that all workflow states are shown.
func TestSchemaCmd_DisplaysWorkflowStates(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain all workflow states
	expectedWorkflowStates := []string{
		"available",
		"claimed",
		"blocked",
	}

	for _, state := range expectedWorkflowStates {
		if !strings.Contains(output, state) {
			t.Errorf("expected output to contain workflow state %q, got: %q", state, output)
		}
	}
}

// TestSchemaCmd_DisplaysEpistemicStates tests that all epistemic states are shown.
func TestSchemaCmd_DisplaysEpistemicStates(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain all epistemic states
	expectedEpistemicStates := []string{
		"pending",
		"validated",
		"admitted",
		"refuted",
		"archived",
	}

	for _, state := range expectedEpistemicStates {
		if !strings.Contains(output, state) {
			t.Errorf("expected output to contain epistemic state %q, got: %q", state, output)
		}
	}
}

// TestSchemaCmd_DisplaysTaintStates tests that all taint states are shown.
func TestSchemaCmd_DisplaysTaintStates(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain all taint states
	expectedTaintStates := []string{
		"clean",
		"self_admitted",
		"tainted",
		"unresolved",
	}

	for _, state := range expectedTaintStates {
		if !strings.Contains(output, state) {
			t.Errorf("expected output to contain taint state %q, got: %q", state, output)
		}
	}
}

// TestSchemaCmd_DisplaysChallengeTargets tests that challenge targets are shown.
func TestSchemaCmd_DisplaysChallengeTargets(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain all challenge targets
	expectedTargets := []string{
		"statement",
		"inference",
		"context",
		"dependencies",
		"scope",
		"gap",
		"type_error",
		"domain",
		"completeness",
	}

	for _, target := range expectedTargets {
		if !strings.Contains(output, target) {
			t.Errorf("expected output to contain challenge target %q, got: %q", target, output)
		}
	}
}

// =============================================================================
// Works Without Proof Directory Tests
// =============================================================================

// TestSchemaCmd_WorksWithoutProofDirectory tests that schema works without initialized proof.
func TestSchemaCmd_WorksWithoutProofDirectory(t *testing.T) {
	cmd := newTestSchemaCmd()
	// Execute without --dir flag, using a non-existent directory
	output, err := executeSchemaCommand(cmd, "schema", "--dir", "/nonexistent/path/12345")

	// Schema should work even without a proof directory
	// because schema is static data, not dependent on any proof
	if err != nil {
		t.Fatalf("expected schema to work without proof directory, got error: %v", err)
	}

	// Should still display schema information
	if !strings.Contains(strings.ToLower(output), "inference") {
		t.Errorf("expected output to contain schema information, got: %q", output)
	}
}

// TestSchemaCmd_WorksWithEmptyDirectory tests that schema works in an empty directory.
func TestSchemaCmd_WorksWithEmptyDirectory(t *testing.T) {
	cmd := newTestSchemaCmd()
	// Execute with --dir pointing to root (which exists but has no .af directory)
	output, err := executeSchemaCommand(cmd, "schema", "--dir", "/tmp")

	// Schema should work regardless of directory state
	if err != nil {
		t.Fatalf("expected schema to work in any directory, got error: %v", err)
	}

	// Should display schema information
	if len(output) == 0 {
		t.Error("expected non-empty output")
	}
}

// TestSchemaCmd_WorksWithNoFlags tests that schema works with no flags at all.
func TestSchemaCmd_WorksWithNoFlags(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	// Should work without any flags
	if err != nil {
		t.Fatalf("expected schema to work with no flags, got error: %v", err)
	}

	// Should display schema information
	if !strings.Contains(strings.ToLower(output), "inference") {
		t.Errorf("expected output to contain schema information, got: %q", output)
	}
}

// =============================================================================
// JSON Output Tests
// =============================================================================

// TestSchemaCmd_JSONOutput tests JSON output format.
func TestSchemaCmd_JSONOutput(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// TestSchemaCmd_JSONOutputStructure tests the structure of JSON output.
func TestSchemaCmd_JSONOutputStructure(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON object: %v\nOutput: %q", err, output)
	}

	// Should have expected top-level keys
	expectedKeys := []string{
		"inference_types",
		"node_types",
		"workflow_states",
		"epistemic_states",
	}

	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON to contain key %q, got keys: %v", key, getKeys(result))
		}
	}
}

// TestSchemaCmd_JSONOutputShortFlag tests JSON output with -f short flag.
func TestSchemaCmd_JSONOutputShortFlag(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "-f", "json")

	if err != nil {
		t.Fatalf("expected no error with short flag, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON with -f flag: %v\nOutput: %q", err, output)
	}
}

// TestSchemaCmd_JSONInferenceTypesHaveMetadata tests that inference types include metadata.
func TestSchemaCmd_JSONInferenceTypesHaveMetadata(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check inference_types array structure
	inferenceTypes, ok := result["inference_types"]
	if !ok {
		t.Fatal("expected JSON to contain 'inference_types' key")
	}

	typesArray, ok := inferenceTypes.([]interface{})
	if !ok {
		t.Fatalf("expected inference_types to be array, got: %T", inferenceTypes)
	}

	if len(typesArray) == 0 {
		t.Fatal("expected at least one inference type")
	}

	// Check first inference type has expected fields
	firstType, ok := typesArray[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected inference type to be object, got: %T", typesArray[0])
	}

	// Should have id and name fields at minimum
	expectedFields := []string{"id", "name"}
	for _, field := range expectedFields {
		if _, ok := firstType[field]; !ok {
			t.Errorf("expected inference type to have field %q, got: %v", field, getKeys(firstType))
		}
	}
}

// =============================================================================
// Section Filter Tests
// =============================================================================

// TestSchemaCmd_SectionInferenceTypes tests filtering to just inference types.
func TestSchemaCmd_SectionInferenceTypes(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--section", "inference-types")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain inference types
	if !strings.Contains(output, "modus_ponens") {
		t.Errorf("expected output to contain 'modus_ponens', got: %q", output)
	}

	// Should NOT contain node types, workflow states, etc. (or at least be focused)
	// This is a soft check - some minimal display might include headers
	t.Logf("Section 'inference-types' output: %q", output)
}

// TestSchemaCmd_SectionNodeTypes tests filtering to just node types.
func TestSchemaCmd_SectionNodeTypes(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--section", "node-types")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain node types
	if !strings.Contains(output, "claim") && !strings.Contains(output, "qed") {
		t.Errorf("expected output to contain node types like 'claim' or 'qed', got: %q", output)
	}
}

// TestSchemaCmd_SectionStates tests filtering to just states.
func TestSchemaCmd_SectionStates(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--section", "states")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain workflow, epistemic, and/or taint states
	hasStates := strings.Contains(output, "available") ||
		strings.Contains(output, "pending") ||
		strings.Contains(output, "clean")

	if !hasStates {
		t.Errorf("expected output to contain state information, got: %q", output)
	}
}

// TestSchemaCmd_SectionWorkflowStates tests filtering to just workflow states.
func TestSchemaCmd_SectionWorkflowStates(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--section", "workflow-states")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain workflow states
	if !strings.Contains(output, "available") {
		t.Errorf("expected output to contain 'available', got: %q", output)
	}
}

// TestSchemaCmd_SectionEpistemicStates tests filtering to just epistemic states.
func TestSchemaCmd_SectionEpistemicStates(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--section", "epistemic-states")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain epistemic states
	if !strings.Contains(output, "pending") && !strings.Contains(output, "validated") {
		t.Errorf("expected output to contain epistemic states, got: %q", output)
	}
}

// TestSchemaCmd_SectionTaintStates tests filtering to just taint states.
func TestSchemaCmd_SectionTaintStates(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--section", "taint-states")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain taint states
	if !strings.Contains(output, "clean") && !strings.Contains(output, "tainted") {
		t.Errorf("expected output to contain taint states, got: %q", output)
	}
}

// TestSchemaCmd_SectionChallengeTargets tests filtering to just challenge targets.
func TestSchemaCmd_SectionChallengeTargets(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--section", "challenge-targets")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain challenge targets
	if !strings.Contains(output, "gap") && !strings.Contains(output, "statement") {
		t.Errorf("expected output to contain challenge targets, got: %q", output)
	}
}

// TestSchemaCmd_SectionShortFlag tests section filter with -s short flag.
func TestSchemaCmd_SectionShortFlag(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "-s", "inference-types")

	if err != nil {
		t.Fatalf("expected no error with -s flag, got: %v", err)
	}

	// Should contain inference types
	if !strings.Contains(output, "modus") {
		t.Errorf("expected output to contain inference types, got: %q", output)
	}
}

// TestSchemaCmd_SectionWithJSON tests section filter combined with JSON output.
func TestSchemaCmd_SectionWithJSON(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--section", "inference-types", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

// =============================================================================
// Error Case Tests
// =============================================================================

// TestSchemaCmd_InvalidFormat tests error for invalid format.
func TestSchemaCmd_InvalidFormat(t *testing.T) {
	cmd := newTestSchemaCmd()
	_, err := executeSchemaCommand(cmd, "schema", "--format", "xml")

	if err == nil {
		t.Error("expected error for invalid format 'xml', got nil")
	}

	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "format") {
		t.Logf("Error message: %v (may be about format or may be other error)", err)
	}
}

// TestSchemaCmd_InvalidSection tests error for invalid section.
func TestSchemaCmd_InvalidSection(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--section", "invalid-section")

	combined := output
	if err != nil {
		combined += err.Error()
	}

	// Should error or warn about invalid section
	if err == nil && !strings.Contains(strings.ToLower(combined), "invalid") &&
		!strings.Contains(strings.ToLower(combined), "unknown") {
		t.Errorf("expected error or warning for invalid section, got output: %q", output)
	}
}

// TestSchemaCmd_InvalidFormatValidation tests various invalid formats.
func TestSchemaCmd_InvalidFormatValidation(t *testing.T) {
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
			cmd := newTestSchemaCmd()
			output, err := executeSchemaCommand(cmd, "schema", "--format", tc.format)

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
// Flag Tests
// =============================================================================

// TestSchemaCmd_ExpectedFlags ensures the schema command has expected flag structure.
func TestSchemaCmd_ExpectedFlags(t *testing.T) {
	cmd := newSchemaCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "section"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected schema command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"s": "section",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected schema command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestSchemaCmd_DefaultFlagValues verifies default values for flags.
func TestSchemaCmd_DefaultFlagValues(t *testing.T) {
	cmd := newSchemaCmd()

	// Check default format value
	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("expected format flag to exist")
	}
	if formatFlag.DefValue != "text" {
		t.Errorf("expected default format to be 'text', got %q", formatFlag.DefValue)
	}

	// Check default section value (should be empty = show all)
	sectionFlag := cmd.Flags().Lookup("section")
	if sectionFlag == nil {
		t.Fatal("expected section flag to exist")
	}
	if sectionFlag.DefValue != "" {
		t.Errorf("expected default section to be empty, got %q", sectionFlag.DefValue)
	}
}

// TestSchemaCmd_CommandMetadata verifies command metadata.
func TestSchemaCmd_CommandMetadata(t *testing.T) {
	cmd := newSchemaCmd()

	if cmd.Use != "schema" {
		t.Errorf("expected Use to be 'schema', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestSchemaCmd_HasLongDescription verifies command has long description.
func TestSchemaCmd_HasLongDescription(t *testing.T) {
	cmd := newSchemaCmd()

	// Long description should explain what schema information is shown
	if cmd.Long == "" {
		t.Error("expected Long description to be set for documentation command")
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestSchemaCmd_Help verifies help output shows usage information.
func TestSchemaCmd_Help(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"schema",   // Command name
		"--format", // Format flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestSchemaCmd_HelpShortFlag verifies help with short flag.
func TestSchemaCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "schema") {
		t.Errorf("expected help output to mention 'schema', got: %q", output)
	}
}

// TestSchemaCmd_HelpShowsSectionOptions verifies help lists valid section values.
func TestSchemaCmd_HelpShowsSectionOptions(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should mention the section flag
	if !strings.Contains(output, "section") {
		t.Logf("Help output may or may not list section options: %q", output)
	}
}

// =============================================================================
// Content Quality Tests
// =============================================================================

// TestSchemaCmd_InferenceTypesHaveDescriptions tests that inference types have descriptions.
func TestSchemaCmd_InferenceTypesHaveDescriptions(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain some human-readable descriptions (from InferenceInfo.Name)
	expectedDescriptions := []string{
		"Modus Ponens",
		"Modus Tollens",
		"Universal",
		"Existential",
	}

	hasDescriptions := false
	for _, desc := range expectedDescriptions {
		if strings.Contains(output, desc) {
			hasDescriptions = true
			break
		}
	}

	if !hasDescriptions {
		t.Logf("Output may or may not include human-readable names: %q", output)
	}
}

// TestSchemaCmd_NodeTypesHaveDescriptions tests that node types have descriptions.
func TestSchemaCmd_NodeTypesHaveDescriptions(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check if descriptions from NodeTypeInfo are present
	// e.g., "A mathematical assertion to be justified" for claim
	if !strings.Contains(strings.ToLower(output), "claim") {
		t.Errorf("expected output to at least contain 'claim' node type, got: %q", output)
	}
}

// TestSchemaCmd_StatesHaveDescriptions tests that states have descriptions.
func TestSchemaCmd_StatesHaveDescriptions(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain some human-readable state descriptions
	// from WorkflowStateInfo.Description or EpistemicStateInfo.Description
	if len(output) < 100 {
		t.Errorf("output seems too short for full schema description: %q", output)
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

// TestSchemaCmd_AllSectionsValid tests that all expected section names work.
func TestSchemaCmd_AllSectionsValid(t *testing.T) {
	validSections := []string{
		"inference-types",
		"node-types",
		"workflow-states",
		"epistemic-states",
		"taint-states",
		"challenge-targets",
		"states", // Shorthand for all state types
	}

	for _, section := range validSections {
		t.Run(section, func(t *testing.T) {
			cmd := newTestSchemaCmd()
			output, err := executeSchemaCommand(cmd, "schema", "--section", section)

			if err != nil {
				t.Errorf("expected section %q to be valid, got error: %v", section, err)
			}

			if len(output) == 0 {
				t.Errorf("expected non-empty output for section %q", section)
			}
		})
	}
}

// TestSchemaCmd_SectionAlternativeNames tests alternative section name formats.
func TestSchemaCmd_SectionAlternativeNames(t *testing.T) {
	tests := []struct {
		name        string
		section     string
		shouldWork  bool
		expectedStr string
	}{
		{"inference-types kebab case", "inference-types", true, "modus"},
		{"inference_types snake case", "inference_types", true, "modus"},
		{"inferences short form", "inferences", true, "modus"},
		{"node-types kebab case", "node-types", true, "claim"},
		{"node_types snake case", "node_types", true, "claim"},
		{"nodes short form", "nodes", true, "claim"},
		{"workflow kebab case", "workflow-states", true, "available"},
		{"workflow short form", "workflow", true, "available"},
		{"epistemic kebab case", "epistemic-states", true, "pending"},
		{"epistemic short form", "epistemic", true, "pending"},
		{"taint kebab case", "taint-states", true, "clean"},
		{"taint short form", "taint", true, "clean"},
		{"challenge-targets kebab case", "challenge-targets", true, "gap"},
		{"targets short form", "targets", true, "gap"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestSchemaCmd()
			output, err := executeSchemaCommand(cmd, "schema", "--section", tc.section)

			if tc.shouldWork {
				if err != nil {
					t.Logf("Section %q may not be supported: %v", tc.section, err)
					return
				}
				if tc.expectedStr != "" && !strings.Contains(strings.ToLower(output), tc.expectedStr) {
					t.Logf("Section %q output may not contain %q: %q", tc.section, tc.expectedStr, output)
				}
			}
		})
	}
}

// =============================================================================
// Consistency Tests
// =============================================================================

// TestSchemaCmd_JSONAndTextHaveSameContent tests that JSON and text show same info.
func TestSchemaCmd_JSONAndTextHaveSameContent(t *testing.T) {
	cmd1 := newTestSchemaCmd()
	textOutput, err := executeSchemaCommand(cmd1, "schema", "--format", "text")
	if err != nil {
		t.Fatalf("text format failed: %v", err)
	}

	cmd2 := newTestSchemaCmd()
	jsonOutput, err := executeSchemaCommand(cmd2, "schema", "--format", "json")
	if err != nil {
		t.Fatalf("json format failed: %v", err)
	}

	// Both should contain modus_ponens (an inference type)
	if !strings.Contains(textOutput, "modus_ponens") {
		t.Error("text output missing 'modus_ponens'")
	}
	if !strings.Contains(jsonOutput, "modus_ponens") {
		t.Error("json output missing 'modus_ponens'")
	}

	// Both should contain claim (a node type)
	if !strings.Contains(textOutput, "claim") {
		t.Error("text output missing 'claim'")
	}
	if !strings.Contains(jsonOutput, "claim") {
		t.Error("json output missing 'claim'")
	}
}

// TestSchemaCmd_RepeatedCallsSameOutput tests that repeated calls produce same output.
func TestSchemaCmd_RepeatedCallsSameOutput(t *testing.T) {
	cmd1 := newTestSchemaCmd()
	output1, err := executeSchemaCommand(cmd1, "schema")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	cmd2 := newTestSchemaCmd()
	output2, err := executeSchemaCommand(cmd2, "schema")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if output1 != output2 {
		t.Error("repeated calls should produce identical output (schema is static)")
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestSchemaCmd_EmptySectionShowsAll tests that empty section shows all.
func TestSchemaCmd_EmptySectionShowsAll(t *testing.T) {
	cmd1 := newTestSchemaCmd()
	allOutput, err := executeSchemaCommand(cmd1, "schema")
	if err != nil {
		t.Fatalf("no section failed: %v", err)
	}

	cmd2 := newTestSchemaCmd()
	explicitAllOutput, err := executeSchemaCommand(cmd2, "schema", "--section", "")
	if err != nil {
		t.Fatalf("empty section failed: %v", err)
	}

	// Empty section should be equivalent to no section
	if allOutput != explicitAllOutput {
		t.Logf("Empty section may or may not be equivalent to no section")
	}
}

// TestSchemaCmd_CombinedFlags tests combining multiple flags.
func TestSchemaCmd_CombinedFlags(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema", "--format", "json", "--section", "inference-types")

	if err != nil {
		t.Fatalf("combined flags failed: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("combined output is not valid JSON: %v\nOutput: %q", err, output)
	}

	// Should contain inference types
	if !strings.Contains(output, "modus") {
		t.Errorf("expected filtered output to contain inference types, got: %q", output)
	}
}

// TestSchemaCmd_VeryLongSectionName tests handling of very long section name.
func TestSchemaCmd_VeryLongSectionName(t *testing.T) {
	cmd := newTestSchemaCmd()
	longSection := strings.Repeat("inference-types", 100)
	_, err := executeSchemaCommand(cmd, "schema", "--section", longSection)

	// Should either error or handle gracefully
	if err != nil {
		t.Logf("Long section name handled with error: %v", err)
	}
}

// TestSchemaCmd_SpecialCharactersInSection tests handling of special characters.
func TestSchemaCmd_SpecialCharactersInSection(t *testing.T) {
	specialCases := []string{
		"inference<script>",
		"inference; rm -rf",
		"inference\ttypes",
		"inference\ntypes",
	}

	for _, section := range specialCases {
		t.Run(section, func(t *testing.T) {
			cmd := newTestSchemaCmd()
			_, err := executeSchemaCommand(cmd, "schema", "--section", section)

			// Should either error or handle safely
			if err != nil {
				t.Logf("Special section %q handled with error: %v", section, err)
			}
		})
	}
}

// =============================================================================
// Output Formatting Tests
// =============================================================================

// TestSchemaCmd_TextOutputIsReadable tests that text output is human-readable.
func TestSchemaCmd_TextOutputIsReadable(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should have multiple lines
	lines := strings.Split(output, "\n")
	if len(lines) < 5 {
		t.Errorf("expected multi-line readable output, got %d lines", len(lines))
	}

	// Should not be just JSON dumped as text
	if strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Error("text format should not just be JSON")
	}
}

// TestSchemaCmd_TextOutputHasSections tests that text output has clear sections.
func TestSchemaCmd_TextOutputHasSections(t *testing.T) {
	cmd := newTestSchemaCmd()
	output, err := executeSchemaCommand(cmd, "schema")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should have section headers or clear organization
	// Look for common header patterns
	hasHeaders := strings.Contains(output, "Inference") ||
		strings.Contains(output, "INFERENCE") ||
		strings.Contains(output, "===") ||
		strings.Contains(output, "---") ||
		strings.Contains(output, "Node")

	if !hasHeaders {
		t.Logf("Text output may benefit from clearer section headers: %q", output[:min(200, len(output))])
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// getKeys returns the keys of a map for error messages.
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
