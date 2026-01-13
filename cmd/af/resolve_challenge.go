package main

import (
	"github.com/spf13/cobra"
)

// newResolveChallengeCmd creates the resolve-challenge command.
// TODO: Implement full resolve-challenge functionality
func newResolveChallengeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve-challenge CHALLENGE_ID",
		Short: "Resolve a challenge",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement resolve-challenge logic
			return nil
		},
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}
