# REPORT-D2 — `amend-deps`: append-only edge correction, one event, batchable

Plan item D2 of `docs/plans/scale-hardening.md` (v3.1; the amendments leave D2
unchanged). Bead `vibefeld-4hut`. Answers **GitHub issue #3** (21 validated
nodes with wrong dependency edges in a 123-node proof).

Branch: `work/d2-amend-deps` (5 commits, nothing pushed). `docs/plans` was not
touched. Gates: `gofmt` clean · `go build ./cmd/af` · `go vet ./...` ·
`go test ./...` all pass.

## What landed

### Event (`internal/ledger/event.go`)
- New `EventNodeDepsAmended = "node_deps_amended"`, registered in `init()` with
  `RegisterEventMinFormat(..., "1.1")`, so a 1.0 workspace refuses it with
  `FORMAT_TOO_NEW` and the `af workspace upgrade --to 1.1` hint.
- `NodeDepsAmended{node_id, previous_dependencies, new_dependencies,
  previous_validation_deps, new_validation_deps, owner, reason,
  previous_content_hash, reopened}` plus `BaseEvent.OperationID`.
- `NodeAmended` gains the optional `reopened` field.

### Replay / state (`internal/state`)
- `applyNodeDepsAmended`: verifies the event's previous lists (and, when
  recorded, the previous content hash) against held state — a mismatch is a
  replay error and never an overwrite — then replaces both lists, recomputes
  the content hash, and appends a dependency amendment.
- `reopenValidated`/`clearValidationFields` are the single definition of the
  `validated -> pending` transition, shared by `NodeUnvalidated` and both
  amendment reopen paths. `node_amended` applies its optional `reopened` too.
- `state.Amendment` gains a `Kind` discriminator (`statement`/`dependencies`),
  dependency fields, and a `Seq`. Replay stamps the event sequence onto the
  just-appended amendment (no `Apply` signature change).

### Service (`internal/service/amend_deps.go`, `amend_deps_manifest.go`)
- `AmendDeps(nodeID, AmendDepsRequest) (AmendDepsResult, error)`. One commit
  closure, preconditions in the mandated order: node exists, operation id
  already committed (short-circuit `applied-already`), epistemic state, claim
  ownership, `--expect-hash`, every target exists, contradictions, change set,
  then `support.CheckCreation` over the prospective node. Exactly one
  `node_deps_amended` event. `Reopen` and the edge replacement are one event.
- `AmendDepsRequest{Add, Remove, AddValidated, RemoveValidated, Reopen,
  ExpectHash, Strict, Owner, Reason, OperationID}`.
- `AmendDepsResult{Outcome, Seq, OldHash, NewHash, Reopened, Added, Removed,
  AddedValidated, RemovedValidated, Reverify}`.
- Manifest: `ParseAmendDepsManifest` (unknown fields rejected, schema
  version 1), `DryRunAmendDepsManifest` (no writes, assigns operation ids,
  per-item diff/hash/rejection path), `ApplyAmendDepsManifest` (file order, one
  commit per item, `applied | applied(already) | unchanged | rejected:<code> |
  blocked:batch-aborted`, every item reported once, a concurrent-modification
  error aborts the rest, re-verification work list). Exit codes mirror
  `verdicts apply`: 5 partial, 6 none applied, 7 all unchanged.

### CLI (`cmd/af/amend_deps.go`, `amend.go`)
- `af amend-deps <node-id> [...]` and `af amend-deps --file ... [--dry-run]
  [--resume]`; help states the policy (states, reopen, one event, hash change
  invalidates stale verdicts). `--dry-run` is marked supported (it is only
  accepted with `--file`).
- `af amend` gains `--reopen` and accepts `needs_refinement`.

### Surfaces
- `af get`: dependency amendment count + last and `validation_deps` (text and
  JSON), and kind-aware amendment history.
- `af amendments`: both kinds in ledger order; statement `vN` numbering is
  unchanged (dependency amendments are a separate list in JSON).
- `af deps`: edges touched by an amendment marked `*` with a legend.
- `af diff`: dependency changes included, including `--since-challenge`.
- `af export --graph json`: per-node `validation_deps` and
  `dependency_amendments` under new `validation-deps` /
  `dependency-amendments` capability tokens (`omitempty`, no schema bump).
- `af log`: renders `node_deps_amended` (and reopened `node_amended`).
- `resolve-challenge` help points at `amend-deps`.
- Docs: `docs/cli-reference.md`, `docs/concepts.md` "Correcting dependencies",
  `docs/amend-deps-example.md` (three nodes, one wrong edge, dry-run → run →
  verdicts apply), 0.1.9 changelog entry.

### Tests (no build tags)
- `internal/ledger/amend_deps_event_test.go`: format 1.1, JSON shape,
  additivity/omission.
