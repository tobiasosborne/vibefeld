# REPORT-BENCH — D10 benchmark job

Bead: `vibefeld-n5fm` (plan `docs/plans/scale-hardening.md` § D10, benchmark
paragraph). Branch `work/benchmark-job`. Not pushed (per brief).

## What shipped

| Deliverable | File |
|---|---|
| Synthetic workspace generator (N nodes, fan-out/depth, cross-refs, R rounds, seeded) | `scripts/synth-workspace.sh` |
| Concurrent-load + single-command + fsync benchmark (markdown + JSON) | `scripts/benchmark.sh` |
| 1000-node invariant scale test (`integration` tag, Go service API) | `e2e/scale_bench_test.go` |
| Test-only directory-fsync switch (`AF_TEST_NO_FSYNC=1`, unsafe) | `internal/ledger/append.go` (+ `nofsync_test.go`) |
| Docs | "Benchmarks and scale tests" in `CONTRIBUTING.md` |
| Changelog | 0.1.10 benchmark line in `cmd/af/changelog.go` |

**Prerequisite merge.** The benchmark measures `af audit`, which is D8 and was
not on this branch's base. The handoff explicitly sequences D8 before the
benchmark job ("Remaining for 0.1.10 after D5/D9 and D8 merge: benchmark job"),
so this branch merges `work/d8-audit` (commit `5fe1bfe`) first; two conflicts
(`cmd/af/changelog.go`, `docs/trust-model.md`) were resolved by keeping both D9
and D8 text. `go build ./cmd/af`, `go vet ./...`, `go test ./...` and
`gofmt -l cmd internal e2e` are clean after the merge (one pre-existing
`internal/fs` atomic-write flake, `vibefeld-8rjx`, passed on re-run and in
isolation).

## How it was run

```
scripts/build.sh
scripts/synth-workspace.sh -n 1000 -s 1 -r 2 -o /tmp/synth   # (used by the benchmark)
scripts/benchmark.sh --workdir /tmp/af-bench-clean --keep
go test -tags integration ./e2e/ -run TestScale_SyntheticWorkspace1000 -v
```

Machine: the branch worktree host (Linux), af `0.1.9` built at commit
`b607361`, bash 5, jq. The benchmark run below built fresh workspaces (no
`--skip-build`). Run duration: 30 s per writer configuration, 2 verification
rounds in the synthetic workspace, 100 and 1000 nodes, 10 and 50 writers.

## Concurrent load

Each command replays the ledger once, so `replays/command = 1` by construction.

| N | W | command | count | p50 (ms) | p95 (ms) | retries (exit 1) | lock waits | events app. | ledger bytes |
|---|---|---------|-------|----------|----------|------------------|------------|-------------|--------------|
| 100 | 10 | claim | 10808 | 23.29 | 28.88 | 227 | 0 | 468 | 163576 |
| 100 | 10 | refine | 183 | 49.59 | 76.8 | 68 | 0 | 468 | 163576 |
| 100 | 10 | release | 183 | 40.88 | 55.26 | 76 | 0 | 468 | 163576 |
| 100 | 10 | accept | 221 | 49.33 | 88.15 | 120 | 0 | 468 | 163576 |
| 100 | 50 | claim | 31000 | 40.11 | 52.3 | 663 | 0 | 305 | 112379 |
| 100 | 50 | refine | 177 | 66.97 | 136.02 | 64 | 0 | 305 | 112379 |
| 100 | 50 | release | 177 | 82.97 | 199.86 | 75 | 0 | 305 | 112379 |
| 100 | 50 | accept | 191 | 77.83 | 237.85 | 171 | 0 | 305 | 112379 |
| 1000 | 10 | claim | 2507 | 82.57 | 152.1 | 2087 | 0 | 337 | 771410 |
| 1000 | 10 | refine | 195 | 196.41 | 216.29 | 181 | 0 | 337 | 771410 |
| 1000 | 10 | release | 195 | 149.05 | 163.29 | 151 | 0 | 337 | 771410 |
| 1000 | 10 | accept | 303 | 158 | 293.21 | 206 | 0 | 337 | 771410 |
| 1000 | 50 | claim | 6672 | 166.84 | 412.78 | 5739 | 0 | 239 | 755203 |
| 1000 | 50 | refine | 178 | 338.84 | 658.57 | 170 | 0 | 239 | 755203 |
| 1000 | 50 | release | 178 | 270.59 | 504.28 | 159 | 0 | 239 | 755203 |
| 1000 | 50 | accept | 922 | 282.52 | 520.6 | 880 | 0 | 239 | 755203 |

Notes: `claim` p50 is dominated by **failed** optimistic-CAS claims, which
return quickly; the successful write commands (`refine`, `release`, `accept`)
are the meaningful write latencies. At 50 writers the claim retry rate is high
(e.g. 5739/6672 = 86% at 1000 nodes) — that is the CAS sequence-mismatch retry
volume, i.e. the optimistic-concurrency tax, not lock acquisition.

