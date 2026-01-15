// Package main contains the af tutorial command for displaying workflow guidance.
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newTutorialCmd creates the tutorial command for displaying a step-by-step guide.
func newTutorialCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tutorial",
		Short: "Show a step-by-step guide for proving something",
		Long: `Display a comprehensive tutorial on how to prove something from start to finish
using the AF (Adversarial Proof Framework).

This tutorial walks through the complete proof workflow:
  - Initializing a new proof
  - Checking proof status
  - Claiming work as a prover or verifier
  - Refining proof steps
  - Accepting/validating steps
  - Handling challenges

This command works without an initialized proof directory because
it displays static tutorial content.

Example:
  af tutorial     Show the complete workflow tutorial`,
		RunE: runTutorial,
	}

	return cmd
}

// runTutorial executes the tutorial command.
func runTutorial(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	fmt.Fprint(out, tutorialContent)
	return nil
}

// tutorialContent contains the full tutorial text.
const tutorialContent = `
================================================================================
                    AF Tutorial: Proving Something Start to Finish
================================================================================

This tutorial walks you through the complete workflow for constructing a
mathematical proof using the AF (Adversarial Proof Framework).

AF uses adversarial verification: PROVERS construct proof steps while
VERIFIERS attack and challenge them. No agent plays both roles. This
separation ensures rigorous verification of every claim.

================================================================================
Step 1: Initialize a New Proof
================================================================================

Start by creating a new proof directory:

    af init "For all natural numbers n, n + 0 = n"

This creates:
  - A .af/ directory with the proof ledger
  - A root node (ID: 1) containing your main claim
  - The root node is marked as "pending" and "available"

Check what was created:

    af status

You'll see the node tree with the root claim waiting to be proved.

================================================================================
Step 2: Understanding Roles - Prover vs Verifier
================================================================================

AF has two agent roles:

PROVER ROLE:
  - Claims available nodes that need proof
  - Refines nodes by breaking them into sub-steps
  - Responds to challenges by providing more detail

VERIFIER ROLE:
  - Reviews pending nodes for correctness
  - Raises challenges when steps are unclear or incorrect
  - Accepts valid steps or refutes invalid ones

To see available work for each role:

    af jobs                    # Shows all available jobs
    af jobs --role prover      # Shows prover jobs only
    af jobs --role verifier    # Shows verifier jobs only

================================================================================
Step 3: Claim Work (Prover)
================================================================================

As a prover, claim a node to work on:

    af claim --role prover 1

This marks node 1 as "claimed" by you. While claimed, other agents cannot
modify it. The command outputs context about the node including:
  - The statement to prove
  - Parent context (if any)
  - Available definitions and lemmas

================================================================================
Step 4: Refine the Proof (Prover)
================================================================================

Break down the claim into smaller proof steps:

    af refine 1 --children '[
      {"statement": "Base case: 0 + 0 = 0", "node_type": "claim"},
      {"statement": "Inductive step: if n + 0 = n, then (n+1) + 0 = n+1", "node_type": "claim"}
    ]'

This creates child nodes 1.1 and 1.2 under node 1.

Node IDs form a hierarchy:
  - Root: 1
  - Children of 1: 1.1, 1.2, 1.3, ...
  - Children of 1.1: 1.1.1, 1.1.2, ...

After refining, release your claim:

    af release 1

================================================================================
Step 5: Review and Accept (Verifier)
================================================================================

As a verifier, claim a node to review:

    af claim --role verifier 1

Review the node and its children. If the breakdown is valid, accept it:

    af accept 1

This marks node 1 as "validated" - the breakdown is correct, though the
children still need to be proved.

If the node is a leaf (no children) and the reasoning is valid, acceptance
means the step is complete.

================================================================================
Step 6: Handling Challenges
================================================================================

If a verifier finds issues, they can raise a challenge:

    af challenge 1.1 --target gap --reason "Missing justification for why 0 + 0 = 0"

Challenge targets include:
  - statement: The claim itself is incorrect
  - inference: The reasoning is invalid
  - gap: Missing steps or justification
  - context: Dependencies are wrong
  - scope: Variable scope issues

The prover can then resolve the challenge:

    af resolve-challenge 1.1 --challenge-id <id> --response "By definition of addition, 0 + 0 = 0"

Or provide more detail by further refining the node.

================================================================================
Step 7: Complete the Proof
================================================================================

Continue the cycle:
  1. Provers claim and refine nodes
  2. Verifiers review and accept or challenge
  3. Provers respond to challenges
  4. Repeat until all nodes are validated

Check progress at any time:

    af status
    af progress    # Shows completion statistics

A proof is complete when the root node and all its descendants are validated.

================================================================================
Key Commands Reference
================================================================================

SETUP:
  af init "<claim>"          Initialize a new proof
  af status                  Show proof status and node tree
  af progress                Show completion progress

PROVER WORKFLOW:
  af claim --role prover <id>    Claim a node to work on
  af refine <id> --children ...  Break down into sub-steps
  af release <id>                Release your claim
  af resolve-challenge ...       Respond to a challenge

VERIFIER WORKFLOW:
  af claim --role verifier <id>  Claim a node to review
  af accept <id>                 Accept a valid step
  af challenge <id> ...          Raise a challenge
  af refute <id>                 Refute an invalid step

EXPLORATION:
  af get <id>                    Show details of a specific node
  af search "<query>"            Search for nodes
  af challenges                  List active challenges

================================================================================
Tips for Success
================================================================================

1. SMALL STEPS: Break proofs into small, verifiable steps. Each node should
   make exactly one logical move.

2. CLEAR STATEMENTS: Write precise statements. Vague claims are hard to verify.

3. CITE DEPENDENCIES: When a step depends on another, make it explicit using
   the inference type and dependencies.

4. RESPOND TO CHALLENGES: Don't ignore challenges - address them by refining
   further or providing justification.

5. USE DEFINITIONS: Define terms explicitly with 'af def-add' to avoid
   ambiguity.

================================================================================

Run 'af --help' for the full command list.
Run 'af <command> --help' for detailed help on any command.
Run 'af schema' to see all valid inference types, node types, and states.
`

func init() {
	rootCmd.AddCommand(newTutorialCmd())
}
