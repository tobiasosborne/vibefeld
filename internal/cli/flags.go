// Package cli provides utilities for CLI argument parsing and user interaction.
package cli

import "github.com/spf13/cobra"

// MustString retrieves a string flag value from a cobra command.
// Panics if the flag was not registered, which is a programming error.
//
// Cobra's GetString only returns an error when the flag doesn't exist
// (not registered), which would be caught immediately during development.
// This helper makes the intent clear and avoids silently ignored errors.
func MustString(cmd *cobra.Command, name string) string {
	val, err := cmd.Flags().GetString(name)
	if err != nil {
		panic("flag not registered: " + name)
	}
	return val
}

// MustBool retrieves a boolean flag value from a cobra command.
// Panics if the flag was not registered, which is a programming error.
func MustBool(cmd *cobra.Command, name string) bool {
	val, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic("flag not registered: " + name)
	}
	return val
}

// MustInt retrieves an integer flag value from a cobra command.
// Panics if the flag was not registered, which is a programming error.
func MustInt(cmd *cobra.Command, name string) int {
	val, err := cmd.Flags().GetInt(name)
	if err != nil {
		panic("flag not registered: " + name)
	}
	return val
}

// MustStringSlice retrieves a string slice flag value from a cobra command.
// Panics if the flag was not registered, which is a programming error.
func MustStringSlice(cmd *cobra.Command, name string) []string {
	val, err := cmd.Flags().GetStringSlice(name)
	if err != nil {
		panic("flag not registered: " + name)
	}
	return val
}
