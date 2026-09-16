# REPORT-D1 — cycle and scope checks on creation and amendment paths

Plan item D1 of `docs/plans/scale-hardening.md` (v3.1; the amendments do not
change D1). Bead `vibefeld-k336`. Also settles `vibefeld-0ko0`.

Branch: `work/d1-support-relation`. No event shapes changed. `docs/plans` was
not touched. Nothing was pushed.

## What changed

### New package `internal/support`

- `Edge{From, To, Kind}` with `Kind = ResultUse | HypothesisUse`.
- `ProspectiveNode{ID, ParentID, Type, Dependencies, ValidationDeps}`. An
  overlay entry whose ID already exists in state **replaces** that node's
  edges; this is what makes a removal-only amendment expressible (D2 hook).
- `ResultUseEdges(st, overlay) Provider`. The provider implements
  `cycle.DependencyProvider`, so `internal/cycle` is reused unchanged. Result
  edges are (i) every non-`local_assume` child of the node, plus (ii) every
  `target` in `Dependencies ∪ ValidationDeps` that is not a `local_assume`.
  Severed (archived/refuted) children are dropped; a severed node is a sink.
  A dependency on a severed or missing node is kept as a sink edge.
- `DanglingDeps(st, overlay)` / `Provider.Dangling()` report dependencies on
  missing (`Severed=false`) or severed (`Severed=true`) nodes for audit.
- `CheckCreation(st, batch)`:
  - (a) `cycle.DetectCycleFrom` from **each prospective node's own position**
    over the overlaid provider, so the error message names the actual path.
  - (b) scope check: a result-use dependency on a node inside a `local_assume`
    scope that does not also enclose the citing node is a scope leak.
  - (c) a dependency on a `local_assume` node is hypothesis-use, allowed only
    when that assumption encloses the citing node; never in the cycle graph.
- Typed errors with the offending IDs in fields: `CycleError{Source, Path}`
  and `ScopeLeakError{Node, Dep, Assumption}`. Both `Unwrap` an
  `aferrors.AFError`, so `errors.Is(err, support.ErrCycle / ErrScopeLeak)`
  and `aferrors.Code(err)` work. Exit code 3.
- `internal/errors`: new code `SCOPE_LEAK` (exit 3).

### Service wiring

- `service.checkSupportBatch` is the single entry point (new file
  `internal/service/support_check.go`). It is the clearly named hook D2
  `amend-deps` will call with a `ProspectiveNode` representing the amended node.
- `Refine`: the old parent-position `cycle.WouldCreateCycle` checks were
  removed and replaced by `checkSupportBatch` over the prospective child
  (existence checks for dependencies are kept, as before).
- `buildChildEvents`: runs `checkSupportBatch` over the WHOLE prospective child
  batch, which wires `RefineNodeBulk` and `RecordProof`.
- `CreateNode`: runs `checkSupportBatch` for the new node.
- `refine-sibling` reaches these through `Refine` / `RefineNodeBulk`.

### Tests

- `internal/support/support_test.go` (hand-built state): claim-ancestor cycle,
  enclosing `local_assume` accepted, sibling/cousin accepted, foreign scope
  leak (with discharge), later-sibling scope leak, batch mutual cycle,
  removal-only amendment with a legacy cycle in state, severed/dangling
  classification.
- `internal/service/support_checks_test.go`: `RefineNodeBulk`, `Refine` and
  `RecordProof` all reject the new cycle/scope errors.
- `cmd/af/refine_test.go`: the skipped
  `TestRefineCmd_WithDependsFlag_DependOnParent` is replaced by
  `TestRefineCmd_DependOnClaimAncestorRejected` and
  `TestRefineCmd_DependOnEnclosingLocalAssumeAccepted`.

### Docs / CLI

- `docs/concepts.md`: "must be ancestors or siblings (no forward references)"
  replaced with the result-use / hypothesis-use definition.
- `af refine --depends` help mentions the one-line rule.
- The `af refine --children` long help already documents `depends`; unchanged.

## Decisions on ambiguity

1. **Scope model.** The brief says a `local_assume` scope covers its
   descendants and later siblings until the matching `local_discharge`. The
   existing `internal/scope.Tracker` is event-based and only answers ancestry
   (`IsInScope` is `scopeNode.NodeID.IsAncestorOf(node)`), so it does not
   answer the later-sibling question; the plan also says scope is derived
   structurally. I therefore derived scope structurally in
   `internal/support/scope.go` with a preorder walk and a stack: a
   `local_assume` pushes, a `local_discharge` pops the innermost scope, and a
   node's enclosing set is the stack snapshot at entry. This matches the
   existing `docs/concepts.md` example (a discharge inside the assumption's
   subtree closes it, so later siblings are outside) and also supports the
   plan's sibling-discharge form.
2. **Child edges and `local_assume`.** Plan text conditions child result-edges
   on the *parent* not being a `local_assume`; the brief conditions them on the
   *child* not being a `local_assume`. I followed the brief (child condition),
   because it keeps a hypothesis out of the result graph consistently with
   "hypotheses are introduced, not established".
3. **Removal-only test.** D2 `amend-deps` is not implemented here, so the
   "removal-only prospective change … with an unrelated legacy cycle" case is
   covered as a support unit test with a hand-built state carrying the legacy
   cycle (equivalently to writing events directly).
4. **Dangling dependencies.** `CheckCreation` does not reject dangling
   dependencies (the plan keeps them as sink edges for audit); creation paths
   still reject a dependency on a missing node as before, preserving existing
   CLI behaviour.

## What I could not do / left for later

- D2 `amend-deps` itself is out of scope. The hook `checkSupportBatch` and the
  replace-semantics overlay are in place for it.
