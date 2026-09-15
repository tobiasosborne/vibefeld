# Scale hardening plan, v3 (0.1.9 – 0.1.11)

Status: DRAFT v3, 2026-09-15. v1 and v2 were reviewed by an independent
model (`scale-hardening-review-codex.md`, `scale-hardening-rereview-codex.md`);
every factual claim v3 relies on was re-verified against HEAD c67a7e6 by
hand. Awaiting Tobias's decision. Companion to GitHub issue #3 and
`docs/trust-model.md`.

## Why

af is being run at a scale it was not exercised at: the MIP\* = RE tree
(123 nodes, 291 challenges, 12 verification rounds, 1732 events; GitHub #3),
Tobias's ~200-lemma DAG, and at least one external writer using it daily.
At that scale the pain is not read performance (a synthetic 1001-node /
3202-event workspace answers every command in under 0.25 s with full replay)
but that

1. the record cannot be corrected (dependency edges are write-once), so
   verifiers re-raise fixed findings as regressions and external verifiers
   judge against the wrong dependency set;
2. the trust signals stop meaning what they say (`validated` can hold with
   pending, unvalidated or refuted descendants; taint ignores
   cross-references; `af health` alarms on every large proof);
3. the write path is not safe under the concurrency it is now used with:
   a verdict's expected-hash check and its accept run on two different
   state reads, and multi-event operations CAS-check only their first event;
4. the gaps in `docs/trust-model.md` let a completion-seeking agent win
   without rigor.

v0.2 (trusted kernel, executable spec) is months out. Everything below ships
on the 0.1.x line. Nothing here changes an existing event shape; each item
adds an event type, an optional field, or a check. v0.2 will replace the
write protocol and may re-encode content hashes with a migration of its
own; nothing here requires reverting an event.

## Principles

- **P0 One read, one commit, one event per semantic unit.** Every mutating
  operation validates its preconditions and appends against a single state
  read, under the ledger lock, with a sequence check covering the whole
  batch. A change that must not be observed half-done (reopen + amend,
  validate + release) is *one event* that replay applies as a unit, because
  one event file is the only crash-atomic write the ledger has.
- **P1 Ledger compatibility.** No existing event shape changes. New fields
  are optional and omitted when empty. New event *types* require workspace
  format 1.1, activated deliberately (D10). The AISM corpus (pinned by
  manifest and digest) must parse, hash-verify and replay without error for
  every release; derived state and export may differ only in fields a
  release note names.
- **P2 Corrections are events.** Every after-the-fact correction is an
  attributed, reasoned, append-only event that replay applies. No in-place
  file rewrites.
- **P3 Verifier control (Law 6).** A change that can weaken a trust signal
  reopens the node for re-verification, in the same event. A change that
  only reveals (an audit finding, added taint) does not.
- **P4 The tool enforces structure (Law 5).** Where the help already
  promises an affordance, the affordance exists.
- **P5 Self-documenting (Law 9).** Every new or changed command states its
  policy in `--help` and in its error messages. Outcomes are structured
  codes, not message substrings.
- **P6 Sequencing.** Answer the issue author first. The commit primitive
  before anything built on it. Edges correctable before edges load-bearing
  for taint. Format activation before the first new event type.
- **P7 Migration is a manifest, not choreography.** A running workspace is
  migrated by a preview-able, resumable, idempotent batch operation whose
  result is a graph diff and an audit. Resumability is durable: the ledger
  itself records which manifest items were applied.
- **P8 Say what a signal means.** `validated` is a recorded verdict.
  Whether the proof under it is *currently* supported is a derived,
  recursive, revision-aware signal with its own name.

## The support relation (used by D1, D4, D6)

One typed relation, defined once:

- **Result-use edge** `n → t`: `t` is a *result* `n` relies on. Sources:
  (i) `t` is a child of `n` and `n` is not a `local_assume` (a parent's
  proof is its children); (ii) `t ∈ n.dependencies ∪ n.validation_deps` and
  `t` is not a `local_assume`.
- **Hypothesis-use edge** `n → h`: `h` is a `local_assume` node whose scope
  encloses `n`. Hypotheses are introduced, not established; the edge carries
  no taint and never forms a cycle. `case` nodes do not open a scope
  (`internal/schema/nodetype.go`); a case hypothesis is a `local_assume`
  child of the case, as `docs/concepts.md` already shows.
- **Scope** is derived structurally at replay from node types (a
  `local_assume` opens a scope over its later siblings until a sibling
  `local_discharge` closes it, and over its own subtree); no new events.
- **Severed** nodes (`archived`, `refuted`) are excluded from a parent's
  result-use edges (as in 0.1.7) but a *dependency* on a severed or missing
  node is an unresolved result-use edge with an audit blocker.

Acyclicity is required over result-use edges only. Legacy workspaces may
contain cycles under this relation; they are detected as strongly connected
components, treated as unresolved, reported by `audit`, never rejected at
replay.

## Decisions

### D0. Fix the commit primitive first

Two defects in `internal/service`, both confirmed:

- `verdicts apply` checks `expect_hash` and verifier-readiness on one
  `LoadState`, then calls `AcceptNodeWithVerifier`, which loads state again
  and takes its CAS sequence from the second read. An amendment between the
  reads is accepted. The challenge path has the same shape.
- `appendBulkIfSequence` CAS-checks the first event and appends the rest
  unconditionally, and `RecordProof` documents itself as atomic on top of
  it. Replay does not re-check acceptance prerequisites, so a stale
  authorisation survives.

Fix: one service primitive, `commit(func(st) ([]Event, error))`, that takes
the ledger lock, loads state once, runs validation against that state, and
appends every event with a continuous sequence check, then fsyncs the
ledger directory (today only the temp file is synced before rename).
Caller preconditions (`expect_hash`, expected revision, verifier-ready,
claim generation) are passed *into* the operation. Every mutating path
(single, bulk, verdict file, record-proof, the raw append in
`cmd/af/challenge.go`) goes through it.

Crash semantics: a multi-event batch is serialized against other writers
and leaves a valid prefix on crash; that is why P0 requires each semantic
unit to be one event. Docs and help say "serialized; prefix on crash" for
multi-event operations, not "atomic". Optional `operation_id` on every
event lets a retried operation recognise its own committed result from the
ledger (D2). Tests: an injected barrier between validate and append; process
death between files of a batch; a lost response followed by a retry.

The ledger lock file has no staleness handling today. It gains `pid` and
`acquired_at`; `af reap --ledger-lock` clears one whose holder is dead or
older than the lock timeout, and only that.

### D1. Cycle and scope checks on every creation and amendment path

Today `internal/cycle` sees only dependency edges, `Refine` checks from the
parent's position (child→parent rejected, child→grandparent accepted, error
names the wrong node; vibefeld-0ko0), and `RefineNodeBulk` / `RecordProof`
run no cycle check at all.

