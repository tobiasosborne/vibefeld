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
   - `local_assume` target (hypothesis-use) → nothing.
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
- No ledger-based command-sequence fuzz was added: the differential fuzz
  samples the command outcomes (epistemic states and edges) directly, and the
  independent spec is the oracle. A ledger round-trip fuzz would duplicate
  `state.Replay` coverage without adding taint semantics.

## Handoff

Unpushed: `git push` the branch and let CI run the corpus check. 0.1.11 is not
tagged; the changelog entry is `Unreleased: true` alongside 0.1.10.
