# Handoff - 2026-09-17: v0.1.11 tagged; D6 landed; scale-hardening epic closed

- **main** = v0.1.11. D6 (taint as a fold over the prepared support graph)
  is merged: one Opus review (`docs/plans/reports/D6.md`, "Review fixes"
  section) found a real regression in the plan's own support-relation text
  (a `local_assume` subtree was invisible to taint, so an admitted step
  under a hypothesis left the root validated/clean) plus 8 should-fix/nit
  items; one Opus fix pass resolved all nine. Plan amended to v3.2
  (`docs/plans/scale-hardening.md`, last section): clause (i) is now "t is
  a child of n" with no local_assume exclusion; refuted/archived children
  stay severed for taint (documented, pinned by test); child edges use the
  nearest present ancestor. `support_current` shares the relation and now
  sees local_assume children.
- Checks on the release binary: gates green, corpus check OK (211),
  `scripts/corpus-taint-diff.sh` 0/211, full `export --graph json` diff vs
  the 0.1.10 binary 0/211 (6 corpus workspaces contain local_assume nodes).
- Epic vibefeld-67y5 and vibefeld-cmww closed. GitHub #3: Tobias posted the
  reply; the issue stays open until he is satisfied 0.1.11 landed.

## For Tobias

1. Close GitHub #3 once you have looked at v0.1.11 (`af changelog` has
   the 0.1.11 notes; GitHub releases for 0.1.9-0.1.11 are optional).
2. Follow-up bead vibefeld-e8td (P2): bulk accept / verdicts apply recompute
   taint once per node (~0.9 s of fold for a 100-node bulk accept on a
   1000-node proof, vs ~0.17 s in 0.1.10); recompute once per batch.
3. Known nit left as-is: `af taint-trace` prints the source id twice
   (`1.1 - tainted via child 1.1 (...)`); docs show the real output.
   `docs/prd.md` still describes the pre-D6 taint model (historical).

## Open beads worth knowing

vibefeld-e8td (above); vibefeld-8rjx (fs.WriteNode corrupt-file flake under
load); vibefeld-bsb1 (ledger lock timeout under load); the P2 feature ideas.

## Process notes

Today's cadence per Tobias: Opus subagents (review, then fix pass), Claude
final check, merge; no codex, no fable subagents. Opus review took ~14 min
and found 9 items; fix pass ~27 min. Worktree `../vibefeld-wt-d6` and
branch `work/d6-support-taint` removed after merge.

---

# Handoff - 2026-09-16 (final): v0.1.10 tagged; D6 preserved on a branch for 0.1.11

Tobias asked to wind up at 0.1.10. State at this handoff:

- **main** = v0.1.10 (tag pushed). All 0.1.9 and 0.1.10 items of
  `docs/plans/scale-hardening.md` v3.1 are merged: D0, D10, D1, D3, D2
  (v0.1.9); D4, D7, D11, D5, D9, D8, benchmark job (v0.1.10). Gates green,
  corpus check green on the release binary. Per-item reports incl. review
  fixes: `docs/plans/reports/D{0,1,2,3,4,5-D9,7-D11,8,10}.md`, `BENCH.md`
  (+ `benchmark-results.json`).
- **D6 (0.1.11, bead vibefeld-cmww) is built but NOT reviewed and NOT
  merged.** Branch `work/d6-support-taint` is pushed to origin (9 commits on
  top of the D8 merge, `REPORT-D6.md` at its root, all gates green when
  built). To land it: `git merge main` into the branch, one codex review
  (gpt-5.6-sol xhigh, read-only, prompt on stdin), one fix pass, Claude
  review, corpus taint diff with `scripts/corpus-taint-diff.sh <old-af>
  <new-af>` (taint_state / taint_counts are the only allowlisted export
  changes), merge, bump VersionInfo to 0.1.11 (its changelog entry is on
  the branch, Unreleased), tag.
- Benchmark verdict (`docs/plans/reports/BENCH.md`): all commands linear in
  events (status/jobs/audit ~150 ms at 1000 nodes), zero lock waits at 50
  writers (retries are CAS conflicts by design), bulk accept ~60 ms/node
  (per-node state reload, linear, the known target), directory fsync ~2
  ms/event. Snapshots are not needed for 0.1.11.
- `af audit` over the 211-workspace corpus: 102 SUPPORT_NOT_CURRENT, 21
  AMENDED_NOT_REVERIFIED, 3 VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE
  (current), plus historical classes; zero HASH_MISMATCH after the
  reconstruction fix.

## For Tobias

