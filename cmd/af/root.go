package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tobias/vibefeld/internal/fuzzy"
)

// unknownFlagPattern matches "unknown flag: --flagname" or "unknown shorthand flag: 'x' in -xyz"
var unknownFlagPattern = regexp.MustCompile(`unknown (?:shorthand )?flag: (?:'([^']+)' in )?-+(\w+)?`)

// AddFuzzyMatching configures a cobra command to suggest similar commands
// when an unknown command is entered, and similar flags when an unknown
// flag is used. This provides a more helpful CLI experience for users
// who mistype commands or flags.
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

	// Set up flag error handler to provide fuzzy suggestions for unknown flags
	cmd.SetFlagErrorFunc(flagErrorWithSuggestions)
}

// flagErrorWithSuggestions wraps flag errors to add fuzzy suggestions for unknown flags.
func flagErrorWithSuggestions(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check if this is an unknown flag error
	matches := unknownFlagPattern.FindStringSubmatch(errStr)
	if matches == nil {
		// Not an unknown flag error, return as-is
		return err
	}

	// Extract the unknown flag name
	unknownFlag := matches[2]
	if unknownFlag == "" && matches[1] != "" {
		// Shorthand flag case: 'v' in -verbo -> extract "verbo"
		unknownFlag = matches[1]
	}
	if unknownFlag == "" {
		return err
	}

	// Collect all available flags
	candidates := collectFlags(cmd)
	if len(candidates) == 0 {
		return err
	}

	// Get fuzzy match suggestions
	result := fuzzy.SuggestFlag(unknownFlag, candidates)

	// Build enhanced error message
	var msg strings.Builder
	msg.WriteString(errStr)

	if len(result.Suggestions) > 0 {
		msg.WriteString("\n\nDid you mean")
		if len(result.Suggestions) == 1 {
			msg.WriteString(fmt.Sprintf(": --%s", result.Suggestions[0]))
		} else {
			msg.WriteString(" one of these?")
			for _, s := range result.Suggestions {
				msg.WriteString(fmt.Sprintf("\n  --%s", s))
			}
		}
	}

	return fmt.Errorf("%s", msg.String())
}

// collectFlags gathers all flag names from a command and its parent.
func collectFlags(cmd *cobra.Command) []string {
	flags := make(map[string]bool)

	// Collect local flags
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			flags[f.Name] = true
		}
	})

	// Collect inherited/persistent flags
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			flags[f.Name] = true
		}
	})

	// Convert to slice
	result := make([]string, 0, len(flags))
	for name := range flags {
		result = append(result, name)
	}
	return result
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
	// Add fuzzy matching to root and all subcommands
	AddFuzzyMatchingRecursive(rootCmd)
}

// AddFuzzyMatchingRecursive adds fuzzy matching to a command and all its subcommands.
func AddFuzzyMatchingRecursive(cmd *cobra.Command) {
	AddFuzzyMatching(cmd)
	for _, sub := range cmd.Commands() {
		AddFuzzyMatchingRecursive(sub)
	}
}
