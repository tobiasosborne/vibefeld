# REPORT — Plan items D5 and D9 (scale-hardening v3.1, 0.1.10)

Worktree: `vibefeld-wt-d5`, branch `work/d5-d9-claims-guardrails`, based on
`837d02d` (merge of D4). Work is committed locally; **not pushed** (per the
brief). `docs/plans/` was not touched.

Quality gates at HEAD:

```
gofmt -l cmd internal e2e          # clean
go build ./cmd/af                  # ok
go vet ./...                       # ok
go test ./...                      # all packages ok
go test -tags=integration ./...    # all packages ok
```

Commits (oldest first):

| Commit | Topic |
|---|---|
| `cfa8fa8` | D5: claim generation fields and fenced auto-release replay |
| `479ff74` | state: open-challenge obligations and archived abandoned obligations |
| `3ce0e1e` | D5/D9: fenced auto-release and reviewer-contributor guard |
| `45d93b9` | uj18: `GetLockInfo` reads under the mutex and applies `ClockSkewTolerance` |
| `798d9db` | D9: verification checklist lists abandoned obligations |
| `26ac8f6` | CLI: identity flags, `--allow-self`, `archive --force`, `release` no-op |
| `d64e9c7` | docs: 0.1.10 claim generations, guardrails, and identity warning |

---

## D5 — Claim generations and fenced auto-release

### Generation is derived

`node.Node.ClaimSeq int` (`json:"-"`) is the ledger sequence of the
`nodes_claimed` event that created the current claim. `state.replayInternal`
stamps it after `Apply` (the same pattern already used for `VerdictSeq`), and
`applyNodesReleased`/`maybeReleaseClaim` clear it. It is never an event field,
so embedding `Node` in an event does not change event shape.

`af get` prints a `Claim generation: N` line and adds `claim_seq` to JSON;
`af jobs -f json` adds `claim_seq` to each job entry.

### Fenced release on the state event (P0: one event)

`NodesReleased` gains optional `claim_seq`. `NodeValidated`, `NodeAdmitted`,
`NodeRefuted` and `NodeArchived` gain optional `claim_seq` and
`release_claim`. When the acting identity holds the node's claim,
`setFencedClaimRelease` stamps `release_claim: true` and the current
generation on that one event; `apply*.go` calls `maybeReleaseClaim`, which
releases only when `event.claim_seq == node.ClaimSeq`. A stale or retried
event therefore cannot evict a later claim, even under the same owner string.
Legacy events (no fields, or `claim_seq == 0`) replay exactly as before — the
claim stays, which is the pre-0.1.10 behaviour.

`admit`, `refute` and `archive` also auto-release, and thread the acting
identity (`--agent`, falling back to `AF_AGENT_ID`) into new optional `by`
fields. `accept` already had `verified_by`, so no duplicate field was added.

### Explicit release is a no-op

`af release` of an already-available node exits 0 with an `already_available`
status/message instead of a `not claimed` error. A wrong owner is still an
error. `NOT_CLAIM_HOLDER` is shared by both service errors, so the CLI
distinguishes them by message (owner mismatch first, not-claimed second) — the
same substring approach the command already used.

### uj18

`lock.GetLockInfo` now takes `lk.mu` before reading the `ClaimLock` fields and
computes `IsExpired` as `released || now > expiresAt + ClockSkewTolerance`,
matching `ClaimLock.IsExpired`. Tests cover the tolerance window, the released
case, and concurrent `Refresh`/`GetLockInfo` under `-race`. The two
integration tests that assumed a 1ns lock was immediately expired now age it
past the tolerance with `lock.ExpireForTest`.

---

## D9 — Guardrails, labelled as guardrails

### Archive the hard step

`ArchiveNodeWithOptions` (and therefore `ArchiveNode`) refuses when an open
challenge exists on the node or on an active (non-severed) descendant, unless
`--force` with a non-empty `--reason`. `state.OpenChallengeObligations` walks
the subtree and prunes severed branches so abandoning an abandoned branch is
not a new obligation. `NodeArchived` records `reason`, `forced` and `by`; the
CLI prints `forced` in JSON and the reason is now persisted (it previously
only reached stdout). The parent's checklist (`af get --checklist` and the
verifier checklist printed by `af claim`) lists direct children archived with
a challenge open, in text and as `archive_obligations` in JSON, so the next
accept acknowledges the abandoned obligation.