1. **Post the #3 reply**: text approved, in `docs/plans/issue-3-reply.md`
   (posting via `gh` was blocked by the agent's permission classifier).
2. GitHub releases for v0.1.9 / v0.1.10 if wanted (`af changelog` has the
   notes).
3. Next agent: land D6 as above, then tag v0.1.11 and close epic
   vibefeld-67y5.

## Open beads worth knowing

- vibefeld-cmww (D6, see above); vibefeld-67y5 (epic, stays open until D6).
- vibefeld-8rjx (fs.WriteNode corrupt-file flake under load), vibefeld-bsb1
  (ledger lock timeout under load; benchmark shows no waits in normal use).
- The AF_PREVIOUS_BINARY fixture test runs only when a 0.1.8 binary is
  supplied.

## Process notes (what worked)

Per item: pi deepseek-flash build from a written brief -> one codex
gpt-5.6-sol xhigh read-only review -> one deepseek fix pass -> Claude
review -> merge -> corpus check. Reviews found 4-9 real issues per item;
no second review round was needed. Agents must be launched detached
(`setsid nohup`) with a staleness watchdog; codex needs its prompt on
stdin. Two deepseek accounts were never exhausted; glm was not needed.

---

# Handoff - 2026-09-16 (later): 0.1.10 in progress — D4, D7, D11 merged; D5/D9 under review; D8 building

Continues the v0.1.9 handoff below. Same cadence per item (pi deepseek-flash
build, one codex gpt-5.6-sol xhigh review, deepseek fix pass, Claude review,
merge; corpus check on every merge).

- **D4 merged** (`docs/plans/reports/D4.md`): `internal/support` Prepare/Walk
  (one prepared graph, SCC order, reusable by D6), `support.Current` with
  stable causes and carried descendant revision seqs, `node.VerdictSeq`
  (derived, json:"-"), shared `checkAcceptEligibility`, prerequisite-scheduled
  bulk accept in one commit with per-item outcomes (exit 5 partial), creation
  gate on validated/admitted/refuted/archived parents, needs_refinement
  classifier. Closed 4tjt, hspn, gxa7.
- **D7 + D11 merged** (`docs/plans/reports/D7-D11.md`): health has no
  "probably false"/subtree alarm; per-node rework hotspots; blockers = open
  challenges, stalled claims (dedicated `--claim-stall`, last-activity
  tracked), stale claims, support failures. One jobs classifier for
  status/get/export/health; `verifier_ready` = children cleared everywhere;
  help examples parsed by a test; `af get` shows validated_by, batch id,
  claim fields; `scripts/auto-prove.sh` uses `--with-note`, identities, a
  fail-closed completion test (root validated + support_current true + clean
  taint + no validated node citing a pending external); stub-agent test in
  e2e (integration tag). Closed 429g, 7ze3, ywsu, ujp4, c8yb, xr7g, 0l3d.
- **D5 + D9** (`work/d5-d9-claims-guardrails`, worktree `../vibefeld-wt-d5`,
  beads 562f, epu4): built and green; codex review in progress at handoff
  time. Fenced auto-release (`claim_seq`/`release_claim` optional fields on
  terminal events, `node.ClaimSeq` derived), identities on admit/refute/
  archive, `af release` no-op after auto-release, uj18 lock fix, archive
  guardrail (`--force --reason`, `reason`/`forced` recorded, abandoned
  obligations on the parent checklist), reviewer≠contributor validator,
  `--allow-self` recorded, missing-identity warning.
- **D8** (`work/d8-audit`, worktree `../vibefeld-wt-d8`, bead fza2): building
  at handoff time: `internal/audit` + `af audit [--strict]` with the strict
  codes, historical classes, migration preflight/postcheck summary in
  amend-deps, health reading from the findings engine.

Remaining for 0.1.10 after D5/D9 and D8 merge: benchmark job (plan D10
paragraph; 100 and 1000 nodes, 10-50 writers, p50/p95, lock waits), then
bump VersionInfo to 0.1.10 (changelog entry exists, Unreleased: true), tag.
Then 0.1.11 = D6 (taint over the prepared support graph, differential fuzz).

Flakes filed: vibefeld-8rjx (fs.WriteNode), vibefeld-bsb1 (ledger lock
timeout under load since per-event fsync).

---

# Handoff - 2026-09-16 (v0.1.9 tagged: scale-hardening plan v3.1 0.1.9 set complete)

All five 0.1.9 items (D0, D10 reduced, D1, D3, D2 + manifest) are merged on
main, `VersionInfo` is 0.1.9, tag `v0.1.9` is created, and the 211-workspace
corpus check passes on the release binary. Each item: pi deepseek-flash
implementation -> one codex gpt-5.6-sol xhigh review -> deepseek fix pass ->
Claude review -> merge. Reports in `docs/plans/reports/D{0,1,2,3,10}.md`.
Worktrees and work/ branches removed after merge.

End-to-end check on the release binary (`docs/amend-deps-example.md`, fixed
to the real `refine` syntax): manifest dry-run, real run, `--resume` reports
applied(already); on a validated node `amend-deps` without `--reopen` is
refused (exit 3), a stale `--expect-hash` is refused, and `--reopen` with the
right hash lands one `node_deps_amended` event that leaves the node pending
with the corrected edge; `af replay --verify` valid.

## For Tobias

1. Post the reply on GitHub #3 (`docs/plans/issue-3-reply.md`); it can now
   point at v0.1.9 and `docs/amend-deps-example.md`. Ask for the ledger.
2. `git push --tags` was done; create the GitHub release for v0.1.9 if wanted
   (`gh release create v0.1.9 --notes-from-tag` or from the in-binary
   changelog `af changelog`).
3. Decide 0.1.10 order. Suggested: D4 (support_current + shared validator;
   closes hspn, gxa7) first, then D7 (ywsu), D8 strict subset, D11 (ujp4),
   D5 (kbzm, uj18), D9. File beads per item under epic vibefeld-67y5.

## Known follow-ups filed / noted

- vibefeld-8rjx: fs.WriteNode concurrent corruption flake.
- D2 report: dangling-edge removal now allowed; `node_amended_reopened` is a
  distinct 1.1 event; `request_fingerprint` binds operation ids.
- The AF_PREVIOUS_BINARY fixture test asserts the unknown-event error only
  when a 0.1.8 binary is supplied; CI does not supply one yet.

# Handoff - 2026-09-16 (scale-hardening v3.1 adopted; D0, D10, D1, D3 merged; D2 in flight)

Plan v3.1 adopted (last section of `docs/plans/scale-hardening.md`): 0.1.9 is
the minimum set (D0, D10 reduced, D1, D3, D2 + manifest); D4/D7/D8/D11 move
to 0.1.10; D0 is optimistic (state read outside the ledger lock, batch tail
check under it); D10 drops the multi-binary matrix. Draft reply to GitHub #3
is in `docs/plans/issue-3-reply.md` (Tobias to post).

## Merged into main this session (each: pi deepseek-flash implementation,
## one codex gpt-5.6-sol xhigh review, fixes, Claude review, merge)

- **D10** (`work/d10-format`): `config.FormatCurrent`="1.1", `CheckFormat` at
  every entry point (service ctor, replay, and the direct-ledger commands via
  `cmd/af/workspace_open.go`), `ledger.RegisterEventMinFormat`,
  `af workspace upgrade --to 1.1 [--dry-run]` (lock first, exclusive backup
  dir, fsync root before stamp), `af version` advertises format/policy,
  `af replay -f json` exits 4 on failure, `scripts/corpus-manifest.sh` +
  `corpus-check.sh` + `docs/corpus-manifest.txt` (211 workspaces, all pass).
  New workspaces are stamped 1.1. Closed q6os, xk7c.
- **D0** (`work/d0-commit-primitive`): `internal/service/commit.go`
  `commit(build)` is the only write path; `ledger.AppendBatchIfSequence`
  (batch-wide CAS, per-rename dir fsync, prefix on crash); `operation_id` on
  BaseEvent + `state.HasOperationID` + `findOperation`; lock file carries
  pid + token, `Release` checks the token, `RemoveIfStale`, `af reap
  --ledger-lock`; verdicts apply gates and accept share one state read;
  ReleaseNodes/UnvalidateBatch/Init/ExtractLemma preconditions inside the
  closure; format gate runs inside commit. Closed qgjg, tlh1.
- **D3** (`work/d3-accept-hash`): `NodeValidated.content_hash` /
  `expected_hash_checked`, `ClaimTested.content_hash`, `af accept
  --expect-hash`, stale claim tests do not count (`CLAIM_TEST_REQUIRED` code,
  exit 2), export fields `validated_content_hash(_checked)`. Closed w3vz.
- **D1** (`work/d1-support-relation`): `internal/support` (result-use /
  hypothesis-use edges, branch-local structural scopes with pre/post-close
  context for discharges, `CheckCreation` over the whole prospective batch,
  new-edge-only cycle rejection, re-validation of existing nodes whose scope
  changed, `SCOPE_LEAK` code); wired into Refine, RefineNodeBulk, RecordProof,
  CreateNode via `service.checkSupportBatch`; ParentID/ChildID divergence
  rejected. Closed k336, 0ko0.

Reports: `docs/plans/reports/D{0,1,3,10}.md`. Reviews and briefs live in the
session scratchpad only.

## In flight

- **D2** `af amend-deps` (`work/d2-amend-deps`, worktree `../vibefeld-wt-d2`,
  bead 4hut): brief covers the event, service, CLI, manifest with
  `--dry-run/--resume`, surfaces, export fields, tests, docs.

## Next steps

1. D2: gates, codex review, fixes, Claude review, merge. Add a real 1.1 event
   to `e2e/fixtures/format-1.1` and tighten the previous-binary test.
2. Tag v0.1.9 after bumping VersionInfo (changelog 0.1.9 entry is marked
   unreleased; the version test skips unreleased entries). Update #3.
3. 0.1.10: D4 (support_current; one DAG walk shared with D6), D7, D8, D11,
   D5, D9. File beads per item under epic 67y5.
4. Open bug filed this session: vibefeld-8rjx (fs.WriteNode concurrent
   corruption flake).

## Operational notes

- pi launch that passes the auto-mode classifier: brief in a file,
  `pi -p --provider deepseek --model deepseek-flash --session-dir <dir> -n <name> @brief.md "Implement the brief."`,
  run under `setsid nohup ... & disown` (the harness killed wrapper shells
  and left orphaned agents editing the same worktree once). codex reviews:
  `codex exec -m gpt-5.6-sol -c model_reasoning_effort=xhigh --sandbox read-only -o out.md - < prompt.md`
  (must feed stdin, otherwise it blocks on "Reading additional input").
- Worktrees: `../vibefeld-wt-d{0,1,2,3,10}`; delete after merge.

---

# Handoff - 2026-09-15 (scale-hardening plan v3; no code changes)

Planning session only. No source changed; `gofmt`, `go vet`, `go build`,
`go test ./...` all green at HEAD.

## What was done

- Surveyed repo + GitHub. One open GitHub issue, **#3** (vidick, MIP\*=RE
  run: 123 nodes, 291 challenges, 1732 events): dependency edges are
  write-once, 21 validated nodes have wrong edge lists, proposes
  `af amend-deps`, asks for a decision (A build it / B defer to v0.2 /
  C recreation is intended). No open PRs. **Needs a reply from Tobias.**
- Wrote **`docs/plans/scale-hardening.md` (v3)**: the plan for making af
  solid at 100-1000 node DAG scale on the 0.1.x line without waiting for
  v0.2. Reviewed twice by codex gpt-6-astra xhigh; both reviews are saved
  beside it (`scale-hardening-review-codex.md`,
  `scale-hardening-rereview-codex.md`). v3 answers every point of the
  re-review; the last section of the plan says what changed and why.
  Recommended answer to #3: option A, `amend-deps` pending-only with an
  explicit `--reopen` that unvalidates and amends in *one event*.
- Measured: synthetic 1001-node / 3202-event workspace, every command
  < 0.25 s with full replay, linear. Read performance is not the problem.
- Epic **vibefeld-67y5** tracks the plan. Bugs found and confirmed this
  session, filed: **tlh1** (P1, verdicts-apply two-read race +
  appendBulkIfSequence CAS only on first event), **gxa7** (P1, refine on a
  validated parent leaves it validated with pending children), **ywsu**
  (health fatigue alarm is absolute; MIP\*=RE root alarms permanently),
  **ujp4** (auto-prove.sh uses `accept --note`, flag is `--with-note`;
  success judged on root state alone), **xk7c** (replay JSON exits 0 on
  failure; config.Validate never called so meta version is unchecked).
- Discrepancy: beads 0ry1, a7p5, 5qrx, gwps, 8x16, w2mt, 0zk7 cited by the
  0.1.7 handoff below and by `docs/trust-model.md` **do not exist** in
  `.beads/issues.jsonl`; commit 392b2da only touched `interactions.jsonl`.
  Recreate them under vibefeld-67y5 when the plan is adopted.

## Next steps

1. Tobias decides on #3 and on the plan (or asks for a third review pass).
2. Reply on #3 (plan "Order of work" step 0); ask for a ledger copy.
3. File one bead per plan item D0-D11 under vibefeld-67y5; start with D0
   (tlh1) and D10, which everything else depends on.
4. Session-close protocol as usual; no tags exist yet, v0.1.9 is the first.

---

# Handoff - 2026-09-14 (integration suite green; artifacts; auto-prove portability)

Follow-ups to the 0.1.8 notes below. No version bump, no tags.

- `go test -tags integration ./...` compiles and passes: 28 packages, 7906
  PASS, 1 SKIP. Nearly all failures were stale tests (API moved on, tagged
  files never updated): duplicate symbols against untagged copies,
  `NewAssumption`/`NewPendingDef` returning errors, lock ClockSkewTolerance,
  derived taint (0.1.7), removed `refine --statement`/`--sibling`,
  `--yes` on archive/refute, and children-before-parent accept. Closes
  vibefeld-hitt.
- Real bugs fixed along the way:
  - `lock.ClaimLock.Release` self-deadlocked: it held `l.mu` and called
    `IsExpired`, which takes `l.mu`. No production caller.
  - `af refine` reported a wrong-owner refine as "parent node is not claimed"
    (`ErrNotClaimed`/`ErrOwnerMismatch` share a code; `AFError.Is` compares
    codes). The hint also still used the removed `-s`.
  - `af refine <id> "stmt"` had lost its warn_depth warning and the "add
    breadth instead" max-depth hint since 88dd188. The warning now goes to
    stderr, so JSON stdout stays clean.
- Flaky `TestScopeAcceptance_FullWorkflow`: `State.AllNodes()` is map order
  and `scope.ValidateScopeBalance` is order-sensitive. The test sorts by
  NodeID now; 20/20 and 100/100 pass.
- Skipped, needs a decision: `TestRefineCmd_WithDependsFlag_DependOnParent`
  (vibefeld-0ko0: the Refine cycle check treats a child citing an ancestor as
  a cycle).
- Removed committed artifacts `af.test`, `hooks` (stray ELF build, not a git
  hook), `coverage*.out`; `.gitignore` covers them.
- `scripts/auto-prove.sh`: `timeout`, then `gtimeout`, then a bash watchdog
  (exit 124 on timeout), plus a bash >= 4 guard (stock macOS bash is 3.2).
- AISM corpus (now 211 workspaces): `af replay --verify` + `af export --graph
  json` output byte-identical between 251f576 and this HEAD (no ledger/replay
  changes were made anyway).
