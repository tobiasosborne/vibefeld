//go:build !integration

// Package main contains TDD tests for the af tutorial command.
// The tutorial command provides a step-by-step guide for how to prove
// something from start to finish using the AF framework.
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestTutorialCmd creates a fresh root command with the tutorial subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestTutorialCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	tutorialCmd := newTutorialCmd()
	cmd.AddCommand(tutorialCmd)

	return cmd
}

// executeTutorialCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeTutorialCommand(root *cobra.Command, args ...string) (string, error) {
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

// TestTutorialCmd_DisplaysTutorial tests that tutorial displays guide content.
func TestTutorialCmd_DisplaysTutorial(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should have non-empty output
	if len(output) == 0 {
		t.Error("expected non-empty tutorial output")
	}

	// Should be substantial (a real tutorial has meaningful content)
	if len(output) < 500 {
		t.Errorf("tutorial output seems too short (%d chars), expected substantial guide", len(output))
	}
}

// TestTutorialCmd_WorksWithoutProofDirectory tests that tutorial works without initialized proof.
func TestTutorialCmd_WorksWithoutProofDirectory(t *testing.T) {
	cmd := newTestTutorialCmd()
	// Execute without being in a proof directory
	output, err := executeTutorialCommand(cmd, "tutorial")

	// Tutorial should work without a proof directory - it's just documentation
	if err != nil {
		t.Fatalf("expected tutorial to work without proof directory, got error: %v", err)
	}

	if len(output) == 0 {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// Content Tests - Key Commands Must Be Covered
// =============================================================================

// TestTutorialCmd_CoversInitCommand tests that tutorial explains af init.
func TestTutorialCmd_CoversInitCommand(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Must cover af init command
	if !strings.Contains(output, "init") {
		t.Error("tutorial must cover 'af init' command")
	}
}

// TestTutorialCmd_CoversStatusCommand tests that tutorial explains af status.
func TestTutorialCmd_CoversStatusCommand(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Must cover af status command
	if !strings.Contains(output, "status") {
		t.Error("tutorial must cover 'af status' command")
	}
}

// TestTutorialCmd_CoversClaimCommand tests that tutorial explains af claim.
func TestTutorialCmd_CoversClaimCommand(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Must cover af claim command
	if !strings.Contains(output, "claim") {
		t.Error("tutorial must cover 'af claim' command")
	}
}

// TestTutorialCmd_CoversRefineCommand tests that tutorial explains af refine.
func TestTutorialCmd_CoversRefineCommand(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Must cover af refine command
	if !strings.Contains(output, "refine") {
		t.Error("tutorial must cover 'af refine' command")
	}
}

// TestTutorialCmd_CoversAcceptCommand tests that tutorial explains af accept.
func TestTutorialCmd_CoversAcceptCommand(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Must cover af accept command
	if !strings.Contains(output, "accept") {
		t.Error("tutorial must cover 'af accept' command")
	}
}

// TestTutorialCmd_CoversChallengeCommand tests that tutorial explains af challenge.
func TestTutorialCmd_CoversChallengeCommand(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Must cover af challenge command
	if !strings.Contains(output, "challenge") {
		t.Error("tutorial must cover 'af challenge' command")
	}
}

// TestTutorialCmd_CoversResolveChallengeCommand tests that tutorial explains af resolve-challenge.
func TestTutorialCmd_CoversResolveChallengeCommand(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Must cover af resolve-challenge command
	if !strings.Contains(output, "resolve") {
		t.Error("tutorial must cover 'af resolve-challenge' command")
	}
}

// =============================================================================
// Workflow Coverage Tests
// =============================================================================

// TestTutorialCmd_ExplainsProverRole tests that tutorial explains the prover role.
func TestTutorialCmd_ExplainsProverRole(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should explain prover role
	outputLower := strings.ToLower(output)
	if !strings.Contains(outputLower, "prover") {
		t.Error("tutorial should explain the prover role")
	}
}

// TestTutorialCmd_ExplainsVerifierRole tests that tutorial explains the verifier role.
func TestTutorialCmd_ExplainsVerifierRole(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should explain verifier role
	outputLower := strings.ToLower(output)
	if !strings.Contains(outputLower, "verifier") {
		t.Error("tutorial should explain the verifier role")
	}
}

// TestTutorialCmd_HasStepByStepStructure tests that tutorial has numbered steps.
func TestTutorialCmd_HasStepByStepStructure(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should have numbered steps or clear section markers
	hasSteps := strings.Contains(output, "Step 1") ||
		strings.Contains(output, "step 1") ||
		strings.Contains(output, "1.") ||
		strings.Contains(output, "1)") ||
		strings.Contains(output, "===")

	if !hasSteps {
		t.Error("tutorial should have step-by-step structure with numbered steps or sections")
	}
}

// =============================================================================
// Command Metadata Tests
// =============================================================================

// TestTutorialCmd_CommandMetadata verifies command metadata.
func TestTutorialCmd_CommandMetadata(t *testing.T) {
	cmd := newTutorialCmd()

	if cmd.Use != "tutorial" {
		t.Errorf("expected Use to be 'tutorial', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestTutorialCmd_HasLongDescription verifies command has long description.
func TestTutorialCmd_HasLongDescription(t *testing.T) {
	cmd := newTutorialCmd()

	// Long description should explain what the tutorial covers
	if cmd.Long == "" {
		t.Error("expected Long description to be set for tutorial command")
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestTutorialCmd_Help verifies help output shows usage information.
func TestTutorialCmd_Help(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include command name
	if !strings.Contains(output, "tutorial") {
		t.Errorf("expected help output to contain 'tutorial', got: %q", output)
	}
}

// TestTutorialCmd_HelpShortFlag verifies help with short flag.
func TestTutorialCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "tutorial") {
		t.Errorf("expected help output to mention 'tutorial', got: %q", output)
	}
}

// =============================================================================
// Output Quality Tests
// =============================================================================

// TestTutorialCmd_OutputIsReadable tests that output is human-readable.
func TestTutorialCmd_OutputIsReadable(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should have multiple lines
	lines := strings.Split(output, "\n")
	if len(lines) < 10 {
		t.Errorf("expected multi-line readable output, got %d lines", len(lines))
	}

	// Should not be JSON
	if strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Error("tutorial output should be human-readable text, not JSON")
	}
}

// TestTutorialCmd_HasClearSections tests that tutorial has clear sections.
func TestTutorialCmd_HasClearSections(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should have section headers or clear organization
	hasHeaders := strings.Contains(output, "===") ||
		strings.Contains(output, "---") ||
		strings.Contains(output, "Step") ||
		strings.Contains(output, "STEP")

	if !hasHeaders {
		t.Error("tutorial should have clear section headers")
	}
}

// TestTutorialCmd_RepeatedCallsSameOutput tests that repeated calls produce same output.
func TestTutorialCmd_RepeatedCallsSameOutput(t *testing.T) {
	cmd1 := newTestTutorialCmd()
	output1, err := executeTutorialCommand(cmd1, "tutorial")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	cmd2 := newTestTutorialCmd()
	output2, err := executeTutorialCommand(cmd2, "tutorial")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if output1 != output2 {
		t.Error("repeated calls should produce identical output (tutorial is static)")
	}
}

// =============================================================================
// Example Usage Tests
// =============================================================================

// TestTutorialCmd_ContainsExampleCommands tests that tutorial includes example commands.
func TestTutorialCmd_ContainsExampleCommands(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain example command invocations
	hasExamples := strings.Contains(output, "af init") ||
		strings.Contains(output, "af status") ||
		strings.Contains(output, "af claim") ||
		strings.Contains(output, "af refine")

	if !hasExamples {
		t.Error("tutorial should contain example command invocations like 'af init'")
	}
}

// TestTutorialCmd_ExplainsNodeHierarchy tests that tutorial explains the node ID hierarchy.
func TestTutorialCmd_ExplainsNodeHierarchy(t *testing.T) {
	cmd := newTestTutorialCmd()
	output, err := executeTutorialCommand(cmd, "tutorial")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should explain hierarchical node IDs (e.g., 1, 1.1, 1.2)
	outputLower := strings.ToLower(output)
	hasNodeExplanation := strings.Contains(outputLower, "node") ||
		strings.Contains(output, "1.1") ||
		strings.Contains(outputLower, "hierarchical") ||
		strings.Contains(outputLower, "child")

	if !hasNodeExplanation {
		t.Error("tutorial should explain node IDs and hierarchy")
	}
}