- `internal/state/amend_deps_apply_test.go`: replace + hash + history, previous
  mismatch is a replay error, reopen clears validation (both event types).
- `internal/service/amend_deps_test.go`: add/remove ref + validation, reopen is
  one event (replayed state is pending with new edges and a new hash), stale
  `expect-hash` refused even when the edges already exist, operation-id retry
  returns `applied-already` and appends nothing, unchanged appends nothing,
  strict no-op, contradiction, state preconditions (validated without reopen,
  admitted → `af unadmit`), cycle with path, scope leak, claim ownership,
  format gate on a 1.0 workspace, dry-run writes no events but persists ids,
  every manifest item reported once, crash-abort at item k + `--resume`
  produces the same ordered amendment events as an uninterrupted run.
- `internal/export/graph_deps_test.go`: export fields and capability tokens.
- `cmd/af/amend_deps_test.go`: single, `--reopen`, manifest dry-run (writes ids
  back) then real run, `af deps` legend, `af amendments` JSON.

## Decisions on ambiguity (closest to the plan/conventions)

1. **`node_amended` stays format 1.0.** The brief registers only
   `node_deps_amended` as a 1.1 event ("Event `node_deps_amended` (format 1.1,
   registered in init)"). The plan also says "format activation before the
   first new event type". A 0.1.8 reader would silently ignore the new
   `reopened` field on a `node_amended`; this is the literal reading and the
   format-gate test targets `amend-deps` only. Flagged as a candidate for a
   follow-up if a 1.0 reader must never misread a reopen.
2. **One `state.Amendment` type with a `Kind` discriminator** rather than a
   separate `DepsAmendment` type and a union slice: it preserves the existing
   `GetAmendmentHistory` signature and every statement-version consumer, and
   lets `af amendments` list both kinds in ledger order. Empty `Kind` is
   treated as `statement`, so legacy records replay unchanged.
3. **Amendment `Seq`** is stamped by replay after `Apply` rather than carrying
   it on the event or changing `Apply`'s signature. Export reports it; direct
   `Apply` callers see 0.
4. **Taint audit events.** `amend-deps --reopen` does not emit separate
   `taint_recomputed` audit events (unlike `af unvalidate`); replay re-derives
   taint authoritatively, and the brief requires exactly one amendment event.
5. **`admitted` rejection** is `INVALID_STATE` (exit 3) with the message "run
   `af unadmit` first", not a blocked (exit 2) tier, matching the brief's
   "exit 3 for rejections".
6. **Claim mismatch** uses `ErrOwnerMismatch` (`NOT_CLAIM_HOLDER`, exit 1), so
   a caller can retry after claiming; this matches `af amend`'s ownership rule
   ("as af amend enforces it today").
