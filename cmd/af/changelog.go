package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// changelog is an ordered list of releases, newest first.
var changelog = []release{
	{
		Version:    "0.1.11",
		Date:       "unreleased",
		Unreleased: true,
		Items: []string{
			"Taint follows proof support (D6). A node's support component is computed as a fold over result-use edges in dependency-topological order, so reference dependencies and validation dependencies now carry taint exactly like children: a severed or missing dependency target is `unresolved`, a pending/draft/needs_refinement target is `unresolved`, an admitted target contributes `tainted` and is not descended, and a validated target contributes its own component. Legacy result-use cycles (strongly connected components) are `unresolved`, and local_assume hypothesis-use edges carry nothing. The ancestor-context component stays separate, so a validated sibling of an admitted node stays `clean`. The old rules 0-7 in `docs/concepts.md` are replaced by the new ordered rule list.",
			"`PropagateTaint`'s affected set now includes reverse dependents (nodes that cite the changed node through dependencies or validation dependencies, transitively), not only ancestors and descendants, and the `TaintRecomputed` audit-emission filter matches. Replay keeps its single authoritative final derivation (moved to the service load path) so stale historical audit events cannot override derived taint.",
			"`af taint-trace` walks the support relation (child, reference dependency, validation dependency), names the edge kind and reports the source's recorded verdict sequence or latest statement/dependency amendment, e.g. `1 tainted via child 1.1 (admitted, verdict seq 5)`; the JSON output adds `support_sources`.",
			"Verification: an independent recursive specification (`internal/taint/spec_test.go`) written from the rule list, and a seeded differential fuzz (`internal/taint/support_fold_fuzz_test.go`, 3000 random 5-40 node graphs with mixed edges, local_assume scopes, missing targets, legacy cycles and random command outcomes) asserting production == spec, idempotence of `RecomputeAll`, incremental `PropagateTaint` == full derivation, no clean node has a support path through a non-validated result, and sibling separation. `scripts/corpus-taint-diff.sh` diffs `af export --graph json` between two binaries over the 211-workspace corpus; only `taint_state` and `validation.taint_counts` are allowlisted to change, and the AISM corpus diff is 0/211 workspaces (the corpus has no reference or validation edge into a non-clean node).",
		},
	},
	{
		Version: "0.1.10",
		Date:    "2026-09-16",
		Items: []string{
			"`af health` no longer infers that repeated scrutiny means a claim is false. The absolute subtree repair-fatigue alarm is removed and replaced with descriptive per-node rework: resolved challenges, statement and dependency amendments, and refuted children. The top hotspots are listed (configurable with `--hotspots`, default 5) and a per-node `--rework-warn` threshold (default 5) marks a warning, explicitly labelled rework rather than evidence of falsity. Health blockers now also report open challenges with severity and age, and stalled (held longer than the lock timeout) or stale (expired) claims with their owner and expiry; claims record their acquisition time separately from their expiry.",
			"One jobs classifier: `af status`'s Prover/Verifier summary, `af status --urgent`, `af get`, `af export --graph json` and `af health` now all use `internal/jobs` (`FindJobs` / `IsProverJob` / `IsVerifierJob`), and the render-side workflow-only heuristic is deleted, so the surfaces can no longer disagree.",
			"`af get` surfaces `validated_by` and `validation_batch_id`, the claim owner/claimed-at/expiry, and `prover_ready` / `verifier_ready` from the authoritative classifier in both text and JSON.",
			"CLI help and error examples no longer show removed flags (`af refine ... -s/--statement`, `af challenge --owner`); `scripts/auto-prove.sh` uses `accept --with-note`, carries a per-worker identity on every generated claim/challenge/refine command, and treats completion as root validated AND taint clean AND no pending external reference cited by a validated node.",
			"`support_current`: a derived, recursive, revision-aware signal for whether a node's recorded `validated` verdict is still supported by the proof under it. It is computed by one memoised walk over result-use edges (`internal/support`), treats legacy result-use cycles as unresolved, and reports a stable cause (`NOT_VALIDATED`, `OPEN_BLOCKING_CHALLENGE`, `TARGET_NOT_CURRENT`, `TARGET_PENDING`, `TARGET_REFUTED`, `TARGET_REVISED`, `SELF_REVISED`, `CYCLE`). `af status` marks not-current validated nodes with `!` and a legend line and adds `support_current` to JSON; `af get` shows the cause and responsible node/seq; `af health` adds a blocker per cause; `af export --graph json` gains per-node `support_current`/`support_cause` behind the new `support-current` capability token. Replay records each node's validation sequence as derived state (no event change).",
			"One acceptance-eligibility validator (`internal/service/accept_eligibility.go`) is now shared by single accept, bulk accept and `af verdicts apply`, and bulk accept is scheduled by actual prerequisites (children and validation dependencies first, ID order as tie-break) in one commit with per-item outcomes (`applied` / `blocked:<code>` / `rejected:<code>`) and the `af verdicts apply` exit tiers (5 partial, 6 none). This closes the hole where bulk accept skipped the children-validated and validation-deps checks, and the hole where `af accept --all` could validate a parent before its pending child.",
			"Creation gate: a node may only be created (refine, bulk refine, record-proof, direct create) under a `pending`, `draft` or `needs_refinement` parent. A `validated` parent is refused with `run af request-refinement <id>`; an `admitted` parent with `run af unadmit <id>`; a `refuted` or `archived` parent is refused with no remedy. Typed errors, exit 3. This closes the hole where refining a validated parent left it looking validated with pending children.",
			"Jobs classifier: `needs_refinement` is a prover job until its children are cleared, then a verifier job (previously it was neither). `af jobs`, `af health` and `af handoff` use the one `internal/jobs` classifier.",
			"`af audit` (D8): a read-only, text/JSON trust audit computed from one ordered ledger pass over derived state. Findings carry a stable code, a severity (`error`/`warning`/`info`), a `current | historical` status, node IDs, ledger sequences and a remediation; the report carries `schema_version: 1`. Strict-current codes are `SUPPORT_NOT_CURRENT`, `HASH_MISMATCH` (recorded), `CYCLE`, `SCOPE_LEAK`, `CITES_SEVERED`, `AMENDED_NOT_REVERIFIED`, `SELF_ACCEPT` (only when an identity was recorded) and `VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE`; historical classes (`ADMITTED`, `ARCHIVED_WITH_OPEN_CHALLENGE`, `AMENDMENTS_PER_NODE`, `PENDING_EXTERNAL_CITED_BY_VALIDATED`, `UNKNOWN_PROVENANCE`, and hash checks that were never recorded) are reported and never gate. `af audit --strict` exits 3 (`AUDIT_FAILED`) when a current finding gates; otherwise it exits 0. Output is bounded by `--limit` (default 200) and filterable by `--code`, `--node <prefix>` and `--status`.",
			"D8 migration preflight/postcheck: `af amend-deps --file m.json --dry-run` and the real manifest run both end with a one-line audit summary of the current workspace (strict-current and historical finding counts) and the exact `af audit --strict` command; in JSON mode the summary is the `audit_summary` field so stdout stays valid JSON. `af health`'s support-failure blockers are now read from the same `internal/audit` findings engine (`SUPPORT_NOT_CURRENT`) instead of a second `support.Current` call, and `scripts/corpus-check.sh` runs `af audit -f json` (non-strict, must exit 0) over every corpus workspace.",
			"Claim generations and fenced auto-release (D5): a claim's generation is the ledger sequence of the `nodes_claimed` event that created it, shown as `claim_seq` by `af get` and `af jobs -f json` and cleared on release. `NodeValidated`/`NodeAdmitted`/`NodeRefuted`/`NodeArchived` carry an optional `claim_seq` and `release_claim`; a terminal action taken by the claim holder releases the claim in the same event, and replay releases only when the generation still matches, so a retried or delayed release cannot evict a later claim. This closes the hole where `af accept` left a node claimed. `af release` afterwards is a documented no-op (exit 0). Legacy events replay unchanged.",
			"`af admit`, `af refute` and `af archive` record the acting agent identity (`--agent` or `AF_AGENT_ID`) as an optional `by` on their events, and auto-release the caller's claim on the same `claim_seq` fence as accept (D5). `internal/lock.GetLockInfo` now reads a `ClaimLock` under its mutex and applies `ClockSkewTolerance` like `IsExpired` (uj18).",
			"Guardrails labelled as guardrails (D9): `af archive` refuses when a challenge is open on the node or on an active (non-severed) descendant unless `--force --reason`; `NodeArchived` records `reason` and `forced`, and the verification checklist lists children archived with a challenge open so the next accept acknowledges the abandoned obligation. `af accept` with an identity refuses when the verifier is recorded as the node's author, proof author or an amender; `--allow-self` is explicit and records `self_accepted: true`; verdict files run the same check and cannot opt out. Without `--agent`/`AF_AGENT_ID`, `af accept` prints a one-line warning (text output) and the identity becomes required in 0.1.11. This is recorded provenance, not proof of independence (see `docs/trust-model.md`).",
			"Benchmark job: `scripts/synth-workspace.sh` generates a deterministic synthetic workspace (N nodes, high fan-out, cross-reference deps, verification rounds) through real `af` commands, and `scripts/benchmark.sh` measures 100- and 1000-node workspaces at 10 and 50 concurrent writers: per-command p50/p95 latency, exit-1 retries, lock waits, events and bytes appended, single-command latency for `status`/`jobs`/`health`/`audit`/`export --graph json`/bulk accept, and the per-event directory-fsync cost via the test-only `AF_TEST_NO_FSYNC` switch. The e2e scale test (`-tags integration`) builds 1000 nodes through the service API and asserts replay, contiguity, audit and `support_current` invariants without wall-time assertions.",
		},
	},
	{
		Version: "0.1.9",
		Date:    "2026-09-16",
		Items: []string{
			"Commit primitive: every mutating command now validates and appends against one state read (loaded outside the ledger lock), with a batch-wide sequence check under the lock and a directory fsync after each event, so a crash leaves a valid ledger prefix and a concurrent write is refused rather than interleaved. `af verdicts apply` checks `expect_hash`, verifier readiness and reviewer≠author on the same read it accepts against. Events may carry an optional `operation_id`; the ledger lock records pid and a release token; `af reap --ledger-lock` removes only a dead or expired ledger lock.",
			"Cycle and scope checks on every creation path (`refine`, bulk refine, `record-proof`, node creation): dependencies are result-use edges (children and cited nodes that are not local assumptions) checked from the new node's own position over the whole prospective batch; citing a `claim` ancestor is a cycle, citing an enclosing `local_assume` is allowed, and citing a node inside a local assumption that does not enclose the citer is a scope leak (SCOPE_LEAK, exit 3).",
			"Workspace format activation: `meta.json` now records a workspace format, new workspaces are stamped 1.1, and `af version -f json` advertises `format` and `policy`. This binary reads formats 1.0 and 1.1 but refuses a newer or unknown format with a structured error (FORMAT_TOO_NEW, exit 3).",
			"`af workspace upgrade --to 1.1 [--dry-run] [-f json]` migrates a 1.0 workspace: it takes the ledger lock, copies `ledger/*.json` and `meta.json` into `backup/<UTC timestamp>/`, fsyncs them, and writes the new stamp atomically. Downgrades are refused; restore the backup to revert.",
			"Event types now carry a minimum workspace format; a 1.0 workspace refuses a 1.1 event type with the instruction to run `af workspace upgrade --to 1.1`. Replay, which bypasses the service entry point, enforces the same gate before replaying.",
			"`af replay -f json` now exits non-zero (exit 4, corruption) when the replay is invalid instead of printing `valid:false` and exiting 0. The JSON payload is still printed.",
			"Acceptance now records what it accepted: `NodeValidated` carries an optional `content_hash` (the node's own fields and dependency IDs, read from the state the accept commits against) and `expected_hash_checked` (true only when a verdict item's `expect_hash` or `af accept --expect-hash` was compared). `af accept` gains `--expect-hash` for single-node use, `af get` and `af export --graph json` surface the recorded hash, and `ClaimTested` records its content hash so acceptance ignores a passing claim-test run against older content (legacy tests without a hash still count).",
			"`af amend-deps` corrects dependency edges with one append-only `node_deps_amended` event (format 1.1), running the D1 result-use cycle and scope checks over the prospective node. It supports reference and validation edges, `--reopen` (which applies `validated -> pending` and the edge replacement in the same event), `--expect-hash`, and `--strict`, and rejects an `admitted` node until `af unadmit`. A manifest form (`--file`) previews with `--dry-run`, persists generated `operation_id`s, resumes after a crash, and reports every item once with exit codes 5/6/7 matching `af verdicts apply`. `af amend` gains `--reopen` and accepts `needs_refinement` nodes. `af get`, `af amendments`, `af deps`, `af diff` and `af export --graph json` surface dependency amendments (export under the `dependency-amendments` / `validation-deps` capability tokens).",
		},
	},
	{
		Version: "0.1.8",
		Date:    "2026-09-14",
		Items: []string{
			"Fix: `go install github.com/tobiasosborne/vibefeld/cmd/af@...` now works. The module path declared in go.mod did not match the repository location; go.mod, all self-imports, and the docs now use github.com/tobiasosborne/vibefeld (GitHub issue #2).",
			"Fix: scripts/build.sh reads the version with POSIX sed instead of GNU-only `grep -oP`, so the stamped build/install works with macOS's BSD grep.",
		},
	},
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
			"Consolidates 16 previously unversioned commits since 0.1.5: af record-proof (serialized prover write: refine + dispose challenge + release, with proof_author stamping and free-text justification/inference labels), af verdicts apply (atomic expect_hash + verifier-ready re-check, batch_id optional for single-item applies) and af unvalidate --batch, internal/verdicts batch verdict-file schema with exit codes 5-7 for batch outcomes, af export --graph json gaining a schema_version, author/validated_by/validation_batch_id, per-node closed flag, always-present features capability list and prover_ready/verifier_ready flags, author/verifier identity wired into init/refine/accept, and per-child dependency recording through RefineNodeBulk.",
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

	// Unreleased marks an entry that is in the tree but has not been cut as a
	// release, so VersionInfo is not required to match it yet.
	Unreleased bool
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
