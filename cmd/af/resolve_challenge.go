// Package main contains the af resolve-challenge command implementation.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/ledger"
)

// newResolveChallengeCmd creates the resolve-challenge command.
func newResolveChallengeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "resolve-challenge <challenge-id>",
		GroupID: GroupProver,
		Short:   "Resolve a challenge with a response",
		Long: `Resolve a previously raised challenge by providing a response.

This is a prover action that addresses a verifier's objection.
The challenge must be in an open state (not already resolved or withdrawn).

WHAT MAKES A GOOD RESOLUTION:

A good response directly addresses the verifier's concern. Match your response
to the challenge target:

  statement     → Clarify or correct the claim text
                  "Amended node statement to say 'x >= 0' instead of 'x > 0'"

  inference     → Justify the reasoning step or add missing justification
                  "Added explicit modus ponens: from P and P→Q, we derive Q"

  context       → Fix or clarify the definition/reference usage
                  "Corrected to use the topological definition of continuity"

  dependencies  → Add/fix the dependency relationship
                  "Added missing dependency on node 1.3 via 'af refine'"

  scope         → Fix scope violation or explain why it's valid
                  "Variable x is still in scope; introduced in 1.1, used in 1.1.2"

  gap           → Add intermediate steps via 'af refine'
                  "Added node 1.2.1 to bridge from A to B, and 1.2.2 from B to C"

  type_error    → Fix type mismatch
                  "Changed argument from set S to element s ∈ S"

  domain        → Address domain restriction
                  "Added precondition that x > 0 before applying sqrt"

TIPS:
  - Reference specific changes: "See amended statement in node 1.2"
  - Explain why the fix addresses the concern, not just what changed
  - If you refined the node, mention the new child nodes by ID
  - Keep responses concise but complete

Examples:
  af resolve-challenge chal-001 --response "Amended 1.2 to clarify x >= 0"
  af resolve-challenge ch-abc -r "Added node 1.3.1 to fill the logical gap"
  af resolve-challenge ch-def -r "Fixed: now depends on 1.3 instead of 1.2"

Workflow:
  After resolving a challenge, use 'af challenges' to check remaining issues.
  Once all blocking challenges are resolved, the node can be accepted with 'af accept'.`,
		Args: cobra.ExactArgs(1),
		RunE: runResolveChallenge,
	}

	// Add flags
	cmd.Flags().StringP("response", "r", "", "Response text for resolving the challenge")
	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

func runResolveChallenge(cmd *cobra.Command, args []string) error {
	// Get and validate challenge ID
	challengeID := args[0]
	if strings.TrimSpace(challengeID) == "" {
		return errors.New("challenge ID cannot be empty")
	}

	// Get flags
	response := cli.MustString(cmd, "response")
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	// Validate response is provided and not empty/whitespace
	if strings.TrimSpace(response) == "" {
		return errors.New("response is required and cannot be empty")
	}

	// Validate directory exists and is a directory
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("proof directory does not exist")
		}
		return fmt.Errorf("error accessing proof directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("path is not a directory")
	}

	// Get ledger
	ledgerDir := filepath.Join(dir, "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		return fmt.Errorf("error accessing ledger: %w", err)
	}

	// Check if proof is initialized
	count, err := ldg.Count()
	if err != nil {
		return fmt.Errorf("error reading ledger: %w", err)
	}
	if count == 0 {
		return errors.New("proof not initialized")
	}

	// Scan ledger to find challenge state
	state := &resolveChallengeState{
		id:     challengeID,
		exists: false,
		status: "",
	}

	err = ldg.Scan(func(seq int, data []byte) error {
		// Parse base event to get type
		var base struct {
			Type        string `json:"type"`
			ChallengeID string `json:"challenge_id"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			return nil // Skip unparseable events
		}

		// Track challenge state changes
		switch base.Type {
		case string(ledger.EventChallengeRaised):
			if base.ChallengeID == challengeID {
				state.exists = true
				state.status = "open"
			}
		case string(ledger.EventChallengeResolved):
			if base.ChallengeID == challengeID {
				state.status = "resolved"
			}
		case string(ledger.EventChallengeWithdrawn):
			if base.ChallengeID == challengeID {
				state.status = "withdrawn"
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error scanning ledger: %w", err)
	}

	// Validate challenge state
	if !state.exists {
		return fmt.Errorf("challenge %q does not exist", challengeID)
	}

	if state.status == "resolved" {
		return fmt.Errorf("challenge %q is not open (already resolved)", challengeID)
	}

	if state.status == "withdrawn" {
		return fmt.Errorf("challenge %q is not open (already withdrawn)", challengeID)
	}

	// Append ChallengeResolved event
	event := ledger.NewChallengeResolved(challengeID)
	_, err = ldg.Append(event)
	if err != nil {
		return fmt.Errorf("error resolving challenge: %w", err)
	}

	// Output result based on format
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"challenge_id": challengeID,
			"status":       "resolved",
			"resolved":     true,
			"response":     response,
		}
		output, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		// Text format
		fmt.Fprintf(cmd.OutOrStdout(), "Challenge %s resolved successfully.\n", challengeID)
	}

	return nil
}

// resolveChallengeState tracks the state of a challenge as we replay events.
type resolveChallengeState struct {
	id     string
	exists bool
	status string // "open", "resolved", "withdrawn"
}

func init() {
	rootCmd.AddCommand(newResolveChallengeCmd())
}
