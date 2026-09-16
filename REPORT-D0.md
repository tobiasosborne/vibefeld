# D0 Report — Optimistic commit primitive

Bead: `vibefeld-qgjg` (also fixes `vibefeld-tlh1`). Branch: `work/d0-commit-primitive`.
Plan: `docs/plans/scale-hardening.md` D0 + v3.1 amendment 2 (D0 is optimistic, not
pessimistic).

## What changed

### Ledger primitives (`internal/ledger`)

- `AppendBatchIfSequence(dir, events, expectedSeq)` + `(*Ledger)` method, and
  `AppendBatchIfSequenceWithTimeout`. Under the exclusive lock it checks the
  whole batch against the ledger tail (`NextSequence`), returning the same
  wrapped `ErrSequenceMismatch` as the single-event CAS. On a mid-batch rename
  failure it leaves the already-renamed prefix in place (a valid ledger prefix),
  removes the remaining temp files, and returns the partial sequences plus the
  error.
- `Append`, `AppendIfSequence`, `AppendBatch` and `AppendBatchIfSequence` now
  `fsync` the ledger directory after the rename(s) (`fsyncDir`). `EINVAL` /
  `ENOTSUP` from directory fsync is treated as best-effort so filesystems
  without it do not fail appends. `AppendBatch` keeps its existing rollback
  semantics; nothing in `internal/service` or `cmd/af` calls it any more.
- Lock file metadata gains `pid`. `readLockFile` still understands legacy
  plain-text lock files (bare agent id; age falls back to the file mtime).
  `Release`/`Holder` use it. New helpers: `Inspect`, `IsProcessAlive`
  (`Signal(0)` semantics), `StaleLock`, `RemoveLockFile`.
- `Event.BaseEvent` gains optional `operation_id` (`omitempty`), so no existing
  event shape changes.

### State (`internal/state`)

- `State` indexes `operation_id -> first sequence` during replay; new
  `RecordOperationID` / `HasOperationID`. Empty ids are never indexed.

### Service (`internal/service`)

- `commit.go`: `commit(build func(st) ([]Event, error))` — one `LoadState`
  outside the lock, build/validate against that read, then
  `AppendBatchIfSequence` with `st.LatestSeq()`; conflicts become
  `ErrConcurrentModification`. `commitRetry(n, build)` (present, unused — see
  below) and `commitThenTaint` for epistemic transitions.
- Every mutating path migrated: `Init`, `CreateNode`, `ClaimNode`,
  `RefreshClaim`, `ReleaseNode`, `Refine`, `AcceptNodeWithVerifier`,
  `AcceptNodeBulkWithVerifier`, `AdmitNode`, `RefuteNode`, `VetoNode`,
  `ArchiveNode`, `AddDefinition`, `ExtractLemma`, `SubmitNode`,
  `RefineNodeBulk`, `AmendNode`, `RecomputeAllTaint`,
  `RequestRefinement`, `UnvalidateNode`, `UnadmitNode`, `RecordApproachTried`,
  `ProposeStrategy`, `AddPattern`, `AddHint`, `AttachEvidence`, `SetOutline`,
  `LinkOutlineStage`, `RunClaimTest`, `RunDefCheck`, `RecordProof`, and the
  taint-audit emission path. `appendBulkIfSequence` is deleted.
- `AcceptNodeWithVerifier` / `RaiseChallengeWithBatch` now delegate to
  `buildAcceptEvents` / `buildChallengeEvents`, which `verdicts apply` uses
  inside its own commit closure together with the `expect_hash`,
  verifier-readiness and reviewer≠author gates. The old two-read race is
  gone.
- New `challenges.go`: `ResolveChallenge`, `WithdrawChallenge` (one-read
  open-status check + append) and `ReleaseNodes`.

### CLI (`cmd/af`)

- `challenge.go`, `resolve_challenge.go`, `withdraw_challenge.go` call the
  service methods instead of appending raw events.
- `reap.go` releases claimed nodes via `ReleaseNodes`; new `--ledger-lock`
  mode removes the ledger lock only when its pid is dead, or (no pid) when
  `acquired_at`/mtime is older than the configured `lock_timeout`. A live lock
  is reported and exits 1. JSON/`-f json` output reports the outcome.
- Multi-event help/changelog wording changed from "atomic" to "serialized; a
  crash leaves a valid prefix" (`record-proof`, `refine` bulk, changelog).
  Single-event / filesystem-POSIX "atomic" uses were left alone.

## Tests (no build tags)

- `internal/ledger/append_batch_cas_test.go`: stale `expectedSeq` appends
  nothing; happy-path consecutive sequences; blocked N-th rename leaves a valid
  prefix and the prefix is readable; empty batch is a no-op.
- `internal/ledger/lock_staleness_test.go`: live pid refused by `StaleLock`;
  dead pid stale + removable; legacy plain-text content read and stale by
  mtime; legacy ownership honoured by `Release`.
- `internal/service/commit_test.go`: barrier test (build appends a concurrent
  event → `ErrConcurrentModification`, nothing appended); successful two-event
  commit; verdicts apply amended between authoring and applying →
  `rejected:content-hash-mismatch` and no `NodeValidated`.
- `internal/state/operation_id_test.go`: `operation_id` indexed on replay;
  first occurrence wins.
- `cmd/af/reap_ledger_lock_test.go`: live pid refused and lock kept; dead pid
  removed; legacy (old-mtime) content removed; no lock is a no-op.

## Verification

```
go build ./cmd/af          # ok
go vet ./...               # ok
go test ./...              # all packages ok
go test -tags integration -run NONE_MATCH ./...   # all tagged tests compile
```

## Notes / deviations

- **`commitRetry` is defined but unused.** The "loop near proof.go L2184" is
  the sequential `TaintRecomputed` append loop in `RecomputeAllTaint`, not a
  retry loop; it is now a single batch commit, so no caller needed retry. The
  helper is left for D2.
- **`operation_id` is plumbing only.** No CLI flag (per brief); the build func
  sets it on the events it returns and `commit` appends them unchanged. Nothing
  yet reads `HasOperationID` to short-circuit a retry; that is D2's `--resume`.
- **Test (c) uses `ledger.ReadAll` rather than `state.Replay`** to avoid an
  import cycle in the `ledger` package tests; it asserts the prefix is present,
  concrete, and readable.
- **Reap `--ledger-lock` age timeout** uses the workspace `lock_timeout`
  (`service.LockTimeout()`); a live pid is never reaped regardless of age.
- **`fsyncDir` ignores `EINVAL`/`ENOTSUP`** so directory-fsync-less
  filesystems still work; other errors are returned.
- `service.Init` (a raw `Append` site) was migrated to `commit` as well.
- `getLedger` remains only for read-only operations (`LoadState`,
  `isInitialized`, `Status`, taint-audit scan).
