# D10 report — workspace format, upgrade, replay exit, version fields

Bead: vibefeld-q6os (plan item D10 as reduced by v3.1 amendment 3).
Branch: `work/d10-format` (worktree `/home/tobiasosborne/Projects/vibefeld-wt-d10`).

## What changed

**1. Format model (`internal/config`, `internal/errors`, `internal/ledger`)**
- `config.FormatCurrent = "1.1"`, `config.FormatsReadable = {"1.0","1.1"}`,
  `config.PolicyVersion = "0.1.9"`.
- `Config.Version` still means the workspace format. `Validate` now accepts any
  readable format and rejects anything else.
- `config.CheckFormat(cfg)` returns a structured `FORMAT_TOO_NEW` error (new
  `ErrorCode`, exit 3) naming the workspace format and the running binary's
  format. Sentinel `config.ErrFormatTooNew` matches by code.
- `config.Save` is now atomic (temp file + fsync + rename + directory fsync).
- `ledger.EventType.MinFormat()` returns `"1.0"` by default; a new
  `ledger.RegisterEventMinFormat` is the seam D2's 1.1 types will use.
- `af init` / `fs.InitProofDir` stamps new workspaces `1.1`; `config.Default()`
  also moved to `FormatCurrent`.

**2. Event-type gating (`internal/service/format.go`, `internal/service/proof.go`)**
- `checkEventFormats(cfg, events)` refuses an event whose `MinFormat` is newer
  than the stamped format with
  `event type X requires workspace format 1.1; run \`af workspace upgrade --to 1.1\``.
- Called from `appendBulkIfSequence` (the batch/commit path on this branch).
- `NewProofService` calls `config.CheckFormat`, so status/claim/etc. refuse an
  unreadable workspace at the service entry point.

**3. Replay (`cmd/af/replay.go`)**
- Reads `meta.json` before replaying, applies `config.CheckFormat`, and refuses
  an event newer than the stamp when the format is readable. Unknown event types
  keep the existing replay error.
- A failed replay now returns `LEDGER_INCONSISTENT` (exit 4) in both JSON and
  text modes; the JSON payload is still printed first.

**4. `af workspace upgrade` (`cmd/af/workspace.go`)**
- New `workspace` parent + `upgrade` subcommand: `--to`, `--dir/-d`,
  `--dry-run`, `--format/-f`.
- Dry-run prints current/target/backup and writes nothing.
- Real run: acquires `ledger.LedgerLock`, copies `ledger/*.json` and `meta.json`
  into `backup/<UTC timestamp>/{ledger,meta.json}`, fsyncs files and directories,
  re-reads `ledger.NextSequence`, writes the stamp via atomic `config.Save`,
  releases. Refuses downgrade and unknown targets; idempotent no-op at target.

**5. `af version` (`cmd/af/version.go`)**
- JSON gains `format` and `policy`; text prints `Format:`/`Policy:`. Added
  `-f/--format` while keeping the existing `--json`.

**6. Fixtures and tests (`e2e/`)**
- `e2e/fixtures/format-1.0/` and `format-1.1/`: minimal workspaces (init + one
  refine + one accept), 5 ledger events each.
- `e2e/format_fixtures_test.go`: opens both formats, refuses a 9.9 stamp,
  exercises the stamp upgrade, and a `AF_PREVIOUS_BINARY` probe (skipped unless
  set; asserts only non-zero exit and logs output).
- Unit/e2e tests for `CheckFormat`, `CompareFormats`, `MinFormat`,
  `checkEventFormats`, the service format gate, `workspace upgrade` (dry-run,
  backup+stamp, idempotency, downgrade refusal), the status/claim/replay gate,
  the replay exit-4 path, and `version -f json`.

**7. Docs and changelog**
- `docs/cli-reference.md`: `workspace upgrade` section + quick-ref row; version
  flags/fields. `docs/concepts.md`: new "Workspace Format" subsection + TOC.
- `cmd/af/changelog.go`: new 0.1.9 entry. It is marked `Unreleased: true` and
  the `TestVersionInfo_MatchesLatestChangelogEntry` test now skips unreleased
  entries, so `VersionInfo` stays `"0.1.8"` as instructed (not bumped).

## What I could not do / decisions

- **No previous release binary available.** `TestFormatFixture_PreviousBinary`
  is skipped without `AF_PREVIOUS_BINARY`; it is implemented and will run in CI
  when a 0.1.8 binary is supplied.
- **No `commit(...)` helper on this branch.** As instructed, the check lives in
  `internal/service/format.go` and is wired into `appendBulkIfSequence`. The
  direct `ldg.Append` paths (`Init`, taint-recompute) were left alone per "do
  not refactor the write paths".
- **Corpus manifest** from the reduced D10 keep-list is not in the brief's
  deliverable list and was not implemented.
- **`config.Validate` vs `CheckFormat`:** `Validate` rejects non-readable
  formats with a plain error; the structured `FORMAT_TOO_NEW` decision is
  `CheckFormat`'s job (called at entry points). `Validate` remains uncalled in
  production, as before.
- **Empty/missing `version` in `meta.json`** is treated as unknown and refused.
  Every `af`-written workspace has a version, so this only affects hand-edited
  files.
- The upgrade loads `meta.json` before taking the lock (needed to decide
  idempotency/downgrade). Concurrent upgrades could pick the same timestamped
  backup directory; the documented procedure is to stop writers first.
- `af version` already had `--json`; the brief's `-f json` was added alongside
  it rather than replacing it, to avoid breaking existing callers.
- One commit accidentally bundled `replay.go` under the service message while
  two commits raced on the git index; history was reset and each file
  re-committed under its own message.

## Commits (branch `work/d10-format`, not pushed)

- `ef5b29d` format: add workspace format model (1.0/1.1) with CheckFormat
- `86bc099` service: gate event appends and workspace open on format
- `290019a` af replay: enforce format gate and exit 4 on invalid replay
- `1403568` af version: advertise format and policy; add unreleased 0.1.9 notes
- `af04794` af workspace upgrade: format migration with backup and lock
- `15adc19` e2e: add 1.0 and 1.1 format fixtures and tests
- `1b8efbc` docs: workspace format and af workspace upgrade

## Quality gates

- `go build ./cmd/af` — pass
- `go vet ./...` — pass (`go vet -tags integration ./...` also pass)
- `go test ./...` — pass (including new `e2e` package tests)
- `gofmt -l cmd internal e2e` — clean
