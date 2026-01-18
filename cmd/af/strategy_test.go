//go:build !integration

// Package main contains TDD tests for the af strategy command.
// The strategy command helps provers structure their proofs with common
// strategies and templates for direct proof, contradiction, induction,
// cases, and contrapositive.
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

// newTestStrategyCmd creates a fresh root command with the strategy subcommand for testing.
func newTestStrategyCmd() *cobra.Command {
	cmd := newTestRootCmd()

	strategyCmd := newStrategyCmd()
	cmd.AddCommand(strategyCmd)

	return cmd
}

// executeStrategyCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeStrategyCommand(root *cobra.Command, args ...string) (string, error) {
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

// TestStrategyCmd_ListDisplaysStrategies tests that 'strategy list' shows strategies.
func TestStrategyCmd_ListDisplaysStrategies(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "list")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should have non-empty output
	if len(output) == 0 {
		t.Error("expected non-empty strategy list output")
	}

	// Should list the required strategies
	required := []string{"direct", "contradiction", "induction", "cases", "contrapositive"}
	for _, name := range required {
		if !strings.Contains(strings.ToLower(output), name) {
			t.Errorf("expected strategy list to include %q", name)
		}
	}
}

// TestStrategyCmd_ListShowsDescriptions tests that list shows strategy descriptions.
func TestStrategyCmd_ListShowsDescriptions(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "list")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show descriptions (not just names)
	// Descriptions typically include words like "prove", "assume", etc.
	lower := strings.ToLower(output)
	hasDescriptions := strings.Contains(lower, "prove") ||
		strings.Contains(lower, "assume") ||
		strings.Contains(lower, "induction") ||
		strings.Contains(lower, "cases")

	if !hasDescriptions {
		t.Error("expected strategy list to show descriptions")
	}
}

// TestStrategyCmd_SuggestAnalyzesConjecture tests 'strategy suggest'.
func TestStrategyCmd_SuggestAnalyzesConjecture(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "suggest", "For all n, n + 0 = n")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(output) == 0 {
		t.Error("expected non-empty suggest output")
	}

	// Should suggest induction for this conjecture
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "induction") {
		t.Error("expected suggest to recommend induction for universal quantification")
	}
}

// TestStrategyCmd_SuggestShowsReasons tests that suggest shows reasons.
func TestStrategyCmd_SuggestShowsReasons(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "suggest", "If n^2 is even, then n is even")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show reasons for suggestions
	lower := strings.ToLower(output)
	hasReasons := strings.Contains(lower, "because") ||
		strings.Contains(lower, "reason") ||
		strings.Contains(lower, "suggest") ||
		strings.Contains(lower, "implication")

	if !hasReasons {
		t.Error("expected suggest to show reasons for recommendations")
	}
}

// TestStrategyCmd_ApplyGeneratesSkeleton tests 'strategy apply'.
func TestStrategyCmd_ApplyGeneratesSkeleton(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "apply", "induction", "For all n, P(n)")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(output) == 0 {
		t.Error("expected non-empty skeleton output")
	}

	// Should include base case and inductive step
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "base") {
		t.Error("expected induction skeleton to include base case")
	}
	if !strings.Contains(lower, "inductive") {
		t.Error("expected induction skeleton to include inductive step")
	}
}

// TestStrategyCmd_ApplyIncludesConjecture tests that apply includes the conjecture.
func TestStrategyCmd_ApplyIncludesConjecture(t *testing.T) {
	cmd := newTestStrategyCmd()
	conjecture := "n + m = m + n"
	output, err := executeStrategyCommand(cmd, "strategy", "apply", "direct", conjecture)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, conjecture) {
		t.Error("expected skeleton to include the conjecture")
	}
}

