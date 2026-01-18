// Package main provides tests for the persistent flags.
package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestPersistentFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "verbose flag long form",
			args: []string{"--verbose", "--help"},
		},
		{
			name: "dry-run flag",
			args: []string{"--dry-run", "--help"},
		},
		{
			name: "both flags together",
			args: []string{"--verbose", "--dry-run", "--help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh root command for each test to avoid state pollution
			cmd := newTestRootCmd()
			cmd.PersistentFlags().Bool("verbose", false, "Enable verbose output for debugging")
			cmd.PersistentFlags().Bool("dry-run", false, "Preview changes without making them")

			cmd.SetArgs(tt.args)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			err := cmd.Execute()
			if err != nil {
				t.Errorf("Execute() with args %v failed: %v", tt.args, err)
			}
		})
	}
}

func TestIsVerbose(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().Bool("verbose", false, "Enable verbose output")

	// Default should be false
	if isVerbose(cmd) {
		t.Error("isVerbose() should return false by default")
	}

	// Set to true
	cmd.SetArgs([]string{"--verbose"})
	_ = cmd.Execute()

	// After parsing with --verbose, should be true
	v, _ := cmd.Flags().GetBool("verbose")
	if !v {
		t.Error("verbose flag should be true after parsing with --verbose")
	}
}

func TestIsDryRun(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().Bool("dry-run", false, "Preview changes")

	// Default should be false
	if isDryRun(cmd) {
		t.Error("isDryRun() should return false by default")
	}

	// Set to true
	cmd.SetArgs([]string{"--dry-run"})
	_ = cmd.Execute()

	// After parsing with --dry-run, should be true
	d, _ := cmd.Flags().GetBool("dry-run")
	if !d {
		t.Error("dry-run flag should be true after parsing with --dry-run")
	}
}

func TestFlagsInheritedBySubcommands(t *testing.T) {
	// Create root command with persistent flags
	root := newTestRootCmd()
	root.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	root.PersistentFlags().Bool("dry-run", false, "Preview changes")

	// Create a subcommand
	var verboseValue, dryRunValue bool
	sub := &cobra.Command{
		Use: "sub",
		Run: func(cmd *cobra.Command, args []string) {
			verboseValue, _ = cmd.Flags().GetBool("verbose")
			dryRunValue, _ = cmd.Flags().GetBool("dry-run")
		},
	}
	root.AddCommand(sub)

	// Test that subcommand inherits flags
	root.SetArgs([]string{"--verbose", "--dry-run", "sub"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if !verboseValue {
		t.Error("subcommand should inherit --verbose flag")
	}
	if !dryRunValue {
		t.Error("subcommand should inherit --dry-run flag")
	}
}