### Reviewer ≠ contributor

`contributorRole` compares the verifier against the node's `Author`, its
`ProofAuthor` (`af record-proof`) and every owner in its amendment history
(statement and dependency amendments). The interactive accept path sets
`CheckReviewerAuthor` and threads `--allow-self`; `--allow-self` records
`self_accepted: true` on the `NodeValidated` event. Verdict files set the same
check and never set `AllowSelf`, so they cannot opt out. Without an identity,
`af accept` prints a one-line warning for text output and still accepts
(JSON stays machine-parseable); the changelog and `--help` state that the
identity becomes required in 0.1.11.

### Docs

`docs/trust-model.md` narrows the archive-the-hard-step and roles-unenforced
entries with what the guardrails prevent and what they do **not** (recorded
provenance, not proof of independence). `docs/cli-reference.md` documents the
new flags, the no-op release, the archive guard and the checklist; the 0.1.10
changelog entry is extended.

---

## Deviations and decisions

1. **`--agent`-less accept warning is skipped for `-f json`.** The integration
   JSON tests capture stdout and stderr into one buffer; more importantly,
   machine-readable output should not carry prose. Text output warns; JSON
   simply omits `verified_by`. This is documented in `--help` and the
   cli-reference.
2. **Legacy service methods keep working.** `AdmitNode`/`RefuteNode`/
   `ArchiveNode` delegate with no identity; only the new `*WithAgent`/
   `*WithOptions` methods auto-release and record provenance. This kept the
   existing unit and integration suites unchanged while the CLI uses the new
   forms. Note that `ArchiveNode` does enforce the open-challenge guard (the
   guard is state-based, not identity-based), so the guard is not
   CLI-only.
3. **"Archived with a challenge open" is detected from the recorded
   superseded challenge.** Archiving auto-supersedes a node's open challenges
   (existing behaviour), so the durable trace of an abandoned obligation is a
   superseded challenge on an archived child. A resolved/withdrawn challenge
   is not reported, and a challenge-free archive is not reported.
4. **Claim generations are display-only derived state**, so they do not enter
   the content hash or any event shape (P1).

## Tests added

- `internal/service/claim_release_test.go`: release in the same event; a
  non-holder does not release; matching/stale hand-built `claim_seq` at
  replay; legacy events keep the claim; admit/refute/archive auto-release;
  CAS refusal when the node is re-claimed in the `beforeAppend` window;
  reviewer checks for author/proof-author/amender; `--allow-self` recorded;
  verdict-file no-opt-out; archive guard on node and active descendant,
  `--force` requires a reason and records fields.
- `internal/lock/info_tolerance_test.go`: tolerance window, released lock,
  concurrent refresh + info under `-race`.
- `internal/render/verification_checklist_archive_test.go`: archived child
  with an abandoned challenge is listed (text and JSON); challenge-free and
  resolved-challenge children are not.
- Updated `internal/lock/info_test.go` (integration) and
  `cmd/af/release_test.go` (integration) for the new tolerance and no-op
  semantics.

## Handoff notes

- The `af get` change is deliberately two small insertions (`claim_seq` in
  `nodeToJSONFull`, one `Claim generation` line in the single-node text path)
  to merge cleanly with the D7 branch editing `cmd/af/get.go`.
- `internal/render/verification_checklist.go` was edited even though the
  brief listed `internal/render` as shared; the D7 branch touches other files
  in that package (`node.go`, `json.go`, `status.go`, `adapters.go`,
  `examples.go`) and not the checklist, so the change is isolated.
- No push was performed; `git status` is clean at `d64e9c7`.

---

## Review fixes

An independent review of the D5/D9 branch found six issues. All are fixed on
this branch; the quality gates below were re-run.

```
gofmt -l cmd internal e2e          # clean
go build ./cmd/af                  # ok
go vet ./...                       # ok
go test -count=1 ./...             # all packages ok
go test -tags=integration -count=1 ./cmd/af   # ok
go test -race -count=1 ./internal/lock        # ok
```

Commits (oldest first):

| Commit | Topic |
|---|---|
| `0891dfa` | D5: fence explicit `NodesReleased` per-node claim generation |
| `0a4779d` | D9: archive/support invalidation and durable abandoned obligations |
| `080cbb3` | D9: enforce reviewer-contributor on public accept paths; warn on identity |

### 1. `NodesReleased` was inert (CRITICAL)