## Single-command latency (1000-node workspace)

| command | count | p50 (ms) | p95 (ms) |
|---------|-------|----------|----------|
| status | 5 | 136.24 | 143.32 |
| jobs | 5 | 128.93 | 131.05 |
| health | 5 | 173.85 | 179.31 |
| audit | 5 | 142.64 | 150.65 |
| export_graph | 5 | 162.94 | 168.16 |
| bulk_accept_20 | 3 | 1216.16 | 1224.74 |

Supporting scaling measurements (ad hoc, same build):

| Measurement | Result |
|---|---|
| `status` / `jobs` / `audit` at 100 nodes | 21 / 19 / 21 ms |
| `status` / `jobs` / `audit` at 1000 nodes | 149 / 146 / 164 ms |
| bulk accept k=1 / 10 / 20 / 40 / 80 on 1000 nodes | 254 / 792 / 1273 / 2408 / 4756 ms |

## Directory-fsync cost (200 sequential single-node refines)

| mode | elapsed (ms) |
|------|--------------|
| fsync | 99967.65 |
| AF_TEST_NO_FSYNC=1 | 98231.25 |

Delta **1736.40 ms** over **600** appended events ≈ **2894.0 µs/event** on the
1000-node workspace. That single pass is noisy because replay dominates
(`af` replays the whole ~2 700-event ledger on every one of the 600 commands);
a repeated measurement on a dedicated 250-node workspace is much steadier:

| Pass | fsync (600 events) | `AF_TEST_NO_FSYNC=1` | delta |
|---|---|---|---|
| 1 | 32875 ms | 31640 ms | 1235 ms |
| 2 | 32955 ms | 31664 ms | 1291 ms |
| 3 | 33004 ms | 31700 ms | 1304 ms |

≈ **2.1 ms per appended event** of directory fsync alone (the per-event file
fsync still runs in both arms). This is the `vibefeld-bsb1` cost that makes the
default 5 s ledger-lock timeout easier to exceed under machine load.

## Interpretation

**Is bulk accept's per-node state reload measurable?** Yes, but it is *not* a
per-node full reload. Bulk accept of 20 nodes takes ~1.2 s on 1000 nodes, while
one accept is ~0.25 s; a per-node replay would put 20 accepts near 5 s. The
k=1→80 sweep is linear at ~50–60 ms per additional node after a single
~0.2 s replay, so the bulk path loads state once and pays a constant per-node
commit/hash/taint cost. The first known target is measurable but bounded.

**Are lock waits at 50 writers a problem?** Not on this machine at this
duration: all 16 command cells recorded **zero** lock-timeout waits even at 50
writers on 1000 nodes. The observed exit-1 volume is optimistic CAS retries
(`ErrSequenceMismatch` / node already claimed), which the worker loop simply
retries; p95 `claim` at 50 writers/1000 nodes is 413 ms. `vibefeld-bsb1`'s
timeouts were seen under a `go test` burst on a loaded machine, so the default
5 s ledger-lock timeout looks adequate here but remains worth revisiting if
fsync latency grows on slower storage.

**Is anything quadratic?** No quadratic behavior was observed inside any one
command. Single-command latency grows ~7–8× for a 10× node count (100→1000),
tracking the linear O(events) replay plus a fixed process startup; bulk accept
is linear in the batch size. The *shell generator* is O(N²) wall time because
each of its O(N) CLI commands replays the whole ledger — that is a property of
driving the CLI one node at a time, not of a command's own complexity (the Go
service builder avoids it).

## Snapshots for 0.1.11?

Snapshots are **not needed for 0.1.11**: 1000-node commands are 130–180 ms, bulk
accept is linear, lock waits are absent, and the only super-linear cost is the
deliberately CLI-driven synthetic generator, not any production command. A
memoised replay or persisted state snapshot would help only when workspaces
reach several thousand events or when an agent needs sub-50 ms calls; neither is
on the 0.1.11 (support-taint) path. Revisit after `af taint-trace`/D6 lands and
the corpus or real workspaces cross the few-thousand-node mark.

## Caveats

- The `af audit` measurements required the D8 merge described above; on a base
  without D8 the benchmark cannot run.
- The benchmark's writer loop partitions candidate nodes across workers, so the
  exit-1 retries it reports are ledger-CAS contention (the interesting kind),
  not all 50 workers racing the same node.
- The fsync table from `scripts/benchmark.sh` is a single pass at 1000 nodes and
  is dominated by replay; the repeated 250-node measurement above is the more
  reliable per-event figure.
- `benchmark-results.json` (written by the run) is not committed; the tables
  above are its contents.
