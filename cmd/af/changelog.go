package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// changelog is an ordered list of releases, newest first.
var changelog = []release{
	{
		Version: "0.1.3",
		Date:    "2026-04-26",
		Items: []string{
			"af unadmit — revoke an admission and revert a node to pending. Symmetric with af unvalidate. Use when admit was a temporary escape hatch and the claim has now been rigorously verified.",
		},
	},
	{
		Version: "0.1.2",
		Date:    "2026-04-26",
		Items: []string{
			"Fix: af accept now treats archived children as terminal-cleared, so an abandoned sub-tree no longer blocks parent re-validation",
			"Fix: 13 CLI commands (get, refine, claim, deps, diff, etc.) now write to stdout instead of stderr — pipes like `af get | jq` work again",
		},
	},
	{
		Version: "0.1.1",
		Date:    "2026-02-15",
		Items: []string{
			"af attach / af evidence — link computational evidence (scripts, results) to proof nodes",
			"af approach-tried / af approach-list — record and review failed proof strategies",
			"af unvalidate — revert validated nodes back to pending for re-examination",
			"af amendments / af diff — view node version history and diffs between amendments",
			"af status --focus / --depth / --compact — navigate large proof trees",
			"af path / af nearby — ancestry chain and neighborhood views",
			"af submit / af refine --draft — draft/WIP workflow for iterative development",
			"af handoff — generate session handoff reports",
			"af challenges --severity / --summary / --active-only — challenge triage filters",
			"af strategy-propose / af strategy-list — record proposed proof strategies",
			"af pattern-add / af pattern-list — register failure patterns per workspace",
			"af veto — human expert force-refute",
		},
	},
	{
		Version: "0.1.0",
		Date:    "2026-01-17",
		Items: []string{
			"Initial release — adversarial proof framework with full prover/verifier workflow",
		},
	},
}

type release struct {
	Version string
	Date    string
	Items   []string
}

func newChangelogCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "changelog",
		GroupID: GroupUtil,
		Short:   "Show what's new in each version",
		Long:    "Show changelog of features added in each af release.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			for i, r := range changelog {
				fmt.Fprintf(out, "=== %s (%s) ===\n", r.Version, r.Date)
				for _, item := range r.Items {
					fmt.Fprintf(out, "  - %s\n", item)
				}
				if i < len(changelog)-1 {
					fmt.Fprintln(out)
				}
			}
			return nil
		},
	}
}

func init() {
	rootCmd.AddCommand(newChangelogCmd())
}
