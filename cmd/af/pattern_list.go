// Package main contains the af pattern-list command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
)

func newPatternListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pattern-list",
		Aliases: []string{"failure-patterns"},
		GroupID: GroupQuery,
		Short:   "List registered failure patterns",
		Long: `Show all failure patterns registered in this workspace.

Failure patterns capture recurring proof anti-patterns so agents can
recognize and avoid them before re-attempting dead ends.

Examples:
  af pattern-list
  af pattern-list -f json`,
		Args: cobra.NoArgs,
		RunE: runPatternList,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

func runPatternList(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	patterns := st.GetPatterns()
	return outputPatternList(cmd, patterns, format)
}

func outputPatternList(cmd *cobra.Command, patterns []state.FailurePattern, format string) error {
	switch strings.ToLower(format) {
	case "json":
		type jsonPattern struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Indicators  string `json:"indicators,omitempty"`
			Remediation string `json:"remediation,omitempty"`
			AddedBy     string `json:"added_by,omitempty"`
			Timestamp   string `json:"timestamp"`
		}
		items := make([]jsonPattern, len(patterns))
		for i, p := range patterns {
			items[i] = jsonPattern{
				Name:        p.Name,
				Description: p.Description,
				Indicators:  p.Indicators,
				Remediation: p.Remediation,
				AddedBy:     p.AddedBy,
				Timestamp:   p.Timestamp.String(),
			}
		}
		result := map[string]interface{}{
			"count":    len(patterns),
			"patterns": items,
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		if len(patterns) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No failure patterns registered in this workspace.")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Failure patterns (%d total):\n\n", len(patterns))
		for i, p := range patterns {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, p.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "     Description: %s\n", p.Description)
			if p.Indicators != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Indicators: %s\n", p.Indicators)
			}
			if p.Remediation != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Remediation: %s\n", p.Remediation)
			}
			if p.AddedBy != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Added by: %s\n", p.AddedBy)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "     When: %s\n", p.Timestamp.String())
			if i < len(patterns)-1 {
				fmt.Fprintln(cmd.OutOrStdout())
			}
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newPatternListCmd())
}
