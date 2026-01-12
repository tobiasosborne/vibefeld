package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fuzzy"
)

// AddFuzzyMatching configures a cobra command to suggest similar commands
// when an unknown command is entered. This provides a more helpful CLI
// experience for users who mistype commands.
func AddFuzzyMatching(cmd *cobra.Command) {
	// Store the original RunE if any
	originalRunE := cmd.RunE

	// Override RunE to handle the case when args are provided but no subcommand matches
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if originalRunE != nil {
			return originalRunE(c, args)
		}
		// If we get here with args, it means an unknown command was entered
		if len(args) > 0 {
			return unknownCommandError(c, args[0])
		}
		// No args: show help
		return c.Help()
	}
}

// unknownCommandError creates an error with fuzzy suggestions for an unknown command.
func unknownCommandError(cmd *cobra.Command, unknown string) error {
	// Collect all subcommand names
	candidates := make([]string, 0)
	for _, sub := range cmd.Commands() {
		// Skip hidden and help commands
		if !sub.Hidden && sub.Name() != "help" && sub.Name() != "completion" {
			candidates = append(candidates, sub.Name())
		}
	}

	// Get fuzzy match suggestions
	result := fuzzy.SuggestCommand(unknown, candidates)

	// Build error message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("unknown command %q for %q", unknown, cmd.Name()))

	if len(result.Suggestions) > 0 {
		msg.WriteString("\n\nDid you mean")
		if len(result.Suggestions) == 1 {
			msg.WriteString(fmt.Sprintf(": %s", result.Suggestions[0]))
		} else {
			msg.WriteString(" one of these?")
			for _, s := range result.Suggestions {
				msg.WriteString(fmt.Sprintf("\n  %s", s))
			}
		}
	}

	return fmt.Errorf("%s", msg.String())
}

func init() {
	AddFuzzyMatching(rootCmd)
}