// TestStrategyCmd_ApplyUnknownStrategy tests error for unknown strategy.
func TestStrategyCmd_ApplyUnknownStrategy(t *testing.T) {
	cmd := newTestStrategyCmd()
	_, err := executeStrategyCommand(cmd, "strategy", "apply", "nonexistent", "Some conjecture")

	if err == nil {
		t.Error("expected error for unknown strategy")
	}

	if !strings.Contains(err.Error(), "unknown strategy") {
		t.Errorf("expected error to mention 'unknown strategy', got: %v", err)
	}
}

// TestStrategyCmd_SuggestRequiresConjecture tests that suggest requires a conjecture.
func TestStrategyCmd_SuggestRequiresConjecture(t *testing.T) {
	cmd := newTestStrategyCmd()
	_, err := executeStrategyCommand(cmd, "strategy", "suggest")

	if err == nil {
		t.Error("expected error when no conjecture provided")
	}
}

// TestStrategyCmd_ApplyRequiresArgs tests that apply requires strategy and conjecture.
func TestStrategyCmd_ApplyRequiresArgs(t *testing.T) {
	cmd := newTestStrategyCmd()

	// Missing both
	_, err := executeStrategyCommand(cmd, "strategy", "apply")
	if err == nil {
		t.Error("expected error when no arguments provided to apply")
	}

	// Missing conjecture
	cmd = newTestStrategyCmd()
	_, err = executeStrategyCommand(cmd, "strategy", "apply", "induction")
	if err == nil {
		t.Error("expected error when no conjecture provided to apply")
	}
}

// =============================================================================
// JSON Output Tests
// =============================================================================

// TestStrategyCmd_ListJSON tests JSON output format for list.
func TestStrategyCmd_ListJSON(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "list", "--format", "json")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v", err)
	}
}

// TestStrategyCmd_SuggestJSON tests JSON output format for suggest.
func TestStrategyCmd_SuggestJSON(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "suggest", "--format", "json", "For all n, P(n)")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v", err)
	}
}

// =============================================================================
// Command Metadata Tests
// =============================================================================

