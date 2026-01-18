// Package main contains the af inferences command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// newInferencesCmd creates the inferences command.
func newInferencesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "inferences",
		GroupID: GroupQuery,
		Short:   "List all valid inference types",
		Long: `List all valid inference types for use with 'af refine -j TYPE'.

Inference types are the logical rules used to justify proof steps. Each
inference type has an ID (used with -j flag), a human-readable name, and
a logical form showing its structure.

Examples:
  af inferences                    List all inference types
  af inferences --format json      Machine-readable output
  af refine -i modus_ponens ...    Use an inference type`,
		RunE: runInferences,
	}

	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// runInferences executes the inferences command.
func runInferences(cmd *cobra.Command, args []string) error {
	// Get flags
	format, _ := cmd.Flags().GetString("format")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Get all inference types (already sorted by service.AllInferences)
	inferences := service.AllInferences()

	// Output based on format
	if format == "json" {
		output := renderInferencesJSON(inferences)
		fmt.Fprintln(cmd.OutOrStdout(), output)
		return nil
	}

	// Text format
	output := renderInferencesText(inferences)
	fmt.Fprint(cmd.OutOrStdout(), output)

	// Add summary
	fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d inference type(s)\n", len(inferences))

	// Add next steps
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af refine -j <type> ...  - Use an inference type in a refinement")

	return nil
}

// renderInferencesText renders inferences as a text table.
func renderInferencesText(inferences []service.InferenceInfo) string {
	if len(inferences) == 0 {
		return "No inference types found.\n"
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("%-28s %-26s %s\n",
		"TYPE", "NAME", "FORM"))

	// Rows
	for _, inf := range inferences {
		sb.WriteString(fmt.Sprintf("%-28s %-26s %s\n",
			inf.ID, inf.Name, inf.Form))
	}

	return sb.String()
}

// inferenceJSON is the JSON representation of an inference type.
type inferenceJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Form string `json:"form"`
}

// inferencesResultJSON is the JSON wrapper for inferences output.
type inferencesResultJSON struct {
	Inferences []inferenceJSON `json:"inferences"`
	Total      int             `json:"total"`
}

// renderInferencesJSON renders inferences as JSON.
func renderInferencesJSON(inferences []service.InferenceInfo) string {
	result := inferencesResultJSON{
		Inferences: make([]inferenceJSON, 0, len(inferences)),
		Total:      len(inferences),
	}

	for _, inf := range inferences {
		ij := inferenceJSON{
			ID:   string(inf.ID),
			Name: inf.Name,
			Form: inf.Form,
		}
		result.Inferences = append(result.Inferences, ij)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal JSON: %v"}`, err)
	}

	return string(data)
}

func init() {
	rootCmd.AddCommand(newInferencesCmd())
}
