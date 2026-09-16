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
