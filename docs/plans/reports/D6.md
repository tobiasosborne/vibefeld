# REPORT-D6 — Taint follows proof support

Bead: `vibefeld-cmww` · branch `work/d6-support-taint` · plan
`docs/plans/scale-hardening.md` § D6 (0.1.11, v3.1 amendment 4) · recreates
the missing `0ry1` · base `f55e477`, head `71e2388` (3 commits, unpushed).

## What shipped

Taint is now a fold over the typed support relation, not a tree walk. A node's
**support component** is computed once per derived-state read by
`support.Prepare(support.ResultUseEdgesFromNodes(nodes))` + `support.Walk`, the
same prepared result-use graph D4 (`support.Current`) folds, so reference and
validation dependencies carry taint exactly like children. The ancestor-context
component is unchanged and stays separate.

Additionally:

- `PropagateTaint` applies and returns every changed node, so reverse
  dependents (nodes that cite the changed node transitively) are updated, and
  the `TaintRecomputed` audit-emission filter matches.
- `af taint-trace` walks the support relation, names the edge kind and the
  source's verdict/revision sequence.
- An independent recursive specification plus a seeded differential fuzz over
  3000 random graphs.
- `scripts/corpus-taint-diff.sh` diffs `af export --graph json` between two
  binaries over the 211-workspace AISM corpus.
- Docs and the `0.1.11` (Unreleased) changelog entry.

## The rule list (replaces concepts.md rules 0–7)

Per-node precedence: **own severed → own admitted → unresolved → tainted →
clean**.

0. archived/refuted → `clean` (severed).
1. own `admitted` → `self_admitted` (own support ignored).
2. own pending/draft/needs_refinement → `unresolved`.
3. non-severed ancestor pending/draft/needs_refinement → `unresolved`.
4. support component over result-use targets:
   - severed child → nothing (not a result-use edge);
   - admitted target → `tainted`, not descended;
   - pending/draft/needs_refinement target → `unresolved`;
   - severed or missing dependency/validation target → `unresolved`;
   - validated target → its own support component;
   - legacy result-use SCC → `unresolved`;
   - `local_assume` *cited as a dependency* (hypothesis-use) → nothing; a
     `local_assume` **child**, and a `local_assume`'s own children, are
     ordinary result-use edges (v3.2 amendment, see the review-fix section).
   Any `unresolved` target wins; else any `tainted` target.
5. non-severed ancestor admitted → `tainted`.
6. otherwise `clean`.

External references stay outside the lattice. The ancestor component is never
pushed into siblings, so a validated sibling of an admitted node stays `clean`.

## Files

| File | Change |
|------|--------|
| `internal/taint/support_fold.go` | `supportValue`, `foldSupport`, `supportComponents` over `support.Prepare`/`Walk` |
| `internal/taint/propagate.go` | replace the subtree walk with the support fold; new `finalTaint` precedence; `PropagateTaint` updates every changed node |
| `internal/support/support.go` | `State` interface (break the import cycle), `ResultUseEdgesFromNodes`, `Provider.EdgesFrom` |
| `internal/support/{check,current}.go` | accept the `State` interface |
| `internal/state/{state,replay}.go` | `LatestAmendmentSeq`; `Replay` is pure event sourcing |
| `internal/service/{proof,exports}.go` | apply the one authoritative `taint.RecomputeAll` on load and in `Replay`/`ReplayWithVerify` |
| `internal/taint/spec_test.go` | independent recursive spec (own graph, own SCC reachability) |
| `internal/taint/support_fold_fuzz_test.go` | 3000-case seeded differential fuzz |
| `internal/taint/support_fold_test.go` | explicit dependency/severed/missing/hypothesis/cycle/sibling tests |
| `cmd/af/taint_trace.go` | support-path trace with edge kinds and revision sequences |
| `cmd/af/changelog.go` | 0.1.11 Unreleased entry |
| `scripts/corpus-taint-diff.sh` | corpus taint diff (old vs new binary) |
| `docs/{concepts,trust-model,cli-reference,state-machines,architecture}.md` | new rule list, closed gap, trace/state docs |

