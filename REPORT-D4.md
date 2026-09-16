# REPORT-D4 — `support_current`, one accept validator, creation gate, classifier

Plan item **D4** of `docs/plans/scale-hardening.md` (v3.1 amendment 4), bead
`vibefeld-4tjt`; closes `vibefeld-hspn` and `vibefeld-gxa7`.

Branch: `work/d4-support-current`. No push. All commits land on the worktree.

## What shipped

### 1–2. One memoised DAG walk and `support_current` (`internal/support`)

* `internal/support/walk.go` — `Walk[T any](p Provider, fold func(n *node.Node,
  targets []Folded[T]) T) map[string]T`, the single traversal D6 will reuse
  with a taint fold (v3.1 amendment 4). Tarjan SCCs are computed once; a node
  folding a target in its own component receives
  `Folded{Cycle: true}`, a vanished target `Folded{Missing: true}`. Every node
  is folded exactly once, dependency-first, deterministically by hierarchical
  ID. `Provider` now carries the state-backed `*node.Node`s.
* `internal/support/current.go` — `Current(st) map[string]SupportStatus` with
  stable causes `NOT_VALIDATED`, `OPEN_BLOCKING_CHALLENGE`, `TARGET_NOT_CURRENT`,
  `TARGET_PENDING`, `TARGET_REFUTED`, `TARGET_REVISED`, `SELF_REVISED`,
  `CYCLE`, and `SupportStatus{Current, Cause, Node, Seq}`.
  `support_current(n)` is true iff n is validated/admitted, has no open
  blocking challenge, has no amendment after its validation sequence, and every
  result-use target is cleared and itself current (archived children are
  allowed).
* The result-use relation severs archived **and** refuted children (0.1.7), so
  `support_current` additionally inspects direct children explicitly: an
  archived child is cleared, a refuted or pending child fails the parent. This
  is what closes the "refuted child leaves the parent clean" hole without
  changing `ResultUseEdges` (whose severed-child behavior existing tests lock).
* `node.Node.ValidatedSeq` is a new derived field, stamped by `replayInternal`
  on every `NodeValidated` (no event change, no format bump) and cleared by
  `clearValidationFields`; `latestRevisionSeq` orders amendments against it.

### 4. One accept validator and prerequisite-scheduled bulk accept

* `internal/service/accept_eligibility.go` — `checkAcceptEligibility(st, n,
  opts)` with typed `AcceptBlockedError` (pending child / validation dep, crux
  claim-test, blocking challenge, needs_refinement-without-children) and
  `AcceptRejectedError` (hash mismatch, not verifier-ready, reviewer==author,
  invalid transition, node-not-found). It is now the single set of
  preconditions used by `buildAcceptEvents` (single accept), `AcceptNodesBulk`,
  and `applyAcceptVerdict` (verdict files).
* `internal/service/accept_bulk.go` — `AcceptNodesBulk` schedules by actual
  prerequisites: a node is accepted only after every child/validation dep is
  already cleared or accepted earlier in the batch, with ID as tie-break; all
  `NodeValidated` events commit in **one** commit. It returns a per-item
  `BulkAcceptReport` (`applied` / `blocked:<code>` / `rejected:<code>`) and the
  `af verdicts apply` exit tiers (5 partial, 6 none). `AcceptNodeBulkWithVerifier`
  delegates and preserves the old aggregate-error contract (still
  `errors.Is`-matchable against `ErrBlockingChallenges` / `ErrClaimTestStale`).
* `af accept` bulk output keeps `accepted`/`count`/`status` and adds `items[]`,
  `applied`, `blocked`, `rejected`; a partial success exits 5.
* Verdict **files** keep file order (`ApplyVerdicts` unchanged) and map the
  typed errors back to the existing `blocked-by:<reason>` / `rejected:<reason>`
  status strings.

### 5. Creation gate

* `internal/service/creation_gate.go` — `checkParentCreationGate(parent)` and
  the typed `ParentStateError`. Wired into `Refine`, `RefineNodeBulk`,
  `RecordProof`, and `CreateNode`. `pending`/`draft`/`needs_refinement` are
  allowed; `validated` → remedy `run af request-refinement <id>`; `admitted` →
  `run af unadmit <id>`; `refuted`/`archived` → refused, no remedy. Exit 3.
  Closes `vibefeld-gxa7` (refine on a validated parent no longer leaves it
  validated with pending children — the child is refused and no event lands).

### 6. Jobs classifier

* `internal/jobs` — `needs_refinement` is a prover job until its children are
  cleared (and while it has no children at all), then a verifier job. The
  exported `IsProverJob` gained the `nodeMap` it needs; `RecordProof` builds
  it from the same state read. `af jobs`, `af health`, `af handoff` all use
  `service.FindJobs` → `internal/jobs`. `internal/render`'s legacy classifier
  was deliberately left untouched (D11 deletes it on another branch).

### 3. Consumers

* `af status` — `!` marker on validated/admitted nodes whose
  `support_current` is false, a Legend line, and `support_current` +
  `support_cause`/`support_node`/`support_seq` per node in JSON (computed once
  per render, not per node).
* `af health` — new `cmd/af/health_support.go`, called from `analyzeHealth` in
  one line, adds a `support_not_current_<CAUSE>` blocker per cause naming the
  responsible nodes; the rest of health is untouched.
* `af get` — `support_current`, and for a not-current validated/admitted node
  the cause plus responsible node/seq (JSON and a text `Support: NOT CURRENT`
  line).
* `af export --graph json` — per-node `support_current`/`support_cause`,
  advertised by the new `support-current` capability token.

### 7. Docs and changelog

* `docs/concepts.md` — "What `validated` means, and `support_current`" (P8).
* `docs/cli-reference.md` — status marker/JSON, get fields, health blocker,
  accept scheduling/items/exit codes and the creation gate, export capability.
* `cmd/af/changelog.go` — new `0.1.10` entry marked `Unreleased: true`.

## Tests

New tests: `internal/support/walk_test.go` (DAG, diamond, legacy cycle,
self-loop), `internal/support/current_test.go` (all causes plus a clean tree),
`internal/service/accept_bulk_test.go` (child-before-parent scheduling,
blocked prerequisite for children and validation deps, typed errors),
`internal/service/creation_gate_test.go` (table for every parent state, wired
refine / bulk refine / record-proof / CreateNode), `internal/jobs/needs_refinement_test.go`
(role by child state), `internal/render/support_test.go`,
`internal/export/graph_support_test.go`, `cmd/af/health_support_test.go`, and
integration `cmd/af/accept_d4_test.go` (`af accept 1 1.2`, blocked
prerequisite, `--all`).

Green at the end of the branch:

```
go build ./cmd/af    # ok
go vet ./...         # ok
go test ./...        # all packages ok
gofmt -l cmd internal # clean
go test -tags integration ./cmd/af/ ./internal/service/   # ok
```

## Notes / deliberate choices

* The plan's "severed children are excluded from result-use edges" and
  `support_current`'s "a refuted target fails it" are reconciled by inspecting
  direct children in `Current` rather than by redefining `ResultUseEdges`.
  D6's taint fold can still ignore severed children in its own fold.
* The single `af accept <id>` path is still the single accept path (byte-for-byte
  its existing success output); prerequisite claims are surfaced by the bulk
  path, where per-item outcomes and exit 5/6 are defined. This avoids breaking
  the many existing single-accept integration tests.
* `support.Current` is computed once per render where performance matters
  (status/export); `af get` computes it for the requested subtree.
* `internal/render`'s job classifier was not changed, per the brief.
