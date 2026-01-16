// Package main contains the af wizard command for guided workflows.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/templates"
)

// newWizardCmd creates the wizard command for guided workflows.
func newWizardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wizard",
		Short: "Guided workflow wizards",
		Long: `Interactive wizards that guide you through common AF workflows.

Available wizards:
  new-proof          Guide through initializing a new proof
  respond-challenge  Guide through responding to a challenge
  review             Guide a verifier through reviewing pending nodes

Each wizard explains what it does, asks for required inputs step by step,
shows a preview of what will happen, and asks for confirmation before executing.

Examples:
  af wizard new-proof                    Start the new proof wizard interactively
  af wizard new-proof --no-confirm       Run without confirmation prompts
  af wizard respond-challenge            Guide through responding to challenges
  af wizard review                       Guide verifier through pending nodes`,
		RunE: runWizard,
	}

	// Add subcommands
	cmd.AddCommand(newWizardNewProofCmd())
	cmd.AddCommand(newWizardRespondChallengeCmd())
	cmd.AddCommand(newWizardReviewCmd())

	return cmd
}

// runWizard is the default handler when no subcommand is specified.
func runWizard(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "AF Workflow Wizards")
	fmt.Fprintln(out, "==================")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Available wizards:")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  new-proof          Initialize a new proof workspace")
	fmt.Fprintln(out, "  respond-challenge  Respond to open challenges")
	fmt.Fprintln(out, "  review             Review pending nodes as a verifier")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  af wizard <wizard-name>")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run 'af wizard <wizard-name> --help' for detailed help on each wizard.")

	return nil
}

// =============================================================================
// New Proof Wizard
// =============================================================================