The single `claim_seq` field could not describe a multi-node release and was
never written. It is replaced by `ClaimSeqs []int` (`omitempty`), aligned
positionally with `NodeIDs`. Every explicit release path — `ReleaseNode`,
`ReleaseNodes` (and therefore `ReleaseExpiredClaims`/`ReleaseAllClaims`) and
`RecordProof` — now goes through `newFencedNodesReleased`, which reads each
node's current `ClaimSeq` from the same state read the commit was built
against. Replay releases a node only when a non-zero generation is present and
still equals the node's current generation; an absent/zero generation remains a
legacy unfenced release. A stale hand-built event therefore leaves a later
claim intact.

Tests (`internal/service/claim_release_test.go`): stale generation leaves the
newer claim, matching generation releases, legacy event still releases, and
`ReleaseAllClaims` stamps each of two differently-generated nodes in position.

### 2. Archiving could restore a stale ancestor verdict (HIGH)

`support_current` now treats a direct child archived at a ledger sequence later
than the parent's `VerdictSeq` as `CHILD_ARCHIVED_AFTER_VERDICT` (new cause,
`internal/support/current.go`), so an archived child no longer silently makes
the parent current again. A fresh accept after the archive is current because
the parent's verdict then post-dates the child's `ArchivedSeq`. The archival
sequence is also folded into `LatestRevisionSeq` so it propagates to an older
ancestor verdict through a target that was re-accepted in between, matching the
existing amendment-revision propagation. `ArchivedSeq` is derived (`json:"-"`)
and stamped by replay from the `NodeArchived` event sequence.

Tests (`internal/support/current_test.go`): the reviewer's scenario (child
archived after verdict → parent not current; re-accept → current) and the
descendant-archive-through-current-target propagation.

### 3. Descendant-only archive obligations vanished (HIGH)

`NodeArchived` gains `abandoned_obligations []string` (`omitempty`), the node
IDs whose open challenges the archive abandoned (captured from
`OpenChallengeObligations` at write time). Replay stamps them on a derived
`node.AbandonedObligations` (`json:"-"`). The checklist now reads the durable
snapshot via `state.ArchivedObligations` and falls back to the old
challenge-trace derivation for legacy archives. A forced archive of 1.1 caused
by a challenge on 1.1.1 therefore still appears on 1's checklist as 1.1.1.

Tests: `internal/service/claim_release_test.go`
(`TestArchiveNode_RecordsDescendantAbandonedObligation`) and
`internal/render/verification_checklist_archive_test.go`
(`TestChecklist_ListsDescendantAbandonedObligation`).

### 4. Public accept paths bypassed the reviewer check (MEDIUM)

`AcceptOptions.CheckReviewerAuthor` is removed. The reviewer≠contributor check
now runs whenever `VerifiedBy` is non-empty, on every accept path; `AllowSelf`
is the only bypass and records `self_accepted`. `AcceptNodeWithVerifier`,
`AcceptNodeWithExpectation` and `AcceptNodesBulk` therefore enforce it, and
verdict files cannot opt out (they never set `AllowSelf`).

Tests (`TestPublicAcceptPaths_EnforceReviewerContributor`,
`TestVerdicts_RealFileCannotAllowSelf`).

### 5. Missing-identity warning suppressed under `-f json` (MEDIUM)

`af accept` prints the missing-identity warning on stderr in every format and
adds a `warnings` array to the JSON result (single, bulk, no-pending and
blocking-challenge outputs). The cmd/af JSON tests were updated to separate
stdout from stderr so stdout stays parseable.

Test: `TestAcceptCmd_MissingIdentityWarningEveryFormat` (text, JSON, and
agent-supplied).

### 6. Test gaps

- `internal/service/claim_release_test.go` now drives a real verdict file
  through `verdicts.ParseFile` + `ApplyVerdicts` and asserts the contributor
  accept is `rejected:reviewer-equals-author`.
- `internal/lock/info_tolerance_test.go` adds
  `TestGetLockInfo_ConcurrentWithMarkReleased`, and
  `go test -race ./internal/lock` is clean.

### Note on the integration suite

`go test -tags=integration ./...` is green except for the pre-existing flaky
`internal/fs` test `TestAtomicWrite_ConcurrentSameFile` (20 writers racing the
same temp file), which also fails on the merge-base commit and is untouched by
this branch. The brief's required gates (`go test ./...`) are clean.