## Import-cycle decision (deviation from a literal reading)

`state` imported `taint`; `taint` now imports `support`; the plan puts
`support`'s walk above `state`. To avoid `state → taint → support → state`, the
support package no longer imports `state` (it takes a four-method `support.State`
interface satisfied by `*state.State`), and `state.Replay` no longer imports
`taint`. The single authoritative `taint.RecomputeAll` is applied by the
service load/replay entry points (`ProofService.LoadState`, `service.Replay`,
`service.ReplayWithVerify`) and by `af recompute-taint`, so every production
caller observes the same derived taint and the same single derivation. A direct
`state.Replay` caller must now call `taint.RecomputeAll` itself; `internal/audit`
already does, and the state tests were updated.

## Verification

### Independent spec + differential fuzz

`internal/taint/spec_test.go` is a from-the-rule-list recursive definition over
its own `specGraph` (no shared helper with the production fold); it computes
same-SCC targets by independent mutual reachability. `support_fold_fuzz_test.go`
runs 3000 seeded graphs of 5–40 nodes with random tree shape, mixed
child/reference/validation edges, `local_assume` types, missing targets, legacy
cycles and random epistemic states (the outcomes of accept/admit/unadmit/
refute/archive/amend/reopen), and asserts:

1. production `RecomputeAll` == spec for every node;
2. `RecomputeAll` is idempotent;
3. incremental `PropagateTaint` from a random root, starting from fully stale
   stored taint, equals the full derivation;
4. no `clean` node has a support path through a non-validated result
   (missing/severed/admitted/pending/refused targets are excluded by the rule);
5. a validated sibling of an admitted node with a clean ancestor chain and no
   other results is not tainted.

A separate ledger-backed test
(`internal/state/replay_taint_test.go::TestReplayTaint_IncrementalEqualsAuthoritative`)
applies a real event sequence one event at a time, propagating taint
incrementally after each, and asserts it converges to a full `Replay` plus the
authoritative pass on a graph whose admitted node is cited through a reference
dependency.

Mismatches report the minimal graph (seed, nodes, types, states, edges). The
suite also includes explicit cases for dependency-vs-child equality, severed
and missing dependencies, hypothesis-use, legacy cycles and sibling separation.

### Corpus diff

`scripts/corpus-taint-diff.sh /tmp/af-main/af /tmp/af-d6/af` (old binary from a
detached `main` worktree at `f55e477`, new binary from this branch) over the 211
manifest workspaces:

| Metric | Value |
|--------|-------|
| Workspaces compared | 211 |
| Workspaces with a changed `taint_state` | **0** |
| Nodes with a changed `taint_state` | **0** |
| Old taint distribution (2883 nodes) | `clean` 2865, `unresolved` 18 |
| Export fields allowed to differ | `taint_state`, `validation.taint_counts` |

Every per-workspace line is `changed=0` (script exit 0); the full output is the
script's stdout. **Interpretation:** the corpus contains no node with
`self_admitted` or `tainted` taint and no reference or validation edge whose
target is non-clean (verified independently across all 211 workspaces: 0
edges into a non-clean node). The 18 `unresolved` nodes are self-pending and
have no dependents, so the new dependency edge rule changes no derived value.
This is the intended behaviour and not a sign the fold is inert: the fuzz and
unit tests pin the dependency cases the corpus does not exercise, and a corpus
seeded with an admitted or pending lemma cited by a validated node does change
(as reported in the changelog entry).

### Gates

`go build ./...`, `go vet ./...`, `go test ./...`, and `gofmt -l` are green.
The fuzz adds ~1.5 s to `internal/taint`.

## Notes and follow-ups

