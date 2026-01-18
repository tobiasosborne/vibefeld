// Package main contains the af challenge command implementation.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

// newChallengeCmd creates the challenge command.
func newChallengeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "challenge <node-id>",
		GroupID: GroupVerifier,
		Short:   "Raise a challenge against a proof node",
		Long: `Raise a challenge (objection) against a proof node.

This is a verifier action that identifies an issue with a node's
statement, inference, context, dependencies, scope, or other aspect.
The prover must address the challenge before the node can be validated.

CHALLENGE TARGETS - WHICH TO USE WHEN:

  statement     - The claim text itself is wrong or unclear
                  Example: "The claim says x > 0 but should be x >= 0"

  inference     - The reasoning/logic step is invalid or unjustified
                  Example: "This modus ponens is invalid; P→Q and R given, not P"

  context       - Referenced definitions are wrong, missing, or misapplied
                  Example: "The definition of 'continuous' is used incorrectly here"

  dependencies  - The node depends on wrong or missing parent nodes
                  Example: "This step assumes 1.2 but should depend on 1.3"

  scope         - Local assumption issues (used outside valid scope)
                  Example: "Variable x was introduced in 1.1 and is out of scope here"

  gap           - Logical gap in reasoning (missing intermediate steps)
                  Example: "How do we get from A to C? Step B is missing"

  type_error    - Mathematical object types don't match
                  Example: "Applying a function to a set when it expects an element"

  domain        - Domain restriction violated (division by zero, etc.)
                  Example: "This uses sqrt(-1) but we're working in reals"

  completeness  - Missing cases or incomplete argument
                  Example: "Proof by cases only covers n=0 and n>0, missing n<0"

SEVERITY LEVELS AND ACCEPTANCE BLOCKING:

  Blocking severities (node CANNOT be accepted until resolved):
    critical - Fundamental error that invalidates the proof step
    major    - Significant issue that must be addressed

  Non-blocking severities (node CAN be accepted without resolution):
    minor    - Minor issue or improvement suggestion
    note     - Clarification request or informational comment

  Choose "critical" or "major" when the issue MUST be fixed before
  the proof can proceed. Choose "minor" or "note" for suggestions
  or clarifications that don't affect correctness.

Examples:
  af challenge 1 --reason "The inference is invalid"
  af challenge 1.2 --reason "Missing case" --target completeness
  af challenge 1.1 --reason "sqrt undefined for negative" --target domain
  af challenge 1 --severity critical --reason "Conclusion contradicts premise"
  af challenge 1 --severity note --reason "Consider clarifying this step"
  af challenge 1 -r "Statement is unclear" -t statement -d ./proof

Workflow:
  After raising a challenge, the prover must address it. Use 'af challenges'
  to monitor status. Use 'af resolve-challenge' to mark it resolved or
  'af withdraw-challenge' if no longer relevant.

Common mistakes:
  - Using --target statement for logic errors (use --target inference)
  - Using --target inference when the claim text is wrong (use --target statement)
  - Using --severity critical for minor clarifications (use --severity note)
  - Missing --target for domain errors like division by zero (use --target domain)`,
		Args: cobra.ExactArgs(1),
		RunE: runChallenge,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().StringP("target", "t", "statement", "Challenge target aspect")
	cmd.Flags().StringP("reason", "r", "", "Reason for the challenge (required)")
	cmd.Flags().StringP("severity", "s", "major", "Challenge severity (critical, major, minor, note)")

	return cmd
}

// runChallenge executes the challenge command.
func runChallenge(cmd *cobra.Command, args []string) error {
	examples := render.GetExamples("af challenge")

	// Parse node ID from positional argument
	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return render.InvalidNodeIDError("af challenge", nodeIDStr, examples)
	}

	// Get flags
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	target, err := cmd.Flags().GetString("target")
	if err != nil {
		return err
	}
	reason, err := cmd.Flags().GetString("reason")
	if err != nil {
		return err
	}
	severity, err := cmd.Flags().GetString("severity")
	if err != nil {
		return err
	}

	// Validate reason is provided and not empty/whitespace
	if strings.TrimSpace(reason) == "" {
		return render.MissingFlagError("af challenge", "reason", examples)
	}

	// Validate target if provided and non-empty
	if target != "" {
		if err := service.ValidateChallengeTarget(target); err != nil {
			return render.InvalidValueError("af challenge", "target", target, render.ValidChallengeTargets, examples)
		}
	}

	// Validate severity
	if err := service.ValidateChallengeSeverity(severity); err != nil {
		return render.InvalidValueError("af challenge", "severity", severity, []string{"critical", "major", "minor", "note"}, examples)
	}

	// Create proof service to check state
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Load state to check if node exists
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Check if node exists
	n := st.GetNode(nodeID)
	if n == nil {
		return fmt.Errorf("node %s does not exist", nodeID.String())
	}

	// Generate a unique challenge ID
	challengeID := generateChallengeID()

	// Create ledger from proof path
	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		return fmt.Errorf("error accessing ledger: %w", err)
	}

	// Get agent ID from environment variable (if set)
	agentID := os.Getenv("AF_AGENT_ID")

	// Append challenge raised event with severity and agent ID
	event := ledger.NewChallengeRaisedWithSeverity(challengeID, nodeID, target, reason, severity, agentID)
	_, err = ldg.Append(event)
	if err != nil {
		return fmt.Errorf("error raising challenge: %w", err)
	}

	// Output result based on format
	switch strings.ToLower(format) {
	case "json":
		return outputChallengeJSON(cmd, nodeID, challengeID, target, reason, severity)
	default:
		return outputChallengeText(cmd, nodeID, challengeID, target, reason, severity)
	}
}

// outputChallengeJSON outputs the challenge result in JSON format.
func outputChallengeJSON(cmd *cobra.Command, nodeID service.NodeID, challengeID, target, reason, severity string) error {
	result := map[string]interface{}{
		"node_id":      nodeID.String(),
		"challenge_id": challengeID,
		"target":       target,
		"reason":       reason,
		"severity":     severity,
		"status":       "raised",
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputChallengeText outputs the challenge result in human-readable text format.
func outputChallengeText(cmd *cobra.Command, nodeID service.NodeID, challengeID, target, reason, severity string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Challenge raised against node %s\n", nodeID.String())
	fmt.Fprintf(cmd.OutOrStdout(), "  Challenge ID: %s\n", challengeID)
	fmt.Fprintf(cmd.OutOrStdout(), "  Target:       %s\n", target)
	fmt.Fprintf(cmd.OutOrStdout(), "  Severity:     %s\n", severity)
	fmt.Fprintf(cmd.OutOrStdout(), "  Reason:       %s\n", reason)

	// Add note about whether this blocks acceptance
	if service.SeverityBlocksAcceptance(service.ChallengeSeverity(severity)) {
		fmt.Fprintln(cmd.OutOrStdout(), "  [This challenge blocks acceptance]")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "  [This challenge does NOT block acceptance]")
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af resolve-challenge  - Resolve this challenge with an explanation")
	fmt.Fprintln(cmd.OutOrStdout(), "  af withdraw-challenge - Withdraw this challenge if no longer relevant")

	return nil
}

// generateChallengeID generates a unique identifier for a challenge.
// Uses random bytes for uniqueness.
func generateChallengeID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// If crypto/rand fails, this indicates a critical system issue
		// Use timestamp-based fallback
		return fmt.Sprintf("ch-%v", service.Now())
	}
	return "ch-" + hex.EncodeToString(b)
}

func init() {
	rootCmd.AddCommand(newChallengeCmd())
}
