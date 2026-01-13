package main

import (
	"github.com/spf13/cobra"
)

// newChallengeCmd creates the challenge command.
// TODO: Implement full challenge functionality
func newChallengeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "challenge NODE_ID",
		Short: "Raise a challenge against a proof node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement challenge logic
			return nil
		},
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().StringP("target", "t", "statement", "Challenge target aspect")
	cmd.Flags().StringP("reason", "r", "", "Reason for the challenge")

	return cmd
}