- `RefineNodeBulk` cannot express a mutual cycle between two new siblings
  because `#N` sibling references only point backward; that case is covered at
  the `internal/support` level.
- I did not switch `service.CheckCycles` / `CheckAllCycles` (proof_cycle.go) to
  the new provider. They still use only `Dependencies ∪ ValidationDeps` without
  child edges. D1 only asked for creation/amendment paths; `audit` (D3/D4) is
  the planned consumer of `DanglingDeps` and the full relation.

## Test output summary

```
$ go build ./cmd/af      # OK
$ go vet ./...           # OK
$ go test ./...          # all packages pass (30 ok, 0 fail)
$ go test -tags integration ./cmd/af/ ./internal/service/ ./e2e/
# ok cmd/af, ok internal/service, ok e2e
```

Targeted new tests all pass:

```
internal/support: all pass
internal/service: TestRefineNodeBulk_RejectsAncestorResultUse,
  TestRefine_RejectsForeignScopeLeak, TestRecordProof_RejectsAncestorResultUse
cmd/af: TestRefineCmd_DependOnClaimAncestorRejected,
  TestRefineCmd_DependOnEnclosingLocalAssumeAccepted
```

Commits (this branch, not pushed):

```
a4d4f11 support: typed result-use/hypothesis-use relation with cycle and scope checks
d085563 service: run support checks on every creation path
d9b6ab5 docs+cli: state the result-use/hypothesis-use rule
```

## Review fixes

An independent review found five issues. All are fixed on
`work/d1-support-relation`, TDD (tests first where they could be isolated),
with `go build ./cmd/af`, `go vet ./...`, `go test ./...` green, `gofmt`
clean, and no push. Commits:

```
433a6de support: branch-local scopes and new-edge-only cycle checks
6827d75 service: reject ParentID divergence from the child ID
```

### 1. Branch-local scope derivation (HIGH)

`computeScopes` no longer keeps one global preorder stack. Each child list is
its own scope interval: a `local_assume` opens a scope over its subtree and its
later siblings in the same list, a `local_discharge` closes the innermost scope
that either structurally encloses it or was opened in the same child list, and
the whole list's scopes are dropped when the list ends. An undischarged
assumption nested under `1.1` therefore does not enclose `1.2`, and a discharge
in one branch cannot pop a scope opened in another. The docs form (a discharge
inside the assumption's subtree closes it) and the plan form (a sibling
discharge) both still work.

Tests: `TestCheckCreation_NestedUndischargedAssumeDoesNotEncloseLaterSibling`,
`TestCheckCreation_ForeignDischargeCannotCloseSiblingScope`; the pre-existing
`TestCheckCreation_ForeignScopeLeakRejected` and
`TestCheckCreation_LaterSiblingScopeLeak` still pass.

### 2. Discharge pre-close vs post-close context (HIGH)

The universe now records two enclosing sets. `ownEncl` is the context a node's
own dependencies are checked against (pre-close for a `local_discharge`);
`encl` is what other nodes see when citing the node (post-close for a
discharge). A discharge's premise from inside the assumption it closes is
accepted, a later sibling citing the discharge is accepted, and a later sibling
citing a node still inside the closed assumption is a scope leak.

Test: `TestCheckCreation_DischargePreAndPostContext`.

### 3. ParentID / ChildID divergence (HIGH)

`Refine` now rejects inside the commit closure when
`spec.ParentID != spec.ChildID.Parent()` with the typed
`service.ErrParentIDMismatch` (code `INVALID_PARENT`, exit 3) and derives the
overlay parent from `ChildID`. `buildChildEvents` derives the overlay parent
from the generated child ID and asserts it equals the requested parent;
`CreateNode` already had no caller-supplied parent and derives it from the ID.

Tests: `TestRefine_RejectsParentIDMismatch`, plus the existing create/refine
paths.

### 4. Re-validate existing nodes whose scope changed (MEDIUM)

After overlaying, `CheckCreation` derives the scope of every pre-existing node
before and after the overlay and re-runs the scope check on any whose
enclosing-assumption set changed. Inserting a `local_discharge` before an
existing later sibling now rejects the sibling's newly orphaned hypothesis-use
even though the sibling is not in the prospective batch.

Test: `TestCheckCreation_InsertedDischargeOrphansExistingHypothesisUse` and the
service-level `TestCreateNode_RejectsOrphanedHypothesisUse`.

### 5. Reject only newly added edges that close a cycle (MEDIUM)

`CheckCreation` no longer runs `DetectCycleFrom` and rejects any reachable
cycle. It compares the pre- and post-overlay result-use graphs, and rejects only
when an edge new relative to the pre-overlay graph has a target that can reach
its source in the post-overlay graph. A removal-only amendment that keeps a
path into an unrelated legacy cycle is accepted; an added edge that closes a
cycle is rejected.

Tests: `TestCheckCreation_RemovalOnlyKeepsLegacyCyclePath`,
`TestCheckCreation_AddedEdgeClosingNewCycleRejected`; the existing
`TestCheckCreation_RemovalOnlyWithLegacyCycle` and batch/ancestor cycle tests
still pass.

### Child/parent `local_assume` edge decision

The recorded decision is implemented and tested: a child result-use edge is
excluded when the child is a `local_assume` (hypotheses are introduced, not
established) and additionally when the parent is a `local_assume`.

Test: `TestResultUseEdges_LocalAssumeOnEitherSideExcluded`.

### Quality gates

```
$ gofmt -l internal/ cmd/   # clean
$ go build ./cmd/af         # OK
$ go vet ./...              # OK
$ go test ./...             # all packages pass
$ go test -tags integration ./cmd/af/ ./internal/service/ ./e2e/   # all pass
```

Branch remains unpushed.
