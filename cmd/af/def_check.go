package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

func newDefCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "def-check <def-name-or-id>",
		GroupID: GroupVerifier,
		Short:   "Run stress test on a definition",
		Long: `Run automated stress tests on a registered definition.

Tests verify boundary cases, non-triviality, or other properties via
user-provided scripts. Failed tests warn provers before building proof
trees on flawed definitions.

Examples:
  af def-check "O-admissible family" --script ./check_boundary.py
  af def-check D1 --script ./non_trivial.sh --check-type non_triviality
  af def-check D1 --script ./verify.sh --agent verifier-1

Script convention:
  Exit code 0 = check passed (definition not degenerate)
  Exit code non-zero = check failed (definition may be vacuous)
  stdout/stderr captured for audit trail`,
		Args: cobra.ExactArgs(1),
		RunE: runDefCheck,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")
	cmd.Flags().String("script", "", "Path to check script (required)")
	cmd.Flags().String("check-type", "script", "Check type (boundary|non_triviality|script)")
	cmd.Flags().String("agent", "", "Agent ID running the check")

	return cmd
}

func runDefCheck(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	script := cli.MustString(cmd, "script")
	checkType := cli.MustString(cmd, "check-type")
	agent := cli.MustString(cmd, "agent")

	defNameOrID := args[0]

	examples := []string{
		"af def-check \"my definition\" --script ./check.sh",
		"af def-check D1 --script ./boundary.py --check-type boundary",
	}

	if strings.TrimSpace(script) == "" {
		return render.MissingFlagError("af def-check", "script", examples)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	passed, output, err := svc.RunDefCheck(defNameOrID, checkType, script, agent)
	if err != nil {
		return fmt.Errorf("def-check failed: %w", err)
	}

	return outputDefCheckResult(cmd, defNameOrID, checkType, passed, output, format)
}

func outputDefCheckResult(cmd *cobra.Command, defName, checkType string, passed bool, output, format string) error {
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"def_name":   defName,
			"check_type": checkType,
			"passed":     passed,
		}
		if output != "" {
			result["output"] = output
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
	default:
		status := "PASSED"
		if !passed {
			status = "FAILED"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Definition check %s for %q (type: %s)\n", status, defName, checkType)
		if output != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "\nOutput:\n%s\n", output)
		}
		if passed {
			fmt.Fprintln(cmd.OutOrStdout(), "\nDefinition appears non-degenerate.")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "\nDefinition may be vacuous or degenerate! Review before building on it.")
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newDefCheckCmd())
}