- New issues: vibefeld-hspn (P1, `af accept 1 1.2` / `--all` validates a
  parent with pending children), vibefeld-0ko0 (decision), vibefeld-uj18
  (LockInfo ignores tolerance + unsynchronized reads), vibefeld-xr7g (root
  help uses removed `refine -s`; integration tag undocumented).

---

# Handoff - 2026-09-14 (0.1.8: module path + build.sh portability, GitHub #2)

- Go module path renamed to `github.com/tobiasosborne/vibefeld` (go.mod, all
  self-imports, docs). `go install github.com/tobiasosborne/vibefeld/cmd/af@main`
  now works from outside a checkout.
- `scripts/build.sh` reads VersionInfo with POSIX `sed -n` instead of GNU-only
  `grep -oP` (macOS BSD grep has no `-P`); output byte-identical on GNU.
- Version bumped to 0.1.8 (source + in-binary changelog). No git tags exist;
  cutting one (e.g. v0.1.8) is an open decision for Tobias.
- Not fixed, noted: `af.test` and `hooks` are stale ELF binaries committed at
  the repo root (along with `coverage*.out`); `go test -tags integration ./...`
  was already broken before this change (build failures in fs/node/render/
  service/taint test files, failures in cmd/af, e2e, lock, state);
  `scripts/auto-prove.sh` uses GNU `timeout` (not on stock macOS).

---

# Handoff - 2026-09-02 (0.1.7: taint propagates upward; trust model; v0.2 target)

## What Was Accomplished

**Bug (vibefeld-0zk7, reported by Thomas via email after running af 0.1.6 on
MIP*=RE):** admitting a child left the validated root's taint `clean`. Root
cause: `internal/taint.ComputeTaint` only read ancestors (top-down only)
while docs and the `af accept` warning promised bottom-up propagation.

**Fix (0.1.7):** taint is now two separate components — an ancestor-chain
component derived from ancestors' *epistemic* states (never their stored
taint) and a subtree component walked deepest-first (archived/refuted
children sever the branch, admitted children contribute `tainted` without
descending, pending/draft/needs_refinement contribute `unresolved`). Keeping
the components separate means a validated sibling of an admitted node stays
`clean`. Rule order 0-7 is in `docs/concepts.md`. Replay ends with one
authoritative `taint.RecomputeAll`, so 0.1.6 workspaces self-heal on load;
per-event recompute in `state.Apply` was removed (it had made replay 4x
slower). `af recompute-taint` now re-syncs the ledger's TaintRecomputed
audit trail against derived state. `af taint-trace` names descendant sources.
`needs_refinement` now counts as unresolved (Tobias-approved semantic change).

**Process:** implemented by codex exec (gpt-5.6-sol, xhigh) from a written
brief; two Opus review rounds (differential fuzz vs an independent spec
implementation over 3000 + 5000 random trees, zero mismatches; end-to-end
checks on the built binary). Review findings and reports are in
`~/.local/state/vibefeld/taint-fix/`.

**Also:** `docs/trust-model.md` (where an agent can "win" without rigor:
archive-the-hard-step, cross-references carry no taint, roles unenforced,
ledger not hash-chained; each with a beads issue). `docs/prd.md` gained a
"v0.2 Target: Trust at Scale" section (LCF-style trusted kernel / untrusted
shell; executable spec + differential fuzz; TLA+ model of ledger + locks;
kernel boundary; assurance/language decision deferred) tracked as epic
vibefeld-w2mt.

## Current State
- `gofmt -l .` clean, `go vet ./...` clean, `go build ./cmd/af` ok,
  `go test ./...` all 28 packages green.
- `go test -tags=integration ./e2e`: the taint/replay tests pass (8 of them
  were failing at the previous HEAD); 16 lock/reap/concurrency tests fail
  identically at the previous HEAD (pre-existing, environmental).
- Two commits: whitespace-only gofmt drift in 65 files (separate), then the fix.

## Next Steps (prioritized)
1. Load Tobias's ~200-lemma DAG under 0.1.7: `af status` / `af taint-trace 1`
   will reveal any admitted lemmas that were invisible on 0.1.6.
2. vibefeld-0ry1 (P1): taint along reference dependencies / external refs —
   the biggest gap for DAG-shaped arguments. Candidate 0.1.8.
3. vibefeld-a7p5 (P1): refuse/flag archiving a node with an open challenge.
4. vibefeld-5qrx: read-only `af audit` script.
5. Review nits filed as P3 tasks (snapshot hoisting, taint-trace helper
   reuse, replay perf guard test).
6. v0.2 epic vibefeld-w2mt when ready.

## Key Files Changed
internal/taint/{compute,propagate}.go, internal/state/{apply,replay}.go,
internal/service/proof.go (+ lastAuditedTaintStates), cmd/af/{taint_trace,
recompute_taint,accept,schema,changelog,version}.go, internal/render legends,
docs/{concepts,prd,cli-reference,state-machines,trust-model}.md, README.md.

---

# Handoff - 2026-07-19 (rk V2/V3: af verdicts apply + af unvalidate --batch)

## What Was Accomplished This Session

Subagented from `../rk` (research-workflows IMPLEMENTATION_PLAN.md vibefeld
table, items V2 and V3, queued as `vibefeld-lzop`/`vibefeld-h4ad`) — the two
items the previous V0/V1 session (below) explicitly left for a future
session. Both closed.

**V2 (`af verdicts apply <file>`).** New pure schema package
`internal/verdicts` (File/Item types, `ParseFile`/`Validate`): schema_version
+ batch_id + verified_by + an ordered item list, each item
`accept|challenge(target,severity,reason)` with a MANDATORY non-empty
`reason` on every item regardless of verdict — the file-schema enforcement
of "no blanket accepts." Versioned independently of rk's own
(concurrently-drafted) `schemas/verdict.v1.json`; aligned in spirit, not
wire-identical — see `docs/verdicts-apply.md`'s seam note.

Application lives in `internal/service/verdicts_apply.go`
(`ProofService.ApplyVerdicts`): applies items in **file order**, never
reordering — that order is what makes "children before parent" accepts a
real constraint rather than an assumption. Every item becomes a normal
per-node event (`AcceptNodeWithVerifier` / the new CAS-protected
`RaiseChallengeWithBatch`, both carrying `f.VerifiedBy`/`f.BatchID`) — never
a wholesale subtree accept. Per-item outcome is exactly one of `applied` /
`blocked-by:<reason>` / `rejected:<reason>`; the batch applies what it can
and only hard-aborts on a genuine concurrent-modification race (remaining
items recorded `blocked-by:batch-aborted`, never silently dropped — every
item in the file always gets exactly one report entry).
**Reviewer≠author**: an accept item whose `verified_by` equals the target
node's recorded `Author` is rejected per-item
(`rejected:reviewer-equals-author`) before the kernel path even runs — PRD
C3 scopes this to accepts only, not challenges. Same provenance caveat as
V1: recorded-and-checkable, not adversary-proof; an empty `Author` (legacy
node) can never falsely trigger it.

Aggregate outcome selects new exit codes: 0 (all-applied), 5
(`VERDICTS_PARTIALLY_APPLIED`), 6 (`VERDICTS_NONE_APPLIED`); the file itself
being invalid or unreadable is 3 (`VERDICTS_FILE_INVALID`), joining the
existing logic-error tier — nothing is attempted from a file that fails
`ParseFile`.

**V3 (`af unvalidate --batch <id>`).** Added `--batch` to the existing
`af unvalidate` command (mutually exclusive with the node-id positional
arg). `ProofService.UnvalidateBatch` finds affected nodes via a **state
scan** (`Node.ValidationBatchID`, set by V1) — not a ledger rescan — and
revokes each through the existing `UnvalidateNode` path (normal, attributed
`NodeUnvalidated` events, `ValidatedBy`/`ValidationBatchID` cleared exactly
as the single-node form already did). A batch id matching no
currently-validated node is a clean no-op (`ErrUnvalidateBatchNotFound`,
exit 7), not a crash or silent success.

**Real bug found and fixed via red-green** (not a pre-existing issue, one I
introduced and caught before it landed): `ErrClaimTestRequired` and
`ErrBlockingChallenges` both carry the same underlying `NODE_BLOCKED`
`aferrors` code, and `AFError.Is` compares codes only — so
`errors.Is(err, ErrBlockingChallenges)` matches a claim-test error too.
Classification now checks the claim-test message substring first (same
technique `cmd/af/accept.go` already uses), `errors.Is(ErrBlockingChallenges)`
only as the fallback once claim-test is ruled out.

**Tests, all red-green, several mutation-proven** (perturb → RED confirmed
→ restore → GREEN — done live for the reviewer≠author check and the
mandatory-justification parse rule): order-dependence (parent-before-child
blocks; child-before-parent both apply), mid-batch challenge blocking a
later accept (both the collateral children-not-validated path and a direct
pre-existing blocking-challenge), reviewer==author rejection (and its
negative: a genuinely different reviewer succeeds), node-not-found,
aggregate all/partial/none-applied selection, file-invalid parsing
(missing/wrong schema_version, missing batch_id/verified_by, empty items,
blanket-accept and empty-challenge-reason rejection, invalid
verdict/target/severity, duplicate node in one file, unknown fields,
trailing content), and the V3 round-trip (apply a batch, unvalidate it,
derived state returns exactly to pre-batch — including
`ValidatedBy`/`ValidationBatchID` cleared; a second unvalidate of the same
id is the clean no-op). CLI-level tests cover flag wiring, exit codes, and
JSON report shape for both commands. `go build ./... && go vet ./... &&
go test ./...` all green (one flaky, pre-existing, unrelated
`internal/fs` concurrency test observed and reproduced-clean on rerun — not
touched by this session).

