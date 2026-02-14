package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

func newCoverageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "coverage",
		GroupID: GroupSetup,
		Short:   "Compute outline coverage metrics",
		Long: `Compute coverage metrics showing what fraction of outline stages
have been started and what fraction are fully validated.

Requires an outline to be set via 'af outline set'.

Examples:
  af coverage
  af coverage --format json`,
		RunE: runCoverage,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

func runCoverage(cmd *cobra.Command, args []string) error {
	dir := service.MustString(cmd, "dir")
	format := service.MustString(cmd, "format")

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	report, err := svc.GetOutlineCoverage()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if strings.ToLower(format) == "json" {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("error encoding JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
		return nil
	}

	// Text format
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Outline Coverage")
	fmt.Fprintln(out, strings.Repeat("=", 50))
	fmt.Fprintf(out, "Stages: %d total, %d mapped, %d started, %d complete\n\n", report.StagesTotal, report.StagesMapped, report.StagesStarted, report.StagesComplete)

	for _, s := range report.Stages {
		prefix := " "
		if s.Criticality == "critical" && !s.Started {
			prefix = "!"
		}
		coverage := "unmapped"
		if s.Mapped {
			if s.TotalNodes == 0 {
				coverage = fmt.Sprintf("%s — 0 nodes", s.NodeID)
			} else {
				coverage = fmt.Sprintf("%s — %d/%d validated (%.0f%%)", s.NodeID, s.Validated, s.TotalNodes, s.Fraction*100)
			}
		}
		fmt.Fprintf(out, "  [%s] %s (%s) — %s — %s\n", prefix, s.Label, s.Criticality, coverage, s.Description)
	}

	if len(report.CriticalUntouched) > 0 {
		fmt.Fprintf(out, "\nWarning: %d critical stage(s) untouched: %s\n", len(report.CriticalUntouched), strings.Join(report.CriticalUntouched, ", "))
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newCoverageCmd())
}