- `docs/prd.md` still describes the pre-D6 taint model; it is a historical PRD
  and was left untouched. `docs/architecture.md`, `docs/state-machines.md` and
  both `docs/cli-reference.md` taint tables were updated for consistency.
- `af taint-trace`'s JSON shape is additive (`support_sources` replaces the
  old `descendant_sources`; `trace` entries no longer carry per-entry
  descendant lists). Human-readable reason text changed; scripts matching it
  should update, as with 0.1.7.
- ~~No ledger-based command-sequence fuzz was added~~ — added in the review-fix
  pass below (`internal/taint/ledger_fuzz_test.go`); the original reasoning was
  wrong, because nothing else exercised `state.Apply` ordering or stale
  `TaintRecomputed` audit events replayed over derived taint.

## Handoff

Unpushed: `git push` the branch and let CI run the corpus check. 0.1.11 is not
tagged; the changelog entry is `Unreleased: true` alongside 0.1.10.

## Review fixes (2026-09-17)

Opus review of this branch (`review-d6-opus.md`), fixed here. One blocker, four
should-fixes, three nits. Gates after every commit: `go build ./cmd/af`,
`go vet ./...`, `go test ./...`, plus `scripts/corpus-check.sh ./af` (211
workspaces OK) and `scripts/corpus-taint-diff.sh` against 0.1.10 (still
0/211 workspaces, 0 nodes changed).

1. **BLOCKER — a `local_assume` subtree contributed nothing.** Clause (i) of the
   support relation excluded `local_assume` in both directions, which
   disconnected a whole hypothesis subtree from the fold: an `af admit` under a
   hypothesis left the enclosing proof `validated` / `clean`, a regression
   against 0.1.10 reachable in eight CLI calls. Clause (i) is now "`t` is a
   child of `n`" with no exclusion; only a `local_assume` cited as a
   *dependency* (clause (ii)) is hypothesis-use and carries nothing, and a
   `local_assume`'s own epistemic state folds like any other node's. The plan
   carries a "v3.2 amendment (2026-09-17)" section saying so.

   *Differential test, before and after.* The spec (`internal/taint/spec_test.go`
   `specGraph.targets`) and the fuzz safety property were rewritten from the
   amended plan text **first**, against unfixed production:
   `go test ./internal/taint/ -run TestDifferentialFuzz_SupportTaintRules`
   FAILED — `seed 1: production != spec: 1.1.1 production=clean spec=unresolved`
   (a validated `local_assume` at 1.1.1 with a draft child). After the
   production change in `internal/support/support.go` the same command passes,
   as do the 3000 in-memory cases and the new ledger-driven cases. The
   ledger-level regression test for the reviewer's exact eight-command scenario
   (`internal/service.TestTaint_AdmittedStepUnderHypothesisTaintsRoot`) fails on
   the pre-fix fold with `node 1 taint = clean, want tainted` / `node 1.1 taint
   = clean, want tainted` and passes after; the CLI now reports the 0.1.10
   numbers for that workspace (`1 clean, 1 self_admitted, 2 tainted`).

   *Consequence for D4.* The relation is shared, so `support_current` sees the
   same edges: a pending `local_assume` child (or a pending step under one) is
   `TARGET_PENDING` for its parent, and a refuted `local_assume` child is
   `TARGET_REFUTED`. `internal/support/current.go` no longer skips
   `local_assume` on either side, and the changelog says so.

