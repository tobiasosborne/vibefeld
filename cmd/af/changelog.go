package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// changelog is an ordered list of releases, newest first.
var changelog = []release{
	{
		Version: "0.1.7",
		Date:    "2026-09-02",
		Items: []string{
			"Fix: taint now propagates upward from admitted, pending, draft, or needs-refinement descendants to their ancestors while validated siblings remain uncontaminated; replay authoritatively recomputes derived taint so existing 0.1.6 ledgers self-heal on their next load, and af taint-trace explains descendant sources.",
			"Fix: needs_refinement now counts as unresolved for taint on the reopened node, its ancestors, and its descendants, so requesting more proof no longer leaves the root looking clean.",
			"Compatibility note: af taint-trace JSON keys remain backward compatible and only gain additive fields, but human-readable `reason` values now name the nearest offending ancestor/descendant and report its epistemic state; scripts matching reason text should update.",
		},
	},
	{
		Version: "0.1.6",
		Date:    "2026-07-25",
		Items: []string{
			"Version stamping fixed: af version's ldflags-backed VersionInfo/GitCommit/BuildDate is now the single source of truth for both `af --version` and `af version --json` (the old hardcoded main.go const had drifted from it, and the unstamped-build default was the unparseable placeholder \"dev\" — that broke rk doctor's D6 stale-binary detection outright). scripts/build.sh stamps version, commit, and build date via ldflags for both build and install.",
			"Consolidates 16 previously unversioned commits since 0.1.5: af record-proof (atomic prover write: refine + dispose challenge + release, with proof_author stamping and free-text justification/inference labels), af verdicts apply (atomic expect_hash + verifier-ready re-check, batch_id optional for single-item applies) and af unvalidate --batch, internal/verdicts batch verdict-file schema with exit codes 5-7 for batch outcomes, af export --graph json gaining a schema_version, author/validated_by/validation_batch_id, per-node closed flag, always-present features capability list and prover_ready/verifier_ready flags, author/verifier identity wired into init/refine/accept, and per-child dependency recording through RefineNodeBulk.",
		},
	},
	{
		Version: "0.1.5",
		Date:    "2026-07-02",
		Items: []string{
			"af jobs --ready — list only verifier jobs whose children are all cleared (validated/admitted/archived), i.e. acceptable now. Server-side bottom-up-ready filter that orchestration drivers otherwise re-implement each round.",
			"af init now writes a workspace .gitignore covering runtime/rebuildable state (locks/, .af/, nodes/, defs/, lemmas/) while keeping ledger/, assumptions/, externals/, and meta.json tracked.",
			"af challenge --category — optional typed classification (gap, missing, dependency, incorrect, unclear, other) so tooling can classify challenges exactly instead of grepping the reason text. af challenges gains a --category filter and includes category in JSON output.",
		},
	},
	{
		Version: "0.1.4",
		Date:    "2026-07-02",
		Items: []string{
			"Fix: --dry-run is no longer silently ignored. It was a global no-op — every mutating command (def-add, refine, accept, ...) accepted the flag and wrote anyway. Commands that don't implement dry-run now refuse the flag loudly instead of mutating; af def-add --dry-run previews without writing and warns on duplicate names.",
		},
	},
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
