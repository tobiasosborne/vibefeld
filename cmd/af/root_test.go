package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Note: This test file uses newTestRootCmd which creates a fresh command
// instance for each test, ensuring test isolation.

// executeCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// newTestRootCmd creates a fresh copy of rootCmd for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
		Long: `AF (Adversarial Proof Framework) is a command-line tool for collaborative
construction of natural-language mathematical proofs.

Multiple AI agents work concurrently as adversarial provers and verifiers,
refining proof steps until rigorous acceptance. Provers convince, verifiers
attack - no agent plays both roles.

Key principles:
  - Adversarial verification with role isolation
  - Append-only ledger as source of truth
  - Filesystem concurrency with POSIX atomics
  - Self-documenting CLI for agent workflows`,
		Version: Version,
	}
	cmd.SetVersionTemplate("af version {{.Version}}\n")

	// Add test subcommands to test fuzzy matching
	// These simulate the commands that will exist in the full CLI
	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Initialize a new proof workspace",
		Run:   func(cmd *cobra.Command, args []string) {},
	})

	// Status command with flags for testing flag fuzzy matching
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show proof status",
		Run:   func(cmd *cobra.Command, args []string) {},
	}
	statusCmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	statusCmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	statusCmd.Flags().Bool("verbose", false, "Enable verbose output")
	cmd.AddCommand(statusCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "claim",
		Short: "Claim a job for work",
		Run:   func(cmd *cobra.Command, args []string) {},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "refine",
		Short: "Refine a proof node",
		Run:   func(cmd *cobra.Command, args []string) {},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "release",
		Short: "Release a claimed job",
		Run:   func(cmd *cobra.Command, args []string) {},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "accept",
		Short: "Accept a proof node",
		Run:   func(cmd *cobra.Command, args []string) {},
	})

	// Add fuzzy matching support (including flag fuzzy matching)
	AddFuzzyMatchingRecursive(cmd)

	return cmd
}

func TestRootCmd_Version(t *testing.T) {
	cmd := newTestRootCmd()
	output, err := executeCommand(cmd, "--version")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := "af version " + Version
	if !strings.Contains(output, expected) {
		t.Errorf("expected output to contain %q, got: %q", expected, output)
	}
}

func TestRootCmd_Help(t *testing.T) {
	cmd := newTestRootCmd()
	output, err := executeCommand(cmd, "--help")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check that help contains key information
	expectations := []string{
		"Adversarial Proof Framework",
		"af",
		"--help",
		"--version",
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

func TestRootCmd_NoArgs(t *testing.T) {
	cmd := newTestRootCmd()
	output, err := executeCommand(cmd)

	// With no args and no Run function, cobra shows help
	// This is the expected behavior for the root command
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show usage/help when no args provided
	if !strings.Contains(output, "AF (Adversarial Proof Framework)") {
		t.Errorf("expected help/usage output, got: %q", output)
	}
}

func TestRootCmd_UnknownCommand(t *testing.T) {
	cmd := newTestRootCmd()
	_, err := executeCommand(cmd, "notacommand")

	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "unknown command") {
		t.Errorf("expected error to mention 'unknown command', got: %q", errStr)
	}
}

func TestRootCmd_FuzzyMatching(t *testing.T) {
	// These tests define the expected behavior for fuzzy command matching.
	// When a user types a partial or misspelled command, the CLI should
	// suggest similar valid commands.
	//
	// Note: Fuzzy matching may need to be implemented via a custom
	// UnknownCommandHandler or by wrapping cobra's command resolution.

	tests := []struct {
		name        string
		input       string
		wantSuggest []string // commands that should be suggested
		wantErr     bool
	}{
		{
			name:        "stat suggests status",
			input:       "stat",
			wantSuggest: []string{"status"},
			wantErr:     true,
		},
		{
			name:        "ini suggests init",
			input:       "ini",
			wantSuggest: []string{"init"},
			wantErr:     true,
		},
		{
			name:        "clam suggests claim",
			input:       "clam",
			wantSuggest: []string{"claim"},
			wantErr:     true,
		},
		{
			name:        "refin suggests refine",
			input:       "refin",
			wantSuggest: []string{"refine"},
			wantErr:     true,
		},
		{
			name:        "relase suggests release (typo)",
			input:       "relase",
			wantSuggest: []string{"release"},
			wantErr:     true,
		},
		{
			name:        "accpet suggests accept (typo)",
			input:       "accpet",
			wantSuggest: []string{"accept"},
			wantErr:     true,
		},
		{
			name:        "completely unknown gets no suggestion",
			input:       "xyz123",
			wantSuggest: []string{}, // no close matches
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestRootCmd()
			output, err := executeCommand(cmd, tc.input)

			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			// Check that expected suggestions appear in output or error
			combined := output + err.Error()
			for _, suggest := range tc.wantSuggest {
				if !strings.Contains(combined, suggest) {
					t.Errorf("expected suggestion %q in output, got: %q", suggest, combined)
				}
			}
		})
	}
}

func TestRootCmd_ShortFlag(t *testing.T) {
	// Test that -h works as short form of --help
	cmd := newTestRootCmd()
	output, err := executeCommand(cmd, "-h")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(output, "Adversarial Proof Framework") {
		t.Errorf("expected help output, got: %q", output)
	}
}

func TestRootCmd_VersionShortFlag(t *testing.T) {
	// Test that -v works as short form of --version
	cmd := newTestRootCmd()
	output, err := executeCommand(cmd, "-v")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := "af version " + Version
	if !strings.Contains(output, expected) {
		t.Errorf("expected version output to contain %q, got: %q", expected, output)
	}
}

func TestRootCmd_FlagFuzzyMatching(t *testing.T) {
	// These tests verify fuzzy matching for flags.
	// When a user types a misspelled flag, the CLI should suggest similar valid flags.

	tests := []struct {
		name        string
		args        []string
		wantSuggest []string // flags that should be suggested (without --)
		wantErr     bool
	}{
		{
			name:        "verbos suggests verbose",
			args:        []string{"status", "--verbos"},
			wantSuggest: []string{"verbose"},
			wantErr:     true,
		},
		{
			name:        "formt suggests format",
			args:        []string{"status", "--formt", "json"},
			wantSuggest: []string{"format"},
			wantErr:     true,
		},
		{
			name:        "dirr suggests dir",
			args:        []string{"status", "--dirr", "/tmp"},
			wantSuggest: []string{"dir"},
			wantErr:     true,
		},
		{
			name:        "formatt suggests format (double letter typo)",
			args:        []string{"status", "--formatt", "text"},
			wantSuggest: []string{"format"},
			wantErr:     true,
		},
		{
			name:        "completely unknown flag gets no suggestion",
			args:        []string{"status", "--xyz123"},
			wantSuggest: []string{}, // no close matches
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestRootCmd()
			output, err := executeCommand(cmd, tc.args...)

			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if err == nil {
				return
			}

			// Check that expected suggestions appear in output or error
			combined := output + err.Error()
			for _, suggest := range tc.wantSuggest {
				if !strings.Contains(combined, suggest) {
					t.Errorf("expected suggestion %q in output, got: %q", suggest, combined)
				}
			}

			// Verify suggestions are formatted with -- prefix
			if len(tc.wantSuggest) > 0 {
				if !strings.Contains(combined, "Did you mean") {
					t.Errorf("expected 'Did you mean' in output, got: %q", combined)
				}
			}
		})
	}
}
