package main

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// resolveAgent returns the acting agent identity for a command that records
// provenance: the --agent flag when set, otherwise the AF_AGENT_ID environment
// variable. This is the same convention `af challenge` and `af accept` use.
// It returns "" when neither is set.
func resolveAgent(cmd *cobra.Command) string {
	if cmd.Flags().Lookup("agent") != nil {
		if v, err := cmd.Flags().GetString("agent"); err == nil {
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				return trimmed
			}
		}
	}
	return strings.TrimSpace(os.Getenv("AF_AGENT_ID"))
}
