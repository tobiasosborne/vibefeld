# Batch verdict application (`af verdicts apply`, `af unvalidate --batch`)

Hand-maintained reference documentation, kept in lockstep with
`internal/verdicts/schema.go` and `internal/service/verdicts_apply.go` — a
change to either that isn't reflected here is incomplete work.

This document is what rk's M3.4 batch-verification driver needs to know to
produce valid verdict files and interpret af's response. It is the af-side
half of rk PRD C3's batched verification mode
(`../research-workflows/PRD.md` §C3, `IMPLEMENTATION_PLAN.md` items V2/V3).

## Background

Batched verification changes the dispatch unit from "one node, one fresh
verifier agent" to "one fresh hostile verifier over a set of verification-
ready items." The verifier returns a schema-validated verdict list; af's job
is to apply that list as normal per-node ledger events — never a wholesale
subtree accept — and report exactly what happened.

`af verdicts apply <file>` is the ingestion verb. `af unvalidate --batch
<id>` is its inverse: bulk revocation of everything a given batch validated.

## The verdict-file schema (`internal/verdicts`)

The schema lives in `internal/verdicts/schema.go` and is versioned
independently of rk's own `schemas/verdict.v1.json` (drafted concurrently in
the sibling `../rk` repo as of 2026-07-19). **The two are aligned in spirit
only** — same verdict vocabulary (`accept`/`challenge`), same idea of a
shared batch id, same "no blanket accepts" rule — but af owns this schema
and versions it on its own schedule (`schema_version` below). Do not assume
byte-for-byte wire compatibility between the two documents; if rk's driver
needs to emit af's exact shape, generate it from this document, not from
rk's schema.

### Top-level document

```json
{
  "schema_version": "1",
  "batch_id": "batch-2026-07-19-001",
  "verified_by": "verifier-session-42",
  "items": [ ... ]
}
```

| Field | Required | Notes |
|---|---|---|
| `schema_version` | yes | Must equal `"1"` (`internal/verdicts.CurrentSchemaVersion`). Any other value is a file-invalid parse error. |
| `batch_id` | yes | Non-empty. Recorded on every event this file produces (`NodeValidated.BatchID`, `ChallengeRaised.BatchID`). This is what `af unvalidate --batch <id>` later scans for. |
| `verified_by` | yes | Non-empty. The verifier identity, recorded on every event (`NodeValidated.VerifiedBy`, `ChallengeRaised.RaisedBy`) and checked against each node's recorded `Author` for the reviewer≠author rule (see below). Driver-supplied, not adversary-proof — see the caveat below. |
| `items` | yes | Non-empty array. **Order is significant** — see "Order-dependence" below. One verdict per node per file; a node appearing twice in the same file is a file-invalid error. |

### Items

```json
{"node": "1.2", "verdict": "accept", "reason": "Follows directly from def 3.2 and node 1.0"}
```

```json
{"node": "1.3", "verdict": "challenge", "target": "inference", "severity": "major", "reason": "Modus ponens misapplied: premises don't match"}
```

