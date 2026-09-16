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
