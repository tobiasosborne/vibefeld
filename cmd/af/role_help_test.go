package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestHelpCmd_RoleProver verifies prover role filtering shows only prover-relevant commands.
func TestHelpCmd_RoleProver(t *testing.T) {
	cmd := newHelpTestRootCmd()
	cmd.SetHelpCommand(newHelpCmd())

	output, err := executeCommand(cmd, "help", "--role", "prover")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain prover commands
	if !strings.Contains(output, "refine") {
		t.Error("expected prover output to contain 'refine'")
	}
	if !strings.Contains(output, "resolve-challenge") {
		t.Error("expected prover output to contain 'resolve-challenge'")
	}
	if !strings.Contains(output, "request-def") {
		t.Error("expected prover output to contain 'request-def'")
	}
	if !strings.Contains(output, "amend") {
		t.Error("expected prover output to contain 'amend'")
	}

	// Should contain shared commands
	if !strings.Contains(output, "claim") {
		t.Error("expected prover output to contain 'claim'")
	}
	if !strings.Contains(output, "jobs") {
		t.Error("expected prover output to contain 'jobs'")
	}

	// Should NOT contain verifier-specific action section
	if strings.Contains(output, "Verifier Actions:") {
		t.Error("prover output should not contain 'Verifier Actions:' section")
	}

	// Should NOT contain admin commands
	if strings.Contains(output, "Administration:") {
		t.Error("prover output should not contain 'Administration:' section")
	}

	// Should contain header indicating prover role
	if !strings.Contains(output, "Commands for prover role:") {
		t.Error("expected prover output to contain role header")
	}
}

// TestHelpCmd_RoleVerifier verifies verifier role filtering shows only verifier-relevant commands.
func TestHelpCmd_RoleVerifier(t *testing.T) {
	cmd := newHelpTestRootCmd()
	cmd.SetHelpCommand(newHelpCmd())

	output, err := executeCommand(cmd, "help", "--role", "verifier")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain verifier commands
	if !strings.Contains(output, "accept") {
		t.Error("expected verifier output to contain 'accept'")
	}
	if !strings.Contains(output, "challenge") {
		t.Error("expected verifier output to contain 'challenge'")
	}

	// Should contain shared commands
	if !strings.Contains(output, "claim") {
		t.Error("expected verifier output to contain 'claim'")
	}
	if !strings.Contains(output, "jobs") {
		t.Error("expected verifier output to contain 'jobs'")
	}

	// Should NOT contain prover-specific action section
	if strings.Contains(output, "Prover Actions:") {
		t.Error("verifier output should not contain 'Prover Actions:' section")
	}

	// Should NOT contain admin commands
	if strings.Contains(output, "Administration:") {
		t.Error("verifier output should not contain 'Administration:' section")
	}

	// Should contain header indicating verifier role
	if !strings.Contains(output, "Commands for verifier role:") {
		t.Error("expected verifier output to contain role header")
	}
}

// TestHelpCmd_NoRole verifies help without role shows all command categories.
func TestHelpCmd_NoRole(t *testing.T) {
	cmd := newHelpTestRootCmd()
	cmd.SetHelpCommand(newHelpCmd())

	output, err := executeCommand(cmd, "help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain all sections
	expectedSections := []string{
		"Prover Actions:",
		"Verifier Actions:",
		"Job Management:",
		"Reference:",
		"Administration:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("expected output to contain section %q", section)
		}
	}

	// Should include hint about role filtering
	if !strings.Contains(output, "af help --role prover") {
		t.Error("expected output to contain role filtering hint")
	}
}

// TestHelpCmd_InvalidRole verifies invalid role returns an error.
func TestHelpCmd_InvalidRole(t *testing.T) {
	cmd := newHelpTestRootCmd()
	cmd.SetHelpCommand(newHelpCmd())

	_, err := executeCommand(cmd, "help", "--role", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}

	if !strings.Contains(err.Error(), "invalid role") {
		t.Errorf("expected error to mention 'invalid role', got: %v", err)
	}
}

// TestHelpCmd_SpecificCommand verifies help for a specific command works.
func TestHelpCmd_SpecificCommand(t *testing.T) {
	cmd := newHelpTestRootCmd()
	cmd.SetHelpCommand(newHelpCmd())

	// Add a test subcommand
	testCmd := &cobra.Command{
		Use:   "testcmd",
		Short: "A test command",
		Long:  "A test command with a longer description.",
	}
	cmd.AddCommand(testCmd)

	output, err := executeCommand(cmd, "help", "testcmd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "A test command with a longer description") {
		t.Error("expected specific command help to show its long description")
	}
}

// TestHelpCmd_UnknownCommand verifies help for unknown command returns error.
func TestHelpCmd_UnknownCommand(t *testing.T) {
	cmd := newHelpTestRootCmd()
	cmd.SetHelpCommand(newHelpCmd())

	_, err := executeCommand(cmd, "help", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}

	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected error to mention 'unknown command', got: %v", err)
	}
}

// TestCommandRoles verifies commandRoles map has expected categorizations.
func TestCommandRoles(t *testing.T) {
	// Verify prover commands
	proverCmds := []string{"refine", "amend", "request-def", "resolve-challenge"}
	for _, cmd := range proverCmds {
		if role, ok := commandRoles[cmd]; !ok || role != RoleProver {
			t.Errorf("expected %q to be prover role, got %q", cmd, role)
		}
	}

	// Verify verifier commands
	verifierCmds := []string{"accept", "challenge"}
	for _, cmd := range verifierCmds {
		if role, ok := commandRoles[cmd]; !ok || role != RoleVerifier {
			t.Errorf("expected %q to be verifier role, got %q", cmd, role)
		}
	}

	// Verify shared commands
	sharedCmds := []string{"claim", "release", "extend-claim", "jobs"}
	for _, cmd := range sharedCmds {
		if role, ok := commandRoles[cmd]; !ok || role != RoleShared {
			t.Errorf("expected %q to be shared role, got %q", cmd, role)
		}
	}
}

// newHelpTestRootCmd creates a minimal root command for help testing.
// This is separate from newTestRootCmd in root_test.go to add more commands.
func newHelpTestRootCmd() *cobra.Command {
	cmd := newTestRootCmd()

	// Add representative commands for each category
	cmd.AddCommand(&cobra.Command{Use: "refine", Short: "Refine a node"})
	cmd.AddCommand(&cobra.Command{Use: "amend", Short: "Amend a node"})
	cmd.AddCommand(&cobra.Command{Use: "request-def", Short: "Request definition"})
	cmd.AddCommand(&cobra.Command{Use: "resolve-challenge", Short: "Resolve challenge"})
	cmd.AddCommand(&cobra.Command{Use: "accept", Short: "Accept a node"})
	cmd.AddCommand(&cobra.Command{Use: "challenge", Short: "Challenge a node"})
	cmd.AddCommand(&cobra.Command{Use: "claim", Short: "Claim a job"})
	cmd.AddCommand(&cobra.Command{Use: "release", Short: "Release a job"})
	cmd.AddCommand(&cobra.Command{Use: "extend-claim", Short: "Extend claim"})
	cmd.AddCommand(&cobra.Command{Use: "jobs", Short: "List jobs"})
	cmd.AddCommand(&cobra.Command{Use: "status", Short: "Show status"})
	cmd.AddCommand(&cobra.Command{Use: "init", Short: "Initialize proof"})

	return cmd
}