// newWizardNewProofCmd creates the new-proof wizard subcommand.
func newWizardNewProofCmd() *cobra.Command {
	var conjecture string
	var author string
	var dir string
	var template string
	var noConfirm bool
	var preview bool

	cmd := &cobra.Command{
		Use:   "new-proof",
		Short: "Guide through initializing a new proof",
		Long: `Interactive wizard that guides you through creating a new proof workspace.

This wizard helps you:
  1. Choose a directory for the proof
  2. Enter the mathematical conjecture to prove
  3. Specify the author name
  4. Optionally select a proof template (induction, contradiction, cases)
  5. Preview and confirm before creating

Non-interactive mode:
  Use --no-confirm to skip confirmation prompts (requires all flags to be set).
  Use --preview to see what would happen without executing.

Examples:
  af wizard new-proof                                    Interactive mode
  af wizard new-proof -c "P implies Q" -a "Claude" -d .  With flags
  af wizard new-proof --preview                          Preview only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWizardNewProof(cmd, conjecture, author, dir, template, noConfirm, preview)
		},
	}

	cmd.Flags().StringVarP(&conjecture, "conjecture", "c", "", "The mathematical conjecture to prove")
	cmd.Flags().StringVarP(&author, "author", "a", "", "The author initiating the proof")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "The directory to initialize the proof in")
	cmd.Flags().StringVarP(&template, "template", "t", "", "Proof template (contradiction, induction, cases)")
	cmd.Flags().BoolVar(&noConfirm, "no-confirm", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&preview, "preview", false, "Show preview only, don't execute")

	return cmd
}

// runWizardNewProof executes the new proof wizard.
func runWizardNewProof(cmd *cobra.Command, conjecture, author, dir, template string, noConfirm, preview bool) error {
	out := cmd.OutOrStdout()
	in := os.Stdin

	// Interactive mode if flags not provided
	interactive := !noConfirm && conjecture == "" && author == ""

	if interactive {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "================================================================================")
		fmt.Fprintln(out, "                       New Proof Wizard")
		fmt.Fprintln(out, "================================================================================")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "This wizard will guide you through creating a new proof workspace.")
		fmt.Fprintln(out, "")
	}

	// Get directory
	if interactive && dir == "." {
		fmt.Fprint(out, "Directory for the proof [.]: ")
		inputDir, err := readWizardLine(in)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		if inputDir != "" {
			dir = inputDir
		}
	}

	// Get conjecture
	if conjecture == "" {
		if interactive {
			fmt.Fprintln(out, "")
			fmt.Fprintln(out, "Enter the mathematical statement you want to prove.")
			fmt.Fprintln(out, "Example: \"All primes greater than 2 are odd\"")
			fmt.Fprintln(out, "")
			fmt.Fprint(out, "Conjecture: ")
			var err error
			conjecture, err = readWizardLine(in)
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
		}
		if err := validateWizardConjecture(conjecture); err != nil {
			return err
		}
	} else {
		if err := validateWizardConjecture(conjecture); err != nil {
			return err
		}
	}

	// Get author
	if author == "" {
		if interactive {
			fmt.Fprintln(out, "")
			fmt.Fprint(out, "Author name: ")
			var err error
			author, err = readWizardLine(in)
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
		}
		if err := validateWizardAuthor(author); err != nil {
			return err
		}
	} else {
		if err := validateWizardAuthor(author); err != nil {
			return err
		}
	}

	// Get template (optional)
	if template == "" && interactive {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Optional: Choose a proof template for common proof strategies.")
		fmt.Fprintln(out, "Available templates:")
		fmt.Fprintln(out, "  - contradiction: Proof by contradiction")
		fmt.Fprintln(out, "  - induction: Proof by induction (base case + inductive step)")
		fmt.Fprintln(out, "  - cases: Proof by case analysis")
		fmt.Fprintln(out, "")
		fmt.Fprint(out, "Template (leave empty for none): ")
		var err error
		template, err = readWizardLine(in)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
	}

	// Validate template if provided
	if template != "" {
		if err := validateWizardTemplate(template); err != nil {
			return err
		}
	}

	// Show preview
	previewText := renderNewProofPreview(conjecture, author, dir, template)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, previewText)

	// If preview only, stop here
	if preview {
		fmt.Fprintln(out, "[Preview mode - no changes made]")
		return nil
	}

	// Confirm
	if !noConfirm {
		fmt.Fprintln(out, "")
		fmt.Fprint(out, "Proceed with creating this proof? [y/N]: ")
		if !readWizardConfirm(in) {
			fmt.Fprintln(out, "Cancelled.")
			return nil
		}
	}

	// Execute: Initialize the proof
	if err := service.Init(dir, conjecture, author); err != nil {
		return fmt.Errorf("failed to initialize proof: %w", err)
	}

	// Apply template if specified
	if template != "" {
		tmpl, ok := templates.Get(template)
		if ok {
			if err := applyTemplate(dir, tmpl); err != nil {
				// Don't fail completely - proof is initialized
				fmt.Fprintf(out, "Warning: failed to apply template: %v\n", err)
			}
		}
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "================================================================================")
	fmt.Fprintln(out, "                       Proof Created Successfully!")
	fmt.Fprintln(out, "================================================================================")
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Conjecture: %s\n", conjecture)
	fmt.Fprintf(out, "Author:     %s\n", author)
	fmt.Fprintf(out, "Directory:  %s\n", dir)
	if template != "" {
		fmt.Fprintf(out, "Template:   %s\n", template)
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Next steps:")
	fmt.Fprintln(out, "  1. Run 'af status' to see the proof structure")
	fmt.Fprintln(out, "  2. Run 'af claim 1 --role prover -o <your-name>' to start working")
	fmt.Fprintln(out, "  3. Run 'af refine 1' to break down the proof into steps")
	fmt.Fprintln(out, "")

	return nil
}

// =============================================================================
// Respond Challenge Wizard
// =============================================================================

// newWizardRespondChallengeCmd creates the respond-challenge wizard subcommand.
func newWizardRespondChallengeCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "respond-challenge",
		Short: "Guide through responding to challenges",
		Long: `Interactive wizard that guides you through responding to open challenges.

This wizard helps you:
  1. List all open challenges in the proof
  2. Select a challenge to respond to
  3. Choose how to respond (resolve, refine further, or request clarification)
  4. Execute the response

A challenge is an objection raised by a verifier against a proof node.
Responding effectively is crucial for advancing the proof.

Examples:
  af wizard respond-challenge              List and respond to challenges
  af wizard respond-challenge --dir ./proof`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWizardRespondChallenge(cmd, dir)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory path")

	return cmd
}

// runWizardRespondChallenge executes the respond challenge wizard.
func runWizardRespondChallenge(cmd *cobra.Command, dir string) error {
	out := cmd.OutOrStdout()

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Check if proof is initialized
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("no proof initialized in %s. Run 'af wizard new-proof' first", dir)
	}

	// Load state to get challenges
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Get all open challenges
	challenges := st.AllChallenges()
	openChallenges := make([]*struct {
		ID       string
		NodeID   string
		Target   string
		Reason   string
		Severity string
	}, 0)

	for _, c := range challenges {
		if c.Status == "open" {
			openChallenges = append(openChallenges, &struct {
				ID       string
				NodeID   string
				Target   string
				Reason   string
				Severity string
			}{
				ID:       c.ID,
				NodeID:   c.NodeID.String(),
				Target:   c.Target,
				Reason:   c.Reason,
				Severity: c.Severity,
			})
		}
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "================================================================================")
	fmt.Fprintln(out, "                    Respond to Challenge Wizard")
	fmt.Fprintln(out, "================================================================================")
	fmt.Fprintln(out, "")

	if len(openChallenges) == 0 {
		fmt.Fprintln(out, "No open challenges found in this proof.")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "This is good news! Either:")
		fmt.Fprintln(out, "  - All challenges have been resolved")
		fmt.Fprintln(out, "  - No verifier has raised challenges yet")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Next steps:")
		fmt.Fprintln(out, "  - Run 'af status' to see the proof structure")
		fmt.Fprintln(out, "  - Run 'af challenges' to see all challenges (including resolved)")
		return nil
	}

	fmt.Fprintf(out, "Found %d open challenge(s):\n", len(openChallenges))
	fmt.Fprintln(out, "")

	for i, c := range openChallenges {
		fmt.Fprintf(out, "  [%d] Challenge %s\n", i+1, c.ID)
		fmt.Fprintf(out, "      Node:     %s\n", c.NodeID)
		fmt.Fprintf(out, "      Target:   %s\n", c.Target)
		fmt.Fprintf(out, "      Severity: %s\n", c.Severity)
		fmt.Fprintf(out, "      Reason:   %s\n", c.Reason)
		fmt.Fprintln(out)
	}

	fmt.Fprintln(out, "How to respond to a challenge:")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  Option 1: Resolve with explanation")
	fmt.Fprintln(out, "    af resolve-challenge <node-id> --challenge-id <id> --response \"...\"")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  Option 2: Refine the node further")
	fmt.Fprintln(out, "    af claim <node-id> --role prover -o <owner>")
	fmt.Fprintln(out, "    af refine <node-id> --children '...'")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  Option 3: Request the verifier to withdraw (if challenge is invalid)")
	fmt.Fprintln(out, "    The verifier can run: af withdraw-challenge <challenge-id>")
	fmt.Fprintln(out, "")

	return nil
}

// =============================================================================
// Review Wizard
// =============================================================================

// newWizardReviewCmd creates the review wizard subcommand.
func newWizardReviewCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Guide a verifier through reviewing pending nodes",
		Long: `Interactive wizard that guides a verifier through reviewing pending nodes.

This wizard helps verifiers:
  1. See all nodes awaiting review (pending epistemic state)
  2. Understand what to look for when reviewing
  3. Choose actions: accept, challenge, or refute

The verifier role is crucial for ensuring proof correctness. Every accepted
node has been verified by an adversarial reviewer.

Examples:
  af wizard review              Review pending nodes
  af wizard review --dir ./proof`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWizardReview(cmd, dir)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory path")

	return cmd
}

// runWizardReview executes the review wizard.
func runWizardReview(cmd *cobra.Command, dir string) error {
	out := cmd.OutOrStdout()

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Check if proof is initialized
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("no proof initialized in %s. Run 'af wizard new-proof' first", dir)
	}

	// Get pending nodes
	pendingNodes, err := svc.GetPendingNodes()
	if err != nil {
		return fmt.Errorf("error getting pending nodes: %w", err)
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "================================================================================")
	fmt.Fprintln(out, "                       Verifier Review Wizard")
	fmt.Fprintln(out, "================================================================================")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "As a verifier, your role is to review pending nodes and ensure correctness.")
	fmt.Fprintln(out, "")

	if len(pendingNodes) == 0 {
		fmt.Fprintln(out, "No pending nodes to review.")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "All nodes have been reviewed. The proof may be:")
		fmt.Fprintln(out, "  - Complete (all validated)")
		fmt.Fprintln(out, "  - Stuck on challenges (check 'af challenges')")
		fmt.Fprintln(out, "  - Awaiting prover work")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Next steps:")
		fmt.Fprintln(out, "  - Run 'af status' to see the full proof state")
		fmt.Fprintln(out, "  - Run 'af progress' to see completion statistics")
		return nil
	}

	fmt.Fprintf(out, "Found %d pending node(s) awaiting review:\n", len(pendingNodes))
	fmt.Fprintln(out, "")

	for i, n := range pendingNodes {
		fmt.Fprintf(out, "  [%d] Node %s\n", i+1, n.ID.String())
		fmt.Fprintf(out, "      Type:      %s\n", n.Type)
		// Truncate statement for display
		stmt := n.Statement
		if len(stmt) > 60 {
			stmt = stmt[:57] + "..."
		}
		fmt.Fprintf(out, "      Statement: %s\n", stmt)
		fmt.Fprintf(out, "      Inference: %s\n", n.Inference)
		fmt.Fprintln(out)
	}

	fmt.Fprintln(out, "--------------------------------------------------------------------------------")
	fmt.Fprintln(out, "                         How to Review a Node")
	fmt.Fprintln(out, "--------------------------------------------------------------------------------")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "1. Examine the node details:")
	fmt.Fprintln(out, "   af get <node-id>")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "2. Claim the node for review:")
	fmt.Fprintln(out, "   af claim <node-id> --role verifier -o <your-name>")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "3. Check for issues:")
	fmt.Fprintln(out, "   - Is the statement clear and correct?")
	fmt.Fprintln(out, "   - Is the inference valid?")
	fmt.Fprintln(out, "   - Are dependencies correctly cited?")
	fmt.Fprintln(out, "   - Are there any gaps in reasoning?")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "4. Take action:")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "   If VALID:    af accept <node-id>")
	fmt.Fprintln(out, "   If UNCLEAR:  af challenge <node-id> --reason \"...\" --target <aspect>")
	fmt.Fprintln(out, "   If WRONG:    af refute <node-id> --reason \"...\"")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Challenge targets: statement, inference, gap, context, dependencies, scope")
	fmt.Fprintln(out, "")

	return nil
}

// =============================================================================
// Validation Helpers
// =============================================================================

// validateWizardConjecture validates a conjecture input.
func validateWizardConjecture(conjecture string) error {
	if strings.TrimSpace(conjecture) == "" {
		return fmt.Errorf("conjecture cannot be empty")
	}
	return nil
}

// validateWizardAuthor validates an author input.
func validateWizardAuthor(author string) error {
	if strings.TrimSpace(author) == "" {
		return fmt.Errorf("author cannot be empty")
	}
	return nil
}

// validateWizardTemplate validates a template name.
func validateWizardTemplate(template string) error {
	if template == "" {
		return nil // Optional
	}
	if _, ok := templates.Get(template); !ok {
		return fmt.Errorf("unknown template %q (valid: contradiction, induction, cases)", template)
	}
	return nil
}

// =============================================================================
// Preview Helpers
// =============================================================================

// renderNewProofPreview creates a preview of what the new proof wizard will do.
func renderNewProofPreview(conjecture, author, dir, template string) string {
	var sb strings.Builder

	sb.WriteString("================================================================================\n")
	sb.WriteString("                           Preview\n")
	sb.WriteString("================================================================================\n")
	sb.WriteString("\n")
	sb.WriteString("The following proof will be created:\n")
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  Conjecture: %s\n", conjecture))
	sb.WriteString(fmt.Sprintf("  Author:     %s\n", author))
	sb.WriteString(fmt.Sprintf("  Directory:  %s\n", dir))

	if template != "" {
		sb.WriteString(fmt.Sprintf("  Template:   %s\n", template))
		sb.WriteString("\n")
		sb.WriteString("Template structure:\n")
		switch template {
		case "contradiction":
			sb.WriteString("  1   - Root conjecture\n")
			sb.WriteString("  1.1 - Assume negation\n")
			sb.WriteString("  1.2 - Derive contradiction\n")
		case "induction":
			sb.WriteString("  1   - Root conjecture\n")
			sb.WriteString("  1.1 - Base case\n")
			sb.WriteString("  1.2 - Inductive step\n")
		case "cases":
			sb.WriteString("  1   - Root conjecture\n")
			sb.WriteString("  1.1 - Case 1\n")
			sb.WriteString("  1.2 - Case 2\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString("This will create a 'ledger' directory with the proof events.\n")

	return sb.String()
}

// =============================================================================
// Input Helpers
// =============================================================================

// readWizardLine reads a line from the reader and trims whitespace.
func readWizardLine(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// readWizardConfirm reads a yes/no confirmation from the reader.
func readWizardConfirm(r io.Reader) bool {
	line, err := readWizardLine(r)
	if err != nil {
		return false
	}
	line = strings.ToLower(line)
	return line == "y" || line == "yes"
}

func init() {
	rootCmd.AddCommand(newWizardCmd())
}
