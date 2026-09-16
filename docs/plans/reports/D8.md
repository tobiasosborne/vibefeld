# REPORT-D8 — `af audit`, one findings engine, migration preflight/postcheck

Bead: `vibefeld-fza2` · branch `work/d8-audit` · plan `docs/plans/scale-hardening.md`
§ D8 (v3.1: strict subset + migration preflight/postcheck in 0.1.10; historical
classes included now because they were cheap).

## What shipped

A read-only audit engine (`internal/audit`) and an `af audit` command, wired as
the single findings engine for `af health` and the `af amend-deps` manifest
migration, plus a corpus-check hook and docs.

### Files

| File | Change |
|------|--------|
| `internal/audit/codes.go` | Stable finding codes, strict-current set, severities/statuses, remediations |
| `internal/audit/audit.go` | `Finding`/`Summary`/`Report`/`Options`, `Run`, filters, limit, summary |
| `internal/audit/checks.go` | The ordered pass: one producer per code |
| `internal/audit/audit_test.go` | Positive + negative unit test per code, strict, filters, limit, schema |
| `internal/support/check.go` | `ScopeLeaks(st)` (reuses `checkScope` over the existing graph) |
| `internal/support/walk.go` | `Graph.CyclicComponents()` (reuses the prepared Tarjan SCCs) |
| `internal/errors/errors.go` | `AUDIT_FAILED` (exit 3) |
| `cmd/af/audit.go` | `af audit [--strict] [-f text\|json] [--code] [--node] [--status] [--limit]` |
| `cmd/af/audit_test.go` | CLI JSON schema, strict exit 3, filters/limit, fixture smoke |
| `cmd/af/health_support.go` | Reads `SUPPORT_NOT_CURRENT` from `internal/audit` |
| `cmd/af/amend_deps.go` | One-line audit preflight/postcheck summary |
| `scripts/corpus-check.sh` | Runs `af audit -f json` (non-strict) per workspace |
| `cmd/af/changelog.go`, `docs/cli-reference.md`, `docs/trust-model.md` | 0.1.10 notes, code table, audit pointers |

### Engine contract

`Run(st *state.State, opts Options) Report` takes one derived-state read;
`RunWithPass(st, pass, opts)` adds an ordered ledger pass. The sequence-
sensitive checks (accepted content hash, acceptance-time contributors, open
challenges at an archival) use that pass, built once by
`audit.BuildPass(*ledger.Ledger)`; the rest read one immutable snapshot of
derived state, so no state getter populates a cache mid-run and two audits of
the same workspace are byte-identical. `Run` remains the state-only entry point
(health and the unit tests); when no pass is available a `HASH_MISMATCH`
without a recorded hash is reported as a historical `reconstructed` note.
`Report{SchemaVersion:1, Findings, Summary, Strict, Passed}`; the summary is
computed after filtering but before the output limit, so counts are never
truncated. Output is filterable by `--code`, dotted-segment-aware
`--node <prefix>` and `--status`, and bounded by `--limit` (default 200, `0` =
unlimited at the CLI).

### Codes

| Code | Status | Severity | Strict? |
|------|--------|----------|---------|
| `SUPPORT_NOT_CURRENT` | current | error | yes |
| `HASH_MISMATCH` (recorded mismatch) | current | error | yes |
| `CYCLE` | current | error | yes |
| `SCOPE_LEAK` | current | error | yes |
| `CITES_SEVERED` | current | error | yes |
| `AMENDED_NOT_REVERIFIED` | current | error | yes |
| `SELF_ACCEPT` | current | error | yes (only when identities recorded) |
| `VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE` | current | error | yes |
| `HASH_MISMATCH` (unrecorded) | historical | warning | no |
| `ADMITTED` | historical | info | no |
| `ARCHIVED_WITH_OPEN_CHALLENGE` | historical | warning | no |
| `AMENDMENTS_PER_NODE` | historical | info | no (top 10 hotspots) |
| `PENDING_EXTERNAL_CITED_BY_VALIDATED` | historical | warning | no |
| `UNKNOWN_PROVENANCE` | historical | info | no |

`SUPPORT_NOT_CURRENT` carries the `cause` and the responsible node as a second
`nodes` entry; `AMENDED_NOT_REVERIFIED` detects both a validated node amended
after its own verdict and a validated consumer of a target amended after the
target's verdict. `CYCLE` is one finding per result-use SCC (reusing the graph
`support.Prepare` already builds for `support_current`); `SCOPE_LEAK` reuses
`support.checkScope`; `CITES_SEVERED` reuses `support.DanglingDeps`.

### CLI

