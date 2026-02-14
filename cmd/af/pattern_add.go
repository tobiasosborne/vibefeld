// Package main contains the af pattern-add command implementation.
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

func newPatternAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pattern-add",
		GroupID: GroupAdmin,
		Short:   "Register a failure pattern",
		Long: `Register a known failure pattern in the workspace.

Failure patterns capture recurring proof anti-patterns so agents can
recognize and avoid them. Each pattern includes a name, description,
indicators (how to recognize it), and remediation (what to do instead).

Examples:
  af pattern-add --name "continuum-to-finite" --description "Applying infinite-dimensional identities at finite n" --indicators "Claims using limits or integrals for finite objects" --remediation "Verify identity holds at finite n before using"
  af pattern-add -n "known-problem-substitution" -D "Reducing novel problems to solved ones without justification" -f json`,
		Args: cobra.NoArgs,
		RunE: runPatternAdd,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("name", "n", "", "Pattern name (required)")
	cmd.Flags().StringP("description", "D", "", "What goes wrong (required)")
	cmd.Flags().StringP("indicators", "I", "", "How to recognize this pattern")
	cmd.Flags().StringP("remediation", "R", "", "What to do instead")
	cmd.Flags().String("agent", "", "Agent ID")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

func runPatternAdd(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	name := cli.MustString(cmd, "name")
	description := cli.MustString(cmd, "description")
	indicators := cli.MustString(cmd, "indicators")
	remediation := cli.MustString(cmd, "remediation")
	agent := cli.MustString(cmd, "agent")
	format := cli.MustString(cmd, "format")

	if strings.TrimSpace(name) == "" {
		return render.MissingFlagError("af pattern-add", "name", []string{
			`af pattern-add --name "continuum-to-finite" --description "..."`,
		})
	}
	if strings.TrimSpace(description) == "" {
		return render.MissingFlagError("af pattern-add", "description", []string{
			`af pattern-add --name "..." --description "Applying infinite-dim identities at finite n"`,
		})
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	if err := svc.AddPattern(name, description, indicators, remediation, agent); err != nil {
		return fmt.Errorf("error adding pattern: %w", err)
	}

	return outputPatternAddResult(cmd, name, description, indicators, remediation, format)
}

func outputPatternAddResult(cmd *cobra.Command, name, description, indicators, remediation, format string) error {
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"name":        name,
			"description": description,
		}
		if indicators != "" {
			result["indicators"] = indicators
		}
		if remediation != "" {
			result["remediation"] = remediation
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Failure pattern registered: %s\n", name)
		fmt.Fprintf(cmd.OutOrStdout(), "  Description: %s\n", description)
		if indicators != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Indicators: %s\n", indicators)
		}
		if remediation != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Remediation: %s\n", remediation)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newPatternAddCmd())
}