- Cycle check over result-use edges, from the node's own position, with the
  prospective node(s) and their edges overlaid (the cycle package returns
  "no cycle" for an absent source today). Runs in `Refine`, `RefineNodeBulk`,
  `RecordProof`, `refine-sibling`, `CreateNode`, `amend-deps`, over the
  complete prospective batch.
- Scope check: a dependency on a node inside a `local_assume` scope that
  does not enclose the citing node is a scope leak and is rejected.
- Edges to a `local_assume` are hypothesis-use: allowed when in scope, never
  cyclic.
- A removal-only amendment is accepted even if an unrelated legacy cycle
  remains.

`docs/concepts.md` replaces "ancestors or siblings" with the relation above.
The skipped test `TestRefineCmd_WithDependsFlag_DependOnParent` is replaced
by two: forbidden result-use of a `claim` ancestor, permitted hypothesis-use
of an enclosing `local_assume`.

### D2. `amend-deps`: append-only edge correction, one event, batchable

```
af amend-deps <node-id> [--add ids] [--remove ids]
                        [--add-validated ids] [--remove-validated ids]
                        [--reopen] [--expect-hash h] [--strict]
                        --owner <agent> --reason "<text>"
af amend-deps --file manifest.json [--dry-run] [--resume] -f json
```

- **States:** `pending`, `draft`, `needs_refinement`. On `validated`,
  refused unless `--reopen`. `admitted` requires `af unadmit` first.
- **One event.** `node_deps_amended` (format 1.1):
  `{node_id, previous_dependencies, new_dependencies,
    previous_validation_deps, new_validation_deps, owner, reason,
    previous_content_hash, reopened: bool, operation_id}`.
  With `reopened`, replay performs the epistemic transition
  `validated → pending` (recorded with the same reason and owner, visible
  in `af log` and audit exactly as a `node_unvalidated` would be) *and* the
  edge replacement in one step. There is no window in which the node is
  pending with its old edges. `node_amended` gains the same optional
  `reopened` and `operation_id`, and `af amend` the same `--reopen` and
  state set; today a `needs_refinement` node can be amended by nobody,
  because `unvalidate` only accepts `validated`.
