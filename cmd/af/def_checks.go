package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/service"
)

func newDefChecksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "def-checks <def-name-or-id>",
		GroupID: GroupQuery,
		Short:   "List stress test history for a definition",
		Long: `Show all stress tests run against a definition.

Displays check results chronologically with type, pass/fail status, and output.

Examples:
  af def-checks "O-admissible family"
  af def-checks D1 -f json`,
		Args: cobra.ExactArgs(1),
		RunE: runDefChecks,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

func runDefChecks(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	defNameOrID := args[0]

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	// Resolve definition name
	defName := defNameOrID
	def := st.GetDefinitionByName(defNameOrID)
	if def == nil {
		def = st.GetDefinition(defNameOrID)
	}
	if def == nil {
		return fmt.Errorf("definition %q not found", defNameOrID)
	}
	defName = def.Name

	results := st.GetDefChecks(defName)

	switch strings.ToLower(format) {
	case "json":
		type jsonResult struct {
			Timestamp  string `json:"timestamp"`
			CheckType  string `json:"check_type"`
			ScriptPath string `json:"script_path,omitempty"`
			Passed     bool   `json:"passed"`
			Output     string `json:"output,omitempty"`
			Agent      string `json:"agent,omitempty"`
		}
		jsonResults := make([]jsonResult, len(results))
		for i, r := range results {
			jsonResults[i] = jsonResult{
				Timestamp:  r.Timestamp.String(),
				CheckType:  r.CheckType,
				ScriptPath: r.ScriptPath,
				Passed:     r.Passed,
				Output:     r.Output,
				Agent:      r.Agent,
			}
		}
		out := map[string]interface{}{
			"def_name": defName,
			"count":    len(results),
			"checks":   jsonResults,
		}
		jsonBytes, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
	default:
		if len(results) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No checks for definition %q.\n", defName)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Checks for definition %q (%d total):\n\n", defName, len(results))
		for i, r := range results {
			status := "PASS"
			if !r.Passed {
				status = "FAIL"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. [%s] type=%s", i+1, status, r.CheckType)
			if r.Agent != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " agent=%s", r.Agent)
			}
			fmt.Fprintf(cmd.OutOrStdout(), " at %s\n", r.Timestamp.String())
			if r.Output != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Output: %s\n", strings.TrimSpace(r.Output))
			}
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newDefChecksCmd())
}
