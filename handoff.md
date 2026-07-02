# Handoff - 2026-07-02 (Session 237)

## What Was Accomplished This Session

### Session 237 Summary: Fixed --dry-run global no-op (vibefeld-52ff, 0.1.4)

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
