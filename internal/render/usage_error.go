package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tobias/vibefeld/internal/fuzzy"
)

// UsageError represents a CLI usage error with helpful suggestions.
// It provides structured information to help users correct their input.
type UsageError struct {
	Command      string   // The command that was invoked (e.g., "af claim")
	Message      string   // Error message describing what went wrong
	Examples     []string // Example correct usage
	Suggestions  []string // Fuzzy match suggestions for typos
	InvalidParam string   // Name of the invalid parameter (if applicable)
	ValidValues  []string // List of valid values for the parameter
}

// NewUsageError creates a new UsageError with the given command, message, and examples.
func NewUsageError(command, message string, examples []string) *UsageError {
	if examples == nil {
		examples = []string{}
	}
	return &UsageError{
		Command:     command,
		Message:     message,
		Examples:    examples,
		Suggestions: []string{},
		ValidValues: []string{},
	}
}

// WithSuggestions adds fuzzy match suggestions to the error.
// Returns a new UsageError with the suggestions set.
func (e *UsageError) WithSuggestions(suggestions []string) *UsageError {
	if suggestions == nil {
		suggestions = []string{}
	}
	return &UsageError{
		Command:      e.Command,
		Message:      e.Message,
		Examples:     e.Examples,
		Suggestions:  suggestions,
		InvalidParam: e.InvalidParam,
		ValidValues:  e.ValidValues,
	}
}

// WithValidValues adds valid values for a parameter to the error.
// Returns a new UsageError with the valid values set.
func (e *UsageError) WithValidValues(param string, values []string) *UsageError {
	if values == nil {
		values = []string{}
	}
	return &UsageError{
		Command:      e.Command,
		Message:      e.Message,
		Examples:     e.Examples,
		Suggestions:  e.Suggestions,
		InvalidParam: param,
		ValidValues:  values,
	}
}

// Error implements the error interface.
// Returns the full formatted error including suggestions and examples.
func (e *UsageError) Error() string {
	var sb strings.Builder

	// Main error message
	sb.WriteString(e.Message)

	// Fuzzy match suggestions
	if len(e.Suggestions) > 0 {
		sb.WriteString("\n\nDid you mean")
		if len(e.Suggestions) == 1 {
			sb.WriteString(fmt.Sprintf(": %s", e.Suggestions[0]))
		} else {
			sb.WriteString(" one of these?")
			for _, s := range e.Suggestions {
				sb.WriteString(fmt.Sprintf("\n  %s", s))
			}
		}
	}

	// Valid values for parameter
	if e.InvalidParam != "" && len(e.ValidValues) > 0 {
		sb.WriteString(fmt.Sprintf("\n\nValid values for --%s:", e.InvalidParam))
		for _, v := range e.ValidValues {
			sb.WriteString(fmt.Sprintf("\n  %s", v))
		}
	}

	// Example usage
	if len(e.Examples) > 0 {
		if len(e.Examples) == 1 {
			sb.WriteString("\n\nExample:")
		} else {
			sb.WriteString("\n\nExamples:")
		}
		for _, ex := range e.Examples {
			sb.WriteString(fmt.Sprintf("\n  %s", ex))
		}
	}

	return sb.String()
}

// FormatUsageError formats a UsageError for CLI display.
// The output includes the error message, suggestions, valid values, and examples
// in a clear, human-readable format.
func FormatUsageError(e *UsageError) string {
	var sb strings.Builder

	// Error message header
	sb.WriteString("Error: ")
	sb.WriteString(e.Message)
	sb.WriteString("\n")

	// Fuzzy match suggestions
	if len(e.Suggestions) > 0 {
		sb.WriteString("\nDid you mean")
		if len(e.Suggestions) == 1 {
			sb.WriteString(fmt.Sprintf(": %s", e.Suggestions[0]))
		} else {
			sb.WriteString(" one of these?")
			for _, s := range e.Suggestions {
				sb.WriteString(fmt.Sprintf("\n  %s", s))
			}
		}
		sb.WriteString("\n")
	}

	// Valid values for parameter
	if e.InvalidParam != "" && len(e.ValidValues) > 0 {
		sb.WriteString(fmt.Sprintf("\nValid values for --%s:\n", e.InvalidParam))
		for _, v := range e.ValidValues {
			sb.WriteString(fmt.Sprintf("  %s\n", v))
		}
	}

	// Example usage
	if len(e.Examples) > 0 {
		if len(e.Examples) == 1 {
			sb.WriteString("\nExample:\n")
		} else {
			sb.WriteString("\nExamples:\n")
		}
		for _, ex := range e.Examples {
			sb.WriteString(fmt.Sprintf("  %s\n", ex))
		}
	}

	return sb.String()
}