**Replay-regression evidence**, same method as the V1 session: built `af`
from HEAD before this session's commits and from the finished tree, ran
`af replay --dir <ws> --verify --format json` and
`af export --graph json --dir <ws>` against all 44 AISM workspaces both
times, `diff -rq` the two output trees — **zero bytes differ** (no new
ledger event schema was added; V2/V3 only reuse V1's existing
`VerifiedBy`/`BatchID` fields). Also live-fired end-to-end against a real
AISM-derived workspace copy (never touching the read-only original): built
a real verdict file, ran `af verdicts apply` against it, confirmed
`validated_by`/`validation_batch_id` in `af export --graph json`, ran
`af unvalidate --batch` to revoke it, confirmed the node returned to
pending with both fields cleared, and confirmed a second `--batch` call on
the same id is a clean exit-7 no-op.

**Docs**: `docs/verdicts-apply.md` (new) is the file schema + partial-
failure-semantics + exit-code reference rk's M3.4 driver needs, plus a
driver checklist. `docs/cli-reference.md` gains `verdicts apply` and
(previously wholly undocumented) `unvalidate` sections including `--batch`,
plus the exit-codes table extended to 5-7. `docs/contributing.md`'s
error-code table likewise extended.

**Known pre-existing gap, not touched**: `af get <node>` (single-node
display) does not surface `validated_by`/`validation_batch_id` — only
`af export --graph json` does (a V1 addition). Confirmed during live-fire
testing; filed as a follow-up, not blocking, since the graph export is the
authoritative machine-readable surface these fields were built for.

**Commits**: `7bdaf85` (exit codes 5-7), `ee5e6e8` (`internal/verdicts`
schema), `598d331` (`ApplyVerdicts`/`UnvalidateBatch` kernel surface),
`eb20a75` (CLI wiring), `658eb99` (docs).

**Next steps for rk**: nothing outstanding on the af side for M3.4's kernel
half. rk's own driver (C9, TS) needs to: compose batches respecting the cap
(~10) and independence/critical-path-exclusion guardrails (af does not
enforce these — driver responsibility per the plan); order `items` so every
accept's dependencies already appear earlier in the file or in
already-cleared state; query authorship before composing a batch to avoid
burning a slot on a guaranteed `rejected:reviewer-equals-author`; and use
`af verdicts apply --format json` / `af unvalidate --batch --format json`
for machine consumption rather than parsing text. Full checklist in
`docs/verdicts-apply.md`'s final section.

---

# Handoff - 2026-07-19 (rk V0/V1: author + verifier identity)

## What Was Accomplished This Session

Subagented from `../rk` (research-workflows IMPLEMENTATION_PLAN.md vibefeld
table, items V0 and V1) — kernel schema groundwork rk's M3.4 batch
verification needs. `V2` (`af verdicts apply`) and `V3` (`af unvalidate
--batch`) are explicitly NOT done here; they consume what this session
built.

**V0 (struck, documented).** `../firstproof` no longer exists locally, no
configured git remote references it. Corpus is AISM's 44 workspaces,
read-only at `../almost-idempotent-stochastic-maps/proofs/`. Recorded in
`CONTRIBUTING.md`'s new "Historical replay corpus" section.

**V1 (done).** Real schema addition, not an optional-fields patch:
- `node.Node.Author` (`internal/node/node.go`) — driver-supplied identity
  recorded at creation. Root node: from `Init`'s `--author`. Refined
  children: the claiming `Owner` (`Refine`/`RefineNodeBulk`).
- `ledger.NodeValidated.VerifiedBy` + `.BatchID`, `ledger.ChallengeRaised.BatchID`
  (`internal/ledger/event.go`) — new `NewNodeValidatedFull` /
  `NewChallengeRaisedWithBatch` constructors; existing constructors
  unchanged (empty verifier/batch).
- `state.apply.go` projects these onto `Node.ValidatedBy` /
  `Node.ValidationBatchID` (cleared on unvalidate) and
  `state.Challenge.BatchID`.
- `service.AcceptNodeWithVerifier` / `AcceptNodeBulkWithVerifier` — the
  kernel surface V2 should call directly with a batch's id.
  `AcceptNode`/`AcceptNodeWithNote`/`AcceptNodeBulk` now thin wrappers.
- `cmd/af/accept.go`'s pre-existing `--agent` flag now also records
  `VerifiedBy` (previously used only for the challenge check).
- `af export --graph json`: `author`/`validated_by`/`validation_batch_id`
  added to `GraphNode`, additive under `docs/export-graph-v1.md`'s existing
  future-additive rule — `schema_version` unchanged at `"1"`.

All three new fields are explicitly documented as DRIVER-SUPPLIED
PROVENANCE (recorded, mechanically checkable) — NOT adversary-proof
enforcement, everywhere they're touched.

**Replay-regression evidence:** built `af` before and after, ran
`af replay --dir <ws> --verify --format json` and
`af export --graph json --dir <ws>` against all 44 AISM workspaces both
times, `diff -rq` the two output trees — **zero bytes differ**. Content
hash exclusion of the new fields (`ComputeContentHash` untouched) plus
`omitempty` on every new field is why: old ledgers replay to identical
node/challenge state and re-export byte-identically.

**Tests:** red-green with mutation-proving (perturb → RED confirmed →
restore → GREEN) on: `TestNode_ContentHash_ExcludesAuthorAndVerifierFields`,
`TestApplyNodeValidated_RecordsVerifierAndBatchID`,
`TestNodeValidated_WireFieldNames`,
`TestProofService_Refine_RecordsAuthorAsOwner`. Full list of new tests in
the three commits below. `go build ./... && go vet ./... && go test ./...`
all green.

