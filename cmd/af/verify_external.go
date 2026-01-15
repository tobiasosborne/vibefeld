// Package main contains the af verify-external command stub.
// This is a stub to allow integration tests to compile.
// TODO: Implement the actual verify-external command.
package main

import (
	"github.com/spf13/cobra"
)

// newVerifyExternalCmd creates a stub for the verify-external command.
// This allows integration tests to compile while the command is not yet implemented.
func newVerifyExternalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify-external <external-id>",
		Short: "Verify an external reference (stub)",
		Long:  "This command is not yet implemented.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")

	return cmd
}