7. **Target existence** applies to remove lists too (the brief says "every
   target exists"), so a dangling legacy edge cannot be removed through
   `amend-deps`; removing such an edge would need a separate allowance.
8. **`--dry-run` exits 0** regardless of per-item rejections (it is a preview);
   the 5/6/7 tiers describe the real run.
9. **`checkSupportBatch`** (D1's named hook) is used unchanged with a single
   `ProspectiveNode` whose edge lists replace the node's.

## Follow-ups

- If a 1.0 reader must be fenced from `node_amended` reopen semantics, register
  a format floor for a reopened `node_amended` (needs a field-sensitive gate or
  a new event type).
- Consider allowing removal of a dangling dependency edge without requiring the
  missing target to exist (D1 already keeps such edges as sinks).
- The D4 `support_current`/audit work (0.1.10) should consume
  `dependency_amendments` to flag `AMENDED_NOT_REVERIFIED`.

## Review fixes

A follow-up review found eleven gaps; all are fixed on `work/d2-amend-deps`,
TDD, with the gates green. Four commits: `6cfa264`, `ca80627`, `287588b`,
`a75bfb9` (the fixture). Each intermediate commit builds (`go build ./...`).

1. **Replay state contract for `node_deps_amended`** (`internal/state/apply.go`).
   `applyNodeDepsAmended` now validates the state/reopen contract before any
   mutation: a non-reopened event on a validated node is a replay error (a
   content change on a validated node must reopen), and admitted/refuted/archived
   nodes are always refused. Every previous list, the previous hash and the
   reopen transition are checked before the edge replacement, hash recompute,
   reopen and amendment record are applied, so a failed `Apply` leaves state
   untouched. Tests: validated-without-reopen, terminal states, reopen-from-pending
   all assert no mutation, plus the existing mismatch tests.

2. **Operation-id binding** (`internal/state`, `internal/service/amend_deps.go`).
   `State` now maps `operation_id -> OperationRecord{seq, event type, node,
   request fingerprint, old/new hash, reopened}`. `AmendDeps` stores a canonical
   SHA-256 fingerprint (node, reopen, four sorted edge lists) in the new optional
   `request_fingerprint` event field. On an id hit, a match returns
   `applied-already` with the original event's old/new hash and reopen flag; a
   node/type/fingerprint mismatch returns the typed
   `ErrAmendDepsOperationIDConflict` (new `OPERATION_ID_CONFLICT` code, exit 3,
   manifest status `rejected:OPERATION_ID_CONFLICT`). Tests: same id + same
   request reports the original hashes; different node and different change set
   both conflict.

3. **Dry run evolves its state** (`internal/service/amend_deps_manifest.go`).
   Each successfully planned event is applied to the fresh replay state before
   the next item is planned (and its operation id recorded), so hashes, no-op
   decisions and cycle/scope verdicts match the real run. The reciprocal-edge
   dry-run case (`1.1 -> 1.2`, then `1.2 -> 1.1`) is asserted as
   `rejected:DEPENDENCY_CYCLE`, and a new dry-run/real-run parity test compares
   per-item status, hashes and edge diff.

4. **Operation ids persisted before the first commit** (`cmd/af/amend_deps.go`).
   A real manifest run calls `EnsureOperationIDs`, and if any id was generated it
   rewrites the manifest atomically (temp file + `rename`) before applying
   anything. `--resume` reads the ids back from disk. Test:
   `TestAmendDepsCmd_PersistsOperationIDsBeforeFirstCommit` inspects the file
   inside the pre-append hook, crashes after item 0, resumes from the on-disk
   file and asserts item 0 is `applied(already)`.

5. **Restart-from-disk test** (`internal/service/amend_deps_test.go`). The old
   in-memory crash test is replaced by `TestAmendDepsManifest_RestartFromDisk`,
   which writes the manifest to disk, crashes before item 1's append, re-reads
   the file, resumes, and compares the full event stream (operation id, owner,
   reason, previous hash, both edge lists) with an uninterrupted run. It includes
   a strict item and a stale-expect-hash rejection.

6. **Manifest exit tiers** (`exitError`). All-unchanged is evaluated before the
   generic success (exit 7 now happens), zero applied is none-applied (6),
   applied-plus-rejected is partial (5), and an empty manifest is none-applied
   rather than silently succeeding. `TestAmendDepsManifest_ExitCodes` covers
   each tier.

7. **Diff dependency interval** (`cmd/af/diff.go`, `internal/state`). Each
   dependency change keeps its ledger `Seq`; `af diff` filters dependency changes
   to `(seq(fromVersion), seq(toVersion)]`, and `--since-challenge` filters by the
   challenge's ledger sequence (now stamped in replay) instead of timestamps.
   Tests assert that the default/`--version 1` diff omits a dependency amendment
   recorded after the last statement change, `--version 0` includes both, and
   `--since-challenge` returns only changes after the challenge.

8. **`af deps` per-kind symmetric differences** (`cmd/af/deps.go`). Only edges an
   amendment actually added keep the `*` mark; removed edges are reported on a
   separate `(-) removed by amendment:` line (with a `v:` prefix for validation
   deps), applied in ledger order so a re-add cancels an earlier removal. Test:
   removing one of two added edges leaves the survivor marked and lists the
   removed edge.

9. **Distinct reopened statement event** (`internal/ledger/event.go`, state,
   service). `NodeAmended` no longer has a `reopened` field; the new
   `node_amended_reopened` event (same fields, always reopens) is registered
   format 1.1. `af amend --reopen` emits it and replay applies it through the
   shared `reopenValidated` helper. Tests: the format gate refuses
   `AmendNodeWithReopen` on a 1.0 workspace, the legacy `node_amended` still
   works and does not reopen, and the ledger JSON shape/min-format are asserted.

10. **Removing dangling edges** (`internal/service/amend_deps.go`).
    `checkAmendDepsTargets` checks ADD lists only; a remove of a present edge is
    always allowed. Test writes a dangling edge directly to the ledger (`1.1 ->
    1.99`) and removes it through `AmendDeps`.

11. **1.1 e2e fixture** (`e2e/fixtures/format-1.1/ledger/000006.json`). A real
    `node_deps_amended` event was appended; `state.ReplayWithVerify` passes. The
    regeneration path (`AF_GEN_FIXTURES=1`) now emits it for the 1.1 fixture, and
    `TestFormatFixture_PreviousBinary` asserts the old binary's error text names
    `node_deps_amended` when `AF_PREVIOUS_BINARY` is set.

### Note on the conflict status name

The review asked for `rejected:operation-id-conflict`. The manifest status is
`rejected:<CODE>` from `ErrorCode.String()`, and every existing code is
upper-snake, so the new code is `OPERATION_ID_CONFLICT` (exit 3). The typed
sentinel is `service.ErrAmendDepsOperationIDConflict`, so callers can match it
regardless of the rendered string.

### Gates after the fixes

`gofmt` clean · `go build ./cmd/af` · `go vet ./...` · `go test -count=1 ./...`
all pass.
