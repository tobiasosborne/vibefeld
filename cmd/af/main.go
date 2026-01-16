// Package main provides the entry point for the af CLI tool.
//
// AF (Adversarial Proof Framework) is a command-line tool for collaborative
// construction of natural-language mathematical proofs. Multiple AI agents
// work concurrently as adversarial provers and verifiers, refining proof
// steps until rigorous acceptance.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is the current version of the af CLI tool.
const Version = "0.1.0"

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
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
  - Self-documenting CLI for agent workflows

Global flags:
  --verbose       Enable verbose output for debugging
  --dry-run       Preview changes without making them`,
	Version: Version,
}

func init() {
	rootCmd.SetVersionTemplate("af version {{.Version}}\n")

	// Persistent flags are inherited by all subcommands
	// Note: -v is already used by Cobra for --version, so verbose has no shorthand
	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose output for debugging")
	rootCmd.PersistentFlags().Bool("dry-run", false, "Preview changes without making them")
}

// isVerbose returns true if verbose mode is enabled.
func isVerbose(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("verbose")
	return v
}

// isDryRun returns true if dry-run mode is enabled.
func isDryRun(cmd *cobra.Command) bool {
	d, _ := cmd.Flags().GetBool("dry-run")
	return d
}
