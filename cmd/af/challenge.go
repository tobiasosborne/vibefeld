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
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newChallengeCmd creates the challenge command.
func newChallengeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "challenge <node-id>",
		Short: "Raise a challenge against a proof node",
		Long: `Raise a challenge (objection) against a proof node.

This is a verifier action that identifies an issue with a node's
statement, inference, context, dependencies, scope, or other aspect.
The prover must address the challenge before the node can be validated.

Severity levels:
  critical - Fundamental error that must be fixed (blocks acceptance)
  major    - Significant issue that should be addressed (blocks acceptance)
  minor    - Minor issue that could be improved (does NOT block acceptance)
  note     - Clarification request or suggestion (does NOT block acceptance)

Valid targets: statement, inference, context, dependencies, scope,
               gap, type_error, domain, completeness

Examples:
  af challenge 1 --reason "The inference is invalid"
  af challenge 1.2 --reason "Missing case" --target completeness
  af challenge 1 --severity critical --reason "This is fundamentally wrong"
  af challenge 1 --severity note --reason "Consider clarifying this step"
  af challenge 1 -r "Statement is unclear" -t statement -d ./proof`,
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
	nodeID, err := types.Parse(nodeIDStr)
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
		if err := schema.ValidateChallengeTarget(target); err != nil {
			return render.InvalidValueError("af challenge", "target", target, render.ValidChallengeTargets, examples)
		}
	}

	// Validate severity
	if err := schema.ValidateChallengeSeverity(severity); err != nil {
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
func outputChallengeJSON(cmd *cobra.Command, nodeID types.NodeID, challengeID, target, reason, severity string) error {
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
func outputChallengeText(cmd *cobra.Command, nodeID types.NodeID, challengeID, target, reason, severity string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Challenge raised against node %s\n", nodeID.String())
	fmt.Fprintf(cmd.OutOrStdout(), "  Challenge ID: %s\n", challengeID)
	fmt.Fprintf(cmd.OutOrStdout(), "  Target:       %s\n", target)
	fmt.Fprintf(cmd.OutOrStdout(), "  Severity:     %s\n", severity)
	fmt.Fprintf(cmd.OutOrStdout(), "  Reason:       %s\n", reason)

	// Add note about whether this blocks acceptance
	if schema.SeverityBlocksAcceptance(schema.ChallengeSeverity(severity)) {
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
		return fmt.Sprintf("ch-%v", types.Now())
	}
	return "ch-" + hex.EncodeToString(b)
}

func init() {
	rootCmd.AddCommand(newChallengeCmd())
}
