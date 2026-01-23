// Package main contains the af def-add command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/service"
)

// newDefAddCmd creates the def-add command for adding definitions.
func newDefAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "def-add <name> [content]",
		GroupID: GroupAdmin,
		Short:   "Add a definition to the proof",
		Long: `Add a definition to the proof.

Definitions provide formal terms that can be referenced in proof steps.
This command allows human operators to provide definitions for terms
that have been requested during proof work.

The content can be provided as an argument or read from a file using --file.
If both are provided, the --file content takes precedence.

Examples:
  af def-add group "A group is a set with a binary operation."
  af def-add homomorphism --file definition.txt
  af def-add kernel "The kernel of f is ker(f) = {x : f(x) = e}" --format json
  af def-add vector_space -d ./proof --file vs_def.txt`,
		RunE: runDefAdd,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().String("file", "", "Read definition content from file")

	return cmd
}

// runDefAdd executes the def-add command.
func runDefAdd(cmd *cobra.Command, args []string) error {
	// Get flags
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	filePath := cli.MustString(cmd, "file")

	// Validate we have at least a name argument
	if len(args) < 1 {
		return fmt.Errorf("requires at least 1 arg(s), only received 0")
	}

	// Get name from first argument
	name := args[0]

	// Validate name is not empty or whitespace
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("definition name cannot be empty")
	}

	// Get content from args or file
	var content string

	// If --file is provided, read content from file (takes precedence)
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("error reading file %q: %w", filePath, err)
		}
		content = string(data)
	} else if len(args) >= 2 {
		// Get content from second argument
		content = args[1]
	}

	// Validate content is not empty or whitespace
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("definition content cannot be empty")
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Add the definition via service
	defID, err := svc.AddDefinition(name, content)
	if err != nil {
		return fmt.Errorf("error adding definition: %w", err)
	}

	// Output result based on format
	switch strings.ToLower(format) {
	case "json":
		return outputDefAddJSON(cmd, name, defID, content)
	default:
		return outputDefAddText(cmd, name, defID)
	}
}

// outputDefAddJSON outputs the def-add result in JSON format.
func outputDefAddJSON(cmd *cobra.Command, name, defID, content string) error {
	result := map[string]interface{}{
		"name":  name,
		"id":    defID,
		"added": true,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputDefAddText outputs the def-add result in human-readable text format.
func outputDefAddText(cmd *cobra.Command, name, defID string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Definition '%s' added with ID: %s\n", name, defID)
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af defs     - List all definitions")
	fmt.Fprintln(cmd.OutOrStdout(), "  af def NAME - View a specific definition")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status   - Check proof status")

	return nil
}

func init() {
	rootCmd.AddCommand(newDefAddCmd())
}