**Known issue not touched (per instruction):** `-tags integration` suite
remains broken (`vibefeld-hitt`). Several files I added tests to have
untagged counterparts already in that broken suite
(`internal/ledger/event_test.go`, `internal/service/proof_test.go`,
`cmd/af/accept_test.go`) — I added matching assertions there too (dead
under the current tag, live if it's ever fixed) AND separately in files
that already run under the default `go test ./...` bar
(`internal/ledger/identity_events_test.go` — new,
`internal/service/service_test.go`, `internal/export/graph_test.go`).

**Commits:** `9581b68` (node/ledger/state schema),
`41a2379` (service + accept.go wiring), `c781fcd` (export additive
fields), `2a21c6e` (V0 docs).

**Next steps for rk / a future session:** V2 (`af verdicts apply`) should
call `AcceptNodeWithVerifier`/`AcceptNodeBulkWithVerifier` with the batch's
own id for every accept, and `NewChallengeRaisedWithBatch` for every
challenge in a mixed verdict list, so a batch's entire verdict set shares
one `BatchID` regardless of per-item outcome. V3 (`af unvalidate --batch
<id>`) can find affected nodes via `Node.ValidationBatchID` in current
state, no ledger rescan needed.

---

# Handoff - 2026-07-02 (Session 237)

## What Was Accomplished This Session

Worked the af-binary section of the aism campaign feedback
(`../almost-idempotent-stochastic-maps/docs/tooling-feedback/AF-FEEDBACK.md`) end to end: the P0
dry-run bug (0.1.4) plus all three P2 feature requests (0.1.5). All four struck in that file.

### Session 237 Part 2: AF-FEEDBACK P2 items #2/#3/#4 (0.1.5)

**#2 — `af jobs --ready` (vibefeld-r0k9).** Server-side bottom-up-ready verifier filter: only
verifier jobs whose direct children are all cleared (validated/admitted/archived — the same
allowlist `af accept` uses). Predicate `AllChildrenCleared` + `FilterReadyVerifierJobs` in
`internal/jobs/verifier.go` (unit-tested, default gate), re-exported via `service`, wired to a
`--ready` flag in `cmd/af/jobs.go`. `--ready` drops prover jobs; `--ready --role prover` errors as
contradictory. Live-verified: root excluded while children pending; becomes the sole ready job once
both children validated.

**#3 — `af init` writes a workspace `.gitignore` (vibefeld-kf3o).** Implemented in
`internal/fs/init.go` (`workspaceGitignore`, idempotent like meta.json — never clobbers a user
file). **Correctness catch:** the feedback said "track only ledger/ + externals/ + meta.json", but
`AddAssumption` is filesystem-primary (writes `assumptions/`, no ledger event, read from disk in
`LoadState`), so assumptions/ MUST be tracked — ignoring it would drop assumption data. Verified via
`grep` of write-callers: `WriteNode`/`WriteDefinition`/`WriteLemma` have ZERO callers (nodes/, defs/,
lemmas/ are always-empty vestigial dirs), `WriteAssumption`/`WriteExternal` are live. Final gitignore:
ignore `locks/ .af/ nodes/ defs/ lemmas/`; track `ledger/ assumptions/ externals/ meta.json`.

**#4 — machine-readable challenge `category` (vibefeld-twdf).** Typed, optional field threaded
through all layers: new `internal/schema/category.go` (enum: gap/missing/dependency/incorrect/
unclear/other; empty allowed; validated), `ChallengeRaised.Category` + `NewChallengeRaisedFull`
factory (WithSeverity delegates with empty category — backward compatible), `state.Challenge.Category`,
`applyChallengeRaised` validates + sets it, service re-exports, `af challenge --category` (validated,
shown in text/JSON), `af challenges --category <x>` filter + `category` in JSON. Live-verified:
raise/persist, invalid rejected, JSON carries it, filter narrows correctly.

**Files changed (Part 2):**
- `internal/jobs/verifier.go` + `internal/jobs/verifier_test.go` — ready predicate + tests
- `internal/fs/init.go` + `internal/fs/init_test.go` — workspace .gitignore + tests
- `internal/schema/category.go` + `internal/schema/category_test.go` — NEW category enum
- `internal/ledger/event.go` — `ChallengeRaised.Category` + `NewChallengeRaisedFull`
- `internal/state/state.go`, `internal/state/apply.go` — Category field + validate/set
- `internal/service/exports.go` — re-export category validators + ready filter
- `cmd/af/challenge.go`, `cmd/af/challenges.go` — `--category` on both, JSON, filter, help
- `cmd/af/jobs.go` + `cmd/af/jobs_ready_test.go` — `--ready` flag + wiring test
- `cmd/af/challenge_category_test.go` — NEW category integration tests
- `cmd/af/main.go` + `cmd/af/changelog.go` — version 0.1.4 → 0.1.5, changelog

**Quality gates:** all 27 packages pass, clean build + vet, `af --version` = 0.1.5, installed.
**bd issues:** vibefeld-r0k9, vibefeld-kf3o, vibefeld-twdf (all P2, closed).

**Remaining AF-FEEDBACK items:** only the in-repo driver-script items (#5-#10) targeting
`scripts/af-orchestrate.py` in the aism repo — NOT this repo. The entire af-binary section is now done.

---

### Session 237 Part 1: Fixed --dry-run global no-op (vibefeld-52ff, 0.1.4)

Field feedback from the aism campaign (`../almost-idempotent-stochastic-maps/docs/tooling-feedback/AF-FEEDBACK.md`, P0 #1) reported `af def-add --dry-run` writing a duplicate definition. Investigation showed the bug was **far broader than def-add**.

**Root cause.** `--dry-run` and `--verbose` were registered as global persistent flags (`cmd/af/main.go:167-168`) and advertised in `af --help` (`main.go:129-130`), with helpers `isDryRun()`/`isVerbose()` — but **neither helper was called by any command**. Both flags were dead across the entire CLI. Every mutating command (`def-add`, `refine`, `accept`, `admit`, `challenge`, `archive`, …) silently accepted `--dry-run` and wrote anyway. A correctness footgun for scripted orchestration.

**Fix (opt-in guard + real preview).**
- `dryRunGuard` on `rootCmd.PersistentPreRunE` (`main.go`): if `--dry-run` is set and the command has not opted in, it **errors loudly before any write** ("--dry-run is not supported by \"af refine\": it would still modify the workspace…") instead of silently mutating. Refusing the flag is strictly safer than the old silent-ignore.
- Opt-in via `markDryRunSupported(cmd)` / `supportsDryRun(cmd)` (annotation `af.dryRunSupport`).
- `def-add` opts in and implements a genuine preview (`previewDefAdd` in `cmd/af/def_add.go`): validates inputs, then prints `[dry-run] Would add definition '<name>' (no changes written)` and **skips `svc.AddDefinition`**. Also loads state and **warns when the name already exists** (the exact duplicate-key symptom reported). Supports `--format json` (`{"added":false,"dry_run":true,"existing":<bool>,"name":...}`).

**Design decision.** The guard rejects `--dry-run` for ALL non-opted commands (including read-only ones) rather than trying to classify "mutating" — honest and simple. Extending real dry-run to other mutating commands (`refine`, `accept`, …) is a clean follow-up; each just opts in and adds a preview branch. `--verbose` is likewise still a dead flag (not a correctness bug since it doesn't mutate) — deferred.

**Files changed (4):**
- `cmd/af/main.go` — `dryRunGuard`, `supportsDryRun`, `markDryRunSupported`, `dryRunSupportAnnotation`; wired `PersistentPreRunE`; version 0.1.3 → 0.1.4
- `cmd/af/def_add.go` — `markDryRunSupported(cmd)` in constructor; dry-run branch + `previewDefAdd`
- `cmd/af/dry_run_test.go` — NEW: 6 tests (guard blocks unsupported / allows supported / inert without flag; def-add supports flag / doesn't write / warns duplicate). Non-integration, run by default `go test ./...`.
- `cmd/af/changelog.go` — 0.1.4 entry

**Also:** struck AF-FEEDBACK.md P0 #1 as fixed (note the fix version) per that file's update policy. Reinstalled binary (`go install`) → `/home/tobias/go/bin/af` now 0.1.4.

**Live smoke test** (`/tmp/af-dryrun-smoke`): def-add --dry-run previews + doesn't write (`defs` shows "No definitions found"); real add then --dry-run same name warns duplicate and count stays 1; `refine 1 --dry-run` errors with exit 1 and adds no children.

**Quality gates:** all 27 packages pass, clean build, clean vet, `af --version` = 0.1.4.

**Remaining AF-FEEDBACK af-binary items (not yet done, P2):** #2 `af jobs --ready` (all-live-children-validated filter), #3 `af init` should drop a workspace `.gitignore`, #4 machine-readable challenge `category` field. Items 5-10 target their in-repo driver script, not us.

**Session-start note:** beads DB was empty again (v1.0.0 Dolt wipe recurred); restored via `bd import .beads/issues.jsonl` (639 issues). Uncommitted `.claude/docs/lean4/*` edits present at session start are unrelated to AF and left alone.

**bd issue:** vibefeld-52ff (P0, closed).

---


### Session 236 Summary: Two related epistemic-state fixes (vibefeld-0mt0, vibefeld-b812), versions 0.1.2 and 0.1.3

Same external consumer flagged two adjacent gaps in the same workflow: archived children blocking parent acceptance, and admit being one-way with no revocation path. Both shipped in one session.

---

### Fix 2: af unadmit — admitted is no longer terminal (vibefeld-b812, 0.1.3)

`schema/epistemic.go` had no outgoing transition from `EpistemicAdmitted`, making admit a permanent stamp. Asymmetric with validated, which has had `unvalidate` since session 232. Worse: admit explicitly means "accepted without full verification" — exactly the state a verifier would want to revoke once the underlying claim has been rigorously verified. Once you used admit to bypass a temporary blocker (including the now-fixed archived-children blocker above), the taint stuck forever and `af accept` rejected the node permanently.

**Fix.** Mirrored the unvalidate pattern end-to-end: schema transition `admitted → pending`, `NodeUnadmitted` ledger event, `applyNodeUnadmitted` state handler, `UnadmitNode` service method, `af unadmit <node-id>` CLI command. Refuted and archived stay terminal as agreed.

**Taint behavior.** Unadmit recomputes taint downward — node becomes `pending`/`unresolved`; descendants that were `tainted` from the admission move to `unresolved` (lineage no longer carries an admission, but isn't re-verified yet). After the user re-validates the node properly with `af accept`, taint propagates back to clean naturally.

**Files changed (10 files):**
- `internal/schema/epistemic.go` — new transition + docstring (admitted → pending; refuted/archived remain terminal)
- `internal/schema/epistemic_test.go` — split `AdmittedToAny` into `AdmittedToPending` (success) + `AdmittedToOthers` (errors)
- `internal/ledger/event.go` — `EventNodeUnadmitted` constant, `NodeUnadmitted` struct, `NewNodeUnadmitted` factory
- `internal/state/apply.go` — `applyNodeUnadmitted` handler + dispatch
- `internal/state/replay.go` — factory entry + deref case
- `internal/state/replay_unit_test.go` — factory completeness for `EventNodeUnadmitted`
- `internal/service/proof.go` — `UnadmitNode(nodeID, reason, revokedBy)` mirroring `UnvalidateNode`
- `cmd/af/unadmit.go` — NEW CLI command (mirrors `unvalidate.go`, verifier group, `--reason`/`--agent`/`--format`/`-y` flags)
- `cmd/af/unadmit_test.go` — NEW integration tests (Success, WithReason, JSONFormat, NotAdmitted, NonExistent, InvalidNodeID, RoundTripToAccept)
- `cmd/af/main.go` + `cmd/af/changelog.go` — version 0.1.2 → 0.1.3, changelog entry

**Smoke test.** Round-trip in `/tmp/af-unadmit-smoke`: admit → `[admitted/self_admitted]` → unadmit → `[pending/unresolved]` → accept → `[validated/clean]`. Taint transitions correctly at each step.

**bd issue:** vibefeld-b812 (P1, closed).

---

### Fix 1: Archived children no longer block parent acceptance (vibefeld-0mt0, 0.1.2)

User-reported bug: when a sub-tree was archived because its strategy was superseded and replaced by a fresh validated chain of new children, the parent could not be re-validated. `af accept` rejected with "children not yet validated", and the only escape was `af admit`, which incorrectly stamped rigorously verified work as taint-introducing.

Root cause was at `internal/service/proof.go:846`: the children-completeness check inside `AcceptNodeWithNote` only allowed `validated || admitted`, even though `archived` is `IsFinal: true` per `schema/epistemic.go` and is treated as a terminal verdict everywhere else (critical-path command, accept-bulk, veto). The documented invariant at `internal/node/validate_invariant.go:49` had the same gap.

**Fix.** Extended both allowlists to include `EpistemicArchived`. Refuted intentionally NOT included: refuted means "this step is *false*", which is a real obstacle to the parent (vs. archived = "branch abandoned, parent no longer relies on it"). Error message ("children not yet validated") kept as-is — only fires now when children are pending/draft/needs_refinement, which is accurate.

**Files changed (6 files, +178/-6):**
- `internal/service/proof.go` — runtime check at line 846, with WHY comment
- `internal/node/validate_invariant.go` — invariant function + docstring
- `internal/node/validate_invariant_test.go` — 4 new MixedChildStates table cases (validated+archived OK, admitted+archived OK, all archived OK, archived+pending still errors)
- `internal/service/proof_test.go` — 2 new regression tests: `RevalidateAfterRefinement_ArchivedChild` and `RevalidateAfterRefinement_ValidatedAndArchivedChildren` (the real deployment scenario from the bug report). Both live next to `RevalidateAfterRefinement_AdmittedChild`, which means they're under `//go:build integration`.
- `cmd/af/main.go` — version 0.1.1 → 0.1.2
- `cmd/af/changelog.go` — 0.1.2 entry covering this fix and the stdout routing fix from session 235

**Smoke test.** Built `/tmp/af-archived-bug-smoke` proof, refined root with two children, archived 1.1, validated 1.2, accepted parent 1 → succeeded. Final tree: `1 [validated/clean]` with `1.1 [archived/clean]` and `1.2 [validated/clean]`. Before the fix this last accept would have errored with "children not yet validated: 1.1".

**Note on test gating discovered en route.** `internal/service/proof_test.go` is gated `//go:build integration` and shares test function names with `internal/service/service_test.go` (untagged). Running with `-tags=integration` errors with duplicate-test redeclarations (pre-existing; not introduced this session). Standard `go test ./...` passes; integration tests are not exercised by the default suite. Worth filing a follow-up if integration-tagged tests should ever run in CI.

**bd issue:** vibefeld-0mt0 (P1, closed).

**Quality gates:** `go test ./...` all 27 packages pass, clean build, `af --version` reports 0.1.2.

---

### Session 235 Summary: Beads v1.0.0 recovery, PR #1 merge, stdout routing fix

Four strands of work this session:

**1. Beads v1.0.0 migration recovery.** `bd` auto-upgraded from v0.55.1 → v1.0.0 on session start. The migration replaced SQLite+JSONL with embedded Dolt storage and reported 0 issues — but `issues.jsonl` (519KB, 639 issues) was intact. `bd import` restored everything. Also:
- Updated `.beads/.gitignore` to cover the new runtime files: `dolt-server.{lock,log,pid,port}`, `embeddeddolt/`, `backup/`, `.beads-credential-key`
- Removed obsolete `interactions.jsonl` (v1.0.0 no longer writes it)
- Reinstalled git hooks (`bd hooks install`) — old shims called `bd hook` which v1.0.0 renamed to `bd hooks run`
- Commit `09947d4`

**2. PR #1 merged (Jonathan Oppenheim, first-time contributor).** Six commits, +2449/−155. All 6 kept with their authorship via `git merge --no-ff`.
- `scripts/auto-prove.sh` overhaul: prover-first dispatch, smart actionability gate (non-leaf provers allowed only with open challenges, fixes the ~88% stall), churn detection with bounded retries + global reset, subtree diversity rotation, parallel dispatch (`--parallel N`, default 5), agent timeout with lock reaping, codex backend
- `internal/export/export.go` LaTeX rewrite: Lamport `\step{ID}` in tcolorbox, ket notation with Greek letter map, type prefixes for assume/case/q.e.d.
- `ralph.sh` codex/claude switch + `set -euo pipefail`
- `demo/no-cloning/`: full Lamport tree + Lean 4 formalisation
- Merge commit `e5e4156`

Follow-up cleanup commit `4c07bf2`:
- Stripped 6 LaTeX build artefacts (.aux, .log, 0-byte .pdf) from `demo/no-cloning/` (~1500 lines of compiler noise)
- Added `demo/.gitignore` for `*.aux`, `*.log`, `*.pdf`, etc.
- Widened ket regex to allow `_` so subscripted kets like `|psi_0>` render
- Added `TestToLaTeX_KetNotationSubscripted`
- Filed `vibefeld-ld1e` for the "af get JSON pipe bug" the PR worked around in `is_actionable`
- Decided NOT to restore LaTeX inference/taint metadata — the cleaner Lamport output is the right call

**3. MaxChildren pain point: no code change needed.** User hit repeated child-limit errors. Investigation:
- Repo source default is already 100 (bumped from 20 in `38f844e`, 2026-03-04)
- Validation cap is 100
- Every real-world meta.json has `max_children` unset, so the running binary's compiled-in default applies
- **Installed binary at `/home/tobiasosborne/go/bin/af` was from Feb 15** — between the 20→100 bump. Its compiled default was 20.
- Fix was `go install ./cmd/af`. No commit.

**4. Fixed `vibefeld-ld1e` — stdout routing bug (commit `d8269b3`).** The bug is broader than the title suggests. 13 CLI commands used cobra's `cmd.Print/Println/Printf`, which default to **stderr**. Text output still looked right on a terminal, so nobody noticed — but piping to `jq`, `grep`, `wc`, or any filter captured zero bytes.

Concrete repro (before fix):
```
af get 1.1.1 -f json 2>/dev/null | jq .   # printed nothing
af get 1.1.1 -f json 2>/tmp/err >/tmp/out # stdout 0 bytes, stderr 1181 bytes
```

Fix: replaced 178 `cmd.Print*` calls with `fmt.Fprint*(cmd.OutOrStdout(), …)` across 13 files (`amend`, `amendments`, `claim`, `deps`, `diff`, `extend-claim`, `get`, `init`, `nearby`, `path`, `refine`, `scope`, `submit`). Matches the pattern already used by `challenges` and the rest of the package. `OutOrStdout()` still honours `SetOut(buf)` in tests.

Also removed the now-stale "Bug 3" workaround comment in `scripts/auto-prove.sh`. The `is_actionable` implementation itself was left alone — calling `af challenges` once and filtering is reasonable regardless of the pipe bug.

**Files changed across session (17 files):**
- `.beads/.gitignore` — bd v1.0.0 runtime files
- `handoff.md` — this file
- `demo/.gitignore` — NEW
- `internal/export/export.go`, `internal/export/export_test.go` — ket regex + test
- `scripts/auto-prove.sh` — stale comment removal
- `cmd/af/{amend,amendments,claim,deps,diff,extend_claim,get,init,nearby,path,refine,scope,submit}.go` — cmd.Print → fmt.Fprint(cmd.OutOrStdout(), …)
- Plus the merged PR diff (already landed)

**Testing**: all 27 packages pass, clean build. Live smoke-tested `af get ... | jq .` end-to-end.

---

### Session 234 Summary: Version bump 0.1.1 + changelog command

**Version bumped** from 0.1.0 to 0.1.1 in `cmd/af/main.go`. Installed for all users via `go install`.

**Added `af changelog`** — simple command listing what's new in each version. Solves discoverability: 60+ commands in `af --help` meant returning users would never notice new features like `af attach`, `af diff`, `af submit`, etc.

**Smoke-tested on all 9 real proof trees** in `../firstproof/problem01` through `problem09`. All load, parse, and render correctly with `af status`, `af progress`, and `af challenges`.

**Files changed** (2 files):
- `cmd/af/main.go` — Version `0.1.0` → `0.1.1`
- `cmd/af/changelog.go` — NEW: changelog command (~75 lines)

---

### Session 233b Summary: Attach computational evidence (vibefeld-tio5, P1)

**Closed vibefeld-tio5 [P1]**: `af attach` and `af evidence` — link scripts and results to proof nodes with content hashing.

**Problem solved**: Every serious deployment created external verification scripts (124 Python scripts in problem04, 63 in examples7, Julia scripts in examples5). AF had zero way to attach, track, or record computational evidence — results were cited in prose but not in the ledger.

**What was added**:
- `af attach <node-id> <file-path> --type verification|computation|test|other` — link evidence to a node
- `af evidence <node-id>` — list all attached evidence for a node
- `EvidenceAttached` ledger event with SHA256 content hash for reproducibility
- `Evidence` state tracking (per-node, replayed from ledger)
- `AttachEvidence()` service method with CAS concurrency + file hashing
- Both commands support `--format json`, `--agent`, `--description` flags
- Evidence type validation (verification, computation, test, other)

**Deferred to follow-ups**: `af verify-run` (execute attached scripts) and `af export --include-scripts` (bundle evidence with exports).

**Files changed** (10 files, ~280 lines):
- `internal/ledger/event.go` — `EventEvidenceAttached` constant, `EvidenceAttached` struct, factory
- `internal/state/state.go` — `Evidence` struct, `evidence` map, accessors
- `internal/state/apply.go` — `applyEvidenceAttached()` handler
- `internal/state/replay.go` — factory + deref for `EvidenceAttached`
- `internal/state/replay_unit_test.go` — factory completeness + extraction tests
- `internal/service/proof.go` — `AttachEvidence()` method with SHA256 hashing
- `cmd/af/attach.go` — NEW: CLI command
- `cmd/af/evidence.go` — NEW: CLI command
- `cmd/af/evidence_test.go` — NEW: 8 integration tests

**Testing**: All 27 packages pass, clean build, 8 new tests.

---

### Session 233a Summary: Failed approach registry (vibefeld-fvxp, P1)

**Closed vibefeld-fvxp [P1]**: `af approach-tried` and `af approach-list` — track exhausted proof strategies in the ledger.

**Problem solved**: No mechanism to record "tried X, it fails because Y." Agents wasted effort re-attempting dead approaches. The only protection was HANDOFF.md "DO NOT RETRY" lists (problem04 had 17 killed approaches across two deployments).

**What was added**:
- `af approach-tried <node-id> --approach "..." --outcome "..."` — record a failed approach
- `af approach-list <node-id>` (alias: `af approaches`) — list all failed approaches for a node
- `ApproachTried` ledger event for full audit trail
- `FailedApproach` state tracking (per-node, replayed from ledger)
- `RecordApproachTried()` service method with CAS concurrency control
- Both commands support `--format json` and `--agent` flags

**Files changed** (10 files, ~250 lines):
- `internal/ledger/event.go` — `EventApproachTried` constant, `ApproachTried` struct, factory
- `internal/state/state.go` — `FailedApproach` struct, `failedApproaches` map, accessors
- `internal/state/apply.go` — `applyApproachTried()` handler
- `internal/state/replay.go` — factory + deref for `ApproachTried`
- `internal/state/replay_unit_test.go` — factory completeness + extraction tests
- `internal/service/proof.go` — `RecordApproachTried()` method
- `cmd/af/approach_tried.go` — NEW: CLI command
- `cmd/af/approach_list.go` — NEW: CLI command
- `cmd/af/approach_test.go` — NEW: 9 integration tests

**Testing**: All 27 packages pass, clean build, 9 new tests.

---

### Session 232 Summary: Unvalidate command (vibefeld-dqh3, P1)

**Closed vibefeld-dqh3 [P1]** plus 6 sub-task issues: `af unvalidate` — revert validated nodes back to pending.

**Problem solved**: Once a node was validated, there was no way to revert it. In af-tests/examples5, a formula error was discovered AFTER 39 nodes were validated — workaround required 3 corrective child nodes and 15 challenge resolutions.

**What was added**:
- `af unvalidate <node-id>` — reverts `validated → pending` for re-examination
- `--reason`, `--agent`, `--format (text|json)`, `--yes` flags
- Confirmation prompt (destructive action) unless `--yes`
- Taint auto-propagation: unvalidated node becomes `TaintUnresolved`, propagates to descendants
- Full audit trail preserved (NodeUnvalidated ledger event)

**Files changed** (8 files, ~200 lines):
- `internal/schema/epistemic.go` — added `validated → pending` transition
- `internal/ledger/event.go` — `NodeUnvalidated` event type, struct, factory
- `internal/state/apply.go` — `applyNodeUnvalidated()` handler
- `internal/state/replay.go` — factory + deref for `NodeUnvalidated`
- `internal/service/proof.go` — `UnvalidateNode()` method with CAS + taint
- `cmd/af/unvalidate.go` — NEW: CLI command
- `cmd/af/unvalidate_test.go` — NEW: 6 integration tests
- `internal/schema/epistemic_test.go` — updated for new transition
- `internal/state/replay_unit_test.go` — factory completeness

**Testing**: All 27 packages pass, clean build, clean vet, 6 new tests.

---

### Session 231b Summary: Status navigation (vibefeld-h4wb, P1)

**Closed vibefeld-h4wb [P1]**: 5 of 6 status navigation features for large proof trees.

**What was added**:
- `af status --focus <node-id>` — show only subtree rooted at a node
- `af status --depth N` — limit tree display depth (relative to focus if combined)
- `af status --compact` — one line per node with challenge count badges, no legend
- `af path <node-id>` — show ancestry chain: 1 [state] → 1.6 [state] → 1.6.4 [state]
- `af nearby <node-id>` — show parent, siblings, and children

**Filed vibefeld-xjwm [P2]**: `--critical-path` follow-up (longest unvalidated chain).

**Files changed** (6 files):
- `cmd/af/status.go` — added --focus, --depth, --compact flags
- `cmd/af/path.go` — NEW: path command
- `cmd/af/nearby.go` — NEW: nearby command
- `cmd/af/status_nav_test.go` — NEW: 9 integration tests
- `internal/render/status.go` — RenderStatusFiltered(), StatusOptions, renderCompactTree()
- `internal/render/tree.go` — FormatNodeLine() public API

**Testing**: All 27 packages pass, 9 new tests.

---

### Session 231a Summary: Amendment diffs (vibefeld-ndzg, P1)

**Closed vibefeld-ndzg [P1]**: Implemented `af amendments` and `af diff` commands for node version history.

**Problem solved**: When nodes are amended, verifiers couldn't tell if their challenge was addressed. Users had to manually diff ledger events. Nodes like problem01's 1.6.4.3 (7 amendments) and problem08's 1.3 (24 challenges, 2 amendments) were untrackable.

**What was added**:
- `af amendments <node-id>` — lists all versions with timestamps, owners, and full statements
- `af diff <node-id>` — shows diff between previous and current version
- `af diff <node-id> --version N` — diff from version N to current
- `af diff <node-id> --all` — shows all diffs in chronological order
- `af diff <node-id> --since-challenge <id>` — changes since a challenge was raised
- Both commands support `--format json` for machine-readable output

**Files changed** (4 files, ~450 lines):
- `cmd/af/amendments.go` — NEW: amendments command (~140 lines)
- `cmd/af/amendments_test.go` — NEW: 7 integration tests
- `cmd/af/diff.go` — NEW: diff command (~260 lines)
- `cmd/af/diff_test.go` — NEW: 10 integration tests

**No service/state changes needed** — `LoadAmendmentHistory()`, `GetAmendmentHistory()`, and `state.Amendment` struct were already implemented. This was purely CLI-layer work.

**Testing**: All 27 packages pass, clean build, 17 new tests all passing.

---

### Session 230 Summary: Draft/WIP epistemic state (vibefeld-qcdm, P0)

**Closed vibefeld-qcdm [P0]** plus 12 sub-task issues: Full implementation of draft/WIP epistemic state for iterative proof development.

**Problem solved**: AF's binary pending/validated model forced premature verification. problem05 abandoned after 45 challenges, examples3 abandoned AF entirely. Proofs develop iteratively but AF had no way to express "work in progress."

**What was added**:
- `EpistemicDraft` state: non-final, no taint, challenges are non-blocking suggestions
- `af refine --draft` flag: creates nodes in draft state
- `af submit <node-id>` command: promotes draft→pending for formal verification
- State transitions: `draft→pending` (submit), `draft→archived` (abandon)
- `NodeSubmitted` ledger event for audit trail
- Blue color rendering for draft nodes in `af status`
- Draft nodes appear as prover jobs (need development), not verifier jobs
- `GetBlockingChallengesForNode()` returns empty for draft nodes

**Files changed** (17 files, +250/-21):
- `internal/schema/epistemic.go` — new state, registry, transitions
- `internal/ledger/event.go` — NodeSubmitted event
- `internal/state/apply.go` — apply handler + LockReaped no-op fix
- `internal/state/replay.go` — factory + deref
- `internal/state/state.go` — draft challenge bypass
- `internal/taint/compute.go` — draft=unresolved
- `internal/node/node.go` — Draft in NodeOptions
- `internal/service/proof.go` — Draft in RefineSpec/ChildSpec, SubmitNode()
- `internal/service/exports.go` — EpistemicDraft export
- `internal/render/color.go` — Blue for draft
- `internal/render/status.go` — stats, legend, jobs
- `internal/jobs/prover.go` — draft as prover job
- `cmd/af/refine.go` — --draft flag
- `cmd/af/refine_sibling.go` — pass-through fix
- `cmd/af/submit.go` — NEW command
- `internal/schema/epistemic_test.go` — updated count
- `internal/state/replay_unit_test.go` — factory completeness + parse test

**Testing**: All 27 packages pass, clean build, smoke-tested end-to-end.

---

### Session 229 Summary: af handoff + challenge triage (vibefeld-4p8f, vibefeld-n52z)

**Closed vibefeld-n52z [P0]**: Challenge triage — added filtering and summary to `af challenges`.

**New flags**:
- `--severity critical|major|minor|note` — filter by severity level
- `--active-only` — shorthand for `--status open`
- `--summary` — aggregate view: counts by node and severity in a table
- `--status` now accepts `superseded` (was missing from validation)

**Auto-supersede**: Already implemented in state layer (`applyNodeArchived`/`applyNodeRefuted` call `supersedeOpenChallengesForNode`). No service-layer changes needed.

**Files changed**:
- `cmd/af/challenges.go` — Added 3 flags, severity filter, summary renderers (~100 lines added)
- `cmd/af/challenges_test.go` — 9 new integration tests, fixed superseded status validation test

**Testing**: All 27 packages pass. One pre-existing integration test failure (`TestChallengesCmd_FilterByNonExistentNode` uses invalid node "2").

---

### Session 229 Summary: af handoff command (vibefeld-4p8f)

**Closed vibefeld-4p8f [P0]**: Implemented `af handoff` command that generates concise handoff reports for session transitions.

**Features**:
- Proof summary: conjecture, completion %, node counts by epistemic state, taint summary
- Open challenges grouped by node with severity counts (critical/major/minor/note), sorted by priority
- Recommended next steps based on available jobs, critical challenges, taint issues
- Recent activity via `--since <seq>` flag (filters noise events like taint_recomputed)
- Both text and JSON output formats (`--format json`)

**Files changed**:
- `cmd/af/handoff.go` — New CLI command (~300 lines)
- `cmd/af/handoff_test.go` — 6 integration tests (no-proof text/JSON, basic proof text/JSON, --since, invalid format)

**Testing**: All 27 packages pass, clean build.

---

### Session 228 Summary: Taint system fixes (vibefeld-w9qr, vibefeld-ayl9)

**1. Fixed P1 bug vibefeld-w9qr**: Archived and refuted nodes now always compute as `TaintClean`, regardless of ancestor taint state. Previously, archived/refuted nodes inherited `TaintUnresolved` from pending ancestors, causing phantom taint to block progress on abandoned branches.

**Change**: Added rule 0 to `ComputeTaint()` in `internal/taint/compute.go` — if node is archived or refuted, return `TaintClean` immediately.

**2. Closed vibefeld-ayl9**: Auto taint computation was already implemented (`emitTaintRecomputedEvents` called from accept/admit/refute/archive). Added accept warning for tainted deps to CLI — `af accept` now warns on stderr if the node has admitted/tainted children.

**Filed vibefeld-z8tc**: `af taint-trace` command (P2 follow-up from vibefeld-ayl9).

**3. Closed vibefeld-hw0w**: Added `af update-external <name-or-id>` command with `--name`, `--source`, `--notes` flags. Resolves by name or ID. Content hash recomputed on source change. Service method `UpdateExternal()` added to proof.go.

**Files changed**:
- `internal/taint/compute.go` — Added rule 0 (6 lines)
- `internal/taint/compute_test.go` — 4 new tests for archived/refuted with tainted ancestors
- `cmd/af/accept.go` — Added `warnTaintedDeps()` function, called before acceptance
- `cmd/af/accept_test.go` — 1 integration test for taint warning
- `internal/service/proof.go` — Added `UpdateExternal()` method
- `cmd/af/update_external.go` — New CLI command `af update-external`
- `cmd/af/update_external_test.go` — 4 integration tests

**Testing**: All 27 packages pass, clean build, clean vet.

---

### Session 227 Summary: Holistic project review and strategic prioritization

**Full project audit** across all 609 tracked issues, 15 field deployments, First Proof post-mortem, codebase health, and git history. Five parallel research agents analyzed issues, docs/PRD, git trajectory, build health, and feature proposals.

**Key findings:**
- Core adversarial verification thesis validated — catches real math errors in every deployment
- UX breaks at scale (50+ nodes): challenges pile up, status is unusable, no iterative workflow
- Taint system (Law 8): code investigation reveals auto-triggering IS implemented (emitTaintRecomputedEvents called from accept/admit/refute/archive), and tree renderer shows taint badges. Deployments show "all unresolved" because most nodes stayed `pending` (taint rule: pending → unresolved is correct). Remaining gaps: no `af accept` warning for tainted deps, no `af taint-trace` command.
- 25 open issues all from field experience, forming a coherent priority stack
- Codebase healthy: all 27 packages pass, clean build/vet, 13 packages above 80% coverage

**Strategic recommendation: Fix v0.1 before building v0.2.**
- P0/P1 issues are well-scoped UX fixes that address ~80% of observed field friction
- v0.2 features (forest mode, slice queries, learnings tree) need the P0/P1 fixes to be useful
- Design v0.2 now, but build it after the foundation is solid

---

### Session 226 Summary: Deployment analysis — 12 improvement issues filed from field usage

**Investigated 15 real AF deployments** across ~/Projects/firstproof/problem0{1,2,3,4,5,8} and ~/Projects/af-tests/examples{1-9}. Spawned 8 background subagents to analyze ledgers, handoffs, and proof trees. Each wrote a report to /tmp/af-deployment-reports/.

**Key finding**: AF's adversarial verification core works — caught real mathematical errors in every deployment (fabricated citations, wrong formulas, logical fallacies). But challenge management, iterative refinement, and proof navigation impose severe friction at scale (50+ nodes).

**Filed 12 beads issues from synthesis:**

| ID | Pri | Title |
|----|-----|-------|
| vibefeld-n52z | P0 | Challenge triage: severity filtering, auto-supersede |
| vibefeld-4p8f | P0 | Auto-generate handoff command |
| vibefeld-qcdm | P0 | Draft/WIP state: non-blocking challenges |
| vibefeld-ndzg | P1 | Amendment diffs: af diff / af amendments |
| vibefeld-h4wb | P1 | Status navigation: --focus, --depth, --critical-path |
| vibefeld-dqh3 | P1 | Unvalidate/supersede validated nodes |
| vibefeld-w9qr | P1 | Archive severs taint propagation |
| vibefeld-tio5 | P1 | Attach computational evidence to nodes |
| vibefeld-fvxp | P1 | Failed approach registry |
| vibefeld-0z3k | P2 | Workspace fork/import |
| vibefeld-hw0w | P2 | External reference update |
| vibefeld-ayl9 | P2 | Auto taint computation |

Each issue includes concrete deployment evidence (node counts, challenge counts, specific examples from ledgers and HANDOFFs).

**Reports preserved at**: /tmp/af-deployment-reports/*.md (8 files, ~125KB total)

---

### Session 225 Summary: Issue triage - Closed over-engineering tasks

**Closed `vibefeld-264n` and `vibefeld-qsyt` as "by design"**
- Both issues proposed breaking service package into sub-services (stateService, persistenceService, validationService)
- After code review, determined this was over-engineering:
  - Service package is a **Facade pattern** - coordinating multiple subsystems is its purpose
  - **Single clear responsibility**: coordinating proof operations across ledger, state, filesystem
  - **No circular dependencies** - imports flow one direction
  - **Clean public API** - well-documented, well-designed methods
- Breaking into 3 sub-services would add indirection without measurable benefit
- `vibefeld-qsyt` closed as duplicate of `vibefeld-264n`
- `vibefeld-264n` closed as "by design"

---

### Session 224 Summary: API design - Added NodeSummary view model

**Closed `vibefeld-vj5y` - API design: Service layer leaks domain types**
- Added `NodeSummary` view model struct to `internal/service/exports.go`
  - Contains only fields needed for CLI display: ID, Type, Statement, Inference
  - Decouples CLI from internal `node.Node` type
- Added `LoadPendingNodeSummaries()` method to proof.go
  - Returns `[]NodeSummary` instead of `[]*node.Node`
  - Prevents CLI from depending on internal domain packages
- Updated CLI callers:
  - `cmd/af/accept.go` - now uses `LoadPendingNodeSummaries()` for `--all` flag
  - `cmd/af/wizard.go` - now uses `LoadPendingNodeSummaries()` for verifier review
- Marked `LoadPendingNodes()` as deprecated (kept for backward compatibility)

---

### Session 223 Summary: Refactored proof.go - Extracted cycle detection

**Closed `vibefeld-tk76` - Refactor proof.go god object into smaller modules**
- Created `internal/service/proof_cycle.go` (90 lines) with:
  - `stateDependencyProvider` type (adapts state.State for cycle detection)
  - `GetNodeDependencies()` method
  - `AllNodeIDs()` method
  - `CheckCycles()` - check cycles from a specific node
  - `CheckAllCycles()` - check all nodes for cycles
  - `WouldCreateCycle()` - validate proposed dependencies
- Reduced `proof.go` from 2071 to 1990 lines (-81 lines)
- All tests pass

---

## Current State

### Test Status
- All tests pass (`go test ./...`) — 27/27 packages
- Build succeeds, `go vet` clean
- Coverage highlights: 13 packages >80%, taint/hash/scope at 100%
- Weak spots: `cmd/af` 23%, `render` 41.6%, `ledger` 59.3%

### Issue Statistics (663 total, 651 closed)
- **P0 open:** 0 (all P0s closed!)
- **P1 open:** 0 (all P1s closed! taint-trace is P2)
- **P2 open:** 5 (critical-path, workspace fork, falsification, def stress testing, strategy diversity)
- **P3 open:** 3 (v0.2 designs: queries, learnings, forest)

### Codebase
- 367 Go files, ~176K LOC, 60+ CLI commands
- 552 commits across 14 active development days

## Recommended Next Steps

### Tier 1 — Highest leverage P1 features

**1. vibefeld-h4wb [P1] Status navigation** — `--focus`, `--depth`, `--compact`, `--critical-path`.
- Addresses the "302KB wall" from the status side
- Touch: render, CLI

**3. vibefeld-dqh3 [P1] Unvalidate/supersede** — Allow reverting validated nodes.
- Touch: schema (transition), service, CLI

**4. vibefeld-tio5 [P1] Attach computational evidence** — Link scripts/results to nodes.
- Touch: ledger (new event), service, CLI

**5. vibefeld-fvxp [P1] Failed approach registry** — Track exhausted strategies.
- Touch: ledger (new event), service, CLI

### Tier 3 — Strategic features (design now, build after Tier 1-2)

**8. vibefeld-fvxp [P1] Failed approach registry** — `af approach-tried`, `af approach-list`
**9. vibefeld-tio5 [P1] Attach computational evidence** — `af attach`, `af verify-run`
**10. vibefeld-dqh3 [P1] Unvalidate/supersede** — `af unvalidate`, `af supersede`

### Tier 4 — v0.2 horizon (design only)

- **vibefeld-t9u6** Forest mode (multiple roots per workspace)
- **vibefeld-q05l** Slice queries (composable tree queries for agents)
- **vibefeld-95mk** Learnings tree (structured meta-knowledge)
- **vibefeld-p125** Conjecture falsification (dual proof/disproof trees)

## Quick Commands

```bash
bd ready           # See ready work
go test ./... -short  # Run tests (fast)
go build ./cmd/af  # Build
```

## Session History

**Session 234:** Version bump 0.1.1 + af changelog command, smoke-tested all 9 proof trees
**Session 233:** Failed approach registry (fvxp) + evidence attachment (tio5) — af approach-tried, af approach-list, af attach, af evidence, 17 new tests
**Session 232:** Unvalidate command (dqh3) — af unvalidate, validated→pending, taint propagation, 6 tests
**Session 231:** Amendment diffs (ndzg) + status navigation (h4wb) — af amendments, af diff, af path, af nearby, --focus/--depth/--compact, 26 tests
**Session 230:** Draft/WIP state (qcdm, P0) — new epistemic state, af refine --draft, af submit, 12 sub-issues closed
**Session 229:** af handoff (4p8f) + challenge triage (n52z) — handoff reports, severity/summary/active-only filters
**Session 228:** Taint fixes (w9qr, ayl9) + update-external command (hw0w), filed z8tc
**Session 227:** Holistic project review — strategic prioritization of 25 open issues into 4 execution tiers
**Session 226:** Deployment analysis — investigated 15 real AF deployments, filed 12 improvement issues (3 P0, 6 P1, 3 P2)
**Session 225:** Issue triage - closed vibefeld-264n, vibefeld-qsyt as "by design" (over-engineering)
**Session 224:** Added NodeSummary view model, LoadPendingNodeSummaries() method (vibefeld-vj5y)
**Session 223:** Extracted cycle detection to proof_cycle.go, proof.go reduced by 81 lines (vibefeld-tk76)
**Session 222:** Eliminated schema import, down to 5 internal imports (vibefeld-jfbc progress)
**Session 221:** CLI API design: refine-sibling command (vibefeld-yo5e), removed --statement flag (vibefeld-9b6m)
**Session 220:** Service test coverage from 67.5% to 75.6% (+8.1%), 25 new tests (vibefeld-8q2j)
**Session 219:** CLI code quality: confirmation helper (vibefeld-1amd) + flag standardization (vibefeld-2yy5)
**Session 218:** Completed request-refinement feature (vibefeld-pno3, vibefeld-na20, vibefeld-boar)
**Session 217:** Added RequestRefinement to proof service (vibefeld-wfkj) and render support for needs_refinement (vibefeld-0hx6)
**Session 216:** Integrated RefinementRequested into state derivation (vibefeld-xt2o) and prover jobs (vibefeld-cvlz)
**Session 215:** Implemented needs_refinement epistemic state (vibefeld-9184) and RefinementRequested ledger event (vibefeld-jkxx)
**Session 214:** Fixed vibefeld-si9g (nil receiver checks for Challenge and Node methods)
**Session 213:** Fixed vibefeld-lwna (lock release-after-free semantics) and vibefeld-bs2m (External return type consistency)
**Session 212:** Fixed P1 bug vibefeld-u3le - LoadState silent error swallowing
**Session 211:** Fixed P1 bug vibefeld-1a4m - Lock clock skew vulnerability
**Session 210:** Fixed P0 bugs vibefeld-db25 (challenge severity validation) and vibefeld-vgqt (AcceptNodeWithNote children validation)
