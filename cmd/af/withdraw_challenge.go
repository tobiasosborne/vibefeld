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

// challengeState tracks the state of a challenge as we replay events.
type challengeState struct {
	id     string
	exists bool
	status string // "open", "resolved", "withdrawn"
}

// newWithdrawChallengeCmd creates the withdraw-challenge command.
func newWithdrawChallengeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "withdraw-challenge CHALLENGE_ID",
		GroupID: GroupVerifier,
		Short:   "Withdraw a challenge",
		Long: `Withdraw a previously raised challenge.

The challenge must be in an open state (not already resolved or withdrawn).
Withdrawing a challenge is typically done by the verifier who originally
raised it when they determine the challenge is no longer valid or relevant.

Examples:
  af withdraw-challenge chal-001            Withdraw challenge chal-001
  af withdraw-challenge chal-abc123 -d .    Withdraw challenge in current directory
  af withdraw-challenge chal-xyz -f json    Withdraw and output result as JSON

Workflow:
  After withdrawing a challenge, use 'af challenges' to check remaining issues.
  Once all blocking challenges are resolved, the node can be accepted with 'af accept'.`,
		Args: cobra.ExactArgs(1),
		RunE: runWithdrawChallenge,
	}

	// Add flags
	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

func runWithdrawChallenge(cmd *cobra.Command, args []string) error {
	// Get and validate challenge ID
	challengeID := args[0]
	if strings.TrimSpace(challengeID) == "" {
		return errors.New("challenge ID cannot be empty")
	}

	// Get flags
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

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
	state := &challengeState{
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

	// Append ChallengeWithdrawn event
	event := ledger.NewChallengeWithdrawn(challengeID)
	_, err = ldg.Append(event)
	if err != nil {
		return fmt.Errorf("error withdrawing challenge: %w", err)
	}

	// Output result based on format
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"challenge_id": challengeID,
			"status":       "withdrawn",
			"withdrawn":    true,
		}
		output, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		// Text format
		fmt.Fprintf(cmd.OutOrStdout(), "Challenge %s withdrawn successfully.\n", challengeID)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newWithdrawChallengeCmd())
}