`af audit` appends no events and takes no ledger lock. Non-strict always exits
0. `--strict` exits 3 with the structured `AUDIT_FAILED` code when a
strict-current finding exists; the report is still printed. Text groups by code
with counts, remediation and finding detail; JSON is the `Report`.

### One findings engine

`analyzeSupportHealth` no longer calls `support.Current` itself; it consumes
`internal/audit` `SUPPORT_NOT_CURRENT` findings (unlimited) and reproduces the
existing `support_not_current_<CAUSE>` blocker text verbatim. Existing health
tests pass unchanged. Health's informational `open_challenge` blocker section
was deliberately left alone: it reports *all* open challenges with severity and
age, whereas `VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE` is only the
validated-node subset, so replacing it would have changed health output and
broken the "keep health's output unchanged" requirement. The
`VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE` code is available to audit consumers.

### Migration preflight/postcheck

`af amend-deps --file m.json --dry-run` and the real run both end with:

```
Audit summary: N strict-current finding(s), M historical; run: af audit --strict
```

implemented by `auditSummaryLine` calling `audit.Run` through
`cmd/af/amend_deps.go`. In `-f json` mode the same line is emitted as the
trailing `audit_summary` field (via anonymous embedding of the service report),
so stdout stays one valid JSON document — the existing amend-deps JSON tests
that parse the whole buffer still pass.

## Tests

- `internal/audit`: positive and negative unit test for every one of the 13
  codes on hand-built state (including the historical/non-gating status of
  `HASH_MISMATCH` unrecorded and the identity-recorded guard on `SELF_ACCEPT`),
  plus strict pass/fail, code/status/node-prefix filters, limit semantics and
  `schema_version`.
- `cmd/af`: JSON schema, `--strict` exit 3 / `AUDIT_FAILED` and non-strict exit
  0 on the same workspace, CLI filters + limit, and a corpus smoke test that
  runs the engine over `e2e/fixtures/*` non-strict (must pass).
- `go build ./cmd/af && go vet ./... && go test ./...` green; `gofmt` clean.

## Local corpus run (AISM `../almost-idempotent-stochastic-maps/proofs`)

`scripts/corpus-check.sh ./af` → **corpus check OK (211 workspaces)** (replay
verify + export + non-strict audit all exit 0), re-run after the review fixes.
Audit over the 211 workspaces found 1046 findings: 126 current (all
strict-current) and 920 historical; 19 workspaces contain at least one
strict-current finding. Findings by code:

| Code | Status | Count |
|------|--------|-------|
| `UNKNOWN_PROVENANCE` | historical | 593 |
| `AMENDMENTS_PER_NODE` | historical | 324 |
| `SUPPORT_NOT_CURRENT` | current | 102 |
| `AMENDED_NOT_REVERIFIED` | current | 21 |
| `VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE` | current | 3 |
| `PENDING_EXTERNAL_CITED_BY_VALIDATED` | historical | 2 |
| `ARCHIVED_WITH_OPEN_CHALLENGE` | historical | 1 |
| `HASH_MISMATCH`, `ADMITTED`, `CYCLE`, `SCOPE_LEAK`, `CITES_SEVERED`, `SELF_ACCEPT` | — | 0 |

`SUPPORT_NOT_CURRENT` causes: `TARGET_NOT_CURRENT` 66, `TARGET_REVISED` 33,
`OPEN_BLOCKING_CHALLENGE` 3. The historical `UNKNOWN_PROVENANCE` volume is the
expected pre-D3 corpus (validated before verifier identities were recorded); it
never gates, which is why the non-strict corpus check passes. The 2812
historical `HASH_MISMATCH` findings recorded in the pre-fix run were audit
false positives and are gone: the ordered pass reconstructs the accepted hash
and finds it equal to the unchanged final content (see Review fixes).

## Notes / known limits

- `ARCHIVED_WITH_OPEN_CHALLENGE` now prefers the durable
  `abandoned_obligations` snapshot on the `NodeArchived` event (D9) and falls
  back to the open challenges observed at the archival sequence in the ordered
  ledger pass. The state-only compatibility path (no pass) still uses the
  current open set, but the CLI and the amend-deps summary always supply a
  pass. Historical and non-gating.
- `PENDING_EXTERNAL_CITED_BY_VALIDATED` treats every cited external as pending,
  matching `af pending-refs` (external verification is not implemented).
- `support.Current` still reads challenges through the state's cached
  challenge-by-node map; `audit` populates that cache once in `newSnapshot`
  before any producer runs, so no getter rebuilds it mid-audit.

## Review fixes (post-merge with D5/D9)

