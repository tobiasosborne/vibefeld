# TLA+ models (exploratory)

**Status: smoketest.** This is a first, small TLA+ model of af's write path,
written to test whether TLA+ earns its place in the v0.2 "trusted kernel" work
(epic vibefeld-w2mt; this is bead vibefeld-w2mt.1, the seed for
vibefeld-8fm6). It is not yet a specification af is held to: nothing checks
that the Go code refines it, and the model covers one protocol only.

## Run it

```bash
scripts/tla-check.sh            # all models, ~30 s
scripts/tla-check.sh -v two-reapers-lost-ack   # one model, with TLC's trace
```

Needs `java`. The script uses `$TLA2TOOLS_JAR`, else
`~/.local/share/tla/tla2tools.jar`, else downloads the pinned tla2tools
v1.7.4 (checksum verified) into `~/.cache/vibefeld/`.

Each `models/*.cfg` declares its expected result on its first line
(`\* expect: pass` or `\* expect: violates <Invariant>`); the script fails if
TLC disagrees. Expected violations are findings recorded as such, not
failures. Every passing model has a `-witness` twin that must be violated, to
show the pass is not vacuous (e.g. that commits actually happen).

## What `LedgerCommit.tla` models

One process step is one filesystem call, and each step is atomic.

| Process | Step | Go |
|---|---|---|
| writer | `Load`: read the ledger tail, no lock | `service.commit` → `s.LoadState()` (`internal/service/commit.go:36`) |
| writer | `Acquire`: create `ledger.lock` with a fresh token | `LedgerLock.tryAcquire`, `O_CREAT\|O_EXCL` (`internal/ledger/lock.go:98`) |
| writer | `AcquireTimeout`: give up while the lock exists | `LedgerLock.Acquire` poll loop (`internal/ledger/lock.go:64`) |
| writer | `Cas`: tail still equals the loaded tail? | `NextSequence` + `actualLatest != expectedSeq` (`internal/ledger/append.go:455`, `internal/ledger/filename.go:54`) |
| writer | `Publish`: one event file per step | `os.Rename(temp, final)` + `fsyncDir` (`internal/ledger/append.go:509`) |
| writer | `Release`: unlink the lock if the token is ours; retry on conflict | `LedgerLock.Release` (`internal/ledger/lock.go:157`); `commitRetry` (`internal/service/commit.go:77`) |
| writer | `Crash`: the process dies, its lock file stays | kill -9, OOM, agent timeout |
| reaper | `ReadLock`: read the lock, judge it stale | `RemoveIfStale` + `staleLockReason` (`internal/ledger/lock.go:335`, `:292`) |
| reaper | `ReRead`: same token as before? | `sameLockForRemoval` (`internal/ledger/lock.go:377`) |
| reaper | `Unlink`: remove the path | `os.Remove(path)` (`internal/ledger/lock.go:365`) |
| reader | `ListAtomic` / `ListStep`: list the ledger dir, no lock | `listEventSequences` (`internal/ledger/read.go:157`); replay rejects gaps (`internal/state/replay.go:61`) |

Switches: `PidVisible` (can the reaper see whether the holder's pid is alive:
false for agents in other containers, PID namespaces or hosts),
`NoReplace` (publish with `link()`/`renameat2(RENAME_NOREPLACE)` instead of a
replacing rename), `AtomicReaddir` (a directory listing is one snapshot;
POSIX does not promise this for entries added during the scan).

Properties: `MutualExclusion`, `NoLostAck` (a commit reported as successful
stays in the ledger), `NoGap`, `BatchPrefix` (a writer's surviving events are a
prefix of its batch), `NoABA` (a passing CAS means the loaded prefix is
unchanged), `ReaderSeesNoGap`.

Not modelled: OS crash and fsync durability, temp files, event contents and
replay semantics, claims (they are ledger events, not locks), the side files
outside the ledger (`assumptions/`, `external/`, `pending-defs/`), liveness.

## Results so far

| Model | Result | Meaning |
|---|---|---|
| `baseline` | pass | Two writers, no reaper, no crash: the lock + CAS protocol holds all properties. |
| `crash-one-reaper` | pass | A writer dies holding the lock, one reaper clears it, later commits are safe. |
| `two-reapers-mutex` | violates `MutualExclusion` | Two concurrent `af reap --ledger-lock` runs both pass the identity re-read of a dead holder's lock; the first unlinks it, a writer takes a fresh lock, the second reaper unlinks *that*, and a second writer gets in. |
| `two-reapers-lost-ack` | violates `NoLostAck` | Both writers pass the CAS against the same tail and publish to the same sequence number; `os.Rename` replaces the first writer's already-acknowledged event. The ledger stays contiguous, so replay cannot notice. |
| `hidden-pid-lost-ack` | violates `NoLostAck` | One reaper suffices when the holder's pid is not visible: it removes a live writer's lock. |
| `hidden-pid-aba` | violates `NoABA` | After such an overwrite, a third writer's CAS passes on the tail number although the event it loaded was replaced. |
| `two-reapers-noreplace` | pass (`NoLostAck`, `NoGap`, `BatchPrefix`, `NoABA`) | With a no-replace publish the same race still breaks mutual exclusion (see its witness) but the second writer's publish fails instead of overwriting: no acknowledged event is lost. |
| `readdir-gap` | violates `ReaderSeesNoGap` | A lock-free reader whose listing is not atomic can see event n+1 without event n and report a healthy ledger as corrupt (replay's gap check). Whether ext4 does this in practice is untested. |

The comment on `RemoveIfStale` says a wrongly removed lock "only forces the
next writer to retry"; the model says it can lose an acknowledged write. This
is a model result; a Go reproduction is the next step (bug vibefeld-y9cp).
The readdir question is vibefeld-5hn4.

## Next steps

See `vibefeld-8fm6`: reproduce the lost write in Go, fix it (no-replace
publish plus an atomic stale-lock takeover), add liveness and OS-crash
durability, then decide whether to model event semantics (claims, epistemic
transitions, "every committed batch replays") and trace validation against
the Go code.