// TestStrategyCmd_CommandMetadata verifies command metadata.
func TestStrategyCmd_CommandMetadata(t *testing.T) {
	cmd := newStrategyCmd()

	if cmd.Use != "strategy" {
		t.Errorf("expected Use to be 'strategy', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestStrategyCmd_HasSubcommands verifies strategy has expected subcommands.
func TestStrategyCmd_HasSubcommands(t *testing.T) {
	cmd := newStrategyCmd()
	subcommands := cmd.Commands()

	// Should have list, suggest, and apply subcommands
	names := make(map[string]bool)
	for _, sub := range subcommands {
		names[sub.Name()] = true
	}

	required := []string{"list", "suggest", "apply"}
	for _, name := range required {
		if !names[name] {
			t.Errorf("expected subcommand %q to exist", name)
		}
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestStrategyCmd_Help verifies help output.
func TestStrategyCmd_Help(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include command name and subcommands
	if !strings.Contains(output, "strategy") {
		t.Error("expected help output to contain 'strategy'")
	}
	if !strings.Contains(output, "list") {
		t.Error("expected help output to mention 'list' subcommand")
	}
}

// TestStrategyCmd_ListHelp verifies list subcommand help.
func TestStrategyCmd_ListHelp(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "list", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	if !strings.Contains(output, "list") {
		t.Error("expected help output to contain 'list'")
	}
}

// TestStrategyCmd_SuggestHelp verifies suggest subcommand help.
func TestStrategyCmd_SuggestHelp(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "suggest", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	if !strings.Contains(output, "suggest") {
		t.Error("expected help output to contain 'suggest'")
	}
}

// TestStrategyCmd_ApplyHelp verifies apply subcommand help.
func TestStrategyCmd_ApplyHelp(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "apply", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	if !strings.Contains(output, "apply") {
		t.Error("expected help output to contain 'apply'")
	}
}

// =============================================================================
// Works Without Proof Directory Tests
// =============================================================================

// TestStrategyCmd_ListWorksWithoutProof tests that list works without proof.
func TestStrategyCmd_ListWorksWithoutProof(t *testing.T) {
	cmd := newTestStrategyCmd()
	// Execute without being in a proof directory
	_, err := executeStrategyCommand(cmd, "strategy", "list")

	// Strategy should work without a proof directory - it's static reference data
	if err != nil {
		t.Fatalf("expected list to work without proof directory, got error: %v", err)
	}
}

// TestStrategyCmd_SuggestWorksWithoutProof tests that suggest works without proof.
func TestStrategyCmd_SuggestWorksWithoutProof(t *testing.T) {
	cmd := newTestStrategyCmd()
	_, err := executeStrategyCommand(cmd, "strategy", "suggest", "Some conjecture")

	if err != nil {
		t.Fatalf("expected suggest to work without proof directory, got error: %v", err)
	}
}

// TestStrategyCmd_ApplyWorksWithoutProof tests that apply works without proof.
func TestStrategyCmd_ApplyWorksWithoutProof(t *testing.T) {
	cmd := newTestStrategyCmd()
	_, err := executeStrategyCommand(cmd, "strategy", "apply", "direct", "Some conjecture")

	if err != nil {
		t.Fatalf("expected apply to work without proof directory, got error: %v", err)
	}
}

// =============================================================================
// Strategy-Specific Apply Tests
// =============================================================================

// TestStrategyCmd_ApplyDirect tests applying direct proof strategy.
func TestStrategyCmd_ApplyDirect(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "apply", "direct", "2 + 2 = 4")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "direct") {
		t.Error("expected skeleton to mention direct proof")
	}
}

// TestStrategyCmd_ApplyContradiction tests applying contradiction strategy.
func TestStrategyCmd_ApplyContradiction(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "apply", "contradiction", "There is no largest prime")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "assume") {
		t.Error("expected contradiction skeleton to mention assuming negation")
	}
	if !strings.Contains(lower, "contradict") {
		t.Error("expected contradiction skeleton to mention deriving contradiction")
	}
}

// TestStrategyCmd_ApplyCases tests applying cases strategy.
func TestStrategyCmd_ApplyCases(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "apply", "cases", "n is even or n is odd")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "case") {
		t.Error("expected cases skeleton to mention cases")
	}
}

// TestStrategyCmd_ApplyContrapositive tests applying contrapositive strategy.
func TestStrategyCmd_ApplyContrapositive(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "apply", "contrapositive", "If P then Q")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "not") && !strings.Contains(lower, "negat") {
		t.Error("expected contrapositive skeleton to mention negation")
	}
}

// =============================================================================
// Suggestion Quality Tests
// =============================================================================

// TestStrategyCmd_SuggestInductionForNaturals tests induction suggestion.
func TestStrategyCmd_SuggestInductionForNaturals(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "suggest", "For all natural numbers n, the sum 1+2+...+n equals n(n+1)/2")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "induction") {
		t.Error("expected induction to be suggested for sum formula")
	}
}

// TestStrategyCmd_SuggestContradictionForNonExistence tests contradiction suggestion.
func TestStrategyCmd_SuggestContradictionForNonExistence(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "suggest", "sqrt(2) is not rational")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "contradiction") {
		t.Error("expected contradiction to be suggested for non-existence")
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestStrategyCmd_ListTextFormat tests default text format.
func TestStrategyCmd_ListTextFormat(t *testing.T) {
	cmd := newTestStrategyCmd()
	output, err := executeStrategyCommand(cmd, "strategy", "list")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should not be JSON (should not start with { or [)
	trimmed := strings.TrimSpace(output)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		t.Error("expected text format by default, got JSON-like output")
	}
}

// TestStrategyCmd_InvalidFormat tests error for invalid format.
func TestStrategyCmd_InvalidFormat(t *testing.T) {
	cmd := newTestStrategyCmd()
	_, err := executeStrategyCommand(cmd, "strategy", "list", "--format", "invalid")

	if err == nil {
		t.Error("expected error for invalid format")
	}
}
