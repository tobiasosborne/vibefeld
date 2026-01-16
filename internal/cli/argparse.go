// Package cli provides utilities for CLI argument parsing and user interaction.
package cli

import (
	"strings"
	"unicode"
)

// ParseArgs parses command-line arguments regardless of order, separating positional
// arguments from flags. Flags not in flagNames are treated as positional arguments.
// Boolean flags (flags with no value) will have an empty string as their value.
//
// Supports:
//   - Long flags: --flag value or --flag=value
//   - Short flags: -f value
//   - Boolean flags: --flag (no value)
//   - Double dash (--) terminates flag parsing
func ParseArgs(args []string, flagNames []string) (positional []string, flags map[string]string) {
	return ParseArgsWithBoolFlags(args, flagNames, nil)
}

// ParseArgsWithBoolFlags is like ParseArgs but explicitly marks certain flags as boolean.
// Boolean flags don't consume the next argument as their value.
// Boolean flags will have "true" as their value unless specified with --flag=value syntax.
func ParseArgsWithBoolFlags(args []string, flagNames []string, boolFlags []string) (positional []string, flags map[string]string) {
	positional = make([]string, 0)
	flags = make(map[string]string)

	if args == nil {
		return positional, flags
	}

	// Build lookup maps for O(1) checks
	flagSet := make(map[string]bool)
	for _, name := range flagNames {
		flagSet[name] = true
	}

	boolFlagSet := make(map[string]bool)
	for _, name := range boolFlags {
		boolFlagSet[name] = true
	}

	flagsTerminated := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Double dash terminates flag parsing
		if arg == "--" {
			flagsTerminated = true
			continue
		}

		// After --, everything is positional
		if flagsTerminated {
			positional = append(positional, arg)
			continue
		}

		// Try to parse as a flag
		flagName, inlineValue, isFlag := ExtractFlagName(arg)
		if !isFlag {
			positional = append(positional, arg)
			continue
		}

		// Check if this is a known flag
		if !flagSet[flagName] {
			// Unknown flag - treat as positional
			positional = append(positional, arg)
			continue
		}

		// Known flag - determine its value
		if inlineValue != "" || strings.Contains(arg, "=") {
			// Value was provided inline with = syntax
			if boolFlagSet[flagName] {
				flags[flagName] = inlineValue
			} else {
				flags[flagName] = inlineValue
			}
		} else if boolFlagSet[flagName] {
			// Explicit boolean flag - doesn't consume next arg
			flags[flagName] = "true"
		} else {
			// Check if next arg exists and should be consumed as value
			if i+1 < len(args) {
				nextArg := args[i+1]
				// If next arg looks like a flag and is a known flag, this is a boolean flag
				nextFlagName, _, nextIsFlag := ExtractFlagName(nextArg)
				if nextIsFlag && flagSet[nextFlagName] {
					// This flag has no value (boolean)
					flags[flagName] = ""
				} else if nextArg == "--" {
					// Double dash follows - this flag has no value
					flags[flagName] = ""
				} else {
					// Consume next arg as value
					flags[flagName] = nextArg
					i++
				}
			} else {
				// No more args - this is a boolean flag
				flags[flagName] = ""
			}
		}
	}

	return positional, flags
}

// NormalizeArgs reorders arguments so that positional arguments come first,
// followed by flags. This makes the args compatible with Cobra's default parsing.
func NormalizeArgs(args []string, flagNames []string) []string {
	return NormalizeArgsWithBoolFlags(args, flagNames, nil)
}