- **Precedence:** preconditions first (state, claim ownership,
  `--expect-hash`, target existence, D1), then the change set. An add of an
  existing edge or a remove of an absent one is `unchanged` and appends
  nothing; `--strict` makes it an error; contradictory add/remove of one id
  is always an error. A stale `--expect-hash` is rejected even if the
  requested edges are already present, *unless* the ledger contains an
  event with this `operation_id`, in which case the outcome is
  `applied` (already) with that event's sequence. This is what makes a
  retry after a lost response safe.
- **Replay** verifies the event's previous lists against the state it holds
  (mismatch is a replay error, never an overwrite), replaces both lists,
  recomputes the content hash, and appends a `DepsAmendment` record to the
  node's amendment history.
- **Manifest form:** JSON items, each the flags above plus an
  `operation_id` (generated by `--dry-run` and kept), applied in file order,
  one D0 commit per item, outcomes
  `applied | applied(already) | unchanged | rejected:<code> | blocked:<code>`,
  every item reported exactly once, exit codes as `verdicts apply` (5
  partial, 6 none, 7 no-op). `--dry-run` prints the exact edge diff, the
  scope or cycle path on rejection, and the resulting hash. `--resume`
  re-runs the file; already-applied items are recognised from the ledger,
  so a crash mid-manifest is recovered by rerunning it. The output ends
  with a **re-verification work list** (node, new hash, reason) for
  `verdicts apply`, and the exact command for the final graph diff and
  audit. This is the migration for the 21 MIP\*=RE nodes.
- **Surfaces:** `af get`, `af amendments` (both kinds, ledger order; statement
  version numbering unchanged), `af diff` (dependency-aware, including
  `--since-challenge`), `af deps` (marks amended edges), `af export --graph
  json` (per-node `dependency_amendments` and `validation_deps`, behind a
  capability flag, shipped with this release), `af log`.
- **Stale inputs:** any verdict, challenge or `record-proof --expect-hash`
  authored against the old hash is rejected by D0 and regenerated after
  review. Never rewrite `expect_hash` in an old verdict file.
- Whether an edge-only repair discharges a `needs_refinement` request is a
  verifier's call: the node stays `needs_refinement` until accepted.

### D3. Acceptance records what it accepted, and says whether it was checked

`NodeValidated` gains optional `content_hash` (the hash of the content
accepted, from the same state read the accept commits against) and
`expected_hash_checked: bool` (true only when the caller supplied an
expectation that was compared; plain `af accept` has no such flag and
records `false`). Absent on legacy events means "not recorded"; replay can
reconstruct the content at the acceptance sequence and audit labels it
"reconstructed".

Coverage, stated: the hash covers the node's own fields and its dependency
*IDs*, not the contents of dependencies, children, scope, evidence or
external references. Changes to cited results are carried by
`support_current` (D4), not by the hash. `ClaimTested` gains optional
`content_hash`; acceptance counts a passing claim test only if its hash
matches the current content (legacy tests without a hash count, and audit
flags them). The legacy hash algorithm is preserved as is.

### D4. What `validated` means, and `support_current`

Holes, all confirmed: bulk accept skips the children-cleared and
validation-deps checks; refine on a validated parent succeeds and leaves
it validated with pending children; unvalidating, vetoing or requesting
refinement of a descendant leaves every ancestor validated, and a refuted
child is severed from taint so the parent shows `clean`.

`validated` stays a recorded verdict and is not revoked transitively. The
derived signal **`support_current(n)`** is defined recursively over
result-use edges:

- `n` is `validated` or `admitted` (admitted is "cleared with taint");
- `n` has no open blocking challenge;
- every result-use target `t` is `validated`, `admitted` or (for children
  only) `archived`, and `support_current(t)`; a `refuted` or `pending`
  target fails it;
- no result-use target has a content revision (statement or dependency
  amendment) with a ledger sequence later than `n`'s recorded validation,
  and `n` itself has none.

So vetoing a grandchild, or amending and re-accepting a cited lemma, makes
every consumer's `support_current` false until each is re-accepted, and
the audit names the cause and the sequence. It is computed by one shared
policy in `internal/jobs`, memoised over the DAG, and consumed by `status`,
`health`, `get`, `export` (new field, capability-flagged), `audit`, and
`auto-prove.sh`.

