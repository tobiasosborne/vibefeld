package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// newInitCmd creates the init command for initializing a new proof workspace.
func newInitCmd() *cobra.Command {
	var conjecture string
	var author string
	var dir string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new proof workspace",
		Long: `Initialize a new proof workspace with a conjecture to prove.

This command creates the proof directory structure and initializes the
append-only ledger with the proof's conjecture and author information.

The conjecture is the mathematical statement to be proven through
adversarial collaboration between prover and verifier agents.

Example:
  af init --conjecture "All primes greater than 2 are odd" --author "Claude"
  af init -c "P = NP" -a "Alice" -d ./my-proof`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, conjecture, author, dir)
		},
	}

	cmd.Flags().StringVarP(&conjecture, "conjecture", "c", "", "The mathematical conjecture to prove (required)")
	cmd.Flags().StringVarP(&author, "author", "a", "", "The author initiating the proof (required)")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "The directory to initialize the proof in")

	cmd.MarkFlagRequired("conjecture")
	cmd.MarkFlagRequired("author")

	return cmd
}

func runInit(cmd *cobra.Command, conjecture, author, dir string) error {
	// Validate conjecture is not empty or whitespace-only
	if strings.TrimSpace(conjecture) == "" {
		return fmt.Errorf("conjecture cannot be empty")
	}

	// Validate author is not empty or whitespace-only
	if strings.TrimSpace(author) == "" {
		return fmt.Errorf("author cannot be empty")
	}

	// Call the service layer to initialize the proof
	if err := service.Init(dir, conjecture, author); err != nil {
		return err
	}

	// Output success message
	cmd.Printf("Proof initialized successfully in %s\n", dir)
	cmd.Printf("Conjecture: %s\n", conjecture)
	cmd.Printf("Author: %s\n", author)
	cmd.Println("\nNext steps:")
	cmd.Println("  af status    - View proof status")
	cmd.Println("  af claim     - Claim a job to work on")

	return nil
}

func init() {
	rootCmd.AddCommand(newInitCmd())
}