// FormatUsageErrorJSON formats a UsageError as JSON.
// This is useful for programmatic consumption of error information.
func FormatUsageErrorJSON(e *UsageError) string {
	output := struct {
		Command      string   `json:"command"`
		Message      string   `json:"message"`
		Examples     []string `json:"examples"`
		Suggestions  []string `json:"suggestions,omitempty"`
		InvalidParam string   `json:"invalid_param,omitempty"`
		ValidValues  []string `json:"valid_values,omitempty"`
	}{
		Command:      e.Command,
		Message:      e.Message,
		Examples:     e.Examples,
		Suggestions:  e.Suggestions,
		InvalidParam: e.InvalidParam,
		ValidValues:  e.ValidValues,
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		// Fallback to basic JSON if marshaling fails
		return fmt.Sprintf(`{"command":%q,"message":%q}`, e.Command, e.Message)
	}

	return string(jsonBytes)
}

// MissingArgError creates a UsageError for a missing required argument.
func MissingArgError(command, argName string, examples []string) *UsageError {
	msg := fmt.Sprintf("missing required argument: <%s>", argName)
	return NewUsageError(command, msg, examples)
}

// MissingFlagError creates a UsageError for a missing required flag.
func MissingFlagError(command, flagName string, examples []string) *UsageError {
	msg := fmt.Sprintf("missing required flag: --%s", flagName)
	return NewUsageError(command, msg, examples)
}

// InvalidValueError creates a UsageError for an invalid parameter value.
// It automatically adds fuzzy match suggestions from the valid values.
func InvalidValueError(command, param, value string, validValues, examples []string) *UsageError {
	msg := fmt.Sprintf("invalid value %q for --%s", value, param)
	err := NewUsageError(command, msg, examples)
	err = err.WithValidValues(param, validValues)

	// Add fuzzy match suggestions
	if len(validValues) > 0 {
		result := fuzzy.Match(value, validValues, 0.5)
		if len(result.Suggestions) > 0 {
			err = err.WithSuggestions(result.Suggestions)
		}
	}

	return err
}

// UnknownCommandError creates a UsageError for an unknown command.
// It automatically adds fuzzy match suggestions from available commands.
func UnknownCommandError(parentCmd, unknown string, availableCommands []string) *UsageError {
	msg := fmt.Sprintf("unknown command %q", unknown)
	err := NewUsageError(parentCmd, msg, nil)

	// Add fuzzy match suggestions
	if len(availableCommands) > 0 {
		result := fuzzy.SuggestCommand(unknown, availableCommands)
		if len(result.Suggestions) > 0 {
			err = err.WithSuggestions(result.Suggestions)
		}
	}

	return err
}

// UnknownFlagError creates a UsageError for an unknown flag.
// It automatically adds fuzzy match suggestions from available flags.
func UnknownFlagError(command, unknown string, availableFlags []string) *UsageError {
	msg := fmt.Sprintf("unknown flag: --%s", unknown)
	err := NewUsageError(command, msg, nil)

	// Add fuzzy match suggestions
	if len(availableFlags) > 0 {
		result := fuzzy.SuggestFlag(unknown, availableFlags)
		if len(result.Suggestions) > 0 {
			// Format suggestions with -- prefix
			suggestions := make([]string, len(result.Suggestions))
			for i, s := range result.Suggestions {
				suggestions[i] = "--" + s
			}
			err = err.WithSuggestions(suggestions)
		}
	}

	return err
}

// InvalidNodeIDError creates a UsageError for an invalid node ID format.
func InvalidNodeIDError(command, value string, examples []string) *UsageError {
	msg := fmt.Sprintf("invalid node ID %q: must be dot-separated integers (e.g., 1, 1.2, 1.2.3)", value)
	return NewUsageError(command, msg, examples)
}

// EmptyValueError creates a UsageError for an empty required value.
func EmptyValueError(command, param string, examples []string) *UsageError {
	msg := fmt.Sprintf("--%s cannot be empty", param)
	return NewUsageError(command, msg, examples)
}

// InvalidDurationError creates a UsageError for an invalid duration format.
func InvalidDurationError(command, param, value string, examples []string) *UsageError {
	msg := fmt.Sprintf("invalid duration %q for --%s: use format like 30m, 1h, 2h30m", value, param)
	return NewUsageError(command, msg, examples)
}