// NormalizeArgsWithBoolFlags is like NormalizeArgs but explicitly marks certain flags as boolean.
func NormalizeArgsWithBoolFlags(args []string, flagNames []string, boolFlags []string) []string {
	if args == nil || len(args) == 0 {
		return make([]string, 0)
	}

	// Build lookup maps for O(1) checks
	flagSet := make(map[string]bool)
	for _, name := range flagNames {
		flagSet[name] = true
	}

	boolFlagSet := make(map[string]bool)
	for _, name := range boolFlags {
		boolFlagSet[name] = true
	}

	var positional []string
	var flagParts []string
	var postDoublePositional []string
	flagsTerminated := false
	hadDoubleDash := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Double dash terminates flag parsing
		if arg == "--" {
			flagsTerminated = true
			hadDoubleDash = true
			continue
		}

		// After --, everything is positional
		if flagsTerminated {
			postDoublePositional = append(postDoublePositional, arg)
			continue
		}

		// Try to parse as a flag
		flagName, _, isFlag := ExtractFlagName(arg)
		if !isFlag {
			positional = append(positional, arg)
			continue
		}

		// Check if this is a known flag
		if !flagSet[flagName] {
			// Unknown flag - treat as positional
			positional = append(positional, arg)
			continue
		}

		// Known flag - collect it with its value (if any)
		if strings.Contains(arg, "=") {
			// Value inline, just add the arg
			flagParts = append(flagParts, arg)
		} else if boolFlagSet[flagName] {
			// Explicit boolean flag
			flagParts = append(flagParts, arg)
		} else {
			// Check if next arg should be consumed as value
			if i+1 < len(args) {
				nextArg := args[i+1]
				nextFlagName, _, nextIsFlag := ExtractFlagName(nextArg)
				if nextIsFlag && flagSet[nextFlagName] {
					// Next is a known flag - this is boolean
					flagParts = append(flagParts, arg)
				} else if nextArg == "--" {
					// Double dash follows
					flagParts = append(flagParts, arg)
				} else {
					// Consume next arg as value
					flagParts = append(flagParts, arg, nextArg)
					i++
				}
			} else {
				// No more args
				flagParts = append(flagParts, arg)
			}
		}
	}

	// Build result: positional from before --, positional from after --, then flags
	result := make([]string, 0, len(args))
	result = append(result, positional...)
	result = append(result, postDoublePositional...)
	result = append(result, flagParts...)
	if hadDoubleDash {
		result = append(result, "--")
	}

	return result
}

// IsFlag returns true if the argument looks like a flag (starts with - or --).
// Returns false for single dash, empty string, or negative numbers.
func IsFlag(arg string) bool {
	if len(arg) < 2 {
		return false
	}

	if arg[0] != '-' {
		return false
	}

	// Single dash is not a flag
	if len(arg) == 1 {
		return false
	}

	// Check for negative number (e.g., -123, -1.23)
	if len(arg) >= 2 && arg[0] == '-' && (unicode.IsDigit(rune(arg[1])) || (arg[1] == '.' && len(arg) > 2 && unicode.IsDigit(rune(arg[2])))) {
		return false
	}

	return true
}

// ExtractFlagName extracts the flag name from an argument.
// Returns the name (without leading dashes), any inline value (for --flag=value syntax),
// and whether the argument is actually a flag.
//
// Examples:
//
//	"--owner" -> ("owner", "", true)
//	"-o" -> ("o", "", true)
//	"--owner=alice" -> ("owner", "alice", true)
//	"--owner=" -> ("owner", "", true)
//	"1.2" -> ("", "", false)
//	"--" -> ("", "", false)
func ExtractFlagName(arg string) (name string, value string, ok bool) {
	if !IsFlag(arg) {
		return "", "", false
	}

	// Strip leading dashes
	stripped := arg
	if strings.HasPrefix(arg, "--") {
		stripped = arg[2:]
	} else if strings.HasPrefix(arg, "-") {
		stripped = arg[1:]
	}

	// Check for empty (just "--")
	if stripped == "" {
		return "", "", false
	}

	// Check for equals sign
	if idx := strings.Index(stripped, "="); idx >= 0 {
		return stripped[:idx], stripped[idx+1:], true
	}

	return stripped, "", true
}
