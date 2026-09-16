package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/cli"
	"github.com/tobiasosborne/vibefeld/internal/service"
)

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

	// Access the proof service (validates the directory and loads state).
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Check if proof is initialized
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error reading ledger: %w", err)
	}
	if !status.Initialized {
		return errors.New("proof not initialized")
	}

	// Withdraw through the service's one-read commit primitive.
	if err := svc.WithdrawChallenge(challengeID); err != nil {
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
