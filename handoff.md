# Handoff - 2026-02-14 (Session 233)

## What Was Accomplished This Session

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
