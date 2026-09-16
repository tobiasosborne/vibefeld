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

`Run(st *state.State, opts Options) Report` takes one derived-state read. The
plan's "ledger only where a sequence is needed" turned out to be unnecessary:
every sequence the findings use is already derived state (`node.VerdictSeq`,
`state.Amendment.Seq`, `state.Challenge.Seq`), so no ledger reader was threaded
in. D3's "reconstructed" hash comparison is represented as a historical
`HASH_MISMATCH` finding rather than replaying to the acceptance sequence; the
brief explicitly permits this ("when not recorded, mark status historical with
a 'reconstructed' note and never strict"). `Report{SchemaVersion:1, Findings,
Summary, Strict, Passed}`; the summary is computed after filtering but before
the output limit, so counts are never truncated. Output is filterable by
`--code`, dotted-segment-aware `--node <prefix>` and `--status`, and bounded by
`--limit` (default 200, `0` = unlimited at the CLI).

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
verify + export + non-strict audit all exit 0). Audit over the 211 workspaces
found 3836 findings: 105 current (all strict-current) and 3731 historical; 19
workspaces contain at least one strict-current finding. Findings by code:

| Code | Status | Count |
|------|--------|-------|
| `HASH_MISMATCH` (unrecorded) | historical | 2812 |
| `UNKNOWN_PROVENANCE` | historical | 593 |
| `AMENDMENTS_PER_NODE` | historical | 324 |
| `SUPPORT_NOT_CURRENT` | current | 102 |
| `VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE` | current | 3 |
| `PENDING_EXTERNAL_CITED_BY_VALIDATED` | historical | 2 |
| `ADMITTED`, `ARCHIVED_WITH_OPEN_CHALLENGE`, `CYCLE`, `SCOPE_LEAK`, `CITES_SEVERED`, `AMENDED_NOT_REVERIFIED`, `SELF_ACCEPT` | — | 0 |

`SUPPORT_NOT_CURRENT` causes: `TARGET_NOT_CURRENT` 66, `TARGET_REVISED` 33,
`OPEN_BLOCKING_CHALLENGE` 3. The historical `HASH_MISMATCH` and
`UNKNOWN_PROVENANCE` volumes are the expected pre-D3 corpus (validated before
content hashes and verifier identities were recorded); they never gate, which
is why the non-strict corpus check passes.

## Notes / known limits

- `ARCHIVED_WITH_OPEN_CHALLENGE` is derived from current state because the
  D9 obligations helper is not on `main`/this branch. It therefore means "an
  open challenge exists now on the archived node or an active descendant",
  which can differ from the challenge set *at archival time*. It is historical
  and non-gating; when D9 lands, the helper can sharpen it.
- `PENDING_EXTERNAL_CITED_BY_VALIDATED` treats every cited external as pending,
  matching `af pending-refs` (external verification is not implemented).
- No ledger reader was needed; if a future audit wants to reconstruct the
  content hash at an acceptance sequence, that is the one place to thread one
  through `Run`.
