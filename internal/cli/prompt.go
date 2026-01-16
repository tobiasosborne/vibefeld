// Package cli provides CLI utilities including argument parsing and prompts.
package cli

import (
	"fmt"
	"strings"
)

// ArgSpec describes a required argument for a command.
type ArgSpec struct {
	Name        string   // e.g., "node-id"
	Description string   // e.g., "The ID of the node to claim"
	Examples    []string // e.g., ["1", "1.2", "1.2.3"]
	Required    bool     // Whether the argument is required
}

// MissingArgError represents a missing required argument.
type MissingArgError struct {
	Command string  // The command that is missing an argument
	Arg     ArgSpec // The specification of the missing argument
}

// Error implements the error interface.
func (e *MissingArgError) Error() string {
	return fmt.Sprintf("missing required argument: %s", e.Arg.Name)
}

// HelpText returns formatted help text with examples.
//
// Output format:
//
//	Missing required argument: node-id
//	  The ID of the node to claim
//
//	Examples:
//	  af claim 1
//	  af claim 1.2
//	  af claim 1.2.3
func (e *MissingArgError) HelpText() string {
	var sb strings.Builder

	// Header line
	sb.WriteString(fmt.Sprintf("Missing required argument: %s\n", e.Arg.Name))

	// Description (indented)
	sb.WriteString(fmt.Sprintf("  %s\n", e.Arg.Description))

	// Examples section (only if examples exist)
	if len(e.Arg.Examples) > 0 {
		sb.WriteString("\nExamples:\n")
		for _, example := range e.Arg.Examples {
			sb.WriteString(fmt.Sprintf("  af %s %s\n", e.Command, example))
		}
	}

	return sb.String()
}

// CheckRequiredArgs checks if all required args are present.
// Returns nil if all required arguments are satisfied.
// Returns a MissingArgError for the first missing required argument.
// Note: The Command field in the returned error will be empty;
// the caller should set it if needed.
func CheckRequiredArgs(args []string, specs []ArgSpec) *MissingArgError {
	argCount := len(args)

	for i, spec := range specs {
		if spec.Required && i >= argCount {
			return &MissingArgError{
				Command: "",
				Arg:     spec,
			}
		}
	}

	return nil
}

// FormatArgHelp formats help text for an argument specification.
// This is useful for generating help documentation.
func FormatArgHelp(spec ArgSpec) string {
	var sb strings.Builder

	// Argument name
	sb.WriteString(fmt.Sprintf("%s\n", spec.Name))

	// Description (indented)
	sb.WriteString(fmt.Sprintf("  %s\n", spec.Description))

	// Examples section (only if examples exist)
	if len(spec.Examples) > 0 {
		sb.WriteString("\nExamples:\n")
		for _, example := range spec.Examples {
			sb.WriteString(fmt.Sprintf("  %s\n", example))
		}
	}

	return sb.String()
}