| Field | Required | Notes |
|---|---|---|
| `node` | yes | Hierarchical node id (`"1"`, `"1.2.3"`, ...). Must parse and, at apply time, must exist in the proof — a non-existent node is `rejected:node-not-found`, not a parse-time failure (existence depends on the live proof, which the file schema can't check). |
| `verdict` | yes | `"accept"` or `"challenge"`. No third option — there is no blanket-accept and no no-op. |
| `reason` | yes | **Mandatory for every item, including accepts.** This is the file-schema enforcement of PRD C3's "mandatory per-item justification (no blanket accepts)." An accept item with an empty `reason` fails `ParseFile` — the whole file is rejected before anything is attempted. For a challenge item, `reason` doubles as the challenge's reason text (the same field `af challenge --reason` requires). |
| `target` | challenge only | One of af's challenge targets (`statement`, `inference`, `context`, `dependencies`, `scope`, `gap`, `type_error`, `domain`, `completeness`). Defaults to `"statement"` if omitted. Must be empty on an accept item (a mixed item is file-invalid). |
| `severity` | challenge only | `critical`, `major`, `minor`, or `note`. Defaults to `"major"` if omitted. Must be empty on an accept item. |
| `category` | challenge only | Optional typed classification (`gap`, `missing`, `dependency`, `incorrect`, `unclear`, `other`). Must be empty on an accept item. |
| `expect_hash` | optional | The node's `content_hash` this verdict was authored against (rk B1). When present, `apply` re-reads the node under its own state read and enforces it atomically: **any item** whose node's current `content_hash` differs is `rejected:content-hash-mismatch` (the node was edited since dispatch); additionally an **accept** whose node is no longer `available` (claimed/blocked) is `rejected:not-verifier-ready`. Omit it to keep the legacy behavior (no hash/availability gate). A driver that bound its verdict to a specific hash should always set this — it is what makes the readiness re-check a kernel guarantee rather than a racy second export. |

### What makes a file invalid (exit code 3, `VERDICTS_FILE_INVALID`)

Checked entirely by `internal/verdicts.ParseFile` before any ledger IO —
nothing is attempted from a file that fails here:

- Not valid JSON, or trailing content after the JSON document.
- Unknown fields (`DisallowUnknownFields` — a typo'd field name is a schema
  violation, not a silently-ignored no-op).
- Wrong or missing `schema_version`.
- Missing or empty `batch_id` / `verified_by`.
- Empty `items`.
- Any item: missing/invalid `node` id, missing/invalid `verdict`, empty
  `reason`, an accept item carrying `target`/`severity`/`category`, an
  invalid `target`/`severity`/`category` on a challenge item, or the same
  node appearing twice in the file.
- Cannot read the file at all (missing path, permission denied) is treated
  identically — same exit code, same "nothing was attempted" guarantee.

## Order-dependence and partial-failure semantics

`af verdicts apply` applies items **in file order** and never reorders them.
This is deliberate, not an oversight: it is what makes "children before
parent" accepts meaningful. A node can only be accepted once every child is
cleared (validated, admitted, or archived) — the same rule `af accept`
already enforces. If a file lists a parent before its children, the
parent's accept item is `blocked-by:children-not-validated`, even if the
children are validated later in the same file. List children first.

A challenge raised earlier in a file can legitimately block an accept
listed later in the same file — most often collaterally, by leaving a child
pending (so the parent's accept fails with
`blocked-by:children-not-validated`), but also directly if the same node
were targeted twice (which the file schema forbids) or if a pre-existing
challenge from before the batch already blocks it
(`blocked-by:blocking-challenge`).

The verb **applies what it can**: a blocked or rejected item does not stop
the batch. Every item gets an outcome, and the ledger is never left
ambiguous about what was and wasn't tried. The **only** thing that stops the
batch early is a concurrent-modification race (some other process wrote to
the ledger mid-batch) — at that point the batch's ordering guarantee can no
longer be trusted, so every remaining item is recorded as
`blocked-by:batch-aborted` rather than attempted.

### Outcome vocabulary (per item)

Every item in the response report (`VerdictItemResult.status`) is exactly
one of:

- **`applied`** — recorded as a normal per-node `NodeValidated` (via
  `AcceptNodeWithVerifier`) or `ChallengeRaised` (via
  `RaiseChallengeWithBatch`) event, attributed to `verified_by` and
  `batch_id`. Never a wholesale subtree accept.
- **`blocked-by:<reason>`** — a legitimate future acceptance, just not yet:
  - `blocked-by:children-not-validated`
  - `blocked-by:validation-deps-not-validated`
  - `blocked-by:claim-test-required` (crux node, no passing claim-test)
  - `blocked-by:blocking-challenge` (a critical/major challenge already
    exists on this node)
  - `blocked-by:needs-refinement-no-children`
  - `blocked-by:batch-aborted` (batch stopped before this item was attempted)
- **`rejected:<reason>`** — the item itself is invalid regardless of proof
  state:
  - `rejected:node-not-found`
  - `rejected:reviewer-equals-author` (accept only — see below)
  - `rejected:content-hash-mismatch` (the item's `expect_hash` no longer
    matches the node's current `content_hash` — accept or challenge)
  - `rejected:not-verifier-ready` (accept only, with `expect_hash`: the node
    is no longer `available`, e.g. claimed or blocked, since dispatch)
  - `rejected:concurrent-modification` (also triggers the abort)
  - `rejected:apply-error` (catch-all for an unclassified kernel error)

## Reviewer ≠ author

**An accept item whose `verified_by` equals the target node's recorded
`Author` is rejected** (`rejected:reviewer-equals-author`), before the
kernel's normal accept path even runs. This applies to accepts only — PRD
C3 does not extend the rule to challenges.

Say this precisely, because it is easy to overstate: this is
**recorded-and-checkable provenance, not adversary-proof enforcement**.
Both `Author` and `verified_by` are driver-supplied strings; af never
verifies either against an external credential. If a node's `Author` was
never recorded (a legacy node from before this field existed, or a root
node created without `--author`), the check simply cannot fire — an empty
`Author` never equals a non-empty `verified_by`. The trust anchor remains
the driver's process discipline (rk's C9 orchestration layer), exactly as
it has always been for `ClaimedBy`. Do not claim more than this anywhere.

## `af unvalidate --batch <id>` (V3)

Bulk-revokes validation on every node whose *current* `ValidationBatchID`
equals `<id>`. This is a state scan (`Node.ValidationBatchID`, set by the
V1 schema work), not a ledger rescan.

Each revocation is applied as the same `NodeUnvalidated` event the
single-node `af unvalidate <node-id>` form already produces — attributed,
normal, and never a silent state rewrite. `ValidatedBy` and
`ValidationBatchID` are cleared on the node exactly as they are for a
single unvalidate.

A batch id matching no currently-validated node is a **clean no-op**, not an
error: it returns a report with `count: 0` and a distinct exit code (7), so
a driver can tell "there was nothing to revoke" apart from a real failure.
This also means calling `unvalidate --batch` twice on the same id is safe —
the second call is the clean no-op.

## Exit codes

| Code | Meaning | Where |
|---|---|---|
| 0 | Every item applied (verdicts apply) / at least one node revoked (unvalidate --batch) | both |
| 3 | `VERDICTS_FILE_INVALID` — file malformed or unreadable; nothing attempted | verdicts apply |
| 5 | `VERDICTS_PARTIALLY_APPLIED` — some items applied, some blocked/rejected | verdicts apply |
| 6 | `VERDICTS_NONE_APPLIED` — file was valid, zero items applied | verdicts apply |
| 7 | `UNVALIDATE_BATCH_NOT_FOUND` — batch id matches no validated node (clean no-op) | unvalidate --batch |
| 1 | Everything else (e.g. a genuine concurrent-modification failure partway through an unvalidate --batch) | either |

These extend af's original 1–4 exit-code taxonomy (see
`docs/cli-reference.md` and `internal/errors`); a verdict batch is neither a
plain success nor a plain error; it can partially apply, so it needs its own
tier.

