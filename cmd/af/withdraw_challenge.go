package main

import (
	"github.com/spf13/cobra"
)

// newWithdrawChallengeCmd creates the withdraw-challenge command.
// TODO: Implement full withdraw-challenge functionality
func newWithdrawChallengeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "withdraw-challenge CHALLENGE_ID",
		Short: "Withdraw a challenge",
		Long: `Withdraw a previously raised challenge.

The challenge must be in an open state (not already resolved or withdrawn).
Withdrawing a challenge is typically done by the verifier who originally
raised it when they determine the challenge is no longer valid or relevant.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement withdraw-challenge logic
			return nil
		},
	}

	// Add flags
	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}