2. **Refuted child.** Kept as-is by decision: a refuted child stays severed and
   contributes nothing; the parent's stale verdict is D4's job
   (`support_current` = `TARGET_REFUTED`, `af audit` = `SUPPORT_NOT_CURRENT`).
   Now documented (`docs/concepts.md` rule 4, `docs/trust-model.md` "What taint
   does not say", plan v3.2 §3) and pinned by
   `taint.TestSupportFold_RefutedChildContributesNothing` and
   `service.TestRefutedChild_TaintCleanButSupportNotCurrent`, which asserts the
   two signals disagreeing on purpose.

3. **Child edges vs. missing intermediate ancestors.** The support graph indexed
   children by the literal `ID.Parent()` while the ancestor pass used
   `nearestExistingParent`, so a node under a missing ancestor was a descendant
   but nobody's child. `universe.effParent` now applies the same
   nearest-present-ancestor rule, and one generated graph in four deletes a
   non-root, non-leaf node so ID holes are fuzzed (the variant fails against the
   old literal-parent map at seed 44).

4. **`state.Replay` taint contract.** `Replay`, `ReplayWithVerify`,
   `replayInternal` and `Apply` now state that replay does not derive taint,
   that `taint.RecomputeAll` is the caller's obligation, and which three
   production callers discharge it. The old `Apply` comment asserted the
   opposite. No rename.

5. **`PropagateTaint`.** Now a documented thin wrapper over `RecomputeAll`
   (`root` is only a nil guard and the caller's event scope); there is no
   affected set, and `docs/concepts.md`, `docs/state-machines.md`, the changelog
   and the service comments no longer claim one. The vacuous fuzz property 3
   (full recompute vs full recompute) is replaced by
   `TestDifferentialFuzz_LedgerCommandSequences`: random command sequences
   (accept, admit, refute, archive, request-refinement, amend, amend-deps
   including `--reopen`, unvalidate, stale `taint_recomputed` events) are
   applied to a real ledger through production `state.Apply`, then
   `state.Replay` + `RecomputeAll` is compared against the independent spec.
   This also fills item 9's missing "command sequences" coverage; it catches the
   item 1 blocker independently (seed 4).

6. **Changelog.** 0.1.11 now names the per-node precedence flip (own severed →
   own admitted → own unresolved → ancestor unresolved; an `admitted` node under
   a `pending` ancestor reports `self_admitted` where 0.1.10 reported
   `unresolved`), the amended clause (i) and its `support_current` consequence,
   and the nearest-present-ancestor rule.

7. **Constant factor.** Adjacency lists are deduplicated and sorted once in
   `resultUseEdges` and consumed as-is by Prepare and Walk (`sortedDeps` is
   gone). Measured at 1000 nodes this is within run-to-run noise
   (3.4 ms → 3.4 ms plain, 4.4 ms → 4.2 ms with one cross-dependency per node):
   the constant is dominated by the per-node maps, not the sorts. `tarjan`'s
   `strongConnect` is now iterative (explicit frame stack), so a deep legacy
   chain cannot exhaust the goroutine stack. The expensive half — the per-node
   `LoadState` + recompute inside the bulk accept / `verdicts apply` loops,
   ~0.9 s of fold for a 100-node bulk accept on a 1000-node proof — is **not**
   changed here; filed as **vibefeld-e8td** (P2) with the measured numbers.

8. **`af taint-trace`.** (a) A node unresolved because of a legacy result-use
   SCC used to print an empty "Support source(s):" block; cyclic components are
   indexed and the source is named, `1.1 — unresolved via cycle 1.1 -> 1.2 ->
   1.1` (`cmd/af.TestTaintTraceCmd_NamesLegacyCycle`). (b) Each line's verb is
   the component the source contributes (`unresolved via` for a pending,
   reopened or severed source, `tainted via` for an admitted one) instead of a
   hardcoded "tainted via"; JSON sources gain `contributes` and `cycle`.
   (c) `docs/cli-reference.md` and the changelog now show output pasted from a
   real run, including the leading source id and `, taint <state>`.

9. **Leftovers.** The unused `latestRevisionSeq` wrapper is deleted; the orphan
   "active descendant" paragraph in `docs/concepts.md` is rewritten into the
   boundary-node rule it was meant to state; the redundant `taint.RecomputeAll`
   in `cmd/af/taint_trace.go` is removed (verified: `LoadState` already derives
   taint) and its comment corrected.