## What rk's M3.4 driver must do

1. Compose a batch respecting the plan's guardrails: cap ~10 items,
   logically independent siblings only (never a chain where accepting item
   *k* biases item *k+1*), exclude any node on the critical path to the
   north-star contract (those always get per-node, cross-vendor treatment).
2. Order `items` so every accept's children (or `validation_deps`) already
   appear earlier in the same file, either as prior verdicts in this batch
   or as already-cleared state from before the batch.
3. Supply a non-empty `reason` on every item — there is no shortcut.
4. Pick a `verified_by` identity that is not the `Author` of any node being
   accepted in this batch (query the proof's authorship, e.g. via `af
   export --graph json`, before composing the batch — af rejects the
   mismatch per-item, but composing it correctly avoids burning a batch
   slot on a guaranteed rejection).
5. Read the JSON report (`af verdicts apply <file> --format json`) rather
   than parsing text; treat exit codes 0/3/5/6 as the four cases above, and
   persist `batch_id` alongside whatever promotion decision rk's registry
   makes, so a later `af unvalidate --batch <id>` can find it.
6. To revoke, call `af unvalidate --batch <id>` — do not hand-roll
   individual `af unvalidate <node>` calls; the batch form finds every
   affected node in one state scan and reports per-node.

## CLI reference

See `docs/cli-reference.md` for `af verdicts apply` and
`af unvalidate --batch` flag-level documentation.