Enforcement:

- One eligibility validator shared by single accept, bulk accept and
  verdict files. Bulk items are scheduled by actual prerequisites, ID as
  tie-break; a verdict *file* keeps its file order. Per-item outcomes
  distinguish `blocked:prerequisite-pending` from `rejected:<code>`; the
  bulk CLI keeps its fields and adds `items`; partial success exits 5.
- Node creation under a parent is allowed only for `pending`, `draft`,
  `needs_refinement`. Refusals name the remedy: `validated` →
  `af request-refinement`; `admitted` → `af unadmit`; `refuted` /
  `archived` → none.
- The jobs classifier treats `needs_refinement` as a prover job until its
  children are cleared, then a verifier job (today it is neither).
- `health` reports every validated node with `support_current` false, by
  cause.

### D5. Terminal verifier actions release the caller's claim, fenced

A claim gets a generation: the sequence number of its `nodes_claimed`
event, shown by `af get` and `af jobs`. `NodesReleased`, `NodeValidated`,
`NodeAdmitted`, `NodeRefuted`, `NodeArchived` gain optional `claim_seq`.
`accept`, `admit`, `refute`, `archive` set `release_claim: true` and
`claim_seq` on the state event itself (one event, P0); replay releases the
claim only if `claim_seq` matches the node's current claim generation, so a
delayed or retried release cannot evict a later claim even under the same
owner string. Caller identity is threaded through `admit`, `refute`,
`archive`. An explicit `release` after auto-release is a documented no-op.

### D6. Taint follows proof support

A node's **support component** is computed over result-use edges in
dependency-topological order (memoised DFS; SCCs from legacy data are
`unresolved`):

