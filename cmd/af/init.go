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
	var template string
	var listTemplates bool

	cmd := &cobra.Command{
		Use:     "init",
		GroupID: GroupSetup,
		Short:   "Initialize a new proof workspace",
		Long: `Initialize a new proof workspace with a conjecture to prove.

This command creates the proof directory structure and initializes the
append-only ledger with the proof's conjecture and author information.

The conjecture is the mathematical statement to be proven through
adversarial collaboration between prover and verifier agents.

Use --template to start with a predefined proof structure:
  - contradiction: Proof by contradiction (assume negation, derive contradiction)
  - induction: Proof by induction (base case, inductive step)
  - cases: Proof by case analysis (multiple cases)

Use --list-templates to see all available templates.

Example:
  af init --conjecture "All primes greater than 2 are odd" --author "Claude"
  af init -c "P = NP" -a "Alice" -d ./my-proof
  af init -c "Sum of first n integers is n(n+1)/2" -a "Claude" --template induction

Workflow:
  After initialization, use 'af status' to view the proof tree, then 'af jobs'
  to see available work, and 'af claim' to start working on a node.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle --list-templates flag
			if listTemplates {
				return runListTemplates(cmd)
			}
			return runInit(cmd, conjecture, author, dir, template)
		},
	}

	cmd.Flags().StringVarP(&conjecture, "conjecture", "c", "", "The mathematical conjecture to prove (required unless --list-templates)")
	cmd.Flags().StringVarP(&author, "author", "a", "", "The author initiating the proof (required unless --list-templates)")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "The directory to initialize the proof in")
	cmd.Flags().StringVarP(&template, "template", "t", "", "Use a proof template (contradiction, induction, cases)")
	cmd.Flags().BoolVar(&listTemplates, "list-templates", false, "List available proof templates")

	return cmd
}

// runListTemplates displays all available proof templates.
func runListTemplates(cmd *cobra.Command) error {
	fmt.Fprintln(cmd.OutOrStdout(), "Available proof templates:")
	fmt.Fprintln(cmd.OutOrStdout())

	for _, tmpl := range service.ListTemplates() {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", tmpl.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", tmpl.Description)
		fmt.Fprintln(cmd.OutOrStdout())
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Usage:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af init --conjecture \"...\" --author \"...\" --template <name>")

	return nil
}

func runInit(cmd *cobra.Command, conjecture, author, dir, templateName string) error {
	// Validate conjecture is not empty or whitespace-only
	if strings.TrimSpace(conjecture) == "" {
		return fmt.Errorf("conjecture cannot be empty")
	}

	// Validate author is not empty or whitespace-only
	if strings.TrimSpace(author) == "" {
		return fmt.Errorf("author cannot be empty")
	}

	// Validate template if specified
	var tmpl service.Template
	hasTemplate := false
	if templateName != "" {
		var ok bool
		tmpl, ok = service.GetTemplate(templateName)
		if !ok {
			return fmt.Errorf("unknown template: %q (use --list-templates to see available templates)", templateName)
		}
		hasTemplate = true
	}

	// Call the service layer to initialize the proof
	if err := service.Init(dir, conjecture, author); err != nil {
		return err
	}

	// Apply template if specified
	if hasTemplate {
		if err := applyTemplate(dir, tmpl); err != nil {
			return fmt.Errorf("failed to apply template: %w", err)
		}
	}

	// Output success message
	fmt.Fprintf(cmd.OutOrStdout(), "Proof initialized successfully in %s\n", dir)
	fmt.Fprintf(cmd.OutOrStdout(), "Conjecture: %s\n", conjecture)
	fmt.Fprintf(cmd.OutOrStdout(), "Author: %s\n", author)

	if hasTemplate {
		fmt.Fprintf(cmd.OutOrStdout(), "Template: %s\n", tmpl.Name)
		fmt.Fprintln(cmd.OutOrStdout(), "\nProof structure created:")
		fmt.Fprintln(cmd.OutOrStdout(), "  1   - Root conjecture")
		for i, child := range tmpl.Children {
			fmt.Fprintf(cmd.OutOrStdout(), "  1.%d - %s\n", i+1, child.StatementTemplate)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  1. af status    - View the proof tree (root node '1' is your conjecture)")
	fmt.Fprintln(cmd.OutOrStdout(), "  2. af jobs      - See available work (root is now a verifier job)")
	fmt.Fprintln(cmd.OutOrStdout(), "  3. af claim 1   - Claim the root node to start working")
	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), "Workflow overview:")
	fmt.Fprintln(cmd.OutOrStdout(), "  - New nodes start as VERIFIER jobs (ready for review)")
	fmt.Fprintln(cmd.OutOrStdout(), "  - Verifiers either ACCEPT nodes or raise CHALLENGES")
	fmt.Fprintln(cmd.OutOrStdout(), "  - Challenged nodes become PROVER jobs (need refinement)")
	fmt.Fprintln(cmd.OutOrStdout(), "  - Provers use REFINE to break down claims into substeps")
	fmt.Fprintln(cmd.OutOrStdout(), "  - The cycle repeats until all nodes are accepted")
	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), "Quick reference:")
	fmt.Fprintln(cmd.OutOrStdout(), "  Verifier commands: af accept, af challenge")
	fmt.Fprintln(cmd.OutOrStdout(), "  Prover commands:   af refine, af amend, af resolve-challenge")
	fmt.Fprintln(cmd.OutOrStdout(), "  Info commands:     af get <id>, af schema, af inferences")

	return nil
}

// applyTemplate creates the child nodes defined by the template.
func applyTemplate(dir string, tmpl service.Template) error {
	svc, err := service.NewProofService(dir)
	if err != nil {
		return err
	}

	rootID, err := service.ParseNodeID("1")
	if err != nil {
		return err
	}

	for i, childSpec := range tmpl.Children {
		childID, err := rootID.Child(i + 1)
		if err != nil {
			return fmt.Errorf("failed to create child ID: %w", err)
		}

		if err := svc.CreateNode(childID, childSpec.NodeType, childSpec.StatementTemplate, childSpec.Inference); err != nil {
			return fmt.Errorf("failed to create node %s: %w", childID.String(), err)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newInitCmd())
}