An independent review found that the sequence-sensitive codes were scanning
final state instead of replaying the ledger in order, and that the audit was
doing avoidable work. This branch was merged with `main` (D5/D9: claim
generations, `NodeArchived.abandoned_obligations`/`reason`/`forced`, derived
`ArchivedSeq`/`ClaimSeq`, `CHILD_ARCHIVED_AFTER_VERDICT`) keeping both sides of
the changelog/trust-model conflicts, and the audit was rebuilt on that base.
All fixes landed TDD; `go build ./cmd/af && go vet ./... && go test ./...` is
green and `gofmt` is clean.

### Ordered ledger pass

`internal/audit/replay.go` adds `BuildPass(*ledger.Ledger) (*state.State,
*Pass, error)`. It replays the ledger event by event through
`state.ParseEvent` + `state.Apply` + `state.StampDerived` (the same priming
replay uses; the two new state entry points are thin exported wrappers around
the existing parser and stamping switch). Before each event it:

- on `NodeValidated`/`NodeAdmitted` records the acceptance sequence, the
  node's `ComputeContentHash()` at that moment, the contributors so far (author,
  proof author at that time, and statement/dependency amendment owners up to
  that sequence), and the recorded verifier;
- on `NodeArchived` records the event's `abandoned_obligations` (D9) and the
  node plus active descendants with an open challenge at that sequence
  (observed *before* `Apply`, which auto-supersedes challenges on the archived
  node).

`Run` keeps its state-only signature; the new `RunWithPass(st, pass, opts)` is
what the CLI and the amend-deps summary call. `LoadStateWithPass` on
`ProofService` builds state and pass in a single replay and then loads on-disk
assumptions/externals, so the CLI no longer calls `Status()` and `LoadState()`
both.

### The five checks

1. **`HASH_MISMATCH`** compares the accepted hash — the recorded
   `ValidatedContentHash` when present, otherwise the hash reconstructed at the
   acceptance sequence — against a fresh `n.ComputeContentHash()` on final
   state. It emits a finding only on an actual mismatch; a recorded mismatch is
   `current`/strict, a reconstructed one `historical` and labelled
   "reconstructed". This removes the 2812 historical corpus false positives.
2. **`AMENDED_NOT_REVERIFIED`** compares a target's latest revision sequence
   (and the consumer's own) against the *consumer's* `VerdictSeq`, not the
   target's; target currentness stays with `SUPPORT_NOT_CURRENT`. This surfaces
   the 21 real consumer findings the old comparison missed.
3. **`SELF_ACCEPT`** compares the verifier with the contributors recorded up to
   that validation sequence from the pass, so a later amender no longer
   retroactively makes an earlier verdict a self-accept.
4. **`ARCHIVED_WITH_OPEN_CHALLENGE`** uses the event's
   `abandoned_obligations` when present, else the pass's open-at-archival set;
   final challenge state is not consulted when a pass is available. One
   previously invisible corpus obligation is now reported.
5. **Determinism / no mutation**: `internal/audit/snapshot.go` takes one
   immutable read at the start (nodes sorted; a parent→children index built in
   one pass; challenges sorted by node, seq, id and pre-populating the state's
   challenge cache once), and every finding sorts its cross-node/sequence
   lists. A test runs the audit twice on one workspace and compares the JSON
   byte-for-byte.

### Performance

Producers are selected before computation from `Options.Codes`; health asks
only for `SUPPORT_NOT_CURRENT` and `VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE`.
Child/descendant scans use the one-pass index. A 1000-node in-process state
audits in well under two seconds across two calls, and `ReplayCount` (a test
hook on `BuildPass`) proves `Run` itself replays zero times.

### New / changed tests

- Hash tests mutate content (statement), not the cached `ContentHash`.
- `CITES_SEVERED`: a live dependency plus an archived child is clean; an
  explicit dependency on an archived node is a finding.
- Node-prefix filter: non-empty positive and empty negative.
- Ledger-backed: reconstructed hash matches → no finding; reconstructed
  mismatch → historical; recorded mismatch → current/strict; target amended and
  re-accepted still flags its earlier consumer; a later amender is not
  `SELF_ACCEPT` for an earlier verdict; a challenge open at archival survives a
  later resolve and a descendant obligation is attributed to the archived
  ancestor, while a challenge opened after archival is not reported; recorded
  `abandoned_obligations` win.
- Determinism (double-run JSON compare) and the 1000-node performance/no-replay
  test.

### Corpus re-run

`scripts/corpus-check.sh ./af` → **corpus check OK (211 workspaces)**. The
finding-count table above is post-fix: 1046 findings (126 current, 920
historical), 19 strict-current workspaces; `HASH_MISMATCH` 2812 → 0,
`AMENDED_NOT_REVERIFIED` 0 → 21, `ARCHIVED_WITH_OPEN_CHALLENGE` 0 → 1.