1. severed children contribute nothing;
2. an `admitted` target contributes `tainted` and is not descended
   (0.1.7's admitted boundary);
3. a `pending` / `draft` / `needs_refinement` target contributes
   `unresolved`;
4. a severed or missing *dependency* target contributes `unresolved` plus
   an audit blocker;
5. a `validated` target contributes its own support component.

Precedence per node: own severed state → own admitted → unresolved →
tainted → clean. The ancestor-context component stays separate and is never
pushed into siblings, so a validated sibling of an admitted node stays
`clean`. Hypothesis-use edges carry nothing. The full rule order replaces
rules 0–7 in `docs/concepts.md`. `PropagateTaint`'s affected set and the
audit-emission filter include reverse dependents; `af taint-trace` names
the path and the source's revision; replay keeps its single authoritative
final derivation. External references stay outside the lattice; `audit`
lists pending external refs cited by validated nodes and `clean` is
documented as "nothing taken on faith among nodes".

Verification: independent spec implementation; differential fuzz over
random DAGs with mixed child / reference / validation edges, local scopes,
missing targets, legacy cycles, and command sequences (amend, reopen,
archive, veto); properties: replay equals incremental, idempotence, no
clean support path through a non-validated result. Export allowlist for
the corpus diff: `taint_state`, `validation.taint_counts`.

### D7. Health stops calling scrutiny evidence of falsity

Repair fatigue alarms at five resolved challenges plus amendments summed
over a subtree, absolute; the MIP\*=RE root has 291. Remove the "probably
false" language and the subtree alarm. Rework becomes descriptive, per
node, with hotspots and configurable, explained thresholds. Blockers are
open challenges, stalled or stale claims (with generation and expiry), and
`support_current` failures.

### D8. `af audit`

Read-only, text and JSON with `schema_version`, computed from an ordered
ledger pass. Findings carry a stable code, severity, `current | historical`,
node IDs, event sequences, remediation. Strict mode fails only on these
*current* codes: `SUPPORT_NOT_CURRENT`, `HASH_MISMATCH`, `CYCLE`,
`SCOPE_LEAK`, `CITES_SEVERED`, `AMENDED_NOT_REVERIFIED`, `SELF_ACCEPT`
(when identity was recorded), `VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE`.
Historical classes (admitted nodes, archives with a challenge open at
archival on the node or an active descendant, amendments per node, pending
external refs cited by validated nodes, reconstructed-hash comparisons,
unknown provenance) are reported, never gating. Output is bounded and
filterable by code, node prefix and status. 0.1.9 ships the strict codes
above plus the migration preflight/postcheck; the rest follows in 0.1.10.
`health` and `audit` share one findings engine.

### D9. Guardrails, labelled as guardrails

- `af archive` refuses when a challenge is open on the node or an active
  descendant unless `--force --reason`; `NodeArchived` gains optional
  `reason`, `forced`. The parent's verification checklist lists children
  archived with a challenge open, so the next accept acknowledges the
  abandoned obligation. `trust-model.md` says what this prevents and what
  it does not.
- Reviewer ≠ contributor: the shared validator compares the verifier
  against author, proof author, and every amender of statement or edges.
  `af accept` warns for one release without `--agent`, then requires it.
  `--allow-self` is explicit and recorded (`self_accepted`); verdict files
  keep their current contract and cannot opt out. Documented as recorded
  provenance, not proof of independence.

### D10. Format activation, compatibility, releases, benchmarks

Verified: `config.Validate` (which rejects `version != "1.0"`) is never
called in production, `LoadState` never reads `meta.json`, and the replay
CLI bypasses `LoadState`. A 0.1.8 binary runs `status`, `claim` and
`replay --verify` on a workspace stamped 1.1 and fails only on the first
unknown event type. **Old binaries cannot be fenced; they can only be kept
away.**

- 0.1.9+ checks the format at every entry point and refuses a newer format
  with a structured error naming the format and running version.
- Activation is explicit: `af workspace upgrade --to 1.1 [--dry-run]`
  reports what changes, takes the ledger lock, copies `ledger/` and
  `meta.json` to `backup/<timestamp>/` and fsyncs, re-reads the sequence,
  persists and fsyncs the new stamp, releases. Format-1.1 event types are
  refused on a 1.0 workspace with "run `af workspace upgrade`". Readers
  that loaded metadata before the stamp and then see a 1.1 event report
  "workspace upgraded during read; retry". Downgrade after any 1.1 event
  means restoring the backup.
- Operators stop old writers before upgrading; `scripts/auto-prove.sh` and
  the documented driver contract preflight every worker binary via
  `af version -f json`, which advertises `format`, `policy` and `commit`.
  Format and policy are separate: 0.1.9 and 0.1.10 both read 1.1 while
  differing in acceptance and claim policy; 0.1.11 changes taint policy.
- Compatibility matrix tested with real release binaries: old-read /
  new-write, new-read / old-write, a process holding stale state across an
  upgrade, an interrupted upgrade. The recorded 0.1.8 behaviour (unknown
  event type error) is the documented expectation, not a refusal contract.
- Git tags `v0.1.9` onward with GitHub releases and the in-binary changelog.
- AISM corpus pinned by a manifest of ledger digests, run for previous and
  new binary, asserting parse, hash verification and replay separately,
  with an explicit projection allowlist per release. The replay CLI's JSON
  mode currently prints `valid:false` and exits 0; from 0.1.9 it exits
  non-zero. Minimal fixtures live in-repo for CI.
- `scripts/synth-workspace.sh` checked in; the e2e scale test asserts
  invariants, not wall time. A benchmark job, run before 0.1.10 is cut,
  measures 100 and 1000 nodes across multiple rounds with high fan-out and
  10–50 writers, reporting events, bytes, replays per command, p50/p95
  latency, retries and lock waits; bulk accept's per-node state reload is
  the first known target. Snapshots are considered only if it shows a need.

### D11. Agent-facing accuracy

One classifier (`internal/jobs`) for `jobs`, `status`, `health`, `get`,
export and `auto-prove.sh`; the render classifier is deleted. `af --help`
workflow example; `af get` shows `validated_by`, `validation_batch_id`,
`support_current`, claim generation; `resolve-challenge` help points at
`amend-deps`; `docs/concepts.md`, `docs/prd.md` invariant 3 ("node ID is
the identity; the hash is the content revision"). `scripts/auto-prove.sh`:
`accept --note` (the flag is `--with-note`), missing identities, and a
completion test that reads the root's epistemic state alone; it now
requires root `support_current`, clean taint, and no pending external ref
cited by a validated node, and its generated commands are tested against a
stub agent on a disposable workspace.

### Deferred: tamper-evident ledger

An unanchored per-event hash chain does not defend against a writer with
filesystem access; it detects accidental edits only and needs canonical
JSON rules, a legacy-prefix commitment and an external head to be more.
v0.2 kernel work. The 0.1.x threat (a confused agent with a file tool) is
mitigated by denying write tools to af workers; D3/D8 surface a validated
node whose content moved. vibefeld-5qi9 stays open with this note.

## Releases

| Release | Contents (cut line) | For a running workspace |
|---|---|---|
| **0.1.9 "Corrections"** | D0, D10, D1, D3, D2 (incl. export fields), D4, D7, D8 strict subset, D11 | Stop writers, upgrade binaries, `af workspace upgrade --to 1.1`, `af audit`. Apply the manifest with `--dry-run`, then for real; re-verify via `verdicts apply`; `af audit --strict`; `af export --graph json` diff. |
| **0.1.10 "Claims and guardrails"** | D5, D9, D8 remainder, benchmark job | Upgrade binaries; `af audit` before and after. Policy number changes. |
| **0.1.11 "Support taint"** | D6, taint-trace, taint export capability | Upgrade binaries. `af taint-trace` and `af audit` explain every node whose taint moved. |

Cut line: anything not in the 0.1.9 column slips to 0.1.10 rather than
delaying the tag. If 0.1.9 runs long, the minimum that still serves the
issue author is D0, D10, D1, D2, D3 and the migration manifest; D4 then
leads 0.1.10.

## Order of work

0. Reply on GitHub #3 now: decision (A), the D1/D2 spec, the manifest
   migration, and an invitation to review the spec or contribute the
   tests-first patch. Ask for a copy of the ledger for the acceptance test.
1. D0 commit primitive, fsync, `operation_id`, lock staleness; migrate all
   mutating paths; barrier, process-death and lost-response tests.
2. D10 format check, `af workspace upgrade`, replay exit code, corpus
   manifest, `af version -f json`, compatibility matrix.
3. D1 support relation, scope derivation, checks on every creation path;
   0ko0 closed.
4. D3 acceptance hash and claim-test hash.
5. D2 `amend-deps` and `amend --reopen`, manifest, surfaces, export fields.
6. D4 `support_current`, shared validator, creation gate, classifier
   (closes hspn).
7. D7 alarm removal; D8 strict subset and preflight; D11.
8. Tag v0.1.9. Update #3.
9. D5 claim generations and fenced release (closes kbzm, uj18).
10. D9 guardrails (recreates a7p5, gwps). D8 remainder (recreates 5qrx).
11. Benchmark job. Tag v0.1.10.
12. D6 with spec and fuzz (recreates 0ry1). Tag v0.1.11.

The beads referenced by `handoff.md` and `docs/trust-model.md` (0ry1, a7p5,
5qrx, gwps, 8x16, w2mt, 0zk7) do not exist in the database; they are
recreated while filing the above, and the items that came from #3 are
mirrored there.

## Acceptance test for 0.1.9

A correction manifest for the MIP\*=RE tree (a copy of the real ledger if
the author provides one; otherwise a fixture built from the four examples
in #3, which cannot stand in for all 21) applied end to end: `--dry-run`
shows every edge diff and the D1 verdict for each cousin edge; the real run
reports every item once; killing the process mid-manifest and rerunning
with `--resume` yields the same final ledger; the re-verification list is
re-accepted through `verdicts apply` in file order; `af audit --strict`
passes; the export diff shows only the corrected dependency sets,
amendment history and `support_current`; a 0.1.8 binary pointed at the
upgraded workspace fails with its unknown-event error, and the driver
preflight excludes it before that.

## Explicitly not in this plan

State caching or replay snapshots (the benchmark job decides); external
reference taint; the hash chain; canonical content-hash encoding;
crash-atomic multi-event batches (a journal); the v0.2 kernel, spec, or
TLA+ work; any change to existing event shapes.

## What changed and why (v1 → v2 → v3)

v2 added the commit primitive, typed cycle rules, explicit reopening, the
manifest, `support_current`, corrected dependency taint, explicit format
activation and the deferral of the hash chain. v3, after the re-review:

- Semantic units are single events (`reopened` on the amendment event,
  `release_claim` on the state event) because batches are not crash-atomic.
- Durable resumability via `operation_id` recorded in the ledger.
- One support relation shared by D1, D4 and D6; `case` is not a scope
  opener, hypotheses are `local_assume` nodes; D6 no longer contradicts D1.
- `support_current` is recursive and revision-aware; a vetoed grandchild
  or a re-accepted cited lemma invalidates consumers.
- Claim generations instead of owner strings; ledger-lock staleness.
- `expected_hash_checked` distinguishes recording from checking; claim
  tests bound to a hash.
- Release table made consistent with its own dependencies; D4 moved into
  0.1.9; a cut line and a minimum set stated; author reply moved first.
- The acceptance test no longer asks an old binary for a refusal it cannot
  give.
