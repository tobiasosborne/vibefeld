package main

import (
	"github.com/spf13/cobra"
)

// newJobsCmd creates the jobs command.
// TODO: Implement full jobs functionality
func newJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "List available jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement jobs logic
			return nil
		},
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}
